package dsm_share

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/go-resty/resty/v2"
)

// do others that not defined in Driver interface

const (
	EntryCgi    = "/sharing/webapi/entry.cgi"
	DownloadCGI = "/fsdownload/webapi/file_download.cgi"
)

func (d *DsmShare) request(method string, path string, callback base.ReqCallback, resp any) ([]byte, error) {
	req := d.client.R()
	if callback != nil {
		callback(req)
	}
	if resp != nil {
		req.SetResult(resp)
	}
	res, err := req.Execute(method, d.Address+path)
	if err != nil {
		return nil, err
	}
	body := res.Body()
	if !utils.Json.Get(body, "success").ToBool() {
		return nil, errors.New(utils.Json.Get(body, "error").ToString())
	}
	return body, nil
}

func (d *DsmShare) getCookie() error {
	// get cookie by login
	if d.Password != "" {
		// var resp LoginResp
		_, err := d.request(http.MethodPost, EntryCgi+"/SYNO.Core.Sharing.Login", func(req *resty.Request) {
			req.SetFormData(map[string]string{
				"api":        "SYNO.Core.Sharing.Login",
				"method":     "login",
				"version":    "1",
				"sharing_id": strconv.Quote(d.ShareID),
				"password":   strconv.Quote(d.Password),
			})
		}, nil)
		if err != nil {
			return err
		}
		// "sharing_sid=" + resp.Data.SharingSid
		return nil
	}
	// get cookie by visit share page
	res, err := d.client.R().Get(d.Address + "/sharing/" + d.ShareID)
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		return errors.New("failed to get share page: " + res.Status())
	}
	c := res.Header().Get("Set-Cookie")
	if c == "" {
		return errors.New("no cookie found")
	}
	return nil
}

func (d *DsmShare) getInitData() error {
	var resp InitDataResp
	_, err := d.request(http.MethodPost, EntryCgi, func(req *resty.Request) {
		req.SetFormData(map[string]string{
			"api":     "SYNO.Core.Sharing.Initdata",
			"method":  "get",
			"version": "1",
		})
		req.SetHeader("X-Syno-Sharing", d.ShareID)
	}, &resp)
	if err != nil {
		return err
	}
	if resp.Data.Private.Filename == "" {
		return errors.New("no filename found in init data, you must set root path to non-root path")
	}
	d.SetRootPath("/" + resp.Data.Private.Filename)
	op.MustSaveDriverStorage(d)
	return nil
}
