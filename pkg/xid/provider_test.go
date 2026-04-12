package xid

import (
	"strings"
	"testing"
)

func TestInitKubernetesUsesPodOrdinal(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	if err := Init(Config{
		Provider:          ProviderKubernetes,
		ServiceBaseNodeID: 128,
		MaxReplicas:       8,
		PodName:           "program-rpc-3",
	}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	first := New()
	second := New()

	if first <= 0 {
		t.Fatalf("expected first id > 0, got %d", first)
	}
	if second <= first {
		t.Fatalf("expected increasing ids, first=%d second=%d", first, second)
	}
}

func TestInitKubernetesRejectsInvalidPodName(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	err := Init(Config{
		Provider:          ProviderKubernetes,
		ServiceBaseNodeID: 128,
		MaxReplicas:       8,
		PodName:           "program-rpc-a",
	})
	if err == nil {
		t.Fatal("expected Init to fail for invalid pod name")
	}
	if !strings.Contains(err.Error(), "pod name") {
		t.Fatalf("expected pod name error, got %v", err)
	}
}

func TestInitKubernetesRejectsOrdinalOutsideMaxReplicas(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	err := Init(Config{
		Provider:          ProviderKubernetes,
		ServiceBaseNodeID: 128,
		MaxReplicas:       3,
		PodName:           "program-rpc-3",
	})
	if err == nil {
		t.Fatal("expected Init to fail when ordinal is outside max replicas")
	}
	if !strings.Contains(err.Error(), "max replicas") {
		t.Fatalf("expected max replicas error, got %v", err)
	}
}

func TestInitRejectsNodeIDOverflow(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	err := Init(Config{
		Provider: ProviderStatic,
		NodeID:   maxNodeID + 1,
	})
	if err == nil {
		t.Fatal("expected Init to fail for overflowing node id")
	}
	if !strings.Contains(err.Error(), "node id") {
		t.Fatalf("expected node id error, got %v", err)
	}
}

func TestResolveNodeIDRejectsUnsupportedProvider(t *testing.T) {
	nodeID, err := resolveNodeID(Config{
		Provider: Provider("unknown"),
	})
	if err == nil {
		t.Fatal("expected resolveNodeID to reject unsupported provider")
	}
	if nodeID != 0 {
		t.Fatalf("expected zero node id on error, got %d", nodeID)
	}
}
