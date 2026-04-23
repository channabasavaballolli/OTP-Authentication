package handlers

import (
	"net/http"

	"otp-demo/pkg/services"
	"otp-demo/pkg/utils"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	otpService  *services.OTPService
	totpService *services.TOTPService
}

func NewAuthHandler(otpService *services.OTPService, totpService *services.TOTPService) *AuthHandler {
	return &AuthHandler{
		otpService:  otpService,
		totpService: totpService,
	}
}

// Request bodies
type OTPRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

type OTPVerifyRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

type TOTPSetupRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type TOTPVerifyRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Code   string `json:"code" binding:"required"`
	Secret string `json:"secret" binding:"required"`
}

// --- STANDARD OTP HANDLERS ---

func (h *AuthHandler) SendOTPHandler(c *gin.Context) {
	var req OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Pass the request context down to the service
	otp, err := h.otpService.SendOTP(c.Request.Context(), req.UserID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusTooManyRequests, err.Error())
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "OTP sent successfully", gin.H{"mock_email_content_otp": otp})
}

func (h *AuthHandler) VerifyOTPHandler(c *gin.Context) {
	var req OTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	valid, err := h.otpService.VerifyOTP(c.Request.Context(), req.UserID, req.Code)
	if err != nil || !valid {
		utils.ErrorResponse(c, http.StatusUnauthorized, err.Error())
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "OTP verified successfully!", nil)
}

// --- TOTP (AUTHENTICATOR APP) HANDLERS ---

func (h *AuthHandler) SetupTOTPHandler(c *gin.Context) {
	var req TOTPSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	secret, qrBase64, err := h.totpService.GenerateTOTPSetup(req.Email)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to setup TOTP")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "TOTP Setup complete. Scan QR code and save secret.", gin.H{
		"secret":    secret,
		"qr_base64": "data:image/png;base64," + qrBase64,
	})
}

func (h *AuthHandler) VerifyTOTPHandler(c *gin.Context) {
	var req TOTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	valid, err := h.totpService.VerifyTOTP(c.Request.Context(), req.UserID, req.Code, req.Secret)
	if err != nil || !valid {
		utils.ErrorResponse(c, http.StatusUnauthorized, err.Error())
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "TOTP verified successfully! Logged in.", nil)
}
