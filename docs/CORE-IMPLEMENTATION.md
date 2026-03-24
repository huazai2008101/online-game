# 核心模块实现详解

**文档版本:** v1.0
**创建时间:** 2026-03-24
**描述:** 核心模块的完整实现代码示例

---

## 目录

1. [Actor 模型实现](#1-actor-模型实现)
2. [游戏引擎实现](#2-游戏引擎实现)
3. [WebSocket 实现](#3-websocket-实现)
4. [中间件实现](#4-中间件实现)
5. [数据访问层实现](#5-数据访问层实现)
6. [服务间通信实现](#6-服务间通信实现)
7. [缓存层实现](#7-缓存层实现)
8. [消息队列实现](#8-消息队列实现)

---

## 1. Actor 模型实现

### 1.1 核心接口定义

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

// Handler 消息处理函数
type Handler func(ctx context.Context, msg Message) error

// Actor Actor接口
type Actor interface {
    // ID 返回Actor ID
    ID() string

    // Type 返回Actor类型
    Type() string

    // Start 启动Actor
    Start(ctx context.Context) error

    // Stop 停止Actor
    Stop(ctx context.Context) error

    // Send 发送消息
    Send(ctx context.Context, msg Message) error

    // Tell 异步发送消息
    Tell(msg Message)

    // Ask 同步请求响应
    Ask(ctx context.Context, msg Message) (Message, error)

    // Restart 重启Actor
    Restart(ctx context.Context) error
}

// ActorRef Actor引用
type ActorRef struct {
    id   string
    path string
    sys  *ActorSystem
}

func (r *ActorRef) ID() string   { return r.id }
func (r *ActorRef) Path() string { return r.path }

func (r *ActorRef) Tell(msg Message) {
    r.sys.Tell(r.id, msg)
}

func (r *ActorRef) Ask(ctx context.Context, msg Message) (Message, error) {
    return r.sys.Ask(ctx, r.id, msg)
}
```

### 1.2 Actor 基础实现

```go
// internal/actor/base_actor.go
package actor

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// BaseActor 基础Actor实现
type BaseActor struct {
    id       string
    actorType string
    mailbox  Mailbox
    handler  Handler
    ctx      context.Context
    cancel   context.CancelFunc
    wg       sync.WaitGroup
    started  bool
    mu       sync.RWMutex
    supervisor Supervisor
    restartCount int
    maxRestarts  int
}

// NewBaseActor 创建基础Actor
func NewBaseActor(id, actorType string, handler Handler, opts ...ActorOption) *BaseActor {
    ctx, cancel := context.WithCancel(context.Background())

    a := &BaseActor{
        id:        id,
        actorType: actorType,
        handler:   handler,
        ctx:       ctx,
        cancel:    cancel,
        mailbox:   NewDefaultMailbox(1000),
        maxRestarts: 10,
    }

    // 应用选项
    for _, opt := range opts {
        opt(a)
    }

    return a
}

// ActorOption Actor配置选项
type ActorOption func(*BaseActor)

func WithMailbox(m Mailbox) ActorOption {
    return func(a *BaseActor) {
        a.mailbox = m
    }
}

func WithSupervisor(s Supervisor) ActorOption {
    return func(a *BaseActor) {
        a.supervisor = s
    }
}

func WithMaxRestarts(n int) ActorOption {
    return func(a *BaseActor) {
        a.maxRestarts = n
    }
}

// ID 实现Actor接口
func (a *BaseActor) ID() string {
    return a.id
}

// Type 实现Actor接口
func (a *BaseActor) Type() string {
    return a.actorType
}

// Start 启动Actor
func (a *BaseActor) Start(ctx context.Context) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    if a.started {
        return fmt.Errorf("actor already started: %s", a.id)
    }

    a.started = true
    a.ctx, a.cancel = context.WithCancel(ctx)

    a.wg.Add(1)
    go a.processLoop()

    return nil
}

// Stop 停止Actor
func (a *BaseActor) Stop(ctx context.Context) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    if !a.started {
        return nil
    }

    a.cancel()
    a.started = false

    done := make(chan struct{})
    go func() {
        a.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Send 发送消息
func (a *BaseActor) Send(ctx context.Context, msg Message) error {
    return a.mailbox.Push(ctx, msg)
}

// Tell 异步发送消息
func (a *BaseActor) Tell(msg Message) {
    a.mailbox.Push(context.Background(), msg)
}

// Ask 同步请求响应
func (a *BaseActor) Ask(ctx context.Context, msg Message) (Message, error) {
    // 实现请求-响应模式
    responseChan := make(chan Message, 1)
    errorChan := make(chan error, 1)

    // 包装消息，添加响应通道
    wrapper := &AskMessage{
        Message:       msg,
        ResponseChan:  responseChan,
        ErrorChan:     errorChan,
    }

    if err := a.Send(ctx, wrapper); err != nil {
        return nil, err
    }

    select {
    case resp := <-responseChan:
        return resp, nil
    case err := <-errorChan:
        return nil, err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

// Restart 重启Actor
func (a *BaseActor) Restart(ctx context.Context) error {
    a.mu.Lock()

    if a.restartCount >= a.maxRestarts {
        a.mu.Unlock()
        return fmt.Errorf("max restarts exceeded for actor: %s", a.id)
    }

    a.restartCount++
    a.mu.Unlock()

    // 停止当前Actor
    a.Stop(ctx)

    // 重新启动
    return a.Start(ctx)
}

// processLoop 消息处理循环
func (a *BaseActor) processLoop() {
    defer a.wg.Done()

    for {
        select {
        case <-a.ctx.Done():
            return
        default:
            msg, err := a.mailbox.Pop(a.ctx)
            if err != nil {
                if err == ErrMailboxClosed {
                    return
                }
                continue
            }

            // 处理消息
            if err := a.handleMessage(a.ctx, msg); err != nil {
                // 处理失败，通知supervisor
                if a.supervisor != nil {
                    a.supervisor.HandleFailure(a, err)
                }
            }
        }
    }
}

// handleMessage 处理单个消息
func (a *BaseActor) handleMessage(ctx context.Context, msg Message) error {
    defer func() {
        if r := recover(); r != nil {
            // 捕获panic，转换为错误
            if a.supervisor != nil {
                a.supervisor.HandleFailure(a, fmt.Errorf("panic: %v", r))
            }
        }
    }()

    return a.handler(ctx, msg)
}

// AskMessage 请求-响应消息包装
type AskMessage struct {
    Message
    ResponseChan chan<- Message
    ErrorChan     chan<- error
}

// Type 实现Message接口
func (m *AskMessage) Type() string {
    return "ask"
}

// Respond 发送响应
func (m *AskMessage) Respond(msg Message) {
    select {
    case m.ResponseChan <- msg:
    default:
    }
}

// RespondError 发送错误响应
func (m *AskMessage) RespondError(err error) {
    select {
    case m.ErrorChan <- err:
    default:
    }
}
```

### 1.3 Mailbox 实现

```go
// internal/actor/mailbox.go
package actor

import (
    "context"
    "sync"
)

var (
    ErrMailboxClosed = fmt.Errorf("mailbox closed")
    ErrMailboxFull   = fmt.Errorf("mailbox full")
)

// Mailbox 邮箱接口
type Mailbox interface {
    // Push 推送消息
    Push(ctx context.Context, msg Message) error
    // Pop 弹出消息（阻塞）
    Pop(ctx context.Context) (Message, error)
    // TryPop 尝试弹出消息（非阻塞）
    TryPop() (Message, error)
    // Size 返回当前消息数量
    Size() int
    // Close 关闭邮箱
    Close() error
}

// DefaultMailbox 默认邮箱实现（基于channel）
type DefaultMailbox struct {
    capacity int
    ch       chan Message
    mu       sync.RWMutex
    closed   bool
}

// NewDefaultMailbox 创建默认邮箱
func NewDefaultMailbox(capacity int) *DefaultMailbox {
    return &DefaultMailbox{
        capacity: capacity,
        ch:       make(chan Message, capacity),
    }
}

// Push 推送消息
func (m *DefaultMailbox) Push(ctx context.Context, msg Message) error {
    m.mu.RLock()
    if m.closed {
        m.mu.RUnlock()
        return ErrMailboxClosed
    }
    m.mu.RUnlock()

    select {
    case m.ch <- msg:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Pop 弹出消息
func (m *DefaultMailbox) Pop(ctx context.Context) (Message, error) {
    select {
    case msg, ok := <-m.ch:
        if !ok {
            return nil, ErrMailboxClosed
        }
        return msg, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

// TryPop 尝试弹出消息
func (m *DefaultMailbox) TryPop() (Message, error) {
    select {
    case msg, ok := <-m.ch:
        if !ok {
            return nil, ErrMailboxClosed
        }
        return msg, nil
    default:
        return nil, nil
    }
}

// Size 返回当前消息数量
func (m *DefaultMailbox) Size() int {
    return len(m.ch)
}

// Close 关闭邮箱
func (m *DefaultMailbox) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.closed {
        return nil
    }

    m.closed = true
    close(m.ch)
    return nil
}

// PriorityMailbox 优先级邮箱实现
type PriorityMailbox struct {
    capacity int
    items    []*priorityItem
    mu       sync.Mutex
    cond     *sync.Cond
    closed   bool
}

type priorityItem struct {
    msg       Message
    priority  int
    sequence  uint64
}

// NewPriorityMailbox 创建优先级邮箱
func NewPriorityMailbox(capacity int) *PriorityMailbox {
    pm := &PriorityMailbox{
        capacity: capacity,
        items:    make([]*priorityItem, 0, capacity),
    }
    pm.cond = sync.NewCond(&pm.mu)
    return pm
}

// Push 推送消息
func (m *PriorityMailbox) Push(ctx context.Context, msg Message) error {
    priority := NormalPriority
    if pm, ok := msg.(PriorityMessage); ok {
        priority = pm.Priority()
    }

    m.mu.Lock()
    defer m.mu.Unlock()

    if m.closed {
        return ErrMailboxClosed
    }

    if len(m.items) >= m.capacity {
        return ErrMailboxFull
    }

    item := &priorityItem{
        msg:      msg,
        priority: priority,
    }

    // 按优先级插入
    m.insertByPriority(item)
    m.cond.Signal()
    return nil
}

// insertByPriority 按优先级插入
func (m *PriorityMailbox) insertByPriority(item *priorityItem) {
    // 使用堆插入
    i := len(m.items)
    m.items = append(m.items, item)

    for i > 0 {
        parent := (i - 1) / 2
        if m.items[parent].priority >= item.priority {
            break
        }
        m.items[i], m.items[parent] = m.items[parent], m.items[i]
        i = parent
    }
}

// Pop 弹出消息
func (m *PriorityMailbox) Pop(ctx context.Context) (Message, error) {
    m.mu.Lock()

    for len(m.items) == 0 && !m.closed {
        m.cond.Wait()
    }

    if m.closed {
        m.mu.Unlock()
        return nil, ErrMailboxClosed
    }

    // 弹出最高优先级
    item := m.items[0]
    n := len(m.items) - 1
    m.items[0], m.items[n] = m.items[n], nil
    m.items = m.items[:n]

    // 堆化
    m.heapify(0)

    m.mu.Unlock()
    return item.msg, nil
}

// heapify 堆化
func (m *PriorityMailbox) heapify(i int) {
    for {
        left := 2*i + 1
        right := 2*i + 2
        largest := i

        if left < len(m.items) && m.items[left].priority > m.items[largest].priority {
            largest = left
        }
        if right < len(m.items) && m.items[right].priority > m.items[largest].priority {
            largest = right
        }

        if largest == i {
            break
        }

        m.items[i], m.items[largest] = m.items[largest], m.items[i]
        i = largest
    }
}

// PriorityMessage 优先级消息接口
type PriorityMessage interface {
    Message
    Priority() int
}

const (
    LowPriority    = 0
    NormalPriority = 1
    HighPriority   = 2
)
```

### 1.4 ActorSystem 实现

```go
// internal/actor/system.go
package actor

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// ActorSystem Actor系统
type ActorSystem struct {
    name      string
    actors    map[string]Actor
    mu        sync.RWMutex
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
    started   bool
    dispatcher Dispatcher
    router     *Router
}

// NewActorSystem 创建Actor系统
func NewActorSystem(name string, opts ...SystemOption) *ActorSystem {
    ctx, cancel := context.WithCancel(context.Background())

    sys := &ActorSystem{
        name:   name,
        actors: make(map[string]Actor),
        ctx:    ctx,
        cancel: cancel,
        dispatcher: NewDefaultDispatcher(),
    }

    // 应用选项
    for _, opt := range opts {
        opt(sys)
    }

    if sys.router == nil {
        sys.router = NewRouter(sys)
    }

    return sys
}

// SystemOption 系统配置选项
type SystemOption func(*ActorSystem)

func WithDispatcher(d Dispatcher) SystemOption {
    return func(s *ActorSystem) {
        s.dispatcher = d
    }
}

// Start 启动系统
func (s *ActorSystem) Start(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.started {
        return fmt.Errorf("system already started")
    }

    s.started = true
    s.ctx, s.cancel = context.WithCancel(ctx)

    return nil
}

// Stop 停止系统
func (s *ActorSystem) Stop(ctx context.Context) error {
    s.mu.Lock()
    s.started = false
    s.mu.Unlock()

    s.cancel()

    // 停止所有Actor
    s.mu.RLock()
    actors := make([]Actor, 0, len(s.actors))
    for _, actor := range s.actors {
        actors = append(actors, actor)
    }
    s.mu.RUnlock()

    // 并发停止
    var wg sync.WaitGroup
    for _, actor := range actors {
        wg.Add(1)
        go func(a Actor) {
            defer wg.Done()
            a.Stop(ctx)
        }(actor)
    }
    wg.Wait()

    return nil
}

// Spawn 创建并启动Actor
func (s *ActorSystem) Spawn(id, actorType string, handler Handler, opts ...ActorOption) (*ActorRef, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if _, exists := s.actors[id]; exists {
        return nil, fmt.Errorf("actor already exists: %s", id)
    }

    actor := NewBaseActor(id, actorType, handler, opts...)

    if err := actor.Start(s.ctx); err != nil {
        return nil, err
    }

    s.actors[id] = actor

    return &ActorRef{
        id:   id,
        path: fmt.Sprintf("/%s/%s", s.name, id),
        sys:  s,
    }, nil
}

// SpawnNamed 创建命名的Actor
func (s *ActorSystem) SpawnNamed(actorType string, handler Handler, opts ...ActorOption) (*ActorRef, error) {
    id := generateID(actorType)
    return s.Spawn(id, actorType, handler, opts...)
}

// StopActor 停止Actor
func (s *ActorSystem) StopActor(id string) error {
    s.mu.Lock()
    actor, exists := s.actors[id]
    if !exists {
        s.mu.Unlock()
        return fmt.Errorf("actor not found: %s", id)
    }
    delete(s.actors, id)
    s.mu.Unlock()

    return actor.Stop(s.ctx)
}

// Tell 异步发送消息
func (s *ActorSystem) Tell(id string, msg Message) {
    s.mu.RLock()
    actor, exists := s.actors[id]
    s.mu.RUnlock()

    if !exists {
        return
    }

    actor.Tell(msg)
}

// Ask 同步请求
func (s *ActorSystem) Ask(ctx context.Context, id string, msg Message) (Message, error) {
    s.mu.RLock()
    actor, exists := s.actors[id]
    s.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("actor not found: %s", id)
    }

    return actor.Ask(ctx, msg)
}

// Broadcast 广播消息
func (s *ActorSystem) Broadcast(msg Message) {
    s.mu.RLock()
    actors := make([]Actor, 0, len(s.actors))
    for _, actor := range s.actors {
        actors = append(actors, actor)
    }
    s.mu.RUnlock()

    for _, actor := range actors {
        actor.Tell(msg)
    }
}

// ActorOf 获取Actor引用
func (s *ActorSystem) ActorOf(id string) (*ActorRef, error) {
    s.mu.RLock()
    _, exists := s.actors[id]
    s.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("actor not found: %s", id)
    }

    return &ActorRef{
        id:   id,
        path: fmt.Sprintf("/%s/%s", s.name, id),
        sys:  s,
    }, nil
}

// generateID 生成唯一ID
func generateID(prefix string) string {
    return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
```

### 1.5 Supervisor 实现

```go
// internal/actor/supervisor.go
package actor

import (
    "errors"
    "log"
)

// Strategy 监督策略
type Strategy int

const (
    // OneForOne 只重启失败的Actor
    OneForOne Strategy = iota
    // OneForAll 重启所有Actor
    OneForAll
    // AllForOne 停止所有Actor
    AllForOne
)

// Supervisor 监督器接口
type Supervisor interface {
    // HandleFailure 处理失败
    HandleFailure(actor Actor, err error)
    // Strategy 返回监督策略
    Strategy() Strategy
}

// BaseSupervisor 基础监督器
type BaseSupervisor struct {
    strategy Strategy
    children []*ActorRef
    maxRestarts int
}

// NewSupervisor 创建监督器
func NewSupervisor(strategy Strategy, maxRestarts int) *BaseSupervisor {
    return &BaseSupervisor{
        strategy:    strategy,
        children:    make([]*ActorRef, 0),
        maxRestarts: maxRestarts,
    }
}

// AddChild 添加子Actor
func (s *BaseSupervisor) AddChild(ref *ActorRef) {
    s.children = append(s.children, ref)
}

// HandleFailure 处理失败
func (s *BaseSupervisor) HandleFailure(failedActor Actor, err error) {
    log.Printf("Actor %s failed: %v", failedActor.ID(), err)

    switch s.strategy {
    case OneForOne:
        s.restartOne(failedActor)
    case OneForAll:
        s.restartAll()
    case AllForOne:
        s.stopAll()
    }
}

// Strategy 返回策略
func (s *BaseSupervisor) Strategy() Strategy {
    return s.strategy
}

// restartOne 重启单个Actor
func (s *BaseSupervisor) restartOne(actor Actor) {
    ctx := context.Background()
    if err := actor.Restart(ctx); err != nil {
        log.Printf("Failed to restart actor %s: %v", actor.ID(), err)
    }
}

// restartAll 重启所有Actor
func (s *BaseSupervisor) restartAll() {
    ctx := context.Background()
    for _, ref := range s.children {
        actor, err := ref.sys.ActorOf(ref.id)
        if err != nil {
            continue
        }
        if err := actor.Restart(ctx); err != nil {
            log.Printf("Failed to restart actor %s: %v", ref.id, err)
        }
    }
}

// stopAll 停止所有Actor
func (s *BaseSupervisor) stopAll() {
    ctx := context.Background()
    for _, ref := range s.children {
        ref.sys.StopActor(ref.id)
    }
}

// Directive 指令
type Directive int

const (
    // Restart 重启
    Restart Directive = iota
    // Stop 停止
    Stop
    // Escalate 上报
    Escalate
    // Resume 忽略
    Resume
)

// Decider 决策函数
type Decider func(err error) Directive

// FunctionalSupervisor 函数式监督器
type FunctionalSupervisor struct {
    decider Decider
    children []*ActorRef
}

// NewFunctionalSupervisor 创建函数式监督器
func NewFunctionalSupervisor(decider Decider) *FunctionalSupervisor {
    return &FunctionalSupervisor{
        decider: decider,
        children: make([]*ActorRef, 0),
    }
}

// HandleFailure 处理失败
func (s *FunctionalSupervisor) HandleFailure(actor Actor, err error) {
    directive := s.decider(err)

    ctx := context.Background()
    switch directive {
    case Restart:
        actor.Restart(ctx)
    case Stop:
        actor.Stop(ctx)
    case Escalate:
        // 上报给上层监督器
        log.Printf("Escalating failure from actor %s: %v", actor.ID(), err)
    case Resume:
        // 忽略，继续运行
    }
}

// Strategy 返回策略
func (s *FunctionalSupervisor) Strategy() Strategy {
    return OneForOne
}

// DefaultDecider 默认决策函数
func DefaultDecider(err error) Directive {
    if errors.Is(err, context.Canceled) {
        return Stop
    }
    return Restart
}
```

---

## 2. 游戏引擎实现

### 2.1 引擎接口定义

```go
// internal/engine/interface.go
package engine

import (
    "context"
    "io"
)

// Engine 游戏引擎接口
type Engine interface {
    // Name 返回引擎名称
    Name() string

    // Type 返回引擎类型
    Type() EngineType

    // Init 初始化引擎
    Init(ctx context.Context, opts ...InitOption) error

    // LoadGame 加载游戏代码
    LoadGame(ctx context.Context, code []byte) error

    // Start 启动游戏
    Start(ctx context.Context) error

    // Stop 停止游戏
    Stop(ctx context.Context) error

    // Call 调用游戏函数
    Call(ctx context.Context, method string, args ...interface{}) (interface{}, error)

    // GetState 获取游戏状态
    GetState(ctx context.Context) (interface{}, error)

    // SetState 设置游戏状态
    SetState(ctx context.Context, state interface{}) error

    // EmitEvent 触发事件
    EmitEvent(ctx context.Context, event string, data interface{}) error

    // OnEvent 监听事件
    OnEvent(ctx context.Context, event string, handler EventHandler) error

    // MemoryStats 返回内存统计
    MemoryStats(ctx context.Context) (*MemoryStats, error)

    // GCOpts 返回GC选项
    GCOpts() []GCOpt

    // Close 关闭引擎
    Close() error
}

// EngineType 引擎类型
type EngineType string

const (
    EngineTypeJavaScript EngineType = "javascript"
    EngineTypeWASM       EngineType = "wasm"
    EngineTypeLua        EngineType = "lua"
)

// InitOption 初始化选项
type InitOption func(*InitConfig)

// InitConfig 初始化配置
type InitConfig struct {
    MaxMemoryBytes  int64
    Timeout         int
    EnableProfiling bool
    Logger          Logger
}

// WithMaxMemory 设置最大内存
func WithMaxMemory(bytes int64) InitOption {
    return func(c *InitConfig) {
        c.MaxMemoryBytes = bytes
    }
}

// WithTimeout 设置超时
func WithTimeout(timeout int) InitOption {
    return func(c *InitConfig) {
        c.Timeout = timeout
    }
}

// EventHandler 事件处理器
type EventHandler func(ctx context.Context, data interface{}) error

// MemoryStats 内存统计
type MemoryStats struct {
    Used     int64
    Allocated int64
    Limit    int64
}

// GCOpt GC选项
type GCOpt struct {
    Key   string
    Value interface{}
}

// Logger 日志接口
type Logger interface {
    Debug(msg string, args ...interface{})
    Info(msg string, args ...interface{})
    Warn(msg string, args ...interface{})
    Error(msg string, args ...interface{})
}
```

### 2.2 JavaScript 引擎实现

```go
// internal/engine/js_engine.go
package engine

import (
    "context"
    "errors"
    "fmt"
    "sync"
    "time"

    "github.com/dop251/goja"
)

// JSEngine JavaScript引擎
type JSEngine struct {
    vm       *goja.Runtime
    conf     InitConfig
    mu       sync.RWMutex
    started  bool
    handlers map[string][]EventHandler
    logger   Logger
}

// NewJSEngine 创建JavaScript引擎
func NewJSEngine(opts ...InitOption) *JSEngine {
    conf := InitConfig{
        MaxMemoryBytes:  50 * 1024 * 1024, // 50MB
        Timeout:         5000,
        EnableProfiling: false,
    }

    for _, opt := range opts {
        opt(&conf)
    }

    return &JSEngine{
        conf:     conf,
        handlers: make(map[string][]EventHandler),
        logger:   conf.Logger,
    }
}

// Name 实现Engine接口
func (e *JSEngine) Name() string {
    return "javascript"
}

// Type 实现Engine接口
func (e *JSEngine) Type() EngineType {
    return EngineTypeJavaScript
}

// Init 初始化引擎
func (e *JSEngine) Init(ctx context.Context, opts ...InitOption) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 应用选项
    for _, opt := range opts {
        opt(&e.conf)
    }

    // 创建VM
    e.vm = goja.New()

    // 设置内存限制
    e.vm.SetMaxCallStackSize(1000)

    // 注入内置函数
    e.injectBuiltins()

    return nil
}

// injectBuiltins 注入内置函数
func (e *JSEngine) injectBuiltins() {
    // 日志函数
    e.vm.Set("log", func(call goja.FunctionCall) goja.Value {
        msg := call.Argument(0).String()
        if e.logger != nil {
            e.logger.Info(msg)
        }
        return goja.Undefined()
    })

    // JSON.stringify
    e.vm.Set("JSON", map[string]interface{}{
        "stringify": func(call goja.FunctionCall) goja.Value {
            v := call.Argument(0)
            s, _ := goja.AssertFunction(e.vm.Get("JSON.stringify"))
            result, _ := s(goja.Undefined(), v)
            return result
        },
    })

    // setTimeout 模拟
    e.vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
        fn, ok := goja.AssertFunction(call.Argument(0))
        if !ok {
            return goja.Undefined()
        }

        delay := 0
        if len(call.Arguments) > 1 {
            delay = int(call.Argument(1).ToInteger())
        }

        go func() {
            time.Sleep(time.Duration(delay) * time.Millisecond)
            fn(goja.Undefined())
        }()

        return goja.Undefined()
    })

    // emitEvent 事件触发
    e.vm.Set("emitEvent", func(call goja.FunctionCall) goja.Value {
        event := call.Argument(0).String()
        data := call.Argument(1).Export()

        go func() {
            e.mu.RLock()
            handlers := e.handlers[event]
            e.mu.RUnlock()

            for _, h := range handlers {
                h(context.Background(), data)
            }
        }()

        return goja.Undefined()
    })
}

// LoadGame 加载游戏代码
func (e *JSEngine) LoadGame(ctx context.Context, code []byte) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 编译并执行代码
    _, err := e.vm.RunString(string(code))
    if err != nil {
        return fmt.Errorf("failed to load game code: %w", err)
    }

    return nil
}

// Start 启动游戏
func (e *JSEngine) Start(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if e.started {
        return errors.New("engine already started")
    }

    // 调用游戏的 onStart 函数
    if err := e.callFunction("onStart"); err != nil {
        // onStart 不是必需的
        if !errors.Is(err, ErrFunctionNotFound) {
            return err
        }
    }

    e.started = true
    return nil
}

// Stop 停止游戏
func (e *JSEngine) Stop(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if !e.started {
        return nil
    }

    // 调用游戏的 onStop 函数
    _ = e.callFunction("onStop")

    e.started = false
    return nil
}

// Call 调用游戏函数
func (e *JSEngine) Call(ctx context.Context, method string, args ...interface{}) (interface{}, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()

    // 获取函数
    fn, ok := goja.AssertFunction(e.vm.Get(method))
    if !ok {
        return nil, fmt.Errorf("function not found: %s", method)
    }

    // 转换参数
    jsArgs := make([]goja.Value, len(args))
    for i, arg := range args {
        jsArgs[i] = e.vm.ToValue(arg)
    }

    // 设置超时
    if e.conf.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, time.Duration(e.conf.Timeout)*time.Millisecond)
        defer cancel()
    }

    // 调用函数
    result, err := fn(goja.Undefined(), jsArgs...)
    if err != nil {
        return nil, fmt.Errorf("function call failed: %w", err)
    }

    return result.Export(), nil
}

// GetState 获取游戏状态
func (e *JSEngine) GetState(ctx context.Context) (interface{}, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()

    // 调用 getState 函数
    state, err := e.Call(ctx, "getState")
    if err != nil {
        return nil, err
    }

    return state, nil
}

// SetState 设置游戏状态
func (e *JSEngine) SetState(ctx context.Context, state interface{}) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 设置状态
    e.vm.Set("gameState", state)

    return nil
}

// EmitEvent 触发事件
func (e *JSEngine) EmitEvent(ctx context.Context, event string, data interface{}) error {
    e.mu.RLock()
    handlers := e.handlers[event]
    e.mu.RUnlock()

    for _, h := range handlers {
        if err := h(ctx, data); err != nil {
            return err
        }
    }

    return nil
}

// OnEvent 监听事件
func (e *JSEngine) OnEvent(ctx context.Context, event string, handler EventHandler) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    e.handlers[event] = append(e.handlers[event], handler)
    return nil
}

// MemoryStats 返回内存统计
func (e *JSEngine) MemoryStats(ctx context.Context) (*MemoryStats, error) {
    // goja 没有直接的内存统计API
    return &MemoryStats{
        Used:     0,
        Allocated: 0,
        Limit:    e.conf.MaxMemoryBytes,
    }, nil
}

// GCOpts 返回GC选项
func (e *JSEngine) GCOpts() []GCOpt {
    return []GCOpt{}
}

// Close 关闭引擎
func (e *JSEngine) Close() error {
    e.mu.Lock()
    defer e.mu.Unlock()

    e.vm = nil
    e.handlers = make(map[string][]EventHandler)
    return nil
}

// callFunction 调用函数
func (e *JSEngine) callFunction(name string, args ...interface{}) error {
    fn, ok := goja.AssertFunction(e.vm.Get(name))
    if !ok {
        return ErrFunctionNotFound
    }

    jsArgs := make([]goja.Value, len(args))
    for i, arg := range args {
        jsArgs[i] = e.vm.ToValue(arg)
    }

    _, err := fn(goja.Undefined(), jsArgs...)
    return err
}

var (
    ErrFunctionNotFound = errors.New("function not found")
)
```

### 2.3 WASM 引擎实现

```go
// internal/engine/wasm_engine.go
package engine

import (
    "context"
    "errors"
    "fmt"
    "io"
    "sync"
    "time"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMEngine WASM引擎
type WASMEngine struct {
    runtime  wazero.Runtime
    module   wazero.CompiledModule
    instance wazero.ModuleInstance
    conf     InitConfig
    mu       sync.RWMutex
    started  bool
    memory   []byte
    logger   Logger
}

// NewWASMEngine 创建WASM引擎
func NewWASMEngine(opts ...InitOption) *WASMEngine {
    conf := InitConfig{
        MaxMemoryBytes:  50 * 1024 * 1024,
        Timeout:         5000,
        EnableProfiling: false,
    }

    for _, opt := range opts {
        opt(&conf)
    }

    return &WASMEngine{
        conf:   conf,
        logger: conf.Logger,
    }
}

// Name 实现Engine接口
func (e *WASMEngine) Name() string {
    return "wasm"
}

// Type 实现Engine接口
func (e *WASMEngine) Type() EngineType {
    return EngineTypeWASM
}

// Init 初始化引擎
func (e *WASMEngine) Init(ctx context.Context, opts ...InitOption) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 应用选项
    for _, opt := range opts {
        opt(&e.conf)
    }

    // 创建运行时
    e.runtime = wazero.NewRuntime(ctx)

    // 注入WASI
    if _, err := wasi_snapshot_preview1.Instantiate(ctx, e.runtime); err != nil {
        return fmt.Errorf("failed to instantiate WASI: %w", err)
    }

    return nil
}

// LoadGame 加载游戏代码
func (e *WASMEngine) LoadGame(ctx context.Context, code []byte) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 编译WASM模块
    module, err := e.runtime.CompileModule(ctx, code)
    if err != nil {
        return fmt.Errorf("failed to compile WASM: %w", err)
    }

    e.module = module
    return nil
}

// Start 启动游戏
func (e *WASMEngine) Start(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if e.started {
        return errors.New("engine already started")
    }

    // 实例化模块
    instance, err := e.runtime.InstantiateModule(ctx, e.module, wazero.NewModuleConfig())
    if err != nil {
        return fmt.Errorf("failed to instantiate module: %w", err)
    }

    e.instance = instance

    // 调用 onStart 函数
    onStart := instance.ExportedFunction("onStart")
    if onStart != nil {
        if _, err := onStart(ctx); err != nil {
            return fmt.Errorf("onStart failed: %w", err)
        }
    }

    e.started = true
    return nil
}

// Stop 停止游戏
func (e *WASMEngine) Stop(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if !e.started {
        return nil
    }

    // 调用 onStop 函数
    if e.instance != nil {
        onStop := e.instance.ExportedFunction("onStop")
        if onStop != nil {
            _, _ = onStop(ctx)
        }
    }

    // 关闭模块
    if e.module != nil {
        _ = e.runtime.CloseModule(ctx, e.module)
    }

    e.started = false
    return nil
}

// Call 调用游戏函数
func (e *WASMEngine) Call(ctx context.Context, method string, args ...interface{}) (interface{}, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()

    if e.instance == nil {
        return nil, errors.New("engine not started")
    }

    // 获取函数
    fn := e.instance.ExportedFunction(method)
    if fn == nil {
        return nil, fmt.Errorf("function not found: %s", method)
    }

    // 调用函数
    results, err := fn(ctx)
    if err != nil {
        return nil, fmt.Errorf("function call failed: %w", err)
    }

    // 返回结果
    if len(results) > 0 {
        return results[0], nil
    }

    return nil, nil
}

// GetState 获取游戏状态
func (e *WASMEngine) GetState(ctx context.Context) (interface{}, error) {
    return e.Call(ctx, "getState")
}

// SetState 设置游戏状态
func (e *WASMEngine) SetState(ctx context.Context, state interface{}) error {
    // WASM中需要通过内存操作设置状态
    return nil
}

// EmitEvent 触发事件
func (e *WASMEngine) EmitEvent(ctx context.Context, event string, data interface{}) error {
    if e.instance == nil {
        return errors.New("engine not started")
    }

    // 调用 WASM 的事件处理函数
    fn := e.instance.ExportedFunction("onEvent")
    if fn == nil {
        return nil
    }

    // 将事件名和数据写入内存，然后调用函数
    // 这里需要具体的内存操作实现

    return nil
}

// OnEvent 监听事件
func (e *WASMEngine) OnEvent(ctx context.Context, event string, handler EventHandler) error {
    // WASM引擎通常不需要从Go侧监听事件
    return nil
}

// MemoryStats 返回内存统计
func (e *WASMEngine) MemoryStats(ctx context.Context) (*MemoryStats, error) {
    if e.instance == nil {
        return &MemoryStats{
            Limit: e.conf.MaxMemoryBytes,
        }, nil
    }

    // 获取内存信息
    mem := e.instance.Memory()
    if mem == nil {
        return &MemoryStats{
            Limit: e.conf.MaxMemoryBytes,
        }, nil
    }

    size, ok := mem.Size(ctx)
    if !ok {
        return &MemoryStats{
            Limit: e.conf.MaxMemoryBytes,
        }, nil
    }

    return &MemoryStats{
        Used:  int64(size),
        Limit: e.conf.MaxMemoryBytes,
    }, nil
}

// GCOpts 返回GC选项
func (e *WASMEngine) GCOpts() []GCOpt {
    return []GCOpt{
        {Key: "memory_growth", Value: int64(1024 * 1024)},
    }
}

// Close 关闭引擎
func (e *WASMEngine) Close() error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if e.runtime != nil {
        ctx := context.Background()
        _ = e.runtime.Close(ctx)
    }

    return nil
}
```

### 2.4 双引擎实现

```go
// internal/engine/dual_engine.go
package engine

import (
    "context"
    "errors"
    "sync"
)

// EngineSelector 引擎选择器
type EngineSelector interface {
    // Select 选择引擎
    Select(code []byte) (EngineType, error)
}

// DualEngine 双引擎实现
type DualEngine struct {
    primary   Engine
    secondary Engine
    selector  EngineSelector
    conf      InitConfig
    mu        sync.RWMutex
    active    Engine
}

// NewDualEngine 创建双引擎
func NewDualEngine(primary, secondary Engine, selector EngineSelector, opts ...InitOption) *DualEngine {
    conf := InitConfig{}
    for _, opt := range opts {
        opt(&conf)
    }

    return &DualEngine{
        primary:   primary,
        secondary: secondary,
        selector:  selector,
        conf:      conf,
    }
}

// Name 实现Engine接口
func (e *DualEngine) Name() string {
    return "dual"
}

// Type 实现Engine接口
func (e *DualEngine) Type() EngineType {
    return EngineTypeJavaScript // 默认类型
}

// Init 初始化引擎
func (e *DualEngine) Init(ctx context.Context, opts ...InitOption) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 应用选项
    for _, opt := range opts {
        opt(&e.conf)
    }

    // 初始化两个引擎
    if err := e.primary.Init(ctx, opts...); err != nil {
        return err
    }

    if err := e.secondary.Init(ctx, opts...); err != nil {
        return err
    }

    return nil
}

// LoadGame 加载游戏代码
func (e *DualEngine) LoadGame(ctx context.Context, code []byte) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 选择引擎
    engineType, err := e.selector.Select(code)
    if err != nil {
        return err
    }

    // 根据类型选择引擎
    switch engineType {
    case EngineTypeJavaScript:
        e.active = e.primary
    case EngineTypeWASM:
        e.active = e.secondary
    default:
        return errors.New("unsupported engine type")
    }

    // 加载代码
    return e.active.LoadGame(ctx, code)
}

