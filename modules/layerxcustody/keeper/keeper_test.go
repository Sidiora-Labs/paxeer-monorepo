package keeper_test

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const tokenDenom = "ufoo"

var (
	tokenPointer = common.HexToAddress("0x00000000000000000000000000000000000f00f0")
	genesisTime  = time.Unix(1_800_000_000, 0).UTC()
)

// env is the real application: the custody keeper wired to the real bank,
// account and EVM keepers, on a branch of the test app's state.
type env struct {
	t          *testing.T
	ctx        sdk.Context
	k          *keeper.Keeper
	withdrawal testvectors.Vector
	exit       testvectors.Vector
	payer      common.Address
	payerAcc   sdk.AccAddress
}

func vectorArray(t *testing.T, v testvectors.Vector, key string) [32]byte {
	t.Helper()
	out, err := v.Array32(key)
	require.NoError(t, err)
	return out
}

func vectorBytes(t *testing.T, v testvectors.Vector, key string) []byte {
	t.Helper()
	out, err := v.Bytes(key)
	require.NoError(t, err)
	return out
}

func vectorNumber(t *testing.T, v testvectors.Vector, key string) uint64 {
	t.Helper()
	out, err := v.Uint64(key)
	require.NoError(t, err)
	return out
}

func newEnv(t *testing.T, withdrawalDelay uint64) *env {
	t.Helper()
	e := newEnvWithoutCheckpoint(t, withdrawalDelay)
	require.NoError(t, e.k.RegisterCheckpoint(e.ctx, vectorNumber(t, e.withdrawal, "batch_number"),
		vectorArray(t, e.withdrawal, "header_state_root"), vectorArray(t, e.withdrawal, "header_receipt_root")))
	return e
}

func custodyTestContext(t *testing.T, parent sdk.Context) sdk.Context {
	t.Helper()
	upgrade := testkeeper.EVMTestApp.UpgradeKeeper
	require.Zero(t, upgrade.GetDoneHeight(parent, types.MergePlanUpgrade))
	plan, scheduled := upgrade.GetUpgradePlan(parent)
	t.Cleanup(func() {
		require.Zero(t, upgrade.GetDoneHeight(parent, types.MergePlanUpgrade), "custody test retired its parent context")
		got, found := upgrade.GetUpgradePlan(parent)
		require.Equal(t, scheduled, found, "custody test changed its parent's upgrade schedule")
		require.Equal(t, plan, got)
	})
	ctx, _ := parent.CacheContext()
	return ctx
}

func newEnvWithoutCheckpoint(t *testing.T, withdrawalDelay uint64) *env {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx := custodyTestContext(t, app.NewContext(false, tmtypes.Header{}).WithBlockHeight(7).WithBlockTime(genesisTime))
	fixture, err := testvectors.Load()
	require.NoError(t, err)
	e := &env{t: t, ctx: ctx, k: app.LayerXCustodyKeeper, withdrawal: fixture["withdrawal"][0], exit: fixture["exit"][0]}
	// This harness exercises custody against its own authority-registered
	// checkpoints; the app wires the anchor module's reader, which
	// TestWithdrawalReadsAnchorFinalizedCheckpoint covers.
	e.k.SetAnchorReader(nil)
	e.payerAcc, e.payer = testkeeper.MockAddressPair()
	app.EvmKeeper.SetAddressMapping(ctx, e.payerAcc, e.payer)
	for _, denom := range []string{sdk.MustGetBaseDenom(), tokenDenom} {
		coins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(50_000_000)))
		require.NoError(t, app.BankKeeper.MintCoins(ctx, "evm", coins))
		require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", e.payerAcc, coins))
	}
	e.k.InitGenesis(ctx, *types.DefaultGenesis())
	batch := vectorNumber(t, e.withdrawal, "batch_number")
	params := types.DefaultParams()
	params.NetworkId = uint32(vectorNumber(t, e.withdrawal, "network_id")) //nolint:gosec
	params.WithdrawalDelaySeconds = withdrawalDelay
	params.SequencerAuthorizations = []types.SequencerAuthorization{{
		SequencerId: e.withdrawal.Fields["sequencer_id"], PublicKey: e.withdrawal.Fields["public_key"],
		FirstBatchNumber: batch, LastBatchNumber: batch}}
	require.NoError(t, e.k.SetParams(ctx, params))
	require.NoError(t, e.k.SetAsset(ctx, types.AssetMapping{AssetId: e.withdrawal.Fields["asset"],
		Denom: sdk.MustGetBaseDenom(), Enabled: true}))
	require.NoError(t, e.k.SetAsset(ctx, types.AssetMapping{AssetId: e.exit.Fields["asset"], Denom: tokenDenom,
		Pointer: tokenPointer.Hex(), Enabled: true, MinimumDeposit: "10"}))
	return e
}

