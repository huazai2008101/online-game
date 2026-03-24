# Actor 模型详细实现

**文档版本:** v1.0
**创建时间:** 2026-03-24

---

## 目录

1. [设计概述](#1-设计概述)
2. [核心接口](#2-核心接口)
3. [Actor 实现](#3-actor-实现)
4. [Mailbox 实现](#4-mailbox-实现)
5. [Supervisor 实现](#5-supervisor-实现)
6. [ActorSystem 实现](#6-actorsystem-实现)
7. [消息类型](#7-消息类型)
8. [使用示例](#8-使用示例)

---

## 1. 设计概述

### 1.1 设计目标

Actor 模型是一种并发计算模型，具有以下特点：

1. **隔离性**: 每个 Actor 独立运行，不共享内存
2. **异步通信**: 通过消息传递进行通信
3. **位置透明**: Actor 可以在本地或远程
4. **监督策略**: 父 Actor 可以监督子 Actor 的失败

### 1.2 核心概念

```
┌─────────────────────────────────────────────────────────────┐
│                      ActorSystem                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Actor A  │  │ Actor B  │  │ Actor C  │  │ Actor D  │   │
│  │          │  │          │  │          │  │          │   │
│  │ ┌──────┐ │  │ ┌──────┐ │  │ ┌──────┐ │  │ ┌──────┐ │   │
│  │ │Mailbox│ │  │ │Mailbox│ │  │ │Mailbox│ │  │ │Mailbox│ │   │
│  │ └──────┘ │  │ └──────┘ │  │ └──────┘ │  │ └──────┘ │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │
│       │             │             │             │           │
│       └─────────────┴─────────────┴─────────────┘           │
│                       │                                     │
│                   Dispatcher                                │
└───────────────────────┼─────────────────────────────────────┘
                        │
                   Message Bus
```

---

## 2. 核心接口

### 2.1 Message 接口

```go
// internal/actor/message.go
package actor

import (
    "encoding/json"
    "time"
)

// Message 消息接口
type Message interface {
    // Type 返回消息类型
    Type() string

    // Timestamp 返回消息时间戳
    Timestamp() time.Time

    // ID 返回消息ID
    ID() string
}

// BaseMessage 基础消息
type BaseMessage struct {
    MsgType   string    `json:"type"`
    MsgID     string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Sender    string    `json:"sender,omitempty"`
    Receiver  string    `json:"receiver,omitempty"`
}

func (m *BaseMessage) Type() string      { return m.MsgType }
func (m *BaseMessage) ID() string        { return m.MsgID }
func (m *BaseMessage) Timestamp() time.Time { return m.Timestamp }

// NewBaseMessage 创建基础消息
func NewBaseMessage(msgType string) *BaseMessage {
    return &BaseMessage{
        MsgType:   msgType,
        MsgID:     generateMessageID(),
        Timestamp: time.Now(),
    }
}

// StringMessage 字符串消息
type StringMessage struct {
    BaseMessage
    Data interface{} `json:"data"`
}

// BytesMessage 字节消息
type BytesMessage struct {
    BaseMessage
    Data []byte `json:"data"`
}

// TypedMessage 泛型消息
type TypedMessage[T any] struct {
    BaseMessage
    Data T `json:"data"`
}

// NewTypedMessage 创建泛型消息
func NewTypedMessage[T any](msgType string, data T) *TypedMessage[T] {
    return &TypedMessage[T]{
        BaseMessage: BaseMessage{
            MsgType:   msgType,
            MsgID:     generateMessageID(),
            Timestamp: time.Now(),
        },
        Data: data,
    }
}

func generateMessageID() string {
    return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}
```

### 2.2 Actor 接口

```go
// internal/actor/actor.go
package actor

import (
    "context"
    "time"
)

// Lifecycle 生命周期接口
type Lifecycle interface {
    // PreStart 启动前回调
    PreStart(ctx context.Context) error

    // PostStop 停止后回调
    PostStop(ctx context.Context) error
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
    Stop(ctx context.Context) error

    // Send 发送消息（异步）
    Send(ctx context.Context, msg Message) error

    // Tell 异步发送消息（非阻塞）
    Tell(msg Message)

    // Ask 同步请求响应
    Ask(ctx context.Context, msg Message) (Message, error)

    // Restart 重启Actor
    Restart(ctx context.Context) error

    // Children 返回子Actor
    Children() []*ActorRef

    // Parent 返回父Actor
    Parent() *ActorRef

    // Path 返回Actor路径
    Path() string
}

// ActorOption Actor配置选项
type ActorOption func(*ActorConfig)

// ActorConfig Actor配置
type ActorConfig struct {
    MailboxSize  int
    SupervisedBy Supervisor
    MaxRestarts  int
    RestartDelay time.Duration
    Dispatcher   Dispatcher
    Props        map[string]interface{}
}

// WithMailboxSize 设置邮箱大小
func WithMailboxSize(size int) ActorOption {
    return func(c *ActorConfig) {
        c.MailboxSize = size
    }
}

// WithSupervisor 设置监督者
func WithSupervisor(supervisor Supervisor) ActorOption {
    return func(c *ActorConfig) {
        c.SupervisedBy = supervisor
    }
}

// WithMaxRestarts 设置最大重启次数
func WithMaxRestarts(n int) ActorOption {
    return func(c *ActorConfig) {
        c.MaxRestarts = n
    }
}

// WithRestartDelay 设置重启延迟
func WithRestartDelay(delay time.Duration) ActorOption {
    return func(c *ActorConfig) {
        c.RestartDelay = delay
    }
}

// WithDispatcher 设置调度器
func WithDispatcher(d Dispatcher) ActorOption {
    return func(c *ActorConfig) {
        c.Dispatcher = d
    }
}

// WithProps 设置属性
func WithProps(key string, value interface{}) ActorOption {
    return func(c *ActorConfig) {
        if c.Props == nil {
            c.Props = make(map[string]interface{})
        }
        c.Props[key] = value
    }
}
```

---

## 3. Actor 实现

### 3.1 BaseActor 实现

```go
// internal/actor/base_actor.go
package actor

import (
    "context"
    "fmt"
    "sync"
    "sync/atomic"
    "time"
)

// BaseActor 基础Actor实现
type BaseActor struct {
    // 基础信息
    id         string
    actorType  string
    path       string

    // 组件
    mailbox    Mailbox
    handler    Handler
    supervisor Supervisor
    dispatcher Dispatcher

    // 状态
    state      atomic.Int32 // 0: created, 1: starting, 2: started, 3: stopping, 4: stopped
    restarts   atomic.Int32

    // 上下文
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup

    // 关系
    parent     *ActorRef
    children   map[string]*ActorRef
    childrenMu sync.RWMutex

    // 配置
    maxRestarts int
    restartDelay time.Duration

    // 属性
    props      map[string]interface{}
    propsMu    sync.RWMutex

    // 创建时间
    createdAt  time.Time
    startedAt  time.Time
}

// NewBaseActor 创建基础Actor
func NewBaseActor(id, actorType string, handler Handler, opts ...ActorOption) *BaseActor {
    ctx, cancel := context.WithCancel(context.Background())

    config := &ActorConfig{
        MailboxSize:  1000,
        MaxRestarts:  10,
        RestartDelay: time.Second,
        Dispatcher:   NewDefaultDispatcher(),
    }

    for _, opt := range opts {
        opt(config)
    }

    return &BaseActor{
        id:         id,
        actorType:  actorType,
        handler:    handler,
        mailbox:    NewDefaultMailbox(config.MailboxSize),
        supervisor: config.SupervisedBy,
        dispatcher: config.Dispatcher,
        ctx:        ctx,
        cancel:     cancel,
        children:   make(map[string]*ActorRef),
        maxRestarts: config.MaxRestarts,
        restartDelay: config.RestartDelay,
        props:      config.Props,
        createdAt:  time.Now(),
    }
}

// ID 返回Actor ID
func (a *BaseActor) ID() string {
    return a.id
}

// Type 返回Actor类型
func (a *BaseActor) Type() string {
    return a.actorType
}

// Start 启动Actor
func (a *BaseActor) Start(ctx context.Context) error {
    if !a.state.CompareAndSwap(0, 1) {
        return fmt.Errorf("actor already started or starting: %s", a.id)
    }

    a.ctx, a.cancel = context.WithCancel(ctx)
    a.startedAt = time.Now()

    // 调用PreStart钩子
    if lc, ok := a.handler.(Lifecycle); ok {
        if err := lc.PreStart(a.ctx); err != nil {
            a.state.Store(0)
            return fmt.Errorf("PreStart failed: %w", err)
        }
    }

    a.state.Store(2)

    // 启动消息处理循环
    a.wg.Add(1)
    go a.processLoop()

    return nil
}

// Stop 停止Actor
func (a *BaseActor) Stop(ctx context.Context) error {
    if !a.state.CompareAndSwap(2, 3) {
        return nil // 已经停止或正在停止
    }

    // 停止所有子Actor
    a.stopAllChildren(ctx)

    // 取消上下文
    a.cancel()

    // 等待处理循环结束
    done := make(chan struct{})
    go func() {
        a.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        a.state.Store(4)

        // 调用PostStop钩子
        if lc, ok := a.handler.(Lifecycle); ok {
            _ = lc.PostStop(context.Background())
        }

        // 关闭邮箱
        _ = a.mailbox.Close()

        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Send 发送消息
func (a *BaseActor) Send(ctx context.Context, msg Message) error {
    if a.state.Load() != 2 {
        return fmt.Errorf("actor not running: %s", a.id)
    }

    return a.mailbox.Push(ctx, msg)
}

// Tell 异步发送消息
func (a *BaseActor) Tell(msg Message) {
    _ = a.Send(context.Background(), msg)
}

// Ask 同步请求响应
func (a *BaseActor) Ask(ctx context.Context, msg Message) (Message, error) {
    responseCh := make(chan Message, 1)
    errorCh := make(chan error, 1)

    wrapper := &AskMessage{
        Message:      msg,
        ResponseChan: responseCh,
        ErrorChan:    errorCh,
    }

    if err := a.Send(ctx, wrapper); err != nil {
        return nil, err
    }

    select {
    case resp := <-responseCh:
        return resp, nil
    case err := <-errorCh:
        return nil, err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

// Restart 重启Actor
func (a *BaseActor) Restart(ctx context.Context) error {
    restarts := a.restars.Add(1)

    if restarts > int32(a.maxRestarts) {
        return fmt.Errorf("max restarts exceeded for actor: %s", a.id)
    }

    // 停止
    stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    if err := a.Stop(stopCtx); err != nil {
        return fmt.Errorf("failed to stop actor: %w", err)
    }

    // 等待重启延迟
    if a.restartDelay > 0 {
        select {
        case <-time.After(a.restartDelay):
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    // 重启
    if err := a.Start(ctx); err != nil {
        return fmt.Errorf("failed to start actor: %w", err)
    }

    return nil
}

// Children 返回子Actor
func (a *BaseActor) Children() []*ActorRef {
    a.childrenMu.RLock()
    defer a.childrenMu.RUnlock()

    children := make([]*ActorRef, 0, len(a.children))
    for _, child := range a.children {
        children = append(children, child)
    }
    return children
}

// Parent 返回父Actor
func (a *BaseActor) Parent() *ActorRef {
    return a.parent
}

// Path 返回Actor路径
func (a *BaseActor) Path() string {
    if a.path != "" {
        return a.path
    }

    if a.parent != nil {
        a.path = a.parent.Path() + "/" + a.id
    } else {
        a.path = "/" + a.id
    }
    return a.path
}

// SpawnChild 创建子Actor
func (a *BaseActor) SpawnChild(id, actorType string, handler Handler, opts ...ActorOption) (*ActorRef, error) {
    a.childrenMu.Lock()
    defer a.childrenMu.Unlock()

    if _, exists := a.children[id]; exists {
        return nil, fmt.Errorf("child actor already exists: %s", id)
    }

    // 添加当前Actor作为监督者
    opts = append(opts, WithSupervisor(a))

    child := NewBaseActor(id, actorType, handler, opts...)
    child.parent = &ActorRef{id: a.id, sys: nil} // 简化引用

    if err := child.Start(a.ctx); err != nil {
        return nil, err
    }

    ref := &ActorRef{
        id:   id,
        path: a.Path() + "/" + id,
    }

    a.children[id] = ref
    return ref, nil
}

// StopChild 停止子Actor
func (a *BaseActor) StopChild(id string) error {
    a.childrenMu.Lock()
    defer a.childrenMu.Unlock()

    ref, exists := a.children[id]
    if !exists {
        return fmt.Errorf("child actor not found: %s", id)
    }

    // TODO: 实际停止子Actor
    delete(a.children, id)
    _ = ref

    return nil
}

// HandleFailure 处理子Actor失败
func (a *BaseActor) HandleFailure(child Actor, err error) {
    if a.supervisor != nil {
        a.supervisor.HandleFailure(child, err)
    }
}

// GetProp 获取属性
func (a *BaseActor) GetProp(key string) (interface{}, bool) {
    a.propsMu.RLock()
    defer a.propsMu.RUnlock()
    v, ok := a.props[key]
    return v, ok
}

// SetProp 设置属性
func (a *BaseActor) SetProp(key string, value interface{}) {
    a.propsMu.Lock()
    defer a.propsMu.Unlock()
    if a.props == nil {
        a.props = make(map[string]interface{})
    }
    a.props[key] = value
}

// Stats 返回统计信息
func (a *BaseActor) Stats() ActorStats {
    return ActorStats{
        ID:          a.id,
        Type:        a.actorType,
        Path:        a.Path(),
        State:       a.getStateName(),
        Restarts:    int(a.restars.Load()),
        Children:    len(a.Children()),
        CreatedAt:   a.createdAt,
        StartedAt:   a.startedAt,
        MailboxSize: a.mailbox.Size(),
    }
}

func (a *BaseActor) getStateName() string {
    switch a.state.Load() {
    case 0:
        return "created"
    case 1:
        return "starting"
    case 2:
        return "started"
    case 3:
        return "stopping"
    case 4:
        return "stopped"
    default:
        return "unknown"
    }
}

func (a *BaseActor) stopAllChildren(ctx context.Context) {
    a.childrenMu.Lock()
    children := make([]*ActorRef, 0, len(a.children))
    for _, child := range a.children {
        children = append(children, child)
    }
    a.childrenMu.Unlock()

    for _, child := range children {
        // TODO: 停止子Actor
        _ = child
    }
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
                a.handleFailure(msg, err)
            }
        }
    }
}

// handleMessage 处理单个消息
func (a *BaseActor) handleMessage(ctx context.Context, msg Message) error {
    defer func() {
        if r := recover(); r != nil {
            err := fmt.Errorf("panic in actor %s: %v", a.id, r)
            a.handleFailure(msg, err)
        }
    }()

    // 处理AskMessage
    if askMsg, ok := msg.(*AskMessage); ok {
        response, err := a.handler(ctx, askMsg.Message)
        if err != nil {
            askMsg.RespondError(err)
        } else {
            askMsg.Respond(response)
        }
        return nil
    }

    return a.handler(ctx, msg)
}

func (a *BaseActor) handleFailure(msg Message, err error) {
    // 通知监督者
    if a.supervisor != nil {
        a.supervisor.HandleFailure(a, err)
    }

    // 记录日志
    // TODO: 添加日志
}

// ActorStats Actor统计信息
type ActorStats struct {
    ID          string
    Type        string
    Path        string
    State       string
    Restarts    int
    Children    int
    CreatedAt   time.Time
    StartedAt   time.Time
    MailboxSize int
}
```

### 3.2 Handler 类型

```go
// internal/actor/handler.go
package actor

import (
    "context"
    "fmt"
)

// Handler 消息处理函数
type Handler func(ctx context.Context, msg Message) error

// HandlerFunc 实现Handler的函数类型
type HandlerFunc func(ctx context.Context, msg Message) error

// ServeHTTP 实现Handler接口
func (f HandlerFunc) Handle(ctx context.Context, msg Message) error {
    return f(ctx, msg)
}

// MessageHandler 消息处理器映射
type MessageHandler struct {
    handlers map[string]Handler
    fallback Handler
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler() *MessageHandler {
    return &MessageHandler{
        handlers: make(map[string]Handler),
    }
}

// Register 注册消息处理器
func (h *MessageHandler) Register(msgType string, handler Handler) {
    h.handlers[msgType] = handler
}

// RegisterFunc 注册处理函数
func (h *MessageHandler) RegisterFunc(msgType string, handler func(ctx context.Context, data interface{}) error) {
    h.handlers[msgType] = func(ctx context.Context, msg Message) error {
        if typed, ok := msg.(*TypedMessage[interface{}]); ok {
            return handler(ctx, typed.Data)
        }
        if str, ok := msg.(*StringMessage); ok {
            return handler(ctx, str.Data)
        }
        return handler(ctx, nil)
    }
}

// SetFallback 设置默认处理器
func (h *MessageHandler) SetFallback(handler Handler) {
    h.fallback = handler
}

// Handle 处理消息
func (h *MessageHandler) Handle(ctx context.Context, msg Message) error {
    handler, ok := h.handlers[msg.Type()]
    if !ok {
        if h.fallback != nil {
            return h.fallback(ctx, msg)
        }
        return fmt.Errorf("no handler for message type: %s", msg.Type())
    }
    return handler(ctx, msg)
}

// Receive 消息接收接口
type Receive interface {
    // Receive 接收消息
    Receive(ctx context.Context, msg Message) error
}

// ReceiveReceiver 将Receive接口转换为Handler
func ReceiveReceiver(r Receive) Handler {
    return func(ctx context.Context, msg Message) error {
        return r.Receive(ctx, msg)
    }
}
```

---

## 4. Mailbox 实现

### 4.1 默认邮箱实现

```go
// internal/actor/mailbox.go
package actor

import (
    "context"
    "fmt"
    "sync"
    "time"
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

    // TryPop 尝试弹出（非阻塞）
    TryPop() (Message, error)

    // Size 返回当前消息数量
    Size() int

    // Capacity 返回容量
    Capacity() int

    // Close 关闭邮箱
    Close() error
}

// DefaultMailbox 默认邮箱（基于channel）
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

// TryPop 尝试弹出
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

// Size 返回大小
func (m *DefaultMailbox) Size() int {
    return len(m.ch)
}

// Capacity 返回容量
func (m *DefaultMailbox) Capacity() int {
    return m.capacity
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
```

### 4.2 优先级邮箱实现

```go
// internal/actor/priority_mailbox.go
package actor

import (
    "context"
    "sync"
    "time"
)

// Priority 优先级
type Priority int

const (
    PriorityLow      Priority = 0
    PriorityNormal   Priority = 1
    PriorityHigh     Priority = 2
    PriorityCritical Priority = 3
)

// PriorityMessage 优先级消息接口
type PriorityMessage interface {
    Message
    Priority() Priority
}

// PriorityMailbox 优先级邮箱
type PriorityMailbox struct {
    capacity int
    items    []*priorityItem
    mu       sync.Mutex
    cond     *sync.Cond
    closed   bool
    sequence uint64
}

type priorityItem struct {
    msg      Message
    priority Priority
    sequence uint64
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
    priority := PriorityNormal
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
        sequence: m.sequence,
    }
    m.sequence++

    m.insert(item)
    m.cond.Signal()
    return nil
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

    item := m.pop()
    m.mu.Unlock()
    return item.msg, nil
}

// TryPop 尝试弹出
func (m *PriorityMailbox) TryPop() (Message, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if len(m.items) == 0 {
        return nil, nil
    }

    item := m.pop()
    return item.msg, nil
}

// Size 返回大小
func (m *PriorityMailbox) Size() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return len(m.items)
}

// Capacity 返回容量
func (m *PriorityMailbox) Capacity() int {
    return m.capacity
}

// Close 关闭邮箱
func (m *PriorityMailbox) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.closed {
        return nil
    }

    m.closed = true
    m.cond.Broadcast()
    return nil
}

// insert 插入元素（维护堆）
func (m *PriorityMailbox) insert(item *priorityItem) {
    m.items = append(m.items, item)
    m.heapifyUp(len(m.items) - 1)
}

// pop 弹出元素
func (m *PriorityMailbox) pop() *priorityItem {
    if len(m.items) == 0 {
        return nil
    }

    item := m.items[0]
    last := len(m.items) - 1
    m.items[0] = m.items[last]
    m.items = m.items[:last]

    if len(m.items) > 0 {
        m.heapifyDown(0)
    }

    return item
}

// heapifyUp 向上堆化
func (m *PriorityMailbox) heapifyUp(i int) {
    for i > 0 {
        parent := (i - 1) / 2
        if m.compare(parent, i) >= 0 {
            break
        }
        m.items[i], m.items[parent] = m.items[parent], m.items[i]
        i = parent
    }
}

// heapifyDown 向下堆化
func (m *PriorityMailbox) heapifyDown(i int) {
    n := len(m.items)
    for {
        left := 2*i + 1
        right := 2*i + 2
        largest := i

        if left < n && m.compare(largest, left) < 0 {
            largest = left
        }
        if right < n && m.compare(largest, right) < 0 {
            largest = right
        }

        if largest == i {
            break
        }

        m.items[i], m.items[largest] = m.items[largest], m.items[i]
        i = largest
    }
}

// compare 比较两个元素
func (m *PriorityMailbox) compare(i, j int) int {
    a, b := m.items[i], m.items[j]

    // 先按优先级比较
    if a.priority != b.priority {
        return int(a.priority) - int(b.priority)
    }

    // 优先级相同按序列号比较（FIFO）
    if int64(a.sequence) < int64(b.sequence) {
        return -1
    } else if int64(a.sequence) > int64(b.sequence) {
        return 1
    }
    return 0
}
```

### 4.3 可靠邮箱实现

```go
// internal/actor/reliable_mailbox.go
package actor

import (
    "context"
    "sync"
    "time"
)

// ReliableMailbox 可靠邮箱（支持持久化）
type ReliableMailbox struct {
    capacity int
    memory   []*Message
    disk     []*Message // 简化实现，实际应该用持久化存储
    mu       sync.RWMutex
    closed   bool

    // 统计
    memoryCount int
    diskCount   int
}

// NewReliableMailbox 创建可靠邮箱
func NewReliableMailbox(capacity int) *ReliableMailbox {
    return &ReliableMailbox{
        capacity: capacity,
        memory:   make([]*Message, 0, capacity),
        disk:     make([]*Message, 0),
    }
}

// Push 推送消息
func (m *ReliableMailbox) Push(ctx context.Context, msg Message) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.closed {
        return ErrMailboxClosed
    }

    // 内存未满，存入内存
    if len(m.memory) < m.capacity {
        m.memory = append(m.memory, &msg)
        m.memoryCount++
        return nil
    }

    // 内存已满，溢出到磁盘
    m.disk = append(m.disk, &msg)
    m.diskCount++
    return nil
}

// Pop 弹出消息
func (m *ReliableMailbox) Pop(ctx context.Context) (Message, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // 先从内存取
    if len(m.memory) > 0 {
        msg := m.memory[0]
        m.memory = m.memory[1:]
        m.memoryCount--
        return *msg, nil
    }

    // 内存空了，从磁盘加载
    if len(m.disk) > 0 {
        // 批量加载
        loadSize := min(m.capacity, len(m.disk))
        for i := 0; i < loadSize; i++ {
            m.memory = append(m.memory, m.disk[i])
        }
        m.disk = m.disk[loadSize:]
        m.diskCount -= loadSize

        // 返回第一个
        msg := m.memory[0]
        m.memory = m.memory[1:]
        m.memoryCount--
        return *msg, nil
    }

    // 等待新消息
    return nil, nil // 简化实现
}

// TryPop 尝试弹出
func (m *ReliableMailbox) TryPop() (Message, error) {
    return m.Pop(context.Background())
}

// Size 返回大小
func (m *ReliableMailbox) Size() int {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.memoryCount + m.diskCount
}

// Capacity 返回容量
func (m *ReliableMailbox) Capacity() int {
    return m.capacity
}

// Close 关闭邮箱
func (m *ReliableMailbox) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.closed {
        return nil
    }

    m.closed = true

    // 持久化剩余消息
    // TODO: 实际实现中应该持久化到磁盘

    return nil
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

---

## 5. Supervisor 实现

### 5.1 监督策略

```go
// internal/actor/supervisor.go
package actor

import (
    "context"
    "log"
    "time"
)

// Strategy 监督策略
type Strategy int

const (
    // OneForOne 只重启失败的Actor
    OneForOne Strategy = iota
    // OneForAll 重启所有子Actor
    OneForAll
    // AllForOne 停止所有子Actor
    AllForOne
)

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

// Supervisor 监督器接口
type Supervisor interface {
    // HandleFailure 处理失败
    HandleFailure(child Actor, err error)

    // Strategy 返回策略
    Strategy() Strategy

    // Children 返回子Actor
    Children() []*ActorRef
}

// Decider 决策函数
type Decider func(error) Directive

// BaseSupervisor 基础监督器
type BaseSupervisor struct {
    strategy    Strategy
    children    []*ActorRef
    decider     Decider
    maxRestarts int
    restarts    map[string]int
    mu          sync.Mutex
}

// NewBaseSupervisor 创建基础监督器
func NewBaseSupervisor(strategy Strategy, decider Decider) *BaseSupervisor {
    return &BaseSupervisor{
        strategy:    strategy,
        children:    make([]*ActorRef, 0),
        decider:     decider,
        maxRestarts: 10,
        restarts:    make(map[string]int),
    }
}

// HandleFailure 处理失败
func (s *BaseSupervisor) HandleFailure(child Actor, err error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // 使用决策函数决定如何处理
    directive := s.decider(err)

    childID := child.ID()

    switch directive {
    case Restart:
        s.handleRestart(child, childID)
    case Stop:
        s.handleStop(child, childID)
    case Escalate:
        s.handleEscalate(child, err)
    case Resume:
        // 忽略，不做处理
    }
}

func (s *BaseSupervisor) handleRestart(child Actor, childID string) {
    // 检查重启次数
    if s.restarts[childID] >= s.maxRestarts {
        log.Printf("Max restarts exceeded for child: %s", childID)
        s.handleStop(child, childID)
        return
    }

    s.restarts[childID]++

    ctx := context.Background()
    if err := child.Restart(ctx); err != nil {
        log.Printf("Failed to restart child %s: %v", childID, err)
    }
}

func (s *BaseSupervisor) handleStop(child Actor, childID string) {
    ctx := context.Background()
    if err := child.Stop(ctx); err != nil {
        log.Printf("Failed to stop child %s: %v", childID, err)
    }
    delete(s.restarts, childID)
}

func (s *BaseSupervisor) handleEscalate(child Actor, err error) {
    log.Printf("Escalating failure from child %s: %v", child.ID(), err)
    // 通知上级监督器
}

// Strategy 返回策略
func (s *BaseSupervisor) Strategy() Strategy {
    return s.strategy
}

// Children 返回子Actor
func (s *BaseSupervisor) Children() []*ActorRef {
    s.mu.Lock()
    defer s.mu.Unlock()

    children := make([]*ActorRef, len(s.children))
    copy(children, s.children)
    return children
}

// AddChild 添加子Actor
func (s *BaseSupervisor) AddChild(ref *ActorRef) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.children = append(s.children, ref)
}

// RemoveChild 移除子Actor
func (s *BaseSupervisor) RemoveChild(ref *ActorRef) {
    s.mu.Lock()
    defer s.mu.Unlock()

    for i, child := range s.children {
        if child.ID() == ref.ID() {
            s.children = append(s.children[:i], s.children[i+1:]...)
            return
        }
    }
}

// DefaultDecider 默认决策函数
func DefaultDecider(err error) Directive {
    switch {
    case isContextCanceled(err):
        return Stop
    case isPanic(err):
        return Restart
    default:
        return Restart
    }
}

func isContextCanceled(err error) bool {
    // 检查是否是context取消错误
    return false
}

func isPanic(err error) bool {
    // 检查是否是panic
    return false
}
```

### 5.2 常用监督器实现

```go
// internal/actor/supervisors.go
package actor

import (
    "context"
    "errors"
    "log"
    "time"
)

// OneForOneSupervisor OneForOne监督器
type OneForOneSupervisor struct {
    *BaseSupervisor
}

// NewOneForOneSupervisor 创建OneForOne监督器
func NewOneForOneSupervisor(decider Decider) *OneForOneSupervisor {
    return &OneForOneSupervisor{
        BaseSupervisor: NewBaseSupervisor(OneForOne, decider),
    }
}

// OneForAllSupervisor OneForAll监督器
type OneForAllSupervisor struct {
    *BaseSupervisor
}

// NewOneForAllSupervisor 创建OneForAll监督器
func NewOneForAllSupervisor(decider Decider) *OneForAllSupervisor {
    return &OneForAllSupervisor{
        BaseSupervisor: NewBaseSupervisor(OneForAll, decider),
    }
}

// HandleFailure 处理失败
func (s *OneForAllSupervisor) HandleFailure(failedActor Actor, err error) {
    directive := s.BaseSupervisor.decoder(err)

    s.mu.Lock()
    defer s.mu.Unlock()

    if directive == Restart {
        // 重启所有子Actor
        for _, childRef := range s.children {
            // TODO: 获取实际Actor并重启
            log.Printf("Restarting child: %s", childRef.ID())
        }
    } else {
        s.BaseSupervisor.HandleFailure(failedActor, err)
    }
}

// AllForOneSupervisor AllForOne监督器
type AllForOneSupervisor struct {
    *BaseSupervisor
}

// NewAllForOneSupervisor 创建AllForOne监督器
func NewAllForOneSupervisor(decider Decider) *AllForOneSupervisor {
    return &AllForOneSupervisor{
        BaseSupervisor: NewBaseSupervisor(AllForOne, decider),
    }
}

// HandleFailure 处理失败
func (s *AllForOneSupervisor) HandleFailure(failedActor Actor, err error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // 停止所有子Actor
    for _, childRef := range s.children {
        // TODO: 停止实际Actor
        log.Printf("Stopping child: %s", childRef.ID())
    }
}

// ExponentialBackoffSupervisor 指数退避监督器
type ExponentialBackoffSupervisor struct {
    *BaseSupervisor
    baseDelay    time.Duration
    maxDelay     time.Duration
    backoffFactor float64
}

// NewExponentialBackoffSupervisor 创建指数退避监督器
func NewExponentialBackoffSupervisor(
    baseDelay time.Duration,
    maxDelay time.Duration,
    backoffFactor float64,
    decider Decider,
) *ExponentialBackoffSupervisor {
    return &ExponentialBackoffSupervisor{
        BaseSupervisor:   NewBaseSupervisor(OneForOne, decider),
        baseDelay:       baseDelay,
        maxDelay:        maxDelay,
        backoffFactor:   backoffFactor,
    }
}

// HandleFailure 处理失败（带退避）
func (s *ExponentialBackoffSupervisor) HandleFailure(child Actor, err error) {
    childID := child.ID()

    s.mu.Lock()
    restartCount := s.restarts[childID]
    s.mu.Unlock()

    // 计算退避延迟
    delay := s.baseDelay
    for i := 0; i < restartCount; i++ {
        delay = time.Duration(float64(delay) * s.backoffFactor)
        if delay > s.maxDelay {
            delay = s.maxDelay
            break
        }
    }

    // 等待延迟后重启
    time.Sleep(delay)
    s.BaseSupervisor.HandleFailure(child, err)
}

// StoppingSupervisor 停止监督器（失败即停止）
type StoppingSupervisor struct {
    *BaseSupervisor
}

// NewStoppingSupervisor 创建停止监督器
func NewStoppingSupervisor() *StoppingSupervisor {
    decider := func(err error) Directive {
        return Stop
    }
    return &StoppingSupervisor{
        BaseSupervisor: NewBaseSupervisor(OneForOne, decider),
    }
}
```

---

## 6. ActorSystem 实现

```go
// internal/actor/system.go
package actor

import (
    "context"
    "fmt"
    "sync"
    "sync/atomic"
    "time"
)

// ActorSystem Actor系统
type ActorSystem struct {
    name       string
    actors     map[string]Actor
    actorsMu   sync.RWMutex

    // 上下文
    ctx        context.Context
    cancel     context.CancelFunc

    // 组件
    dispatcher Dispatcher
    router     *Router

    // 配置
    config     SystemConfig

    // 状态
    started    atomic.Bool
    stopped    atomic.Bool

    // 统计
    stats      SystemStats
}

// SystemConfig 系统配置
type SystemConfig struct {
    Name              string
    DefaultMailboxSize int
    DefaultDispatcher string
}

// SystemStats 系统统计
type SystemStats struct {
    TotalActors   int64
    ActiveActors  int64
    MessagesSent  int64
    MessagesReceived int64
    StartTime     time.Time
}

// NewActorSystem 创建Actor系统
func NewActorSystem(name string, opts ...SystemOption) *ActorSystem {
    ctx, cancel := context.WithCancel(context.Background())

    config := SystemConfig{
        Name:               name,
        DefaultMailboxSize: 1000,
        DefaultDispatcher:  "default",
    }

    for _, opt := range opts {
        opt(&config)
    }

    sys := &ActorSystem{
        name:     config.Name,
        actors:   make(map[string]Actor),
        ctx:      ctx,
        cancel:   cancel,
        config:   config,
        stats:    SystemStats{StartTime: time.Now()},
    }

    // 初始化调度器
    sys.dispatcher = NewDefaultDispatcher()

    // 初始化路由器
    sys.router = NewRouter(sys)

    return sys
}

// SystemOption 系统配置选项
type SystemOption func(*SystemConfig)

// WithDefaultMailboxSize 设置默认邮箱大小
func WithDefaultMailboxSize(size int) SystemOption {
    return func(c *SystemConfig) {
        c.DefaultMailboxSize = size
    }
}

// Start 启动系统
func (s *ActorSystem) Start(ctx context.Context) error {
    if !s.started.CompareAndSwap(false, true) {
        return fmt.Errorf("system already started")
    }

    s.ctx, s.cancel = context.WithCancel(ctx)

    return nil
}

// Stop 停止系统
func (s *ActorSystem) Stop(ctx context.Context) error {
    if !s.started.CompareAndSwap(true, false) {
        return nil
    }

    if !s.stopped.CompareAndSwap(false, true) {
        return nil
    }

    s.cancel()

    // 停止所有Actor
    s.actorsMu.Lock()
    actors := make([]Actor, 0, len(s.actors))
    for _, actor := range s.actors {
        actors = append(actors, actor)
    }
    s.actorsMu.Unlock()

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
    s.actorsMu.Lock()
    defer s.actorsMu.Unlock()

    if _, exists := s.actors[id]; exists {
        return nil, fmt.Errorf("actor already exists: %s", id)
    }

    // 添加默认选项
    opts = append(opts, WithMailboxSize(s.config.DefaultMailboxSize))

    actor := NewBaseActor(id, actorType, handler, opts...)

    if err := actor.Start(s.ctx); err != nil {
        return nil, err
    }

    s.actors[id] = actor
    atomic.AddInt64(&s.stats.TotalActors, 1)
    atomic.AddInt64(&s.stats.ActiveActors, 1)

    ref := &ActorRef{
        id:   id,
        path: fmt.Sprintf("/%s/%s", s.name, id),
        sys:  s,
    }

    return ref, nil
}

// SpawnActorOf 创建ActorRef并启动
func (s *ActorSystem) SpawnActorOf(props Props, handler Handler) (*ActorRef, error) {
    id := props.ID
    if id == "" {
        id = generateActorID(props.Type)
    }

    opts := []ActorOption{}
    if props.MailboxSize > 0 {
        opts = append(opts, WithMailboxSize(props.MailboxSize))
    }
    if props.Supervisor != nil {
        opts = append(opts, WithSupervisor(props.Supervisor))
    }
    if props.Dispatcher != nil {
        opts = append(opts, WithDispatcher(props.Dispatcher))
    }

    return s.Spawn(id, props.Type, handler, opts...)
}

// StopActor 停止Actor
func (s *ActorSystem) StopActor(id string) error {
    s.actorsMu.Lock()
    actor, exists := s.actors[id]
    if !exists {
        s.actorsMu.Unlock()
        return fmt.Errorf("actor not found: %s", id)
    }
    delete(s.actors, id)
    s.actorsMu.Unlock()

    atomic.AddInt64(&s.stats.ActiveActors, -1)

    return actor.Stop(s.ctx)
}

// Tell 异步发送消息
func (s *ActorSystem) Tell(id string, msg Message) {
    s.actorsMu.RLock()
    actor, exists := s.actors[id]
    s.actorsMu.RUnlock()

    if !exists {
        return
    }

    atomic.AddInt64(&s.stats.MessagesSent, 1)
    actor.Tell(msg)
}

// Ask 同步请求
func (s *ActorSystem) Ask(ctx context.Context, id string, msg Message) (Message, error) {
    s.actorsMu.RLock()
    actor, exists := s.actors[id]
    s.actorsMu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("actor not found: %s", id)
    }

    atomic.AddInt64(&s.stats.MessagesSent, 1)
    return actor.Ask(ctx, msg)
}

// Broadcast 广播消息
func (s *ActorSystem) Broadcast(msg Message) {
    s.actorsMu.RLock()
    actors := make([]Actor, 0, len(s.actors))
    for _, actor := range s.actors {
        actors = append(actors, actor)
    }
    s.actorsMu.RUnlock()

    for _, actor := range actors {
        actor.Tell(msg)
    }
}

// ActorOf 获取Actor引用
func (s *ActorSystem) ActorOf(id string) (*ActorRef, error) {
    s.actorsMu.RLock()
    _, exists := s.actors[id]
    s.actorsMu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("actor not found: %s", id)
    }

    return &ActorRef{
        id:   id,
        path: fmt.Sprintf("/%s/%s", s.name, id),
        sys:  s,
    }, nil
}

// Stats 返回统计信息
func (s *ActorSystem) Stats() SystemStats {
    return SystemStats{
        TotalActors:      atomic.LoadInt64(&s.stats.TotalActors),
        ActiveActors:     atomic.LoadInt64(&s.stats.ActiveActors),
        MessagesSent:     atomic.LoadInt64(&s.stats.MessagesSent),
        MessagesReceived: atomic.LoadInt64(&s.stats.MessagesReceived),
        StartTime:        s.stats.StartTime,
    }
}

// generateActorID 生成Actor ID
func generateActorID(actorType string) string {
    return fmt.Sprintf("%s-%d", actorType, time.Now().UnixNano())
}

// Props Actor属性
type Props struct {
    ID          string
    Type        string
    MailboxSize int
    Supervisor  Supervisor
    Dispatcher  Dispatcher
}

// NewProps 创建属性
func NewProps(actorType string) *Props {
    return &Props{
        Type:        actorType,
        MailboxSize: 1000,
    }
}

// WithID 设置ID
func (p *Props) WithID(id string) *Props {
    p.ID = id
    return p
}

// WithMailboxSize 设置邮箱大小
func (p *Props) WithMailboxSize(size int) *Props {
    p.MailboxSize = size
    return p
}

// WithSupervisor 设置监督器
func (p *Props) WithSupervisor(s Supervisor) *Props {
    p.Supervisor = s
    return p
}

// WithDispatcher 设置调度器
func (p *Props) WithDispatcher(d Dispatcher) *Props {
    p.Dispatcher = d
    return p
}
```

---

## 7. 消息类型

### 7.1 常用消息类型

```go
// internal/actor/messages.go
package actor

import (
    "time"
)

// PoisonPill 停止消息
type PoisonPill struct {
    BaseMessage
}

// NewPoisonPill 创建停止消息
func NewPoisonPill() *PoisonPill {
    return &PoisonPill{
        BaseMessage: BaseMessage{
            MsgType:   "poison_pill",
            MsgID:     generateMessageID(),
            Timestamp: time.Now(),
        },
    }
}

// Ping 心跳消息
type Ping struct {
    BaseMessage
    From string
}

// NewPing 创建心跳消息
func NewPing(from string) *Ping {
    return &Ping{
        BaseMessage: BaseMessage{
            MsgType:   "ping",
            MsgID:     generateMessageID(),
            Timestamp: time.Now(),
        },
        From: from,
    }
}

// Pong 心跳响应
type Pong struct {
    BaseMessage
    From string
}

// NewPong 创建心跳响应
func NewPong(from string) *Pong {
    return &Pong{
        BaseMessage: BaseMessage{
            MsgType:   "pong",
            MsgID:     generateMessageID(),
            Timestamp: time.Now(),
        },
        From: from,
    }
}

// StatusRequest 状态请求
type StatusRequest struct {
    BaseMessage
}

// NewStatusRequest 创建状态请求
func NewStatusRequest() *StatusRequest {
    return &StatusRequest{
        BaseMessage: BaseMessage{
            MsgType:   "status_request",
            MsgID:     generateMessageID(),
            Timestamp: time.Now(),
        },
    }
}

// StatusResponse 状态响应
type StatusResponse struct {
    BaseMessage
    Status interface{} `json:"status"`
}

// NewStatusResponse 创建状态响应
func NewStatusResponse(status interface{}) *StatusResponse {
    return &StatusResponse{
        BaseMessage: BaseMessage{
            MsgType:   "status_response",
            MsgID:     generateMessageID(),
            Timestamp: time.Now(),
        },
        Status: status,
    }
}
```

---

## 8. 使用示例

### 8.1 简单Actor示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/your-org/online-game/internal/actor"
)

// EchoActor 回显Actor
type EchoActor struct{}

func (e *EchoActor) Receive(ctx context.Context, msg actor.Message) error {
    fmt.Printf("Received: %v\n", msg)
    return nil
}

func main() {
    // 创建Actor系统
    system := actor.NewActorSystem("example")
    system.Start(context.Background())
    defer system.Stop(context.Background())

    // 创建Actor
    handler := func(ctx context.Context, msg actor.Message) error {
        if echo, ok := msg.(*actor.StringMessage); ok {
            fmt.Printf("Echo: %v\n", echo.Data)
        }
        return nil
    }

    ref, err := system.Spawn("echo", "echo", handler)
    if err != nil {
        panic(err)
    }

    // 发送消息
    ref.Tell(&actor.StringMessage{
        BaseMessage: actor.BaseMessage{
            MsgType: "hello",
            MsgID:   "msg-1",
        },
        Data: "Hello, Actor!",
    })

    // 等待处理
    time.Sleep(time.Second)
}
```

### 8.2 请求-响应示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/your-org/online-game/internal/actor"
)

func main() {
    system := actor.NewActorSystem("example")
    system.Start(context.Background())
    defer system.Stop(context.Background())

    // 创建计算Actor
    handler := func(ctx context.Context, msg actor.Message) error {
        if calc, ok := msg.(*actor.TypedMessage[int]); ok {
            result := calc.Data * calc.Data
            // 返回响应需要特殊处理
            // 实际实现中需要获取响应通道
        }
        return nil
    }

    ref, _ := system.Spawn("calculator", "calculator", handler)

    // 发送请求
    response, err := ref.Ask(context.Background(),
        actor.NewTypedMessage("square", 5))
    if err != nil {
        panic(err)
    }

    fmt.Printf("Response: %v\n", response)
}
```

### 8.3 监督示例

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/your-org/online-game/internal/actor"
)

func main() {
    system := actor.NewActorSystem("example")
    system.Start(context.Background())
    defer system.Stop(context.Background())

    // 创建监督器
    supervisor := actor.NewOneForOneSupervisor(func(err error) actor.Directive {
        if errors.Is(err, context.Canceled) {
            return actor.Stop
        }
        return actor.Restart
    })

    // 创建被监督的Actor
    handler := func(ctx context.Context, msg actor.Message) error {
        if msg.Type() == "fail" {
            return errors.New("intentional failure")
        }
        fmt.Printf("Processing: %s\n", msg.Type())
        return nil
    }

    ref, _ := system.Spawn("worker", "worker", handler,
        actor.WithSupervisor(supervisor))

    // 发送正常消息
    ref.Tell(actor.NewBaseMessage("hello"))

    // 发送失败消息
    ref.Tell(actor.NewBaseMessage("fail"))

    // Actor会自动重启
    time.Sleep(time.Second)

    // 发送正常消息
    ref.Tell(actor.NewBaseMessage("hello"))
}
```
