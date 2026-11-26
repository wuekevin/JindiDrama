// Package rollbackcli provides the SQLite rollback command for both entry points.
package rollbackcli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"jindi-drama/internal/storage/sqlite"
)

const DesktopFlag = "--prepare-rollback"

// Run creates a compatible copy and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("prepare-rollback", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", "", "源 SQLite 数据库路径")
	output := flags.String("output", "", "独立输出路径，文件必须不存在")
	target := flags.Int("target", 0, "旧版应用支持的数据库版本")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *source == "" || *output == "" || *target == 0 || flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if err := sqlite.PrepareRollbackCopy(*source, *output, *target); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "SQLite v%d 回滚副本已生成 %s\n", *target, *output)
	return 0
}
