# 错误处理设计指南

本文档定义统一的 API 错误处理机制。

## 设计原则

1. **HTTP 200**: 请求成功，Body 包含业务数据
2. **Non-200**: 请求失败，HTTP 状态码表示错误类型
3. **X-Error-Message Header**: 失败时返回可读的错误信息
4. **X-Request-ID Header**: 请求追踪 ID
6. **Body**: 成功时返回业务数据；失败时通常为空，但 AppError 有 Data 字段时可返回额外数据

---

## 1. AppError 定义

```go
// pkg/apperror/error.go
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError 自定义应用错误类型
type AppError struct {
	Code      int    // 业务错误码
	Message   string // 错误信息（会设置到 X-Error-Message header）
	Internal  error  // 内部错误（用于日志，不暴露给客户端）
	Operation string // 操作名称（用于日志追踪）
	Data      any    // 业务数据（可选，失败时如需返回额外数据使用）
}

// Error 实现 error 接口
func (e AppError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Internal)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 实现 errors.Unwrap 接口
func (e AppError) Unwrap() error {
	return e.Internal
}

// HTTPStatus 返回对应的 HTTP 状态码
func (e AppError) HTTPStatus() int {
	// 根据错误码范围映射到标准 HTTP 状态码
	switch {
	case e.Code >= 40000 && e.Code < 40100:
		return http.StatusBadRequest
	case e.Code >= 40100 && e.Code < 40200:
		return http.StatusUnauthorized
	case e.Code >= 40300 && e.Code < 40400:
		return http.StatusForbidden
	case e.Code >= 40400 && e.Code < 40500:
		return http.StatusNotFound
	case e.Code >= 40500 && e.Code < 40600:
		return http.StatusMethodNotAllowed
	case e.Code >= 40900 && e.Code < 41000:
		return http.StatusConflict
	case e.Code >= 42900 && e.Code < 43000:
		return http.StatusTooManyRequests
	case e.Code >= 40000 && e.Code < 50000:
		return http.StatusBadRequest
	case e.Code >= 50000:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// WithMessage 返回带有自定义消息的新 AppError
// 注意：返回新对象，原对象不变
func (e AppError) WithMessage(msg string) AppError {
	return AppError{
		Code:      e.Code,
		Message:   msg,
		Internal:  e.Internal,
		Operation: e.Operation,
		Data:      e.Data,
	}
}

// WithMessagef 返回带有格式化消息的新 AppError
func (e AppError) WithMessagef(format string, args ...any) AppError {
	return AppError{
		Code:      e.Code,
		Message:   fmt.Sprintf(format, args...),
		Internal:  e.Internal,
		Operation: e.Operation,
		Data:      e.Data,
	}
}

// WithData 返回带有业务数据的新 AppError
// 失败时如需返回额外数据给客户端，使用此方法
func (e AppError) WithData(data any) AppError {
	return AppError{
		Code:      e.Code,
		Message:   e.Message,
		Internal:  e.Internal,
		Operation: e.Operation,
		Data:      data,
	}
}

// WithOperation 返回带有操作名称的新 AppError
func (e AppError) WithOperation(op string) AppError {
	return AppError{
		Code:      e.Code,
		Message:   e.Message,
		Internal:  e.Internal,
		Operation: op,
		Data:      e.Data,
	}
}

// Wrap 包装底层错误，返回新 AppError
func (e AppError) Wrap(err error) AppError {
	if err == nil {
		return e
	}
	return AppError{
		Code:      e.Code,
		Message:   e.Message,
		Internal:  err,
		Operation: e.Operation,
		Data:      e.Data,
	}
}

// ==================== 预定义错误变量 ====================

var (
	// 4xx 客户端错误 (40000-49999)
	ErrBadRequest      = AppError{Code: 40000, Message: "Bad Request"}
	ErrUnauthorized    = AppError{Code: 40100, Message: "Unauthorized"}
	ErrTokenExpired    = AppError{Code: 40101, Message: "Token Expired"}
	ErrTokenInvalid    = AppError{Code: 40102, Message: "Invalid Token"}
	ErrForbidden       = AppError{Code: 40300, Message: "Forbidden"}
	ErrNotFound        = AppError{Code: 40400, Message: "Not Found"}
	ErrMethodNotAllowed = AppError{Code: 40500, Message: "Method Not Allowed"}
	ErrConflict        = AppError{Code: 40900, Message: "Conflict"}
	ErrResourceExists  = AppError{Code: 40901, Message: "Resource Already Exists"}
	ErrInvalidState    = AppError{Code: 40902, Message: "Invalid State"}
	ErrRateLimit       = AppError{Code: 42900, Message: "Rate Limit Exceeded"}

	// 5xx 服务器错误 (50000-59999)
	ErrInternal           = AppError{Code: 50000, Message: "Internal Server Error"}
	ErrDatabase           = AppError{Code: 50001, Message: "Database Error"}
	ErrCache              = AppError{Code: 50002, Message: "Cache Error"}
	ErrServiceUnavailable = AppError{Code: 50300, Message: "Service Unavailable"}
	ErrGatewayTimeout     = AppError{Code: 50400, Message: "Gateway Timeout"}
)

// ==================== 工具函数 ====================

// As 将 err 断言为 AppError
func As(err error) (AppError, bool) {
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return AppError{}, false
}

// IsCode 判断 err 是否为指定错误码
func IsCode(err error, code int) bool {
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// New 创建一个新的 AppError
func New(code int, message string) AppError {
	return AppError{Code: code, Message: message}
}
```

