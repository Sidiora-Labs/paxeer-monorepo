// Proves: bank.PrecompileExecutor -> common.DynamicGasPrecompile.RunAndCalculateGas Error(string) -> layerx_sdk.revert.decode_revert_reason and decodeRevertReason.
// Repairs: Python decode_revert_reason accepted empty code segments, empty
// detail and oversized reasons; TypeScript decodeRevertReason leaked TypeError
// for malformed runtime input. Bank refusal.lxpError now constructs its reason
// directly through common.LXPError instead of an encode/decode fallback.
// Ledger, tokenfactory and bank share the same exported asset-pause code.
// Each invocation is followed by an application commit, close and reopen of the
// same persistent database before balances or ABI results are consumed.
package bank_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	tfkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	tftypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type proofBankReason struct {
	Data      string  `json:"data"`
	Code      string  `json:"code"`
	Message   string  `json:"message"`
	Detail    *string `json:"detail"`
	Retryable bool    `json:"retryable"`
}

type proofBankTranscript struct {
	Reasons   []proofBankReason `json:"reasons"`
	Malformed []string          `json:"malformed"`
	Successes []string          `json:"successes"`
}

func proofBankOpenApp(t *testing.T, dir string, restarting bool) (*app.App, sdk.Context) {
	t.Helper()
	db, err := sdk.NewLevelDB("bank-proof", dir)
	require.NoError(t, err)
	encoding := app.MakeEncodingConfig()
	a := app.New(db, nil, true, map[int64]bool{}, dir, 1, true,
		tmcfg.DefaultConfig().SetRoot(dir), encoding, nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	if !restarting {
		genesis, marshalErr := json.Marshal(app.NewDefaultGenesisState(encoding.Marshaler))
		require.NoError(t, marshalErr)
		_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
			Time: time.Unix(1_800_000_000, 0).UTC(), ChainId: "pax-test",
			ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
		})
		require.NoError(t, err)
	}
	header := tmproto.Header{ChainID: "pax-test", Height: a.LastBlockHeight() + 1, Time: time.Unix(1_800_000_000, 0).UTC()}
	var ctx sdk.Context
	if restarting {
		ctx, _ = a.NewUncachedContext(false, header).CacheContext()
	} else {
		ctx = a.NewContext(false, header)
	}
	return a, ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
}

