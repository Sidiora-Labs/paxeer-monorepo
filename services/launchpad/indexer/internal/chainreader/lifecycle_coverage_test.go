package chainreader

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestHistoricalScalarAndIdentityBoundsOnActualChain(t *testing.T) {
	c := actualAdversarialChain(t)
	token, _, _, created := c.deploy(t, "SidioraERC20", "Ledger", "USDL", big.NewInt(1000000), c.owner)
	code, err := c.client.CodeAtHash(c.ctx, token, created.BlockHash)
	if err != nil {
		t.Fatal(err)
	}
	d := HistoricalDeployment{Address: token, Role: "SidioraERC20", DeploymentEpoch: created.TxHash.Hex(), Kind: "direct", CodeDigest: crypto.Keccak256Hash(code)}
	reader, err := NewHistoricalReader(c.ctx, c.client)
	if err != nil {
		t.Fatal(err)
	}
	unit, result, err := reader.USDLUnit(c.ctx, d, created.BlockHash, token)
	if err != nil || unit == nil || unit.Decimals != 6 || unit.Symbol != "USDL" || unit.Address == nil || *unit.Address != strings.ToLower(token.Hex()) || result.Status != "available" {
		t.Fatalf("USDL identity %+v %+v %v", unit, result, err)
	}
	for _, approved := range []common.Address{{}, c.owner} {
		if unit, result, err = reader.USDLUnit(c.ctx, d, created.BlockHash, approved); err == nil || unit != nil || result.Status != "unavailable" {
			t.Fatalf("unapproved USDL %+v %v", unit, err)
		}
	}
	registryAddress, registry, _, registryDeployment := c.proxy(t, "PoolRegistry", common.Address{}, c.owner)
	c.transact(t, registry, "grantRole", crypto.Keccak256Hash([]byte("FACTORY_ROLE")), c.owner)
	registered := c.transact(t, registry, "register", token, token, c.owner, common.Address{}, big.NewInt(17))
	result, err = reader.NFTIDByPool(c.ctx, registryDeployment, registered.BlockHash, token)
	if err != nil || result.Value == nil || *result.Value != "17" || result.Contract != strings.ToLower(registryAddress.Hex()) {
		t.Fatalf("registry NFT %+v %v", result, err)
	}
	result, err = reader.NFTIDByPool(c.ctx, registryDeployment, registered.BlockHash, c.owner)
	if err == nil || result.Value != nil || result.Failure != "pool NFT absent" {
		t.Fatalf("absent pool %+v %v", result, err)
	}
	if _, err = reader.NFTIDByPool(c.ctx, d, registered.BlockHash, common.Address{}); err == nil {
		t.Fatal("unbound registry query accepted")
	}
	for _, runtime := range [][]byte{{0x61, 0x01, 0x00, 0x60, 0, 0x52, 0x60, 0x20, 0x60, 0, 0xf3}, {0x60, 0x12, 0x60, 0, 0x52, 0x60, 0x20, 0x60, 0, 0xf3}, {0x60, 0, 0x60, 0, 0xf3}} {
		address, receipt := deployRuntime(t, c, runtime)
		hostile := HistoricalDeployment{Address: address, Role: "SidioraERC20", DeploymentEpoch: receipt.TxHash.Hex(), Kind: "direct", CodeDigest: crypto.Keccak256Hash(runtime)}
		result, err = reader.Decimals(c.ctx, hostile, receipt.BlockHash)
		if runtime[0] == 0x60 && runtime[1] == 0x12 {
			if err != nil || result.Value == nil || *result.Value != "18" {
				t.Fatalf("non-USDL decimals %+v %v", result, err)
			}
			if unit, _, err = reader.USDLUnit(c.ctx, hostile, receipt.BlockHash, address); err == nil || unit != nil {
				t.Fatal("non-six-decimal settlement admitted")
			}
		} else if err == nil || result.Status != "unavailable" || result.Value != nil {
			t.Fatalf("invalid scalar %+v %v", result, err)
		}
	}
	for _, tc := range []struct {
		name string
		edit func(*HistoricalDeployment)
	}{
		{"role", func(d *HistoricalDeployment) { d.Role = "SidioraNFT" }},
		{"direct implementation", func(d *HistoricalDeployment) { d.Implementation = c.owner }},
		{"kind", func(d *HistoricalDeployment) { d.Kind = "unknown" }},
		{"proxy missing implementation", func(d *HistoricalDeployment) { d.Kind = "erc1967" }},
		{"code", func(d *HistoricalDeployment) { d.CodeDigest = common.HexToHash("0x1") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := d
			tc.edit(&changed)
			if err := reader.verifyCode(c.ctx, changed, created.BlockHash); err == nil {
				t.Fatal("unbound deployment accepted")
			}
		})
	}
	changed := registryDeployment
	changed.Implementation = c.owner
	if err = reader.verifyCode(c.ctx, changed, registered.BlockHash); err == nil {
		t.Fatal("different proxy implementation admitted")
	}
	changed = registryDeployment
	changed.ImplementationDigest = common.HexToHash("0x1")
	if err = reader.verifyCode(c.ctx, changed, registered.BlockHash); err == nil {
		t.Fatal("different implementation bytecode admitted")
	}
	for _, tc := range []struct {
		d    HistoricalDeployment
		hash common.Hash
		args []byte
	}{{d, created.BlockHash, make([]byte, 33)}, {HistoricalDeployment{}, created.BlockHash, nil}, {d, common.Hash{}, nil}} {
		if out, err := reader.read(c.ctx, tc.d, tc.hash, "decimals()", tc.args); err == nil || out.Status != "unavailable" || out.Failure == "" {
			t.Fatalf("unbounded read %+v %v", out, err)
		}
	}
	if _, err = reader.verifyCachedDecimals(c.ctx, d, common.Hash{}, "6"); err == nil {
		t.Fatal("cache without exact block admitted")
	}
	canceled, cancel := context.WithCancel(c.ctx)
	cancel()
	for i := 0; i < cap(reader.calls); i++ {
		reader.calls <- struct{}{}
	}
	if _, err = reader.read(canceled, d, created.BlockHash, "decimals()", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled admission %v", err)
	}
	if _, err = reader.verifyCachedDecimals(canceled, d, created.BlockHash, "6"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled cached admission %v", err)
	}
	for len(reader.calls) > 0 {
		<-reader.calls
	}
	if err = (*HistoricalReader)(nil).acquire(c.ctx); err == nil {
		t.Fatal("absent admission accepted")
	}
	reader.mu.Lock()
	for i := 0; i < 4096; i++ {
		reader.decimals[new(big.Int).SetInt64(int64(i)).String()] = &decimalRead{done: make(chan struct{})}
	}
	reader.mu.Unlock()
	changed = d
	changed.DeploymentEpoch = "new-epoch"
	if out, err := reader.Decimals(c.ctx, changed, created.BlockHash); err == nil || out.Failure != "decimal cache capacity reached" {
		t.Fatalf("unbounded decimals %+v %v", out, err)
	}
	for _, endpoint := range []string{"ftp://localhost", "http:///", "http://127.0.0.1/#fragment", "%"} {
		if r, err := NewBoundedHistoricalReader(c.ctx, endpoint); err == nil {
			r.Close()
			t.Fatalf("invalid endpoint %q", endpoint)
		}
	}
	reader.Close()
	(*HistoricalReader)(nil).Close()
}

