package sqlite

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"jindi-drama/internal/fileprivacy"
)

// PrepareRollbackCopy exports a standalone database at target without modifying
// the source or replacing an existing destination. The source may contain WAL.
func PrepareRollbackCopy(source, output string, target int) error {
	return prepareRollbackCopy(source, output, target, downgradeSnapshot)
}

func prepareRollbackCopy(source, output string, target int, downgrade func(string, int) error) error {
	if target < 1 || target > len(migrations) {
		return fmt.Errorf("无效的回滚目标版本 v%d", target)
	}
	output, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	name := filepath.Base(output)
	normalized := strings.ToLower(strings.TrimRight(name, ". "))
	if strings.HasSuffix(normalized, "-wal") || strings.HasSuffix(normalized, "-shm") || strings.HasSuffix(normalized, "-journal") {
		return errors.New("输出路径不能使用 SQLite 辅助文件名")
	}
	destination, err := os.OpenRoot(filepath.Dir(output))
	if err != nil {
		return err
	}
	defer destination.Close()
	if err := checkRollbackOutput(destination, name, false); err != nil {
		return err
	}

	work, err := openRollbackWorkspace()
	if err != nil {
		return err
	}
	const stagedName = "rollback.db"
	defer cleanRollbackStage(work, stagedName)
	if err := snapshotDatabase(source, work, stagedName); err != nil {
		return fmt.Errorf("读取源数据库快照失败: %w", err)
	}
	if !sameRollbackStage(work) {
		return errors.New("回滚工作目录已更换")
	}
	if err := downgrade(filepath.Join(work.Name(), stagedName), target); err != nil {
		return err
	}
	if !sameRollbackStage(work) {
		return errors.New("回滚工作目录已更换")
	}
	if !sameRollbackDestination(destination) {
		return errRollbackDestinationMoved
	}

	stage, err := openRollbackStage(destination)
	if err != nil {
		return err
	}
	defer cleanRollbackStage(stage, stagedName)
	if err := copyRollbackOutput(work, stage, stagedName); err != nil {
		return fmt.Errorf("暂存回滚副本失败: %w", err)
	}
	return publishRollbackOutput(stage, stagedName, destination, name, linkRollbackFile)
}

var (
	errRollbackDestinationMoved    = errors.New("回滚输出目录已移动或替换，请重新选择目录")
	errRollbackPublishedBeforeMove = errors.New("回滚输出目录已移动或替换，请在原目录查找已保存的副本")
)

func checkRollbackOutput(destination *os.Root, name string, published bool) error {
	for _, suffix := range [...]string{"", "-wal", "-shm", "-journal"} {
		if published && suffix == "" {
			continue
		}
		if _, err := destination.Lstat(name + suffix); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return errors.New("输出文件或 SQLite 辅助文件已存在，拒绝覆盖")
			}
			return err
		}
	}
	return nil
}

func openRollbackWorkspace() (*os.Root, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return nil, err
	}
	parent, err := os.OpenRoot(cache)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return openRollbackStage(parent)
}

func openRollbackStage(parent *os.Root) (*os.Root, error) {
	name := ".jindi-rollback-" + rand.Text()
	if err := parent.Mkdir(name, 0o700); err != nil {
		return nil, err
	}
	stage, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	if err := protectRollbackStage(stage); err != nil {
		cleanRollbackStage(stage, "")
		return nil, err
	}
	return stage, nil
}

func protectRollbackStage(stage *os.Root) error {
	if err := checkEmptyRollbackStage(stage); err != nil {
		return err
	}
	if err := fileprivacy.ProtectDirectory(stage); err != nil {
		return fmt.Errorf("无法保护回滚暂存目录: %w", err)
	}
	return checkEmptyRollbackStage(stage)
}

func copyRollbackOutput(work, stage *os.Root, name string) error {
	input, err := work.OpenFile(name, snapshotReadFlags, 0)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("回滚副本必须是普通文件")
	}
	output, err := stage.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = io.CopyN(output, input, info.Size())
	if err == nil {
		var actual os.FileInfo
		actual, err = input.Stat()
		if err == nil && !sameRollbackFile(info, actual) {
			err = errors.New("已校验的回滚副本在复制期间发生变化")
		}
	}
	if err == nil {
		err = output.Sync()
	}
	return errors.Join(err, output.Close())
}

func sameRollbackDestination(destination *os.Root) bool {
	original, err := destination.Stat(".")
	if err != nil {
		return false
	}
	selected, err := os.Stat(destination.Name())
	return err == nil && selected.IsDir() && os.SameFile(original, selected)
}

func publishRollbackOutput(stage *os.Root, staged string, destination *os.Root, name string, link func(*os.Root, string, *os.Root, string) error) error {
	if !sameRollbackDestination(destination) {
		return errRollbackDestinationMoved
	}
	if err := checkRollbackOutput(destination, name, false); err != nil {
		return err
	}
	if err := link(stage, staged, destination, name); err != nil {
		return fmt.Errorf("发布回滚副本失败: %w", err)
	}
	var result error
	if !sameRollbackDestination(destination) {
		result = errRollbackPublishedBeforeMove
	}
	if err := checkRollbackOutput(destination, name, true); err != nil {
		result = errors.Join(result, fmt.Errorf("回滚副本已保存，但发布后检查失败，请勿使用该副本: %w", err))
	}
	return result
}

