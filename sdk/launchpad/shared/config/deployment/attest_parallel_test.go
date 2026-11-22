package deployment

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type sideBySideReader struct {
	Reader
	mu                        sync.Mutex
	active, peak, afterCancel int
	reads                     []string
	fail                      map[string]bool
	failed                    map[string]int
	gate                      chan struct{}
	target                    int
	hold                      bool
	cancel                    context.CancelFunc
}

func (r *sideBySideReader) begin(ctx context.Context, key string, state bool) (func(), error) {
	r.mu.Lock()
	r.active++
	r.peak = max(r.peak, r.active)
	r.reads = append(r.reads, key)
	if ctx.Err() != nil {
		r.afterCancel++
	}
	if state && r.gate != nil && r.active >= r.target {
		close(r.gate)
		r.gate = nil
	}
	gate := r.gate
	if state && r.cancel != nil && r.active >= r.target {
		r.cancel()
		r.cancel = nil
	}
	fail := r.fail[key]
	if fail {
		r.failed[key]++
	}
	r.mu.Unlock()
	end := func() {
		r.mu.Lock()
		r.active--
		r.mu.Unlock()
	}
	if state {
		if r.hold {
			<-ctx.Done()
		} else if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
			}
		}
		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
		}
	}
	if err := ctx.Err(); err != nil {
		return end, err
	}
	if fail {
		return end, errors.New("read unavailable")
	}
	return end, nil
}

func (r *sideBySideReader) ChainID(ctx context.Context) (*big.Int, error) {
	end, err := r.begin(ctx, "chain", false)
	defer end()
	if err != nil {
		return nil, err
	}
	return r.Reader.ChainID(ctx)
}

func (r *sideBySideReader) HeaderByHash(ctx context.Context, block common.Hash) (*types.Header, error) {
	end, err := r.begin(ctx, "header", false)
	defer end()
	if err != nil {
		return nil, err
	}
	return r.Reader.HeaderByHash(ctx, block)
}

func (r *sideBySideReader) CodeAtHash(ctx context.Context, address common.Address, block common.Hash) ([]byte, error) {
	end, err := r.begin(ctx, "code/"+address.Hex(), true)
	defer end()
	if err != nil {
		return nil, err
	}
	return r.Reader.CodeAtHash(ctx, address, block)
}

func (r *sideBySideReader) StorageAtHash(ctx context.Context, address common.Address, slot, block common.Hash) ([]byte, error) {
	end, err := r.begin(ctx, "storage/"+address.Hex(), true)
	defer end()
	if err != nil {
		return nil, err
	}
	return r.Reader.StorageAtHash(ctx, address, slot, block)
}

func (r *sideBySideReader) CallContractAtHash(ctx context.Context, call ethereum.CallMsg, block common.Hash) ([]byte, error) {
	key := "call/" + call.To.Hex() + "/" + common.Bytes2Hex(call.Data)
	end, err := r.begin(ctx, key, true)
	defer end()
	if err != nil {
		return nil, err
	}
	return r.Reader.CallContractAtHash(ctx, call, block)
}

func sideBySideEVM(t *testing.T) (Manifest, Reader, common.Hash, Policy) {
	t.Helper()
	m, reader, block, policy := newEVM(t, func(m *Manifest, alloc types.GenesisAlloc) {
		address := common.HexToAddress("0x2222222222222222222222222222222222222222")
		usdl := m.Roles[valueUSDL]
		m.Roles["Surface"] = Role{Address: address.Hex(), Kind: valueSurface, RuntimeHash: usdl.RuntimeHash}
		alloc[address] = alloc[common.HexToAddress(usdl.Address)]
	}, ChainID)
	m.Roles[LaunchpadPrecompileRole] = Role{Address: LaunchpadPrecompileAddress, Kind: valuePrecompile}
	kinds := map[string]bool{}
	for _, role := range m.Roles {
		kinds[role.Kind] = true
	}
	for _, kind := range []string{valueDirect, valueSurface, valuePrecompile, valueErc1967, valueBeacon, valueBeaconProxy} {
		if !kinds[kind] {
			t.Fatalf("missing role kind %s", kind)
		}
	}
	return m, reader, block, policy
}

func newSideBySideReader(reader Reader, limit int, fail map[string]bool) *sideBySideReader {
	return &sideBySideReader{Reader: reader, target: max(1, limit), gate: make(chan struct{}), fail: fail, failed: map[string]int{}}
}

