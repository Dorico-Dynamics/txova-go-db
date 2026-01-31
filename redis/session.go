// Package redis provides Redis client utilities for the Txova platform.
package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"
)

// Default session settings.
const (
	// DefaultSessionTTL is the default TTL for sessions (30 days).
	DefaultSessionTTL = 30 * 24 * time.Hour
)

// Session represents a user session.
type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	DeviceID   string    `json:"device_id,omitempty"`
	DeviceInfo string    `json:"device_info,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	ExpiresAt  time.Time `json:"expires_at"`
	Data       any       `json:"data,omitempty"`
}

// SessionStore provides session management operations.
type SessionStore struct {
	client     *Client
	logger     *slog.Logger
	keyPrefix  string
	defaultTTL time.Duration
}

// SessionStoreOption is a functional option for configuring the SessionStore.
type SessionStoreOption func(*SessionStore)

// WithSessionKeyPrefix sets a prefix for all session keys.
func WithSessionKeyPrefix(prefix string) SessionStoreOption {
	return func(s *SessionStore) {
		s.keyPrefix = prefix
	}
}

// WithSessionDefaultTTL sets the default TTL for sessions.
func WithSessionDefaultTTL(ttl time.Duration) SessionStoreOption {
	return func(s *SessionStore) {
		s.defaultTTL = ttl
	}
}

// WithSessionLogger sets the logger for the session store.
func WithSessionLogger(logger *slog.Logger) SessionStoreOption {
	return func(s *SessionStore) {
		s.logger = logger
	}
}

// NewSessionStore creates a new SessionStore instance.
func NewSessionStore(client *Client, opts ...SessionStoreOption) *SessionStore {
	store := &SessionStore{
		client:     client,
		logger:     slog.Default(),
		keyPrefix:  "session",
		defaultTTL: DefaultSessionTTL,
	}

	for _, opt := range opts {
		opt(store)
	}

	return store
}

// sessionKey builds the session key.
func (s *SessionStore) sessionKey(sessionID string) string {
	return s.keyPrefix + ":" + sessionID
}

// userSessionsKey builds the user sessions index key.
func (s *SessionStore) userSessionsKey(userID string) string {
	return s.keyPrefix + ":user:sessions:" + userID
}

// generateSessionID generates a cryptographically secure unique session ID.
// Returns an error if random bytes cannot be generated.
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("failed to generate secure session ID: " + err.Error())
	}
	return hex.EncodeToString(b), nil
}

// Create creates a new session.
func (s *SessionStore) Create(ctx context.Context, userID string, opts ...SessionCreateOption) (*Session, error) {
	return s.CreateWithTTL(ctx, userID, s.defaultTTL, opts...)
}

// SessionCreateOption is a functional option for creating a session.
type SessionCreateOption func(*Session)

// WithDeviceID sets the device ID for the session.
func WithDeviceID(deviceID string) SessionCreateOption {
	return func(sess *Session) {
		sess.DeviceID = deviceID
	}
}

// WithDeviceInfo sets the device info for the session.
func WithDeviceInfo(deviceInfo string) SessionCreateOption {
	return func(sess *Session) {
		sess.DeviceInfo = deviceInfo
	}
}

// WithIPAddress sets the IP address for the session.
func WithIPAddress(ip string) SessionCreateOption {
	return func(sess *Session) {
		sess.IPAddress = ip
	}
}

// WithSessionData sets custom data for the session.
func WithSessionData(data any) SessionCreateOption {
	return func(sess *Session) {
		sess.Data = data
	}
}

// CreateWithTTL creates a new session with a custom TTL.
func (s *SessionStore) CreateWithTTL(ctx context.Context, userID string, ttl time.Duration, opts ...SessionCreateOption) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		s.logger.Error("failed to generate session ID", "user_id", userID, "error", err)
		return nil, err
	}

	now := time.Now()
	session := &Session{
		ID:         sessionID,
		UserID:     userID,
		CreatedAt:  now,
		LastActive: now,
		ExpiresAt:  now.Add(ttl),
	}

	for _, opt := range opts {
		opt(session)
	}

	data, err := json.Marshal(session)
	if err != nil {
		return nil, SerializationWrap("failed to marshal session", err)
	}

	sessionKey := s.sessionKey(session.ID)
	userSessionsKey := s.userSessionsKey(userID)

	// Get current TTL of user sessions index to avoid overwriting with shorter TTL
	currentTTL, err := s.client.client.TTL(ctx, userSessionsKey).Result()
	if err != nil {
		s.logger.Error("failed to get user sessions index TTL", "user_id", userID, "error", err)
		return nil, FromRedisError(err)
	}

	// Use the maximum of current TTL and new session TTL to preserve longer-lived sessions
	indexTTL := ttl
	if currentTTL > 0 && currentTTL > ttl {
		indexTTL = currentTTL
	}

	pipe := s.client.client.Pipeline()
	pipe.Set(ctx, sessionKey, data, ttl)
	pipe.SAdd(ctx, userSessionsKey, session.ID)
	pipe.Expire(ctx, userSessionsKey, indexTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("session create error", "session_id", session.ID, "user_id", userID, "error", err)
		return nil, FromRedisError(err)
	}

	s.logger.Debug("session created", "session_id", session.ID, "user_id", userID, "ttl", ttl)
	return session, nil
}

// Get retrieves a session by ID.
// Updates last_active time on successful retrieval.
func (s *SessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	return s.GetWithTouch(ctx, sessionID, true)
}

// GetWithTouch retrieves a session by ID, optionally updating last_active.
func (s *SessionStore) GetWithTouch(ctx context.Context, sessionID string, touch bool) (*Session, error) {
	sessionKey := s.sessionKey(sessionID)

	data, err := s.client.client.Get(ctx, sessionKey).Bytes()
	if err != nil {
		redisErr := FromRedisError(err)
		if IsNotFound(redisErr) {
			return nil, NotFound("session not found")
		}
		s.logger.Error("session get error", "session_id", sessionID, "error", err)
		return nil, redisErr
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, SerializationWrap("failed to unmarshal session", err)
	}

	if touch {
		session.LastActive = time.Now()
		if updateErr := s.update(ctx, &session); updateErr != nil {
			s.logger.Warn("failed to update session last_active", "session_id", sessionID, "error", updateErr)
		}
	}

	return &session, nil
}

// update saves changes to an existing session.
func (s *SessionStore) update(ctx context.Context, session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return SerializationWrap("failed to marshal session", err)
	}

	sessionKey := s.sessionKey(session.ID)
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		ttl = s.defaultTTL
		session.ExpiresAt = time.Now().Add(ttl)
	}

	err = s.client.client.Set(ctx, sessionKey, data, ttl).Err()
	if err != nil {
		return FromRedisError(err)
	}

	return nil
}

// Update updates a session with new data.
func (s *SessionStore) Update(ctx context.Context, session *Session) error {
	session.LastActive = time.Now()
	return s.update(ctx, session)
}

// Delete deletes a session by ID.
func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	// First get the session to find the user ID
	session, err := s.GetWithTouch(ctx, sessionID, false)
	if err != nil {
		if IsNotFound(err) {
			return nil // Already deleted
		}
		return err
	}

	sessionKey := s.sessionKey(sessionID)
	userSessionsKey := s.userSessionsKey(session.UserID)

	pipe := s.client.client.Pipeline()
	pipe.Del(ctx, sessionKey)
	pipe.SRem(ctx, userSessionsKey, sessionID)
	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("session delete error", "session_id", sessionID, "error", err)
		return FromRedisError(err)
	}

	s.logger.Debug("session deleted", "session_id", sessionID)
	return nil
}

// DeleteByUserID deletes all sessions for a user.
func (s *SessionStore) DeleteByUserID(ctx context.Context, userID string) (int64, error) {
	userSessionsKey := s.userSessionsKey(userID)

	// Get all session IDs for the user
	sessionIDs, err := s.client.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return 0, FromRedisError(err)
	}

	if len(sessionIDs) == 0 {
		return 0, nil
	}

	// Build keys to delete
	keys := make([]string, len(sessionIDs)+1)
	for i, id := range sessionIDs {
		keys[i] = s.sessionKey(id)
	}
	keys[len(sessionIDs)] = userSessionsKey

	deleted, err := s.client.client.Del(ctx, keys...).Result()
	if err != nil {
		s.logger.Error("session delete by user error", "user_id", userID, "error", err)
		return 0, FromRedisError(err)
	}

	s.logger.Debug("sessions deleted by user", "user_id", userID, "count", deleted)
	return deleted, nil
}

// ListByUserID returns all sessions for a user.
func (s *SessionStore) ListByUserID(ctx context.Context, userID string) ([]*Session, error) {
	userSessionsKey := s.userSessionsKey(userID)

	sessionIDs, err := s.client.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return nil, FromRedisError(err)
	}

	if len(sessionIDs) == 0 {
		return []*Session{}, nil
	}

	// Build session keys
	keys := make([]string, len(sessionIDs))
	for i, id := range sessionIDs {
		keys[i] = s.sessionKey(id)
	}

	// Get all sessions
	results, err := s.client.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, FromRedisError(err)
	}

	sessions := make([]*Session, 0, len(results))
	expiredIDs := make([]string, 0)

	for i, result := range results {
		if result == nil {
			// Session expired but still in index
			expiredIDs = append(expiredIDs, sessionIDs[i])
			continue
		}

		data, ok := result.(string)
		if !ok {
			continue
		}

		var session Session
		if err := json.Unmarshal([]byte(data), &session); err != nil {
			s.logger.Warn("failed to unmarshal session", "session_id", sessionIDs[i], "error", err)
			continue
		}

		sessions = append(sessions, &session)
	}

	// Clean up expired session IDs from the index
	if len(expiredIDs) > 0 {
		go func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			args := make([]any, len(expiredIDs))
			for i, id := range expiredIDs {
				args[i] = id
			}
			//nolint:errcheck,gosec // Best-effort cleanup of expired session references.
			s.client.client.SRem(cleanupCtx, userSessionsKey, args...).Err()
		}()
	}

	return sessions, nil
}

// Exists checks if a session exists.
func (s *SessionStore) Exists(ctx context.Context, sessionID string) (bool, error) {
	sessionKey := s.sessionKey(sessionID)
	count, err := s.client.client.Exists(ctx, sessionKey).Result()
	if err != nil {
		return false, FromRedisError(err)
	}
	return count > 0, nil
}

// Extend extends the session TTL.
func (s *SessionStore) Extend(ctx context.Context, sessionID string, ttl time.Duration) error {
	session, err := s.GetWithTouch(ctx, sessionID, false)
	if err != nil {
		return err
	}

	session.ExpiresAt = time.Now().Add(ttl)
	session.LastActive = time.Now()

	return s.update(ctx, session)
}

// Count returns the number of active sessions for a user.
func (s *SessionStore) Count(ctx context.Context, userID string) (int64, error) {
	userSessionsKey := s.userSessionsKey(userID)
	count, err := s.client.client.SCard(ctx, userSessionsKey).Result()
	if err != nil {
		return 0, FromRedisError(err)
	}
	return count, nil
}
