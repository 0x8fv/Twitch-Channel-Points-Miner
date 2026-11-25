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

	"TwitchChannelPointsMiner/internal/auth"
	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/privacy"
	"TwitchChannelPointsMiner/internal/random"
	"TwitchChannelPointsMiner/internal/twitchapi"
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

// session is what the client needs from a logged-in account.
type session interface {
	Login(cookiesPath string) error
	AuthToken() string
	UserID() string
}

type Client struct {
	username       string
	userAgent      string
	deviceID       string
	clientSession  string
	clientVersion  string
	versionMu      sync.Mutex
	versionFetched time.Time
	versionTTL     time.Duration
	twitchLogin    session
	client         *http.Client
	twilightRegexp *regexp.Regexp
	settingsRegex  *regexp.Regexp
	spadeRegex     *regexp.Regexp
	logger         debugLogger
	anonymizer     *privacy.Anonymizer
	onGameChange   func(streamer *domain.Streamer, previous, current string)
	// sleep replaces time.Sleep when set, so waits can be skipped under test.
	sleep func(time.Duration)
}

func NewClient(username, userAgent, password string, logger debugLogger, anonymizer *privacy.Anonymizer) (*Client, error) {
	deviceID := random.String(32)
	login, err := auth.NewLogin(twitchapi.ClientID, deviceID, username, userAgent, password)
	if err != nil {
		return nil, err
	}

	return &Client{
		username:       username,
		userAgent:      userAgent,
		deviceID:       deviceID,
		clientSession:  random.Hex(8),
		clientVersion:  twitchapi.ClientVersion,
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
func (t *Client) SetGameChangeHandler(handler func(streamer *domain.Streamer, previous, current string)) {
	t.onGameChange = handler
}

func (t *Client) Login(username string) error {
	cookiesPath := filepath.Join("cookies", fmt.Sprintf("%s.json", username))
	if err := t.twitchLogin.Login(cookiesPath); err != nil {
		return err
	}
	return nil
}

func (t *Client) ChatToken() string {
	if t == nil || t.twitchLogin == nil {
		return ""
	}
	return t.twitchLogin.AuthToken()
}

func (t *Client) debugf(format string, args ...interface{}) {
	if t.logger != nil && t.logger.DebugEnabled() {
		t.logger.Debugf(format, args...)
	}
}

func (t *Client) deepDebugf(format string, args ...interface{}) {
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
func (t *Client) UpdateClientVersion() string {
	if t == nil {
		return twitchapi.ClientVersion
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

	resp, err := client.Get(twitchapi.URL)
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

// AuthToken is the OAuth token of the logged-in account.
func (t *Client) AuthToken() string {
	return t.twitchLogin.AuthToken()
}

// UserID is the Twitch user id of the logged-in account.
func (t *Client) UserID() string {
	return t.twitchLogin.UserID()
}

// LoginName is the login the session was opened with.
func (t *Client) LoginName() string {
	return t.username
}

// pause waits for d between the steps of a slow exchange with Twitch.
func (t *Client) pause(d time.Duration) {
	if t.sleep != nil {
		t.sleep(d)
		return
	}
	time.Sleep(d)
}
