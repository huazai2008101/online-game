package actor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type mockHub struct {
	mu         sync.Mutex
	broadcasts []broadcastRecord
	sentTo     []sendToRecord
	sentExcept []sendExceptRecord
}

type broadcastRecord struct {
	roomID string
	event  string
	data   any
}

type sendToRecord struct {
	playerID string
	event    string
	data     any
}

type sendExceptRecord struct {
	roomID         string
	exceptPlayerID string
	event          string
	data           any
}

func (m *mockHub) BroadcastToRoom(roomID string, event string, data any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcasts = append(m.broadcasts, broadcastRecord{roomID, event, data})
}

func (m *mockHub) SendTo(playerID string, event string, data any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentTo = append(m.sentTo, sendToRecord{playerID, event, data})
}

func (m *mockHub) SendExcept(roomID string, exceptPlayerID string, event string, data any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentExcept = append(m.sentExcept, sendExceptRecord{roomID, exceptPlayerID, event, data})
}

func (m *mockHub) getBroadcasts() []broadcastRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]broadcastRecord{}, m.broadcasts...)
}

func (m *mockHub) getSentTo() []sendToRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]sendToRecord{}, m.sentTo...)
}

type mockRoomMgr struct {
	mu            sync.Mutex
	statusUpdates []statusUpdateRecord
	results       []resultsRecord
}

type statusUpdateRecord struct {
	roomID string
	status string
}

type resultsRecord struct {
	roomID  string
	results any
}

func (m *mockRoomMgr) GetPlayerIDs(roomID string) []string      { return nil }
func (m *mockRoomMgr) GetConfig(roomID string) map[string]any   { return nil }

func (m *mockRoomMgr) UpdateRoomStatus(roomID string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusUpdates = append(m.statusUpdates, statusUpdateRecord{roomID, status})
}

func (m *mockRoomMgr) RecordGameResults(roomID string, results any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, resultsRecord{roomID, results})
}

func (m *mockRoomMgr) getStatusUpdates() []statusUpdateRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]statusUpdateRecord{}, m.statusUpdates...)
}

type mockEngine struct {
	mu        sync.Mutex
	hooks     []hookCall
	destroyed bool
}

type hookCall struct {
	name string
	args []any
}

func (m *mockEngine) TriggerHook(name string, args ...any) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hookCall{name, args})
	return nil, nil
}

func (m *mockEngine) LoadScript(_ string) error { return nil }

func (m *mockEngine) Destroy() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyed = true
}

func (m *mockEngine) getHooks() []hookCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]hookCall{}, m.hooks...)
}

func (m *mockEngine) isDestroyed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.destroyed
}

// --- Helper ---

func newTestGameActor() (*GameActor, *mockHub, *mockRoomMgr, *mockEngine) {
	hub := &mockHub{}
	roomMgr := &mockRoomMgr{}
	eng := &mockEngine{}
	cfg := GameActorConfig{
		GameID:     "g1",
		RoomID:     "r1",
		GameCode:   "test",
		MaxPlayers: 4,
		InboxCap:   16,
		Hub:        hub,
		RoomMgr:    roomMgr,
		Engine:     eng,
	}
	ga := NewGameActor(cfg)
	return ga, hub, roomMgr, eng
}

func startAndWait(ga *GameActor) {
	ga.Start()
	time.Sleep(10 * time.Millisecond)
}

func joinPlayer(t *testing.T, ga *GameActor, pid, nickname string) {
	t.Helper()
	require.NoError(t, ga.Send(NewMessage(MsgPlayerJoin, pid, &PlayerJoinData{
		Nickname: nickname,
		Avatar:   "",
	})))
}

func waitForPlayerCount(ga *GameActor, expected int) bool {
	return assert.Eventually(nil, func() bool {
		return ga.PlayerCount() == expected
	}, time.Second, 10*time.Millisecond)
}

// --- Tests ---

func TestGameActor_PlayerJoin(t *testing.T) {
	ga, hub, _, eng := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "Alice")
	waitForPlayerCount(ga, 1)
	assert.Equal(t, 1, ga.PlayerCount())

	// Check hub broadcast
	bcasts := hub.getBroadcasts()
	require.Len(t, bcasts, 1)
	assert.Equal(t, "r1", bcasts[0].roomID)
	assert.Equal(t, "player_join", bcasts[0].event)

	// Check engine hook
	hooks := eng.getHooks()
	require.Len(t, hooks, 1)
	assert.Equal(t, "onPlayerJoin", hooks[0].name)
}

