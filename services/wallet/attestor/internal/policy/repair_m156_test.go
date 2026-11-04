package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/testkernel"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func repairM156Daily(t *testing.T, h *harness, account, asset string, want int64) {
	t.Helper()
	got, err := h.ledger.Spent(AccountKey(account), asset, h.now.Add(-SpendWindow))
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(big.NewInt(want)) != 0 {
		t.Fatalf("daily %s = %s, want %d", asset, got, want)
	}
}

func repairM156ConstructDebits(t *testing.T, shipped bool, methods []string) {
	t.Helper()
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			native := method == "budgetCreateNative" || method == "budgetFundNative"
			f := repair244OpenSource(t, native)
			if shipped {
				doc, err := LoadFile(filepath.Join("..", "..", "..", "deploy", "attestor-policy.json"))
				if err != nil {
					t.Fatal(err)
				}
				f.h.policy = New(doc)
				f.h.policy.SetConstructStateReader(func(Context) (*ConstructState, error) { return f.state, nil })
			}
			source := uint(0)
			if native {
				source = 3
			}
			asset := f.accounts[source].Asset
			key := ConstructAsset(asset)
			cap := Cap{PerTransaction: "10", Daily: "15"}
			f.h.policy.doc.Defaults.Caps[key] = cap
			domain := "layerxpay"
			if method == "transfer" || method == "sandboxFund" {
				domain = "layerxprograms"
			}
			address, _ := evm.PrecompileAddress(domain)
			if !slices.Contains(f.h.policy.doc.Defaults.Selectors[strings.ToLower(address.Hex())], selectorHex(t, domain, method)) && shipped {
				t.Fatalf("shipped policy is missing %s.%s", domain, method)
			}
			initial := int64(0)
			if method == "budgetFund" || method == "budgetFundNative" {
				// Resolve funding against a budget actually created by the native module.
				created := f.create(t, native, 1)
				if !shipped || !native {
					repair244Spend(t, f.decide(t, "layerxpay", created), asset, 1)
					initial = 1
				}
				if err := testkernel.Execute("layerxpay", created); err != nil {
					t.Fatal(err)
				}
				f.refresh(t, true)
			}
			pack := func(n int64) []byte {
				switch method {
				case "budgetCreate", "budgetCreateNative":
					return f.create(t, native, n)
				case "budgetFund":
					return repair244Pack(t, domain, method, f.budget, big.NewInt(n))
				case "budgetFundNative":
					return repair244Pack(t, domain, method, f.budget, big.NewInt(n), uint64(1))
				case "escrowOpen":
					return repair244Pack(t, domain, method, [32]byte{1}, f.accounts[0].ID, f.accounts[4].ID, f.accounts[2].ID, [32]byte{2}, asset, big.NewInt(n), uint64(1000000), uint64(1000), [32]byte{3}, [32]byte{4})
				case "transfer":
					return repair244Pack(t, domain, method, [32]byte{1}, []policyTestTransferLeg{{From: f.accounts[0].ID, Asset: asset, To: f.accounts[2].ID, Amount: big.NewInt(n / 2)}, {From: f.accounts[3].ID, Asset: asset, To: f.accounts[1].ID, Amount: big.NewInt(n - n/2)}})
				case "sandboxFund":
					return repair244Pack(t, domain, method, [32]byte{1}, f.accounts[0].ID, [32]byte{3}, f.accounts[4].ID, asset, big.NewInt(n), uint64(1000000), uint32(1), [7]uint64{}, []byte{1}, [32]byte{3}, []policyTestTransferLeg{{From: f.accounts[0].ID, Asset: asset, To: f.accounts[4].ID, Amount: big.NewInt(n / 2)}, {From: f.accounts[3].ID, Asset: asset, To: f.accounts[4].ID, Amount: big.NewInt(n - n/2)}})
				default:
					t.Fatalf("uncovered debit selector %s", method)
					return nil
				}
			}
			data := pack(10)
			before := make([]testkernel.Account, 6)
			for i := range before {
				var err error
				before[i], err = testkernel.Info(uint(i))
				if err != nil {
					t.Fatal(err)
				}
			}
			unresolved := newHarness(t)
			unresolved.policy.doc.Defaults.Selectors[address.Hex()] = []string{selectorHex(t, domain, method)}
			expect(t, unresolved.policy.Evaluate(testAccount, unresolved.tx(t, address, big.NewInt(0), data), unresolved.ledger), CodeMissingField)
			expect(t, unresolved.policy.Evaluate(testAccount, unresolved.tx(t, address, big.NewInt(1), data), unresolved.ledger), CodeDecodeError)
			repairM156Daily(t, unresolved, testAccount, AssetNative, 0)
			expect(t, f.decide(t, domain, pack(11)), CodeValueCap)
			delete(f.h.policy.doc.Defaults.Caps, key)
			refused := f.decide(t, domain, data)
			expect(t, refused, CodeNoCap)
			if len(refused.Spends) != 0 {
				t.Fatalf("uncapped refusal recorded %+v", refused.Spends)
			}
			repairM156Daily(t, f.h, testAccount, key, initial)
			for i, account := range before {
				repair244Balance(t, uint(i), account.Balance)
			}
			f.h.policy.doc.Defaults.Caps[key] = cap
			repair244Spend(t, f.decide(t, domain, data), asset, 10)
			if err := testkernel.Execute(domain, data); err != nil {
				t.Fatal(err)
			}
			if method == "transfer" || method == "sandboxFund" {
				repair244Balance(t, 0, 995)
				repair244Balance(t, 3, 995)
				if method == "transfer" {
					repair244Balance(t, 2, 5)
					repair244Balance(t, 1, 5)
				} else {
					repair244Balance(t, 4, 10)
				}
			} else {
				setup := uint64(0)
				if method == "budgetFund" || method == "budgetFundNative" {
					setup = 1
				}
				repair244Balance(t, source, 990-setup)
				target := uint(1)
				if method == "escrowOpen" {
					target = 4
				}
				repair244Balance(t, target, 10+setup)
				if native {
					repair244Balance(t, 0, 1000)
				}
			}
			repairM156Daily(t, f.h, testAccount, key, 10+initial)
			repairM156Daily(t, f.h, testAccount, AssetNative, 0)
			if native {
				repairM156Daily(t, f.h, testAccount, ConstructAsset(f.accounts[0].Asset), 0)
			}
			for i := range before {
				var err error
				before[i], err = testkernel.Info(uint(i))
				if err != nil {
					t.Fatal(err)
				}
			}
			expect(t, f.decide(t, domain, pack(6)), CodeDailyCap)
			repairM156Daily(t, f.h, testAccount, key, 10+initial)
			for i, account := range before {
				repair244Balance(t, uint(i), account.Balance)
			}
		})
	}
}

