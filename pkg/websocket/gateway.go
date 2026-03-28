package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType defines WebSocket message types.
type MessageType string

const (
	TypeAction   MessageType = "action"
	TypeReady    MessageType = "ready"
	TypeChat     MessageType = "chat"
	TypePing     MessageType = "ping"
	TypePong     MessageType = "pong"
	TypeError    MessageType = "error"
	TypeConnect  MessageType = "connected"
)

// WsMessage is the wire format for all WebSocket messages.
type WsMessage struct {
	Type MessageType   `json:"type"`
	Data any           `json:"data,omitempty"`
	Seq  int64         `json:"seq,omitempty"`
	Ts   int64         `json:"ts,omitempty"`
}

// Conn wraps a single WebSocket connection.
type Conn struct {
	ID       string
	PlayerID string
	RoomID   string
	ws       *websocket.Conn
	send     chan []byte
	hub      *Gateway

	mu       sync.Mutex
	closed   atomic.Bool
	lastPing atomic.Int64
}

// Gateway manages all WebSocket connections and message routing.
type Gateway struct {
	// Connection registry
	conns sync.Map // connID -> *Conn

	// Room membership
	rooms   sync.Map // roomID -> map[playerID]*Conn
	roomMu  sync.Map // roomID -> *sync.RWMutex (lazy init)

	// Player index
	playerConns sync.Map // playerID -> *Conn

	// Config
	writeWait  time.Duration
	pongWait   time.Duration
	pingPeriod time.Duration
	maxMsgSize int64
	sendBuf    int

	// Stats
	connCount atomic.Int64
	msgCount  atomic.Int64
}

// GatewayConfig holds gateway configuration.
type GatewayConfig struct {
	WriteWait  time.Duration
	PongWait   time.Duration
	MaxMsgSize int64
	SendBuf    int
}

// DefaultGatewayConfig returns sensible defaults.
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		WriteWait:  10 * time.Second,
		PongWait:   60 * time.Second,
		MaxMsgSize: 4096,
		SendBuf:    256,
	}
}

// NewGateway creates a new WebSocket gateway.
func NewGateway(cfg GatewayConfig) *Gateway {
	if cfg.SendBuf <= 0 {
		cfg.SendBuf = 256
	}
	return &Gateway{
		writeWait:  cfg.WriteWait,
		pongWait:   cfg.PongWait,
		pingPeriod: (cfg.PongWait * 9) / 10,
		maxMsgSize: cfg.MaxMsgSize,
		sendBuf:    cfg.SendBuf,
	}
}

// Register adds a connection to the gateway.
func (g *Gateway) Register(conn *Conn) {
	g.conns.Store(conn.ID, conn)
	g.playerConns.Store(conn.PlayerID, conn)
	g.connCount.Add(1)

	// Add to room if specified
	if conn.RoomID != "" {
		g.addToRoom(conn.RoomID, conn.PlayerID, conn)
	}

	slog.Debug("ws connection registered",
		"conn_id", conn.ID,
		"player_id", conn.PlayerID,
		"room_id", conn.RoomID,
	)
}

// Unregister removes a connection from the gateway.
func (g *Gateway) Unregister(connID string) {
	val, ok := g.conns.LoadAndDelete(connID)
	if !ok {
		return
	}
	conn := val.(*Conn)

	if conn.closed.CompareAndSwap(false, true) {
		conn.ws.Close()
	}

	g.playerConns.Delete(conn.PlayerID)
	g.connCount.Add(-1)

	// Remove from room
	if conn.RoomID != "" {
		g.removeFromRoom(conn.RoomID, conn.PlayerID)
	}

	slog.Debug("ws connection unregistered",
		"conn_id", connID,
		"player_id", conn.PlayerID,
	)
}

// BroadcastToRoom sends a message to all connections in a room.
func (g *Gateway) BroadcastToRoom(roomID string, event string, data any) {
	roomVal, ok := g.rooms.Load(roomID)
	if !ok {
		return
	}
	room := roomVal.(map[string]*Conn)

	msg := WsMessage{
		Type: MessageType(event),
		Data: data,
		Ts:   time.Now().UnixMilli(),
	}
	payload, _ := json.Marshal(msg)

	for _, conn := range room {
		conn.sendJSON(payload)
	}
	g.msgCount.Add(1)
}

// SendTo delivers a message to a specific player.
func (g *Gateway) SendTo(playerID string, event string, data any) {
	val, ok := g.playerConns.Load(playerID)
	if !ok {
		return
	}
	conn := val.(*Conn)

	msg := WsMessage{
		Type: MessageType(event),
		Data: data,
		Ts:   time.Now().UnixMilli(),
	}
	payload, _ := json.Marshal(msg)
	conn.sendJSON(payload)
	g.msgCount.Add(1)
}

// SendExcept sends a message to all room members except one player.
func (g *Gateway) SendExcept(roomID string, exceptPlayerID string, event string, data any) {
	roomVal, ok := g.rooms.Load(roomID)
	if !ok {
		return
	}
	room := roomVal.(map[string]*Conn)

	msg := WsMessage{
		Type: MessageType(event),
		Data: data,
		Ts:   time.Now().UnixMilli(),
	}
	payload, _ := json.Marshal(msg)

	for pid, conn := range room {
		if pid != exceptPlayerID {
			conn.sendJSON(payload)
		}
	}
	g.msgCount.Add(1)
}

// GetConn retrieves a connection by ID.
func (g *Gateway) GetConn(connID string) (*Conn, bool) {
	val, ok := g.conns.Load(connID)
	if !ok {
		return nil, false
	}
	return val.(*Conn), true
}

// Stats returns gateway statistics.
func (g *Gateway) Stats() (conns, messages int64) {
	return g.connCount.Load(), g.msgCount.Load()
}

// --- internal helpers ---

func (g *Gateway) addToRoom(roomID, playerID string, conn *Conn) {
	for {
		val, ok := g.rooms.Load(roomID)
		if !ok {
			newRoom := map[string]*Conn{playerID: conn}
			if _, loaded := g.rooms.LoadOrStore(roomID, newRoom); !loaded {
				return
			}
			// Race: another goroutine stored first, retry
			continue
		}
		room := val.(map[string]*Conn)
		room[playerID] = conn
		return
	}
}

func (g *Gateway) removeFromRoom(roomID, playerID string) {
	val, ok := g.rooms.Load(roomID)
	if !ok {
		return
	}
	room := val.(map[string]*Conn)
	delete(room, playerID)

	// Clean up empty room
	if len(room) == 0 {
		g.rooms.CompareAndDelete(roomID, val)
	}
}

// sendJSON sends a pre-marshaled message (non-blocking).
func (c *Conn) sendJSON(payload []byte) {
	if c.closed.Load() {
		return
	}
	select {
	case c.send <- payload:
	default:
		// Send buffer full — drop message
		slog.Warn("conn send buffer full, dropping",
			"conn_id", c.ID,
			"player_id", c.PlayerID,
		)
	}
}

// NewConn creates a new connection wrapper.
func NewConn(id, playerID, roomID string, ws *websocket.Conn, hub *Gateway) *Conn {
	return &Conn{
		ID:       id,
		PlayerID: playerID,
		RoomID:   roomID,
		ws:       ws,
		send:     make(chan []byte, hub.sendBuf),
		hub:      hub,
	}
}