// Start 启动游戏
func (e *DualEngine) Start(ctx context.Context) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return errors.New("no active engine")
    }

    return active.Start(ctx)
}

// Stop 停止游戏
func (e *DualEngine) Stop(ctx context.Context) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil
    }

    return active.Stop(ctx)
}

// Call 调用游戏函数
func (e *DualEngine) Call(ctx context.Context, method string, args ...interface{}) (interface{}, error) {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil, errors.New("no active engine")
    }

    return active.Call(ctx, method, args...)
}

// GetState 获取游戏状态
func (e *DualEngine) GetState(ctx context.Context) (interface{}, error) {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil, errors.New("no active engine")
    }

    return active.GetState(ctx)
}

// SetState 设置游戏状态
func (e *DualEngine) SetState(ctx context.Context, state interface{}) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return errors.New("no active engine")
    }

    return active.SetState(ctx, state)
}

// EmitEvent 触发事件
func (e *DualEngine) EmitEvent(ctx context.Context, event string, data interface{}) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return errors.New("no active engine")
    }

    return active.EmitEvent(ctx, event, data)
}

// OnEvent 监听事件
func (e *DualEngine) OnEvent(ctx context.Context, event string, handler EventHandler) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return errors.New("no active engine")
    }

    return active.OnEvent(ctx, event, handler)
}

