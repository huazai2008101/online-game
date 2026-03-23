package permission

import (
	"time"
)

type Role struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RoleName    string    `gorm:"size:50" json:"role_name"`
	RoleCode    string    `gorm:"uniqueIndex;size:50" json:"role_code"`
	Description string    `gorm:"size:200" json:"description"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
}

type Permission struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	PermissionName string    `gorm:"size:50" json:"permission_name"`
	PermissionCode string    `gorm:"uniqueIndex;size:100" json:"permission_code"`
	Module         string    `gorm:"size:50" json:"module"`
	Resource       string    `gorm:"size:50" json:"resource"`
	Action         string    `gorm:"size:20" json:"action"`
	CreatedAt      time.Time `json:"created_at"`
}

type RolePermission struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	RoleID        uint      `json:"role_id"`
	PermissionID  uint      `json:"permission_id"`
}

type UserRole struct {
	ID     uint      `gorm:"primaryKey" json:"id"`
	UserID uint      `json:"user_id"`
	RoleID uint      `json:"role_id"`
}

func (Role) TableName() string { return "roles" }
func (Permission) TableName() string { return "permissions" }
func (RolePermission) TableName() string { return "role_permissions" }
func (UserRole) TableName() string { return "user_roles" }
