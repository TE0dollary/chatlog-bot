package database

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/chatlog/ctx"
	"github.com/TE0dollary/chatlog-bot/internal/chatlog/webhook"
	"github.com/TE0dollary/chatlog-bot/internal/model"
	"github.com/TE0dollary/chatlog-bot/internal/wechatdb"
)

// 数据库服务状态
const (
	StateInit       = iota // 初始状态，尚未启动
	StateDecrypting        // 正在解密数据库
	StateReady             // 数据库已就绪，可以查询
	StateError             // 出错状态
)

type Config interface {
	GetWorkDir() string
	GetPlatform() string
	GetVersion() int
	GetWebhook() *ctx.Webhook
}

// Service 是数据库访问层，封装了 wechatdb.DB 提供的查询能力。
// 主要职责：
//   - 管理解密后的 SQLite 数据库连接（通过 wechatdb.DB）
//   - 提供消息、联系人、群聊、会话、媒体等数据的查询接口
//   - 管理 Webhook 回调（监听数据库文件变化，推送新消息到外部 URL）
type Service struct {
	State         int                // 当前状态（Init/Decrypting/Ready/Error）
	StateMsg      string             // 错误消息（StateError 时使用）
	conf          Config             // 配置接口
	db            *wechatdb.DB       // 数据库访问对象（管理多个 SQLite 文件）
	webhook       *webhook.Service   // Webhook 服务
	webhookCancel context.CancelFunc // 用于取消 Webhook 的 context
}

func NewService(conf Config) *Service {
	return &Service{
		conf:    conf,
		webhook: webhook.New(conf),
	}
}

// Start 初始化数据库连接，打开 WorkDir 下所有解密后的 SQLite 文件，并启动 Webhook 监听。
func (s *Service) Start() error {
	db, err := wechatdb.New(s.conf.GetWorkDir(), s.conf.GetPlatform(), s.conf.GetVersion())
	if err != nil {
		return err
	}
	s.SetReady()
	s.db = db
	_ = s.initWebhook()
	return nil
}

func (s *Service) Stop() error {
	if s.db != nil {
		s.db.Close()
	}
	s.SetInit()
	s.db = nil
	if s.webhookCancel != nil {
		s.webhookCancel()
		s.webhookCancel = nil
	}
	return nil
}

func (s *Service) SetInit() {
	s.State = StateInit
}

func (s *Service) SetDecrypting() {
	s.State = StateDecrypting
}

func (s *Service) SetReady() {
	s.State = StateReady
}

func (s *Service) SetError(msg string) {
	s.State = StateError
	s.StateMsg = msg
}

func (s *Service) GetDB() *wechatdb.DB {
	return s.db
}

func (s *Service) GetMessages(start, end time.Time, talker string, sender string, keyword string, limit, offset int) ([]*model.Message, error) {
	return s.db.GetMessages(start, end, talker, sender, keyword, limit, offset)
}

func (s *Service) GetContacts(key string, limit, offset int) (*wechatdb.GetContactsResp, error) {
	return s.db.GetContacts(key, limit, offset)
}

func (s *Service) GetChatRooms(key string, limit, offset int) (*wechatdb.GetChatRoomsResp, error) {
	return s.db.GetChatRooms(key, limit, offset)
}

// GetSession retrieves session information
func (s *Service) GetSessions(key string, limit, offset int) (*wechatdb.GetSessionsResp, error) {
	return s.db.GetSessions(key, limit, offset)
}

func (s *Service) GetMedia(_type string, key string) (*model.Media, error) {
	return s.db.GetMedia(_type, key)
}

// initWebhook 根据配置中的 Webhook 规则，为对应的数据库文件组注册变化回调。
// 当数据库文件变化时，查询新消息并按规则过滤后 POST 到配置的 URL。
func (s *Service) initWebhook() error {
	if s.webhook == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.webhookCancel = cancel
	hooks := s.webhook.GetHooks(ctx, s.db)
	for _, hook := range hooks {
		log.Info().Msgf("set callback %#v", hook)
		if err := s.db.SetCallback(hook.Group(), hook.Callback); err != nil {
			log.Error().Err(err).Msgf("set callback %#v failed", hook)
			return err
		}
		hook.Trigger() // 启动时强制触发一次，处理离线期间的消息
	}
	return nil
}

