package main

import (
	"errors"
	"io/fs"
	"path/filepath"
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
