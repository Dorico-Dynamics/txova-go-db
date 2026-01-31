package redis

import (
	"errors"
	"testing"

	coreerrors "github.com/Dorico-Dynamics/txova-go-core/errors"
	"github.com/redis/go-redis/v9"
)

func TestCode_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code Code
		want string
	}{
		{CodeNotFound, "REDIS_NOT_FOUND"},
		{CodeConnection, "REDIS_CONNECTION"},
		{CodeTimeout, "REDIS_TIMEOUT"},
		{CodeLockFailed, "REDIS_LOCK_FAILED"},
		{CodeLockNotHeld, "REDIS_LOCK_NOT_HELD"},
		{CodeRateLimited, "REDIS_RATE_LIMITED"},
		{CodeSerialization, "REDIS_SERIALIZATION"},
		{CodeInternal, "REDIS_INTERNAL"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.code.String(); got != tt.want {
				t.Errorf("Code.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCode_CoreCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code     Code
		coreCode coreerrors.Code
	}{
		{CodeNotFound, coreerrors.CodeNotFound},
		{CodeConnection, coreerrors.CodeServiceUnavailable},
		{CodeTimeout, coreerrors.CodeServiceUnavailable},
		{CodeLockFailed, coreerrors.CodeConflict},
		{CodeLockNotHeld, coreerrors.CodeConflict},
		{CodeRateLimited, coreerrors.CodeRateLimited},
		{CodeSerialization, coreerrors.CodeValidationError},
		{CodeInternal, coreerrors.CodeInternalError},
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			t.Parallel()
			if got := tt.code.CoreCode(); got != tt.coreCode {
				t.Errorf("CoreCode() = %v, want %v", got, tt.coreCode)
			}
		})
	}
}

func TestCode_CoreCode_Unknown(t *testing.T) {
	t.Parallel()

	unknownCode := Code("UNKNOWN")
	if got := unknownCode.CoreCode(); got != coreerrors.CodeInternalError {
		t.Errorf("CoreCode() for unknown = %v, want %v", got, coreerrors.CodeInternalError)
	}
}

func TestNewError(t *testing.T) {
	t.Parallel()

	err := NewError(CodeNotFound, "key not found")

	if err.Code() != CodeNotFound {
		t.Errorf("Code() = %v, want %v", err.Code(), CodeNotFound)
	}
	if err.Message() != "key not found" {
		t.Errorf("Message() = %q, want %q", err.Message(), "key not found")
	}
	if err.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", err.Unwrap())
	}
}

func TestNewErrorf(t *testing.T) {
	t.Parallel()

	err := NewErrorf(CodeNotFound, "key %s not found", "test-key")

	if err.Code() != CodeNotFound {
		t.Errorf("Code() = %v, want %v", err.Code(), CodeNotFound)
	}
	if err.Message() != "key test-key not found" {
		t.Errorf("Message() = %q, want %q", err.Message(), "key test-key not found")
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("connection refused")
	err := Wrap(CodeConnection, "failed to connect", cause)

	if err.Code() != CodeConnection {
		t.Errorf("Code() = %v, want %v", err.Code(), CodeConnection)
	}
	if err.Message() != "failed to connect" {
		t.Errorf("Message() = %q, want %q", err.Message(), "failed to connect")
	}
	if err.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
	}
}

func TestWrapf(t *testing.T) {
	t.Parallel()

	cause := errors.New("timeout")
	err := Wrapf(CodeTimeout, cause, "operation %s timed out", "GET")

	if err.Code() != CodeTimeout {
		t.Errorf("Code() = %v, want %v", err.Code(), CodeTimeout)
	}
	if err.Message() != "operation GET timed out" {
		t.Errorf("Message() = %q, want %q", err.Message(), "operation GET timed out")
	}
}

func TestError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "without cause",
			err:  NewError(CodeNotFound, "key not found"),
			want: "REDIS_NOT_FOUND: key not found",
		},
		{
			name: "with cause",
			err:  Wrap(CodeConnection, "failed to connect", errors.New("timeout")),
			want: "REDIS_CONNECTION: failed to connect: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestError_Is(t *testing.T) {
	t.Parallel()

	err1 := NewError(CodeNotFound, "key not found")
	err2 := NewError(CodeNotFound, "another key not found")
	err3 := NewError(CodeConnection, "connection error")

	if !errors.Is(err1, err2) {
		t.Error("errors.Is(err1, err2) should be true for same code")
	}
	if errors.Is(err1, err3) {
		t.Error("errors.Is(err1, err3) should be false for different code")
	}
}

