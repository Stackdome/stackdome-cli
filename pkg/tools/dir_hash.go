package tools

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func ComputeDirHash(dir string) string {
	return genRandomHash()
}

func genRandomHash() string {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
		return ""
	}

	// Create a new SHA-256 hash
	hash := sha256.New()

	// Write the random bytes to the hash
	_, err = hash.Write(randomBytes[0:6])
	if err != nil {
		panic(err)
		return ""
	}

	// Get the hash sum
	hashSum := hash.Sum(nil)

	// Convert the hash sum to a hexadecimal string representation
	return hex.EncodeToString(hashSum[0:6])
}
