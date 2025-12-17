package classes

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/privacy"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/utils"
)

var ErrStreamerOffline = errors.New("streamer offline")

type debugLogger interface {
	Debugf(format string, args ...interface{})
	DebugEnabled() bool
	Printf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	EmojiPrintf(emoji, format string, args ...interface{})
}

type deepDebugLogger interface {
	DeepDebugf(format string, args ...interface{})
	DeepDebugEnabled() bool
}

type Twitch struct {
	userAgent      string
	deviceID       string
	clientSession  string
	clientVersion  string
	versionMu      sync.Mutex
	versionFetched time.Time
	versionTTL     time.Duration
	twitchLogin    *TwitchLogin
	client         *http.Client
	twilightRegexp *regexp.Regexp
	settingsRegex  *regexp.Regexp
	spadeRegex     *regexp.Regexp
	logger         debugLogger
	anonymizer     *privacy.Anonymizer
	onGameChange   func(streamer *entities.Streamer, previous, current string)
}

type ClaimedDrop struct {
	RewardName    string
	CampaignName  string
	CurrentValue  int
	RequiredValue int
}

func NewTwitch(username, userAgent, password string, logger debugLogger, anonymizer *privacy.Anonymizer) (*Twitch, error) {
	deviceID := randomString(32)
	login, err := NewTwitchLogin(constants.ClientID, deviceID, username, userAgent, password)
	if err != nil {
		return nil, err
	}

	return &Twitch{
		userAgent:      userAgent,
		deviceID:       deviceID,
		clientSession:  randomHex(8),
		clientVersion:  constants.ClientVersion,
		versionTTL:     10 * time.Hour,
		twitchLogin:    login,
		client:         login.Client(),
		twilightRegexp: regexp.MustCompile(`window\.__twilightBuildID\s*=\s*"([0-9a-fA-F\-]{36})"`),
		settingsRegex:  regexp.MustCompile(`(https://static\.twitchcdn\.net/config/settings.*?\.js|https://assets\.twitch\.tv/config/settings.*?\.js)`),
		spadeRegex:     regexp.MustCompile(`"spade_url":"(.*?)"`),
		logger:         logger,
		anonymizer:     anonymizer,
	}, nil
}

// ? SetGameChangeHandler registers a callback fired whenever stream game metadata changes.
func (t *Twitch) SetGameChangeHandler(handler func(streamer *entities.Streamer, previous, current string)) {
	t.onGameChange = handler
}

func (t *Twitch) Login(username string) error {
	cookiesPath := filepath.Join("cookies", fmt.Sprintf("%s.json", username))
	if err := t.twitchLogin.Login(cookiesPath); err != nil {
		return err
	}
	return nil
}

func (t *Twitch) ChatToken() string {
	if t == nil || t.twitchLogin == nil {
		return ""
	}
	return t.twitchLogin.AuthToken()
}

func (t *Twitch) debugf(format string, args ...interface{}) {
	if t.logger != nil && t.logger.DebugEnabled() {
		t.logger.Debugf(format, args...)
	}
}

func (t *Twitch) deepDebugf(format string, args ...interface{}) {
	if t == nil {
		return
	}
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		return
	}
	if t.logger == nil {
		return
	}
	deep, ok := t.logger.(deepDebugLogger)
	if !ok || !deep.DeepDebugEnabled() {
		return
	}
	deep.DeepDebugf(format, args...)
}

// ? UpdateClientVersion refreshes the Twitch build id used for GQL calls.
func (t *Twitch) UpdateClientVersion() string {
	if t == nil {
		return constants.ClientVersion
	}

	t.versionMu.Lock()
	ttl := t.versionTTL
	if ttl <= 0 {
		ttl = 10 * time.Hour
		t.versionTTL = ttl
	}
	if !t.versionFetched.IsZero() && time.Since(t.versionFetched) < ttl {
		version := t.clientVersion
		t.versionMu.Unlock()
		return version
	}
	t.versionFetched = time.Now()
	version := t.clientVersion
	client := t.client
	t.versionMu.Unlock()

	if client == nil {
		return version
	}

	resp, err := client.Get(constants.URL)
	if err != nil {
		return version
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.debugf("UpdateClientVersion request failed with status %d", resp.StatusCode)
		return version
	}
	m := t.twilightRegexp.FindStringSubmatch(string(body))
	if len(m) > 1 {
		t.versionMu.Lock()
		t.clientVersion = m[1]
		version = t.clientVersion
		t.versionMu.Unlock()
		t.debugf("Client version updated to %s", version)
		return version
	} else {
		t.debugf("UpdateClientVersion: unable to extract build id")
	}
	t.versionMu.Lock()
	version = t.clientVersion
	t.versionMu.Unlock()
	return version
}

