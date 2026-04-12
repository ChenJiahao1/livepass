package xid

import "fmt"

func resolveStaticNodeID(cfg Config) (int64, error) {
	if cfg.NodeID < 0 || cfg.NodeID > maxNodeID {
		return 0, fmt.Errorf("xid node id %d out of range [0,%d]", cfg.NodeID, maxNodeID)
	}

	return cfg.NodeID, nil
}
