package darwin

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/decrypt/common"

	"golang.org/x/crypto/pbkdf2"
)

// Darwin Version 4 same as WIndows Version 4

// V4-specific constants.
const (
	V4PageSize     = 4096
	V4IterCount    = 256000
	HmacSHA512Size = 64
)

// V4Decryptor implements the V4 database decryptor.
type V4Decryptor struct {
	iterCount int
	hmacSize  int
	hashFunc  func() hash.Hash
	reserve   int
	pageSize  int
	version   string
}

// NewV4Decryptor creates a new V4 decryptor.
func NewV4Decryptor() *V4Decryptor {
	hashFunc := sha512.New
	hmacSize := HmacSHA512Size
	reserve := common.IVSize + hmacSize
	if reserve%common.AESBlockSize != 0 {
		reserve = ((reserve / common.AESBlockSize) + 1) * common.AESBlockSize
	}

	return &V4Decryptor{
		iterCount: V4IterCount,
		hmacSize:  hmacSize,
		hashFunc:  hashFunc,
		reserve:   reserve,
		pageSize:  V4PageSize,
		version:   "macOS v4",
	}
}

// deriveKeys derives the encryption key and MAC key via PBKDF2.
func (d *V4Decryptor) deriveKeys(key []byte, salt []byte) ([]byte, []byte) {
	encKey := pbkdf2.Key(key, salt, d.iterCount, common.KeySize, d.hashFunc)

	macSalt := common.XorBytes(salt, 0x3a)
	macKey := pbkdf2.Key(encKey, macSalt, 2, common.KeySize, d.hashFunc)

	return encKey, macKey
}

// deriveKeysFromDerived computes macKey from an already-derived encKey (skips expensive PBKDF2).
func (d *V4Decryptor) deriveKeysFromDerived(encKey []byte, salt []byte) ([]byte, []byte) {
	macSalt := common.XorBytes(salt, 0x3a)
	macKey := pbkdf2.Key(encKey, macSalt, 2, common.KeySize, d.hashFunc)
	return encKey, macKey
}

// Validate checks whether the raw key can decrypt the first page.
func (d *V4Decryptor) Validate(page1 []byte, key []byte) bool {
	if len(page1) < d.pageSize || len(key) != common.KeySize {
		return false
	}

	salt := page1[:common.SaltSize]
	return common.ValidateKey(page1, key, salt, d.hashFunc, d.hmacSize, d.reserve, d.pageSize, d.deriveKeys)
}

// ValidateDerived checks whether the derived encKey can decrypt the first page (2-round PBKDF2 only).
func (d *V4Decryptor) ValidateDerived(page1 []byte, encKey []byte) bool {
	if len(page1) < d.pageSize || len(encKey) != common.KeySize {
		return false
	}

	salt := page1[:common.SaltSize]
	return common.ValidateDerivedKey(page1, encKey, salt, d.hashFunc, d.hmacSize, d.reserve, d.pageSize)
}

// Decrypt decrypts a database file and writes plaintext SQLite to output.
func (d *V4Decryptor) Decrypt(ctx context.Context, dbfile string, hexKey string, output io.Writer) error {
	start := time.Now()
	dbName := filepath.Base(dbfile)
	log.Debug().Str("file", dbName).Msg("开始解密数据库")

	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return errors.DecodeKeyFailed(err)
	}

	dbInfo, err := common.OpenDBFile(dbfile, d.pageSize)
	if err != nil {
		return err
	}

	// try raw key first, then pre-derived encKey
	var encKey, macKey []byte
	if d.Validate(dbInfo.FirstPage, key) {
		log.Debug().Str("file", dbName).Msg("密钥验证通过（原始密钥），开始派生（256000轮）")
		encKey, macKey = d.deriveKeys(key, dbInfo.Salt)
	} else if d.ValidateDerived(dbInfo.FirstPage, key) {
		log.Debug().Str("file", dbName).Msg("密钥验证通过（已派生 encKey）")
		encKey, macKey = d.deriveKeysFromDerived(key, dbInfo.Salt)
	} else {
		log.Debug().Str("file", dbName).Msg("密钥验证失败")
		return errors.ErrDecryptIncorrectKey
	}

	dbFile, err := os.Open(dbfile)
	if err != nil {
		return errors.OpenFileFailed(dbfile, err)
	}
	defer dbFile.Close()

	// write SQLite header
	_, err = output.Write([]byte(common.SQLiteHeader))
	if err != nil {
		return errors.WriteOutputFailed(err)
	}

	pageBuf := make([]byte, d.pageSize)
	var decryptedPages int64

	for curPage := int64(0); curPage < dbInfo.TotalPages; curPage++ {
		select {
		case <-ctx.Done():
			return errors.ErrDecryptOperationCanceled
		default:
		}

		n, err := io.ReadFull(dbFile, pageBuf)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if n > 0 {
					break
				}
			}
			return errors.ReadFileFailed(dbfile, err)
		}

		allZeros := true
		for _, b := range pageBuf {
			if b != 0 {
				allZeros = false
				break
			}
		}

		if allZeros {
			_, err = output.Write(pageBuf)
			if err != nil {
				return errors.WriteOutputFailed(err)
			}
			continue
		}

		decryptedData, err := common.DecryptPage(pageBuf, encKey, macKey, curPage, d.hashFunc, d.hmacSize, d.reserve, d.pageSize)
		if err != nil {
			return err
		}

		_, err = output.Write(decryptedData)
		if err != nil {
			return errors.WriteOutputFailed(err)
		}
		decryptedPages++
	}

	log.Debug().
		Str("file", dbName).
		Int64("pages", decryptedPages).
		Str("elapsed", time.Since(start).Round(time.Millisecond).String()).
		Msg("数据库解密完成")

	return nil
}

// GetPageSize returns the page size used by this decryptor.
func (d *V4Decryptor) GetPageSize() int {
	return d.pageSize
}
