# 在线游戏平台 - 实施计划

## 当前进度（更新于 2026-03-29，第二轮）

### 已完成 ✅

**Go 后端微服务（约 30 个文件，约 5000 行，`go build` + `go vet` + `go test` 全部通过）**

| 模块 | 文件 | 说明 |
|------|------|------|
| **cmd/api-gateway** | main.go | 反向代理，路径路由到各服务，静态资源服务，WebSocket 代理 |
| **cmd/user-service** | main.go | 用户服务：HTTP+gRPC 双协议、注册、登录、JWT、GORM |
| **cmd/game-service** | main.go | 游戏服务：HTTP+gRPC 双协议、ActorSystem + WebSocket + goja |
| **cmd/admin-service** | main.go | 管理服务：gRPC 客户端鉴权、游戏CRUD、zip上传解压校验 |
| **pkg/actor** | actor.go, system.go, message.go, game_actor.go | Actor 内核：BaseActor inbox channel + GameActor 持有 goja |
| **pkg/engine** | engine.go, js_engine.go, sandbox.go | goja 引擎：20+ 宿主API注入、SDK runtime、沙箱限制 |
| **pkg/websocket** | gateway.go, conn.go | WebSocket 网关：连接管理、房间广播、ReadPump/WritePump |
| **pkg/grpc** | server.go, client.go | gRPC 服务器/客户端：StartServer + UserServiceClient + GameServiceClient |
| **pkg/config** | config.go | 环境驱动配置加载（含 GRPCPort） |
| **pkg/db** | db.go | GORM PostgreSQL 连接池 |
| **pkg/auth** | jwt.go | JWT 生成/验证 + Redis 黑名单 |
| **pkg/redis** | redis.go | Redis 客户端 + 缓存辅助 |
| **pkg/api** | response.go | 统一 JSON 响应 `{code, message, data}` |
| **pkg/apperror** | app_error.go | 统一错误码 (HTTP_STATUS*100+SEQ) |
| **proto/user** | user.proto, user.pb.go, user_grpc.pb.go | UserService gRPC: ValidateToken + GetUser |
| **proto/game** | game.proto, game.pb.go, game_grpc.pb.go | GameService gRPC: GetGame + ListGames + GetRoom |
| **internal/user** | model.go, service.go, handler.go, grpc.go | 用户注册/登录、JWT 鉴权、gRPC 服务端 |
| **internal/game** | model.go, service.go, handler.go, grpc.go | 房间CRUD、Actor 创建、WebSocket 桥接、gRPC 服务端 |
| **internal/admin** | handler.go | 游戏 CRUD、zip 解压校验、manifest 验证 |
| **internal/server** | server.go | 通用 HTTP server（Gin + 优雅关闭） |

**TypeScript 双端 SDK + 示例游戏 + 部署配置**

