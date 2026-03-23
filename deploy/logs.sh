#!/bin/bash
# Logs script for viewing service logs

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
COMPOSE_FILE="deploy/docker-compose.yml"
PROJECT_NAME="game-platform"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Determine docker compose command
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Show usage
show_usage() {
    echo "Usage: $0 [service-name] [options]"
    echo ""
    echo "Services:"
    echo "  user-service        User management service"
    echo "  game-service        Game logic service"
    echo "  payment-service     Payment processing service"
    echo "  player-service      Player statistics service"
    echo "  activity-service    Activity management service"
    echo "  guild-service       Guild management service"
    echo "  item-service        Item management service"
    echo "  notification        Notification service"
    echo "  organization        Organization management service"
    echo "  permission          Permission management service"
    echo "  id-service          ID generation service"
    echo "  file-service        File storage service"
    echo "  api-gateway         API Gateway"
    echo "  all                 All services (default)"
    echo ""
    echo "Options:"
    echo "  -f, --follow        Follow log output (default)"
    echo "  -n, --lines NUM     Number of lines to show (default: 100)"
    echo "  --tail              Show last lines only"
    echo ""
    echo "Examples:"
    echo "  $0 game-service         # Follow logs for game-service"
    echo "  $0 all -n 500           # Show last 500 lines for all services"
    echo "  $0 payment-service -f   # Follow logs for payment-service"
}

# Default options
FOLLOW=true
LINES="100"
SERVICE=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--follow)
            FOLLOW=true
            shift
            ;;
        -n|--lines)
            LINES="$2"
            shift 2
            ;;
        --tail)
            FOLLOW=false
            shift
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        user-service|game-service|payment-service|player-service|activity-service|guild-service|item-service|notification-service|organization-service|permission-service|id-service|file-service|api-gateway|all)
            SERVICE="$1"
            shift
            ;;
        *)
            log_warn "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Build docker compose logs command
LOGS_CMD="$DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME logs"

if [ "$FOLLOW" = true ]; then
    LOGS_CMD="$LOGS_CMD -f"
fi

LOGS_CMD="$LOGS_CMD --tail=$LINES"

if [ -n "$SERVICE" ] && [ "$SERVICE" != "all" ]; then
    LOGS_CMD="$LOGS_CMD $SERVICE"
fi

# Show logs
log_info "Showing logs for: ${SERVICE:-all services}"
log_info "Press Ctrl+C to exit..."
echo ""

$LOGS_CMD
