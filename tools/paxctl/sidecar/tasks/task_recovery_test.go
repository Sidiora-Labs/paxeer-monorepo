package tasks

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/sei-protocol/sei-chain/sei-cosmos/crypto/keyring"
	sdk "github.com/sei-protocol/sei-chain/sei-cosmos/types"
	authsigning "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/signing"
	authtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/types"
	vestingtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/vesting/types"
	banktypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/bank/types"
	"github.com/sei-protocol/sei-chain/sei-cosmos/x/genutil"
	genutiltypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/genutil/types"
	stakingtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/staking/types"
	tmtypes "github.com/sei-protocol/sei-chain/sei-tendermint/types"
	seiconfig "github.com/sei-protocol/sei-config"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/rpc"
	seis3 "github.com/sei-protocol/seictl/sidecar/s3"
)

const taskRecoveryChain = "repair-m836"

func taskRecoveryRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func taskRecoveryWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func taskRecoveryParams(value any) map[string]any {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var params map[string]any
	if err := json.Unmarshal(data, &params); err != nil {
		panic(err)
	}
	return params
}

// Real init, codec and module-default genesis, before the gentx producer runs.
func taskRecoveryInit(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if _, err := NewIdentityGenerator(home).Handler()(context.Background(), map[string]any{"chainId": taskRecoveryChain, "moniker": "validator"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "config", "genesis.json")
	doc, err := tmtypes.GenesisDocFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cdc, _ := makeCodec()
	staking := stakingtypes.DefaultGenesisState()
	staking.Params.BondDenom = feeDenom
	state := map[string]json.RawMessage{
		authtypes.ModuleName:    cdc.MustMarshalJSON(authtypes.DefaultGenesisState()),
		banktypes.ModuleName:    cdc.MustMarshalJSON(banktypes.DefaultGenesisState()),
		stakingtypes.ModuleName: cdc.MustMarshalJSON(staking),
		genutiltypes.ModuleName: cdc.MustMarshalJSON(genutiltypes.DefaultGenesisState()),
	}
	doc.AppState, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := genutil.ExportGenesisFile(doc, path); err != nil {
		t.Fatal(err)
	}
	return home
}

func taskRecoveryGentxRequest() GenerateGentxRequest {
	return GenerateGentxRequest{ChainID: taskRecoveryChain, StakingAmount: "1000000uhpx", AccountBalance: "100000000uhpx"}
}

func taskRecoveryKey(t *testing.T, home string) keyring.Info {
	t.Helper()
	kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, home, nil)
	if err != nil {
		t.Fatal(err)
	}
	key, err := kb.Key(validatorKeyName)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func taskRecoveryCheckGentx(t *testing.T, home string, op gentxOperation) {
	t.Helper()
	data := taskRecoveryRead(t, filepath.Join(home, "config", "gentx", op.GentxName))
	if !bytes.Equal(data, op.Signed) {
		t.Fatal("signed gentx bytes changed across reopen")
	}
	_, txCfg := makeCodec()
	tx, err := txCfg.TxJSONDecoder()(data)
	if err != nil {
		t.Fatal(err)
	}
	signed, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		t.Fatal("gentx is not a signed transaction")
	}
	signatures, err := signed.GetSignaturesV2()
	if err != nil || len(signatures) != 1 {
		t.Fatalf("signatures: %v %v", signatures, err)
	}
	if !signatures[0].PubKey.Equals(taskRecoveryKey(t, home).GetPubKey()) {
		t.Fatal("gentx signer differs from durable key")
	}
	if err := authsigning.VerifySignature(signatures[0].PubKey, authsigning.SignerData{ChainID: taskRecoveryChain}, signatures[0].Data, txCfg.SignModeHandler(), tx); err != nil {
		t.Fatalf("invalid recovered gentx signature: %v", err)
	}
	msgs := tx.GetMsgs()
	if len(msgs) != 1 {
		t.Fatal("wrong message count")
	}
	msg, ok := msgs[0].(*stakingtypes.MsgCreateValidator)
	if !ok || msg.DelegatorAddress != op.Address || msg.Value.String() != taskRecoveryGentxRequest().StakingAmount {
		t.Fatalf("gentx request changed: %+v", msg)
	}
	cdc, _ := makeCodec()
	state, _, err := genutiltypes.GenesisStateFromGenFile(filepath.Join(home, "config", "genesis.json"))
	if err != nil {
		t.Fatal(err)
	}
	auth := authtypes.GetGenesisStateFromAppState(cdc, state)
	accounts, err := authtypes.UnpackAccounts(auth.Accounts)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, acc := range accounts {
		if acc.GetAddress().String() == op.Address {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("recovered account count=%d", count)
	}
	bank := banktypes.GetGenesisStateFromAppState(cdc, state)
	count = 0
	for _, balance := range bank.Balances {
		if balance.Address == op.Address {
			count++
			if balance.Coins.String() != taskRecoveryGentxRequest().AccountBalance {
				t.Fatal("recovered balance changed")
			}
		}
	}
	if count != 1 {
		t.Fatalf("recovered balance count=%d", count)
	}
}

func TestTaskRecoveryGentxCrashResumable(t *testing.T) {
	for _, boundary := range []string{"key", "account", "signed"} {
		t.Run(boundary, func(t *testing.T) {
			home := taskRecoveryInit(t)
			request := taskRecoveryGentxRequest()
			input := taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))
			var unblock func()
			switch boundary {
			case "key":
				path := filepath.Join(home, gentxStageDir)
				taskRecoveryWrite(t, path, []byte("interrupt before account stage"))
				unblock = func() {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			case "account":
				path := filepath.Join(home, "config", "node_key.json")
				saved := taskRecoveryRead(t, path)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				unblock = func() {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					taskRecoveryWrite(t, path, saved)
				}
			case "signed":
				// Bind the operation at the key boundary, then interrupt the
				// final completion write after the signed gentx is durable.
				stageBlocker := filepath.Join(home, gentxStageDir)
				taskRecoveryWrite(t, stageBlocker, []byte("bind operation before completion interruption"))
				if _, err := NewGentxGenerator(home).Handler()(context.Background(), taskRecoveryParams(request)); err == nil {
					t.Fatal("expected key-boundary interruption")
				}
				if err := os.Remove(stageBlocker); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(home, gentxMarkerFile)
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				unblock = func() {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			}
			g := NewGentxGenerator(home)
			if _, err := g.Handler()(context.Background(), taskRecoveryParams(request)); err == nil {
				t.Fatal("interrupted ceremony reported success")
			}
			var before gentxOperation
			if err := json.Unmarshal(taskRecoveryRead(t, filepath.Join(home, gentxOperationFile)), &before); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(filepath.Join(home, gentxOperationFile))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("private journal permissions: %v %v", info, err)
			}
			key := taskRecoveryKey(t, home)
			if key.GetAddress().String() != before.Address {
				t.Fatal("key was not bound before creation")
			}
			if boundary == "key" && (!bytes.Equal(input, taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))) || len(before.Account) != 0) {
				t.Fatal("key boundary modified genesis")
			}
			if boundary == "account" && (!bytes.Equal(before.Account, taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))) || len(before.Signed) != 0) {
				t.Fatal("account boundary not reached")
			}
			if boundary == "signed" {
				taskRecoveryCheckGentx(t, home, before)
			}
			// Stop using this instance, remove only the injected failing peer,
			// and reopen all state through a fresh production handler/keyring.
			g = nil
			unblock()
			g = NewGentxGenerator(home)
			if _, err := g.Handler()(context.Background(), taskRecoveryParams(request)); err != nil {
				t.Fatal(err)
			}
			var after gentxOperation
			if err := json.Unmarshal(taskRecoveryRead(t, filepath.Join(home, gentxOperationFile)), &after); err != nil {
				t.Fatal(err)
			}
			if after.Address != before.Address || !taskRecoveryKey(t, home).GetPubKey().Equals(key.GetPubKey()) {
				t.Fatal("restart replaced the signing identity")
			}
			if boundary == "signed" && !bytes.Equal(before.Signed, after.Signed) {
				t.Fatal("restart signed a different transaction")
			}
			taskRecoveryCheckGentx(t, home, after)
			if !markerExists(home, gentxMarkerFile) {
				t.Fatal("completion was not durably recorded")
			}
			changed := request
			changed.AccountBalance = "200000000uhpx"
			if _, err := NewGentxGenerator(home).Handler()(context.Background(), taskRecoveryParams(changed)); err == nil {
				t.Fatal("completion adopted a different request")
			}
			if _, err := NewGentxGenerator(home).Handler()(context.Background(), taskRecoveryParams(request)); err != nil {
				t.Fatal(err)
			}
			taskRecoveryCheckGentx(t, home, after)
		})
	}
}

