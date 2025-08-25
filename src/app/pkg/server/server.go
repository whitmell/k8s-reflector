package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/yourorg/cluster-reflector/app/pkg/discovery"
	"github.com/yourorg/cluster-reflector/app/pkg/types"
)

// Server represents the HTTP server
type Server struct {
	router    *mux.Router
	discovery *discovery.ClusterDiscovery
	config    *types.Config
	logger    *logrus.Logger
	server    *http.Server
}

// NewServer creates a new HTTP server instance
func NewServer(cfg *types.Config, discovery *discovery.ClusterDiscovery, logger *logrus.Logger) *Server {
	s := &Server{
		router:    mux.NewRouter(),
		discovery: discovery,
		config:    cfg,
		logger:    logger,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	// Main endpoints
	s.router.HandleFunc("/cluster-info", s.handleClusterInfo).Methods("GET")
	s.router.HandleFunc("/healthz", s.handleHealthz).Methods("GET")
	
	// Optional metrics endpoint
	if s.config.MetricsEnabled {
		s.router.HandleFunc("/metrics", s.handleMetrics).Methods("GET")
	}

	// Middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:         s.config.Listen,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.WithField("address", s.config.Listen).Info("Starting HTTP server")

	// Start server in goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.WithError(err).Error("HTTP server failed")
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	s.logger.Info("Shutting down HTTP server")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.server.Shutdown(shutdownCtx)
}

// handleClusterInfo handles GET /cluster-info
func (s *Server) handleClusterInfo(w http.ResponseWriter, r *http.Request) {
	info := s.discovery.GetClusterInfo()
	
	w.Header().Set("Content-Type", "application/json")
	
	if err := json.NewEncoder(w).Encode(info); err != nil {
		s.logger.WithError(err).Error("Failed to encode cluster info")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.WithFields(logrus.Fields{
		"nodes": len(info.Nodes),
		"apps":  len(info.Apps),
	}).Debug("Served cluster info")
}

// handleHealthz handles GET /healthz
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.discovery.HealthCheck(ctx); err != nil {
		s.logger.WithError(err).Warn("Health check failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
	})
}

// handleMetrics handles GET /metrics (basic implementation)
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	info := s.discovery.GetClusterInfo()
	
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	
	fmt.Fprintf(w, "# HELP cluster_reflector_nodes_total Total number of nodes in the cluster\n")
	fmt.Fprintf(w, "# TYPE cluster_reflector_nodes_total gauge\n")
	fmt.Fprintf(w, "cluster_reflector_nodes_total %d\n", len(info.Nodes))
	
	fmt.Fprintf(w, "# HELP cluster_reflector_apps_total Total number of discovered applications\n")
	fmt.Fprintf(w, "# TYPE cluster_reflector_apps_total gauge\n")
	fmt.Fprintf(w, "cluster_reflector_apps_total %d\n", len(info.Apps))
	
	// Count control plane vs worker nodes
	controlPlaneNodes := 0
	workerNodes := 0
	for _, node := range info.Nodes {
		if node.Role == "control-plane" {
			controlPlaneNodes++
		} else {
			workerNodes++
		}
	}
	
	fmt.Fprintf(w, "# HELP cluster_reflector_control_plane_nodes Total number of control plane nodes\n")
	fmt.Fprintf(w, "# TYPE cluster_reflector_control_plane_nodes gauge\n")
	fmt.Fprintf(w, "cluster_reflector_control_plane_nodes %d\n", controlPlaneNodes)
	
	fmt.Fprintf(w, "# HELP cluster_reflector_worker_nodes Total number of worker nodes\n")
	fmt.Fprintf(w, "# TYPE cluster_reflector_worker_nodes gauge\n")
	fmt.Fprintf(w, "cluster_reflector_worker_nodes %d\n", workerNodes)
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response recorder to capture status code
		rr := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(rr, r)
		
		duration := time.Since(start)
		
		s.logger.WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     rr.statusCode,
			"duration":   duration,
			"user_agent": r.UserAgent(),
			"remote_addr": r.RemoteAddr,
		}).Info("HTTP request")
	})
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// responseRecorder wraps http.ResponseWriter to capture status code
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}
