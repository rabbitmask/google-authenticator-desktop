package storage

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const (
	dbFileName       = "authenticator.db"
	verifierConstant = "AUTHENTICATOR_KEY_VERIFIER_V1"
)

// Database 封装数据库操作
type Database struct {
	db        *sql.DB
	masterKey []byte
	mu        sync.RWMutex
	dbPath    string
}

// Account 账户结构
type Account struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Issuer    string `json:"issuer"`
	Secret    string `json:"secret"`
	Algorithm string `json:"algorithm"`
	Digits    int    `json:"digits"`
	Type      string `json:"type"`
	Counter   int64  `json:"counter"`
	Period    int    `json:"period"`
	Group     string `json:"group"`
}

// Settings 设置结构
type Settings struct {
	PasswordEnabled bool   `json:"password_enabled"`
	Theme           string `json:"theme"`
	AutoLockMinutes int    `json:"auto_lock_minutes"`
}

// DefaultSettings 默认设置
func DefaultSettings() Settings {
	return Settings{
		PasswordEnabled: false,
		Theme:           "light",
		AutoLockMinutes: 5,
	}
}

// encodeBytes 将字节编码为 Base64 字符串
func encodeBytes(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// decodeBytes 从 Base64 字符串解码字节
func decodeBytes(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

// NewDatabase 创建数据库实例
func NewDatabase() (*Database, error) {
	// 获取可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)

	// 创建 data 目录
	dataDir := filepath.Join(execDir, "data")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, dbFileName)

	// 打开数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 初始化表结构
	if err := initTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Database{
		db:     db,
		dbPath: dbPath,
	}, nil
}

// initTables 初始化数据库表
func initTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS metadata (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		data TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// IsInitialized 检查数据库是否已初始化（设置了主密钥）
func (d *Database) IsInitialized() bool {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM metadata WHERE key = 'salt'").Scan(&count)
	return err == nil && count > 0
}

// HasPassword 检查是否设置了密码
func (d *Database) HasPassword() bool {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM metadata WHERE key = 'password_verifier'").Scan(&count)
	return err == nil && count > 0
}

// Initialize 初始化数据库（首次使用，无密码）
func (d *Database) Initialize() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 生成盐值
	salt, err := GenerateSalt()
	if err != nil {
		return err
	}

	// 使用设备密钥
	d.masterKey = GetDeviceKey()

	// 保存盐值（Base64 编码）
	_, err = d.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('salt', ?)",
		encodeBytes(salt))
	if err != nil {
		return fmt.Errorf("failed to save salt: %w", err)
	}

	// 创建验证器（用于验证密钥正确性）
	verifier, err := Encrypt([]byte(verifierConstant), d.masterKey)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('device_verifier', ?)",
		encodeBytes(verifier))
	if err != nil {
		return fmt.Errorf("failed to save verifier: %w", err)
	}

	// 保存默认设置
	return d.saveSettingsInternal(DefaultSettings())
}

// SetPassword 设置密码保护
func (d *Database) SetPassword(password string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 获取当前数据
	accounts, err := d.getAllAccountsInternal()
	if err != nil {
		return err
	}

	settings, err := d.getSettingsInternal()
	if err != nil {
		settings = DefaultSettings()
	}

	// 生成新盐值
	salt, err := GenerateSalt()
	if err != nil {
		return err
	}

	// 派生新主密钥
	newMasterKey := DeriveKey(password, salt)

	// 保存新盐值（Base64 编码）
	_, err = d.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('salt', ?)",
		encodeBytes(salt))
	if err != nil {
		return fmt.Errorf("failed to save salt: %w", err)
	}

	// 创建密码验证器
	verifier, err := Encrypt([]byte(verifierConstant), newMasterKey)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('password_verifier', ?)",
		encodeBytes(verifier))
	if err != nil {
		return fmt.Errorf("failed to save verifier: %w", err)
	}

	// 删除设备验证器
	d.db.Exec("DELETE FROM metadata WHERE key = 'device_verifier'")

	// 更新主密钥
	d.masterKey = newMasterKey

	// 重新加密所有数据
	settings.PasswordEnabled = true
	if err := d.saveSettingsInternal(settings); err != nil {
		return err
	}

	// 清空并重新保存账户
	d.db.Exec("DELETE FROM accounts")
	for _, acc := range accounts {
		if err := d.saveAccountInternal(acc); err != nil {
			return err
		}
	}

	return nil
}

