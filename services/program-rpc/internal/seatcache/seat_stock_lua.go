package seatcache

import _ "embed"

var (
	//go:embed freeze_auto_assigned_seats.lua
	freezeAutoAssignedSeatsScript string

	//go:embed release_frozen_seats.lua
	releaseFrozenSeatsScript string

	//go:embed confirm_frozen_seats.lua
	confirmFrozenSeatsScript string
)
