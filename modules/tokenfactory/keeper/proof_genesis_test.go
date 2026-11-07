// Proves: keeper.Keeper.ExportGenesis/InitGenesis -> types.GenesisState protobuf and codec.JSONCodec -> tokenfactory.AppModule.InitGenesis and keeper.MsgServer.Mint.
// Repairs modules/tokenfactory/keeper/genesis.go: InitGenesis accepted duplicate
// denoms and could write accounts, params and bank metadata before refusing an
// invalid genesis. It now validates the complete input before any write.
// Repairs ExportGenesisStream in transfer, launchpad, layerxanchor, layerxbridge
// and layerxcustody module.go: deferred goroutine reads could observe later state
// or a closed store. Streams now capture the producer's state before returning.
// The inventory's module.go callers export their own module genesis, rather than
// tokenfactory genesis; the proof exercises those real module methods separately.
// No producer, codec, module, keeper or persistent store is replaced by a double.
package keeper_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transfer "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type proofGenesisChain struct {
	store    storetypes.CommitMultiStore
	ctx      sdk.Context
	cdc      *codec.ProtoCodec
	accounts authkeeper.AccountKeeper
	bank     bankkeeper.BaseKeeper
	factory  keeper.Keeper
	module   tokenfactory.AppModule
}

// openProofGenesisChain constructs fresh keeper objects on a real disk database.
// Reopening retains only committed SDK state, including auth, bank and params.
func openProofGenesisChain(t *testing.T, dir string) proofGenesisChain {
	t.Helper()
	db, err := tmdb.NewGoLevelDB("genesis", dir)
	require.NoError(t, err)
	ms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(types.StoreKey, authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keys[paramtypes.StoreKey], transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: {authtypes.Minter, authtypes.Burner}})
	bank := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], accounts, subspace(banktypes.ModuleName), map[string]bool{})
	factory := keeper.NewKeeper(cdc, keys[types.StoreKey], subspace(types.ModuleName), accounts, bank)
	return proofGenesisChain{store: ms, ctx: sdk.NewContext(ms, tmproto.Header{Height: 1}, false), cdc: cdc,
		accounts: accounts, bank: bank, factory: factory, module: tokenfactory.NewAppModule(factory, accounts, bank)}
}

