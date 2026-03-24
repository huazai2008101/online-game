// Package main 提供游戏引擎技术验证POC
// 用途: 在正式开发前验证JavaScript引擎的性能和可行性
//
// 安装依赖:
//   go get github.com/robertkrimen/otto
//
// 运行方式:
//   cd docs/poc/engine && go run main.go
//
// 验证指标:
//   - 执行性能: 目标 FPS > 30
//   - 内存占用: 目标 < 50MB
//   - 启动时间: 目标 < 100ms
package main

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robertkrimen/otto"
)

// ==================== 引擎接口 ====================

// GameEngine 游戏引擎接口
type GameEngine interface {
	// Init 初始化引擎
	Init(ctx context.Context, gameID string, state interface{}) error

	// LoadScript 加载游戏脚本
	LoadScript(script string) error

	// Execute 执行游戏逻辑
	Execute(ctx context.Context, method string, args interface{}) (interface{}, error)

	// Stats 获取性能统计
	Stats() *EngineStats
}

// EngineStats 引擎统计信息
type EngineStats struct {
	ExecuteCount   atomic.Int64
	ExecuteTime    atomic.Int64 // 纳秒
	ErrorCount     atomic.Int64
	CacheHitCount  atomic.Int64
	CacheMissCount atomic.Int64
}

// ==================== JavaScript引擎实现 ====================

// JSEngine JavaScript引擎实现
type JSEngine struct {
	vm    *otto.Otto
	stats EngineStats
	mu    sync.RWMutex
}

// NewJSEngine 创建JavaScript引擎
func NewJSEngine() *JSEngine {
	vm := otto.New()

	// 设置全局函数
	vm.Set("log", func(call otto.FunctionCall) otto.Value {
		fmt.Printf("[JS] %s\n", call.Argument(0).String())
		return otto.UndefinedValue()
	})

	// 设置数学函数
	vm.Set("rand", func(call otto.FunctionCall) otto.Value {
		val, _ := vm.ToValue(rand.Float64())
		return val
	})

	return &JSEngine{
		vm: vm,
	}
}

// Init 初始化引擎
func (e *JSEngine) Init(ctx context.Context, gameID string, state interface{}) error {
	return nil
}

// LoadScript 加载游戏脚本
func (e *JSEngine) LoadScript(script string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 预编译脚本
	_, err := e.vm.Run(script)
	if err != nil {
		return fmt.Errorf("script load failed: %w", err)
	}

	return nil
}

// Execute 执行游戏逻辑
func (e *JSEngine) Execute(ctx context.Context, method string, args interface{}) (interface{}, error) {
	start := time.Now()

	// 执行方法
	e.mu.RLock()
	vm := e.vm
	e.mu.RUnlock()

	result, err := vm.Run(method)
	if err != nil {
		e.stats.ErrorCount.Add(1)
		e.stats.ExecuteCount.Add(1)
		return nil, err
	}

	e.stats.CacheMissCount.Add(1)
	e.stats.ExecuteCount.Add(1)
	e.stats.ExecuteTime.Add(time.Since(start).Nanoseconds())

	return result.Export(), nil
}

// Stats 获取统计信息
func (e *JSEngine) Stats() *EngineStats {
	return &e.stats
}

// ==================== 测试脚本 ====================

// 复杂计算脚本 - 用于性能测试
const complexCalcScript = `
// 模拟游戏逻辑
function simulateGameTick() {
	var players = [];
	for (var i = 0; i < 6; i++) {
		players.push({
			id: i,
			chips: 1000 + Math.floor(rand() * 9000),
			bet: Math.floor(rand() * 100),
			cards: [Math.floor(rand() * 52), Math.floor(rand() * 52)]
		});
	}

	var pot = 0;
	for (var i = 0; i < players.length; i++) {
		pot += players[i].bet;
	}

	var winner = Math.floor(rand() * players.length);
	players[winner].chips += pot;

	return {
		players: players,
		pot: pot,
		winner: winner
	};
}
`

// ==================== 性能测试 ====================

// EnginePerfResult 性能测试结果
type EnginePerfResult struct {
	ExecuteCount  int64
	TotalDuration time.Duration
	AvgDuration   time.Duration
	P50Duration   time.Duration
	P95Duration   time.Duration
	P99Duration   time.Duration
	ThroughputFPS float64
	ErrorCount    int64
}

