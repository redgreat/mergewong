[OPEN] xa-tx-not-synced

## 现象
- MySQL 执行 XA 事务不再报错，但增量数据没有同步到目标端
- 同步任务在管理端看起来“正常”，但目标库数据不变化

## 期望
- XA 事务内的 INSERT/UPDATE/DELETE 能被 CDC 链路捕获并同步（或至少明确不支持并给出可观测的告警/日志）

## 环境
- 项目：MergeWong
- 管理库连接：23（用户说明：可在 MCP 查询）

## 假设（待证伪）
- H1：CDC 解析层对 XA 事务的 binlog 事件序列处理不完整，导致事务提交点（XID/COMMIT）未触发 apply
- H2：事件被过滤（schema/table mapping 不匹配、库名不一致、表不在 mappings 中、sync_state 非 active）
- H3：checkpoint/位点推进逻辑在 XA 场景下异常（位点已推进但 operations 未落库），导致“看似正常但无写入”
- H4：目标端写入实际报错但被吞（applyCDCTransaction / writeMySQLBatchTx 返回错误未上报或被覆盖）
- H5：延迟/运行状态指标更新正常，但 last_event_at/rows_processed 没有增长，属于“假活跃”

## 计划（只做插桩，不改业务逻辑）
- 启动 Debug Server 收集运行日志（pre）
- 在 CDC 事件循环中打点：RowsEvent、QueryEvent、XIDEvent、RotateEvent、applyCDCTransaction 前后、checkpoint 更新
- 复现：执行一次 XA 事务写入（用户侧）
- 基于证据锁定根因后再做最小修复（post）

