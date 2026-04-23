package config

import (
	"log"

	"go.uber.org/zap"
)

// InitLogger initializes and returns a zap structured logger.
func InitLogger() *zap.Logger {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	return logger
}
