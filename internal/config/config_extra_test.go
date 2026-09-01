package config_test

import (
	"testing"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestConfigGetInt(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	user, err := s.CreateUser("intuser", "int@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Missing key -> registry default (backup.retention_days default is 90), nil.
	if v, err := cfg.GetInt(user.ID, "backup.retention_days"); err != nil || v != 90 {
		t.Fatalf("GetInt missing = %d, %v", v, err)
	}

	// Set an int value (stored as JSON number -> float64 branch).
	if err := cfg.Set(user.ID, "backup.retention_days", 7); err != nil {
		t.Fatalf("Set retention_days: %v", err)
	}
	if v, err := cfg.GetInt(user.ID, "backup.retention_days"); err != nil || v != 7 {
		t.Fatalf("GetInt = %d, %v", v, err)
	}
}

func TestConfigValidateTokenAndGetUser(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	user, err := s.CreateUser("tokuser", "tok@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	const token = "test-token-abc-123"
	if err := s.SetSetting(user.ID, "api.token", token, true); err != nil {
		t.Fatalf("SetSetting api.token: %v", err)
	}
	if err := cfg.Set(user.ID, "api.enabled", true); err != nil {
		t.Fatalf("Set api.enabled: %v", err)
	}

	// Valid token -> returns the user ID.
	uid, err := cfg.ValidateTokenAndGetUser(token)
	if err != nil || uid != user.ID {
		t.Fatalf("ValidateTokenAndGetUser valid = %q, %v; want %q", uid, err, user.ID)
	}

	// Wrong token -> no user, no error.
	if uid, err := cfg.ValidateTokenAndGetUser("wrong-token"); err != nil || uid != "" {
		t.Fatalf("ValidateTokenAndGetUser wrong = %q, %v; want empty", uid, err)
	}

	// API disabled -> ErrAPIDisabled.
	if err := cfg.Set(user.ID, "api.enabled", false); err != nil {
		t.Fatalf("Set api.enabled false: %v", err)
	}
	if _, err := cfg.ValidateTokenAndGetUser(token); err != config.ErrAPIDisabled {
		t.Fatalf("ValidateTokenAndGetUser disabled err = %v, want ErrAPIDisabled", err)
	}
}

func TestConfigGetInt_AdditionalCases(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	user, err := s.CreateUser("intuser2", "int2@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// String value that parses to int -> string case (Atoi success)
	if err := cfg.Set(user.ID, "backup.retention_days", "14"); err != nil {
		t.Fatalf("Set string: %v", err)
	}
	if v, err := cfg.GetInt(user.ID, "backup.retention_days"); err != nil || v != 14 {
		t.Fatalf("GetInt string = %d, %v", v, err)
	}

	// String value that cannot be parsed -> default
	if err := cfg.Set(user.ID, "backup.retention_days", "not-a-number"); err != nil {
		t.Fatalf("Set bad string: %v", err)
	}
	if v, err := cfg.GetInt(user.ID, "backup.retention_days"); err != nil || v != 0 {
		t.Fatalf("GetInt bad string = %d, %v", v, err)
	}

	// Boolean value -> default (nil return)
	if err := cfg.Set(user.ID, "backup.retention_days", true); err != nil {
		t.Fatalf("Set bool: %v", err)
	}
	if v, err := cfg.GetInt(user.ID, "backup.retention_days"); err != nil || v != 0 {
		t.Fatalf("GetInt bool = %d, %v", v, err)
	}

	// Overwrite back to int via float64
	if err := cfg.Set(user.ID, "backup.retention_days", 30); err != nil {
		t.Fatalf("Set float: %v", err)
	}
	if v, err := cfg.GetInt(user.ID, "backup.retention_days"); err != nil || v != 30 {
		t.Fatalf("GetInt float = %d, %v", v, err)
	}
}
