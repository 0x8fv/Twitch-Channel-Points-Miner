package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

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
