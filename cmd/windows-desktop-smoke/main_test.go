package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jindi-drama/internal/storage/sqlite"
)

func TestSeedCreatesOnlyAnIsolatedDatabase(t *testing.T) {
	for scenario, action := range map[string]string{
		"Minimize": `"最小化到任务栏"`,
		"Tray":     `"隐藏到菜单栏"`,
	} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "jindi.db")
			if err := seed(path, scenario); err != nil {
				t.Fatal(err)
			}
			if err := seed(path, "Tray"); !errors.Is(err, fs.ErrExist) {
				t.Fatalf("seeding an existing database: %v", err)
			}
			store, err := sqlite.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			got, exists, err := store.GetSetting("window_close_action")
			if err != nil || !exists || got != action {
				t.Fatalf("fixture preference = %q, %v, %v", got, exists, err)
			}
		})
	}
}

func TestVerifyDBCommandUsesExistingProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jindi.db")
	if err := run([]string{"verify-db", path}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing profile: %v", err)
	}
	if err := run([]string{"seed", path, "Minimize"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify-db", path}); err != nil {
		t.Fatalf("existing profile: %v", err)
	}
}

func TestVerifyRollbackWALRequiresCommittedLibraryRows(t *testing.T) {
	for _, fixture := range []struct {
		name      string
		ignoreWAL bool
	}{
		{name: "complete"},
		{name: "drops-wal", ignoreWAL: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			dir := t.TempDir()
			err := verifyRollbackWAL(dir, func(source string, check func(string) error) error {
				output := filepath.Join(t.TempDir(), "old.db")
				if fixture.ignoreWAL {
					contents, err := os.ReadFile(source)
					if err != nil {
						return err
					}
					source = filepath.Join(t.TempDir(), "checkpointed.db")
					if err := os.WriteFile(source, contents, 0o600); err != nil {
						return err
					}
				}
				if err := sqlite.PrepareRollbackCopy(source, output, 2); err != nil {
					return err
				}
				return check(output)
			})
			if fixture.ignoreWAL {
				if err == nil || !strings.Contains(err.Error(), "lost or changed committed WAL library data") {
					t.Fatalf("WAL data loss was not detected: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
				t.Fatalf("temporary fixture was not cleaned: %v, %v", entries, err)
			}
		})
	}
}
