package dashboard

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Bodies below this size are not worth compressing.
const gzipMinBytes = 1024

var gzipPool = sync.Pool{New: func() interface{} {
	w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return w
}}

func gzipBytes(data []byte, level int) []byte {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, level)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

func acceptsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func writeJSON(w http.ResponseWriter, r *http.Request, v interface{}) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	h.Set("Vary", "Accept-Encoding")
	if len(body) < gzipMinBytes || !acceptsGzip(r) {
		_, _ = w.Write(body)
		return
	}
	h.Set("Content-Encoding", "gzip")
	gz := gzipPool.Get().(*gzip.Writer)
	gz.Reset(w)
	_, _ = gz.Write(body)
	_ = gz.Close()
	gzipPool.Put(gz)
}
