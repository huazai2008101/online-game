# 游戏引擎详细实现

**文档版本:** v1.0
**创建时间:** 2026-03-24

---

## 目录

1. [设计概述](#1-设计概述)
2. [引擎接口](#2-引擎接口)
3. [JavaScript 引擎](#3-javascript-引擎)
4. [WASM 引擎](#4-wasm-引擎)
5. [Lua 引擎](#5-lua-引擎)
6. [双引擎实现](#6-双引擎实现)
7. [引擎管理器](#7-引擎管理器)
8. [使用示例](#8-使用示例)

---

## 1. 设计概述

### 1.1 设计目标

游戏引擎的设计目标：

1. **多语言支持**: 支持 JavaScript、WASM、Lua 等多种脚本语言
2. **隔离性**: 每个游戏实例独立运行，互不影响
3. **资源控制**: 限制内存和CPU使用
4. **热更新**: 支持游戏代码热更新
5. **事件驱动**: 基于事件的通信机制

### 1.2 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    GameEngineManager                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ JSEngine     │  │ WASMEngine   │  │ LuaEngine    │      │
│  │              │  │              │  │              │      │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │      │
│  │ │ goja VM  │ │  │ │ wazero   │ │  │ │ gopher  │ │      │
│  │ └──────────┘ │  │ │ Runtime  │ │  │ │ -lua    │ │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                           │
                    ┌──────┴──────┐
                    │             │
              ┌─────┴─────┐ ┌───┴────┐
              │ Game      │ │  Game  │
              │ Instance  │ │Instance │
              └───────────┘ └─────────┘
```

---

## 2. 引擎接口

### 2.1 核心接口定义

```go
// internal/engine/engine.go
package engine

import (
    "context"
    "io"
    "time"
)

// Engine 游戏引擎接口
type Engine interface {
    // 基本信息
    Name() string
    Type() EngineType
    Version() string

    // 生命周期
    Init(ctx context.Context, opts ...InitOption) error
    LoadGame(ctx context.Context, code []byte) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Close() error

    // 函数调用
    Call(ctx context.Context, method string, args ...interface{}) (interface{}, error)
    CallAsync(ctx context.Context, method string, args ...interface{}) (chan interface{}, chan error)

    // 状态管理
    GetState(ctx context.Context) (interface{}, error)
    SetState(ctx context.Context, state interface{}) error
    ResetState(ctx context.Context) error

    // 事件处理
    EmitEvent(ctx context.Context, event string, data interface{}) error
    OnEvent(ctx context.Context, event string, handler EventHandler) error
    OffEvent(ctx context.Context, event string) error

    // 资源管理
    MemoryStats(ctx context.Context) (*MemoryStats, error)
    GCOpts() []GCOpt
    ForceGC(ctx context.Context) error

    // 调试
    EnableProfiling(ctx context.Context, enabled bool) error
    GetProfile(ctx context.Context, name string) (io.ReadCloser, error)
}

// EngineType 引擎类型
type EngineType string

const (
    EngineTypeJavaScript EngineType = "javascript"
    EngineTypeWASM       EngineType = "wasm"
    EngineTypeLua        EngineType = "lua"
    EngineTypePython      EngineType = "python"
)

// InitOption 初始化选项
type InitOption func(*InitConfig)

// InitConfig 初始化配置
type InitConfig struct {
    // 资源限制
    MaxMemoryBytes   int64
    MaxExecutionTime int
    Timeout          int

    // 功能开关
    EnableProfiling bool
    EnableDebug     bool
    EnableNetwork   bool

    // 日志
    Logger Logger

    // 自定义配置
    Config map[string]interface{}
}

// Logger 日志接口
type Logger interface {
    Debug(msg string, args ...interface{})
    Info(msg string, args ...interface{})
    Warn(msg string, args ...interface{})
    Error(msg string, args ...interface{})
}

// EventHandler 事件处理器
type EventHandler func(ctx context.Context, data interface{}) error

// MemoryStats 内存统计
type MemoryStats struct {
    Used      int64
    Allocated int64
    Limit     int64
    Objects   int64
}

// GCOpt GC选项
type GCOpt struct {
    Key   string
    Value interface{}
}

// GameInstance 游戏实例
type GameInstance struct {
    ID       string
    Engine   Engine
    Code     []byte
    State    interface{}
    CreatedAt time.Time
    StartedAt *time.Time
    StoppedAt *time.Time
}
```

### 2.2 引擎配置选项

```go
// internal/engine/options.go
package engine

import (
    "context"
    "time"
)

// WithMaxMemory 设置最大内存
func WithMaxMemory(bytes int64) InitOption {
    return func(c *InitConfig) {
        c.MaxMemoryBytes = bytes
    }
}

// WithTimeout 设置超时时间（毫秒）
func WithTimeout(timeout int) InitOption {
    return func(c *InitConfig) {
        c.Timeout = timeout
    }
}

// WithMaxExecutionTime 设置最大执行时间
func WithMaxExecutionTime(duration int) InitOption {
    return func(c *InitConfig) {
        c.MaxExecutionTime = duration
    }
}

// WithProfiling 启用性能分析
func WithProfiling(enabled bool) InitOption {
    return func(c *InitConfig) {
        c.EnableProfiling = enabled
    }
}

// WithDebug 启用调试
func WithDebug(enabled bool) InitOption {
    return func(c *InitConfig) {
        c.EnableDebug = enabled
    }
}

// WithLogger 设置日志
func WithLogger(logger Logger) InitOption {
    return func(c *InitConfig) {
        c.Logger = logger
    }
}

// WithConfig 设置自定义配置
func WithConfig(key string, value interface{}) InitOption {
    return func(c *InitConfig) {
        if c.Config == nil {
            c.Config = make(map[string]interface{})
        }
        c.Config[key] = value
    }
}
```

---

## 3. JavaScript 引擎

### 3.1 引擎实现

```go
// internal/engine/js_engine.go
package engine

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/dop251/goja"
    "github.com/dop251/goja_nodejs"
)

// JSEngine JavaScript引擎
type JSEngine struct {
    vm       *goja.Runtime
    conf     InitConfig
    mu       sync.RWMutex
    started  bool
    handlers map[string][]EventHandler
    logger   Logger
    stats    EngineStats
}

// EngineStats 引擎统计
type EngineStats struct {
    CallsTotal     int64
    CallsSuccess   int64
    CallsFailed    int64
    EventsEmitted  int64
    EventsHandled  int64
    MemoryUsed     int64
    LastGCAt       time.Time
}

// NewJSEngine 创建JavaScript引擎
func NewJSEngine(opts ...InitOption) *JSEngine {
    conf := InitConfig{
        MaxMemoryBytes:   50 * 1024 * 1024, // 50MB
        Timeout:          5000,
        MaxExecutionTime: 5000,
        EnableProfiling:  false,
        EnableDebug:      false,
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

// Name 返回引擎名称
func (e *JSEngine) Name() string {
    return "javascript"
}

// Type 返回引擎类型
func (e *JSEngine) Type() EngineType {
    return EngineTypeJavaScript
}

// Version 返回版本
func (e *JSEngine) Version() string {
    return "1.0.0"
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
    if e.conf.EnableDebug {
        // 使用Node.js兼容模式
        e.vm = goja_nodejs.New()
    } else {
        e.vm = goja.New()
    }

    // 设置超时
    if e.conf.Timeout > 0 {
        e.vm.SetMaxCallStackSize(1000)
    }

    // 注入内置对象
    e.injectBuiltins()

    return nil
}

// injectBuiltins 注入内置对象和函数
func (e *JSEngine) injectBuiltins() {
    // 注入console
    console := map[string]interface{}{
        "log":   e.jsLog,
        "debug": e.jsDebug,
        "info":  e.jsInfo,
        "warn":  e.jsWarn,
        "error": e.jsError,
    }
    _ = e.vm.Set("console", console)

    // 注入setTimeout
    _ = e.vm.Set("setTimeout", e.jsSetTimeout)
    _ = e.vm.Set("clearTimeout", e.jsClearTimeout)

    // 注入setInterval
    _ = e.vm.Set("setInterval", e.jsSetInterval)
    _ = e.vm.Set("clearInterval", e.jsClearInterval)

    // 注入Promise
    // goja内置支持Promise

    // 注入JSON
    _ = e.vm.Set("JSON", map[string]interface{}{
        "stringify": e.jsJSONStringify,
        "parse":     e.jsJSONParse,
    })

    // 注入emitEvent函数
    _ = e.vm.Set("emitEvent", e.jsEmitEvent)

    // 注入游戏API
    gameAPI := map[string]interface{}{
        "getState":     e.jsGetState,
        "setState":     e.jsSetState,
        "getPlayer":    e.jsGetPlayer,
        "getPlayers":   e.jsGetPlayers,
        "sendToPlayer": e.jsSendToPlayer,
        "broadcast":    e.jsBroadcast,
        "endGame":      e.jsEndGame,
    }
    _ = e.vm.Set("Game", gameAPI)
}

// LoadGame 加载游戏代码
func (e *JSEngine) LoadGame(ctx context.Context, code []byte) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 编译代码
    script, err := goja.Compile("", string(code), false)
    if err != nil {
        return fmt.Errorf("failed to compile game code: %w", err)
    }

    // 执行代码
    _, err = e.vm.RunProgram(script)
    if err != nil {
        return fmt.Errorf("failed to execute game code: %w", err)
    }

    return nil
}

// Start 启动游戏
func (e *JSEngine) Start(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if e.started {
        return fmt.Errorf("engine already started")
    }

    // 调用onStart函数
    if err := e.callFunction("onStart"); err != nil {
        if !isNotFoundError(err) {
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

    // 调用onStop函数
    _ = e.callFunction("onStop")

    // 清理定时器
    e.cleanupTimers()

    e.started = false
    return nil
}

// Call 调用游戏函数
func (e *JSEngine) Call(ctx context.Context, method string, args ...interface{}) (interface{}, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()

    e.stats.CallsTotal++

    // 设置超时
    callCtx := ctx
    if e.conf.Timeout > 0 {
        var cancel context.CancelFunc
        callCtx, cancel = context.WithTimeout(ctx, time.Duration(e.conf.Timeout)*time.Millisecond)
        defer cancel()
    }

    // 执行调用
    resultChan := make(chan interface{}, 1)
    errChan := make(chan error, 1)

    go func() {
        defer func() {
            if r := recover(); r != nil {
                errChan <- fmt.Errorf("panic: %v", r)
            }
        }()

        result, err := e.callFunction(method, args...)
        if err != nil {
            errChan <- err
            return
        }
        resultChan <- result
    }()

    select {
    case result := <-resultChan:
        e.stats.CallsSuccess++
        return result, nil
    case err := <-errChan:
        e.stats.CallsFailed++
        return nil, err
    case <-callCtx.Done():
        e.stats.CallsFailed++
        return nil, callCtx.Err()
    }
}

// CallAsync 异步调用
func (e *JSEngine) CallAsync(ctx context.Context, method string, args ...interface{}) (chan interface{}, chan error) {
    resultChan := make(chan interface{}, 1)
    errChan := make(chan error, 1)

    go func() {
        result, err := e.Call(ctx, method, args...)
        if err != nil {
            errChan <- err
            return
        }
        resultChan <- result
    }()

    return resultChan, errChan
}

// GetState 获取游戏状态
func (e *JSEngine) GetState(ctx context.Context) (interface{}, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()

    return e.Call(ctx, "getState")
}

// SetState 设置游戏状态
func (e *JSEngine) SetState(ctx context.Context, state interface{}) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 转换为goja值
    jsValue := e.vm.ToValue(state)

    // 设置全局状态
    _ = e.vm.Set("gameState", jsValue)

    return nil
}

// ResetState 重置状态
func (e *JSEngine) ResetState(ctx context.Context) error {
    return e.SetState(ctx, nil)
}

// EmitEvent 触发事件
func (e *JSEngine) EmitEvent(ctx context.Context, event string, data interface{}) error {
    e.mu.RLock()
    handlers := e.handlers[event]
    e.mu.RUnlock()

    e.stats.EventsEmitted++

    for _, handler := range handlers {
        if err := handler(ctx, data); err != nil {
            return err
        }
        e.stats.EventsHandled++
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

// OffEvent 取消监听
func (e *JSEngine) OffEvent(ctx context.Context, event string) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    delete(e.handlers, event)
    return nil
}

// MemoryStats 返回内存统计
func (e *JSEngine) MemoryStats(ctx context.Context) (*MemoryStats, error) {
    return &MemoryStats{
        Used:      e.stats.MemoryUsed,
        Limit:     e.conf.MaxMemoryBytes,
    }, nil
}

// GCOpts 返回GC选项
func (e *JSEngine) GCOpts() []GCOpt {
    return []GCOpt{
        {Key: "enabled", Value: true},
    }
}

// ForceGC 强制GC
func (e *JSEngine) ForceGC(ctx context.Context) error {
    // goja没有暴露GC接口
    e.stats.LastGCAt = time.Now()
    return nil
}

// EnableProfiling 启用性能分析
func (e *JSEngine) EnableProfiling(ctx context.Context, enabled bool) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    e.conf.EnableProfiling = enabled
    return nil
}

// GetProfile 获取性能分析数据
func (e *JSEngine) GetProfile(ctx context.Context, name string) (io.ReadCloser, error) {
    // TODO: 实现CPU/内存profile
    return nil, nil
}

// Close 关闭引擎
func (e *JSEngine) Close() error {
    e.mu.Lock()
    defer e.mu.Unlock()

    e.vm = nil
    e.handlers = make(map[string][]EventHandler)
    e.cleanupTimers()

    return nil
}

// Stats 返回统计信息
func (e *JSEngine) Stats() EngineStats {
    e.mu.RLock()
    defer e.mu.RUnlock()
    return e.stats
}

// ====== JavaScript 辅助函数 ======

func (e *JSEngine) callFunction(name string, args ...interface{}) (interface{}, error) {
    fn, ok := goja.AssertFunction(e.vm.Get(name))
    if !ok {
        return nil, fmt.Errorf("function not found: %s", name)
    }

    jsArgs := make([]goja.Value, len(args))
    for i, arg := range args {
        jsArgs[i] = e.vm.ToValue(arg)
    }

    return fn(goja.Undefined(), jsArgs...)
}

// ====== JavaScript 内置函数 ======

func (e *JSEngine) jsLog(call goja.FunctionCall) goja.Value {
    msg := call.Argument(0).String()
    if e.logger != nil {
        e.logger.Info(msg)
    }
    return goja.Undefined()
}

func (e *JSEngine) jsDebug(call goja.FunctionCall) goja.Value {
    msg := call.Argument(0).String()
    if e.logger != nil {
        e.logger.Debug(msg)
    }
    return goja.Undefined()
}

func (e *JSEngine) jsInfo(call goja.FunctionCall) goja.Value {
    msg := call.Argument(0).String()
    if e.logger != nil {
        e.logger.Info(msg)
    }
    return goja.Undefined()
}

func (e *JSEngine) jsWarn(call goja.FunctionCall) goja.Value {
    msg := call.Argument(0).String()
    if e.logger != nil {
        e.logger.Warn(msg)
    }
    return goja.Undefined()
}

func (e *JSEngine) jsError(call goja.FunctionCall) goja.Value {
    msg := call.Argument(0).String()
    if e.logger != nil {
        e.logger.Error(msg)
    }
    return goja.Undefined()
}

var (
    timerID   int64
    timers    = make(map[int64]*time.Timer)
    timersMu  sync.Mutex
    intervals = make(map[int64]*time.Ticker)
)

func (e *JSEngine) jsSetTimeout(call goja.FunctionCall) goja.Value {
    fn, ok := goja.AssertFunction(call.Argument(0))
    if !ok {
        return goja.Undefined()
    }

    delay := call.Argument(1).ToInteger()
    if delay < 0 {
        delay = 0
    }

    timersMu.Lock()
    timerID++
    id := timerID
    timersMu.Unlock()

    timer := time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
        _, _ = fn(goja.Undefined())
    })

    timersMu.Lock()
    timers[id] = timer
    timersMu.Unlock()

    return e.vm.ToValue(id)
}

func (e *JSEngine) jsClearTimeout(call goja.FunctionCall) goja.Value {
    id := int64(call.Argument(0).ToInteger())

    timersMu.Lock()
    if timer, ok := timers[id]; ok {
        timer.Stop()
        delete(timers, id)
    }
    timersMu.Unlock()

    return goja.Undefined()
}

func (e *JSEngine) jsSetInterval(call goja.FunctionCall) goja.Value {
    fn, ok := goja.AssertFunction(call.Argument(0))
    if !ok {
        return goja.Undefined()
    }

    delay := call.Argument(1).ToInteger()
    if delay < 0 {
        delay = 0
    }

    timersMu.Lock()
    timerID++
    id := timerID
    timersMu.Unlock()

    ticker := time.NewTicker(time.Duration(delay) * time.Millisecond)
    go func() {
        for range ticker.C {
            _, _ = fn(goja.Undefined())
        }
    }()

    timersMu.Lock()
    intervals[id] = ticker
    timersMu.Unlock()

    return e.vm.ToValue(id)
}

func (e *JSEngine) jsClearInterval(call goja.FunctionCall) goja.Value {
    id := int64(call.Argument(0).ToInteger())

    timersMu.Lock()
    if ticker, ok := intervals[id]; ok {
        ticker.Stop()
        delete(intervals, id)
    }
    timersMu.Unlock()

    return goja.Undefined()
}

func (e *JSEngine) cleanupTimers() {
    timersMu.Lock()
    defer timersMu.Unlock()

    for _, timer := range timers {
        timer.Stop()
    }
    for _, ticker := range intervals {
        ticker.Stop()
    }

    timers = make(map[int64]*time.Timer)
    intervals = make(map[int64]*time.Ticker)
}

func (e *JSEngine) jsJSONStringify(call goja.FunctionCall) goja.Value {
    value := call.Argument(0)
    str, err := goja.AssertFunction(e.vm.Get("JSON.stringify"))(goja.Undefined(), value)
    if err != nil {
        return goja.Undefined()
    }
    return str
}

func (e *JSEngine) jsJSONParse(call goja.FunctionCall) goja.Value {
    str := call.Argument(0).String()
    value, err := goja.AssertFunction(e.vm.Get("JSON.parse"))(goja.Undefined(), e.vm.ToValue(str))
    if err != nil {
        return goja.Undefined()
    }
    return value
}

func (e *JSEngine) jsEmitEvent(call goja.FunctionCall) goja.Value {
    event := call.Argument(0).String()
    data := call.Argument(1).Export()

    go func() {
        _ = e.EmitEvent(context.Background(), event, data)
    }()

    return goja.Undefined()
}

// ====== 游戏API ======

func (e *JSEngine) jsGetState(call goja.FunctionCall) goja.Value {
    state, _ := e.GetState(context.Background())
    return e.vm.ToValue(state)
}

func (e *JSEngine) jsSetState(call goja.FunctionCall) goja.Value {
    state := call.Argument(0).Export()
    _ = e.SetState(context.Background(), state)
    return goja.Undefined()
}

func (e *JSEngine) jsGetPlayer(call goja.FunctionCall) goja.Value {
    // TODO: 实现获取玩家逻辑
    return goja.Undefined()
}

func (e *JSEngine) jsGetPlayers(call goja.FunctionCall) goja.Value {
    // TODO: 实现获取玩家列表逻辑
    return e.vm.ToValue([]interface{}{})
}

func (e *JSEngine) jsSendToPlayer(call goja.FunctionCall) goja.Value {
    // TODO: 实现发送消息给玩家逻辑
    return goja.Undefined()
}

func (e *JSEngine) jsBroadcast(call goja.FunctionCall) goja.Value {
    // TODO: 实现广播逻辑
    return goja.Undefined()
}

func (e *JSEngine) jsEndGame(call goja.FunctionCall) goja.Value {
    // TODO: 实现结束游戏逻辑
    return goja.Undefined()
}

func isNotFoundError(err error) bool {
    return err != nil && (err.Error() == "function not found" ||
        err.Error() == "undefined is not a function")
}
```

---

## 4. WASM 引擎

### 4.1 引擎实现

```go
// internal/engine/wasm_engine.go
package engine

import (
    "context"
    "embed"
    "errors"
    "fmt"
    "io"
    "sync"
    "time"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/api"
    "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMEngine WASM引擎
type WASMEngine struct {
    runtime  wazero.Runtime
    module   wazero.CompiledModule
    instance api.Module
    conf     InitConfig
    mu       sync.RWMutex
    started  bool
    memory   []byte
    logger   Logger
    stats    EngineStats
}

// NewWASMEngine 创建WASM引擎
func NewWASMEngine(opts ...InitOption) *WASMEngine {
    conf := InitConfig{
        MaxMemoryBytes:   50 * 1024 * 1024,
        Timeout:          5000,
        MaxExecutionTime: 5000,
        EnableProfiling:  false,
        EnableDebug:      false,
    }

    for _, opt := range opts {
        opt(&conf)
    }

    return &WASMEngine{
        conf:   conf,
        logger: conf.Logger,
    }
}

// Name 返回引擎名称
func (e *WASMEngine) Name() string {
    return "wasm"
}

// Type 返回引擎类型
func (e *WASMEngine) Type() EngineType {
    return EngineTypeWASM
}

// Version 返回版本
func (e *WASMEngine) Version() string {
    return "1.0.0"
}

// Init 初始化引擎
func (e *WASMEngine) Init(ctx context.Context, opts ...InitOption) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    for _, opt := range opts {
        opt(&e.conf)
    }

    // 创建运行时
    config := wazero.NewRuntimeConfig().
        WithDebugInfoEnabled(e.conf.EnableDebug).
        WithMemoryLimit(e.conf.MaxMemoryBytes)

    e.runtime = wazero.NewRuntimeWithConfig(ctx, config)

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
        return fmt.Errorf("engine already started")
    }

    if e.module == nil {
        return fmt.Errorf("no module loaded")
    }

    // 实例化模块
    config := wazero.NewModuleConfig().
        WithStdout(&wasmWriter{logger: e.logger, level: "info"}).
        WithStderr(&wasmWriter{logger: e.logger, level: "error"})

    instance, err := e.runtime.InstantiateModule(ctx, e.module, config)
    if err != nil {
        return fmt.Errorf("failed to instantiate module: %w", err)
    }

    e.instance = instance

    // 调用onStart函数
    if onStart := instance.ExportedFunction("onStart"); onStart != nil {
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

    // 调用onStop函数
    if e.instance != nil {
        if onStop := e.instance.ExportedFunction("onStop"); onStop != nil {
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
        return nil, fmt.Errorf("engine not started")
    }

    e.stats.CallsTotal++

    fn := e.instance.ExportedFunction(method)
    if fn == nil {
        e.stats.CallsFailed++
        return nil, fmt.Errorf("function not found: %s", method)
    }

    // 转换参数
    wasmArgs, err := e.convertArgs(fn, args...)
    if err != nil {
        e.stats.CallsFailed++
        return nil, fmt.Errorf("failed to convert args: %w", err)
    }

    // 调用函数
    results, err := fn(ctx, wasmArgs...)
    if err != nil {
        e.stats.CallsFailed++
        return nil, fmt.Errorf("function call failed: %w", err)
    }

    e.stats.CallsSuccess++

    // 返回结果
    if len(results) > 0 {
        return e.convertResult(results[0]), nil
    }

    return nil, nil
}

// CallAsync 异步调用
func (e *WASMEngine) CallAsync(ctx context.Context, method string, args ...interface{}) (chan interface{}, chan error) {
    resultChan := make(chan interface{}, 1)
    errChan := make(chan error, 1)

    go func() {
        result, err := e.Call(ctx, method, args...)
        if err != nil {
            errChan <- err
            return
        }
        resultChan <- result
    }()

    return resultChan, errChan
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

// ResetState 重置状态
func (e *WASMEngine) ResetState(ctx context.Context) error {
    _, err := e.Call(ctx, "resetState")
    return err
}

// EmitEvent 触发事件
func (e *WASMEngine) EmitEvent(ctx context.Context, event string, data interface{}) error {
    e.mu.RLock()
    defer e.mu.RUnlock()

    e.stats.EventsEmitted++

    if e.instance == nil {
        return nil
    }

    // 调用WASM的事件处理函数
    if onEvent := e.instance.ExportedFunction("onEvent"); onEvent != nil {
        // 将事件名和数据写入内存
        // 这里简化处理
        _, _ = onEvent(ctx)
        e.stats.EventsHandled++
    }

    return nil
}

// OnEvent 监听事件
func (e *WASMEngine) OnEvent(ctx context.Context, event string, handler EventHandler) error {
    // WASM引擎通常不需要从Go侧监听事件
    return nil
}

// OffEvent 取消监听
func (e *WASMEngine) OffEvent(ctx context.Context, event string) error {
    return nil
}

// MemoryStats 返回内存统计
func (e *WASMEngine) MemoryStats(ctx context.Context) (*MemoryStats, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()

    if e.instance == nil {
        return &MemoryStats{
            Limit: e.conf.MaxMemoryBytes,
        }, nil
    }

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

// ForceGC 强制GC
func (e *WASMEngine) ForceGC(ctx context.Context) error {
    // WASM有自动内存管理
    e.stats.LastGCAt = time.Now()
    return nil
}

// EnableProfiling 启用性能分析
func (e *WASMEngine) EnableProfiling(ctx context.Context, enabled bool) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.conf.EnableProfiling = enabled
    return nil
}

// GetProfile 获取性能分析数据
func (e *WASMEngine) GetProfile(ctx context.Context, name string) (io.ReadCloser, error) {
    // TODO: 实现CPU/内存profile
    return nil, nil
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

// convertArgs 转换参数为WASM格式
func (e *WASMEngine) convertArgs(fn api.Function, args ...interface{}) ([]uint64, error) {
    paramTypes := fn.ParamTypes()
    if len(args) != len(paramTypes) {
        return nil, fmt.Errorf("argument count mismatch: expected %d, got %d", len(paramTypes), len(args))
    }

    result := make([]uint64, len(args))
    for i, arg := range args {
        switch paramTypes[i] {
        case api.ValueTypeI32:
            if v, ok := arg.(int32); ok {
                result[i] = uint64(v)
            } else if v, ok := arg.(int); ok {
                result[i] = uint64(uint32(v))
            } else {
                return nil, fmt.Errorf("invalid argument type for param %d", i)
            }
        case api.ValueTypeI64:
            if v, ok := arg.(int64); ok {
                result[i] = uint64(v)
            } else if v, ok := arg.(int); ok {
                result[i] = uint64(v)
            } else {
                return nil, fmt.Errorf("invalid argument type for param %d", i)
            }
        case api.ValueTypeF32:
            if v, ok := arg.(float32); ok {
                result[i] = uint64(math.Float32bits(v))
            } else {
                return nil, fmt.Errorf("invalid argument type for param %d", i)
            }
        case api.ValueTypeF64:
            if v, ok := arg.(float64); ok {
                result[i] = uint64(math.Float64bits(v))
            } else {
                return nil, fmt.Errorf("invalid argument type for param %d", i)
            }
        default:
            return nil, fmt.Errorf("unsupported param type: %v", paramTypes[i])
        }
    }

    return result, nil
}

// convertResult 转换结果
func (e *WASMEngine) convertResult(v uint64) interface{} {
    // 根据函数签名转换结果
    // 这里简化处理
    return v
}

// wasmWriter WASM输出写入器
type wasmWriter struct {
    logger Logger
    level  string
}

func (w *wasmWriter) Write(p []byte) (n int, err error) {
    if w.logger != nil {
        msg := string(p)
        switch w.level {
        case "debug":
            w.logger.Debug(msg)
        case "info":
            w.logger.Info(msg)
        case "warn":
            w.logger.Warn(msg)
        case "error":
            w.logger.Error(msg)
        }
    }
    return len(p), nil
}
```

---

## 5. Lua 引擎

### 5.1 引擎实现

```go
// internal/engine/lua_engine.go
package engine

import (
    "context"
    "fmt"
    "sync"

    "github.com/yuin/gopher-lua"
)

// LuaEngine Lua引擎
type LuaEngine struct {
    L      *lua.LState
    conf   InitConfig
    mu     sync.RWMutex
    started bool
    logger Logger
    stats  EngineStats
}

// NewLuaEngine 创建Lua引擎
func NewLuaEngine(opts ...InitOption) *LuaEngine {
    conf := InitConfig{
        MaxMemoryBytes:   50 * 1024 * 1024,
        Timeout:          5000,
        MaxExecutionTime: 5000,
        EnableProfiling:  false,
        EnableDebug:      false,
    }

    for _, opt := range opts {
        opt(&conf)
    }

    return &LuaEngine{
        conf:   conf,
        logger: conf.Logger,
    }
}

// Name 返回引擎名称
func (e *LuaEngine) Name() string {
    return "lua"
}

// Type 返回引擎类型
func (e *LuaEngine) Type() EngineType {
    return EngineTypeLua
}

// Version 返回版本
func (e *LuaEngine) Version() string {
    return "1.0.0"
}

// Init 初始化引擎
func (e *LuaEngine) Init(ctx context.Context, opts ...InitOption) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    for _, opt := range opts {
        opt(&e.conf)
    }

    e.L = lua.NewState()
    e.injectBuiltins()

    return nil
}

// injectBuiltins 注入内置函数
func (e *LuaEngine) injectBuiltins() {
    // 注入print
    e.L.SetGlobal("print", e.L.NewFunction(e.luaPrint))

    // 注入游戏API
    gameTable := e.L.NewTable()
    e.L.SetField(gameTable, "getState", e.L.NewFunction(e.luaGetState))
    e.L.SetField(gameTable, "setState", e.L.NewFunction(e.luaSetState))
    e.L.SetGlobal("Game", gameTable)
}

// LoadGame 加载游戏代码
func (e *LuaEngine) LoadGame(ctx context.Context, code []byte) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    return e.L.DoString(string(code))
}

// Start 启动游戏
func (e *LuaEngine) Start(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if e.started {
        return fmt.Errorf("engine already started")
    }

    // 调用onStart
    if err := e.L.CallByParam(lua.P{
        Fn:      e.L.GetGlobal("onStart"),
        NRet:    0,
        Protect: true,
    }); err != nil && err.Error() != "attempt to call a nil value" {
        return err
    }

    e.started = true
    return nil
}

// Stop 停止游戏
func (e *LuaEngine) Stop(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if !e.started {
        return nil
    }

    // 调用onStop
    _ = e.L.CallByParam(lua.P{
        Fn:      e.L.GetGlobal("onStop"),
        NRet:    0,
        Protect: true,
    })

    e.started = false
    return nil
}

// Call 调用游戏函数
func (e *LuaEngine) Call(ctx context.Context, method string, args ...interface{}) (interface{}, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()

    e.stats.CallsTotal++

    fn := e.L.GetGlobal(method)
    if !lua.LVIsFunction(fn) {
        e.stats.CallsFailed++
        return nil, fmt.Errorf("function not found: %s", method)
    }

    // 推入参数
    e.L.Push(fn)
    for _, arg := range args {
        e.L.Push(e.convertToLua(arg))
    }

    // 调用函数
    err := e.L.PCall(len(args), 1, nil)
    if err != nil {
        e.stats.CallsFailed++
        return nil, err
    }

    e.stats.CallsSuccess++

    // 获取返回值
    ret := e.L.Get(-1)
    e.L.Pop(1)

    return e.convertFromLua(ret), nil
}

// CallAsync 异步调用
func (e *LuaEngine) CallAsync(ctx context.Context, method string, args ...interface{}) (chan interface{}, chan error) {
    resultChan := make(chan interface{}, 1)
    errChan := make(chan error, 1)

    go func() {
        result, err := e.Call(ctx, method, args...)
        if err != nil {
            errChan <- err
            return
        }
        resultChan <- result
    }()

    return resultChan, errChan
}

// GetState 获取游戏状态
func (e *LuaEngine) GetState(ctx context.Context) (interface{}, error) {
    return e.Call(ctx, "getState")
}

// SetState 设置游戏状态
func (e *LuaEngine) SetState(ctx context.Context, state interface{}) error {
    _, err := e.Call(ctx, "setState", state)
    return err
}

// ResetState 重置状态
func (e *LuaEngine) ResetState(ctx context.Context) error {
    return e.SetState(ctx, nil)
}

// EmitEvent 触发事件
func (e *LuaEngine) EmitEvent(ctx context.Context, event string, data interface{}) error {
    e.stats.EventsEmitted++
    _, err := e.Call(ctx, "onEvent", event, data)
    if err == nil {
        e.stats.EventsHandled++
    }
    return err
}

// OnEvent 监听事件
func (e *LuaEngine) OnEvent(ctx context.Context, event string, handler EventHandler) error {
    // Lua引擎中事件处理通常在Lua代码中实现
    return nil
}

// OffEvent 取消监听
func (e *LuaEngine) OffEvent(ctx context.Context, event string) error {
    return nil
}

// MemoryStats 返回内存统计
func (e *LuaEngine) MemoryStats(ctx context.Context) (*MemoryStats, error) {
    return &MemoryStats{
        Used:  e.stats.MemoryUsed,
        Limit: e.conf.MaxMemoryBytes,
    }, nil
}

// GCOpts 返回GC选项
func (e *LuaEngine) GCOpts() []GCOpt {
    return []GCOpt{
        {Key: "enabled", Value: true},
    }
}

// ForceGC 强制GC
func (e *LuaEngine) ForceGC(ctx context.Context) error {
    e.L.GC(1) // 强制执行垃圾回收
    e.stats.LastGCAt = time.Now()
    return nil
}

// EnableProfiling 启用性能分析
func (e *LuaEngine) EnableProfiling(ctx context.Context, enabled bool) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.conf.EnableProfiling = enabled
    return nil
}

// GetProfile 获取性能分析数据
func (e *LuaEngine) GetProfile(ctx context.Context, name string) (io.ReadCloser, error) {
    return nil, nil
}

// Close 关闭引擎
func (e *LuaEngine) Close() error {
    e.mu.Lock()
    defer e.mu.Unlock()

    if e.L != nil {
        e.L.Close()
    }

    return nil
}

// ====== Lua 辅助函数 ======

func (e *LuaEngine) convertToLua(v interface{}) lua.LValue {
    switch val := v.(type) {
    case int:
        return lua.LNumber(val)
    case int32:
        return lua.LNumber(val)
    case int64:
        return lua.LNumber(val)
    case float32:
        return lua.LNumber(val)
    case float64:
        return lua.LNumber(val)
    case string:
        return lua.LString(val)
    case bool:
        if val {
            return lua.LTrue
        }
        return lua.LFalse
    case []interface{}:
        tbl := e.L.NewTable()
        for i, item := range val {
            e.L.SetField(tbl, lua.LNumber(i+1), e.convertToLua(item))
        }
        return tbl
    case map[string]interface{}:
        tbl := e.L.NewTable()
        for k, item := range val {
            e.L.SetField(tbl, lua.LString(k), e.convertToLua(item))
        }
        return tbl
    case nil:
        return lua.LNil
    default:
        return lua.LNil
    }
}

func (e *LuaEngine) convertFromLua(v lua.LValue) interface{} {
    switch v.Type() {
    case lua.LTNil:
        return nil
    case lua.LTBool:
        return v == lua.LTrue
    case lua.LTNumber:
        return float64(v.(lua.LNumber))
    case lua.LTString:
        return v.String()
    case lua.LTTable:
        // 判断是数组还是对象
        if isArray(v.(*lua.LTable)) {
            arr := make([]interface{}, 0)
            v.(*lua.LTable).ForEach(func(key, value lua.LValue) {
                arr = append(arr, e.convertFromLua(value))
            })
            return arr
        }
        obj := make(map[string]interface{})
        v.(*lua.LTable).ForEach(func(key, value lua.LValue) {
            obj[key.String()] = e.convertFromLua(value)
        })
        return obj
    default:
        return nil
    }
}

func isArray(tbl *lua.LTable) bool {
    max := int64(0)
    tbl.ForEach(func(key, _ lua.LValue) {
        if key.Type() == lua.LTNumber {
            n := int64(key.(lua.LNumber))
            if n > max {
                max = n
            }
        }
    })
    return max > 0 && max == int64(len(tbl.RawGetString("length")))
}

// ====== Lua 内置函数 ======

func (e *LuaEngine) luaPrint(L *lua.LState) int {
    msg := L.ToString(1)
    if e.logger != nil {
        e.logger.Info(msg)
    }
    return 0
}

func (e *LuaEngine) luaGetState(L *lua.LState) int {
    state, _ := e.GetState(context.Background())
    L.Push(e.convertToLua(state))
    return 1
}

func (e *LuaEngine) luaSetState(L *lua.LState) int {
    state := e.convertFromLua(L.Get(1))
    _ = e.SetState(context.Background(), state)
    return 0
}
```

---

## 6. 双引擎实现

### 6.1 双引擎实现

```go
// internal/engine/dual_engine.go
package engine

import (
    "context"
    "errors"
    "sync"
)

// DualEngine 双引擎实现
type DualEngine struct {
    primary   Engine
    secondary Engine
    selector  EngineSelector
    active    Engine
    conf      InitConfig
    mu        sync.RWMutex
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

// Name 返回引擎名称
func (e *DualEngine) Name() string {
    return "dual"
}

// Type 返回引擎类型
func (e *DualEngine) Type() EngineType {
    return EngineTypeJavaScript // 默认类型
}

// Version 返回版本
func (e *DualEngine) Version() string {
    return "1.0.0"
}

// Init 初始化引擎
func (e *DualEngine) Init(ctx context.Context, opts ...InitOption) error {
    e.mu.Lock()
    defer e.mu.Unlock()

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
    case EngineTypeLua:
        // 如果有第三个引擎
        return errors.New("lua engine not configured")
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

// CallAsync 异步调用
func (e *DualEngine) CallAsync(ctx context.Context, method string, args ...interface{}) (chan interface{}, chan error) {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil, nil
    }

    return active.CallAsync(ctx, method, args...)
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

// ResetState 重置状态
func (e *DualEngine) ResetState(ctx context.Context) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil
    }

    return active.ResetState(ctx)
}

// EmitEvent 触发事件
func (e *DualEngine) EmitEvent(ctx context.Context, event string, data interface{}) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil
    }

    return active.EmitEvent(ctx, event, data)
}

// OnEvent 监听事件
func (e *DualEngine) OnEvent(ctx context.Context, event string, handler EventHandler) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil
    }

    return active.OnEvent(ctx, event, handler)
}

// OffEvent 取消监听
func (e *DualEngine) OffEvent(ctx context.Context, event string) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil
    }

    return active.OffEvent(ctx, event)
}

// MemoryStats 返回内存统计
func (e *DualEngine) MemoryStats(ctx context.Context) (*MemoryStats, error) {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return &MemoryStats{
            Limit: e.conf.MaxMemoryBytes,
        }, nil
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

// ForceGC 强制GC
func (e *DualEngine) ForceGC(ctx context.Context) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil
    }

    return active.ForceGC(ctx)
}

