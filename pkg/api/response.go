// Package api provides common API response types and utilities
package api

import (
	"net/http"

	"online-game/pkg/apperror"

	"github.com/gin-gonic/gin"
)

// Response represents a standard API response
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Success sends a success response
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    apperror.OK,
		Message: "success",
		Data:    data,
	})
}

// Error sends an error response
func Error(c *gin.Context, err error) {
	appErr, ok := err.(apperror.AppError)
	if !ok {
		c.Header(XHeaderErrorMessage, err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, Response{
			Code:    apperror.InternalServerError,
			Message: err.Error(),
		})
		return
	}
	c.Header(XHeaderErrorMessage, appErr.Message)
	c.AbortWithStatusJSON(appErr.HTTPStatus(), Response{
		Code:    appErr.Code,
		Message: appErr.Message,
		Data:    appErr.Data,
	})
}

// HandleError is an alias for Error function for backward compatibility
func HandleError(c *gin.Context, err error) {
	Error(c, err)
}

// ValidationError sends a validation error response
func ValidationError(c *gin.Context, err error) {
	c.Header(XHeaderErrorMessage, err.Error())
	c.AbortWithStatusJSON(http.StatusBadRequest, Response{
		Code:    apperror.BadRequest,
		Message: err.Error(),
	})
}

// BadRequest sends a bad request error response
func BadRequest(c *gin.Context, message string) {
	c.Header(XHeaderErrorMessage, message)
	c.AbortWithStatusJSON(http.StatusBadRequest, Response{
		Code:    apperror.BadRequest,
		Message: message,
	})
}

// InternalError sends an internal server error response
func InternalError(c *gin.Context, message string) {
	c.Header(XHeaderErrorMessage, message)
	c.AbortWithStatusJSON(http.StatusInternalServerError, Response{
		Code:    apperror.InternalServerError,
		Message: message,
	})
}

// Unauthorized sends an unauthorized error response
func Unauthorized(c *gin.Context, message string) {
	c.Header(XHeaderErrorMessage, message)
	c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
		Code:    apperror.Unauthorized,
		Message: message,
	})
}

// Forbidden sends a forbidden error response
func Forbidden(c *gin.Context, message string) {
	c.Header(XHeaderErrorMessage, message)
	c.AbortWithStatusJSON(http.StatusForbidden, Response{
		Code:    apperror.Forbidden,
		Message: message,
	})
}

// NotFound sends a not found error response
func NotFound(c *gin.Context, message string) {
	c.Header(XHeaderErrorMessage, message)
	c.AbortWithStatusJSON(http.StatusNotFound, Response{
		Code:    apperror.NotFound,
		Message: message,
	})
}

// SuccessWithMessage sends a success response with custom message
func SuccessWithMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    apperror.OK,
		Message: message,
		Data:    data,
	})
}

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	PerPage  int    `form:"per_page" binding:"omitempty,min=1,max=100"` // Alias for PageSize
	OrderBy  string `form:"order_by" binding:"omitempty"`
	Order    string `form:"order" binding:"omitempty,oneof=asc desc"`
}

// GetPageSize returns the page size from params, falling back to PageSize field or default
func (p *PaginationParams) GetPageSize() int {
	if p.PerPage > 0 {
		return p.PerPage
	}
	if p.PageSize > 0 {
		return p.PageSize
	}
	return 20
}

// GetPage returns the page number from params, defaulting to 1
func (p *PaginationParams) GetPage() int {
	if p.Page > 0 {
		return p.Page
	}
	return 1
}

// DefaultPagination returns default pagination values
func DefaultPagination() (page, pageSize int) {
	return 1, 20
}

// Paginated sends a paginated response
func Paginated(c *gin.Context, total int64, page, pageSize int, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    apperror.OK,
		Message: "success",
		Data: map[string]any{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"items":     data,
		},
	})
}
