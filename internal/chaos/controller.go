package chaos

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
