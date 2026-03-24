package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"online-game/pkg/actor"
	"online-game/pkg/apperror"
	"online-game/pkg/engine"
)

var (
	ErrGameNotFound      = errors.New("game not found")
	ErrRoomNotFound      = errors.New("room not found")
	ErrGameAlreadyRunning = errors.New("game already running")
	ErrRoomFull          = errors.New("room is full")
	ErrPlayerNotInRoom   = errors.New("player not in room")
)

// GameRepository defines the interface for game data operations
type GameRepository interface {
	CreateGame(game *Game) error
	GetGameByID(id uint) (*Game, error)
	GetGameByCode(code string) (*Game, error)
	ListGames(orgID int64, offset, limit int) ([]*Game, int64, error)
	UpdateGame(game *Game) error
	DeleteGame(id uint) error

	CreateRoom(room *GameRoom) error
	GetRoomByID(id uint) (*GameRoom, error)
	GetRoomByRoomID(roomID string) (*GameRoom, error)
	ListRooms(gameID uint, status string, offset, limit int) ([]*GameRoom, int64, error)
	UpdateRoom(room *GameRoom) error
	DeleteRoom(id uint) error

	CreateSession(session *GameSession) error
	GetSessionByID(id uint) (*GameSession, error)
	UpdateSession(session *GameSession) error
	GetActiveSession(roomID string) (*GameSession, error)

	CreateGameVersion(version *GameVersion) error
	GetGameVersion(id uint) (*GameVersion, error)
	ListGameVersions(gameID uint) ([]*GameVersion, error)
	GetLatestVersion(gameID string) (*GameVersion, error)
	UpdateGameVersion(version *GameVersion) error
	DeleteGameVersion(id uint) error
}

// Service provides game business logic
type Service struct {
	repo   GameRepository
	mgr    *GameInstanceManager
}

// NewService creates a new game service
func NewService(repo GameRepository, mgr *GameInstanceManager) *Service {
	return &Service{
		repo: repo,
		mgr:  mgr,
	}
}

// Game operations

// CreateGame creates a new game
func (s *Service) CreateGame(ctx context.Context, req *CreateGameRequest) (*Game, error) {
	// Validate input
	if req.GameCode == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "game_code", "message": "不能为空"})
	}
	if req.GameName == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "game_name", "message": "不能为空"})
	}
	if req.GameType == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "game_type", "message": "不能为空"})
	}

	game := &Game{
		OrgID:    req.OrgID,
		GameCode: req.GameCode,
		GameName: req.GameName,
		GameType: req.GameType,
		Status:   1,
	}

	if err := s.repo.CreateGame(game); err != nil {
		return nil, err
	}

	return game, nil
}

// GetGame retrieves a game by ID
func (s *Service) GetGame(ctx context.Context, id uint) (*Game, error) {
	return s.repo.GetGameByID(id)
}

// ListGames lists games with pagination
func (s *Service) ListGames(ctx context.Context, orgID int64, page, pageSize int) ([]*Game, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.ListGames(orgID, offset, pageSize)
}

// UpdateGame updates a game
func (s *Service) UpdateGame(ctx context.Context, id uint, req *UpdateGameRequest) error {
	game, err := s.repo.GetGameByID(id)
	if err != nil {
		return err
	}

	if req.GameName != "" {
		game.GameName = req.GameName
	}
	if req.GameType != "" {
		game.GameType = req.GameType
	}
	if req.GameIcon != "" {
		game.GameIcon = req.GameIcon
	}
	if req.GameCover != "" {
		game.GameCover = req.GameCover
	}
	if req.Status != nil {
		if *req.Status < 0 || *req.Status > 2 {
			return apperror.ErrBadRequest.WithData(map[string]string{"field": "status", "message": "值必须为0-2"})
		}
		game.Status = *req.Status
	}

	return s.repo.UpdateGame(game)
}

// DeleteGame deletes a game
func (s *Service) DeleteGame(ctx context.Context, id uint) error {
	return s.repo.DeleteGame(id)
}

