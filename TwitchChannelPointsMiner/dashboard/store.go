// Package dashboard stores channel-point analytics as append-only JSONL files
// and serves a small embedded web UI on top of them. Standard library only.
package dashboard

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
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

// Event types kept from the miner's log events. Gains (watch streak, claim, raid)
// are not listed here: they are derived from the recorded points instead.
var storedEvents = map[string]bool{
	"STREAMER_ONLINE":  true,
	"STREAMER_OFFLINE": true,
	"BET_WIN":          true,
	"BET_LOSE":         true,
	"BET_REFUND":       true,
	"DROP_CLAIM":       true,
	"MOMENT_CLAIM":     true,
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

// RecordPoint appends a balance sample for a streamer.
func (s *Store) RecordPoint(streamer string, points, delta int, reason string) {
	name := cleanName(streamer)
	if name == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordLocked(name, points, delta, reason)
}

// RecordSnapshot records the current balance (skipped while unchanged) and online state.
func (s *Store) RecordSnapshot(streamer string, points int, online bool) {
	name := cleanName(streamer)
	if name == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordLocked(name, points, 0, "").online = online
}

func (s *Store) recordLocked(name string, points, delta int, reason string) *streamerState {
	st := s.streamers[name]
	if st == nil {
		st = &streamerState{}
		s.streamers[name] = st
	}
	if !st.hasSession {
		st.sessionP, st.hasSession = points-delta, true
	}
	now := s.now().UnixMilli()
	if delta == 0 && reason == "" && st.hasLast {
		if st.last.P == points && now-st.last.T < unchangedInterval.Milliseconds() {
			return st
		}
		delta = points - st.last.P
	}
	p := Point{T: now, P: points, D: delta, R: reason}
	st.last, st.hasLast = p, true
	s.appendLine(name+pointsSuffix, p)
	return st
}

// RecordEvent stores a miner log event, attributed to the streamer named in the message.
func (s *Store) RecordEvent(event, message string) {
	typ := strings.ToUpper(strings.TrimSpace(event))
	if !storedEvents[typ] {
		return
	}
	msg := strings.TrimSpace(message)
	if r := []rune(msg); len(r) > maxEventMessage {
		msg = string(r[:maxEventMessage]) + "…"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := s.matchLocked(msg)
	switch typ {
	case "STREAMER_ONLINE", "STREAMER_OFFLINE":
		if name == "" {
			return
		}
		s.streamers[name].online = typ == "STREAMER_ONLINE"
	}
	if name == "" {
		name = globalName
	}
	s.appendLine(name+eventsSuffix, Event{T: s.now().UnixMilli(), Type: typ, Msg: msg})
}

// matchLocked returns the known streamer whose name appears first in the message
// as a whole word (longest name wins a tie), or "".
func (s *Store) matchLocked(message string) string {
	msg := strings.ToLower(message)
	best, bestAt := "", -1
	for name := range s.streamers {
		at := indexWord(msg, name)
		if at < 0 {
			continue
		}
		if bestAt < 0 || at < bestAt || (at == bestAt && len(name) > len(best)) {
			best, bestAt = name, at
		}
	}
	return best
}

func indexWord(msg, word string) int {
	for from := 0; from < len(msg); {
		i := strings.Index(msg[from:], word)
		if i < 0 {
			return -1
		}
		i += from
		end := i + len(word)
		if (i == 0 || !isNameByte(msg[i-1])) && (end == len(msg) || !isNameByte(msg[end])) {
			return i
		}
		from = i + 1
	}
	return -1
}

func (s *Store) appendLine(file string, v interface{}) {
	line, err := json.Marshal(v)
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(s.dir, file), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
	_ = f.Close()
}

// Known reports whether name is a recorded streamer.
func (s *Store) Known(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.streamers[name]
	return ok
}

// Streamers returns the live summary of every known streamer, sorted by name.
func (s *Store) Streamers() []StreamerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StreamerInfo, 0, len(s.streamers))
	for name, st := range s.streamers {
		info := StreamerInfo{Name: name, Points: st.last.P, Online: st.online, Updated: st.last.T}
		if st.hasSession {
			info.SessionGain = st.last.P - st.sessionP
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Points loads a streamer's samples with T >= since. prev is the last sample
// before since (nil if none), so a chart can start from a known balance.
func (s *Store) Points(name string, since int64) (points []Point, prev *Point, err error) {
	err = readLines(filepath.Join(s.dir, name+pointsSuffix), func(line []byte) {
		var p Point
		if json.Unmarshal(line, &p) != nil || p.T == 0 {
			return
		}
		if p.T < since {
			prev = &p
			return
		}
		points = append(points, p)
	})
	return points, prev, err
}

// Events loads a streamer's events with T >= since, including unattributed ones.
func (s *Store) Events(name string, since int64) ([]Event, error) {
	var events []Event
	for _, n := range []string{name, globalName} {
		err := readLines(filepath.Join(s.dir, n+eventsSuffix), func(line []byte) {
			var e Event
			if json.Unmarshal(line, &e) != nil || e.T < since || e.Type == "" {
				return
			}
			events = append(events, e)
		})
		if err != nil {
			return nil, err
		}
	}
	return events, nil
}

// readLines calls fn for every line of a file; a missing file is not an error.
// Files are append-only, so a torn last line simply fails to parse and is skipped.
func readLines(path string, fn func(line []byte)) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := sc.Bytes(); len(line) > 0 {
			fn(line)
		}
	}
	return sc.Err()
}

func readLastPoint(path string) (Point, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Point{}, false
	}
	defer f.Close()
	const tail = 4096
	if info, err := f.Stat(); err == nil && info.Size() > tail {
		_, _ = f.Seek(-tail, io.SeekEnd)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return Point{}, false
	}
	lines := bytes.Split(data, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		var p Point
		if len(lines[i]) > 0 && json.Unmarshal(lines[i], &p) == nil && p.T != 0 {
			return p, true
		}
	}
	return Point{}, false
}
