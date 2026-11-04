package cli

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	coretypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw1155"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw20"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw721"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/native"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const repairM1240Address = "0x1111111111111111111111111111111111111111"

func repairM1240Execute(cmd *cobra.Command, args []string, ctx context.Context, clientCtx client.Context) (string, error) {
	var output bytes.Buffer
	clientCtx = clientCtx.WithOutput(&output)
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if cmd.Name() == "payload" || strings.HasSuffix(cmd.Name(), "-payload") {
		args = append([]string{"--"}, args...)
	}
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(ctx)
	return output.String(), err
}

func repairM1240KeyContext(t *testing.T) (client.Context, *ecdsa.PrivateKey) {
	t.Helper()
	keys := keyring.NewInMemory()
	info, _, err := keys.NewMnemonic("repair-m1240", keyring.English, "", "", hd.Secp256k1)
	require.NoError(t, err)
	address := info.GetAddress()
	// Key reload uses the same persisted LocalInfo representation as the CLI.
	info, err = keys.Key("repair-m1240")
	require.NoError(t, err)
	local, ok := info.(keyring.LocalInfo)
	require.True(t, ok)
	private, err := legacy.PrivKeyFromBytes([]byte(local.PrivKeyArmor))
	require.NoError(t, err)
	key, err := crypto.HexToECDSA(hex.EncodeToString(private.Bytes()))
	require.NoError(t, err)
	return client.Context{}.WithKeyring(keys).WithFrom("repair-m1240").
		WithFromName("repair-m1240").WithFromAddress(address).WithChainID("pax-test"), key
}

func TestRepairM_1240_IntegerBounds(t *testing.T) {
	for _, bits := range []int{8, 16, 24, 32, 64, 72, 128, 256} {
		for _, signed := range []bool{true, false} {
			kind := "uint"
			exponent := bits
			if signed {
				kind = "int"
				exponent--
			}
			typ := fmt.Sprintf("%s%d", kind, bits)
			t.Run(typ, func(t *testing.T) {
				text := fmt.Sprintf(`[{"type":"function","name":"set","inputs":[{"name":"value","type":%q}],"outputs":[]}]`, typ)
				contract, err := abi.JSON(strings.NewReader(text))
				require.NoError(t, err)
				path := filepath.Join(t.TempDir(), "abi.json")
				require.NoError(t, os.WriteFile(path, []byte(text), 0600))
				limit := new(big.Int).Lsh(big.NewInt(1), uint(exponent))
				maximum := new(big.Int).Sub(new(big.Int).Set(limit), big.NewInt(1))
				minimum := new(big.Int)
				if signed {
					minimum.Neg(limit)
				}
				for _, n := range []*big.Int{minimum, big.NewInt(0), maximum} {
					output, err := repairM1240Execute(CmdQueryPayload(), []string{path, "set", n.String()}, context.Background(), client.Context{})
					require.NoError(t, err)
					payload, err := hex.DecodeString(output)
					require.NoError(t, err)
					method, err := contract.MethodById(payload[:4])
					require.NoError(t, err)
					require.Equal(t, "set", method.Name)
					decoded, err := method.Inputs.Unpack(payload[4:])
					require.NoError(t, err)
					require.Equal(t, n.String(), fmt.Sprint(decoded[0]), "CLI -> ABI must preserve the exact signed/unsigned value")
				}
				invalid := []string{new(big.Int).Sub(new(big.Int).Set(minimum), big.NewInt(1)).String(), limit.String(), "garbage", "1.5", "0x10"}
				if !signed {
					invalid = append(invalid, "-0")
				}
				for _, value := range invalid {
					output, err := repairM1240Execute(CmdQueryPayload(), []string{path, "set", value}, context.Background(), client.Context{})
					require.Error(t, err, "%s %s", typ, value)
					require.Empty(t, output)
				}
				for _, args := range [][]string{nil, {"unknown"}, {"set"}, {"set", "0", "1"}} {
					payload, err := getMethodPayload(contract, args)
					require.Error(t, err)
					require.Empty(t, payload)
				}
			})
		}
	}
}