func TestRepairM_156_ConstructDebitAccounting(t *testing.T) {
	repairM156ConstructDebits(t, false, []string{"budgetCreate", "budgetCreateNative", "budgetFund", "budgetFundNative", "escrowOpen", "transfer", "sandboxFund"})
}

func TestRepairM_156_ShippedFundedConstructs(t *testing.T) {
	TestShippedPolicyAllowsConstructSelectors(t)
}

// This peer serves ABI-encoded position snapshots over real JSON-RPC. It is
// deliberately separate from the production policy and resolver under test.
type repairM156PositionRPC struct {
	contract abi.ABI
	account  common.Address
	token    common.Address
	block    common.Hash
	position *big.Int
	output   []byte
	reads    int
}

func (f *repairM156PositionRPC) Call(args map[string]json.RawMessage, hash common.Hash) (hexutil.Bytes, error) {
	if hash != f.block {
		return nil, errors.New("position read must use the reviewed block hash")
	}
	var from, to common.Address
	var data hexutil.Bytes
	if err := json.Unmarshal(args["from"], &from); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(args["to"], &to); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(args["input"], &data); err != nil {
		return nil, err
	}
	address, _ := evm.PrecompileAddress("perps")
	want, err := f.contract.Pack("getPosition", f.position)
	if err != nil {
		return nil, err
	}
	if from != f.account || to != address || !bytes.Equal(data, want) {
		return nil, errors.New("unexpected position request")
	}
	f.reads++
	return append([]byte(nil), f.output...), nil
}

