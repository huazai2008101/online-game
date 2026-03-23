// Package api provides common API response types and utilities
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response represents a standard API response
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Page    int         `json:"page,omitempty"`
	PerPage int         `json:"per_page,omitempty"`
	Total   int64       `json:"total,omitempty"`
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

// Success sends a success response
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithMessage sends a success response with a custom message
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// Paginated sends a paginated response
func Paginated(c *gin.Context, data interface{}, page, perPage int, total int64) {
	c.JSON(http.StatusOK, PaginatedResponse{
		Code:    0,
		Message: "success",
		Data:    data,
		Page:    page,
		PerPage: perPage,
		Total:   total,
	})
}

// Error sends an error response
func Error(c *gin.Context, code int, message string) {
	statusCode := http.StatusBadRequest
	if code >= 500 {
		statusCode = http.StatusInternalServerError
	} else if code == 401 {
		statusCode = http.StatusUnauthorized
	} else if code == 403 {
		statusCode = http.StatusForbidden
	} else if code == 404 {
		statusCode = http.StatusNotFound
	}

	c.JSON(statusCode, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequest sends a bad request response
func BadRequest(c *gin.Context, message string) {
	Error(c, 400, message)
}

// Unauthorized sends an unauthorized response
func Unauthorized(c *gin.Context, message string) {
	Error(c, 401, message)
}

// Forbidden sends a forbidden response
func Forbidden(c *gin.Context, message string) {
	Error(c, 403, message)
}

// NotFound sends a not found response
func NotFound(c *gin.Context, message string) {
	Error(c, 404, message)
}

// InternalError sends an internal server error response
func InternalError(c *gin.Context, message string) {
	Error(c, 500, message)
}

// ValidationError sends a validation error response
func ValidationError(c *gin.Context, err error) {
	Error(c, 400, "参数错误: "+err.Error())
}
