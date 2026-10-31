package served_test

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func newServedApp(t *testing.T) *app.TestWrapper {
	t.Helper()
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), true)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	w.Ctx = w.Ctx.WithBlockHeight(100)
	w.App.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
	require.EqualValues(t, 100, w.App.UpgradeKeeper.GetDoneHeight(w.Ctx, app.MergeUpgrade))
	return w
}

func TestAgentMergeAddressesServedAfterUpgrade(t *testing.T) {
	w := newServedApp(t)
	want := map[common.Address]struct {
		typeName string
		method   string
	}{
		common.HexToAddress("0x0901"): {"*common.Precompile", "resolveOffset"},
		common.HexToAddress("0x0902"): {"*clearing.ClearingPrecompile", "computeClearing"},
		common.HexToAddress("0x0903"): {"*common.Precompile", "aggregate"},
		common.HexToAddress("0x0904"): {"*pofq.PofqPrecompile", "scoreBatch"},
		common.HexToAddress("0x0905"): {"*scheduler.SchedulerPrecompile", "schedule"},
		common.HexToAddress("0x0907"): {"*teeattestor.TeeAttestorPrecompile", "verify"},
		common.HexToAddress("0x0908"): {"*common.Precompile", "domainSeparator"},
	}
	checked := 0
	for _, row := range precompiles.MergeAddressTable {
		switch row.Address {
		case common.HexToAddress("0x101A"), common.HexToAddress("0x101B"),
			common.HexToAddress("0x101C"), common.HexToAddress("0x101D"), common.HexToAddress("0x0906"):
			continue
		}
		checked++
		t.Run(row.Address.Hex(), func(t *testing.T) {
			expected, known := want[row.Address]
			require.True(t, known, "unaccounted merge address %s", row.Address)
			for _, tracing := range []bool{false, true} {
				before := w.Ctx.WithBlockHeight(99).WithIsTracing(tracing)
				require.NotContains(t, w.App.EvmKeeper.CustomPrecompiles(before), row.Address)
				require.NotContains(t, w.App.EvmKeeper.GetCustomPrecompilesVersions(before), row.Address)
				after := w.Ctx.WithIsTracing(tracing)
				served := w.App.EvmKeeper.CustomPrecompiles(after)
				require.Contains(t, served, row.Address)
				contract, ok := served[row.Address].(precompiles.IPrecompile)
				require.True(t, ok)
				require.Equal(t, expected.typeName, fmt.Sprintf("%T", contract))
				require.Equal(t, row.Address, contract.Address())
				require.Equal(t, row.Name, contract.GetName())
				require.Contains(t, contract.GetABI().Methods, expected.method)
				require.Equal(t, app.MergeUpgrade, w.App.EvmKeeper.GetCustomPrecompilesVersions(after)[row.Address])
			}
		})
	}
	require.Equal(t, len(want), checked)
}

