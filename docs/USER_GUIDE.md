# User Guide - DevOps Pipeline & Observability Platform

## Table of Contents
1. [Quick Start](#quick-start)
2. [Service Access](#service-access)
3. [API Documentation](#api-documentation)
4. [Monitoring Guide](#monitoring-guide)
5. [CI/CD Pipeline](#cicd-pipeline)
6. [Troubleshooting](#troubleshooting)
7. [Advanced Configuration](#advanced-configuration)

## Quick Start

### Prerequisites
- Docker and Docker Compose installed
- At least 8GB RAM and 2 CPU cores
- 20GB free disk space
- Git for cloning the repository

### Installation Steps

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd microservice-pipeline-monitoring
   ```

2. **Run the setup script**
   ```bash
   chmod +x scripts/setup.sh
   ./scripts/setup.sh
   ```

3. **Verify the installation**
   - Open http://localhost:3000 in your browser (Grafana)
   - Check that all services are healthy
   - Verify metrics are being collected

## Service Access

### Web Interfaces

| Service | URL | Credentials | Purpose |
|---------|-----|-------------|---------|
| Grafana | http://localhost:3000 | admin/admin | Dashboards and visualization |
| Prometheus | http://localhost:9090 | None | Metrics and alerting |
| API Gateway | http://localhost:8090 | None | Main API endpoint |
| Business Service | http://localhost:8081 | None | Business logic API |
| Data Service | http://localhost:8082 | None | Data processing API |
| Jenkins | http://localhost:8080 | admin/admin | CI/CD Pipeline |
| cAdvisor | http://localhost:8083 | None | Container metrics |

### API Endpoints

#### API Gateway
- `GET /` - Service information
- `GET /health` - Health check
- `GET /ready` - Readiness probe
- `GET /metrics` - Prometheus metrics
- `GET /api/v1/services` - Service list
- `ANY /api/v1/proxy/{service}/{path}` - Proxy requests

#### Business Service
- `GET /` - Service information
- `GET /health` - Health check
- `GET /ready` - Readiness probe
- `GET /metrics` - Prometheus metrics
- `GET /api/v1/orders` - List orders
- `POST /api/v1/orders` - Create order
- `GET /api/v1/orders/{id}` - Get specific order
- `PUT /api/v1/orders/{id}` - Update order
- `DELETE /api/v1/orders/{id}` - Delete order
- `GET /api/v1/metrics` - Business metrics
- `POST /api/v1/simulate` - Simulate activity

#### Data Service
- `GET /` - Service information
- `GET /health` - Health check
- `GET /ready` - Readiness probe
- `GET /metrics` - Prometheus metrics
- `GET /api/v1/records` - List data records
- `POST /api/v1/records` - Create data record
- `GET /api/v1/records/{id}` - Get specific record
- `GET /api/v1/jobs` - List processing jobs
- `POST /api/v1/jobs` - Create processing job
- `GET /api/v1/jobs/{id}` - Get job details
- `POST /api/v1/generate` - Generate test data
- `DELETE /api/v1/cleanup` - Clean old records

## API Documentation

### Creating an Order (Business Service)

**Request:**
```bash
curl -X POST http://localhost:8081/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "product": "Laptop",
    "quantity": 2,
    "price": 999.99
  }'
```

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "product": "Laptop",
  "quantity": 2,
  "price": 999.99,
  "status": "completed",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:02Z"
}
```

### Creating a Data Record (Data Service)

**Request:**
```bash
curl -X POST http://localhost:8082/api/v1/records \
  -H "Content-Type: application/json" \
  -d '{
    "type": "user_event",
    "data": {
      "user_id": "user123",
      "action": "login",
      "timestamp": "2024-01-15T10:30:00Z"
    }
  }'
```

**Response:**
```json
{
  "id": "456e7890-e89b-12d3-a456-426614174001",
  "type": "user_event",
  "data": {
    "user_id": "user123",
    "action": "login",
    "timestamp": "2024-01-15T10:30:00Z"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "processed": false
}
```

## Monitoring Guide

### Grafana Dashboards

#### Microservices Overview
- Access: http://localhost:3000/d/microservices-overview
- Shows: Request rates, response times, error rates, service health
- Refresh rate: 15 seconds

#### Key Metrics to Monitor

**Performance Metrics:**
- Request Rate: `rate(http_requests_total[5m])`
- Response Time (P95): `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))`
- Error Rate: `rate(http_requests_total{status=~"5.."}[5m])`

**Business Metrics:**
- Active Orders: `business_active_orders`
- Total Revenue: `business_total_revenue`
- Processing Rate: `rate(data_processing_duration_seconds_sum[5m]) / rate(data_processing_duration_seconds_count[5m])`

**System Metrics:**
- CPU Usage: `100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`
- Memory Usage: `(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100`
- Disk Usage: `(1 - (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"})) * 100`

### Prometheus Queries

**Find slow requests:**
```promql
topk(10, histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])))
```

**Find errors by service:**
```promql
sum by (job) (rate(http_requests_total{status=~"5.."}[5m]))
```

**Service availability:**
```promql
up{job=~".*-service"}
```

### Log Analysis with Loki

**View error logs:**
```logql
{level="error"} |= "error"
```

**Filter by service:**
```logql
{service="api-gateway"}
```

**Correlate logs with metrics:**
```logql
{service="business-service"} |= "order"
```

## CI/CD Pipeline

### Jenkins Pipeline Overview

The Jenkins pipeline automates the entire software delivery process:

1. **Code Checkout** - Retrieves source code from repository
2. **Static Analysis** - Runs linters and security scanners
3. **Testing** - Executes unit and integration tests
4. **Build** - Compiles applications and creates Docker images
5. **Staging Deployment** - Deploys to staging environment
6. **Performance Testing** - Runs load tests
7. **Production Deployment** - Blue-green deployment to production

### Triggering Builds

**Manual Build:**
1. Access Jenkins at http://localhost:8080
2. Select your pipeline
3. Click "Build Now"

**Automatic Triggers:**
- Git webhooks on code commits
- Scheduled builds (every 5 minutes)
- Pull request merges

### Pipeline Configuration

Edit the `Jenkinsfile` to customize:
- Build steps and commands
- Test suites and coverage thresholds
- Deployment strategies
- Notification settings

### Environment Variables

Configure these in Jenkins:
- `DOCKER_REGISTRY` - Your container registry URL
- `DOCKER_CREDENTIALS` - Registry credentials ID
- `SLACK_WEBHOOK` - Slack notification webhook
- `EMAIL_RECIPIENTS` - Alert email addresses

## Troubleshooting

### Common Issues

#### Services Won't Start
**Problem:** Container exits immediately
**Solution:**
```bash
# Check logs
docker-compose logs <service-name>

# Check port conflicts
netstat -tulpn | grep <port>

# Check Docker resources
docker system df
```

#### Metrics Not Appearing
**Problem:** No data in Prometheus/Grafana
**Solution:**
1. Verify service endpoints: `curl http://localhost:8090/metrics`
2. Check Prometheus targets: http://localhost:9090/targets
3. Verify network connectivity: `docker network ls`

#### High Memory Usage
**Problem:** Services consuming too much memory
**Solution:**
```bash
# Check container resource usage
docker stats

# Limit container resources
# Edit docker-compose.yml and add:
# deploy:
#   resources:
#     limits:
#       memory: 512M
```

#### Jenkins Build Fails
**Problem:** Pipeline execution fails
**Solution:**
1. Check Jenkins console output
2. Verify Docker daemon is accessible
3. Check workspace permissions
4. Review build logs for specific errors

### Health Checks

**Check all services:**
```bash
./scripts/health-check.sh
```

**Manual health checks:**
```bash
curl http://localhost:8090/health
curl http://localhost:8081/health
curl http://localhost:8082/health
```

### Log Analysis

**View all service logs:**
```bash
docker-compose logs -f
```

**View specific service logs:**
```bash
docker-compose logs -f api-gateway
```

**Search logs for errors:**
```bash
docker-compose logs | grep ERROR
```

## Advanced Configuration

### Custom Metrics

Add custom metrics to your Go services:

```go
// Define a new metric
var customCounter = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "custom_operations_total",
        Help: "Total number of custom operations",
    },
    []string{"operation", "status"},
)

// Register the metric
prometheus.MustRegister(customCounter)

// Use the metric
customCounter.WithLabelValues("process", "success").Inc()
```

### Alert Configuration

Edit `monitoring/prometheus/rules/alerts.yml` to add custom alerts:

```yaml
- alert: CustomAlert
  expr: custom_metric > threshold
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Custom alert triggered"
    description: "Custom metric {{ $labels.custom_label }} is {{ $value }}"
```

### Scaling Services

**Manual scaling:**
```bash
# Scale specific service
docker-compose up -d --scale api-gateway=3

# Verify scaling
docker-compose ps
```

**Auto-scaling setup:**
1. Install Kubernetes
2. Use Horizontal Pod Autoscaler
3. Configure metrics-based scaling

### Custom Dashboards

1. Access Grafana (http://localhost:3000)
2. Click "+" → "Dashboard"
3. Add panels with Prometheus queries
4. Save and share your dashboard

### Security Hardening

**Enable HTTPS:**
```bash
# Generate SSL certificates
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx/ssl/nginx.key \
  -out nginx/ssl/nginx.crt
```

**Network security:**
```bash
# Create custom networks
docker network create --driver bridge microservices
docker network create --driver bridge monitoring
```

### Performance Optimization

**Database optimization:**
- Implement connection pooling
- Add database indexing
- Use read replicas for queries

**Caching strategy:**
- Add Redis for distributed caching
- Implement application-level caching
- Use CDN for static assets

**Resource tuning:**
```yaml
# In docker-compose.yml
services:
  api-gateway:
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
```

### Backup and Recovery

**Data backup:**
```bash
# Backup volumes
docker run --rm -v jenkins_data:/data -v $(pwd):/backup alpine \
  tar czf /backup/jenkins-backup.tar.gz -C /data .

# Backup Prometheus data
docker exec prometheus tar czf /tmp/prometheus-backup.tar.gz /prometheus
```

**Restore procedure:**
```bash
# Stop services
docker-compose down

# Restore volumes
docker run --rm -v jenkins_data:/data -v $(pwd):/backup alpine \
  tar xzf /backup/jenkins-backup.tar.gz -C /data

# Start services
docker-compose up -d
```

This guide should help you get started with the platform and troubleshoot common issues. For more advanced configurations, refer to the individual service documentation.