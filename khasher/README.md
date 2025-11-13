# khasher

A flexible, production-ready password hashing library for Go with support for multiple algorithms.

## Features

- **Multiple Algorithms**: Support for Argon2id (default) and bcrypt
- **Auto-detection**: Automatically detects hash format when comparing passwords
- **Secure Defaults**: Uses OWASP-recommended parameters
- **Context-aware**: All operations support context for cancellation and timeouts
- **Input Validation**: Comprehensive validation of passwords and configuration
- **Easy Migration**: Seamlessly migrate between hashing algorithms
- **Type-safe**: Strongly-typed configuration with compile-time checks

## Installation

```bash
go get github.com/karu-codes/karu-kits/khasher
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/karu-codes/karu-kits/khasher"
)

func main() {
    ctx := context.Background()

    // Create hasher with default settings (Argon2id)
    h, err := khasher.New(khasher.Config{})
    if err != nil {
        log.Fatal(err)
    }

    // Hash a password
    hash, err := h.Hash(ctx, "my-secure-password")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Hash:", hash)

    // Verify password
    err = h.Compare(ctx, hash, "my-secure-password")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Password verified!")
}
```

## Usage

### Basic Configuration

```go
// Use default settings (Argon2id with secure defaults)
h, err := khasher.New(khasher.Config{})
```

### Custom Argon2id Configuration

```go
h, err := khasher.New(khasher.Config{
    Default: khasher.AlgorithmArgon2id,
    Argon2: khasher.Argon2Config{
        Time:        3,          // Number of iterations
        Memory:      64 * 1024,  // Memory in KiB (64 MiB)
        Parallelism: 2,          // Degree of parallelism
        KeyLength:   32,         // Length of the hash in bytes
        SaltLength:  16,         // Length of the salt in bytes
    },
})
```

### Using bcrypt

```go
h, err := khasher.New(khasher.Config{
    Default: khasher.AlgorithmBcrypt,
    Bcrypt: khasher.BcryptConfig{
        Cost: 12, // bcrypt cost factor (4-31)
    },
})
```

### Hashing with Specific Algorithm

```go
// Hash with default algorithm
hash, err := h.Hash(ctx, password)

// Hash with specific algorithm
hash, err := h.HashWith(ctx, khasher.AlgorithmBcrypt, password)
```

### Password Verification

```go
// Compare automatically detects the hash format
err := h.Compare(ctx, storedHash, userPassword)
if err != nil {
    switch {
    case errors.Is(err, khasher.ErrPasswordMismatch):
        // Password doesn't match
    case errors.Is(err, khasher.ErrUnknownHashFormat):
        // Invalid hash format
    default:
        // Other error
    }
}
```

### Context Support

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

hash, err := h.Hash(ctx, password)

// With cancellation
ctx, cancel := context.WithCancel(context.Background())
// ... cancel when needed
```

## Error Handling

The library defines several sentinel errors:

- `ErrPasswordMismatch`: Password doesn't match the hash
- `ErrPasswordEmpty`: Password is empty
- `ErrPasswordTooLong`: Password exceeds 72 bytes (bcrypt limit)
- `ErrUnknownHashFormat`: Hash format not recognized
- `ErrUnsupportedAlgorithm`: Requested algorithm not available

```go
err := h.Compare(ctx, hash, password)
if errors.Is(err, khasher.ErrPasswordMismatch) {
    // Handle authentication failure
}
```

## Algorithm Migration

The library makes it easy to migrate between algorithms:

```go
// Old hasher using bcrypt
oldHasher, _ := khasher.New(khasher.Config{
    Default: khasher.AlgorithmBcrypt,
})

// New hasher using Argon2id
newHasher, _ := khasher.New(khasher.Config{
    Default: khasher.AlgorithmArgon2id,
})

// Both can verify existing hashes
if err := newHasher.Compare(ctx, oldBcryptHash, password); err == nil {
    // Verified! Now rehash with new algorithm
    newHash, _ := newHasher.Hash(ctx, password)
    // Store newHash in database
}
```

## Security Considerations

### Argon2id (Default)

Default parameters:
- Time: 3 iterations
- Memory: 64 MiB
- Parallelism: 2 threads
- Key length: 32 bytes
- Salt length: 16 bytes

These values are based on OWASP recommendations for general use. Adjust based on your security requirements and available resources.

### bcrypt

Default cost: 12

This provides a good balance between security and performance. Higher values increase security but also computation time.

### Password Length

Maximum password length: 72 bytes (bcrypt limitation)

While Argon2id supports longer passwords, we enforce this limit for consistency across algorithms.

## Validation

The library performs comprehensive validation:

### Argon2id Limits:
- Salt length: 8-64 bytes
- Key length: 16-128 bytes
- Memory: 1024 KiB - 2 GiB
- Parallelism: 1-255
- Time: 1-100 iterations

### bcrypt Limits:
- Cost: 4-31 (recommended: 10-14)

### Password Limits:
- Minimum: 1 byte (non-empty)
- Maximum: 72 bytes

## Performance

Approximate hashing times on modern hardware:

- **Argon2id** (default): ~100-300ms
- **bcrypt** (cost 12): ~300-500ms

These times are intentional to prevent brute-force attacks. Adjust parameters based on your security/performance requirements.

## Testing

```bash
go test -v ./khasher
```

Run benchmarks:

```bash
go test -bench=. ./khasher
```

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass
2. Code follows Go conventions
3. Security-sensitive changes are reviewed carefully
4. New features include tests

## License

[Your License Here]

## References

- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [Argon2 RFC 9106](https://datatracker.ietf.org/doc/html/rfc9106)
- [bcrypt](https://en.wikipedia.org/wiki/Bcrypt)
