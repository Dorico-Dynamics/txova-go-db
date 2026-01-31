// Package postgres provides PostgreSQL database utilities for the Txova platform.
// It includes connection pooling, transaction management, query building, and
// migration support with proper error handling and logging integration.
package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Code represents a database-specific error code.
type Code string

// Standard database error codes for the Txova platform.
const (
	// CodeNotFound indicates the requested record was not found.
	CodeNotFound Code = "DB_NOT_FOUND"
	// CodeDuplicate indicates a unique constraint violation.
	CodeDuplicate Code = "DB_DUPLICATE"
	// CodeForeignKey indicates a foreign key constraint violation.
	CodeForeignKey Code = "DB_FOREIGN_KEY"
	// CodeCheckViolation indicates a check constraint violation.
	CodeCheckViolation Code = "DB_CHECK_VIOLATION"
	// CodeConnection indicates a connection error.
	CodeConnection Code = "DB_CONNECTION"
	// CodeTimeout indicates a query or connection timeout.
	CodeTimeout Code = "DB_TIMEOUT"
	// CodeSerialization indicates a serialization failure in transactions.
	CodeSerialization Code = "DB_SERIALIZATION"
	// CodeDeadlock indicates a deadlock was detected.
	CodeDeadlock Code = "DB_DEADLOCK"
	// CodeInvalidInput indicates invalid input data.
	CodeInvalidInput Code = "DB_INVALID_INPUT"
	// CodeInternal indicates an unclassified internal database error.
	CodeInternal Code = "DB_INTERNAL"
)

// String returns the string representation of the error code.
func (c Code) String() string {
	return string(c)
}

// Error represents a database-specific error with a machine-readable code,
// human-readable message, and optional wrapped error.
type Error struct {
	code       Code
	message    string
	cause      error
	sqlState   string // PostgreSQL SQLSTATE code
	detail     string // Additional detail from PostgreSQL
	hint       string // Hint from PostgreSQL
	tableName  string // Table name if available
	column     string // Column name if available
	constraint string // Constraint name if available
}

// New creates a new Error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{
		code:    code,
		message: message,
	}
}

// Wrap creates a new Error that wraps an existing error.
func Wrap(code Code, message string, cause error) *Error {
	return &Error{
		code:    code,
		message: message,
		cause:   cause,
	}
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

// Code returns the error code.
func (e *Error) Code() Code {
	return e.code
}

// Message returns the human-readable error message.
func (e *Error) Message() string {
	return e.message
}

// SQLState returns the PostgreSQL SQLSTATE code if available.
func (e *Error) SQLState() string {
	return e.sqlState
}

// Detail returns additional detail from PostgreSQL if available.
func (e *Error) Detail() string {
	return e.detail
}

// Hint returns the hint from PostgreSQL if available.
func (e *Error) Hint() string {
	return e.hint
}

// TableName returns the table name if available.
func (e *Error) TableName() string {
	return e.tableName
}

// Column returns the column name if available.
func (e *Error) Column() string {
	return e.column
}

// Constraint returns the constraint name if available.
func (e *Error) Constraint() string {
	return e.constraint
}

// Unwrap returns the wrapped error, if any.
func (e *Error) Unwrap() error {
	return e.cause
}

// Is reports whether the target error is an Error with the same code.
func (e *Error) Is(target error) bool {
	var dbErr *Error
	if errors.As(target, &dbErr) {
		return e.code == dbErr.code
	}
	return false
}

// WithMessage returns a new Error with the same code but a different message.
func (e *Error) WithMessage(message string) *Error {
	newErr := *e
	newErr.message = message
	return &newErr
}

// WithCause returns a new Error wrapping a different cause.
func (e *Error) WithCause(cause error) *Error {
	newErr := *e
	newErr.cause = cause
	return &newErr
}

// PostgreSQL SQLSTATE code classes.
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	// Class 08 — Connection Exception
	sqlStateConnectionException = "08"
	// Class 23 — Integrity Constraint Violation
	sqlStateUniqueViolation     = "23505"
	sqlStateForeignKeyViolation = "23503"
	sqlStateCheckViolation      = "23514"
	sqlStateNotNullViolation    = "23502"
	// Class 40 — Transaction Rollback
	sqlStateSerializationFailure = "40001"
	sqlStateDeadlockDetected     = "40P01"
	// Class 57 — Operator Intervention
	sqlStateQueryCanceled = "57014"
)

