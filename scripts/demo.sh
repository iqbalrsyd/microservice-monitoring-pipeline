#!/bin/bash

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Demo configuration
API_GATEWAY_URL="http://localhost:8080"
BUSINESS_SERVICE_URL="http://localhost:8081"
DATA_SERVICE_URL="http://localhost:8082"
GRAFANA_URL="http://localhost:3000"
PROMETHEUS_URL="http://localhost:9090"

print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}🚀 Microservices Demo${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo
}

print_section() {
    echo -e "${PURPLE}📌 $1${NC}"
    echo -e "${PURPLE}----------------------------------------${NC}"
}

print_step() {
    echo -e "${CYAN}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

wait_for_user() {
    echo
    read -p "Press Enter to continue..."
    echo
}

# Function to check if service is available
check_service() {
    local url=$1
    local service_name=$2

    if curl -f -s "$url" >/dev/null 2>&1; then
        return 0
    else
        echo -e "${RED}❌ $service_name is not available at $url${NC}"
        return 1
    fi
}

# Function to make API call and show response
api_call() {
    local method=$1
    local url=$2
    local data=$3
    local description=$4

    print_step "$description"
    echo "Request: $method $url"

    if [ -n "$data" ]; then
        echo "Data: $data"
        response=$(curl -s -X "$method" -H "Content-Type: application/json" -d "$data" "$url" 2>/dev/null)
    else
        response=$(curl -s -X "$method" -H "Content-Type: application/json" "$url" 2>/dev/null)
    fi

    if [ $? -eq 0 ]; then
        echo "Response: $response" | jq . 2>/dev/null || echo "Response: $response"
        print_success "API call successful"
    else
        print_warning "API call failed"
    fi
    echo
}

# Function to show metrics
show_metrics() {
    local service_name=$1
    local url=$2

    print_step "Fetching metrics from $service_name"
    echo "URL: $url/metrics"

    if curl -f -s "$url/metrics" 2>/dev/null | head -20; then
        print_success "Metrics retrieved successfully"
    else
        print_warning "Failed to retrieve metrics"
    fi
    echo
}

# Main demo
main() {
    print_header

    # Check prerequisites
    print_section "Prerequisites Check"

    services_ok=true

    if ! check_service "$API_GATEWAY_URL" "API Gateway"; then
        services_ok=false
    fi

    if ! check_service "$BUSINESS_SERVICE_URL" "Business Service"; then
        services_ok=false
    fi

    if ! check_service "$DATA_SERVICE_URL" "Data Service"; then
        services_ok=false
    fi

    if ! check_service "$GRAFANA_URL" "Grafana"; then
        services_ok=false
    fi

    if ! check_service "$PROMETHEUS_URL" "Prometheus"; then
        services_ok=false
    fi

    if [ "$services_ok" = false ]; then
        echo -e "${RED}❌ Some services are not running. Please start the services first:${NC}"
        echo "Run: ./scripts/setup.sh"
        exit 1
    fi

    print_success "All services are running!"
    wait_for_user

    # Service Information
    print_section "Service Information"

    api_call "GET" "$API_GATEWAY_URL/" "" "API Gateway Service Information"
    api_call "GET" "$BUSINESS_SERVICE_URL/" "" "Business Service Information"
    api_call "GET" "$DATA_SERVICE_URL/" "" "Data Service Information"

    wait_for_user

    # Health Checks
    print_section "Health Checks"

    api_call "GET" "$API_GATEWAY_URL/health" "" "API Gateway Health Check"
    api_call "GET" "$BUSINESS_SERVICE_URL/health" "" "Business Service Health Check"
    api_call "GET" "$DATA_SERVICE_URL/health" "" "Data Service Health Check"

    wait_for_user

    # Business Service Demo - Order Management
    print_section "Business Service - Order Management"

    # Create orders
    order_data='{"product": "Laptop", "quantity": 2, "price": 999.99}'
    api_call "POST" "$BUSINESS_SERVICE_URL/api/v1/orders" "$order_data" "Create Order #1"

    order_data='{"product": "Smartphone", "quantity": 1, "price": 699.99}'
    api_call "POST" "$BUSINESS_SERVICE_URL/api/v1/orders" "$order_data" "Create Order #2"

    order_data='{"product": "Tablet", "quantity": 3, "price": 299.99}'
    api_call "POST" "$BUSINESS_SERVICE_URL/api/v1/orders" "$order_data" "Create Order #3"

    # List all orders
    api_call "GET" "$BUSINESS_SERVICE_URL/api/v1/orders" "" "List All Orders"

    # Get business metrics
    api_call "GET" "$BUSINESS_SERVICE_URL/api/v1/metrics" "" "Business Metrics"

    wait_for_user

    # Data Service Demo - Data Processing
    print_section "Data Service - Data Processing"

    # Create data records
    record_data='{
      "type": "user_event",
      "data": {
        "user_id": "user123",
        "action": "login",
        "ip_address": "192.168.1.100"
      }
    }'
    api_call "POST" "$DATA_SERVICE_URL/api/v1/records" "$record_data" "Create Data Record #1"

    record_data='{
      "type": "system_log",
      "data": {
        "level": "info",
        "message": "Service started successfully",
        "component": "api-gateway"
      }
    }'
    api_call "POST" "$DATA_SERVICE_URL/api/v1/records" "$record_data" "Create Data Record #2"

    # List data records
    api_call "GET" "$DATA_SERVICE_URL/api/v1/records" "" "List Data Records"

    # Create processing job
    api_call "POST" "$DATA_SERVICE_URL/api/v1/jobs" "" "Create Processing Job"

    # Generate test data
    api_call "POST" "$DATA_SERVICE_URL/api/v1/generate" "" "Generate Test Data"

    # Get data service metrics
    api_call "GET" "$DATA_SERVICE_URL/api/v1/metrics" "" "Data Service Metrics"

    wait_for_user

    # API Gateway Demo - Service Proxy
    print_section "API Gateway - Service Discovery and Proxy"

    api_call "GET" "$API_GATEWAY_URL/api/v1/services" "" "Discover Available Services"

    # Simulate high load
    print_step "Simulating High Load"
    print_info "Sending 100 requests to API Gateway..."

    for i in {1..20}; do
        curl -s "$API_GATEWAY_URL/" > /dev/null 2>&1 &
        curl -s "$BUSINESS_SERVICE_URL/api/v1/orders" > /dev/null 2>&1 &
    done
    wait

    print_success "Load simulation completed"

    # Start business activity simulation
    print_step "Starting Business Activity Simulation"
    api_call "POST" "$BUSINESS_SERVICE_URL/api/v1/simulate" "" "Simulate Business Activity"

    wait_for_user

    # Metrics Collection Demo
    print_section "Metrics Collection"

    show_metrics "API Gateway" "$API_GATEWAY_URL"
    show_metrics "Business Service" "$BUSINESS_SERVICE_URL"
    show_metrics "Data Service" "$DATA_SERVICE_URL"

    wait_for_user

    # Monitoring Dashboard Demo
    print_section "Monitoring Dashboards"

    print_info "Opening monitoring dashboards in your default browser..."

    if command -v xdg-open >/dev/null 2>&1; then
        xdg-open "$GRAFANA_URL"
    elif command -v open >/dev/null 2>&1; then
        open "$GRAFANA_URL"
    elif command -v start >/dev/null 2>&1; then
        start "$GRAFANA_URL"
    fi

    print_step "Grafana Dashboard: $GRAFANA_URL (admin/admin)"
    print_step "Prometheus: $PROMETHEUS_URL"

    echo -e "${CYAN}📊 Dashboard Features to Explore:${NC}"
    echo "• Microservices Overview - Service health and performance"
    echo "• Request Rate - HTTP requests per second"
    echo "• Response Time - Latency percentiles"
    echo "• Error Rates - 4xx and 5xx errors"
    echo "• Business Metrics - Orders and revenue"
    echo "• System Metrics - CPU, memory, disk usage"

    wait_for_user

    # Alerting Demo
    print_section "Alerting and Monitoring"

    print_step "Checking Prometheus Alerts"

    if curl -s "$PROMETHEUS_URL/api/v1/alerts" | jq '.data.alerts[] | select(.state=="firing")' 2>/dev/null; then
        print_warning "There are active alerts!"
    else
        print_success "No active alerts - everything looks good!"
    fi

    wait_for_user

    # Log Analysis Demo
    print_section "Log Analysis"

    print_step "Checking service logs..."

    echo -e "${CYAN}📋 Recent API Gateway Logs:${NC}"
    docker-compose logs --tail=10 api-gateway 2>/dev/null | grep -E "(INFO|ERROR|WARN)" || echo "No recent logs found"

    echo
    echo -e "${CYAN}📋 Recent Business Service Logs:${NC}"
    docker-compose logs --tail=10 business-service 2>/dev/null | grep -E "(INFO|ERROR|WARN)" || echo "No recent logs found"

    wait_for_user

    # Performance Testing Demo
    print_section "Performance Testing"

    print_step "Running quick performance test..."

    if command -v ab >/dev/null 2>&1; then
        echo "Running Apache Bench test on API Gateway..."
        ab -n 100 -c 10 "$API_GATEWAY_URL/" 2>/dev/null | grep -E "(Requests per second|Time per request|Failed requests)" || echo "Apache Bench test completed"
    else
        print_info "Apache Bench (ab) not found. Install it for performance testing."
    fi

    wait_for_user

    # Cleanup Demo
    print_section "Cleanup and Summary"

    print_step "Cleaning up test data..."

    # Clean up old data records
    api_call "DELETE" "$DATA_SERVICE_URL/api/v1/cleanup?cutoff=$(date -d '1 hour ago' -I)" "" "Clean Old Data Records"

    # Show final status
    print_step "Final System Status"

    api_call "GET" "$BUSINESS_SERVICE_URL/api/v1/metrics" "" "Final Business Metrics"
    api_call "GET" "$DATA_SERVICE_URL/api/v1/metrics" "" "Final Data Metrics"

    # Summary
    print_section "Demo Summary"

    echo -e "${GREEN}🎉 Demo completed successfully!${NC}"
    echo
    echo -e "${BLUE}✨ What we demonstrated:${NC}"
    echo "• Microservices architecture with 3 services"
    echo "• RESTful API endpoints for business operations"
    echo "• Real-time metrics collection with Prometheus"
    echo "• Beautiful dashboards with Grafana"
    echo "• Centralized logging with Loki"
    echo "• Health checks and monitoring"
    echo "• Load simulation and performance testing"
    echo "• API Gateway with service discovery"
    echo "• Background job processing"
    echo

    echo -e "${BLUE}🔍 What to explore next:${NC}"
    echo "• Check Grafana dashboards for live metrics"
    echo "• Explore Prometheus query interface"
    echo "• Monitor logs in Grafana with Loki"
    echo "• Try the Jenkins CI/CD pipeline"
    echo "• Scale services and observe behavior"
    echo "• Configure custom alerts and notifications"
    echo

    echo -e "${CYAN}📚 Useful Links:${NC}"
    echo "• Grafana: $GRAFANA_URL (admin/admin)"
    echo "• Prometheus: $PROMETHEUS_URL"
    echo "• API Documentation: ./docs/USER_GUIDE.md"
    echo "• Architecture: ./docs/ARCHITECTURE.md"
    echo

    print_success "Thank you for trying the Microservices Demo! 🚀"
}

# Check if required tools are available
if ! command -v curl >/dev/null 2>&1; then
    echo -e "${RED}❌ curl is required for this demo${NC}"
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  jq is recommended for better JSON formatting${NC}"
    echo "Install jq: sudo apt-get install jq (Ubuntu) or brew install jq (macOS)"
    echo
fi

# Run the demo
main "$@"