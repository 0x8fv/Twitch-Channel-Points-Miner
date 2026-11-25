package dashboard

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
	"strings"
)

// The page is written as markup (web/page) plus one file per part
// (web/styles, web/scripts) and put back together here, so the browser still gets everything in one response.
//
//go:embed web
var web embed.FS

// Stylesheets, in cascade order.
var pageStyles = []string{
	"tokens.css",
	"base.css",
	"streamers.css",
	"balance.css",
	"activity.css",
	"phone.css",
}

// Scripts, in load order: they share one scope, so a file may only use at load
// time what an earlier file defines.
var pageScripts = []string{
	"events.js",
	"dom.js",
	"state.js",
	"format.js",
	"theme.js",
	"data.js",
	"streamers.js",
	"balance.js",
	"legend.js",
	"activity.js",
	"chart-scale.js",
	"chart.js",
	"chart-hover.js",
	"boot.js",
}

// indexHTML is the assembled page. It never changes while the process runs, so
// it is compressed once and revalidated by ETag: a reload costs a 304 instead
// of the whole document.
var (
	indexHTML = buildPage()
	indexGzip = gzipBytes(indexHTML, gzip.BestCompression)
	indexETag = func() string {
		sum := sha256.Sum256(indexHTML)
		return `"` + hex.EncodeToString(sum[:8]) + `"`
	}()
)

func buildPage() []byte {
	page := readWeb("web/page/index.html")
	page = inline(page, "/*styles*/", "web/styles/", pageStyles)
	page = inline(page, "/*scripts*/", "web/scripts/", pageScripts)
	return page
}

// inline replaces marker with the named files, joined in order.
func inline(page []byte, marker, dir string, names []string) []byte {
	if bytes.Count(page, []byte(marker)) != 1 {
		panic("dashboard: web/page/index.html must contain " + marker + " exactly once")
	}
	var parts [][]byte
	for _, name := range names {
		parts = append(parts, bytes.TrimRight(readWeb(dir+name), "\n"))
	}
	return bytes.Replace(page, []byte(marker), bytes.Join(parts, []byte("\n")), 1)
}

func readWeb(name string) []byte {
	data, err := web.ReadFile(name)
	if err != nil {
		panic("dashboard: " + err.Error())
	}
	return data
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("ETag", indexETag)
	h.Set("Vary", "Accept-Encoding")
	if strings.Contains(r.Header.Get("If-None-Match"), indexETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if acceptsGzip(r) {
		h.Set("Content-Encoding", "gzip")
		_, _ = w.Write(indexGzip)
		return
	}
	_, _ = w.Write(indexHTML)
}
