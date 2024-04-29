package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/perocha/goutils/pkg/telemetry"
	"github.com/perocha/microadmin/pkg/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

	// Create a Kubernetes clientset
	clientset, err := createClientset(ctx)
	if err != nil {
		xTelemetry.Error(ctx, "Main::Failed to create Kubernetes clientset", telemetry.String("Error", err.Error()))
		panic(err)
	}

	// Define an HTTP handler function for refreshing configuration
	http.HandleFunc("/refresh-config", func(w http.ResponseWriter, r *http.Request) {
		err := RefreshConfig(ctx, clientset, "producer")
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

func RefreshConfig(ctx context.Context, clientset *kubernetes.Clientset, appname string) error {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)

	// List pods matching a specific label selector
	pods, err := listPods(ctx, clientset, appname)
	if err != nil {
		xTelemetry.Error(ctx, "RefreshConfig::Failed to list pods", telemetry.String("Error", err.Error()))
		return err
	}

	// Iterate over the pods and send a refresh request to each one
	for _, pod := range pods.Items {
		podIP := pod.Status.PodIP
		if podIP == "" {
			xTelemetry.Info(ctx, "Pod does not have an IP address", telemetry.String("PodName", pod.Name))
			continue
		}

		err := sendRefreshRequest(ctx, podIP)
		if err != nil {
			xTelemetry.Error(ctx, "Failed to send refresh request to pod", telemetry.String("PodName", pod.Name), telemetry.String("Error", err.Error()))
			return err
		}
	}

	xTelemetry.Info(ctx, "Refresh request sent to all pods", telemetry.String("AppName", appname))
	return nil
}

func listPods(ctx context.Context, clientset *kubernetes.Clientset, appname string) (*v1.PodList, error) {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)

	// List pods matching a specific label selector
	pods, err := clientset.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + appname,
	})
	if err != nil {
		xTelemetry.Error(ctx, "listPods::Failed to list pods", telemetry.String("Error", err.Error()))
		return nil, err
	}

	xTelemetry.Info(ctx, "Pods listed successfully", telemetry.Int("PodCount", len(pods.Items)))
	return pods, nil
}

func createClientset(ctx context.Context) (*kubernetes.Clientset, error) {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)

	// Path to the Kubernetes configuration file
	//	kubeconfig := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	xTelemetry.Info(ctx, "Kubeconfig file", telemetry.String("Kubeconfig", kubeconfig))

	// Load Kubernetes config from file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		xTelemetry.Error(ctx, "Failed to load Kubernetes config", telemetry.String("Error", err.Error()))
		return nil, err
	}

	// Create a Kubernetes clientset using the loaded configuration
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		xTelemetry.Error(ctx, "Failed to create Kubernetes clientset", telemetry.String("Error", err.Error()))
		return nil, err
	}

	xTelemetry.Info(ctx, "Kubernetes clientset created successfully")
	return clientset, nil
}

func sendRefreshRequest(ctx context.Context, podIP string) error {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)

	// Send an HTTP POST request to the pod's IP address to trigger configuration refresh
	url := fmt.Sprintf("http://%s:8081/refresh-config", podIP)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		xTelemetry.Error(ctx, "Failed to send refresh request to pod", telemetry.String("PodIP", podIP), telemetry.String("Error", err.Error()))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		xTelemetry.Error(ctx, "Unexpected status code", telemetry.String("PodIP", podIP), telemetry.Int("StatusCode", resp.StatusCode))
		return err
	}

	xTelemetry.Info(ctx, "Refresh request sent to pod", telemetry.String("PodIP", podIP))
	return nil
}
