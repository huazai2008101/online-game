# Online Game Platform

> 基于 Actor 模型和双引擎系统的高性能在线游戏平台

## 特性

- **Actor 模型**: 高并发消息驱动架构
- **双引擎系统**: JavaScript (V8) + WebAssembly 智能切换
- **微服务架构**: 12 个独立服务，职责清晰
- **高性能**: Redis 缓存、连接池、消息批处理
- **可扩展**: 支持水平扩展和负载均衡
- **可观测**: Prometheus + Grafana + Jaeger 监控追踪

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         API Gateway (8080)                       │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼────────┐
│ User Service   │  │  Game Service   │  │ Payment Service│
│    (8001)      │  │    (8002)       │  │    (8003)      │
└────────────────┘  └─────────────────┘  └────────────────┘
        │                     │                     │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼────────┐
│ Player Service │  │ Activity Service│  │ Guild Service  │
│    (8004)      │  │    (8005)       │  │    (8006)      │
└────────────────┘  └─────────────────┘  └────────────────┘
        │                     │                     │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼────────┐
│  Item Service  │  │ Notification    │  │ Organization  │
│    (8007)      │  │    (8008)       │  │    (8009)      │
└────────────────┘  └─────────────────┘  └────────────────┘
        │                     │                     │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼────────┐
│  Permission    │  │   ID Service    │  │  File Service  │
│    (8010)      │  │    (8011)       │  │    (8012)      │
└────────────────┘  └─────────────────┘  └────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼────────┐
│   PostgreSQL  │  │     Redis       │  │     Kafka      │
│  (6 databases) │  │     Cache       │  │   Message Q    │
└────────────────┘  └─────────────────┘  └────────────────┘
```

## 目录结构

```
online-game/
├── cmd/                    # 服务入口
│   ├── user-service/       # 用户服务
│   ├── game-service/       # 游戏服务
│   ├── payment-service/    # 支付服务
│   ├── player-service/     # 玩家服务
│   ├── activity-service/   # 活动服务
│   ├── guild-service/      # 公会服务
│   ├── item-service/       # 道具服务
│   ├── notification/       # 通知服务
│   ├── organization/       # 组织服务
│   ├── permission/         # 权限服务
│   ├── id-service/         # ID 服务
│   ├── file-service/       # 文件服务
│   └── api-gateway/        # API 网关
├── internal/               # 内部实现
│   ├── user/               # 用户服务逻辑
│   ├── game/               # 游戏服务逻辑
│   ├── payment/            # 支付服务逻辑
│   ├── gateway/            # 网关实现
│   └── server/             # HTTP 服务器框架
├── pkg/                    # 公共包
│   ├── actor/              # Actor 模型实现
│   ├── engine/             # 双引擎系统
│   ├── cache/              # 缓存层
│   ├── db/                 # 数据库连接池
│   ├── health/             # 健康检查
│   ├── kafka/              # Kafka 集成
│   ├── websocket/          # WebSocket 网关
│   ├── config/             # 配置管理
│   ├── logger/             # 日志
│   └── api/                # API 响应工具
├── deploy/                 # 部署配置
│   ├── docker-compose.yml  # Docker 编排
│   ├── docker/             # Dockerfile
│   ├── start.sh            # 启动脚本
│   └── stop.sh             # 停止脚本
├── scripts/                # 脚本
│   └── init.sql            # 数据库初始化
├── tests/                  # 测试
│   ├── integration_test.go # 集成测试
│   └── performance_test.go # 性能测试
└── docs/                   # 文档
    ├── API-REFERENCE.md    # API 参考
    └── DEPLOYMENT.md       # 部署指南
```

## 快速开始

### 前置要求

- Go 1.23+
- Docker & Docker Compose
- 8GB+ RAM
- 50GB+ 磁盘空间

### 本地开发

```bash
# 克隆项目
git clone https://github.com/your-org/online-game.git
cd online-game

# 安装依赖
go mod download

# 运行测试
go test ./...

# 启动单个服务 (例如用户服务)
go run ./cmd/user-service/main.go
```

### Docker 部署

```bash
# 一键启动所有服务
./deploy/start.sh

# 查看日志
./deploy/logs.sh

# 停止所有服务
./deploy/stop.sh
```

### 访问服务

| 服务 | 地址 |
|------|------|
| API Gateway | http://localhost:8080 |
| Grafana | http://localhost:3000 (admin/admin) |
| Prometheus | http://localhost:9090 |
| Jaeger UI | http://localhost:16686 |

## API 示例

### 用户注册

```bash
curl -X POST http://localhost:8080/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "player1",
    "password": "securepass",
    "email": "player1@example.com",
    "nickname": "Player One"
  }'
```

### 用户登录

```bash
curl -X POST http://localhost:8080/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "player1",
    "password": "securepass"
  }'
```

### 获取游戏列表

```bash
curl http://localhost:8080/api/v1/games \
  -H "Authorization: Bearer <token>"
```

### 创建游戏房间

```bash
curl -X POST http://localhost:8080/api/v1/games/rooms \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "game_id": 1,
    "room_name": "My Room",
    "max_players": 6
  }'
```

## 性能基准

| 指标 | 结果 |
|------|------|
| Actor 消息发送 | 1051 ns/op |
| 消息批处理 | 197 ns/op |
| 引擎选择器 | 0.94 ns/op |
| 并发消息 | 190 ns/op |
| 连接池查询 | 93 ns/op |

## 开发指南

### 添加新服务

1. 在 `internal/` 创建服务目录
2. 在 `cmd/` 创建服务入口
3. 在 `internal/gateway/gateway.go` 注册后端
4. 更新 `deploy/docker-compose.yml`

### 编写测试

```bash
# 运行所有测试
go test ./...

# 运行性能测试
go test ./tests/... -bench=. -benchtime=1s

# 运行集成测试
go test ./tests/... -v
```

## 部署

### 生产环境配置

1. 修改数据库密码
2. 配置 Redis 集群
3. 启用 TLS/SSL
4. 配置负载均衡
5. 设置监控告警

详见 [部署指南](docs/DEPLOYMENT.md)

## 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.23 |
| Web 框架 | Gin |
| 数据库 | PostgreSQL 15 |
| 缓存 | Redis 7 |
| 消息队列 | Kafka 3.5 |
| JS 引擎 | otto (V8) |
| WASM 引擎 | wasmtime |
| 容器 | Docker |
| 监控 | Prometheus + Grafana |
| 追踪 | Jaeger |

## 许可证

MIT License

## 贡献

欢迎提交 Pull Request！

## 联系方式

- 项目主页: https://github.com/your-org/online-game
- 问题反馈: https://github.com/your-org/online-game/issues
