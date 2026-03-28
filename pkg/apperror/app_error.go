package apperror

import (
	"fmt"
	"net/http"
)

// AppError represents a business error with an HTTP-mapped code.
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *AppError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// HTTPStatus maps the business code to an HTTP status code.
// Convention: code = HTTP_STATUS * 100 + SEQ (e.g., 40401 -> 404).
func (e *AppError) HTTPStatus() int {
	return e.Code / 100
}

// WithData returns a new error with attached data.
func (e *AppError) WithData(data any) *AppError {
	return &AppError{Code: e.Code, Message: e.Message, Data: data}
}

// WithMessage returns a new error with a custom message.
func (e *AppError) WithMessage(msg string) *AppError {
	return &AppError{Code: e.Code, Message: msg, Data: e.Data}
}

// New creates a new AppError.
func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// --- Error definitions ---

// Common errors (40000-40999)
var (
	ErrBadRequest   = New(40000, "请求参数错误")
	ErrUnauthorized = New(40100, "未授权")
	ErrForbidden    = New(40300, "禁止访问")
	ErrNotFound     = New(40400, "资源不存在")
)

// User errors (41000-41999)
var (
	ErrUserNotFound    = New(41401, "用户不存在")
	ErrUserExists      = New(41402, "用户已存在")
	ErrInvalidPassword = New(41403, "密码错误")
	ErrInvalidToken    = New(41404, "无效的Token")
)

// Game errors (44000-44999)
var (
	ErrGameNotFound   = New(44401, "游戏不存在")
	ErrGameNotReady   = New(44402, "游戏未就绪")
	ErrInvalidPackage = New(44001, "游戏包格式无效")
	ErrScriptError    = New(44002, "脚本执行错误")
)

// Room errors (46000-46999)
var (
	ErrRoomNotFound    = New(46401, "房间不存在")
	ErrRoomFull        = New(46901, "房间已满")
	ErrRoomNotJoinable = New(46402, "房间不可加入")
	ErrNotRoomOwner    = New(46403, "非房主操作")
	ErrGameRunning     = New(46404, "游戏进行中")
)

// Server errors (50000-50999)
var (
	ErrInternalServer = New(50000, "服务器内部错误")
	ErrDatabaseError  = New(50001, "数据库错误")
	ErrCacheError     = New(50002, "缓存错误")
)

// IsAppError checks if an error is an AppError.
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError extracts an AppError from any error.
func GetAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return ErrInternalServer.WithData(err.Error())
}

var _ error = (*AppError)(nil)

func init() {
	if ErrNotFound.HTTPStatus() != http.StatusNotFound {
		panic("ErrNotFound HTTP status mapping broken")
	}
}
