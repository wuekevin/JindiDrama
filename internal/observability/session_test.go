package observability

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testSessionLog(t *testing.T) (*Logger, string) {
	t.Helper()
	root := t.TempDir()
	log, err := New(filepath.Join(root, "logs"), DefaultLogBytes)
	if err != nil {
		t.Fatal(err)
	}
	return log, filepath.Join(root, "sessions")
}

func startTestSession(t *testing.T, dir, version string, log *Logger) *Session {
	t.Helper()
	session, err := StartSession(dir, version, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.End(true) })
	return session
}

func TestSessionReportsStoppedStagesAndSkipsLiveOrCleanSessions(t *testing.T) {
	log, dir := testSessionLog(t)
	earlier := startTestSession(t, dir, "v1.0.0", log)
	earlier.End(false)
	running := startTestSession(t, dir, "v1.1.0", log)
	running.Ready()
	live := startTestSession(t, dir, "v1.2.0", log)
	live.Ready()
	lines, err := log.Tail(10)
	if err != nil || len(lines) != 1 || !strings.Contains(lines[0], "窗口就绪前") ||
		!strings.Contains(lines[0], "v1.0.0") {
		t.Fatalf("report before readiness = %v, %v", lines, err)
	}
	running.End(false)
	clean := startTestSession(t, dir, "v1.3.0", log)
	clean.Ready()
	clean.End(true)
	next := startTestSession(t, dir, "v1.4.0", log)
	next.End(true)
	live.End(true)

	lines, err = log.Tail(10)
	if err != nil || len(lines) != 2 || !strings.Contains(lines[1], "窗口就绪后") ||
		!strings.Contains(lines[1], "v1.1.0") {
		t.Fatalf("reported a live or clean session, or duplicated a report: %v, %v", lines, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("session marker cleanup = %v, %v", entries, err)
	}
}

func TestSessionCollectorsReportStoppedSessionOnce(t *testing.T) {
	log, dir := testSessionLog(t)
	stopped := startTestSession(t, dir, "v1.0.0", log)
	stopped.End(false)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			session, err := StartSession(dir, "v2.0.0", log)
			if err != nil {
				t.Error(err)
				return
			}
			session.End(true)
		})
	}
	wg.Wait()
	lines, err := log.Tail(10)
	if err != nil || len(lines) != 1 {
		t.Fatalf("reports = %v, %v", lines, err)
	}
}

func TestSessionDoesNotCopyInvalidMarkerContent(t *testing.T) {
	log, dir := testSessionLog(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "run-invalid.session")
	if err := os.WriteFile(path, []byte("R{\"version\":\"token=private\",\"os\":\"darwin\",\"arch\":\"arm64\",\"started\":1}"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := startTestSession(t, dir, "v1.0.0", log)
	session.End(true)
	if _, err := log.Tail(10); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid marker should not produce a diagnostic: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid marker should be removed: %v", err)
	}
}

func TestSessionReportSurvivesLogRotation(t *testing.T) {
	root := t.TempDir()
	log, err := New(root, 100)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "sessions")
	stopped := startTestSession(t, dir, "v1.0.0", log)
	stopped.Ready()
	stopped.End(false)
	next := startTestSession(t, dir, "v1.1.0", log)
	next.End(true)
	if err := log.Log("warn", "storage", "startup warning"); err != nil {
		t.Fatal(err)
	}
	externalLogger, err := New(root, 100)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := externalLogger.Tail(2)
	if err != nil || len(lines) != 2 || !strings.Contains(lines[0], "窗口就绪后") ||
		!strings.Contains(lines[1], "startup warning") {
		t.Fatalf("rotated crash diagnostic was not included: %v, %v", lines, err)
	}
}

func TestSessionCleansOnlyStaleEmptyMarkers(t *testing.T) {
	log, dir := testSessionLog(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "run-empty-old.session")
	fresh := filepath.Join(dir, "run-empty-fresh.session")
	for _, path := range []string{old, fresh} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Now().Add(-2 * staleEmptyAge)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	startTestSession(t, dir, "v1.0.0", log).End(true)
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale marker was not removed: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh marker could be in preparation: %v", err)
	}
}

func TestReadySessionCanBeReadWithoutTakingItsLock(t *testing.T) {
	log, dir := testSessionLog(t)
	session := startTestSession(t, dir, "v1.0.0", log)
	session.Ready()

	observer, err := os.OpenFile(session.file.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()
	var stage [1]byte
	if _, err := observer.ReadAt(stage[:], 0); err != nil || stage[0] != sessionReady {
		t.Fatalf("readiness marker while app is running = %q, %v", stage[0], err)
	}
	if locked, err := tryFileLock(observer); err != nil || locked {
		t.Fatalf("observer acquired a live session lock: %v, %v", locked, err)
	}
}
