package game

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"online-game/pkg/actor"
	"online-game/pkg/apperror"
	"online-game/pkg/websocket"
)

// setupTestDB creates an in-memory SQLite database and runs migrations.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to open SQLite in-memory database")
	return db
}

// setupService creates a Service with an in-memory DB, actor system, ws gateway, and no cache.
func setupService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	actorSystem := actor.NewActorSystem()
	wsGateway := websocket.NewGateway(websocket.DefaultGatewayConfig())
	svc := NewService(db, actorSystem, wsGateway, nil)

	// Run migrations
	err := svc.Migrate()
	require.NoError(t, err, "failed to run migrations")

	t.Cleanup(func() {
		actorSystem.Shutdown(5 * time.Second)
	})

	return svc, db
}

// createTestGame inserts a game directly into the DB for testing.
func createTestGame(t *testing.T, db *gorm.DB, overrides ...func(g *Game)) *Game {
	t.Helper()
	game := &Game{
		GameCode:   fmt.Sprintf("game-%d", time.Now().UnixNano()),
		GameName:   "Test Game",
		GameType:   "turn-based",
		Status:     "published",
		MinPlayers: 2,
		MaxPlayers: 10,
	}
	for _, fn := range overrides {
		fn(game)
	}
	require.NoError(t, db.Create(game).Error, "failed to create test game")
	return game
}

// createTestRoom inserts a room directly into the DB for testing.
func createTestRoom(t *testing.T, db *gorm.DB, gameID uint, overrides ...func(r *GameRoom)) *GameRoom {
	t.Helper()
	room := &GameRoom{
		RoomID:     fmt.Sprintf("room-%d", time.Now().UnixNano()),
		GameID:     gameID,
		RoomName:   "Test Room",
		OwnerID:    "player-1",
		MaxPlayers: 4,
		Status:     "waiting",
	}
	for _, fn := range overrides {
		fn(room)
	}
	require.NoError(t, db.Create(room).Error, "failed to create test room")
	return room
}

// setupRoomWithActor creates a room in the DB and registers a running BaseActor
// with a no-op handler. This is needed because ActorSystem.Register only auto-starts
// *BaseActor (not *GameActor which embeds it), so rooms created via CreateRoom have
// actors that never enter the running state.
func setupRoomWithActor(t *testing.T, svc *Service, gameID uint) *GameRoom {
	t.Helper()
	room := createTestRoom(t, svc.db, gameID)

	actorID := fmt.Sprintf("game:%d:%s", room.GameID, room.RoomID)
	ba := actor.NewBaseActor(actorID, func(msg *actor.Message) error { return nil }, 256)
	ba.Start()
	require.NoError(t, svc.actorSystem.Register(ba), "failed to register test actor")

	return room
}

// ---------- Tests ----------

func TestListGames_Empty(t *testing.T) {
	svc, _ := setupService(t)

	games, total, err := svc.ListGames(&GameListQuery{}, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, games)
}

func TestCreateAndGetGame(t *testing.T) {
	svc, db := setupService(t)

	// Insert a published game directly
	game := createTestGame(t, db, func(g *Game) {
		g.GameCode = "blackjack"
		g.GameName = "Blackjack"
		g.Status = "published"
	})

	// Fetch by ID
	found, err := svc.GetGame(game.ID)
	assert.NoError(t, err)
	assert.Equal(t, game.ID, found.ID)
	assert.Equal(t, "blackjack", found.GameCode)
	assert.Equal(t, "Blackjack", found.GameName)
	assert.Equal(t, "published", found.Status)
}

func TestGetGame_NotFound(t *testing.T) {
	svc, _ := setupService(t)

	_, err := svc.GetGame(99999)
	assert.Error(t, err)
	assert.Equal(t, apperror.ErrGameNotFound, err)
}

func TestGetLatestVersion_NotFound(t *testing.T) {
	svc, _ := setupService(t)

	_, err := svc.GetLatestVersion(99999)
	assert.Error(t, err)
	appErr, ok := err.(*apperror.AppError)
	require.True(t, ok, "expected AppError")
	assert.Equal(t, 40400, appErr.Code) // ErrNotFound code
}

func TestCreateRoom_Success(t *testing.T) {
	svc, _ := setupService(t)

	game := createTestGame(t, svc.db)

	room, err := svc.CreateRoom(context.Background(), "player-1", "Alice", &CreateRoomRequest{
		GameID:     game.ID,
		RoomName:   "Alice's Room",
		MaxPlayers: 6,
	})
	assert.NoError(t, err)
	assert.NotNil(t, room)
	assert.Equal(t, game.ID, room.GameID)
	assert.Equal(t, "Alice's Room", room.RoomName)
	assert.Equal(t, "player-1", room.OwnerID)
	assert.Equal(t, 6, room.MaxPlayers)
	assert.Equal(t, "waiting", room.Status)
	assert.NotEmpty(t, room.RoomID)
}

func TestCreateRoom_GameNotFound(t *testing.T) {
	svc, _ := setupService(t)

	_, err := svc.CreateRoom(context.Background(), "player-1", "Alice", &CreateRoomRequest{
		GameID:     99999,
		RoomName:   "Ghost Room",
		MaxPlayers: 4,
	})
	assert.Error(t, err)
	assert.Equal(t, apperror.ErrGameNotFound, err)
}

