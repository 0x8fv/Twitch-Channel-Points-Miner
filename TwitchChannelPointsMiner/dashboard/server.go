package dashboard

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

//go:embed index.html
var indexHTML []byte

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

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(indexHTML)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

type totals struct {
	Points      int `json:"points"`
	SessionGain int `json:"session_gain"`
	Online      int `json:"online"`
	Count       int `json:"count"`
}

func (s *Server) handleStreamers(w http.ResponseWriter, _ *http.Request) {
	list := s.store.Streamers()
	t := totals{Count: len(list)}
	for _, st := range list {
		t.Points += st.Points
		t.SessionGain += st.SessionGain
		if st.Online {
			t.Online++
		}
	}
	writeJSON(w, map[string]interface{}{
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
	writeJSON(w, resp)
}
