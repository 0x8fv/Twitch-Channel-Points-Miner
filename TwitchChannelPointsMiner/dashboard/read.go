package dashboard

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
)

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
