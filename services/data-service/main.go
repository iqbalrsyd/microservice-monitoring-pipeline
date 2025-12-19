package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type DataRecord struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Data        map[string]string `json:"data"`
	Timestamp   time.Time         `json:"timestamp"`
	Processed   bool              `json:"processed"`
	ProcessedAt *time.Time        `json:"processed_at,omitempty"`
}

type DataMetrics struct {
	TotalRecords      int     `json:"total_records"`
	ProcessedRecords  int     `json:"processed_records"`
	PendingRecords    int     `json:"pending_records"`
	ProcessingRate    float64 `json:"processing_rate_per_second"`
	DataSize          int64   `json:"data_size_bytes"`
}

type ProcessingJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Records   int       `json:"records_processed"`
	Error     string    `json:"error,omitempty"`
}

var (
	startTime = time.Now()
	db        *bolt.DB
	jobs      = make(map[string]ProcessingJob)

	// Prometheus metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "data_http_requests_total",
			Help: "Total number of HTTP requests for data service",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "data_http_request_duration_seconds",
			Help:    "HTTP request duration for data service",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "endpoint", "status"},
	)

	dataRecordsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "data_records_total",
			Help: "Total number of data records by status",
		},
		[]string{"status"},
	)

	dataProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "data_processing_duration_seconds",
			Help:    "Time taken to process data records",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"record_type"},
	)

	dataSizeBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "data_size_bytes",
			Help: "Total size of data in bytes",
		},
	)

	activeJobs = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "data_active_jobs",
			Help: "Number of active data processing jobs",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(dataRecordsTotal)
	prometheus.MustRegister(dataProcessingDuration)
	prometheus.MustRegister(dataSizeBytes)
	prometheus.MustRegister(activeJobs)

	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)
}

func main() {
	loadConfig()

	// Initialize database
	var err error
	db, err = bolt.Open("data.db", 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		logrus.WithError(err).Fatal("Failed to open database")
	}
	defer db.Close()

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("records"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("jobs"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create buckets")
	}

	// Start background data processing
	go processDataContinuously()

	router := mux.NewRouter()

	// Middleware
	router.Use(loggingMiddleware)
	router.Use(metricsMiddleware)

	// Routes
	router.HandleFunc("/", homeHandler).Methods("GET")
	router.HandleFunc("/health", healthHandler).Methods("GET")
	router.HandleFunc("/ready", readinessHandler).Methods("GET")
	router.HandleFunc("/metrics", promhttp.Handler().ServeHTTP).Methods("GET")

	// Data endpoints
	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/records", createRecordHandler).Methods("POST")
	api.HandleFunc("/records", getRecordsHandler).Methods("GET")
	api.HandleFunc("/records/{id}", getRecordHandler).Methods("GET")
	api.HandleFunc("/jobs", createJobHandler).Methods("POST")
	api.HandleFunc("/jobs", getJobsHandler).Methods("GET")
	api.HandleFunc("/jobs/{id}", getJobHandler).Methods("GET")
	api.HandleFunc("/metrics", dataMetricsHandler).Methods("GET")
	api.HandleFunc("/generate", generateTestData).Methods("POST")
	api.HandleFunc("/cleanup", cleanupOldRecords).Methods("DELETE")

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", viper.GetString("port")),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logrus.WithField("port", viper.GetString("port")).Info("Starting Data Service")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Server failed to start")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down data service...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Server forced to shutdown")
	}

	logrus.Info("Data service exited")
}

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetDefault("port", "8082")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("processing_interval", "5s")
	viper.SetDefault("batch_size", 10)

	if err := viper.ReadInConfig(); err != nil {
		logrus.WithError(err).Warn("Could not read config file, using defaults")
	}

	viper.AutomaticEnv()
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		logrus.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      wrapped.statusCode,
			"duration":    duration.String(),
			"user_agent":  r.UserAgent(),
			"remote_addr": r.RemoteAddr,
		}).Info("Data service request")
	})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode)).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var totalRecords int
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("records"))
		totalRecords = b.Stats().KeyN
		return nil
	})

	response := map[string]interface{}{
		"service":     "Data Service",
		"version":     "1.0.0",
		"status":      "running",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"uptime":      time.Since(startTime).String(),
		"records":     totalRecords,
		"active_jobs": len(jobs),
	}

	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check database health
	dbHealthy := true
	err := db.View(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("health_check"))
		return err
	})
	if err != nil {
		dbHealthy = false
	}

	healthy := dbHealthy
	status := "healthy"
	statusCode := http.StatusOK
	if !healthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(startTime).String(),
		"checks": map[string]bool{
			"database": dbHealthy,
		},
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "ready",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func createRecordHandler(w http.ResponseWriter, r *http.Request) {
	var record DataRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	record.ID = uuid.New().String()
	record.Timestamp = time.Now()
	record.Processed = false

	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("records"))
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put([]byte(record.ID), data)
	})

	if err != nil {
		http.Error(w, "Failed to save record", http.StatusInternalServerError)
		return
	}

	dataRecordsTotal.WithLabelValues("pending").Inc()

	logrus.WithFields(logrus.Fields{
		"record_id": record.ID,
		"type":      record.Type,
	}).Info("Data record created")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(record)
}

