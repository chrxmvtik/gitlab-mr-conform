package logger

import (
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func New() *Logger {
	// Create a JSON handler that writes to stdout
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	log := slog.New(handler)

	return &Logger{Logger: log}
}

func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.Logger.Debug(msg, toSlogArgs(fields...)...)
}

func (l *Logger) Info(msg string, fields ...interface{}) {
	l.Logger.Info(msg, toSlogArgs(fields...)...)
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