func (e *env) evidence() keeper.WithdrawalEvidence {
	return keeper.WithdrawalEvidence{Receipt: vectorBytes(e.t, e.withdrawal, "receipt"),
		Proof: vectorBytes(e.t, e.withdrawal, "proof"), Header: vectorBytes(e.t, e.withdrawal, "header"),
		HeaderSignature: vectorBytes(e.t, e.withdrawal, "header_signature")}
}

func (e *env) exitEvidence(batch uint64) keeper.ExitEvidence {
	return keeper.ExitEvidence{Witness: vectorBytes(e.t, e.exit, "witness"), BatchNumber: batch,
		Account: vectorArray(e.t, e.exit, "account"), AssetID: vectorArray(e.t, e.exit, "asset"),
		Recipient:          common.BytesToAddress(vectorBytes(e.t, e.exit, "recipient")),
		RecipientSignature: vectorBytes(e.t, e.exit, "recipient_signature")}
}

func (e *env) seedClaim(kind types.ClaimKind, batch uint64) types.Claim {
	e.t.Helper()
	app := testkeeper.EVMTestApp
	store := e.ctx.KVStore(app.GetKey(types.StoreKey))
	params := e.k.GetParams(e.ctx)
	v := e.withdrawal
	delay := params.WithdrawalDelaySeconds
	if kind == types.ClaimKind_CLAIM_KIND_FORCED_EXIT {
		v, delay = e.exit, params.ForcedExitDelaySeconds
	} else {
		require.Equal(e.t, types.ClaimKind_CLAIM_KIND_WITHDRAWAL, kind)
		require.Equal(e.t, vectorNumber(e.t, v, "batch_number"), batch)
	}
	account, assetID := vectorArray(e.t, v, "account"), vectorArray(e.t, v, "asset")
	recipient := common.BytesToAddress(vectorBytes(e.t, v, "recipient"))
	var anchor, withdrawalID, nullifier, claimID [32]byte
	var amount sdk.Int
	if kind == types.ClaimKind_CLAIM_KIND_FORCED_EXIT {
		var found bool
		anchor, found = e.k.Anchor().FinalizedStateRoot(e.ctx, batch)
		require.True(e.t, found)
		require.Equal(e.t, vectorArray(e.t, v, "state_root"), anchor)
		withdrawalID = types.ExitWithdrawalID(params.NetworkId, account, assetID, anchor)
		proven := codec.U128{Lo: vectorNumber(e.t, v, "balance")}
		nullifier = types.WithdrawalNullifier(params.NetworkId, withdrawalID, account, assetID, proven, anchor)
		claimID = types.ExitClaimID(app.EvmKeeper.ChainID(e.ctx), nullifier)
		amount = sdk.NewIntFromBigInt(types.AmountInt(proven))
		require.False(e.t, store.Has(types.ConsumedKey(account, assetID, anchor)))
		exited := sdk.ZeroInt()
		if bz := store.Get(types.ExitedTotalKey(account, assetID)); bz != nil {
			var err error
			exited, err = types.ParseAmount(string(bz))
			require.NoError(e.t, err)
		}
		amount = amount.Sub(exited)
		require.True(e.t, amount.IsPositive())
		if exitBatch, frozen := e.k.ExitCheckpoint(e.ctx); frozen {
			require.Equal(e.t, exitBatch, batch)
		}
		store.Set(types.ConsumedKey(account, assetID, anchor), []byte{1})
		store.Set(types.ExitedTotalKey(account, assetID), []byte(exited.Add(amount).String()))
		store.Set(types.ExitCheckpointKey, binary.BigEndian.AppendUint64(nil, batch))
	} else {
		anchor, withdrawalID = vectorArray(e.t, v, "anchor"), vectorArray(e.t, v, "withdrawal_id")
		nullifier = vectorArray(e.t, v, "nullifier")
		claimID = types.WithdrawalClaimID(app.EvmKeeper.ChainID(e.ctx), nullifier, recipient)
		var err error
		amount, err = types.ParseAmount(v.Fields["amount"])
		require.NoError(e.t, err)
	}
	asset, found := e.k.GetAsset(e.ctx, assetID)
	require.True(e.t, found)
	claim := types.Claim{ClaimId: types.Hash32(claimID), Kind: kind, Status: types.ClaimStatus_CLAIM_STATUS_PENDING,
		Nullifier: types.Hash32(nullifier), WithdrawalId: types.Hash32(withdrawalID), Account: types.Hash32(account),
		AssetId: asset.AssetId, Denom: asset.Denom, Recipient: types.Address(recipient), Amount: amount.String(),
		BatchNumber: batch, Anchor: types.Hash32(anchor), AvailableAt: e.ctx.BlockTime().Unix() + int64(delay)}
	require.NoError(e.t, claim.Validate())
	require.False(e.t, store.Has(types.ClaimKey(claimID)))
	require.False(e.t, store.Has(types.NullifierKey(nullifier)))
	require.False(e.t, store.Has(types.WithdrawalIDKey(withdrawalID)))
	store.Set(types.ClaimKey(claimID), app.AppCodec().MustMarshal(&claim))
	record := types.Nullifier{Nullifier: claim.Nullifier, Status: types.NullifierStatus_NULLIFIER_STATUS_RESERVED,
		ClaimId: claim.ClaimId, WithdrawalId: claim.WithdrawalId}
	store.Set(types.NullifierKey(nullifier), app.AppCodec().MustMarshal(&record))
	store.Set(types.WithdrawalIDKey(withdrawalID), []byte{1})
	totals := e.k.GetTotals(e.ctx, assetID)
	pending, err := types.ParseAmount(totals.Pending)
	require.NoError(e.t, err)
	totals.Pending = pending.Add(amount).String()
	store.Set(types.AssetTotalsKey(assetID), app.AppCodec().MustMarshal(&totals))
	return claim
}

