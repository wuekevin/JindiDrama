package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// VerifyDesktopDatabase checks the profile produced by a desktop smoke run.
// The app must have exited before this check starts. Copying SQLite's files
// lets the verifier account for committed WAL data without touching the profile.
func VerifyDesktopDatabase(path string) error {
	snapshot, err := os.MkdirTemp("", "jindi-sqlite-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(snapshot)
	copyPath := filepath.Join(snapshot, "jindi.db")
	if err := copyDatabaseFile(path, copyPath); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-journal"} {
		if err := copyDatabaseFile(path+suffix, copyPath+suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	// SQLite can rebuild the volatile -shm index in the temporary directory.
	db, err := openDatabase(copyPath, "ro")
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA trusted_schema=OFF"); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var integrity string
	if err := tx.QueryRow("PRAGMA integrity_check(1)").Scan(&integrity); err != nil {
		return fmt.Errorf("desktop SQLite integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("desktop SQLite integrity check failed: %.160s", integrity)
	}
	version, err := supportedVersion(tx)
	if err != nil {
		return fmt.Errorf("desktop schema migration history: %w", err)
	}
	if want := len(migrations); version != want {
		return fmt.Errorf("desktop schema version v%d, expected v%d", version, want)
	}
	if err := verifyDesktopBusinessSchema(tx); err != nil {
		return err
	}

	// Compare against the schema built from the actual migrations. This
	// covers required columns, constraints and explicit indexes without
	// maintaining a second copy of the application's schema.
	expected, err := Open(":memory:")
	if err != nil {
		return fmt.Errorf("build expected desktop schema: %w", err)
	}
	defer expected.Close()
	rows, err := expected.db.Query(`
SELECT type, name, sql FROM sqlite_schema
WHERE sql IS NOT NULL AND name NOT GLOB 'sqlite_*' ORDER BY name`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, name, want string
		if err := rows.Scan(&kind, &name, &want); err != nil {
			return err
		}
		var actual string
		err := tx.QueryRow("SELECT sql FROM sqlite_schema WHERE type=? AND name=?", kind, name).Scan(&actual)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && actual != want) {
			return fmt.Errorf("desktop database schema mismatch for %s %s", kind, name)
		}
		if err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyDatabaseFile(source, target string) error {
	from, err := os.Open(source)
	if err != nil {
		return err
	}
	defer from.Close()
	info, err := from.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("desktop database inputs must be regular files")
	}
	to, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(to, from)
	return errors.Join(err, to.Close())
}

func verifyDesktopBusinessSchema(tx *sql.Tx) error {
	// This read contract stays independent of the migrations. A migration
	// cannot remove an entity used by the app and its own acceptance check.
	rows, err := tx.Query(`
SELECT f.drama_id, f.sort_order, h.drama_id, h.episode, h.position_ms, h.duration_ms, h.percent, h.episode_count_at_watch,
       s.query, s.searched_at, cfg.key, cfg.value_json, c.drama_id, c.episode, c.snapshot, c.write_id,
       d.drama_id, d.episode, d.offset_ms, d.text, r.writer, r.revision
FROM favorites f, history h, search_history s, settings cfg,
     local_comments c, local_danmaku d, library_receipts r
LIMIT 0`)
	if err != nil {
		return fmt.Errorf("desktop database is missing required business tables or columns: %w", err)
	}
	return rows.Close()
}
