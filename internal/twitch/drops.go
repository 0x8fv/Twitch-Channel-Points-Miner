package twitch

import (
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/jsonvalue"
	"TwitchChannelPointsMiner/internal/random"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

type ClaimedDrop struct {
	RewardName    string
	CampaignName  string
	CurrentValue  int
	RequiredValue int
}

type DropStatus struct {
	DropInstanceID string
	RewardName     string
	CampaignName   string
	GameName       string
	ChannelName    string
	CurrentValue   int
	RequiredValue  int
	Claimable      bool
	Claimed        bool
}

// ? ClaimDrop claims a single drop instance.
func (t *Client) ClaimDrop(dropInstanceID string) (bool, error) {
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.DropsPageClaimDropRewards)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{"dropInstanceID": dropInstanceID}
	resp, err := t.PostGQL(op)
	if err != nil {
		return false, err
	}
	status := jsonvalue.At(resp, "data.claimDropRewards.status")
	switch status {
	case "DROP_INSTANCE_ALREADY_CLAIMED", "ELIGIBLE_FOR_ALL":
		return true, nil
	default:
		return false, nil
	}
}

func (t *Client) ClaimAllDropsFromInventory() ([]ClaimedDrop, error) {
	claimedDrops, _, err := t.ClaimAllDropsFromInventoryWithStatuses()
	return claimedDrops, err
}

func (t *Client) ClaimAllDropsFromInventoryWithStatuses() ([]ClaimedDrop, []DropStatus, error) {
	var claimedDrops []ClaimedDrop
	inv := t.inventory()
	if inv == nil {
		return claimedDrops, nil, nil
	}
	statuses := dropStatusesFromInventory(inv)
	var claimErr error
	for _, status := range statuses {
		if status.DropInstanceID == "" || status.Claimed {
			continue
		}
		ok, err := t.ClaimDrop(status.DropInstanceID)
		if err != nil {
			if claimErr == nil {
				claimErr = err
			}
			continue
		}
		if ok {
			claimedDrops = append(claimedDrops, ClaimedDrop{
				RewardName:    status.RewardName,
				CampaignName:  status.CampaignName,
				CurrentValue:  status.CurrentValue,
				RequiredValue: status.RequiredValue,
			})
			time.Sleep(time.Duration(random.Int(5, 10)) * time.Second)
		}
	}
	return claimedDrops, statuses, claimErr
}

// ? Fetch campaign IDs for a streamer if drops enabled.
func (t *Client) CampaignIDsForStreamer(streamer *domain.Streamer) ([]string, error) {
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.DropsHighlightServiceAvailable)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelID"] = streamer.ChannelID
	resp, err := t.PostGQL(op)
	if err != nil {
		return nil, err
	}
	cams := jsonvalue.At(resp, "data.channel.viewerDropCampaigns")
	if cams == nil {
		return []string{}, nil
	}
	arr := cams.([]interface{})
	var res []string
	for _, c := range arr {
		if id, ok := c.(map[string]interface{})["id"].(string); ok {
			res = append(res, id)
		}
	}
	return res, nil
}

func (t *Client) inventory() map[string]interface{} {
	resp, err := t.PostGQL(twitchapi.GQLOperations.Inventory)
	if err != nil || resp == nil {
		return nil
	}
	inv := jsonvalue.At(resp, "data.currentUser.inventory")
	if inv == nil {
		return nil
	}
	return inv.(map[string]interface{})
}
