package geegeng

import (
	"context"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

type Geegeng struct {
	model.Storage
	Addition
}

func (d *Geegeng) Config() driver.Config {
	return config
}

func (d *Geegeng) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Geegeng) Init(ctx context.Context) error {
	// TODO login / refresh token
	//op.MustSaveDriverStorage(d)
	return nil
}

func (d *Geegeng) Drop(ctx context.Context) error {
	return nil
}

func (d *Geegeng) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	// TODO return the files list, required
	return nil, errs.NotImplement
}

func (d *Geegeng) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	// TODO return link of file, required
	return nil, errs.NotImplement
}

func (d *Geegeng) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	// TODO create folder, optional
	return nil, errs.NotImplement
}

func (d *Geegeng) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	// TODO move obj, optional
	return nil, errs.NotImplement
}

func (d *Geegeng) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	// TODO rename obj, optional
	return nil, errs.NotImplement
}

func (d *Geegeng) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	// TODO copy obj, optional
	return nil, errs.NotImplement
}

func (d *Geegeng) Remove(ctx context.Context, obj model.Obj) error {
	// TODO remove obj, optional
	return errs.NotImplement
}

func (d *Geegeng) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	// TODO upload file, optional
	return nil, errs.NotImplement
}

func (d *Geegeng) GetDetails(ctx context.Context) (*model.StorageDetails, error) {
	// TODO return storage details (total space, free space, etc.)
	return nil, errs.NotImplement
}

var _ driver.Driver = (*Geegeng)(nil)
