package main

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"google-authenticator/internal/migration"
	"google-authenticator/internal/otp"
	"google-authenticator/internal/qrcode"
	"google-authenticator/internal/storage"
	"google-authenticator/internal/tray"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
	db  *storage.Database
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 初始化数据库
	db, err := storage.NewDatabase()
	if err != nil {
		runtime.LogError(ctx, fmt.Sprintf("Failed to initialize database: %v", err))
		return
	}
	a.db = db

	// 检查是否已初始化
	if !db.IsInitialized() {
		// 首次使用，初始化数据库
		if err := db.Initialize(); err != nil {
			runtime.LogError(ctx, fmt.Sprintf("Failed to initialize database: %v", err))
			return
		}
	} else if !db.HasPassword() {
		// 无密码保护，使用设备密钥解锁
		if err := db.UnlockWithDeviceKey(); err != nil {
			runtime.LogError(ctx, fmt.Sprintf("Failed to unlock database: %v", err))
		}
	}
	// 如果有密码保护，等待前端调用 Unlock

	// 启动系统托盘
	go tray.Init(a.ShowWindow, func() {
		a.CloseDB()
		releaseLock()
		os.Exit(0)
	})
}

// beforeClose 关闭时始终最小化到托盘
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	runtime.Hide(ctx)
	return true // 始终阻止关闭，改为隐藏到托盘
}

// shutdown 应用退出时调用
func (a *App) shutdown(_ context.Context) {
	// 关闭数据库
	if a.db != nil {
		err := a.db.Close()
		if err != nil {
			return
		}
	}
	// 退出系统托盘
	tray.Quit()
}

// ShowWindow 显示窗口
func (a *App) ShowWindow() {
	runtime.Show(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
}

// HideWindow 隐藏窗口到托盘
func (a *App) HideWindow() {
	runtime.Hide(a.ctx)
}

// QuitApp 完全退出应用
func (a *App) QuitApp() {
	runtime.Quit(a.ctx)
}

// CloseDB 关闭数据库连接
func (a *App) CloseDB() {
	if a.db != nil {
		err := a.db.Close()
		if err != nil {
			return
		}
	}
}

// GetDatabaseStatus 获取数据库状态（用于调试）
func (a *App) GetDatabaseStatus() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"error": "database not initialized",
		}
	}
	return a.db.GetStatus()
}

// NeedsUnlock 检查是否需要解锁
func (a *App) NeedsUnlock() bool {
	if a.db == nil {
		return false
	}
	return a.db.NeedsUnlock()
}

// === 密码管理 ===

// IsPasswordEnabled 检查是否启用密码
func (a *App) IsPasswordEnabled() bool {
	if a.db == nil {
		return false
	}
	return a.db.HasPassword()
}

// EnablePassword 启用密码保护
func (a *App) EnablePassword(password string) bool {
	if a.db == nil || password == "" {
		return false
	}
	return a.db.SetPassword(password) == nil
}

// DisablePassword 禁用密码保护（需要验证当前密码）
func (a *App) DisablePassword(currentPassword string) bool {
	if a.db == nil {
		return false
	}
	if !a.db.VerifyPassword(currentPassword) {
		return false
	}
	return a.db.RemovePassword() == nil
}

// ChangePassword 修改密码（需要验证当前密码）
func (a *App) ChangePassword(currentPassword, newPassword string) bool {
	if a.db == nil || newPassword == "" {
		return false
	}
	if !a.db.VerifyPassword(currentPassword) {
		return false
	}
	return a.db.ChangePassword(newPassword) == nil
}

// VerifyPassword 验证密码
func (a *App) VerifyPassword(password string) bool {
	if a.db == nil {
		return false
	}
	return a.db.VerifyPassword(password)
}

// Unlock 解锁数据库
func (a *App) Unlock(password string) bool {
	if a.db == nil {
		return false
	}
	return a.db.Unlock(password) == nil
}

// === 设置管理 ===

