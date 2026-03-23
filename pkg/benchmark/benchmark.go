// Package benchmark provides performance testing utilities for the game platform
package benchmark

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"online-game/pkg/actor"
	"online-game/pkg/engine"
)

// BenchmarkResult represents the result of a benchmark
type BenchmarkResult struct {
	Name           string
	Iterations      int64
	Duration       time.Duration
	AvgLatency     time.Duration
	P50Latency     time.Duration
	P95Latency     time.Duration
	P99Latency     time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	Throughput     float64 // Operations per second
	ErrorCount     int64
	SuccessCount   int64
}

// String returns a string representation of the benchmark result
func (r *BenchmarkResult) String() string {
	return fmt.Sprintf(
		"%s:\n"+
			"  Iterations: %d\n"+
			"  Duration: %v\n"+
			"  Avg Latency: %v\n"+
			"  P50: %v, P95: %v, P99: %v\n"+
			"  Min: %v, Max: %v\n"+
			"  Throughput: %.2f ops/sec\n"+
			"  Errors: %d (%.2f%%)\n",
		r.Name,
		r.Iterations,
		r.Duration,
		r.AvgLatency,
		r.P50Latency, r.P95Latency, r.P99Latency,
		r.MinLatency, r.MaxLatency,
		r.Throughput,
		r.ErrorCount,
		float64(r.ErrorCount)/float64(r.Iterations)*100,
	)
}

// BenchmarkConfig configures a benchmark
type BenchmarkConfig struct {
	Name        string
	Iterations  int64
	Duration    time.Duration
	Concurrency int
	Warmup      int64 // Number of warmup iterations
}

// Benchmark runs a benchmark function
func Benchmark(cfg BenchmarkConfig, fn func() error) *BenchmarkResult {
	// Warmup
	for i := int64(0); i < cfg.Warmup; i++ {
		_ = fn()
	}

	latencies := make([]time.Duration, 0, cfg.Iterations)
	var errorCount atomic.Int64
	var successCount atomic.Int64

	start := time.Now()

	// Run benchmark
	if cfg.Concurrency > 1 {
		// Concurrent benchmark
		var wg sync.WaitGroup
		perGoroutine := cfg.Iterations / int64(cfg.Concurrency)

		for i := 0; i < cfg.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := int64(0); j < perGoroutine; j++ {
					opStart := time.Now()
					err := fn()
					opDuration := time.Since(opStart)

					if err != nil {
						errorCount.Add(1)
					} else {
						successCount.Add(1)
						latencies = append(latencies, opDuration)
					}
				}
			}()
		}
		wg.Wait()
	} else {
		// Sequential benchmark
		for i := int64(0); i < cfg.Iterations; i++ {
			opStart := time.Now()
			err := fn()
			opDuration := time.Since(opStart)

			if err != nil {
				errorCount.Add(1)
			} else {
				successCount.Add(1)
				latencies = append(latencies, opDuration)
			}
		}
	}

	duration := time.Since(start)

	// Calculate statistics
	return calculateStats(cfg.Name, duration, latencies, errorCount.Load(), successCount.Load())
}

// calculateStats calculates benchmark statistics
func calculateStats(name string, duration time.Duration, latencies []time.Duration, errors, success int64) *BenchmarkResult {
	if len(latencies) == 0 {
		return &BenchmarkResult{
			Name:         name,
			Duration:     duration,
			ErrorCount:   errors,
			SuccessCount: success,
		}
	}

	// Sort latencies for percentile calculation
	// Simple bubble sort for small datasets
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	var sum time.Duration
	min := latencies[0]
	max := latencies[0]

	for _, l := range latencies {
		sum += l
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
	}

	avg := sum / time.Duration(len(latencies))

	return &BenchmarkResult{
		Name:         name,
		Iterations:   success + errors,
		Duration:     duration,
		AvgLatency:   avg,
		P50Latency:   latencies[len(latencies)*50/100],
		P95Latency:   latencies[len(latencies)*95/100],
		P99Latency:   latencies[len(latencies)*99/100],
		MinLatency:   min,
		MaxLatency:   max,
		Throughput:   float64(len(latencies)) / duration.Seconds(),
		ErrorCount:   errors,
		SuccessCount: success,
	}
}

// ==================== Actor Benchmarks ====================

// BenchmarkActorSend benchmarks actor message sending
func BenchmarkActorSend(iterations int64, concurrency int) *BenchmarkResult {
	cfg := BenchmarkConfig{
		Name:        "Actor Send",
		Iterations:  iterations,
		Concurrency: concurrency,
		Warmup:      100,
	}

	// Create actor system
	system := actor.NewActorSystem()
	testActor := actor.NewActor("test", "benchmark", 1000, func(ctx context.Context, msg actor.Message) error {
		return nil
	})
	_ = system.Register(testActor)
	_ = testActor.Start(context.Background())

	result := Benchmark(cfg, func() error {
		return testActor.Send(&actor.PingMessage{Timestamp: time.Now().Unix()})
	})

	_ = testActor.Stop()
	_ = system.Stop()

	return result
}

