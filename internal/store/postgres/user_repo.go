package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepo implements domain.UserRepository backed by PostgreSQL.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a new PostgreSQL-backed UserRepository.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM app_users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting app_users: %w", err)
	}
	return count, nil
}

func (r *UserRepo) ListUsers(ctx context.Context) ([]domain.AppUser, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, username, display_name, password_hash, role, is_active, last_login_at, created_at, updated_at
		FROM app_users
		ORDER BY CASE WHEN role = 'admin' THEN 0 ELSE 1 END, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing app_users: %w", err)
	}
	defer rows.Close()

	users := make([]domain.AppUser, 0)
	for rows.Next() {
		var u domain.AppUser
		var role string
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning app_user: %w", err)
		}
		u.Role = domain.UserRole(role)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (*domain.AppUser, error) {
	var u domain.AppUser
	var role string
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, display_name, password_hash, role, is_active, last_login_at, created_at, updated_at
		FROM app_users
		WHERE id = $1
	`, id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	u.Role = domain.UserRole(role)
	return &u, nil
}

func (r *UserRepo) GetUserByUsername(ctx context.Context, username string) (*domain.AppUser, error) {
	var u domain.AppUser
	var role string
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, display_name, password_hash, role, is_active, last_login_at, created_at, updated_at
		FROM app_users
		WHERE LOWER(username) = LOWER($1)
	`, username).Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting user by username: %w", err)
	}
	u.Role = domain.UserRole(role)
	return &u, nil
}

func (r *UserRepo) CreateUser(ctx context.Context, u *domain.AppUser) error {
	now := time.Now()
	err := r.pool.QueryRow(ctx, `
		INSERT INTO app_users (username, display_name, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id, created_at, updated_at
	`, u.Username, u.DisplayName, u.PasswordHash, string(u.Role), u.IsActive, now).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, u *domain.AppUser) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE app_users
		SET display_name = $2, role = $3, is_active = $4, updated_at = NOW()
		WHERE id = $1
	`, u.ID, u.DisplayName, string(u.Role), u.IsActive)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE app_users
		SET password_hash = $2, updated_at = NOW()
		WHERE id = $1
	`, id, passwordHash)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdateLastLogin(ctx context.Context, id int64, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE app_users
		SET last_login_at = $2, updated_at = NOW()
		WHERE id = $1
	`, id, at)
	return err
}

func (r *UserRepo) DeleteUser(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM app_users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	return nil
}

func (r *UserRepo) CreateSession(ctx context.Context, s *domain.AppSession) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO app_sessions (token_hash, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
	`, s.TokenHash, s.UserID, s.ExpiresAt, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

func (r *UserRepo) GetSessionWithUser(ctx context.Context, tokenHash string) (*domain.AppSession, *domain.AppUser, error) {
	var s domain.AppSession
	var u domain.AppUser
	var role string
	err := r.pool.QueryRow(ctx, `
		SELECT
			s.token_hash, s.user_id, s.expires_at, s.created_at,
			u.id, u.username, u.display_name, u.password_hash, u.role, u.is_active, u.last_login_at, u.created_at, u.updated_at
		FROM app_sessions s
		JOIN app_users u ON u.id = s.user_id
		WHERE s.token_hash = $1
	`, tokenHash).Scan(
		&s.TokenHash, &s.UserID, &s.ExpiresAt, &s.CreatedAt,
		&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("getting session with user: %w", err)
	}
	u.Role = domain.UserRole(role)
	return &s, &u, nil
}

func (r *UserRepo) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM app_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *UserRepo) DeleteSessionsByUser(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM app_sessions WHERE user_id = $1`, userID)
	return err
}

func (r *UserRepo) CleanExpiredSessions(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM app_sessions WHERE expires_at <= NOW()`)
	return err
}
