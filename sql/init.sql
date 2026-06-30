-- ============================================================================
-- MergeWong 管理库初始化脚本（PostgreSQL 14+）
--
-- 执行方式：
--   psql -h 127.0.0.1 -p 5432 -U postgres -d postgres -f sql/init.sql
--
-- 默认管理库：mergewong
-- 默认数据库账号：mergewong
-- 默认数据库密码：MergeWong@2026!
-- 默认后台管理员：admin / admin123
--
-- 生产环境执行前必须修改数据库密码；首次登录后必须修改后台管理员密码。
-- 本文件使用 psql 的 \gexec 和 \connect 命令，应通过 psql 执行。
-- ============================================================================

\set ON_ERROR_STOP on

-- ----------------------------------------------------------------------------
-- 1. 创建或更新应用账号
-- ----------------------------------------------------------------------------
DO $do$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'user_merge') THEN
    CREATE ROLE user_merge
      LOGIN
      PASSWORD 'Merge@2026!'
      NOSUPERUSER
      NOCREATEDB
      NOCREATEROLE
      NOREPLICATION;
  ELSE
    ALTER ROLE user_merge
      WITH LOGIN PASSWORD 'Merge@2026!';
  END IF;
END
$do$;

-- ----------------------------------------------------------------------------
-- 2. 创建管理数据库
-- PostgreSQL 不支持 CREATE DATABASE IF NOT EXISTS，使用 psql \gexec 实现幂等。
-- ----------------------------------------------------------------------------
SELECT format(
  'CREATE DATABASE %I OWNER %I ENCODING %L TEMPLATE template0',
  'mergewong', 'user_merge', 'UTF8'
)
WHERE NOT EXISTS (
  SELECT 1 FROM pg_database WHERE datname = 'mergewong'
) \gexec

ALTER DATABASE mergewong OWNER TO user_merge;
GRANT CONNECT, TEMPORARY ON DATABASE mergewong TO user_merge;

\connect mergewong

-- 后续对象由应用账号拥有，便于 Go 服务执行 GORM AutoMigrate。
SET ROLE user_merge;
SET client_encoding = 'UTF8';
SET timezone = 'Asia/Shanghai';

-- ----------------------------------------------------------------------------
-- 3. 后台用户表
-- 对应 internal/models/user.go
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NULL,
  updated_at TIMESTAMPTZ NULL,
  deleted_at TIMESTAMPTZ NULL,
  username VARCHAR(50) NOT NULL,
  password VARCHAR(255) NOT NULL,
  email VARCHAR(100) NULL,
  role VARCHAR(20) NOT NULL DEFAULT 'viewer',
  status BIGINT NOT NULL DEFAULT 1,
  CONSTRAINT ck_users_role CHECK (role IN ('admin', 'viewer')),
  CONSTRAINT ck_users_status CHECK (status IN (0, 1))
);

CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username);

COMMENT ON TABLE users IS 'MergeWong 后台用户';
COMMENT ON COLUMN users.password IS 'bcrypt 密码哈希';

-- ----------------------------------------------------------------------------
-- 4. 动态数据库连接表
-- 对应 internal/models/database.go
-- 注意：当前程序会明文保存动态连接密码，生产使用前应完成加密存储改造。
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS database_connections (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NULL,
  updated_at TIMESTAMPTZ NULL,
  deleted_at TIMESTAMPTZ NULL,
  name VARCHAR(50) NOT NULL,
  type VARCHAR(20) NOT NULL,
  host VARCHAR(100) NOT NULL,
  port BIGINT NOT NULL,
  "database" VARCHAR(100) NOT NULL,
  username VARCHAR(100) NOT NULL,
  password VARCHAR(255) NOT NULL,
  charset VARCHAR(20) NOT NULL DEFAULT 'utf8mb4',
  max_idle BIGINT NOT NULL DEFAULT 10,
  max_open BIGINT NOT NULL DEFAULT 100,
  status BIGINT NOT NULL DEFAULT 1,
  user_id BIGINT NOT NULL,
  CONSTRAINT ck_database_connections_status CHECK (status IN (0, 1))
);

