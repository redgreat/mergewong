---
description: 
alwaysApply: false
enabled: true
updatedAt: 2026-09-14T05:26:43.210Z
provider: 
---

# 数据库 MCP 工具使用规范

本文件为 AI IDE 提供 MCP 工具的使用指南。请严格遵循以下规范调用工具，确保操作安全、高效。

---

## 核心原则

1. **先查连接，再操作** — 不知道 `connection_id` 时必须先调 `list_connections`，禁止猜测 ID
2. **逐层探索** — 按 `list_connections` → `list_databases` → `list_tables` → `describe_table` 的顺序探索
3. **只读优先** — 查询用 `execute_query`，写入/DDL 用 `execute_sql`（需确认）
4. **默认数据库** — 大部分工具的 `database` 参数可选，不传则使用连接的默认数据库
5. **明确告知** — 每次操作后向用户说明使用的 `connection_id`、`database`、工具名称

---

## 工具速查表

| 工具 | 用途 | 必需参数 | 需权限 |
|------|------|----------|--------|
| `list_connections` | 列出可用连接 | 无 | - |
| `list_databases` | 列出数据库 | `connection_id` | 读 |
| `list_tables` | 列出表 | `connection_id` | 读 |
| `list_views` | 列出视图 | `connection_id` | 读 |
| `list_procedures` | 列出存储过程 | `connection_id` | 读 |
| `describe_table` | 查看表结构 | `connection_id`, `table` | 读 |
| `execute_query` | 执行 SELECT | `connection_id`, `sql` | 读 |
| `execute_sql` | 执行任意 SQL | `connection_id`, `sql` | DDL |
| `export_db_doc` | 导出数据库文档 | `connection_id` | 读 |
| `generate_er_diagram` | 生成 ER 图 | `connection_id` | 读 |
| `generate_data_flow` | 生成数据流图 | `connection_id` | 读 |
| `suggest_columns` | 字段添加建议 | `connection_id`, `table` | 读 |
| `analyze_performance` | 性能分析 | `connection_id` | 读 |
| `compare_schemas` | Schema 对比 | `source_connection_id`, `target_connection_id` | 读 |
| `generate_mock_data` | 生成测试数据 | `connection_id`, `table` | 读 |
| `analyze_sql` | SQL 审查 | `connection_id`, `sql` | 读 |
| `backup_table` | 表级备份 | `connection_id`, `table` | DDL |
| `analyze_db_config` | 数据库参数分析 | `connection_id` | 读 |

---

## 一、连接与探索类

### 1.1 list_connections — 列出可用连接

**何时使用：** 每次会话开始、用户提到数据库但未指定连接时，必须首先调用。

**参数：**
- `search`（可选）：按名称或数据库类型筛选，如 `"mysql"`、`"postgres"`、`"生产"`

**调用示例：**
```json
// 列出全部连接
{"search": ""}

// 搜索 MySQL 类型的连接
{"search": "mysql"}

// 搜索包含"生产"的连接
{"search": "生产"}
```

**返回解读：**
- 返回 `connections` 数组，每项含 `id`（即 connection_id）、`conn_name`、`db_type`、`host`、`database`
- 向用户展示时建议用表格格式列出，隐藏 host 等敏感信息

**注意事项：**
- 这是唯一**不需要** `connection_id` 的工具
- 如果返回空列表，提示用户在管理后台 检查 APPKEY 是否已授权连接

---

### 1.2 list_databases — 列出数据库

**何时使用：** 用户说"看看有哪些库"或需要确认数据库名称时。

**参数：**
- `connection_id`（必需）

**调用流程：**
1. 先 `list_connections` 获取 `connection_id`
2. 调 `list_databases`
3. 将返回的数据库名称列表展示给用户

---

### 1.3 list_tables — 列出表

**何时使用：** 用户说"看看这个库有哪些表"时。

**参数：**
- `connection_id`（必需）
- `database`（可选，不传则用连接默认库）

**调用示例：**
```json
{"connection_id": 1, "database": "mydb"}
```

---

### 1.4 list_views — 列出视图

**用法同 `list_tables`**，返回视图列表。

