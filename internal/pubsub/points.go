package pubsub

import (
	"fmt"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/jsonvalue"
)

func (p *Client) processPointsEarned(payload map[string]interface{}, channelID string) error {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return nil
	}
	if channelID == "" {
		channelID = channelIDFromPayload(payload, "")
	}
	if channelID == "" {
		return nil
	}
	streamer := p.streamerMap[channelID]
	if streamer == nil {
		return nil
	}

	pointGainVal := jsonvalue.At(data, "point_gain")
	pointGain, _ := pointGainVal.(map[string]interface{})
	if pointGain == nil {
		return nil
	}
	reason := strings.ToUpper(fmt.Sprint(pointGain["reason_code"]))
	earned := int(jsonvalue.Float(pointGain["total_points"]))
	balance := streamer.ChannelPoints
	if balanceValue := jsonvalue.At(data, "balance.balance"); balanceValue != nil {
		balance = int(jsonvalue.Float(balanceValue))
	}
	if p.onGain != nil {
		p.onGain(streamer, earned, reason, balance)
	}
	return nil
}

func (p *Client) processClaimAvailable(payload map[string]interface{}, channelID string) error {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return nil
	}
	claim, _ := data["claim"].(map[string]interface{})
	claimID := jsonvalue.String(claim["id"])

	if channelID == "" && claim != nil {
		channelID = stringValue(claim["channel_id"])
	}
	if channelID == "" {
		channelID = stringValue(data["channel_id"])
	}
	if channelID == "" {
		if balanceID := jsonvalue.At(data, "balance.channel_id"); balanceID != nil {
			channelID = stringValue(balanceID)
		}
	}
	if channelID == "" && len(p.streamerMap) == 1 {
		for id := range p.streamerMap {
			channelID = id
		}
	}
	streamer := p.streamerMap[channelID]
	if streamer == nil || claimID == "" {
		if p.anonymizeLogs() {
			p.logger.Errorf("claim-available ignored: [redacted]")
		} else {
			p.logger.Errorf("claim-available ignored: channel=%s claim=%s streamer=%v", channelID, claimID, streamer != nil)
			p.deepDebugf("claim-available payload: %v", payload)
		}
		return nil
	}
	// p.logger.EmojiPrintf(":gift:", "Claim bonus for %s (claim %s, channel %s)", streamer.Username, claimID, channelID)
	if err := p.twitch.ClaimBonus(streamer, claimID); err != nil {
		if p.anonymizeLogs() {
			p.logger.Errorf("claim bonus %s: %v", p.streamerName(streamer), err)
		} else {
			p.logger.Errorf("claim bonus %s (channel %s): %v", streamer.Username, channelID, err)
		}
		return nil
	}
	// p.logger.Printf("Claim bonus success for %s (claim %s)", streamer.Username, claimID)
	return nil
}

// ? pollPendingClaims proactively checks each streamer for any outstanding community-point bonuses.
func (p *Client) pollPendingClaims(stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Hour)
	defer ticker.Stop()
	p.checkPendingClaims()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			p.checkPendingClaims()
		}
	}
}

func (p *Client) checkPendingClaims() {
	for _, streamer := range p.streamers {
		if streamer == nil || streamer.Username == "" || streamer.ChannelID == "" {
			continue
		}
		if p.anonymizeLogs() {
			p.debugf("Checking pending bonus for %s", p.streamerName(streamer))
		} else {
			p.debugf("Checking pending bonus for %s (%s)", streamer.Username, streamer.ChannelID)
		}
		if _, err := p.twitch.LoadChannelPointsContext(streamer); err != nil {
			p.logger.Errorf("pending bonus check %s: %v", p.streamerName(streamer), err)
		}
	}
}
