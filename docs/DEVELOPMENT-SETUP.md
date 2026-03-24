# 开发环境搭建指南

**文档版本:** v1.0
**创建时间:** 2026-03-24
**适用范围:** online-game 平台开发者

---

## 目录

1. [系统要求](#系统要求)
2. [基础环境安装](#基础环境安装)
3. [项目配置](#项目配置)
4. [依赖服务](#依赖服务)
5. [开发工具](#开发工具)
6. [IDE 配置](#ide-配置)
7. [常用命令](#常用命令)
8. [故障排除](#故障排除)

---

## 系统要求

### 最低配置

| 组件 | 最低要求 | 推荐配置 |
|------|---------|---------|
| 操作系统 | Ubuntu 20.04+ / macOS 12+ / Windows 10+ | Ubuntu 22.04 LTS / macOS 14+ |
| CPU | 4 核心 | 8 核心 |
| 内存 | 8 GB | 16 GB+ |
| 磁盘空间 | 20 GB 可用空间 | 50 GB+ SSD |
| 网络 | 稳定的互联网连接 | - |

### 软件要求

| 软件 | 最低版本 | 推荐版本 |
|------|---------|---------|
| Go | 1.21 | 1.23+ |
| Docker | 20.10 | 24.0+ |
| Docker Compose | 2.0 | 2.20+ |
| Git | 2.30 | 2.40+ |
| PostgreSQL | 14 | 16+ |
| Redis | 6 | 7+ |

---

## 基础环境安装

### macOS 安装

```bash
# 1. 安装 Homebrew (如果未安装)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 2. 安装 Go
brew install go

# 3. 安装 Docker Desktop
brew install --cask docker

# 4. 安装其他工具
brew install git postgresql redis nats-server
brew install protobuf buf

# 5. 验证安装
go version
docker --version
git --version
```

### Ubuntu/Debian 安装

```bash
# 1. 更新系统
sudo apt update && sudo apt upgrade -y

# 2. 安装 Go
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 3. 安装 Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# 4. 安装其他依赖
sudo apt install -y git postgresql redis-server nats-server protobuf-compiler

# 5. 安装 buf (Protobuf 工具)
go install github.com/bufbuild/buf/cmd/buf@latest

# 6. 验证安装
go version
docker --version
git --version
```

### Windows 安装

```powershell
# 1. 使用 Chocolatey 安装 (以管理员身份运行 PowerShell)
Set-ExecutionPolicy Bypass -Scope Process -Force
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# 2. 安装必要软件
choco install golang docker-desktop git postgresql redis-64 nats-server protoc

# 3. 安装 WSL2 (推荐用于开发)
wsl --install -d Ubuntu-22.04

# 4. 验证安装
go version
docker --version
git --version
```

---

## 项目配置

### 克隆项目

```bash
# 克隆仓库
git clone https://github.com/your-org/online-game.git
cd online-game

# 查看远程仓库
git remote -v

# 添加上游仓库 (如果是 fork)
git remote add upstream https://github.com/original-org/online-game.git
```

### 安装依赖

```bash
# 下载依赖
go mod download

# 验证依赖
go mod verify

# 查看依赖图
go mod graph

# 整理依赖
go mod tidy
```

### 环境变量配置

```bash
# 复制环境变量模板
cp .env.example .env

# 编辑环境变量
nano .env
```

**`.env` 文件示例:**

```env
# 服务配置
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
ENVIRONMENT=development
LOG_LEVEL=debug

# 数据库配置
DB_HOST=localhost
DB_PORT=5432
DB_NAME=online_game
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSL_MODE=disable
DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=10

# Redis 配置
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# NATS 配置
NATS_ADDR=nats://localhost:4222
NATS_SUBJECT=events.>

# JWT 配置
JWT_SECRET=your-secret-key-change-in-production
JWT_EXPIRATION=24h

# 监控配置
METRICS_ENABLED=true
METRICS_PORT=9090
TRACE_ENABLED=true
TRACE_ENDPOINT=http://localhost:4318
```

---

## 依赖服务

### 使用 Docker Compose 启动所有服务

```bash
# 启动所有依赖服务
docker-compose -f docker-compose.deps.yml up -d

# 查看服务状态
docker-compose -f docker-compose.deps.yml ps

# 查看日志
docker-compose -f docker-compose.deps.yml logs -f

# 停止服务
docker-compose -f docker-compose.deps.yml down
```

### 单独安装服务

#### PostgreSQL

```bash
# macOS (使用 Homebrew)
brew install postgresql@16
brew services start postgresql@16

# Ubuntu
sudo apt install postgresql-16
sudo systemctl start postgresql

# 创建数据库
createdb online_game

# 运行迁移
make db-migrate
```

#### Redis

```bash
# macOS
brew install redis
brew services start redis

# Ubuntu
sudo apt install redis-server
sudo systemctl start redis

# 验证连接
redis-cli ping
```

#### NATS

```bash
# macOS
brew install nats-server
nats-server -js &

# Ubuntu
wget https://github.com/nats-io/nats-server/releases/download/v2.10.7/nats-server-v2.10.7-linux-amd64.tar.gz
tar -xzf nats-server-v2.10.7-linux-amd64.tar.gz
sudo mv nats-server-v2.10.7-linux-amd64/nats-server /usr/local/bin/

# 启动 NATS (启用 JetStream)
nats-server -js -p 4222 &
```

---

## 开发工具

### 安装开发工具

```bash
# 使用 Makefile 安装所有工具
make install-tools
```

**手动安装:**

```bash
# 代码检查工具
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 导入格式化工具
go install golang.org/x/tools/cmd/goimports@latest

# 数据库迁移工具
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Protobuf 相关
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
go install github.com/envoyproxy/protoc-gen-validate@latest

# API 文档生成
go install github.com/swaggo/swag/cmd/swag@latest

# 性能分析
go install github.com/google/pprof@latest

# 测试覆盖率
go install github.com/axw/gocov/gocov@latest
```

### 生成 Protobuf 代码

```bash
# 使用 Makefile
make proto-gen

# 或使用脚本
./scripts/proto-gen.sh

# 监听模式 (自动重新生成)
./scripts/proto-gen.sh --watch
```

---

## IDE 配置

### VS Code 配置

**推荐扩展:**

```json
{
  "recommendations": [
    "golang.go",
    "ms-vscode.makefile-tools",
    "redhat.vscode-yaml",
    "eamodio.gitlens",
    "humao.rest-client",
    "zxh404.vscode-proto3"
  ]
}
```

**工作区配置 (`.vscode/settings.json`):**

```json
{
  "go.useLanguageServer": true,
  "go.autocompleteUnimportedPackages": true,
  "go.docsTool": "gogetdoc",
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "go.formatTool": "goimports",
  "go.formatOnSave": true,
  "go.testFlags": ["-v", "-race"],
  "go.testTimeout": "30s",
  "go.buildOnSave": "workspace",
  "go.liveErrors": {
    "enabled": true,
    "delay": 500
  },
  "go.generateTestsFlags": ["-all"],
  "files.watcherExclude": {
    "**/.git/objects/**": true,
    "**/.git/subtree-cache/**": true,
    "**/node_modules/*/**": true,
    "**/.hg/store/**": true,
    "**/build/**": true
  }
}
```

**任务配置 (`.vscode/tasks.json`):**

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "go: build",
      "type": "shell",
      "command": "go",
      "args": ["build", "./..."],
      "group": {
        "kind": "build",
        "isDefault": true
      }
    },
    {
      "label": "go: test",
      "type": "shell",
      "command": "go",
      "args": ["test", "./...", "-v", "-race"],
      "group": {
        "kind": "test",
        "isDefault": true
      }
    },
    {
      "label": "make: dev",
      "type": "shell",
      "command": "make",
      "args": ["dev"]
    }
  ]
}
```

### GoLand 配置

1. **打开项目**
   - File → Open → 选择项目目录

2. **配置 Go SDK**
   - Go to Settings → Go → GOROOT
   - 选择 Go 安装路径

3. **配置运行配置**
   - Run → Edit Configurations
   - 添加新的 Go Build 配置
   - 设置 Working Directory 为项目根目录

4. **启用实时文件监听**
   - Settings → Tools → File Watchers
   - 启用 goimports 和 gofmt

### Vim/Neovim 配置

**使用 vim-go:**

```vim
" 安装 vim-plug (如果未安装)
" https://github.com/junegunn/vim-plug

" .vimrc 或 init.vim
call plug#begin('~/.vim/plugged')
Plug 'fatih/vim-go', { 'do': ':GoUpdateBinaries' }
Plug 'preservim/nerdtree'
Plug 'airblade/vim-gitgutter'
call plug#end()

" 配置
let g:go_fmt_command = "goimports"
let g:go_autodetect_gopath = 0
let g:go_list_type = "quickfix"
```

---

## 常用命令

### 开发命令

```bash
# 启动开发环境
make dev

# 运行测试
make test

# 运行测试并生成覆盖率报告
make test-cover

# 代码格式化
make fmt

# 运行 linter
make lint

# 构建所有服务
make build

# 生成代码
make generate
```

### 数据库命令

```bash
# 启动数据库
make db-up

# 运行迁移
make db-migrate

# 创建新迁移
make db-migrate-create NAME=add_users_table

# 回滚迁移
make db-rollback

# 打开数据库控制台
make db-console

# 重置数据库
make db-reset
```

### Docker 命令

```bash
# 构建 Docker 镜像
make docker-build

# 推送镜像
make docker-push

# 查看容器日志
docker-compose logs -f gateway

# 进入容器
docker-compose exec gateway bash

# 重启服务
docker-compose restart gateway
```

### 监控命令

```bash
# 启动监控服务
make monitor-up

# 查看 Prometheus
open http://localhost:9090

# 查看 Grafana
open http://localhost:3000

# 查看 Jaeger (链路追踪)
open http://localhost:16686
```

---

## 故障排除

### Go 命令找不到

```bash
# 检查 Go 安装
which go

# 如果未找到，添加到 PATH
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$(go env GOPATH)/bin

# 永久添加到 shell 配置
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc
```

### Docker 权限错误

```bash
# Linux: 将用户添加到 docker 组
sudo usermod -aG docker $USER
newgrp docker

# 验证
docker ps
```

### 端口已被占用

```bash
# 查找占用端口的进程
lsof -i :8080
netstat -tuln | grep 8080

# 杀死进程
kill -9 <PID>

# 或修改服务端口
export SERVER_PORT=8081
```

### 数据库连接失败

```bash
# 检查 PostgreSQL 是否运行
pg_isready

# 启动 PostgreSQL
brew services start postgresql@16  # macOS
sudo systemctl start postgresql   # Linux

# 检查连接
psql -U postgres -d online_game
```

### 模块校验和错误

```bash
# 清理模块缓存
go clean -modcache

# 重新下载依赖
go mod download
go mod verify

# 更新 go.sum
go mod tidy
```

### Protobuf 生成失败

```bash
# 检查 protoc 安装
protoc --version

# 检查插件
which protoc-gen-go
which protoc-gen-go-grpc

# 重新安装插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 添加到 PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

---

## 首次运行

### 完整初始化流程

```bash
# 1. 克隆项目
git clone https://github.com/your-org/online-game.git
cd online-game

# 2. 安装开发工具
make install-tools

# 3. 启动依赖服务
docker-compose -f docker-compose.deps.yml up -d

# 4. 配置环境变量
cp .env.example .env
nano .env

# 5. 运行数据库迁移
make db-migrate

# 6. 生成 Protobuf 代码
make proto-gen

# 7. 运行测试
make test

# 8. 启动开发环境
make dev

# 9. 验证服务
curl http://localhost:8080/health
curl http://localhost:8081/health
```

### 验证安装

```bash
# 检查所有服务健康状态
for service in gateway ws-gateway game-service user-service payment-service match-service; do
    echo "Checking $service..."
    curl -s http://localhost:8080/health | jq .
done

# 运行完整测试套件
make test-cover

# 检查代码质量
make lint
```

---

## 下一步

- 阅读 [架构文档](./ARCHITECTURE-IMPLEMENTATION-GUIDE.md)
- 查看 [API 参考](./API-REFERENCE.md)
- 了解 [错误处理](./ERROR-HANDLING-GUIDE.md)
- 参考 [故障排查手册](./TROUBLESHOOTING.md)
