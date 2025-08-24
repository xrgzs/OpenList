package dsm_share

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/cookie"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/go-resty/resty/v2"
)

type DsmShare struct {
	model.Storage
	Addition
	client *resty.Client
}

func (d *DsmShare) Config() driver.Config {
	return config
}

func (d *DsmShare) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *DsmShare) Init(ctx context.Context) error {
	var err error
	d.client = base.NewRestyClient()
	err = d.getCookie()
	if err != nil {
		return err
	}
	if d.GetRootPath() == "" || d.GetRootPath() == "/" {
		return d.getInitData()
	}
	return nil
}

func (d *DsmShare) Drop(ctx context.Context) error {
	return nil
}

func (d *DsmShare) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	var resp ListResp
	_, err := d.request(http.MethodPost, EntryCgi, func(req *resty.Request) {
		req.SetFormData(map[string]string{
			"api":            "SYNO.FolderSharing.List",
			"method":         "list",
			"version":        "2",
			"offset":         "0",
			"limit":          "1000",
			"sort_by":        strconv.Quote("name"),
			"sort_direction": strconv.Quote("ASC"),
			"action":         strconv.Quote("enum"),
			"additional":     `["size","owner","time","perm","type","mount_point_type"]`,
			"filetype":       strconv.Quote("all"),
			"folder_path":    strconv.Quote(dir.GetPath()),
			"_sharing_id":    strconv.Quote(d.ShareID),
		})
	}, &resp)
	if err != nil {
		return nil, err
	}
	return utils.SliceConvert(resp.Data.Files, func(s File) (model.Obj, error) {
		return &model.Object{
			Path:     s.Path,
			Name:     s.Name,
			Size:     s.Additional.Size,
			Modified: time.Unix(s.Additional.Time.Mtime, 0),
			Ctime:    time.Unix(s.Additional.Time.Ctime, 0),
			IsFolder: s.Isdir,
		}, nil
	})

}

func (d *DsmShare) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	baseURL, err := url.Parse(d.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse address %s: %w", d.Address, err)
	}

	downloadURL := baseURL.JoinPath(DownloadCGI, url.QueryEscape(file.GetName()))

	params := url.Values{
		"dlink":       {strconv.Quote(hex.EncodeToString([]byte(file.GetPath())))},
		"noCache":     {strconv.FormatInt(time.Now().UnixMilli(), 10)},
		"_sharing_id": {strconv.Quote(d.ShareID)},
		"api":         {"SYNO.FolderSharing.Download"},
		"version":     {"2"},
		"method":      {"download"},
		"mode":        {"download"},
		"stdhtml":     {"false"},
	}
	downloadURL.RawQuery = params.Encode()

	cookies := d.client.GetClient().Jar.Cookies(&url.URL{Scheme: baseURL.Scheme, Host: baseURL.Host})

	return &model.Link{
		URL: downloadURL.String(),
		Header: http.Header{
			"Cookie":     {cookie.ToString(cookies)},
			"User-Agent": {d.client.Header.Get("User-Agent")},
		},
	}, nil
}

func (d *DsmShare) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *DsmShare) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *DsmShare) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *DsmShare) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *DsmShare) Remove(ctx context.Context, obj model.Obj) error {
	return errs.NotImplement
}

func (d *DsmShare) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *DsmShare) GetArchiveMeta(ctx context.Context, obj model.Obj, args model.ArchiveArgs) (model.ArchiveMeta, error) {
	return nil, errs.NotImplement
}

func (d *DsmShare) ListArchive(ctx context.Context, obj model.Obj, args model.ArchiveInnerArgs) ([]model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *DsmShare) Extract(ctx context.Context, obj model.Obj, args model.ArchiveInnerArgs) (*model.Link, error) {
	return nil, errs.NotImplement
}

func (d *DsmShare) ArchiveDecompress(ctx context.Context, srcObj, dstDir model.Obj, args model.ArchiveDecompressArgs) ([]model.Obj, error) {
	return nil, errs.NotImplement
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*DsmShare)(nil)
