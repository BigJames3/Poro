// Package model defines the auth service domain types.
package model

import (
	"time"

	"github.com/google/uuid"
)

// UserRole is the authorization role of an account.
type UserRole string

const (
	RolePersonal   UserRole = "PERSONAL"
	RoleCreator    UserRole = "CREATOR"
	RoleBusiness   UserRole = "BUSINESS"
	RoleEnterprise UserRole = "ENTERPRISE"
	RoleAdmin      UserRole = "ADMIN"
	RoleModerator  UserRole = "MODERATOR"
	RoleSupport    UserRole = "SUPPORT"
)

// UserStatus is the lifecycle state of an account.
type UserStatus string

const (
	StatusPending   UserStatus = "PENDING"
	StatusActive    UserStatus = "ACTIVE"
	StatusSuspended UserStatus = "SUSPENDED"
	StatusDeleted   UserStatus = "DELETED"
)

// User is an auth account. PasswordHash and DeletedAt stay out of JSON responses.
type User struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Phone        *string    `json:"phone,omitempty" db:"phone"`
	Email        *string    `json:"email,omitempty" db:"email"`
	PasswordHash *string    `json:"-" db:"password_hash"`
	Role         UserRole   `json:"role" db:"role"`
	Status       UserStatus `json:"status" db:"status"`
	CountryCode  *string    `json:"country_code,omitempty" db:"country_code"`
	Language     string     `json:"language" db:"language"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt    *time.Time `json:"-" db:"deleted_at"`
}
