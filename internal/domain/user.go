package domain

import (
	"context"
	"time"
)

type userCtxKey struct{}

// WithAppUser attaches the authenticated AppUser to a context.
func WithAppUser(ctx context.Context, u *AppUser) context.Context {
	return context.WithValue(ctx, userCtxKey{}, u)
}

// AppUserFromContext retrieves the authenticated AppUser from a context, or nil if unauthenticated.
func AppUserFromContext(ctx context.Context) *AppUser {
	if ctx == nil {
		return nil
	}
	u, _ := ctx.Value(userCtxKey{}).(*AppUser)
	return u
}

// UserRole defines access levels in aura-forge.
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// IsValid returns true if the role is either admin or user.
func (r UserRole) IsValid() bool {
	return r == RoleAdmin || r == RoleUser
}

// AppUser represents an authenticated application user account.
// Accounts are created and managed exclusively by admins (no public self-registration).
type AppUser struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	DisplayName  string     `json:"display_name"`
	PasswordHash string     `json:"-"`
	Role         UserRole   `json:"role"`
	IsActive     bool       `json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// IsAdmin returns true if the user holds the admin role.
func (u *AppUser) IsAdmin() bool {
	return u != nil && u.Role == RoleAdmin
}

// AppSession represents a persisted browser login session.
type AppSession struct {
	TokenHash string    `json:"-"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
