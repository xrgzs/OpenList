package stream

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	hcache "github.com/OpenListTeam/OpenList/v4/internal/hybrid_cache"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/net"
	"github.com/OpenListTeam/OpenList/v4/pkg/buffer"
	"github.com/OpenListTeam/OpenList/v4/pkg/http_range"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type RangeReaderFunc func(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error)

func (f RangeReaderFunc) RangeRead(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error) {
	return f(ctx, httpRange)
}

func GetRangeReaderFromLink(size int64, link *model.Link) (model.RangeReaderIF, error) {
	if link.RangeReader != nil {
		if link.Concurrency < 1 && link.PartSize < 1 {
			return link.RangeReader, nil
		}
		down := net.NewDownloader(func(d *net.Downloader) {
			d.Concurrency = link.Concurrency
			d.PartSize = link.PartSize
			d.HttpClient = net.GetRangeReaderHttpRequestFunc(link.RangeReader)
		})
		rangeReader := func(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error) {
			return down.Download(ctx, &net.HttpRequestParams{
				Range: httpRange,
				Size:  size,
			})
		}
		// RangeReader只能在驱动限速
		return RangeReaderFunc(rangeReader), nil
	}

	if len(link.URL) == 0 {
		return nil, errors.New("invalid link: must have at least one of URL or RangeReader")
	}

	if link.Concurrency > 0 || link.PartSize > 0 {
		down := net.NewDownloader(func(d *net.Downloader) {
			d.Concurrency = link.Concurrency
			d.PartSize = link.PartSize
			d.HttpClient = func(ctx context.Context, params *net.HttpRequestParams) (*http.Response, error) {
				if ServerDownloadLimit == nil {
					return net.DefaultHttpRequestFunc(ctx, params)
				}
				resp, err := net.DefaultHttpRequestFunc(ctx, params)
				if err == nil && resp.Body != nil {
					resp.Body = &RateLimitReader{
						Ctx:     ctx,
						Reader:  resp.Body,
						Limiter: ServerDownloadLimit,
					}
				}
				return resp, err
			}
		})
		rangeReader := func(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error) {
			requestHeader, _ := ctx.Value(conf.RequestHeaderKey).(http.Header)
			header := net.ProcessHeader(requestHeader, link.Header)
			return down.Download(ctx, &net.HttpRequestParams{
				Range:     httpRange,
				Size:      size,
				URL:       link.URL,
				HeaderRef: header,
			})
		}
		return RangeReaderFunc(rangeReader), nil
	}

	rangeReader := func(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error) {
		if httpRange.Length < 0 || httpRange.Start+httpRange.Length > size {
			httpRange.Length = size - httpRange.Start
		}
		requestHeader, _ := ctx.Value(conf.RequestHeaderKey).(http.Header)
		header := net.ProcessHeader(requestHeader, link.Header)
		header = http_range.ApplyRangeToHttpHeader(httpRange, header)

		response, err := net.RequestHttp(ctx, "GET", header, link.URL)
		if err != nil {
			if _, ok := errs.UnwrapOrSelf(err).(net.HttpStatusCodeError); ok {
				return nil, err
			}
			return nil, fmt.Errorf("http request failure, err:%w", err)
		}
		if ServerDownloadLimit != nil {
			response.Body = &RateLimitReader{
				Ctx:     ctx,
				Reader:  response.Body,
				Limiter: ServerDownloadLimit,
			}
		}
		if httpRange.Start == 0 && httpRange.Length == size ||
			response.StatusCode == http.StatusPartialContent ||
			checkContentRange(&response.Header, httpRange.Start) {
			return response.Body, nil
		} else if response.StatusCode == http.StatusOK {
			log.Warnf("remote http server not supporting range request, expect low perfromace!")
			readCloser, err := net.GetRangedHttpReader(response.Body, httpRange.Start, httpRange.Length)
			if err != nil {
				return nil, err
			}
			return readCloser, nil
		}
		return response.Body, nil
	}
	return RangeReaderFunc(rangeReader), nil
}

