package admin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"online-game/pkg/apperror"
)

// setupTestDB creates an in-memory SQLite database with migrations applied.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to open sqlite in-memory database")

	// We need to register the Game and GameVersion models.
	// Import the game package and let the admin Service.Migrate handle it.
	return db
}

// setupTestService creates a Service with an in-memory DB and a temp storage directory.
func setupTestService(t *testing.T) (*Service, string, func()) {
	t.Helper()
	db := setupTestDB(t)

	storagePath, err := os.MkdirTemp("", "admin-test-storage-*")
	require.NoError(t, err, "failed to create temp storage directory")

	svc := NewService(db, storagePath)
	require.NoError(t, svc.Migrate(), "failed to run migrations")

	cleanup := func() {
		os.RemoveAll(storagePath)
	}
	return svc, storagePath, cleanup
}

// --- Test 1: CreateGame Success ---

func TestCreateGame_Success(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	g, err := svc.CreateGame("blackjack", "21点", "turn-based", 2, 6)

	assert.NoError(t, err)
	assert.NotNil(t, g)
	assert.NotZero(t, g.ID, "game ID should be auto-generated")
	assert.Equal(t, "blackjack", g.GameCode)
	assert.Equal(t, "21点", g.GameName)
	assert.Equal(t, "turn-based", g.GameType)
	assert.Equal(t, 2, g.MinPlayers)
	assert.Equal(t, 6, g.MaxPlayers)
	assert.Equal(t, "draft", g.Status, "new games should be in draft status")
}

// --- Test 2: CreateGame then GetGame ---

func TestCreateGame_AndGetGame(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("chess", "国际象棋", "turn-based", 2, 2)
	require.NoError(t, err, "create game should succeed")

	found, err := svc.GetGame(created.ID)

	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.GameCode, found.GameCode)
	assert.Equal(t, created.GameName, found.GameName)
	assert.Equal(t, created.GameType, found.GameType)
	assert.Equal(t, created.MinPlayers, found.MinPlayers)
	assert.Equal(t, created.MaxPlayers, found.MaxPlayers)
}

// --- Test 3: GetGame Not Found ---

func TestGetGame_NotFound(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	_, err := svc.GetGame(99999)

	assert.Error(t, err)
	assert.Equal(t, apperror.ErrGameNotFound, err)
}

// --- Test 4: UpdateGame ---

func TestUpdateGame(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("poker", "德州扑克", "turn-based", 2, 10)
	require.NoError(t, err, "create game should succeed")

	updated, err := svc.UpdateGame(created.ID, map[string]any{
		"game_name":  "德州扑克-升级版",
		"max_players": 8,
	})

	assert.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "德州扑克-升级版", updated.GameName)
	assert.Equal(t, 8, updated.MaxPlayers)
	// Unchanged fields remain intact
	assert.Equal(t, "poker", updated.GameCode)
	assert.Equal(t, "turn-based", updated.GameType)
}

// --- Test 5: DeleteGame ---

func TestDeleteGame(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("go", "围棋", "turn-based", 2, 2)
	require.NoError(t, err, "create game should succeed")

	err = svc.DeleteGame(created.ID)
	assert.NoError(t, err, "delete should succeed")

	// After deletion the game should not be found (soft delete)
	_, err = svc.GetGame(created.ID)
	assert.Error(t, err, "should not find a deleted game")
	assert.Equal(t, apperror.ErrGameNotFound, err)
}

// --- Test 6: ListGames with Pagination ---

func TestListGames_Pagination(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	// Create 5 games
	for i := 0; i < 5; i++ {
		_, err := svc.CreateGame(
			fmt.Sprintf("game-%d", i),
			fmt.Sprintf("游戏%d", i),
			"turn-based",
			2,
			4,
		)
		require.NoError(t, err, "create game %d should succeed", i)
	}

	// Page 1, pageSize 2
	games, total, err := svc.ListGames(1, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total, "total should be 5")
	assert.Len(t, games, 2, "page 1 should have 2 games")

	// Page 2, pageSize 2
	games, total, err = svc.ListGames(2, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, games, 2, "page 2 should have 2 games")

	// Page 3, pageSize 2 (only 1 remaining)
	games, total, err = svc.ListGames(3, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, games, 1, "page 3 should have 1 game")

	// Page beyond range
	games, total, err = svc.ListGames(10, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, games, 0, "page beyond range should have 0 games")
}

