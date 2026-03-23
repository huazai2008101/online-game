#!/bin/bash
# Restart script for the game platform

# Colors for output
GREEN='\033[0;32m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_info "Restarting game platform..."
./deploy/stop.sh "$@"
sleep 5
./deploy/start.sh "$@"

log_info "Restart completed."
