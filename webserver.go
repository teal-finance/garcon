// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

// StaticWebServer is a webserver serving static files
// among HTML, CSS, JS and popular image formats.
type StaticWebServer struct {
	Dir    string
	Writer Writer
}

// NewStaticWebServer creates a StaticWebServer.
func NewStaticWebServer(dir string, gw Writer) StaticWebServer {
	return StaticWebServer{dir, gw}
}

const avifContentType = "image/avif"

// ServeFile handles one specific file (and its specific Content-Type).
func (ws *StaticWebServer) ServeFile(urlPath, contentType string) func(w http.ResponseWriter, r *http.Request) {
	absPath := path.Join(ws.Dir, urlPath)

	if strings.HasPrefix(contentType, "text/html") {
		return func(w http.ResponseWriter, r *http.Request) {
			// Set short "Cache-Control" because index.html may change on a daily basis
			w.Header().Set("Cache-Control", "public,max-age=3600")
			w.Header().Set("Content-Type", contentType)
			ws.send(w, r, absPath)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Set aggressive "Cache-Control" because ServeFile() is often used
		// to serve "favicon.ico" and other assets that do not change often
		w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
		w.Header().Set("Content-Type", contentType)
		ws.send(w, r, absPath)
	}
}

// ServeDir handles the static files using the same Content-Type.
func (ws *StaticWebServer) ServeDir(contentType string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if TraversalPath(w, r) {
			return
		}

		// JS and CSS files should contain a [hash].
		// Thus the path changes when content changes,
		// enabling aggressive Cache-Control parameters:
		// public            Can be cached by proxy (reverse-proxy. CDN…) and by browser
		// max-age=31536000  Store it up to 1 year (browser stores it some days due to limited cache size)
		// immutable         Only supported by Firefox and Safari
		w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
		w.Header().Set("Content-Type", contentType)

		absPath := path.Join(ws.Dir, r.URL.Path)
		ws.send(w, r, absPath)
	}
}

// ServeImages detects the Content-Type depending on the image extension.
func (ws *StaticWebServer) ServeImages() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if TraversalPath(w, r) {
			return
		}

		// Images are supposed never change, else better to create a new image
		// (or to wait some days the browser clears out data based on LRU).
		w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")

		absPath, contentType := ws.imagePathAndType(r)
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}

		ws.send(w, r, absPath)
	}
}

// ServeAssets detects the Content-Type depending on the asset extension.
func (ws *StaticWebServer) ServeAssets() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if TraversalPath(w, r) {
			return
		}

		extPos := extIndex(r.URL.Path)
		ext := r.URL.Path[extPos:]
		contentType := assetContentType(ext)

		var absPath string
		if contentType == "" {
			absPath, contentType = ws.imagePathAndTypeFromExt(r, extPos, ext)
		}

		w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}

		if absPath == "" {
			absPath = path.Join(ws.Dir, r.URL.Path)
		}
		ws.send(w, r, absPath)
	}
}

func (ws *StaticWebServer) openFile(w http.ResponseWriter, r *http.Request, absPath string) (*os.File, string) {
	// if client (browser) supports Brotli and the *.br file is present
	// => send the *.br file
	accept := r.Header.Get("Accept-Encoding")
	if strings.Contains(accept, "br") {
		brotli := absPath + ".br"
		file, err := os.Open(brotli)
		if err == nil {
			w.Header().Set("Content-Encoding", "br")
			return file, brotli
		}
	}

	file, err := os.Open(absPath)
	if err != nil {
		log.Print("WRN WebServer: ", err)
		ws.Writer.WriteErr(w, r, http.StatusNotFound, "Page not found")
		return nil, ""
	}

	return file, absPath
}

