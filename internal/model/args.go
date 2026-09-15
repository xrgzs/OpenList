package model

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/OpenListTeam/OpenList/v4/pkg/http_range"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
)

type ListArgs struct {
	ReqPath            string
	S3ShowPlaceholder  bool
	Refresh            bool
	WithStorageDetails bool
	SkipHook           bool
}

type LinkArgs struct {
	IP       string
	Header   http.Header
	Type     string
	Redirect bool
}

type Link struct {
	URL         string        `json:"url"`    // most common way
	Header      http.Header   `json:"header"` // needed header (for url)
	RangeReader RangeReaderIF `json:"-"`      // recommended way if can't use URL

	Expiration *time.Duration // local cache expire Duration

	//for accelerating request, use multi-thread downloading
	Concurrency   int   `json:"concurrency"`
	PartSize      int   `json:"part_size"`
	ContentLength int64 `json:"content_length"` // 转码视频、缩略图

	utils.SyncClosers `json:"-"`
	// 如果SyncClosers中的资源被关闭后Link将不可用，则此值应为 true
	RequireReference bool `json:"-"`

	Time *time.Time // link 生成时间，留空则为获取时自动生成

	// CacheInfo 用于保存后端驱动的缓存元数据，便于在 Expiration 被中间驱动清除后仍可获取缓存信息
	CacheInfo *LinkCacheInfo `json:"-"`
}

// LinkCacheInfo 保存链路缓存的元数据
type LinkCacheInfo struct {
	// 缓存生成时间
	Time time.Time
	// 缓存过期时长
	Expiration time.Duration
}

type linkCacheInfoKeyType struct{}

var linkCacheInfoKey = linkCacheInfoKeyType{}

// WithLinkCacheInfo 将 LinkCacheInfo 存入 context
func WithLinkCacheInfo(ctx context.Context, info *LinkCacheInfo) context.Context {
	return context.WithValue(ctx, linkCacheInfoKey, info)
}

// GetLinkCacheInfo 从 context 中获取 LinkCacheInfo
func GetLinkCacheInfo(ctx context.Context) *LinkCacheInfo {
	v, _ := ctx.Value(linkCacheInfoKey).(*LinkCacheInfo)
	return v
}

func (l *Link) Clone() *Link {
	return &Link{
		URL:              l.URL,
		Header:           l.Header,
		RangeReader:      l.RangeReader,
		Expiration:       l.Expiration,
		Concurrency:      l.Concurrency,
		PartSize:         l.PartSize,
		ContentLength:    l.ContentLength,
		SyncClosers:      utils.NewSyncClosers(l),
		RequireReference: l.RequireReference,
	}
}

type OtherArgs struct {
	Obj    Obj
	Method string
	Data   interface{}
}

type FsOtherArgs struct {
	Path   string      `json:"path" form:"path"`
	Method string      `json:"method" form:"method"`
	Data   interface{} `json:"data" form:"data"`
}

type ArchiveArgs struct {
	Password string
	LinkArgs
}

type ArchiveInnerArgs struct {
	ArchiveArgs
	InnerPath string
}

type ArchiveMetaArgs struct {
	ArchiveArgs
	Refresh bool
}

type ArchiveListArgs struct {
	ArchiveInnerArgs
	Refresh bool
}

type ArchiveDecompressArgs struct {
	ArchiveInnerArgs
	CacheFull     bool
	PutIntoNewDir bool
	Overwrite     bool
}

type SharingListArgs struct {
	Refresh bool
	Pwd     string
}

type SharingArchiveMetaArgs struct {
	ArchiveMetaArgs
	Pwd string
}

type SharingArchiveListArgs struct {
	ArchiveListArgs
	Pwd string
}

type SharingLinkArgs struct {
	Pwd string
	LinkArgs
}

type RangeReaderIF interface {
	RangeRead(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error)
}

type RangeReadCloserIF interface {
	RangeReaderIF
	utils.ClosersIF
}

var _ RangeReadCloserIF = (*RangeReadCloser)(nil)

type RangeReadCloser struct {
	RangeReader RangeReaderIF
	utils.Closers
}

func (r *RangeReadCloser) RangeRead(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error) {
	rc, err := r.RangeReader.RangeRead(ctx, httpRange)
	r.Add(rc)
	return rc, err
}
