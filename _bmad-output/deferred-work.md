- source_plan: _bmad-output/plan-e13-update-stage-cleanup.md
  summary: 在独立的 macOS 更新安全 Story 中核查暂存文件句柄与目录项的身份一致性。
  evidence: 独占重命名成功后，清理已不再触碰原文件名。发布前及硬链接回退仍按暂存文件名操作，应进一步确保它们绑定到实际经过校验的文件，并在异常时保留无关文件。