func (f *repairM156PositionRPC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request struct {
		JSONRPC string            `json:"jsonrpc"`
		ID      int               `json:"id"`
		Method  string            `json:"method"`
		Params  []json.RawMessage `json:"params"`
	}
	var args map[string]json.RawMessage
	var block struct {
		Hash      common.Hash `json:"blockHash"`
		Canonical bool        `json:"requireCanonical"`
	}
	err := json.NewDecoder(r.Body).Decode(&request)
	if err == nil && (r.Method != http.MethodPost || request.JSONRPC != "2.0" || request.Method != "eth_call" || len(request.Params) != 2) {
		err = errors.New("invalid JSON-RPC execution request")
	}
	if err == nil {
		err = json.Unmarshal(request.Params[0], &args)
	}
	if err == nil {
		err = json.Unmarshal(request.Params[1], &block)
	}
	if err == nil && !block.Canonical {
		err = errors.New("canonical reviewed block is required")
	}
	var output hexutil.Bytes
	if err == nil {
		output, err = f.Call(args, block.Hash)
	}
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
	if err != nil {
		response["error"] = map[string]any{"code": -32000, "message": err.Error()}
	} else {
		response["result"] = output
	}
	_ = json.NewEncoder(w).Encode(response)
}

func repairM156PositionClient(t *testing.T, account, token common.Address, block common.Hash) (*repairM156PositionRPC, *PerpsCollateralRPC) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "precompiles", "perps", "abi", "abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	producer, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	f := &repairM156PositionRPC{contract: producer, account: account, token: token, block: block, position: big.NewInt(4)}
	f.output, err = producer.Methods["getPosition"].Outputs.Pack(account, big.NewInt(0), true, big.NewInt(1), big.NewInt(1), token, big.NewInt(1), big.NewInt(1), big.NewInt(1), true)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(f)
	t.Cleanup(httpServer.Close)
	client, err := NewPerpsCollateralRPC(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.client.CloseIdleConnections)
	return f, client
}

func repairM156InstallPerpsReader(t *testing.T, p *Policy, account, token common.Address) (*repairM156PositionRPC, PerpsCollateralPin) {
	t.Helper()
	pin := PerpsCollateralPin{ChainID: 125, Account: account, BlockHash: common.BytesToHash(append([]byte{0x15, 0x6}, token.Bytes()...)), StateRoot: common.BytesToHash(token.Bytes())}
	f, client := repairM156PositionClient(t, account, token, pin.BlockHash)
	p.SetPerpsCollateralReader(func(ctx Context, position *big.Int) (*PerpsCollateralState, error) {
		return ResolvePerpsCollateral(context.Background(), pin, position, token, client)
	})
	return f, pin
}

func repairM156Add(t *testing.T, h *harness, account string, n int64) Decision {
	t.Helper()
	contract, err := evm.PrecompileABI("perps")
	if err != nil {
		t.Fatal(err)
	}
	data, err := contract.Pack("addCollateral", big.NewInt(4), big.NewInt(n))
	if err != nil {
		t.Fatal(err)
	}
	to, _ := evm.PrecompileAddress("perps")
	h.policy.doc.Defaults.Selectors[to.Hex()] = []string{selectorHex(t, "perps", "addCollateral")}
	return h.policy.Evaluate(account, h.tx(t, to, big.NewInt(0), data), h.ledger)
}

