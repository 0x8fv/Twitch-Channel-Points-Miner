// Package dashboard stores channel-point analytics as append-only JSONL files
// and serves a small embedded web UI on top of them. Standard library only.
package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	pointsSuffix = ".points.jsonl"
	eventsSuffix = ".events.jsonl"
	// globalName holds events that could not be attributed to one streamer (e.g. drop claims).
	globalName = "_global"
	// An unchanged balance is re-recorded at most this often.
	unchangedInterval = time.Hour
	maxEventMessage   = 300
)

// Point is one balance sample: time (unix ms), points, delta and reason.
type Point struct {
	T int64  `json:"t"`
	P int    `json:"p"`
	D int    `json:"d,omitempty"`
	R string `json:"r,omitempty"`
}

// Event is one chart annotation.
type Event struct {
	T    int64  `json:"t"`
	Type string `json:"type"`
	Msg  string `json:"msg"`
}

// StreamerInfo is the live summary of one streamer.
type StreamerInfo struct {
	Name        string `json:"name"`
	Points      int    `json:"points"`
	SessionGain int    `json:"session_gain"`
	Online      bool   `json:"online"`
	Updated     int64  `json:"updated"`
}

type streamerState struct {
	last       Point
	hasLast    bool
	sessionP   int
	hasSession bool
	online     bool
}

// Store is safe for concurrent use.
type Store struct {
	dir string
	now func() time.Time

	mu        sync.Mutex
	streamers map[string]*streamerState
}

// NewStore opens (creating if needed) the analytics directory.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, now: time.Now, streamers: make(map[string]*streamerState)}
	files, _ := filepath.Glob(filepath.Join(dir, "*"+pointsSuffix))
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), pointsSuffix)
		if cleanName(name) != name {
			continue
		}
		st := &streamerState{}
		if p, ok := readLastPoint(f); ok {
			st.last, st.hasLast = p, true
		}
		s.streamers[name] = st
	}
	return s, nil
}

// cleanName lowercases a Twitch login and rejects anything that is not a safe file name.
func cleanName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" || len(name) > 64 {
		return ""
	}
	for i := 0; i < len(name); i++ {
		if !isNameByte(name[i]) {
			return ""
		}
	}
	return name
}

func isNameByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}
