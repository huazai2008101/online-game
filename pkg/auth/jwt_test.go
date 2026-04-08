package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_GenerateAndValidate(t *testing.T) {
	mgr := NewJWTManager("test-secret", 24*time.Hour)

	token, expiresAt, err := mgr.GenerateToken(1, "alice", "player")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.True(t, expiresAt.After(time.Now()))

	claims, err := mgr.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, uint(1), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, "player", claims.Role)
	assert.Equal(t, "online-game", claims.Issuer)
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	mgr := NewJWTManager("test-secret", -1*time.Second) // already expired

	token, _, err := mgr.GenerateToken(1, "alice", "player")
	require.NoError(t, err)

	_, err = mgr.ValidateToken(token)
	assert.Error(t, err)
}

func TestJWTManager_WrongSecret(t *testing.T) {
	mgr1 := NewJWTManager("secret-1", 24*time.Hour)
	mgr2 := NewJWTManager("secret-2", 24*time.Hour)

	token, _, err := mgr1.GenerateToken(1, "alice", "player")
	require.NoError(t, err)

	_, err = mgr2.ValidateToken(token)
	assert.Error(t, err)
}

func TestJWTManager_InvalidTokenFormat(t *testing.T) {
	mgr := NewJWTManager("test-secret", 24*time.Hour)

	_, err := mgr.ValidateToken("not-a-valid-token")
	assert.Error(t, err)

	_, err = mgr.ValidateToken("")
	assert.Error(t, err)
}

func TestJWTManager_TokenBlacklistKey(t *testing.T) {
	token := "test-token-value"
	key := tokenBlacklistKey(token)

	// Should be SHA256 hash with prefix
	expectedHash := sha256.Sum256([]byte(token))
	expected := "jwt_blacklist:" + hex.EncodeToString(expectedHash[:])
	assert.Equal(t, expected, key)

	// Same token should produce same key
	assert.Equal(t, tokenBlacklistKey(token), tokenBlacklistKey(token))

	// Different tokens should produce different keys
	assert.NotEqual(t, tokenBlacklistKey("token1"), tokenBlacklistKey("token2"))
}

func TestJWTManager_NoRedisBlacklist(t *testing.T) {
	// Without Redis, ValidateToken should work normally
	mgr := NewJWTManager("test-secret", 24*time.Hour)

	token, _, err := mgr.GenerateToken(1, "alice", "player")
	require.NoError(t, err)

	// Validate should succeed (no blacklist check)
	claims, err := mgr.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "alice", claims.Username)

	// BlacklistToken should be no-op
	err = mgr.BlacklistToken(token)
	assert.NoError(t, err)

	// IsBlacklisted should return false
	blacklisted, err := mgr.IsBlacklisted(token)
	assert.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestDefaultSecret(t *testing.T) {
	secret := DefaultSecret()
	assert.NotEmpty(t, secret)
	assert.Equal(t, "online-game-jwt-secret-change-in-prod", secret)
}
