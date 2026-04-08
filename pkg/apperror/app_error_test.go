package apperror

import (
	"errors"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	t.Run("without data", func(t *testing.T) {
		e := &AppError{Code: 40400, Message: "资源不存在"}
		got := e.Error()
		want := "[40400] 资源不存在"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("with data", func(t *testing.T) {
		e := &AppError{Code: 40400, Message: "资源不存在", Data: "extra info"}
		got := e.Error()
		want := "[40400] 资源不存在: extra info"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("with nil data", func(t *testing.T) {
		e := &AppError{Code: 40000, Message: "请求参数错误", Data: nil}
		got := e.Error()
		want := "[40000] 请求参数错误"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})
}

func TestAppError_HTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"40000 -> 400", 40000, 400},
		{"40100 -> 401", 40100, 401},
		{"40300 -> 403", 40300, 403},
		{"40400 -> 404", 40400, 404},
		{"50000 -> 500", 50000, 500},
		{"50001 -> 500", 50001, 500},
		{"41401 -> 414", 41401, 414},
		{"44401 -> 444", 44401, 444},
		{"46901 -> 469", 46901, 469},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &AppError{Code: tt.code}
			if got := e.HTTPStatus(); got != tt.want {
				t.Errorf("HTTPStatus() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAppError_WithData(t *testing.T) {
	original := &AppError{Code: 40400, Message: "资源不存在"}
	withData := original.WithData("some details")

	// Returns a new instance
	if withData == original {
		t.Error("WithData() should return a new instance")
	}

	// Original is unchanged
	if original.Data != nil {
		t.Error("original Data should be nil")
	}

	// New instance has data
	if withData.Data != "some details" {
		t.Errorf("WithData().Data = %v, want %q", withData.Data, "some details")
	}

	// Code and Message are preserved
	if withData.Code != original.Code {
		t.Errorf("WithData().Code = %d, want %d", withData.Code, original.Code)
	}
	if withData.Message != original.Message {
		t.Errorf("WithData().Message = %q, want %q", withData.Message, original.Message)
	}
}

func TestAppError_WithMessage(t *testing.T) {
	original := &AppError{Code: 40400, Message: "资源不存在", Data: "original-data"}
	withMsg := original.WithMessage("custom message")

	// Returns a new instance
	if withMsg == original {
		t.Error("WithMessage() should return a new instance")
	}

	// Original is unchanged
	if original.Message != "资源不存在" {
		t.Error("original Message should not change")
	}

	// New instance has new message
	if withMsg.Message != "custom message" {
		t.Errorf("WithMessage().Message = %q, want %q", withMsg.Message, "custom message")
	}

	// Code and Data are preserved
	if withMsg.Code != original.Code {
		t.Errorf("WithMessage().Code = %d, want %d", withMsg.Code, original.Code)
	}
	if withMsg.Data != original.Data {
		t.Errorf("WithMessage().Data = %v, want %v", withMsg.Data, original.Data)
	}
}

func TestIsAppError(t *testing.T) {
	t.Run("is AppError", func(t *testing.T) {
		err := ErrNotFound
		if !IsAppError(err) {
			t.Error("IsAppError(ErrNotFound) = false, want true")
		}
	})

	t.Run("is not AppError", func(t *testing.T) {
		err := errors.New("plain error")
		if IsAppError(err) {
			t.Error("IsAppError(plain error) = true, want false")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if IsAppError(nil) {
			t.Error("IsAppError(nil) = true, want false")
		}
	})
}

func TestGetAppError_AppError(t *testing.T) {
	err := ErrNotFound
	appErr := GetAppError(err)
	if appErr != ErrNotFound {
		t.Errorf("GetAppError(ErrNotFound) returned different pointer")
	}
	if appErr.Code != 40400 {
		t.Errorf("GetAppError().Code = %d, want 40400", appErr.Code)
	}
}

func TestGetAppError_OtherError(t *testing.T) {
	plainErr := errors.New("something went wrong")
	appErr := GetAppError(plainErr)

	// Should be wrapped as ErrInternalServer
	if appErr.Code != ErrInternalServer.Code {
		t.Errorf("GetAppError(plain).Code = %d, want %d", appErr.Code, ErrInternalServer.Code)
	}
	if appErr.Message != ErrInternalServer.Message {
		t.Errorf("GetAppError(plain).Message = %q, want %q", appErr.Message, ErrInternalServer.Message)
	}
	// Data should contain the original error string
	if appErr.Data != plainErr.Error() {
		t.Errorf("GetAppError(plain).Data = %v, want %q", appErr.Data, plainErr.Error())
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    *AppError
		code   int
		status int
		msg    string
	}{
		{"ErrBadRequest", ErrBadRequest, 40000, 400, "请求参数错误"},
		{"ErrUnauthorized", ErrUnauthorized, 40100, 401, "未授权"},
		{"ErrForbidden", ErrForbidden, 40300, 403, "禁止访问"},
		{"ErrNotFound", ErrNotFound, 40400, 404, "资源不存在"},
		{"ErrUserNotFound", ErrUserNotFound, 41401, 414, "用户不存在"},
		{"ErrUserExists", ErrUserExists, 41402, 414, "用户已存在"},
		{"ErrInvalidPassword", ErrInvalidPassword, 41403, 414, "密码错误"},
		{"ErrInvalidToken", ErrInvalidToken, 41404, 414, "无效的Token"},
		{"ErrGameNotFound", ErrGameNotFound, 44401, 444, "游戏不存在"},
		{"ErrGameNotReady", ErrGameNotReady, 44402, 444, "游戏未就绪"},
		{"ErrInvalidPackage", ErrInvalidPackage, 44001, 440, "游戏包格式无效"},
		{"ErrScriptError", ErrScriptError, 44002, 440, "脚本执行错误"},
		{"ErrRoomNotFound", ErrRoomNotFound, 46401, 464, "房间不存在"},
		{"ErrRoomFull", ErrRoomFull, 46901, 469, "房间已满"},
		{"ErrRoomNotJoinable", ErrRoomNotJoinable, 46402, 464, "房间不可加入"},
		{"ErrNotRoomOwner", ErrNotRoomOwner, 46403, 464, "非房主操作"},
		{"ErrGameRunning", ErrGameRunning, 46404, 464, "游戏进行中"},
		{"ErrInternalServer", ErrInternalServer, 50000, 500, "服务器内部错误"},
		{"ErrDatabaseError", ErrDatabaseError, 50001, 500, "数据库错误"},
		{"ErrCacheError", ErrCacheError, 50002, 500, "缓存错误"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Code = %d, want %d", tt.err.Code, tt.code)
			}
			if tt.err.HTTPStatus() != tt.status {
				t.Errorf("HTTPStatus() = %d, want %d", tt.err.HTTPStatus(), tt.status)
			}
			if tt.err.Message != tt.msg {
				t.Errorf("Message = %q, want %q", tt.err.Message, tt.msg)
			}
		})
	}
}