func (e *env) payStoredWithdrawal(ctx sdk.Context, claim types.Claim) keeper.ClaimResult {
	e.t.Helper()
	app := testkeeper.EVMTestApp
	require.Equal(e.t, types.ClaimKind_CLAIM_KIND_WITHDRAWAL, claim.Kind)
	require.Equal(e.t, types.ClaimStatus_CLAIM_STATUS_PENDING, claim.Status)
	require.GreaterOrEqual(e.t, ctx.BlockTime().Unix(), claim.AvailableAt)
	claimID, err := types.ParseHash32(claim.ClaimId)
	require.NoError(e.t, err)
	stored, found := e.k.GetClaim(ctx, claimID)
	require.True(e.t, found)
	require.Equal(e.t, claim, stored)
	assetID, err := types.ParseHash32(claim.AssetId)
	require.NoError(e.t, err)
	asset, found := e.k.GetAsset(ctx, assetID)
	require.True(e.t, found)
	require.True(e.t, asset.Enabled)
	require.False(e.t, asset.Paused)
	require.Equal(e.t, claim.Denom, asset.Denom)
	amount, err := types.ParseAmount(claim.Amount)
	require.NoError(e.t, err)
	totals := e.k.GetTotals(ctx, assetID)
	custodied, err := types.ParseAmount(totals.Custodied)
	require.NoError(e.t, err)
	released, err := types.ParseAmount(totals.Released)
	require.NoError(e.t, err)
	pending, err := types.ParseAmount(totals.Pending)
	require.NoError(e.t, err)
	require.True(e.t, custodied.GTE(amount))
	require.True(e.t, pending.GTE(amount))
	require.True(e.t, app.BankKeeper.GetBalance(ctx, e.k.ModuleAddress(), claim.Denom).Amount.GTE(amount))
	nullifierID, err := types.ParseHash32(claim.Nullifier)
	require.NoError(e.t, err)
	record, found := e.k.GetNullifier(ctx, nullifierID)
	require.True(e.t, found)
	require.Equal(e.t, types.NullifierStatus_NULLIFIER_STATUS_RESERVED, record.Status)
	require.Equal(e.t, claim.ClaimId, record.ClaimId)
	record.Status = types.NullifierStatus_NULLIFIER_STATUS_CONSUMED
	claim.Status = types.ClaimStatus_CLAIM_STATUS_PAID
	totals.Custodied, totals.Released, totals.Pending = custodied.Sub(amount).String(), released.Add(amount).String(), pending.Sub(amount).String()
	store := ctx.KVStore(app.GetKey(types.StoreKey))
	store.Set(types.NullifierKey(nullifierID), app.AppCodec().MustMarshal(&record))
	store.Set(types.ClaimKey(claimID), app.AppCodec().MustMarshal(&claim))
	store.Set(types.AssetTotalsKey(assetID), app.AppCodec().MustMarshal(&totals))
	recipient, err := types.ParseAddress(claim.Recipient)
	require.NoError(e.t, err)
	require.NoError(e.t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName,
		app.EvmKeeper.GetPaxAddressOrDefault(ctx, recipient), sdk.NewCoins(sdk.NewCoin(claim.Denom, amount))))
	require.NoError(e.t, ctx.EventManager().EmitTypedEvent(&types.EventClaimFinalised{ClaimId: claim.ClaimId, Nullifier: claim.Nullifier}))
	require.NoError(e.t, ctx.EventManager().EmitTypedEvent(&types.EventCustodyRelease{ClaimId: claim.ClaimId,
		AssetId: claim.AssetId, Recipient: claim.Recipient, Amount: claim.Amount, Denom: claim.Denom}))
	return keeper.ClaimResult{Claim: claim}
}

