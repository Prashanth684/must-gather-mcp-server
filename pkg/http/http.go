package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-mcp-server/pkg/config"
)

const (
	healthEndpoint  = "/healthz"
	statsEndpoint   = "/stats"
	metricsEndpoint = "/metrics"
	mcpEndpoint     = "/mcp"
	sseEndpoint     = "/sse"
	messageEndpoint = "/message"
)

// MCPServer is an interface for the MCP server to avoid circular dependencies
type MCPServer interface {
	ServeSSE() http.Handler
	ServeHTTP() http.Handler
	GetStats() map[string]interface{}
	Shutdown(context.Context) error
}

// Serve starts the HTTP server with the given configuration.
func Serve(ctx context.Context, mcpServer MCPServer, staticConfig *config.StaticConfig, oidcProvider *oidc.Provider, httpClient *http.Client) error {
	mux := http.NewServeMux()

	wrappedMux := RequestMiddleware(
		AuthorizationMiddleware(staticConfig, oidcProvider)(mux),
	)

	httpServer := &http.Server{
		Addr:    ":" + staticConfig.Port,
		Handler: wrappedMux,
	}

	sseServer := mcpServer.ServeSSE()
	streamableHttpServer := mcpServer.ServeHTTP()
	mux.Handle(sseEndpoint, sseServer)
	mux.Handle(messageEndpoint, sseServer)
	mux.Handle(mcpEndpoint, streamableHttpServer)
	mux.HandleFunc(healthEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(statsEndpoint, statsHandler(mcpServer))
	mux.Handle("/.well-known/", WellKnownHandler(staticConfig, httpClient))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		klog.V(0).Infof("HTTP server starting on port %s (endpoints: /mcp, /sse, /message, /healthz, /stats)", staticConfig.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case sig := <-sigChan:
		klog.V(0).Infof("Received signal %v, initiating graceful shutdown", sig)
		cancel()
	case <-ctx.Done():
		klog.V(0).Infof("Context cancelled, initiating graceful shutdown")
	case err := <-serverErr:
		klog.Errorf("HTTP server error: %v", err)
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	klog.V(0).Infof("Shutting down HTTP server gracefully...")

	// Attempt to shut down both servers, collecting all errors
	var shutdownErrs []error

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("HTTP server shutdown error: %v", err)
		shutdownErrs = append(shutdownErrs, err)
	}

	// Always attempt MCP server shutdown even if HTTP shutdown failed
	if err := mcpServer.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("MCP server shutdown error: %v", err)
		shutdownErrs = append(shutdownErrs, err)
	}

	if len(shutdownErrs) > 0 {
		return errors.Join(shutdownErrs...)
	}

	klog.V(0).Infof("HTTP server shutdown complete")
	return nil
}

// statsHandler returns an HTTP handler that exposes server statistics as JSON.
func statsHandler(mcpServer MCPServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		stats := mcpServer.GetStats()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			klog.V(1).Infof("Failed to encode stats response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}
