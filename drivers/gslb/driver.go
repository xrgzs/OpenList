package gslb

import (
	"context"
	"fmt"
	"net/netip"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/fs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/sign"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/oschwald/geoip2-golang/v2"
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
	var ipinfo *geoip2.ASN
	ip := args.IP
	nip, _ := netip.ParseAddr(ip)
	geo := db.GetGeoDb()
	if geo != nil && nip.IsValid() {
		info, err := geo.ASN(nip)
		if err == nil && info.HasData() {
			ipinfo = info
		}
	}
	utils.Log.Infof("[gslb] request ip info: %+v", ipinfo)

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
		// 按地理位置匹配，有则提高优先级
		if ipinfo != nil {
			if strings.Contains(ipinfo.AutonomousSystemOrganization, a.Aso) && !strings.Contains(ipinfo.AutonomousSystemOrganization, b.Aso) {
				return -1
			}
			if slices.Contains(a.Asn, ipinfo.AutonomousSystemNumber) && !slices.Contains(b.Asn, ipinfo.AutonomousSystemNumber) {
				return -1
			}
		}
		// 最后保留原有顺序
		return 0
	})

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
			return fs.Link(ctxChild, rp, args)
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
		resultLink.SyncClosers = utils.NewSyncClosers(link)
		return &resultLink, nil
	}
	return nil, errs.ObjectNotFound
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*Gslb)(nil)
