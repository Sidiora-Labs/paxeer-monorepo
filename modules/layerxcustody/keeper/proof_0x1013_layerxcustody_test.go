// Proves: keeper.NewMsgServerImpl, Keeper.ReleaseRetired, Keeper.Retired -> exported Go MsgServer, SDK codec and stores -> layerxcustody.NewProposalHandler and AppModule.RegisterServices.
//
// The consumers use the same proposal packing, decoding and routing as
// handler_test.go and node/gov_custody_test.go. Real account, bank,
// EVM and upgrade keepers run over a disk-backed SDK multistore, closed and
// reopened between production and observation. No transport is replaced.
//
// Repairs: handler.go NewProposalHandler and msg_server.go MsgServer methods
// used to panic on typed-nil proposals and nil requests; they now refuse them
// without writes, with retirement still taking precedence.
// deposit_root_test.go also used the retired verify.BatchHeader export; it now
// verifies the signature with Ed25519Digest over codec.BatchHeaderDigest and
// records that same digest alongside the already decoded header fields.
//
// Out-of-scope behavior: claims.go Keeper.VerifyWithdrawal always returns
// ErrModuleRetired, even below the merge height. Assert that refusal through
// NewMsgServerImpl; do not synthesize a pending claim to bypass the producer.
//
// The authority anchor owns no state. In anchor.go, authorityAnchor's root and
// authorization readers must re-read custody on each call and refuse malformed
// stored hex when decoding. The proof fault-injects a changed field of a real
// producer result to exercise that existing behavior without changing anchor.go.
package keeper_test

import (
	"context"
	"encoding/json"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// proofCustodyDisk copies the real app's initialized state once, then uses only
// the persistent SDK store. Reopening constructs fresh stores and a fresh keeper.
func proofCustodyDisk(t *testing.T, initial sdk.Context) (sdk.Context, *keeper.Keeper, func(sdk.Context) (sdk.Context, *keeper.Keeper)) {
	t.Helper()
	app := testkeeper.EVMTestApp
	dir := t.TempDir()
	var keys []storetypes.StoreKey
	for _, key := range initial.MultiStore().StoreKeys() {
		if _, ok := key.(*storetypes.KVStoreKey); ok {
			keys = append(keys, key)
		}
	}
	require.NotEmpty(t, keys)
	var disk *rootmulti.Store
	open := func(parent sdk.Context) (sdk.Context, *keeper.Keeper) {
		db, err := dbm.NewGoLevelDB("custody-proof", dir)
		require.NoError(t, err)
		disk = rootmulti.NewStore(db)
		for _, key := range keys {
			disk.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
		}
		require.NoError(t, disk.LoadLatestVersion())
		ctx := parent.WithMultiStore(disk).WithEventManager(sdk.NewEventManager())
		k := keeper.NewKeeper(app.AppCodec(), app.GetKey(types.StoreKey), app.AccountKeeper,
			app.BankKeeper, &app.EvmKeeper, app.UpgradeKeeper)
		return ctx, k
	}
	ctx, k := open(initial)
	for _, key := range keys {
		it := initial.KVStore(key).Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			ctx.KVStore(key).Set(append([]byte(nil), it.Key()...), append([]byte{}, it.Value()...))
		}
		require.NoError(t, it.Close())
	}
	t.Cleanup(func() { require.NoError(t, disk.Close()) })
	reopen := func(parent sdk.Context) (sdk.Context, *keeper.Keeper) {
		t.Helper()
		committed := disk.Commit(true)
		require.Positive(t, committed.Version)
		require.NoError(t, disk.Close())
		ctx, k := open(parent)
		require.Equal(t, committed, disk.LastCommitID())
		return ctx, k
	}
	return ctx, k, reopen
}