| 模块 | 文件 | 说明 |
|------|------|------|
| **sdks/server-sdk** | package.json, tsconfig.json, src/types.ts, src/GameServer.ts, src/index.ts | 后端 SDK：ES5 目标、GameServer 抽象类 + register()、__platform_* 声明 |
| **sdks/client-sdk** | package.json, tsconfig.json, src/types.ts, src/EventEmitter.ts, src/Connection.ts, src/GameClient.ts, src/index.ts | 前端 SDK：ES2020 目标、WebSocket + 断线重连 + 心跳 + EventEmitter + 生命周期钩子 |
| **examples/games/blackjack** | manifest.json, server/main.ts, client/index.html, package.json, tsconfig.json | 完整 21 点示例：下注/要牌/停牌/加倍/庄家AI/胜负判定 |
| **deploy/** | docker-compose.yml, docker/Dockerfile.service, config/*.env | Docker Compose 4 服务 + PostgreSQL + 统一 Dockerfile |

### 待完成 ⏳

| 任务 | 状态 | 说明 |
|------|------|------|
| **gRPC 通信** | ✅ | proto/user + proto/game + gRPC server/client + admin-service 通过 gRPC 鉴权 |
| **Redis 缓存** | ✅ | pkg/redis + JWT 黑名单 + 游戏/用户缓存 + 沙箱 |
| **Vue 3 大厅 SPA** | ✅ | web/lobby/ — Vue 3 + TypeScript + Vite + Pinia + 暗色主题 |
| **Actor 沙箱** | ✅ | 执行超时、调用栈限制、脚本大小限制、定时器限制、eval 禁用 |
| **WebSocket 代理** | ✅ | /ws 端点 + ReadPump/WritePump + Gateway WS 代理 |
| **单元/集成测试** | 🔲 | Actor、Engine、Service 测试 |

### 实际目录结构

```
online-game/
├── go.mod / go.sum
├── plan.md
├── cmd/
│   ├── api-gateway/main.go      ✅
│   ├── user-service/main.go     ✅
│   ├── game-service/main.go     ✅
│   └── admin-service/main.go    ✅
├── internal/
│   ├── user/                    ✅ model + service + handler
│   ├── game/                    ✅ model + service + handler
│   ├── admin/                   ✅ handler (Service+Handler 合并)
│   └── server/                  ✅ 通用 server
├── pkg/
│   ├── actor/                   ✅ actor + system + message + game_actor
│   ├── engine/                  ✅ engine 接口 + js_engine (goja)
│   ├── websocket/               ✅ gateway (连接管理+房间广播)
│   ├── config/                  ✅
│   ├── db/                      ✅
│   ├── auth/                    ✅ JWT
│   ├── api/                     ✅ 统一响应
│   └── apperror/                ✅ 统一错误码
├── sdks/                        ✅
│   ├── server-sdk/              ✅ GameServer 抽象类 + register() + types
│   └── client-sdk/              ✅ GameClient + Connection + EventEmitter + types
├── deploy/                      ✅
│   ├── docker-compose.yml       ✅
│   ├── docker/Dockerfile.service ✅
│   └── config/*.env             ✅ (4个服务)
├── examples/                    ✅
│   └── games/blackjack/         ✅ 完整 21 点示例（服务端+客户端）
└── data/games/                  (运行时创建)
```

---

## 架构概览

4个微服务 + 2个SDK + 1个前端：

```
[Vue 3 大厅 SPA]
       |
       | 加载游戏前端 (独立页面跳转)
       v
[游戏前端 SPA] -- 使用 @gameplatform/client-sdk
       |
       | WebSocket (事件驱动)
       v
[API Gateway :8080] --> [User Service :8001]  (gRPC:9001)
                    --> [Game Service :8002]  (gRPC:9002) -- 使用 @gameplatform/server-sdk (注入goja)
                    --> [Admin Service :8003] (gRPC:9003)
```

- 服务间通信：gRPC
- 数据库：PostgreSQL + Redis
- JS游戏引擎：goja
- 游戏前端加载：独立页面跳转
- SDK分发：npm 包（TypeScript）

---

## 核心：Actor 模型驱动 + 双端 SDK 事件驱动架构

### Actor 模型核心设计

**一个 GameActor = 一个游戏房间 = 一个 goja 引擎实例 = 一个 goroutine**

```
┌─────────────────────── Actor System ──────────────────────────┐
│                                                                │
│  GameActor[room-001]     GameActor[room-002]     GameActor[...]│
│  ┌──────────────────┐    ┌──────────────────┐                  │
│  │ goroutine (独占)  │    │ goroutine (独占)  │                  │
│  │                  │    │                  │                  │
│  │ inbox chan Msg   │    │ inbox chan Msg   │                  │
│  │   ↓              │    │   ↓              │                  │
│  │ [处理循环]        │    │ [处理循环]        │                  │
│  │   ↓              │    │   ↓              │                  │
│  │ goja.Runtime     │    │ goja.Runtime     │                  │
│  │ (JS游戏脚本)      │    │ (JS游戏脚本)      │                  │
│  │   ↓              │    │   ↓              │                  │
│  │ broadcast/sendTo │    │ broadcast/sendTo │                  │
│  └────────┬─────────┘    └────────┬─────────┘                  │
│           │                       │                            │
│  ┌────────▼───────────────────────▼──────────────────────────┐│
│  │                    WebSocket Hub                          ││
│  │  conn-map: playerID → WebSocket Conn                     ││
│  │  将 Actor 输出的事件路由到对应的 WebSocket 连接              ││
│  └──────────────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────────────────┘

关键特性：
- 每个 GameActor 独占一个 goroutine，通过 inbox channel 串行化消息处理
- goja Runtime 只在 Actor 的 goroutine 中执行，天然线程安全，无需任何锁
- 多个 GameActor 并行运行在不同 goroutine，充分利用多核
- WebSocket Hub 负责将 Actor 输出的事件推送到对应的客户端连接
```

### 消息流转全链路

```
玩家点击"出牌"按钮
     │
     ▼
前端 SDK: this.sendAction('play_card', { card: 7 })
     │
     ▼ WebSocket JSON: { type:"action", data:{action:"play_card", data:{card:7}} }
     │
     ▼
WebSocket Hub 收到消息，根据 conn 的 roomID 路由
     │
     ▼
GameActor[roomID].Send(PlayerActionMsg{playerID, action, data})
     │    非阻塞写入 inbox channel（带缓冲）
     │
     ▼
GameActor 处理循环（单 goroutine，无锁）:
  1. 从 inbox 取出 PlayerActionMsg
  2. 调用 goja engine: vm.Call("onPlayerAction", playerID, action, data)
  3. JS 脚本执行游戏逻辑（SDK 层面）
  4. JS 中调用 this.broadcast('state_update', newState)
  5. 触发 Go 宿主函数 __platform_broadcast(event, data)
  6. 宿主函数调用 Hub.BroadcastToRoom(roomID, event, data)
     │
     ▼
WebSocket Hub 将事件推送给房间内所有玩家连接
     │
     ▼
前端 SDK 收到 { type:"state_update", data:{...} }
     │
     ▼
触发 onStateUpdate() 钩子，UI 更新
```

### GameActor 消息类型定义

```go
// pkg/actor/message.go

type MsgType int

const (
    MsgPlayerJoin    MsgType = iota + 1 // 玩家加入
    MsgPlayerLeave                       // 玩家离开
    MsgPlayerReady                       // 玩家准备
    MsgPlayerAction                      // 玩家操作
    MsgGameStart                         // 开始游戏
    MsgGameTick                          // 游戏帧（实时游戏）
    MsgTimer                             // 定时器回调
    MsgGameEnd                           // 游戏结束
    MsgRestore                           // 状态恢复
    MsgShutdown                          // 关闭 Actor
)

type Message struct {
    Type      MsgType
    PlayerID  string
    Action    string         // for MsgPlayerAction
    Data      any            // 携带数据
    Timestamp int64
    Reply     chan any       // 可选：需要同步返回结果时使用
}
```

### GameActor 核心实现骨架

```go
// pkg/actor/game_actor.go

type GameActor struct {
    id        string              // "game:{gameID}:{roomID}"
    gameID    string
    roomID    string
    gameCode  string
    inbox     chan *Message       // 有缓冲消息队列，串行化所有操作
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup

    // 引擎（独占，只在 Run goroutine 中访问）
    vm        *goja.Runtime       // goja JS 引擎实例

    // 回调（注入的平台能力）
    hub       HubInterface        // WebSocket Hub 接口
    roomMgr   RoomManagerInterface

    // 运行时状态
    players   map[string]*PlayerState
    gameState any                 // JS 脚本维护的游戏状态
    isRunning bool
    stats     ActorStats
}

// Start 启动 Actor 处理循环
func (a *GameActor) Start() {
    a.wg.Add(1)
    go a.run()
}

// run 核心处理循环 —— 单 goroutine，无锁
func (a *GameActor) run() {
    defer a.wg.Done()
    for {
        select {
        case msg, ok := <-a.inbox:
            if !ok {
                return // inbox closed
            }
            a.processMessage(msg)
        case <-a.ctx.Done():
            return
        }
    }
}

// processMessage 串行处理每一条消息（无需任何锁）
func (a *GameActor) processMessage(msg *Message) {
    start := time.Now()
    defer func() {
        if r := recover(); r != nil {
            // JS 脚本 panic 捕获
            a.hub.SendTo(msg.PlayerID, "error", map[string]any{
                "code": 47001, "message": "脚本执行异常",
            })
        }
        a.stats.Record(time.Since(start))
    }()

    switch msg.Type {
    case MsgPlayerJoin:
        a.handlePlayerJoin(msg)
    case MsgPlayerLeave:
        a.handlePlayerLeave(msg)
    case MsgPlayerReady:
        a.handlePlayerReady(msg)
    case MsgPlayerAction:
        a.handlePlayerAction(msg)
    case MsgGameStart:
        a.handleGameStart(msg)
    case MsgGameTick:
        a.handleGameTick(msg)
    case MsgTimer:
        a.handleTimer(msg)
    case MsgShutdown:
        a.handleShutdown(msg)
    }
}

// handlePlayerAction 核心路径：玩家操作 → goja 执行 → 广播
func (a *GameActor) handlePlayerAction(msg *Message) {
    // 调用 goja 引擎执行 JS 脚本中的 onPlayerAction 钩子
    fn, ok := goja.AssertFunction(a.vm.Get("onPlayerAction"))
    if !ok {
        return
    }
    _, err := fn(nil, a.vm.ToValue(msg.PlayerID),
                     a.vm.ToValue(msg.Action),
                     a.vm.ToValue(msg.Data))
    if err != nil {
        a.hub.SendTo(msg.PlayerID, "error", map[string]any{
            "code": 47001, "message": err.Error(),
        })
    }
}

// handlePlayerJoin 玩家加入
func (a *GameActor) handlePlayerJoin(msg *Message) {
    playerID := msg.PlayerID
    playerInfo := msg.Data.(map[string]any)

    a.players[playerID] = &PlayerState{
        ID:       playerID,
        Nickname: playerInfo["nickname"].(string),
        Avatar:   playerInfo["avatar"].(string),
        IsReady:  false,
    }

    // 调用 JS 钩子
    fn, _ := goja.AssertFunction(a.vm.Get("onPlayerJoin"))
    if fn != nil {
        fn(nil, a.vm.ToValue(playerID), a.vm.ToValue(playerInfo))
    }

    // 广播给房间内所有人
    a.hub.BroadcastToRoom(a.roomID, "player_join", map[string]any{
        "playerId": playerID,
        "nickname": playerInfo["nickname"],
        "avatar":   playerInfo["avatar"],
    })
}

// Send 非阻塞发送消息到 inbox
func (a *GameActor) Send(msg *Message) error {
    select {
    case a.inbox <- msg:
        return nil
    default:
        return ErrInboxFull // inbox 满时返回错误，不阻塞调用者
    }
}
```

### Hub 接口（Actor 与 WebSocket 的桥梁）

```go
// pkg/actor/hub.go

type HubInterface interface {
    // 广播到房间内所有 WebSocket 连接
    BroadcastToRoom(roomID string, event string, data any)
    // 发送给指定玩家
    SendTo(playerID string, event string, data any)
    // 发送给除指定玩家外的所有人
    SendExcept(roomID string, exceptPlayerID string, event string, data any)
}

// RoomManagerInterface 房间管理接口
type RoomManagerInterface interface {
    GetPlayerIDs(roomID string) []string
    GetConfig(roomID string) map[string]any
    UpdateRoomStatus(roomID string, status string)
    RecordGameResults(roomID string, results any)
}
```

### ActorSystem（Actor 注册中心）

```go
// pkg/actor/system.go

type ActorSystem struct {
    actors sync.Map   // actorID -> *GameActor
}

// Get 获取 Actor（用于路由消息）
func (s *ActorSystem) Get(actorID string) (*GameActor, bool) {
    val, ok := s.actors.Load(actorID)
    if !ok {
        return nil, false
    }
    return val.(*GameActor), true
}

// Register 注册新 Actor
func (s *ActorSystem) Register(actor *GameActor) error {
    if _, exists := s.actors.LoadOrStore(actor.id, actor); exists {
        return ErrActorExists
    }
    actor.Start()
    return nil
}

// Unregister 注销并停止 Actor
func (s *ActorSystem) Unregister(actorID string) {
    if val, ok := s.actors.LoadAndDelete(actorID); ok {
        actor := val.(*GameActor)
        actor.Send(&Message{Type: MsgShutdown})
        actor.Wait() // 等待 goroutine 退出
    }
}

// SendToActor 便捷方法：通过 roomID 找到对应 Actor 并发送消息
func (s *ActorSystem) SendToActor(roomID string, msg *Message) error {
    actorID := "game:" + roomID
    actor, ok := s.Get(actorID)
    if !ok {
        return ErrActorNotFound
    }
    return actor.Send(msg)
}
```

### Actor 与 goja 引擎的初始化

```go
// pkg/engine/js_engine.go

type GojaEngine struct {
    vm       *goja.Runtime
    hub      HubInterface
    roomMgr  RoomManagerInterface
    timers   map[int]*time.Timer
    timerID  int32
    roomID   string
    gameID   string
}

func NewGojaEngine(ctx *EngineContext, hub HubInterface, roomMgr RoomManagerInterface) *GojaEngine {
    e := &GojaEngine{
        vm:      goja.New(),
        hub:     hub,
        roomMgr: roomMgr,
        timers:  make(map[int]*time.Timer),
        roomID:  ctx.RoomID,
        gameID:  ctx.GameID,
    }
    e.injectHostAPI()
    return e
}

// injectHostAPI 注入 SDK runtime 的底层平台函数
func (e *GojaEngine) injectHostAPI() {
    vm := e.vm

    // --- 通信 API ---
    vm.Set("__platform_broadcast", func(call goja.FunctionCall) goja.Value {
        event := call.Argument(0).String()
        data := call.Argument(1).Export()
        e.hub.BroadcastToRoom(e.roomID, event, data)
        return vm.ToValue(true)
    })

    vm.Set("__platform_sendTo", func(call goja.FunctionCall) goja.Value {
        playerID := call.Argument(0).String()
        event := call.Argument(1).String()
        data := call.Argument(2).Export()
        e.hub.SendTo(playerID, event, data)
        return vm.ToValue(true)
    })

    vm.Set("__platform_sendExcept", func(call goja.FunctionCall) goja.Value {
        exceptPID := call.Argument(0).String()
        event := call.Argument(1).String()
        data := call.Argument(2).Export()
        e.hub.SendExcept(e.roomID, exceptPID, event, data)
        return vm.ToValue(true)
    })

    // --- 游戏控制 ---
    vm.Set("__platform_endGame", func(call goja.FunctionCall) goja.Value {
        results := call.Argument(0).Export()
        e.roomMgr.RecordGameResults(e.roomID, results)
        e.hub.BroadcastToRoom(e.roomID, "game_end", results)
        return vm.ToValue(true)
    })

    // --- 房间信息 ---
    vm.Set("__platform_getPlayers", func(call goja.FunctionCall) goja.Value {
        players := e.roomMgr.GetPlayerIDs(e.roomID)
        return vm.ToValue(players)
    })

    vm.Set("__platform_getRoomConfig", func(call goja.FunctionCall) goja.Value {
        config := e.roomMgr.GetConfig(e.roomID)
        return vm.ToValue(config)
    })

    // --- 定时器 ---
    vm.Set("__platform_setTimeout", func(call goja.FunctionCall) goja.Value {
        fn := call.Argument(0)
        ms := call.Argument(1).ToInteger()
        id := int(atomic.AddInt32(&e.timerID, 1))
        e.timers[id] = time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
            // 定时器回调通过 Actor inbox 串行化，保持无锁
            actor := actorSystem.Get("game:" + e.roomID)
            if actor != nil {
                actor.Send(&Message{Type: MsgTimer, Data: TimerData{ID: id, Fn: fn}})
            }
        })
        return vm.ToValue(id)
    })

    vm.Set("__platform_clearTimeout", func(call goja.FunctionCall) goja.Value {
        id := int(call.Argument(0).ToInteger())
        if t, ok := e.timers[id]; ok {
            t.Stop()
            delete(e.timers, id)
        }
        return vm.Undefined()
    })

    // --- 工具 ---
    vm.Set("__platform_randomInt", e.randomInt)
    vm.Set("__platform_shuffle", e.shuffle)
    vm.Set("__platform_uuid", e.uuid)
    vm.Set("__platform_log", e.log)
}

// LoadScript 加载游戏脚本
func (e *GojaEngine) LoadScript(scriptContent string) error {
    // 先加载 SDK runtime（GameServer 基类实现）
    if _, err := e.vm.RunString(serverSDKRuntime); err != nil {
        return fmt.Errorf("load sdk runtime: %w", err)
    }
    // 再加载第三方游戏脚本
    if _, err := e.vm.RunString(scriptContent); err != nil {
        return fmt.Errorf("load game script: %w", err)
    }
    return nil
}

// TriggerHook 触发 JS 生命周期钩子
func (e *GojaEngine) TriggerHook(hookName string, args ...any) (any, error) {
    fn, ok := goja.AssertFunction(e.vm.Get(hookName))
    if !ok {
        return nil, nil // hook 未实现，忽略
    }
    goArgs := make([]goja.Value, len(args))
    for i, arg := range args {
        goArgs[i] = e.vm.ToValue(arg)
    }
    result, err := fn(nil, goArgs...)
    if err != nil {
        return nil, err
    }
    return result.Export(), nil
}
```

### 并发安全保证（为什么无锁）

```
                    ┌───── goroutine 1 ─────┐
                    │  GameActor[room-001]   │
                    │  ┌──────────────┐      │
                    │  │ inbox chan   │      │  单 goroutine 串行处理
                    │  │ ──▶ process  │      │  goja Runtime 独占访问
                    │  │ ──▶ goja vm  │      │  无需任何 mutex
                    │  │ ──▶ broadcast│      │
                    │  └──────────────┘      │
                    └────────────────────────┘

                    ┌───── goroutine 2 ─────┐
                    │  GameActor[room-002]   │  独立 goroutine
                    │  ┌──────────────┐      │  互不干扰
                    │  │ inbox chan   │      │  充分利用多核
                    │  └──────────────┘      │
                    └────────────────────────┘

                    ┌───── goroutine N ─────┐
                    │  WebSocket Hub         │  负责消息路由
                    │  connMap (sync.Map)    │  写入 WebSocket 连接
                    └────────────────────────┘

安全保证：
1. goja Runtime 只在所属 Actor 的 goroutine 中访问 → 无数据竞争
2. inbox channel 是 Go 原生并发原语 → 无需手动加锁
3. Hub 使用 sync.Map 管理连接 → 并发安全
4. 定时器回调通过 Actor inbox 串行化 → 回调在 Actor goroutine 中执行
5. 每个房间状态完全隔离 → 房间间无共享状态
```

---

## 双端 SDK 事件驱动架构

### 设计理念

前后端 SDK 采用**对称的事件驱动模型**：

```
┌─────────────────────────────────────────────────────┐
│                    游戏生命周期                        │
│                                                       │
│  [created] → [waiting] → [ready] → [playing] → [ended] │
│                  ↑         ↓            │              │
│              player_join  all_ready  game_end          │
│              player_leave              │              │
│                                      results          │
└─────────────────────────────────────────────────────┘

前端 SDK (client-sdk)           后端 SDK (server-sdk)
┌─────────────────────┐         ┌─────────────────────┐
│  GameClient          │   WS    │  GameServer          │
│  ┌───────────────┐   │◄──────►│  ┌───────────────┐   │
│  │ Event Emitter │   │ Events │  │ Event Emitter │   │
│  └───────┬───────┘   │        │  └───────┬───────┘   │
│          │            │        │          │            │
│  生命周期钩子:         │        │  生命周期钩子:         │
│  onInit()             │        │  onInit()             │
│  onGameStart()        │        │  onGameStart()        │
│  onPlayerJoin()       │        │  onPlayerJoin()       │
│  onPlayerLeave()      │        │  onPlayerLeave()      │
│  onPlayerAction()     │        │  onPlayerAction()     │
│  onGameEnd()          │        │  onGameEnd()          │
│  onStateUpdate()      │        │                       │
│  onError()            │        │  平台API:              │
│                       │        │  game.broadcast()     │
│  操作方法:             │        │  game.sendTo()        │
│  game.sendAction()    │        │  game.endGame()       │
│  game.ready()         │        │  game.setState()      │
│  game.chat()          │        │  room.getPlayers()    │
│                       │        │  player.get/set()     │
│  状态查询:             │        │  timer.setTimeout()   │
│  game.getState()      │        │  random.int/shuffle() │
│  game.getPlayers()    │        │                       │
└─────────────────────┘         └─────────────────────┘
```

---

## 一、后端 SDK：@gameplatform/server-sdk

**运行环境**：goja 沙箱（Game Service 进程内）
**分发方式**：npm 包，提供类型声明 + 脚手架
**编译目标**：ES5（goja 兼容）

### 1.1 生命周期钩子

第三方开发者通过继承 `GameServer` 基类，实现生命周期钩子：

```typescript
// 第三方开发者编写 server.ts（编译为 server.js）
import { GameServer, GameContext } from '@gameplatform/server-sdk'

export default class MyGame extends GameServer {

  // ========== 生命周期钩子 ==========

  /** 游戏初始化（房间创建后调用一次） */
  onInit(ctx: GameContext): void {
    this.state = {
      phase: 'waiting',
      deck: this.utils.shuffle([...Array(52).keys()]),
      players: {},
      pot: 0,
    }
  }

  /** 玩家加入房间 */
  onPlayerJoin(playerId: string, playerInfo: PlayerInfo): void {
    this.state.players[playerId] = {
      chips: 1000,
      hand: [],
      bet: 0,
      folded: false,
    }
    this.broadcast('player_joined', { playerId, nickname: playerInfo.nickname })
  }

  /** 玩家离开房间 */
  onPlayerLeave(playerId: string, reason: LeaveReason): void {
    delete this.state.players[playerId]
    this.broadcast('player_left', { playerId, reason })
  }

  /** 游戏开始（所有玩家准备后触发） */
  onGameStart(): void {
    this.state.phase = 'playing'
    // 发牌
    for (const pid of Object.keys(this.state.players)) {
      this.state.players[pid].hand = [this.state.deck.pop(), this.state.deck.pop()]
    }
    this.broadcast('game_started', { state: this.getPublicState() })
  }

  /** 玩家操作（核心逻辑） */
  onPlayerAction(playerId: string, action: string, data: any): void {
    switch (action) {
      case 'bet':
        this.handleBet(playerId, data.amount)
        break
      case 'fold':
        this.handleFold(playerId)
        break
      case 'hit':
        this.handleHit(playerId)
        break
    }
    // 广播更新后的状态
    this.broadcast('state_update', { state: this.getPublicState() })
    // 检查游戏是否结束
    if (this.checkGameEnd()) {
      this.endGame(this.calculateResults())
    }
  }

  /** 游戏结束（由 endGame() 触发） */
  onGameEnd(results: GameResults): void {
    this.state.phase = 'ended'
    this.broadcast('game_ended', { results })
  }

  /** 游戏恢复（断线重连后） */
  onRestore(state: any): void {
    this.state = state
  }

  /** 定时Tick（仅实时游戏，需在 manifest 中声明 gameType: 'realtime'） */
  onTick(deltaTime: number): void {
    // 实时游戏每帧调用，deltaTime 单位毫秒
  }
}
```

### 1.2 平台 API（注入到 goja 沙箱中的宿主函数）

```typescript
declare abstract class GameServer {

