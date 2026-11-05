package keeper_test

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func retiredAnchorStore(t *testing.T, s *suite) map[string][]byte {
	t.Helper()
	for _, key := range s.ctx.MultiStore().StoreKeys() {
		if key.Name() != types.StoreKey {
			continue
		}
		iterator := s.ctx.KVStore(key).Iterator(nil, nil)
		defer iterator.Close()
		state := make(map[string][]byte)
		for ; iterator.Valid(); iterator.Next() {
			state[string(iterator.Key())] = bytes.Clone(iterator.Value())
		}
		return state
	}
	t.Fatal("anchor store is not mounted")
	return nil
}

func seedRetiredAnchorState(t *testing.T, s *suite) {
	t.Helper()
	s.bondAll()
	_, err := s.k.BeginUnbond(s.ctx, s.operators[0], s.ids[0], sdk.NewInt(bond/2))
	require.NoError(t, err)
	genesis := s.k.ExportGenesis(s.ctx)
	genesis.Checkpoints = []types.Checkpoint{
		{BatchNumber: 1, CheckpointID: types.Hash32{0x11}, StateRoot: types.Hash32{0x12}, ReceiptRoot: types.Hash32{0x13},
			Status: types.CheckpointFinal, Guarantors: []types.Hash32{s.ids[0], s.ids[1]}, DeclaredThreshold: 2},
		{BatchNumber: 2, CheckpointID: types.Hash32{0x21}, StateRoot: types.Hash32{0x22}, ReceiptRoot: types.Hash32{0x23},
			Status: types.CheckpointSubmitted, Guarantors: []types.Hash32{s.ids[0], s.ids[1]}, DeclaredThreshold: 2},
	}
	genesis.HasLatestFinalized, genesis.LatestFinalized = true, 1
	genesis.Availability = []types.AvailabilityAttestation{{BatchNumber: 2, CheckpointID: types.Hash32{0x21},
		GuarantorID: s.ids[0], ClassMask: 0x1f}}
	require.NoError(t, genesis.Validate())
	s.k.InitGenesis(s.ctx, *genesis)
	s.invariant()
}

func TestSubmitCheckpointRefusedWithoutKernelVerifier(t *testing.T) {
	s := newSuite(t, nil)
	t.Cleanup(func() { require.NoError(t, s.app.Close()) })
	for _, populated := range []bool{false, true} {
		if populated {
			seedRetiredAnchorState(t, s)
		}
		before := retiredAnchorStore(t, s)
		events := s.ctx.EventManager().Events()
		for _, submitter := range []sdk.AccAddress{nil, s.reporter, s.authority} {
			for _, input := range [][]byte{nil, {}, {0xff}, bytes.Repeat([]byte{0xff}, 274), bytes.Repeat([]byte{0x42}, 4096)} {
				checkpoint, err := s.k.SubmitCheckpoint(s.ctx, submitter, input, [64]byte{0xff}, input)
				require.ErrorIs(t, err, types.ErrModuleRetired)
				require.Equal(t, types.Checkpoint{}, checkpoint)
				require.Equal(t, before, retiredAnchorStore(t, s))
				require.Equal(t, events, s.ctx.EventManager().Events())
			}
		}
	}
	checkpoint, err := (keeper.Keeper{}).SubmitCheckpoint(sdk.Context{}, nil, nil, [64]byte{}, nil)
	require.ErrorIs(t, err, types.ErrModuleRetired)
	require.Equal(t, types.Checkpoint{}, checkpoint)
}

func TestSubmitAvailabilityRefusedWithoutKernelVerifier(t *testing.T) {
	s := newSuite(t, nil)
	t.Cleanup(func() { require.NoError(t, s.app.Close()) })
	for _, populated := range []bool{false, true} {
		if populated {
			seedRetiredAnchorState(t, s)
		}
		before := retiredAnchorStore(t, s)
		events := s.ctx.EventManager().Events()
		for _, input := range [][]byte{nil, {}, {0xff}, bytes.Repeat([]byte{0xff}, 274), bytes.Repeat([]byte{0x42}, 4096)} {
			checkpoint, record, err := s.k.SubmitAvailabilityAttestation(s.ctx, input)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Equal(t, types.Checkpoint{}, checkpoint)
			require.Equal(t, types.AvailabilityAttestation{}, record)
			require.Equal(t, before, retiredAnchorStore(t, s))
			require.Equal(t, events, s.ctx.EventManager().Events())
		}
	}
	checkpoint, record, err := (keeper.Keeper{}).SubmitAvailabilityAttestation(sdk.Context{}, nil)
	require.ErrorIs(t, err, types.ErrModuleRetired)
	require.Equal(t, types.Checkpoint{}, checkpoint)
	require.Equal(t, types.AvailabilityAttestation{}, record)
}

func TestSubmitEquivocationRefusedWithoutKernelVerifier(t *testing.T) {
	s := newSuite(t, nil)
	t.Cleanup(func() { require.NoError(t, s.app.Close()) })
	for _, populated := range []bool{false, true} {
		if populated {
			seedRetiredAnchorState(t, s)
		}
		before := retiredAnchorStore(t, s)
		events := s.ctx.EventManager().Events()
		guarantors, unbondings := s.k.GetGuarantors(s.ctx), s.k.GetUnbondings(s.ctx)
		supply := s.app.BankKeeper.GetSupply(s.ctx, "uhpx")
		moduleBalance, reporterBalance := s.k.ModuleBalance(s.ctx), s.balance(s.reporter)
		for _, reporter := range []sdk.AccAddress{nil, s.reporter, s.authority} {
			for _, evidence := range [][2][]byte{{nil, nil}, {{}, {}}, {{0xff}, nil}, {nil, {0xff}},
				{bytes.Repeat([]byte{0xff}, 274), bytes.Repeat([]byte{0x42}, 274)}} {
				record, err := s.k.SubmitEquivocation(s.ctx, reporter, evidence[0], evidence[1])
				require.ErrorIs(t, err, types.ErrModuleRetired)
				require.Equal(t, types.SlashRecord{}, record)
				require.Empty(t, s.k.GetSlashRecords(s.ctx))
				require.Equal(t, before, retiredAnchorStore(t, s))
				require.Equal(t, guarantors, s.k.GetGuarantors(s.ctx))
				require.Equal(t, unbondings, s.k.GetUnbondings(s.ctx))
				require.Equal(t, supply, s.app.BankKeeper.GetSupply(s.ctx, "uhpx"))
				require.Equal(t, moduleBalance, s.k.ModuleBalance(s.ctx))
				require.Equal(t, reporterBalance, s.balance(s.reporter))
				require.Equal(t, events, s.ctx.EventManager().Events())
			}
		}
		s.invariant()
	}
	record, err := (keeper.Keeper{}).SubmitEquivocation(sdk.Context{}, nil, nil, nil)
	require.ErrorIs(t, err, types.ErrModuleRetired)
	require.Equal(t, types.SlashRecord{}, record)
}
