package twitch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/jsonvalue"
	"TwitchChannelPointsMiner/internal/prediction"
	"TwitchChannelPointsMiner/internal/random"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

// ? LoadChannelPointsContext fetches points and claims any bonuses.
func (t *Client) LoadChannelPointsContext(streamer *domain.Streamer) (int, error) {
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.ChannelPointsContext)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelLogin"] = streamer.Username
	var resp gqlChannelPointsContextResponse
	if err := t.PostGQLDecode(op, &resp); err != nil {
		return 0, err
	}
	channel := resp.Data.Community.Channel
	if channel == nil {
		name := streamer.Username
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			name = t.anonymizer.StreamerName(streamer)
		}
		return 0, fmt.Errorf("channel missing for %s: %w", name, ErrChannelNotFound)
	}
	pointsData := channel.Self.CommunityPoints
	balance := pointsData.Balance
	streamer.ChannelPoints = balance
	if len(pointsData.ActiveMultipliers) > 0 {
		streamer.ActiveMultipliers = pointsData.ActiveMultipliers
	} else {
		streamer.ActiveMultipliers = nil
	}
	if streamer.Settings.CommunityGoals {
		streamer.CommunityGoals = parseCommunityGoalsJSON(channel.CommunityPointsSettings.Goals)
		t.ContributeToCommunityGoals(streamer)
	}
	if pointsData.AvailableClaim != nil {
		claimID := pointsData.AvailableClaim.ID
		if claimID == "" {
			if t.logger != nil && t.logger.DebugEnabled() {
				name := streamer.Username
				if t.anonymizer != nil && t.anonymizer.Enabled() {
					name = t.anonymizer.StreamerName(streamer)
					t.logger.Debugf("availableClaim present but missing id for %s", name)
				} else {
					t.logger.Debugf("availableClaim present but missing id for %s", name)
				}
			}
			return balance, nil
		}
		if t.logger != nil {
			name := streamer.Username
			if t.anonymizer != nil && t.anonymizer.Enabled() {
				name = t.anonymizer.StreamerName(streamer)
				t.logger.EmojiPrintf(":gift:", "Pending bonus detected for %s", name)
			} else {
				t.logger.EmojiPrintf(":gift:", "Pending bonus detected for %s (claim %s, channel %s)", name, claimID, streamer.ChannelID)
			}
		}
		if err := t.ClaimBonus(streamer, claimID); err != nil {
			if t.logger != nil {
				name := streamer.Username
				if t.anonymizer != nil && t.anonymizer.Enabled() {
					name = t.anonymizer.StreamerName(streamer)
				}
				t.logger.Errorf("claim bonus on context load for %s failed: %v", name, err)
			}
		} else if t.logger != nil {
			name := streamer.Username
			if t.anonymizer != nil && t.anonymizer.Enabled() {
				name = t.anonymizer.StreamerName(streamer)
				t.logger.Printf("Claim bonus success for %s", name)
			} else {
				t.logger.Printf("Claim bonus success for %s (claim %s)", name, claimID)
			}
		}
	}
	return balance, nil
}

// ? ClaimBonus redeems the community points bonus.
func (t *Client) ClaimBonus(streamer *domain.Streamer, claimID string) error {
	if claimID == "" {
		return fmt.Errorf("missing claim id")
	}
	if streamer == nil || streamer.ChannelID == "" {
		return fmt.Errorf("missing streamer channel id")
	}
	return t.claimBonusTV(streamer, claimID)
}

