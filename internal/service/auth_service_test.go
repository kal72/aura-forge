package service_test

import (
	"context"
	"testing"

	"aura-forge/internal/domain"
	"aura-forge/internal/service"
)

func TestAuthService_SeedAuthenticateAndUserManagement(t *testing.T) {
	ctx := context.Background()
	auth := service.NewAuthService(nil)

	// 1. Seed initial admin
	created, err := auth.EnsureDefaultAdmin(ctx, "admin", "secret123")
	if err != nil || !created {
		t.Fatalf("expected initial admin seed to succeed, created=%v err=%v", created, err)
	}

	// Second seed must be a no-op
	createdAgain, err := auth.EnsureDefaultAdmin(ctx, "admin", "secret123")
	if err != nil || createdAgain {
		t.Fatalf("expected second seed to be no-op, created=%v err=%v", createdAgain, err)
	}

	// 2. Authenticate admin
	tok, adminUser, err := auth.Authenticate(ctx, "admin", "secret123")
	if err != nil || tok == "" || adminUser == nil || !adminUser.IsAdmin() {
		t.Fatalf("expected admin authentication to succeed, err=%v", err)
	}

	// Wrong password fails
	if _, _, err := auth.Authenticate(ctx, "admin", "wrongpass"); err == nil {
		t.Fatalf("expected wrong password to fail")
	}

	// Validate session
	validated, err := auth.ValidateSession(ctx, tok)
	if err != nil || validated == nil || validated.Username != "admin" {
		t.Fatalf("expected valid session for admin, got=%+v err=%v", validated, err)
	}

	// 3. Admin creates a regular user
	pilot, err := auth.CreateUser(ctx, "pilot.jita", "Jita Pilot", "pilot123", domain.RoleUser)
	if err != nil || pilot == nil || pilot.Role != domain.RoleUser {
		t.Fatalf("expected creating regular user to succeed, err=%v", err)
	}

	// Duplicate username rejected
	if _, err := auth.CreateUser(ctx, "pilot.jita", "Dup", "pilot123", domain.RoleUser); err == nil {
		t.Fatalf("expected duplicate username to be rejected")
	}

	// 4. Safety guards: admin cannot deactivate or delete self
	if _, err := auth.UpdateUser(ctx, adminUser.ID, adminUser.ID, "Admin", domain.RoleAdmin, false); err == nil {
		t.Fatalf("expected self-deactivation to be blocked")
	}
	if err := auth.DeleteUser(ctx, adminUser.ID, adminUser.ID); err == nil {
		t.Fatalf("expected self-deletion to be blocked")
	}

	// 5. Reset password revokes existing sessions
	pilotTok, _, err := auth.Authenticate(ctx, "pilot.jita", "pilot123")
	if err != nil {
		t.Fatalf("pilot auth failed: %v", err)
	}
	if err := auth.ResetUserPassword(ctx, pilot.ID, "newpass999"); err != nil {
		t.Fatalf("reset password failed: %v", err)
	}
	if u, _ := auth.ValidateSession(ctx, pilotTok); u != nil {
		t.Fatalf("expected old pilot session to be revoked after password reset")
	}
	if _, _, err := auth.Authenticate(ctx, "pilot.jita", "newpass999"); err != nil {
		t.Fatalf("expected new password to authenticate: %v", err)
	}

	// 6. Logout revokes admin session
	if err := auth.Logout(ctx, tok); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if u, _ := auth.ValidateSession(ctx, tok); u != nil {
		t.Fatalf("expected admin session to be invalidated after logout")
	}
}
