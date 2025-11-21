package classes

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/utils"
)

var ErrStreamerOffline = errors.New("streamer offline")

type Twitch struct {
	userAgent      string
	deviceID       string
	clientSession  string
	clientVersion  string
	twitchLogin    *TwitchLogin
	client         *http.Client
	twilightRegexp *regexp.Regexp
	settingsRegex  *regexp.Regexp
	spadeRegex     *regexp.Regexp
}

type ClaimedDrop struct {
	RewardName    string
	CampaignName  string
	CurrentValue  int
	RequiredValue int
}

func NewTwitch(username, userAgent, password string) (*Twitch, error) {
	deviceID := randomString(32)
	login, err := NewTwitchLogin(constants.ClientID, deviceID, username, userAgent, password)
	if err != nil {
		return nil, err
	}

	return &Twitch{
		userAgent:      userAgent,
		deviceID:       deviceID,
		clientSession:  randomString(32),
		clientVersion:  constants.ClientVersion,
		twitchLogin:    login,
		client:         login.Client(),
		twilightRegexp: regexp.MustCompile(`window\.__twilightBuildID\s*=\s*"([0-9a-fA-F\-]{36})"`),
		settingsRegex:  regexp.MustCompile(`(https://static\.twitchcdn\.net/config/settings.*?\.js|https://assets\.twitch\.tv/config/settings.*?\.js)`),
		spadeRegex:     regexp.MustCompile(`"spade_url":"(.*?)"`),
	}, nil
}

func (t *Twitch) Login(username string) error {
	cookiesPath := filepath.Join("cookies", fmt.Sprintf("%s.json", username))
	if err := t.twitchLogin.Login(cookiesPath); err != nil {
		return err
	}
	return nil
}

