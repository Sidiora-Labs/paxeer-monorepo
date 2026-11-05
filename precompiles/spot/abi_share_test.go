package spot_test

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type shareGoldenRow struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Signature string `json:"signature"`
	ID        string `json:"id"`
}

func TestPECORVaultShareABIGolden(t *testing.T) {
	requireShareSnapshot(t)
	t.Run("supported", requireNativeShareSupported)
	t.Run("unsupported", requireNativeShareUnsupported)
	t.Run("revert_bytes", requireNativeShareReverts)
}

func requireShareSnapshot(t *testing.T) {
	t.Helper()
	abiBz, err := os.ReadFile("abi/share-solidity.json")
	require.NoError(t, err)
	parsed, err := abi.JSON(bytes.NewReader(abiBz))
	require.NoError(t, err)

	goldenBz, err := os.ReadFile("testdata/share.golden.json")
	require.NoError(t, err)
	var rows []shareGoldenRow
	dec := json.NewDecoder(bytes.NewReader(goldenBz))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&rows))

	snapshot := map[string]shareGoldenRow{}
	for name, m := range parsed.Methods {
		snapshot["function "+name] = shareGoldenRow{Kind: "function", Name: name, Signature: m.Sig, ID: hexutil.Encode(m.ID)}
	}
	for name, e := range parsed.Events {
		snapshot["event "+name] = shareGoldenRow{Kind: "event", Name: name, Signature: e.Sig, ID: e.ID.Hex()}
	}
	for name, e := range parsed.Errors {
		snapshot["error "+name] = shareGoldenRow{Kind: "error", Name: name, Signature: e.Sig, ID: hexutil.Encode(e.ID[:4])}
	}

	golden := map[string]shareGoldenRow{}
	for _, row := range rows {
		key := row.Kind + " " + row.Name
		_, dup := golden[key]
		require.False(t, dup, "duplicate golden row %s", key)
		golden[key] = row
	}

	for key, row := range golden {
		got, ok := snapshot[key]
		require.True(t, ok, "golden row %s %s missing from the snapshot ABI", key, row.ID)
		require.Equal(t, row, got, key)
	}
	for key, got := range snapshot {
		_, ok := golden[key]
		require.True(t, ok, "snapshot ABI entry %s %s missing from the golden fixture", key, got.ID)
	}
	require.Len(t, snapshot, 25)

	require.Equal(t, "0xa9059cbb", hexutil.Encode(parsed.Methods["transfer"].ID))
	require.Equal(t, "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef", parsed.Events["Transfer"].ID.Hex())
	require.Equal(t, "0xe450d38c", snapshot["error ERC20InsufficientBalance"].ID)
	require.Equal(t, "view", parsed.Methods["balanceOf"].StateMutability)
	require.Equal(t, "nonpayable", parsed.Methods["mint"].StateMutability)
	require.True(t, parsed.Events["Transfer"].Inputs[0].Indexed)
	require.True(t, parsed.Events["Transfer"].Inputs[1].Indexed)
	require.False(t, parsed.Events["Transfer"].Inputs[2].Indexed)
	require.Len(t, parsed.Constructor.Inputs, 4)
	require.False(t, parsed.HasReceive())
	require.False(t, parsed.HasFallback())
}