CREATE INDEX IF NOT EXISTS idx_database_connections_deleted_at
  ON database_connections (deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_database_connections_name
  ON database_connections (name);
CREATE INDEX IF NOT EXISTS idx_database_connections_user_id
  ON database_connections (user_id);

COMMENT ON TABLE database_connections IS '源库和目标库连接配置';
COMMENT ON COLUMN database_connections.password IS '数据库密码（当前为明文）';

-- ----------------------------------------------------------------------------
-- 5. 同步任务表
-- 对应 internal/models/sync_task.go
-- 当前 sync_type 支持 full / incremental；CDC 字段将在同步内核实现时迁移。
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sync_tasks (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NULL,
  updated_at TIMESTAMPTZ NULL,
  deleted_at TIMESTAMPTZ NULL,
  name VARCHAR(100) NOT NULL,
  source_db VARCHAR(50) NOT NULL,
  source_table VARCHAR(100) NOT NULL,
  target_db VARCHAR(50) NOT NULL,
  target_table VARCHAR(100) NOT NULL,
  field_mapping JSON NULL,
  sync_type VARCHAR(20) NOT NULL,
  incremental_key VARCHAR(100) NULL,
  cron_expression VARCHAR(100) NULL,
  status BIGINT NOT NULL DEFAULT 1,
  last_run_at TIMESTAMPTZ NULL,
  last_run_status VARCHAR(20) NULL,
  last_run_message TEXT NULL,
  user_id BIGINT NOT NULL,
  CONSTRAINT ck_sync_tasks_sync_type CHECK (sync_type IN ('full', 'incremental')),
  CONSTRAINT ck_sync_tasks_status CHECK (status IN (0, 1))
);

CREATE INDEX IF NOT EXISTS idx_sync_tasks_deleted_at ON sync_tasks (deleted_at);
CREATE INDEX IF NOT EXISTS idx_sync_tasks_user_id ON sync_tasks (user_id);
CREATE INDEX IF NOT EXISTS idx_sync_tasks_status ON sync_tasks (status);

COMMENT ON TABLE sync_tasks IS '数据同步任务';

-- ----------------------------------------------------------------------------
-- 6. 同步运行日志表
-- 对应 internal/models/sync_task.go 中的 SyncLog
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sync_logs (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NULL,
  task_id BIGINT NOT NULL,
  status VARCHAR(20) NOT NULL,
  message TEXT NULL,
  rows_affected BIGINT NOT NULL DEFAULT 0,
  duration BIGINT NOT NULL DEFAULT 0,
  error_detail TEXT NULL,
  CONSTRAINT ck_sync_logs_status CHECK (status IN ('running', 'success', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_sync_logs_task_id ON sync_logs (task_id);
CREATE INDEX IF NOT EXISTS idx_sync_logs_created_at ON sync_logs (created_at);
CREATE INDEX IF NOT EXISTS idx_sync_logs_status ON sync_logs (status);

COMMENT ON TABLE sync_logs IS '同步任务运行日志';
COMMENT ON COLUMN sync_logs.duration IS '执行耗时，毫秒';

-- ----------------------------------------------------------------------------
-- 7. 初始化后台管理员
-- password 是 admin123 的 bcrypt 哈希。
-- 重复执行时只恢复管理员角色和启用状态，不覆盖用户已修改的密码。
-- ----------------------------------------------------------------------------
INSERT INTO users
  (created_at, updated_at, username, password, email, role, status)
VALUES
  (CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'admin',
   '$2b$10$htXk7nUVm2O2qsb49w3Xh.iD9XSBP7UaLFMv/zES9an1Cmh5BuMGW',
   'admin@mergewong.local', 'admin', 1)
ON CONFLICT (username) DO UPDATE SET
  role = EXCLUDED.role,
  status = EXCLUDED.status,
  updated_at = CURRENT_TIMESTAMP;

-- ----------------------------------------------------------------------------
-- 8. 确保应用账号拥有现有及未来对象权限
-- ----------------------------------------------------------------------------
GRANT USAGE, CREATE ON SCHEMA public TO mergewong;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO mergewong;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO mergewong;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT ALL PRIVILEGES ON TABLES TO mergewong;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT ALL PRIVILEGES ON SEQUENCES TO mergewong;

-- ----------------------------------------------------------------------------
-- 9. 初始化结果检查
-- ----------------------------------------------------------------------------
SELECT current_database() AS current_database, current_user AS current_user;
SELECT id, username, email, role, status
FROM users
WHERE username = 'admin';

-- 应用配置参考：
-- databases:
--   system:
--     type: "postgres"
--     host: "你的 PostgreSQL 地址"
--     port: 5432
--     database: "mergewong"
--     username: "mergewong"
--     password: "MergeWong@2026!"
--     max_idle: 10
--     max_open: 100