// ? UpdateClientVersion refreshes the Twitch build id used for GQL calls.
func (t *Twitch) UpdateClientVersion() string {
	resp, err := t.client.Get(constants.URL)
	if err != nil {
		return t.clientVersion
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	m := t.twilightRegexp.FindStringSubmatch(string(body))
	if len(m) > 1 {
		t.clientVersion = m[1]
	}
	return t.clientVersion
}

func (t *Twitch) PostGQL(payload interface{}) (map[string]interface{}, error) {
	if payload == nil {
		return map[string]interface{}{}, nil
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, constants.GQLOperations.URL, bytes.NewReader(body))
	req.Header.Set("Authorization", fmt.Sprintf("OAuth %s", t.twitchLogin.AuthToken()))
	req.Header.Set("Client-Id", constants.ClientID)
	req.Header.Set("Client-Session-Id", t.clientSession)
	req.Header.Set("Client-Version", t.UpdateClientVersion())
	req.Header.Set("User-Agent", t.userAgent)
	req.Header.Set("X-Device-Id", t.deviceID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *Twitch) GetChannelID(login string) (string, error) {
	op := constants.GQLOperations.GetIDFromLogin
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["login"] = strings.ToLower(login)
	resp, err := t.PostGQL(op)
	if err != nil {
		return "", err
	}
	user := navigate(resp, "data.user.id")
	if s, ok := user.(string); ok && s != "" {
		return s, nil
	}
	return "", fmt.Errorf("user %s not found", login)
}

func (t *Twitch) GetFollowers(limit int, order entities.FollowersOrder) ([]string, error) {
	op := constants.GQLOperations.ChannelFollows
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
		followsResp := navigate(resp, "data.user.follows")
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

// ? LoadChannelPointsContext fetches points and claims any bonuses.
func (t *Twitch) LoadChannelPointsContext(streamer *entities.Streamer) (int, error) {
	op := constants.GQLOperations.ChannelPointsContext
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelLogin"] = streamer.Username
	resp, err := t.PostGQL(op)
	if err != nil {
		return 0, err
	}
	channel := navigate(resp, "data.community.channel")
	if channel == nil {
		return 0, fmt.Errorf("channel missing for %s", streamer.Username)
	}
	self := navigate(resp, "data.community.channel.self.communityPoints")
	pointsData, _ := self.(map[string]interface{})
	balance := int(fromFloat(pointsData["balance"]))
	streamer.ChannelPoints = balance
	if available := navigate(resp, "data.community.channel.self.communityPoints.availableClaim"); available != nil {
		if claimID, ok := navigate(available, "id").(string); ok && claimID != "" {
			_ = t.ClaimBonus(streamer, claimID)
		}
	}
	return balance, nil
}

func (t *Twitch) CheckStreamerOnline(streamer *entities.Streamer) (bool, error) {
	_, err := t.streamInfo(streamer.Username)
	if err == ErrStreamerOffline {
		streamer.IsOnline = false
		streamer.OfflineAt = time.Now()
		return false, nil
	}
	if err != nil {
		return streamer.IsOnline, err
	}
	streamer.IsOnline = true
	streamer.OnlineAt = time.Now()
	return true, nil
}

func (t *Twitch) streamInfo(username string) (map[string]interface{}, error) {
	op := constants.GQLOperations.VideoPlayerStreamInfoOverlay
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channel"] = strings.ToLower(username)
	resp, err := t.PostGQL(op)
	if err != nil {
		return nil, err
	}
	stream := navigate(resp, "data.user.stream")
	if stream == nil {
		return nil, ErrStreamerOffline
	}
	user := navigate(resp, "data.user")
	if user == nil {
		return nil, fmt.Errorf("missing user data for %s", username)
	}
	return user.(map[string]interface{}), nil
}

// ? UpdateStream refreshes metadata and payload required for minute-watched events.
func (t *Twitch) UpdateStream(streamer *entities.Streamer) error {
	if streamer.Stream == nil {
		streamer.Stream = entities.NewStream()
	}
	if !streamer.Stream.UpdateRequired() {
		return nil
	}
	info, err := t.streamInfo(streamer.Username)
	if err != nil {
		return err
	}
	streamData, _ := info["stream"].(map[string]interface{})
	broadcastSettings, _ := info["broadcastSettings"].(map[string]interface{})
	if streamData == nil || broadcastSettings == nil {
		return ErrStreamerOffline
	}
	title, _ := broadcastSettings["title"].(string)
	game, _ := broadcastSettings["game"].(map[string]interface{})
	tagsIface, _ := streamData["tags"].([]interface{})
	viewers := int(fromFloat(streamData["viewersCount"]))
	streamer.Stream.Update(
		fmt.Sprint(streamData["id"]),
		title,
		game,
		convertTags(tagsIface),
		viewers,
		constants.DropID,
	)

	eventProps := map[string]interface{}{
		"channel_id":   streamer.ChannelID,
		"broadcast_id": streamer.Stream.BroadcastID,
		"user_id":      t.twitchLogin.UserID(),
		"player":       "site",
		"live":         true,
		"channel":      streamer.Username,
	}
	if name, ok := game["name"].(string); ok && name != "" && streamer.Settings.ClaimDrops {
		eventProps["game"] = name
		if id, ok := game["id"].(string); ok {
			eventProps["game_id"] = id
		}
		campaigns, err := t.CampaignIDsForStreamer(streamer)
		if err == nil {
			streamer.Stream.CampaignIDs = campaigns
		}
	}
	streamer.Stream.Payload = []map[string]interface{}{
		{
			"event":      "minute-watched",
			"properties": eventProps,
		},
	}
	return nil
}

func (t *Twitch) GetSpadeURL(streamer *entities.Streamer) error {
	if streamer.Stream == nil {
		streamer.Stream = entities.NewStream()
	}
	headers := map[string]string{
		"User-Agent": utils.UserAgents["Linux"]["FIREFOX"],
	}
	pageURL := streamer.StreamerURL
	if pageURL == "" {
		pageURL = fmt.Sprintf("%s/%s", constants.URL, streamer.Username)
	}
	mainReq, _ := http.NewRequest(http.MethodGet, pageURL, nil)
	for k, v := range headers {
		mainReq.Header.Set(k, v)
	}
	resp, err := t.client.Do(mainReq)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	match := t.settingsRegex.FindStringSubmatch(string(body))
	if len(match) < 2 {
		return errors.New("settings script not found")
	}
	settingsReq, _ := http.NewRequest(http.MethodGet, match[1], nil)
	for k, v := range headers {
		settingsReq.Header.Set(k, v)
	}
	settingsResp, err := t.client.Do(settingsReq)
	if err != nil {
		return err
	}
	defer settingsResp.Body.Close()
	settingsBody, err := io.ReadAll(settingsResp.Body)
	if err != nil {
		return err
	}
	spade := t.spadeRegex.FindStringSubmatch(string(settingsBody))
	if len(spade) < 2 {
		return errors.New("spade url not found")
	}
	streamer.Stream.SpadeURL = spade[1]
	return nil
}

func (t *Twitch) SendMinuteWatched(streamer *entities.Streamer) error {
	if err := t.UpdateStream(streamer); err != nil {
		return err
	}
	if streamer.Stream.SpadeURL == "" {
		if err := t.GetSpadeURL(streamer); err != nil {
			return err
		}
	}
	streamer.Stream.UpdateMinuteWatched()
	payload, err := streamer.Stream.EncodePayload()
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("data", payload["data"])
	req, _ := http.NewRequest(http.MethodPost, streamer.Stream.SpadeURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", t.userAgent)
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		streamer.Stream.UpdateMinuteWatched()
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("minute watched failed: %d %s", resp.StatusCode, string(body))
}

// ? ClaimBonus redeems the community points bonus (blue chest).
func (t *Twitch) ClaimBonus(streamer *entities.Streamer, claimID string) error {
	op := constants.GQLOperations.ClaimCommunityPoints
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{
		"channelID": streamer.ChannelID,
		"claimID":   claimID,
	}
	_, err := t.PostGQL(op)
	return err
}

// ? ClaimDrop claims a single drop instance.
func (t *Twitch) ClaimDrop(dropInstanceID string) (bool, error) {
	op := constants.GQLOperations.DropsPageClaimDropRewards
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{"dropInstanceID": dropInstanceID}
	resp, err := t.PostGQL(op)
	if err != nil {
		return false, err
	}
	status := navigate(resp, "data.claimDropRewards.status")
	switch status {
	case "DROP_INSTANCE_ALREADY_CLAIMED", "ELIGIBLE_FOR_ALL":
		return true, nil
	default:
		return false, nil
	}
}

func (t *Twitch) ClaimAllDropsFromInventory() ([]ClaimedDrop, error) {
	var claimedDrops []ClaimedDrop
	inv := t.inventory()
	if inv == nil {
		return claimedDrops, nil
	}
	active, _ := inv["dropCampaignsInProgress"].([]interface{})
	var claimErr error
	for _, c := range active {
		campaign, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		campaignName := campaignNameFromInventory(campaign)
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
			alreadyClaimed, _ := self["isClaimed"].(bool)
			id, _ := self["dropInstanceID"].(string)
			if id == "" || alreadyClaimed {
				continue
			}
			rewardName := rewardNameFromInventory(inner)
			current, required := dropProgress(inner, self)
			ok, err := t.ClaimDrop(id)
			if err != nil {
				if claimErr == nil {
					claimErr = err
				}
				continue
			}
			if ok {
				claimedDrops = append(claimedDrops, ClaimedDrop{
					RewardName:    rewardName,
					CampaignName:  campaignName,
					CurrentValue:  current,
					RequiredValue: required,
				})
				time.Sleep(time.Duration(randomInt(5, 10)) * time.Second)
			}
		}
	}
	return claimedDrops, claimErr
}

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

func dropProgress(drop map[string]interface{}, self map[string]interface{}) (int, int) {
	current := mapIntValue(self, "currentMinutesWatched", "currentSecondsWatched", "currentProgress", "currentAmount")
	required := mapIntValue(drop, "requiredMinutesWatched", "requiredSecondsWatched", "requiredProgress", "requiredAmount")
	if required == 0 {
		required = mapIntValue(self, "requiredMinutesWatched", "requiredSecondsWatched")
	}
	return current, required
}

func mapStringValue(data map[string]interface{}, keys ...string) string {
	if data == nil {
		return ""
	}
	for _, key := range keys {
		if val, ok := data[key].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

func mapIntValue(data map[string]interface{}, keys ...string) int {
	if data == nil {
		return 0
	}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			if intVal := int(fromFloat(val)); intVal != 0 {
				return intVal
			}
		}
	}
	return 0
}

// ? Fetch campaign IDs for a streamer if drops enabled.
func (t *Twitch) CampaignIDsForStreamer(streamer *entities.Streamer) ([]string, error) {
	op := constants.GQLOperations.DropsHighlightServiceAvailable
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelID"] = streamer.ChannelID
	resp, err := t.PostGQL(op)
	if err != nil {
		return nil, err
	}
	cams := navigate(resp, "data.channel.viewerDropCampaigns")
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

func (t *Twitch) inventory() map[string]interface{} {
	resp, err := t.PostGQL(constants.GQLOperations.Inventory)
	if err != nil || resp == nil {
		return nil
	}
	inv := navigate(resp, "data.currentUser.inventory")
	if inv == nil {
		return nil
	}
	return inv.(map[string]interface{})
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, length)
	for i := range buf {
		nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		buf[i] = charset[nBig.Int64()]
	}
	return string(buf)
}

func randomInt(min, max int) int {
	if max <= min {
		return min
	}
	nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return min + int(nBig.Int64())
}

func fromFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func navigate(data interface{}, path string) interface{} {
	if data == nil {
		return nil
	}
	current := data
	parts := strings.Split(path, ".")
	for _, p := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[p]
		if current == nil {
			return nil
		}
	}
	return current
}

func convertTags(tags []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tags))
	for _, tag := range tags {
		if m, ok := tag.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}
