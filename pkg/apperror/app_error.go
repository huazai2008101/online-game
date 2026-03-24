// Package apperror provides centralized error handling for the application
package apperror

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

// WithMessage adds message to the error
func (e AppError) WithMessage(message string) AppError {
	e.Message = message
	return e
}

// WithData adds data to the error
func (e AppError) WithData(data any) AppError {
	e.Data = data
	return e
}
