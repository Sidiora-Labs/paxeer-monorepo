package capabilities

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type preflightReuseReads struct {
	chain, header, code, storage, call int
}

type preflightReuseReader struct {
	*PinnedRPCReader
	mu    sync.Mutex
	reads preflightReuseReads
	hold  <-chan struct{}
}

func (r *preflightReuseReader) counts() preflightReuseReads {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads
}

func (r *preflightReuseReader) ChainID(ctx context.Context) (*big.Int, error) {
	r.mu.Lock()
	r.reads.chain++
	r.mu.Unlock()
	if r.hold != nil {
		select {
		case <-r.hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.PinnedRPCReader.ChainID(ctx)
}

func (r *preflightReuseReader) HeaderByReportedHash(ctx context.Context, block common.Hash) (*types.Header, error) {
	r.mu.Lock()
	r.reads.header++
	r.mu.Unlock()
	return r.PinnedRPCReader.HeaderByReportedHash(ctx, block)
}

func (r *preflightReuseReader) CodeAtHash(ctx context.Context, address common.Address, block common.Hash) ([]byte, error) {
	r.mu.Lock()
	r.reads.code++
	r.mu.Unlock()
	return r.PinnedRPCReader.CodeAtHash(ctx, address, block)
}

func (r *preflightReuseReader) StorageAtHash(ctx context.Context, address common.Address, slot, block common.Hash) ([]byte, error) {
	r.mu.Lock()
	r.reads.storage++
	r.mu.Unlock()
	return r.PinnedRPCReader.StorageAtHash(ctx, address, slot, block)
}

func (r *preflightReuseReader) CallContractAtHash(ctx context.Context, call ethereum.CallMsg, block common.Hash) ([]byte, error) {
	r.mu.Lock()
	r.reads.call++
	r.mu.Unlock()
	return r.PinnedRPCReader.CallContractAtHash(ctx, call, block)
}

func preflightReuseChain(t *testing.T, hold <-chan struct{}, timeout time.Duration) (*preflightFixtureChain, *preflightReuseReader) {
	t.Helper()
	chain := preflightFixtureOpenChain(t, false)
	reader := &preflightReuseReader{PinnedRPCReader: chain.reader, hold: hold}
	engine, err := NewPreflightEngine(reader, chain.engine.manifest, chain.engine.poolRuntime.Hex(), strings.Repeat("a", 64), time.Hour, timeout)
	if err != nil {
		t.Fatal(err)
	}
	chain.engine = engine
	return chain, reader
}

func preflightReuseAttestation(t *testing.T, engine *PreflightEngine, block common.Hash, now time.Time) deployment.ActivationAttestation {
	t.Helper()
	attestation, err := engine.Attestation(t.Context(), block, now)
	if err != nil || !attestation.Eligible || len(attestation.Issues) != 0 {
		t.Fatalf("block attestation = %+v, %v", attestation, err)
	}
	return attestation
}

func TestPreflightAttestationReuseKeptIdentity(t *testing.T) {
	chain, reader := preflightReuseChain(t, nil, 10*time.Second)
	block := chain.peer.blocks[1]
	first := preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	before := reader.counts()
	if before.chain != 1 || before.header != 1 || before.code == 0 || before.storage == 0 || before.call == 0 {
		t.Fatalf("identity read counts = %+v", before)
	}
	second := preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	if !reflect.DeepEqual(first, second) || reader.counts() != before {
		t.Fatalf("kept identity changed or was reread: %+v", reader.counts())
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := chain.engine.Attestation(ctx, block, time.Now()); !errors.Is(err, context.Canceled) || reader.counts() != before {
		t.Fatalf("canceled asking = %v, counts %+v", err, reader.counts())
	}
	deadline, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer stop()
	if _, err := chain.engine.Attestation(deadline, block, time.Now()); !errors.Is(err, context.DeadlineExceeded) || reader.counts() != before {
		t.Fatalf("expired asking = %v, counts %+v", err, reader.counts())
	}
}

type preflightReuseWaitContext struct {
	context.Context
	waiting chan<- struct{}
}

func (c preflightReuseWaitContext) Done() <-chan struct{} {
	c.waiting <- struct{}{}
	return c.Context.Done()
}

func TestPreflightAttestationReuseConcurrentAskers(t *testing.T) {
	for _, withIssue := range []bool{false, true} {
		t.Run(map[bool]string{false: "eligible", true: "issue"}[withIssue], func(t *testing.T) {
			hold := make(chan struct{})
			chain, reader := preflightReuseChain(t, hold, 10*time.Second)
			block := chain.peer.blocks[1]
			if withIssue {
				chain.peer.mu.Lock()
				chain.peer.codes[chain.peer.address("Router")] = []byte{0}
				chain.peer.mu.Unlock()
			}
			const askers = 16
			waiting := make(chan struct{}, askers)
			answers := make([]deployment.ActivationAttestation, askers)
			errs := make([]error, askers)
			var workers sync.WaitGroup
			now := time.Now().UTC()
			for i := range answers {
				workers.Add(1)
				go func() {
					defer workers.Done()
					answers[i], errs[i] = chain.engine.Attestation(preflightReuseWaitContext{t.Context(), waiting}, block, now)
				}()
			}
			for range askers {
				<-waiting
			}
			close(hold)
			workers.Wait()
			for i, answer := range answers {
				if errs[i] != nil || answer.Eligible == withIssue || !reflect.DeepEqual(answer, answers[0]) {
					t.Fatalf("shared answer %d = %+v, %v", i, answer, errs[i])
				}
			}
			if counts := reader.counts(); counts.chain != 1 || counts.header != 1 {
				t.Fatalf("concurrent identity reads = %+v", counts)
			}
			if withIssue {
				answers[0].Issues[0].Code = "changed"
				identity := answers[0].Identities["Router"]
				identity.Issues[0].Code = "changed"
				answers[0].Identities["Router"] = identity
				if answers[1].Issues[0].Code == "changed" || answers[1].Identities["Router"].Issues[0].Code == "changed" {
					t.Fatal("shared answers share issue lists")
				}
			}
		})
	}
}

func TestPreflightAttestationReuseAskerCancellation(t *testing.T) {
	hold := make(chan struct{})
	chain, reader := preflightReuseChain(t, hold, 10*time.Second)
	block := chain.peer.blocks[1]
	waiting := make(chan struct{}, 2)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	first := make(chan error, 1)
	now := time.Now().UTC()
	go func() {
		_, err := chain.engine.Attestation(preflightReuseWaitContext{ctx, waiting}, block, now)
		first <- err
	}()
	<-waiting
	second := make(chan error, 1)
	go func() {
		_, err := chain.engine.Attestation(preflightReuseWaitContext{t.Context(), waiting}, block, now)
		second <- err
	}()
	<-waiting
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled asker = %v", err)
	}
	select {
	case err := <-second:
		t.Fatalf("shared read ended with its asker: %v", err)
	default:
	}
	close(hold)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	if counts := reader.counts(); counts.chain != 1 {
		t.Fatalf("canceled asker replaced the shared read: %+v", counts)
	}
}

func TestPreflightAttestationReuseEngineTimeout(t *testing.T) {
	hold := make(chan struct{})
	chain, reader := preflightReuseChain(t, hold, time.Second)
	block := chain.peer.blocks[1]
	_, err := chain.engine.Attestation(t.Context(), block, time.Now().UTC())
	var issue deployment.Issue
	if !errors.As(err, &issue) || issue.Code != "chain_read_failed" || reader.counts().header != 0 {
		t.Fatalf("identity read timeout = %v, counts %+v", err, reader.counts())
	}
	close(hold)
	preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	if counts := reader.counts(); counts.chain != 2 {
		t.Fatalf("timed out read was retained: %+v", counts)
	}
}

func TestPreflightAttestationReuseFailedRead(t *testing.T) {
	chain, reader := preflightReuseChain(t, nil, 10*time.Second)
	block := chain.peer.blocks[1]
	chain.peer.mu.Lock()
	header := chain.peer.headers[block]
	delete(chain.peer.headers, block)
	chain.peer.mu.Unlock()
	_, err := chain.engine.Attestation(t.Context(), block, time.Now().UTC())
	var issue deployment.Issue
	if !errors.As(err, &issue) || issue.Code != "block_identity_unverified" {
		t.Fatalf("unavailable header = %v", err)
	}
	chain.peer.mu.Lock()
	chain.peer.headers[block] = header
	chain.peer.mu.Unlock()
	preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	if counts := reader.counts(); counts.chain != 2 || counts.header != 2 || counts.code == 0 {
		t.Fatalf("failed read was retained: %+v", counts)
	}
}

func TestPreflightAttestationReuseIssueNotKept(t *testing.T) {
	chain, reader := preflightReuseChain(t, nil, 10*time.Second)
	block := chain.peer.blocks[1]
	address := chain.peer.address("Router")
	chain.peer.mu.Lock()
	code := chain.peer.codes[address]
	chain.peer.codes[address] = []byte{0}
	chain.peer.mu.Unlock()
	for range 2 {
		attestation, err := chain.engine.Attestation(t.Context(), block, time.Now().UTC())
		if err != nil || attestation.Eligible || len(attestation.Issues) == 0 || attestation.Identities["Router"].Verified {
			t.Fatalf("unverified identity = %+v, %v", attestation, err)
		}
	}
	chain.peer.mu.Lock()
	chain.peer.codes[address] = code
	chain.peer.mu.Unlock()
	preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	before := reader.counts()
	preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	if before.chain != 3 || reader.counts() != before {
		t.Fatalf("issue or eligible retention differs: %+v", reader.counts())
	}
}

func TestPreflightAttestationReuseBlockAge(t *testing.T) {
	chain, reader := preflightReuseChain(t, nil, 10*time.Second)
	block := chain.peer.blocks[1]
	first := preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	before := reader.counts()
	atBound := preflightReuseAttestation(t, chain.engine, block, first.BlockTime.Add(chain.engine.maxAge))
	if atBound.ObservedAt != first.ObservedAt {
		t.Fatal("kept observation time changed")
	}
	for _, now := range []time.Time{first.BlockTime.Add(chain.engine.maxAge + time.Nanosecond), first.BlockTime.Add(-time.Nanosecond), time.Unix(-1, 0)} {
		_, err := chain.engine.Attestation(t.Context(), block, now)
		var issue deployment.Issue
		if !errors.As(err, &issue) || issue.Code != "attestation_stale" {
			t.Fatalf("block age = %v", err)
		}
	}
	_, err := chain.engine.Attestation(t.Context(), block, time.Time{})
	var issue deployment.Issue
	if !errors.As(err, &issue) || issue.Code != "attestation_policy_invalid" {
		t.Fatalf("empty asking time = %v", err)
	}
	if reader.counts() != before {
		t.Fatal("age check reread the identity")
	}
}

func TestPreflightAttestationReuseAnswerIsolation(t *testing.T) {
	chain, _ := preflightReuseChain(t, nil, 10*time.Second)
	block := chain.peer.blocks[1]
	first := preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	want := clonePreflightAttestation(first)
	delete(first.Identities, "Router")
	identity := first.Identities["Quoter"]
	identity.Verified = false
	identity.Issues = append(identity.Issues, deployment.Issue{Code: "changed"})
	first.Identities["Quoter"] = identity
	first.PolicyValues["minFeeBps()"] = "changed"
	first.Issues = append(first.Issues, deployment.Issue{Code: "changed"})
	second := preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("answer mutation reached kept identity: %+v", second)
	}
}

func TestPreflightAttestationReuseEvictsLowestBlock(t *testing.T) {
	chain, reader := preflightReuseChain(t, nil, 10*time.Second)
	peer := chain.peer
	template := peer.blocks[1]
	blocks := make([]common.Hash, 17)
	peer.mu.Lock()
	for i := range blocks {
		header := types.CopyHeader(peer.headers[template])
		header.Number = new(big.Int).SetUint64(chain.engine.manifest.ActivationHeight + uint64(i+50))
		if i == 1 {
			header.Number = new(big.Int).SetUint64(chain.engine.manifest.ActivationHeight + 1)
		}
		blocks[i] = header.Hash()
		peer.headers[blocks[i]] = header
		for key, value := range peer.calls {
			if strings.HasSuffix(key, template.Hex()) {
				peer.calls[strings.TrimSuffix(key, template.Hex())+blocks[i].Hex()] = value
			}
		}
	}
	peer.mu.Unlock()
	for _, block := range blocks[:16] {
		preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
	}
	preflightReuseAttestation(t, chain.engine, blocks[1], time.Now().UTC())
	preflightReuseAttestation(t, chain.engine, blocks[16], time.Now().UTC())
	before := reader.counts()
	for i, block := range blocks {
		if i != 1 {
			preflightReuseAttestation(t, chain.engine, block, time.Now().UTC())
		}
	}
	if before.chain != 17 || reader.counts() != before {
		t.Fatalf("higher block was evicted: %+v", reader.counts())
	}
	preflightReuseAttestation(t, chain.engine, blocks[1], time.Now().UTC())
	if reader.counts().chain != 18 {
		t.Fatal("lowest block was retained")
	}
	chain.engine.attestations.mu.Lock()
	defer chain.engine.attestations.mu.Unlock()
	if len(chain.engine.attestations.kept) != 16 {
		t.Fatal("attestation retention exceeds sixteen block hashes")
	}
}

func TestPreflightAttestationReuseAcrossEngineMethods(t *testing.T) {
	chain, reader := preflightReuseChain(t, nil, 10*time.Second)
	block := chain.peer.blocks[1]
	publication, err := chain.engine.Capabilities(t.Context(), block)
	if err != nil || publication.BlockHash != block.Hex() {
		t.Fatalf("capabilities = %+v, %v", publication, err)
	}
	before := reader.counts()
	endpoints := deployment.PublicEndpoints{RPC: "https://rpc.example", Explorer: "https://scan.example", AllowedRPCs: []string{"https://rpc.example"}, AllowedExplorers: []string{"https://scan.example"}}
	exported, err := chain.engine.SerialExport(t.Context(), block, endpoints, chain.engine.manifest.Addresses(), chain.engine.manifest.Addresses())
	if err != nil || exported.SchemaVersion != 2 {
		t.Fatalf("serial export = %+v, %v", exported, err)
	}
	request := chain.request(Creation, block)
	request.CreationPath = "Router"
	result, err := chain.engine.Preflight(t.Context(), request)
	if err != nil || !result.Supported || result.CreationCost == nil || !result.CreationCost.GasEstimateAvailable {
		t.Fatalf("preflight = %+v, %v", result, err)
	}
	if before.chain != 1 || reader.counts() != before {
		t.Fatalf("engine methods reread identity: before %+v, after %+v", before, reader.counts())
	}
}
