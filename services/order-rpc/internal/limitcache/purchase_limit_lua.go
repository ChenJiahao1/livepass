package limitcache

import _ "embed"

var (
	//go:embed reserve_purchase_limit.lua
	reservePurchaseLimitScript string

	//go:embed release_purchase_limit.lua
	releasePurchaseLimitScript string
)
