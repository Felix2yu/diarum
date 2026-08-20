package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Level represents the logging level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func slogLevel(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

var (
	currentLevel = LevelInfo
	levelVar     = new(slog.LevelVar)
)

func init() {
	currentLevel = LevelFromEnv()
	levelVar.Set(slogLevel(currentLevel))
}

// LevelFromEnv resolves the initial logging level from the LOG_LEVEL and DEBUG
// environment variables. It is exported so the resolution logic can be unit
// tested independently of package initialization.
func LevelFromEnv() Level {
	level := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch level {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		// Also check DEBUG env var for backwards compatibility
		if os.Getenv("DEBUG") != "" {
			return LevelDebug
		}
		return LevelInfo
	}
}

// log is the structured logger. The handler can be swapped (e.g. for
// slog.NewJSONHandler, or slog.NewMultiHandler in Go 1.26) without touching any
// call sites.
var log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: levelVar}))

// Debug logs debug level messages
func Debug(format string, v ...any) {
	if currentLevel <= LevelDebug {
		log.Debug(fmt.Sprintf(format, v...))
	}
}

// Info logs info level messages
func Info(format string, v ...any) {
	if currentLevel <= LevelInfo {
		log.Info(fmt.Sprintf(format, v...))
	}
}

// Warn logs warning level messages
func Warn(format string, v ...any) {
	if currentLevel <= LevelWarn {
		log.Warn(fmt.Sprintf(format, v...))
	}
}

// Error logs error level messages
func Error(format string, v ...any) {
	if currentLevel <= LevelError {
		log.Error(fmt.Sprintf(format, v...))
	}
}

// Infow logs a structured info message with key/value pairs.
func Infow(msg string, kv ...any) {
	log.Info(msg, kv...)
}

// Errorw logs a structured error message with key/value pairs.
func Errorw(msg string, kv ...any) {
	log.Error(msg, kv...)
}

// SetLevel sets the current logging level
func SetLevel(level Level) {
	currentLevel = level
	levelVar.Set(slogLevel(level))
}

// GetLevel returns the current logging level
func GetLevel() Level {
	return currentLevel
}