func TestProofGenesisProducerTransportConsumer(t *testing.T) {
	dir := t.TempDir()
	source := openProofGenesisChain(t, dir)
	source.accounts.SetParams(source.ctx, authtypes.DefaultParams())
	source.bank.SetParams(source.ctx, banktypes.DefaultParams())
	source.factory.InitGenesis(source.ctx, *types.DefaultGenesis())
	creator := sdk.AccAddress(bytes.Repeat([]byte{0x31}, 20))
	server := keeper.NewMsgServerImpl(source.factory)
	paused, err := server.CreateDenom(sdk.WrapSDKContext(source.ctx), types.NewMsgCreateDenom(creator.String(), "paused"))
	require.NoError(t, err)
	active, err := server.CreateDenom(sdk.WrapSDKContext(source.ctx), types.NewMsgCreateDenom(creator.String(), "active"))
	require.NoError(t, err)
	_, err = server.SetDenomPaused(sdk.WrapSDKContext(source.ctx), types.NewMsgSetDenomPaused(creator.String(), paused.NewTokenDenom, true))
	require.NoError(t, err)
	source.factory.SetParams(source.ctx, types.Params{DenomAllowlistMaxSize: 17})
	expected := source.factory.ExportGenesis(source.ctx)
	require.Equal(t, []string{paused.NewTokenDenom}, expected.GetPausedDenoms())
	commit := source.store.Commit(true)
	require.NoError(t, source.store.Close())

	// Producer state survives stopping and reopening the database and keepers.
	reopened := openProofGenesisChain(t, dir)
	defer func() { require.NoError(t, reopened.store.Close()) }()
	require.Equal(t, commit, reopened.store.LastCommitID())
	require.Equal(t, expected, reopened.factory.ExportGenesis(reopened.ctx))

	// Cross both real wire formats, using the producer's output exclusively.
	producerJSON := reopened.module.ExportGenesis(reopened.ctx, reopened.cdc)
	var message types.GenesisState
	require.NoError(t, reopened.cdc.UnmarshalAsJSON(producerJSON, &message))
	wire, err := message.Marshal()
	require.NoError(t, err)
	var received types.GenesisState
	require.NoError(t, received.Unmarshal(wire))
	require.Equal(t, expected, &received)
	consumerJSON := reopened.cdc.MustMarshalJSON(&received)
	targetDir := t.TempDir()
	target := openProofGenesisChain(t, targetDir)
	target.accounts.SetParams(target.ctx, authtypes.DefaultParams())
	target.bank.SetParams(target.ctx, banktypes.DefaultParams())
	require.NoError(t, target.module.ValidateGenesis(target.cdc, nil, consumerJSON))
	require.Empty(t, target.module.InitGenesis(target.ctx, target.cdc, consumerJSON))
	targetCommit := target.store.Commit(true)
	require.NoError(t, target.store.Close())
	consumer := openProofGenesisChain(t, targetDir)
	defer func() { require.NoError(t, consumer.store.Close()) }()
	require.Equal(t, targetCommit, consumer.store.LastCommitID())

	// Observe fresh reads twice, then change the producer and observe the change.
	for i := 0; i < 2; i++ {
		require.Equal(t, expected, consumer.factory.ExportGenesis(consumer.ctx))
		require.Equal(t, uint32(17), consumer.factory.GetDenomAllowListMaxSize(consumer.ctx))
		result, err := consumer.factory.DenomPaused(sdk.WrapSDKContext(consumer.ctx), &types.QueryDenomPausedRequest{Denom: paused.NewTokenDenom})
		require.NoError(t, err)
		require.True(t, result.Paused)
		require.False(t, consumer.factory.IsDenomPaused(consumer.ctx, active.NewTokenDenom))
	}
	consumerServer := keeper.NewMsgServerImpl(consumer.factory)
	_, err = consumerServer.Mint(sdk.WrapSDKContext(consumer.ctx), types.NewMsgMint(creator.String(), sdk.NewInt64Coin(active.NewTokenDenom, 9)))
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(9), consumer.bank.GetBalance(consumer.ctx, creator, active.NewTokenDenom).Amount)
	_, err = consumerServer.Mint(sdk.WrapSDKContext(consumer.ctx), types.NewMsgMint(creator.String(), sdk.NewInt64Coin(paused.NewTokenDenom, 9)))
	require.ErrorIs(t, err, types.ErrDenomPaused)
	require.True(t, consumer.bank.GetBalance(consumer.ctx, creator, paused.NewTokenDenom).IsZero())
	_, err = consumerServer.SetDenomPaused(sdk.WrapSDKContext(consumer.ctx), types.NewMsgSetDenomPaused(creator.String(), paused.NewTokenDenom, false))
	require.NoError(t, err)
	result, err := consumer.factory.DenomPaused(sdk.WrapSDKContext(consumer.ctx), &types.QueryDenomPausedRequest{Denom: paused.NewTokenDenom})
	require.NoError(t, err)
	require.False(t, result.Paused)
	require.Empty(t, consumer.factory.ExportGenesis(consumer.ctx).PausedDenoms)

	// Truncation of actual producer bytes is refused at the real decoders.
	var broken types.GenesisState
	require.Error(t, broken.Unmarshal(wire[:len(wire)-1]))
	before := consumer.module.ExportGenesis(consumer.ctx, consumer.cdc)
	malformed := producerJSON[:len(producerJSON)-1]
	require.Error(t, consumer.module.ValidateGenesis(consumer.cdc, nil, malformed))
	require.Panics(t, func() { consumer.module.InitGenesis(consumer.ctx, consumer.cdc, malformed) })
	require.Equal(t, before, consumer.module.ExportGenesis(consumer.ctx, consumer.cdc))
}

func TestProofGenesisRefusesInvalidInputBeforeWriting(t *testing.T) {
	source := openProofGenesisChain(t, t.TempDir())
	defer func() { require.NoError(t, source.store.Close()) }()
	source.factory.InitGenesis(source.ctx, *types.DefaultGenesis())
	creator := sdk.AccAddress(bytes.Repeat([]byte{0x32}, 20)).String()
	denom, err := source.factory.CreateDenom(source.ctx, creator, "duplicate")
	require.NoError(t, err)
	produced := source.factory.ExportGenesis(source.ctx)
	// Corrupt a real producer result while retaining a decodable protobuf.
	produced.FactoryDenoms = append(produced.FactoryDenoms, produced.FactoryDenoms[0])
	produced.Params.DenomAllowlistMaxSize = 7
	encoded := source.cdc.MustMarshalJSON(produced)
	target := openProofGenesisChain(t, t.TempDir())
	defer func() { require.NoError(t, target.store.Close()) }()
	require.Error(t, target.module.ValidateGenesis(target.cdc, nil, encoded))
	require.Panics(t, func() { target.module.InitGenesis(target.ctx, target.cdc, encoded) })
	require.Nil(t, target.accounts.GetAccount(target.ctx, authtypes.NewModuleAddress(types.ModuleName)))
	_, exists := target.bank.GetDenomMetaData(target.ctx, denom)
	require.False(t, exists)
	for _, key := range target.ctx.MultiStore().StoreKeys() {
		if _, ok := key.(*storetypes.KVStoreKey); !ok {
			continue
		}
		iterator := target.ctx.KVStore(key).Iterator(nil, nil)
		require.False(t, iterator.Valid(), "invalid genesis wrote store %s", key.Name())
		require.NoError(t, iterator.Close())
	}
}

