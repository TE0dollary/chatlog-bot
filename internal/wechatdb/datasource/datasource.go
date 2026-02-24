package datasource

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/model"
	v4 "github.com/TE0dollary/chatlog-bot/internal/wechatdb/datasource/v4"
)

type DataSource interface {

	// messages
	GetMessages(ctx context.Context, startTime, endTime time.Time, talker string, sender string, keyword string, limit, offset int) ([]*model.Message, error)

	// contacts
	GetContacts(ctx context.Context, key string, limit, offset int) ([]*model.Contact, error)

	// chat rooms
	GetChatRooms(ctx context.Context, key string, limit, offset int) ([]*model.ChatRoom, error)

	// sessions
	GetSessions(ctx context.Context, key string, limit, offset int) ([]*model.Session, error)

	// media
	GetMedia(ctx context.Context, _type string, key string) (*model.Media, error)

	// set file-change callback
	SetCallback(group string, callback func(event fsnotify.Event) error) error

	Close() error
}

func New(path string, platform string, version int) (DataSource, error) {
	switch {
	case platform == "darwin" && version == 4:
		return v4.New(path)
	default:
		return nil, errors.PlatformUnsupported(platform, version)
	}
}
