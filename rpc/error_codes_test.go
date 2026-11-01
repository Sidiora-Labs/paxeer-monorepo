package evmrpc

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type errorCodeRegistryWireError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (e *errorCodeRegistryWireError) Error() string  { return e.Message }
func (e *errorCodeRegistryWireError) ErrorCode() int { return e.Code }

func TestErrorCodeRegistry(t *testing.T) {
	assertCode := func(t *testing.T, err error, want int) {
		t.Helper()
		require.Error(t, err)
		var coded rpc.Error
		require.ErrorAs(t, err, &coded)
		require.Equal(t, want, coded.ErrorCode())
	}
	decodeError := func(t *testing.T, response *httptest.ResponseRecorder) error {
		t.Helper()
		var message struct {
			JSONRPC string                      `json:"jsonrpc"`
			Error   *errorCodeRegistryWireError `json:"error"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &message))
		require.Equal(t, "2.0", message.JSONRPC)
		require.NotNil(t, message.Error)
		return message.Error
	}

	t.Run("unsupported methods", func(t *testing.T) {
		require.Equal(t, -32000, ErrCodeEVMNotSupported)
		server := rpc.NewServer()
		t.Cleanup(server.Stop)
		require.NoError(t, server.RegisterName("eth", &InfoAPI{connectionType: ConnectionTypeHTTP}))
		client := rpc.DialInProc(server)
		t.Cleanup(client.Close)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		for _, method := range []string{"eth_blobBaseFee", "eth_accounts"} {
			var result json.RawMessage
			err := client.CallContext(ctx, &result, method)
			assertCode(t, err, -32000)
			require.Nil(t, result)
		}
	})

	t.Run("disabled pax methods", func(t *testing.T) {
		require.Equal(t, -32601, paxLegacyNotEnabled)
		server := rpc.NewServer()
		t.Cleanup(server.Stop)
		handler := wrapPaxLegacyHTTP(server, BuildPaxLegacyEnabledSet(nil))
		for _, method := range []string{"pax_getBlockByNumber", "pax2_getBlockByNumber"} {
			assertCode(t, paxLegacyGateError(method, BuildPaxLegacyEnabledSet(nil)), -32601)
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
				`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":["latest",false]}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code)
			err := decodeError(t, response)
			assertCode(t, err, -32601)
			require.Contains(t, err.Error(), "deprecated")
		}
	})

	t.Run("sequencer receipt not served", func(t *testing.T) {
		require.Equal(t, -32000, sequencerNotServedCode)
		api, err := newSequencerReceiptAPI(nil, nil, SequencerReceiptOption{}, ConnectionTypeHTTP)
		require.NoError(t, err)
		result, err := api.SequencerReceipt(context.Background(), common.HexToHash("0x702001"))
		require.Nil(t, result)
		assertCode(t, err, -32000)
		require.Equal(t, "pax_sequencerReceipt served by sequencer", err.Error())
	})

	t.Run("execution reverted", func(t *testing.T) {
		data, _, executionErr := runtime.Execute(common.FromHex("0x602a60005260206000fd"), nil, &runtime.Config{GasLimit: 100_000})
		require.ErrorIs(t, executionErr, vm.ErrExecutionReverted)
		require.Equal(t, common.LeftPadBytes([]byte{42}, 32), data)
		var err error = NewRevertError(&core.ExecutionResult{Err: executionErr, ReturnData: data})
		var reverted *RevertError
		require.ErrorAs(t, err, &reverted)
		assertCode(t, err, 3)
		require.Equal(t, hexutil.Encode(data), reverted.ErrorData())
	})

	t.Run("rate limit exceeded", func(t *testing.T) {
		server := rpc.NewServer()
		t.Cleanup(server.Stop)
		require.NoError(t, server.RegisterName("web3", &Web3API{}))
		limiter := newIPRateLimiter(0.001, 1)
		t.Cleanup(limiter.stop)
		handler := limiter.wrap(server)
		for i := 0; i < 2; i++ {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
				`{"jsonrpc":"2.0","id":1,"method":"web3_sha3","params":["0x"]}`))
			request.RemoteAddr = "192.0.2.1:12345"
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if i == 0 {
				require.Equal(t, http.StatusOK, response.Code)
				var message struct {
					Result string          `json:"result"`
					Error  json.RawMessage `json:"error"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &message))
				require.Empty(t, message.Error)
				require.Equal(t, crypto.Keccak256Hash(nil).Hex(), message.Result)
				continue
			}
			require.Equal(t, http.StatusTooManyRequests, response.Code)
			err := decodeError(t, response)
			assertCode(t, err, -32005)
			require.Equal(t, "rate limit exceeded", err.Error())
		}
	})

	t.Run("sync send timeout", func(t *testing.T) {
		client, err := rpc.DialHTTP("http://127.0.0.1:7777")
		require.NoError(t, err)
		t.Cleanup(client.Close)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var chainID hexutil.Big
		require.NoError(t, client.CallContext(ctx, &chainID, "eth_chainId"))
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		to := common.HexToAddress("0x702001")
		tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID.ToInt()), &ethtypes.DynamicFeeTx{
			ChainID: chainID.ToInt(), Nonce: 702001, Gas: 21000, To: &to,
			GasFeeCap: big.NewInt(1_000_000_000), GasTipCap: big.NewInt(1), Value: big.NewInt(1),
		})
		require.NoError(t, err)
		raw, err := tx.MarshalBinary()
		require.NoError(t, err)
		var receipt map[string]interface{}
		require.NoError(t, client.CallContext(ctx, &receipt, "eth_getTransactionReceipt", tx.Hash()))
		require.Nil(t, receipt)
		start := time.Now()
		err = client.CallContext(ctx, &receipt, "eth_sendRawTransactionSync", hexutil.Bytes(raw), hexutil.Uint64(200))
		elapsed := time.Since(start)
		assertCode(t, err, 4)
		require.Equal(t, "transaction not yet included", err.Error())
		var dataErr rpc.DataError
		require.True(t, errors.As(err, &dataErr))
		require.Equal(t, tx.Hash().Hex(), dataErr.ErrorData())
		require.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
		require.Less(t, elapsed, time.Second)
		require.Nil(t, receipt)
	})
}