// EnableProfiling 启用性能分析
func (e *DualEngine) EnableProfiling(ctx context.Context, enabled bool) error {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil
    }

    return active.EnableProfiling(ctx, enabled)
}

// GetProfile 获取性能分析数据
func (e *DualEngine) GetProfile(ctx context.Context, name string) (io.ReadCloser, error) {
    e.mu.RLock()
    active := e.active
    e.mu.RUnlock()

    if active == nil {
        return nil, nil
    }

    return active.GetProfile(ctx, name)
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

---

## 7. 引擎管理器

### 7.1 引擎管理器实现

```go
// internal/engine/manager.go
package engine

import (
    "context"
    "sync"
)

// Manager 引擎管理器
type Manager struct {
    engines map[string]Engine
    mu      sync.RWMutex
}

// NewManager 创建引擎管理器
func NewManager() *Manager {
    return &Manager{
        engines: make(map[string]Engine),
    }
}

// Register 注册引擎
func (m *Manager) Register(id string, engine Engine) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.engines[id] = engine
}

// Unregister 注销引擎
func (m *Manager) Unregister(id string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.engines, id)
}

// Get 获取引擎
func (m *Manager) Get(id string) (Engine, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    engine, ok := m.engines[id]
    return engine, ok
}

// CreateInstance 创建游戏实例
func (m *Manager) CreateInstance(ctx context.Context, engineID string, code []byte) (*GameInstance, error) {
    engine, ok := m.Get(engineID)
    if !ok {
        return nil, fmt.Errorf("engine not found: %s", engineID)
    }

    instance := &GameInstance{
        ID:        generateInstanceID(),
        Engine:    engine,
        Code:      code,
        CreatedAt: time.Now(),
    }

    // 加载代码
    if err := engine.LoadGame(ctx, code); err != nil {
        return nil, err
    }

    return instance, nil
}

// StartInstance 启动游戏实例
func (m *Manager) StartInstance(ctx context.Context, instance *GameInstance) error {
    now := time.Now()
    instance.StartedAt = &now
    return instance.Engine.Start(ctx)
}

// StopInstance 停止游戏实例
func (m *Manager) StopInstance(ctx context.Context, instance *GameInstance) error {
    now := time.Now()
    instance.StoppedAt = &now
    return instance.Engine.Stop(ctx)
}

// List 列出所有引擎
func (m *Manager) List() []string {
    m.mu.RLock()
    defer m.mu.RUnlock()

    ids := make([]string, 0, len(m.engines))
    for id := range m.engines {
        ids = append(ids, id)
    }
    return ids
}

func generateInstanceID() string {
    return fmt.Sprintf("game-%d", time.Now().UnixNano())
}
```

---

## 8. 使用示例

### 8.1 JavaScript 游戏示例

```javascript
// 游戏代码 (game.js)

// 游戏状态
let state = {
    players: [],
    started: false,
    round: 0
};

// 生命周期钩子
function onStart() {
    console.log("Game started!");
    state.started = true;
}

function onStop() {
    console.log("Game stopped!");
    state.started = false;
}

// 玩家加入
function onPlayerJoined(playerId) {
    console.log("Player joined:", playerId);
    state.players.push({
        id: playerId,
        score: 0
    });
}

// 玩家离开
function onPlayerLeft(playerId) {
    console.log("Player left:", playerId);
    state.players = state.players.filter(p => p.id !== playerId);
}

// 处理玩家动作
function handleAction(playerId, action) {
    const player = state.players.find(p => p.id === playerId);
    if (!player) return { error: "Player not found" };

    switch (action.type) {
        case "move":
            player.position = action.position;
            break;
        case "score":
            player.score += action.points;
            break;
    }

    return { success: true };
}

// 获取游戏状态
function getState() {
    return JSON.stringify(state);
}

// 设置游戏状态
function setState(newState) {
    state = JSON.parse(newState);
}

// 广播消息
function broadcast(message) {
    Game.broadcast(message);
}

// 结束游戏
function endGame(winnerId) {
    state.started = false;
    Game.endGame(winnerId);
}
```

### 8.2 Go 端使用示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/your-org/online-game/internal/engine"
)