func TestTaskRecoveryGentxRejectsUnboundState(t *testing.T) {
	home := taskRecoveryInit(t)
	cdc, _ := makeCodec()
	if _, err := NewGentxGenerator(home).addValidatorKey(cdc); err != nil {
		t.Fatal(err)
	}
	key := taskRecoveryKey(t, home)
	if _, err := NewGentxGenerator(home).Handler()(context.Background(), taskRecoveryParams(taskRecoveryGentxRequest())); err == nil || !strings.Contains(err.Error(), "unbound") {
		t.Fatalf("unbound key adopted: %v", err)
	}
	if !taskRecoveryKey(t, home).GetPubKey().Equals(key.GetPubKey()) {
		t.Fatal("unbound key replaced")
	}

	home = taskRecoveryInit(t)
	blocker := filepath.Join(home, gentxStageDir)
	taskRecoveryWrite(t, blocker, []byte("stop after key"))
	if _, err := NewGentxGenerator(home).Handler()(context.Background(), taskRecoveryParams(taskRecoveryGentxRequest())); err == nil {
		t.Fatal("expected interruption")
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	address := taskRecoveryKey(t, home).GetAddress().String()
	if err := NewGentxGenerator(home).addGenesisAccount(cdc, address, "3uhpx"); err != nil {
		t.Fatal(err)
	}
	conflicting := taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))
	if _, err := NewGentxGenerator(home).Handler()(context.Background(), taskRecoveryParams(taskRecoveryGentxRequest())); err == nil {
		t.Fatal("conflicting account adopted")
	}
	if !bytes.Equal(conflicting, taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))) {
		t.Fatal("conflicting account overwritten")
	}
}

