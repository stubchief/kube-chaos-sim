package k8s

import (
	"log"
	"sync"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
)

type PodInfo struct {
	Key          string
	Name         string
	Namespace    string
	Phase        string
	Status       string
	NodeName     string
	Zone         string
	PodIP        string
	Ready        bool
	RestartCount int32
	Age          time.Time
	Deleting     bool
	LastRestart  time.Time
	CPUUsage     string
}

type HPAInfo struct {
	Name               string
	Namespace          string
	TargetRef          string
	MinReplicas        int32
	MaxReplicas        int32
	CurrentReplicas    int32
	DesiredReplicas    int32
	TargetCPUUtilization int32
	CurrentCPUUtilization int32
}

type PDBInfo struct {
	Name               string
	Namespace          string
	MinAvailable       int32
	MaxUnavailable     int32
	CurrentHealthy     int32
	DesiredHealthy     int32
	DisruptionsAllowed int32
}

type Store struct {
	mu        sync.RWMutex
	pods      map[string]*PodInfo
	nodeZones map[string]string
	hpas      map[string]*HPAInfo
	pdbs      map[string]*PDBInfo
	onChange  func()
}

func NewStore(onChange func()) *Store {
	return &Store{
		pods:      make(map[string]*PodInfo),
		nodeZones: make(map[string]string),
		hpas:      make(map[string]*HPAInfo),
		pdbs:      make(map[string]*PDBInfo),
		onChange:  onChange,
	}
}

func (s *Store) Snapshot() []PodInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PodInfo, 0, len(s.pods))
	for _, p := range s.pods {
		result = append(result, *p)
	}
	return result
}

func (s *Store) UserPodsSnapshot() []PodInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PodInfo, 0)
	for _, p := range s.pods {
		// Filter out system namespaces
		if p.Namespace == "kube-system" || p.Namespace == "kube-public" || 
		   p.Namespace == "kube-node-lease" || p.Namespace == "local-path-storage" {
			continue
		}
		result = append(result, *p)
	}
	return result
}

func (s *Store) HPASnapshot() []HPAInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]HPAInfo, 0, len(s.hpas))
	for _, h := range s.hpas {
		result = append(result, *h)
	}
	return result
}

func (s *Store) PDBSnapshot() []PDBInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PDBInfo, 0, len(s.pdbs))
	for _, p := range s.pdbs {
		result = append(result, *p)
	}
	return result
}

func (s *Store) OnPodAdd(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Printf("OnPodAdd: unexpected type %T", obj)
		return
	}
	s.updatePod(pod)
}

func (s *Store) OnPodUpdate(oldObj, newObj interface{}) {
	pod, ok := newObj.(*v1.Pod)
	if !ok {
		log.Printf("OnPodUpdate: unexpected type %T", newObj)
		return
	}
	s.updatePod(pod)
}

func (s *Store) OnPodDelete(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Printf("OnPodDelete: unexpected type %T", obj)
		return
	}

	key := pod.Namespace + "/" + pod.Name
	s.mu.Lock()
	delete(s.pods, key)
	s.mu.Unlock()

	if s.onChange != nil {
		s.onChange()
	}
}

func (s *Store) OnNodeAdd(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		log.Printf("OnNodeAdd: unexpected type %T", obj)
		return
	}
	s.updateNode(node)
}

func (s *Store) OnNodeUpdate(oldObj, newObj interface{}) {
	node, ok := newObj.(*v1.Node)
	if !ok {
		log.Printf("OnNodeUpdate: unexpected type %T", newObj)
		return
	}
	s.updateNode(node)
}

func (s *Store) OnNodeDelete(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		log.Printf("OnNodeDelete: unexpected type %T", obj)
		return
	}

	s.mu.Lock()
	delete(s.nodeZones, node.Name)
	s.mu.Unlock()
}

func (s *Store) updatePod(pod *v1.Pod) {
	s.mu.Lock()
	key := pod.Namespace + "/" + pod.Name
	zone := s.nodeZones[pod.Spec.NodeName]

	info := &PodInfo{
		Key:       key,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Phase:     string(pod.Status.Phase),
		NodeName:  pod.Spec.NodeName,
		Zone:      zone,
		PodIP:     pod.Status.PodIP,
		Age:       pod.CreationTimestamp.Time,
		Deleting:  pod.DeletionTimestamp != nil,
	}

	info.Status = extractStatus(pod)
	info.Ready, info.RestartCount = extractContainerInfo(pod)

	// Track restart timing for transitional state
	if oldPod, exists := s.pods[key]; exists {
		if info.RestartCount > oldPod.RestartCount {
			info.LastRestart = time.Now()
		} else {
			info.LastRestart = oldPod.LastRestart
		}
	}

	s.pods[key] = info
	s.mu.Unlock()

	if s.onChange != nil {
		s.onChange()
	}
}

func (s *Store) updateNode(node *v1.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()

	zone := node.Labels["topology.kubernetes.io/zone"]
	s.nodeZones[node.Name] = zone
}

func extractStatus(pod *v1.Pod) string {
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}

	// Check for CrashLoopBackOff or other error states
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason == "CrashLoopBackOff" {
				return "CrashLoopBackOff"
			}
			if reason != "" {
				return reason
			}
		}
	}

	// Check if pod is restarting (not ready but was recently running)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.RestartCount > 0 && !cs.Ready {
			// Pod has restarted and is not ready yet
			if cs.State.Running != nil || cs.State.Waiting != nil {
				return "Restarting"
			}
		}
	}

	if pod.Status.Phase != "" {
		return string(pod.Status.Phase)
	}

	return "Unknown"
}