// --- Test 7: UploadPackage Missing Manifest ---

func TestUploadPackage_MissingManifest(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("test", "测试游戏", "realtime", 2, 4)
	require.NoError(t, err, "create game should succeed")

	// Build a zip with no manifest.json
	zipData := createZip(t, map[string]string{
		"server/main.js": "// hello",
	})

	_, err = svc.UploadPackage(created.ID, "1.0.0", zipData)

	assert.Error(t, err)
	appErr, ok := err.(*apperror.AppError)
	require.True(t, ok, "error should be an AppError")
	assert.Equal(t, apperror.ErrInvalidPackage.Code, appErr.Code)
	assert.Contains(t, appErr.Message, "manifest")
}

// --- Test 8: UploadPackage Game Not Found ---

func TestUploadPackage_GameNotFound(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	zipData := createZip(t, map[string]string{
		"manifest.json": `{"name":"t","version":"1.0.0","entry":"main.js"}`,
		"main.js":       "// entry",
	})

	_, err := svc.UploadPackage(99999, "1.0.0", zipData)

	assert.Error(t, err)
	assert.Equal(t, apperror.ErrGameNotFound, err)
}

// --- Test 9: PublishGame ---

func TestPublishGame(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("mahjong", "麻将", "realtime", 4, 4)
	require.NoError(t, err, "create game should succeed")

	// Upload a valid package first so a version exists
	zipData := createZip(t, map[string]string{
		"manifest.json": `{"name":"麻将","version":"1.0.0","entry":"server/main.js"}`,
		"server/main.js": "// game logic",
	})
	_, err = svc.UploadPackage(created.ID, "1.0.0", zipData)
	require.NoError(t, err, "upload package should succeed")

	published, err := svc.PublishGame(created.ID, "1.0.0")

	assert.NoError(t, err)
	assert.Equal(t, "published", published.Status)
	assert.Equal(t, created.ID, published.ID)
}

// --- Test 10: UnpublishGame ---

func TestUnpublishGame(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("uno", "UNO", "realtime", 2, 10)
	require.NoError(t, err, "create game should succeed")

	// Upload a version and publish
	zipData := createZip(t, map[string]string{
		"manifest.json": `{"name":"UNO","version":"2.0.0","entry":"index.js"}`,
		"index.js":      "// entry",
	})
	_, err = svc.UploadPackage(created.ID, "2.0.0", zipData)
	require.NoError(t, err, "upload package should succeed")

	_, err = svc.PublishGame(created.ID, "2.0.0")
	require.NoError(t, err, "publish should succeed")

	// Now unpublish
	err = svc.UnpublishGame(created.ID)
	assert.NoError(t, err, "unpublish should succeed")

	// Verify the game is now offline
	game, err := svc.GetGame(created.ID)
	require.NoError(t, err, "get game should succeed")
	assert.Equal(t, "offline", game.Status)
}

// --- UploadPackage with valid package (integration) ---

func TestUploadPackage_ValidPackage(t *testing.T) {
	svc, storagePath, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("blackjack", "21点", "turn-based", 2, 6)
	require.NoError(t, err, "create game should succeed")

	manifestJSON := `{"name":"21点","version":"1.0.0","gameCode":"blackjack","entry":"server/main.js"}`
	zipData := createZip(t, map[string]string{
		"manifest.json":  manifestJSON,
		"server/main.js": "function onJoin(player) { /* ... */ }",
		"client/index.html": "<h1>21点</h1>",
	})

	gv, err := svc.UploadPackage(created.ID, "1.0.0", zipData)

	assert.NoError(t, err)
	assert.NotNil(t, gv)
	assert.NotZero(t, gv.ID)
	assert.Equal(t, created.ID, gv.GameID)
	assert.Equal(t, "1.0.0", gv.Version)
	assert.Equal(t, "js", gv.ScriptType)
	assert.Equal(t, "server/main.js", gv.EntryScript)
	assert.Equal(t, "active", gv.Status)
	assert.Greater(t, gv.PackageSize, int64(0), "package size should be positive")
	assert.NotEmpty(t, gv.PackageHash, "package hash should not be empty")

	// Verify files were extracted to storage
	expectedEntry := filepath.Join(storagePath, "games", fmt.Sprintf("%d", created.ID), "1.0.0", "server", "main.js")
	_, err = os.Stat(expectedEntry)
	assert.NoError(t, err, "entry script should be extracted to storage path")
}

// --- UploadPackage Missing Entry Script ---

