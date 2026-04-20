package guangyapan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

const (
	accountBaseURL = "https://account.guangyapan.com"
	apiBaseURL     = "https://api.guangyapan.com"
	defaultClient  = "aMe-8VSlkrbQXpUR"
)

type GuangYaPan struct {
	model.Storage
	Addition

	accountClient *resty.Client
	apiClient     *resty.Client
}

func (d *GuangYaPan) Config() driver.Config {
	return config
}

func (d *GuangYaPan) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *GuangYaPan) Init(ctx context.Context) error {
	d.ClientID = strings.TrimSpace(d.ClientID)
	if d.ClientID == "" {
		d.ClientID = defaultClient
	}
	d.DeviceID = normalizeDeviceID(d.DeviceID)
	if d.DeviceID == "" {
		d.DeviceID = randomDeviceID()
	}
	if d.PageSize <= 0 {
		d.PageSize = 100
	}
	if d.OrderBy < 0 {
		d.OrderBy = 3
	}
	if d.SortType != 0 && d.SortType != 1 {
		d.SortType = 1
	}

	d.AccessToken = strings.TrimSpace(d.AccessToken)
	d.RefreshToken = strings.TrimSpace(d.RefreshToken)
	d.PhoneNumber = strings.TrimSpace(d.PhoneNumber)
	d.VerifyCode = strings.TrimSpace(d.VerifyCode)
	d.CaptchaToken = strings.TrimSpace(d.CaptchaToken)
	d.VerificationID = strings.TrimSpace(d.VerificationID)

	d.accountClient = base.NewRestyClient().
		SetBaseURL(accountBaseURL).
		SetHeader("Accept", "application/json, text/plain, */*").
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Device-Model", "chrome%2F147.0.0.0").
		SetHeader("X-Device-Name", "PC-Chrome").
		SetHeader("X-Device-Sign", "wdi10."+d.DeviceID+"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx").
		SetHeader("X-Net-Work-Type", "NONE").
		SetHeader("X-OS-Version", "MacIntel").
		SetHeader("X-Platform-Version", "1").
		SetHeader("X-Protocol-Version", "301").
		SetHeader("X-Provider-Name", "NONE").
		SetHeader("X-SDK-Version", "9.0.2").
		SetHeader("X-Client-Id", d.ClientID).
		SetHeader("X-Client-Version", "0.0.1").
		SetHeader("X-Device-Id", d.DeviceID)
	if d.CaptchaToken != "" {
		d.accountClient.SetHeader("X-Captcha-Token", d.CaptchaToken)
	}

	d.apiClient = base.NewRestyClient().
		SetBaseURL(apiBaseURL).
		SetHeader("Accept", "application/json, text/plain, */*").
		SetHeader("Content-Type", "application/json").
		SetHeader("Did", d.DeviceID).
		SetHeader("Dt", "4")

	// Priority: access_token -> refresh_token -> sms login.
	if d.AccessToken != "" {
		if err := d.validateToken(ctx); err == nil {
			return nil
		}
		d.AccessToken = ""
	}
	if d.RefreshToken != "" {
		if err := d.refreshToken(ctx); err == nil {
			if err2 := d.validateToken(ctx); err2 == nil {
				return nil
			}
		}
	}
	// Two-stage SMS flow:
	// 1) phone only + send_code=true: send code and cache verification_id (do not fail init).
	// 2) phone + verify_code: complete login and save tokens.
	if d.PhoneNumber != "" {
		if d.canSMSLogin() {
			if err := d.loginBySMSCode(ctx); err != nil {
				return err
			}
			return d.validateToken(ctx)
		}
		if d.SendCode {
			d.setTempStatus("SMS sending in progress...")
			if err := d.prepareSMSCode(ctx); err != nil {
				d.setTempStatus(fmt.Sprintf("SMS send failed: %v. Please check captcha/meta and set send_code=true to retry.", err))
				log.Warnf("guangyapan: prepare sms code failed: %v", err)
			} else {
				d.setTempStatus("SMS sent successfully. Please fill verify_code and save to complete login.")
			}
		}
		return nil
	}
	return errors.New("login failed: provide a valid access_token, or refresh_token, or phone_number + verify_code + captcha_token")
}

