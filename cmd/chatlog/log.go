package chatlog

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/TE0dollary/chatlog-bot/internal/chatlog/ctx"
	"github.com/TE0dollary/chatlog-bot/internal/ui/logview"
	"github.com/TE0dollary/chatlog-bot/pkg/util"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Debug 控制是否启用调试级别日志。
var Debug bool

// TuiLogView 是 TUI 模式下的日志面板，由 initTuiLog 初始化后传入 App。
var TuiLogView *logview.LogView

// fileWriter 是日志轮转文件写入器，由 initLog 初始化，供 initTuiLog 复用。
var fileWriter *lumberjack.Logger

// initLog 是所有命令的 PersistentPreRun，统一设置日志级别和文件写入。
// 日志同时输出到：stderr（控制台可见）+ chatlog.log（按大小轮转，保留 7 天）。
// --debug 仅控制日志级别（debug / info），不影响文件写入。
func initLog(_ *cobra.Command, _ []string) {
	level := zerolog.InfoLevel
	if Debug {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)

	workDir := workDirFromConfig()
	_ = util.PrepareDir(workDir)
	fileWriter = &lumberjack.Logger{
		Filename:   filepath.Join(workDir, "chatlog.log"),
		MaxSize:    5,    // 单文件上限 50 MB，超出后自动轮转
		MaxAge:     7,    // 保留最近 7 天的历史文件
		MaxBackups: 7,    // 最多保留 7 个旧文件
		Compress:   true, // 旧文件 gzip 压缩节省空间
		LocalTime:  true, // 文件名时间戳使用本地时区
	}

	logOutput := io.MultiWriter(os.Stderr, fileWriter)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: logOutput, TimeFormat: time.RFC3339})
	logrus.SetOutput(logOutput)
}

// initTuiLog 是 TUI 根命令的 PreRun，在 initLog 之后执行。
// 复用 initLog 已建立的 fileWriter，将 stderr 替换为 TUI 日志面板。
func initTuiLog(_ *cobra.Command, _ []string) {
	lv := logview.New()
	TuiLogView = lv

	// 日志级别已由 initLog 设置，此处无需重复设置
	// 将 stderr 替换为 TUI 面板，保留文件输出
	logOutput := io.MultiWriter(lv, fileWriter)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: logOutput, NoColor: true, TimeFormat: time.RFC3339})
	logrus.SetOutput(logOutput)
}

// workDirFromConfig 从配置文件中读取基础工作目录，用于存放日志文件。
// configPath 为空时依次尝试 CHATLOG_DIR 环境变量和默认路径 ./data。
func workDirFromConfig() string {
	wCtx, err := ctx.New(configPath)
	if err != nil {
		return filepath.Join(".", "data")
	}
	return wCtx.GetBaseWorkDir()
}
