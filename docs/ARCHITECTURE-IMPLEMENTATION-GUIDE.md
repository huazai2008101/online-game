# 游戏平台架构实现指南

**文档版本:** v1.0
**创建时间:** 2026-03-24
**核心设计:** Actor模型 + 双引擎 + 微服务

---

## 目录

1. [项目结构](#1-项目结构)
2. [核心模块实现](#2-核心模块实现)
3. [服务间通信](#3-服务间通信)
4. [数据库实现](#4-数据库实现)
5. [缓存实现](#5-缓存实现)
6. [消息队列实现](#6-消息队列实现)
7. [API网关实现](#7-api网关实现)
8. [WebSocket网关实现](#8-websocket网关实现)
9. [监控与日志](#9-监控与日志)
10. [部署配置](#10-部署配置)

---

## 1. 项目结构

### 1.1 整体目录结构

```
online-game/
├── cmd/                          # 服务入口
│   ├── gateway/                  # API网关
│   │   └── main.go
│   ├── ws-gateway/               # WebSocket网关
│   │   └── main.go
│   ├── game-service/             # 游戏服务
│   │   └── main.go
│   ├── user-service/             # 用户服务
│   │   └── main.go
│   └── payment-service/          # 支付服务
│       └── main.go
│
├── internal/                     # 内部实现
│   ├── actor/                    # Actor模型
│   │   ├── actor.go
│   │   ├── system.go
│   │   ├── mailbox.go
│   │   ├── supervisor.go
│   │   └── persistence.go
│   │
│   ├── engine/                   # 游戏引擎
│   │   ├── interface.go
│   │   ├── js_engine.go
│   │   ├── wasm_engine.go
│   │   ├── dual_engine.go
│   │   └── selector.go
│   │
│   ├── game/                     # 游戏服务实现
│   │   ├── handler/              # HTTP处理器
│   │   ├── service/              # 业务逻辑
│   │   ├── repository/           # 数据访问
│   │   └── model/                # 数据模型
│   │
│   ├── user/                     # 用户服务实现
│   │   ├── handler/
│   │   ├── service/
│   │   ├── repository/
│   │   └── model/
│   │
│   ├── payment/                  # 支付服务实现
│   │   ├── handler/
│   │   ├── service/
│   │   ├── repository/
│   │   └── model/
│   │
│   ├── ws/                       # WebSocket实现
│   │   ├── connection.go
│   │   ├── hub.go
│   │   ├── message.go
│   │   └── broadcast.go
│   │
│   └── middleware/               # 中间件
│       ├── auth.go
│       ├── recovery.go
│       ├── request_id.go
│       └── logging.go
│
├── pkg/                          # 公共包
│   ├── apperror/                 # 错误处理
│   │   └── error.go
│   ├── response/                 # 响应处理
│   │   └── response.go
│   ├── config/                   # 配置管理
│   │   └── config.go
│   ├── database/                 # 数据库
│   │   ├── postgres.go
│   │   ├── redis.go
│   │   └── migrate.go
│   ├── cache/                    # 缓存
│   │   └── redis.go
│   ├── queue/                    # 消息队列
│   │   └── kafka.go
│   ├── logger/                   # 日志
│   │   └── logger.go
│   ├── metrics/                  # 指标
│   │   └── metrics.go
│   ├── trace/                    # 链路追踪
│   │   └── trace.go
│   └── grpc/                     # gRPC客户端
│       └── client.go
│
├── api/                          # API定义
│   ├── proto/                    # Protobuf定义
│   │   ├── game.proto
│   │   ├── user.proto
│   │   └── payment.proto
│   └── openapi/                  # OpenAPI规范
│       └── openapi.yaml
│
├── web/                          # 前端资源
│   ├── sdk/                      # 客户端SDK
│   │   └── game-sdk.js
│   └── admin/                    # 管理后台
│
├── scripts/                      # 脚本
│   ├── migrate.sh
│   ├── deploy.sh
│   └── test.sh
│
├── deployments/                  # 部署配置
│   ├── docker/
│   │   ├── Dockerfile.gateway
│   │   ├── Dockerfile.game
│   │   └── docker-compose.yml
│   └── k8s/                      # Kubernetes配置
│       ├── gateway.yaml
│       ├── game-service.yaml
│       └── configmap.yaml
│
├── configs/                      # 配置文件
│   ├── config.dev.yaml
│   ├── config.prod.yaml
│   └── config.test.yaml
│
├── docs/                         # 文档
│   ├── ERROR-HANDLING-GUIDE.md
│   └── API-REFERENCE.md
│
├── tests/                        # 测试
│   ├── unit/
│   ├── integration/
│   └── e2e/
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 1.2 服务端口分配

| 服务 | 端口 | 协议 |
|------|------|------|
| API Gateway | 8080 | HTTP |
| WebSocket Gateway | 8081 | WebSocket |
| Game Service | 8002 | gRPC |
| User Service | 8001 | gRPC |
| Payment Service | 8003 | gRPC |
| Player Service | 8004 | gRPC |
| Notification Service | 8008 | gRPC |

---

## 2. 核心模块实现

### 2.1 Actor模型

#### 2.1.1 Actor接口定义

```go
// internal/actor/actor.go
package actor

import (
	"context"
	"time"
)

// Message 消息接口
type Message interface {
	// Type 返回消息类型
	Type() string
}

// Actor Actor接口
type Actor interface {
	// ID 返回Actor ID
	ID() string

	// Type 返回Actor类型
	Type() string

	// Start 启动Actor
	Start(ctx context.Context) error

	// Stop 停止Actor
	Stop() error

	// Send 发送消息
	Send(msg Message) error

	// Receive 接收并处理消息
	Receive(ctx context.Context, msg Message) error
}

// ActorStats Actor统计信息
type ActorStats struct {
	MessageCount    int64         // 消息总数
	ProcessTime     time.Duration // 总处理时间
	ErrorCount      int64         // 错误数
	LastMessageTime time.Time     // 最后消息时间
	InboxSize       int           // 邮箱大小
}
```

#### 2.1.2 BaseActor实现

```go
// internal/actor/base_actor.go
package actor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BaseActor Actor基础实现
type BaseActor struct {
	id        string
	actorType string
	inbox     chan Message
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	stats     ActorStats
	handler   MessageHandler
}

// MessageHandler 消息处理器
type MessageHandler interface {
	Receive(ctx context.Context, msg Message) error
}

// NewBaseActor 创建基础Actor
func NewBaseActor(id, actorType string, inboxSize int, handler MessageHandler) *BaseActor {
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

// Start 启动Actor
func (a *BaseActor) Start(ctx context.Context) error {
	a.wg.Add(1)
	go a.run()
	return nil
}

// run Actor主循环
func (a *BaseActor) run() {
	defer a.wg.Done()

	for {
		select {
		case msg, ok := <-a.inbox:
			if !ok {
				return
			}
			a.processMessage(msg)

		case <-a.ctx.Done():
			return
		}
	}
}

// processMessage 处理消息
func (a *BaseActor) processMessage(msg Message) {
	start := time.Now()

	err := a.handler.Receive(a.ctx, msg)

	a.mu.Lock()
	a.stats.MessageCount++
	a.stats.ProcessTime += time.Since(start)
	a.stats.LastMessageTime = time.Now()
	if err != nil {
		a.stats.ErrorCount++
	}
	a.mu.Unlock()
}

// Send 发送消息
func (a *BaseActor) Send(msg Message) error {
	select {
	case a.inbox <- msg:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("actor %s inbox full", a.id)
	}
}

// Stop 停止Actor
func (a *BaseActor) Stop() error {
	a.cancel()
	a.wg.Wait()
	close(a.inbox)
	return nil
}

// ID 返回Actor ID
func (a *BaseActor) ID() string {
	return a.id
}

// Type 返回Actor类型
func (a *BaseActor) Type() string {
	return a.actorType
}

// Stats 返回统计信息
func (a *BaseActor) Stats() ActorStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 复制stats避免并发问题
	stats := a.stats
	stats.InboxSize = len(a.inbox)
	return stats
}
```

#### 2.1.3 GameActor实现

```go
// internal/actor/game_actor.go
package actor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// GameActor 游戏Actor
type GameActor struct {
	*BaseActor
	gameID    string
	roomID    string
	state     *GameState
	players   map[string]*PlayerActor
	stateLock sync.RWMutex
	engine    GameEngine
}

// GameState 游戏状态
type GameState struct {
	GameID     string                 `json:"game_id"`
	RoomID     string                 `json:"room_id"`
	Status     string                 `json:"status"` // waiting, playing, ended
	Config     map[string]interface{} `json:"config"`
	Data       map[string]interface{} `json:"data"`
	Tick       int64                  `json:"tick"`
	MaxPlayers int                    `json:"max_players"`
	CreatedAt  int64                  `json:"created_at"`
	UpdatedAt  int64                  `json:"updated_at"`
}

// GameEngine 游戏引擎接口
type GameEngine interface {
	Init(gameID string, state *GameState) error
	Execute(method string, args interface{}) (interface{}, error)
	GetState() (*GameState, error)
	Cleanup() error
}

// NewGameActor 创建游戏Actor
func NewGameActor(gameID, roomID string, engine GameEngine) *GameActor {
	actor := &GameActor{
		gameID:  gameID,
		roomID:  roomID,
		state: &GameState{
			GameID:     gameID,
			RoomID:     roomID,
			Status:     "waiting",
			Config:     make(map[string]interface{}),
			Data:       make(map[string]interface{}),
			MaxPlayers: 4,
		},
		players: make(map[string]*PlayerActor),
		engine:  engine,
	}

	actor.BaseActor = NewBaseActor(
		fmt.Sprintf("game:%s:%s", gameID, roomID),
		"game",
		1000,
		actor,
	)

	return actor
}

// Receive 处理消息
func (a *GameActor) Receive(ctx context.Context, msg Message) error {
	switch m := msg.(type) {
	case *GameStartMessage:
		return a.handleGameStart(ctx, m)
	case *GameTickMessage:
		return a.handleGameTick(ctx, m)
	case *GameStopMessage:
		return a.handleGameStop(ctx, m)
	case *PlayerJoinMessage:
		return a.handlePlayerJoin(ctx, m)
	case *PlayerLeaveMessage:
		return a.handlePlayerLeave(ctx, m)
	case *PlayerActionMessage:
		return a.handlePlayerAction(ctx, m)
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

// GameStartMessage 游戏开始消息
type GameStartMessage struct {
	Timestamp int64
}

// handleGameStart 处理游戏开始
func (a *GameActor) handleGameStart(ctx context.Context, msg *GameStartMessage) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	if a.state.Status != "waiting" {
		return fmt.Errorf("game not in waiting status")
	}

	if len(a.players) < 2 {
		return fmt.Errorf("not enough players")
	}

	// 初始化引擎
	if err := a.engine.Init(a.gameID, a.state); err != nil {
		return err
	}

	// 执行游戏开始逻辑
	result, err := a.engine.Execute("onStart", nil)
	if err != nil {
		return err
	}

	// 更新状态
	a.state.Status = "playing"
	if data, ok := result.(map[string]interface{}); ok {
		for k, v := range data {
			a.state.Data[k] = v
		}
	}

	return nil
}

// GameTickMessage 游戏tick消息
type GameTickMessage struct {
	Timestamp int64
	TickCount int64
}

// handleGameTick 处理游戏tick
func (a *GameActor) handleGameTick(ctx context.Context, msg *GameTickMessage) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	if a.state.Status != "playing" {
		return nil
	}

	a.state.Tick = msg.TickCount

	// 执行tick逻辑
	result, err := a.engine.Execute("onTick", map[string]interface{}{
		"tick": msg.TickCount,
	})
	if err != nil {
		return err
	}

	// 更新状态
	if data, ok := result.(map[string]interface{}); ok {
		for k, v := range data {
			a.state.Data[k] = v
		}
	}

	return nil
}

// GameStopMessage 游戏停止消息
type GameStopMessage struct {
	Reason string
}

// handleGameStop 处理游戏停止
func (a *GameActor) handleGameStop(ctx context.Context, msg *GameStopMessage) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	a.state.Status = "ended"

	// 清理引擎
	return a.engine.Cleanup()
}

// PlayerJoinMessage 玩家加入消息
type PlayerJoinMessage struct {
	PlayerID   string
	PlayerName string
	PlayerData map[string]interface{}
}

// handlePlayerJoin 处理玩家加入
func (a *GameActor) handlePlayerJoin(ctx context.Context, msg *PlayerJoinMessage) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	if a.state.Status != "waiting" {
		return fmt.Errorf("game not accepting players")
	}

	if len(a.players) >= a.state.MaxPlayers {
		return fmt.Errorf("game is full")
	}

	if _, exists := a.players[msg.PlayerID]; exists {
		return fmt.Errorf("player already in game")
	}

	// 创建玩家Actor
	playerActor := NewPlayerActor(msg.PlayerID, a.gameID, a.roomID)
	a.players[msg.PlayerID] = playerActor

	return nil
}

// PlayerLeaveMessage 玩家离开消息
type PlayerLeaveMessage struct {
	PlayerID string
	Reason   string
}

// handlePlayerLeave 处理玩家离开
func (a *GameActor) handlePlayerLeave(ctx context.Context, msg *PlayerLeaveMessage) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	if player, exists := a.players[msg.PlayerID]; exists {
		player.Stop()
		delete(a.players, msg.PlayerID)
	}

	// 如果没有玩家了，结束游戏
	if len(a.players) == 0 {
		a.state.Status = "ended"
		return a.engine.Cleanup()
	}

	return nil
}

// PlayerActionMessage 玩家操作消息
type PlayerActionMessage struct {
	PlayerID string
	Action   string
	Data     interface{}
}

// handlePlayerAction 处理玩家操作
func (a *GameActor) handlePlayerAction(ctx context.Context, msg *PlayerActionMessage) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	if a.state.Status != "playing" {
		return fmt.Errorf("game not in playing status")
	}

	if _, exists := a.players[msg.PlayerID]; !exists {
		return fmt.Errorf("player not in game")
	}

	// 执行玩家操作
	result, err := a.engine.Execute("onAction", map[string]interface{}{
		"player_id": msg.PlayerID,
		"action":    msg.Action,
		"data":      msg.Data,
	})
	if err != nil {
		return err
	}

	// 更新状态
	if data, ok := result.(map[string]interface{}); ok {
		for k, v := range data {
			a.state.Data[k] = v
		}
	}

	return nil
}

// GetState 获取游戏状态
func (a *GameActor) GetState() *GameState {
	a.stateLock.RLock()
	defer a.stateLock.RUnlock()

	// 深拷贝状态
	stateBytes, _ := json.Marshal(a.state)
	var state GameState
	json.Unmarshal(stateBytes, &state)
	return &state
}
```

#### 2.1.4 ActorSystem实现

```go
// internal/actor/system.go
package actor

import (
	"context"
	"fmt"
	"sync"
)

// ActorSystem Actor系统
type ActorSystem struct {
	actors   map[string]Actor
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	supervisor *Supervisor
}

// NewActorSystem 创建Actor系统
func NewActorSystem() *ActorSystem {
	ctx, cancel := context.WithCancel(context.Background())
	return &ActorSystem{
		actors: make(map[string]Actor),
		ctx:    ctx,
		cancel: cancel,
		supervisor: NewSupervisor(),
	}
}

// Start 启动Actor系统
func (s *ActorSystem) Start() error {
	return s.supervisor.Start(s.ctx)
}

// Stop 停止Actor系统
func (s *ActorSystem) Stop() error {
	s.cancel()

	s.mu.RLock()
	defer s.mu.RUnlock()

	var wg sync.WaitGroup
	for _, actor := range s.actors {
		wg.Add(1)
		go func(a Actor) {
			defer wg.Done()
			a.Stop()
		}(actor)
	}
	wg.Wait()

	return s.supervisor.Stop()
}

// RegisterActor 注册Actor
func (s *ActorSystem) RegisterActor(actor Actor) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	actorID := actor.ID()
	if _, exists := s.actors[actorID]; exists {
		return fmt.Errorf("actor %s already exists", actorID)
	}

	s.actors[actorID] = actor
	s.supervisor.Watch(actor)

	return actor.Start(s.ctx)
}

// UnregisterActor 注销Actor
func (s *ActorSystem) UnregisterActor(actorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	actor, exists := s.actors[actorID]
	if !exists {
		return fmt.Errorf("actor %s not found", actorID)
	}

	s.supervisor.Unwatch(actor)
	actor.Stop()
	delete(s.actors, actorID)

	return nil
}

// GetActor 获取Actor
func (s *ActorSystem) GetActor(actorID string) (Actor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	actor, exists := s.actors[actorID]
	if !exists {
		return nil, fmt.Errorf("actor %s not found", actorID)
	}

	return actor, nil
}

// Send 发送消息给Actor
func (s *ActorSystem) Send(actorID string, msg Message) error {
	actor, err := s.GetActor(actorID)
	if err != nil {
		return err
	}

	return actor.Send(msg)
}

// GetStats 获取系统统计信息
func (s *ActorSystem) GetStats() map[string]ActorStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]ActorStats)
	for id, actor := range s.actors {
		if baseActor, ok := actor.(*BaseActor); ok {
			stats[id] = baseActor.Stats()
		}
	}
	return stats
}
```

#### 2.1.5 Supervisor实现

```go
// internal/actor/supervisor.go
package actor

import (
	"context"
	"log"
	"sync"
	"time"
)

// Supervisor 监督者
type Supervisor struct {
	actors      map[string]Actor
	strategies  map[string]RestartStrategy
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	restartChan chan string
}

// RestartStrategy 重启策略
type RestartStrategy struct {
	MaxRestarts int
	Backoff     time.Duration
	current     int
}

// NewSupervisor 创建监督者
func NewSupervisor() *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{
		actors:      make(map[string]Actor),
		strategies:  make(map[string]RestartStrategy),
		ctx:         ctx,
		cancel:      cancel,
		restartChan: make(chan string, 100),
	}
}

// Start 启动监督者
func (s *Supervisor) Start(ctx context.Context) error {
	go s.monitor()
	return nil
}

// Stop 停止监督者
func (s *Supervisor) Stop() error {
	s.cancel()
	return nil
}

// Watch 监督Actor
func (s *Supervisor) Watch(actor Actor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.actors[actor.ID()] = actor
	s.strategies[actor.ID()] = RestartStrategy{
		MaxRestarts: 3,
		Backoff:     5 * time.Second,
	}
}

// Unwatch 取消监督
func (s *Supervisor) Unwatch(actor Actor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.actors, actor.ID())
	delete(s.strategies, actor.ID())
}

// monitor 监控Actor
func (s *Supervisor) monitor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case actorID := <-s.restartChan:
			s.handleRestart(actorID)
		case <-ticker.C:
			s.healthCheck()
		}
	}
}

// handleRestart 处理重启
func (s *Supervisor) handleRestart(actorID string) {
	s.mu.Lock()
	strategy, exists := s.strategies[actorID]
	if !exists {
		s.mu.Unlock()
		return
	}

	if strategy.current >= strategy.MaxRestarts {
		s.mu.Unlock()
		log.Printf("[Supervisor] Actor %s exceeded max restarts", actorID)
		return
	}

	strategy.current++
	s.strategies[actorID] = strategy
	s.mu.Unlock()

	log.Printf("[Supervisor] Restarting actor %s (attempt %d)", actorID, strategy.current)
	time.Sleep(strategy.Backoff)

	// TODO: 实际重启逻辑
}

// healthCheck 健康检查
func (s *Supervisor) healthCheck() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, actor := range s.actors {
		// TODO: 检查Actor健康状态
		_ = actor
	}
}
```

---

### 2.2 双引擎实现

#### 2.2.1 引擎接口

```go
// internal/engine/interface.go
package engine

import (
	"context"
)

// EngineType 引擎类型
type EngineType int

const (
	EngineJS EngineType = iota
	EngineWASM
)

// GameEngine 游戏引擎接口
type GameEngine interface {
	// Init 初始化引擎
	Init(ctx context.Context, gameID string, config map[string]interface{}) error

	// LoadScript 加载游戏脚本
	LoadScript(scriptType EngineType, script []byte) error

	// Execute 执行游戏逻辑
	Execute(ctx context.Context, method string, args interface{}) (interface{}, error)

	// GetState 获取游戏状态
	GetState(ctx context.Context) (interface{}, error)

	// Cleanup 清理资源
	Cleanup(ctx context.Context) error

	// Type 获取引擎类型
	Type() EngineType

	// Stats 获取性能统计
	Stats() EngineStats
}

// EngineStats 引擎统计
type EngineStats struct {
	Type           EngineType
	ExecuteCount   int64
	ExecuteTime    int64  // 纳秒
	ErrorCount     int64
	CacheHitCount  int64
	CacheMissCount int64
	MemoryUsage    int64
}
```

#### 2.2.2 JavaScript引擎实现

```go
// internal/engine/js_engine.go
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/robertkrimen/otto"
)

// JSEngine JavaScript引擎
type JSEngine struct {
	vm         *otto.Otto
	vmPool     sync.Pool
	cache      sync.Map
	stats      EngineStats
	cacheLock  sync.RWMutex
	gameID     string
	config     map[string]interface{}
}

// cachedScript 缓存的脚本
type cachedScript struct {
	script    *otto.Script
	timestamp time.Time
}

// NewJSEngine 创建JavaScript引擎
func NewJSEngine() *JSEngine {
	return &JSEngine{
		vm: otto.New(),
		stats: EngineStats{
			Type: EngineJS,
		},
		config: make(map[string]interface{}),
		vmPool: sync.Pool{
			New: func() interface{} {
				return otto.New()
			},
		},
	}
}