func (t *Twitch) PostGQL(payload interface{}) (map[string]interface{}, error) {
	return t.postGQLWithHeaders(payload, nil)
}

func (t *Twitch) PostGQLDecode(payload interface{}, out interface{}) error {
	return t.postGQLDecodeWithHeaders(payload, nil, out)
}

func (t *Twitch) postGQLWithHeaders(payload interface{}, extraHeaders map[string]string) (map[string]interface{}, error) {
	if payload == nil {
		return map[string]interface{}{}, nil
	}
	var result map[string]interface{}
	if err := t.postGQLDecodeWithHeaders(payload, extraHeaders, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *Twitch) postGQLDecodeWithHeaders(payload interface{}, extraHeaders map[string]string, out interface{}) error {
	if payload == nil {
		return nil
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

func (t *Twitch) GetChannelID(login string) (string, error) {
	op := constants.ClonePersistedOperation(constants.GQLOperations.GetIDFromLogin)
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
	op := constants.ClonePersistedOperation(constants.GQLOperations.ChannelFollows)
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
	op := constants.ClonePersistedOperation(constants.GQLOperations.ChannelPointsContext)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelLogin"] = streamer.Username
	var resp gqlChannelPointsContextResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		return 0, err
	}
	channel := resp.Data.Community.Channel
	if channel == nil {
		name := streamer.Username
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			name = t.anonymizer.StreamerName(streamer)
		}
		return 0, fmt.Errorf("channel missing for %s", name)
	}
	pointsData := channel.Self.CommunityPoints
	balance := pointsData.Balance
	streamer.ChannelPoints = balance
	if len(pointsData.ActiveMultipliers) > 0 {
		streamer.ActiveMultipliers = pointsData.ActiveMultipliers
	} else {
		streamer.ActiveMultipliers = nil
	}
	if streamer.Settings.CommunityGoals {
		streamer.CommunityGoals = parseCommunityGoalsJSON(channel.CommunityPointsSettings.Goals)
		t.ContributeToCommunityGoals(streamer)
	}
	if pointsData.AvailableClaim != nil {
		claimID := pointsData.AvailableClaim.ID
		if claimID == "" {
			if t.logger != nil && t.logger.DebugEnabled() {
				name := streamer.Username
				if t.anonymizer != nil && t.anonymizer.Enabled() {
					name = t.anonymizer.StreamerName(streamer)
					t.logger.Debugf("availableClaim present but missing id for %s", name)
				} else {
					t.logger.Debugf("availableClaim present but missing id for %s", name)
				}
			}
			return balance, nil
		}
		if t.logger != nil {
			name := streamer.Username
			if t.anonymizer != nil && t.anonymizer.Enabled() {
				name = t.anonymizer.StreamerName(streamer)
				t.logger.EmojiPrintf(":gift:", "Pending bonus detected for %s", name)
			} else {
				t.logger.EmojiPrintf(":gift:", "Pending bonus detected for %s (claim %s, channel %s)", name, claimID, streamer.ChannelID)
			}
		}
		if err := t.ClaimBonus(streamer, claimID); err != nil {
			if t.logger != nil {
				name := streamer.Username
				if t.anonymizer != nil && t.anonymizer.Enabled() {
					name = t.anonymizer.StreamerName(streamer)
				}
				t.logger.Errorf("claim bonus on context load for %s failed: %v", name, err)
			}
		} else if t.logger != nil {
			name := streamer.Username
			if t.anonymizer != nil && t.anonymizer.Enabled() {
				name = t.anonymizer.StreamerName(streamer)
				t.logger.Printf("Claim bonus success for %s", name)
			} else {
				t.logger.Printf("Claim bonus success for %s (claim %s)", name, claimID)
			}
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

// ? IsStreamLive performs a lightweight live check without refreshing stream metadata.
func (t *Twitch) IsStreamLive(channelID string) (bool, error) {
	if channelID == "" {
		return false, fmt.Errorf("missing channel id")
	}
	op := constants.ClonePersistedOperation(constants.GQLOperations.WithIsStreamLiveQuery)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["id"] = channelID
	var resp gqlIsStreamLiveResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		return false, err
	}
	if resp.Data.User == nil {
		return false, nil
	}
	return resp.Data.User.Stream != nil, nil
}

type streamInfoResult struct {
	StreamID     string
	Title        string
	Game         map[string]interface{}
	Tags         []map[string]interface{}
	ViewersCount int
}

func (t *Twitch) streamInfo(username string) (*streamInfoResult, error) {
	op := constants.ClonePersistedOperation(constants.GQLOperations.VideoPlayerStreamInfoOverlay)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channel"] = strings.ToLower(username)
	var resp gqlStreamInfoOverlayResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		return nil, err
	}
	if resp.Data.User == nil {
		return nil, fmt.Errorf("missing user data for %s", username)
	}
	if resp.Data.User.Stream == nil || resp.Data.User.BroadcastSettings == nil {
		return nil, ErrStreamerOffline
	}
	return &streamInfoResult{
		StreamID:     resp.Data.User.Stream.ID,
		Title:        resp.Data.User.BroadcastSettings.Title,
		Game:         gameToInterfaceMap(resp.Data.User.BroadcastSettings.Game),
		Tags:         tagsToInterfaceMaps(resp.Data.User.Stream.Tags),
		ViewersCount: resp.Data.User.Stream.ViewersCount,
	}, nil
}

// ? UpdateStream refreshes metadata and payload required for minute-watched events.
func (t *Twitch) UpdateStream(streamer *entities.Streamer) error {
	if streamer.Stream == nil {
		streamer.Stream = entities.NewStream()
	}
	if !streamer.Stream.UpdateRequired() {
		return nil
	}
	prevGame := strings.TrimSpace(streamer.Stream.GameName())
	prevBroadcastID := streamer.Stream.BroadcastID
	info, err := t.streamInfo(streamer.Username)
	if err != nil {
		return err
	}
	game := info.Game
	streamer.Stream.Update(
		info.StreamID,
		info.Title,
		game,
		info.Tags,
		info.ViewersCount,
		constants.DropID,
	)
	if prevBroadcastID != "" && prevBroadcastID != streamer.Stream.BroadcastID {
		streamer.Stream.WatchStreakMissing = true
		streamer.Stream.ResetWatchProgress()
	}

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
		// campaigns, hasGameDrops, err := t.CampaignIDsForStreamer(streamer)
		campaigns, err := t.CampaignIDsForStreamer(streamer)
		if err == nil {
			streamer.Stream.CampaignIDs = campaigns
			// streamer.Stream.CampaignsResolved = true
			// streamer.Stream.DropsActive = hasGameDrops
		}
	}
	streamer.Stream.Payload = []map[string]interface{}{
		{
			"event":      "minute-watched",
			"properties": eventProps,
		},
	}
	if t.onGameChange != nil {
		if gameName := strings.TrimSpace(streamer.Stream.GameName()); gameName != "" && gameName != prevGame {
			t.onGameChange(streamer, prevGame, gameName)
		}
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
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		t.debugf("GetSpadeURL main page status %d", resp.StatusCode)
	} else {
		t.debugf("GetSpadeURL main page for %s status %d", streamer.Username, resp.StatusCode)
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
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		t.debugf("GetSpadeURL settings status %d", settingsResp.StatusCode)
	} else {
		t.debugf("GetSpadeURL settings for %s status %d", streamer.Username, settingsResp.StatusCode)
	}
	spade := t.spadeRegex.FindStringSubmatch(string(settingsBody))
	if len(spade) < 2 {
		return errors.New("spade url not found")
	}
	streamer.Stream.SpadeURL = spade[1]
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		t.debugf("Spade URL resolved")
	} else {
		t.debugf("Spade URL for %s resolved to %s", streamer.Username, streamer.Stream.SpadeURL)
	}
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
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		t.debugf("Send minute watched payload")
	} else {
		t.debugf("Send minute watched payload to %s (%s)", streamer.Username, streamer.Stream.SpadeURL)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		t.debugf("Minute watched response: %d", resp.StatusCode)
	} else {
		t.debugf("Minute watched response for %s: %d %s", streamer.Username, resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	if resp.StatusCode == http.StatusNoContent {
		streamer.Stream.UpdateMinuteWatched()
		return nil
	}
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		return fmt.Errorf("minute watched failed: %d", resp.StatusCode)
	}
	return fmt.Errorf("minute watched failed: %d %s", resp.StatusCode, string(bodyBytes))
}