func TestProof0x1013LayerxcustodyProducerTransportConsumer(t *testing.T) {
	e := newEnv(t, 600)
	app := testkeeper.EVMTestApp
	ctx, k, reopen := proofCustodyDisk(t, e.ctx)
	cdc := app.AppCodec()
	assetID := vectorArray(t, e.withdrawal, "asset")
	deposit, err := k.Deposit(ctx, e.payer, e.payerAcc, assetID, [32]byte{0xbe, 0xef}, sdk.NewInt(1000))
	require.NoError(t, err)
	evidence := e.evidence()
	// The real inclusion producer is retired independently of the merge gate.
	require.False(t, k.Retired(ctx))
	response, err := keeper.NewMsgServerImpl(k).RequestWithdrawal(sdk.WrapSDKContext(ctx), &types.MsgRequestWithdrawal{
		Sender: e.payerAcc.String(), Receipt: evidence.Receipt, Proof: evidence.Proof,
		Header: evidence.Header, HeaderSignature: evidence.HeaderSignature,
	})
	require.ErrorIs(t, err, types.ErrModuleRetired)
	require.Nil(t, response)
	depositID, err := types.ParseHash32(deposit.DepositId)
	require.NoError(t, err)
	ctx, k = reopen(ctx)
	for i := 0; i < 2; i++ {
		observed, found := k.GetDeposit(ctx, depositID)
		require.True(t, found)
		require.Equal(t, deposit, observed)
		require.Empty(t, k.ExportGenesis(ctx).Claims)
		require.Empty(t, k.ExportGenesis(ctx).Nullifiers)
	}

	// The module consumer registers NewMsgServerImpl through the real SDK router.
	registry := cdctypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	serviceCodec := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am := layerxcustody.NewAppModule(k)
	am.RegisterServices(module.NewConfigurator(serviceCodec, router, baseapp.NewGRPCQueryRouter()))
	route := router.Handler(&types.MsgSetEmergency{})
	require.NotNil(t, route)
	for _, enabled := range []bool{true, false} {
		packed, err := serviceCodec.MarshalInterface(&types.MsgSetEmergency{Authority: k.Authority(ctx), Enabled: enabled})
		require.NoError(t, err)
		var decoded sdk.Msg
		require.NoError(t, serviceCodec.UnmarshalInterface(packed, &decoded))
		result, err := route(ctx, decoded)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, enabled, k.GetEmergency(ctx), "registered service must read the current keeper")
	}
	before := k.ExportGenesis(ctx)
	_, err = route(ctx, &types.MsgSetEmergency{Authority: e.payerAcc.String(), Enabled: true})
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Equal(t, before, k.ExportGenesis(ctx))

	// Governance consumes a producer-packed proposal after a real codec crossing.
	handler := layerxcustody.NewProposalHandler(k)
	governance := govtypes.NewRouter().AddRoute(types.RouterKey, handler)
	for _, enabled := range []bool{true, false} {
		proposal, err := types.NewCustodyProposal("Custody proof", "Observe each message through governance",
			&types.MsgSetEmergency{Authority: types.GovernanceAuthority(), Enabled: enabled})
		require.NoError(t, err)
		packed, err := cdc.MarshalInterface(proposal)
		require.NoError(t, err)
		var content govtypes.Content
		require.NoError(t, cdc.UnmarshalInterface(packed, &content))
		require.NoError(t, governance.GetRoute(content.ProposalRoute())(ctx, content))
		require.Equal(t, enabled, k.GetEmergency(ctx))
		// Corrupt the actual producer's wire bytes; the decoder must refuse them.
		var malformed govtypes.Content
		require.Error(t, cdc.UnmarshalInterface(packed[:len(packed)-1], &malformed))
	}
	var nilProposal *types.CustodyProposal
	before = k.ExportGenesis(ctx)
	require.ErrorIs(t, handler(ctx, nilProposal), govtypes.ErrInvalidProposalContent)
	require.Equal(t, before, k.ExportGenesis(ctx))

	// The anchor has no cache: change a producer-owned field between reads.
	anchor := k.Anchor()
	batch := vectorNumber(t, e.withdrawal, "batch_number")
	checkpoint, found := k.GetCheckpoint(ctx, batch)
	require.True(t, found)
	for i := 0; i < 2; i++ {
		root, found := anchor.FinalizedStateRoot(ctx, batch)
		require.True(t, found)
		require.Equal(t, vectorArray(t, e.withdrawal, "header_state_root"), root)
		authorization, found := anchor.SequencerAuthorization(ctx, batch)
		require.True(t, found)
		require.Equal(t, vectorArray(t, e.withdrawal, "public_key"), authorization.PublicKey)
	}
	malformedCheckpoint := checkpoint
	malformedCheckpoint.StateRoot = "malformed"
	store := ctx.KVStore(app.GetKey(types.StoreKey))
	store.Set(types.CheckpointKey(batch), cdc.MustMarshal(&malformedCheckpoint))
	_, found = anchor.FinalizedStateRoot(ctx, batch)
	require.False(t, found, "the existing anchor must decode again instead of returning a cached root")
	store.Set(types.CheckpointKey(batch), cdc.MustMarshal(&checkpoint))
	root, found := anchor.FinalizedStateRoot(ctx, batch)
	require.True(t, found)
	require.Equal(t, vectorArray(t, e.withdrawal, "header_state_root"), root)

	// Choose an unblocked release recipient through the real authority service.
	releaseParams := k.GetParams(ctx)
	releaseParams.Authority = e.payerAcc.String()
	_, err = keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
		Authority: k.Authority(ctx), Params: releaseParams,
	})
	require.NoError(t, err)
	require.Equal(t, releaseParams.Authority, k.Authority(ctx))

	// A real scheduled merge releases the real deposit balance and emits the
	// typed result consumed by the SDK event decoder and module genesis export.
	plan := upgradetypes.Plan{Name: types.MergePlanUpgrade, Height: 20}
	require.NoError(t, app.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	before = k.ExportGenesis(ctx)
	require.ErrorIs(t, k.ReleaseRetired(ctx), types.ErrModuleRetired)
	require.Equal(t, before, k.ExportGenesis(ctx))
	ctx = ctx.WithBlockHeight(plan.Height).WithEventManager(sdk.NewEventManager())
	require.False(t, k.Retired(ctx))
	authority, err := sdk.AccAddressFromBech32(k.Authority(ctx))
	require.NoError(t, err)
	balanceBefore := app.BankKeeper.GetBalance(ctx, authority, sdk.MustGetBaseDenom()).Amount
	require.NoError(t, k.ReleaseRetired(ctx))
	var release *types.EventRetiredRelease
	for _, event := range ctx.EventManager().Events() {
		if event.Type != "paxprotocol.paxchain.layerxcustody.EventRetiredRelease" {
			continue
		}
		decoded, err := sdk.ParseTypedEvent(abci.Event(event))
		require.NoError(t, err)
		got, ok := decoded.(*types.EventRetiredRelease)
		require.True(t, ok)
		if got.AssetId == e.withdrawal.Fields["asset"] {
			release = got
		}
	}
	require.Equal(t, &types.EventRetiredRelease{AssetId: e.withdrawal.Fields["asset"], Recipient: authority.String(),
		Amount: "1000", Denom: sdk.MustGetBaseDenom(), CancelledClaims: 0}, release)
	// The same already-constructed consumers must observe the new done height.
	app.UpgradeKeeper.SetDone(ctx, plan.Name)
	require.True(t, k.Retired(ctx))
	require.ErrorIs(t, handler(ctx, nilProposal), types.ErrModuleRetired)
	_, err = route(ctx, &types.MsgSetEmergency{Authority: k.Authority(ctx), Enabled: true})
	require.ErrorIs(t, err, types.ErrModuleRetired)
	ctx, k = reopen(ctx)
	for i := 0; i < 2; i++ {
		require.True(t, k.Retired(ctx))
		require.ErrorIs(t, handler(ctx, nilProposal), types.ErrModuleRetired)
		require.True(t, app.BankKeeper.GetBalance(ctx, k.ModuleAddress(), sdk.MustGetBaseDenom()).IsZero())
		require.Equal(t, balanceBefore.Add(sdk.NewInt(1000)), app.BankKeeper.GetBalance(ctx, authority, sdk.MustGetBaseDenom()).Amount)
		observed, found := k.GetDeposit(ctx, depositID)
		require.True(t, found)
		require.Equal(t, deposit, observed)
		require.Empty(t, k.ExportGenesis(ctx).Claims)
		require.Empty(t, k.ExportGenesis(ctx).Nullifiers)
		require.Equal(t, types.AssetTotals{AssetId: e.withdrawal.Fields["asset"], Custodied: "0", Released: "1000", Pending: "0"}, k.GetTotals(ctx, assetID))
		exported := layerxcustody.NewAppModule(k).ExportGenesis(ctx, cdc)
		require.NoError(t, layerxcustody.NewAppModuleBasic().ValidateGenesis(cdc, nil, exported))
		var decoded types.GenesisState
		require.NoError(t, cdc.UnmarshalAsJSON(exported, &decoded))
		require.Equal(t, exported, json.RawMessage(cdc.MustMarshalJSON(&decoded)))
		// Truncate the real export at the JSON crossing into the module consumer.
		require.Error(t, layerxcustody.NewAppModuleBasic().ValidateGenesis(cdc, nil, json.RawMessage(exported[:len(exported)-1])))
	}
	before = k.ExportGenesis(ctx)
	require.ErrorIs(t, k.ReleaseRetired(ctx), types.ErrModuleRetired)
	require.Equal(t, before, k.ExportGenesis(ctx))
	message, broken := keeper.SolvencyInvariant(k)(ctx)
	require.False(t, broken, message)
}

