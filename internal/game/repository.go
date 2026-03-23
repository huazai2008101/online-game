package game

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"online-game/pkg/actor"
	"online-game/pkg/engine"
)

// Repository provides database operations
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateGame creates a new game
func (r *Repository) CreateGame(game *Game) error {
	return r.db.Create(game).Error
}

// GetGameByID retrieves a game by ID
func (r *Repository) GetGameByID(id uint) (*Game, error) {
	var game Game
	err := r.db.First(&game, id).Error
	if err != nil {
		return nil, err
	}
	return &game, nil
}

// GetGameByCode retrieves a game by code
func (r *Repository) GetGameByCode(code string) (*Game, error) {
	var game Game
	err := r.db.Where("game_code = ?", code).First(&game).Error
	if err != nil {
		return nil, err
	}
	return &game, nil
}

// ListGames lists games with pagination
func (r *Repository) ListGames(orgID int64, offset, limit int) ([]*Game, int64, error) {
	var games []*Game
	var total int64

	query := r.db.Model(&Game{})
	if orgID > 0 {
		query = query.Where("org_id = ?", orgID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(offset).Limit(limit).Find(&games).Error
	return games, total, err
}

// UpdateGame updates a game
func (r *Repository) UpdateGame(game *Game) error {
	return r.db.Save(game).Error
}

// DeleteGame deletes a game
func (r *Repository) DeleteGame(id uint) error {
	return r.db.Delete(&Game{}, id).Error
}

// CreateRoom creates a new game room
func (r *Repository) CreateRoom(room *GameRoom) error {
	return r.db.Create(room).Error
}

// GetRoomByID retrieves a room by ID
func (r *Repository) GetRoomByID(id uint) (*GameRoom, error) {
	var room GameRoom
	err := r.db.First(&room, id).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// GetRoomByRoomID retrieves a room by room ID
func (r *Repository) GetRoomByRoomID(roomID string) (*GameRoom, error) {
	var room GameRoom
	err := r.db.Where("room_id = ?", roomID).First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// ListRooms lists rooms with pagination
func (r *Repository) ListRooms(gameID uint, status string, offset, limit int) ([]*GameRoom, int64, error) {
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
		return nil, 0, err
	}

	err := query.Offset(offset).Limit(limit).Find(&rooms).Error
	return rooms, total, err
}

// UpdateRoom updates a room
func (r *Repository) UpdateRoom(room *GameRoom) error {
	return r.db.Save(room).Error
}

// CreateSession creates a new game session
func (r *Repository) CreateSession(session *GameSession) error {
	return r.db.Create(session).Error
}

// GetSessionByID retrieves a session by ID
func (r *Repository) GetSessionByID(id uint) (*GameSession, error) {
	var session GameSession
	err := r.db.First(&session, id).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateSession updates a session
func (r *Repository) UpdateSession(session *GameSession) error {
	return r.db.Save(session).Error
}

// GameInstanceManager manages running game instances
type GameInstanceManager struct {
	actorSystem  *actor.ActorSystem
	engines      map[string]*engine.DualEngine // gameID -> engine
	repositories map[string]*Repository
}

// NewGameInstanceManager creates a new game instance manager
func NewGameInstanceManager(db *gorm.DB) *GameInstanceManager {
	return &GameInstanceManager{
		actorSystem:  actor.NewActorSystem(),
		engines:      make(map[string]*engine.DualEngine),
		repositories: make(map[string]*Repository),
	}
}

// GetActorSystem returns the actor system
func (m *GameInstanceManager) GetActorSystem() *actor.ActorSystem {
	return m.actorSystem
}

// CreateGameEngine creates a new dual engine for a game
func (m *GameInstanceManager) CreateGameEngine(gameID string) (*engine.DualEngine, error) {
	if eng, exists := m.engines[gameID]; exists {
		return eng, nil
	}

	eng, err := engine.NewDualEngine(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to create dual engine: %w", err)
	}

	ctx := context.Background()
	if err := eng.Init(ctx, gameID, nil); err != nil {
		return nil, fmt.Errorf("failed to initialize engine: %w", err)
	}

	m.engines[gameID] = eng
	return eng, nil
}

// GetGameEngine retrieves or creates a game engine
func (m *GameInstanceManager) GetGameEngine(gameID string) (*engine.DualEngine, error) {
	if eng, exists := m.engines[gameID]; exists {
		return eng, nil
	}
	return m.CreateGameEngine(gameID)
}

// RemoveGameEngine removes a game engine
func (m *GameInstanceManager) RemoveGameEngine(gameID string) {
	if eng, exists := m.engines[gameID]; exists {
		_ = eng.Cleanup()
		delete(m.engines, gameID)
	}
}

// CreateGameActor creates a new game actor
func (m *GameInstanceManager) CreateGameActor(gameID, roomID string) (*actor.GameActor, error) {
	gameActor := actor.NewGameActor(gameID, roomID)

	if err := m.actorSystem.Register(gameActor); err != nil {
		return nil, fmt.Errorf("failed to register game actor: %w", err)
	}

	if err := gameActor.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start game actor: %w", err)
	}

	return gameActor, nil
}

// GetGameActor retrieves a game actor by ID
func (m *GameInstanceManager) GetGameActor(gameID, roomID string) (*actor.GameActor, error) {
	actorID := fmt.Sprintf("game:%s:%s", gameID, roomID)
	a, err := m.actorSystem.Get(actorID)
	if err != nil {
		return nil, err
	}

	if gameActor, ok := a.(*actor.GameActor); ok {
		return gameActor, nil
	}

	return nil, fmt.Errorf("actor is not a GameActor")
}

// StopGameActor stops a game actor
func (m *GameInstanceManager) StopGameActor(gameID, roomID string) error {
	actor, err := m.GetGameActor(gameID, roomID)
	if err != nil {
		return err
	}

	if err := actor.Stop(); err != nil {
		return err
	}

	return m.actorSystem.Unregister(actor.ID())
}

// Shutdown shuts down the game instance manager
func (m *GameInstanceManager) Shutdown() error {
	// Stop all actors
	_ = m.actorSystem.Stop()

	// Cleanup all engines
	for gameID := range m.engines {
		m.RemoveGameEngine(gameID)
	}

	return nil
}
