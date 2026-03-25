package sharding

import "fmt"

const (
	MigrationModeLegacyOnly       = "legacy_only"
	MigrationModeDualWriteShadow  = "dual_write_shadow"
	MigrationModeDualWriteReadOld = "dual_write_new_read_old"
	MigrationModeDualWriteReadNew = "dual_write_new_read_new"
	MigrationModeShardOnly        = "shard_only"
)

var allowedMigrationModeTransitions = map[string]map[string]struct{}{
	MigrationModeLegacyOnly: {
		MigrationModeLegacyOnly:      {},
		MigrationModeDualWriteShadow: {},
	},
	MigrationModeDualWriteShadow: {
		MigrationModeLegacyOnly:       {},
		MigrationModeDualWriteShadow:  {},
		MigrationModeDualWriteReadOld: {},
	},
	MigrationModeDualWriteReadOld: {
		MigrationModeDualWriteShadow:  {},
		MigrationModeDualWriteReadOld: {},
		MigrationModeDualWriteReadNew: {},
	},
	MigrationModeDualWriteReadNew: {
		MigrationModeDualWriteReadOld: {},
		MigrationModeDualWriteReadNew: {},
		MigrationModeShardOnly:        {},
	},
	MigrationModeShardOnly: {
		MigrationModeDualWriteReadNew: {},
		MigrationModeShardOnly:        {},
	},
}

func ValidateMigrationModeTransition(from, to string) error {
	if _, ok := allowedMigrationModeTransitions[from]; !ok {
		return fmt.Errorf("unsupported migration mode: %s", from)
	}
	if _, ok := allowedMigrationModeTransitions[to]; !ok {
		return fmt.Errorf("unsupported migration mode: %s", to)
	}
	if _, ok := allowedMigrationModeTransitions[from][to]; !ok {
		return fmt.Errorf("illegal migration mode transition: %s -> %s", from, to)
	}

	return nil
}
