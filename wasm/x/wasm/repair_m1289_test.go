package wasm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxcmd "github.com/Sidiora-Labs/Paxeer-X-Network/daemon/paxd/cmd"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdkclient "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	storev2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	wasmvm "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	wasmclient "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/client"
	wasmcli "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func repairM1289Code(t *testing.T, path string) []byte {
	t.Helper()
	code, err := os.ReadFile(path)
	require.NoError(t, err)
	return code
}

func repairM1289Open(t *testing.T, dir string) (sdk.Context, keeper.TestKeepers, *wasmvm.VM, func()) {
	t.Helper()
	sc := config.DefaultStateCommitConfig()
	sc.WriteMode = config.MemiavlOnly
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.MemIAVLConfig.SnapshotInterval = 1
	sc.MemIAVLConfig.SnapshotMinTimeInterval = 0
	ms := storev2.NewStore(dir, sc, config.StateStoreConfig{}, nil)
	vm, err := wasmvm.NewVM(filepath.Join(dir, "wasm"), "iterator,staking,stargate", 32, false, 32)
	require.NoError(t, err)
	ctx, k := keeper.CreateTestInputWithMultiStore(t, "iterator,staking,stargate", ms, keeper.WithWasmEngine(keeper.NewVMWrapper(vm)))
	if k.CapabilityKeeper.GetLatestIndex(ctx) == 0 {
		require.NoError(t, k.CapabilityKeeper.InitializeIndex(ctx, 1))
	}
	k.CapabilityKeeper.InitMemStore(ctx)
	closed := false
	closeStore := func() {
		if !closed {
			closed = true
			vm.Cleanup()
			require.NoError(t, ms.Close())
		}
	}
	t.Cleanup(closeStore)
	return ctx, k, vm, closeStore
}

// Generate the same unsigned transaction as the documented paxd proposal CLI,
// decode its protobuf content, then deliver that content to governance.
func repairM1289Proposal(t *testing.T, k keeper.TestKeepers, cmd *cobra.Command, args ...string) govtypes.Content {
	t.Helper()
	var out bytes.Buffer
	proposer := keeper.RandomAccountAddress(t)
	encoding := k.EncodingConfig
	clientCtx := sdkclient.Context{}.
		WithCodec(encoding.Marshaler).WithLegacyAmino(encoding.Amino).
		WithTxConfig(encoding.TxConfig).WithInterfaceRegistry(encoding.InterfaceRegistry).
		WithFrom(proposer.String()).WithFromAddress(proposer).WithKeyring(keyring.NewInMemory()).
		WithChainID("repair-1289").WithGenerateOnly(true).WithOffline(true).WithOutput(&out)
	flags.AddTxFlagsToCmd(cmd)
	cmd.SetContext(context.WithValue(context.Background(), sdkclient.ClientContextKey, &clientCtx))
	cmd.SetArgs(append(args, "--title", "Repair evidence", "--description", "Exercise registered Wasm governance", "--deposit", "1stake"))
	require.NoError(t, cmd.Execute())
	tx, err := encoding.TxConfig.TxJSONDecoder()(bytes.TrimSpace(out.Bytes()))
	require.NoError(t, err, out.String())
	require.Len(t, tx.GetMsgs(), 1)
	msg, ok := tx.GetMsgs()[0].(*govtypes.MsgSubmitProposal)
	require.True(t, ok)
	require.NoError(t, msg.ValidateBasic())
	return msg.GetContent()
}

