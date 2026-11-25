package twitch

import (
	"fmt"
	"slices"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

func (t *Twitch) RecoverStreak(streamer *domain.Streamer) (bool, error) {
	if streamer == nil || streamer.ChannelID == "" {
		return false, fmt.Errorf("missing streamer channel id")
	}
	name := displayName(streamer.Username)
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		// should ChannelID also be anonymized somehow?
		name = t.anonymizer.StreamerName(streamer)
	}
	rewardlistop := ClonePersistedOperation(GQLOperations.RewardList)
	if rewardlistop.Variables == nil {
		rewardlistop.Variables = map[string]interface{}{}
	}
	rewardlistop.Variables["channelID"] = streamer.ChannelID
	var resp0 gqlRewardListResponse
	if err := t.PostGQLDecode(rewardlistop, &resp0); err != nil {
		return false, fmt.Errorf("RewardList lookup failed for channel %s: %v", streamer.ChannelID, err)
	}
	if &resp0 == nil || resp0.Data.Channel == nil || resp0.Data.Channel.Self == nil {
		return false, fmt.Errorf("RewardList response does not have data.channel.self for channel %s", streamer.ChannelID)
	}
	if expiresAt := extractWatchStreakExpiresAt(&resp0); expiresAt.IsZero() {
		// streak not expiring ... or some missing data in getting expiresAt
		return false, nil
	}
	milestone := resp0.Data.Channel.Self.WatchStreakMilestone
	if milestone == nil || milestone.MissedStreams == nil {
		return false, fmt.Errorf("RewardList response does not have missedStreams for channel %s", streamer.ChannelID)
	}
	var missedstreamids []string
	for _, stream := range milestone.MissedStreams {
		for _, id := range stream.BroadcastIdentifiers {
			missedstreamids = append(missedstreamids, id.ID)
		}
	}
	length0 := extractWatchStreakLength(&resp0)

	clipsfound := 0
	vodsfound := 0
	for _, filter := range []string{"LAST_DAY", "LAST_WEEK", "videos"} {
		// check clips, last day first then last week (in case clips just over 24 hours old are from missed stream)
		// then check vods
		mode := "clips"
		op := ClonePersistedOperation(GQLOperations.ClipsCardsUser)
		if filter == "videos" {
			mode = "videos"
			op = ClonePersistedOperation(GQLOperations.FilterableVideoTowerVideos)
			if op.Variables == nil {
				op.Variables = map[string]interface{}{}
			}
			op.Variables["channelOwnerLogin"] = streamer.Username
		} else {
			if op.Variables == nil {
				op.Variables = map[string]interface{}{}
			}
			op.Variables["login"] = streamer.Username
			op.Variables["criteria"] = map[string]interface{}{"filter": filter}
		}
		hasNext := true
		cursor := ""

		for hasNext {
			op.Variables["cursor"] = cursor
			resp, err := t.PostGQL(op)
			if err != nil {
				return false, err
			}
			respdata := navigate(resp, "data.user."+mode)
			if respdata == nil {
				return false, fmt.Errorf("missing %s data for channel %s", mode, streamer.ChannelID)
			}
			data := respdata.(map[string]interface{})
			edges, _ := data["edges"].([]interface{})
			pageInfo, _ := data["pageInfo"].(map[string]interface{})
			cursor = ""
			for _, edge := range edges {
				e := edge.(map[string]interface{})
				node, _ := e["node"].(map[string]interface{})
				broadcastIdentifier, _ := node["broadcastIdentifier"].(map[string]interface{})
				id, _ := broadcastIdentifier["id"].(string)
				if c, ok := e["cursor"].(string); ok {
					cursor = c
				}

				if slices.Contains(missedstreamids, id) {
					// hardcoding this for now to avoid calling GetSpadeURL on a bunch of offline streamers
					spadeurl := "https://spade.twitch.tv/track"

					if mode == "clips" {
						// clip from a missed stream, eligible for saving streak, watch it
						clipsfound += 1
						url, _ := node["url"].(string)
						clipid, _ := node["id"].(string)
						slug, _ := node["slug"].(string)
						createdAt, _ := node["createdAt"].(string)
						playsessionid := randomString(32)
						eventProps := map[string]interface{}{
							"location":        "vod",
							"url":             url,
							"channel_id":      streamer.ChannelID,
							"vod_type":        "clip",
							"vod_id":          clipid,
							"content_mode":    "clip",
							"live":            false,
							"minutes_logged":  0,
							"play_session_id": playsessionid,
							"player":          "site",
							"user_id":         t.twitchLogin.UserID(),
							"vod_timestamp":   0,
							"clip_slug":       slug,
						}
						payload := []map[string]interface{}{
							{
								"event":      "video-play",
								"properties": eventProps,
							},
						}
						if t.logger != nil {
							t.logger.EmojiPrintf(":ambulance:", "streak recovery %s: watching clip %s from %s", name, slug, parseRFC3339Timestamp(createdAt).Local())
						}
						if err := t.sendSpadePayload(spadeurl, streamer.Username, payload); err != nil {
							return false, fmt.Errorf("video-play post failed for channel %s: %v", streamer.ChannelID, err)
						}
						time.Sleep(5 * time.Second)
						eventProps["platform"] = "web"
						eventProps["seconds_after_play"] = 5
						eventProps["vod_timestamp"] = 4.9
						payload[0]["event"] = "n_second_play"
						payload[0]["properties"] = eventProps
						if err := t.sendSpadePayload(spadeurl, streamer.Username, payload); err != nil {
							return false, fmt.Errorf("n_second_play post failed for channel %s: %v", streamer.ChannelID, err)
						}

						var resp1 gqlRewardListResponse
						if err := t.PostGQLDecode(rewardlistop, &resp1); err != nil {
							return false, fmt.Errorf("RewardList lookup failed for channel %s: %v", streamer.ChannelID, err)
						}
						if &resp1 == nil || resp1.Data.Channel == nil || resp1.Data.Channel.Self == nil {
							return false, fmt.Errorf("RewardList response does not have data.channel.self for channel %s", streamer.ChannelID)
						}
						if expiresAt := extractWatchStreakExpiresAt(&resp1); expiresAt.IsZero() {
							// streak not expiring ... or some missing data in getting expiresAt
							length1 := extractWatchStreakLength(&resp1)
							return length0 == length1, nil
						}
					} else {
						// vod from a missed stream, eligible for saving streak, watch it
						vodsfound += 1
						vodid, _ := node["id"].(string)
						publishedAt, _ := node["publishedAt"].(string)
						eventProps := map[string]interface{}{
							"channel_id":   streamer.ChannelID,
							"broadcast_id": nil,
							"player":       "site",
							"user_id":      t.twitchLogin.UserID(),
							"live":         false,
							"channel":      streamer.Username,
							"vod_id":       vodid,
							"content_mode": "vod",
						}
						payload := []map[string]interface{}{
							{
								"event":      "minute-watched",
								"properties": eventProps,
							},
						}
						for i := range 6 {
							if t.logger != nil {
								t.logger.EmojiPrintf(":ambulance:", "streak recovery %s: watching vod minute %d from %s", name, i, parseRFC3339Timestamp(publishedAt).Local())
							}
							if err := t.sendSpadePayload(spadeurl, streamer.Username, payload); err != nil {
								return false, fmt.Errorf("minute-watched post failed for channel %s: %v", streamer.ChannelID, err)
							}
							time.Sleep(5 * time.Second)

							var resp1 gqlRewardListResponse
							if err := t.PostGQLDecode(rewardlistop, &resp1); err != nil {
								return false, fmt.Errorf("RewardList lookup failed for channel %s: %v", streamer.ChannelID, err)
							}
							if &resp1 == nil || resp1.Data.Channel == nil || resp1.Data.Channel.Self == nil {
								return false, fmt.Errorf("RewardList response does not have data.channel.self for channel %s", streamer.ChannelID)
							}
							if expiresAt := extractWatchStreakExpiresAt(&resp1); expiresAt.IsZero() {
								// streak not expiring ... or some missing data in getting expiresAt
								length1 := extractWatchStreakLength(&resp1)
								return length0 == length1, nil
							}
							time.Sleep(55 * time.Second)
						}
					}
				}
			}
			hasNext, _ = pageInfo["hasNextPage"].(bool)
			if hasNext && len(edges) == 0 {
				if t.logger != nil {
					t.logger.EmojiPrintf(":ambulance:", "streak recovery %s: empty %s edges but hasNextPage == true with cursor %s", name, mode, op.Variables["cursor"])
				}
				hasNext = false
			}
		}
	}
	if vodsfound == 0 {
		// add an extra sleep to let streak expiration refresh here
		if clipsfound == 1 {
			time.Sleep(10 * time.Second)
		} else if clipsfound > 1 {
			time.Sleep(5 * time.Second)
		}
	}

	var resp2 gqlRewardListResponse
	if err := t.PostGQLDecode(rewardlistop, &resp2); err != nil {
		return false, fmt.Errorf("RewardList lookup failed for channel %s: %v", streamer.ChannelID, err)
	}
	if &resp2 == nil || resp2.Data.Channel == nil || resp2.Data.Channel.Self == nil {
		return false, fmt.Errorf("RewardList response does not have data.channel.self for channel %s", streamer.ChannelID)
	}
	expiresAt := extractWatchStreakExpiresAt(&resp2)
	if expiresAt.IsZero() {
		// streak not expiring ... or some missing data in getting expiresAt
		length2 := extractWatchStreakLength(&resp2)
		return length0 == length2, nil
	}
	return false, fmt.Errorf("no eligible clips or vods found to save streak expiring at %s", expiresAt.Local())
}
