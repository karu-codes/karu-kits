package khasher_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/karu-codes/karu-kits/khash/khasher"
)

func ExampleHasher() {
	ctx := context.Background()
	h, err := khasher.New(khasher.Config{})
	if err != nil {
		panic(err)
	}

	hash, err := h.Hash(ctx, "hunter2")
	if err != nil {
		panic(err)
	}

	fmt.Println("default:", h.DefaultAlgorithm())
	fmt.Println("registered:", h.AvailableAlgorithms())
	fmt.Println("argon2 hash:", strings.HasPrefix(hash, "$argon2id$"))
	fmt.Println("compare:", h.Compare(ctx, hash, "hunter2"))
	fmt.Println("mismatch:", errors.Is(h.Compare(ctx, hash, "nope"), khasher.ErrPasswordMismatch))

	// Output:
	// default: argon2id
	// registered: [argon2id bcrypt]
	// argon2 hash: true
	// compare: <nil>
	// mismatch: true
}
