package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"online-game/internal/game"
	"online-game/internal/server"
	"online-game/pkg/actor"
	"online-game/pkg/auth"
	"online-game/pkg/config"
	"online-game/pkg/db"
	grpcserver "online-game/pkg/grpc"
	pkgredis "online-game/pkg/redis"
	"online-game/pkg/websocket"
	pb "online-game/proto/game"
)

func main() {
	cfg := config.Load("game-service")
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	slog.Info("starting game-service", "port", cfg.Port, "grpc_port", cfg.GRPCPort)

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

	actorSystem := actor.NewActorSystem()
	wsGateway := websocket.NewGateway(websocket.DefaultGatewayConfig())

	gameSvc := game.NewService(database, actorSystem, wsGateway, redisClient)
	gameSvc.Migrate()

	// JWT manager for auth middleware + WebSocket token validation
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, time.Duration(cfg.JWTExpiryHours)*time.Hour)
	jwtManager.SetRedisClient(redisClient)

	// gRPC server
	gameGRPC := game.NewGRPCServer(gameSvc)
	grpcSrv, err := grpcserver.StartServer(cfg.Host+":"+cfg.GRPCPort, func(srv *grpc.Server) {
		pb.RegisterGameServiceServer(srv, gameGRPC)
	})
	if err != nil {
		slog.Error("gRPC server failed", "error", err)
		os.Exit(1)
	}
	defer grpcserver.GracefulStop(grpcSrv)

	// HTTP server
	gameHandler := game.NewHandler(gameSvc, jwtManager)

	srv := server.New(&server.ServerConfig{Host: cfg.Host, Port: cfg.Port, Env: cfg.Env})

	// JWT auth middleware for HTTP API routes
	authMW := func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", fmt.Sprintf("%d", claims.UserID))
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}

	gameHandler.RegisterRoutes(srv.Router().Group("/api/v1", authMW))

	// WebSocket endpoint (auth via query param, handled by handler)
	srv.Router().GET("/ws", gameHandler.HandleWebSocket)

	srv.Router().GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "game-service", "actors": actorSystem.Count()})
	})

	go srv.Start()
	srv.WaitForShutdown()

	actorSystem.Shutdown(10 * time.Second)
}