func getRecordsHandler(w http.ResponseWriter, r *http.Request) {
	var records []DataRecord

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("records"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var record DataRecord
			if err := json.Unmarshal(v, &record); err != nil {
				return err
			}
			records = append(records, record)
		}
		return nil
	})

	if err != nil {
		http.Error(w, "Failed to retrieve records", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"records": records,
		"total":   len(records),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getRecordHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	recordID := vars["id"]

	var record DataRecord
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("records"))
		data := b.Get([]byte(recordID))
		if data == nil {
			return fmt.Errorf("record not found")
		}
		return json.Unmarshal(data, &record)
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

func createJobHandler(w http.ResponseWriter, r *http.Request) {
	job := ProcessingJob{
		ID:        uuid.New().String(),
		Status:    "pending",
		StartTime: time.Now(),
		Records:   0,
	}

	jobs[job.ID] = job
	activeJobs.Inc()

	// Start job processing in background
	go processJob(job.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func getJobsHandler(w http.ResponseWriter, r *http.Request) {
	jobList := make([]ProcessingJob, 0, len(jobs))
	for _, job := range jobs {
		jobList = append(jobList, job)
	}

	response := map[string]interface{}{
		"jobs":  jobList,
		"total": len(jobList),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getJobHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	job, exists := jobs[jobID]
	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func dataMetricsHandler(w http.ResponseWriter, r *http.Request) {
	var totalRecords, processedRecords, pendingRecords int

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("records"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var record DataRecord
			if err := json.Unmarshal(v, &record); err != nil {
				continue
			}
			totalRecords++
			if record.Processed {
				processedRecords++
			} else {
				pendingRecords++
			}
		}
		return nil
	})

	processingRate := float64(processedRecords) / time.Since(startTime).Seconds()

	// Calculate approximate data size
	dataSize := int64(totalRecords * 500) // Rough estimate

	metrics := DataMetrics{
		TotalRecords:      totalRecords,
		ProcessedRecords:  processedRecords,
		PendingRecords:    pendingRecords,
		ProcessingRate:    processingRate,
		DataSize:          dataSize,
	}

	// Update Prometheus metrics
	dataRecordsTotal.WithLabelValues("processed").Set(float64(processedRecords))
	dataRecordsTotal.WithLabelValues("pending").Set(float64(pendingRecords))
	dataSizeBytes.Set(float64(dataSize))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func generateTestData(w http.ResponseWriter, r *http.Request) {
	go func() {
		recordTypes := []string{"user_event", "system_log", "metric", "trace"}

		for i := 0; i < 50; i++ {
			record := DataRecord{
				ID: uuid.New().String(),
				Type: recordTypes[rand.Intn(len(recordTypes))],
				Data: map[string]string{
					"source":     "generator",
					"category":   fmt.Sprintf("category_%d", rand.Intn(10)),
					"priority":   fmt.Sprintf("%d", rand.Intn(5)+1),
					"session_id": uuid.New().String(),
				},
				Timestamp: time.Now().Add(-time.Duration(rand.Intn(3600)) * time.Second),
				Processed: false,
			}

			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("records"))
				data, err := json.Marshal(record)
				if err != nil {
					return err
				}
				return b.Put([]byte(record.ID), data)
			})

			if err != nil {
				logrus.WithError(err).Error("Failed to save test record")
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":   "Test data generation started",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func cleanupOldRecords(w http.ResponseWriter, r *http.Request) {
	// Parse cutoff time from query param
	cutoffStr := r.URL.Query().Get("cutoff")
	cutoffTime := time.Now().Add(-24 * time.Hour) // Default: 24 hours ago

	if cutoffStr != "" {
		if parsed, err := time.Parse(time.RFC3339, cutoffStr); err == nil {
			cutoffTime = parsed
		}
	}

	var deletedCount int
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("records"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var record DataRecord
			if err := json.Unmarshal(v, &record); err != nil {
				continue
			}

			if record.Timestamp.Before(cutoffTime) {
				if err := b.Delete(k); err == nil {
					deletedCount++
				}
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, "Failed to cleanup records", http.StatusInternalServerError)
		return
	}

	logrus.WithField("deleted_count", deletedCount).Info("Old records cleaned up")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "Cleanup completed",
		"deleted_count": deletedCount,
		"cutoff_time":   cutoffTime.Format(time.RFC3339),
	})
}

func processDataContinuously() {
	interval, _ := time.ParseDuration(viper.GetString("processing_interval"))
	batchSize := viper.GetInt("batch_size")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		processPendingRecords(batchSize)
	}
}

func processPendingRecords(batchSize int) {
	var records []DataRecord

	// Fetch pending records
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("records"))
		c := b.Cursor()

		for k, v := c.First(); k != nil && len(records) < batchSize; k, v = c.Next() {
			var record DataRecord
			if err := json.Unmarshal(v, &record); err != nil {
				continue
			}

			if !record.Processed {
				records = append(records, record)
			}
		}
		return nil
	})

	if err != nil || len(records) == 0 {
		return
	}

	// Process records
	for _, record := range records {
		start := time.Now()

		// Simulate processing time
		time.Sleep(time.Duration(rand.Intn(500)+100) * time.Millisecond)

		now := time.Now()
		record.Processed = true
		record.ProcessedAt = &now

		// Update record in database
		err = db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("records"))
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			return b.Put([]byte(record.ID), data)
		})

		if err == nil {
			processingTime := time.Since(start).Seconds()
			dataProcessingDuration.WithLabelValues(record.Type).Observe(processingTime)
			dataRecordsTotal.WithLabelValues("pending").Dec()
			dataRecordsTotal.WithLabelValues("processed").Inc()

			logrus.WithFields(logrus.Fields{
				"record_id":      record.ID,
				"type":           record.Type,
				"processing_time": processingTime,
			}).Debug("Record processed")
		}
	}
}

func processJob(jobID string) {
	job, exists := jobs[jobID]
	if !exists {
		return
	}

	job.Status = "running"
	jobs[jobID] = job

	// Process a batch of records
	processPendingRecords(20)

	// Update job status
	job.Status = "completed"
	now := time.Now()
	job.EndTime = &now
	job.Records = 20 // Simplified for demo

	jobs[jobID] = job
	activeJobs.Dec()

	logrus.WithField("job_id", jobID).Info("Job completed")
}