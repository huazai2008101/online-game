package apperror

var (
	// Common error codes
	ErrInternalServer = AppError{Code: 500, Message: "服务器内部错误"}
	ErrNotFound       = AppError{Code: 404, Message: "资源不存在"}
)
