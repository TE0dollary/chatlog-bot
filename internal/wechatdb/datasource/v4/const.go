package v4

import "github.com/TE0dollary/chatlog-bot/internal/wechatdb/datasource/dbm"

const (
	Message = "message"
	Contact = "contact"
	Session = "session"
	Media   = "media"
	Voice   = "voice"
)

var Groups = []*dbm.Group{
	{
		Name:      Message,
		Pattern:   `^message_([0-9]?[0-9])?\.db$`,
		BlackList: []string{},
	},
	{
		Name:      Contact,
		Pattern:   `^contact\.db$`,
		BlackList: []string{},
	},
	{
		Name:      Session,
		Pattern:   `session\.db$`,
		BlackList: []string{},
	},
	{
		Name:      Media,
		Pattern:   `^hardlink\.db$`,
		BlackList: []string{},
	},
	{
		Name:      Voice,
		Pattern:   `^media_([0-9]?[0-9])?\.db$`,
		BlackList: []string{},
	},
}