// Init 初始化引擎
func (e *JSEngine) Init(ctx context.Context, gameID string, config map[string]interface{}) error {
	e.gameID = gameID
	e.config = config

	// 设置全局变量
	e.vm.Set("gameID", gameID)
	e.vm.Set("config", config)
	e.vm.Set("state", make(map[string]interface{}))

	// 注册内置函数
	e.registerBuiltins()

	return nil
}

// registerBuiltins 注册内置函数
func (e *JSEngine) registerBuiltins() {
	// 日志函数
	e.vm.Set("log", func(call otto.FunctionCall) otto.Value {
		args := call.ArgumentList
		for _, arg := range args {
			fmt.Printf("[JS] %v ", arg)
		}
		fmt.Println()
		return otto.TrueValue()
	})

	// 广播函数
	e.vm.Set("broadcast", func(call otto.FunctionCall) otto.Value {
		// TODO: 实现广播逻辑
		return otto.TrueValue()
	})
}

// LoadScript 加载脚本
func (e *JSEngine) LoadScript(scriptType EngineType, script []byte) error {
	if scriptType != EngineJS {
		return fmt.Errorf("invalid script type: %v", scriptType)
	}

	// 编译脚本
	compiled, err := e.vm.Compile("", script)
	if err != nil {
		return fmt.Errorf("compile script failed: %w", err)
	}

	// 执行初始化
	_, err = e.vm.Run(compiled)
	if err != nil {
		return fmt.Errorf("run script failed: %w", err)
	}

	return nil
}

// Execute 执行方法
func (e *JSEngine) Execute(ctx context.Context, method string, args interface{}) (interface{}, error) {
	start := time.Now()

	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%v", method, args)
	if cached, ok := e.cache.Load(cacheKey); ok {
		e.stats.CacheHitCount++
		e.stats.ExecuteCount++
		e.stats.ExecuteTime += time.Since(start).Nanoseconds()
		return cached, nil
	}
	e.stats.CacheMissCount++

	// 准备参数
	argsJSON, _ := json.Marshal(args)

	// 构造执行代码
	code := fmt.Sprintf("(%s)(%s)", method, string(argsJSON))

	// 执行
	value, err := e.vm.Run(code)
	if err != nil {
		e.stats.ExecuteCount++
		e.stats.ErrorCount++
		e.stats.ExecuteTime += time.Since(start).Nanoseconds()
		return nil, fmt.Errorf("execute failed: %w", err)
	}

	// 转换结果
	result, err := value.Export()
	if err != nil {
		e.stats.ExecuteCount++
		e.stats.ErrorCount++
		e.stats.ExecuteTime += time.Since(start).Nanoseconds()
		return nil, fmt.Errorf("export result failed: %w", err)
	}

	// 缓存结果
	e.cache.Store(cacheKey, result)

	e.stats.ExecuteCount++
	e.stats.ExecuteTime += time.Since(start).Nanoseconds()

	return result, nil
}

// GetState 获取状态
func (e *JSEngine) GetState(ctx context.Context) (interface{}, error) {
	value, err := e.vm.Get("state")
	if err != nil {
		return nil, err
	}
	return value.Export()
}

// Cleanup 清理
func (e *JSEngine) Cleanup(ctx context.Context) error {
	e.cache = sync.Map{}
	e.vm = otto.New()
	return nil
}

// Type 返回引擎类型
func (e *JSEngine) Type() EngineType {
	return EngineJS
}

// Stats 返回统计信息
func (e *JSEngine) Stats() EngineStats {
	return e.stats
}
```

#### 2.2.3 双引擎管理器

```go
// internal/engine/dual_engine.go
package engine

import (
	"context"
	"sync"
	"time"
)

// DualEngine 双引擎管理器
type DualEngine struct {
	jsEngine    GameEngine
	wasmEngine  GameEngine
	current     GameEngine
	selector    *EngineSelector
	mu          sync.RWMutex
	perfData    *PerformanceData
	gameID      string
	config      map[string]interface{}
}

// PerformanceData 性能数据
type PerformanceData struct {
	AvgLatency  time.Duration
	AvgFPS      float64
	ErrorRate   float64
	MemoryUsage int64
	samples     []time.Duration
}

// NewDualEngine 创建双引擎
func NewDualEngine(gameID string) (*DualEngine, error) {
	jsEngine := NewJSEngine()

	// WASM引擎可选，暂时不创建
	var wasmEngine GameEngine = nil

	return &DualEngine{
		jsEngine:   jsEngine,
		wasmEngine: wasmEngine,
		current:    jsEngine,
		selector:   NewEngineSelector(),
		perfData:   &PerformanceData{},
		gameID:     gameID,
		config:     make(map[string]interface{}),
	}, nil
}

// Init 初始化
func (d *DualEngine) Init(ctx context.Context, gameID string, config map[string]interface{}) error {
	d.gameID = gameID
	d.config = config

	// 初始化当前引擎
	return d.current.Init(ctx, gameID, config)
}

// LoadScript 加载脚本
func (d *DualEngine) LoadScript(scriptType EngineType, script []byte) error {
	switch scriptType {
	case EngineJS:
		return d.jsEngine.LoadScript(scriptType, script)
	case EngineWASM:
		if d.wasmEngine == nil {
			return fmt.Errorf("WASM engine not available")
		}
		return d.wasmEngine.LoadScript(scriptType, script)
	default:
		return fmt.Errorf("unknown script type: %v", scriptType)
	}
}

// Execute 执行
func (d *DualEngine) Execute(ctx context.Context, method string, args interface{}) (interface{}, error) {
	start := time.Now()

	result, err := d.current.Execute(ctx, method, args)

	// 记录性能数据
	latency := time.Since(start)
	d.recordPerformance(latency, err != nil)

	// 检查是否需要切换引擎
	d.checkAndSwitchEngine()

	return result, err
}

// GetState 获取状态
func (d *DualEngine) GetState(ctx context.Context) (interface{}, error) {
	return d.current.GetState(ctx)
}

// Cleanup 清理
func (d *DualEngine) Cleanup(ctx context.Context) error {
	return d.current.Cleanup(ctx)
}

// Type 返回当前引擎类型
func (d *DualEngine) Type() EngineType {
	return d.current.Type()
}

// Stats 返回统计信息
func (d *DualEngine) Stats() EngineStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	jsStats := d.jsEngine.Stats()
	wasmStats := EngineStats{}
	if d.wasmEngine != nil {
		wasmStats = d.wasmEngine.Stats()
	}

	return EngineStats{
		Type:           d.current.Type(),
		ExecuteCount:   jsStats.ExecuteCount + wasmStats.ExecuteCount,
		ExecuteTime:    jsStats.ExecuteTime + wasmStats.ExecuteTime,
		ErrorCount:     jsStats.ErrorCount + wasmStats.ErrorCount,
		CacheHitCount:  jsStats.CacheHitCount + wasmStats.CacheHitCount,
		CacheMissCount: jsStats.CacheMissCount + wasmStats.CacheMissCount,
	}
}

// recordPerformance 记录性能
func (d *DualEngine) recordPerformance(latency time.Duration, isError bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.perfData.samples = append(d.perfData.samples, latency)

	// 只保留最近100个样本
	if len(d.perfData.samples) > 100 {
		d.perfData.samples = d.perfData.samples[1:]
	}

	// 计算平均延迟
	var total time.Duration
	for _, s := range d.perfData.samples {
		total += s
	}
	d.perfData.AvgLatency = total / time.Duration(len(d.perfData.samples))

	// 计算FPS
	if d.perfData.AvgLatency > 0 {
		d.perfData.AvgFPS = float64(time.Second) / float64(d.perfData.AvgLatency)
	}
}

// checkAndSwitchEngine 检查并切换引擎
func (d *DualEngine) checkAndSwitchEngine() {
	if d.wasmEngine == nil {
		return
	}

	recommended := d.selector.Select(d.perfData)

	if recommended != d.current.Type() {
		d.switchEngine(recommended)
	}
}

// switchEngine 切换引擎
func (d *DualEngine) switchEngine(engineType EngineType) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if engineType == EngineWASM && d.wasmEngine != nil {
		d.current = d.wasmEngine
	} else {
		d.current = d.jsEngine
	}
}
```

#### 2.2.4 引擎选择器

```go
// internal/engine/selector.go
package engine

import (
	"time"
)

// EngineSelector 引擎选择器
type EngineSelector struct {
	latencyThreshold time.Duration
	fpsThreshold     float64
	errorThreshold   float64
}

// NewEngineSelector 创建引擎选择器
func NewEngineSelector() *EngineSelector {
	return &EngineSelector{
		latencyThreshold: 100 * time.Millisecond,
		fpsThreshold:     30,
		errorThreshold:   0.05,
	}
}

// Select 选择引擎
func (s *EngineSelector) Select(perf *PerformanceData) EngineType {
	// 性能良好，保持当前
	if perf.AvgLatency < s.latencyThreshold && perf.AvgFPS >= s.fpsThreshold {
		return EngineJS
	}

	// 性能不足，考虑WASM
	if perf.AvgLatency > s.latencyThreshold*2 || perf.AvgFPS < s.fpsThreshold {
		return EngineWASM
	}

	// 默认JavaScript
	return EngineJS
}
```

---

## 3. 服务间通信

### 3.1 gRPC服务定义

```protobuf
// api/proto/game.proto
syntax = "proto3";

package game.v1;

option go_package = "your-project/api/proto/game/v1;gamev1";

import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

// 游戏服务
service GameService {
  // 创建游戏
  rpc CreateGame(CreateGameRequest) returns (CreateGameResponse);

  // 获取游戏
  rpc GetGame(GetGameRequest) returns (GetGameResponse);

  // 列出游戏
  rpc ListGames(ListGamesRequest) returns (ListGamesResponse);

  // 更新游戏
  rpc UpdateGame(UpdateGameRequest) returns (UpdateGameResponse);

  // 删除游戏
  rpc DeleteGame(DeleteGameRequest) returns (google.protobuf.Empty);

  // 创建房间
  rpc CreateRoom(CreateRoomRequest) returns (CreateRoomResponse);

  // 加入房间
  rpc JoinRoom(JoinRoomRequest) returns (JoinRoomResponse);

  // 离开房间
  rpc LeaveRoom(LeaveRoomRequest) returns (google.protobuf.Empty);

  // 开始游戏
  rpc StartGame(StartGameRequest) returns (StartGameResponse);

  // 获取房间状态
  rpc GetRoomState(GetRoomStateRequest) returns (GetRoomStateResponse);
}

// 游戏信息
message Game {
  int64 id = 1;
  int64 org_id = 2;
  string game_code = 3;
  string game_name = 4;
  string game_type = 5;
  string game_icon = 6;
  string game_cover = 7;
  map<string, string> game_config = 8;
  int32 status = 9;
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
}

// 创建游戏请求
message CreateGameRequest {
  int64 org_id = 1;
  string game_code = 2;
  string game_name = 3;
  string game_type = 4;
  string description = 5;
  map<string, string> config = 6;
}

// 创建游戏响应
message CreateGameResponse {
  Game game = 1;
}

// 获取游戏请求
message GetGameRequest {
  int64 id = 1;
}

// 获取游戏响应
message GetGameResponse {
  Game game = 1;
}

// 列出游戏请求
message ListGamesRequest {
  int64 org_id = 1;
  int32 page = 2;
  int32 page_size = 3;
  string status = 4;
}

// 列出游戏响应
message ListGamesResponse {
  repeated Game games = 1;
  int32 total = 2;
}

// 更新游戏请求
message UpdateGameRequest {
  int64 id = 1;
  string game_name = 2;
  string description = 3;
  map<string, string> config = 4;
}

// 更新游戏响应
message UpdateGameResponse {
  Game game = 1;
}

// 删除游戏请求
message DeleteGameRequest {
  int64 id = 1;
}

// 创建房间请求
message CreateRoomRequest {
  int64 game_id = 1;
  string room_name = 2;
  int32 max_players = 3;
  map<string, string> config = 4;
}

// 创建房间响应
message CreateRoomResponse {
  string room_id = 1;
}

// 加入房间请求
message JoinRoomRequest {
  string room_id = 1;
  int64 player_id = 2;
  string player_name = 3;
}

// 加入房间响应
message JoinRoomResponse {
  bool success = 1;
  string message = 2;
}

// 离开房间请求
message LeaveRoomRequest {
  string room_id = 1;
  int64 player_id = 2;
}

// 开始游戏请求
message StartGameRequest {
  string room_id = 1;
}

// 开始游戏响应
message StartGameResponse {
  bool success = 1;
  string message = 2;
}

// 获取房间状态请求
message GetRoomStateRequest {
  string room_id = 1;
}

// 获取房间状态响应
message GetRoomStateResponse {
  string room_id = 1;
  string status = 2;
  repeated Player players = 3;
  map<string, string> state = 4;
}

// 玩家信息
message Player {
  int64 id = 1;
  string nickname = 2;
  string avatar = 3;
  int32 status = 4;
}
```

### 3.2 gRPC客户端封装

```go
// pkg/grpc/client.go
package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientConfig 客户端配置
type ClientConfig struct {
	ServiceName string
	Host        string
	Port        int
	Timeout     time.Duration
}

// Client gRPC客户端
type Client struct {
	conn   *grpc.ClientConn
	config ClientConfig
}

// NewClient 创建客户端
func NewClient(config ClientConfig) (*Client, error) {
	target := fmt.Sprintf("%s:%d", config.Host, config.Port)

	conn, err := grpc.Dial(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}

	return &Client{
		conn:   conn,
		config: config,
	}, nil
}

// Close 关闭连接
func (c *Client) Close() error {
	return c.conn.Close()
}

// Context 创建上下文
func (c *Client) Context() (context.Context, context.CancelFunc) {
	if c.config.Timeout > 0 {
		return context.WithTimeout(context.Background(), c.config.Timeout)
	}
	return context.WithCancel(context.Background())
}

// Conn 获取连接
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

// ClientManager 客户端管理器
type ClientManager struct {
	clients map[string]*Client
	mu      sync.RWMutex
}

// NewClientManager 创建客户端管理器
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Client),
	}
}

// GetClient 获取客户端
func (m *ClientManager) GetClient(serviceName string) (*Client, error) {
	m.mu.RLock()
	client, exists := m.clients[serviceName]
	m.mu.RUnlock()

	if exists {
		return client, nil
	}

	// TODO: 从服务发现获取地址
	config := ClientConfig{
		ServiceName: serviceName,
		Host:        "localhost",
		Port:        8001,
		Timeout:     5 * time.Second,
	}

	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.clients[serviceName] = client
	m.mu.Unlock()

	return client, nil
}

// Close 关闭所有客户端
func (m *ClientManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for _, client := range m.clients {
		if err := client.Close(); err != nil {
			lastErr = err
		}
	}

	m.clients = make(map[string]*Client)
	return lastErr
}
```

---

## 4. 数据库实现

### 4.1 数据库配置

```go
// pkg/database/postgres.go
package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 数据库配置
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	LogLevel        string
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "",
		DBName:          "game_db",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
		LogLevel:        "info",
	}
}

// Open 打开数据库连接
func Open(config *Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.DBName,
		config.SSLMode,
	)

	logLevel := logger.Silent
	switch config.LogLevel {
	case "info":
		logLevel = logger.Info
	case "warn":
		logLevel = logger.Warn
	case "error":
		logLevel = logger.Error
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db failed: %w", err)
	}

	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	return db, nil
}

// DBManager 数据库管理器
type DBManager struct {
	dbs map[string]*gorm.DB
	mu  sync.RWMutex
}

// NewDBManager 创建数据库管理器
func NewDBManager() *DBManager {
	return &DBManager{
		dbs: make(map[string]*gorm.DB),
	}
}

// Register 注册数据库
func (m *DBManager) Register(name string, config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.dbs[name]; exists {
		return fmt.Errorf("database %s already registered", name)
	}

	db, err := Open(config)
	if err != nil {
		return err
	}

	m.dbs[name] = db
	return nil
}

// Get 获取数据库
func (m *DBManager) Get(name string) (*gorm.DB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db, exists := m.dbs[name]
	if !exists {
		return nil, fmt.Errorf("database %s not found", name)
	}

	return db, nil
}

// Close 关闭所有数据库连接
func (m *DBManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, db := range m.dbs {
		sqlDB, err := db.DB()
		if err != nil {
			lastErr = err
			continue
		}
		if err := sqlDB.Close(); err != nil {
			lastErr = fmt.Errorf("close %s failed: %w", name, err)
		}
	}

	m.dbs = make(map[string]*gorm.DB)
	return lastErr
}
```

### 4.2 模型定义

```go
// internal/game/model/game.go
package model

import (
	"time"
)

