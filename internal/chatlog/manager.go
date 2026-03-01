package chatlog

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/chatlog/ctx"
	"github.com/TE0dollary/chatlog-bot/internal/chatlog/database"
	"github.com/TE0dollary/chatlog-bot/internal/chatlog/http"
	"github.com/TE0dollary/chatlog-bot/internal/chatlog/wechat"
	"github.com/TE0dollary/chatlog-bot/internal/ui/logview"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
	"github.com/TE0dollary/chatlog-bot/pkg/util"
	"github.com/TE0dollary/chatlog-bot/pkg/util/dat2img"
)

// Manager 是应用核心调度器，协调以下组件：
//   - ctx.Context：全局状态管理（账号信息、密钥、目录路径、服务状态等）
//   - wechat.Service：微信数据解密服务（密钥提取、数据库解密、自动解密监控）
//   - database.Service：数据库访问层（通过 wechatdb 读取解密后的 SQLite 数据）
//   - http.Service：HTTP/MCP 服务器（Gin 框架，提供 REST API 和 MCP 协议）
//   - App：TUI 终端界面（tview 框架）
//
// Manager 支持两种运行模式：
//   - TUI 模式（Run）：启动交互式终端界面，用户通过菜单操作
//   - Server 模式（CommandHTTPServer）：无 TUI，直接启动 HTTP 服务
type Manager struct {
	ctx *ctx.Context // 全局状态上下文（TUI 模式使用）

	// Services - 三个核心服务
	db     *database.Service // 数据库访问服务
	http   *http.Service     // HTTP/MCP 服务器
	wechat *wechat.Service   // 微信解密服务

	// Terminal UI
	app *App // TUI 界面（仅 TUI 模式使用）
}

// New 创建 Manager 实例并完成初始化。
// 流程：加载配置 → 初始化三大服务 → 探测微信进程 → 恢复上次的 HTTP 服务状态。
func New(configPath string) (*Manager, error) {
	log.Info().Msg("初始化 chatlog")
	m := &Manager{}

	var err error
	m.ctx, err = ctx.New(configPath)
	if err != nil {
		return nil, err
	}

	m.wechat = wechat.NewService(m.ctx)

	m.db = database.NewService(m.ctx)

	m.http = http.NewService(m.ctx, m.db)

	// 探测当前运行的微信实例，自动选中第一个
	m.ctx.SetProcesses(m.wechat.Processes())
	procs := m.ctx.GetProcesses()
	if len(procs) >= 1 {
		m.ctx.SetProcess(procs[0])
		log.Info().Int("count", len(procs)).Str("account", procs[0].Name).Msg("检测到微信进程，自动选中")
	} else {
		log.Info().Msg("未检测到运行中的微信进程")
	}

	if m.ctx.GetHTTPEnabled() {
		// 恢复上次退出时的 HTTP 服务状态
		log.Info().Msg("恢复上次 HTTP 服务状态")
		if err := m.StartService(); err != nil {
			_ = m.StopService()
		}
	}

	if m.ctx.GetAutoDecrypt() {
		// 恢复上次退出时的自动解密状态
		log.Info().Msg("恢复上次自动解密状态")
		if err := m.StartAutoDecrypt(); err != nil {
			log.Warn().Err(err).Msg("自动解密恢复失败，已重置状态")
			m.ctx.SetAutoDecrypt(false)
		}
	}

	return m, nil
}

// GetWorkDir 返回当前账号的工作目录路径。
func (m *Manager) GetWorkDir() string {
	return m.ctx.GetWorkDir()
}

// Run 以 TUI 模式启动应用（阻塞直到用户退出）。
func (m *Manager) Run() {
	m.app = NewApp(m.ctx, m, nil)
	_ = m.app.Run()
}

// RunWithLogView 以 TUI 模式启动应用，并将日志面板嵌入界面。
func (m *Manager) RunWithLogView(lv *logview.LogView) {
	m.app = NewApp(m.ctx, m, lv)
	_ = m.app.Run()
}

// Switch 切换当前操作的微信账号。
// 支持切换到运行中的微信进程（info != nil）或历史账号（通过 history 名称查找）。
// 切换时会先停止自动解密和 HTTP 服务，切换完成后按需恢复。
func (m *Manager) Switch(info *model.Process, history string) error {
	if m.ctx.GetAutoDecrypt() {
		if err := m.StopAutoDecrypt(); err != nil {
			return err
		}
	}
	if m.ctx.GetHTTPEnabled() {
		if err := m.stopService(); err != nil {
			return err
		}
		defer func() {
			// 启动HTTP服务
			if err := m.StartService(); err != nil {
				log.Info().Err(err).Msg("启动服务失败")
				_ = m.StopService()
			}
		}()
	}
	if info != nil {
		m.ctx.SetProcess(info)
		return nil
	}
	pConf := m.ctx.GetAccounts()[history]
	proc := &model.Process{
		Name:        pConf.Account,
		Platform:    pConf.Platform,
		Version:     pConf.Version,
		FullVersion: pConf.FullVersion,
		DataDir:     pConf.DataDir,
		Status:      model.StatusOffline,
	}
	m.ctx.SetProcess(proc)
	return nil
}

