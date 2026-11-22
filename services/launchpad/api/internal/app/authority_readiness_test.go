package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// authorityStateReader reads isolated Solidity accounts and executes their
// getters in the EVM at a single hash, without a network listener.
type authorityStateReader struct {
	state  *state.StateDB
	header *types.Header
	chain  *params.ChainConfig
	reads  atomic.Int64
}

func (r *authorityStateReader) check(ctx context.Context, hash common.Hash) error {
	r.reads.Add(1)
	if err := ctx.Err(); err != nil {
		return err
	}
	if hash != r.header.Hash() {
		return ethereum.NotFound
	}
	return nil
}
func (r *authorityStateReader) ChainID(ctx context.Context) (*big.Int, error) {
	if err := r.check(ctx, r.header.Hash()); err != nil {
		return nil, err
	}
	return new(big.Int).Set(r.chain.ChainID), nil
}
func (r *authorityStateReader) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	if err := r.check(ctx, hash); err != nil {
		return nil, err
	}
	return types.CopyHeader(r.header), nil
}
func (r *authorityStateReader) CodeAtHash(ctx context.Context, address common.Address, hash common.Hash) ([]byte, error) {
	if err := r.check(ctx, hash); err != nil {
		return nil, err
	}
	return append([]byte(nil), r.state.GetCode(address)...), nil
}
func (r *authorityStateReader) StorageAtHash(ctx context.Context, address common.Address, key, hash common.Hash) ([]byte, error) {
	if err := r.check(ctx, hash); err != nil {
		return nil, err
	}
	return r.state.GetState(address, key).Bytes(), nil
}
func (r *authorityStateReader) CallContractAtHash(ctx context.Context, call ethereum.CallMsg, hash common.Hash) ([]byte, error) {
	if err := r.check(ctx, hash); err != nil {
		return nil, err
	}
	out, _, err := runtime.Call(*call.To, call.Data, &runtime.Config{
		State: r.state.Copy(), ChainConfig: r.chain, BlockNumber: r.header.Number,
		Time: r.header.Time, GasLimit: 30_000_000,
	})
	return out, err
}

