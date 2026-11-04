package keeper_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	genesistypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/genesis"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	wasmvm "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime"
	vmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types"
	wasm "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair184Code(t *testing.T, path string) []byte {
	t.Helper()
	code, err := os.ReadFile(path)
	require.NoError(t, err)
	return code
}

func repair184JSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

func repair184Query(t *testing.T, ctx sdk.Context, keepers keeper.TestKeepers, method string, request, response codec.ProtoMarshaler) {
	t.Helper()
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(keepers.EncodingConfig.InterfaceRegistry)
	types.RegisterQueryServer(router, keeper.Querier(keepers.WasmKeeper))
	path := "/cosmwasm.wasm.v1.Query/" + method
	handler := router.Route(path)
	require.NotNil(t, handler)
	result, err := handler(ctx, abci.RequestQuery{Path: path, Data: keepers.EncodingConfig.Marshaler.MustMarshal(request)})
	require.NoError(t, err)
	require.NoError(t, keepers.EncodingConfig.Marshaler.Unmarshal(result.Value, response))
}

func repair184Open(t *testing.T, dir string) (sdk.Context, keeper.TestKeepers, func()) {
	t.Helper()
	db, err := dbm.NewDB("state", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	vm, err := wasmvm.NewVM(filepath.Join(dir, "wasm"), "iterator,staking,stargate", 32, false, 32)
	require.NoError(t, err)
	ctx, keepers := keeper.Repair184PersistentInput(t, db, keeper.WithWasmEngine(keeper.NewVMWrapper(vm)))
	return ctx, keepers, func() {
		vm.Cleanup()
		require.NoError(t, keepers.MultiStore.Close())
	}
}

func TestRepair_184_HistorySequence(t *testing.T) {
	dir := t.TempDir()
	ctx, keepers, closeStore := repair184Open(t, dir)
	instance := keeper.InstantiateHackatomExampleContract(t, ctx, keepers)
	addr, admin := instance.Contract, instance.CreatorAddr
	codeIDs := []uint64{instance.CodeID}
	keepers.MultiStore.Commit(true)
	for i := 0; i < 3; i++ {
		ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithEventManager(sdk.NewEventManager())
		id, err := keepers.ContractKeeper.Create(ctx, admin, repair184Code(t, "testdata/hackatom.wasm"), nil)
		require.NoError(t, err)
		codeIDs = append(codeIDs, id)
		_, err = keepers.ContractKeeper.Migrate(ctx, addr, admin, id, repair184JSON(t, map[string]string{"verifier": admin.String()}))
		require.NoError(t, err)
		keepers.MultiStore.Commit(true)
		var history types.QueryContractHistoryResponse
		repair184Query(t, ctx, keepers, "ContractHistory", &types.QueryContractHistoryRequest{Address: addr.String()}, &history)
		require.Len(t, history.Entries, len(codeIDs))
		for j, expectedID := range codeIDs {
			require.Equal(t, expectedID, history.Entries[j].CodeID)
		}
		for j, expectedID := range codeIDs {
			var contracts types.QueryContractsByCodeResponse
			repair184Query(t, ctx, keepers, "ContractsByCode", &types.QueryContractsByCodeRequest{CodeId: expectedID}, &contracts)
			if j == len(codeIDs)-1 {
				require.Equal(t, []string{addr.String()}, contracts.Contracts)
			} else {
				require.Empty(t, contracts.Contracts)
			}
		}
		if i == 1 {
			height := ctx.BlockHeight()
			closeStore()
			ctx, keepers, closeStore = repair184Open(t, dir)
			ctx = ctx.WithBlockHeight(height)
		}
	}
	closeStore()
	ctx, keepers, closeStore = repair184Open(t, dir)
	defer closeStore()
	var history types.QueryContractHistoryResponse
	repair184Query(t, ctx, keepers, "ContractHistory", &types.QueryContractHistoryRequest{Address: addr.String()}, &history)
	require.Len(t, history.Entries, 4)
	require.Equal(t, types.ContractCodeHistoryOperationTypeInit, history.Entries[0].Operation)
	for i, entry := range history.Entries {
		require.Equal(t, codeIDs[i], entry.CodeID)
		if i > 0 {
			require.Equal(t, types.ContractCodeHistoryOperationTypeMigrate, entry.Operation)
		}
	}
	var state types.QuerySmartContractStateResponse
	repair184Query(t, ctx, keepers, "SmartContractState", &types.QuerySmartContractStateRequest{Address: addr.String(), QueryData: []byte(`{"verifier":{}}`)}, &state)
	require.JSONEq(t, fmt.Sprintf(`{"verifier":%q}`, admin.String()), string(state.Data))
}

func TestRepair_184_HistoryRejectsCorruptionAndExhaustion(t *testing.T) {
	ctx, keepers := keeper.CreateTestInput(t, false, "iterator,staking,stargate")
	addr := keeper.RandomAccountAddress(t)
	entry := types.ContractCodeHistoryEntry{CodeID: 1}
	keeper.Repair184AppendHistory(keepers.WasmKeeper, ctx, addr, entry)
	keeper.Repair184AppendHistory(keepers.WasmKeeper, ctx, addr, entry)
	require.Len(t, keepers.WasmKeeper.GetContractHistory(ctx, addr), 2, "short protobuf entries must not panic or supply the sequence")
	store := keeper.Repair184Store(keepers.WasmKeeper, ctx)
	badAddr := keeper.RandomAccountAddress(t)
	badKey := append(types.GetContractCodeHistoryElementPrefix(badAddr), byte(1))
	store.Set(badKey, keepers.EncodingConfig.Marshaler.MustMarshal(&entry))
	require.PanicsWithValue(t, "invalid contract history sequence; restore history from verified transaction records before upgrading", func() { keeper.Repair184AppendHistory(keepers.WasmKeeper, ctx, badAddr, entry) })
	exhausted := keeper.RandomAccountAddress(t)
	store.Set(types.GetContractCodeHistoryElementKey(exhausted, math.MaxUint64), keepers.EncodingConfig.Marshaler.MustMarshal(&entry))
	require.PanicsWithValue(t, "contract history sequence exhausted", func() { keeper.Repair184AppendHistory(keepers.WasmKeeper, ctx, exhausted, entry) })
	require.False(t, store.Has(types.GetContractCodeHistoryElementKey(exhausted, 0)))
	instance := keeper.InstantiateHackatomExampleContract(t, ctx, keepers)
	first := types.GetContractCodeHistoryElementKey(instance.Contract, 1)
	store.Set(types.GetContractCodeHistoryElementKey(instance.Contract, 7), bytes.Clone(store.Get(first)))
	store.Delete(first)
	_, err := keepers.ContractKeeper.Migrate(ctx, instance.Contract, instance.CreatorAddr, instance.CodeID, repair184JSON(t, map[string]string{"verifier": instance.CreatorAddr.String()}))
	require.ErrorContains(t, err, "restore verified history")
	require.Equal(t, instance.CodeID, keepers.WasmKeeper.GetContractInfo(ctx, instance.Contract).CodeID)
}

type repair184CompileBarrier struct {
	types.WasmerEngine
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *repair184CompileBarrier) Create(code wasmvm.WasmCode) (wasmvm.Checksum, error) {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return b.WasmerEngine.Create(code)
}

func TestRepair_184_CompileWallClock(t *testing.T) {
	vm, err := wasmvm.NewVM(t.TempDir(), "iterator,staking,stargate", 32, false, 32)
	require.NoError(t, err)
	defer vm.Cleanup()
	code := repair184Code(t, "testdata/hackatom.wasm")
	checksum, err := vm.Create(code)
	require.NoError(t, err)
	barrier := &repair184CompileBarrier{WasmerEngine: vm, entered: make(chan struct{}), release: make(chan struct{})}
	wrapped := keeper.NewVMWrapper(barrier)
	ctx, keepers := keeper.CreateTestInput(t, false, "iterator,staking,stargate", keeper.WithWasmEngine(wrapped))
	admin := keeper.RandomAccountAddress(t)
	type upload struct {
		id  uint64
		err error
	}
	uploaded := make(chan upload, 1)
	go func() { id, err := keepers.ContractKeeper.Create(ctx, admin, code, nil); uploaded <- upload{id, err} }()
	<-barrier.entered
	pinned := make(chan error, 1)
	loaded := make(chan error, 1)
	go func() { pinned <- wrapped.Pin(checksum) }()
	go func() { _, err := wrapped.GetCode(checksum); loaded <- err }()
	// The delay crosses the former consensus timeout while compilation retains
	// ownership. The actual native compiler still supplies every result.
	timer := time.NewTimer(16 * time.Second)
	select {
	case result := <-uploaded:
		close(barrier.release)
		t.Fatalf("upload returned before native compilation finished: %+v", result)
	case <-timer.C:
	}
	select {
	case err := <-pinned:
		close(barrier.release)
		t.Fatalf("Pin escaped compile ownership: %v", err)
	default:
	}
	select {
	case err := <-loaded:
		close(barrier.release)
		t.Fatalf("GetCode escaped compile ownership: %v", err)
	default:
	}
	close(barrier.release)
	result := <-uploaded
	require.NoError(t, result.err)
	require.NoError(t, <-pinned)
	require.NoError(t, <-loaded)
	info := keepers.WasmKeeper.GetCodeInfo(ctx, result.id)
	require.Equal(t, []byte(checksum), info.CodeHash)
	stored, err := keepers.WasmKeeper.GetByteCode(ctx, result.id)
	require.NoError(t, err)
	require.Equal(t, code, stored)
	require.NoError(t, wrapped.Unpin(checksum))
	id, err := keepers.ContractKeeper.Create(ctx, admin, code, nil)
	require.NoError(t, err)
	require.Equal(t, info.CodeHash, keepers.WasmKeeper.GetCodeInfo(ctx, id).CodeHash)
	_, err = wrapped.Create([]byte("invalid wasm"))
	require.Error(t, err)
	_, err = wrapped.Create(make([]byte, types.MaxWasmSize+1))
	require.ErrorContains(t, err, "maximum size")
}

type repair184GasCharge struct {
	Amount     uint64
	Descriptor string
}
type repair184Meter struct {
	sdk.GasMeter
	Charges []repair184GasCharge
}

func (m *repair184Meter) ConsumeGas(amount uint64, descriptor string) {
	m.Charges = append(m.Charges, repair184GasCharge{amount, descriptor})
	m.GasMeter.ConsumeGas(amount, descriptor)
}

type repair184BoundaryGas struct {
	Method                                     string
	SDKBefore, SDKAfter, WasmBefore, WasmAfter uint64
	Charges                                    []repair184GasCharge
	Limit, Used                                uint64
	RuntimeChargeIndex                         int
}

type repair184ObservedEngine struct {
	types.WasmerEngine
	Meter *repair184Meter
	Calls []repair184BoundaryGas
}

func (e *repair184ObservedEngine) begin(method string, meter wasmvm.GasMeter, limit uint64, used *uint64) func() {
	call := repair184BoundaryGas{Limit: limit, Method: method, SDKBefore: e.Meter.GasConsumed(), WasmBefore: meter.GasConsumed()}
	start := len(e.Meter.Charges)
	return func() {
		call.SDKAfter, call.WasmAfter = e.Meter.GasConsumed(), meter.GasConsumed()
		call.Used = *used
		call.RuntimeChargeIndex = len(e.Meter.Charges)
		call.Charges = append([]repair184GasCharge(nil), e.Meter.Charges[start:]...)
		e.Calls = append(e.Calls, call)
	}
}

func (e *repair184ObservedEngine) IBCChannelOpen(checksum wasmvm.Checksum, env vmtypes.Env, msg vmtypes.IBCChannelOpenMsg, store wasmvm.KVStore, api wasmvm.GoAPI, q wasmvm.Querier, meter wasmvm.GasMeter, limit uint64, cost vmtypes.UFraction) (response *vmtypes.IBC3ChannelOpenResponse, used uint64, err error) {
	defer e.begin("IBCChannelOpen", meter, limit, &used)()
	return e.WasmerEngine.IBCChannelOpen(checksum, env, msg, store, api, q, meter, limit, cost)
}

func (e *repair184ObservedEngine) IBCChannelConnect(checksum wasmvm.Checksum, env vmtypes.Env, msg vmtypes.IBCChannelConnectMsg, store wasmvm.KVStore, api wasmvm.GoAPI, q wasmvm.Querier, meter wasmvm.GasMeter, limit uint64, cost vmtypes.UFraction) (response *vmtypes.IBCBasicResponse, used uint64, err error) {
	defer e.begin("IBCChannelConnect", meter, limit, &used)()
	return e.WasmerEngine.IBCChannelConnect(checksum, env, msg, store, api, q, meter, limit, cost)
}

func (e *repair184ObservedEngine) IBCChannelClose(checksum wasmvm.Checksum, env vmtypes.Env, msg vmtypes.IBCChannelCloseMsg, store wasmvm.KVStore, api wasmvm.GoAPI, q wasmvm.Querier, meter wasmvm.GasMeter, limit uint64, cost vmtypes.UFraction) (response *vmtypes.IBCBasicResponse, used uint64, err error) {
	defer e.begin("IBCChannelClose", meter, limit, &used)()
	return e.WasmerEngine.IBCChannelClose(checksum, env, msg, store, api, q, meter, limit, cost)
}

func (e *repair184ObservedEngine) IBCPacketReceive(checksum wasmvm.Checksum, env vmtypes.Env, msg vmtypes.IBCPacketReceiveMsg, store wasmvm.KVStore, api wasmvm.GoAPI, q wasmvm.Querier, meter wasmvm.GasMeter, limit uint64, cost vmtypes.UFraction) (response *vmtypes.IBCReceiveResult, used uint64, err error) {
	defer e.begin("IBCPacketReceive", meter, limit, &used)()
	return e.WasmerEngine.IBCPacketReceive(checksum, env, msg, store, api, q, meter, limit, cost)
}

func (e *repair184ObservedEngine) IBCPacketAck(checksum wasmvm.Checksum, env vmtypes.Env, msg vmtypes.IBCPacketAckMsg, store wasmvm.KVStore, api wasmvm.GoAPI, q wasmvm.Querier, meter wasmvm.GasMeter, limit uint64, cost vmtypes.UFraction) (response *vmtypes.IBCBasicResponse, used uint64, err error) {
	defer e.begin("IBCPacketAck", meter, limit, &used)()
	return e.WasmerEngine.IBCPacketAck(checksum, env, msg, store, api, q, meter, limit, cost)
}

func (e *repair184ObservedEngine) IBCPacketTimeout(checksum wasmvm.Checksum, env vmtypes.Env, msg vmtypes.IBCPacketTimeoutMsg, store wasmvm.KVStore, api wasmvm.GoAPI, q wasmvm.Querier, meter wasmvm.GasMeter, limit uint64, cost vmtypes.UFraction) (response *vmtypes.IBCBasicResponse, used uint64, err error) {
	defer e.begin("IBCPacketTimeout", meter, limit, &used)()
	return e.WasmerEngine.IBCPacketTimeout(checksum, env, msg, store, api, q, meter, limit, cost)
}

func TestRepair_184_IBCGasUnits(t *testing.T) {
	for _, multiplier := range []uint64{keeper.DefaultGasMultiplier, 21_000_000} {
		t.Run(fmt.Sprint(multiplier), func(t *testing.T) {
			vm, err := wasmvm.NewVM(t.TempDir(), "iterator,staking,stargate", 32, false, 32)
			require.NoError(t, err)
			defer vm.Cleanup()
			engine := &repair184ObservedEngine{WasmerEngine: keeper.NewVMWrapper(vm)}
			config := keeper.DefaultGasRegisterConfig()
			config.GasMultiplier = multiplier
			ctx, keepers := keeper.CreateTestInput(t, false, "iterator,staking,stargate", keeper.WithWasmEngine(engine), keeper.WithGasRegister(keeper.NewWasmGasRegister(config)))
			admin := keeper.RandomAccountAddress(t)
			reflectID := keeper.StoreReflectContract(t, ctx, keepers)
			// Use the committed IBC3 fixture whose native handshake returns the
			// negotiated version, rather than the older fixture's unit response.
			ibcID, err := keepers.ContractKeeper.Create(ctx, admin, repair184Code(t, "../../../../wasm-runtime/testdata/ibc_reflect.wasm"), nil)
			require.NoError(t, err)
			contract, _, err := keepers.ContractKeeper.Instantiate(ctx, ibcID, admin, admin, repair184JSON(t, keeper.IBCReflectInitMsg{ReflectCodeID: reflectID}), "IBC3 gas boundary", nil)
			require.NoError(t, err)
			instance := keeper.IBCReflectExampleInstance{Contract: contract, Admin: admin, CodeID: ibcID, ReflectCodeID: reflectID}
			meter := &repair184Meter{GasMeter: sdk.NewGasMeter(5_000_000, 1, 1)}
			ctx = ctx.WithGasMeter(meter)
			engine.Meter = meter
			channel := vmtypes.IBCChannel{
				Endpoint:             vmtypes.IBCEndpoint{PortID: keepers.WasmKeeper.GetContractInfo(ctx, instance.Contract).IBCPortID, ChannelID: "channel-184"},
				CounterpartyEndpoint: vmtypes.IBCEndpoint{PortID: "wasm.remote", ChannelID: "channel-185"},
				Order:                vmtypes.Ordered, Version: "ibc-reflect-v1", ConnectionID: "connection-0",
			}
			version, err := keepers.WasmKeeper.OnOpenChannel(ctx, instance.Contract, vmtypes.IBCChannelOpenMsg{OpenInit: &vmtypes.IBCOpenInit{Channel: channel}})
			require.NoError(t, err)
			require.Equal(t, "ibc-reflect-v1", version)
			require.NoError(t, keepers.WasmKeeper.OnConnectChannel(ctx, instance.Contract, vmtypes.IBCChannelConnectMsg{OpenAck: &vmtypes.IBCOpenAck{Channel: channel, CounterpartyVersion: channel.Version}}))
			accounts, err := keepers.WasmKeeper.QuerySmart(ctx, instance.Contract, []byte(`{"list_accounts":{}}`))
			require.NoError(t, err)
			var list struct {
				Accounts []struct {
					Account   string `json:"account"`
					ChannelID string `json:"channel_id"`
				} `json:"accounts"`
			}
			require.NoError(t, json.Unmarshal(accounts, &list))
			require.Len(t, list.Accounts, 1)
			require.Equal(t, channel.Endpoint.ChannelID, list.Accounts[0].ChannelID)
			packet := vmtypes.IBCPacket{Data: []byte(`{"who_am_i":{}}`), Src: channel.CounterpartyEndpoint, Dest: channel.Endpoint, Sequence: 1, Timeout: vmtypes.IBCTimeout{Block: &vmtypes.IBCTimeoutBlock{Revision: 1, Height: 99}}}
			receiveStart := meter.GasConsumed()
			ack, err := keepers.WasmKeeper.OnRecvPacket(ctx, instance.Contract, vmtypes.IBCPacketReceiveMsg{Packet: packet, Relayer: instance.Admin.String()})
			require.NoError(t, err)
			require.True(t, json.Valid(ack))
			require.Contains(t, string(ack), list.Accounts[0].Account)
			receiveCall := engine.Calls[len(engine.Calls)-1]
			metadataCost := receiveCall.SDKBefore - receiveStart
			storageCost := receiveCall.SDKAfter - receiveCall.SDKBefore
			require.Positive(t, storageCost)
			limitedNative := &repair184Meter{GasMeter: sdk.NewGasMeter(metadataCost+storageCost/2, 1, 1)}
			engine.Meter = limitedNative
			callsBefore := len(engine.Calls)
			var refused error
			var outOfGas interface{}
			func() {
				defer func() { outOfGas = recover() }()
				_, refused = keepers.WasmKeeper.OnRecvPacket(ctx.WithGasMeter(limitedNative), instance.Contract, vmtypes.IBCPacketReceiveMsg{Packet: packet, Relayer: instance.Admin.String()})
			}()
			if outOfGas != nil {
				require.IsType(t, sdk.ErrorOutOfGas{}, outOfGas)
			} else {
				require.Error(t, refused)
			}
			require.Len(t, engine.Calls, callsBefore+1, "budget refusal must cross the native IBC callback boundary")
			require.Positive(t, limitedNative.GasConsumed())
			engine.Calls = engine.Calls[:callsBefore]
			engine.Meter = meter
			require.NoError(t, keepers.WasmKeeper.OnAckPacket(ctx, instance.Contract, vmtypes.IBCPacketAckMsg{OriginalPacket: packet, Acknowledgement: vmtypes.IBCAcknowledgement{Data: ack}, Relayer: instance.Admin.String()}))
			require.NoError(t, keepers.WasmKeeper.OnTimeoutPacket(ctx, instance.Contract, vmtypes.IBCPacketTimeoutMsg{Packet: packet, Relayer: instance.Admin.String()}))
			require.NoError(t, keepers.WasmKeeper.OnCloseChannel(ctx, instance.Contract, vmtypes.IBCChannelCloseMsg{CloseConfirm: &vmtypes.IBCCloseConfirm{Channel: channel}}))
			require.Len(t, engine.Calls, 6)
			methods := make(map[string]bool)
			var nativeReads, nativeWrites, iterations int
			for _, call := range engine.Calls {
				methods[call.Method] = true
				require.Positive(t, call.SDKBefore)
				require.Equal(t, (meter.Limit()-call.SDKBefore)*multiplier, call.Limit, call.Method)
				require.Positive(t, call.Used, call.Method)
				require.LessOrEqual(t, call.Used+(call.WasmAfter-call.WasmBefore), call.Limit, call.Method)
				require.Less(t, call.RuntimeChargeIndex, len(meter.Charges))
				require.Equal(t, "wasm contract", meter.Charges[call.RuntimeChargeIndex].Descriptor, call.Method)
				require.Equal(t, call.Used/multiplier, meter.Charges[call.RuntimeChargeIndex].Amount, call.Method)
				require.Equal(t, call.SDKBefore*multiplier, call.WasmBefore, call.Method)
				require.Equal(t, call.SDKAfter*multiplier, call.WasmAfter, call.Method)
				require.Equal(t, (call.SDKAfter-call.SDKBefore)*multiplier, call.WasmAfter-call.WasmBefore, call.Method)
				for _, charge := range call.Charges {
					if charge.Descriptor == "ReadFlat" {
						nativeReads++
					}
					if charge.Descriptor == "WriteFlat" {
						nativeWrites++
					}
				}
			}
			require.Len(t, methods, 6)
			require.Positive(t, nativeReads)
			require.Positive(t, nativeWrites)
			for _, charge := range meter.Charges {
				if charge.Descriptor == "IterNextFlat" {
					iterations++
				}
			}
			require.Positive(t, iterations, "the native account-list query must iterate the stored channel records")
			// The real callback must refuse a budget already exhausted by keeper
			// metadata reads, rather than executing with an unmetered SDK limit.
			limited := &repair184Meter{GasMeter: sdk.NewGasMeter(1, 1, 1)}
			engine.Meter = limited
			require.Panics(t, func() {
				_, _ = keepers.WasmKeeper.OnRecvPacket(ctx.WithGasMeter(limited), instance.Contract, vmtypes.IBCPacketReceiveMsg{Packet: packet, Relayer: instance.Admin.String()})
			})
			require.True(t, limited.IsOutOfGas())
		})
	}
}

type repair184QueryPlugins struct {
	keeper.WasmVMQueryHandler
	SDKCosts []uint64
}

func (p *repair184QueryPlugins) HandleQuery(ctx sdk.Context, caller sdk.AccAddress, req vmtypes.QueryRequest) ([]byte, error) {
	before := ctx.GasMeter().GasConsumed()
	defer func() { p.SDKCosts = append(p.SDKCosts, ctx.GasMeter().GasConsumed()-before) }()
	return p.WasmVMQueryHandler.HandleQuery(ctx, caller, req)
}

type repair184Querier struct {
	keeper.QueryHandler
	SDKDeltas, WasmDeltas, Limits []uint64
}

func (q *repair184Querier) Query(req vmtypes.QueryRequest, limit uint64) ([]byte, error) {
	sdkBefore, wasmBefore := q.Ctx.GasMeter().GasConsumed(), q.GasConsumed()
	q.Limits = append(q.Limits, limit)
	defer func() {
		q.SDKDeltas = append(q.SDKDeltas, q.Ctx.GasMeter().GasConsumed()-sdkBefore)
		q.WasmDeltas = append(q.WasmDeltas, q.GasConsumed()-wasmBefore)
	}()
	return q.QueryHandler.Query(req, limit)
}

func TestRepair_184_QueryGasUnits(t *testing.T) {
	for _, multiplier := range []uint64{keeper.DefaultGasMultiplier, 21_000_000} {
		t.Run(fmt.Sprint(multiplier), func(t *testing.T) {
			config := keeper.DefaultGasRegisterConfig()
			config.GasMultiplier = multiplier
			register := keeper.NewWasmGasRegister(config)
			ctx, keepers := keeper.CreateTestInput(t, false, "iterator,staking,stargate", keeper.WithGasRegister(register))
			owner := keeper.RandomAccountAddress(t)
			codeID := keeper.StoreReflectContract(t, ctx, keepers)
			contract, _, err := keepers.ContractKeeper.Instantiate(ctx, codeID, owner, owner, []byte(`{}`), "query gas transport", nil)
			require.NoError(t, err)
			keepers.Faucet.Fund(ctx, owner, sdk.NewInt64Coin("denom", 184))
			plugins := keeper.DefaultQueryPlugins(keepers.BankKeeper, keepers.StakingKeeper, keepers.DistKeeper, keepers.IBCKeeper.ChannelKeeper, nil, keepers.WasmKeeper)
			requests := []struct {
				name     string
				request  vmtypes.QueryRequest
				expected string
				failure  bool
			}{
				{"bank", vmtypes.QueryRequest{Bank: &vmtypes.BankQuery{Balance: &vmtypes.BalanceQuery{Address: owner.String(), Denom: "denom"}}}, `{"amount":{"denom":"denom","amount":"184"}}`, false},
				{"contract", vmtypes.QueryRequest{Wasm: &vmtypes.WasmQuery{Smart: &vmtypes.SmartQuery{ContractAddr: contract.String(), Msg: []byte(`{"owner":{}}`)}}}, fmt.Sprintf(`{"owner":%q}`, owner.String()), false},
				{"error", vmtypes.QueryRequest{Wasm: &vmtypes.WasmQuery{Smart: &vmtypes.SmartQuery{ContractAddr: keeper.BuildContractAddress(999, 999).String(), Msg: []byte(`{"owner":{}}`)}}}, "", true},
			}
			for _, tc := range requests {
				t.Run(tc.name, func(t *testing.T) {
					meter := &repair184Meter{GasMeter: sdk.NewGasMeter(5_000_000, 1, 1)}
					callCtx := ctx.WithGasMeter(meter)
					observed := &repair184QueryPlugins{WasmVMQueryHandler: plugins}
					q := &repair184Querier{QueryHandler: keeper.NewQueryHandler(callCtx, observed, contract, register)}
					msg := repair184JSON(t, map[string]interface{}{"chain": map[string]interface{}{"request": tc.request}})
					response, _, err := keeper.Repair184NativeQuery(keepers.WasmKeeper, callCtx, contract, msg, q, register.ToWasmVMGas(1_000_000))
					if tc.failure {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						var wrapped struct {
							Data []byte `json:"data"`
						}
						require.NoError(t, json.Unmarshal(response, &wrapped))
						require.JSONEq(t, tc.expected, string(wrapped.Data))
					}
					require.Len(t, observed.SDKCosts, 1)
					require.Len(t, q.SDKDeltas, 1)
					require.Positive(t, q.SDKDeltas[0])
					require.Equal(t, observed.SDKCosts[0], q.SDKDeltas[0], "charge the parent exactly once")
					require.Equal(t, q.SDKDeltas[0]*multiplier, q.WasmDeltas[0])
					var charged uint64
					var count int
					for _, charge := range meter.Charges {
						if charge.Descriptor == "contract sub-query" {
							charged += charge.Amount
							count++
						}
					}
					require.Equal(t, 1, count)
					require.Equal(t, q.SDKDeltas[0], charged)
					if !tc.failure {
						actual, err := keepers.WasmKeeper.QuerySmart(ctx.WithGasMeter(sdk.NewGasMeter(5_000_000, 1, 1)), contract, msg)
						require.NoError(t, err)
						require.JSONEq(t, string(response), string(actual))
					}
				})
			}
			// QueryHandler must account for the real bank store read even when
			// its converted subquery limit is exceeded and the SDK panics.
			meter := &repair184Meter{GasMeter: sdk.NewGasMeter(5_000_000, 1, 1)}
			observed := &repair184QueryPlugins{WasmVMQueryHandler: plugins}
			q := keeper.NewQueryHandler(ctx.WithGasMeter(meter), observed, contract, register)
			require.Panics(t, func() { _, _ = q.Query(requests[0].request, register.ToWasmVMGas(1)) })
			require.Len(t, observed.SDKCosts, 1)
			require.Equal(t, observed.SDKCosts[0], meter.GasConsumed())
			require.Equal(t, meter.GasConsumed()*multiplier, q.GasConsumed())
			// A budget too small for the real nested contract is refused by the
			// native boundary; the callback's deferred parent accounting survives.
			refusalCtx := ctx.WithGasMeter(sdk.NewGasMeter(5_000_000, 1, 1))
			refusalQ := &repair184Querier{QueryHandler: keeper.NewQueryHandler(refusalCtx, plugins, contract, register)}
			msg := repair184JSON(t, map[string]interface{}{"chain": map[string]interface{}{"request": requests[1].request}})
			_, _, err = keeper.Repair184NativeQuery(keepers.WasmKeeper, refusalCtx, contract, msg, refusalQ, register.ToWasmVMGas(50_000))
			require.Error(t, err)
			require.NotEmpty(t, refusalQ.SDKDeltas)
			require.Positive(t, refusalQ.SDKDeltas[0])
			require.Equal(t, refusalQ.SDKDeltas[0]*multiplier, refusalQ.WasmDeltas[0])
		})
	}
}

func repair184Module(keepers keeper.TestKeepers) wasm.AppModule {
	return wasm.NewAppModule(keepers.EncodingConfig.Marshaler, keepers.WasmKeeper, &keepers.StakingKeeper, keepers.AccountKeeper, keepers.BankKeeper)
}

func repair184ExportFile(t *testing.T, ctx sdk.Context, keepers keeper.TestKeepers) (string, []module.ModuleState) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "genesis-stream.json")
	file, err := os.Create(path)
	require.NoError(t, err)
	manager := module.NewManager(repair184Module(keepers))
	var records []module.ModuleState
	require.NoError(t, manager.ProcessGenesisPerModule(ctx, keepers.EncodingConfig.Marshaler, func(name string, data json.RawMessage) error {
		record := module.ModuleState{AppState: module.AppState{Module: name, Data: bytes.Clone(data)}}
		records = append(records, record)
		_, err := fmt.Fprintln(file, string(repair184JSON(t, record)))
		return err
	}))
	require.NoError(t, file.Close())
	return path, records
}

