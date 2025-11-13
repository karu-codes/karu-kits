package dbx

import (
	"context"
	"database/sql"
	"net"
	"strings"

	"github.com/karu-codes/karu-kits/errors"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrorType represents the type of database error
type ErrorType string

const (
	ErrorTypeConnection      ErrorType = "connection"
	ErrorTypeQuery           ErrorType = "query"
	ErrorTypeTransaction     ErrorType = "transaction"
	ErrorTypeConstraint      ErrorType = "constraint"
	ErrorTypeSerialization   ErrorType = "serialization"
	ErrorTypeDeadlock        ErrorType = "deadlock"
	ErrorTypeTimeout         ErrorType = "timeout"
	ErrorTypeNotFound        ErrorType = "not_found"
	ErrorTypeUnique          ErrorType = "unique_violation"
	ErrorTypeForeignKey      ErrorType = "foreign_key_violation"
	ErrorTypeUnknown         ErrorType = "unknown"
)

// classifyError determines the type of database error
func classifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}

	// Standard errors
	if err == sql.ErrNoRows {
		return ErrorTypeNotFound
	}
	if err == sql.ErrTxDone {
		return ErrorTypeTransaction
	}
	if err == sql.ErrConnDone {
		return ErrorTypeConnection
	}

	// Context errors
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ErrorTypeTimeout
	}

	// Network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ErrorTypeConnection
	}

	// PostgreSQL specific errors via pgconn
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return classifyPgError(pgErr)
	}

	// Fallback to string matching for both MySQL and Postgres
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "connect") {
		return ErrorTypeConnection
	}
	if strings.Contains(errStr, "serialization") || strings.Contains(errStr, "could not serialize") {
		return ErrorTypeSerialization
	}
	if strings.Contains(errStr, "deadlock") {
		return ErrorTypeDeadlock
	}
	if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "unique constraint") {
		return ErrorTypeUnique
	}
	if strings.Contains(errStr, "foreign key") {
		return ErrorTypeForeignKey
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "timed out") {
		return ErrorTypeTimeout
	}

	return ErrorTypeUnknown
}

// classifyPgError classifies PostgreSQL-specific errors using error codes
// Reference: https://www.postgresql.org/docs/current/errcodes-appendix.html
func classifyPgError(pgErr *pgconn.PgError) ErrorType {
	code := pgErr.Code

	// Connection errors (Class 08)
	if strings.HasPrefix(code, "08") {
		return ErrorTypeConnection
	}

	// Serialization failures (Class 40)
	if code == "40001" { // serialization_failure
		return ErrorTypeSerialization
	}
	if code == "40P01" { // deadlock_detected
		return ErrorTypeDeadlock
	}

	// Integrity constraint violations (Class 23)
	if code == "23505" { // unique_violation
		return ErrorTypeUnique
	}
	if code == "23503" { // foreign_key_violation
		return ErrorTypeForeignKey
	}
	if strings.HasPrefix(code, "23") {
		return ErrorTypeConstraint
	}

	// Query errors (Class 42)
	if strings.HasPrefix(code, "42") {
		return ErrorTypeQuery
	}

	return ErrorTypeUnknown
}

// wrapDBError wraps a database error with the errors package
func wrapDBError(err error, operation string) error {
	if err == nil {
		return nil
	}

	errType := classifyError(err)

	// Create base error with appropriate code
	var code errors.Code
	switch errType {
	case ErrorTypeNotFound:
		code = errors.CodeNotFound
	case ErrorTypeTimeout:
		code = errors.CodeTimeout
	case ErrorTypeConnection, ErrorTypeDeadlock, ErrorTypeSerialization:
		code = errors.CodeUnavailable
	case ErrorTypeUnique, ErrorTypeConstraint, ErrorTypeForeignKey:
		code = errors.CodeConflict
	default:
		code = errors.CodeDatabase
	}

	wrappedErr := errors.Wrap(err, code, operation)
	wrappedErr.WithDetail("error_type", string(errType))

	// Add PostgreSQL specific details
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		wrappedErr.
			WithDetail("pg_code", pgErr.Code).
			WithDetail("pg_severity", pgErr.Severity).
			WithDetail("pg_detail", pgErr.Detail).
			WithDetail("pg_hint", pgErr.Hint)
		if pgErr.TableName != "" {
			wrappedErr.WithDetail("table", pgErr.TableName)
		}
		if pgErr.ColumnName != "" {
			wrappedErr.WithDetail("column", pgErr.ColumnName)
		}
		if pgErr.ConstraintName != "" {
			wrappedErr.WithDetail("constraint", pgErr.ConstraintName)
		}
	}

	return wrappedErr
}

// IsRetryable determines if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errType := classifyError(err)

	switch errType {
	case ErrorTypeConnection, ErrorTypeSerialization, ErrorTypeDeadlock, ErrorTypeTimeout:
		return true
	default:
		return false
	}
}

// IsNotFound checks if the error is a "not found" error
func IsNotFound(err error) bool {
	return classifyError(err) == ErrorTypeNotFound
}

// IsConstraintViolation checks if the error is a constraint violation
func IsConstraintViolation(err error) bool {
	errType := classifyError(err)
	return errType == ErrorTypeConstraint ||
	       errType == ErrorTypeUnique ||
	       errType == ErrorTypeForeignKey
}
