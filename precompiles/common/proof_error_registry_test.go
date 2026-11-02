// Proves: common.LXPCodeName/LXPRevert -> RevertReason Error(string)/receipt.returnData -> layerx_sdk.revert.decode_revert_reason/raise_for_receipt_status
//
// Repairs lxp_errors.go's LXP_AGENT_NOT_ACTIVE message: its embedded "; "
// delimiter was decoded as detail instead of the canonical agent message.
// Repairs revert.py's raise_for_receipt_status: a non-mapping envelope must
// raise PlatformSdkError(DECODE_FAILURE), rather than leaking AttributeError.
// The registry is stateless; fresh production and decoding on each delivery,
// with changing details, prove repeated observation without a cached result.
//
// Outside scope, precompiles/common/precompiles.go:LXPReason preserves malformed
// LXP_-prefixed text verbatim; the expected refusal is at the Python decoder.
// precompiles/layerxagent/common/common.go:Precompile.Run likewise preserves
// HeightGate's contextual message, rather than substituting the registry's
// canonical message; the call-site proof asserts that current wire behaviour.
package common_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/eip712"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type proofRegistryReason struct {
	Code      string  `json:"code"`
	Message   string  `json:"message"`
	Detail    *string `json:"detail"`
	Retryable bool    `json:"retryable"`
}

type proofRegistryDelivery struct {
	Name        string               `json:"name"`
	Data        string               `json:"data"`
	ReceiptMode bool                 `json:"receipt_mode"`
	Receipt     interface{}          `json:"receipt"`
	Want        *proofRegistryReason `json:"want"`
}

// Both entry points receive the very bytes returned by the producer. JSON hex
// is only the subprocess carrier; the Python SDK owns all ABI/text decoding.
func proofRegistryDeliver(deliveries *[]proofRegistryDelivery, name string, data []byte, want *proofRegistryReason) {
	wire := hex.EncodeToString(data)
	*deliveries = append(*deliveries,
		proofRegistryDelivery{Name: name + "/decode", Data: wire, Want: want},
		proofRegistryDelivery{Name: name + "/receipt", ReceiptMode: true,
			Receipt: map[string]interface{}{"status": "0x0", "returnData": "0x" + wire}, Want: want},
	)
}

