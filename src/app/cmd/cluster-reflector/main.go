package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yourorg/cluster-reflector/app/pkg/discovery"
	"github.com/yourorg/cluster-reflector/app/pkg/server"
	"github.com/yourorg/cluster-reflector/app/pkg/types"
)

var (
	// Version is set during build
	Version = "dev"
	// GitCommit is set during build
	GitCommit = "unknown"
	// BuildDate is set during build
	BuildDate = "unknown"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cluster-reflector",
	Short: "Kubernetes cluster metadata and application version reflector",
	Long: `cluster-reflector is a service that provides real-time information about 
your Kubernetes cluster, including node metadata and application versions.

It serves HTTP endpoints:
  - GET /cluster-info: Returns cluster nodes and application versions
  - GET /healthz: Health check endpoint
  - GET /metrics: Prometheus metrics (if enabled)`,
	RunE: runServer,
}

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Perform a health check and exit",
	Long:  "Perform a health check against the local server and exit with appropriate code",
	RunE:  runHealthcheck,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cluster-reflector version %s\n", Version)
		fmt.Printf("Git commit: %s\n", GitCommit)
		fmt.Printf("Build date: %s\n", BuildDate)
	},
}

var config = &types.Config{}

func init() {
	// Add subcommands
	rootCmd.AddCommand(healthcheckCmd)
	rootCmd.AddCommand(versionCmd)

	// Server flags
	rootCmd.Flags().StringVar(&config.Listen, "listen", ":8080", "Address to listen on")
	rootCmd.Flags().DurationVar(&config.CacheTTL, "cache-ttl", 10*time.Second, "Cache TTL for cluster data")
	rootCmd.Flags().StringVar(&config.NamespaceSelector, "namespace-selector", "", "Namespace selector for app discovery (empty = all namespaces)")
	rootCmd.Flags().BoolVar(&config.PreferCRD, "prefer-crd", true, "Prefer AppVersion CRDs over workload discovery")
	rootCmd.Flags().BoolVar(&config.FallbackWorkloads, "fallback-workloads", true, "Enable workload fallback discovery")
	rootCmd.Flags().BoolVar(&config.CRDOnly, "crd-only", false, "Only discover from AppVersion CRDs, ignore workload discovery")
	rootCmd.Flags().StringVar(&config.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.Flags().StringSliceVar(&config.WorkloadKinds, "workload-kinds", []string{"Deployment", "StatefulSet"}, "Workload kinds to discover")
	rootCmd.Flags().BoolVar(&config.MetricsEnabled, "metrics", false, "Enable Prometheus metrics endpoint")

	// Healthcheck flags
	healthcheckCmd.Flags().StringVar(&config.Listen, "listen", ":8080", "Address to check")
}

func runServer(cmd *cobra.Command, args []string) error {
	// Setup logging
	logger := setupLogging(config.LogLevel)
	
	logger.WithFields(logrus.Fields{
		"version":    Version,
		"git_commit": GitCommit,
		"build_date": BuildDate,
	}).Info("Starting cluster-reflector")

	// Create discovery service
	disc, err := discovery.NewClusterDiscovery(config, logger)
	if err != nil {
		return fmt.Errorf("failed to create discovery service: %w", err)
	}

	// Create HTTP server
	srv := server.NewServer(config, disc, logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start discovery service
	go func() {
		if err := disc.Start(ctx); err != nil {
			logger.WithError(err).Error("Discovery service failed")
			cancel()
		}
	}()

	// Start HTTP server
	go func() {
		if err := srv.Start(ctx); err != nil {
			logger.WithError(err).Error("HTTP server failed")
			cancel()
		}
	}()

	// Wait for shutdown signal or context cancellation
	select {
	case sig := <-sigCh:
		logger.WithField("signal", sig).Info("Received shutdown signal")
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	// Graceful shutdown
	logger.Info("Shutting down...")
	cancel()

	// Stop discovery
	disc.Stop()

	logger.Info("Shutdown complete")
	return nil
}

func runHealthcheck(cmd *cobra.Command, args []string) error {
	// Parse listen address to get host and port
	addr := config.Listen
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}

	// Make HTTP request to health endpoint
	url := fmt.Sprintf("http://%s/healthz", addr)
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check failed: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("Health check passed")
	return nil
}

func setupLogging(level string) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	switch strings.ToLower(level) {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
		logger.WithField("level", level).Warn("Unknown log level, using info")
	}

	// Set JSON formatter for structured logging
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	return logger
}