// GetSettings 获取设置
func (a *App) GetSettings() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"password_enabled":  false,
			"theme":             "light",
			"auto_lock_minutes": 5,
		}
	}

	settings, err := a.db.GetSettings()
	if err != nil {
		settings = storage.DefaultSettings()
	}

	return map[string]interface{}{
		"password_enabled":  a.db.HasPassword(),
		"theme":             settings.Theme,
		"auto_lock_minutes": settings.AutoLockMinutes,
	}
}

// SetTheme 设置主题
func (a *App) SetTheme(theme string) bool {
	if a.db == nil {
		return false
	}

	settings, _ := a.db.GetSettings()
	settings.Theme = theme
	return a.db.SaveSettings(settings) == nil
}

// SetAutoLockMinutes 设置自动锁定时间
func (a *App) SetAutoLockMinutes(minutes int) bool {
	if a.db == nil {
		return false
	}
	if minutes < 0 {
		minutes = 0
	}

	settings, _ := a.db.GetSettings()
	settings.AutoLockMinutes = minutes
	return a.db.SaveSettings(settings) == nil
}

// GetAutoLockMinutes 获取自动锁定时间
func (a *App) GetAutoLockMinutes() int {
	if a.db == nil {
		return 5
	}
	settings, _ := a.db.GetSettings()
	return settings.AutoLockMinutes
}

// === 账户操作相关结构 ===

// ImportResult represents the result of an import operation
type ImportResult struct {
	Success  bool          `json:"success"`
	Message  string        `json:"message"`
	Count    int           `json:"count"`
	Accounts []otp.Account `json:"accounts"`
}

// GenerateCodeResult represents a generated OTP code
type GenerateCodeResult struct {
	Code      string `json:"code"`
	Remaining int    `json:"remaining"`
	Progress  int    `json:"progress"`
}

// ExportQRResult represents an exported QR code
type ExportQRResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	QRCodeURL string `json:"qr_code_url"` // Base64 data URL
}

// === 账户操作 ===

// storageAccountToOTP 将 storage.Account 转换为 otp.Account
func storageAccountToOTP(acc storage.Account) otp.Account {
	return otp.Account{
		ID:        acc.ID,
		Name:      acc.Name,
		Issuer:    acc.Issuer,
		Secret:    acc.Secret,
		Algorithm: acc.Algorithm,
		Digits:    acc.Digits,
		Type:      acc.Type,
		Counter:   acc.Counter,
		Period:    acc.Period,
		Group:     acc.Group,
	}
}

// otpAccountToStorage 将 otp.Account 转换为 storage.Account
func otpAccountToStorage(acc otp.Account) storage.Account {
	return storage.Account{
		ID:        acc.ID,
		Name:      acc.Name,
		Issuer:    acc.Issuer,
		Secret:    acc.Secret,
		Algorithm: acc.Algorithm,
		Digits:    acc.Digits,
		Type:      acc.Type,
		Counter:   acc.Counter,
		Period:    acc.Period,
		Group:     acc.Group,
	}
}

// GetAllAccounts returns all accounts
func (a *App) GetAllAccounts() []otp.Account {
	if a.db == nil {
		return []otp.Account{}
	}

	storageAccounts, err := a.db.GetAllAccounts()
	if err != nil {
		return []otp.Account{}
	}

	accounts := make([]otp.Account, len(storageAccounts))
	for i, acc := range storageAccounts {
		accounts[i] = storageAccountToOTP(acc)
	}
	return accounts
}

// ImportFromMigrationURI imports accounts from Google Authenticator migration URI
func (a *App) ImportFromMigrationURI(uri string) ImportResult {
	if a.db == nil {
		return ImportResult{Success: false, Message: "数据库未初始化"}
	}

	params, err := migration.ParseMigrationURISimple(uri)
	if err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("解析失败: %v", err),
		}
	}

	accounts := make([]otp.Account, 0)
	for _, p := range params {
		acc := otp.Account{
			ID:        uuid.New().String(),
			Name:      p.Name,
			Issuer:    p.Issuer,
			Secret:    base32.StdEncoding.EncodeToString(p.Secret),
			Algorithm: p.Algorithm.String(),
			Digits:    p.Digits.ToInt(),
			Type:      p.Type.String(),
			Counter:   p.Counter,
			Period:    30,
		}

		// 保存到数据库
		if err := a.db.SaveAccount(otpAccountToStorage(acc)); err != nil {
			continue
		}
		accounts = append(accounts, acc)
	}

	return ImportResult{
		Success:  true,
		Message:  fmt.Sprintf("成功导入 %d 个账户", len(accounts)),
		Count:    len(accounts),
		Accounts: accounts,
	}
}

