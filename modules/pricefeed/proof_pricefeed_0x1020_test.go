// Proves: pricefeed.MarkSubmission and pricefeed.AppModuleBasic.ValidateGenesis -> sdk/client.TxConfig and module.AppModuleBasic -> ante.CheckMessage and pricefeed.AppModule.InitGenesis.
//
// The producer uses real staking/auth keepers, RegisterFeedKey, SubmitPrices,
// Aggregate and ExportGenesis. Its IAVL database is closed and reopened before
// consumers read it; exported JSON is also imported into a fresh real store.
// Each CheckMessage call decodes newly encoded SDK transaction bytes. Repeated
// reads, changed heights and revoked keys demonstrate that there is no cached
// acceptance. Keeper.MarketSymbol (the third changed/new inventory surface)
// currently has no listed caller; its exported result is checked against the
// imported producer state and a subsequent parameter update.
//
// Repairs modules/pricefeed/ante.go:MarkSubmission and
// node/ante/cosmos_checktx.go:CheckMessage accepting mixed price submissions as
// non-submissions, including during ReCheckTx. Both must return
// ErrSubmissionAlone without writing a rate mark. The module's validation and
// import boundaries share their JSON decoder so malformed producer output
// carries the same registered refusal at both boundaries.
package pricefeed_test

import (
	"encoding/json"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	oraclekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// proofPricefeedStore contains only production stores and keepers. No test
// application, overlaid store, transport implementation or keeper is substituted.
type proofPricefeedStore struct {
	stores   storetypes.CommitMultiStore
	cache    sdk.CacheMultiStore
	ctx      sdk.Context
	key      *sdk.KVStoreKey
	staking  stakingkeeper.Keeper
	keeper   keeper.Keeper
	module   pricefeed.AppModule
	txConfig client.TxConfig
}

func openProofPricefeedStore(t *testing.T, dir string) *proofPricefeedStore {
	t.Helper()
	registry := cdctypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	basic := pricefeed.AppModuleBasic{}
	basic.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	key := sdk.NewKVStoreKey(types.StoreKey)
	stakingKey := sdk.NewKVStoreKey(stakingtypes.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	db, err := dbm.NewDB("pricefeed-proof", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	stores := store.NewCommitMultiStore(db)
	for _, mounted := range []*sdk.KVStoreKey{key, stakingKey, authKey, paramsKey} {
		stores.MountStoreWithDB(mounted, sdk.StoreTypeIAVL, nil)
	}
	stores.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, stores.LoadLatestVersion())
	cache := stores.CacheMultiStore()
	ctx := sdk.NewContext(cache, tmproto.Header{Height: 50, Time: time.UnixMilli(moduleNowMs).UTC()}, false).
		WithEventManager(sdk.NewEventManager())
	accounts := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{
			stakingtypes.BondedPoolName:    nil,
			stakingtypes.NotBondedPoolName: nil,
		})
	staking := stakingkeeper.NewKeeper(cdc, stakingKey, accounts, nil,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, stakingtypes.ModuleName))
	k := keeper.NewKeeper(key, staking, accounts, nil)
	return &proofPricefeedStore{stores: stores, cache: cache, ctx: ctx, key: key,
		staking: staking, keeper: k, module: pricefeed.NewAppModule(k),
		txConfig: authtx.NewTxConfig(cdc, authtx.DefaultSignModes)}
}

func (s *proofPricefeedStore) close(t *testing.T) {
	t.Helper()
	s.cache.Close()
	require.NoError(t, s.stores.Close())
}

// checkMessage crosses the application's actual protobuf TxEncoder/TxDecoder
// boundary before invoking the post-signature CheckTx consumer itself.
func (s *proofPricefeedStore) checkMessage(t *testing.T, ctx sdk.Context, msgs ...sdk.Msg) error {
	t.Helper()
	builder := s.txConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msgs...))
	encoded, err := s.txConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	tx, err := s.txConfig.TxDecoder()(encoded)
	require.NoError(t, err)
	return ante.CheckMessage(ctx.WithTxBytes(encoded), tx, nil, oraclekeeper.Keeper{}, s.keeper)
}

