package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"jindi-drama/internal/storage/sqlite"
)

func verifyRollback(executable, source string) (result error) {
	if err := verifyRollbackCopy(executable, source, sqlite.VerifyDesktopRollback); err != nil {
		return err
	}
	fixtureRoot, err := os.MkdirTemp("", "jindi-packaged-rollback-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(fixtureRoot)) }()
	return verifyRollbackWAL(fixtureRoot, func(input string, check func(string) error) error {
		return verifyRollbackCopy(executable, input, check)
	})
}

func verifyRollbackCopy(executable, source string, check func(string) error) error {
	before, err := readRollbackSource(source)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Dir(source), "rollback-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	output := filepath.Join(work, "older copy.db")
	profile := filepath.Join(work, "unexpected-profile")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, "--prepare-rollback", "-source", source, "-output", output, "-target", "2")
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = append(cmd.Environ(), "JINDIDRAMA_DATA_DIR="+profile,
		"JINDIDRAMA_CMS_URL=", "JINDIDRAMA_LICENSED_MEDIA_DIR=")
	unexpected := []string{profile, output + "-wal", output + "-shm", output + "-journal"}
	if runtime.GOOS == "windows" {
		// Use the smoke harness's existing private AppData area if the app starts unexpectedly.
		appData := filepath.Dir(filepath.Dir(source))
		webview := filepath.Join(appData, filepath.Base(executable), filepath.Base(work))
		cmd.Env = append(cmd.Env, "APPDATA="+appData, "LOCALAPPDATA="+appData,
			"WEBVIEW2_USER_DATA_FOLDER="+webview)
		unexpected = append(unexpected, webview)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("packaged rollback failed: %w: %.4096s", err, output)
	}
	after, err := readRollbackSource(source)
	if err != nil {
		return err
	}
	if !maps.EqualFunc(before, after, bytes.Equal) {
		return errors.New("packaged rollback changed the source database")
	}
	for _, name := range unexpected {
		if _, err := os.Lstat(name); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("unexpected rollback side effect: %s", filepath.Base(name))
		}
	}
	return check(output)
}

func verifyRollbackWAL(parent string, run func(string, func(string) error) error) (result error) {
	dir, err := os.MkdirTemp(parent, "rollback-wal-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(dir)) }()
	source := filepath.Join(dir, "fixture.db")
	store, err := sqlite.Open(source)
	if err != nil {
		return err
	}
	if err := store.Close(); err != nil {
		return err
	}
	checkpointed, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	store, err = sqlite.Open(source)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, store.Close()) }()

	favorites := []string{"wal-favorite-second", "wal-favorite-first"}
	history := sqlite.HistoryEntry{DramaID: "wal-history", Episode: 3, PositionMs: 6789,
		DurationMs: 10000, Percent: 67, WatchedAt: 1735689600123, EpisodeCountAtWatch: 5}
	if err := store.SetFavorites(favorites); err != nil {
		return err
	}
	if err := store.RecordHistory(history); err != nil {
		return err
	}
	files, err := readRollbackSource(source)
	if err != nil {
		return err
	}
	if !bytes.Equal(checkpointed, files[""]) || len(files["-wal"]) <= 32 {
		return errors.New("rollback fixture records are not isolated in the WAL")
	}
	// The v2 schema stores the position and episode but predates episode totals.
	history.EpisodeCountAtWatch = 0
	return run(source, func(output string) error {
		if err := sqlite.VerifyDesktopRollback(output); err != nil {
			return err
		}
		root, err := os.OpenRoot(dir)
		if err != nil {
			return err
		}
		defer root.Close()
		library, err := sqlite.ReadLibraryBackup(output, root, "readback.db")
		if err != nil {
			return err
		}
		if !slices.Equal(library.Favorites, favorites) ||
			!slices.Equal(library.History, []sqlite.HistoryEntry{history}) {
			return errors.New("packaged rollback lost or changed committed WAL library data")
		}
		return nil
	})
}

// SQLite may leave committed data in the WAL. Include durable sidecars in
// the comparison, excluding the volatile shared-memory index.
func readRollbackSource(source string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	for _, suffix := range []string{"", "-wal", "-journal"} {
		data, err := os.ReadFile(source + suffix)
		if suffix != "" && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files[suffix] = data
	}
	return files, nil
}