func proofBankDurablePath(t *testing.T) proofBankTranscript {
	t.Helper()
	dir := t.TempDir()
	a, ctx := proofBankOpenApp(t, dir, false)
	t.Cleanup(func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	})
	k := &a.EvmKeeper
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(100), pcommon.MergeUpgrade)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(200), bank.V6120Upgrade)
	require.Equal(t, app.V6120Upgrade, bank.V6120Upgrade)
	require.Equal(t, "v6.12.0", bank.V6120Upgrade)

	fromEVM, toEVM := common.HexToAddress("0x217701"), common.HexToAddress("0x217702")
	from, to := sdk.AccAddress(fromEVM.Bytes()), sdk.AccAddress(toEVM.Bytes())
	pointer := common.HexToAddress("0x217703")
	k.SetAddressMapping(ctx, from, fromEVM)
	k.SetAddressMapping(ctx, to, toEVM)
	tf := tfkeeper.NewMsgServerImpl(a.TokenFactoryKeeper)
	created, err := tf.CreateDenom(sdk.WrapSDKContext(ctx), tftypes.NewMsgCreateDenom(from.String(), "bank-proof"))
	require.NoError(t, err)
	denom := created.NewTokenDenom
	_, err = tf.Mint(sdk.WrapSDKContext(ctx), tftypes.NewMsgMint(from.String(), sdk.NewInt64Coin(denom, 25)))
	require.NoError(t, err)
	require.NoError(t, k.SetERC20NativePointer(ctx, denom, pointer))

	restart := func() {
		t.Helper()
		require.NoError(t, a.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
		_, commitErr := a.Commit(context.Background())
		require.NoError(t, commitErr)
		committed := a.LastCommitID()
		require.Positive(t, committed.Version)
		require.NotEmpty(t, committed.Hash)
		require.NoError(t, a.Close())
		a = nil
		a, ctx = proofBankOpenApp(t, dir, true)
		require.Equal(t, committed, a.LastCommitID(), "recover the committed application, without genesis or state reseeding")
		k = &a.EvmKeeper
		tf = tfkeeper.NewMsgServerImpl(a.TokenFactoryKeeper)
	}

	bankABI := bank.GetABI()
	pack := func(name string, args ...interface{}) []byte {
		t.Helper()
		data, packErr := bankABI.Pack(name, args...)
		require.NoError(t, packErr)
		return data
	}
	call := func(height int64, caller common.Address, input []byte, value *big.Int, readOnly, delegate bool) ([]byte, error) {
		t.Helper()
		p, newErr := bank.NewPrecompile(a.GetPrecompileKeepers())
		require.NoError(t, newErr)
		db := state.NewDBImpl(ctx.WithBlockHeight(height), k, false)
		defer db.Cleanup()
		out, _, runErr := p.RunAndCalculateGas(&vm.EVM{StateDB: db}, caller, caller, input, 1_000_000, value, nil, readOnly, delegate)
		if runErr == nil {
			_, finalizeErr := db.Finalize()
			require.NoError(t, finalizeErr)
		} else {
			require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
			require.Empty(t, db.GetAllLogs())
		}
		db.Cleanup()
		restart()
		return out, runErr
	}
	assertBalances := func(sender, receiver int64) {
		t.Helper()
		// Read only through the new application and keeper recovered from disk.
		db := state.NewDBImpl(ctx, k, false)
		defer db.Cleanup()
		require.Equal(t, sdk.NewInt(sender), a.BankKeeper.GetBalance(db.Ctx(), from, denom).Amount)
		require.Equal(t, sdk.NewInt(receiver), a.BankKeeper.GetBalance(db.Ctx(), to, denom).Amount)
		require.Equal(t, sdk.NewInt(25), a.BankKeeper.GetSupply(db.Ctx(), denom).Amount)
	}
	send := pack(bank.SendMethod, fromEVM, toEVM, denom, big.NewInt(5))
	var successes []string
	for i := int64(1); i <= 2; i++ {
		out, runErr := call(100, pointer, send, nil, false, false)
		require.NoError(t, runErr)
		decoded, decodeErr := bankABI.Unpack(bank.SendMethod, out)
		require.NoError(t, decodeErr)
		require.Equal(t, []interface{}{true}, decoded)
		successes = append(successes, "0x"+hex.EncodeToString(out))
		assertBalances(25-5*i, 5*i)
		// A new balance query observes the newly written value, not a cached result.
		balance, queryErr := call(100, pointer, pack(bank.BalanceMethod, toEVM, denom), nil, true, false)
		require.NoError(t, queryErr)
		values, decodeErr := bankABI.Unpack(bank.BalanceMethod, balance)
		require.NoError(t, decodeErr)
		require.Equal(t, big.NewInt(5*i), values[0])
	}

	// The real pause producer also refuses a wrong signer without changing state.
	_, err = tf.SetDenomPaused(sdk.WrapSDKContext(ctx), tftypes.NewMsgSetDenomPaused(to.String(), denom, true))
	require.ErrorIs(t, err, tftypes.ErrUnauthorized)
	require.False(t, a.TokenFactoryKeeper.IsDenomPaused(ctx, denom))
	_, err = tf.SetDenomPaused(sdk.WrapSDKContext(ctx), tftypes.NewMsgSetDenomPaused(from.String(), denom, true))
	require.NoError(t, err)
	restart()
	require.True(t, a.TokenFactoryKeeper.IsDenomPaused(ctx, denom), "recover the real producer's pause flag")
	_, mintErr := tf.Mint(sdk.WrapSDKContext(ctx), tftypes.NewMsgMint(from.String(), sdk.NewInt64Coin(denom, 1)))
	require.ErrorIs(t, mintErr, tftypes.ErrDenomPaused)
	require.EqualError(t, mintErr, tftypes.DenomPausedReason)
	require.Equal(t, ledgertypes.AssetPausedCode, ledgertypes.ErrAssetPaused.Error())
	require.Equal(t, ledgertypes.AssetPausedCode, pcommon.LXPCodeName(-403))

	var reasons []proofBankReason
	var malformed []string
	observe := func(data []byte, code, message string, detail *string, retryable bool) {
		t.Helper()
		text, decodeErr := abi.UnpackRevert(data)
		require.NoError(t, decodeErr)
		want := code + ": " + message
		if detail != nil {
			want += "; " + *detail
		}
		require.Equal(t, want, text)
		reasons = append(reasons, proofBankReason{"0x" + hex.EncodeToString(data), code, message, detail, retryable})
	}
	pauseDetail := denom + " transfers are currently disabled: send transactions are disabled"
	methodDetail := "sendNative"
	cases := []struct {
		name, code, message string
		input               []byte
		value               *big.Int
		readOnly, delegate  bool
		detail              *string
		retryable           bool
	}{
		{"paused", ledgertypes.AssetPausedCode, "asset paused", send, nil, false, false, &pauseDetail, true},
		{"invalid_address", "LXP_PRECOMPILE_INVALID_ADDR", "invalid addr", pack(bank.BalanceMethod, common.Address{}, denom), nil, false, false, nil, false},
		{"delegatecall", "LXP_BANK_DELEGATECALL", "cannot delegatecall <method>", pack(bank.SendNativeMethod, to.String()), big.NewInt(1), false, true, &methodDetail, false},
		{"staticcall", "LXP_BANK_STATICCALL", "cannot call <method> from staticcall", pack(bank.SendNativeMethod, to.String()), big.NewInt(1), true, false, &methodDetail, false},
		{"zero_value", "LXP_BANK_ZERO_VALUE", "set `value` field to non-zero to send", pack(bank.SendNativeMethod, to.String()), nil, false, false, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, runErr := call(99, pointer, tc.input, tc.value, tc.readOnly, tc.delegate)
			require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
			require.Nil(t, out, "before merge, preserve legacy empty revert")
			for i := 0; i < 2; i++ {
				out, runErr = call(100+int64(i), pointer, tc.input, tc.value, tc.readOnly, tc.delegate)
				require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
				observe(out, tc.code, tc.message, tc.detail, tc.retryable)
				assertBalances(15, 10)
			}
			// Corrupt actual producer bytes at ABI framing and UTF-8 boundaries.
			malformed = append(malformed, "0x"+hex.EncodeToString(out[:len(out)-1]))
			badOffset := bytes.Clone(out)
			badOffset[35] = 64
			malformed = append(malformed, "0x"+hex.EncodeToString(badOffset))
			badUTF8 := bytes.Clone(out)
			badUTF8[68] = 0xff
			malformed = append(malformed, "0x"+hex.EncodeToString(badUTF8))
		})
	}
	// Both registered module surfaces travel through the real common encoder.
	observe(pcommon.LXPReason(mintErr), ledgertypes.AssetPausedCode, "denom is paused", nil, true)
	observe(pcommon.LXPRevert(ledgertypes.ErrAssetPaused.Error(), ""), ledgertypes.AssetPausedCode, "asset paused", nil, true)
	for _, text := range []string{
		"LXP__BANK_ZERO_VALUE: set `value` field to non-zero to send",
		"LXP_BANK_ZERO_VALUE: set `value` field to non-zero to send; ",
	} {
		// Re-encode corrupted reason text with the production transport encoder.
		malformed = append(malformed, "0x"+hex.EncodeToString(pcommon.RevertReason(errors.New(text))))
	}
	malformed = append(malformed, "0x"+hex.EncodeToString(pcommon.LXPRevert(ledgertypes.AssetPausedCode, strings.Repeat("x", 64*1024))))

	_, err = tf.SetDenomPaused(sdk.WrapSDKContext(ctx), tftypes.NewMsgSetDenomPaused(from.String(), denom, false))
	require.NoError(t, err)
	restart()
	require.False(t, a.TokenFactoryKeeper.IsDenomPaused(ctx, denom), "recover the real producer's unpause")
	tooMuch := pack(bank.SendMethod, fromEVM, toEVM, denom, big.NewInt(26))
	out, runErr := call(199, pointer, tooMuch, nil, false, false)
	require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
	legacyReason, decodeErr := abi.UnpackRevert(out)
	require.NoError(t, decodeErr)
	require.Contains(t, legacyReason, "insufficient funds")
	malformed = append(malformed, "0x"+hex.EncodeToString(out)) // not an LXP reason
	for i := int64(0); i < 2; i++ {
		out, runErr = call(200+i, pointer, tooMuch, nil, false, false)
		require.NoError(t, runErr)
		values, unpackErr := bankABI.Unpack(bank.SendMethod, out)
		require.NoError(t, unpackErr)
		require.Equal(t, []interface{}{false}, values)
		assertBalances(15, 10)
	}

	return proofBankTranscript{reasons, malformed, successes}
}