func TestRepairM_156_GovernedCollateralChange(t *testing.T) {
	h := newHarness(t)
	h.policy.doc.Defaults.Caps = mergeCaps(h.policy.doc.Defaults.Caps, nil)
	old := common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")
	next := common.HexToAddress(sidiora)
	for _, asset := range []common.Address{old, next} {
		h.policy.doc.Defaults.Caps[TokenAsset(asset)] = Cap{PerTransaction: "10", Daily: "15"}
	}
	f, pin := repairM156InstallPerpsReader(t, h.policy, common.HexToAddress(testAccount), old)
	d := repairM156Add(t, h, testAccount, 7)
	expect(t, d, CodeAllowed)
	if len(d.Spends) != 1 || d.Spends[0].Asset != TokenAsset(old) || d.Spends[0].Amount.Cmp(big.NewInt(7)) != 0 || d.ReviewedBlock != pin.BlockHash {
		t.Fatalf("initial collateral %+v", d)
	}
	oldBlock := pin.BlockHash
	// The next reviewed block carries a different governed collateral token.
	f, pin = repairM156InstallPerpsReader(t, h.policy, common.HexToAddress(testAccount), next)
	if pin.BlockHash == oldBlock {
		t.Fatal("governed change reused the previous reviewed block")
	}
	d = repairM156Add(t, h, testAccount, 8)
	expect(t, d, CodeAllowed)
	if len(d.Spends) != 1 || d.Spends[0].Asset != TokenAsset(next) || d.Spends[0].Amount.Cmp(big.NewInt(8)) != 0 || d.ReviewedBlock != pin.BlockHash || f.reads != 1 {
		t.Fatalf("governed collateral %+v; reads %d", d, f.reads)
	}
	repairM156Daily(t, h, testAccount, TokenAsset(old), 7)
	repairM156Daily(t, h, testAccount, TokenAsset(next), 8)
	delete(h.policy.doc.Defaults.Caps, TokenAsset(next))
	expect(t, repairM156Add(t, h, testAccount, 1), CodeNoCap)
	repairM156Daily(t, h, testAccount, TokenAsset(old), 7)
	repairM156Daily(t, h, testAccount, TokenAsset(next), 8)
}

func TestRepairM_156_RecordedExecutionView(t *testing.T) {
	// This output is committed by the real perps precompile's golden test.
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "precompiles", "perps", "testdata", "views_account_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var transcript map[string]string
	if err := json.Unmarshal(raw, &transcript); err != nil {
		t.Fatal(err)
	}
	account := common.HexToAddress("0xa11ce")
	token := common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")
	f, client := repairM156PositionClient(t, account, token, common.HexToHash("0x156"))
	f.output, err = hexutil.Decode(transcript["getPosition"])
	if err != nil {
		t.Fatal(err)
	}
	f.position = big.NewInt(2)
	pin := PerpsCollateralPin{ChainID: 125, Account: account, BlockHash: f.block, StateRoot: common.HexToHash("0x15601")}
	h := newHarness(t)
	h.policy.doc.Defaults.Caps[TokenAsset(token)] = Cap{PerTransaction: "10", Daily: "15"}
	h.policy.SetPerpsCollateralReader(func(ctx Context, position *big.Int) (*PerpsCollateralState, error) {
		return ResolvePerpsCollateral(context.Background(), pin, position, token, client)
	})
	to, _ := evm.PrecompileAddress("perps")
	h.policy.doc.Defaults.Selectors[to.Hex()] = []string{selectorHex(t, "perps", "addCollateral")}
	data, err := f.contract.Pack("addCollateral", f.position, big.NewInt(6))
	if err != nil {
		t.Fatal(err)
	}
	d := h.policy.Evaluate(account.Hex(), h.tx(t, to, big.NewInt(0), data), h.ledger)
	expect(t, d, CodeAllowed)
	if f.reads != 1 || len(d.Spends) != 1 || d.Spends[0].Asset != TokenAsset(token) || d.Spends[0].Amount.Cmp(big.NewInt(6)) != 0 || d.ReviewedBlock != pin.BlockHash {
		t.Fatalf("recorded execution view decision %+v", d)
	}
	repairM156Daily(t, h, account.Hex(), TokenAsset(token), 6)
}

