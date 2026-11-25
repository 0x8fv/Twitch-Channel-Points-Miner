package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info != nil && !info.IsDir()
}

func isGoRunExecutable(path string) bool {
	lower := strings.ToLower(path)
	if strings.Contains(lower, "go-build") {
		return true
	}
	temp := strings.ToLower(os.TempDir())
	return strings.HasPrefix(lower, temp)
}

type appPaths struct {
	WorkDir    string
	ConfigPath string
}

func resolveAppPaths(configFlag, dataDirFlag string) (appPaths, error) {
	if dataDirFlag != "" {
		abs, err := filepath.Abs(dataDirFlag)
		if err != nil {
			return appPaths{}, err
		}
		return appPaths{
			WorkDir:    abs,
			ConfigPath: filepath.Join(abs, "config.json"),
		}, nil
	}

	if configFlag != "" {
		abs, err := filepath.Abs(configFlag)
		if err != nil {
			return appPaths{}, err
		}
		return appPaths{
			WorkDir:    filepath.Dir(abs),
			ConfigPath: abs,
		}, nil
	}

	// ? Environment overrides (useful for non-interactive launches).
	if raw := strings.TrimSpace(os.Getenv("TCPM_DATA_DIR")); raw != "" {
		abs, err := filepath.Abs(raw)
		if err != nil {
			return appPaths{}, err
		}
		return appPaths{
			WorkDir:    abs,
			ConfigPath: filepath.Join(abs, "config.json"),
		}, nil
	}

	if raw := strings.TrimSpace(os.Getenv("TCPM_CONFIG")); raw != "" {
		abs, err := filepath.Abs(raw)
		if err != nil {
			return appPaths{}, err
		}
		return appPaths{
			WorkDir:    filepath.Dir(abs),
			ConfigPath: abs,
		}, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	// ? Preserve the historical behavior when the user runs from a folder that already has config.json.
	if cwd != "" && fileExists(filepath.Join(cwd, "config.json")) {
		return appPaths{
			WorkDir:    cwd,
			ConfigPath: filepath.Join(cwd, "config.json"),
		}, nil
	}

	exePath, err := os.Executable()
	if err == nil && exePath != "" && !isGoRunExecutable(exePath) {
		exeDir := filepath.Dir(exePath)
		return appPaths{
			WorkDir:    exeDir,
			ConfigPath: filepath.Join(exeDir, "config.json"),
		}, nil
	}

	// ? Fallback (dev runs, restricted environments).
	if cwd == "" {
		cwd = "."
	}
	return appPaths{
		WorkDir:    cwd,
		ConfigPath: filepath.Join(cwd, "config.json"),
	}, nil
}

func shouldFallbackToUserConfig(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	return errors.Is(err, syscall.EROFS)
}