func TestProof0x1013LayerxcustodyNilMessages(t *testing.T) {
	e := newEnv(t, 600)
	server := keeper.NewMsgServerImpl(e.k)
	calls := []struct {
		name string
		run  func(context.Context) (any, error)
	}{
		{"UpdateParams", func(ctx context.Context) (any, error) { return server.UpdateParams(ctx, nil) }},
		{"SetAsset", func(ctx context.Context) (any, error) { return server.SetAsset(ctx, nil) }},
		{"RegisterCheckpoint", func(ctx context.Context) (any, error) { return server.RegisterCheckpoint(ctx, nil) }},
		{"SetEmergency", func(ctx context.Context) (any, error) { return server.SetEmergency(ctx, nil) }},
		{"CancelClaim", func(ctx context.Context) (any, error) { return server.CancelClaim(ctx, nil) }},
		{"RequestWithdrawal", func(ctx context.Context) (any, error) { return server.RequestWithdrawal(ctx, nil) }},
		{"FinaliseWithdrawal", func(ctx context.Context) (any, error) { return server.FinaliseWithdrawal(ctx, nil) }},
		{"RequestForcedExit", func(ctx context.Context) (any, error) { return server.RequestForcedExit(ctx, nil) }},
		{"ExecuteForcedExit", func(ctx context.Context) (any, error) { return server.ExecuteForcedExit(ctx, nil) }},
	}
	before := e.k.ExportGenesis(e.ctx)
	for _, retired := range []bool{false, true} {
		if retired {
			testkeeper.EVMTestApp.UpgradeKeeper.SetDone(e.ctx, types.MergePlanUpgrade)
		}
		for _, call := range calls {
			t.Run(call.name+map[bool]string{false: "/active", true: "/retired"}[retired], func(t *testing.T) {
				ctx := e.ctx.WithEventManager(sdk.NewEventManager())
				result, err := call.run(sdk.WrapSDKContext(ctx))
				if retired {
					require.ErrorIs(t, err, types.ErrModuleRetired)
				} else {
					require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				}
				require.Nil(t, result)
				require.Equal(t, before, e.k.ExportGenesis(ctx))
				require.Empty(t, ctx.EventManager().Events())
			})
		}
	}
}
