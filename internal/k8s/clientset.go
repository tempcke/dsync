package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func kubeCS(kcp string) (kubernetes.Interface, error) {
	cfg, err := restConfig(kcp)
	if err != nil {
		return nil, err
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cs, nil
}
func restConfig(kcp string) (*rest.Config, error) {
	if kcp != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kcp)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	return rest.InClusterConfig()
}