// UpdateGameConfig updates game configuration
func (s *Service) UpdateGameConfig(ctx context.Context, id uint, config map[string]interface{}) error {
	game, err := s.repo.GetGameByID(id)
	if err != nil {
		return err
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	game.GameConfig = string(configBytes)
	return s.repo.UpdateGame(game)
}

// GetGameConfig retrieves game configuration
func (s *Service) GetGameConfig(ctx context.Context, id uint) (map[string]interface{}, error) {
	game, err := s.repo.GetGameByID(id)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if game.GameConfig != "" {
		if err := json.Unmarshal([]byte(game.GameConfig), &config); err != nil {
			return nil, apperror.ErrInternalServer.WithData(err.Error())
		}
	}

	return config, nil
}

// Room operations

// CreateRoom creates a new game room
func (s *Service) CreateRoom(ctx context.Context, req *CreateRoomRequest) (*GameRoom, error) {
	// Validate input
	if req.RoomID == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "room_id", "message": "不能为空"})
	}
	if req.RoomName == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "room_name", "message": "不能为空"})
	}
	if req.MaxPlayers < 2 || req.MaxPlayers > 100 {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "max_players", "message": "值必须在2-100之间"})
	}

	room := &GameRoom{
		RoomID:     req.RoomID,
		GameID:     req.GameID,
		RoomName:   req.RoomName,
		MaxPlayers: req.MaxPlayers,
		Status:     "waiting",
	}

	if req.Config != nil {
		configBytes, _ := json.Marshal(req.Config)
		room.Config = string(configBytes)
	}

	if err := s.repo.CreateRoom(room); err != nil {
		return nil, err
	}

	// Create room actor
	roomActor := actor.NewRoomActor(req.RoomID, fmt.Sprintf("%d", req.GameID), req.MaxPlayers)
	s.mgr.GetActorSystem().Register(roomActor)
	roomActor.Start(ctx)

	return room, nil
}

// GetRoom retrieves a room by ID
func (s *Service) GetRoom(ctx context.Context, id uint) (*GameRoom, error) {
	return s.repo.GetRoomByID(id)
}

// GetRoomByRoomID retrieves a room by room ID
func (s *Service) GetRoomByRoomID(ctx context.Context, roomID string) (*GameRoom, error) {
	return s.repo.GetRoomByRoomID(roomID)
}

// ListRooms lists rooms with pagination
func (s *Service) ListRooms(ctx context.Context, gameID uint, status string, page, pageSize int) ([]*GameRoom, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.ListRooms(gameID, status, offset, pageSize)
}

// UpdateRoom updates a room
func (s *Service) UpdateRoom(ctx context.Context, id uint, updates map[string]interface{}) error {
	room, err := s.repo.GetRoomByID(id)
	if err != nil {
		return err
	}

	if status, ok := updates["status"].(string); ok {
		room.Status = status
	}
	if maxPlayers, ok := updates["max_players"].(int); ok {
		if maxPlayers < 2 || maxPlayers > 100 {
			return apperror.ErrBadRequest.WithData(map[string]string{"field": "max_players", "message": "值必须在2-100之间"})
		}
		room.MaxPlayers = maxPlayers
	}
	if config, ok := updates["config"].(map[string]interface{}); ok {
		configBytes, _ := json.Marshal(config)
		room.Config = string(configBytes)
	}

	return s.repo.UpdateRoom(room)
}

// CloseRoom closes a room
func (s *Service) CloseRoom(ctx context.Context, id uint) error {
	room, err := s.repo.GetRoomByID(id)
	if err != nil {
		return err
	}

	room.Status = "closed"
	now := time.Now()
	room.ClosedAt = &now

	if err := s.repo.UpdateRoom(room); err != nil {
		return err
	}

	// Unregister room actor
	actorID := fmt.Sprintf("room:%s", room.RoomID)
	_ = s.mgr.GetActorSystem().Unregister(actorID)

	return nil
}