---

### 1.5 list_procedures — 列出存储过程

**用法同 `list_tables`**，返回存储过程列表。

---

### 1.6 describe_table — 查看表结构

**何时使用：** 用户问"这个表有哪些字段"、需要了解表结构再写 SQL 时。

**参数：**
- `connection_id`（必需）
- `table`（必需）：表名
- `database`（可选）

**调用示例：**
```json
{"connection_id": 1, "table": "users", "database": "mydb"}
```

**返回解读：**
- 返回 `columns` 数组，每列含字段名、类型、可空、默认值、键信息、备注
- 建议用表格展示给用户

**最佳实践：**
- 写 SQL 前先 `describe_table` 确认字段名和类型
- 帮用户写 INSERT/UPDATE 时先看表结构，避免字段遗漏

---

## 二、SQL 执行类

### 2.1 execute_query — 只读查询

**何时使用：** 用户需要查询数据（SELECT/SHOW）时。

**参数：**
- `connection_id`（必需）
- `sql`（必需）：仅支持 SELECT/SHOW 等只读语句

**调用示例：**
```json
{"connection_id": 1, "sql": "SELECT * FROM users LIMIT 10"}
```

**返回解读：**
- `rows` 数组 + `count` 行数
- 敏感数据会被自动脱敏

**注意事项：**
- 大表查询**必须加 LIMIT**，建议不超过 100 行
- 如果用户没写 LIMIT，帮他加上
- 不支持 INSERT/UPDATE/DELETE/DDL，这些用 `execute_sql`

---

### 2.2 execute_sql — 任意 SQL 执行

**何时使用：** 用户需要执行 INSERT/UPDATE/DELETE/CREATE/ALTER 等操作时。

**参数：**
- `connection_id`（必需）
- `sql`（必需）

**安全规范（极其重要）：**
1. **DDL 操作（CREATE/ALTER/DROP）需要管理员授予 DDL 权限**，如果报权限不足提示用户
2. **dangerous 操作必须先确认：**
   - `DROP TABLE` / `DROP DATABASE` → 向用户确认两次
   - `DELETE` 无 WHERE → 提醒用户这会删除全部数据
   - `TRUNCATE` → 提醒不可回滚
   - `UPDATE` 无 WHERE → 提醒会更新全部行
3. **建议在执行 DELETE/UPDATE 前先用 `execute_query` 做 SELECT 确认影响范围**
4. **批量操作建议先 `backup_table` 备份**

**调用流程（以 ALTER TABLE 为例）：**
1. `describe_table` 查看当前表结构
2. 向用户确认变更内容
3. 建议用 `backup_table` 先备份
4. `execute_sql` 执行

---

## 三、文档与图表类

### 3.1 export_db_doc — 导出数据库文档

**何时使用：** 用户说"导出数据库文档"、"生成数据字典"时。

**参数：**
- `connection_id`（必需）
- `database`（可选）
- `format`（可选）：`"markdown"`（默认）或 `"pdf"`

**调用示例：**
```json
// Markdown 格式（推荐，可直接展示）
{"connection_id": 1, "database": "mydb"}

// PDF 格式
{"connection_id": 1, "database": "mydb", "format": "pdf"}
```

**返回解读：**
- **markdown 格式**：返回 `content` 字段中的文本，包含完整的 Markdown，AI 应直接渲染展示。
- **pdf 格式**：返回标准 MCP `resource`。其中 `uri` 包含 `base64` 数据，`mimeType` 为 `application/pdf`。AI IDE 会自动识别此格式，**AI 应告知用户“文件已生成，点击界面上的链接或图标即可下载/保存”**。

**最佳实践：**
- 默认用 markdown 方便即时预览。
- 用户要求“下载”、“保存”或“离线查看”时，显式指定 `format="pdf"`。
- 文档包含：表汇总（表名+备注）→ 每张表字段详情（字段名、类型、长度、可空、默认值、备注）→ 索引信息

---

### 3.2 generate_er_diagram — 生成 ER 图

**何时使用：** 用户说"画 ER 图"、"看看表之间的关系"、"数据库关系图"时。