func TestGameActor_PlayerJoinRoomFull(t *testing.T) {
	ga, hub, _, _ := newTestGameActor()
	ga.maxPlayers = 2
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	require.True(t, waitForPlayerCount(ga, 1))
	joinPlayer(t, ga, "p2", "B")
	require.True(t, waitForPlayerCount(ga, 2))

	// 3rd player should be rejected
	joinPlayer(t, ga, "p3", "C")

	assert.Eventually(t, func() bool {
		sent := hub.getSentTo()
		for _, s := range sent {
			if s.playerID == "p3" && s.event == "error" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, 2, ga.PlayerCount())
}

func TestGameActor_PlayerJoinWhilePlaying(t *testing.T) {
	ga, hub, _, _ := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	require.True(t, waitForPlayerCount(ga, 1))

	require.NoError(t, ga.Send(NewMessage(MsgGameStart, "", nil)))
	assert.Eventually(t, func() bool {
		return len(hub.getBroadcasts()) >= 2
	}, time.Second, 10*time.Millisecond)

	// Try joining during game
	joinPlayer(t, ga, "p2", "B")

	assert.Eventually(t, func() bool {
		sent := hub.getSentTo()
		for _, s := range sent {
			if s.playerID == "p2" && s.event == "error" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, 1, ga.PlayerCount())
}

func TestGameActor_PlayerLeave_OwnerTransfer(t *testing.T) {
	ga, hub, _, _ := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	joinPlayer(t, ga, "p2", "B")
	require.True(t, waitForPlayerCount(ga, 2))

	// Owner (p1) leaves
	require.NoError(t, ga.Send(NewMessage(MsgPlayerLeave, "p1", &PlayerLeaveData{Reason: "voluntary"})))
	require.True(t, waitForPlayerCount(ga, 1))

	// Check owner_change broadcast
	bcasts := hub.getBroadcasts()
	var ownerChangeFound bool
	for _, b := range bcasts {
		if b.event == "owner_change" {
			ownerChangeFound = true
			break
		}
	}
	assert.True(t, ownerChangeFound, "owner_change event should be broadcast")
}

func TestGameActor_PlayerLeave_EmptyRoom(t *testing.T) {
	ga, _, roomMgr, _ := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	require.True(t, waitForPlayerCount(ga, 1))

	require.NoError(t, ga.Send(NewMessage(MsgPlayerLeave, "p1", &PlayerLeaveData{Reason: "voluntary"})))
	require.True(t, waitForPlayerCount(ga, 0))

	assert.Eventually(t, func() bool {
		updates := roomMgr.getStatusUpdates()
		for _, u := range updates {
			if u.status == "closed" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

func TestGameActor_PlayerReady(t *testing.T) {
	ga, hub, _, _ := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	joinPlayer(t, ga, "p2", "B")
	require.True(t, waitForPlayerCount(ga, 2))

	require.NoError(t, ga.Send(NewMessage(MsgPlayerReady, "p1", nil)))
	require.NoError(t, ga.Send(NewMessage(MsgPlayerReady, "p2", nil)))

	assert.Eventually(t, func() bool {
		bcasts := hub.getBroadcasts()
		for _, b := range bcasts {
			if b.event == "all_ready" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

func TestGameActor_GameStart(t *testing.T) {
	ga, hub, _, eng := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	require.True(t, waitForPlayerCount(ga, 1))

	require.NoError(t, ga.Send(NewMessage(MsgGameStart, "", nil)))

	assert.Eventually(t, func() bool {
		hooks := eng.getHooks()
		for _, h := range hooks {
			if h.name == "onGameStart" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	bcasts := hub.getBroadcasts()
	var startFound bool
	for _, b := range bcasts {
		if b.event == "game_start" {
			startFound = true
			break
		}
	}
	assert.True(t, startFound)
}

func TestGameActor_PlayerActionBeforeStart(t *testing.T) {
	ga, hub, _, _ := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	require.True(t, waitForPlayerCount(ga, 1))

	// Action before game start should error
	require.NoError(t, ga.Send(ActionMessage("p1", "bet", 100)))

	assert.Eventually(t, func() bool {
		sent := hub.getSentTo()
		for _, s := range sent {
			if s.event == "error" && s.playerID == "p1" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

func TestGameActor_PlayerActionDuringGame(t *testing.T) {
	ga, _, _, eng := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	require.True(t, waitForPlayerCount(ga, 1))

	require.NoError(t, ga.Send(NewMessage(MsgGameStart, "", nil)))
	assert.Eventually(t, func() bool {
		return len(eng.getHooks()) >= 1
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, ga.Send(ActionMessage("p1", "bet", 100)))

	assert.Eventually(t, func() bool {
		hooks := eng.getHooks()
		for _, h := range hooks {
			if h.name == "onPlayerAction" && h.args[0] == "p1" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

func TestGameActor_Shutdown(t *testing.T) {
	ga, _, roomMgr, eng := newTestGameActor()
	startAndWait(ga)

	joinPlayer(t, ga, "p1", "A")
	require.True(t, waitForPlayerCount(ga, 1))

	require.NoError(t, ga.Send(NewMessage(MsgGameStart, "", nil)))
	assert.Eventually(t, func() bool { return len(eng.getHooks()) >= 1 }, time.Second, 10*time.Millisecond)

	require.NoError(t, ga.Send(NewMessage(MsgShutdown, "", nil)))

	assert.Eventually(t, func() bool {
		return eng.isDestroyed()
	}, time.Second, 10*time.Millisecond)

	updates := roomMgr.getStatusUpdates()
	assert.Equal(t, "closed", updates[len(updates)-1].status)
}

func TestGameActor_DuplicateJoin(t *testing.T) {
	ga, _, _, _ := newTestGameActor()
	startAndWait(ga)
	defer ga.Stop(context.Background())

	joinPlayer(t, ga, "p1", "A")
	require.True(t, waitForPlayerCount(ga, 1))

	// Send join again — should be ignored (no duplicate)
	joinPlayer(t, ga, "p1", "A")
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 1, ga.PlayerCount())
}
