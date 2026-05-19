package platform

import (
	"encoding/hex"
	"encoding/json"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type lockMetadata struct {
	PID        int    `json:"pid"`
	Executable string `json:"executable"`
	StartedAt  int64  `json:"started_at"`
}

var lockFilePath string

func AcquireAppLock() bool {
	if AcquireSingleInstance(getInstanceLockName()) {
		cleanupLegacyLockFile()
		return true
	}
	if SupportsSingleInstance() {
		return false
	}

	lockFilePath = getLockFilePath()
	if lockFilePath == "" {
		return true
	}

	dataDir := filepath.Dir(lockFilePath)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return true
	}

	current, ok := currentProcessMetadata()
	if !ok {
		current = lockMetadata{PID: os.Getpid()}
	}
	current.Executable = normalizeExecutablePath(current.Executable)

	if metadata, err := readLockMetadata(lockFilePath); err == nil && metadata.PID > 0 {
		if process, ok := lookupProcessMetadata(metadata.PID); ok && isSameProcess(metadata, process, current) {
			return false
		}
		if err := os.Remove(lockFilePath); err != nil {
			return false
		}
	}

	if err := writeLockMetadata(lockFilePath, current); err != nil {
		return true
	}

	return true
}

func ReleaseAppLock() {
	ReleaseSingleInstance()
	if lockFilePath == "" {
		return
	}
	_ = os.Remove(lockFilePath)
}

func getLockFilePath() string {
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execDir := filepath.Dir(execPath)
	dataDir := filepath.Join(execDir, "data")
	return filepath.Join(dataDir, ".lock")
}

func getInstanceLockName() string {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "google-authenticator"
	}

	hash := fnv.New128a()
	_, _ = hash.Write([]byte(normalizeExecutablePath(execPath)))
	return "google-authenticator:" + hex.EncodeToString(hash.Sum(nil))
}

func currentProcessMetadata() (lockMetadata, bool) {
	info, ok := GetProcessMetadata(os.Getpid())
	if !ok {
		return lockMetadata{}, false
	}
	return lockMetadata{
		PID:        info.PID,
		Executable: info.Executable,
		StartedAt:  info.StartedAt,
	}, true
}

func lookupProcessMetadata(pid int) (lockMetadata, bool) {
	info, ok := GetProcessMetadata(pid)
	if !ok {
		return lockMetadata{}, false
	}
	return lockMetadata{
		PID:        info.PID,
		Executable: info.Executable,
		StartedAt:  info.StartedAt,
	}, true
}

func readLockMetadata(path string) (lockMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return lockMetadata{}, err
	}

	var metadata lockMetadata
	if err := json.Unmarshal(data, &metadata); err == nil && metadata.PID > 0 {
		metadata.Executable = normalizeExecutablePath(metadata.Executable)
		return metadata, nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return lockMetadata{}, err
	}

	return lockMetadata{PID: pid}, nil
}

func writeLockMetadata(path string, metadata lockMetadata) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func normalizeExecutablePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return strings.ToLower(filepath.Clean(path))
}

func isSameProcess(lock lockMetadata, process lockMetadata, current lockMetadata) bool {
	if lock.PID <= 0 || process.PID <= 0 || lock.PID != process.PID {
		return false
	}
	if lock.StartedAt > 0 && process.StartedAt > 0 {
		return lock.StartedAt == process.StartedAt
	}
	if lock.Executable != "" && process.Executable != "" {
		return lock.Executable == process.Executable
	}
	if lock.Executable == "" && lock.StartedAt == 0 && process.Executable != "" && current.Executable != "" {
		return process.Executable == current.Executable
	}
	return true
}

func cleanupLegacyLockFile() {
	lockFilePath = getLockFilePath()
	if lockFilePath == "" {
		return
	}

	metadata, err := readLockMetadata(lockFilePath)
	if err != nil || metadata.PID <= 0 {
		_ = os.Remove(lockFilePath)
		return
	}

	current, ok := currentProcessMetadata()
	if !ok {
		current = lockMetadata{PID: os.Getpid()}
	}
	current.Executable = normalizeExecutablePath(current.Executable)

	process, ok := lookupProcessMetadata(metadata.PID)
	if !ok || !isSameProcess(metadata, process, current) {
		_ = os.Remove(lockFilePath)
	}
}
