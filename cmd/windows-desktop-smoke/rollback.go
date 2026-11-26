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
	"time"

	"jindi-drama/internal/storage/sqlite"
)

func verifyRollback(executable, source string) error {
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
	return sqlite.VerifyDesktopRollback(output)
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
