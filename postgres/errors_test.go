package postgres

import (
	"errors"
	"testing"

	coreerrors "github.com/Dorico-Dynamics/txova-go-core/errors"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCode_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code Code
		want string
	}{
		{CodeNotFound, "DB_NOT_FOUND"},
		{CodeDuplicate, "DB_DUPLICATE"},
		{CodeForeignKey, "DB_FOREIGN_KEY"},
		{CodeCheckViolation, "DB_CHECK_VIOLATION"},
		{CodeConnection, "DB_CONNECTION"},
		{CodeTimeout, "DB_TIMEOUT"},
		{CodeSerialization, "DB_SERIALIZATION"},
		{CodeDeadlock, "DB_DEADLOCK"},
		{CodeInvalidInput, "DB_INVALID_INPUT"},
		{CodeInternal, "DB_INTERNAL"},
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

func TestNew(t *testing.T) {
	t.Parallel()

	err := New(CodeNotFound, "user not found")

	if err.Code() != CodeNotFound {
		t.Errorf("Code() = %v, want %v", err.Code(), CodeNotFound)
	}
	if err.Message() != "user not found" {
		t.Errorf("Message() = %q, want %q", err.Message(), "user not found")
	}
	if err.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", err.Unwrap())
	}
	if err.SQLState() != "" {
		t.Errorf("SQLState() = %q, want empty", err.SQLState())
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

func TestError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "without cause",
			err:  New(CodeNotFound, "resource not found"),
			want: "DB_NOT_FOUND: resource not found",
		},
		{
			name: "with cause",
			err:  Wrap(CodeConnection, "failed to connect", errors.New("timeout")),
			want: "DB_CONNECTION: failed to connect: timeout",
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

	err1 := New(CodeNotFound, "user not found")
	err2 := New(CodeNotFound, "order not found")
	err3 := New(CodeDuplicate, "user exists")

	if !errors.Is(err1, err2) {
		t.Error("errors.Is(err1, err2) should be true for same code")
	}
	if errors.Is(err1, err3) {
		t.Error("errors.Is(err1, err3) should be false for different code")
	}
}

func TestError_WithMessage(t *testing.T) {
	t.Parallel()

	original := New(CodeNotFound, "original message")
	modified := original.WithMessage("new message")

	if modified.Message() != "new message" {
		t.Errorf("Message() = %q, want %q", modified.Message(), "new message")
	}
	if modified.Code() != original.Code() {
		t.Errorf("Code() = %v, want %v", modified.Code(), original.Code())
	}
	if original.Message() != "original message" {
		t.Error("original should not be modified")
	}
}

func TestError_WithCause(t *testing.T) {
	t.Parallel()

	original := New(CodeNotFound, "not found")
	cause := errors.New("underlying error")
	modified := original.WithCause(cause)

	if modified.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want %v", modified.Unwrap(), cause)
	}
	if original.Unwrap() != nil {
		t.Error("original should not be modified")
	}
}

func TestMapSQLState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sqlState string
		want     Code
	}{
		{"unique violation", "23505", CodeDuplicate},
		{"foreign key violation", "23503", CodeForeignKey},
		{"check violation", "23514", CodeCheckViolation},
		{"not null violation", "23502", CodeInvalidInput},
		{"serialization failure", "40001", CodeSerialization},
		{"deadlock detected", "40P01", CodeDeadlock},
		{"query canceled", "57014", CodeTimeout},
		{"connection exception", "08000", CodeConnection},
		{"connection exception 08001", "08001", CodeConnection},
		{"unknown code", "99999", CodeInternal},
		{"empty code", "", CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mapSQLState(tt.sqlState); got != tt.want {
				t.Errorf("mapSQLState(%q) = %v, want %v", tt.sqlState, got, tt.want)
			}
		})
	}
}

