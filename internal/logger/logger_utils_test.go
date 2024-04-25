package logger

import (
	"context"
	"github.com/sirupsen/logrus"
	"testing"
)

func TestLogger_Schedule_Json(t *testing.T) {
	config := Config{ModuleToFile: map[string]string{
		"schedule":          "logs/schedule.log",
		"module_controller": "logs/module_controller.log",
	},
		LogLevel:  InfoLevel,
		LogFormat: &logrus.JSONFormatter{},
	}

	Initialize(config)

	logger := New("schedule")
	logger.Info(Fields{"hello": "word", "foo": "bar"}, "message")
	logger.Error(Fields{"key": "value"}, "a mc error occurred")
}

func TestLogger_module_controller_text(t *testing.T) {
	config := Config{ModuleToFile: map[string]string{
		"schedule":          "logs/schedule.log",
		"module_controller": "logs/module_controller.log",
	},
		LogLevel:  InfoLevel,
		LogFormat: &logrus.TextFormatter{},
	}

	Initialize(config)

	ctx := context.WithValue(context.Background(), TraceID, "12345")
	logger := NewFromContext(ctx, "module_controller")
	logger.Info(Fields{"hello": "word", "foo": "bar"}, "message")
	logger.Error(Fields{"key": "value"}, "a mc error occurred")
}

func TestLogger_Warn(t *testing.T) {
	config := Config{ModuleToFile: map[string]string{
		"schedule":          "logs/schedule.log",
		"module_controller": "logs/module_controller.log",
	},
		LogLevel:  InfoLevel,
		LogFormat: &logrus.TextFormatter{},
	}

	Initialize(config)

	logger := New("schedule")
	logger.Info(Fields{"hello": "word", "foo": "bar"}, "message")

	ctx := context.WithValue(context.Background(), TraceID, "12345")
	logger = NewFromContext(ctx, "module_controller")
	logger.Error(Fields{"key": "value"}, "a mc error occurred")
}
