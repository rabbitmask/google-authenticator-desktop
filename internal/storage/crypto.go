package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id 参数
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
	argonKeyLen  = 32 // AES-256

	// 盐值长度
	saltLen = 16

	// Nonce 长度 (AES-GCM 标准)
	nonceLen = 12
)

var (
	ErrDecryptionFailed = errors.New("decryption failed: invalid key or corrupted data")
	ErrInvalidData      = errors.New("invalid encrypted data format")
)

// DeriveKey 使用 Argon2id 从密码派生密钥
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
}

// GenerateSalt 生成随机盐值
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// Encrypt 使用 AES-256-GCM 加密数据
// 返回格式: nonce(12) + ciphertext + tag(16)
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal appends the encrypted data to nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 使用 AES-256-GCM 解密数据
func Decrypt(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) < nonceLen+16 { // nonce + minimum tag size
		return nil, ErrInvalidData
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := ciphertext[:nonceLen]
	ciphertext = ciphertext[nonceLen:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptToBase64 加密并返回 Base64 编码的字符串
func EncryptToBase64(plaintext, key []byte) (string, error) {
	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptFromBase64 从 Base64 字符串解密
func DecryptFromBase64(encoded string, key []byte) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	return Decrypt(ciphertext, key)
}

// GetDeviceKey 获取设备唯一标识派生的默认密钥
// 用于未设置密码时的基础加密
func GetDeviceKey() []byte {
	// 组合多个设备特征
	var deviceID string

	// 获取主机名
	hostname, _ := os.Hostname()
	deviceID += hostname

	// 获取用户目录
	homeDir, _ := os.UserHomeDir()
	deviceID += homeDir

	// 获取操作系统信息
	deviceID += runtime.GOOS + runtime.GOARCH

	// 使用 SHA256 生成固定长度的密钥
	hash := sha256.Sum256([]byte(deviceID))
	return hash[:]
}

// VerifyKey 验证密钥是否正确（通过尝试解密验证数据）
func VerifyKey(encryptedVerifier []byte, key []byte) bool {
	_, err := Decrypt(encryptedVerifier, key)
	return err == nil
}