func TestFromPgError(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()
		if got := FromPgError(nil); got != nil {
			t.Errorf("FromPgError(nil) = %v, want nil", got)
		}
	})

	t.Run("non-pg error", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("some error")
		got := FromPgError(cause)

		if got.Code() != CodeInternal {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeInternal)
		}
		if got.Unwrap() != cause {
			t.Errorf("Unwrap() = %v, want %v", got.Unwrap(), cause)
		}
	})

	t.Run("pg unique violation", func(t *testing.T) {
		t.Parallel()
		pgErr := &pgconn.PgError{
			Code:           "23505",
			Message:        "duplicate key value violates unique constraint",
			Detail:         "Key (email)=(test@example.com) already exists.",
			TableName:      "users",
			ColumnName:     "email",
			ConstraintName: "users_email_key",
		}

		got := FromPgError(pgErr)

		if got.Code() != CodeDuplicate {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeDuplicate)
		}
		if got.SQLState() != "23505" {
			t.Errorf("SQLState() = %q, want %q", got.SQLState(), "23505")
		}
		if got.TableName() != "users" {
			t.Errorf("TableName() = %q, want %q", got.TableName(), "users")
		}
		if got.Column() != "email" {
			t.Errorf("Column() = %q, want %q", got.Column(), "email")
		}
		if got.Constraint() != "users_email_key" {
			t.Errorf("Constraint() = %q, want %q", got.Constraint(), "users_email_key")
		}
		if got.Detail() != "Key (email)=(test@example.com) already exists." {
			t.Errorf("Detail() = %q, want expected detail", got.Detail())
		}
	})

	t.Run("pg foreign key violation", func(t *testing.T) {
		t.Parallel()
		pgErr := &pgconn.PgError{
			Code:           "23503",
			Message:        "foreign key constraint violation",
			ConstraintName: "orders_user_id_fkey",
		}

		got := FromPgError(pgErr)

		if got.Code() != CodeForeignKey {
			t.Errorf("Code() = %v, want %v", got.Code(), CodeForeignKey)
		}
	})
}

func TestIsError(t *testing.T) {
	t.Parallel()

	dbErr := New(CodeNotFound, "not found")
	stdErr := errors.New("standard error")
	wrapped := errors.Join(errors.New("wrapper"), dbErr)

	if !IsError(dbErr) {
		t.Error("IsError(dbErr) should be true")
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

	dbErr := New(CodeNotFound, "not found")
	stdErr := errors.New("standard error")

	if got := AsError(dbErr); got != dbErr {
		t.Errorf("AsError(dbErr) = %v, want %v", got, dbErr)
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

	dbErr := New(CodeDuplicate, "duplicate")
	stdErr := errors.New("standard error")

	if got := GetCode(dbErr); got != CodeDuplicate {
		t.Errorf("GetCode(dbErr) = %v, want %v", got, CodeDuplicate)
	}
	if got := GetCode(stdErr); got != CodeInternal {
		t.Errorf("GetCode(stdErr) = %v, want %v", got, CodeInternal)
	}
}

func TestIsCode(t *testing.T) {
	t.Parallel()

	dbErr := New(CodeNotFound, "not found")
	stdErr := errors.New("standard error")

	if !IsCode(dbErr, CodeNotFound) {
		t.Error("IsCode(dbErr, CodeNotFound) should be true")
	}
	if IsCode(dbErr, CodeDuplicate) {
		t.Error("IsCode(dbErr, CodeDuplicate) should be false")
	}
	if IsCode(stdErr, CodeNotFound) {
		t.Error("IsCode(stdErr, CodeNotFound) should be false for non-db error")
	}
	if IsCode(nil, CodeNotFound) {
		t.Error("IsCode(nil, CodeNotFound) should be false")
	}
}

func TestErrorCoreCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *Error
		wantCode coreerrors.Code
	}{
		{"NotFound", New(CodeNotFound, "not found"), coreerrors.CodeNotFound},
		{"Duplicate", New(CodeDuplicate, "duplicate"), coreerrors.CodeConflict},
		{"Connection", New(CodeConnection, "connection"), coreerrors.CodeServiceUnavailable},
		{"Internal", New(CodeInternal, "internal"), coreerrors.CodeInternalError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.CoreCode(); got != tt.wantCode {
				t.Errorf("CoreCode() = %v, want %v", got, tt.wantCode)
			}
		})
	}
}

