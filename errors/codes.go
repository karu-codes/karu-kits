package errors

// Code represents an error code
type Code string

// Common error codes
const (
	// Generic errors
	CodeInternal        Code = "INTERNAL_ERROR"
	CodeUnknown         Code = "UNKNOWN_ERROR"
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeNotFound        Code = "NOT_FOUND"
	CodeAlreadyExists   Code = "ALREADY_EXISTS"
	CodePermission      Code = "PERMISSION_DENIED"
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeTimeout         Code = "TIMEOUT"
	CodeCancelled       Code = "CANCELLED"
	CodeUnavailable     Code = "UNAVAILABLE"
	CodeUnimplemented   Code = "UNIMPLEMENTED"
	CodeConflict        Code = "CONFLICT"
	CodeInvalidState    Code = "INVALID_STATE"

	// Infrastructure errors
	CodeDatabase      Code = "DATABASE_ERROR"
	CodeNetwork       Code = "NETWORK_ERROR"
	CodeThirdParty    Code = "THIRD_PARTY_ERROR"
	CodeCache         Code = "CACHE_ERROR"
	CodeQueue         Code = "QUEUE_ERROR"
	CodeFileSystem    Code = "FILESYSTEM_ERROR"
	CodeSerialization Code = "SERIALIZATION_ERROR"
)

// String returns the string representation of the code
func (c Code) String() string {
	return string(c)
}

// HTTPStatusCode returns the HTTP status code for the error code
func (c Code) HTTPStatusCode() int {
	switch c {
	case CodeInvalidArgument:
		return 400 // Bad Request
	case CodeUnauthenticated:
		return 401 // Unauthorized
	case CodePermission:
		return 403 // Forbidden
	case CodeNotFound:
		return 404 // Not Found
	case CodeAlreadyExists, CodeConflict:
		return 409 // Conflict
	case CodeTimeout:
		return 408 // Request Timeout
	case CodeCancelled:
		return 499 // Client Closed Request
	case CodeUnimplemented:
		return 501 // Not Implemented
	case CodeUnavailable, CodeNetwork:
		return 503 // Service Unavailable
	case CodeInvalidState:
		return 422 // Unprocessable Entity
	case CodeInternal, CodeUnknown, CodeDatabase, CodeThirdParty,
		CodeCache, CodeQueue, CodeFileSystem, CodeSerialization:
		return 500 // Internal Server Error
	default:
		return 500 // Internal Server Error
	}
}

// IsClientError returns true if the error is a client error (4xx)
func (c Code) IsClientError() bool {
	status := c.HTTPStatusCode()
	return status >= 400 && status < 500
}

// IsServerError returns true if the error is a server error (5xx)
func (c Code) IsServerError() bool {
	status := c.HTTPStatusCode()
	return status >= 500 && status < 600
}

// GRPCCode returns the gRPC status code for the error code
// This is useful if you're also using gRPC
func (c Code) GRPCCode() int {
	switch c {
	case CodeInvalidArgument:
		return 3 // InvalidArgument
	case CodeUnauthenticated:
		return 16 // Unauthenticated
	case CodePermission:
		return 7 // PermissionDenied
	case CodeNotFound:
		return 5 // NotFound
	case CodeAlreadyExists, CodeConflict:
		return 6 // AlreadyExists
	case CodeTimeout:
		return 4 // DeadlineExceeded
	case CodeCancelled:
		return 1 // Cancelled
	case CodeUnimplemented:
		return 12 // Unimplemented
	case CodeUnavailable, CodeNetwork:
		return 14 // Unavailable
	case CodeInvalidState:
		return 9 // FailedPrecondition
	case CodeInternal, CodeDatabase, CodeThirdParty,
		CodeCache, CodeQueue, CodeFileSystem, CodeSerialization:
		return 13 // Internal
	case CodeUnknown:
		return 2 // Unknown
	default:
		return 2 // Unknown
	}
}
