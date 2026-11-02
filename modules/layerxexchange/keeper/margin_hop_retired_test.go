package keeper_test

import (
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func newMarginHopKeeper(t *testing.T) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.NewContext(false, tmtypes.Header{}).WithBlockHeight(11).WithBlockTime(genesisTime).CacheContext()
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx = withStore(t, ctx, key)
	upgradeKey := sdk.NewKVStoreKey("margin-hop-upgrade")
	ctx = withStore(t, ctx, upgradeKey)
	k := keeper.NewKeeper(app.AppCodec(), key, nil, &app.EvmKeeper)
	k.SetUpgradeKeeper(upgradekeeper.NewKeeper(nil, upgradeKey, app.AppCodec(), t.TempDir(), nil))
	k.InitGenesis(ctx, *types.DefaultGenesis())
	return k, ctx.WithEventManager(sdk.NewEventManager())
}

func TestDepositMarginIntentWithoutCustody(t *testing.T) {
	k, ctx := newMarginHopKeeper(t)
	assetID := [32]byte{1}
	owner := common.HexToAddress("0x1234")
	ownerAccount := sdk.AccAddress(owner.Bytes())
	amount := sdk.NewInt(2_500)
	require.Nil(t, k.Custody())
	require.NoError(t, k.SetMarket(ctx, types.Market{MarketId: custodytypes.Hash32(marketID),
		MarginAssetId: custodytypes.Hash32(assetID), Enabled: true}))
	before := testkeeper.EVMTestApp.BankKeeper.GetBalance(ctx, ownerAccount, sdk.MustGetBaseDenom())

	intent, err := k.RecordDepositMarginIntent(ctx, owner, ownerAccount, assetID, account, amount)
	require.NoError(t, err)
	intentID := types.IntentID(testkeeper.EVMTestApp.EvmKeeper.ChainID(ctx), owner, types.IntentKind_INTENT_KIND_DEPOSIT, 1)
	require.Equal(t, types.Intent{
		IntentId: custodytypes.Hash32(intentID), Kind: types.IntentKind_INTENT_KIND_DEPOSIT,
		Status: types.IntentStatus_INTENT_STATUS_PENDING, Owner: custodytypes.Address(owner),
		Account: custodytypes.Hash32(account), AssetId: custodytypes.Hash32(assetID),
		Amount: amount.String(), Nonce: 1, Height: ctx.BlockHeight(),
	}, intent)
	stored, found := k.GetIntent(ctx, intentID)
	require.True(t, found)
	require.Equal(t, intent, stored)
	require.Equal(t, uint64(1), k.GetOwnerNonce(ctx, owner))
	require.Equal(t, uint64(1), k.GetIntentCount(ctx))
	require.Equal(t, before, testkeeper.EVMTestApp.BankKeeper.GetBalance(ctx, ownerAccount, sdk.MustGetBaseDenom()))
	expectedEvent, err := sdk.TypedEventToEvent(&types.EventMarginDeposited{DepositId: intent.IntentId,
		MarginAccount: intent.Account, Owner: intent.Owner, AssetId: intent.AssetId, Amount: amount})
	require.NoError(t, err)
	require.Equal(t, sdk.Events{expectedEvent}, ctx.EventManager().Events())
}

func TestDepositMarginIntentRefusesNonMarginAsset(t *testing.T) {
	for _, tc := range []struct {
		name    string
		assetID [32]byte
		enabled bool
	}{
		{name: "asset absent from enabled markets", assetID: [32]byte{2}, enabled: true},
		{name: "asset margins only a disabled market", assetID: [32]byte{1}, enabled: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := newMarginHopKeeper(t)
			owner := common.HexToAddress("0x1234")
			require.NoError(t, k.SetMarket(ctx, types.Market{MarketId: custodytypes.Hash32(marketID),
				MarginAssetId: custodytypes.Hash32([32]byte{1}), Enabled: tc.enabled}))
			before := k.ExportGenesis(ctx)

			intent, err := k.RecordDepositMarginIntent(ctx, owner, sdk.AccAddress(owner.Bytes()), tc.assetID, account, sdk.NewInt(10))
			require.ErrorIs(t, err, types.ErrNotMarginAsset)
			require.Equal(t, types.Intent{}, intent)
			require.Equal(t, before, k.ExportGenesis(ctx))
			require.Zero(t, k.GetOwnerNonce(ctx, owner))
			require.Zero(t, k.GetIntentCount(ctx))
			require.Empty(t, ctx.EventManager().Events())
		})
	}
}
