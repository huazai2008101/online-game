package grpcserver

import (
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// StartServer starts a gRPC server on the given address and registers services.
// Returns a stop function the caller should defer.
func StartServer(addr string, register func(srv *grpc.Server)) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("grpc listen %s: %w", addr, err)
	}

	srv := grpc.NewServer()
	register(srv)
	reflection.Register(srv) // useful for grpcurl debugging

	go func() {
		slog.Info("gRPC server starting", "addr", addr)
		if err := srv.Serve(lis); err != nil {
			slog.Error("gRPC server error", "addr", addr, "error", err)
		}
	}()

	return srv, nil
}

// GracefulStop gracefully stops the gRPC server.
func GracefulStop(srv *grpc.Server) {
	if srv != nil {
		slog.Info("stopping gRPC server...")
		srv.GracefulStop()
	}
}
