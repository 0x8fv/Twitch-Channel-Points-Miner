package twitch

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, length)
	for i := range buf {
		nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		buf[i] = charset[nBig.Int64()]
	}
	return string(buf)
}

func randomHex(length int) string {
	if length <= 0 {
		return ""
	}
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return randomString(length)
	}
	return hex.EncodeToString(bytes)
}

func randomInt(min, max int) int {
	if max <= min {
		return min
	}
	nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return min + int(nBig.Int64())
}
