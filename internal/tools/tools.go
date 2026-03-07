package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"unicode"
)

func CheckOrderID(orderID string) bool {
	for _, r := range orderID {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	sum := 0
	for i := len(orderID) - 1; i >= 0; i-- {
		digit, _ := strconv.Atoi(string(orderID[i]))
		if (len(orderID)-i)%2 == 0 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}

func HashSha256(password string) string {
	data := sha256.Sum256([]byte(password))
	return hex.EncodeToString(data[:])
}
