package cli_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	genutiltest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/client/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/ioutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func repairM1286Execute(cmd *cobra.Command, clientCtx client.Context, args ...string) error {
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	ctx := context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)
	return cmd.ExecuteContext(ctx)
}

func repairM1286Genesis(t *testing.T, count int) (types.GenesisState, []byte, string) {
	t.Helper()
	state := types.GenesisState{
		Params:    types.DefaultParams(),
		Sequences: []types.Sequence{{IDKey: types.KeyLastCodeID, Value: 7}},
	}
	homeDir := setupGenesis(t, state)
	cfg, err := genutiltest.CreateDefaultTendermintConfig(homeDir)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithCodec(keeper.MakeTestCodec(t)).WithHomeDir(homeDir)
	ctx := context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, server.NewContext(viper.New(), cfg))
	wasmPath := filepath.Join("..", "..", "keeper", "testdata", "hackatom.wasm")
	raw, err := os.ReadFile(wasmPath)
	require.NoError(t, err)
	for i := 0; i < count; i++ {
		cmd := cli.GenesisStoreCodeCmd(homeDir, cli.NewDefaultGenesisIO())
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		args := []string{wasmPath, "--run-as", myWellFundedAccount}
		if i > 0 {
			args = append(args, "--instantiate-nobody", "true")
		}
		cmd.SetArgs(args)
		require.NoError(t, cmd.ExecuteContext(ctx))
	}
	state = loadModuleState(t, homeDir)
	require.NoError(t, state.ValidateBasic())
	require.Len(t, state.GenMsgs, count)
	return state, raw, homeDir
}

func TestRepairM_1286_GenesisCodeHash(t *testing.T) {
	state, raw, homeDir := repairM1286Genesis(t, 2)
	for _, genMsg := range state.GenMsgs {
		upload := genMsg.GetStoreCode()
		require.NotNil(t, upload)
		require.True(t, ioutils.IsGzip(upload.WASMByteCode))
		require.NotEqual(t, sha256.Sum256(raw), sha256.Sum256(upload.WASMByteCode))
	}

	cfg, err := genutiltest.CreateDefaultTendermintConfig(homeDir)
	require.NoError(t, err)
	var output bytes.Buffer
	clientCtx := client.Context{}.WithCodec(keeper.MakeTestCodec(t)).WithHomeDir(homeDir).WithOutput(&output)
	ctx := context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, server.NewContext(viper.New(), cfg))
	cmd := cli.GenesisListCodesCmd(homeDir, cli.NewDefaultGenesisIO())
	cmd.SetArgs([]string{})
	require.NoError(t, cmd.ExecuteContext(ctx))
	var preview []cli.CodeMeta
	require.NoError(t, json.Unmarshal(output.Bytes(), &preview))
	require.Len(t, preview, 2)

	sdkCtx, keepers := keeper.CreateDefaultTestInput(t)
	_, err = keeper.InitGenesis(sdkCtx, keepers.WasmKeeper, state, &keepers.StakingKeeper, wasm.NewHandler(keepers.ContractKeeper))
	require.NoError(t, err)
	expectedHash := sha256.Sum256(raw)
	for i, code := range preview {
		require.Equal(t, uint64(7+i), code.CodeID)
		require.Equal(t, expectedHash[:], code.Info.CodeHash)
		info := keepers.WasmKeeper.GetCodeInfo(sdkCtx, code.CodeID)
		require.NotNil(t, info)
		require.Equal(t, code.Info, *info)
		stored, err := keepers.WasmKeeper.GetByteCode(sdkCtx, code.CodeID)
		require.NoError(t, err)
		require.Equal(t, raw, stored)
	}
	// The uncompressed representation must describe the very same code identity.
	for i := range state.GenMsgs {
		state.GenMsgs[i].GetStoreCode().WASMByteCode = raw
	}
	uncompressed, err := cli.GetAllCodes(&state)
	require.NoError(t, err)
	require.Equal(t, preview, uncompressed)
}

