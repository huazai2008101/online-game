package organization

import (
	"time"
)

type Organization struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	OrgCode        string    `gorm:"uniqueIndex;size:50" json:"org_code"`
	OrgName        string    `gorm:"size:100" json:"org_name"`
	OrgType        string    `gorm:"size:20" json:"org_type"`
	ContactPerson  string    `gorm:"size:50" json:"contact_person"`
	ContactEmail   string    `gorm:"size:100" json:"contact_email"`
	ContactPhone   string    `gorm:"size:20" json:"contact_phone"`
	Status         int       `gorm:"default:1" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type OrganizationMember struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrgID     uint      `gorm:"index" json:"org_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Role      string    `gorm:"size:20" json:"role"`
	Status    int       `gorm:"default:1" json:"status"`
	JoinedAt  time.Time `json:"joined_at"`
}

func (Organization) TableName() string { return "organizations" }
func (OrganizationMember) TableName() string { return "organization_members" }