// JoinRoom adds a player to a room
func (s *Service) JoinRoom(ctx context.Context, roomID, playerID string) error {
	if roomID == "" {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "room_id", "message": "不能为空"})
	}
	if playerID == "" {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "player_id", "message": "不能为空"})
	}

	room, err := s.repo.GetRoomByRoomID(roomID)
	if err != nil {
		return err
	}

	if room.PlayerCount >= room.MaxPlayers {
		return ErrRoomFull
	}

	room.PlayerCount++
	if err := s.repo.UpdateRoom(room); err != nil {
		return err
	}

	// Notify room actor
	actorID := fmt.Sprintf("room:%s", roomID)
	if a, err := s.mgr.GetActorSystem().Get(actorID); err == nil {
		_ = a.Send(&actor.PlayerJoinMessage{
			PlayerID: playerID,
			Timestamp: time.Now().Unix(),
		})
	}

	return nil
}

// LeaveRoom removes a player from a room
func (s *Service) LeaveRoom(ctx context.Context, roomID, playerID string) error {
	if roomID == "" {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "room_id", "message": "不能为空"})
	}
	if playerID == "" {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "player_id", "message": "不能为空"})
	}

	room, err := s.repo.GetRoomByRoomID(roomID)
	if err != nil {
		return err
	}

	if room.PlayerCount > 0 {
		room.PlayerCount--
		if err := s.repo.UpdateRoom(room); err != nil {
			return err
		}
	}

	// Notify room actor
	actorID := fmt.Sprintf("room:%s", roomID)
	if a, err := s.mgr.GetActorSystem().Get(actorID); err == nil {
		_ = a.Send(&actor.PlayerLeaveMessage{
			PlayerID: playerID,
			Timestamp: time.Now().Unix(),
		})
	}

	return nil
}

// Game execution operations

// StartGame starts a game instance
func (s *Service) StartGame(ctx context.Context, gameID uint, req *StartGameRequest) (*GameSession, error) {
	// Validate input
	if req.RoomID == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "room_id", "message": "不能为空"})
	}
	if len(req.Players) == 0 {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "players", "message": "至少需要一个玩家"})
	}

	game, err := s.repo.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}

	gameIDStr := fmt.Sprintf("%d", gameID)

	// Create game actor
	gameActor, err := s.mgr.CreateGameActor(gameIDStr, req.RoomID)
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	// Create and initialize engine
	eng, err := s.mgr.CreateGameEngine(gameIDStr)
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	// Initialize game with config
	var engineConfig map[string]interface{}
	if req.Config != nil {
		engineConfig = req.Config
	}
	_ = eng.Init(ctx, game.GameCode, engineConfig)

	// Create session
	session := &GameSession{
		RoomID:    req.RoomID,
		GameID:    gameID,
		StartTime: time.Now(),
		Status:    "running",
	}

	if err := s.repo.CreateSession(session); err != nil {
		return nil, err
	}

	// Send start message to game actor
	_ = gameActor.Send(&actor.GameStartMessage{
		GameID:    gameIDStr,
		RoomID:    req.RoomID,
		Players:   req.Players,
		Timestamp: time.Now().Unix(),
	})

	// Add players to the game
	for _, playerID := range req.Players {
		_ = gameActor.Send(&actor.PlayerJoinMessage{
			PlayerID: playerID,
			Timestamp: time.Now().Unix(),
		})
	}

	return session, nil
}

// StopGame stops a running game
func (s *Service) StopGame(ctx context.Context, gameID uint, roomID, reason string) error {
	gameIDStr := fmt.Sprintf("%d", gameID)

	// Stop game actor
	if err := s.mgr.StopGameActor(gameIDStr, roomID); err != nil {
		return apperror.ErrNotFound.WithMessage("游戏实例")
	}

	// Update session
	session, err := s.repo.GetActiveSession(roomID)
	if err == nil {
		now := time.Now()
		session.EndTime = &now
		session.Duration = int(now.Sub(session.StartTime).Seconds())
		session.Status = "completed"

		if reason != "" {
			session.FinalState = fmt.Sprintf(`{"reason": "%s"}`, reason)
		}

		_ = s.repo.UpdateSession(session)
	}

	return nil
}

