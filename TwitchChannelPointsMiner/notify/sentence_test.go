package notify

import (
	"strings"
	"testing"
)

func TestEventSentence(t *testing.T) {
	online := "🥳 Alice (1.2k points) is Online! | Playing: Valorant | streak length 3 | expires at 2026-11-25 15:04:05.123 +0100 CET | started at 2026-11-25 12:00:00 +0100 CET | streak achievement timestamp 2026-11-25 13:00:00 +0100 CET"
	place := "🍀 Place 1,234 points on: Yes (BLUE), Points: 50,000, Users: 120 (45.20%), Odds: 1.80 (55.56%) for bob"
	prediction := "📊 EventPrediction: bob - will alice win? - Decision: A: Yes (BLUE) - Result: WIN, Gained: +500"
	drop := "DROP_PROGRESS [####----------------] 40% | Game / Camp / chan / 12m of 30m / remaining 18m / active"

	cases := []struct {
		in, want string
	}{
		{online, "Alice is online with 1.2k points, playing Valorant. The watch streak is 3."},
		{"😴 Alice (1.2k points) is Offline! | streak length 0 | expires at 2026-11-25 15:04:05 +0100 CET", "Alice is offline with 1.2k points."},
		{"😴 Alice (1.2k points) is Offline! | streak length -1 | started at sometime", "Alice is offline with 1.2k points."},
		{"🚀 +50 → Alice (12.4k points) - Reason: WATCH | Game: Valorant", "Alice gained 50 points for watching Valorant and now has 12.4k points."},
		{"+450 → Alice (1.5k points) - Reason: WATCH_STREAK", "Alice gained 450 points for a watch streak and now has 1.5k points."},
		{place, "bob placed 1,234 points on Yes."},
		{"Place bet on: Yes (BLUE), Points: 10, Users: 2 (50.00%), Odds: 2 (50%) for Stream", "Stream placed a bet on Yes."},
		{prediction, "bob won 500 points on \"will alice win?\" after choosing Yes."},
		{"Place bet after 4m0s for Alice", "The bet for Alice will be placed in 4m0s."},
		{"Skip bet for Alice: balance below minimum_points", "The bet for Alice was skipped because the balance is below the minimum."},
		{"prediction Alice: timeout", "The prediction for Alice failed: timeout."},
		{"Joining raid from Alice to Bob", "Joining the raid from Alice to Bob."},
		{"Alice Claimed Moment", "Alice claimed a moment."},
		{"Claim Cool Hat (Summer Drops) 60/60 (100%)", "Claimed Cool Hat from the Summer Drops campaign, now at 60/60 (100%)."},
		{drop, "The drop Game / Camp / chan is 40% complete, with 12m watched out of 30m."},
		{"nick at #alice wrote: hello <b>there</b>", "nick mentioned you in #alice: \"hello <b>there</b>\"."},
		{"Start session: 'abc'", "The mining session has started."},
		{"Ending session: 'abc'", "The mining session has ended."},
		{"Alice is Online!", "Alice is online."},
	}
	for _, tc := range cases {
		got := Sentence(tc.in)
		if got != tc.want {
			t.Fatalf("sentence(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
		if strings.Contains(got, " · ") {
			t.Fatalf("sentence should not be a chip list: %q", got)
		}
		if again := Sentence(got); again != got {
			t.Fatalf("sentence is not idempotent: %q -> %q", got, again)
		}
	}

	long := "nick at #alice wrote: " + strings.Repeat("é", 800)
	if got := Sentence(long); len([]rune(got)) > sentenceLimit {
		t.Fatalf("mention should be capped at %d runes, got %d", sentenceLimit, len([]rune(got)))
	}
}
