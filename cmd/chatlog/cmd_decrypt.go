package chatlog

import (
	"fmt"

	"github.com/TE0dollary/chatlog-bot/internal/chatlog/ctx"
	"github.com/TE0dollary/chatlog-bot/internal/chatlog/wechat"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(decryptCmd)
	decryptCmd.Flags().StringVarP(&decryptPlatform, "platform", "p", "", "platform")
	decryptCmd.Flags().IntVarP(&decryptVer, "version", "v", 0, "version")
	decryptCmd.Flags().StringVarP(&decryptDataDir, "data-dir", "d", "", "data dir")
	decryptCmd.Flags().StringVarP(&decryptDatakey, "data-key", "k", "", "data key")
	decryptCmd.Flags().StringVarP(&decryptWorkDir, "work-dir", "w", "", "work dir")
}

var (
	decryptPlatform string // 平台类型（darwin/windows）
	decryptVer      int    // 微信版本号（3 或 4）
	decryptDataDir  string // 微信加密数据库所在目录
	decryptDatakey  string // 数据解密密钥（hex 编码）
	decryptWorkDir  string // 解密后数据库的输出目录
)

func getDecryptConfig() map[string]any {
	cmdConf := make(map[string]any)
	if len(decryptDataDir) != 0 {
		cmdConf["data_dir"] = decryptDataDir
	}
	if len(decryptDatakey) != 0 {
		cmdConf["data_key"] = decryptDatakey
	}
	if len(decryptWorkDir) != 0 {
		cmdConf["work_dir"] = decryptWorkDir
	}
	if len(decryptPlatform) != 0 {
		cmdConf["platform"] = decryptPlatform
	}
	if decryptVer != 0 {
		cmdConf["version"] = decryptVer
	}
	return cmdConf
}

// decryptCmd 解密微信本地加密的 SQLite 数据库文件。
// 使用 SQLCipher 兼容算法（PBKDF2-HMAC-SHA512 + AES-CBC），逐页解密后输出标准 SQLite 文件。
// 解密后的数据库保存到 work-dir，保持与原始目录相同的子目录结构。
var decryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "decrypt",
	Run: func(cmd *cobra.Command, args []string) {
		cmdConf := getDecryptConfig()
		if err := CommandDecrypt(configPath, cmdConf); err != nil {
			log.Err(err).Msg("failed to decrypt")
			return
		}
		fmt.Println("decrypt success")
	},
}

// CommandDecrypt 处理 `chatlog decrypt` 命令，加载统一配置并批量解密数据库。
func CommandDecrypt(configPath string, cmdConf map[string]any) error {
	wCtx, err := ctx.NewWithConf(configPath, cmdConf)
	if err != nil {
		return err
	}
	if len(wCtx.GetDataDir()) == 0 {
		return fmt.Errorf("dataDir is required")
	}
	if !wCtx.HasDecryptKey() {
		return fmt.Errorf("dataKey or derivedKeyMap is required")
	}
	if err := wechat.NewService(wCtx).DecryptDBFiles(); err != nil {
		return err
	}
	return nil
}