  // ========== 状态管理 ==========

  /** 游戏状态（开发者自由定义结构） */
  state: Record<string, any>

  /** 获取公共状态（过滤掉敏感信息，如其他玩家的手牌） */
  abstract getPublicState(): Record<string, any>

  // ========== 通信 API ==========

  /** 广播消息给房间内所有玩家 */
  broadcast(event: string, data: any): void

  /** 发送消息给指定玩家（私密消息，其他玩家不可见） */
  sendTo(playerId: string, event: string, data: any): void

  /** 发送给除指定玩家外的所有人 */
  sendExcept(playerId: string, event: string, data: any): void

  // ========== 游戏控制 ==========

  /** 结束游戏并返回结果 */
  endGame(results: GameResults): void

  // ========== 房间信息 ==========

  /** 获取房间内所有玩家ID列表 */
  getPlayers(): string[]

  /** 获取房间配置 */
  getRoomConfig(): RoomConfig

  /** 获取当前玩家数量 */
  getPlayerCount(): number

  /** 获取房间ID */
  getRoomId(): string

  /** 获取游戏ID */
  getGameId(): string

  // ========== 玩家数据 ==========

  /** 获取玩家私有数据 */
  getPlayerData(playerId: string): Record<string, any>

  /** 设置玩家私有数据 */
  setPlayerData(playerId: string, key: string, value: any): void