// ? ClaimBonus redeems the community points bonus.
func (t *Twitch) ClaimBonus(streamer *entities.Streamer, claimID string) error {
	if claimID == "" {
		return fmt.Errorf("missing claim id")
	}
	if streamer == nil || streamer.ChannelID == "" {
		return fmt.Errorf("missing streamer channel id")
	}
	return t.claimBonusTV(streamer, claimID)
}

func (t *Twitch) claimBonusTV(streamer *entities.Streamer, claimID string) error {
	if streamer == nil || streamer.ChannelID == "" {
		return fmt.Errorf("missing streamer/channel")
	}
	if claimID == "" {
		return fmt.Errorf("missing claim id")
	}

	op := constants.ClonePersistedOperation(constants.GQLOperations.ClaimCommunityPoints)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{
		"channelID": streamer.ChannelID,
		"claimID":   claimID,
	}

	reqBody, _ := json.Marshal(op)
	req, _ := http.NewRequest(http.MethodPost, constants.GQLOperations.URL, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", fmt.Sprintf("OAuth %s", t.twitchLogin.AuthToken()))
	req.Header.Set("Client-Id", constants.ClientID)
	req.Header.Set("Client-Session-Id", t.clientSession)
	req.Header.Set("Client-Version", t.UpdateClientVersion())
	req.Header.Set("User-Agent", t.userAgent) // ? Android TV UA
	req.Header.Set("X-Device-Id", t.deviceID)
	req.Header.Set("Content-Type", "application/json")
	authToken := t.twitchLogin.AuthToken()
	userID := t.twitchLogin.UserID()
	if authToken != "" && userID != "" {
		req.Header.Set("Cookie", fmt.Sprintf("auth-token=%s; persistent=%s", authToken, userID))
	} else if authToken != "" {
		req.Header.Set("Cookie", fmt.Sprintf("auth-token=%s", authToken))
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("claim bonus request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if t.logger != nil && t.logger.DebugEnabled() {
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			t.logger.Debugf("ClaimCommunityPoints status=%d", resp.StatusCode)
		} else {
			t.logger.Debugf("ClaimCommunityPoints status=%d", resp.StatusCode)
			t.deepDebugf("ClaimCommunityPoints status=%d headers=%v req=%s resp=%s", resp.StatusCode, req.Header, strings.TrimSpace(string(reqBody)), strings.TrimSpace(string(respBody)))
		}
	}

	if resp.StatusCode != http.StatusOK {
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			return fmt.Errorf("claim bonus status %d", resp.StatusCode)
		}
		return fmt.Errorf("claim bonus status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("claim bonus decode: %w", err)
	}

	if gqlErrs, ok := result["errors"]; ok {
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			return fmt.Errorf("claim bonus gql errors")
		}
		return fmt.Errorf("claim bonus gql errors: %v", gqlErrs)
	}
	if status := navigate(result, "data.claimCommunityPoints.status"); status != nil {
		statusStr := strings.ToUpper(fmt.Sprint(status))
		if statusStr != "" && statusStr != "SUCCESS" && statusStr != "ALREADY_CLAIMED" {
			if t.anonymizer != nil && t.anonymizer.Enabled() {
				return fmt.Errorf("claim bonus status %s", statusStr)
			}
			return fmt.Errorf("claim bonus status %s (resp=%v)", statusStr, result)
		}
	}
	if msg := navigate(result, "data.claimCommunityPoints.error.message"); msg != nil {
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			return fmt.Errorf("claim bonus error")
		}
		return fmt.Errorf("claim bonus error: %v (resp=%v)", msg, result)
	}
	return nil
}

