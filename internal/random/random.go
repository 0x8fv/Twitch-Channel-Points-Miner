// Package random makes the nonces, ids and jittered delays sent to Twitch.
package random

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

// String returns length random letters and digits.
func String(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, length)
	for i := range buf {
		nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		buf[i] = charset[nBig.Int64()]
	}
	return string(buf)
}

// Hex returns length random bytes written as hexadecimal.
func Hex(length int) string {
	if length <= 0 {
		return ""
	}
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return String(length)
	}
	return hex.EncodeToString(bytes)
}

// Int returns a random number from min to max, both included.
func Int(min, max int) int {
	if max <= min {
		return min
	}
	nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return min + int(nBig.Int64())
}