// GetGameStatus retrieves the status of a running game
func (s *Service) GetGameStatus(ctx context.Context, gameID uint, roomID string) (*GameStatusInfo, error) {
	gameIDStr := fmt.Sprintf("%d", gameID)

	actor, err := s.mgr.GetGameActor(gameIDStr, roomID)
	if err != nil {
		return nil, apperror.ErrNotFound.WithMessage("游戏实例")
	}

	stats := actor.Stats()

	// Get engine info
	eng, _ := s.mgr.GetGameEngine(gameIDStr)

	return &GameStatusInfo{
		GameID:            gameIDStr,
		RoomID:            roomID,
		IsRunning:         actor.IsRunning(),
		PlayerCount:       actor.GetPlayerCount(),
		MessagesProcessed: stats.MessageCount,
		CurrentEngine:     eng.CurrentEngine(),
		GameState:         actor.GetState(),
	}, nil
}

// PlayerAction sends a player action to the game
func (s *Service) PlayerAction(ctx context.Context, gameID uint, roomID, playerID string, action string, data map[string]interface{}) error {
	if roomID == "" {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "room_id", "message": "不能为空"})
	}
	if playerID == "" {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "player_id", "message": "不能为空"})
	}
	if action == "" {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "action", "message": "不能为空"})
	}

	gameIDStr := fmt.Sprintf("%d", gameID)

	actor, err := s.mgr.GetGameActor(gameIDStr, roomID)
	if err != nil {
		return apperror.ErrNotFound.WithMessage("游戏实例")
	}

	return actor.Send(&actor.PlayerActionMessage{
		PlayerID:  playerID,
		Action:    action,
		Data:      data,
		Timestamp: time.Now().Unix(),
	})
}

// GetSession retrieves a session by ID
func (s *Service) GetSession(ctx context.Context, id uint) (*GameSession, error) {
	return s.repo.GetSessionByID(id)
}

// GetSessionState retrieves the state of a session
func (s *Service) GetSessionState(ctx context.Context, id uint) (map[string]interface{}, error) {
	session, err := s.repo.GetSessionByID(id)
	if err != nil {
		return nil, err
	}

	var state map[string]interface{}
	if session.FinalState != "" {
		if err := json.Unmarshal([]byte(session.FinalState), &state); err != nil {
			return nil, apperror.ErrInternalServer.WithData(err.Error())
		}
	}

	return state, nil
}

// Game version operations

// CreateGameVersion creates a new game version
func (s *Service) CreateGameVersion(ctx context.Context, req *CreateGameVersionRequest) (*GameVersion, error) {
	// Validate input
	if req.Version == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "version", "message": "不能为空"})
	}
	if req.ScriptType != "js" && req.ScriptType != "wasm" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "script_type", "message": "必须是js或wasm"})
	}
	if req.ScriptPath == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "script_path", "message": "不能为空"})
	}
	if req.ScriptHash == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "script_hash", "message": "不能为空"})
	}

	version := &GameVersion{
		GameID:      req.GameID,
		Version:     req.Version,
		ScriptType:  req.ScriptType,
		ScriptPath:  req.ScriptPath,
		ScriptHash:  req.ScriptHash,
		Description: req.Description,
		Status:      1, // Active
	}

	if err := s.repo.CreateGameVersion(version); err != nil {
		return nil, err
	}

	return version, nil
}

// GetGameVersion retrieves a game version by ID
func (s *Service) GetGameVersion(ctx context.Context, id uint) (*GameVersion, error) {
	return s.repo.GetGameVersion(id)
}

// ListGameVersions lists all versions for a game
func (s *Service) ListGameVersions(ctx context.Context, gameID uint) ([]*GameVersion, error) {
	return s.repo.ListGameVersions(gameID)
}

// GetLatestVersion retrieves the latest version for a game
func (s *Service) GetLatestVersion(ctx context.Context, gameCode string) (*GameVersion, error) {
	return s.repo.GetLatestVersion(gameCode)
}

// UpdateGameVersion updates a game version
func (s *Service) UpdateGameVersion(ctx context.Context, id uint, req *UpdateGameVersionRequest) error {
	version, err := s.repo.GetGameVersion(id)
	if err != nil {
		return err
	}

	if req.Status != nil {
		if *req.Status < 0 || *req.Status > 2 {
			return apperror.ErrBadRequest.WithData(map[string]string{"field": "status", "message": "值必须为0-2"})
		}
		version.Status = *req.Status
	}
	if req.Description != "" {
		version.Description = req.Description
	}

	return s.repo.UpdateGameVersion(version)
}

