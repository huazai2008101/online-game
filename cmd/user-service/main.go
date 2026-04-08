package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"online-game/internal/server"
	"online-game/internal/user"
	"online-game/pkg/auth"
	"online-game/pkg/config"
	"online-game/pkg/db"
	grpcserver "online-game/pkg/grpc"
	pkgredis "online-game/pkg/redis"
	pb "online-game/proto/user"
)

func main() {
	cfg := config.Load("user-service")
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	slog.Info("starting user-service", "port", cfg.Port, "grpc_port", cfg.GRPCPort)

	database, err := db.New(&cfg.Database)
	if err != nil {
		slog.Error("database error", "error", err)
		os.Exit(1)
	}
	defer db.Close(database)

	// Redis cache
	redisClient := pkgredis.NewClient(&cfg.Redis)
	if err := pkgredis.Health(redisClient); err != nil {
		slog.Warn("redis not available, caching disabled", "error", err)
		redisClient = nil
	} else {
		defer pkgredis.Close(redisClient)
		slog.Info("redis connected", "addr", cfg.RedisAddr())
	}

	jwtManager := auth.NewJWTManager(cfg.JWTSecret, time.Duration(cfg.JWTExpiryHours)*time.Hour)
	jwtManager.SetRedisClient(redisClient)

	userSvc := user.NewService(database, jwtManager)
	userSvc.SetCache(redisClient)
	userSvc.Migrate()

	// gRPC server
	userGRPC := user.NewGRPCServer(userSvc)
	grpcSrv, err := grpcserver.StartServer(cfg.Host+":"+cfg.GRPCPort, func(srv *grpc.Server) {
		pb.RegisterUserServiceServer(srv, userGRPC)
	})
	if err != nil {
		slog.Error("gRPC server failed", "error", err)
		os.Exit(1)
	}
	defer grpcserver.GracefulStop(grpcSrv)

	// HTTP server
	userHandler := user.NewHandler(userSvc)

	srv := server.New(&server.ServerConfig{Host: cfg.Host, Port: cfg.Port, Env: cfg.Env})
	userHandler.RegisterRoutes(srv.Router().Group("/api/v1"))
	srv.Router().GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "user-service"})
	})

	go srv.Start()
	srv.WaitForShutdown()
}