// MemoryStats 返回内存统计
func (e *DualEngine) MemoryStats(ctx context.Context) (*MemoryStats, error) {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil, errors.New("no active engine")
    }

    return active.MemoryStats(ctx)
}

// GCOpts 返回GC选项
func (e *DualEngine) GCOpts() []GCOpt {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil
    }

    return active.GCOpts()
}

// Close 关闭引擎
func (e *DualEngine) Close() error {
    e.mu.Lock()
    defer e.mu.Unlock()

    var errs []error

    if e.primary != nil {
        if err := e.primary.Close(); err != nil {
            errs = append(errs, err)
        }
    }

    if e.secondary != nil {
        if err := e.secondary.Close(); err != nil {
            errs = append(errs, err)
        }
    }

    if len(errs) > 0 {
        return errs[0]
    }

    return nil
}
```

### 2.5 引擎选择器实现

```go
// internal/engine/selector.go
package engine

import (
    "bytes"
    "errors"
)

// DefaultSelector 默认引擎选择器
type DefaultSelector struct{}

// NewDefaultSelector 创建默认选择器
func NewDefaultSelector() *DefaultSelector {
    return &DefaultSelector{}
}

// Select 选择引擎
func (s *DefaultSelector) Select(code []byte) (EngineType, error) {
    // 检查WASM魔数
    if isWASM(code) {
        return EngineTypeWASM, nil
    }

    // 默认使用JavaScript
    return EngineTypeJavaScript, nil
}

