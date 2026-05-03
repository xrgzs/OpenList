package geegeng

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/go-resty/resty/v2"
)

type Geegeng struct {
	model.Storage
	Addition

	client   *resty.Client
	userInfo *UserInfo
}

func (d *Geegeng) Config() driver.Config {
	return config
}

func (d *Geegeng) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Geegeng) Init(ctx context.Context) error {
	d.Token = strings.TrimPrefix(d.Token, "Bearer ")
	op.MustSaveDriverStorage(d)

	d.client = base.NewRestyClient().
		SetBaseURL("https://www.geegeng.com/api").
		SetCookieJar(nil).
		SetHeaders(map[string]string{
			"Authorization": "Bearer " + d.Token,
			"Cookie":        d.Cookie,
			"Origin":        "https://www.geegeng.com",
			"Referer":       "https://www.geegeng.com/files",
			"User-Agent":    d.getUA(),
		})
	userInfo, err := d.getUserInfo()
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}
	d.userInfo = &userInfo
	return nil
}

func (d *Geegeng) Drop(ctx context.Context) error {
	d.client = nil
	d.userInfo = nil
	return nil
}

func (d *Geegeng) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	var resp ListResp
	res, err := d.client.R().
		SetBody(base.Json{
			"parentId": dir.GetID(),
			"page":     1,
			"pageSize": 50,
		}).
		SetResult(&resp).
		Post("/files/getFilesList")
	if err != nil {
		return nil, err
	}
	if !res.IsSuccess() {
		return nil, fmt.Errorf("request failed with status code: %d", res.StatusCode())
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return utils.SliceConvert(resp.Data.List, func(src List) (model.Obj, error) {
		return src.toObj(dir.GetID()), nil
	})
}

func (d *Geegeng) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	var resp LinkResp
	res, err := d.client.R().
		SetBody(base.Json{
			"id": file.GetID(),
		}).
		SetResult(&resp).
		Post("/files/downFile")
	if err != nil {
		return nil, err
	}
	if !res.IsSuccess() {
		return nil, fmt.Errorf("request failed with status code: %d", res.StatusCode())
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return &model.Link{
		URL: resp.Data.Url,
		Header: http.Header{
			"Referer":    []string{"https://www.geegeng.com/"},
			"User-Agent": []string{d.getUA()},
		},
	}, nil
}