// ImportFromStandardURI imports a single account from standard otpauth:// URI
func (a *App) ImportFromStandardURI(uri string) ImportResult {
	if a.db == nil {
		return ImportResult{Success: false, Message: "数据库未初始化"}
	}

	param, err := migration.ParseOTPAuthURI(uri)
	if err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("解析失败: %v", err),
		}
	}

	acc := otp.Account{
		ID:        uuid.New().String(),
		Name:      param.Name,
		Issuer:    param.Issuer,
		Secret:    base32.StdEncoding.EncodeToString(param.Secret),
		Algorithm: param.Algorithm.String(),
		Digits:    param.Digits.ToInt(),
		Type:      param.Type.String(),
		Counter:   param.Counter,
		Period:    30,
	}

	// 保存到数据库
	if err := a.db.SaveAccount(otpAccountToStorage(acc)); err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("保存失败: %v", err),
		}
	}

	return ImportResult{
		Success:  true,
		Message:  fmt.Sprintf("成功添加账户: %s", acc.Name),
		Count:    1,
		Accounts: []otp.Account{acc},
	}
}

// ImportFromQRCodeImage imports accounts from QR code image (base64 encoded)
func (a *App) ImportFromQRCodeImage(base64Image string) ImportResult {
	if a.db == nil {
		return ImportResult{Success: false, Message: "数据库未初始化"}
	}

	// Remove data URL prefix if present
	if strings.HasPrefix(base64Image, "data:image") {
		parts := strings.Split(base64Image, ",")
		if len(parts) == 2 {
			base64Image = parts[1]
		}
	}

	// Decode base64
	imgData, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("图片解码失败: %v", err),
		}
	}

	// Scan QR code
	uri, err := qrcode.ScanQRCodeFromBytes(imgData)
	if err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("QR码识别失败: %v", err),
		}
	}

	// Check URI type and import accordingly
	if strings.HasPrefix(uri, "otpauth-migration://") {
		return a.ImportFromMigrationURI(uri)
	} else if strings.HasPrefix(uri, "otpauth://") {
		return a.ImportFromStandardURI(uri)
	} else {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("不支持的URI格式: %s", uri),
		}
	}
}

// ImportFromFile opens a file dialog and imports QR code from selected image file
func (a *App) ImportFromFile() ImportResult {
	if a.db == nil {
		return ImportResult{Success: false, Message: "数据库未初始化"}
	}

	// Open file dialog
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择二维码图片",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "图片文件 (*.png;*.jpg;*.jpeg)",
				Pattern:     "*.png;*.jpg;*.jpeg",
			},
		},
	})

	if err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("打开文件失败: %v", err),
		}
	}

	if file == "" {
		return ImportResult{
			Success: false,
			Message: "未选择文件",
		}
	}

	// Read file
	imgData, err := readFile(file)
	if err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("读取文件失败: %v", err),
		}
	}

	// Scan QR code
	uri, err := qrcode.ScanQRCodeFromBytes(imgData)
	if err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("QR码识别失败: %v", err),
		}
	}

	// Check URI type and import accordingly
	if strings.HasPrefix(uri, "otpauth-migration://") {
		return a.ImportFromMigrationURI(uri)
	} else if strings.HasPrefix(uri, "otpauth://") {
		return a.ImportFromStandardURI(uri)
	} else {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("不支持的URI格式: %s", uri),
		}
	}
}

// readFile 读取文件内容
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// AddAccount adds a new account manually
func (a *App) AddAccount(name, issuer, secret, algorithm, otpType string, digits, period int) ImportResult {
	return a.AddAccountWithGroup(name, issuer, secret, algorithm, otpType, digits, period, "")
}