func repair184WriteRecords(t *testing.T, records []module.ModuleState) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "genesis-stream.json")
	file, err := os.Create(path)
	require.NoError(t, err)
	for _, record := range records {
		_, err := fmt.Fprintln(file, string(repair184JSON(t, record)))
		require.NoError(t, err)
	}
	require.NoError(t, file.Close())
	return path
}

func repair184ImportFile(ctx sdk.Context, keepers keeper.TestKeepers, path string) {
	manager := module.NewManager(repair184Module(keepers))
	manager.InitGenesis(ctx, keepers.EncodingConfig.Marshaler, nil, genesistypes.GenesisImportConfig{StreamGenesisImport: true, GenesisStreamFile: path})
}

func TestRepair_184_StreamContractReimport(t *testing.T) {
	srcCtx, source := keeper.CreateTestInput(t, false, "iterator,staking,stargate")
	instance := keeper.InstantiateHackatomExampleContract(t, srcCtx, source)
	models := make([]types.Model, 50003)
	for i := range models {
		models[i] = types.Model{Key: []byte(fmt.Sprintf("repair184/%05d", i)), Value: sdk.Uint64ToBigEndian(uint64(i))}
	}
	require.NoError(t, keeper.Repair184ImportState(source.WasmKeeper, srcCtx, instance.Contract, models))
	source.MultiStore.Commit(true)
	file, records := repair184ExportFile(t, srcCtx, source)
	var contractRecords []int
	for i, record := range records {
		var state types.GenesisState
		source.EncodingConfig.Marshaler.MustUnmarshalJSON(record.AppState.Data, &state)
		if len(state.Contracts) != 0 {
			contractRecords = append(contractRecords, i)
			require.Len(t, state.Contracts, 1)
			require.NotEmpty(t, state.Contracts[0].ContractState)
		}
	}
	require.Len(t, contractRecords, 2)
	var firstChunk types.GenesisState
	source.EncodingConfig.Marshaler.MustUnmarshalJSON(records[contractRecords[0]].AppState.Data, &firstChunk)
	require.Len(t, firstChunk.Contracts[0].ContractState, keeper.GENSIS_STATE_STREAM_BUF_THRESHOLD)
	for _, recoverMidStream := range []bool{false, true} {
		t.Run(fmt.Sprintf("restart_during_import_%v", recoverMidStream), func(t *testing.T) {
			dir := t.TempDir()
			ctx, destination, closeStore := repair184Open(t, dir)
			if recoverMidStream {
				prefix := repair184WriteRecords(t, records[:contractRecords[0]+1])
				repair184ImportFile(ctx, destination, prefix)
				require.ErrorContains(t, destination.WasmKeeper.FinalizeGenesisImport(ctx), "final record")
				_, err := destination.WasmKeeper.QuerySmart(ctx, instance.Contract, []byte(`{"verifier":{}}`))
				require.ErrorContains(t, err, "incomplete")
				destination.MultiStore.Commit(true)
				closeStore()
				ctx, destination, closeStore = repair184Open(t, dir)
				require.Error(t, destination.WasmKeeper.FinalizeGenesisImport(ctx))
				suffix := repair184WriteRecords(t, records[contractRecords[0]+1:])
				repair184ImportFile(ctx, destination, suffix)
			} else {
				repair184ImportFile(ctx, destination, file)
			}
			require.NoError(t, destination.WasmKeeper.FinalizeGenesisImport(ctx))
			destination.MultiStore.Commit(true)
			closeStore()
			ctx, destination, closeStore = repair184Open(t, dir)
			defer closeStore()
			require.NoError(t, destination.WasmKeeper.FinalizeGenesisImport(ctx))
			var info types.QueryContractInfoResponse
			repair184Query(t, ctx, destination, "ContractInfo", &types.QueryContractInfoRequest{Address: instance.Contract.String()}, &info)
			expected := *source.WasmKeeper.GetContractInfo(srcCtx, instance.Contract)
			expected.Created = nil
			require.Equal(t, expected, info.ContractInfo)
			var all []types.Model
			var next []byte
			for {
				var response types.QueryAllContractStateResponse
				repair184Query(t, ctx, destination, "AllContractState", &types.QueryAllContractStateRequest{Address: instance.Contract.String(), Pagination: &query.PageRequest{Key: next, Limit: 1000}}, &response)
				all = append(all, response.Models...)
				next = response.Pagination.NextKey
				if len(next) == 0 {
					break
				}
			}
			require.Len(t, all, len(models)+1)
			for i, model := range models {
				require.Equal(t, model, all[i+1])
			}
			var smart types.QuerySmartContractStateResponse
			repair184Query(t, ctx, destination, "SmartContractState", &types.QuerySmartContractStateRequest{Address: instance.Contract.String(), QueryData: []byte(`{"verifier":{}}`)}, &smart)
			require.JSONEq(t, fmt.Sprintf(`{"verifier":%q}`, instance.VerifierAddr.String()), string(smart.Data))
			var byCode types.QueryContractsByCodeResponse
			repair184Query(t, ctx, destination, "ContractsByCode", &types.QueryContractsByCodeRequest{CodeId: instance.CodeID}, &byCode)
			require.Equal(t, []string{instance.Contract.String()}, byCode.Contracts)
			history := destination.WasmKeeper.GetContractHistory(ctx, instance.Contract)
			require.Len(t, history, 1, "metadata, history and index are created once across all chunks")
			require.Equal(t, types.ContractCodeHistoryOperationTypeGenesis, history[0].Operation)
		})
	}
}

