package user

import (
	"gorm.io/gorm"
)

// Repository provides database operations for users
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateUser creates a new user
func (r *Repository) CreateUser(user *User) error {
	return r.db.Create(user).Error
}

// GetUserByID retrieves a user by ID
func (r *Repository) GetUserByID(id uint) (*User, error) {
	var user User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (r *Repository) GetUserByUsername(username string) (*User, error) {
	var user User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (r *Repository) GetUserByEmail(email string) (*User, error) {
	var user User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers lists users with pagination
func (r *Repository) ListUsers(offset, limit int) ([]*User, int64, error) {
	var users []*User
	var total int64

	if err := r.db.Model(&User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}

// UpdateUser updates a user
func (r *Repository) UpdateUser(user *User) error {
	return r.db.Save(user).Error
}

// DeleteUser deletes a user
func (r *Repository) DeleteUser(id uint) error {
	return r.db.Delete(&User{}, id).Error
}

// CreateProfile creates a user profile
func (r *Repository) CreateProfile(profile *UserProfile) error {
	return r.db.Create(profile).Error
}

// GetProfileByUserID retrieves a profile by user ID
func (r *Repository) GetProfileByUserID(userID uint) (*UserProfile, error) {
	var profile UserProfile
	err := r.db.Where("user_id = ?", userID).First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// UpdateProfile updates a user profile
func (r *Repository) UpdateProfile(profile *UserProfile) error {
	return r.db.Save(profile).Error
}

// CreateFriend creates a friend relationship
func (r *Repository) CreateFriend(friend *Friend) error {
	return r.db.Create(friend).Error
}

// GetFriends retrieves friends for a user
func (r *Repository) GetFriends(userID uint, status string) ([]*Friend, error) {
	var friends []*Friend
	query := r.db.Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Find(&friends).Error
	return friends, err
}

// UpdateFriend updates a friend relationship
func (r *Repository) UpdateFriend(friend *Friend) error {
	return r.db.Save(friend).Error
}

// DeleteFriend deletes a friend relationship
func (r *Repository) DeleteFriend(userID, friendID uint) error {
	return r.db.Where("user_id = ? AND friend_id = ?", userID, friendID).Delete(&Friend{}).Error
}

// CreateSession creates a user session
func (r *Repository) CreateSession(session *UserSession) error {
	return r.db.Create(session).Error
}

// GetSessionByToken retrieves a session by token
func (r *Repository) GetSessionByToken(token string) (*UserSession, error) {
	var session UserSession
	err := r.db.Where("token = ?", token).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// DeleteSession deletes a session
func (r *Repository) DeleteSession(token string) error {
	return r.db.Where("token = ?", token).Delete(&UserSession{}).Error
}

// DeleteUserSessions deletes all sessions for a user
func (r *Repository) DeleteUserSessions(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&UserSession{}).Error
}
