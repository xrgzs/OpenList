package zbrowser

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/go-resty/resty/v2"
)

type ZBrowser struct {
	model.Storage
	Addition

	client *resty.Client
}

func (d *ZBrowser) Config() driver.Config {
	return config
}

func (d *ZBrowser) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *ZBrowser) Init(ctx context.Context) error {
	// TODO login / refresh token
	//op.MustSaveDriverStorage(d)
	d.client = base.NewRestyClient().
		SetBaseURL("https://pan.zbrowser.cn").
		SetHeaders(map[string]string{
			"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.6834.83 Safari/537.36 ZEROBROWSER",
			"Connection":         "keep-alive",
			"Accept":             "application/json, text/plain, */*",
			"Accept-Encoding":    "gzip, deflate, br, zstd",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-ch-ua":          "\"Not A(Brand\";v=\"8\", \"Chromium\";v=\"132\", \"Google Chrome\";v=\"132\"",
			"sec-ch-ua-mobile":   "?0",
			"origin":             "chrome://cloud-drive",
			"sec-fetch-site":     "cross-site",
			"sec-fetch-mode":     "cors",
			"sec-fetch-dest":     "empty",
			"accept-language":    "zh-CN,zh;q=0.9",
			"priority":           "u=1, i",
		})
	return nil
}

func (d *ZBrowser) Drop(ctx context.Context) error {
	d.client = nil
	return nil
}

func (d *ZBrowser) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	var fileList []model.Obj

	// fetch directories
	var dirResp dirListRespV1
	_, err := d.apiRequest(ctx, "/v1/dir/list", base.Json{
		"pid":  dir.GetID(),
		"ver":  0,
		"sort": 0,
		"desc": 1,
	}, &dirResp)
	if err != nil {
		return nil, err
	}
	dirObjs, err := dirResp.toObj()
	if err != nil {
		return nil, err
	}
	fileList = append(fileList, dirObjs...)

	// fetch files with pagination
	next := "0"
	for {
		var fileResp fileListRespV1
		_, err := d.apiRequest(ctx, "/v1/file/list", base.Json{
			"pid":  dir.GetID(),
			"ver":  0,
			"oft":  next,
			"num":  50,
			"sort": 0,
			"desc": 1,
		}, &fileResp)
		if err != nil {
			return nil, err
		}

		objs, err := fileResp.toObj()
		if err != nil {
			return nil, err
		}
		fileList = append(fileList, objs...)

		if fileResp.Data.HasMore == 0 || fileResp.Data.Next == next {
			break
		}
		next = fileResp.Data.Next
	}

	return fileList, nil
}

func (d *ZBrowser) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	pid := "0"
	if obj, ok := file.(*Obj); ok {
		pid = obj.ParentID
	}
	var dl fileDlRespV3
	_, err := d.apiRequest(ctx, "/v3/file/dl", base.Json{
		"pid": pid,
		"ver": 0,
		"items": []base.Json{
			{"type": 2, "id": file.GetID()},
		},
	}, &dl)
	if err != nil {
		return nil, err
	}
	if len(dl.Data.List.Files) == 0 {
		return nil, fmt.Errorf("[ZBrowser] no download links returned")
	}
	referer := dl.Data.List.Files[0].URL
	if idx := strings.LastIndex(referer, "/"); idx >= 0 {
		referer = referer[:idx+1]
	}
	return &model.Link{
		URL: dl.Data.List.Files[0].URL,
		Header: http.Header{
			"Sec-Fetch-Site":  []string{"same-origin"},
			"Sec-Fetch-Mode":  []string{"navigate"},
			"Sec-Fetch-Dest":  []string{"empty"},
			"Referer":         []string{referer},
			"User-Agent":      []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.6834.83 Safari/537.36 ZEROBROWSER"},
			"Accept-Language": []string{"zh-CN,zh;q=0.9"},
			"Cookie":          []string{"__guid=" + d.GUID + "; Q=" + d.Q + "; __NS_Q=" + d.Q + "; T=" + d.T + "; __NS_T=" + d.T},
		},
	}, nil
}

func (d *ZBrowser) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	var new dirNewRespV3
	_, err := d.apiRequest(ctx, "/v3/dir/new", base.Json{
		"pid":  parentDir.GetID(),
		"ver":  0,
		"name": dirName,
	}, &new)
	return err
}

func (d *ZBrowser) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	src, ok := srcObj.(*Obj)
	if !ok {
		return fmt.Errorf("srcObj is not a zbrowser obj")
	}
	var resp baseResp
	_, err := d.apiRequest(ctx, "/v3/file/move", base.Json{
		"pid":    src.ParentID,
		"ver":    0,
		"newPid": dstDir.GetID(),
		"items": []base.Json{
			{
				"type": func() int {
					if srcObj.IsDir() {
						return 1
					}
					return 2
				}(),
				"id": srcObj.GetID(),
			},
		},
	}, &resp)
	return err
}

func (d *ZBrowser) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	var resp baseResp
	_, err := d.apiRequest(ctx, "/v3/file/rename", base.Json{
		"id":       srcObj.GetID(),
		"fromName": srcObj.GetName(),
		"toName":   newName,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &model.Object{
		ID:       srcObj.GetID(),
		Name:     newName,
		Size:     srcObj.GetSize(),
		Modified: srcObj.ModTime(),
		IsFolder: srcObj.IsDir(),
	}, nil
}

func (d *ZBrowser) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	src, ok := srcObj.(*Obj)
	if !ok {
		return nil, fmt.Errorf("srcObj is not a zbrowser obj")
	}
	var resp baseResp
	_, err := d.apiRequest(ctx, "/v3/dir/copy", base.Json{
		"pid":    src.ParentID,
		"ver":    0,
		"newPid": dstDir.GetID(),
		"items": []base.Json{
			{
				"type": func() int {
					if srcObj.IsDir() {
						return 1
					}
					return 2
				}(),
				"id": srcObj.GetID(),
			},
		},
	}, &resp)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (d *ZBrowser) Remove(ctx context.Context, obj model.Obj) error {
	src, ok := obj.(*Obj)
	if !ok {
		return fmt.Errorf("obj is not a zbrowser obj")
	}
	path := "/v3/recycle/move"
	if d.DeleteMode == "delete" {
		path = "/v3/recycle/del"
	}
	var resp baseResp
	_, err := d.apiRequest(ctx, path, base.Json{
		"pid": src.ParentID,
		"ver": 0,
		"items": []base.Json{
			{
				"type": func() int {
					if obj.IsDir() {
						return 1
					}
					return 2
				}(),
				"id": obj.GetID(),
			},
		},
	}, &resp)
	return err
}

func (d *ZBrowser) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	// TODO upload file, optional
	return nil, errs.NotImplement
}

func (d *ZBrowser) GetDetails(ctx context.Context) (*model.StorageDetails, error) {
	var space userSpaceResp
	_, err := d.apiRequest(ctx, "/v1/user/space", nil, &space)
	if err != nil {
		return nil, err
	}
	return &model.StorageDetails{
		DiskUsage: model.DiskUsage{
			TotalSpace: space.Data.Total,
			UsedSpace:  space.Data.Used,
		},
	}, nil
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*ZBrowser)(nil)
