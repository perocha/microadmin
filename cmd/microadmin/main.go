package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	pods, err := clientset.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=order-processing",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing pods: %v\n", err)
		os.Exit(1)
	}

	// Send a request to each pod to refresh configuration
	for _, pod := range pods.Items {
		// For demonstration purposes, we'll print the pod's name
		fmt.Println("Pod:", pod.Name)

		// You can send an HTTP request to the pod here to trigger configuration refresh
		// For example:
		// sendRefreshRequest(pod.Status.PodIP)
	}
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

func sendRefreshRequest(podIP string) {
	// Send an HTTP request to the pod's IP address to trigger configuration refresh
	// For demonstration purposes, we'll simply print a message
	fmt.Printf("Sending refresh request to pod %s\n", podIP)
}
