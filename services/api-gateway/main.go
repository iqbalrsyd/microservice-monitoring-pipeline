package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type ServiceHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	URL     string `json:"url"`
	Healthy bool   `json:"healthy"`
}

type HealthResponse struct {
	Status   string          `json:"status"`
	Services []ServiceHealth `json:"services"`
	Uptime   string          `json:"uptime"`
}

var (
	startTime = time.Now()

	// Prometheus metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active connections",
		},
	)

	serviceHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_health",
			Help: "Health status of downstream services (1=healthy, 0=unhealthy)",
		},
		[]string{"service_name"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(activeConnections)
	prometheus.MustRegister(serviceHealth)

	// Configure logging
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)
}

func main() {
	// Load configuration
	loadConfig()

	router := mux.NewRouter()

	// Middleware
	router.Use(loggingMiddleware)
	router.Use(metricsMiddleware)

	// Routes
	router.HandleFunc("/", homeHandler).Methods("GET")
	router.HandleFunc("/health", healthHandler).Methods("GET")
	router.HandleFunc("/ready", readinessHandler).Methods("GET")
	router.HandleFunc("/metrics", promhttp.Handler().ServeHTTP).Methods("GET")

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/proxy/{service}/{path:.*}", proxyHandler).Methods("GET", "POST", "PUT", "DELETE")
	api.HandleFunc("/services", servicesHandler).Methods("GET")

	// Health checks for downstream services
	checkServiceHealth("business-service", viper.GetString("services.business"))
	checkServiceHealth("data-service", viper.GetString("services.data"))

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", viper.GetString("port")),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logrus.WithField("port", viper.GetString("port")).Info("Starting API Gateway")

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Server failed to start")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Server forced to shutdown")
	}

	logrus.Info("Server exited")
}

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set defaults
	viper.SetDefault("port", "8080")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("services.business", "http://business-service:8081")
	viper.SetDefault("services.data", "http://data-service:8082")

	if err := viper.ReadInConfig(); err != nil {
		logrus.WithError(err).Warn("Could not read config file, using defaults")
	}

	// Allow environment variables to override config
	viper.AutomaticEnv()
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
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
		}).Info("HTTP request")
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

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		activeConnections.Inc()
		defer activeConnections.Dec()

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode)).Observe(duration)
	})
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"service":     "API Gateway",
		"version":     "1.0.0",
		"status":      "running",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"uptime":      time.Since(startTime).String(),
	}

	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	services := []ServiceHealth{
		{Name: "business-service", URL: viper.GetString("services.business")},
		{Name: "data-service", URL: viper.GetString("services.data")},
	}

	allHealthy := true
	for i := range services {
		healthy := checkHealth(services[i].URL)
		services[i].Healthy = healthy
		services[i].Status = "healthy"
		if !healthy {
			services[i].Status = "unhealthy"
			allHealthy = false
		}
	}

	status := "healthy"
	if !allHealthy {
		status = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	response := HealthResponse{
		Status:   status,
		Services: services,
		Uptime:   time.Since(startTime).String(),
	}

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

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["service"]
	path := vars["path"]

	var targetURL string
	switch serviceName {
	case "business":
		targetURL = viper.GetString("services.business") + "/" + path
	case "data":
		targetURL = viper.GetString("services.data") + "/" + path
	default:
		http.Error(w, "Unknown service", http.StatusNotFound)
		return
	}

	logrus.WithFields(logrus.Fields{
		"service": serviceName,
		"path":    path,
		"target":  targetURL,
	}).Info("Proxying request")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Proxy functionality - request would be forwarded to target service",
		"service":    serviceName,
		"path":       path,
		"target_url": targetURL,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}

func servicesHandler(w http.ResponseWriter, r *http.Request) {
	services := map[string]interface{}{
		"services": []map[string]string{
			{
				"name": "business-service",
				"url":  viper.GetString("services.business"),
				"type": "REST API",
			},
			{
				"name": "data-service",
				"url":  viper.GetString("services.data"),
				"type": "REST API",
			},
		},
		"gateway_version": "1.0.0",
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(services)
}

func checkHealth(url string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func checkServiceHealth(serviceName, url string) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			healthy := checkHealth(url)
			value := float64(0)
			if healthy {
				value = 1
			}
			serviceHealth.WithLabelValues(serviceName).Set(value)

			logrus.WithFields(logrus.Fields{
				"service": serviceName,
				"healthy": healthy,
			}).Debug("Service health check")
		}
	}()
}