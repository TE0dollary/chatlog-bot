package v4

import (
	"context"
	"fmt"
	"time"

	"github.com/TE0dollary/chatlog-bot/internal/errors"
	"github.com/TE0dollary/chatlog-bot/internal/model"
	"github.com/TE0dollary/chatlog-bot/internal/wechatdb/datasource/dbm"
)

type SessionModule struct {
	dbm *dbm.DBManager
}

func NewSessionModule(d *dbm.DBManager) *SessionModule {
	return &SessionModule{dbm: d}
}

// GetSessions returns sessions matching key, or all sessions if key is empty.
func (s *SessionModule) GetSessions(ctx context.Context, key string, limit, offset int) ([]*model.Session, error) {
	var query string
	var args []interface{}

	if key != "" {
		query = `SELECT username, summary, last_timestamp, last_msg_sender, last_sender_display_name
				FROM SessionTable
				WHERE username = ? OR last_sender_display_name = ?
				ORDER BY sort_timestamp DESC`
		args = []interface{}{key, key}
	} else {
		query = `SELECT username, summary, last_timestamp, last_msg_sender, last_sender_display_name
				FROM SessionTable
				ORDER BY sort_timestamp DESC`
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
		if offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", offset)
		}
	}

	db, err := s.dbm.GetDB(Session)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.QueryFailed(query, err)
	}
	defer rows.Close()

	sessions := []*model.Session{}
	for rows.Next() {
		var sessionV4 model.SessionV4
		err := rows.Scan(
			&sessionV4.Username,
			&sessionV4.Summary,
			&sessionV4.LastTimestamp,
			&sessionV4.LastMsgSender,
			&sessionV4.LastSenderDisplayName,
		)

		if err != nil {
			return nil, errors.ScanRowFailed(err)
		}

		sessions = append(sessions, sessionV4.Wrap())
	}

	return sessions, nil
}

// GetTalkersAfter returns all session usernames whose last message timestamp is at or after t.
func (s *SessionModule) GetTalkersAfter(ctx context.Context, t time.Time) ([]string, error) {
	db, err := s.dbm.GetDB(Session)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT username FROM SessionTable WHERE last_timestamp >= ?",
		t.Unix(),
	)
	if err != nil {
		return nil, errors.QueryFailed("getTalkersAfter", err)
	}
	defer rows.Close()

	var talkers []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, errors.ScanRowFailed(err)
		}
		talkers = append(talkers, username)
	}
	return talkers, nil
}
