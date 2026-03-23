// Demo program for the online game platform
// This demonstrates the Actor model and Dual Engine system
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"online-game/pkg/actor"
	"online-game/pkg/engine"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  Online Game Platform - Phase 1 Demo")
	fmt.Println("  Actor Model + Dual Engine System")
	fmt.Println("========================================")
	fmt.Println()

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived shutdown signal...")
		cancel()
	}()

	// Run demonstrations
	demonstrateActorModel(ctx)
	demonstrateDualEngine(ctx)
	demonstrateIntegration(ctx)

	// Run benchmarks (optional - comment out if not needed)
	// Uncomment to run benchmarks:
	/*
		fmt.Println("\n========================================")
		fmt.Println("  Running Performance Benchmarks")
		fmt.Println("========================================")
		fmt.Println()

		results := benchmark.RunAllBenchmarks(10000)
		benchmark.PrintResults(results)
	*/

	fmt.Println("\n========================================")
	fmt.Println("  Demo Complete!")
	fmt.Println("========================================")
}

// demonstrateActorModel demonstrates the Actor model
func demonstrateActorModel(ctx context.Context) {
	fmt.Println("========================================")
	fmt.Println("  Actor Model Demonstration")
	fmt.Println("========================================")
	fmt.Println()

	// Create actor system
	system := actor.NewActorSystem()
	defer system.Stop()

	// Create a game actor
	gameActor := actor.NewGameActor("demo-game", "demo-room")

	system.Register(gameActor)
	gameActor.Start(ctx)
	defer gameActor.Stop()

	// Create player actors
	for i := 1; i <= 3; i++ {
		playerID := fmt.Sprintf("player%d", i)
		playerActor := actor.NewPlayerActor(playerID)
		playerActor.SetNickname(fmt.Sprintf("Player %d", i))
		playerActor.SetReady(true)

		system.Register(playerActor)
		playerActor.Start(ctx)
		defer playerActor.Stop()

		fmt.Printf("  Created player: %s (nickname: %s, ready: %v)\n",
			playerActor.ID(), playerActor.GetNickname(), playerActor.IsReady())
	}

	// Create a room actor
	roomActor := actor.NewRoomActor("demo-room", "demo-game", 4)
	system.Register(roomActor)
	roomActor.Start(ctx)
	defer roomActor.Stop()

	// Send messages
	fmt.Println("\n  Sending messages...")

	// Start the game
	gameActor.Send(&actor.GameStartMessage{
		GameID:    "demo-game",
		RoomID:    "demo-room",
		Players:   []string{"player1", "player2", "player3"},
		Timestamp: time.Now().Unix(),
	})

	// Players join
	for i := 1; i <= 3; i++ {
		gameActor.Send(&actor.PlayerJoinMessage{
			PlayerID:   fmt.Sprintf("player%d", i),
			PlayerName: fmt.Sprintf("Player %d", i),
			GameID:     "demo-game",
			RoomID:     "demo-room",
		})
	}

	// Wait for messages to be processed
	time.Sleep(200 * time.Millisecond)

	// Show statistics
	stats := gameActor.Stats()
	fmt.Printf("\n  Game Actor Stats:\n")
	fmt.Printf("    Messages processed: %d\n", stats.MessageCount)
	fmt.Printf("    Total processing time: %v\n", stats.ProcessTime)

	avgTime := gameActor.GetAverageProcessTime()
	fmt.Printf("    Average processing time: %v\n", avgTime)

	fmt.Println("\n  Actor Model Demo Complete!")
}

