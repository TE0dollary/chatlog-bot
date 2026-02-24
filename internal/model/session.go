package model

import (
	"strings"
	"time"
)

type Session struct {
	UserName string    `json:"userName"`
	NOrder   int       `json:"nOrder"`
	NickName string    `json:"nickName"`
	Content  string    `json:"content"`
	NTime    time.Time `json:"nTime"`
}

func (s *Session) PlainText(limit int) string {
	buf := strings.Builder{}
	buf.WriteString(s.NickName)
	buf.WriteString("(")
	buf.WriteString(s.UserName)
	buf.WriteString(") ")
	buf.WriteString(s.NTime.Format("2006-01-02 15:04:05"))
	buf.WriteString("\n")
	if limit > 0 {
		if len(s.Content) > limit {
			buf.WriteString(s.Content[:limit])
			buf.WriteString(" <...>")
		} else {
			buf.WriteString(s.Content)
		}
	}
	buf.WriteString("\n")
	return buf.String()
}
