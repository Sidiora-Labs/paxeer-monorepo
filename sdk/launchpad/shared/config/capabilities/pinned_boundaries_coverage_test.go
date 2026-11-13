package capabilities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type pinnedBoundaryReader struct {
	*PinnedRPCReader
	address  common.Address
	selector []byte
	mode     string
}

func (r pinnedBoundaryReader) CodeAtHash(ctx context.Context, address common.Address, hash common.Hash) ([]byte, error) {
	if r.mode == "code error" && address == r.address {
		return nil, context.DeadlineExceeded
	}
	return r.PinnedRPCReader.CodeAtHash(ctx, address, hash)
}
func (r pinnedBoundaryReader) StorageAtHash(ctx context.Context, address common.Address, slot, hash common.Hash) ([]byte, error) {
	if r.mode == "storage error" && address == r.address {
		return nil, context.DeadlineExceeded
	}
	return r.PinnedRPCReader.StorageAtHash(ctx, address, slot, hash)
}
func (r pinnedBoundaryReader) CallContractAtHash(ctx context.Context, msg ethereum.CallMsg, hash common.Hash) ([]byte, error) {
	if msg.To != nil && *msg.To == r.address && bytes.Equal(msg.Data[:min(len(msg.Data), 4)], r.selector) {
		return nil, context.DeadlineExceeded
	}
	return r.PinnedRPCReader.CallContractAtHash(ctx, msg, hash)
}
func (r pinnedBoundaryReader) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	if r.mode == "price error" {
		return nil, context.DeadlineExceeded
	}
	if r.mode == "price overflow" {
		return new(big.Int).Lsh(big.NewInt(1), 256), nil
	}
	if r.mode == "cost overflow" {
		return new(big.Int).Lsh(big.NewInt(1), 255), nil
	}
	return r.PinnedRPCReader.SuggestGasPrice(ctx)
}
func (r pinnedBoundaryReader) EstimateGasAtHash(ctx context.Context, msg ethereum.CallMsg, hash common.Hash) (uint64, error) {
	if r.mode == "estimate error" {
		return 0, context.DeadlineExceeded
	}
	return r.PinnedRPCReader.EstimateGasAtHash(ctx, msg, hash)
}

func TestPinnedPoolStateRejectsIncompleteReads(t *testing.T) {
	for _, native := range []bool{false, true} {
		chain := repair38OpenChain(t, native)
		block, pool := chain.peer.blocks[1], chain.peer.epoch.Pool
		for _, signature := range []string{"protocolConfig()", "usdlAddress()", "eventEmitter()", "factory()", "getReserves()", "opticalAddress()", "guardian()", "paused()"} {
			t.Run(chain.engine.manifest.DeploymentVersion+"/"+signature, func(t *testing.T) {
				engine := *chain.engine
				engine.reader = pinnedBoundaryReader{PinnedRPCReader: chain.reader, address: pool, selector: crypto.Keccak256([]byte(signature))[:4]}
				if _, err := engine.poolState(context.Background(), pool, block); err == nil {
					t.Fatal("unavailable pool state admitted")
				}
			})
		}
		for _, mode := range []string{"code error", "storage error"} {
			if native && mode == "storage error" {
				continue
			}
			engine := *chain.engine
			engine.reader = pinnedBoundaryReader{PinnedRPCReader: chain.reader, address: pool, mode: mode}
			if _, err := engine.poolState(context.Background(), pool, block); err == nil {
				t.Fatal("unavailable runtime identity admitted")
			}
		}
		for _, signature := range []string{"getReserves()", "opticalAddress()", "guardian()", "paused()"} {
			chain.peer.override(pool, signature, []byte{2})
			if _, err := chain.engine.poolState(context.Background(), pool, block); err == nil {
				t.Fatal("noncanonical pool ABI admitted")
			}
			chain.peer.mu.Lock()
			for _, pin := range chain.peer.blocks {
				delete(chain.peer.overrides, repair38Key(pool, repair38Call(t, signature), pin))
			}
			chain.peer.mu.Unlock()
		}
		if _, err := chain.engine.poolState(context.Background(), common.Address{}, block); err == nil {
			t.Fatal("absent pool identity admitted")
		}
		if native {
			for _, signature := range []string{"tokenAddress()", "feeAccumulator()"} {
				engine := *chain.engine
				engine.reader = pinnedBoundaryReader{PinnedRPCReader: chain.reader, address: pool, selector: crypto.Keccak256([]byte(signature))[:4]}
				if _, err := engine.poolState(context.Background(), pool, block); err == nil {
					t.Fatal("missing native pool wiring admitted")
				}
			}
			registry := chain.peer.address("PoolRegistry")
			for _, signature := range []string{"isRegisteredPool(address)", "getPoolByToken(address)", "getPoolMetadata(address)"} {
				engine := *chain.engine
				engine.reader = pinnedBoundaryReader{PinnedRPCReader: chain.reader, address: registry, selector: crypto.Keccak256([]byte(signature))[:4]}
				if _, err := engine.poolState(context.Background(), pool, block); err == nil {
					t.Fatal("unavailable registration binding admitted")
				}
			}
		}
	}
}