func TestRepairM_1240_ERC20ArgumentCounts(t *testing.T) {
	for method, count := range map[string]int{"name": 2, "symbol": 2, "decimals": 2, "totalSupply": 2, "balanceOf": 3, "allowance": 4} {
		t.Run(method, func(t *testing.T) {
			valid := []string{repairM1240Address, method}
			for len(valid) < count {
				valid = append(valid, repairM1240Address)
			}
			require.NoError(t, CmdQueryERC20().Args(CmdQueryERC20(), valid))
			for n := 0; n <= count+1; n++ {
				if n == count {
					continue
				}
				args := append([]string(nil), valid...)
				for len(args) < n {
					args = append(args, repairM1240Address)
				}
				args = args[:n]
				// A deliberately invalid endpoint must not be read before argument validation.
				cmd := CmdQueryERC20()
				require.NoError(t, cmd.Flags().Set("node", ":invalid"))
				output, err := repairM1240Execute(cmd, args, context.Background(), client.Context{})
				require.Error(t, err)
				require.EqualError(t, err, validateERC20QueryArgs(nil, args).Error())
				require.Empty(t, output)
				require.Error(t, cmd.RunE(cmd, args), "direct handler must also reject missing arguments")
			}
		})
	}
	for _, args := range [][]string{{repairM1240Address, "unknown"}, {repairM1240Address, "unknown", repairM1240Address}} {
		output, err := repairM1240Execute(CmdQueryERC20(), args, context.Background(), client.Context{})
		require.ErrorContains(t, err, "unknown method")
		require.Empty(t, output)
	}

	contract, err := native.NativeMetaData.GetAbi()
	require.NoError(t, err)
	var calls atomic.Int32
	// The loopback peer consumes the actual protobuf StaticCall request and ABI,
	// and produces its reply with the production protobuf and ABI encoders.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage            `json:"id"`
			Method string                     `json:"method"`
			Params coretypes.RequestABCIQuery `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "abci_query", request.Method)
		require.Equal(t, "/paxprotocol.paxchain.evm.Query/StaticCall", request.Params.Path)
		var query types.QueryStaticCallRequest
		require.NoError(t, query.Unmarshal(request.Params.Data))
		require.Equal(t, repairM1240Address, query.To)
		method, err := contract.MethodById(query.Data[:4])
		require.NoError(t, err)
		params, err := method.Inputs.Unpack(query.Data[4:])
		require.NoError(t, err)
		for _, value := range params {
			require.Equal(t, common.HexToAddress(repairM1240Address), value)
		}
		var result interface{} = big.NewInt(42)
		switch method.Name {
		case "name", "symbol":
			result = "Token"
		case "decimals":
			result = uint8(18)
		}
		data, err := method.Outputs.Pack(result)
		require.NoError(t, err)
		response := types.QueryStaticCallResponse{Data: data}
		value, err := response.Marshal()
		require.NoError(t, err)
		calls.Add(1)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0", "id": request.ID,
			"result": &coretypes.ResultABCIQuery{Response: abci.ResponseQuery{Value: value, Height: 1}},
		}))
	}))
	defer server.Close()
	node, err := client.NewClientFromNode(server.URL)
	require.NoError(t, err)
	for method, want := range map[string]string{"name": "Token", "symbol": "Token", "decimals": "18", "totalSupply": "42", "balanceOf": "42", "allowance": "42"} {
		args := []string{repairM1240Address, method}
		if method == "balanceOf" || method == "allowance" {
			args = append(args, repairM1240Address)
		}
		if method == "allowance" {
			args = append(args, repairM1240Address)
		}
		output, err := repairM1240Execute(CmdQueryERC20(), args, context.Background(), client.Context{}.WithClient(node))
		require.NoError(t, err)
		require.Equal(t, want, output)
	}
	require.EqualValues(t, 6, calls.Load())
}

func TestRepairM_1240_TokenPayloadValidation(t *testing.T) {
	a20, err := cw20.Cw20MetaData.GetAbi()
	require.NoError(t, err)
	a721, err := cw721.Cw721MetaData.GetAbi()
	require.NoError(t, err)
	a1155, err := cw1155.Cw1155MetaData.GetAbi()
	require.NoError(t, err)
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
	overflow := new(big.Int).Lsh(big.NewInt(1), 256).String()
	cases := []struct {
		name     string
		command  func() *cobra.Command
		contract *abi.ABI
		args     []string
		numeric  []int
		arrays   []int
		boolean  int
	}{
		{"erc20-transfer", CmdQueryERC20Payload, a20, []string{"transfer", repairM1240Address, max}, []int{2}, nil, 0},
		{"erc20-approve", CmdQueryERC20Payload, a20, []string{"approve", repairM1240Address, max}, []int{2}, nil, 0},
		{"erc20-transferFrom", CmdQueryERC20Payload, a20, []string{"transferFrom", repairM1240Address, repairM1240Address, max}, []int{3}, nil, 0},
		{"erc721-approve", CmdQueryERC721Payload, a721, []string{"approve", repairM1240Address, max}, []int{2}, nil, 0},
		{"erc721-transferFrom", CmdQueryERC721Payload, a721, []string{"transferFrom", repairM1240Address, repairM1240Address, max}, []int{3}, nil, 0},
		{"erc721-approval", CmdQueryERC721Payload, a721, []string{"setApprovalForAll", repairM1240Address, "true"}, nil, nil, 2},
		{"erc1155-transfer", CmdQueryERC1155Payload, a1155, []string{"safeTransferFrom", repairM1240Address, repairM1240Address, max, max, "data"}, []int{3, 4}, nil, 0},
		{"erc1155-batch", CmdQueryERC1155Payload, a1155, []string{"safeBatchTransferFrom", repairM1240Address, repairM1240Address, "[0, " + max + "]", "[0, " + max + "]", "data"}, nil, []int{3, 4}, 0},
		{"erc1155-approval", CmdQueryERC1155Payload, a1155, []string{"setApprovalForAll", repairM1240Address, "true"}, nil, nil, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decode := func(args []string) []interface{} {
				output, err := repairM1240Execute(tc.command(), args, context.Background(), client.Context{})
				require.NoError(t, err)
				payload, err := hex.DecodeString(output)
				require.NoError(t, err)
				method, err := tc.contract.MethodById(payload[:4])
				require.NoError(t, err)
				require.Equal(t, args[0], method.Name)
				values, err := method.Inputs.Unpack(payload[4:])
				require.NoError(t, err)
				return values
			}
			values := decode(tc.args)
			for _, i := range tc.numeric {
				require.Equal(t, max, values[i-1].(*big.Int).String())
				args := append([]string(nil), tc.args...)
				args[i] = "0"
				require.Zero(t, decode(args)[i-1].(*big.Int).Sign())
			}
			for _, i := range tc.arrays {
				decoded := values[i-1].([]*big.Int)
				require.Len(t, decoded, 2)
				require.Zero(t, decoded[0].Sign())
				require.Equal(t, max, decoded[1].String())
			}
			if tc.boolean != 0 {
				require.Equal(t, true, values[tc.boolean-1])
				args := append([]string(nil), tc.args...)
				args[tc.boolean] = "false"
				require.Equal(t, false, decode(args)[tc.boolean-1])
			}
			reject := func(args []string) {
				output, err := repairM1240Execute(tc.command(), args, context.Background(), client.Context{})
				require.Error(t, err, "%v", args)
				require.Empty(t, output)
			}
			for _, i := range append(append([]int(nil), tc.numeric...), tc.arrays...) {
				for _, invalid := range []string{"bad", "-1", "-0", "1.5", overflow} {
					args := append([]string(nil), tc.args...)
					args[i] = invalid
					for _, arrayIndex := range tc.arrays {
						if i == arrayIndex {
							args[i] = "[0," + invalid + "]"
						}
					}
					reject(args)
				}
			}
			if tc.boolean != 0 {
				for _, invalid := range []string{"yes", "TRUE", "False", "1", ""} {
					args := append([]string(nil), tc.args...)
					args[tc.boolean] = invalid
					reject(args)
				}
			}
			for n := 1; n < len(tc.args); n++ {
				reject(tc.args[:n])
			}
			reject(append(append([]string(nil), tc.args...), "extra"))
			reject([]string{"unknown"})
		})
	}
}

type repairM1240RPCRequest struct {
	ID     json.RawMessage   `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func repairM1240ReadRPC(t *testing.T, r *http.Request) repairM1240RPCRequest {
	t.Helper()
	require.Equal(t, http.MethodPost, r.Method)
	require.Equal(t, "application/json", r.Header.Get("Content-Type"))
	var req repairM1240RPCRequest
	require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
	return req
}

func repairM1240RPCResult(t *testing.T, w http.ResponseWriter, req repairM1240RPCRequest, result interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result}))
}

