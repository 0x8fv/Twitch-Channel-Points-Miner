package twitchchannelpointsminer

import (
	"io"
	"log"
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func TestHandlePubSubGainSupportsPredictionStakeDeduction(t *testing.T) {
	m := &Miner{
		logger: &Logger{base: log.New(io.Discard, "", 0)},
	}
	streamer := &entities.Streamer{
		Username:      "tester",
		ChannelPoints: 1_000_000,
		PointsInit:    true,
	}

	// ? Stake spend should decrease the local balance
	m.handlePubSubGain(streamer, -250_000, "PREDICTION", 0)
	if streamer.ChannelPoints != 750_000 {
		t.Fatalf("after stake deduction got %d want %d", streamer.ChannelPoints, 750_000)
	}

	// ? Payout (stake + profit) should bring balance to original + profit
	m.handlePubSubGain(streamer, 256_827, "PREDICTION", 0)
	if streamer.ChannelPoints != 1_006_827 {
		t.Fatalf("after payout got %d want %d", streamer.ChannelPoints, 1_006_827)
	}

	entry := streamer.History["PREDICTION"]
	if entry == nil {
		t.Fatalf("expected prediction history entry")
	}
	if entry.Amount != 6_827 {
		t.Fatalf("history amount got %d want %d", entry.Amount, 6_827)
	}
	if entry.Count != 2 {
		t.Fatalf("history count got %d want %d", entry.Count, 2)
	}
}

func TestUpdateHistoryWatchFallbackClearsPendingStreakAfterTwoWatchEvents(t *testing.T) {
	m := &Miner{}
	streamer := &entities.Streamer{
		Stream: entities.NewStream(),
	}

	m.updateHistory(streamer, "WATCH", 10)
	if streamer.Stream.WatchCount != 1 {
		t.Fatalf("watch count after first WATCH got %d want 1", streamer.Stream.WatchCount)
	}
	if !streamer.Stream.WatchStreakMissing {
		t.Fatalf("first WATCH should keep streak pending")
	}

	m.updateHistory(streamer, "WATCH", 10)
	if streamer.Stream.WatchCount != 2 {
		t.Fatalf("watch count after second WATCH got %d want 2", streamer.Stream.WatchCount)
	}
	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("second WATCH should clear pending streak")
	}
}

func TestUpdateHistoryWatchStreakStillClearsPendingStateImmediately(t *testing.T) {
	m := &Miner{}
	streamer := &entities.Streamer{
		Stream: entities.NewStream(),
	}

	m.updateHistory(streamer, "WATCH_STREAK", 450)
	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("WATCH_STREAK should clear pending state immediately")
	}
}