type proofGenesisModule interface {
	DefaultGenesis(codec.JSONCodec) json.RawMessage
	ValidateGenesis(codec.JSONCodec, client.TxEncodingConfig, json.RawMessage) error
	ValidateGenesisStream(codec.JSONCodec, client.TxEncodingConfig, <-chan json.RawMessage) error
	InitGenesis(sdk.Context, codec.JSONCodec, json.RawMessage) []abci.ValidatorUpdate
	ExportGenesis(sdk.Context, codec.JSONCodec) json.RawMessage
	ExportGenesisStream(sdk.Context, codec.JSONCodec) <-chan json.RawMessage
}

func TestProofGenesisListedModuleConsumers(t *testing.T) {
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	app := s.App
	for _, tc := range []struct {
		name    string
		module  proofGenesisModule
		section string
		field   string
		value   json.RawMessage
	}{
		{"tokenfactory", tokenfactory.NewAppModule(app.TokenFactoryKeeper, app.AccountKeeper, app.BankKeeper), "params", "denom_allowlist_max_size", json.RawMessage(`17`)},
		{"transfer", transfer.NewAppModule(app.TransferKeeper), "params", "send_enabled", json.RawMessage(`false`)},
		{"launchpad", launchpad.NewAppModule(app.LaunchpadKeeper), "params", "creation_fee", json.RawMessage(`"1000001"`)},
		{"anchor", layerxanchor.NewAppModule(app.LayerXAnchorKeeper, app.UpgradeKeeper), "params", "challenge_window_seconds", json.RawMessage(`3`)},
		{"bridge", layerxbridge.NewAppModule(app.LayerXBridgeKeeper), "", "paused", json.RawMessage(`true`)},
		{"custody", layerxcustody.NewAppModule(app.LayerXCustodyKeeper), "params", "network_id", json.RawMessage(`42`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := s.Ctx.CacheContext()
			cdc := app.AppCodec()
			tc.module.InitGenesis(ctx, cdc, tc.module.DefaultGenesis(cdc))
			first := tc.module.ExportGenesis(ctx, cdc)
			stream := tc.module.ExportGenesisStream(ctx, cdc)
			// Advance the real producer before draining the first stream.
			var next map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(first, &next))
			if tc.section == "" {
				next[tc.field] = tc.value
			} else {
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(next[tc.section], &fields))
				fields[tc.field] = tc.value
				encoded, err := json.Marshal(fields)
				require.NoError(t, err)
				next[tc.section] = encoded
			}
			changed, err := json.Marshal(next)
			require.NoError(t, err)
			require.NoError(t, tc.module.ValidateGenesis(cdc, nil, changed))
			tc.module.InitGenesis(ctx, cdc, changed)
			captured, ok := <-stream
			require.True(t, ok)
			require.JSONEq(t, string(first), string(captured))
			_, ok = <-stream
			require.False(t, ok)
			second := tc.module.ExportGenesis(ctx, cdc)
			require.NotEqual(t, string(first), string(second))
			for i := 0; i < 2; i++ {
				current := tc.module.ExportGenesisStream(ctx, cdc)
				payload, ok := <-current
				require.True(t, ok)
				require.JSONEq(t, string(second), string(payload))
				consumerCtx, _ := s.Ctx.CacheContext()
				require.NoError(t, tc.module.ValidateGenesis(cdc, nil, payload))
				tc.module.InitGenesis(consumerCtx, cdc, payload)
				require.JSONEq(t, string(second), string(tc.module.ExportGenesis(consumerCtx, cdc)))
			}
			require.NoError(t, tc.module.ValidateGenesisStream(cdc, nil, tc.module.ExportGenesisStream(ctx, cdc)))
			malformed := second[:len(second)-1]
			invalidStream := make(chan json.RawMessage, 1)
			invalidStream <- malformed
			close(invalidStream)
			require.Error(t, tc.module.ValidateGenesisStream(cdc, nil, invalidStream))
			require.Panics(t, func() { tc.module.InitGenesis(ctx, cdc, malformed) })
			require.JSONEq(t, string(second), string(tc.module.ExportGenesis(ctx, cdc)))
		})
	}
}

