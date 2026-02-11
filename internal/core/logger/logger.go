//logging errors and events in the project for debugging

package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
    *zap.Logger
}

func New(level string) (*Logger, error) {
    //Wrapper to zap lib to controll it
    zapLevel := zapcore.InfoLevel
    switch level {
    case "debug":
        zapLevel = zapcore.DebugLevel
    case "info":
        zapLevel = zapcore.InfoLevel
    case "warn":
        zapLevel = zapcore.WarnLevel
    case "error":
        zapLevel = zapcore.ErrorLevel
    }
    
    // Create config
    config := zap.NewProductionConfig()
    config.Level = zap.NewAtomicLevelAt(zapLevel)
    config.EncoderConfig.TimeKey = "timestamp"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    
    // Build logger
    zapLogger, err := config.Build()
    if err != nil {
        return nil, err
    }
    
    return &Logger{zapLogger}, nil
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
    l.Logger.Debug(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
    l.Logger.Info(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
    l.Logger.Error(msg, fields...)
}