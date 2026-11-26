package sqlite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type LocalDanmaku struct {
	ID       int64  `json:"id"`
	OffsetMS int64  `json:"offsetMs"`
	Text     string `json:"text"`
}

func (s *Store) LocalDanmaku(dramaID string, episode int) ([]LocalDanmaku, error) {
	rows, err := s.db.Query(`SELECT id, offset_ms, text FROM local_danmaku WHERE drama_id=? AND episode=? ORDER BY id`, dramaID, episode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]LocalDanmaku, 0)
	for rows.Next() {
		var item LocalDanmaku
		if err := rows.Scan(&item.ID, &item.OffsetMS, &item.Text); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) AddLocalDanmaku(dramaID string, episode int, offsetMS int64, text string) (LocalDanmaku, error) {
	result, err := s.db.Exec(`INSERT INTO local_danmaku(drama_id,episode,offset_ms,text) VALUES(?,?,?,?)`, dramaID, episode, offsetMS, text)
	if err != nil {
		return LocalDanmaku{}, err
	}
	id, err := result.LastInsertId()
	return LocalDanmaku{ID: id, OffsetMS: offsetMS, Text: text}, err
}

// AddLocalDanmakuOnce returns the original row when the same request is
// retried. A request identifier can never be reassigned to another payload.
func (s *Store) AddLocalDanmakuOnce(dramaID string, episode int, offsetMS int64, text, requestID string) (LocalDanmaku, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return LocalDanmaku{}, err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO local_danmaku(drama_id,episode,offset_ms,text,request_id) VALUES(?,?,?,?,?)
ON CONFLICT(request_id) WHERE request_id IS NOT NULL DO NOTHING`, dramaID, episode, offsetMS, text, requestID)
	if err != nil {
		return LocalDanmaku{}, err
	}
	var saved LocalDanmaku
	var savedDrama string
	var savedEpisode int
	err = tx.QueryRow(`SELECT id,drama_id,episode,offset_ms,text FROM local_danmaku WHERE request_id=?`, requestID).
		Scan(&saved.ID, &savedDrama, &savedEpisode, &saved.OffsetMS, &saved.Text)
	if err != nil {
		return LocalDanmaku{}, err
	}
	if savedDrama != dramaID || savedEpisode != episode || saved.OffsetMS != offsetMS || saved.Text != text {
		return LocalDanmaku{}, errors.New("弹幕请求已用于其他内容")
	}
	if err := tx.Commit(); err != nil {
		return LocalDanmaku{}, err
	}
	return saved, nil
}

// DanmakuProfile scopes a WebView journal to both the opened path and the
// database's own identity. An unrelated database at the same path stays separate.
const danmakuDatabaseIDKey = "internal_danmaku_database_id"

func (s *Store) DanmakuProfile() (string, error) {
	path := s.CommentProfile()
	value, exists, err := s.GetSetting(danmakuDatabaseIDKey)
	if err != nil {
		return "", err
	}
	var identity string
	if path == "" || !exists || json.Unmarshal([]byte(value), &identity) != nil {
		return "", errors.New("本地弹幕暂存配置不可用")
	}
	data, err := hex.DecodeString(identity)
	if err != nil || len(data) != 16 {
		return "", errors.New("本地弹幕暂存配置无效")
	}
	digest := sha256.Sum256([]byte(path + ":" + identity))
	return hex.EncodeToString(digest[:]), nil
}