// ? ClaimMoment redeems a community moment callout.
func (t *Twitch) ClaimMoment(streamer *entities.Streamer, momentID string) error {
	if momentID == "" {
		return nil
	}
	op := constants.ClonePersistedOperation(constants.GQLOperations.CommunityMomentCalloutClaim)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{"momentID": momentID}
	_, err := t.PostGQL(op)
	return err
}

// ? JoinRaid follows a raid target to mimic viewer behavior.
func (t *Twitch) JoinRaid(streamer *entities.Streamer, raidID string) error {
	if raidID == "" {
		return fmt.Errorf("missing raid id")
	}
	op := constants.ClonePersistedOperation(constants.GQLOperations.JoinRaid)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{"raidID": raidID}
	_, err := t.PostGQL(op)
	return err
}

// ? MakePrediction places a bet for the provided event.
func (t *Twitch) MakePrediction(event *PredictionEvent) error {
	if event == nil || event.Streamer == nil {
		return fmt.Errorf("nil prediction event")
	}
	if event.Decision.Amount < 10 || event.Decision.OutcomeID == "" || event.EventID == "" {
		return fmt.Errorf("invalid prediction decision")
	}
	op := constants.ClonePersistedOperation(constants.GQLOperations.MakePrediction)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{
		"eventID":       event.EventID,
		"outcomeID":     event.Decision.OutcomeID,
		"points":        event.Decision.Amount,
		"transactionID": randomHex(16),
	}
	_, err := t.PostGQL(op)
	return err
}

