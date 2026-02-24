package model

import (
	"github.com/TE0dollary/chatlog-bot/internal/model/wxproto"

	"google.golang.org/protobuf/proto"
)

type ChatRoom struct {
	Name  string         `json:"name"`
	Owner string         `json:"owner"`
	Users []ChatRoomUser `json:"users"`

	// Extra From Contact
	Remark   string `json:"remark"`
	NickName string `json:"nickName"`

	User2DisplayName map[string]string `json:"-"`
}

type ChatRoomUser struct {
	UserName    string `json:"userName"`
	DisplayName string `json:"displayName"`
}

func ParseRoomData(b []byte) (users []ChatRoomUser) {
	var pbMsg wxproto.RoomData
	if err := proto.Unmarshal(b, &pbMsg); err != nil {
		return
	}
	if pbMsg.Users == nil {
		return
	}

	users = make([]ChatRoomUser, 0, len(pbMsg.Users))
	for _, user := range pbMsg.Users {
		u := ChatRoomUser{UserName: user.UserName}
		if user.DisplayName != nil {
			u.DisplayName = *user.DisplayName
		}
		users = append(users, u)
	}
	return users
}

func (c *ChatRoom) DisplayName() string {
	switch {
	case c.Remark != "":
		return c.Remark
	case c.NickName != "":
		return c.NickName
	}
	return ""
}
