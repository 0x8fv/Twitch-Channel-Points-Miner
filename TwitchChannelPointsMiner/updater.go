package twitchchannelpointsminer

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
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

func RunAutoUpdate(disableSSL bool) (bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("locate executable: %w", err)
	}

	devRun := isGoRunExecutable(exePath)

	release, err := fetchLatestRelease(disableSSL)
	if err != nil {
		return false, err
	}

	currentVersion := normalizeVersion(constants.Version)
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
	tempPath, err := downloadAsset(asset.BrowserDownloadURL, filepath.Dir(exePath), disableSSL)
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

func fetchLatestRelease(disableSSL bool) (githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, releasesURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "TwitchChannelPointsMiner-Updater")

	client := newHTTPClient(disableSSL, 15*time.Second)
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
	expected := fmt.Sprintf("TwitchChannelPointsMiner-%s-%s", goos, arch)
	if goos == "windows" {
		expected += ".exe"
	}
	for _, asset := range assets {
		if strings.EqualFold(asset.Name, expected) {
			return asset, nil
		}
	}
	return releaseAsset{}, fmt.Errorf("no release asset for %s/%s", goos, arch)
}

func downloadAsset(url, dir string, disableSSL bool) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "TwitchChannelPointsMiner-Updater")

	client := newHTTPClient(disableSSL, 5*time.Minute)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("download asset: unexpected status %s", resp.Status)
	}

	temp, err := os.CreateTemp(dir, "miner-update-*")
	if err != nil {
		return "", err
	}
	defer temp.Close()

	var reader io.Reader = resp.Body
	var progress *progressWriter
	if resp.ContentLength > 0 {
		log.Printf("auto-update: downloading update (%.1f MB)...", bytesToMB(resp.ContentLength))
		progress = newProgressWriter(resp.ContentLength)
		reader = io.TeeReader(resp.Body, progress)
	}

	if _, err := io.Copy(temp, reader); err != nil {
		return "", err
	}
	if progress != nil {
		progress.Done()
	}

	return temp.Name(), nil
}

func replaceExecutable(targetPath, newPath string) error {
	if err := os.Chmod(newPath, 0o755); err != nil && !errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("chmod new executable: %w", err)
	}
	if err := os.Rename(newPath, targetPath); err != nil {
		return fmt.Errorf("swap executable: %w", err)
	}
	return nil
}

func relaunch(targetPath string, args []string) error {
	cmd := exec.Command(targetPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Start()
}

func launchWindowsUpdater(targetPath, newPath string, args []string) error {
	scriptPath := filepath.Join(filepath.Dir(targetPath), fmt.Sprintf("update-%d.bat", time.Now().UnixNano()))
	argString := formatArgs(args)
	script := fmt.Sprintf(`@echo off
setlocal
set "TARGET=%s"
set "NEWFILE=%s"
set "WORKDIR=%s"
cd /D "%%WORKDIR%%"
:wait
ping 127.0.0.1 -n 2 >nul 2>nul
:loop
move /Y "%%NEWFILE%%" "%%TARGET%%" >nul 2>nul
if errorlevel 1 (
  ping 127.0.0.1 -n 3 >nul 2>nul
  goto loop
)
start "" /b "%%TARGET%%"%s
start "" /b cmd /c "del /q ""%%~f0"""
exit /b
`, escapeForBatch(targetPath), escapeForBatch(newPath), escapeForBatch(filepath.Dir(targetPath)), argString)

	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return fmt.Errorf("write updater script: %w", err)
	}

	cmd := exec.Command("cmd", "/C", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func formatArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = " " + strconv.Quote(arg)
	}
	return strings.Join(parts, "")
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

func newHTTPClient(disableSSL bool, timeout time.Duration) *http.Client {
	transport := &http.Transport{}
	if disableSSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func isGoRunExecutable(path string) bool {
	lower := strings.ToLower(path)
	if strings.Contains(lower, "go-build") {
		return true
	}
	temp := strings.ToLower(os.TempDir())
	return strings.HasPrefix(lower, temp)
}

type progressWriter struct {
	total   int64
	written int64
	step    int64
}

func newProgressWriter(total int64) *progressWriter {
	step := total / 20 // ? ~5% steps
	if step == 0 {
		step = total
	}
	return &progressWriter{total: total, step: step}
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.written += int64(n)
	if p.total > 0 && p.written >= p.step {
		p.logProgress()
		for p.written >= p.step {
			p.step += p.total / 20
			if p.total/20 == 0 {
				p.step = p.written + 1
			}
		}
	}
	return n, nil
}

func (p *progressWriter) Done() {
	if p.total > 0 && p.written < p.total {
		p.written = p.total
	}
	p.logProgress()
}

func (p *progressWriter) logProgress() {
	if p.total == 0 {
		return
	}
	percent := float64(p.written) * 100 / float64(p.total)
	if percent > 100 {
		percent = 100
	}
	log.Printf("auto-update: download %.0f%% (%.1f/%.1f MB)", percent, bytesToMB(p.written), bytesToMB(p.total))
}

func bytesToMB(b int64) float64 {
	return float64(b) / (1024 * 1024)
}

func escapeForBatch(path string) string {
	replacer := strings.NewReplacer(
		"^", "^^",
		"&", "^&",
		"|", "^|",
		"<", "^<",
		">", "^>",
		"%", "%%",
		"\"", "\"\"",
	)
	return replacer.Replace(path)
}