func TestTeeAttestorServedRevertsRegistryCode(t *testing.T) {
	w := newServedApp(t)
	ctx := w.Ctx.WithEventManager(sdk.NewEventManager())
	keys := make([]*ecdsa.PrivateKey, 5)
	for i := range keys {
		seed := make([]byte, 32)
		seed[0], seed[31] = 0x38, byte(i+1)
		key, err := crypto.ToECDSA(seed)
		require.NoError(t, err)
		keys[i] = key
		signer := crypto.PubkeyToAddress(key.PublicKey)
		require.NoError(t, w.App.XWebKeeper.RegisterAttestor(ctx, xwebtypes.MsgRegisterAttestor{
			Authority: xwebtypes.DefaultAuthority(),
			Attestor:  xwebtypes.Attestor{Signer: xwebtypes.Address20(signer), Payout: sdk.AccAddress(signer.Bytes()).String()},
		}))
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(crypto.PubkeyToAddress(keys[i].PublicKey).Bytes(), crypto.PubkeyToAddress(keys[j].PublicKey).Bytes()) < 0
	})
	set := w.App.XWebKeeper.GetAttestorSet(ctx)
	require.Len(t, set.Attestors, 5)
	require.EqualValues(t, 3, set.Threshold)
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	address := common.HexToAddress("0x0907")
	served := w.App.EvmKeeper.CustomPrecompiles(ctx)
	require.Contains(t, served, address)
	contract, ok := served[address].(precompiles.IPrecompile)
	require.True(t, ok)
	a := contract.GetABI()
	require.Contains(t, a.Events, "AttestationVerified")
	db := state.NewDBImpl(ctx, &w.App.EvmKeeper, false)
	block, err := w.App.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(w.App.EvmKeeper.ChainID(db.Ctx()))
	evm := vm.NewEVM(*block, db, chain, vm.Config{}, served)
	caller := common.HexToAddress("0xa2381")
	input := func(method string, args ...interface{}) []byte {
		t.Helper()
		data, err := a.Pack(method, args...)
		require.NoError(t, err)
		return data
	}
	view := func(method string, args ...interface{}) interface{} {
		t.Helper()
		output, _, err := evm.StaticCall(caller, address, input(method, args...), 2_000_000)
		require.NoError(t, err)
		require.NoError(t, db.Err())
		values, err := a.Methods[method].Outputs.Unpack(output)
		require.NoError(t, err)
		require.Len(t, values, 1)
		require.Empty(t, db.GetAllLogs())
		require.Empty(t, db.Ctx().EventManager().Events())
		return values[0]
	}
	for family := uint8(0); family < 4; family++ {
		require.Equal(t, big.NewInt(int64(len(set.Attestors))), view("rootCount", family))
		for index, attestor := range set.Attestors {
			require.Equal(t, attestor.Signer[:], view("rootOf", family, big.NewInt(int64(index))))
		}
		require.Empty(t, view("rootOf", family, big.NewInt(int64(len(set.Attestors)))))
	}
	require.Zero(t, view("rootCount", uint8(9)).(*big.Int).Sign())
	require.Empty(t, view("rootOf", uint8(9), new(big.Int)))
	quote := make([]byte, 96)
	copy(quote, "PXVE")
	quote[4] = 1
	quote[7], quote[39] = 0x31, 0x42
	binary.BigEndian.PutUint64(quote[71:79], uint64(ctx.BlockTime().Unix()))
	chainID := w.App.EvmKeeper.ChainID(ctx)
	require.True(t, chainID.IsUint64())
	binary.BigEndian.PutUint64(quote[79:87], chainID.Uint64())
	binary.BigEndian.PutUint64(quote[87:95], 1)
	quote[95] = byte(set.Threshold)
	digest := crypto.Keccak256(quote[:95])
	for _, key := range keys[:set.Threshold] {
		signature, err := crypto.Sign(digest, key)
		require.NoError(t, err)
		signature[64] += 27
		quote = append(quote, signature...)
	}
	type attestation struct {
		Family     uint8
		Mrtd       [32]byte
		ReportData [32]byte
		Timestamp  uint64
		Debug      bool
	}
	want := attestation{Mrtd: [32]byte{0x31}, ReportData: [32]byte{0x42}, Timestamp: uint64(ctx.BlockTime().Unix())}
	for _, static := range []bool{false, true} {
		data := input("verify", uint8(0), quote)
		var output []byte
		if static {
			output, _, err = evm.StaticCall(caller, address, data, 2_000_000)
		} else {
			output, _, err = evm.Call(caller, address, data, 2_000_000, uint256.NewInt(0))
		}
		require.NoError(t, err)
		values, err := a.Methods["verify"].Outputs.Unpack(output)
		require.NoError(t, err)
		require.Len(t, values, 1)
		require.Equal(t, &want, abi.ConvertType(values[0], new(attestation)))
		require.Empty(t, db.GetAllLogs())
		require.Empty(t, db.Ctx().EventManager().Events())
		data = input("verify", uint8(9), quote)
		if static {
			output, _, err = evm.StaticCall(caller, address, data, 2_000_000)
		} else {
			output, _, err = evm.Call(caller, address, data, 2_000_000, uint256.NewInt(0))
		}
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, pcommon.LXPRevert("LXP_AGENT_TEE_UNKNOWN_FAMILY", ""), output)
		reason, err := abi.UnpackRevert(output)
		require.NoError(t, err)
		require.Equal(t, "LXP_AGENT_TEE_UNKNOWN_FAMILY: unknown family", reason)
		require.Empty(t, db.GetAllLogs())
		require.Empty(t, db.Ctx().EventManager().Events())
		require.NoError(t, db.Err())
	}
	require.Equal(t, set, w.App.XWebKeeper.GetAttestorSet(db.Ctx()))
}
