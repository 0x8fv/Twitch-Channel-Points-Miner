package miner

import (
	"strings"
)

type watchPriority int

const (
	watchPriorityOrder watchPriority = iota
	watchPriorityStreak
	watchPriorityDrops
	watchPrioritySubscribed
	watchPriorityPointsAscending
	watchPriorityPointsDescending
)

const maxConcurrentWatchers = 2

func defaultWatchPriorities() []watchPriority {
	return []watchPriority{
		watchPriorityStreak,
		watchPriorityDrops,
		watchPriorityOrder,
	}
}

func parseWatchPriorities(priorityNames []string) []watchPriority {
	if len(priorityNames) == 0 {
		return defaultWatchPriorities()
	}
	seen := make(map[watchPriority]struct{})
	parsed := make([]watchPriority, 0, len(priorityNames))
	add := func(p watchPriority) {
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		parsed = append(parsed, p)
	}
	for _, raw := range priorityNames {
		name := strings.ToUpper(strings.TrimSpace(raw))
		switch name {
		case "ORDER":
			add(watchPriorityOrder)
		case "STREAK":
			add(watchPriorityStreak)
		case "DROPS":
			add(watchPriorityDrops)
		case "SUBSCRIBED", "SUBS", "MULTIPLIER":
			add(watchPrioritySubscribed)
		case "POINTS_ASC", "POINTS_ASCENDING":
			add(watchPriorityPointsAscending)
		case "POINTS_DESC", "POINTS_DESCENDING":
			add(watchPriorityPointsDescending)
		}
	}
	if len(parsed) == 0 {
		return defaultWatchPriorities()
	}
	return parsed
}