**参数：**
- `connection_id`（必需）
- `database`（可选）
- `include_columns`（可选，默认 true）：是否包含字段详情
- `include_implicit`（可选，默认 true）：是否包含推断的隐含关系（如 `user_id` → `users` 表）
- `output_type`（可选，默认 `"both"`）：`"mermaid"` / `"text"` / `"both"`

**调用示例：**
```json
// 完整 ER 图（推荐）
{"connection_id": 1, "database": "mydb"}

// 只要 Mermaid 代码（用户说"给我代码"）
{"connection_id": 1, "database": "mydb", "output_type": "mermaid"}

// 简洁版，不含字段详情（表多时推荐）
{"connection_id": 1, "database": "mydb", "include_columns": false}

// 只看显式外键，不推断隐含关系
{"connection_id": 1, "database": "mydb", "include_implicit": false}
```

**返回解读：**
- `mermaid_result`：Mermaid erDiagram 代码，可直接在 Markdown 中用 ```mermaid 渲染
- `text_description`：文字描述各表的字段和关系

**最佳实践：**
- 表非常多（>30）时建议 `include_columns: false`，否则图表巨大
- 返回后用 Mermaid 代码块展示给用户
- 如果用户对关系有疑问，可以调整 `include_implicit` 再次生成

---

### 3.3 generate_data_flow — 生成数据流图

**何时使用：** 用户说"数据流图"、"数据怎么流转的"、"触发器和视图之间的关系"时。

**参数：**
- `connection_id`（必需）
- `database`（可选）
- `output_type`（可选，默认 `"both"`）：`"mermaid"` / `"text"` / `"both"`

**调用示例：**
```json
{"connection_id": 1, "database": "mydb"}
```

**返回解读：**
- `mermaid_result`：Mermaid 流程图代码
- `text_description`：文字描述数据流向
- 分析内容包括：外键关系→触发器数据流→视图依赖→存储过程调用链

**最佳实践：**
- 向用户展示时用 Mermaid 代码块
- 建议先生成一次，让用户确认流程是否正确，再根据反馈调整

---

## 四、分析与优化类

### 4.1 suggest_columns — 字段添加建议

**何时使用：** 用户说"想给表加字段"、"ALTER TABLE 加列"时。

**两种用法：**

#### 用法 A：仅查看表信息
```json
{"connection_id": 1, "table": "users", "get_table_info": true}
```
返回表的完整信息：字段、索引、外键、触发器、被其他表引用的关系。

#### 用法 B：分析字段添加的影响
```json
{
  "connection_id": 1,
  "table": "users",
  "columns": [
    {"name": "avatar_url", "type": "VARCHAR(500)", "comment": "头像地址"},
    {"name": "last_login", "type": "TIMESTAMP", "nullable": "YES", "default": "NULL", "comment": "最后登录时间"},
    {"name": "score", "type": "INT", "nullable": "NO", "default": "0", "comment": "积分"}
  ]
}
```

**返回解读：**
- `ddl`：生成的 ALTER TABLE 语句，可以直接执行
- `dependent_views`：依赖此表的视图，可能需要更新
- `dependent_procedures`：引用此表的存储过程，可能需要调整
- `suggestions`：风险提示（如 NOT NULL 无默认值在已有数据表上会失败）

**最佳实践：**
1. 先用 `get_table_info: true` 查看表现状
2. 根据用户需求组织 `columns` 数组
3. 调用分析影响
4. 向用户展示 DDL 和影响分析
5. 用户确认后用 `execute_sql` 执行 DDL

---

### 4.2 analyze_performance — 数据库性能分析

**何时使用：** 用户说"数据库很慢"、"性能分析"、"看看有什么问题"时。

**参数：**
- `connection_id`（必需）
- `database`（可选）
- `analysis_type`（可选，默认 `"full"`）

**分析类型说明：**
| 值 | 分析内容 |
|----|----------|
| `full` | 全面分析（推荐首次使用） |
| `connections` | 仅连接数统计 |
| `slow_queries` | 仅慢 SQL 列表 |
| `locks` | 仅锁等待信息 |
| `table_stats` | 仅表大小/行数/碎片率 |
| `index_usage` | 仅索引使用情况 |

**调用示例：**
```json
// 全面分析（推荐）
{"connection_id": 1, "database": "mydb"}

