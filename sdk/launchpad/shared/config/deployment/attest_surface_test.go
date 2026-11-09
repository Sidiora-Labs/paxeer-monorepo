package deployment

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// keeperSurfaceRuntime reads the keeper declaration without importing the root
// Cosmos module into this independent SDK module.
func keeperSurfaceRuntime(t *testing.T) []byte {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "../../../../../modules/launchpad/keeper/market.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var code []byte
	ast.Inspect(file, func(node ast.Node) bool {
		decl, ok := node.(*ast.ValueSpec)
		if !ok || len(decl.Names) != 1 || decl.Names[0].Name != "SurfaceRuntime" {
			return true
		}
		if len(decl.Values) != 1 {
			t.Fatal("keeper SurfaceRuntime must have one initializer")
		}
		call, ok := decl.Values[0].(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			t.Fatal("keeper SurfaceRuntime must be a hex declaration")
		}
		fun, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || fun.Sel.Name != "FromHex" {
			t.Fatal("keeper SurfaceRuntime must use FromHex")
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			t.Fatal("keeper SurfaceRuntime must be a string literal")
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		code, err = hexutil.Decode(value)
		if err != nil {
			t.Fatal(err)
		}
		return false
	})
	if len(code) == 0 {
		t.Fatal("keeper SurfaceRuntime declaration missing or empty")
	}
	return code
}

const surfaceRuntimeHash = "0xe67f4b121cfa00710e9a74cd027b8d56f7ca67d14ebe357a8c080f8c2e819993"

// attestedSurfaceManifest preserves canonical identities and uses isolated USDL code.
func attestedSurfaceManifest(t *testing.T, usdlCode []byte) Manifest {
	t.Helper()
	m := Current()
	m.Status = "active"
	m.ActivationHeight = 1
	r := m.Roles["USDL"]
	r.RuntimeHash = crypto.Keccak256Hash(usdlCode).Hex()
	m.Roles["USDL"] = r
	if err := m.Validate(); err != nil {
		t.Fatalf("surface manifest invalid: %v", err)
	}
	return m
}

type rpcBackend struct {
	mu     sync.Mutex
	codes  map[common.Address][]byte
	store  map[common.Address]map[common.Hash]common.Hash
	calls  map[string][]byte
	header *types.Header
	asked  map[common.Address]bool
}

func (b *rpcBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	addr := func(i int) common.Address {
		var s string
		if len(req.Params) > i {
			_ = json.Unmarshal(req.Params[i], &s)
		}
		return common.HexToAddress(s)
	}
	var result any
	switch req.Method {
	case "eth_chainId":
		result = hexutil.EncodeBig(big.NewInt(ChainID))
	case "eth_getBlockByHash":
		result = b.header
	case "eth_getCode":
		a := addr(0)
		b.asked[a] = true
		result = hexutil.Bytes(b.codes[a])
	case "eth_getStorageAt":
		var key string
		_ = json.Unmarshal(req.Params[1], &key)
		v := b.store[addr(0)][common.HexToHash(key)]
		result = hexutil.Bytes(v.Bytes())
	case "eth_call":
		var call struct {
			Data  string `json:"data"`
			Input string `json:"input"`
		}
		_ = json.Unmarshal(req.Params[0], &call)
		data := call.Input
		if data == "" {
			data = call.Data
		}
		word, ok := b.calls[strings.ToLower(strings.TrimPrefix(data, "0x"))]
		if !ok {
			word = common.LeftPadBytes([]byte{1}, 32)
		}
		result = hexutil.Bytes(word)
	default:
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(req.ID)+`,"error":{"code":-32601,"message":"method not found"}}`)
		return
	}
	out, _ := json.Marshal(result)
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(req.ID)+`,"result":`+string(out)+`}`)
}

