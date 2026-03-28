package actor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

// Actor is the interface that all actors must implement.
type Actor interface {
	ID() string
	Send(msg *Message) error
	Stop(ctx context.Context) error
}

// MessageHandler processes a single message.
type MessageHandler func(msg *Message) error

// BaseActor provides the core actor behavior: a single goroutine
// processing messages from an inbox channel sequentially.
// No locks are needed because all state is accessed only from the run goroutine.
type BaseActor struct {
	id       string
	inbox    chan *Message
	handler  MessageHandler
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  atomic.Bool
	inboxCap int

	// Stats
	processed atomic.Int64
	errors    atomic.Int64
}

// NewBaseActor creates a new base actor with the given inbox capacity.
// The handler will be called for each message on a single goroutine.
func NewBaseActor(id string, handler MessageHandler, inboxCap int) *BaseActor {
	if inboxCap <= 0 {
		inboxCap = 256
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &BaseActor{
		id:       id,
		inbox:    make(chan *Message, inboxCap),
		handler:  handler,
		ctx:      ctx,
		cancel:   cancel,
		inboxCap: inboxCap,
	}
}

// Start launches the actor's message processing goroutine.
func (a *BaseActor) Start() {
	if !a.running.CompareAndSwap(false, true) {
		return // already running
	}
	a.wg.Add(1)
	go a.run()
}

// run is the core processing loop. Single goroutine, no locks.
func (a *BaseActor) run() {
	defer a.wg.Done()
	defer a.running.Store(false)

	for {
		select {
		case msg, ok := <-a.inbox:
			if !ok {
				return // inbox closed
			}
			a.processMessage(msg)

		case <-a.ctx.Done():
			a.drain()
			return
		}
	}
}

// processMessage handles a single message with panic recovery.
func (a *BaseActor) processMessage(msg *Message) {
	defer func() {
		if r := recover(); r != nil {
			a.errors.Add(1)
			slog.Error("actor panic",
				"actor_id", a.id,
				"msg_type", msg.Type,
				"panic", r,
			)
		}
	}()

	if err := a.handler(msg); err != nil {
		a.errors.Add(1)
		slog.Error("actor handler error",
			"actor_id", a.id,
			"msg_type", msg.Type,
			"error", err,
		)
	}
	a.processed.Add(1)
}

// drain processes any remaining messages in the inbox.
func (a *BaseActor) drain() {
	for {
		select {
		case msg, ok := <-a.inbox:
			if !ok {
				return
			}
			a.processMessage(msg)
		default:
			return
		}
	}
}

// Send delivers a message to the inbox (non-blocking).
func (a *BaseActor) Send(msg *Message) error {
	if !a.running.Load() {
		return fmt.Errorf("actor %s: not running", a.id)
	}
	select {
	case a.inbox <- msg:
		return nil
	default:
		return fmt.Errorf("actor %s: inbox full (%d)", a.id, a.inboxCap)
	}
}

// Stop gracefully stops the actor, waiting for in-flight messages.
func (a *BaseActor) Stop(ctx context.Context) error {
	a.cancel()
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("actor %s: shutdown timed out", a.id)
	}
}

// ID returns the actor's unique identifier.
func (a *BaseActor) ID() string { return a.id }

// Stats returns current actor statistics.
func (a *BaseActor) Stats() (processed, errors int64) {
	return a.processed.Load(), a.errors.Load()
}

// IsRunning reports whether the actor is active.
func (a *BaseActor) IsRunning() bool { return a.running.Load() }
