package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset    kubernetes.Interface
	factory      informers.SharedInformerFactory
	podInformer  coreinformers.PodInformer
	nodeInformer coreinformers.NodeInformer
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

	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	return &Client{
		clientset:    clientset,
		factory:      factory,
		podInformer:  factory.Core().V1().Pods(),
		nodeInformer: factory.Core().V1().Nodes(),
	}, nil
}

func (c *Client) PodInformer() coreinformers.PodInformer {
	return c.podInformer
}

func (c *Client) NodeInformer() coreinformers.NodeInformer {
	return c.nodeInformer
}

func (c *Client) Run(ctx context.Context) {
	c.factory.Start(ctx.Done())
	log.Println("Waiting for informer caches to sync...")
	c.factory.WaitForCacheSync(ctx.Done())
	log.Println("Informer caches synced")
}