func TestRepairM_1286_GenesisNormalizationBounds(t *testing.T) {
	state, raw, _ := repairM1286Genesis(t, 1)
	compressed := state.GenMsgs[0].GetStoreCode().WASMByteCode
	oversized, err := ioutils.GzipIt(bytes.Repeat([]byte{1}, types.MaxWasmSize+1))
	require.NoError(t, err)
	sdkCtx, keepers := keeper.CreateDefaultTestInput(t)
	for name, payload := range map[string][]byte{
		"truncated gzip":      compressed[:len(compressed)-4],
		"expanded size limit": oversized,
		"plain size limit":    append(raw, make([]byte, types.MaxWasmSize)...),
	} {
		t.Run(name, func(t *testing.T) {
			state.GenMsgs[0].GetStoreCode().WASMByteCode = payload
			preview, previewErr := cli.GetAllCodes(&state)
			require.Error(t, previewErr)
			require.Nil(t, preview)
			cached, _ := sdkCtx.CacheContext()
			_, initErr := keeper.InitGenesis(cached, keepers.WasmKeeper, state, &keepers.StakingKeeper, wasm.NewHandler(keepers.ContractKeeper))
			require.Error(t, initErr)
			require.Contains(t, initErr.Error(), previewErr.Error())
			require.Nil(t, keepers.WasmKeeper.GetCodeInfo(cached, 7))
		})
	}
}

func TestRepairM_1286_MigrationValidation(t *testing.T) {
	enc := keeper.MakeEncodingConfig(t)
	keys := keyring.NewInMemory()
	key, err := keys.NewAccount("repair-m1286", testdata.TestMnemonic, "", sdk.FullFundraiserPath, hd.Secp256k1)
	require.NoError(t, err)
	var requests atomic.Int64
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected RPC request", http.StatusInternalServerError)
	}))
	defer endpoint.Close()
	rpc, err := rpchttp.New(endpoint.URL)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithCodec(enc.Marshaler).WithTxConfig(enc.TxConfig).
		WithKeyring(keys).WithFrom("repair-m1286").WithFromAddress(key.GetAddress()).
		WithChainID("repair-m1286").WithClient(rpc)
	contract := keeper.BuildContractAddress(7, 1).String()
	for _, tc := range []struct {
		name, contract, code, payload, errorText string
	}{
		{"invalid address", "invalid", "7", `{}`, "contract"},
		{"zero code ID", contract, "0", `{}`, "code id is required"},
		{"malformed JSON", contract, "7", `{`, "payload msg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			err := repairM1286Execute(cli.GetTxCmd(), clientCtx.WithOutput(&output), "migrate", tc.contract, tc.code, tc.payload)
			require.ErrorContains(t, err, tc.errorText)
			require.Empty(t, output.String())
			require.Zero(t, requests.Load(), "invalid migration must never reach RPC or broadcast")
		})
	}
	// A valid command reaches the real transaction encoder with the same context.
	var output bytes.Buffer
	require.NoError(t, repairM1286Execute(cli.GetTxCmd(), clientCtx.WithOutput(&output), "migrate", contract, "7", `{}`, "--generate-only"))
	decoded, err := enc.TxConfig.TxJSONDecoder()(output.Bytes())
	require.NoError(t, err)
	require.Len(t, decoded.GetMsgs(), 1)
	msg, ok := decoded.GetMsgs()[0].(*types.MsgMigrateContract)
	require.True(t, ok)
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, contract, msg.Contract)
	require.Equal(t, key.GetAddress().String(), msg.Sender)
	require.Equal(t, uint64(7), msg.CodeID)
	require.Zero(t, requests.Load())
}

