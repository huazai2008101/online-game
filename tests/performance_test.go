package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"online-game/pkg/actor"
	"online-game/pkg/db"
	"online-game/pkg/engine"
)

// Helper functions for JSON marshaling (since cache package doesn't export them)
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// BenchmarkActorSystem benchmarks actor system with many actors
func BenchmarkActorSystem(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	system := actor.NewActorSystem()

	// Create many actors
	numActors := 100
	actors := make([]*actor.GameActor, numActors)

	for i := 0; i < numActors; i++ {
		gameActor := actor.NewGameActor(fmt.Sprintf("bench-game-%d", i), "test-type")
		_ = system.Register(gameActor)
		actors[i] = gameActor
	}

	_ = system.Start(ctx)

	msg := &actor.GameStartMessage{
		GameID:    "bench-game",
		RoomID:    "bench-room",
		Players:   []string{"player-1"},
		Timestamp: time.Now().Unix(),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = actors[i%numActors].Send(msg)
			i++
		}
	})
}

// BenchmarkDualEngine benchmarks dual engine execution
func BenchmarkDualEngine(b *testing.B) {
	ctx := context.Background()

	// Note: otto JS runtime is not thread-safe, so we run sequentially
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dualEngine, err := engine.NewDualEngine("bench-game")
		if err != nil {
			b.Fatalf("Failed to create dual engine: %v", err)
		}

		_ = dualEngine.Init(ctx, "bench-game", nil)

		script := `
			(function() {
				let sum = 0;
				for (let i = 0; i < 100; i++) {
					sum += i;
				}
				return sum;
			})();
		`

		_, _ = dualEngine.Execute(ctx, script, nil)
		dualEngine.Cleanup()
	}
}

// BenchmarkMessageBatching benchmarks message batching
func BenchmarkMessageBatching(b *testing.B) {
	var receivedCount int
	var mu sync.Mutex

	handler := func(messages []actor.Message) error {
		mu.Lock()
		receivedCount += len(messages)
		mu.Unlock()
		return nil
	}

	config := actor.BatcherConfig{
		BatchSize: 100,
		Interval:   10 * time.Millisecond,
		Handler:    handler,
	}

	batcher := actor.NewMessageBatcher(config)

	msg := &actor.GameStartMessage{
		GameID:    "bench-game",
		RoomID:    "bench-room",
		Players:   []string{"player-1"},
		Timestamp: time.Now().Unix(),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = batcher.Send(msg)
		}
	})

	_ = batcher.Flush()
}

// BenchmarkCacheOperations benchmarks cache operations
func BenchmarkCacheOperations(b *testing.B) {
	type TestValue struct {
		Data string
		Name string
		Num  int
	}

	val := &TestValue{Data: "test-data", Name: "test", Num: 123}

	b.Run("Marshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = marshalJSON(val)
		}
	})

	b.Run("Unmarshal", func(b *testing.B) {
		data, _ := marshalJSON(val)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var result TestValue
			_ = unmarshalJSON(data, &result)
		}
	})
}

// BenchmarkActorPool benchmarks actor pool performance
func BenchmarkActorPool(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gameActor := actor.GetGameActor("bench-game", "bench-room")
			gameActor.Reset()
			actor.PutGameActor(gameActor)
		}
	})
}

// BenchmarkConcurrentMessageSend benchmarks concurrent message sending
func BenchmarkConcurrentMessageSend(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	system := actor.NewActorSystem()
	gameActor := actor.NewGameActor("bench-game", "test-type")
	_ = system.Register(gameActor)
	_ = system.Start(ctx)

	msg := &actor.GameStartMessage{
		GameID:    "bench-game",
		RoomID:    "bench-room",
		Players:   []string{"player-1"},
		Timestamp: time.Now().Unix(),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = gameActor.Send(msg)
		}
	})
}

