package v4

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/model"
	"github.com/TE0dollary/chatlog-bot/internal/wechatdb/datasource/dbm"
)

type MediaModule struct {
	dbm *dbm.DBManager
}

func NewMediaModule(d *dbm.DBManager) *MediaModule {
	return &MediaModule{dbm: d}
}

func (m *MediaModule) isExist(_db string, table string) bool {
	db, err := m.dbm.GetDB(_db)
	if err != nil {
		return false
	}
	var tableName string
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?;"
	if err = db.QueryRow(query, table).Scan(&tableName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		return false
	}
	return true
}

func (m *MediaModule) GetMedia(ctx context.Context, _type string, key string) (*model.Media, error) {
	if key == "" {
		return nil, errors.ErrKeyEmpty
	}

	var table string
	switch _type {
	case "image":
		table = "image_hardlink_info_v3"
		// v4 table used since 4.1.0
		if !m.isExist(Media, table) {
			table = "image_hardlink_info_v4"
		}
	case "video":
		table = "video_hardlink_info_v3"
		if !m.isExist(Media, table) {
			table = "video_hardlink_info_v4"
		}
	case "file":
		table = "file_hardlink_info_v3"
		if !m.isExist(Media, table) {
			table = "file_hardlink_info_v4"
		}
	case "voice":
		return m.GetVoice(ctx, key)
	default:
		return nil, errors.MediaTypeUnsupported(_type)
	}

	query := fmt.Sprintf(`
	SELECT
		f.md5,
		f.file_name,
		f.file_size,
		f.modify_time,
		IFNULL(d1.username,""),
		IFNULL(d2.username,"")
	FROM
		%s f
	LEFT JOIN
		dir2id d1 ON d1.rowid = f.dir1
	LEFT JOIN
		dir2id d2 ON d2.rowid = f.dir2
	`, table)
	query += " WHERE f.md5 = ? OR f.file_name LIKE ? || '%'"
	args := []interface{}{key, key}

	db, err := m.dbm.GetDB(Media)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.QueryFailed(query, err)
	}
	defer rows.Close()

	var media *model.Media
	for rows.Next() {
		var mediaV4 model.MediaV4
		err := rows.Scan(
			&mediaV4.Key,
			&mediaV4.Name,
			&mediaV4.Size,
			&mediaV4.ModifyTime,
			&mediaV4.Dir1,
			&mediaV4.Dir2,
		)
		if err != nil {
			return nil, errors.ScanRowFailed(err)
		}
		mediaV4.Type = _type
		media = mediaV4.Wrap()

		// prefer HD image if available
		if _type == "image" && strings.HasSuffix(mediaV4.Name, "_h.dat") {
			break
		}
	}

	if media == nil {
		return nil, errors.ErrMediaNotFound
	}

	return media, nil
}

func (m *MediaModule) GetVoice(ctx context.Context, key string) (*model.Media, error) {
	if key == "" {
		return nil, errors.ErrKeyEmpty
	}

	query := `
	SELECT voice_data
	FROM VoiceInfo
	WHERE svr_id = ?
	`
	args := []interface{}{key}

	dbs, err := m.dbm.GetDBs(Voice)
	if err != nil {
		return nil, errors.DBConnectFailed("", err)
	}

	for _, db := range dbs {
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, errors.QueryFailed(query, err)
		}
		defer rows.Close()

		for rows.Next() {
			var voiceData []byte
			err := rows.Scan(
				&voiceData,
			)
			if err != nil {
				return nil, errors.ScanRowFailed(err)
			}
			if len(voiceData) > 0 {
				return &model.Media{
					Type: "voice",
					Key:  key,
					Data: voiceData,
				}, nil
			}
		}
	}

	return nil, errors.ErrMediaNotFound
}
