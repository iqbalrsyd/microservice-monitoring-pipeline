# Quick Start Guide

## 🚀 Start the Platform in 3 Simple Steps

### Prerequisites
- Docker and Docker Compose installed
- At least 8GB RAM and 2 CPU cores

### Step 1: Clone & Setup
```bash
git clone <repository-url>
cd microservice-pipeline-monitoring
chmod +x scripts/*.sh
```

### Step 2: Start All Services
```bash
./scripts/setup.sh
```

### Step 3: Run Demo (Optional)
```bash
./scripts/demo.sh
```

## 📊 Access Points

| Service | URL | Credentials | Purpose |
|---------|-----|-------------|---------|
| Grafana | http://localhost:3000 | admin/admin | Main Dashboard |
| API Gateway | http://localhost:8090 | - | Main API |
| Business Service | http://localhost:8081 | - | Orders API |
| Data Service | http://localhost:8082 | - | Data Processing |
| Prometheus | http://localhost:9090 | - | Metrics |
| Jenkins | http://localhost:8084 | admin/admin | CI/CD |

## 🛠️ Useful Commands

```bash
# View logs
docker-compose logs -f

# Check status
docker-compose ps

# Stop all services
./scripts/teardown.sh

# Restart specific service
docker-compose restart api-gateway
```

## 📚 Learn More

- **User Guide**: [docs/USER_GUIDE.md](docs/USER_GUIDE.md)
- **Architecture**: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **Full README**: [README.md](README.md)

## ✨ Features Demonstrated

- ✅ 3 Go microservices with instrumentation
- ✅ Real-time monitoring with Prometheus/Grafana
- ✅ Centralized logging with Loki
- ✅ CI/CD pipeline with Jenkins
- ✅ Health checks and alerting
- ✅ Docker containerization
- ✅ Load balancing and service discovery

Enjoy your microservices observability platform! 🎯