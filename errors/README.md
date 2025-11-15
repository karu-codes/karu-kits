# Errors Package

A comprehensive error handling package for Go applications with structured error codes, stack traces, and HTTP/gRPC integration.

## Features

- **Structured Error Codes**: Predefined error codes for common scenarios
- **Stack Trace Capture**: Automatic stack trace recording for debugging
- **Error Wrapping**: Context-preserving error wrapping
- **HTTP Integration**: Automatic HTTP status code mapping
- **gRPC Support**: gRPC status code mapping
- **Error Details**: Attach arbitrary metadata to errors
- **Type-Safe**: Strongly typed error codes and structures

## Installation

```bash
go get github.com/karu-codes/karu-kits/errors
```

## Quick Start

### Creating Errors

```go
import "github.com/karu-codes/karu-kits/errors"

// Simple error
err := errors.New(errors.CodeNotFound, "user not found")

// Formatted error message
err := errors.Newf(errors.CodeInvalidArgument, "invalid email: %s", email)

// Error with additional details
err := errors.New(errors.CodePermission, "access denied").
    WithDetail("user_id", userID).
    WithDetail("resource", "admin_panel")
```

### Wrapping Errors

```go
// Wrap an existing error with context
dbErr := db.Query(...)
if dbErr != nil {
    return errors.Wrap(dbErr, errors.CodeDatabase, "failed to fetch user")
}

// Wrap with formatted message
err := errors.Wrapf(dbErr, errors.CodeDatabase, "failed to fetch user %s", userID)
```

### Checking Errors

```go
// Check if error has specific code
if errors.HasCode(err, errors.CodeNotFound) {
    // Handle not found
}

// Get error code
code := errors.GetCode(err)

// Get error details
details := errors.GetDetails(err)
userID := details["user_id"]

// Standard library compatibility
if errors.Is(err, someError) {
    // ...
}

var customErr *errors.Error
if errors.As(err, &customErr) {
    // Access custom error fields
}
```

## Error Codes

### Generic Errors

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `CodeInternal` | 500 | Internal server error |
| `CodeUnknown` | 500 | Unknown error |
| `CodeInvalidArgument` | 400 | Invalid input/argument |
| `CodeNotFound` | 404 | Resource not found |
| `CodeAlreadyExists` | 409 | Resource already exists |
| `CodePermission` | 403 | Permission denied |
| `CodeUnauthenticated` | 401 | Authentication required |
| `CodeTimeout` | 408 | Request timeout |
| `CodeCancelled` | 499 | Request cancelled |
| `CodeUnavailable` | 503 | Service unavailable |
| `CodeUnimplemented` | 501 | Not implemented |
| `CodeConflict` | 409 | Resource conflict |
| `CodeInvalidState` | 422 | Invalid state transition |

### Infrastructure Errors

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `CodeDatabase` | 500 | Database error |
| `CodeNetwork` | 503 | Network error |
| `CodeThirdParty` | 500 | Third-party service error |
| `CodeCache` | 500 | Cache error |
| `CodeQueue` | 500 | Queue error |
| `CodeFileSystem` | 500 | File system error |
| `CodeSerialization` | 500 | Serialization error |

## HTTP Integration

### Converting to HTTP Errors

```go
// Convert error to HTTP error structure
httpErr := errors.ToHTTPError(err, includeStackTrace)

// Get HTTP status code from error
statusCode := errors.HTTPStatusCode(err)

// Convert to full HTTP response
response := errors.ToHTTPResponse(err, includeStackTrace)

// Write as JSON
jsonStr, _ := response.WriteJSON()
```

### HTTP Response Format

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "user not found",
    "details": {
      "user_id": "12345"
    },
    "stack_trace": [
      "main.getUser at /app/handler.go:42",
      "main.handleRequest at /app/handler.go:23"
    ]
  }
}
```

### Example HTTP Handler

```go
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := h.userService.GetByID(userID)
    if err != nil {
        response := errors.ToHTTPResponse(err, h.config.Debug)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(response.StatusCode)

        jsonStr, _ := response.WriteJSON()
        w.Write([]byte(jsonStr))
        return
    }

    // Success response...
}
```

## Code Methods

### HTTP Status Mapping

```go
code := errors.CodeNotFound

