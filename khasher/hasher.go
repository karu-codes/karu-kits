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
	return h.HashWith(ctx, h.defaultAlg, password)
}

// HashWith produces a password hash using a specific algorithm.
func (h *Hasher) HashWith(ctx context.Context, alg Algorithm, password string) (string, error) {
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
