package twitch

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/twitchapi"
)

// recoveryTwitch plays Twitch for a streak recovery: it answers the reward list,
// the clip and video listings, and records what was reported as watched.
type recoveryTwitch struct {
	t *testing.T

	// rewardLists are returned in order, one per RewardList request; the last repeats.
	rewardLists []string
	// listings maps "LAST_DAY", "LAST_WEEK" and "videos" to their pages, in order.
	listings map[string][]string

	rewardListCalls int
	listingCursors  map[string][]string
	watched         []string
	sleeps          []time.Duration
}

const (
	streakExpiring = `{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"value":"3"},"missedStreams":[{"broadcastIdentifiers":[{"id":"missed-1"}]}],"expiresAt":"2026-11-26T00:00:00Z"}}}}}`
	streakSaved    = `{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"value":"3"},"missedStreams":[]}}}}}`
	streakLost     = `{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"value":"0"},"missedStreams":[]}}}}}`
	emptyClips     = `{"data":{"user":{"clips":{"edges":[],"pageInfo":{"hasNextPage":false}}}}}`
	emptyVideos    = `{"data":{"user":{"videos":{"edges":[],"pageInfo":{"hasNextPage":false}}}}}`
)

func clipPage(broadcastID, cursor string, hasNext bool) string {
	return fmt.Sprintf(`{"data":{"user":{"clips":{"edges":[{"cursor":%q,"node":{"id":"clip-id","slug":"clip-slug","url":"https://clips.example/clip-slug","createdAt":"2026-11-24T10:00:00Z","broadcastIdentifier":{"id":%q}}}],"pageInfo":{"hasNextPage":%t}}}}}`, cursor, broadcastID, hasNext)
}

func videoPage(broadcastID string) string {
	return fmt.Sprintf(`{"data":{"user":{"videos":{"edges":[{"cursor":"v1","node":{"id":"vod-id","publishedAt":"2026-11-24T10:00:00Z","broadcastIdentifier":{"id":%q}}}],"pageInfo":{"hasNextPage":false}}}}}`, broadcastID)
}

