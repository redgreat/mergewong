# APIWong - 数据库管理 API 服务

一个功能强大的后台 API 服务，支持多数据库连接、JWT 鉴权、数据 CRUD、跨数据库同步以及定时任务调度。

## 功能特性

✅ **JWT 认证** - 基于 JWT 的用户认证和授权  
✅ **多数据库支持** - 支持 MySQL、PostgreSQL、SQL Server 等多种数据库  
✅ **动态数据库连接** - 运行时动态管理多个数据库连接  
✅ **通用 CRUD API** - 提供统一的增删改查接口  
✅ **数据同步** - 支持全量和增量数据同步，支持字段映射  
✅ **定时任务** - 基于 Cron 表达式的定时同步任务  
✅ **Docker 部署** - 完整的 Docker 和 docker-compose 支持  
✅ **CI/CD** - GitHub Actions 自动构建和发布 Docker 镜像  

## 技术栈

- **框架**: Gin
- **ORM**: GORM
- **认证**: JWT (golang-jwt/jwt)
- **配置**: Viper
- **定时任务**: robfig/cron
- **数据库驱动**: 
  - MySQL: gorm.io/driver/mysql
  - PostgreSQL: gorm.io/driver/postgres
  - SQL Server: gorm.io/driver/sqlserver

## 快速开始

### 使用 Docker Compose（推荐）

1. 克隆项目
```bash
git clone https://github.com/redgreat/apiwong.git
cd apiwong
```

2. 启动服务
```bash
docker-compose up -d
```

3. 访问 API
```bash
curl http://localhost:8080/health
```

### 本地开发

1. 安装依赖
```bash
go mod download
```

2. 配置数据库

编辑 `configs/config.yaml`，配置系统数据库：

```yaml
databases:
  system:
    type: "mysql"
    host: "localhost"
    port: 3306
    database: "apiwong"
    username: "root"
    password: "password"
```

3. 运行服务
```bash
go run cmd/server/main.go
```

## API 文档

### 认证接口

#### 登录
```bash
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin123"
}
```

响应：
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "user_id": 1,
    "username": "admin",
    "role": "admin"
  },
  "timestamp": 1706534283
}
```

#### 注册
```bash
POST /api/auth/register
Content-Type: application/json

{
  "username": "user1",
  "password": "password123",
  "email": "user1@example.com"
}
```

### 数据库操作接口

所有以下接口都需要在请求头中携带 JWT：
```
Authorization: Bearer <token>
```

#### 查询数据
```bash
POST /api/db/:name/query
Content-Type: application/json

{
  "sql": "SELECT * FROM users WHERE status = ?",
  "params": [1],
  "page": 1,
  "page_size": 10
}
```

#### 执行 SQL
```bash
POST /api/db/:name/exec
Content-Type: application/json

{
  "sql": "UPDATE users SET status = ? WHERE id = ?",
  "params": [1, 123]
}
```

#### 列出所有表
```bash
GET /api/db/:name/tables
```

#### 获取表结构
```bash
GET /api/db/:name/table/:table/schema
```

#### 插入数据
```bash
POST /api/db/:name/table/:table/data
Content-Type: application/json

{
  "data": {
    "name": "张三",
    "age": 25,
    "email": "zhangsan@example.com"
  }
}
```

#### 更新数据
```bash
PUT /api/db/:name/table/:table/data/:id
Content-Type: application/json

{
  "data": {
    "age": 26
  }
}
```

#### 删除数据
```bash
DELETE /api/db/:name/table/:table/data/:id
```

### 同步任务接口

#### 创建同步任务
```bash
POST /api/sync/tasks
Content-Type: application/json

{
  "name": "用户数据同步",
  "source_db": "mysql_db",
  "source_table": "users",
  "target_db": "postgres_db",
  "target_table": "users_copy",
  "field_mapping": {
    "user_id": "id",
    "user_name": "name"
  },
  "sync_type": "incremental",
  "incremental_key": "updated_at",
  "cron_expression": "0 */6 * * *"
}
```

#### 列出所有任务
```bash
GET /api/sync/tasks?page=1&page_size=10
```

#### 获取任务详情
```bash
GET /api/sync/tasks/:id
```

#### 更新任务
```bash
PUT /api/sync/tasks/:id
Content-Type: application/json

{
  "status": 0,
  "cron_expression": "0 0 * * *"
}
```

#### 删除任务
```bash
DELETE /api/sync/tasks/:id
```

#### 手动执行任务
```bash
POST /api/sync/tasks/:id/execute
```

#### 查看任务日志
```bash
GET /api/sync/tasks/:id/logs?page=1&page_size=10
```

## 配置说明

### config.yaml 配置文件

```yaml
server:
  port: "8080"
  mode: "debug"  # debug, release, test

jwt:
  secret: "your-secret-key"
  expire_time: 24  # 小时

databases:
  system:  # 系统数据库（必需）
    type: "mysql"
    host: "localhost"
    port: 3306
    database: "apiwong"
    username: "root"
    password: "password"
    charset: "utf8mb4"
    max_idle: 10
    max_open: 100

  # 其他数据库连接
  my_postgres:
    type: "postgres"
    host: "localhost"
    port: 5432
    database: "testdb"
    username: "postgres"
    password: "password"

log:
  level: "info"
  output_path: "logs/app.log"
```

### Cron 表达式示例

```
0 */6 * * *    # 每6小时执行一次
0 0 * * *      # 每天凌晨执行
0 0 * * 0      # 每周日凌晨执行
*/5 * * * *    # 每5分钟执行一次
```

## 环境变量

可以通过环境变量覆盖配置文件中的设置：

```bash
export SERVER_PORT=8080
export JWT_SECRET=my-secret-key
export DATABASES_SYSTEM_HOST=localhost
export DATABASES_SYSTEM_PASSWORD=newpassword
```

## Docker 镜像

### 从 GitHub Container Registry 拉取

```bash
docker pull ghcr.io/redgreat/apiwong:latest
```

### 运行镜像

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/configs:/root/configs \
  -v $(pwd)/logs:/root/logs \
  ghcr.io/redgreat/apiwong:latest
```

## 开发

### 项目结构

```
apiwong/
├── cmd/
│   └── server/          # 程序入口
├── internal/
│   ├── config/          # 配置管理
│   ├── models/          # 数据模型
│   ├── middleware/      # 中间件
│   ├── handlers/        # 请求处理器
│   ├── services/        # 业务逻辑
│   ├── database/        # 数据库管理
│   ├── scheduler/       # 定时任务
│   └── utils/           # 工具函数
├── configs/             # 配置文件
├── Dockerfile           # Docker 构建文件
├── docker-compose.yml   # Docker Compose 配置
└── .github/workflows/   # CI/CD 配置
```

### 添加新的数据库类型支持

1. 在 `internal/database/connector.go` 中添加新的 case
2. 安装对应的 GORM 驱动
3. 更新配置示例

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
