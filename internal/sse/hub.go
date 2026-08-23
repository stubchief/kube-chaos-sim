package sse

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	"kube-chaos-sim/internal/k8s"
)

type Hub struct {
	mu       sync.RWMutex
	clients  map[chan string]struct{}
	snapshot func() []k8s.PodInfo
	hpaSnapshot func() []k8s.HPAInfo
	pdbSnapshot func() []k8s.PDBInfo

	debounceMu    sync.Mutex
	debounceTimer *time.Timer
}

func NewHub(snapshot func() []k8s.PodInfo, hpaSnapshot func() []k8s.HPAInfo, pdbSnapshot func() []k8s.PDBInfo) *Hub {
	return &Hub{
		clients:  make(map[chan string]struct{}),
		snapshot: snapshot,
		hpaSnapshot: hpaSnapshot,
		pdbSnapshot: pdbSnapshot,
	}
}

func (h *Hub) Subscribe() chan string {
	ch := make(chan string, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan string) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *Hub) Broadcast(html string) {
	h.debounceMu.Lock()
	defer h.debounceMu.Unlock()

	if h.debounceTimer != nil {
		h.debounceTimer.Stop()
	}

	h.debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
		h.mu.RLock()
		defer h.mu.RUnlock()

		for ch := range h.clients {
			select {
			case ch <- html:
			default:
				log.Printf("Dropping SSE message for slow client")
			}
		}
	})
}

func (h *Hub) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("SSE client connected from %s", r.RemoteAddr)
		sse := datastar.NewSSE(w, r)

		ch := h.Subscribe()
		defer h.Unsubscribe(ch)

		pods := h.snapshot()
		hpas := h.hpaSnapshot()
		pdbs := h.pdbSnapshot()
		log.Printf("Snapshot: %d pods, %d HPAs, %d PDBs", len(pods), len(hpas), len(pdbs))
		initialHTML := RenderPodGrid(pods) + RenderHPAPanel(hpas) + RenderPDBPanel(pdbs)
		
		if err := sse.PatchElements(initialHTML); err != nil {
			log.Printf("Failed to send initial snapshot: %v", err)
			return
		}

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				log.Printf("SSE client disconnected")
				return
			case html, ok := <-ch:
				if !ok {
					return
				}
				if err := sse.PatchElements(html); err != nil {
					log.Printf("Failed to send SSE update: %v", err)
					return
				}
			}
		}
	}
}