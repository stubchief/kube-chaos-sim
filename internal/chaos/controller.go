package chaos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"kube-chaos-sim/internal/k8s"
	"kube-chaos-sim/internal/metrics"
)

// Controller executes chaos actions against the Kubernetes cluster.
type Controller struct {
	clientset  kubernetes.Interface
	store      *k8s.Store
	metricsGen *metrics.Generator
}

// NewController creates a new Chaos Controller.
func NewController(clientset kubernetes.Interface, store *k8s.Store, metricsGen *metrics.Generator) *Controller {
	return &Controller{
		clientset:  clientset,
		store:      store,
		metricsGen: metricsGen,
	}
}

// KillPod deletes a pod by name and namespace.
func (c *Controller) KillPod(ctx context.Context, podName, namespace string) error {
	log.Printf("Killing pod %s/%s", namespace, podName)
	
	err := c.clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod %s/%s: %w", namespace, podName, err)
	}
	
	log.Printf("Successfully deleted pod %s/%s", namespace, podName)
	return nil
}

// InjectLatency sends a request to podinfo's /delay endpoint to inject latency.
func (c *Controller) InjectLatency(ctx context.Context, podName, namespace string, seconds int) error {
	log.Printf("Injecting %ds latency to pod %s/%s", seconds, namespace, podName)
	
	// Use kubectl exec to call the delay endpoint from inside the cluster
	// Use curl instead of wget (podinfo has curl, wget is BusyBox version with limited options)
	cmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "--",
		"curl", "-s", "--max-time", fmt.Sprintf("%d", seconds+5), fmt.Sprintf("http://localhost:9898/delay/%d", seconds))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to call podinfo delay endpoint via kubectl exec: %w, output: %s", err, string(output))
	}
	
	// Register latency injection in metrics (duration = seconds, matching the button label)
	c.metricsGen.InjectLatency(float64(seconds)*1000, time.Duration(seconds)*time.Second)
	
	log.Printf("Successfully injected %ds latency to pod %s/%s", seconds, namespace, podName)
	return nil
}

// MemorySpike triggers a memory spike by calling podinfo's /panic endpoint,
// which crashes the process (exit 255) and demonstrates CrashLoopBackOff behavior.
func (c *Controller) MemorySpike(ctx context.Context, podName, namespace string, megabytes int) error {
	log.Printf("Triggering memory spike in pod %s/%s (%dMB requested, using /panic)", namespace, podName, megabytes)
	
	// Use kubectl exec to call the panic endpoint from inside the cluster
	// Use curl instead of wget (podinfo has curl, wget is BusyBox version with limited options)
	cmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "--",
		"curl", "-s", "--max-time", "5", "http://localhost:9898/panic")
	
	output, err := cmd.CombinedOutput()
	// /panic endpoint crashes the pod, so connection error or timeout is expected
	if err != nil {
		log.Printf("Pod %s/%s crashed as expected (memory spike simulated), output: %s", namespace, podName, string(output))
		return nil
	}
	
	log.Printf("Memory spike triggered in pod %s/%s, output: %s", namespace, podName, string(output))
	return nil
}

