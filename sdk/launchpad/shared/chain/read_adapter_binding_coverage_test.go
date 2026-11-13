package chain

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func TestReadAdapterRequiresAttestedRuntime(t *testing.T) {
	manifest := deployment.Current()
	height := max(uint64(100), manifest.ActivationHeight)
	header := testHeader(int64(height))
	var calls atomic.Int64
	server := pinnedServer(t, header, false, &calls)
	manifestHash, err := manifest.Hash()
	if err != nil {
		t.Fatal(err)
	}
	config := ReadAdapterConfig{
		Manifest: manifest, ManifestHash: manifestHash,
		CapabilityHash: capabilities.Frozen().SourceSHA256,
		InterfaceHash:  strings.Repeat("a", 64), EndpointSetVersion: "fixture",
		AnchorHeight: height, AnchorHash: consensusHash(header),
		ConfigurationRevision: "fixture", AttestationMaxAge: time.Minute,
		HistoricalConcurrency: 1, PoolProxyRuntimeHash: common.HexToHash("0x01"),
	}
	adapter, err := NewReadAdapter(context.Background(), server.URL, "", config)
	if err == nil {
		adapter.Close()
		t.Fatal("unattested deployment served as a read adapter")
	}
	client, err := NewVerifiedClient(context.Background(), server.URL, "", VerifiedConfig{profile.Paxeer("fixture", manifest.DeploymentVersion), height, consensusHash(header)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	pin, err := client.Pin(context.Background(), height)
	if err != nil {
		t.Fatal(err)
	}
	bound := profile.ManifestBinding{Profile: profile.Paxeer("fixture", manifest.DeploymentVersion), ManifestHash: manifestHash, CapabilityHash: config.CapabilityHash, InterfaceHash: config.InterfaceHash}
	reader := &ReadAdapter{client: client, manifest: manifest, binding: bound, configurationRevision: "fixture", maxAge: time.Minute, poolProxyRuntimeHash: config.PoolProxyRuntimeHash}
	if _, err := reader.Snapshot(context.Background(), pin, common.Address{}); err == nil || !strings.Contains(err.Error(), "pool identity") {
		t.Fatalf("empty pool identity: %v", err)
	}
	unknown := pin
	unknown.Hash = common.HexToHash("0x02")
	pool := common.HexToAddress("0x1111111111111111111111111111111111111111")
	if _, err := reader.Snapshot(context.Background(), unknown, pool); err == nil || !strings.Contains(err.Error(), "unrecognized") {
		t.Fatalf("unissued snapshot pin: %v", err)
	}
	if _, err := reader.Snapshot(context.Background(), pin, pool); err == nil {
		t.Fatal("unattested pool snapshot served")
	}
}

func TestReadAdapterConfigurationAndEndpointAdmission(t *testing.T) {
	adapter, backend := snapshotRuntimeFixture(t, "surface")
	binding := adapter.binding
	hash, err := adapter.manifest.Hash()
	if err != nil {
		t.Fatal(err)
	}
	config := ReadAdapterConfig{Manifest: adapter.manifest, ManifestHash: hash, CapabilityHash: binding.CapabilityHash, InterfaceHash: binding.InterfaceHash, EndpointSetVersion: binding.Profile.EndpointSetVersion, AnchorHeight: 100, AnchorHash: consensusHash(backend.header), ConfigurationRevision: adapter.configurationRevision, AttestationMaxAge: time.Minute, HistoricalConcurrency: 1, PoolProxyRuntimeHash: adapter.poolProxyRuntimeHash}
	for _, mode := range []string{"configuration", "manifest binding", "endpoint", "anchor"} {
		candidate := config
		endpoint := "http://127.0.0.1:1"
		switch mode {
		case "configuration":
			candidate.HistoricalConcurrency = 0
		case "manifest binding":
			candidate.ManifestHash = strings.Repeat("0", 64)
		case "endpoint":
			endpoint = "file:///unavailable"
		case "anchor":
			candidate.AnchorHash = common.Hash{}
		}
		if got, err := NewReadAdapter(context.Background(), endpoint, "", candidate); err == nil || got != nil {
			t.Fatalf("invalid %s configuration admitted", mode)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	adapter.headPermit <- struct{}{}
	if _, err := adapter.ObserveHead(cancelled); err == nil {
		t.Fatal("cancelled saturated head read admitted")
	}
	<-adapter.headPermit
	if _, _, err := adapter.Transaction(context.Background(), common.Hash{}); err == nil {
		t.Fatal("unbound transaction admitted")
	}
	if _, _, err := adapter.client.ReceiptState(context.Background(), readpolicy.TransactionState{}); err == nil {
		t.Fatal("unbound transaction state admitted")
	}
	if _, err := TransactionStatus(context.Background(), nil, common.Hash{}); err == nil {
		t.Fatal("unconfigured transaction status admitted")
	}
}