func TestCodeCoreCodeUnknown(t *testing.T) {
	t.Parallel()

	// Test that unknown codes default to CodeInternalError
	unknownCode := Code("UNKNOWN_CODE")
	if got := unknownCode.CoreCode(); got != coreerrors.CodeInternalError {
		t.Errorf("CoreCode() for unknown code = %v, want %v", got, coreerrors.CodeInternalError)
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
			err:  New(CodeNotFound, "not found"),
			checks: map[string]bool{
				"IsNotFound": true, "IsDuplicate": false, "IsForeignKey": false,
				"IsConnection": false, "IsTimeout": false, "IsSerialization": false, "IsDeadlock": false,
			},
		},
		{
			name: "duplicate",
			err:  New(CodeDuplicate, "duplicate"),
			checks: map[string]bool{
				"IsNotFound": false, "IsDuplicate": true, "IsForeignKey": false,
				"IsConnection": false, "IsTimeout": false, "IsSerialization": false, "IsDeadlock": false,
			},
		},
		{
			name: "foreign key",
			err:  New(CodeForeignKey, "fk violation"),
			checks: map[string]bool{
				"IsNotFound": false, "IsDuplicate": false, "IsForeignKey": true,
				"IsConnection": false, "IsTimeout": false, "IsSerialization": false, "IsDeadlock": false,
			},
		},
		{
			name: "connection",
			err:  New(CodeConnection, "connection error"),
			checks: map[string]bool{
				"IsNotFound": false, "IsDuplicate": false, "IsForeignKey": false,
				"IsConnection": true, "IsTimeout": false, "IsSerialization": false, "IsDeadlock": false,
			},
		},
		{
			name: "timeout",
			err:  New(CodeTimeout, "timeout"),
			checks: map[string]bool{
				"IsNotFound": false, "IsDuplicate": false, "IsForeignKey": false,
				"IsConnection": false, "IsTimeout": true, "IsSerialization": false, "IsDeadlock": false,
			},
		},
		{
			name: "serialization",
			err:  New(CodeSerialization, "serialization"),
			checks: map[string]bool{
				"IsNotFound": false, "IsDuplicate": false, "IsForeignKey": false,
				"IsConnection": false, "IsTimeout": false, "IsSerialization": true, "IsDeadlock": false,
			},
		},
		{
			name: "deadlock",
			err:  New(CodeDeadlock, "deadlock"),
			checks: map[string]bool{
				"IsNotFound": false, "IsDuplicate": false, "IsForeignKey": false,
				"IsConnection": false, "IsTimeout": false, "IsSerialization": false, "IsDeadlock": true,
			},
		},
	}

	checkers := map[string]func(error) bool{
		"IsNotFound":      IsNotFound,
		"IsDuplicate":     IsDuplicate,
		"IsForeignKey":    IsForeignKey,
		"IsConnection":    IsConnection,
		"IsTimeout":       IsTimeout,
		"IsSerialization": IsSerialization,
		"IsDeadlock":      IsDeadlock,
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

func TestConvenienceConstructors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *Error
		wantCode Code
		wantMsg  string
	}{
		{"NotFound", NotFound("not found"), CodeNotFound, "not found"},
		{"NotFoundf", NotFoundf("user %s not found", "123"), CodeNotFound, "user 123 not found"},
		{"Duplicate", Duplicate("duplicate"), CodeDuplicate, "duplicate"},
		{"Duplicatef", Duplicatef("email %s exists", "test@x.com"), CodeDuplicate, "email test@x.com exists"},
		{"ForeignKey", ForeignKey("fk error"), CodeForeignKey, "fk error"},
		{"Connection", Connection("conn error"), CodeConnection, "conn error"},
		{"Timeout", Timeout("timeout"), CodeTimeout, "timeout"},
		{"Internal", Internal("internal"), CodeInternal, "internal"},
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

func TestError_Hint(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{
		Code:    "23505",
		Message: "error",
		Hint:    "Try using a different value",
	}

	got := FromPgError(pgErr)

	if got.Hint() != "Try using a different value" {
		t.Errorf("Hint() = %q, want %q", got.Hint(), "Try using a different value")
	}
}

// TestCoreErrorIntegration verifies that database errors integrate correctly
// with txova-go-core/errors, enabling unified error handling across the application.
func TestCoreErrorIntegration(t *testing.T) {
	t.Parallel()

	t.Run("database NotFound works with core IsNotFound", func(t *testing.T) {
		t.Parallel()
		dbErr := NotFound("user not found")

		// Database error should be detectable by core error helpers.
		if !coreerrors.IsNotFound(dbErr) {
			t.Error("coreerrors.IsNotFound(dbErr) should be true for DB_NOT_FOUND")
		}
	})

	t.Run("database Duplicate works with core IsConflict", func(t *testing.T) {
		t.Parallel()
		dbErr := Duplicate("email already exists")

		if !coreerrors.IsConflict(dbErr) {
			t.Error("coreerrors.IsConflict(dbErr) should be true for DB_DUPLICATE")
		}
	})

	t.Run("database ForeignKey works with core IsConflict", func(t *testing.T) {
		t.Parallel()
		dbErr := ForeignKey("referenced user does not exist")

		if !coreerrors.IsConflict(dbErr) {
			t.Error("coreerrors.IsConflict(dbErr) should be true for DB_FOREIGN_KEY")
		}
	})

	t.Run("database Connection works with core IsServiceUnavailable", func(t *testing.T) {
		t.Parallel()
		dbErr := Connection("connection refused")

		if !coreerrors.IsServiceUnavailable(dbErr) {
			t.Error("coreerrors.IsServiceUnavailable(dbErr) should be true for DB_CONNECTION")
		}
	})

	t.Run("database Timeout works with core IsServiceUnavailable", func(t *testing.T) {
		t.Parallel()
		dbErr := Timeout("query timeout")

		if !coreerrors.IsServiceUnavailable(dbErr) {
			t.Error("coreerrors.IsServiceUnavailable(dbErr) should be true for DB_TIMEOUT")
		}
	})

	t.Run("database Internal works with core IsInternalError", func(t *testing.T) {
		t.Parallel()
		dbErr := Internal("unexpected error")

		if !coreerrors.IsInternalError(dbErr) {
			t.Error("coreerrors.IsInternalError(dbErr) should be true for DB_INTERNAL")
		}
	})

	t.Run("database CheckViolation works with core IsValidationError", func(t *testing.T) {
		t.Parallel()
		dbErr := New(CodeCheckViolation, "check constraint failed")

		if !coreerrors.IsValidationError(dbErr) {
			t.Error("coreerrors.IsValidationError(dbErr) should be true for DB_CHECK_VIOLATION")
		}
	})

	t.Run("database InvalidInput works with core IsValidationError", func(t *testing.T) {
		t.Parallel()
		dbErr := New(CodeInvalidInput, "null value not allowed")

		if !coreerrors.IsValidationError(dbErr) {
			t.Error("coreerrors.IsValidationError(dbErr) should be true for DB_INVALID_INPUT")
		}
	})
}

// TestCoreCodeMapping verifies the mapping from database codes to core codes.
func TestCoreCodeMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dbCode   Code
		coreCode coreerrors.Code
	}{
		{CodeNotFound, coreerrors.CodeNotFound},
		{CodeDuplicate, coreerrors.CodeConflict},
		{CodeForeignKey, coreerrors.CodeConflict},
		{CodeCheckViolation, coreerrors.CodeValidationError},
		{CodeConnection, coreerrors.CodeServiceUnavailable},
		{CodeTimeout, coreerrors.CodeServiceUnavailable},
		{CodeSerialization, coreerrors.CodeConflict},
		{CodeDeadlock, coreerrors.CodeConflict},
		{CodeInvalidInput, coreerrors.CodeValidationError},
		{CodeInternal, coreerrors.CodeInternalError},
	}

	for _, tt := range tests {
		t.Run(tt.dbCode.String(), func(t *testing.T) {
			t.Parallel()
			if got := tt.dbCode.CoreCode(); got != tt.coreCode {
				t.Errorf("CoreCode() = %v, want %v", got, tt.coreCode)
			}
		})
	}
}

