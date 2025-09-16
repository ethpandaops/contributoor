package application

import (
	"context"
	"net/http"

	// We are safe to expose this import as we are using a custom
	// handler only enabled if the pprof flag is on.
	_ "net/http/pprof" // #nosec G108
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// startMetricsServer starts the Prometheus metrics server if configured.
func (a *Application) startMetricsServer() error {
	if a.config.MetricsAddress == "" {
		a.log.Info("Metrics server disabled")

		return nil
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	a.servers.metricsServer = &http.Server{
		Addr:              a.config.MetricsAddress,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		a.log.WithField("address", a.config.MetricsAddress).Info("Starting metrics server")

		if err := a.servers.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.WithError(err).Error("Failed to start metrics server")
		}
	}()

	return nil
}

// startPProfServer starts the pprof debug server if configured.
func (a *Application) startPProfServer() error {
	if a.config.PprofAddress == "" {
		a.log.Info("PProf server disabled")

		return nil
	}

	// pprof handlers are registered by importing net/http/pprof
	a.servers.pprofServer = &http.Server{
		Addr:              a.config.PprofAddress,
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		a.log.WithField("address", a.config.PprofAddress).Info("Starting pprof server")

		if err := a.servers.pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.WithError(err).Error("Failed to start pprof server")
		}
	}()

	return nil
}

// startHealthCheckServer starts the health check server.
// If no address is configured, it defaults to 127.0.0.1:9191.
func (a *Application) startHealthCheckServer() error {
	address := a.config.HealthCheckAddress
	if address == "" {
		// Use default address if none configured
		address = "127.0.0.1:9191"
		a.log.WithField("address", address).Info("Starting health check server with default address")
	} else {
		a.log.WithField("address", address).Info("Starting health check server")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.handleHealthCheck)

	a.servers.healthCheckServer = &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		if err := a.servers.healthCheckServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.WithError(err).Error("Failed to start health check server")
		}
	}()

	return nil
}

// stopServers gracefully shuts down all HTTP servers.
func (a *Application) stopServers(ctx context.Context) {
	// Shutdown metrics server
	if a.servers.metricsServer != nil {
		a.log.Debug("Shutting down metrics server")

		if err := a.servers.metricsServer.Shutdown(ctx); err != nil {
			a.log.WithError(err).Error("Failed to gracefully shutdown metrics server")
		}
	}

	// Shutdown pprof server
	if a.servers.pprofServer != nil {
		a.log.Debug("Shutting down pprof server")

		if err := a.servers.pprofServer.Shutdown(ctx); err != nil {
			a.log.WithError(err).Error("Failed to gracefully shutdown pprof server")
		}
	}

	// Shutdown health check server
	if a.servers.healthCheckServer != nil {
		a.log.Debug("Shutting down health check server")

		if err := a.servers.healthCheckServer.Shutdown(ctx); err != nil {
			a.log.WithError(err).Error("Failed to gracefully shutdown health check server")
		}
	}
}

// Servers returns the server manager for testing purposes.
func (a *Application) Servers() *ServerManager {
	return a.servers
}
