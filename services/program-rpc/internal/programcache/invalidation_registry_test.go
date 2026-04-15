package programcache

import "testing"

func TestInvalidationRegistryDispatchesDetailAndCategoryEntries(t *testing.T) {
	detail := &stubDetailInvalidator{}
	category := &stubCategoryInvalidator{}
	registry := newInvalidationRegistry(detail, category)

	msg := InvalidationMessage{
		Entries: []InvalidationEntry{
			{
				Cache:     cacheProgramDetailView,
				ProgramID: 20001,
			},
			{
				Cache: cacheCategorySnapshot,
			},
		},
	}

	if err := registry.Dispatch(msg); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if len(detail.calls) != 1 || detail.calls[0] != 20001 {
		t.Fatalf("expected detail invalidation for 20001, got %+v", detail.calls)
	}
	if category.calls != 1 {
		t.Fatalf("expected category invalidation once, got %d", category.calls)
	}
}

func TestInvalidationRegistryIgnoresDuplicateEntries(t *testing.T) {
	detail := &stubDetailInvalidator{}
	category := &stubCategoryInvalidator{}
	registry := newInvalidationRegistry(detail, category)

	msg := InvalidationMessage{
		Entries: []InvalidationEntry{
			{
				Cache:     cacheProgramDetailView,
				ProgramID: 20002,
			},
			{
				Cache:     cacheProgramDetailView,
				ProgramID: 20002,
			},
			{
				Cache: cacheCategorySnapshot,
			},
			{
				Cache: cacheCategorySnapshot,
			},
		},
	}

	if err := registry.Dispatch(msg); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if len(detail.calls) != 1 || detail.calls[0] != 20002 {
		t.Fatalf("expected detail invalidation once for 20002, got %+v", detail.calls)
	}
	if category.calls != 1 {
		t.Fatalf("expected category invalidation once, got %d", category.calls)
	}
}

type stubDetailInvalidator struct {
	calls []int64
}

func (s *stubDetailInvalidator) Invalidate(programID int64) {
	s.calls = append(s.calls, programID)
}

type stubCategoryInvalidator struct {
	calls int
}

func (s *stubCategoryInvalidator) Invalidate() {
	s.calls++
}