// RunEnginePerformanceTest 运行引擎性能测试
func RunEnginePerformanceTest(iterations int) *EnginePerfResult {
	engine := NewJSEngine()

	// 加载脚本
	if err := engine.LoadScript(complexCalcScript); err != nil {
		fmt.Printf("加载脚本失败: %v\n", err)
		return nil
	}

	// 预热
	for i := 0; i < 100; i++ {
		engine.Execute(context.Background(), "simulateGameTick()", nil)
	}

	// 执行测试
	durations := make([]time.Duration, iterations)
	var errorCount int64

	startTime := time.Now()

	for i := 0; i < iterations; i++ {
		iterStart := time.Now()
		_, err := engine.Execute(context.Background(), "simulateGameTick()", nil)
		durations[i] = time.Since(iterStart)

		if err != nil {
			errorCount++
		}
	}

	totalDuration := time.Since(startTime)

	// 排序延迟
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}

	p50 := durations[len(durations)*50/100]
	p95 := durations[len(durations)*95/100]
	p99 := durations[len(durations)*99/100]

	// 计算平均延迟
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	avg := sum / time.Duration(len(durations))

	return &EnginePerfResult{
		ExecuteCount:  int64(iterations),
		TotalDuration: totalDuration,
		AvgDuration:   avg,
		P50Duration:   p50,
		P95Duration:   p95,
		P99Duration:   p99,
		ThroughputFPS: float64(iterations) / totalDuration.Seconds(),
		ErrorCount:    errorCount,
	}
}

// ==================== 内存测试 ====================

// EngineMemResult 内存测试结果
type EngineMemResult struct {
	Iterations     int
	MemoryBefore   uint64
	MemoryAfter    uint64
	MemoryUsed     uint64
	MemoryPerIter  uint64
}

// RunEngineMemoryTest 运行引擎内存测试
func RunEngineMemoryTest(iterations int) *EngineMemResult {
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	memoryBefore := m1.HeapAlloc

	engine := NewJSEngine()
	engine.LoadScript(complexCalcScript)

	// 执行多次
	for i := 0; i < iterations; i++ {
		engine.Execute(context.Background(), "simulateGameTick()", nil)
	}

	// GC后测量
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	memoryAfter := m2.HeapAlloc

	return &EngineMemResult{
		Iterations:     iterations,
		MemoryBefore:   memoryBefore,
		MemoryAfter:    memoryAfter,
		MemoryUsed:     memoryAfter - memoryBefore,
		MemoryPerIter:  (memoryAfter - memoryBefore) / uint64(iterations),
	}
}

// ==================== 并发测试 ====================

// EngineConcurResult 并发测试结果
type EngineConcurResult struct {
	NumEngines    int
	Iterations    int
	TotalDuration time.Duration
	ThroughputQPS float64
	SuccessCount  int64
	FailureCount  int64
}

