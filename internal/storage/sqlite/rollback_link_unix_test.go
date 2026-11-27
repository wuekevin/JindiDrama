//go:build unix

package sqlite

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareRollbackCopyRefusesMovedDestination(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "source.db")
	store, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	wantSource, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(parent, "chosen")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(destination, "rollback.db")
	moved := destination + "-original"
	err = prepareRollbackCopy(source, output, 2, func(path string, version int) error {
		if err := os.Rename(destination, moved); err != nil {
			return err
		}
		if err := os.Mkdir(destination, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(output, []byte("replacement"), 0o600); err != nil {
			return err
		}
		return downgradeSnapshot(path, version)
	})
	if !errors.Is(err, errRollbackDestinationMoved) {
		t.Fatalf("moved output directory: %v", err)
	}
	if data, err := os.ReadFile(output); err != nil || string(data) != "replacement" {
		t.Fatalf("replacement contents: %q %v", data, err)
	}
	if files, err := os.ReadDir(moved); err != nil || len(files) != 0 {
		t.Fatalf("original destination left files: %v %v", files, err)
	}
	if data, err := os.ReadFile(source); err != nil || !bytes.Equal(wantSource, data) {
		t.Fatalf("source changed: %v", err)
	}
}

func TestRollbackLinkUsesOpenedDirectories(t *testing.T) {
	parent := t.TempDir()
	open := func(name string) *os.Root {
		t.Helper()
		path := filepath.Join(parent, name)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		root, err := os.OpenRoot(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = root.Close() })
		return root
	}
	source := open("source")
	destination := open("destination")
	if err := source.WriteFile("verified.db", []byte("verified content"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []*os.Root{source, destination} {
		if err := os.Rename(dir.Name(), dir.Name()+"-original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(dir.Name(), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source.Name(), "verified.db"), []byte("substitute"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := linkRollbackFile(source, "verified.db", destination, "rollback.db"); err != nil {
		t.Fatal(err)
	}
	if err := linkRollbackFile(source, "verified.db", destination, "rollback.db"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected a collision, got %v", err)
	}
	if actual, err := destination.ReadFile("rollback.db"); err != nil || string(actual) != "verified content" {
		t.Fatalf("linked contents: %q %v", actual, err)
	}
	if _, err := os.Stat(filepath.Join(destination.Name(), "rollback.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement acquired a rollback file: %v", err)
	}
}

func TestPublishRollbackOutputReportsChangesDuringLink(t *testing.T) {
	for _, change := range []string{"move", "sidecar"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "output")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			destination, err := os.OpenRoot(path)
			if err != nil {
				t.Fatal(err)
			}
			defer destination.Close()
			stage, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer stage.Close()
			if err := stage.WriteFile("checked.db", []byte("verified"), 0o600); err != nil {
				t.Fatal(err)
			}
			err = publishRollbackOutput(stage, "checked.db", destination, "final.db",
				func(from *os.Root, file string, to *os.Root, name string) error {
					if change == "move" {
						if err := os.Rename(path, path+"-original"); err != nil {
							return err
						}
						if err := os.Mkdir(path, 0o700); err != nil {
							return err
						}
					}
					if err := linkRollbackFile(from, file, to, name); err != nil {
						return err
					}
					if change == "sidecar" {
						return to.WriteFile(name+"-wal", []byte("other"), 0o600)
					}
					return nil
				})
			if change == "move" {
				if !errors.Is(err, errRollbackPublishedBeforeMove) {
					t.Fatalf("moved while linking: %v", err)
				}
				if _, err := os.Stat(filepath.Join(path, "final.db")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("replacement acquired output: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("reported success despite a newly created WAL")
				}
				if data, err := destination.ReadFile("final.db-wal"); err != nil || string(data) != "other" {
					t.Fatalf("late WAL changed: %q %v", data, err)
				}
			}
			if data, err := destination.ReadFile("final.db"); err != nil || string(data) != "verified" {
				t.Fatalf("unexpected original output: %q %v", data, err)
			}
		})
	}
}
