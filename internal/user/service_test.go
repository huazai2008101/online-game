package user

import (
	"fmt"
	"testing"
	"time"

	"online-game/pkg/auth"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test DB: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &UserSession{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	jwtManager := auth.NewJWTManager("test-secret-key", 24*time.Hour)
	svc := NewService(db, jwtManager)
	svc.SetCache(nil) // no Redis in tests
	return svc
}

func TestService_Register_Success(t *testing.T) {
	svc := newTestService(t)

	user, err := svc.Register(&RegisterRequest{
		Username: "testuser",
		Password: "password123",
		Email:    "test@example.com",
		Nickname: "TestNick",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if user.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if user.Username != "testuser" {
		t.Errorf("expected username=testuser, got %s", user.Username)
	}
	if user.Nickname != "TestNick" {
		t.Errorf("expected nickname=TestNick, got %s", user.Nickname)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email, got %s", user.Email)
	}
	if user.Status != 1 {
		t.Errorf("expected status=1, got %d", user.Status)
	}
}

func TestService_Register_DuplicateUsername(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Register(&RegisterRequest{
		Username: "testuser",
		Password: "password123",
		Email:    "test1@example.com",
	})
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	_, err = svc.Register(&RegisterRequest{
		Username: "testuser",
		Password: "password456",
		Email:    "test2@example.com",
	})
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestService_Register_NicknameDefaultsToUsername(t *testing.T) {
	svc := newTestService(t)

	user, err := svc.Register(&RegisterRequest{
		Username: "nonick",
		Password: "password123",
		Email:    "nonick@example.com",
		// Nickname intentionally empty
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if user.Nickname != "nonick" {
		t.Errorf("expected nickname to default to username, got %s", user.Nickname)
	}
}

func TestService_Login_Success(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Register(&RegisterRequest{
		Username: "loginuser",
		Password: "mypassword",
		Email:    "login@example.com",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	resp, err := svc.Login(&LoginRequest{
		Username: "loginuser",
		Password: "mypassword",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.User == nil {
		t.Fatal("expected user in response")
	}
	if resp.User.Username != "loginuser" {
		t.Errorf("expected username=loginuser, got %s", resp.User.Username)
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Register(&RegisterRequest{
		Username: "wrongpwd",
		Password: "password123",
		Email:    "wrong@example.com",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	_, err = svc.Login(&LoginRequest{
		Username: "wrongpwd",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestService_Login_UserNotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Login(&LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestService_GetUser_Success(t *testing.T) {
	svc := newTestService(t)

	regUser, err := svc.Register(&RegisterRequest{
		Username: "getuser",
		Password: "password123",
		Email:    "getuser@example.com",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	user, err := svc.GetUser(regUser.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user.ID != regUser.ID {
		t.Errorf("expected ID=%d, got %d", regUser.ID, user.ID)
	}
}

func TestService_GetUser_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.GetUser(99999)
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestService_GetUserInfo(t *testing.T) {
	svc := newTestService(t)

	regUser, _ := svc.Register(&RegisterRequest{
		Username: "userinfo",
		Password: "password123",
		Email:    "userinfo@example.com",
		Nickname: "Info User",
	})

	info, err := svc.GetUserInfo(regUser.ID)
	if err != nil {
		t.Fatalf("GetUserInfo failed: %v", err)
	}
	if info.ID != regUser.ID {
		t.Errorf("expected ID=%d, got %d", regUser.ID, info.ID)
	}
	if info.Nickname != "Info User" {
		t.Errorf("expected Nickname=Info User, got %s", info.Nickname)
	}
}

func TestService_ValidateToken(t *testing.T) {
	svc := newTestService(t)

	regUser, _ := svc.Register(&RegisterRequest{
		Username: "tokenuser",
		Password: "password123",
		Email:    "token@example.com",
	})

	resp, _ := svc.Login(&LoginRequest{
		Username: "tokenuser",
		Password: "password123",
	})

	claims, err := svc.ValidateToken(resp.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != regUser.ID {
		t.Errorf("expected UserID=%d, got %d", regUser.ID, claims.UserID)
	}
}

func TestService_ValidateToken_Invalid(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.ValidateToken("invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestService_ListUsers(t *testing.T) {
	svc := newTestService(t)

	for i := 0; i < 5; i++ {
		_, err := svc.Register(&RegisterRequest{
			Username: fmt.Sprintf("listuser%d", i),
			Password: "password123",
			Email:    fmt.Sprintf("list%d@example.com", i),
		})
		if err != nil {
			t.Fatalf("Register %d failed: %v", i, err)
		}
	}

	users, total, err := svc.ListUsers(1, 10)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(users) != 5 {
		t.Errorf("expected 5 users, got %d", len(users))
	}
}

func TestService_ListUsers_Pagination(t *testing.T) {
	svc := newTestService(t)

	for i := 0; i < 5; i++ {
		_, _ = svc.Register(&RegisterRequest{
			Username: fmt.Sprintf("pageuser%d", i),
			Password: "password123",
			Email:    fmt.Sprintf("page%d@example.com", i),
		})
	}

	users, total, err := svc.ListUsers(2, 2)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users on page 2, got %d", len(users))
	}
}