func TestRepairM_156_CollateralStateRefusals(t *testing.T) {
	for _, mutation := range []string{"missing reader", "reader error", "nil state", "chain", "account", "position", "block", "root", "zero token", "disagreement", "inactive", "owner", "truncated", "trailing", "transport block"} {
		t.Run(mutation, func(t *testing.T) {
			h := newHarness(t)
			token := common.HexToAddress(sidiora)
			f, client := repairM156PositionClient(t, common.HexToAddress(testAccount), token, common.HexToHash("0x156"))
			pin := PerpsCollateralPin{ChainID: 125, Account: f.account, BlockHash: f.block, StateRoot: common.HexToHash("0x15601")}
			configured := token
			h.policy.SetPerpsCollateralReader(func(ctx Context, position *big.Int) (*PerpsCollateralState, error) {
				switch mutation {
				case "reader error":
					return nil, errors.New("params unavailable")
				case "nil state":
					return nil, nil
				case "chain":
					pin.ChainID++
				case "account":
					pin.Account = common.HexToAddress(friend)
				case "position":
					position = big.NewInt(5)
				case "block":
					pin.BlockHash = common.Hash{}
				case "root":
					pin.StateRoot = common.Hash{}
				case "zero token":
					configured = common.Address{}
				case "disagreement":
					configured = common.HexToAddress(friend)
				case "inactive":
					f.output[len(f.output)-1] = 0
				case "owner":
					f.output[31] ^= 1
				case "truncated":
					f.output = f.output[:len(f.output)-1]
				case "trailing":
					f.output = append(f.output, 0)
				case "transport block":
					pin.BlockHash = common.HexToHash("0x157")
				}
				return ResolvePerpsCollateral(context.Background(), pin, position, configured, client)
			})
			if mutation == "missing reader" {
				h.policy.SetPerpsCollateralReader(nil)
			}
			d := repairM156Add(t, h, testAccount, 1)
			expect(t, d, CodeMissingField)
			if len(d.Spends) != 0 {
				t.Fatalf("state refusal returned spends: %+v", d)
			}
			repairM156Daily(t, h, testAccount, TokenAsset(token), 0)
			repairM156Daily(t, h, testAccount, AssetNative, 0)
		})
	}
}

func TestRepairM_156_CollateralBatchPins(t *testing.T) {
	h := newHarness(t)
	h.policy.doc.Defaults.Caps = mergeCaps(h.policy.doc.Defaults.Caps, nil)
	token := common.HexToAddress(sidiora)
	f, client := repairM156PositionClient(t, common.HexToAddress(testAccount), token, common.HexToHash("0x156"))
	pin := PerpsCollateralPin{ChainID: 125, Account: f.account, BlockHash: f.block, StateRoot: common.HexToHash("0x15601")}
	h.policy.SetPerpsCollateralReader(func(ctx Context, position *big.Int) (*PerpsCollateralState, error) {
		return ResolvePerpsCollateral(context.Background(), pin, position, token, client)
	})
	to, _ := evm.PrecompileAddress("perps")
	h.policy.doc.Defaults.Selectors[to.Hex()] = []string{selectorHex(t, "perps", "addCollateral")}
	h.policy.doc.Defaults.Caps[TokenAsset(token)] = Cap{PerTransaction: "10", Daily: "15"}
	contract, err := evm.PrecompileABI("perps")
	if err != nil {
		t.Fatal(err)
	}
	batch := sponsoredBatch()
	batch.Quote.TokenAmount = big.NewInt(0)
	batch.Calls = nil
	for _, n := range []int64{4, 3} {
		data, err := contract.Pack("addCollateral", big.NewInt(4), big.NewInt(n))
		if err != nil {
			t.Fatal(err)
		}
		batch.Calls = append(batch.Calls, evm.BatchCall{To: to, Value: big.NewInt(0), Data: data})
	}
	digest, err := evm.SponsoredBatchDigest(batch)
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Kind: KindSponsoredBatch, View: &evm.SponsoredBatchClaim{Batch: batch, ClaimedDigest: digest}}
	d := h.policy.Evaluate(testAccount, req, h.ledger)
	expect(t, d, CodeAllowed)
	if len(d.Spends) != 1 || d.Spends[0].Asset != TokenAsset(token) || d.Spends[0].Amount.Cmp(big.NewInt(7)) != 0 || d.ReviewedBlock != pin.BlockHash {
		t.Fatalf("batch %+v", d)
	}
	reads := 0
	h.policy.SetPerpsCollateralReader(func(ctx Context, position *big.Int) (*PerpsCollateralState, error) {
		reads++
		if reads == 2 {
			pin.StateRoot[0] ^= 1
		}
		return ResolvePerpsCollateral(context.Background(), pin, position, token, client)
	})
	expect(t, h.policy.Evaluate(testAccount, req, h.ledger), CodeMissingField)
	repairM156Daily(t, h, testAccount, TokenAsset(token), 7)
}

