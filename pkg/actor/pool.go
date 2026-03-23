package actor

import (
	"context"
	"fmt"
	"sync"
)

// ActorPool manages a pool of reusable actors
type ActorPool struct {
	pool sync.Pool
}

// PoolConfig configures an actor pool
type PoolConfig struct {
	New   func() Actor
	Reset func(Actor)
}

// NewActorPool creates a new actor pool
func NewActorPool(config PoolConfig) *ActorPool {
	return &ActorPool{
		pool: sync.Pool{
			New: func() any {
				return config.New()
			},
		},
	}
}

// Get retrieves an actor from the pool
func (p *ActorPool) Get() Actor {
	return p.pool.Get().(Actor)
}

// Put returns an actor to the pool
func (p *ActorPool) Put(actor Actor) {
	p.pool.Put(actor)
}

// ==================== Specific Actor Pools ====================

var (
	// gameActorPool is the pool for GameActor instances
	gameActorPool = NewActorPool(PoolConfig{
		New: func() Actor {
			ctx, cancel := context.WithCancel(context.Background())
			return &GameActor{
				BaseActor: BaseActor{
					id:        "",
					actorType: "game",
					inbox:     make(chan Message, 100),
					ctx:       ctx,
					cancel:    cancel,
				},
				state:     make(map[string]any),
				players:   make(map[string]*PlayerActor),
				isRunning: false,
			}
		},
	})

	// playerActorPool is the pool for PlayerActor instances
	playerActorPool = NewActorPool(PoolConfig{
		New: func() Actor {
			ctx, cancel := context.WithCancel(context.Background())
			return &PlayerActor{
				BaseActor: BaseActor{
					id:        "",
					actorType: "player",
					inbox:     make(chan Message, 50),
					ctx:       ctx,
					cancel:    cancel,
				},
				state:   make(map[string]any),
				isReady: false,
			}
		},
	})

	// roomActorPool is the pool for RoomActor instances
	roomActorPool = NewActorPool(PoolConfig{
		New: func() Actor {
			ctx, cancel := context.WithCancel(context.Background())
			return &RoomActor{
				BaseActor: BaseActor{
					id:        "",
					actorType: "room",
					inbox:     make(chan Message, 50),
					ctx:       ctx,
					cancel:    cancel,
				},
				players:     make([]string, 0),
				maxPlayers:  4,
				isRunning:   false,
				gameStarted: false,
			}
		},
	})
)

// GetGameActor retrieves a GameActor from the pool
func GetGameActor(gameID, roomID string) *GameActor {
	actor := gameActorPool.Get().(*GameActor)
	actor.gameID = gameID
	actor.roomID = roomID
	actor.id = fmt.Sprintf("game:%s:%s", gameID, roomID)
	actor.Reset()
	return actor
}

// PutGameActor returns a GameActor to the pool
func PutGameActor(actor *GameActor) {
	actor.Reset()
	gameActorPool.Put(actor)
}

// GetPlayerActor retrieves a PlayerActor from the pool
func GetPlayerActor(playerID string) *PlayerActor {
	actor := playerActorPool.Get().(*PlayerActor)
	actor.playerID = playerID
	actor.id = fmt.Sprintf("player:%s", playerID)
	actor.Reset()
	return actor
}

// PutPlayerActor returns a PlayerActor to the pool
func PutPlayerActor(actor *PlayerActor) {
	actor.Reset()
	playerActorPool.Put(actor)
}

// GetRoomActor retrieves a RoomActor from the pool
func GetRoomActor(roomID string) *RoomActor {
	actor := roomActorPool.Get().(*RoomActor)
	actor.roomID = roomID
	actor.id = fmt.Sprintf("room:%s", roomID)
	actor.Reset()
	return actor
}

// PutRoomActor returns a RoomActor to the pool
func PutRoomActor(actor *RoomActor) {
	actor.Reset()
	roomActorPool.Put(actor)
}
