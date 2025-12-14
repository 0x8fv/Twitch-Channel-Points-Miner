package privacy

import (
	"crypto/rand"
	"math/big"
	"strings"
	"sync"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

type Anonymizer struct {
	enabled bool

	mu sync.Mutex

	nextStreamerIndex int
	streamerAliases   map[string]string
	points            map[string]*pointsState

	initialPointsMin int
	initialPointsMax int
}

type pointsState struct {
	initialized bool
	lastReal    int
	pseudo      int
}

func New(enabled bool) *Anonymizer {
	return &Anonymizer{
		enabled:           enabled,
		nextStreamerIndex: 1,
		streamerAliases:   make(map[string]string),
		points:            make(map[string]*pointsState),
		initialPointsMin:  100,
		initialPointsMax:  1000,
	}
}

func (a *Anonymizer) Enabled() bool {
	return a != nil && a.enabled
}

func (a *Anonymizer) StreamerName(streamer *entities.Streamer) string {
	if streamer == nil {
		return ""
	}
	if name := strings.TrimSpace(streamer.Username); name != "" {
		return a.Name(name)
	}
	if id := strings.TrimSpace(streamer.ChannelID); id != "" {
		return a.Name("id:" + id)
	}
	return ""
}

func (a *Anonymizer) Name(raw string) string {
	if !a.Enabled() {
		return raw
	}
	key := normalizeKey(raw)
	if key == "" {
		return ""
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if alias, ok := a.streamerAliases[key]; ok {
		return alias
	}
	alias := "Streamer" + strconvItoa(a.nextStreamerIndex)
	a.nextStreamerIndex++
	a.streamerAliases[key] = alias
	return alias
}

func (a *Anonymizer) PseudoChannelPoints(streamer *entities.Streamer) int {
	if streamer == nil {
		return 0
	}
	if !a.Enabled() {
		return streamer.ChannelPoints
	}

	key := streamerKey(streamer)
	if key == "" {
		key = normalizeKey(streamer.Username)
	}
	if key == "" {
		return streamer.ChannelPoints
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	state := a.points[key]
	if state == nil {
		state = &pointsState{}
		a.points[key] = state
	}

	if !state.initialized {
		state.pseudo = randInt(a.initialPointsMin, a.initialPointsMax)
		state.lastReal = streamer.ChannelPoints
		state.initialized = true
		return state.pseudo
	}

	delta := streamer.ChannelPoints - state.lastReal
	state.pseudo += delta
	state.lastReal = streamer.ChannelPoints
	return state.pseudo
}

func streamerKey(streamer *entities.Streamer) string {
	if streamer == nil {
		return ""
	}
	if id := strings.TrimSpace(streamer.ChannelID); id != "" {
		return "id:" + id
	}
	if name := normalizeKey(streamer.Username); name != "" {
		return "name:" + name
	}
	return ""
}

func normalizeKey(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func randInt(min, max int) int {
	if max <= min {
		return min
	}
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return min
	}
	return min + int(nBig.Int64())
}

func strconvItoa(v int) string {
	// ? small, allocation-free integer formatting for sequential ids
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [32]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