// 用户问"哪个表最大"
{"connection_id": 1, "database": "mydb", "analysis_type": "table_stats"}

// 用户问"有没有慢SQL"
{"connection_id": 1, "database": "mydb", "analysis_type": "slow_queries"}
```

**返回解读：**
- `connection_stats`：连接数统计（总连接/活跃/空闲/最大连接）
- `slow_queries`：慢 SQL 列表（查询/调用次数/平均耗时/最大耗时）
- `lock_info`：锁等待（阻塞者/被阻塞者/查询）
- `table_stats`：表统计（大小/行数/碎片率），按大小降序
- `suggestions`：自动生成的优化建议，含严重级别

**最佳实践：**
- 首次排查用 `full`，然后根据 suggestions 的提示深入某个维度
- 向用户展示时重点突出 `suggestions` 中 severity=high 的项

---

### 4.3 analyze_sql — SQL 审查

**何时使用：** 用户说"帮我看看这条 SQL 有没有问题"、"优化下这个查询"、"SQL 审查"时。

**参数：**
- `connection_id`（必需）
- `sql`（必需）：要分析的 SQL 语句
- `database`（可选）

**调用示例：**
```json
{
  "connection_id": 1,
  "sql": "SELECT * FROM orders WHERE user_id = 100 ORDER BY created_at DESC",
  "database": "mydb"
}
```

**返回解读：**
- `explain_plan`：EXPLAIN 执行计划（JSON 格式）
- `tables_involved`：涉及的表名列表
- `plan_issues`：执行计划中发现的问题
  - `full_table_scan`（严重）：全表扫描，缺索引
  - `filesort`（中等）：文件排序
  - `temp_table`（中等）：使用临时表
  - `nested_loop`（低）：嵌套循环
- `index_suggestions`：索引建议（哪些字段缺索引）
- `rewrite_suggestions`：SQL 改写建议
  - SELECT * → 指定字段
  - LIKE '%xxx' → 前缀匹配或全文索引
  - NOT IN → NOT EXISTS
  - OR → UNION ALL
  - 无 LIMIT → 加 LIMIT
  - 大 OFFSET → 游标分页
- `overall_assessment`：综合评价

**最佳实践：**
- 先分析，再根据建议改写 SQL，然后再分析改写后的版本
- 向用户逐条解释每个 issue 和 suggestion
- 如果建议加索引，可以接着用 `execute_sql` 创建索引（需确认）

---

### 4.4 analyze_db_config — 数据库参数分析

**何时使用：** 用户说"数据库参数需要调优"、"默认配置合不合适"、"内存/连接数怎么配"时。

**参数：**
- `connection_id`（必需）

**调用示例：**
```json
{"connection_id": 1}
```

**返回解读（MySQL）：**
- `current_config`：当前配置参数值（innodb_buffer_pool_size, max_connections 等）
- `runtime_status`：运行状态统计（Threads_connected, Slow_queries 等）
- `suggestions`：调优建议，每条含：
  - `parameter`：参数名
  - `current`：当前值
  - `recommended`：推荐值（如有）
  - `severity`：严重级别（high/medium/low/info）
  - `message`：详细说明

**返回解读（PostgreSQL）：**
- `current_config`：参数及其 value/unit/source/description
- `runtime_stats`：数据库级统计（提交/回滚/缓存命中率/死锁/临时文件等）
- `suggestions`：调优建议

**分析的关键指标：**
| 指标 | MySQL | PostgreSQL |
|------|-------|------------|
| 缓存命中率 | Buffer Pool hit ratio | shared_buffers cache hit |
| 连接数 | max_connections vs Max_used | max_connections vs backends |
| 临时表 | Created_tmp_disk_tables | temp_files |
| 慢查询 | Slow_queries / Questions | log_min_duration_statement |
| 排序内存 | sort_buffer_size | work_mem |
| 并行查询 | - | max_parallel_workers |
| 自动清理 | - | autovacuum |
| 磁盘 IO | innodb_io_capacity | random_page_cost / effective_io_concurrency |

**最佳实践：**
- 配合 `analyze_performance` 一起使用，先看性能瓶颈再看配置
- 向用户展示时按 severity 排序，高优先
- 参数调整需要重启数据库的要特别提醒用户

---

## 五、Schema 管理类

### 5.1 compare_schemas — 数据库 Schema 对比

**何时使用：** 用户说"对比两个库的表结构"、"开发环境和生产环境有什么差异"、"同步表结构"时。

**⚠️ 特殊：** 此工具使用 `source_connection_id` + `target_connection_id`，而非 `connection_id`。

**参数：**
- `source_connection_id`（必需）：源库连接 ID（通常是开发/测试环境）
- `target_connection_id`（必需）：目标库连接 ID（通常是生产环境）
- `source_database`（可选）
- `target_database`（可选）
- `generate_ddl`（可选，默认 true）：是否生成同步 DDL

**调用流程：**
1. `list_connections` 获取两个连接的 ID
2. 确认源库（新版本/开发）和目标库（旧版本/生产）
3. 调用 `compare_schemas`

**调用示例：**
```json
{
  "source_connection_id": 1,
  "target_connection_id": 2,
  "source_database": "mydb_dev",
  "target_database": "mydb_prod"
}
```

**返回解读：**
- `comparison`：差异详情
  - `only_in_source`：仅源库有的表（需创建）
  - `only_in_target`：仅目标库有的表（可能需删除）
  - `column_diffs`：字段差异
    - `only_in_source`：源库有目标库没有的字段
    - `only_in_target`：目标库有源库没有的字段
    - `modified`：两边都有但定义不同的字段（类型/长度/可空等）
  - `index_diffs`：索引差异
- `sync_ddl`：从源库同步到目标库的 DDL 脚本

**最佳实践：**
- 先展示差异摘要（多少表差异、多少字段差异）
- 然后逐项展示具体差异
- DDL 脚本展示后让用户确认，不要自动执行
- 提示用户在目标库执行 DDL 前做好备份

---

### 5.2 backup_table — 表级备份

**何时使用：** 用户说"先备份这张表"、"做个快照"，或在做危险 DDL/DML 操作前主动建议。

**⚠️ 需要 DDL 权限。**

**参数：**
- `connection_id`（必需）
- `table`（必需）：要备份的表名
- `database`（可选）
- `suffix`（可选）：备份表后缀，默认 `YYYYMMDD_HHMMSS`

**调用示例：**
```json
// 使用默认时间戳后缀
{"connection_id": 1, "table": "users"}

