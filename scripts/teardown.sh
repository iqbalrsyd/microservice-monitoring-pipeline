#!/bin/bash

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_status "🛑 Tearing down Microservices Pipeline"
echo

# Stop and remove containers
print_status "📦 Stopping and removing containers..."
docker-compose down -v

# Remove networks
print_status "🌐 Removing Docker networks..."
docker network prune -f

# Remove unused images
print_status "🧹 Cleaning up unused Docker images..."
docker image prune -f

# Remove volumes (be careful with this)
read -p "Do you want to remove all Docker volumes? This will delete all data! (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_status "💾 Removing Docker volumes..."
    docker volume prune -f
fi

# Clean up log files
print_status "🗂️ Cleaning up log files..."
rm -rf logs/*
rm -rf data/*

print_success "✅ Teardown completed!"
echo

print_status "To start the services again, run: ./scripts/setup.sh"