// Proves: cli.GetQueryCmd/GetTxCmd -> built paxd subprocess and HTTP ABCI/transaction JSON -> tokenfactory.AppModule.RegisterServices.
//
// The proof uses the actual daemon, SDK codecs, routers and keepers. Queries
// cross a loopback HTTP JSON-RPC server into the module's registered services;
// generated transactions cross process stdout into the SDK transaction decoder
// and the module's registered message handler. No transport or keeper is doubled.
// Every query is repeated by a fresh process, including after a committed store
// is closed and reopened. Malformed producer output is refused by real decoders.
//
// Repairs tx.go: ParseAllowListJSON previously accepted a second JSON document
// or malformed trailing bytes, silently ignoring part of the producer's file.
package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	tokenfactorymodule "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

type proofCliState struct {
	ctx      sdk.Context
	stores   sdk.CommitMultiStore
	bank     bankkeeper.BaseKeeper
	keeper   keeper.Keeper
	codec    *codec.ProtoCodec
	txConfig client.TxConfig
	queries  *baseapp.GRPCQueryRouter
	messages *baseapp.MsgServiceRouter
}

// tokenfactory opens real module state and registers exactly the consumer entry
// points used by the application. Callers close it before opening the same DB.
func tokenfactory(t *testing.T, directory string) *proofCliState {
	t.Helper()
	db, err := sdk.NewLevelDB("proof-cli", directory)
	require.NoError(t, err)
	stores := store.NewCommitMultiStore(db)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	factoryKey := sdk.NewKVStoreKey(types.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []sdk.StoreKey{bankKey, factoryKey, paramsKey} {
		stores.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	stores.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, stores.LoadLatestVersion())
	ctx := sdk.NewContext(stores, tmproto.Header{Height: stores.LastCommitID().Version + 1}, false)
	registry := cdctypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	basic := tokenfactorymodule.NewAppModuleBasic()
	basic.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	bank := bankkeeper.NewBaseKeeper(cdc, bankKey, nil,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, banktypes.ModuleName), nil)
	factory := keeper.NewKeeper(cdc, factoryKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName), nil, bank)
	queries := baseapp.NewGRPCQueryRouter()
	queries.SetInterfaceRegistry(registry)
	messages := baseapp.NewMsgServiceRouter()
	messages.SetInterfaceRegistry(registry)
	tokenfactorymodule.NewAppModule(factory, nil, bank).RegisterServices(module.NewConfigurator(cdc, messages, queries))
	return &proofCliState{ctx: ctx, stores: stores, bank: bank, keeper: factory,
		codec: cdc, txConfig: authtx.NewTxConfig(cdc, authtx.DefaultSignModes), queries: queries, messages: messages}
}

