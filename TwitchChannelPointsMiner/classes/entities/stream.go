package entities

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Stream struct {
	BroadcastID  string
	Title        string
	Game         map[string]interface{}
	Tags         []map[string]interface{}
	DropsTags    bool
	Campaigns    []interface{}
	CampaignIDs  []string
	ViewersCount int
	SpadeURL     string
	Payload      []map[string]interface{}

	WatchStreakMissing bool
	MinuteWatched      float64
	lastUpdate         time.Time
	lastMinuteUpdate   time.Time
}

func NewStream() *Stream {
	return &Stream{
		Game:               map[string]interface{}{},
		Tags:               []map[string]interface{}{},
		Campaigns:          []interface{}{},
		CampaignIDs:        []string{},
		WatchStreakMissing: true,
	}
}

func (s *Stream) Update(broadcastID, title string, game map[string]interface{}, tags []map[string]interface{}, viewers int, dropID string) {
	s.BroadcastID = broadcastID
	s.Title = strings.TrimSpace(title)
	s.Game = game
	s.Tags = tags
	s.ViewersCount = viewers
	s.DropsTags = false
	for _, tag := range tags {
		if id, ok := tag["id"].(string); ok && id == dropID && len(game) > 0 {
			s.DropsTags = true
			break
		}
	}
	s.lastUpdate = time.Now()
}

func (s *Stream) UpdateRequired() bool {
	return s.lastUpdate.IsZero() || time.Since(s.lastUpdate) >= 120*time.Second
}

func (s *Stream) UpdateMinuteWatched() {
	if !s.lastMinuteUpdate.IsZero() {
		s.MinuteWatched += time.Since(s.lastMinuteUpdate).Minutes()
	}
	s.lastMinuteUpdate = time.Now()
}

func (s *Stream) LastUpdateAgo() time.Duration {
	if s == nil || s.lastUpdate.IsZero() {
		return 0
	}
	return time.Since(s.lastUpdate)
}

func (s *Stream) EncodePayload() (map[string]string, error) {
	raw, err := json.Marshal(s.Payload)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"data": base64.StdEncoding.EncodeToString(raw),
	}, nil
}

func (s *Stream) String() string {
	return fmt.Sprintf("%s (%s)", s.Title, s.gameName())
}

func (s *Stream) gameName() string {
	if s.Game == nil {
		return ""
	}
	if v, ok := s.Game["displayName"].(string); ok {
		return v
	}
	return ""
}
