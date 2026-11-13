package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/cli"
	gcutils "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/utils"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func governanceHistoryExecute(ctx context.Context, clientCtx client.Context, cmd *cobra.Command, args ...string) (string, error) {
	var output bytes.Buffer
	clientCtx = clientCtx.WithOutput(&output)
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.WithValue(ctx, client.ClientContextKey, &clientCtx))
	return output.String(), err
}

func governanceHistoryQueryCase(t *testing.T, name string) {
	t.Helper()
	chain := newVoteHistoryChain(t, "kv")
	voter := sdk.AccAddress(bytes.Repeat([]byte{11}, 20))
	other := sdk.AccAddress(bytes.Repeat([]byte{12}, 20))
	require.NoError(t, chain.keeper.AddVote(chain.ctx, chain.proposal.ProposalId, voter, govtypes.NewNonSplitVoteOption(govtypes.OptionYes)))
	deposit := govtypes.NewDeposit(chain.proposal.ProposalId, voter, sdk.NewCoins(sdk.NewInt64Coin("stake", 37)))
	chain.keeper.SetDeposit(chain.ctx, deposit)
	chain.commit()
	clientCtx := chain.client(t)
	id := fmt.Sprint(chain.proposal.ProposalId)
	query := func(cmd *cobra.Command, args ...string) string {
		t.Helper()
		output, err := governanceHistoryExecute(t.Context(), clientCtx, cmd, append(args, "--output=json")...)
		require.NoError(t, err)
		return output
	}
	fails := func(cmd *cobra.Command, args ...string) {
		t.Helper()
		output, err := governanceHistoryExecute(t.Context(), clientCtx, cmd, args...)
		require.Error(t, err)
		require.Empty(t, output)
	}
	decode := func(output string, target proto.Message) {
		t.Helper()
		require.NoError(t, clientCtx.Codec.UnmarshalAsJSON([]byte(output), target), output)
	}
	switch name {
	case "params":
		var params govtypes.Params
		require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON([]byte(query(cli.GetCmdQueryParams())), &params))
		require.Equal(t, govtypes.NewParams(chain.keeper.GetVotingParams(chain.ctx), chain.keeper.GetTallyParams(chain.ctx), chain.keeper.GetDepositParams(chain.ctx)), params)
		output, err := governanceHistoryExecute(t.Context(), clientCtx, cli.GetCmdQueryParams(), "--output=text")
		require.NoError(t, err)
		jsonOutput := query(cli.GetCmdQueryParams())
		var value any
		require.NoError(t, json.Unmarshal([]byte(jsonOutput), &value))
		expected, err := yaml.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, string(expected), output)
	case "param":
		for _, kind := range []string{"voting", "tallying", "deposit"} {
			output := query(cli.GetCmdQueryParam(), kind)
			switch kind {
			case "voting":
				var got govtypes.VotingParams
				require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON([]byte(output), &got))
				require.Equal(t, chain.keeper.GetVotingParams(chain.ctx), got)
			case "tallying":
				var got govtypes.TallyParams
				require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON([]byte(output), &got))
				require.Equal(t, chain.keeper.GetTallyParams(chain.ctx), got)
			case "deposit":
				var got govtypes.DepositParams
				require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON([]byte(output), &got))
				require.Equal(t, chain.keeper.GetDepositParams(chain.ctx), got)
			}
		}
	case "proposer":
		fails(cli.GetCmdQueryProposer())
		msg, err := govtypes.NewMsgSubmitProposal(govtypes.NewTextProposal("Indexed proposer", "Preserve proposer account identity", false), sdk.NewCoins(), voter)
		require.NoError(t, err)
		chain.cast(t, msg, true)
		chain.commit()
		var got gcutils.Proposer
		require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON([]byte(query(cli.GetCmdQueryProposer(), "2")), &got))
		require.Equal(t, gcutils.NewProposer(2, voter.String()), got)
	case "tally":
		fails(cli.GetCmdQueryTally())
		var got govtypes.TallyResult
		decode(query(cli.GetCmdQueryTally(), id), &got)
		require.Equal(t, govtypes.EmptyTallyResult(), got)
		output, err := governanceHistoryExecute(t.Context(), clientCtx, cli.GetCmdQueryTally(), id, "--output=text")
		require.NoError(t, err)
		jsonOutput := query(cli.GetCmdQueryTally(), id)
		var value any
		require.NoError(t, json.Unmarshal([]byte(jsonOutput), &value))
		expected, err := yaml.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, string(expected), output)
	case "proposal":
		var got govtypes.Proposal
		decode(query(cli.GetCmdQueryProposal(), id), &got)
		require.True(t, chain.proposal.Equal(got))
	case "proposals":
		fails(cli.GetCmdQueryProposals(), "--status=unknown")
		var got govtypes.QueryProposalsResponse
		decode(query(cli.GetCmdQueryProposals()), &got)
		require.Len(t, got.Proposals, 1)
		require.True(t, chain.proposal.Equal(got.Proposals[0]))
	case "deposits":
		fails(cli.GetCmdQueryDeposits(), "10")
		var got govtypes.QueryDepositsResponse
		decode(query(cli.GetCmdQueryDeposits(), id), &got)
		require.Equal(t, []govtypes.Deposit{deposit}, got.Deposits)
	case "deposit":
		fails(cli.GetCmdQueryDeposit(), id)
		fails(cli.GetCmdQueryDeposit(), id, "wrong address")
		fails(cli.GetCmdQueryDeposit(), id, other.String())
		var got govtypes.Deposit
		decode(query(cli.GetCmdQueryDeposit(), id, voter.String()), &got)
		require.Equal(t, deposit, got)
	case "votes":
		fails(cli.GetCmdQueryVotes())
		fails(cli.GetCmdQueryVotes(), "10")
		var got govtypes.QueryVotesResponse
		decode(query(cli.GetCmdQueryVotes(), id), &got)
		require.Len(t, got.Votes, 1)
		require.Equal(t, voter.String(), got.Votes[0].Voter)
		require.Equal(t, chain.proposal.ProposalId, got.Votes[0].ProposalId)
		require.Equal(t, govtypes.NewNonSplitVoteOption(govtypes.OptionYes), govtypes.WeightedVoteOptions(got.Votes[0].Options))
	case "vote":
		fails(cli.GetCmdQueryVote(), "10", voter.String())
		fails(cli.GetCmdQueryVote(), id, "wrong address")
		fails(cli.GetCmdQueryVote(), id, other.String())
		var got govtypes.Vote
		decode(query(cli.GetCmdQueryVote(), id, voter.String()), &got)
		require.Equal(t, voter.String(), got.Voter)
		require.Equal(t, chain.proposal.ProposalId, got.ProposalId)
		require.Equal(t, govtypes.NewNonSplitVoteOption(govtypes.OptionYes), govtypes.WeightedVoteOptions(got.Options))
	default:
		t.Fatalf("unknown query case %q", name)
	}
}

func (s *CLITestSuite) TestCmdParams()        { governanceHistoryQueryCase(s.T(), "params") }
func (s *CLITestSuite) TestCmdParam()         { governanceHistoryQueryCase(s.T(), "param") }
func (s *CLITestSuite) TestCmdProposer()      { governanceHistoryQueryCase(s.T(), "proposer") }
func (s *CLITestSuite) TestCmdTally()         { governanceHistoryQueryCase(s.T(), "tally") }
func (s *CLITestSuite) TestCmdGetProposal()   { governanceHistoryQueryCase(s.T(), "proposal") }
func (s *CLITestSuite) TestCmdGetProposals()  { governanceHistoryQueryCase(s.T(), "proposals") }
func (s *CLITestSuite) TestCmdQueryDeposits() { governanceHistoryQueryCase(s.T(), "deposits") }
func (s *CLITestSuite) TestCmdQueryDeposit()  { governanceHistoryQueryCase(s.T(), "deposit") }
func (s *CLITestSuite) TestCmdQueryVotes()    { governanceHistoryQueryCase(s.T(), "votes") }
func (s *CLITestSuite) TestCmdQueryVote()     { governanceHistoryQueryCase(s.T(), "vote") }
