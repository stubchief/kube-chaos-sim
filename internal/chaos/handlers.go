package chaos

import (
	"log"
	"net/http"
	"strconv"
)
// HandleKillPod handles HTTP requests to kill a pod.
func (c *Controller) HandleKillPod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	podName := r.URL.Query().Get("podName")
	namespace := r.URL.Query().Get("namespace")

	if podName == "" || namespace == "" {
		http.Error(w, "podName and namespace are required", http.StatusBadRequest)
		return
	}

	if err := c.KillPod(r.Context(), podName, namespace); err != nil {
		log.Printf("Failed to kill pod: %v", err)
		http.Error(w, "Failed to kill pod: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Pod deletion initiated: %s/%s", namespace, podName)
	// SSE connection will receive updates from informers automatically
	w.WriteHeader(http.StatusAccepted)
}

// HandleInjectLatency handles HTTP requests to inject latency.
func (c *Controller) HandleInjectLatency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	podName := r.URL.Query().Get("podName")
	namespace := r.URL.Query().Get("namespace")
	secondsStr := r.URL.Query().Get("seconds")

	if podName == "" || namespace == "" {
		http.Error(w, "podName and namespace are required", http.StatusBadRequest)
		return
	}

	seconds := 5 // default
	if secondsStr != "" {
		if s, err := strconv.Atoi(secondsStr); err == nil {
			seconds = s
		}
	}

	if seconds <= 0 || seconds > 60 {
		http.Error(w, "seconds must be between 1 and 60", http.StatusBadRequest)
		return
	}

	if err := c.InjectLatency(r.Context(), podName, namespace, seconds); err != nil {
		log.Printf("Failed to inject latency: %v", err)
		http.Error(w, "Failed to inject latency: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Latency injection completed: %s/%s for %ds", namespace, podName, seconds)
	// SSE connection will receive updates from informers automatically
	w.WriteHeader(http.StatusAccepted)
}

// HandleMemorySpike handles HTTP requests to trigger a memory spike.
func (c *Controller) HandleMemorySpike(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	podName := r.URL.Query().Get("podName")
	namespace := r.URL.Query().Get("namespace")
	megabytesStr := r.URL.Query().Get("megabytes")

	if podName == "" || namespace == "" {
		http.Error(w, "podName and namespace are required", http.StatusBadRequest)
		return
	}

	megabytes := 100 // default
	if megabytesStr != "" {
		if m, err := strconv.Atoi(megabytesStr); err == nil {
			megabytes = m
		}
	}

	if megabytes <= 0 {
		megabytes = 100
	}

	if err := c.MemorySpike(r.Context(), podName, namespace, megabytes); err != nil {
		log.Printf("Failed to trigger memory spike: %v", err)
		http.Error(w, "Failed to trigger memory spike: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Memory spike triggered: %s/%s for %dMB", namespace, podName, megabytes)
	// SSE connection will receive updates from informers automatically
	w.WriteHeader(http.StatusAccepted)
}

// HandleSetHPA handles HTTP requests to update HPA settings.
func (c *Controller) HandleSetHPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hpaName := r.URL.Query().Get("hpaName")
	namespace := r.URL.Query().Get("namespace")
	minReplicasStr := r.URL.Query().Get("minReplicas")
	maxReplicasStr := r.URL.Query().Get("maxReplicas")
	targetCPUStr := r.URL.Query().Get("targetCPU")

	if hpaName == "" || namespace == "" {
		http.Error(w, "hpaName and namespace are required", http.StatusBadRequest)
		return
	}

	minReplicas := int32(1)
	if minReplicasStr != "" {
		if m, err := strconv.ParseInt(minReplicasStr, 10, 32); err == nil {
			minReplicas = int32(m)
		}
	}

	maxReplicas := int32(10)
	if maxReplicasStr != "" {
		if m, err := strconv.ParseInt(maxReplicasStr, 10, 32); err == nil {
			maxReplicas = int32(m)
		}
	}

	targetCPU := int32(50)
	if targetCPUStr != "" {
		if t, err := strconv.ParseInt(targetCPUStr, 10, 32); err == nil {
			targetCPU = int32(t)
		}
	}

	if minReplicas < 1 {
		minReplicas = 1
	}
	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}
	if targetCPU < 1 || targetCPU > 100 {
		targetCPU = 50
	}

	if err := c.SetHPA(r.Context(), hpaName, namespace, minReplicas, maxReplicas, targetCPU); err != nil {
		log.Printf("Failed to update HPA: %v", err)
		http.Error(w, "Failed to update HPA: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("HPA updated: %s/%s min=%d max=%d targetCPU=%d%%", namespace, hpaName, minReplicas, maxReplicas, targetCPU)
	w.WriteHeader(http.StatusAccepted)
}

// HandleSetPDB handles HTTP requests to update PDB settings.
func (c *Controller) HandleSetPDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pdbName := r.URL.Query().Get("pdbName")
	namespace := r.URL.Query().Get("namespace")
	minAvailableStr := r.URL.Query().Get("minAvailable")

	if pdbName == "" || namespace == "" {
		http.Error(w, "pdbName and namespace are required", http.StatusBadRequest)
		return
	}

	minAvailable := int32(1)
	if minAvailableStr != "" {
		if m, err := strconv.ParseInt(minAvailableStr, 10, 32); err == nil {
			minAvailable = int32(m)
		}
	}

	if minAvailable < 0 {
		minAvailable = 0
	}

	if err := c.SetPDB(r.Context(), pdbName, namespace, minAvailable); err != nil {
		log.Printf("Failed to update PDB: %v", err)
		http.Error(w, "Failed to update PDB: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("PDB updated: %s/%s minAvailable=%d", namespace, pdbName, minAvailable)
	w.WriteHeader(http.StatusAccepted)
}
