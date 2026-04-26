package gtar

import (
	"bytes"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

// Compress compresses src using the given codec.
// brotliQuality is only used when codec == CodecBrotli; valid range 0-11.
func Compress(codec uint16, src []byte, brotliQuality int) ([]byte, error) {
	switch codec {
	case CodecNone:
		return src, nil

	case CodecLZ4:
		var buf bytes.Buffer
		w := lz4.NewWriter(&buf)
		if err := w.Apply(lz4.BlockSizeOption(lz4.Block4Mb)); err != nil {
			return nil, err
		}
		if _, err := w.Write(src); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case CodecZstd:
		enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
		if err != nil {
			return nil, err
		}
		return enc.EncodeAll(src, nil), nil

	case CodecZstdDict:
		return nil, fmt.Errorf("codec zstd_dict (3) not supported by gtartool: requires external dictionary")

	case CodecBrotli:
		if brotliQuality < 0 || brotliQuality > 11 {
			return nil, fmt.Errorf("brotli quality must be 0-11, got %d", brotliQuality)
		}
		var buf bytes.Buffer
		w := brotli.NewWriterOptions(&buf, brotli.WriterOptions{
			Quality: brotliQuality,
			LGWin:   22,
		})
		if _, err := w.Write(src); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	default:
		return nil, fmt.Errorf("unknown codec: %d", codec)
	}
}

// Decompress decompresses src using the given codec into a buffer of rawSize bytes.
func Decompress(codec uint16, src []byte, rawSize uint32) ([]byte, error) {
	switch codec {
	case CodecNone:
		return src, nil

	case CodecLZ4:
		dst := make([]byte, rawSize)
		r := lz4.NewReader(bytes.NewReader(src))
		if _, err := io.ReadFull(r, dst); err != nil {
			return nil, fmt.Errorf("lz4 decompress: %w", err)
		}
		return dst, nil

	case CodecZstd:
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
		dst, err := dec.DecodeAll(src, make([]byte, 0, rawSize))
		if err != nil {
			return nil, fmt.Errorf("zstd decompress: %w", err)
		}
		return dst, nil

	case CodecZstdDict:
		return nil, fmt.Errorf("codec zstd_dict (3) not supported: requires external dictionary")

	case CodecBrotli:
		r := brotli.NewReader(bytes.NewReader(src))
		dst := make([]byte, 0, rawSize)
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("brotli decompress: %w", err)
		}
		dst = append(dst, buf...)
		return dst, nil

	default:
		return nil, fmt.Errorf("unknown codec: %d", codec)
	}
}