// Game 游戏模型
type Game struct {
	ID          int64                  `gorm:"primaryKey;autoIncrement" json:"id"`
	OrgID       int64                  `gorm:"not null;index" json:"org_id"`
	GameCode    string                 `gorm:"unique;not null;size:50;index" json:"game_code"`
	GameName    string                 `gorm:"not null;size:100" json:"game_name"`
	GameType    string                 `gorm:"not null;size:20" json:"game_type"`
	GameIcon    string                 `gorm:"size:255" json:"game_icon"`
	GameCover   string                 `gorm:"size:255" json:"game_cover"`
	GameConfig  map[string]interface{} `gorm:"type:jsonb" json:"game_config"`
	Status      int32                  `gorm:"default:1;index" json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// TableName 表名
func (Game) TableName() string {
	return "games"
}

// GameRoom 游戏房间模型
type GameRoom struct {
	ID          int64                  `gorm:"primaryKey;autoIncrement" json:"id"`
	GameID      int64                  `gorm:"not null;index" json:"game_id"`
	RoomID      string                 `gorm:"unique;not null;size:50;index" json:"room_id"`
	RoomName    string                 `gorm:"size:100" json:"room_name"`
	MaxPlayers  int32                  `gorm:"default:4" json:"max_players"`
	RoomConfig  map[string]interface{} `gorm:"type:jsonb" json:"room_config"`
	RoomState   map[string]interface{} `gorm:"type:jsonb" json:"room_state"`
	Status      string                 `gorm:"size:20;default:'waiting'" json:"status"`
	CreatedBy   int64                  `gorm:"index" json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// TableName 表名
func (GameRoom) TableName() string {
	return "game_rooms"
}

// RoomPlayer 房间玩家模型
type RoomPlayer struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoomID    string    `gorm:"not null;size:50;index:idx_room_player" json:"room_id"`
	PlayerID  int64     `gorm:"not null;index:idx_room_player" json:"player_id"`
	Nickname  string    `gorm:"size:50" json:"nickname"`
	Avatar    string    `gorm:"size:255" json:"avatar"`
	Status    string    `gorm:"size:20;default:'ready'" json:"status"`
	Score     int64     `gorm:"default:0" json:"score"`
	PlayerData map[string]interface{} `gorm:"type:jsonb" json:"player_data"`
	JoinedAt  time.Time `json:"joined_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 表名
func (RoomPlayer) TableName() string {
	return "room_players"
}
```

### 4.3 Repository实现

```go
// internal/game/repository/game_repo.go
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"your-project/internal/game/model"
)

// GameRepository 游戏仓储
type GameRepository interface {
	Create(ctx context.Context, game *model.Game) error
	GetByID(ctx context.Context, id int64) (*model.Game, error)
	GetByCode(ctx context.Context, code string) (*model.Game, error)
	Update(ctx context.Context, game *model.Game) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter *GameFilter) ([]*model.Game, int64, error)
}

// GameFilter 游戏筛选条件
type GameFilter struct {
	OrgID  *int64
	Status *int32
	Page   int
	Size   int
}

// gameRepository 游戏仓储实现
type gameRepository struct {
	db *gorm.DB
}

// NewGameRepository 创建游戏仓储
func NewGameRepository(db *gorm.DB) GameRepository {
	return &gameRepository{db: db}
}

// Create 创建游戏
func (r *gameRepository) Create(ctx context.Context, game *model.Game) error {
	return r.db.WithContext(ctx).Create(game).Error
}

// GetByID 根据ID获取游戏
func (r *gameRepository) GetByID(ctx context.Context, id int64) (*model.Game, error) {
	var game model.Game
	err := r.db.WithContext(ctx).First(&game, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &game, nil
}

// GetByCode 根据代码获取游戏
func (r *gameRepository) GetByCode(ctx context.Context, code string) (*model.Game, error) {
	var game model.Game
	err := r.db.WithContext(ctx).Where("game_code = ?", code).First(&game).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &game, nil
}

// Update 更新游戏
func (r *gameRepository) Update(ctx context.Context, game *model.Game) error {
	result := r.db.WithContext(ctx).Save(game)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete 删除游戏
func (r *gameRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&model.Game{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// List 列出游戏
func (r *gameRepository) List(ctx context.Context, filter *GameFilter) ([]*model.Game, int64, error) {
	var games []*model.Game
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Game{})

	if filter.OrgID != nil {
		query = query.Where("org_id = ?", *filter.OrgID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	// 计数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	if filter.Page > 0 && filter.Size > 0 {
		offset := (filter.Page - 1) * filter.Size
		query = query.Offset(offset).Limit(filter.Size)
	}

	// 查询
	err := query.Order("created_at DESC").Find(&games).Error
	return games, total, err
}

// ErrNotFound 记录不存在
var ErrNotFound = errors.New("record not found")
```

---

## 5. 缓存实现

```go
// pkg/cache/redis.go
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient Redis客户端
type RedisClient struct {
	client *redis.Client
}

// Config Redis配置
type Config struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(config *Config) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	return &RedisClient{client: client}, nil
}

// Set 设置键值
func (c *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Get 获取值
func (c *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

// GetJSON 获取JSON值
func (c *RedisClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// SetJSON 设置JSON值
func (c *RedisClient) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, data, expiration)
}

// Del 删除键
func (c *RedisClient) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func (c *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, key).Result()
	return count > 0, err
}

// Expire 设置过期时间
func (c *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// Incr 自增
func (c *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// Decr 自减
func (c *RedisClient) Decr(ctx context.Context, key string) (int64, error) {
	return c.client.Decr(ctx, key).Result()
}

// HSet 设置哈希字段
func (c *RedisClient) HSet(ctx context.Context, key, field string, value interface{}) error {
	return c.client.HSet(ctx, key, field, value).Err()
}

// HGet 获取哈希字段
func (c *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	return c.client.HGet(ctx, key, field).Result()
}

// HGetAll 获取所有哈希字段
func (c *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

// HDel 删除哈希字段
func (c *RedisClient) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, key, fields...).Err()
}

// ZAdd 添加有序集合成员
func (c *RedisClient) ZAdd(ctx context.Context, key string, score float64, member string) error {
	return c.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// ZRange 按范围获取有序集合成员
func (c *RedisClient) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.ZRange(ctx, key, start, stop).Result()
}

// ZRank 获取成员排名
func (c *RedisClient) ZRank(ctx context.Context, key, member string) (int64, error) {
	return c.client.ZRank(ctx, key, member).Result()
}

// Publish 发布消息
func (c *RedisClient) Publish(ctx context.Context, channel string, message interface{}) error {
	return c.client.Publish(ctx, channel, message).Err()
}

// Subscribe 订阅消息
func (c *RedisClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// Close 关闭连接
func (c *RedisClient) Close() error {
	return c.client.Close()
}
```

---

## 6. 消息队列实现

```go
// pkg/queue/kafka.go
package queue

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// KafkaClient Kafka客户端
type KafkaClient struct {
	writer *kafka.Writer
	reader *kafka.Reader
}

// Config Kafka配置
type Config struct {
	Brokers []string
	Topic   string
	GroupID string
}

// NewKafkaClient 创建Kafka客户端
func NewKafkaClient(config *Config) *KafkaClient {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(config.Brokers...),
		Topic:    config.Topic,
		Balancer: &kafka.LeastBytes{},
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  config.Brokers,
		Topic:    config.Topic,
		GroupID:  config.GroupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &KafkaClient{
		writer: writer,
		reader: reader,
	}
}

// Publish 发布消息
func (c *KafkaClient) Publish(ctx context.Context, key string, value []byte) error {
	return c.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: value,
	})
}

// Consume 消费消息
func (c *KafkaClient) Consume(ctx context.Context, handler func(key, value []byte) error) error {
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		if err := handler(msg.Key, msg.Value); err != nil {
			return err
		}
	}
}

// Close 关闭连接
func (c *KafkaClient) Close() error {
	if err := c.writer.Close(); err != nil {
		return err
	}
	return c.reader.Close()
}
```

---

## 7. API网关实现

```go
// cmd/gateway/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"your-project/internal/middleware"
	"your-project/pkg/config"
	"your-project/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.prod.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// 初始化日志
	if err := logger.Init(cfg.Log); err != nil {
		log.Fatalf("init logger failed: %v", err)
	}

	// 创建路由
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.Use(middleware.Logging())
	r.Use(middleware.CORS())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})

	// API路由
	setupRoutes(r, cfg)

	// 启动服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Gateway.Port),
		Handler: r,
	}

	go func() {
		logger.Info("gateway starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen failed", "error", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("gateway shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("shutdown failed", "error", err)
	}

	logger.Info("gateway exited")
}

func setupRoutes(r *gin.Engine, cfg *config.Config) {
	// 代理到后端服务
	proxy := NewServiceProxy(cfg.Services)

	api := r.Group("/api/v1")
	{
		// 用户服务
		user := api.Group("/user")
		user.Any("/*path", proxy.Proxy("user-service"))

		// 游戏服务
		game := api.Group("/game")
		game.Any("/*path", proxy.Proxy("game-service"))

		// 支付服务
		payment := api.Group("/payment")
		payment.Any("/*path", proxy.Proxy("payment-service"))
	}
}
```

---

## 8. WebSocket网关实现

```go
// internal/ws/hub.go
package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"your-project/pkg/queue"
)

// Hub WebSocket连接中心
type Hub struct {
	connections map[string]*Connection
	rooms       map[string]map[string]*Connection
	broadcast   chan *Message
	register    chan *Connection
	unregister  chan *Connection
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	publisher   *queue.KafkaClient
}

// Message WebSocket消息
type Message struct {
	Type    string      `json:"type"`
	From    string      `json:"from,omitempty"`
	To      string      `json:"to,omitempty"`
	RoomID  string      `json:"room_id,omitempty"`
	Data    interface{} `json:"data"`
}

// NewHub 创建Hub
func NewHub(kafkaClient *queue.KafkaClient) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		connections: make(map[string]*Connection),
		rooms:       make(map[string]map[string]*Connection),
		broadcast:   make(chan *Message, 1000),
		register:    make(chan *Connection, 100),
		unregister:  make(chan *Connection, 100),
		ctx:         ctx,
		cancel:      cancel,
		publisher:   kafkaClient,
	}
}

// Run 运行Hub
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.registerConnection(conn)

		case conn := <-h.unregister:
			h.unregisterConnection(conn)

		case msg := <-h.broadcast:
			h.handleBroadcast(msg)

		case <-h.ctx.Done():
			return
		}
	}
}

// registerConnection 注册连接
func (h *Hub) registerConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.connections[conn.ID] = conn

	log.Printf("[Hub] connection registered: %s", conn.ID)
}

// unregisterConnection 注销连接
func (h *Hub) unregisterConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 从所有房间移除
	for roomID, conns := range h.rooms {
		delete(conns, conn.ID)
		if len(conns) == 0 {
			delete(h.rooms, roomID)
		}
	}

	// 删除连接
	delete(h.connections, conn.ID)

	close(conn.Send)

	log.Printf("[Hub] connection unregistered: %s", conn.ID)
}

// JoinRoom 加入房间
func (h *Hub) JoinRoom(conn *Connection, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.rooms[roomID]; !exists {
		h.rooms[roomID] = make(map[string]*Connection)
	}

	h.rooms[roomID][conn.ID] = conn
	conn.RoomID = roomID

	log.Printf("[Hub] connection %s joined room %s", conn.ID, roomID)
}

// LeaveRoom 离开房间
func (h *Hub) LeaveRoom(conn *Connection, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conns, exists := h.rooms[roomID]; exists {
		delete(conns, conn.ID)
		if len(conns) == 0 {
			delete(h.rooms, roomID)
		}
	}

	conn.RoomID = ""

	log.Printf("[Hub] connection %s left room %s", conn.ID, roomID)
}

// BroadcastToRoom 向房间广播
func (h *Hub) BroadcastToRoom(roomID string, msg *Message) {
	h.mu.RLock()
	conns, exists := h.rooms[roomID]
	h.mu.RUnlock()

	if !exists {
		return
	}

	data, _ := json.Marshal(msg)

	for _, conn := range conns {
		select {
		case conn.Send <- data:
		default:
			// 邮箱满，关闭连接
			h.unregister <- conn
		}
	}
}

// SendToConnection 向指定连接发送消息
func (h *Hub) SendToConnection(connID string, msg *Message) error {
	h.mu.RLock()
	conn, exists := h.connections[connID]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		return fmt.Errorf("connection %s mailbox full", connID)
	}
}

// handleBroadcast 处理广播消息
func (h *Hub) handleBroadcast(msg *Message) {
	if msg.RoomID != "" {
		h.BroadcastToRoom(msg.RoomID, msg)
	} else if msg.To != "" {
		h.SendToConnection(msg.To, msg)
	}
}

// Close 关闭Hub
func (h *Hub) Close() {
	h.cancel()

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, conn := range h.connections {
		close(conn.Send)
	}

	h.connections = make(map[string]*Connection)
	h.rooms = make(map[string]map[string]*Connection)
}
```

---

## 9. 监控与日志

### 9.1 日志实现

```go
// pkg/logger/logger.go
package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// Config 日志配置
type Config struct {
	Level      string
	Format     string
	Output     string
	MaxSize    int
	MaxBackups int
	MaxAge     int
}

// Init 初始化日志
func Init(cfg *Config) error {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return nil
}

// Debug 调试日志
func Debug(msg string, fields ...Field) {
	log.Debug(msg, toZapFields(fields...)...)
}

// Info 信息日志
func Info(msg string, fields ...Field) {
	log.Info(msg, toZapFields(fields...)...)
}

// Warn 警告日志
func Warn(msg string, fields ...Field) {
	log.Warn(msg, toZapFields(fields...)...)
}

// Error 错误日志
func Error(msg string, fields ...Field) {
	log.Error(msg, toZapFields(fields...)...)
}

// Fatal 致命日志
func Fatal(msg string, fields ...Field) {
	log.Fatal(msg, toZapFields(fields...)...)
}

// With 添加字段
func With(fields ...Field) *zap.Logger {
	return log.With(toZapFields(fields...)...)
}

// Field 日志字段
type Field struct {
	Key   string
	Value interface{}
}

// String 字符串字段
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int64 int64字段
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Err 错误字段
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any 任意类型字段
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// toZapFields 转换为zap字段
func toZapFields(fields ...Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))
	for i, f := range fields {
		zapFields[i] = zap.Any(f.Key, f.Value)
	}
	return zapFields
}
```

### 9.2 指标收集

```go
// pkg/metrics/metrics.go
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP请求相关
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Actor相关
	actorMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "actor_messages_total",
			Help: "Total number of actor messages",
		},
		[]string{"actor_type", "status"},
	)

	actorMessageDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "actor_message_duration_seconds",
			Help:    "Actor message processing duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"actor_type"},
	)

	actorActiveCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "actor_active_count",
			Help: "Number of active actors",
		},
		[]string{"actor_type"},
	)

	// 游戏相关
	gameRoomsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "game_rooms_active",
			Help: "Number of active game rooms",
		},
	)

	gamePlayersActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "game_players_active",
			Help: "Number of active game players",
		},
	)

	// 引擎相关
	engineExecuteDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engine_execute_duration_seconds",
			Help:    "Game engine execute duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"engine_type", "method"},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		actorMessagesTotal,
		actorMessageDuration,
		actorActiveCount,
		gameRoomsActive,
		gamePlayersActive,
		engineExecuteDuration,
	)
}

// RecordHTTPRequest 记录HTTP请求
func RecordHTTPRequest(method, endpoint string, status int, duration time.Duration) {
	httpRequestsTotal.WithLabelValues(method, endpoint, fmt.Sprintf("%d", status)).Inc()
	httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordActorMessage 记录Actor消息
func RecordActorMessage(actorType string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}
	actorMessagesTotal.WithLabelValues(actorType, status).Inc()
	actorMessageDuration.WithLabelValues(actorType).Observe(duration.Seconds())
}

// SetActorActiveCount 设置活跃Actor数量
func SetActorActiveCount(actorType string, count int) {
	actorActiveCount.WithLabelValues(actorType).Set(float64(count))
}

// SetGameRoomsActive 设置活跃房间数量
func SetGameRoomsActive(count int) {
	gameRoomsActive.Set(float64(count))
}

// SetGamePlayersActive 设置活跃玩家数量
func SetGamePlayersActive(count int) {
	gamePlayersActive.Set(float64(count))
}

// RecordEngineExecute 记录引擎执行
func RecordEngineExecute(engineType, method string, duration time.Duration) {
	engineExecuteDuration.WithLabelValues(engineType, method).Observe(duration.Seconds())
}

// Handler 指标HTTP处理器
func Handler() http.Handler {
	return promhttp.Handler()
}
```

---

## 10. 部署配置

### 10.1 Docker配置

```dockerfile
# deployments/docker/Dockerfile.game
FROM golang:1.23-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建
RUN CGO_ENABLED=0 go build -o game-service ./cmd/game-service

# 运行
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/game-service .
COPY --from=builder /app/configs ./configs

EXPOSE 8002

CMD ["./game-service"]
```

### 10.2 Docker Compose

```yaml
# deployments/docker/docker-compose.yml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: game_db
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  kafka:
    image: bitnami/kafka:latest
    environment:
      KAFKA_CFG_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
    ports:
      - "9092:9092"
    depends_on:
      - zookeeper

  zookeeper:
    image: bitnami/zookeeper:latest
    environment:
      ZOO_SERVER_ID: 1

  game-service:
    build:
      context: ..
      dockerfile: deployments/docker/Dockerfile.game
    ports:
      - "8002:8002"
    depends_on:
      - postgres
      - redis
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis

  gateway:
    build:
      context: ..
      dockerfile: deployments/docker/Dockerfile.gateway
    ports:
      - "8080:8080"
    depends_on:
      - game-service

volumes:
  postgres_data:
  redis_data:
```

### 10.3 Kubernetes配置

```yaml
# deployments/k8s/game-service.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: game-service-config
data:
  config.yaml: |
    server:
      port: 8002
    database:
      host: postgres-service
      port: 5432
      user: postgres
      password: postgres
      dbname: game_db
    redis:
      host: redis-service
      port: 6379

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: game-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: game-service
  template:
    metadata:
      labels:
        app: game-service
    spec:
      containers:
      - name: game-service
        image: game-service:latest
        ports:
        - containerPort: 8002
        env:
        - name: CONFIG_PATH
          value: "/config/config.yaml"
        volumeMounts:
        - name: config
          mountPath: /config
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8002
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8002
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: game-service-config

---
apiVersion: v1
kind: Service
metadata:
  name: game-service
spec:
  selector:
    app: game-service
  ports:
  - port: 8002
    targetPort: 8002
  type: ClusterIP
```

---

## 11. Actor持久化机制

### 11.1 持久化接口

```go
// internal/actor/persistence.go
package actor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ActorSnapshot Actor快照
type ActorSnapshot struct {
	ID        int64                  `gorm:"primaryKey"`
	ActorID   string                 `gorm:"uniqueIndex;not null;size:100"`
	ActorType string                 `gorm:"not null;size:50"`
	Version   int64                  `gorm:"not null;default:0"`
	State     []byte                 `gorm:"type:jsonb;not null"`
	CreatedAt time.Time              `gorm:"not null"`
}

// ActorEvent Actor事件
type ActorEvent struct {
	ID        int64                  `gorm:"primaryKey"`
	ActorID   string                 `gorm:"index;not null;size:100"`
	EventType string                 `gorm:"not null;size:50"`
	Payload   []byte                 `gorm:"type:jsonb"`
	Version   int64                  `gorm:"not null"`
	Timestamp time.Time              `gorm:"not null;index"`
}

// Persistence 持久化接口
type Persistence interface {
	// SaveSnapshot 保存快照
	SaveSnapshot(ctx context.Context, actorID string, version int64, state interface{}) error

	// LoadSnapshot 加载最新快照
	LoadSnapshot(ctx context.Context, actorID string) (*ActorSnapshot, error)

	// AppendEvent 追加事件
	AppendEvent(ctx context.Context, event *ActorEvent) error

	// ReplayEvents 重放事件
	ReplayEvents(ctx context.Context, actorID string, fromVersion int64) ([]*ActorEvent, error)
}

// GormPersistence GORM持久化实现
type GormPersistence struct {
	db *gorm.DB
}

// NewGormPersistence 创建持久化
func NewGormPersistence(db *gorm.DB) *GormPersistence {
	return &GormPersistence{db: db}
}

// SaveSnapshot 保存快照
func (p *GormPersistence) SaveSnapshot(ctx context.Context, actorID string, version int64, state interface{}) error {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state failed: %w", err)
	}

	snapshot := &ActorSnapshot{
		ActorID:   actorID,
		ActorType: "game", // 从Actor获取
		Version:   version,
		State:     stateBytes,
		CreatedAt: time.Now(),
	}

	return p.db.WithContext(ctx).Save(snapshot).Error
}

// LoadSnapshot 加载快照
func (p *GormPersistence) LoadSnapshot(ctx context.Context, actorID string) (*ActorSnapshot, error) {
	var snapshot ActorSnapshot
	err := p.db.WithContext(ctx).
		Where("actor_id = ?", actorID).
		Order("version DESC").
		First(&snapshot).Error

	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// AppendEvent 追加事件
func (p *GormPersistence) AppendEvent(ctx context.Context, event *ActorEvent) error {
	return p.db.WithContext(ctx).Create(event).Error
}

// ReplayEvents 重放事件
func (p *GormPersistence) ReplayEvents(ctx context.Context, actorID string, fromVersion int64) ([]*ActorEvent, error) {
	var events []*ActorEvent
	err := p.db.WithContext(ctx).
		Where("actor_id = ? AND version > ?", actorID, fromVersion).
		Order("version ASC").
		Find(&events).Error

	return events, err
}
```

### 11.2 快照策略

```go
// internal/actor/snapshot.go
package actor

import (
	"context"
	"sync"
	"time"
)

// SnapshotStrategy 快照策略
type SnapshotStrategy struct {
	MessageInterval int           // 消息间隔
	TimeInterval    time.Duration // 时间间隔
	mu              sync.Mutex
	lastSnapshot    time.Time
	messageCount    int64
}

// NewSnapshotStrategy 创建快照策略
func NewSnapshotStrategy(messageInterval int, timeInterval time.Duration) *SnapshotStrategy {
	return &SnapshotStrategy{
		MessageInterval: messageInterval,
		TimeInterval:    timeInterval,
		lastSnapshot:    time.Now(),
	}
}

// ShouldSnapshot 判断是否需要快照
func (s *SnapshotStrategy) ShouldSnapshot() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messageCount++

	// 检查消息间隔
	if s.MessageInterval > 0 && s.messageCount >= int64(s.MessageInterval) {
		s.messageCount = 0
		s.lastSnapshot = time.Now()
		return true
	}

	// 检查时间间隔
	if s.TimeInterval > 0 && time.Since(s.lastSnapshot) >= s.TimeInterval {
		s.messageCount = 0
		s.lastSnapshot = time.Now()
		return true
	}

	return false
}

// PersistingActor 持久化Actor
type PersistingActor struct {
	*BaseActor
	persistence Persistence
	strategy   *SnapshotStrategy
	version    int64
}

// NewPersistingActor 创建持久化Actor
func NewPersistingActor(
	id, actorType string,
	inboxSize int,
	handler MessageHandler,
	persistence Persistence,
	strategy *SnapshotStrategy,
) *PersistingActor {
	return &PersistingActor{
		BaseActor:  NewBaseActor(id, actorType, inboxSize, handler),
		persistence: persistence,
		strategy:   strategy,
	}
}

// Start 启动Actor（尝试从快照恢复）
func (a *PersistingActor) Start(ctx context.Context) error {
	// 尝试加载快照
	snapshot, err := a.persistence.LoadSnapshot(ctx, a.ID())
	if err == nil {
		// 恢复状态
		a.restoreFromSnapshot(snapshot)
	}

	return a.BaseActor.Start(ctx)
}

// processMessage 处理消息（带持久化）
func (a *PersistingActor) processMessage(msg Message) {
	start := time.Now()

	err := a.handler.Receive(a.ctx, msg)

	// 记录事件
	event := &ActorEvent{
		ActorID:   a.ID(),
		EventType: msg.Type(),
		Version:   a.version + 1,
		Timestamp: time.Now(),
	}
	_ = a.persistence.AppendEvent(a.ctx, event)
	a.version++

	// 检查是否需要快照
	if a.strategy.ShouldSnapshot() {
		state := a.getState()
		_ = a.persistence.SaveSnapshot(a.ctx, a.ID(), a.version, state)
	}

	// 更新统计
	a.mu.Lock()
	a.stats.MessageCount++
	a.stats.ProcessTime += time.Since(start)
	a.stats.LastMessageTime = time.Now()
	if err != nil {
		a.stats.ErrorCount++
	}
	a.mu.Unlock()
}

// restoreFromSnapshot 从快照恢复
func (a *PersistingActor) restoreFromSnapshot(snapshot *ActorSnapshot) {
	// TODO: 根据Actor类型恢复状态
	a.version = snapshot.Version
}

// getState 获取状态
func (a *PersistingActor) getState() interface{} {
	// TODO: 根据Actor类型获取状态
	return nil
}
```

---

## 12. WebSocket消息协议

### 12.1 消息格式

```go
// internal/ws/message.go
package ws

import (
	"encoding/json"
)

// MessageType 消息类型
type MessageType string

const (
	// 系统消息
	MessageTypeConnect     MessageType = "connect"
	MessageTypeDisconnect  MessageType = "disconnect"
	MessageTypePing        MessageType = "ping"
	MessageTypePong        MessageType = "pong"
	MessageTypeError       MessageType = "error"

	// 房间消息
	MessageTypeRoomCreate  MessageType = "room.create"
	MessageTypeRoomJoin    MessageType = "room.join"
	MessageTypeRoomLeave   MessageType = "room.leave"
	MessageTypeRoomStart   MessageType = "room.start"
	MessageTypeRoomStop    MessageType = "room.stop"
	MessageTypeRoomState   MessageType = "room.state"

	// 游戏消息
	MessageTypeGameTick    MessageType = "game.tick"
	MessageTypeGameAction  MessageType = "game.action"
	MessageTypeGameState   MessageType = "game.state"

	// 聊天消息
	MessageTypeChatMessage MessageType = "chat.message"
)

// Packet WebSocket数据包
type Packet struct {
	Type      MessageType   `json:"type"`
	RequestID string        `json:"request_id,omitempty"`
	Data      interface{}   `json:"data,omitempty"`
	Error     string        `json:"error,omitempty"`
	Timestamp int64         `json:"timestamp"`
}

// ConnectData 连接数据
type ConnectData struct {
	Token     string `json:"token"`
	UserID    int64  `json:"user_id"`
	DeviceID  string `json:"device_id"`
}

// RoomCreateData 创建房间数据
type RoomCreateData struct {
	GameID     int64                  `json:"game_id"`
	RoomName   string                 `json:"room_name"`
	MaxPlayers int                    `json:"max_players"`
	Config     map[string]interface{} `json:"config"`
}

// RoomJoinData 加入房间数据
type RoomJoinData struct {
	RoomID string `json:"room_id"`
}

// RoomLeaveData 离开房间数据
type RoomLeaveData struct {
	RoomID string `json:"room_id"`
}

// GameActionData 游戏操作数据
type GameActionData struct {
	RoomID string                 `json:"room_id"`
	Action string                 `json:"action"`
	Data   map[string]interface{} `json:"data"`
}

// NewPacket 创建数据包
func NewPacket(typ MessageType, data interface{}) *Packet {
	return &Packet{
		Type:      typ,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

// ParsePacket 解析数据包
func ParsePacket(data []byte) (*Packet, error) {
	var packet Packet
	if err := json.Unmarshal(data, &packet); err != nil {
		return nil, err
	}
	return &packet, nil
}

// ToJSON 转换为JSON
func (p *Packet) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}
```

### 12.2 连接处理

```go
// internal/ws/connection.go
package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境需要验证
	},
}

// Connection WebSocket连接
type Connection struct {
	ID        string
	UserID    int64
	DeviceID  string
	RoomID    string
	ws        *websocket.Conn
	Send      chan []byte
	hub       *Hub
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewConnection 创建连接
func NewConnection(ws *websocket.Conn, userID int64, deviceID string, hub *Hub) *Connection {
	ctx, cancel := context.WithCancel(context.Background())

	return &Connection{
		ID:       generateConnectionID(),
		UserID:   userID,
		DeviceID: deviceID,
		ws:       ws,
		Send:     make(chan []byte, 256),
		hub:      hub,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// ReadPump 读取循环
func (c *Connection) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.ws.Close()
	}()

	c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] unexpected close: %v", err)
			}
			break
		}

		// 解析消息
		packet, err := ParsePacket(message)
		if err != nil {
			c.SendError("invalid message format")
			continue
		}

		// 处理消息
		c.handlePacket(packet)
	}
}

// WritePump 写入循环
func (c *Connection) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// handlePacket 处理数据包
func (c *Connection) handlePacket(packet *Packet) {
	switch packet.Type {
	case MessageTypePing:
		c.SendPong()

	case MessageTypeRoomJoin:
		c.handleRoomJoin(packet)

	case MessageTypeRoomLeave:
		c.handleRoomLeave(packet)

	case MessageTypeGameAction:
		c.handleGameAction(packet)

	default:
		log.Printf("[WS] unknown message type: %s", packet.Type)
	}
}

// handleRoomJoin 处理加入房间
func (c *Connection) handleRoomJoin(packet *Packet) {
	dataBytes, _ := json.Marshal(packet.Data)
	var data RoomJoinData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		c.SendError("invalid room join data")
		return
	}

	// 加入房间
	c.hub.JoinRoom(c, data.RoomID)

	// 发送成功响应
	c.SendResponse(MessageTypeRoomJoin, map[string]string{
		"room_id": data.RoomID,
		"status":  "joined",
	})
}

// handleRoomLeave 处理离开房间
func (c *Connection) handleRoomLeave(packet *Packet) {
	dataBytes, _ := json.Marshal(packet.Data)
	var data RoomLeaveData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		c.SendError("invalid room leave data")
		return
	}

	// 离开房间
	c.hub.LeaveRoom(c, data.RoomID)

	// 发送成功响应
	c.SendResponse(MessageTypeRoomLeave, map[string]string{
		"room_id": data.RoomID,
		"status":  "left",
	})
}

// handleGameAction 处理游戏操作
func (c *Connection) handleGameAction(packet *Packet) {
	dataBytes, _ := json.Marshal(packet.Data)
	var data GameActionData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		c.SendError("invalid game action data")
		return
	}

	// 转发到游戏服务
	c.hub.BroadcastToRoom(data.RoomID, &Message{
		Type:    string(packet.Type),
		From:    c.ID,
		RoomID:  data.RoomID,
		Data:    data,
	})
}

// SendResponse 发送响应
func (c *Connection) SendResponse(typ MessageType, data interface{}) {
	packet := NewPacket(typ, data)
	bytes, err := packet.ToJSON()
	if err != nil {
		return
	}

	select {
	case c.Send <- bytes:
	default:
		log.Printf("[WS] connection %s send buffer full", c.ID)
	}
}

// SendError 发送错误
func (c *Connection) SendError(message string) {
	packet := NewPacket(MessageTypeError, map[string]string{
		"message": message,
	})
	bytes, _ := packet.ToJSON()

	select {
	case c.Send <- bytes:
	default:
	}
}

// SendPong 发送pong
func (c *Connection) SendPong() {
	packet := NewPacket(MessageTypePong, nil)
	bytes, _ := packet.ToJSON()

	select {
	case c.Send <- bytes:
	default:
	}
}

// Close 关闭连接
func (c *Connection) Close() {
	c.cancel()
	close(c.Send)
}

// generateConnectionID 生成连接ID
func generateConnectionID() string {
	return fmt.Sprintf("conn_%d_%s", time.Now().UnixNano(), randomString(8))
}
```

### 12.3 HTTP处理

```go
// internal/ws/handler.go
package ws

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler WebSocket处理器
type Handler struct {
	hub *Hub
}

// NewHandler 创建处理器
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// HandleWebSocket 处理WebSocket连接
func (h *Handler) HandleWebSocket(c *gin.Context) {
	// 升级为WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] upgrade failed: %v", err)
		return
	}

	// 获取用户信息
	userID := c.GetInt64("user_id")
	if userID == 0 {
		// 从token解析
		token := c.Query("token")
		userID, err = parseToken(token)
		if err != nil {
			conn.Close()
			return
		}
	}

	deviceID := c.Query("device_id")
	if deviceID == "" {
		deviceID = c.ClientIP()
	}

	// 创建连接
	connection := NewConnection(conn, userID, deviceID, h.hub)

	// 注册到Hub
	h.hub.register <- connection

	// 启动读写循环
	go connection.WritePump()
	go connection.ReadPump()
}
```

---

## 13. 游戏引擎脚本示例

### 13.1 JavaScript游戏脚本

```javascript
// games/card-game/game.js

// 游戏状态
const state = {
    players: [],
    currentPlayer: 0,
    deck: [],
    hands: {},
    scores: {},
    status: 'waiting',
    round: 0
};

// 初始化游戏
function onStart(config) {
    log('Game starting with config:', JSON.stringify(config));
    state.status = 'playing';
    state.deck = createDeck();
    shuffleDeck();

    // 发牌
    dealCards();

    return {
        status: state.status,
        round: state.round
    };
}

// 游戏tick
function onTick(tick) {
    if (state.status !== 'playing') {
        return;
    }

    // 检查游戏是否结束
    if (shouldEndGame()) {
        endGame();
    }
}

// 玩家操作
function onAction(action) {
    if (state.status !== 'playing') {
        return { error: 'Game not in playing status' };
    }

    const playerId = action.player_id;
    const actionType = action.data.action;

    switch (actionType) {
        case 'play_card':
            return playCard(playerId, action.data.card);
        case 'draw_card':
            return drawCard(playerId);
        case 'pass':
            return passTurn(playerId);
        default:
            return { error: 'Unknown action: ' + actionType };
    }
}

// 创建牌组
function createDeck() {
    const suits = ['hearts', 'diamonds', 'clubs', 'spades'];
    const ranks = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'];
    const deck = [];

    for (const suit of suits) {
        for (const rank of ranks) {
            deck.push({ suit, rank, value: getCardValue(rank) });
        }
    }

    return deck;
}

// 洗牌
function shuffleDeck() {
    for (let i = state.deck.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [state.deck[i], state.deck[j]] = [state.deck[j], state.deck[i]];
    }
}

// 发牌
function dealCards() {
    const cardsPerPlayer = 5;

    for (const player of state.players) {
        const hand = [];
        for (let i = 0; i < cardsPerPlayer; i++) {
            hand.push(state.deck.pop());
        }
        state.hands[player.id] = hand;
        state.scores[player.id] = 0;
    }
}

// 出牌
function playCard(playerId, card) {
    const hand = state.hands[playerId];
    const cardIndex = hand.findIndex(c => c.suit === card.suit && c.rank === card.rank);

    if (cardIndex === -1) {
        return { error: 'Card not in hand' };
    }

    hand.splice(cardIndex, 1);
    state.hands[playerId] = hand;

    // 广播出牌
    broadcast({
        type: 'card_played',
        player_id: playerId,
        card: card
    });

    // 切换到下一个玩家
    nextPlayer();

    return { success: true };
}

// 抽牌
function drawCard(playerId) {
    if (state.deck.length === 0) {
        return { error: 'Deck is empty' };
    }

    const card = state.deck.pop();
    state.hands[playerId].push(card);

    return { success: true, card: card };
}

// 跳过
function passTurn(playerId) {
    nextPlayer();
    return { success: true };
}

// 下一个玩家
function nextPlayer() {
    state.currentPlayer = (state.currentPlayer + 1) % state.players.length;
}

// 游戏是否结束
function shouldEndGame() {
    // 当所有玩家手牌为空或牌堆为空
    for (const player of state.players) {
        if (state.hands[player.id].length === 0) {
            return true;
        }
    }
    return state.deck.length === 0;
}

// 结束游戏
function endGame() {
    state.status = 'ended';

    // 计算分数
    calculateScores();

    // 广播结果
    broadcast({
        type: 'game_ended',
        scores: state.scores,
        winner: getWinner()
    });
}

// 计算分数
function calculateScores() {
    for (const player of state.players) {
        const hand = state.hands[player.id];
        let score = 0;
        for (const card of hand) {
            score += card.value;
        }
        state.scores[player.id] = score;
    }
}

// 获取获胜者
function getWinner() {
    let minScore = Infinity;
    let winner = null;

    for (const player of state.players) {
        if (state.scores[player.id] < minScore) {
            minScore = state.scores[player.id];
            winner = player.id;
        }
    }

    return winner;
}

// 获取卡牌值
function getCardValue(rank) {
    const values = {
        'A': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6,
        '7': 7, '8': 8, '9': 9, '10': 10, 'J': 10, 'Q': 10, 'K': 10
    };
    return values[rank] || 0;
}

// 广播消息
function broadcast(data) {
    // 通过WebSocket广播
    if (typeof broadcast !== 'undefined') {
        broadcast(JSON.stringify(data));
    }
}

// 获取游戏状态
function getState() {
    return {
        players: state.players,
        current_player: state.players[state.currentPlayer]?.id,
        hands: state.hands,
        scores: state.scores,
        status: state.status,
        round: state.round,
        deck_count: state.deck.length
    };
}
```

### 13.2 游戏配置示例

```json
{
    "game_id": "card-game",
    "game_name": "卡牌游戏",
    "game_type": "card",
    "max_players": 4,
    "min_players": 2,
    "config": {
        "initial_cards": 5,
        "deck_type": "standard",
        "scoring": "low_wins",
        "round_time": 30
    },
    "script": {
        "type": "javascript",
        "entry": "game.js",
        "version": "1.0.0"
    }
}
```

---

## 14. 中间件实现

### 14.1 认证中间件

```go
// internal/middleware/auth.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"your-project/pkg/apperror"
	"your-project/pkg/jwt"
)

// Auth 认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从header获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, apperror.ErrUnauthorized.WithMessage("Missing authorization header"))
			c.Abort()
			return
		}

		// 解析Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Error(c, apperror.ErrUnauthorized.WithMessage("Invalid authorization format"))
			c.Abort()
			return
		}

		token := parts[1]

		// 验证token
		claims, err := jwt.ParseToken(token)
		if err != nil {
			response.Error(c, apperror.ErrTokenExpired.WithMessage(err.Error()))
			c.Abort()
			return
		}

		// 设置用户信息到context
		c.Set("user_id", claims.UserID)
		c.Set("org_id", claims.OrgID)
		c.Set("roles", claims.Roles)

		c.Next()
	}
}

// OptionalAuth 可选认证中间件
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		token := parts[1]

		claims, err := jwt.ParseToken(token)
		if err == nil {
			c.Set("user_id", claims.UserID)
			c.Set("org_id", claims.OrgID)
			c.Set("roles", claims.Roles)
		}

		c.Next()
	}
}
```

### 14.2 CORS中间件

```go
// internal/middleware/cors.go
package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
```

### 14.3 限流中间件

```go
// internal/middleware/rate_limit.go
package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"your-project/pkg/cache"
)

// RateLimiter 限流器
type RateLimiter struct {
	redis    *cache.RedisClient
	requests map[string]*limiter
	mu       sync.RWMutex
}

type limiter struct {
	tokens    int
	lastVisit time.Time
}

// NewRateLimiter 创建限流器
func NewRateLimiter(redis *cache.RedisClient) *RateLimiter {
	return &RateLimiter{
		redis:    redis,
		requests: make(map[string]*limiter),
	}
}

// Middleware 限流中间件
func (rl *RateLimiter) Middleware(rate int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rate_limit:%s", c.ClientIP())

		// 使用Redis滑动窗口
		allowed, err := rl.allowRequest(key, rate, window)
		if err != nil {
			// 出错时允许请求
			c.Next()
			return
		}

		if !allowed {
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rate))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}

// allowRequest 检查是否允许请求
func (rl *RateLimiter) allowRequest(key string, rate int, window time.Duration) (bool, error) {
	// 使用Redis的滑动窗口
	now := time.Now().UnixNano()
	windowStart := now - window.Nanoseconds()

	// 移除窗口外的记录
	rl.redis.ZRemRangeByScore(context.Background(), key, "0", fmt.Sprintf("%d", windowStart))

	// 获取当前窗口内的请求数
	count, err := rl.redis.ZCard(context.Background(), key)
	if err != nil {
		return false, err
	}

	if count >= int64(rate) {
		return false, nil
	}

	// 添加当前请求
	rl.redis.ZAdd(context.Background(), key, float64(now), fmt.Sprintf("%d", now))
	rl.redis.Expire(context.Background(), key, window)

	return true, nil
}
```

---

## 15. 配置管理

### 15.1 配置结构

```go
// pkg/config/config.go
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config 配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Log      LogConfig      `mapstructure:"log"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	GroupID string   `mapstructure:"group_id"`
	Topics  struct {
		GameEvents string `mapstructure:"game_events"`
		Chat      string `mapstructure:"chat"`
	} `mapstructure:"topics"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	ExpireTime time.Duration `mapstructure:"expire_time"`
	Issuer     string        `mapstructure:"issuer"`
}

// Load 加载配置
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// 设置默认值
	setDefaults(v)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config failed: %w", err)
	}

	// 环境变量覆盖
	v.AutomaticEnv()
	v.SetEnvPrefix("APP")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)

	// Database
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "game_db")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", 5*time.Minute)
	v.SetDefault("database.conn_max_idle_time", 30*time.Second)

	// Redis
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.min_idle_conns", 2)
	v.SetDefault("redis.dial_timeout", 5*time.Second)
	v.SetDefault("redis.read_timeout", 3*time.Second)
	v.SetDefault("redis.write_timeout", 3*time.Second)

	// Log
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.output", "stdout")

	// JWT
	v.SetDefault("jwt.secret", "your-secret-key")
	v.SetDefault("jwt.expire_time", 24*time.Hour)
	v.SetDefault("jwt.issuer", "game-platform")
}
```

### 15.2 配置文件示例

```yaml
# configs/config.prod.yaml
server:
  host: 0.0.0.0
  port: 8002
  mode: release
  read_timeout: 30s
  write_timeout: 30s

database:
  host: postgres-service
  port: 5432
  user: postgres
  password: ${DB_PASSWORD}
  dbname: game_db
  sslmode: disable
  max_open_conns: 25
  max_idle_conns: 10
  conn_max_lifetime: 5m
  conn_max_idle_time: 30s

redis:
  host: redis-service
  port: 6379
  password: ${REDIS_PASSWORD}
  db: 0
  pool_size: 10
  min_idle_conns: 2
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s

kafka:
  brokers:
    - kafka-service:9092
  group_id: game-service
  topics:
    game_events: game.events
    chat: chat.messages

log:
  level: info
  format: json
  output: stdout

jwt:
  secret: ${JWT_SECRET}
  expire_time: 24h
  issuer: game-platform
```

---

## 16. User Service实现

### 16.1 用户服务结构

```go
// internal/user/service/user_service.go
package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"your-project/internal/user/model"
	"your-project/internal/user/repository"
	"your-project/pkg/apperror"
)

// UserService 用户服务
type UserService struct {
	repo   repository.UserRepository
	jwt    *jwt.Manager
	redis  *cache.RedisClient
}

// NewUserService 创建用户服务
func NewUserService(
	repo repository.UserRepository,
	jwt *jwt.Manager,
	redis *cache.RedisClient,
) *UserService {
	return &UserService{
		repo:  repo,
		jwt:   jwt,
		redis: redis,
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
	Nickname string `json:"nickname" binding:"required,min=2,max=20"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	User      *UserInfo `json:"user"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	CreatedAt int64  `json:"created_at"`
}

// Register 用户注册
func (s *UserService) Register(ctx context.Context, req *RegisterRequest) (*LoginResponse, error) {
	// 检查用户名是否存在
	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.exists_by_username")
	}
	if exists {
		return nil, apperror.ErrResourceExists.WithMessage("Username already exists")
	}

	// 检查邮箱是否存在
	exists, err = s.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.exists_by_email")
	}
	if exists {
		return nil, apperror.ErrResourceExists.WithMessage("Email already exists")
	}

	// 创建用户
	user := &model.User{
		Username: req.Username,
		Password: hashPassword(req.Password),
		Email:    req.Email,
		Nickname: req.Nickname,
		Status:   1,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.create")
	}

	// 生成token
	token, expiresAt, err := s.jwt.GenerateToken(user.ID, user.OrgID)
	if err != nil {
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.generate_token")
	}

	return &LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      s.toUserInfo(user),
	}, nil
}

// Login 用户登录
func (s *UserService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// 获取用户
	user, err := s.repo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrUnauthorized.WithMessage("Invalid username or password")
		}
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.get_by_username")
	}

	// 验证密码
	if !verifyPassword(user.Password, req.Password) {
		return nil, apperror.ErrUnauthorized.WithMessage("Invalid username or password")
	}

	// 检查状态
	if user.Status != 1 {
		return nil, apperror.ErrForbidden.WithMessage("User account is disabled")
	}

	// 更新登录时间
	user.LastLoginAt = time.Now()
	_ = s.repo.Update(ctx, user)

	// 生成token
	token, expiresAt, err := s.jwt.GenerateToken(user.ID, user.OrgID)
	if err != nil {
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.generate_token")
	}

	return &LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      s.toUserInfo(user),
	}, nil
}

// GetProfile 获取用户资料
func (s *UserService) GetProfile(ctx context.Context, userID int64) (*UserInfo, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrNotFound.WithMessage("User not found")
		}
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.get_by_id")
	}

	return s.toUserInfo(user), nil
}

// UpdateProfile 更新用户资料
type UpdateProfileRequest struct {
	Nickname string `json:"nickname" binding:"min=2,max=20"`
	Avatar   string `json:"avatar" binding:"omitempty,url"`
}

func (s *UserService) UpdateProfile(ctx context.Context, userID int64, req *UpdateProfileRequest) (*UserInfo, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrNotFound.WithMessage("User not found")
		}
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.get_by_id")
	}

	user.Nickname = req.Nickname
	user.Avatar = req.Avatar

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("user.update")
	}

	return s.toUserInfo(user), nil
}

// toUserInfo 转换为用户信息
func (s *UserService) toUserInfo(user *model.User) *UserInfo {
	return &UserInfo{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		CreatedAt: user.CreatedAt.Unix(),
	}
}

// hashPassword 哈希密码
func hashPassword(password string) string {
	// TODO: 实现密码哈希
	return password
}

// verifyPassword 验证密码
func verifyPassword(hashed, password string) bool {
	// TODO: 实现密码验证
	return hashed == password
}
```

### 16.2 用户Handler

```go
// internal/user/handler/user_handler.go
package handler

import (
	"github.com/gin-gonic/gin"
	"your-project/internal/user/service"
	"your-project/pkg/apperror"
	"your-project/pkg/response"
)

// UserHandler 用户处理器
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler 创建用户处理器
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// Register 注册
func (h *UserHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Invalid parameter: "+err.Error()))
		return
	}

	result, err := h.userService.Register(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// Login 登录
func (h *UserHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Invalid parameter"))
		return
	}

	result, err := h.userService.Login(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// GetProfile 获取资料
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	profile, err := h.userService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, profile)
}

// UpdateProfile 更新资料
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req service.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Invalid parameter: "+err.Error()))
		return
	}

	profile, err := h.userService.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, profile)
}
```

---

## 17. 完整的服务启动流程

### 17.1 Game Service启动

```go
// cmd/game-service/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"your-project/internal/actor"
	"your-project/internal/engine"
	"your-project/internal/game/handler"
	"your-project/internal/game/repository"
	"your-project/internal/game/service"
	"your-project/internal/middleware"
	"your-project/pkg/config"
	"your-project/pkg/database"
	"your-project/pkg/logger"
	"your-project/pkg/metrics"
	"your-project/pkg/queue"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.prod.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// 初始化日志
	if err := logger.Init(&cfg.Log); err != nil {
		log.Fatalf("init logger failed: %v", err)
	}

	logger.Info("game-service starting...")

	// 初始化数据库
	dbManager := database.NewDBManager()
	if err := dbManager.Register("game_main_db", &database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		DBName:          "game_main_db",
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	}); err != nil {
		logger.Fatal("register database failed", "error", err)
	}
	defer dbManager.Close()

	db, err := dbManager.Get("game_main_db")
	if err != nil {
		logger.Fatal("get database failed", "error", err)
	}

	// 初始化Redis
	redisClient, err := cache.NewRedisClient(&cache.Config{
		Host:         cfg.Redis.Host,
		Port:         cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err != nil {
		logger.Fatal("init redis failed", "error", err)
	}
	defer redisClient.Close()

	// 初始化Kafka
	kafkaClient := queue.NewKafkaClient(&queue.Config{
		Brokers: cfg.Kafka.Brokers,
		GroupID: cfg.Kafka.GroupID,
	})
	defer kafkaClient.Close()

	// 初始化Actor系统
	actorSystem := actor.NewActorSystem()
	if err := actorSystem.Start(); err != nil {
		logger.Fatal("start actor system failed", "error", err)
	}
	defer actorSystem.Stop()

	// 初始化Repository
	gameRepo := repository.NewGameRepository(db)
	roomRepo := repository.NewRoomRepository(db)

	// 初始化Service
	gameService := service.NewGameService(gameRepo, roomRepo, actorSystem, redisClient)
	playerService := service.NewPlayerService(roomRepo, actorSystem)

	// 启动gRPC服务器
	server := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.GRPCLogging()),
		grpc.UnaryInterceptor(middleware.GRPCRecovery()),
		grpc.UnaryInterceptor(middleware.GRPCMetrics()),
	)

	// 注册服务
	RegisterGameServiceServer(server, gameService)
	RegisterPlayerServiceServer(server, playerService)

	// 注册反射
	reflection.Register(server)

	// 启动监听
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		logger.Fatal("listen failed", "error", err)
	}

	// 启动指标服务器
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		logger.Info("metrics server starting", "addr", ":9090")
		if err := http.ListenAndServe(":9090", mux); err != nil {
			logger.Error("metrics server failed", "error", err)
		}
	}()

	// 启动服务器
	go func() {
		logger.Info("game-service started", "port", cfg.Server.Port)
		if err := server.Serve(lis); err != nil {
			logger.Fatal("serve failed", "error", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("game-service shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		logger.Info("game-service stopped")
	case <-ctx.Done():
		logger.Warn("shutdown timeout, forcing stop")
		server.Stop()
	}
}
```

### 17.2 WebSocket Gateway启动

```go
// cmd/ws-gateway/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"your-project/internal/ws"
	"your-project/pkg/cache"
	"your-project/pkg/config"
	"your-project/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.prod.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// 初始化日志
	if err := logger.Init(&cfg.Log); err != nil {
		log.Fatalf("init logger failed: %v", err)
	}

	logger.Info("ws-gateway starting...")

	// 初始化Redis
	redisClient, err := cache.NewRedisClient(&cache.Config{
		Host:         cfg.Redis.Host,
		Port:         cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
	})
	if err != nil {
		logger.Fatal("init redis failed", "error", err)
	}
	defer redisClient.Close()

	// 初始化Hub
	hub := ws.NewHub(redisClient)
	go hub.Run()

	// 创建路由
	r := gin.Default()
	r.Use(middleware.CORS())
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.Use(middleware.Logging())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})

	// WebSocket端点
	wsHandler := ws.NewHandler(hub)
	r.GET("/ws", wsHandler.HandleWebSocket)

	// 启动服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: r,
	}

	go func() {
		logger.Info("ws-gateway started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen failed", "error", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("ws-gateway shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("shutdown failed", "error", err)
	}

	logger.Info("ws-gateway exited")
}
```

---

## 18. Payment Service实现

### 18.1 支付服务结构

```go
// internal/payment/service/payment_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"your-project/internal/payment/model"
	"your-project/internal/payment/repository"
	"your-project/pkg/apperror"
)

// PaymentService 支付服务
type PaymentService struct {
	orderRepo   repository.OrderRepository
	scoreRepo   repository.ScoreRepository
	redis       *cache.RedisClient
	kafka       *queue.KafkaClient
}

// NewPaymentService 创建支付服务
func NewPaymentService(
	orderRepo repository.OrderRepository,
	scoreRepo repository.ScoreRepository,
	redis *cache.RedisClient,
	kafka *queue.KafkaClient,
) *PaymentService {
	return &PaymentService{
		orderRepo: orderRepo,
		scoreRepo: scoreRepo,
		redis:     redis,
		kafka:     kafka,
	}
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	UserID    int64  `json:"user_id" binding:"required"`
	ProductID string `json:"product_id" binding:"required"`
	Amount    int64  `json:"amount" binding:"required,min=1"`
	Currency  string `json:"currency" binding:"required"`
}

// CreateOrderResponse 创建订单响应
type CreateOrderResponse struct {
	OrderID     string `json:"order_id"`
	OrderNo     string `json:"order_no"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
}

