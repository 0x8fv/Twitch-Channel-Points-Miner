package dashboard

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var indexHTML []byte

// The page never changes while the process runs, so it is compressed once and
// revalidated by ETag: a reload costs a 304 instead of the whole document.
var (
	indexGzip = gzipBytes(indexHTML, gzip.BestCompression)
	indexETag = func() string {
		sum := sha256.Sum256(indexHTML)
		return `"` + hex.EncodeToString(sum[:8]) + `"`
	}()
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

// maxEvents caps the markers sent per request (the most recent are kept).
const maxEvents = 2000

var ranges = map[string]time.Duration{
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
	"all": 0,
}

// Gains shown as chart markers, derived from recorded point reasons.
var markerReasons = map[string]bool{"WATCH_STREAK": true, "CLAIM": true, "RAID": true}

func pointsSentence(delta int) string {
	n := delta
	if n < 0 {
		n = -n
	}
	switch {
	case delta < 0:
		return fmt.Sprintf("Lost %d points.", n)
	case delta == 0:
		return "Points did not change."
	default:
		return fmt.Sprintf("Gained %d points.", n)
	}
}

// Server serves the dashboard UI and its JSON API.
type Server struct {
	store *Store
}

func NewServer(store *Store) *Server {
	return &Server{store: store}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /api/streamers", s.handleStreamers)
	mux.HandleFunc("GET /api/streamer/{name}", s.handleStreamer)
	return mux
}

// ListenAndServe blocks serving the dashboard on addr.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	return srv.ListenAndServe()
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

type totals struct {
	Points      int `json:"points"`
	SessionGain int `json:"session_gain"`
	Online      int `json:"online"`
	Count       int `json:"count"`
}

func (s *Server) handleStreamers(w http.ResponseWriter, r *http.Request) {
	list := s.store.Streamers()
	t := totals{Count: len(list)}
	for _, st := range list {
		t.Points += st.Points
		t.SessionGain += st.SessionGain
		if st.Online {
			t.Online++
		}
	}
	writeJSON(w, r, map[string]interface{}{
		"now":       s.store.now().UnixMilli(),
		"totals":    t,
		"streamers": list,
	})
}

type seriesStats struct {
	Start int `json:"start"`
	End   int `json:"end"`
	Gain  int `json:"gain"`
	Min   int `json:"min"`
	Max   int `json:"max"`
}

type streamerResponse struct {
	Name            string      `json:"name"`
	Range           string      `json:"range"`
	From            int64       `json:"from"`
	To              int64       `json:"to"`
	Series          [][2]int64  `json:"series"`
	Events          []Event     `json:"events"`
	EventsTruncated bool        `json:"events_truncated"`
	Stats           seriesStats `json:"stats"`
}

func (s *Server) handleStreamer(w http.ResponseWriter, r *http.Request) {
	name := cleanName(r.PathValue("name"))
	if name == "" || !s.store.Known(name) {
		http.Error(w, "unknown streamer", http.StatusNotFound)
		return
	}
	rangeName := r.URL.Query().Get("range")
	if rangeName == "" {
		rangeName = "24h"
	}
	span, ok := ranges[rangeName]
	if !ok {
		http.Error(w, "range must be one of 24h, 7d, 30d, all", http.StatusBadRequest)
		return
	}
	now := s.store.now().UnixMilli()
	var since int64
	if span > 0 {
		since = now - span.Milliseconds()
	}

	points, prev, err := s.store.Points(name, since)
	if err != nil {
		http.Error(w, "failed to read points", http.StatusInternalServerError)
		return
	}
	events, err := s.store.Events(name, since)
	if err != nil {
		http.Error(w, "failed to read events", http.StatusInternalServerError)
		return
	}
	for _, p := range points {
		if markerReasons[p.R] {
			events = append(events, Event{T: p.T, Type: p.R, Msg: pointsSentence(p.D)})
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].T < events[j].T })

	resp := streamerResponse{Name: name, Range: rangeName, From: since, To: now, Events: events}
	if len(events) > maxEvents {
		resp.Events = events[len(events)-maxEvents:]
		resp.EventsTruncated = true
	}
	if resp.Events == nil {
		resp.Events = []Event{}
	}

	// Carry the balance held at the start of the window into the series.
	if prev != nil {
		points = append([]Point{{T: since, P: prev.P}}, points...)
	}
	if len(points) > 0 {
		if since == 0 {
			resp.From = points[0].T
		}
		st := seriesStats{Start: points[0].P, End: points[len(points)-1].P, Min: points[0].P, Max: points[0].P}
		for _, p := range points {
			if p.P < st.Min {
				st.Min = p.P
			}
			if p.P > st.Max {
				st.Max = p.P
			}
		}
		st.Gain = st.End - st.Start
		resp.Stats = st
	} else if since == 0 {
		resp.From = now
	}
	sampled := Downsample(points, MaxSeriesPoints)
	resp.Series = make([][2]int64, len(sampled))
	for i, p := range sampled {
		resp.Series[i] = [2]int64{p.T, int64(p.P)}
	}
	writeJSON(w, r, resp)
}