func attestSurface(t *testing.T, mutate func(*rpcBackend, Manifest)) (ActivationAttestation, Manifest, Policy, *rpcBackend) {
	t.Helper()
	usdlCode := common.FromHex("0x6080604052600080fd")
	m := attestedSurfaceManifest(t, usdlCode)
	surfaceRuntime := keeperSurfaceRuntime(t)
	f := fixtures(t)
	b := &rpcBackend{
		codes: map[common.Address][]byte{},
		store: map[common.Address]map[common.Hash]common.Hash{},
		calls: map[string][]byte{
			common.Bytes2Hex(selector("owner()")):          addrWord(m.Governance.ExpectedBeaconOwner).Bytes(),
			common.Bytes2Hex(selector("pendingOwner()")):   make([]byte, 32),
			common.Bytes2Hex(selector("implementation()")): addrWord(m.Roles["PoolBeacon"].ImplementationAddress).Bytes(),
			common.Bytes2Hex(selector("creationFee()")):    common.LeftPadBytes(big.NewInt(1000000).Bytes(), 32),
			common.Bytes2Hex(selector("usdlAddress()")):    addrWord(m.Roles["USDL"].Address).Bytes(),
			common.Bytes2Hex(selector("decimals()")):       common.LeftPadBytes([]byte{6}, 32),
		},
		asked: map[common.Address]bool{},
	}
	for name, r := range m.Roles {
		addr := common.HexToAddress(r.Address)
		impl := common.HexToAddress(r.ImplementationAddress)
		switch r.Kind {
		case "surface":
			b.codes[addr] = surfaceRuntime
		case "erc1967":
			b.codes[addr] = runtime(f, "UUPSProxy", addr)
			b.codes[impl] = runtime(f, name, impl)
			b.store[addr] = map[common.Hash]common.Hash{ImplementationSlot: addrWord(r.ImplementationAddress)}
		case "beacon":
			b.codes[addr] = runtime(f, "PoolBeacon", addr)
			b.codes[impl] = runtime(f, "SidioraPool", impl)
		case "direct":
			b.codes[addr] = usdlCode
		}
	}
	h := &types.Header{Number: big.NewInt(100), Time: uint64(time.Now().Unix()), Difficulty: big.NewInt(0), GasLimit: 30000000}
	b.header = h
	if mutate != nil {
		mutate(b, m)
	}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	c, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	p := Policy{Now: time.Unix(int64(h.Time), 0).Add(time.Second), MaxAge: time.Minute, Timeout: 10 * time.Second}
	a, err := Attest(context.Background(), Reader(c), m, h.Hash(), p)
	if err != nil {
		t.Fatalf("Attest: %v", err)
	}
	return a, m, p, b
}

