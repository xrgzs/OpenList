package seewo_pinco

import (
	"context"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/cookie"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/go-resty/resty/v2"
)

type SeewoPinco struct {
	model.Storage
	Addition
	client *resty.Client
}

func (d *SeewoPinco) Config() driver.Config {
	return config
}

func (d *SeewoPinco) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *SeewoPinco) Init(ctx context.Context) error {
	// TODO login / refresh token
	//op.MustSaveDriverStorage(d)
	d.client = base.NewRestyClient()
	d.client.SetCookieJar(nil)
	c := cookie.Parse(d.Cookie)
	d.client.SetCookies(c)
	return nil
}

func (d *SeewoPinco) Drop(ctx context.Context) error {
	d.Cookie = cookie.ToString(d.client.Cookies)
	return nil
}

func (d *SeewoPinco) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	var r GetV1DriveMaterialsResp
	var c []Content
	var page int = 0
	for {
		err := d.api(ctx, "GetV1DriveMaterials", base.Json{
			"keyword":  "",
			"size":     50,
			"tagName":  "resource",
			"page":     page,
			"folderId": dir.GetID(),
		}, &r)
		if err != nil {
			return nil, err
		}
		c = append(c, r.Data.Content...)
		page++
		if r.Data.Last {
			break
		}
	}
	return utils.SliceConvert(c, func(src Content) (model.Obj, error) {
		return contentToObj(src), nil
	})

}

func (d *SeewoPinco) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if obj, ok := file.(*model.ObjThumbURL); ok {
		return &model.Link{
			URL: obj.URL(),
		}, nil
	}
	return nil, errs.NotImplement
}

func (d *SeewoPinco) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	return d.api(ctx, "PostV1DriveMaterialsFolders", base.Json{
		"name":           dirName,
		"parentFolderId": parentDir.GetID(),
	}, nil)
}

func (d *SeewoPinco) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	return d.api(ctx, "PutV1DriveMaterialsLocations", base.Json{
		"resIdList": []string{srcObj.GetID()},
	}, nil)
}

func (d *SeewoPinco) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	return d.api(ctx, "PutV1DriveMaterialsByMaterialIdName", base.Json{
		"materialId": srcObj.GetID(),
		"name":       newName,
	}, nil)
}

func (d *SeewoPinco) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	// TODO copy obj, optional
	return nil, errs.NotImplement
}

func (d *SeewoPinco) Remove(ctx context.Context, obj model.Obj) error {
	return d.api(ctx, "PutV1DriveMaterialsLocations", base.Json{
		"resIdList":      []string{obj.GetID()},
		"targetFolderId": obj.GetID(),
	}, nil)
}

func (d *SeewoPinco) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	// TODO upload file, optional
	return nil, errs.NotImplement
}

func (d *SeewoPinco) GetDetails(ctx context.Context) (*model.StorageDetails, error) {
	var r GetV1DriveMaterialsCapacityResp
	err := d.api(ctx, "GetV1DriveMaterialsCapacity", base.Json{
		"type": 1,
	}, &r)
	if err != nil {
		return nil, err
	}
	return &model.StorageDetails{
		DiskUsage: driver.DiskUsageFromUsedAndTotal(r.Data.Used, r.Data.Capacity),
	}, nil
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*SeewoPinco)(nil)
