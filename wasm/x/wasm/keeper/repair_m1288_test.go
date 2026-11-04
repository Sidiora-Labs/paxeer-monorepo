package keeper_test

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	vmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types"
	wasm "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1288_GasOverflow(t *testing.T) {
	for _, multiplier := range []uint64{1, 3, keeper.DefaultGasMultiplier, math.MaxUint64} {
		t.Run(fmt.Sprint(multiplier), func(t *testing.T) {
			cfg := keeper.DefaultGasRegisterConfig()
			cfg.GasMultiplier = multiplier
			g := keeper.NewWasmGasRegister(cfg)
			max := uint64(math.MaxUint64) / multiplier
			require.Equal(t, max*multiplier, g.ToWasmVMGas(max))
			require.Zero(t, g.ToWasmVMGas(0))
			if max < math.MaxUint64 {
				require.PanicsWithValue(t, sdk.ErrorOutOfGas{Descriptor: "overflow"}, func() { g.ToWasmVMGas(max + 1) })
			}
		})
	}
	g := keeper.NewDefaultWasmGasRegister()
	var wrappedInput uint64 = 1 << 40
	require.Greater(t, wrappedInput*keeper.DefaultGasMultiplier, wrappedInput, "the old less-than check missed this wraparound")
	require.PanicsWithValue(t, sdk.ErrorOutOfGas{Descriptor: "overflow"}, func() { g.ToWasmVMGas(wrappedInput) })
	cfg := keeper.DefaultGasRegisterConfig()
	cfg.GasMultiplier = 0
	require.Panics(t, func() { keeper.NewWasmGasRegister(cfg) })
	require.Panics(t, func() { (keeper.WasmGasRegister{}).ToWasmVMGas(0) })
}

func TestRepairM_1288_GovEncoderOverride(t *testing.T) {
	var calls int
	var contract sdk.AccAddress
	recipient := keeper.RandomAccountAddress(t)
	encoders := &keeper.MessageEncoders{Gov: func(sender sdk.AccAddress, msg *vmtypes.GovMsg) ([]sdk.Msg, error) {
		calls++
		require.Equal(t, contract, sender)
		require.Equal(t, uint64(42), msg.Vote.ProposalId)
		return []sdk.Msg{&banktypes.MsgSend{FromAddress: sender.String(), ToAddress: recipient.String(), Amount: sdk.NewCoins(sdk.NewInt64Coin("stake", 7))}}, nil
	}}
	ctx, keepers := keeper.CreateTestInput(t, false, keeper.SupportedFeatures, keeper.WithMessageEncoders(encoders))
	admin := keeper.RandomAccountAddress(t)
	id := keeper.StoreReflectContract(t, ctx, keepers)
	var err error
	contract, _, err = keepers.ContractKeeper.Instantiate(ctx, id, admin, admin, []byte(`{}`), "gov override", nil)
	require.NoError(t, err)
	keepers.Faucet.Fund(ctx, contract, sdk.NewInt64Coin("stake", 7))
	_, err = keepers.ContractKeeper.Execute(ctx, contract, admin, []byte(`{"reflect_msg":{"msgs":[{"gov":{"vote":{"proposal_id":42,"vote":"yes"}}}]}}`), nil)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, sdk.NewInt(7), keepers.BankKeeper.GetBalance(ctx, recipient, "stake").Amount)
	require.True(t, keepers.BankKeeper.GetBalance(ctx, contract, "stake").IsZero())

	for _, tc := range []struct {
		vote     vmtypes.VoteMsg
		expected govtypes.VoteOption
	}{
		{vmtypes.VoteMsg{ProposalId: 42, Vote: vmtypes.Yes}, govtypes.OptionYes},
		{vmtypes.VoteMsg{ProposalId: 42, Vote: vmtypes.No}, govtypes.OptionNo},
		{vmtypes.VoteMsg{ProposalId: 42, Vote: vmtypes.NoWithVeto}, govtypes.OptionNoWithVeto},
		{vmtypes.VoteMsg{ProposalId: 42, Vote: vmtypes.Abstain}, govtypes.OptionAbstain},
	} {
		msgs, err := keeper.DefaultEncoders(nil, nil).Encode(ctx, contract, "", vmtypes.CosmosMsg{Gov: &vmtypes.GovMsg{Vote: &tc.vote}}, vmtypes.MessageInfo{}, types.CodeInfo{})
		require.NoError(t, err)
		require.Equal(t, []sdk.Msg{&govtypes.MsgVote{ProposalId: 42, Voter: contract.String(), Option: tc.expected}}, msgs)
	}
	for _, msg := range []*vmtypes.GovMsg{{}, {Vote: &vmtypes.VoteMsg{ProposalId: 42}}, {Vote: &vmtypes.VoteMsg{ProposalId: 42, Vote: 99}}, {VoteWeighted: &vmtypes.VoteWeightedMsg{ProposalId: 42, Options: []vmtypes.WeightedVoteOption{{Option: vmtypes.Yes, Weight: "1.0"}}}}} {
		msgs, err := keeper.DefaultEncoders(nil, nil).Encode(ctx, contract, "", vmtypes.CosmosMsg{Gov: msg}, vmtypes.MessageInfo{}, types.CodeInfo{})
		require.ErrorIs(t, err, types.ErrUnknownMsg)
		require.Nil(t, msgs)
	}
}

