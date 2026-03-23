// Package api provides common API response types and utilities
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "online-game/pkg/errors"
)

// Response represents a standard API response
type Response struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Success sends a success response
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    apperrors.Success,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithMessage sends a success response with a custom message
func SuccessWithMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    apperrors.Success,
		Message: message,
		Data:    data,
	})
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Code    int32 `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
	Total   int64  `json:"total,omitempty"`
}

// Paginated sends a paginated response
func Paginated(c *gin.Context, data any, page, perPage int, total int64) {
	c.JSON(http.StatusOK, PaginatedResponse{
		Code:    apperrors.Success,
		Message: "success",
		Data:    data,
		Page:    page,
		PerPage: perPage,
		Total:   total,
	})
}

// Error sends an error response
func Error(c *gin.Context, code int32, message string) {
	status := apperrors.HTTPStatus(code)
	c.JSON(status, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData sends an error response with data
func ErrorWithData(c *gin.Context, code int32, message string, data any) {
	status := apperrors.HTTPStatus(code)
	c.JSON(status, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// HandleError handles any error and sends appropriate response
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	if appErr, ok := err.(*apperrors.AppError); ok {
		status := apperrors.HTTPStatus(appErr.Code)
		c.JSON(status, Response{
			Code:    appErr.Code,
			Message: appErr.Message,
			Data:    appErr.Data,
		})
		return
	}

	// Unknown error - treat as internal error
	c.JSON(http.StatusInternalServerError, Response{
		Code:    apperrors.Internal,
		Message: "服务器内部错误",
	})
}

// ValidationError sends a validation error response
func ValidationError(c *gin.Context, err error) {
	Error(c, apperrors.Validation, "参数错误: "+err.Error())
}

// BadRequest sends a bad request response
func BadRequest(c *gin.Context, message string) {
	Error(c, apperrors.InvalidInput, message)
}

// Unauthorized sends an unauthorized response
func Unauthorized(c *gin.Context, message string) {
	Error(c, apperrors.Unauthorized, message)
}

// Forbidden sends a forbidden response
func Forbidden(c *gin.Context, message string) {
	Error(c, apperrors.Forbidden, message)
}

// NotFound sends a not found response
func NotFound(c *gin.Context, message string) {
	Error(c, apperrors.NotFound, message)
}

// Conflict sends a conflict response
func Conflict(c *gin.Context, message string) {
	Error(c, apperrors.Conflict, message)
}

// InternalError sends an internal server error response
func InternalError(c *gin.Context, message string) {
	Error(c, apperrors.Internal, message)
}

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Page   int    `form:"page" binding:"min=1"`
	PerPage int    `form:"per_page" binding:"min=1,max=100"`
	Sort   string `form:"sort"`
	Order  string `form:"order" binding:"oneof=asc desc"`
}

// DefaultPagination returns default pagination values
func DefaultPagination() PaginationParams {
	return PaginationParams{
		Page:   1,
		PerPage: 20,
		Sort:   "id",
		Order:  "asc",
	}
}

// GetOffset returns the offset for database queries
func (p *PaginationParams) GetOffset() int {
	return (p.Page - 1) * p.PerPage
}
