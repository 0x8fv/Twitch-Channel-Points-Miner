package update

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func RunAutoUpdate() (bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("locate executable: %w", err)
	}

	devRun := isGoRunExecutable(exePath)

	release, err := fetchLatestRelease()
	if err != nil {
		return false, err
	}

	currentVersion := normalizeVersion(Version)
	latestVersion := normalizeVersion(release.TagName)
	if compareVersions(latestVersion, currentVersion) <= 0 {
		return false, nil
	}

	if devRun {
		log.Printf("\033[31mauto-update: newer version available (%s). Rebuild binary to update.\033[0m", release.TagName)
		return false, nil
	}

	asset, err := pickAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return false, err
	}

	log.Printf("auto-update: found newer version %s (asset %s)", release.TagName, asset.Name)
	tempPath, err := downloadAsset(asset.BrowserDownloadURL, filepath.Dir(exePath))
	if err != nil {
		return false, fmt.Errorf("download update: %w", err)
	}

	if runtime.GOOS == "windows" {
		return true, launchWindowsUpdater(exePath, tempPath, os.Args[1:])
	}

	if err := replaceExecutable(exePath, tempPath); err != nil {
		return false, err
	}
	if err := relaunch(exePath, os.Args[1:]); err != nil {
		return false, err
	}
	return true, nil
}

func normalizeVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "v")
	if idx := strings.IndexAny(raw, " -"); idx > 0 {
		raw = raw[:idx]
	}
	if idx := strings.Index(raw, "+"); idx > 0 {
		raw = raw[:idx]
	}
	return raw
}

func compareVersions(a, b string) int {
	parse := func(v string) []int {
		parts := strings.Split(v, ".")
		out := make([]int, len(parts))
		for i, p := range parts {
			val, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil {
				val = 0
			}
			out[i] = val
		}
		return out
	}

	as := parse(a)
	bs := parse(b)
	max := len(as)
	if len(bs) > max {
		max = len(bs)
	}
	for i := 0; i < max; i++ {
		av, bv := 0, 0
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func isGoRunExecutable(path string) bool {
	lower := strings.ToLower(path)
	if strings.Contains(lower, "go-build") {
		return true
	}
	temp := strings.ToLower(os.TempDir())
	return strings.HasPrefix(lower, temp)
}