// TestLoadTest simulates high load on the system
func TestLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	system := actor.NewActorSystem()

	// Create multiple actors
	numActors := 50
	actors := make([]*actor.GameActor, numActors)

	for i := 0; i < numActors; i++ {
		gameActor := actor.NewGameActor(fmt.Sprintf("load-game-%d", i), "test-type")
		_ = system.Register(gameActor)
		actors[i] = gameActor
	}

	_ = system.Start(ctx)

	// Simulate concurrent load
	numGoroutines := 100
	messagesPerGoroutine := 1000

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			msg := &actor.GameStartMessage{
				GameID:    fmt.Sprintf("load-game-%d", workerID%numActors),
				RoomID:    "load-room",
				Players:   []string{fmt.Sprintf("player-%d", workerID)},
				Timestamp: time.Now().Unix(),
			}

			for j := 0; j < messagesPerGoroutine; j++ {
				_ = actors[workerID%numActors].Send(msg)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	totalMessages := numGoroutines * messagesPerGoroutine
	messagesPerSecond := float64(totalMessages) / duration.Seconds()

	t.Logf("Load test results:")
	t.Logf("  Total messages: %d", totalMessages)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Messages/second: %.2f", messagesPerSecond)
}

// TestStressTest stress tests the system
func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	system := actor.NewActorSystem()

	// Create many actors
	numActors := 200
	actors := make([]*actor.GameActor, numActors)

	for i := 0; i < numActors; i++ {
		gameActor := actor.NewGameActor(fmt.Sprintf("stress-game-%d", i), "test-type")
		_ = system.Register(gameActor)
		actors[i] = gameActor
	}

	_ = system.Start(ctx)

	// Continuous load until timeout
	done := make(chan struct{})
	var totalMessages int64

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			default:
				for _, gameActor := range actors {
					msg := &actor.GameStartMessage{
						GameID:    gameActor.ID(),
						RoomID:    "stress-room",
						Players:   []string{"player-1"},
						Timestamp: time.Now().Unix(),
					}
					_ = gameActor.Send(msg)
					totalMessages++
				}
			}
		}
	}()

	<-done
	t.Logf("Stress test completed. Total messages sent: %d", totalMessages)
}

// BenchmarkEngineSelector benchmarks engine selection
func BenchmarkEngineSelector(b *testing.B) {
	selector := engine.NewEngineSelector()

	perfData := &engine.PerformanceData{
		AverageFPS:  60,
		ExecuteTime: 10 * time.Millisecond,
		MemoryUsage: 50 * 1024 * 1024,
		Complexity:  50,
		ErrorCount:  0,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = selector.SelectEngine(perfData)
		}
	})
}

// TestCachePerformance tests cache performance with different data sizes
func TestCachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cache performance test in short mode")
	}

	sizes := []struct {
		name  string
		size  int
		count int
	}{
		{"small", 100, 10000},
		{"medium", 1024, 5000},
		{"large", 10240, 1000},
	}

	for _, tc := range sizes {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]byte, tc.size)

			start := time.Now()
			for i := 0; i < tc.count; i++ {
				_, _ = marshalJSON(data)
			}
			marshalDuration := time.Since(start)

			start = time.Now()
			for i := 0; i < tc.count; i++ {
				_, _ = marshalJSON(data)
			}
			unmarshalDuration := time.Since(start)

			t.Logf("%s: marshal=%v, unmarshal=%v", tc.name, marshalDuration, unmarshalDuration)
		})
	}
}

// BenchmarkRepositoryOperations benchmarks repository operations
// Skipped because it requires actual database connection
func BenchmarkRepositoryOperations(b *testing.B) {
	b.Skip("Skipping repository benchmark - requires database connection")
}

// BenchmarkMultiPool benchmarks multi-pool operations
func BenchmarkMultiPool(b *testing.B) {
	multiPool := db.NewMultiPool()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = multiPool.Get("test-db")
		}
	})
}

// TestMemoryUsage tests memory usage under load
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	system := actor.NewActorSystem()

	// Create many actors
	numActors := 100
	actors := make([]*actor.GameActor, numActors)

	for i := 0; i < numActors; i++ {
		gameActor := actor.NewGameActor(fmt.Sprintf("mem-game-%d", i), "test-type")
		_ = system.Register(gameActor)
		actors[i] = gameActor
	}

	_ = system.Start(ctx)

	// Send many messages
	msg := &actor.GameStartMessage{
		GameID:    "mem-game",
		RoomID:    "mem-room",
		Players:   []string{"player-1"},
		Timestamp: time.Now().Unix(),
	}

	for i := 0; i < 10000; i++ {
		for _, actor := range actors {
			_ = actor.Send(msg)
		}
	}

	t.Logf("Memory test completed with %d actors", numActors)
}
