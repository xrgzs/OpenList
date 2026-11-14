package fnos_share

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/go-resty/resty/v2"
)

type FnOsShare struct {
	model.Storage
	Addition
	Token   string
	ShareID string
	Cookie  string
}

func (d *FnOsShare) Config() driver.Config {
	return config
}

func (d *FnOsShare) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *FnOsShare) Init(ctx context.Context) error {
	var err error
	err = d.setShareID()
	if err != nil {
		return err
	}
	err = d.getShareData()
	if err != nil {
		return err
	}
	return nil
}

func (d *FnOsShare) Drop(ctx context.Context) error {
	return nil
}

func (d *FnOsShare) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	var id *int
	var err error
	intID, err := strconv.Atoi(dir.GetID())
	if err != nil {
		id = nil
	} else {
		id = &intID
	}
	var resp ListResp
	_, err = d.request(http.MethodPost, "/api/v1/share/list", func(req *resty.Request) {
		req.SetBody(ListReq{
			ShareID: d.ShareID,
			Path:    dir.GetPath(),
			FileID:  id,
		})
		req.SetHeader("Auth", d.Token)
	}, &resp)
	if err != nil {
		return nil, err
	}
	return utils.SliceConvert(resp.Data.Files, func(s FileInfo) (model.Obj, error) {
		return &model.Object{
			ID:       strconv.Itoa(s.FileID),
			Name:     s.File,
			Size:     s.Size,
			Modified: time.Unix(s.ModTime, 0),
			IsFolder: s.IsDir == 1,
		}, nil
	})
}

func (d *FnOsShare) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	id, err := strconv.Atoi(file.GetID())
	if err != nil {
		return nil, err
	}
	var resp DownloadResp
	_, err = d.request(http.MethodPost, "/api/v1/share/download", func(req *resty.Request) {
		req.SetBody(DownloadReq{
			Files: []DownloadFile{
				{
					Path:   file.GetPath(),
					FileID: id,
				},
			},
			ShareID:          d.ShareID,
			DownloadFilename: file.GetName(),
		})
		req.SetHeader("Auth", d.Token)
	}, &resp)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(d.Address)
	if err != nil {
		return nil, err
	}
	u.Path = ""
	if d.CustomHost != "" {
		u.Host = d.CustomHost
	}
	return &model.Link{
		URL: u.String() + resp.Data.Path,
		Header: http.Header{
			"Cookie":     {d.Cookie},
			"Referer":    {d.Address},
			"User-Agent": {base.UserAgent},
		},
	}, nil
}

func (d *FnOsShare) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *FnOsShare) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *FnOsShare) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *FnOsShare) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *FnOsShare) Remove(ctx context.Context, obj model.Obj) error {
	return errs.NotImplement
}

func (d *FnOsShare) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *FnOsShare) GetArchiveMeta(ctx context.Context, obj model.Obj, args model.ArchiveArgs) (model.ArchiveMeta, error) {
	return nil, errs.NotImplement
}

func (d *FnOsShare) ListArchive(ctx context.Context, obj model.Obj, args model.ArchiveInnerArgs) ([]model.Obj, error) {
	return nil, errs.NotImplement
}

func (d *FnOsShare) Extract(ctx context.Context, obj model.Obj, args model.ArchiveInnerArgs) (*model.Link, error) {
	return nil, errs.NotImplement
}

func (d *FnOsShare) ArchiveDecompress(ctx context.Context, srcObj, dstDir model.Obj, args model.ArchiveDecompressArgs) ([]model.Obj, error) {
	return nil, errs.NotImplement
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*FnOsShare)(nil)
