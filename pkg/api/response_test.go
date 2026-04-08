package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"online-game/pkg/apperror"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func TestSuccess(t *testing.T) {
	c, w := newTestContext()
	data := map[string]string{"key": "value"}
	Success(c, data)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("resp.Code = %d, want 0", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("resp.Message = %q, want %q", resp.Message, "success")
	}
}

func TestCreated(t *testing.T) {
	c, w := newTestContext()
	data := map[string]int{"id": 42}
	Created(c, data)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("resp.Code = %d, want 0", resp.Code)
	}
	if resp.Message != "created" {
		t.Errorf("resp.Message = %q, want %q", resp.Message, "created")
	}
}

func TestSuccessWithMessage(t *testing.T) {
	c, w := newTestContext()
	data := "hello"
	SuccessWithMessage(c, "custom ok", data)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("resp.Code = %d, want 0", resp.Code)
	}
	if resp.Message != "custom ok" {
		t.Errorf("resp.Message = %q, want %q", resp.Message, "custom ok")
	}
}

func TestError(t *testing.T) {
	t.Run("AppError", func(t *testing.T) {
		c, w := newTestContext()
		Error(c, apperror.ErrNotFound)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.Code != 40400 {
			t.Errorf("resp.Code = %d, want 40400", resp.Code)
		}
		if resp.Message != "资源不存在" {
			t.Errorf("resp.Message = %q, want %q", resp.Message, "资源不存在")
		}
	})

	t.Run("plain error", func(t *testing.T) {
		c, w := newTestContext()
		Error(c, errors.New("some failure"))

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.Code != 50000 {
			t.Errorf("resp.Code = %d, want 50000", resp.Code)
		}
		if resp.Message != "服务器内部错误" {
			t.Errorf("resp.Message = %q, want %q", resp.Message, "服务器内部错误")
		}
	})
}

func TestPaginated(t *testing.T) {
	c, w := newTestContext()
	items := []string{"a", "b", "c"}
	Paginated(c, items, 100, 2, 10)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("resp.Code = %d, want 0", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("resp.Message = %q, want %q", resp.Message, "success")
	}

	// The Data field holds a PaginatedResponse serialized as map
	dataBytes, _ := json.Marshal(resp.Data)
	var paginated PaginatedResponse
	if err := json.Unmarshal(dataBytes, &paginated); err != nil {
		t.Fatalf("failed to unmarshal paginated data: %v", err)
	}
	if paginated.Total != 100 {
		t.Errorf("Total = %d, want 100", paginated.Total)
	}
	if paginated.Page != 2 {
		t.Errorf("Page = %d, want 2", paginated.Page)
	}
	if paginated.PageSize != 10 {
		t.Errorf("PageSize = %d, want 10", paginated.PageSize)
	}
}

func TestGetPagination(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

		page, pageSize := GetPagination(c)
		if page != 1 {
			t.Errorf("page = %d, want 1", page)
		}
		if pageSize != 20 {
			t.Errorf("pageSize = %d, want 20", pageSize)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?page=3&page_size=50", nil)

		page, pageSize := GetPagination(c)
		if page != 3 {
			t.Errorf("page = %d, want 3", page)
		}
		if pageSize != 50 {
			t.Errorf("pageSize = %d, want 50", pageSize)
		}
	})

	t.Run("invalid page values", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?page=abc&page_size=-5", nil)

		page, pageSize := GetPagination(c)
		if page != 1 {
			t.Errorf("page = %d, want 1 (default for invalid)", page)
		}
		if pageSize != 20 {
			t.Errorf("pageSize = %d, want 20 (default for invalid)", pageSize)
		}
	})

	t.Run("page_size over 100", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?page=1&page_size=200", nil)

		page, pageSize := GetPagination(c)
		if page != 1 {
			t.Errorf("page = %d, want 1", page)
		}
		if pageSize != 20 {
			t.Errorf("pageSize = %d, want 20 (default for over-limit)", pageSize)
		}
	})

	t.Run("page zero", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?page=0&page_size=10", nil)

		page, _ := GetPagination(c)
		if page != 1 {
			t.Errorf("page = %d, want 1 (default for zero)", page)
		}
	})
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"123", 123},
		{"", 0},
		{"abc", 0},
		{"12a3", 0},
		{"-5", 0},
		{" 10", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseInt(tt.input); got != tt.want {
				t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
