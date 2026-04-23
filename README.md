# Golang OTP & TOTP Authentication Module

This repository contains a production-ready Golang module demonstrating the implementation of **One-Time Passwords (OTP)** and **Time-Based One-Time Passwords (TOTP)** using **Gin** (for the web framework) and **Redis** (for caching, TTL, and replay-attack prevention).

## Features Implemented
*   **Standard OTP Generation:** Secure 6-digit random number generation.
*   **OTP Cooldowns:** Redis-backed rate limiting to prevent SMS/Email spam (1-minute cooldown).
*   **OTP Expiration:** Redis TTL ensures OTPs automatically expire after 5 minutes.
*   **TOTP Setup:** Generates mathematical secrets and Base64 QR codes for Google Authenticator.
*   **TOTP Verification:** Mathematically validates codes against the current time.
*   **Replay Attack Prevention:** Caches successfully used TOTP codes in Redis for 30 seconds to guarantee a code can only be used exactly once, even within its valid time window.

---

## How to Run

1. **Start Redis:** Ensure you have a Redis server running locally. *(Note: By default, this project looks for Redis on `localhost:6380`)*.
2. **Setup Environment:** Create a `.env` file in the root directory (or use the defaults).
   ```text
   REDIS_ADDR=localhost:6380
   SERVER_PORT=8081
   ISSUER_NAME=OTP-Demo-App
   ```
3. **Install Dependencies:**
   ```bash
   go mod tidy
   ```
4. **Run the Server:**
   ```bash
   go run cmd/api/main.go
   ```
   The API will start running at `http://localhost:8081` (or whatever `SERVER_PORT` you set).

---

## Architecture: "Senior Developer" Best Practices Applied
This project was recently refactored to implement production-grade patterns:
*   **Dependency Injection:** Global variables were removed. Database clients and loggers are now passed directly into the services via constructors (`NewOTPService()`), and services are passed into handlers (`NewAuthHandler()`). This makes the codebase highly testable.
*   **Structured Logging:** Replaced standard `fmt.Println` with Uber's `zap` logger for high-performance, JSON-structured logging.
*   **Environment Variables:** Hardcoded values (like ports and issuer names) were extracted into a `.env` file loaded via `godotenv`.

---

## File Structure & Explanations

```text
├── cmd/
│   └── api/
│       └── main.go       # Application Entry Point (Wires all dependencies)
├── pkg/
│   ├── config/
│   │   ├── env.go        # Loads .env file
│   │   ├── logger.go     # Initializes Uber Zap Logger
│   │   └── redis.go      # Returns a new Redis client instance
│   ├── handlers/
│   │   └── auth.go       # API Controllers & Routing Logic
│   ├── services/
│   │   ├── otp.go        # Standard OTP Business Logic
│   │   └── totp.go       # Authenticator (TOTP) Business Logic
│   └── utils/
│       └── response.go   # JSON Response Formatters
├── .env
├── go.mod
└── README.md
```

### 1. `cmd/api/main.go`
**Purpose:** This is the entry point of the application. It loads environment variables, initializes the Logger and Redis client, instantiates the Services and Handlers (Dependency Injection), and sets up the Gin router.

**Important Code:**
```go
// 1. Dependency Injection: Services receive the Redis client and Logger
otpService := services.NewOTPService(rdb, logger)
totpService := services.NewTOTPService(rdb, logger, issuerName)
authHandler := handlers.NewAuthHandler(otpService, totpService)

// 2. Groups routes cleanly under /api
api := router.Group("/api")
{
    otp := api.Group("/otp")
    {
        otp.POST("/send", authHandler.SendOTPHandler)
        otp.POST("/verify", authHandler.VerifyOTPHandler)
    }
}
router.Run(":8081") // Starts the server
```

### 2. `pkg/config/redis.go`
**Purpose:** Establishes a connection to the Redis cache and returns the client instance (no global state).

**Important Code:**
```go
func NewRedisClient(addr string) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        Addr: addr, 
    })
    
    // Pings to ensure connection is actually alive
    _, err := client.Ping(ctx).Result()
    return client, nil
}
```

### 3. `pkg/handlers/auth.go`
**Purpose:** Acts as the controller layer. It receives the incoming HTTP requests, extracts and validates the JSON payload, calls the appropriate service, and returns a formatted JSON response.

**Important Code:**
```go
// Example of validating incoming JSON
var req OTPRequest
if err := c.ShouldBindJSON(&req); err != nil {
    utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
    return
}
```

### 4. `pkg/services/otp.go`
**Purpose:** Contains the core business logic for standard (SMS/Email) OTPs. It handles secure random generation and interacts with Redis to enforce cooldowns and expirations.

**Important Code (Cooldown Logic):**
```go
// Check if user requested an OTP too recently to prevent spam
exists, _ := rdb.Exists(ctx, cooldownKey).Result()
if exists > 0 {
    return "", fmt.Errorf("please wait before requesting another OTP")
}

// Save OTP with an automatic 5-minute expiration (TTL)
rdb.Set(ctx, otpKey, otp, 5 * time.Minute)

// Lock the user out from requesting another for 1 minute
rdb.Set(ctx, cooldownKey, "locked", 1 * time.Minute)
```

### 5. `pkg/services/totp.go`
**Purpose:** Contains the logic for interacting with Authenticator apps. Wraps the `pquerna/otp` library for math validation and adds a layer of Redis caching for extreme security.

**Important Code (Replay Attack Prevention):**
```go
// 1. Math Validation (Does the code match the time?)
isValid := totp.Validate(providedCode, userSecret)

// 2. Replay Prevention Cache
// Even if valid, has it been used already in this 30-second window?
replayKey := fmt.Sprintf("totp_used:%s:%s", userID, providedCode)
exists, _ := rdb.Exists(ctx, replayKey).Result()
if exists > 0 {
    return false, fmt.Errorf("code already used")
}

// 3. Mark Code as Used for 30 seconds
rdb.Set(ctx, replayKey, "used", 30*time.Second)
```
