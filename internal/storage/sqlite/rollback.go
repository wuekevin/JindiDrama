package sqlite

import (
	"errors"
	"fmt"
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
	db, err := openDatabase(source, "ro")
	if err != nil {
		return err
	}
	defer db.Close()
	return (&Store{db: db, Path: source}).Backup(staged)
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