func (e *env) deposit(asset string, amount int64) types.Deposit {
	e.t.Helper()
	assetID, err := types.ParseHash32(asset)
	require.NoError(e.t, err)
	deposit, err := e.k.Deposit(e.ctx, e.payer, e.payerAcc, assetID, [32]byte{0xbe, 0xef}, sdk.NewInt(amount))
	require.NoError(e.t, err)
	return deposit
}

func (e *env) balance(account sdk.AccAddress, denom string) sdk.Int {
	return testkeeper.EVMTestApp.BankKeeper.GetBalance(e.ctx, account, denom).Amount
}

func (e *env) recipient() sdk.AccAddress {
	return testkeeper.EVMTestApp.EvmKeeper.GetPaxAddressOrDefault(e.ctx,
		common.BytesToAddress(vectorBytes(e.t, e.withdrawal, "recipient")))
}

func (e *env) solvent() {
	e.t.Helper()
	message, broken := keeper.SolvencyInvariant(e.k)(e.ctx)
	require.False(e.t, broken, message)
}

func TestSetAssetRejectsPointerAliases(t *testing.T) {
	pointer := common.HexToAddress("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	aliases := []struct{ name, value string }{
		{"lower", strings.ToLower(pointer.Hex())},
		{"upper", "0x" + strings.ToUpper(pointer.Hex()[2:])},
		{"EIP55", pointer.Hex()},
	}
	require.NotEqual(t, aliases[0].value, aliases[2].value)
	require.NotEqual(t, aliases[1].value, aliases[2].value)
	for _, original := range aliases {
		for _, alias := range aliases {
			t.Run(original.name+"/"+alias.name, func(t *testing.T) {
				e := newEnv(t, 0)
				firstID := vectorArray(t, e.exit, "asset")
				first, found := e.k.GetAsset(e.ctx, firstID)
				require.True(t, found)
				first.Pointer = original.value
				require.NoError(t, e.k.SetAsset(e.ctx, first))
				first.Pointer = pointer.Hex()
				stored, found := e.k.GetAsset(e.ctx, firstID)
				require.True(t, found)
				require.Equal(t, first, stored)
				sameOwner := first
				sameOwner.Pointer = alias.value
				require.NoError(t, e.k.SetAsset(e.ctx, sameOwner))
				stored, found = e.k.GetAsset(e.ctx, firstID)
				require.True(t, found)
				require.Equal(t, first, stored)
				_, found = e.k.GetAssetByPointer(e.ctx, tokenPointer)
				require.False(t, found, "replacing the first pointer removes its old index")

				secondID := vectorArray(t, e.withdrawal, "asset")
				second, found := e.k.GetAsset(e.ctx, secondID)
				require.True(t, found)
				secondPointer := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
				second.Pointer = secondPointer.Hex()
				require.NoError(t, e.k.SetAsset(e.ctx, second))
				before := e.k.ExportGenesis(e.ctx)
				store := e.ctx.KVStore(testkeeper.EVMTestApp.GetKey(types.StoreKey))
				firstBytes := append([]byte(nil), store.Get(types.AssetKey(firstID))...)
				secondBytes := append([]byte(nil), store.Get(types.AssetKey(secondID))...)
				eventCount := len(e.ctx.EventManager().Events())
				for _, create := range []bool{false, true} {
					candidate := second
					if create {
						candidate.AssetId = types.Hash32([32]byte{0xfa})
						candidate.Denom = "ualias"
					}
					candidate.Pointer = alias.value
					require.NoError(t, candidate.Validate())
					require.ErrorIs(t, e.k.SetAsset(e.ctx, candidate), types.ErrInvalidAsset)
					require.Equal(t, before, e.k.ExportGenesis(e.ctx))
					require.Equal(t, firstBytes, store.Get(types.AssetKey(firstID)))
					require.Equal(t, secondBytes, store.Get(types.AssetKey(secondID)))
					require.Equal(t, firstID[:], store.Get(types.AssetPointerKey(pointer)))
					require.Equal(t, secondID[:], store.Get(types.AssetPointerKey(secondPointer)))
					for _, expected := range []types.AssetMapping{first, second} {
						got, found := e.k.GetAssetByPointer(e.ctx, common.HexToAddress(expected.Pointer))
						require.True(t, found)
						require.Equal(t, expected, got)
						got, found = e.k.GetAssetByDenom(e.ctx, expected.Denom)
						require.True(t, found)
						require.Equal(t, expected, got)
					}
					require.Len(t, e.ctx.EventManager().Events(), eventCount)
				}
				second.Pointer = alias.value
				_, err := keeper.NewMsgServerImpl(e.k).SetAsset(sdk.WrapSDKContext(e.ctx),
					&types.MsgSetAsset{Authority: e.k.Authority(e.ctx), Asset: second})
				require.ErrorIs(t, err, types.ErrInvalidAsset)
				require.Equal(t, before, e.k.ExportGenesis(e.ctx))
				require.Len(t, e.ctx.EventManager().Events(), eventCount)
			})
		}
	}
}