func TestRepair_184_StreamRejectsConflicts(t *testing.T) {
	ctx, source := keeper.CreateTestInput(t, false, "iterator,staking,stargate")
	instance := keeper.InstantiateHackatomExampleContract(t, ctx, source)
	models := make([]types.Model, 50001)
	for i := range models {
		models[i] = types.Model{Key: []byte(fmt.Sprintf("key/%05d", i)), Value: []byte{1}}
	}
	require.NoError(t, keeper.Repair184ImportState(source.WasmKeeper, ctx, instance.Contract, models))
	_, records := repair184ExportFile(t, ctx, source)
	var chunks []types.GenesisState
	var first int
	for i, record := range records {
		var state types.GenesisState
		source.EncodingConfig.Marshaler.MustUnmarshalJSON(record.AppState.Data, &state)
		if len(state.Contracts) > 0 {
			if len(chunks) == 0 {
				first = i
			}
			chunks = append(chunks, state)
		}
	}
	require.Len(t, chunks, 2)
	dstCtx, destination := keeper.CreateTestInput(t, false, "iterator,staking,stargate")
	repair184ImportFile(dstCtx, destination, repair184WriteRecords(t, records[:first+1]))
	am := repair184Module(destination)
	cdc := destination.EncodingConfig.Marshaler
	firstBytes := cdc.MustMarshalJSON(&chunks[0])
	require.Panics(t, func() { am.InitGenesis(dstCtx, cdc, firstBytes) }, "repeated first chunk remains a duplicate")
	conflict := chunks[1]
	conflict.Contracts = append([]types.Contract(nil), conflict.Contracts...)
	conflict.Contracts[0].ContractInfo.Label = "conflicting metadata"
	require.Panics(t, func() { am.InitGenesis(dstCtx, cdc, cdc.MustMarshalJSON(&conflict)) })
	outOfOrder := chunks[1]
	outOfOrder.Contracts = append([]types.Contract(nil), outOfOrder.Contracts...)
	outOfOrder.Contracts[0].ContractState = []types.Model{models[0]}
	require.Panics(t, func() { am.InitGenesis(dstCtx, cdc, cdc.MustMarshalJSON(&outOfOrder)) })
	missing := records[len(records)-1]
	require.Panics(t, func() { am.InitGenesis(dstCtx, cdc, missing.AppState.Data) }, "finalization refuses missing continuation")
	// Every rejected record was atomic: the unchanged, producer-generated
	// continuation and final record still complete the import.
	repair184ImportFile(dstCtx, destination, repair184WriteRecords(t, records[first+1:]))
	require.NoError(t, destination.WasmKeeper.FinalizeGenesisImport(dstCtx))
	require.Len(t, destination.WasmKeeper.GetContractHistory(dstCtx, instance.Contract), 1)
	full := keeper.ExportGenesis(ctx, source.WasmKeeper)
	full.Contracts = append(full.Contracts, full.Contracts[0])
	ordinaryCtx, ordinary := keeper.CreateTestInput(t, false, "iterator,staking,stargate")
	_, err := keeper.InitGenesis(ordinaryCtx, ordinary.WasmKeeper, *full, &ordinary.StakingKeeper, nil)
	require.ErrorIs(t, err, types.ErrDuplicate, "ordinary genesis must retain duplicate-contract rejection")
}