func TestFromRedisError(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()
		if got := FromRedisError(nil); got != nil {
			t.Errorf("FromRedisError(nil) = %v, want nil", got)
		}
	})

	t.Run("redis.Nil", func(t *testing.T) {
		t.Parallel()
		got := FromRedisError(redis.Nil)
		if got.Code() != CodeNotFound {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeNotFound)
		}
	})

	t.Run("redis.ErrClosed", func(t *testing.T) {
		t.Parallel()
		got := FromRedisError(redis.ErrClosed)
		if got.Code() != CodeConnection {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeConnection)
		}
	})

	t.Run("connection refused", func(t *testing.T) {
		t.Parallel()
		got := FromRedisError(errors.New("dial tcp: connection refused"))
		if got.Code() != CodeConnection {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeConnection)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		got := FromRedisError(errors.New("i/o timeout"))
		if got.Code() != CodeTimeout {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeTimeout)
		}
	})

	t.Run("context deadline exceeded", func(t *testing.T) {
		t.Parallel()
		got := FromRedisError(errors.New("context deadline exceeded"))
		if got.Code() != CodeTimeout {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeTimeout)
		}
	})

	t.Run("unknown error", func(t *testing.T) {
		t.Parallel()
		got := FromRedisError(errors.New("some unknown error"))
		if got.Code() != CodeInternal {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeInternal)
		}
	})
}

func TestConvenienceConstructors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *Error
		wantCode Code
		wantMsg  string
	}{
		{"NotFound", NotFound("not found"), CodeNotFound, "not found"},
		{"NotFoundf", NotFoundf("key %s not found", "x"), CodeNotFound, "key x not found"},
		{"Connection", Connection("connection error"), CodeConnection, "connection error"},
		{"Timeout", Timeout("timeout"), CodeTimeout, "timeout"},
		{"LockFailed", LockFailed("lock failed"), CodeLockFailed, "lock failed"},
		{"LockNotHeld", LockNotHeld("lock not held"), CodeLockNotHeld, "lock not held"},
		{"RateLimited", RateLimited("rate limited"), CodeRateLimited, "rate limited"},
		{"Serialization", Serialization("serialization error"), CodeSerialization, "serialization error"},
		{"Internal", Internal("internal error"), CodeInternal, "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err.Code() != tt.wantCode {
				t.Errorf("Code() = %v, want %v", tt.err.Code(), tt.wantCode)
			}
			if tt.err.Message() != tt.wantMsg {
				t.Errorf("Message() = %q, want %q", tt.err.Message(), tt.wantMsg)
			}
		})
	}
}

func TestWrapConstructors(t *testing.T) {
	t.Parallel()

	cause := errors.New("underlying")

	tests := []struct {
		name     string
		err      *Error
		wantCode Code
	}{
		{"ConnectionWrap", ConnectionWrap("conn error", cause), CodeConnection},
		{"TimeoutWrap", TimeoutWrap("timeout", cause), CodeTimeout},
		{"SerializationWrap", SerializationWrap("serial error", cause), CodeSerialization},
		{"InternalWrap", InternalWrap("internal", cause), CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err.Code() != tt.wantCode {
				t.Errorf("Code() = %v, want %v", tt.err.Code(), tt.wantCode)
			}
			if tt.err.Unwrap() != cause {
				t.Errorf("Unwrap() = %v, want %v", tt.err.Unwrap(), cause)
			}
		})
	}
}

func TestIsError(t *testing.T) {
	t.Parallel()

	redisErr := NewError(CodeNotFound, "not found")
	stdErr := errors.New("standard error")
	wrapped := errors.Join(errors.New("wrapper"), redisErr)

	if !IsError(redisErr) {
		t.Error("IsError(redisErr) should be true")
	}
	if IsError(stdErr) {
		t.Error("IsError(stdErr) should be false")
	}
	if !IsError(wrapped) {
		t.Error("IsError(wrapped) should be true")
	}
	if IsError(nil) {
		t.Error("IsError(nil) should be false")
	}
}

func TestAsError(t *testing.T) {
	t.Parallel()

	redisErr := NewError(CodeNotFound, "not found")
	stdErr := errors.New("standard error")

	if got := AsError(redisErr); got != redisErr {
		t.Errorf("AsError(redisErr) = %v, want %v", got, redisErr)
	}
	if got := AsError(stdErr); got != nil {
		t.Errorf("AsError(stdErr) = %v, want nil", got)
	}
	if got := AsError(nil); got != nil {
		t.Errorf("AsError(nil) = %v, want nil", got)
	}
}

func TestGetCode(t *testing.T) {
	t.Parallel()

	redisErr := NewError(CodeNotFound, "not found")
	stdErr := errors.New("standard error")

	if got := GetCode(redisErr); got != CodeNotFound {
		t.Errorf("GetCode(redisErr) = %v, want %v", got, CodeNotFound)
	}
	if got := GetCode(stdErr); got != CodeInternal {
		t.Errorf("GetCode(stdErr) = %v, want %v", got, CodeInternal)
	}
}

func TestIsCode(t *testing.T) {
	t.Parallel()

	redisErr := NewError(CodeNotFound, "not found")
	stdErr := errors.New("standard error")

	if !IsCode(redisErr, CodeNotFound) {
		t.Error("IsCode(redisErr, CodeNotFound) should be true")
	}
	if IsCode(redisErr, CodeConnection) {
		t.Error("IsCode(redisErr, CodeConnection) should be false")
	}
	if IsCode(stdErr, CodeNotFound) {
		t.Error("IsCode(stdErr, CodeNotFound) should be false for non-redis error")
	}
	if IsCode(nil, CodeNotFound) {
		t.Error("IsCode(nil, CodeNotFound) should be false")
	}
}

func TestIsHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    *Error
		checks map[string]bool
	}{
		{
			name: "not found",
			err:  NewError(CodeNotFound, "not found"),
			checks: map[string]bool{
				"IsNotFound": true, "IsConnection": false, "IsTimeout": false,
				"IsLockFailed": false, "IsLockNotHeld": false, "IsRateLimited": false, "IsSerialization": false,
			},
		},
		{
			name: "connection",
			err:  NewError(CodeConnection, "connection"),
			checks: map[string]bool{
				"IsNotFound": false, "IsConnection": true, "IsTimeout": false,
				"IsLockFailed": false, "IsLockNotHeld": false, "IsRateLimited": false, "IsSerialization": false,
			},
		},
		{
			name: "timeout",
			err:  NewError(CodeTimeout, "timeout"),
			checks: map[string]bool{
				"IsNotFound": false, "IsConnection": false, "IsTimeout": true,
				"IsLockFailed": false, "IsLockNotHeld": false, "IsRateLimited": false, "IsSerialization": false,
			},
		},
		{
			name: "lock failed",
			err:  NewError(CodeLockFailed, "lock failed"),
			checks: map[string]bool{
				"IsNotFound": false, "IsConnection": false, "IsTimeout": false,
				"IsLockFailed": true, "IsLockNotHeld": false, "IsRateLimited": false, "IsSerialization": false,
			},
		},
		{
			name: "lock not held",
			err:  NewError(CodeLockNotHeld, "lock not held"),
			checks: map[string]bool{
				"IsNotFound": false, "IsConnection": false, "IsTimeout": false,
				"IsLockFailed": false, "IsLockNotHeld": true, "IsRateLimited": false, "IsSerialization": false,
			},
		},
		{
			name: "rate limited",
			err:  NewError(CodeRateLimited, "rate limited"),
			checks: map[string]bool{
				"IsNotFound": false, "IsConnection": false, "IsTimeout": false,
				"IsLockFailed": false, "IsLockNotHeld": false, "IsRateLimited": true, "IsSerialization": false,
			},
		},
		{
			name: "serialization",
			err:  NewError(CodeSerialization, "serialization"),
			checks: map[string]bool{
				"IsNotFound": false, "IsConnection": false, "IsTimeout": false,
				"IsLockFailed": false, "IsLockNotHeld": false, "IsRateLimited": false, "IsSerialization": true,
			},
		},
	}

	checkers := map[string]func(error) bool{
		"IsNotFound":      IsNotFound,
		"IsConnection":    IsConnection,
		"IsTimeout":       IsTimeout,
		"IsLockFailed":    IsLockFailed,
		"IsLockNotHeld":   IsLockNotHeld,
		"IsRateLimited":   IsRateLimited,
		"IsSerialization": IsSerialization,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for checkName, expected := range tt.checks {
				checker := checkers[checkName]
				if got := checker(tt.err); got != expected {
					t.Errorf("%s() = %v, want %v", checkName, got, expected)
				}
			}
		})
	}
}

func TestCoreErrorIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Redis NotFound works with core IsNotFound", func(t *testing.T) {
		t.Parallel()
		redisErr := NotFound("key not found")
		if !coreerrors.IsNotFound(redisErr) {
			t.Error("coreerrors.IsNotFound(redisErr) should be true")
		}
	})

	t.Run("Redis Connection works with core IsServiceUnavailable", func(t *testing.T) {
		t.Parallel()
		redisErr := Connection("connection error")
		if !coreerrors.IsServiceUnavailable(redisErr) {
			t.Error("coreerrors.IsServiceUnavailable(redisErr) should be true")
		}
	})

	t.Run("Redis Timeout works with core IsServiceUnavailable", func(t *testing.T) {
		t.Parallel()
		redisErr := Timeout("timeout")
		if !coreerrors.IsServiceUnavailable(redisErr) {
			t.Error("coreerrors.IsServiceUnavailable(redisErr) should be true")
		}
	})

	t.Run("Redis RateLimited works with core IsRateLimited", func(t *testing.T) {
		t.Parallel()
		redisErr := RateLimited("rate limited")
		if !coreerrors.IsRateLimited(redisErr) {
			t.Error("coreerrors.IsRateLimited(redisErr) should be true")
		}
	})

	t.Run("Redis Internal works with core IsInternalError", func(t *testing.T) {
		t.Parallel()
		redisErr := Internal("internal error")
		if !coreerrors.IsInternalError(redisErr) {
			t.Error("coreerrors.IsInternalError(redisErr) should be true")
		}
	})
}

func TestContainsIgnoreCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"Connection Refused", "connection refused", true},
		{"TIMEOUT", "timeout", true},
		{"hello world", "WORLD", true},
		{"hello", "goodbye", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			t.Parallel()
			if got := containsIgnoreCase(tt.s, tt.substr); got != tt.want {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}