func TestDepositMovesFundsIntoTheModuleAccountAndRecords(t *testing.T) {
	e := newEnv(t, 0)
	before := e.balance(e.payerAcc, sdk.MustGetBaseDenom())
	first := e.deposit(e.withdrawal.Fields["asset"], 1_000)
	second := e.deposit(e.withdrawal.Fields["asset"], 1_000)
	require.Equal(t, sdk.NewInt(2_000), e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom()))
	require.Equal(t, before.SubRaw(2_000), e.balance(e.payerAcc, sdk.MustGetBaseDenom()))
	require.Equal(t, uint64(2), e.k.GetDepositCount(e.ctx))
	require.NotEqual(t, first.DepositId, second.DepositId)
	require.Equal(t, uint64(2), second.Nonce)
	require.Equal(t, e.payer.Hex(), first.Depositor)
	require.Equal(t, types.Hash32([32]byte{0xbe, 0xef}), first.Beneficiary)
	require.Equal(t, int64(7), first.Height)
	byIndex, found := e.k.GetDepositByIndex(e.ctx, 2)
	require.True(t, found)
	require.Equal(t, second, byIndex)
	assetID, _ := types.ParseHash32(e.withdrawal.Fields["asset"])
	expected := types.DepositID(testkeeper.EVMTestApp.EvmKeeper.ChainID(e.ctx), e.payer, assetID, [32]byte{0xbe, 0xef},
		sdk.NewInt(1_000).BigInt(), 1)
	require.Equal(t, types.Hash32(expected), first.DepositId)
	found = false
	for _, event := range e.ctx.EventManager().Events() {
		found = found || event.Type == "paxprotocol.paxchain.layerxcustody.EventCustodyDeposit"
	}
	require.True(t, found, "deposit typed event")
	e.solvent()

	_, err := e.k.Deposit(e.ctx, e.payer, e.payerAcc, assetID, [32]byte{}, sdk.NewInt(5))
	require.ErrorIs(t, err, types.ErrInvalidDeposit)
	_, err = e.k.Deposit(e.ctx, e.payer, e.payerAcc, [32]byte{9}, [32]byte{1}, sdk.NewInt(5))
	require.ErrorIs(t, err, types.ErrUnknownAsset)
	exitAsset, _ := types.ParseHash32(e.exit.Fields["asset"])
	_, err = e.k.Deposit(e.ctx, e.payer, e.payerAcc, exitAsset, [32]byte{1}, sdk.NewInt(9))
	require.ErrorIs(t, err, types.ErrInvalidDeposit)
}

