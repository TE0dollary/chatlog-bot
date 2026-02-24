package decrypt

import (
	"context"
	"io"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/decrypt/darwin"
)

// Decryptor 定义数据库解密的接口
type Decryptor interface {
	// Decrypt 解密数据库
	Decrypt(ctx context.Context, dbfile string, key string, output io.Writer) error

	// Validate 验证密钥是否有效
	Validate(page1 []byte, key []byte) bool

	// ValidateDerived 验证已派生的 encKey 是否有效（跳过昂贵的 PBKDF2 派生）
	ValidateDerived(page1 []byte, encKey []byte) bool

	// GetPageSize 返回页面大小
	GetPageSize() int
}

// NewDecryptor 创建一个新的解密器
func NewDecryptor(platform string, version int) (Decryptor, error) {
	if platform == "darwin" && version == 4 {
		return darwin.NewV4Decryptor(), nil
	}
	return nil, errors.PlatformUnsupported(platform, version)
}