func GetRangeReaderFromMFile(size int64, file model.File) *model.FileRangeReader {
	return &model.FileRangeReader{
		RangeReaderIF: RangeReaderFunc(func(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error) {
			length := httpRange.Length
			if length < 0 || httpRange.Start+length > size {
				length = size - httpRange.Start
			}
			return &model.FileCloser{File: io.NewSectionReader(file, httpRange.Start, length)}, nil
		}),
	}
}

// 139 cloud does not properly return 206 http status code, add a hack here
func checkContentRange(header *http.Header, offset int64) bool {
	start, _, err := http_range.ParseContentRange(header.Get("Content-Range"))
	if err != nil {
		log.Warnf("exception trying to parse Content-Range, will ignore,err=%s", err)
	}
	if start == offset {
		return true
	}
	return false
}

type ReaderWithCtx struct {
	io.Reader
	Ctx context.Context
}

func (r *ReaderWithCtx) Read(p []byte) (n int, err error) {
	if utils.IsCanceled(r.Ctx) {
		return 0, r.Ctx.Err()
	}
	return r.Reader.Read(p)
}

func (r *ReaderWithCtx) Close() error {
	if c, ok := r.Reader.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func CacheFullAndHash(stream model.FileStreamer, up *model.UpdateProgress, hashType *utils.HashType, hashParams ...any) (model.File, string, error) {
	h := hashType.NewFunc(hashParams...)
	tmpF, err := stream.CacheFullAndWriter(up, h)
	if err != nil {
		return nil, "", err
	}
	return tmpF, hex.EncodeToString(h.Sum(nil)), nil
}

type StreamSectionReader interface {
	// 线程不安全
	GetSectionReader(off, length int64) (io.ReadSeeker, error)
	// 线程安全
	FreeSectionReader(sr io.ReadSeeker)
	// 线程不安全
	DiscardSection(off int64, length int64) error
}

func NewStreamSectionReader(file model.FileStreamer, sectionSize int, up *model.UpdateProgress) (StreamSectionReader, error) {
	if file.GetFile() != nil {
		return &cachedSectionReader{file.GetFile()}, nil
	}

	blockSize := min(uint64(sectionSize), uint64(file.GetSize()), conf.MaxBlockLimit)
	hc, err := hcache.NewHybridCache(blockSize, uint64(file.GetSize()))
	if err != nil {
		return nil, err
	}
	file.Add(hc)

	ss := &hybridSectionReader{
		file:          file,
		hc:            hc,
		prefetchTotal: file.GetSize(),
		chunkSize:     int64(sectionSize),
	}

	// 设置可配置深度的预读流水线
	var depth int
	if conf.Conf != nil {
		depth = conf.Conf.PrefetchChunks
	}
	if depth > 0 {
		numChunks := (file.GetSize() + int64(sectionSize) - 1) / int64(sectionSize)
		if numChunks > 1 {
			// 预读深度不超过实际分片数-1
			if int64(depth) > numChunks-1 {
				depth = int(numChunks - 1)
			}
			poolCh := make(chan buffer.Block, depth+1)
			allocSize := uint64(sectionSize)
			if allocSize > uint64(file.GetSize()) {
				allocSize = uint64(file.GetSize())
			}
			for i := 0; i < depth; i++ {
				b, err := hc.AllocBlock(allocSize)
				if err != nil {
					break
				}
				poolCh <- b
			}
			ss.prefetchCh = make(chan buffer.Block, depth)
			ss.poolCh = poolCh
		}
	}

	return ss, nil
}

type cachedSectionReader struct {
	cache io.ReaderAt
}

func (*cachedSectionReader) DiscardSection(off int64, length int64) error {
	return nil
}
func (s *cachedSectionReader) GetSectionReader(off, length int64) (io.ReadSeeker, error) {
	return io.NewSectionReader(s.cache, off, length), nil
}
func (*cachedSectionReader) FreeSectionReader(sr io.ReadSeeker) {}

type hybridSectionReader struct {
	file       model.FileStreamer
	fileOffset int64
	hc         *hcache.HybridCache
	mu         sync.Mutex
	cache      []buffer.Block

	// 预读流水线：poolCh → prefetch goroutine → prefetchCh → GetSectionReader
	prefetchCh     chan buffer.Block // 已预读完成的分片队列
	poolCh         chan buffer.Block // block 回收池
	prefetchTotal  int64
	chunkSize      int64
	started        bool
	prefetchCancel context.CancelFunc
	prefetchErr    error
}

// 线程不安全
func (ss *hybridSectionReader) DiscardSection(off int64, length int64) error {
	ss.stopPrefetch()
	n, err := utils.CopyWithBufferN(io.Discard, ss.file, length)
	ss.fileOffset += n
	if err != nil {
		return fmt.Errorf("failed to skip data: (expect =%d, actual =%d) %w", length, n, err)
	}
	return nil
}

type blockRefReadSeeker struct {
	io.ReadSeeker
	b buffer.Block
}

// 线程不安全
func (ss *hybridSectionReader) GetSectionReader(off, length int64) (io.ReadSeeker, error) {
	// 预读流水线路径
	if ss.prefetchCh != nil {
		if !ss.started {
			// 第一个分片：直接从源流读取，然后启动预读协程
			ss.started = true
			rs, err := ss.readFromFile(length)
			if err != nil {
				return nil, err
			}
			ss.startPrefetch()
			return rs, nil
		}
		// 后续分片：从预读队列中取
		b, ok := <-ss.prefetchCh
		if !ok {
			if ss.prefetchErr != nil {
				return nil, ss.prefetchErr
			}
			return nil, io.ErrUnexpectedEOF
		}
		if length == b.Size() {
			rs := buffer.ReadAtSeekerOf(b)
			if _, err := rs.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to seek prefetched block: %w", err)
			}
			return &blockRefReadSeeker{rs, b}, nil
		}
		return &blockRefReadSeeker{io.NewSectionReader(b, 0, length), b}, nil
	}

	// 无预读路径（depth=0 或单分片文件）
	if off != ss.fileOffset {
		return nil, fmt.Errorf("stream not cached: request offset %d != current offset %d", off, ss.fileOffset)
	}
	return ss.readFromFile(length)
}

