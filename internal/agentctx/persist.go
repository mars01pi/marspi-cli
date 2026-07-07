package agentctx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// sessionTmpSuffix 是原子写入使用的临时文件后缀。
const sessionTmpSuffix = ".tmp"

// AtomicWriteFile 将 data 原子写入 path（同目录临时文件 + rename）。
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + sessionTmpSuffix
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// readSessionBytes 读取会话文件；主文件损坏时尝试从 .tmp 恢复。
func readSessionBytes(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil && json.Valid(data) {
		return data, nil
	}
	tmp := path + sessionTmpSuffix
	tmpData, tmpErr := os.ReadFile(tmp)
	if tmpErr == nil && json.Valid(tmpData) {
		_ = AtomicWriteFile(path, tmpData, 0o644)
		_ = os.Remove(tmp)
		return tmpData, nil
	}
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("read session: %w", err)
	}
	return nil, fmt.Errorf("invalid session json")
}