// The storage peer commits actual HTTP uploads to files before acknowledging.
// Both sides use the production AWS codec, HTTP transport and transfer manager.
type taskRecoveryObjects struct {
	root     string
	mu       sync.Mutex
	reject   map[string]bool
	afterPut func(string) error
	puts     []string
}

func taskRecoveryTransport(t *testing.T) (*taskRecoveryObjects, S3ClientFactory, seis3.UploaderFactory) {
	t.Helper()
	objects := &taskRecoveryObjects{root: t.TempDir(), reject: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if !filepath.IsLocal(name) {
			http.Error(w, "invalid object key", 400)
			return
		}
		objects.mu.Lock()
		rejected := objects.reject[r.Method+" "+r.URL.Path]
		after := objects.afterPut
		objects.mu.Unlock()
		if rejected {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(503)
			_, _ = io.WriteString(w, "<Error><Code>ServiceUnavailable</Code><Message>interrupted storage peer</Message></Error>")
			return
		}
		path := filepath.Join(objects.root, name)
		if r.Method != http.MethodPut {
			http.FileServer(http.Dir(objects.root)).ServeHTTP(w, r)
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		f, err := os.Create(path)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, err = io.Copy(f, r.Body)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			http.Error(w, "durable write failed", 500)
			return
		}
		if after != nil {
			if err := after(path); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		objects.mu.Lock()
		objects.puts = append(objects.puts, path)
		objects.mu.Unlock()
		w.Header().Set("ETag", `"durable-local-object"`)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	client := awss3.NewFromConfig(aws.Config{Region: "local", Credentials: aws.AnonymousCredentials{}, HTTPClient: srv.Client()}, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
		o.UsePathStyle = true
		o.RetryMaxAttempts = 1
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return objects, func(context.Context, string) (S3GetObjectAPI, error) { return client, nil }, func(context.Context, string) (seis3.Uploader, error) { return transfermanager.New(client), nil }
}

func (o *taskRecoveryObjects) refuse(method, path string, value bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reject[method+" "+path] = value
}

func TestTaskRecoveryExternalAccountsAssemblyRetry(t *testing.T) {
	ctx := context.Background()
	producer := taskRecoveryInit(t)
	if _, err := NewGentxGenerator(producer).Handler()(ctx, taskRecoveryParams(taskRecoveryGentxRequest())); err != nil {
		t.Fatal(err)
	}
	objects, download, upload := taskRecoveryTransport(t)
	if _, err := NewGenesisArtifactUploader(producer, "ceremony", "local", taskRecoveryChain, upload).Handler()(ctx, map[string]any{"nodeName": "val-0"}); err != nil {
		t.Fatal(err)
	}
	home := taskRecoveryInit(t)
	input := taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))
	external := randomSeiAddr(t)
	request := AssembleGenesisRequest{AccountBalance: "100000000uhpx", Namespace: "ceremony", Nodes: []AssembleNodeEntry{{Name: "val-0"}}, Accounts: []GenesisAccountEntry{{Address: external, Balance: "123uhpx", Vesting: &GenesisAccountVesting{Amount: "100uhpx", EndTime: 4102444800}}}}
	// Interrupt collection after external accounts have been written to the
	// stage. The immutable source must remain byte-for-byte unchanged.
	blocker := filepath.Join(home, assemblyStageDir, "config", "node_key.json")
	if err := os.MkdirAll(blocker, 0700); err != nil {
		t.Fatal(err)
	}
	a := NewGenesisAssembler(home, "ceremony", "local", taskRecoveryChain, download, upload)
	if _, err := a.Handler()(ctx, taskRecoveryParams(request)); err == nil {
		t.Fatal("expected interrupted collection")
	}
	staged := filepath.Join(home, assemblyStageDir, "config", "genesis.json")
	if findAuthAccount(t, staged, external) == nil {
		t.Fatal("interruption did not reach external-account write")
	}
	if !bytes.Equal(input, taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))) {
		t.Fatal("incomplete assembly mutated immutable input")
	}
	a = nil
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	// The next attempt rebuilds from input, reaches durable staging, then
	// loses the second upload. A subsequent restart must adopt that stage.
	objects.refuse("PUT", "/ceremony/"+taskRecoveryChain+"/peers.json", true)
	a = NewGenesisAssembler(home, "ceremony", "local", taskRecoveryChain, download, upload)
	if _, err := a.Handler()(ctx, taskRecoveryParams(request)); err == nil {
		t.Fatal("expected interrupted peers upload")
	}
	stageBytes := taskRecoveryRead(t, staged)
	var operation assemblyOperation
	if err := json.Unmarshal(taskRecoveryRead(t, filepath.Join(home, assemblyOperationFile)), &operation); err != nil {
		t.Fatal(err)
	}
	if operation.StageHash != recoveryHash(stageBytes) {
		t.Fatal("upload started before durable stage identity")
	}
	if !bytes.Equal(input, taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))) {
		t.Fatal("failed upload published local genesis")
	}
	a = nil
	objects.refuse("PUT", "/ceremony/"+taskRecoveryChain+"/peers.json", false)
	objects.refuse("GET", "/ceremony/"+taskRecoveryChain+"/val-0/gentx.json", true)
	a = NewGenesisAssembler(home, "ceremony", "local", taskRecoveryChain, download, upload)
	result, err := a.Handler()(ctx, taskRecoveryParams(request))
	if err != nil {
		t.Fatal(err)
	}
	var outcome AssembleGenesisResult
	if err := json.Unmarshal(result, &outcome); err != nil {
		t.Fatal(err)
	}
	installed := taskRecoveryRead(t, filepath.Join(home, "config", "genesis.json"))
	uploaded := taskRecoveryRead(t, filepath.Join(objects.root, "ceremony", taskRecoveryChain, "genesis.json"))
	if !bytes.Equal(stageBytes, installed) || !bytes.Equal(installed, uploaded) || outcome.GenesisHash != recoveryHash(uploaded) {
		t.Fatal("restart did not publish the exact bound stage")
	}
	account, ok := findAuthAccount(t, filepath.Join(home, "config", "genesis.json"), external).(*vestingtypes.ContinuousVestingAccount)
	if !ok || account.OriginalVesting.String() != "100uhpx" || account.EndTime != 4102444800 {
		t.Fatalf("recovered vesting schedule changed: %+v", account)
	}
	state, doc, err := genutiltypes.GenesisStateFromGenFile(filepath.Join(home, "config", "genesis.json"))
	if err != nil {
		t.Fatal(err)
	}
	cdc, _ := makeCodec()
	bank := banktypes.GetGenesisStateFromAppState(cdc, state)
	count := 0
	for _, balance := range bank.Balances {
		if balance.Address == external {
			count++
			if balance.Coins.String() != "123uhpx" {
				t.Fatal("recovered balance changed")
			}
		}
	}
	if count != 1 || len(doc.Validators) != 1 {
		t.Fatal("duplicate external account or missing collected validator")
	}
	replay, err := NewGenesisAssembler(home, "ceremony", "local", taskRecoveryChain, download, upload).Handler()(ctx, taskRecoveryParams(request))
	if err != nil || !bytes.Equal(result, replay) {
		t.Fatalf("completion recovery changed result: %s %v", replay, err)
	}
	request.Accounts[0].Balance = "999uhpx"
	if _, err := NewGenesisAssembler(home, "ceremony", "local", taskRecoveryChain, download, upload).Handler()(ctx, taskRecoveryParams(request)); err == nil {
		t.Fatal("different assembly adopted previous completion")
	}
	// The same real producer's validator account remains a genuine collision.
	collisionHome := taskRecoveryInit(t)
	objects.refuse("GET", "/ceremony/"+taskRecoveryChain+"/val-0/gentx.json", false)
	request.Accounts = []GenesisAccountEntry{{Address: taskRecoveryKey(t, producer).GetAddress().String(), Balance: "1uhpx"}}
	if _, err := NewGenesisAssembler(collisionHome, "ceremony", "local", taskRecoveryChain, download, upload).Handler()(ctx, taskRecoveryParams(request)); err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("genuine collision accepted: %v", err)
	}
}

