package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Order struct {
	ID        string    `json:"id"`
	Product   string    `json:"product"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BusinessMetrics struct {
	TotalOrders      int     `json:"total_orders"`
	TotalRevenue     float64 `json:"total_revenue"`
	OrdersPerMinute  float64 `json:"orders_per_minute"`
	AverageOrderSize float64 `json:"average_order_size"`
}

var (
	startTime = time.Now()
	orders    = make(map[string]Order)
	orderLock = make(map[string]bool)

	// Prometheus metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "business_http_requests_total",
			Help: "Total number of HTTP requests for business service",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "business_http_request_duration_seconds",
			Help:    "HTTP request duration for business service",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "endpoint", "status"},
	)

	activeOrders = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "business_active_orders",
			Help: "Number of currently active orders",
		},
	)

	totalRevenue = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "business_total_revenue",
			Help: "Total revenue from all orders",
		},
	)

	orderProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "business_order_processing_duration_seconds",
			Help:    "Time taken to process orders",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"status"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(activeOrders)
	prometheus.MustRegister(totalRevenue)
	prometheus.MustRegister(orderProcessingDuration)

	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)
}

func main() {
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

	// Business logic endpoints
	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/orders", createOrderHandler).Methods("POST")
	api.HandleFunc("/orders", getOrdersHandler).Methods("GET")
	api.HandleFunc("/orders/{id}", getOrderHandler).Methods("GET")
	api.HandleFunc("/orders/{id}", updateOrderHandler).Methods("PUT")
	api.HandleFunc("/orders/{id}", deleteOrderHandler).Methods("DELETE")
	api.HandleFunc("/metrics", businessMetricsHandler).Methods("GET")
	api.HandleFunc("/simulate", simulateBusinessActivity).Methods("POST")

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", viper.GetString("port")),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logrus.WithField("port", viper.GetString("port")).Info("Starting Business Service")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Server failed to start")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down business service...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Server forced to shutdown")
	}

	logrus.Info("Business service exited")
}

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetDefault("port", "8081")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("order_processing_time", "2s")

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
		}).Info("Business service request")
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
	response := map[string]interface{}{
		"service":   "Business Service",
		"version":   "1.0.0",
		"status":    "running",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(startTime).String(),
		"orders":    len(orders),
	}

	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Simulate some business logic check
	healthy := true
	if len(orders) > 1000 { // Example threshold
		healthy = false
	}

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
		"orders":    len(orders),
		"checks": map[string]bool{
			"database":   true,
			"processing": healthy,
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

func createOrderHandler(w http.ResponseWriter, r *http.Request) {
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	order.ID = uuid.New().String()
	order.Status = "pending"
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	orderLock[order.ID] = true
	defer delete(orderLock, order.ID)

	// Simulate order processing time
	processingTime := time.Duration(rand.Intn(3)+1) * time.Second
	time.Sleep(processingTime)

	// Randomly fail some orders (5% failure rate for demo)
	if rand.Float32() < 0.05 {
		order.Status = "failed"
		orderProcessingDuration.WithLabelValues("failed").Observe(processingTime.Seconds())
	} else {
		order.Status = "completed"
		orderProcessingDuration.WithLabelValues("completed").Observe(processingTime.Seconds())
	}

	orders[order.ID] = order
	activeOrders.Inc()
	totalRevenue.Add(order.Price * float64(order.Quantity))

	logrus.WithFields(logrus.Fields{
		"order_id": order.ID,
		"status":   order.Status,
		"price":    order.Price,
	}).Info("Order processed")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func getOrdersHandler(w http.ResponseWriter, r *http.Request) {
	orderList := make([]Order, 0, len(orders))
	for _, order := range orders {
		orderList = append(orderList, order)
	}

	response := map[string]interface{}{
		"orders": orderList,
		"total":  len(orderList),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getOrderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	order, exists := orders[orderID]
	if !exists {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func updateOrderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	order, exists := orders[orderID]
	if !exists {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	var updateData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if status, ok := updateData["status"].(string); ok {
		order.Status = status
	}
	order.UpdatedAt = time.Now()

	orders[orderID] = order

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func deleteOrderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	_, exists := orders[orderID]
	if !exists {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	delete(orders, orderID)
	activeOrders.Dec()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Order deleted successfully",
		"order_id": orderID,
	})
}

func businessMetricsHandler(w http.ResponseWriter, r *http.Request) {
	totalOrders := len(orders)
	var totalRev float64
	for _, order := range orders {
		totalRev += order.Price * float64(order.Quantity)
	}

	ordersPerMinute := float64(totalOrders) / time.Since(startTime).Minutes()
	avgOrderSize := float64(totalOrders)
	if totalOrders > 0 {
		avgOrderSize = float64(totalOrders) / float64(len(orders))
	}

	metrics := BusinessMetrics{
		TotalOrders:      totalOrders,
		TotalRevenue:     totalRev,
		OrdersPerMinute:  ordersPerMinute,
		AverageOrderSize: avgOrderSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func simulateBusinessActivity(w http.ResponseWriter, r *http.Request) {
	go func() {
		products := []string{"Laptop", "Phone", "Tablet", "Headphones", "Mouse", "Keyboard"}
		for i := 0; i < 10; i++ {
			order := Order{
				ID:        uuid.New().String(),
				Product:   products[rand.Intn(len(products))],
				Quantity:  rand.Intn(5) + 1,
				Price:     float64(rand.Intn(1000)+100) / 10,
				Status:    "completed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			orders[order.ID] = order
			activeOrders.Inc()
			totalRevenue.Add(order.Price * float64(order.Quantity))

			logrus.WithField("order_id", order.ID).Info("Simulated order created")

			time.Sleep(1 * time.Second)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Business activity simulation started",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}