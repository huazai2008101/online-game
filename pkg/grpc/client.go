package grpcserver

import (
	"fmt"
	"log/slog"

	pbgame "online-game/proto/game"
	pbuser "online-game/proto/user"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserServiceClient wraps a gRPC connection to the user service.
type UserServiceClient struct {
	conn   *grpc.ClientConn
	Client pbuser.UserServiceClient
}

// GameServiceClient wraps a gRPC connection to the game service.
type GameServiceClient struct {
	conn   *grpc.ClientConn
	Client pbgame.GameServiceClient
}

// NewUserServiceClient dials the user service gRPC server.
func NewUserServiceClient(addr string) (*UserServiceClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial user-service %s: %w", addr, err)
	}
	slog.Info("gRPC client connected", "service", "user-service", "addr", addr)
	return &UserServiceClient{conn: conn, Client: pbuser.NewUserServiceClient(conn)}, nil
}

// Close closes the user service client connection.
func (c *UserServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// NewGameServiceClient dials the game service gRPC server.
func NewGameServiceClient(addr string) (*GameServiceClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial game-service %s: %w", addr, err)
	}
	slog.Info("gRPC client connected", "service", "game-service", "addr", addr)
	return &GameServiceClient{conn: conn, Client: pbgame.NewGameServiceClient(conn)}, nil
}

// Close closes the game service client connection.
func (c *GameServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
