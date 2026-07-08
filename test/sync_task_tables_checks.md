# Sync task table checks

这些检查原来位于 `internal/services/sync_task_tables_test.go`，用于覆盖同步表配置、字段映射、忽略字段和 MySQL 扫描值规整。

已移出正式服务代码目录。后续如果需要恢复自动化测试，应优先把对应逻辑整理为可导出的纯函数，或在 `internal/services` 包内按 Go 标准保留 `_test.go` 文件。

覆盖点：

- 同步表配置不能为空。
- 源表、目标表不能重复。
- 表名和字段映射必须是安全标识符。
- 多个源字段不能映射到同一个目标字段。
- 同名字段映射会被忽略。
- `ignored_fields` 会从同步字段列表中过滤。
- MySQL 扫描出的 `[]byte` 文本值需要转成 string，避免 GORM 参数展开导致列数不一致。