// RunEngineConcurrencyTest 运行引擎并发测试
func RunEngineConcurrencyTest(numEngines, iterations int) *EngineConcurResult {
	ctx := context.Background()
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var failureCount atomic.Int64

	startTime := time.Now()

	for i := 0; i < numEngines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			engine := NewJSEngine()
			if err := engine.LoadScript(complexCalcScript); err != nil {
				failureCount.Add(int64(iterations))
				return
			}

			for j := 0; j < iterations; j++ {
				_, err := engine.Execute(ctx, "simulateGameTick()", nil)
				if err != nil {
					failureCount.Add(1)
				} else {
					successCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	return &EngineConcurResult{
		NumEngines:    numEngines,
		Iterations:    iterations,
		TotalDuration: totalDuration,
		ThroughputQPS: float64(successCount.Load()) / totalDuration.Seconds(),
		SuccessCount:  successCount.Load(),
		FailureCount:  failureCount.Load(),
	}
}

// ==================== 主函数 ====================

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                  JavaScript引擎技术验证POC                        ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 测试1: 性能测试
	fmt.Println("┌───────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ 测试1: 单引擎性能测试                                              │")
	fmt.Println("│ 配置: 执行10000次游戏tick                                          │")
	fmt.Println("└───────────────────────────────────────────────────────────────────┘")

	perfResult := RunEnginePerformanceTest(10000)
	if perfResult != nil {
		fmt.Println()
		fmt.Println("结果:")
		fmt.Printf("  执行次数:      %d\n", perfResult.ExecuteCount)
		fmt.Printf("  总耗时:        %v\n", perfResult.TotalDuration)
		fmt.Printf("  平均延迟:      %v\n", perfResult.AvgDuration)
		fmt.Printf("  P50延迟:       %v\n", perfResult.P50Duration)
		fmt.Printf("  P95延迟:       %v\n", perfResult.P95Duration)
		fmt.Printf("  P99延迟:       %v\n", perfResult.P99Duration)
		fmt.Printf("  吞吐量:        %.2f ops/s\n", perfResult.ThroughputFPS)
		fmt.Printf("  错误数:        %d\n", perfResult.ErrorCount)
		fmt.Println()

		// 计算FPS (假设每帧执行一次游戏逻辑)
		fps := 1000000000.0 / float64(perfResult.P99Duration.Nanoseconds())
		fmt.Printf("  预估FPS (P99): %.2f\n", fps)

		if fps >= 30 {
			fmt.Println("  ✅ 性能测试通过! (目标: FPS >= 30)")
		} else {
			fmt.Println("  ⚠️  性能未达标 (目标: FPS >= 30)")
		}
	}

	fmt.Println()

	// 测试2: 内存测试
	fmt.Println("┌───────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ 测试2: 内存测试                                                   │")
	fmt.Println("│ 配置: 执行10000次操作                                              │")
	fmt.Println("└───────────────────────────────────────────────────────────────────┘")

	memResult := RunEngineMemoryTest(10000)
	fmt.Println()
	fmt.Println("结果:")
	fmt.Printf("  执行次数:      %d\n", memResult.Iterations)
	fmt.Printf("  内存使用:      %.2f MB\n", float64(memResult.MemoryUsed)/1024/1024)
	fmt.Printf("  每次操作内存:  %.2f KB\n", float64(memResult.MemoryPerIter)/1024)
	fmt.Println()

	if memResult.MemoryUsed < 50*1024*1024 {
		fmt.Println("  ✅ 内存测试通过! (目标: <50MB)")
	} else {
		fmt.Println("  ⚠️  内存使用超标 (目标: <50MB)")
	}

	fmt.Println()

	// 测试3: 并发测试
	fmt.Println("┌───────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ 测试3: 并发性能测试                                                │")
	fmt.Println("│ 配置: 10个引擎并发,每个执行1000次                                  │")
	fmt.Println("└───────────────────────────────────────────────────────────────────┘")

	concurResult := RunEngineConcurrencyTest(10, 1000)
	fmt.Println()
	fmt.Println("结果:")
	fmt.Printf("  引擎数量:      %d\n", concurResult.NumEngines)
	fmt.Printf("  每引擎执行:    %d\n", concurResult.Iterations)
	fmt.Printf("  总执行数:      %d\n", concurResult.SuccessCount+concurResult.FailureCount)
	fmt.Printf("  成功数:        %d\n", concurResult.SuccessCount)
	fmt.Printf("  失败数:        %d\n", concurResult.FailureCount)
	fmt.Printf("  总耗时:        %v\n", concurResult.TotalDuration)
	fmt.Printf("  总吞吐量:      %.2f ops/s\n", concurResult.ThroughputQPS)
	fmt.Println()

	if concurResult.FailureCount == 0 {
		fmt.Println("  ✅ 并发测试通过! 无错误")
	} else {
		fmt.Printf("  ⚠️  并发测试有 %d 个错误\n", concurResult.FailureCount)
	}

	fmt.Println()

	// 总结
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                           测试总结                                 ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("验收标准:")
	fmt.Println("  - 执行性能:     FPS >= 30")
	fmt.Println("  - 内存占用:     < 50MB")
	fmt.Println("  - 并发稳定性:   无错误")
	fmt.Println()

	if perfResult != nil {
		fps := 1000000000.0 / float64(perfResult.P99Duration.Nanoseconds())
		perfOK := fps >= 30
		memOK := memResult.MemoryUsed < 50*1024*1024
		concurOK := concurResult.FailureCount == 0

		if perfOK && memOK && concurOK {
			fmt.Println("🎉 所有测试通过! JavaScript引擎可以用于生产环境。")
		} else {
			fmt.Println("⚠️  部分测试未通过，需要优化后再使用。")
			if !perfOK {
				fmt.Println("   - 性能需要优化")
			}
			if !memOK {
				fmt.Println("   - 内存使用需要优化")
			}
			if !concurOK {
				fmt.Println("   - 并发稳定性需要改进")
			}
		}
	}
}
