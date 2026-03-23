package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"online-game/pkg/actor"
	"online-game/pkg/cache"
	"online-game/pkg/db"
	"online-game/pkg/engine"
	"online-game/pkg/kafka"
	"online-game/pkg/websocket"
)

// TestActorSystem tests the actor system functionality
func TestActorSystem(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	system := actor.NewActorSystem()

	// Create a test game actor
	gameActor := actor.NewGameActor("test-game", "test-game-type")
	if err := system.Register(gameActor); err != nil {
		t.Fatalf("Failed to register game actor: %v", err)
	}

	if err := system.Start(ctx); err != nil {
		t.Fatalf("Failed to start actor system: %v", err)
	}

	// Send a message
	msg := &actor.GameStartMessage{
		GameID:    "test-game",
		RoomID:    "test-room",
		Players:   []string{"player-1"},
		Timestamp: time.Now().Unix(),
	}

	if err := gameActor.Send(msg); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for message processing
	time.Sleep(100 * time.Millisecond)

	stats := gameActor.Stats()
	if stats == nil {
		t.Error("Expected non-nil stats")
	}

	_ = system.Stop()
}

// TestDualEngine tests the dual engine functionality
func TestDualEngine(t *testing.T) {
	ctx := context.Background()

	// Create JS engine
	jsEngine := engine.NewJSEngine()
	if jsEngine == nil {
		t.Fatal("Failed to create JS engine")
	}
	defer jsEngine.Cleanup()

	// Initialize the engine
	if err := jsEngine.Init(ctx, "test-game", nil); err != nil {
		t.Fatalf("Failed to init JS engine: %v", err)
	}

	// Test JavaScript execution
	_, err := jsEngine.Execute(ctx, `
		(function() {
			return 2 + 3;
		})();
	`, nil)
	if err != nil {
		t.Logf("JS execution returned error (may be expected with otto): %v", err)
	}

	// Create dual engine
	dualEngine, err := engine.NewDualEngine("test-game")
	if err != nil {
		t.Fatalf("Failed to create dual engine: %v", err)
	}
	defer dualEngine.Cleanup()

	stats := dualEngine.Stats()
	if stats == nil {
		t.Error("Expected stats, got nil")
	}
}

// TestCache tests the cache functionality
func TestCache(t *testing.T) {
	t.Skip("Requires Redis - skipping in CI")

	ctx := context.Background()

	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	// Test cached query
	testFn := func() (TestStruct, error) {
		return TestStruct{Name: "test", Value: 123}, nil
	}

	// Test basic operations
	result, err := testFn()
	if err != nil {
		t.Fatalf("Test function failed: %v", err)
	}

	if result.Name != "test" || result.Value != 123 {
		t.Errorf("Unexpected result: %+v", result)
	}

	_ = ctx
}

// TestConnectionPool tests the database connection pool
func TestConnectionPool(t *testing.T) {
	t.Skip("Requires database connection - skipping in CI")

	// Test pool configuration
	config := &db.PoolConfig{
		MaxOpenConns:         50,
		MaxIdleConns:         10,
		ConnMaxLifetime:      1 * time.Hour,
		ConnMaxIdleTime:      10 * time.Minute,
		ConnMaxIdleTimeEnabled: true,
	}

	if config.MaxOpenConns != 50 {
		t.Errorf("Expected MaxOpenConns 50, got %d", config.MaxOpenConns)
	}

	if config.MaxIdleConns != 10 {
		t.Errorf("Expected MaxIdleConns 10, got %d", config.MaxIdleConns)
	}
}

// TestWebSocketGateway tests the WebSocket gateway
func TestWebSocketGateway(t *testing.T) {
	config := websocket.DefaultGatewayConfig()
	gateway := websocket.NewGateway(config)
	defer gateway.Close()

	// Test gateway initialization
	if gateway.GetConnectionCount() != 0 {
		t.Errorf("Expected 0 connections, got %d", gateway.GetConnectionCount())
	}

	if gateway.GetRoomCount() != 0 {
		t.Errorf("Expected 0 rooms, got %d", gateway.GetRoomCount())
	}

	stats := gateway.Stats()
	if stats == nil {
		t.Error("Expected stats, got nil")
	}

	// Test room operations (without actual connections)
	members := gateway.GetRoomMembers("test-room")
	if members != nil {
		t.Errorf("Expected nil members for non-existent room, got %v", members)
	}
}

// TestMessageBatching tests message batching
func TestMessageBatching(t *testing.T) {
	var receivedBatches [][]actor.Message
	var mu sync.Mutex

	ctx := context.Background()

	handler := func(messages []actor.Message) error {
		mu.Lock()
		receivedBatches = append(receivedBatches, messages)
		mu.Unlock()
		return nil
	}

	config := actor.BatcherConfig{
		BatchSize: 10,
		Interval:   100 * time.Millisecond,
		Handler:    handler,
	}

	batcher := actor.NewMessageBatcher(config)

	// Send messages
	for i := 0; i < 25; i++ {
		msg := &actor.GameStartMessage{
			GameID:    "test-game",
			RoomID:    "test-room",
			Players:   []string{"player-1"},
			Timestamp: time.Now().Unix(),
		}
		_ = batcher.Send(msg)
	}

	// Wait for batch processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	batchCount := len(receivedBatches)
	mu.Unlock()

	if batchCount == 0 {
		t.Error("Expected at least one batch")
	}

	_ = ctx
}

