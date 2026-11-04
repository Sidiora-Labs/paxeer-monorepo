package keeper_test

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func repairM1303Snapshot(ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	out := make(map[string][]byte)
	it := ctx.KVStore(key).Iterator(nil, nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		out[string(it.Key())] = bytes.Clone(it.Value())
	}
	return out
}

func TestRepairM_1303_DeclarationAuthority(t *testing.T) {
	keeper.ProveRepairM1303Authority(t)
	// The family wrapper consumes the address-derived authority at Submit,
	// and the real host receipt identifies the same caller and activity.
	w := keeper.OpenRepairM1303World(t)
	authority, err := w.Keeper.AuthorityFor(w.Context, w.Caller)
	require.NoError(t, err)
	_, main, err := lxphost.MainAccountID(authority.AuthorityKey)
	require.NoError(t, err)
	require.Equal(t, w.Open.Owner, main)
	require.Equal(t, authority.Kernel.Actor, authority.Kernel.Principal)
	require.Equal(t, authority.AuthorityKey, authority.Kernel.VerifiedKey)
	receipt, err := codec.DecodeReceipt(w.Receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(types.EscrowModuleID), receipt.ModuleID)
	require.Equal(t, w.Receipt.GlobalSequence, receipt.GlobalSequence)
	record, found, err := w.Keeper.GetEscrow(w.Context, w.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, main, record.Owner)
}

func TestRepairM_1303_DeclarationSubmit(t *testing.T) {
	w := keeper.OpenRepairM1303World(t)
	var submit func(sdk.Context, common.Address, string, []byte, [32]byte) (lxphost.Receipt, error) = w.Keeper.Submit
	require.Equal(t, types.ActivityEscrowOpen, types.MethodActivity["escrowOpen"].Type)
	result, found, err := w.Keeper.GetEscrowResult(w.Context, w.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, w.Receipt.GlobalSequence, result.GlobalSequence)
	require.Equal(t, w.Open.Amount, result.LockedAfter)
	w.Capture(t)
	result, found, err = w.Keeper.GetEscrowResult(w.Context, w.ResultKey)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint16(3), result.Ordinal)
	require.Equal(t, codec.U128{Lo: 25}, result.Amount)
	require.Equal(t, w.Receipt.GlobalSequence, result.GlobalSequence)
	hostBefore, payBefore := repairM1303Snapshot(w.Context, w.HostKey), repairM1303Snapshot(w.Context, w.PayKey)
	_, err = submit(w.Context, w.Caller, "notAPayMethod", nil, [32]byte{1})
	require.ErrorIs(t, err, lxphost.Code(-106))
	_, err = submit(w.Context, common.Address{}, "escrowCapture", nil, [32]byte{1})
	require.ErrorIs(t, err, types.ErrUnboundCaller)
	require.Equal(t, hostBefore, repairM1303Snapshot(w.Context, w.HostKey))
	require.Equal(t, payBefore, repairM1303Snapshot(w.Context, w.PayKey))
	w.Reopen(t)
	after, found, err := w.Keeper.GetEscrowResult(w.Context, w.ResultKey)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, result, after)
}

