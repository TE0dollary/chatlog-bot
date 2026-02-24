package model

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/TE0dollary/chatlog-bot/internal/model/wxproto"
	"github.com/TE0dollary/chatlog-bot/pkg/util/zstd"
	"google.golang.org/protobuf/proto"
)

// CREATE TABLE Msg_md5(talker)(
// local_id INTEGER PRIMARY KEY AUTOINCREMENT,
// server_id INTEGER,
// local_type INTEGER,
// sort_seq INTEGER,
// real_sender_id INTEGER,
// create_time INTEGER,
// status INTEGER,
// upload_status INTEGER,
// download_status INTEGER,
// server_seq INTEGER,
// origin_source INTEGER,
// source TEXT,
// message_content TEXT,
// compress_content TEXT,
// packed_info_data BLOB,
// WCDB_CT_message_content INTEGER DEFAULT NULL,
// WCDB_CT_source INTEGER DEFAULT NULL
// )
type MessageV4 struct {
	SortSeq        int64  `json:"sort_seq"`         // 10-digit timestamp + 3-digit sequence
	ServerID       int64  `json:"server_id"`        // message ID, used to link voice data
	LocalType      int64  `json:"local_type"`       // message type
	UserName       string `json:"user_name"`        // sender, joined from Name2Id table
	CreateTime     int64  `json:"create_time"`      // unix timestamp (seconds)
	MessageContent []byte `json:"message_content"`  // plain text or zstd-compressed content
	PackedInfoData []byte `json:"packed_info_data"` // extra proto-like data (v4 format)
	Status         int    `json:"status"`           // 2=sent, 4=received; used to infer IsSelf (FIXME: use UserName instead)
}

func (m *MessageV4) Wrap(talker string) *Message {

	_m := &Message{
		Seq:        m.SortSeq,
		Time:       time.Unix(m.CreateTime, 0),
		Talker:     talker,
		IsChatRoom: strings.HasSuffix(talker, "@chatroom"),
		Sender:     m.UserName,
		Type:       m.LocalType,
		Contents:   make(map[string]interface{}),
		Version:    WeChatV4,
	}

	// FIXME: use UserName for IsSelf detection; current status-based heuristic may be inaccurate
	_m.IsSelf = m.Status == 2 || (!_m.IsChatRoom && talker != m.UserName)

	content := ""
	if bytes.HasPrefix(m.MessageContent, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		if b, err := zstd.Decompress(m.MessageContent); err == nil {
			content = string(b)
		}
	} else {
		content = string(m.MessageContent)
	}

	if _m.IsChatRoom {
		split := strings.SplitN(content, ":\n", 2)
		if len(split) == 2 {
			_m.Sender = split[0]
			content = split[1]
		}
	}

	_m.ParseMediaInfo(content)

	// 语音消息
	if _m.Type == 34 {
		_m.Contents["voice"] = fmt.Sprint(m.ServerID)
	}

	if len(m.PackedInfoData) != 0 {
		if packedInfo := ParsePackedInfo(m.PackedInfoData); packedInfo != nil {
			// FIXME 尝试解决 v4 版本 xml 数据无法匹配到 hardlink 记录的问题
			if _m.Type == 3 && packedInfo.Image != nil {
				_talkerMd5Bytes := md5.Sum([]byte(talker))
				talkerMd5 := hex.EncodeToString(_talkerMd5Bytes[:])
				_m.Contents["path"] = filepath.Join("msg", "attach", talkerMd5, _m.Time.Format("2006-01"), "Img", packedInfo.Image.Md5)
			}
			if _m.Type == 43 && packedInfo.Video != nil {
				_m.Contents["path"] = filepath.Join("msg", "video", _m.Time.Format("2006-01"), packedInfo.Video.Md5)
			}
		}
	}

	return _m
}

func ParsePackedInfo(b []byte) *wxproto.PackedInfo {
	var pbMsg wxproto.PackedInfo
	if err := proto.Unmarshal(b, &pbMsg); err != nil {
		return nil
	}
	return &pbMsg
}