func TestRepairM_1289_Governance(t *testing.T) {
	oldAll, oldSpecific := node.WasmProposalsEnabled, node.EnableSpecificWasmProposals
	t.Cleanup(func() { node.WasmProposalsEnabled, node.EnableSpecificWasmProposals = oldAll, oldSpecific })
	require.Equal(t, "true", oldAll)
	require.Empty(t, oldSpecific)
	require.Len(t, node.GetWasmEnabledProposals(), 10)
	doc := string(repairM1289Code(t, "Governance.md"))
	for _, kind := range node.GetWasmEnabledProposals() {
		require.Contains(t, doc, "`"+string(kind)+"`")
	}
	require.Contains(t, doc, "WasmProposalsEnabled")
	require.Contains(t, doc, "EnableSpecificWasmProposals")
	require.Len(t, wasmclient.ProposalHandlers, 10)
	names := make(map[string]bool)
	for _, handler := range wasmclient.ProposalHandlers {
		cmd := handler.CLIHandler()
		require.False(t, names[cmd.Name()], cmd.Name())
		names[cmd.Name()] = true
		require.Contains(t, doc, "`"+cmd.Name()+" ")
		require.NotNil(t, handler.RESTHandler)
	}
	node.WasmProposalsEnabled = "false"
	require.Empty(t, node.GetWasmEnabledProposals())
	node.EnableSpecificWasmProposals = "PinCodes,SudoContract"
	require.Equal(t, []types.ProposalType{types.ProposalTypePinCodes, types.ProposalTypeSudoContract}, node.GetWasmEnabledProposals())
	node.EnableSpecificWasmProposals = "PinCodes, SudoContract"
	require.Panics(t, func() { node.GetWasmEnabledProposals() })
	node.EnableSpecificWasmProposals = ""
	node.WasmProposalsEnabled = "true"

	ctx, k, _, closeStore := repairM1289Open(t, t.TempDir())
	defer closeStore()
	creator := k.Faucet.NewFundedAccount(ctx, sdk.NewInt64Coin("denom", 100))
	id, err := k.ContractKeeper.Create(ctx, creator, repairM1289Code(t, "keeper/testdata/hackatom.wasm"), nil)
	require.NoError(t, err)
	recipient := keeper.RandomAccountAddress(t)
	init, err := json.Marshal(keeper.HackatomExampleInitMsg{Verifier: creator, Beneficiary: recipient})
	require.NoError(t, err)
	contract, _, err := k.ContractKeeper.Instantiate(ctx, id, creator, creator, init, "governance evidence", sdk.NewCoins(sdk.NewInt64Coin("denom", 100)))
	require.NoError(t, err)
	pin := repairM1289Proposal(t, k, wasmcli.ProposalPinCodesCmd(), fmt.Sprint(id))
	require.Equal(t, string(types.ProposalTypePinCodes), pin.ProposalType())
	stored, err := k.GovKeeper.SubmitProposal(ctx, pin)
	require.NoError(t, err)
	require.False(t, k.WasmKeeper.IsPinnedCode(ctx, id), "submission must not execute the proposal")
	require.NoError(t, k.GovKeeper.Router().GetRoute(stored.ProposalRoute())(ctx, stored.GetContent()))
	require.True(t, k.WasmKeeper.IsPinnedCode(ctx, id))

	sudoMsg := fmt.Sprintf(`{"steal_funds":{"recipient":%q,"amount":[{"denom":"denom","amount":"75"}]}}`, recipient.String())
	_, err = k.ContractKeeper.Execute(ctx, contract, creator, []byte(sudoMsg), nil)
	require.Error(t, err, "ordinary execute cannot invoke the sudo entry point")
	require.Equal(t, sdk.NewInt(100), k.BankKeeper.GetBalance(ctx, contract, "denom").Amount)
	sudo := repairM1289Proposal(t, k, wasmcli.ProposalSudoContractCmd(), contract.String(), sudoMsg)
	require.Equal(t, string(types.ProposalTypeSudoContract), sudo.ProposalType())
	require.ErrorContains(t, keeper.NewWasmProposalHandler(k.WasmKeeper, []types.ProposalType{types.ProposalTypePinCodes})(ctx, sudo), "unsupported wasm proposal")
	stored, err = k.GovKeeper.SubmitProposal(ctx, sudo)
	require.NoError(t, err)
	require.NoError(t, k.GovKeeper.Router().GetRoute(stored.ProposalRoute())(ctx, stored.GetContent()))
	require.Equal(t, sdk.NewInt(25), k.BankKeeper.GetBalance(ctx, contract, "denom").Amount)
	require.Equal(t, sdk.NewInt(75), k.BankKeeper.GetBalance(ctx, recipient, "denom").Amount)
}

