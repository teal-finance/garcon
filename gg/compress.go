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

var (
	SupportedEncoders = []string{BrotliExt, GZipExt, S2Ext, ZStdExt}
	SupportedDecoders = []string{BrotliExt, GZipExt, S2Ext, ZStdExt, Bzip2Ext}
)

func Compress(buf []byte, fn, format string, level int) time.Duration {
	file, err := os.Create(fn)
	if err != nil {
		log.Warnf("Cannot create file %v because %v", fn, err)
		return 0
	}

	t := time.Now()

	enc, err := encoder(file, format, level)
	if err != nil {
		log.Errorf("Cannot create encoder format=%v level=%v err: %v", format, level, err)
		return 0
	}

	ok := true
	if _, err := enc.Write(buf); err != nil {
		log.Warnf("Write() %v: %v", format, err)
		ok = false
	}

	if err := enc.Close(); err != nil {
		log.Warnf("Close() %v: %v", format, err)
		ok = false
	}

	if err := file.Close(); err != nil {
		log.Warnf("Cannot close file %q because %v", fn, err)
		ok = false
	}

	if !ok {
		return 0
	}

	return time.Since(t)
}

func Decompress(fn, format string) []byte {
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

	reader := decoder(fn, format, file)

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

func compress(file *os.File, format string, buf []byte, level int) (ok bool) {
	writer, err := encoder(file, format, level)
	if err != nil {
		log.Errorf("Cannot create encoder format=%v level=%v err: %v", format, level, err)
		return false
	}

	ok = true
	if _, err := writer.Write(buf); err != nil {
		log.Warnf("Write() %v: %v", format, err)
		ok = false
	}

	if err := writer.Close(); err != nil {
		log.Warnf("Close() %v: %v", format, err)
		ok = false
	}

	return ok
}

func encoder(file *os.File, format string, level int) (io.WriteCloser, error) {
	switch format {
	case BrotliExt:
		return brotli.NewWriterLevel(file, level), nil

	case GZipExt:
		return gzip.NewWriterLevel(file, level)

	case S2Ext:
		switch level {
		default:
			return s2.NewWriter(file), nil
		case 1:
			return s2.NewWriter(file, s2.WriterUncompressed()), nil
		case 3:
			return s2.NewWriter(file, s2.WriterBetterCompression()), nil
		case 4:
			return s2.NewWriter(file, s2.WriterBestCompression()), nil
		}

	case ZStdExt:
		l := zstd.EncoderLevel(level)
		if l >= zstd.SpeedBestCompression {
			log.Printf("Reduce Zstd level=%d to SpeedBestCompression=%d", level, zstd.SpeedBestCompression)
			l = zstd.SpeedBestCompression
		}
		return zstd.NewWriter(file, zstd.WithEncoderLevel(l))

	default:
		log.Printf("Do not compress because %q is neither %v", format, SupportedEncoders)
		return &fakeWClose{file}, nil // file will be closed by caller
	}
}

func decoder(fn, format string, file *os.File) io.ReadCloser {
	switch format {
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
		log.Printf("Loading without decompression because %q is neither %v", format, SupportedDecoders)
		return &fakeRClose{file} // file will already be closed by caller
	}
}

// fakeWClose is just to avoid the file being closed twice (when no compression).
type fakeWClose struct {
	io.Writer
}

func (*fakeWClose) Close() error { return nil }

// fakeRClose is required because gzip.NewReader() is the single encoder requiring to be explicitly closed.
// That's a pity because gzip.z.Close() only returns z.decompressor.err :-(.
type fakeRClose struct {
	io.Reader
}

func (*fakeRClose) Close() error { return nil }