// readFromFile 从源流读取一个分片，优先复用池中 block
func (ss *hybridSectionReader) readFromFile(length int64) (io.ReadSeeker, error) {
	b := ss.get()
	if b == nil {
		offset := int64(ss.hc.Size())
		written, err := ss.hc.CopyFromN(ss.file, length)
		ss.fileOffset += written
		if written != length {
			return nil, fmt.Errorf("failed to read all data: (expect =%d, actual =%d) %w", length, written, err)
		}
		b = buffer.NewBlockAdapter(
			io.NewOffsetWriter(ss.hc, offset),
			io.NewSectionReader(ss.hc, offset, length),
		)
	} else {
		ws := buffer.WriteAtSeekerOf(b)
		if _, err := ws.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to reset cached block writer: %w", err)
		}
		written, err := utils.CopyWithBufferN(ws, ss.file, length)
		ss.fileOffset += written
		if written != length {
			return nil, fmt.Errorf("failed to read all data: (expect =%d, actual =%d) %w", length, written, err)
		}
	}

	if length == b.Size() {
		rs := buffer.ReadAtSeekerOf(b)
		if _, err := rs.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to reset cached block reader: %w", err)
		}
		return &blockRefReadSeeker{rs, b}, nil
	}
	return &blockRefReadSeeker{io.NewSectionReader(b, 0, length), b}, nil
}

