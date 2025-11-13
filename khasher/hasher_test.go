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

func TestHasherEmptyPassword(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	_, err = h.Hash(context.Background(), "")
	if !errors.Is(err, ErrPasswordEmpty) {
		t.Fatalf("expected ErrPasswordEmpty, got %v", err)
	}

	_, err = h.HashWith(context.Background(), AlgorithmBcrypt, "")
	if !errors.Is(err, ErrPasswordEmpty) {
		t.Fatalf("expected ErrPasswordEmpty for bcrypt, got %v", err)
	}
}

func TestHasherPasswordTooLong(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	longPassword := string(make([]byte, 73))
	_, err = h.Hash(context.Background(), longPassword)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
}

func TestHasherCanceledContext(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = h.Hash(ctx, "password")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestArgon2ConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Argon2Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Argon2Config{
				Time:        3,
				Memory:      64 * 1024,
				Parallelism: 2,
				KeyLength:   32,
				SaltLength:  16,
			},
			wantErr: false,
		},
		{
			name: "salt too short",
			cfg: Argon2Config{
				Time:        3,
				Memory:      64 * 1024,
				Parallelism: 2,
				KeyLength:   32,
				SaltLength:  7,
			},
			wantErr: true,
		},
		{
			name: "salt too long",
			cfg: Argon2Config{
				Time:        3,
				Memory:      64 * 1024,
				Parallelism: 2,
				KeyLength:   32,
				SaltLength:  65,
			},
			wantErr: true,
		},
		{
			name: "key too short",
			cfg: Argon2Config{
				Time:        3,
				Memory:      64 * 1024,
				Parallelism: 2,
				KeyLength:   15,
				SaltLength:  16,
			},
			wantErr: true,
		},
		{
			name: "memory too low",
			cfg: Argon2Config{
				Time:        3,
				Memory:      1023,
				Parallelism: 2,
				KeyLength:   32,
				SaltLength:  16,
			},
			wantErr: true,
		},
		{
			name: "memory too high",
			cfg: Argon2Config{
				Time:        3,
				Memory:      2*1024*1024 + 1,
				Parallelism: 2,
				KeyLength:   32,
				SaltLength:  16,
			},
			wantErr: true,
		},
		{
			name: "time cost too high",
			cfg: Argon2Config{
				Time:        101,
				Memory:      64 * 1024,
				Parallelism: 2,
				KeyLength:   32,
				SaltLength:  16,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(Config{
				Default: AlgorithmArgon2id,
				Argon2:  tt.cfg,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBcryptConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     BcryptConfig
		wantErr bool
	}{
		{
			name:    "valid cost",
			cfg:     BcryptConfig{Cost: 12},
			wantErr: false,
		},
		{
			name:    "cost too low",
			cfg:     BcryptConfig{Cost: 3},
			wantErr: true,
		},
		{
			name:    "cost too high",
			cfg:     BcryptConfig{Cost: 32},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(Config{
				Default: AlgorithmBcrypt,
				Bcrypt:  tt.cfg,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHasherCompareEmptyHash(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	if err := h.Compare(context.Background(), "", "password"); !errors.Is(err, ErrUnknownHashFormat) {
		t.Fatalf("expected ErrUnknownHashFormat, got %v", err)
	}

	if err := h.Compare(context.Background(), "   ", "password"); !errors.Is(err, ErrUnknownHashFormat) {
		t.Fatalf("expected ErrUnknownHashFormat for whitespace, got %v", err)
	}
}

func TestHasherAvailableAlgorithms(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	algs := h.AvailableAlgorithms()
	if len(algs) != 2 {
		t.Fatalf("expected 2 algorithms, got %d", len(algs))
	}

	// Should be sorted
	if algs[0] != AlgorithmArgon2id || algs[1] != AlgorithmBcrypt {
		t.Fatalf("expected [argon2id, bcrypt], got %v", algs)
	}
}

func TestHasherDefaultAlgorithm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Config
		want    Algorithm
		wantErr bool
	}{
		{
			name:    "default to argon2id",
			cfg:     Config{},
			want:    AlgorithmArgon2id,
			wantErr: false,
		},
		{
			name:    "explicit bcrypt",
			cfg:     Config{Default: AlgorithmBcrypt},
			want:    AlgorithmBcrypt,
			wantErr: false,
		},
		{
			name:    "unsupported algorithm",
			cfg:     Config{Default: "unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && h.DefaultAlgorithm() != tt.want {
				t.Errorf("DefaultAlgorithm() = %v, want %v", h.DefaultAlgorithm(), tt.want)
			}
		})
	}
}

func TestHasherUnsupportedAlgorithm(t *testing.T) {
	t.Parallel()

	h, err := New(Config{})
	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	_, err = h.HashWith(context.Background(), "unknown", "password")
	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("expected ErrUnsupportedAlgorithm, got %v", err)
	}
}
