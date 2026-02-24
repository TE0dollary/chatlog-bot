package decrypt

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/wechat/decrypt/common"
	"github.com/TE0dollary/chatlog-bot/pkg/util/dat2img"
)

type Validator struct {
	platform        string
	version         int
	dataDir         string
	dbPath          string
	decryptor       Decryptor
	dbFile          *common.DBFile
	allDBFiles      []*common.DBFile // all db files for derived-key scan
	imgKeyValidator *dat2img.AesKeyValidator
}

// NewValidator 创建一个仅用于验证的验证器
func NewValidator(platform string, version int, dataDir string) (*Validator, error) {
	return NewValidatorWithFile(platform, version, dataDir)
}

func NewValidatorWithFile(platform string, version int, dataDir string) (*Validator, error) {
	dbFile := GetSimpleDBFile(platform, version)
	dbPath := filepath.Join(dataDir, dbFile)
	decryptor, err := NewDecryptor(platform, version)
	if err != nil {
		return nil, err
	}
	d, err := common.OpenDBFile(dbPath, decryptor.GetPageSize())
	if err != nil {
		return nil, err
	}

	validator := &Validator{
		platform:  platform,
		version:   version,
		dataDir:   dataDir,
		dbPath:    dbPath,
		decryptor: decryptor,
		dbFile:    d,
	}

	if version == 4 {
		validator.imgKeyValidator = dat2img.NewImgKeyValidator(dataDir)
	}

	return validator, nil
}

func (v *Validator) ValidateDerived(key []byte) bool {
	return v.decryptor.ValidateDerived(v.dbFile.FirstPage, key)
}

// ValidateDerivedAll checks whether key is the encKey for any known database
// and returns the matching relative paths.
func (v *Validator) ValidateDerivedAll(key []byte) []string {
	var result []string
	for _, dbFile := range v.allDBFiles {
		if v.decryptor.ValidateDerived(dbFile.FirstPage, key) {
			relPath, err := filepath.Rel(v.dataDir, dbFile.Path)
			if err != nil {
				relPath = dbFile.Path
			}
			result = append(result, relPath)
		}
	}
	return result
}

// ScanAllDBFiles scans dataDir for all encrypted SQLite database files.
func (v *Validator) ScanAllDBFiles() error {
	v.allDBFiles = nil
	pageSize := v.decryptor.GetPageSize()

	err := filepath.Walk(v.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			return nil
		}
		lower := strings.ToLower(info.Name())
		if !strings.HasSuffix(lower, ".db") {
			return nil
		}
		// skip FTS (full-text search) auxiliary files
		if strings.Contains(lower, "fts") {
			return nil
		}
		dbFile, err := common.OpenDBFile(path, pageSize)
		if err != nil {
			return nil // skip invalid or already-decrypted files
		}
		v.allDBFiles = append(v.allDBFiles, dbFile)
		return nil
	})

	log.Debug().Msgf("ScanAllDBFiles: found %d encrypted db files in %s", len(v.allDBFiles), v.dataDir)
	return err
}

func (v *Validator) ValidateImgKey(key []byte) bool {
	if v.imgKeyValidator == nil {
		return false
	}
	return v.imgKeyValidator.Validate(key)
}

func GetSimpleDBFile(platform string, version int) string {
	switch {
	case platform == "windows" && version == 4:
		return "db_storage\\message\\message_0.db"
	case platform == "darwin" && version == 4:
		return "db_storage/message/message_0.db"
	}
	return ""
}
