package repository

import (
	"context"
	"strings"
	"time"

	"github.com/TE0dollary/chatlog-bot/internal/model"
	"github.com/TE0dollary/chatlog-bot/pkg/util"

	"github.com/rs/zerolog/log"
)

// GetMessages implements the Repository interface.
func (r *Repository) GetMessages(ctx context.Context, startTime, endTime time.Time, talker string, sender string, keyword string, limit, offset int) ([]*model.Message, error) {

	talker, sender = r.parseTalkerAndSender(ctx, talker, sender)
	messages, err := r.ds.GetMessages(ctx, startTime, endTime, talker, sender, keyword, limit, offset)
	if err != nil {
		return nil, err
	}

	if err := r.EnrichMessages(ctx, messages); err != nil {
		log.Debug().Msgf("EnrichMessages failed: %v", err)
	}

	return messages, nil
}

// EnrichMessages fills in extra fields for a batch of messages.
func (r *Repository) EnrichMessages(ctx context.Context, messages []*model.Message) error {
	for _, msg := range messages {
		r.enrichMessage(msg)
	}
	return nil
}

// enrichMessage fills in extra fields for a single message.
func (r *Repository) enrichMessage(msg *model.Message) {
	if msg.IsChatRoom {
		if chatRoom, ok := r.chatRoomCache[msg.Talker]; ok {
			msg.TalkerName = chatRoom.DisplayName()

			if displayName, ok := chatRoom.User2DisplayName[msg.Sender]; ok {
				msg.SenderName = displayName
			}
		}
	}

	if msg.SenderName == "" && !msg.IsSelf {
		contact := r.getFullContact(msg.Sender)
		if contact != nil {
			msg.SenderName = contact.DisplayName()
		}
	}
}

func (r *Repository) parseTalkerAndSender(ctx context.Context, talker, sender string) (string, string) {
	displayName2User := make(map[string]string)
	users := make(map[string]bool)

	talkers := util.Str2List(talker, ",")
	if len(talkers) > 0 {
		for i := 0; i < len(talkers); i++ {
			if contact, _ := r.GetContact(ctx, talkers[i]); contact != nil {
				talkers[i] = contact.UserName
			} else if chatRoom, _ := r.GetChatRoom(ctx, talker); chatRoom != nil {
				talkers[i] = chatRoom.Name
			}
		}
		for i := 0; i < len(talkers); i++ {
			if chatRoom, _ := r.GetChatRoom(ctx, talkers[i]); chatRoom != nil {
				for user, displayName := range chatRoom.User2DisplayName {
					displayName2User[displayName] = user
				}
				for _, user := range chatRoom.Users {
					users[user.UserName] = true
				}
			}
		}
		talker = strings.Join(talkers, ",")
	}

	senders := util.Str2List(sender, ",")
	if len(senders) > 0 {
		for i := 0; i < len(senders); i++ {
			if user, ok := displayName2User[senders[i]]; ok {
				senders[i] = user
			} else {
				// FIXME many group members share display names; GetContact lookup may be ambiguous
				for user := range users {
					if contact := r.getFullContact(user); contact != nil {
						if contact.DisplayName() == senders[i] {
							senders[i] = user
							break
						}
					}
				}
			}
		}
		sender = strings.Join(senders, ",")
	}

	return talker, sender
}