// CreateOrder 创建订单
func (s *PaymentService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	// 生成订单号
	orderNo := generateOrderNo()

	// 创建订单
	order := &model.Order{
		OrderNo:     orderNo,
		UserID:     req.UserID,
		ProductID:   req.ProductID,
		Amount:     req.Amount,
		Currency:    req.Currency,
		Status:     "pending",
		ExpiredAt:  time.Now().Add(30 * time.Minute),
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("order.create")
	}

	// 发送订单创建事件
	s.publishOrderEvent(ctx, "order.created", order)

	return &CreateOrderResponse{
		OrderID:   fmt.Sprintf("%d", order.ID),
		OrderNo:   orderNo,
		Amount:    order.Amount,
		Currency:  order.Currency,
		Status:    order.Status,
		CreatedAt: order.CreatedAt.Unix(),
	}, nil
}

// ProcessPayment 处理支付
func (s *PaymentService) ProcessPayment(ctx context.Context, orderID string, paymentMethod string, paymentData map[string]interface{}) error {
	// 获取订单
	order, err := s.orderRepo.GetByOrderNo(ctx, orderID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperror.ErrNotFound.WithMessage("Order not found")
		}
		return apperror.ErrInternal.Wrap(err).WithOperation("order.get")
	}

	// 检查订单状态
	if order.Status != "pending" {
		return apperror.ErrInvalidState.WithMessage(fmt.Sprintf("Order status is %s", order.Status))
	}

	// 检查订单是否过期
	if time.Now().After(order.ExpiredAt) {
		order.Status = "expired"
		_ = s.orderRepo.Update(ctx, order)
		return apperror.ErrInvalidState.WithMessage("Order has expired")
	}

	// 处理支付
	switch paymentMethod {
	case "alipay":
		return s.processAlipay(ctx, order, paymentData)
	case "wechat":
		return s.processWechat(ctx, order, paymentData)
	case "score":
		return s.processScorePayment(ctx, order, paymentData)
	default:
		return apperror.ErrBadRequest.WithMessage("Unsupported payment method")
	}
}

