package main

import (
	"log"

	"otp-demo/pkg/config"
	"otp-demo/pkg/handlers"
	"otp-demo/pkg/services"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Load Environment Variables
	config.LoadEnv()
	redisAddr := config.GetEnv("REDIS_ADDR", "localhost:6380")
	serverPort := config.GetEnv("SERVER_PORT", "8081")
	issuerName := config.GetEnv("ISSUER_NAME", "OTP-Demo-App")

	// 2. Initialize Structured Logger
	logger := config.InitLogger()
	defer logger.Sync() // Flushes buffer, if any
	logger.Info("Starting application...")

	// 3. Initialize Redis Connection (Dependency Injection)
	rdb, err := config.NewRedisClient(redisAddr)
	if err != nil {
		logger.Fatal("Failed to connect to Redis: " + err.Error())
	}
	logger.Info("Successfully connected to Redis!")

	// 4. Instantiate Services (Passing Dependencies)
	otpService := services.NewOTPService(rdb, logger)
	totpService := services.NewTOTPService(rdb, logger, issuerName)

	// 5. Instantiate Handlers (Passing Services)
	authHandler := handlers.NewAuthHandler(otpService, totpService)

	// 6. Initialize Gin Router
	router := gin.Default()

	// 7. Define Routes
	api := router.Group("/api")
	{
		otp := api.Group("/otp")
		{
			otp.POST("/send", authHandler.SendOTPHandler)
			otp.POST("/verify", authHandler.VerifyOTPHandler)
		}

		totp := api.Group("/totp")
		{
			totp.POST("/setup", authHandler.SetupTOTPHandler)
			totp.POST("/verify", authHandler.VerifyTOTPHandler)
		}
	}

	// 8. Start Server
	logger.Info("Server is running on http://localhost:" + serverPort)
	if err := router.Run(":" + serverPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
