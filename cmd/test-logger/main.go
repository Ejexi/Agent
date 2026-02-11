package main

import (
	"github.com/Ejexi/Agent/internal/core/logger"
	"go.uber.org/zap"
)

func main() {
	// Create a logger
	log, err := logger.New("debug")
	if err != nil {
		//crash the program
		panic(err)
	}
	log.Info("\nApplication started\n")
	log.Info("\nServer running", zap.Int("port", 8080))
	log.Debug("\nThis won't show because level is 'info'\n")
	log.Warn("\nThis is a warning\n")
	log.Error("\nThis is an error\n")

	// Create a child logger with context
	userLog := log.With(zap.String("user_id", "12345\n"))
	userLog.Info("User logged in\n")
	userLog.Info("User performed action\n", zap.String("action", "upload"))
}
