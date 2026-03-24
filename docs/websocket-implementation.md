# WebSocket 详细实现

**文档版本:** v1.0
**创建时间:** 2026-03-24

---

## 目录

1. [设计概述](#1-设计概述)
2. [连接管理](#2-连接管理)
3. [消息处理](#3-消息处理)
4. [房间管理](#4-房间管理)
5. [Hub 实现](#5-hub-实现)
6. [中间件](#6-中间件)
7. [使用示例](#7-使用示例)

---

## 1. 设计概述

### 1.1 设计目标

1. **高性能**: 支持大量并发连接
2. **可扩展**: 支持水平扩展
3. **可靠性**: 自动重连和消息确认
4. **安全性**: 身份验证和消息加密

### 1.2 架构设计

```
                    ┌─────────────────┐
                    │   HTTP Server   │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  WebSocket      │
                    │  Upgrader       │
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
         ┌────▼─────┐                 ┌────▼─────┐
         │   Hub    │                 │  Router  │
         │          │                 │          │
         └────┬─────┘                 └────┬─────┘
              │                             │
    ┌─────────┼─────────┐         ┌─────────┼─────────┐
    │         │         │         │         │         │
┌───▼──┐ ┌───▼──┐ ┌───▼──┐ ┌───▼──┐ ┌───▼──┐ ┌───▼──┐
│ Conn │ │ Conn │ │ Conn │ │ Conn │ │ Conn │ │ Conn │
└──────┘ └──────┘ └──────┘ └──────┘ └──────┘ └──────┘
    │        │        │        │        │        │
    └────────┴────────┴────────┴────────┴────────┘
                        │
                  ┌─────▼─────┐
                  │  Room 1   │
                  └───────────┘
```

---

## 2. 连接管理

### 2.1 连接实现

```go
// internal/ws/connection.go
package ws

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "sync/atomic"
    "time"

    "github.com/gorilla/websocket"
)

// Conn WebSocket连接
type Conn struct {
    // 基础信息
    id       string
    ws       *websocket.Conn
    hub      *Hub

    // 用户信息
    userID   string
    metadata map[string]interface{}
    metadataMu sync.RWMutex

    // 状态
    connected atomic.Bool
    closed    atomic.Bool

    // 通道
    send     chan *Envelope
    recv     chan *Message

    // 统计
    stats    ConnStats

    // 配置
    config   ConnConfig

    // 上下文
    ctx      context.Context
    cancel   context.CancelFunc
    wg       sync.WaitGroup
}

// ConnConfig 连接配置
type ConnConfig struct {
    ReadBufferSize  int
    WriteBufferSize int
    MaxMessageSize  int64
    PingPeriod      time.Duration
    PongWait        time.Duration
    WriteWait       time.Duration
    EnableCompression bool
}

// DefaultConnConfig 默认配置
var DefaultConnConfig = ConnConfig{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
    MaxMessageSize:  512 * 1024, // 512KB
    PingPeriod:      54 * time.Second,
    PongWait:        60 * time.Second,
    WriteWait:       10 * time.Second,
    EnableCompression: false,
}

// NewConn 创建连接
func NewConn(id string, ws *websocket.Conn, hub *Hub, config ConnConfig) *Conn {
    ctx, cancel := context.WithCancel(context.Background())

    return &Conn{
        id:       id,
        ws:       ws,
        hub:      hub,
        send:     make(chan *Envelope, 256),
        recv:     make(chan *Message, 256),
        metadata: make(map[string]interface{}),
        config:   config,
        ctx:      ctx,
        cancel:   cancel,
        stats: ConnStats{
            ConnectedAt: time.Now(),
        },
    }
}

// ID 返回连接ID
func (c *Conn) ID() string {
    return c.id
}

// UserID 返回用户ID
func (c *Conn) UserID() string {
    return c.userID
}

// SetUserID 设置用户ID
func (c *Conn) SetUserID(userID string) {
    c.userID = userID
    c.metadataMu.Lock()
    c.metadata["user_id"] = userID
    c.metadataMu.Unlock()
}

// Metadata 获取元数据
func (c *Conn) Metadata() map[string]interface{} {
    c.metadataMu.RLock()
    defer c.metadataMu.RUnlock()

    metadata := make(map[string]interface{}, len(c.metadata))
    for k, v := range c.metadata {
        metadata[k] = v
    }
    return metadata
}

// SetMetadata 设置元数据
func (c *Conn) SetMetadata(key string, value interface{}) {
    c.metadataMu.Lock()
    defer c.metadataMu.Unlock()
    c.metadata[key] = value
}

// GetMetadata 获取元数据
func (c *Conn) GetMetadata(key string) (interface{}, bool) {
    c.metadataMu.RLock()
    defer c.metadataMu.RUnlock()
    v, ok := c.metadata[key]
    return v, ok
}

// RemoteAddr 返回远程地址
func (c *Conn) RemoteAddr() string {
    return c.ws.RemoteAddr().String()
}

// UserAgent 返回用户代理
func (c *Conn) UserAgent() string {
    return c.ws.Request().UserAgent()
}

// Start 启动连接
func (c *Conn) Start() {
    c.connected.Store(true)

    // 启动读取循环
    c.wg.Add(1)
    go c.readPump()

    // 启动写入循环
    c.wg.Add(1)
    go c.writePump()

    // 启动心跳
    c.wg.Add(1)
    go c.heartbeat()

    // 启动消息处理
    c.wg.Add(1)
    go c.processMessages()
}

// Stop 停止连接
func (c *Conn) Stop() error {
    if !c.closed.CompareAndSwap(false, true) {
        return nil // 已经关闭
    }

    c.cancel()
    c.connected.Store(false)

    // 发送关闭消息
    select {
    case c.send <- &Envelope{Type: "close"}:
    case <-time.After(time.Second):
    }

    // 等待所有goroutine结束
    done := make(chan struct{})
    go func() {
        c.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
    case <-time.After(5 * time.Second):
        // 强制关闭
    }

    // 关闭WebSocket连接
    _ = c.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(
        websocket.CloseNormalClosure, ""))
    _ = c.ws.Close()

    return nil
}

// Send 发送消息
func (c *Conn) Send(msg *Message) error {
    if c.closed.Load() {
        return fmt.Errorf("connection closed")
    }

    select {
    case c.send <- &Envelope{Message: msg}:
        return nil
    case <-time.After(c.config.WriteWait):
        return fmt.Errorf("send timeout")
    }
}

// SendBytes 发送字节数据
func (c *Conn) SendBytes(data []byte) error {
    if c.closed.Load() {
        return fmt.Errorf("connection closed")
    }

    select {
    case c.send <- &Envelope{Data: data}:
        return nil
    case <-time.After(c.config.WriteWait):
        return fmt.Errorf("send timeout")
    }
}

// SendJSON 发送JSON数据
func (c *Conn) SendJSON(v interface{}) error {
    data, err := json.Marshal(v)
    if err != nil {
        return err
    }
    return c.SendBytes(data)
}

// readPump 读取循环
func (c *Conn) readPump() {
    defer c.wg.Done()

    c.ws.SetReadLimit(int(c.config.MaxMessageSize))
    c.ws.SetReadDeadline(time.Now().Add(c.config.PongWait))
    c.ws.SetPongHandler(func(string) error {
        c.ws.SetReadDeadline(time.Now().Add(c.config.PongWait))
        c.stats.LastActivity = time.Now()
        return nil
    })

    for {
        select {
        case <-c.ctx.Done():
            return
        default:
        }

        messageType, data, err := c.ws.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                // 记录错误
            }
            return
        }

        c.stats.MessagesReceived++
        c.stats.BytesReceived += int64(len(data))
        c.stats.LastActivity = time.Now()

        // 处理消息
        msg := &Message{
            Type:     getMessageType(messageType),
            Data:     data,
            ConnID:   c.id,
            UserID:   c.userID,
        }

        select {
        case c.recv <- msg:
        case <-c.ctx.Done():
            return
        case <-time.After(time.Second):
            // 接收通道满，丢弃消息
        }
    }
}

// writePump 写入循环
func (c *Conn) writePump() {
    defer c.wg.Done()

    ticker := time.NewTicker(c.config.PingPeriod)
    defer ticker.Stop()

    for {
        select {
        case envelope, ok := <-c.send:
            c.ws.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
            if !ok || envelope.Type == "close" {
                return
            }

            // 发送数据
            if err := c.write(envelope); err != nil {
                return
            }

            c.stats.MessagesSent++
            if envelope.Data != nil {
                c.stats.BytesSent += int64(len(envelope.Data))
            }

        case <-ticker.C:
            c.ws.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
            if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }

        case <-c.ctx.Done():
            return
        }
    }
}

// write 写入数据
func (c *Conn) write(envelope *Envelope) error {
    if envelope.Message != nil {
        // 发送结构化消息
        data, err := json.Marshal(envelope.Message)
        if err != nil {
            return err
        }
        return c.ws.WriteMessage(websocket.TextMessage, data)
    }

    if envelope.Data != nil {
        // 发送原始数据
        messageType := websocket.TextMessage
        if envelope.IsBinary {
            messageType = websocket.BinaryMessage
        }
        return c.ws.WriteMessage(messageType, envelope.Data)
    }

    return nil
}

// processMessages 处理消息
func (c *Conn) processMessages() {
    defer c.wg.Done()

    for {
        select {
        case msg := <-c.recv:
            if err := c.handleMessage(msg); err != nil {
                c.SendJSON(map[string]interface{}{
                    "type":    "error",
                    "code":    err.Error(),
                    "message": err.Error(),
                })
            }

        case <-c.ctx.Done():
            return
        }
    }
}

// handleMessage 处理单个消息
func (c *Conn) handleMessage(msg *Message) error {
    // 解析消息
    var parsed struct {
        Type string                 `json:"type"`
        Data map[string]interface{} `json:"data"`
    }

    if err := json.Unmarshal(msg.Data, &parsed); err != nil {
        return fmt.Errorf("invalid message format")
    }

    // 根据类型路由
    switch parsed.Type {
    case "ping":
        return c.handlePing(parsed.Data)
    case "pong":
        return c.handlePong(parsed.Data)
    case "subscribe":
        return c.handleSubscribe(parsed.Data)
    case "unsubscribe":
        return c.handleUnsubscribe(parsed.Data)
    case "join_room":
        return c.handleJoinRoom(parsed.Data)
    case "leave_room":
        return c.handleLeaveRoom(parsed.Data)
    default:
        // 转发到Hub处理
        c.hub.RouteMessage(msg, c)
    }

    return nil
}

func (c *Conn) handlePing(data map[string]interface{}) error {
    return c.SendJSON(map[string]interface{}{
        "type": "pong",
        "time": time.Now().Unix(),
    })
}

func (c *Conn) handlePong(data map[string]interface{}) error {
    c.stats.LastActivity = time.Now()
    return nil
}

func (c *Conn) handleSubscribe(data map[string]interface{}) error {
    channel, ok := data["channel"].(string)
    if !ok {
        return fmt.Errorf("channel required")
    }

    return c.hub.Subscribe(c, channel)
}

func (c *Conn) handleUnsubscribe(data map[string]interface{}) error {
    channel, ok := data["channel"].(string)
    if !ok {
        return fmt.Errorf("channel required")
    }

    return c.hub.Unsubscribe(c, channel)
}

func (c *Conn) handleJoinRoom(data map[string]interface{}) error {
    roomID, ok := data["room_id"].(string)
    if !ok {
        return fmt.Errorf("room_id required")
    }

    return c.hub.JoinRoom(roomID, c)
}

func (c *Conn) handleLeaveRoom(data map[string]interface{}) error {
    roomID, ok := data["room_id"].(string)
    if !ok {
        return fmt.Errorf("room_id required")
    }

    return c.hub.LeaveRoom(roomID, c)
}

// heartbeat 心跳检查
func (c *Conn) heartbeat() {
    defer c.wg.Done()

    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if time.Since(c.stats.LastActivity) > c.config.PongWait {
                // 超时，关闭连接
                c.Stop()
                return
            }

        case <-c.ctx.Done():
            return
        }
    }
}

// Stats 返回统计信息
func (c *Conn) Stats() ConnStats {
    return ConnStats{
        ID:              c.id,
        UserID:          c.userID,
        RemoteAddr:      c.RemoteAddr(),
        UserAgent:       c.UserAgent(),
        ConnectedAt:     c.stats.ConnectedAt,
        LastActivity:    c.stats.LastActivity,
        MessagesSent:    atomic.LoadInt64(&c.stats.MessagesSent),
        MessagesReceived: atomic.LoadInt64(&c.stats.MessagesReceived),
        BytesSent:       atomic.LoadInt64(&c.stats.BytesSent),
        BytesReceived:   atomic.LoadInt64(&c.stats.BytesReceived),
    }
}

// Envelope 消息封装
type Envelope struct {
    Type    string
    Message *Message
    Data    []byte
    IsBinary bool
}

// Message 消息
type Message struct {
    Type     string                 `json:"type"`
    Data     []byte                 `json:"-"`
    Payload  map[string]interface{} `json:"payload"`
    ConnID   string                 `json:"conn_id,omitempty"`
    UserID   string                 `json:"user_id,omitempty"`
    RoomID   string                 `json:"room_id,omitempty"`
}

// ConnStats 连接统计
type ConnStats struct {
    ID               string
    UserID           string
    RemoteAddr       string
    UserAgent        string
    ConnectedAt      time.Time
    LastActivity     time.Time
    MessagesSent     int64
    MessagesReceived int64
    BytesSent        int64
    BytesReceived    int64
}

func getMessageType(t int) string {
    switch t {
    case websocket.TextMessage:
        return "text"
    case websocket.BinaryMessage:
        return "binary"
    default:
        return "unknown"
    }
}
```

### 2.2 连接管理器

```go
// internal/ws/manager.go
package ws

import (
    "context"
    "net/http"
    "sync"

    "github.com/google/uuid"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
    CheckOrigin: func(r *http.Request) bool {
        return true // 生产环境需要验证Origin
    },
}

// Manager 连接管理器
type Manager struct {
    hub    *Hub
    conns  map[string]*Conn
    mu     sync.RWMutex
}

// NewManager 创建管理器
func NewManager(hub *Hub) *Manager {
    return &Manager{
        hub:   hub,
        conns: make(map[string]*Conn),
    }
}

// Upgrade 升级HTTP连接到WebSocket
func (m *Manager) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
    // 升级连接
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return nil, err
    }

    // 创建连接
    connID := uuid.New().String()
    conn := NewConn(connID, ws, m.hub, DefaultConnConfig)

    // 注册到Hub
    m.hub.Register(conn)

    // 保存到管理器
    m.mu.Lock()
    m.conns[connID] = conn
    m.mu.Unlock()

    // 启动连接
    conn.Start()

    return conn, nil
}

// Get 获取连接
func (m *Manager) Get(connID string) (*Conn, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    conn, ok := m.conns[connID]
    return conn, ok
}

// Remove 移除连接
func (m *Manager) Remove(connID string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if conn, ok := m.conns[connID]; ok {
        conn.Stop()
        delete(m.conns, connID)
    }
}

// Broadcast 广播消息
func (m *Manager) Broadcast(msg *Message) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, conn := range m.conns {
        _ = conn.Send(msg)
    }
}

// SendToUser 发送消息给指定用户
func (m *Manager) SendToUser(userID string, msg *Message) error {
    m.mu.RLock()
    defer m.mu.RUnlock()

    sent := false
    for _, conn := range m.conns {
        if conn.UserID() == userID {
            _ = conn.Send(msg)
            sent = true
        }
    }

    if !sent {
        return fmt.Errorf("user not connected: %s", userID)
    }

    return nil
}

// GetConnectionsByUserID 获取用户的所有连接
func (m *Manager) GetConnectionsByUserID(userID string) []*Conn {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var conns []*Conn
    for _, conn := range m.conns {
        if conn.UserID() == userID {
            conns = append(conns, conn)
        }
    }

    return conns
}

// Stats 返回统计信息
func (m *Manager) Stats() ManagerStats {
    m.mu.RLock()
    defer m.mu.RUnlock()

    stats := ManagerStats{
        TotalConnections: len(m.conns),
    }

    for _, conn := range m.conns {
        connStats := conn.Stats()
        stats.MessagesSent += connStats.MessagesSent
        stats.MessagesReceived += connStats.MessagesReceived
        stats.BytesSent += connStats.BytesSent
        stats.BytesReceived += connStats.BytesReceived
    }

    return stats
}

// ManagerStats 管理器统计
type ManagerStats struct {
    TotalConnections int
    MessagesSent     int64
    MessagesReceived int64
    BytesSent        int64
    BytesReceived    int64
}
```

---

## 3. 消息处理

### 3.1 消息路由

```go
// internal/ws/router.go
package ws

import (
    "context"
    "fmt"
    "sync"
)

// Router 消息路由器
type Router struct {
    routes   map[string]MessageHandler
    mu       sync.RWMutex
    fallback MessageHandler
}

// MessageHandler 消息处理器
type MessageHandler func(ctx context.Context, msg *Message, conn *Conn) error

// NewRouter 创建路由器
func NewRouter() *Router {
    return &Router{
        routes: make(map[string]MessageHandler),
    }
}

// Register 注册路由
func (r *Router) Register(msgType string, handler MessageHandler) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.routes[msgType] = handler
}

// RegisterFunc 注册处理函数
func (r *Router) RegisterFunc(msgType string, handler func(*Message, *Conn) error) {
    r.Register(msgType, func(ctx context.Context, msg *Message, conn *Conn) error {
        return handler(msg, conn)
    })
}

// SetFallback 设置默认处理器
func (r *Router) SetFallback(handler MessageHandler) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.fallback = handler
}

// Route 路由消息
func (r *Router) Route(ctx context.Context, msg *Message, conn *Conn) error {
    r.mu.RLock()
    handler, ok := r.routes[msg.Type]
    r.mu.RUnlock()

    if !ok {
        if r.fallback != nil {
            return r.fallback(ctx, msg, conn)
        }
        return fmt.Errorf("no handler for message type: %s", msg.Type)
    }

    return handler(ctx, msg, conn)
}
```

### 3.2 消息中间件

```go
// internal/ws/middleware.go
package ws

import (
    "context"
    "time"
)

// Middleware 消息中间件
type Middleware func(MessageHandler) MessageHandler

// Chain 中间件链
func Chain(handlers ...Middleware) Middleware {
    return func(handler MessageHandler) MessageHandler {
        for i := len(handlers) - 1; i >= 0; i-- {
            handler = handlers[i](handler)
        }
        return handler
    }
}

// Logging 日志中间件
func Logging(logger Logger) Middleware {
    return func(next MessageHandler) MessageHandler {
        return func(ctx context.Context, msg *Message, conn *Conn) error {
            start := time.Now()

            err := next(ctx, msg, conn)

            duration := time.Since(start)
            logger.Info("WebSocket message",
                "type", msg.Type,
                "conn_id", conn.ID(),
                "user_id", conn.UserID(),
                "duration", duration,
                "error", err,
            )

            return err
        }
    }
}

// Recovery 恢复中间件
func Recovery(logger Logger) Middleware {
    return func(next MessageHandler) MessageHandler {
        return func(ctx context.Context, msg *Message, conn *Conn) (err error) {
            defer func() {
                if r := recover(); r != nil {
                    err = fmt.Errorf("panic: %v", r)
                    logger.Error("WebSocket panic",
                        "type", msg.Type,
                        "conn_id", conn.ID(),
                        "panic", r,
                    )
                }
            }()
            return next(ctx, msg, conn)
        }
    }
}

// RateLimit 限流中间件
func RateLimit(limiter RateLimiter) Middleware {
    return func(next MessageHandler) MessageHandler {
        return func(ctx context.Context, msg *Message, conn *Conn) error {
            if !limiter.Allow(conn.ID()) {
                return fmt.Errorf("rate limit exceeded")
            }
            return next(ctx, msg, conn)
        }
    }
}

// Authentication 认证中间件
func Authentication(auth AuthProvider) Middleware {
    return func(next MessageHandler) MessageHandler {
        return func(ctx context.Context, msg *Message, conn *Conn) error {
            if conn.UserID() == "" {
                // 尝试从消息中获取token
                token, ok := msg.Payload["token"].(string)
                if !ok {
                    return fmt.Errorf("authentication required")
                }

                userID, err := auth.Validate(token)
                if err != nil {
                    return fmt.Errorf("authentication failed: %w", err)
                }

                conn.SetUserID(userID)
            }
            return next(ctx, msg, conn)
        }
    }
}

// RateLimiter 限流器接口
type RateLimiter interface {
    Allow(key string) bool
}

// AuthProvider 认证提供者接口
type AuthProvider interface {
    Validate(token string) (string, error)
}
```

---

## 4. 房间管理

### 4.1 房间实现

```go
// internal/ws/room.go
package ws

import (
    "context"
    "encoding/json"
    "sync"
    "time"
)

// Room 房间
type Room struct {
    id        string
    hub       *Hub
    conns     map[string]*Conn
    data      map[string]interface{}
    dataMu    sync.RWMutex
    mu        sync.RWMutex
    closed    bool
    createdAt time.Time
}

// NewRoom 创建房间
func NewRoom(id string, hub *Hub) *Room {
    return &Room{
        id:        id,
        hub:       hub,
        conns:     make(map[string]*Conn),
        data:      make(map[string]interface{}),
        createdAt: time.Now(),
    }
}

// ID 返回房间ID
func (r *Room) ID() string {
    return r.id
}

// Join 加入房间
func (r *Room) Join(conn *Conn) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if r.closed {
        return ErrRoomClosed
    }

    if _, exists := r.conns[conn.id]; exists {
        return nil // 已经在房间中
    }

    r.conns[conn.id] = conn

    // 通知房间成员
    r.broadcastLocked(map[string]interface{}{
        "type":     "member_joined",
        "room_id":  r.id,
        "conn_id":  conn.id,
        "user_id":  conn.UserID(),
    })

    return nil
}

// Leave 离开房间
func (r *Room) Leave(connID string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.conns[connID]; !exists {
        return nil // 不在房间中
    }

    delete(r.conns, connID)

    // 通知房间成员
    r.broadcastLocked(map[string]interface{}{
        "type":    "member_left",
        "room_id": r.id,
        "conn_id": connID,
    })

    // 如果房间为空，自动关闭
    if len(r.conns) == 0 {
        go r.Close()
    }

    return nil
}

// Broadcast 广播消息到房间
func (r *Room) Broadcast(msg interface{}) error {
    r.mu.RLock()
    defer r.mu.RUnlock()

    if r.closed {
        return ErrRoomClosed
    }

    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    for _, conn := range r.conns {
        _ = conn.SendBytes(data)
    }

    return nil
}

// SendTo 发送消息给指定连接
func (r *Room) SendTo(connID string, msg interface{}) error {
    r.mu.RLock()
    defer r.mu.RUnlock()

    conn, ok := r.conns[connID]
    if !ok {
        return ErrConnNotFound
    }

    return conn.SendJSON(msg)
}

// Members 返回房间成员
func (r *Room) Members() []*Conn {
    r.mu.RLock()
    defer r.mu.RUnlock()

    members := make([]*Conn, 0, len(r.conns))
    for _, conn := range r.conns {
        members = append(members, conn)
    }

    return members
}

// MemberCount 返回成员数量
func (r *Room) MemberCount() int {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return len(r.conns)
}

// HasMember 检查是否有指定成员
func (r *Room) HasMember(connID string) bool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    _, exists := r.conns[connID]
    return exists
}

// Set 设置房间数据
func (r *Room) Set(key string, value interface{}) {
    r.dataMu.Lock()
    defer r.dataMu.Unlock()
    r.data[key] = value
}

// Get 获取房间数据
func (r *Room) Get(key string) (interface{}, bool) {
    r.dataMu.RLock()
    defer r.dataMu.RUnlock()
    v, ok := r.data[key]
    return v, ok
}

// Close 关闭房间
func (r *Room) Close() error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if r.closed {
        return nil
    }

    r.closed = true

    // 通知所有成员
    r.broadcastLocked(map[string]interface{}{
        "type":    "room_closed",
        "room_id": r.id,
    })

    // 清空连接
    r.conns = make(map[string]*Conn)

    return nil
}

// IsClosed 检查房间是否关闭
func (r *Room) IsClosed() bool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.closed
}

// Stats 返回统计信息
func (r *Room) Stats() RoomStats {
    r.mu.RLock()
    defer r.mu.RUnlock()

    return RoomStats{
        RoomID:      r.id,
        MemberCount: len(r.conns),
        CreatedAt:   r.createdAt,
        Closed:      r.closed,
    }
}

// broadcastLocked 广播（已加锁）
func (r *Room) broadcastLocked(msg interface{}) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    for _, conn := range r.conns {
        _ = conn.SendBytes(data)
    }

    return nil
}

// RoomStats 房间统计
type RoomStats struct {
    RoomID      string
    MemberCount int
    CreatedAt   time.Time
    Closed      bool
}
```

---

## 5. Hub 实现

### 5.1 Hub 实现

```go
// internal/ws/hub.go
package ws

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
)

// Hub WebSocket中心
type Hub struct {
    // 连接管理
    connections map[string]*Conn
    connsByUser map[string][]*Conn
    connsMu     sync.RWMutex

    // 房间管理
    rooms      map[string]*Room
    roomsMu    sync.RWMutex

    // 频道管理
    channels   map[string]map[string]*Conn
    channelsMu sync.RWMutex

    // 消息路由
    router     *Router

    // 配置
    config     HubConfig

    // 上下文
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup

    // 统计
    stats      HubStats
}

// HubConfig Hub配置
type HubConfig struct {
    MaxConnections int
    MaxRooms       int
    MaxRoomMembers int
    MessageTimeout time.Duration
}

// DefaultHubConfig 默认配置
var DefaultHubConfig = HubConfig{
    MaxConnections: 10000,
    MaxRooms:       1000,
    MaxRoomMembers: 100,
    MessageTimeout: 10 * time.Second,
}

// NewHub 创建Hub
func NewHub(config HubConfig) *Hub {
    ctx, cancel := context.WithCancel(context.Background())

    return &Hub{
        connections: make(map[string]*Conn),
        connsByUser: make(map[string][]*Conn),
        rooms:       make(map[string]*Room),
        channels:    make(map[string]map[string]*Conn),
        router:      NewRouter(),
        config:      config,
        ctx:         ctx,
        cancel:      cancel,
        stats: HubStats{
            StartTime: time.Now(),
        },
    }
}

// Start 启动Hub
func (h *Hub) Start() {
    // 启动清理任务
    h.wg.Add(1)
    go h.cleanupLoop()
}

// Stop 停止Hub
func (h *Hub) Stop() {
    h.cancel()
    h.wg.Wait()

    // 关闭所有连接
    h.connsMu.Lock()
    for _, conn := range h.connections {
        conn.Stop()
    }
    h.connsMu.Unlock()

    // 关闭所有房间
    h.roomsMu.Lock()
    for _, room := range h.rooms {
        room.Close()
    }
    h.roomsMu.Unlock()
}

// Register 注册连接
func (h *Hub) Register(conn *Conn) {
    h.connsMu.Lock()
    defer h.connsMu.Unlock()

    h.connections[conn.id] = conn

    if conn.userID != "" {
        h.connsByUser[conn.userID] = append(h.connsByUser[conn.userID], conn)
    }

    atomic.AddInt64(&h.stats.TotalConnections, 1)
    atomic.AddInt64(&h.stats.ActiveConnections, 1)
}

// Unregister 注销连接
func (h *Hub) Unregister(conn *Conn) {
    h.connsMu.Lock()
    defer h.connsMu.Unlock()

    if _, exists := h.connections[conn.id]; !exists {
        return
    }

    delete(h.connections, conn.id)
    atomic.AddInt64(&h.stats.ActiveConnections, -1)

    // 从用户映射中移除
    if conn.userID != "" {
        conns := h.connsByUser[conn.userID]
        for i, c := range conns {
            if c.id == conn.id {
                h.connsByUser[conn.userID] = append(conns[:i], conns[i+1:]...)
                break
            }
        }
        if len(h.connsByUser[conn.userID]) == 0 {
            delete(h.connsByUser, conn.userID)
        }
    }

    // 从所有房间中移除
    h.roomsMu.Lock()
    for _, room := range h.rooms {
        room.Leave(conn.id)
    }
    h.roomsMu.Unlock()

    // 从所有频道中移除
    h.channelsMu.Lock()
    for _, channel := range h.channels {
        delete(channel, conn.id)
    }
    h.channelsMu.Unlock()
}

// GetConnection 获取连接
func (h *Hub) GetConnection(connID string) (*Conn, bool) {
    h.connsMu.RLock()
    defer h.connsMu.RUnlock()
    conn, ok := h.connections[connID]
    return conn, ok
}

// GetConnectionsByUserID 获取用户的所有连接
func (h *Hub) GetConnectionsByUserID(userID string) []*Conn {
    h.connsMu.RLock()
    defer h.connsMu.RUnlock()

    conns := h.connsByUser[userID]
    if conns == nil {
        return nil
    }

    result := make([]*Conn, len(conns))
    copy(result, conns)
    return result
}

// Broadcast 广播消息
func (h *Hub) Broadcast(msg *Message) {
    h.connsMu.RLock()
    defer h.connsMu.RUnlock()

    for _, conn := range h.connections {
        _ = conn.Send(msg)
    }
}

// BroadcastToRoom 广播消息到房间
func (h *Hub) BroadcastToRoom(roomID string, msg interface{}) error {
    h.roomsMu.RLock()
    room, ok := h.rooms[roomID]
    h.roomsMu.RUnlock()

    if !ok {
        return ErrRoomNotFound
    }

    return room.Broadcast(msg)
}

// SendToUser 发送消息给用户
func (h *Hub) SendToUser(userID string, msg *Message) error {
    conns := h.GetConnectionsByUserID(userID)
    if len(conns) == 0 {
        return ErrUserNotFound
    }

    for _, conn := range conns {
        _ = conn.Send(msg)
    }

    return nil
}

// SendToConn 发送消息给连接
func (h *Hub) SendToConn(connID string, msg *Message) error {
    h.connsMu.RLock()
    conn, ok := h.connections[connID]
    h.connsMu.RUnlock()

    if !ok {
        return ErrConnNotFound
    }

    return conn.Send(msg)
}

// JoinRoom 加入房间
func (h *Hub) JoinRoom(roomID string, conn *Conn) error {
    h.roomsMu.Lock()
    defer h.roomsMu.Unlock()

    room, ok := h.rooms[roomID]
    if !ok {
        if len(h.rooms) >= h.config.MaxRooms {
            return ErrMaxRoomsReached
        }
        room = NewRoom(roomID, h)
        h.rooms[roomID] = room
    }

    return room.Join(conn)
}

// LeaveRoom 离开房间
func (h *Hub) LeaveRoom(roomID string, conn *Conn) error {
    h.roomsMu.RLock()
    room, ok := h.rooms[roomID]
    h.roomsMu.RUnlock()

    if !ok {
        return ErrRoomNotFound
    }

    return room.Leave(conn.id)
}

// GetRoom 获取房间
func (h *Hub) GetRoom(roomID string) (*Room, bool) {
    h.roomsMu.RLock()
    defer h.roomsMu.RUnlock()
    room, ok := h.rooms[roomID]
    return room, ok
}

// CreateRoom 创建房间
func (h *Hub) CreateRoom(roomID string) (*Room, error) {
    h.roomsMu.Lock()
    defer h.roomsMu.Unlock()

    if _, exists := h.rooms[roomID]; exists {
        return nil, ErrRoomExists
    }

    if len(h.rooms) >= h.config.MaxRooms {
        return nil, ErrMaxRoomsReached
    }

    room := NewRoom(roomID, h)
    h.rooms[roomID] = room

    return room, nil
}

// DeleteRoom 删除房间
func (h *Hub) DeleteRoom(roomID string) error {
    h.roomsMu.Lock()
    defer h.roomsMu.Unlock()

    room, ok := h.rooms[roomID]
    if !ok {
        return ErrRoomNotFound
    }

    room.Close()
    delete(h.rooms, roomID)

    return nil
}

// Subscribe 订阅频道
func (h *Hub) Subscribe(conn *Conn, channel string) error {
    h.channelsMu.Lock()
    defer h.channelsMu.Unlock()

    if _, ok := h.channels[channel]; !ok {
        h.channels[channel] = make(map[string]*Conn)
    }

    h.channels[channel][conn.id] = conn
    return nil
}

// Unsubscribe 取消订阅
func (h *Hub) Unsubscribe(conn *Conn, channel string) error {
    h.channelsMu.Lock()
    defer h.channelsMu.Unlock()

    ch, ok := h.channels[channel]
    if !ok {
        return nil
    }

    delete(ch, conn.id)

    if len(ch) == 0 {
        delete(h.channels, channel)
    }

    return nil
}

// Publish 发布消息到频道
func (h *Hub) Publish(channel string, msg *Message) error {
    h.channelsMu.RLock()
    defer h.channelsMu.RUnlock()

    ch, ok := h.channels[channel]
    if !ok {
        return nil // 没有订阅者
    }

    for _, conn := range ch {
        _ = conn.Send(msg)
    }

    return nil
}

// RouteMessage 路由消息
func (h *Hub) RouteMessage(msg *Message, conn *Conn) error {
    return h.router.Route(context.Background(), msg, conn)
}

// RegisterHandler 注册消息处理器
func (h *Hub) RegisterHandler(msgType string, handler MessageHandler) {
    h.router.Register(msgType, handler)
}

// Stats 返回统计信息
func (h *Hub) Stats() HubStats {
    h.connsMu.RLock()
    h.roomsMu.RLock()
    defer h.connsMu.RUnlock()
    defer h.roomsMu.RUnlock()

    return HubStats{
        TotalConnections:  atomic.LoadInt64(&h.stats.TotalConnections),
        ActiveConnections:  atomic.LoadInt64(&h.stats.ActiveConnections),
        TotalRooms:        len(h.rooms),
        TotalChannels:     len(h.channels),
        MessagesSent:       atomic.LoadInt64(&h.stats.MessagesSent),
        MessagesReceived:   atomic.LoadInt64(&h.stats.MessagesReceived),
        StartTime:          h.stats.StartTime,
    }
}

// cleanupLoop 清理循环
func (h *Hub) cleanupLoop() {
    defer h.wg.Done()

    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            h.cleanup()

        case <-h.ctx.Done():
            return
        }
    }
}

// cleanup 清理过期资源
func (h *Hub) cleanup() {
    // 清理空房间
    h.roomsMu.Lock()
    for id, room := range h.rooms {
        if room.MemberCount() == 0 && time.Since(room.createdAt) > time.Hour {
            room.Close()
            delete(h.rooms, id)
        }
    }
    h.roomsMu.Unlock()

    // 清理空频道
    h.channelsMu.Lock()
    for id, channel := range h.channels {
        if len(channel) == 0 {
            delete(h.channels, id)
        }
    }
    h.channelsMu.Unlock()
}

// HubStats Hub统计
type HubStats struct {
    TotalConnections  int64
    ActiveConnections int64
    TotalRooms        int
    TotalChannels     int
    MessagesSent       int64
    MessagesReceived   int64
    StartTime         time.Time
}
```

---

## 6. 中间件

### 6.1 HTTP 中间件

```go
// internal/ws/http_middleware.go
package ws

import (
    "net/http"
    "strings"
)

// CORS CORS中间件
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")

            allowed := false
            for _, allowedOrigin := range allowedOrigins {
                if allowedOrigin == "*" || allowedOrigin == origin {
                    allowed = true
                    break
                }
            }

            if allowed {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
                w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
                w.Header().Set("Access-Control-Allow-Credentials", "true")
            }

            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusOK)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// Auth 认证中间件
func Auth(authProvider AuthProvider) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractToken(r)
            if token == "" {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            userID, err := authProvider.Validate(token)
            if err != nil {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            // 将用户ID存入请求上下文
            r = r.WithContext(context.WithValue(r.Context(), "user_id", userID))

            next.ServeHTTP(w, r)
        })
    }
}

func extractToken(r *http.Request) string {
    // 从查询参数获取
    if token := r.URL.Query().Get("token"); token != "" {
        return token
    }

    // 从Header获取
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return auth[7:]
    }

    return ""
}

// RateLimit 限流中间件
func RateLimit(limiter RateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := r.RemoteAddr

            if !limiter.Allow(key) {
                http.Error(w, "Too many requests", http.StatusTooManyRequests)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 7. 使用示例

### 7.1 HTTP 服务器示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"

    "github.com/your-org/online-game/internal/ws"
)

func main() {
    // 创建Hub
    hub := ws.NewHub(ws.DefaultHubConfig)
    hub.Start()
    defer hub.Stop()

    // 创建管理器
    manager := ws.NewManager(hub)

    // 注册消息处理器
    hub.RegisterHandler("chat", func(ctx context.Context, msg *ws.Message, conn *ws.Conn) error {
        // 广播聊天消息
        hub.Broadcast(&ws.Message{
            Type: "chat",
            Payload: map[string]interface{}{
                "user_id": conn.UserID(),
                "message": msg.Payload["message"],
            },
        })
        return nil
    })

    // 设置路由
    mux := http.NewServeMux()

    // WebSocket端点
    mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := manager.Upgrade(w, r)
        if err != nil {
            log.Printf("Upgrade failed: %v", err)
            return
        }
        log.Printf("New connection: %s", conn.ID())
    })

    // API端点
    mux.HandleFunc("/api/broadcast", func(w http.ResponseWriter, r *http.Request) {
        // 广播消息
        hub.Broadcast(&ws.Message{
            Type: "broadcast",
            Payload: map[string]interface{}{
                "message": "Hello from server!",
            },
        })
        w.WriteHeader(http.StatusOK)
    })

    // 启动服务器
    addr := ":8080"
    log.Printf("Server started on %s", addr)
    log.Fatal(http.ListenAndServe(addr, mux))
}
```

### 7.2 客户端示例

```html
<!DOCTYPE html>
<html>
<head>
    <title>WebSocket Example</title>
</head>
<body>
    <h1>WebSocket Example</h1>
    <div id="messages"></div>
    <input type="text" id="message" placeholder="Enter message">
    <button onclick="sendMessage()">Send</button>

    <script>
        const ws = new WebSocket('ws://localhost:8080/ws');

        ws.onopen = function() {
            console.log('Connected');
            addMessage('System', 'Connected to server');
        };

        ws.onmessage = function(event) {
            const msg = JSON.parse(event.data);
            addMessage('Server', msg);
        };

        ws.onclose = function() {
            console.log('Disconnected');
            addMessage('System', 'Disconnected from server');
        };

        function sendMessage() {
            const input = document.getElementById('message');
            const msg = {
                type: 'chat',
                payload: {
                    message: input.value
                }
            };
            ws.send(JSON.stringify(msg));
            input.value = '';
        }

        function addMessage(sender, msg) {
            const div = document.createElement('div');
            div.textContent = sender + ': ' + JSON.stringify(msg);
            document.getElementById('messages').appendChild(div);
        }
    </script>
</body>
</html>
```
