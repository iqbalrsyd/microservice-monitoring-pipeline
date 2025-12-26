#!/bin/bash

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check if docker compose is available (either as plugin or standalone)
docker_compose_exists() {
    # Check for docker compose plugin (modern)
    if docker compose version >/dev/null 2>&1; then
        return 0
    fi
    # Check for $(get_docker_compose_cmd) standalone (legacy)
    if command_exists "$(get_docker_compose_cmd)"; then
        return 0
    fi
    return 1
}

# Get the appropriate docker compose command
get_docker_compose_cmd() {
    if docker compose version >/dev/null 2>&1; then
        echo "docker compose"
    else
        echo "$(get_docker_compose_cmd)"
    fi
}

# Function to check if service is healthy
check_service_health() {
    local service_name=$1
    local url=$2
    local max_attempts=30
    local attempt=1

    print_status "Checking health of $service_name..."

    while [ $attempt -le $max_attempts ]; do
        if curl -f -s "$url" >/dev/null; then
            print_success "$service_name is healthy!"
            return 0
        fi

        echo -n "."
        sleep 2
        ((attempt++))
    done

    print_error "$service_name health check failed after $max_attempts attempts"
    return 1
}

print_status "🚀 Starting Microservices Pipeline Setup"
echo

# Check prerequisites
print_status "📋 Checking prerequisites..."

prerequisites=("docker" "git" "curl")
missing_prereqs=()

for prereq in "${prerequisites[@]}"; do
    if ! command_exists "$prereq"; then
        missing_prereqs+=("$prereq")
    else
        print_success "✓ $prereq is installed"
    fi
done

# Check for Docker Compose (either plugin or standalone)
if ! docker_compose_exists; then
    missing_prereqs+=("$(get_docker_compose_cmd)")
else
    print_success "✓ $(get_docker_compose_cmd) is installed"
fi

if [ ${#missing_prereqs[@]} -ne 0 ]; then
    print_error "Missing prerequisites: ${missing_prereqs[*]}"
    echo "Please install the missing tools and run this script again."
    exit 1
fi

echo

# Create necessary directories
print_status "📁 Creating directories..."
mkdir -p logs data/prometheus data/grafana data/loki data/alertmanager data/jenkins
print_success "Directories created"

# Set proper permissions
print_status "🔐 Setting permissions..."
chmod 755 scripts
chmod +x scripts/*.sh
print_success "Permissions set"

# Build and start services
print_status "🏗️ Building and starting services..."

# Start with monitoring stack first
print_status "Starting monitoring stack..."
$(get_docker_compose_cmd) up -d prometheus grafana loki promtail node-exporter cadvisor alertmanager

# Wait for monitoring services to be ready
sleep 10

# Check monitoring services health
check_service_health "Prometheus" "http://localhost:9090/-/healthy"
check_service_health "Grafana" "http://localhost:3000/api/health"

# Start microservices
print_status "Starting microservices..."
$(get_docker_compose_cmd) up -d api-gateway business-service data-service

# Wait for microservices to be ready
sleep 15

# Check microservices health
check_service_health "API Gateway" "http://localhost:8090/health"
check_service_health "Business Service" "http://localhost:8081/health"
check_service_health "Data Service" "http://localhost:8082/health"

# Start Jenkins
print_status "Starting Jenkins..."
$(get_docker_compose_cmd) up -d jenkins

# Wait for Jenkins to be ready
print_status "Waiting for Jenkins to start..."
sleep 30

# Check Jenkins health (it takes longer to start)
check_service_health "Jenkins" "http://localhost:8084/login"

# Start Nginx
print_status "Starting Nginx reverse proxy..."
$(get_docker_compose_cmd) up -d nginx

sleep 5

# Generate some test data
print_status "📊 Generating test data..."

# Generate test orders
curl -X POST http://localhost:8081/api/v1/simulate -H "Content-Type: application/json" >/dev/null 2>&1 || true

# Generate test data records
curl -X POST http://localhost:8082/api/v1/generate -H "Content-Type: application/json" >/dev/null 2>&1 || true

# Create basic alert rules in Prometheus
print_status "🚨 Reloading Prometheus configuration..."
curl -X POST http://localhost:9090/-/reload >/dev/null 2>&1 || true

echo
print_success "🎉 Setup completed successfully!"
echo

# Print access information
echo -e "${BLUE}📋 Service Access Information:${NC}"
echo -e "• API Gateway:     ${GREEN}http://localhost:8090${NC}"
echo -e "• Business Service:${GREEN}http://localhost:8081${NC}"
echo -e "• Data Service:    ${GREEN}http://localhost:8082${NC}"
echo -e "• Grafana:         ${GREEN}http://localhost:3000${NC} (admin/admin)"
echo -e "• Prometheus:      ${GREEN}http://localhost:9090${NC}"
echo -e "• Loki:            ${GREEN}http://localhost:3100${NC}"
echo -e "• Jenkins:         ${GREEN}http://localhost:8084${NC}"
echo -e "• Node Exporter:   ${GREEN}http://localhost:9100${NC}"
echo -e "• cAdvisor:        ${GREEN}http://localhost:8083${NC}"
echo

# Print useful commands
echo -e "${BLUE}🛠️ Useful Commands:${NC}"
echo -e "• View logs:          ${YELLOW}$(get_docker_compose_cmd) logs -f [service-name]${NC}"
echo -e "• Stop all services:  ${YELLOW}$(get_docker_compose_cmd) down${NC}"
echo -e "• Restart service:    ${YELLOW}$(get_docker_compose_cmd) restart [service-name]${NC}"
echo -e "• Check status:       ${YELLOW}$(get_docker_compose_cmd) ps${NC}"
echo -e "• View metrics:       ${YELLOW}curl http://localhost:8090/metrics${NC}"
echo

# Print next steps
echo -e "${BLUE}📚 Next Steps:${NC}"
echo "1. Open Grafana and explore the pre-configured dashboards"
echo "2. Check Prometheus targets and alerts"
echo "3. Test the microservices API endpoints"
echo "4. Configure Jenkins pipeline with your repository"
echo "5. Set up alert notifications (email/Slack)"
echo

print_success "Enjoy your microservices monitoring platform! 🎯"