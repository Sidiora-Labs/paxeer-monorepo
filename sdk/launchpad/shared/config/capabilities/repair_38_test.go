package capabilities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

type repair38Epoch struct {
	ActivationHeight    uint64
	Pool, Token, Trader common.Address
	Codes               map[common.Address]hexutil.Bytes
}
type repair38Snapshot struct {
	NumberOffset    int64
	Reserves, Quote []string
	Allowance       string
	Gas             hexutil.Uint64
	EstimateError   string
}
type repair38Fixture struct {
	Provenance map[string]string
	Epochs     map[string]repair38Epoch
	Policy     map[string]int64
	Snapshots  []repair38Snapshot
}
type repair38RPC struct {
	t         *testing.T
	mu        sync.Mutex
	manifest  deployment.Manifest
	epoch     repair38Epoch
	fixture   repair38Fixture
	headers   map[common.Hash]*types.Header
	snapshots map[common.Hash]repair38Snapshot
	blocks    []common.Hash
	calls     map[string]hexutil.Bytes
	overrides map[string]hexutil.Bytes
	codes     map[common.Address]hexutil.Bytes
	storage   map[string]hexutil.Bytes
	seen      map[string]int
}

func repair38Word(v int64) []byte             { return common.LeftPadBytes(big.NewInt(v).Bytes(), 32) }
func repair38Address(a common.Address) []byte { return common.LeftPadBytes(a.Bytes(), 32) }
func repair38Key(address common.Address, data []byte, block common.Hash) string {
	return address.Hex() + "/" + hexutil.Encode(data) + "/" + block.Hex()
}
func repair38Call(t *testing.T, signature string, args ...any) []byte {
	t.Helper()
	data, err := encodeCall(signature, args...)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func repair38Tuple(t *testing.T, values []string) []byte {
	t.Helper()
	var out []byte
	for _, s := range values {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok || n.Sign() < 0 || n.BitLen() > 256 {
			t.Fatalf("invalid committed ABI uint256 %q", s)
		}
		out = append(out, common.LeftPadBytes(n.Bytes(), 32)...)
	}
	return out
}
func (b *repair38RPC) address(role string) common.Address {
	return common.HexToAddress(b.manifest.Roles[role].Address)
}
func (b *repair38RPC) put(address common.Address, signature string, value []byte, args ...any) {
	data := repair38Call(b.t, signature, args...)
	for _, block := range b.blocks {
		b.calls[repair38Key(address, data, block)] = value
	}
}
func (b *repair38RPC) override(address common.Address, signature string, value []byte, args ...any) {
	b.t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, block := range b.blocks {
		b.overrides[repair38Key(address, repair38Call(b.t, signature, args...), block)] = value
	}
}

// The peer replays committed bytecode and ABI vectors. No default response is
// allowed: a request outside the recorded identity, calldata or block fails.
func (b *repair38RPC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid RPC request", 400)
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var result any
	var problem string
	code := -32000
	decode := func(i int, v any) bool { return i < len(req.Params) && json.Unmarshal(req.Params[i], v) == nil }
	fail := func(s string) { problem = s }
	pinned := func(i int) (common.Hash, bool) {
		var pin struct {
			Hash      common.Hash `json:"blockHash"`
			Canonical bool        `json:"requireCanonical"`
		}
		if !decode(i, &pin) || pin.Canonical != (req.Method == "eth_estimateGas") || b.headers[pin.Hash] == nil {
			fail("read is not pinned to a committed canonical fixture block")
			return common.Hash{}, false
		}
		return pin.Hash, true
	}
	switch req.Method {
	case "eth_chainId":
		result = "0x7d"
	case "eth_gasPrice":
		result = "0x3b9aca00"
	case "eth_getBlockByHash":
		var hash common.Hash
		if !decode(0, &hash) || b.headers[hash] == nil {
			fail("unrecorded block")
		} else {
			result = b.headers[hash]
		}
	case "eth_getCode":
		var address common.Address
		if !decode(0, &address) {
			fail("invalid code address")
			break
		}
		if _, ok := pinned(1); !ok {
			break
		}
		b.seen["code/"+address.Hex()]++
		var ok bool
		result, ok = b.codes[address]
		if !ok {
			fail("unrecorded code address")
		}
	case "eth_getStorageAt":
		var address common.Address
		var slot common.Hash
		if !decode(0, &address) || !decode(1, &slot) {
			fail("invalid storage request")
			break
		}
		if _, ok := pinned(2); !ok {
			break
		}
		key := address.Hex() + "/" + slot.Hex()
		b.seen["storage/"+key]++
		var ok bool
		result, ok = b.storage[key]
		if !ok {
			fail("unrecorded storage slot")
		}
	case "eth_call", "eth_estimateGas":
		var call struct {
			From  common.Address
			To    common.Address
			Data  hexutil.Bytes
			Input hexutil.Bytes
		}
		if !decode(0, &call) {
			fail("invalid call")
			break
		}
		if len(call.Data) == 0 {
			call.Data = call.Input
		}
		block, ok := pinned(1)
		if !ok {
			break
		}
		key := repair38Key(call.To, call.Data, block)
		b.seen[req.Method+"/"+key]++
		if req.Method == "eth_estimateGas" {
			expected := repair38Call(b.t, "createMarket(string,string,uint8,address)", "Repair native", "R38", uint8(0), common.Address{})
			if (call.To != b.address("Router") && call.To != b.address("SidioraFactory")) || call.From != b.epoch.Trader || !bytes.Equal(call.Data, expected) {
				fail("unrecorded creation destination, caller or calldata")
				break
			}
			snapshot := b.snapshots[block]
			if snapshot.EstimateError != "" {
				fail(snapshot.EstimateError)
				code = 3
			} else {
				result = snapshot.Gas
			}
		} else {
			result, ok = b.overrides[key]
			if !ok {
				result, ok = b.calls[key]
			}
			if !ok {
				fail("unrecorded call " + key)
			}
			quote := repair38Call(b.t, "quoteExactInput(address,uint256,bool)", b.epoch.Pool, big.NewInt(1_000_000), true)
			if call.To == b.address("Quoter") && (!bytes.Equal(call.Data, quote) || call.From != b.epoch.Trader) {
				fail("quote lost the recorded trader context")
			}
		}
	default:
		fail("unrecorded RPC method " + req.Method)
	}
	response := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	if problem != "" {
		response["error"] = map[string]any{"code": code, "message": problem}
	} else {
		response["result"] = result
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		b.t.Error(err)
	}
}

type repair38Chain struct {
	t      *testing.T
	ctx    context.Context
	peer   *repair38RPC
	reader *PinnedRPCReader
	engine *PreflightEngine
	trader common.Address
}

func repair38OpenChain(t *testing.T, native bool) *repair38Chain {
	t.Helper()
	raw, err := os.ReadFile("testdata/repair38-rpc.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture repair38Fixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	name := "native"
	if !native {
		name = "historical"
	}
	epoch, ok := fixture.Epochs[name]
	if !ok {
		t.Fatal("missing committed epoch")
	}
	m, ok := deployment.Approved(epoch.ActivationHeight)
	if !ok {
		t.Fatal("unapproved fixture epoch")
	}
	b := &repair38RPC{t: t, manifest: m, epoch: epoch, fixture: fixture, headers: map[common.Hash]*types.Header{}, snapshots: map[common.Hash]repair38Snapshot{}, calls: map[string]hexutil.Bytes{}, overrides: map[string]hexutil.Bytes{}, codes: epoch.Codes, storage: map[string]hexutil.Bytes{}, seen: map[string]int{}}
	for _, snapshot := range fixture.Snapshots {
		// Only the freshness envelope moves with the test clock. Recorded code,
		// policy, pool state, quote and payment identity remain unchanged.
		h := &types.Header{Number: new(big.Int).SetUint64(m.ActivationHeight + uint64(snapshot.NumberOffset)), Time: uint64(time.Now().Unix()), Difficulty: big.NewInt(0), GasLimit: 30_000_000}
		block := h.Hash()
		b.headers[block] = h
		b.snapshots[block] = snapshot
		b.blocks = append(b.blocks, block)
	}
	for name, role := range m.Roles {
		if role.Kind == "precompile" {
			continue
		}
		address := common.HexToAddress(role.Address)
		if hash := crypto.Keccak256Hash(b.codes[address]).Hex(); hash != role.RuntimeHash {
			t.Fatalf("%s committed runtime hash=%s want=%s", name, hash, role.RuntimeHash)
		}
		if role.ImplementationAddress != "" {
			impl := common.HexToAddress(role.ImplementationAddress)
			if hash := crypto.Keccak256Hash(b.codes[impl]).Hex(); hash != role.ImplementationHash {
				t.Fatalf("%s committed implementation hash=%s want=%s", name, hash, role.ImplementationHash)
			}
			if role.Kind == "erc1967" {
				b.storage[address.Hex()+"/"+deployment.ImplementationSlot.Hex()] = repair38Address(impl)
			} else {
				b.put(address, "implementation()", repair38Address(impl))
			}
		}
	}
	b.put(b.address("PoolBeacon"), "owner()", repair38Address(common.HexToAddress(m.Governance.ExpectedBeaconOwner)))
	b.put(b.address("PoolBeacon"), "pendingOwner()", repair38Word(0))
	for signature, value := range fixture.Policy {
		role := "ProtocolConfig"
		if signature == "decimals()" {
			role = "USDL"
		}
		b.put(b.address(role), signature, repair38Word(value))
	}
	b.put(b.address("ProtocolConfig"), "usdlAddress()", repair38Address(b.address("USDL")))
	for signature, role := range map[string]string{"protocolConfig()": "ProtocolConfig", "usdlAddress()": "USDL", "eventEmitter()": "EventEmitter", "factory()": "SidioraFactory", "feeAccumulator()": "FeeAccumulator"} {
		b.put(epoch.Pool, signature, repair38Address(b.address(role)))
	}
	b.put(epoch.Pool, "opticalAddress()", repair38Word(0))
	b.put(epoch.Pool, "tokenAddress()", repair38Address(epoch.Token))
	b.put(epoch.Pool, "guardian()", repair38Address(epoch.Trader))
	b.put(epoch.Pool, "paused()", repair38Word(0))
	b.put(b.address("PoolRegistry"), "isRegisteredPool(address)", repair38Word(1), epoch.Pool)
	b.put(b.address("PoolRegistry"), "isRegisteredPool(address)", repair38Word(0), epoch.Token)
	b.put(b.address("PoolRegistry"), "getPoolByToken(address)", repair38Address(epoch.Pool), epoch.Token)
	metadata := append(repair38Address(epoch.Trader), repair38Address(epoch.Token)...)
	metadata = append(metadata, repair38Word(0)...)
	metadata = append(metadata, repair38Word(1)...)
	metadata = append(metadata, repair38Word(1_700_000_000)...)
	metadata = append(metadata, repair38Word(int64(m.ActivationHeight))...)
	b.put(b.address("PoolRegistry"), "getPoolMetadata(address)", metadata, epoch.Pool)
	for _, block := range b.blocks {
		snapshot := b.snapshots[block]
		b.calls[repair38Key(epoch.Pool, repair38Call(t, "getReserves()"), block)] = repair38Tuple(t, snapshot.Reserves)
		b.calls[repair38Key(b.address("Quoter"), repair38Call(t, "quoteExactInput(address,uint256,bool)", epoch.Pool, big.NewInt(1_000_000), true), block)] = repair38Tuple(t, snapshot.Quote)
		for _, spender := range []common.Address{b.address(deployment.LaunchpadPrecompileRole), b.address("Router"), b.address("SidioraFactory")} {
			allowance := snapshot.Allowance
			if spender != b.address(deployment.LaunchpadPrecompileRole) {
				allowance = "1000000"
			}
			b.calls[repair38Key(b.address("USDL"), repair38Call(t, "allowance(address,address)", epoch.Trader, spender), block)] = repair38Tuple(t, []string{allowance})
		}
	}
	slot := repair38Word(0)
	if !native {
		slot = repair38Address(b.address("PoolBeacon"))
	}
	b.storage[epoch.Pool.Hex()+"/"+deployment.BeaconSlot.Hex()] = slot
	server := httptest.NewServer(b)
	t.Cleanup(server.Close)
	// A dedicated transport prevents a proxy environment from redirecting even
	// the loopback request. The RPC client cannot dial any other host.
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	rpcClient, err := rpc.DialOptions(t.Context(), server.URL, rpc.WithHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpcClient.Close)
	reader, err := NewPinnedRPCReader(rpcClient)
	if err != nil {
		t.Fatal(err)
	}
	runtime := crypto.Keccak256Hash(b.codes[epoch.Pool]).Hex()
	engine, err := NewPreflightEngine(reader, m, runtime, strings.Repeat("a", 64), time.Hour, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return &repair38Chain{t: t, ctx: t.Context(), peer: b, reader: reader, engine: engine, trader: epoch.Trader}
}
func (c *repair38Chain) request(family Family, block common.Hash) PreflightRequest {
	return PreflightRequest{Family: family, ExpectedRevision: c.engine.Revision().RevisionSHA256, BlockHash: block, Trader: c.trader, Recipient: c.trader, AmountIn: "1000000", IsBuy: true, Name: "Repair native", Symbol: "R38"}
}
func (c *repair38Chain) connected(request PreflightRequest) interfaces.ConnectedCapabilitiesPreflight {
	c.t.Helper()
	status, body := servePreflight(c.t, c.engine, request)
	if status != http.StatusOK {
		c.t.Fatalf("connected preflight: %d %s", status, body)
	}
	var result interfaces.ConnectedCapabilitiesPreflight
	if err := json.Unmarshal(body, &result); err != nil {
		c.t.Fatal(err)
	}
	return result
}
func (c *repair38Chain) allowance(spender common.Address, block common.Hash) *big.Int {
	c.t.Helper()
	value, err := c.engine.call(c.ctx, c.peer.address("USDL"), "allowance(address,address)", block, c.trader, spender)
	if err != nil || len(value) != 32 {
		c.t.Fatalf("recorded allowance: %x %v", value, err)
	}
	return new(big.Int).SetBytes(value)
}

func TestRepair_38_NativeCostSpender(t *testing.T) {
	c := repair38OpenChain(t, true)
	authority := c.peer.address(deployment.LaunchpadPrecompileRole)
	for _, path := range []string{"Router", "SidioraFactory"} {
		t.Run(path, func(t *testing.T) {
			child := *c
			child.t = t
			request := child.request(Creation, c.peer.blocks[0])
			request.CreationPath = path
			before := child.connected(request)
			if before.Supported || !before.CreationCost.Present || before.CreationCost.Value == nil {
				t.Fatalf("spender must be disclosed before approval: %+v", before)
			}
			cost := before.CreationCost.Value
			if common.HexToAddress(cost.Spender) != authority || cost.Spender == c.peer.address(path).Hex() || cost.ProtocolFeeRaw != "1000000" || cost.BlockHash != request.BlockHash.Hex() || cost.GasEstimateAvailable {
				t.Fatalf("native payment identity before approval: %+v", cost)
			}
			if child.allowance(c.peer.address(path), request.BlockHash).String() != "1000000" || child.allowance(common.HexToAddress(cost.Spender), request.BlockHash).Sign() != 0 {
				t.Fatal("destination-only approval must not satisfy native spender")
			}
			request.BlockHash = c.peer.blocks[1]
			after := child.connected(request)
			if !after.Supported || !after.CreationCost.Present || after.CreationCost.Value == nil {
				t.Fatalf("recorded native approval did not admit creation: %+v", after)
			}
			paid := after.CreationCost.Value
			if paid.Spender != cost.Spender || !paid.GasEstimateAvailable || paid.EstimatedGasUnits != 200_000 || paid.EstimatedGasRaw != "200000000000000" || paid.GasPriceRaw != "1000000000" || paid.BlockHash != request.BlockHash.Hex() || paid.ConfigRevision != cost.ConfigRevision {
				t.Fatalf("native approval cost context: %+v", paid)
			}
			if child.allowance(common.HexToAddress(paid.Spender), request.BlockHash).String() != paid.ProtocolFeeRaw {
				t.Fatal("disclosed spender does not round-trip to approved fee")
			}
			if child.allowance(authority, c.peer.blocks[2]).Sign() != 0 || child.allowance(c.peer.address(path), c.peer.blocks[2]).String() != "1000000" {
				t.Fatal("recorded creation consumed the wrong allowance")
			}
			data := repair38Call(t, "createMarket(string,string,uint8,address)", request.Name, request.Symbol, request.FeeStrategy, request.Optical)
			c.peer.mu.Lock()
			seen := c.peer.seen["eth_estimateGas/"+repair38Key(c.peer.address(path), data, request.BlockHash)]
			c.peer.mu.Unlock()
			if seen == 0 {
				t.Fatal("creation simulation did not retain its transaction destination")
			}
		})
	}
}

func TestRepair_38_NativePoolIdentity(t *testing.T) {
	c := repair38OpenChain(t, true)
	pool, token := c.peer.epoch.Pool, c.peer.epoch.Token
	for _, family := range []Family{PlainQuote, Trading} {
		t.Run(string(family), func(t *testing.T) {
			child := *c
			child.t = t
			request := child.request(family, c.peer.blocks[1])
			request.Pool = pool
			before, err := c.engine.Preflight(c.ctx, request)
			if err != nil || !before.Supported || before.Pool == nil || before.Quote == nil {
				t.Fatalf("native %s admission: %+v %v", family, before, err)
			}
			state := before.Pool
			if state.ImplementationVersion != c.engine.manifest.DeploymentVersion+"/Pool/"+c.engine.poolRuntime.Hex() || state.RedeemableLiquidityRaw != "0" || state.RealUSDLBalance != "0" || state.Paused || state.VirtualUSDLReserve != "10000000000" || state.TokenReserve != "1000000000000000" {
				t.Fatalf("native generation and initial state: %+v", state)
			}
			connected := child.connected(request)
			if !connected.Supported || !connected.Pool.Present || connected.Pool.Value == nil || connected.Pool.Value.ImplementationVersion != state.ImplementationVersion || !connected.Quote.Present || connected.Quote.Value == nil || connected.Quote.Value.AmountOut != before.Quote.AmountOut {
				t.Fatal("native state or quote lost in connected consumer")
			}
			request.Pool = token
			if unknown := child.connected(request); unknown.Supported || !unknown.ReasonCode.Present || unknown.ReasonCode.Value != string(PoolUnknown) {
				t.Fatal("token admitted as registered pool")
			}
			request.Pool = pool
			request.BlockHash = c.peer.blocks[2]
			after, err := c.engine.Preflight(c.ctx, request)
			if err != nil || !after.Supported || after.Pool == nil || after.Quote == nil || after.Pool.RealUSDLBalance != "997000" || after.ObservedRevisionSHA256 == before.ObservedRevisionSHA256 || after.Quote.AmountOut == before.Quote.AmountOut {
				t.Fatalf("committed trade state did not change admission: %+v %v", after, err)
			}
			request.BlockHash = c.peer.blocks[1]
			historical, err := c.engine.Preflight(c.ctx, request)
			if err != nil || historical.Pool == nil || *historical.Pool != *before.Pool || historical.Quote == nil || *historical.Quote != *before.Quote || historical.ObservedRevisionSHA256 != before.ObservedRevisionSHA256 {
				t.Fatalf("later trade contaminated pinned creation state: %+v %v", historical, err)
			}
		})
	}
	c.peer.mu.Lock()
	beaconReads := c.peer.seen["storage/"+pool.Hex()+"/"+deployment.BeaconSlot.Hex()]
	c.peer.mu.Unlock()
	if beaconReads != 0 {
		t.Fatal("native identity read a beacon slot")
	}
	slot, err := c.reader.StorageAtHash(c.ctx, pool, deployment.BeaconSlot, c.peer.blocks[1])
	if err != nil || len(slot) != 32 || new(big.Int).SetBytes(slot).Sign() != 0 {
		t.Fatalf("native fixture must have no beacon: %x %v", slot, err)
	}
}

func TestRepair_38_DeploymentAuthority(t *testing.T) {
	engine := &PreflightEngine{manifest: deployment.Current()}
	if native, err := engine.nativeDeployment(); err != nil || !native {
		t.Fatalf("approved native deployment refused: %v", err)
	}
	for i, mutate := range []func(*deployment.Manifest){
		func(m *deployment.Manifest) { delete(m.Roles, deployment.LaunchpadPrecompileRole) },
		func(m *deployment.Manifest) {
			r := m.Roles[deployment.LaunchpadPrecompileRole]
			r.Address = m.Roles["Router"].Address
			m.Roles[deployment.LaunchpadPrecompileRole] = r
		},
		func(m *deployment.Manifest) { m.ActivationHeight-- },
		func(m *deployment.Manifest) { m.DeploymentVersion += "-unapproved" },
		func(m *deployment.Manifest) {
			r := m.Roles["PoolRegistry"]
			r.RuntimeHash = m.Roles["USDL"].RuntimeHash
			m.Roles["PoolRegistry"] = r
		},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			bad := &PreflightEngine{manifest: deployment.Current()}
			mutate(&bad.manifest)
			if native, err := bad.nativeDeployment(); err == nil || native {
				t.Fatal("unapproved native authority admitted")
			}
		})
	}
	previous, ok := deployment.Approved(32_226_215)
	if !ok {
		t.Fatal("historical deployment missing")
	}
	if native, err := (&PreflightEngine{manifest: previous}).nativeDeployment(); err != nil || native {
		t.Fatalf("historical beacon selection lost: %v", err)
	}
}

func TestRepair_38_HistoricalPoolIdentity(t *testing.T) {
	c := repair38OpenChain(t, false)
	block, pool := c.peer.blocks[1], c.peer.epoch.Pool
	state, err := c.engine.poolState(c.ctx, pool, block)
	if err != nil || state.ImplementationVersion != c.engine.manifest.Roles["PoolBeacon"].ImplementationHash {
		t.Fatalf("historical beacon version: %+v %v", state, err)
	}
	for _, family := range []Family{PlainQuote, Trading} {
		request := c.request(family, block)
		request.Pool = pool
		result := c.connected(request)
		if !result.Supported || !result.Pool.Present || result.Pool.Value == nil || result.Pool.Value.ImplementationVersion != state.ImplementationVersion || !result.Quote.Present {
			t.Fatalf("historical %s no longer admitted: %+v", family, result)
		}
	}
	c.peer.mu.Lock()
	c.peer.storage[pool.Hex()+"/"+deployment.BeaconSlot.Hex()] = repair38Address(common.HexToAddress("0xdead"))
	c.peer.mu.Unlock()
	if _, err := c.engine.poolState(c.ctx, pool, block); err == nil {
		t.Fatal("historical beacon mismatch admitted")
	} else if reason, _, ok := Refused(err); !ok || reason != IdentityUnverified {
		t.Fatalf("historical mismatch: %v", err)
	}
	for _, path := range []string{"Router", "SidioraFactory"} {
		request := c.request(Creation, block)
		request.CreationPath = path
		cost := c.connected(request)
		if !cost.Supported || !cost.CreationCost.Present || cost.CreationCost.Value == nil || cost.CreationCost.Value.Spender != c.peer.address(path).Hex() {
			t.Fatalf("historical %s spender changed: %+v", path, cost)
		}
	}
}

func TestRepair_38_NativeIdentityRefusals(t *testing.T) {
	for _, caseName := range []string{"runtime", "factory", "config", "usdl", "emitter", "fee_accumulator", "zero_token", "reverse_registry", "metadata_token", "metadata_optical", "metadata_creator", "metadata_length", "registration", "pool_paused", "guardian", "authority_runtime", "registry_runtime"} {
		t.Run(caseName, func(t *testing.T) {
			c := repair38OpenChain(t, true)
			b := c.peer
			pool := b.epoch.Pool
			wrong := repair38Address(common.HexToAddress("0xdead"))
			switch caseName {
			case "runtime":
				b.codes[pool] = common.FromHex("0x6000")
			case "factory":
				b.override(pool, "factory()", wrong)
			case "config":
				b.override(pool, "protocolConfig()", wrong)
			case "usdl":
				b.override(pool, "usdlAddress()", wrong)
			case "emitter":
				b.override(pool, "eventEmitter()", wrong)
			case "fee_accumulator":
				b.override(pool, "feeAccumulator()", wrong)
			case "zero_token":
				b.override(pool, "tokenAddress()", repair38Word(0))
			case "reverse_registry":
				b.override(b.address("PoolRegistry"), "getPoolByToken(address)", wrong, b.epoch.Token)
			case "metadata_token", "metadata_optical", "metadata_creator", "metadata_length":
				data := repair38Call(t, "getPoolMetadata(address)", pool)
				key := repair38Key(b.address("PoolRegistry"), data, b.blocks[1])
				metadata := append([]byte(nil), b.calls[key]...)
				switch caseName {
				case "metadata_token":
					copy(metadata[32:64], wrong)
				case "metadata_optical":
					copy(metadata[64:96], wrong)
				case "metadata_creator":
					copy(metadata[:32], repair38Word(0))
				case "metadata_length":
					metadata = metadata[:160]
				}
				b.override(b.address("PoolRegistry"), "getPoolMetadata(address)", metadata, pool)
			case "registration":
				b.override(b.address("PoolRegistry"), "isRegisteredPool(address)", repair38Word(0), pool)
			case "pool_paused":
				b.override(pool, "paused()", repair38Word(1))
			case "guardian":
				b.override(pool, "guardian()", append([]byte{1}, make([]byte, 31)...))
			case "authority_runtime":
				b.codes[b.address("SidioraFactory")] = common.FromHex("0x6000")
			case "registry_runtime":
				b.codes[b.address("PoolRegistry")] = common.FromHex("0x6000")
			}
			for _, family := range []Family{PlainQuote, Trading} {
				request := c.request(family, b.blocks[1])
				request.Pool = pool
				result := c.connected(request)
				reason := string(IdentityUnverified)
				if caseName == "registration" {
					reason = string(PoolUnknown)
				}
				if caseName == "pool_paused" {
					reason = string(UnexecutableQuote)
				}
				if result.Supported || result.ExecutionPermitted || result.Quote.Present || !result.ReasonCode.Present || result.ReasonCode.Value != reason {
					t.Fatalf("%s admitted invalid native %s identity: %+v", family, caseName, result)
				}
			}
		})
	}
}
