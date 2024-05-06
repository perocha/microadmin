package service

import (
	"context"
	"os"
	"path/filepath"

	"github.com/perocha/goadapters/comms"
	"github.com/perocha/goadapters/comms/httpadapter"
	"github.com/perocha/goadapters/messaging"
	"github.com/perocha/goutils/pkg/telemetry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type PodManagerImpl struct {
	k8scli       *kubernetes.Clientset
	httpSender   comms.CommsSender
	httpReceiver comms.CommsReceiver
}

// Initializes a new PodManagerImpl object
func InitializePodManager(ctx context.Context, httpSender comms.CommsSender, httpReceiver comms.CommsReceiver) (*PodManagerImpl, error) {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)

	// First we try to load the Kubernetes configuration from the in-cluster configuration
	// Path to the Kubernetes configuration file
	kubeconfig := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
	// kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	xTelemetry.Info(ctx, "Kubeconfig file", telemetry.String("Kubeconfig", kubeconfig))
	// Load Kubernetes config from file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)

	if err != nil {
		xTelemetry.Info(ctx, "Failed to load Kubernetes config, load config from in-cluster", telemetry.String("Error", err.Error()))

		// Failed to load k8s config file, now try to load Kubernetes config from in-cluster configuration
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	// Create a Kubernetes clientset using the loaded configuration
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &PodManagerImpl{
		k8scli:       clientset,
		httpSender:   httpSender,
		httpReceiver: httpReceiver,
	}, nil
}

// RefreshConfiguration is a callback function that is called when a refresh request is received
func (p *PodManagerImpl) RefreshConfiguration(ctx context.Context, w comms.ResponseWriter, r comms.Request) {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)
	xTelemetry.Info(ctx, "RefreshConfiguration")

	// Get the application name from the request
	appname := r.Header("appname")
	xTelemetry.Debug(ctx, "RefreshConfiguration::Application name", telemetry.String("Appname", appname))

	// List pods matching a specific label selector
	pods, err := p.listPods(ctx, appname)
	if err != nil {
		xTelemetry.Error(ctx, "RefreshConfig::Failed to list pods", telemetry.String("Error", err.Error()))
		w.WriteHeader(int(httpadapter.StatusInternalServerError))
		w.Write([]byte("Failed to list pods"))
		return
	}

	// Iterate over the pods and send a refresh request to each one
	for _, pod := range pods.Items {
		podIP := pod.Status.PodIP
		if podIP == "" {
			xTelemetry.Info(ctx, "Pod does not have an IP address", telemetry.String("PodName", pod.Name))
			continue
		}

		msg := messaging.NewMessage("", nil, "status", "command", nil)
		endpoint := httpadapter.NewEndpoint(podIP, "8081", "/refresh-config")
		err := p.httpSender.SendRequest(ctx, endpoint, msg)
		if err != nil {
			xTelemetry.Error(ctx, "Failed to send refresh request to pod", telemetry.String("PodIP", podIP), telemetry.String("Error", err.Error()))
			w.WriteHeader(int(httpadapter.StatusInternalServerError))
			w.Write([]byte("Failed to send refresh request to pod"))
			return
		}

		xTelemetry.Debug(ctx, "Refresh request sent to pod", telemetry.String("PodIP", podIP))
	}

	xTelemetry.Debug(ctx, "Refresh request sent to all pods", telemetry.String("Appname", appname))
	w.WriteHeader(int(httpadapter.StatusOK))
	w.Write([]byte("Refresh request sent to all pods"))
	return
}

func (p *PodManagerImpl) TestCallback(ctx context.Context, w comms.ResponseWriter, r comms.Request) {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)
	xTelemetry.Info(ctx, "TestCallback")
	w.WriteHeader(int(httpadapter.StatusOK))
	w.Write([]byte("Hello World 2"))
}

// Start the pod manager
func (p *PodManagerImpl) Start(ctx context.Context, signals <-chan os.Signal) error {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)
	xTelemetry.Info(ctx, "PodManager started")

	// Initialize the HTTP server for receiving messages
	err := p.httpReceiver.RegisterEndPoint(ctx, "/refresh", p.RefreshConfiguration)
	if err != nil {
		return err
	}

	err = p.httpReceiver.RegisterEndPoint(ctx, "/prueba", p.TestCallback)
	if err != nil {
		return err
	}

	// Start the HTTP server for receiving messages
	err = p.httpReceiver.Start(ctx)
	if err != nil {
		xTelemetry.Error(ctx, "Failed to start HTTP server", telemetry.String("Error", err.Error()))
		return err
	}

	// Wait for termination signals
	for range signals {
		// Termination signal received
		xTelemetry.Info(ctx, "Received termination signal")
		p.Stop(ctx)
		return nil
	}

	return nil
}

// Stop the pod manager
func (p *PodManagerImpl) Stop(ctx context.Context) {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)
	xTelemetry.Info(ctx, "PodManager stopped")

	// Close the Kubernetes clientset
	p.k8scli = nil
}

// List all pods for a specific application
func (p *PodManagerImpl) listPods(ctx context.Context, appname string) (*v1.PodList, error) {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)

	// List pods matching a specific label selector
	pods, err := p.k8scli.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + appname,
	})
	if err != nil {
		xTelemetry.Error(ctx, "listPods::Failed to list pods", telemetry.String("Error", err.Error()))
		return nil, err
	}

	//xTelemetry.Info(ctx, "Pods listed successfully", telemetry.Int("PodCount", len(pods.Items)))
	xTelemetry.Info(ctx, "Pods listed successfully")
	return pods, nil
}
