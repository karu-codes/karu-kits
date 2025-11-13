package kdbx

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	kerrors "github.com/karu-codes/karu-kits/errors"
)

// Predefined errors for database operations.
var (
	// Configuration errors
	ErrInvalidDriver      = kerrors.New(kerrors.CodeInvalidArgument, "invalid database driver")
	ErrMissingDatabaseURL = kerrors.New(kerrors.CodeInvalidArgument, "database URL is required")
	ErrInvalidPoolConfig  = kerrors.New(kerrors.CodeInvalidArgument, "invalid connection pool configuration")
	ErrInvalidRetryConfig = kerrors.New(kerrors.CodeInvalidArgument, "invalid retry configuration")

	// Connection errors
	ErrConnectionFailed     = kerrors.New(kerrors.CodeUnavailable, "failed to connect to database")
	ErrConnectionTimeout    = kerrors.New(kerrors.CodeTimeout, "database connection timeout")
	ErrMaxRetriesExceeded   = kerrors.New(kerrors.CodeUnavailable, "maximum retry attempts exceeded")
	ErrDatabaseUnavailable  = kerrors.New(kerrors.CodeUnavailable, "database is unavailable")

	// Query errors
	ErrQueryFailed      = kerrors.New(kerrors.CodeDatabase, "query execution failed")
	ErrNoRows           = kerrors.New(kerrors.CodeNotFound, "no rows found")
	ErrTooManyRows      = kerrors.New(kerrors.CodeInvalidState, "query returned too many rows")

	// Transaction errors
	ErrTransactionFailed = kerrors.New(kerrors.CodeDatabase, "transaction failed")
	ErrDeadlock          = kerrors.New(kerrors.CodeConflict, "deadlock detected")
	ErrSerializationFail = kerrors.New(kerrors.CodeConflict, "serialization failure")

	// Constraint errors
	ErrConstraintViolation = kerrors.New(kerrors.CodeInvalidArgument, "constraint violation")
	ErrUniqueViolation     = kerrors.New(kerrors.CodeAlreadyExists, "unique constraint violation")
	ErrForeignKeyViolation = kerrors.New(kerrors.CodeInvalidArgument, "foreign key constraint violation")
	ErrCheckViolation      = kerrors.New(kerrors.CodeInvalidArgument, "check constraint violation")
	ErrNotNullViolation    = kerrors.New(kerrors.CodeInvalidArgument, "not null constraint violation")

	// Context errors
	ErrContextCancelled = kerrors.New(kerrors.CodeCancelled, "operation cancelled")
	ErrContextTimeout   = kerrors.New(kerrors.CodeTimeout, "operation timeout")
)

// WrapError wraps a database error with context and classification.
func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}

	// Check for standard errors
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
		return kerrors.Wrap(err, kerrors.CodeNotFound, msg)
	}

	if errors.Is(err, sql.ErrTxDone) {
		return kerrors.Wrap(err, kerrors.CodeInvalidState, msg)
	}

	if errors.Is(err, sql.ErrConnDone) {
		return kerrors.Wrap(err, kerrors.CodeUnavailable, msg)
	}

	// Check for context errors
	if errors.Is(err, context.Canceled) {
		return kerrors.Wrap(err, kerrors.CodeCancelled, msg)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return kerrors.Wrap(err, kerrors.CodeTimeout, msg)
	}

	// Try PostgreSQL error classification
	if pgErr, ok := classifyPostgresError(err); ok {
		return kerrors.Wrap(err, pgErr, msg)
	}

	// Try MySQL error classification
	if mysqlErr, ok := classifyMySQLError(err); ok {
		return kerrors.Wrap(err, mysqlErr, msg)
	}

	// Default to generic database error
	return kerrors.Wrap(err, kerrors.CodeDatabase, msg)
}

