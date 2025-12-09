# Karu Kits: Errors Package

A comprehensive, structured error handling package for Go, designed to provide semantic meaning, stack traces, and automatic protocol adaptation (HTTP, gRPC, CLI) for robust application development.

## 1. Overview

The `errors` package solves the problem of lossy error propagation in Go applications. It augments standard Go errors with:

*   **Stable Error Codes**: String-based codes (e.g., `NOT_FOUND`, `INVALID_ARGUMENT`) that form a contract between services.
*   **Stack Traces**: Automatically captured at the point of error creation or first wrap.
*   **Contextual Details**: Arbitrary key-value pairs (`map[string]any`) to carry metadata.
*   **Protocol Mapping**: Native conversion to HTTP Status/JSON, gRPC Status, and CLI formats without extra boilerplate.

## 2. Core Concepts

### 2.1 The Error Model

The core `Error` struct is the heart of this package. It implements the standard `error` interface.

```go
type Error struct {
    Code       Code             // The semantic error code (e.g., "NOT_FOUND")
    Message    string           // Human-readable error message
    Cause      error            // The underlying error (if wrapped)
    StackTrace []StackFrame     // Captured call stack
    Details    map[string]any   // Structured metadata
}
```

### 2.2 Error Codes

Error codes are typed as `Code` (string). They determine how an error maps to transport protocols (HTTP 4xx/5xx, gRPC status codes).

**Key Behavior:**
*   **Client Errors**: `INVALID_ARGUMENT`, `NOT_FOUND`, `ALREADY_EXISTS`, etc. map to 4xx.
*   **Server Errors**: `INTERNAL_ERROR`, `DATABASE_ERROR`, `NETWORK_ERROR` map to 5xx.

## 3. Error Codes Reference

| Code | HTTP Status | gRPC Code | Description |
| :--- | :--- | :--- | :--- |
| **Generic** | | | |
| `CodeInternal` | 500 | 13 (Internal) | Unexpected internal error. |
| `CodeUnknown` | 500 | 2 (Unknown) | Error logic could not determine the cause. |
| `CodeInvalidArgument` | 400 | 3 (InvalidArgument) | Client specified an invalid argument. |
| `CodeNotFound` | 404 | 5 (NotFound) | Resource was not found. |
| `CodeAlreadyExists` | 409 | 6 (AlreadyExists) | Resource already exists. |
| `CodePermission` | 403 | 7 (PermissionDenied) | Caller does not have permission. |
| `CodeUnauthenticated` | 401 | 16 (Unauthenticated) | Request does not have valid credentials. |
| `CodeTimeout` | 408 | 4 (DeadlineExceeded) | Request timed out. |
| `CodeCancelled` | 499 | 1 (Cancelled) | Request was cancelled by the client. |
| `CodeUnavailable` | 503 | 14 (Unavailable) | Service is currently unavailable. |
| `CodeUnimplemented` | 501 | 12 (Unimplemented) | Operation is not implemented. |
| `CodeConflict` | 409 | 6 (AlreadyExists) | Resource conflict (generic). |
| `CodeInvalidState` | 422 | 9 (FailedPrecondition) | System is not in a state to handle request. |
| **Infrastructure** | | | |
| `CodeDatabase` | 500 | 13 | Database operation failure. |
| `CodeNetwork` | 503 | 14 | Network communication failure. |
| `CodeThirdParty` | 500 | 13 | External third-party service failure. |
| `CodeCache` | 500 | 13 | Cache system failure. |
| `CodeQueue` | 500 | 13 | Message queue failure. |
| `CodeFileSystem` | 500 | 13 | File system I/O failure. |
| `CodeSerialization` | 500 | 13 | Data encoding/decoding failure. |

## 4. API Reference

### 4.1 Creating Errors

**`New(code Code, message string) *Error`**
Creates a new error with a stack trace.
```go
err := errors.New(errors.CodeNotFound, "user not found")
```

**`Newf(code Code, format string, args ...any) *Error`**
Creates a new error with formatted message and stack trace.
```go
err := errors.Newf(errors.CodeInternal, "failed to connect: %s", host)
```

### 4.2 Wrapping Errors