func TestTaskRecoveryReloadRestartRequired(t *testing.T) {
	home := t.TempDir()
	writeDefaultConfig(t, home, seiconfig.ModeFull)
	handler := NewConfigReloader(home).Handler()
	_, err := handler(context.Background(), map[string]any{"fields": map[string]any{"logging.level": "debug"}})
	var outcome *engine.TaskError
	if !errors.As(err, &outcome) || outcome.Operation != "restart-required" || outcome.Retryable {
		t.Fatalf("disk-only reload reported applied: %v", err)
	}
	cfg, err := seiconfig.ReadConfigFromDir(home)
	if err != nil || cfg.Logging.Level != "debug" {
		t.Fatalf("valid settings were not staged: %v", err)
	}
	_, err = NewConfigReloader(home).Handler()(context.Background(), map[string]any{"fields": map[string]any{"logging.level": "debug"}})
	if !errors.As(err, &outcome) || outcome.Operation != "restart-required" {
		t.Fatal("retry hid the restart requirement")
	}
	if _, err := handler(context.Background(), map[string]any{"fields": map[string]any{"storage.db_backend": "rocksdb"}}); err == nil {
		t.Fatal("non-hot-reload field accepted")
	}
}

type taskRecoverySource struct {
	height       atomic.Int64
	unavailable  atomic.Bool
	blockFailure atomic.Bool
	rpcFailure   atomic.Bool
	server       *httptest.Server
}