func (t *Client) claimBonusTV(streamer *domain.Streamer, claimID string) error {
	if streamer == nil || streamer.ChannelID == "" {
		return fmt.Errorf("missing streamer/channel")
	}
	if claimID == "" {
		return fmt.Errorf("missing claim id")
	}

	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.ClaimCommunityPoints)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{
		"channelID": streamer.ChannelID,
		"claimID":   claimID,
	}

	reqBody, _ := json.Marshal(op)
	req, _ := http.NewRequest(http.MethodPost, twitchapi.GQLOperations.URL, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", fmt.Sprintf("OAuth %s", t.twitchLogin.AuthToken()))
	req.Header.Set("Client-Id", twitchapi.ClientID)
	req.Header.Set("Client-Session-Id", t.clientSession)
	req.Header.Set("Client-Version", t.UpdateClientVersion())
	req.Header.Set("User-Agent", t.userAgent) // ? Android TV UA
	req.Header.Set("X-Device-Id", t.deviceID)
	req.Header.Set("Content-Type", "application/json")
	authToken := t.twitchLogin.AuthToken()
	userID := t.twitchLogin.UserID()
	if authToken != "" && userID != "" {
		req.Header.Set("Cookie", fmt.Sprintf("auth-token=%s; persistent=%s", authToken, userID))
	} else if authToken != "" {
		req.Header.Set("Cookie", fmt.Sprintf("auth-token=%s", authToken))
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("claim bonus request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if t.logger != nil && t.logger.DebugEnabled() {
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			t.logger.Debugf("ClaimCommunityPoints status=%d", resp.StatusCode)
		} else {
			t.logger.Debugf("ClaimCommunityPoints status=%d", resp.StatusCode)
			t.deepDebugf("ClaimCommunityPoints status=%d headers=%v req=%s resp=%s", resp.StatusCode, req.Header, strings.TrimSpace(string(reqBody)), strings.TrimSpace(string(respBody)))
		}
	}

	if resp.StatusCode != http.StatusOK {
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			return fmt.Errorf("claim bonus status %d", resp.StatusCode)
		}
		return fmt.Errorf("claim bonus status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("claim bonus decode: %w", err)
	}

	if gqlErrs, ok := result["errors"]; ok {
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			return fmt.Errorf("claim bonus gql errors")
		}
		return fmt.Errorf("claim bonus gql errors: %v", gqlErrs)
	}
	if status := jsonvalue.At(result, "data.claimCommunityPoints.status"); status != nil {
		statusStr := strings.ToUpper(fmt.Sprint(status))
		if statusStr != "" && statusStr != "SUCCESS" && statusStr != "ALREADY_CLAIMED" {
			if t.anonymizer != nil && t.anonymizer.Enabled() {
				return fmt.Errorf("claim bonus status %s", statusStr)
			}
			return fmt.Errorf("claim bonus status %s (resp=%v)", statusStr, result)
		}
	}
	if msg := jsonvalue.At(result, "data.claimCommunityPoints.error.message"); msg != nil {
		if t.anonymizer != nil && t.anonymizer.Enabled() {
			return fmt.Errorf("claim bonus error")
		}
		return fmt.Errorf("claim bonus error: %v (resp=%v)", msg, result)
	}
	return nil
}

// ? ClaimMoment redeems a community moment callout.
func (t *Client) ClaimMoment(streamer *domain.Streamer, momentID string) error {
	if momentID == "" {
		return nil
	}
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.CommunityMomentCalloutClaim)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{"momentID": momentID}
	_, err := t.PostGQL(op)
	return err
}

// ? JoinRaid follows a raid target to mimic viewer behavior.
func (t *Client) JoinRaid(streamer *domain.Streamer, raidID string) error {
	if raidID == "" {
		return fmt.Errorf("missing raid id")
	}
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.JoinRaid)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{"raidID": raidID}
	_, err := t.PostGQL(op)
	return err
}

// ? MakePrediction places a bet for the provided event.
func (t *Client) MakePrediction(event *prediction.Event) error {
	if event == nil || event.Streamer == nil {
		return fmt.Errorf("nil prediction event")
	}
	if event.Decision.Amount < 10 || event.Decision.OutcomeID == "" || event.EventID == "" {
		return fmt.Errorf("invalid prediction decision")
	}
	op := twitchapi.ClonePersistedOperation(twitchapi.GQLOperations.MakePrediction)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{
		"eventID":       event.EventID,
		"outcomeID":     event.Decision.OutcomeID,
		"points":        event.Decision.Amount,
		"transactionID": random.Hex(16),
	}
	_, err := t.PostGQL(op)
	return err
}
