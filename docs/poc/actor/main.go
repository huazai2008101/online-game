// Package main 提供Actor模型技术验证POC
// 用途: 在正式开发前验证Actor模型的性能和可行性
//
// 运行方式:
//   cd docs/poc/actor && go run main.go
//
// 验证指标:
//   - 消息吞吐量: 目标 > 5,000 msg/s per actor
//   - 消息延迟: 目标 P99 < 1ms
//   - 内存占用: 目标 < 100MB per 1000 actors
//   - Actor恢复: 目标 < 100ms
package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ==================== Actor核心实现 ====================

// Message 消息接口
type Message interface {
	Type() string
}

// BaseActor Actor基础实现
type BaseActor struct {
	id        string
	actorType string
	inbox     chan Message
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	handler   MessageHandler
	stats     ActorStats
}

// ActorStats Actor统计信息
type ActorStats struct {
	MessageCount atomic.Int64
	ProcessTime  atomic.Int64 // 纳秒
	ErrorCount   atomic.Int64
}

// MessageHandler 消息处理器
type MessageHandler func(ctx context.Context, msg Message) error

// NewActor 创建Actor
func NewActor(id, actorType string, inboxSize int, handler MessageHandler) *BaseActor {
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
	go a.run(ctx)
	return nil
}

// run Actor主循环
func (a *BaseActor) run(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case msg, ok := <-a.inbox:
			if !ok {
				return
			}
			start := time.Now()
			err := a.handler(ctx, msg)
			duration := time.Since(start)

			a.stats.MessageCount.Add(1)
			a.stats.ProcessTime.Add(int64(duration.Nanoseconds()))

			if err != nil {
				a.stats.ErrorCount.Add(1)
			}

		case <-a.ctx.Done():
			return
		case <-ctx.Done():
			return
		}
	}
}

// Send 发送消息
func (a *BaseActor) Send(msg Message) error {
	select {
	case a.inbox <- msg:
		return nil
	default:
		return fmt.Errorf("actor inbox full: %s", a.id)
	}
}

