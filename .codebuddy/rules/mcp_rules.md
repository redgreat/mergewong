---
alwaysApply: false
description: 数据库服务MCP生效
---
1.查找连接
不知道 connection_id 时，先调用 `list_connections()` 列出全部
可用 `search` 参数按名称或类型（mysql/postgres）筛选
禁止猜测 ID，必须先查询
2. 探索结构
- 获得 connection_id 后按顺序：
  1. `list_databases` → 选库
  2. `list_tables` → 找表
  3. `describe_table` → 查字段

3. 执行查询
只读用 `execute_query`（SELECT/SHOW）
写入/DDL 用 `execute_sql`（需权限）
危险操作需用户确认

4. 错误处理
权限不足时说明可能是 APPKEY 配置问题
返回空结果时提示检查管理后台授权

5. 结果展示
明确告知使用的 connection_id 和 database
