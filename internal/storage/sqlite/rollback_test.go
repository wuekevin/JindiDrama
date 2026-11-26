package sqlite

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestOpenRejectsFutureSchemaWithoutChangingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO schema_migrations VALUES(?,0)", len(migrations)+1); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(len(migrations)); err == nil || !strings.Contains(err.Error(), "高于此应用支持") {
		t.Fatalf("Migrate should report incompatible schema: %v", err)
	}
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode=DELETE").Scan(&mode); err != nil || mode != "delete" {
		t.Fatalf("journal mode = %q: %v", mode, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("v%d 高于", len(migrations)+1)) {
		t.Fatalf("Open should report incompatible schema: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("Open modified a future database")
	}
	if _, err := os.Stat(path + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("incompatible database journal changed: %v", err)
	}
}

func TestMigrateRollsBackAllChangesOnIntermediateFailure(t *testing.T) {
	s := open(t)
	if _, err := s.db.Exec("DROP TABLE local_comments"); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(2); err == nil {
		t.Fatal("missing v4 table should fail downgrade")
	}
	version, err := s.Version()
	if err != nil || version != len(migrations) {
		t.Fatalf("partial rollback left version v%d: %v", version, err)
	}
	if _, err := s.db.Exec("SELECT revision FROM library_receipts"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("SELECT sort_order FROM favorites"); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareRollbackCopyIncludesWALAndPreservesLegacyOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user #1%.db")
	source, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := source.SetFavorites([]string{"d2", "d1", "d3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := source.db.Exec("UPDATE favorites SET sort_order=CASE drama_id WHEN 'd1' THEN 10 WHEN 'd3' THEN 9 ELSE 8 END"); err != nil {
		t.Fatal(err)
	}
	if err := source.RecordHistory(HistoryEntry{DramaID: "d1", Episode: 3, PositionMs: 1700, DurationMs: 10000}); err != nil {
		t.Fatal(err)
	}
	if err := source.SetSearchHistory([]string{"短剧"}); err != nil {
		t.Fatal(err)
	}
	if err := source.SetSetting("app", `{"theme":"dark"}`); err != nil {
		t.Fatal(err)
	}
	wal, err := os.ReadFile(path + "-wal")
	if err != nil || len(wal) == 0 {
		t.Fatalf("fixture must have committed WAL: %v", err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	outputDir := dir
	if runtime.GOOS != "windows" {
		outputDir = filepath.Join(dir, "backup?directory")
		if err := os.Mkdir(outputDir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(outputDir, "older app.db")
	input := path
	if runtime.GOOS != "windows" {
		input = filepath.Join(dir, "active-link.db")
		if err := os.Symlink(path, input); err != nil {
			t.Fatal(err)
		}
	}
	if err := PrepareRollbackCopy(input, target, 2); err != nil {
		t.Fatal(err)
	}
	db, err := openDatabase(target, "ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	version, err := schemaVersion(db)
	if err != nil || version != 2 {
		t.Fatalf("output schema version = %d, %v", version, err)
	}
	rows, err := db.Query("SELECT drama_id FROM favorites WHERE account_scope='local' ORDER BY created_at DESC, id DESC")
	if err != nil {
		t.Fatal(err)
	}
	var favorites []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		favorites = append(favorites, id)
	}
	rows.Close()
	if !slices.Equal(favorites, []string{"d1", "d3", "d2"}) {
		t.Fatalf("v2 favorites order = %v", favorites)
	}
	var position int
	var query, setting string
	for stmt, target := range map[string]any{
		"SELECT position_ms FROM history WHERE drama_id='d1'": &position,
		"SELECT query FROM search_history":                    &query,
		"SELECT value_json FROM settings WHERE key='app'":     &setting,
	} {
		if err := db.QueryRow(stmt).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if position != 1700 || query != "短剧" || setting != `{"theme":"dark"}` {
		t.Fatalf("older data = %d %q %q", position, query, setting)
	}
	for file, before := range map[string][]byte{path: original, path + "-wal": wal} {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("rollback changed source file %s: %v", file, err)
		}
	}
	if err := PrepareRollbackCopy(path, target, 2); err == nil {
		t.Fatal("must not overwrite existing output")
	}
	if err := PrepareRollbackCopy(path, path+"-journal", 2); err == nil {
		t.Fatal("must not write to a SQLite sidecar")
	}
	version, err = source.Version()
	if err != nil || version != len(migrations) {
		t.Fatalf("source version changed to %d: %v", version, err)
	}
}

func TestPrepareRollbackCopyDoesNotPublishInvalidInput(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.db")
	if err := os.WriteFile(bad, []byte("not SQLite"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{bad, filepath.Join(dir, "missing.db")} {
		output := filepath.Join(dir, "not-created.db")
		if err := PrepareRollbackCopy(input, output, 2); err == nil {
			t.Fatal("missing or corrupt input must fail")
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatalf("must not publish an invalid output: %v", err)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary staging was not removed: %v, %v", files, err)
	}
}

func TestOpenRejectsFutureSchemaInUncheckpointedWAL(t *testing.T) {
	live := open(t)
	if _, err := live.db.Exec("INSERT INTO schema_migrations VALUES(?,0)", len(migrations)+1); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "unclean-exit.db")
	images := make(map[string][]byte)
	for _, suffix := range []string{"", "-wal"} {
		data, err := os.ReadFile(live.Path + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+suffix, data, 0o600); err != nil {
			t.Fatal(err)
		}
		images[path+suffix] = data
	}
	_, err := Open(path)
	var unsupported *UnsupportedSchemaError
	if !errors.As(err, &unsupported) || unsupported.Found != len(migrations)+1 {
		t.Fatalf("future schema in WAL not identified: %v", err)
	}
	for file, before := range images {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("incompatible file changed: %s: %v", file, err)
		}
	}
}

func TestPrepareRollbackCopyRefusesDestinationWithExistingSidecar(t *testing.T) {
	source := open(t)
	output := filepath.Join(t.TempDir(), "older.db")
	sidecar := []byte("existing rollback journal")
	if err := os.WriteFile(output+"-journal", sidecar, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := PrepareRollbackCopy(source.Path, output, 2); err == nil {
		t.Fatal("rollback should not attach an unrelated journal")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("unexpected output: %v", err)
	}
	got, err := os.ReadFile(output + "-journal")
	if err != nil || !bytes.Equal(got, sidecar) {
		t.Fatalf("existing sidecar modified: %v", err)
	}
}

func TestConcurrentOpenCanMigrateSameOlderDatabase(t *testing.T) {
	source := open(t)
	if err := source.Migrate(2); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	const count = 5
	start := make(chan struct{})
	results := make(chan error, count)
	for range count {
		go func() {
			<-start
			store, err := Open(source.Path)
			if err == nil {
				err = store.Close()
			}
			results <- err
		}()
	}
	close(start)
	for range count {
		if err := <-results; err != nil {
			t.Errorf("concurrent startup failed: %v", err)
		}
	}
}

func TestPrepareRollbackCopyDoesNotCreateClosedSourceSidecars(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "closed.db")
	store, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetFavorites([]string{"d2", "d1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	checkSidecars := func() {
		for _, suffix := range []string{"-wal", "-shm", "-journal"} {
			if _, err := os.Lstat(source + suffix); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected source sidecar %s: %v", suffix, err)
			}
		}
	}
	checkSidecars()
	output := filepath.Join(dir, "older.db")
	if err := PrepareRollbackCopy(source, output, 2); err != nil {
		t.Fatal(err)
	}
	checkSidecars()
	if err := VerifyDesktopRollback(output); err != nil {
		t.Fatal(err)
	}
}
