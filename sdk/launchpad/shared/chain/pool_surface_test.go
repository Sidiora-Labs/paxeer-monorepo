package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// keeperSurfaceRuntime reads the keeper declaration without importing the root
// Cosmos module into this independent SDK module.
func keeperSurfaceRuntime(t *testing.T) []byte {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "../../../../modules/launchpad/keeper/market.go", nil, 0)
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

func TestPoolSurfaceIdentity(t *testing.T) {
	manifest := deployment.Current()
	hashes := surfaceRuntimeHash(manifest)
	stagedRuntime := keeperSurfaceRuntime(t)
	if got := crypto.Keccak256Hash(stagedRuntime); got.Hex() != deployment.StagedSurfaceRuntimeHash {
		t.Fatalf("staged surface runtime hash %s, constant %s", got.Hex(), deployment.StagedSurfaceRuntimeHash)
	}
	if _, ok := hashes[crypto.Keccak256Hash(stagedRuntime)]; !ok || len(hashes) != 2 {
		t.Fatalf("keeper surface runtime %s absent from active and staged set: %v", crypto.Keccak256Hash(stagedRuntime), hashes)
	}
	artifactBytes, err := os.ReadFile("../config/deployment/testdata/evm-artifacts.json")
	if err != nil {
		t.Fatal(err)
	}
	var artifacts struct {
		Contracts map[string]struct {
			Runtime string `json:"runtime"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(artifactBytes, &artifacts); err != nil {
		t.Fatal(err)
	}
	surfaceRuntime, err := hexutil.Decode(artifacts.Contracts["SurfaceRuntime"].Runtime)
	if err != nil || len(surfaceRuntime) == 0 {
		t.Fatalf("committed surface runtime missing: %v", err)
	}
	for name, role := range manifest.Roles {
		if role.Kind == "surface" && (common.HexToHash(role.RuntimeHash) != crypto.Keccak256Hash(surfaceRuntime) || common.HexToHash(role.StagedRuntimeHash) != crypto.Keccak256Hash(stagedRuntime) || role.StagedUpgrade != "v7.0") {
			t.Fatalf("canonical %s active or staged surface pin differs: %+v", name, role)
		}
	}
	proxyCode, err := hexutil.Decode(artifacts.Contracts["BeaconProxy"].Runtime)
	if err != nil || len(proxyCode) == 0 {
		t.Fatalf("approved BeaconProxy runtime missing: %v", err)
	}
	registry := common.HexToAddress("0x00000000000000000000000000000000000000a1")
	beacon := common.HexToAddress("0x00000000000000000000000000000000000000b1")
	pool := common.HexToAddress("0x00000000000000000000000000000000000000c1")
	header := testHeader(7)
	cases := []struct {
		name         string
		registered   bool
		code         []byte
		want         string
		wrongBeacon  bool
		storageReads int
		activeOnly   bool
	}{
		{name: "registered surface", registered: true, code: surfaceRuntime},
		{name: "registered staged surface", registered: true, code: stagedRuntime},
		{name: "active-only manifest admits live surface", registered: true, code: surfaceRuntime, activeOnly: true},
		{name: "active-only manifest refuses staged surface", registered: true, code: stagedRuntime, activeOnly: true, want: "pool proxy runtime identity mismatch"},
		{name: "unregistered surface", code: surfaceRuntime, want: "pool not registered at selected block"},
		{name: "unregistered staged surface", code: stagedRuntime, want: "pool not registered at selected block"},
		{name: "registered other code", registered: true, code: []byte{0x60, 0x00, 0x60, 0x00, 0xf3}, want: "pool proxy runtime identity mismatch"},
		{name: "registered empty code", registered: true, want: "pool proxy runtime identity mismatch"},
		{name: "registered beacon proxy", registered: true, code: proxyCode, storageReads: 1},
		{name: "unregistered beacon proxy", code: proxyCode, want: "pool not registered at selected block"},
		{name: "registered proxy with wrong beacon", registered: true, code: proxyCode, wrongBeacon: true, storageReads: 1, want: "pool beacon identity mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var storageReads, membershipReads, codeReads int
			registryABI, err := bindings.PoolRegistryMetaData.GetAbi()
			if err != nil {
				t.Fatal(err)
			}
			membershipData, err := registryABI.Pack("isRegisteredPool", pool)
			if err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     json.RawMessage
					Method string
					Params []json.RawMessage
				}
				json.NewDecoder(r.Body).Decode(&req)
				var result any
				switch req.Method {
				case "eth_chainId":
					result = "0x7d"
				case "eth_getBlockByNumber":
					result = reportedJSON(header)
				case "eth_call":
					membershipReads++
					var call struct {
						To    common.Address
						Data  hexutil.Bytes
						Input hexutil.Bytes
					}
					if err := json.Unmarshal(req.Params[0], &call); err != nil {
						t.Error(err)
					}
					payload := call.Data
					if len(payload) == 0 {
						payload = call.Input
					}
					if !bytes.Equal(payload, membershipData) {
						t.Errorf("membership call = %x, want %x", payload, membershipData)
					}
					if call.To != registry {
						t.Error("membership read not addressed to the PoolRegistry", call.To)
					}
					word := make([]byte, 32)
					if tc.registered {
						word[31] = 1
					}
					result = hexutil.Encode(word)
				case "eth_getCode":
					codeReads++
					var address common.Address
					if err := json.Unmarshal(req.Params[0], &address); err != nil {
						t.Error(err)
					}
					if address != pool {
						t.Error("identity code read not addressed to pool", address)
					}
					result = hexutil.Encode(tc.code)
				case "eth_getStorageAt":
					storageReads++
					var address common.Address
					var key common.Hash
					if err := json.Unmarshal(req.Params[0], &address); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal(req.Params[1], &key); err != nil {
						t.Error(err)
					}
					if address != pool || key != deployment.BeaconSlot {
						t.Error("unexpected beacon slot read", address, key)
					}
					actualBeacon := beacon
					if tc.wrongBeacon {
						actualBeacon = registry
					}
					result = hexutil.Encode(common.LeftPadBytes(actualBeacon.Bytes(), 32))
				default:
					t.Error("unexpected method", req.Method)
				}
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
			}))
			defer srv.Close()
			e, err := ethclient.Dial(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			pin := BlockPin{Number: header.Number.Uint64(), Hash: consensusHash(header), DeploymentVersion: "v-test"}
			c := &Client{primary: e, quarantine: map[*ethclient.Client]bool{}, attestations: map[*ethclient.Client]EndpointAttestation{}, issuedPins: map[BlockPin]struct{}{pin: {}}, publicReads: readpolicy.NewCoalescer(), readPermit: make(chan struct{}, 4)}
			c.verified = &VerifiedConfig{Profile: profile.Profile{DeploymentVersion: "v-test"}, AnchorHeight: pin.Number, AnchorHash: pin.Hash}
			selected := deployment.Current()
			if tc.activeOnly {
				for name, role := range selected.Roles {
					role.StagedRuntimeHash, role.StagedUpgrade = "", ""
					selected.Roles[name] = role
				}
			}
			r := selected.Roles["PoolRegistry"]
			r.Address = registry.Hex()
			selected.Roles["PoolRegistry"] = r
			r = selected.Roles["PoolBeacon"]
			r.Address = beacon.Hex()
			selected.Roles["PoolBeacon"] = r
			a := &ReadAdapter{client: c, manifest: selected, poolProxyRuntimeHash: crypto.Keccak256Hash(proxyCode)}
			err = a.identifyPool(context.Background(), pin, pool)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errorChainContains(err, tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
			if storageReads != tc.storageReads {
				t.Fatalf("beacon slot reads = %d, want %d", storageReads, tc.storageReads)
			}
			wantCodeReads := 0
			if tc.registered {
				wantCodeReads = 1
			}
			if membershipReads != 1 || codeReads != wantCodeReads {
				t.Fatalf("membership reads = %d, code reads = %d, want 1 and %d", membershipReads, codeReads, wantCodeReads)
			}
		})
	}
}

// errorChainContains reports whether err or any error it wraps carries want.
func errorChainContains(err error, want string) bool {
	for ; err != nil; err = errors.Unwrap(err) {
		if strings.Contains(err.Error(), want) {
			return true
		}
	}
	return false
}
