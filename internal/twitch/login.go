package twitch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"
)

type TwitchLogin struct {
	ClientID  string
	DeviceID  string
	Token     string
	Username  string
	Password  string
	UserAgent string

	client *http.Client
	userID string
	mu     sync.Mutex
}

func NewTwitchLogin(clientID, deviceID, username, userAgent, password string) (*TwitchLogin, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	return &TwitchLogin{
		ClientID:  clientID,
		DeviceID:  deviceID,
		Username:  username,
		Password:  password,
		UserAgent: userAgent,
		client:    client,
	}, nil
}

func (t *TwitchLogin) Client() *http.Client { return t.client }

func (t *TwitchLogin) Login(cookiesPath string) error {
	if err := t.loadCookies(cookiesPath); err == nil && t.Token != "" {
		if ok := t.checkLogin(); ok {
			return nil
		}
	}
	if err := t.runDeviceFlow(); err != nil {
		return err
	}
	if err := t.saveCookies(cookiesPath); err != nil {
		return err
	}
	return nil
}

func (t *TwitchLogin) runDeviceFlow() error {
	postData := url.Values{
		"client_id": {t.ClientID},
		"scopes":    {("channel_read chat:read user_blocks_edit user_blocks_read user_follows_edit user_read")},
	}

	addDeviceHeaders := func(req *http.Request) {
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Accept-Language", "en-US")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Client-Id", t.ClientID)
		req.Header.Set("Host", "id.twitch.tv")
		req.Header.Set("Origin", "https://android.tv.twitch.tv")
		req.Header.Set("Pragma", "no-cache")
		req.Header.Set("Referer", "https://android.tv.twitch.tv/")
		req.Header.Set("User-Agent", UserAgents["Android"]["TV"])
		req.Header.Set("X-Device-Id", t.DeviceID)
	}

	req, _ := http.NewRequest(http.MethodPost, "https://id.twitch.tv/oauth2/device", bytes.NewBufferString(postData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addDeviceHeaders(req)

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device flow start failed: %s", string(body))
	}
	var payload struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
		Interval   int    `json:"interval"`
		ExpiresIn  int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	fmt.Printf("Open https://www.twitch.tv/activate and enter code: %s (expires in %d minutes)\n", payload.UserCode, payload.ExpiresIn/60)

	tokenData := url.Values{
		"client_id":   {t.ClientID},
		"device_code": {payload.DeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	deadline := time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(payload.Interval) * time.Second)
		req, _ := http.NewRequest(http.MethodPost, "https://id.twitch.tv/oauth2/token", bytes.NewBufferString(tokenData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addDeviceHeaders(req)
		resp, err := t.client.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var tok struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.Unmarshal(body, &tok); err != nil {
				return err
			}
			if tok.AccessToken == "" {
				return errors.New("no access token received")
			}
			t.setToken(tok.AccessToken)
			return nil
		}
	}
	return errors.New("device code expired before authorization")
}

func (t *TwitchLogin) setToken(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Token = token
	t.ensureSessionCookiesLocked()
}

func (t *TwitchLogin) AuthToken() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Token
}

func (t *TwitchLogin) UserID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.userID != "" {
		return t.userID
	}
	t.mu.Unlock()
	t.checkLogin()
	t.mu.Lock()
	return t.userID
}

func (t *TwitchLogin) checkLogin() bool {
	payload := ClonePersistedOperation(GQLOperations.GetIDFromLogin)
	if payload.Variables == nil {
		payload.Variables = map[string]interface{}{}
	}
	payload.Variables["login"] = t.Username
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, GQLOperations.URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("OAuth %s", t.Token))
	req.Header.Set("Client-Id", t.ClientID)
	req.Header.Set("X-Device-Id", t.DeviceID)
	req.Header.Set("User-Agent", t.UserAgent)
	resp, err := t.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var res struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return false
	}
	if res.Data.User.ID != "" {
		t.mu.Lock()
		t.userID = res.Data.User.ID
		t.ensureSessionCookiesLocked()
		t.mu.Unlock()
		return true
	}
	return false
}
