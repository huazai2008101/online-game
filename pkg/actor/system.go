package actor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// ActorSystem manages the lifecycle of actors
type ActorSystem struct {
	actors    sync.Map // map[string]Actor
	actorType sync.Map // map[string][]string - actors by type
	count     atomic.Int64
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewActorSystem creates a new actor system
func NewActorSystem() *ActorSystem {
	ctx, cancel := context.WithCancel(context.Background())
	return &ActorSystem{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the actor system
func (s *ActorSystem) Start(ctx context.Context) error {
	return nil
}

// Stop stops all actors in the system
func (s *ActorSystem) Stop() error {
	s.cancel()

	// Stop all actors
	s.actors.Range(func(key, value any) bool {
		actor := value.(Actor)
		_ = actor.Stop()
		return true
	})

	s.wg.Wait()
	return nil
}

// Register registers an actor with the system
func (s *ActorSystem) Register(actor Actor) error {
	actorID := actor.ID()
	actorType := actor.Type()

	if _, exists := s.actors.Load(actorID); exists {
		return fmt.Errorf("actor %s already registered", actorID)
	}

	s.actors.Store(actorID, actor)

	// Add to type index
	var actorsOfType []string
	if v, ok := s.actorType.Load(actorType); ok {
		actorsOfType = v.([]string)
	}
	actorsOfType = append(actorsOfType, actorID)
	s.actorType.Store(actorType, actorsOfType)

	s.count.Add(1)
	return nil
}

// Unregister removes an actor from the system
func (s *ActorSystem) Unregister(actorID string) error {
	actor, err := s.Get(actorID)
	if err != nil {
		return err
	}

	actorType := actor.Type()

	// Remove from main registry
	s.actors.Delete(actorID)

	// Remove from type index
	if v, ok := s.actorType.Load(actorType); ok {
		actorsOfType := v.([]string)
		newList := make([]string, 0, len(actorsOfType))
		for _, id := range actorsOfType {
			if id != actorID {
				newList = append(newList, id)
			}
		}
		if len(newList) > 0 {
			s.actorType.Store(actorType, newList)
		} else {
			s.actorType.Delete(actorType)
		}
	}

	s.count.Add(-1)
	return nil
}

// Get retrieves an actor by ID
func (s *ActorSystem) Get(actorID string) (Actor, error) {
	v, ok := s.actors.Load(actorID)
	if !ok {
		return nil, fmt.Errorf("actor %s not found", actorID)
	}
	return v.(Actor), nil
}

// Send sends a message to an actor
func (s *ActorSystem) Send(actorID string, msg Message) error {
	actor, err := s.Get(actorID)
	if err != nil {
		return err
	}
	return actor.Send(msg)
}

// Broadcast sends a message to all actors of a specific type
func (s *ActorSystem) Broadcast(actorType string, msg Message) error {
	v, ok := s.actorType.Load(actorType)
	if !ok {
		return fmt.Errorf("no actors of type %s found", actorType)
	}

	actorsOfType := v.([]string)
	var lastErr error
	for _, actorID := range actorsOfType {
		if err := s.Send(actorID, msg); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// GetByType retrieves all actors of a specific type
func (s *ActorSystem) GetByType(actorType string) []Actor {
	v, ok := s.actorType.Load(actorType)
	if !ok {
		return nil
	}

	actorIDs := v.([]string)
	actors := make([]Actor, 0, len(actorIDs))
	for _, id := range actorIDs {
		if actor, err := s.Get(id); err == nil {
			actors = append(actors, actor)
		}
	}
	return actors
}

// Count returns the total number of actors in the system
func (s *ActorSystem) Count() int64 {
	return s.count.Load()
}

// CountByType returns the number of actors of a specific type
func (s *ActorSystem) CountByType(actorType string) int {
	v, ok := s.actorType.Load(actorType)
	if !ok {
		return 0
	}
	return len(v.([]string))
}

// List returns all actor IDs
func (s *ActorSystem) List() []string {
	ids := make([]string, 0)
	s.actors.Range(func(key, value any) bool {
		ids = append(ids, key.(string))
		return true
	})
	return ids
}
