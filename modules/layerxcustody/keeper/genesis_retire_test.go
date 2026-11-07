package keeper_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	dbm "github.com/tendermint/tm-db"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	anchorkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenesisRoundTripAfterRetiredRelease(t *testing.T) {
	e, withdrawal, plan := prepareCustodyMerge(t)
	params := e.k.GetParams(e.ctx)
	params.Authority = e.payerAcc.String()
	params.ForcedExitDelaySeconds = 600
	require.NoError(t, e.k.SetParams(e.ctx, params))
	e.deposit(e.exit.Fields["asset"], 5_998_000)
	exitBatch := vectorNumber(t, e.withdrawal, "batch_number") + 1
	require.NoError(t, e.k.RegisterCheckpoint(e.ctx, exitBatch, vectorArray(t, e.exit, "state_root"), [32]byte{7}))
	require.NoError(t, e.k.SetEmergency(e.ctx, true))
	exit := e.seedClaim(types.ClaimKind_CLAIM_KIND_FORCED_EXIT, exitBatch)

	claims := []types.Claim{withdrawal, exit}
	nullifiers := make(map[string]types.Nullifier)
	totals := make(map[string]types.AssetTotals)
	for _, claim := range claims {
		require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PENDING, claim.Status)
		nullifierID, err := types.ParseHash32(claim.Nullifier)
		require.NoError(t, err)
		nullifier, found := e.k.GetNullifier(e.ctx, nullifierID)
		require.True(t, found)
		require.Equal(t, types.NullifierStatus_NULLIFIER_STATUS_RESERVED, nullifier.Status)
		nullifiers[claim.Nullifier] = nullifier
		assetID, err := types.ParseHash32(claim.AssetId)
		require.NoError(t, err)
		totals[claim.AssetId] = e.k.GetTotals(e.ctx, assetID)
	}
	root := registerCustodyRecoveryDepositRoot(t, e)
	e.solvent()
	e.ctx = e.ctx.WithBlockHeight(plan.Height)
	testkeeper.EVMTestApp.UpgradeKeeper.ApplyUpgrade(e.ctx, plan)
	require.Equal(t, plan.Height, testkeeper.EVMTestApp.UpgradeKeeper.GetDoneHeight(e.ctx, plan.Name))
	require.True(t, e.k.Retired(e.ctx))

	exported := e.k.ExportGenesis(e.ctx)
	require.NoError(t, exported.Validate())
	require.Len(t, exported.Claims, 2)
	require.Len(t, exported.Nullifiers, 2)
	require.Len(t, exported.Totals, 2)
	require.Equal(t, []types.DepositRootRegistration{root}, exported.DepositRoots)
	// Commit a snapshot of the actual merge result in a standalone application.
	sourceDir := t.TempDir()
	source := openCustodyRecoveryApp(t, sourceDir, true)
	t.Cleanup(func() {
		if source != nil {
			require.NoError(t, source.Close())
		}
	})
	commitCustodyRecoveryApp(t, source, e.ctx)
	appExport, err := source.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	require.Equal(t, plan.Height+1, appExport.Height)
	sourceCtx := source.NewUncachedContext(false, e.ctx.BlockHeader())
	lifecycle, err := source.LayerXCustodyKeeper.ExportGenesisWithRetirement(sourceCtx)
	require.NoError(t, err)
	require.Equal(t, plan.Height, lifecycle.RetiredAt)
	require.Equal(t, *exported, lifecycle.State)
	lifecycleWire, err := json.Marshal(lifecycle)
	require.NoError(t, err)
	var appState map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(appExport.AppState, &appState))
	var appCustody types.GenesisState
	require.NoError(t, source.AppCodec().UnmarshalAsJSON(appState[types.ModuleName], &appCustody))
	require.Equal(t, *exported, appCustody)
	payerBalance := source.BankKeeper.GetAllBalances(sourceCtx, e.payerAcc)
	authority, err := sdk.AccAddressFromBech32(e.k.Authority(e.ctx))
	require.NoError(t, err)
	authorityBalance := source.BankKeeper.GetAllBalances(sourceCtx, authority)
	require.NoError(t, source.Close())
	source = nil
	source = openCustodyRecoveryApp(t, sourceDir, false)
	sourceCtx = source.NewUncachedContext(false, e.ctx.BlockHeader())
	require.Equal(t, plan.Height, source.LastBlockHeight())
	require.Equal(t, plan.Height, source.UpgradeKeeper.GetDoneHeight(sourceCtx, types.MergePlanUpgrade))
	assertCustodyRecoveryRefusals(t, source, sourceCtx, e, lifecycle, payerBalance, authorityBalance)

	// The legacy module adapter carries history. Restore its separately exported
	// lifecycle through the validated initialization API, never SetDone.
	consumerDir := t.TempDir()
	second := openCustodyRecoveryApp(t, consumerDir, false)
	t.Cleanup(func() {
		if second != nil {
			require.NoError(t, second.Close())
		}
	})
	_, err = second.InitChain(context.Background(), &abci.RequestInitChain{
		Time: e.ctx.BlockTime(), ChainId: "pax-test", InitialHeight: appExport.Height,
		ConsensusParams: appExport.ConsensusParams, AppStateBytes: appExport.AppState,
	})
	require.NoError(t, err)
	fresh := second.NewContext(false, tmtypes.Header{ChainID: "pax-test", Height: appExport.Height, Time: e.ctx.BlockTime()})
	imported := second.LayerXCustodyKeeper
	require.Zero(t, second.UpgradeKeeper.GetDoneHeight(fresh, types.MergePlanUpgrade))
	require.False(t, imported.Retired(fresh), "legacy adapter still needs the recorded lifecycle wiring")
	require.Equal(t, exported, imported.ExportGenesis(fresh))
	var decoded keeper.RetirementGenesis
	require.NoError(t, json.Unmarshal(lifecycleWire, &decoded))
	require.NoError(t, decoded.Validate())
	require.NoError(t, imported.InitGenesisWithRetirement(fresh, decoded))
	require.True(t, imported.Retired(fresh))
	assert.Equal(t, exported, imported.ExportGenesis(fresh))
	assertCustodyRecoveryRefusals(t, second, fresh, e, lifecycle, payerBalance, authorityBalance)
	require.NoError(t, second.SetSealedStateToCommit(fresh, fresh.MultiStore().(sdk.CacheMultiStore)))
	_, err = second.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, second.Close())
	second = nil
	second = openCustodyRecoveryApp(t, consumerDir, false)
	fresh = second.NewUncachedContext(false, tmtypes.Header{ChainID: "pax-test", Height: 0, Time: e.ctx.BlockTime()})
	imported = second.LayerXCustodyKeeper
	require.Equal(t, appExport.Height, second.LastBlockHeight())
	require.Zero(t, second.UpgradeKeeper.GetDoneHeight(fresh, types.MergePlanUpgrade))
	assertCustodyRecoveryRefusals(t, second, fresh, e, lifecycle, payerBalance, authorityBalance)

	for _, original := range claims {
		claimID, err := types.ParseHash32(original.ClaimId)
		require.NoError(t, err)
		claim, found := imported.GetClaim(fresh, claimID)
		require.True(t, found)
		original.Status = types.ClaimStatus_CLAIM_STATUS_CANCELLED
		assert.Equal(t, original, claim)
		sourceClaim, found := e.k.GetClaim(e.ctx, claimID)
		require.True(t, found)
		assert.Equal(t, sourceClaim, claim)

		nullifierID, err := types.ParseHash32(original.Nullifier)
		require.NoError(t, err)
		nullifier, found := imported.GetNullifier(fresh, nullifierID)
		require.True(t, found)
		expectedNullifier := nullifiers[original.Nullifier]
		expectedNullifier.Status = types.NullifierStatus_NULLIFIER_STATUS_CANCELLED
		assert.Equal(t, expectedNullifier, nullifier)
		sourceNullifier, found := e.k.GetNullifier(e.ctx, nullifierID)
		require.True(t, found)
		assert.Equal(t, sourceNullifier, nullifier)
		withdrawalID, err := types.ParseHash32(original.WithdrawalId)
		require.NoError(t, err)
		assert.True(t, imported.WithdrawalIDUsed(fresh, withdrawalID))

		assetID, err := types.ParseHash32(original.AssetId)
		require.NoError(t, err)
		before := totals[original.AssetId]
		custodied, ok := sdk.NewIntFromString(before.Custodied)
		require.True(t, ok)
		released, ok := sdk.NewIntFromString(before.Released)
		require.True(t, ok)
		assert.Equal(t, types.AssetTotals{AssetId: original.AssetId, Custodied: "0",
			Released: released.Add(custodied).String(), Pending: "0"}, imported.GetTotals(fresh, assetID))
		assert.Equal(t, e.k.GetTotals(e.ctx, assetID), imported.GetTotals(fresh, assetID))
		assert.True(t, second.BankKeeper.GetBalance(fresh, imported.ModuleAddress(), original.Denom).IsZero())
	}
	_, frozen := imported.ExitCheckpoint(fresh)
	assert.False(t, frozen)
	message, broken := keeper.SolvencyInvariant(imported)(fresh)
	assert.False(t, broken, message)
}

