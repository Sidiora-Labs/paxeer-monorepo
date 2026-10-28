package integration_test

import (
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	agentkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	govkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type agentGovernanceTest struct {
	*app.TestWrapper
	agent agentkeeper.Keeper
}

func newAgentGovernanceTest(t *testing.T) (*agentGovernanceTest, sdk.AccAddress) {
	t.Helper()
	pub := ed25519.GenPrivKey().PubKey()
	wrapper := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), pub, true)
	w := &agentGovernanceTest{TestWrapper: wrapper, agent: wrapper.App.LayerXAgentKeeper.Keeper}
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	contract, err := scheduler.NewPrecompile(w.agent, &w.App.EvmKeeper, &w.App.UpgradeKeeper, app.MergeUpgrade)
	require.NoError(t, err)
	served := precompiles.GetCustomPrecompiles(app.LatestUpgrade, w.App.GetPrecompileKeepers())
	served[contract.Address()] = putils.VersionedPrecompiles{app.LatestUpgrade: contract, app.MergeUpgrade: contract}
	w.App.EvmKeeper.SetCustomPrecompiles(served, app.LatestUpgrade)
	voter := sdk.AccAddress(pub.Address())
	validator, found := w.App.StakingKeeper.GetValidator(w.Ctx, sdk.ValAddress(voter))
	require.True(t, found)
	bond := w.App.StakingKeeper.TokensFromConsensusPower(w.Ctx, 10)
	denom := w.App.StakingKeeper.GetParams(w.Ctx).BondDenom
	w.FundAcc(voter, sdk.NewCoins(sdk.NewCoin(denom, bond)))
	_, err = w.App.StakingKeeper.Delegate(w.Ctx, voter, bond, stakingtypes.Unbonded, validator, true)
	require.NoError(t, err)
	staking.EndBlocker(w.Ctx, w.App.StakingKeeper)
	validator, found = w.App.StakingKeeper.GetValidator(w.Ctx, sdk.ValAddress(voter))
	require.True(t, found)
	require.True(t, validator.IsBonded())
	require.True(t, w.App.StakingKeeper.TotalBondedTokens(w.Ctx).IsPositive())
	return w, voter
}

