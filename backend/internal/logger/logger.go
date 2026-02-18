package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(level string) *zap.Logger {
	var levelLogging zapcore.Level
	switch level {
	case "info":
		levelLogging = zapcore.InfoLevel
	default:
		levelLogging = zapcore.DebugLevel
	}
	encoderCfg := zap.NewDevelopmentEncoderConfig()
	encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.AddSync(os.Stdout),
		levelLogging,
	))
	return logger
}
