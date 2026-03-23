package actor

import (
	"sync"
	"time"
)

// MessageBatcher batches messages to reduce context switching
type MessageBatcher struct {
	batchSize int           // Batch size: 100
	interval  time.Duration // Batch interval: 10ms
	buffer    []Message     // Message buffer
	timer     *time.Timer   // Timer
	mu        sync.Mutex    // Mutex
	handler   BatchHandler  // Batch handler
	actor     Actor         // Target actor
}

// BatchHandler handles a batch of messages
type BatchHandler func(messages []Message) error

// BatcherConfig configures a message batcher
type BatcherConfig struct {
	BatchSize int           // Default: 100
	Interval  time.Duration // Default: 10ms
	Actor     Actor
	Handler   BatchHandler
}

// NewMessageBatcher creates a new message batcher
func NewMessageBatcher(config BatcherConfig) *MessageBatcher {
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	interval := config.Interval
	if interval == 0 {
		interval = 10 * time.Millisecond
	}

	return &MessageBatcher{
		batchSize: batchSize,
		interval:  interval,
		buffer:    make([]Message, 0, batchSize),
		actor:     config.Actor,
		handler:   config.Handler,
	}
}

// Send adds a message to the batch
func (b *MessageBatcher) Send(msg Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = append(b.buffer, msg)

	if len(b.buffer) >= b.batchSize {
		return b.flush()
	}

	if b.timer == nil {
		b.timer = time.AfterFunc(b.interval, func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			b.flush()
		})
	}

	return nil
}

// flush flushes the buffered messages
func (b *MessageBatcher) flush() error {
	if len(b.buffer) == 0 {
		return nil
	}

	messages := make([]Message, len(b.buffer))
	copy(messages, b.buffer)
	b.buffer = b.buffer[:0]

	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}

	if b.handler != nil {
		return b.handler(messages)
	}

	if b.actor != nil {
		for _, msg := range messages {
			_ = b.actor.Send(msg)
		}
	}

	return nil
}

// Flush manually flushes the buffer
func (b *MessageBatcher) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flush()
}

// Size returns the current buffer size
func (b *MessageBatcher) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buffer)
}

// Stop stops the batcher and flushes remaining messages
func (b *MessageBatcher) Stop() error {
	return b.Flush()
}
