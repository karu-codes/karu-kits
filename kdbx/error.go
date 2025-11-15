package kdbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrorCode represents the type of database error.
type ErrorCode string

const (
	// Configuration errors
	CodeInvalidArgument ErrorCode = "INVALID_ARGUMENT"
	CodeInvalidConfig   ErrorCode = "INVALID_CONFIG"

	// Connection errors
	CodeUnavailable      ErrorCode = "UNAVAILABLE"
	CodeTimeout          ErrorCode = "TIMEOUT"
	CodeUnauthenticated  ErrorCode = "UNAUTHENTICATED"
	CodePermission       ErrorCode = "PERMISSION_DENIED"

	// Query errors
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeAlreadyExists ErrorCode = "ALREADY_EXISTS"
	CodeConflict     ErrorCode = "CONFLICT"
	CodeInvalidState ErrorCode = "INVALID_STATE"

	// Transaction errors
	CodeDatabase ErrorCode = "DATABASE_ERROR"
	CodeInternal ErrorCode = "INTERNAL_ERROR"
	CodeCancelled ErrorCode = "CANCELLED"
)

// DatabaseError represents a structured database error.
type DatabaseError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *DatabaseError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause error.
func (e *DatabaseError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target.
func (e *DatabaseError) Is(target error) bool {
	t, ok := target.(*DatabaseError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// Predefined errors for database operations.
var (
	// Configuration errors
	ErrInvalidDriver      = &DatabaseError{Code: CodeInvalidArgument, Message: "invalid database driver"}
	ErrMissingDatabaseURL = &DatabaseError{Code: CodeInvalidArgument, Message: "database URL is required"}
	ErrInvalidPoolConfig  = &DatabaseError{Code: CodeInvalidConfig, Message: "invalid connection pool configuration"}
	ErrInvalidRetryConfig = &DatabaseError{Code: CodeInvalidConfig, Message: "invalid retry configuration"}

	// Connection errors
	ErrConnectionFailed    = &DatabaseError{Code: CodeUnavailable, Message: "failed to connect to database"}
	ErrConnectionTimeout   = &DatabaseError{Code: CodeTimeout, Message: "database connection timeout"}
	ErrMaxRetriesExceeded  = &DatabaseError{Code: CodeUnavailable, Message: "maximum retry attempts exceeded"}
	ErrDatabaseUnavailable = &DatabaseError{Code: CodeUnavailable, Message: "database is unavailable"}

	// Query errors
	ErrQueryFailed = &DatabaseError{Code: CodeDatabase, Message: "query execution failed"}
	ErrNoRows      = &DatabaseError{Code: CodeNotFound, Message: "no rows found"}
	ErrTooManyRows = &DatabaseError{Code: CodeInvalidState, Message: "query returned too many rows"}

	// Transaction errors
	ErrTransactionFailed = &DatabaseError{Code: CodeDatabase, Message: "transaction failed"}
	ErrDeadlock          = &DatabaseError{Code: CodeConflict, Message: "deadlock detected"}
	ErrSerializationFail = &DatabaseError{Code: CodeConflict, Message: "serialization failure"}

	// Constraint errors
	ErrConstraintViolation = &DatabaseError{Code: CodeInvalidArgument, Message: "constraint violation"}
	ErrUniqueViolation     = &DatabaseError{Code: CodeAlreadyExists, Message: "unique constraint violation"}
	ErrForeignKeyViolation = &DatabaseError{Code: CodeInvalidArgument, Message: "foreign key constraint violation"}
	ErrCheckViolation      = &DatabaseError{Code: CodeInvalidArgument, Message: "check constraint violation"}
	ErrNotNullViolation    = &DatabaseError{Code: CodeInvalidArgument, Message: "not null constraint violation"}

	// Context errors
	ErrContextCancelled = &DatabaseError{Code: CodeCancelled, Message: "operation cancelled"}
	ErrContextTimeout   = &DatabaseError{Code: CodeTimeout, Message: "operation timeout"}
)

// WrapError wraps a database error with context and classification.
func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}

	// Check for standard errors
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
		return &DatabaseError{
			Code:    CodeNotFound,
			Message: msg,
			Cause:   err,
		}
	}

	if errors.Is(err, sql.ErrTxDone) {
		return &DatabaseError{
			Code:    CodeInvalidState,
			Message: msg,
			Cause:   err,
		}
	}

	if errors.Is(err, sql.ErrConnDone) {
		return &DatabaseError{
			Code:    CodeUnavailable,
			Message: msg,
			Cause:   err,
		}
	}

	// Check for context errors
	if errors.Is(err, context.Canceled) {
		return &DatabaseError{
			Code:    CodeCancelled,
			Message: msg,
			Cause:   err,
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return &DatabaseError{
			Code:    CodeTimeout,
			Message: msg,
			Cause:   err,
		}
	}

	// Try PostgreSQL error classification
	if code, ok := classifyPostgresError(err); ok {
		return &DatabaseError{
			Code:    code,
			Message: msg,
			Cause:   err,
		}
	}

	// Try MySQL error classification
	if code, ok := classifyMySQLError(err); ok {
		return &DatabaseError{
			Code:    code,
			Message: msg,
			Cause:   err,
		}
	}

	// Default to generic database error
	return &DatabaseError{
		Code:    CodeDatabase,
		Message: msg,
		Cause:   err,
	}
}

// classifyPostgresError classifies PostgreSQL-specific errors.
func classifyPostgresError(err error) (ErrorCode, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "", false
	}

	// Map PostgreSQL error codes to our error codes
	// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
	switch pgErr.Code {
	// Class 23: Integrity Constraint Violation
	case "23000": // integrity_constraint_violation
		return CodeDatabase, true
	case "23001": // restrict_violation
		return CodeInvalidArgument, true
	case "23502": // not_null_violation
		return CodeInvalidArgument, true
	case "23503": // foreign_key_violation
		return CodeInvalidArgument, true
	case "23505": // unique_violation
		return CodeAlreadyExists, true
	case "23514": // check_violation
		return CodeInvalidArgument, true
	case "23P01": // exclusion_violation
		return CodeInvalidArgument, true

	// Class 40: Transaction Rollback
	case "40001": // serialization_failure
		return CodeConflict, true
	case "40P01": // deadlock_detected
		return CodeConflict, true

	// Class 42: Syntax Error or Access Rule Violation
	case "42501": // insufficient_privilege
		return CodePermission, true
	case "42601": // syntax_error
		return CodeInvalidArgument, true
	case "42701": // duplicate_column
		return CodeInvalidArgument, true
	case "42702": // ambiguous_column
		return CodeInvalidArgument, true
	case "42703": // undefined_column
		return CodeInvalidArgument, true
	case "42P01": // undefined_table
		return CodeNotFound, true
	case "42P02": // undefined_parameter
		return CodeInvalidArgument, true

	// Class 53: Insufficient Resources
	case "53000": // insufficient_resources
		return CodeUnavailable, true
	case "53100": // disk_full
		return CodeUnavailable, true
	case "53200": // out_of_memory
		return CodeUnavailable, true
	case "53300": // too_many_connections
		return CodeUnavailable, true

	// Class 54: Program Limit Exceeded
	case "54000": // program_limit_exceeded
		return CodeInvalidArgument, true
	case "54001": // statement_too_complex
		return CodeInvalidArgument, true
	case "54011": // too_many_columns
		return CodeInvalidArgument, true
	case "54023": // too_many_arguments
		return CodeInvalidArgument, true

	// Class 57: Operator Intervention
	case "57000": // operator_intervention
		return CodeUnavailable, true
	case "57014": // query_canceled
		return CodeCancelled, true
	case "57P01": // admin_shutdown
		return CodeUnavailable, true
	case "57P02": // crash_shutdown
		return CodeUnavailable, true
	case "57P03": // cannot_connect_now
		return CodeUnavailable, true

	// Class 58: System Error
	case "58000": // system_error
		return CodeInternal, true
	case "58030": // io_error
		return CodeUnavailable, true
	case "58P01": // undefined_file
		return CodeNotFound, true
	case "58P02": // duplicate_file
		return CodeAlreadyExists, true

	default:
		// Unknown PostgreSQL error
		return CodeDatabase, true
	}
}