func TestRepairM_1288_HistoryQuery(t *testing.T) {
	// Reuse the committed real instantiate/migrate/commit/reopen lifecycle.
	TestRepair_184_HistorySequence(t)
	ctx, keepers := keeper.CreateTestInput(t, false, keeper.SupportedFeatures)
	var history types.QueryContractHistoryResponse
	repair184Query(t, ctx, keepers, "ContractHistory", &types.QueryContractHistoryRequest{Address: keeper.RandomBech32AccountAddress(t)}, &history)
	require.Empty(t, history.Entries, "unknown addresses have an empty history, not a fabricated error")
}

func TestRepairM_1288_IBCBeforeMetadata(t *testing.T) {
	dir := t.TempDir()
	ctx, keepers, closeStore := repair184Open(t, dir)
	admin := keeper.RandomAccountAddress(t)
	id, err := keepers.ContractKeeper.Create(ctx, admin, repair184Code(t, "testdata/repair_m1288_ibc.wasm"), nil)
	require.NoError(t, err)
	explicit := channeltypes.Channel{State: channeltypes.OPEN, Ordering: channeltypes.ORDERED, Counterparty: channeltypes.Counterparty{PortId: "remote", ChannelId: "channel-9"}, ConnectionHops: []string{"connection-0"}, Version: "m1288"}
	keepers.IBCKeeper.ChannelKeeper.SetChannel(ctx, "transfer", "channel-0", explicit)
	requests := [][]byte{[]byte(`{"ibc":{"port_id":{}}}`), []byte(`{"ibc":{"channel":{"channel_id":"channel-0"}}}`), []byte(`{"ibc":{"channel":{"channel_id":"channel-0","port_id":"transfer"}}}`)}
	var contracts []sdk.AccAddress
	for i, request := range requests {
		addr, _, err := keepers.ContractKeeper.Instantiate(ctx, id, admin, admin, request, fmt.Sprintf("prebinding %d", i), nil)
		require.NoError(t, err, "instantiate must run the contract's native query_chain callback")
		contracts = append(contracts, addr)
		var answer vmtypes.QuerierResult
		require.NoError(t, json.Unmarshal(keepers.WasmKeeper.QueryRaw(ctx, addr, []byte("init_query")), &answer))
		require.Nil(t, answer.Err)
		require.NotNil(t, answer.Ok)
		require.Empty(t, answer.Ok.Err)
		switch i {
		case 0:
			require.JSONEq(t, `{"port_id":""}`, string(answer.Ok.Ok))
		case 1:
			require.JSONEq(t, `{}`, string(answer.Ok.Ok))
		case 2:
			var channel vmtypes.ChannelResponse
			require.NoError(t, json.Unmarshal(answer.Ok.Ok, &channel))
			require.NotNil(t, channel.Channel)
			require.Equal(t, "transfer", channel.Channel.Endpoint.PortID)
		}
		port := keepers.WasmKeeper.GetContractInfo(ctx, addr).IBCPortID
		require.Equal(t, keeper.PortIDForContract(addr), port)
		keepers.IBCKeeper.ChannelKeeper.SetChannel(ctx, port, "channel-0", explicit)
	}
	checkBound := func() {
		for _, addr := range contracts {
			port := keepers.WasmKeeper.GetContractInfo(ctx, addr).IBCPortID
			for i, request := range requests {
				bz, err := keepers.WasmKeeper.QuerySmart(ctx, addr, request)
				require.NoError(t, err)
				if i == 0 {
					require.JSONEq(t, fmt.Sprintf(`{"port_id":%q}`, port), string(bz))
				} else {
					var response vmtypes.ChannelResponse
					require.NoError(t, json.Unmarshal(bz, &response))
					require.NotNil(t, response.Channel)
					expectedPort := port
					if i == 2 {
						expectedPort = "transfer"
					}
					require.Equal(t, expectedPort, response.Channel.Endpoint.PortID)
					require.Equal(t, "channel-0", response.Channel.Endpoint.ChannelID)
				}
			}
		}
	}
	checkBound()
	keepers.MultiStore.Commit(true)
	closeStore()
	ctx, keepers, closeStore = repair184Open(t, dir)
	defer closeStore()
	checkBound()
}

