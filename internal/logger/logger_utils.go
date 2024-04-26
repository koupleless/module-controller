package logger

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"path/filepath"
	"time"
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
	ModuleNameToLogFileDir map[string]string
	LogLevel               LogLevel
	LogFormat              logrus.Formatter
}

type TextVFormatter struct{}

func (f *TextVFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := entry.Time.Format(time.RFC3339)
	traceID := entry.Data[TraceID]
	logLevel := entry.Level.String()
	message := entry.Message
	log := fmt.Sprintf("%s [%s][%s] %s\n", timestamp, traceID, logLevel, message)
	return []byte(log), nil
}

var ModuleNameToLoggers map[string]*logrus.Logger
var config Config

func Initialize(cfg Config) {
	config = cfg
	ModuleNameToLoggers = make(map[string]*logrus.Logger)
	setLogLevel()

	for moduleName, logFileDir := range config.ModuleNameToLogFileDir {
		logger := createLogger(moduleName, logFileDir, "info")
		ModuleNameToLoggers[fmt.Sprintf("%s_%v", moduleName, "info")] = logger
		logger = createLogger(moduleName, logFileDir, "error")
		ModuleNameToLoggers[fmt.Sprintf("%s_%v", moduleName, "error")] = logger
	}
}

func createLogger(moduleName, logFileDir, logLevel string) *logrus.Logger {
	logFilePath := getLogFilePath(logFileDir, moduleName, logLevel)
	logger := logrus.New()
	logger.Formatter = config.LogFormat
	logger.Out = &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    1024,
		MaxBackups: 5,
		MaxAge:     28,
		Compress:   false,
		LocalTime:  true,
	}
	return logger
}

func setLogLevel() {
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
}

func getLogFilePath(logDir, moduleName, logLevel string) string {
	fileName := fmt.Sprintf("%s_%s.log", moduleName, logLevel)
	return filepath.Join(logDir, fileName)
}

func New(moduleName string) *Logger {
	return &Logger{
		moduleName: moduleName,
		logger:     ModuleNameToLoggers[fmt.Sprintf("%s_%v", moduleName, "info")].WithField(TraceID, ""),
	}
}

func NewError(moduleName string) *Logger {
	return &Logger{
		moduleName: moduleName,
		logger:     ModuleNameToLoggers[fmt.Sprintf("%s_%v", moduleName, "error")].WithField(TraceID, ""),
	}
}

func NewFromContext(ctx context.Context, moduleName string) *Logger {
	traceID, ok := ctx.Value(TraceID).(string)
	if !ok {
		traceID = "unknown"
	}
	return &Logger{
		moduleName: moduleName,
		logger:     ModuleNameToLoggers[fmt.Sprintf("%s_%v", moduleName, "info")].WithField(TraceID, traceID),
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

	NewError(l.moduleName).logger.WithFields(logrus.Fields(fields)).Error(message)
}
