// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package gg

import (
	"compress/bzip2"
	"io"
	"os"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
)

const (
	BrotliExt = ".br"
	Bzip2Ext  = ".bz2"
	GZipExt   = ".gz"
	S2Ext     = ".s2" // S2 is a Snappy extension
	ZStdExt   = ".zst"
)

func SupportedEncoders() []string { return []string{BrotliExt, GZipExt, S2Ext, ZStdExt} }
func SupportedDecoders() []string { return []string{BrotliExt, GZipExt, S2Ext, ZStdExt, Bzip2Ext} }

func Compress(buf []byte, fn, ext string, level int) time.Duration {
	file, err := os.Create(fn)
	if err != nil {
		log.Warnf("Cannot create file %v because %v", fn, err)
		return 0
	}

	startTime := time.Now()

	enc, err := Compressor(file, ext, level)
	if err != nil {
		log.Errorf("Cannot create encoder extension %q level=%v err: %v", ext, level, err)
		return 0
	}

	ok := true
	if _, err := enc.Write(buf); err != nil {
		log.Warnf("Write() %v: %v", ext, err)
		ok = false
	}

	if err := enc.Close(); err != nil {
		log.Warnf("Close() %v: %v", ext, err)
		ok = false
	}

	if err := file.Close(); err != nil {
		log.Warnf("Cannot close file %q because %v", fn, err)
		ok = false
	}

	if !ok {
		return 0
	}

	return time.Since(startTime)
}

func Decompress(fn, ext string) []byte {
	file, err := os.Open(fn)
	if err != nil {
		log.Print("Skip cache file after", err)
		return nil
	}

	defer func() {
		if e := file.Close(); e != nil {
			log.Error("File Close() err:", e)
		}
	}()

	reader := decoder(fn, ext, file)

	defer func() {
		if e := reader.Close(); e != nil {
			log.Error("File Close() err:", e)
		}
	}()

	buf, err := io.ReadAll(reader)
	if err != nil {
		log.Warnf("Cannot ioutil.ReadAll(%v) %v", fn, err)
		return nil
	}

	return buf
}

func Compressor(file *os.File, ext string, level int) (io.WriteCloser, error) {
	switch ext {
	case BrotliExt:
		return BrotliCompressor(file, level), nil

	case GZipExt:
		return GZipCompressor(file, level)

	case S2Ext:
		return S2Compressor(file, level), nil

	case ZStdExt:
		return ZStdCompressor(file, level)

	default:
		log.Printf("Do not compress because extension %q is neither %v", ext, SupportedEncoders())
		return &noCompression{file}, nil // file will be closed by caller
	}
}

func BrotliCompressor(file *os.File, level int) io.WriteCloser {
	if level < brotli.BestSpeed {
		log.Printf("Increase Brotli level=%d to BestSpeed=%d", level, brotli.BestSpeed)
		level = brotli.BestSpeed
	} else if level > brotli.BestCompression {
		log.Printf("Reduce Brotli level=%d to BestCompression=%d", level, brotli.BestCompression)
		level = brotli.BestCompression
	}
	return brotli.NewWriterLevel(file, level)
}

func GZipCompressor(file *os.File, level int) (io.WriteCloser, error) {
	if level < gzip.StatelessCompression {
		log.Printf("Increase GZip level=%d to StatelessCompression=%d", level, gzip.StatelessCompression)
		level = gzip.StatelessCompression
	} else if level > gzip.BestCompression {
		log.Printf("Reduce GZip level=%d to BestCompression=%d", level, gzip.BestCompression)
		level = gzip.BestCompression
	}
	return gzip.NewWriterLevel(file, level)
}

func S2Compressor(file *os.File, level int) io.WriteCloser {
	switch level {
	case 1:
		return s2.NewWriter(file, s2.WriterUncompressed())
	default:
		log.Printf("Change S2 level=%d to default compression level: Fast=2", level)
		fallthrough
	case 2:
		return s2.NewWriter(file)
	case 3:
		return s2.NewWriter(file, s2.WriterBetterCompression())
	case 4:
		return s2.NewWriter(file, s2.WriterBestCompression())
	}
}

func ZStdCompressor(file *os.File, level int) (io.WriteCloser, error) {
	l := zstd.EncoderLevel(level)
	if l < zstd.SpeedFastest {
		log.Printf("Increase Zstd level=%d to SpeedFastest=%d", level, zstd.SpeedFastest)
		l = zstd.SpeedFastest
	} else if l > zstd.SpeedBestCompression {
		log.Printf("Reduce Zstd level=%d to SpeedBestCompression=%d", level, zstd.SpeedBestCompression)
		l = zstd.SpeedBestCompression
	}
	return zstd.NewWriter(file, zstd.WithEncoderLevel(l))
}

func decoder(fn, ext string, file *os.File) io.ReadCloser {
	switch ext {
	case BrotliExt:
		log.Print("Decompressing Brotli from", fn)
		return &fakeRClose{brotli.NewReader(file)}

	case GZipExt:
		log.Print("Decompressing Gzip from", fn)
		r, e := gzip.NewReader(file)
		if e != nil {
			log.Warn("Cannot create gzip.Reader:", e)
			return nil
		}
		return r

	case S2Ext:
		log.Print("Decompressing S2 (Snappy) from", fn)
		return &fakeRClose{s2.NewReader(file)}

	case ZStdExt:
		r, err := zstd.NewReader(file, zstd.WithDecoderConcurrency(4))
		if err != nil {
			log.Warn("Cannot create Zstd Reader:", err)
			return nil
		}
		log.Print("Decompressing Zstd from", fn)
		return &fakeRClose{r}

	case Bzip2Ext:
		log.Print("Decompressing Bzip2 from", fn)
		return &fakeRClose{bzip2.NewReader(file)}

	default:
		log.Printf("Loading without decompression because extension %q is neither %v", ext, SupportedDecoders())
		return &fakeRClose{file} // file will already be closed by caller
	}
}

// noCompression is just to avoid the file being closed twice (when no compression).
type noCompression struct {
	io.Writer
}

func (*noCompression) Close() error { return nil }

// fakeRClose is required because gzip.NewReader() is the single encoder requiring to be explicitly closed.
// That's a pity because gzip.z.Close() only returns z.decompressor.err :-(.
type fakeRClose struct {
	io.Reader
}

func (*fakeRClose) Close() error { return nil }