func (r *recoveryTwitch) client() *Client {
	r.listingCursors = map[string][]string{}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if req.URL.Host == "spade.twitch.tv" {
			form, _ := url.ParseQuery(string(raw))
			decoded, _ := base64.StdEncoding.DecodeString(form.Get("data"))
			var events []struct {
				Event      string                 `json:"event"`
				Properties map[string]interface{} `json:"properties"`
			}
			if err := json.Unmarshal(decoded, &events); err != nil || len(events) != 1 {
				r.t.Fatalf("unexpected spade payload %q: %v", decoded, err)
			}
			if events[0].Properties["user_id"] != "user-id" || events[0].Properties["channel_id"] != "123456" {
				r.t.Fatalf("spade event has the wrong identity: %v", events[0].Properties)
			}
			r.watched = append(r.watched, events[0].Event)
			return jsonResponse(http.StatusNoContent, ""), nil
		}
		var body struct {
			OperationName string `json:"operationName"`
			Variables     struct {
				Cursor   string `json:"cursor"`
				Criteria struct {
					Filter string `json:"filter"`
				} `json:"criteria"`
			} `json:"variables"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			r.t.Fatalf("unexpected request body %q: %v", raw, err)
		}
		switch body.OperationName {
		case "RewardList":
			i := r.rewardListCalls
			if i >= len(r.rewardLists) {
				i = len(r.rewardLists) - 1
			}
			r.rewardListCalls++
			return jsonResponse(200, r.rewardLists[i]), nil
		case "ClipsCards__User", "FilterableVideoTower_Videos":
			key := body.Variables.Criteria.Filter
			if body.OperationName == "FilterableVideoTower_Videos" {
				key = "videos"
			}
			pages := r.listings[key]
			i := len(r.listingCursors[key])
			r.listingCursors[key] = append(r.listingCursors[key], body.Variables.Cursor)
			if i >= len(pages) {
				r.t.Fatalf("%s was asked for page %d but only %d exist", key, i+1, len(pages))
			}
			return jsonResponse(200, pages[i]), nil
		}
		r.t.Fatalf("unexpected operation %q", body.OperationName)
		return nil, nil
	})
	return &Client{
		userAgent:      "ua",
		deviceID:       "device",
		clientSession:  "session",
		clientVersion:  twitchapi.ClientVersion,
		versionTTL:     time.Hour,
		versionFetched: time.Now(),
		twitchLogin:    fakeSession{token: "token", userID: "user-id"},
		client:         &http.Client{Transport: transport},
		sleep:          func(d time.Duration) { r.sleeps = append(r.sleeps, d) },
	}
}

func TestRecoverStreakDoesNothingWhenStreakIsNotExpiring(t *testing.T) {
	r := &recoveryTwitch{t: t, rewardLists: []string{streakSaved}}
	saved, err := r.client().RecoverStreak(newTestStreamer(true))
	if saved || err != nil {
		t.Fatalf("got saved=%t err=%v, want false and no error", saved, err)
	}
	if r.rewardListCalls != 1 || len(r.listingCursors) != 0 || len(r.watched) != 0 {
		t.Fatalf("nothing beyond one reward list lookup should happen: %d lookups, listings %v, watched %v", r.rewardListCalls, r.listingCursors, r.watched)
	}
}

func TestRecoverStreakRequiresChannelID(t *testing.T) {
	r := &recoveryTwitch{t: t}
	streamer := newTestStreamer(true)
	streamer.ChannelID = ""
	if _, err := r.client().RecoverStreak(streamer); err == nil || !strings.Contains(err.Error(), "missing streamer channel id") {
		t.Fatalf("got %v, want a missing channel id error", err)
	}
	if _, err := r.client().RecoverStreak(nil); err == nil {
		t.Fatalf("nil streamer should be an error")
	}
}

func TestRecoverStreakWatchesClipOfMissedStream(t *testing.T) {
	r := &recoveryTwitch{
		t:           t,
		rewardLists: []string{streakExpiring, streakSaved},
		listings:    map[string][]string{"LAST_DAY": {clipPage("missed-1", "c1", false)}},
	}
	saved, err := r.client().RecoverStreak(newTestStreamer(true))
	if !saved || err != nil {
		t.Fatalf("got saved=%t err=%v, want the streak saved", saved, err)
	}
	if want := []string{"video-play", "n_second_play"}; !reflect.DeepEqual(r.watched, want) {
		t.Fatalf("watched %v, want %v", r.watched, want)
	}
	if want := []time.Duration{5 * time.Second}; !reflect.DeepEqual(r.sleeps, want) {
		t.Fatalf("sleeps %v, want %v", r.sleeps, want)
	}
	if r.rewardListCalls != 2 {
		t.Fatalf("reward list lookups got %d, want 2", r.rewardListCalls)
	}
}

func TestRecoverStreakReportsLostStreakAfterClip(t *testing.T) {
	r := &recoveryTwitch{
		t:           t,
		rewardLists: []string{streakExpiring, streakLost},
		listings:    map[string][]string{"LAST_DAY": {clipPage("missed-1", "c1", false)}},
	}
	saved, err := r.client().RecoverStreak(newTestStreamer(true))
	if saved || err != nil {
		t.Fatalf("got saved=%t err=%v, want false: the streak length changed", saved, err)
	}
}

func TestRecoverStreakFollowsClipPagesByCursor(t *testing.T) {
	r := &recoveryTwitch{
		t:           t,
		rewardLists: []string{streakExpiring, streakSaved},
		listings:    map[string][]string{"LAST_DAY": {clipPage("other-stream", "c1", true), clipPage("missed-1", "c2", false)}},
	}
	saved, err := r.client().RecoverStreak(newTestStreamer(true))
	if !saved || err != nil {
		t.Fatalf("got saved=%t err=%v, want the streak saved from the second page", saved, err)
	}
	if want := []string{"", "c1"}; !reflect.DeepEqual(r.listingCursors["LAST_DAY"], want) {
		t.Fatalf("cursors sent %v, want %v", r.listingCursors["LAST_DAY"], want)
	}
}

func TestRecoverStreakFallsBackToVideos(t *testing.T) {
	r := &recoveryTwitch{
		t:           t,
		rewardLists: []string{streakExpiring, streakExpiring, streakSaved},
		listings:    map[string][]string{"LAST_DAY": {emptyClips}, "LAST_WEEK": {emptyClips}, "videos": {videoPage("missed-1")}},
	}
	saved, err := r.client().RecoverStreak(newTestStreamer(true))
	if !saved || err != nil {
		t.Fatalf("got saved=%t err=%v, want the streak saved by a vod", saved, err)
	}
	// The first minute leaves the streak expiring, the second saves it.
	if want := []string{"minute-watched", "minute-watched"}; !reflect.DeepEqual(r.watched, want) {
		t.Fatalf("watched %v, want %v", r.watched, want)
	}
	if want := []time.Duration{5 * time.Second, 55 * time.Second, 5 * time.Second}; !reflect.DeepEqual(r.sleeps, want) {
		t.Fatalf("sleeps %v, want %v", r.sleeps, want)
	}
}

func TestRecoverStreakFailsWhenNothingIsEligible(t *testing.T) {
	r := &recoveryTwitch{
		t:           t,
		rewardLists: []string{streakExpiring},
		listings:    map[string][]string{"LAST_DAY": {emptyClips}, "LAST_WEEK": {emptyClips}, "videos": {emptyVideos}},
	}
	saved, err := r.client().RecoverStreak(newTestStreamer(true))
	if saved || err == nil || !strings.Contains(err.Error(), "no eligible clips or vods found") {
		t.Fatalf("got saved=%t err=%v, want a no-eligible-content error", saved, err)
	}
	if len(r.watched) != 0 || len(r.sleeps) != 0 {
		t.Fatalf("nothing should be watched or waited for: watched %v, sleeps %v", r.watched, r.sleeps)
	}
	if r.rewardListCalls != 2 {
		t.Fatalf("reward list lookups got %d, want 2 (before and after the search)", r.rewardListCalls)
	}
}

func TestRecoverStreakWaitsAfterClipsBeforeFinalCheck(t *testing.T) {
	// The clip is watched but the streak still shows as expiring right after it,
	// so the search continues and the final check comes after a short wait.
	r := &recoveryTwitch{
		t:           t,
		rewardLists: []string{streakExpiring, streakExpiring, streakSaved},
		listings:    map[string][]string{"LAST_DAY": {clipPage("missed-1", "c1", false)}, "LAST_WEEK": {emptyClips}, "videos": {emptyVideos}},
	}
	saved, err := r.client().RecoverStreak(newTestStreamer(true))
	if !saved || err != nil {
		t.Fatalf("got saved=%t err=%v, want the streak saved at the final check", saved, err)
	}
	if want := []time.Duration{5 * time.Second, 10 * time.Second}; !reflect.DeepEqual(r.sleeps, want) {
		t.Fatalf("sleeps %v, want %v", r.sleeps, want)
	}
}
