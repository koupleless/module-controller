package logger

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"testing"
)

func TestLogger_Json(t *testing.T) {
	config := Config{ModuleNameToLogFileDir: map[string]string{
		"schedule":          "logs",
		"module_controller": "logs",
	},
		LogLevel:  InfoLevel,
		LogFormat: &logrus.JSONFormatter{},
	}

	Initialize(config)

	logger := New("schedule")
	logger.Info(Fields{"hello": "word", "foo": "bar"}, "message")
	logger.Error(Fields{"key": "value"}, "a mc error occurred")

	ctx := context.WithValue(context.Background(), TraceID, "12345")
	logger = NewFromContext(ctx, "module_controller")
	logger.Info(Fields{"hello": "word", "foo": "bar"}, "message")
	logger.Error(Fields{"key": "value"}, "a mc error occurred")
}

func TestLogger_CustomVText(t *testing.T) {
	config := Config{ModuleNameToLogFileDir: map[string]string{
		"schedule":          "logs",
		"module_controller": "logs",
	},
		LogLevel:  ErrorLevel,
		LogFormat: &TextVFormatter{},
	}

	Initialize(config)

	logger := New("schedule")
	logger.Info(Fields{"hello": "word", "foo": "bar"}, "info message")
	logger.Error(Fields{"key": "value"}, "error message")

	ctx := context.WithValue(context.Background(), TraceID, "12345")
	logger = NewFromContext(ctx, "module_controller")
	for i := 0; i < 10; i++ {
		logger.Info(Fields{"hello": "word", "foo": "bar"}, fmt.Sprintf("info message %d", i))
	}
	for i := 0; i < 10; i++ {
		logger.Error(Fields{"hello": "word", "foo": "bar"}, fmt.Sprintf("info message %d", i))
	}
}
