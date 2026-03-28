package game

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"online-game/pkg/actor"
	"online-game/pkg/apperror"
	"online-game/pkg/engine"
	"online-game/pkg/websocket"
)

// Service handles game business logic.
type Service struct {
	db          *gorm.DB
	actorSystem *actor.ActorSystem
	wsGateway   *websocket.Gateway
}

// NewService creates a new game service.
func NewService(db *gorm.DB, actorSystem *actor.ActorSystem, wsGateway *websocket.Gateway) *Service {
	return &Service{
		db:          db,
		actorSystem: actorSystem,
		wsGateway:   wsGateway,
	}
}

// --- Game Management ---

// ListGames returns published games for the lobby.
func (s *Service) ListGames(query *GameListQuery, page, pageSize int) ([]Game, int64, error) {
	var games []Game
	var total int64

	q := s.db.Model(&Game{})
	if query.Status != "" {
		q = q.Where("status = ?", query.Status)
	} else {
		q = q.Where("status = ?", "published") // default: only published
	}
	if query.GameType != "" {
		q = q.Where("game_type = ?", query.GameType)
	}
	if query.Search != "" {
		q = q.Where("game_name LIKE ?", "%"+query.Search+"%")
	}

	q.Count(&total)
	offset := (page - 1) * pageSize
	if err := q.Offset(offset).Limit(pageSize).Order("sort_order ASC, created_at DESC").Find(&games).Error; err != nil {
		return nil, 0, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return games, total, nil
}

// GetGame retrieves a game by ID.
func (s *Service) GetGame(id uint) (*Game, error) {
	var game Game
	if err := s.db.First(&game, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrGameNotFound
		}
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return &game, nil
}

// GetLatestVersion returns the latest active version of a game.
func (s *Service) GetLatestVersion(gameID uint) (*GameVersion, error) {
	var version GameVersion
	if err := s.db.Where("game_id = ? AND status = 'active'", gameID).
		Order("created_at DESC").First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrNotFound.WithMessage("没有可用版本")
		}
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return &version, nil
}

// --- Room Management ---

// CreateRoom creates a new game room and initializes a GameActor.
func (s *Service) CreateRoom(ctx context.Context, playerID, nickname string, req *CreateRoomRequest) (*GameRoom, error) {
	// Get game info
	game, err := s.GetGame(req.GameID)
	if err != nil {
		return nil, err
	}

	maxPlayers := req.MaxPlayers
	if maxPlayers <= 0 {
		maxPlayers = game.MaxPlayers
	}

	roomID := uuid.New().String()
	roomName := req.RoomName
	if roomName == "" {
		roomName = game.GameName + " - " + nickname + "的房间"
	}

	room := &GameRoom{
		RoomID:     roomID,
		GameID:     req.GameID,
		RoomName:   roomName,
		OwnerID:    playerID,
		MaxPlayers: maxPlayers,
		Status:     "waiting",
	}

	if err := s.db.Create(room).Error; err != nil {
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}

	// Create GameActor with goja engine
	if err := s.createGameActor(game, roomID, maxPlayers); err != nil {
		slog.Error("failed to create game actor", "room_id", roomID, "error", err)
		// Room is created but actor failed — room will be cleaned up
	}

	// Auto-join creator
	joinMsg := actor.NewMessage(actor.MsgPlayerJoin, playerID, &actor.PlayerJoinData{
		Nickname: nickname,
	})
	s.actorSystem.SendTo("game:"+fmt.Sprintf("%d", req.GameID)+":"+roomID, joinMsg)

	return room, nil
}

// JoinRoom adds a player to a room.
func (s *Service) JoinRoom(roomID, playerID, nickname string) error {
	var room GameRoom
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperror.ErrRoomNotFound
		}
		return apperror.ErrDatabaseError.WithData(err.Error())
	}

	if room.Status == "closed" {
		return apperror.ErrRoomNotJoinable
	}

	if room.Status == "playing" {
		return apperror.ErrGameRunning.WithMessage("游戏进行中，无法加入")
	}

	// Send join message to actor
	actorID := fmt.Sprintf("game:%d:%s", room.GameID, roomID)
	msg := actor.NewMessage(actor.MsgPlayerJoin, playerID, &actor.PlayerJoinData{
		Nickname: nickname,
	})
	if err := s.actorSystem.SendTo(actorID, msg); err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	// Record join
	playerRecord := &RoomPlayer{
		RoomID:   roomID,
		PlayerID: playerID,
		Nickname: nickname,
		Status:   "waiting",
		JoinedAt: time.Now(),
	}
	s.db.Create(playerRecord)

	return nil
}

// LeaveRoom removes a player from a room.
func (s *Service) LeaveRoom(roomID, playerID string) error {
	var room GameRoom
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return apperror.ErrRoomNotFound
	}

	actorID := fmt.Sprintf("game:%d:%s", room.GameID, roomID)
	msg := actor.NewMessage(actor.MsgPlayerLeave, playerID, &actor.PlayerLeaveData{
		Reason: "voluntary",
	})
	s.actorSystem.SendTo(actorID, msg)

	now := time.Now()
	s.db.Model(&RoomPlayer{}).
		Where("room_id = ? AND player_id = ? AND left_at IS NULL", roomID, playerID).
		Update("left_at", now)

	return nil
}