// processScorePayment 积分支付
func (s *PaymentService) processScorePayment(ctx context.Context, order *model.Order, data map[string]interface{}) error {
	userID := order.UserID
	amount := order.Amount

	// 获取用户积分
	score, err := s.scoreRepo.GetByUserID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperror.ErrForbidden.WithMessage("Insufficient score balance")
		}
		return apperror.ErrInternal.Wrap(err).WithOperation("score.get")
	}

	// 检查积分余额
	if score.Balance < amount {
		return apperror.ErrForbidden.WithMessage("Insufficient score balance")
	}

	// 扣除积分
	if err := s.scoreRepo.AddBalance(ctx, userID, -amount); err != nil {
		return apperror.ErrInternal.Wrap(err).WithOperation("score.deduct")
	}

	// 更新订单状态
	order.Status = "paid"
	order.PaidAt = time.Now()
	order.PaymentMethod = "score"

	if err := s.orderRepo.Update(ctx, order); err != nil {
		return apperror.ErrInternal.Wrap(err).WithOperation("order.update")
	}

	// 记录积分变动
	_ = s.scoreRepo.CreateLog(ctx, &model.ScoreLog{
		UserID:  userID,
		Amount:  -amount,
		Type:    "consume",
		OrderID:  order.ID,
		Remark:  fmt.Sprintf("支付订单 %s", order.OrderNo),
	})

	// 发送支付完成事件
	s.publishOrderEvent(ctx, "order.paid", order)

	return nil
}

// QueryOrder 查询订单
func (s *PaymentService) QueryOrder(ctx context.Context, orderNo string) (*model.Order, error) {
	order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrNotFound.WithMessage("Order not found")
		}
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("order.get")
	}

	// 如果是外部支付，查询第三方状态
	if order.Status == "pending" && order.PaymentMethod != "score" {
		s.syncPaymentStatus(ctx, order)
	}

	return order, nil
}

// RefundOrder 退款
func (s *PaymentService) RefundOrder(ctx context.Context, orderNo string, reason string) error {
	order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperror.ErrNotFound.WithMessage("Order not found")
		}
		return apperror.ErrInternal.Wrap(err).WithOperation("order.get")
	}

	// 检查订单状态
	if order.Status != "paid" {
		return apperror.ErrInvalidState.WithMessage("Order can only be refunded when paid")
	}

	// 检查是否已退款
	if order.Status == "refunded" {
		return apperror.ErrInvalidState.WithMessage("Order already refunded")
	}

	// 处理退款
	switch order.PaymentMethod {
	case "score":
		// 退还积分
		if err := s.scoreRepo.AddBalance(ctx, order.UserID, order.Amount); err != nil {
			return apperror.ErrInternal.Wrap(err).WithOperation("score.refund")
		}

		// 记录积分变动
		_ = s.scoreRepo.CreateLog(ctx, &model.ScoreLog{
			UserID:  order.UserID,
			Amount:  order.Amount,
			Type:    "refund",
			OrderID:  order.ID,
			Remark:  fmt.Sprintf("退款订单 %s: %s", order.OrderNo, reason),
		})

	default:
		return apperror.ErrBadRequest.WithMessage("Refund not supported for this payment method")
	}

	// 更新订单状态
	order.Status = "refunded"
	order.RefundedAt = time.Now()
	order.RefundReason = reason

	if err := s.orderRepo.Update(ctx, order); err != nil {
		return apperror.ErrInternal.Wrap(err).WithOperation("order.update")
	}

	// 发送退款事件
	s.publishOrderEvent(ctx, "order.refunded", order)

	return nil
}

// publishOrderEvent 发布订单事件
func (s *PaymentService) publishOrderEvent(ctx context.Context, eventType string, order *model.Order) {
	event := map[string]interface{}{
		"type":      eventType,
		"order_id":  order.ID,
		"order_no":  order.OrderNo,
		"user_id":   order.UserID,
		"amount":    order.Amount,
		"status":    order.Status,
		"timestamp": time.Now().Unix(),
	}

	data, _ := json.Marshal(event)
	_ = s.kafka.Publish(ctx, "order.events", data)
}

// generateOrderNo 生成订单号
func generateOrderNo() string {
	return fmt.Sprintf("ORD%s%s", time.Now().Format("20060102150405"), randomString(8))
}
```

### 18.2 积分服务

```go
// internal/payment/service/score_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"your-project/internal/payment/model"
	"your-project/internal/payment/repository"
	"your-project/pkg/apperror"
)

// ScoreService 积分服务
type ScoreService struct {
	scoreRepo repository.ScoreRepository
	redis     *cache.RedisClient
}

// NewScoreService 创建积分服务
func NewScoreService(
	scoreRepo repository.ScoreRepository,
	redis *cache.RedisClient,
) *ScoreService {
	return &ScoreService{
		scoreRepo: scoreRepo,
		redis:     redis,
	}
}

// GetBalance 获取积分余额
func (s *ScoreService) GetBalance(ctx context.Context, userID int64) (int64, error) {
	// 先从缓存获取
	cacheKey := fmt.Sprintf("score:%d", userID)
	if balance, err := s.redis.Get(ctx, cacheKey); err == nil {
		var b int64
		if fmt.Sscan(balance, &b) == 1 {
			return b, nil
		}
	}

	// 从数据库获取
	score, err := s.scoreRepo.GetByUserID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新账户
			score = &model.Score{
				UserID:  userID,
				Balance: 0,
			}
			_ = s.scoreRepo.Create(ctx, score)
		} else {
			return 0, apperror.ErrInternal.Wrap(err).WithOperation("score.get")
		}
	}

	// 写入缓存
	_ = s.redis.Set(ctx, cacheKey, fmt.Sprintf("%d", score.Balance), 5*time.Minute)

	return score.Balance, nil
}

// AddBalance 增加积分
func (s *ScoreService) AddBalance(ctx context.Context, userID int64, amount int64, remark string) error {
	if amount == 0 {
		return nil
	}

	// 获取当前积分
	score, err := s.scoreRepo.GetByUserID(ctx, userID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return apperror.ErrInternal.Wrap(err).WithOperation("score.get")
	}

	// 更新积分
	if score == nil {
		score = &model.Score{
			UserID:  userID,
			Balance: amount,
		}
		if err := s.scoreRepo.Create(ctx, score); err != nil {
			return apperror.ErrInternal.Wrap(err).WithOperation("score.create")
		}
	} else {
		if err := s.scoreRepo.AddBalance(ctx, userID, amount); err != nil {
			return apperror.ErrInternal.Wrap(err).WithOperation("score.add")
		}
	}

	// 记录日志
	_ = s.scoreRepo.CreateLog(ctx, &model.ScoreLog{
		UserID:  userID,
		Amount:  amount,
		Type:    getLogType(amount),
		Remark:  remark,
	})

	// 清除缓存
	cacheKey := fmt.Sprintf("score:%d", userID)
	_ = s.redis.Del(ctx, cacheKey)

	return nil
}

// GetScoreLog 获取积分日志
func (s *ScoreService) GetScoreLog(ctx context.Context, userID int64, page, pageSize int) ([]*model.ScoreLog, int64, error) {
	return s.scoreRepo.GetLogsByUserID(ctx, userID, page, pageSize)
}

// getLogType 获取日志类型
func getLogType(amount int64) string {
	if amount > 0 {
		return "income"
	}
	return "expense"
}
```

### 18.3 支付Handler

```go
// internal/payment/handler/payment_handler.go
package handler

import (
	"github.com/gin-gonic/gin"
	"your-project/internal/payment/service"
	"your-project/pkg/apperror"
	"your-project/pkg/response"
)

// PaymentHandler 支付处理器
type PaymentHandler struct {
	paymentService *service.PaymentService
	scoreService   *service.ScoreService
}

// NewPaymentHandler 创建支付处理器
func NewPaymentHandler(
	paymentService *service.PaymentService,
	scoreService *service.ScoreService,
) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		scoreService:   scoreService,
	}
}

// CreateOrder 创建订单
func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	var req service.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Invalid parameter: "+err.Error()))
		return
	}

	// 设置用户ID
	if req.UserID == 0 {
		req.UserID = c.GetInt64("user_id")
	}

	result, err := h.paymentService.CreateOrder(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// ProcessPayment 处理支付
func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	var req struct {
		OrderNo       string                 `json:"order_no" binding:"required"`
		PaymentMethod string                 `json:"payment_method" binding:"required"`
		PaymentData   map[string]interface{} `json:"payment_data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Invalid parameter"))
		return
	}

	if err := h.paymentService.ProcessPayment(c.Request.Context(), req.OrderNo, req.PaymentMethod, req.PaymentData); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"status": "processing"})
}

// QueryOrder 查询订单
func (h *PaymentHandler) QueryOrder(c *gin.Context) {
	orderNo := c.Query("order_no")
	if orderNo == "" {
		response.Error(c, apperror.ErrBadRequest.WithMessage("order_no is required"))
		return
	}

	order, err := h.paymentService.QueryOrder(c.Request.Context(), orderNo)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, order)
}

// RefundOrder 退款
func (h *PaymentHandler) RefundOrder(c *gin.Context) {
	var req struct {
		OrderNo string `json:"order_no" binding:"required"`
		Reason  string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrBadRequest.WithMessage("Invalid parameter"))
		return
	}

	if err := h.paymentService.RefundOrder(c.Request.Context(), req.OrderNo, req.Reason); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"status": "refunded"})
}

// GetBalance 获取积分余额
func (h *PaymentHandler) GetBalance(c *gin.Context) {
	userID := c.GetInt64("user_id")

	balance, err := h.scoreService.GetBalance(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"balance": balance})
}

// GetScoreLog 获取积分日志
func (h *PaymentHandler) GetScoreLog(c *gin.Context) {
	userID := c.GetInt64("user_id")
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")

	// 解析分页参数
	p, _ := strconv.Atoi(page)
	ps, _ := strconv.Atoi(pageSize)

	logs, total, err := h.scoreService.GetScoreLog(c.Request.Context(), userID, p, ps)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{
		"logs":  logs,
		"total": total,
		"page":  p,
		"page_size": ps,
	})
}
```

---

## 19. Notification Service实现

### 19.1 通知服务结构

```go
// internal/notification/service/notification_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"your-project/internal/notification/model"
	"your-project/internal/notification/repository"
	"your-project/pkg/cache"
	"your-project/pkg/queue"
)

// NotificationService 通知服务
type NotificationService struct {
	repo      repository.NotificationRepository
	redis     *cache.RedisClient
	kafka     *queue.KafkaClient
	emailSender *EmailSender
	smsSender   *SMSSender
	pushSender  *PushSender
}

// NewNotificationService 创建通知服务
func NewNotificationService(
	repo repository.NotificationRepository,
	redis *cache.RedisClient,
	kafka *queue.KafkaClient,
	email *EmailSender,
	sms *SMSSender,
	push *PushSender,
) *NotificationService {
	return &NotificationService{
		repo:        repo,
		redis:       redis,
		kafka:       kafka,
		emailSender: email,
		smsSender:   sms,
		pushSender:  push,
	}
}

// SendRequest 发送请求
type SendRequest struct {
	Type     string   `json:"type" binding:"required"` // email, sms, push, in_app
	UserIDs  []int64  `json:"user_ids"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Data     map[string]interface{} `json:"data"`
}

// Send 发送通知
func (s *NotificationService) Send(ctx context.Context, req *SendRequest) error {
	// 创建通知记录
	notification := &model.Notification{
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
		Data:    req.Data,
		Status:  "sending",
	}

	if err := s.repo.Create(ctx, notification); err != nil {
		return err
	}

	// 根据类型发送
	switch req.Type {
	case "email":
		go s.sendEmail(notification, req.UserIDs)
	case "sms":
		go s.sendSMS(notification, req.UserIDs)
	case "push":
		go s.sendPush(notification, req.UserIDs)
	case "in_app":
		go s.sendInApp(ctx, notification, req.UserIDs)
	default:
		return fmt.Errorf("unknown notification type: %s", req.Type)
	}

	return nil
}

// sendEmail 发送邮件
func (s *NotificationService) sendEmail(notification *model.Notification, userIDs []int64) {
	for _, userID := range userIDs {
		// 获取用户邮箱
		email, err := s.getUserEmail(userID)
		if err != nil {
			continue
		}

		// 发送邮件
		if err := s.emailSender.Send(email, notification.Title, notification.Content); err != nil {
			// 更新状态为失败
			_ = s.repo.UpdateStatus(context.Background(), notification.ID, "failed")
			continue
		}
	}

	// 更新状态为已发送
	_ = s.repo.UpdateStatus(context.Background(), notification.ID, "sent")
}

// sendSMS 发送短信
func (s *NotificationService) sendSMS(notification *model.Notification, userIDs []int64) {
	for _, userID := range userIDs {
		// 获取用户手机号
		phone, err := s.getUserPhone(userID)
		if err != nil {
			continue
		}

		// 发送短信
		if err := s.smsSender.Send(phone, notification.Content); err != nil {
			continue
		}
	}

	_ = s.repo.UpdateStatus(context.Background(), notification.ID, "sent")
}

// sendPush 发送推送
func (s *NotificationService) sendPush(notification *model.Notification, userIDs []int64) {
	for _, userID := range userIDs {
		// 获取用户设备token
		tokens, err := s.getUserDeviceTokens(userID)
		if err != nil {
			continue
		}

		// 发送推送
		for _, token := range tokens {
			_ = s.pushSender.Send(token, notification.Title, notification.Content, notification.Data)
		}
	}

	_ = s.repo.UpdateStatus(context.Background(), notification.ID, "sent")
}

// sendInApp 发送站内通知
func (s *NotificationService) sendInApp(ctx context.Context, notification *model.Notification, userIDs []int64) {
	for _, userID := range userIDs {
		// 创建接收记录
		receipt := &model.NotificationReceipt{
			NotificationID: notification.ID,
			UserID:         userID,
			IsRead:         false,
		}
		_ = s.repo.CreateReceipt(ctx, receipt)

		// 通过WebSocket推送
		s.sendToUser(ctx, userID, notification)
	}

	_ = s.repo.UpdateStatus(ctx, notification.ID, "sent")
}

// sendToUser 通过WebSocket发送给用户
func (s *NotificationService) sendToUser(ctx context.Context, userID int64, notification *model.Notification) {
	// 发布到Redis频道
	channel := fmt.Sprintf("user:%d:notification", userID)
	message := map[string]interface{}{
		"type":    "notification",
		"title":   notification.Title,
		"content": notification.Content,
		"data":    notification.Data,
	}
	data, _ := json.Marshal(message)
	_ = s.redis.Publish(ctx, channel, data)
}

// MarkAsRead 标记为已读
func (s *NotificationService) MarkAsRead(ctx context.Context, userID int64, notificationID int64) error {
	return s.repo.MarkAsRead(ctx, userID, notificationID)
}

// GetUnreadCount 获取未读数量
func (s *NotificationService) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	return s.repo.GetUnreadCount(ctx, userID)
}

// ListNotifications 列出通知
func (s *NotificationService) ListNotifications(ctx context.Context, userID int64, page, pageSize int) ([]*model.Notification, int64, error) {
	return s.repo.ListByUserID(ctx, userID, page, pageSize)
}
```

### 19.2 邮件/短信/推送发送器

```go
// pkg/notification/email.go
package notification

