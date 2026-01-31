package redis

import (
	"log/slog"
	"testing"
	"time"
)

func TestNewSessionStore(t *testing.T) {
	t.Parallel()

	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("with defaults", func(t *testing.T) {
		t.Parallel()
		store := NewSessionStore(client)

		if store.client != client {
			t.Error("client not set correctly")
		}
		if store.keyPrefix != "session" {
			t.Errorf("keyPrefix = %q, want %q", store.keyPrefix, "session")
		}
		if store.defaultTTL != DefaultSessionTTL {
			t.Errorf("defaultTTL = %v, want %v", store.defaultTTL, DefaultSessionTTL)
		}
		if store.logger == nil {
			t.Error("logger is nil")
		}
	})

	t.Run("with options", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		store := NewSessionStore(client,
			WithSessionKeyPrefix("mysess"),
			WithSessionDefaultTTL(7*24*time.Hour),
			WithSessionLogger(logger),
		)

		if store.keyPrefix != "mysess" {
			t.Errorf("keyPrefix = %q, want %q", store.keyPrefix, "mysess")
		}
		if store.defaultTTL != 7*24*time.Hour {
			t.Errorf("defaultTTL = %v, want %v", store.defaultTTL, 7*24*time.Hour)
		}
		if store.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestSessionStore_Keys(t *testing.T) {
	t.Parallel()

	client, _ := New()
	store := NewSessionStore(client)

	t.Run("sessionKey", func(t *testing.T) {
		t.Parallel()
		if got := store.sessionKey("abc123"); got != "session:abc123" {
			t.Errorf("sessionKey() = %q, want %q", got, "session:abc123")
		}
	})

	t.Run("sessionKey with custom prefix", func(t *testing.T) {
		t.Parallel()
		customStore := NewSessionStore(client, WithSessionKeyPrefix("mysess"))
		if got := customStore.sessionKey("abc123"); got != "mysess:abc123" {
			t.Errorf("sessionKey() = %q, want %q", got, "mysess:abc123")
		}
	})

	t.Run("userSessionsKey", func(t *testing.T) {
		t.Parallel()
		// userSessionsKey should include the keyPrefix for namespace isolation
		if got := store.userSessionsKey("user456"); got != "session:user:sessions:user456" {
			t.Errorf("userSessionsKey() = %q, want %q", got, "session:user:sessions:user456")
		}
	})
}

func TestGenerateSessionID(t *testing.T) {
	t.Parallel()

	id1, err1 := generateSessionID()
	if err1 != nil {
		t.Fatalf("generateSessionID() error = %v", err1)
	}

	id2, err2 := generateSessionID()
	if err2 != nil {
		t.Fatalf("generateSessionID() error = %v", err2)
	}

	if id1 == "" {
		t.Error("generateSessionID() returned empty string")
	}
	if id2 == "" {
		t.Error("generateSessionID() returned empty string")
	}
	if id1 == id2 {
		t.Error("generateSessionID() should return unique values")
	}

	// Check length (64 hex chars for 32 bytes)
	if len(id1) != 64 {
		t.Errorf("session ID length = %d, want 64", len(id1))
	}
}

func TestDefaultSessionTTL(t *testing.T) {
	t.Parallel()

	if DefaultSessionTTL != 30*24*time.Hour {
		t.Errorf("DefaultSessionTTL = %v, want %v", DefaultSessionTTL, 30*24*time.Hour)
	}
}

func TestSessionStoreOptions(t *testing.T) {
	t.Parallel()

	client, _ := New()

	t.Run("WithSessionKeyPrefix", func(t *testing.T) {
		t.Parallel()
		store := NewSessionStore(client, WithSessionKeyPrefix("test-prefix"))
		if store.keyPrefix != "test-prefix" {
			t.Errorf("keyPrefix = %q, want %q", store.keyPrefix, "test-prefix")
		}
	})

	t.Run("WithSessionDefaultTTL", func(t *testing.T) {
		t.Parallel()
		store := NewSessionStore(client, WithSessionDefaultTTL(24*time.Hour))
		if store.defaultTTL != 24*time.Hour {
			t.Errorf("defaultTTL = %v, want %v", store.defaultTTL, 24*time.Hour)
		}
	})

	t.Run("WithSessionLogger", func(t *testing.T) {
		t.Parallel()
		logger := slog.Default()
		store := NewSessionStore(client, WithSessionLogger(logger))
		if store.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestSession(t *testing.T) {
	t.Parallel()

	now := time.Now()
	session := &Session{
		ID:         "test-session-id",
		UserID:     "user123",
		DeviceID:   "device456",
		DeviceInfo: "Chrome on macOS",
		IPAddress:  "192.168.1.100",
		CreatedAt:  now,
		LastActive: now,
		ExpiresAt:  now.Add(24 * time.Hour),
		Data:       map[string]string{"role": "admin"},
	}

	if session.ID != "test-session-id" {
		t.Errorf("ID = %q, want %q", session.ID, "test-session-id")
	}
	if session.UserID != "user123" {
		t.Errorf("UserID = %q, want %q", session.UserID, "user123")
	}
	if session.DeviceID != "device456" {
		t.Errorf("DeviceID = %q, want %q", session.DeviceID, "device456")
	}
	if session.DeviceInfo != "Chrome on macOS" {
		t.Errorf("DeviceInfo = %q, want %q", session.DeviceInfo, "Chrome on macOS")
	}
	if session.IPAddress != "192.168.1.100" {
		t.Errorf("IPAddress = %q, want %q", session.IPAddress, "192.168.1.100")
	}
}

func TestSessionCreateOptions(t *testing.T) {
	t.Parallel()

	session := &Session{}

	WithDeviceID("device123")(session)
	if session.DeviceID != "device123" {
		t.Errorf("DeviceID = %q, want %q", session.DeviceID, "device123")
	}

	WithDeviceInfo("Firefox on Windows")(session)
	if session.DeviceInfo != "Firefox on Windows" {
		t.Errorf("DeviceInfo = %q, want %q", session.DeviceInfo, "Firefox on Windows")
	}

	WithIPAddress("10.0.0.1")(session)
	if session.IPAddress != "10.0.0.1" {
		t.Errorf("IPAddress = %q, want %q", session.IPAddress, "10.0.0.1")
	}

	customData := map[string]int{"count": 42}
	WithSessionData(customData)(session)
	if session.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestSessionCreateOptions_Combined(t *testing.T) {
	t.Parallel()

	session := &Session{}

	opts := []SessionCreateOption{
		WithDeviceID("dev1"),
		WithDeviceInfo("Safari"),
		WithIPAddress("8.8.8.8"),
		WithSessionData("custom"),
	}

	for _, opt := range opts {
		opt(session)
	}

	if session.DeviceID != "dev1" {
		t.Errorf("DeviceID = %q, want %q", session.DeviceID, "dev1")
	}
	if session.DeviceInfo != "Safari" {
		t.Errorf("DeviceInfo = %q, want %q", session.DeviceInfo, "Safari")
	}
	if session.IPAddress != "8.8.8.8" {
		t.Errorf("IPAddress = %q, want %q", session.IPAddress, "8.8.8.8")
	}
	if session.Data != "custom" {
		t.Errorf("Data = %v, want %q", session.Data, "custom")
	}
}
