package discovery

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubernetesDiscoverer finds services in a Kubernetes cluster.
type KubernetesDiscoverer struct {
	client    kubernetes.Interface
	namespace string
	selector  string // label selector (e.g., "app=meerkat")
}

// NewKubernetes creates a K8s discoverer.
// If namespace is empty, it discovers across all namespaces.
func NewKubernetes(namespace, selector string) (*KubernetesDiscoverer, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("create in-cluster config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create k8s client: %w", err)
	}

	return &KubernetesDiscoverer{
		client:    client,
		namespace: namespace,
		selector:  selector,
	}, nil
}

// NewKubernetesWithClient creates a K8s discoverer with an existing client.
func NewKubernetesWithClient(client kubernetes.Interface, namespace, selector string) *KubernetesDiscoverer {
	return &KubernetesDiscoverer{
		client:    client,
		namespace: namespace,
		selector:  selector,
	}
}

// Name returns the discoverer name.
func (k *KubernetesDiscoverer) Name() string {
	return "kubernetes"
}

// Discover returns services/endpoints as targets.
func (k *KubernetesDiscoverer) Discover(ctx context.Context) ([]Target, error) {
	endpoints, err := k.client.CoreV1().Endpoints(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: k.selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}

	var targets []Target
	for _, ep := range endpoints.Items {
		for _, subset := range ep.Subsets {
			for _, addr := range subset.Addresses {
				for _, port := range subset.Ports {
					targets = append(targets, Target{
						Name:      ep.Name,
						Address:   fmt.Sprintf("%s:%d", addr.IP, port.Port),
						Namespace: ep.Namespace,
						Labels:    ep.Labels,
					})
				}
			}
		}
	}

	return targets, nil
}
