package ctx

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
	"github.com/TE0dollary/chatlog-bot/pkg/util"
)

// Context 是全局状态上下文，管理以下数据：
//   - 微信账号信息（账号名、平台、版本、数据目录）
//   - 加密密钥（DataKey 全局密钥、DerivedKeyMap 每库独立派生密钥、ImgKey 图片密钥）
//   - 服务状态（HTTP 服务是否启动、自动解密是否开启）
//   - 历史账号记录（多账号切换）
//
// Context 同时实现了 wechat.Config 和 database.Config 接口，作为配置传递给各服务。
// 配置持久化到 ./data/config.yaml（唯一的配置文件）。
type Context struct {
	cfg ConfigManager
	mu  sync.RWMutex

	// 存储额外传递进来的Key, 临时的
	DataKey string
	ImgKey  string
	DataDir string

	// 目录相关状态
	DataUsage string
	WorkUsage string

	// 自动解密
	LastSession time.Time

	// 当前选中的微信进程
	Process *model.Process

	// 所有可用的微信进程
	Processes []*model.Process
}

func New(configPath string) (*Context, error) {
	return NewWithConf(configPath, nil)
}

func NewWithConf(configPath string, cmdConf map[string]any) (*Context, error) {
	cfg, err := newConfigManager(configPath, cmdConf)
	if err != nil {
		return nil, err
	}
	ctx := &Context{
		cfg: cfg,
	}
	ctx.OnLoad()
	return ctx, nil
}

func (c *Context) OnLoad() {
	if c.Process == nil {
		// 如果配置中记录了上次使用的账号，从历史中恢复为默认 Process
		lastAccount := c.cfg.GetLastAccount()
		if lastAccount != "" {
			if hist, ok := c.cfg.GetAccounts()[lastAccount]; ok {
				c.SetProcess(&model.Process{
					Name:        hist.Account,
					Platform:    hist.Platform,
					Version:     hist.Version,
					FullVersion: hist.FullVersion,
					DataDir:     hist.DataDir,
				})
				c.DataDir = hist.DataDir
				c.DataKey = hist.DataKey
				c.ImgKey = hist.ImgKey
			}
		}
	}

	if c.Process != nil && c.Process.DataDir != "" {
		go func() {
			c.DataUsage = util.GetDirSize(c.Process.DataDir)
		}()
	}
	if c.WorkUsage == "" && c.GetWorkDir() != "" {
		go func() {
			c.WorkUsage = util.GetDirSize(c.GetWorkDir())
		}()
	}
}

func (c *Context) GetDataDir() string {
	if c.DataDir == "" {
		return c.Process.DataDir
	}
	return c.DataDir
}

func (c *Context) GetVersion() int {
	if c.Process == nil {
		return 0
	}
	return c.Process.Version
}

func (c *Context) GetDataKey() string {
	return c.DataKey
}

func (c *Context) GetHTTPAddr() string {
	return c.cfg.GetHTTPAddr()
}

func (c *Context) GetAccounts() map[string]AccountConfig {
	return c.cfg.GetAccounts()
}

func (c *Context) GetWebhook() *Webhook {
	return c.cfg.GetWebhook()
}

func (c *Context) GetSaveDecryptedMedia() bool {
	return true
}

func (c *Context) GetWorkDir() string {
	if c.Process == nil {
		return filepath.Join(c.cfg.GetBaseWorkDir(), "default")
	}
	return filepath.Join(c.cfg.GetBaseWorkDir(), c.Process.Name)
}

func (c *Context) GetBaseWorkDir() string {
	return c.cfg.GetBaseWorkDir()
}

func (c *Context) GetImgKey() string {
	return c.ImgKey
}

func (c *Context) GetProcess() *model.Process {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Process
}

func (c *Context) GetProcesses() []*model.Process {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Processes
}

func (c *Context) SetProcesses(processes []*model.Process) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Processes = processes
}

func (c *Context) SetDataKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DataKey = key
}

func (c *Context) GetLastSession() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LastSession
}

func (c *Context) SetLastSession(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastSession = t
}

func (c *Context) GetDataUsage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DataUsage
}

func (c *Context) GetWorkUsage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WorkUsage
}

// GetDataDirOverride 返回用户手动设置的数据目录（不做 fallback）。
// 与 GetDataDir 的区别：GetDataDir 在未设置时会返回 Process.DataDir。
func (c *Context) GetDataDirOverride() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DataDir
}

// Refresh 异步刷新数据目录和工作目录的磁盘占用统计。
func (c *Context) Refresh() {
	if c.Process != nil && c.Process.DataDir != "" {
		go func() {
			usage := util.GetDirSize(c.Process.DataDir)
			c.mu.Lock()
			c.DataUsage = usage
			c.mu.Unlock()
		}()
	}
	if c.GetWorkDir() != "" {
		go func() {
			usage := util.GetDirSize(c.GetWorkDir())
			c.mu.Lock()
			c.WorkUsage = usage
			c.mu.Unlock()
		}()
	}
}

func (c *Context) GetDerivedKeyMap() map[string]string {
	account, ok := c.cfg.GetAccounts()[c.Process.Name]
	if !ok {
		return nil
	}
	return account.DerivedKeyMap
}

func (c *Context) SetHTTPEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.GetHTTPEnabled() == enabled {
		return
	}
	_ = c.cfg.SetHTTPEnabled(enabled)
}

func (c *Context) SetHTTPAddr(addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.GetHTTPAddr() == addr {
		return
	}
	_ = c.cfg.SetHTTPAddr(addr)
}

func (c *Context) SetBaseWorkDir(dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.GetBaseWorkDir() == dir {
		return
	}
	_ = c.cfg.SetBaseWorkDir(dir)
}

func (c *Context) HasDecryptKey() bool {
	if c.Process == nil {
		return false
	}
	ac := c.cfg.GetAccounts()[c.Process.Name]
	if ac.DataKey != "" || len(ac.DerivedKeyMap) != 0 {
		return true
	}
	return false
}

func (c *Context) GetPlatform() string {
	return c.Process.Platform
}

func (c *Context) SetDataDir(dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.DataDir == dir {
		return
	}
	c.DataDir = dir
}

func (c *Context) SetImgKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ImgKey == key {
		return
	}
	c.ImgKey = key
}

func (c *Context) SetAccount(a AccountConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.SetAccount(a)
}

func (c *Context) SetProcess(p *model.Process) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Process = p
	c.DataDir = p.DataDir
}

func (c *Context) SetAutoDecrypt(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.GetAutoDecrypt() == enabled {
		return
	}
	_ = c.cfg.SetAutoDecrypt(enabled)

}

func (c *Context) GetAutoDecrypt() bool {
	return c.cfg.GetAutoDecrypt()
}

func (c *Context) GetHTTPEnabled() bool {
	return c.cfg.GetHTTPEnabled()
}