// 自定义后缀
{"connection_id": 1, "table": "users", "suffix": "before_migration"}
```

**返回解读：**
- `success`：是否成功
- `source_table` / `backup_table`：原表名 / 备份表名
- `source_rows` / `backup_rows`：原表行数 / 备份行数（用于验证）
- `notes`：注意事项（备份不含索引/外键/约束）

**最佳实践：**
- 在执行 `ALTER TABLE`、`DELETE`、`UPDATE` 批量操作前主动建议备份
- 告知用户备份表的还原方式：`INSERT INTO 原表 SELECT * FROM 备份表`
- 告知清理方式：`DROP TABLE 备份表`
- 如果备份表已存在会报错，需换后缀或删除旧备份

---

## 六、数据生成类

### 6.1 generate_mock_data — 生成测试数据

**何时使用：** 用户说"生成测试数据"、"造一些假数据"、"INSERT 语句"时。

**参数：**
- `connection_id`（必需）
- `table`（必需）：目标表名
- `database`（可选）
- `count`（可选，默认 10）：生成行数，最大 100

**调用示例：**
```json
{"connection_id": 1, "table": "users", "count": 20}
```

**返回解读：**
- `inserts`：生成的全部 INSERT 语句文本
- `columns`：涉及的字段列表
- `foreign_key_refs`：外键引用关系
- `notes`：注意事项

**智能特性（自动处理）：**
- 自增主键字段 → 自动跳过
- 外键字段 → 从目标表取真实数据作为值
- 字段名含 `email` → 生成 `xxx@test.com`
- 字段名含 `phone`/`mobile` → 生成手机号格式
- 字段名含 `name`/`title` → 生成 `测试1`、`测试2`
- 字段名含 `url`/`link` → 生成 URL 格式
- `status`/`state` 字段 → 随机 0 或 1
- JSON/JSONB 字段 → 生成有效 JSON
- UUID 字段 → 生成有效 UUID
- 日期/时间字段 → 生成合理的日期时间

**最佳实践：**
- 先 `describe_table` 确认表结构
- 生成后向用户展示 INSERT 语句让其确认
- 用户确认后用 `execute_sql` 逐条或批量执行
- 提示用户生成的数据仅用于测试，不适用于生产环境

---

## 七、常见使用场景工作流

### 场景 1：用户说"帮我看看数据库"
```
1. list_connections → 展示可用连接
2. 用户选择连接后 → list_databases → 展示数据库
3. 用户选择数据库后 → list_tables → 展示表列表
4. 用户感兴趣的表 → describe_table → 展示结构
```

### 场景 2：用户说"数据库很慢，帮我看看"
```
1. list_connections → 确认连接
2. analyze_performance(analysis_type="full") → 全面分析
3. 根据 suggestions 重点关注 severity=high 的项
4. 如果有慢 SQL → 用 analyze_sql 深入分析具体语句
5. 如果碎片率高 → 建议 OPTIMIZE TABLE / VACUUM
6. analyze_db_config → 检查配置参数是否合理
```

### 场景 3：用户说"对比开发和生产的表结构"
```
1. list_connections → 找到两个连接
2. compare_schemas(source=开发, target=生产) → 获取差异
3. 展示差异摘要和详细对比
4. 如果用户要同步 → 展示 sync_ddl → 确认后在目标库执行
5. 执行前建议先 backup_table 备份关键表
```

### 场景 4：用户说"帮我加几个字段"
```
1. suggest_columns(get_table_info=true) → 先看表现状
2. 根据用户需求组织 columns 数组
3. suggest_columns(columns=[...]) → 查看 DDL 和影响分析
4. 如果有 dependent_views/procedures → 提醒用户可能需要同步修改
5. backup_table → 备份
6. execute_sql → 执行 ALTER DDL
```

### 场景 5：用户说"给这个表生成些测试数据"
```
1. describe_table → 了解表结构
2. generate_mock_data(count=10) → 生成测试数据
3. 展示 INSERT 语句给用户确认
4. 确认后 → execute_sql 逐条执行
```

### 场景 6：用户说"导出数据库文档"
```
1. list_connections → 确认连接
2. export_db_doc(format="markdown") → 生成预览
3. 展示 Markdown 文档内容
4. 如果用户说"导出 PDF 到 [路径]" 或环境支持自动保存：
   export_db_doc(format="pdf", save_path="D:\path\to\doc.pdf")
5. AI 告知用户文件已成功写入本地路径
```

### 场景 7：用户说"画个 ER 图"
```
1. list_connections → 确认连接
2. generate_er_diagram → 生成图表
3. 用 ```mermaid 代码块展示
4. 如果图表太大 → 加 include_columns=false 重新生成简洁版
```

---

## 八、错误处理

| 错误 | 原因 | 解决方式 |
|------|------|----------|
| "访问密钥不存在" | APPKEY 无效或已禁用 | 提示检查管理后台的密钥状态 |
| "该密钥无权访问此数据库连接" | 未授权 | 提示在管理后台给密钥添加连接权限 |
| "缺少必需参数: connection_id" | 未传连接 ID | 先调 `list_connections` |
| "IP 不在白名单中" | 客户端 IP 未授权 | 提示在管理后台添加 IP 白名单 |
| "风险SQL" | SQL 被安全拦截 | 检查 SQL 是否含危险操作，确认后重试 |
| "DDL 权限不足" | 需要 DDL 权限 | 提示在管理后台给连接启用 allow_ddl |
| 空结果集 | 数据/权限问题 | 确认数据库名和表名正确，检查权限 |