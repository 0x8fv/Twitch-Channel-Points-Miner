package twitch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"TwitchChannelPointsMiner/internal/twitchapi"
)

func (t *Client) PostGQL(payload interface{}) (map[string]interface{}, error) {
	return t.postGQLWithHeaders(payload, nil)
}

func (t *Client) PostGQLDecode(payload interface{}, out interface{}) error {
	return t.postGQLDecodeWithHeaders(payload, nil, out)
}

func (t *Client) postGQLWithHeaders(payload interface{}, extraHeaders map[string]string) (map[string]interface{}, error) {
	if payload == nil {
		return map[string]interface{}{}, nil
	}
	var result map[string]interface{}
	if err := t.postGQLDecodeWithHeaders(payload, extraHeaders, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *Client) postGQLDecodeWithHeaders(payload interface{}, extraHeaders map[string]string, out interface{}) error {
	if payload == nil {
		return nil
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, twitchapi.GQLOperations.URL, bytes.NewReader(body))
	req.Header.Set("Authorization", fmt.Sprintf("OAuth %s", t.twitchLogin.AuthToken()))
	req.Header.Set("Client-Id", twitchapi.ClientID)
	req.Header.Set("Client-Session-Id", t.clientSession)
	req.Header.Set("Client-Version", t.UpdateClientVersion())
	req.Header.Set("User-Agent", t.userAgent)
	req.Header.Set("X-Device-Id", t.deviceID)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		t.debugf("GQL request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	t.debugf("GQL %s | Status %d", operationLabel(payload, true), resp.StatusCode)

	deepEnabled := false
	if t.anonymizer == nil || !t.anonymizer.Enabled() {
		if deep, ok := t.logger.(deepDebugLogger); ok && deep.DeepDebugEnabled() {
			deepEnabled = true
		}
	}

	if deepEnabled {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		t.deepDebugf(
			"GQL %s | Status %d | Headers: %v | Request: %s | Response: %s",
			operationLabel(payload, false),
			resp.StatusCode,
			req.Header,
			strings.TrimSpace(string(body)),
			strings.TrimSpace(string(respBody)),
		)
		if out == nil {
			return nil
		}
		return json.Unmarshal(respBody, out)
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

func operationLabel(payload interface{}, includeNote bool) string {
	names := operationNames(payload)
	if len(names) == 0 {
		return "unknown"
	}
	if len(names) == 1 {
		name := names[0]
		if includeNote {
			if note := operationNote(name); note != "" {
				return fmt.Sprintf("%s (%s)", name, note)
			}
		}
		return name
	}
	display := names
	if len(display) > 3 {
		display = display[:3]
	}
	label := strings.Join(display, ", ")
	if len(names) > len(display) {
		label = fmt.Sprintf("%s +%d", label, len(names)-len(display))
	}
	return fmt.Sprintf("batch[%s]", label)
}

func operationNames(payload interface{}) []string {
	var names []string
	seen := map[string]struct{}{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	switch p := payload.(type) {
	case twitchapi.GQLPersistedOperation:
		add(p.OperationName)
		return names
	case *twitchapi.GQLPersistedOperation:
		if p != nil {
			add(p.OperationName)
		}
		return names
	case []twitchapi.GQLPersistedOperation:
		for i := range p {
			add(p[i].OperationName)
		}
		return names
	case []*twitchapi.GQLPersistedOperation:
		for _, op := range p {
			if op != nil {
				add(op.OperationName)
			}
		}
		return names
	case map[string]interface{}:
		if name, ok := p["operationName"].(string); ok {
			add(name)
		}
		return names
	case []interface{}:
		for _, raw := range p {
			if m, ok := raw.(map[string]interface{}); ok {
				if name, ok := m["operationName"].(string); ok {
					add(name)
				}
			}
		}
		return names
	}

	// ? fallback for other struct-like payloads
	val := reflect.ValueOf(payload)
	for val.IsValid() && val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return names
		}
		val = val.Elem()
	}
	if !val.IsValid() {
		return names
	}
	if val.Kind() == reflect.Struct {
		field := val.FieldByName("OperationName")
		if field.IsValid() && field.Kind() == reflect.String {
			add(field.String())
		}
		return names
	}
	if val.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			item := val.Index(i)
			for item.IsValid() && item.Kind() == reflect.Pointer {
				if item.IsNil() {
					break
				}
				item = item.Elem()
			}
			if !item.IsValid() || item.Kind() != reflect.Struct {
				continue
			}
			field := item.FieldByName("OperationName")
			if field.IsValid() && field.Kind() == reflect.String {
				add(field.String())
			}
		}
	}
	return names
}

func operationNote(name string) string {
	switch name {
	case "WithIsStreamLiveQuery":
		return "check live"
	case "VideoPlayerStreamInfoOverlayChannel":
		return "stream info"
	case "RewardList":
		return "reward list"
	case "PlaybackAccessToken":
		return "playback token"
	case "ChannelPointsContext":
		return "points/bonus"
	case "ClaimCommunityPoints":
		return "claim bonus"
	case "CommunityMomentCallout_Claim":
		return "claim moment"
	case "JoinRaid":
		return "join raid"
	case "DropsPage_ClaimDropRewards":
		return "claim drop"
	case "ViewerDropsDashboard":
		return "drops dashboard"
	case "DropCampaignDetails":
		return "drop details"
	case "DropsHighlightService_AvailableDrops":
		return "available drops"
	case "Inventory":
		return "inventory"
	case "GetIDFromLogin":
		return "resolve id"
	case "ChannelFollows":
		return "followers"
	case "PersonalSections":
		return "followed streams"
	case "UserPointsContribution":
		return "community goal"
	case "ContributeCommunityPointsCommunityGoal":
		return "contribute goal"
	case "MakePrediction":
		return "make prediction"
	case "ModViewChannelQuery":
		return "mod view"
	case "ClipsCards__User":
		return "clips"
	case "FilterableVideoTower_Videos":
		return "vods"
	default:
		return ""
	}
}