func TestRepairM_1289_GenesisValidatorLifecycle(t *testing.T) {
	cdc := keeper.MakeTestCodec(t)
	valid := types.GenesisState{Params: types.DefaultParams()}
	invalid := valid
	invalid.Sequences = []types.Sequence{{}}
	malformed := json.RawMessage(`{"params":`)
	var decoded types.GenesisState
	decodeErr := cdc.UnmarshalAsJSON(malformed, &decoded)
	require.Error(t, decodeErr)
	validRaw := cdc.MustMarshalJSON(&valid)
	invalidRaw := cdc.MustMarshalJSON(&invalid)
	invalidCh := make(chan types.GenesisState, 1)
	invalidCh <- invalid
	close(invalidCh)
	invalidErr := types.ValidateGenesisStream(invalidCh)
	require.Error(t, invalidErr)
	emptyCh := make(chan types.GenesisState)
	close(emptyCh)
	emptyErr := types.ValidateGenesisStream(emptyCh)
	require.Error(t, emptyErr)
	cases := []struct {
		name     string
		chunks   []json.RawMessage
		expected string
	}{
		{"valid early closure", []json.RawMessage{validRaw}, ""},
		{"empty early closure", nil, emptyErr.Error()},
		{"malformed middle", []json.RawMessage{validRaw, malformed, validRaw, invalidRaw}, decodeErr.Error()},
		{"malformed first", []json.RawMessage{malformed, validRaw}, decodeErr.Error()},
		{"validation failure", []json.RawMessage{validRaw, invalidRaw, validRaw}, invalidErr.Error()},
		{"decode error owns partial validation", []json.RawMessage{invalidRaw, malformed, validRaw}, decodeErr.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for iteration := 0; iteration < 40; iteration++ {
				input := make(chan json.RawMessage)
				producerDone := make(chan struct{})
				abort := make(chan struct{})
				var sent atomic.Int32
				go func() {
					defer close(producerDone)
					defer close(input)
					for _, raw := range tc.chunks {
						select {
						case input <- raw:
							sent.Add(1)
						case <-abort:
							return
						}
					}
				}()
				result := make(chan error, 1)
				go func() { result <- (wasm.AppModuleBasic{}).ValidateGenesisStream(cdc, nil, input) }()
				select {
				case err := <-result:
					close(abort)
					if tc.expected == "" {
						require.NoError(t, err)
					} else {
						require.EqualError(t, err, tc.expected)
					}
				case <-time.After(3 * time.Second):
					close(abort)
					t.Fatal("stream validator did not terminate")
				}
				select {
				case <-producerDone:
				case <-time.After(time.Second):
					t.Fatal("producer stranded after validation")
				}
				require.Equal(t, int32(len(tc.chunks)), sent.Load(), "validation must drain all producer chunks")
			}
		})
	}
	// The worker may still be leaving its final send instruction; require it to
	// terminate as well, rather than only observing the caller's return value.
	require.Eventually(t, func() bool {
		stacks := make([]byte, 1<<20)
		n := runtime.Stack(stacks, true)
		return !bytes.Contains(stacks[:n], []byte("wasm.AppModuleBasic.ValidateGenesisStream.func"))
	}, time.Second, time.Millisecond)
}

func TestRepairM_1289_SimulationConfigType(t *testing.T) {
	appTemplate, appConfig := paxcmd.InitAppConfig()
	parsed, err := template.New("app").Parse(appTemplate)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, raw string
		expected  uint64
	}{
		{"template default", "", 1234567},
		{"quoted template limit", "987654", 987654},
		{"largest limit", "18446744073709551615", math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appConfig.WASM.SimulationGasLimit = tc.raw
			var rendered bytes.Buffer
			require.NoError(t, parsed.Execute(&rendered, appConfig))
			opts := viper.New()
			opts.SetConfigType("toml")
			require.NoError(t, opts.ReadConfig(&rendered))
			require.Equal(t, tc.raw, opts.Get("wasm.simulation_gas_limit"))
			cfg, err := wasm.ReadWasmConfig(opts)
			require.NoError(t, err)
			require.Equal(t, uint64(300000), cfg.SmartQueryGasLimit)
			if tc.raw == "" {
				require.Nil(t, cfg.SimulationGasLimit)
			} else {
				require.Equal(t, tc.expected, *cfg.SimulationGasLimit)
			}
			repairM1289Meter(t, cfg, tc.expected)
		})
	}
	unset, err := wasm.ReadWasmConfig(viper.New())
	require.NoError(t, err)
	require.Nil(t, unset.SimulationGasLimit)
	repairM1289Meter(t, unset, 1234567)
	for _, raw := range []string{"0", "-1", "1.5", "18446744073709551616", "limit", " 100", "0x100"} {
		t.Run(raw, func(t *testing.T) {
			opts := viper.New()
			opts.Set("wasm.simulation_gas_limit", raw)
			cfg, err := wasm.ReadWasmConfig(opts)
			require.ErrorContains(t, err, "wasm.simulation_gas_limit")
			require.Nil(t, cfg.SimulationGasLimit)
		})
	}
	for _, line := range []string{"987654", "0", "-1", "1.5", "true", "[100]"} {
		t.Run("unsupported TOML "+line, func(t *testing.T) {
			opts := viper.New()
			opts.SetConfigType("toml")
			require.NoError(t, opts.ReadConfig(strings.NewReader("[wasm]\nsimulation_gas_limit = "+line+"\n")))
			cfg, err := wasm.ReadWasmConfig(opts)
			require.ErrorContains(t, err, "wasm.simulation_gas_limit: expected a quoted positive decimal integer")
			require.Nil(t, cfg.SimulationGasLimit)
		})
	}
}

