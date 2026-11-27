package sqlite

import (
	"context"
	"errors"
	"os"
)

// WriteBackup writes a compact SQLite snapshot to an exclusive, empty staging file.
func (s *Store) WriteBackup(output *os.File) error {
	return withBackupVFS(output, func(uri string) error {
		return s.compactBackupInto(context.Background(), uri)
	})
}

func (s *Store) compactBackupInto(ctx context.Context, uri string) (result error) {
	source, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer source.Close()
	var prior int
	if err := source.QueryRowContext(ctx, "PRAGMA temp_store").Scan(&prior); err != nil {
		return err
	}
	if prior != 2 {
		if _, err := source.ExecContext(ctx, "PRAGMA temp_store=MEMORY"); err != nil {
			return err
		}
		defer func() {
			setting := "PRAGMA temp_store=DEFAULT"
			if prior == 1 {
				setting = "PRAGMA temp_store=FILE"
			}
			_, err := source.ExecContext(ctx, setting)
			result = errors.Join(result, err)
		}()
	}
	_, result = source.ExecContext(ctx, "VACUUM INTO ?", uri)
	return result
}