// classifyPostgresError classifies PostgreSQL-specific errors.
func classifyPostgresError(err error) (kerrors.Code, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "", false
	}

	// Map PostgreSQL error codes to our error codes
	// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
	switch pgErr.Code {
	// Class 23: Integrity Constraint Violation
	case "23000": // integrity_constraint_violation
		return kerrors.CodeDatabase, true
	case "23001": // restrict_violation
		return kerrors.CodeInvalidArgument, true
	case "23502": // not_null_violation
		return kerrors.CodeInvalidArgument, true
	case "23503": // foreign_key_violation
		return kerrors.CodeInvalidArgument, true
	case "23505": // unique_violation
		return kerrors.CodeAlreadyExists, true
	case "23514": // check_violation
		return kerrors.CodeInvalidArgument, true
	case "23P01": // exclusion_violation
		return kerrors.CodeInvalidArgument, true

	// Class 40: Transaction Rollback
	case "40001": // serialization_failure
		return kerrors.CodeConflict, true
	case "40P01": // deadlock_detected
		return kerrors.CodeConflict, true

	// Class 42: Syntax Error or Access Rule Violation
	case "42501": // insufficient_privilege
		return kerrors.CodePermission, true
	case "42601": // syntax_error
		return kerrors.CodeInvalidArgument, true
	case "42701": // duplicate_column
		return kerrors.CodeInvalidArgument, true
	case "42702": // ambiguous_column
		return kerrors.CodeInvalidArgument, true
	case "42703": // undefined_column
		return kerrors.CodeInvalidArgument, true
	case "42P01": // undefined_table
		return kerrors.CodeNotFound, true
	case "42P02": // undefined_parameter
		return kerrors.CodeInvalidArgument, true

	// Class 53: Insufficient Resources
	case "53000": // insufficient_resources
		return kerrors.CodeUnavailable, true
	case "53100": // disk_full
		return kerrors.CodeUnavailable, true
	case "53200": // out_of_memory
		return kerrors.CodeUnavailable, true
	case "53300": // too_many_connections
		return kerrors.CodeUnavailable, true

	// Class 54: Program Limit Exceeded
	case "54000": // program_limit_exceeded
		return kerrors.CodeInvalidArgument, true
	case "54001": // statement_too_complex
		return kerrors.CodeInvalidArgument, true
	case "54011": // too_many_columns
		return kerrors.CodeInvalidArgument, true
	case "54023": // too_many_arguments
		return kerrors.CodeInvalidArgument, true

	// Class 57: Operator Intervention
	case "57000": // operator_intervention
		return kerrors.CodeUnavailable, true
	case "57014": // query_canceled
		return kerrors.CodeCancelled, true
	case "57P01": // admin_shutdown
		return kerrors.CodeUnavailable, true
	case "57P02": // crash_shutdown
		return kerrors.CodeUnavailable, true
	case "57P03": // cannot_connect_now
		return kerrors.CodeUnavailable, true

	// Class 58: System Error
	case "58000": // system_error
		return kerrors.CodeInternal, true
	case "58030": // io_error
		return kerrors.CodeUnavailable, true
	case "58P01": // undefined_file
		return kerrors.CodeNotFound, true
	case "58P02": // duplicate_file
		return kerrors.CodeAlreadyExists, true

	default:
		// Unknown PostgreSQL error
		return kerrors.CodeDatabase, true
	}
}