func repair184CW721(t *testing.T) (*app.App, sdk.Context, sdk.AccAddress, sdk.AccAddress, sdk.AccAddress) {
	t.Helper()
	a := app.Setup(t, false, false, false)
	ctx := a.GetContextForDeliverTx(nil).WithChainID("pax-test").WithBlockHeight(1).WithBlockTime(time.Unix(1_800_000_000, 0)).WithGasMeter(sdk.NewGasMeter(100_000_000, 1, 1))
	owner, recipient := keeper.RandomAccountAddress(t), keeper.RandomAccountAddress(t)
	contracts := keeper.NewDefaultPermissionKeeper(&a.WasmKeeper)
	code, err := contracts.Create(ctx, owner, repair184Code(t, "../../../../contracts/wasm/cw721_base.wasm"), nil)
	require.NoError(t, err)
	address, _, err := contracts.Instantiate(ctx, code, owner, owner, repair184JSON(t, map[string]string{"name": "repair 184", "symbol": "R184", "minter": owner.String()}), "real CW721 receipt", nil)
	require.NoError(t, err)
	return a, ctx, address, owner, recipient
}

func TestRepair_184_CW721JSONStrings(t *testing.T) {
	a, ctx, address, owner, recipient := repair184CW721(t)
	pointer := common.HexToAddress("0x184")
	require.NoError(t, a.EvmKeeper.SetERC721CW721Pointer(ctx, address.String(), pointer))
	contracts := keeper.NewDefaultPermissionKeeper(&a.WasmKeeper)
	handler := wasm.NewHandler(contracts)
	for _, encoding := range []struct{ name, token string }{{"plain", `"1"`}, {"escaped", `"\u0031"`}} {
		t.Run(encoding.name, func(t *testing.T) {
			_, err := contracts.Execute(ctx, address, owner, repair184JSON(t, map[string]interface{}{"mint": map[string]string{"token_id": "1", "owner": owner.String()}}), nil)
			require.NoError(t, err)
			for _, action := range []string{"transfer_nft", "burn"} {
				sender, expectedOwner, expectedTo := owner, owner, a.EvmKeeper.GetEVMAddressOrDefault(ctx, recipient)
				message := []byte(fmt.Sprintf(`{"transfer_nft":{"recipient":%q,"token_id":%s}}`, recipient.String(), encoding.token))
				if action == "burn" {
					sender, expectedOwner, expectedTo = recipient, recipient, common.Address{}
					message = []byte(fmt.Sprintf(`{"burn":{"token_id":%s}}`, encoding.token))
				}
				callCtx := ctx.WithEventManager(sdk.NewEventManager())
				result, err := handler(callCtx, &types.MsgExecuteContract{Sender: sender.String(), Contract: address.String(), Msg: message})
				require.NoError(t, err)
				var ownerEvents []abci.Event
				for _, event := range result.Events {
					if event.Type == types.EventTypeCW721PreTransferOwner {
						ownerEvents = append(ownerEvents, event)
					}
				}
				require.Len(t, ownerEvents, 1)
				token, found := app.GetAttributeValue(ownerEvents[0], types.AttributeKeyTokenId)
				require.True(t, found)
				require.Equal(t, "1", token)
				before, found := app.GetAttributeValue(ownerEvents[0], types.AttributeKeyOwner)
				require.True(t, found)
				require.Equal(t, expectedOwner.String(), before)
				checksum := sha256.Sum256(append([]byte(encoding.name+action), message...))
				require.NoError(t, a.AddCosmosEventsToEVMReceiptIfApplicable(callCtx, nil, checksum, sdk.DeliverTxHookInput{Events: result.Events}))
				receipt, err := a.EvmKeeper.GetTransientReceipt(ctx, common.BytesToHash(checksum[:]), uint64(ctx.TxIndex()))
				require.NoError(t, err)
				require.Len(t, receipt.Logs, 1)
				log := receipt.Logs[0]
				require.Equal(t, pointer.Hex(), log.Address)
				from := a.EvmKeeper.GetEVMAddressOrDefault(ctx, expectedOwner)
				require.Equal(t, []string{app.ERC721TransferTopic.Hex(), common.BytesToHash(from.Bytes()).Hex(), common.BytesToHash(expectedTo.Bytes()).Hex(), common.HexToHash("0x1").Hex()}, log.Topics)
				if action == "transfer_nft" {
					response, err := a.WasmKeeper.QuerySmart(ctx, address, []byte(`{"owner_of":{"token_id":"1"}}`))
					require.NoError(t, err)
					var owned struct {
						Owner string `json:"owner"`
					}
					require.NoError(t, json.Unmarshal(response, &owned))
					require.Equal(t, recipient.String(), owned.Owner)
				} else {
					_, err := a.WasmKeeper.QuerySmart(ctx, address, []byte(`{"owner_of":{"token_id":"1"}}`))
					require.Error(t, err)
				}
			}
		})
	}
}

