package op

import (
	"errors"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

var errConflictingLinkLifecycle = errors.New("invalid link lifecycle: expiration cannot be combined with owned closers or RequireReference")

type linkCachePolicy struct {
	expiration       *time.Duration
	requireReference bool
}

func admitLink(link *model.Link, obj model.Obj) (*objWithLink, error) {
	if link.Expiration != nil && (link.RequireReference || link.SyncClosers.Length() > 0) {
		return nil, errors.Join(errConflictingLinkLifecycle, link.Close())
	}
	if link.Time == nil {
		now := time.Now()
		link.Time = &now
	}
	// 设置 CacheInfo：如果 link 上已有（从后端驱动传递），保留；否则根据 Time/Expiration 生成
	if link.CacheInfo == nil && link.Expiration != nil {
		link.CacheInfo = &model.LinkCacheInfo{
			Time:       *link.Time,
			Expiration: *link.Expiration,
		}
	}
	return &objWithLink{
		link: link,
		obj:  obj,
		policy: linkCachePolicy{
			expiration:       link.Expiration,
			requireReference: link.RequireReference,
		},
	}, nil
}

func (ol *objWithLink) acquire() bool {
	return ol.policy.expiration != nil ||
		ol.link.SyncClosers.AcquireReference() || !ol.policy.requireReference
}

// GetLinkCacheInfoFromLink 从 link 中获取缓存信息，优先使用 CacheInfo，回退到 Time/Expiration
func GetLinkCacheInfoFromLink(link *model.Link) *model.LinkCacheInfo {
	if link.CacheInfo != nil {
		return link.CacheInfo
	}
	if link.Expiration != nil && link.Time != nil {
		return &model.LinkCacheInfo{
			Time:       *link.Time,
			Expiration: *link.Expiration,
		}
	}
	return nil
}