  // ========== 定时器 ==========

  /** 延迟执行（返回 timerId） */
  setTimeout(callback: () => void, ms: number): number

  /** 周期执行（返回 timerId） */
  setInterval(callback: () => void, ms: number): number

  /** 取消定时器 */
  clearTimeout(id: number): void
  clearInterval(id: number): void

  // ========== 工具 ==========

  /** 随机整数 [min, max] */
  randomInt(min: number, max: number): number

  /** 洗牌（Fisher-Yates） */
  shuffle(array: any[]): any[]

  /** 从数组随机选一个 */
  randomChoice(array: any[]): any

  /** 生成 UUID */
  uuid(): string

  // ========== 日志 ==========

  /** 日志输出（会写入平台日志系统） */
  log(message: string, ...args: any[]): void
  warn(message: string, ...args: any[]): void
  error(message: string, ...args: any[]): void
}
```

### 1.3 类型定义

```typescript
// 类型声明文件（随 npm 包提供）

interface GameContext {
  gameId: string
  roomId: string
  gameCode: string
  version: string
  config: Record<string, any>    // manifest.json 中的 config
  minPlayers: number
  maxPlayers: number
}

interface PlayerInfo {
  playerId: string
  nickname: string
  avatar: string
  metadata: Record<string, any>
}

