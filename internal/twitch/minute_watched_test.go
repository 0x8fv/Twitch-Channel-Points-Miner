package twitch

import (
	"net/http"
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/auth"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

func TestSendMinuteWatchedRequestsStreamSegments(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var heads []string
			logger := &stubLogger{}
			twitch := &Client{
				logger:         logger,
				userAgent:      "fixture",
				clientVersion:  twitchapi.ClientVersion,
				versionFetched: time.Now(),
				versionTTL:     time.Hour,
				twitchLogin:    &auth.Login{Token: "token"},
				client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.URL.Host == "gql.twitch.tv":
						return jsonResponse(200, `{"data":{"streamPlaybackAccessToken":{"value":"token+&=","signature":"signature"}}}`), nil
					case req.URL.Host == "usher.ttvnw.net":
						if req.URL.Query().Get("token") != "token+&=" {
							return jsonResponse(403, ""), nil
						}
						return jsonResponse(200, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\nhttps://video.example/high.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=64000\nhttps://video.example/media/playlist.m3u8\n"), nil
					case req.Method == http.MethodGet && req.URL.Path == "/media/playlist.m3u8":
						return jsonResponse(200, "#EXTM3U\n#EXTINF:2.0,\none.ts\n#EXTINF:2.0,\ntwo.ts\n#EXTINF:2.0,\nthree.ts\n#EXT-X-ENDLIST\n"), nil
					case req.Method == http.MethodHead:
						heads = append(heads, req.URL.Path)
						return jsonResponse(status, ""), nil
					case req.Method == http.MethodPost && req.URL.Host == "spade.example":
						return jsonResponse(http.StatusNoContent, ""), nil
					default:
						t.Errorf("unexpected request: %s %s", req.Method, req.URL)
						return jsonResponse(400, ""), nil
					}
				})},
			}
			streamer := newTestStreamer(false)
			streamer.Stream.Update("broadcast", "title", nil, nil, 0, time.Time{}, "")
			streamer.Stream.SpadeURL = "https://spade.example/track"
			// A failed segment request is logged and the minute-watch is still sent.
			if err := twitch.SendMinuteWatched(streamer); err != nil {
				t.Fatalf("watch error got %v for segment status %d", err, status)
			}
			wantErrors := 0
			if status != http.StatusOK {
				wantErrors = 1
			}
			if logger.errorCalls != wantErrors {
				t.Fatalf("logged errors got %d, want %d for segment status %d", logger.errorCalls, wantErrors, status)
			}
			want := []string{"/media/one.ts", "/media/two.ts", "/media/three.ts"}
			if status != http.StatusOK {
				want = want[:1]
			}
			if len(heads) != len(want) {
				t.Fatalf("segment HEADs got %v, want %v", heads, want)
			}
			for i := range want {
				if heads[i] != want[i] {
					t.Fatalf("segment HEADs got %v, want %v", heads, want)
				}
			}
		})
	}
}
