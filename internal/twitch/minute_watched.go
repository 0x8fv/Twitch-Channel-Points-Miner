package twitch

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

func (t *Client) GetSpadeURL(streamer *domain.Streamer) error {
	if streamer.Stream == nil {
		streamer.Stream = domain.NewStream()
	}
	headers := map[string]string{
		"User-Agent": twitchapi.UserAgents["Linux"]["FIREFOX"],
	}
	pageURL := streamer.StreamerURL
	if pageURL == "" {
		pageURL = fmt.Sprintf("%s/%s", twitchapi.URL, streamer.Username)
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

func (t *Client) sendSpadePayload(spadeurl string, username string, rawpayload []map[string]interface{}) error {
	raw, err := json.Marshal(rawpayload)
	if err != nil {
		return err
	}
	payload := map[string]string{
		"data": base64.StdEncoding.EncodeToString(raw),
	}
	form := url.Values{}
	form.Set("data", payload["data"])
	req, _ := http.NewRequest(http.MethodPost, spadeurl, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", t.userAgent)
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		t.debugf("Send minute watched payload")
	} else {
		t.debugf("Send minute watched payload to %s (%s)", username, spadeurl)
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
		t.debugf("Minute watched response for %s: %d %s", username, resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if t.anonymizer != nil && t.anonymizer.Enabled() {
		return fmt.Errorf("minute watched failed: %d", resp.StatusCode)
	}
	return fmt.Errorf("minute watched failed: %d %s", resp.StatusCode, string(bodyBytes))
}

// ? check listed media segments
func (t *Client) sendWatchPlaylist(streamer *domain.Streamer) error {
	masterURL := ""
	if streamer.Stream.PlaylistURL == "" {
		op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.PlaybackAccessToken)
		op.Variables = map[string]interface{}{
			"isLive": true, "isVod": false, "login": streamer.Username,
			"platform": "web", "playerType": "site", "vodID": "",
		}
		var response struct {
			Data struct {
				Token *struct {
					Value     string `json:"value"`
					Signature string `json:"signature"`
				} `json:"streamPlaybackAccessToken"`
			} `json:"data"`
		}
		if err := t.PostGQLDecode(op, &response); err != nil {
			return err
		}
		token := response.Data.Token
		if token == nil || token.Value == "" || token.Signature == "" {
			return errors.New("stream playback token unavailable")
		}
		masterURL = "https://usher.ttvnw.net/api/channel/hls/" + url.PathEscape(streamer.Username) + ".m3u8?" +
			url.Values{"sig": {token.Signature}, "token": {token.Value}}.Encode()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if masterURL != "" {
		variants, err := t.readStreamPlaylist(ctx, masterURL)
		if err != nil {
			return err
		}
		selected := variants[len(variants)-1]
		if !strings.HasSuffix(selected.Path, ".m3u8") {
			return errors.New("invalid stream master playlist")
		}
		streamer.Stream.PlaylistURL = selected.String()
	}
	segments, err := t.readStreamPlaylist(ctx, streamer.Stream.PlaylistURL)
	if err != nil {
		streamer.Stream.PlaylistURL = ""
		return err
	}
	for _, segment := range segments {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, segment.String(), nil)
		if err != nil {
			return errors.New("invalid stream segment URL")
		}
		req.Header.Set("User-Agent", t.userAgent)
		resp, err := t.client.Do(req)
		if err != nil {
			return errors.New("stream segment request failed")
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			streamer.Stream.PlaylistURL = ""
			return fmt.Errorf("stream segment returned status %d", resp.StatusCode)
		}
	}
	return nil
}

func (t *Client) readStreamPlaylist(ctx context.Context, address string) ([]*url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, errors.New("invalid stream playlist URL")
	}
	req.Header.Set("User-Agent", t.userAgent)
	req.Close = true
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, errors.New("stream playlist request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stream playlist returned status %d", resp.StatusCode)
	}
	scanner := bufio.NewScanner(resp.Body)
	var entries []*url.URL
	header := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !header {
			if line != "#EXTM3U" {
				return nil, errors.New("invalid stream playlist")
			}
			header = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		entry, err := url.Parse(line)
		if err != nil {
			return nil, errors.New("invalid stream playlist entry")
		}
		entry = req.URL.ResolveReference(entry)
		if (entry.Scheme != "http" && entry.Scheme != "https") || entry.Host == "" || entry.User != nil {
			return nil, errors.New("invalid stream playlist entry")
		}
		entries = append(entries, entry)
	}
	if scanner.Err() != nil || len(entries) == 0 {
		return nil, errors.New("unreadable or empty stream playlist")
	}
	return entries, nil
}

func (t *Client) SendMinuteWatched(streamer *domain.Streamer) error {
	if err := t.UpdateStream(streamer); err != nil {
		return err
	}
	if err := t.sendWatchPlaylist(streamer); err != nil && t.logger != nil {
		name := displayName(streamer.Username)
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			name = t.anonymizer.StreamerName(streamer)
		}
		t.logger.Errorf("minute watch %s: %v", name, err)
	}
	if streamer.Stream.SpadeURL == "" {
		if err := t.GetSpadeURL(streamer); err != nil {
			return err
		}
	}
	streamer.Stream.UpdateMinuteWatched()
	err := t.sendSpadePayload(streamer.Stream.SpadeURL, streamer.Username, streamer.Stream.Payload)
	if err == nil {
		streamer.Stream.UpdateMinuteWatched()
	}
	return err
}
