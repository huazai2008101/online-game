package websocket

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	ErrConnectionClosed    = errors.New("connection closed")
	ErrMessageTooLarge     = errors.New("message too large")
	ErrConnectionNotFound  = errors.New("connection not found")
	ErrRoomNotFound        = errors.New("room not found")
	ErrAlreadyInRoom       = errors.New("already in room")
)

// MessageType represents WebSocket message types
type MessageType string

const (
	MessageTypeText       MessageType = "text"
	MessageTypeBinary     MessageType = "binary"
	MessageTypePing       MessageType = "ping"
	MessageTypePong       MessageType = "pong"
	MessageTypeClose      MessageType = "close"
	MessageTypeError      MessageType = "error"
	MessageTypeRoomJoin   MessageType = "room_join"
	MessageTypeRoomLeave  MessageType = "room_leave"
	MessageTypeRoomMsg    MessageType = "room_msg"
)

// Message represents a WebSocket message
type Message struct {
	Type    MessageType `json:"type"`
	From    string      `json:"from,omitempty"`
	To      string      `json:"to,omitempty"`
	Room    string      `json:"room,omitempty"`
	Content interface{} `json:"content,omitempty"`
	Data    []byte      `json:"-"`
	Time    time.Time   `json:"time"`
}

// ConnStats tracks connection statistics
type ConnStats struct {
	MessagesSent     atomic.Int64
	MessagesReceived atomic.Int64
	BytesSent        atomic.Int64
	BytesReceived    atomic.Int64
	Errors           atomic.Int64
	ConnectedAt      time.Time
	LastActivityAt   atomic.Value
}

// Conn represents a WebSocket connection
type Conn struct {
	id       string
	ws       *websocket.Conn
	send     chan *Message
	receive  chan *Message
	close    chan struct{}
	once     sync.Once
	stats    *ConnStats
	handlers map[MessageType]func(*Message)
	mu       sync.RWMutex
	rooms    map[string]bool
	metadata map[string]interface{}
}

// NewConn creates a new WebSocket connection wrapper
func NewConn(id string, ws *websocket.Conn) *Conn {
	stats := &ConnStats{
		ConnectedAt: time.Now(),
	}
	stats.LastActivityAt.Store(time.Now())

	return &Conn{
		id:       id,
		ws:       ws,
		send:     make(chan *Message, 256),
		receive:  make(chan *Message, 256),
		close:    make(chan struct{}),
		stats:    stats,
		handlers: make(map[MessageType]func(*Message)),
		rooms:    make(map[string]bool),
		metadata: make(map[string]interface{}),
	}
}

// ID returns the connection ID
func (c *Conn) ID() string {
	return c.id
}

// Send sends a message to the connection
func (c *Conn) Send(msg *Message) error {
	select {
	case c.send <- msg:
		return nil
	case <-c.close:
		return ErrConnectionClosed
	default:
		return errors.New("send buffer full")
	}
}

// Receive returns the receive channel
func (c *Conn) Receive() <-chan *Message {
	return c.receive
}

// RegisterHandler registers a message handler
func (c *Conn) RegisterHandler(msgType MessageType, handler func(*Message)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = handler
}

// SetMetadata sets metadata for the connection
func (c *Conn) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata gets metadata for the connection
func (c *Conn) GetMetadata(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.metadata[key]
	return val, ok
}

// Stats returns connection statistics
func (c *Conn) Stats() *ConnStats {
	return c.stats
}

// JoinRoom adds the connection to a room
func (c *Conn) JoinRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms[room] = true
}

// LeaveRoom removes the connection from a room
func (c *Conn) LeaveRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rooms, room)
}

// InRoom checks if connection is in a room
func (c *Conn) InRoom(room string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rooms[room]
}

// GetRooms returns all rooms the connection is in
func (c *Conn) GetRooms() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rooms := make([]string, 0, len(c.rooms))
	for room := range c.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// Close closes the connection
func (c *Conn) Close() error {
	var err error
	c.once.Do(func() {
		close(c.close)
		err = c.ws.Close()
	})
	return err
}

