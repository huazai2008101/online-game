package game

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"online-game/pkg/actor"
	"online-game/pkg/engine"
)

// GameInstanceManager manages running game instances
type GameInstanceManager struct {
	actorSystem  *actor.ActorSystem
	engines      map[string]*engine.DualEngine // gameID -> engine
	repositories map[string]interface{}
}

// NewGameInstanceManager creates a new game instance manager
func NewGameInstanceManager(db *gorm.DB) *GameInstanceManager {
	return &GameInstanceManager{
		actorSystem:  actor.NewActorSystem(),
		engines:      make(map[string]*engine.DualEngine),
		repositories: make(map[string]interface{}),
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
