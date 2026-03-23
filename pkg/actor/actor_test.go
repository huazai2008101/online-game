package actor

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBaseActor(t *testing.T) {
	t.Run("Create and start actor", func(t *testing.T) {
		received := make([]Message, 0)
		mu := &sync.Mutex{}

		a := NewActor("test-actor", "test", 10, func(ctx context.Context, msg Message) error {
			mu.Lock()
			defer mu.Unlock()
			received = append(received, msg)
			return nil
		})

		if a.ID() != "test-actor" {
			t.Errorf("expected ID test-actor, got %s", a.ID())
		}

		if a.Type() != "test" {
			t.Errorf("expected Type test, got %s", a.Type())
		}

		ctx := context.Background()
		if err := a.Start(ctx); err != nil {
			t.Fatalf("failed to start actor: %v", err)
		}
		defer a.Stop()

		// Send a message
		msg := &PingMessage{Timestamp: time.Now().Unix()}
		if err := a.Send(msg); err != nil {
			t.Fatalf("failed to send message: %v", err)
		}

		// Wait for message to be processed
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if len(received) != 1 {
			t.Errorf("expected 1 message, got %d", len(received))
		}
		mu.Unlock()
	})

	t.Run("Actor stats", func(t *testing.T) {
		a := NewActor("test-actor-2", "test", 10, func(ctx context.Context, msg Message) error {
			return nil
		})

		ctx := context.Background()
		_ = a.Start(ctx)
		defer a.Stop()

		// Send some messages
		for i := 0; i < 5; i++ {
			_ = a.Send(&PingMessage{})
		}

		time.Sleep(100 * time.Millisecond)

		stats := a.Stats()
		if stats.MessageCount != 5 {
			t.Errorf("expected 5 messages, got %d", stats.MessageCount)
		}
	})
}