// writePump pumps messages to the WebSocket connection
func (c *Conn) writePump(ctx context.Context) {
	defer c.Close()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.close:
			return
		case msg, ok := <-c.send:
			if !ok {
				return
			}

			if err := c.ws.WriteJSON(msg); err != nil {
				c.stats.Errors.Add(1)
				return
			}

			c.stats.MessagesSent.Add(1)
			c.stats.LastActivityAt.Store(time.Now())

		case <-ticker.C:
			// Send ping
			if err := c.ws.WriteJSON(&Message{
				Type: MessageTypePing,
				Time: time.Now(),
			}); err != nil {
				return
			}
		}
	}
}

// readPump reads messages from the WebSocket connection
func (c *Conn) readPump(ctx context.Context) {
	defer c.Close()

	c.ws.SetReadLimit(32768)
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.close:
			return
		default:
			_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))

			var msg Message
			if err := c.ws.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.stats.Errors.Add(1)
				}
				return
			}

			msg.Time = time.Now()
			msg.From = c.id
			c.stats.MessagesReceived.Add(1)
			c.stats.LastActivityAt.Store(time.Now())

			// Handle ping/pong
			if msg.Type == MessageTypePong {
				continue
			}

			if msg.Type == MessageTypePing {
				_ = c.Send(&Message{Type: MessageTypePong})
				continue
			}

			// Call registered handler
			c.mu.RLock()
			handler, ok := c.handlers[msg.Type]
			c.mu.RUnlock()

			if ok {
				go handler(&msg)
			}

			// Send to receive channel if room message
			if msg.Type == MessageTypeRoomMsg {
				select {
				case c.receive <- &msg:
				default:
					c.stats.Errors.Add(1)
				}
			}
		}
	}
}

// GatewayConfig defines gateway configuration
type GatewayConfig struct {
	ReadBufferSize    int
	WriteBufferSize   int
	PingPeriod        time.Duration
	PongWait          time.Duration
	MaxMessageSize    int64
	EnableCompression bool
}

// DefaultGatewayConfig returns default gateway configuration
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		PingPeriod:        30 * time.Second,
		PongWait:          60 * time.Second,
		MaxMessageSize:    32768,
		EnableCompression: false,
	}
}

// Gateway manages WebSocket connections
type Gateway struct {
	conns    map[string]*Conn
	rooms    map[string]map[string]*Conn // room -> connID -> conn
	mu       sync.RWMutex
	config   *GatewayConfig
	upgrader *websocket.Upgrader
	stats    *GatewayStats
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// GatewayStats tracks gateway statistics
type GatewayStats struct {
	ConnectionsTotal  atomic.Int64
	ConnectionsActive atomic.Int64
	MessagesSent      atomic.Int64
	MessagesReceived  atomic.Int64
	Errors            atomic.Int64
	RoomsCount        atomic.Int64
}

// NewGateway creates a new WebSocket gateway
func NewGateway(config *GatewayConfig) *Gateway {
	if config == nil {
		config = DefaultGatewayConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	g := &Gateway{
		conns:  make(map[string]*Conn),
		rooms:  make(map[string]map[string]*Conn),
		config: config,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  config.ReadBufferSize,
			WriteBufferSize: config.WriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				return true // Configure appropriately for production
			},
		},
		stats:  &GatewayStats{},
		ctx:    ctx,
		cancel: cancel,
	}

	return g
}

// AddConnection adds a new connection to the gateway
func (g *Gateway) AddConnection(conn *Conn) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.conns[conn.id] = conn
	g.stats.ConnectionsTotal.Add(1)
	g.stats.ConnectionsActive.Add(1)

	// Start read/write pumps
	g.wg.Add(2)
	go conn.writePump(g.ctx)
	go conn.readPump(g.ctx)

	// Cleanup on close
	go func() {
		<-conn.close
		g.RemoveConnection(conn.id)
	}()
}

