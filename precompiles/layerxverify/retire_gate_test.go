package layerxverify_test

import (
	"fmt"
	"math/big"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

const verifyRetirementHeight int64 = 100

type retirementVector struct {
	name       string
	method     string
	input      []byte
	valid      bool
	signatures uint64
	nodes      uint64
}

func retirementVectors(t *testing.T) []retirementVector {
	t.Helper()
	h := newHarness(t)
	var vectors []retirementVector
	add := func(name, method string, valid bool, signatures, nodes uint64, args ...interface{}) {
		vectors = append(vectors, retirementVector{
			name: method + "/" + name, method: method, input: h.input(method, args...),
			valid: valid, signatures: signatures, nodes: nodes,
		})
	}
	for _, v := range h.cases("ed25519") {
		domain := layerxverify.RawMessageDomain
		if _, ok := v.Fields["domain"]; ok {
			domain = uint8(number(t, v, "domain")) //nolint:gosec
		}
		add(v.Name, layerxverify.VerifyEd25519Method, v.Valid, 1, 0,
			array(t, v, "public_key"), domain, bytesOf(t, v, "message"), bytesOf(t, v, "signature"))
	}
	for _, v := range h.cases("receipts") {
		add(v.Name, layerxverify.VerifyReceiptMethod, v.Valid, 1, 0,
			bytesOf(t, v, "receipt"), array(t, v, "public_key"))
	}
	for _, v := range h.cases("inclusion") {
		add(v.Name, layerxverify.VerifyReceiptInclusionMethod, v.Valid, 2,
			uint64(len(bytesOf(t, v, "proof")))/32, inclusionArgs(t, v)...)
	}
	for _, v := range h.cases("state") {
		witness := bytesOf(t, v, "witness")
		add(v.Name, layerxverify.VerifyStateProofMethod, v.Valid, 0, uint64(len(witness))/32,
			witness, array(t, v, "state_root"))
	}
	for _, v := range h.cases("discovery") {
		add(v.Name, layerxverify.VerifyDiscoveryProofMethod, v.Valid, 1, 0,
			bytesOf(t, v, "payload"), bytesOf(t, v, "proof_material"), array(t, v, "program_id"),
			number(t, v, "staleness_ms"), array(t, v, "public_key"))
	}
	return vectors
}

func newRetirementHarness(t *testing.T) (*pcommon.Precompile, *vm.EVM, sdk.Context) {
	t.Helper()
	testApp := testkeeper.EVMTestApp
	ctx := testApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(verifyRetirementHeight - 1)
	ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
	testApp.EvmKeeper.UpgradeKeeper().SetDone(ctx.WithBlockHeight(verifyRetirementHeight), pcommon.MergeUpgrade)
	p, err := layerxverify.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	stateDB := &state.DBImpl{}
	stateDB.WithCtx(ctx)
	return p, &vm.EVM{StateDB: stateDB}, ctx
}

func TestEveryLayerXVerifyMethodRevertsModuleDisabledAfterTheMerge(t *testing.T) {
	p, evm, ctx := newRetirementHarness(t)
	require.Equal(t, common.HexToAddress(layerxverify.LayerXVerifyAddress), p.Address())
	require.Len(t, p.GetABI().Methods, 5)
	want := pcommon.RetiredRevert("layerxVerify")
	reason, err := abi.UnpackRevert(want)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_MODULE_DISABLED")
	require.Contains(t, reason, "layerxVerify retired")
	seen := make(map[string]bool)
	for _, vector := range retirementVectors(t) {
		seen[vector.method] = true
		for _, height := range []int64{verifyRetirementHeight, verifyRetirementHeight + 1} {
			for _, call := range []struct {
				name     string
				value    *big.Int
				readOnly bool
				delegate bool
			}{
				{name: "call"},
				{name: "staticcall", readOnly: true},
				{name: "value", value: big.NewInt(1)},
				{name: "delegatecall", delegate: true},
			} {
				t.Run(fmt.Sprintf("%s/%d/%s", vector.name, height, call.name), func(t *testing.T) {
					state.GetDBImpl(evm.StateDB).WithCtx(ctx.WithBlockHeight(height))
					res, err := p.Run(evm, common.Address{}, common.Address{}, vector.input,
						call.value, call.readOnly, call.delegate, nil)
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					require.Equal(t, want, res)
				})
			}
		}
	}
	for name := range p.GetABI().Methods {
		require.True(t, seen[name], name)
	}
}

func TestLayerXVerifyBelowTheMergeKeepsItsResults(t *testing.T) {
	p, evm, ctx := newRetirementHarness(t)
	legacy := newHarness(t)
	for _, vector := range retirementVectors(t) {
		t.Run(vector.name, func(t *testing.T) {
			want, wantErr := legacy.precompile.Run(legacy.evm, common.Address{}, common.Address{},
				vector.input, nil, true, false, nil)
			if vector.valid || vector.method == layerxverify.VerifyEd25519Method {
				require.NoError(t, wantErr)
				require.NotEmpty(t, want)
				if vector.method == layerxverify.VerifyEd25519Method {
					out, err := legacy.method(vector.method).Outputs.Unpack(want)
					require.NoError(t, err)
					require.Equal(t, vector.valid, out[0].(bool))
				}
			} else {
				require.ErrorIs(t, wantErr, vm.ErrExecutionReverted)
				require.Nil(t, want)
			}
			for _, height := range []int64{1, verifyRetirementHeight - 1} {
				state.GetDBImpl(evm.StateDB).WithCtx(ctx.WithBlockHeight(height))
				res, err := p.Run(evm, common.Address{}, common.Address{}, vector.input, nil, true, false, nil)
				require.Equal(t, wantErr, err)
				require.Equal(t, want, res)
			}
		})
	}
	input := retirementVectors(t)[0].input
	for _, call := range []struct {
		value    *big.Int
		delegate bool
	}{
		{value: big.NewInt(1)},
		{delegate: true},
	} {
		res, err := p.Run(evm, common.Address{}, common.Address{}, input, call.value, true, call.delegate, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Nil(t, res)
	}
}

func TestLayerXVerifyRetiredRevertChargesTheDocumentedGas(t *testing.T) {
	p, evm, ctx := newRetirementHarness(t)
	for _, vector := range retirementVectors(t) {
		t.Run(vector.name, func(t *testing.T) {
			wantGas := uint64(3000) + 16*uint64(len(vector.input)-4) + 4000*vector.signatures + 100*vector.nodes
			require.Equal(t, wantGas, p.RequiredGas(vector.input))
			state.GetDBImpl(evm.StateDB).WithCtx(ctx.WithBlockHeight(verifyRetirementHeight))
			res, remaining, err := vm.RunPrecompiledContract(p, evm, common.Address{}, common.Address{},
				vector.input, wantGas-1, nil, nil, true, false)
			require.ErrorIs(t, err, vm.ErrOutOfGas)
			require.Nil(t, res)
			require.Zero(t, remaining)
			for _, extra := range []uint64{0, 17} {
				res, remaining, err = vm.RunPrecompiledContract(p, evm, common.Address{}, common.Address{},
					vector.input, wantGas+extra, nil, nil, true, false)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.Equal(t, pcommon.RetiredRevert("layerxVerify"), res)
				require.Equal(t, extra, remaining)
			}
		})
	}
}
