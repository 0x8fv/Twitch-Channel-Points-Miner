package twitch

import (
	"fmt"
	"time"

	"TwitchChannelPointsMiner/internal/random"
)

// watchClip reports a clip of a missed stream as played, then checks the streak.
func (r *streakRecovery) watchClip(node map[string]interface{}) (saved, settled bool, err error) {
	r.clips++
	url, _ := node["url"].(string)
	clipID, _ := node["id"].(string)
	slug, _ := node["slug"].(string)
	createdAt, _ := node["createdAt"].(string)
	eventProps := map[string]interface{}{
		"location":        "vod",
		"url":             url,
		"channel_id":      r.streamer.ChannelID,
		"vod_type":        "clip",
		"vod_id":          clipID,
		"content_mode":    "clip",
		"live":            false,
		"minutes_logged":  0,
		"play_session_id": random.String(32),
		"player":          "site",
		"user_id":         r.client.twitchLogin.UserID(),
		"vod_timestamp":   0,
		"clip_slug":       slug,
	}
	payload := []map[string]interface{}{
		{
			"event":      "video-play",
			"properties": eventProps,
		},
	}
	r.logf("watching clip %s from %s", slug, parseRFC3339Timestamp(createdAt).Local())
	if err := r.client.sendSpadePayload(spadeTrackURL, r.streamer.Username, payload); err != nil {
		return false, false, fmt.Errorf("video-play post failed for channel %s: %v", r.streamer.ChannelID, err)
	}
	r.client.pause(5 * time.Second)
	eventProps["platform"] = "web"
	eventProps["seconds_after_play"] = 5
	eventProps["vod_timestamp"] = 4.9
	payload[0]["event"] = "n_second_play"
	payload[0]["properties"] = eventProps
	if err := r.client.sendSpadePayload(spadeTrackURL, r.streamer.Username, payload); err != nil {
		return false, false, fmt.Errorf("n_second_play post failed for channel %s: %v", r.streamer.ChannelID, err)
	}
	return r.checkSettled()
}

// watchVOD reports up to six minutes of a missed stream's vod as watched, checking
// the streak after each minute.
func (r *streakRecovery) watchVOD(node map[string]interface{}) (saved, settled bool, err error) {
	r.vods++
	vodID, _ := node["id"].(string)
	publishedAt, _ := node["publishedAt"].(string)
	payload := []map[string]interface{}{
		{
			"event": "minute-watched",
			"properties": map[string]interface{}{
				"channel_id":   r.streamer.ChannelID,
				"broadcast_id": nil,
				"player":       "site",
				"user_id":      r.client.twitchLogin.UserID(),
				"live":         false,
				"channel":      r.streamer.Username,
				"vod_id":       vodID,
				"content_mode": "vod",
			},
		},
	}
	for minute := range 6 {
		r.logf("watching vod minute %d from %s", minute, parseRFC3339Timestamp(publishedAt).Local())
		if err := r.client.sendSpadePayload(spadeTrackURL, r.streamer.Username, payload); err != nil {
			return false, false, fmt.Errorf("minute-watched post failed for channel %s: %v", r.streamer.ChannelID, err)
		}
		r.client.pause(5 * time.Second)
		saved, settled, err := r.checkSettled()
		if err != nil || settled {
			return saved, settled, err
		}
		r.client.pause(55 * time.Second)
	}
	return false, false, nil
}