func TestRepair_184_CW721OwnerFailures(t *testing.T) {
	a, ctx, address, owner, recipient := repair184CW721(t)
	contracts := keeper.NewDefaultPermissionKeeper(&a.WasmKeeper)
	for _, message := range [][]byte{
		[]byte(fmt.Sprintf(`{"transfer_nft":{"recipient":%q,"token_id":"\u0039"}}`, recipient.String())),
		[]byte(`{"burn":{"token_id":1}}`),
		[]byte(`{"burn":{"token_id":null}}`),
		[]byte(`{"send_nft":{"contract":"bad-address","token_id":"9","msg":""}}`),
	} {
		callCtx := ctx.WithEventManager(sdk.NewEventManager())
		_, err := contracts.Execute(callCtx, address, owner, message, nil)
		require.ErrorIs(t, err, keeper.ErrCW721OwnerLookup)
		for _, event := range callCtx.EventManager().Events() {
			require.NotEqual(t, types.EventTypeCW721PreTransferOwner, event.Type)
		}
	}
	_, err := contracts.Execute(ctx, address, owner, repair184JSON(t, map[string]interface{}{"mint": map[string]string{"token_id": "1", "owner": owner.String()}}), nil)
	require.NoError(t, err)
	// A real send reaches the real receiver lookup. With no receiver contract,
	// it fails and its cached transfer is discarded, preserving ownership.
	for _, encoded := range []string{`"1"`, `"\u0031"`} {
		cache, _ := ctx.CacheContext()
		cache = cache.WithEventManager(sdk.NewEventManager())
		_, err := contracts.Execute(cache, address, owner, []byte(fmt.Sprintf(`{"send_nft":{"contract":%q,"token_id":%s,"msg":""}}`, recipient.String(), encoded)), nil)
		require.Error(t, err)
		var events []sdk.Event
		for _, event := range cache.EventManager().Events() {
			if event.Type == types.EventTypeCW721PreTransferOwner {
				events = append(events, event)
			}
		}
		require.Len(t, events, 1)
		token, found := app.GetAttributeValue(abci.Event(events[0]), types.AttributeKeyTokenId)
		require.True(t, found)
		require.Equal(t, "1", token)
		response, err := a.WasmKeeper.QuerySmart(ctx, address, []byte(`{"owner_of":{"token_id":"1"}}`))
		require.NoError(t, err)
		require.Contains(t, string(response), owner.String())
	}
	pointer := common.HexToAddress("0x184")
	require.NoError(t, a.EvmKeeper.SetERC721CW721Pointer(ctx, address.String(), pointer))
	result, err := wasm.NewHandler(contracts)(ctx, &types.MsgExecuteContract{Sender: owner.String(), Contract: address.String(), Msg: []byte(fmt.Sprintf(`{"transfer_nft":{"recipient":%q,"token_id":"\u0031"}}`, recipient.String()))})
	require.NoError(t, err)
	var dropped []abci.Event
	for _, event := range result.Events {
		if event.Type != types.EventTypeCW721PreTransferOwner {
			dropped = append(dropped, event)
		}
	}
	checksum := sha256.Sum256([]byte("missing required owner transport event"))
	err = a.AddCosmosEventsToEVMReceiptIfApplicable(ctx, nil, checksum, sdk.DeliverTxHookInput{Events: dropped})
	require.ErrorIs(t, err, app.ErrSyntheticReceiptTranslation)
	require.ErrorIs(t, err, keeper.ErrCW721OwnerLookup)
	receipt, err := a.EvmKeeper.GetTransientReceipt(ctx, common.BytesToHash(checksum[:]), uint64(ctx.TxIndex()))
	require.Error(t, err)
	require.Nil(t, receipt)
}
