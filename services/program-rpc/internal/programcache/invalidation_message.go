package programcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	cacheProgramDetail    = "program_detail"
	cacheCategorySnapshot = "category_snapshot"
)

var (
	errInvalidationEntriesEmpty    = errors.New("invalidation message entries is empty")
	errInvalidationCacheRequired   = errors.New("invalidation entry cache is required")
	errInvalidationProgramIDNeeded = errors.New("program_detail requires program_id")
	errInvalidationProgramIDExtra  = errors.New("category_snapshot does not accept program_id")
)

type InvalidationEntry struct {
	Cache     string `json:"cache"`
	ProgramID int64  `json:"program_id,omitempty"`
}

type InvalidationMessage struct {
	Version     string              `json:"version"`
	Service     string              `json:"service"`
	InstanceID  string              `json:"instance_id"`
	PublishedAt time.Time           `json:"published_at"`
	Entries     []InvalidationEntry `json:"entries"`
}

func (m InvalidationMessage) Validate() error {
	if len(m.Entries) == 0 {
		return errInvalidationEntriesEmpty
	}

	for _, entry := range m.Entries {
		if entry.Cache == "" {
			return errInvalidationCacheRequired
		}
		switch entry.Cache {
		case cacheProgramDetail:
			if entry.ProgramID <= 0 {
				return errInvalidationProgramIDNeeded
			}
		case cacheCategorySnapshot:
			if entry.ProgramID != 0 {
				return errInvalidationProgramIDExtra
			}
		default:
			return fmt.Errorf("unknown cache type: %s", entry.Cache)
		}
	}

	return nil
}

func MarshalInvalidationMessage(msg InvalidationMessage) ([]byte, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(msg)
}

func ParseInvalidationMessage(payload []byte) (InvalidationMessage, error) {
	if len(payload) == 0 {
		return InvalidationMessage{}, errors.New("invalidation payload is empty")
	}

	var msg InvalidationMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return InvalidationMessage{}, err
	}
	if err := msg.Validate(); err != nil {
		return InvalidationMessage{}, err
	}

	return msg, nil
}
