package zbrowser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/go-resty/resty/v2"
)

// do others that not defined in Driver interface

func (d *ZBrowser) apiRequest(ctx context.Context, path string, value any, resp any) (*resty.Response, error) {
	if d.client == nil {
		return nil, fmt.Errorf("[ZBrowser] client not initialized")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] API request marshal error: %w", err)
	}
	cookie := fmt.Sprintf("__guid=%s; Q=%s; __NS_Q=%s; T=%s; __NS_T=%s", d.GUID, d.Q, d.Q, d.T, d.T)
	res, err := d.client.NewRequest().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("cookie", cookie).
		SetQueryParams(map[string]string{
			"mid":       d.MID,
			"m2":        d.MID,
			"ver":       "1.0.1182.0",
			"timestamp": fmt.Sprintf("%d", time.Now().UnixMilli()),
			"from":      "7",
		}).
		SetFormData(map[string]string{
			"d": string(data),
		}).Post(path)
	if err != nil {
		return nil, err
	}
	if res.IsError() {
		return nil, fmt.Errorf("[ZBrowser] API error: %s, body: %s", res.Status(), res.String())
	}
	if resp != nil {
		var base baseResp
		if err := json.Unmarshal(res.Body(), &base); err != nil {
			return nil, fmt.Errorf("[ZBrowser] API base response unmarshal error: %w", err)
		}
		if base.Code != 0 {
			return nil, fmt.Errorf("[ZBrowser] API error: %s", base.Msg)
		}
		if err := json.Unmarshal(res.Body(), resp); err != nil {
			return nil, fmt.Errorf("[ZBrowser] API response unmarshal error: %w", err)
		}
	}
	return res, nil
}

func (d *ZBrowser) xuRequest(ctx context.Context, path string, value any, file model.FileStreamer, up driver.UpdateProgress, resp any) (*resty.Response, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] xuAPI request marshal error: %w", err)
	}
	encData, err := Encrypt(data)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] xuAPI request encrypt error: %w", err)
	}
	query := url.Values{
		"from": []string{"6"},
		"ver":  []string{"1.0.1182.0"},
		"mid":  []string{d.MID},
		"m2":   []string{d.MID},
	}
	cookie := fmt.Sprintf("Q=%s; T=%s;", d.Q, d.T)
	headers := http.Header{
		"Accept":     []string{"*/*"},
		"Cookie":     []string{cookie},
		"Host":       []string{"xu.zbrowser.cn"},
		"User-Agent": []string{"curl/8.11.0-DEV"},
	}

	var rd io.Reader
	if file == nil {
		rd = strings.NewReader(url.Values{
			"d": []string{string(encData)},
		}.Encode())
		headers.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		var b bytes.Buffer
		// for upload, use multipart form
		writer := multipart.NewWriter(&b)
		if err := writer.WriteField("d", string(encData)); err != nil {
			return nil, fmt.Errorf("[ZBrowser] xuAPI request write field error: %w", err)
		}
		// part, err := writer.CreateFormFile("file", file.GetName())
		// if err != nil {
		// 	return nil, fmt.Errorf("[ZBrowser] xuAPI request create form file error: %w", err)
		// }
		// if _, err := io.Copy(part, file); err != nil {
		// 	return nil, fmt.Errorf("[ZBrowser] xuAPI request copy file error: %w", err)
		// }
		headSize := b.Len()
		head := bytes.NewReader(b.Bytes()[:headSize])
		tail := bytes.NewReader(b.Bytes()[headSize:])

		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("[ZBrowser] xuAPI request close writer error: %w", err)
		}
		rd = driver.NewLimitedUploadStream(ctx, io.MultiReader(head, file, tail))
		headers.Set("Content-Type", writer.FormDataContentType())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://xu.zbrowser.cn"+path+"?"+query.Encode(), rd)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] xuAPI request error: %w", err)
	}
	req.Header = headers

	res, err := base.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] xuAPI request error: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("[ZBrowser] xuAPI error: %s", res.Status)
	}
	if resp != nil {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("[ZBrowser] xuAPI response read error: %w", err)
		}
		decBody, err := Decrypt(string(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("[ZBrowser] xuAPI response decrypt error: %w", err)
		}
		var base baseResp
		if err := json.Unmarshal([]byte(decBody), &base); err != nil {
			return nil, fmt.Errorf("[ZBrowser] xuAPI base response decode error: %w", err)
		}
		if base.Code != 0 {
			return nil, fmt.Errorf("[ZBrowser] xuAPI error: %s", base.Msg)
		}
	}
	return nil, nil
}
