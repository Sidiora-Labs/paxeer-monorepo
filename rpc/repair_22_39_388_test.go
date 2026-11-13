//go:build cgo

package evmrpc_test

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/stretchr/testify/require"
)

func TestRepair22_39_388_FundAfterBlockEndTransition(t *testing.T) {
	wasm := repair478PackageGuest(t, repair478RawGuest(t, "")).packaged
	for _, candidate := range []struct {
		name          string
		priorActivity bool
	}{
		{name: "first_activity"},
		{name: "after_activity", priorActivity: true},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			repair478RunWithProbe(t, wasm, repair478CPUAllowance, false, "", func(f repair478ProbeFixture) {
				ctx := f.Context
				last, err := f.Host.ReadLastSequence(ctx, f.DID)
				require.NoError(t, err)
				FinalizeProgramsOccupancy(t, f.Host, ctx, last+1)
				ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
				ctx = ctx.WithBlockTime(time.UnixMilli(seqVecTimeBase + ctx.BlockHeight()*seqVecBlockMs))
				if candidate.priorActivity {
					// An idempotent account registration occupies the first activity.
					f.ExecuteAt(ctx, 0x00090006, f.Registration)
				}
				rewards := [32]byte(f.Fixture["rewards"])
				caps := []byte{0, 4, 3, 5}
				caps = append(caps, f.Asset[:]...)
				caps = append(caps, rewards[:]...)
				caps = append(caps, make([]byte, 8)...)
				caps = binary.BigEndian.AppendUint64(caps, 100)
				caps = append(caps, 7, 8)
				budget := [7]uint64{repair478CPUAllowance, 16777216, 1048576, 1048576, 64, 1048576, 4096}
				fund, err := lp.EncodeCallPayload(repair478Program, 5, "layerx_call", f.Fixture["fund"], caps, []byte("LayerX/programs/access-declaration/v1\x00\x00"), 16384, budget)
				require.NoError(t, err)
				sequence, err := f.Host.ReadIdentitySequence(ctx, f.DID)
				require.NoError(t, err)
				last, err = f.Host.ReadLastSequence(ctx, f.DID)
				require.NoError(t, err)
				var feeLimit [16]byte
				binary.BigEndian.PutUint64(feeLimit[8:], 100000000)
				timestamp := uint64(ctx.BlockTime().UnixMilli())
				receipt, err := f.Host.Execute(lxphost.WithSequence(ctx, last+1), lxphost.Activity{
					ProtocolVersion: codec.StateCommitmentProtocolVersion, Type: 0x00090003,
					ActorDID: f.DID, AuthorityKey: f.PublicKey, AccountSequence: sequence,
					NotBefore: timestamp, NotAfter: timestamp, IdempotencyKey: sha256.Sum256(fund),
					Payload: fund, FeeLimit: feeLimit,
				}, f.Authority)
				require.NoError(t, err)
				require.Zero(t, receipt.ResultCode)
				require.NotNil(t, receipt.ProgramOutcome)
				require.Zero(t, receipt.ProgramOutcome.Status)
				require.Equal(t, uint8(1), receipt.ProgramOutcome.Kind)
				require.Len(t, receipt.Transfers, 1)
				require.Equal(t, f.Asset, receipt.Transfers[0].Asset)
				require.Equal(t, rewards, receipt.Transfers[0].To)
				require.Equal(t, int64(100), receipt.Transfers[0].Amount.Int64())
			})
		})
	}
}
