package migration

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// Simple protobuf wire format parser for Google Authenticator migration
// Based on: https://github.com/dim13/otpauth

const (
	wireVarint    = 0
	wireLen       = 2
)

type decoder struct {
	data []byte
	pos  int
}

func (d *decoder) varint() (uint64, error) {
	var x uint64
	var s uint
	for i := 0; ; i++ {
		if d.pos >= len(d.data) {
			return 0, io.ErrUnexpectedEOF
		}
		b := d.data[d.pos]
		d.pos++
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return 0, fmt.Errorf("varint overflow")
			}
			return x | uint64(b)<<s, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}

func (d *decoder) bytes() ([]byte, error) {
	n, err := d.varint()
	if err != nil {
		return nil, err
	}
	if d.pos+int(n) > len(d.data) {
		return nil, io.ErrUnexpectedEOF
	}
	b := d.data[d.pos : d.pos+int(n)]
	d.pos += int(n)
	return b, nil
}

func (d *decoder) string() (string, error) {
	b, err := d.bytes()
	return string(b), err
}

// ParseMigrationURISimple parses otpauth-migration:// URI
func ParseMigrationURISimple(uri string) ([]*OtpParameters, error) {
	if !strings.HasPrefix(uri, "otpauth-migration://") {
		return nil, fmt.Errorf("invalid URI scheme")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	dataParam := u.Query().Get("data")
	if dataParam == "" {
		return nil, fmt.Errorf("missing 'data' parameter")
	}

	protoData, err := base64.StdEncoding.DecodeString(dataParam)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error: %w", err)
	}

	return parsePayload(protoData)
}

func parsePayload(data []byte) ([]*OtpParameters, error) {
	d := &decoder{data: data}
	var accounts []*OtpParameters

	for d.pos < len(d.data) {
		tag, err := d.varint()
		if err != nil {
			break
		}

		wire := tag & 7
		field := tag >> 3

		switch field {
		case 1: // otp_parameters (repeated)
			if wire != wireLen {
				return nil, fmt.Errorf("invalid wire type for otp_parameters")
			}
			paramBytes, err := d.bytes()
			if err != nil {
				return nil, err
			}
			param, err := parseOtpParameters(paramBytes)
			if err != nil {
				return nil, err
			}
			accounts = append(accounts, param)

		case 2, 3, 4, 5: // version, batch_size, batch_index, batch_id
			if wire == wireVarint {
				_, _ = d.varint() // Skip these fields
			}
		default:
			return nil, fmt.Errorf("unknown field: %d", field)
		}
	}

	return accounts, nil
}

func parseOtpParameters(data []byte) (*OtpParameters, error) {
	d := &decoder{data: data}
	param := &OtpParameters{
		Algorithm: AlgorithmSHA1,    // Default
		Digits:    DigitCountSix,    // Default
		Type:      OtpTypeTOTP,      // Default
	}

	for d.pos < len(d.data) {
		tag, err := d.varint()
		if err != nil {
			break
		}

		wire := tag & 7
		field := tag >> 3

		switch field {
		case 1: // secret
			param.Secret, err = d.bytes()
		case 2: // name
			param.Name, err = d.string()
		case 3: // issuer
			param.Issuer, err = d.string()
		case 4: // algorithm
			val, err2 := d.varint()
			if err2 == nil {
				param.Algorithm = Algorithm(val)
			}
			err = err2
		case 5: // digits
			val, err2 := d.varint()
			if err2 == nil {
				param.Digits = DigitCount(val)
			}
			err = err2
		case 6: // type
			val, err2 := d.varint()
			if err2 == nil {
				param.Type = OtpType(val)
			}
			err = err2
		case 7: // counter
			val, err2 := d.varint()
			if err2 == nil {
				param.Counter = int64(val)
			}
			err = err2
		default:
			// Skip unknown fields
			if wire == wireVarint {
				_, err = d.varint()
			} else if wire == wireLen {
				_, err = d.bytes()
			}
		}

		if err != nil {
			return nil, err
		}
	}

	return param, nil
}

// GenerateMigrationURISimple generates otpauth-migration:// URI
func GenerateMigrationURISimple(accounts []*OtpParameters) (string, error) {
	// Encode payload
	payloadData := encodePayload(accounts)

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(payloadData)

	// URL encode the base64 string (important!)
	encodedURL := url.QueryEscape(encoded)

	return fmt.Sprintf("otpauth-migration://offline?data=%s", encodedURL), nil
}

func encodePayload(accounts []*OtpParameters) []byte {
	var buf []byte

	for _, acc := range accounts {
		paramData := encodeOtpParameters(acc)
		// Field 1, wire type 2 (length-delimited)
		buf = appendTag(buf, 1, wireLen)
		buf = appendBytes(buf, paramData)
	}

	// version = 1 (field 2)
	buf = appendTag(buf, 2, wireVarint)
	buf = appendVarint(buf, 1)

	// batch_size (field 3)
	buf = appendTag(buf, 3, wireVarint)
	buf = appendVarint(buf, uint64(len(accounts)))

	// batch_index = 0 (field 4) - 从0开始
	buf = appendTag(buf, 4, wireVarint)
	buf = appendVarint(buf, 0)

	// batch_id (field 5) - 使用时间戳作为唯一ID
	buf = appendTag(buf, 5, wireVarint)
	buf = appendVarint(buf, uint64(time.Now().UnixNano()%1000000))

	return buf
}

func encodeOtpParameters(param *OtpParameters) []byte {
	var buf []byte

	// secret (field 1)
	if len(param.Secret) > 0 {
		buf = appendTag(buf, 1, wireLen)
		buf = appendBytes(buf, param.Secret)
	}

	// name (field 2)
	if param.Name != "" {
		buf = appendTag(buf, 2, wireLen)
		buf = appendString(buf, param.Name)
	}

	// issuer (field 3)
	if param.Issuer != "" {
		buf = appendTag(buf, 3, wireLen)
		buf = appendString(buf, param.Issuer)
	}

	// algorithm (field 4)
	buf = appendTag(buf, 4, wireVarint)
	buf = appendVarint(buf, uint64(param.Algorithm))

	// digits (field 5)
	buf = appendTag(buf, 5, wireVarint)
	buf = appendVarint(buf, uint64(param.Digits))

	// type (field 6)
	buf = appendTag(buf, 6, wireVarint)
	buf = appendVarint(buf, uint64(param.Type))

	// counter (field 7) - for HOTP
	if param.Type == OtpTypeHOTP {
		buf = appendTag(buf, 7, wireVarint)
		buf = appendVarint(buf, uint64(param.Counter))
	}

	return buf
}

func appendTag(buf []byte, field, wire uint64) []byte {
	return appendVarint(buf, field<<3|wire)
}

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func appendBytes(buf []byte, b []byte) []byte {
	buf = appendVarint(buf, uint64(len(b)))
	return append(buf, b...)
}

func appendString(buf []byte, s string) []byte {
	return appendBytes(buf, []byte(s))
}
