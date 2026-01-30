.PHONY: help build run clean docker-build docker-up docker-down test

help: ## 显示帮助信息
	@echo "可用命令："
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## 编译应用
	@echo "编译应用..."
	go build -o apiwong ./cmd/server

run: ## 运行应用
	@echo "运行应用..."
	go run ./cmd/server/main.go

clean: ## 清理编译产物
	@echo "清理编译产物..."
	rm -f apiwong
	rm -rf logs/*.log

test: ## 运行测试
	@echo "运行测试..."
	go test -v ./...

tidy: ## 整理依赖
	@echo "整理依赖..."
	go mod tidy

docker-build: ## 构建 Docker 镜像
	@echo "构建 Docker 镜像..."
	docker build -t apiwong:latest .

docker-up: ## 启动 Docker Compose
	@echo "启动 Docker Compose..."
	docker-compose up -d

docker-down: ## 停止 Docker Compose
	@echo "停止 Docker Compose..."
	docker-compose down

docker-logs: ## 查看 Docker 日志
	@echo "查看 Docker 日志..."
	docker-compose logs -f api

lint: ## 运行代码检查
	@echo "运行代码检查..."
	golangci-lint run

fmt: ## 格式化代码
	@echo "格式化代码..."
	go fmt ./...

migrate: ## 运行数据库迁移
	@echo "运行数据库迁移..."
	go run ./cmd/server/main.go
