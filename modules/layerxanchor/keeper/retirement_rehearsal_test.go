package keeper_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type retirementPrecompileKeepers struct {
	utils.Keepers
	anchor keeper.Keeper
}

func (k retirementPrecompileKeepers) AnchorK() utils.AnchorKeeper { return k.anchor }

func newAnchorRetirementSuite(t *testing.T) *suite {
	t.Helper()
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(8).WithBlockTime(time.Unix(1_800_000_000, 0))
	stores := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	keys := make(map[string]storetypes.StoreKey)
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() != types.StoreKey {
			stores[key] = ctx.MultiStore().GetKVStore(key)
			keys[key.Name()] = key
		}
	}
	anchorKey := sdk.NewKVStoreKey(types.StoreKey)
	anchorDB, cacheDB := dbm.NewMemDB(), dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, anchorDB.Close()) })
	t.Cleanup(func() { require.NoError(t, cacheDB.Close()) })
	stores[anchorKey] = dbadapter.Store{DB: anchorDB}
	keys[anchorKey.Name()] = anchorKey
	ctx = ctx.WithMultiStore(cachemulti.NewStore(cacheDB, stores, keys, nil, nil, nil, 0))
	a.LayerXAnchorKeeper = keeper.NewKeeper(anchorKey, a.AccountKeeper, a.BankKeeper)
	return initializeSuite(t, a, ctx, nil)
}

