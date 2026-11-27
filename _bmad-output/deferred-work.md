- source_plan: _bmad-output/plan-e13-update-stage-cleanup.md
  summary: 在独立的 macOS 更新安全 Story 中核查暂存文件句柄与目录项的身份一致性。
  evidence: 独占重命名成功后，清理已不再触碰原文件名。发布前及硬链接回退仍按暂存文件名操作，应进一步确保它们绑定到实际经过校验的文件，并在异常时保留无关文件。

- source_plan: _bmad-output/plan-e13-macos-update-identity.md
  summary: 继续核查 macOS 非 APFS 更新保存的字节完整性，以及同一账户进程并发修改暂存区的边界。
  evidence: 当前兼容路径在发布前后检查 inode 并拒绝报告身份不符的文件。该检查无法发现同一 inode 的字节变化。目录项检查与重命名、删除是独立操作。需要强化兼容卷的落盘校验与异常处理，同时保留正常的独占保存。APFS 只有初始化的目录项摘除涉及此类检查，下载与最终克隆均从打开的文件句柄执行。
