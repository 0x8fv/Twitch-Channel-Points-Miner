package dashboard

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIndexAndJSONAreCompressedAndRevalidated(t *testing.T) {
	clock := time.UnixMilli(1_700_000_000_000)
	s := newTestStore(t, t.TempDir(), &clock)
	for i := 0; i < 500; i++ {
		clock = clock.Add(time.Minute)
		s.RecordPoint("alice", 1000+i*10, 10, "CLAIM")
	}
	srv := httptest.NewServer(NewServer(s).Handler())
	defer srv.Close()
	// Compression is requested by hand so the client does not undo it transparently.
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	do := func(path string, header map[string]string) (*http.Response, []byte) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		for k, v := range header {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp, body
	}
	gunzip := func(data []byte) []byte {
		t.Helper()
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		out, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("gunzip: %v", err)
		}
		return out
	}

	resp, body := do("/", nil)
	etag := resp.Header.Get("ETag")
	if etag == "" || resp.Header.Get("Content-Encoding") != "" || !bytes.Equal(body, indexHTML) {
		t.Fatalf("plain index: etag=%q encoding=%q len=%d", etag, resp.Header.Get("Content-Encoding"), len(body))
	}
	resp, body = do("/", map[string]string{"Accept-Encoding": "gzip"})
	if resp.Header.Get("Content-Encoding") != "gzip" || !bytes.Equal(gunzip(body), indexHTML) {
		t.Fatalf("gzip index does not round-trip")
	}
	if len(body) >= len(indexHTML)/2 {
		t.Fatalf("index barely compressed: %d of %d bytes", len(body), len(indexHTML))
	}
	resp, body = do("/", map[string]string{"If-None-Match": etag})
	if resp.StatusCode != http.StatusNotModified || len(body) != 0 {
		t.Fatalf("revalidation: status %d, %d bytes", resp.StatusCode, len(body))
	}

	resp, body = do("/api/streamer/alice?range=all", map[string]string{"Accept-Encoding": "gzip"})
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("large JSON should be compressed")
	}
	var got streamerResponse
	if err := json.Unmarshal(gunzip(body), &got); err != nil || got.Stats.End != 5990 {
		t.Fatalf("compressed JSON: err=%v stats=%+v", err, got.Stats)
	}
	// A small body is sent as is.
	if resp, _ = do("/api/streamers", map[string]string{"Accept-Encoding": "gzip"}); resp.Header.Get("Content-Encoding") != "" {
		t.Fatalf("small JSON should not be compressed")
	}
}
