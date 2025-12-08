# Errors Package

A comprehensive error handling package for Go applications with structured error codes, stack traces, and built-in support for HTTP, gRPC, and CLI (command-line) outputs.

## Features

- **Structured Error Codes**: Predefined, semantically meaningful error codes.
- **Stack Trace Capture**: Automatically captures stack traces for debugging.
- **Context Awareness**: Supports error wrapping and unwrapping.
- **Multi-Protocol Support**:
    - **HTTP**: Automatic mapping to HTTP status codes and JSON responses.
    - **gRPC**: Automatic mapping to gRPC status codes.
    - **CLI**: Formatted output for terminal applications.
- **Metadata**: Attach arbitrary key-value details to errors.

## Installation

```bash
go get github.com/karu-codes/karu-kits/errors
```

## Quick Start

### 1. HTTP Service Example

**Handler Implementation:**

```go
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    if err := h.svc.CreateUser(r.Context(), req); err != nil {
        // Automatically converts error to HTTP response with appropriate status code
        response := errors.ToHTTPResponse(err, false) // false = hide stack trace
        
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(response.StatusCode)
        w.Write([]byte(response.MustToJSON())) // or json.NewEncoder(w).Encode(response)
        return
    }
    // ...
}
```

**JSON Response Output:**

*400 Bad Request*
```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "invalid email format",
    "details": {
      "field": "email",
      "value": "invalid-email"
    }
  }
}
```

---

### 2. gRPC Service Example

**Service Implementation:**

```go
func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    user, err := s.repo.FindUser(ctx, req.Id)
    if err != nil {
        // wrap internal error and convert to gRPC error
        // if err is CodeNotFound, gRPC code will be NotFound (5)
        return nil, errors.ToGRPCError(err) 
    }
    return user, nil
}
```

**gRPC Response (Status):**

If `repo.FindUser` returns a `CodeNotFound` error:
- **gRPC Code**: `5` (NotFound)
- **Message**: "user not found"

Client side usage:
```go
resp, err := client.GetUser(ctx, &pb.GetUserRequest{Id: "123"})
if err != nil {
    st, _ := status.FromError(err)
    if st.Code() == codes.NotFound {
        // Handle not found
    }
}
```

---

### 3. CLI (Command Line) Example

**Main Function:**

```go
func main() {
    if err := run(); err != nil {
        // Format error for terminal output
        fmt.Fprintln(os.Stderr, errors.ToCMDError(err))
        
        // For debug mode with stack trace:
        // fmt.Fprintln(os.Stderr, errors.ToCMDErrorWithStack(err))
        os.Exit(1)
    }
}

func run() error {
    return errors.New(errors.CodeTimeout, "operation timed out")
}
```

**Terminal Output:**

```text
[TIMEOUT] operation timed out
```

**With Stack Trace:**

```text
[TIMEOUT] operation timed out
Stack Trace:
  at main.run:25 /path/to/main.go
  at main.main:12 /path/to/main.go
```

---

## Core Concepts

### Creating Errors

```go
// Simple
err := errors.New(errors.CodeNotFound, "item not found")

// With Formatting
err := errors.Newf(errors.CodeInternal, "failed to connect to %s", host)

// With Details
err := errors.New(errors.CodeInvalidArgument, "validation failed").
    WithDetail("field", "username").
    WithDetail("reason", "too short")
```

### Wrapping Errors

Wrapping preserves the original error stack trace and adds context.

```go
if err := db.Query(); err != nil {
    // Wrap external errors with a code and message
    return errors.Wrap(err, errors.CodeDatabase, "failed to query users")
}
```

### Checking Errors

```go
// Check specific code
if errors.IsCode(err, errors.CodeNotFound) {
    // handle not found
}

// Get the code
code := errors.GetCode(err) // e.g. "NOT_FOUND"

// Get details
details := errors.GetDetails(err) // map[string]any
```

## Error Codes Reference

| Code | HTTP | gRPC | Description |
|------|------|------|-------------|
| `CodeInternal` | 500 | 13 | Internal server error |
| `CodeInvalidArgument` | 400 | 3 | Invalid input arguments |
| `CodeNotFound` | 404 | 5 | Resource not found |
| `CodeAlreadyExists` | 409 | 6 | Resource already exists |
| `CodePermission` | 403 | 7 | Permission denied |
| `CodeUnauthenticated` | 401 | 16 | Not authenticated |
| `CodeTimeout` | 408 | 4 | Request timeout |
| `CodeUnavailable` | 503 | 14 | Service unavailable |
| `CodeUnknown` | 500 | 2 | Unknown error |

*(See `codes.go` for the full list)*

## Contributing

1. Fork the repo.
2. Create a feature branch.
3. Commit your changes.
4. Push and create a Pull Request.

## License

Part of the **karu-kits** project.
