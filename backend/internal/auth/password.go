package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonKeyLength   = 32
	argonSaltLength  = 16
)

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 128 || strings.TrimSpace(password) == "" {
		return "", ErrInvalidPassword
	}
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonIterations, argonParallelism, encode(salt), encode(key)), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations uint64
	var parallelism uint8
	for _, param := range strings.Split(parts[3], ",") {
		name, value, ok := strings.Cut(param, "=")
		if !ok {
			return false
		}
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return false
		}
		switch name {
		case "m":
			memory = n
		case "t":
			iterations = n
		case "p":
			if n > 255 {
				return false
			}
			parallelism = uint8(n)
		default:
			return false
		}
	}
	if memory < 8*uint64(parallelism) || memory > argonMemory || iterations == 0 || iterations > argonIterations || parallelism == 0 {
		return false
	}
	decode := base64.RawStdEncoding.DecodeString
	salt, err := decode(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false
	}
	expected, err := decode(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