func (ws *StaticWebServer) send(w http.ResponseWriter, r *http.Request, absPath string) {
	file, absPath := ws.openFile(w, r, absPath)

	defer func() {
		if e := file.Close(); e != nil {
			log.Print("WRN WebServer: Close() ", e)
		}
	}()

	if fi, err := file.Stat(); err != nil {
		log.Print("WRN WebServer: Stat(", absPath, ") ", err)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		w.Header().Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))
		// We do not manage PartialContent because too much stuff
		// to handle the headers Range If-Range Etag and Content-Range.
	}

	if n, err := io.Copy(w, file); err != nil {
		log.Print("WRN WebServer: Copy(", absPath, ") ", err)
	} else {
		log.Print("INF WebServer sent ", absPath, " ", ConvertSize64(n))
	}
}

func (ws *StaticWebServer) avifPath(r *http.Request, extPos int) (absPath string) {
	// Just check the first "Accept" header because missing an "image/avif" (from another "Accept" header)
	// do not break anything: will send the image with the original requested encoding format.
	accept := r.Header.Get("Accept")

	// The search is fast but not 100% sure, hoping there is no Content-Type such as "image/avifauna".
	if strings.Contains(accept, avifContentType) {
		imgFile := r.URL.Path[:extPos] + "avif"
		absPath = path.Join(ws.Dir, imgFile)
		_, err := os.Stat(absPath)
		if err == nil {
			return absPath
		}
	}

	return ""
}

// imagePathAndType returns the path/filename and the Content-Type of the image.
// If the client (browser) supports AVIF, imagePathAndType replaces the requested image by the AVIF one.
func (ws *StaticWebServer) imagePathAndType(r *http.Request) (absPath, contentType string) {
	extPos := extIndex(r.URL.Path)

	absPath = ws.avifPath(r, extPos)
	if absPath != "" {
		return absPath, avifContentType
	}

	absPath = path.Join(ws.Dir, r.URL.Path)
	ext := r.URL.Path[extPos:]
	return absPath, imageContentType(ext)
}

func (ws *StaticWebServer) imagePathAndTypeFromExt(r *http.Request, extPos int, ext string) (absPath, contentType string) {
	absPath = ws.avifPath(r, extPos)
	if absPath != "" {
		return absPath, avifContentType
	}
	return "", imageContentType(ext)
}

// extIndex returns the position of the extension within the the urlPath.
// If no dot, returns the ending position.
func extIndex(urlPath string) int {
	for i := len(urlPath) - 1; i >= 0 && urlPath[i] != '/'; i-- {
		if urlPath[i] == '.' {
			return i + 1
		}
	}
	return len(urlPath)
}

// imageContentType determines the Content-Type depending on the file extension.
// Only the image extensions used by Teal.Finance are currently supported.
// Contact the Teal.Finance team if you need more image file extensions.
func imageContentType(ext string) string {
	switch ext {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "svg":
		return "image/svg+xml"
	}
	log.Print("WRN WebServer does not support image extension: ", ext)
	return ""
}

// assetContentType currently supports only the files present in the /dist/assets/ folder at Teal.Finance.
// We may drop ".eot", ".ttf" and ".woff" in the future.
// Contact the Teal.Finance team if you need to keep all the current file extensions, or if you need other ones.
func assetContentType(ext string) string {
	switch ext {
	case "css":
		return "text/css; charset=utf-8"
	case "woff2":
		return "font/woff2"
	case "ttf":
		return "font/ttf"
	case "eot":
		return "application/vnd.ms-fontobject"
	case "woff":
		return "font/woff"
	}
	return ""
}

// https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types/Common_types
//
// Extension  MIME type
// ---------  --------------------------------
//  .html     text/html; charset=utf-8
//  .css      text/css; charset=utf-8
//  .csv      text/csv; charset=utf-8
//  .xml      text/xml; charset=utf-8
//  .js       text/javascript; charset=utf-8
//  .md       text/markdown; charset=utf-8
//  .yaml     text/x-yaml; charset=utf-8
//  .json     application/json; charset=utf-8
//  .pdf      application/pdf
//  .eot      application/vnd.ms-fontobject
//  .ttf      font/ttf
//  .woff     font/woff
//  .woff2    font/woff2
//  .avif     image/avif
//  .gif      image/gif
//  .ico      image/x-icon
//  .jpg      image/jpeg
//  .png      image/png
//  .svg      image/svg+xml
//  .webp     image/webp