func TestActorSystem(t *testing.T) {
	t.Run("Register and retrieve actor", func(t *testing.T) {
		system := NewActorSystem()
		defer system.Stop()

		a := NewActor("sys-actor-1", "test", 10, func(ctx context.Context, msg Message) error {
			return nil
		})

		if err := system.Register(a); err != nil {
			t.Fatalf("failed to register actor: %v", err)
		}

		retrieved, err := system.Get("sys-actor-1")
		if err != nil {
			t.Fatalf("failed to get actor: %v", err)
		}

		if retrieved.ID() != "sys-actor-1" {
			t.Errorf("retrieved wrong actor")
		}

		count := system.Count()
		if count != 1 {
			t.Errorf("expected count 1, got %d", count)
		}
	})

	t.Run("Send message through system", func(t *testing.T) {
		system := NewActorSystem()
		defer system.Stop()

		received := false
		a := NewActor("sys-actor-2", "test", 10, func(ctx context.Context, msg Message) error {
			received = true
			return nil
		})

		_ = system.Register(a)
		_ = a.Start(context.Background())
		defer a.Stop()

		if err := system.Send("sys-actor-2", &PingMessage{}); err != nil {
			t.Fatalf("failed to send: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		if !received {
			t.Error("message not received")
		}
	})
}

func TestGameActor(t *testing.T) {
	t.Run("Create game actor", func(t *testing.T) {
		game := NewGameActor("game-1", "room-1")

		if game.ID() != "game:game-1:room-1" {
			t.Errorf("unexpected ID: %s", game.ID())
		}

		if game.Type() != "game" {
			t.Errorf("expected type game, got %s", game.Type())
		}
	})

	t.Run("Game start and stop", func(t *testing.T) {
		game := NewGameActor("game-2", "room-2")
		ctx := context.Background()

		_ = game.Start(ctx)
		defer game.Stop()

		// Send game start message
		err := game.Receive(ctx, &GameStartMessage{
			GameID:    "game-2",
			RoomID:    "room-2",
			Players:   []string{"player1", "player2"},
			Timestamp: time.Now().Unix(),
		})

		if err != nil {
			t.Fatalf("game start failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if !game.IsRunning() {
			t.Error("game should be running")
		}

		// Send game stop message
		err = game.Receive(ctx, &GameStopMessage{
			Reason: "test",
			GameID: "game-2",
			RoomID: "room-2",
		})

		if err != nil {
			t.Fatalf("game stop failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if game.IsRunning() {
			t.Error("game should be stopped")
		}
	})

	t.Run("Player join and leave", func(t *testing.T) {
		game := NewGameActor("game-3", "room-3")
		ctx := context.Background()

		_ = game.Start(ctx)
		defer game.Stop()

		// Join player
		err := game.Receive(ctx, &PlayerJoinMessage{
			PlayerID:   "player1",
			PlayerName: "Test Player",
			GameID:     "game-3",
			RoomID:     "room-3",
		})

		if err != nil {
			t.Fatalf("player join failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if game.GetPlayerCount() != 1 {
			t.Errorf("expected 1 player, got %d", game.GetPlayerCount())
		}

		// Leave player
		err = game.Receive(ctx, &PlayerLeaveMessage{
			PlayerID: "player1",
			GameID:   "game-3",
			RoomID:   "room-3",
		})

		if err != nil {
			t.Fatalf("player leave failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if game.GetPlayerCount() != 0 {
			t.Errorf("expected 0 players, got %d", game.GetPlayerCount())
		}
	})
}

func TestPlayerActor(t *testing.T) {
	t.Run("Create player actor", func(t *testing.T) {
		player := NewPlayerActor("player-1")

		if player.ID() != "player:player-1" {
			t.Errorf("unexpected ID: %s", player.ID())
		}

		if player.Type() != "player" {
			t.Errorf("expected type player, got %s", player.Type())
		}
	})

	t.Run("Player state", func(t *testing.T) {
		player := NewPlayerActor("player-2")

		player.SetNickname("TestPlayer")
		if player.GetNickname() != "TestPlayer" {
			t.Errorf("expected nickname TestPlayer, got %s", player.GetNickname())
		}

		player.SetReady(true)
		if !player.IsReady() {
			t.Error("player should be ready")
		}
	})
}

func TestRoomActor(t *testing.T) {
	t.Run("Create room actor", func(t *testing.T) {
		room := NewRoomActor("room-1", "game-1", 4)

		if room.ID() != "room:room-1" {
			t.Errorf("unexpected ID: %s", room.ID())
		}

		if room.Type() != "room" {
			t.Errorf("expected type room, got %s", room.Type())
		}
	})

	t.Run("Room join and leave", func(t *testing.T) {
		room := NewRoomActor("room-2", "game-2", 2)
		ctx := context.Background()

		_ = room.Start(ctx)
		defer room.Stop()

		// Create room
		err := room.Receive(ctx, &RoomCreateMessage{
			RoomID:     "room-2",
			GameID:     "game-2",
			MaxPlayers: 2,
		})

		if err != nil {
			t.Fatalf("room create failed: %v", err)
		}

		// Join players
		err = room.Receive(ctx, &RoomJoinMessage{
			RoomID:   "room-2",
			PlayerID: "player1",
		})

		if err != nil {
			t.Fatalf("player1 join failed: %v", err)
		}

		err = room.Receive(ctx, &RoomJoinMessage{
			RoomID:   "room-2",
			PlayerID: "player2",
		})

		if err != nil {
			t.Fatalf("player2 join failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if room.GetPlayerCount() != 2 {
			t.Errorf("expected 2 players, got %d", room.GetPlayerCount())
		}

		if !room.IsFull() {
			t.Error("room should be full")
		}

		// Leave player
		err = room.Receive(ctx, &RoomLeaveMessage{
			RoomID:   "room-2",
			PlayerID: "player1",
		})

		if err != nil {
			t.Fatalf("player leave failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		if room.GetPlayerCount() != 1 {
			t.Errorf("expected 1 player, got %d", room.GetPlayerCount())
		}
	})
}

func TestActorPool(t *testing.T) {
	t.Run("Get and put game actor", func(t *testing.T) {
		a1 := GetGameActor("game-1", "room-1")
		a1ID := a1.ID()

		PutGameActor(a1)

		a2 := GetGameActor("game-2", "room-2")
		a2ID := a2.ID()

		PutGameActor(a2)

		if a1ID != "game:game-1:room-1" {
			t.Errorf("unexpected a1 ID: %s", a1ID)
		}

		if a2ID != "game:game-2:room-2" {
			t.Errorf("unexpected a2 ID: %s", a2ID)
		}
	})

	t.Run("Get and put player actor", func(t *testing.T) {
		a1 := GetPlayerActor("player-1")
		a1ID := a1.ID()

		PutPlayerActor(a1)

		a2 := GetPlayerActor("player-2")
		a2ID := a2.ID()

		PutPlayerActor(a2)

		if a1ID != "player:player-1" {
			t.Errorf("unexpected a1 ID: %s", a1ID)
		}

		if a2ID != "player:player-2" {
			t.Errorf("unexpected a2 ID: %s", a2ID)
		}
	})
}

func TestMessageBatcher(t *testing.T) {
	t.Run("Batch messages", func(t *testing.T) {
		system := NewActorSystem()
		defer system.Stop()

		received := 0
		mu := &sync.Mutex{}

		a := NewActor("batch-test", "test", 100, func(ctx context.Context, msg Message) error {
			mu.Lock()
			defer mu.Unlock()
			received++
			return nil
		})

		_ = system.Register(a)
		_ = a.Start(context.Background())
		defer a.Stop()

		batcher := NewMessageBatcher(BatcherConfig{
			BatchSize: 10,
			Interval:  50 * time.Millisecond,
			Actor:     a,
		})
		defer batcher.Stop()

		// Send 5 messages (less than batch size)
		for i := 0; i < 5; i++ {
			_ = batcher.Send(&PingMessage{})
		}

		// Flush to force processing
		_ = batcher.Flush()

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if received != 5 {
			t.Errorf("expected 5 messages, got %d", received)
		}
		mu.Unlock()
	})
}
