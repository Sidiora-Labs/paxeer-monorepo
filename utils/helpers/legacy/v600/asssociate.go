package v600

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils/helpers"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/mod/semver"
)

// IsHistoricalReplay matches the owning precompile selector's v6.0.0 range.
// Ordinary execution always uses the current helper, even at an old height.
func IsHistoricalReplay(ctx sdk.Context) bool {
	version := ctx.ClosestUpgradeName()
	return ctx.IsTracing() && semver.IsValid(version) &&
		semver.Compare(version, "v6.0.0") >= 0 && semver.Compare(version, "v6.0.1") < 0
}

type AssociationHelper struct {
	evmKeeper     utils.EVMKeeper
	bankKeeper    utils.BankKeeper
	accountKeeper utils.AccountKeeper
}

func NewAssociationHelper(evmKeeper utils.EVMKeeper, bankKeeper utils.BankKeeper, accountKeeper utils.AccountKeeper) *AssociationHelper {
	return &AssociationHelper{evmKeeper: evmKeeper, bankKeeper: bankKeeper, accountKeeper: accountKeeper}
}

func (p AssociationHelper) AssociateAddresses(ctx sdk.Context, paxAddr sdk.AccAddress, evmAddr common.Address, pubkey cryptotypes.PubKey) error {
	if !IsHistoricalReplay(ctx) {
		return helpers.NewAssociationHelper(p.evmKeeper, p.bankKeeper, p.accountKeeper).
			AssociateAddresses(ctx, paxAddr, evmAddr, pubkey, false)
	}
	p.evmKeeper.SetAddressMapping(ctx, paxAddr, evmAddr)
	if acc := p.accountKeeper.GetAccount(ctx, paxAddr); acc.GetPubKey() == nil {
		if err := acc.SetPubKey(pubkey); err != nil {
			return err
		}
		p.accountKeeper.SetAccount(ctx, acc)
	}
	return p.MigrateBalance(ctx, evmAddr, paxAddr)
}

func (p AssociationHelper) MigrateBalance(ctx sdk.Context, evmAddr common.Address, paxAddr sdk.AccAddress) error {
	if !IsHistoricalReplay(ctx) {
		return helpers.NewAssociationHelper(p.evmKeeper, p.bankKeeper, p.accountKeeper).
			MigrateBalance(ctx, evmAddr, paxAddr, false)
	}
	castAddr := sdk.AccAddress(evmAddr[:])
	castAddrBalances := p.bankKeeper.SpendableCoins(ctx, castAddr)
	if !castAddrBalances.IsZero() {
		if err := p.bankKeeper.SendCoins(ctx, castAddr, paxAddr, castAddrBalances); err != nil {
			return err
		}
	}
	castAddrWei := p.bankKeeper.GetWeiBalance(ctx, castAddr)
	if !castAddrWei.IsZero() {
		if err := p.bankKeeper.SendCoinsAndWei(ctx, castAddr, paxAddr, sdk.ZeroInt(), castAddrWei); err != nil {
			return err
		}
	}
	if p.bankKeeper.LockedCoins(ctx, castAddr).IsZero() {
		p.accountKeeper.RemoveAccount(ctx, authtypes.NewBaseAccountWithAddress(castAddr))
	}
	return nil
}