func (d *GuangYaPan) Drop(ctx context.Context) error {
	return nil
}

func (d *GuangYaPan) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	if err := d.ensureAccessToken(ctx); err != nil {
		return nil, err
	}

	parentID := dir.GetID()
	if parentID == d.RootFolderID {
		parentID = ""
	}

	res := make([]model.Obj, 0, d.PageSize)
	for page := 0; ; page++ {
		var resp listResp
		body := map[string]any{
			"parentId":  parentID,
			"page":      page,
			"pageSize":  d.PageSize,
			"orderBy":   d.OrderBy,
			"sortType":  d.SortType,
			"fileTypes": []int{},
		}
		if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/file/get_file_list", body, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp.Data.List {
			res = append(res, &model.Object{
				ID:       item.FileID,
				Path:     parentID,
				Name:     item.FileName,
				Size:     item.FileSize,
				Modified: unixOrZero(item.UTime),
				Ctime:    unixOrZero(item.CTime),
				IsFolder: item.ResType == 2,
			})
		}
		if len(resp.Data.List) < d.PageSize {
			break
		}
		if resp.Data.Total > 0 && len(res) >= resp.Data.Total {
			break
		}
	}
	return res, nil
}

func (d *GuangYaPan) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if file.IsDir() {
		return nil, errs.NotFile
	}
	if err := d.ensureAccessToken(ctx); err != nil {
		return nil, err
	}

	var resp downloadResp
	if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/get_res_download_url", map[string]any{
		"fileId": file.GetID(),
	}, &resp); err != nil {
		return nil, err
	}

	url := strings.TrimSpace(resp.Data.SignedURL)
	if url == "" {
		url = strings.TrimSpace(resp.Data.DownloadURL)
	}
	if url == "" {
		return nil, errors.New("empty download url")
	}
	return &model.Link{URL: url}, nil
}

func (d *GuangYaPan) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	if err := d.ensureAccessToken(ctx); err != nil {
		return err
	}

	name := strings.TrimSpace(dirName)
	if name == "" {
		return errors.New("dir name is empty")
	}

	parentID := parentDir.GetID()
	if parentID == d.RootFolderID {
		parentID = ""
	}

	var out createDirResp
	if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/file/create_dir", map[string]any{
		"parentId": parentID,
		"dirName":  name,
	}, &out); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(out.Msg), "success") {
		return fmt.Errorf("make dir failed: %s", strings.TrimSpace(out.Msg))
	}
	return nil
}

func (d *GuangYaPan) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	if err := d.ensureAccessToken(ctx); err != nil {
		return err
	}

	fileID := strings.TrimSpace(srcObj.GetID())
	if fileID == "" {
		return errors.New("file id is empty")
	}
	name := strings.TrimSpace(newName)
	if name == "" {
		return errors.New("new name is empty")
	}

	var out commonResp
	if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/file/rename", map[string]any{
		"fileId":  fileID,
		"newName": name,
	}, &out); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(out.Msg), "success") {
		return fmt.Errorf("rename failed: %s", strings.TrimSpace(out.Msg))
	}
	return nil
}

func (d *GuangYaPan) Remove(ctx context.Context, obj model.Obj) error {
	if err := d.ensureAccessToken(ctx); err != nil {
		return err
	}

	fileID := strings.TrimSpace(obj.GetID())
	if fileID == "" {
		return errors.New("file id is empty")
	}

	var del deleteResp
	if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/file/delete_file", map[string]any{
		"fileIds": []string{fileID},
	}, &del); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(del.Msg), "success") {
		return fmt.Errorf("delete failed: %s", strings.TrimSpace(del.Msg))
	}

	taskID := strings.TrimSpace(del.Data.TaskID)
	if taskID == "" {
		// Some backends may apply deletion synchronously.
		return nil
	}
	return d.waitTaskDone(ctx, taskID)
}

