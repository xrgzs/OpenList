package guangyapan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/go-resty/resty/v2"
)

func (d *GuangYaPan) ensureAccessToken(ctx context.Context) error {
	if strings.TrimSpace(d.AccessToken) != "" {
		return nil
	}
	if strings.TrimSpace(d.RefreshToken) == "" {
		if d.canSMSLogin() {
			return d.loginBySMSCode(ctx)
		}
		if d.PhoneNumber != "" {
			return errors.New("not logged in yet: please fill verify_code and save storage to finish SMS login")
		}
		return errors.New("access token is empty")
	}
	return d.refreshToken(ctx)
}

func (d *GuangYaPan) validateToken(ctx context.Context) error {
	var me userMeResp
	resp, err := d.accountClient.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+d.AccessToken).
		SetResult(&me).
		Get("/v1/user/me")
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("validate token failed: status=%d body=%s", resp.StatusCode(), resp.String())
	}
	if strings.TrimSpace(me.Sub) == "" {
		return errors.New("validate token failed: empty user sub")
	}
	return nil
}

func (d *GuangYaPan) refreshToken(ctx context.Context) error {
	if strings.TrimSpace(d.RefreshToken) == "" {
		return errors.New("refresh_token is empty")
	}

	var out tokenResp
	resp, err := d.accountClient.R().
		SetContext(ctx).
		SetBody(map[string]any{
			"client_id":     d.ClientID,
			"grant_type":    "refresh_token",
			"refresh_token": d.RefreshToken,
		}).
		SetResult(&out).
		Post("/v1/auth/token")
	if err != nil {
		return err
	}
	if resp.IsError() || out.Error != "" || strings.TrimSpace(out.AccessToken) == "" {
		errMsg := strings.TrimSpace(out.ErrorDesc)
		if errMsg == "" {
			errMsg = strings.TrimSpace(out.Error)
		}
		if errMsg == "" {
			errMsg = strings.TrimSpace(resp.String())
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("status=%d", resp.StatusCode())
		}
		return fmt.Errorf("refresh token failed: %s", errMsg)
	}

	d.AccessToken = strings.TrimSpace(out.AccessToken)
	if strings.TrimSpace(out.RefreshToken) != "" {
		d.RefreshToken = strings.TrimSpace(out.RefreshToken)
	}
	op.MustSaveDriverStorage(d)
	return nil
}

func (d *GuangYaPan) canSMSLogin() bool {
	return d.PhoneNumber != "" && d.VerifyCode != ""
}

func (d *GuangYaPan) loginBySMSCode(ctx context.Context) error {
	verificationID := strings.TrimSpace(d.VerificationID)
	if verificationID == "" {
		var err error
		verificationID, err = d.requestVerificationID(ctx)
		if err != nil {
			return err
		}
	}

	var step2 verifyResp
	resp, err := d.accountClient.R().
		SetContext(ctx).
		SetBody(map[string]any{
			"verification_id":   verificationID,
			"verification_code": d.VerifyCode,
			"client_id":         d.ClientID,
		}).
		SetResult(&step2).
		Post("/v1/auth/verification/verify")
	if err != nil {
		return err
	}
	if resp.IsError() || step2.Error != "" || strings.TrimSpace(step2.VerificationToken) == "" {
		return fmt.Errorf("verify code failed: %s", d.accountErr(step2.ErrorDesc, step2.Error, resp))
	}

	var out tokenResp
	resp, err = d.accountClient.R().
		SetContext(ctx).
		SetBody(map[string]any{
			"verification_code":  d.VerifyCode,
			"verification_token": step2.VerificationToken,
			"username":           normalizePhoneE164(d.PhoneNumber),
			"client_id":          d.ClientID,
		}).
		SetResult(&out).
		Post("/v1/auth/signin")
	if err != nil {
		return err
	}
	if resp.IsError() || out.Error != "" || strings.TrimSpace(out.AccessToken) == "" {
		return fmt.Errorf("signin failed: %s", d.accountErr(out.ErrorDesc, out.Error, resp))
	}

	d.AccessToken = strings.TrimSpace(out.AccessToken)
	d.RefreshToken = strings.TrimSpace(out.RefreshToken)
	d.VerificationID = ""
	// One-time SMS code should not be reused after successful login.
	d.VerifyCode = ""
	op.MustSaveDriverStorage(d)
	return nil
}

