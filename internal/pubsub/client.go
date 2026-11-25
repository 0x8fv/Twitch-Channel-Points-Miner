package pubsub

import (
	"errors"
	"sync"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/prediction"
	"TwitchChannelPointsMiner/internal/privacy"
	"TwitchChannelPointsMiner/internal/twitch"
)

type Logger interface {
	Printf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	EmojiPrintf(emoji, format string, args ...interface{})
	Eventf(event events.Event, format string, args ...interface{})
	EmojiEventf(emoji string, event events.Event, format string, args ...interface{})
	ErrorEventf(event events.Event, format string, args ...interface{})
	Debugf(format string, args ...interface{})
	DebugEnabled() bool
}

var ErrReconnectRequested = errors.New("pubsub reconnect requested")

type Client struct {
	twitch      *twitch.Client
	logger      Logger
	anonymizer  *privacy.Anonymizer
	disableSSL  bool
	streamers   []*domain.Streamer
	streamerMap map[string]*domain.Streamer
	predictions map[string]*prediction.Event
	predMu      sync.Mutex
	onGain      func(streamer *domain.Streamer, earned int, reason string, balance int)
	onPresence  func(streamer *domain.Streamer, online bool, reason string)
}

func (p *Client) anonymizeLogs() bool {
	return p != nil && p.anonymizer != nil && p.anonymizer.Enabled()
}

func (p *Client) streamerName(streamer *domain.Streamer) string {
	if streamer == nil {
		return ""
	}
	if p.anonymizeLogs() {
		return p.anonymizer.StreamerName(streamer)
	}
	return streamer.Username
}

func (p *Client) name(raw string) string {
	if p.anonymizeLogs() {
		return p.anonymizer.Name(raw)
	}
	return raw
}

func (p *Client) debugf(format string, args ...interface{}) {
	if p.logger != nil && p.logger.DebugEnabled() {
		p.logger.Debugf(format, args...)
	}
}

// deepDebugLogger is the optional extra a Logger may offer.
type deepDebugLogger interface {
	DeepDebugf(format string, args ...interface{})
	DeepDebugEnabled() bool
}

func (p *Client) deepDebugf(format string, args ...interface{}) {
	if p == nil || p.anonymizeLogs() {
		return
	}
	if p.logger == nil {
		return
	}
	deep, ok := p.logger.(deepDebugLogger)
	if !ok || !deep.DeepDebugEnabled() {
		return
	}
	deep.DeepDebugf(format, args...)
}

func NewClient(
	twitch *twitch.Client,
	logger Logger,
	anonymizer *privacy.Anonymizer,
	streamers []*domain.Streamer,
	disableSSL bool,
	onGain func(*domain.Streamer, int, string, int),
	onPresence func(*domain.Streamer, bool, string),
) *Client {
	streamerMap := make(map[string]*domain.Streamer)
	for _, s := range streamers {
		if s.ChannelID != "" {
			streamerMap[s.ChannelID] = s
		}
	}
	return &Client{
		twitch:      twitch,
		logger:      logger,
		anonymizer:  anonymizer,
		disableSSL:  disableSSL,
		streamers:   streamers,
		streamerMap: streamerMap,
		predictions: make(map[string]*prediction.Event),
		onGain:      onGain,
		onPresence:  onPresence,
	}
}

func (p *Client) Start(stop <-chan struct{}) {
	if p.disableSSL {
		p.logger.Printf("SSL certificate verification is disabled! Be aware!")
	}
	topics, err := p.buildTopics()
	if err != nil {
		p.logger.Errorf("PubSub topic error: %v", err)
		return
	}
	go p.pollPendingClaims(stop)
	batches := chunkTopics(topics, 50)
	for i, batch := range batches {
		idx := i + 1
		go p.run(idx, batch, stop)
	}
}

func (p *Client) run(connIndex int, topics []string, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}

		if err := p.connectAndListen(connIndex, topics, stop); err != nil {
			if errors.Is(err, ErrReconnectRequested) {
				p.logger.Printf("PubSub[%d] reconnect requested; waiting ~60 seconds", connIndex)
				time.Sleep(60 * time.Second)
				continue
			}
			p.logger.Errorf("PubSub[%d] connection error: %v", connIndex, err)
			time.Sleep(10 * time.Second)
		}
	}
}
