package scheduler_test

import (
	"math/big"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestSchedulerMergeConstructorNeedsAgentKeeper(t *testing.T) {
	application := testkeeper.EVMTestApp
	keepers := app.NewPrecompileKeepers(application)
	var _ scheduler.LayerXAgentKeepers = keepers
	missingEVM := app.NewPrecompileKeepers(application)
	missingEVM.EVMKeeper = nil
	nilEVM := app.NewPrecompileKeepers(application)
	nilEVM.EVMKeeper = (*evmkeeper.Keeper)(nil)
	for _, tc := range []struct {
		name    string
		keepers utils.Keepers
	}{
		{"nil keepers", nil},
		{"missing agent capability", struct{ utils.Keepers }{keepers}},
		{"nil agent pointer", &app.PrecompileKeepers{EVMKeeper: &application.EvmKeeper}},
		{"missing evm", missingEVM},
		{"nil evm pointer", nilEVM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := scheduler.NewMergePrecompile(tc.keepers, precompiles.MergeUpgrade)
			require.Nil(t, p)
			require.ErrorIs(t, err, agentcommon.ErrNoContext)
			require.Contains(t, err.Error(), "LXP_PRECOMPILE_NO_CONTEXT")
		})
	}
	p, err := scheduler.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	address := common.HexToAddress(scheduler.SchedulerAddress)
	require.Equal(t, address, p.Address())
	require.Equal(t, "scheduler", p.GetName())
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address)
	}
	served, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, keepers)
	require.NoError(t, err)
	require.Contains(t, served, address)
	require.Equal(t, address, served[address][precompiles.MergeUpgrade].(precompiles.IPrecompile).Address())
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, scheduler.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")
}

func TestSchedulerJobOutcomesAreTypedEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		code   []byte
		gas    uint64
		failed bool
	}{
		{"executed", []byte{byte(vm.STOP)}, 21_000, false},
		{"failed", []byte{byte(vm.PUSH1), 0, byte(vm.PUSH1), 0, byte(vm.REVERT)}, 21_006, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newSchedulerHarness(t)
			application := *testkeeper.EVMTestApp
			application.LayerXAgentKeeper.Keeper = h.keeper
			built, err := scheduler.NewMergePrecompile(application.GetPrecompileKeepers(), schedulerPlan)
			require.NoError(t, err)
			h.precompile = built.(*scheduler.SchedulerPrecompile)
			h.evm.SetPrecompiles(map[common.Address]vm.PrecompiledContract{built.Address(): built})
			a := built.GetABI()
			require.Equal(t, common.HexToHash("0x2052025f45e2d6b12cb8a34d6b7ff4754b5cedda17c3781b420f341933da273b"), a.Events["JobExecuted"].ID)
			require.Equal(t, common.HexToHash("0x0bf83276f88d28dd259bbe90cade9458a526aa69f119c31d6a6f511e0ef46cf7"), a.Events["JobFailed"].ID)
			for _, name := range []string{"JobExecuted", "JobFailed"} {
				event := a.Events[name]
				require.False(t, event.Anonymous)
				require.True(t, event.Inputs[0].Indexed)
				require.True(t, event.Inputs[1].Indexed)
			}
			testkeeper.EVMTestApp.EvmKeeper.SetCode(h.stateDB.Ctx(), h.target, tc.code)
			id := h.write(big.NewInt(200_000), "schedule", h.target, []byte{}, uint64(schedulerHeight+10), schedulerGas)[0].(*big.Int)
			require.Len(t, h.stateDB.GetAllLogs(), 1)
			h.log("JobScheduled", scheduledTopic, id, &h.creator, h.target, uint64(schedulerHeight+10), schedulerGas, big.NewInt(200_000))
			_, err = h.stateDB.Finalize()
			require.NoError(t, err)
			end := h.stateDB.Ctx().WithBlockHeight(schedulerHeight + 10).WithEventManager(sdk.NewEventManager())
			gas, err := h.keeper.RunDue(end)
			require.NoError(t, err)
			require.Equal(t, tc.gas, gas)
			var outcomes []proto.Message
			for _, event := range end.EventManager().Events() {
				require.NotEqual(t, "JobExecuted", event.Type)
				require.NotEqual(t, "JobFailed", event.Type)
				if event.Type == proto.MessageName(&agenttypes.EventJobExecuted{}) || event.Type == proto.MessageName(&agenttypes.EventJobFailed{}) {
					parsed, err := sdk.ParseTypedEvent(abci.Event(event))
					require.NoError(t, err)
					outcomes = append(outcomes, parsed)
				}
			}
			require.Len(t, outcomes, 1)
			if tc.failed {
				require.Equal(t, &agenttypes.EventJobFailed{JobId: id.Uint64(), Creator: h.creator.Hex(), Reason: "execution reverted"}, outcomes[0])
			} else {
				require.Equal(t, &agenttypes.EventJobExecuted{JobId: id.Uint64(), Creator: h.creator.Hex(), GasUsed: tc.gas, Refund: "179000"}, outcomes[0])
			}
			require.Len(t, h.stateDB.GetAllLogs(), 1)
			for _, log := range h.stateDB.GetAllLogs() {
				require.NotEqual(t, a.Events["JobExecuted"].ID, log.Topics[0])
				require.NotEqual(t, a.Events["JobFailed"].ID, log.Topics[0])
			}
			_, found := h.keeper.GetJob(end, id.Uint64())
			require.False(t, found)
			require.Empty(t, h.keeper.Pending(end, h.creator))
			require.Zero(t, testkeeper.EVMTestApp.EvmKeeper.GetBalance(end, h.keeper.ModuleAddress()).Sign())
		})
	}
}
