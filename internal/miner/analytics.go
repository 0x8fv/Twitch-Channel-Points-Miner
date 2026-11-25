package miner

import (
	"sync/atomic"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
)

// AnalyticsSink receives point changes and log events for the analytics dashboard.
// Implementations must be safe for concurrent use and must not block for long.
type AnalyticsSink interface {
	RecordPoint(streamer string, points, delta int, reason string)
	RecordSnapshot(streamer string, points int, online bool)
	RecordEvent(event, message string)
}

const analyticsSnapshotInterval = 5 * time.Minute

type analyticsSinkHolder struct{ sink AnalyticsSink }

var analyticsSink atomic.Pointer[analyticsSinkHolder]

// SetAnalyticsSink installs the process-wide analytics sink (nil disables it).
func SetAnalyticsSink(sink AnalyticsSink) {
	if sink == nil {
		analyticsSink.Store(nil)
		return
	}
	analyticsSink.Store(&analyticsSinkHolder{sink: sink})
}

func currentAnalyticsSink() AnalyticsSink {
	if h := analyticsSink.Load(); h != nil {
		return h.sink
	}
	return nil
}

// analyticsEmitEvent forwards a logged event (ANSI colors stripped) to the sink.
func analyticsEmitEvent(event events.Event, message string) {
	if sink := currentAnalyticsSink(); sink != nil {
		sink.RecordEvent(string(event), ansiRegexp.ReplaceAllString(message, ""))
	}
}

func (m *Miner) analyticsRecordPoint(streamer *domain.Streamer, delta int, reason string) {
	if streamer == nil {
		return
	}
	if sink := currentAnalyticsSink(); sink != nil {
		sink.RecordPoint(streamer.Username, streamer.ChannelPoints, delta, reason)
	}
}

// analyticsSnapshotter records every streamer's balance and online state now and
// then every five minutes, catching changes that never pass through logPointsDelta.
func (m *Miner) analyticsSnapshotter(stop <-chan struct{}) {
	ticker := time.NewTicker(analyticsSnapshotInterval)
	defer ticker.Stop()
	for {
		if sink := currentAnalyticsSink(); sink != nil {
			for _, s := range m.streamers {
				if s != nil && s.PointsInit {
					sink.RecordSnapshot(s.Username, s.ChannelPoints, s.IsOnline)
				}
			}
		}
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}
