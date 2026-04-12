package xid

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func resolveKubernetesNodeID(cfg Config) (int64, error) {
	if cfg.ServiceBaseNodeID < 0 || cfg.ServiceBaseNodeID > maxNodeID {
		return 0, fmt.Errorf("xid service base node id %d out of range [0,%d]", cfg.ServiceBaseNodeID, maxNodeID)
	}
	if cfg.MaxReplicas <= 0 {
		return 0, fmt.Errorf("xid max replicas must be > 0")
	}

	podName := cfg.PodName
	if podName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return 0, fmt.Errorf("resolve pod name from hostname: %w", err)
		}
		podName = hostname
	}

	ordinal, err := parseOrdinal(podName)
	if err != nil {
		return 0, err
	}
	if ordinal >= cfg.MaxReplicas {
		return 0, fmt.Errorf("xid pod ordinal %d outside max replicas %d", ordinal, cfg.MaxReplicas)
	}

	nodeID := cfg.ServiceBaseNodeID + ordinal
	if nodeID > maxNodeID {
		return 0, fmt.Errorf("xid node id %d out of range [0,%d]", nodeID, maxNodeID)
	}

	return nodeID, nil
}

func parseOrdinal(name string) (int64, error) {
	idx := strings.LastIndex(name, "-")
	if idx <= 0 || idx == len(name)-1 {
		return 0, fmt.Errorf("invalid kubernetes pod name %q: expected <name>-<ordinal>", name)
	}

	ordinal, err := strconv.ParseInt(name[idx+1:], 10, 64)
	if err != nil || ordinal < 0 {
		return 0, fmt.Errorf("invalid kubernetes pod name %q: expected <name>-<ordinal>", name)
	}

	return ordinal, nil
}