func repairM1240TxServer(t *testing.T, consume func(*ethtypes.Transaction)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := repairM1240ReadRPC(t, r)
		switch req.Method {
		case "eth_getTransactionCount":
			require.Len(t, req.Params, 2)
			require.JSONEq(t, `"pending"`, string(req.Params[1]))
			repairM1240RPCResult(t, w, req, "0x5")
		case "eth_chainId":
			require.Empty(t, req.Params)
			repairM1240RPCResult(t, w, req, "0x7d")
		case "eth_sendRawTransaction":
			require.Len(t, req.Params, 1)
			var encoded hexutil.Bytes
			require.NoError(t, json.Unmarshal(req.Params[0], &encoded))
			var transaction ethtypes.Transaction
			require.NoError(t, transaction.UnmarshalBinary(encoded))
			consume(&transaction)
			repairM1240RPCResult(t, w, req, transaction.Hash().Hex())
		default:
			t.Errorf("unexpected RPC method %q", req.Method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}

func TestRepairM_1240_DeployBinaryErrorsAndConventions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contract.bin")
	output, err := repairM1240Execute(CmdDeployContract(), []string{path}, context.Background(), client.Context{})
	require.ErrorContains(t, err, path)
	require.ErrorContains(t, err, "read contract binary")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Empty(t, output)
	for _, text := range []string{"zz", "abc", "00 11", "0X0011", "0xzz"} {
		require.NoError(t, os.WriteFile(path, []byte(text), 0600))
		output, err := repairM1240Execute(CmdDeployContract(), []string{path}, context.Background(), client.Context{})
		require.ErrorContains(t, err, path)
		require.ErrorContains(t, err, "decode contract binary")
		require.Empty(t, output)
		var invalid hex.InvalidByteError
		require.True(t, errors.As(err, &invalid) || errors.Is(err, hex.ErrLength), "underlying hex error must be wrapped")
	}
	require.Contains(t, CmdDeployContract().Long, "whitespace")
	require.Contains(t, CmdDeployContract().Long, "0x")
	clientCtx, key := repairM1240KeyContext(t)
	var submissions atomic.Int32
	server := repairM1240TxServer(t, func(transaction *ethtypes.Transaction) {
		require.Nil(t, transaction.To())
		require.Equal(t, []byte{0x60, 0x00, 0x60, 0x01}, transaction.Data())
		require.EqualValues(t, 5, transaction.Nonce())
		require.Equal(t, "125", transaction.ChainId().String())
		sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(transaction.ChainId()), transaction)
		require.NoError(t, err)
		require.Equal(t, crypto.PubkeyToAddress(key.PublicKey), sender)
		submissions.Add(1)
	})
	defer server.Close()
	for _, text := range []string{"60006001", " \n60006001\t\r\n", "0x60006001", "\n 0x60006001 \n"} {
		require.NoError(t, os.WriteFile(path, []byte(text), 0600))
		_, err := repairM1240Execute(CmdDeployContract(), []string{path, "--evm-rpc=" + server.URL}, context.Background(), clientCtx)
		require.NoError(t, err)
	}
	require.EqualValues(t, 4, submissions.Load())
}

