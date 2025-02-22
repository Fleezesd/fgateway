package fgateway

import (
	"context"
	"os"

	"github.com/fleezesd/fgateway/internal/version"
	"github.com/go-logr/zapr"
	"github.com/solo-io/go-utils/contextutils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// SetupLogging setup zap for controller-runtime logger
func SetupLogging(ctx context.Context, loggerName string) {
	level := zapcore.InfoLevel
	logger := contextutils.LoggerFrom(ctx)
	if envLogLevel := os.Getenv(contextutils.LogLevelEnvName); envLogLevel != "" {
		if err := (&level).Set(envLogLevel); err != nil {
			logger.Infof("Could not set log level from env %s=%s, available levels "+
				"can be found here: https://pkg.go.dev/go.uber.org/zap/zapcore?tab=doc#Level",
				contextutils.LogLevelEnvName,
				envLogLevel,
				zap.Error(err),
			)
		}
	}
	atomicLevel := zap.NewAtomicLevelAt(level)

	baseLogger := ctrlzap.NewRaw(
		ctrlzap.Level(&atomicLevel),
		ctrlzap.RawZapOpts(zap.Fields(
			zap.String("version", version.Version),
		)),
	).Named(loggerName)

	// setup logger
	ctrllog.SetLogger(zapr.NewLogger(baseLogger))
}
