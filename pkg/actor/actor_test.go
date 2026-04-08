package actor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseActor_MessageProcessing(t *testing.T) {
	var received []*Message
	var mu sync.Mutex

	handler := func(msg *Message) error {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
		return nil
	}

	a := NewBaseActor("test-actor", handler, 16)
	a.Start()
	defer a.Stop(context.Background())

	// Send messages
	for i := 0; i < 5; i++ {
		msg := NewMessage(MsgPlayerAction, "player1", i)
		require.NoError(t, a.Send(msg))
	}

	// Wait for processing
	assert.Eventually(t, func() bool {
		processed, _ := a.Stats()
		return processed >= 5
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, received, 5)
	for i, msg := range received {
		assert.Equal(t, MsgPlayerAction, msg.Type)
		assert.Equal(t, "player1", msg.PlayerID)
		assert.Equal(t, i, msg.Data)
	}
}

func TestBaseActor_InboxFull(t *testing.T) {
	// Handler that blocks to keep inbox from draining
	blockCh := make(chan struct{})
	handler := func(msg *Message) error {
		<-blockCh
		return nil
	}

	a := NewBaseActor("full-actor", handler, 4) // small inbox
	a.Start()
	defer func() {
		close(blockCh)
		a.Stop(context.Background())
	}()

	// Fill inbox: first message blocks the handler, rest fill inbox
	for i := 0; i < 4; i++ {
		require.NoError(t, a.Send(NewMessage(MsgPlayerJoin, "", i)))
	}

	// Next send should fail
	err := a.Send(NewMessage(MsgPlayerJoin, "", "overflow"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inbox full")
}

func TestBaseActor_StopPreventsSend(t *testing.T) {
	handler := func(msg *Message) error { return nil }
	a := NewBaseActor("stop-actor", handler, 16)
	a.Start()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, a.Stop(ctx))

	err := a.Send(NewMessage(MsgPlayerJoin, "", nil))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestBaseActor_PanicRecovery(t *testing.T) {
	var processed atomic.Int32

	handler := func(msg *Message) error {
		if msg.Data == "panic" {
			panic("test panic")
		}
		processed.Add(1)
		return nil
	}

	a := NewBaseActor("panic-actor", handler, 16)
	a.Start()
	defer a.Stop(context.Background())

	// Send panic message
	require.NoError(t, a.Send(NewMessage(MsgPlayerAction, "p1", "panic")))
	// Send normal messages after panic
	require.NoError(t, a.Send(NewMessage(MsgPlayerAction, "p1", "ok1")))
	require.NoError(t, a.Send(NewMessage(MsgPlayerAction, "p1", "ok2")))

	assert.Eventually(t, func() bool {
		return processed.Load() == 2
	}, time.Second, 10*time.Millisecond)

	_, errors := a.Stats()
	assert.Equal(t, int64(1), errors)
}

func TestBaseActor_Stats(t *testing.T) {
	var processed, errored atomic.Int32

	handler := func(msg *Message) error {
		if msg.Data == "error" {
			errored.Add(1)
			return assert.AnError
		}
		processed.Add(1)
		return nil
	}

	a := NewBaseActor("stats-actor", handler, 16)
	a.Start()
	defer a.Stop(context.Background())

	require.NoError(t, a.Send(NewMessage(MsgPlayerAction, "", "ok")))
	require.NoError(t, a.Send(NewMessage(MsgPlayerAction, "", "error")))
	require.NoError(t, a.Send(NewMessage(MsgPlayerAction, "", "ok")))

	assert.Eventually(t, func() bool {
		p, e := a.Stats()
		return p == 3 && e == 1
	}, time.Second, 10*time.Millisecond)
}

func TestBaseActor_ConcurrentSend(t *testing.T) {
	var received atomic.Int32

	handler := func(msg *Message) error {
		received.Add(1)
		return nil
	}

	a := NewBaseActor("concurrent-actor", handler, 1024)
	a.Start()
	defer a.Stop(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.Send(NewMessage(MsgPlayerAction, "", nil))
		}()
	}
	wg.Wait()

	assert.Eventually(t, func() bool {
		return received.Load() == 100
	}, 2*time.Second, 10*time.Millisecond)
}

func TestBaseActor_StopWithContext(t *testing.T) {
	// Handler that never returns
	handler := func(msg *Message) error {
		select {} // blocks forever
	}

	a := NewBaseActor("timeout-actor", handler, 16)
	a.Start()

	// Send a message that will block the handler
	require.NoError(t, a.Send(NewMessage(MsgPlayerJoin, "", nil)))

	// Stop with short timeout should return error
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := a.Stop(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown timed out")
}

func TestBaseActor_DefaultInboxCap(t *testing.T) {
	handler := func(msg *Message) error { return nil }
	a := NewBaseActor("default-cap", handler, 0) // 0 should default to 256
	assert.Equal(t, 256, a.inboxCap)
}

func TestBaseActor_DoubleStart(t *testing.T) {
	handler := func(msg *Message) error { return nil }
	a := NewBaseActor("double-start", handler, 16)
	a.Start()
	defer a.Stop(context.Background())

	// Second start should be a no-op (already running)
	a.Start()
	assert.True(t, a.IsRunning())
}
