package user

import (
	"context"

	pb "online-game/proto/user"
)

// GRPCServer implements the UserService gRPC server.
type GRPCServer struct {
	pb.UnimplementedUserServiceServer
	svc *Service
}

// NewGRPCServer creates a new gRPC server wrapping the user Service.
func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// ValidateToken validates a JWT token via gRPC.
func (s *GRPCServer) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	claims, err := s.svc.ValidateToken(req.Token)
	if err != nil {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}
	return &pb.ValidateTokenResponse{
		Valid:    true,
		UserId:   int64(claims.UserID),
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}

// GetUser retrieves user info via gRPC.
func (s *GRPCServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
	user, err := s.svc.GetUserInfo(uint(req.UserId))
	if err != nil {
		return nil, err
	}
	return &pb.UserResponse{
		Id:       int64(user.ID),
		Username: user.Username,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Status:   int32(user.Status),
	}, nil
}