func TestProof0x1001Bank(t *testing.T) {
	input, err := json.Marshal(proofBankDurablePath(t))
	require.NoError(t, err)
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	python := exec.Command("python3", "-B", "-c", proofBankPythonConsumer)
	python.Dir = root
	python.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "services/agent/sdk/python"))
	node := exec.Command("node", "--disable-warning=ExperimentalWarning", "--experimental-transform-types", "--input-type=module", "-e", proofBankTypeScriptConsumer)
	node.Dir = root
	for _, consumer := range []*exec.Cmd{python, node} {
		consumer.Stdin = bytes.NewReader(input)
		output, consumerErr := consumer.CombinedOutput()
		require.NoError(t, consumerErr, "%s: %s", consumer.Path, output)
		require.Equal(t, "bank consumers passed\n", string(output))
	}
}

const proofBankPythonConsumer = `
import json, sys
from layerx_sdk.revert import decode_revert_reason, raise_for_receipt_status, RevertReasonError
from layerx_sdk.production import PlatformSdkError, SdkErrorCode
data = json.load(sys.stdin)
for row in data["reasons"]:
    decoded = decode_revert_reason(bytes.fromhex(row["data"][2:]))
    assert isinstance(decoded, RevertReasonError)
    assert (decoded.code, decoded.message, decoded.detail, decoded.retryable) == (row["code"], row["message"], row["detail"], row["retryable"])
    try:
        raise_for_receipt_status({"status": "0x0", "returnData": row["data"]})
    except RevertReasonError as error:
        assert (error.code, error.message, error.detail, error.retryable) == (decoded.code, decoded.message, decoded.detail, decoded.retryable)
    else:
        raise AssertionError("failed receipt accepted")
for raw in data["malformed"]:
    for decode in (lambda: decode_revert_reason(bytes.fromhex(raw[2:])), lambda: raise_for_receipt_status({"status": "0x0", "returnData": raw})):
        try:
            decode()
        except PlatformSdkError as error:
            assert error.code == SdkErrorCode.DECODE_FAILURE and error.retry == "never"
        else:
            raise AssertionError("malformed revert accepted")
for raw in data["successes"]:
    assert raise_for_receipt_status({"status": "0x1", "returnData": raw}) is None
print("bank consumers passed")
`

