package twitch

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/privacy"
)

var (
	ErrStreamerOffline = errors.New("streamer offline")
	ErrChannelNotFound = errors.New("channel not found")
)

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
	onGameChange   func(streamer *domain.Streamer, previous, current string)
}

func NewTwitch(username, userAgent, password string, logger debugLogger, anonymizer *privacy.Anonymizer) (*Twitch, error) {
	deviceID := randomString(32)
	login, err := NewTwitchLogin(ClientID, deviceID, username, userAgent, password)
	if err != nil {
		return nil, err
	}

	return &Twitch{
		userAgent:      userAgent,
		deviceID:       deviceID,
		clientSession:  randomHex(8),
		clientVersion:  ClientVersion,
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
func (t *Twitch) SetGameChangeHandler(handler func(streamer *domain.Streamer, previous, current string)) {
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
		return ClientVersion
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

	resp, err := client.Get(URL)
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

func displayName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
