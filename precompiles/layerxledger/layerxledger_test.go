package layerxledger_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestLedgerMergeConstructorRegistered(t *testing.T) {
	h := newHarness(t)
	address := common.HexToAddress(layerxledger.LedgerAddress)
	require.Equal(t, common.HexToAddress("0x101A"), address)
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, layerxledger.NewMergePrecompile),
		fmt.Sprintf("merge constructor for %s already registered", address.Hex()))
	p, err := layerxledger.NewMergePrecompile(h.keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.Equal(t, address, p.Address())
	require.Equal(t, "layerxledger", p.GetName())
	found := false
	for _, entry := range precompiles.MergeAddressTable {
		if entry.Address == address {
			found = true
			require.Equal(t, entry.Name, p.GetName())
		}
	}
	require.True(t, found)
	built := precompiles.GetCustomPrecompiles(precompiles.MergeUpgrade, h.keepers)
	require.Contains(t, built, address)
	require.Contains(t, built[address], precompiles.MergeUpgrade)
	served, ok := built[address][precompiles.MergeUpgrade].(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, address, served.Address())
	require.Equal(t, "layerxledger", served.GetName())
	for name, method := range p.GetABI().Methods {
		packed, err := method.Inputs.Pack(ledgerArguments(name)...)
		require.NoError(t, err, name)
		input := append(append([]byte(nil), method.ID...), packed...)
		require.Equal(t, layerxledger.GasTable[name].Gas(packed), p.RequiredGas(input), name)
	}
}

func TestLedgerWriteRefusals(t *testing.T) {
	for _, refusal := range []struct {
		name     string
		bound    bool
		readOnly bool
		delegate bool
		value    *big.Int
		reason   string
	}{
		{name: "staticcall", bound: true, readOnly: true, reason: "LXP_UNAUTHORIZED_DEBIT: cannot call a layerxledger state change from staticcall"},
		{name: "delegatecall", bound: true, delegate: true, reason: "LXP_UNAUTHORIZED_DEBIT: cannot delegatecall layerxledger"},
		{name: "value", bound: true, value: big.NewInt(1), reason: "LXP_PRECOMPILE_NON_PAYABLE: sending funds to a non-payable function"},
		{name: "unbound", reason: "LXP_UNAUTHORIZED_DEBIT"},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			h := newHarness(t)
			if refusal.bound {
				h.bindCaller(t)
			} else {
				_, err := h.keeper.Actor(h.ctx, h.caller)
				require.Error(t, err)
			}
			for name, method := range h.precompile.GetABI().Methods {
				if method.IsConstant() {
					continue
				}
				t.Run(name, func(t *testing.T) {
					packed, err := method.Inputs.Pack(ledgerArguments(name)...)
					require.NoError(t, err)
					input := append(append([]byte(nil), method.ID...), packed...)
					hostBefore := storeSnapshot(t, h.ctx.KVStore(h.hostKey))
					ledgerBefore := storeSnapshot(t, h.ctx.KVStore(h.storeKey))
					eventsBefore := len(h.ctx.EventManager().Events())
					logsBefore := len(h.stateDB.GetAllLogs())
					data, err := h.precompile.Run(h.evm, h.caller, common.HexToAddress("0xdead"), input,
						refusal.value, refusal.readOnly, refusal.delegate, nil)
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					assertLedgerRevert(t, data, refusal.reason)
					require.Equal(t, hostBefore, storeSnapshot(t, h.ctx.KVStore(h.hostKey)))
					require.Equal(t, ledgerBefore, storeSnapshot(t, h.ctx.KVStore(h.storeKey)))
					require.Len(t, h.ctx.EventManager().Events(), eventsBefore)
					require.Len(t, h.stateDB.GetAllLogs(), logsBefore)
				})
			}
		})
	}
}

func ledgerArguments(name string) []interface{} {
	id := [32]byte{1}
	switch name {
	case "register":
		return []interface{}{id, [32]byte{2}, "TST", "Test asset", uint8(6), big.NewInt(100), uint8(0), []byte{}}
	case "pause", "unpause", "openAccount", "getAsset", "totalUnits", "accountOwner", "getPayerGrant":
		return []interface{}{id}
	case "send", "issueGrant":
		return []interface{}{[]byte{}}
	case "draw":
		return []interface{}{id, id, big.NewInt(1), id, uint64(0), id, id}
	case "payExact":
		return []interface{}{id, id, id, big.NewInt(1), uint64(0), id}
	case "revokeGrant":
		return []interface{}{id, uint64(1)}
	case "mint", "burn":
		return []interface{}{id, id, big.NewInt(1)}
	case "balanceOf", "accountId", "nextSequence":
		return []interface{}{id, id}
	default:
		panic("missing test arguments for " + name)
	}
}

func assertLedgerRevert(t *testing.T, data []byte, expected string) {
	t.Helper()
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(reason, "LXP_"), reason)
	require.Equal(t, expected, reason)
}
