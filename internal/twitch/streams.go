package twitch

import (
	"fmt"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

func (t *Client) CheckStreamerOnline(streamer *domain.Streamer) (bool, error) {
	info, err := t.streamInfoOverlay(streamer.Username, streamer.ChannelID)
	if err == ErrStreamerOffline {
		streamer.IsOnline = false
		streamer.OfflineAt = time.Now()
		return false, nil
	}
	if err != nil {
		return streamer.IsOnline, err
	}
	if streamer.Stream == nil {
		streamer.Stream = domain.NewStream()
	}
	streamer.Stream.Update(
		info.StreamID,
		info.Title,
		info.Game,
		info.Tags,
		info.ViewersCount,
		info.CreatedAt,
		twitchapi.DropID,
	)
	streamer.IsOnline = true
	streamer.OnlineAt = time.Now()
	return true, nil
}

// ? IsStreamLive performs a lightweight live check without refreshing stream metadata.
func (t *Client) IsStreamLive(channelID string) (bool, error) {
	if channelID == "" {
		return false, fmt.Errorf("missing channel id")
	}
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.WithIsStreamLiveQuery)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["id"] = channelID
	var resp gqlIsStreamLiveResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		return false, err
	}
	if resp.Data.User == nil {
		return false, nil
	}
	return resp.Data.User.Stream != nil, nil
}

type streamInfoResult struct {
	StreamID            string
	Title               string
	Game                map[string]interface{}
	Tags                []map[string]interface{}
	ViewersCount        int
	CreatedAt           time.Time
	WatchStreakComplete bool
}

func parseRFC3339Timestamp(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func (t *Client) fallbackStreamCreatedAt(channelID string) time.Time {
	if channelID == "" {
		return time.Time{}
	}
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.WithIsStreamLiveQuery)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["id"] = channelID
	var resp gqlIsStreamLiveResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		t.debugf("fallback createdAt lookup failed for channel %s: %v", channelID, err)
		return time.Time{}
	}
	if resp.Data.User == nil || resp.Data.User.Stream == nil {
		return time.Time{}
	}
	return parseRFC3339Timestamp(resp.Data.User.Stream.CreatedAt)
}

func (t *Client) LogExtraInfo(channelID string, online bool) string {
	if channelID == "" {
		return ""
	}
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.RewardList)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelID"] = channelID
	var resp gqlRewardListResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		t.debugf("RewardList lookup failed for channel %s: %v", channelID, err)
		return ""
	}
	extrainfoSuffix := fmt.Sprintf(" | streak length %d", extractWatchStreakLength(&resp))
	if expiresAt := extractWatchStreakExpiresAt(&resp); !expiresAt.IsZero() {
		extrainfoSuffix += fmt.Sprintf(" | expires at %s", expiresAt.Local())
	}
	if online {
		if createdAt := t.fallbackStreamCreatedAt(channelID); !createdAt.IsZero() {
			extrainfoSuffix += fmt.Sprintf(" | started at %s", createdAt.Local())
		}
		if achievementAt := extractWatchStreakAchievementAt(&resp); !achievementAt.IsZero() {
			extrainfoSuffix += fmt.Sprintf(" | streak achievement timestamp %s", achievementAt.Local())
		}
	}
	return extrainfoSuffix
}

func (t *Client) streamInfoOverlay(username, channelID string) (*streamInfoResult, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.VideoPlayerStreamInfoOverlay)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channel"] = username
	var resp gqlStreamInfoOverlayResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		return nil, err
	}
	if resp.Data.User == nil {
		return nil, fmt.Errorf("missing user data for %s: %w", username, ErrChannelNotFound)
	}
	if resp.Data.User.Stream == nil || resp.Data.User.BroadcastSettings == nil {
		return nil, ErrStreamerOffline
	}
	createdAt := parseRFC3339Timestamp(resp.Data.User.Stream.CreatedAt)
	if createdAt.IsZero() {
		createdAt = t.fallbackStreamCreatedAt(channelID)
	}
	return &streamInfoResult{
		StreamID:     resp.Data.User.Stream.ID,
		Title:        resp.Data.User.BroadcastSettings.Title,
		Game:         gameToInterfaceMap(resp.Data.User.BroadcastSettings.Game),
		Tags:         tagsToInterfaceMaps(resp.Data.User.Stream.Tags),
		ViewersCount: resp.Data.User.Stream.ViewersCount,
		CreatedAt:    createdAt,
	}, nil
}

func (t *Client) streamInfo(streamer *domain.Streamer) (*streamInfoResult, error) {
	result, err := t.streamInfoOverlay(streamer.Username, streamer.ChannelID)
	if err != nil {
		return nil, err
	}
	if streamer.Settings.WatchStreak && streamer.ChannelID != "" {
		achievementAt := t.rewardListAchievementAt(streamer.ChannelID)
		result.WatchStreakComplete = currentStreamHasCompletedWatchStreak(result.CreatedAt, achievementAt)
	}
	return result, nil
}

// ? UpdateStream refreshes metadata and payload required for minute-watched events.
func (t *Client) UpdateStream(streamer *domain.Streamer) error {
	if streamer.Stream == nil {
		streamer.Stream = domain.NewStream()
	}
	if !streamer.Stream.UpdateRequired() {
		return nil
	}
	prevGame := strings.TrimSpace(streamer.Stream.GameName())
	info, err := t.streamInfo(streamer)
	if err != nil {
		return err
	}
	game := info.Game
	streamer.Stream.Update(
		info.StreamID,
		info.Title,
		game,
		info.Tags,
		info.ViewersCount,
		info.CreatedAt,
		twitchapi.DropID,
	)
	if info.WatchStreakComplete {
		streamer.CompletedWatchStreak = true
		streamer.Stream.WatchStreakMissing = false
	}

	eventProps := map[string]interface{}{
		"channel_id":   streamer.ChannelID,
		"broadcast_id": streamer.Stream.BroadcastID,
		"user_id":      t.twitchLogin.UserID(),
		"player":       "site",
		"live":         true,
		"channel":      streamer.Username,
	}
	gameName, hasGameName := game["name"].(string)
	if hasGameName && gameName != "" {
		eventProps["game"] = gameName
		if id, ok := game["id"].(string); ok {
			eventProps["game_id"] = id
		}
	}
	if hasGameName && gameName != "" && streamer.Settings.ClaimDrops {
		// campaigns, hasGameDrops, err := t.CampaignIDsForStreamer(streamer)
		campaigns, err := t.CampaignIDsForStreamer(streamer)
		if err == nil {
			streamer.Stream.CampaignIDs = campaigns
			// streamer.Stream.CampaignsResolved = true
			// streamer.Stream.DropsActive = hasGameDrops
		}
	}
	streamer.Stream.Payload = []map[string]interface{}{
		{
			"event":      "minute-watched",
			"properties": eventProps,
		},
	}
	if t.onGameChange != nil {
		if gameName := strings.TrimSpace(streamer.Stream.GameName()); gameName != "" && gameName != prevGame {
			t.onGameChange(streamer, prevGame, gameName)
		}
	}
	return nil
}

func convertTags(tags []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tags))
	for _, tag := range tags {
		if m, ok := tag.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}
