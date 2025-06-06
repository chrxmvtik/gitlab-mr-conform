package logger

import (
	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Logger
}

func New() *Logger {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.JSONFormatter{})

	return &Logger{Logger: log}
}

func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.WithFields(toLogrusFields(fields...)).Debug(msg)
}

func (l *Logger) Info(msg string, fields ...interface{}) {
	l.WithFields(toLogrusFields(fields...)).Info(msg)
}

func (l *Logger) Error(msg string, fields ...interface{}) {
	l.WithFields(toLogrusFields(fields...)).Error(msg)
}

func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.WithFields(toLogrusFields(fields...)).Fatal(msg)
}

func toLogrusFields(keyvals ...interface{}) logrus.Fields {
	fields := make(logrus.Fields)
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key := keyvals[i].(string)
			fields[key] = keyvals[i+1]
		}
	}
	return fields
}
