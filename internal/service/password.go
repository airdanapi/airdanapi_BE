package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonSaltLength  = 16
	argonKeyLength   = 32
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}

	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false
	}

	memory, iterations, parallelism, err := parseArgonParams(params)
	if err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	actualHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}

func parseArgonParams(parts []string) (uint32, uint32, uint8, error) {
	memory, err := parseUintParam(parts[0], "m")
	if err != nil {
		return 0, 0, 0, err
	}
	iterations, err := parseUintParam(parts[1], "t")
	if err != nil {
		return 0, 0, 0, err
	}
	parallelism, err := parseUintParam(parts[2], "p")
	if err != nil {
		return 0, 0, 0, err
	}
	if parallelism > 255 {
		return 0, 0, 0, errors.New("parallelism is too large")
	}
	return uint32(memory), uint32(iterations), uint8(parallelism), nil
}

func parseUintParam(part, key string) (uint64, error) {
	prefix := key + "="
	if !strings.HasPrefix(part, prefix) {
		return 0, errors.New("argon parameter is missing")
	}
	return strconv.ParseUint(strings.TrimPrefix(part, prefix), 10, 32)
}