// StartService 按依赖顺序启动数据库服务和 HTTP 服务。
func (m *Manager) StartService() error {
	log.Info().Msg("启动数据库和 HTTP 服务")

	// 按依赖顺序启动服务：先数据库，后 HTTP
	if err := m.db.Start(); err != nil {
		log.Error().Err(err).Msg("数据库服务启动失败")
		return err
	}

	if err := m.http.Start(); err != nil {
		log.Error().Err(err).Msg("HTTP 服务启动失败")
		m.db.Stop()
		return err
	}

	// 如果是 4.0 版本，更新下 xorkey
	if m.ctx.GetVersion() == 4 {
		dat2img.SetAesKey(m.ctx.GetImgKey())
		go dat2img.ScanAndSetXorKey(m.ctx.GetDataDir())
	}

	// 更新状态
	m.ctx.SetHTTPEnabled(true)
	log.Info().Msg("服务启动成功")

	return nil
}

// StopService 停止 HTTP 和数据库服务并更新状态。
func (m *Manager) StopService() error {
	log.Info().Msg("停止 HTTP 和数据库服务")
	if err := m.stopService(); err != nil {
		return err
	}

	// 更新状态
	m.ctx.SetHTTPEnabled(false)
	log.Info().Msg("服务已停止")

	return nil
}

func (m *Manager) stopService() error {
	// 按依赖的反序停止服务
	var errs []error

	if err := m.http.Stop(); err != nil {
		errs = append(errs, err)
	}

	if err := m.db.Stop(); err != nil {
		errs = append(errs, err)
	}

	// 如果有错误，返回第一个错误
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

func (m *Manager) SetHTTPAddr(text string) error {
	var addr string
	if util.IsNumeric(text) {
		addr = fmt.Sprintf("127.0.0.1:%s", text)
	} else if strings.HasPrefix(text, "http://") {
		addr = strings.TrimPrefix(text, "http://")
	} else if strings.HasPrefix(text, "https://") {
		addr = strings.TrimPrefix(text, "https://")
	} else {
		addr = text
	}
	m.ctx.SetHTTPAddr(addr)
	return nil
}

// GetDataKey 从当前微信进程中提取加密密钥并更新配置。
func (m *Manager) GetDataKey() error {
	process := m.ctx.GetProcess()
	if process == nil {
		return fmt.Errorf("未选择任何账号")
	}
	log.Info().Str("account", process.Name).Uint32("pid", process.PID).Msg("开始提取数据密钥")
	imgKey, derivedKeyMap, err := m.wechat.ExtractKey(context.Background(), process)
	if err != nil && len(derivedKeyMap) == 0 {
		log.Error().Err(err).Msg("密钥提取失败")
		return err
	}
	account := m.ctx.GetAccounts()[process.Name]
	account.Account = process.Name
	account.Platform = process.Platform
	account.Version = process.Version
	account.FullVersion = process.FullVersion
	account.DataDir = process.DataDir
	account.ImgKey = imgKey
	account.DerivedKeyMap = derivedKeyMap
	log.Info().Int("derived_keys", len(derivedKeyMap)).Bool("img_key", imgKey != "").Msg("密钥提取成功，已保存配置")
	return m.ctx.SetAccount(account)
}

// DecryptDBFiles 解密所有微信数据库文件。如果尚未提取密钥，会自动先提取。
func (m *Manager) DecryptDBFiles() error {
	if len(m.ctx.GetDerivedKeyMap()) == 0 {
		if m.ctx.GetProcess() == nil {
			return fmt.Errorf("未选择任何账号")
		}
		if err := m.GetDataKey(); err != nil {
			return err
		}
	}

	log.Info().Str("data_dir", m.ctx.GetDataDir()).Msg("开始批量解密数据库文件")
	if err := m.wechat.DecryptDBFiles(); err != nil {
		log.Error().Err(err).Msg("批量解密失败")
		return err
	}
	log.Info().Msg("批量解密完成")
	m.ctx.Refresh()
	return nil
}

// StartAutoDecrypt 启动文件监控，自动解密数据目录中新增或更新的 .db 文件。
func (m *Manager) StartAutoDecrypt() error {
	if len(m.ctx.GetDerivedKeyMap()) == 0 || m.ctx.GetDataDirOverride() == "" {
		return fmt.Errorf("请先获取密钥")
	}
	if m.ctx.GetWorkDir() == "" {
		return fmt.Errorf("请先执行解密数据")
	}

	log.Info().Str("data_dir", m.ctx.GetDataDir()).Msg("启动自动解密")
	if err := m.wechat.StartAutoDecrypt(); err != nil {
		log.Error().Err(err).Msg("自动解密启动失败")
		return err
	}

	m.ctx.SetAutoDecrypt(true)
	return nil
}

func (m *Manager) StopAutoDecrypt() error {
	log.Info().Msg("停止自动解密")
	if err := m.wechat.StopAutoDecrypt(); err != nil {
		return err
	}
	m.ctx.SetAutoDecrypt(false)
	return nil
}

func (m *Manager) RefreshSession() error {
	if m.db.GetDB() == nil {
		if err := m.db.Start(); err != nil {
			return err
		}
	}
	resp, err := m.db.GetSessions("", 1, 0)
	if err != nil {
		return err
	}
	if len(resp.Items) == 0 {
		return nil
	}
	m.ctx.SetLastSession(resp.Items[0].NTime)
	return nil
}
