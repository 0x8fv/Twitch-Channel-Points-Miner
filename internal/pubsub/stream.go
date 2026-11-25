package pubsub

import (
	"fmt"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/jsonvalue"
)

func (p *Client) processPlaybackMessage(topic string, payload map[string]interface{}) error {
	channelID := ""
	if strings.HasPrefix(topic, "video-playback-by-id.") {
		channelID = strings.TrimPrefix(topic, "video-playback-by-id.")
	}
	if channelID == "" {
		return nil
	}
	streamer, ok := p.streamerMap[channelID]
	if !ok {
		return nil
	}
	if p.onPresence == nil {
		return nil
	}
	msgType := strings.ToLower(fmt.Sprint(payload["type"]))
	switch msgType {
	case "stream-up":
		if streamer.Stream == nil {
			streamer.Stream = domain.NewStream()
		} else {
			streamer.Stream.WatchStreakMissing = true
			streamer.Stream.ResetWatchProgress()
		}
		streamer.Stream.StreamUpAt = time.Now()
		p.onPresence(streamer, true, msgType)
	case "viewcount":
		if streamer.Stream != nil && !streamer.Stream.StreamUpElapsed() {
			return nil
		}
		if !streamer.OfflineAt.IsZero() && time.Since(streamer.OfflineAt) < time.Minute {
			return nil
		}
		live, err := p.twitch.IsStreamLive(streamer.ChannelID)
		if err != nil {
			p.logger.Errorf("live check %s: %v", p.streamerName(streamer), err)
			return nil
		}
		if live {
			p.onPresence(streamer, true, msgType)
		}
	case "stream-down":
		p.onPresence(streamer, false, msgType)
	}
	return nil
}

func (p *Client) processRaidMessage(topic string, payload map[string]interface{}) error {
	channelID := strings.TrimPrefix(topic, "raid.")
	streamer := p.streamerMap[channelID]
	if streamer == nil || !streamer.Settings.FollowRaid {
		return nil
	}
	raidData, _ := payload["raid"].(map[string]interface{})
	raidID, _ := raidData["id"].(string)
	target, _ := raidData["target_login"].(string)
	if raidID == "" {
		return nil
	}
	if streamer.LastRaidID == raidID {
		return nil
	}
	streamer.LastRaidID = raidID
	if err := p.twitch.JoinRaid(streamer, raidID); err != nil {
		p.logger.Errorf("join raid %s->%s: %v", p.streamerName(streamer), p.name(target), err)
		return nil
	}
	if target == "" {
		target = "raid target"
	}
	p.logger.EmojiEventf(":performing_arts:", events.EventJoinRaid, "Joining raid from %s to %s", p.streamerName(streamer), p.name(target))
	return nil
}

func (p *Client) processMomentMessage(topic string, payload map[string]interface{}) error {
	channelID := strings.TrimPrefix(topic, "community-moments-channel-v1.")
	streamer := p.streamerMap[channelID]
	if streamer == nil || !streamer.Settings.ClaimMoments {
		return nil
	}
	if strings.ToLower(fmt.Sprint(payload["type"])) != "active" {
		return nil
	}
	data, _ := payload["data"].(map[string]interface{})
	momentID, _ := data["moment_id"].(string)
	if momentID == "" {
		return nil
	}
	if err := p.twitch.ClaimMoment(streamer, momentID); err != nil {
		p.logger.Errorf("claim moment %s: %v", p.streamerName(streamer), err)
		return nil
	}
	p.logger.EmojiEventf(":video_camera:", events.EventMomentClaim, "%s Claimed Moment", p.streamerName(streamer))
	return nil
}

func (p *Client) processCommunityPointChannel(topic string, payload map[string]interface{}) error {
	channelID := strings.TrimPrefix(topic, "community-points-channel-v1.")
	streamer := p.streamerMap[channelID]
	if streamer == nil || !streamer.Settings.CommunityGoals {
		return nil
	}
	msgType := strings.ToLower(fmt.Sprint(payload["type"]))
	if streamer.CommunityGoals == nil {
		streamer.CommunityGoals = make(map[string]*domain.CommunityGoal)
	}
	switch msgType {
	case "community-goal-created", "community-goal-updated":
		data, _ := payload["data"].(map[string]interface{})
		goalData, _ := data["community_goal"].(map[string]interface{})
		if goal := domain.NewCommunityGoalFromPubSub(goalData); goal != nil && goal.ID != "" {
			streamer.CommunityGoals[goal.ID] = goal
		}
		p.twitch.ContributeToCommunityGoals(streamer)
	case "community-goal-deleted":
		data, _ := payload["data"].(map[string]interface{})
		goalData, _ := data["community_goal"].(map[string]interface{})
		id := jsonvalue.String(goalData["id"])
		if id != "" {
			delete(streamer.CommunityGoals, id)
		}
	}
	return nil
}
