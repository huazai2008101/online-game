package api

import (
	"net/http"

	"online-game/pkg/apperror"

	"github.com/gin-gonic/gin"
)

// Response is the standard API response format.
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Success sends a successful response.
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithMessage sends a successful response with a custom message.
func SuccessWithMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// Created sends a 201 Created response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// Error sends an error response from an AppError.
func Error(c *gin.Context, err error) {
	appErr := apperror.GetAppError(err)
	status := appErr.HTTPStatus()
	c.JSON(status, Response{
		Code:    appErr.Code,
		Message: appErr.Message,
	})
}

// ErrorWithDetails sends an error response with additional data.
func ErrorWithDetails(c *gin.Context, err error, data any) {
	appErr := apperror.GetAppError(err)
	status := appErr.HTTPStatus()
	c.JSON(status, Response{
		Code:    appErr.Code,
		Message: appErr.Message,
		Data:    data,
	})
}

// PaginationParams holds pagination query parameters.
type PaginationParams struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// GetPagination extracts pagination params from query string.
func GetPagination(c *gin.Context) (page, pageSize int) {
	page = 1
	pageSize = 20
	if p := c.Query("page"); p != "" {
		if v := parseInt(p); v > 0 {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v := parseInt(ps); v > 0 && v <= 100 {
			pageSize = v
		}
	}
	return
}

// PaginatedResponse wraps a list with pagination metadata.
type PaginatedResponse struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// Paginated sends a paginated list response.
func Paginated(c *gin.Context, items any, total int64, page, pageSize int) {
	Success(c, PaginatedResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func parseInt(s string) int {
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		v = v*10 + int(c-'0')
	}
	return v
}
