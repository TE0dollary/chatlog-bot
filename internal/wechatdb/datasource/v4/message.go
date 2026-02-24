package v4

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/model"
	"github.com/TE0dollary/chatlog-bot/internal/wechatdb/datasource/dbm"
	"github.com/TE0dollary/chatlog-bot/pkg/util"
)

// MessageDBInfo holds metadata for a message database shard.
type MessageDBInfo struct {
	FilePath  string
	StartTime time.Time
	EndTime   time.Time
}

type MessageModule struct {
	dbm          *dbm.DBManager
	messageInfos []MessageDBInfo
}

func NewMessageModule(d *dbm.DBManager) *MessageModule {
	return &MessageModule{
		dbm:          d,
		messageInfos: make([]MessageDBInfo, 0),
	}
}

func (m *MessageModule) initMessageDbs() error {
	dbPaths, err := m.dbm.GetDBPath(Message)
	if err != nil {
		if strings.Contains(err.Error(), "db file not found") {
			m.messageInfos = make([]MessageDBInfo, 0)
			return nil
		}
		return err
	}

	infos := make([]MessageDBInfo, 0)
	for _, filePath := range dbPaths {
		db, err := m.dbm.OpenDB(filePath)
		if err != nil {
			log.Err(err).Msgf("获取数据库 %s 失败", filePath)
			continue
		}

		var startTime time.Time
		var timestamp int64

		row := db.QueryRow("SELECT timestamp FROM Timestamp LIMIT 1")
		if err := row.Scan(&timestamp); err != nil {
			log.Err(err).Msgf("获取数据库 %s 的时间戳失败", filePath)
			continue
		}
		startTime = time.Unix(timestamp, 0)

		infos = append(infos, MessageDBInfo{
			FilePath:  filePath,
			StartTime: startTime,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].StartTime.Before(infos[j].StartTime)
	})

	// set EndTime for each shard
	for i := range infos {
		if i == len(infos)-1 {
			infos[i].EndTime = time.Now().Add(time.Hour)
		} else {
			infos[i].EndTime = infos[i+1].StartTime
		}
	}
	if len(m.messageInfos) > 0 && len(infos) < len(m.messageInfos) {
		log.Warn().Msgf("message db count decreased from %d to %d, skip init", len(m.messageInfos), len(infos))
		return nil
	}
	m.messageInfos = infos
	return nil
}

// getDBInfosForTimeRange returns the database shards that overlap with the given time range.
func (m *MessageModule) getDBInfosForTimeRange(startTime, endTime time.Time) []MessageDBInfo {
	var dbs []MessageDBInfo
	for _, info := range m.messageInfos {
		if info.StartTime.Before(endTime) && info.EndTime.After(startTime) {
			dbs = append(dbs, info)
		}
	}
	return dbs
}

func (m *MessageModule) GetMessages(ctx context.Context, startTime, endTime time.Time, talker string, sender string, keyword string, limit, offset int) ([]*model.Message, error) {
	if talker == "" {
		return nil, errors.ErrTalkerEmpty
	}

	// talker supports comma-separated list
	talkers := util.Str2List(talker, ",")
	if len(talkers) == 0 {
		return nil, errors.ErrTalkerEmpty
	}

	dbInfos := m.getDBInfosForTimeRange(startTime, endTime)
	if len(dbInfos) == 0 {
		return nil, errors.TimeRangeNotFound(startTime, endTime)
	}

	// sender supports comma-separated list
	senders := util.Str2List(sender, ",")

	// pre-compile keyword regex
	var regex *regexp.Regexp
	if keyword != "" {
		var err error
		regex, err = regexp.Compile(keyword)
		if err != nil {
			return nil, errors.QueryFailed("invalid regex pattern", err)
		}
	}

	filteredMessages := []*model.Message{}

	for _, dbInfo := range dbInfos {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		db, err := m.dbm.OpenDB(dbInfo.FilePath)
		if err != nil {
			log.Error().Msgf("数据库 %s 未打开", dbInfo.FilePath)
			continue
		}

		for _, talkerItem := range talkers {
			_talkerMd5Bytes := md5.Sum([]byte(talkerItem))
			talkerMd5 := hex.EncodeToString(_talkerMd5Bytes[:])
			tableName := "Msg_" + talkerMd5

			var exists bool
			err = db.QueryRowContext(ctx,
				"SELECT 1 FROM sqlite_master WHERE type='table' AND name=?",
				tableName).Scan(&exists)

			if err != nil {
				if err == sql.ErrNoRows {
					continue
				}
				return nil, errors.QueryFailed("", err)
			}

			conditions := []string{"create_time >= ? AND create_time <= ?"}
			args := []interface{}{startTime.Unix(), endTime.Unix()}
			log.Debug().Msgf("Table name: %s", tableName)
			log.Debug().Msgf("Start time: %d, End time: %d", startTime.Unix(), endTime.Unix())

			query := fmt.Sprintf(`
				SELECT m.sort_seq, m.server_id, m.local_type, n.user_name, m.create_time, m.message_content, m.packed_info_data, m.status
				FROM %s m
				LEFT JOIN Name2Id n ON m.real_sender_id = n.rowid
				WHERE %s
				ORDER BY m.sort_seq ASC
			`, tableName, strings.Join(conditions, " AND "))

			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				// 如果表不存在，SQLite 会返回错误
				if strings.Contains(err.Error(), "no such table") {
					continue
				}
				log.Err(err).Msgf("从数据库 %s 查询消息失败", dbInfo.FilePath)
				continue
			}

			for rows.Next() {
				var msg model.MessageV4
				err := rows.Scan(
					&msg.SortSeq,
					&msg.ServerID,
					&msg.LocalType,
					&msg.UserName,
					&msg.CreateTime,
					&msg.MessageContent,
					&msg.PackedInfoData,
					&msg.Status,
				)
				if err != nil {
					rows.Close()
					return nil, errors.ScanRowFailed(err)
				}

				message := msg.Wrap(talkerItem)

				// apply sender filter
				if len(senders) > 0 {
					senderMatch := false
					for _, s := range senders {
						if message.Sender == s {
							senderMatch = true
							break
						}
					}
					if !senderMatch {
						continue
					}
				}

				// apply keyword filter
				if regex != nil {
					plainText := message.PlainTextContent()
					if !regex.MatchString(plainText) {
						continue
					}
				}

				filteredMessages = append(filteredMessages, message)

				// early exit when enough messages collected for pagination
				if limit > 0 && len(filteredMessages) >= offset+limit {
					rows.Close()

					sort.Slice(filteredMessages, func(i, j int) bool {
						return filteredMessages[i].Seq < filteredMessages[j].Seq
					})

					if offset >= len(filteredMessages) {
						return []*model.Message{}, nil
					}
					end := offset + limit
					if end > len(filteredMessages) {
						end = len(filteredMessages)
					}
					return filteredMessages[offset:end], nil
				}
			}
			rows.Close()
		}
	}

	sort.Slice(filteredMessages, func(i, j int) bool {
		return filteredMessages[i].Seq < filteredMessages[j].Seq
	})

	if limit > 0 {
		if offset >= len(filteredMessages) {
			return []*model.Message{}, nil
		}
		end := offset + limit
		if end > len(filteredMessages) {
			end = len(filteredMessages)
		}
		return filteredMessages[offset:end], nil
	}

	return filteredMessages, nil
}