import (
	"net/smtp"
	"strings"
)

// EmailSender 邮件发送器
type EmailSender struct {
	host     string
	port     int
	username string
	password string
	from     string
}

// NewEmailSender 创建邮件发送器
func NewEmailSender(host string, port int, username, password, from string) *EmailSender {
	return &EmailSender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// Send 发送邮件
func (s *EmailSender) Send(to, subject, body string) error {
	// 构建邮件内容
	msg := strings.NewReader(fmt.Sprintf(
		"To: %s\r\nSubject: %s\r\n\r\n%s",
		to, subject, body,
	))

	// 发送邮件
	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	return smtp.SendMail(
		fmt.Sprintf("%s:%d", s.host, s.port),
		auth,
		s.from,
		[]string{to},
		msg,
	)
}

// pkg/notification/sms.go
package notification

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SMSSender 短信发送器
type SMSSender struct {
	accessKey string
	secretKey string
	signName  string
	template  string
}

// NewSMSSender 创建短信发送器
func NewSMSSender(accessKey, secretKey, signName, template string) *SMSSender {
	return &SMSSender{
		accessKey: accessKey,
		secretKey: secretKey,
		signName:  signName,
		template:  template,
	}
}

// Send 发送短信
func (s *SMSSender) Send(phone, content string) error {
	// 调用阿里云短信API
	// TODO: 实现具体的短信发送逻辑
	return nil
}

// pkg/notification/push.go
package notification

import (
	"context"

	"firebase.google.com/go/v4/messaging"
)

// PushSender 推送发送器
type PushSender struct {
	client *messaging.Client
}

// NewPushSender 创建推送发送器
func NewPushSender(credentialsPath string) (*PushSender, error) {
	client, err := messaging.NewClient(context.Background(), messaging.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, err
	}

	return &PushSender{client: client}
}

// Send 发送推送
func (s *PushSender) Send(token, title, body string, data map[string]interface{}) error {
	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	_, err := s.client.Send(context.Background(), message)
	return err
}

// SendMulticast 批量发送
func (s *PushSender) SendMulticast(tokens []string, title, body string, data map[string]interface{}) error {
	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	_, err := s.client.SendMulticast(context.Background(), message)
	return err
}
```

---

## 20. Player Service实现

```go
// internal/player/service/player_service.go
package service

import (
	"context"
	"fmt"

	"your-project/internal/actor"
	"your-project/internal/player/model"
	"your-project/internal/player/repository"
	"your-project/pkg/apperror"
)

// PlayerService 玩家服务
type PlayerService struct {
	playerRepo repository.PlayerRepository
	actorSystem *actor.ActorSystem
}

// NewPlayerService 创建玩家服务
func NewPlayerService(
	playerRepo repository.PlayerRepository,
	actorSystem *actor.ActorSystem,
) *PlayerService {
	return &PlayerService{
		playerRepo:  playerRepo,
		actorSystem: actorSystem,
	}
}

// GetPlayer 获取玩家信息
func (s *PlayerService) GetPlayer(ctx context.Context, playerID int64) (*model.Player, error) {
	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return nil, apperror.ErrNotFound.WithMessage("Player not found")
	}

	return player, nil
}

// GetPlayerByGame 获取游戏中的玩家
func (s *PlayerService) GetPlayerByGame(ctx context.Context, userID, gameID int64) (*model.Player, error) {
	player, err := s.playerRepo.GetByUserAndGame(ctx, userID, gameID)
	if err != nil {
		return nil, apperror.ErrNotFound.WithMessage("Player not found in this game")
	}

	return player, nil
}

// UpdatePlayerState 更新玩家状态
func (s *PlayerService) UpdatePlayerState(ctx context.Context, playerID int64, state string, data map[string]interface{}) error {
	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return apperror.ErrNotFound.WithMessage("Player not found")
	}

	player.State = state
	player.StateData = data

	if err := s.playerRepo.Update(ctx, player); err != nil {
		return apperror.ErrInternal.Wrap(err).WithOperation("player.update")
	}

	// 通知PlayerActor
	actorID := fmt.Sprintf("player:%d", playerID)
	if actor, err := s.actorSystem.GetActor(actorID); err == nil {
		actor.Send(&actor.StateUpdateMessage{
			State: state,
			Data:  data,
		})
	}

	return nil
}

// UpdatePlayerScore 更新玩家分数
func (s *PlayerService) UpdatePlayerScore(ctx context.Context, playerID int64, score int64) error {
	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return apperror.ErrNotFound.WithMessage("Player not found")
	}

	player.Score += score

	if err := s.playerRepo.Update(ctx, player); err != nil {
		return apperror.ErrInternal.Wrap(err).WithOperation("player.update")
	}

	return nil
}