func (d *Geegeng) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	var resp BaseResp
	res, err := d.client.R().
		SetBody(base.Json{
			"name":     dirName,
			"parentId": parentDir.GetID(),
		}).
		SetResult(&resp).
		Post("/files/createFolder")
	if err != nil {
		return err
	}
	if !res.IsSuccess() {
		return fmt.Errorf("request failed with status code: %d", res.StatusCode())
	}
	if resp.Code != 0 {
		return fmt.Errorf("API error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (d *Geegeng) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	var resp BaseResp
	res, err := d.client.R().
		SetBody(base.Json{
			"ids":      []string{srcObj.GetID()},
			"parentId": dstDir.GetID(),
		}).
		SetResult(&resp).
		Post("/files/moveFiles")
	if err != nil {
		return err
	}
	if !res.IsSuccess() {
		return fmt.Errorf("request failed with status code: %d", res.StatusCode())
	}
	if resp.Code != 0 {
		return fmt.Errorf("API error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (d *Geegeng) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	var resp BaseResp
	res, err := d.client.R().
		SetBody(base.Json{
			"ID":       srcObj.GetID(),
			"parentId": srcObj.GetPath(),
			"name":     newName,
		}).
		SetResult(&resp).
		Post("/files/updateFileName")
	if err != nil {
		return err
	}
	if !res.IsSuccess() {
		return fmt.Errorf("request failed with status code: %d", res.StatusCode())
	}
	if resp.Code != 0 {
		return fmt.Errorf("API error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (d *Geegeng) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	var resp BaseResp
	res, err := d.client.R().
		SetBody(base.Json{
			"ids":      []string{srcObj.GetID()},
			"parentId": dstDir.GetID(),
		}).
		SetResult(&resp).
		Post("/files/copyFiles")
	if err != nil {
		return err
	}
	if !res.IsSuccess() {
		return fmt.Errorf("request failed with status code: %d", res.StatusCode())
	}
	if resp.Code != 0 {
		return fmt.Errorf("API error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (d *Geegeng) Remove(ctx context.Context, obj model.Obj) error {
	var resp BaseResp
	res, err := d.client.R().
		SetBody(base.Json{
			"ids": []string{obj.GetID()},
		}).
		SetResult(&resp).
		Post("/files/deleteFilesByIds")
	if err != nil {
		return err
	}
	if !res.IsSuccess() {
		return fmt.Errorf("request failed with status code: %d", res.StatusCode())
	}
	if resp.Code != 0 {
		return fmt.Errorf("API error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (d *Geegeng) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) error {
	cachedFile, err := file.CacheFullAndWriter(nil, nil)
	if err != nil {
		return fmt.Errorf("cache file failed: %w", err)
	}

	digest, err := computeFileDigest(cachedFile, file.GetSize())
	if err != nil {
		return err
	}
	cachedFile.Seek(0, io.SeekStart)

	// 调用 findFile 检查是否可以秒传
	var find FindFileResp
	res, err := d.client.R().
		SetBody(base.Json{
			"name":     file.GetName(),
			"md5":      digest.FullMD5,
			"path":     file.GetName(),
			"size":     file.GetSize(),
			"salt":     digest.Salt,
			"parentId": dstDir.GetID(),
		}).
		SetResult(&find).
		Post("/files/findFile")
	if err != nil {
		return err
	}
	if !res.IsSuccess() {
		return fmt.Errorf("request failed with status code: %d", res.StatusCode())
	}
	if find.Code == 0 && find.Data.File.UploadStatus && find.Msg == "success" {
		return nil // 秒传成功，无需上传
	}
	if find.Code != 7 {
		return fmt.Errorf("API error: code %d, message: %s", find.Code, find.Msg)
	}
	uploadBase := find.Data.File.Url

	// 初始化分片上传
	var initResp InitMultiUploadResp
	if err := d.httpPostForm(ctx, uploadBase+"/initMultiUpload", map[string]string{
		"name": file.GetName(),
		"size": strconv.FormatInt(file.GetSize(), 10),
		"md5":  digest.FullMD5,
		"salt": digest.Salt,
		"sign": find.Data.File.Sign,
		"fid":  find.Data.File.Fid,
		"uid":  d.userInfo.ID,
	}, &initResp); err != nil {
		return err
	}
	if initResp.Code != 1 {
		return fmt.Errorf("initMultiUpload error: code %d, message: %s", initResp.Code, initResp.Msg)
	}

	fileName := initResp.Data.FileName
	uploadId := initResp.Data.UploadId
	chunkSize := getChunkSize(file.GetSize())
	chunkCount := len(digest.ChunkMD5s)

	needUploadParts := d.getNeedUploadParts(ctx, uploadBase, fileName, uploadId, chunkCount)
	if len(needUploadParts) == 0 {
		return d.commitMultiUploadFile(ctx, uploadBase, fileName, uploadId)
	}

	for i, partIndex := range needUploadParts {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		chunkIdx := partIndex - 1
		offset := int64(chunkIdx) * chunkSize
		chunkLen := chunkSize
		if offset+chunkLen > file.GetSize() {
			chunkLen = file.GetSize() - offset
		}

		partsInfo := base64.StdEncoding.EncodeToString(
			[]byte(strconv.Itoa(partIndex) + "-" + digest.ChunkMD5s[chunkIdx]),
		)
		var urlResp GetMultiUploadUrlsResp
		if err := d.httpGetJSON(ctx, uploadBase+"/getMultiUploadUrls", map[string]string{
			"fileName":  fileName,
			"uploadId":  uploadId,
			"partsInfo": partsInfo,
			"t":         strconv.FormatInt(time.Now().UnixMilli(), 10),
		}, &urlResp); err != nil {
			return fmt.Errorf("getMultiUploadUrls failed: %w", err)
		}
		if urlResp.Code != 1 {
			return fmt.Errorf("getMultiUploadUrls error: code %d, message: %s", urlResp.Code, urlResp.Msg)
		}

		key := "partNumber_" + strconv.Itoa(partIndex)
		urlItem, ok := urlResp.Data.UploadUrls[key]
		if !ok {
			return fmt.Errorf("upload URL for part %d not found", partIndex)
		}

		uploadURL := urlItem.RequestUrl
		if !strings.HasPrefix(uploadURL, "https://") {
			uploadURL = uploadBase + uploadURL
		}

		header := parseHeaderString(urlItem.RequestHeader)
		chunkReader := io.NewSectionReader(cachedFile, offset, chunkLen)

		resp, err := d.httpPut(ctx, uploadURL, header, chunkReader, chunkLen)
		if err != nil {
			return fmt.Errorf("upload chunk %d failed: %w", partIndex, err)
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("upload chunk %d failed with status code: %d", partIndex, resp.StatusCode)
		}

		etag := resp.Header.Get("ETag")
		expectedETag := `"` + strings.ToLower(digest.ChunkMD5s[chunkIdx]) + `"`
		if etag != "" && etag != expectedETag {
			return fmt.Errorf("chunk %d ETag mismatch: got %s, want %s", partIndex, etag, expectedETag)
		}

		up(float64(i+1) * 100 / float64(len(needUploadParts)))
	}

	return d.commitMultiUploadFile(ctx, uploadBase, fileName, uploadId)
}

func (d *Geegeng) GetDetails(ctx context.Context) (*model.StorageDetails, error) {
	userInfo, err := d.getUserInfo()
	if err != nil {
		return nil, err
	}
	return &model.StorageDetails{
		DiskUsage: model.DiskUsage{
			TotalSpace: userInfo.Store,
			UsedSpace:  userInfo.VusedStore,
		},
	}, nil
}

var _ driver.Driver = (*Geegeng)(nil)