// ? ClaimDrop claims a single drop instance.
func (t *Twitch) ClaimDrop(dropInstanceID string) (bool, error) {
	op := constants.ClonePersistedOperation(constants.GQLOperations.DropsPageClaimDropRewards)
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

// ? ContributeToCommunityGoals mirrors the site behavior by spending points into active community goals.
func (t *Twitch) ContributeToCommunityGoals(streamer *entities.Streamer) {
	if streamer == nil || !streamer.Settings.CommunityGoals || len(streamer.CommunityGoals) == 0 {
		return
	}
	hasActive := false
	for _, goal := range streamer.CommunityGoals {
		if goal != nil && goal.Status == "STARTED" && goal.IsInStock {
			hasActive = true
			break
		}
	}
	if !hasActive {
		return
	}

	op := constants.ClonePersistedOperation(constants.GQLOperations.UserPointsContribution)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelLogin"] = streamer.Username
	resp, err := t.PostGQL(op)
	if err != nil {
		return
	}
	contribs := navigate(resp, "data.user.channel.self.communityPoints.goalContributions")
	arr, ok := contribs.([]interface{})
	if !ok {
		return
	}
	for _, raw := range arr {
		goalContribution, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		goalData, _ := goalContribution["goal"].(map[string]interface{})
		goalID, _ := goalData["id"].(string)
		if goalID == "" {
			continue
		}
		goal := streamer.CommunityGoals[goalID]
		if goal == nil {
			continue
		}
		userPoints := int(fromFloat(goalContribution["userPointsContributedThisStream"]))
		userLeft := goal.PerStreamUserMaximumContribution - userPoints
		amount := minInt(goal.AmountLeft(), userLeft, streamer.ChannelPoints)
		if amount > 0 {
			_ = t.ContributeToCommunityGoal(streamer, goalID, goal.Title, amount)
		}
	}
}

// ? ContributeToCommunityGoal sends a single contribution transaction.
func (t *Twitch) ContributeToCommunityGoal(streamer *entities.Streamer, goalID, title string, amount int) error {
	if amount <= 0 || goalID == "" {
		return nil
	}
	op := constants.ClonePersistedOperation(constants.GQLOperations.ContributeCommunityPointsCommunityGoal)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{
		"amount":        amount,
		"channelID":     streamer.ChannelID,
		"goalID":        goalID,
		"transactionID": randomHex(16),
	}
	resp, err := t.PostGQL(op)
	if err != nil {
		return err
	}
	if errVal := navigate(resp, "data.contributeCommunityPointsCommunityGoal.error"); errVal != nil {
		if errStr, ok := errVal.(string); ok && errStr != "" {
			return fmt.Errorf("unable to contribute to %s: %s", title, errStr)
		}
	}
	streamer.ChannelPoints -= amount
	if streamer.ChannelPoints < 0 {
		streamer.ChannelPoints = 0
	}
	return nil
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
	op := constants.ClonePersistedOperation(constants.GQLOperations.DropsHighlightServiceAvailable)
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

func parseCommunityGoals(goals interface{}) map[string]*entities.CommunityGoal {
	arr, ok := goals.([]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]*entities.CommunityGoal, len(arr))
	for _, raw := range arr {
		if goalMap, ok := raw.(map[string]interface{}); ok {
			if goal := entities.NewCommunityGoalFromGQL(goalMap); goal != nil && goal.ID != "" {
				result[goal.ID] = goal
			}
		}
	}
	return result
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
	case constants.GQLPersistedOperation:
		add(p.OperationName)
		return names
	case *constants.GQLPersistedOperation:
		if p != nil {
			add(p.OperationName)
		}
		return names
	case []constants.GQLPersistedOperation:
		for i := range p {
			add(p[i].OperationName)
		}
		return names
	case []*constants.GQLPersistedOperation:
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
	default:
		return ""
	}
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

func randomHex(length int) string {
	if length <= 0 {
		return ""
	}
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return randomString(length)
	}
	return hex.EncodeToString(bytes)
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
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