// RemovePassword 移除密码保护
func (d *Database) RemovePassword() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 获取当前数据
	accounts, err := d.getAllAccountsInternal()
	if err != nil {
		return err
	}

	settings, err := d.getSettingsInternal()
	if err != nil {
		settings = DefaultSettings()
	}

	// 使用设备密钥
	newMasterKey := GetDeviceKey()

	// 生成新盐值
	salt, err := GenerateSalt()
	if err != nil {
		return err
	}

	// 保存盐值（Base64 编码）
	_, err = d.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('salt', ?)",
		encodeBytes(salt))
	if err != nil {
		return fmt.Errorf("failed to save salt: %w", err)
	}

	// 创建设备验证器
	verifier, err := Encrypt([]byte(verifierConstant), newMasterKey)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('device_verifier', ?)",
		encodeBytes(verifier))
	if err != nil {
		return fmt.Errorf("failed to save verifier: %w", err)
	}

	// 删除密码验证器
	d.db.Exec("DELETE FROM metadata WHERE key = 'password_verifier'")

	// 更新主密钥
	d.masterKey = newMasterKey

	// 重新加密所有数据
	settings.PasswordEnabled = false
	if err := d.saveSettingsInternal(settings); err != nil {
		return err
	}

	// 清空并重新保存账户
	d.db.Exec("DELETE FROM accounts")
	for _, acc := range accounts {
		if err := d.saveAccountInternal(acc); err != nil {
			return err
		}
	}

	return nil
}

// Unlock 使用密码解锁数据库
func (d *Database) Unlock(password string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 获取盐值
	var saltEncoded string
	err := d.db.QueryRow("SELECT value FROM metadata WHERE key = 'salt'").Scan(&saltEncoded)
	if err != nil {
		return fmt.Errorf("database not initialized")
	}

	salt, err := decodeBytes(saltEncoded)
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}

	// 派生密钥
	key := DeriveKey(password, salt)

	// 获取验证器
	var verifierEncoded string
	err = d.db.QueryRow("SELECT value FROM metadata WHERE key = 'password_verifier'").Scan(&verifierEncoded)
	if err != nil {
		return fmt.Errorf("no password set")
	}

	verifier, err := decodeBytes(verifierEncoded)
	if err != nil {
		return fmt.Errorf("failed to decode verifier: %w", err)
	}

	// 验证密钥
	if !VerifyKey(verifier, key) {
		return fmt.Errorf("invalid password")
	}

	d.masterKey = key
	return nil
}

// UnlockWithDeviceKey 使用设备密钥解锁（无密码时）
func (d *Database) UnlockWithDeviceKey() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := GetDeviceKey()

	// 获取设备验证器
	var verifierEncoded string
	err := d.db.QueryRow("SELECT value FROM metadata WHERE key = 'device_verifier'").Scan(&verifierEncoded)
	if err != nil {
		return fmt.Errorf("device verifier not found")
	}

	verifier, err := decodeBytes(verifierEncoded)
	if err != nil {
		return fmt.Errorf("failed to decode verifier: %w", err)
	}

	// 验证密钥
	if !VerifyKey(verifier, key) {
		return fmt.Errorf("device key verification failed")
	}

	d.masterKey = key
	return nil
}

// ChangePassword 修改密码
func (d *Database) ChangePassword(newPassword string) error {
	return d.SetPassword(newPassword)
}

// VerifyPassword 验证密码
func (d *Database) VerifyPassword(password string) bool {
	// 获取盐值
	var saltEncoded string
	err := d.db.QueryRow("SELECT value FROM metadata WHERE key = 'salt'").Scan(&saltEncoded)
	if err != nil {
		return false
	}

	salt, err := decodeBytes(saltEncoded)
	if err != nil {
		return false
	}

	// 派生密钥
	key := DeriveKey(password, salt)

	// 获取验证器
	var verifierEncoded string
	err = d.db.QueryRow("SELECT value FROM metadata WHERE key = 'password_verifier'").Scan(&verifierEncoded)
	if err != nil {
		return false
	}

	verifier, err := decodeBytes(verifierEncoded)
	if err != nil {
		return false
	}

	return VerifyKey(verifier, key)
}

// IsUnlocked 检查数据库是否已解锁
func (d *Database) IsUnlocked() bool {
	return len(d.masterKey) == 32
}

// ensureUnlocked 确保数据库已解锁（无密码时自动解锁）
func (d *Database) ensureUnlocked() error {
	if d.IsUnlocked() {
		return nil
	}

	// 如果没有密码保护，自动使用设备密钥解锁
	if !d.HasPassword() {
		return d.unlockWithDeviceKeyInternal()
	}

	return fmt.Errorf("database is locked, password required")
}

// unlockWithDeviceKeyInternal 内部方法，不加锁
func (d *Database) unlockWithDeviceKeyInternal() error {
	key := GetDeviceKey()

	// 获取设备验证器
	var verifierEncoded string
	err := d.db.QueryRow("SELECT value FROM metadata WHERE key = 'device_verifier'").Scan(&verifierEncoded)
	if err != nil {
		// 如果没有验证器，说明是旧数据或损坏，重新初始化
		return d.reinitializeWithDeviceKey()
	}

	verifier, err := decodeBytes(verifierEncoded)
	if err != nil {
		return d.reinitializeWithDeviceKey()
	}

	// 验证密钥
	if !VerifyKey(verifier, key) {
		return d.reinitializeWithDeviceKey()
	}

	d.masterKey = key
	return nil
}

