package utils

import "github.com/gin-gonic/gin"

// SuccessResponse formats a standard successful JSON response
func SuccessResponse(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// ErrorResponse formats a standard error JSON response
func ErrorResponse(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error":   message,
	})
}