func recordedCreationResolver(t *testing.T, mutate func(string, json.RawMessage) json.RawMessage) (*CreationResolver, common.Hash, common.Address) {
	t.Helper()
	file, err := os.Open("../internaltest/chainfixture/testdata/paxeer125-market.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer zipped.Close()
	scanner := bufio.NewScanner(zipped)
	scanner.Buffer(make([]byte, 4096), 64<<20)
	var hash common.Hash
	for scanner.Scan() {
		var entry struct {
			Key    string          `json:"k"`
			Result json.RawMessage `json:"result"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(entry.Key, "eth_getTransactionReceipt ") {
			continue
		}
		var r struct {
			BlockNumber     string
			TransactionHash common.Hash
		}
		if err = json.Unmarshal(entry.Result, &r); err != nil {
			t.Fatal(err)
		}
		if r.BlockNumber == "0x2063ec4" {
			hash = r.TransactionHash
			break
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if hash == (common.Hash{}) {
		t.Fatal("recorded market creation absent")
	}
	f := chainfixture.New(t)
	f.Mutate = mutate
	client, err := ethclient.DialContext(context.Background(), recordedHeaderEndpoint(t, f))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	catalog, err := os.ReadFile("../../../../../sdk/launchpad/protocol/contracts/v1/chain/abi-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var c decode.Catalog
	if err = json.Unmarshal(catalog, &c); err != nil {
		t.Fatal(err)
	}
	var factory common.Address
	var deployments []decode.Deployment
	// The recorded factory address is carried by the native creation log.
	receipt, err := client.TransactionReceipt(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	for _, log := range receipt.Logs {
		if len(log.Topics) > 0 && log.Topics[0] == marketCreatedFactoryTopic {
			factory = log.Address
		}
	}
	for _, contract := range c.Contracts {
		if contract.Name == "SidioraFactory" {
			deployments = append(deployments, decode.Deployment{Address: factory, Role: contract.Name, ABIRevision: contract.ABIRevision})
		}
	}
	registry, err := decode.NewRegistry(catalog, deployments)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewCreationResolver(context.Background(), client, registry)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := CreationFromReceipt(registry, receipt, hash)
	if err != nil {
		t.Fatal(err)
	}
	return resolver, hash, identity.Creator
}

func TestCreationResolverPinsRecordedReceiptAndCreator(t *testing.T) {
	resolver, hash, creator := recordedCreationResolver(t, nil)
	for _, expected := range []*common.Address{nil, &creator} {
		out, err := resolver.Resolve(context.Background(), 125, hash.Hex(), expected)
		if err != nil || out.Status != "ready" || out.Token == nil || out.Pool == nil || out.Creator == nil || *out.Creator != strings.ToLower(creator.Hex()) || out.NFTID == nil || out.EvidenceLogIndex == nil {
			t.Fatalf("recorded creation %+v %v", out, err)
		}
	}
	wrong := common.HexToAddress("0x123")
	for _, expected := range []*common.Address{&wrong, new(common.Address)} {
		out, err := resolver.Resolve(context.Background(), 125, hash.Hex(), expected)
		if err == nil || out.Status != "integrity_error" {
			t.Fatalf("wrong creator %+v %v", out, err)
		}
	}
	if _, err := resolver.Resolve(context.Background(), 1, hash.Hex(), nil); err == nil {
		t.Fatal("foreign chain resolved")
	}
	if out, err := (*CreationResolver)(nil).Resolve(context.Background(), 125, hash.Hex(), nil); err == nil || out.Status != "unavailable" {
		t.Fatalf("absent resolver %+v %v", out, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := resolver.Resolve(canceled, 125, hash.Hex(), nil); err == nil || out.Failure != "receipt_rpc" {
		t.Fatalf("cancelled resolution %+v %v", out, err)
	}
	for _, tc := range []struct {
		name   string
		edit   func(map[string]json.RawMessage)
		valid  bool
		status string
	}{
		{"failed", func(m map[string]json.RawMessage) { m["status"] = json.RawMessage(`"0x0"`) }, true, "failed"},
		{"identity", func(m map[string]json.RawMessage) { m["transactionHash"], _ = json.Marshal(common.HexToHash("0x1")) }, false, "integrity_error"},
		{"header", func(m map[string]json.RawMessage) { m["hash"], _ = json.Marshal(common.HexToHash("0x1")) }, false, "integrity_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := chainfixture.New(t)
			f.Mutate = func(key string, raw json.RawMessage) json.RawMessage {
				if tc.name == "header" && !strings.HasPrefix(key, "eth_getBlockByNumber ") || tc.name != "header" && !strings.HasPrefix(key, "eth_getTransactionReceipt ") {
					return raw
				}
				var m map[string]json.RawMessage
				if err := json.Unmarshal(raw, &m); err != nil {
					t.Error(err)
					return raw
				}
				tc.edit(m)
				body, err := json.Marshal(m)
				if err != nil {
					t.Error(err)
					return raw
				}
				return body
			}
			client, err := ethclient.DialContext(context.Background(), recordedHeaderEndpoint(t, f))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			changed, err := NewCreationResolver(context.Background(), client, resolver.registry)
			if err != nil {
				t.Fatal(err)
			}
			out, err := changed.Resolve(context.Background(), 125, hash.Hex(), nil)
			if (err == nil) != tc.valid || out.Status != tc.status {
				t.Fatalf("altered creation %+v %v", out, err)
			}
		})
	}
}

func recordedHeaderEndpoint(t *testing.T, f *chainfixture.Fixture) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var call struct {
			Method string
			Params []json.RawMessage
		}
		if err = json.Unmarshal(body, &call); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[1]) == "false" {
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(body, &fields); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			call.Params[1] = json.RawMessage("true")
			fields["params"], err = json.Marshal(call.Params)
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			body, err = json.Marshal(fields)
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		f.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}
