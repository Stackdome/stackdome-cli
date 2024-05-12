package tools

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func ComputeDirHash(dir string, volumeName string) string {
	return GenRandomHash()
}

func GenRandomHash() string {
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return ""
	}

	// Create a new SHA-256 hash
	hash := sha256.New()

	// Write the random bytes to the hash
	_, err = hash.Write(randomBytes[0:6])
	if err != nil {
		return ""
	}

	// Get the hash sum
	hashSum := hash.Sum(nil)

	// Convert the hash sum to a hexadecimal string representation
	return hex.EncodeToString(hashSum)
}
