package actor

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorSystem_RegisterAndGet(t *testing.T) {
	sys := NewActorSystem()
	handler := func(msg *Message) error { return nil }

	a := NewBaseActor("actor-1", handler, 16)
	require.NoError(t, sys.Register(a))

	found, ok := sys.Get("actor-1")
	assert.True(t, ok)
	assert.Equal(t, "actor-1", found.ID())
}

func TestActorSystem_RegisterDuplicate(t *testing.T) {
	sys := NewActorSystem()
	handler := func(msg *Message) error { return nil }

	a1 := NewBaseActor("dup", handler, 16)
	require.NoError(t, sys.Register(a1))

	a2 := NewBaseActor("dup", handler, 16)
	err := sys.Register(a2)
	assert.ErrorIs(t, err, ErrActorExists)
}

func TestActorSystem_Unregister(t *testing.T) {
	sys := NewActorSystem()
	handler := func(msg *Message) error { return nil }

	a := NewBaseActor("removable", handler, 16)
	require.NoError(t, sys.Register(a))

	require.NoError(t, sys.Unregister("removable"))

	_, ok := sys.Get("removable")
	assert.False(t, ok)
}

func TestActorSystem_UnregisterNotFound(t *testing.T) {
	sys := NewActorSystem()
	err := sys.Unregister("nonexistent")
	assert.ErrorIs(t, err, ErrActorNotFound)
}

func TestActorSystem_SendTo(t *testing.T) {
	sys := NewActorSystem()
	var received atomic.Bool
	handler := func(msg *Message) error {
		received.Store(true)
		return nil
	}

	a := NewBaseActor("target", handler, 16)
	require.NoError(t, sys.Register(a))
	defer sys.Unregister("target")

	msg := NewMessage(MsgPlayerAction, "p1", nil)
	require.NoError(t, sys.SendTo("target", msg))

	assert.Eventually(t, func() bool {
		return received.Load()
	}, time.Second, 10*time.Millisecond)
}

func TestActorSystem_SendToNotFound(t *testing.T) {
	sys := NewActorSystem()
	msg := NewMessage(MsgPlayerAction, "", nil)
	err := sys.SendTo("missing", msg)
	assert.ErrorIs(t, err, ErrActorNotFound)
}

func TestActorSystem_Count(t *testing.T) {
	sys := NewActorSystem()
	handler := func(msg *Message) error { return nil }

	assert.Equal(t, int64(0), sys.Count())

	a1 := NewBaseActor("c1", handler, 16)
	require.NoError(t, sys.Register(a1))
	assert.Equal(t, int64(1), sys.Count())

	a2 := NewBaseActor("c2", handler, 16)
	require.NoError(t, sys.Register(a2))
	assert.Equal(t, int64(2), sys.Count())

	require.NoError(t, sys.Unregister("c1"))
	assert.Equal(t, int64(1), sys.Count())
}

func TestActorSystem_Shutdown(t *testing.T) {
	sys := NewActorSystem()
	handler := func(msg *Message) error { return nil }

	for i := 0; i < 5; i++ {
		id := "s" + string(rune('0'+i))
		a := NewBaseActor(id, handler, 16)
		require.NoError(t, sys.Register(a))
	}

	assert.Equal(t, int64(5), sys.Count())
	sys.Shutdown(5 * time.Second)
	assert.Equal(t, int64(0), sys.Count())
}
