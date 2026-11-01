package evmrpc_test

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

const debugTracerShapeGasUsed = params.TxGas + 3*params.TxTokenPerNonZeroByte*params.TxCostFloorPerToken

type debugTracerShapeCallFrame struct {
	Type         string                      `json:"type"`
	From         common.Address              `json:"from"`
	To           *common.Address             `json:"to,omitempty"`
	Gas          hexutil.Uint64              `json:"gas"`
	GasUsed      hexutil.Uint64              `json:"gasUsed"`
	Input        hexutil.Bytes               `json:"input"`
	Output       hexutil.Bytes               `json:"output,omitempty"`
	Value        *hexutil.Big                `json:"value,omitempty"`
	Error        string                      `json:"error,omitempty"`
	RevertReason string                      `json:"revertReason,omitempty"`
	Calls        []debugTracerShapeCallFrame `json:"calls,omitempty"`
}

type debugTracerShapeAccount struct {
	Balance *hexutil.Big                `json:"balance,omitempty"`
	Nonce   uint64                      `json:"nonce,omitempty"`
	Code    hexutil.Bytes               `json:"code,omitempty"`
	Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
}

type debugTracerShapeFixture struct {
	tx     *types.Transaction
	from   common.Address
	code   []byte
	output []byte
}

func newDebugTracerShapeFixture(t *testing.T) debugTracerShapeFixture {
	t.Helper()
	previousCtx, previousTx := Ctx, Tx1
	Ctx, _ = Ctx.CacheContext()
	t.Cleanup(func() {
		Ctx, Tx1 = previousCtx, previousTx
	})

	to := common.HexToAddress("0x010203")
	builder, tx := buildTx(types.DynamicFeeTx{
		ChainID: EVMKeeper.ChainID(Ctx), Nonce: 0, Gas: 100000,
		GasFeeCap: big.NewInt(1000000000), To: &to,
		Value: big.NewInt(1000), Data: []byte("abc"),
	})
	Tx1 = builder.GetTx()
	msg, ok := Tx1.GetMsgs()[0].(*evmtypes.MsgEVMTransaction)
	require.True(t, ok)
	require.NotNil(t, tx)
	require.NotNil(t, tx.To())
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	require.NoError(t, err)
	require.Equal(t, uint64(0), tx.Nonce())
	require.Equal(t, tx.Nonce(), EVMKeeper.GetNonce(Ctx, from))

	code := common.FromHex("0x602a60005260206000f3")
	EVMKeeper.SetCode(Ctx, *tx.To(), code)
	EVMKeeper.SetNonce(Ctx, *tx.To(), 1)
	err = EVMKeeper.BankKeeper().AddCoins(Ctx, EVMKeeper.GetPaxAddressOrDefault(Ctx, *tx.To()), sdk.NewCoins(sdk.NewInt64Coin(EVMKeeper.GetBaseDenom(Ctx), 1)), false)
	require.NoError(t, err)

	output := make([]byte, 32)
	output[31] = 42
	executionCtx, _ := Ctx.CacheContext()
	executionCtx = executionCtx.WithBlockHeight(MockHeight8).WithTxIndex(0).WithBlockTime(mockBlockHeader(MockHeight8).Time)
	require.NoError(t, ante.Preprocess(executionCtx, msg, tx.ChainId(), false))
	executed, err := evmkeeper.NewMsgServerImpl(EVMKeeper).EVMTransaction(sdk.WrapSDKContext(executionCtx), msg)
	require.NoError(t, err)
	require.Empty(t, executed.VmError)
	require.Equal(t, output, executed.ReturnData)
	require.Equal(t, debugTracerShapeGasUsed, executed.GasUsed)
	receipt, err := EVMKeeper.GetTransientReceipt(executionCtx, tx.Hash(), 0)
	require.NoError(t, err)
	require.Equal(t, uint32(types.ReceiptStatusSuccessful), receipt.Status)
	require.NoError(t, EVMKeeper.ReceiptStore().SetReceipts(executionCtx, []receiptstore.ReceiptRecord{{TxHash: tx.Hash(), Receipt: receipt}}))
	require.Eventually(t, func() bool {
		stored, err := EVMKeeper.GetReceipt(Ctx, tx.Hash())
		return err == nil && stored.BlockNumber == MockHeight8 && stored.GasUsed == executed.GasUsed
	}, 2*time.Second, 10*time.Millisecond)

	committed := sendRequestGood(t, "getTransactionByHash", tx.Hash().Hex())
	require.NotContains(t, committed, "error")
	transaction, ok := committed["result"].(map[string]interface{})
	require.True(t, ok, "expected a committed transaction: %v", committed)
	require.Equal(t, tx.Hash().Hex(), transaction["hash"])
	require.Equal(t, "0x8", transaction["blockNumber"])
	require.Equal(t, "0x0", transaction["transactionIndex"])
	require.Equal(t, common.BytesToHash(MockBlockID.Hash).Hex(), transaction["blockHash"])

	return debugTracerShapeFixture{tx: tx, from: from, code: code, output: output}
}

