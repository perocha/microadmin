package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/perocha/goadapters/comms/httpadapter"
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
	telemetryConfig := telemetry.NewXTelemetryConfig(cfg.AppInsightsInstrumentationKey, SERVICE_NAME, "debug", 1)
	xTelemetry, err := telemetry.NewXTelemetry(telemetryConfig)
	if err != nil {
		log.Fatalf("Main::Fatal error::Failed to initialize XTelemetry %s\n", err.Error())
	}
	// Add telemetry object to the context, so that it can be reused across the application
	ctx := context.WithValue(context.Background(), telemetry.TelemetryContextKey, xTelemetry)

	// Initialize the http adapter for publishing messages
	httpSendAdapter, err := httpadapter.HttpSenderInit(ctx, "", "", "")
	if err != nil {
		xTelemetry.Error(ctx, "Main::Failed to initialize HTTP Publisher", telemetry.String("Error", err.Error()))
		panic(err)
	}

	// Initialize the http adapter for consuming messages
	endpoint := httpadapter.NewEndpoint("localhost", "8080", "/")
	httpServerAdapter, err := httpadapter.HTTPServerAdapterInit(ctx, endpoint)
	if err != nil {
		xTelemetry.Error(ctx, "Main::Failed to initialize HTTP Consumer", telemetry.String("Error", err.Error()))
		panic(err)
	}

	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Initialize and start the pod manager
	podManager, err := service.InitializePodManager(ctx, httpSendAdapter, httpServerAdapter)
	if err != nil {
		xTelemetry.Error(ctx, "Main::Failed to initialize Pod Manager", telemetry.String("Error", err.Error()))
		panic(err)
	}
	go podManager.Start(ctx, signals)

	// Wait for a termination signal
	for {
		select {
		case <-signals:
			// Termination signal received
			xTelemetry.Info(ctx, "Main::Received termination signal")
			return
		case <-time.After(2 * time.Minute):
			// Do nothing
			xTelemetry.Debug(ctx, "Main::Waiting for termination signal")
		}
	}
}
