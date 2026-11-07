package gslb

import (
	"context"
	"fmt"
	"math/rand/v2"
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
	// 校验配置合法性，并处理字段隐含关系
	for i := range d.storages {
		if err := d.storages[i].Validate(); err != nil {
			return err
		}
		// balance_universal 隐含 balance: true
		if d.storages[i].BalanceUniversal {
			d.storages[i].Balance = true
		}
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

	// 拷贝存储节点列表，过滤不可下载的节点，并计分
	type scoredStorage struct {
		storage GslbStorage
		score   int
	}
	scored := make([]scoredStorage, 0, len(d.storages))
	for _, s := range d.storages {
		if s.NoDown {
			continue
		}
		if s.MinSize.Int64() > 0 && file.GetSize() < s.MinSize.Int64() {
			continue
		}
		if s.MaxSize.Int64() > 0 && file.GetSize() > s.MaxSize.Int64() {
			continue
		}
		carrierScore := calcCarrierScore(s, ipinfo)
		countryScore := calcCountryScore(s, ipinfo.CountryCode)
		scored = append(scored, scoredStorage{
			storage: s,
			score:   carrierScore + countryScore,
		})
	}

	// Boost 阶段：对 balance_universal 节点条件性提升分数
	maxNonUniversalScore := 0
	for _, ss := range scored {
		if !ss.storage.BalanceUniversal && ss.score > maxNonUniversalScore {
			maxNonUniversalScore = ss.score
		}
	}
	for i := range scored {
		ss := &scored[i]
		if !ss.storage.BalanceUniversal {
			continue
		}
		countryScore := calcCountryScore(ss.storage, ipinfo.CountryCode)
		if countryScore > 0 && maxNonUniversalScore <= 2 && maxNonUniversalScore > ss.score {
			ss.score = maxNonUniversalScore
		}
	}

	// 排序阶段：按分数降序，同分时按原有优先级 tie-breaker
	slices.SortStableFunc(scored, func(a, b scoredStorage) int {
		if a.score != b.score {
			if a.score > b.score {
				return -1
			}
			return 1
		}
		// Tie-breaker: ASN > ASO > ISP > CountryCode
		aAsn := slices.Contains(a.storage.Asn, ipinfo.Asn)
		bAsn := slices.Contains(b.storage.Asn, ipinfo.Asn)
		if aAsn != bAsn {
			if aAsn {
				return -1
			}
			return 1
		}
		aAso := slices.ContainsFunc(a.storage.Aso, func(s string) bool {
			return strings.Contains(strings.ToLower(ipinfo.Aso), strings.ToLower(s))
		})
		bAso := slices.ContainsFunc(b.storage.Aso, func(s string) bool {
			return strings.Contains(strings.ToLower(ipinfo.Aso), strings.ToLower(s))
		})
		if aAso != bAso {
			if aAso {
				return -1
			}
			return 1
		}
		aIsp := slices.ContainsFunc(a.storage.Isp, func(s string) bool {
			return strings.HasPrefix(strings.ToLower(ipinfo.Isp), strings.ToLower(s))
		})
		bIsp := slices.ContainsFunc(b.storage.Isp, func(s string) bool {
			return strings.HasPrefix(strings.ToLower(ipinfo.Isp), strings.ToLower(s))
		})
		if aIsp != bIsp {
			if aIsp {
				return -1
			}
			return 1
		}
		aCC := slices.Contains(a.storage.CountryCode, ipinfo.CountryCode)
		bCC := slices.Contains(b.storage.CountryCode, ipinfo.CountryCode)
		if aCC != bCC {
			if aCC {
				return -1
			}
			return 1
		}
		return 0
	})

	// 同分组内：balance 节点随机打乱，non-balance 保持原序
	sorted := make([]GslbStorage, 0, len(scored))
	i := 0
	for i < len(scored) {
		score := scored[i].score
		j := i + 1
		for j < len(scored) && scored[j].score == score {
			j++
		}
		group := scored[i:j]
		var balanceGroup []GslbStorage
		var nonBalanceGroup []GslbStorage
		for _, ss := range group {
			if ss.storage.Balance {
				balanceGroup = append(balanceGroup, ss.storage)
			} else {
				nonBalanceGroup = append(nonBalanceGroup, ss.storage)
			}
		}
		if len(balanceGroup) >= 2 {
			rand.Shuffle(len(balanceGroup), func(a, b int) {
				balanceGroup[a], balanceGroup[b] = balanceGroup[b], balanceGroup[a]
			})
		}
		sorted = append(sorted, balanceGroup...)
		sorted = append(sorted, nonBalanceGroup...)
		i = j
	}

	var logSorted []string
	for _, s := range sorted {
		logSorted = append(logSorted, s.Path)
	}
	utils.Log.Infof("[gslb] sorted: %v", logSorted)

	// 按顺序依次尝试获取链接
	for i, s := range sorted {
		rp := path.Join(s.Path, file.GetPath())
		link, o, err := func() (*model.Link, model.Obj, error) { // 为defer使用闭包
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
		// 检查文件大小是否匹配
		if d.Addition.CheckFileSize && o.GetSize() != file.GetSize() {
			if i == len(sorted)-1 {
				return nil, fmt.Errorf("file size mismatch: expected %d, got %d", file.GetSize(), o.GetSize())
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
		// 链接类型
		resultLink := *link
		if len(s.Replace) > 0 {
			// 进行链接替换
			for rk, rv := range s.Replace {
				// 只替换第一次出现的
				resultLink.URL = strings.Replace(resultLink.URL, rk, rv, 1)
			}
		}
		resultLink.Expiration = nil // 清空缓存时间
		resultLink.SyncClosers = utils.NewSyncClosers(link)
		return &resultLink, nil
	}
	return nil, errs.ObjectNotFound
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*Gslb)(nil)