func (d *GuangYaPan) prepareSMSCode(ctx context.Context) error {
	// Explicit send action should always refresh verification_id.
	d.VerificationID = ""
	if err := d.ensureCaptchaToken(ctx, false); err != nil {
		return err
	}
	verificationID, err := d.requestVerificationID(ctx)
	if err != nil {
		return err
	}
	d.VerificationID = verificationID
	d.SendCode = false
	op.MustSaveDriverStorage(d)
	return nil
}

func (d *GuangYaPan) requestVerificationID(ctx context.Context) (string, error) {
	if d.CaptchaToken != "" {
		d.accountClient.SetHeader("X-Captcha-Token", d.CaptchaToken)
	}

	var step1 verificationResp
	resp, err := d.accountClient.R().
		SetContext(ctx).
		SetBody(map[string]any{
			"phone_number": normalizePhoneE164(d.PhoneNumber),
			"target":       "ANY",
			"client_id":    d.ClientID,
		}).
		SetResult(&step1).
		Post("/v1/auth/verification")
	if err != nil {
		return "", err
	}
	if resp.IsError() || step1.Error != "" || strings.TrimSpace(step1.VerificationID) == "" {
		// If captcha token is expired/invalid, refresh it once and retry.
		if strings.Contains(step1.Error, "captcha_invalid") || strings.Contains(step1.ErrorDesc, "captcha_token expired") {
			if err := d.ensureCaptchaToken(ctx, true); err == nil {
				return d.requestVerificationID(ctx)
			}
		}
		return "", fmt.Errorf("request verification failed: %s", d.accountErr(step1.ErrorDesc, step1.Error, resp))
	}
	return strings.TrimSpace(step1.VerificationID), nil
}

func (d *GuangYaPan) ensureCaptchaToken(ctx context.Context, force bool) error {
	if !force && d.CaptchaToken != "" {
		d.accountClient.SetHeader("X-Captcha-Token", d.CaptchaToken)
		return nil
	}

	var out captchaInitResp
	resp, err := d.accountClient.R().
		SetContext(ctx).
		SetBody(map[string]any{
			"client_id": d.ClientID,
			"action":    "POST:/v1/auth/verification",
			"device_id": d.DeviceID,
			"meta": map[string]any{
				"username":           normalizePhoneE164(d.PhoneNumber),
				"phone_number":       normalizePhoneE164(d.PhoneNumber),
				"VERIFICATION_PHONE": normalizePhoneE164(d.PhoneNumber),
			},
		}).
		SetResult(&out).
		Post("/v1/shield/captcha/init")
	if err != nil {
		return err
	}
	if resp.IsError() || out.Error != "" || strings.TrimSpace(out.CaptchaToken) == "" {
		return fmt.Errorf("init captcha token failed: %s", d.accountErr(out.ErrorDesc, out.Error, resp))
	}
	d.CaptchaToken = strings.TrimSpace(out.CaptchaToken)
	d.accountClient.SetHeader("X-Captcha-Token", d.CaptchaToken)
	op.MustSaveDriverStorage(d)
	return nil
}

func normalizeCaptchaUsername(phone string) string {
	p := strings.TrimSpace(phone)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.TrimPrefix(p, "+")
	// Keep only digits.
	b := make([]rune, 0, len(p))
	for _, ch := range p {
		if ch >= '0' && ch <= '9' {
			b = append(b, ch)
		}
	}
	digits := string(b)
	// Mainland number normalization: +86xxxxxxxxxxx -> xxxxxxxxxxx
	if strings.HasPrefix(digits, "86") && len(digits) > 11 {
		digits = digits[2:]
	}
	return digits
}

func normalizePhoneE164(phone string) string {
	p := strings.TrimSpace(phone)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, " ", "")
	if strings.HasPrefix(p, "+") {
		// Format as "+86 1xxxxxxxxxx" to match browser payload expectations.
		if strings.HasPrefix(p, "+86") && len(p) > 3 {
			rest := strings.TrimPrefix(p, "+86")
			return "+86 " + rest
		}
		return p
	}
	// If raw mainland number is provided, normalize with +86 prefix.
	digits := normalizeCaptchaUsername(p)
	if len(digits) == 11 {
		return "+86 " + digits
	}
	return p
}