func TestJoinRoom_Success(t *testing.T) {
	svc, _ := setupService(t)

	game := createTestGame(t, svc.db)

	// Create a room with a running actor
	room := setupRoomWithActor(t, svc, game.ID)

	// Another player joins
	err := svc.JoinRoom(room.RoomID, "player-2", "Bob")
	assert.NoError(t, err)

	// Verify player record was created
	var player RoomPlayer
	err = svc.db.Where("room_id = ? AND player_id = ?", room.RoomID, "player-2").First(&player).Error
	assert.NoError(t, err)
	assert.Equal(t, "player-2", player.PlayerID)
	assert.Equal(t, "Bob", player.Nickname)
	assert.Equal(t, "waiting", player.Status)
}

func TestJoinRoom_RoomNotFound(t *testing.T) {
	svc, _ := setupService(t)

	err := svc.JoinRoom("nonexistent-room", "player-1", "Alice")
	assert.Error(t, err)
	assert.Equal(t, apperror.ErrRoomNotFound, err)
}

func TestLeaveRoom_Success(t *testing.T) {
	svc, _ := setupService(t)

	game := createTestGame(t, svc.db)

	// Create a room with a running actor
	room := setupRoomWithActor(t, svc, game.ID)

	// Join a second player
	err := svc.JoinRoom(room.RoomID, "player-2", "Bob")
	require.NoError(t, err)

	// Player 2 leaves
	err = svc.LeaveRoom(room.RoomID, "player-2")
	assert.NoError(t, err)

	// Verify left_at was set
	var player RoomPlayer
	err = svc.db.Where("room_id = ? AND player_id = ?", room.RoomID, "player-2").First(&player).Error
	assert.NoError(t, err)
	assert.NotNil(t, player.LeftAt, "left_at should be set after leaving")
}

func TestGetRooms(t *testing.T) {
	svc, _ := setupService(t)

	game := createTestGame(t, svc.db)

	// Initially no rooms
	rooms, total, err := svc.GetRooms(game.ID, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, rooms)

	// Create two rooms
	_, err = svc.CreateRoom(context.Background(), "player-1", "Alice", &CreateRoomRequest{
		GameID:   game.ID,
		RoomName: "Room 1",
	})
	require.NoError(t, err)

	_, err = svc.CreateRoom(context.Background(), "player-2", "Bob", &CreateRoomRequest{
		GameID:   game.ID,
		RoomName: "Room 2",
	})
	require.NoError(t, err)

	// Should list both rooms
	rooms, total, err = svc.GetRooms(game.ID, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, rooms, 2)

	// Verify closed rooms are excluded
	rooms[0].Status = "closed"
	svc.db.Save(&rooms[0])

	rooms, total, err = svc.GetRooms(game.ID, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, rooms, 1)
}

func TestCloseRoom(t *testing.T) {
	svc, _ := setupService(t)

	game := createTestGame(t, svc.db)

	room, err := svc.CreateRoom(context.Background(), "player-1", "Alice", &CreateRoomRequest{
		GameID:   game.ID,
		RoomName: "Closeable Room",
	})
	require.NoError(t, err)
	assert.Equal(t, "waiting", room.Status)

	// Close the room
	err = svc.CloseRoom(room.RoomID)
	assert.NoError(t, err)

	// Verify room status changed
	var updated GameRoom
	err = svc.db.Where("room_id = ?", room.RoomID).First(&updated).Error
	assert.NoError(t, err)
	assert.Equal(t, "closed", updated.Status)
	assert.NotNil(t, updated.ClosedAt, "closed_at should be set")
}

func TestMigrate(t *testing.T) {
	svc, _ := setupService(t)

	// Migrate was already called in setupService; calling again should not fail.
	err := svc.Migrate()
	assert.NoError(t, err)

	// Verify tables exist by creating records
	game := &Game{
		GameCode:   "mig-test",
		GameName:   "Migration Test",
		Status:     "published",
		MinPlayers: 2,
		MaxPlayers: 4,
	}
	assert.NoError(t, svc.db.Create(game).Error)

	version := &GameVersion{
		GameID:     game.ID,
		Version:    "1.0.0",
		ScriptType: "js",
		Status:     "active",
	}
	assert.NoError(t, svc.db.Create(version).Error)

	room := &GameRoom{
		RoomID:     "mig-room-1",
		GameID:     game.ID,
		RoomName:   "Migration Room",
		OwnerID:    "owner-1",
		MaxPlayers: 4,
		Status:     "waiting",
	}
	assert.NoError(t, svc.db.Create(room).Error)

	session := &GameSession{
		RoomID:    room.RoomID,
		GameID:    game.ID,
		Status:    "running",
		StartTime: time.Now(),
	}
	assert.NoError(t, svc.db.Create(session).Error)

	player := &RoomPlayer{
		RoomID:   room.RoomID,
		PlayerID: "player-1",
		Nickname: "Tester",
		Status:   "waiting",
		JoinedAt: time.Now(),
	}
	assert.NoError(t, svc.db.Create(player).Error)
}