func TestUploadPackage_MissingEntryScript(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("test2", "测试2", "turn-based", 2, 4)
	require.NoError(t, err, "create game should succeed")

	// Manifest references "server/main.js" but the zip only contains "other.js"
	zipData := createZip(t, map[string]string{
		"manifest.json": `{"name":"t","version":"1.0.0","entry":"server/main.js"}`,
		"other.js":      "// not the entry",
	})

	_, err = svc.UploadPackage(created.ID, "1.0.0", zipData)
	assert.Error(t, err)
	appErr, ok := err.(*apperror.AppError)
	require.True(t, ok, "error should be an AppError")
	assert.Equal(t, apperror.ErrInvalidPackage.Code, appErr.Code)
	assert.Contains(t, appErr.Message, "入口脚本不存在")
}

// --- UploadPackage Invalid Manifest JSON ---

func TestUploadPackage_InvalidManifestJSON(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("test3", "测试3", "turn-based", 2, 4)
	require.NoError(t, err, "create game should succeed")

	zipData := createZip(t, map[string]string{
		"manifest.json": `{invalid json}`,
		"main.js":       "// entry",
	})

	_, err = svc.UploadPackage(created.ID, "1.0.0", zipData)
	assert.Error(t, err)
	appErr, ok := err.(*apperror.AppError)
	require.True(t, ok, "error should be an AppError")
	assert.Equal(t, apperror.ErrInvalidPackage.Code, appErr.Code)
}

// --- UploadPackage Manifest Missing Required Fields ---

func TestUploadPackage_ManifestMissingFields(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("test4", "测试4", "turn-based", 2, 4)
	require.NoError(t, err, "create game should succeed")

	tests := []struct {
		name     string
		manifest string
	}{
		{
			name:     "missing name",
			manifest: `{"version":"1.0.0","entry":"main.js"}`,
		},
		{
			name:     "missing version",
			manifest: `{"name":"t","entry":"main.js"}`,
		},
		{
			name:     "missing entry",
			manifest: `{"name":"t","version":"1.0.0"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			zipData := createZip(t, map[string]string{
				"manifest.json": tc.manifest,
				"main.js":       "// entry",
			})

			_, err := svc.UploadPackage(created.ID, "1.0.0", zipData)
			assert.Error(t, err)
			appErr, ok := err.(*apperror.AppError)
			require.True(t, ok, "error should be an AppError")
			assert.Equal(t, apperror.ErrInvalidPackage.Code, appErr.Code)
			assert.Contains(t, appErr.Message, "manifest缺少必要字段")
		})
	}
}

// --- PublishGame Version Not Found ---

func TestPublishGame_VersionNotFound(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	created, err := svc.CreateGame("nomatch", "无版本游戏", "turn-based", 2, 4)
	require.NoError(t, err, "create game should succeed")

	_, err = svc.PublishGame(created.ID, "99.0.0")
	assert.Error(t, err)
	appErr, ok := err.(*apperror.AppError)
	require.True(t, ok, "error should be an AppError")
	assert.Equal(t, apperror.ErrNotFound.Code, appErr.Code)
}

// --- DeleteGame Nonexistent ---

func TestDeleteGame_Nonexistent(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	// Deleting a nonexistent ID should not error (GORM soft-delete is idempotent)
	err := svc.DeleteGame(99999)
	assert.NoError(t, err)
}

// --- UpdateGame Nonexistent ---

func TestUpdateGame_Nonexistent(t *testing.T) {
	svc, _, cleanup := setupTestService(t)
	defer cleanup()

	_, err := svc.UpdateGame(99999, map[string]any{"game_name": "不存在"})
	assert.Error(t, err)
	assert.Equal(t, apperror.ErrGameNotFound, err)
}

// --- Helper: createZip builds an in-memory zip archive from a map of filename -> content. ---

func createZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		require.NoError(t, err, "failed to create zip entry %q", name)
		_, err = fw.Write([]byte(content))
		require.NoError(t, err, "failed to write zip entry %q", name)
	}
	require.NoError(t, w.Close(), "failed to finalize zip archive")
	return buf.Bytes()
}

// --- Helper: decodeManifest is a convenience for verifying manifest content in tests. ---

func decodeManifest(t *testing.T, data []byte) Manifest {
	t.Helper()
	var m Manifest
	require.NoError(t, json.Unmarshal(data, &m), "failed to decode manifest JSON")
	return m
}