// AddAccountWithGroup adds a new account with group
func (a *App) AddAccountWithGroup(name, issuer, secret, algorithm, otpType string, digits, period int, group string) ImportResult {
	if a.db == nil {
		return ImportResult{Success: false, Message: "数据库未初始化"}
	}

	acc := otp.Account{
		ID:        uuid.New().String(),
		Name:      name,
		Issuer:    issuer,
		Secret:    secret,
		Algorithm: algorithm,
		Digits:    digits,
		Type:      otpType,
		Period:    period,
		Counter:   0,
		Group:     group,
	}

	// 保存到数据库
	if err := a.db.SaveAccount(otpAccountToStorage(acc)); err != nil {
		return ImportResult{
			Success: false,
			Message: fmt.Sprintf("保存失败: %v", err),
		}
	}

	return ImportResult{
		Success:  true,
		Message:  "账户添加成功",
		Count:    1,
		Accounts: []otp.Account{acc},
	}
}

// DeleteAccount deletes an account
func (a *App) DeleteAccount(accountID string) bool {
	if a.db == nil {
		return false
	}
	return a.db.DeleteAccount(accountID) == nil
}

// DeleteAccounts deletes multiple accounts
func (a *App) DeleteAccounts(accountIDs []string) int {
	if a.db == nil {
		return 0
	}

	count := 0
	for _, id := range accountIDs {
		if a.db.DeleteAccount(id) == nil {
			count++
		}
	}
	return count
}

// DeleteAllAccounts deletes all accounts
func (a *App) DeleteAllAccounts() bool {
	if a.db == nil {
		return false
	}
	return a.db.DeleteAllAccounts() == nil
}

// UpdateAccountGroup updates the group of an account
func (a *App) UpdateAccountGroup(accountID, group string) bool {
	if a.db == nil {
		return false
	}

	acc, err := a.db.GetAccount(accountID)
	if err != nil || acc == nil {
		return false
	}

	acc.Group = group
	return a.db.SaveAccount(*acc) == nil
}

// UpdateAccountsGroup updates the group of multiple accounts
func (a *App) UpdateAccountsGroup(accountIDs []string, group string) int {
	if a.db == nil {
		return 0
	}

	count := 0
	for _, id := range accountIDs {
		acc, err := a.db.GetAccount(id)
		if err != nil || acc == nil {
			continue
		}
		acc.Group = group
		if a.db.SaveAccount(*acc) == nil {
			count++
		}
	}
	return count
}

// UpdateAccount 更新账户基础信息（账户名、发行者、分组）
func (a *App) UpdateAccount(accountID, name, issuer, group string) bool {
	if a.db == nil {
		return false
	}

	acc, err := a.db.GetAccount(accountID)
	if err != nil || acc == nil {
		return false
	}

	// 只更新基础字段
	acc.Name = name
	acc.Issuer = issuer
	acc.Group = group

	return a.db.SaveAccount(*acc) == nil
}

// UpdateAccountAdvanced 更新账户高级选项（算法、位数、周期）
// 警告：修改这些参数会导致生成的验证码改变
func (a *App) UpdateAccountAdvanced(accountID, algorithm string, digits, period int) bool {
	if a.db == nil {
		return false
	}

	acc, err := a.db.GetAccount(accountID)
	if err != nil || acc == nil {
		return false
	}

	// 更新高级选项
	acc.Algorithm = algorithm
	acc.Digits = digits
	acc.Period = period

	return a.db.SaveAccount(*acc) == nil
}

// GetAccountSecret 获取账户密钥明文（需要密码验证）
func (a *App) GetAccountSecret(accountID, password string) string {
	if a.db == nil {
		return ""
	}

	// 如果启用了密码保护，必须验证密码
	if a.db.HasPassword() {
		if !a.db.VerifyPassword(password) {
			return ""
		}
	}

	acc, err := a.db.GetAccount(accountID)
	if err != nil || acc == nil {
		return ""
	}

	return acc.Secret
}

