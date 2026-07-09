[OPEN] xa-commit-missing-prepared

## 现象
- 执行外部 XA 事务写入源表成功，但 CDC 链路报错：XA COMMIT 未找到 prepared 缓存
- 管理库中存在 sync_xa_prepared_transactions 记录，但 xid_key 与报错中的 xid_key 不一致

## 期望
- XA PREPARE 与 XA COMMIT 能用同一个 xid_key 关联到同一条 prepared 事务缓存，从而在 COMMIT 时正确回放到目标库

## 复现
- 源库执行：
  - XA START ...; INSERT ...; XA END ...; XA PREPARE ...; XA COMMIT ...;
- CDC 报错：XA COMMIT 未找到 prepared 缓存: xid=...

## 假设（可证伪）
- H1：XA_PREPARE_LOG_EVENT 的 xid 解析偏移不正确，导致 saveXAPrepared 写入的 xid_key 与 XA COMMIT QueryEvent 解析出的 xid_key 不一致
- H2：XA COMMIT 实际对应的 prepared 记录被错误删除（deleteXAPrepared 的模糊匹配误删）
- H3：XA COMMIT 事件到达时，PREPARE 对应的操作列表未被持久化（operations 为空或被过滤）
- H4：任务表映射/库名过滤导致 RowsEvent 没有进入 operations，最终 prepared 记录缺少 operations_json 或为空

## 证据（当前）
- 任务报错：XA COMMIT 未找到 prepared 缓存: xid=1:6d775f78615f303035:
- prepared 表记录 xid_key：1:0000006d775f78615f:（存在前导 000000 且末尾缺少 303035）

