// Package chatlog 定义 CLI 命令层，使用 cobra 框架组织所有子命令。
// 根命令（无参数直接运行）启动 TUI 交互界面，其他子命令见各 cmd_*.go 文件。
package chatlog

import (
	"github.com/TE0dollary/chatlog-bot/internal/chatlog"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// configPath 是配置目录路径，通过 --config 参数传入，空值时使用 CHATLOG_DIR 环境变量或默认 ./data。
var configPath string

func init() {
	// windows only：禁用 cobra 对从资源管理器双击运行的提示
	cobra.MousetrapHelpText = ""

	rootCmd.PersistentFlags().BoolVar(&Debug, "debug", false, "debug")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config directory path")
	rootCmd.PersistentPreRun = initLog
}

// Execute 是 CLI 的入口，由 main.go 调用。
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Err(err).Msg("command execution failed")
	}
}

// rootCmd 是根命令，不带子命令时启动 TUI 交互界面。
// TUI 界面基于 tview 框架，提供菜单式操作：获取密钥、解密数据、启停 HTTP 服务等。
var rootCmd = &cobra.Command{
	Use:     "chatlog",
	Short:   "chatlog",
	Long:    `chatlog`,
	Example: `chatlog`,
	Args:    cobra.MinimumNArgs(0),
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	PreRun: initTuiLog, // TUI 模式下日志输出到文件而非终端
	Run:    Root,
}

// Root 处理根命令，创建 Manager 并启动 TUI 界面。
func Root(_ *cobra.Command, _ []string) {
	m, err := chatlog.New(configPath)
	if err != nil {
		log.Err(err).Msg("failed to initialize chatlog instance")
		return
	}
	m.RunWithLogView(TuiLogView)
}
