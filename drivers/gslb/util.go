package gslb

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/fs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/server/common"
)

// do others that not defined in Driver interface

func (d *Gslb) link(ctx context.Context, reqPath string, args model.LinkArgs) (*model.Link, model.Obj, error) {
	storage, reqActualPath, err := op.GetStorageAndActualPath(reqPath)
	if err != nil {
		return nil, nil, err
	}
	if !args.Redirect {
		return op.Link(ctx, storage, reqActualPath, args)
	}
	obj, err := fs.Get(ctx, reqPath, &fs.GetArgs{NoLog: true})
	if err != nil {
		return nil, nil, err
	}
	if common.ShouldProxy(storage, path.Base(reqPath)) {
		return nil, obj, nil
	}
	return op.Link(ctx, storage, reqActualPath, args)
}

// calcCountryScore 计算国家匹配分数
// 命中=2，无配置=1，不命中=0
func calcCountryScore(s GslbStorage, countryCode string) int {
	if len(s.CountryCodeNot) > 0 {
		for _, cc := range s.CountryCodeNot {
			if cc == countryCode {
				return 0 // 被排除
			}
		}
		return 2 // 用户不在排除列表中
	}
	if len(s.CountryCode) > 0 {
		for _, cc := range s.CountryCode {
			if cc == countryCode {
				return 2 // code 命中
			}
		}
		return 0 // 不匹配
	}
	return 1 // 无配置，通用节点
}

// calcCarrierScore 计算运营商匹配分数
// ASN/ASO/ISP 任意命中=1，否则=0
func calcCarrierScore(s GslbStorage, ipinfo model.IPInfo) int {
	// ASN
	if slices.Contains(s.Asn, ipinfo.Asn) {
		return 1
	}
	// ASO
	if slices.ContainsFunc(s.Aso, func(v string) bool {
		return strings.Contains(strings.ToLower(ipinfo.Aso), strings.ToLower(v))
	}) {
		return 1
	}
	// ISP
	if slices.ContainsFunc(s.Isp, func(v string) bool {
		return strings.HasPrefix(strings.ToLower(ipinfo.Isp), strings.ToLower(v))
	}) {
		return 1
	}
	return 0
}
