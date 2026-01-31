// Package redis provides Redis client utilities for the Txova platform.
package redis

import (
	"errors"
	"fmt"

	coreerrors "github.com/Dorico-Dynamics/txova-go-core/errors"
	"github.com/redis/go-redis/v9"
)

// Code represents a Redis-specific error code.
type Code string

// Redis error codes.
const (
	// CodeNotFound indicates the requested key was not found.
	CodeNotFound Code = "REDIS_NOT_FOUND"
	// CodeConnection indicates a connection error.
	CodeConnection Code = "REDIS_CONNECTION"
	// CodeTimeout indicates a timeout error.
	CodeTimeout Code = "REDIS_TIMEOUT"
	// CodeLockFailed indicates a lock acquisition failure.
	CodeLockFailed Code = "REDIS_LOCK_FAILED"
	// CodeLockNotHeld indicates the lock is not held by the caller.
	CodeLockNotHeld Code = "REDIS_LOCK_NOT_HELD"
	// CodeRateLimited indicates rate limit exceeded.
	CodeRateLimited Code = "REDIS_RATE_LIMITED"
	// CodeSerialization indicates a serialization/deserialization error.
	CodeSerialization Code = "REDIS_SERIALIZATION"
	// CodeInternal indicates an internal Redis error.
	CodeInternal Code = "REDIS_INTERNAL"
)

// String returns the string representation of the error code.
func (c Code) String() string {
	return string(c)
}

// coreCodeMapping maps Redis codes to core application codes.
var coreCodeMapping = map[Code]coreerrors.Code{
	CodeNotFound:      coreerrors.CodeNotFound,
	CodeConnection:    coreerrors.CodeServiceUnavailable,
	CodeTimeout:       coreerrors.CodeServiceUnavailable,
	CodeLockFailed:    coreerrors.CodeConflict,
	CodeLockNotHeld:   coreerrors.CodeConflict,
	CodeRateLimited:   coreerrors.CodeRateLimited,
	CodeSerialization: coreerrors.CodeValidationError,
	CodeInternal:      coreerrors.CodeInternalError,
}

// CoreCode returns the corresponding core.Code for this Redis error code.
func (c Code) CoreCode() coreerrors.Code {
	if coreCode, ok := coreCodeMapping[c]; ok {
		return coreCode
	}
	return coreerrors.CodeInternalError
}

// Error represents a Redis-specific error with detailed context.
type Error struct {
	*coreerrors.AppError
	code Code
}

// NewError creates a new Redis Error with the given code and message.
func NewError(code Code, message string) *Error {
	return &Error{
		AppError: coreerrors.New(code.CoreCode(), message),
		code:     code,
	}
}

// NewErrorf creates a new Redis Error with a formatted message.
func NewErrorf(code Code, format string, args ...any) *Error {
	return NewError(code, fmt.Sprintf(format, args...))
}

// Wrap creates a new Redis Error that wraps an existing error.
func Wrap(code Code, message string, cause error) *Error {
	return &Error{
		AppError: coreerrors.Wrap(code.CoreCode(), message, cause),
		code:     code,
	}
}

// Wrapf creates a new Redis Error that wraps an existing error with a formatted message.
func Wrapf(code Code, cause error, format string, args ...any) *Error {
	return Wrap(code, fmt.Sprintf(format, args...), cause)
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Unwrap() != nil {
		return fmt.Sprintf("%s: %s: %v", e.code, e.Message(), e.Unwrap())
	}
	return fmt.Sprintf("%s: %s", e.code, e.Message())
}

// Code returns the Redis-specific error code.
func (e *Error) Code() Code {
	return e.code
}

// Is implements errors.Is for comparing errors by code.
func (e *Error) Is(target error) bool {
	if e.AppError.Is(target) {
		return true
	}
	var redisErr *Error
	if errors.As(target, &redisErr) {
		return e.code == redisErr.code
	}
	return false
}

// As implements errors.As to allow extraction of the embedded AppError.
func (e *Error) As(target any) bool {
	switch t := target.(type) {
	case **coreerrors.AppError:
		*t = e.AppError
		return true
	case **Error:
		*t = e
		return true
	}
	return false
}

// FromRedisError converts a go-redis error to a Redis Error.
func FromRedisError(err error) *Error {
	if err == nil {
		return nil
	}

	// Check for redis.Nil (key not found)
	if errors.Is(err, redis.Nil) {
		return NewError(CodeNotFound, "key not found")
	}

	// Check for context errors (timeout/cancellation)
	if errors.Is(err, redis.ErrClosed) {
		return Wrap(CodeConnection, "connection closed", err)
	}

	// Check error message for common patterns
	errMsg := err.Error()

	// Connection errors
	if isConnectionError(errMsg) {
		return Wrap(CodeConnection, "Redis connection error", err)
	}

	// Timeout errors
	if isTimeoutError(errMsg) {
		return Wrap(CodeTimeout, "Redis operation timeout", err)
	}

	// Default to internal error
	return Wrap(CodeInternal, "Redis operation failed", err)
}

