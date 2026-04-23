package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	otpTTL      = 5 * time.Minute // OTP is valid for 5 minutes
	otpCooldown = 1 * time.Minute // Cannot request another OTP within 1 minute
)

type OTPService struct {
	rdb    *redis.Client
	logger *zap.Logger
}

func NewOTPService(rdb *redis.Client, logger *zap.Logger) *OTPService {
	return &OTPService{
		rdb:    rdb,
		logger: logger,
	}
}

// GenerateRandomOTP creates a secure 6-digit random number string
func (s *OTPService) GenerateRandomOTP() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		s.logger.Error("Failed to generate random OTP", zap.Error(err))
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// SendOTP handles generating, caching, and ensuring cooldowns for standard OTPs
func (s *OTPService) SendOTP(ctx context.Context, userID string) (string, error) {
	cooldownKey := fmt.Sprintf("otp_cooldown:%s", userID)
	otpKey := fmt.Sprintf("otp:%s", userID)

	// 1. Check Cooldown
	exists, err := s.rdb.Exists(ctx, cooldownKey).Result()
	if err != nil {
		s.logger.Error("Redis error checking cooldown", zap.Error(err), zap.String("userID", userID))
		return "", fmt.Errorf("redis error checking cooldown")
	}
	if exists > 0 {
		s.logger.Warn("OTP requested during cooldown", zap.String("userID", userID))
		return "", fmt.Errorf("please wait before requesting another OTP")
	}

	// 2. Generate OTP
	otp, err := s.GenerateRandomOTP()
	if err != nil {
		return "", fmt.Errorf("failed to generate OTP")
	}

	// 3. Save OTP with TTL
	err = s.rdb.Set(ctx, otpKey, otp, otpTTL).Err()
	if err != nil {
		s.logger.Error("Failed to save OTP to redis", zap.Error(err), zap.String("userID", userID))
		return "", fmt.Errorf("failed to save OTP")
	}

	// 4. Set Cooldown Key
	err = s.rdb.Set(ctx, cooldownKey, "locked", otpCooldown).Err()
	if err != nil {
		s.logger.Error("Failed to set cooldown", zap.Error(err), zap.String("userID", userID))
		return "", fmt.Errorf("failed to set cooldown")
	}

	s.logger.Info("OTP generated successfully", zap.String("userID", userID))
	return otp, nil
}

// VerifyOTP validates the OTP provided by the user against the one in Redis
func (s *OTPService) VerifyOTP(ctx context.Context, userID string, providedOTP string) (bool, error) {
	otpKey := fmt.Sprintf("otp:%s", userID)

	// 1. Fetch OTP from Redis
	storedOTP, err := s.rdb.Get(ctx, otpKey).Result()
	if err == redis.Nil {
		s.logger.Warn("OTP verification failed (expired or missing)", zap.String("userID", userID))
		return false, fmt.Errorf("OTP expired or invalid")
	} else if err != nil {
		s.logger.Error("Redis error during OTP fetch", zap.Error(err), zap.String("userID", userID))
		return false, fmt.Errorf("internal server error")
	}

	// 2. Compare OTPs
	if storedOTP != providedOTP {
		s.logger.Warn("OTP verification failed (incorrect code)", zap.String("userID", userID))
		return false, fmt.Errorf("incorrect OTP")
	}

	// 3. Success! Delete the OTP immediately to prevent reuse
	s.rdb.Del(ctx, otpKey)
	s.logger.Info("OTP verified successfully", zap.String("userID", userID))

	return true, nil
}
