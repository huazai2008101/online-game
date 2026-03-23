// Package middleware provides common HTTP middleware
package middleware

import (
	"log"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
	apperrors "online-game/pkg/errors"
)

// ErrorHandler is a middleware that recovers from panics and handles errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Recover from panic
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic recovered: %v\n%s", r, debug.Stack())
				api.InternalError(c, "服务器内部错误")
				c.Abort()
			}
		}()

		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			api.HandleError(c, err)
			c.Abort()
		}
	}
}

// WrapError wraps an error and adds it to the context
func WrapError(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err)
	}
}

// WrapAppError wraps an AppError and adds it to the context
func WrapAppError(c *gin.Context, appErr *apperrors.AppError) {
	if appErr != nil {
		_ = c.Error(appErr)
	}
}

// AbortWithError aborts the request with an error
func AbortWithError(c *gin.Context, err error) {
	c.Error(err)
	c.Abort()
}

// AbortWithAppError aborts the request with an AppError
func AbortWithAppError(c *gin.Context, appErr *apperrors.AppError) {
	c.Error(appErr)
	c.Abort()
}
