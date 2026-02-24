package repository

import (
	"context"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/model"
	"github.com/TE0dollary/chatlog-bot/internal/wechatdb/datasource"
)

// Repository implements the repository interface.
type Repository struct {
	ds datasource.DataSource

	// Cache for contact
	contactCache      map[string]*model.Contact
	aliasToContact    map[string][]*model.Contact
	remarkToContact   map[string][]*model.Contact
	nickNameToContact map[string][]*model.Contact
	chatRoomInContact map[string]*model.Contact
	contactList       []string
	aliasList         []string
	remarkList        []string
	nickNameList      []string

	// Cache for chat room
	chatRoomCache      map[string]*model.ChatRoom
	remarkToChatRoom   map[string][]*model.ChatRoom
	nickNameToChatRoom map[string][]*model.ChatRoom
	chatRoomList       []string
	chatRoomRemark     []string
	chatRoomNickName   []string

	// index: chatroom user -> contact
	chatRoomUserToInfo map[string]*model.Contact
}

// New creates a new Repository backed by the given DataSource.
func New(ds datasource.DataSource) (*Repository, error) {
	r := &Repository{
		ds:                 ds,
		contactCache:       make(map[string]*model.Contact),
		aliasToContact:     make(map[string][]*model.Contact),
		remarkToContact:    make(map[string][]*model.Contact),
		nickNameToContact:  make(map[string][]*model.Contact),
		chatRoomUserToInfo: make(map[string]*model.Contact),
		contactList:        make([]string, 0),
		aliasList:          make([]string, 0),
		remarkList:         make([]string, 0),
		nickNameList:       make([]string, 0),
		chatRoomCache:      make(map[string]*model.ChatRoom),
		remarkToChatRoom:   make(map[string][]*model.ChatRoom),
		nickNameToChatRoom: make(map[string][]*model.ChatRoom),
		chatRoomList:       make([]string, 0),
		chatRoomRemark:     make([]string, 0),
		chatRoomNickName:   make([]string, 0),
	}

	if err := r.initCache(context.Background()); err != nil {
		return nil, errors.InitCacheFailed(err)
	}

	ds.SetCallback("contact", r.contactCallback)
	ds.SetCallback("chatroom", r.chatroomCallback)

	return r, nil
}

// initCache initializes all in-memory caches.
func (r *Repository) initCache(ctx context.Context) error {
	if err := r.initContactCache(ctx); err != nil {
		return err
	}

	if err := r.initChatRoomCache(ctx); err != nil {
		return err
	}

	return nil
}

func (r *Repository) contactCallback(event fsnotify.Event) error {
	if !event.Op.Has(fsnotify.Create) {
		return nil
	}
	if err := r.initContactCache(context.Background()); err != nil {
		log.Err(err).Msgf("Failed to reinitialize contact cache: %s", event.Name)
	}
	return nil
}

func (r *Repository) chatroomCallback(event fsnotify.Event) error {
	if !event.Op.Has(fsnotify.Create) {
		return nil
	}
	if err := r.initChatRoomCache(context.Background()); err != nil {
		log.Err(err).Msgf("Failed to reinitialize contact cache: %s", event.Name)
	}
	return nil
}

func (r *Repository) SetCallback(group string, callback func(event fsnotify.Event) error) error {
	return r.ds.SetCallback(group, callback)
}

// Close releases underlying datasource resources.
func (r *Repository) Close() error {
	return r.ds.Close()
}
