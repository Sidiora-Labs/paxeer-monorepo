// Proves: orob.ResolveOffset/ResolveOffsetBatch/ToOffset and NewMergePrecompile -> keeper.StaticCallEVM and vm.EVM.Call/StaticCall at 0x0901 -> abi.ABI.Unpack and agentcommon.MethodGas.
// The application installs the registered merge constructor and supplies the real
// upgrade reader, StateDB, precompile dispatch and ABI codec. OROB has no stored
// producer state to reopen: independent EVM instances execute each request twice,
// paying the full gas charge on both calls. Truncated responses are derived from
// actual producer output and refused by the consumer's ABI decoder; they never
// replace the transport or executor. No production defect is assumed by this proof.
package orob_test

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/orob"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProofOrob0x0901(t *testing.T) {
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), true)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	ctx := w.Ctx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	w.App.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	k := &w.App.EvmKeeper
	address := common.HexToAddress(orob.OrobAddress)
	caller := common.HexToAddress("0xa901")
	constructed, err := orob.NewMergePrecompile(w.App.GetPrecompileKeepers(), app.MergeUpgrade)
	require.NoError(t, err)
	require.IsType(t, &agentcommon.Precompile{}, constructed)
	require.Equal(t, address, constructed.Address())
	require.Equal(t, orob.PrecompileName, constructed.GetName())
	require.NotContains(t, k.CustomPrecompiles(ctx.WithBlockHeight(99)), address)
	served := k.CustomPrecompiles(ctx)
	require.Contains(t, served, address)
	p, ok := served[address].(precompiles.IPrecompile)
	require.True(t, ok)
	require.IsType(t, constructed, p)
	require.Equal(t, constructed.GetABI(), p.GetABI())
	require.Equal(t, app.MergeUpgrade, k.GetCustomPrecompilesVersions(ctx)[address])
	a := p.GetABI()
	require.Len(t, a.Methods, 3)

	newEVM := func(t *testing.T) (*vm.EVM, *state.DBImpl) {
		t.Helper()
		db := state.NewDBImpl(ctx, k, false)
		block, err := k.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
		require.NoError(t, err)
		chain := evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(db.Ctx()))
		evm := vm.NewEVM(*block, db, chain, vm.Config{}, k.CustomPrecompiles(db.Ctx()))
		evm.SetTxContext(vm.TxContext{Origin: caller})
		return evm, db
	}
	const suppliedGas uint64 = 2_000_000

	for _, row := range []struct {
		name     string
		method   string
		selector []byte
		args     []interface{}
		want     interface{}
		gas      uint64
	}{
		{"resolve", orob.ResolveOffsetMethod, []byte{0x45, 0x4a, 0xd3, 0xa1}, []interface{}{big.NewInt(2_000_000), int16(250)}, big.NewInt(2_050_000), 1204},
		{"resolve_changed_price", orob.ResolveOffsetMethod, []byte{0x45, 0x4a, 0xd3, 0xa1}, []interface{}{big.NewInt(3), int16(5000)}, big.NewInt(4), 1204},
		{"batch", orob.ResolveOffsetBatchMethod, []byte{0x3d, 0xf5, 0x97, 0xe3}, []interface{}{big.NewInt(10000), []int16{-9999, 0, 32767}}, []*big.Int{big.NewInt(1), big.NewInt(10000), big.NewInt(42767)}, 1588},
		{"offset", orob.ToOffsetMethod, []byte{0x3f, 0x77, 0x9a, 0xef}, []interface{}{big.NewInt(1000), big.NewInt(1025)}, int16(250), 1204},
		{"negative_offset", orob.ToOffsetMethod, []byte{0x3f, 0x77, 0x9a, 0xef}, []interface{}{big.NewInt(3), big.NewInt(1)}, int16(-6666), 1204},
	} {
		t.Run(row.name, func(t *testing.T) {
			input, err := a.Pack(row.method, row.args...)
			require.NoError(t, err)
			require.Equal(t, row.selector, input[:4])
			require.Equal(t, "pure", a.Methods[row.method].StateMutability)
			require.Equal(t, row.gas, agentcommon.MethodGas(input[:4], len(input)))
			require.Equal(t, row.gas, p.RequiredGas(input))
			var previous []byte
			for attempt := 0; attempt < 2; attempt++ {
				evm, db := newEVM(t)
				var output []byte
				var left uint64
				if attempt == 0 {
					output, left, err = evm.StaticCall(caller, address, input, suppliedGas)
				} else {
					output, left, err = evm.Call(caller, address, input, suppliedGas, uint256.NewInt(0))
				}
				require.NoError(t, err)
				require.NoError(t, db.Err())
				require.Equal(t, row.gas, suppliedGas-left, "every invocation must execute and charge gas")
				values, err := a.Unpack(row.method, output)
				require.NoError(t, err)
				require.Equal(t, []interface{}{row.want}, values)
				require.Empty(t, db.GetAllLogs())
				require.Empty(t, db.Ctx().EventManager().Events())
				if attempt == 1 {
					require.Equal(t, previous, output)
				}
				previous = append([]byte(nil), output...)

				// Use the production keeper's own entry point as well as the
				// direct EVM call used to observe the precise gas charge.
				observed, err := k.StaticCallEVM(ctx, sdk.AccAddress(caller.Bytes()), &address, input)
				require.NoError(t, err)
				require.Equal(t, output, observed)
				values, err = a.Unpack(row.method, observed)
				require.NoError(t, err)
				require.Equal(t, []interface{}{row.want}, values)

				_, err = a.Unpack(row.method, observed[:len(observed)-1])
				require.Error(t, err, "consumer must refuse a truncated producer result")
			}

			evm, _ := newEVM(t)
			output, left, err := evm.StaticCall(caller, address, input, row.gas-1)
			require.ErrorIs(t, err, vm.ErrOutOfGas)
			require.Empty(t, output)
			require.Zero(t, left)

			evm, _ = newEVM(t)
			output, _, err = evm.StaticCall(caller, address, input[:len(input)-1], suppliedGas)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(output)
			require.NoError(t, err)
			require.Contains(t, reason, string(agentcommon.ErrABIDecode)+":")
		})
	}

	t.Run("merge_batch_cap", func(t *testing.T) {
		offsets := make([]int16, orob.MergeMaxBatchSize)
		for i := range offsets {
			offsets[i] = int16(i - 32)
		}
		input, err := a.Pack(orob.ResolveOffsetBatchMethod, big.NewInt(10000), offsets)
		require.NoError(t, err)
		for attempt := 0; attempt < 2; attempt++ {
			output, err := k.StaticCallEVM(ctx, sdk.AccAddress(caller.Bytes()), &address, input)
			require.NoError(t, err)
			values, err := a.Unpack(orob.ResolveOffsetBatchMethod, output)
			require.NoError(t, err)
			require.Len(t, values, 1)
			prices, ok := values[0].([]*big.Int)
			require.True(t, ok)
			require.Len(t, prices, orob.MergeMaxBatchSize)
			for i, offset := range offsets {
				require.Equal(t, big.NewInt(10000+int64(offset)), prices[i])
			}
		}
	})

	for _, row := range []struct {
		name   string
		method string
		args   []interface{}
		code   agentcommon.ErrorCode
		reason string
	}{
		{"zero_oracle", orob.ResolveOffsetMethod, []interface{}{big.NewInt(0), int16(0)}, agentcommon.ErrOrobOraclePrice, "LXP_AGENT_OROB_ORACLE_PRICE: oraclePrice cannot be zero"},
		{"invalid_offset", orob.ResolveOffsetMethod, []interface{}{big.NewInt(10000), int16(-10000)}, agentcommon.ErrOrobOffsetRange, "LXP_AGENT_OROB_OFFSET_RANGE: offset out of range"},
		{"oversized_batch", orob.ResolveOffsetBatchMethod, []interface{}{big.NewInt(10000), make([]int16, orob.MergeMaxBatchSize+1)}, agentcommon.ErrBatchTooLarge, "LXP_AGENT_BATCH_TOO_LARGE: batch over the cap (cap set at activation rehearsal)"},
		{"offset_overflow", orob.ToOffsetMethod, []interface{}{big.NewInt(10000), big.NewInt(42768)}, agentcommon.ErrOverflow, "LXP_AGENT_OVERFLOW: result out of range"},
	} {
		t.Run(row.name, func(t *testing.T) {
			input, err := a.Pack(row.method, row.args...)
			require.NoError(t, err)
			evm, db := newEVM(t)
			output, _, err := evm.StaticCall(caller, address, input, suppliedGas)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.ErrorIs(t, db.GetPrecompileError(), row.code)
			reason, err := abi.UnpackRevert(output)
			require.NoError(t, err)
			require.Equal(t, row.reason, reason)
			_, err = k.StaticCallEVM(ctx, sdk.AccAddress(caller.Bytes()), &address, input)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Empty(t, db.GetAllLogs())
		})
	}
	require.EqualValues(t, 100, w.App.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	require.Empty(t, ctx.EventManager().Events())
}