func TestAnchorRetirementRehearsal(t *testing.T) {
	s := newAnchorRetirementSuite(t)
	const mergeHeight, deleteHeight = int64(20), int64(40)
	require.Equal(t, "v7.0", types.MergeUpgrade)
	require.Equal(t, "v7.0-anchor-store-delete", types.StoreDeleteUpgrade)
	require.NoError(t, s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, upgradetypes.Plan{
		Name: types.MergeUpgrade, Height: mergeHeight,
	}))
	initialBalances := make([]sdk.Int, len(s.operators))
	for i, operator := range s.operators {
		initialBalances[i] = s.balance(operator)
	}
	s.bondAll()
	checkpoint, err := s.submit("batch_1_quorum")
	require.NoError(t, err)
	require.Equal(t, types.CheckpointFinal, checkpoint.Status)
	total := sdk.NewInt(int64(len(s.ids)) * bond)
	supply := s.app.BankKeeper.GetSupply(s.ctx, s.k.GetParams(s.ctx).BondDenom)
	p, err := layerxanchor.NewPrecompile(retirementPrecompileKeepers{
		Keepers: s.app.GetPrecompileKeepers(), anchor: s.k,
	})
	require.NoError(t, err)
	db := state.NewDBImpl(s.ctx, &s.app.EvmKeeper, true)
	evm := &vm.EVM{StateDB: db}
	operator := common.BytesToAddress(s.operators[0])
	s.app.EvmKeeper.SetAddressMapping(s.ctx, s.operators[0], operator)
	reads := map[string][]interface{}{
		layerxanchor.LatestFinalizedMethod:      {},
		layerxanchor.CheckpointMethod:           {uint64(1)},
		layerxanchor.FinalizedStateRootMethod:   {uint64(1)},
		layerxanchor.FinalizedReceiptRootMethod: {uint64(1)},
		layerxanchor.GuarantorMethod:            {s.ids[0]},
		layerxanchor.ThresholdMethod:            {},
		layerxanchor.StatusOfMethod:             {uint64(1)},
		layerxanchor.CheckpointBatchMethod:      {[32]byte(checkpoint.CheckpointID)},
		layerxanchor.CheckpointGuarantorsMethod: {uint64(1)},
	}
	v := s.vector("checkpoints", "batch_1_quorum")
	header, err := v.Bytes("header")
	require.NoError(t, err)
	signature, err := v.Bytes("header_signature")
	require.NoError(t, err)
	certificate, err := v.Bytes("certificate")
	require.NoError(t, err)
	authorization := s.k.GetSequencerAuthorizations(s.ctx)[0]
	writes := map[string][]interface{}{
		layerxanchor.SubmitCheckpointMethod:          {header, signature, certificate},
		layerxanchor.SubmitAvailabilityMethod:        {s.attestation("batch_1_attestation_1")},
		layerxanchor.FinalizeMethod:                  {uint64(1)},
		layerxanchor.RegisterGuarantorMethod:         {s.ids[0], common.Address(s.signers[0])},
		layerxanchor.IncreaseBondMethod:              {s.ids[0]},
		layerxanchor.BeginUnbondMethod:               {s.ids[0], big.NewInt(1_000_000)},
		layerxanchor.CompleteUnbondMethod:            {s.ids[0]},
		layerxanchor.SubmitEquivocationMethod:        {s.attestation("batch_1_attestation_0"), s.attestation("batch_1_conflicting_attestation_0")},
		layerxanchor.OpenChallengeMethod:             {uint64(1), types.ChallengeFraud, [32]byte{1}},
		layerxanchor.ResolveChallengeMethod:          {uint64(1), false},
		layerxanchor.ActivateGuarantorMethod:         {s.ids[0]},
		layerxanchor.SetSequencerAuthorizationMethod: {[32]byte(authorization.SequencerID), [32]byte(authorization.PublicKey), uint64(1), uint64(8)},
	}
	require.Len(t, reads, 9)
	require.Len(t, writes, 12)
	for name, method := range p.GetABI().Methods {
		if method.StateMutability == "view" {
			require.Contains(t, reads, name)
		} else {
			require.Contains(t, writes, name)
		}
	}
	unbonding := sdk.ZeroInt()
	for _, height := range []int64{8, mergeHeight - 1, mergeHeight, mergeHeight + 1, deleteHeight - 1, deleteHeight, deleteHeight + 1} {
		t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
			s.ctx = s.ctx.WithBlockHeight(height)
			db.WithCtx(s.ctx)
			if height == mergeHeight {
				returned, err := s.k.RetireAtMerge(s.ctx)
				require.NoError(t, err)
				require.Equal(t, total, returned)
				s.app.UpgradeKeeper.SetDone(s.ctx, types.MergeUpgrade)
				require.NoError(t, s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, upgradetypes.Plan{
					Name: types.StoreDeleteUpgrade, Height: deleteHeight,
				}))
			}
			if height == deleteHeight {
				s.app.UpgradeKeeper.SetDone(s.ctx, types.StoreDeleteUpgrade)
			}
			require.Equal(t, height >= mergeHeight, keeper.MergeRetired(s.ctx, &s.app.UpgradeKeeper))
			require.Equal(t, height >= deleteHeight, pcommon.RetiredAt(s.ctx, &s.app.UpgradeKeeper, types.StoreDeleteUpgrade, height))
			before := s.k.ExportGenesis(s.ctx)
			logs := len(db.GetAllLogs())
			if height < mergeHeight {
				method := p.GetABI().Methods[layerxanchor.BeginUnbondMethod]
				res, err := p.GetExecutor().Execute(s.ctx, &method, operator, operator, writes[method.Name], nil, false, evm, nil)
				require.NoError(t, err)
				out, err := method.Outputs.Unpack(res)
				require.NoError(t, err)
				require.Equal(t, []interface{}{uint64(s.ctx.BlockTime().Unix()) + unbondingDelay}, out)
				unbonding = unbonding.AddRaw(1_000_000)
			} else {
				for name, args := range writes {
					method := p.GetABI().Methods[name]
					res, err := p.GetExecutor().Execute(s.ctx, &method, operator, operator, args, nil, false, evm, nil)
					require.ErrorIs(t, err, vm.ErrExecutionReverted, name)
					require.Equal(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), res, name)
				}
				require.Equal(t, before, s.k.ExportGenesis(s.ctx))
				require.Len(t, db.GetAllLogs(), logs)
			}
			for name, args := range reads {
				method := p.GetABI().Methods[name]
				for _, readOnly := range []bool{false, true} {
					res, err := p.GetExecutor().Execute(s.ctx, &method, operator, operator, args, nil, readOnly, evm, nil)
					if height >= deleteHeight {
						require.ErrorIs(t, err, vm.ErrExecutionReverted, name)
						require.Equal(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), res, name)
						continue
					}
					require.NoError(t, err, name)
					out, err := method.Outputs.Unpack(res)
					require.NoError(t, err, name)
					switch name {
					case layerxanchor.LatestFinalizedMethod:
						require.Equal(t, []interface{}{uint64(1), true}, out)
					case layerxanchor.FinalizedStateRootMethod:
						require.Equal(t, []interface{}{[32]byte(checkpoint.StateRoot), true}, out)
					case layerxanchor.FinalizedReceiptRootMethod:
						require.Equal(t, []interface{}{[32]byte(checkpoint.ReceiptRoot), true}, out)
					case layerxanchor.ThresholdMethod:
						require.Equal(t, []interface{}{uint32(2)}, out)
					case layerxanchor.StatusOfMethod:
						require.Equal(t, []interface{}{types.CheckpointFinal}, out)
					case layerxanchor.CheckpointBatchMethod:
						require.Equal(t, []interface{}{uint64(1), types.CheckpointFinal}, out)
					case layerxanchor.CheckpointGuarantorsMethod:
						ids := make([][32]byte, len(checkpoint.Guarantors))
						for i, id := range checkpoint.Guarantors {
							ids[i] = id
						}
						require.Equal(t, []interface{}{ids}, out)
					case layerxanchor.CheckpointMethod:
						view := abi.ConvertType(out[0], new(layerxanchor.CheckpointView)).(*layerxanchor.CheckpointView)
						require.Equal(t, checkpoint.BatchNumber, view.BatchNumber)
						require.Equal(t, [32]byte(checkpoint.CheckpointID), view.CheckpointId)
						require.Equal(t, [32]byte(checkpoint.StateRoot), view.StateRoot)
						require.Equal(t, [32]byte(checkpoint.ReceiptRoot), view.ReceiptRoot)
						require.Equal(t, types.CheckpointFinal, view.Status)
					case layerxanchor.GuarantorMethod:
						view := abi.ConvertType(out[0], new(layerxanchor.GuarantorView)).(*layerxanchor.GuarantorView)
						require.Equal(t, s.ids[0], view.GuarantorId)
						require.Equal(t, common.Address(s.signers[0]), view.Signer)
						require.Equal(t, operator, view.Operator)
						if height < mergeHeight {
							require.Equal(t, sdk.NewInt(bond).Sub(unbonding).BigInt(), view.Bond)
							require.Equal(t, unbonding.BigInt(), view.Unbonding)
							require.True(t, view.Eligible)
						} else {
							require.Zero(t, view.Bond.Sign())
							require.Zero(t, view.Unbonding.Sign())
							require.False(t, view.Eligible)
						}
					}
				}
			}
			wantEscrow := total
			if height >= mergeHeight {
				wantEscrow = sdk.ZeroInt()
				require.Empty(t, s.k.GetUnbondings(s.ctx))
				for _, guarantor := range s.k.GetGuarantors(s.ctx) {
					require.True(t, guarantor.Bond.IsZero())
				}
			}
			require.Equal(t, wantEscrow, s.k.ModuleBalance(s.ctx))
			require.Equal(t, wantEscrow, s.k.Escrowed(s.ctx))
			for i, operator := range s.operators {
				want := initialBalances[i]
				if height < mergeHeight {
					want = want.SubRaw(bond)
				}
				require.Equal(t, want, s.balance(operator))
			}
			message, broken := keeper.BalanceInvariant(s.k)(s.ctx)
			require.False(t, broken, message)
			require.Equal(t, supply, s.app.BankKeeper.GetSupply(s.ctx, s.k.GetParams(s.ctx).BondDenom))
		})
	}
	require.Equal(t, mergeHeight, s.app.UpgradeKeeper.GetDoneHeight(s.ctx, types.MergeUpgrade))
	require.Equal(t, deleteHeight, s.app.UpgradeKeeper.GetDoneHeight(s.ctx, types.StoreDeleteUpgrade))
}