// Get HTTP status code
statusCode := code.HTTPStatusCode() // 404

// Check error category
isClientError := code.IsClientError() // true
isServerError := code.IsServerError() // false
```

### gRPC Integration

```go
code := errors.CodeInvalidArgument

// Get gRPC code
grpcCode := code.GRPCCode() // 3 (InvalidArgument)
```

## Advanced Usage

### Custom Error Codes

```go
const (
    CodeCustom errors.Code = "CUSTOM_ERROR"
)

err := errors.New(CodeCustom, "custom error occurred")
```

### Accessing Stack Traces

```go
var customErr *errors.Error
if errors.As(err, &customErr) {
    for _, frame := range customErr.StackTrace {
        fmt.Printf("%s:%d in %s\n", frame.File, frame.Line, frame.Function)
    }
}
```

### Error Details

```go
err := errors.New(errors.CodeInvalidArgument, "validation failed").
    WithDetail("field", "email").
    WithDetail("reason", "invalid format").
    WithDetail("value", email)

// Later retrieve details
details := errors.GetDetails(err)
field := details["field"].(string)
```

### Finding Root Cause

```go
// Get the root cause of wrapped errors
rootErr := errors.Cause(err)
```

## Best Practices

### 1. Use Appropriate Error Codes

Choose error codes that accurately reflect the failure:

```go
// Good: Specific error code
if user == nil {
    return errors.New(errors.CodeNotFound, "user not found")
}

// Bad: Generic error code
if user == nil {
    return errors.New(errors.CodeInternal, "user not found")
}
```

### 2. Add Context When Wrapping

Always add meaningful context when wrapping errors:

```go
// Good: Adds context
err := repo.GetUser(id)
if err != nil {
    return errors.Wrapf(err, errors.CodeDatabase, "failed to get user %s", id)
}

// Bad: No additional context
err := repo.GetUser(id)
if err != nil {
    return errors.Wrap(err, errors.CodeDatabase, "database error")
}
```

### 3. Use Details for Structured Data

Use details for machine-readable data:

```go
return errors.New(errors.CodeInvalidArgument, "validation failed").
    WithDetail("field", "email").
    WithDetail("constraint", "format").
    WithDetail("received", email)
```

### 4. Control Stack Trace Exposure

Only include stack traces in development/debug mode:

```go
// Production: hide stack traces
response := errors.ToHTTPResponse(err, false)

// Development: show stack traces
response := errors.ToHTTPResponse(err, true)
```

### 5. Handle nil Errors

The package handles nil errors gracefully:

```go
// These are safe
errors.Wrap(nil, code, msg)    // returns nil
errors.Wrapf(nil, code, msg)   // returns nil
errors.HTTPStatusCode(nil)     // returns 200
```

## Error Structure

### Error Type

```go
type Error struct {
    Code       Code              // Error code
    Message    string            // Human-readable message
    Cause      error             // Wrapped error (if any)
    StackTrace []StackFrame      // Stack trace
    Details    map[string]any    // Additional metadata
}
```

### Stack Frame

```go
type StackFrame struct {
    File     string  // File path
    Line     int     // Line number
    Function string  // Function name
}
```

## Migration Guide

### From Standard Library

```go
// Before
return fmt.Errorf("user not found: %w", err)

// After
return errors.Wrapf(err, errors.CodeNotFound, "user not found")
```

### From pkg/errors

```go
// Before
return pkgerrors.Wrap(err, "database error")

// After
return errors.Wrap(err, errors.CodeDatabase, "database error")
```

## Contributing

When adding new error codes:

1. Add the constant to [codes.go](codes.go)
2. Update the HTTP status code mapping in `Code.HTTPStatusCode()`
3. Update the gRPC code mapping in `Code.GRPCCode()`
4. Document the new code in this README

## License

Part of the karu-kits project.