func (d *GuangYaPan) setTempStatus(status string) {
	// initStorage sets status to WORK after Init returns, so we update it shortly after.
	time.AfterFunc(200*time.Millisecond, func() {
		d.GetStorage().SetStatus(status)
		op.MustSaveDriverStorage(d)
	})
}

func (d *GuangYaPan) accountErr(desc, short string, resp *resty.Response) string {
	msg := strings.TrimSpace(desc)
	if msg == "" {
		msg = strings.TrimSpace(short)
	}
	if msg == "" && resp != nil {
		msg = strings.TrimSpace(resp.String())
	}
	if msg == "" && resp != nil {
		msg = fmt.Sprintf("status=%d", resp.StatusCode())
	}
	if msg == "" {
		msg = "unknown error"
	}
	return msg
}

func (d *GuangYaPan) postAPI(ctx context.Context, path string, body any, out any) error {
	if strings.TrimSpace(d.AccessToken) == "" {
		return errors.New("access token is empty")
	}
	resp, err := d.apiClient.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+d.AccessToken).
		SetBody(body).
		SetResult(out).
		Post(path)
	if err != nil {
		return err
	}
	if resp.StatusCode() == 401 || resp.StatusCode() == 403 {
		if strings.TrimSpace(d.RefreshToken) == "" {
			return fmt.Errorf("request failed: status=%d body=%s", resp.StatusCode(), resp.String())
		}
		if err := d.refreshToken(ctx); err != nil {
			return err
		}
		resp, err = d.apiClient.R().
			SetContext(ctx).
			SetHeader("Authorization", "Bearer "+d.AccessToken).
			SetBody(body).
			SetResult(out).
			Post(path)
		if err != nil {
			return err
		}
	}
	if resp.IsError() {
		return fmt.Errorf("request failed: status=%d body=%s", resp.StatusCode(), resp.String())
	}
	return nil
}

func (d *GuangYaPan) waitTaskDone(ctx context.Context, taskID string) error {
	const (
		maxTry   = 30
		interval = 300 * time.Millisecond
	)
	for i := 0; i < maxTry; i++ {
		var out taskStatusResp
		if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/get_task_status", map[string]any{
			"taskId": taskID,
		}, &out); err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(out.Msg), "success") {
			return fmt.Errorf("get task status failed: %s", strings.TrimSpace(out.Msg))
		}
		switch out.Data.Status {
		case 2:
			return nil
		case -1, 3:
			return fmt.Errorf("task %s failed with status=%d", taskID, out.Data.Status)
		}
		if i == maxTry-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("task %s timeout", taskID)
}

func (d *GuangYaPan) getUploadToken(ctx context.Context, parentID, name string, size int64) (*uploadTokenData, int, error) {
	var out uploadTokenResp
	err := d.postAPI(ctx, "/nd.bizuserres.s/v1/get_res_center_token", map[string]any{
		"capacity": 2,
		"name":     name,
		"parentId": parentID,
		"res": map[string]any{
			"fileSize": size,
		},
	}, &out)
	if err != nil {
		return nil, 0, err
	}
	msg := strings.TrimSpace(out.Msg)
	if msg != "" && !strings.EqualFold(msg, "success") {
		return nil, out.Code, fmt.Errorf("get upload token failed: %s", msg)
	}
	if out.Data.TaskID == "" {
		return nil, out.Code, errors.New("get upload token failed: empty task id")
	}
	if out.Data.AccessKeyID == "" {
		out.Data.AccessKeyID = out.Data.Creds.AccessKeyID
	}
	if out.Data.SecretAccessKey == "" {
		out.Data.SecretAccessKey = out.Data.Creds.SecretAccessKey
	}
	if out.Data.SessionToken == "" {
		out.Data.SessionToken = out.Data.Creds.SessionToken
	}
	if strings.TrimSpace(out.Data.EndPoint) == "" {
		out.Data.EndPoint = strings.TrimSpace(out.Data.FullEndPoint)
	}
	if strings.TrimSpace(out.Data.EndPoint) != "" && !strings.HasPrefix(out.Data.EndPoint, "http://") && !strings.HasPrefix(out.Data.EndPoint, "https://") {
		if strings.TrimSpace(out.Data.FullEndPoint) != "" {
			out.Data.EndPoint = strings.TrimSpace(out.Data.FullEndPoint)
		} else if strings.TrimSpace(out.Data.BucketName) != "" {
			host := strings.TrimSpace(out.Data.EndPoint)
			prefix := strings.TrimSpace(out.Data.BucketName) + "."
			if strings.HasPrefix(host, prefix) {
				out.Data.EndPoint = "https://" + host
			} else {
				out.Data.EndPoint = "https://" + strings.TrimSpace(out.Data.BucketName) + "." + host
			}
		} else {
			out.Data.EndPoint = "https://" + strings.TrimSpace(out.Data.EndPoint)
		}
	}
	return &out.Data, out.Code, nil
}