func TestProofCli(t *testing.T) {
	started := time.Now()
	defer func() { t.Logf("CLI proof wall time: %s", time.Since(started)) }()
	// Building the executable is part of the proof: all command invocations below
	// execute this repository's daemon, rather than a helper test process.
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../.."))
	binary := filepath.Join(t.TempDir(), "paxd")
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./daemon/paxd")
	build.Dir = root
	t.Log("building real paxd")
	buildOutput, err := build.CombinedOutput()
	t.Logf("real paxd build wall time: %s", time.Since(started))
	require.NoError(t, err, "build real paxd: %s", buildOutput)

	home := t.TempDir()
	run := func(args ...string) ([]byte, string, error) {
		t.Helper()
		t.Logf("paxd %v", args)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, append(args, "--home", home)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		return output, stderr.String(), err
	}
	success := func(args ...string) []byte {
		t.Helper()
		output, stderr, err := run(args...)
		require.NoError(t, err, "paxd %v: %s", args, stderr)
		return output
	}
	refusal := func(want string, args ...string) {
		t.Helper()
		output, stderr, err := run(args...)
		require.Error(t, err, "paxd %v unexpectedly accepted input", args)
		require.Contains(t, stderr, want)
		require.Empty(t, output, "refused commands must not emit a usable result")
	}

	// The two module roots are producers too. Fresh processes must discover the
	// registered commands twice and reject an unknown command under either root.
	for i := 0; i < 2; i++ {
		queryHelp := string(success("query", "tokenfactory", "--help"))
		for _, name := range []string{"denom-allow-list", "denom-metadata", "denom-paused"} {
			require.Contains(t, queryHelp, name)
		}
		require.Contains(t, string(success("tx", "tokenfactory", "--help")), "set-denom-paused")
	}
	refusal("unknown command", "query", "tokenfactory", "proof-unknown")
	refusal("unknown command", "tx", "tokenfactory", "proof-unknown")

	// Derive the address with the daemon's real key producer so this test does
	// not depend on process-global SDK bech32 configuration in other tests.
	keyJSON := success("keys", "add", "proof-admin", "--keyring-backend", "test", "--output", "json")
	var key struct {
		Address string `json:"address"`
	}
	require.NoError(t, json.Unmarshal(keyJSON, &key))
	require.NotEmpty(t, key.Address)
	directory := t.TempDir()
	state := tokenfactory(t, directory)
	t.Cleanup(func() { require.NoError(t, state.stores.Close()) })
	denom, err := state.keeper.CreateDenom(state.ctx, key.Address, "proof")
	require.NoError(t, err)
	metadata, found := state.bank.GetDenomMetaData(state.ctx, denom)
	require.True(t, found)
	// Use explicit, nonempty metadata fields so the proof checks their exact
	// preservation through protobuf JSON, whose repeated fields encode as [].
	metadata.Description = "Metadata produced for the CLI path proof"
	metadata.DenomUnits[0].Aliases = []string{"uproof"}
	state.bank.SetDenomMetaData(state.ctx, metadata)
	allowList := banktypes.AllowList{Addresses: []string{key.Address}}
	state.bank.SetDenomAllowList(state.ctx, denom, allowList)

	var mu sync.Mutex
	requests := map[string]int{}
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			mu.Lock()
			defer mu.Unlock()
			requests[req.Path]++
			handler := state.queries.Route(req.Path)
			if handler == nil {
				return nil, fmt.Errorf("unknown query route %s", req.Path)
			}
			response, err := handler(state.ctx, abci.RequestQuery{Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove})
			return &coretypes.ResultABCIQuery{Response: response}, err
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	count := func(method string) int {
		mu.Lock()
		defer mu.Unlock()
		return requests["/paxprotocol.paxchain.tokenfactory.Query/"+method]
	}
	query := func(command, method, argument string, response proto.Message) []byte {
		t.Helper()
		before := count(method)
		output := success("query", "tokenfactory", command, argument, "--node", server.URL, "--output", "json")
		require.Equal(t, before+1, count(method), "a fresh process must reach the registered keeper")
		require.NoError(t, state.codec.UnmarshalAsJSON(output, response))
		malformed := output[:len(output)/2]
		require.Error(t, state.codec.UnmarshalAsJSON(malformed, response), "consumer must reject corrupted producer JSON")
		// Restore the successfully decoded response for the caller's assertions.
		require.NoError(t, state.codec.UnmarshalAsJSON(output, response))
		return output
	}
	observe := func(paused bool) {
		t.Helper()
		for i := 0; i < 2; i++ {
			var gotMetadata types.QueryDenomMetadataResponse
			query("denom-metadata", "DenomMetadata", denom, &gotMetadata)
			require.Equal(t, metadata, gotMetadata.Metadata)
			var gotAllowList types.QueryDenomAllowListResponse
			query("denom-allow-list", "DenomAllowList", denom, &gotAllowList)
			require.Equal(t, allowList, gotAllowList.AllowList)
			var gotPaused types.QueryDenomPausedResponse
			query("denom-paused", "DenomPaused", denom, &gotPaused)
			require.Equal(t, paused, gotPaused.Paused)
			require.Empty(t, gotPaused.IssuerDid)
		}
	}
	generate := func(sender, paused string) []byte {
		t.Helper()
		return success("tx", "tokenfactory", "set-denom-paused", denom, paused,
			"--from", sender, "--generate-only", "--offline", "--keyring-backend", "test", "--chain-id", "proof-cli")
	}
	consume := func(output []byte) error {
		t.Helper()
		transaction, err := state.txConfig.TxJSONDecoder()(output)
		require.NoError(t, err)
		require.Len(t, transaction.GetMsgs(), 1)
		msg, ok := transaction.GetMsgs()[0].(*types.MsgSetDenomPaused)
		require.True(t, ok)
		require.Equal(t, denom, msg.Denom)
		handler := state.messages.Handler(msg)
		require.NotNil(t, handler, "AppModule.RegisterServices must wire the message")
		_, err = handler(state.ctx, msg)
		return err
	}
	reopen := func() {
		t.Helper()
		state.stores.Commit(true)
		require.NoError(t, state.stores.Close())
		mu.Lock()
		defer mu.Unlock()
		state = tokenfactory(t, directory)
	}

	observe(false)
	for _, paused := range []bool{true, false} {
		output := generate(key.Address, strconv.FormatBool(paused))
		transaction, err := state.txConfig.TxJSONDecoder()(output)
		require.NoError(t, err)
		msg := transaction.GetMsgs()[0].(*types.MsgSetDenomPaused)
		require.Equal(t, key.Address, msg.Sender)
		require.Equal(t, paused, msg.Paused)
		_, err = state.txConfig.TxJSONDecoder()(output[:len(output)/2])
		require.Error(t, err, "truncated subprocess output must be refused by the consumer")
		require.NoError(t, consume(output))
		reopen()
		observe(paused)
		// Replay reaches the keeper and is refused; no cached successful result.
		require.ErrorIs(t, consume(output), types.ErrPauseUnchanged)
	}
	wrongKey := success("keys", "add", "proof-other", "--keyring-backend", "test", "--output", "json")
	require.NoError(t, json.Unmarshal(wrongKey, &key))
	require.ErrorIs(t, consume(generate(key.Address, "true")), types.ErrUnauthorized)
	observe(false)
	refusal("invalid syntax", "tx", "tokenfactory", "set-denom-paused", denom, "maybe", "--from", key.Address,
		"--generate-only", "--offline", "--keyring-backend", "test", "--chain-id", "proof-cli")
	for _, name := range []string{"denom-allow-list", "denom-metadata", "denom-paused"} {
		refusal("accepts 1 arg(s)", "query", "tokenfactory", name)
	}
	refusal("authority metadata for denom", "query", "tokenfactory", "denom-paused", denom+"-missing", "--node", server.URL, "--output", "json")
	refusal("client metadata for denom", "query", "tokenfactory", "denom-metadata", denom+"-missing", "--node", server.URL, "--output", "json")
	refusal("invalid denom", "query", "tokenfactory", "denom-allow-list", "", "--node", server.URL, "--output", "json")

	// Feed a real query producer's allow-list value through the supported file
	// format and the real update-denom command's ParseAllowListJSON consumer.
	var produced types.QueryDenomAllowListResponse
	query("denom-allow-list", "DenomAllowList", denom, &produced)
	document, err := state.codec.MarshalAsJSON(&produced.AllowList)
	require.NoError(t, err)
	file := filepath.Join(t.TempDir(), "allow-list.json")
	updateArgs := []string{"tx", "tokenfactory", "update-denom", denom, "--allow-list", file,
		"--from", allowList.Addresses[0], "--generate-only", "--offline", "--keyring-backend", "test", "--chain-id", "proof-cli"}
	for i := 0; i < 2; i++ {
		require.NoError(t, os.WriteFile(file, append(append([]byte(nil), document...), '\n'), 0600))
		transaction, err := state.txConfig.TxJSONDecoder()(success(updateArgs...))
		require.NoError(t, err)
		require.Len(t, transaction.GetMsgs(), 1)
		update, ok := transaction.GetMsgs()[0].(*types.MsgUpdateDenom)
		require.True(t, ok)
		require.Equal(t, &allowList, update.AllowList)
		require.Equal(t, denom, update.Denom)
		require.Equal(t, allowList.Addresses[0], update.Sender)
	}
	require.NoError(t, os.WriteFile(file, append(append([]byte(nil), document...), document...), 0600))
	refusal("exactly one JSON document", updateArgs...)
	require.NoError(t, os.WriteFile(file, append(append([]byte(nil), document...), '!'), 0600))
	refusal("invalid trailing allow list data", updateArgs...)
}
