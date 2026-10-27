package legacyabci_test

import (
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestEndBlockAddsAgentRunDueGas(t *testing.T) {
	for _, name := range []string{"live", "before_merge", "nil_merge_live", "nil_agent_keeper"} {
		t.Run(name, func(t *testing.T) {
			h := newEndBlockAgentHarness(t)
			h.app.EvmKeeper.SetCode(h.ctx, h.target, []byte{0x00})
			h.ctx = h.ctx.WithConsensusParams(&tmtypes.ConsensusParams{
				Block: &tmtypes.BlockParams{MaxGas: 1_000_000},
			})
			params := h.app.EvmKeeper.GetParams(h.ctx)
			params.MinimumFeePerGas = sdk.NewDec(1)
			params.MaximumFeePerGas = sdk.NewDec(100)
			params.TargetGasUsedPerBlock = 100_000
			params.MaxDynamicBaseFeeDownwardAdjustment = sdk.NewDecWithPrec(5, 1)
			h.app.EvmKeeper.SetParams(h.ctx, params)
			h.keepers.MergeLive = func() bool { return true }
			switch name {
			case "before_merge":
				h.keepers.MergeLive = func() bool { return false }
			case "nil_merge_live":
				h.keepers.MergeLive = nil
			case "nil_agent_keeper":
				h.keepers.AgentKeeper = nil
			}

			legacyabci.EndBlock(h.ctx, h.ctx.BlockHeight(), 100, h.keepers)

			var executed []*agenttypes.EventJobExecuted
			for _, event := range h.ctx.EventManager().Events() {
				require.NotEqual(t, proto.MessageName(&agenttypes.EventJobFailed{}), event.Type)
				if event.Type == proto.MessageName(&agenttypes.EventJobExecuted{}) {
					parsed, err := sdk.ParseTypedEvent(abci.Event(event))
					require.NoError(t, err)
					executed = append(executed, parsed.(*agenttypes.EventJobExecuted))
				}
			}
			_, found := h.agent.GetJob(h.ctx, h.jobID)
			if name == "live" {
				require.False(t, found)
				require.Len(t, executed, 1)
				require.Equal(t, h.jobID, executed[0].JobId)
				require.Equal(t, uint64(21_000), executed[0].GasUsed)
				require.Equal(t, sdk.MustNewDecFromStr("6.055"), h.app.EvmKeeper.GetNextBaseFeePerGas(h.ctx))
			} else {
				require.True(t, found)
				require.Empty(t, executed)
				require.Equal(t, sdk.MustNewDecFromStr("5.005"), h.app.EvmKeeper.GetNextBaseFeePerGas(h.ctx))
			}
		})
	}
}
