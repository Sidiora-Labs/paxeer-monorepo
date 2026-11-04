package cli_test

import (
	"bytes"
	"path/filepath"
	"strconv"
	"testing"

	paxcmd "github.com/Sidiora-Labs/Paxeer-X-Network/daemon/paxd/cmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func repairM944Help(t *testing.T, name, example string) *cobra.Command {
	t.Helper()
	root, _ := paxcmd.NewRootCmd()
	require.Equal(t, "paxd", root.Name())
	command, remaining, err := root.Find([]string{"query", "launchpad", name})
	require.NoError(t, err)
	require.Empty(t, remaining)
	require.Equal(t, "paxd query launchpad "+name, command.CommandPath())
	require.Equal(t, example, command.Example)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"query", "launchpad", name, "--help"})
	require.NoError(t, root.ExecuteContext(t.Context()), stderr.String())
	require.Contains(t, stdout.String(), "paxd query launchpad "+command.Use)
	require.Contains(t, stdout.String(), example)
	require.NotContains(t, stdout.String(), "layerxd")
	return command
}

func TestRepairM_944_f11_AirdropHelp(t *testing.T) {
	repairM944Help(t, "airdrop-epoch", "paxd query launchpad airdrop-epoch 0x0000000000000000000000000000000000000001 1 --output json")
	repairM944Help(t, "airdrop-claimed", "paxd query launchpad airdrop-claimed 0x0000000000000000000000000000000000000001 0x0000000000000000000000000000000000000002 1 --output json")
}

func TestRepairM_944_f12_FeeRightHelp(t *testing.T) {
	repairM944Help(t, "fee-right", "paxd query launchpad fee-right 0xe8DcC844a8006dbf64f5Bb851bCad90292a6409B 1 --output json")
}

func TestRepairM_944_f13_MarketHelp(t *testing.T) {
	repairM944Help(t, "market", "paxd query launchpad market launchpad/0x0000000000000000000000000000000000000001 --output json")
	repairM944Help(t, "market-by-address", "paxd query launchpad market-by-address 0x0000000000000000000000000000000000000001 --output json")
}

func TestRepairM_944_f14_CreatorPagination(t *testing.T) {
	command := repairM944Help(t, "markets-by-creator", "paxd query launchpad markets-by-creator <creator> --offset 0 --limit 1 --output json")
	require.Equal(t, "0", command.Flags().Lookup(flags.FlagOffset).DefValue)
	require.Equal(t, "100", command.Flags().Lookup(flags.FlagLimit).DefValue)
	genesis, _, market := repair35State(t)
	cfg, net := proofModuleCliNetwork(t, genesis)
	height, err := net.WaitForHeight(1)
	require.NoError(t, err)
	ctx := net.Validators[0].ClientCtx.WithHeight(height)
	for offset := 0; offset <= len(genesis.Markets); offset++ {
		output, err := clitestutil.ExecTestCLICmd(ctx, cli.CmdQueryMarketsByCreator(), []string{
			market.Creator, "--offset=" + strconv.Itoa(offset), "--limit=1", "--output=json",
			"--height=" + strconv.FormatInt(height, 10),
		})
		require.NoError(t, err)
		var got types.QueryMarketsByCreatorResponse
		require.NoError(t, cfg.Codec.UnmarshalAsJSON(output.Bytes(), &got))
		if offset < len(genesis.Markets) {
			require.Equal(t, []types.MarketInfo{types.MarketInfoOf(genesis.Markets[offset])}, got.Markets)
		} else {
			require.Empty(t, got.Markets)
		}
	}
	for _, limit := range []string{"0", "101"} {
		output, err := clitestutil.ExecTestCLICmd(ctx, cli.CmdQueryMarketsByCreator(), []string{
			market.Creator, "--limit=" + limit, "--output=json",
		})
		require.ErrorContains(t, err, types.ErrInvalidRange.Error())
		require.NotContains(t, output.String(), `"markets"`)
	}
}

func TestRepairM_944_f15_FeeAccountingHelp(t *testing.T) {
	repairM944Help(t, "fee-accounting", "paxd query launchpad fee-accounting --output json")
}

func TestRepairM_944_f17_MergedRoot(t *testing.T) {
	require.Equal(t, "paxd", filepath.Base(proofModuleCliBinary(t)))
	root, _ := paxcmd.NewRootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.ExecuteContext(t.Context()))
	require.Equal(t, "paxd", root.Name())
	require.Contains(t, output.String(), "paxd [command]")
	require.NotContains(t, output.String(), "layerxd")
	repairM944Help(t, "market", "paxd query launchpad market launchpad/0x0000000000000000000000000000000000000001 --output json")
}

func TestRepairM_944_f18_AccountingFromGenesis(t *testing.T) {
	genesis, market, epoch := proofModuleCliState(t)
	require.NotEmpty(t, genesis.FeeAccounting.Pools)
	require.NotEmpty(t, genesis.FeeAccounting.Epochs)
	require.NotEmpty(t, genesis.FeeAccounting.Claims)
	want := types.FeeAccountingInfoOf(genesis.FeeAccounting)
	cfg, net := proofModuleCliNetwork(t, genesis)
	height, err := net.WaitForHeight(1)
	require.NoError(t, err)
	ctx := net.Validators[0].ClientCtx.WithHeight(height)
	service := types.NewQueryClient(ctx)
	gotEpoch, err := service.AirdropEpoch(t.Context(), &types.QueryAirdropEpochRequest{Pool: market.PoolAddress, Epoch: epoch.Epoch})
	require.NoError(t, err)
	require.True(t, gotEpoch.Found)
	wantEpoch := types.AirdropEpochInfoOf(epoch)
	require.Equal(t, &wantEpoch, gotEpoch.AirdropEpoch)
	claim := genesis.FeeAccounting.Claims[0]
	gotClaim, err := service.AirdropClaimed(t.Context(), &types.QueryAirdropClaimedRequest{Pool: claim.Pool, Holder: claim.Holder, Epoch: claim.Epoch})
	require.NoError(t, err)
	require.True(t, gotClaim.Claimed)
	output, err := clitestutil.ExecTestCLICmd(ctx, cli.CmdQueryFeeAccounting(), []string{
		"--height=" + strconv.FormatInt(height, 10), "--output=json",
	})
	require.NoError(t, err)
	var got types.QueryFeeAccountingResponse
	require.NoError(t, cfg.Codec.UnmarshalAsJSON(output.Bytes(), &got))
	require.Equal(t, want, got.Accounting)
}

func TestRepairM_944_f09_FundedUint256Quote(t *testing.T) {
	t.Run("arguments-and-height-overflow", TestCmdQueryPriceArgs)
	t.Run("funded-network", TestCmdQueryPriceNetwork)
}
