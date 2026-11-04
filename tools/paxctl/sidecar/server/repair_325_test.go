package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clienttx "github.com/sei-protocol/sei-chain/sei-cosmos/client/tx"
	"github.com/sei-protocol/sei-chain/sei-cosmos/codec"
	codectypes "github.com/sei-protocol/sei-chain/sei-cosmos/codec/types"
	cryptocodec "github.com/sei-protocol/sei-chain/sei-cosmos/crypto/codec"
	"github.com/sei-protocol/sei-chain/sei-cosmos/crypto/hd"
	"github.com/sei-protocol/sei-chain/sei-cosmos/crypto/keyring"
	sdk "github.com/sei-protocol/sei-chain/sei-cosmos/types"
	signingtypes "github.com/sei-protocol/sei-chain/sei-cosmos/types/tx/signing"
	authtx "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/tx"
	govtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/gov/types"

	sidecar "github.com/sei-protocol/seictl/sidecar/client"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/tasks"
	"github.com/sei-protocol/seictl/sidecar/wire"
)

func TestRepair_325_UnsafeDefaultAuth(t *testing.T) {
	t.Setenv("SEI_SIDECAR_AUTHN_MODE", "")
	t.Setenv("SEI_SIDECAR_ISOLATED_DEVELOPMENT", "")
	mode, err := AuthnMode()
	if err != nil || mode != AuthnModeTrustedHeader {
		t.Fatalf("default authentication = %q, %v", mode, err)
	}
	if addr := BindAddress("7777", mode); addr != "127.0.0.1:7777" {
		t.Fatalf("default listener = %q", addr)
	}
	if addr := BindAddress("7777", "unrecognized"); addr != "127.0.0.1:7777" {
		t.Fatalf("unrecognized mode exposed listener: %q", addr)
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(home, "data", "chain-state")
	if err := os.WriteFile(sentinel, []byte("retain"), 0600); err != nil {
		t.Fatal(err)
	}
	kr, err := OpenKeyring(BackendTest, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := kr.NewMnemonic("operator", keyring.English, sdk.GetConfig().GetFullBIP44Path(), "", hd.Secp256k1); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(t, map[engine.TaskType]engine.TaskHandler{
		engine.TaskResetData: tasks.NewResetDataer(home).Handler(),
		engine.TaskGovVote:   tasks.NewGovVoter(engine.ExecutionConfig{Keyring: kr}).Handler(),
		engine.TaskMarkReady: tasks.MarkReadyHandler(),
	})
	request := sidecar.GovVoteTask{ChainID: "repair-325", KeyName: "operator", ProposalID: 9007199254740993, Option: "yes", Fees: "1usei", Gas: 9007199254740993}
	for _, authMode := range []string{mode, ""} {
		srv := NewServer(BindAddress("0", mode), eng, home, authMode)
		httpServer := httptest.NewServer(srv.handler)
		client, err := sidecar.NewClientWithResponses(httpServer.URL)
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range []sidecar.TaskRequest{request.ToTaskRequest(), (sidecar.ResetDataTask{}).ToTaskRequest()} {
			resp, err := client.SubmitTaskWithResponse(context.Background(), task)
			if err != nil || resp.StatusCode() != http.StatusUnauthorized {
				t.Fatalf("unauthorized %s: response=%+v err=%v", task.Type, resp, err)
			}
		}
		httpServer.Close()
	}
	srv := NewServer("127.0.0.1:0", eng, home, mode)
	for _, header := range [][]string{nil, {""}, {"   "}, {"attacker", "operator"}} {
		req := httptest.NewRequest(http.MethodPost, "/v0/tasks", strings.NewReader(`{"type":"mark-ready"}`))
		req.Header[remoteUserHeader] = header
		rec := httptest.NewRecorder()
		srv.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("headers %q: status %d", header, rec.Code)
		}
	}
	if got := eng.RecentResults(); len(got) != 0 {
		t.Fatalf("unauthorized calls persisted tasks: %+v", got)
	}
	if rec := serveHTTP(srv, http.MethodPost, "/v0/tasks", `{"type":"mark-ready"}`); rec.Code != http.StatusCreated {
		t.Fatalf("authorized real handler: %d %s", rec.Code, rec.Body.String())
	}
	waitForReady(eng)
	if !eng.Healthz() {
		t.Fatal("authorized mark-ready did not execute")
	}
	for _, address := range []string{":7777", "0.0.0.0:7777", "[::]:7777", "192.0.2.1:7777", "localhost:7777"} {
		srv := NewServer(address, eng, home, mode)
		if err := srv.ListenAndServe(context.Background()); err == nil {
			t.Fatalf("trusted headers allowed on unprotected listener %q", address)
		}
		req := httptest.NewRequest(http.MethodPost, "/v0/tasks", strings.NewReader(`{"type":"reset-data"}`))
		req.Header.Set(remoteUserHeader, "forged")
		rec := httptest.NewRecorder()
		srv.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("unprotected handler %q: status %d", address, rec.Code)
		}
	}
	t.Setenv("SEI_SIDECAR_AUTHN_MODE", "unauthenticated")
	if _, err := AuthnMode(); err == nil {
		t.Fatal("unauthenticated mode accepted without isolated-development override")
	}
	srv = NewServer(":0", eng, home, AuthnModeUnauthenticated)
	if err := srv.ListenAndServe(context.Background()); err == nil {
		t.Fatal("constructor bypassed isolated-development override")
	}
	t.Setenv("SEI_SIDECAR_ISOLATED_DEVELOPMENT", "true")
	devMode, err := AuthnMode()
	if err != nil || devMode != AuthnModeUnauthenticated {
		t.Fatalf("explicit isolated development: %q, %v", devMode, err)
	}
	srv = NewServer(BindAddress("0", devMode), eng, home, devMode)
	for _, body := range []string{`{"type":"gov-vote"}`, `{"type":"reset-data"}`, `{"type":"mark-ready"}`} {
		if rec := serveHTTP(srv, http.MethodPost, "/v0/tasks", body); rec.Code != http.StatusForbidden {
			t.Fatalf("unprotected task accepted: %d %s", rec.Code, rec.Body.String())
		}
	}
	if rec := serveHTTP(srv, http.MethodDelete, "/v0/tasks/00000000-0000-0000-0000-000000000001", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("unprotected delete accepted: %d", rec.Code)
	}
	if rec := serveHTTP(srv, http.MethodGet, "/v0/livez", ""); rec.Code != http.StatusOK {
		t.Fatalf("isolated-development liveness: %d", rec.Code)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "retain" {
		t.Fatalf("unauthorized request changed chain state: %q, %v", data, err)
	}
	t.Setenv("SEI_SIDECAR_AUTHN_MODE", "trusted_header")
	if _, err := AuthnMode(); err == nil {
		t.Fatal("invalid mode did not fail closed")
	}
}

// Drive a typed producer over HTTP into the real engine, stop its handler
// after consumption, reopen SQLite, and let the engine recover the same task.
func repair325Recover(t *testing.T, request sidecar.TaskRequest, consumer engine.TaskHandler) (json.RawMessage, json.RawMessage) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "sidecar.db")
	store, err := NewTaskStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	consumed := make(chan json.RawMessage, 1)
	stopped := make(chan struct{})
	handler := func(ctx context.Context, params map[string]any) (json.RawMessage, error) {
		defer close(stopped)
		raw, err := consumer(ctx, params)
		if err != nil {
			return nil, err
		}
		consumed <- raw
		<-ctx.Done()
		return nil, ctx.Err()
	}
	eng := engine.NewEngine(ctx, map[engine.TaskType]engine.TaskHandler{engine.TaskType(request.Type): handler}, store)
	eng.Config.Checkpointer = store
	srv := NewServer("127.0.0.1:0", eng, t.TempDir(), AuthnModeTrustedHeader)
	httpServer := httptest.NewServer(srv.handler)
	t.Cleanup(httpServer.Close)
	client, err := sidecar.NewClientWithResponses(httpServer.URL, sidecar.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set(remoteUserHeader, "operator")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.SubmitTaskWithResponse(ctx, request)
	if err != nil || resp.StatusCode() != http.StatusCreated || resp.JSON201 == nil {
		t.Fatalf("typed submission: response=%+v err=%v", resp, err)
	}
	id := resp.JSON201.Id.String()
	var first json.RawMessage
	select {
	case first = <-consumed:
	case <-stopped:
		t.Fatalf("consumer failed: %+v", waitForTaskResult(eng, id))
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not receive the HTTP request")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not stop")
	}
	httpServer.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewTaskStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	row, err := reopened.Get(id)
	if err != nil || row == nil || row.Status != engine.TaskStatusRunning {
		t.Fatalf("recoverable row: %+v, %v", row, err)
	}
	for _, read := range []func() ([]engine.TaskResult, error){
		func() ([]engine.TaskResult, error) { return reopened.List(100) },
		reopened.ListStaleTasks,
	} {
		rows, err := read()
		if err != nil || len(rows) != 1 {
			t.Fatalf("reopened list: %+v, %v", rows, err)
		}
		want, _ := json.Marshal(row.Params)
		got, err := json.Marshal(rows[0].Params)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("list changed params: %s != %s (%v)", got, want, err)
		}
	}
	latest, err := reopened.LatestByType(request.Type)
	if err != nil || latest == nil || latest.ID != id {
		t.Fatalf("latest recovered task: %+v, %v", latest, err)
	}
	want, _ := json.Marshal(row.Params)
	got, err := json.Marshal(latest.Params)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("latest changed params: %s != %s (%v)", got, want, err)
	}
	recoveryCtx, stopRecovery := context.WithCancel(context.Background())
	t.Cleanup(stopRecovery)
	recovery := engine.NewEngine(recoveryCtx, map[engine.TaskType]engine.TaskHandler{engine.TaskType(request.Type): consumer}, reopened)
	recovery.RehydrateStaleTasks()
	result := waitForTaskResult(recovery, id)
	if result == nil || result.Status != engine.TaskStatusCompleted {
		t.Fatalf("recovered consumer failed: %+v", result)
	}
	if result.Run != row.Run {
		t.Fatalf("recovery changed run: %d != %d", result.Run, row.Run)
	}
	return first, result.Result
}

