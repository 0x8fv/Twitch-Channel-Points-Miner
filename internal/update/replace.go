package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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