func sameRollbackStage(root *os.Root) bool {
	expected, err := root.Stat(".")
	if err != nil {
		return false
	}
	current, err := os.Lstat(root.Name())
	return err == nil && current.IsDir() && os.SameFile(expected, current)
}

func checkEmptyRollbackStage(root *os.Root) error {
	if !sameRollbackStage(root) {
		return errors.New("回滚暂存目录已更换")
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	entries, err := dir.Readdirnames(1)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) != 0 {
		return errors.New("回滚暂存目录已有其他文件")
	}
	return nil
}

func cleanRollbackStage(root *os.Root, name string) {
	if name != "" {
		for _, suffix := range [...]string{"", "-wal", "-journal", "-shm"} {
			_ = root.Remove(name + suffix)
		}
	}
	removeDirectory := sameRollbackStage(root)
	_ = root.Close()
	if removeDirectory {
		// A directory that acquired other entries will remain in place.
		_ = fileprivacy.RemoveEmptyDirectory(root.Name())
	}
}

func snapshotDatabase(source string, root *os.Root, staged string) error {
	selected, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	source, err = filepath.EvalSymlinks(selected)
	if err != nil {
		return err
	}
	// SQLite may create sidecars when it opens even a read-only WAL database.
	// Keep it on a private copy of the durable file set.
	inputs := make(map[string]*os.File, len(snapshotSuffixes))
	originals := make(map[string]os.FileInfo, len(snapshotSuffixes))
	for _, suffix := range snapshotSuffixes {
		input, info, err := openSnapshotInput(source + suffix)
		if suffix != "" && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		defer input.Close()
		inputs[suffix] = input
		originals[suffix] = info
	}
	for suffix, input := range inputs {
		output, err := root.OpenFile(staged+suffix, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, err = io.CopyN(output, input, originals[suffix].Size())
		if err := errors.Join(err, output.Close()); err != nil {
			return err
		}
	}
	for suffix, input := range inputs {
		info := originals[suffix]
		original, err := rollbackDigest(io.NewSectionReader(input, 0, info.Size()), info.Size())
		if err != nil {
			return err
		}
		copied, err := stagedSnapshotDigest(root, staged+suffix, info.Size())
		if err != nil {
			return err
		}
		actual, err := input.Stat()
		if err != nil {
			return err
		}
		if original != copied || !sameRollbackFile(info, actual) {
			return errSnapshotChanged
		}
	}
	return checkSnapshotSources(selected, source, originals)
}

func checkSnapshotSources(selected, source string, files map[string]os.FileInfo) error {
	resolved, err := filepath.EvalSymlinks(selected)
	if err != nil {
		return errors.Join(errSnapshotChanged, err)
	}
	if resolved != source {
		return errSnapshotChanged
	}
	after, err := rollbackSourceFiles(source)
	if err != nil {
		return err
	}
	if !maps.EqualFunc(files, after, sameRollbackFile) {
		return errSnapshotChanged
	}
	return nil
}

var errSnapshotChanged = errors.New("源数据库正在变化，请退出应用后重试")

func openSnapshotInput(path string) (*os.File, os.FileInfo, error) {
	input, err := os.OpenFile(path, snapshotReadFlags, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := input.Stat()
	if err != nil {
		input.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		input.Close()
		return nil, nil, errors.New("源数据库及辅助文件必须是普通文件")
	}
	return input, info, nil
}

func stagedSnapshotDigest(root *os.Root, name string, size int64) ([sha256.Size]byte, error) {
	file, err := root.OpenFile(name, snapshotReadFlags, 0)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return [sha256.Size]byte{}, errSnapshotChanged
	}
	return rollbackDigest(file, size)
}

func rollbackDigest(file io.Reader, size int64) ([sha256.Size]byte, error) {
	hash := sha256.New()
	if _, err := io.CopyN(hash, file, size); err != nil {
		return [sha256.Size]byte{}, err
	}
	return [sha256.Size]byte(hash.Sum(nil)), nil
}

var snapshotSuffixes = [...]string{"", "-wal", "-journal"}

func rollbackSourceFiles(source string) (map[string]os.FileInfo, error) {
	files := make(map[string]os.FileInfo)
	for _, suffix := range snapshotSuffixes {
		info, err := os.Stat(source + suffix)
		if suffix != "" && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("源数据库及辅助文件必须是普通文件")
		}
		files[suffix] = info
	}
	return files, nil
}

func sameRollbackFile(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func downgradeSnapshot(path string, target int) error {
	db, err := openDatabase(path, "rw")
	if err != nil {
		return err
	}
	defer db.Close()
	current, err := supportedVersion(db)
	if err != nil {
		return err
	}
	if current < target {
		return fmt.Errorf("源数据库版本 v%d 低于目标 v%d，只能生成同版或更低版本的副本", current, target)
	}
	var journal string
	if err := db.QueryRow("PRAGMA journal_mode=DELETE").Scan(&journal); err != nil {
		return err
	}
	if journal != "delete" {
		return errors.New("无法生成独立的 SQLite 回滚副本")
	}
	if err := (&Store{db: db, Path: path}).Migrate(target); err != nil {
		return err
	}
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("回滚副本完整性检查失败: %.160s", integrity)
	}
	return db.Close()
}