func repairM1289Meter(t *testing.T, cfg types.WasmConfig, expected uint64) {
	t.Helper()
	ctx := sdk.Context{}.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 1234567}})
	decorator := keeper.NewLimitSimulationGasDecorator(cfg.SimulationGasLimit, keeper.DefaultGasMeterSetter())
	next := func(ctx sdk.Context, tx sdk.Tx, simulation bool) (sdk.Context, error) { return ctx, nil }
	simulated, err := decorator.AnteHandle(ctx, nil, true, next)
	require.NoError(t, err)
	require.Equal(t, expected, simulated.GasMeter().Limit())
	if expected < math.MaxUint64 {
		simulated.GasMeter().ConsumeGas(expected, "configured bound")
		require.PanicsWithValue(t, sdk.ErrorOutOfGas{Descriptor: "past configured bound"}, func() { simulated.GasMeter().ConsumeGas(1, "past configured bound") })
	}
	delivered, err := decorator.AnteHandle(ctx, nil, false, next)
	require.NoError(t, err)
	require.Same(t, ctx.GasMeter(), delivered.GasMeter())
	fallback := ctx.WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: -1}})
	if cfg.SimulationGasLimit == nil {
		simulated, err = decorator.AnteHandle(fallback, nil, true, next)
		require.NoError(t, err)
		require.Same(t, fallback.GasMeter(), simulated.GasMeter())
	}
}

