package game

import (
	"context"

	pb "online-game/proto/game"
)

// GRPCServer implements the GameService gRPC server.
type GRPCServer struct {
	pb.UnimplementedGameServiceServer
	svc *Service
}

// NewGRPCServer creates a new gRPC server wrapping the game Service.
func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// GetGame retrieves a game by ID via gRPC.
func (s *GRPCServer) GetGame(ctx context.Context, req *pb.GetGameRequest) (*pb.GameResponse, error) {
	game, err := s.svc.GetGame(uint(req.GameId))
	if err != nil {
		return nil, err
	}
	return &pb.GameResponse{
		Id:          int64(game.ID),
		GameCode:    game.GameCode,
		GameName:    game.GameName,
		GameType:    game.GameType,
		MinPlayers:  int32(game.MinPlayers),
		MaxPlayers:  int32(game.MaxPlayers),
		Status:      game.Status,
		Description: game.Description,
	}, nil
}

// ListGames returns a paginated list of games via gRPC.
func (s *GRPCServer) ListGames(ctx context.Context, req *pb.ListGamesRequest) (*pb.ListGamesResponse, error) {
	query := &GameListQuery{
		Status:   req.Status,
		GameType: req.GameType,
		Search:   req.Search,
	}
	page := int(req.Page)
	pageSize := int(req.PageSize)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	games, total, err := s.svc.ListGames(query, page, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &pb.ListGamesResponse{Total: total}
	for _, g := range games {
		resp.Games = append(resp.Games, &pb.GameResponse{
			Id:          int64(g.ID),
			GameCode:    g.GameCode,
			GameName:    g.GameName,
			GameType:    g.GameType,
			MinPlayers:  int32(g.MinPlayers),
			MaxPlayers:  int32(g.MaxPlayers),
			Status:      g.Status,
			Description: g.Description,
		})
	}
	return resp, nil
}

// GetRoom retrieves a room by room ID via gRPC.
func (s *GRPCServer) GetRoom(ctx context.Context, req *pb.GetRoomRequest) (*pb.RoomResponse, error) {
	var room GameRoom
	if err := s.svc.DB().Where("room_id = ?", req.RoomId).First(&room).Error; err != nil {
		return nil, err
	}
	return &pb.RoomResponse{
		Id:         int64(room.ID),
		RoomId:     room.RoomID,
		GameId:     int64(room.GameID),
		RoomName:   room.RoomName,
		OwnerId:    room.OwnerID,
		MaxPlayers: int32(room.MaxPlayers),
		Status:     room.Status,
	}, nil
}
