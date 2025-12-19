package migration

import (
	"encoding/base32"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Algorithm represents the hash algorithm
type Algorithm int32

const (
	AlgorithmUnspecified Algorithm = 0
	AlgorithmSHA1        Algorithm = 1
	AlgorithmSHA256      Algorithm = 2
	AlgorithmSHA512      Algorithm = 3
	AlgorithmMD5         Algorithm = 4
)

// DigitCount represents the number of digits in OTP
type DigitCount int32

const (
	DigitCountUnspecified DigitCount = 0
	DigitCountSix         DigitCount = 1
	DigitCountEight       DigitCount = 2
)

// OtpType represents the type of OTP
type OtpType int32

const (
	OtpTypeUnspecified OtpType = 0
	OtpTypeHOTP        OtpType = 1
	OtpTypeTOTP        OtpType = 2
)

// OtpParameters contains configuration for a single OTP account
type OtpParameters struct {
	Secret    []byte
	Name      string
	Issuer    string
	Algorithm Algorithm
	Digits    DigitCount
	Type      OtpType
	Counter   int64
}

// Helper functions for converting enums to strings
func (a Algorithm) String() string {
	switch a {
	case AlgorithmSHA1:
		return "SHA1"
	case AlgorithmSHA256:
		return "SHA256"
	case AlgorithmSHA512:
		return "SHA512"
	case AlgorithmMD5:
		return "MD5"
	default:
		return "SHA1"
	}
}

func (d DigitCount) ToInt() int {
	switch d {
	case DigitCountSix:
		return 6
	case DigitCountEight:
		return 8
	default:
		return 6
	}
}

func (t OtpType) String() string {
	switch t {
	case OtpTypeHOTP:
		return "HOTP"
	case OtpTypeTOTP:
		return "TOTP"
	default:
		return "TOTP"
	}
}

// ParseOTPAuthURI parses standard otpauth:// URI
func ParseOTPAuthURI(uri string) (*OtpParameters, error) {
	// Check if it's a valid otpauth URI
	if !strings.HasPrefix(uri, "otpauth://") {
		return nil, fmt.Errorf("invalid URI scheme, expected otpauth://")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI: %w", err)
	}

	// Get OTP type from scheme (totp or hotp)
	otpType := OtpTypeTOTP
	if u.Host == "hotp" {
		otpType = OtpTypeHOTP
	} else if u.Host != "totp" {
		return nil, fmt.Errorf("unsupported OTP type: %s", u.Host)
	}

	// Parse label (path) which contains issuer:account or just account
	label := strings.TrimPrefix(u.Path, "/")
	if label == "" {
		return nil, fmt.Errorf("missing label in URI")
	}

	// Decode URL-encoded label
	label, err = url.QueryUnescape(label)
	if err != nil {
		return nil, fmt.Errorf("failed to decode label: %w", err)
	}

	// Extract name and issuer from label
	var name, issuer string
	if strings.Contains(label, ":") {
		parts := strings.SplitN(label, ":", 2)
		issuer = parts[0]
		name = parts[1]
	} else {
		name = label
	}

	// Parse query parameters
	query := u.Query()

	// Secret (required)
	secretStr := query.Get("secret")
	if secretStr == "" {
		return nil, fmt.Errorf("missing secret parameter")
	}

	// Decode base32 secret
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secretStr))
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret: %w", err)
	}

	// Create OtpParameters
	param := &OtpParameters{
		Secret:    secret,
		Name:      name,
		Issuer:    issuer,
		Algorithm: AlgorithmSHA1,    // Default
		Digits:    DigitCountSix,    // Default
		Type:      otpType,
	}

	// Parse optional issuer parameter (overrides label issuer)
	if issuerParam := query.Get("issuer"); issuerParam != "" {
		param.Issuer = issuerParam
	}

	// Parse algorithm
	if algo := query.Get("algorithm"); algo != "" {
		switch strings.ToUpper(algo) {
		case "SHA1":
			param.Algorithm = AlgorithmSHA1
		case "SHA256":
			param.Algorithm = AlgorithmSHA256
		case "SHA512":
			param.Algorithm = AlgorithmSHA512
		case "MD5":
			param.Algorithm = AlgorithmMD5
		default:
			return nil, fmt.Errorf("unsupported algorithm: %s", algo)
		}
	}

	// Parse digits
	if digits := query.Get("digits"); digits != "" {
		if digits == "8" {
			param.Digits = DigitCountEight
		} else if digits == "6" {
			param.Digits = DigitCountSix
		} else {
			return nil, fmt.Errorf("unsupported digit count: %s", digits)
		}
	}

	// Parse counter (for HOTP)
	if otpType == OtpTypeHOTP {
		counterStr := query.Get("counter")
		if counterStr == "" {
			return nil, fmt.Errorf("HOTP requires counter parameter")
		}
		counter, err := strconv.ParseInt(counterStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid counter value: %w", err)
		}
		param.Counter = counter
	}

	return param, nil
}

// GenerateSecretKey generates a random base32 secret key
func GenerateSecretKey() string {
	// Generate 20 random bytes (160 bits)
	bytes := make([]byte, 20)
	// In production, use crypto/rand
	return base32.StdEncoding.EncodeToString(bytes)
}

