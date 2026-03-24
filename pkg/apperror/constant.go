package apperror

// 业务错误码，与 HTTP 状态码保持一致
const (
	// 1xx 信息响应 (暂不使用)

	// 2xx 成功响应
	OK              = 200
	Created         = 201
	Accepted        = 202
	NoContent       = 204

	// 3xx 重定向 (暂不使用)

	// 4xx 客户端错误
	BadRequest           = 400
	Unauthorized         = 401
	PaymentRequired      = 402
	Forbidden            = 403
	NotFound             = 404
	MethodNotAllowed     = 405
	NotAcceptable        = 406
	ProxyAuthRequired    = 407
	RequestTimeout       = 408
	Conflict             = 409
	Gone                 = 410
	LengthRequired       = 411
	PreconditionFailed   = 412
	PayloadTooLarge      = 413
	URITooLong           = 414
	UnsupportedMediaType = 415
	RangeNotSatisfiable  = 416
	ExpectationFailed     = 417
	ImATeapot            = 418
	MisdirectedRequest   = 421
	UnprocessableEntity  = 422
	Locked               = 423
	FailedDependency     = 424
	TooEarly             = 425
	UpgradeRequired      = 426
	PreconditionRequired = 428
	TooManyRequests      = 429

	// 5xx 服务器错误
	InternalServerError           = 500
	NotImplemented                = 501
	BadGateway                    = 502
	ServiceUnavailable            = 503
	GatewayTimeout                = 504
	HTTPVersionNotSupported       = 505
	VariantAlsoNegotiates         = 506
	InsufficientStorage           = 507
	LoopDetected                  = 508
	NotExtended                   = 510
	NetworkAuthenticationRequired = 511
)

// 预定义错误实例
var (
	// 通用错误
	ErrInternalServer = AppError{Code: InternalServerError, Message: "服务器内部错误"}
	ErrNotFound       = AppError{Code: NotFound, Message: "资源不存在"}
	ErrBadRequest     = AppError{Code: BadRequest, Message: "请求参数错误"}
	ErrUnauthorized   = AppError{Code: Unauthorized, Message: "未授权"}
	ErrForbidden      = AppError{Code: Forbidden, Message: "无权限访问"}
	ErrConflict       = AppError{Code: Conflict, Message: "资源冲突"}
	ErrTooManyRequest = AppError{Code: TooManyRequests, Message: "请求过于频繁"}

	// 用户相关错误 (4000-4099)
	ErrUserNotFound      = AppError{Code: 40401, Message: "用户不存在"}
	ErrUserAlreadyExists = AppError{Code: 40901, Message: "用户已存在"}
	ErrInvalidPassword   = AppError{Code: 40001, Message: "密码错误"}
	ErrInvalidUsername   = AppError{Code: 40002, Message: "用户名格式错误"}
	ErrInvalidEmail      = AppError{Code: 40003, Message: "邮箱格式错误"}

	// 游戏相关错误 (4100-4199)
	ErrGameNotFound       = AppError{Code: 41401, Message: "游戏不存在"}
	ErrGameAlreadyExists  = AppError{Code: 41901, Message: "游戏已存在"}
	ErrGameNotRunning     = AppError{Code: 41402, Message: "游戏未运行"}
	ErrGameFull           = AppError{Code: 41902, Message: "游戏已满员"}
	ErrInvalidGameVersion = AppError{Code: 40004, Message: "游戏版本不兼容"}

	// 玩家相关错误 (4200-4299)
	ErrPlayerNotFound      = AppError{Code: 42401, Message: "玩家不存在"}
	ErrPlayerAlreadyExists = AppError{Code: 42901, Message: "玩家已存在"}
	ErrPlayerNotInGame     = AppError{Code: 42402, Message: "玩家不在游戏中"}
	ErrPlayerAlreadyInGame = AppError{Code: 42902, Message: "玩家已在游戏中"}

	// 支付相关错误 (4300-4399)
	ErrPaymentNotFound    = AppError{Code: 43401, Message: "支付记录不存在"}
	ErrPaymentFailed      = AppError{Code: 43901, Message: "支付失败"}
	ErrInvalidAmount      = AppError{Code: 43001, Message: "金额无效"}
	ErrInsufficientFunds  = AppError{Code: 43902, Message: "余额不足"}
	ErrOrderNotFound      = AppError{Code: 43402, Message: "订单不存在"}
	ErrOrderAlreadyPaid   = AppError{Code: 43903, Message: "订单已支付"}
	ErrInsufficientScore  = AppError{Code: 43904, Message: "积分不足"}

	// 通知相关错误 (4400-4499)
	ErrNotificationNotFound = AppError{Code: 44401, Message: "通知不存在"}
	ErrInvalidNotifyType    = AppError{Code: 44001, Message: "通知类型无效"}

	// 匹配相关错误 (4500-4599)
	ErrMatchNotFound    = AppError{Code: 45401, Message: "匹配不存在"}
	ErrMatchTimeout     = AppError{Code: 45901, Message: "匹配超时"}
	ErrNoAvailableMatch = AppError{Code: 45402, Message: "无可用匹配"}
)
