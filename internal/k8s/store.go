package k8s

import (
	"log"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
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
}

type Store struct {
	mu        sync.RWMutex
	pods      map[string]*PodInfo
	nodeZones map[string]string
	onChange  func()
}

func NewStore(onChange func()) *Store {
	return &Store{
		pods:      make(map[string]*PodInfo),
		nodeZones: make(map[string]string),
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

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason != "" {
				return reason
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