func TestRepair_325_TaskParamNumberPrecision(t *testing.T) {
	kr, err := OpenKeyring(BackendTest, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	info, _, err := kr.NewMnemonic("operator", keyring.English, sdk.GetConfig().GetFullBIP44Path(), "", hd.Secp256k1)
	if err != nil {
		t.Fatal(err)
	}
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	govtypes.RegisterInterfaces(registry)
	txConfig := authtx.NewTxConfig(codec.NewProtoCodec(registry), authtx.DefaultSignModes)
	consumer := engine.TypedHandlerWithResult(func(_ context.Context, params tasks.GovVoteRequest) ([]byte, error) {
		option, err := wire.ParseVoteOption(params.Option)
		if err != nil {
			return nil, err
		}
		msg := govtypes.NewMsgVote(info.GetAddress(), params.ProposalID, govtypes.VoteOption(option))
		factory := clienttx.Factory{}.
			WithChainID(params.ChainID).
			WithKeybase(kr).
			WithTxConfig(txConfig).
			WithAccountNumber(17).
			WithSequence(42).
			WithGas(params.Gas).
			WithFees(params.Fees).
			WithMemo(params.Memo).
			WithSignMode(signingtypes.SignMode_SIGN_MODE_DIRECT)
		builder, err := clienttx.BuildUnsignedTx(factory, msg)
		if err != nil {
			return nil, err
		}
		if err := clienttx.Sign(factory, params.KeyName, builder, true); err != nil {
			return nil, err
		}
		return txConfig.TxEncoder()(builder.GetTx())
	})
	for _, boundary := range []uint64{1<<53 - 1, 1 << 53, 1<<53 + 1, math.MaxUint64 - 1, math.MaxUint64} {
		t.Run(fmt.Sprint(boundary), func(t *testing.T) {
			task := sidecar.GovVoteTask{ChainID: "repair-325", KeyName: "operator", ProposalID: boundary, Option: "yes", Fees: "1usei", Gas: boundary, Memo: "exact JSON"}
			if err := task.Validate(); err != nil {
				t.Fatal(err)
			}
			first, recovered := repair325Recover(t, task.ToTaskRequest(), consumer)
			for _, raw := range []json.RawMessage{first, recovered} {
				var signed []byte
				if err := json.Unmarshal(raw, &signed); err != nil {
					t.Fatal(err)
				}
				decoded, err := txConfig.TxDecoder()(signed)
				if err != nil {
					t.Fatal(err)
				}
				feeTx, ok := decoded.(sdk.FeeTx)
				if !ok || feeTx.GetGas() != boundary {
					t.Fatalf("signed gas changed: %+v, expected %d", decoded, boundary)
				}
				if len(decoded.GetMsgs()) != 1 {
					t.Fatalf("signed message count = %d", len(decoded.GetMsgs()))
				}
				vote, ok := decoded.GetMsgs()[0].(*govtypes.MsgVote)
				if !ok || vote.ProposalId != boundary || vote.Voter != info.GetAddress().String() {
					t.Fatalf("signed vote changed: %+v, expected proposal %d", vote, boundary)
				}
			}
			if !bytes.Equal(first, recovered) {
				t.Fatal("recovery changed deterministic signed transaction bytes")
			}
		})
	}
}

func TestRepair_325_GenesisOverridePrecision(t *testing.T) {
	overrides := map[string]json.RawMessage{
		"staking.params.max_validators": json.RawMessage(`9007199254740993`),
		"gov.boundary":                  json.RawMessage(`18446744073709551615`),
		"gov.nested":                    json.RawMessage(`{"z":[9007199254740993,18446744073709551615,1.0000000000000000001],"a":1e100}`),
		"gov.literal":                   json.RawMessage(`"9007199254740993"`),
	}
	task := sidecar.AssembleAndUploadGenesisTask{AccountBalance: "1usei", Namespace: "repair-325", Nodes: []sidecar.GenesisNodeParam{{Name: "validator"}}, Overrides: overrides}
	if err := task.Validate(); err != nil {
		t.Fatal(err)
	}
	consumer := engine.TypedHandlerWithResult(func(_ context.Context, params tasks.AssembleGenesisRequest) (map[string]json.RawMessage, error) {
		return params.Overrides, nil
	})
	first, recovered := repair325Recover(t, task.ToTaskRequest(), consumer)
	for _, raw := range []json.RawMessage{first, recovered} {
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != len(overrides) {
			t.Fatalf("override count: %d != %d", len(got), len(overrides))
		}
		for key, expected := range overrides {
			if !bytes.Equal(got[key], expected) {
				t.Fatalf("override %s changed: %s != %s", key, got[key], expected)
			}
		}
	}
}

func TestRepair_325_RejectInvalidTaskJSON(t *testing.T) {
	eng := newTestEngine(t, map[engine.TaskType]engine.TaskHandler{engine.TaskMarkReady: tasks.MarkReadyHandler()})
	srv := NewServer("127.0.0.1:0", eng, t.TempDir(), AuthnModeTrustedHeader)
	for _, body := range []string{
		`{"type":"mark-ready"} {"type":"reset-data"}`,
		`{"type":"mark-ready"} trailing`,
		`{"type":"mark-ready","params":9007199254740993}`,
		`{"type":"mark-ready","params":{"number":18446744073709551615x}}`,
	} {
		if rec := serveHTTP(srv, http.MethodPost, "/v0/tasks", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid JSON accepted: %s, status=%d", body, rec.Code)
		}
	}
	if results := eng.RecentResults(); len(results) != 0 {
		t.Fatalf("invalid requests persisted tasks: %+v", results)
	}
}

func TestRepair_325_TaskStoreEnvelope(t *testing.T) {
	store, err := NewTaskStore(filepath.Join(t.TempDir(), "sidecar.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for i, params := range []map[string]any{nil, {}, {exactParamsKey: "user value", "nested": map[string]any{"number": json.Number("18446744073709551615")}}} {
		r := &engine.TaskResult{ID: fmt.Sprint(i), Type: "repair-325", Status: engine.TaskStatusRunning, Params: params, SubmittedAt: time.Now().UTC()}
		before, _ := json.Marshal(r.Params)
		if err := store.Save(r); err != nil {
			t.Fatal(err)
		}
		after, _ := json.Marshal(r.Params)
		if !bytes.Equal(before, after) {
			t.Fatal("Save modified the caller's params")
		}
		got, err := store.Get(r.ID)
		if err != nil || got == nil {
			t.Fatalf("Get: %+v, %v", got, err)
		}
		encoded, _ := json.Marshal(got.Params)
		if !bytes.Equal(before, encoded) {
			t.Fatalf("params changed: %s != %s", before, encoded)
		}
	}
	if got, err := store.Get("absent"); err != nil || got != nil {
		t.Fatalf("absent row: %+v, %v", got, err)
	}
	if got, err := store.LatestByType("absent"); err != nil || got != nil {
		t.Fatalf("absent latest row: %+v, %v", got, err)
	}
	bad := &engine.TaskResult{ID: "bad", Type: "bad", Status: engine.TaskStatusRunning, SubmittedAt: time.Now().UTC(), Params: map[string]any{"nested": []any{json.Number("9007199254740993")}}}
	if err := store.SQLiteStore.Save(bad); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("bad"); err == nil || !strings.Contains(err.Error(), "migration required") {
		t.Fatalf("legacy numeric params did not fail closed: %v", err)
	}
	bad.Params = map[string]any{exactParamsKey: 1}
	if err := store.SQLiteStore.Save(bad); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("bad"); err == nil {
		t.Fatal("corrupt envelope accepted")
	}
	bad.Params = map[string]any{exactParamsKey: "{malformed"}
	if err := store.SQLiteStore.Save(bad); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("bad"); err == nil {
		t.Fatal("malformed envelope accepted")
	}
	if deleted, err := store.Delete("bad"); err != nil || !deleted {
		t.Fatalf("delete: %v, %v", deleted, err)
	}
	if n, err := store.DeleteByType("repair-325"); err != nil || n != 3 {
		t.Fatalf("delete by type: %d, %v", n, err)
	}
}