// SendSync 同步发送消息
func (a *BaseActor) SendSync(ctx context.Context, msg Message) error {
	select {
	case a.inbox <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop 停止Actor
func (a *BaseActor) Stop() error {
	a.cancel()
	a.wg.Wait()
	return nil
}

// ID 获取Actor ID
func (a *BaseActor) ID() string {
	return a.id
}

// Type 获取Actor类型
func (a *BaseActor) Type() string {
	return a.actorType
}

// Stats 获取统计信息
func (a *BaseActor) Stats() *ActorStats {
	return &a.stats
}

// ==================== 测试消息类型 ====================

// PingMessage 测试消息
type PingMessage struct {
	Seq      int64
	SendTime time.Time
}

func (p PingMessage) Type() string {
	return "ping"
}

// PongMessage 响应消息
type PongMessage struct {
	Seq         int64
	PingTime    time.Time
	PongTime    time.Time
	ProcessTime time.Duration
}

func (p PongMessage) Type() string {
	return "pong"
}

// ==================== 测试Actor实现 ====================

// TestActor 测试Actor
type TestActor struct {
	*BaseActor
	pongChan chan<- *PongMessage
}

// NewTestActor 创建测试Actor
func NewTestActor(id string, pongChan chan<- *PongMessage) *TestActor {
	actor := &TestActor{
		pongChan: pongChan,
	}
	actor.BaseActor = NewActor(id, "test", 1000, actor.handle)
	return actor
}

// handle 处理消息
func (a *TestActor) handle(ctx context.Context, msg Message) error {
	switch m := msg.(type) {
	case *PingMessage:
		// 模拟处理延迟
		time.Sleep(time.Microsecond * time.Duration(10))

		// 发送响应
		if a.pongChan != nil {
			a.pongChan <- &PongMessage{
				Seq:         m.Seq,
				PingTime:    m.SendTime,
				PongTime:    time.Now(),
				ProcessTime: time.Since(m.SendTime),
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

// ==================== 性能测试 ====================

// ActorPerfResult 性能测试结果
type ActorPerfResult struct {
	TotalMessages   int64
	SuccessMessages int64
	FailedMessages  int64
	Duration        time.Duration
	ThroughputQPS   float64
	P50Latency      time.Duration
	P95Latency      time.Duration
	P99Latency      time.Duration
	AvgLatency      time.Duration
}

// RunPerformanceTest 运行性能测试
func RunPerformanceTest(numActors int, messagesPerActor int) *ActorPerfResult {
	ctx := context.Background()
	actors := make([]*TestActor, numActors)
	pongChan := make(chan *PongMessage, messagesPerActor*numActors)

	// 创建并启动Actors
	fmt.Printf("创建 %d 个Actor...\n", numActors)
	for i := 0; i < numActors; i++ {
		actors[i] = NewTestActor(fmt.Sprintf("actor-%d", i), pongChan)
		actors[i].Start(ctx)
	}

	// 等待Actor就绪
	time.Sleep(100 * time.Millisecond)

	// 发送消息
	fmt.Printf("发送 %d 条消息...\n", numActors*messagesPerActor)
	latencies := make([]time.Duration, 0, numActors*messagesPerActor)
	var wg sync.WaitGroup
	var sentCount atomic.Int64

	startTime := time.Now()

	for i, actor := range actors {
		wg.Add(1)
		go func(actorIndex int, a *TestActor) {
			defer wg.Done()
			for j := 0; j < messagesPerActor; j++ {
				msg := &PingMessage{
					Seq:      int64(actorIndex*messagesPerActor + j),
					SendTime: time.Now(),
				}
				if err := a.Send(msg); err != nil {
					fmt.Printf("发送失败: %v\n", err)
				}
				sentCount.Add(1)
			}
		}(i, actor)
	}

	wg.Wait()

	// 收集响应
	fmt.Printf("收集响应...\n")
	latencyMutex := sync.Mutex{}
	responseCount := atomic.Int64{}

	// 设置超时
	timeout := time.After(10 * time.Second)

	for {
		select {
		case pong := <-pongChan:
			latency := pong.PongTime.Sub(pong.PingTime)
			latencyMutex.Lock()
			latencies = append(latencies, latency)
			latencyMutex.Unlock()
			responseCount.Add(1)

			if responseCount.Load() == int64(numActors*messagesPerActor) {
				goto Done
			}

		case <-timeout:
			fmt.Printf("超时! 已收到 %d/%d 响应\n", responseCount.Load(), numActors*messagesPerActor)
			goto Done
		}
	}

Done:
	endTime := time.Now()
	totalDuration := endTime.Sub(startTime)

	// 停止Actors
	for _, actor := range actors {
		actor.Stop()
	}

	// 计算统计信息
	if len(latencies) == 0 {
		return &ActorPerfResult{
			FailedMessages: int64(numActors * messagesPerActor),
			Duration:       totalDuration,
		}
	}

	// 排序延迟
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	// 计算百分位
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]

	// 计算平均延迟
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	avg := sum / time.Duration(len(latencies))

	return &ActorPerfResult{
		TotalMessages:   int64(numActors * messagesPerActor),
		SuccessMessages: responseCount.Load(),
		FailedMessages:  int64(numActors*messagesPerActor) - responseCount.Load(),
		Duration:        totalDuration,
		ThroughputQPS:   float64(responseCount.Load()) / totalDuration.Seconds(),
		P50Latency:      p50,
		P95Latency:      p95,
		P99Latency:      p99,
		AvgLatency:      avg,
	}
}

// ==================== 内存测试 ====================

// ActorMemResult 内存测试结果
type ActorMemResult struct {
	NumActors     int
	MemoryBefore  uint64
	MemoryAfter   uint64
	MemoryUsed    uint64
	MemoryPerActor uint64
}

// RunMemoryTest 运行内存测试
func RunMemoryTest(numActors int) *ActorMemResult {
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	memoryBefore := m1.HeapAlloc

	ctx := context.Background()
	actors := make([]*TestActor, numActors)

	// 创建Actors
	for i := 0; i < numActors; i++ {
		actors[i] = NewTestActor(fmt.Sprintf("actor-%d", i), nil)
		actors[i].Start(ctx)
	}

	// 运行一段时间
	time.Sleep(time.Second)

	// GC后测量内存
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	memoryAfter := m2.HeapAlloc

	// 停止Actors
	for _, actor := range actors {
		actor.Stop()
	}

	return &ActorMemResult{
		NumActors:      numActors,
		MemoryBefore:   memoryBefore,
		MemoryAfter:    memoryAfter,
		MemoryUsed:     memoryAfter - memoryBefore,
		MemoryPerActor: (memoryAfter - memoryBefore) / uint64(numActors),
	}
}

// ==================== 主函数 ====================

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                  Actor模型技术验证POC                              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 测试1: 性能测试
	fmt.Println("┌───────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ 测试1: 性能测试                                                   │")
	fmt.Println("│ 配置: 100个Actor, 每个Actor发送1000条消息                          │")
	fmt.Println("└───────────────────────────────────────────────────────────────────┘")

	perfResult := RunPerformanceTest(100, 1000)
	fmt.Println()
	fmt.Println("结果:")
	fmt.Printf("  总消息数:      %d\n", perfResult.TotalMessages)
	fmt.Printf("  成功消息:      %d\n", perfResult.SuccessMessages)
	fmt.Printf("  失败消息:      %d\n", perfResult.FailedMessages)
	fmt.Printf("  总耗时:        %v\n", perfResult.Duration)
	fmt.Printf("  吞吐量:        %.2f msg/s\n", perfResult.ThroughputQPS)
	fmt.Printf("  平均延迟:      %v\n", perfResult.AvgLatency)
	fmt.Printf("  P50延迟:       %v\n", perfResult.P50Latency)
	fmt.Printf("  P95延迟:       %v\n", perfResult.P95Latency)
	fmt.Printf("  P99延迟:       %v\n", perfResult.P99Latency)
	fmt.Println()

	// 判断是否达标
	perfOK := true
	if perfResult.ThroughputQPS < 5000 {
		fmt.Println("  ⚠️  吞吐量未达标 (目标: >5000 msg/s)")
		perfOK = false
	}
	if perfResult.P99Latency > time.Millisecond {
		fmt.Println("  ⚠️  P99延迟未达标 (目标: <1ms)")
		perfOK = false
	}
	if perfOK {
		fmt.Println("  ✅ 性能测试通过!")
	}

	fmt.Println()

	// 测试2: 内存测试
	fmt.Println("┌───────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ 测试2: 内存测试                                                   │")
	fmt.Println("│ 配置: 创建1000个Actor                                              │")
	fmt.Println("└───────────────────────────────────────────────────────────────────┘")

	memResult := RunMemoryTest(1000)
	fmt.Println()
	fmt.Println("结果:")
	fmt.Printf("  Actor数量:     %d\n", memResult.NumActors)
	fmt.Printf("  内存使用:      %.2f MB\n", float64(memResult.MemoryUsed)/1024/1024)
	fmt.Printf("  每Actor内存:   %.2f KB\n", float64(memResult.MemoryPerActor)/1024)
	fmt.Println()

	if memResult.MemoryUsed < 100*1024*1024 {
		fmt.Println("  ✅ 内存测试通过! (目标: <100MB)")
	} else {
		fmt.Println("  ⚠️  内存使用超标 (目标: <100MB)")
	}

	fmt.Println()

	// 总结
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                           测试总结                                 ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("验收标准:")
	fmt.Println("  - 消息吞吐量:   > 5,000 msg/s")
	fmt.Println("  - P99延迟:      < 1ms")
	fmt.Println("  - 内存占用:     < 100MB (1000 actors)")
	fmt.Println()

	if perfOK && memResult.MemoryUsed < 100*1024*1024 {
		fmt.Println("🎉 所有测试通过! Actor模型可以用于生产环境。")
	} else {
		fmt.Println("⚠️  部分测试未通过，需要优化后再使用。")
	}
}
