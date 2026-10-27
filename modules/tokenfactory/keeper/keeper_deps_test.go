package keeper_test

import (
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	"github.com/stretchr/testify/require"
)

func TestNewKeeperWithoutDistrKeeper(t *testing.T) {
	_, hasDistrKeeper := reflect.TypeOf(keeper.Keeper{}).FieldByName("distrKeeper")
	require.False(t, hasDistrKeeper)

	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()

	app := s.App
	constructed := keeper.NewKeeper(
		app.AppCodec(),
		app.GetKey(types.StoreKey),
		app.GetSubspace(types.ModuleName),
		app.AccountKeeper,
		app.BankKeeper.(bankkeeper.BaseKeeper).WithMintCoinsRestriction(types.NewTokenFactoryDenomMintCoinsRestriction()),
	)
	for _, tc := range []struct {
		name string
		k    keeper.Keeper
	}{
		{name: "app", k: app.TokenFactoryKeeper},
		{name: "constructor", k: constructed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := keeper.NewMsgServerImpl(tc.k)
			ctx := sdk.WrapSDKContext(s.Ctx)
			creator := s.TestAccs[0]
			created, err := server.CreateDenom(ctx, types.NewMsgCreateDenom(creator.String(), tc.name))
			require.NoError(t, err)
			require.NotNil(t, created)
			denom := "factory/" + creator.String() + "/" + tc.name
			require.Equal(t, denom, created.NewTokenDenom)

			authority, err := tc.k.GetAuthorityMetadata(s.Ctx, denom)
			require.NoError(t, err)
			require.Equal(t, creator.String(), authority.Admin)
			metadata, found := app.BankKeeper.GetDenomMetaData(s.Ctx, denom)
			require.True(t, found)
			require.Equal(t, denom, metadata.Base)

			amount := sdk.NewInt64Coin(denom, 25)
			_, err = server.Mint(ctx, types.NewMsgMint(creator.String(), amount))
			require.NoError(t, err)
			require.Equal(t, amount, app.BankKeeper.GetBalance(s.Ctx, creator, denom))
			require.Equal(t, amount, app.BankKeeper.GetSupply(s.Ctx, denom))
		})
	}
}
