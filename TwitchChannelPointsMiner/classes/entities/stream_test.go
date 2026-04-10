package entities

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestStreamUpdateAndFlags(t *testing.T) {
	stream := NewStream()
	if !stream.UpdateRequired() {
		t.Fatalf("new stream should require update")
	}

	tagID := "drop-tag"
	game := map[string]interface{}{"displayName": "Game"}
	createdAt := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	stream.Update("id", "title", game, []map[string]interface{}{{"id": tagID}}, 100, createdAt, tagID)

	if stream.Title != "title" || stream.BroadcastID != "id" {
		t.Fatalf("unexpected stream fields: %#v", stream)
	}
	if stream.DropsTags != true {
		t.Fatalf("expected drops tag detection")
	}
	if !stream.CreatedAt.Equal(createdAt) {
		t.Fatalf("createdAt not persisted: got %s want %s", stream.CreatedAt, createdAt)
	}
	if stream.UpdateRequired() {
		t.Fatalf("recent update should not require refresh")
	}
}

func TestStreamWatchProgress(t *testing.T) {
	stream := NewStream()
	stream.lastMinuteUpdate = time.Now().Add(-2 * time.Minute)
	stream.UpdateMinuteWatched()
	if stream.MinuteWatched < 1.9 || stream.MinuteWatched > 2.1 {
		t.Fatalf("minute watched out of range: %f", stream.MinuteWatched)
	}
	stream.WatchCount = 2
	stream.CreatedAt = time.Now()

	stream.ResetWatchProgress()
	if stream.MinuteWatched != 0 || stream.WatchCount != 0 || !stream.CreatedAt.IsZero() || !stream.lastMinuteUpdate.IsZero() {
		t.Fatalf("reset should clear progress")
	}
}

func TestStreamUpdateResetsWatchStreakStateOnBroadcastChange(t *testing.T) {
	stream := NewStream()
	firstCreatedAt := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	stream.Update("broadcast-1", "title", map[string]interface{}{"displayName": "Game"}, nil, 100, firstCreatedAt, "drop-tag")
	stream.WatchStreakMissing = false
	stream.MinuteWatched = 3
	stream.WatchCount = 2
	stream.CreatedAt = firstCreatedAt
	stream.lastMinuteUpdate = time.Now()

	secondCreatedAt := firstCreatedAt.Add(2 * time.Hour)
	stream.Update("broadcast-2", "new title", map[string]interface{}{"displayName": "Game"}, nil, 50, secondCreatedAt, "drop-tag")

	if !stream.WatchStreakMissing {
		t.Fatalf("expected watch streak to reset on new broadcast")
	}
	if stream.MinuteWatched != 0 {
		t.Fatalf("minute watched should reset, got %f", stream.MinuteWatched)
	}
	if stream.WatchCount != 0 {
		t.Fatalf("watch count should reset, got %d", stream.WatchCount)
	}
	if !stream.CreatedAt.Equal(secondCreatedAt) {
		t.Fatalf("createdAt should update to new broadcast start, got %s want %s", stream.CreatedAt, secondCreatedAt)
	}
	if !stream.lastMinuteUpdate.IsZero() {
		t.Fatalf("lastMinuteUpdate should reset on new broadcast")
	}
}

func TestStreamEncodePayload(t *testing.T) {
	stream := NewStream()
	stream.Payload = []map[string]interface{}{
		{"k": "v"},
	}

	encoded, err := stream.EncodePayload()
	if err != nil {
		t.Fatalf("encode payload error: %v", err)
	}
	data, ok := encoded["data"]
	if !ok || data == "" {
		t.Fatalf("expected base64 payload")
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	var decoded []map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if len(decoded) != 1 || decoded[0]["k"] != "v" {
		t.Fatalf("unexpected decoded payload: %#v", decoded)
	}
}

func TestStreamGameName(t *testing.T) {
	stream := NewStream()
	if stream.GameName() != "" {
		t.Fatalf("expected empty name for nil game")
	}
	stream.Game = map[string]interface{}{"displayName": "My Game"}
	if stream.GameName() != "My Game" {
		t.Fatalf("displayName not used")
	}
	stream.Game = map[string]interface{}{"name": "Other"}
	if stream.GameName() != "Other" {
		t.Fatalf("fallback name not used")
	}
}