func TestProofPricefeed0x1020ProducerTransportConsumer(t *testing.T) {
	dir := t.TempDir()
	producer := openProofPricefeedStore(t, dir)
	basic := module.AppModuleBasic(pricefeed.AppModuleBasic{})
	initial := basic.DefaultGenesis(nil)
	require.NoError(t, basic.ValidateGenesis(nil, nil, initial))
	producer.module.InitGenesis(producer.ctx, nil, initial)
	producer.staking.SetParams(producer.ctx, stakingtypes.DefaultParams())
	operator := sdk.ValAddress(secp256k1.GenPrivKeyFromSecret([]byte("proof 0x1020 operator")).PubKey().Address())
	validator, err := stakingtypes.NewValidator(operator,
		ed25519.GenPrivKeyFromSecret([]byte("proof 0x1020 consensus")).PubKey(), stakingtypes.Description{})
	require.NoError(t, err)
	validator = validator.UpdateStatus(stakingtypes.Bonded)
	validator.Tokens = sdk.NewInt(1_000_000)
	producer.staking.SetValidator(producer.ctx, validator)
	producer.staking.SetValidatorByPowerIndex(producer.ctx, validator)
	feeder, _, err := producer.keeper.RegisterFeedKey(producer.ctx, operator,
		secp256k1.GenPrivKeyFromSecret([]byte("proof 0x1020 feed")).PubKey().Bytes())
	require.NoError(t, err)
	msg := &types.MsgSubmitPrices{Feeder: feeder.String(), Observations: []types.Observation{{
		MarketId: 0, Sequence: 1, IndexPrice: "65000000000000000000000",
		MarkPrice: "65000000000000000000000", SourceTimestampMs: uint64(moduleNowMs - 1000),
	}}}
	response, err := keeper.NewMsgServerImpl(producer.keeper).SubmitPrices(sdk.WrapSDKContext(producer.ctx), msg)
	require.NoError(t, err)
	require.Equal(t, uint32(1), response.Accepted)
	require.Zero(t, response.Refused)
	producer.module.MidBlock(producer.ctx, 50)
	exported := producer.module.ExportGenesis(producer.ctx, nil)
	producer.cache.Write()
	commit := producer.stores.Commit(true)
	require.Positive(t, commit.Version)
	producer.close(t)

	consumer := openProofPricefeedStore(t, dir)
	defer consumer.close(t)
	require.Equal(t, commit, consumer.stores.LastCommitID())
	require.JSONEq(t, string(exported), string(consumer.module.ExportGenesis(consumer.ctx, nil)))

	// A separate importer receives only bytes emitted by ExportGenesis. Its
	// registered module interfaces perform validation and import as the app does.
	imported := openProofPricefeedStore(t, t.TempDir())
	defer imported.close(t)
	for round := int64(0); round < 2; round++ {
		require.NoError(t, basic.ValidateGenesis(nil, nil, exported))
		check := consumer.ctx.WithIsCheckTx(true).WithBlockHeight(50 + round)
		require.NoError(t, consumer.checkMessage(t, check, msg))
		last, found := consumer.keeper.GetLastSubmissionHeight(check, operator)
		require.True(t, found)
		require.Equal(t, int64(51)+round, last)
		require.ErrorIs(t, consumer.checkMessage(t, check, msg), types.ErrRateLimited)
		// ReCheckTx must neither refuse the existing mark nor advance it.
		recheck := check.WithBlockHeight(100 + round).WithIsReCheckTx(true)
		require.NoError(t, consumer.checkMessage(t, recheck, msg))
		last, found = consumer.keeper.GetLastSubmissionHeight(check, operator)
		require.True(t, found)
		require.Equal(t, int64(51)+round, last)

		imported.module.InitGenesis(imported.ctx, nil, exported)
		price, err := imported.keeper.GetPrice(imported.ctx, 0)
		require.NoError(t, err)
		require.True(t, price.Fresh)
		require.Equal(t, msg.Observations[0].IndexPrice, price.Index.String())
		require.Equal(t, msg.Observations[0].MarkPrice, price.Mark.String())
		require.Equal(t, int64(50), price.Height)
		require.Equal(t, uint32(1), price.Sources)
		symbol, enabled, err := imported.keeper.MarketSymbol(imported.ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "BTC", symbol)
		require.True(t, enabled)
		_, found = imported.keeper.GetLastSubmissionHeight(imported.ctx, operator)
		require.False(t, found, "check-state rate marks must not cross genesis export")
	}

	params := imported.keeper.GetParams(imported.ctx)
	params.Markets[0].Enabled = false
	require.NoError(t, imported.keeper.SetParams(imported.ctx, params))
	for i := 0; i < 2; i++ {
		symbol, enabled, err := imported.keeper.MarketSymbol(imported.ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "BTC", symbol)
		require.False(t, enabled, "the read must observe the updated params")
		_, _, err = imported.keeper.MarketSymbol(imported.ctx, uint64(len(params.Markets)))
		require.ErrorIs(t, err, types.ErrUnknownMarket)
	}

	_, err = consumer.keeper.RevokeFeedKey(consumer.ctx, operator)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		check := consumer.ctx.WithIsCheckTx(true).WithBlockHeight(60)
		require.ErrorIs(t, consumer.checkMessage(t, check, msg), types.ErrUnregisteredFeeder)
		last, found := consumer.keeper.GetLastSubmissionHeight(check, operator)
		require.True(t, found)
		require.Equal(t, int64(52), last, "refusal must preserve the previous mark")
	}
}

