package k8smeta

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ServiceMetadata struct {
	Name      string
	Namespace string
	ClusterIP string
}

type DstResolution struct {
	Kind      string // Service, Pod, Workload
	Name      string // pretty display name
	Namespace string
	IP        string

	Service *ServiceMetadata
	Pod     *PodMetadata
}

type DstResolver struct {
	client kubernetes.Interface

	mu sync.RWMutex

	servicesByIP map[string]ServiceMetadata
	podsByIP     map[string]PodMetadata
}

func NewDstResolver(client kubernetes.Interface) *DstResolver {
	return &DstResolver{
		client:       client,
		servicesByIP: make(map[string]ServiceMetadata),
		podsByIP:     make(map[string]PodMetadata),
	}
}

func normalizeIP(ip net.IP) string {
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}

func (r *DstResolver) Start(ctx context.Context) error {
	if err := r.Refresh(ctx); err != nil {
		return err
	}

	go r.refreshLoop(ctx)
	return nil
}

func (r *DstResolver) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Refresh(ctx); err != nil {
				log.Printf("refresh dst resolver failed: %v", err)
			}
		}
	}
}

func (r *DstResolver) Refresh(ctx context.Context) error {
	services, err := r.loadServices(ctx)
	if err != nil {
		return err
	}

	pods, err := r.loadPods(ctx)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.servicesByIP = services
	r.podsByIP = pods
	r.mu.Unlock()

	return nil
}

func (r *DstResolver) ResolveDstIP(ip net.IP) (DstResolution, bool) {
	key := normalizeIP(ip)
	if key == "" {
		return DstResolution{}, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if svc, ok := r.servicesByIP[key]; ok {
		return DstResolution{
			Kind:      "Service",
			Name:      svc.Namespace + "/Service/" + svc.Name,
			Namespace: svc.Namespace,
			IP:        svc.ClusterIP,
			Service:   &svc,
		}, true
	}

	if pod, ok := r.podsByIP[key]; ok {
		name := pod.Namespace + "/Pod/" + pod.PodName
		kind := "Pod"

		if pod.WorkloadKind != "" && pod.WorkloadName != "" {
			kind = "Workload"
			name = pod.Namespace + "/" + pod.WorkloadKind + "/" + pod.WorkloadName
		}

		return DstResolution{
			Kind:      kind,
			Name:      name,
			Namespace: pod.Namespace,
			IP:        key,
			Pod:       &pod,
		}, true
	}

	return DstResolution{}, false
}

func (r *DstResolver) loadServices(ctx context.Context) (map[string]ServiceMetadata, error) {
	list, err := r.client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := make(map[string]ServiceMetadata, len(list.Items))
	for _, svc := range list.Items {
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		out[svc.Spec.ClusterIP] = ServiceMetadata{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			ClusterIP: svc.Spec.ClusterIP,
		}
	}
	return out, nil
}

func (r *DstResolver) loadPods(ctx context.Context) (map[string]PodMetadata, error) {
	list, err := r.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := make(map[string]PodMetadata, len(list.Items))
	for _, pod := range list.Items {
		if pod.Status.PodIP == "" {
			continue
		}

		meta := r.toPodMetadata(&pod)
		out[pod.Status.PodIP] = meta
	}
	return out, nil
}

func (r *DstResolver) toPodMetadata(pod *v1.Pod) PodMetadata {
	meta := PodMetadata{
		PodUID:        string(pod.UID),
		PodName:       pod.Name,
		Namespace:     pod.Namespace,
		NodeName:      pod.Spec.NodeName,
		ContainerName: firstContainerName(pod),
	}

	if len(pod.OwnerReferences) > 0 {
		owner := pod.OwnerReferences[0]
		meta.WorkloadKind = owner.Kind
		meta.WorkloadName = owner.Name
	}

	return meta
}

func firstContainerName(pod *v1.Pod) string {
	if len(pod.Spec.Containers) == 0 {
		return ""
	}
	return pod.Spec.Containers[0].Name
}
