package logger

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"testing"
	"time"
)

func TestLogger_Json(t *testing.T) {
	config := Config{ModuleNameToLogFileDir: map[string]string{
		"schedule":          "logs",
		"module_controller": "logs",
	},
		LogLevel:  InfoLevel,
		LogFormat: &logrus.JSONFormatter{TimestampFormat: "2006-01-02T15:04:05.000Z07:00"},
	}

	Initialize(config)

	logger := New("schedule")
	logger.Info(Fields{"hello": "word", "foo": "bar"}, "schedule info message")
	logger.Error(Fields{"key": "value"}, "schedule error message")

	ctx := context.WithValue(context.Background(), TraceID, GetTraceId())
	logger = NewFromContext(ctx, "module_controller")
	logger.Info(Fields{"hello": "word", "foo": "bar"}, "module_controller info message")
	logger.Error(Fields{"key": "value"}, "module_controller error message")

	logger.Info(Fields{"hello": "word", "foo": "bar"}, "module_controller info message")
	logger.Error(Fields{"key": "value"}, "module_controller error message")

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

	ctx := context.WithValue(context.Background(), TraceID, GetTraceId())
	logger = NewFromContext(ctx, "module_controller")
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond)
		logger.Info(Fields{"hello": "word", "foo": "bar"}, fmt.Sprintf("info message %d", i))
		logger.Error(Fields{"hello": "word", "foo": "bar"}, fmt.Sprintf("info message %d", i))
	}
}