---

## 2. Response 响应处理

```go
// pkg/response/response.go
package response

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"your-project/pkg/apperror"
)

// Success 成功响应，HTTP 200
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// Created 创建成功响应，HTTP 201
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}

// NoContent 无内容响应，HTTP 204
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error 错误响应
// 对 err 断言是否为 AppError：
//   - 是 AppError：使用其 HTTPStatus() 和 Message
//   - 不是 AppError：使用 500 和 err.Error()
//
// 如果 AppError.Data 不为空，则响应 body 包含该数据
func Error(c *gin.Context, err error) {
	if err == nil {
		// nil 错误视为成功
		return
	}

	var appErr apperror.AppError

	// 断言是否为 AppError
	if errors.As(err, &appErr) {
		// 是 AppError，使用其 HTTPStatus() 和 Message
		c.Header("X-Error-Message", appErr.Message)

		// 有 Data 则返回 body，否则只返回状态码
		if appErr.Data != nil {
			c.JSON(appErr.HTTPStatus(), appErr.Data)
		} else {
			c.Status(appErr.HTTPStatus())
		}

		// 记录日志
		logError(c, appErr)
		return
	}

	// 不是 AppError，使用默认值
	c.Header("X-Error-Message", err.Error())
	c.Status(http.StatusInternalServerError)

	// 记录日志
	logError(c, err)
}

// logError 记录错误日志
func logError(c *gin.Context, err error) {
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = c.GetString("request_id")
	}

	if appErr, ok := apperror.As(err); ok {
		// AppError 日志
		fields := []any{
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"code", appErr.Code,
			"message", appErr.Message,
		}
		if appErr.Operation != "" {
			fields = append(fields, "operation", appErr.Operation)
		}
		if appErr.Internal != nil {
			fields = append(fields, "internal", appErr.Internal.Error())
		}
		// logger.Error("request error", fields...)
		fmt.Printf("[ERROR] request_id=%s method=%s path=%s code=%d message=%s\n",
			requestID, c.Request.Method, c.Request.URL.Path, appErr.Code, appErr.Message)
		if appErr.Internal != nil {
			fmt.Printf("  internal: %v\n", appErr.Internal)
		}
		return
	}

	// 普通 error 日志
	// logger.Error("request error", "request_id", requestID, "error", err)
	fmt.Printf("[ERROR] request_id=%s method=%s path=%s error=%v\n",
		requestID, c.Request.Method, c.Request.URL.Path, err)
}
```

---

## 3. 中间件

```go
// pkg/middleware/request_id.go
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDKey = "request_id"

// RequestID 为每个请求生成唯一 ID
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否已有 Request ID（客户端传入或网关设置）
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// 保存到 context
		c.Set(RequestIDKey, requestID)

		// 返回给客户端
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
```