func TestAttestSideBySideEquivalent(t *testing.T) {
	m, reader, block, policy := sideBySideEVM(t)
	callKey := func(role, signature string) string {
		return "call/" + common.HexToAddress(m.Roles[role].Address).Hex() + "/" + common.Bytes2Hex(selector(signature))
	}
	code := "code/" + common.HexToAddress(m.Roles["Router"].Address).Hex()
	storage := "storage/" + common.HexToAddress(m.Roles["FixturePool"].Address).Hex()
	owner := callKey(valuePoolBeacon, "owner()")
	value := callKey(valueProtocolConfig, "minFeeBps()")
	for _, tc := range []struct {
		name  string
		fail  map[string]bool
		issue string
	}{
		{"healthy", nil, ""},
		{"code", map[string]bool{code: true}, "code_read_failed"},
		{"storage", map[string]bool{storage: true}, valueIdentityUnverified},
		{"owner", map[string]bool{owner: true}, "governance_read_failed"},
		{"policy", map[string]bool{value: true}, "configuration_policy_read_failed"},
		{"creation", map[string]bool{callKey(valueProtocolConfig, "creationFee()"): true}, "creation_policy_read_failed"},
		{"ordered issues", map[string]bool{code: true, storage: true, owner: true, value: true}, "configuration_policy_read_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serial := newSideBySideReader(reader, 1, tc.fail)
			policy.ConcurrentReads = 1
			want, err := Attest(t.Context(), serial, m, block, policy)
			if err != nil {
				t.Fatal(err)
			}
			parallel := newSideBySideReader(reader, 8, tc.fail)
			policy.ConcurrentReads = 8
			got, err := Attest(t.Context(), parallel, m, block, policy)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("attestations differ:\nserial %+v\nparallel %+v", want, got)
			}
			if tc.issue == "" && !got.Eligible {
				t.Fatalf("healthy deployment refused: %+v", got.Issues)
			}
			if tc.issue != "" && (got.Eligible || !hasIssue(got, tc.issue, "")) {
				t.Fatalf("missing refusal: %+v", got.Issues)
			}
			if serial.peak != 1 || parallel.peak != 8 || parallel.active != 0 {
				t.Fatalf("read concurrency serial=%d parallel=%d active=%d", serial.peak, parallel.peak, parallel.active)
			}
			for key := range tc.fail {
				if serial.failed[key] != 1 || parallel.failed[key] != 1 {
					t.Fatalf("failure %s counts serial=%d parallel=%d", key, serial.failed[key], parallel.failed[key])
				}
			}
		})
	}
}

func TestAttestSideBySideLimits(t *testing.T) {
	m, reader, block, policy := sideBySideEVM(t)
	var serialReads []string
	for _, limit := range []int{0, 1, 2, 8, 16, -1, 17} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			r := newSideBySideReader(reader, min(limit, 8), nil)
			policy.ConcurrentReads = limit
			a, err := Attest(t.Context(), r, m, block, policy)
			if limit < 0 || limit > 16 {
				var issue Issue
				if !errors.As(err, &issue) || issue.Code != "attestation_policy_invalid" || len(r.reads) != 0 {
					t.Fatalf("invalid policy read: %v, %v", r.reads, err)
				}
				return
			}
			if err != nil || !a.Eligible {
				t.Fatalf("attestation %+v: %v", a, err)
			}
			if r.peak > max(1, limit) || r.peak < min(max(1, limit), 8) {
				t.Fatalf("limit %d peak %d", limit, r.peak)
			}
			if limit == 0 {
				serialReads = r.reads
			}
			if limit == 1 && !reflect.DeepEqual(serialReads, r.reads) {
				t.Fatalf("serial order changed: %v, %v", serialReads, r.reads)
			}
		})
	}
}

func TestAttestSideBySideCancellation(t *testing.T) {
	m, reader, block, policy := sideBySideEVM(t)
	for _, limit := range []int{0, 1, 8} {
		for _, mode := range []string{"before", "during", "timeout"} {
			t.Run(strconv.Itoa(limit)+"/"+mode, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				r := newSideBySideReader(reader, max(1, limit), nil)
				policy.ConcurrentReads = limit
				policy.Timeout = time.Second
				want := "attestation_canceled"
				switch mode {
				case "before":
					cancel()
					want = "chain_read_failed"
				case "during":
					r.cancel = cancel
				case "timeout":
					r.hold = true
					policy.Timeout = 20 * time.Millisecond
				}
				_, err := Attest(ctx, r, m, block, policy)
				var issue Issue
				if !errors.As(err, &issue) || issue.Code != want {
					t.Fatalf("cancellation = %v, want %s", err, want)
				}
				if r.afterCancel != 0 || r.active != 0 {
					t.Fatalf("reads after cancellation=%d active=%d", r.afterCancel, r.active)
				}
				if mode == "before" && len(r.reads) != 0 {
					t.Fatalf("canceled context started reads: %v", r.reads)
				}
			})
		}
	}
}

func TestAttestSideBySideReadDependencies(t *testing.T) {
	m, reader, block, policy := sideBySideEVM(t)
	r := newSideBySideReader(reader, 8, nil)
	policy.ConcurrentReads = 8
	if _, err := Attest(t.Context(), r, m, block, policy); err != nil {
		t.Fatal(err)
	}
	position := func(key string) int {
		for i, read := range r.reads {
			if read == key {
				return i
			}
		}
		t.Fatalf("missing read %s", key)
		return -1
	}
	for _, role := range m.Roles {
		if role.Kind == valueErc1967 {
			code := position("code/" + common.HexToAddress(role.Address).Hex())
			storage := position("storage/" + common.HexToAddress(role.Address).Hex())
			implementation := position("code/" + common.HexToAddress(role.ImplementationAddress).Hex())
			if code >= storage || storage >= implementation {
				t.Fatalf("role dependency order %d %d %d", code, storage, implementation)
			}
		}
	}
	for _, read := range r.reads {
		if bytes.Contains([]byte(read), []byte(common.Bytes2Hex(selector("owner()")))) {
			if position("code/"+common.HexToAddress(m.Roles[valuePoolBeacon].ImplementationAddress).Hex()) >= position(read) {
				t.Fatal("owner read before beacon identity")
			}
		}
	}
}