func TestRepairM_1303_DeclarationGate(t *testing.T) {
	w := keeper.OpenRepairM1303World(t)
	var gate func(sdk.Context, string) error = w.Keeper.CheckWrite
	hostBefore := repairM1303Snapshot(w.Context, w.HostKey)
	w.Keeper.SetParams(w.Context, types.Params{BudgetEnabled: true})
	require.ErrorIs(t, gate(w.Context, "escrowCapture"), types.ErrFamilyDisabled)
	_, err := w.Keeper.EscrowCapture(w.Context, w.Beneficiary, w.ID, codec.U128{Lo: 1}, w.ResultKey)
	require.ErrorIs(t, err, types.ErrFamilyDisabled)
	w.Keeper.SetParams(w.Context, types.Params{Paused: true, EscrowEnabled: true})
	require.ErrorIs(t, gate(w.Context, "escrowCapture"), govtypes.ErrPausedScope)
	_, err = w.Keeper.EscrowCapture(w.Context, w.Beneficiary, w.ID, codec.U128{Lo: 1}, w.ResultKey)
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	require.NoError(t, gate(w.Context, "escrowRelease"))
	require.NoError(t, gate(w.Context, "getEscrow"))
	require.Equal(t, hostBefore, repairM1303Snapshot(w.Context, w.HostKey))
	w.Keeper.SetParams(w.Context, types.Params{EscrowEnabled: true})
	require.NoError(t, gate(w.Context, "escrowCapture"))
	enableKey, err := types.KernelModuleEnableKey(types.MethodActivity["escrowCapture"].Module)
	require.NoError(t, err)
	w.Context.KVStore(w.HostKey).Delete(enableKey)
	before := repairM1303Snapshot(w.Context, w.HostKey)
	require.ErrorIs(t, gate(w.Context, "escrowCapture"), types.ErrKernelGateMismatch)
	_, err = w.Keeper.EscrowCapture(w.Context, w.Beneficiary, w.ID, codec.U128{Lo: 1}, w.ResultKey)
	require.ErrorIs(t, err, types.ErrKernelGateMismatch)
	require.Equal(t, before, repairM1303Snapshot(w.Context, w.HostKey))
}

func TestRepairM_1303_DeclarationQuery(t *testing.T) {
	w := keeper.OpenRepairM1303World(t)
	var server types.QueryServer = keeper.Querier{Keeper: *w.Keeper}
	response, err := server.Params(context.Background(), nil)
	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := sdkcodec.NewProtoCodec(registry)
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(registry)
	types.RegisterQueryServer(router, server)
	route := router.Route("/paxprotocol.paxchain.layerxpay.Query/Params")
	require.NotNil(t, route)
	for _, params := range []types.Params{{Paused: true}, {EscrowEnabled: true, BudgetEnabled: true}} {
		w.Keeper.SetParams(w.Context, params)
		hostBefore, payBefore := repairM1303Snapshot(w.Context, w.HostKey), repairM1303Snapshot(w.Context, w.PayKey)
		reply, err := route(w.Context, abci.RequestQuery{Data: cdc.MustMarshal(&types.QueryParamsRequest{})})
		require.NoError(t, err)
		var got types.QueryParamsResponse
		require.NoError(t, cdc.Unmarshal(reply.Value, &got))
		require.Equal(t, params, got.Params)
		require.Equal(t, hostBefore, repairM1303Snapshot(w.Context, w.HostKey))
		require.Equal(t, payBefore, repairM1303Snapshot(w.Context, w.PayKey))
	}
}

func TestRepairM_1303_DeclarationKeeper(t *testing.T)      { keeper.ProveRepairM1303Keeper(t) }
func TestRepairM_1303_DeclarationConstructor(t *testing.T) { keeper.ProveRepairM1303Constructor(t) }
func TestRepairM_1303_BudgetTimeView(t *testing.T)         { keeper.ProveRepairM1303BudgetTime(t) }
func TestRepairM_1303_EscrowRecovery(t *testing.T)         { keeper.ProveRepairM1303EscrowRecovery(t) }

