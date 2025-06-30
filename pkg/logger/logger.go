package logger

import (
	"log/slog"
	"os"
	"strings"
)

type Logger struct {
	*slog.Logger
	levelVar *slog.LevelVar
}

func New() *Logger {
	// Defaults to INFO
	return NewWithLevel("INFO")
}

func NewWithLevel(level string) *Logger {
	// Create a LevelVar that can be changed dynamically
	levelVar := &slog.LevelVar{}
	levelVar.Set(parseLevel(level))

	// Create a JSON handler that uses the dynamic level
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: levelVar,
	})

	log := slog.New(handler)
	return &Logger{
		Logger:   log,
		levelVar: levelVar,
	}
}

// SetLevel dynamically changes the log level
func (l *Logger) SetLevel(level string) {
	l.levelVar.Set(parseLevel(level))
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() slog.Level {
	return l.levelVar.Level()
}

// parseLevel converts string to slog.Level
func parseLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.Logger.Debug(msg, toSlogArgs(fields...)...)
}

func (l *Logger) Info(msg string, fields ...interface{}) {
	l.Logger.Info(msg, toSlogArgs(fields...)...)
}

func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.Logger.Warn(msg, toSlogArgs(fields...)...)
}

func (l *Logger) Error(msg string, fields ...interface{}) {
	l.Logger.Error(msg, toSlogArgs(fields...)...)
}

func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.Logger.Error(msg, toSlogArgs(fields...)...)
	os.Exit(1)
}

func toSlogArgs(keyvals ...interface{}) []interface{} {
	args := make([]interface{}, 0, len(keyvals))
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key := keyvals[i].(string)
			value := keyvals[i+1]
			args = append(args, key, value)
		}
	}
	return args
}
