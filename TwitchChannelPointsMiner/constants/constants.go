package constants

const (
	URL           = "https://www.twitch.tv"
	IRC           = "irc.chat.twitch.tv"
	IRCPort       = 6667
	WebsocketURL  = "wss://pubsub-edge.twitch.tv/v1"
	ClientID      = "ue6666qo983tsx6so1t0vnawi233wa"
	DropID        = "c2542d6d-cd10-4532-919b-3d19f30a768b"
	ClientVersion = "ef928475-9403-42f2-8a34-55784bd08e16"
	Version       = "1.3"

	ColorGreen  = "\033[38;5;46m"
	ColorRed    = "\033[38;5;196m"
	ColorCyan   = "\033[38;5;14m"
	ColorPurple = "\033[38;5;141m"
	ColorYellow = "\033[38;5;220m"
	ColorReset  = "\033[0m"
)

type GQLPersistedOperation struct {
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	Extensions    GQLPersistedExtensions `json:"extensions"`
}

type GQLPersistedExtensions struct {
	PersistedQuery GQLPersistedQuery `json:"persistedQuery"`
}

type GQLPersistedQuery struct {
	Version    int    `json:"version"`
	Sha256Hash string `json:"sha256Hash"`
}

// ? ClonePersistedOperation returns a copy of the operation with a deep-copied Variables map
// ? This prevents callers from accidentally mutating shared global state when customizing variables per request
func ClonePersistedOperation(op GQLPersistedOperation) GQLPersistedOperation {
	cloned := op
	if op.Variables != nil {
		cloned.Variables = deepCopyStringInterfaceMap(op.Variables)
	}
	return cloned
}

func deepCopyStringInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = deepCopyInterface(v)
	}
	return out
}

func deepCopyInterface(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return deepCopyStringInterfaceMap(t)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i := range t {
			out[i] = deepCopyInterface(t[i])
		}
		return out
	case []string:
		out := make([]string, len(t))
		copy(out, t)
		return out
	default:
		return v
	}
}

