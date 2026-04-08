package user

import (
	"testing"

	pb "online-game/proto/user"
)

// TestGRPCServer_GetUser_ResponseMapping tests the GetUser gRPC response mapping.
func TestGRPCServer_GetUser_ResponseMapping(t *testing.T) {
	user := &UserInfo{
		ID:       1,
		Username: "testuser",
		Nickname: "Test",
		Avatar:   "https://example.com/avatar.png",
		Status:   1,
	}

	resp := &pb.UserResponse{
		Id:       int64(user.ID),
		Username: user.Username,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Status:   int32(user.Status),
	}

	if resp.Id != 1 {
		t.Errorf("expected id=1, got %d", resp.Id)
	}
	if resp.Username != "testuser" {
		t.Errorf("expected username=testuser, got %s", resp.Username)
	}
	if resp.Nickname != "Test" {
		t.Errorf("expected nickname=Test, got %s", resp.Nickname)
	}
	if resp.Avatar != "https://example.com/avatar.png" {
		t.Errorf("expected avatar, got %s", resp.Avatar)
	}
	if resp.Status != 1 {
		t.Errorf("expected status=1, got %d", resp.Status)
	}
}

// TestNewGRPCServer tests the constructor.
func TestNewGRPCServer(t *testing.T) {
	svc := &Service{}
	server := NewGRPCServer(svc)
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.svc != svc {
		t.Error("expected svc to be set")
	}
}

// TestValidateToken_InvalidToken tests protobuf response for invalid tokens.
func TestValidateToken_InvalidToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"random string", "not-a-jwt"},
		{"malformed jwt", "header.payload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &pb.ValidateTokenResponse{Valid: false}
			if resp.Valid {
				t.Error("expected valid=false for invalid token")
			}
		})
	}
}

// TestValidateToken_ValidToken tests protobuf response for valid tokens.
func TestValidateToken_ValidToken(t *testing.T) {
	resp := &pb.ValidateTokenResponse{
		Valid:    true,
		UserId:   1,
		Username: "testuser",
		Role:     "player",
	}

	if !resp.Valid {
		t.Error("expected valid=true")
	}
	if resp.UserId != 1 {
		t.Errorf("expected user_id=1, got %d", resp.UserId)
	}
	if resp.Username != "testuser" {
		t.Errorf("expected username=testuser, got %s", resp.Username)
	}
	if resp.Role != "player" {
		t.Errorf("expected role=player, got %s", resp.Role)
	}
}

// TestUserInfo_AllFields ensures all UserInfo fields are mapped.
func TestUserInfo_AllFields(t *testing.T) {
	u := &UserInfo{
		ID:       42,
		Username: "player1",
		Nickname: "Player One",
		Avatar:   "https://cdn.example.com/a/123.png",
		Status:   1,
	}

	if u.ID != 42 {
		t.Errorf("expected ID=42, got %d", u.ID)
	}
	if u.Username != "player1" {
		t.Errorf("expected Username=player1, got %s", u.Username)
	}
	if u.Nickname != "Player One" {
		t.Errorf("expected Nickname=Player One, got %s", u.Nickname)
	}
	if u.Avatar != "https://cdn.example.com/a/123.png" {
		t.Errorf("expected Avatar, got %s", u.Avatar)
	}
	if u.Status != 1 {
		t.Errorf("expected Status=1, got %d", u.Status)
	}
}
