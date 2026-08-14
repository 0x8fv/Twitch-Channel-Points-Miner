package classes

import (
	"encoding/json"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

type gqlChannelPointsContextResponse struct {
	Data struct {
		Community struct {
			Channel *struct {
				Self struct {
					CommunityPoints struct {
						Balance           int                         `json:"balance"`
						ActiveMultipliers []entities.ActiveMultiplier `json:"activeMultipliers"`
						AvailableClaim    *struct {
							ID string `json:"id"`
						} `json:"availableClaim"`
					} `json:"communityPoints"`
				} `json:"self"`
				CommunityPointsSettings struct {
					Goals     json.RawMessage `json:"goals"`
					IsEnabled bool            `json:"isEnabled"`
				} `json:"communityPointsSettings"`
			} `json:"channel"`
		} `json:"community"`
	} `json:"data"`
}

type TwitchUserIdentity struct {
	ID          string
	Login       string
	DisplayName string
}

type helixUsersResponse struct {
	Data []struct {
		ID          string `json:"id"`
		Login       string `json:"login"`
		DisplayName string `json:"display_name"`
	} `json:"data"`
}

type gqlID struct {
	ID string `json:"id"`
}

type gqlGame struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type gqlStreamInfoOverlayResponse struct {
	Data struct {
		User *struct {
			Stream *struct {
				ID           string  `json:"id"`
				CreatedAt    string  `json:"createdAt"`
				ViewersCount int     `json:"viewersCount"`
				Tags         []gqlID `json:"tags"`
			} `json:"stream"`
			BroadcastSettings *struct {
				Title string   `json:"title"`
				Game  *gqlGame `json:"game"`
			} `json:"broadcastSettings"`
		} `json:"user"`
	} `json:"data"`
}

type gqlIsStreamLiveResponse struct {
	Data struct {
		User *struct {
			Stream *struct {
				CreatedAt string `json:"createdAt"`
			} `json:"stream"`
		} `json:"user"`
	} `json:"data"`
}

type gqlMissedStream struct {
	BroadcastIdentifiers []gqlID `json:"broadcastIdentifiers"`
}

type gqlRewardListResponse struct {
	Data struct {
		Channel *struct {
			Self *struct {
				WatchStreakMilestone *struct {
					WatchStreakMilestone *struct {
						Value                string `json:"value"`
						AchievementTimestamp string `json:"achievementTimestamp"`
					} `json:"watchStreakMilestone"`
					MissedStreams []gqlMissedStream `json:"missedStreams"`
					ExpiresAt     string            `json:"expiresAt"`
				} `json:"watchStreakMilestone"`
			} `json:"self"`
		} `json:"channel"`
	} `json:"data"`
}

func gameToInterfaceMap(game *gqlGame) map[string]interface{} {
	if game == nil {
		return nil
	}
	result := map[string]interface{}{}
	if game.ID != "" {
		result["id"] = game.ID
	}
	if game.DisplayName != "" {
		result["displayName"] = game.DisplayName
	}
	if game.Name != "" {
		result["name"] = game.Name
	}
	return result
}

func tagsToInterfaceMaps(tags []gqlID) []map[string]interface{} {
	if len(tags) == 0 {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(tags))
	for _, tag := range tags {
		if tag.ID == "" {
			continue
		}
		result = append(result, map[string]interface{}{"id": tag.ID})
	}
	return result
}

func parseCommunityGoalsJSON(raw json.RawMessage) map[string]*entities.CommunityGoal {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var goals []map[string]interface{}
	if err := json.Unmarshal(raw, &goals); err != nil {
		return nil
	}
	result := make(map[string]*entities.CommunityGoal, len(goals))
	for _, goalMap := range goals {
		if goal := entities.NewCommunityGoalFromGQL(goalMap); goal != nil && goal.ID != "" {
			result[goal.ID] = goal
		}
	}
	return result
}