func TestForcedExitClosesDepositsWithAConstantFreezeRead(t *testing.T) {
	e := newEnv(t, 600)
	freezeReadGas := func() uint64 {
		ctx := e.ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
		_, frozen := e.k.ExitCheckpoint(ctx)
		require.False(t, frozen)
		return ctx.GasMeter().GasConsumed()
	}
	empty := freezeReadGas()
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	e.deposit(e.exit.Fields["asset"], 6_000_000)
	e.seedClaim(types.ClaimKind_CLAIM_KIND_WITHDRAWAL, vectorNumber(t, e.withdrawal, "batch_number"))
	require.Equal(t, empty, freezeReadGas(), "the freeze read does not grow with the claim store")

	params := e.k.GetParams(e.ctx)
	params.ForcedExitDelaySeconds = 120
	require.NoError(t, e.k.SetParams(e.ctx, params))
	exitBatch := vectorNumber(t, e.withdrawal, "batch_number") + 1
	require.NoError(t, e.k.RegisterCheckpoint(e.ctx, exitBatch, vectorArray(t, e.exit, "state_root"), [32]byte{7}))
	require.NoError(t, e.k.SetEmergency(e.ctx, true))
	exit := e.seedClaim(types.ClaimKind_CLAIM_KIND_FORCED_EXIT, exitBatch)
	frozenAt, frozen := e.k.ExitCheckpoint(e.ctx)
	require.True(t, frozen)
	require.Equal(t, exitBatch, frozenAt)

	// A deposit now could only be credited after the exit checkpoint, where
	// it could be neither withdrawn nor force-exited.
	assetID := vectorArray(t, e.exit, "asset")
	_, err := e.k.Deposit(e.ctx, e.payer, e.payerAcc, assetID, [32]byte{0xbe, 0xef}, sdk.NewInt(1_000))
	require.ErrorIs(t, err, types.ErrDepositFrozen)
	require.Equal(t, sdk.NewInt(6_000_000), e.balance(e.k.ModuleAddress(), tokenDenom))

	// A cancelled exit took nothing, so it lifts the freeze.
	claimID, err := types.ParseHash32(exit.ClaimId)
	require.NoError(t, err)
	_, err = e.k.CancelClaim(e.ctx, claimID)
	require.NoError(t, err)
	_, frozen = e.k.ExitCheckpoint(e.ctx)
	require.False(t, frozen)
	e.deposit(e.exit.Fields["asset"], 1_000)
	e.solvent()
}