func TestRepairM_1288_IBCGas(t *testing.T) {
	TestRepair_184_IBCGasUnits(t)
}

func TestRepairM_1288_SnapshotRecovery(t *testing.T) {
	repairM1288SnapshotRecovery(t)
}

func TestRepairM_1288_HandlerResponses(t *testing.T) {
	ctx, keepers := keeper.CreateTestInput(t, false, keeper.SupportedFeatures)
	admin := keeper.RandomAccountAddress(t)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(keepers.EncodingConfig.InterfaceRegistry)
	types.RegisterMsgServer(router, keeper.NewMsgServerImpl(keepers.ContractKeeper))
	production, helper := wasm.NewHandler(keepers.ContractKeeper), keeper.TestHandler(keepers.ContractKeeper)
	compare := func(msg sdk.Msg, response proto.Message) *sdk.Result {
		t.Helper()
		var reference *sdk.Result
		for _, handler := range []sdk.Handler{production, helper, sdk.Handler(router.Handler(msg))} {
			branch, _ := ctx.CacheContext()
			result, err := handler(branch, msg)
			require.NoError(t, err)
			if reference == nil {
				reference = result
			} else {
				require.Equal(t, reference.Data, result.Data)
			}
			require.NoError(t, proto.Unmarshal(result.Data, response))
		}
		return reference
	}
	store := &types.MsgStoreCode{Sender: admin.String(), WASMByteCode: repair184Code(t, "testdata/reflect.wasm")}
	var stored types.MsgStoreCodeResponse
	compare(store, &stored)
	require.Equal(t, uint64(1), stored.CodeID)
	_, err := router.Handler(store)(ctx, store)
	require.NoError(t, err)
	instantiate := &types.MsgInstantiateContract{Sender: admin.String(), Admin: admin.String(), CodeID: stored.CodeID, Label: "reply bytes", Msg: []byte(`{}`)}
	var instantiated types.MsgInstantiateContractResponse
	compare(instantiate, &instantiated)
	result, err := router.Handler(instantiate)(ctx, instantiate)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(result.Data, &instantiated))
	addr, err := sdk.AccAddressFromBech32(instantiated.Address)
	require.NoError(t, err)
	nextOwner := keeper.RandomAccountAddress(t)
	change := &types.MsgExecuteContract{Sender: admin.String(), Contract: addr.String(), Msg: repair184JSON(t, map[string]interface{}{"change_owner": map[string]string{"owner": nextOwner.String()}})}
	var executed types.MsgExecuteContractResponse
	compare(change, &executed)

	// Real reflect submessages store the protobuf responses in native replies.
	queryReply := func(id uint64, response proto.Message, expected []byte) {
		t.Helper()
		replyBz, err := keepers.WasmKeeper.QuerySmart(ctx, addr, repair184JSON(t, map[string]interface{}{"sub_msg_result": map[string]uint64{"id": id}}))
		require.NoError(t, err)
		var reply vmtypes.Reply
		require.NoError(t, json.Unmarshal(replyBz, &reply))
		require.Equal(t, id, reply.ID)
		require.NotNil(t, reply.Result.Ok)
		require.Empty(t, reply.Result.Err)
		require.Equal(t, expected, reply.Result.Ok.Data)
		require.NoError(t, proto.Unmarshal(reply.Result.Ok.Data, response))
	}
	dispatch := func(id uint64, msg vmtypes.CosmosMsg) {
		t.Helper()
		submessage := vmtypes.SubMsg{ID: id, ReplyOn: vmtypes.ReplySuccess, Msg: msg}
		_, err := keepers.ContractKeeper.Execute(ctx, addr, admin, repair184JSON(t, map[string]interface{}{"reflect_sub_msg": map[string]interface{}{"msgs": []vmtypes.SubMsg{submessage}}}), nil)
		require.NoError(t, err)
	}
	code := &types.MsgStoreCode{Sender: addr.String(), WASMByteCode: repair184Code(t, "../../../../wasm-runtime/testdata/hackatom.wasm")}
	var replyStore types.MsgStoreCodeResponse
	expectedStore := compare(code, &replyStore)
	dispatch(1288, vmtypes.CosmosMsg{Stargate: &vmtypes.StargateMsg{TypeURL: sdk.MsgTypeURL(code), Value: keepers.EncodingConfig.Marshaler.MustMarshal(code)}})
	queryReply(1288, &replyStore, expectedStore.Data)
	require.Equal(t, uint64(2), replyStore.CodeID)

	keepers.Faucet.Fund(ctx, addr, sdk.NewInt64Coin("denom", 5))
	initMsg := repair184JSON(t, map[string]string{"verifier": addr.String(), "beneficiary": admin.String()})
	childInit := &types.MsgInstantiateContract{Sender: addr.String(), CodeID: replyStore.CodeID, Label: "native reply child", Msg: initMsg, Funds: sdk.NewCoins(sdk.NewInt64Coin("denom", 3))}
	var replyInit types.MsgInstantiateContractResponse
	expectedInit := compare(childInit, &replyInit)
	dispatch(1289, vmtypes.CosmosMsg{Wasm: &vmtypes.WasmMsg{Instantiate: &vmtypes.InstantiateMsg{CodeID: childInit.CodeID, Label: childInit.Label, Msg: initMsg, Funds: []vmtypes.Coin{{Denom: "denom", Amount: "3"}}}}})
	queryReply(1289, &replyInit, expectedInit.Data)
	require.NotEmpty(t, replyInit.Address)
	child, err := sdk.AccAddressFromBech32(replyInit.Address)
	require.NoError(t, err)
	require.NotNil(t, keepers.WasmKeeper.GetContractInfo(ctx, child))
	childExecute := &types.MsgExecuteContract{Sender: addr.String(), Contract: child.String(), Msg: []byte(`{"release":{}}`)}
	var replyExecute types.MsgExecuteContractResponse
	expectedExecute := compare(childExecute, &replyExecute)
	require.Equal(t, []byte{0xf0, 0x0b, 0xaa}, replyExecute.Data)
	dispatch(1290, vmtypes.CosmosMsg{Wasm: &vmtypes.WasmMsg{Execute: &vmtypes.ExecuteMsg{ContractAddr: child.String(), Msg: childExecute.Msg}}})
	queryReply(1290, &replyExecute, expectedExecute.Data)
	require.Equal(t, []byte{0xf0, 0x0b, 0xaa}, replyExecute.Data)
	require.Equal(t, sdk.NewInt(3), keepers.BankKeeper.GetBalance(ctx, admin, "denom").Amount)
}