func (d *GuangYaPan) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	if err := d.ensureAccessToken(ctx); err != nil {
		return err
	}

	fileID := strings.TrimSpace(srcObj.GetID())
	if fileID == "" {
		return errors.New("file id is empty")
	}
	parentID := dstDir.GetID()
	if parentID == d.RootFolderID {
		parentID = ""
	}

	var out deleteResp
	if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/file/move_file", map[string]any{
		"fileIds":  []string{fileID},
		"parentId": parentID,
	}, &out); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(out.Msg), "success") {
		return fmt.Errorf("move failed: %s", strings.TrimSpace(out.Msg))
	}
	taskID := strings.TrimSpace(out.Data.TaskID)
	if taskID == "" {
		return nil
	}
	return d.waitTaskDone(ctx, taskID)
}

func (d *GuangYaPan) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	if err := d.ensureAccessToken(ctx); err != nil {
		return err
	}

	fileID := strings.TrimSpace(srcObj.GetID())
	if fileID == "" {
		return errors.New("file id is empty")
	}
	parentID := dstDir.GetID()
	if parentID == d.RootFolderID {
		parentID = ""
	}

	var out deleteResp
	if err := d.postAPI(ctx, "/nd.bizuserres.s/v1/file/copy_file", map[string]any{
		"fileIds":  []string{fileID},
		"parentId": parentID,
	}, &out); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(out.Msg), "success") {
		return fmt.Errorf("copy failed: %s", strings.TrimSpace(out.Msg))
	}
	taskID := strings.TrimSpace(out.Data.TaskID)
	if taskID == "" {
		return nil
	}
	return d.waitTaskDone(ctx, taskID)
}

func (d *GuangYaPan) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) error {
	if err := d.ensureAccessToken(ctx); err != nil {
		return err
	}
	if file == nil {
		return errors.New("file is nil")
	}
	if file.GetSize() < 0 {
		return errors.New("invalid file size")
	}
	name := strings.TrimSpace(file.GetName())
	if name == "" {
		return errors.New("file name is empty")
	}

	parentID := dstDir.GetID()
	if parentID == d.RootFolderID {
		parentID = ""
	}

	token, code, err := d.getUploadToken(ctx, parentID, name, file.GetSize())
	if err != nil {
		return err
	}
	taskID := strings.TrimSpace(token.TaskID)
	if code == 156 {
		if taskID == "" {
			return errors.New("instant upload returns empty task id")
		}
		return d.waitUploadTaskInfo(ctx, taskID)
	}

	if token.ObjectPath == "" || token.BucketName == "" || token.EndPoint == "" || token.AccessKeyID == "" || token.SecretAccessKey == "" {
		return errors.New("upload token is incomplete")
	}

	ossEndpoint := normalizeOSSEndpoint(token.EndPoint, token.BucketName)
	client, err := oss.New(ossEndpoint, token.AccessKeyID, token.SecretAccessKey, oss.SecurityToken(token.SessionToken))
	if err != nil {
		return fmt.Errorf("create oss client failed: %w", err)
	}
	bucket, err := client.Bucket(token.BucketName)
	if err != nil {
		return fmt.Errorf("create oss bucket failed: %w", err)
	}

	if file.GetSize() == 0 {
		if err := bucket.PutObject(token.ObjectPath, strings.NewReader("")); err != nil {
			return err
		}
	} else {
		if err := d.multipartUploadToOSS(ctx, bucket, token.ObjectPath, file, up); err != nil {
			return err
		}
	}

	if taskID == "" {
		return nil
	}
	return d.waitUploadTaskInfo(ctx, taskID)
}

var _ driver.Driver = (*GuangYaPan)(nil)
