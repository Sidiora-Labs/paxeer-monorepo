package layerxprograms_test

import (
	"math/big"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestProgramsMergeConstructorRegistered(t *testing.T) {
	address := lp.Address
	var rows int
	for _, row := range precompiles.MergeAddressTable {
		if row.Address == address {
			require.Equal(t, lp.PrecompileName, row.Name)
			rows++
		}
	}
	require.Equal(t, 1, rows)
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, lp.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")
	for _, keepers := range []utils.Keepers{nil, &utils.EmptyKeepers{}} {
		p, err := lp.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
		require.Nil(t, p)
		require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT:")
	}

	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: sdk.NewKVStoreKey("programs-constructor"), NetworkID: 7,
		ParameterVersion: 1, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	evm, upgrade := &evmkeeper.Keeper{}, &upgradekeeper.Keeper{}
	p, err := lp.NewPrecompile(host, evm, upgrade, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.Equal(t, address, p.Address())
	require.Equal(t, lp.PrecompileName, p.GetName())
	require.Equal(t, lp.ProgramsABI, p.GetABI())
	method := lp.ProgramsABI.Methods["call"]
	input, err := lp.ProgramsABI.Pack("call", [32]byte{1}, uint16(1), "main", []byte{}, []byte{}, []byte{}, uint32(64), [7]uint64{1}, big.NewInt(10))
	require.NoError(t, err)
	require.Equal(t, lp.GasTable[method.Name].Gas(input[4:]), p.RequiredGas(input))
	executor := &lp.PrecompileExecutor{}
	require.Equal(t, p.RequiredGas(input), executor.RequiredGas(input[4:], &method))
	for _, tc := range []struct {
		host    *lxphost.Host
		evm     lp.EVMKeeper
		upgrade lp.UpgradeKeeper
		plan    string
	}{
		{nil, evm, upgrade, precompiles.MergeUpgrade},
		{host, nil, upgrade, precompiles.MergeUpgrade},
		{host, evm, nil, precompiles.MergeUpgrade},
		{host, evm, upgrade, ""},
	} {
		p, err := lp.NewPrecompile(tc.host, tc.evm, tc.upgrade, tc.plan)
		require.Nil(t, p)
		require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT:")
	}
}

func TestProgramsExecuteUnknownMethodRefused(t *testing.T) {
	executor := &lp.PrecompileExecutor{}
	ctx := sdk.Context{}.WithBlockHeight(12)
	caller, contract := common.HexToAddress("0x1234"), common.HexToAddress("0x5678")
	for _, method := range []*abi.Method{nil, {Name: "unregistered"}} {
		out, err := executor.Execute(ctx, method, caller, contract, nil, nil, false, nil, nil)
		require.Nil(t, out)
		require.ErrorContains(t, err, "LXP_PRECOMPILE_UNKNOWN_METHOD:")
		reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
		require.NoError(t, unpackErr)
		require.Equal(t, err.Error(), reason)
	}

	budget := [7]uint64{1, 2, 3, 4, 5, 6, 7}
	fee := big.NewInt(0x10203)
	legs := []lp.TransferLeg{{From: [32]byte{2}, Asset: [32]byte{3}, To: [32]byte{4}, Amount: big.NewInt(5)}}
	for _, tc := range []struct {
		name string
		args []interface{}
	}{
		{"call", []interface{}{[32]byte{1}, uint16(2), "main", []byte{3}, []byte{4}, []byte{5}, uint32(64), budget, fee}},
		{"transfer", []interface{}{[32]byte{1}, legs}},
		{"getProgram", []interface{}{[32]byte{1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			method := lp.ProgramsABI.Methods[tc.name]
			packed, err := method.Inputs.Pack(tc.args...)
			require.NoError(t, err)
			args, err := method.Inputs.Unpack(packed)
			require.NoError(t, err)
			out, err := executor.Execute(ctx, &method, caller, contract, args, nil, false, nil, nil)
			require.Nil(t, out)
			require.EqualError(t, err, "LXP_PRECOMPILE_UNKNOWN_METHOD: "+tc.name)
		})
	}

	t.Run("registryRead executes once with decoded arguments", func(t *testing.T) {
		h := newHarness(t)
		ctx := h.stateDB.Ctx()
		method := lp.ProgramsABI.Methods["registryRead"]
		program, key := [32]byte{0xa1}, []byte("manifest")
		packed, err := method.Inputs.Pack(program, key)
		require.NoError(t, err)
		args, err := method.Inputs.Unpack(packed)
		require.NoError(t, err)
		payload, err := lp.EncodeRegistryReadPayload(program, key)
		require.NoError(t, err)
		reference, _ := ctx.CacheContext()
		receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(h.caller, &method), 0x00090004, payload, [16]byte{})
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		require.Equal(t, uint64(1), receipt.GlobalSequence)
		require.Equal(t, []lxphost.ReceiptEvent{{Code: 4, Body: program[:]}}, receipt.Events)

		_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
		require.NoError(t, err)
		sequence, err := h.host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		require.Zero(t, sequence)
		out, err := h.executor.Execute(ctx, &method, h.caller, contract, args, big.NewInt(0), false, h.evm, nil)
		require.NoError(t, err)
		require.Nil(t, out)
		sequence, err = h.host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		require.Equal(t, uint64(1), sequence)
		require.Equal(t, h.dump(reference), h.dump(ctx))
	})
}
