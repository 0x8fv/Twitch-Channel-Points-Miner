package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const releasesURL = "https://api.github.com/repos/0x8fv/Twitch-Channel-Points-Miner/releases/latest"

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func fetchLatestRelease() (githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, releasesURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "TwitchChannelPointsMiner-Updater")

	client := newHTTPClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return githubRelease{}, fmt.Errorf("fetch release: unexpected status %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("parse release: %w", err)
	}
	return release, nil
}

func pickAsset(assets []releaseAsset, goos, arch string) (releaseAsset, error) {
	expected := []string{fmt.Sprintf("TwitchChannelPointsMiner-%s-%s", goos, arch)}
	if goos == "windows" {
		expected[0] += ".exe"
	}
	if goos == "darwin" {
		expected = append(expected,
			fmt.Sprintf("TwitchChannelPointsMiner-macos-%s", arch),
			fmt.Sprintf("TwitchChannelPointsMiner-osx-%s", arch),
		)
	}
	for _, asset := range assets {
		for _, name := range expected {
			if strings.EqualFold(asset.Name, name) {
				return asset, nil
			}
		}
	}
	return releaseAsset{}, fmt.Errorf("no release asset for %s/%s", goos, arch)
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