```go
// pkg/middleware/recovery.go
package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"your-project/pkg/response"
)

// Recovery 捕获 panic 并转换为错误响应
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 记录 panic 日志
				requestID := c.GetString("request_id")
				fmt.Printf("[PANIC] request_id=%s method=%s path=%s panic=%v\n",
					requestID, c.Request.Method, c.Request.URL.Path, r)

				// 返回 500 错误
				response.Error(c, fmt.Errorf("panic recovered"))
			}
		}()
		c.Next()
	}
}
```

---

## 4. Handler 使用示例

```go
// internal/handler/user.go
package handler

import (
	"github.com/gin-gonic/gin"
	"your-project/pkg/apperror"
	"your-project/pkg/response"
)

type UserHandler struct {
	userService *UserService
	authService *AuthService
}

// Register 用户注册
func (h *UserHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Invalid parameter: "+err.Error()))
		return
	}

	// 检查用户名是否存在
	exists, err := h.userService.Exists(c, req.Username)
	if err != nil {
		response.Error(c, apperror.ErrInternal.Wrap(err).WithOperation("user.exists"))
		return
	}

	if exists {
		response.Error(c, apperror.ErrResourceExists.
			WithMessage("Username already exists").
			WithData(map[string]string{
				"field":    "username",
				"username": req.Username,
			}))
		return
	}

	// 创建用户
	user, err := h.userService.Create(c, req)
	if err != nil {
		response.Error(c, apperror.ErrInternal.Wrap(err).WithOperation("user.create"))
		return
	}

	response.Success(c, gin.H{
		"user_id":    user.ID,
		"username":   user.Username,
		"created_at": user.CreatedAt,
	})
}

// Login 用户登录
func (h *UserHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Invalid parameter"))
		return
	}

	user, err := h.userService.Validate(c, req.Username, req.Password)
	if err != nil {
		// 判断是否是认证错误
		if apperror.IsCode(err, apperror.ErrUnauthorized.Code) {
			response.Error(c, apperror.ErrUnauthorized.WithMessage("Invalid username or password"))
			return
		}
		response.Error(c, apperror.ErrInternal.Wrap(err).WithOperation("user.validate"))
		return
	}

	token, err := h.authService.Generate(user.ID)
	if err != nil {
		response.Error(c, apperror.ErrInternal.Wrap(err))
		return
	}

	response.Success(c, gin.H{
		"user_id":    user.ID,
		"token":      token,
		"expires_at": token.ExpiresAt,
	})
}

// GetProfile 获取用户资料
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	user, err := h.userService.GetByID(c, userID)
	if err != nil {
		if apperror.IsCode(err, apperror.ErrNotFound.Code) {
			response.Error(c, apperror.ErrNotFound.WithMessage("User not found"))
			return
		}
		response.Error(c, apperror.ErrInternal.Wrap(err))
		return
	}

	response.Success(c, user)
}

// UpdateProfile 更新用户资料
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Nickname string `json:"nickname" binding:"required,min=2,max=20"`
		Avatar   string `json:"avatar" binding:"omitempty,url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Validation failed: "+err.Error()))
		return
	}

	user, err := h.userService.UpdateProfile(c, userID, req.Nickname, req.Avatar)
	if err != nil {
		if apperror.IsCode(err, apperror.ErrNotFound.Code) {
			response.Error(c, apperror.ErrNotFound)
			return
		}
		response.Error(c, apperror.ErrInternal.Wrap(err))
		return
	}

	response.Success(c, user)
}
```

---

## 5. Service 层返回错误

```go
// internal/service/user.go
package service

import (
	"your-project/pkg/apperror"
)

type UserService struct {
	repo *repository.UserRepository
}

func (s *UserService) GetByID(ctx context.Context, userID int64) (*User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrNotFound.WithMessage("User not found")
		}
		return nil, apperror.ErrDatabase.Wrap(err).WithOperation("user.find_by_id")
	}
	return user, nil
}