func TestProofGenesisTokenfactoryStreamDiskRecovery(t *testing.T) {
	dir := t.TempDir()
	producer := openProofGenesisChain(t, dir)
	producer.accounts.SetParams(producer.ctx, authtypes.DefaultParams())
	producer.bank.SetParams(producer.ctx, banktypes.DefaultParams())
	producer.factory.InitGenesis(producer.ctx, *types.DefaultGenesis())
	creator := sdk.AccAddress(bytes.Repeat([]byte{0x43}, 20)).String()
	denom, err := producer.factory.CreateDenom(producer.ctx, creator, "stream")
	require.NoError(t, err)
	require.NoError(t, producer.factory.SetIssuerDid(producer.ctx, denom, adminsIssuerDid(0x43)))
	require.NoError(t, producer.factory.SetDenomPausedByIssuer(producer.ctx, denom, adminsIssuerDid(0x43), true))
	producer.factory.SetParams(producer.ctx, types.Params{DenomAllowlistMaxSize: 17})
	expected := producer.factory.ExportGenesis(producer.ctx)
	first := producer.module.ExportGenesis(producer.ctx, producer.cdc)
	commit := producer.store.Commit(true)
	require.NoError(t, producer.store.Close())

	// Reconstruct the actual producer from its committed disk state.
	producer = openProofGenesisChain(t, dir)
	closed := false
	t.Cleanup(func() {
		if !closed {
			require.NoError(t, producer.store.Close())
		}
	})
	require.Equal(t, commit, producer.store.LastCommitID())
	require.Equal(t, expected, producer.factory.ExportGenesis(producer.ctx))
	stream := producer.module.ExportGenesisStream(producer.ctx, producer.cdc)
	// Change valid live state before consuming: the channel must retain the
	// genesis that existed when ExportGenesisStream returned.
	producer.factory.SetParams(producer.ctx, types.Params{DenomAllowlistMaxSize: 23})
	later := producer.module.ExportGenesis(producer.ctx, producer.cdc)
	require.NotEqual(t, string(first), string(later))
	laterCommit := producer.store.Commit(true)
	require.NoError(t, producer.store.Close())
	closed = true

	receive := func() (json.RawMessage, bool) {
		t.Helper()
		select {
		case value, ok := <-stream:
			return value, ok
		case <-time.After(5 * time.Second):
			t.Fatal("tokenfactory genesis stream did not deliver or close after producer shutdown")
			return nil, false
		}
	}
	captured, ok := receive()
	require.True(t, ok)
	_, ok = receive()
	require.False(t, ok, "producer must close the channel after its one genesis value")
	// Keep exercising disk recovery and the consumer even when the snapshot
	// assertion diagnoses a producer outside this task's writable paths.
	snapshotMatches := json.Valid(captured) && bytes.Equal(first, captured)
	if !snapshotMatches {
		t.Errorf("stream snapshot changed before drain: expected %s, received %s", first, captured)
	}
	reopened := openProofGenesisChain(t, dir)
	defer func() { require.NoError(t, reopened.store.Close()) }()
	require.Equal(t, laterCommit, reopened.store.LastCommitID())
	require.JSONEq(t, string(later), string(reopened.module.ExportGenesis(reopened.ctx, reopened.cdc)))

	targetDir := t.TempDir()
	target := openProofGenesisChain(t, targetDir)
	target.accounts.SetParams(target.ctx, authtypes.DefaultParams())
	target.bank.SetParams(target.ctx, banktypes.DefaultParams())
	require.NoError(t, target.module.ValidateGenesis(target.cdc, nil, captured))
	require.Empty(t, target.module.InitGenesis(target.ctx, target.cdc, captured))
	consumerCommit := target.store.Commit(true)
	require.NoError(t, target.store.Close())
	consumer := openProofGenesisChain(t, targetDir)
	defer func() { require.NoError(t, consumer.store.Close()) }()
	require.Equal(t, consumerCommit, consumer.store.LastCommitID())
	require.Equal(t, expected, consumer.factory.ExportGenesis(consumer.ctx))
	require.JSONEq(t, string(first), string(consumer.module.ExportGenesis(consumer.ctx, consumer.cdc)))
}
