package model

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleLearner    Role = "learner"
	RoleInstructor Role = "instructor"
	RoleAdmin      Role = "admin"
)

type User struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Username     string     `json:"username" db:"username"`
	Email        *string    `json:"email,omitempty" db:"email"`
	DisplayName  string     `json:"display_name" db:"display_name"`
	Role         Role       `json:"role" db:"role"`
	PasswordHash *string    `json:"-" db:"password_hash"`
	ExternalID   *string    `json:"-" db:"external_id"`
	AuthProvider string     `json:"auth_provider" db:"auth_provider"`
	Active       bool       `json:"active" db:"active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}
