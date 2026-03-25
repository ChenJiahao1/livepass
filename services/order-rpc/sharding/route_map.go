package sharding

import (
	"errors"
	"fmt"
)

const (
	RouteStatusStable      = "stable"
	RouteStatusShadowWrite = "shadow_write"
	RouteStatusBackfilling = "backfilling"
	RouteStatusVerifying   = "verifying"
	RouteStatusPrimaryNew  = "primary_new"
	RouteStatusRollback    = "rollback"

	WriteModeLegacyPrimary = "legacy_primary"
	WriteModeDualWrite     = "dual_write"
	WriteModeShardPrimary  = "shard_primary"
)

var (
	ErrRouteNotFound       = errors.New("route not found")
	ErrInvalidRouteEntry   = errors.New("invalid route entry")
	ErrRouteVersionMissing = errors.New("route map version missing")
)

var allowedRouteStatusTransitions = map[string]map[string]struct{}{
	RouteStatusStable: {
		RouteStatusStable:      {},
		RouteStatusShadowWrite: {},
	},
	RouteStatusShadowWrite: {
		RouteStatusStable:      {},
		RouteStatusShadowWrite: {},
		RouteStatusBackfilling: {},
	},
	RouteStatusBackfilling: {
		RouteStatusShadowWrite: {},
		RouteStatusBackfilling: {},
		RouteStatusVerifying:   {},
	},
	RouteStatusVerifying: {
		RouteStatusBackfilling: {},
		RouteStatusVerifying:   {},
		RouteStatusPrimaryNew:  {},
	},
	RouteStatusPrimaryNew: {
		RouteStatusPrimaryNew: {},
		RouteStatusRollback:   {},
	},
	RouteStatusRollback: {
		RouteStatusRollback:    {},
		RouteStatusShadowWrite: {},
		RouteStatusStable:      {},
	},
}

type Route struct {
	LogicSlot   int
	DBKey       string
	TableSuffix string
	Version     string
	WriteMode   string
	Status      string
}

type RouteEntry struct {
	Version     string
	LogicSlot   int
	DBKey       string
	TableSuffix string
	Status      string
	WriteMode   string
}

type RouteMap struct {
	currentVersion string
	entries        map[string]map[int]RouteEntry
}

func NewRouteMap(version string, entries []RouteEntry) (*RouteMap, error) {
	if version == "" {
		return nil, ErrRouteVersionMissing
	}

	snapshot := &RouteMap{
		currentVersion: version,
		entries:        make(map[string]map[int]RouteEntry),
	}
	for _, entry := range entries {
		if err := validateRouteEntry(entry); err != nil {
			return nil, err
		}
		if snapshot.entries[entry.Version] == nil {
			snapshot.entries[entry.Version] = make(map[int]RouteEntry)
		}
		if _, exists := snapshot.entries[entry.Version][entry.LogicSlot]; exists {
			return nil, fmt.Errorf("%w: duplicate slot=%d version=%s", ErrInvalidRouteEntry, entry.LogicSlot, entry.Version)
		}
		snapshot.entries[entry.Version][entry.LogicSlot] = entry
	}

	if _, ok := snapshot.entries[version]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrRouteVersionMissing, version)
	}

	return snapshot, nil
}

func (m *RouteMap) CurrentVersion() string {
	if m == nil {
		return ""
	}
	return m.currentVersion
}

func (m *RouteMap) RouteByLogicSlot(logicSlot int) (Route, error) {
	return m.RouteByVersionAndSlot(m.currentVersion, logicSlot)
}

func (m *RouteMap) RouteByVersionAndSlot(version string, logicSlot int) (Route, error) {
	if m == nil {
		return Route{}, ErrRouteNotFound
	}
	versionEntries, ok := m.entries[version]
	if !ok {
		return Route{}, fmt.Errorf("%w: version=%s", ErrRouteNotFound, version)
	}
	entry, ok := versionEntries[logicSlot]
	if !ok {
		return Route{}, fmt.Errorf("%w: version=%s slot=%d", ErrRouteNotFound, version, logicSlot)
	}

	return Route{
		LogicSlot:   entry.LogicSlot,
		DBKey:       entry.DBKey,
		TableSuffix: entry.TableSuffix,
		Version:     entry.Version,
		WriteMode:   entry.WriteMode,
		Status:      entry.Status,
	}, nil
}

func validateRouteEntry(entry RouteEntry) error {
	if entry.Version == "" || entry.DBKey == "" || entry.TableSuffix == "" {
		return fmt.Errorf("%w: empty required field", ErrInvalidRouteEntry)
	}
	if entry.LogicSlot < 0 || entry.LogicSlot >= logicSlotCount {
		return fmt.Errorf("%w: invalid logic slot %d", ErrInvalidRouteEntry, entry.LogicSlot)
	}
	if !isSupportedRouteStatus(entry.Status) {
		return fmt.Errorf("%w: unsupported route status %s", ErrInvalidRouteEntry, entry.Status)
	}
	if !isSupportedWriteMode(entry.WriteMode) {
		return fmt.Errorf("%w: unsupported write mode %s", ErrInvalidRouteEntry, entry.WriteMode)
	}
	if err := validateRouteStatusWriteMode(entry.Status, entry.WriteMode); err != nil {
		return err
	}

	return nil
}

func validateRouteStatusWriteMode(status, writeMode string) error {
	switch status {
	case RouteStatusStable, RouteStatusRollback:
		if writeMode != WriteModeLegacyPrimary {
			return fmt.Errorf("%w: route status %s requires write mode %s", ErrInvalidRouteEntry, status, WriteModeLegacyPrimary)
		}
	case RouteStatusShadowWrite, RouteStatusBackfilling, RouteStatusVerifying:
		if writeMode != WriteModeDualWrite {
			return fmt.Errorf("%w: route status %s requires write mode %s", ErrInvalidRouteEntry, status, WriteModeDualWrite)
		}
	case RouteStatusPrimaryNew:
		if writeMode != WriteModeShardPrimary {
			return fmt.Errorf("%w: route status %s requires write mode %s", ErrInvalidRouteEntry, status, WriteModeShardPrimary)
		}
	}
	return nil
}

func isSupportedRouteStatus(status string) bool {
	switch status {
	case RouteStatusStable, RouteStatusShadowWrite, RouteStatusBackfilling, RouteStatusVerifying, RouteStatusPrimaryNew, RouteStatusRollback:
		return true
	default:
		return false
	}
}

func isSupportedWriteMode(mode string) bool {
	switch mode {
	case WriteModeLegacyPrimary, WriteModeDualWrite, WriteModeShardPrimary:
		return true
	default:
		return false
	}
}

func ValidateRouteStatusTransition(from, to string) error {
	if !isSupportedRouteStatus(from) {
		return fmt.Errorf("%w: unsupported route status %s", ErrInvalidRouteEntry, from)
	}
	if !isSupportedRouteStatus(to) {
		return fmt.Errorf("%w: unsupported route status %s", ErrInvalidRouteEntry, to)
	}
	if _, ok := allowedRouteStatusTransitions[from][to]; !ok {
		return fmt.Errorf("%w: illegal route status transition %s -> %s", ErrInvalidRouteEntry, from, to)
	}
	return nil
}