func (f debugTracerShapeFixture) TraceTransaction(t *testing.T, tracer string) json.RawMessage {
	t.Helper()
	response := sendRequestGoodWithNamespace(t, "debug", "traceTransaction", f.tx.Hash().Hex(), map[string]interface{}{"tracer": tracer})
	require.NotContains(t, response, "error", "%s failed: %v", tracer, response)
	result, ok := response["result"]
	require.True(t, ok, "%s response must contain a result", tracer)
	require.NotNil(t, result)
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	return raw
}

func TestDebugTraceTransactionCallTracerShape(t *testing.T) {
	fixture := newDebugTracerShapeFixture(t)
	raw := fixture.TraceTransaction(t, "callTracer")
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	for _, name := range []string{"type", "from", "to", "gas", "gasUsed", "input", "output"} {
		require.Contains(t, fields, name)
		require.NotEqual(t, "null", string(fields[name]), "%s must not be null", name)
	}
	require.NotContains(t, fields, "error")
	require.NotContains(t, fields, "revertReason")

	var frame debugTracerShapeCallFrame
	require.NoError(t, json.Unmarshal(raw, &frame))
	require.Equal(t, "CALL", frame.Type)
	require.Equal(t, fixture.from, frame.From)
	require.NotNil(t, frame.To)
	require.Equal(t, *fixture.tx.To(), *frame.To)
	require.Equal(t, fixture.tx.Gas(), uint64(frame.Gas))
	require.Equal(t, debugTracerShapeGasUsed, uint64(frame.GasUsed))
	require.LessOrEqual(t, frame.GasUsed, frame.Gas)
	require.Equal(t, fixture.tx.Data(), []byte(frame.Input))
	require.Equal(t, fixture.output, []byte(frame.Output))
	require.NotNil(t, frame.Value)
	require.Equal(t, fixture.tx.Value(), frame.Value.ToInt())
	require.Empty(t, frame.Error)
	require.Empty(t, frame.RevertReason)
	require.Empty(t, frame.Calls)
}

func TestDebugTraceTransactionPrestateTracerShape(t *testing.T) {
	fixture := newDebugTracerShapeFixture(t)
	raw := fixture.TraceTransaction(t, "prestateTracer")
	var fields map[string]map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.NotEmpty(t, fields)
	require.NotContains(t, fields, "pre")
	require.NotContains(t, fields, "post")
	var prestate map[common.Address]debugTracerShapeAccount
	require.NoError(t, json.Unmarshal(raw, &prestate))
	require.Contains(t, prestate, fixture.from)
	require.Contains(t, prestate, *fixture.tx.To())

	for address, accountFields := range fields {
		require.True(t, common.IsHexAddress(address), "invalid prestate address %q", address)
		addr := common.HexToAddress(address)
		account := prestate[addr]
		require.Contains(t, accountFields, "balance", "account %s", address)
		require.NotNil(t, account.Balance, "account %s", address)
		wantBalance := EVMKeeper.GetBalance(Ctx, EVMKeeper.GetPaxAddressOrDefault(Ctx, addr))
		require.Zero(t, wantBalance.Cmp(account.Balance.ToInt()), "prestate balance for %s", address)
		wantNonce := EVMKeeper.GetNonce(Ctx, addr)
		require.Equal(t, wantNonce, account.Nonce, "prestate nonce for %s", address)
		if wantNonce == 0 {
			require.NotContains(t, accountFields, "nonce", "geth omits zero nonces")
		} else {
			require.Contains(t, accountFields, "nonce")
		}
		wantCode := EVMKeeper.GetCode(Ctx, addr)
		require.Equal(t, wantCode, []byte(account.Code), "prestate code for %s", address)
		if len(wantCode) == 0 {
			require.NotContains(t, accountFields, "code", "geth omits empty code")
		} else {
			require.Contains(t, accountFields, "code")
		}
	}
	contract := prestate[*fixture.tx.To()]
	require.Equal(t, uint64(1), contract.Nonce)
	require.Equal(t, fixture.code, []byte(contract.Code))
}
