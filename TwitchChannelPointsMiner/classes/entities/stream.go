package entities

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Stream struct {
	BroadcastID       string
	Title             string
	Game              map[string]interface{}
	Tags              []map[string]interface{}
	DropsTags         bool
	Campaigns         []interface{}
	CampaignIDs       []string
	CampaignsResolved bool
	DropsActive       bool
	ViewersCount      int
	SpadeURL          string
	Payload           []map[string]interface{}

	WatchStreakMissing bool
	MinuteWatched      float64
	WatchCount         int
	CreatedAt          time.Time
	StreamUpAt         time.Time
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

func (s *Stream) Update(broadcastID, title string, game map[string]interface{}, tags []map[string]interface{}, viewers int, createdAt time.Time, dropID string) {
	if s.BroadcastID != "" && s.BroadcastID != broadcastID {
		s.WatchStreakMissing = true
		s.ResetWatchProgress()
	}
	s.BroadcastID = broadcastID
	s.Title = strings.TrimSpace(title)
	s.Game = game
	s.Tags = tags
	s.ViewersCount = viewers
	if !createdAt.IsZero() {
		s.CreatedAt = createdAt
	}
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
		duration := time.Since(s.lastMinuteUpdate).Minutes()
		// if it's been more than 2 minutes since last minute-watched update,
		// stream probably was not being continuously watched, so reset to 0
		// (likely got pre-empted by something higher priority)
		if duration > 2.0 {
			s.MinuteWatched = 0
		} else {
			s.MinuteWatched += duration
		}
	}
	s.lastMinuteUpdate = time.Now()
}

func (s *Stream) ResetWatchProgress() {
	if s == nil {
		return
	}
	s.MinuteWatched = 0
	s.WatchCount = 0
	s.CreatedAt = time.Time{}
	s.lastMinuteUpdate = time.Time{}
}

func (s *Stream) LastUpdateAgo() time.Duration {
	if s == nil || s.lastUpdate.IsZero() {
		return 0
	}
	return time.Since(s.lastUpdate)
}

func (s *Stream) StreamUpElapsed() bool {
	if s == nil || s.StreamUpAt.IsZero() {
		return true
	}
	return time.Since(s.StreamUpAt) > 2*time.Minute
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
	return fmt.Sprintf("%s (%s)", s.Title, s.GameName())
}

func (s *Stream) GameName() string {
	if s == nil || s.Game == nil {
		return ""
	}
	if v, ok := s.Game["displayName"].(string); ok {
		return strings.TrimSpace(v)
	}
	if v, ok := s.Game["name"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
