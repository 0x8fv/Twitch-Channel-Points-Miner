package chat

import (
	"strings"
	"sync"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/privacy"
)

// Presence joins and leaves each streamer's chat as their IRC mode asks.
type Presence struct {
	username            string
	token               func() string
	logger              Logger
	disableAtInNickname bool
	anonymizer          *privacy.Anonymizer
	displayName         func(*domain.Streamer) string

	mu      sync.Mutex
	clients map[string]*Client
}

// NewPresence returns a Presence for the account username. token supplies the
// chat token when a chat is joined (an empty token means "not logged in yet"),
// and displayName is how a streamer is shown in the log.
func NewPresence(username string, token func() string, logger Logger, disableAtInNickname bool, anonymizer *privacy.Anonymizer, displayName func(*domain.Streamer) string) *Presence {
	return &Presence{
		username:            username,
		token:               token,
		logger:              logger,
		disableAtInNickname: disableAtInNickname,
		anonymizer:          anonymizer,
		displayName:         displayName,
		clients:             make(map[string]*Client),
	}
}

// Update joins or leaves the streamer's chat depending on their IRC mode and
// whether they are online.
func (p *Presence) Update(streamer *domain.Streamer, online bool) {
	if p == nil || streamer == nil {
		return
	}
	if ShouldJoin(streamer.Settings.IRCMode, online) {
		p.join(streamer)
		return
	}
	p.Leave(streamer)
}

func (p *Presence) join(streamer *domain.Streamer) {
	token := p.token()
	if token == "" {
		return
	}
	key := strings.ToLower(streamer.Username)
	p.mu.Lock()
	if _, exists := p.clients[key]; exists {
		p.mu.Unlock()
		return
	}
	client := NewClient(p.username, token, streamer.Username, p.logger, p.disableAtInNickname, p.anonymizer)
	p.clients[key] = client
	p.mu.Unlock()
	if p.logger != nil {
		p.logger.EmojiPrintf(":speech_balloon:", "Join IRC Chat: %s", p.displayName(streamer))
	}
	client.Start()
}

// Leave closes the streamer's chat if it is open.
func (p *Presence) Leave(streamer *domain.Streamer) {
	if p == nil || streamer == nil {
		return
	}
	key := strings.ToLower(streamer.Username)
	p.mu.Lock()
	client, ok := p.clients[key]
	if ok {
		delete(p.clients, key)
	}
	p.mu.Unlock()
	if ok && client != nil {
		if p.logger != nil {
			p.logger.EmojiPrintf(":speech_balloon:", "Leave IRC Chat: %s", p.displayName(streamer))
		}
		client.Stop()
	}
}

// LeaveAll closes every open chat.
func (p *Presence) LeaveAll() {
	if p == nil {
		return
	}
	p.mu.Lock()
	clients := make([]*Client, 0, len(p.clients))
	for key, client := range p.clients {
		if client != nil {
			clients = append(clients, client)
		}
		delete(p.clients, key)
	}
	p.mu.Unlock()
	for _, client := range clients {
		client.Stop()
	}
}

// ShouldJoin reports whether a chat should be open for the given IRC mode.
func ShouldJoin(mode domain.IRCMode, online bool) bool {
	switch mode {
	case domain.IRCModeAlways:
		return true
	case domain.IRCModeNever:
		return false
	case domain.IRCModeOffline:
		return !online
	case domain.IRCModeOnline:
		return online
	default:
		return online
	}
}
