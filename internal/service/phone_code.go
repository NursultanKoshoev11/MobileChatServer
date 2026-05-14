package service

import (
	"crypto/rand"
	"strings"
)

func newNumericCode(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var builder strings.Builder
	for _, value := range buf {
		builder.WriteByte(byte('0' + value%10))
	}
	return builder.String(), nil
}