// StartGame starts a game in a room.
func (s *Service) StartGame(roomID string) error {
	var room GameRoom
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return apperror.ErrRoomNotFound
	}

	actorID := fmt.Sprintf("game:%d:%s", room.GameID, roomID)
	msg := actor.NewMessage(actor.MsgGameStart, "", nil)
	if err := s.actorSystem.SendTo(actorID, msg); err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	// Create game session
	session := &GameSession{
		RoomID:    roomID,
		GameID:    room.GameID,
		Status:    "running",
		StartTime: time.Now(),
	}
	s.db.Create(session)

	return nil
}

// HandlePlayerAction forwards a player action to the game actor.
func (s *Service) HandlePlayerAction(roomID, playerID, action string, data any) error {
	var room GameRoom
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return apperror.ErrRoomNotFound
	}

	actorID := fmt.Sprintf("game:%d:%s", room.GameID, roomID)
	msg := actor.ActionMessage(playerID, action, data)
	if err := s.actorSystem.SendTo(actorID, msg); err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// GetRooms lists rooms for a game.
func (s *Service) GetRooms(gameID uint, page, pageSize int) ([]GameRoom, int64, error) {
	var rooms []GameRoom
	var total int64

	q := s.db.Model(&GameRoom{}).Where("game_id = ? AND status != 'closed'", gameID)
	q.Count(&total)
	offset := (page - 1) * pageSize
	if err := q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&rooms).Error; err != nil {
		return nil, 0, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return rooms, total, nil
}

// CloseRoom closes a room and stops its actor.
func (s *Service) CloseRoom(roomID string) error {
	var room GameRoom
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return apperror.ErrRoomNotFound
	}

	actorID := fmt.Sprintf("game:%d:%s", room.GameID, roomID)
	shutdownMsg := actor.NewMessage(actor.MsgShutdown, "", nil)
	s.actorSystem.SendTo(actorID, shutdownMsg)

	now := time.Now()
	room.Status = "closed"
	room.ClosedAt = &now
	s.db.Save(&room)

	return nil
}

// Migrate runs auto migration for game tables.
func (s *Service) Migrate() error {
	return s.db.AutoMigrate(
		&Game{}, &GameVersion{}, &GameRoom{},
		&GameSession{}, &RoomPlayer{},
	)
}

// --- Internal ---

// createGameActor initializes a GameActor with a goja engine for a room.
func (s *Service) createGameActor(game *Game, roomID string, maxPlayers int) error {
	// Create goja engine with hub adapter
	hubAdapter := &wsHubAdapter{gateway: s.wsGateway}
	roomAdapter := &gormRoomAdapter{db: s.db, roomID: roomID}
	eng := engine.NewGojaEngine(hubAdapter, roomAdapter)

	// Initialize engine
	ctx := context.Background()
	config := &engine.EngineConfig{
		GameID:   fmt.Sprintf("%d", game.ID),
		RoomID:   roomID,
		GameCode: game.GameCode,
		Version:  "latest",
	}
	if err := eng.Init(ctx, config); err != nil {
		return fmt.Errorf("init engine: %w", err)
	}

	// Create and register actor
	cfg := actor.GameActorConfig{
		GameID:     fmt.Sprintf("%d", game.ID),
		RoomID:     roomID,
		GameCode:   game.GameCode,
		MaxPlayers: maxPlayers,
		InboxCap:   256,
		Hub:        hubAdapter,
		RoomMgr:    roomAdapter,
		Engine:     eng,
	}
	gameActor := actor.NewGameActor(cfg)
	return s.actorSystem.Register(gameActor)
}

// wsHubAdapter adapts websocket.Gateway to actor.HubInterface.
type wsHubAdapter struct {
	gateway *websocket.Gateway
}

func (a *wsHubAdapter) BroadcastToRoom(roomID string, event string, data any) {
	a.gateway.BroadcastToRoom(roomID, event, data)
}
func (a *wsHubAdapter) SendTo(playerID string, event string, data any) {
	a.gateway.SendTo(playerID, event, data)
}
func (a *wsHubAdapter) SendExcept(roomID string, exceptPlayerID string, event string, data any) {
	a.gateway.SendExcept(roomID, exceptPlayerID, event, data)
}

// gormRoomAdapter adapts gorm.DB to actor.RoomManagerInterface.
type gormRoomAdapter struct {
	db     *gorm.DB
	roomID string
}

func (a *gormRoomAdapter) GetPlayerIDs(roomID string) []string {
	var players []RoomPlayer
	a.db.Where("room_id = ? AND left_at IS NULL", roomID).Find(&players)
	ids := make([]string, len(players))
	for i, p := range players {
		ids[i] = p.PlayerID
	}
	return ids
}
func (a *gormRoomAdapter) GetConfig(roomID string) map[string]any {
	var room GameRoom
	if err := a.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return map[string]any{}
	}
	return map[string]any{
		"roomID":     room.RoomID,
		"maxPlayers": room.MaxPlayers,
		"gameID":     room.GameID,
	}
}
func (a *gormRoomAdapter) UpdateRoomStatus(roomID string, status string) {
	a.db.Model(&GameRoom{}).Where("room_id = ?", roomID).Update("status", status)
}
func (a *gormRoomAdapter) RecordGameResults(roomID string, results any) {
	var session GameSession
	if err := a.db.Where("room_id = ? AND status = 'running'", roomID).First(&session).Error; err != nil {
		return
	}
	now := time.Now()
	session.Status = "completed"
	session.EndTime = &now
	session.Duration = int(now.Sub(session.StartTime).Seconds())
	slog.Info("game session completed",
		"room_id", roomID,
		"duration", session.Duration,
	)
	a.db.Save(&session)
}
