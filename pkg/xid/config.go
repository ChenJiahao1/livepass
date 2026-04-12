package xid

import "fmt"

const maxNodeID int64 = 1023

type Provider string

const (
	ProviderStatic     Provider = "static"
	ProviderKubernetes Provider = "kubernetes"
)

type Config struct {
	Provider          Provider
	NodeID            int64
	ServiceBaseNodeID int64
	MaxReplicas       int64
	PodName           string
}

func resolveNodeID(cfg Config) (int64, error) {
	switch cfg.Provider {
	case ProviderStatic:
		return resolveStaticNodeID(cfg)
	case ProviderKubernetes:
		return resolveKubernetesNodeID(cfg)
	default:
		return 0, fmt.Errorf("unsupported xid provider %q", cfg.Provider)
	}
}