func TestCreationCostChecksMetadataAndGasDependencies(t *testing.T) {
	chain := repair38OpenChain(t, true)
	ctx := context.Background()
	block := chain.peer.blocks[1]
	policy := deployment.Policy{Now: time.Now(), MaxAge: time.Hour, Timeout: 10 * time.Second}
	attestation, err := deployment.Attest(ctx, chain.reader, chain.engine.manifest, block, policy)
	if err != nil {
		t.Fatal(err)
	}
	request := chain.request(Creation, block)
	request.CreationPath = "Router"
	for name, edit := range map[string]func(*PreflightRequest, *deployment.ActivationAttestation){
		"path":     func(r *PreflightRequest, _ *deployment.ActivationAttestation) { r.CreationPath = "unknown" },
		"metadata": func(r *PreflightRequest, _ *deployment.ActivationAttestation) { r.Name = "" },
		"optical":  func(r *PreflightRequest, _ *deployment.ActivationAttestation) { r.Optical = chain.peer.epoch.Token },
		"fee":      func(_ *PreflightRequest, a *deployment.ActivationAttestation) { a.CreationFeeRaw = "0" },
		"approval account": func(r *PreflightRequest, _ *deployment.ActivationAttestation) {
			r.CreationStage = "approval"
			r.Recipient = chain.peer.epoch.Token
		},
		"attestation": func(_ *PreflightRequest, a *deployment.ActivationAttestation) { a.BlockHash = "" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate, att := request, attestation
			edit(&candidate, &att)
			if _, err := chain.engine.creationCost(ctx, candidate, att); err == nil {
				t.Fatal("unverified creation cost admitted")
			}
		})
	}
	for _, mode := range []string{"price error", "price overflow", "cost overflow", "estimate error"} {
		engine := *chain.engine
		engine.reader = pinnedBoundaryReader{PinnedRPCReader: chain.reader, mode: mode}
		if _, err := engine.creationCost(ctx, request, attestation); err == nil {
			t.Fatal("unbounded gas cost admitted")
		}
	}
	request.CreationStage = "approval"
	engine := *chain.engine
	engine.reader = pinnedBoundaryReader{PinnedRPCReader: chain.reader, address: chain.peer.address("USDL"), selector: crypto.Keccak256([]byte("approve(address,uint256)"))[:4]}
	if _, err := engine.creationCost(ctx, request, attestation); err == nil {
		t.Fatal("unavailable approval simulation admitted")
	}
	chain.peer.override(chain.peer.address("USDL"), "approve(address,uint256)", repair38Word(1), chain.peer.address(deployment.LaunchpadPrecompileRole), big.NewInt(1000000))
	cost, err := chain.engine.creationCost(ctx, request, attestation)
	var unavailable Unavailable
	if !errors.As(err, &unavailable) || unavailable.DependencyID != "transaction_simulation" || cost.GasEstimateAvailable || cost.Spender != chain.peer.address(deployment.LaunchpadPrecompileRole).Hex() || cost.ProtocolFeeRaw != "1000000" {
		t.Fatalf("approval cost binding: %+v %v", cost, err)
	}
}

func TestPreflightRejectsMalformedBodiesAndRevisionChanges(t *testing.T) {
	chain := repair38OpenChain(t, true)
	ctx := context.Background()
	request := chain.request(PlainQuote, chain.peer.blocks[1])
	request.Pool = chain.peer.epoch.Pool
	for _, wire := range []string{`{`, `{} {}`, `{"family":"plain_quote","family":"creation"}`, strings.Repeat("x", (1<<16)+1)} {
		response := httptest.NewRecorder()
		chain.engine.Handler(PlainQuote).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(wire)))
		if response.Code < 400 {
			t.Fatalf("invalid preflight body admitted: %d", response.Code)
		}
	}
	good, err := chain.engine.Preflight(ctx, request)
	if err != nil || !good.Supported {
		t.Fatalf("initial pinned preflight: %+v %v", good, err)
	}
	for _, mode := range []string{"family", "revision", "block", "pool", "context"} {
		t.Run(mode, func(t *testing.T) {
			candidate := request
			prior := good
			bounded := ctx
			switch mode {
			case "family":
				candidate.Family = Family("unknown")
			case "revision":
				candidate.ExpectedRevision = strings.Repeat("b", 64)
			case "block":
				candidate.BlockHash = common.Hash{}
			case "pool":
				candidate.Pool = common.Address{}
			case "context":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				bounded = cancelled
			}
			got, err := chain.engine.Preflight(bounded, candidate)
			if err == nil && got.Supported {
				t.Fatal("invalid pinned preflight admitted")
			}
			if _, err := chain.engine.RevalidateAt(bounded, prior, candidate); err == nil {
				t.Fatal("changed preflight admitted as current")
			}
		})
	}
	if _, err := chain.engine.RevalidateAt(ctx, Preflight{}, request); err == nil {
		t.Fatal("absent prior preflight admitted")
	}
	var capture bytes.Buffer
	recorder := &measurementCapture{reader: chain.reader, writer: &capture, hash: sha256.New(), maxBytes: 1 << 20}
	if _, err := recorder.EstimateGasAtHash(ctx, ethereum.CallMsg{}, common.Hash{}); err == nil {
		t.Fatal("unbound simulation admitted")
	}
	if _, err := recorder.ReportedBlockByHash(ctx, common.Hash{}); err == nil {
		t.Fatal("unbound receipt block admitted")
	}
	if _, err := (&PreflightEngine{}).Capabilities(ctx, common.Hash{}); err == nil {
		t.Fatal("unconfigured capabilities admitted")
	}
	if _, err := NewPreflightEngine(nil, deployment.Current(), "invalid", "invalid", time.Hour, time.Second); err == nil {
		t.Fatal("unconfigured engine admitted")
	}
	if _, err := encodeCall("invalid"); err == nil {
		t.Fatal("invalid ABI signature admitted")
	}
	if _, err := encodeCall("approve(address,uint256)", "invalid", big.NewInt(1)); err == nil {
		t.Fatal("invalid ABI argument admitted")
	}
	if code, _, ok := Refused(errors.New("unavailable")); ok || code != "" {
		t.Fatal("unclassified dependency treated as deterministic refusal")
	}

}
