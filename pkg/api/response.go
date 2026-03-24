// Package api provides common API response types and utilities
package api

import (
	"net/http"
	"online-game/pkg/apperror"

	"github.com/pkg/errors"

	"github.com/gin-gonic/gin"
)

// Response represents a standard API response
type Response struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Success sends a success response
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// Error sends an error response
func Error(c *gin.Context, err error) {
	bizErr := errors.Unwrap(err)
	if bizErr == nil {
		bizErr = err
	}
	appErr, ok := bizErr.(apperror.AppError)

	if !ok {
		c.Header(XHeaderErrorMessage, err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Header(XHeaderErrorMessage, appErr.Message)
	if appErr.Data != nil {
		c.AbortWithStatusJSON(appErr.Code, appErr.Data)
		return
	}
	c.AbortWithStatus(appErr.Code)
}