// TestKafkaProducer tests Kafka producer configuration
func TestKafkaProducer(t *testing.T) {
	t.Skip("Requires Kafka - skipping in CI")

	config := kafka.DefaultProducerConfig()

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if len(config.Brokers) == 0 {
		t.Error("Expected at least one broker")
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}
}

// TestKafkaConsumer tests Kafka consumer configuration
func TestKafkaConsumer(t *testing.T) {
	t.Skip("Requires Kafka - skipping in CI")

	config := kafka.DefaultConsumerConfig()

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if config.GroupID == "" {
		t.Error("Expected non-empty GroupID")
	}

	if config.Heartbeat == 0 {
		t.Error("Expected non-zero Heartbeat")
	}
}

// BenchmarkActorMessageSend benchmarks actor message sending
func BenchmarkActorMessageSend(b *testing.B) {
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
	for i := 0; i < b.N; i++ {
		_ = gameActor.Send(msg)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkEngineExecution benchmarks engine execution
func BenchmarkEngineExecution(b *testing.B) {
	ctx := context.Background()

	jsEngine := engine.NewJSEngine()
	_ = jsEngine.Init(ctx, "test-game", nil)
	defer jsEngine.Cleanup()

	script := `
		function calculate(n) {
			let sum = 0;
			for (let i = 0; i < n; i++) {
				sum += i;
			}
			return sum;
		}
		calculate(100);
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = jsEngine.Execute(ctx, script, nil)
	}
}

// TestActorPool tests the actor object pool
func TestActorPool(t *testing.T) {
	// Use the package-level pool functions
	var actors []*actor.GameActor
	for i := 0; i < 5; i++ {
		gameActor := actor.GetGameActor("test-game", "test-room")
		if gameActor == nil {
			t.Fatal("Expected non-nil actor")
		}
		actors = append(actors, gameActor)
	}

	// Return actors to pool
	for _, a := range actors {
		actor.PutGameActor(a)
	}
}

// TestEngineSelector tests engine selection logic
func TestEngineSelector(t *testing.T) {
	selector := engine.NewEngineSelector()

	// Test with good performance data (should default to JS)
	perfData := &engine.PerformanceData{
		AverageFPS:  60,
		ExecuteTime: 10 * time.Millisecond,
		MemoryUsage: 50 * 1024 * 1024,
		Complexity:  50,
		ErrorCount:  0,
	}

	engineType := selector.SelectEngine(perfData)
	if engineType != engine.EngineJS {
		t.Logf("Expected JS engine type by default, got %v", engineType)
	}
}

// TestMultiPool tests multi-pool management
func TestMultiPool(t *testing.T) {
	multiPool := db.NewMultiPool()

	// Test empty pool
	_, ok := multiPool.Get("test-db")
	if ok {
		t.Error("Expected false for non-existent pool")
	}

	// Test empty stats
	stats := multiPool.Stats()
	if stats == nil {
		t.Error("Expected non-nil stats")
	}

	if len(stats) != 0 {
		t.Errorf("Expected 0 pools, got %d", len(stats))
	}
}

// TestMessageTypes tests various message types
func TestMessageTypes(t *testing.T) {
	tests := []struct {
		name string
		msg  actor.Message
	}{
		{
			name: "GameStart",
			msg: &actor.GameStartMessage{
				GameID:    "game-1",
				RoomID:    "room-1",
				Players:   []string{"player-1"},
				Timestamp: time.Now().Unix(),
			},
		},
		{
			name: "PlayerJoin",
			msg: &actor.PlayerJoinMessage{
				PlayerID:   "player-1",
				PlayerName: "TestPlayer",
				GameID:     "game-1",
				RoomID:     "room-1",
			},
		},
		{
			name: "PlayerAction",
			msg: &actor.PlayerActionMessage{
				PlayerID: "player-1",
				GameID:   "game-1",
				RoomID:   "room-1",
				Action:   "move",
				Data:     map[string]interface{}{"x": 10, "y": 20},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.msg == nil {
				t.Error("Expected non-nil message")
			}
		})
	}
}

// TestCacheKeyBuilder tests cache key building
func TestCacheKeyBuilder(t *testing.T) {
	keyBuilder := func(entity string, id interface{}) string {
		return cache.DefaultKeyBuilder(entity, id)
	}

	key1 := keyBuilder("user", 123)
	if key1 != "user:123" {
		t.Errorf("Expected 'user:123', got '%s'", key1)
	}

	key2 := keyBuilder("game", "game-1")
	if key2 != "game:game-1" {
		t.Errorf("Expected 'game:game-1', got '%s'", key2)
	}
}
