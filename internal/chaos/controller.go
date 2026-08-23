package chaos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"kube-chaos-sim/internal/k8s"
)

// Controller executes chaos actions against the Kubernetes cluster.
type Controller struct {
	clientset kubernetes.Interface
	store     *k8s.Store
}

// NewController creates a new Chaos Controller.
func NewController(clientset kubernetes.Interface, store *k8s.Store) *Controller {
	return &Controller{
		clientset: clientset,
		store:     store,
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
	cmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "--",
		"wget", "-qO-", fmt.Sprintf("--timeout=%d", seconds+5), fmt.Sprintf("http://localhost:9898/delay/%d", seconds))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to call podinfo delay endpoint via kubectl exec: %w, output: %s", err, string(output))
	}
	
	log.Printf("Successfully injected %ds latency to pod %s/%s", seconds, namespace, podName)
	return nil
}

// MemorySpike triggers a memory spike by calling podinfo's /panic endpoint,
// which crashes the process (exit 255) and demonstrates CrashLoopBackOff behavior.
func (c *Controller) MemorySpike(ctx context.Context, podName, namespace string, megabytes int) error {
	log.Printf("Triggering memory spike in pod %s/%s (%dMB requested, using /panic)", namespace, podName, megabytes)
	
	// Use kubectl exec to call the panic endpoint from inside the cluster
	cmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "--",
		"wget", "-qO-", "--timeout=5", "http://localhost:9898/panic")
	
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
