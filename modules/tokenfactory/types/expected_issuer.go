package types

import sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

type IssuerKeeper interface {
	CreateDenom(ctx sdk.Context, creatorAddr string, subdenom string) (string, error)
	SetIssuerDid(ctx sdk.Context, denom string, issuerDid string) error
	IssuerDid(ctx sdk.Context, denom string) (string, error)
	IsDenomPaused(ctx sdk.Context, denom string) bool
	SetDenomPausedByIssuer(ctx sdk.Context, denom, callerDid string, paused bool) error
	MintByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, to string) error
	BurnByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, from string) error
}
