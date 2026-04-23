package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"
)

type TOTPService struct {
	rdb        *redis.Client
	logger     *zap.Logger
	issuerName string
}

func NewTOTPService(rdb *redis.Client, logger *zap.Logger, issuerName string) *TOTPService {
	return &TOTPService{
		rdb:        rdb,
		logger:     logger,
		issuerName: issuerName,
	}
}

// GenerateTOTPSetup generates a secret for a user and a base64 encoded QR code
func (s *TOTPService) GenerateTOTPSetup(userEmail string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuerName,
		AccountName: userEmail,
	})
	if err != nil {
		s.logger.Error("Failed to generate totp secret", zap.Error(err), zap.String("email", userEmail))
		return "", "", fmt.Errorf("failed to generate totp secret")
	}

	secret := key.Secret()
	url := key.URL()

	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		s.logger.Error("Failed to generate qr code", zap.Error(err), zap.String("email", userEmail))
		return "", "", fmt.Errorf("failed to generate qr code")
	}

	base64QR := base64.StdEncoding.EncodeToString(png)

	s.logger.Info("TOTP setup generated successfully", zap.String("email", userEmail))
	return secret, base64QR, nil
}

// VerifyTOTP checks if the user's code is mathematically valid AND prevents replay attacks
func (s *TOTPService) VerifyTOTP(ctx context.Context, userID string, providedCode string, userSecret string) (bool, error) {
	// 1. Math Validation
	isValid := totp.Validate(providedCode, userSecret)
	if !isValid {
		s.logger.Warn("Invalid TOTP code provided", zap.String("userID", userID))
		return false, fmt.Errorf("invalid TOTP code")
	}

	// 2. Replay Prevention
	replayKey := fmt.Sprintf("totp_used:%s:%s", userID, providedCode)

	exists, err := s.rdb.Exists(ctx, replayKey).Result()
	if err != nil {
		s.logger.Error("Redis error checking replay", zap.Error(err), zap.String("userID", userID))
		return false, fmt.Errorf("internal server error")
	}
	if exists > 0 {
		s.logger.Warn("Replay attack prevented", zap.String("userID", userID))
		return false, fmt.Errorf("code already used. please wait for the next code")
	}

	// 3. Mark Code as Used
	err = s.rdb.Set(ctx, replayKey, "used", 30*time.Second).Err()
	if err != nil {
		s.logger.Error("Failed to save replay prevention key", zap.Error(err), zap.String("userID", userID))
		return false, fmt.Errorf("internal server error")
	}

	s.logger.Info("TOTP verified successfully", zap.String("userID", userID))
	return true, nil
}