func TestRepairM_1240_RPCResponseValidation(t *testing.T) {
	_, key := repairM1240KeyContext(t)
	invalid := []struct{ name, body, message string }{
		{"rpc-error", `{"jsonrpc":"2.0","id":"send-cli","error":{"code":-32000,"message":"account unavailable","data":{"retry":false}}}`, "account unavailable"},
		{"error-with-result", `{"result":"0x1","error":{"code":-32603,"message":"internal failure"}}`, "internal failure"},
		{"missing", `{}`, "no string result"},
		{"null", `{"result":null}`, "no string result"},
		{"numeric", `{"result":4}`, "decode RPC response"},
		{"object", `{"result":{"value":"0x1"}}`, "decode RPC response"},
		{"array", `{"result":[]}`, "decode RPC response"},
		{"boolean", `{"result":false}`, "decode RPC response"},
		{"bad-json", `{`, "decode RPC response"},
		{"empty", `{"result":""}`, "invalid"},
		{"no-prefix", `{"result":"123"}`, "invalid"},
		{"empty-hex", `{"result":"0x"}`, "invalid"},
		{"bad-hex", `{"result":"0xgg"}`, "invalid"},
		{"negative", `{"result":"-0x1"}`, "invalid"},
		{"leading-zero", `{"result":"0x01"}`, "invalid"},
		{"overflow", `{"result":"0x1` + strings.Repeat("0", 64) + `"}`, "invalid"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				req := repairM1240ReadRPC(t, r)
				require.Contains(t, []string{"eth_chainId", "eth_getTransactionCount"}, req.Method)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			nonce, err := getNonce(context.Background(), server.URL, key.PublicKey)
			require.ErrorContains(t, err, tc.message)
			require.Zero(t, nonce)
			chainID, err := getChainId(context.Background(), server.URL)
			require.ErrorContains(t, err, tc.message)
			require.Nil(t, chainID)
			if tc.name == "rpc-error" {
				var rpcError *cliRPCError
				require.ErrorAs(t, err, &rpcError)
				require.Equal(t, -32000, rpcError.Code)
				require.JSONEq(t, `{"retry":false}`, string(rpcError.Data))
			}
		})
	}
	for _, tc := range []struct {
		result string
		nonce  uint64
	}{{"0x0", 0}, {"0x5", 5}, {"0xffffffffffffffff", ^uint64(0)}} {
		server := getRPCServer(t, tc.result)
		nonce, err := getNonce(context.Background(), server.URL, key.PublicKey)
		require.NoError(t, err)
		require.Equal(t, tc.nonce, nonce)
		chainID, err := getChainId(context.Background(), server.URL)
		require.NoError(t, err)
		expected, err := hexutil.DecodeBig(hexutil.EncodeUint64(tc.nonce))
		require.NoError(t, err)
		require.Equal(t, expected, chainID)
		server.Close()
	}
	server := getRPCServer(t, "0x10000000000000000")
	defer server.Close()
	_, err := getNonce(context.Background(), server.URL, key.PublicKey)
	require.ErrorContains(t, err, "invalid nonce")
	chainID, err := getChainId(context.Background(), server.URL)
	require.NoError(t, err)
	require.Equal(t, new(big.Int).Lsh(big.NewInt(1), 64), chainID)
	server256 := getRPCServer(t, "0x"+strings.Repeat("f", 64))
	defer server256.Close()
	chainID, err = getChainId(context.Background(), server256.URL)
	require.NoError(t, err)
	require.Equal(t, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)), chainID)
}