// FromPgError converts a PostgreSQL error to a domain Error.
// It extracts the SQLSTATE code and maps it to the appropriate domain error code.
func FromPgError(err error) *Error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		// Not a PostgreSQL error, wrap as internal error.
		return Wrap(CodeInternal, "database error", err)
	}

	code := mapSQLState(pgErr.Code)
	dbErr := &Error{
		code:       code,
		message:    pgErr.Message,
		cause:      err,
		sqlState:   pgErr.Code,
		detail:     pgErr.Detail,
		hint:       pgErr.Hint,
		tableName:  pgErr.TableName,
		column:     pgErr.ColumnName,
		constraint: pgErr.ConstraintName,
	}

	return dbErr
}

// mapSQLState maps a PostgreSQL SQLSTATE code to a domain error code.
func mapSQLState(sqlState string) Code {
	// Check specific codes first.
	switch sqlState {
	case sqlStateUniqueViolation:
		return CodeDuplicate
	case sqlStateForeignKeyViolation:
		return CodeForeignKey
	case sqlStateCheckViolation:
		return CodeCheckViolation
	case sqlStateNotNullViolation:
		return CodeInvalidInput
	case sqlStateSerializationFailure:
		return CodeSerialization
	case sqlStateDeadlockDetected:
		return CodeDeadlock
	case sqlStateQueryCanceled:
		return CodeTimeout
	}

	// Check class (first two characters).
	if len(sqlState) >= 2 {
		class := sqlState[:2]
		switch class {
		case sqlStateConnectionException:
			return CodeConnection
		}
	}

	return CodeInternal
}

// Helper functions for error checking.

// IsError checks if the given error is a database Error.
func IsError(err error) bool {
	var dbErr *Error
	return errors.As(err, &dbErr)
}

// AsError attempts to extract a database Error from the given error.
// Returns nil if the error is not a database Error.
func AsError(err error) *Error {
	var dbErr *Error
	if errors.As(err, &dbErr) {
		return dbErr
	}
	return nil
}

// GetCode returns the error code from an error if it's a database Error,
// or CodeInternal otherwise.
func GetCode(err error) Code {
	if dbErr := AsError(err); dbErr != nil {
		return dbErr.Code()
	}
	return CodeInternal
}

// IsCode checks if the given error is a database Error with the specified code.
func IsCode(err error, code Code) bool {
	if dbErr := AsError(err); dbErr != nil {
		return dbErr.Code() == code
	}
	return false
}

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return IsCode(err, CodeNotFound)
}

// IsDuplicate checks if the error is a duplicate/unique constraint violation.
func IsDuplicate(err error) bool {
	return IsCode(err, CodeDuplicate)
}

// IsForeignKey checks if the error is a foreign key constraint violation.
func IsForeignKey(err error) bool {
	return IsCode(err, CodeForeignKey)
}

// IsConnection checks if the error is a connection error.
func IsConnection(err error) bool {
	return IsCode(err, CodeConnection)
}

// IsTimeout checks if the error is a timeout error.
func IsTimeout(err error) bool {
	return IsCode(err, CodeTimeout)
}

// IsSerialization checks if the error is a serialization failure.
func IsSerialization(err error) bool {
	return IsCode(err, CodeSerialization)
}

// IsDeadlock checks if the error is a deadlock error.
func IsDeadlock(err error) bool {
	return IsCode(err, CodeDeadlock)
}

// Convenience constructors.

// NotFound creates a new not found error with the given message.
func NotFound(message string) *Error {
	return New(CodeNotFound, message)
}

// NotFoundf creates a new not found error with a formatted message.
func NotFoundf(format string, args ...any) *Error {
	return New(CodeNotFound, fmt.Sprintf(format, args...))
}

// Duplicate creates a new duplicate error with the given message.
func Duplicate(message string) *Error {
	return New(CodeDuplicate, message)
}

// Duplicatef creates a new duplicate error with a formatted message.
func Duplicatef(format string, args ...any) *Error {
	return New(CodeDuplicate, fmt.Sprintf(format, args...))
}

// ForeignKey creates a new foreign key violation error.
func ForeignKey(message string) *Error {
	return New(CodeForeignKey, message)
}

// Connection creates a new connection error.
func Connection(message string) *Error {
	return New(CodeConnection, message)
}

// ConnectionWrap creates a new connection error wrapping an existing error.
func ConnectionWrap(message string, cause error) *Error {
	return Wrap(CodeConnection, message, cause)
}

// Timeout creates a new timeout error.
func Timeout(message string) *Error {
	return New(CodeTimeout, message)
}

// TimeoutWrap creates a new timeout error wrapping an existing error.
func TimeoutWrap(message string, cause error) *Error {
	return Wrap(CodeTimeout, message, cause)
}

// Internal creates a new internal error.
func Internal(message string) *Error {
	return New(CodeInternal, message)
}

// InternalWrap creates a new internal error wrapping an existing error.
func InternalWrap(message string, cause error) *Error {
	return Wrap(CodeInternal, message, cause)
}