// RemoveConnection removes a connection from the gateway
func (g *Gateway) RemoveConnection(connID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	conn, ok := g.conns[connID]
	if !ok {
		return
	}

	// Remove from all rooms
	for room := range conn.rooms {
		if roomConns, ok := g.rooms[room]; ok {
			delete(roomConns, connID)
			if len(roomConns) == 0 {
				delete(g.rooms, room)
				g.stats.RoomsCount.Add(-1)
			}
		}
	}

	delete(g.conns, connID)
	g.stats.ConnectionsActive.Add(-1)
}

// GetConnection returns a connection by ID
func (g *Gateway) GetConnection(connID string) (*Conn, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	conn, ok := g.conns[connID]
	return conn, ok
}

// Broadcast sends a message to all connections
func (g *Gateway) Broadcast(msg *Message) int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	count := 0
	for _, conn := range g.conns {
		if err := conn.Send(msg); err == nil {
			count++
		}
	}
	g.stats.MessagesSent.Add(int64(count))
	return count
}

// SendTo sends a message to a specific connection
func (g *Gateway) SendTo(connID string, msg *Message) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	conn, ok := g.conns[connID]
	if !ok {
		return ErrConnectionNotFound
	}

	return conn.Send(msg)
}

// SendToRoom sends a message to all connections in a room
func (g *Gateway) SendToRoom(room string, msg *Message) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	roomConns, ok := g.rooms[room]
	if !ok {
		return 0, ErrRoomNotFound
	}

	count := 0
	for _, conn := range roomConns {
		if err := conn.Send(msg); err == nil {
			count++
		}
	}
	g.stats.MessagesSent.Add(int64(count))
	return count, nil
}

// JoinRoom adds a connection to a room
func (g *Gateway) JoinRoom(connID, room string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	conn, ok := g.conns[connID]
	if !ok {
		return ErrConnectionNotFound
	}

	if conn.InRoom(room) {
		return ErrAlreadyInRoom
	}

	// Create room if not exists
	if _, ok := g.rooms[room]; !ok {
		g.rooms[room] = make(map[string]*Conn)
		g.stats.RoomsCount.Add(1)
	}

	g.rooms[room][connID] = conn
	conn.JoinRoom(room)

	// Notify room
	_, _ = g.SendToRoom(room, &Message{
		Type: MessageTypeRoomJoin,
		From: connID,
		Room: room,
		Time: time.Now(),
	})

	return nil
}

// LeaveRoom removes a connection from a room
func (g *Gateway) LeaveRoom(connID, room string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	conn, ok := g.conns[connID]
	if !ok {
		return ErrConnectionNotFound
	}

	if !conn.InRoom(room) {
		return nil
	}

	delete(g.rooms[room], connID)
	conn.LeaveRoom(room)

	// Clean up empty room
	if len(g.rooms[room]) == 0 {
		delete(g.rooms, room)
		g.stats.RoomsCount.Add(-1)
	}

	// Notify room
	_, _ = g.SendToRoom(room, &Message{
		Type: MessageTypeRoomLeave,
		From: connID,
		Room: room,
		Time: time.Now(),
	})

	return nil
}

// GetRoomMembers returns all members of a room
func (g *Gateway) GetRoomMembers(room string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	roomConns, ok := g.rooms[room]
	if !ok {
		return nil
	}

	members := make([]string, 0, len(roomConns))
	for connID := range roomConns {
		members = append(members, connID)
	}
	return members
}

// GetRoomCount returns the number of rooms
func (g *Gateway) GetRoomCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.rooms)
}

// GetConnectionCount returns the number of active connections
func (g *Gateway) GetConnectionCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.conns)
}

// Stats returns gateway statistics
func (g *Gateway) Stats() *GatewayStats {
	return g.stats
}

// Close closes the gateway and all connections
func (g *Gateway) Close() error {
	g.cancel()

	g.mu.Lock()
	conns := make([]*Conn, 0, len(g.conns))
	for _, conn := range g.conns {
		conns = append(conns, conn)
	}
	g.mu.Unlock()

	// Close all connections
	for _, conn := range conns {
		_ = conn.Close()
	}

	g.wg.Wait()
	return nil
}