func registerCustodyRecoveryDepositRoot(t *testing.T, e *env) types.DepositRootRegistration {
	t.Helper()
	app := testkeeper.EVMTestApp
	anchor := app.LayerXAnchorKeeper
	headerBytes := vectorBytes(t, e.withdrawal, "header")
	header, err := codec.DecodeBatchHeader(headerBytes)
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	params := e.k.GetParams(e.ctx)
	params.DepositRootAuthority = hex.EncodeToString(public)
	require.NoError(t, e.k.SetParams(e.ctx, params))
	genesis := anchortypes.DefaultGenesis()
	genesis.Params = anchortypes.DefaultParams(e.payerAcc.String())
	genesis.Params.NetworkID = header.NetworkID
	genesis.Params.PaxeerChainID = anchorPaxeerChainID
	genesis.Params.Threshold = 1
	genesis.Anchor = anchortypes.Anchor{Set: true, BatchNumber: header.BatchNumber - 1,
		LastSequence: header.FirstSequence - 1, StateRoot: anchortypes.Hash32(header.PreviousStateRoot)}
	genesis.Sequencers = []anchortypes.SequencerAuthorization{{
		SequencerID:      anchortypes.Hash32(header.SequencerID),
		PublicKey:        anchortypes.Hash32(vectorArray(t, e.withdrawal, "public_key")),
		FirstBatchNumber: header.BatchNumber, LastBatchNumber: header.BatchNumber,
	}}
	require.NoError(t, genesis.Validate())
	anchor.InitGenesis(e.ctx, *genesis)
	authority, _ := testkeeper.MockAddressPair()
	anchorParams := anchor.GetParams(e.ctx)
	anchorParams.Authority = authority.String()
	anchorParams.PermissionlessRegistration = false
	_, err = anchorkeeper.NewMsgServerImplWithUpgrades(anchor, app.UpgradeKeeper).UpdateParams(
		sdk.WrapSDKContext(e.ctx), &anchortypes.MsgUpdateParams{
			Authority: e.payerAcc.String(), Params: anchorParams,
		})
	require.NoError(t, err)
	require.Equal(t, anchorParams, anchor.GetParams(e.ctx))
	guarantorID := [32]byte{0x62}
	_, signer := guarantorCertificate(t, headerBytes, guarantorID, [20]byte(genesis.Params.SettlementContract))
	bond := sdk.NewCoins(sdk.NewCoin(genesis.Params.BondDenom, genesis.Params.MinBond))
	require.NoError(t, app.BankKeeper.MintCoins(e.ctx, "evm", bond))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(e.ctx, "evm", e.payerAcc, bond))
	guarantor, err := anchor.RegisterGuarantor(e.ctx, e.payerAcc, guarantorID, signer, genesis.Params.MinBond)
	require.NoError(t, err)
	require.Equal(t, anchortypes.GuarantorPending, guarantor.Status)
	require.NoError(t, anchor.ActivateGuarantor(e.ctx, authority, guarantorID))
	guarantor, found := anchor.GetGuarantor(e.ctx, guarantorID)
	require.True(t, found)
	require.Equal(t, anchortypes.GuarantorActive, guarantor.Status)
	require.Equal(t, genesis.Params.MinBond, guarantor.Bond)
	genesis = anchor.ExportGenesis(e.ctx)
	checkpointID := codec.CheckpointHash(headerBytes, nil)
	genesis.Checkpoints = []anchortypes.Checkpoint{{
		BatchNumber: header.BatchNumber, CheckpointID: anchortypes.Hash32(checkpointID),
		HeaderDigest: anchortypes.Hash32(codec.BatchHeaderDigest(headerBytes)), ProtocolVersion: header.ProtocolVersion,
		NetworkID: header.NetworkID, Epoch: header.Epoch, FirstSequence: header.FirstSequence,
		LastSequence: header.LastSequence, PreviousStateRoot: anchortypes.Hash32(header.PreviousStateRoot),
		StateRoot: anchortypes.Hash32(header.ResultingStateRoot), ReceiptRoot: anchortypes.Hash32(header.ReceiptMerkleRoot),
		DataAvailabilityRoot: anchortypes.Hash32(header.DataAvailabilityRoot), SequencerID: anchortypes.Hash32(header.SequencerID),
		TimestampMs: header.TimestampMs, Status: anchortypes.CheckpointFinal, DeclaredThreshold: 1,
		Guarantors: []anchortypes.Hash32{anchortypes.Hash32(guarantorID)}, AvailabilityMask: codec.AvailabilityAll,
		Submitter: e.payerAcc.String(), SubmittedHeight: e.ctx.BlockHeight(), SubmittedTime: e.ctx.BlockTime().Unix(),
		FinalizedHeight: e.ctx.BlockHeight(), FinalizedTime: e.ctx.BlockTime().Unix(),
	}}
	genesis.HasLatestFinalized = true
	genesis.LatestFinalized = header.BatchNumber
	require.NoError(t, genesis.Validate())
	anchor.InitGenesis(e.ctx, *genesis)
	e.k.SetAnchorReader(anchorkeeper.NewCustodyAnchor(anchor))
	defer e.k.SetAnchorReader(nil)
	registration := depositRootRegistration(checkpointID, header.ResultingStateRoot, [32]byte{0xd1}, [32]byte{0xc1},
		header.NetworkID, header.ProtocolVersion)
	root, err := e.k.RegisterDepositRoot(e.ctx, e.payerAcc, registration, ed25519.Sign(private, registration), [][32]byte{{1}, {2}})
	require.NoError(t, err)
	stored, found := e.k.GetDepositRoot(e.ctx, checkpointID)
	require.True(t, found)
	require.Equal(t, root, stored)
	return root
}

