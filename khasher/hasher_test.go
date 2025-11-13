package khasher

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHasherHashAndCompareArgon2(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	const password = "secret-password"
	hash, err := h.Hash(context.Background(), password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected argon2id hash prefix, got %q", hash)
	}
	if err := h.Compare(context.Background(), hash, password); err != nil {
		t.Fatalf("compare: %v", err)
	}
	if err := h.Compare(context.Background(), hash, "mismatch"); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("expected ErrPasswordMismatch, got %v", err)
	}
}

func TestHasherCompareBcrypt(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	const password = "bcrypt-password"
	hash, err := h.HashWith(context.Background(), AlgorithmBcrypt, password)
	if err != nil {
		t.Fatalf("hash with bcrypt: %v", err)
	}
	if err := h.Compare(context.Background(), hash, password); err != nil {
		t.Fatalf("compare bcrypt: %v", err)
	}
}

func TestHasherCompareUnknownFormat(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	if err := h.Compare(context.Background(), "plain-text", "password"); !errors.Is(err, ErrUnknownHashFormat) {
		t.Fatalf("expected ErrUnknownHashFormat, got %v", err)
	}
}