// isWASM 检查是否是WASM代码
func isWASM(code []byte) bool {
    // WASM魔数: 0x00 0x61 0x73 0x6D
    return bytes.HasPrefix(code, []byte{0x00, 0x61, 0x73, 0x6D})
}

// SizeSelector 基于代码大小的选择器
type SizeSelector struct {
    wasmThreshold int // 代码大小阈值，超过则使用WASM
}

// NewSizeSelector 创建基于大小的选择器
func NewSizeSelector(threshold int) *SizeSelector {
    return &SizeSelector{
        wasmThreshold: threshold,
    }
}

// Select 选择引擎
func (s *SizeSelector) Select(code []byte) (EngineType, error) {
    if isWASM(code) {
        return EngineTypeWASM, nil
    }

    // 如果代码很大，建议使用WASM
    if len(code) > s.wasmThreshold {
        return EngineTypeWASM, errors.New("code too large for JS engine, please compile to WASM")
    }

    return EngineTypeJavaScript, nil
}

// PerformanceSelector 性能选择器
type PerformanceSelector struct {
    profile map[EngineType]float64 // 各引擎的性能评分
}

// NewPerformanceSelector 创建性能选择器
func NewPerformanceSelector() *PerformanceSelector {
    return &PerformanceSelector{
        profile: make(map[EngineType]float64),
    }
}

