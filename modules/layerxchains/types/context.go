package types

import sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

type domainContextKey struct{}

var DomainContextKey = domainContextKey{}

func WithDomain(ctx sdk.Context, id uint32) sdk.Context {
	return ctx.WithValue(DomainContextKey, id)
}

func DomainFrom(ctx sdk.Context) (uint32, bool) {
	id, ok := ctx.Value(DomainContextKey).(uint32)
	return id, ok && id != 0
}

// ChainIDFrom returns the transaction's virtual chain ID, or zero on the root network.
func ChainIDFrom(ctx sdk.Context) uint64 {
	id, ok := DomainFrom(ctx)
	if !ok {
		return 0
	}
	return ChainIDOf(id).Uint64()
}
