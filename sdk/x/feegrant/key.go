package feegrant

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/address"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

const (
	// ModuleName is the module name constant used in many places
	ModuleName = "feegrant"

	// StoreKey is the store key string for supply
	StoreKey = ModuleName

	// RouterKey is the message route for supply
	RouterKey = ModuleName

	// QuerierRoute is the querier route for supply
	QuerierRoute = ModuleName
)

var (
	// FeeAllowanceKeyPrefix is the set of the kvstore for fee allowance data
	FeeAllowanceKeyPrefix = []byte{0x00}
)

// FeeAllowanceKey is the canonical key to store a grant from granter to grantee
// We store by grantee first to allow searching by everyone who granted to you
func FeeAllowanceKey(granter sdk.AccAddress, grantee sdk.AccAddress) []byte {
	return append(FeeAllowancePrefixByGrantee(grantee), address.MustLengthPrefix(granter.Bytes())...)
}

// FeeAllowancePrefixByGrantee returns a prefix to scan for all grants to this given address.
func FeeAllowancePrefixByGrantee(grantee sdk.AccAddress) []byte {
	return append(FeeAllowanceKeyPrefix, address.MustLengthPrefix(grantee.Bytes())...)
}

func ParseAddressesFromFeeAllowanceKey(key []byte) (granter, grantee sdk.AccAddress) {
	// key is of format:
	// 0x00<granteeAddressLen (1 Byte)><granteeAddress_Bytes><granterAddressLen (1 Byte)><granterAddress_Bytes>
	kv.AssertKeyAtLeastLength(key, 2)
	if key[0] != FeeAllowanceKeyPrefix[0] {
		panic("invalid fee allowance key prefix")
	}
	granteeAddrLen := int(key[1])
	if granteeAddrLen == 0 {
		panic("empty fee allowance grantee address")
	}
	granteeEnd := 2 + granteeAddrLen
	kv.AssertKeyAtLeastLength(key, granteeEnd+1)
	granterAddrLen := int(key[granteeEnd])
	if granterAddrLen == 0 {
		panic("empty fee allowance granter address")
	}
	granterStart := granteeEnd + 1
	kv.AssertKeyLength(key, granterStart+granterAddrLen)
	grantee = sdk.AccAddress(key[2:granteeEnd])
	granter = sdk.AccAddress(key[granterStart:])

	return granter, grantee
}