func proofRegistryConsume(t *testing.T, deliveries []proofRegistryDelivery) {
	t.Helper()
	require.NotEmpty(t, deliveries)
	input, err := json.Marshal(deliveries)
	require.NoError(t, err)
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-B", "-c", `
import json
import sys
import unittest

from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from layerx_sdk.revert import (
    LXP_RETRYABLE, RevertReasonError, decode_revert_reason, raise_for_receipt_status,
)

check = unittest.TestCase()
deliveries = json.load(sys.stdin)
previous = None
for delivery in deliveries:
    with check.subTest(delivery=delivery["name"]):
        want = delivery["want"]
        if delivery["receipt_mode"]:
            if want is None:
                with check.assertRaises(PlatformSdkError) as raised:
                    raise_for_receipt_status(delivery["receipt"])
                check.assertIs(raised.exception.code, SdkErrorCode.DECODE_FAILURE)
                check.assertEqual(raised.exception.retry, "never")
                continue
            with check.assertRaises(RevertReasonError) as raised:
                raise_for_receipt_status(delivery["receipt"])
            result = raised.exception
        else:
            data = bytes.fromhex(delivery["data"])
            if want is None:
                with check.assertRaises(PlatformSdkError) as raised:
                    decode_revert_reason(data)
                check.assertIs(raised.exception.code, SdkErrorCode.DECODE_FAILURE)
                check.assertEqual(raised.exception.retry, "never")
                continue
            result = decode_revert_reason(data)
        check.assertIs(type(result), RevertReasonError)
        check.assertEqual(
            (result.code, result.message, result.detail, result.retryable),
            (want["code"], want["message"], want["detail"], want["retryable"]),
        )
        check.assertIs(result.retryable, LXP_RETRYABLE[want["code"]])
        text = want["code"] + ": " + want["message"]
        if want["detail"] is not None:
            text += "; " + want["detail"]
        check.assertEqual(str(result), text)
        check.assertIsNot(result, previous)
        previous = result

check.assertIsNone(raise_for_receipt_status({"status": "0x1", "returnData": "0x"}))
print(json.dumps({"observed": len(deliveries)}))
`)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "services", "agent", "sdk", "python"))
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "real Python consumer: %s", output)
	var observed struct {
		Count int `json:"observed"`
	}
	require.NoError(t, json.Unmarshal(output, &observed), "%s", output)
	require.Equal(t, len(deliveries), observed.Count)
	t.Logf("Python consumer asserted %d deliveries", observed.Count)
}

func TestProofErrorRegistry(t *testing.T) {
	rows := []struct {
		code    string
		message string
		kernel  int32
	}{
		{"LXP_ARBITER_REQUIRED", "arbiter required", -737},
		{"LXP_UNAUTHORIZED_ARBITER", "unauthorized arbiter", -738},
		{"LXP_ESCROW_LINK_MISMATCH", "escrow link mismatch", -739},
		{"LXP_AGENT_NOT_ACTIVE", "agent function not active", 0},
		{"LXP_BANK_DELEGATECALL", "cannot delegatecall <method>", 0},
		{"LXP_BANK_STATICCALL", "cannot call <method> from staticcall", 0},
		{"LXP_BANK_ZERO_VALUE", "set `value` field to non-zero to send", 0},
		{"LXP_PRICEFEED_BATCH_TOO_LARGE", "getPrices takes at most 64 market ids", 0},
		{"LXP_PRICEFEED_DELEGATECALL", "cannot delegatecall pricefeed", 0},
		{"LXP_PRICEFEED_STATICCALL", "cannot call pricefeed <method> from staticcall", 0},
		{"LXP_PRICEFEED_KEEPER_MISSING", "pricefeed: the pricefeed keeper is not wired into this application", 0},
	}
	var deliveries []proofRegistryDelivery
	for _, row := range rows {
		code := row.code
		if row.kernel != 0 {
			code = common.LXPCodeName(row.kernel)
			require.Equal(t, row.code, code)
		}
		for round := 0; round < 2; round++ {
			name := fmt.Sprintf("%s/%d", code, round)
			proofRegistryDeliver(&deliveries, name, common.LXPRevert(code, ""),
				&proofRegistryReason{Code: row.code, Message: row.message})
			detail := fmt.Sprintf("delivery %d; market α", round)
			proofRegistryDeliver(&deliveries, name+"/detail", common.LXPRevert(code, detail),
				&proofRegistryReason{Code: row.code, Message: row.message, Detail: &detail})
		}
	}
	// Positive retryability and unknown identifiers exercise the consumer's
	// code-based classification as well as the terminal cases above.
	for round := 0; round < 2; round++ {
		proofRegistryDeliver(&deliveries, "retryable", common.LXPRevert("LXP_BANK_INSUFFICIENT_FUNDS", ""),
			&proofRegistryReason{Code: "LXP_BANK_INSUFFICIENT_FUNDS", Message: "insufficient funds", Retryable: true})
		detail := fmt.Sprintf("LXP_NOT_REGISTERED; delivery %d", round)
		proofRegistryDeliver(&deliveries, "unknown", common.LXPRevert("LXP_NOT_REGISTERED", fmt.Sprintf("delivery %d", round)),
			&proofRegistryReason{Code: "LXP_UNKNOWN", Message: "unknown", Detail: &detail})
	}
	require.Equal(t, "LXP_OK", common.LXPCodeName(0))
	require.Equal(t, "LXP_UNKNOWN", common.LXPCodeName(-999))
	require.Empty(t, common.LXPCodeName(-1001), "fatal kernel errors never become reverts")

	wire := common.LXPRevert("LXP_ARBITER_REQUIRED", "")
	proofRegistryDeliver(&deliveries, "truncated", wire[:len(wire)-1], nil)
	proofRegistryDeliver(&deliveries, "missing selector", wire[4:], nil)
	proofRegistryDeliver(&deliveries, "trailing word", append(bytes.Clone(wire), make([]byte, 32)...), nil)
	badOffset := bytes.Clone(wire)
	badOffset[35] = 64
	proofRegistryDeliver(&deliveries, "noncanonical offset", badOffset, nil)
	badLength := bytes.Clone(wire)
	badLength[36] = 0xff
	proofRegistryDeliver(&deliveries, "oversized length", badLength, nil)
	badPadding := bytes.Clone(wire)
	badPadding[len(badPadding)-1] = 1
	proofRegistryDeliver(&deliveries, "nonzero padding", badPadding, nil)
	proofRegistryDeliver(&deliveries, "invalid UTF-8", common.LXPRevert("LXP_ARBITER_REQUIRED", string([]byte{0xff})), nil)
	proofRegistryDeliver(&deliveries, "malformed producer text", common.LXPReason(errors.New("LXP_ARBITER_REQUIRED")), nil)
	for _, receipt := range []interface{}{
		nil, "0x0", []interface{}{}, 0,
		map[string]interface{}{},
		map[string]interface{}{"status": "0x2", "returnData": "0x" + hex.EncodeToString(wire)},
		map[string]interface{}{"status": "0x0", "returnData": "0x0"},
		map[string]interface{}{"status": "0x0", "returnData": "0xgg"},
	} {
		deliveries = append(deliveries, proofRegistryDelivery{Name: "malformed receipt", ReceiptMode: true, Receipt: receipt})
	}
	proofRegistryConsume(t, deliveries)
}

// call enters the real bank dispatch and its merge-gated registry conversion.
func call(t *testing.T, ctx sdk.Context, p *common.DynamicGasPrecompile, value *big.Int, readOnly, delegated bool) []byte {
	t.Helper()
	caller := ethcommon.HexToAddress("0x215001")
	receiver := sdk.AccAddress(ethcommon.HexToAddress("0x215002").Bytes()).String()
	input, err := p.ABI.Pack(bank.SendNativeMethod, receiver)
	require.NoError(t, err)
	db := state.NewDBImpl(ctx.WithGasMeter(sdk.NewGasMeter(1_000_000, 1, 1)), &testkeeper.EVMTestApp.EvmKeeper, false)
	data, _, err := p.RunAndCalculateGas(&vm.EVM{StateDB: db}, caller, caller, input, 1_000_000, value, nil, readOnly, delegated)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.NotEmpty(t, data)
	require.Empty(t, db.GetAllLogs())
	return data
}

func delegatecall(t *testing.T, ctx sdk.Context, p *common.DynamicGasPrecompile) []byte {
	t.Helper()
	return call(t, ctx, p, big.NewInt(1), false, true)
}

func TestProofErrorRegistryCallSites(t *testing.T) {
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	const height = int64(215001)
	app.EvmKeeper.UpgradeKeeper().SetDone(ctx.WithBlockHeight(height), common.MergeUpgrade)
	bankPrecompile, err := bank.NewPrecompile(app.GetPrecompileKeepers())
	require.NoError(t, err)
	feed, err := pricefeed.NewPrecompile(app.GetPrecompileKeepers())
	require.NoError(t, err)
	agent, err := eip712.NewPrecompile(app.EvmKeeper.UpgradeKeeper(), common.MergeUpgrade, 64, 64)
	require.NoError(t, err)
	ids := make([]*big.Int, 65)
	for i := range ids {
		ids[i] = big.NewInt(int64(i + 1))
	}
	feedInput, err := feed.Pack(pricefeed.GetPricesMethod, ids)
	require.NoError(t, err)
	agentInput, err := agent.Pack(eip712.HashTypedDataMethod, [32]byte{}, [32]byte{})
	require.NoError(t, err)
	var deliveries []proofRegistryDelivery
	for round := 0; round < 2; round++ {
		activeCtx := ctx.WithBlockHeight(height + int64(round))
		method := bank.SendNativeMethod
		proofRegistryDeliver(&deliveries, "bank delegatecall", delegatecall(t, activeCtx, bankPrecompile),
			&proofRegistryReason{Code: "LXP_BANK_DELEGATECALL", Message: "cannot delegatecall <method>", Detail: &method})
		proofRegistryDeliver(&deliveries, "bank staticcall", call(t, activeCtx, bankPrecompile, big.NewInt(1), true, false),
			&proofRegistryReason{Code: "LXP_BANK_STATICCALL", Message: "cannot call <method> from staticcall", Detail: &method})
		proofRegistryDeliver(&deliveries, "bank zero value", call(t, activeCtx, bankPrecompile, big.NewInt(0), false, false),
			&proofRegistryReason{Code: "LXP_BANK_ZERO_VALUE", Message: "set `value` field to non-zero to send"})
		db := state.NewDBImpl(activeCtx, &app.EvmKeeper, false)
		data, runErr := feed.Run(&vm.EVM{StateDB: db}, ethcommon.Address{}, feed.Address(), feedInput, nil, true, false, nil)
		require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
		proofRegistryDeliver(&deliveries, "pricefeed batch", data,
			&proofRegistryReason{Code: "LXP_PRICEFEED_BATCH_TOO_LARGE", Message: "getPrices takes at most 64 market ids"})
		inactiveCtx := ctx.WithBlockHeight(height - 1 - int64(round))
		db = state.NewDBImpl(inactiveCtx, &app.EvmKeeper, false)
		data, runErr = agent.Run(&vm.EVM{StateDB: db}, ethcommon.Address{}, agent.Address(), agentInput, nil, true, false, nil)
		require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
		proofRegistryDeliver(&deliveries, "inactive agent", data,
			&proofRegistryReason{Code: "LXP_AGENT_NOT_ACTIVE", Message: fmt.Sprintf("%s not active at height %d", common.MergeUpgrade, inactiveCtx.BlockHeight())})
	}
	proofRegistryConsume(t, deliveries)
}
