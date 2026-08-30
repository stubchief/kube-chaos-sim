package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"kube-chaos-sim/internal/chaos"
	"kube-chaos-sim/internal/config"
	"kube-chaos-sim/internal/k8s"
	"kube-chaos-sim/internal/metrics"
	"kube-chaos-sim/internal/sse"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig (empty for in-cluster)")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	k8sClient, err := k8s.NewClient(*kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create k8s client: %v", err)
	}

	// Create metrics generator (120 points * 1s = 2 minutes of history)
	// SLO 80% means we tolerate 20% errors
	metricsGen := metrics.NewGenerator(config.MetricsHistorySize, config.MetricsTickInterval, 80)

	var store *k8s.Store
	hub := sse.NewHub(func() []k8s.PodInfo {
		return store.UserPodsSnapshot()
	}, func() []k8s.HPAInfo {
		return store.HPASnapshot()
	}, func() []k8s.PDBInfo {
		return store.PDBSnapshot()
	}, metricsGen)

	store = k8s.NewStore(func() {
		pods := store.UserPodsSnapshot()
		hpas := store.HPASnapshot()
		pdbs := store.PDBSnapshot()
		
		// Compute and record metrics
		m := metricsGen.Compute(pods)
		metricsGen.Record(m)
		
		// Render inner content of panels to avoid redrawing details tag
		html := sse.RenderPodGrid(pods) + 
			sse.RenderHPAContent(hpas) + 
			sse.RenderPDBContent(pdbs) +
			sse.RenderMetricsPanel(metricsGen.Snapshot(), metricsGen)
		hub.Broadcast(html)
	})

	podInformer := k8sClient.PodInformer()
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    store.OnPodAdd,
		UpdateFunc: store.OnPodUpdate,
		DeleteFunc: store.OnPodDelete,
	})

	nodeInformer := k8sClient.NodeInformer()
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    store.OnNodeAdd,
		UpdateFunc: store.OnNodeUpdate,
		DeleteFunc: store.OnNodeDelete,
	})

	hpaInformer := k8sClient.HPAInformer()
	hpaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    store.OnHPAAdd,
		UpdateFunc: store.OnHPAUpdate,
		DeleteFunc: store.OnHPADelete,
	})

	pdbInformer := k8sClient.PDBInformer()
	pdbInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    store.OnPDBAdd,
		UpdateFunc: store.OnPDBUpdate,
		DeleteFunc: store.OnPDBDelete,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal")
		cancel()
	}()

	k8sClient.Run(ctx)

	// Start CPU metrics collector
	go func() {
		ticker := time.NewTicker(config.CPUMetricsInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				podMetrics, err := k8sClient.MetricsClient().MetricsV1beta1().PodMetricses("default").List(ctx, metav1.ListOptions{})
				if err != nil {
					log.Printf("Failed to get pod metrics: %v", err)
					continue
				}
				for _, pm := range podMetrics.Items {
					var totalCPU int64
					for _, c := range pm.Containers {
						totalCPU += c.Usage.Cpu().MilliValue()
					}
					store.UpdatePodCPU(pm.Name, pm.Namespace, fmt.Sprintf("%dm", totalCPU))
				}
			}
		}
	}()

	// Start metrics ticker to update metrics even without pod changes
	go func() {
		ticker := time.NewTicker(config.MetricsTickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pods := store.UserPodsSnapshot()
				hpas := store.HPASnapshot()
				pdbs := store.PDBSnapshot()
				
				m := metricsGen.Compute(pods)
				metricsGen.Record(m)
				
				// Render inner content of panels for real-time updates without panel flickering
				html := sse.RenderPodGrid(pods) + 
					sse.RenderHPAContent(hpas) + 
					sse.RenderPDBContent(pdbs) +
					sse.RenderMetricsPanel(metricsGen.Snapshot(), metricsGen)
				hub.Broadcast(html)
			}
		}
	}()

	// Auto-create HPA and PDB for podinfo if they don't exist
	if err := ensureHPA(ctx, k8sClient.Clientset()); err != nil {
		log.Printf("Warning: failed to ensure HPA: %v", err)
	}
	if err := ensurePDB(ctx, k8sClient.Clientset()); err != nil {
		log.Printf("Warning: failed to ensure PDB: %v", err)
	}

	chaosController := chaos.NewController(k8sClient.Clientset(), store, metricsGen)

	http.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("web/static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
	http.HandleFunc("/events", hub.Handler())
	http.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Snapshot request from %s", r.RemoteAddr)
		sseWriter := datastar.NewSSE(w, r)
		pods := store.UserPodsSnapshot()
		hpas := store.HPASnapshot()
		pdbs := store.PDBSnapshot()
		
		// Compute and record metrics
		m := metricsGen.Compute(pods)
		metricsGen.Record(m)
		
		html := sse.RenderPodGrid(pods) + 
			sse.RenderHPAContent(hpas) + 
			sse.RenderPDBContent(pdbs) +
			sse.RenderMetricsPanel(metricsGen.Snapshot(), metricsGen)
		sseWriter.PatchElements(html)
	})
	http.HandleFunc("/api/chaos/kill-pod", chaosController.HandleKillPod)
	http.HandleFunc("/api/chaos/inject-latency", chaosController.HandleInjectLatency)
	http.HandleFunc("/api/chaos/memory-spike", chaosController.HandleMemorySpike)
	http.HandleFunc("/api/chaos/set-hpa", chaosController.HandleSetHPA)
	http.HandleFunc("/api/chaos/set-pdb", chaosController.HandleSetPDB)
	http.HandleFunc("/api/chaos/set-slo", chaosController.HandleSetSLO)
	http.HandleFunc("/api/chaos/rolling-update", chaosController.HandleRollingUpdate)
	http.HandleFunc("/api/chaos/cpu-stress", chaosController.HandleCPUStress)
	http.HandleFunc("/api/chaos/cpu-stress-all", chaosController.HandleCPUStressAll)

	log.Printf("Listening on %s", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func ensureHPA(ctx context.Context, clientset kubernetes.Interface) error {
	hpaName := "podinfo-hpa"
	namespace := "default"

	// Check if HPA already exists
	_, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, hpaName, metav1.GetOptions{})
	if err == nil {
		log.Printf("HPA %s/%s already exists", namespace, hpaName)
		return nil
	}

	// Create HPA
	minReplicas := int32(config.DefaultHPAMinReplicas)
	maxReplicas := int32(config.DefaultHPAMaxReplicas)
	targetCPU := int32(config.DefaultHPATargetCPU)

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpaName,
			Namespace: namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "podinfo",
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &targetCPU,
						},
					},
				},
			},
		},
	}

	_, err = clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Create(ctx, hpa, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create HPA: %w", err)
	}

	log.Printf("Created HPA %s/%s (min=%d, max=%d, targetCPU=%d%%)", namespace, hpaName, minReplicas, maxReplicas, targetCPU)
	return nil
}

func ensurePDB(ctx context.Context, clientset kubernetes.Interface) error {
	pdbName := "podinfo-pdb"
	namespace := "default"

	// Check if PDB already exists
	_, err := clientset.PolicyV1().PodDisruptionBudgets(namespace).Get(ctx, pdbName, metav1.GetOptions{})
	if err == nil {
		log.Printf("PDB %s/%s already exists", namespace, pdbName)
		return nil
	}

	// Create PDB
	minAvailable := intstr.FromInt32(int32(config.DefaultPDBMinAvailable))

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pdbName,
			Namespace: namespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "podinfo",
				},
			},
		},
	}

	_, err = clientset.PolicyV1().PodDisruptionBudgets(namespace).Create(ctx, pdb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PDB: %w", err)
	}

	log.Printf("Created PDB %s/%s (minAvailable=%d)", namespace, pdbName, minAvailable.IntVal)
	return nil
}