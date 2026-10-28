package app_test

import (
	"testing"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRPCPendingSnapshotServesBaseFee(t *testing.T) {
	for _, withFloor := range []bool{false, true} {
		name := "standby"
		if withFloor {
			name = "sequencer"
		}
		t.Run(name, func(t *testing.T) {
			a, exec, genesis := newSequencerContextApp(t, false)
			_, err := a.RPCPendingSnapshot()
			require.ErrorIs(t, err, app.ErrPendingServedBySequencer)
			if withFloor {
				exec, err = sequencer.NewFeeFloor(exec, a, "", a.ParamsKeeper, a.OracleKeeper, a.PricefeedKeeper, &a.EvmKeeper)
				require.NoError(t, err)
			}
			a.SetOpenBlock(exec)
			pending, err := a.RPCPendingSnapshot()
			require.NoError(t, err)
			baseFee, ok := pending.(evmrpc.PendingBaseFee)
			require.True(t, ok, "rpcPendingSnapshot must implement PendingBaseFee")
			_, err = baseFee.BaseFeePerGas()
			require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
			require.NoError(t, exec.Open(seqCtxOpen, genesis.Add(time.Second).UnixMilli()))
			ctx, err := exec.Snapshot()
			require.NoError(t, err)
			want := a.EvmKeeper.GetNextBaseFeePerGas(ctx)
			require.True(t, want.IsPositive())
			if withFloor {
				fixed, err := exec.(evmrpc.PendingBaseFee).BaseFeePerGas()
				require.NoError(t, err)
				require.True(t, want.Equal(fixed))
			}
			got, err := baseFee.BaseFeePerGas()
			require.NoError(t, err)
			require.True(t, want.Equal(got), "want %s, got %s", want, got)
			// A write to a pending read must not change the block's fixed fee.
			snapshot, err := pending.Snapshot()
			require.NoError(t, err)
			a.EvmKeeper.SetNextBaseFeePerGas(snapshot, want.Add(sdk.OneDec()))
			got, err = baseFee.BaseFeePerGas()
			require.NoError(t, err)
			require.True(t, want.Equal(got))
			_, err = exec.Seal(seqCtxOpen)
			require.NoError(t, err)
			_, err = baseFee.BaseFeePerGas()
			require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
		})
	}
}