// reinitializeWithDeviceKey 重新初始化设备密钥（数据可能丢失）
func (d *Database) reinitializeWithDeviceKey() error {
	// 生成盐值
	salt, err := GenerateSalt()
	if err != nil {
		return err
	}

	// 使用设备密钥
	d.masterKey = GetDeviceKey()

	// 保存盐值
	_, err = d.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('salt', ?)",
		encodeBytes(salt))
	if err != nil {
		return fmt.Errorf("failed to save salt: %w", err)
	}

	// 创建新的设备验证器
	verifier, err := Encrypt([]byte(verifierConstant), d.masterKey)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('device_verifier', ?)",
		encodeBytes(verifier))
	if err != nil {
		return fmt.Errorf("failed to save verifier: %w", err)
	}

	return nil
}

// === 账户操作 ===

func (d *Database) saveAccountInternal(acc Account) error {
	data, err := json.Marshal(acc)
	if err != nil {
		return fmt.Errorf("failed to marshal account: %w", err)
	}

	encrypted, err := Encrypt(data, d.masterKey)
	if err != nil {
		return err
	}

	// 使用 Base64 编码存储
	_, err = d.db.Exec("INSERT OR REPLACE INTO accounts (id, data) VALUES (?, ?)",
		acc.ID, encodeBytes(encrypted))
	return err
}

// SaveAccount 保存账户
func (d *Database) SaveAccount(acc Account) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 确保已解锁
	if err := d.ensureUnlocked(); err != nil {
		return err
	}

	return d.saveAccountInternal(acc)
}

// GetAccount 获取单个账户
func (d *Database) GetAccount(id string) (*Account, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 确保已解锁
	if err := d.ensureUnlocked(); err != nil {
		return nil, err
	}

	var encryptedEncoded string
	err := d.db.QueryRow("SELECT data FROM accounts WHERE id = ?", id).Scan(&encryptedEncoded)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	encryptedData, err := decodeBytes(encryptedEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode account data: %w", err)
	}

	decrypted, err := Decrypt(encryptedData, d.masterKey)
	if err != nil {
		return nil, err
	}

	var acc Account
	if err := json.Unmarshal(decrypted, &acc); err != nil {
		return nil, err
	}

	return &acc, nil
}

func (d *Database) getAllAccountsInternal() ([]Account, error) {
	rows, err := d.db.Query("SELECT data FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var encryptedEncoded string
		if err := rows.Scan(&encryptedEncoded); err != nil {
			continue
		}

		encryptedData, err := decodeBytes(encryptedEncoded)
		if err != nil {
			continue
		}

		decrypted, err := Decrypt(encryptedData, d.masterKey)
		if err != nil {
			continue
		}

		var acc Account
		if err := json.Unmarshal(decrypted, &acc); err != nil {
			continue
		}
		accounts = append(accounts, acc)
	}

	return accounts, nil
}

// GetAllAccounts 获取所有账户
func (d *Database) GetAllAccounts() ([]Account, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 确保已解锁
	if err := d.ensureUnlocked(); err != nil {
		return nil, err
	}

	return d.getAllAccountsInternal()
}

// DeleteAccount 删除账户
func (d *Database) DeleteAccount(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM accounts WHERE id = ?", id)
	return err
}

// DeleteAllAccounts 删除所有账户
func (d *Database) DeleteAllAccounts() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM accounts")
	return err
}

// === 设置操作 ===

func (d *Database) saveSettingsInternal(s Settings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	encrypted, err := Encrypt(data, d.masterKey)
	if err != nil {
		return err
	}

	// 使用 Base64 编码存储
	_, err = d.db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES ('main', ?)",
		encodeBytes(encrypted))
	return err
}

// SaveSettings 保存设置
func (d *Database) SaveSettings(s Settings) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 确保已解锁
	if err := d.ensureUnlocked(); err != nil {
		return err
	}

	return d.saveSettingsInternal(s)
}

func (d *Database) getSettingsInternal() (Settings, error) {
	var encryptedEncoded string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = 'main'").Scan(&encryptedEncoded)
	if err != nil {
		return DefaultSettings(), err
	}

	encryptedData, err := decodeBytes(encryptedEncoded)
	if err != nil {
		return DefaultSettings(), fmt.Errorf("failed to decode settings: %w", err)
	}

	decrypted, err := Decrypt(encryptedData, d.masterKey)
	if err != nil {
		return DefaultSettings(), err
	}

	var s Settings
	if err := json.Unmarshal(decrypted, &s); err != nil {
		return DefaultSettings(), err
	}

	return s, nil
}

// GetSettings 获取设置
func (d *Database) GetSettings() (Settings, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 确保已解锁
	if err := d.ensureUnlocked(); err != nil {
		return DefaultSettings(), err
	}

	return d.getSettingsInternal()
}

// GetDBPath 获取数据库路径
func (d *Database) GetDBPath() string {
	return d.dbPath
}

// GetStatus 获取数据库状态（用于调试）
func (d *Database) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"initialized":   d.IsInitialized(),
		"has_password":  d.HasPassword(),
		"unlocked":      d.IsUnlocked(),
		"master_key_len": len(d.masterKey),
	}
}

// NeedsUnlock 检查是否需要解锁
func (d *Database) NeedsUnlock() bool {
	return d.HasPassword() && !d.IsUnlocked()
}