func (s *UserService) Validate(ctx context.Context, username, password string) (*User, error) {
	user, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrUnauthorized.WithMessage("Invalid username or password")
		}
		return nil, apperror.ErrDatabase.Wrap(err).WithOperation("user.find_by_username")
	}

	if !user.VerifyPassword(password) {
		return nil, apperror.ErrUnauthorized.WithMessage("Invalid username or password")
	}

	return user, nil
}
```

---

## 6. 路由配置

```go
// internal/router/router.go
package router

import (
	"github.com/gin-gonic/gin"
	"your-project/internal/handler"
	"your-project/pkg/middleware"
)

func Setup(r *gin.Engine) {
	// 全局中间件
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"request_id": c.GetString("request_id"),
		})
	})

	// 初始化 handler
	userHandler := handler.NewUserHandler()

	// API 路由组
	api := r.Group("/api/v1")
	{
		// 公开路由
		public := api.Group("")
		{
			public.POST("/users/register", userHandler.Register)
			public.POST("/users/login", userHandler.Login)
		}

		// 需要认证的路由
		auth := api.Group("")
		auth.Use(middleware.Auth())
		{
			auth.GET("/users/profile", userHandler.GetProfile)
			auth.PUT("/users/profile", userHandler.UpdateProfile)
		}
	}

	// 404 处理
	r.NoRoute(func(c *gin.Context) {
		c.Header("X-Error-Message", "API endpoint not found")
		c.Status(404)
	})
}
```

---

## 7. 客户端处理示例

### TypeScript

```typescript
class APIError extends Error {
  constructor(
    public statusCode: number,
    message: string,
    public data?: any
  ) {
    super(message);
    this.name = 'APIError';
  }
}

class APIClient {
  async request<T>(method: string, path: string, body?: any): Promise<T> {
    const response = await fetch(`/api/v1${path}`, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined,
    });

    // 检查 HTTP 状态码
    if (response.status !== 200) {
      const errorMessage = response.headers.get('X-Error-Message') || 'Request failed';

      // 尝试读取错误 body 中的数据
      let errorData;
      try {
        errorData = await response.json();
      } catch {
        // body 为空或非 JSON，忽略
      }

      throw new APIError(response.status, errorMessage, errorData);
    }

    return response.json();
  }
}

// 使用示例
const client = new APIClient();

try {
  const user = await client.request('POST', '/users/login', {
    username: 'player1',
    password: 'password123',
  });
  console.log('登录成功:', user);
} catch (error) {
  if (error instanceof APIError) {
    console.error(`登录失败: ${error.message}`);

    // 如果有 data，可以显示更详细的错误信息
    if (error.data) {
      console.log('错误详情:', error.data);
      // 例如: { field: "username", reason: "required" }
    }

    // 根据状态码处理
    switch (error.statusCode) {
      case 401:
        console.log('用户名或密码错误');
        break;
      case 400:
        console.log('参数错误');
        break;
      default:
        console.log('服务器错误');
    }
  }
}
```

---

## 8. 测试示例

```go
// pkg/response/response_test.go
package response_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"your-project/pkg/apperror"
	"your-project/pkg/response"
)

func TestErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		err             error
		expectedStatus  int
		expectedMessage string
	}{
		{
			name:            "AppError - ErrNotFound",
			err:             apperror.ErrNotFound,
			expectedStatus:  http.StatusNotFound,
			expectedMessage: "Not Found",
		},
		{
			name:            "AppError - ErrUnauthorized",
			err:             apperror.ErrUnauthorized,
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "Unauthorized",
		},
		{
			name:            "AppError - WithMessage",
			err:             apperror.ErrNotFound.WithMessage("User not found"),
			expectedStatus:  http.StatusNotFound,
			expectedMessage: "User not found",
		},
		{
			name:            "Plain error",
			err:             errors.New("database failed"),
			expectedStatus:  http.StatusInternalServerError,
			expectedMessage: "database failed",
		},
		{
			name:            "nil error",
			err:             nil,
			expectedStatus:  http.StatusOK, // 默认 200
			expectedMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			response.Error(c, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedMessage, w.Header().Get("X-Error-Message"))
		})
	}
}