func taskRecoveryRPC(t *testing.T, chain string, height int64) *taskRecoverySource {
	t.Helper()
	source := &taskRecoverySource{}
	source.height.Store(height)
	source.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if source.unavailable.Load() {
			http.Error(w, "source unavailable", 503)
			return
		}
		switch r.URL.Path {
		case "/status":
			status := rpc.StatusResult{NodeInfo: rpc.NodeInfo{Network: chain}, SyncInfo: rpc.SyncInfo{LatestBlockHeight: fmt.Sprint(source.height.Load())}}
			_ = json.NewEncoder(w).Encode(status)
		case "/block_results":
			if source.blockFailure.Load() {
				http.Error(w, "block results pruned", 500)
				return
			}
			if source.rpcFailure.Load() {
				_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":-1,"error":{"code":-32603,"message":"height pruned"}}`)
				return
			}
			// A recorded minimal committed-block response, with the queried
			// height carried through the actual RPC/archive producer path.
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":-1,"result":{"height":%q,"txs_results":[]}}`, r.URL.Query().Get("height"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(source.server.Close)
	return source
}

func taskRecoveryExportConfig(source *taskRecoverySource) ResultExportRequest {
	return ResultExportRequest{Bucket: "results", Prefix: "archive/", Region: "local", RPCEndpoint: source.server.URL}
}

func taskRecoveryCheckPage(t *testing.T, path string, start, end int64) {
	t.Helper()
	raw := taskRecoveryRead(t, path)
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	scanner := bufio.NewScanner(gz)
	height := start
	for scanner.Scan() {
		var record struct {
			Height  int64 `json:"height"`
			Results struct {
				Result struct {
					Height string `json:"height"`
				} `json:"result"`
			} `json:"block_results"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record.Height != height || record.Results.Result.Height != fmt.Sprint(height) {
			t.Fatalf("archive/source height mismatch at %d: %+v", height, record)
		}
		height++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if height != end+1 {
		t.Fatalf("page ended at %d, want %d", height-1, end)
	}
}

func TestTaskRecoveryExportDestinationBoundRecovery(t *testing.T) {
	source := taskRecoveryRPC(t, taskRecoveryChain, 1000)
	objects, _, upload := taskRecoveryTransport(t)
	home := t.TempDir()
	cfg := taskRecoveryExportConfig(source)
	exporter := NewResultExporter(home, taskRecoveryChain, "node", upload)
	if _, err := exporter.Handler()(context.Background(), taskRecoveryParams(cfg)); err != nil {
		t.Fatal(err)
	}
	taskRecoveryCheckPage(t, filepath.Join(objects.root, "results", "archive", "1-1000.ndjson.gz"), 1, 1000)
	exporter = nil
	source.height.Store(2000)
	exporter = NewResultExporter(home, taskRecoveryChain, "node", upload)
	if _, err := exporter.Handler()(context.Background(), taskRecoveryParams(cfg)); err != nil {
		t.Fatal(err)
	}
	taskRecoveryCheckPage(t, filepath.Join(objects.root, "results", "archive", "1001-2000.ndjson.gz"), 1001, 2000)
	checkpoint, err := exporter.readCheckpoint(cfg)
	if err != nil || checkpoint.LastExportedHeight != 2000 || checkpoint.Identity != exporter.exportIdentity(cfg) {
		t.Fatalf("reopened progress not bound: %+v %v", checkpoint, err)
	}
	original := taskRecoveryRead(t, filepath.Join(home, exportStateFile))
	other := taskRecoveryRPC(t, taskRecoveryChain, 3000)
	for _, field := range []string{"bucket", "prefix", "region", "source", "chain"} {
		t.Run(field, func(t *testing.T) {
			changed := cfg
			chain := taskRecoveryChain
			switch field {
			case "bucket":
				changed.Bucket = "elsewhere"
			case "prefix":
				changed.Prefix = "new-prefix"
			case "region":
				changed.Region = "other-region"
			case "source":
				changed.RPCEndpoint = other.server.URL
			case "chain":
				chain = "other-chain"
				changed.RPCEndpoint = taskRecoveryRPC(t, chain, 3000).server.URL
			}
			_, err := NewResultExporter(home, chain, "node", upload).Handler()(context.Background(), taskRecoveryParams(changed))
			var failure *engine.TaskError
			if !errors.As(err, &failure) || failure.Operation != "checkpoint-identity" || failure.Retryable {
				t.Fatalf("unrelated progress adopted: %v", err)
			}
			if !bytes.Equal(original, taskRecoveryRead(t, filepath.Join(home, exportStateFile))) {
				t.Fatal("refusal changed progress")
			}
		})
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".write-") {
			t.Fatal("atomic checkpoint left temporary data")
		}
	}
}

func taskRecoveryRequireFailure(t *testing.T, err error, operation string, retryable bool) {
	t.Helper()
	var failure *engine.TaskError
	if !errors.As(err, &failure) || failure.Operation != operation || failure.Retryable != retryable {
		t.Fatalf("want %s retryable=%v, got %v", operation, retryable, err)
	}
}

func TestTaskRecoveryExportFailuresAreObservable(t *testing.T) {
	source := taskRecoveryRPC(t, taskRecoveryChain, 1000)
	objects, _, upload := taskRecoveryTransport(t)
	cfg := taskRecoveryExportConfig(source)
	for _, kind := range []string{"source", "block-source", "jsonrpc-source", "checkpoint-corrupt", "checkpoint-legacy", "checkpoint-read", "snapshot-corrupt", "no-progress", "upload", "checkpoint-write"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			source.unavailable.Store(false)
			source.blockFailure.Store(false)
			source.rpcFailure.Store(false)
			source.height.Store(1000)
			objects.refuse("PUT", "/results/archive/1-1000.ndjson.gz", false)
			objects.mu.Lock()
			objects.afterPut = nil
			objects.mu.Unlock()
			operation := "checkpoint"
			retryable := false
			switch kind {
			case "source":
				source.unavailable.Store(true)
				operation = "source"
				retryable = true
			case "block-source":
				source.blockFailure.Store(true)
				operation = "source"
				retryable = true
			case "jsonrpc-source":
				source.rpcFailure.Store(true)
				operation = "source"
				retryable = true
			case "checkpoint-corrupt":
				taskRecoveryWrite(t, filepath.Join(home, exportStateFile), []byte(`{"lastExportedHeight":`))
			case "checkpoint-legacy":
				taskRecoveryWrite(t, filepath.Join(home, exportStateFile), []byte(`{"lastExportedHeight":99999}`))
			case "checkpoint-read":
				if err := os.Mkdir(filepath.Join(home, exportStateFile), 0700); err != nil {
					t.Fatal(err)
				}
			case "snapshot-corrupt":
				taskRecoveryWrite(t, filepath.Join(home, SnapshotHeightFile), []byte("corrupt"))
			case "no-progress":
				source.height.Store(999)
				operation = "no-progress"
				retryable = true
			case "upload":
				objects.refuse("PUT", "/results/archive/1-1000.ndjson.gz", true)
				operation = "S3"
				retryable = true
			case "checkpoint-write":
				retryable = true
				objects.mu.Lock()
				objects.afterPut = func(string) error { return os.Mkdir(filepath.Join(home, exportStateFile), 0700) }
				objects.mu.Unlock()
			}
			exporter := NewResultExporter(home, taskRecoveryChain, "node", upload)
			_, err := exporter.Handler()(context.Background(), taskRecoveryParams(cfg))
			taskRecoveryRequireFailure(t, err, operation, retryable)
			if kind == "checkpoint-write" {
				// The upload is durable, but absent checkpoint progress must
				// fail. Reopening after repair backfills that page safely.
				taskRecoveryCheckPage(t, filepath.Join(objects.root, "results", "archive", "1-1000.ndjson.gz"), 1, 1000)
				objects.mu.Lock()
				objects.afterPut = nil
				objects.mu.Unlock()
				if err := os.Remove(filepath.Join(home, exportStateFile)); err != nil {
					t.Fatal(err)
				}
				exporter = nil
				if _, err := NewResultExporter(home, taskRecoveryChain, "node", upload).Handler()(context.Background(), taskRecoveryParams(cfg)); err != nil {
					t.Fatal(err)
				}
				state, err := NewResultExporter(home, taskRecoveryChain, "node", upload).readCheckpoint(cfg)
				if err != nil || state.LastExportedHeight != 1000 {
					t.Fatalf("durable retry did not converge: %+v %v", state, err)
				}
			}
		})
	}
}

func TestTaskRecoveryExportEngineDoesNotCompleteFailedWork(t *testing.T) {
	source := taskRecoveryRPC(t, taskRecoveryChain, 1000)
	source.unavailable.Store(true)
	_, _, upload := taskRecoveryTransport(t)
	home := t.TempDir()
	storePath := filepath.Join(home, "tasks.db")
	store, err := engine.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	exporter := NewResultExporter(home, taskRecoveryChain, "node", upload)
	e := engine.NewEngine(ctx, map[engine.TaskType]engine.TaskHandler{engine.TaskResultExport: exporter.Handler()}, store)
	id := uuid.NewString()
	if _, err := e.Submit(engine.Task{ID: id, Type: engine.TaskResultExport, Params: taskRecoveryParams(taskRecoveryExportConfig(source))}); err != nil {
		t.Fatal(err)
	}
	result := repairWaitResult(t, e, id)
	if result.Status != engine.TaskStatusFailed || !strings.Contains(result.Error, "source") {
		t.Fatalf("engine concealed failed source: %+v", result)
	}
	cancel()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = engine.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	e = engine.NewEngine(ctx, map[engine.TaskType]engine.TaskHandler{engine.TaskResultExport: NewResultExporter(home, taskRecoveryChain, "node", upload).Handler()}, store)
	restored, err := e.GetResult(id)
	if err != nil {
		t.Fatal(err)
	}
	if restored == nil || restored.Status != engine.TaskStatusFailed {
		t.Fatalf("reopen lost failed outcome: %+v", restored)
	}
	source.unavailable.Store(false)
	retryID := uuid.NewString()
	if _, err := e.Submit(engine.Task{ID: retryID, Type: engine.TaskResultExport, Params: taskRecoveryParams(taskRecoveryExportConfig(source))}); err != nil {
		t.Fatal(err)
	}
	result = repairWaitResult(t, e, retryID)
	if result.Status != engine.TaskStatusCompleted {
		t.Fatalf("explicit retry did not export: %+v", result)
	}
}

func TestTaskRecoveryExportCancellation(t *testing.T) {
	source := taskRecoveryRPC(t, taskRecoveryChain, 1000)
	_, _, upload := taskRecoveryTransport(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err := NewResultExporter(t.TempDir(), taskRecoveryChain, "node", upload).Handler()(ctx, taskRecoveryParams(taskRecoveryExportConfig(source)))
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("export ignored cancellation: %v", err)
	}
}

func TestTaskRecoveryExportComparisonCheckpointFailure(t *testing.T) {
	shadow := fakeRPCAndBlockServer(5, "SHADOW", "RESULTS", nil)
	canonical := fakeRPCAndBlockServer(5, "CANONICAL", "RESULTS", nil)
	defer shadow.Close()
	defer canonical.Close()
	home := t.TempDir()
	objects, _, upload := taskRecoveryTransport(t)
	cfg := ResultExportRequest{Bucket: "comparison", Region: "local", Prefix: "survey/", RPCEndpoint: shadow.URL, CanonicalRPC: canonical.URL}
	objects.mu.Lock()
	objects.afterPut = func(path string) error {
		if strings.HasSuffix(path, ".compare.ndjson.gz") {
			return os.Mkdir(filepath.Join(home, exportStateFile), 0700)
		}
		return nil
	}
	objects.mu.Unlock()
	_, err := NewResultExporter(home, "test-1", "shadow", upload).Handler()(context.Background(), taskRecoveryParams(cfg))
	taskRecoveryRequireFailure(t, err, "checkpoint", true)
	objects.mu.Lock()
	objects.afterPut = nil
	objects.mu.Unlock()
	if err := os.Remove(filepath.Join(home, exportStateFile)); err != nil {
		t.Fatal(err)
	}
	exporter := NewResultExporter(home, "test-1", "shadow", upload)
	if _, err := exporter.Handler()(context.Background(), taskRecoveryParams(cfg)); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := exporter.readCheckpoint(cfg)
	if err != nil || checkpoint.LastExportedHeight != 1 {
		t.Fatalf("comparison failed to retain progress after reopen: %+v %v", checkpoint, err)
	}
	cfg.Prefix = "elsewhere/"
	_, err = NewResultExporter(home, "test-1", "shadow", upload).Handler()(context.Background(), taskRecoveryParams(cfg))
	taskRecoveryRequireFailure(t, err, "checkpoint-identity", false)
}
