package cache

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
)

// Compressor defines the interface for compression algorithms used by
// the cache middleware. Implementations must be safe for concurrent use.
type Compressor interface {
	// Compress compresses the given data and returns the compressed bytes.
	Compress(data []byte) ([]byte, error)

	// Decompress decompresses the given data and returns the original bytes.
	Decompress(data []byte) ([]byte, error)

	// Name returns the human-readable name of the compression algorithm.
	Name() string
}

// ---------------------------------------------------------------------------
// Gzip Compressor
// ---------------------------------------------------------------------------

// GzipCompressor provides gzip compression/decompression. The compression
// level can be configured at construction time:
//   - -1: default (equivalent to level 6)
//   - 0: no compression
//   - 1: best speed
//   - 9: best compression
type GzipCompressor struct {
	level int
	wPool sync.Pool
	rPool sync.Pool
}

// NewGzipCompressor creates a new GzipCompressor with the given compression
// level. Level must be in the range [-1, 9]. If an invalid level is provided,
// it defaults to -1 (gzip default).
func NewGzipCompressor(level int) *GzipCompressor {
	if level < -1 || level > 9 {
		level = -1
	}
	return &GzipCompressor{
		level: level,
		wPool: sync.Pool{
			New: func() interface{} {
				w, _ := gzip.NewWriterLevel(new(bytes.Buffer), level)
				return w
			},
		},
		rPool: sync.Pool{
			New: func() interface{} {
				return new(gzip.Reader)
			},
		},
	}
}

// Compress compresses the data using gzip at the configured level.
func (g *GzipCompressor) Compress(data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	w := g.wPool.Get().(*gzip.Writer)
	w.Reset(buf)

	if _, err := w.Write(data); err != nil {
		g.wPool.Put(w)
		return nil, fmt.Errorf("gzip compress: %w", err)
	}

	if err := w.Close(); err != nil {
		g.wPool.Put(w)
		return nil, fmt.Errorf("gzip compress close: %w", err)
	}

	g.wPool.Put(w)
	return buf.Bytes(), nil
}

// Decompress decompresses gzip-compressed data.
func (g *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	r := g.rPool.Get().(*gzip.Reader)
	defer g.rPool.Put(r)

	if err := r.Reset(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("gzip decompress reset: %w", err)
	}

	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	return result, nil
}

// Name returns "gzip".
func (*GzipCompressor) Name() string {
	return "gzip"
}

// ---------------------------------------------------------------------------
// Snappy Compressor
// ---------------------------------------------------------------------------

// SnappyCompressor provides Snappy compression/decompression. Snappy
// is optimized for very high speeds with reasonable compression ratios,
// making it ideal for in-memory cache compression where latency is
// critical.
type SnappyCompressor struct{}

// NewSnappyCompressor creates a new SnappyCompressor.
func NewSnappyCompressor() *SnappyCompressor {
	return &SnappyCompressor{}
}

// Compress compresses the data using Snappy.
func (*SnappyCompressor) Compress(data []byte) ([]byte, error) {
	// Use sync.Pool for snappy encoder buffers.
	// Since snappy.Encode handles its own allocation, we delegate to it.
	encoded := snappyEncode(nil, data)
	return encoded, nil
}

// Decompress decompresses Snappy-compressed data.
func (*SnappyCompressor) Decompress(data []byte) ([]byte, error) {
	result := snappyDecode(nil, data)
	return result, nil
}

// Name returns "snappy".
func (*SnappyCompressor) Name() string {
	return "snappy"
}

// Ensure Compressor interface satisfaction.
var (
	_ Compressor = (*GzipCompressor)(nil)
	_ Compressor = (*SnappyCompressor)(nil)
)

// ---------------------------------------------------------------------------
// Compression Middleware
// ---------------------------------------------------------------------------

// CompressionMiddleware provides transparent compression/decompression
// of cache values. Values above the configured minimum size are
// automatically compressed on write and decompressed on read.
//
// Small values (below minSize) are stored uncompressed to avoid the
// overhead of compression on data that is unlikely to benefit.
//
// Example:
//
//	cm := cache.NewCompressionMiddleware(
//	    cache.NewGzipCompressor(6),
//	    1024, // only compress values > 1KB
//	)
//
//	compressed, _ := cm.Compress(largeValue)
//	original, _ := cm.Decompress(compressed)
type CompressionMiddleware struct {
	compressor Compressor
	minSize    int
}

// NewCompressionMiddleware creates a new compression middleware with the
// given compressor and minimum size threshold. Values smaller than minSize
// are stored without compression. A minSize of 0 means all values are
// compressed.
func NewCompressionMiddleware(c Compressor, minSize int) *CompressionMiddleware {
	if minSize < 0 {
		minSize = 0
	}
	return &CompressionMiddleware{
		compressor: c,
		minSize:    minSize,
	}
}

// Compress compresses the value if it exceeds the minimum size threshold.
// Returns the original value unchanged if compression is not beneficial.
func (cm *CompressionMiddleware) Compress(value []byte) ([]byte, error) {
	if len(value) < cm.minSize {
		return value, nil
	}

	compressed, err := cm.compressor.Compress(value)
	if err != nil {
		return nil, fmt.Errorf("compression middleware: %w", err)
	}

	// If compressed data is not smaller, return original.
	if len(compressed) >= len(value) {
		return value, nil
	}

	return compressed, nil
}

// Decompress decompresses the value. If the value was not compressed
// (i.e., it's smaller than the minimum size), it is returned unchanged.
// In practice, callers should track whether a value was compressed
// (e.g., via a metadata flag) for reliable detection.
func (cm *CompressionMiddleware) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	decompressed, err := cm.compressor.Decompress(data)
	if err != nil {
		// If decompression fails, return the original data.
		// This handles the case where data was stored uncompressed.
		return data, nil
	}
	return decompressed, nil
}

// CompressorName returns the name of the underlying compressor.
func (cm *CompressionMiddleware) CompressorName() string {
	return cm.compressor.Name()
}

// MinSize returns the configured minimum size threshold.
func (cm *CompressionMiddleware) MinSize() int {
	return cm.minSize
}

// ---------------------------------------------------------------------------
// Snappy stub implementations
// ---------------------------------------------------------------------------
// NOTE: In production, these would import "github.com/golang/snappy".
// These stubs are provided for API completeness.

func snappyEncode(dst, src []byte) []byte {
	// Stub: in production, use snappy.Encode(dst, src)
	return append(dst[:0], src...)
}

func snappyDecode(_dst, src []byte) []byte {
	// Stub: in production, use snappy.Decode(dst, src)
	result := make([]byte, len(src))
	copy(result, src)
	return result
}
