package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func SumSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func SignHMAC256(message, secret string) string {
	sig := hmac.New(sha256.New, []byte(secret))
	sig.Write([]byte(message))
	return hex.EncodeToString(sig.Sum(nil))
}

func EncodeB64(message string) string {
	return base64.StdEncoding.EncodeToString([]byte(message))
}

func DecodeB64(message string) string {
	if s, err := base64.StdEncoding.DecodeString(message); err == nil {
		return string(s)
	}
	return ""
}

func CreatePassword(password string) string {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	Check(err)
	return string(hashedPassword)
}

func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CreateToken(value, secret string, mins int) string {
	t := strconv.FormatInt(time.Now().UTC().Add(time.Duration(mins)*time.Minute).Unix(), 10)
	message := EncodeB64(value + "." + t)
	signature := SignHMAC256(message, secret)
	return message + "." + signature
}

func ValidateToken(token, secret string) (string, bool) {
	if token == "" {
		return "no token", false
	}
	parts := strings.Split(token, ".")
	signature := parts[1]
	if signature != SignHMAC256(parts[0], secret) {
		return "signature mismatch", false
	}
	parts = strings.Split(DecodeB64(parts[0]), ".")
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "can't parse time", false
	}
	if time.Now().UTC().Unix() >= expiry {
		return "expired", false
	}
	return parts[0], true
}
