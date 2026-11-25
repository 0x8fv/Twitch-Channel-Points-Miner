package main

import (
	"net"
	"os"
	"strconv"
	"strings"

	"TwitchChannelPointsMiner/internal/dashboard"
	"TwitchChannelPointsMiner/internal/miner"
)

type analyticsConfig struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
}

func defaultAnalyticsConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled": true,
		"host":    "0.0.0.0",
		"port":    5000,
	}
}

// analyticsAddr resolves the dashboard listen address; TCPM_ANALYTICS_ADDR overrides the config.
func analyticsAddr(cfg analyticsConfig) string {
	if raw := strings.TrimSpace(os.Getenv("TCPM_ANALYTICS_ADDR")); raw != "" {
		return raw
	}
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = "0.0.0.0"
	}
	port := cfg.Port
	if port <= 0 {
		port = 5000
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// autoUpdateDisabledByEnv reports whether TCPM_DISABLE_AUTO_UPDATE forces the
// self-updater off (container images must not replace their own binary).
func autoUpdateDisabledByEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TCPM_DISABLE_AUTO_UPDATE"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// startAnalytics wires the analytics store into the miner and serves the
// dashboard in the background. Data lives in ./analytics of the data directory.
func startAnalytics(cfg analyticsConfig, logger *miner.Logger) {
	if !cfg.Enabled {
		return
	}
	store, err := dashboard.NewStore("analytics")
	if err != nil {
		logger.Errorf("analytics disabled: %v", err)
		return
	}
	miner.SetAnalyticsSink(store)
	addr := analyticsAddr(cfg)
	logger.Printf("Analytics dashboard listening on http://%s", addr)
	go func() {
		if err := dashboard.NewServer(store).ListenAndServe(addr); err != nil {
			logger.Errorf("analytics dashboard stopped: %v", err)
		}
	}()
}
