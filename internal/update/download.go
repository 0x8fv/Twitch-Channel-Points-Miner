package update

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func downloadAsset(url, dir string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "TwitchChannelPointsMiner-Updater")

	client := newHTTPClient(5 * time.Minute)
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
