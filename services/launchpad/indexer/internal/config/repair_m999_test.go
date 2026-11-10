package config_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/runtimedocs"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type repairM999Response struct {
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

type repairM999Packet struct {
	Options    receipt.RPCOptions
	Markets    []receipt.DiscoveredMarket
	NativeSwap common.Hash
	TenantSwap common.Hash
	Responses  map[string]repairM999Response
}

func repairM999ReadKey(method string, params []any) (string, error) {
	if params == nil {
		params = []any{}
	}
	if method == "eth_call" && len(params) == 2 {
		if fields, ok := params[0].(map[string]any); ok && fields["from"] == "0x0000000000000000000000000000000000000000" {
			// Match ethclient's zero sender to omitted capture fields without
			// changing the target, input, pinned block or any nonzero sender.
			delete(fields, "from")
		}
	}
	encoded, err := json.Marshal(params)
	return method + " " + string(encoded), err
}

// Decision 3177: recorded digests may name Current or an Approved history
// entry until the packet is re-recorded after the upgrade (decision 3119).
func repairM999ApprovedSourceDigest(digest string) bool {
	if digest == deployment.Current().SourceManifestSHA256 {
		return true
	}
	for height, ok := deployment.NextActivationHeight(0); ok; height, ok = deployment.NextActivationHeight(height) {
		manifest, approved := deployment.Approved(height)
		if approved && digest == manifest.SourceManifestSHA256 {
			return true
		}
	}
	return false
}

func TestRepair_22_39_328(t *testing.T) {
	if !repairM999ApprovedSourceDigest(deployment.Current().SourceManifestSHA256) {
		t.Fatal("current source digest refused")
	}
	history := 0
	for height, ok := deployment.NextActivationHeight(0); ok; height, ok = deployment.NextActivationHeight(height) {
		manifest, approved := deployment.Approved(height)
		if !approved || !repairM999ApprovedSourceDigest(manifest.SourceManifestSHA256) {
			t.Fatalf("approved source digest refused at activation %d", height)
		}
		if height < deployment.Current().ActivationHeight {
			history++
		}
	}
	if history == 0 {
		t.Fatal("approved history must exercise recorded digest admission")
	}
	for _, digest := range []string{"", strings.Repeat("0", 64), strings.Repeat("f", 64), deployment.Current().SourceManifestSHA256[:63]} {
		if repairM999ApprovedSourceDigest(digest) {
			t.Fatalf("unapproved source digest accepted: %q", digest)
		}
	}
}

func repairM999AssertManifest(t *testing.T, options receipt.RPCOptions) {
	t.Helper()
	recorded, current := options.Manifest, deployment.Current()
	// Decision 3177: retain an approved recorded source digest until the packet
	// is re-recorded after the upgrade; compare every other manifest field.
	if !repairM999ApprovedSourceDigest(recorded.SourceManifestSHA256) {
		t.Fatal("recorded manifest must bind a current or approved historical source manifest")
	}
	recorded.SourceManifestSHA256 = current.SourceManifestSHA256
	// Decision 3119: keep the producer's runtime identities until the upgrade;
	// re-record the packet after the upgrade to restore exact runtime equality.
	// Copy both role maps so code attestation still sees the recorded hashes.
	recorded.Roles = make(map[string]deployment.Role, len(options.Manifest.Roles))
	currentRoles := current.Roles
	current.Roles = make(map[string]deployment.Role, len(currentRoles))
	for name, role := range options.Manifest.Roles {
		approved, ok := currentRoles[name]
		if !ok || role.Address != approved.Address || role.Kind != approved.Kind {
			t.Fatalf("recorded role %s must retain the current address and kind", name)
		}
		role.RuntimeHash = ""
		recorded.Roles[name] = role
	}
	for name, role := range currentRoles {
		role.RuntimeHash = ""
		current.Roles[name] = role
	}
	if !reflect.DeepEqual(recorded, current) {
		t.Fatal("recorded manifest must match the current deployment apart from role runtime hashes")
	}

	var catalog, reviewed decode.Catalog
	if err := json.Unmarshal(options.CatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "sdk", "launchpad", "protocol", "catalog", "reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &reviewed); err != nil {
		t.Fatal(err)
	}
	// Decision 3177: only the embedded evidence may retain an approved history
	// digest until re-recording after the upgrade; the live review is exact.
	if !repairM999ApprovedSourceDigest(catalog.ManifestSHA256) {
		t.Fatal("recorded ABI must bind a current or approved historical source manifest")
	}
	if reviewed.ManifestSHA256 != current.SourceManifestSHA256 {
		t.Fatal("reviewed ABI must bind the current source manifest exactly")
	}
	recordedRevisions, reviewedRevisions := map[string]string{}, map[string]string{}
	for _, contract := range catalog.Contracts {
		recordedRevisions[contract.Name] = contract.ABIRevision
	}
	for _, contract := range reviewed.Contracts {
		reviewedRevisions[contract.Name] = contract.ABIRevision
	}
	if len(recordedRevisions) == 0 || !reflect.DeepEqual(recordedRevisions, reviewedRevisions) {
		t.Fatal("recorded ABI revisions must match the reviewed catalog")
	}
	for _, epoch := range options.Epochs {
		role := epoch.Deployment.Role
		if role == deployment.LaunchpadPrecompileRole {
			role = "EventEmitter"
		}
		if epoch.Deployment.ABIRevision == "" || epoch.Deployment.ABIRevision != reviewedRevisions[role] {
			t.Fatalf("recorded epoch %s must retain its reviewed ABI revision", epoch.Deployment.Role)
		}
	}
	for _, template := range options.Discovery {
		if template.ABIRevision == "" || template.ABIRevision != reviewedRevisions[template.Role] {
			t.Fatalf("recorded discovery role %s must retain its reviewed ABI revision", template.Role)
		}
	}
}

