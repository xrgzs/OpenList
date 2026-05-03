package geegeng

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
)

// do others that not defined in Driver interface

func (d *Geegeng) getUA() string {
	if d.CustomUA != "" {
		return d.CustomUA
	}
	return base.UserAgent
}

func (d *Geegeng) getUserInfo() (UserInfo, error) {
	var resp GetUserInfoResp
	res, err := d.client.R().
		SetResult(&resp).
		Post("/user/getUserInfo")
	if err != nil {
		return UserInfo{}, err
	}
	if !res.IsSuccess() {
		return UserInfo{}, fmt.Errorf("getUserInfo request failed with status code: %d", res.StatusCode())
	}
	if resp.Code != 0 {
		return UserInfo{}, fmt.Errorf("getUserInfo error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return resp.Data.UserInfo, nil
}

const tenMB = 10 * 1024 * 1024 // 10485760

func getChunkSize(fileSize int64) int64 {
	// 文件大小大于 10MB * 2 * 999 ≈ 19.98 GB
	if fileSize > tenMB*2*999 {
		s := math.Ceil(float64(fileSize) / 1999 / float64(tenMB))
		if s < 5 {
			s = 5
		}
		return int64(s) * tenMB
	}
	// 文件大小大于 10MB * 999 ≈ 9.99 GB 且 ≤ 19.98 GB
	if fileSize > tenMB*999 {
		return tenMB * 2
	}
	// 普通文件（≤ 9.99 GB）
	return tenMB
}

// FileDigest 包含整个文件的 MD5 和 salt
type FileDigest struct {
	FullMD5   string   // 整个文件的 MD5
	Salt      string   // 各分块 MD5 拼接后计算的 MD5
	ChunkMD5s []string // 每个分块的 MD5（十六进制）
}

// computeFileDigest 读取文件，计算 fullMD5 和 salt
// r: 文件流（需要支持 Seek 或允许读取多次，内部会创建 TeeReader 以便同时计算分块 MD5）
// size: 文件总大小
func computeFileDigest(r io.Reader, size int64) (*FileDigest, error) {
	chunkSize := getChunkSize(size)
	chunkCount := int((size + chunkSize - 1) / chunkSize)

	// 用于计算整体 MD5
	fullHasher := md5.New()
	// 存储每个分块的 MD5 字符串
	chunkMD5s := make([]string, 0, chunkCount)

	// 循环读取文件分块
	for i := 0; i < chunkCount; i++ {
		// 当前分块的大小（最后一块可能小于 chunkSize）
		currentChunkSize := chunkSize
		if i == chunkCount-1 {
			currentChunkSize = size - int64(i)*chunkSize
		}

		// 限制读取长度
		chunkReader := io.LimitReader(r, currentChunkSize)
		// 计算分块 MD5
		chunkHasher := md5.New()
		// 同时写入 fullHasher 和 chunkHasher
		tee := io.TeeReader(chunkReader, chunkHasher)

		if _, err := io.Copy(fullHasher, tee); err != nil {
			return nil, err
		}

		chunkMD5 := hex.EncodeToString(chunkHasher.Sum(nil))
		chunkMD5s = append(chunkMD5s, chunkMD5)
	}

	fullMD5 := hex.EncodeToString(fullHasher.Sum(nil))

	// 将所有分块 MD5 按行拼接（与前端一致：用换行符 '\n' 连接）
	concatenated := strings.Join(chunkMD5s, "\n")
	saltHasher := md5.Sum([]byte(concatenated))
	salt := hex.EncodeToString(saltHasher[:])

	return &FileDigest{
		FullMD5:   fullMD5,
		Salt:      salt,
		ChunkMD5s: chunkMD5s,
	}, nil
}

func (d *Geegeng) getNeedUploadParts(ctx context.Context, uploadBase, fileName, uploadId string, chunkCount int) []int {
	var resp GetUploadedPartsInfoResp
	if err := d.httpGetJSON(ctx, uploadBase+"/getUploadedPartsInfo", map[string]string{
		"fileName": fileName,
		"uploadId": uploadId,
		"t":        strconv.FormatInt(time.Now().UnixMilli(), 10),
	}, &resp); err != nil || resp.Code != 1 {
		all := make([]int, chunkCount)
		for i := range all {
			all[i] = i + 1
		}
		return all
	}

	uploaded := resp.Data.UploadedParts
	if strings.TrimSpace(uploaded) == "" {
		all := make([]int, chunkCount)
		for i := range all {
			all[i] = i + 1
		}
		return all
	}

	uploadedSet := make(map[int]struct{})
	for _, s := range strings.Split(uploaded, ",") {
		s = strings.TrimSpace(s)
		if n, err := strconv.Atoi(s); err == nil {
			uploadedSet[n] = struct{}{}
		}
	}

	var need []int
	for i := 1; i <= chunkCount; i++ {
		if _, ok := uploadedSet[i]; !ok {
			need = append(need, i)
		}
	}
	return need
}

func (d *Geegeng) commitMultiUploadFile(ctx context.Context, uploadBase, fileName, uploadId string) error {
	var resp BaseResp
	if err := d.httpPostForm(ctx, uploadBase+"/commitMultiUploadFile", map[string]string{
		"fileName": fileName,
		"uploadId": uploadId,
	}, &resp); err != nil {
		return fmt.Errorf("commitMultiUploadFile failed: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("commitMultiUploadFile error: code %d, message: %s", resp.Code, resp.Msg)
	}
	return nil
}

// httpPostForm POST form-encoded data and decode JSON response
func (d *Geegeng) httpPostForm(ctx context.Context, rawURL string, form map[string]string, result any) error {
	vals := url.Values{}
	for k, v := range form {
		vals.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(vals.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", d.getUA())
	req.Header.Set("Origin", "https://www.geegeng.com")
	req.Header.Set("Referer", "https://www.geegeng.com/")
	resp, err := base.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

// httpGetJSON GET with query params and decode JSON response
func (d *Geegeng) httpGetJSON(ctx context.Context, rawURL string, params map[string]string, result any) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", d.getUA())
	req.Header.Set("Origin", "https://www.geegeng.com")
	req.Header.Set("Referer", "https://www.geegeng.com/")
	resp, err := base.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

// httpPut PUT with custom headers and body, caller must close resp.Body
func (d *Geegeng) httpPut(ctx context.Context, rawURL string, headers map[string]string, body io.Reader, contentLength int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, rawURL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.ContentLength = contentLength
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", d.getUA())
	req.Header.Set("Origin", "https://www.geegeng.com")
	req.Header.Set("Referer", "https://www.geegeng.com/")
	return base.HttpClient.Do(req)
}

func parseHeaderString(headerStr string) map[string]string {
	result := make(map[string]string)
	if headerStr == "" {
		return result
	}
	for _, pair := range strings.Split(headerStr, "&") {
		if idx := strings.Index(pair, "="); idx != -1 {
			key := pair[:idx]
			value := pair[idx+1:]
			result[key] = value
		}
	}
	return result
}
