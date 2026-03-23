// Package actor implements the Actor model for concurrent message processing
// The Actor model provides:
// - Lock-free concurrency through message passing
// - High performance and low latency
// - Easy to test and scale
package actor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Message is the interface for all actor messages
type Message interface{}

// Actor is the interface for all actors
type Actor interface {
	// Receive processes a message
	Receive(ctx context.Context, msg Message) error

	// Send sends a message to the actor
	Send(msg Message) error

	// Start starts the actor's message processing loop
	Start(ctx context.Context) error

	// Stop gracefully stops the actor
	Stop() error

	// ID returns the actor's unique identifier
	ID() string

	// Type returns the actor's type
	Type() string

	// Stats returns the actor's statistics
	Stats() *ActorStats
}

// ActorStats holds statistics about an actor
type ActorStats struct {
	MessageCount  int64
	ProcessTime   time.Duration
	ErrorCount    int64
	LastErrorTime time.Time
	mu            sync.RWMutex
}

// BaseActor provides a base implementation of the Actor interface
type BaseActor struct {
	id        string
	actorType string
	inbox     chan Message
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	stats     ActorStats
	handler   MessageHandler
}

// MessageHandler handles incoming messages
type MessageHandler func(ctx context.Context, msg Message) error

// NewActor creates a new BaseActor
func NewActor(id, actorType string, inboxSize int, handler MessageHandler) *BaseActor {
	ctx, cancel := context.WithCancel(context.Background())
	return &BaseActor{
		id:        id,
		actorType: actorType,
		inbox:     make(chan Message, inboxSize),
		ctx:       ctx,
		cancel:    cancel,
		handler:   handler,
	}
}

// Start starts the actor's message processing loop
func (a *BaseActor) Start(ctx context.Context) error {
	a.wg.Add(1)
	go a.run(ctx)
	return nil
}

// run is the main message processing loop
func (a *BaseActor) run(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case msg, ok := <-a.inbox:
			if !ok {
				return
			}
			start := time.Now()
			err := a.Receive(ctx, msg)
			duration := time.Since(start)

			a.stats.mu.Lock()
			a.stats.ProcessTime += duration
			a.stats.MessageCount++
			if err != nil {
				a.stats.ErrorCount++
				a.stats.LastErrorTime = time.Now()
			}
			a.stats.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

// Receive processes a message using the registered handler
func (a *BaseActor) Receive(ctx context.Context, msg Message) error {
	if a.handler == nil {
		return fmt.Errorf("no message handler registered for actor %s", a.id)
	}
	return a.handler(ctx, msg)
}

// Send sends a message to the actor
func (a *BaseActor) Send(msg Message) error {
	select {
	case a.inbox <- msg:
		return nil
	default:
		return fmt.Errorf("actor %s inbox is full", a.id)
	}
}

// SendWithContext sends a message with context cancellation support
func (a *BaseActor) SendWithContext(ctx context.Context, msg Message) error {
	select {
	case a.inbox <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop gracefully stops the actor
func (a *BaseActor) Stop() error {
	a.cancel() // Signal the run loop to stop

	// Close inbox if not already closed
	select {
	case <-a.inbox:
		// Channel is already closed
	default:
		close(a.inbox)
	}

	a.wg.Wait() // Wait for run to complete
	return nil
}

// ID returns the actor's unique identifier
func (a *BaseActor) ID() string {
	return a.id
}

// Type returns the actor's type
func (a *BaseActor) Type() string {
	return a.actorType
}

// Stats returns the actor's statistics
func (a *BaseActor) Stats() *ActorStats {
	return &a.stats
}

// GetAverageProcessTime returns the average message processing time
func (a *BaseActor) GetAverageProcessTime() time.Duration {
	a.stats.mu.RLock()
	defer a.stats.mu.RUnlock()

	if a.stats.MessageCount == 0 {
		return 0
	}
	return a.stats.ProcessTime / time.Duration(a.stats.MessageCount)
}
