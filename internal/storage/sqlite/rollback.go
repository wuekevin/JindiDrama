package sqlite

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// PrepareRollbackCopy exports a standalone database at target without modifying
// the source or replacing an existing destination. The source may contain WAL.
func PrepareRollbackCopy(source, output string, target int) error {
	if target < 1 || target > len(migrations) {
		return fmt.Errorf("无效的回滚目标版本 v%d", target)
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("源数据库必须是普通文件")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	name := strings.ToLower(strings.TrimRight(filepath.Base(output), ". "))
	if strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") || strings.HasSuffix(name, "-journal") {
		return errors.New("输出路径不能使用 SQLite 辅助文件名")
	}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(output + suffix); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return errors.New("输出文件或 SQLite 辅助文件已存在，拒绝覆盖")
			}
			return err
		}
	}

	dir, err := os.MkdirTemp(filepath.Dir(output), ".jindi-rollback-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	staged := filepath.Join(dir, "rollback.db")
	if err := snapshotDatabase(source, staged); err != nil {
		return fmt.Errorf("读取源数据库快照失败: %w", err)
	}
	if err := downgradeSnapshot(staged, target); err != nil {
		return err
	}
	if err := os.Chmod(staged, 0o600); err != nil {
		return err
	}
	// Link publishes the completed file without replacing a concurrently created
	// destination. Keeping the staging directory beside it ensures the same volume.
	if err := os.Link(staged, output); err != nil {
		return fmt.Errorf("发布回滚副本失败: %w", err)
	}
	return nil
}

func snapshotDatabase(source, staged string) error {
	files, err := rollbackSourceFiles(source)
	if err != nil {
		return err
	}
	// Opening even a read-only WAL database can create sidecars. Let SQLite
	// open and, if necessary, recover a private copy of the durable file set.
	for suffix := range files {
		if err := copyDatabaseFile(source+suffix, staged+suffix); err != nil {
			return err
		}
	}
	for suffix := range files {
		original, err := rollbackDigest(source + suffix)
		if err != nil {
			return err
		}
		copied, err := rollbackDigest(staged + suffix)
		if err != nil {
			return err
		}
		if original != copied {
			return errors.New("源数据库正在变化，请退出应用后重试")
		}
	}
	after, err := rollbackSourceFiles(source)
	if err != nil {
		return err
	}
	if !maps.EqualFunc(files, after, sameRollbackFile) {
		return errors.New("源数据库正在变化，请退出应用后重试")
	}
	return nil
}

func rollbackDigest(path string) (digest [sha256.Size]byte, err error) {
	file, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return digest, err
	}
	return [sha256.Size]byte(hash.Sum(nil)), nil
}

func rollbackSourceFiles(source string) (map[string]os.FileInfo, error) {
	files := make(map[string]os.FileInfo)
	for _, suffix := range []string{"", "-wal", "-journal"} {
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
