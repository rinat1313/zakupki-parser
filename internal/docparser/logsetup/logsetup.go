package logsetup

import (
	"log/slog"
	"os"
	"strings"
	"time"
)

func Init(level string) {
	lvl := parseLevel(level)
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(h))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func LogDone(name string, start time.Time, err error, attrs ...any) {
	args := append([]any{"duration_ms", time.Since(start).Milliseconds(), "err", err}, attrs...)
	slog.Info(name+" done", args...)
}

func LogStart(name string, attrs ...any) {
	slog.Debug(name+" start", attrs...)
}
