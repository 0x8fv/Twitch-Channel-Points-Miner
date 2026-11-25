package twitch

import (
	"strconv"
	"time"

	"TwitchChannelPointsMiner/internal/twitchapi"
)

func currentStreamHasCompletedWatchStreak(createdAt, achievementAt time.Time) bool {
	if createdAt.IsZero() || achievementAt.IsZero() {
		return false
	}
	return achievementAt.After(createdAt)
}

func extractWatchStreakAchievementAt(resp *gqlRewardListResponse) time.Time {
	if resp == nil || resp.Data.Channel == nil || resp.Data.Channel.Self == nil {
		return time.Time{}
	}
	milestone := resp.Data.Channel.Self.WatchStreakMilestone
	if milestone == nil || milestone.WatchStreakMilestone == nil {
		return time.Time{}
	}
	return parseRFC3339Timestamp(milestone.WatchStreakMilestone.AchievementTimestamp)
}

func extractWatchStreakExpiresAt(resp *gqlRewardListResponse) time.Time {
	if resp == nil || resp.Data.Channel == nil || resp.Data.Channel.Self == nil {
		return time.Time{}
	}
	milestone := resp.Data.Channel.Self.WatchStreakMilestone
	if milestone == nil {
		return time.Time{}
	}
	return parseRFC3339Timestamp(milestone.ExpiresAt)
}

func extractWatchStreakLength(resp *gqlRewardListResponse) int {
	if resp == nil || resp.Data.Channel == nil || resp.Data.Channel.Self == nil {
		return -1
	}
	milestone := resp.Data.Channel.Self.WatchStreakMilestone
	if milestone == nil || milestone.WatchStreakMilestone == nil {
		return -1
	}
	out, err := strconv.Atoi(milestone.WatchStreakMilestone.Value)
	if err != nil {
		return -1
	}
	return out
}

func (t *Client) rewardListAchievementAt(channelID string) time.Time {
	if channelID == "" {
		return time.Time{}
	}
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.RewardList)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelID"] = channelID
	var resp gqlRewardListResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		t.debugf("RewardList lookup failed for channel %s: %v", channelID, err)
		return time.Time{}
	}
	return extractWatchStreakAchievementAt(&resp)
}
