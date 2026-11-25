package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestStore returns a store with a controllable clock.
func newTestStore(t *testing.T, dir string, clock *time.Time) *Store {
	t.Helper()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.now = func() time.Time { return *clock }
	return s
}

func TestDownsampleKeepsEndsAndExtremes(t *testing.T) {
	points := make([]Point, 10000)
	for i := range points {
		points[i] = Point{T: int64(i), P: 1000 + i%7}
	}
	points[4321].P = 999999 // spike
	points[8765].P = -5     // dip

	out := Downsample(points, MaxSeriesPoints)
	if len(out) > MaxSeriesPoints {
		t.Fatalf("got %d points, want <= %d", len(out), MaxSeriesPoints)
	}
	if out[0] != points[0] || out[len(out)-1] != points[len(points)-1] {
		t.Fatalf("first/last point not preserved")
	}
	var spike, dip bool
	for i, p := range out {
		if i > 0 && p.T <= out[i-1].T {
			t.Fatalf("points out of order at %d", i)
		}
		spike = spike || p.P == 999999
		dip = dip || p.P == -5
	}
	if !spike || !dip {
		t.Fatalf("extremes lost: spike=%v dip=%v", spike, dip)
	}
}

func TestDownsampleSmallInputUnchanged(t *testing.T) {
	points := []Point{{T: 1, P: 1}, {T: 2, P: 2}, {T: 3, P: 3}}
	if out := Downsample(points, MaxSeriesPoints); len(out) != 3 {
		t.Fatalf("got %d points, want 3", len(out))
	}
	if out := Downsample(nil, MaxSeriesPoints); len(out) != 0 {
		t.Fatalf("nil input should stay empty")
	}
}

