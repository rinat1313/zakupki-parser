package eis

import (
	"os"
	"path/filepath"
)

func writeFile(destPath string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destPath, body, 0o644)
}
