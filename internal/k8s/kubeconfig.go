package k8s

import (
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// RESTConfigFromKubeconfigYAML parses a kubeconfig document into a client rest.Config.
func RESTConfigFromKubeconfigYAML(yaml string) (*rest.Config, error) {
	raw, err := clientcmd.Load([]byte(yaml))
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}
	return clientcmd.NewDefaultClientConfig(*raw, &clientcmd.ConfigOverrides{}).ClientConfig()
}
