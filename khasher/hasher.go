// Package khasher provides a flexible password hashing library with support
// for multiple algorithms including Argon2id and bcrypt.
//
// Features:
//   - Multiple algorithm support via a pluggable strategy pattern
//   - Automatic hash format detection for comparison
//   - Secure defaults (Argon2id with OWASP-recommended parameters)
//   - Context-aware operations for cancellation and timeouts
//   - Comprehensive input validation
//
// Basic usage:
//
//	h, err := khasher.New(khasher.Config{})
//	if err != nil {
//	    return err
//	}
//
//	// Hash a password using the default algorithm (Argon2id)
//	hash, err := h.Hash(ctx, "my-password")
//	if err != nil {
//	    return err
//	}
//
//	// Compare a password against a stored hash
//	if err := h.Compare(ctx, hash, "my-password"); err != nil {
//	    if errors.Is(err, khasher.ErrPasswordMismatch) {
//	        // Password doesn't match
//	    }
//	    return err
//	}
//
// Custom configuration:
//
//	h, err := khasher.New(khasher.Config{
//	    Default: khasher.AlgorithmArgon2id,
//	    Argon2: khasher.Argon2Config{
//	        Time:        3,
//	        Memory:      64 * 1024, // 64 MiB
//	        Parallelism: 2,
//	        KeyLength:   32,
//	        SaltLength:  16,
//	    },
//	})
//
// The Compare method automatically detects the hash format, making it easy
// to migrate between algorithms over time. Simply hash new passwords with
// a different algorithm while still being able to verify old hashes.
package khasher

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Algorithm represents the hashing algorithm identifier.
type Algorithm string

const (
	// AlgorithmArgon2id uses the Argon2id KDF.
	AlgorithmArgon2id Algorithm = "argon2id"
	// AlgorithmBcrypt uses the bcrypt password hashing scheme.
	AlgorithmBcrypt Algorithm = "bcrypt"
)

var (
	// ErrUnsupportedAlgorithm indicates an unsupported hashing algorithm selection.
	ErrUnsupportedAlgorithm = errors.New("khasher: unsupported algorithm")
	// ErrUnknownHashFormat indicates that no registered strategy can handle the provided hash.
	ErrUnknownHashFormat = errors.New("khasher: unknown hash format")
	// ErrPasswordMismatch indicates that the provided password does not match the hashed value.
	ErrPasswordMismatch = errors.New("khasher: password mismatch")
	// ErrPasswordEmpty indicates that the password is empty.
	ErrPasswordEmpty = errors.New("khasher: password cannot be empty")
	// ErrPasswordTooLong indicates that the password exceeds the maximum length.
	ErrPasswordTooLong = errors.New("khasher: password too long (max 72 bytes)")
)

// Config drives the construction of a Hasher.
type Config struct {
	// Default defines which algorithm Hash will use when none is specified.
	Default Algorithm
	// Argon2 customizes the Argon2id strategy.
	Argon2 Argon2Config
	// Bcrypt customizes the bcrypt strategy.
	Bcrypt BcryptConfig
}

// Hasher exposes high-level helpers to hash and compare passwords.
type Hasher struct {
	defaultAlg Algorithm
	strategies map[Algorithm]strategy
}

// New constructs a Hasher using the provided configuration.
func New(cfg Config) (*Hasher, error) {
	cfg.setDefaults()

	strats := make(map[Algorithm]strategy, 2)

	argon2Strategy, err := newArgon2Strategy(cfg.Argon2)
	if err != nil {
		return nil, err
	}
	strats[AlgorithmArgon2id] = argon2Strategy

	bcryptStrategy, err := newBcryptStrategy(cfg.Bcrypt)
	if err != nil {
		return nil, err
	}
	strats[AlgorithmBcrypt] = bcryptStrategy

	if _, ok := strats[cfg.Default]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, cfg.Default)
	}

	return &Hasher{
		defaultAlg: cfg.Default,
		strategies: strats,
	}, nil
}

// Hash produces a password hash using the default algorithm.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	return h.HashWith(ctx, h.defaultAlg, password)
}

// HashWith produces a password hash using a specific algorithm.
func (h *Hasher) HashWith(ctx context.Context, alg Algorithm, password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	strat, ok := h.strategies[alg]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
	return strat.hash(ctx, password)
}

// Compare attempts to match the provided password against the stored hash.
// The hash format determines which strategy is used.
func (h *Hasher) Compare(ctx context.Context, hashed, password string) error {
	hashed = strings.TrimSpace(hashed)
	if hashed == "" {
		return ErrUnknownHashFormat
	}

	for _, strat := range h.orderedStrategies() {
		if strat.canHandle(hashed) {
			return strat.compare(ctx, hashed, password)
		}
	}

	// Fallback to the default strategy if detection failed.
	if strat, ok := h.strategies[h.defaultAlg]; ok {
		if err := strat.compare(ctx, hashed, password); err == nil {
			return nil
		}
	}

	return ErrUnknownHashFormat
}

// DefaultAlgorithm returns the algorithm used for Hash.
func (h *Hasher) DefaultAlgorithm() Algorithm {
	return h.defaultAlg
}

// AvailableAlgorithms lists all registered algorithms in deterministic order.
func (h *Hasher) AvailableAlgorithms() []Algorithm {
	algs := make([]Algorithm, 0, len(h.strategies))
	for alg := range h.strategies {
		algs = append(algs, alg)
	}
	sort.Slice(algs, func(i, j int) bool { return algs[i] < algs[j] })
	return algs
}

func (cfg *Config) setDefaults() {
	cfg.Argon2.setDefaults()
	cfg.Bcrypt.setDefaults()
	if cfg.Default == "" {
		cfg.Default = AlgorithmArgon2id
	}
}

func (h *Hasher) orderedStrategies() []strategy {
	algs := h.AvailableAlgorithms()
	strats := make([]strategy, 0, len(algs))
	for _, alg := range algs {
		strats = append(strats, h.strategies[alg])
	}
	return strats
}

type strategy interface {
	hash(context.Context, string) (string, error)
	compare(context.Context, string, string) error
	canHandle(string) bool
}

func validatePassword(password string) error {
	if password == "" {
		return ErrPasswordEmpty
	}
	// bcrypt has a limit of 72 bytes, so we enforce this for all algorithms
	if len(password) > 72 {
		return ErrPasswordTooLong
	}
	return nil
}
