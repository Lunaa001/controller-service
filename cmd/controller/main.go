package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"service-controller-notebookum/internal/config"
	"service-controller-notebookum/internal/web"
)

// runHealthCheck is invoked as `controller healthcheck` from the Dockerfile's
// HEALTHCHECK instruction. The runtime image is gcr.io/distroless/static —
// it has no shell, curl or wget, so the binary must check its own health.
func runHealthCheck() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:" + port + "/health")
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		runHealthCheck()
		return
	}

	cfg := config.Load()
	router := web.NewRouter(cfg)

	// ─── Consul Service Registration ────────────────────────────────────
	// Tags are read from Consul KV automatically (no hardcoded labels)
	consulCfg := config.LoadConsulConfig()
	servicePort, _ := strconv.Atoi(cfg.Port)
	if servicePort == 0 {
		servicePort = 5000
	}

	if err := consulCfg.RegisterService(
		"controller-service",
		servicePort,
		"/health",
		nil, // tags fetched from Consul KV
	); err != nil {
		log.Printf("warning: consul registration failed: %v (service will still start)", err)
	}

	// ─── HTTP Server with Graceful Shutdown ──────────────────────────────
	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Controller service starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Deregister from Consul
	if err := consulCfg.DeregisterService("controller-service", servicePort); err != nil {
		log.Printf("warning: consul deregistration failed: %v", err)
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server forced shutdown: %v", err)
	}

	log.Println("Server stopped")
}
