package sqlite

import (
	"fmt"
	"os"

	"jindi-drama/internal/fileprivacy"
)

func createRollbackStage(parent *os.Root, name string) (*os.Root, error) {
	stage, err := fileprivacy.CreatePrivateDirectory(parent, name)
	if err != nil {
		return nil, fmt.Errorf("无法保护回滚暂存目录: %w", err)
	}
	return stage, nil
}
