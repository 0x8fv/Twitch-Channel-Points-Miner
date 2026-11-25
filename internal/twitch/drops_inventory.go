package twitch

import (
	"strings"
)

func campaignNameFromInventory(campaign map[string]interface{}) string {
	if campaign == nil {
		return ""
	}
	if name := mapStringValue(campaign, "name", "displayName", "gameDisplayName"); name != "" {
		return name
	}
	if name, _ := navigate(campaign, "game.displayName").(string); name != "" {
		return name
	}
	if name, _ := navigate(campaign, "game.name").(string); name != "" {
		return name
	}
	return ""
}

func rewardNameFromInventory(drop map[string]interface{}) string {
	if drop == nil {
		return ""
	}
	if benefit, ok := drop["benefit"].(map[string]interface{}); ok {
		if name := mapStringValue(benefit, "name", "displayName"); name != "" {
			return name
		}
	}
	if name := mapStringValue(drop, "name", "displayName"); name != "" {
		return name
	}
	if name, _ := navigate(drop, "benefit.edges.0.node.name").(string); name != "" {
		return name
	}
	return ""
}

func dropStatusesFromInventory(inv map[string]interface{}) []DropStatus {
	if inv == nil {
		return nil
	}
	active, _ := inv["dropCampaignsInProgress"].([]interface{})
	statuses := make([]DropStatus, 0)
	for _, c := range active {
		campaign, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		campaignName := campaignNameFromInventory(campaign)
		gameName := gameNameFromInventory(campaign)
		channelName := channelNameFromInventory(campaign)
		td, _ := campaign["timeBasedDrops"].([]interface{})
		for _, d := range td {
			inner, ok := d.(map[string]interface{})
			if !ok {
				continue
			}
			self, _ := inner["self"].(map[string]interface{})
			if self == nil {
				continue
			}
			claimed, _ := self["isClaimed"].(bool)
			current, required := dropProgress(inner, self)
			statuses = append(statuses, DropStatus{
				DropInstanceID: mapStringValue(self, "dropInstanceID"),
				RewardName:     rewardNameFromInventory(inner),
				CampaignName:   campaignName,
				GameName:       gameName,
				ChannelName:    channelName,
				CurrentValue:   current,
				RequiredValue:  required,
				Claimable:      dropClaimable(inner, self, current, required, claimed),
				Claimed:        claimed,
			})
		}
	}
	return statuses
}

func gameNameFromInventory(campaign map[string]interface{}) string {
	if campaign == nil {
		return ""
	}
	if game, ok := campaign["game"].(map[string]interface{}); ok {
		if name := mapStringValue(game, "displayName", "name"); name != "" {
			return name
		}
	}
	return mapStringValue(campaign, "gameDisplayName", "gameName")
}

func channelNameFromInventory(campaign map[string]interface{}) string {
	if campaign == nil {
		return ""
	}
	if name := mapStringValue(campaign, "channelLogin", "channelName", "channelDisplayName"); name != "" {
		return name
	}
	for _, path := range []string{"channel.login", "channel.name", "channel.displayName"} {
		if name, _ := navigate(campaign, path).(string); strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	for _, key := range []string{"channels", "eligibleChannels", "allowChannels", "allowedChannels"} {
		if name := channelNameFromList(campaign[key]); name != "" {
			return name
		}
	}
	return ""
}

func channelNameFromList(raw interface{}) string {
	channels, ok := raw.([]interface{})
	if !ok {
		return ""
	}
	for _, item := range channels {
		channel, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if name := mapStringValue(channel, "login", "name", "displayName"); name != "" {
			return name
		}
		for _, path := range []string{"channel.login", "channel.name", "channel.displayName"} {
			if name, _ := navigate(channel, path).(string); strings.TrimSpace(name) != "" {
				return strings.TrimSpace(name)
			}
		}
	}
	return ""
}

func dropClaimable(drop map[string]interface{}, self map[string]interface{}, current, required int, claimed bool) bool {
	if claimed {
		return false
	}
	for _, data := range []map[string]interface{}{self, drop} {
		for _, key := range []string{"isClaimable", "claimable", "canClaim", "isClaimAvailable", "isClaimReady"} {
			if val, ok := data[key].(bool); ok {
				return val
			}
		}
	}
	return required > 0 && current >= required
}

func dropProgress(drop map[string]interface{}, self map[string]interface{}) (int, int) {
	current := mapIntValue(self, "currentMinutesWatched", "currentSecondsWatched", "currentProgress", "currentAmount")
	required := mapIntValue(drop, "requiredMinutesWatched", "requiredSecondsWatched", "requiredProgress", "requiredAmount")
	if required == 0 {
		required = mapIntValue(self, "requiredMinutesWatched", "requiredSecondsWatched")
	}
	return current, required
}
