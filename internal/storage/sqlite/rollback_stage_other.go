//go:build !windows

package sqlite

import (
	"fmt"
	"os"

	"jindi-drama/internal/fileprivacy"
)

func createRollbackStage(parent *os.Root, name string) (*os.Root, error) {
	if err := parent.Mkdir(name, 0o700); err != nil {
		return nil, err
	}
	stage, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	if err := checkEmptyRollbackStage(stage); err != nil {
		cleanRollbackStage(stage, "")
		return nil, err
	}
	if err := fileprivacy.ProtectDirectory(stage); err != nil {
		cleanRollbackStage(stage, "")
		return nil, fmt.Errorf("无法保护回滚暂存目录: %w", err)
	}
	return stage, nil
}