func openCustodyRecoveryApp(t *testing.T, dir string, initialize bool) *paxapp.App {
	t.Helper()
	db, err := dbm.NewGoLevelDB("custody-recovery", dir)
	require.NoError(t, err)
	app := paxapp.New(db, nil, true, map[int64]bool{}, dir, 1, true,
		tmcfg.TestConfig().SetRoot(dir), paxapp.MakeEncodingConfig(), nil, paxapp.TestAppOpts{}, paxapp.EmptyWasmOpts, nil)
	if initialize {
		genesis, err := json.Marshal(paxapp.NewDefaultGenesisState(app.AppCodec()))
		require.NoError(t, err)
		_, err = app.InitChain(context.Background(), &abci.RequestInitChain{
			Time: genesisTime, ChainId: "pax-test", ConsensusParams: paxapp.DefaultConsensusParams,
			AppStateBytes: genesis,
		})
		require.NoError(t, err)
	}
	return app
}

func commitCustodyRecoveryApp(t *testing.T, app *paxapp.App, initial sdk.Context) {
	t.Helper()
	// Flatten the fixture's isolated cache branches into the app's own store keys.
	ms := app.CommitMultiStore().CacheMultiStore()
	ctx := sdk.NewContext(ms, initial.BlockHeader(), false)
	sourceKeys := make(map[string]storetypes.StoreKey)
	for _, key := range initial.MultiStore().StoreKeys() {
		sourceKeys[key.Name()] = key
	}
	for _, key := range ms.StoreKeys() {
		if _, persistent := key.(*storetypes.KVStoreKey); !persistent {
			continue
		}
		sourceKey, found := sourceKeys[key.Name()]
		require.True(t, found, key.Name())
		iterator := initial.KVStore(sourceKey).Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			ctx.KVStore(key).Set(append([]byte(nil), iterator.Key()...), append([]byte{}, iterator.Value()...))
		}
		require.NoError(t, iterator.Close())
	}
	require.NoError(t, app.CommitMultiStore().SetInitialVersion(initial.BlockHeight()))
	require.NoError(t, app.SetSealedStateToCommit(ctx, ms))
	_, err := app.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, initial.BlockHeight(), app.LastBlockHeight())
}