func passAgentParamsProposal(t *testing.T, w *agentGovernanceTest, voter sdk.AccAddress, params agenttypes.Params) {
	t.Helper()
	before := w.agent.GetParams(w.Ctx)
	content, err := layerxgovtypes.NewLayerXProposal("Update agent parameters", "Govern scheduler limits and pause scope",
		&agenttypes.MsgUpdateParams{Authority: layerxgovtypes.GovernanceAuthority(), Params: params})
	require.NoError(t, err)
	require.NoError(t, content.ValidateBasic())
	encoding := app.MakeEncodingConfig()
	encoded, err := encoding.Marshaler.MarshalInterfaceJSON(content)
	require.NoError(t, err)
	var decoded govtypes.Content
	require.NoError(t, encoding.Marshaler.UnmarshalInterfaceJSON(encoded, &decoded))
	require.IsType(t, &layerxgovtypes.LayerXProposal{}, decoded)
	messages, err := decoded.(*layerxgovtypes.LayerXProposal).GetMessages()
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, &agenttypes.MsgUpdateParams{Authority: layerxgovtypes.GovernanceAuthority(), Params: params}, messages[0])
	deposit := w.App.GovKeeper.GetDepositParams(w.Ctx).MinDeposit
	w.FundAcc(voter, deposit)
	submit, err := govtypes.NewMsgSubmitProposal(decoded, deposit, voter)
	require.NoError(t, err)
	require.NoError(t, submit.ValidateBasic())
	encoded, err = govtypes.ModuleCdc.MarshalAsJSON(submit)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"layerxagent/MsgUpdateParams"`)
	var decodedSubmit govtypes.MsgSubmitProposal
	require.NoError(t, encoding.Amino.UnmarshalAsJSON(encoded, &decodedSubmit))
	require.NoError(t, decodedSubmit.ValidateBasic())
	roundTrip, err := govtypes.ModuleCdc.MarshalAsJSON(&decodedSubmit)
	require.NoError(t, err)
	require.JSONEq(t, string(encoded), string(roundTrip))
	require.Equal(t, decoded, decodedSubmit.GetContent())
	server := govkeeper.NewMsgServerImpl(w.App.GovKeeper)
	response, err := server.SubmitProposal(sdk.WrapSDKContext(w.Ctx), &decodedSubmit)
	require.NoError(t, err)
	proposal, found := w.App.GovKeeper.GetProposal(w.Ctx, response.ProposalId)
	require.True(t, found)
	require.Equal(t, govtypes.StatusVotingPeriod, proposal.Status)
	require.Equal(t, before, w.agent.GetParams(w.Ctx))
	_, err = server.Vote(sdk.WrapSDKContext(w.Ctx), govtypes.NewMsgVote(voter, proposal.ProposalId, govtypes.OptionYes))
	require.NoError(t, err)
	require.Equal(t, before, w.agent.GetParams(w.Ctx))
	w.Ctx = w.Ctx.WithBlockHeight(w.Ctx.BlockHeight() + 1).WithBlockTime(proposal.VotingEndTime.Add(time.Second))
	gov.EndBlocker(w.Ctx, w.App.GovKeeper)
	proposal, found = w.App.GovKeeper.GetProposal(w.Ctx, proposal.ProposalId)
	require.True(t, found)
	require.Equal(t, govtypes.StatusPassed, proposal.Status)
	require.True(t, proposal.FinalTallyResult.Yes.IsPositive())
	require.True(t, proposal.FinalTallyResult.No.IsZero())
	require.True(t, proposal.FinalTallyResult.NoWithVeto.IsZero())
	require.Equal(t, params, w.agent.GetParams(w.Ctx))
}

func TestAgentParamsThroughLayerXProposal(t *testing.T) {
	w, voter := newAgentGovernanceTest(t)
	before := w.agent.GetParams(w.Ctx)
	params := before
	params.MaxBlockGas = 20_000_000
	require.NotEqual(t, before.MaxBlockGas, params.MaxBlockGas)
	passAgentParamsProposal(t, w, voter, params)
	require.Equal(t, uint64(20_000_000), w.agent.GetParams(w.Ctx).MaxBlockGas)
	query := agentkeeper.Querier{Keeper: w.agent}
	response, err := query.Params(sdk.WrapSDKContext(w.Ctx), &agenttypes.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, params, response.Params)
	require.Equal(t, uint64(20_000_000), response.Params.MaxBlockGas)
}

func callServedAgentScheduler(t *testing.T, w *agentGovernanceTest, caller common.Address, value *big.Int, method string, args ...interface{}) ([]byte, error) {
	t.Helper()
	address := common.HexToAddress(scheduler.SchedulerAddress)
	served := w.App.EvmKeeper.CustomPrecompiles(w.Ctx)
	require.Contains(t, served, address)
	contract, ok := served[address].(*scheduler.SchedulerPrecompile)
	require.True(t, ok)
	input, err := contract.GetABI().Pack(method, args...)
	require.NoError(t, err)
	db := state.NewDBImpl(w.Ctx, &w.App.EvmKeeper, false)
	block, err := w.App.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(w.App.EvmKeeper.ChainID(db.Ctx()))
	evm := vm.NewEVM(*block, db, chain, vm.Config{}, served)
	amount, overflow := uint256.FromBig(value)
	require.False(t, overflow)
	output, _, callErr := evm.Call(caller, address, input, 2_000_000, amount)
	require.NoError(t, db.Err())
	_, err = db.Finalize()
	require.NoError(t, err)
	return output, callErr
}

func TestAgentPauseThroughProposalKeepsCancel(t *testing.T) {
	w, voter := newAgentGovernanceTest(t)
	address := common.HexToAddress(scheduler.SchedulerAddress)
	activationHeight := w.Ctx.BlockHeight() + 1
	w.App.UpgradeKeeper.SetDone(w.Ctx.WithBlockHeight(activationHeight), app.MergeUpgrade)
	require.NotContains(t, w.App.EvmKeeper.CustomPrecompiles(w.Ctx), address)
	w.Ctx = w.Ctx.WithBlockHeight(activationHeight)
	require.Contains(t, w.App.EvmKeeper.CustomPrecompiles(w.Ctx), address)
	w.App.EvmKeeper.SetNextBaseFeePerGas(w.Ctx, sdk.NewDec(1))
	creator := common.HexToAddress("0xa111")
	target := common.HexToAddress("0xc333")
	account := w.App.EvmKeeper.GetPaxAddressOrDefault(w.Ctx, creator)
	w.FundAcc(account, sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), sdk.NewInt(100))))
	balance := func() *big.Int { return w.App.EvmKeeper.GetBalance(w.Ctx, account) }
	escrow := func() *big.Int { return w.App.EvmKeeper.GetBalance(w.Ctx, w.agent.ModuleAddress()) }
	before := balance()
	deposit := big.NewInt(1_000_000_000_123)
	require.Zero(t, escrow().Sign())
	output, err := callServedAgentScheduler(t, w, creator, deposit, "schedule", target, []byte{1}, uint64(w.Ctx.BlockHeight()+10), uint64(100_000))
	require.NoError(t, err)
	contract := w.App.EvmKeeper.CustomPrecompiles(w.Ctx)[address].(*scheduler.SchedulerPrecompile)
	values, err := contract.GetABI().Methods["schedule"].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, values, 1)
	id := values[0].(*big.Int)
	require.Equal(t, uint64(1), id.Uint64())
	job, found := w.agent.GetJob(w.Ctx, id.Uint64())
	require.True(t, found)
	require.True(t, job.Active)
	require.Equal(t, creator, job.Creator)
	require.Equal(t, target, job.Target)
	require.Zero(t, job.Deposit.BigInt().Cmp(deposit))
	require.Zero(t, new(big.Int).Sub(before, balance()).Cmp(deposit))
	require.Zero(t, escrow().Cmp(deposit))

	params := w.agent.GetParams(w.Ctx)
	require.False(t, params.Paused)
	params.Paused = true
	passAgentParamsProposal(t, w, voter, params)
	require.True(t, w.agent.GetParams(w.Ctx).Paused)
	nextID := w.agent.NextJobID(w.Ctx)
	afterSchedule := balance()
	output, err = callServedAgentScheduler(t, w, creator, deposit, "schedule", target, []byte{1}, uint64(w.Ctx.BlockHeight()+10), uint64(100_000))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Equal(t, "LXP_PAUSED_SCOPE", strings.SplitN(reason, ":", 2)[0])
	require.Zero(t, balance().Cmp(afterSchedule))
	require.Zero(t, escrow().Cmp(deposit))
	require.Equal(t, nextID, w.agent.NextJobID(w.Ctx))
	stillPending, found := w.agent.GetJob(w.Ctx, id.Uint64())
	require.True(t, found)
	require.Equal(t, job, stillPending)
	require.Equal(t, []uint64{id.Uint64()}, w.agent.Pending(w.Ctx, creator))

	_, err = callServedAgentScheduler(t, w, creator, new(big.Int), "cancel", id)
	require.NoError(t, err)
	require.True(t, w.agent.GetParams(w.Ctx).Paused)
	_, found = w.agent.GetJob(w.Ctx, id.Uint64())
	require.False(t, found)
	require.Empty(t, w.agent.Pending(w.Ctx, creator))
	require.Zero(t, escrow().Sign())
	require.Zero(t, balance().Cmp(before))
	require.Zero(t, w.App.EvmKeeper.GetBalance(w.Ctx, w.App.EvmKeeper.GetPaxAddressOrDefault(w.Ctx, address)).Sign())
}