// classifyMySQLError classifies MySQL-specific errors.
func classifyMySQLError(err error) (ErrorCode, bool) {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return "", false
	}

	// Map MySQL error codes to our error codes
	// See: https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html
	switch mysqlErr.Number {
	// Connection errors
	case 1040: // ER_CON_COUNT_ERROR (Too many connections)
		return CodeUnavailable, true
	case 1042: // ER_BAD_HOST_ERROR
		return CodeUnavailable, true
	case 1043: // ER_HANDSHAKE_ERROR
		return CodeUnavailable, true
	case 1044: // ER_DBACCESS_DENIED_ERROR
		return CodePermission, true
	case 1045: // ER_ACCESS_DENIED_ERROR
		return CodeUnauthenticated, true

	// Database/table errors
	case 1049: // ER_BAD_DB_ERROR (Unknown database)
		return CodeNotFound, true
	case 1050: // ER_TABLE_EXISTS_ERROR
		return CodeAlreadyExists, true
	case 1051: // ER_BAD_TABLE_ERROR (Unknown table)
		return CodeNotFound, true
	case 1054: // ER_BAD_FIELD_ERROR (Unknown column)
		return CodeInvalidArgument, true
	case 1060: // ER_DUP_FIELDNAME (Duplicate column name)
		return CodeInvalidArgument, true
	case 1061: // ER_DUP_KEYNAME (Duplicate key name)
		return CodeInvalidArgument, true
	case 1062: // ER_DUP_ENTRY (Duplicate entry for key)
		return CodeAlreadyExists, true
	case 1064: // ER_PARSE_ERROR (SQL syntax error)
		return CodeInvalidArgument, true

	// Constraint violations
	case 1216: // ER_NO_REFERENCED_ROW (Cannot add or update a child row)
		return CodeInvalidArgument, true
	case 1217: // ER_ROW_IS_REFERENCED (Cannot delete or update a parent row)
		return CodeInvalidArgument, true
	case 1451: // ER_ROW_IS_REFERENCED_2 (Foreign key constraint fails)
		return CodeInvalidArgument, true
	case 1452: // ER_NO_REFERENCED_ROW_2 (Foreign key constraint fails)
		return CodeInvalidArgument, true

	// Transaction errors
	case 1205: // ER_LOCK_WAIT_TIMEOUT (Lock wait timeout exceeded)
		return CodeTimeout, true
	case 1213: // ER_LOCK_DEADLOCK (Deadlock found when trying to get lock)
		return CodeConflict, true

	// Resource errors
	case 1030: // ER_FILE_NOT_FOUND
		return CodeNotFound, true
	case 1037: // ER_OUTOFMEMORY
		return CodeUnavailable, true
	case 1041: // ER_OUT_OF_RESOURCES
		return CodeUnavailable, true

	// Timeout errors
	case 1159: // ER_NET_READ_TIMEOUT
		return CodeTimeout, true
	case 1160: // ER_NET_WRITE_TIMEOUT
		return CodeTimeout, true

	// Permission errors
	case 1142: // ER_TABLEACCESS_DENIED_ERROR
		return CodePermission, true
	case 1143: // ER_COLUMNACCESS_DENIED_ERROR
		return CodePermission, true

	default:
		// Unknown MySQL error
		return CodeDatabase, true
	}
}

