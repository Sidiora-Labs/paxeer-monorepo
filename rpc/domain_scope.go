package evmrpc

import (
	"context"
	"math/big"
)

type DomainScope struct {
	ID      uint32
	ChainID *big.Int
}

type domainScopeKey struct{}

func WithDomainScope(ctx context.Context, scope DomainScope) context.Context {
	return context.WithValue(ctx, domainScopeKey{}, scope)
}

func DomainScopeFrom(ctx context.Context) (DomainScope, bool) {
	scope, ok := ctx.Value(domainScopeKey{}).(DomainScope)
	return scope, ok
}

type DomainResolver func(id uint32) (DomainScope, bool)

type domainResolverOption struct {
	resolver DomainResolver
}

func (domainResolverOption) serverOption() {}

func WithDomainResolver(resolver DomainResolver) ServerOption {
	return domainResolverOption{resolver: resolver}
}
