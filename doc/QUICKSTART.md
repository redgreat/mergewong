# MergeWong 快速开始指南

本指南用于快速启动当前 MergeWong 原型。同步能力与生产限制请先阅读 [README](../README.md) 和 [架构说明](ARCHITECTURE.md)。

## 前提条件

- Go 1.21+ 已安装
- PostgreSQL 14+ 数据库可访问（用于管理库）
- 已配置 `configs/config.yaml` 文件

## 启动步骤

### 1. 配置数据库

编辑 `configs/config.yaml`，配置系统数据库连接信息：

```yaml
databases:
  system:
    type: "postgres"
    host: "your-postgres-host"
    port: 5432
    database: "mergewong"
    username: "mergewong"
    password: "your-password"
    charset: "utf8"
    max_idle: 10
    max_open: 100
```

### 2. 编译项目

```bash
make build
# 或者（当前二进制名仍沿用旧项目名称，计划在 P0 统一）
go build -o apiwong ./cmd/server
```

### 3. 启动服务

```bash
./start.sh
# 或者直接运行
./apiwong
```

首次启动时，系统会自动：
- 检查数据库连接
- 创建必要的表结构
- 初始化默认管理员账户（用户名: admin，密码: admin123）

### 4. 测试 API

使用测试脚本验证服务是否正常运行：

```bash
./test-api.sh
```

或手动测试：

```bash
# 健康检查
curl http://localhost:8080/health

# 登录获取 token
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

## 首次登录

1. 使用默认管理员账户登录
   - 用户名: `admin`
   - 密码: `admin123`

2. 登录后立即修改密码：

```bash
curl -X PUT http://localhost:8080/api/profile \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "password": "your-new-password"
  }'
```

## 常用命令

```bash
# 编译项目
make build

# 运行项目
make run

# 查看帮助
make help

# 使用 Docker Compose
make docker-up    # 启动所有服务
make docker-down  # 停止所有服务
make docker-logs  # 查看日志
```

## 常见问题

### 1. 数据库连接失败

检查配置文件中的数据库连接信息是否正确，确保：
- 数据库服务正在运行
- 网络连接正常
- 用户名和密码正确
- 数据库已创建

### 2. 端口被占用

如果 8080 端口被占用，可以修改 `configs/config.yaml` 中的端口配置：

```yaml
server:
  port: "8081"  # 修改为其他端口
```

### 3. 权限问题

确保对以下目录有写权限：
- `logs/` - 日志目录
- 当前目录 - 编译产物

## 下一步

- 查看 [README.md](../README.md) 了解项目状态和实现逻辑
- 配置更多数据库连接
- 创建同步任务
- 设置定时任务

## 需要帮助？

如遇到问题，请检查：
1. 日志文件 `logs/app.log`
2. 控制台输出
3. GitHub Issues