func TestProofPricefeed0x1020MixedSubmissionRefused(t *testing.T) {
	s := openProofPricefeedStore(t, t.TempDir())
	defer s.close(t)
	msg := &types.MsgSubmitPrices{Feeder: account("proof mixed").String()}
	for _, recheck := range []bool{false, true} {
		ctx := s.ctx.WithIsCheckTx(true).WithIsReCheckTx(recheck)
		for _, msgs := range [][]sdk.Msg{{msg, msg}, {send("proof mixed"), msg}, {msg, send("proof mixed")}} {
			require.ErrorIs(t, s.checkMessage(t, ctx, msgs...), types.ErrSubmissionAlone)
		}
	}
	iterator := sdk.KVStorePrefixIterator(s.ctx.KVStore(s.key), types.LastSubmitPrefix)
	defer func() { require.NoError(t, iterator.Close()) }()
	require.False(t, iterator.Valid(), "mixed submissions cannot write a mark")
}

func TestProofPricefeed0x1020MalformedProducerResultRefused(t *testing.T) {
	s := openProofPricefeedStore(t, t.TempDir())
	defer s.close(t)
	basic := pricefeed.AppModuleBasic{}
	s.module.InitGenesis(s.ctx, nil, basic.DefaultGenesis(nil))
	exported := s.module.ExportGenesis(s.ctx, nil)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(exported, &fields))
	// Damage a real producer result at its transport boundary, rather than
	// supplying a hand-authored substitute for the happy-path producer.
	fields["feed_keys"] = json.RawMessage(`"not an array"`)
	wrongType, err := json.Marshal(fields)
	require.NoError(t, err)
	for _, malformed := range []json.RawMessage{exported[:len(exported)-1], wrongType} {
		for i := 0; i < 2; i++ {
			err := basic.ValidateGenesis(nil, nil, malformed)
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
			name, found := types.LXPName(err)
			require.True(t, found)
			require.Equal(t, "LXP_PRICEFEED_INVALID_GENESIS", name)
			require.PanicsWithError(t, err.Error(), func() { s.module.InitGenesis(s.ctx, nil, malformed) })
			require.JSONEq(t, string(exported), string(s.module.ExportGenesis(s.ctx, nil)))
		}
	}
	// Syntactically valid JSON must still receive the keeper's full checks.
	var genesis types.GenesisState
	require.NoError(t, json.Unmarshal(exported, &genesis))
	genesis.Aggregates = []types.GenesisAggregate{{MarketID: uint64(len(genesis.Params.Markets)), Aggregate: types.Aggregate{
		IndexPrice: sdk.OneInt(), MarkPrice: sdk.OneInt(), Basis: sdk.ZeroInt(), Sources: 1,
	}}}
	unknownMarket, err := json.Marshal(genesis)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		err := basic.ValidateGenesis(nil, nil, unknownMarket)
		require.ErrorIs(t, err, types.ErrInvalidGenesis)
		require.Contains(t, err.Error(), "unlisted market")
		require.PanicsWithError(t, err.Error(), func() { s.module.InitGenesis(s.ctx, nil, unknownMarket) })
		require.JSONEq(t, string(exported), string(s.module.ExportGenesis(s.ctx, nil)))
	}
}