func requireNativeShareSupported(t *testing.T) {
	t.Helper()
	e := newVaultEnv(t)
	client, err := spotabi.Share()
	require.NoError(t, err)
	require.Equal(t, e.share, client)
	holder, pointer := e.shareHolder()
	registered, _, exists := e.app.EvmKeeper.GetERC20NativePointer(e.ctx, e.k.ShareDenom(e.usdl))
	require.True(t, exists)
	require.Equal(t, registered, pointer)
	require.Equal(t, pointer, e.view("shareTokens", e.usdl)[0])
	require.NotEmpty(t, e.app.EvmKeeper.GetCode(e.ctx, pointer))

	views := []struct {
		name string
		args []interface{}
		want interface{}
	}{
		{"name", nil, "Spot Vault Share USDL"},
		{"nname", nil, "Spot Vault Share USDL"},
		{"symbol", nil, "sUSDL"},
		{"ssymbol", nil, "sUSDL"},
		{"decimals", nil, uint8(6)},
		{"ddecimals", nil, uint8(6)},
		{"denom", nil, e.k.ShareDenom(e.usdl)},
		{"BankPrecompile", nil, common.HexToAddress("0x1001")},
		{"totalSupply", nil, big.NewInt(5_000_000)},
		{"balanceOf", []interface{}{holder}, big.NewInt(4_999_000)},
		{"allowance", []interface{}{holder, vaultAddress}, big.NewInt(0)},
	}
	covered := []string{"approve", "transfer", "transferFrom"}
	for _, view := range views {
		t.Run(view.name, func(t *testing.T) {
			input, err := client.Pack(view.name, view.args...)
			require.NoError(t, err)
			caller := e.app.EvmKeeper.GetPaxAddressOrDefault(e.ctx, holder)
			ret, err := e.app.EvmKeeper.StaticCallEVM(e.ctx, caller, &pointer, input)
			require.NoError(t, err)
			out, err := client.Unpack(view.name, ret)
			require.NoError(t, err)
			requireShareValues(t, []interface{}{view.want}, out)
			wantBytes, err := client.Methods[view.name].Outputs.Pack(view.want)
			require.NoError(t, err)
			require.Equal(t, wantBytes, ret)
		})
		covered = append(covered, view.name)
	}
	var declared []string
	for name := range client.Methods {
		declared = append(declared, name)
	}
	require.ElementsMatch(t, declared, covered)

	spender := vaultTestAddress(t.Name() + " spender")
	recipient := vaultTestAddress(t.Name() + " recipient")
	for _, call := range []struct {
		name   string
		caller common.Address
		args   []interface{}
	}{
		{"approve", holder, []interface{}{spender, big.NewInt(2_000_000)}},
		{"transfer", holder, []interface{}{recipient, big.NewInt(1_000_000)}},
		{"transferFrom", spender, []interface{}{holder, recipient, big.NewInt(1_500_000)}},
	} {
		ret, err := e.pointerCallABI(pointer, call.caller, client, call.name, call.args...)
		require.NoError(t, err, call.name)
		out, err := client.Unpack(call.name, ret)
		require.NoError(t, err)
		require.Equal(t, []interface{}{true}, out)
	}
	requireAmount(t, 2_499_000, e.balance(pointer, holder))
	requireAmount(t, 2_500_000, e.balance(pointer, recipient))
	requireAmount(t, 500_000, e.tokenView(pointer, "allowance", holder, spender))
	requireAmount(t, 5_000_000, e.tokenView(pointer, "totalSupply"))
	denom := e.k.ShareDenom(e.usdl)
	bankHolder := e.app.EvmKeeper.GetPaxAddressOrDefault(e.ctx, holder)
	bankRecipient := e.app.EvmKeeper.GetPaxAddressOrDefault(e.ctx, recipient)
	require.Equal(t, "2499000", e.app.BankKeeper.GetBalance(e.ctx, bankHolder, denom).Amount.String())
	require.Equal(t, "2500000", e.app.BankKeeper.GetBalance(e.ctx, bankRecipient, denom).Amount.String())
	require.Equal(t, "5000000", e.app.BankKeeper.GetSupply(e.ctx, denom).Amount.String())
}

func requireShareValues(t *testing.T, want, got []interface{}) {
	t.Helper()
	require.Len(t, got, len(want))
	for i, value := range want {
		if amount, ok := value.(*big.Int); ok {
			actual, ok := got[i].(*big.Int)
			require.True(t, ok, "argument %d: %T is not an amount", i, got[i])
			require.Zero(t, amount.Cmp(actual), "argument %d: want %s, got %s", i, amount, actual)
		} else {
			require.Equal(t, value, got[i], "argument %d", i)
		}
	}
}

