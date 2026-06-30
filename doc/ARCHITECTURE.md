# MergeWong 架构与现状分析

## 1. 目标

MergeWong 的目标不是复刻 DataX、Flink CDC 或 Debezium，而是提供一个容易部署和维护的中小型表同步服务：用户在管理端配置源库、目标库、表、字段映射和同步方式，服务持续、可恢复地把变更写入目标表。

这里需要区分两个产品能力：

- **准实时增量**：每隔数秒或数分钟查询一次递增字段，成本低、跨数据库容易实现。
- **CDC 实时**：读取 binlog/WAL/CDC 日志，能捕获插入、更新和删除，延迟更低，但依赖数据库权限和各自协议。

建议先把准实时增量做可靠，再添加 MySQL → MySQL 的 CDC。不要在同一个版本同时覆盖三种数据库的 CDC。

## 2. 现有代码如何工作

### 启动阶段

`cmd/server/main.go` 依次完成：读取 YAML 配置、建立配置中的数据库连接、在 `system` 库自动迁移元数据表、加载管理端保存的连接、加载启用的 Cron 任务、注册 Gin API，并托管 `web/dist` 静态文件。

### 元数据

系统库目前保存四类数据：

- `users`：登录用户；
- `database_connections`：动态数据源配置；
- `sync_tasks`：源/目标、字段映射、同步类型和 Cron；
- `sync_logs`：一次执行的状态、行数、耗时和错误。

### 当前同步链路

`internal/services/sync_service.go` 的执行逻辑是：

1. 读取任务并标记为 `running`；
2. 全量任务执行 `SELECT * FROM source_table`；
3. 增量任务查询目标表 `MAX(incremental_key)`，再从源表读取大于该值的数据；
4. 将每行转换为 map，应用字段名映射；
5. 在目标库的一个事务内逐行 `INSERT`；
6. 写同步日志并更新任务最后状态。

### 参考项目 manualrepl

`D:\github\manualrepl` 使用 Python `mysql-replication` 持续读取 MySQL binlog，将行事件区分为新增、更新、删除，再通过表专用 Processor 写入目标宽表。它值得复用的是事件模型和业务转换思路，不建议直接把其代码并入本项目：它没有持久化 binlog 位点、通用任务模型、重连/重试与管理面，且转换逻辑与具体供应商表强耦合。

## 3. 现有实现的关键风险

1. **水位线不可靠**：从目标表最大值推导进度，目标数据被修改、字段映射改名或写入部分成功时会丢数或重放。
2. **同值数据可能丢失**：仅使用 `WHERE key > ?`，当多行具有相同时间戳且一个批次只处理了部分数据时，没有主键作为第二游标。
3. **更新不可重放**：目标端只有 insert；任务重试、全量重跑或源记录更新可能触发主键冲突。
4. **删除无法同步**：轮询当前表看不到已物理删除的行。
5. **大表风险**：无分页、无批量、一个长事务逐行写入，会占用内存、连接和锁。
6. **并发重入**：手动执行和 Cron 可能同时运行同一任务。
7. **SQL 标识符风险**：表名和列名直接拼接，缺少白名单校验和方言转义。
8. **任务更新不完整**：连接或任务更新后，运行时连接池/调度项的一致性需要系统性验证。
9. **凭据明文存储**：动态连接密码直接写入系统库，尚未实现可轮换的加密存储。
10. **缺少测试**：当前 `go test ./...` 只有编译检查，没有同步语义测试。

因此当前版本只能作为原型，不应对生产数据库直接开启大表同步。

## 4. 推荐目标架构

### 控制面

保留 Gin + Svelte，负责连接、任务、检查点、运行历史和操作入口。管理库统一使用 PostgreSQL；若想降低本地部署门槛，可在后续支持 SQLite，但不要让业务数据经过 SQLite。

### 数据面

把同步执行拆成稳定接口：

```go
type Reader interface {
    Read(ctx context.Context, checkpoint Checkpoint, limit int) (Batch, error)
}

type Transformer interface {
    Transform(ctx context.Context, batch Batch) (Batch, error)
}

type Writer interface {
    Write(ctx context.Context, batch Batch) error
}
```

统一事件至少包含：操作类型、源库/表、主键、行前/行后数据、源端位置和事件时间。轮询 Reader 产生 upsert 事件；CDC Reader 产生 insert/update/delete 事件。Writer 负责按目标数据库方言生成批量 upsert/delete。

### 检查点

新增独立 `sync_checkpoints` 表。轮询检查点保存 `cursor_value + primary_key`；MySQL CDC 保存 `binlog_file + position`，有 GTID 时优先保存 GTID。只有目标批次成功提交后才能推进检查点。

严格的“目标写入 + 系统库检查点”无法跨异构数据库做原子事务，因此采用 **at-least-once + 幂等 upsert**：崩溃时允许重放，不能丢数据。

### 初始全量与 CDC 衔接

CDC 任务通常需要先全量再追增量：先记录日志位点，再做一致性快照，最后从该位点消费变化。首版可限制源/目标均为 MySQL，并明确所需的 binlog 格式、账号权限和 server ID。

## 5. 技术选型结论

### Go（推荐）

适合当前团队和目标规模。建议优先使用 `database/sql` 完成数据面，GORM 保留给元数据 CRUD；同步 SQL 对方言、批量和返回类型控制要求更高，完全依赖 ORM 反而会变复杂。MySQL CDC 可评估 `go-mysql-org/go-mysql`。

### Java

只有在以下情况更合适：团队已熟悉 JVM；需要直接二次开发 Debezium/Flink CDC/DataX；需要 Kafka 生态和分布式状态管理；吞吐、连接器数量或一致性要求已明显超出单体服务。当前不满足这些条件。

### Python

适合 PoC 和业务转换插件，不建议作为主同步守护进程。`manualrepl` 可用于验证具体表的 binlog 映射规则，也可将成熟规则翻译为 Go Transformer。

## 6. 建议的首版范围

首个可生产试用版本建议只承诺：

- MySQL → MySQL；
- 全量初始化 + `updated_at/id` 复合游标轮询；
- 主键 upsert；
- 软删除字段同步；
- 分页读取、批量提交、任务互斥、失败重试、独立检查点；
- 单实例运行；
- 管理端可观察运行状态。

这一范围做稳后，再按真实需求选择 MySQL binlog CDC 或增加 PostgreSQL/SQL Server Writer。
