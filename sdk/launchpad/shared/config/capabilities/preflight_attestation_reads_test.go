package capabilities

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

type preflightAttestationReads struct {
	*PinnedRPCReader
	mu           sync.Mutex
	active, peak int
	gate         chan struct{}
	target       int
}

func (r *preflightAttestationReads) begin(ctx context.Context) func() {
	r.mu.Lock()
	r.active++
	r.peak = max(r.peak, r.active)
	if r.gate != nil && r.active >= r.target {
		close(r.gate)
		r.gate = nil
	}
	gate := r.gate
	r.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	return func() {
		r.mu.Lock()
		r.active--
		r.mu.Unlock()
	}
}

func (r *preflightAttestationReads) CodeAtHash(ctx context.Context, address common.Address, block common.Hash) ([]byte, error) {
	end := r.begin(ctx)
	defer end()
	return r.PinnedRPCReader.CodeAtHash(ctx, address, block)
}

func (r *preflightAttestationReads) StorageAtHash(ctx context.Context, address common.Address, slot, block common.Hash) ([]byte, error) {
	end := r.begin(ctx)
	defer end()
	return r.PinnedRPCReader.StorageAtHash(ctx, address, slot, block)
}

func (r *preflightAttestationReads) CallContractAtHash(ctx context.Context, call ethereum.CallMsg, block common.Hash) ([]byte, error) {
	end := r.begin(ctx)
	defer end()
	return r.PinnedRPCReader.CallContractAtHash(ctx, call, block)
}

func preflightAttestationEVM(t *testing.T) (*PinnedRPCReader, deployment.Manifest, common.Hash, time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	ipc := filepath.Join(dir, "rpc.ipc")
	command := exec.Command("anvil", "--host", "127.0.0.1", "--port", "0", "--ipc", ipc, "--chain-id", "125", "--accounts", "0", "--threads", "2", "--silent")
	command.Env = append(os.Environ(), "RAYON_NUM_THREADS=2")
	command.Stdout = io.Discard
	var stderr lockedBuffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	exited := false
	t.Cleanup(func() {
		if !exited {
			_ = command.Process.Kill()
			<-done
		}
	})
	var client *rpc.Client
	var err error
	for {
		client, err = rpc.DialIPC(ctx, ipc)
		if err == nil {
			break
		}
		select {
		case processErr := <-done:
			exited = true
			t.Fatalf("Anvil unavailable: %v: %s", processErr, stderr.String())
		default:
		}
		if ctx.Err() != nil {
			t.Fatalf("Anvil unavailable: %v: %s", ctx.Err(), stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(client.Close)
	reader, err := NewPinnedRPCReader(client)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("../deployment/testdata/evm-artifacts.json")
	if err != nil {
		t.Fatal(err)
	}
	var artifacts struct {
		Contracts map[string]quoteArtifact `json:"contracts"`
	}
	if err := json.Unmarshal(encoded, &artifacts); err != nil {
		t.Fatal(err)
	}
	install := func(name string, address common.Address) string {
		t.Helper()
		artifact, ok := artifacts.Contracts[name]
		if !ok {
			t.Fatalf("artifact missing: %s", name)
		}
		code := common.FromHex(artifact.Runtime)
		for _, references := range artifact.Immutables {
			for _, reference := range references {
				copy(code[reference.Start:reference.Start+reference.Length], common.LeftPadBytes(address.Bytes(), reference.Length))
			}
		}
		if err := client.CallContext(ctx, nil, "anvil_setCode", address, hexutil.Bytes(code)); err != nil {
			t.Fatal(err)
		}
		return crypto.Keccak256Hash(code).Hex()
	}
	set := func(address common.Address, slot int64, value common.Hash) {
		t.Helper()
		if err := client.CallContext(ctx, nil, "anvil_setStorageAt", address, common.BigToHash(big.NewInt(slot)), value); err != nil {
			t.Fatal(err)
		}
	}
	m := historicalPreflightManifest(t)
	m.Status, m.ActivationHeight = "eligible", 1
	for name, role := range m.Roles {
		address := common.HexToAddress(role.Address)
		switch role.Kind {
		case "erc1967":
			role.RuntimeHash = install("UUPSProxy", address)
			role.ImplementationHash = install(name, common.HexToAddress(role.ImplementationAddress))
			if err := client.CallContext(ctx, nil, "anvil_setStorageAt", address, deployment.ImplementationSlot, common.BytesToHash(common.HexToAddress(role.ImplementationAddress).Bytes())); err != nil {
				t.Fatal(err)
			}
		case "beacon":
			role.RuntimeHash = install("PoolBeacon", address)
			role.ImplementationHash = install("SidioraPool", common.HexToAddress(role.ImplementationAddress))
			set(address, 0, common.BytesToHash(common.HexToAddress(role.ImplementationAddress).Bytes()))
			set(address, 1, common.BytesToHash(common.HexToAddress(m.Governance.ExpectedBeaconOwner).Bytes()))
		case "direct":
			role.RuntimeHash = install("MockERC20", address)
			set(address, 2, common.BigToHash(big.NewInt(6)))
		default:
			t.Fatalf("unexpected role kind: %s", role.Kind)
		}
		m.Roles[name] = role
	}
	cfg := common.HexToAddress(m.Roles["ProtocolConfig"].Address)
	set(cfg, 1, common.BytesToHash(common.HexToAddress(m.Roles["USDL"].Address).Bytes()))
	set(cfg, 11, common.BigToHash(big.NewInt(1000000)))
	if err := client.CallContext(ctx, nil, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	header, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	return reader, m, header.Hash(), time.Unix(int64(header.Time), 0).UTC().Add(time.Second)
}

func TestPreflightAttestationReads(t *testing.T) {
	reader, m, block, now := preflightAttestationEVM(t)
	var serial deployment.ActivationAttestation
	for _, limit := range []int{0, 8} {
		name := "default"
		if limit == 8 {
			name = "eight"
		}
		t.Run(name, func(t *testing.T) {
			observed := &preflightAttestationReads{PinnedRPCReader: reader, gate: make(chan struct{}), target: max(1, limit)}
			engine, err := NewPreflightEngine(observed, m, m.Roles["PoolBeacon"].ImplementationHash, strings.Repeat("a", 64), time.Minute, 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if limit != 0 {
				if err := engine.SetAttestationReads(limit); err != nil {
					t.Fatal(err)
				}
			}
			for _, invalid := range []int{-1, 17} {
				var issue deployment.Issue
				if err := engine.SetAttestationReads(invalid); !errors.As(err, &issue) || issue.Code != "attestation_policy_invalid" {
					t.Fatalf("invalid limit accepted: %v", err)
				}
			}
			got, err := engine.Attestation(t.Context(), block, now)
			if err != nil || !got.Eligible {
				t.Fatalf("attestation %+v: %v", got, err)
			}
			observed.mu.Lock()
			peak, active := observed.peak, observed.active
			observed.mu.Unlock()
			if peak != max(1, limit) || active != 0 {
				t.Fatalf("limit %d peak %d active %d", limit, peak, active)
			}
			if limit == 0 {
				serial = got
			} else if !reflect.DeepEqual(serial, got) {
				t.Fatalf("engine reports differ: %+v, %+v", serial, got)
			}
		})
	}
	var missing *PreflightEngine
	if err := missing.SetAttestationReads(8); err == nil {
		t.Fatal("missing engine accepted")
	}
	if err := (&PreflightEngine{}).SetAttestationReads(8); err == nil {
		t.Fatal("uninitialized engine accepted")
	}
}
