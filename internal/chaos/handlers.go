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
