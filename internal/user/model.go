package user

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user account
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;size:50" json:"username"`
	Password  string    `gorm:"size:255" json:"-"`
	Email     string    `gorm:"uniqueIndex;size:100" json:"email"`
	Phone     string    `gorm:"uniqueIndex;size:20" json:"phone"`
	Nickname  string    `gorm:"size:50" json:"nickname"`
	Avatar    string    `gorm:"size:255" json:"avatar"`
	Status    int       `gorm:"default:1" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserProfile represents extended user profile information
type UserProfile struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex" json:"user_id"`
	Gender    string    `gorm:"size:10" json:"gender"`
	Birthday *time.Time `json:"birthday"`
	Location string    `gorm:"size:100" json:"location"`
	Bio      string    `gorm:"type:text" json:"bio"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Friend represents a friendship between users
type Friend struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	FriendID   uint      `gorm:"index" json:"friend_id"`
	Status     string    `gorm:"size:20;default:pending" json:"status"` // pending, accepted, blocked
	Remark     string    `gorm:"size:50" json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserSession represents a user session
type UserSession struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index" json:"user_id"`
	Token        string    `gorm:"uniqueIndex;size:255" json:"token"`
	RefreshToken string   `gorm:"size:255" json:"refresh_token"`
	IPAddress   string    `gorm:"size:50" json:"ip_address"`
	UserAgent   string    `gorm:"size:255" json:"user_agent"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

// TableName specifies the table name for UserProfile
func (UserProfile) TableName() string {
	return "user_profiles"
}

// TableName specifies the table name for Friend
func (Friend) TableName() string {
	return "friends"
}

// TableName specifies the table name for UserSession
func (UserSession) TableName() string {
	return "user_sessions"
}
