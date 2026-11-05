package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/rpc"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/snikch/goodman/transaction"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	core "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	jsonclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repairM1151Case(t *testing.T, id, uri string) transaction.Transaction {
	t.Helper()
	var tx transaction.Transaction
	require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"id":%q,"name":%q,"request":{"uri":%q,"method":"GET","headers":{"X-Contract":"retained"}},"expected":{"statusCode":"200","body":"unchanged contract example","bodySchema":{"type":"object"},"headers":{"content-type":"application/json"}}}`, id, "opaque "+id, uri)), &tx))
	return tx
}

func repairM1151Execute(t *testing.T, remote string, tx transaction.Transaction) transaction.Transaction {
	t.Helper()
	u, err := url.Parse(tx.Request.URI)
	require.NoError(t, err)
	base, err := url.Parse(remote)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), tx.Request.Method, base.ResolveReference(u).String(), strings.NewReader(tx.Request.Body))
	require.NoError(t, err)
	for key, value := range tx.Request.Headers {
		req.Header.Set(key, value.(string))
	}
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var real transaction.Transaction
	require.NoError(t, json.Unmarshal([]byte(`{"real":{"body":"","statusCode":200}}`), &real))
	tx.Real = real.Real
	tx.Real.Body = string(body)
	tx.Real.StatusCode = response.StatusCode
	return tx
}

func repairM1151Hooks(t *testing.T, runner *HooksRunner) *rpc.Client {
	t.Helper()
	handler, err := hookHandler(runner)
	require.NoError(t, err)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	client, err := rpc.DialHTTPPath("tcp", u.Host, "/")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

func TestRepairM_1151_LiveContractHooks(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	t.Cleanup(cancel)
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	conf.Mode = config.ModeValidator
	conf.Consensus.CreateEmptyBlocks = false
	app := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	n, closeNode, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, closeNode(ctx)) })
	env := n.(interface{ RPCEnvironment() *core.Environment }).RPCEnvironment()
	remote := strings.Replace(conf.RPC.ListenAddress, "tcp://", "http://", 1)
	chain, err := rpchttp.New(remote)
	require.NoError(t, err)
	bootstrap, err := chain.BroadcastTxCommit(ctx, types.Tx("bootstrap=ready"))
	require.NoError(t, err)
	require.Zero(t, bootstrap.CheckTx.Code)
	require.Zero(t, bootstrap.TxResult.Code)
	block, err := chain.Block(ctx, &bootstrap.Height)
	require.NoError(t, err)
	pv, err := privval.LoadFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile())
	require.NoError(t, err)
	genesis, err := types.GenesisDocFromFile(conf.GenesisFile())
	require.NoError(t, err)
	vals, err := env.StateStore.LoadValidators(bootstrap.Height)
	require.NoError(t, err)
	votes := make([]*types.Vote, 2)
	for i := range votes {
		vote := &types.Vote{
			ValidatorAddress: pv.Key.PrivKey.Public().Address(), ValidatorIndex: 0,
			Height: bootstrap.Height, Type: tmproto.PrevoteType, Timestamp: block.Block.Time,
			BlockID: types.BlockID{Hash: crypto.Checksum([]byte(fmt.Sprintf("conflict-%d", i))).Bytes(),
				PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.Checksum([]byte("parts")).Bytes()}},
		}
		vote.Signature = utils.Some(pv.Key.PrivKey.Sign(types.VoteSignBytes(genesis.ChainID, vote.ToProto())))
		require.NoError(t, vote.Verify(genesis.ChainID, pv.Key.PrivKey.Public()))
		votes[i] = vote
	}
	evidence, err := types.NewDuplicateVoteEvidence(votes[0], votes[1], block.Block.Time, vals)
	require.NoError(t, err)
	require.NoError(t, evidence.ValidateBasic())

	fixtures := Fixtures{Seed: types.Tx("repair-key=committed-value"), Evidence: coretypes.RequestBroadcastEvidence{Evidence: evidence}}
	fixtures.Query.Path = "/store"
	fixtures.Query.Data = []byte("repair-key")
	fixtures.Query.Value = []byte("committed-value")
	// The production kvstore application charges no fee and needs no bank
	// account. Live bank applications supply funded, signed bytes in this same
	// fixture format; the hooks enforce successful CheckTx and execution.
	for i := range 5 {
		fixtures.Transactions = append(fixtures.Transactions, types.Tx(fmt.Sprintf("repair-%d=value-%d", i, i)))
	}
	encoded, err := json.Marshal(fixtures)
	require.NoError(t, err)
	var decoded Fixtures
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, evidence.Hash(), decoded.Evidence.Evidence.Hash())
	rpcClient, err := jsonclient.New(remote)
	require.NoError(t, err)
	runner, err := newHooksRunner(rpcClient, decoded)
	require.NoError(t, err)
	hooks := repairM1151Hooks(t, runner)

	paths := []string{"/broadcast_tx_sync?tx=456", "/broadcast_tx?tx=456", "/broadcast_tx_async?tx=123", "/broadcast_tx_commit?tx=785", "/check_tx?tx=785", "/tx?hash=0x00&prove=true", "/tx_search?query=invalid&prove=true&order_by=asc", "/abci_query?path=/a/b/c&data=IHAVENOIDEA&prove=false", "/broadcast_evidence?evidence=JSON_EVIDENCE_encoded"}
	cases := make([]*transaction.Transaction, len(paths))
	for i, path := range paths {
		tx := repairM1151Case(t, fmt.Sprintf("case-%d", i), path)
		cases[i] = &tx
	}
	var prepared []*transaction.Transaction
	require.NoError(t, hooks.Call("HooksRunner.RunBeforeAll", cases, &prepared))
	require.Len(t, prepared, len(paths))
	var emitted []transaction.Transaction
	for i, original := range cases {
		var tx transaction.Transaction
		require.NoError(t, hooks.Call("HooksRunner.RunBeforeEach", *original, &tx))
		require.False(t, tx.Skip)
		require.Nil(t, tx.Fail)
		require.Equal(t, original.Expected, tx.Expected)
		require.Equal(t, "retained", tx.Request.Headers["X-Contract"])
		if i == 5 {
			require.Equal(t, "POST", tx.Request.Method)
			var request rpctypes.RPCRequest
			require.NoError(t, json.Unmarshal([]byte(tx.Request.Body), &request))
			require.Equal(t, "tx", request.Method)
			var params coretypes.RequestTx
			require.NoError(t, json.Unmarshal(request.Params, &params))
			require.Equal(t, fixtures.Seed.Hash().Bytes(), params.Hash)
			require.True(t, params.Prove)
		}
		tx = repairM1151Execute(t, remote, tx)
		var validated transaction.Transaction
		require.NoError(t, hooks.Call("HooksRunner.RunBeforeEachValidation", tx, &validated), "endpoint %s: %s", paths[i], tx.Real.Body)
		require.Equal(t, original.Expected, validated.Expected)
		require.False(t, validated.Skip)
		emitted = append(emitted, tx)
	}
	lookup, err := chain.Tx(ctx, fixtures.Seed.Hash().Bytes(), true)
	require.NoError(t, err)
	require.Equal(t, fixtures.Seed, lookup.Tx)
	require.NoError(t, lookup.Proof.Validate(lookup.Proof.RootHash))
	query, err := chain.ABCIQuery(ctx, fixtures.Query.Path, fixtures.Query.Data)
	require.NoError(t, err)
	require.Equal(t, fixtures.Query.Value, query.Response.Value)

	t.Run("rejects-another-successful-transactions-hash", func(t *testing.T) {
		wrong := emitted[0]
		real := *wrong.Real
		wrong.Real = &real
		wrong.Real.Body = emitted[1].Real.Body
		var reply transaction.Transaction
		require.Error(t, hooks.Call("HooksRunner.RunBeforeEachValidation", wrong, &reply))
	})
	t.Run("rejects-unexpected-server-errors", func(t *testing.T) {
		// Environment without an evidence pool emits its real unavailable error.
		mux := http.NewServeMux()
		rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{"broadcast_evidence": rpcserver.NewRPCFunc((&core.Environment{}).BroadcastEvidence)})
		server := httptest.NewServer(mux)
		defer server.Close()
		failed := repairM1151Execute(t, server.URL, emitted[8])
		var reply transaction.Transaction
		require.Error(t, hooks.Call("HooksRunner.RunBeforeEachValidation", failed, &reply))
	})
	t.Run("unsupported-is-an-asserted-error", func(t *testing.T) {
		// A real RPC server with this route unavailable produces method-not-found.
		routes := core.NewRoutesMap(env, nil)
		delete(routes, "broadcast_evidence")
		mux := http.NewServeMux()
		rpcserver.RegisterRPCFuncs(mux, routes)
		server := httptest.NewServer(mux)
		defer server.Close()
		unsupportedFixtures := fixtures
		unsupportedFixtures.Seed = types.Tx("unsupported-seed=value")
		want := rpctypes.RPCError{Code: int(rpctypes.CodeMethodNotFound), Message: rpctypes.CodeMethodNotFound.String(), Data: "method broadcast_evidence not found"}
		unsupportedFixtures.Unsupported = map[string]rpctypes.RPCError{"broadcast_evidence": want}
		client, err := jsonclient.New(server.URL)
		require.NoError(t, err)
		r, err := newHooksRunner(client, unsupportedFixtures)
		require.NoError(t, err)
		h := repairM1151Hooks(t, r)
		original := repairM1151Case(t, "unsupported", "/broadcast_evidence?evidence=dummy")
		var all []*transaction.Transaction
		require.NoError(t, h.Call("HooksRunner.RunBeforeAll", []*transaction.Transaction{&original}, &all))
		var tx transaction.Transaction
		require.NoError(t, h.Call("HooksRunner.RunBeforeEach", original, &tx))
		failed := repairM1151Execute(t, server.URL, tx)
		var validated transaction.Transaction
		require.NoError(t, h.Call("HooksRunner.RunBeforeEachValidation", failed, &validated))
		require.False(t, validated.Skip)
		require.Equal(t, original.Expected, validated.Expected)
		var got rpctypes.RPCError
		require.NoError(t, json.Unmarshal([]byte(validated.Real.Body), &got))
		require.Equal(t, want, got)
		// A supported response must not pass the unsupported assertion.
		success := repairM1151Execute(t, remote, tx)
		require.Error(t, h.Call("HooksRunner.RunBeforeEachValidation", success, &validated))
		// Nor may an unrelated error satisfy it.
		unexpected := failed
		real := *unexpected.Real
		unexpected.Real = &real
		unexpected.Real.Body = emitted[8].Real.Body
		require.Error(t, h.Call("HooksRunner.RunBeforeEachValidation", unexpected, &validated))
	})
	t.Run("failed-seed-does-not-enable-contract-cases", func(t *testing.T) {
		invalid := fixtures
		invalid.Seed = types.Tx("val:invalid")
		r, err := newHooksRunner(rpcClient, invalid)
		require.NoError(t, err)
		original := repairM1151Case(t, "seed-fails", "/tx")
		var reply []*transaction.Transaction
		require.Error(t, r.RunBeforeAll([]*transaction.Transaction{&original}, &reply))
		var tx transaction.Transaction
		require.Error(t, r.RunBeforeEach(original, &tx))
	})
}

func TestRepairM_1151_SetupValidationAndNoNameBasedSkips(t *testing.T) {
	client, err := jsonclient.New("http://127.0.0.1:1")
	require.NoError(t, err)
	fixtures := Fixtures{Seed: types.Tx("seed=value")}
	_, err = newHooksRunner(client, Fixtures{})
	require.Error(t, err)
	duplicate := fixtures
	duplicate.Transactions = []types.Tx{fixtures.Seed}
	_, err = newHooksRunner(client, duplicate)
	require.Error(t, err)
	invalidError := fixtures
	invalidError.Unsupported = map[string]rpctypes.RPCError{"tx": {Code: int(rpctypes.CodeInvalidParams), Message: rpctypes.CodeInvalidParams.String()}}
	_, err = newHooksRunner(client, invalidError)
	require.Error(t, err)
	for _, path := range []string{"/broadcast_tx", "/abci_query", "/broadcast_evidence"} {
		t.Run(path, func(t *testing.T) {
			r, err := newHooksRunner(client, fixtures)
			require.NoError(t, err)
			original := repairM1151Case(t, path, path)
			var all []*transaction.Transaction
			require.Error(t, r.RunBeforeAll([]*transaction.Transaction{&original}, &all))
			require.False(t, original.Skip)
		})
	}
	r, err := newHooksRunner(client, fixtures)
	require.NoError(t, err)
	for _, name := range []string{"Tx > /health", "Info > /tx misleading name", "Info > /broadcast_evidence", "ABCI > /abci_query"} {
		original := repairM1151Case(t, name, "/health")
		original.Name = name
		var reply transaction.Transaction
		require.NoError(t, r.RunBeforeEach(original, &reply))
		require.True(t, reflect.DeepEqual(original, reply))
		require.False(t, reply.Skip)
	}
	original := repairM1151Case(t, "not-prepared", "/tx")
	var reply transaction.Transaction
	require.Error(t, r.RunBeforeEach(original, &reply))
	require.False(t, reply.Skip)
	original.Skip = true
	var all []*transaction.Transaction
	require.Error(t, r.RunBeforeAll([]*transaction.Transaction{&original}, &all))
}
