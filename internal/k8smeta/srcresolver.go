package k8smeta

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PodMetadata struct {
	PodUID        string
	PodName       string
	Namespace     string
	NodeName      string
	ContainerName string

	WorkloadKind string
	WorkloadName string
}

type NetnsCacheEntry struct {
	NetnsIno  uint32
	Pod       PodMetadata
	PID       int
	UpdatedAt time.Time
}

type SrcResolver struct {
	client   kubernetes.Interface
	nodeName string

	mu sync.RWMutex

	podsByUID  map[string]*v1.Pod
	rsToDeploy map[string]string // namespace/name(rs) -> deployment name
	netnsToPod map[uint32]NetnsCacheEntry
}

func NewSrcResolver(client kubernetes.Interface, nodeName string) *SrcResolver {
	return &SrcResolver{
		client:     client,
		nodeName:   nodeName,
		podsByUID:  make(map[string]*v1.Pod),
		rsToDeploy: make(map[string]string),
		netnsToPod: make(map[uint32]NetnsCacheEntry),
	}
}

func (r *SrcResolver) Start(ctx context.Context) error {
	factory := informers.NewSharedInformerFactory(r.client, 0)

	podInformer := factory.Core().V1().Pods().Informer()
	rsInformer := factory.Apps().V1().ReplicaSets().Informer()

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if pod, ok := obj.(*v1.Pod); ok {
				r.mu.Lock()
				r.podsByUID[string(pod.UID)] = pod
				r.mu.Unlock()
			}
		},
		UpdateFunc: func(_, newObj any) {
			if pod, ok := newObj.(*v1.Pod); ok {
				r.mu.Lock()
				r.podsByUID[string(pod.UID)] = pod
				r.mu.Unlock()
			}
		},
		DeleteFunc: func(obj any) {
			if pod, ok := obj.(*v1.Pod); ok {
				r.mu.Lock()
				delete(r.podsByUID, string(pod.UID))
				r.mu.Unlock()
			}
		},
	})

	rsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if rs, ok := obj.(*appsv1.ReplicaSet); ok {
				r.cacheReplicaSet(rs)
			}
		},
		UpdateFunc: func(_, newObj any) {
			if rs, ok := newObj.(*appsv1.ReplicaSet); ok {
				r.cacheReplicaSet(rs)
			}
		},
		DeleteFunc: func(obj any) {
			if rs, ok := obj.(*appsv1.ReplicaSet); ok {
				key := rs.Namespace + "/" + rs.Name
				r.mu.Lock()
				delete(r.rsToDeploy, key)
				r.mu.Unlock()
			}
		},
	})

	factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced, rsInformer.HasSynced) {
		return fmt.Errorf("wait for informer sync failed")
	}

	go r.refreshLoop(ctx)
	return nil
}

func (r *SrcResolver) cacheReplicaSet(rs *appsv1.ReplicaSet) {
	key := rs.Namespace + "/" + rs.Name
	deployName := ""

	for _, owner := range rs.OwnerReferences {
		if owner.Kind == "Deployment" {
			deployName = owner.Name
			break
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.rsToDeploy[key] = deployName
}

func (r *SrcResolver) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 시작 시 1회
	r.refreshNetnsCache()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.refreshNetnsCache()
		}
	}
}

func (r *SrcResolver) refreshNetnsCache() {
	procs, err := ScanProcForPodNetns()
	if err != nil {
		log.Printf("refrehNetnsCache: scan proc failed: %v", err)
		return
	}

	now := time.Now()
	next := make(map[uint32]NetnsCacheEntry, len(procs))

	r.mu.RLock()
	for _, p := range procs {
		pod, ok := r.podsByUID[p.PodUID]
		if !ok {
			continue
		}
		if r.nodeName != "" && pod.Spec.NodeName != r.nodeName {
			continue
		}

		meta := PodMetadata{
			PodUID:    string(pod.UID),
			PodName:   pod.Name,
			Namespace: pod.Namespace,
			NodeName:  pod.Spec.NodeName,
		}
		meta.WorkloadKind, meta.WorkloadName = r.resolveWorkloadLocked(pod)

		next[p.NetnsIno] = NetnsCacheEntry{
			NetnsIno:  p.NetnsIno,
			Pod:       meta,
			PID:       p.PID,
			UpdatedAt: now,
		}
	}
	r.mu.RUnlock()

	r.mu.Lock()
	r.netnsToPod = next
	r.mu.Unlock()
}

func (r *SrcResolver) resolveWorkloadLocked(pod *v1.Pod) (string, string) {
	for _, owner := range pod.OwnerReferences {
		switch owner.Kind {
		case "DaemonSet", "StatefulSet", "Job":
			return owner.Kind, owner.Name
		case "ReplicaSet":
			key := pod.Namespace + "/" + owner.Name
			if deployName := r.rsToDeploy[key]; deployName != "" {
				return "Deployment", deployName
			}
			return "ReplicaSet", owner.Name
		}
	}
	return "Pod", pod.Name
}

func (r *SrcResolver) ResolveNetns(netnsIno uint32) (PodMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.netnsToPod[netnsIno]
	if !ok {
		return PodMetadata{}, false
	}
	return entry.Pod, true
}

func (r *SrcResolver) SnapshotMappings() []NetnsCacheEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]NetnsCacheEntry, 0, len(r.netnsToPod))
	for _, v := range r.netnsToPod {
		out = append(out, v)
	}
	return out
}
