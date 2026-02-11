package main

import (
	"errors"

	"github.com/Ejexi/Agent/internal/core/config"
	"github.com/Ejexi/Agent/internal/core/logger"
	errTypes "github.com/Ejexi/Agent/internal/errors/types"
	"go.uber.org/zap"
)

func main() {
	// 1. Load config
	cfg, err := config.Load("configs/dev/config.yaml")
	if err != nil {
		panic(err)
	}

	// 2. Create logger with level from config
	log, err := logger.New(cfg.App.LogLevel)
	if err != nil {
		panic(err)
	}

	// 3. Log startup
	log.Info("Application starting",
		zap.String("env", cfg.App.Env),
		zap.String("log_level", cfg.App.LogLevel),
	)

	// 4. Simulate an error
	err = simulateError()
	if err != nil {
		// Log the error
		log.Error("Operation failed", zap.Error(err))

		// If it's our custom error, log the context
		var appErr *errTypes.AppError
		if errors.As(err, &appErr) {
			log.Error("Error details",
				zap.String("code", string(appErr.Code)),
				zap.Any("context", appErr.Context),
			)
		}
	}

	// 5. Log some info
	// log.Info("Database config",
	// 	zap.String("host", cfg.Database.Host),
	// 	zap.Int("port", cfg.Database.Port),
	// )

	log.Info("LLM config",
		zap.String("model", cfg.LLM.Model),
		zap.Float64("temperature", cfg.LLM.Temperature),
	)

	log.Info("Application ready")
}

func simulateError() error {
	// Simulate a database error
	dbErr := errors.New("connection timeout")

	// Wrap it in our error type
	return errTypes.Wrap(dbErr, errTypes.ErrCodeInternal, "Database connection failed").
		WithContext("host", "localhost").
		WithContext("port", 5432)
}
