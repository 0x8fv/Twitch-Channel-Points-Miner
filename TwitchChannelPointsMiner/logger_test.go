package twitchchannelpointsminer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	name := `bad/name\\:*?"<>|`
	got := sanitizeFilename(name)
	if got == name || got == "" {
		t.Fatalf("sanitize did not change invalid name: %q", got)
	}
	if strings.ContainsAny(got, `/\\:*?"<>|`) {
		t.Fatalf("sanitized name still has forbidden chars: %q", got)
	}
}

func TestNewLoggerCreatesFileWhenSaveEnabled(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd error: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(wd)

	logger := NewLogger(LoggerSettings{Save: true}, "tester", false)
	logger.Printf("hello")

	logPath := filepath.Join("log", "tester.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file at %s: %v", logPath, err)
	}
	if w, ok := logger.base.Writer().(dualWriter); ok {
		if f, ok := w.file.(*os.File); ok {
			_ = f.Close()
		}
	}
}

func TestEmojize(t *testing.T) {
	if got := emojize(":rocket:"); got == ":rocket:" {
		t.Fatalf("expected known emoji mapping, got %q", got)
	}
	if got := emojize(":unknown:"); got != ":unknown:" {
		t.Fatalf("unknown emoji should pass through, got %q", got)
	}
}