// GetGroups returns all unique group names
func (a *App) GetGroups() []string {
	if a.db == nil {
		return []string{}
	}

	accounts, _ := a.db.GetAllAccounts()
	groupMap := make(map[string]bool)
	for _, acc := range accounts {
		if acc.Group != "" {
			groupMap[acc.Group] = true
		}
	}

	groups := make([]string, 0, len(groupMap))
	for g := range groupMap {
		groups = append(groups, g)
	}
	return groups
}

// GenerateCode generates OTP code for an account
func (a *App) GenerateCode(accountID string) GenerateCodeResult {
	if a.db == nil {
		return GenerateCodeResult{Code: "------"}
	}

	acc, err := a.db.GetAccount(accountID)
	if err != nil || acc == nil {
		return GenerateCodeResult{
			Code:      "------",
			Remaining: 0,
			Progress:  0,
		}
	}

	// Generate code
	if strings.ToUpper(acc.Type) == "HOTP" {
		code, err := otp.GenerateHOTP(acc.Secret, acc.Algorithm, acc.Digits, acc.Counter)
		if err != nil {
			return GenerateCodeResult{Code: "ERROR"}
		}
		return GenerateCodeResult{
			Code:      code,
			Remaining: 0,
			Progress:  0,
		}
	}

	// TOTP
	period := acc.Period
	if period == 0 {
		period = 30
	}
	code, remaining, err := otp.GenerateTOTP(acc.Secret, acc.Algorithm, acc.Digits, period)
	if err != nil {
		return GenerateCodeResult{Code: "ERROR"}
	}

	progress := otp.GetProgress(period)

	return GenerateCodeResult{
		Code:      code,
		Remaining: remaining,
		Progress:  progress,
	}
}

// ExportToMigrationQR exports selected accounts to QR code
func (a *App) ExportToMigrationQR(accountIDs []string, size int) ExportQRResult {
	if a.db == nil {
		return ExportQRResult{Success: false, Message: "数据库未初始化"}
	}

	accounts, _ := a.db.GetAllAccounts()

	// Find accounts by IDs
	var selectedAccounts []*migration.OtpParameters
	for _, id := range accountIDs {
		for _, acc := range accounts {
			if acc.ID == id {
				// Decode secret
				secret, err := base32.StdEncoding.DecodeString(acc.Secret)
				if err != nil {
					continue
				}

				param := &migration.OtpParameters{
					Secret: secret,
					Name:   acc.Name,
					Issuer: acc.Issuer,
				}

				// Set algorithm
				switch strings.ToUpper(acc.Algorithm) {
				case "SHA256":
					param.Algorithm = migration.AlgorithmSHA256
				case "SHA512":
					param.Algorithm = migration.AlgorithmSHA512
				case "MD5":
					param.Algorithm = migration.AlgorithmMD5
				default:
					param.Algorithm = migration.AlgorithmSHA1
				}

				// Set digits
				if acc.Digits == 8 {
					param.Digits = migration.DigitCountEight
				} else {
					param.Digits = migration.DigitCountSix
				}

				// Set type
				if strings.ToUpper(acc.Type) == "HOTP" {
					param.Type = migration.OtpTypeHOTP
					param.Counter = acc.Counter
				} else {
					param.Type = migration.OtpTypeTOTP
				}

				selectedAccounts = append(selectedAccounts, param)
				break
			}
		}
	}

	if len(selectedAccounts) == 0 {
		return ExportQRResult{
			Success: false,
			Message: "没有选中任何账户",
		}
	}

	// Generate migration URI
	uri, err := migration.GenerateMigrationURISimple(selectedAccounts)
	if err != nil {
		return ExportQRResult{
			Success: false,
			Message: fmt.Sprintf("生成URI失败: %v", err),
		}
	}

	// Generate QR code
	if size == 0 {
		size = 512
	}
	qrDataURL, err := qrcode.GenerateQRCodeBase64(uri, size)
	if err != nil {
		return ExportQRResult{
			Success: false,
			Message: fmt.Sprintf("生成QR码失败: %v", err),
		}
	}

	return ExportQRResult{
		Success:   true,
		Message:   fmt.Sprintf("成功导出 %d 个账户", len(selectedAccounts)),
		QRCodeURL: qrDataURL,
	}
}