func repairM999PacketFromProducer(t *testing.T) repairM999Packet {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "receipt", "testdata", "repair53-producer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var packet repairM999Packet
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	repairM999AssertManifest(t, packet.Options)
	if packet.Options.Anchor == nil {
		t.Fatal("recorded producer must bind the approved current deployment and anchor")
	}
	responses := make(map[string]repairM999Response, len(packet.Responses))
	for key, response := range packet.Responses {
		method, encoded, ok := strings.Cut(key, " ")
		var params []any
		if !ok || json.Unmarshal([]byte(encoded), &params) != nil {
			t.Fatalf("invalid recorded request key %q", key)
		}
		canonical, err := repairM999ReadKey(method, params)
		if err != nil {
			t.Fatal(err)
		}
		if previous, exists := responses[canonical]; exists && !reflect.DeepEqual(previous, response) {
			t.Fatalf("conflicting producer responses for %s", canonical)
		}
		responses[canonical] = response
	}
	packet.Responses = responses
	return packet
}

func repairM999Documents(t *testing.T, packet repairM999Packet) map[string]app.RenderedDocument {
	t.Helper()
	// Decision 3177: render recorded options with their byte-identical historical
	// review until the packet is re-recorded after the upgrade.
	reviewedName := "reviewed.json"
	if packet.Options.Manifest.SourceManifestSHA256 != deployment.Current().SourceManifestSHA256 {
		if !repairM999ApprovedSourceDigest(packet.Options.Manifest.SourceManifestSHA256) {
			t.Fatal("runtime documents require an approved source digest")
		}
		reviewedName = filepath.Join("history", "reviewed-"+packet.Options.Manifest.SourceManifestSHA256[:16]+".json")
	}
	reviewed, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "sdk", "launchpad", "protocol", "catalog", reviewedName))
	if err != nil {
		t.Fatal(err)
	}
	in := runtimedocs.Inputs{Manifest: packet.Options.Manifest, ReviewedCatalog: reviewed, Anchor: *packet.Options.Anchor}
	for _, template := range packet.Options.Discovery {
		if template.Role == "SidioraPool" {
			in.PoolRuntimeHash = common.HexToHash(template.RuntimeHash)
		}
		if template.Role != "SidioraERC20" {
			continue
		}
		for key, response := range packet.Responses {
			if !strings.HasPrefix(key, "eth_getCode ") {
				continue
			}
			var encoded string
			if err := json.Unmarshal(response.Result, &encoded); err != nil {
				t.Fatal(err)
			}
			code, err := hexutil.Decode(encoded)
			if err != nil {
				t.Fatal(err)
			}
			hash, err := runtimedocs.MaskedRuntimeHash(code, template.Immutables)
			if err == nil && hash.Hex() == template.RuntimeHash {
				in.ERC20RuntimeCode = code
				break
			}
		}
	}
	if len(in.ERC20RuntimeCode) == 0 {
		t.Fatal("producer transcript lacks reviewed ERC20 runtime")
	}
	docs, err := app.RenderRuntimeDocuments(in, app.DefaultDeliveryParameters())
	if err != nil {
		t.Fatal(err)
	}
	// Retain the producer-issued historical epochs needed to recover its market.
	var receiptDoc map[string]json.RawMessage
	if err := json.Unmarshal(docs["receipt.json"].Bytes, &receiptDoc); err != nil {
		t.Fatal(err)
	}
	receiptDoc["production"], err = json.Marshal(packet.Options)
	if err != nil {
		t.Fatal(err)
	}
	receiptRaw, err := json.Marshal(receiptDoc)
	if err != nil {
		t.Fatal(err)
	}
	docs["receipt.json"] = app.RenderedDocument{Bytes: receiptRaw, SHA256: fmt.Sprintf("%x", sha256.Sum256(receiptRaw))}
	// Issue a test-owned backfill request over the two recorded current blocks.
	// Preserve every field the real runtime document producer generated.
	var lifecycle map[string]json.RawMessage
	if err := json.Unmarshal(docs["lifecycle.json"].Bytes, &lifecycle); err != nil {
		t.Fatal(err)
	}
	from := int64(packet.Options.Anchor.Height + 1)
	request := store.ReplayRequest{JobID: "repair-m999", SourceStream: app.DefaultDeliveryParameters().CanonicalStreamID,
		Target: "indexer", Generation: "repair-m999", ProjectionRevision: "repair-m999", Mode: "backfill",
		Requester: "repair-m999", AuthorizationID: "repair-m999", FromHeight: from, ToHeight: from + 1,
		CheckpointHeight: from + 1, CheckpointHash: "0x73a1ccc1b108a639a3102c6fd55fc043f818f04a55a92b3f2febc43ba5a2ca4b"}
	if err := store.ValidateReplayRequest(request); err != nil {
		t.Fatal(err)
	}
	lifecycle["backfillRequest"], err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	docs["lifecycle.json"] = app.RenderedDocument{Bytes: raw, SHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	return docs
}

