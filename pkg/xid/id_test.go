package xid

import (
	"sort"
	"sync"
	"testing"
)

func TestNewPanicsBeforeInit(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	_ = Close()

	defer func() {
		if recover() == nil {
			t.Fatal("expected New to panic before initialization")
		}
	}()

	_ = New()
}

func TestInitStaticGeneratesIncreasingIDs(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	if err := Init(Config{
		Provider: ProviderStatic,
		NodeID:   7,
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

func TestCloseStopsGenerator(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	if err := Init(Config{
		Provider: ProviderStatic,
		NodeID:   9,
	}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	if err := Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected New to panic after Close")
		}
	}()

	_ = New()
}

func TestNewGeneratesUniqueIDsConcurrently(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	if err := Init(Config{
		Provider: ProviderStatic,
		NodeID:   11,
	}); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	const total = 128

	results := make(chan int64, total)
	var wg sync.WaitGroup
	wg.Add(total)

	for i := 0; i < total; i++ {
		go func() {
			defer wg.Done()
			results <- New()
		}()
	}

	wg.Wait()
	close(results)

	ids := make([]int64, 0, total)
	seen := make(map[int64]struct{}, total)
	for id := range results {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id generated: %d", id)
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			t.Fatalf("expected unique ids, duplicate=%d", ids[i])
		}
	}
}
