package vk

import (
	"context"
	"fmt"
	"regexp"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/go-resty/resty/v2"
)

type VK struct {
	model.Storage
	Addition

	client *resty.Client
}

func (d *VK) Config() driver.Config {
	return config
}

func (d *VK) GetAddition() driver.Additional {
	return &d.Addition
}

func (Addition) GetRootPath() string {
	return "/"
}

func (d *VK) Init(ctx context.Context) error {
	// TODO login / refresh token
	//op.MustSaveDriverStorage(d)
	d.client = base.NewRestyClient()
	return nil
}

func (d *VK) Drop(ctx context.Context) error {
	return nil
}

func (d *VK) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	var resp DocGetResp
	_, err := d.client.R().
		SetFormData(map[string]string{
			"count":        "50",
			"offset":       "0",
			"access_token": d.Addition.AccessToken,
		}).
		SetResult(&resp).
		Post("https://api.vk.com/method/docs.get?v=5.269")
	if err != nil {
		return nil, err
	}
	var results []model.Obj
	for _, f := range resp.Response.Items {
		results = append(results, &Object{
			Object: model.Object{
				Name: f.Title,
				Size: f.Size,
			},
			URL: f.URL,
		})
	}
	return results, nil
}

func (d *VK) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if o, ok := file.(*Object); ok {
		res, err := d.client.R().
			Get(o.URL)
		if err != nil {
			return nil, err
		}
		re := regexp.MustCompile(`href="(https://psv4\.userapi\.com[^"]*)"`)
		matches := re.FindStringSubmatch(res.String())
		if len(matches) < 2 {
			return nil, fmt.Errorf("URL not found")
		}
		fmt.Printf("Link is %s", matches[1])
		return &model.Link{
			URL: matches[1],
		}, nil
	}
	return nil, errs.NotImplement
}

func (d *VK) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	// TODO create folder, optional
	return nil, errs.NotImplement
}

func (d *VK) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	// TODO move obj, optional
	return nil, errs.NotImplement
}

func (d *VK) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	// TODO rename obj, optional
	return nil, errs.NotImplement
}

func (d *VK) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	// TODO copy obj, optional
	return nil, errs.NotImplement
}

func (d *VK) Remove(ctx context.Context, obj model.Obj) error {
	// TODO remove obj, optional
	return errs.NotImplement
}

func (d *VK) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	// TODO upload file, optional
	return nil, errs.NotImplement
}

var _ driver.Driver = (*VK)(nil)