type LeaveReason = 'disconnect' | 'voluntary' | 'kicked' | 'timeout'

interface GameResults {
  winners: string[]              // 赢家ID列表
  scores: Record<string, number> // 每个玩家的得分
  stats: Record<string, any>     // 开发者自定义统计
  duration: number               // 游戏时长(ms)
}

interface RoomConfig {
  maxPlayers: number
  gameType: 'realtime' | 'turn-based'
  customConfig: Record<string, any>
}
```

---

## 二、前端 SDK：@gameplatform/client-sdk

**运行环境**：浏览器（游戏前端 SPA）
**分发方式**：npm 包，提供类型声明
**编译目标**：ES2015+

### 2.1 生命周期钩子

第三方前端开发者通过继承 `GameClient` 基类：

```typescript
// 第三方开发者编写 game.ts（编译为 game.js，打包到 client/ 目录）
import { GameClient } from '@gameplatform/client-sdk'

export default class MyGameUI extends GameClient {

  // ========== 生命周期钩子 ==========

  /** 游戏初始化（SDK连接成功后触发） */
  onInit(config: GameConfig): void {
    this.log('Game initialized', config)
    this.renderWaitingRoom()
  }

  /** 连接到房间成功 */
  onRoomJoined(roomInfo: RoomInfo): void {
    this.log('Joined room', roomInfo.roomId)
    this.renderLobby(roomInfo.players)
  }

