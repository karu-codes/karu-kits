package khasher

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Config customizes the Argon2id parameters.
type Argon2Config struct {
	Time        uint32
	Memory      uint32
	Parallelism uint8
	KeyLength   uint32
	SaltLength  uint32
}

func (c *Argon2Config) setDefaults() {
	if c.Time == 0 {
		c.Time = 3
	}
	if c.Memory == 0 {
		c.Memory = 64 * 1024
	}
	if c.Parallelism == 0 {
		c.Parallelism = 2
	}
	if c.KeyLength == 0 {
		c.KeyLength = 32
	}
	if c.SaltLength == 0 {
		c.SaltLength = 16
	}
}

type argon2Strategy struct {
	cfg Argon2Config
}

func newArgon2Strategy(cfg Argon2Config) (strategy, error) {
	switch {
	case cfg.SaltLength < 8:
		return nil, errors.New("khasher: argon2 salt length must be >= 8 bytes")
	case cfg.KeyLength < 16:
		return nil, errors.New("khasher: argon2 key length must be >= 16 bytes")
	case cfg.Parallelism == 0:
		return nil, errors.New("khasher: argon2 parallelism must be > 0")
	case cfg.Memory < 1024:
		return nil, errors.New("khasher: argon2 memory must be >= 1024 kib")
	case cfg.Time == 0:
		return nil, errors.New("khasher: argon2 time cost must be > 0")
	}

	return &argon2Strategy{cfg: cfg}, nil
}

func (s *argon2Strategy) hash(_ context.Context, password string) (string, error) {
	salt := make([]byte, s.cfg.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("khasher: generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, s.cfg.Time, s.cfg.Memory, s.cfg.Parallelism, s.cfg.KeyLength)

	return encodeArgon2Hash(salt, hash, s.cfg), nil
}

func (s *argon2Strategy) compare(_ context.Context, encoded, password string) error {
	params, salt, hash, err := decodeArgon2Hash(encoded)
	if err != nil {
		return err
	}

	derived := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.parallelism, params.keyLen)
	if subtle.ConstantTimeCompare(hash, derived) == 1 {
		return nil
	}
	return ErrPasswordMismatch
}

func (s *argon2Strategy) canHandle(hashed string) bool {
	return strings.HasPrefix(hashed, "$argon2")
}

func encodeArgon2Hash(salt, hash []byte, cfg Argon2Config) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		cfg.Memory,
		cfg.Time,
		cfg.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

type argon2Params struct {
	time        uint32
	memory      uint32
	parallelism uint8
	keyLen      uint32
}

func decodeArgon2Hash(encoded string) (argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return argon2Params{}, nil, nil, ErrUnknownHashFormat
	}
	if !strings.HasPrefix(parts[1], "argon2") {
		return argon2Params{}, nil, nil, ErrUnknownHashFormat
	}

	if _, err := strconv.Atoi(strings.TrimPrefix(parts[2], "v=")); err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("khasher: invalid argon2 version: %w", err)
	}

	paramsPart := parts[3]
	sub := strings.Split(paramsPart, ",")
	if len(sub) != 3 {
		return argon2Params{}, nil, nil, ErrUnknownHashFormat
	}

	var params argon2Params
	for _, chunk := range sub {
		switch {
		case strings.HasPrefix(chunk, "m="):
			val, err := parseUint32(chunk, "m=")
			if err != nil {
				return argon2Params{}, nil, nil, err
			}
			params.memory = val
		case strings.HasPrefix(chunk, "t="):
			val, err := parseUint32(chunk, "t=")
			if err != nil {
				return argon2Params{}, nil, nil, err
			}
			params.time = val
		case strings.HasPrefix(chunk, "p="):
			val, err := parseUint32(chunk, "p=")
			if err != nil {
				return argon2Params{}, nil, nil, err
			}
			if val > uint32(^uint8(0)) {
				return argon2Params{}, nil, nil, fmt.Errorf("khasher: argon2 parallelism out of range")
			}
			params.parallelism = uint8(val)
		}
	}

	if params.memory == 0 || params.time == 0 || params.parallelism == 0 {
		return argon2Params{}, nil, nil, ErrUnknownHashFormat
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("khasher: decode argon2 salt: %w", err)
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("khasher: decode argon2 hash: %w", err)
	}
	params.keyLen = uint32(len(hash))
	return params, salt, hash, nil
}

func parseUint32(value, prefix string) (uint32, error) {
	parsed, err := strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("khasher: parse %s: %w", prefix, err)
	}
	return uint32(parsed), nil
}
