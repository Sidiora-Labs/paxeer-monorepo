package types

import (
	"encoding/binary"
)

const (
	// ModuleName is the module name and the name of the account that escrows
	// every market's token supply, real quote balance and undistributed fees.
	ModuleName = "launchpad"

	// TreasuryName is the module account that receives creation fees and the
	// protocol share of buy fees.
	TreasuryName = "launchpad_treasury"

	// StoreKey is the primary module store key.
	StoreKey = ModuleName

	// RouterKey is the message route.
	RouterKey = ModuleName

	// QuerierRoute is the query routing key.
	QuerierRoute = ModuleName

	// LaunchpadAddress is the EVM address of the launchpad precompile.
	LaunchpadAddress = "0x0000000000000000000000000000000000001017"

	// DeadAddress is FeeAccumulator's burn destination.
	DeadAddress = "0x000000000000000000000000000000000000dEaD"

	// TokenDecimals is the exponent of every launched token's display unit.
	TokenDecimals = 6
)

var (
	ParamsKey            = []byte{0x01}
	MarketCountKey       = []byte{0x02}
	ProtocolFeesPrefix   = []byte{0x03}
	MarketPrefix         = []byte{0x10}
	MarketIndexPrefix    = []byte{0x11}
	MarketPointerPrefix  = []byte{0x12}
	CreatorMarketPrefix  = []byte{0x13}
	AirdropEpochPrefix   = []byte{0x20}
	AirdropClaimPrefix   = []byte{0x21}
	HoldingHistoryPrefix = []byte{0x22}
	HoldingCountPrefix   = []byte{0x23}
	HoldingEntryPrefix   = []byte{0x24}
	AirdropBasisPrefix   = []byte{0x25}

	SurfacePrefix        = []byte{0x30}
	RolePrefix           = []byte{0x31}
	HeldQuotePrefix      = []byte{0x32}
	FeeHeldPrefix        = []byte{0x33}
	TreasuryLedgerPrefix = []byte{0x34}
	TreasuryHeldPrefix   = []byte{0x35}
	BookHeldDeltaPrefix  = []byte{0x36}
	FeeRightIDMapPrefix  = []byte{0x37}
	RetiredPrefix        = []byte{0x38}
	MigratedKey          = []byte{0x39}
)

func join(prefix []byte, parts ...[]byte) []byte {
	out := append([]byte(nil), prefix...)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func lengthPrefixed(value []byte) []byte {
	return append([]byte{byte(len(value))}, value...)
}

func uint64Bytes(value uint64) []byte { return binary.BigEndian.AppendUint64(nil, value) }

func MarketKey(denom string) []byte            { return join(MarketPrefix, []byte(denom)) }
func MarketIndexKey(index uint64) []byte       { return join(MarketIndexPrefix, uint64Bytes(index)) }
func MarketPointerKey(pointer [20]byte) []byte { return join(MarketPointerPrefix, pointer[:]) }

// CreatorMarketPrefixFor is the prefix of one creator's market index.
func CreatorMarketPrefixFor(creator []byte) []byte {
	return join(CreatorMarketPrefix, lengthPrefixed(creator))
}

func CreatorMarketKey(creator []byte, index uint64) []byte {
	return join(CreatorMarketPrefixFor(creator), uint64Bytes(index))
}

// SurfaceKey is a registered surface address; its record names the role,
// generation and market the surface serves.
func SurfaceKey(surface [20]byte) []byte { return join(SurfacePrefix, surface[:]) }

// RoleKey is one account's membership of a role at a role contract.
func RoleKey(contract [20]byte, role [32]byte, account [20]byte) []byte {
	return join(RolePrefix, contract[:], role[:], account[:])
}

// HeldQuoteKey is the quote held in custody for one pool.
func HeldQuoteKey(pool [20]byte) []byte { return join(HeldQuotePrefix, pool[:]) }

// FeeHeldKey is the quote held in custody for one FeeAccumulator.
func FeeHeldKey(accumulator [20]byte) []byte { return join(FeeHeldPrefix, accumulator[:]) }

// TreasuryLedgerKey is the Treasury _balances ledger of one token.
func TreasuryLedgerKey(token [20]byte) []byte { return join(TreasuryLedgerPrefix, token[:]) }

// TreasuryHeldKey is the amount of one token held in custody for the Treasury.
func TreasuryHeldKey(token [20]byte) []byte { return join(TreasuryHeldPrefix, token[:]) }

// BookHeldDeltaKey is the migration book-held delta of one pool and asset.
func BookHeldDeltaKey(pool [20]byte, asset [20]byte) []byte {
	return join(BookHeldDeltaPrefix, pool[:], asset[:])
}

// FeeRightIDMapKey maps a generation 1 fee-right id to its generation 2 id.
func FeeRightIDMapKey(generation1ID uint64) []byte {
	return join(FeeRightIDMapPrefix, uint64Bytes(generation1ID))
}

// RetiredKey is a retired generation 1 address; its record names the role and
// replacement.
func RetiredKey(address [20]byte) []byte { return join(RetiredPrefix, address[:]) }
