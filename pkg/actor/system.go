package actor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ErrActorNotFound is returned when an actor lookup fails.
var ErrActorNotFound = fmt.Errorf("actor not found")

// ErrActorExists is returned when registering a duplicate actor.
var ErrActorExists = fmt.Errorf("actor already exists")

// ActorSystem is the registry and message router for all actors.
// Uses sync.Map for concurrent-safe actor lookup without global locks.
type ActorSystem struct {
	actors sync.Map // actorID -> Actor
	count  atomic.Int64
}

// NewActorSystem creates a new actor system.
func NewActorSystem() *ActorSystem {
	return &ActorSystem{}
}

// Register adds an actor to the system and starts it.
func (s *ActorSystem) Register(actor Actor) error {
	id := actor.ID()
	if _, loaded := s.actors.LoadOrStore(id, actor); loaded {
		return fmt.Errorf("%w: %s", ErrActorExists, id)
	}
	s.count.Add(1)

	// Start the actor if it's a BaseActor
	if ba, ok := actor.(*BaseActor); ok {
		ba.Start()
	}

	slog.Info("actor registered", "actor_id", id)
	return nil
}

// Unregister removes and stops an actor.
func (s *ActorSystem) Unregister(actorID string) error {
	val, loaded := s.actors.LoadAndDelete(actorID)
	if !loaded {
		return fmt.Errorf("%w: %s", ErrActorNotFound, actorID)
	}
	s.count.Add(-1)

	actor := val.(Actor)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := actor.Stop(ctx); err != nil {
		slog.Warn("actor stop error", "actor_id", actorID, "error", err)
	}

	slog.Info("actor unregistered", "actor_id", actorID)
	return nil
}

// Get retrieves an actor by ID.
func (s *ActorSystem) Get(actorID string) (Actor, bool) {
	val, ok := s.actors.Load(actorID)
	if !ok {
		return nil, false
	}
	return val.(Actor), true
}

// SendTo sends a message to the actor with the given ID.
func (s *ActorSystem) SendTo(actorID string, msg *Message) error {
	actor, ok := s.Get(actorID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrActorNotFound, actorID)
	}
	return actor.Send(msg)
}

// Count returns the number of active actors.
func (s *ActorSystem) Count() int64 {
	return s.count.Load()
}

// Shutdown gracefully stops all actors in parallel.
func (s *ActorSystem) Shutdown(timeout time.Duration) {
	slog.Info("actor system shutting down", "actors", s.count.Load())

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var wg sync.WaitGroup
	s.actors.Range(func(key, value any) bool {
		wg.Add(1)
		go func(id string, actor Actor) {
			defer wg.Done()
			if err := actor.Stop(ctx); err != nil {
				slog.Warn("actor shutdown error", "actor_id", id, "error", err)
			}
		}(key.(string), value.(Actor))
		return true
	})
	wg.Wait()

	s.actors = sync.Map{}
	s.count.Store(0)
	slog.Info("actor system shutdown complete")
}
