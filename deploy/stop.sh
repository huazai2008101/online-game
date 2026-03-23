#!/bin/bash
# Stop script for the game platform

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="deploy/docker-compose.yml"
PROJECT_NAME="game-platform"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Determine docker compose command
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Stop services
log_info "Stopping $PROJECT_NAME services..."
$DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME down

# Optional: Remove volumes
if [ "$1" == "--volumes" ]; then
    log_info "Removing volumes..."
    $DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME down -v
fi

log_info "All services stopped."