// classifyMySQLError classifies MySQL-specific errors.
func classifyMySQLError(err error) (kerrors.Code, bool) {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return "", false
	}

	// Map MySQL error codes to our error codes
	// See: https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html
	switch mysqlErr.Number {
	// Connection errors
	case 1040: // ER_CON_COUNT_ERROR (Too many connections)
		return kerrors.CodeUnavailable, true
	case 1042: // ER_BAD_HOST_ERROR
		return kerrors.CodeUnavailable, true
	case 1043: // ER_HANDSHAKE_ERROR
		return kerrors.CodeUnavailable, true
	case 1044: // ER_DBACCESS_DENIED_ERROR
		return kerrors.CodePermission, true
	case 1045: // ER_ACCESS_DENIED_ERROR
		return kerrors.CodeUnauthenticated, true

	// Database/table errors
	case 1049: // ER_BAD_DB_ERROR (Unknown database)
		return kerrors.CodeNotFound, true
	case 1050: // ER_TABLE_EXISTS_ERROR
		return kerrors.CodeAlreadyExists, true
	case 1051: // ER_BAD_TABLE_ERROR (Unknown table)
		return kerrors.CodeNotFound, true
	case 1054: // ER_BAD_FIELD_ERROR (Unknown column)
		return kerrors.CodeInvalidArgument, true
	case 1060: // ER_DUP_FIELDNAME (Duplicate column name)
		return kerrors.CodeInvalidArgument, true
	case 1061: // ER_DUP_KEYNAME (Duplicate key name)
		return kerrors.CodeInvalidArgument, true
	case 1062: // ER_DUP_ENTRY (Duplicate entry for key)
		return kerrors.CodeAlreadyExists, true
	case 1064: // ER_PARSE_ERROR (SQL syntax error)
		return kerrors.CodeInvalidArgument, true

	// Constraint violations
	case 1216: // ER_NO_REFERENCED_ROW (Cannot add or update a child row)
		return kerrors.CodeInvalidArgument, true
	case 1217: // ER_ROW_IS_REFERENCED (Cannot delete or update a parent row)
		return kerrors.CodeInvalidArgument, true
	case 1451: // ER_ROW_IS_REFERENCED_2 (Foreign key constraint fails)
		return kerrors.CodeInvalidArgument, true
	case 1452: // ER_NO_REFERENCED_ROW_2 (Foreign key constraint fails)
		return kerrors.CodeInvalidArgument, true

	// Transaction errors
	case 1205: // ER_LOCK_WAIT_TIMEOUT (Lock wait timeout exceeded)
		return kerrors.CodeTimeout, true
	case 1213: // ER_LOCK_DEADLOCK (Deadlock found when trying to get lock)
		return kerrors.CodeConflict, true

	// Resource errors
	case 1030: // ER_FILE_NOT_FOUND
		return kerrors.CodeNotFound, true
	case 1037: // ER_OUTOFMEMORY
		return kerrors.CodeUnavailable, true
	case 1041: // ER_OUT_OF_RESOURCES
		return kerrors.CodeUnavailable, true

	// Timeout errors
	case 1159: // ER_NET_READ_TIMEOUT
		return kerrors.CodeTimeout, true
	case 1160: // ER_NET_WRITE_TIMEOUT
		return kerrors.CodeTimeout, true

	// Permission errors
	case 1142: // ER_TABLEACCESS_DENIED_ERROR
		return kerrors.CodePermission, true
	case 1143: // ER_COLUMNACCESS_DENIED_ERROR
		return kerrors.CodePermission, true

	default:
		// Unknown MySQL error
		return kerrors.CodeDatabase, true
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
	var kerr *kerrors.Error
	if !errors.As(err, &kerr) {
		return false
	}

	// Only retry transient errors
	switch kerr.Code {
	case kerrors.CodeUnavailable: // Database temporarily unavailable
		return true
	case kerrors.CodeTimeout: // Timeout (may succeed on retry)
		return true
	case kerrors.CodeConflict: // Deadlock or serialization failure
		return true
	default:
		return false
	}
}

// IsNotFound checks if the error is a "not found" error.
func IsNotFound(err error) bool {
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
		return true
	}

	var kerr *kerrors.Error
	if errors.As(err, &kerr) {
		return kerr.Code == kerrors.CodeNotFound
	}

	return false
}

// IsUniqueViolation checks if the error is a unique constraint violation.
func IsUniqueViolation(err error) bool {
	var kerr *kerrors.Error
	if errors.As(err, &kerr) {
		return kerr.Code == kerrors.CodeAlreadyExists
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
