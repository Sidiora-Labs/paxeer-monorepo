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
