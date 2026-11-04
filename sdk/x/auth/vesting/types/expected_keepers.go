package types

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
)

// BankKeeper defines the expected interface contract the vesting module requires
// for creating vesting accounts with funds.
type BankKeeper interface {
	IsSendEnabledCoins(ctx sdk.Context, coins ...sdk.Coin) error
	IsInDenomAllowList(ctx sdk.Context, addr sdk.AccAddress, coins sdk.Coins, cache map[string]bankkeeper.AllowedAddresses) bool
	SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	BlockedAddr(addr sdk.AccAddress) bool
}