// GetRanking 获取排行榜
func (s *PlayerService) GetRanking(ctx context.Context, gameID int64, limit int) ([]*model.Player, error) {
	players, err := s.playerRepo.GetTopByGame(ctx, gameID, limit)
	if err != nil {
		return nil, apperror.ErrInternal.Wrap(err).WithOperation("player.get_ranking")
	}

	return players, nil
}
```

---

## 21. JWT工具实现

```go
// pkg/jwt/jwt.go
package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT声明
type Claims struct {
	UserID int64    `json:"user_id"`
	OrgID  int64    `json:"org_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

// Manager JWT管理器
type Manager struct {
	secret     string
	expireTime time.Duration
	issuer     string
}

// NewManager 创建JWT管理器
func NewManager(secret string, expireTime time.Duration, issuer string) *Manager {
	return &Manager{
		secret:     secret,
		expireTime: expireTime,
		issuer:     issuer,
	}
}

// GenerateToken 生成Token
func (m *Manager) GenerateToken(userID, orgID int64) (string, int64, error) {
	now := time.Now()
	expiresAt := now.Add(m.expireTime)

	claims := &Claims{
		UserID: userID,
		OrgID:  orgID,
		Roles:  []string{"user"},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.secret))
	if err != nil {
		return "", 0, fmt.Errorf("sign token failed: %w", err)
	}

	return tokenString, expiresAt.Unix(), nil
}

// ParseToken 解析Token
func (m *Manager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token failed: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// RefreshToken 刷新Token
func (m *Manager) RefreshToken(tokenString string) (string, int64, error) {
	claims, err := m.ParseToken(tokenString)
	if err != nil {
		return "", 0, err
	}

	// 生成新token
	return m.GenerateToken(claims.UserID, claims.OrgID)
}
```

---

## 22. 测试用例

### 22.1 Actor测试

```go
// internal/actor/actor_test.go
package actor_test

import (
	"context"
	"testing"
	"time"

	"your-project/internal/actor"
	"your-project/pkg/assert"
)

// MockActorHandler 模拟Actor处理器
type MockActorHandler struct {
	received []actor.Message
}

func (m *MockActorHandler) Receive(ctx context.Context, msg actor.Message) error {
	m.received = append(m.received, msg)
	return nil
}

// TestActorSendReceive 测试Actor发送接收
func TestActorSendReceive(t *testing.T) {
	handler := &MockActorHandler{}
	act := actor.NewBaseActor("test-actor", "test", 10, handler)

	ctx := context.Background()
	if err := act.Start(ctx); err != nil {
		t.Fatalf("start actor failed: %v", err)
	}
	defer act.Stop()

	// 发送消息
	msg := &TestMessage{Data: "test"}
	if err := act.Send(msg); err != nil {
		t.Fatalf("send message failed: %v", err)
	}

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	// 验证接收
	assert.Equal(t, 1, len(handler.received))
	assert.Equal(t, "test", handler.received[0].(*TestMessage).Data)
}

// TestActorStats 测试Actor统计
func TestActorStats(t *testing.T) {
	handler := &MockActorHandler{}
	act := actor.NewBaseActor("test-actor", "test", 10, handler)

	ctx := context.Background()
	act.Start(ctx)
	defer act.Stop()

	// 发送多条消息
	for i := 0; i < 5; i++ {
		_ = act.Send(&TestMessage{Data: fmt.Sprintf("test-%d", i)})
	}

	time.Sleep(200 * time.Millisecond)

	stats := act.Stats()
	assert.Equal(t, int64(5), stats.MessageCount)
	assert.Equal(t, int64(0), stats.ErrorCount)
}

// TestMessage 类型定义
type TestMessage struct {
	Data string
}

func (m *TestMessage) Type() string {
	return "test"
}
```

### 22.2 Service测试

```go
// internal/user/service/user_service_test.go
package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"your-project/internal/user/model"
	"your-project/internal/user/repository"
	"your-project/internal/user/service"
	"your-project/pkg/jwt"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&model.User{})
	require.NoError(t, err)

	return db
}

func TestUserService_Register(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewUserRepository(db)
	jwtManager := jwt.NewManager("test-secret", 24*time.Hour, "test")

	userService := service.NewUserService(repo, jwtManager, nil)

	ctx := context.Background()

	// 测试注册
	req := &service.RegisterRequest{
		Username: "testuser",
		Password: "password123",
		Email:    "test@example.com",
		Nickname: "Test User",
	}

	result, err := userService.Register(ctx, req)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	assert.NotNil(t, result.User)
	assert.Equal(t, "testuser", result.User.Username)

	// 测试重复注册
	_, err = userService.Register(ctx, req)
	assert.Error(t, err)
}

func TestUserService_Login(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewUserRepository(db)
	jwtManager := jwt.NewManager("test-secret", 24*time.Hour, "test")

	userService := service.NewUserService(repo, jwtManager, nil)

	ctx := context.Background()

	// 先注册
	req := &service.RegisterRequest{
		Username: "testuser",
		Password: "password123",
		Email:    "test@example.com",
		Nickname: "Test User",
	}
	_, err := userService.Register(ctx, req)
	require.NoError(t, err)

	// 测试登录
	loginReq := &service.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}

	result, err := userService.Login(ctx, loginReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	assert.Equal(t, "testuser", result.User.Username)

	// 测试错误密码
	loginReq.Password = "wrongpassword"
	_, err = userService.Login(ctx, loginReq)
	assert.Error(t, err)
}
```

### 22.3 集成测试

```go
// tests/integration/game_service_test.go
package integration_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"your-project/api/proto/game/v1"
	"your-project/internal/game/service"
)

func TestGameService_CreateAndJoinRoom(t *testing.T) {
	// 创建测试服务器
	lis := bufconn.Listen(testing.WithBuffSize(1024 * 1024))
	defer lis.Close()

	s := grpc.NewServer()
	gameService := service.NewGameService(nil, nil, nil, nil)

	gamev1.RegisterGameServiceServer(s, gameService)

	go s.Serve(lis)
	defer s.Stop()

	// 创建客户端
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := gamev1.NewGameServiceClient(conn)

	// 创建游戏
	createResp, err := client.CreateGame(ctx, &gamev1.CreateGameRequest{
		OrgId:    1,
		GameCode: "test-game",
		GameName: "Test Game",
		GameType: "card",
	})
	require.NoError(t, err)
	assert.NotNil(t, createResp.Game)

	// 创建房间
	roomResp, err := client.CreateRoom(ctx, &gamev1.CreateRoomRequest{
		GameId:    createResp.Game.Id,
		RoomName:  "Test Room",
		MaxPlayers: 4,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, roomResp.RoomId)

	// 加入房间
	joinResp, err := client.JoinRoom(ctx, &gamev1.JoinRoomRequest{
		RoomId:    roomResp.RoomId,
		PlayerId:  123,
		PlayerName: "Test Player",
	})
	require.NoError(t, err)
	assert.True(t, joinResp.Success)
}
```

---

## 23. 部署脚本

### 23.1 构建脚本

```bash
#!/bin/bash
# scripts/build.sh

set -e

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Building all services...${NC}"

# 服务列表
SERVICES=("gateway" "ws-gateway" "game-service" "user-service" "payment-service")

# 构建函数
build_service() {
    SERVICE_NAME=$1
    echo -e "${GREEN}Building ${SERVICE_NAME}...${NC}"

    cd cmd/${SERVICE_NAME}
    go build -o ../../bin/${SERVICE_NAME} .
    cd ../..
}

# 并行构建
for SERVICE in "${SERVICES[@]}"; do
    build_service "$SERVICE" &
done

# 等待所有构建完成
wait

echo -e "${GREEN}All services built successfully!${NC}"
echo "Binaries are in ./bin/"
```

### 23.2 运行脚本

```bash
#!/bin/bash
# scripts/run.sh

set -e

SERVICE=$1
if [ -z "$SERVICE" ]; then
    echo "Usage: ./run.sh <service-name>"
    echo "Available services: gateway, ws-gateway, game-service, user-service, payment-service"
    exit 1
fi

echo "Starting ${SERVICE}..."

# 检查二进制文件
if [ ! -f "bin/${SERVICE}" ]; then
    echo "Binary not found. Please run ./scripts/build.sh first"
    exit 1
fi

# 加载配置
CONFIG_FILE="configs/config.dev.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    CONFIG_FILE="configs/config.prod.yaml"
fi

# 启动服务
./bin/${SERVICE} --config=$CONFIG_FILE
```

### 23.3 数据库迁移脚本

```bash
#!/bin/bash
# scripts/migrate.sh

set -e

ACTION=$1
if [ -z "$ACTION" ]; then
    echo "Usage: ./migrate.sh <up|down|create|drop>"
    exit 1
fi

# 数据库配置
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-}
DB_NAME=${DB_NAME:-game_db}

case $ACTION in
    "create")
        echo "Creating database ${DB_NAME}..."
        PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "CREATE DATABASE ${DB_NAME};"
        ;;
    "drop")
        echo "Dropping database ${DB_NAME}..."
        PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "DROP DATABASE IF EXISTS ${DB_NAME};"
        ;;
    "up")
        echo "Running migrations..."
        go run cmd/migrate/main.go up
        ;;
    "down")
        echo "Rolling back migrations..."
        go run cmd/migrate/main.go down
        ;;
    *)
        echo "Unknown action: $ACTION"
        exit 1
        ;;
esac
```

### 23.4 部署脚本

```bash
#!/bin/bash
# scripts/deploy.sh

set -e

ENV=${1:-dev}
SERVICE=$2

if [ -z "$SERVICE" ]; then
    echo "Usage: ./deploy.sh <env> <service>"
    exit 1
fi

echo "Deploying ${SERVICE} to ${ENV}..."

# 构建
echo "Building..."
./scripts/build.sh

# Docker构建
echo "Building Docker image..."
docker build -f deployments/docker/Dockerfile.${SERVICE} -t ${SERVICE}:${ENV} .

# 推送镜像
if [ "$ENV" != "local" ]; then
    echo "Pushing image..."
    docker tag ${SERVICE}:${ENV} registry.example.com/${SERVICE}:${ENV}
    docker push registry.example.com/${SERVICE}:${ENV}
fi

# Kubernetes部署
if [ "$ENV" = "prod" ]; then
    echo "Deploying to Kubernetes..."
    kubectl apply -f deployments/k8s/${SERVICE}.yaml
    kubectl rollout restart deployment/${SERVICE}
fi

echo "Deployment complete!"
```

---

## 24. Makefile

```makefile
# Makefile

.PHONY: all build test clean run docker-build docker-up docker-down migrate migrate-up migrate-down

# 变量
BINARY_DIR=bin
SERVICES=gateway ws-gateway game-service user-service payment-service
GO=go
DOCKER_COMPOSE=docker-compose

# 默认目标
all: build

# 构建所有服务
build:
	@echo "Building all services..."
	@./scripts/build.sh

# 构建单个服务
build-%:
	@echo "Building $*..."
	@cd cmd/$* && $(GO) build -o ../../$(BINARY_DIR)/$* .

# 运行服务
run:
	@./scripts/run.sh

# 运行单个服务
run-%:
	@echo "Running $*..."
	@./$(BINARY_DIR)/$* --config=configs/config.dev.yaml

# 测试
test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# 单元测试
test-unit:
	@echo "Running unit tests..."
	$(GO) test -v -short ./...

# 集成测试
test-integration:
	@echo "Running integration tests..."
	$(GO) test -v -tags=integration ./tests/integration/...

# 清理
clean:
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html
	@$(GO) clean

# Docker构建
docker-build:
	@echo "Building Docker images..."
	@for service in $(SERVICES); do \
		docker build -f deployments/docker/Dockerfile.$$service -t $$service:latest .; \
	done

# Docker启动
docker-up:
	@echo "Starting Docker services..."
	$(DOCKER_COMPOSE) up -d

# Docker停止
docker-down:
	@echo "Stopping Docker services..."
	$(DOCKER_COMPOSE) down

# Docker日志
docker-logs:
	$(DOCKER_COMPOSE) logs -f

# 数据库迁移
migrate-create:
	@./scripts/migrate.sh create

migrate-up:
	@./scripts/migrate.sh up

migrate-down:
	@./scripts/migrate.sh down

# 代码生成
generate:
	@echo "Generating code..."
	$(GO) generate ./...

# 代码检查
lint:
	@echo "Running linters..."
	@golangci-lint run ./...

# 格式化
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# 依赖整理
tidy:
	@echo "Tidying dependencies..."
	$(GO) mod tidy

# 依赖更新
update:
	@echo "Updating dependencies..."
	$(GO) get -u ./...

# Proto生成
proto:
	@echo "Generating proto files..."
	@cd api/proto && \
		protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		*.proto

# 帮助
help:
	@echo "Available targets:"
	@echo "  all           - Build all services"
	@echo "  build         - Build all services"
	@echo "  build-%       - Build specific service"
	@echo "  run           - Run service (default: ws-gateway)"
	@echo "  run-%         - Run specific service"
	@echo "  test          - Run all tests"
	@echo "  test-unit     - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker images"
	@echo "  docker-up     - Start Docker services"
	@echo "  docker-down   - Stop Docker services"
	@echo "  migrate-create - Create database"
	@echo "  migrate-up     - Run migrations"
	@echo "  migrate-down   - Rollback migrations"
	@echo "  proto         - Generate proto files"
	@echo "  lint          - Run linters"
	@echo "  fmt           - Format code"
	@echo "  tidy          - Tidy dependencies"
```

---

## 25. API Gateway完整实现

### 25.1 Gateway结构

```go
// cmd/gateway/service.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"your-project/pkg/config"
)

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name     string
	Target   string
	Timeout  time.Duration
	MaxRetry int
}

// Gateway API网关
type Gateway struct {
	config    *config.Config
	services  map[string]*ServiceConfig
	client    *http.Client
	mu        sync.RWMutex
}

// NewGateway 创建网关
func NewGateway(cfg *config.Config) *Gateway {
	return &Gateway{
		config:   cfg,
		services: make(map[string]*ServiceConfig),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegisterService 注册服务
func (g *Gateway) RegisterService(name, target string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.services[name] = &ServiceConfig{
		Name:    name,
		Target:  target,
		Timeout: 30 * time.Second,
		MaxRetry: 3,
	}
}

// Proxy 代理中间件
func (g *Gateway) Proxy(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取服务配置
		service, err := g.getService(serviceName)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": fmt.Sprintf("service %s not found", serviceName),
			})
			c.Abort()
			return
		}

		// 构建目标URL
		targetURL, err := url.Parse(service.Target)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "invalid service target",
			})
			c.Abort()
			return
		}

		// 更新请求路径
		proxyPath := c.Request.URL.Path
		if strings.HasPrefix(proxyPath, "/api/"+serviceName) {
			proxyPath = proxyPath[len("/api/"+serviceName):]
		}

		targetURL.Path = proxyPath
		targetURL.RawQuery = c.Request.URL.RawQuery

		// 创建代理请求
	proxyReq, err := http.NewRequestWithContext(
			c.Request.Context(),
			c.Request.Method,
			targetURL.String(),
			c.Request.Body,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to create proxy request",
			})
			c.Abort()
			return
		}

		// 复制headers
		copyHeaders(c.Request.Header, proxyReq.Header)

		// 发送请求
		resp, err := g.client.Do(proxyReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": fmt.Sprintf("service unavailable: %v", err),
			})
			c.Abort()
			return
		}
		defer resp.Body.Close()

		// 复制响应headers
		copyHeaders(resp.Header, c.Writer.Header())

		// 设置状态码
		c.Status(resp.StatusCode)

		// 复制响应body
	_, err = io.Copy(c.Writer, resp.Body)
		if err != nil {
			// 已经开始写入，无法返回错误
			return
		}
	}
}

// getService 获取服务配置
func (g *Gateway) getService(name string) (*ServiceConfig, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	service, exists := g.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}

	return service, nil
}

// copyHeaders 复制headers
func copyHeaders(src, dst http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
```

### 25.2 路由配置

```go
// cmd/gateway/router.go
package main

import (
	"github.com/gin-gonic/gin"
	"your-project/internal/middleware"
)

func setupRoutes(r *gin.Engine, gateway *Gateway) {
	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"timestamp": time.Now().Unix(),
		})
	})

	// 指标端点
	r.GET("/metrics", func(c *gin.Context) {
		metrics.Handler()(c.Writer, c.Request)
	})

	// API路由
	api := r.Group("/api/v1")
	{
		// 用户服务
		user := api.Group("/user")
		user.Use(middleware.OptionalAuth())
		{
			user.POST("/register", gateway.Proxy("user-service"))
			user.POST("/login", gateway.Proxy("user-service"))
			user.GET("/profile", middleware.Auth(), gateway.Proxy("user-service"))
			user.PUT("/profile", middleware.Auth(), gateway.Proxy("user-service"))
		}

		// 游戏服务
		game := api.Group("/game")
		game.Use(middleware.Auth())
		{
			game.GET("/list", gateway.Proxy("game-service"))
			game.GET("/:id", gateway.Proxy("game-service"))
			game.POST("/rooms", gateway.Proxy("game-service"))
			game.POST("/rooms/:id/join", gateway.Proxy("game-service"))
			game.POST("/rooms/:id/leave", gateway.Proxy("game-service"))
		}

		// 支付服务
		payment := api.Group("/payment")
		payment.Use(middleware.Auth())
		{
			payment.POST("/orders", gateway.Proxy("payment-service"))
			payment.GET("/orders/:id", gateway.Proxy("payment-service"))
			payment.POST("/orders/:id/pay", gateway.Proxy("payment-service"))
			payment.POST("/orders/:id/refund", gateway.Proxy("payment-service"))
			payment.GET("/balance", gateway.Proxy("payment-service"))
		}

		// 通知服务
		notification := api.Group("/notification")
		notification.Use(middleware.Auth())
		{
			notification.GET("/list", gateway.Proxy("notification-service"))
			notification.POST("/send", gateway.Proxy("notification-service"))
			notification.PUT("/:id/read", gateway.Proxy("notification-service"))
		}
	}

	// WebSocket升级
	r.GET("/ws", func(c *gin.Context) {
		// 直接转发到WebSocket网关
		proxyWS(c)
	})
}

// proxyWS 代理WebSocket
func proxyWS(c *gin.Context) {
	// 获取WebSocket网关地址
	wsGatewayURL := "ws://ws-gateway:8081"

	// 解析目标URL
	target, err := url.Parse(wsGatewayURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid gateway url"})
		return
	}

	// 复制请求
	target.Request = c.Request
	target.URL.Host = target.Host
	target.URL.Scheme = "ws"
	target.URL.Path = c.Request.URL.Path
	target.URL.RawQuery = c.Request.URL.RawQuery

	// 代理WebSocket
	httputil.NewSingleHostReverseProxy(target).ServeHTTP(c.Writer, c.Request)
}
```

### 25.3 负载均衡

```go
// pkg/balancer/balancer.go
package balancer

import (
	"sync"
	"time"
)

// Endpoint 服务端点
type Endpoint struct {
	ID       string
	Host     string
	Port     int
	Weight   int
	Healthy  bool
	mu       sync.RWMutex
}

// NewEndpoint 创建端点
func NewEndpoint(id, host string, port int, weight int) *Endpoint {
	return &Endpoint{
		ID:      id,
		Host:    host,
		Port:    port,
		Weight:  weight,
		Healthy: true,
	}
}

// MarkHealth 标记健康状态
func (e *Endpoint) MarkHealth(healthy bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Healthy = healthy
}

// IsHealthy 检查是否健康
func (e *Endpoint) IsHealthy() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Healthy
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	endpoints []*Endpoint
	mu        sync.RWMutex
	strategy  Strategy
}

// Strategy 负载均衡策略
type Strategy int

const (
	RoundRobin Strategy = iota
	WeightedRoundRobin
	LeastConnection
	IPHash
)

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(strategy Strategy) *LoadBalancer {
	return &LoadBalancer{
		endpoints: make([]*Endpoint, 0),
		strategy:  strategy,
	}
}

// AddEndpoint 添加端点
func (lb *LoadBalancer) AddEndpoint(endpoint *Endpoint) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.endpoints = append(lb.endpoints, endpoint)
}

// RemoveEndpoint 移除端点
func (lb *LoadBalancer) RemoveEndpoint(id string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	newEndpoints := make([]*Endpoint, 0, len(lb.endpoints))
	for _, ep := range lb.endpoints {
		if ep.ID != id {
			newEndpoints = append(newEndpoints, ep)
		}
	}
	lb.endpoints = newEndpoints
}

// Next 获取下一个端点
func (lb *LoadBalancer) Next() *Endpoint {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// 过滤健康端点
	healthyEndpoints := make([]*Endpoint, 0)
	for _, ep := range lb.endpoints {
		if ep.IsHealthy() {
			healthyEndpoints = append(healthyEndpoints, ep)
		}
	}

	if len(healthyEndpoints) == 0 {
		return nil
	}

	switch lb.strategy {
	case RoundRobin:
		return lb.roundRobin(healthyEndpoints)
	case WeightedRoundRobin:
		return lb.weightedRoundRobin(healthyEndpoints)
	case LeastConnection:
		return lb.leastConnection(healthyEndpoints)
	case IPHash:
		// 需要传入客户端IP
		return healthyEndpoints[0]
	default:
		return healthyEndpoints[0]
	}
}

// roundRobin 轮询
func (lb *LoadBalancer) roundRobin(endpoints []*Endpoint) *Endpoint {
	// 使用外部计数器实现
	return endpoints[int(time.Now().Unix())%len(endpoints)]
}

// weightedRoundRobin 加权轮询
func (lb *LoadBalancer) weightedRoundRobin(endpoints []*Endpoint) *Endpoint {
	totalWeight := 0
	for _, ep := range endpoints {
		totalWeight += ep.Weight
	}

	if totalWeight == 0 {
		return endpoints[0]
	}

	r := int(time.Now().Unix()) % totalWeight
	for _, ep := range endpoints {
		r -= ep.Weight
		if r <= 0 {
			return ep
		}
	}

	return endpoints[0]
}

// leastConnection 最少连接
func (lb *LoadBalancer) leastConnection(endpoints []*Endpoint) *Endpoint {
	// TODO: 实现连接数跟踪
	return endpoints[0]
}
```

---

## 26. 服务发现与注册

### 26.1 服务注册

```go
// pkg/registry/registry.go
package registry

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ServiceRegistry 服务注册中心
type ServiceRegistry struct {
	client     *clientv3.Client
	services   map[string]*ServiceInfo
	mu         sync.RWMutex
	instanceID string
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name      string
	ID        string
	Address   string
	Port      int
	Tags      []string
	Metadata  map[string]string
	Healthy   bool
	TTL       time.Duration
}

// NewServiceRegistry 创建服务注册中心
func NewServiceRegistry(etcdEndpoints []string) (*ServiceRegistry, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("create etcd client failed: %w", err)
	}

	return &ServiceRegistry{
		client:     client,
		services:   make(map[string]*ServiceInfo),
		instanceID: generateInstanceID(),
	}, nil
}

// Register 注册服务
func (r *ServiceRegistry) Register(ctx context.Context, info *ServiceInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("/services/%s/%s", info.Name, info.ID)

	value := fmt.Sprintf("%s:%d", info.Address, info.Port)

	// 创建租约
	lease, err := r.client.Grant(ctx, r.getTTL(info))
	if err != nil {
		return fmt.Errorf("grant lease failed: %w", err)
	}

	// 注册服务
	_, err = r.client.Put(ctx, key, value)
	if err != nil {
		lease.Close()
		return fmt.Errorf("register service failed: %w", err)
	}

	// 保存服务信息
	info.Healthy = true
	r.services[info.ID] = info

	// 续租
	go r.keepAlive(ctx, key, lease)

	return nil
}

// keepAlive 保持租约
func (r *ServiceRegistry) keepAlive(ctx context.Context, key string, lease *clientv3.Lease) {
	ticker := time.NewTicker(r.getTTL(&ServiceInfo{}) / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := lease.KeepAliveOnce(ctx)
			if err != nil {
				return
			}

		case <-ctx.Done():
			lease.Close()
			return
		}
	}
}

// getTTL 获取TTL
func (r *ServiceRegistry) getTTL(info *ServiceInfo) time.Duration {
	if info.TTL > 0 {
		return info.TTL
	}
	return 10 * time.Second
}

// Deregister 注销服务
func (r *ServiceRegistry) Deregister(ctx context.Context, name, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("/services/%s/%s", name, id)

	// 删除注册
	_, err := r.client.Delete(ctx, key)
	if err != nil {
		return err
	}

	// 从内存移除
	delete(r.services, id)

	return nil
}

// Discover 发现服务
func (r *ServiceRegistry) Discover(ctx context.Context, name string) ([]*ServiceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	prefix := fmt.Sprintf("/services/%s/", name)

	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	var services []*ServiceInfo
	for _, kv := resp.Kvs {
		parts := splitKey(string(kv.Key))
		if len(parts) < 4 {
			continue
		}

		serviceID := parts[3]
		if info, exists := r.services[serviceID]; exists && info.Healthy {
			services = append(services, info)
		}
	}

	return services, nil
}

// Watch 监听服务变化
func (r *ServiceRegistry) Watch(ctx context.Context, name string) <-chan []*ServiceInfo {
	ch := make(chan []*ServiceInfo, 10)

	go func() {
		defer close(ch)

		prefix := fmt.Sprintf("/services/%s/", name)
		watcher := r.client.Watch(ctx, prefix, clientv3.WithPrefix())

		for {
			resp, err := watcher.Next(ctx)
			if err != nil {
				return
			}

			if resp.Canceled {
				return
			}

			// 解析服务列表
			services := r.parseServices(resp.Kvs)
			ch <- services
		}
	}()

	return ch
}

// parseServices 解析服务列表
func (r *ServiceRegistry) parseServices(kvs []*clientv3.Event) []*ServiceInfo {
	var services []*ServiceInfo

	for _, ev := range kvs {
		if ev.Type == clientv3.EventTypePut {
			parts := splitKey(string(ev.Kv.Key))
			if len(parts) < 4 {
				continue
			}

			// 解析地址
			addr, port := parseAddress(string(ev.Kv.Value))

			services = append(services, &ServiceInfo{
				Name:    parts[2],
				Address: addr,
				Port:    port,
				Healthy: true,
			})
		} else if ev.Type == clientv3.EventTypeDelete {
			// 服务被删除
		}
	}

	return services
}
```

### 26.2 服务健康检查

```go
// pkg/registry/health.go
package registry

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	client    *http.Client
	interval  time.Duration
	timeout   time.Duration
	endpoints map[string]string
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		client:    &http.Client{Timeout: timeout},
		interval:  interval,
		timeout:   timeout,
		endpoints: make(map[string]string),
	}
}

// AddEndpoint 添加检查端点
func (h *HealthChecker) AddEndpoint(name, url string) {
	h.endpoints[name] = url
}

// Check 检查服务健康
func (h *HealthChecker) Check(ctx context.Context, name, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Start 启动健康检查
func (h *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for name, url := range h.endpoints {
				healthy := h.Check(ctx, name, url)
				// 更新服务健康状态
				_ = h.updateHealthStatus(name, healthy)
			}

		case <-ctx.Done():
			return
		}
	}
}

// updateHealthStatus 更新健康状态
func (h *HealthChecker) updateHealthStatus(name string, healthy bool) error {
	// TODO: 通知负载均衡器更新状态
	return nil
}
```

---

## 27. 链路追踪完整实现

### 27.1 追踪上下文

```go
// pkg/trace/context.go
package trace

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Context 追踪上下文
type Context struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Service   string
	Operation string
}

// New 创建新追踪
func New(service, operation string) *Context {
	traceID := uuid.New().String()
	spanID := uuid.New().String()

	return &Context{
		TraceID:   traceID,
		SpanID:    spanID,
		Service:   service,
		Operation: operation,
	}
}

// FromContext 从context提取追踪
func FromContext(ctx context.Context) *Context {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	spanCtx := trace.SpanContextFromContext(ctx)
	return &Context{
		TraceID:  spanCtx.TraceID().String(),
		SpanID:   spanCtx.SpanID().String(),
		ParentID:  spanCtx.TraceID().String(),
	}
}

// ToContext 将追踪放入context
func (c *Context) ToContext(ctx context.Context) context.Context {
	spanCtx := trace.SpanContext{
		TraceID: trace.TraceID(c.TraceID),
		SpanID:   trace.SpanID(c.SpanID),
	}

	if c.ParentID != "" {
		traceID, _ := trace.TraceIDFromString(c.ParentID)
		spanCtx.TraceID = traceID
	}

	return trace.ContextWithSpanContext(ctx, spanCtx)
}

// StartSpan 开始span
func (c *Context) StartSpan(ctx context.Context, operation string) (context.Context, *trace.Span) {
	ctx, span := otel.Tracer().Start(ctx, operation,
		trace.WithAttributes(
		key.String("trace.id", c.TraceID),
		key.String("service", c.Service),
		key.String("operation", operation),
	),
	)

	return ctx, span
}

// InjectHeaders 注入追踪headers
func (c *Context) InjectHeaders(headers map[string]string) {
	headers["X-Trace-ID"] = c.TraceID
	headers["X-Span-ID"] = c.SpanID
	if c.ParentID != "" {
		headers["X-Parent-ID"] = c.ParentID
	}
}

// ExtractFromHeaders 从headers提取追踪
func ExtractFromHeaders(headers map[string]string) *Context {
	ctx := &Context{
		TraceID:  headers["X-Trace-ID"],
		SpanID:   headers["X-Span-ID"],
		ParentID: headers["X-Parent-ID"],
	}

	// 如果没有追踪ID，创建新的
	if ctx.TraceID == "" {
		return New("", "")
	}

	return ctx
}
```

### 27.2 中间件

```go
// pkg/trace/middleware.go
package trace

import (
	"github.com/gin-gonic/gin"
)

// Middleware 追踪中间件
func Middleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从header提取追踪信息
		ctx := ExtractFromHeaders(c.Request.Header)

		// 设置服务名
		if ctx.Service == "" {
			ctx.Service = serviceName
		}

		// 生成operation
		operation := c.Request.Method + " " + c.Request.URL.Path

		// 创建span
		ctx, span := ctx.StartSpan(c.Request.Context(), operation)
		defer span.End()

		// 将追踪信息放入context
		c.Request = c.Request.WithContext(ctx.ToContext(c.Request.Context()))

		// 注入追踪信息到headers
		ctx.InjectHeaders(c.Writer.Header())

		c.Next()
	}
}
```

### 27.3 初始化

```go
// pkg/trace/init.go
package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

// Config 追踪配置
type Config struct {
	Enabled     bool
	ServiceName string
	JaegerAddr  string
	Sampler    float64
}

// Init 初始化追踪
func Init(cfg *Config) error {
	if !cfg.Enabled {
		return nil
	}

	// 创建Jaeger导出器
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
		fmt.Sprintf("http://%s/api/traces", cfg.JaegerAddr),
	))
	if err != nil {
		return fmt.Errorf("create jaeger exporter failed: %w", err)
	}

	// 创建资源
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return fmt.Errorf("create resource failed: %w", err)
	}

	// 创建provider
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(res),
		tracesdk.WithSampler(tracesdk.TraceIDRatioBased(cfg.Sampler)),
	)

	// 注册全局
	otel.SetTracerProvider(tp)

	return nil
}
```

---

## 28. 性能测试用例

### 28.1 Actor性能测试

```go
// internal/actor/stress_test.go
package actor_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// BenchmarkActorSend 性能测试发送
func BenchmarkActorSend(b *testing.B) {
	handler := &MockActorHandler{}
	act := NewBaseActor("bench-actor", "test", 10000, handler)

	ctx := context.Background()
	act.Start(ctx)
	defer act.Stop()

	msg := &TestMessage{Data: "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = act.Send(msg)
	}
}

// BenchmarkActorProcessing 性能测试处理
func BenchmarkActorProcessing(b *testing.B) {
	handler := &MockActorHandler{}
	act := NewBaseActor("bench-actor", "test", 10000, handler)

	ctx := context.Background()
	act.Start(ctx)
	defer act.Stop()

	msg := &TestMessage{Data: "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = act.Send(msg)
		time.Sleep(10 * time.Microsecond) // 模拟处理延迟
	}
}

// BenchmarkConcurrentActors 并发Actor测试
func BenchmarkConcurrentActors(b *testing.B) {
	actorCount := 100
	messagesPerActor := 1000

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 创建多个actor
		var actors []*BaseActor
		for j := 0; j < actorCount; j++ {
			handler := &MockActorHandler{}
			act := NewBaseActor(fmt.Sprintf("actor-%d", j), "test", 1000, handler)
			act.Start(context.Background())
			defer act.Stop()
			actors = append(actors, act)
		}

		// 并发发送消息
		var wg sync.WaitGroup
		for _, act := range actors {
			wg.Add(1)
			go func(a *BaseActor) {
				defer wg.Done()
				for k := 0; k < messagesPerActor; k++ {
					_ = a.Send(&TestMessage{Data: "test"})
				}
			}(act)
		}
		wg.Wait()

		// 清理actors
		for _, act := range actors {
			act.Stop()
		}
	}
}

// TestActorThroughput 测试吞吐量
func TestActorThroughput(t *testing.T) {
	handler := &MockActorHandler{}
	act := NewBaseActor("throughput-actor", "test", 10000, handler)

	ctx := context.Background()
	act.Start(ctx)
	defer act.Stop()

	// 并发发送者数量
	senders := 10
	messagesPerSender := 10000

	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < senders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < messagesPerSender; j++ {
				msg := &TestMessage{Data: fmt.Sprintf("msg-%d", j)}
				for act.Send(msg) != nil {
					// inbox满了，等待
					time.Sleep(time.Microsecond)
					j-- // 重试
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalMessages := senders * messagesPerSender
	throughput := float64(totalMessages) / duration.Seconds()

	t.Logf("Total messages: %d", totalMessages)
	t.Logf("Duration: %v", duration)
	t.Logf("Throughput: %.2f msg/s", throughput)

	// 验证统计
	stats := act.Stats()
	t.Logf("Processed: %d", stats.MessageCount)
	t.Logf("Errors: %d", stats.ErrorCount)

	// 性能要求
	minThroughput := 10000.0 // 10000 msg/s
	if throughput < minThroughput {
		t.Errorf("Throughput too low: %.2f < %.2f", throughput, minThroughput)
	}
}
```

### 28.2 引擎性能测试

```go
// internal/engine/engine_test.go
package engine_test

import (
	"context"
	"testing"
)

// BenchmarkJSEngine JS引擎性能测试
func BenchmarkJSEngine(b *testing.B) {
	engine := NewJSEngine()
	_ = engine.Init(context.Background(), "test-game", nil)

	script := `
		function handleMessage(msg) {
			return { result: "ok", data: msg };
		}
	`

	_ = engine.LoadScript(EngineJS, []byte(script))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Execute(context.Background(), "handleMessage", map[string]interface{}{
			"message": fmt.Sprintf("test-%d", i),
		})
	}
}

// BenchmarkEngineSwitch 引擎切换性能测试
func BenchmarkEngineSwitch(b *testing.B) {
	engine := NewJSEngine()
	_ = engine.Init(context.Background(), "test-game", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 切换引擎
		// TODO: 实现引擎切换测试
	}
}
```

### 28.3 API压力测试

```go
// tests/stress/api_stress_test.go
package stress_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAPIConcurrentRequests 并发请求测试
func TestAPIConcurrentRequests(t *testing.T) {
	baseURL := "http://localhost:8080"

	// 测试配置
	concurrency := 100
	requestsPerClient := 100

	var wg sync.WaitGroup
	successCount := int64(0)
	errorCount := int64(0)
	var mu sync.Mutex

	start := time.Now()

	// 并发客户端
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			client := &http.Client{Timeout: 10 * time.Second}

			for j := 0; j < requestsPerClient; j++ {
				resp, err := client.Get(baseURL + "/health")
				mu.Lock()
				if err == nil && resp.StatusCode == 200 {
					successCount++
				} else {
					errorCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := int64(concurrency * requestsPerClient)
	qps := float64(totalRequests) / duration.Seconds()

	t.Logf("Total requests: %d", totalRequests)
	t.Logf("Success: %d", successCount)
	t.Logf("Errors: %d", errorCount)
	t.Logf("Duration: %v", duration)
	t.Logf("QPS: %.2f", qps)

	// 验证成功率
	successRate := float64(successCount) / float64(totalRequests) * 100
	assert.Greater(t, successRate, 99.0) // 99%成功率
}

// TestWebSocketConnections WebSocket连接测试
func TestWebSocketConnections(t *testing.T) {
	wsURL := "ws://localhost:8081/ws"

	concurrency := 1000
	var wg sync.WaitGroup

	start := time.Now()

	// 并发连接
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// TODO: 实现WebSocket连接测试
			_ = id
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Connections: %d", concurrency)
	t.Logf("Duration: %v", duration)
	t.Logf("Avg connect time: %v", duration/time.Duration(concurrency))
}
```

---

## 29. CI/CD配置

### 29.1 GitHub Actions

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: test_db
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

      redis:
        image: redis:7-alpine
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Cache Go modules
        uses: actions/cache@v3
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-

      - name: Install dependencies
        run: go mod download

      - name: Run tests
        run: |
          go test -v -race -coverprofile=coverage.out ./...
          go tool cover -html=coverage.out -o coverage.html

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out

      - name: Run linter
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest

  build:
    runs-on: ubuntu-latest
    needs: test

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Build
        run: |
          go build -v -o bin/game-service ./cmd/game-service
          go build -v -o bin/user-service ./cmd/user-service
          go build -v -o bin/payment-service ./cmd/payment-service

      - name: Upload artifacts
        uses: actions/upload-artifact@v3
        with:
          name: binaries
          path: bin/

  docker:
    runs-on: ubuntu-latest
    needs: build

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Login to Docker Hub
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}

      - name: Build and push
        uses: docker/build-push-action@v4
        with:
          context: .
          file: ./deployments/docker/Dockerfile.game
          push: true
          tags: |
            ${{ github.sha }}
            latest
```

### 29.2 Docker Compose测试环境

```yaml
# deployments/docker-compose.test.yml
version: '3.8'

