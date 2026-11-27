package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"jindi-drama/internal/storage/sqlite"
)

func TestRestoreBackupChooserCancelOrFailure(t *testing.T) {
	app := New()
	result, err := app.restoreSelectedLibraryBackup(func() (string, error) { return "", nil })
	if result != nil || err != nil {
		t.Fatalf("cancel: %v, %v", result, err)
	}
	result, err = app.restoreSelectedLibraryBackup(func() (string, error) { return "", errors.New("private fixture") })
	if result != nil || err == nil || err.Error() != "无法打开系统文件选择器" {
		t.Fatalf("chooser error: %v, %v", result, err)
	}
}

func TestRestoreBackupReadsWALWithoutCreatingSourceSidecars(t *testing.T) {
	for _, tc := range []struct {
		name   string
		active bool
	}{{"closed", false}, {"active", true}} {
		t.Run(tc.name, func(t *testing.T) {
			a := New()
			if err := a.openStore(":memory:"); err != nil {
				t.Fatal(err)
			}
			defer a.close()
			root := t.TempDir()
			input := filepath.Join(root, "source.db")
			source, err := sqlite.Open(input)
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			entry := sqlite.HistoryEntry{DramaID: "fixture", Episode: 2, WatchedAt: 20}
			if err := source.SetFavorites([]string{"fixture"}); err != nil {
				t.Fatal(err)
			}
			if err := source.RecordHistory(entry); err != nil {
				t.Fatal(err)
			}
			if !tc.active {
				if err := source.Close(); err != nil {
					t.Fatal(err)
				}
			}
			sourceFiles := []string{input}
			if tc.active {
				sourceFiles = append(sourceFiles, input+"-wal", input+"-shm")
			}
			sourceBefore := make([][]byte, len(sourceFiles))
			for i, path := range sourceFiles {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				sourceBefore[i] = data
			}
			result, err := a.restoreSelectedLibraryBackup(func() (string, error) { return input, nil })
			if err != nil || result == nil || result.FavoritesAdded != 1 || result.HistoryAdded != 1 ||
				!slices.Equal(result.State.History, []sqlite.HistoryEntry{entry}) {
				t.Fatalf("restore result: %+v, %v", result, err)
			}
			files, err := os.ReadDir(root)
			if err != nil || len(files) != len(sourceFiles) {
				t.Fatalf("source directory contents: %v, %v", files, err)
			}
			for i, path := range sourceFiles {
				data, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(sourceBefore[i], data) {
					t.Fatalf("source file %s changed: %v", filepath.Base(path), err)
				}
			}
			if err := os.WriteFile(input+"-corrupt.db", []byte("corrupt fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			result, err = a.restoreSelectedLibraryBackup(func() (string, error) { return input + "-corrupt.db", nil })
			if result != nil || err == nil || err.Error() != "无法读取所选备份，请选择金帝短剧生成的数据库备份" {
				t.Fatalf("corrupt restore result: %+v, %v", result, err)
			}
			favorites, err := a.store.Favorites()
			if err != nil || !slices.Equal(favorites, []string{"fixture"}) {
				t.Fatalf("favorites changed after failed restore: %v, %v", favorites, err)
			}
			history, err := a.store.History()
			if err != nil || !slices.Equal(history, []sqlite.HistoryEntry{entry}) {
				t.Fatalf("history changed after failed restore: %v, %v", history, err)
			}
		})
	}
}

func TestRestoreBackupRejectsCurrentDatabase(t *testing.T) {
	dir := t.TempDir()
	a := New()
	input := filepath.Join(dir, "current.db")
	if err := a.openStore(input); err != nil {
		t.Fatal(err)
	}
	defer a.close()
	if err := a.store.SetFavorites([]string{"current"}); err != nil {
		t.Fatal(err)
	}
	result, err := a.restoreSelectedLibraryBackup(func() (string, error) { return input, nil })
	if result != nil || err == nil || err.Error() != "请选择备份文件，当前数据库正在使用" {
		t.Fatalf("restore current database: %+v, %v", result, err)
	}
	favorites, err := a.store.Favorites()
	if err != nil || !slices.Equal(favorites, []string{"current"}) {
		t.Fatalf("current database after restore refusal: %v, %v", favorites, err)
	}
}
