package key

import (
	"context"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/decrypt"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/key/darwin"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
)

// Extractor 定义密钥提取器接口
type Extractor interface {
	// Extract 从进程中提取密钥
	// imgKey, derivedKeyMap (dbRelPath -> keyHex), error
	Extract(ctx context.Context, proc *model.Process) (string, map[string]string, error)

	SetValidate(validator *decrypt.Validator)
}

// NewExtractor 创建适合当前平台的密钥提取器
func NewExtractor(platform string, version int) (Extractor, error) {
	if platform == "darwin" && version == 4 {
		return darwin.NewV4Extractor(), nil
	}
	return nil, errors.PlatformUnsupported(platform, version)
}
