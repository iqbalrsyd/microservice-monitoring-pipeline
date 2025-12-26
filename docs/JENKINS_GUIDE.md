# 🚀 JENKINS CI/CD GUIDE

## 📋 APA ITU JENKINS?

**Jenkins** adalah automation server untuk CI/CD (Continuous Integration/Continuous Deployment)

### Kenapa Perlu Jenkins? 🤔

Meskipun sudah deploy dengan Docker, Jenkins memberikan:

| Tanpa Jenkins | Dengan Jenkins |
|---------------|----------------|
| Manual build setiap kali code berubah | ✅ Auto build saat push ke Git |
| Manual test setiap deployment | ✅ Auto testing sebelum deploy |
| Manual deploy ke server | ✅ Auto deploy jika test pass |
| Tidak ada tracking | ✅ History lengkap semua deployment |
| Prone to human error | ✅ Consistent & reliable |

### Workflow Jenkins:

```
1. Developer push code ke Git
   ↓
2. Jenkins detect perubahan (webhook/polling)
   ↓
3. Jenkins pull code terbaru
   ↓
4. Jenkins build Docker image
   ↓
5. Jenkins run automated tests
   ↓
6. Jika test PASS → Deploy ke staging
   ↓
7. Manual approval (optional)
   ↓
8. Deploy ke production
   ↓
9. Notify team (Slack/Email)
```

---

## 🔐 AKSES JENKINS

### Web Interface:
- **URL**: http://localhost:8084
- **Username**: `admin`
- **Password**: `admin`

### Port Mapping:
- `8084` → Jenkins Web UI
- `50001` → Jenkins Agent communication

---

## 📝 CARA MENGGUNAKAN JENKINS

### 1. Login ke Jenkins
```bash
xdg-open http://localhost:8084
```
Login dengan: `admin` / `admin`

### 2. Membuat Pipeline Baru

**A. Via Web UI:**
1. Dashboard → **New Item**
2. Masukkan nama: `microservices-pipeline`
3. Pilih **Pipeline**
4. Klik **OK**
5. Di section **Pipeline**:
   - Definition: **Pipeline script from SCM**
   - SCM: **Git**
   - Repository URL: (your git repo)
   - Script Path: `jenkins/Jenkinsfile`
6. **Save**

**B. Via Jenkinsfile (Recommended):**

Jenkinsfile sudah ada di: `jenkins/Jenkinsfile`

Contoh pipeline stages:

```groovy
pipeline {
    agent any
    
    stages {
        stage('Checkout') {
            steps {
                git 'https://github.com/your-repo.git'
            }
        }
        
        stage('Build') {
            steps {
                sh 'docker-compose build'
            }
        }
        
        stage('Test') {
            steps {
                sh './scripts/test.sh'
            }
        }
        
        stage('Deploy Staging') {
            steps {
                sh 'docker-compose up -d'
            }
        }
        
        stage('Health Check') {
            steps {
                sh 'curl http://localhost:8090/health'
            }
        }
    }
    
    post {
        success {
            echo 'Pipeline berhasil! ✅'
        }
        failure {
            echo 'Pipeline gagal! ❌'
        }
    }
}
```

### 3. Trigger Build

**Manual Build:**
1. Dashboard → Pilih pipeline
2. Klik **Build Now**
3. Lihat progress di **Build History**

**Auto Trigger (Git Webhook):**
1. Pipeline → **Configure**
2. Section **Build Triggers**
3. Check **Poll SCM** atau **GitHub hook trigger**
4. Schedule: `H/5 * * * *` (check every 5 minutes)

### 4. Monitor Pipeline

- **Console Output**: Klik build number → **Console Output**
- **Pipeline Visualization**: Lihat stage-by-stage execution
- **Build History**: Semua history builds
- **Test Results**: Jika ada test yang dijalankan

---

## 🛠️ CONTOH USE CASES

### Use Case 1: Auto Build on Git Push

```groovy
pipeline {
    agent any
    triggers {
        pollSCM('H/5 * * * *')  // Check Git every 5 minutes
    }
    stages {
        stage('Build Services') {
            steps {
                sh '''
                    cd services/api-gateway && docker build -t api-gateway .
                    cd services/business-service && docker build -t business-service .
                    cd services/data-service && docker build -t data-service .
                '''
            }
        }
    }
}
```

### Use Case 2: Run Tests Before Deploy

```groovy
stage('Unit Tests') {
    steps {
        sh '''
            # Run Go tests
            cd services/api-gateway && go test ./...
            cd services/business-service && go test ./...
            cd services/data-service && go test ./...
        '''
    }
}
```

### Use Case 3: Blue-Green Deployment

```groovy
stage('Deploy Blue') {
    steps {
        sh 'docker-compose -f docker-compose.blue.yml up -d'
    }
}
stage('Health Check Blue') {
    steps {
        sh 'curl http://blue-env:8090/health'
    }
}
stage('Switch to Blue') {
    steps {
        sh './scripts/switch-to-blue.sh'
    }
}
```

---

## 🔧 JENKINS PLUGINS YANG BERGUNA

Install via: **Manage Jenkins** → **Manage Plugins**

1. **Docker Pipeline** - Build & push Docker images
2. **Git Plugin** - Git integration
3. **Pipeline** - Pipeline as code
4. **Blue Ocean** - Modern UI for pipelines
5. **Slack Notification** - Send notif ke Slack
6. **Email Extension** - Email notifications
7. **Prometheus Plugin** - Export Jenkins metrics

---

## 📊 INTEGRASI DENGAN MONITORING

Jenkins sudah ter-integrasi dengan monitoring stack:

### Check di Prometheus:
```promql
# Jenkins status
up{job="jenkins"}

# Build success rate
jenkins_builds_success_count / jenkins_builds_total
```

### Alert di Prometheus:
```yaml
- alert: JenkinsDown
  expr: up{job="jenkins"} == 0
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "Jenkins is down"
```

---

## 🎯 QUICK TEST PIPELINE

Buat file test di `jenkins/Jenkinsfile`:

```groovy
pipeline {
    agent any
    
    stages {
        stage('Test Monitoring Stack') {
            steps {
                script {
                    // Health checks
                    sh 'curl -f http://api-gateway:8080/health || exit 1'
                    sh 'curl -f http://business-service:8081/health || exit 1'
                    sh 'curl -f http://data-service:8082/health || exit 1'
                    
                    // Check metrics
                    sh 'curl -f http://prometheus:9090/-/healthy || exit 1'
                    sh 'curl -f http://grafana:3000/api/health || exit 1'
                    
                    echo '✅ All services healthy!'
                }
            }
        }
    }
}
```

---

## 🚨 TROUBLESHOOTING

### Jenkins tidak bisa start container?
```bash
# Give Jenkins access to Docker
docker exec -u root jenkins chmod 666 /var/run/docker.sock
```

### Build stuck?
- Check **Console Output** untuk error
- Check resource: `docker stats`
- Check logs: `docker-compose logs jenkins`

### Cannot connect to Git?
- Setup SSH keys atau Personal Access Token
- **Manage Jenkins** → **Credentials** → Add Git credentials

---

## 📚 RESOURCES

- **Jenkins Docs**: https://jenkins.io/doc/
- **Pipeline Syntax**: http://localhost:8084/pipeline-syntax/
- **Jenkinsfile Examples**: https://github.com/jenkinsci/pipeline-examples