func TestRepairM_1240_RPCMetadataFailureStopsSubmission(t *testing.T) {
	clientCtx, _ := repairM1240KeyContext(t)
	for _, failMethod := range []string{"eth_getTransactionCount", "eth_chainId"} {
		t.Run(failMethod, func(t *testing.T) {
			var submissions atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				req := repairM1240ReadRPC(t, r)
				if req.Method == failMethod {
					require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
						"jsonrpc": "2.0", "id": req.ID, "error": map[string]interface{}{"code": -32000, "message": "metadata lookup refused"},
					}))
					return
				}
				if req.Method == "eth_sendRawTransaction" {
					submissions.Add(1)
				}
				repairM1240RPCResult(t, w, req, "0x7d")
			}))
			defer server.Close()
			_, err := repairM1240Execute(CmdSend(), []string{repairM1240Address, "1", "--evm-rpc=" + server.URL}, context.Background(), clientCtx)
			require.ErrorContains(t, err, "metadata lookup refused")
			require.Zero(t, submissions.Load())
		})
	}
}

func TestRepairM_1240_RPCCancellationAndBounds(t *testing.T) {
	clientCtx, key := repairM1240KeyContext(t)
	for _, stage := range []string{"pax_associate", "eth_getTransactionCount", "eth_chainId", "eth_sendRawTransaction"} {
		t.Run(stage, func(t *testing.T) {
			command := func() (*cobra.Command, []string) {
				if stage == "pax_associate" {
					return CmdAssociateAddress(), []string{hex.EncodeToString(crypto.FromECDSA(key))}
				}
				return CmdSend(), []string{repairM1240Address, "1"}
			}
			t.Run("cancel", func(t *testing.T) {
				reached := make(chan struct{})
				released := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					req := repairM1240ReadRPC(t, r)
					if req.Method != stage {
						repairM1240RPCResult(t, w, req, "0x7d")
						return
					}
					// Flush a partial response so cancellation must interrupt a body read.
					_, _ = io.WriteString(w, `{"result":`)
					w.(http.Flusher).Flush()
					close(reached)
					<-r.Context().Done()
					close(released)
				}))
				defer server.Close()
				defer server.CloseClientConnections()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				cmd, args := command()
				go func() {
					_, err := repairM1240Execute(cmd, append(args, "--evm-rpc="+server.URL), ctx, clientCtx)
					done <- err
				}()
				select {
				case <-reached:
				case <-time.After(5 * time.Second):
					t.Fatal("RPC stage was not reached")
				}
				cancel()
				select {
				case err := <-done:
					require.ErrorIs(t, err, context.Canceled)
				case <-time.After(3 * time.Second):
					t.Fatal("command ignored Cobra context cancellation")
				}
				select {
				case <-released:
				case <-time.After(3 * time.Second):
					t.Fatal("HTTP request was left open after cancellation")
				}
			})
			t.Run("oversized", func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					req := repairM1240ReadRPC(t, r)
					if req.Method != stage {
						repairM1240RPCResult(t, w, req, "0x7d")
						return
					}
					_, _ = io.WriteString(w, strings.Repeat(" ", maxCLIResponseBytes+1))
				}))
				defer server.Close()
				cmd, args := command()
				_, err := repairM1240Execute(cmd, append(args, "--evm-rpc="+server.URL), context.Background(), clientCtx)
				require.ErrorContains(t, err, "RPC response exceeds")
			})
			t.Run("http-error", func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					req := repairM1240ReadRPC(t, r)
					if req.Method != stage {
						repairM1240RPCResult(t, w, req, "0x7d")
						return
					}
					http.Error(w, "RPC unavailable", http.StatusServiceUnavailable)
				}))
				defer server.Close()
				cmd, args := command()
				_, err := repairM1240Execute(cmd, append(args, "--evm-rpc="+server.URL), context.Background(), clientCtx)
				require.ErrorContains(t, err, "503")
			})
		})
	}
	// The exact response limit is accepted; the extra byte is what causes refusal.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("a", maxCLIResponseBytes))
	}))
	defer server.Close()
	body, err := postCLIRequest(context.Background(), server.URL, `{}`)
	require.NoError(t, err)
	require.Len(t, body, maxCLIResponseBytes)
}

func TestRepairM_1240_RPCFiniteTimeout(t *testing.T) {
	require.Greater(t, cliRPCTimeout, time.Duration(0))
	require.LessOrEqual(t, cliRPCTimeout, 30*time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	defer server.CloseClientConnections()
	start := time.Now()
	_, err := getChainId(context.Background(), server.URL)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded), "%v", err)
	require.Less(t, time.Since(start), cliRPCTimeout+3*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start = time.Now()
	_, err = getChainId(ctx, server.URL)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 3*time.Second)
}
