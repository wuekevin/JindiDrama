// prepare-rollback creates a database copy that an older desktop build can read.
package main

import (
	"flag"
	"fmt"
	"os"

	"jindi-drama/internal/storage/sqlite"
)

func main() {
	source := flag.String("source", "", "源 SQLite 数据库路径")
	output := flag.String("output", "", "独立输出路径，文件必须不存在")
	target := flag.Int("target", 0, "旧版应用支持的数据库版本")
	flag.Parse()
	if *source == "" || *output == "" || *target == 0 || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := sqlite.PrepareRollbackCopy(*source, *output, *target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("SQLite v%d 回滚副本已生成 %s\n", *target, *output)
}
