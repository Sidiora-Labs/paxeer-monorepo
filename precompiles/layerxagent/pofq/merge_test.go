package pofq_test

import (
	"math/big"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/pofq"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestPofqMergeConstructorRegistered(t *testing.T) {
	for _, test := range []struct {
		name    string
		keepers utils.Keepers
	}{
		{"nil keepers", nil},
		{"missing evm keeper", &app.PrecompileKeepers{}},
		{"nil evm keeper", &app.PrecompileKeepers{EVMKeeper: (*evmkeeper.Keeper)(nil)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, err := pofq.NewMergePrecompile(test.keepers, precompiles.MergeUpgrade)
			require.Nil(t, p)
			require.ErrorIs(t, err, agentcommon.ErrNoContext)
		})
	}

	testApp := app.Setup(t, false, true, false)
	keepers := testApp.GetPrecompileKeepers()
	address := common.HexToAddress("0x0000000000000000000000000000000000000904")
	require.Equal(t, address, common.HexToAddress(pofq.PofqAddress))
	set := precompiles.GetCustomPrecompiles(app.LatestUpgrade, keepers)
	versions, found := set[address]
	require.True(t, found)
	served, ok := versions[precompiles.MergeUpgrade].(*pofq.PofqPrecompile)
	require.True(t, ok)
	require.Same(t, served, versions[app.LatestUpgrade])
	require.Equal(t, address, served.Address())
	require.Equal(t, "pofq", served.GetName())

	p, err := pofq.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.Equal(t, address, p.Address())
	require.Equal(t, "pofq", p.GetName())
}

func TestPofqMergeBatchCap(t *testing.T) {
	require.Equal(t, uint64(256), pofq.MergeMaxBatch)
	testApp := app.Setup(t, false, true, false)
	set := precompiles.GetCustomPrecompiles(app.LatestUpgrade, testApp.GetPrecompileKeepers())
	p, ok := set[common.HexToAddress(pofq.PofqAddress)][precompiles.MergeUpgrade].(*pofq.PofqPrecompile)
	require.True(t, ok)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(100)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	testApp.EvmKeeper.UpgradeKeeper().SetDone(ctx, precompiles.MergeUpgrade)

	fills := make([]pofq.Fill, pofq.MergeMaxBatch)
	for i := range fills {
		fills[i] = pofq.Fill{
			FillPrice:   big.NewInt(1000 + 10*int64(i%2)),
			OraclePrice: big.NewInt(1000),
			Size:        big.NewInt(2),
		}
	}
	calldata := input(t, p, pofq.ScoreBatchMethod, fills)
	require.Equal(t, []byte{0x1b, 0x1f, 0xba, 0x48}, calldata[:4])
	refusal(t, p, ctx.WithBlockHeight(99), pofq.ScoreBatchMethod, agentcommon.ErrNotActive,
		"LXP_AGENT_NOT_ACTIVE: "+precompiles.MergeUpgrade+" not active at height 99", fills)
	got := run(t, p, ctx, pofq.ScoreBatchMethod, fills)
	require.Len(t, got, 2)
	require.Equal(t, "995000000000000000", got[0].String())
	require.Equal(t, "512", got[1].String())

	fills = append(fills, pofq.Fill{FillPrice: big.NewInt(1000), OraclePrice: big.NewInt(1000), Size: big.NewInt(2)})
	require.Len(t, fills, int(pofq.MergeMaxBatch)+1)
	refusal(t, p, ctx, pofq.ScoreBatchMethod, agentcommon.ErrBatchTooLarge,
		"LXP_AGENT_BATCH_TOO_LARGE: batch over the cap (cap set at activation rehearsal)", fills)
}
