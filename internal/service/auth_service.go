package service

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"aura-forge/internal/domain"
)

const (
	pbkdf2Iterations = 210000
	pbkdf2KeyLen     = 32
	sessionTTL       = 7 * 24 * time.Hour
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserInactive       = errors.New("account has been deactivated by an administrator")
	usernamePattern       = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,40}$`)
)

// AuthService manages application login sessions, password hashing, and admin-only user management.
type AuthService struct {
	repo domain.UserRepository
}

// NewAuthService creates a new AuthService. If repo is nil, an in-memory repository is used.
func NewAuthService(repo domain.UserRepository) *AuthService {
	if repo == nil {
		repo = NewMemoryUserRepo()
	}
	return &AuthService{repo: repo}
}

// EnsureDefaultAdmin seeds the initial administrator account if no users exist yet.
// Returns (true, nil) if a new admin account was created.
func (s *AuthService) EnsureDefaultAdmin(ctx context.Context, username, password string) (bool, error) {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}

	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}
	if len(password) < 6 {
		password = "admin123"
	}

	hash, err := HashPassword(password)
	if err != nil {
		return false, err
	}

	u := &domain.AppUser{
		Username:     strings.ToLower(username),
		DisplayName:  "Fleet Administrator",
		PasswordHash: hash,
		Role:         domain.RoleAdmin,
		IsActive:     true,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return false, err
	}
	return true, nil
}

// Authenticate validates credentials, creates a persistent login session, and returns the raw session token + user.
func (s *AuthService) Authenticate(ctx context.Context, username, password string) (string, *domain.AppUser, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", nil, ErrInvalidCredentials
	}

	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return "", nil, fmt.Errorf("looking up user: %w", err)
	}
	if user == nil || !VerifyPassword(password, user.PasswordHash) {
		return "", nil, ErrInvalidCredentials
	}
	if !user.IsActive {
		return "", nil, ErrUserInactive
	}

	rawToken, err := generateRandomToken(32)
	if err != nil {
		return "", nil, err
	}
	now := time.Now()
	sess := &domain.AppSession{
		TokenHash: HashSessionToken(rawToken),
		UserID:    user.ID,
		ExpiresAt: now.Add(sessionTTL),
		CreatedAt: now,
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return "", nil, err
	}
	_ = s.repo.UpdateLastLogin(ctx, user.ID, now)
	_ = s.repo.CleanExpiredSessions(ctx)
	user.LastLoginAt = &now
	return rawToken, user, nil
}

// ValidateSession verifies a raw session cookie token and returns the active AppUser if valid.
func (s *AuthService) ValidateSession(ctx context.Context, rawToken string) (*domain.AppUser, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, nil
	}
	tokenHash := HashSessionToken(rawToken)
	sess, user, err := s.repo.GetSessionWithUser(ctx, tokenHash)
	if err != nil || sess == nil || user == nil {
		return nil, err
	}
	if time.Now().After(sess.ExpiresAt) || !user.IsActive {
		_ = s.repo.DeleteSession(ctx, tokenHash)
		return nil, nil
	}
	return user, nil
}

// Logout revokes the given raw session token.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, HashSessionToken(rawToken))
}

// ListUsers returns all users for the admin management screen.
func (s *AuthService) ListUsers(ctx context.Context) ([]domain.AppUser, error) {
	return s.repo.ListUsers(ctx)
}

// CreateUser creates a new user account (admin-only operation).
func (s *AuthService) CreateUser(ctx context.Context, username, displayName, password string, role domain.UserRole) (*domain.AppUser, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	displayName = strings.TrimSpace(displayName)
	if !usernamePattern.MatchString(username) {
		return nil, fmt.Errorf("username harus 3–40 karakter (huruf, angka, titik, underscore, atau strip)")
	}
	if displayName == "" {
		displayName = username
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("password minimal 6 karakter")
	}
	if !role.IsValid() {
		return nil, fmt.Errorf("role tidak valid (harus admin atau user)")
	}

	existing, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("username '%s' sudah digunakan", username)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	u := &domain.AppUser{
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: hash,
		Role:         role,
		IsActive:     true,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// UpdateUser updates display name, role, and active status while protecting the last active admin.
func (s *AuthService) UpdateUser(ctx context.Context, actorID, targetID int64, displayName string, role domain.UserRole, isActive bool) (*domain.AppUser, error) {
	if !role.IsValid() {
		return nil, fmt.Errorf("role tidak valid")
	}
	target, err := s.repo.GetUserByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("akun user tidak ditemukan")
	}

	if actorID == targetID {
		if !isActive {
			return nil, fmt.Errorf("tidak dapat menonaktifkan akun sendiri")
		}
		if role != domain.RoleAdmin {
			return nil, fmt.Errorf("tidak dapat menurunkan role akun sendiri")
		}
	}

	// Ensure at least one active admin always remains
	if target.Role == domain.RoleAdmin && target.IsActive && (role != domain.RoleAdmin || !isActive) {
		activeAdmins, err := s.countActiveAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if activeAdmins <= 1 {
			return nil, fmt.Errorf("minimal harus ada 1 akun admin yang aktif")
		}
	}

	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = target.Username
	}
	target.DisplayName = displayName
	target.Role = role
	target.IsActive = isActive

	if err := s.repo.UpdateUser(ctx, target); err != nil {
		return nil, err
	}
	if !isActive {
		_ = s.repo.DeleteSessionsByUser(ctx, target.ID)
	}
	return target, nil
}

// ResetUserPassword updates a user's password and revokes their existing sessions.
func (s *AuthService) ResetUserPassword(ctx context.Context, targetID int64, newPassword string) error {
	if len(newPassword) < 6 {
		return fmt.Errorf("password baru minimal 6 karakter")
	}
	target, err := s.repo.GetUserByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("akun user tidak ditemukan")
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(ctx, targetID, hash); err != nil {
		return err
	}
	_ = s.repo.DeleteSessionsByUser(ctx, targetID)
	return nil
}

// DeleteUser removes a user account while preventing self-deletion or removing the last active admin.
func (s *AuthService) DeleteUser(ctx context.Context, actorID, targetID int64) error {
	if actorID == targetID {
		return fmt.Errorf("tidak dapat menghapus akun sendiri")
	}
	target, err := s.repo.GetUserByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("akun user tidak ditemukan")
	}
	if target.Role == domain.RoleAdmin && target.IsActive {
		activeAdmins, err := s.countActiveAdmins(ctx)
		if err != nil {
			return err
		}
		if activeAdmins <= 1 {
			return fmt.Errorf("tidak dapat menghapus satu-satunya admin aktif")
		}
	}
	return s.repo.DeleteUser(ctx, targetID)
}

func (s *AuthService) countActiveAdmins(ctx context.Context) (int, error) {
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, u := range users {
		if u.Role == domain.RoleAdmin && u.IsActive {
			count++
		}
	}
	return count, nil
}

// HashPassword derives a salted PBKDF2-HMAC-SHA256 hash using Go's standard library.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLen)
	if err != nil {
		return "", fmt.Errorf("deriving password key: %w", err)
	}
	return fmt.Sprintf("pbkdf2$sha256$%d$%s$%s", pbkdf2Iterations, hex.EncodeToString(salt), hex.EncodeToString(dk)), nil
}

// VerifyPassword compares a plaintext password against a stored PBKDF2 hash in constant time.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "pbkdf2" || parts[1] != "sha256" {
		return false
	}
	iters, err := strconv.Atoi(parts[2])
	if err != nil || iters < 10000 {
		return false
	}
	salt, err := hex.DecodeString(parts[3])
	if err != nil || len(salt) == 0 {
		return false
	}
	expected, err := hex.DecodeString(parts[4])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iters, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

// HashSessionToken returns the SHA-256 hex digest of a raw session cookie token.
func HashSessionToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func generateRandomToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// MemoryUserRepo is a thread-safe in-memory UserRepository for tests and degraded mode.
type MemoryUserRepo struct {
	mu       sync.RWMutex
	nextID   int64
	users    map[int64]*domain.AppUser
	sessions map[string]*domain.AppSession
}

// NewMemoryUserRepo constructs an empty in-memory UserRepository.
func NewMemoryUserRepo() *MemoryUserRepo {
	return &MemoryUserRepo{
		nextID:   1,
		users:    make(map[int64]*domain.AppUser),
		sessions: make(map[string]*domain.AppSession),
	}
}

func (m *MemoryUserRepo) CountUsers(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.users), nil
}

func (m *MemoryUserRepo) ListUsers(ctx context.Context) ([]domain.AppUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.AppUser, 0, len(m.users))
	for _, u := range m.users {
		cp := *u
		out = append(out, cp)
	}
	return out, nil
}

func (m *MemoryUserRepo) GetUserByID(ctx context.Context, id int64) (*domain.AppUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

func (m *MemoryUserRepo) GetUserByUsername(ctx context.Context, username string) (*domain.AppUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if strings.EqualFold(u.Username, username) {
			cp := *u
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *MemoryUserRepo) CreateUser(ctx context.Context, u *domain.AppUser) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	u.ID = m.nextID
	m.nextID++
	u.CreatedAt = now
	u.UpdatedAt = now
	cp := *u
	m.users[u.ID] = &cp
	return nil
}

func (m *MemoryUserRepo) UpdateUser(ctx context.Context, u *domain.AppUser) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.users[u.ID]
	if !ok {
		return nil
	}
	existing.DisplayName = u.DisplayName
	existing.Role = u.Role
	existing.IsActive = u.IsActive
	existing.UpdatedAt = time.Now()
	return nil
}

func (m *MemoryUserRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		u.PasswordHash = passwordHash
		u.UpdatedAt = time.Now()
	}
	return nil
}

func (m *MemoryUserRepo) UpdateLastLogin(ctx context.Context, id int64, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		t := at
		u.LastLoginAt = &t
		u.UpdatedAt = at
	}
	return nil
}

func (m *MemoryUserRepo) DeleteUser(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.users, id)
	for k, s := range m.sessions {
		if s.UserID == id {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *MemoryUserRepo) CreateSession(ctx context.Context, s *domain.AppSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *s
	m.sessions[s.TokenHash] = &cp
	return nil
}

func (m *MemoryUserRepo) GetSessionWithUser(ctx context.Context, tokenHash string) (*domain.AppSession, *domain.AppUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[tokenHash]
	if !ok {
		return nil, nil, nil
	}
	u, ok := m.users[s.UserID]
	if !ok {
		return nil, nil, nil
	}
	scp := *s
	ucp := *u
	return &scp, &ucp, nil
}

func (m *MemoryUserRepo) DeleteSession(ctx context.Context, tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, tokenHash)
	return nil
}

func (m *MemoryUserRepo) DeleteSessionsByUser(ctx context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *MemoryUserRepo) CleanExpiredSessions(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, s := range m.sessions {
		if now.After(s.ExpiresAt) {
			delete(m.sessions, k)
		}
	}
	return nil
}