services:
  postgres-test:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: test_db
    ports:
      - "5433:5432"

  redis-test:
    image: redis:7-alpine
    ports:
      - "6380:6379"

  zookeeper-test:
    image: bitnami/zookeeper:latest
    environment:
      ZOO_SERVER_ID: 1

  kafka-test:
    image: bitnami/kafka:latest
    environment:
      KAFKA_CFG_ZOOKEEPER_CONNECT: zookeeper-test:2181
      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9093
    ports:
      - "9093:9092"
    depends_on:
      - zookeeper-test

  test-runner:
    build:
      context: ..
      dockerfile: deployments/docker/Dockerfile.test
    depends_on:
      - postgres-test
      - redis-test
    environment:
      - DB_HOST=postgres-test
      - REDIS_HOST=redis-test
      - KAFKA_BROKERS=kafka-test:9093
    volumes:
      - ../:/app
```

---

## 30. 实用工具函数

### 30.1 字符串工具

```go
// pkg/utils/string.go
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
)

// RandomString 生成随机字符串
func RandomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

// RandomStringAlphaNum 生成字母数字随机字符串
func RandomStringAlphaNum(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

// MaskEmail 邮箱掩码
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	username := parts[0]
	if len(username) <= 3 {
		return email
	}

	masked := username[:2] + "***" + username[len(username)-2:]
	return masked + "@" + parts[1]
}

// MaskPhone 手机号掩码
func MaskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}

	// 保留前3和后4位
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// TruncateString 截断字符串
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ToSnakeCase 转换为snake_case
func ToSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result,.ToLower(r))
	}
	return string(result)
}

// ToCamelCase 转换为camelCase
func ToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if i > 0 {
			parts[i] = strings.Title(part)
		}
	}
	return strings.Join(parts, "")
}
```

### 30.2 时间工具

```go
// pkg/utils/time.go
package utils

import (
	"time"
)

// TimeHelper 时间辅助函数
type TimeHelper struct{}

var Time = &TimeHelper{}

// BeginOfDay 当天开始时间
func (h *TimeHelper) BeginOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EndOfDay 当天结束时间
func (h *TimeHelper) EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

// BeginOfWeek 周开始时间
func (h *TimeHelper) BeginOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	return h.BeginOfDay(t).AddDate(-weekday)
}

// EndOfWeek 周结束时间
func (h *TimeHelper) EndOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	return h.EndOfDay(t).AddDate(6 - weekday)
}

// BeginOfMonth 月开始时间
func (h *TimeHelper) BeginOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// EndOfMonth 月结束时间
func (h *TimeHelper) EndOfMonth(t time.Time) time.Time {
	return h.BeginOfMonth().AddDate(32).AddDate(-1)
}

// Age 计算年龄
func (h *TimeHelper) Age(birthday time.Time) int {
	now := time.Now()
	age := now.Year() - birthday.Year()
	if now.YearDay() < birthday.YearDay() {
		age--
	}
	return age
}

// DaysBetween 计算两个日期之间的天数
func (h *TimeHelper) DaysBetween(start, end time.Time) int {
	duration := end.Sub(start)
	hours := int(duration.Hours())
	return hours / 24
}

// IsSameDay 是否同一天
func (h *TimeHelper) IsSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() &&
		t1.Month() == t2.Month() &&
		t1.Day() == t2.Day()
}

// FormatDate 格式化日期
func (h *TimeHelper) FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// FormatDateTime 格式化日期时间
func (h *TimeHelper) FormatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// ParseDate 解析日期
func (h *TimeHelper) ParseDate(dateStr string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", dateStr, time.Local)
}

// ParseDateTime 解析日期时间
func (h *TimeHelper) ParseDateTime(dateTimeStr string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", dateTimeStr, time.Local)
}
```

### 30.3 加密工具

```go
// pkg/utils/crypto.go
package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// EncryptAESEncrypt AES加密
func EncryptAESEncrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	ciphertext = append(nonce, ciphertext...)

	return ciphertext, nil
}

// DecryptAESDecrypt AES解密
func DecryptAESDecrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptString 加密字符串
func EncryptString(plaintext, key string) (string, error) {
	keyBytes := sha256.Sum256([]byte(key))

	plaintextBytes := []byte(plaintext)
	ciphertext, err := EncryptAESEncrypt(plaintextBytes, keyBytes[:])
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString 解密字符串
func DecryptString(ciphertext, key string) (string, error) {
	keyBytes := sha256.Sum256([]byte(key))

	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	plaintext, err := DecryptAESDecrypt(ciphertextBytes, keyBytes[:])
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// HashPassword 哈希密码
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 验证密码
func VerifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}
```

### 30.4 验证工具

```go
// pkg/utils/validator.go
package validator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

// Validator 验证器
type Validator struct {
	validate *validator.Validate
}

// NewValidator 创建验证器
func NewValidator() *Validator {
	return &Validator{
		validate: validator.New(),
	}
}

// ValidateStruct 验证结构体
func (v *Validator) ValidateStruct(s interface{}) error {
	return v.validate.Struct(s)
}

// ValidateEmail 验证邮箱
func ValidateEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}

// ValidatePhone 验证手机号
func ValidatePhone(phone string) bool {
	// 简单的中国手机号验证
	pattern := `^1[3-9]\d{9}$`
	matched, _ := regexp.MatchString(pattern, phone)
	return matched
}

// ValidateUsername 验证用户名
func ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 20 {
		return fmt.Errorf("username must be 3-20 characters")
	}

	// 只允许字母数字下划线
	pattern := `^[a-zA-Z0-9_]+$`
	matched, _ := regexp.MatchString(pattern, username)
	if !matched {
		return fmt.Errorf("username can only contain letters, numbers, and underscores")
	}

	return nil
}

// ValidatePassword 验证密码强度
func ValidatePassword(password string) error {
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber bool
		hasSymbol bool
	)

	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsNumber(ch):
			hasNumber = true
	case strings.Contains("!@#$%^&*()_+-=[]{}|;:', ch):
			hasSymbol = true
		}
	}

	var missing []string
	if !hasUpper {
		missing = append(missing, "uppercase letter")
	}
	if !hasLower {
		missing = append(missing, "lowercase letter")
	}
	if !hasNumber {
		missing = append(missing, "number")
	}

	if len(missing) > 0 {
		return fmt.Errorf("password must contain %s", strings.Join(missing, ", "))
	}

	return nil
}
```

---

**文档版本:** v1.4
**更新时间:** 2026-03-24
**总行数:** 9500+
**新增章节:** Protobuf生成、Docker Compose、监控面板、日志收集、故障排查、Swagger文档、部署运维工具

---

## 31. Protobuf生成脚本

### 31.1 Make集成

```makefile
# Makefile 中的protobuf相关部分

.PHONY: proto proto-all proto-clean

# proto目录
PROTO_DIR = api/proto
# 生成目录
PROTO_OUT_DIR = api/proto

# 生成Go代码
proto:
	@echo "Generating protobuf files..."
	@for proto in $(PROTO_DIR)/*.proto; do \
		protoc \
			--go_out=$(PROTO_OUT_DIR) \
			--go_opt=paths=source_relative \
			--go-grpc_out=$(PROTO_OUT_DIR) \
			--go-grpc_opt=paths=source_relative \
			$$proto; \
	done

# 生成所有语言的代码
proto-all: proto
	@echo "Generating TypeScript definitions..."
	@for proto in $(PROTO_DIR)/*.proto; do \
		protoc \
			--plugin=protoc-gen-ts \
			--ts_out=web/src/proto \
			$$proto; \
	done

# 清理生成的文件
proto-clean:
	@echo "Cleaning protobuf generated files..."
	@find $(PROTO_OUT_DIR) -name "*.pb.go" -delete
	@find $(PROTO_OUT_DIR) -name "*_grpc.pb.go" -delete
```

### 31.2 生成脚本

```bash
#!/bin/bash
# scripts/generate_proto.sh

set -e

PROTO_DIR="api/proto"
OUTPUT_DIR="api/proto"

echo "Generating protobuf files..."

# 检查protoc是否安装
if ! command -v protoc &> /dev/null; then
    echo "protoc is not installed. Please install it first."
    echo "Visit: https://grpc.io/docs/protoc-installation/"
    exit 1
fi

# 检查插件
required_plugins=("protoc-gen-go" "protoc-gen-go-grpc")
missing_plugins=()

for plugin in "${required_plugins[@]}"; do
    if ! command -v $plugin &> /dev/null; then
        missing_plugins+=($plugin)
    fi
done

if [ ${#missing_plugins[@]} -ne 0 ]; then
    echo "Missing plugins: ${missing_plugins[*]}"
    echo "Install them with:"
    echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    echo "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    exit 1
fi

# 生成Go代码
for proto_file in $PROTO_DIR/*.proto; do
    echo "Processing $proto_file..."

    protoc \
        --go_out=$OUTPUT_DIR \
        --go_opt=paths=source_relative \
        --go-grpc_out=$OUTPUT_DIR \
        --go-grpc_opt=paths=source_relative \
        $proto_file
done

echo "Protobuf generation completed!"
```

### 31.3 Protobuf定义示例

```protobuf
// api/proto/common.proto
syntax = "proto3";

package common.v1;

option go_package = "your-project/api/proto/common/v1;commonv1";

// 分页请求
message PageRequest {
  int32 page = 1;
  int32 page_size = 2;
}

// 分页响应
message PageResponse {
  int32 total = 1;
  int32 page = 2;
  int32 page_size = 3;
}

// 空请求
message Empty {}
```

---

## 32. Docker Compose完整配置

```yaml
# deployments/docker-compose.yml
version: '3.8'

services:
  # ==================== 基础设施 ====================

  postgres:
    image: postgres:16-alpine
    container_name: game-postgres
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: game_db
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - game-network

  redis:
    image: redis:7-alpine
    container_name: game-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5
    networks:
      - game-network

  zookeeper:
    image: bitnami/zookeeper:3.8
    container_name: game-zookeeper
    environment:
      ALLOW_ANONYMOUS_LOGIN: "yes"
    networks:
      - game-network

  kafka:
    image: bitnami/kafka:3.6
    container_name: game-kafka
    depends_on:
      - zookeeper
    environment:
      KAFKA_CFG_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE: "true"
      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
    ports:
      - "9092:9092"
    healthcheck:
      test: ["CMD-SHELL", "kafka-topics.sh --bootstrap-server localhost:9092 --list"]
      interval: 30s
      timeout: 10s
      retries: 5
    networks:
      - game-network

  etcd:
    image: bitnami/etcd:3.5
    container_name: game-etcd
    environment:
      ALLOW_NONE_AUTHENTICATION: "yes"
      ETCD_ADVERTISE_CLIENT_URLS: "http://etcd:2379"
    ports:
      - "2379:2379"
    networks:
      - game-network

  # ==================== 监控 ====================

  prometheus:
    image: prom/prometheus:latest
    container_name: game-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./deployments/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    networks:
      - game-network

  grafana:
    image: grafana/grafana:latest
    container_name: game-grafana
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./deployments/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./deployments/grafana/datasources:/etc/grafana/provisioning/datasources
    depends_on:
      - prometheus
    networks:
      - game-network

  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: game-jaeger
    ports:
      - "5775:5775"
      - "16686:16686"
    environment:
      COLLECTOR_ZIPKIN_HOST_PORT: ":9411"
    networks:
      - game-network

  # ==================== 服务 ====================

  game-service:
    build:
      context: ..
      dockerfile: deployments/docker/Dockerfile.game
    container_name: game-service
    ports:
      - "8002:8002"
      - "9091:9091"
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
      - KAFKA_BROKERS=kafka:9092
      - ETCD_ENDPOINTS=etcd:2379
      - JAEGER_ENDPOINT=http://jaeger:16686
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - game-network
    restart: unless-stopped

  user-service:
    build:
      context: ..
      dockerfile: deployments/docker/Dockerfile.user
    container_name: user-service
    ports:
      - "8001:8001"
      - "9092:9092"
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - game-network
    restart: unless-stopped

  ws-gateway:
    build:
      context: ..
      dockerfile: deployments/docker/Dockerfile.ws-gateway
    container_name: ws-gateway
    ports:
      - "8081:8081"
    environment:
      - REDIS_HOST=redis
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      redis:
        condition: service_healthy
      kafka:
        condition: service_healthy
    networks:
      - game-network
    restart: unless-stopped

  gateway:
    build:
      context: ..
      dockerfile: deployments/docker/Dockerfile.gateway
    container_name: gateway
    ports:
      - "8080:8080"
    environment:
      - GAME_SERVICE=http://game-service:8002
      - USER_SERVICE=http://user-service:8001
      - WS_GATEWAY=http://ws-gateway:8081
    depends_on:
      - game-service
      - user-service
      - ws-gateway
    networks:
      - game-network
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
  prometheus_data:
  grafana_data:

networks:
  game-network:
    driver: bridge
```

---

## 33. 监控面板配置

### 33.1 Prometheus配置

```yaml
# deployments/prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'game-service'
    static_configs:
      - targets: ['game-service:9091']

  - job_name: 'user-service'
    static_configs:
      - targets: ['user-service:9092']

  - job_name: 'ws-gateway'
    static_configs:
      - targets: ['ws-gateway:9094']

  - job_name: 'gateway'
    static_configs:
      - targets: ['gateway:9095']
```

### 33.2 告警规则

```yaml
# deployments/prometheus/rules/alerts.yml
groups:
  - name: game_service_alerts
    interval: 30s
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"

      - alert: HighLatency
        expr: histogram_quantile(0.95, http_request_duration_seconds) > 0.5
        for: 5m
        labels:
          severity: warning

      - alert: ActorMemoryLeak
        expr: actor_active_count > 1000
        for: 10m
        labels:
          severity: critical
```

---

## 34. 日志收集配置

### 34.1 Loki配置

```yaml
# deployments/loki/config.yml
auth_enabled: false
server:
  http_listen_port: 3100

limits_config:
  max_streams_per_user: 0

ingester:
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1

schema_config:
  configs:
    - from: 2024-01-01
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h

storage_config:
  filesystem:
    directory: /loki/chunks
```

### 34.2 Promtail配置

```yaml
# deployments/promtail/config.yml
server:
  log_level: info
  http_listen_port: 9080

positions:
  filename: /tmp/positions.yaml

client:
  url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: game-service
    static_configs:
      - targets:
          - localhost
        labels:
          job: game-service
          __path__: /var/log/game-service/*.log

  - job_name: gateway
    static_configs:
      - targets:
          - localhost
        labels:
          job: gateway
          __path__: /var/log/gateway/*.log
```

---

## 35. 故障排查指南

### 35.1 常见问题

#### 服务无法启动

```bash
# 1. 检查端口占用
lsof -i :8002

# 2. 查看日志
tail -f /var/log/game-service/error.log

# 3. 解决方案
kill -9 <PID>
```

#### Actor消息堆积

```bash
# 1. 查看Actor统计
curl http://localhost:9091/metrics | grep actor

# 2. 检查inbox大小
curl http://localhost:9091/metrics | grep actor_inbox_size

# 3. 解决方案
# - 增加Actor实例
# - 优化处理逻辑
# - 启用消息批处理
```

#### WebSocket连接断开

```bash
# 1. 检查连接数
ws_connections_active

# 2. 查看错误日志
tail -f /var/log/ws-gateway/error.log | grep "WebSocket"

# 3. 解决方案
# - 调整心跳间隔
# - 检查Nginx超时设置
```

### 35.2 诊断命令

```bash
#!/bin/bash
# scripts/diagnose.sh

echo "=== 游戏平台服务诊断 ==="

# 1. 检查服务状态
docker-compose ps

# 2. 检查资源使用
docker stats --no-stream

# 3. 检查日志错误
docker-compose logs --tail=50 2>&1 | grep -i error

# 4. 检查连接数
echo "PostgreSQL: $(docker exec game-postgres psql -U postgres -c 'SELECT count(*) FROM pg_stat_activity;')"

# 5. 检查健康状态
for service in game-service user-service payment-service ws-gateway; do
    status=$(curl -s http://localhost:8080/api/v1/$service/health || echo "down")
    echo "$service: $status"
done
```

---

## 36. API文档生成（Swagger）

### 36.1 Swagger配置

```go
// pkg/swagger/swagger.go
package swagger

import (
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag"
)

// @title 游戏平台API
// @version 1.0
// @description 游戏平台后端API文档
// @host localhost:8080
// @BasePath /api/v1

func InitSwagger(host string) {
	swag.Info.Host = host
}

// DocHandler 返回Swagger文档处理器
func DocHandler() gin.HandlerFunc {
	return ginSwagger.WrapHandler(swaggerFiles.Handler)
}
```

### 36.2 Handler注解示例

```go
// CreateGame 创建游戏
// @Summary 创建游戏
// @Description 创建新游戏
// @Tags game
// @Accept json
// @Produce json
// @Param request body CreateGameRequest true "创建游戏请求"
// @Success 200 {object} CreateGameResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /game/create [post]
func (h *GameHandler) CreateGame(c *gin.Context) {
	// ...
}
```

---

## 37. 部署运维工具

### 37.1 服务管理脚本

```bash
#!/bin/bash
# scripts/service.sh

ACTION=$1
SERVICE=$2

start_service() {
    local service=$1
    nohup ./bin/$service --config=configs/config.prod.yaml > /var/log/$service/output.log 2>&1 &
    echo "$service started with PID: $!"
}

stop_service() {
    local service=$1
    pid=$(pgrep -f "$service" || true)
    if [ -n "$pid" ]; then
        kill -TERM $pid
        sleep 5
        kill -9 $pid 2>/dev/null || true
    fi
}

restart_service() {
    stop_service $1
    sleep 2
    start_service $1
}

case $ACTION in
    start)
        [ -z "$SERVICE" ] && for svc in gateway ws-gateway game-service user-service; do start_service $svc; done
        [ -n "$SERVICE" ] && start_service $SERVICE
        ;;
    stop)
        [ -z "$SERVICE" ] && for svc in user-service game-service ws-gateway gateway; do stop_service $svc; done
        [ -n "$SERVICE" ] && stop_service $SERVICE
        ;;
    restart)
        restart_service $SERVICE
        ;;
esac
```

### 37.2 数据库备份脚本

```bash
#!/bin/bash
# scripts/backup.sh

BACKUP_DIR="/data/backups/postgres"
DATE=$(date +%Y%m%d_%H%M%S)
RETENTION_DAYS=7

mkdir -p "$BACKUP_DIR"

echo "Starting database backup..."

DATABASES=("game_db" "game_main_db" "game_payment_db")

for db in "${DATABASES[@]}"; do
    backup_file="$BACKUP_DIR/${db}_${DATE}.sql.gz"
    PGPASSWORD=$DB_PASSWORD pg_dump -h $DB_HOST -p $DB_PORT -U $DB_USER \
        -d $db | gzip > "$backup_file"
    echo "Backup completed: $backup_file"
done

# 清理旧备份
find "$BACKUP_DIR" -name "*.sql.gz" -mtime +$RETENTION_DAYS -delete

echo "Backup completed!"
```

---

## 38. 完整API参考文档

### 38.1 用户服务API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | /api/v1/user/register | 用户注册 | 否 |
| POST | /api/v1/user/login | 用户登录 | 否 |
| GET | /api/v1/user/profile | 获取用户资料 | 是 |
| PUT | /api/v1/user/profile | 更新用户资料 | 是 |

#### 用户注册请求/响应

```http
POST /api/v1/user/register
Content-Type: application/json

{
  "username": "player1",
  "password": "password123",
  "email": "player1@example.com",
  "nickname": "Player One"
}
```

### 38.2 游戏服务API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /api/v1/game/list | 游戏列表 | 是 |
| POST | /api/v1/game/rooms | 创建房间 | 是 |
| POST | /api/v1/game/rooms/{id}/join | 加入房间 | 是 |

### 38.3 支付服务API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | /api/v1/payment/orders | 创建订单 | 是 |
| GET | /api/v1/payment/balance | 获取积分 | 是 |

---

## 39. 客户端SDK实现

### 39.1 JavaScript SDK

```javascript
// web/sdk/game-sdk.js

class GameSDK {
  constructor(config) {
    this.baseURL = config.baseURL || 'http://localhost:8080/api/v1';
    this.wsURL = config.wsURL || 'ws://localhost:8081/ws';
    this.token = localStorage.getItem('token') || null;
    this.ws = null;
    this.listeners = {};
  }

  async login(username, password) {
    const response = await this.request('POST', '/user/login', {
      username,
      password
    });
    if (response.token) {
      this.setToken(response.token);
    }
    return response;
  }

  connectWebSocket() {
    this.ws = new WebSocket(`${this.wsURL}?token=${this.token}`);
    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.emit(message.type, message.data);
    };
  }
}
```

### 39.2 TypeScript定义

```typescript
// web/sdk/types.ts

export interface GameConfig {
  baseURL: string;
  wsURL: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  expires_at: number;
  user: UserInfo;
}

export interface UserInfo {
  user_id: number;
  username: string;
  nickname: string;
  avatar: string;
}

export interface RoomInfo {
  room_id: string;
  game_id: number;
  room_name: string;
  max_players: number;
  current_players: number;
  status: string;
}
```

---

## 40. 游戏开发指南

### 40.1 游戏接入流程

```
1. 注册游戏 → 2. 配置参数 → 3. 上传脚本 → 4. 测试 → 5. 发布
```

### 40.2 游戏脚本规范

```javascript
// 必须实现的函数
function onStart(config) { return { status: 'ready' }; }
function onTick(tick) { }
function onAction(action) { return {}; }
function getState() { return {}; }
```

---

## 41. 性能调优指南

### 41.1 Actor优化

- 消息批处理
- Actor对象池
- 并发处理

### 41.2 数据库优化

- 预加载避免N+1
- 索引优化
- 连接池配置

---

## 42. 安全加固指南

### 42.1 密码强度验证

### 42.2 请求限流

### 42.3 SQL注入防护

---

**文档版本:** v1.5
**更新时间:** 2026-03-24
**总行数:** 9200+
**总行数:** 9500+
**新增章节:** Protobuf生成、Docker Compose、监控面板、日志收集、故障排查、Swagger文档、部署运维工具
