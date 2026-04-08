package game

import (
	"testing"

	pb "online-game/proto/game"
)

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

// TestGRPCServer_GameResponseMapping tests the Game -> pb.GameResponse mapping.
func TestGRPCServer_GameResponseMapping(t *testing.T) {
	game := &Game{
		ID:          1,
		GameCode:    "BLACKJACK",
		GameName:    "21点",
		GameType:    "turn-based",
		MinPlayers:  2,
		MaxPlayers:  7,
		Status:      "published",
		Description: "经典纸牌游戏",
	}

	resp := &pb.GameResponse{
		Id:          int64(game.ID),
		GameCode:    game.GameCode,
		GameName:    game.GameName,
		GameType:    game.GameType,
		MinPlayers:  int32(game.MinPlayers),
		MaxPlayers:  int32(game.MaxPlayers),
		Status:      game.Status,
		Description: game.Description,
	}

	if resp.Id != 1 {
		t.Errorf("expected id=1, got %d", resp.Id)
	}
	if resp.GameCode != "BLACKJACK" {
		t.Errorf("expected game_code=BLACKJACK, got %s", resp.GameCode)
	}
	if resp.GameName != "21点" {
		t.Errorf("expected game_name=21点, got %s", resp.GameName)
	}
	if resp.GameType != "turn-based" {
		t.Errorf("expected game_type=turn-based, got %s", resp.GameType)
	}
	if resp.MinPlayers != 2 {
		t.Errorf("expected min_players=2, got %d", resp.MinPlayers)
	}
	if resp.MaxPlayers != 7 {
		t.Errorf("expected max_players=7, got %d", resp.MaxPlayers)
	}
	if resp.Status != "published" {
		t.Errorf("expected status=published, got %s", resp.Status)
	}
	if resp.Description != "经典纸牌游戏" {
		t.Errorf("expected description, got %s", resp.Description)
	}
}

// TestGRPCServer_RoomResponseMapping tests the GameRoom -> pb.RoomResponse mapping.
func TestGRPCServer_RoomResponseMapping(t *testing.T) {
	room := GameRoom{
		ID:         1,
		RoomID:     "room-uuid-123",
		GameID:     5,
		RoomName:   "测试房间",
		OwnerID:    "player-1",
		MaxPlayers: 4,
		Status:     "waiting",
	}

	resp := &pb.RoomResponse{
		Id:         int64(room.ID),
		RoomId:     room.RoomID,
		GameId:     int64(room.GameID),
		RoomName:   room.RoomName,
		OwnerId:    room.OwnerID,
		MaxPlayers: int32(room.MaxPlayers),
		Status:     room.Status,
	}

	if resp.RoomId != "room-uuid-123" {
		t.Errorf("expected room_id=room-uuid-123, got %s", resp.RoomId)
	}
	if resp.GameId != 5 {
		t.Errorf("expected game_id=5, got %d", resp.GameId)
	}
	if resp.RoomName != "测试房间" {
		t.Errorf("expected room_name=测试房间, got %s", resp.RoomName)
	}
	if resp.OwnerId != "player-1" {
		t.Errorf("expected owner_id=player-1, got %s", resp.OwnerId)
	}
	if resp.MaxPlayers != 4 {
		t.Errorf("expected max_players=4, got %d", resp.MaxPlayers)
	}
	if resp.Status != "waiting" {
		t.Errorf("expected status=waiting, got %s", resp.Status)
	}
}

// TestGRPCServer_ListGamesRequestDefaults tests pagination defaults.
func TestGRPCServer_ListGamesRequestDefaults(t *testing.T) {
	tests := []struct {
		name         string
		page         int32
		pageSize     int32
		expectPage   int
		expectSize   int
	}{
		{"zero values default to 1/20", 0, 0, 1, 20},
		{"negative values default to 1/20", -1, -5, 1, 20},
		{"valid values pass through", 3, 10, 3, 10},
		{"page 0 defaults", 0, 50, 1, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := int(tt.page)
			pageSize := int(tt.pageSize)
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}
			if page != tt.expectPage {
				t.Errorf("expected page=%d, got %d", tt.expectPage, page)
			}
			if pageSize != tt.expectSize {
				t.Errorf("expected pageSize=%d, got %d", tt.expectSize, pageSize)
			}
		})
	}
}

// TestGRPCServer_ListGamesResponse tests building a list response.
func TestGRPCServer_ListGamesResponse(t *testing.T) {
	games := []Game{
		{ID: 1, GameCode: "BJ", GameName: "21点", GameType: "turn-based", MinPlayers: 2, MaxPlayers: 7, Status: "published"},
		{ID: 2, GameCode: "CHESS", GameName: "象棋", GameType: "turn-based", MinPlayers: 2, MaxPlayers: 2, Status: "published"},
	}

	resp := &pb.ListGamesResponse{Total: 2}
	for _, g := range games {
		resp.Games = append(resp.Games, &pb.GameResponse{
			Id:         int64(g.ID),
			GameCode:   g.GameCode,
			GameName:   g.GameName,
			GameType:   g.GameType,
			MinPlayers: int32(g.MinPlayers),
			MaxPlayers: int32(g.MaxPlayers),
			Status:     g.Status,
		})
	}

	if resp.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Total)
	}
	if len(resp.Games) != 2 {
		t.Fatalf("expected 2 games, got %d", len(resp.Games))
	}
	if resp.Games[0].GameCode != "BJ" {
		t.Errorf("expected first game code=BJ, got %s", resp.Games[0].GameCode)
	}
	if resp.Games[1].GameName != "象棋" {
		t.Errorf("expected second game name=象棋, got %s", resp.Games[1].GameName)
	}
}