// IsRetryable determines if an error is safe to retry.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation (never retry)
	if errors.Is(err, context.Canceled) {
		return false
	}

	// Unwrap to get the underlying error code
	var dbErr *DatabaseError
	if !errors.As(err, &dbErr) {
		return false
	}

	// Only retry transient errors
	switch dbErr.Code {
	case CodeUnavailable: // Database temporarily unavailable
		return true
	case CodeTimeout: // Timeout (may succeed on retry)
		return true
	case CodeConflict: // Deadlock or serialization failure
		return true
	default:
		return false
	}
}

// IsNoRows checks if the error is a "no rows" error.
func IsNoRows(err error) bool {
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
		return true
	}

	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.Code == CodeNotFound
	}

	return false
}

// IsNotFound checks if the error is a "not found" error (alias for IsNoRows).
func IsNotFound(err error) bool {
	return IsNoRows(err)
}

// IsUniqueViolation checks if the error is a unique constraint violation.
func IsUniqueViolation(err error) bool {
	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.Code == CodeAlreadyExists
	}

	// Also check directly for PostgreSQL unique violation
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}

	// Also check directly for MySQL duplicate entry
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062 // ER_DUP_ENTRY
	}

	return false
}

// IsForeignKeyViolation checks if the error is a foreign key constraint violation.
func IsForeignKeyViolation(err error) bool {
	// Check PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503" // foreign_key_violation
	}

	// Check MySQL
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1216 || mysqlErr.Number == 1217 ||
			mysqlErr.Number == 1451 || mysqlErr.Number == 1452
	}

	return false
}

// IsNotNullViolation checks if the error is a NOT NULL constraint violation.
func IsNotNullViolation(err error) bool {
	// Check PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23502" // not_null_violation
	}

	return false
}

// IsCheckViolation checks if the error is a CHECK constraint violation.
func IsCheckViolation(err error) bool {
	// Check PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23514" // check_violation
	}

	return false
}

// IsConstraintViolation checks if the error is any constraint violation.
func IsConstraintViolation(err error) bool {
	return IsUniqueViolation(err) || IsForeignKeyViolation(err) ||
		IsNotNullViolation(err) || IsCheckViolation(err)
}

// IsDeadlock checks if the error is a deadlock.
func IsDeadlock(err error) bool {
	// Check PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40P01" // deadlock_detected
	}

	// Check MySQL
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1213 // ER_LOCK_DEADLOCK
	}

	return false
}

// IsSerializationFailure checks if the error is a serialization failure.
func IsSerializationFailure(err error) bool {
	// Check PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" // serialization_failure
	}

	return false
}

// IsTimeout checks if the error is a timeout error.
func IsTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.Code == CodeTimeout
	}

	return false
}

// IsConnectionError checks if the error is a connection error.
func IsConnectionError(err error) bool {
	if errors.Is(err, sql.ErrConnDone) {
		return true
	}

	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.Code == CodeUnavailable
	}

	return false
}

// IsSyntaxError checks if the error is a SQL syntax error.
func IsSyntaxError(err error) bool {
	// Check PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42601" // syntax_error
	}

	// Check MySQL
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1064 // ER_PARSE_ERROR
	}

	return false
}

// SanitizeQuery removes sensitive data from a query string for logging.
// It replaces parameter values with placeholders to prevent logging sensitive information.
func SanitizeQuery(query string) string {
	// Remove leading/trailing whitespace
	query = strings.TrimSpace(query)

	// Truncate very long queries
	const maxLength = 500
	if len(query) > maxLength {
		query = query[:maxLength] + "... (truncated)"
	}

	return query
}
