# Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] - 2026-03-23

### Added

#### Phase 1: Core Architecture
- Actor model implementation (`pkg/actor/`)
  - BaseActor with message processing
  - GameActor, PlayerActor, RoomActor
  - Message batching for performance
  - Actor object pooling
  - ActorSystem for lifecycle management

- Dual Engine System (`pkg/engine/`)
  - JavaScript engine (V8 via otto)
  - WebAssembly engine (wasmtime)
  - Intelligent engine selector
  - Performance monitoring
  - Automatic switching based on metrics

#### Phase 2: Microservices (12 services)
- user-service (8001) - User management
- game-service (8002) - Game logic
- payment-service (8003) - Payment processing
- player-service (8004) - Player statistics
- activity-service (8005) - Activity management
- guild-service (8006) - Guild system
- item-service (8007) - Item management
- notification-service (8008) - Notifications
- organization-service (8009) - Organization management
- permission-service (8010) - Permission control
- id-service (8011) - ID generation (Snowflake)
- file-service (8012) - File storage

#### Phase 3: Database Optimization
- Database initialization scripts (`scripts/init.sql`)
  - 6 PostgreSQL databases
  - Complete table schemas with indexes
- Connection pool optimization (`pkg/db/pool.go`)
  - Multi-pool management
  - Health checks
  - Performance metrics

#### Phase 4: Performance Optimization
- Redis cache layer (`pkg/cache/`)
  - Generic cache operations
  - Hash, List, Set, Sorted Set support
  - Distributed locking
  - Cache invalidation strategies
- Kafka message queue (`pkg/kafka/`)
  - Producer/Consumer patterns
  - Message batching
  - Event bus abstraction
- WebSocket Gateway (`pkg/websocket/`)
  - Room management
  - Message broadcasting
  - Connection pooling

#### Phase 5: Testing & Deployment
- Integration tests (`tests/integration_test.go`)
- Performance benchmarks (`tests/performance_test.go`)
- Docker deployment (`deploy/docker-compose.yml`)
- 13 service Dockerfiles
- Deployment scripts
- Health checks (`pkg/health/`)

#### Additional Features
- API Gateway (`internal/gateway/`)
  - Service routing
  - Rate limiting
  - Circuit breaker
  - Load balancing
- CLI tool (`cmd/gamectl`)
  - Service management
  - Health checks
  - Log viewing
- Development tools
  - Makefile with 20+ commands
  - Hot reload (air.toml)
  - VSCode configuration

#### Documentation
- README.md with architecture diagram
- API Reference (`docs/API-REFERENCE.md`)
- Deployment Guide (`docs/DEPLOYMENT.md`)

#### Example Games
- Texas Hold'em Poker (`examples/games/texas-holdem.js`)
- Blackjack (`examples/games/blackjack.js`)

### Performance Benchmarks

| Operation | Performance |
|-----------|-------------|
| Actor message send | 1051 ns/op |
| Message batching | 197 ns/op |
| Engine selector | 0.94 ns/op |
| Concurrent messages | 190 ns/op |
| Connection pool query | 93 ns/op |
| JS engine execution | 100 μs/op |
| JSON marshal | 780 ns/op |

### Dependencies

- github.com/gin-gonic/gin
- github.com/IBM/sarama
- github.com/gorilla/websocket
- github.com/redis/go-redis/v9
- gorm.io/gorm
- github.com/robertkrimen/otto
- github.com/bytecodealliance/wasmtime-go

### Project Statistics

- 74 Go files
- ~12,000 lines of code
- 13 services
- 6 databases
- Complete deployment configuration

---

## [Unreleased]

### Planned

- Real-time game lobby
- Tournament system
- Leaderboard service
- Chat service
- Replay system
- Analytics service
- Admin panel
- Mobile SDK
