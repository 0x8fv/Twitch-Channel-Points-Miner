package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
	// The legend groups events by polarity.
	for _, want := range []string{"Positive", "Negative", "Neutral"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}
