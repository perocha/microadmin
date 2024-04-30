package main

import (
	"context"
	"log"
	"net/http"

	"github.com/perocha/goutils/pkg/telemetry"
	"github.com/perocha/microadmin/pkg/config"
	"github.com/perocha/microadmin/pkg/service"
)

const SERVICE_NAME = "microadmin"

// Initialize configuration
func initialize() (*config.MicroserviceConfig, error) {
	// Load the configuration
	cfg, err := config.InitializeConfig()
	if err != nil {
		return nil, err
	}
	// Refresh the configuration with the latest values
	err = cfg.RefreshConfig()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func main() {
	// Initialize config
	cfg, err := initialize()
	if err != nil {
		log.Fatalf("Main::Fatal error::Failed to load configuration %s\n", err.Error())
	}

	// Initialize telemetry package
	telemetryConfig := telemetry.NewXTelemetryConfig(cfg.AppInsightsInstrumentationKey, SERVICE_NAME, "info", 1)
	xTelemetry, err := telemetry.NewXTelemetry(telemetryConfig)
	if err != nil {
		log.Fatalf("Main::Fatal error::Failed to initialize XTelemetry %s\n", err.Error())
	}
	// Add telemetry object to the context, so that it can be reused across the application
	ctx := context.WithValue(context.Background(), telemetry.TelemetryContextKey, xTelemetry)

	k8scli, err := service.InitializePodManager(ctx)
	if err != nil {
		xTelemetry.Error(ctx, "Main::Failed to initialize PodManager", telemetry.String("Error", err.Error()))
		panic(err)
	}

	// Define an HTTP handler function for refreshing configuration
	http.HandleFunc("/refresh-config", func(w http.ResponseWriter, r *http.Request) {
		err := k8scli.RefreshConfig(ctx, "producer")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	// Start an HTTP server
	go func() {
		err := http.ListenAndServe(":"+cfg.HttpPortNumber, nil)
		if err != nil {
			xTelemetry.Error(ctx, "Main::Failed to start HTTP server", telemetry.String("Error", err.Error()))
			panic(err)
		}
	}()

	// Wait forever
	xTelemetry.Info(ctx, "Main::Microadmin started successfully", telemetry.String("HttpPortNumber", cfg.HttpPortNumber))
	select {}
}
