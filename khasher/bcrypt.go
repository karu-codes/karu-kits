package khasher

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// BcryptConfig customizes the bcrypt strategy.
type BcryptConfig struct {
	Cost int
}

func (c *BcryptConfig) setDefaults() {
	if c.Cost == 0 {
		c.Cost = 12
	}
}

type bcryptStrategy struct {
	cost int
}

func newBcryptStrategy(cfg BcryptConfig) (strategy, error) {
	if cfg.Cost < bcrypt.MinCost || cfg.Cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("khasher: bcrypt cost must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost)
	}
	return &bcryptStrategy{cost: cfg.Cost}, nil
}

func (s *bcryptStrategy) hash(ctx context.Context, password string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return "", fmt.Errorf("khasher: bcrypt hash: %w", err)
	}
	return string(hashed), nil
}

func (s *bcryptStrategy) compare(ctx context.Context, hashed, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrPasswordMismatch
		}
		return fmt.Errorf("khasher: bcrypt compare: %w", err)
	}
	return nil
}

func (s *bcryptStrategy) canHandle(hashed string) bool {
	return strings.HasPrefix(hashed, "$2a$") ||
		strings.HasPrefix(hashed, "$2b$") ||
		strings.HasPrefix(hashed, "$2y$")
}
