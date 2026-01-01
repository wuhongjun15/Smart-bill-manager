package models

import "time"

// Invite represents a one-time invitation code for creating a normal user account.
// The plaintext code is never stored in the database; only its hash is persisted.
type Invite struct {
	ID       string `json:"id" gorm:"primaryKey"`
	CodeHash string `json:"-" gorm:"uniqueIndex;not null"`
	CodeHint string `json:"code_hint" gorm:"index"`

	CreatedBy string     `json:"created_by" gorm:"index;not null"`
	ExpiresAt *time.Time `json:"expires_at" gorm:"index"`
	UsedAt    *time.Time `json:"used_at" gorm:"index"`
	UsedBy    *string    `json:"used_by" gorm:"index"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Invite) TableName() string {
	return "invites"
}

