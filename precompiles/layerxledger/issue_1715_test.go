package layerxledger_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestIssue1715WebNextSequence(t *testing.T) {
	f := newRepair144Fixture(t)
	require.NotEqual(t, f.payee.DIDKey, f.to)
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	produce := func(recipient, asset [32]byte) (common.Address, []byte) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "cargo", "run", "--quiet", "--manifest-path", "services/agent/Cargo.toml", "-p", "layerx-mcp", "--example", "issue_1715_next_sequence", "--",
			hex.EncodeToString(f.payee.DIDKey[:]), hex.EncodeToString(recipient[:]), hex.EncodeToString(asset[:]))
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		require.NoError(t, err, "Rust producer must succeed: %s", stderr.String())
		require.Len(t, output, 20+4+32+32)
		target := common.BytesToAddress(output[:20])
		input := output[20:]
		require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), target)
		require.Equal(t, []byte{0xce, 0x2f, 0x92, 0x1f}, input[:4])
		require.Equal(t, f.p.abi.Methods["nextSequence"].ID, input[:4])
		require.Equal(t, recipient[:], input[4:36])
		require.Equal(t, asset[:], input[36:68])
		return target, input
	}
	staticCall := func(target common.Address, input []byte) ([]byte, error) {
		t.Helper()
		before := storeSnapshot(t, f.p.ctx.KVStore(f.p.app.GetKey(app.LayerXHostStoreKey)))
		db := state.NewDBImpl(f.p.ctx.WithEventManager(sdk.NewEventManager()), &f.p.app.EvmKeeper, false)
		block, err := f.p.app.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
		require.NoError(t, err)
		config := evmtypes.DefaultChainConfig().EthereumConfig(f.p.app.EvmKeeper.ChainID(f.p.ctx))
		evm := vm.NewEVM(*block, db, config, vm.Config{}, f.p.app.EvmKeeper.CustomPrecompiles(db.Ctx()))
		evm.TxContext.Origin = f.recipient
		require.Contains(t, f.p.app.EvmKeeper.CustomPrecompiles(db.Ctx()), target)
		data, _, callErr := evm.StaticCall(f.recipient, target, input, 2_000_000)
		require.Empty(t, db.GetAllLogs())
		if callErr == nil {
			_, err = db.Finalize()
			require.NoError(t, err)
		}
		require.Equal(t, before, storeSnapshot(t, f.p.ctx.KVStore(f.p.app.GetKey(app.LayerXHostStoreKey))))
		return data, callErr
	}
	target, input := produce(f.to, f.asset)
	read := func(want uint64) []byte {
		t.Helper()
		data, err := staticCall(target, input)
		require.NoError(t, err, "native nextSequence: %x", data)
		values, err := f.p.abi.Methods["nextSequence"].Outputs.Unpack(data)
		require.NoError(t, err)
		name, id, err := lxphost.AssetAccountID(f.payee.DIDKey, f.asset)
		require.NoError(t, err)
		require.Equal(t, f.to, id)
		account, err := f.p.app.LayerXHost.ReadAccount(f.p.ctx, name, id)
		require.NoError(t, err)
		require.Equal(t, want, account.NextSequence)
		require.Equal(t, []interface{}{account.NextSequence}, values)
		return data
	}
	read(0)
	f.p.success(t, f.sender, "mint", f.asset, f.to, big.NewInt(250))
	debit := func(reference byte) {
		f.p.success(t, f.recipient, "payExact", f.to, f.from, f.asset, big.NewInt(1),
			uint64(f.p.ctx.BlockTime().UnixMilli())+60_000, [32]byte{0x17, reference})
	}
	debit(1)
	read(1)
	f.reopen(t)
	read(1)
	f.balances(t, 1, 249)
	for _, refusal := range []struct {
		recipient, asset [32]byte
		reason           string
	}{
		{f.payee.DIDKey, f.asset, "LXP_UNKNOWN_ACCOUNT_NAMESPACE"},
		{f.to, [32]byte{0xff}, "LXP_ASSET_MISMATCH"},
	} {
		badTarget, badInput := produce(refusal.recipient, refusal.asset)
		data, err := staticCall(badTarget, badInput)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, refusal.reason)
	}
	grantInput, err := f.p.abi.Pack("getPayerGrant", [32]byte{0x17})
	require.NoError(t, err)
	data, err := staticCall(target, grantInput)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, ledgertypes.ErrNoPayerGrant.Error())
	read(1)

	// The historical replay keeps its signed draws; only its sequence-read words change.
	// Confirm its sequence result against a supported native debit history.
	for sequence := byte(2); sequence <= 19; sequence++ {
		debit(sequence)
	}
	f.reopen(t)
	nativeResult := read(19)
	recording, err := os.ReadFile(filepath.Join(root, "services/interop/crates/x-websearch/tests/fixtures/gateway/metered.json"))
	require.NoError(t, err)
	var fixture struct {
		Endpoints struct {
			Gateway map[string][]struct {
				Method string            `json:"method"`
				Params []json.RawMessage `json:"params"`
				Result json.RawMessage   `json:"result"`
			} `json:"gateway"`
		} `json:"endpoints"`
		Ledger struct {
			ReceiverDIDKey   string            `json:"receiverDidKey"`
			ReceiverAccounts map[string]string `json:"receiverAccounts"`
			Grants           []struct {
				Canonical string `json:"canonical"`
			} `json:"grants"`
		} `json:"ledger"`
	}
	require.NoError(t, json.Unmarshal(recording, &fixture))
	observations := 0
	for _, record := range fixture.Endpoints.Gateway["*"] {
		if record.Method != "eth_call" {
			continue
		}
		var call struct{ To, Data string }
		require.NoError(t, json.Unmarshal(record.Params[0], &call))
		if len(call.Data) < 10 || call.Data[:10] != "0xce2f921f" {
			continue
		}
		require.Equal(t, target, common.HexToAddress(call.To))
		require.Len(t, call.Data, 138)
		account, asset := call.Data[10:74], call.Data[74:138]
		require.NotEqual(t, fixture.Ledger.ReceiverDIDKey, account)
		require.Contains(t, fixture.Ledger.ReceiverAccounts, "SID")
		canonicalAccount := false
		for _, id := range fixture.Ledger.ReceiverAccounts {
			canonicalAccount = canonicalAccount || id == account
		}
		require.True(t, canonicalAccount)
		found := false
		for _, grant := range fixture.Ledger.Grants {
			if len(grant.Canonical) >= 256 && grant.Canonical[192:256] == asset && grant.Canonical[128:192] == account {
				found = true
			}
		}
		require.True(t, found, "full account and asset pair must match the historical grant")
		var result string
		require.NoError(t, json.Unmarshal(record.Result, &result))
		require.Equal(t, "0x"+hex.EncodeToString(nativeResult), result)
		observations++
	}
	require.Equal(t, 4, observations)
}