func TestErrorResponseWithData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := apperror.ErrBadRequest.WithData(map[string]any{
		"field": "username",
		"reason": "required",
	})
	response.Error(c, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Bad Request", w.Header().Get("X-Error-Message"))
	assert.Contains(t, w.Body.String(), `"username"`)
	assert.Contains(t, w.Body.String(), `"reason"`)
}

func TestHTTPStatusMapping(t *testing.T) {
	tests := []struct {
		appErr          apperror.AppError
		expectedStatus  int
	}{
		{apperror.ErrBadRequest, http.StatusBadRequest},
		{apperror.ErrUnauthorized, http.StatusUnauthorized},
		{apperror.ErrTokenExpired, http.StatusUnauthorized},
		{apperror.ErrForbidden, http.StatusForbidden},
		{apperror.ErrNotFound, http.StatusNotFound},
		{apperror.ErrConflict, http.StatusConflict},
		{apperror.ErrRateLimit, http.StatusTooManyRequests},
		{apperror.ErrInternal, http.StatusInternalServerError},
		{apperror.ErrDatabase, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.appErr.Message, func(t *testing.T) {
			assert.Equal(t, tt.expectedStatus, tt.appErr.HTTPStatus())
		})
	}
}
```

---

## 9. 错误码规范

### 格式: `XXXXX`
- 第 1 位: HTTP 类别 (4=客户端错误, 5=服务器错误)
- 第 2-3 位: 具体状态码 (00, 01, 02...)
- 第 4-5 位: 序号 (00-99)

### 预定义错误码列表

| 错误码 | HTTP状态 | 说明 |
|--------|----------|------|
| 40000 | 400 | 请求参数错误 |
| 40100 | 401 | 未认证 |
| 40101 | 401 | Token过期 |
| 40102 | 401 | Token无效 |
| 40300 | 403 | 无权限 |
| 40400 | 404 | 资源不存在 |
| 40500 | 405 | 方法不允许 |
| 40900 | 409 | 冲突 |
| 40901 | 409 | 资源已存在 |
| 40902 | 409 | 状态无效 |
| 42900 | 429 | 请求限流 |
| 50000 | 500 | 内部错误 |
| 50001 | 500 | 数据库错误 |
| 50002 | 500 | 缓存错误 |
| 50300 | 503 | 服务不可用 |
| 50400 | 504 | 网关超时 |

---

## 10. 响应格式总结

| 场景 | HTTP状态 | X-Error-Message | Body |
|------|----------|-----------------|------|
| 成功 | 200 | - | 业务数据JSON |
| 失败（无Data） | 4xx/5xx | 错误描述 | 空 |
| 失败（有Data） | 4xx/5xx | 错误描述 | Data 字段内容 |

---

## 11. 使用要点

### Handler 层
```go
// ✅ 正确 - 使用 response.Error
response.Error(c, apperror.ErrNotFound)
response.Error(c, apperror.ErrBadRequest.WithMessage("参数错误"))
response.Error(c, err) // 普通 error 自动转 500

// ❌ 错误 - 不要直接使用 c.JSON
c.JSON(400, gin.H{"error": "message"}) // 不符合规范
```

### Service 层
```go
// ✅ 正确 - 返回 AppError
return nil, apperror.ErrNotFound
return nil, apperror.ErrDatabase.Wrap(err).WithOperation("user.find")

// ❌ 错误 - 不要返回普通 error（除非是底层库错误）
return nil, errors.New("not found") // 应该用 AppError
```

### 链式调用
```go
// ✅ 正确 - 链式调用创建新对象
err := apperror.ErrNotFound.
    WithMessage("User not found").
    WithOperation("user.get")

// ⚠️ 注意 - 原对象不变
original := apperror.ErrNotFound
modified := original.WithMessage("new message")
// original.Message 仍然是 "Not Found"
```

### WithData 使用
```go
// 失败时需要返回额外数据给客户端
response.Error(c, apperror.ErrBadRequest.
    WithMessage("Validation failed").
    WithData(map[string]any{
        "field":   "username",
        "reason":  "too short",
        "minimum": 3,
    }))

// 客户端收到的响应：
// HTTP/1.1 400 Bad Request
// X-Error-Message: Validation failed
// Content-Type: application/json
//
// {"field": "username", "reason": "too short", "minimum": 3}
```
