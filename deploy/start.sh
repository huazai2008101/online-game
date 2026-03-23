#!/bin/bash
# Deployment script for the game platform

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="deploy/docker-compose.yml"
PROJECT_NAME="game-platform"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is installed
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    log_info "Docker is installed: $(docker --version)"
}

# Check if Docker Compose is installed
check_docker_compose() {
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    if docker compose version &> /dev/null; then
        DOCKER_COMPOSE="docker compose"
    else
        DOCKER_COMPOSE="docker-compose"
    fi
    log_info "Docker Compose is available"
}

# Generate Dockerfiles if they don't exist
generate_dockerfiles() {
    if [ ! -d "deploy/docker" ]; then
        log_info "Creating deploy/docker directory..."
        mkdir -p deploy/docker
    fi

    if [ ! -f "deploy/docker/Dockerfile.user" ]; then
        log_info "Generating Dockerfiles..."
        ./deploy/generate-dockerfiles.sh
    fi
}

# Start infrastructure services
start_infrastructure() {
    log_info "Starting infrastructure services (PostgreSQL, Redis, Kafka, etc.)..."
    $DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME up -d postgres-platform postgres-core postgres-payment postgres-notification postgres-file redis zookeeper kafka nats

    log_info "Waiting for infrastructure services to be ready..."
    sleep 30
}

# Start monitoring services
start_monitoring() {
    log_info "Starting monitoring services (Prometheus, Grafana, Jaeger)..."
    $DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME up -d prometheus grafana jaeger
}

# Start game services
start_services() {
    log_info "Starting game services..."
    $DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME up -d user-service game-service payment-service player-service activity-service guild-service item-service notification-service organization-service permission-service id-service file-service

    log_info "Waiting for services to be ready..."
    sleep 15
}

# Start API Gateway
start_gateway() {
    log_info "Starting API Gateway..."
    $DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME up -d api-gateway
}

# Show service status
show_status() {
    log_info "Service status:"
    $DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME ps
}

# Show service URLs
show_urls() {
    log_info ""
    log_info "=================================="
    log_info "Service URLs:"
    log_info "=================================="
    log_info "API Gateway:       http://localhost:8080"
    log_info "User Service:      http://localhost:8001"
    log_info "Game Service:      http://localhost:8002"
    log_info "Payment Service:   http://localhost:8003"
    log_info "Player Service:    http://localhost:8004"
    log_info "Activity Service:  http://localhost:8005"
    log_info "Guild Service:     http://localhost:8006"
    log_info "Item Service:      http://localhost:8007"
    log_info "Notification:      http://localhost:8008"
    log_info "Organization:      http://localhost:8009"
    log_info "Permission:        http://localhost:8010"
    log_info "ID Service:        http://localhost:8011"
    log_info "File Service:      http://localhost:8012"
    log_info ""
    log_info "Prometheus:        http://localhost:9090"
    log_info "Grafana:           http://localhost:3000 (admin/admin)"
    log_info "Jaeger UI:         http://localhost:16686"
    log_info "=================================="
}

# Main deployment function
deploy() {
    log_info "Starting deployment of $PROJECT_NAME..."
    echo ""

    check_docker
    check_docker_compose
    generate_dockerfiles

    # Parse command line arguments
    SKIP_INFRASTRUCTURE=false
    SKIP_MONITORING=false
    SKIP_SERVICES=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-infrastructure)
                SKIP_INFRASTRUCTURE=true
                shift
                ;;
            --skip-monitoring)
                SKIP_MONITORING=true
                shift
                ;;
            --skip-services)
                SKIP_SERVICES=true
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                echo "Usage: $0 [--skip-infrastructure] [--skip-monitoring] [--skip-services]"
                exit 1
                ;;
        esac
    done

    # Start services
    if [ "$SKIP_INFRASTRUCTURE" = false ]; then
        start_infrastructure
    fi

    if [ "$SKIP_MONITORING" = false ]; then
        start_monitoring
    fi

    if [ "$SKIP_SERVICES" = false ]; then
        start_services
        start_gateway
    fi

    show_status
    show_urls

    log_info "Deployment completed successfully!"
}

# Run deployment
deploy "$@"
