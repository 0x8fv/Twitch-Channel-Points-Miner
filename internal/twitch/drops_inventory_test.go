package twitch

import (
	"testing"
)

func TestDropStatusesFromInventoryParsesProgress(t *testing.T) {
	statuses := dropStatusesFromInventory(map[string]interface{}{
		"dropCampaignsInProgress": []interface{}{
			map[string]interface{}{
				"name": "World of Tanks",
				"game": map[string]interface{}{
					"displayName": "World of Tanks",
				},
				"channels": []interface{}{
					map[string]interface{}{
						"login": "example_streamer",
					},
				},
				"timeBasedDrops": []interface{}{
					map[string]interface{}{
						"name":                   "Summer Token Store #12",
						"requiredMinutesWatched": float64(180),
						"self": map[string]interface{}{
							"dropInstanceID":        "drop-1",
							"currentMinutesWatched": float64(63),
							"isClaimed":             false,
						},
					},
				},
			},
		},
	})

	if len(statuses) != 1 {
		t.Fatalf("drop status count got %d want 1", len(statuses))
	}
	status := statuses[0]
	if status.DropInstanceID != "drop-1" {
		t.Fatalf("drop instance id got %q", status.DropInstanceID)
	}
	if status.CampaignName != "World of Tanks" {
		t.Fatalf("campaign name got %q", status.CampaignName)
	}
	if status.GameName != "World of Tanks" {
		t.Fatalf("game name got %q", status.GameName)
	}
	if status.RewardName != "Summer Token Store #12" {
		t.Fatalf("reward name got %q", status.RewardName)
	}
	if status.ChannelName != "example_streamer" {
		t.Fatalf("channel name got %q", status.ChannelName)
	}
	if status.CurrentValue != 63 || status.RequiredValue != 180 {
		t.Fatalf("progress got %d/%d want 63/180", status.CurrentValue, status.RequiredValue)
	}
	if status.Claimed {
		t.Fatalf("drop should not be claimed")
	}
	if status.Claimable {
		t.Fatalf("drop should not be claimable before required progress")
	}
}