func TestStorePointsRoundTripAndRange(t *testing.T) {
	dir := t.TempDir()
	clock := time.UnixMilli(1_700_000_000_000)
	s := newTestStore(t, dir, &clock)

	s.RecordPoint("Alice", 1000, 0, "") // baseline
	clock = clock.Add(time.Minute)
	s.RecordPoint("alice", 1010, 10, "WATCH")
	clock = clock.Add(time.Hour)
	s.RecordPoint("alice", 1460, 450, "WATCH_STREAK")
	s.RecordPoint("../evil", 1, 1, "WATCH") // unsafe names are ignored

	all, prev, err := s.Points("alice", 0)
	if err != nil || prev != nil || len(all) != 3 {
		t.Fatalf("all points: got %d prev=%v err=%v", len(all), prev, err)
	}
	if all[2].P != 1460 || all[2].D != 450 || all[2].R != "WATCH_STREAK" {
		t.Fatalf("unexpected last point %+v", all[2])
	}

	recent, prev, err := s.Points("alice", clock.Add(-time.Minute).UnixMilli())
	if err != nil || len(recent) != 1 || prev == nil || prev.P != 1010 {
		t.Fatalf("range filter: got %d points prev=%+v err=%v", len(recent), prev, err)
	}

	list := s.Streamers()
	if len(list) != 1 || list[0].Name != "alice" || list[0].Points != 1460 || list[0].SessionGain != 460 {
		t.Fatalf("unexpected summary %+v", list)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("expected a single data file, got %d", len(entries))
	}

	// A new process picks up the last balance from disk and tolerates a torn line.
	f, _ := os.OpenFile(filepath.Join(dir, "alice"+pointsSuffix), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString(`{"t":17000`)
	_ = f.Close()
	reopened := newTestStore(t, dir, &clock)
	if list := reopened.Streamers(); len(list) != 1 || list[0].Points != 1460 || list[0].SessionGain != 0 {
		t.Fatalf("unexpected summary after reopen %+v", list)
	}
	if all, _, _ := reopened.Points("alice", 0); len(all) != 3 {
		t.Fatalf("torn line should be skipped, got %d points", len(all))
	}
}

func TestStoreSnapshotSkipsUnchangedBalance(t *testing.T) {
	clock := time.UnixMilli(1_700_000_000_000)
	s := newTestStore(t, t.TempDir(), &clock)

	s.RecordPoint("bob", 500, 0, "")
	for i := 0; i < 5; i++ {
		clock = clock.Add(5 * time.Minute)
		s.RecordSnapshot("bob", 500, true)
	}
	if pts, _, _ := s.Points("bob", 0); len(pts) != 1 {
		t.Fatalf("unchanged snapshots should be skipped, got %d points", len(pts))
	}

	clock = clock.Add(5 * time.Minute)
	s.RecordSnapshot("bob", 300, true) // e.g. a bet placed outside logPointsDelta
	pts, _, _ := s.Points("bob", 0)
	if len(pts) != 2 || pts[1].P != 300 || pts[1].D != -200 {
		t.Fatalf("changed snapshot not recorded: %+v", pts)
	}

	clock = clock.Add(unchangedInterval)
	s.RecordSnapshot("bob", 300, false)
	if pts, _, _ := s.Points("bob", 0); len(pts) != 3 {
		t.Fatalf("hourly keep-alive point missing, got %d points", len(pts))
	}
	if list := s.Streamers(); list[0].Online || list[0].SessionGain != -200 {
		t.Fatalf("unexpected summary %+v", list)
	}
}

func TestStoreEventsAreAttributedByName(t *testing.T) {
	clock := time.UnixMilli(1_700_000_000_000)
	s := newTestStore(t, t.TempDir(), &clock)
	s.RecordPoint("alice", 1, 0, "")
	s.RecordPoint("alice_tv", 1, 0, "")
	s.RecordPoint("bob", 1, 0, "")

	s.RecordEvent("STREAMER_ONLINE", "🥳 Alice_tv (1.2k points) is Online!")
	s.RecordEvent("BET_WIN", "📊 EventPrediction: bob - will alice win? - Decision: Yes - Result: WIN, Gained: +500")
	s.RecordEvent("DROP_CLAIM", "📦 Claim Some Reward (Campaign) 60/60 (100%)")
	s.RecordEvent("GAIN_FOR_WATCH", "+10 → Alice (1.2k points) - Reason: WATCH") // not stored
	s.RecordEvent("STREAMER_ONLINE", "nobody we know is Online!")                // dropped

	count := func(name, typ string) int {
		events, err := s.Events(name, 0)
		if err != nil {
			t.Fatalf("Events(%s): %v", name, err)
		}
		n := 0
		for _, e := range events {
			if e.Type == typ {
				n++
			}
		}
		return n
	}
	if count("alice_tv", "STREAMER_ONLINE") != 1 || count("alice", "STREAMER_ONLINE") != 0 {
		t.Fatalf("online event should belong to alice_tv only")
	}
	if count("bob", "BET_WIN") != 1 || count("alice", "BET_WIN") != 0 {
		t.Fatalf("bet event should belong to the first streamer named in the message")
	}
	if count("alice", "DROP_CLAIM") != 1 || count("bob", "DROP_CLAIM") != 1 {
		t.Fatalf("unattributed drop claims should be visible for every streamer")
	}
	if count("alice", "GAIN_FOR_WATCH") != 0 {
		t.Fatalf("watch gains must not be stored as events")
	}
	for _, st := range s.Streamers() {
		if st.Online != (st.Name == "alice_tv") {
			t.Fatalf("unexpected online state %+v", st)
		}
	}
}

func TestAPI(t *testing.T) {
	clock := time.UnixMilli(1_700_000_000_000)
	s := newTestStore(t, t.TempDir(), &clock)
	s.RecordPoint("alice", 1000, 0, "")
	for i := 1; i <= 3000; i++ {
		clock = clock.Add(20 * time.Second)
		reason := "WATCH"
		if i%1000 == 0 {
			reason = "WATCH_STREAK"
		}
		s.RecordPoint("alice", 1000+i*10, 10, reason)
	}
	s.RecordEvent("STREAMER_ONLINE", "Alice (31k points) is Online!")
	srv := httptest.NewServer(NewServer(s).Handler())
	defer srv.Close()

	get := func(path string, into interface{}) int {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		if into != nil && resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
		}
		return resp.StatusCode
	}

	var list struct {
		Totals    totals         `json:"totals"`
		Streamers []StreamerInfo `json:"streamers"`
	}
	if code := get("/api/streamers", &list); code != 200 {
		t.Fatalf("streamers status %d", code)
	}
	if list.Totals.Count != 1 || list.Totals.Points != 31000 || list.Totals.SessionGain != 30000 || list.Totals.Online != 1 {
		t.Fatalf("unexpected totals %+v", list.Totals)
	}

	var all streamerResponse
	if code := get("/api/streamer/alice?range=all", &all); code != 200 {
		t.Fatalf("streamer status %d", code)
	}
	if len(all.Series) == 0 || len(all.Series) > MaxSeriesPoints {
		t.Fatalf("series has %d points, want 1..%d", len(all.Series), MaxSeriesPoints)
	}
	if all.Stats.Start != 1000 || all.Stats.End != 31000 || all.Stats.Gain != 30000 {
		t.Fatalf("unexpected stats %+v", all.Stats)
	}
	types := map[string]int{}
	for _, e := range all.Events {
		types[e.Type]++
	}
	if types["WATCH_STREAK"] != 3 || types["STREAMER_ONLINE"] != 1 || len(types) != 2 {
		t.Fatalf("unexpected events %v", types)
	}

	// 3000 samples * 20s = 1000 minutes; the last 24h holds them all, plus nothing before.
	var day streamerResponse
	get("/api/streamer/alice", &day)
	if day.Range != "24h" || day.Stats.Gain != 30000 {
		t.Fatalf("default range response unexpected: range=%s stats=%+v", day.Range, day.Stats)
	}

	if code := get("/api/streamer/alice?range=bogus", nil); code != http.StatusBadRequest {
		t.Fatalf("bad range status %d", code)
	}
	if code := get("/api/streamer/unknown", nil); code != http.StatusNotFound {
		t.Fatalf("unknown streamer status %d", code)
	}
	if code := get("/", nil); code != 200 {
		t.Fatalf("index status %d", code)
	}
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	for _, want := range []string{"Positive", "Negative", "Neutral", "📈", "📉", "🟢", "🔴", "⏫", "↩️"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}
