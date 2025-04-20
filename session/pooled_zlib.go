package session

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"sync"
)

var compressorPool = sync.Pool{
	New: func() any {
		return &pooledCompressor{}
	},
}

func getCompressor() *pooledCompressor {
	return compressorPool.Get().(*pooledCompressor)
}

func putCompressor(pc *pooledCompressor) {
	compressorPool.Put(pc)
}

type pooledCompressor struct {
	Buf    bytes.Buffer
	Writer *zlib.Writer
}

func (p *pooledCompressor) Compress(data []byte) ([]byte, error) {
	if p.Writer == nil {
		p.Writer = zlib.NewWriter(&p.Buf)
	} else {
		p.Buf.Reset()
		p.Writer.Reset(&p.Buf)
	}
	if _, err := p.Writer.Write(data); err != nil {
		return nil, fmt.Errorf("writing data to compressor: %w", err)
	}
	if err := p.Writer.Close(); err != nil {
		return nil, fmt.Errorf("flushing compressor: %w", err)
	}
	return p.Buf.Bytes(), nil
}

var decompressorPool = sync.Pool{
	New: func() any {
		return &pooledDecompressor{}
	},
}

func getDecompressor() *pooledDecompressor {
	return decompressorPool.Get().(*pooledDecompressor)
}

func putDecompressor(pc *pooledDecompressor) {
	decompressorPool.Put(pc)
}

type pooledDecompressor struct {
	Reader io.ReadCloser
}

func (p *pooledDecompressor) Decompress(data []byte) ([]byte, error) {
	if p.Reader == nil {
		r, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("creating reader: %w", err)
		}
		p.Reader = r
	} else {
		if err := p.Reader.(zlib.Resetter).Reset(bytes.NewReader(data), nil); err != nil {
			return nil, fmt.Errorf("resetting reader: %w", err)
		}
	}
	b, err := io.ReadAll(p.Reader)
	if err != nil {
		return nil, fmt.Errorf("decompressing: %w", err)
	}
	_ = p.Reader.Close()
	return b, nil
}