// SetHPA patches an HPA to update min/max replicas and target CPU utilization.
func (c *Controller) SetHPA(ctx context.Context, hpaName, namespace string, minReplicas, maxReplicas, targetCPU int32) error {
	log.Printf("Updating HPA %s/%s: min=%d, max=%d, targetCPU=%d%%", namespace, hpaName, minReplicas, maxReplicas, targetCPU)

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"minReplicas": minReplicas,
			"maxReplicas": maxReplicas,
			"metrics": []map[string]interface{}{
				{
					"type": "Resource",
					"resource": map[string]interface{}{
						"name": "cpu",
						"target": map[string]interface{}{
							"type":               "Utilization",
							"averageUtilization": targetCPU,
						},
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal HPA patch: %w", err)
	}

	_, err = c.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Patch(
		ctx, hpaName, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch HPA %s/%s: %w", namespace, hpaName, err)
	}

	log.Printf("Successfully updated HPA %s/%s", namespace, hpaName)
	return nil
}

// SetPDB patches a PDB to update minAvailable.
func (c *Controller) SetPDB(ctx context.Context, pdbName, namespace string, minAvailable int32) error {
	log.Printf("Updating PDB %s/%s: minAvailable=%d", namespace, pdbName, minAvailable)

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"minAvailable": minAvailable,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal PDB patch: %w", err)
	}

	_, err = c.clientset.PolicyV1().PodDisruptionBudgets(namespace).Patch(
		ctx, pdbName, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch PDB %s/%s: %w", namespace, pdbName, err)
	}

	log.Printf("Successfully updated PDB %s/%s", namespace, pdbName)
	return nil
}

// SetSLO updates the SLO target percentage in the metrics generator.
func (c *Controller) SetSLO(slo float64) {
	log.Printf("Updating SLO to %.1f%%", slo)
	c.metricsGen.SetSLO(slo)
}

// RollingUpdate triggers a rolling update of the deployment.
func (c *Controller) RollingUpdate(ctx context.Context, deploymentName, namespace string) error {
	log.Printf("Triggering rolling update for deployment %s/%s", namespace, deploymentName)

	// Use kubectl rollout restart
	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "restart", "deployment/"+deploymentName, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl rollout restart failed: %w, output: %s", err, string(output))
	}

	log.Printf("Rolling update triggered for %s/%s: %s", namespace, deploymentName, string(output))
	return nil
}

// CPUStressAsync triggers CPU stress on a pod asynchronously (doesn't block).
func (c *Controller) CPUStressAsync(podName, namespace string, durationSec int) {
	go func() {
		// Use background context since we don't want to cancel when HTTP request ends
		ctx := context.Background()
		if err := c.CPUStress(ctx, podName, namespace, durationSec); err != nil {
			log.Printf("CPU stress failed for %s/%s: %v", namespace, podName, err)
		}
	}()
}

// CPUStress triggers CPU stress on a pod via shell command.
func (c *Controller) CPUStress(ctx context.Context, podName, namespace string, durationSec int) error {
	log.Printf("Triggering CPU stress on pod %s/%s for %d seconds", namespace, podName, durationSec)

	// podinfo doesn't have a /stress endpoint, so we use shell commands inside the pod.
	// Spawn busy loops in background, sleep for duration, then kill all of them.
	// Works with any container that has sh (podinfo uses alpine-based image).
	script := fmt.Sprintf(
		"for i in $(seq 1 4); do yes > /dev/null & done; sleep %d; pkill -P $$ yes || true",
		durationSec,
	)

	cmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "--",
		"sh", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl exec failed: %w, output: %s", err, string(output))
	}

	log.Printf("CPU stress triggered on %s/%s: %s", namespace, podName, string(output))
	return nil
}

// CPUStressAllAsync triggers CPU stress on all pods of a deployment asynchronously.
func (c *Controller) CPUStressAllAsync(deploymentName, namespace string, durationSec int) {
	go func() {
		ctx := context.Background()
		if err := c.CPUStressAll(ctx, deploymentName, namespace, durationSec); err != nil {
			log.Printf("CPU stress all failed for %s/%s: %v", namespace, deploymentName, err)
		}
	}()
}

// CPUStressAll triggers CPU stress on all Running pods of a deployment.
func (c *Controller) CPUStressAll(ctx context.Context, deploymentName, namespace string, durationSec int) error {
	log.Printf("Triggering CPU stress on all pods of %s/%s for %d seconds", namespace, deploymentName, durationSec)

	// Get all pods from the store
	pods := c.store.UserPodsSnapshot()

	// Filter pods by namespace and status
	var targetPods []k8s.PodInfo
	for _, pod := range pods {
		if pod.Namespace == namespace && pod.Status == "Running" {
			targetPods = append(targetPods, pod)
		}
	}

	if len(targetPods) == 0 {
		return fmt.Errorf("no Running pods found for deployment %s in namespace %s", deploymentName, namespace)
	}

	log.Printf("Found %d Running pods for CPU stress", len(targetPods))

	// Trigger CPU stress on each pod
	for _, pod := range targetPods {
		c.CPUStressAsync(pod.Name, pod.Namespace, durationSec)
	}

	return nil
}