// TestHTTPStatusMapping verifies that database errors have correct HTTP status codes.
func TestHTTPStatusMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        *Error
		wantStatus int
	}{
		{"NotFound", NotFound("not found"), 404},
		{"Duplicate", Duplicate("duplicate"), 409},
		{"ForeignKey", ForeignKey("fk error"), 409},
		{"Connection", Connection("conn error"), 503},
		{"Timeout", Timeout("timeout"), 503},
		{"Internal", Internal("internal"), 500},
		{"CheckViolation", New(CodeCheckViolation, "check"), 400},
		{"InvalidInput", New(CodeInvalidInput, "invalid"), 400},
		{"Serialization", New(CodeSerialization, "serial"), 409},
		{"Deadlock", New(CodeDeadlock, "deadlock"), 409},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.HTTPStatus(); got != tt.wantStatus {
				t.Errorf("HTTPStatus() = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

// TestErrorsIsWithCoreAppError verifies errors.Is works across error types.
func TestErrorsIsWithCoreAppError(t *testing.T) {
	t.Parallel()

	t.Run("database error matches core error with same code", func(t *testing.T) {
		t.Parallel()
		dbErr := NotFound("user not found")
		coreErr := coreerrors.NotFound("resource not found")

		// Both should be considered "not found" errors.
		if !errors.Is(dbErr, coreErr) {
			t.Error("errors.Is(dbErr, coreErr) should be true for matching codes")
		}
	})

	t.Run("database error does not match core error with different code", func(t *testing.T) {
		t.Parallel()
		dbErr := NotFound("user not found")
		coreErr := coreerrors.Conflict("conflict")

		if errors.Is(dbErr, coreErr) {
			t.Error("errors.Is(dbErr, coreErr) should be false for different codes")
		}
	})
}

// TestFromPgErrorCoreIntegration verifies FromPgError produces errors compatible with core.
func TestFromPgErrorCoreIntegration(t *testing.T) {
	t.Parallel()

	t.Run("unique violation maps to Conflict", func(t *testing.T) {
		t.Parallel()
		pgErr := &pgconn.PgError{
			Code:    "23505",
			Message: "duplicate key",
		}

		dbErr := FromPgError(pgErr)

		if !coreerrors.IsConflict(dbErr) {
			t.Error("coreerrors.IsConflict should be true for unique violation")
		}
		if dbErr.HTTPStatus() != 409 {
			t.Errorf("HTTPStatus() = %d, want 409", dbErr.HTTPStatus())
		}
	})

	t.Run("connection error maps to ServiceUnavailable", func(t *testing.T) {
		t.Parallel()
		pgErr := &pgconn.PgError{
			Code:    "08000",
			Message: "connection exception",
		}

		dbErr := FromPgError(pgErr)

		if !coreerrors.IsServiceUnavailable(dbErr) {
			t.Error("coreerrors.IsServiceUnavailable should be true for connection error")
		}
		if dbErr.HTTPStatus() != 503 {
			t.Errorf("HTTPStatus() = %d, want 503", dbErr.HTTPStatus())
		}
	})
}

func TestError_As_WrongType(t *testing.T) {
	t.Parallel()

	dbErr := New(CodeNotFound, "not found")

	// Try to extract as wrong type (should return false)
	var s string
	if dbErr.As(&s) {
		t.Error("As() should return false for wrong type")
	}

	// Try to extract as nil interface (should return false)
	var i interface{}
	if dbErr.As(&i) {
		t.Error("As() should return false for interface{}")
	}
}

func TestError_As_AppError(t *testing.T) {
	t.Parallel()

	dbErr := New(CodeNotFound, "user not found")

	// Extract as AppError (should work)
	var appErr *coreerrors.AppError
	if !dbErr.As(&appErr) {
		t.Error("As() should return true for *coreerrors.AppError")
	}
	if appErr == nil {
		t.Error("appErr should not be nil")
	}
	if appErr.Code() != coreerrors.CodeNotFound {
		t.Errorf("appErr.Code() = %v, want %v", appErr.Code(), coreerrors.CodeNotFound)
	}
}