// Record 记录性能数据
func (s *PerformanceSelector) Record(engineType EngineType, score float64) {
    s.profile[engineType] = score
}

// Select 选择引擎
func (s *PerformanceSelector) Select(code []byte) (EngineType, error) {
    if isWASM(code) {
        return EngineTypeWASM, nil
    }

    // 如果没有性能数据，使用默认
    if len(s.profile) == 0 {
        return EngineTypeJavaScript, nil
    }

    // 选择性能最好的引擎
    var best EngineType
    var bestScore float64
    for engineType, score := range s.profile {
        if score > bestScore {
            best = engineType
            bestScore = score
        }
    }

    return best, nil
}
```

---

## 3. WebSocket 实现

### 3.1 连接管理

```go
// internal/ws/connection.go
package ws

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "sync/atomic"
    "time"

    "github.com/gorilla/websocket"
)

// Conn WebSocket连接
type Conn struct {
    id       string
    ws       *websocket.Conn
    mu       sync.Mutex
    send     chan []byte
    hub      *Hub
    userID   string
    metadata map[string]interface{}
    once     sync.Once
    closed   int32
    // 统计
    messagesSent     int64
    messagesReceived int64
    bytesSent        int64
    bytesReceived    int64
    lastActive       time.Time
}

// NewConn 创建连接
func NewConn(id string, ws *websocket.Conn, hub *Hub) *Conn {
    return &Conn{
        id:       id,
        ws:       ws,
        send:     make(chan []byte, 256),
        hub:      hub,
        metadata: make(map[string]interface{}),
        lastActive: time.Now(),
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
    c.mu.Lock()
    defer c.mu.Unlock()
    c.userID = userID
}

// Metadata 获取元数据
func (c *Conn) Metadata() map[string]interface{} {
    c.mu.Lock()
    defer c.mu.Unlock()

    // 返回副本
    m := make(map[string]interface{}, len(c.metadata))
    for k, v := range c.metadata {
        m[k] = v
    }
    return m
}

// SetMetadata 设置元数据
func (c *Conn) SetMetadata(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.metadata[key] = value
}

// GetMetadata 获取元数据
func (c *Conn) GetMetadata(key string) (interface{}, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    v, ok := c.metadata[key]
    return v, ok
}

// ReadPump 读取循环
func (c *Conn) ReadPump(ctx context.Context) {
    defer func() {
        c.hub.Unregister(c)
        c.ws.Close()
    }()

    c.ws.SetReadLimit(maxMessageSize)
    c.ws.SetReadDeadline(time.Now().Add(pongWait))
    c.ws.SetPongHandler(func(string) error {
        c.ws.SetReadDeadline(time.Now().Add(pongWait))
        atomic.StoreInt64(&c.lastActive.Unix, time.Now().Unix())
        return nil
    })

    for {
        _, message, err := c.ws.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                // 记录错误
            }
            break
        }

        atomic.AddInt64(&c.messagesReceived, 1)
        atomic.AddInt64(&c.bytesReceived, int64(len(message)))
        atomic.StoreInt64(&c.lastActive.Unix, time.Now().Unix())

        // 处理消息
        c.handleMessage(ctx, message)
    }
}