func TestGenesisRoundTrip(t *testing.T) {
	e := newEnv(t, 600)
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	e.deposit(e.exit.Fields["asset"], 6_000_000)
	withdrawal := e.seedClaim(types.ClaimKind_CLAIM_KIND_WITHDRAWAL, vectorNumber(t, e.withdrawal, "batch_number"))
	exitBatch := vectorNumber(t, e.withdrawal, "batch_number") + 1
	require.NoError(t, e.k.RegisterCheckpoint(e.ctx, exitBatch, vectorArray(t, e.exit, "state_root"), [32]byte{7}))
	require.NoError(t, e.k.SetEmergency(e.ctx, true))
	e.seedClaim(types.ClaimKind_CLAIM_KIND_FORCED_EXIT, exitBatch)
	_, err := e.k.ExecuteForcedExit(e.ctx, e.exitEvidence(exitBatch))
	require.NoError(t, err)

	exported := e.k.ExportGenesis(e.ctx)
	require.NoError(t, exported.Validate())
	require.Len(t, exported.Deposits, 2)
	require.Len(t, exported.Claims, 2)
	require.Len(t, exported.Nullifiers, 2)
	require.Len(t, exported.ConsumedBalances, 1)
	require.Len(t, exported.Checkpoints, 2)
	require.True(t, exported.Emergency)

	bz, err := testkeeper.EVMTestApp.AppCodec().MarshalAsJSON(exported)
	require.NoError(t, err)
	var decoded types.GenesisState
	require.NoError(t, testkeeper.EVMTestApp.AppCodec().UnmarshalAsJSON(bz, &decoded))

	// Import into a branch whose custody store is empty but whose bank
	// balances are the exported chain's.
	fresh, _ := e.ctx.CacheContext()
	store := fresh.KVStore(testkeeper.EVMTestApp.GetKey(types.StoreKey))
	iterator := store.Iterator(nil, nil)
	var keys [][]byte
	for ; iterator.Valid(); iterator.Next() {
		keys = append(keys, append([]byte(nil), iterator.Key()...))
	}
	require.NoError(t, iterator.Close())
	for _, key := range keys {
		store.Delete(key)
	}
	require.Equal(t, uint64(0), e.k.GetDepositCount(fresh))
	e.k.InitGenesis(fresh, decoded)
	require.Equal(t, exported, e.k.ExportGenesis(fresh))
	message, broken := keeper.SolvencyInvariant(e.k)(fresh)
	require.False(t, broken, message)

	// The paid exit stays paid and the pending withdrawal stays pending.
	_, err = e.k.ExecuteForcedExit(fresh, e.exitEvidence(exitBatch))
	require.ErrorIs(t, err, types.ErrClaimNotPending)
	result := e.payStoredWithdrawal(fresh.WithBlockTime(genesisTime.Add(time.Hour)), withdrawal)
	require.False(t, result.Queued)

	reopened := *exported
	reopened.Nullifiers = append([]types.Nullifier(nil), exported.Nullifiers...)
	for i := range reopened.Nullifiers {
		reopened.Nullifiers[i].Status = types.NullifierStatus_NULLIFIER_STATUS_RESERVED
	}
	require.ErrorIs(t, reopened.Validate(), types.ErrInvalidGenesis)
}

func TestSolvencyInvariantDetectsMissingFunds(t *testing.T) {
	e := newEnv(t, 0)
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	e.solvent()
	drained, _ := e.ctx.CacheContext()
	require.NoError(t, testkeeper.EVMTestApp.BankKeeper.SendCoinsFromModuleToAccount(drained, types.ModuleName, e.payerAcc,
		sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), sdk.NewInt(1)))))
	_, broken := keeper.SolvencyInvariant(e.k)(drained)
	require.True(t, broken)
}

func TestAuthorityGatesAdministration(t *testing.T) {
	e := newEnv(t, 0)
	server := keeper.NewMsgServerImpl(e.k)
	goCtx := sdk.WrapSDKContext(e.ctx)
	_, err := server.SetEmergency(goCtx, &types.MsgSetEmergency{Authority: e.payerAcc.String(), Enabled: true})
	require.Error(t, err)
	require.False(t, e.k.GetEmergency(e.ctx))
	_, err = server.RegisterCheckpoint(goCtx, &types.MsgRegisterCheckpoint{Authority: e.payerAcc.String(), BatchNumber: 99,
		StateRoot: types.Hash32([32]byte{1}), ReceiptRoot: types.Hash32([32]byte{2})})
	require.Error(t, err)
	_, err = server.SetEmergency(goCtx, &types.MsgSetEmergency{Authority: e.k.Authority(e.ctx), Enabled: true})
	require.NoError(t, err)
	require.True(t, e.k.GetEmergency(e.ctx))
	batch := vectorNumber(t, e.withdrawal, "batch_number")
	_, err = server.RegisterCheckpoint(goCtx, &types.MsgRegisterCheckpoint{Authority: e.k.Authority(e.ctx), BatchNumber: batch,
		StateRoot: types.Hash32([32]byte{1}), ReceiptRoot: types.Hash32([32]byte{2})})
	require.ErrorIs(t, err, types.ErrInvalidCheckpoint, "a finalized checkpoint is immutable")
}