// DeleteGameVersion deletes a game version
func (s *Service) DeleteGameVersion(ctx context.Context, id uint) error {
	return s.repo.DeleteGameVersion(id)
}

// Request types

// CreateGameRequest represents the request to create a game
type CreateGameRequest struct {
	OrgID       int64  `json:"org_id" binding:"required"`
	GameCode    string `json:"game_code" binding:"required,min=1,max=50"`
	GameName    string `json:"game_name" binding:"required,min=1,max=100"`
	GameType    string `json:"game_type" binding:"required,min=1,max=20"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateGameRequest represents the request to update a game
type UpdateGameRequest struct {
	GameName  string `json:"game_name" binding:"omitempty,min=1,max=100"`
	GameType  string `json:"game_type" binding:"omitempty,min=1,max=20"`
	GameIcon  string `json:"game_icon" binding:"omitempty,max=255"`
	GameCover string `json:"game_cover" binding:"omitempty,max=255"`
	Status    *int   `json:"status" binding:"omitempty,min=0,max=2"`
}

// CreateRoomRequest represents the request to create a room
type CreateRoomRequest struct {
	RoomID     string                 `json:"room_id" binding:"required,min=1,max=50"`
	GameID     uint                   `json:"game_id" binding:"required"`
	RoomName   string                 `json:"room_name" binding:"required,min=1,max=100"`
	MaxPlayers int                    `json:"max_players" binding:"min=2,max=100"`
	Config     map[string]interface{} `json:"config"`
}

// StartGameRequest represents the request to start a game
type StartGameRequest struct {
	RoomID  string                 `json:"room_id" binding:"required"`
	Players []string               `json:"players" binding:"required,min=1"`
	Config  map[string]interface{} `json:"config"`
}

// CreateGameVersionRequest represents the request to create a game version
type CreateGameVersionRequest struct {
	GameID      uint   `json:"game_id" binding:"required"`
	Version     string `json:"version" binding:"required,min=1,max=20"`
	ScriptType  string `json:"script_type" binding:"required,oneof=js wasm"`
	ScriptPath  string `json:"script_path" binding:"required,max=255"`
	ScriptHash  string `json:"script_hash" binding:"required,max=64"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateGameVersionRequest represents the request to update a game version
type UpdateGameVersionRequest struct {
	Status      *int    `json:"status" binding:"omitempty,min=0,max=2"`
	Description string  `json:"description" binding:"omitempty,max=500"`
}

// GameStatusInfo represents the status of a running game
type GameStatusInfo struct {
	GameID            string                 `json:"game_id"`
	RoomID            string                 `json:"room_id"`
	IsRunning         bool                   `json:"is_running"`
	PlayerCount       int                    `json:"player_count"`
	MessagesProcessed int64                  `json:"messages_processed"`
	CurrentEngine     string                 `json:"current_engine"`
	GameState         map[string]interface{} `json:"game_state"`
}

// RepositoryImpl implements GameRepository
type RepositoryImpl struct {
	db *gorm.DB
}

// NewRepositoryImpl creates a new repository implementation
func NewRepositoryImpl(db *gorm.DB) GameRepository {
	return &RepositoryImpl{db: db}
}

// CreateGame creates a new game
func (r *RepositoryImpl) CreateGame(game *Game) error {
	err := r.db.Create(game).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// GetGameByID retrieves a game by ID
func (r *RepositoryImpl) GetGameByID(id uint) (*Game, error) {
	var game Game
	err := r.db.First(&game, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrGameNotFound
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &game, nil
}

// GetGameByCode retrieves a game by code
func (r *RepositoryImpl) GetGameByCode(code string) (*Game, error) {
	var game Game
	err := r.db.Where("game_code = ?", code).First(&game).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrGameNotFound
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &game, nil
}

// ListGames lists games with pagination
func (r *RepositoryImpl) ListGames(orgID int64, offset, limit int) ([]*Game, int64, error) {
	var games []*Game
	var total int64

	query := r.db.Model(&Game{})
	if orgID > 0 {
		query = query.Where("org_id = ?", orgID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}

	err := query.Offset(offset).Limit(limit).Find(&games).Error
	if err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}
	return games, total, nil
}

// UpdateGame updates a game
func (r *RepositoryImpl) UpdateGame(game *Game) error {
	err := r.db.Save(game).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// DeleteGame deletes a game
func (r *RepositoryImpl) DeleteGame(id uint) error {
	err := r.db.Delete(&Game{}, id).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// CreateRoom creates a new game room
func (r *RepositoryImpl) CreateRoom(room *GameRoom) error {
	err := r.db.Create(room).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// GetRoomByID retrieves a room by ID
func (r *RepositoryImpl) GetRoomByID(id uint) (*GameRoom, error) {
	var room GameRoom
	err := r.db.First(&room, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrPlayerNotFound.WithMessage("房间不存在")
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &room, nil
}

// GetRoomByRoomID retrieves a room by room ID
func (r *RepositoryImpl) GetRoomByRoomID(roomID string) (*GameRoom, error) {
	var room GameRoom
	err := r.db.Where("room_id = ?", roomID).First(&room).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrPlayerNotFound.WithMessage("房间不存在")
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &room, nil
}

// ListRooms lists rooms with pagination
func (r *RepositoryImpl) ListRooms(gameID uint, status string, offset, limit int) ([]*GameRoom, int64, error) {
	var rooms []*GameRoom
	var total int64

	query := r.db.Model(&GameRoom{})
	if gameID > 0 {
		query = query.Where("game_id = ?", gameID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}

	err := query.Offset(offset).Limit(limit).Find(&rooms).Error
	if err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}
	return rooms, total, nil
}

// UpdateRoom updates a room
func (r *RepositoryImpl) UpdateRoom(room *GameRoom) error {
	err := r.db.Save(room).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// DeleteRoom deletes a room
func (r *RepositoryImpl) DeleteRoom(id uint) error {
	err := r.db.Delete(&GameRoom{}, id).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// CreateSession creates a new game session
func (r *RepositoryImpl) CreateSession(session *GameSession) error {
	err := r.db.Create(session).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// GetSessionByID retrieves a session by ID
func (r *RepositoryImpl) GetSessionByID(id uint) (*GameSession, error) {
	var session GameSession
	err := r.db.First(&session, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrNotFound.WithMessage("游戏会话")
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &session, nil
}

// UpdateSession updates a session
func (r *RepositoryImpl) UpdateSession(session *GameSession) error {
	err := r.db.Save(session).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// GetActiveSession retrieves the active session for a room
func (r *RepositoryImpl) GetActiveSession(roomID string) (*GameSession, error) {
	var session GameSession
	err := r.db.Where("room_id = ? AND status = ?", roomID, "running").First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrNotFound.WithMessage("游戏会话")
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &session, nil
}

// Game version repository methods

// CreateGameVersion creates a new game version
func (r *RepositoryImpl) CreateGameVersion(version *GameVersion) error {
	err := r.db.Create(version).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// GetGameVersion retrieves a game version by ID
func (r *RepositoryImpl) GetGameVersion(id uint) (*GameVersion, error) {
	var version GameVersion
	err := r.db.First(&version, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrNotFound.WithMessage("游戏版本")
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &version, nil
}

// ListGameVersions lists all versions for a game
func (r *RepositoryImpl) ListGameVersions(gameID uint) ([]*GameVersion, error) {
	var versions []*GameVersion
	err := r.db.Where("game_id = ?", gameID).Order("created_at DESC").Find(&versions).Error
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return versions, nil
}

// GetLatestVersion retrieves the latest version for a game
func (r *RepositoryImpl) GetLatestVersion(gameID string) (*GameVersion, error) {
	var version GameVersion
	err := r.db.Where("game_code = ?", gameID).Order("created_at DESC").First(&version).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrNotFound.WithMessage("游戏版本")
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &version, nil
}

// UpdateGameVersion updates a game version
func (r *RepositoryImpl) UpdateGameVersion(version *GameVersion) error {
	err := r.db.Save(version).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// DeleteGameVersion deletes a game version
func (r *RepositoryImpl) DeleteGameVersion(id uint) error {
	err := r.db.Delete(&GameVersion{}, id).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}
