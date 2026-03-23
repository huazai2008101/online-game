// Package errors provides centralized error handling for the application
package apperrors

import (
	"fmt"
	"net/http"
)

// Error codes (int32)
const (
	// Success (0)
	Success = 0

	// Validation errors (400) - 1xxx
	Validation    = 1001
	InvalidInput   = 1002
	InvalidFormat  = 1003
	MissingParam   = 1004

	// Authentication errors (401) - 2xxx
	Unauthorized   = 2001
	InvalidToken   = 2002
	TokenExpired   = 2003
	InvalidCreds   = 2004

	// Authorization errors (403) - 3xxx
	Forbidden      = 3001
	AccessDenied   = 3002

	// Not found errors (404) - 4xxx
	NotFound       = 4001
	UserNotFound   = 4002
	GameNotFound   = 4003
	RoomNotFound   = 4004
	OrderNotFound  = 4005
	PlayerNotFound = 4006

	// Conflict errors (409) - 5xxx
	Conflict       = 5001
	AlreadyExists  = 5002
	Duplicate      = 5003

	// Business logic errors (422) - 6xxx
	BusinessLogic  = 6001
	Insufficient   = 6002
	InvalidState   = 6003
	RoomFull       = 6004
	GameNotStarted = 6005

	// Server errors (500) - 9xxx
	Internal       = 9001
	Database       = 9002
	ServiceUnavailable = 9003
	ThirdParty     = 9004
)

// HTTPStatus returns the HTTP status code for an error code
func HTTPStatus(code int32) int {
	switch code {
	case Success:
		return http.StatusOK
	// Validation errors (400)
	case Validation, InvalidInput, InvalidFormat, MissingParam:
		return http.StatusBadRequest
	// Authentication errors (401)
	case Unauthorized, InvalidToken, TokenExpired, InvalidCreds:
		return http.StatusUnauthorized
	// Authorization errors (403)
	case Forbidden, AccessDenied:
		return http.StatusForbidden
	// Not found errors (404)
	case NotFound, UserNotFound, GameNotFound, RoomNotFound, OrderNotFound, PlayerNotFound:
		return http.StatusNotFound
	// Conflict errors (409)
	case Conflict, AlreadyExists, Duplicate:
		return http.StatusConflict
	// Business logic errors (422)
	case BusinessLogic, Insufficient, InvalidState, RoomFull, GameNotStarted:
		return http.StatusUnprocessableEntity
	// Server errors (500)
	default:
		return http.StatusInternalServerError
	}
}

// AppError represents an application error
type AppError struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error returns the error message
func (e *AppError) Error() string {
	return e.Message
}

// New creates a new AppError
func New(code int32, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// WithData adds data to the error
func (e *AppError) WithData(data any) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Data:    data,
	}
}

// Common error constructors

func ValidationError(message string) *AppError {
	return New(Validation, message)
}

func InvalidInput(field, message string) *AppError {
	return New(InvalidInput, fmt.Sprintf("%s: %s", field, message))
}

func MissingParam(param string) *AppError {
	return New(MissingParam, fmt.Sprintf("缺少参数: %s", param))
}

func Unauthorized(message string) *AppError {
	return New(Unauthorized, message)
}

func InvalidToken(message string) *AppError {
	return New(InvalidToken, message)
}

func TokenExpired() *AppError {
	return New(TokenExpired, "令牌已过期")
}

func InvalidCredentials() *AppError {
	return New(InvalidCreds, "用户名或密码错误")
}

func Forbidden(message string) *AppError {
	return New(Forbidden, message)
}

func NotFound(resource string) *AppError {
	return New(NotFound, resource+"不存在")
}

func UserNotFound() *AppError {
	return New(UserNotFound, "用户不存在")
}

func GameNotFound() *AppError {
	return New(GameNotFound, "游戏不存在")
}

func RoomNotFound() *AppError {
	return New(RoomNotFound, "房间不存在")
}

func PlayerNotFound() *AppError {
	return New(PlayerNotFound, "角色不存在")
}

func AlreadyExists(resource string) *AppError {
	return New(AlreadyExists, resource+"已存在")
}

func InsufficientBalance(message string) *AppError {
	return New(Insufficient, message)
}

func InvalidState(message string) *AppError {
	return New(InvalidState, message)
}

func RoomFull() *AppError {
	return New(RoomFull, "房间已满")
}

func InternalError(message string) *AppError {
	return New(Internal, message)
}

func DatabaseError(message string) *AppError {
	return New(Database, message)
}

func ServiceUnavailable(message string) *AppError {
	return New(ServiceUnavailable, message)
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*AppError)
	return ok
}

// GetAppError converts an error to AppError
func GetAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return InternalError(err.Error())
}