func repairM999Environment(t *testing.T, docs map[string]app.RenderedDocument) {
	t.Helper()
	// Remove inherited service settings; the gate supplies all of its inputs.
	var clear func(reflect.Type)
	clear = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous {
				clear(field.Type)
				continue
			}
			if name := strings.Split(field.Tag.Get("env"), ",")[0]; name != "" {
				t.Setenv(name, "")
			}
		}
	}
	clear(reflect.TypeOf(config.Config{}))
	for key, value := range map[string]string{
		"DATABASE_URL": "postgres://u:p@localhost:5432/db", "REDIS_URL": "redis://localhost:6379",
		"REDIS_BULL_URL": "redis://localhost:6379/1", "RPC_URL": "http://localhost:8545",
		"NODE_ENV": "production", "INDEXER_MODE": "backfill",
	} {
		t.Setenv(key, value)
	}
	for name, role := range map[string]string{
		"EVENT_EMITTER_ADDRESS": "EventEmitter", "POOL_REGISTRY_ADDRESS": "PoolRegistry", "ROUTER_ADDRESS": "Router",
		"FACTORY_ADDRESS": "SidioraFactory", "QUOTER_ADDRESS": "Quoter", "PROTOCOL_CONFIG_ADDRESS": "ProtocolConfig",
		"FEE_ACCUMULATOR_ADDRESS": "FeeAccumulator", "SIDIORA_NFT_ADDRESS": "SidioraNFT",
		"FEES_ROUTER_ADDRESS": "FeesRouter", "POOL_BEACON_ADDRESS": "PoolBeacon",
	} {
		t.Setenv(name, deployment.Current().Roles[role].Address)
	}
	for prefix, name := range map[string]string{"INDEXER_RECEIPT_CONFIG": "receipt.json", "DELIVERY_CONFIG": "delivery.json", "INDEXER_LIFECYCLE_CONFIG": "lifecycle.json"} {
		doc := docs[name]
		t.Setenv(prefix+"_DOCUMENT", base64.StdEncoding.EncodeToString(doc.Bytes))
		t.Setenv(prefix+"_SHA256", doc.SHA256)
	}
}

