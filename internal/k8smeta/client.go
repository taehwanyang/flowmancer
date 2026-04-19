package k8smeta

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewKubernetesClient() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return kubernetes.NewForConfig(cfg)
	}

	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build kubeconfig: %w", err)
	}

	return kubernetes.NewForConfig(cfg)
}