func TestAttestSurfaceAndPrecompileRoles(t *testing.T) {
	surfaceRuntime := keeperSurfaceRuntime(t)
	if got := crypto.Keccak256Hash(surfaceRuntime).Hex(); got != surfaceRuntimeHash {
		t.Fatalf("surface runtime hash = %s, want %s", got, surfaceRuntimeHash)
	}
	surfaces := 0
	for name, role := range Current().Roles {
		if role.Kind == "surface" {
			surfaces++
			if role.RuntimeHash != crypto.Keccak256Hash(surfaceRuntime).Hex() {
				t.Fatalf("canonical %s surface pin differs from keeper: %s", name, role.RuntimeHash)
			}
		}
	}
	if surfaces != 9 {
		t.Fatalf("canonical surface inventory = %d, want 9", surfaces)
	}
	t.Run("identified", func(t *testing.T) {
		a, m, p, b := attestSurface(t, nil)
		if len(a.Issues) != 0 || !a.Eligible {
			t.Fatalf("issues = %v eligible = %v", a.Issues, a.Eligible)
		}
		for name := range m.Roles {
			id, role := a.Identities[name], m.Roles[name]
			if !id.Verified || !strings.EqualFold(id.Address, role.Address) || id.RuntimeHash != role.RuntimeHash || id.ImplementationHash != role.ImplementationHash {
				t.Fatalf("%s identity differs from approved manifest: %+v", name, id)
			}
			if hasImplementation(role.Kind) && !strings.EqualFold(id.ImplementationAddress, role.ImplementationAddress) {
				t.Fatalf("%s implementation differs from approved manifest: %+v", name, id)
			}
		}
		for _, name := range []string{"Router", LaunchpadPrecompileRole} {
			id := a.Identities[name]
			if id.ImplementationAddress != "" || id.ImplementationHash != "" || id.BeaconAddress != "" {
				t.Fatalf("%s carries implementation identity: %+v", name, id)
			}
		}
		if a.Identities["Router"].RuntimeHash != surfaceRuntimeHash {
			t.Fatalf("Router runtime hash = %s", a.Identities["Router"].RuntimeHash)
		}
		if a.Identities["EventEmitter"].ImplementationAddress != common.HexToAddress(m.Roles["EventEmitter"].ImplementationAddress).Hex() {
			t.Fatalf("EventEmitter implementation = %s", a.Identities["EventEmitter"].ImplementationAddress)
		}
		if b.asked[common.HexToAddress(LaunchpadPrecompileAddress)] {
			t.Fatal("Attest read code at the precompile address")
		}
		if err := a.ValidateFor(m, p); err != nil {
			t.Fatalf("ValidateFor: %v", err)
		}
		if err := a.ValidateFamilyFor(m, p, []string{"Router", LaunchpadPrecompileRole, "EventEmitter"}, true); err != nil {
			t.Fatalf("ValidateFamilyFor: %v", err)
		}
	})
	t.Run("surface holds other code", func(t *testing.T) {
		a, m, p, _ := attestSurface(t, func(b *rpcBackend, m Manifest) {
			b.codes[common.HexToAddress(m.Roles["Router"].Address)] = common.FromHex("0x6080604052600080fd")
		})
		if !hasIssue(a, "unknown_runtime_hash", "Router") || a.Identities["Router"].Verified || a.Eligible {
			t.Fatalf("other code at a surface was accepted: issues = %v", a.Issues)
		}
		if err := a.ValidateFamilyFor(m, p, []string{"Router"}, false); err == nil {
			t.Fatal("ValidateFamilyFor accepted an unidentified surface")
		}
	})
	for _, tc := range []struct {
		name, role, issue string
		mutate            func(*rpcBackend, Manifest)
	}{
		{"proxy implementation code changed", "EventEmitter", "unknown_implementation_hash", func(b *rpcBackend, m Manifest) {
			b.codes[common.HexToAddress(m.Roles["EventEmitter"].ImplementationAddress)] = []byte{0x00}
		}},
		{"proxy implementation address changed", "OpticalRegistry", "implementation_mismatch", func(b *rpcBackend, m Manifest) {
			b.store[common.HexToAddress(m.Roles["OpticalRegistry"].Address)][ImplementationSlot] = addrWord(m.Roles["EventEmitter"].ImplementationAddress)
		}},
		{"beacon runtime changed", "PoolBeacon", "unknown_runtime_hash", func(b *rpcBackend, m Manifest) {
			b.codes[common.HexToAddress(m.Roles["PoolBeacon"].Address)] = []byte{0x00}
		}},
		{"beacon implementation changed", "PoolBeacon", "implementation_mismatch", func(b *rpcBackend, m Manifest) {
			b.calls[common.Bytes2Hex(selector("implementation()"))] = addrWord(m.Roles["EventEmitter"].ImplementationAddress).Bytes()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, m, p, _ := attestSurface(t, tc.mutate)
			if a.Eligible || a.Identities[tc.role].Verified || !hasIssue(a, tc.issue, tc.role) {
				t.Fatalf("changed %s identity accepted: %+v", tc.role, a)
			}
			if err := a.ValidateFor(m, p); err == nil {
				t.Fatal("ValidateFor accepted changed identity")
			}
			if err := a.ValidateFamilyFor(m, p, []string{tc.role}, false); err == nil {
				t.Fatal("ValidateFamilyFor accepted changed identity")
			}
		})
	}
	t.Run("precompile role off the precompile address", func(t *testing.T) {
		a, m, _, _ := attestSurface(t, nil)
		r := m.Roles[LaunchpadPrecompileRole]
		r.Address = m.Roles["Router"].Address
		if runtimeIdentity(r, a.Identities[LaunchpadPrecompileRole]) {
			t.Fatal("precompile identity accepted at a non-precompile address")
		}
	})
}