func requireNativeShareUnsupported(t *testing.T) {
	t.Helper()
	e := newVaultEnv(t)
	holder, pointer := e.shareHolder()
	historical, err := spotabi.HistoricalSolidityShare()
	require.NoError(t, err)
	recipient := vaultTestAddress(t.Name() + " recipient")
	cases := []struct {
		name     string
		selector string
		args     []interface{}
	}{
		{"mint", "0x40c10f19", []interface{}{recipient, big.NewInt(1_000)}},
		{"burn", "0x9dc29fac", []interface{}{holder, big.NewInt(1_000)}},
		{"owner", "0x8da5cb5b", nil},
		{"renounceOwnership", "0x715018a6", nil},
		{"transferOwnership", "0xf2fde38b", []interface{}{recipient}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotContains(t, e.share.Methods, tc.name)
			require.Equal(t, tc.selector, hexutil.Encode(historical.Methods[tc.name].ID))
			for _, caller := range []common.Address{holder, vaultAddress, types.GovernanceEVMAddress()} {
				ret, err := e.pointerCallABI(pointer, caller, historical, tc.name, tc.args...)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.Len(t, ret, 0, "unsupported %s from %s", tc.name, caller)
				requireAmount(t, 4_999_000, e.balance(pointer, holder))
				requireAmount(t, 0, e.balance(pointer, recipient))
				requireAmount(t, 1_000, e.balance(pointer, e.k.CustodyEVMAddress(e.ctx)))
				requireAmount(t, 5_000_000, e.tokenView(pointer, "totalSupply"))
			}
		})
	}
}

func requireNativeShareReverts(t *testing.T) {
	t.Helper()
	e := newVaultEnv(t)
	holder, pointer := e.shareHolder()
	spender := vaultTestAddress(t.Name() + " spender")
	recipient := vaultTestAddress(t.Name() + " recipient")
	assertRevert := func(caller common.Address, method, errorName, selector string, errorArgs []interface{}, args ...interface{}) {
		t.Helper()
		ret, err := e.pointerCall(pointer, caller, method, args...)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		declared, ok := e.share.Errors[errorName]
		require.True(t, ok)
		require.Equal(t, selector, hexutil.Encode(declared.ID[:4]))
		payload, err := declared.Inputs.Pack(errorArgs...)
		require.NoError(t, err)
		require.Equal(t, append(common.FromHex(selector), payload...), ret)
		decoded, err := declared.Unpack(ret)
		require.NoError(t, err)
		values, ok := decoded.([]interface{})
		require.True(t, ok, "%T is not an ABI argument list", decoded)
		requireShareValues(t, errorArgs, values)
		requireAmount(t, 4_999_000, e.balance(pointer, holder))
		requireAmount(t, 0, e.balance(pointer, recipient))
		requireAmount(t, 5_000_000, e.tokenView(pointer, "totalSupply"))
	}
	assertRevert(holder, "transfer", "ERC20InsufficientBalance", "0xe450d38c",
		[]interface{}{holder, big.NewInt(4_999_000), big.NewInt(4_999_001)}, recipient, big.NewInt(4_999_001))
	assertRevert(spender, "transferFrom", "ERC20InsufficientAllowance", "0xfb8f41b2",
		[]interface{}{spender, big.NewInt(0), big.NewInt(1)}, holder, recipient, big.NewInt(1))
	_, err := e.pointerCall(pointer, holder, "approve", spender, big.NewInt(5_000_000))
	require.NoError(t, err)
	assertRevert(spender, "transferFrom", "ERC20InsufficientBalance", "0xe450d38c",
		[]interface{}{holder, big.NewInt(4_999_000), big.NewInt(5_000_000)}, holder, recipient, big.NewInt(5_000_000))
	requireAmount(t, 5_000_000, e.tokenView(pointer, "allowance", holder, spender))
	assertRevert(holder, "transfer", "ERC20InvalidReceiver", "0xec442f05",
		[]interface{}{common.Address{}}, common.Address{}, big.NewInt(1))
	assertRevert(holder, "approve", "ERC20InvalidSpender", "0x94280d62",
		[]interface{}{common.Address{}}, common.Address{}, big.NewInt(1))
	assertRevert(common.Address{}, "approve", "ERC20InvalidApprover", "0xe602df05",
		[]interface{}{common.Address{}}, spender, big.NewInt(1))
	assertRevert(common.Address{}, "transfer", "ERC20InvalidSender", "0x96c6fd1e",
		[]interface{}{common.Address{}}, recipient, big.NewInt(1))
}