func (ss *hybridSectionReader) get() buffer.Block {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if len(ss.cache) > 0 {
		b := ss.cache[len(ss.cache)-1]
		ss.cache = ss.cache[:len(ss.cache)-1]
		return b
	}
	return nil
}
func (ss *hybridSectionReader) put(b buffer.Block) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.cache = append(ss.cache, b)
}

func (ss *hybridSectionReader) FreeSectionReader(rs io.ReadSeeker) {
	if sr, ok := rs.(*blockRefReadSeeker); ok {
		if ss.poolCh != nil {
			// 预读路径：归还到 poolCh 供预读协程复用
			ss.poolCh <- sr.b
		} else {
			ss.put(sr.b)
		}
		sr.b = nil
		sr.ReadSeeker = nil
	}
}

// startPrefetch 启动后台预读协程，持续从源流读取分片塞入 prefetchCh
func (ss *hybridSectionReader) startPrefetch() {
	if ss.prefetchCh == nil || ss.poolCh == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ss.prefetchCancel = cancel
	go ss.prefetchLoop(ctx)
}

// prefetchLoop 预读协程主循环
func (ss *hybridSectionReader) prefetchLoop(ctx context.Context) {
	defer close(ss.prefetchCh)
	for {
		if ss.fileOffset >= ss.prefetchTotal {
			return
		}

		remaining := ss.prefetchTotal - ss.fileOffset
		nextLen := ss.chunkSize
		if nextLen > remaining {
			nextLen = remaining
		}

		// 从 pool 取一个 block（阻塞直到 FreeSectionReader 归还）
		var b buffer.Block
		select {
		case b = <-ss.poolCh:
		case <-ctx.Done():
			return
		}

		ws := buffer.WriteAtSeekerOf(b)
		if _, err := ws.Seek(0, io.SeekStart); err != nil {
			ss.prefetchErr = fmt.Errorf("failed to seek prefetch block: %w", err)
			select {
			case ss.poolCh <- b:
			default:
			}
			return
		}

		// 从源流读取下一个分片（可被取消）
		done := make(chan struct{})
		var written int64
		var readErr error
		go func() {
			written, readErr = utils.CopyWithBufferN(ws, ss.file, nextLen)
			close(done)
		}()

		select {
		case <-done:
			if readErr != nil {
				ss.prefetchErr = fmt.Errorf("failed to prefetch data: (expect =%d, actual =%d) %w", nextLen, written, readErr)
				select {
				case ss.poolCh <- b:
				default:
				}
				return
			}
			ss.fileOffset += written
			// 送入预读队列（阻塞直到 GetSectionReader 消费）
			select {
			case ss.prefetchCh <- b:
			case <-ctx.Done():
				select {
				case ss.poolCh <- b:
				default:
				}
				return
			}
		case <-ctx.Done():
			select {
			case ss.poolCh <- b:
			default:
			}
			return
		}
	}
}

// stopPrefetch 停止预读协程，清空预读队列
func (ss *hybridSectionReader) stopPrefetch() {
	if ss.prefetchCancel != nil {
		ss.prefetchCancel()
		ss.prefetchCancel = nil
		// 等待 prefetch 协程退出（prefetchCh 被关闭）
		if ss.prefetchCh != nil {
			for range ss.prefetchCh {
			}
		}
	}
	// 无论 prefetch 是否启动过，重置状态
	// prefetchCh 可能在未启动时就被 stopPrefetch 调用（如 DiscardSection 在 GetSectionReader 之前调用）
	// 此时 prefetchCh 未被关闭，不能 drain，重建即可
	if ss.prefetchCh != nil && ss.prefetchCancel == nil {
		// prefetch 从未启动，非阻塞 drain
		for {
			select {
			case _, ok := <-ss.prefetchCh:
				if !ok {
					goto resetCh
				}
			default:
				goto resetCh
			}
		}
	}
resetCh:
	if ss.prefetchCh != nil {
		ss.prefetchCh = make(chan buffer.Block, cap(ss.prefetchCh))
	}
	ss.started = false
	ss.prefetchErr = nil
}
