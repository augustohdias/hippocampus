#!/usr/bin/env bash
# Hippocampus Startup Hook
# Auto-starts Qdrant and Ollama services when OpenCode starts.
# Add this to your shell profile or run manually.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    log_info "Please install Docker: https://docs.docker.com/engine/install/"
    exit 1
fi

# Check Docker daemon
if ! docker info &> /dev/null; then
    log_error "Docker daemon is not running. Please start Docker."
    exit 1
fi
log_success "Docker daemon is running"

# Determine docker compose command
if command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
elif docker compose version &> /dev/null; then
    COMPOSE_CMD="docker compose"
else
    log_error "Neither docker-compose nor docker compose plugin found"
    log_info "Install Docker Compose: https://docs.docker.com/compose/install/"
    exit 1
fi

# Check if Qdrant container is running
if $COMPOSE_CMD ps -q qdrant 2>/dev/null | grep -q .; then
    log_success "Qdrant container is already running"
else
    log_info "Starting Qdrant container..."
    $COMPOSE_CMD up -d qdrant
    log_success "Qdrant container started"
    # Wait for Qdrant to be ready
    sleep 5
fi

# Check Ollama
if ! command -v ollama &> /dev/null; then
    log_warn "Ollama is not installed or not in PATH"
    log_info "Please install Ollama: https://ollama.com/download"
    exit 1
fi

# Check if Ollama service is running
if ollama list &> /dev/null; then
    log_success "Ollama service is running"
else
    log_warn "Ollama service not running. Attempting to start..."
    # Start ollama serve in background
    nohup ollama serve > /dev/null 2>&1 &
    sleep 3
    # Verify it's up
    if ollama list &> /dev/null; then
        log_success "Ollama service started"
    else
        log_error "Failed to start Ollama service"
        exit 1
    fi
fi

# Check embedding model
log_info "Checking for embedding model..."
if ollama list | grep -q "qwen3-embedding:4b"; then
    log_success "Embedding model already present"
else
    log_info "Pulling qwen3-embedding:4b model (this may take a few minutes)..."
    ollama pull qwen3-embedding:4b
    log_success "Embedding model pulled successfully"
fi

log_success "Hippocampus services are ready!"
echo
echo "To automatically run this hook when OpenCode starts, add the following to your shell profile (~/.bashrc, ~/.zshrc):"
echo "  source $(pwd)/setup/hippocampus-startup-hook.sh"
echo
echo "Or run it manually each time before using hippocampus."