// WritePump 写入循环
func (c *Conn) WritePump(ctx context.Context) {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.ws.Close()
    }()

    for {
        select {
        case message, ok := <-c.send:
            c.ws.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                c.ws.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            w, err := c.ws.NextWriter(websocket.TextMessage)
            if err != nil {
                return
            }
            w.Write(message)

            // 添加队列中的消息
            n := len(c.send)
            for i := 0; i < n; i++ {
                w.Write([]byte{'\n'})
                w.Write(<-c.send)
            }

            if err := w.Close(); err != nil {
                return
            }

            atomic.AddInt64(&c.messagesSent, 1)
            atomic.AddInt64(&c.bytesSent, int64(len(message)))

        case <-ticker.C:
            c.ws.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }

        case <-ctx.Done():
            return
        }
    }
}

// handleMessage 处理消息
func (c *Conn) handleMessage(ctx context.Context, data []byte) {
    var msg Message
    if err := json.Unmarshal(data, &msg); err != nil {
        c.SendError("invalid_message", "Failed to parse message")
        return
    }

    // 设置消息元数据
    msg.ConnID = c.id
    msg.UserID = c.userID

    // 路由到处理器
    c.hub.RouteMessage(ctx, &msg, c)
}

// Send 发送消息
func (c *Conn) Send(data []byte) error {
    if atomic.LoadInt32(&c.closed) == 1 {
        return fmt.Errorf("connection closed")
    }

    select {
    case c.send <- data:
        return nil
    default:
        return fmt.Errorf("send buffer full")
    }
}

// SendJSON 发送JSON消息
func (c *Conn) SendJSON(v interface{}) error {
    data, err := json.Marshal(v)
    if err != nil {
        return err
    }
    return c.Send(data)
}

// SendError 发送错误消息
func (c *Conn) SendError(code, message string) error {
    return c.SendJSON(map[string]interface{}{
        "type":    "error",
        "code":    code,
        "message": message,
    })
}

// Close 关闭连接
func (c *Conn) Close() error {
    if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
        return nil // 已经关闭
    }

    close(c.send)
    return c.ws.Close()
}

// Stats 返回统计信息
func (c *Conn) Stats() ConnStats {
    return ConnStats{
        MessagesSent:     atomic.LoadInt64(&c.messagesSent),
        MessagesReceived: atomic.LoadInt64(&c.messagesReceived),
        BytesSent:        atomic.LoadInt64(&c.bytesSent),
        BytesReceived:    atomic.LoadInt64(&c.bytesReceived),
        LastActive:       time.Unix(atomic.LoadInt64(&c.lastActive.Unix), 0),
    }
}

// ConnStats 连接统计
type ConnStats struct {
    MessagesSent     int64
    MessagesReceived int64
    BytesSent        int64
    BytesReceived    int64
    LastActive       time.Time
}

const (
    // Time allowed to write a message to the peer.
    writeWait = 10 * time.Second

    // Time allowed to read the next pong message from the peer.
    pongWait = 60 * time.Second

    // Send pings to peer with this period. Must be less than pongWait.
    pingPeriod = (pongWait * 9) / 10

    // Maximum message size allowed from peer.
    maxMessageSize = 512 * 1024
)
```

### 3.2 Hub 实现

```go
// internal/ws/hub.go
package ws

import (
    "context"
    "encoding/json"
    "sync"
    "time"

    "github.com/google/uuid"
)

// Hub WebSocket连接中心
type Hub struct {
    // 连接注册
    connections map[string]*Conn
    connsByUser map[string][]*Conn // 用户ID到连接的映射

    // 消息通道
    broadcast  chan []byte
    register   chan *Conn
    unregister chan *Conn

    // 房间
    rooms map[string]*Room

    // 消息处理器
    handlers map[string]MessageHandler

    mu sync.RWMutex
}

// MessageHandler 消息处理器
type MessageHandler func(ctx context.Context, msg *Message, conn *Conn) error

// NewHub 创建Hub
func NewHub() *Hub {
    return &Hub{
        connections: make(map[string]*Conn),
        connsByUser: make(map[string][]*Conn),
        broadcast:   make(chan []byte, 256),
        register:    make(chan *Conn),
        unregister:  make(chan *Conn),
        rooms:       make(map[string]*Room),
        handlers:    make(map[string]MessageHandler),
    }
}

// Run 运行Hub
func (h *Hub) Run(ctx context.Context) {
    for {
        select {
        case conn := <-h.register:
            h.registerConn(conn)

        case conn := <-h.unregister:
            h.unregisterConn(conn)

        case message := <-h.broadcast:
            h.broadcastMessage(message)

        case <-ctx.Done():
            return
        }
    }
}

// Register 注册连接
func (h *Hub) Register(conn *Conn) {
    h.register <- conn
}

// registerConn 内部注册逻辑
func (h *Hub) registerConn(conn *Conn) {
    h.mu.Lock()
    defer h.mu.Unlock()

    h.connections[conn.id] = conn

    // 添加到用户映射
    if conn.userID != "" {
        h.connsByUser[conn.userID] = append(h.connsByUser[conn.userID], conn)
    }
}

// Unregister 注销连接
func (h *Hub) Unregister(conn *Conn) {
    h.unregister <- conn
}

// unregisterConn 内部注销逻辑
func (h *Hub) unregisterConn(conn *Conn) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if _, ok := h.connections[conn.id]; !ok {
        return
    }

    delete(h.connections, conn.id)

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

    // 从所有房间移除
    for _, room := range h.rooms {
        room.Leave(conn.id)
    }
}

// Broadcast 广播消息
func (h *Hub) Broadcast(data []byte) {
    h.broadcast <- data
}

// broadcastMessage 内部广播逻辑
func (h *Hub) broadcastMessage(data []byte) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    for _, conn := range h.connections {
        conn.Send(data)
    }
}

// SendToUser 发送消息给指定用户
func (h *Hub) SendToUser(userID string, data []byte) error {
    h.mu.RLock()
    defer h.mu.RUnlock()

    conns := h.connsByUser[userID]
    if len(conns) == 0 {
        return ErrUserNotFound
    }

    for _, conn := range conns {
        conn.Send(data)
    }

    return nil
}

// SendToConn 发送消息给指定连接
func (h *Hub) SendToConn(connID string, data []byte) error {
    h.mu.RLock()
    defer h.mu.RUnlock()

    conn, ok := h.connections[connID]
    if !ok {
        return ErrConnNotFound
    }

    return conn.Send(data)
}

// RegisterHandler 注册消息处理器
func (h *Hub) RegisterHandler(msgType string, handler MessageHandler) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.handlers[msgType] = handler
}

// RouteMessage 路由消息
func (h *Hub) RouteMessage(ctx context.Context, msg *Message, conn *Conn) {
    h.mu.RLock()
    handler, ok := h.handlers[msg.Type]
    h.mu.RUnlock()

    if !ok {
        conn.SendError("unknown_message_type", "Unknown message type: "+msg.Type)
        return
    }

    if err := handler(ctx, msg, conn); err != nil {
        conn.SendError("handler_error", err.Error())
    }
}

// GetOrCreateRoom 获取或创建房间
func (h *Hub) GetOrCreateRoom(roomID string) *Room {
    h.mu.Lock()
    defer h.mu.Unlock()

    if room, ok := h.rooms[roomID]; ok {
        return room
    }

    room := NewRoom(roomID, h)
    h.rooms[roomID] = room
    return room
}

// Stats 返回统计信息
func (h *Hub) Stats() HubStats {
    h.mu.RLock()
    defer h.mu.RUnlock()

    totalConns := len(h.connections)
    totalUsers := len(h.connsByUser)

    roomCount := len(h.rooms)
    roomStats := make([]RoomStats, 0, len(h.rooms))
    for _, room := range h.rooms {
        roomStats = append(roomStats, room.Stats())
    }

    return HubStats{
        TotalConnections: totalConns,
        TotalUsers:       totalUsers,
        RoomCount:        roomCount,
        RoomStats:        roomStats,
    }
}

var (
    ErrUserNotFound = fmt.Errorf("user not found")
    ErrConnNotFound = fmt.Errorf("connection not found")
)

