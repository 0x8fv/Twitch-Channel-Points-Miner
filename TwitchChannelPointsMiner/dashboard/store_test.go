package dashboard

import (
	"os"
	"path/filepath"
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
