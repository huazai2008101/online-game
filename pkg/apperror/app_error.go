// Package apperror provides centralized error handling for the application
package apperror

import "net/http"

// AppError represents an application error
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error returns the error message
func (e AppError) Error() string {
	return e.Message
}

// New creates a new AppError
func New(code int, message string) AppError {
	return AppError{
		Code:    code,
		Message: message,
	}
}

// AlreadyExists creates a conflict error for resources that already exist
func AlreadyExists(resource string) AppError {
	return New(Conflict, resource+"已存在")
}

// WithCode returns a new AppError with the specified code
func (e AppError) WithCode(code int) AppError {
	e.Code = code
	return e
}

// WithMessage returns a new AppError with the specified message
func (e AppError) WithMessage(message string) AppError {
	e.Message = message
	return e
}

// WithData returns a new AppError with the specified data
func (e AppError) WithData(data any) AppError {
	e.Data = data
	return e
}

// HTTPStatus returns the HTTP status code for the error
func (e AppError) HTTPStatus() int {
	// 业务错误码: 首两位数字对应 HTTP 状态码
	// 例如: 40401 -> 404, 50001 -> 500
	code := int(e.Code)
	if code >= 10000 {
		return code / 100
	}
	// 直接使用 HTTP 状态码
	return code
}

// Is returns whether the error matches the target
func (e AppError) Is(target error) bool {
	t, ok := target.(AppError)
	return ok && e.Code == t.Code
}

// StatusCode returns the HTTP status code for any error
func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if appErr, ok := err.(AppError); ok {
		return appErr.HTTPStatus()
	}
	return http.StatusInternalServerError
}
