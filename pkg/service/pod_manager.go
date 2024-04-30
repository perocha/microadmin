package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/perocha/goutils/pkg/telemetry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type PodManagerImpl struct {
	k8scli *kubernetes.Clientset
}

func InitializePodManager(ctx context.Context) (*PodManagerImpl, error) {
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
		k8scli: clientset,
	}, nil
}

func (p *PodManagerImpl) RefreshConfig(ctx context.Context, appname string) error {
	xTelemetry := telemetry.GetXTelemetryClient(ctx)

	// List pods matching a specific label selector
	pods, err := p.ListPods(ctx, appname)
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

		err := p.sendRefreshRequest(ctx, podIP)
		if err != nil {
			xTelemetry.Error(ctx, "Failed to send refresh request to pod", telemetry.String("PodName", pod.Name), telemetry.String("Error", err.Error()))
			return err
		}
	}

	xTelemetry.Info(ctx, "Refresh request sent to all pods", telemetry.String("AppName", appname))
	return nil
}

func (p *PodManagerImpl) ListPods(ctx context.Context, appname string) (*v1.PodList, error) {
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

func (p *PodManagerImpl) sendRefreshRequest(ctx context.Context, podIP string) error {
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