const proofBankTypeScriptConsumer = `
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';
// Node executes the actual TypeScript sources. Resolve their emitted .js import
// spelling to source .ts; no consumer or transport implementation is replaced.
const sdk = pathToFileURL(process.cwd() + '/services/agent/sdk/typescript/src/').href;
registerHooks({ resolve(specifier, context, nextResolve) {
  if (context.parentURL?.startsWith(sdk) && specifier.endsWith('.js')) {
    return nextResolve(specifier.slice(0, -3) + '.ts', context);
  }
  return nextResolve(specifier, context);
}});
const { decodeRevertReason, RevertReasonError } = await import(sdk + 'revert.ts');
const { PlatformSdkError } = await import(sdk + 'production.ts');
const data = JSON.parse(readFileSync(0, 'utf8'));
for (const row of data.reasons) {
  for (const raw of [row.data, Uint8Array.from(Buffer.from(row.data.slice(2), 'hex'))]) {
    const decoded = decodeRevertReason(raw);
    assert.ok(decoded instanceof RevertReasonError);
    assert.deepEqual([decoded.code, decoded.message, decoded.detail ?? null, decoded.retryable], [row.code, row.message, row.detail, row.retryable]);
    assert.equal(decoded.retry, row.retryable ? 'safe' : 'never');
  }
}
for (const raw of [...data.malformed, null, undefined, 42, {}, '0xzz']) {
  assert.throws(() => decodeRevertReason(raw), (error) => error instanceof PlatformSdkError && error.code === 'decode-failure' && error.retry === 'never');
}
printResult();
function printResult() { process.stdout.write('bank consumers passed\n'); }
`
