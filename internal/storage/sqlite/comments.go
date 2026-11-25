package sqlite

import (
	"database/sql"
	"errors"
)

func (s *Store) LocalComments(dramaID string, episode int) (string, error) {
	var snapshot string
	err := s.db.QueryRow(`SELECT snapshot FROM local_comments WHERE drama_id=? AND episode=?`, dramaID, episode).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return snapshot, err
}

func (s *Store) SaveLocalComments(dramaID string, episode int, snapshot string) error {
	_, err := s.db.Exec(`INSERT INTO local_comments(drama_id,episode,snapshot) VALUES(?,?,?)
ON CONFLICT(drama_id,episode) DO UPDATE SET snapshot=excluded.snapshot`, dramaID, episode, snapshot)
	return err
}
