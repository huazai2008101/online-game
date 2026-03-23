.PHONY: help build test run clean docker-up docker-down docker-logs fmt lint deps

# 默认目标
help:
	@echo "Online Game Platform - Makefile"
	@echo ""
	@echo "可用命令:"
	@echo "  make build          - 编译所有服务"
	@echo "  make test           - 运行测试"
	@echo "  make run            - 运行API网关"
	@echo "  make clean          - 清理编译文件"
	@echo "  make fmt            - 格式化代码"
	@echo "  make lint           - 代码检查"
	@echo "  make deps           - 安装依赖"
	@echo "  make docker-up      - 启动Docker服务"
	@echo "  make docker-down    - 停止Docker服务"
	@echo "  make docker-logs    - 查看服务日志"
	@echo ""
	@echo "服务相关:"
	@echo "  make user-service   - 运行用户服务"
	@echo "  make game-service   - 运行游戏服务"
	@echo "  make gateway        - 运行API网关"

# 编译所有服务
build:
	@echo "编译所有服务..."
	@go build -o bin/user-service ./cmd/user-service
	@go build -o bin/game-service ./cmd/game-service
	@go build -o bin/payment-service ./cmd/payment-service
	@go build -o bin/player-service ./cmd/player-service
	@go build -o bin/activity-service ./cmd/activity-service
	@go build -o bin/guild-service ./cmd/guild-service
	@go build -o bin/item-service ./cmd/item-service
	@go build -o bin/notification-service ./cmd/notification-service
	@go build -o bin/organization-service ./cmd/organization-service
	@go build -o bin/permission-service ./cmd/permission-service
	@go build -o bin/id-service ./cmd/id-service
	@go build -o bin/file-service ./cmd/file-service
	@go build -o bin/api-gateway ./cmd/api-gateway
	@echo "编译完成!"

# 运行测试
test:
	@echo "运行测试..."
	@go test -v ./...

# 运行性能测试
bench:
	@echo "运行性能测试..."
	@go test ./tests/... -bench=. -benchtime=1s

# 运行API网关
run: build
	@echo "启动API网关..."
	@./bin/api-gateway

# 运行用户服务
user-service:
	@echo "启动用户服务..."
	@go run ./cmd/user-service/main.go

# 运行游戏服务
game-service:
	@echo "启动游戏服务..."
	@go run ./cmd/game-service/main.go

# 运行支付服务
payment-service:
	@echo "启动支付服务..."
	@go run ./cmd/payment-service/main.go

# 运行API网关
gateway:
	@echo "启动API网关..."
	@go run ./cmd/api-gateway/main.go

# 清理编译文件
clean:
	@echo "清理编译文件..."
	@rm -rf bin/
	@find . -name "*.log" -delete
	@echo "清理完成!"

# 格式化代码
fmt:
	@echo "格式化代码..."
	@go fmt ./...

# 代码检查
lint:
	@echo "代码检查..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint未安装，跳过代码检查"; \
		echo "安装: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# 安装依赖
deps:
	@echo "安装依赖..."
	@go mod download
	@go mod tidy

# Docker操作
docker-up:
	@echo "启动Docker服务..."
	@./deploy/start.sh

docker-down:
	@echo "停止Docker服务..."
	@./deploy/stop.sh

docker-logs:
	@echo "查看服务日志..."
	@./deploy/logs.sh

docker-restart:
	@echo "重启Docker服务..."
	@./deploy/restart.sh

# 数据库初始化
db-init:
	@echo "初始化数据库..."
	@docker exec -i game-platform-db psql -U gameuser -d game_platform_db < scripts/init.sql

# 生成配置
config:
	@echo "生成配置文件..."
	@./deploy/generate-configs.sh

# 安装工具
install-tools:
	@echo "安装开发工具..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/air-verse/air@latest
	@echo "工具安装完成!"

# 热重载开发
dev:
	@echo "启动热重载开发环境..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air未安装，运行: make install-tools"; \
	fi

# 查看服务状态
status:
	@echo "服务状态:"
	@docker-compose -f deploy/docker-compose.yml -p game-platform ps

# 查看服务健康检查
health:
	@echo "服务健康检查:"
	@curl -s http://localhost:8080/health | jq . || echo "网关未运行"

# 构建Docker镜像
docker-build:
	@echo "构建Docker镜像..."
	@docker-compose -f deploy/docker-compose.yml -p game-platform build

# 推送Docker镜像
docker-push:
	@echo "推送Docker镜像..."
	@docker-compose -f deploy/docker-compose.yml -p game-platform push

# 生成API文档
api-docs:
	@echo "生成API文档..."
	@swag init -g cmd/api-gateway/main.go -o docs/

# 运行演示
demo:
	@echo "启动Actor模型和双引擎演示..."
	@go run ./cmd/demo/main.go

# 代码覆盖率
coverage:
	@echo "生成代码覆盖率报告..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告: coverage.html"

# 性能分析
pprof:
	@echo "启动性能分析..."
	@go tool pprof http://localhost:8080/debug/pprof/profile

# 清理Docker资源
docker-clean:
	@echo "清理Docker资源..."
	@docker-compose -f deploy/docker-compose.yml -p game-platform down -v
	@docker system prune -f

# 查看Docker资源使用
docker-stats:
	@echo "Docker资源使用:"
	@docker stats --no-stream

# 查看Git状态
git-status:
	@echo "Git状态:"
	@git status

# Git提交
git-commit:
	@echo "提交更改..."
	@git add -A
	@git commit -m "更新代码"

# Git推送
git-push:
	@echo "推送代码..."
	@git push origin main

# 完整的CI流程
ci: deps fmt lint test build
	@echo "CI流程完成!"

# 创建发布版本
release: clean deps test build
	@echo "创建发布版本..."
	@mkdir -p release
	@cp bin/* release/
	@cp -r deploy/* release/
	@cp README.md release/
	@echo "发布版本创建完成: release/"
