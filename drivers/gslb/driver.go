package gslb

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/fs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/sign"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"gopkg.in/yaml.v3"
)

type Gslb struct {
	model.Storage
	Addition
	storages []GslbStorage
}

func (d *Gslb) Config() driver.Config {
	return config
}

func (d *Gslb) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Gslb) Init(ctx context.Context) error {
	// 解析paths
	if d.Addition.Paths == "" {
		return fmt.Errorf("invalid gslb paths")
	}
	err := yaml.Unmarshal([]byte(d.Addition.Paths), &d.storages)
	if err != nil {
		return err
	}
	if !db.GetIPDB().HasData() {
		// return fmt.Errorf("ipdb is not initialized")
		d.SetStatus("ipdb is not initialized")
	}
	return nil
}

func (d *Gslb) Drop(ctx context.Context) error {
	d.storages = nil
	return nil
}

func (d *Gslb) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	// 如果有 Ref 标记的存储节点，则只访问这些节点
	hasRef := false
	for _, storage := range d.storages {
		if storage.Ref {
			hasRef = true
			break
		}
	}

	var objs []model.Obj
	for _, s := range d.storages {
		if hasRef && !s.Ref {
			continue
		}
		rp := path.Join(s.Path, dir.GetPath())
		o, err := func() ([]model.Obj, error) { // 为defer使用闭包
			var ctxChild context.Context
			if d.Timeout > 0 && !s.Ref {
				var cancel context.CancelFunc
				ctxChild, cancel = context.WithTimeout(ctx, time.Duration(d.Addition.Timeout)*time.Second)
				defer cancel()
			} else {
				ctxChild = ctx
			}
			return fs.List(ctxChild, rp, &fs.ListArgs{
				Refresh:            args.Refresh,
				NoLog:              true,
				WithStorageDetails: args.WithStorageDetails,
			})
		}()
		if err != nil {
			continue
		}
		c, err := utils.SliceConvert(o, func(s model.Obj) (model.Obj, error) {
			return model.Obj(&model.Object{
				ID:       s.GetID(),
				Path:     path.Join(dir.GetPath(), s.GetName()),
				Name:     s.GetName(),
				Size:     s.GetSize(),
				Modified: s.ModTime(),
				Ctime:    s.CreateTime(),
				IsFolder: s.IsDir(),
				HashInfo: s.GetHash(),
			}), nil
		})
		if err != nil {
			return nil, err
		}

		objs = append(objs, c...)
	}
	return objs, nil
}

func (d *Gslb) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	// proxy || ftp,s3
	if common.GetApiUrl(ctx) == "" {
		args.Redirect = false
	}

	// 获取客户端 IP 信息
	ipinfo := op.GetIPInfo(args.IP)

	if data, err := utils.Json.MarshalToString(ipinfo); err == nil {
		utils.Log.Infof("[gslb] request ip info: %s", data)
	}

	// 拷贝存储节点列表，过滤不可下载的节点
	sorted := make([]GslbStorage, 0, len(d.storages))
	for _, s := range d.storages {
		if s.NoDown {
			continue
		}
		sorted = append(sorted, s)
	}

	// 按优先级排序存储节点
	slices.SortStableFunc(sorted, func(a, b GslbStorage) int {
		// asn
		aAsn := slices.Contains(a.Asn, ipinfo.Asn)
		bAsn := slices.Contains(b.Asn, ipinfo.Asn)
		if aAsn != bAsn {
			if aAsn {
				return -1
			}
			return 1
		}
		// aso
		aAso := slices.ContainsFunc(a.Aso, func(s string) bool {
			return strings.Contains(strings.ToLower(ipinfo.Aso), strings.ToLower(s))
		})
		bAso := slices.ContainsFunc(b.Aso, func(s string) bool {
			return strings.Contains(strings.ToLower(ipinfo.Aso), strings.ToLower(s))
		})
		if aAso != bAso {
			if aAso {
				return -1
			}
			return 1
		}
		// ISP
		aIsp := slices.ContainsFunc(a.Isp, func(s string) bool {
			return ipinfo.Isp == s
		})
		bIsp := slices.ContainsFunc(b.Isp, func(s string) bool {
			return ipinfo.Isp == s
		})
		if aIsp != bIsp {
			if aIsp {
				return -1
			}
			return 1
		}
		// CountryCode
		aCountryCode := slices.Contains(a.CountryCode, ipinfo.CountryCode)
		bCountryCode := slices.Contains(b.CountryCode, ipinfo.CountryCode)
		if aCountryCode != bCountryCode {
			if aCountryCode {
				return -1
			}
			return 1
		}
		return 0
	})

	var logSorted []string
	for _, s := range sorted {
		logSorted = append(logSorted, s.Path)
	}
	fmt.Printf("[glsb] sorted: %v", logSorted)

	// 按顺序依次尝试获取链接
	for i, s := range sorted {
		rp := path.Join(s.Path, file.GetPath())
		link, _, err := func() (*model.Link, model.Obj, error) { // 为defer使用闭包
			var ctxChild context.Context
			if d.Timeout > 0 {
				var cancel context.CancelFunc
				ctxChild, cancel = context.WithTimeout(ctx, time.Duration(d.Addition.Timeout)*time.Second)
				defer cancel()
			} else {
				ctxChild = ctx
			}
			return d.link(ctxChild, rp, args)
		}()
		if err != nil {
			// 最后一个存储节点出错则返回错误
			if i == len(sorted)-1 {
				return nil, err
			}
			continue
		}
		// 文件/代理类型，重定向回原始存储路径
		if link == nil || link.URL == "" {
			return &model.Link{
				URL: fmt.Sprintf("%s/p%s?sign=%s",
					common.GetApiUrl(ctx),
					utils.EncodePath(rp, true),
					sign.Sign(rp)),
			}, nil
		}
		resultLink := *link
		resultLink.Expiration = nil
		resultLink.SyncClosers = utils.NewSyncClosers(link)
		return &resultLink, nil
	}
	return nil, errs.ObjectNotFound
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*Gslb)(nil)