func newAuthorityReadinessState(t *testing.T) (*networkAuthority, *authorityStateReader, committedHead) {
	t.Helper()
	var artifacts struct {
		Contracts map[string]struct {
			Runtime    string                                   `json:"runtime"`
			Immutables map[string][]struct{ Start, Length int } `json:"immutableReferences"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(repoFile(t, "sdk/launchpad/shared/config/deployment/testdata/evm-artifacts.json"), &artifacts); err != nil {
		t.Fatal(err)
	}
	var manifest deployment.Manifest
	if err := json.Unmarshal(repoFile(t, "sdk/launchpad/shared/config/deployment/history/paxeer125-a35ff7c6f532d2f9.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	st, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatal(err)
	}
	code := func(name string, self common.Address) []byte {
		a, ok := artifacts.Contracts[name]
		if !ok {
			t.Fatalf("contract artifact absent: %s", name)
		}
		b := common.FromHex(a.Runtime)
		for _, refs := range a.Immutables {
			for _, ref := range refs {
				copy(b[ref.Start:ref.Start+ref.Length], common.LeftPadBytes(self.Bytes(), ref.Length))
			}
		}
		return b
	}
	slot := func(n int64) common.Hash { return common.BigToHash(big.NewInt(n)) }
	word := func(a string) common.Hash { return common.BytesToHash(common.HexToAddress(a).Bytes()) }
	for name, role := range manifest.Roles {
		address := common.HexToAddress(role.Address)
		var deployed []byte
		switch role.Kind {
		case "erc1967":
			deployed = code("UUPSProxy", address)
			st.SetState(address, deployment.ImplementationSlot, word(role.ImplementationAddress))
			impl := common.HexToAddress(role.ImplementationAddress)
			implementation := code(name, impl)
			if crypto.Keccak256Hash(implementation).Hex() != role.ImplementationHash {
				t.Fatalf("%s implementation identity differs", name)
			}
			st.SetCode(impl, implementation, tracing.CodeChangeUnspecified)
		case "beacon":
			deployed = code("PoolBeacon", address)
			st.SetState(address, slot(0), word(role.ImplementationAddress))
			st.SetState(address, slot(1), word(manifest.Governance.ExpectedBeaconOwner))
			impl := common.HexToAddress(role.ImplementationAddress)
			implementation := code("SidioraPool", impl)
			if crypto.Keccak256Hash(implementation).Hex() != role.ImplementationHash {
				t.Fatal("pool implementation identity differs")
			}
			st.SetCode(impl, implementation, tracing.CodeChangeUnspecified)
		case "direct":
			deployed = code("MockERC20", address)
			st.SetState(address, slot(2), slot(6))
			role.RuntimeHash = crypto.Keccak256Hash(deployed).Hex()
			manifest.Roles[name] = role
		default:
			t.Fatalf("unsupported account kind %s", role.Kind)
		}
		if crypto.Keccak256Hash(deployed).Hex() != role.RuntimeHash {
			t.Fatalf("%s runtime identity differs", name)
		}
		st.SetCode(address, deployed, tracing.CodeChangeUnspecified)
		if name == "ProtocolConfig" {
			st.SetState(address, slot(1), word(manifest.Roles["USDL"].Address))
			st.SetState(address, slot(11), slot(1_000_000))
		}
	}
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(deployment.ChainID)
	reader := &authorityStateReader{state: st, chain: &chainConfig,
		header: &types.Header{Number: new(big.Int).SetUint64(manifest.ActivationHeight + 1), Time: uint64(time.Now().Unix()), GasLimit: 30_000_000, Difficulty: new(big.Int)}}
	engine, err := capabilities.NewPreflightEngine(reader, manifest, developmentPoolRuntime, developmentSerialInterface, authorityMaxAge, authorityAttestationTimeout)
	if err != nil {
		t.Fatal(err)
	}
	head := committedHead{Number: reader.header.Number.String(), Hash: reader.header.Hash().Hex()}
	a := &networkAuthority{reader: reader, engine: engine, config: networkAuthorityConfig{
		SchemaVersion: 2, Manifest: manifest, GoAddresses: manifest.Addresses(), FrontendAddresses: manifest.Addresses(),
		Endpoints: deployment.PublicEndpoints{RPC: "https://rpc.example", Explorer: "https://explorer.example", AllowedRPCs: []string{"https://rpc.example"}, AllowedExplorers: []string{"https://explorer.example"}},
	}}
	return a, reader, head
}

func TestAuthorityReadinessFreshnessAndRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, reader, head := newAuthorityReadinessState(t)
		headReads := 0
		a.readHead = func(ctx context.Context) (committedHead, error) { headReads++; return head, ctx.Err() }
		check := func(ready bool) {
			t.Helper()
			beforeReader, beforeHead := reader.reads.Load(), headReads
			for range 8 {
				if err := a.Ready(t.Context()); (err == nil) != ready {
					t.Fatalf("readiness = %v, want ready %t", err, ready)
				}
			}
			if reader.reads.Load() != beforeReader || headReads != beforeHead {
				t.Fatal("readiness read an authority dependency")
			}
		}
		check(false)
		if err := a.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		check(true)
		if headReads != 1 || reader.reads.Load() == 0 || a.lastEnvelope.CommittedHead != head {
			t.Fatal("refresh did not attest the committed head")
		}
		raw, err := json.Marshal(a.lastEnvelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := interfaces.ValidateJSON("ConnectedNetworkAuthorityResponse", raw); err != nil {
			t.Fatal(err)
		}
		builtAt, envelope := a.builtAt, a.lastEnvelope
		time.Sleep(authorityRefreshInterval)
		head.Hash = common.HexToHash("0x01").Hex()
		if err := a.refresh(t.Context()); err == nil {
			t.Fatal("unavailable committed block refreshed the envelope")
		}
		if a.builtAt != builtAt || a.lastEnvelope.CommittedHead != envelope.CommittedHead {
			t.Fatal("failed refresh replaced the saved envelope")
		}
		check(true)
		time.Sleep(authorityMaxAge - authorityRefreshInterval)
		check(true)
		time.Sleep(time.Nanosecond)
		check(false)
		reader.header.Time = uint64(time.Now().Unix())
		reader.header.Number.Add(reader.header.Number, big.NewInt(1))
		head = committedHead{Number: reader.header.Number.String(), Hash: reader.header.Hash().Hex()}
		if err := a.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		check(true)
		if !a.builtAt.After(builtAt) || a.lastEnvelope.CommittedHead != head {
			t.Fatal("successful refresh did not replace the saved envelope")
		}
	})
}

func TestAuthorityReadinessRefreshLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, reader, head := newAuthorityReadinessState(t)
		var headReads atomic.Int64
		var headMu sync.Mutex
		hold := make(chan struct{})
		var canceled atomic.Bool
		a.readHead = func(ctx context.Context) (committedHead, error) {
			headReads.Add(1)
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > authorityAttestationTimeout {
				return committedHead{}, fmt.Errorf("refresh deadline exceeds attestation policy")
			}
			headMu.Lock()
			currentHead, currentHold := head, hold
			headMu.Unlock()
			select {
			case <-currentHold:
				return currentHead, nil
			case <-ctx.Done():
				canceled.Store(true)
				return committedHead{}, ctx.Err()
			}
		}
		savedAt := func() time.Time {
			a.mu.RLock()
			defer a.mu.RUnlock()
			return a.builtAt
		}
		if err := a.Ready(t.Context()); err == nil {
			t.Fatal("authority ready before refresh")
		}
		a.startRefresh(t.Context())
		defer a.Close()
		synctest.Wait()
		if headReads.Load() != 1 || reader.reads.Load() != 0 {
			t.Fatal("first refresh did not wait for the committed head")
		}
		if err := a.Ready(t.Context()); err == nil {
			t.Fatal("authority ready before first refresh completed")
		}
		close(hold)
		synctest.Wait()
		if err := a.Ready(t.Context()); err != nil {
			t.Fatal(err)
		}
		builtAt := savedAt()
		headMu.Lock()
		head.Hash = common.HexToHash("0x01").Hex()
		headMu.Unlock()
		time.Sleep(authorityRefreshInterval)
		synctest.Wait()
		if headReads.Load() != 2 || savedAt() != builtAt {
			t.Fatal("failed periodic refresh changed the saved envelope")
		}
		if err := a.Ready(t.Context()); err != nil {
			t.Fatal(err)
		}
		headMu.Lock()
		head.Hash = reader.header.Hash().Hex()
		headMu.Unlock()
		time.Sleep(authorityRefreshInterval)
		synctest.Wait()
		if headReads.Load() != 3 || !savedAt().After(builtAt) {
			t.Fatal("periodic refresh did not recover at the next interval")
		}
		headMu.Lock()
		hold = make(chan struct{})
		headMu.Unlock()
		time.Sleep(authorityRefreshInterval)
		synctest.Wait()
		if headReads.Load() != 4 {
			t.Fatalf("periodic head reads = %s", strconv.FormatInt(headReads.Load(), 10))
		}
		time.Sleep(authorityAttestationTimeout - time.Nanosecond)
		synctest.Wait()
		if headReads.Load() != 4 {
			t.Fatal("refreshes overlapped")
		}
		a.Close()
		if !canceled.Load() {
			t.Fatal("close did not cancel the active refresh")
		}
		select {
		case <-a.refreshDone:
		default:
			t.Fatal("close returned before refresh stopped")
		}
		reads := reader.reads.Load()
		time.Sleep(3 * authorityRefreshInterval)
		synctest.Wait()
		if headReads.Load() != 4 || reader.reads.Load() != reads {
			t.Fatal("refresh continued after close")
		}
	})
}

func TestAuthorityReadinessExistingAuthorityCases(t *testing.T) {
	t.Run("routes", TestNetworkAuthorityRoutesFailClosedWithoutCommittedSource)
	t.Run("bindings", TestNetworkAuthorityBindsReviewedManifestAndAddresses)
	t.Run("response bound", TestNetworkRPCResponseBound)
	t.Run("attestation projection", TestAttestationProjectionCoversBuilderFields)
}