  /** 其他玩家加入 */
  onPlayerJoin(player: PlayerInfo): void {
    this.addPlayerToList(player)
    this.showNotification(`${player.nickname} 加入了房间`)
  }

  /** 其他玩家离开 */
  onPlayerLeave(playerId: string, reason: string): void {
    this.removePlayerFromList(playerId)
  }

  /** 游戏开始 */
  onGameStart(data: GameStartData): void {
    this.renderGameBoard(data.state)
    this.playSound('game_start')
  }

  /** 收到游戏状态更新 */
  onStateUpdate(state: any): void {
    this.updateGameBoard(state)
  }

  /** 收到私密消息（如自己的手牌） */
  onPrivateMessage(event: string, data: any): void {
    if (event === 'your_hand') {
      this.renderMyCards(data.cards)
    }
  }

  /** 其他玩家的操作通知 */
  onPlayerAction(playerId: string, action: string, data: any): void {
    this.showPlayerAction(playerId, action, data)
    this.playSound(action)
  }

  /** 聊天消息 */
  onChat(from: string, message: string): void {
    this.addChatMessage(from, message)
  }

  /** 游戏结束 */
  onGameEnd(results: GameResults): void {
    this.renderResults(results)
    this.playSound('game_end')
  }

  /** 连接断开 */
  onDisconnect(reason: string): void {
    this.showReconnecting()
  }

  /** 重新连接成功 */
  onReconnect(): void {
    this.hideReconnecting()
  }

  /** 错误 */
  onError(error: GameError): void {
    this.showError(error.message)
  }
}
```

### 2.2 客户端 API

```typescript
declare abstract class GameClient {

  // ========== 玩家操作 ==========

  /** 发送操作给后端（核心方法） */
  sendAction(action: string, data?: any): void

  /** 标记准备 */
  ready(): void

  /** 发送聊天消息 */
  chat(message: string): void

  // ========== 状态查询 ==========

  /** 获取本地缓存的游戏状态 */
  getState(): Record<string, any>

  /** 获取当前房间信息 */
  getRoomInfo(): RoomInfo

  /** 获取当前玩家信息 */
  getCurrentPlayer(): PlayerInfo

  /** 获取房间内所有玩家 */
  getPlayers(): PlayerInfo[]

  // ========== 连接管理 ==========

  /** 获取连接状态 */
  getStatus(): ConnectionStatus

  /** 获取我的玩家ID */
  getMyPlayerId(): string

  // ========== 音频（平台提供基础音频管理） ==========

  /** 播放音效 */
  playSound(name: string): void

  /** 播放背景音乐 */
  playBGM(name: string): void

  /** 停止背景音乐 */
  stopBGM(): void

  // ========== 日志 ==========

  log(message: string, ...args: any[]): void
  warn(message: string, ...args: any[]): void
  error(message: string, ...args: any[]): void
}
```

### 2.3 初始化与连接

```typescript
// 游戏入口（游戏前端 SPA 的 main.ts）
import MyGameUI from './MyGameUI'

// SDK 从 URL 参数或全局变量自动获取配置
const game = new MyGameUI({
  // 平台注入的配置（自动从 URL params 获取）
  // ?token=xxx&gameId=xxx&roomId=xxx&gameServerUrl=xxx
  autoConnect: true,       // 自动连接 WebSocket
  reconnect: true,         // 断线自动重连
  reconnectInterval: 3000, // 重连间隔
  maxReconnectAttempts: 5, // 最大重连次数
})

// 或手动指定配置
const game = new MyGameUI({
  serverUrl: 'ws://localhost:8080/ws',
  token: 'jwt-token-here',
  gameId: 'game-123',
  roomId: 'room-456',
})
```

### 2.4 类型定义

```typescript
interface GameConfig {
  gameId: string
  gameCode: string
  version: string
  gameType: 'realtime' | 'turn-based'
  minPlayers: number
  maxPlayers: number
  customConfig: Record<string, any>
}

interface RoomInfo {
  roomId: string
  roomName: string
  owner: string
  players: PlayerInfo[]
  maxPlayers: number
  status: 'waiting' | 'playing' | 'ended'
  config: Record<string, any>
}

interface PlayerInfo {
  playerId: string
  nickname: string
  avatar: string
  isOwner: boolean
  isReady: boolean
  metadata: Record<string, any>
}

interface GameStartData {
  state: Record<string, any>
  yourRole?: string
  yourData?: Record<string, any>
}

interface GameResults {
  winners: string[]
  scores: Record<string, number>
  stats: Record<string, any>
  duration: number
}

