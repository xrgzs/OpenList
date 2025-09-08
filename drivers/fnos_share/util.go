package fnos_share

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils/random"
	"github.com/go-resty/resty/v2"
)

// do others that not defined in Driver interface

func (d *FnOsShare) request(method string, path string, callback base.ReqCallback, resp any) ([]byte, error) {
	u, err := url.Parse(d.Address + path)
	if err != nil {
		return nil, err
	}
	req := base.RestyClient.R()

	if callback != nil {
		callback(req)
	}

	req.SetHeader("Cookie", d.Cookie)
	d.setAuthx(u.Path, req)

	if resp != nil {
		req.SetResult(resp)
	}
	res, err := req.Execute(method, u.String())
	if err != nil {
		return nil, err
	}
	body := res.Body()
	if utils.Json.Get(body, "code").ToInt() != 0 {
		return nil, errors.New(utils.Json.Get(body, "msg").ToString())
	}
	return body, nil
}

var shareDataReg = regexp.MustCompile(`<script id="share-data" type="application/json">(.+?)</script>`)

func (d *FnOsShare) getShareData() error {
	req := base.RestyClient.R()
	data, err := req.Get(d.Address)
	if err != nil {
		return err
	}
	if data.StatusCode() != 200 {
		return fmt.Errorf("failed to get share page: %s", data.Status())
	}
	shareData := shareDataReg.FindSubmatch(data.Body())
	if len(shareData) < 2 {
		return errors.New("failed to parse share data")
	}
	var resp ShareDataResp
	err = utils.Json.Unmarshal(shareData[1], &resp)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		// 3000008: Need Password
		if resp.Code == 3000008 {
			_, err = d.request(http.MethodPost, "/api/v1/share/pwd", func(req *resty.Request) {
				req.SetBody(PwdReq{
					ShareID: d.ShareID,
					Passwd:  utils.HashData(utils.SHA256, []byte(d.Password)),
				})
			}, &resp)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("%d: %s", resp.Code, resp.Msg)
		}
	}
	d.Token = resp.Data.Token
	d.Cookie = fmt.Sprintf("%s=%s", d.ShareID, d.Token)
	return nil
}

const apiKey = "814&d6470861a4cfbbb4fe2fd3f$6581f6"

func (d *FnOsShare) setAuthx(path string, req *resty.Request) {
	var body string
	rb := req.Body
	if rb != nil {
		switch v := rb.(type) {
		case string:
			body = v
		case []byte:
			body = string(v)
		default:
			jsonBytes, err := utils.Json.Marshal(v)
			if err != nil {
				body = ""
			} else {
				body = string(jsonBytes)
			}
		}
	}

	ts := time.Now().UnixMilli()
	tss := strconv.FormatInt(ts, 10)
	nonce := strconv.FormatInt(random.RangeInt64(1e5, 1e6-1), 10)
	toSign := []string{
		"NDzZTVxnRKP8Z0jXg1VAMonaG8akvh",
		path,
		nonce,
		tss,
		hashSignatureData(body),
		apiKey,
	}
	sign := utils.HashData(utils.MD5, []byte(strings.Join(toSign, "_")))
	authx := url.Values{}
	authx.Set("nonce", nonce)
	authx.Set("timestamp", tss)
	authx.Set("sign", sign)
	req.SetHeader("Authx", authx.Encode())
}

func hashSignatureData(s string) string {
	var (
		dataToHash string
		err        error
	)
	dataToHash, err = url.PathUnescape(s)
	if err != nil {
		dataToHash = s
	}
	return utils.HashData(utils.MD5, []byte(dataToHash))
}

func (d *FnOsShare) setShareID() error {
	parts := strings.Split(d.Address, "/")
	if len(parts) > 0 {
		d.ShareID = parts[len(parts)-1]
		return nil
	}
	return fmt.Errorf("failed to get share id")
}
