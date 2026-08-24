package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	autoscalinginformers "k8s.io/client-go/informers/autoscaling/v2"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	policyinformers "k8s.io/client-go/informers/policy/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Client struct {
	clientset     kubernetes.Interface
	metricsClient metricsv.Interface
	factory       informers.SharedInformerFactory
	podInformer   coreinformers.PodInformer
	nodeInformer  coreinformers.NodeInformer
	hpaInformer   autoscalinginformers.HorizontalPodAutoscalerInformer
	pdbInformer   policyinformers.PodDisruptionBudgetInformer
}

func NewClient(kubeconfig string) (*Client, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
	} else {
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	return &Client{
		clientset:     clientset,
		metricsClient: metricsClient,
		factory:       factory,
		podInformer:   factory.Core().V1().Pods(),
		nodeInformer:  factory.Core().V1().Nodes(),
		hpaInformer:   factory.Autoscaling().V2().HorizontalPodAutoscalers(),
		pdbInformer:   factory.Policy().V1().PodDisruptionBudgets(),
	}, nil
}

func (c *Client) PodInformer() coreinformers.PodInformer {
	return c.podInformer
}

func (c *Client) NodeInformer() coreinformers.NodeInformer {
	return c.nodeInformer
}

func (c *Client) HPAInformer() autoscalinginformers.HorizontalPodAutoscalerInformer {
	return c.hpaInformer
}

func (c *Client) PDBInformer() policyinformers.PodDisruptionBudgetInformer {
	return c.pdbInformer
}

func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

func (c *Client) MetricsClient() metricsv.Interface {
	return c.metricsClient
}

func (c *Client) Run(ctx context.Context) {
	c.factory.Start(ctx.Done())
	log.Println("Waiting for informer caches to sync...")
	c.factory.WaitForCacheSync(ctx.Done())
	log.Println("Informer caches synced")
}