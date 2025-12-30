package kubernetes

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	// Namespace for workspaces
	WorkspaceNamespace = "justup-workspaces"
	// Label for identifying justup workspaces
	WorkspaceLabel = "justup.io/workspace"
	// Annotation for storing git URL
	GitURLAnnotation = "justup.io/git-url"
	// Annotation for storing branch
	BranchAnnotation = "justup.io/branch"
)

// Client wraps the Kubernetes client
type Client struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
}

// NewClient creates a new Kubernetes client
func NewClient() (*Client, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset:  clientset,
		restConfig: config,
	}, nil
}

// getKubeConfig returns the Kubernetes configuration
func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig file
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

// EnsureNamespace creates the workspace namespace if it doesn't exist
func (c *Client) EnsureNamespace(ctx context.Context) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: WorkspaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "justup",
				"app.kubernetes.io/component": "workspaces",
			},
		},
	}

	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, WorkspaceNamespace, metav1.GetOptions{})
	if err == nil {
		return nil // Already exists
	}

	_, err = c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

// PortForward establishes a port-forward to a workspace pod
func (c *Client) PortForward(ctx context.Context, name string, localPort, remotePort int, ready chan struct{}) error {
	podName := "ws-" + name

	// Get the pod to ensure it exists
	pod, err := c.clientset.CoreV1().Pods(WorkspaceNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("pod not found: %w", err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("pod is not running (phase: %s)", pod.Status.Phase)
	}

	// Build the port-forward request
	url := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(WorkspaceNamespace).
		Name(podName).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(c.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		close(stopCh)
	}()

	pf, err := portforward.New(dialer, ports, stopCh, readyCh, nil, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	// Signal that we're ready
	go func() {
		<-readyCh
		close(ready)
	}()

	return pf.ForwardPorts()
}
