package seewo_pinco

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/avast/retry-go"
)

func (d *SeewoPinco) regularUpload(ctx context.Context, dstDir model.Obj, file model.FileStreamer, md5 string, formMeta PostV1DriveMaterialsMatchResp_FormUploadMeta, up driver.UpdateProgress) error {
	// Create multipart form
	b := bytes.NewBuffer(make([]byte, 0, bytes.MinRead))
	w := multipart.NewWriter(b)
	for k, v := range formMeta.Fields {
		if err := w.WriteField(k, v); err != nil {
			return err
		}
	}
	_, err := w.CreateFormFile("file", file.GetName())
	if err != nil {
		return err
	}
	headSize := b.Len()
	if err := w.Close(); err != nil {
		return err
	}

	// Prepare upload stream with progress
	head := bytes.NewReader(b.Bytes()[:headSize])
	tail := bytes.NewBuffer(b.Bytes()[headSize:])
	length := int64(b.Len()) + file.GetSize()

	rd := driver.NewLimitedUploadStream(ctx, &driver.ReaderUpdatingProgress{
		Reader: &driver.SimpleReaderWithSize{
			Reader: io.MultiReader(head, file, tail),
			Size:   length,
		},
		UpdateProgress: up,
	})

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, formMeta.URL, rd)
	if err != nil {
		return err
	}
	req.Header.Set("content-type", w.FormDataContentType())
	req.Header.Set("user-agent", base.UserAgent)
	req.ContentLength = length
	for k, v := range formMeta.Headers {
		req.Header.Set(k, v)
	}

	// Execute upload
	res, err := base.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("cstore upload error: status %d, body: %s", res.StatusCode, string(body))
	}

	var csr CStoreUploadResp
	if err := json.Unmarshal(body, &csr); err != nil {
		return err
	}
	if csr.StatusCode != 0 {
		return fmt.Errorf("cstore upload error: status %d, body: %s", csr.StatusCode, csr.Message)
	}

	// Finalize upload
	return d.finalizeUpload(ctx, dstDir, file, md5, csr.Data)
}

func (d *SeewoPinco) chunkedUpload(ctx context.Context, dstDir model.Obj, file model.FileStreamer, md5 string, up driver.UpdateProgress) error {
	// Get chunked upload policy
	var policyResp PostV1CstoreMultipartUploadPolicyResp
	err := d.api(ctx, "PostV1CstoreMultipartUploadPolicy", base.Json{
		"fileSize": file.GetSize(),
		"fileName": file.GetName(),
		"mimeType": file.GetMimetype(),
		"suffix":   utils.Ext(file.GetName()),
	}, &policyResp)
	if err != nil {
		return err
	}

	if len(policyResp.Data.PolicyList) == 0 {
		return fmt.Errorf("no upload policy received")
	}
	policy := policyResp.Data.PolicyList[0]

	// Validate upload method
	if policy.UploadMethod != "PUT" || policy.UploadMsgField != "etag" || policy.UploadMsgType != "HEADER" {
		return fmt.Errorf("unsupported upload policy: %+v", policy)
	}

	uploadId := policy.UploadID

	// Calculate chunk size and count
	chunkSize := policy.PartSize // int64(2097152) // 2MB per chunk
	totalSize := file.GetSize()
	chunkCount := int((totalSize + chunkSize - 1) / chunkSize)

	// start chunked upload
	partMsgList := make([]string, 0)

	// Use stream section reader for efficient chunk reading
	ss, err := stream.NewStreamSectionReader(file, int(chunkSize), &up)
	if err != nil {
		return err
	}

	// Upload each chunk
	for i := range chunkCount {
		// Check for cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		start := int64(i) * chunkSize
		end := min(start+chunkSize, totalSize)
		length := end - start

		// Get rule for this specific part
		var rule PostV1CstoreMultipartUploadRuleResp
		err = d.api(ctx, "PostV1CstoreMultipartUploadRule", base.Json{
			"uploadId":   uploadId,
			"partNumber": i + 1,
			"size":       length,
		}, &rule)
		if err != nil {
			return err
		}

		// Get section reader for this chunk
		rd, err := ss.GetSectionReader(start, length)
		if err != nil {
			return err
		}

		// Upload chunk with retry logic
		var etag string
		err = retry.Do(func() error {
			// Reset reader position for retry
			rd.Seek(0, io.SeekStart)

			req, err := http.NewRequestWithContext(ctx, http.MethodPut, rule.Data.UploadURL, rd)
			if err != nil {
				return err
			}
			req.ContentLength = length

			req.Header.Set("user-agent", base.UserAgent)
			for _, header := range rule.Data.HeaderFields {
				req.Header.Set(header.Key, header.Value)
			}

			res, err := base.HttpClient.Do(req)
			if err != nil {
				return err
			}
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}

			if res.StatusCode != 200 {
				return fmt.Errorf("chunk upload error: status %d, body: %s", res.StatusCode, string(body))
			}

			if etagHeader := res.Header.Get("ETag"); etagHeader != "" {
				etag = etagHeader
			} else {
				return fmt.Errorf("chunk upload error: missing ETag in response headers")
			}

			return nil
		},
			retry.Context(ctx),
			retry.Attempts(3),
			retry.Delay(time.Second),
			retry.DelayType(retry.BackOffDelay))
		// Free the section reader
		ss.FreeSectionReader(rd)
		if err != nil {
			return err
		}

		// Store part message for completion
		partMsgList = append(partMsgList, etag)

		// Update progress
		if up != nil {
			percent := float64(i+1) / float64(chunkCount) * 100
			up(percent)
		}
	}

	// Complete multipart upload
	var completeResp PostV1CstoreMultipartCompleteResp
	err = d.api(ctx, "PostV1CstoreMultipartComplete", base.Json{
		"uploadId":    uploadId,
		"partMsgList": partMsgList,
		"fileSize":    totalSize,
	}, &completeResp)
	if err != nil {
		return err
	}

	// Finalize with the complete response data
	finalData := CStoreUploadResp_Data{
		FileSize:    completeResp.Data.FileSize,
		DownloadURL: completeResp.Data.DownloadURL,
		FileKey:     completeResp.Data.FileKey,
	}

	return d.finalizeUpload(ctx, dstDir, file, md5, finalData)
}

func (d *SeewoPinco) finalizeUpload(ctx context.Context, dstDir model.Obj, file model.FileStreamer, md5 string, uploadData CStoreUploadResp_Data) error {
	// Step3: Finalize the upload
	var cr PostV1DriveMaterialsCstoreWayResp
	err := d.api(ctx, "PostV1DriveMaterialsCstoreWay", base.Json{
		"fileSize":       uploadData.FileSize,
		"downloadUrl":    uploadData.DownloadURL,
		"fileKey":        uploadData.FileKey,
		"fileMd5":        md5,
		"name":           file.GetName(),
		"parentFolderId": dstDir.GetID(),
		"size":           file.GetSize(),
		"mimeType":       file.GetMimetype(),
	}, &cr)
	if err != nil {
		return err
	}
	return nil
}
