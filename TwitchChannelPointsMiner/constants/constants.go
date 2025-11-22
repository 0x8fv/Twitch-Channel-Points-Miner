package constants

const (
	URL           = "https://www.twitch.tv"
	IRC           = "irc.chat.twitch.tv"
	IRCPort       = 6667
	WebsocketURL  = "wss://pubsub-edge.twitch.tv/v1"
	ClientID      = "ue6666qo983tsx6so1t0vnawi233wa"
	DropID        = "c2542d6d-cd10-4532-919b-3d19f30a768b"
	ClientVersion = "ef928475-9403-42f2-8a34-55784bd08e16"
	Branch        = "master"
	Version       = "0.0.4 - BETA"

	ColorGreen = "\033[38;5;46m"
	ColorRed   = "\033[38;5;196m"
	ColorCyan  = "\033[38;5;14m"
	ColorReset = "\033[0m"
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

var GQLOperations = struct {
	URL                                    string
	IntegrityURL                           string
	WithIsStreamLiveQuery                  GQLPersistedOperation
	PlaybackAccessToken                    GQLPersistedOperation
	VideoPlayerStreamInfoOverlay           GQLPersistedOperation
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
}{
	URL:                          "https://gql.twitch.tv/gql",
	IntegrityURL:                 "https://gql.twitch.tv/integrity",
	WithIsStreamLiveQuery:        newPersistedOperation("WithIsStreamLiveQuery", "04e46329a6786ff3a81c01c50bfa5d725902507a0deb83b0edbf7abe7a3716ea", nil),
	PlaybackAccessToken:          newPersistedOperation("PlaybackAccessToken", "3093517e37e4f4cb48906155bcd894150aef92617939236d2508f3375ab732ce", nil),
	VideoPlayerStreamInfoOverlay: newPersistedOperation("VideoPlayerStreamInfoOverlayChannel", "a5f2e34d626a9f4f5c0204f910bab2194948a9502089be558bb6e779a9e1b3d2", nil),
	ClaimCommunityPoints:         newPersistedOperation("ClaimCommunityPoints", "46aaeebe02c99afdf4fc97c7c0cba964124bf6b0af229395f1f6d1feed05b3d0", nil),
	CommunityMomentCalloutClaim:  newPersistedOperation("CommunityMomentCallout_Claim", "e2d67415aead910f7f9ceb45a77b750a1e1d9622c936d832328a0689e054db62", nil),
	DropsPageClaimDropRewards:    newPersistedOperation("DropsPage_ClaimDropRewards", "a455deea71bdc9015b78eb49f4acfbce8baa7ccbedd28e549bb025bd0f751930", nil),
	ChannelPointsContext:         newPersistedOperation("ChannelPointsContext", "1530a003a7d374b0380b79db0be0534f30ff46e61cffa2bc0e2468a909fbc024", nil),
	JoinRaid:                     newPersistedOperation("JoinRaid", "c6a332a86d1087fbbb1a8623aa01bd1313d2386e7c63be60fdb2d1901f01a4ae", nil),
	ModViewChannelQuery:          newPersistedOperation("ModViewChannelQuery", "df5d55b6401389afb12d3017c9b2cf1237164220c8ef4ed754eae8188068a807", nil),
	Inventory: newPersistedOperation("Inventory", "d86775d0ef16a63a33ad52e80eaff963b2d5b72fada7c991504a57496e1d8e4b", map[string]interface{}{
		"fetchRewardCampaigns": true,
	}),
	MakePrediction:                 newPersistedOperation("MakePrediction", "b44682ecc88358817009f20e69d75081b1e58825bb40aa53d5dbadcc17c881d8", nil),
	ViewerDropsDashboard:           newPersistedOperation("ViewerDropsDashboard", "5a4da2ab3d5b47c9f9ce864e727b2cb346af1e3ea8b897fe8f704a97ff017619", map[string]interface{}{"fetchRewardCampaigns": true}),
	DropCampaignDetails:            newPersistedOperation("DropCampaignDetails", "f6396f5ffdde867a8f6f6da18286e4baf02e5b98d14689a69b5af320a4c7b7b8", nil),
	DropsHighlightServiceAvailable: newPersistedOperation("DropsHighlightService_AvailableDrops", "9a62a09bce5b53e26e64a671e530bc599cb6aab1e5ba3cbd5d85966d3940716f", nil),
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