func (d *GuangYaPan) waitUploadTaskInfo(ctx context.Context, taskID string) error {
	const (
		maxTry   = 300
		interval = 1 * time.Second
	)
	for i := 0; i < maxTry; i++ {
		var out taskInfoResp
		if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/file/get_info_by_task_id", map[string]any{
			"taskId": taskID,
		}, &out); err != nil {
			return err
		}
		if out.Data.FileID != "" {
			return nil
		}
		switch out.Code {
		case 145, 146, 147, 155, 163, 0:
			// uploading/verifying/processing
		default:
			if strings.TrimSpace(out.Msg) != "" {
				return fmt.Errorf("upload task failed: code=%d msg=%s", out.Code, strings.TrimSpace(out.Msg))
			}
		}
		if i == maxTry-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("upload task %s timeout", taskID)
}

func (d *GuangYaPan) multipartUploadToOSS(ctx context.Context, bucket *oss.Bucket, objectPath string, file model.FileStreamer, up driver.UpdateProgress) error {
	partSize := calcUploadPartSize(file.GetSize())
	imur, err := bucket.InitiateMultipartUpload(objectPath, oss.Sequential())
	if err != nil {
		return err
	}

	total := file.GetSize()
	partCount := int((total + partSize - 1) / partSize)
	parts := make([]oss.UploadPart, 0, partCount)
	var uploaded int64
	partNumber := 1

	for uploaded < total {
		if err := ctx.Err(); err != nil {
			return err
		}
		curPartSize := partSize
		left := total - uploaded
		if left < curPartSize {
			curPartSize = left
		}

		reader := io.LimitReader(file, curPartSize)
		part, err := bucket.UploadPart(imur, driver.NewLimitedUploadStream(ctx, reader), curPartSize, partNumber)
		if err != nil {
			return err
		}
		parts = append(parts, part)
		uploaded += curPartSize
		partNumber++
		if total > 0 {
			up(100 * float64(uploaded) / float64(total))
		}
	}

	_, err = bucket.CompleteMultipartUpload(imur, parts)
	return err
}

func calcUploadPartSize(size int64) int64 {
	const (
		mb = int64(1024 * 1024)
		gb = int64(1024 * 1024 * 1024)
	)
	switch {
	case size <= 100*mb:
		return 1 * mb
	case size <= 16*gb:
		return 2 * mb
	case size <= 160*gb:
		return 4 * mb
	default:
		return 8 * mb
	}
}

func normalizeOSSEndpoint(endpoint, bucket string) string {
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		return ep
	}
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "https://" + ep
	}
	u, err := url.Parse(ep)
	if err != nil || u.Host == "" {
		return ep
	}
	host := u.Host
	prefix := strings.TrimSpace(bucket)
	if prefix != "" && strings.HasPrefix(host, prefix+".") {
		host = strings.TrimPrefix(host, prefix+".")
	}
	u.Host = host
	return u.String()
}

func normalizeDeviceID(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "-", "")
	if len(v) != 32 {
		return ""
	}
	for _, ch := range v {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return v
}

func randomDeviceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "0123456789abcdef0123456789abcdef"
	}
	return hex.EncodeToString(b)
}
