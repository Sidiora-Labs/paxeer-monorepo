package receipt

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	_ "modernc.org/sqlite"
)

// These inputs identify actual producer-issued receipts and approved runtime
// documents. No RPC state, receipt, market, or native event is substituted.
type repair53Inputs struct {
	Endpoint       string
	Options        RPCOptions
	Markets        []DiscoveredMarket
	NativeSwap     common.Hash
	TenantSwap     common.Hash
	HistoricalSwap common.Hash
	Responses      map[string]repair53Recorded
}

type repair53Recorded struct {
	Key    string          `json:"k"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

func repair53AssertManifest(t *testing.T, options RPCOptions) {
	t.Helper()
	recorded, current := options.Manifest, deployment.Current()
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
	if catalog.ManifestSHA256 != current.SourceManifestSHA256 || reviewed.ManifestSHA256 != current.SourceManifestSHA256 {
		t.Fatal("recorded and reviewed ABIs must bind the current source manifest")
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

func repair53InputsFromProducer(t *testing.T) repair53Inputs {
	t.Helper()
	// The transcript must be captured by the owning node domain from actual
	// root and tenant execution. Qualification never launches a toolchain or
	// substitutes constructed logs for producer-issued receipts.
	raw, err := os.ReadFile("testdata/repair53-producer.json")
	if err != nil {
		t.Fatalf("committed native/tenant producer transcript required (node domain 8 wiring request in repair_53.kvx): %v", err)
	}
	var input repair53Inputs
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		t.Fatal(err)
	}
	repair53AssertManifest(t, input.Options)
	if len(input.Markets) == 0 {
		t.Fatal("producer packet must name the approved current deployment and real markets")
	}
	return repair53Replay(t, input)
}

func repair53Replay(t *testing.T, input repair53Inputs) repair53Inputs {
	t.Helper()
	responses := make(map[string]repair53Recorded, len(input.Responses))
	for key, response := range input.Responses {
		method, encoded, ok := strings.Cut(key, " ")
		var params []any
		if !ok || json.Unmarshal([]byte(encoded), &params) != nil {
			t.Fatalf("invalid recorded request key %q", key)
		}
		canonical, err := repair53ReadKey(method, params)
		if err != nil {
			t.Fatal(err)
		}
		if prior, exists := responses[canonical]; exists && (!bytes.Equal(prior.Result, response.Result) || !bytes.Equal(prior.Error, response.Error)) {
			t.Fatalf("conflicting recorded responses for %s", canonical)
		}
		responses[canonical] = response
	}
	// This server transports immutable results from actual execution. Missing
	// reads fail closed; no invented RPC answer or live endpoint is available.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage
			Method string
			Params []any
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		key, err := repair53ReadKey(request.Method, request.Params)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		response, exists := responses[key]
		if !exists {
			http.Error(w, "unrecorded producer read: "+key, 502)
			return
		}
		result := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		if len(response.Error) > 0 {
			result["error"] = response.Error
		} else {
			result["result"] = response.Result
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Errorf("producer replay: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	input.Endpoint = server.URL
	return input
}

func repair53ReadKey(method string, params []any) (string, error) {
	// Omitted params and [] both describe a no-argument JSON-RPC request.
	if params == nil {
		params = []any{}
	}
	if method == "eth_call" && len(params) == 2 {
		if call, ok := params[0].(map[string]any); ok && call["from"] == "0x0000000000000000000000000000000000000000" {
			// ethclient sends the zero sender explicitly; some captures omit
			// it. Keep the target, input, pinned block and all other fields
			// exact, including any nonzero sender that can affect execution.
			delete(call, "from")
		}
	}
	encoded, err := json.Marshal(params)
	return method + " " + string(encoded), err
}

func TestRepair_53_recorded_read_matching(t *testing.T) {
	key := func(encoded string) string {
		t.Helper()
		var params []any
		if err := json.Unmarshal([]byte(encoded), &params); err != nil {
			t.Fatal(err)
		}
		result, err := repair53ReadKey("eth_call", params)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	const recorded = `[{"gas":"0x249f0","input":"0x071de20d","to":"0xf3669edc475cb6e1f0d7635240abd8e8fb7e20c6"},{"blockHash":"0x367aa367ab921f314429468f9023ba3ec4410a43afd00305f9b2cc3b53d3186e"}]`
	explicitZero := strings.Replace(recorded, `[{`, `[{"from":"0x0000000000000000000000000000000000000000",`, 1)
	if key(recorded) != key(explicitZero) {
		t.Fatal("omitted and explicit zero sender must match the same recorded read")
	}
	for _, changed := range []string{
		strings.Replace(explicitZero, "0x0000000000000000000000000000000000000000", "0x0000000000000000000000000000000000000001", 1),
		strings.Replace(recorded, "0xf3669edc475cb6e1f0d7635240abd8e8fb7e20c6", "0x0000000000000000000000000000000000000001", 1),
		strings.Replace(recorded, "0x071de20d", "0x071de20e", 1),
		strings.Replace(recorded, "0x367aa367", "0x467aa367", 1),
	} {
		if key(recorded) == key(changed) {
			t.Fatalf("different execution request matched a recorded read: %s", changed)
		}
	}
}

func repair53Open(t *testing.T, ctx context.Context, input repair53Inputs) (*RPCQualification, func()) {
	t.Helper()
	client, closeClient, err := NewBoundedRPCClient(ctx, input.Endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewRPCQualification(ctx, client, input.Options)
	if err != nil {
		closeClient()
		t.Fatal(err)
	}
	return authority, closeClient
}

func repair53Fill(t *testing.T, ctx context.Context, authority *RPCQualification, hash common.Hash) (accounting.FillV1, *QualifiedReceipt) {
	t.Helper()
	if hash == (common.Hash{}) {
		t.Fatal("producer transaction identity is required")
	}
	bundle, err := authority.Bundle(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	for _, log := range bundle.Receipt.Logs {
		env, err := authority.current().registry.Decode(*log)
		if err != nil {
			t.Fatalf("producer log %d at %s refused: status=%s reason=%s: %v", log.Index, log.Address, env.Status, env.FailureReason, err)
		}
		if log.Address == decode.LaunchpadPrecompileAddress && env.EventName == "Swap" && !qualifiedSwapFamily(env, log) {
			t.Fatalf("native Swap refused: status=%s reason=%s role=%s family=%+v", env.Status, env.FailureReason, env.EmitterRole, env.Family)
		}
	}
	batch, qualified, err := authority.FetchQualifiedBatch(ctx, bundle.Header.Number.Uint64(), bundle.Header.Number.Uint64())
	if err != nil {
		t.Fatalf("qualify producer transaction %s at height %s: %v", hash, bundle.Header.Number, err)
	}
	if batch.ChainID != 125 || len(batch.Blocks) != 1 || batch.Blocks[0].Hash != strings.ToLower(bundleBlockHash(bundle).Hex()) {
		t.Fatal("root block identity changed")
	}
	if err = ValidateBindings(ctx, batch, qualified); err != nil {
		t.Fatal(err)
	}
	for _, receipt := range qualified {
		tx, _, _, err := receipt.SourceContext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if tx != strings.ToLower(hash.Hex()) {
			continue
		}
		fills, err := receipt.Fills(ctx)
		if err != nil || len(fills) != 1 {
			t.Fatalf("actual swap must yield one accounting fill: %v", err)
		}
		if fills[0].Position.ChainID != "125" || fills[0].Position.BlockHash != strings.ToLower(bundleBlockHash(bundle).Hex()) {
			t.Fatal("signed tenant domain replaced parent source identity")
		}
		return fills[0], receipt
	}
	t.Fatal("producer swap was not qualified")
	return accounting.FillV1{}, nil
}

func repair53Delivery(t *testing.T, fill accounting.FillV1) []byte {
	t.Helper()
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	position := accounting.PartitionPosition{ProjectionVersion: "repair53", PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress, Sequence: "1", SourceEventID: fill.SourceFillID, PayloadDigest: digest, SourceOrder: fill.Position}
	position.EffectIdentity = accounting.EffectIdentity(fill.SourceFillID, "market", position.ProjectionVersion, "", position.PartitionKind, position.PartitionKey)
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	seconds, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: accounting.MarketSubject, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: fill.Position.Height, TransactionIndex: fill.Position.TransactionIndex, LogIndex: fill.Position.LogIndex}, EmittedAt: time.Unix(seconds, 0).UTC(), TraceID: publication.PublicationID, PayloadHash: delivery.Hash(payload), Payload: payload}
	wire, err := envelope.Encode(repair53Policy(), accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func repair53Policy() delivery.Policy {
	return delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{accounting.MarketSubject: true}}
}

func repair53ConsumeAfterRestart(t *testing.T, ctx context.Context, wire []byte, want accounting.FillV1) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "publication.sqlite")
	// SQLite is the committed-fixture transport peer. Both endpoints use the
	// production delivery/accounting codecs; storage closes before reopening.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "CREATE TABLE publications (sequence INTEGER PRIMARY KEY, wire BLOB NOT NULL)"); err == nil {
		_, err = db.ExecContext(ctx, "INSERT INTO publications VALUES (1, ?)", wire)
	}
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("durable transport write: %v; close: %v", err, closeErr)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var recovered []byte
	if err = db.QueryRowContext(ctx, "SELECT wire FROM publications WHERE sequence=1").Scan(&recovered); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire, recovered) {
		t.Fatal("durable broker changed exact publication bytes")
	}
	envelope, err := delivery.Decode(recovered, repair53Policy(), accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := accounting.DecodePublicationV1(envelope.Payload)
	if err != nil || publication.Fill == nil {
		t.Fatalf("real accounting consumer rejected publication: %v", err)
	}
	if !reflect.DeepEqual(*publication.Fill, want) || envelope.ChainID != 125 {
		t.Fatal("durable transport changed the qualified economic fact")
	}
	if len(publication.Positions) != 1 || publication.Positions[0].PartitionKey != "125:"+want.PoolAddress || publication.Positions[0].SourceOrder != want.Position {
		t.Fatal("durable publication lost its partition/source coordinates")
	}
}

// SQLite is a real local durable database. Each invocation closes its database
// connection, and restoration uses the production canonical-row codec.
func repair53Database(t *testing.T, path string, row *canonicalMarketRow, payload []byte, nft string) []DiscoveredMarket {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if row != nil {
		if _, err = db.Exec("CREATE TABLE creations (token TEXT, pool TEXT, height INTEGER, transaction_index INTEGER, block_hash TEXT, transaction_hash TEXT, log_index INTEGER, provenance BLOB, payload BLOB, nft TEXT)"); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec("INSERT INTO creations VALUES (?,?,?,?,?,?,?,?,?,?)", row.Token, row.Pool, row.Height, row.TransactionIndex, row.BlockHash, row.TransactionHash, row.LogIndex, []byte(row.Provenance), payload, nft); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	var recovered canonicalMarketRow
	var recoveredPayload []byte
	var recoveredNFT string
	if err = db.QueryRow("SELECT token,pool,height,transaction_index,block_hash,transaction_hash,log_index,provenance,payload,nft FROM creations ORDER BY height,transaction_index,log_index").Scan(&recovered.Token, &recovered.Pool, &recovered.Height, &recovered.TransactionIndex, &recovered.BlockHash, &recovered.TransactionHash, &recovered.LogIndex, &recovered.Provenance, &recoveredPayload, &recoveredNFT); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recoveredPayload, payload) || recoveredNFT != nft {
		t.Fatal("reopened database changed canonical creation payload")
	}
	market, err := recovered.decode()
	if err != nil {
		t.Fatal(err)
	}
	return []DiscoveredMarket{market}
}

func repair53SwapRestart(t *testing.T, tenant bool) {
	t.Helper()
	input := repair53InputsFromProducer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	authority, closeClient := repair53Open(t, ctx, input)
	defer func() { closeClient() }()
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	hash := input.NativeSwap
	if tenant {
		hash = input.TenantSwap
	}
	fill, qualified := repair53Fill(t, ctx, authority, hash)
	domain, err := qualified.SignedChainID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tenant {
		if domain.Uint64() <= virtualChainIDBase || domain.Uint64() > virtualChainIDBase+math.MaxUint32 {
			t.Fatal("test receipt was not signed by a tenant")
		}
	} else {
		if domain.Cmp(big.NewInt(125)) != 0 || fill.ActualEmitter != deployment.LaunchpadPrecompileAddress {
			t.Fatal("test receipt did not originate from the native root swap producer")
		}
		envelopes, err := qualified.QualifiedEvents(ctx)
		if err != nil {
			t.Fatal(err)
		}
		native := false
		for _, env := range envelopes {
			if env.EventName == "Swap" && (env.EmitterRole == deployment.LaunchpadPrecompileRole || env.EmitterRole == "SidioraPool") {
				native = true
			}
		}
		if !native {
			t.Fatal("native producer ABI did not reach the consumer")
		}
		metadata, err := authority.current().StableMetadata(ctx, common.HexToAddress(fill.PoolAddress), common.HexToHash(fill.Position.BlockHash))
		if err != nil {
			t.Fatal(err)
		}
		proof, err := qualified.SourceProvenance(ctx, common.HexToAddress(deployment.LaunchpadPrecompileAddress))
		if err != nil || metadata.EmitterCodeEpoch != proof.EmitterCodeEpoch || fill.Provenance.EmitterCodeEpoch != proof.EmitterCodeEpoch || metadata.ABIRevision != proof.ABIRevision {
			t.Fatal("native metadata and accounting fill must attest the actual precompile emitter epoch")
		}
		retired, err := authority.client.CodeAtHash(ctx, common.HexToAddress(input.Options.Manifest.Roles["EventEmitter"].Address), common.HexToHash(fill.Position.BlockHash))
		if err != nil || len(retired) != 0 {
			t.Fatal("native producer capture must prove qualification with the retired EventEmitter code absent")
		}
	}
	bundle, err := authority.Bundle(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	sender, err := types.Sender(types.LatestSignerForChainID(domain), bundle.Transaction)
	if err != nil || fill.Attribution.TransactionSender != strings.ToLower(sender.Hex()) {
		t.Fatal("qualified sender does not match the admitted signed domain")
	}
	if tenant {
		envelopes, err := qualified.QualifiedEvents(ctx)
		if err != nil {
			t.Fatal(err)
		}
		boundActor := false
		for _, envelope := range envelopes {
			if envelope.EventName != "Swap" {
				continue
			}
			actor := envelope.Actors
			if actor == nil || actor.TransactionOrigin == nil || *actor.TransactionOrigin != strings.ToLower(sender.Hex()) || actor.Recipient == nil || *actor.Recipient != strings.ToLower(sender.Hex()) || actor.Beneficiary == nil || *actor.Beneficiary != *actor.Recipient || actor.AttributionMethod != "unique_pool_output_transfer" || actor.AttributionConfidence != "receipt_evidence" || len(actor.EvidenceIdentity) != 2 {
				t.Fatal("tenant actor consumer lost the signed origin or actual pool output transfer")
			}
			for _, evidence := range actor.EvidenceIdentity {
				if evidence.ChainID != 125 || evidence.BlockHash != fill.Position.BlockHash || evidence.TransactionHash != fill.Position.TransactionHash {
					t.Fatal("tenant actor evidence changed shared parent identity")
				}
			}
			boundActor = true
		}
		if !boundActor {
			t.Fatal("tenant economic envelope did not reach actor consumer")
		}
	}
	wire := repair53Delivery(t, fill)
	closeClient()
	authority, closeClient = repair53Open(t, ctx, input)
	if err = authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	replay, _ := repair53Fill(t, ctx, authority, hash)
	if !reflect.DeepEqual(fill, replay) {
		t.Fatal("restart changed source identity or economic amounts")
	}
	repair53ConsumeAfterRestart(t, ctx, wire, fill)
}

func TestRepair_53_native_swap_qualification(t *testing.T)        { repair53SwapRestart(t, false) }
func TestRepair_53_tenant_transaction_qualification(t *testing.T) { repair53SwapRestart(t, true) }

func TestRepair_53_migrated_role_epochs(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	bundle, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	want := crypto.Keccak256Hash(common.FromHex(migratedSurfaceRuntime)).Hex()
	checked := 0
	for _, original := range input.Options.Epochs {
		if original.Identity.Kind != "surface" {
			continue
		}
		active, err := authority.epoch(original.Deployment.Address, decode.Order{Block: bundle.Header.Number.Uint64()})
		if err != nil || active.Identity.RuntimeHash != want || !reflect.DeepEqual(active.Deployment, original.Deployment) {
			t.Fatalf("%s must retain its binding and attest the installed migration runtime: %+v: %v", original.Deployment.Role, active, err)
		}
		if _, err := authority.validateCode(ctx, active, bundleBlockHash(bundle)); err != nil {
			t.Fatalf("%s actual migrated runtime refused: %v", active.Deployment.Role, err)
		}
		if _, err := authority.validateCode(ctx, original, bundleBlockHash(bundle)); !errors.Is(err, ErrQualification) {
			t.Fatalf("%s obsolete runtime identity must fail exact code attestation: %v", original.Deployment.Role, err)
		}
		altered := original
		altered.Identity.RuntimeHash = common.HexToHash("0x01").Hex()
		if got := migratedRoleEpoch(input.Options.Manifest, altered); !reflect.DeepEqual(got, altered) {
			t.Fatal("unapproved role identity was normalized into an admitted execution epoch")
		}
		checked++
	}
	if checked != 8 {
		t.Fatalf("expected all eight configured migration role surfaces, checked %d", checked)
	}
}

func TestRepair_53_approved_history_rehydration(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	authority, closeClient := repair53Open(t, ctx, input)
	defer func() { closeClient() }()
	var market DiscoveredMarket
	for _, candidate := range input.Markets {
		if candidate.Height == 33963716 {
			market = candidate
		}
	}
	if market.Creation == nil || market.Height >= input.Options.Manifest.ActivationHeight {
		t.Fatal("packet must contain durable provenance for the approved pre-activation real creation")
	}
	historical, err := authority.historicalAuthority(ctx, market.Height)
	if err != nil {
		t.Fatal(err)
	}
	if err = historical.proveHistoricalMarket(ctx, market); err != nil {
		t.Fatal(err)
	}
	bundle, err := historical.Bundle(ctx, market.Creation.TransactionHash)
	if err != nil {
		t.Fatal(err)
	}
	_, receipts, err := historical.pinnedBlock(ctx, market.Creation.BlockHash)
	if err != nil {
		t.Fatal(err)
	}
	discovered, err := historical.discover(ctx, receipts)
	if err != nil {
		t.Fatal(err)
	}
	creation, err := Qualify(ctx, discovered.registry, discovered, bundle)
	if err != nil {
		t.Fatal(err)
	}
	events, err := creation.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var args map[string]any
	for _, event := range events {
		if event.EventName == "MarketCreated" && event.LogIndex == market.Creation.LogIndex && event.Args["pool"] == strings.ToLower(market.Pool.Hex()) {
			args = event.Args
		}
	}
	if args == nil {
		t.Fatal("qualified historical creation missing")
	}
	nft, ok := args["nftId"].(string)
	if !ok {
		t.Fatal("historical factory creation NFT identity missing")
	}
	payload, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := json.Marshal(market.Creation.Provenance)
	if err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(t.TempDir(), "canonical.sqlite")
	row := canonicalMarketRow{Token: strings.ToLower(market.Token.Hex()), Pool: strings.ToLower(market.Pool.Hex()), Height: int64(market.Height), TransactionIndex: int64(market.TransactionIndex), BlockHash: strings.ToLower(market.Creation.BlockHash.Hex()), TransactionHash: strings.ToLower(market.Creation.TransactionHash.Hex()), LogIndex: int64(market.Creation.LogIndex), Provenance: provenance}
	repair53Database(t, database, &row, payload, nft)
	persisted := repair53Database(t, database, nil, payload, nft)
	if !reflect.DeepEqual(persisted, []DiscoveredMarket{market}) {
		t.Fatal("canonical creation provenance changed on persistence")
	}
	if err = authority.Rehydrate(ctx, persisted); err != nil {
		t.Fatal(err)
	}
	first, _ := repair53Fill(t, ctx, authority, input.HistoricalSwap)
	height, err := strconv.ParseUint(first.Position.Height, 10, 64)
	if err != nil || height < input.Options.Manifest.ActivationHeight || first.PoolAddress != strings.ToLower(market.Pool.Hex()) || first.Provenance.DeploymentID != market.Creation.Provenance.DeploymentID {
		t.Fatal("test must resume the historical market across current activation without relabeling its generation")
	}
	closeClient()
	authority, closeClient = repair53Open(t, ctx, input)
	recovered := repair53Database(t, database, nil, payload, nft)
	if !reflect.DeepEqual(persisted, recovered) {
		t.Fatal("reopened canonical source identities changed")
	}
	if err = authority.Rehydrate(ctx, recovered); err != nil {
		t.Fatal(err)
	}
	second, _ := repair53Fill(t, ctx, authority, input.HistoricalSwap)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("reopening rewrote historical fill identity")
	}
	corrupt, err := clone(recovered)
	if err != nil {
		t.Fatal(err)
	}
	corrupt[0].Creation.Provenance.DeploymentID = input.Options.Manifest.DeploymentVersion
	if err = authority.Rehydrate(ctx, corrupt); err == nil {
		t.Fatal("historical creation relabeled as current generation was accepted")
	}
	corrupt, err = clone(recovered)
	if err != nil {
		t.Fatal(err)
	}
	corrupt[0].Creation.TransactionHash[0] ^= 1
	if err = authority.Rehydrate(ctx, corrupt); err == nil {
		t.Fatal("altered historical source transaction was accepted")
	}
}

func TestRepair_53_signed_domain_bounds(t *testing.T) {
	for _, n := range []uint64{125, virtualChainIDBase + 1, virtualChainIDBase + math.MaxUint32} {
		if !validSignedChainID(new(big.Int).SetUint64(n)) {
			t.Fatalf("valid domain %d rejected", n)
		}
	}
	for _, n := range []uint64{0, 1, 124, 126, virtualChainIDBase, virtualChainIDBase + math.MaxUint32 + 1, math.MaxUint64} {
		if validSignedChainID(new(big.Int).SetUint64(n)) {
			t.Fatalf("unmapped domain %d admitted", n)
		}
	}
	if validSignedChainID(new(big.Int).Lsh(big.NewInt(1), 80)) || validSignedChainID(big.NewInt(-125)) || validSignedChainID(nil) {
		t.Fatal("invalid signed chain ID admitted")
	}
	key, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []uint64{125, virtualChainIDBase + 1} {
		chain := new(big.Int).SetUint64(n)
		tx, err := types.SignTx(types.NewTransaction(0, common.HexToAddress("0x1017"), big.NewInt(0), 21000, big.NewInt(1), nil), types.LatestSignerForChainID(chain), key)
		if err != nil {
			t.Fatal(err)
		}
		sender, err := types.Sender(types.LatestSignerForChainID(chain), tx)
		if err != nil || sender != crypto.PubkeyToAddress(key.PublicKey) {
			t.Fatal("signed domain did not recover the real signer")
		}
		header := &types.Header{Number: big.NewInt(37220321), Time: 1792063125}
		bundle := Bundle{Header: header, Transaction: tx, Receipt: &types.Receipt{
			Type: tx.Type(), Status: types.ReceiptStatusSuccessful,
			TxHash: tx.Hash(), BlockHash: header.Hash(), BlockNumber: new(big.Int).Set(header.Number),
		}}
		qualifiedSender, err := coordinates(bundle)
		if err != nil || qualifiedSender != strings.ToLower(sender.Hex()) {
			t.Fatalf("receipt coordinates rejected signer for admitted domain %d: %q %v", n, qualifiedSender, err)
		}
		bundle.Receipt.BlockHash[0] ^= 1
		if _, err = coordinates(bundle); err == nil {
			t.Fatal("signed-domain support admitted a changed parent block identity")
		}
	}
}

func TestRepair_53_registry_abi_parity(t *testing.T) {
	raw, err := os.ReadFile("../../../../../precompiles/layerxchains/abi.json")
	if err != nil {
		t.Fatal(err)
	}
	producer, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := abi.JSON(bytes.NewReader(virtualChainABIJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(producer.Methods["getChain"], consumer.Methods["getChain"]) {
		t.Fatal("registry producer and receipt consumer ABI differ")
	}
	for _, name := range []string{"ChainCreated", "ChainStatusChanged"} {
		if !reflect.DeepEqual(producer.Events[name], consumer.Events[name]) {
			t.Fatalf("registry event %s ABI differs", name)
		}
	}
}

func TestRepair_53_history_epoch_transition(t *testing.T) {
	historical, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("approved historical deployment missing")
	}
	current := deployment.Current()
	anchor := BlockAnchor{Height: current.ActivationHeight, Hash: crypto.Keccak256Hash([]byte("epoch-document-identity"))}
	oldOptions := receiptManifestOptions(t, historical, anchor)
	options := receiptManifestOptions(t, current, anchor)
	var reviewed decode.Catalog
	if err := json.Unmarshal(options.CatalogJSON, &reviewed); err != nil {
		t.Fatal(err)
	}
	for _, contract := range reviewed.Contracts {
		if contract.Name == "SidioraERC20" {
			identity := historical.Roles["USDL"]
			template := RPCDiscovery{Role: contract.Name, ABIRevision: contract.ABIRevision, RuntimeHash: identity.RuntimeHash}
			oldOptions.Discovery = append(oldOptions.Discovery, template)
			options.Discovery = append(options.Discovery, template)
		}
		if contract.Name == "SidioraPool" {
			oldOptions.Discovery = append(oldOptions.Discovery, RPCDiscovery{Role: contract.Name, Kind: "beacon_proxy", ABIRevision: contract.ABIRevision, RuntimeHash: historical.Roles["PoolBeacon"].RuntimeHash})
			options.Discovery = append(options.Discovery, RPCDiscovery{Role: contract.Name, Kind: "surface", ABIRevision: contract.ABIRevision, RuntimeHash: crypto.Keccak256Hash(common.FromHex("0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3")).Hex()})
		}
	}
	options.History = []RPCGeneration{{ActivationHeight: historical.ActivationHeight, Epochs: oldOptions.Epochs, Discovery: oldOptions.Discovery}}
	currentHash, err := current.Hash()
	if err != nil {
		t.Fatal(err)
	}
	historicalHash, err := historical.Hash()
	if err != nil {
		t.Fatal(err)
	}
	authority := &RPCQualification{options: options, manifestHash: currentHash}
	generation := &RPCQualification{options: oldOptions, manifestHash: historicalHash}
	market := DiscoveredMarket{Token: common.HexToAddress("0x0F92FDD087c96dD3413eF98646b94D0d986192FA"), Pool: common.HexToAddress("0xCE20Da50F92D21d8BeE497910c782E9Aa25b6263"), Height: 33963716, Creation: &MarketCreationProvenance{Provenance: accounting.Provenance{DeploymentID: historical.DeploymentVersion}}}
	epochs, err := authority.rehydratedMarketEpochs(market, generation)
	if err != nil || len(epochs) != 4 {
		t.Fatalf("historical market epoch transition: %v, %+v", err, epochs)
	}
	for i := 0; i < len(epochs); i += 2 {
		original, active := epochs[i], epochs[i+1]
		if original.Deployment.From.Block != market.Height || original.Deployment.Until == nil || *original.Deployment.Until != active.Deployment.From || active.Deployment.From.Block != current.ActivationHeight || active.SourceActivationHeight != historical.ActivationHeight || active.Deployment.Until != nil {
			t.Fatalf("source or runtime epoch was relabeled: %+v %+v", original, active)
		}
		if active.Deployment.Role == "SidioraPool" && (original.Identity.Kind != "beacon_proxy" || active.Identity.Kind != "surface" || original.Identity.RuntimeHash == active.Identity.RuntimeHash) {
			t.Fatal("historical proxy was not separated from the native surface binding")
		}
		if active.Deployment.Role == "SidioraPool" && active.Identity.RuntimeHash != "0xe67f4b121cfa00710e9a74cd027b8d56f7ca67d14ebe357a8c080f8c2e819993" {
			t.Fatal("historical pool did not retain the exact migrated nonpayable surface identity")
		}
		unbounded := original
		unbounded.Deployment.Until = nil
		before, err := generation.codeEpochDigest(unbounded)
		if err != nil {
			t.Fatal(err)
		}
		after, err := authority.codeEpochDigest(original)
		if err != nil || before != after {
			t.Fatalf("activation rewrote original code provenance: %q != %q: %v", before, after, err)
		}
		binding, err := authority.epochManifest(active)
		if err != nil || binding.DeploymentVersion != current.DeploymentVersion {
			t.Fatalf("runtime binding lost current deployment: %v", err)
		}
		source, err := authority.sourceManifest(active)
		if err != nil || source.DeploymentVersion != historical.DeploymentVersion {
			t.Fatalf("original market generation lost: %v", err)
		}
	}
	// Preserve the derived epoch metadata across its serialization boundary.
	// The integration test above separately requires the canonical database
	// and producer receipts; this codec check does not replace that proof.
	options.Epochs = append(options.Epochs, epochs...)
	raw, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "receipt-options.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	authority = nil
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var recovered RPCOptions
	if err = json.Unmarshal(raw, &recovered); err != nil {
		t.Fatal(err)
	}
	authority = &RPCQualification{options: recovered, manifestHash: currentHash}
	poolEpoch, err := authority.epoch(market.Pool, decode.Order{Block: current.ActivationHeight})
	if err != nil || poolEpoch.Identity.Kind != "surface" {
		t.Fatalf("reopened document selected retired pool code: %+v %v", poolEpoch, err)
	}
	source, err := authority.sourceManifest(poolEpoch)
	if err != nil || source.DeploymentVersion != historical.DeploymentVersion {
		t.Fatalf("document codec rewrote source identity: %v", err)
	}
	for _, activation := range []uint64{historical.ActivationHeight - 1, current.ActivationHeight, current.ActivationHeight + 1} {
		corrupt := poolEpoch
		corrupt.SourceActivationHeight = activation
		if _, err = authority.sourceManifest(corrupt); err == nil {
			t.Fatalf("unapproved source generation %d admitted", activation)
		}
	}
	// Historical admission needs the original generation and its durable
	// creation; an arbitrary pre-activation height is never sufficient.
	for _, height := range []uint64{historical.ActivationHeight - 1, current.ActivationHeight} {
		corrupt := market
		corrupt.Height = height
		if _, err := authority.rehydratedMarketEpochs(corrupt, generation); err == nil {
			t.Fatalf("historical generation admitted invalid creation height %d", height)
		}
	}
	corrupt := market
	corrupt.Creation = nil
	if _, err := authority.rehydratedMarketEpochs(corrupt, generation); err == nil {
		t.Fatal("historical generation admitted a market without durable provenance")
	}
	if _, err := authority.rehydratedMarketEpochs(market, authority); err == nil {
		t.Fatal("current generation admitted an unproven pre-activation creation")
	}
}

func TestRepair_53_native_abi_contract(t *testing.T) {
	producerRaw, err := os.ReadFile("../../../../../precompiles/launchpad/abi.json")
	if err != nil {
		t.Fatal(err)
	}
	producer, err := abi.JSON(bytes.NewReader(producerRaw))
	if err != nil {
		t.Fatal(err)
	}
	options := receiptManifestOptions(t, deployment.Current(), BlockAnchor{})
	var reviewed decode.Catalog
	if err = json.Unmarshal(options.CatalogJSON, &reviewed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, contract := range reviewed.Contracts {
		if contract.Name != "EventEmitter" {
			continue
		}
		consumer, err := abi.JSON(bytes.NewReader(contract.ABI))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(producer.Events["Swap"], consumer.Events["Swap"]) {
			t.Fatal("native producer and receipt consumer Swap ABIs differ")
		}
		found = true
	}
	if !found {
		t.Fatal("reviewed EventEmitter ABI missing")
	}
}

// The archive is an unchanged capture from the real chain, including signed
// transactions, factory creation, proxy storage and pinned metadata reads.
// This supplements, and does not replace, the required post-activation proof.
func TestRepair_53_historical_archive_restart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	file, err := os.Open("testdata/repair53-historical.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	archive, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	input := repair53Inputs{Responses: map[string]repair53Recorded{}}
	scanner := bufio.NewScanner(archive)
	scanner.Buffer(make([]byte, 4096), 64<<20)
	for scanner.Scan() {
		var row repair53Recorded
		if err = json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		input.Responses[row.Key] = row
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	// The node's immutable exported Treasury state completes code reads the
	// original archive omitted; receipts and source coordinates stay unchanged.
	stateRaw, err := os.ReadFile("testdata/repair53-history-state.json")
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]repair53Recorded
	if err = json.Unmarshal(stateRaw, &state); err != nil {
		t.Fatal(err)
	}
	for key, row := range state {
		if _, exists := input.Responses[key]; exists {
			t.Fatalf("historical state overwrites archived read %s", key)
		}
		input.Responses[key] = row
	}
	// Index the same captured full blocks by hash and number. The old
	// archive recorded full blocks by height; reopening the real authority
	// also looks them up by their recorded hash. No block content changes.
	blockAliases := map[string]repair53Recorded{}
	for key, row := range input.Responses {
		if !strings.HasPrefix(key, "eth_getBlockByNumber ") || !strings.HasSuffix(key, ",true]") {
			continue
		}
		var identity struct {
			Hash   string `json:"hash"`
			Number string `json:"number"`
		}
		if err = json.Unmarshal(row.Result, &identity); err != nil || identity.Hash == "" || identity.Number == "" {
			t.Fatalf("captured full block identity: %v", err)
		}
		for _, alias := range []string{
			"eth_getBlockByHash [\"" + identity.Hash + "\",true]",
			"eth_getBlockByHash [\"" + identity.Hash + "\",false]",
			"eth_getBlockByNumber [\"" + identity.Number + "\",false]",
		} {
			if _, alreadyRecorded := input.Responses[alias]; !alreadyRecorded {
				blockAliases[alias] = repair53Recorded{Key: alias, Result: row.Result}
			}
		}
	}
	for key, row := range blockAliases {
		input.Responses[key] = row
	}
	manifest, approved := deployment.Approved(32226215)
	if !approved {
		t.Fatal("approved historical manifest absent")
	}
	anchor := BlockAnchor{Height: 33963715, Hash: common.HexToHash("0x82477110c5fc791f6bef5a7d96636dc457e83daf59c8b802e264bada02f13395")}
	input.Options = receiptManifestOptions(t, manifest, anchor)
	var epochCatalog decode.Catalog
	if err = json.Unmarshal(input.Options.CatalogJSON, &epochCatalog); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"SidioraNFT", "FeeAccumulator", "FeesRouter"} {
		for _, contract := range epochCatalog.Contracts {
			if contract.Name == role {
				identity := manifest.Roles[role]
				input.Options.Epochs = append(input.Options.Epochs, RPCCodeEpoch{Deployment: decode.Deployment{Address: common.HexToAddress(identity.Address), Role: role, ABIRevision: contract.ABIRevision, From: decode.Order{Block: manifest.ActivationHeight}}, Identity: identity})
			}
		}
	}

	input.Options.BlockIdentity = BlockIdentityReported
	catalogRaw, err := os.ReadFile("../../../../../sdk/launchpad/protocol/catalog/reviewed.json")
	if err != nil {
		t.Fatal(err)
	}
	var reviewed struct {
		Contracts []struct {
			Name        string                 `json:"name"`
			ABIRevision string                 `json:"abiSha256"`
			Immutables  map[string][]CodeRange `json:"immutableReferences"`
		}
	}
	if err = json.Unmarshal(catalogRaw, &reviewed); err != nil {
		t.Fatal(err)
	}
	pool := common.HexToAddress("0xce20da50f92d21d8bee497910c782e9aa25b6263")
	token := common.HexToAddress("0x0f92fdd087c96dd3413ef98646b94d0d986192fa")
	creationBlock := "0x81a18d822e327fbcbfe2be91b7f666bfa043016cb0e3f094c5bbabe91f9d192b"
	code := func(address common.Address) []byte {
		key := "eth_getCode [\"" + strings.ToLower(address.Hex()) + "\",{\"blockHash\":\"" + creationBlock + "\"}]"
		var encoded string
		if err := json.Unmarshal(input.Responses[key].Result, &encoded); err != nil {
			t.Fatalf("recorded code for %s: %v", address, err)
		}
		return common.FromHex(encoded)
	}
	for _, contract := range reviewed.Contracts {
		if contract.Name == "SidioraPool" {
			input.Options.Discovery = append(input.Options.Discovery, RPCDiscovery{Role: contract.Name, Kind: "beacon_proxy", ABIRevision: contract.ABIRevision, RuntimeHash: crypto.Keccak256Hash(code(pool)).Hex()})
		}
		if contract.Name == "SidioraERC20" {
			var ranges []CodeRange
			for _, spans := range contract.Immutables {
				ranges = append(ranges, spans...)
			}
			masked := code(token)
			for _, span := range ranges {
				if span.Start < 0 || span.Length < 1 || span.Start+span.Length > len(masked) {
					t.Fatal("reviewed immutable span exceeds captured runtime")
				}
				clear(masked[span.Start : span.Start+span.Length])
			}
			input.Options.Discovery = append(input.Options.Discovery, RPCDiscovery{Role: contract.Name, ABIRevision: contract.ABIRevision, RuntimeHash: crypto.Keccak256Hash(masked).Hex(), Immutables: ranges})
		}
	}
	input = repair53Replay(t, input)
	authority, closeClient := repair53Open(t, ctx, input)
	defer func() { closeClient() }()
	_, qualified, err := authority.FetchQualifiedBatch(ctx, 33963716, 33963716)
	if err != nil {
		t.Fatal(err)
	}
	var market DiscoveredMarket
	var payload []byte
	var nft string
	for _, q := range qualified {
		envelopes, err := q.QualifiedEvents(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, envelope := range envelopes {
			event := envelope.Event
			if envelope.EmitterRole != "SidioraFactory" || envelope.Raw.Emitter != strings.ToLower(manifest.Roles["SidioraFactory"].Address) || event == nil || event.EventName != "MarketCreated" || event.Args["pool"] != strings.ToLower(pool.Hex()) {
				continue
			}
			tx, _, _, err := q.SourceContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			bundle, err := authority.Bundle(ctx, common.HexToHash(tx))
			if err != nil {
				t.Fatal(err)
			}
			proof, err := q.SourceProvenance(ctx, common.HexToAddress(manifest.Roles["SidioraFactory"].Address))
			if err != nil {
				t.Fatal(err)
			}
			market = DiscoveredMarket{Token: token, Pool: pool, Height: 33963716, TransactionIndex: bundle.Receipt.TransactionIndex, Creation: &MarketCreationProvenance{BlockHash: bundleBlockHash(bundle), TransactionHash: bundle.Transaction.Hash(), LogIndex: event.LogIndex, Provenance: proof}}
			payload, err = json.Marshal(event.Args)
			if err != nil {
				t.Fatal(err)
			}
			nft, _ = event.Args["nftId"].(string)
		}
	}
	if market.Creation == nil || nft == "" {
		t.Fatal("captured historical creation did not qualify")
	}
	provenance, err := json.Marshal(market.Creation.Provenance)
	if err != nil {
		t.Fatal(err)
	}
	row := canonicalMarketRow{Token: strings.ToLower(token.Hex()), Pool: strings.ToLower(pool.Hex()), Height: int64(market.Height), TransactionIndex: int64(market.TransactionIndex), BlockHash: strings.ToLower(market.Creation.BlockHash.Hex()), TransactionHash: strings.ToLower(market.Creation.TransactionHash.Hex()), LogIndex: int64(market.Creation.LogIndex), Provenance: provenance}
	path := filepath.Join(t.TempDir(), "canonical.sqlite")
	repair53Database(t, path, &row, payload, nft)
	closeClient()
	authority, closeClient = repair53Open(t, ctx, input)
	recovered := repair53Database(t, path, nil, payload, nft)
	if !reflect.DeepEqual(recovered, []DiscoveredMarket{market}) {
		t.Fatal("database restart changed authentic canonical creation")
	}
	if err = authority.proveHistoricalMarket(ctx, recovered[0]); err != nil {
		t.Fatalf("reopened source rejected original receipt provenance: %v", err)
	}
	if err = authority.Rehydrate(ctx, recovered); err != nil {
		t.Fatalf("reopened source rejected original market binding: %v", err)
	}
}

func TestRepair_53_historical_emitter_epoch(t *testing.T) {
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("approved generation missing")
	}
	options := receiptManifestOptions(t, manifest, BlockAnchor{})
	epochs, err := historicalEmitterEpochs(manifest, options.CatalogJSON)
	if err != nil || len(epochs) != 1 {
		t.Fatalf("historical source authority: %+v %v", epochs, err)
	}
	epoch := epochs[0]
	if epoch.Deployment.Role != "Treasury" || epoch.Deployment.From.Block != manifest.ActivationHeight || epoch.Identity.Kind != "erc1967" || epoch.Identity.Address != deployment.Current().Roles["Treasury"].Address || epoch.Identity.RuntimeHash == deployment.Current().Roles["Treasury"].RuntimeHash {
		t.Fatal("historical Treasury inherited the successor's native code authority")
	}
	changed := manifest
	changed.SourceManifestSHA256 = strings.Repeat("0", 64)
	if _, err = historicalEmitterEpochs(changed, options.CatalogJSON); err == nil {
		t.Fatal("unapproved historical source document admitted")
	}
	var catalog decode.Catalog
	if err = json.Unmarshal(options.CatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	catalog.ManifestSHA256 = strings.Repeat("0", 64)
	raw, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = historicalEmitterEpochs(manifest, raw); err == nil {
		t.Fatal("ABI catalog from another deployment admitted")
	}
}