**`Wrap(err error, code Code, message string) *Error`**
Wraps an existing error.
*   If `err` is nil, returns nil.
*   If `err` is already an `*Error`, it preserves the original details and stack trace.
*   If `err` is a standard error, it captures a new stack trace.
```go
if err != nil {
    return errors.Wrap(err, errors.CodeDatabase, "query failed")
}
```

**`Wrapf(err error, code Code, format string, args ...any) *Error`**
Wraps with formatting.
```go
return errors.Wrapf(err, errors.CodeDatabase, "query failed for id %s", id)
```

### 4.3 Adding Context

**`WithDetail(key string, value any) *Error`**
Adds structured metadata to the error. This is useful for passing field validation errors or specific context ID.
```go
return errors.New(errors.CodeInvalidArgument, "validation error").
    WithDetail("field", "email").
    WithDetail("reason", "missing_at_symbol")
```

### 4.4 Inspecting Errors

**`IsCode(err error, code Code) bool`**
Checks if the error chain contains an error with the specific code.
```go
if errors.IsCode(err, errors.CodeNotFound) {
    // Handle 404
}
```

**`GetCode(err error) Code`**
Extracts the code from the error chain. Returns `CodeInternal` if not found.

**`GetDetails(err error) map[string]any`**
Extracts details map from the error.

**`Cause(err error) error`**
Unwraps the error chain to find the root cause.

## 5. Protocol Adapters

### 5.1 HTTP Adapter
Located in `http.go`. Used to automatically convert errors into clean JSON responses.

**`ToHTTPResponse(err error, includeStackTrace bool) HTTPResponse`**
Converts an error to a response struct containing the status code and JSON body.

**Response Format:**
```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "invalid input",
    "details": {
      "field": "age"
    },
    "stack_trace": [ ... ] // if enabled
  }
}
```

**Usage Example:**
```go
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    err := doSomething()
    if err != nil {
        resp := errors.ToHTTPResponse(err, false)
        w.WriteHeader(resp.StatusCode)
        w.Write([]byte(resp.MustToJSON()))
        return
    }
}
```

### 5.2 gRPC Adapter
Located in `grpc.go`.

**`ToGRPCError(err error) error`**
Converts the internal error to a gRPC `status.Error`. Use this in your gRPC handler returns.
*   Maps `Code` to `google.golang.org/grpc/codes`.
*   Preserves the error message.

**Usage Example:**
```go
func (s *Service) GetUser(ctx context.Context, req *Req) (*Res, error) {
    err := s.logic(ctx)
    if err != nil {
        return nil, errors.ToGRPCError(err)
    }
    return &Res{}, nil
}
```

### 5.3 CLI Adapter
Located in `cmd.go`.

**`ToCMDError(err error) string`**
Returns a single-line string formatted as `[CODE] Message`.

**`ToCMDErrorWithStack(err error) string`**
Returns the error message followed by the full stack trace.

**Usage Example:**
```go
if err := app.Run(); err != nil {
    fmt.Fprintln(os.Stderr, errors.ToCMDErrorWithStack(err))
    os.Exit(1)
}
```

## 6. Implementation Details

*   **Stack Traces**: Uses `runtime.Callers` to capture up to 32 frames. Frames are resolved to file/line/function using `runtime.FuncForPC`.
*   **Immutability**: The `New` and `Wrap` functions return pointers, but the `Code` type is a string constant. `WithDetail` mutates the details map of the specific error instance (builder pattern).
*   **Nil Safety**: All `Wrap` and conversion functions handle `nil` errors gracefully by returning `nil` or success equivalents.

## 7. Best Practices

1.  **Wrap at Boundaries**: When an error returns from a database or external library, wrap it with `errors.Wrap` to assign it a semantic code (e.g., `CodeDatabase`).
2.  **Don't Double Wrap**: If you call a function that already returns a `karu-kits/errors` type, usually you don't need to wrap it again unless you want to change the code or add context.
3.  **Use `WithDetail` for Validation**: Instead of putting dynamic data into the error string (message), put it in `Details`. This allows clients to parse specific failure fields programmatically.
4.  **Hide Stack Traces in Production**: When using `ToHTTPResponse`, pass `false` for `includeStackTrace` in production environments to avoid leaking internal paths.
