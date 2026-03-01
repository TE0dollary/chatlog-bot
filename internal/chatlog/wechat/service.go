package wechat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/wechat"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/decrypt"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"
	"github.com/TE0dollary/chatlog-bot/pkg/filemonitor"
	"github.com/TE0dollary/chatlog-bot/pkg/util"
)

var (
	// DebounceTime 防抖间隔：ticker 以此为周期，每个周期结束时统一处理 pending 文件
	DebounceTime = 1 * time.Second
)

// Service 封装微信数据解密相关功能：
//   - 密钥提取：从微信进程内存中搜索加密密钥
//   - 数据库解密：使用 SQLCipher 兼容算法解密 .db 文件
//   - 自动解密：通过 fsnotify 监控数据目录，使用防抖机制自动解密变化的文件
type Service struct {
	ctx     Context                  // 上下文接口（由 Context 或 ServerConfig 实现）
	manager *wechat.Manager          // 微信进程管理器
	fm      *filemonitor.FileMonitor // 文件变化监控器

	eventCh chan string        // 文件路径事件 channel，用于解耦回调与处理逻辑
	stopCh  chan struct{}      // 停止信号
	wg      sync.WaitGroup     // 等待 eventLoop goroutine 退出
	sf      singleflight.Group // 确保同一文件同时只有一个解密操作在运行
}

type Context interface {
	GetDataKey() string
	GetDataDir() string
	GetWorkDir() string
	GetPlatform() string
	GetVersion() int
	GetDerivedKeyMap() map[string]string
	HasDecryptKey() bool
}

func NewService(ctx Context) *Service {
	return &Service{
		ctx:     ctx,
		manager: wechat.NewManager(),
	}
}

// Processes returns all running WeChat processes
func (s *Service) Processes() []*model.Process {
	return s.manager.GetProcesses()
}

// ExtractKey 从指定进程提取加密密钥
func (s *Service) ExtractKey(ctx context.Context, proc *model.Process) (string, map[string]string, error) {
	return s.manager.ExtractKey(ctx, proc.Name)
}

func (s *Service) StartAutoDecrypt() error {
	log.Info().Msgf("start auto decrypt, data dir: %s", s.ctx.GetDataDir())
	dbGroup, err := filemonitor.NewFileGroup("wechat", s.ctx.GetDataDir(), `.*\.db$`, []string{"fts"})
	if err != nil {
		return err
	}
	dbGroup.AddCallback(s.DecryptFileCallback)

	s.fm = filemonitor.NewFileMonitor()
	s.fm.AddGroup(dbGroup)
	if err := s.fm.Start(); err != nil {
		log.Debug().Err(err).Msg("failed to start file monitor")
		return err
	}

	s.eventCh = make(chan string, 100)
	s.stopCh = make(chan struct{})
	s.wg.Add(1)
	go s.eventLoop()

	return nil
}

func (s *Service) StopAutoDecrypt() error {
	if s.fm != nil {
		if err := s.fm.Stop(); err != nil {
			return err
		}
		s.fm = nil
	}
	if s.stopCh != nil {
		close(s.stopCh)
		s.wg.Wait()
		s.stopCh = nil
	}
	log.Info().Msg("自动解密已停止")
	return nil
}

// DecryptFileCallback 是文件变化事件的回调函数。
// 仅过滤事件类型并将文件路径发送到 eventCh，由 eventLoop goroutine 统一处理防抖逻辑。
func (s *Service) DecryptFileCallback(event fsnotify.Event) error {
	// Local file system
	// WRITE         "/db_storage/message/message_0.db"
	// WRITE         "/db_storage/message/message_0.db"
	// WRITE|CHMOD   "/db_storage/message/message_0.db"
	// Syncthing
	// REMOVE        "/app/data/db_storage/session/session.db"
	// CREATE        "/app/data/db_storage/session/session.db" ← "/app/data/db_storage/session/.syncthing.session.db.tmp"
	// CHMOD         "/app/data/db_storage/session/session.db"
	if !(event.Op.Has(fsnotify.Write) || event.Op.Has(fsnotify.Create)) {
		return nil
	}
	log.Debug().Str("op", event.Op.String()).Str("file", filepath.Base(event.Name)).Msg("检测到数据库文件变化")
	select {
	case s.eventCh <- event.Name:
	default:
		// channel 已满时丢弃，防抖计时器会在下次 tick 时继续推进
		log.Debug().Str("file", filepath.Base(event.Name)).Msg("事件队列已满，丢弃此次事件")
	}
	return nil
}