// BenchmarkActorBatchSend benchmarks actor batch message sending
func BenchmarkActorBatchSend(iterations int64, batchSize int) *BenchmarkResult {
	cfg := BenchmarkConfig{
		Name:       "Actor Batch Send",
		Iterations: iterations / int64(batchSize),
		Warmup:     10,
	}

	// Create actor system
	system := actor.NewActorSystem()
	testActor := actor.NewActor("test", "benchmark", 10000, func(ctx context.Context, msg actor.Message) error {
		return nil
	})
	_ = system.Register(testActor)
	_ = testActor.Start(context.Background())

	batcher := actor.NewMessageBatcher(actor.BatcherConfig{
		BatchSize: batchSize,
		Interval:  10 * time.Millisecond,
		Actor:     testActor,
	})

	result := Benchmark(cfg, func() error {
		messages := make([]actor.Message, batchSize)
		for i := 0; i < batchSize; i++ {
			messages[i] = &actor.PingMessage{Timestamp: time.Now().Unix()}
		}
		for _, msg := range messages {
			_ = batcher.Send(msg)
		}
		return nil
	})

	_ = batcher.Stop()
	_ = testActor.Stop()
	_ = system.Stop()

	return result
}

// BenchmarkActorThroughput benchmarks actor message throughput
func BenchmarkActorThroughput(duration time.Duration) *BenchmarkResult {
	var count atomic.Int64
	system := actor.NewActorSystem()
	testActor := actor.NewActor("test", "benchmark", 10000, func(ctx context.Context, msg actor.Message) error {
		count.Add(1)
		return nil
	})
	_ = system.Register(testActor)
	_ = testActor.Start(context.Background())

	start := time.Now()
	deadline := start.Add(duration)

	for time.Now().Before(deadline) {
		_ = testActor.Send(&actor.PingMessage{Timestamp: time.Now().Unix()})
	}

	_ = testActor.Stop()
	_ = system.Stop()

	elapsed := time.Since(start)

	return &BenchmarkResult{
		Name:       "Actor Throughput",
		Iterations: count.Load(),
		Duration:   elapsed,
		Throughput: float64(count.Load()) / elapsed.Seconds(),
	}
}

// ==================== Engine Benchmarks ====================

// BenchmarkJSEngine benchmarks JavaScript engine execution
func BenchmarkJSEngine(iterations int64) *BenchmarkResult {
	cfg := BenchmarkConfig{
		Name:       "JS Engine",
		Iterations: iterations,
		Warmup:     100,
	}

	jsEngine := engine.NewJSEngine()
	_ = jsEngine.Init(context.Background(), "test", nil)

	// Simple JS function
	script := `
		function add(a, b) {
			return a + b;
		}
	`
	_ = jsEngine.LoadScript(engine.EngineJS, []byte(script))

	result := Benchmark(cfg, func() error {
		_, err := jsEngine.Execute(context.Background(), "add", []interface{}{1, 2})
		return err
	})

	_ = jsEngine.Cleanup()

	return result
}

// BenchmarkDualEngine benchmarks dual engine execution
func BenchmarkDualEngine(iterations int64) *BenchmarkResult {
	cfg := BenchmarkConfig{
		Name:       "Dual Engine",
		Iterations: iterations,
		Warmup:     100,
	}

	dualEngine, err := engine.NewDualEngine("test")
	if err != nil {
		return &BenchmarkResult{Name: cfg.Name, ErrorCount: 1}
	}
	_ = dualEngine.Init(context.Background(), "test", nil)

	// Simple JS function
	script := `
		function calculate(value) {
			return value * 2;
		}
	`
	_ = dualEngine.LoadScript(engine.EngineJS, []byte(script))

	result := Benchmark(cfg, func() error {
		_, err := dualEngine.Execute(context.Background(), "calculate", 21)
		return err
	})

	_ = dualEngine.Cleanup()

	return result
}

// ==================== Comparison Benchmarks ====================

// RunAllBenchmarks runs all benchmarks and returns results
func RunAllBenchmarks(iterations int64) []*BenchmarkResult {
	results := make([]*BenchmarkResult, 0)

	fmt.Println("Running benchmarks...")

	// Actor benchmarks
	fmt.Println("Benchmarking Actor Send (sequential)...")
	results = append(results, BenchmarkActorSend(iterations, 1))

	fmt.Println("Benchmarking Actor Send (concurrent)...")
	results = append(results, BenchmarkActorSend(iterations, 10))

	fmt.Println("Benchmarking Actor Throughput...")
	results = append(results, BenchmarkActorThroughput(1*time.Second))

	// Engine benchmarks
	fmt.Println("Benchmarking JS Engine...")
	results = append(results, BenchmarkJSEngine(iterations))

	fmt.Println("Benchmarking Dual Engine...")
	results = append(results, BenchmarkDualEngine(iterations))

	return results
}

// PrintResults prints benchmark results
func PrintResults(results []*BenchmarkResult) {
	fmt.Println("\n==================== Benchmark Results ====================")
	for _, result := range results {
		fmt.Println(result.String())
	}
	fmt.Println("========================================================")
}

// CompareResults compares two benchmark results
func CompareResults(a, b *BenchmarkResult) string {
	throughputImprovement := ((b.Throughput - a.Throughput) / a.Throughput) * 100
	latencyImprovement := float64((a.AvgLatency - b.AvgLatency)) / float64(a.AvgLatency) * 100

	return fmt.Sprintf(
		"%s vs %s:\n"+
			"  Throughput: %.2f%% %s\n"+
			"  Latency: %.2f%% %s",
		a.Name, b.Name,
		abs(throughputImprovement), sign(throughputImprovement),
		abs(latencyImprovement), sign(latencyImprovement),
	)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x float64) string {
	if x > 0 {
		return "improvement"
	}
	if x < 0 {
		return "regression"
	}
	return "no change"
}