// HubStats Hub统计
type HubStats struct {
    TotalConnections int
    TotalUsers       int
    RoomCount        int
    RoomStats        []RoomStats
}
```

### 3.3 Room 实现

```go
// internal/ws/room.go
package ws

import (
    "context"
    "sync"
)

// Room 房间
type Room struct {
    id     string
    hub    *Hub
    conns  map[string]*Conn
    data   map[string]interface{}
    mu     sync.RWMutex
    closed bool
}

// NewRoom 创建房间
func NewRoom(id string, hub *Hub) *Room {
    return &Room{
        id:    id,
        hub:   hub,
        conns: make(map[string]*Conn),
        data:  make(map[string]interface{}),
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

    r.conns[conn.id] = conn

    // 通知房间成员
    r.broadcastLocked(map[string]interface{}{
        "type":     "room_joined",
        "room_id":  r.id,
        "conn_id":  conn.id,
        "user_id":  conn.UserID(),
    })

    return nil
}

// Leave 离开房间
func (r *Room) Leave(connID string) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, ok := r.conns[connID]; !ok {
        return
    }

    delete(r.conns, connID)

    // 通知房间成员
    r.broadcastLocked(map[string]interface{}{
        "type":    "room_left",
        "room_id": r.id,
        "conn_id": connID,
    })
}

// Broadcast 广播消息到房间
func (r *Room) Broadcast(data []byte) error {
    r.mu.RLock()
    defer r.mu.RUnlock()

    if r.closed {
        return ErrRoomClosed
    }

    for _, conn := range r.conns {
        conn.Send(data)
    }

    return nil
}

// broadcastLocked 广播（已加锁）
func (r *Room) broadcastLocked(msg interface{}) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    for _, conn := range r.conns {
        conn.Send(data)
    }

    return nil
}

// SendTo 发送消息给指定连接
func (r *Room) SendTo(connID string, data []byte) error {
    r.mu.RLock()
    defer r.mu.RUnlock()

    conn, ok := r.conns[connID]
    if !ok {
        return ErrConnNotFound
    }

    return conn.Send(data)
}

// Members 返回房间成员
func (r *Room) Members() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()

    members := make([]string, 0, len(r.conns))
    for _, conn := range r.conns {
        members = append(members, conn.id)
    }

    return members
}

// MemberCount 返回成员数量
func (r *Room) MemberCount() int {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return len(r.conns)
}

// Set 设置房间数据
func (r *Room) Set(key string, value interface{}) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.data[key] = value
}

// Get 获取房间数据
func (r *Room) Get(key string) (interface{}, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
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

// Stats 返回统计信息
func (r *Room) Stats() RoomStats {
    r.mu.RLock()
    defer r.mu.RUnlock()

    return RoomStats{
        RoomID:      r.id,
        MemberCount: len(r.conns),
        Closed:      r.closed,
    }
}

var (
    ErrRoomClosed = fmt.Errorf("room closed")
)

// RoomStats 房间统计
type RoomStats struct {
    RoomID      string
    MemberCount int
    Closed      bool
}
```

---

## 4. 中间件实现

### 4.1 认证中间件

```go
// internal/middleware/auth.go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
    jwtSecret   string
    jwtExpires  int
    keyFunc     jwt.Keyfunc
}

type contextKey string

const (
    UserIDKey   contextKey = "user_id"
    TokenKey    contextKey = "token"
)

type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    jwt.RegisteredClaims
}

func NewAuthMiddleware(secret string) *AuthMiddleware {
    return &AuthMiddleware{
        jwtSecret:  secret,
        jwtExpires: 24 * 3600, // 24小时
        keyFunc: func(token *jwt.Token) (interface{}, error) {
            return []byte(secret), nil
        },
    }
}

func (m *AuthMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
    // 从Header获取token
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        http.Error(rw, "Missing authorization header", http.StatusUnauthorized)
        return
    }

    // Bearer token格式
    parts := strings.SplitN(authHeader, " ", 2)
    if len(parts) != 2 || parts[0] != "Bearer" {
        http.Error(rw, "Invalid authorization format", http.StatusUnauthorized)
        return
    }

    tokenString := parts[1]

    // 解析token
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, m.keyFunc)
    if err != nil || !token.Valid {
        http.Error(rw, "Invalid token", http.StatusUnauthorized)
        return
    }

    claims, ok := token.Claims.(*Claims)
    if !ok {
        http.Error(rw, "Invalid token claims", http.StatusUnauthorized)
        return
    }

    // 将用户信息存入context
    ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
    ctx = context.WithValue(ctx, TokenKey, tokenString)

    next(rw, r.WithContext(ctx))
}

// OptionalAuth 可选认证中间件
func (m *AuthMiddleware) OptionalAuth() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                next.ServeHTTP(rw, r)
                return
            }

            parts := strings.SplitN(authHeader, " ", 2)
            if len(parts) != 2 || parts[0] != "Bearer" {
                next.ServeHTTP(rw, r)
                return
            }

            token, err := jwt.ParseWithClaims(parts[1], &Claims{}, m.keyFunc)
            if err != nil {
                next.ServeHTTP(rw, r)
                return
            }

            if claims, ok := token.Claims.(*Claims); ok && token.Valid {
                ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
                r = r.WithContext(ctx)
            }

            next.ServeHTTP(rw, r)
        })
    }
}

// GenerateToken 生成token
func (m *AuthMiddleware) GenerateToken(userID, username string) (string, error) {
    claims := Claims{
        UserID:   userID,
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            // ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(m.jwtExpires) * time.Second)),
            // IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(m.jwtSecret))
}

// GetUserID 从context获取用户ID
func GetUserID(r *http.Request) (string, bool) {
    userID, ok := r.Context().Value(UserIDKey).(string)
    return userID, ok
}
```

### 4.2 恢复中间件

```go
// internal/middleware/recovery.go
package middleware

import (
    "fmt"
    "net/http"
    "runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // 记录panic
                fmt.Printf("PANIC: %v\n%s\n", err, debug.Stack())

                // 返回500错误
                http.Error(rw, "Internal server error", http.StatusInternalServerError)
            }
        }()

        next.ServeHTTP(rw, r)
    })
}
```

### 4.3 请求ID中间件

```go
// internal/middleware/request_id.go
package middleware

import (
    "context"
    "net/http"

    "github.com/google/uuid"
)

type contextKey string

const (
    RequestIDKey contextKey = "request_id"
)

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
        // 从header获取或生成新的request ID
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
        }

        // 存入context
        ctx := context.WithValue(r.Context(), RequestIDKey, requestID)

        // 设置响应header
        rw.Header().Set("X-Request-ID", requestID)

        next.ServeHTTP(rw, r.WithContext(ctx))
    })
}

// GetRequestID 获取请求ID
func GetRequestID(r *http.Request) string {
    if requestID, ok := r.Context().Value(RequestIDKey).(string); ok {
        return requestID
    }
    return ""
}
```

### 4.4 日志中间件

```go
// internal/middleware/logging.go
package middleware

import (
    "log"
    "net/http"
    "time"
)

type responseWriter struct {
    http.ResponseWriter
    status      int
    wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
    if rw.wroteHeader {
        return
    }
    rw.status = code
    rw.wroteHeader = true
    rw.ResponseWriter.WriteHeader(code)
}

func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // 包装ResponseWriter
        wrappedRW := &responseWriter{ResponseWriter: rw, status: http.StatusOK}

        // 调用下一个处理器
        next.ServeHTTP(wrappedRW, r)

        // 记录日志
        duration := time.Since(start)
        requestID := GetRequestID(r)
        userID, _ := GetUserID(r)

        log.Printf("[%s] %s %s %s %d %v user=%s",
            requestID,
            r.Method,
            r.URL.Path,
            r.RemoteAddr,
            wrappedRW.status,
            duration,
            userID,
        )
    })
}
```

### 4.5 CORS中间件

```go
// internal/middleware/cors.go
package middleware

import (
    "net/http"
)

type CORSConfig struct {
    AllowedOrigins   []string
    AllowedMethods   []string
    AllowedHeaders   []string
    AllowCredentials bool
    MaxAge           int
}

func CORS(config CORSConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")

            // 检查允许的源
            allowed := false
            for _, allowedOrigin := range config.AllowedOrigins {
                if allowedOrigin == "*" || allowedOrigin == origin {
                    allowed = true
                    break
                }
            }

            if allowed {
                rw.Header().Set("Access-Control-Allow-Origin", origin)
            }

            // 设置其他CORS头
            if len(config.AllowedMethods) > 0 {
                rw.Header().Set("Access-Control-Allow-Methods", join(config.AllowedMethods, ", "))
            }

            if len(config.AllowedHeaders) > 0 {
                rw.Header().Set("Access-Control-Allow-Headers", join(config.AllowedHeaders, ", "))
            }

            if config.AllowCredentials {
                rw.Header().Set("Access-Control-Allow-Credentials", "true")
            }

            if config.MaxAge > 0 {
                rw.Header().Set("Access-Control-Max-Age", string(rune(config.MaxAge)))
            }

            // 处理预检请求
            if r.Method == http.MethodOptions {
                rw.WriteHeader(http.StatusOK)
                return
            }

            next.ServeHTTP(rw, r)
        })
    }
}