// eventLoop 是单一 goroutine 的事件消费循环。
// 独占 pending 状态无需 mutex；ticker 每隔 DebounceTime 统一处理所有 pending 文件，
// ticker 间隔本身即为防抖周期。singleflight 确保同一文件同时只有一个解密操作在运行。
func (s *Service) eventLoop() {
	defer s.wg.Done()

	pending := make(map[string]struct{})
	ticker := time.NewTicker(DebounceTime)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return

		case dbFileName := <-s.eventCh:
			_, alreadyPending := pending[dbFileName]
			pending[dbFileName] = struct{}{}
			if !alreadyPending {
				log.Debug().Str("file", filepath.Base(dbFileName)).Int("pending", len(pending)).Msg("文件加入待解密队列")
			}

		case <-ticker.C:
			if len(pending) == 0 {
				continue
			}
			log.Info().Int("count", len(pending)).Msg("防抖周期结束，开始处理待解密文件")
			for dbFileName := range pending {
				delete(pending, dbFileName)
				f := dbFileName
				go func() {
					_, _, shared := s.sf.Do(f, func() (any, error) {
						return nil, s.DecryptDBFile(f)
					})
					if shared {
						log.Debug().Str("file", filepath.Base(f)).Msg("singleflight: 已有相同文件的解密在进行，共用结果")
					}
				}()
			}
		}
	}
}

// DecryptDBFile 解密单个数据库文件。
// 密钥选择优先级：DerivedKeyMap 中对应路径的独立派生密钥 > 全局 DataKey。
// 输出写入 WorkDir 下对应的子路径，先写 .tmp 再 rename 保证原子性。
func (s *Service) DecryptDBFile(dbFile string) error {

	decryptor, err := decrypt.NewDecryptor(s.ctx.GetPlatform(), s.ctx.GetVersion())
	if err != nil {
		return err
	}

	// 确定用于解密的密钥：优先查 DerivedKeyMap，找不到再回退到全局 DataKey
	encKey := s.ctx.GetDataKey()
	dm := s.ctx.GetDerivedKeyMap()
	if len(dm) > 0 {
		relPath := strings.TrimPrefix(dbFile[len(s.ctx.GetDataDir()):], "/")
		if dk, ok := dm[relPath]; ok && dk != "" {
			encKey = dk
			log.Debug().Str("file", filepath.Base(dbFile)).Str("rel_path", relPath).Msg("使用派生密钥解密")
		} else {
			log.Warn().Str("rel_path", relPath).Msg("无对应派生密钥，跳过")
			return nil
		}
	} else {
		log.Debug().Str("file", filepath.Base(dbFile)).Msg("使用全局 DataKey 解密")
	}

	output := filepath.Join(s.ctx.GetWorkDir(), dbFile[len(s.ctx.GetDataDir()):])
	if err := util.PrepareDir(filepath.Dir(output)); err != nil {
		return err
	}

	outputTemp := output + ".tmp"
	outputFile, err := os.Create(outputTemp)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer func() {
		outputFile.Close()
		if err := os.Rename(outputTemp, output); err != nil {
			log.Debug().Err(err).Msgf("failed to rename %s to %s", outputTemp, output)
		}
	}()

	if err := decryptor.Decrypt(context.Background(), dbFile, encKey, outputFile); err != nil {
		if errors.Is(err, errors.ErrAlreadyDecrypted) {
			log.Debug().Str("file", filepath.Base(dbFile)).Msg("文件已解密，直接复制")
			if data, err := os.ReadFile(dbFile); err == nil {
				outputFile.Write(data)
			}
			return nil
		}
		log.Error().Err(err).Str("file", filepath.Base(dbFile)).Msg("解密失败")
		return err
	}

	log.Info().Str("src", filepath.Base(dbFile)).Str("dst", output).Msg("解密完成")

	return nil
}

// DecryptDBFiles 批量解密数据目录下所有 .db 文件（排除 fts 全文索引目录）。
func (s *Service) DecryptDBFiles() error {
	dbGroup, err := filemonitor.NewFileGroup("wechat", s.ctx.GetDataDir(), `.*\.db$`, []string{"fts"})
	if err != nil {
		return err
	}
	dbFiles, err := dbGroup.List()
	if err != nil {
		return err
	}
	total := len(dbFiles)
	log.Info().Int("total", total).Msg("开始批量解密")
	var failed int
	for _, dbFile := range dbFiles {
		if err := s.DecryptDBFile(dbFile); err != nil {
			log.Debug().Err(err).Str("file", filepath.Base(dbFile)).Msg("解密失败，跳过")
			failed++
			continue
		}
	}
	log.Info().Int("total", total).Int("success", total-failed).Int("failed", failed).Msg("批量解密结束")
	return nil
}