func assertCustodyRecoveryRefusals(t *testing.T, app *paxapp.App, ctx sdk.Context, e *env,
	expected *keeper.RetirementGenesis, payerBalance, authorityBalance sdk.Coins) {
	t.Helper()
	k := app.LayerXCustodyKeeper
	require.True(t, k.Retired(ctx))
	require.Equal(t, expected.State, *k.ExportGenesis(ctx))
	recovered, err := k.ExportGenesisWithRetirement(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, recovered)
	authority, err := sdk.AccAddressFromBech32(k.Authority(ctx))
	require.NoError(t, err)
	require.Equal(t, payerBalance, app.BankKeeper.GetAllBalances(ctx, e.payerAcc))
	require.Equal(t, authorityBalance, app.BankKeeper.GetAllBalances(ctx, authority))
	require.Empty(t, app.BankKeeper.GetAllBalances(ctx, k.ModuleAddress()))
	assetID := vectorArray(t, e.withdrawal, "asset")
	asset, found := k.GetAsset(ctx, assetID)
	require.True(t, found)
	server := keeper.NewMsgServerImpl(k)
	for _, operation := range []struct {
		name string
		run  func(sdk.Context) error
	}{
		{"deposit", func(ctx sdk.Context) error {
			_, err := k.Deposit(ctx, e.payer, e.payerAcc, assetID, [32]byte{0xbe, 0xef}, sdk.NewInt(10))
			return err
		}},
		{"params", func(ctx sdk.Context) error {
			params := expected.State.Params
			params.WithdrawalDelaySeconds++
			_, err := server.UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{Authority: k.Authority(ctx), Params: params})
			return err
		}},
		{"asset", func(ctx sdk.Context) error {
			asset.Enabled = !asset.Enabled
			_, err := server.SetAsset(sdk.WrapSDKContext(ctx), &types.MsgSetAsset{Authority: k.Authority(ctx), Asset: asset})
			return err
		}},
		{"emergency", func(ctx sdk.Context) error {
			_, err := server.SetEmergency(sdk.WrapSDKContext(ctx), &types.MsgSetEmergency{Authority: k.Authority(ctx), Enabled: !expected.State.Emergency})
			return err
		}},
		{"release", func(ctx sdk.Context) error { return k.ReleaseRetired(ctx) }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			callCtx := ctx.WithEventManager(sdk.NewEventManager())
			require.ErrorIs(t, operation.run(callCtx), types.ErrModuleRetired)
			require.Equal(t, expected.State, *k.ExportGenesis(callCtx))
			require.Equal(t, payerBalance, app.BankKeeper.GetAllBalances(callCtx, e.payerAcc))
			require.Equal(t, authorityBalance, app.BankKeeper.GetAllBalances(callCtx, authority))
			require.Empty(t, app.BankKeeper.GetAllBalances(callCtx, k.ModuleAddress()))
			require.Empty(t, callCtx.EventManager().Events())
			after, err := k.ExportGenesisWithRetirement(callCtx)
			require.NoError(t, err)
			require.Equal(t, expected, after)
		})
	}
}