func main() {
    // 创建JavaScript引擎
    jsEngine := engine.NewJSEngine(
        engine.WithMaxMemory(50*1024*1024),
        engine.WithTimeout(5000),
        engine.WithDebug(true),
    )

    // 初始化引擎
    ctx := context.Background()
    if err := jsEngine.Init(ctx); err != nil {
        panic(err)
    }
    defer jsEngine.Close()

    // 加载游戏代码
    code := []byte(`
        let state = { count: 0 };

        function onStart() {
            console.log("Game started!");
        }

        function getState() {
            return state;
        }

        function setState(newState) {
            state = newState;
        }

        function increment() {
            state.count++;
            return state.count;
        }
    `)

    if err := jsEngine.LoadGame(ctx, code); err != nil {
        panic(err)
    }

    // 启动游戏
    if err := jsEngine.Start(ctx); err != nil {
        panic(err)
    }

    // 调用游戏函数
    result, err := jsEngine.Call(ctx, "increment")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Result: %v\n", result)

    // 获取状态
    state, err := jsEngine.GetState(ctx)
    if err != nil {
        panic(err)
    }
    fmt.Printf("State: %v\n", state)

    // 停止游戏
    if err := jsEngine.Stop(ctx); err != nil {
        panic(err)
    }

    time.Sleep(time.Second)
}
```
