package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Create a Kubernetes clientset
	clientset, err := createClientset()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes clientset: %v\n", err)
		os.Exit(1)
	}

	// List pods matching a specific label selector
	pods, err := listPods(clientset, "producer")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing pods: %v\n", err)
		os.Exit(1)
	}

	// Iterate over the pods and send a refresh request to each one
	err = refreshConfig(pods)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending refresh request: %v\n", err)
		os.Exit(1)
	}
}

func listPods(clientset *kubernetes.Clientset, appname string) (*v1.PodList, error) {
	// List pods matching a specific label selector
	pods, err := clientset.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + appname,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	return pods, nil
}

// Iterate over the pods and send a refresh request to each one
func refreshConfig(pods *v1.PodList) error {
	// Iterate over the pods and send a refresh request to each one
	for _, pod := range pods.Items {
		podIP := pod.Status.PodIP
		if podIP == "" {
			fmt.Printf("Pod %s does not have an IP address\n", pod.Name)
			continue
		}

		err := sendRefreshRequest(podIP)
		if err != nil {
			fmt.Printf("Error sending refresh request to pod %s: %v\n", podIP, err)
		}
	}

	return nil
}

func createClientset() (*kubernetes.Clientset, error) {
	// Path to the Kubernetes configuration file
	kubeconfig := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")

	// Load Kubernetes config from file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load Kubernetes config: %v", err)
	}

	// Create a Kubernetes clientset using the loaded configuration
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %v", err)
	}

	return clientset, nil
}

func sendRefreshRequest(podIP string) error {
	// Send an HTTP POST request to the pod's IP address to trigger configuration refresh
	url := fmt.Sprintf("http://%s:8080/refresh-config", podIP)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	fmt.Printf("Refresh request sent to pod %s\n", podIP)
	return nil
}