func TestRepairM_1303_EscrowPublicReads(t *testing.T) {
	keeper.ProveRepairM1303ServiceEscrowCorruption(t)
	w := keeper.OpenRepairM1303World(t)
	w.Capture(t)
	precompile, err := lp.NewPrecompile(w.Keeper)
	require.NoError(t, err)
	contract, err := lp.GetABI()
	require.NoError(t, err)
	// Exercise the production ABI decoder, executor, keeper and output decoder.
	call := func(name string, id [32]byte) ([]interface{}, error) {
		input, err := contract.Pack(name, id)
		require.NoError(t, err)
		method, err := contract.MethodById(input[:4])
		require.NoError(t, err)
		args, err := method.Inputs.Unpack(input[4:])
		require.NoError(t, err)
		output, err := precompile.GetExecutor().Execute(w.Context, method, common.Address{}, common.Address{}, args, big.NewInt(0), true, nil, nil)
		if err != nil {
			return nil, err
		}
		return method.Outputs.Unpack(output)
	}
	holdKey := types.KernelKVKey(types.EscrowModuleID, append([]byte("hold:"), w.ID[:]...))
	resultKey := types.KernelKVKey(types.EscrowModuleID, append([]byte("result:"), w.ResultKey[:]...))
	holdWire, resultWire := bytes.Clone(w.Context.KVStore(w.HostKey).Get(holdKey)), bytes.Clone(w.Context.KVStore(w.HostKey).Get(resultKey))
	w.Keeper.SetParams(w.Context, types.Params{Paused: true})
	assertReadable := func() {
		t.Helper()
		hostBefore, payBefore := repairM1303Snapshot(w.Context, w.HostKey), repairM1303Snapshot(w.Context, w.PayKey)
		out, err := call("getEscrow", w.ID)
		require.NoError(t, err)
		tuple := reflect.ValueOf(out[0])
		require.Equal(t, w.Open.Owner, tuple.FieldByName("Owner").Interface())
		require.Equal(t, big.NewInt(575), tuple.FieldByName("LockedAmount").Interface())
		out, err = call("escrowRemaining", w.ID)
		require.NoError(t, err)
		require.Equal(t, big.NewInt(575), out[0])
		out, err = call("getEscrowResult", w.ResultKey)
		require.NoError(t, err)
		require.Equal(t, resultWire, out[0])
		decoded, err := types.DecodeEscrowResult(out[0].([]byte))
		require.NoError(t, err)
		require.Equal(t, codec.U128{Lo: 25}, decoded.Amount)
		require.Equal(t, w.Receipt.GlobalSequence, decoded.GlobalSequence)
		require.True(t, decoded.ContextBound)
		require.Equal(t, hostBefore, repairM1303Snapshot(w.Context, w.HostKey))
		require.Equal(t, payBefore, repairM1303Snapshot(w.Context, w.PayKey))
	}
	assertReadable()
	for _, family := range []string{"hold", "result"} {
		key, wire := holdKey, holdWire
		if family == "result" {
			key, wire = resultKey, resultWire
		}
		for _, damage := range []string{"short", "identifier_or_version"} {
			t.Run(family+"_"+damage, func(t *testing.T) {
				bad := bytes.Clone(wire)
				if damage == "short" {
					bad = bad[:len(bad)-1]
				} else if family == "hold" {
					bad[0] ^= 1
				} else {
					bad[243] = 1
				}
				w.Context.KVStore(w.HostKey).Set(key, bad)
				before := repairM1303Snapshot(w.Context, w.HostKey)
				names := []string{"getEscrowResult"}
				id := w.ResultKey
				if family == "hold" {
					names, id = []string{"getEscrow", "escrowRemaining"}, w.ID
				}
				for _, name := range names {
					var readErr error
					require.NotPanics(t, func() { _, readErr = call(name, id) })
					require.ErrorIs(t, readErr, keeper.ErrEscrowCorruptState)
				}
				require.Equal(t, before, repairM1303Snapshot(w.Context, w.HostKey))
				w.Context.KVStore(w.HostKey).Set(key, wire)
				assertReadable()
			})
		}
	}
	for _, name := range []string{"getEscrow", "escrowRemaining", "getEscrowResult"} {
		_, err := call(name, [32]byte{0xee})
		require.ErrorContains(t, err, "LXP_UNKNOWN_FIELD")
		require.NotErrorIs(t, err, keeper.ErrEscrowCorruptState)
	}
	w.Reopen(t)
	precompile, err = lp.NewPrecompile(w.Keeper)
	require.NoError(t, err)
	assertReadable()
}
