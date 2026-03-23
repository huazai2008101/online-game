#!/bin/bash
# Generate example configuration files for all services

CONFIG_DIR="deploy/config"
mkdir -p "$CONFIG_DIR"

# User Service
cat > "$CONFIG_DIR/user-service.env" <<'EOF'
# User Service Configuration
PORT=8001
MODE=debug
DB_HOST=localhost
DB_PORT=5432
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_platform_db
REDIS_HOST=localhost
REDIS_PORT=6379
JWT_SECRET=your-secret-key-change-in-production
JWT_EXPIRATION=24h
KAFKA_BROKERS=localhost:9092
LOG_LEVEL=info
EOF

# Game Service
cat > "$CONFIG_DIR/game-service.env" <<'EOF'
# Game Service Configuration
PORT=8002
MODE=debug
DB_HOST=localhost
DB_PORT=5433
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_core_db
REDIS_HOST=localhost
REDIS_PORT=6379
KAFKA_BROKERS=localhost:9092
ACTOR_POOL_SIZE=100
MESSAGE_BATCH_SIZE=100
LOG_LEVEL=info
EOF

# Payment Service
cat > "$CONFIG_DIR/payment-service.env" <<'EOF'
# Payment Service Configuration
PORT=8003
MODE=debug
DB_HOST=localhost
DB_PORT=5434
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_payment_db
REDIS_HOST=localhost
REDIS_PORT=6379
KAFKA_BROKERS=localhost:9092
LOG_LEVEL=info
EOF

# Player Service
cat > "$CONFIG_DIR/player-service.env" <<'EOF'
# Player Service Configuration
PORT=8004
MODE=debug
DB_HOST=localhost
DB_PORT=5433
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_core_db
REDIS_HOST=localhost
REDIS_PORT=6379
LOG_LEVEL=info
EOF

# Activity Service
cat > "$CONFIG_DIR/activity-service.env" <<'EOF'
# Activity Service Configuration
PORT=8005
MODE=debug
DB_HOST=localhost
DB_PORT=5433
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_core_db
REDIS_HOST=localhost
REDIS_PORT=6379
LOG_LEVEL=info
EOF

# Guild Service
cat > "$CONFIG_DIR/guild-service.env" <<'EOF'
# Guild Service Configuration
PORT=8006
MODE=debug
DB_HOST=localhost
DB_PORT=5433
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_core_db
REDIS_HOST=localhost
REDIS_PORT=6379
KAFKA_BROKERS=localhost:9092
LOG_LEVEL=info
EOF

# Item Service
cat > "$CONFIG_DIR/item-service.env" <<'EOF'
# Item Service Configuration
PORT=8007
MODE=debug
DB_HOST=localhost
DB_PORT=5433
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_core_db
REDIS_HOST=localhost
REDIS_PORT=6379
LOG_LEVEL=info
EOF

# Notification Service
cat > "$CONFIG_DIR/notification-service.env" <<'EOF'
# Notification Service Configuration
PORT=8008
MODE=debug
DB_HOST=localhost
DB_PORT=5435
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_notification_db
REDIS_HOST=localhost
REDIS_PORT=6379
KAFKA_BROKERS=localhost:9092
LOG_LEVEL=info
EOF

# Organization Service
cat > "$CONFIG_DIR/organization-service.env" <<'EOF'
# Organization Service Configuration
PORT=8009
MODE=debug
DB_HOST=localhost
DB_PORT=5432
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_platform_db
REDIS_HOST=localhost
REDIS_PORT=6379
LOG_LEVEL=info
EOF

# Permission Service
cat > "$CONFIG_DIR/permission-service.env" <<'EOF'
# Permission Service Configuration
PORT=8010
MODE=debug
DB_HOST=localhost
DB_PORT=5432
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_platform_db
REDIS_HOST=localhost
REDIS_PORT=6379
LOG_LEVEL=info
EOF

# ID Service
cat > "$CONFIG_DIR/id-service.env" <<'EOF'
# ID Service Configuration
PORT=8011
MODE=debug
REDIS_HOST=localhost
REDIS_PORT=6379
LOG_LEVEL=info
EOF

# File Service
cat > "$CONFIG_DIR/file-service.env" <<'EOF'
# File Service Configuration
PORT=8012
MODE=debug
DB_HOST=localhost
DB_PORT=5436
DB_USER=gameuser
DB_PASS=gamepass
DB_NAME=game_file_db
REDIS_HOST=localhost
REDIS_PORT=6379
STORAGE_PATH=./data/uploads
MAX_FILE_SIZE=104857600
LOG_LEVEL=info
EOF

# API Gateway
cat > "$CONFIG_DIR/gateway.env" <<'EOF'
# API Gateway Configuration
PORT=8080
MODE=release
USER_SERVICE_URL=http://user-service:8001
GAME_SERVICE_URL=http://game-service:8002
PAYMENT_SERVICE_URL=http://payment-service:8003
PLAYER_SERVICE_URL=http://player-service:8004
ACTIVITY_SERVICE_URL=http://activity-service:8005
GUILD_SERVICE_URL=http://guild-service:8006
ITEM_SERVICE_URL=http://item-service:8007
NOTIFICATION_SERVICE_URL=http://notification-service:8008
ORGANIZATION_SERVICE_URL=http://organization-service:8009
PERMISSION_SERVICE_URL=http://permission-service:8010
ID_SERVICE_URL=http://id-service:8011
FILE_SERVICE_URL=http://file-service:8012
RATE_LIMIT_REQUESTS=1000
RATE_LIMIT_WINDOW=1h
CIRCUIT_BREAKER_MAX_FAILURES=5
CIRCUIT_BREAKER_RESET_TIMEOUT=30s
READ_TIMEOUT=10s
WRITE_TIMEOUT=10s
SHUTDOWN_TIMEOUT=30s
EOF

echo "Configuration files generated in $CONFIG_DIR/"
ls -la "$CONFIG_DIR/"