func TestRepairM_1286_PageKeyRoundTrip(t *testing.T) {
	state, _, _ := repairM1286Genesis(t, 2)
	sdkCtx, keepers := keeper.CreateDefaultTestInput(t)
	_, err := keeper.InitGenesis(sdkCtx, keepers.WasmKeeper, state, &keepers.StakingKeeper, wasm.NewHandler(keepers.ContractKeeper))
	require.NoError(t, err)
	var storeKey sdk.StoreKey
	for _, key := range sdkCtx.MultiStore().StoreKeys() {
		if key.Name() == types.StoreKey {
			storeKey = key
		}
	}
	require.NotNil(t, storeKey)
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(keepers.EncodingConfig.InterfaceRegistry)
	types.RegisterQueryServer(router, keeper.NewGrpcQuerier(keepers.EncodingConfig.Marshaler, storeKey, keepers.WasmKeeper, 3_000_000, nil))
	var requests atomic.Int64
	receivedKeys := make(chan []byte, 2)
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			requests.Add(1)
			var decoded types.QueryCodesRequest
			if err := decoded.Unmarshal(req.Data); err != nil {
				return nil, err
			}
			receivedKeys <- bytes.Clone(decoded.Pagination.Key)
			handler := router.Route(req.Path)
			if handler == nil {
				return nil, types.ErrNotFound
			}
			res, err := handler(sdkCtx, abci.RequestQuery{Path: req.Path, Data: req.Data, Height: int64(req.Height)})
			return &coretypes.ResultABCIQuery{Response: res}, err
		}),
	})
	endpoint := httptest.NewServer(mux)
	defer endpoint.Close()
	rpc, err := rpchttp.New(endpoint.URL)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithCodec(keepers.EncodingConfig.Marshaler).
		WithKeyring(keyring.NewInMemory()).WithClient(rpc).WithOutputFormat("json")
	var firstOutput bytes.Buffer
	require.NoError(t, repairM1286Execute(cli.GetQueryCmd(), clientCtx.WithOutput(&firstOutput), "list-code", "--limit", "1"))
	var first types.QueryCodesResponse
	require.NoError(t, clientCtx.Codec.UnmarshalAsJSON(firstOutput.Bytes(), &first))
	require.Len(t, first.CodeInfos, 1)
	require.Equal(t, uint64(7), first.CodeInfos[0].CodeID)
	require.Empty(t, <-receivedKeys)
	require.NotNil(t, first.Pagination)
	require.NotEmpty(t, first.Pagination.NextKey)
	var printed struct {
		Pagination struct {
			NextKey string `json:"next_key"`
		} `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(firstOutput.Bytes(), &printed))
	require.NotEmpty(t, printed.Pagination.NextKey)
	var secondOutput bytes.Buffer
	require.NoError(t, repairM1286Execute(cli.GetQueryCmd(), clientCtx.WithOutput(&secondOutput), "list-code", "--limit", "1", "--page-key", printed.Pagination.NextKey))
	var second types.QueryCodesResponse
	require.NoError(t, clientCtx.Codec.UnmarshalAsJSON(secondOutput.Bytes(), &second))
	require.Equal(t, first.Pagination.NextKey, <-receivedKeys)
	require.Len(t, second.CodeInfos, 1)
	require.Equal(t, uint64(8), second.CodeInfos[0].CodeID)
	require.Equal(t, first.CodeInfos[0].DataHash, second.CodeInfos[0].DataHash)
	require.Empty(t, second.Pagination.NextKey)
	require.Equal(t, int64(2), requests.Load())
}

func TestRepairM_1286_PageKeyErrors(t *testing.T) {
	enc := keeper.MakeEncodingConfig(t)
	var requests atomic.Int64
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected RPC request", http.StatusInternalServerError)
	}))
	defer endpoint.Close()
	rpc, err := rpchttp.New(endpoint.URL)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithCodec(enc.Marshaler).WithKeyring(keyring.NewInMemory()).WithClient(rpc)
	contract := keeper.BuildContractAddress(7, 1).String()
	for _, args := range [][]string{
		{"list-code"}, {"list-contract-by-code", strconv.Itoa(7)},
		{"contract-state", "all", contract}, {"contract-history", contract}, {"pinned"},
	} {
		for _, failure := range []string{"malformed", "missing flag", "wrong flag type"} {
			t.Run(args[0]+"/"+failure, func(t *testing.T) {
				cmd := cli.GetQueryCmd()
				commandArgs := append([]string(nil), args...)
				if failure == "malformed" {
					commandArgs = append(commandArgs, "--page-key", "%%%")
				} else {
					leaf, _, err := cmd.Find(args)
					require.NoError(t, err)
					leaf.ResetFlags()
					flags.AddQueryFlagsToCmd(leaf)
					if failure == "wrong flag type" {
						leaf.Flags().Bool(flags.FlagPageKey, false, "invalid pagination flag registration")
					}
				}
				var output bytes.Buffer
				err := repairM1286Execute(cmd, clientCtx.WithOutput(&output), commandArgs...)
				require.ErrorContains(t, err, "page key:")
				if failure == "malformed" {
					require.ErrorContains(t, err, "illegal base64")
				}
				require.Empty(t, output.String())
				require.Zero(t, requests.Load(), "invalid cursor must be refused before querying")
			})
		}
	}
}
