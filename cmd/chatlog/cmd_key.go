package chatlog

import (
	"context"
	"fmt"
	"sort"

	"github.com/TE0dollary/chatlog-bot/internal/chatlog/ctx"
	iwechat "github.com/TE0dollary/chatlog-bot/internal/wechat"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
	"github.com/TE0dollary/chatlog-bot/pkg/util/dat2img"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(keyCmd)
	keyCmd.Flags().IntVarP(&keyPID, "pid", "p", 0, "pid")
	keyCmd.Flags().BoolVarP(&keyForce, "force", "f", false, "force")
	keyCmd.Flags().BoolVarP(&keyShowXorKey, "xor-key", "x", false, "show xor key")
}

var (
	keyPID        int  // 指定微信进程 PID（多实例时使用）
	keyForce      bool // 强制重新提取密钥（即使已有缓存）
	keyShowXorKey bool // 是否同时显示 XOR 密钥（用于 .dat 图片解密）
)

// keyCmd 从运行中的微信进程内存中提取加密密钥。
// 在 macOS 上通过 vmmap 读取进程内存，搜索 ImgKey（图片密钥）和 DerivedKeyMap（每个数据库独立的派生密钥）。
// 注意：macOS 需要禁用 SIP 才能读取进程内存，提取过程约需 20 秒。
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "key",
	Run: func(cmd *cobra.Command, args []string) {
		ret, err := CommandKey(configPath, keyPID, keyForce, keyShowXorKey)
		if err != nil {
			log.Err(err).Msg("failed to get key")
			return
		}
		fmt.Println(ret)
	},
}

// CommandKey 处理 `chatlog key` 命令，从微信进程提取密钥并输出。
// 如果有多个微信进程，需要通过 pid 参数指定；单进程时自动选择。
func CommandKey(configPath string, pid int, force bool, showXorKey bool) (string, error) {
	wCtx, err := ctx.New(configPath)
	if err != nil {
		return "", err
	}

	wm := iwechat.NewManager()
	processes := wm.GetProcesses()
	if len(processes) == 0 {
		return "", fmt.Errorf("wechat process not found")
	}
	if len(processes) == 1 {
		return extractKeyResult(wCtx, wm, processes[0], force, showXorKey)
	}
	if pid == 0 {
		str := "Select a process:\n"
		for _, proc := range processes {
			str += fmt.Sprintf("PID: %d. %s[Version: %s Data Dir: %s ]\n", proc.PID, proc.Name, proc.FullVersion, proc.DataDir)
		}
		return str, nil
	}
	for _, proc := range processes {
		if proc.PID == uint32(pid) {
			return extractKeyResult(wCtx, wm, proc, force, showXorKey)
		}
	}
	return "", fmt.Errorf("wechat process not found")
}

// extractKeyResult 提取指定微信进程的密钥并格式化为结果字符串。
func extractKeyResult(wCtx *ctx.Context, wm *iwechat.Manager, proc *model.Process, force bool, showXorKey bool) (string, error) {

	// 检查是否有存在
	account := wCtx.GetAccounts()[proc.Name]
	if (len(account.ImgKey) == 0 && len(account.DerivedKeyMap) == 0) || force {
		imgKey, derivedKeyMap, err := wm.ExtractKey(context.Background(), proc.Name)
		if err != nil && len(derivedKeyMap) == 0 {
			return "", err
		}
		account.Account = proc.Name
		account.Platform = proc.Platform
		account.Version = proc.Version
		account.FullVersion = proc.FullVersion
		account.DataDir = proc.DataDir
		account.ImgKey = imgKey
		account.DerivedKeyMap = derivedKeyMap
		_ = wCtx.SetAccount(account)
	}

	result := fmt.Sprintf("Image Key: [%s]", account.ImgKey)
	if wCtx.GetVersion() == 4 && showXorKey {
		if b, err := dat2img.ScanAndSetXorKey(wCtx.DataDir); err == nil {
			result += fmt.Sprintf("\nXor Key: [0x%X]", b)
		}
	}
	result += formatDerivedKeyMap(account.DerivedKeyMap)
	return result, nil
}

// formatDerivedKeyMap 将派生密钥 map 格式化为可读字符串，按路径排序
func formatDerivedKeyMap(derivedKeyMap map[string]string) string {
	if len(derivedKeyMap) == 0 {
		return ""
	}
	paths := make([]string, 0, len(derivedKeyMap))
	for p := range derivedKeyMap {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	result := "\nDerived Keys per Database:"
	for _, p := range paths {
		result += fmt.Sprintf("\n  %s: [%s]", p, derivedKeyMap[p])
	}
	return result
}
