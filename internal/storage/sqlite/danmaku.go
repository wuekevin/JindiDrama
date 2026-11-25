package sqlite

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