// isConnectionError checks if the error message indicates a connection error.
func isConnectionError(msg string) bool {
	connectionPatterns := []string{
		"connection refused",
		"connection reset",
		"no connection",
		"connect:",
		"dial tcp",
		"EOF",
		"broken pipe",
	}
	for _, pattern := range connectionPatterns {
		if containsIgnoreCase(msg, pattern) {
			return true
		}
	}
	return false
}

// isTimeoutError checks if the error message indicates a timeout error.
func isTimeoutError(msg string) bool {
	timeoutPatterns := []string{
		"timeout",
		"deadline exceeded",
		"context deadline",
		"i/o timeout",
	}
	for _, pattern := range timeoutPatterns {
		if containsIgnoreCase(msg, pattern) {
			return true
		}
	}
	return false
}

// containsIgnoreCase checks if s contains substr (case insensitive).
func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

// toLower is a simple lowercase conversion.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Convenience constructors

// NotFound creates a not found error.
func NotFound(message string) *Error {
	return NewError(CodeNotFound, message)
}

// NotFoundf creates a not found error with a formatted message.
func NotFoundf(format string, args ...any) *Error {
	return NewErrorf(CodeNotFound, format, args...)
}

// Connection creates a connection error.
func Connection(message string) *Error {
	return NewError(CodeConnection, message)
}

// ConnectionWrap creates a connection error wrapping a cause.
func ConnectionWrap(message string, cause error) *Error {
	return Wrap(CodeConnection, message, cause)
}

// Timeout creates a timeout error.
func Timeout(message string) *Error {
	return NewError(CodeTimeout, message)
}

// TimeoutWrap creates a timeout error wrapping a cause.
func TimeoutWrap(message string, cause error) *Error {
	return Wrap(CodeTimeout, message, cause)
}

// LockFailed creates a lock acquisition failure error.
func LockFailed(message string) *Error {
	return NewError(CodeLockFailed, message)
}

// LockNotHeld creates a lock not held error.
func LockNotHeld(message string) *Error {
	return NewError(CodeLockNotHeld, message)
}

// RateLimited creates a rate limited error.
func RateLimited(message string) *Error {
	return NewError(CodeRateLimited, message)
}

// Serialization creates a serialization error.
func Serialization(message string) *Error {
	return NewError(CodeSerialization, message)
}

// SerializationWrap creates a serialization error wrapping a cause.
func SerializationWrap(message string, cause error) *Error {
	return Wrap(CodeSerialization, message, cause)
}

// Internal creates an internal error.
func Internal(message string) *Error {
	return NewError(CodeInternal, message)
}

// InternalWrap creates an internal error wrapping a cause.
func InternalWrap(message string, cause error) *Error {
	return Wrap(CodeInternal, message, cause)
}

// Error checking helpers

// IsError checks if the given error is a Redis Error.
func IsError(err error) bool {
	var redisErr *Error
	return errors.As(err, &redisErr)
}

// AsError returns the error as a Redis Error, or nil if it's not one.
func AsError(err error) *Error {
	var redisErr *Error
	if errors.As(err, &redisErr) {
		return redisErr
	}
	return nil
}

// GetCode returns the Redis error code from an error, or CodeInternal if not a Redis error.
func GetCode(err error) Code {
	if redisErr := AsError(err); redisErr != nil {
		return redisErr.Code()
	}
	return CodeInternal
}

// IsCode checks if the given error is a Redis Error with the specified code.
func IsCode(err error, code Code) bool {
	if redisErr := AsError(err); redisErr != nil {
		return redisErr.Code() == code
	}
	return false
}

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return IsCode(err, CodeNotFound)
}

// IsConnection checks if the error is a connection error.
func IsConnection(err error) bool {
	return IsCode(err, CodeConnection)
}

// IsTimeout checks if the error is a timeout error.
func IsTimeout(err error) bool {
	return IsCode(err, CodeTimeout)
}

// IsLockFailed checks if the error is a lock acquisition failure.
func IsLockFailed(err error) bool {
	return IsCode(err, CodeLockFailed)
}

// IsLockNotHeld checks if the error is a lock not held error.
func IsLockNotHeld(err error) bool {
	return IsCode(err, CodeLockNotHeld)
}

// IsRateLimited checks if the error is a rate limited error.
func IsRateLimited(err error) bool {
	return IsCode(err, CodeRateLimited)
}

// IsSerialization checks if the error is a serialization error.
func IsSerialization(err error) bool {
	return IsCode(err, CodeSerialization)
}
