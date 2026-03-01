package ctx

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/TE0dollary/chatlog-bot/pkg/config"
)

const (
	AppName      = "chatlog"
	ConfigName   = "config"
	EnvPrefix    = "CHATLOG"
	EnvConfigDir = "CHATLOG_DIR"

	DefaultHTTPAddr = "127.0.0.1:5030"
)

// ConfigManager 封装配置数据与配置文件管理，对外提供读写接口。
type ConfigManager interface {
	GetLastAccount() string
	GetAccounts() map[string]AccountConfig
	GetWebhook() *Webhook
	GetAutoDecrypt() bool
	GetBaseWorkDir() string
	GetHTTPAddr() string
	GetHTTPEnabled() bool

	SetBaseWorkDir(workDir string) error
	SetAutoDecrypt(bool) error
	SetAccount(account AccountConfig) error
	SetHTTPAddr(addr string) error
	SetHTTPEnabled(enabled bool) error

	Save() error
}

// appConfig 是统一的配置结构体，合并了原来的 TUIConfig 和 ServerConfig。
// TUI 和 Server 模式共用此结构，同时实现所有服务层接口。
// 账号相关数据（平台、版本、目录、密钥等）统一存储在 History[LastAccount] 中，
// 命令行参数作为临时覆盖，getter 方法优先使用命令行参数，再回退到 History[LastAccount]。
type appConfig struct {
	ConfigDir string `mapstructure:"-" json:"-"` // 配置文件所在目录（运行时填充，不持久化）

	BaseWorkDir string `mapstructure:"base_work_dir" yaml:"base_work_dir,omitempty"`
	HTTPEnabled bool   `mapstructure:"http_enabled" yaml:"http_enabled,omitempty"`
	HTTPAddr    string `mapstructure:"http_addr" yaml:"http_addr,omitempty"`
	// 当前/上次使用的账号
	LastAccount string `mapstructure:"last_account" yaml:"last_account,omitempty"`

	// 自动解密
	AutoDecrypt bool `mapstructure:"auto_decrypt" yaml:"auto_decrypt,omitempty"`

	// 媒体保存
	SaveDecryptedMedia bool `mapstructure:"save_decrypted_media" yaml:"save_decrypted_media,omitempty"`

	// 多账号历史
	Accounts map[string]AccountConfig `mapstructure:"accounts" yaml:"history,omitempty"`

	// Webhook
	Webhook *Webhook `mapstructure:"webhook" yaml:"webhook,omitempty"`
}

// configDefaults 配置默认值
var configDefaults = map[string]any{
	"http_addr":            DefaultHTTPAddr,
	"save_decrypted_media": true,
	"base_work_dir":        "./data",
}

// AccountConfig 表示一个历史账号的配置快照
type AccountConfig struct {
	Account       string            `mapstructure:"account" yaml:"account,omitempty"`
	Platform      string            `mapstructure:"platform" yaml:"platform,omitempty"`
	Version       int               `mapstructure:"version" yaml:"version,omitempty"`
	FullVersion   string            `mapstructure:"full_version" yaml:"full_version,omitempty"`
	DataDir       string            `mapstructure:"data_dir" yaml:"data_dir,omitempty"`
	DataKey       string            `mapstructure:"data_key" yaml:"data_key,omitempty"`
	ImgKey        string            `mapstructure:"img_key" yaml:"img_key,omitempty"`
	DerivedKeyMap map[string]string `mapstructure:"derived_key_map" yaml:"derived_key_map,omitempty"`
	LastTime      int64             `mapstructure:"last_time" yaml:"last_time,omitempty"`
	Files         []File            `mapstructure:"files" yaml:"files,omitempty"`
}

// File 记录已处理文件的元数据
type File struct {
	Path         string `mapstructure:"path" yaml:"path,omitempty"`
	ModifiedTime int64  `mapstructure:"modified_time" yaml:"modified_time,omitempty"`
	Size         int64  `mapstructure:"size" yaml:"size,omitempty"`
}

// configManager 将配置数据（appConfig）与文件管理器（config.Manager）封装在一起。
type configManager struct {
	data *appConfig
	cm   *config.Manager
}

// newConfigManager 加载配置文件并返回 configManager。
// configPath: 配置目录路径（为空则使用 CHATLOG_DIR 环境变量或默认 ./data）
// cmdConf: 命令行参数覆盖的配置项（可为 nil）
func newConfigManager(configPath string, cmdConf map[string]any) (*configManager, error) {
	if configPath == "" {
		configPath = os.Getenv(EnvConfigDir)
	}
	cm, err := config.New(configPath, ConfigName, EnvPrefix, true)
	if err != nil {
		log.Error().Err(err).Msg("load config failed")
		return nil, err
	}
	data := &appConfig{}
	config.SetDefaults(cm.Viper, data, configDefaults)

	// 应用命令行参数覆盖
	for key, value := range cmdConf {
		_ = cm.SetConfig(key, value)
	}
	if err := cm.Load(data); err != nil {
		log.Error().Err(err).Msg("load config failed")
		return nil, err
	}
	data.ConfigDir = cm.Path
	log.Info().Msgf("config loaded from: %s", cm.Path)
	return &configManager{data: data, cm: cm}, nil
}

func (m *configManager) GetLastAccount() string {
	return m.data.LastAccount
}

func (m *configManager) GetAccounts() map[string]AccountConfig {
	return m.data.Accounts
}

func (m *configManager) GetBaseWorkDir() string {
	return m.data.BaseWorkDir
}

func (m *configManager) GetWebhook() *Webhook {
	return m.data.Webhook
}

func (m *configManager) GetAutoDecrypt() bool {
	return m.data.AutoDecrypt
}

func (m *configManager) GetHTTPAddr() string {
	return m.data.HTTPAddr
}

func (m *configManager) GetHTTPEnabled() bool {
	return m.data.HTTPEnabled
}

func (m *configManager) SetHTTPEnabled(enabled bool) error {
	m.data.HTTPEnabled = enabled
	return m.Save()
}

func (m *configManager) SetAutoDecrypt(decrypt bool) error {
	m.data.AutoDecrypt = decrypt
	return m.Save()
}

func (m *configManager) SetAccount(account AccountConfig) error {
	if m.data.Accounts == nil {
		m.data.Accounts = make(map[string]AccountConfig)
	}
	m.data.LastAccount = account.Account
	m.data.Accounts[account.Account] = account
	return m.Save()
}

func (m *configManager) SetBaseWorkDir(workDir string) error {
	m.data.BaseWorkDir = workDir
	return m.Save()
}

func (m *configManager) SetHTTPAddr(addr string) error {
	m.data.HTTPAddr = addr
	return m.Save()
}

func (m *configManager) Save() error {
	return m.cm.Save(m.data)
}
