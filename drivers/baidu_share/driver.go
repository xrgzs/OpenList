package baidu_share

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/baidu_netdisk"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/go-resty/resty/v2"
)

type BaiduShare struct {
	model.Storage
	Addition
	client *resty.Client
	info   struct {
		Root    string
		Seckey  string
		Shareid string
		Uk      string
	}
	ref *baidu_netdisk.BaiduNetdisk
}

func (d *BaiduShare) Config() driver.Config {
	return config
}

func (d *BaiduShare) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *BaiduShare) InitReference(storage driver.Driver) error {
	refStorage, ok := storage.(*baidu_netdisk.BaiduNetdisk)
	if ok {
		d.ref = refStorage
		return nil
	}
	return fmt.Errorf("ref: storage is not BaiduNetdisk")
}

func (d *BaiduShare) Init(ctx context.Context) error {
	// TODO login / refresh token
	//op.MustSaveDriverStorage(d)
	d.client = resty.New().
		SetBaseURL("https://pan.baidu.com").
		SetHeader("User-Agent", "netdisk").
		SetHeader("Referer", "https://pan.baidu.com").
		SetCookie(&http.Cookie{Name: "BDUSS", Value: d.BDUSS}).
		SetCookie(&http.Cookie{Name: "ndut_fmt"})
	respJson := struct {
		Errno int64 `json:"errno"`
		Data  struct {
			List [1]struct {
				Path string `json:"path"`
			} `json:"list"`
			Uk      json.Number `json:"uk"`
			Shareid json.Number `json:"shareid"`
			Seckey  string      `json:"seckey"`
		} `json:"data"`
	}{}
	resp, err := d.client.R().
		SetBody(url.Values{
			"pwd":      {d.Pwd},
			"root":     {"1"},
			"shorturl": {d.Surl},
		}.Encode()).
		SetResult(&respJson).
		Post("share/wxlist?channel=weixin&version=2.2.2&clienttype=25&web=1")
	if err == nil {
		if resp.IsSuccess() && respJson.Errno == 0 {
			d.info.Root = path.Dir(respJson.Data.List[0].Path)
			d.info.Seckey = respJson.Data.Seckey
			d.info.Shareid = respJson.Data.Shareid.String()
			d.info.Uk = respJson.Data.Uk.String()
		} else {
			err = fmt.Errorf(" %s; %s; ", resp.Status(), resp.Body())
		}
	}
	return err
}

func (d *BaiduShare) Drop(ctx context.Context) error {
	return nil
}

func (d *BaiduShare) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	// TODO return the files list, required
	reqDir := dir.GetPath()
	isRoot := "0"
	if reqDir == d.RootFolderPath {
		reqDir = path.Join(d.info.Root, reqDir)
	}
	if reqDir == d.info.Root {
		isRoot = "1"
	}
	objs := []model.Obj{}
	var err error
	var page uint64 = 1
	more := true
	for more && err == nil {
		respJson := struct {
			Errno int64 `json:"errno"`
			Data  struct {
				More bool `json:"has_more"`
				List []struct {
					Fsid  json.Number `json:"fs_id"`
					Isdir json.Number `json:"isdir"`
					Path  string      `json:"path"`
					Name  string      `json:"server_filename"`
					Mtime json.Number `json:"server_mtime"`
					Size  json.Number `json:"size"`
				} `json:"list"`
			} `json:"data"`
		}{}
		resp, e := d.client.R().
			SetBody(url.Values{
				"dir":      {reqDir},
				"num":      {"1000"},
				"order":    {"time"},
				"page":     {fmt.Sprint(page)},
				"pwd":      {d.Pwd},
				"root":     {isRoot},
				"shorturl": {d.Surl},
			}.Encode()).
			SetResult(&respJson).
			Post("share/wxlist?channel=weixin&version=2.2.2&clienttype=25&web=1")
		err = e
		if err == nil {
			if resp.IsSuccess() && respJson.Errno == 0 {
				page++
				more = respJson.Data.More
				for _, v := range respJson.Data.List {
					size, _ := v.Size.Int64()
					mtime, _ := v.Mtime.Int64()
					objs = append(objs, &model.Object{
						ID:       v.Fsid.String(),
						Path:     v.Path,
						Name:     v.Name,
						Size:     size,
						Modified: time.Unix(mtime, 0),
						IsFolder: v.Isdir.String() == "1",
					})
				}
			} else {
				err = fmt.Errorf(" %s; %s; ", resp.Status(), resp.Body())
			}
		}
	}
	return objs, err
}

func (d *BaiduShare) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if d.ref == nil {
		return nil, fmt.Errorf("no reference")
	}
	// 1. 转存
	transferJson := struct {
		Errno int64 `json:"errno"`
		Extra struct {
			List []struct {
				From     string `json:"from"`
				FromFsID int64  `json:"from_fs_id"`
				To       string `json:"to"`
				ToFsID   int64  `json:"to_fs_id"`
			} `json:"list"`
		} `json:"extra"`
		Info []struct {
			Errno int64  `json:"errno"`
			Fsid  int64  `json:"fsid"`
			Path  string `json:"path"`
		} `json:"info"`
		Newno     string `json:"newno"`
		RequestID int64  `json:"request_id"`
		ShowMsg   string `json:"show_msg"`
		TaskID    int64  `json:"task_id"`
	}{}
	resp, err := d.client.R().
		SetQueryParams(map[string]string{
			"shareid":    d.info.Shareid,
			"from":       d.info.Uk,
			"sekey":      d.info.Seckey,
			"ondup":      "newcopy",
			"async":      "1",
			"channel":    "chunlei",
			"web":        "1",
			"app_id":     "250528",
			"clienttype": "0",
		}).
		SetFormData(map[string]string{
			"fsidlist": fmt.Sprintf("[%s]", file.GetID()),
			"path":     "/",
		}).
		SetResult(&transferJson).
		Post("share/transfer")
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() || transferJson.Errno != 0 {
		return nil, fmt.Errorf(" %s; %s; ", resp.Status(), resp.Body())
	}

	// 2. 获取转存后到下载链接
	if len(transferJson.Extra.List) == 0 {
		return nil, fmt.Errorf("transfer response missing Extra.List")
	}
	obj, ok := file.(*model.Object)
	if !ok {
		return nil, fmt.Errorf("file is not *model.Object")
	}
	obj.ID = fmt.Sprint(transferJson.Extra.List[0].ToFsID)
	obj.Path = transferJson.Extra.List[0].To

	return d.ref.Link(ctx, obj, args)
}

func (d *BaiduShare) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	// TODO create folder, optional
	return errs.NotSupport
}

func (d *BaiduShare) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	// TODO move obj, optional
	return errs.NotSupport
}

func (d *BaiduShare) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	// TODO rename obj, optional
	return errs.NotSupport
}

func (d *BaiduShare) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	// TODO copy obj, optional
	return errs.NotSupport
}

func (d *BaiduShare) Remove(ctx context.Context, obj model.Obj) error {
	// TODO remove obj, optional
	return errs.NotSupport
}

func (d *BaiduShare) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) error {
	// TODO upload file, optional
	return errs.NotSupport
}

var _ driver.Driver = (*BaiduShare)(nil)
