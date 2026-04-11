package programcache

import "errors"

type detailInvalidator interface {
	Invalidate(programID int64)
}

type categoryInvalidator interface {
	Invalidate()
}

type InvalidationRegistry struct {
	detailCache   detailInvalidator
	categoryCache categoryInvalidator
}

func NewInvalidationRegistry(detailCache *ProgramDetailCache, categoryCache *CategorySnapshotCache) *InvalidationRegistry {
	return newInvalidationRegistry(detailCache, categoryCache)
}

func newInvalidationRegistry(detailCache detailInvalidator, categoryCache categoryInvalidator) *InvalidationRegistry {
	return &InvalidationRegistry{
		detailCache:   detailCache,
		categoryCache: categoryCache,
	}
}

func (r *InvalidationRegistry) Dispatch(msg InvalidationMessage) error {
	if r == nil {
		return errors.New("invalidation registry is nil")
	}
	if err := msg.Validate(); err != nil {
		return err
	}

	seen := make(map[entryKey]struct{}, len(msg.Entries))
	for _, entry := range msg.Entries {
		key := entryKey{
			cache:     entry.Cache,
			programID: entry.ProgramID,
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		switch entry.Cache {
		case cacheProgramDetail:
			if r.detailCache != nil {
				r.detailCache.Invalidate(entry.ProgramID)
			}
		case cacheCategorySnapshot:
			if r.categoryCache != nil {
				r.categoryCache.Invalidate()
			}
		default:
			return errors.New("unknown cache type: " + entry.Cache)
		}
	}

	return nil
}

type entryKey struct {
	cache     string
	programID int64
}
