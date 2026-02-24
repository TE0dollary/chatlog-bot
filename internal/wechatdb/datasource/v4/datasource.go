package v4

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/model"
	"github.com/TE0dollary/chatlog-bot/internal/wechatdb/datasource/dbm"
)

type DataSource struct {
	path    string
	dbm     *dbm.DBManager
	message *MessageModule
	contact *ContactModule
	session *SessionModule
	media   *MediaModule
}

func New(path string) (*DataSource, error) {
	d := dbm.NewDBManager(path)
	for _, g := range Groups {
		_ = d.AddGroup(g)
	}
	if err := d.Start(); err != nil {
		return nil, err
	}

	message := NewMessageModule(d)
	if err := message.initMessageDbs(); err != nil {
		return nil, errors.DBInitFailed(err)
	}

	_ = d.AddCallback(Message, func(event fsnotify.Event) error {
		if !event.Op.Has(fsnotify.Create) {
			return nil
		}
		if err := message.initMessageDbs(); err != nil {
			log.Err(err).Msgf("Failed to reinitialize message DBs: %s", event.Name)
		}
		return nil
	})

	return &DataSource{
		path:    path,
		dbm:     d,
		message: message,
		contact: NewContactModule(d),
		session: NewSessionModule(d),
		media:   NewMediaModule(d),
	}, nil
}

func (ds *DataSource) SetCallback(group string, callback func(event fsnotify.Event) error) error {
	if group == "chatroom" {
		group = Contact
	}
	return ds.dbm.AddCallback(group, callback)
}

func (ds *DataSource) GetMessages(ctx context.Context, startTime, endTime time.Time, talker string, sender string, keyword string, limit, offset int) ([]*model.Message, error) {
	return ds.message.GetMessages(ctx, startTime, endTime, talker, sender, keyword, limit, offset)
}

func (ds *DataSource) GetContacts(ctx context.Context, key string, limit, offset int) ([]*model.Contact, error) {
	return ds.contact.GetContacts(ctx, key, limit, offset)
}

func (ds *DataSource) GetChatRooms(ctx context.Context, key string, limit, offset int) ([]*model.ChatRoom, error) {
	return ds.contact.GetChatRooms(ctx, key, limit, offset)
}

func (ds *DataSource) GetSessions(ctx context.Context, key string, limit, offset int) ([]*model.Session, error) {
	return ds.session.GetSessions(ctx, key, limit, offset)
}

func (ds *DataSource) GetMedia(ctx context.Context, _type string, key string) (*model.Media, error) {
	return ds.media.GetMedia(ctx, _type, key)
}

func (ds *DataSource) GetVoice(ctx context.Context, key string) (*model.Media, error) {
	return ds.media.GetVoice(ctx, key)
}

func (ds *DataSource) Close() error {
	return ds.dbm.Close()
}
