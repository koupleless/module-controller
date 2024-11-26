package zaplogger

import (
	"context"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var logger = zap.New(zap.UseFlagOptions(&zap.Options{Development: true}))

func GetLogger() logr.Logger {
	return logger
}

func WithLogger(ctx context.Context, logger logr.Logger) context.Context {
	return logr.NewContext(ctx, logger)
}

func FromContext(ctx context.Context) logr.Logger {
	l, err := logr.FromContext(ctx)
	if err != nil {
		return logger
	}

	return l
}