// demonstrateDualEngine demonstrates the Dual Engine system
func demonstrateDualEngine(ctx context.Context) {
	fmt.Println("\n========================================")
	fmt.Println("  Dual Engine Demonstration")
	fmt.Println("========================================")
	fmt.Println()

	// Create dual engine
	dualEngine, err := engine.NewDualEngine("demo-game")
	if err != nil {
		fmt.Printf("  Error creating dual engine: %v\n", err)
		return
	}
	defer dualEngine.Cleanup()

	// Initialize
	err = dualEngine.Init(ctx, "demo-game", map[string]any{"version": "1.0"})
	if err != nil {
		fmt.Printf("  Error initializing dual engine: %v\n", err)
		return
	}

	fmt.Printf("  Dual engine created for game: %s\n", dualEngine.GetGameID())
	fmt.Printf("  Current engine: %s\n\n", dualEngine.CurrentEngine())

	// Load JavaScript game logic
	jsScript := `
		// Game state
		var gameState = {
			players: [],
			score: 0,
			tick: 0
		};

		// Initialize game
		function init(config) {
			gameState.players = config.players || [];
			return "Game initialized with " + gameState.players.length + " players";
		}

		// Update game state
		function update(deltaTime) {
			gameState.tick++;
			gameState.score += deltaTime * 10;
			return {
				tick: gameState.tick,
				score: Math.floor(gameState.score)
			};
		}

		// Player action
		function playerAction(playerId, action) {
			return "Player " + playerId + " performed " + action;
		}

		// Get state
		function getState() {
			return gameState;
		}
	`

	err = dualEngine.LoadScript(engine.EngineJS, []byte(jsScript))
	if err != nil {
		fmt.Printf("  Error loading script: %v\n", err)
		return
	}
	fmt.Println("  JavaScript game logic loaded")

	// Execute functions
	fmt.Println("\n  Executing game functions:")

	// Initialize game
	result, err := dualEngine.Execute(ctx, "init", map[string]any{
		"players": []string{"player1", "player2", "player3"},
	})
	if err == nil {
		fmt.Printf("    init(): %v\n", result)
	}

	// Update game state
	for i := 1; i <= 3; i++ {
		result, err = dualEngine.Execute(ctx, "update", 16.67) // ~60 FPS
		if err == nil {
			fmt.Printf("    update(): tick=%v, score=%v\n", result.(map[string]any)["tick"], result.(map[string]any)["score"])
		}
	}

	// Player action
	result, err = dualEngine.Execute(ctx, "playerAction", []interface{}{"player1", "jump"})
	if err == nil {
		fmt.Printf("    playerAction(): %v\n", result)
	}

	// Show engine stats
	stats := dualEngine.Stats()
	fmt.Printf("\n  Engine Statistics:\n")
	fmt.Printf("    Executions: %d\n", stats.ExecuteCount)
	fmt.Printf("    Total time: %v\n", time.Duration(stats.ExecuteTime))
	if stats.ExecuteCount > 0 {
		avgNs := stats.ExecuteTime / stats.ExecuteCount
		fmt.Printf("    Avg time: %v\n", time.Duration(avgNs))
	}

	fmt.Println("\n  Dual Engine Demo Complete!")
}

// demonstrateIntegration demonstrates the integration of Actor and Engine
func demonstrateIntegration(ctx context.Context) {
	fmt.Println("\n========================================")
	fmt.Println("  Integration Demonstration")
	fmt.Println("========================================")
	fmt.Println()

	// Create a game with integrated engine
	system := actor.NewActorSystem()
	defer system.Stop()

	// Create game with dual engine
	dualEngine, _ := engine.NewDualEngine("integrated-game")
	_ = dualEngine.Init(ctx, "integrated-game", nil)

	// Simple game logic
	jsScript := `
		var gameState = { running: false, tick: 0 };

		function start() {
			gameState.running = true;
			return "Game started";
		}

		function tick() {
			if (gameState.running) {
				gameState.tick++;
			}
			return { tick: gameState.tick, running: gameState.running };
		}

		function stop() {
			gameState.running = false;
			return "Game stopped";
		}
	`
	_ = dualEngine.LoadScript(engine.EngineJS, []byte(jsScript))

	// Create custom actor with engine integration
	customActor := actor.NewActor("integrated-game", "integrated", 100, func(ctx context.Context, msg actor.Message) error {
		switch msg.(type) {
		case *actor.GameStartMessage:
			result, _ := dualEngine.Execute(ctx, "start", nil)
			fmt.Printf("  [Integrated] Game start: %v\n", result)
		case *actor.GameTickMessage:
			result, _ := dualEngine.Execute(ctx, "tick", nil)
			fmt.Printf("  [Integrated] Game tick: %v\n", result)
		case *actor.GameStopMessage:
			result, _ := dualEngine.Execute(ctx, "stop", nil)
			fmt.Printf("  [Integrated] Game stop: %v\n", result)
		}
		return nil
	})

	system.Register(customActor)
	customActor.Start(ctx)
	defer customActor.Stop()
	defer dualEngine.Cleanup()

	fmt.Println("  Simulating game loop:")

	// Start game
	customActor.Send(&actor.GameStartMessage{
		GameID:    "integrated-game",
		RoomID:    "integrated-room",
		Timestamp: time.Now().Unix(),
	})

	// Run some ticks
	for i := 0; i < 5; i++ {
		customActor.Send(&actor.GameTickMessage{
			Timestamp: time.Now().Unix(),
			TickCount: int64(i + 1),
		})
		time.Sleep(50 * time.Millisecond)
	}

	// Stop game
	customActor.Send(&actor.GameStopMessage{
		GameID: "integrated-game",
		Reason: "demo complete",
	})

	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n  Integration Demo Complete!")
}