func TestRepairM_999_ProductionBackfillDefault(t *testing.T) {
	packet := repairM999PacketFromProducer(t)
	docs := repairM999Documents(t, packet)
	repairM999Environment(t, docs)
	for _, mode := range []string{config.ModeBackfill, config.ModeLive} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("INDEXER_MODE", mode)
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.NodeEnv != "production" || cfg.IndexerMode != mode || cfg.BackfillBatchSize != 64 {
				t.Fatalf("production %s default: environment=%q mode=%q batch=%d, want 64", mode, cfg.NodeEnv, cfg.IndexerMode, cfg.BackfillBatchSize)
			}
		})
	}
	for _, size := range []string{"1", "32", "64"} {
		t.Setenv("BACKFILL_BATCH_SIZE", size)
		cfg, err := config.Load()
		if err != nil || fmt.Sprint(cfg.BackfillBatchSize) != size {
			t.Fatalf("explicit batch %s: %d, %v", size, cfg.BackfillBatchSize, err)
		}
	}
}

func TestRepairM_999_ReceiptQualifiedBackfill(t *testing.T) {
	packet := repairM999PacketFromProducer(t)
	repairM999Environment(t, repairM999Documents(t, packet))
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BackfillBatchSize != 64 {
		t.Fatalf("default batch %d, want 64", cfg.BackfillBatchSize)
	}
	var reads atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		var call struct {
			ID     json.RawMessage
			Method string
			Params []any
		}
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			t.Error(err)
			http.Error(w, "invalid request", 400)
			return
		}
		key, err := repairM999ReadKey(call.Method, call.Params)
		if err != nil {
			t.Error(err)
			http.Error(w, "invalid params", 400)
			return
		}
		response, ok := packet.Responses[key]
		if !ok {
			t.Errorf("unrecorded producer read: %s", key)
			http.Error(w, "unrecorded read", 502)
			return
		}
		wire := map[string]any{"jsonrpc": "2.0", "id": call.ID}
		if len(response.Error) != 0 {
			wire["error"] = response.Error
		} else {
			wire["result"] = response.Result
		}
		if err := json.NewEncoder(w).Encode(wire); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	var doc struct {
		Realm          string                           `json:"realm"`
		Production     receipt.RPCOptions               `json:"production"`
		PrivateFixture receipt.PrivateFixtureRPCOptions `json:"privateFixture"`
	}
	if err := config.ReadBoundDocument(cfg.ReceiptConfigFile, cfg.ReceiptConfigSHA256, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Realm != "production" {
		t.Fatalf("receipt realm %q", doc.Realm)
	}
	authority, err := receipt.NewRPCQualification(ctx, client, doc.Production)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.Rehydrate(ctx, packet.Markets); err != nil {
		t.Fatal(err)
	}
	from := packet.Options.Anchor.Height + 1
	// The final batch ends at the issued checkpoint, as production replay does.
	to := min(from+uint64(cfg.BackfillBatchSize)-1, from+1)
	batch, qualified, err := authority.FetchQualifiedBatch(ctx, from, to)
	if err != nil {
		t.Fatalf("qualify default-sized final backfill batch: %v", err)
	}
	if len(batch.Blocks) != 2 || len(qualified) == 0 || batch.PreviousHeight != int64(from)-1 {
		t.Fatalf("backfill blocks=%d receipts=%d previous=%d", len(batch.Blocks), len(qualified), batch.PreviousHeight)
	}
	if err := receipt.ValidateBindings(ctx, batch, qualified); err != nil {
		t.Fatal(err)
	}
	wantTransactions := map[string]bool{
		strings.ToLower(packet.NativeSwap.Hex()): false,
		strings.ToLower(packet.TenantSwap.Hex()): false,
	}
	if packet.NativeSwap == (common.Hash{}) || packet.TenantSwap == (common.Hash{}) || len(wantTransactions) != 2 {
		t.Fatal("producer must identify distinct native and tenant swaps")
	}
	for _, qualifiedReceipt := range qualified {
		transaction, _, _, err := qualifiedReceipt.SourceContext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := wantTransactions[transaction]; exists {
			wantTransactions[transaction] = true
		}
	}
	for transaction, found := range wantTransactions {
		if !found {
			t.Errorf("recorded swap %s missing from qualified backfill", transaction)
		}
	}
	before := reads.Load()
	for _, count := range []uint64{65, 2000} {
		if _, _, err := authority.FetchQualifiedBatch(ctx, from, from+count-1); !errors.Is(err, receipt.ErrBinding) {
			t.Fatalf("%d blocks must fail receipt range admission: %v", count, err)
		}
	}
	if reads.Load() != before {
		t.Fatal("oversized receipt range reached RPC")
	}
}