interface GameError {
  code: number
  message: string
  data?: Record<string, any>
}

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting'
```

---

## 三、WebSocket 事件协议

前后端 SDK 之间通过以下事件协议通信（JSON格式）：

### 3.1 客户端 → 服务端事件

| 事件 | 数据 | 说明 |
|------|------|------|
| `ready` | `{}` | 玩家准备 |
| `action` | `{ action, data }` | 游戏操作 |
| `chat` | `{ message }` | 聊天消息 |

### 3.2 服务端 → 客户端事件

| 事件 | 数据 | 说明 |
|------|------|------|
| `connected` | `{ playerId, roomInfo }` | 连接成功，返回房间信息 |
| `player_join` | `{ playerId, nickname, avatar }` | 玩家加入 |
| `player_leave` | `{ playerId, reason }` | 玩家离开 |
| `player_ready` | `{ playerId }` | 玩家准备 |
| `game_start` | `{ state, yourData }` | 游戏开始 |
| `state_update` | `{ state }` | 公共状态更新 |
| `private_message` | `{ event, data }` | 私密消息（仅发送给指定玩家） |
| `player_action` | `{ playerId, action, data }` | 其他玩家操作通知 |
| `chat` | `{ from, message, nickname }` | 聊天消息 |
| `game_end` | `{ results }` | 游戏结束 |
| `error` | `{ code, message }` | 错误 |
| `pong` | `{ ts }` | 心跳响应 |

### 3.3 消息格式

```json
{
  "type": "action",
  "data": {
    "action": "bet",
    "data": { "amount": 100 }
  },
  "seq": 42,
  "ts": 1711605600000
}
```

---

## 四、SDK 与平台集成方式

### 4.1 后端 SDK 集成（Go goja 层）

```go
// pkg/engine/js_engine.go 中的宿主API注入

func (e *GojaEngine) injectHostAPI(vm *goja.Runtime, ctx *EngineContext) {
    // 注入 GameServer 基类的方法实现
    vm.Set("__platform_broadcast", func(call goja.FunctionCall) goja.Value {
        event := call.Argument(0).String()
        data := call.Argument(1).Export()
        e.hub.BroadcastToRoom(ctx.RoomID, event, data)
        return vm.ToValue(true)
    })

    vm.Set("__platform_sendTo", func(call goja.FunctionCall) goja.Value {
        playerID := call.Argument(0).String()
        event := call.Argument(1).String()
        data := call.Argument(2).Export()
        e.hub.SendToPlayer(ctx.RoomID, playerID, event, data)
        return vm.ToValue(true)
    })

    vm.Set("__platform_sendExcept", func(call goja.FunctionCall) goja.Value {
        exceptPlayerID := call.Argument(0).String()
        event := call.Argument(1).String()
        data := call.Argument(2).Export()
        e.hub.SendExcept(ctx.RoomID, exceptPlayerID, event, data)
        return vm.ToValue(true)
    })

    vm.Set("__platform_endGame", func(call goja.FunctionCall) goja.Value {
        results := call.Argument(0).Export()
        e.onGameEnd(ctx.RoomID, results)
        return vm.ToValue(true)
    })

    vm.Set("__platform_getPlayers", func(call goja.FunctionCall) goja.Value {
        players := e.roomMgr.GetPlayerIDs(ctx.RoomID)
        return vm.ToValue(players)
    })

    vm.Set("__platform_getRoomConfig", func(call goja.FunctionCall) goja.Value {
        config := e.roomMgr.GetConfig(ctx.RoomID)
        return vm.ToValue(config)
    })

    vm.Set("__platform_log", func(call goja.FunctionCall) goja.Value {
        msg := call.Argument(0).String()
        log.Printf("[Game:%s Room:%s] %s", ctx.GameID, ctx.RoomID, msg)
        return vm.Undefined()
    })

    vm.Set("__platform_randomInt", func(call goja.FunctionCall) goja.Value {
        min := call.Argument(0).ToInteger()
        max := call.Argument(1).ToInteger()
        n := rand.Intn(int(max-min+1)) + int(min)
        return vm.ToValue(n)
    })

    vm.Set("__platform_shuffle", func(call goja.FunctionCall) goja.Value {
        // Fisher-Yates shuffle
        arr := call.Argument(0).Export().([]interface{})
        shuffled := make([]interface{}, len(arr))
        copy(shuffled, arr)
        for i := len(shuffled) - 1; i > 0; i-- {
            j := rand.Intn(i + 1)
            shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
        }
        return vm.ToValue(shuffled)
    })

    vm.Set("__platform_setTimeout", func(call goja.FunctionCall) goja.Value {
        fn := call.Argument(0)
        ms := call.Argument(1).ToInteger()
        id := e.setTimer(ctx.RoomID, fn, time.Duration(ms)*time.Millisecond, false)
        return vm.ToValue(id)
    })

    vm.Set("__platform_setInterval", func(call goja.FunctionCall) goja.Value {
        fn := call.Argument(0)
        ms := call.Argument(1).ToInteger()
        id := e.setTimer(ctx.RoomID, fn, time.Duration(ms)*time.Millisecond, true)
        return vm.ToValue(id)
    })

    vm.Set("__platform_clearTimeout", func(call goja.FunctionCall) goja.Value {
        id := call.Argument(0).ToInteger()
        e.clearTimer(int(id))
        return vm.Undefined()
    })

    vm.Set("__platform_uuid", func(call goja.FunctionCall) goja.Value {
        return vm.ToValue(uuid.New().String())
    })
}
```

### 4.2 前端 SDK 核心实现

```typescript
// @gameplatform/client-sdk 核心类
export class GameClient {
  private ws: WebSocket | null = null
  private listeners: Map<string, Function[]> = new Map()
  private seq = 0
  private state: Record<string, any> = {}
  private roomInfo: RoomInfo | null = null
  private options: GameClientOptions

  constructor(options: Partial<GameClientOptions>) {
    this.options = { ...defaultOptions, ...options }
    if (this.options.autoConnect) {
      this.connect()
    }
  }

  // === 连接管理 ===
  connect(): void {
    const url = this.buildWsUrl()
    this.ws = new WebSocket(url)
    this.ws.onmessage = (e) => this.handleMessage(JSON.parse(e.data))
    this.ws.onclose = (e) => this.handleDisconnect(e.reason)
    this.ws.onerror = () => this.emit('error', { code: 0, message: 'Connection error' })
  }

  // === 事件系统 ===
  on(event: string, handler: Function): void
  once(event: string, handler: Function): void
  off(event: string, handler: Function): void

  // === 消息发送 ===
  sendAction(action: string, data?: any): void {
    this.send({ type: 'action', data: { action, data } })
  }

  ready(): void {
    this.send({ type: 'ready', data: {} })
  }

  chat(message: string): void {
    this.send({ type: 'chat', data: { message } })
  }

