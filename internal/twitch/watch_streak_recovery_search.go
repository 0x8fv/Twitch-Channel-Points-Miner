package twitch

import (
	"fmt"
	"slices"

	"TwitchChannelPointsMiner/internal/jsonvalue"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

// listing returns the operation that lists the streamer's clips for a period
// ("LAST_DAY", "LAST_WEEK") or their vods ("videos"), and the key its results sit under.
func (r *streakRecovery) listing(source string) (mode string, op twitchapi.GQLPersistedOperation) {
	if source == "videos" {
		op = twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.FilterableVideoTowerVideos)
		if op.Variables == nil {
			op.Variables = map[string]interface{}{}
		}
		op.Variables["channelOwnerLogin"] = r.streamer.Username
		return "videos", op
	}
	op = twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.ClipsCardsUser)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["login"] = r.streamer.Username
	op.Variables["criteria"] = map[string]interface{}{"filter": source}
	return "clips", op
}

// watchMissedFrom pages through one listing and watches every clip or vod that
// belongs to a missed stream. It stops as soon as the streak is settled.
func (r *streakRecovery) watchMissedFrom(source string) (saved, settled bool, err error) {
	mode, op := r.listing(source)
	hasNext := true
	cursor := ""
	for hasNext {
		op.Variables["cursor"] = cursor
		resp, err := r.client.PostGQL(op)
		if err != nil {
			return false, false, err
		}
		respdata := jsonvalue.At(resp, "data.user."+mode)
		if respdata == nil {
			return false, false, fmt.Errorf("missing %s data for channel %s", mode, r.streamer.ChannelID)
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
			if !slices.Contains(r.missed, id) {
				continue
			}
			if mode == "clips" {
				saved, settled, err = r.watchClip(node)
			} else {
				saved, settled, err = r.watchVOD(node)
			}
			if err != nil || settled {
				return saved, settled, err
			}
		}
		hasNext, _ = pageInfo["hasNextPage"].(bool)
		if hasNext && len(edges) == 0 {
			r.logf("empty %s edges but hasNextPage == true with cursor %s", mode, op.Variables["cursor"])
			hasNext = false
		}
	}
	return false, false, nil
}
