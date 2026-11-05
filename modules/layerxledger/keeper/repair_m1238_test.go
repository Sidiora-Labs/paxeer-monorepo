package keeper_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type repairM1238Fixture struct {
	app                      *app.App
	ctx                      sdk.Context
	ledger                   *ledgerkeeper.IssuerBindingKeeper
	actor                    ledgerkeeper.Actor
	args                     types.RegisterArgs
	creator, subdenom, denom string
}

func newRepairM1238Fixture(t *testing.T) *repairM1238Fixture {
	t.Helper()
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.NewContext(false, tmproto.Header{
		ChainID: "hyperpax_125-1", Height: 11,
		Time: time.Unix(1_800_000_000, 0).UTC(),
	})
	ledger := ledgerkeeper.NewIssuerBindingKeeper(a.LayerXLedgerKeeper,
		ledgerkeeper.NewTokenfactoryIssuerKeeper(a.TokenFactoryKeeper))
	signer := newLedgerViewActor(t, 123)
	caller := common.Address{0x12, 0x38}
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(signer.private, message))
	_, err = a.EvmKeeper.BindLayerX(ctx, caller, signer.key, signature)
	require.NoError(t, err)
	actor, err := ledger.Actor(ctx, caller)
	require.NoError(t, err)
	require.Equal(t, signer.did, actor.DID)
	salt := [32]byte{0x12, 0x38}
	material := append([]byte("LX:ASSET:v1"), actor.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	creator := authtypes.NewModuleAddress(types.ModuleName).String()
	subdenom := fmt.Sprintf("%043s", new(big.Int).SetBytes(asset[:]).Text(62))
	denom, err := tokenfactorytypes.GetTokenDenom(creator, subdenom)
	require.NoError(t, err)
	return &repairM1238Fixture{
		app: a, ctx: ctx, ledger: ledger, actor: actor,
		args: types.RegisterArgs{AssetID: asset, Salt: salt, Symbol: "RPR", Name: "Issuer binding repair",
			Decimals: 6, SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative},
		creator: creator, subdenom: subdenom, denom: denom,
	}
}

func (f *repairM1238Fixture) stores(t *testing.T) map[string]map[string][]byte {
	t.Helper()
	rows := make(map[string]map[string][]byte)
	for _, name := range []string{"lxphost", types.StoreKey, tokenfactorytypes.StoreKey, banktypes.StoreKey, authtypes.StoreKey} {
		key := f.app.GetKey(name)
		require.NotNil(t, key)
		rows[name] = ledgerViewStore(t, f.ctx, key)
	}
	return rows
}

func (f *repairM1238Fixture) assertPause(t *testing.T, ledgerPaused, denomPaused bool) {
	t.Helper()
	asset, err := f.ledger.GetAsset(f.ctx, f.args.AssetID)
	require.NoError(t, err)
	require.Equal(t, f.actor.Authority.Actor, asset.IssuerDID)
	require.Equal(t, ledgerPaused, asset.Paused)
	require.Equal(t, denomPaused, f.app.TokenFactoryKeeper.IsDenomPaused(f.ctx, f.denom))
	response, err := f.app.TokenFactoryKeeper.DenomPaused(sdk.WrapSDKContext(f.ctx),
		&tokenfactorytypes.QueryDenomPausedRequest{Denom: f.denom})
	require.NoError(t, err)
	require.Equal(t, denomPaused, response.Paused)
	require.Equal(t, string(f.actor.DID), response.IssuerDid)
}

func TestRepairM_1238_RegisterQueriesAndPause(t *testing.T) {
	f := newRepairM1238Fixture(t)
	_, found := f.app.BankKeeper.GetDenomMetaData(f.ctx, f.denom)
	require.False(t, found)
	registered, err := f.ledger.RegisterAsset(f.ctx, f.actor, f.args, [32]byte{1})
	require.NoError(t, err)
	require.Zero(t, registered.Receipt.ResultCode)
	require.NotEmpty(t, registered.Receipt.Encoded)
	metadata, err := f.app.TokenFactoryKeeper.DenomMetadata(sdk.WrapSDKContext(f.ctx),
		&tokenfactorytypes.QueryDenomMetadataRequest{Denom: f.denom})
	require.NoError(t, err)
	require.Equal(t, f.denom, metadata.Metadata.Base)
	require.NoError(t, metadata.Metadata.Validate())
	authority, err := f.app.TokenFactoryKeeper.DenomAuthorityMetadata(sdk.WrapSDKContext(f.ctx),
		&tokenfactorytypes.QueryDenomAuthorityMetadataRequest{Denom: f.denom})
	require.NoError(t, err)
	require.Equal(t, tokenfactorytypes.DenomAuthorityMetadata{
		Admin: f.creator, IssuerDid: string(f.actor.DID),
	}, authority.AuthorityMetadata)
	issuer, err := f.app.TokenFactoryKeeper.IssuerDid(f.ctx, f.denom)
	require.NoError(t, err)
	require.Equal(t, string(f.actor.DID), issuer)
	denoms, err := f.app.TokenFactoryKeeper.DenomsFromCreator(sdk.WrapSDKContext(f.ctx),
		&tokenfactorytypes.QueryDenomsFromCreatorRequest{Creator: f.creator})
	require.NoError(t, err)
	require.Contains(t, denoms.Denoms, f.denom)
	_, issuance, err := ledgerkeeper.IssuanceAccountID(f.args.AssetID)
	require.NoError(t, err)
	mirror := types.MirrorDenom(f.args.AssetID)
	require.Equal(t, f.args.SupplyCap, f.app.BankKeeper.GetSupply(f.ctx, mirror).Amount)
	require.Equal(t, f.args.SupplyCap, f.app.BankKeeper.GetBalance(f.ctx, ledgerkeeper.BankAddress(issuance), mirror).Amount)
	f.assertPause(t, false, false)

	from := sdk.AccAddress(common.Address{0x38, 1}.Bytes())
	to := sdk.AccAddress(common.Address{0x38, 2}.Bytes())
	require.NoError(t, f.app.TokenFactoryKeeper.MintByIssuer(f.ctx, issuer,
		sdk.NewInt64Coin(f.denom, 100), from.String()))
	bank := bankkeeper.NewMsgServerImpl(f.app.BankKeeper)
	send := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin(f.denom, 10)))
	require.NoError(t, send.ValidateBasic())
	_, err = bank.Send(sdk.WrapSDKContext(f.ctx), send)
	require.NoError(t, err)
	paused, err := f.ledger.PauseAsset(f.ctx, f.actor, f.args.AssetID, [32]byte{2})
	require.NoError(t, err)
	require.Zero(t, paused.Receipt.ResultCode)
	f.assertPause(t, true, true)
	before := f.stores(t)
	events := f.ctx.EventManager().Events()
	_, err = bank.Send(sdk.WrapSDKContext(f.ctx), send)
	require.ErrorIs(t, err, banktypes.ErrSendDisabled)
	require.Equal(t, before, f.stores(t))
	require.Equal(t, events, f.ctx.EventManager().Events())
	unpaused, err := f.ledger.UnpauseAsset(f.ctx, f.actor, f.args.AssetID, [32]byte{3})
	require.NoError(t, err)
	require.Zero(t, unpaused.Receipt.ResultCode)
	f.assertPause(t, false, false)
	_, err = bank.Send(sdk.WrapSDKContext(f.ctx), send)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(80), f.app.BankKeeper.GetBalance(f.ctx, from, f.denom).Amount)
	require.Equal(t, sdk.NewInt(20), f.app.BankKeeper.GetBalance(f.ctx, to, f.denom).Amount)
}

