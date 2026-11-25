// Package twitchapi describes Twitch from the outside: its endpoints, the client
// identity the miner presents, and the persisted GQL operations it calls.
package twitchapi

// Where and as which client the miner talks to Twitch.
const (
	URL           = "https://www.twitch.tv"
	IRC           = "irc.chat.twitch.tv"
	IRCPort       = 6667
	WebsocketURL  = "wss://pubsub-edge.twitch.tv/v1"
	ClientID      = "ue6666qo983tsx6so1t0vnawi233wa"
	DropID        = "c2542d6d-cd10-4532-919b-3d19f30a768b"
	ClientVersion = "ef928475-9403-42f2-8a34-55784bd08e16"
)