var GQLOperations = struct {
	URL                                    string
	IntegrityURL                           string
	WithIsStreamLiveQuery                  GQLPersistedOperation
	PlaybackAccessToken                    GQLPersistedOperation
	VideoPlayerStreamInfoOverlay           GQLPersistedOperation
	RewardList                             GQLPersistedOperation
	ClaimCommunityPoints                   GQLPersistedOperation
	CommunityMomentCalloutClaim            GQLPersistedOperation
	DropsPageClaimDropRewards              GQLPersistedOperation
	ChannelPointsContext                   GQLPersistedOperation
	JoinRaid                               GQLPersistedOperation
	ModViewChannelQuery                    GQLPersistedOperation
	Inventory                              GQLPersistedOperation
	MakePrediction                         GQLPersistedOperation
	ViewerDropsDashboard                   GQLPersistedOperation
	DropCampaignDetails                    GQLPersistedOperation
	DropsHighlightServiceAvailable         GQLPersistedOperation
	GetIDFromLogin                         GQLPersistedOperation
	PersonalSections                       []GQLPersistedOperation
	ChannelFollows                         GQLPersistedOperation
	UserPointsContribution                 GQLPersistedOperation
	ContributeCommunityPointsCommunityGoal GQLPersistedOperation
	ClipsCardsUser                         GQLPersistedOperation
	FilterableVideoTowerVideos             GQLPersistedOperation
}{
	URL:                          "https://gql.twitch.tv/gql",
	IntegrityURL:                 "https://gql.twitch.tv/integrity",
	WithIsStreamLiveQuery:        newPersistedOperation("WithIsStreamLiveQuery", "04e46329a6786ff3a81c01c50bfa5d725902507a0deb83b0edbf7abe7a3716ea", nil),
	PlaybackAccessToken:          newPersistedOperation("PlaybackAccessToken", "3093517e37e4f4cb48906155bcd894150aef92617939236d2508f3375ab732ce", nil),
	VideoPlayerStreamInfoOverlay: newPersistedOperation("VideoPlayerStreamInfoOverlayChannel", "e785b65ff71ad7b363b34878335f27dd9372869ad0c5740a130b9268bcdbe7e7", nil),
	RewardList:                   newPersistedOperation("RewardList", "0b1471876d7647993731b9e3c6a13bf304c67fb31d07f06a945d42286ee377c4", map[string]interface{}{"shouldIncludeAllSuspendedStreaks": false}),
	ClaimCommunityPoints:         newPersistedOperation("ClaimCommunityPoints", "46aaeebe02c99afdf4fc97c7c0cba964124bf6b0af229395f1f6d1feed05b3d0", nil),
	CommunityMomentCalloutClaim:  newPersistedOperation("CommunityMomentCallout_Claim", "e2d67415aead910f7f9ceb45a77b750a1e1d9622c936d832328a0689e054db62", nil),
	DropsPageClaimDropRewards:    newPersistedOperation("DropsPage_ClaimDropRewards", "a455deea71bdc9015b78eb49f4acfbce8baa7ccbedd28e549bb025bd0f751930", nil),
	ChannelPointsContext:         newPersistedOperation("ChannelPointsContext", "374314de591e69925fce3ddc2bcf085796f56ebb8cad67a0daa3165c03adc345", nil),
	JoinRaid:                     newPersistedOperation("JoinRaid", "c6a332a86d1087fbbb1a8623aa01bd1313d2386e7c63be60fdb2d1901f01a4ae", nil),
	ModViewChannelQuery:          newPersistedOperation("ModViewChannelQuery", "df5d55b6401389afb12d3017c9b2cf1237164220c8ef4ed754eae8188068a807", nil),
	Inventory: newPersistedOperation("Inventory", "d86775d0ef16a63a33ad52e80eaff963b2d5b72fada7c991504a57496e1d8e4b", map[string]interface{}{
		"fetchRewardCampaigns": true,
	}),
	MakePrediction:                 newPersistedOperation("MakePrediction", "b44682ecc88358817009f20e69d75081b1e58825bb40aa53d5dbadcc17c881d8", nil),
	ViewerDropsDashboard:           newPersistedOperation("ViewerDropsDashboard", "5a4da2ab3d5b47c9f9ce864e727b2cb346af1e3ea8b897fe8f704a97ff017619", map[string]interface{}{"fetchRewardCampaigns": true}),
	DropCampaignDetails:            newPersistedOperation("DropCampaignDetails", "f6396f5ffdde867a8f6f6da18286e4baf02e5b98d14689a69b5af320a4c7b7b8", nil),
	DropsHighlightServiceAvailable: newPersistedOperation("DropsHighlightService_AvailableDrops", "782dad0f032942260171d2d80a654f88bdd0c5a9dddc392e9bc92218a0f42d20", nil),
	GetIDFromLogin: newPersistedOperation("GetIDFromLogin", "94e82a7b1e3c21e186daa73ee2afc4b8f23bade1fbbff6fe8ac133f50a2f58ca", map[string]interface{}{
		"login": nil,
	}),
	PersonalSections: []GQLPersistedOperation{
		newPersistedOperation("PersonalSections", "9fbdfb00156f754c26bde81eb47436dee146655c92682328457037da1a48ed39", map[string]interface{}{
			"input": map[string]interface{}{
				"sectionInputs":         []string{"FOLLOWED_SECTION"},
				"recommendationContext": map[string]interface{}{"platform": "web"},
			},
			"channelLogin":                          nil,
			"withChannelUser":                       false,
			"creatorAnniversariesExperimentEnabled": false,
		}),
	},
	ChannelFollows: newPersistedOperation("ChannelFollows", "eecf815273d3d949e5cf0085cc5084cd8a1b5b7b6f7990cf43cb0beadf546907", map[string]interface{}{
		"limit": 100,
		"order": "ASC",
	}),
	UserPointsContribution:                 newPersistedOperation("UserPointsContribution", "23ff2c2d60708379131178742327ead913b93b1bd6f665517a6d9085b73f661f", nil),
	ContributeCommunityPointsCommunityGoal: newPersistedOperation("ContributeCommunityPointsCommunityGoal", "5774f0ea5d89587d73021a2e03c3c44777d903840c608754a1be519f51e37bb6", nil),
	ClipsCardsUser: newPersistedOperation("ClipsCards__User", "1cd671bfa12cec480499c087319f26d21925e9695d1f80225aae6a4354f23088", map[string]interface{}{
		"limit":    20,
		"criteria": map[string]interface{}{"filter": "ALL_TIME"},
	}),
	FilterableVideoTowerVideos: newPersistedOperation("FilterableVideoTower_Videos", "67004f7881e65c297936f32c75246470629557a393788fb5a69d6d9a25a8fd5f", map[string]interface{}{
		"limit":     20,
		"videoSort": "TIME",
	}),
}

func newPersistedOperation(name, hash string, variables map[string]interface{}) GQLPersistedOperation {
	return GQLPersistedOperation{
		OperationName: name,
		Variables:     variables,
		Extensions: GQLPersistedExtensions{
			PersistedQuery: GQLPersistedQuery{
				Version:    1,
				Sha256Hash: hash,
			},
		},
	}
}
