// Package drops claims finished Twitch drops and reports their progress.
package drops

import (
	"time"

	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/twitch"
)

// claimInterval is how often the inventory is checked for claimable drops.
const claimInterval = 30 * time.Minute

// Inventory is the part of the Twitch client that lists and claims drops.
type Inventory interface {
	ClaimAllDropsFromInventory() ([]twitch.ClaimedDrop, error)
	ClaimAllDropsFromInventoryWithStatuses() ([]twitch.ClaimedDrop, []twitch.DropStatus, error)
}

// Logger is where claims and progress are reported.
type Logger interface {
	Printf(format string, args ...interface{})
	Eventf(event events.Event, format string, args ...interface{})
	EmojiEventf(emoji string, event events.Event, format string, args ...interface{})
}

// Claimer claims drops from the inventory, and logs their progress when asked to.
type Claimer struct {
	inventory    Inventory
	logger       Logger
	showProgress bool
}

func NewClaimer(inventory Inventory, logger Logger, showProgress bool) *Claimer {
	return &Claimer{inventory: inventory, logger: logger, showProgress: showProgress}
}

// Run claims on every interval until stop is closed.
func (c *Claimer) Run(stop <-chan struct{}) {
	ticker := time.NewTicker(claimInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.Claim("drop claim failed")
		case <-stop:
			return
		}
	}
}

// Claim claims whatever is claimable now. A failure is logged with errorPrefix.
func (c *Claimer) Claim(errorPrefix string) {
	if c == nil || c.inventory == nil {
		return
	}
	if c.showProgress {
		drops, statuses, err := c.inventory.ClaimAllDropsFromInventoryWithStatuses()
		if err != nil {
			c.logger.Printf("%s: %v", errorPrefix, err)
			return
		}
		c.logStatuses(statuses)
		c.logClaimed(drops)
		return
	}
	drops, err := c.inventory.ClaimAllDropsFromInventory()
	if err != nil {
		c.logger.Printf("%s: %v", errorPrefix, err)
		return
	}
	c.logClaimed(drops)
}

func (c *Claimer) logClaimed(drops []twitch.ClaimedDrop) {
	for _, drop := range drops {
		reward := drop.RewardName
		if reward == "" {
			reward = "Drop"
		}
		campaign := drop.CampaignName
		if campaign == "" {
			campaign = "Unknown Campaign"
		}
		progress := formatDropProgress(drop.CurrentValue, drop.RequiredValue)
		percent := progressPercent(drop.CurrentValue, drop.RequiredValue)
		c.logger.EmojiEventf(":package:", events.EventDropClaim, "Claim %s (%s) %s (%d%%)", reward, campaign, progress, percent)
	}
}

func (c *Claimer) logStatuses(statuses []twitch.DropStatus) {
	if c == nil || c.logger == nil || !c.showProgress {
		return
	}
	for _, status := range statuses {
		line := formatDropStatusLine(status)
		if line == "" {
			continue
		}
		c.logger.Eventf(events.EventDropStatus, "%s", line)
	}
}