  // === 内部方法 ===
  private send(msg: any): void {
    this.ws!.send(JSON.stringify({ ...msg, seq: ++this.seq, ts: Date.now() }))
  }

  private handleMessage(msg: any): void {
    const { type, data } = msg
    switch (type) {
      case 'connected':
        this.roomInfo = data.roomInfo
        this.emit('init', data.config)
        this.emit('roomJoined', data.roomInfo)
        break
      case 'player_join':
        this.emit('playerJoin', data)
        break
      case 'player_leave':
        this.emit('playerLeave', data.playerId, data.reason)
        break
      case 'game_start':
        this.emit('gameStart', data)
        break
      case 'state_update':
        this.state = data.state
        this.emit('stateUpdate', data.state)
        break
      case 'private_message':
        this.emit('privateMessage', data.event, data.data)
        break
      case 'player_action':
        this.emit('playerAction', data.playerId, data.action, data.data)
        break
      case 'game_end':
        this.emit('gameEnd', data.results)
        break
      case 'error':
        this.emit('error', data)
        break
    }
  }
}
```

---

## 五、游戏包目录约定（更新）

```
blackjack-v1.0.0.zip
├── manifest.json          # 游戏元数据
├── server/                # 后端游戏逻辑
│   ├── main.ts            # 入口（编译为 main.js）
│   └── game-logic.ts      # 辅助模块
├── client/                # 前端游戏资源
│   ├── index.html         # 游戏入口
│   ├── src/
│   │   ├── main.ts        # 前端入口
│   │   ├── GameUI.ts      # 游戏UI逻辑
│   │   └── renderer.ts    # 渲染器
│   ├── package.json       # 前端依赖（含 @gameplatform/client-sdk）
│   └── vite.config.ts     # Vite 配置
├── tsconfig.json          # 后端TS配置（编译目标ES5）
└── package.json           # 后端依赖（含 @gameplatform/server-sdk）
```

### manifest.json 完整格式

```json
{
  "name": "blackjack",
  "version": "1.0.0",
  "gameCode": "BLACKJACK",
  "gameType": "turn-based",
  "entry": "server/main.js",
  "clientEntry": "client/index.html",
  "minPlayers": 2,
  "maxPlayers": 7,
  "description": "21点纸牌游戏",
  "author": "GameStudio",
  "config": {
    "deckCount": 6,
    "initialChips": 1000,
    "betLimits": [10, 500]
  }
}
```

---

## 六、实施阶段（更新于 2026-03-29）

### Phase 1: 基础设施 + Actor 内核 ✅ 已完成
1. ✅ 初始化 go.mod，引入核心依赖（goja, gin, gorm, gorilla/websocket, jwt, uuid, grpc）
2. ✅ proto 定义（user.proto, game.proto）+ Go stub 生成
3. ✅ 公共包：config, db, api/response, apperror, auth/jwt
4. ✅ **Actor 内核**：pkg/actor/
5. ✅ **goja 引擎**：pkg/engine/（含 sandbox.go 沙箱策略）
6. ✅ 通用 server 启动器（Gin + 优雅关闭 SIGINT/SIGTERM）
7. ✅ 单元测试（actor, auth, config, engine, websocket, grpc）

### Phase 2: WebSocket Hub ✅ 已完成
1. ✅ pkg/websocket/gateway.go + conn.go: ReadPump/WritePump + 心跳
2. ✅ WebSocket-Actor 桥接
3. ✅ WebSocket 代理（API Gateway → Game Service）

### Phase 3: 后端 SDK ✅ 已完成
### Phase 4: 前端 SDK ✅ 已完成
### Phase 5: User Service ✅ 已完成（含 gRPC Server）
### Phase 6: Admin Service ✅ 已完成（含 gRPC Client 鉴权）
### Phase 7: Game Service ✅ 已完成（含 gRPC Server）

### Phase 8: API Gateway ✅ 已完成
1. ✅ HTTP 反向代理 + WebSocket 代理
2. 🔲 限流、熔断

### Phase 9: 示例游戏 & 部署 ✅ 已完成
### Phase 10: gRPC 通信 ✅ 已完成
1. ✅ proto/user: ValidateToken + GetUser
2. ✅ proto/game: GetGame + ListGames + GetRoom
3. ✅ pkg/grpc/server.go: StartServer + GracefulStop
4. ✅ pkg/grpc/client.go: UserServiceClient + GameServiceClient
5. ✅ internal/user/grpc.go: gRPC server 实现
6. ✅ internal/game/grpc.go: gRPC server 实现
7. ✅ admin-service: gRPC 客户端连接 user-service 进行 token 验证
8. ✅ docker-compose.yml: 暴露 gRPC 端口 (9001, 9002)
9. ✅ env 配置: GRPC_PORT + USER_GRPC_ADDR

---

## 七、目录结构（更新）

```
online-game/
├── go.mod
├── Makefile
├── proto/
│   ├── user/
│   │   ├── user.proto
│   │   ├── user.pb.go
│   │   └── user_grpc.pb.go
│   └── game/
│       ├── game.proto
│       ├── game.pb.go
│       └── game_grpc.pb.go
├── cmd/
│   ├── api-gateway/main.go       # HTTP 反向代理 + WS 代理
│   ├── user-service/main.go      # HTTP + gRPC 双协议
│   ├── game-service/main.go      # HTTP + gRPC 双协议
│   └── admin-service/main.go     # gRPC 客户端鉴权
├── internal/
│   ├── user/
│   │   ├── model.go, service.go, handler.go, grpc.go
│   ├── game/
│   │   ├── model.go, service.go, handler.go, grpc.go
│   ├── admin/
│   │   └── handler.go
│   └── server/
│       └── server.go
├── pkg/
│   ├── actor/
│   ├── engine/
│   ├── websocket/
│   ├── grpc/                     # gRPC 服务器/客户端工具
│   │   ├── server.go
│   │   ├── client.go
│   │   └── grpc_test.go
│   ├── redis/
│   ├── config/, db/, auth/, api/, apperror/
├── sdks/
│   ├── server-sdk/
│   └── client-sdk/
├── deploy/
│   ├── docker-compose.yml
│   ├── docker/Dockerfile.service
│   └── config/*.env
└── examples/
    └── games/blackjack/
```
