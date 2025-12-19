package otp

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"math"
	"strings"
	"time"
)

// Account represents a 2FA account
type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Issuer    string    `json:"issuer"`
	Secret    string    `json:"secret"`    // Base32 encoded
	Algorithm string    `json:"algorithm"` // SHA1, SHA256, SHA512, MD5
	Digits    int       `json:"digits"`    // 6 or 8
	Type      string    `json:"type"`      // TOTP or HOTP
	Counter   int64     `json:"counter"`   // For HOTP
	Period    int       `json:"period"`    // For TOTP, default 30
	CreatedAt time.Time `json:"created_at"`
	Group     string    `json:"group"`     // 分组名称（本工具独有）
}

// GenerateTOTP generates a TOTP code for the given account
func GenerateTOTP(secret, algorithm string, digits int, period int) (string, int, error) {
	if period == 0 {
		period = 30
	}

	// Get current time counter
	counter := time.Now().Unix() / int64(period)

	// Generate code
	code, err := generateCode(secret, counter, algorithm, digits)
	if err != nil {
		return "", 0, err
	}

	// Calculate remaining seconds
	remaining := period - int(time.Now().Unix()%int64(period))

	return code, remaining, nil
}

// GenerateHOTP generates a HOTP code for the given account
func GenerateHOTP(secret, algorithm string, digits int, counter int64) (string, error) {
	return generateCode(secret, counter, algorithm, digits)
}

// generateCode is the core HMAC-based One-Time Password algorithm
func generateCode(secret string, counter int64, algorithm string, digits int) (string, error) {
	// Decode base32 secret
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	secretBytes, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("invalid secret: %w", err)
	}

	// Create HMAC hash function
	var h func() hash.Hash
	switch strings.ToUpper(algorithm) {
	case "SHA1", "":
		h = sha1.New
	case "SHA256":
		h = sha256.New
	case "SHA512":
		h = sha512.New
	case "MD5":
		h = md5.New
	default:
		h = sha1.New
	}

	// Generate HMAC
	mac := hmac.New(h, secretBytes)
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, uint64(counter))
	mac.Write(counterBytes)
	hash := mac.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	// Generate OTP
	otp := truncated % uint32(math.Pow10(digits))

	// Format with leading zeros
	format := fmt.Sprintf("%%0%dd", digits)
	return fmt.Sprintf(format, otp), nil
}

// ValidateTOTP validates a TOTP code
func ValidateTOTP(secret, code, algorithm string, digits, period int) bool {
	generated, _, err := GenerateTOTP(secret, algorithm, digits, period)
	if err != nil {
		return false
	}
	return generated == code
}

// ValidateHOTP validates a HOTP code
func ValidateHOTP(secret, code, algorithm string, digits int, counter int64) bool {
	generated, err := GenerateHOTP(secret, algorithm, digits, counter)
	if err != nil {
		return false
	}
	return generated == code
}

// GetRemainingSeconds returns the remaining seconds until the next TOTP code
func GetRemainingSeconds(period int) int {
	if period == 0 {
		period = 30
	}
	return period - int(time.Now().Unix()%int64(period))
}

// GetProgress returns the progress percentage (0-100) for the current TOTP period
func GetProgress(period int) int {
	if period == 0 {
		period = 30
	}
	elapsed := int(time.Now().Unix() % int64(period))
	return (elapsed * 100) / period
}
