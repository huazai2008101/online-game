package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "online-game/proto/user"
)

// mockUserService implements pb.UserServiceServer for testing.
type mockUserService struct {
	pb.UnimplementedUserServiceServer
}

func (m *mockUserService) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	if req.Token == "valid-token" {
		return &pb.ValidateTokenResponse{
			Valid:    true,
			UserId:   1,
			Username: "testuser",
			Role:     "player",
		}, nil
	}
	return &pb.ValidateTokenResponse{Valid: false}, nil
}

func (m *mockUserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
	return &pb.UserResponse{
		Id:       req.UserId,
		Username: "testuser",
		Nickname: "Test",
		Status:   1,
	}, nil
}

func TestStartServerAndClient(t *testing.T) {
	// Start a gRPC server on a random port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterUserServiceServer(srv, &mockUserService{})

	go func() {
		srv.Serve(lis)
	}()
	defer srv.GracefulStop()

	// Dial the server
	addr := lis.Addr().String()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test ValidateToken - valid
	resp, err := client.ValidateToken(ctx, &pb.ValidateTokenRequest{Token: "valid-token"})
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if !resp.Valid {
		t.Error("expected valid=true")
	}
	if resp.Username != "testuser" {
		t.Errorf("expected username=testuser, got %s", resp.Username)
	}

	// Test ValidateToken - invalid
	resp, err = client.ValidateToken(ctx, &pb.ValidateTokenRequest{Token: "bad-token"})
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid=false for bad token")
	}

	// Test GetUser
	userResp, err := client.GetUser(ctx, &pb.GetUserRequest{UserId: 42})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if userResp.Id != 42 {
		t.Errorf("expected id=42, got %d", userResp.Id)
	}
	if userResp.Username != "testuser" {
		t.Errorf("expected username=testuser, got %s", userResp.Username)
	}
}

func TestNewUserServiceClient(t *testing.T) {
	// Test that client creation fails gracefully for unreachable address
	_, err := NewUserServiceClient("localhost:19999")
	if err != nil {
		// NewClient doesn't actually connect until first RPC, so this should succeed
		// but the connection will fail on actual calls
		t.Logf("NewUserServiceClient returned error (expected for unreachable): %v", err)
	}
}
