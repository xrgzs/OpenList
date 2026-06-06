package zbrowser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

func (d *ZBrowser) xuRequest(ctx context.Context, path string, value any, file io.Reader, fileName string, up driver.UpdateProgress, resp any) (*http.Response, error) {
	data, err := xuAPIMarshal(value)
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
		"m2":   []string{""},
	}
	cookie := fmt.Sprintf("Q=%s;T=%s", d.Q, d.T)
	headers := http.Header{
		"Accept":     []string{"*/*"},
		"Cookie":     []string{cookie},
		"Host":       []string{"xu.zbrowser.cn"},
		"User-Agent": []string{"curl/8.11.0-DEV"},
	}

	boundary := fmt.Sprintf("------------------------%026d", time.Now().UnixNano()%1000000000000000)
	var rd io.Reader
	if file == nil && fileName == "" {
		// pIndex: form-urlencoded
		rd = strings.NewReader(url.Values{
			"d": []string{string(encData)},
		}.Encode())
		headers.Set("Content-Type", "application/x-www-form-urlencoded")
	} else if file != nil {
		// upload: multipart with file content (手动拼接，与 DLL 输出完全一致)
		var b bytes.Buffer
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Disposition: form-data; name=\"d\"\r\n\r\n")
		b.WriteString(string(encData))
		b.WriteString("\r\n--" + boundary + "\r\n")
		b.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"f\"; filename=\"%s\"\r\n\r\n", fileName))
		if _, err := io.Copy(&b, file); err != nil {
			return nil, fmt.Errorf("[ZBrowser] xuAPI request copy file error: %w", err)
		}
		b.WriteString("\r\n--" + boundary + "--\r\n")

		// 调试：保存完整请求到文件
		tmpReq, _ := os.CreateTemp("", "zbrowser-req-*.txt")
		fmt.Fprintf(tmpReq, "POST http://xu.zbrowser.cn%s?%s HTTP/1.1\r\n", path, query.Encode())
		for k, vv := range headers {
			for _, v := range vv {
				fmt.Fprintf(tmpReq, "%s: %s\r\n", k, v)
			}
		}
		fmt.Fprintf(tmpReq, "Content-Length: %d\r\n\r\n", b.Len())
		tmpReq.Write(b.Bytes())
		tmpReq.Close()
		fmt.Printf("[ZBrowser-debug] upload request saved to: %s\n", tmpReq.Name())

		rd = bytes.NewReader(b.Bytes())
		headers.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	} else {
		// confirm: multipart with empty f field
		var b bytes.Buffer
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Disposition: form-data; name=\"d\"\r\n\r\n")
		b.WriteString(string(encData))
		b.WriteString("\r\n--" + boundary + "\r\n")
		b.WriteString("Content-Disposition: form-data; name=\"f\"\r\n\r\n\r\n")
		b.WriteString("--" + boundary + "--\r\n")

		rd = bytes.NewReader(b.Bytes())
		headers.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://xu.zbrowser.cn"+path+"?"+query.Encode(), rd)
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
		return nil, fmt.Errorf("[ZBrowser] xuAPI HTTP error: %s, url: %s", res.Status, req.URL.String())
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] xuAPI response read error: %w", err)
	}
	decBody, err := Decrypt(string(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] xuAPI response decrypt error: %w", err)
	}
	if resp != nil {
		if err := json.Unmarshal([]byte(decBody), resp); err != nil {
			return nil, fmt.Errorf("[ZBrowser] xuAPI response decode error: %w, body: %s, url: %s", err, decBody, res.Request.URL.String())
		}
	}
	return res, nil
}

func objType(obj model.Obj) int {
	if obj.IsDir() {
		return 1
	}
	return 2
}

// xuAPIValue 构造 xuAPI 请求的通用 JSON 结构
func (d *ZBrowser) xuAPIValue(param map[string]any) map[string]any {
	return map[string]any{
		"Q":     "",
		"T":     "",
		"m2":    "",
		"mid":   d.MID,
		"param": param,
		"t":     time.Now().Unix(),
	}
}

// xuAPIMarshal 将 xuAPI 请求值序列化为 JSON（缩进3格+CRLF，与浏览器 DLL 一致）
func xuAPIMarshal(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "   ")
	if err != nil {
		return nil, err
	}
	s := string(data)
	// Go 会把非 ASCII 字符转义成 \uXXXX，DLL 用原始 UTF-8，需要还原
	s = unescapeUnicode(s)
	// DLL 使用 \r\n 换行，且末尾也有 \r\n
	s = strings.ReplaceAll(s, "\n", "\r\n")
	if !strings.HasSuffix(s, "\r\n") {
		s += "\r\n"
	}
	return []byte(s), nil
}

// unescapeUnicode 将 JSON 中的 \uXXXX 转义还原为原始 UTF-8 字符
func unescapeUnicode(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if i+5 < len(s) && s[i] == '\\' && s[i+1] == 'u' {
			var r rune
			for j := 2; j < 6; j++ {
				c := s[i+j]
				switch {
				case c >= '0' && c <= '9':
					r = r*16 + rune(c-'0')
				case c >= 'a' && c <= 'f':
					r = r*16 + rune(c-'a'+10)
				case c >= 'A' && c <= 'F':
					r = r*16 + rune(c-'A'+10)
				default:
					r = -1
				}
			}
			if r >= 0 {
				result.WriteRune(r)
				i += 5
				continue
			}
		}
		result.WriteByte(s[i])
	}
	return result.String()
}
