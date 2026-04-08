package websocket

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestConn creates a Conn with a send buffer but no real WebSocket.
// closed is set to true so Unregister skips ws.Close().
func newTestConn(id, playerID, roomID string, g *Gateway) *Conn {
	c := &Conn{
		ID:       id,
		PlayerID: playerID,
		RoomID:   roomID,
		send:     make(chan []byte, 256),
		hub:      g,
	}
	c.closed.Store(true) // skip ws.Close() in Unregister
	return c
}

// newSendConn creates a Conn that can receive messages (closed=false).
// Caller must NOT call Unregister on this conn (no real ws).
func newSendConn(id, playerID, roomID string, g *Gateway) *Conn {
	return &Conn{
		ID:       id,
		PlayerID: playerID,
		RoomID:   roomID,
		send:     make(chan []byte, 256),
		hub:      g,
	}
}

func TestGateway_RegisterUnregister_NoRoom(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())
	assert.Equal(t, int64(0), g.Count())

	conn := newTestConn("c1", "p1", "", g)
	g.Register(conn)
	assert.Equal(t, int64(1), g.Count())

	found, ok := g.GetConn("c1")
	assert.True(t, ok)
	assert.Equal(t, "c1", found.ID)

	g.Unregister("c1")
	assert.Equal(t, int64(0), g.Count())

	_, ok = g.GetConn("c1")
	assert.False(t, ok)
}

func TestGateway_RegisterWithRoom(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())

	conn := newTestConn("c1", "p1", "room1", g)
	g.Register(conn)

	roomVal, ok := g.rooms.Load("room1")
	assert.True(t, ok)
	room := roomVal.(map[string]*Conn)
	assert.Contains(t, room, "p1")
}

func TestGateway_DoubleUnregister(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())

	conn := newTestConn("c1", "p1", "", g)
	g.Register(conn)
	g.Unregister("c1")
	g.Unregister("c1") // should not panic

	assert.Equal(t, int64(0), g.Count())
}

func TestGateway_BroadcastToRoom(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())

	// Use sendConns (closed=false) to actually receive messages
	conn1 := newSendConn("c1", "p1", "room1", g)
	conn2 := newSendConn("c2", "p2", "room1", g)
	conn3 := newSendConn("c3", "p3", "room2", g)

	g.Register(conn1)
	g.Register(conn2)
	g.Register(conn3)

	g.BroadcastToRoom("room1", "test_event", map[string]any{"key": "value"})

	// conn1 and conn2 should receive, conn3 should not
	msgs1 := drainChan(conn1.send)
	msgs2 := drainChan(conn2.send)
	msgs3 := drainChan(conn3.send)

	require.Len(t, msgs1, 1)
	require.Len(t, msgs2, 1)
	assert.Len(t, msgs3, 0)

	var msg WsMessage
	require.NoError(t, json.Unmarshal(msgs1[0], &msg))
	assert.Equal(t, MessageType("test_event"), msg.Type)
}

func TestGateway_BroadcastToEmptyRoom(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())
	g.BroadcastToRoom("nonexistent", "event", nil) // should not panic
}

func TestGateway_SendTo(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())

	conn := newSendConn("c1", "p1", "", g)
	g.Register(conn)

	g.SendTo("p1", "private_msg", map[string]any{"secret": 42})

	msgs := drainChan(conn.send)
	require.Len(t, msgs, 1)

	var msg WsMessage
	require.NoError(t, json.Unmarshal(msgs[0], &msg))
	assert.Equal(t, MessageType("private_msg"), msg.Type)
}

func TestGateway_SendToUnknown(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())
	g.SendTo("unknown_player", "event", nil) // should not panic
}

func TestGateway_SendExcept(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())

	conn1 := newSendConn("c1", "p1", "room1", g)
	conn2 := newSendConn("c2", "p2", "room1", g)

	g.Register(conn1)
	g.Register(conn2)

	g.SendExcept("room1", "p1", "hidden_event", "data")

	assert.Len(t, drainChan(conn1.send), 0)
	assert.Len(t, drainChan(conn2.send), 1)
}

func TestGateway_Stats(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())

	conn1 := newSendConn("c1", "p1", "room1", g)
	g.Register(conn1)

	g.BroadcastToRoom("room1", "event1", nil)
	g.BroadcastToRoom("room1", "event2", nil)

	conns, msgs := g.Stats()
	assert.Equal(t, int64(1), conns)
	assert.Equal(t, int64(2), msgs)
}

func TestGateway_ConcurrentRegisterUnregister(t *testing.T) {
	g := NewGateway(DefaultGatewayConfig())
	var count atomic.Int64

	done := make(chan struct{})
	for i := range 50 {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			id := "c" + string(rune('0'+n))
			pid := "p" + string(rune('0'+n))
			conn := newTestConn(id, pid, "", g)
			g.Register(conn)
			count.Add(1)
			g.Unregister(id)
			count.Add(-1)
		}(i)
	}

	for range 50 {
		<-done
	}

	assert.Equal(t, int64(0), g.Count())
}

func TestDefaultGatewayConfig(t *testing.T) {
	cfg := DefaultGatewayConfig()
	assert.Equal(t, 10*time.Second, cfg.WriteWait)
	assert.Equal(t, 60*time.Second, cfg.PongWait)
	assert.Equal(t, int64(4096), cfg.MaxMsgSize)
}

// --- Helper ---

func drainChan(ch <-chan []byte) [][]byte {
	var msgs [][]byte
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}