func TestExportAfterRetireValidates(t *testing.T) {
	for _, name := range []string{"bonded", "unbonding", "mixed_escrow"} {
		t.Run(name, func(t *testing.T) {
			s := newAnchorRetirementSuite(t)
			s.bondAll()
			checkpoint, err := s.submit("batch_1_quorum")
			require.NoError(t, err)
			if name != "bonded" {
				amount := int64(1_000_000)
				if name == "unbonding" {
					amount = bond
				}
				for i := range s.ids {
					_, err := s.k.BeginUnbond(s.ctx, s.operators[i], s.ids[i], sdk.NewInt(amount))
					require.NoError(t, err)
				}
				require.Len(t, s.k.GetUnbondings(s.ctx), len(s.ids))
			}
			challengeBond := sdk.ZeroInt()
			if name == "mixed_escrow" {
				challengeBond = s.k.GetParams(s.ctx).ChallengeBond
				s.fund(s.reporter, challengeBond.Int64())
				_, err := s.k.OpenChallenge(s.ctx, s.reporter, 1, types.ChallengeFraud, [32]byte{1}, challengeBond)
				require.NoError(t, err)
			}
			before := s.k.ExportGenesis(s.ctx)
			require.NoError(t, before.Validate())
			require.Equal(t, sdk.NewInt(int64(len(s.ids))*bond).Add(challengeBond), s.k.Escrowed(s.ctx))
			balances := make([]sdk.Int, len(s.operators))
			for i, operator := range s.operators {
				balances[i] = s.balance(operator)
			}
			reporterBalance := s.balance(s.reporter)
			s.ctx = s.ctx.WithBlockHeight(20)
			returned, err := s.k.RetireAtMerge(s.ctx)
			require.NoError(t, err)
			require.Equal(t, sdk.NewInt(int64(len(s.ids))*bond).Add(challengeBond), returned)
			genesis := s.k.ExportGenesis(s.ctx)
			require.NoError(t, genesis.Validate())
			require.Len(t, genesis.Guarantors, len(s.ids))
			for _, guarantor := range genesis.Guarantors {
				require.True(t, guarantor.Bond.IsZero())
			}
			require.Empty(t, genesis.Unbondings)
			require.True(t, genesis.HasLatestFinalized)
			require.Equal(t, checkpoint.BatchNumber, genesis.LatestFinalized)
			require.Equal(t, []types.Checkpoint{checkpoint}, genesis.Checkpoints)
			require.Equal(t, before.Sequencers, genesis.Sequencers)
			for _, challenge := range genesis.Challenges {
				require.Equal(t, types.ChallengeUpheld, challenge.Status)
				require.Equal(t, s.ctx.BlockHeight(), challenge.ResolvedHeight)
			}
			for i, operator := range s.operators {
				require.Equal(t, balances[i].AddRaw(bond), s.balance(operator))
			}
			require.Equal(t, reporterBalance.Add(challengeBond), s.balance(s.reporter))
			require.Equal(t, sdk.ZeroInt(), s.k.ModuleBalance(s.ctx))
			require.Equal(t, sdk.ZeroInt(), s.k.Escrowed(s.ctx))
			message, broken := keeper.BalanceInvariant(s.k)(s.ctx)
			require.False(t, broken, message)
		})
	}
}