func TestRepairM_156_MixedBatchStateDisagreement(t *testing.T) {
	f := repair244Open(t)
	token := common.HexToAddress(sidiora)
	repairM156InstallPerpsReader(t, f.h.policy, common.HexToAddress(testAccount), token)
	perps, _ := evm.PrecompileAddress("perps")
	pay, _ := evm.PrecompileAddress("layerxpay")
	f.h.policy.doc.Defaults.Selectors[perps.Hex()] = []string{selectorHex(t, "perps", "addCollateral")}
	contract, err := evm.PrecompileABI("perps")
	if err != nil {
		t.Fatal(err)
	}
	data, err := contract.Pack("addCollateral", big.NewInt(4), big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	batch := sponsoredBatch()
	batch.Quote.TokenAmount = big.NewInt(0)
	batch.Calls = []evm.BatchCall{{To: perps, Value: big.NewInt(0), Data: data}, {To: pay, Value: big.NewInt(0), Data: f.create(t, false, 1)}}
	digest, err := evm.SponsoredBatchDigest(batch)
	if err != nil {
		t.Fatal(err)
	}
	d := f.h.policy.Evaluate(testAccount, Request{Kind: KindSponsoredBatch, View: &evm.SponsoredBatchClaim{Batch: batch, ClaimedDigest: digest}}, f.h.ledger)
	expect(t, d, CodeMissingField)
	if len(d.Spends) != 0 {
		t.Fatalf("mixed state batch recorded spends %+v", d)
	}
	repairM156Daily(t, f.h, testAccount, TokenAsset(token), 0)
	repairM156Daily(t, f.h, testAccount, ConstructAsset(f.accounts[0].Asset), 0)
	repair244Balance(t, 0, 1000)
}

func TestRepairM_156_CollateralTransportRefusals(t *testing.T) {
	for _, name := range []string{"http", "id", "version", "missing result", "rpc error", "malformed", "trailing", "oversize"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := `{"jsonrpc":"2.0","id":1,"result":"0x"}`
				switch name {
				case "http":
					w.WriteHeader(http.StatusServiceUnavailable)
				case "id":
					body = `{"jsonrpc":"2.0","id":2,"result":"0x"}`
				case "version":
					body = `{"jsonrpc":"1.0","id":1,"result":"0x"}`
				case "missing result":
					body = `{"jsonrpc":"2.0","id":1}`
				case "rpc error":
					body = `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"reviewed block unavailable"}}`
				case "malformed":
					body = `{`
				case "trailing":
					body += `{}`
				case "oversize":
					body += strings.Repeat(" ", 16385)
				}
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(server.Close)
			client, err := NewPerpsCollateralRPC(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(client.client.CloseIdleConnections)
			h := newHarness(t)
			token := common.HexToAddress(sidiora)
			pin := PerpsCollateralPin{ChainID: 125, Account: common.HexToAddress(testAccount), BlockHash: common.HexToHash("0x156"), StateRoot: common.HexToHash("0x15601")}
			h.policy.SetPerpsCollateralReader(func(ctx Context, position *big.Int) (*PerpsCollateralState, error) {
				return ResolvePerpsCollateral(context.Background(), pin, position, token, client)
			})
			expect(t, repairM156Add(t, h, testAccount, 1), CodeMissingField)
			repairM156Daily(t, h, testAccount, TokenAsset(token), 0)
		})
	}
}