func TestRepairM_1238_RegisterRefusalRollback(t *testing.T) {
	f := newRepairM1238Fixture(t)
	denom, err := f.app.TokenFactoryKeeper.CreateDenom(f.ctx, f.creator, f.subdenom)
	require.NoError(t, err)
	require.Equal(t, f.denom, denom)
	require.NoError(t, f.app.TokenFactoryKeeper.SetIssuerDid(f.ctx, denom, string(f.actor.DID)))
	before := f.stores(t)
	events := f.ctx.EventManager().Events()
	result, err := f.ledger.RegisterAsset(f.ctx, f.actor, f.args, [32]byte{1})
	require.ErrorIs(t, err, tokenfactorytypes.ErrDenomExists)
	require.Zero(t, result.Receipt.ResultCode)
	require.NotEmpty(t, result.Receipt.Encoded)
	require.Len(t, result.Events, 1)
	require.EqualValues(t, 1, result.Events[0].Code)
	require.Equal(t, f.args.AssetID[:], result.Events[0].Body)
	require.Equal(t, before, f.stores(t))
	require.Equal(t, events, f.ctx.EventManager().Events())
	_, err = f.ledger.GetAsset(f.ctx, f.args.AssetID)
	require.ErrorIs(t, err, types.ErrAssetMismatch)
	require.True(t, f.app.BankKeeper.GetSupply(f.ctx, types.MirrorDenom(f.args.AssetID)).IsZero())
	issuer, err := f.app.TokenFactoryKeeper.IssuerDid(f.ctx, f.denom)
	require.NoError(t, err)
	require.Equal(t, string(f.actor.DID), issuer)
}

func TestRepairM_1238_PauseRefusalRollback(t *testing.T) {
	f := newRepairM1238Fixture(t)
	_, err := f.ledger.RegisterAsset(f.ctx, f.actor, f.args, [32]byte{1})
	require.NoError(t, err)
	for _, operation := range []struct {
		name   string
		paused bool
		key    byte
		call   func(sdk.Context, ledgerkeeper.Actor, [32]byte, [32]byte) (ledgerkeeper.Result, error)
	}{
		{"pause", true, 2, f.ledger.PauseAsset},
		{"unpause", false, 3, f.ledger.UnpauseAsset},
	} {
		t.Run(operation.name, func(t *testing.T) {
			require.NoError(t, f.app.TokenFactoryKeeper.SetDenomPausedByIssuer(f.ctx,
				f.denom, string(f.actor.DID), operation.paused))
			f.assertPause(t, !operation.paused, operation.paused)
			before := f.stores(t)
			events := f.ctx.EventManager().Events()
			result, err := operation.call(f.ctx, f.actor, f.args.AssetID, [32]byte{operation.key})
			require.ErrorIs(t, err, tokenfactorytypes.ErrPauseUnchanged)
			require.Zero(t, result.Receipt.ResultCode)
			require.NotEmpty(t, result.Receipt.Encoded)
			require.Equal(t, before, f.stores(t))
			require.Equal(t, events, f.ctx.EventManager().Events())
			f.assertPause(t, !operation.paused, operation.paused)
			require.NoError(t, f.app.TokenFactoryKeeper.SetDenomPausedByIssuer(f.ctx,
				f.denom, string(f.actor.DID), !operation.paused))
			result, err = operation.call(f.ctx, f.actor, f.args.AssetID, [32]byte{operation.key})
			require.NoError(t, err)
			require.Zero(t, result.Receipt.ResultCode)
			f.assertPause(t, operation.paused, operation.paused)
		})
	}
}