func TestRepairM_1289_IBC(t *testing.T) {
	dir := t.TempDir()
	ctx, k, vm, closeStore := repairM1289Open(t, dir)
	actor := keeper.RandomAccountAddress(t)
	reflectID, err := k.ContractKeeper.Create(ctx, actor, repairM1289Code(t, "keeper/testdata/reflect.wasm"), nil)
	require.NoError(t, err)
	ordinary, _, err := k.ContractKeeper.Instantiate(ctx, reflectID, actor, actor, []byte(`{}`), "ordinary reflect", nil)
	require.NoError(t, err)
	require.Empty(t, k.WasmKeeper.GetContractInfo(ctx, ordinary).IBCPortID)
	ibcID, err := k.ContractKeeper.Create(ctx, actor, repairM1289Code(t, "../../../wasm-runtime/testdata/ibc_reflect.wasm"), nil)
	require.NoError(t, err)
	report, err := vm.AnalyzeCode(k.WasmKeeper.GetCodeInfo(ctx, ibcID).CodeHash)
	require.NoError(t, err)
	require.True(t, report.HasIBCEntryPoints)
	contract, _, err := k.ContractKeeper.Instantiate(ctx, ibcID, actor, actor, keeper.IBCReflectInitMsg{ReflectCodeID: reflectID}.GetBytes(t), "IBC evidence", nil)
	require.NoError(t, err)
	port := k.WasmKeeper.GetContractInfo(ctx, contract).IBCPortID
	require.Equal(t, keeper.PortIDForContract(contract), port)
	module, portCap, err := k.IBCKeeper.PortKeeper.LookupModuleByPort(ctx, port)
	require.NoError(t, err)
	require.Equal(t, types.ModuleName, module)
	require.True(t, k.WasmKeeper.AuthenticateCapability(ctx, portCap, host.PortPath(port)))

	handler := wasm.NewIBCHandler(k.WasmKeeper, k.IBCKeeper.ChannelKeeper)
	counterparty := channeltypes.NewCounterparty("wasm.remote", "channel-1")
	cap, err := k.ScopedWasmKeeper.NewCapability(ctx, host.ChannelCapabilityPath(port, "channel-0"))
	require.NoError(t, err)
	_, err = handler.OnChanOpenTry(ctx, channeltypes.ORDERED, []string{"connection-0"}, port, "channel-0", cap, counterparty, "wrong-version")
	require.Error(t, err)
	version, err := handler.OnChanOpenTry(ctx, channeltypes.ORDERED, []string{"connection-0"}, port, "channel-0", cap, counterparty, "ibc-reflect-v1")
	require.NoError(t, err)
	require.Equal(t, "ibc-reflect-v1", version)
	k.IBCKeeper.ChannelKeeper.WriteOpenTryChannel(ctx, port, "channel-0", channeltypes.ORDERED, []string{"connection-0"}, counterparty, version)
	require.NoError(t, handler.OnChanOpenConfirm(ctx, port, "channel-0"))
	before, err := k.WasmKeeper.QuerySmart(ctx, contract, []byte(`{"list_accounts":{}}`))
	require.NoError(t, err)
	// Use the production example's query transport shape.
	var list struct {
		Accounts []struct {
			Account   string `json:"account"`
			ChannelID string `json:"channel_id"`
		} `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(before, &list))
	require.Len(t, list.Accounts, 1)
	require.Equal(t, "channel-0", list.Accounts[0].ChannelID)
	require.NotEmpty(t, list.Accounts[0].Account)
	packet := channeltypes.NewPacket([]byte(`{"who_am_i":{}}`), 1, counterparty.PortId, counterparty.ChannelId, port, "channel-0", clienttypes.NewHeight(1, 9999999), 0)
	ack := handler.OnRecvPacket(ctx, packet, actor)
	require.True(t, ack.Success())
	require.Contains(t, string(ack.Acknowledgement()), list.Accounts[0].Account)
	// For source callbacks the local contract is selected from the source port.
	outgoing := channeltypes.NewPacket(packet.Data, 1, port, "channel-0", counterparty.PortId, counterparty.ChannelId, packet.TimeoutHeight, 0)
	require.NoError(t, handler.OnAcknowledgementPacket(ctx, outgoing, ack.Acknowledgement(), actor))
	require.NoError(t, handler.OnTimeoutPacket(ctx, outgoing, actor))
	require.NoError(t, keeper.NewGovPermissionKeeper(k.WasmKeeper).PinCode(ctx, ibcID))
	ctx.MultiStore().(sdk.CacheMultiStore).Write()
	k.MultiStore.Commit(true)
	closeStore()

	ctx, k, vm, closeStore = repairM1289Open(t, dir)
	defer closeStore()
	require.Equal(t, port, k.WasmKeeper.GetContractInfo(ctx, contract).IBCPortID)
	module, recoveredPort, err := k.IBCKeeper.PortKeeper.LookupModuleByPort(ctx, port)
	require.NoError(t, err)
	require.Equal(t, types.ModuleName, module)
	require.True(t, k.WasmKeeper.AuthenticateCapability(ctx, recoveredPort, host.PortPath(port)))
	recoveredChannel, ok := k.ScopedWasmKeeper.GetCapability(ctx, host.ChannelCapabilityPath(port, "channel-0"))
	require.True(t, ok)
	require.True(t, k.WasmKeeper.AuthenticateCapability(ctx, recoveredChannel, host.ChannelCapabilityPath(port, "channel-0")))
	metrics, err := vm.GetMetrics()
	require.NoError(t, err)
	require.Zero(t, metrics.ElementsPinnedMemoryCache)
	require.NoError(t, k.WasmKeeper.InitializePinnedCodes(ctx))
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	after, err := k.WasmKeeper.QuerySmart(ctx, contract, []byte(`{"list_accounts":{}}`))
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
	handler = wasm.NewIBCHandler(k.WasmKeeper, k.IBCKeeper.ChannelKeeper)
	recoveredAck := handler.OnRecvPacket(ctx, packet, actor)
	require.True(t, recoveredAck.Success())
	require.JSONEq(t, string(ack.Acknowledgement()), string(recoveredAck.Acknowledgement()))
	doc := string(repairM1289Code(t, "IBC.md"))
	for _, term := range []string{"AnalyzeCode", "HasIBCEntryPoints", "counterparty_version", "OnAcknowledgementPacket", "OnTimeoutPacket", "InitMemStore", "InitializePinnedCodes"} {
		require.Contains(t, doc, term)
	}
}