func join(items []string, sep string) string {
    if len(items) == 0 {
        return ""
    }
    result := items[0]
    for _, item := range items[1:] {
        result += sep + item
    }
    return result
}
```

---

## 5. 数据访问层实现

### 5.1 Repository 接口

```go
// internal/user/repository/repository.go
package repository

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// User 用户实体
type User struct {
    ID        string    `db:"id"`
    Username  string    `db:"username"`
    Email     string    `db:"email"`
    Phone     string    `db:"phone"`
    Password  string    `db:"password"`
    Nickname  string    `db:"nickname"`
    Avatar    string    `db:"avatar"`
    Status    int       `db:"status"`
    CreatedAt time.Time `db:"created_at"`
    UpdatedAt time.Time `db:"updated_at"`
}

// Repository 用户仓储接口
type Repository interface {
    // Create 创建用户
    Create(ctx context.Context, user *User) error

    // FindByID 根据ID查找用户
    FindByID(ctx context.Context, id string) (*User, error)

    // FindByUsername 根据用户名查找
    FindByUsername(ctx context.Context, username string) (*User, error)

    // FindByEmail 根据邮箱查找
    FindByEmail(ctx context.Context, email string) (*User, error)

    // Update 更新用户
    Update(ctx context.Context, user *User) error

    // Delete 删除用户
    Delete(ctx context.Context, id string) error

    // List 列出用户
    List(ctx context.Context, opts ...ListOption) ([]*User, int64, error)

    // Transaction 事务执行
    Transaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// ListOption 列表选项
type ListOption func(*ListOptions)

// ListOptions 列表查询选项
type ListOptions struct {
    Limit      int
    Offset     int
    OrderBy    string
    OrderDesc  bool
    Status     *int
    Keyword    string
}

// WithLimit 设置限制
func WithLimit(limit int) ListOption {
    return func(o *ListOptions) {
        o.Limit = limit
    }
}

// WithOffset 设置偏移
func WithOffset(offset int) ListOption {
    return func(o *ListOptions) {
        o.Offset = offset
    }
}

// WithOrderBy 设置排序
func WithOrderBy(field string, desc bool) ListOption {
    return func(o *ListOptions) {
        o.OrderBy = field
        o.OrderDesc = desc
    }
}
```

### 5.2 PostgreSQL 实现

```go
// internal/user/repository/postgres.go
package repository

import (
    "context"
    "errors"
    "fmt"

    "github.com/jackc/pgerr"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
    "github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
    db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
    return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, user *User) error {
    query := `
        INSERT INTO users (id, username, email, phone, password, nickname, avatar, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING created_at, updated_at
    `

    err := r.db.QueryRow(ctx, query,
        user.ID,
        user.Username,
        user.Email,
        user.Phone,
        user.Password,
        user.Nickname,
        user.Avatar,
        user.Status,
    ).Scan(&user.CreatedAt, &user.UpdatedAt)

    if err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) {
            switch pgErr.Code {
            case "23505": // 唯一约束冲突
                return ErrDuplicateKey
            }
        }
        return fmt.Errorf("failed to create user: %w", err)
    }

    return nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*User, error) {
    query := `
        SELECT id, username, email, phone, password, nickname, avatar, status, created_at, updated_at
        FROM users
        WHERE id = $1
    `

    user := &User{}
    err := r.db.QueryRow(ctx, query, id).Scan(
        &user.ID,
        &user.Username,
        &user.Email,
        &user.Phone,
        &user.Password,
        &user.Nickname,
        &user.Avatar,
        &user.Status,
        &user.CreatedAt,
        &user.UpdatedAt,
    )

    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrUserNotFound
        }
        return nil, fmt.Errorf("failed to find user: %w", err)
    }

    return user, nil
}

func (r *PostgresRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
    query := `
        SELECT id, username, email, phone, password, nickname, avatar, status, created_at, updated_at
        FROM users
        WHERE username = $1
    `

    user := &User{}
    err := r.db.QueryRow(ctx, query, username).Scan(
        &user.ID,
        &user.Username,
        &user.Email,
        &user.Phone,
        &user.Password,
        &user.Nickname,
        &user.Avatar,
        &user.Status,
        &user.CreatedAt,
        &user.UpdatedAt,
    )

    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrUserNotFound
        }
        return nil, fmt.Errorf("failed to find user: %w", err)
    }

    return user, nil
}

func (r *PostgresRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
    query := `
        SELECT id, username, email, phone, password, nickname, avatar, status, created_at, updated_at
        FROM users
        WHERE email = $1
    `

    user := &User{}
    err := r.db.QueryRow(ctx, query, email).Scan(
        &user.ID,
        &user.Username,
        &user.Email,
        &user.Phone,
        &user.Password,
        &user.Nickname,
        &user.Avatar,
        &user.Status,
        &user.CreatedAt,
        &user.UpdatedAt,
    )

    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrUserNotFound
        }
        return nil, fmt.Errorf("failed to find user: %w", err)
    }

    return user, nil
}

func (r *PostgresRepository) Update(ctx context.Context, user *User) error {
    query := `
        UPDATE users
        SET username = $2, email = $3, phone = $4, password = $5,
            nickname = $6, avatar = $7, status = $8, updated_at = NOW()
        WHERE id = $1
        RETURNING updated_at
    `

    err := r.db.QueryRow(ctx, query,
        user.ID,
        user.Username,
        user.Email,
        user.Phone,
        user.Password,
        user.Nickname,
        user.Avatar,
        user.Status,
    ).Scan(&user.UpdatedAt)

    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return ErrUserNotFound
        }
        return fmt.Errorf("failed to update user: %w", err)
    }

    return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
    query := `DELETE FROM users WHERE id = $1`

    result, err := r.db.Exec(ctx, query, id)
    if err != nil {
        return fmt.Errorf("failed to delete user: %w", err)
    }

    if result.RowsAffected() == 0 {
        return ErrUserNotFound
    }

    return nil
}

func (r *PostgresRepository) List(ctx context.Context, opts ...ListOption) ([]*User, int64, error) {
    options := &ListOptions{
        Limit:     20,
        Offset:    0,
        OrderBy:   "created_at",
        OrderDesc: true,
    }

    for _, opt := range opts {
        opt(options)
    }

    // 构建查询
    baseQuery := `FROM users WHERE 1=1`
    countQuery := `SELECT COUNT(*) ` + baseQuery
    selectQuery := `SELECT id, username, email, phone, password, nickname, avatar, status, created_at, updated_at ` + baseQuery

    // 添加条件
    args := []interface{}{}
    argIdx := 1

    if options.Status != nil {
        condition := fmt.Sprintf(" AND status = $%d", argIdx)
        selectQuery += condition
        countQuery += condition
        args = append(args, *options.Status)
        argIdx++
    }

    if options.Keyword != "" {
        condition := fmt.Sprintf(" AND (username LIKE $%d OR nickname LIKE $%d OR email LIKE $%d)", argIdx, argIdx, argIdx)
        selectQuery += condition
        countQuery += condition
        args = append(args, "%"+options.Keyword+"%")
        argIdx++
    }

    // 获取总数
    var total int64
    err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to count users: %w", err)
    }

    // 添加排序和分页
    order := "ASC"
    if options.OrderDesc {
        order = "DESC"
    }
    selectQuery += fmt.Sprintf(" ORDER BY %s %s", options.OrderBy, order)
    selectQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
    args = append(args, options.Limit, options.Offset)

    // 查询数据
    rows, err := r.db.Query(ctx, selectQuery, args...)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to list users: %w", err)
    }
    defer rows.Close()

    users := []*User{}
    for rows.Next() {
        user := &User{}
        err := rows.Scan(
            &user.ID,
            &user.Username,
            &user.Email,
            &user.Phone,
            &user.Password,
            &user.Nickname,
            &user.Avatar,
            &user.Status,
            &user.CreatedAt,
            &user.UpdatedAt,
        )
        if err != nil {
            return nil, 0, fmt.Errorf("failed to scan user: %w", err)
        }
        users = append(users, user)
    }

    return users, total, nil
}

func (r *PostgresRepository) Transaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
    return pgx.BeginFunc(ctx, r.db, fn)
}

var (
    ErrUserNotFound  = fmt.Errorf("user not found")
    ErrDuplicateKey  = fmt.Errorf("duplicate key")
)
```

---

继续文档下一部分...