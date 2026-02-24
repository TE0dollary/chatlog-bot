package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// DefaultWorkDir returns the default work directory path for decrypted output.
// Each account gets its own subdirectory under ./data/.
func DefaultWorkDir(account string) string {
	if account == "" {
		account = "default"
	}
	return filepath.Join(".", "data", account)
}

func GetDirSize(dir string) string {
	var size int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			size += info.Size()
		}
		return nil
	})
	return ByteCountSI(size)
}

func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

// PrepareDir ensures that the specified directory path exists.
// If the directory does not exist, it attempts to create it.
func PrepareDir(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	} else if !stat.IsDir() {
		log.Debug().Msgf("%s is not a directory", path)
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}
