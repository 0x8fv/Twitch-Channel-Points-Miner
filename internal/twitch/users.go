package twitch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/jsonvalue"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

func (t *Client) GetUserByID(channelID string) (*TwitchUserIdentity, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("missing channel id: %w", ErrChannelNotFound)
	}
	if t == nil || t.client == nil || t.twitchLogin == nil {
		return nil, fmt.Errorf("missing twitch client")
	}

	endpoint := "https://api.twitch.tv/helix/users?id=" + url.QueryEscape(channelID)
	req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+t.twitchLogin.AuthToken())
	req.Header.Set("Client-Id", twitchapi.ClientID)
	if t.userAgent != "" {
		req.Header.Set("User-Agent", t.userAgent)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("helix user lookup failed: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out helixUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("channel id %s not found: %w", channelID, ErrChannelNotFound)
	}

	user := out.Data[0]
	return &TwitchUserIdentity{
		ID:          user.ID,
		Login:       strings.ToLower(strings.TrimSpace(user.Login)),
		DisplayName: strings.TrimSpace(user.DisplayName),
	}, nil
}

func (t *Client) GetChannelID(login string) (string, error) {
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.GetIDFromLogin)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["login"] = strings.ToLower(login)
	resp, err := t.PostGQL(op)
	if err != nil {
		return "", err
	}
	user := jsonvalue.At(resp, "data.user.id")
	if s, ok := user.(string); ok && s != "" {
		return s, nil
	}
	return "", fmt.Errorf("user %s not found", login)
}

func (t *Client) GetFollowers(limit int, order domain.FollowersOrder) ([]string, error) {
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.ChannelFollows)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["limit"] = limit
	op.Variables["order"] = string(order)
	hasNext := true
	cursor := ""
	var follows []string

	for hasNext {
		op.Variables["cursor"] = cursor
		resp, err := t.PostGQL(op)
		if err != nil {
			return nil, err
		}
		followsResp := jsonvalue.At(resp, "data.user.follows")
		if followsResp == nil {
			break
		}
		data := followsResp.(map[string]interface{})
		edges, _ := data["edges"].([]interface{})
		pageInfo, _ := data["pageInfo"].(map[string]interface{})
		cursor = ""
		for _, edge := range edges {
			e := edge.(map[string]interface{})
			node, _ := e["node"].(map[string]interface{})
			login, _ := node["login"].(string)
			follows = append(follows, strings.ToLower(login))
			if c, ok := e["cursor"].(string); ok {
				cursor = c
			}
		}
		hasNext, _ = pageInfo["hasNextPage"].(bool)
	}
	return follows, nil
}
