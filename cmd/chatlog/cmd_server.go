package chatlog

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/chatlog/ctx"
	"github.com/TE0dollary/chatlog-bot/internal/chatlog/database"
	"github.com/TE0dollary/chatlog-bot/internal/chatlog/http"
	"github.com/TE0dollary/chatlog-bot/internal/chatlog/wechat"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.PersistentPreRun = initLog
	serverCmd.PersistentFlags().BoolVar(&Debug, "debug", false, "debug")
	serverCmd.Flags().StringVarP(&serverAddr, "addr", "a", "", "server address")
	serverCmd.Flags().StringVarP(&serverPlatform, "platform", "p", "", "platform")
	serverCmd.Flags().IntVarP(&serverVer, "version", "v", 0, "version")
	serverCmd.Flags().StringVarP(&serverDataDir, "data-dir", "d", "", "data dir")
	serverCmd.Flags().StringVarP(&serverDataKey, "data-key", "k", "", "data key")
	serverCmd.Flags().StringVarP(&serverImgKey, "img-key", "i", "", "img key")
	serverCmd.Flags().StringVarP(&serverWorkDir, "work-dir", "w", "", "work dir")
	serverCmd.Flags().BoolVarP(&serverAutoDecrypt, "auto-decrypt", "", false, "auto decrypt")
}

var (
	serverAddr        string // HTTP 服务监听地址（默认 0.0.0.0:5030）
	serverDataDir     string // 微信加密数据目录
	serverDataKey     string // 数据解密密钥
	serverImgKey      string // 图片解密密钥
	serverWorkDir     string // 解密后数据库存储目录
	serverPlatform    string // 平台类型
	serverVer         int    // 微信版本号
	serverAutoDecrypt bool   // 是否启用自动解密（监控数据目录变化并实时解密）
)

func getServerConfig() map[string]any {
	cmdConf := make(map[string]any)
	if len(serverAddr) != 0 {
		cmdConf["http_addr"] = serverAddr
	}
	if len(serverDataDir) != 0 {
		cmdConf["data_dir"] = serverDataDir
	}
	if len(serverDataKey) != 0 {
		cmdConf["data_key"] = serverDataKey
	}
	if len(serverImgKey) != 0 {
		cmdConf["img_key"] = serverImgKey
	}
	if len(serverWorkDir) != 0 {
		cmdConf["work_dir"] = serverWorkDir
	}
	if len(serverPlatform) != 0 {
		cmdConf["platform"] = serverPlatform
	}
	if serverVer != 0 {
		cmdConf["version"] = serverVer
	}
	if serverAutoDecrypt {
		cmdConf["auto_decrypt"] = true
	}
	return cmdConf
}

// serverCmd 以无 TUI 的纯 HTTP 服务模式启动。
// 适用于无人值守场景（如 Docker 部署），启动后提供：
//   - REST API（/api/v1/*）：查询聊天记录、联系人、群聊、会话
//   - 媒体文件服务（/image/*, /video/*, /voice/*, /file/*）
//   - MCP 协议（/mcp, /sse）：供 AI 助手通过 MCP 查询聊天记录
//   - 内嵌 Web 前端（/static/*）
//
// 如果工作目录为空，会自动先执行解密。
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start HTTP server",
	Run: func(cmd *cobra.Command, args []string) {

		cmdConf := getServerConfig()
		log.Info().Msgf("server cmd config: %+v", cmdConf)

		if err := CommandHTTPServer(configPath, cmdConf); err != nil {
			log.Err(err).Msg("failed to start server")
			return
		}
	},
}

// CommandHTTPServer 处理 `chatlog server` 命令，以无 TUI 的纯 HTTP 服务模式启动。
// 流程：加载配置 → 初始化解密和数据库服务 → 按需自动解密 → 阻塞式启动 HTTP 服务。
func CommandHTTPServer(configPath string, cmdConf map[string]any) error {

	wCtx, err := ctx.NewWithConf(configPath, cmdConf)
	if err != nil {
		return err
	}

	dataDir := wCtx.GetDataDir()
	workDir := wCtx.GetWorkDir()
	if len(dataDir) == 0 && len(workDir) == 0 {
		return fmt.Errorf("dataDir or workDir is required")
	}

	if !wCtx.HasDecryptKey() {
		return fmt.Errorf("dataKey or derivedKeyMap is required")
	}

	log.Info().Msgf("server config: %+v", wCtx)

	wechat := wechat.NewService(wCtx)
	db := database.NewService(wCtx)
	http := http.NewService(wCtx, db)

	if wCtx.GetAutoDecrypt() {
		if err := wechat.StartAutoDecrypt(); err != nil {
			return err
		}
		log.Info().Msg("auto decrypt is enabled")
	}

	// init db
	go func() {
		// 如果工作目录为空，则解密数据
		if entries, err := os.ReadDir(workDir); err == nil && len(entries) == 0 {
			log.Info().Msgf("work dir is empty, decrypt data.")
			db.SetDecrypting()
			if err := wechat.DecryptDBFiles(); err != nil {
				log.Info().Msgf("decrypt data failed: %v", err)
				return
			}
			log.Info().Msg("decrypt data success")
		}

		// 按依赖顺序启动服务
		if err := db.Start(); err != nil {
			log.Info().Msgf("start db failed, try to decrypt data.")
			db.SetDecrypting()
			if err := wechat.DecryptDBFiles(); err != nil {
				log.Info().Msgf("decrypt data failed: %v", err)
				return
			}
			log.Info().Msg("decrypt data success")
			if err := db.Start(); err != nil {
				log.Info().Msgf("start db failed: %v", err)
				db.SetError(err.Error())
				return
			}
		}
	}()

	return http.ListenAndServe()
}
