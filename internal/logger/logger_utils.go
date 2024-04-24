package logger

import (
	"context"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

type LogLevel string

const (
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	TraceID    string   = "traceID"
)

type Fields map[string]interface{}

type Logger struct {
	moduleName string
	logger     *logrus.Entry
}

type Config struct {
	ModuleToFile map[string]string
	LogLevel     LogLevel
	LogFormat    logrus.Formatter
}

var config Config

func Initialize(cfg Config) {
	config = cfg
	logrus.SetFormatter(config.LogFormat)
	switch config.LogLevel {
	case InfoLevel:
		logrus.SetLevel(logrus.InfoLevel)
	case WarnLevel:
		logrus.SetLevel(logrus.WarnLevel)
	case ErrorLevel:
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	for _, file := range config.ModuleToFile {
		path := filepath.Dir(file)
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			logrus.WithError(err).Fatal("cannot create log directory")
		}
		file, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			logrus.WithError(err).Fatal("cannot open log file")
		}

		logrus.SetOutput(file)
	}
}

func New(moduleName string) *Logger {
	return &Logger{
		moduleName: moduleName,
		logger:     logrus.WithField("module", moduleName),
	}
}

func NewFromContext(ctx context.Context, moduleName string) *Logger {
	traceID, ok := ctx.Value(TraceID).(string)
	if !ok {
		traceID = "unknown"
	}
	return &Logger{
		moduleName: moduleName,
		logger:     logrus.WithField("module", moduleName).WithField(TraceID, traceID),
	}
}

func (l *Logger) Info(fields Fields, message string) {
	l.logger.WithFields(logrus.Fields(fields)).Info(message)
}

func (l *Logger) Warn(fields Fields, message string) {
	l.logger.WithFields(logrus.Fields(fields)).Warn(message)
}

func (l *Logger) Error(fields Fields, message string) {
	l.logger.WithFields(logrus.Fields(fields)).Error(message)
}
