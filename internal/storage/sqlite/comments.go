package sqlite

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
)

type LocalCommentState struct {
	Snapshot string `json:"snapshot"`
	Revision string `json:"revision"`
	WriteID  string `json:"writeId"`
}

func (s *Store) LocalComments(dramaID string, episode int) (string, bool, error) {
	state, err := s.LocalCommentsState(dramaID, episode)
	return state.Snapshot, state.Revision != "", err
}

func readLocalComments(row *sql.Row) (LocalCommentState, error) {
	var state LocalCommentState
	err := row.Scan(&state.Snapshot, &state.WriteID)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return LocalCommentState{}, err
	}
	// Include the database bytes so a change by another client is visible
	// even when it does not update the application write identifier.
	state.Revision = commentRevision(state.WriteID, state.Snapshot)
	return state, nil
}

func commentRevision(writeID, snapshot string) string {
	digest := sha256.Sum256([]byte(writeID + "\x00" + snapshot))
	return hex.EncodeToString(digest[:])
}

const selectLocalComments = `SELECT snapshot, write_id FROM local_comments WHERE drama_id=? AND episode=?`

func (s *Store) LocalCommentsState(dramaID string, episode int) (LocalCommentState, error) {
	return readLocalComments(s.db.QueryRow(selectLocalComments, dramaID, episode))
}

func (s *Store) SaveLocalComments(dramaID string, episode int, snapshot string) error {
	_, err := s.db.Exec(`INSERT INTO local_comments(drama_id,episode,snapshot,write_id) VALUES(?,?,?,lower(hex(randomblob(16))))
ON CONFLICT(drama_id,episode) DO UPDATE SET snapshot=excluded.snapshot,write_id=excluded.write_id`, dramaID, episode, snapshot)
	return err
}

// CompareAndSaveLocalComments checks the revision and writes in the same
// transaction. Repeating an acknowledged write identifier is idempotent.
func (s *Store) CompareAndSaveLocalComments(dramaID string, episode int, expected, snapshot, writeID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	current, err := readLocalComments(tx.QueryRow(selectLocalComments, dramaID, episode))
	if err != nil {
		return "", err
	}
	if current.WriteID == writeID {
		if current.Snapshot == snapshot {
			return current.Revision, nil
		}
		return "", nil
	}
	if current.Revision != expected {
		return "", nil
	}
	_, err = tx.Exec(`INSERT INTO local_comments(drama_id,episode,snapshot,write_id) VALUES(?,?,?,?)
ON CONFLICT(drama_id,episode) DO UPDATE SET snapshot=excluded.snapshot,write_id=excluded.write_id`, dramaID, episode, snapshot, writeID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return commentRevision(writeID, snapshot), nil
}

// CommentProfile returns an opaque scope for the database connection's fixed path.
func (s *Store) CommentProfile() string {
	if s.canonicalPath == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(s.canonicalPath))
	return hex.EncodeToString(digest[:])
}