func extractContainerInfo(pod *v1.Pod) (ready bool, restartCount int32) {
	for _, cs := range pod.Status.ContainerStatuses {
		restartCount += cs.RestartCount
		if cs.Ready {
			ready = true
		}
	}
	if len(pod.Status.ContainerStatuses) > 0 {
		allReady := true
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				allReady = false
				break
			}
		}
		ready = allReady
	}
	return
}

func (s *Store) OnHPAAdd(obj interface{}) {
	hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		log.Printf("OnHPAAdd: unexpected type %T", obj)
		return
	}
	s.updateHPA(hpa)
}

func (s *Store) OnHPAUpdate(oldObj, newObj interface{}) {
	hpa, ok := newObj.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		log.Printf("OnHPAUpdate: unexpected type %T", newObj)
		return
	}
	s.updateHPA(hpa)
}

func (s *Store) OnHPADelete(obj interface{}) {
	hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		log.Printf("OnHPADelete: unexpected type %T", obj)
		return
	}

	key := hpa.Namespace + "/" + hpa.Name
	s.mu.Lock()
	delete(s.hpas, key)
	s.mu.Unlock()

	if s.onChange != nil {
		s.onChange()
	}
}

func (s *Store) updateHPA(hpa *autoscalingv2.HorizontalPodAutoscaler) {
	s.mu.Lock()
	key := hpa.Namespace + "/" + hpa.Name

	info := &HPAInfo{
		Name:            hpa.Name,
		Namespace:       hpa.Namespace,
		TargetRef:       hpa.Spec.ScaleTargetRef.Name,
		MinReplicas:     *hpa.Spec.MinReplicas,
		MaxReplicas:     hpa.Spec.MaxReplicas,
		CurrentReplicas: hpa.Status.CurrentReplicas,
		DesiredReplicas: hpa.Status.DesiredReplicas,
	}

	// Extract target CPU utilization from metrics
	for _, metric := range hpa.Spec.Metrics {
		if metric.Type == autoscalingv2.ResourceMetricSourceType && metric.Resource != nil {
			if metric.Resource.Name == "cpu" && metric.Resource.Target.AverageUtilization != nil {
				info.TargetCPUUtilization = *metric.Resource.Target.AverageUtilization
			}
		}
	}

	// Extract current CPU utilization from status
	for _, metric := range hpa.Status.CurrentMetrics {
		if metric.Type == autoscalingv2.ResourceMetricSourceType && metric.Resource != nil {
			if metric.Resource.Name == "cpu" && metric.Resource.Current.AverageUtilization != nil {
				info.CurrentCPUUtilization = *metric.Resource.Current.AverageUtilization
			}
		}
	}

	s.hpas[key] = info
	s.mu.Unlock()

	if s.onChange != nil {
		s.onChange()
	}
}

func (s *Store) OnPDBAdd(obj interface{}) {
	pdb, ok := obj.(*policyv1.PodDisruptionBudget)
	if !ok {
		log.Printf("OnPDBAdd: unexpected type %T", obj)
		return
	}
	s.updatePDB(pdb)
}

func (s *Store) OnPDBUpdate(oldObj, newObj interface{}) {
	pdb, ok := newObj.(*policyv1.PodDisruptionBudget)
	if !ok {
		log.Printf("OnPDBUpdate: unexpected type %T", newObj)
		return
	}
	s.updatePDB(pdb)
}

func (s *Store) OnPDBDelete(obj interface{}) {
	pdb, ok := obj.(*policyv1.PodDisruptionBudget)
	if !ok {
		log.Printf("OnPDBDelete: unexpected type %T", obj)
		return
	}

	key := pdb.Namespace + "/" + pdb.Name
	s.mu.Lock()
	delete(s.pdbs, key)
	s.mu.Unlock()

	if s.onChange != nil {
		s.onChange()
	}
}

func (s *Store) updatePDB(pdb *policyv1.PodDisruptionBudget) {
	s.mu.Lock()
	key := pdb.Namespace + "/" + pdb.Name

	info := &PDBInfo{
		Name:               pdb.Name,
		Namespace:          pdb.Namespace,
		CurrentHealthy:     pdb.Status.CurrentHealthy,
		DesiredHealthy:     pdb.Status.DesiredHealthy,
		DisruptionsAllowed: pdb.Status.DisruptionsAllowed,
	}

	// Extract minAvailable or maxUnavailable from spec
	if pdb.Spec.MinAvailable != nil {
		info.MinAvailable = pdb.Spec.MinAvailable.IntVal
	}
	if pdb.Spec.MaxUnavailable != nil {
		info.MaxUnavailable = pdb.Spec.MaxUnavailable.IntVal
	}

	s.pdbs[key] = info
	s.mu.Unlock()

	if s.onChange != nil {
		s.onChange()
	}
}

// UpdatePodCPU updates CPU usage for a specific pod.
func (s *Store) UpdatePodCPU(podName, namespace, cpuUsage string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := namespace + "/" + podName
	if pod, exists := s.pods[key]; exists {
		pod.CPUUsage = cpuUsage
	}
}