package deployment

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type boundedAttestationReader struct {
	Reader
	address common.Address
	mode    string
	method  string
}

func (r boundedAttestationReader) ChainID(ctx context.Context) (*big.Int, error) {
	if r.mode == "chain" {
		return nil, context.DeadlineExceeded
	}
	return r.Reader.ChainID(ctx)
}
func (r boundedAttestationReader) HeaderByReportedHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	h, err := headerIdentity(ctx, r.Reader, hash)
	if err != nil {
		return nil, err
	}
	h = types.CopyHeader(h)
	if r.mode == "header time" {
		h.Time = ^uint64(0)
	}
	if r.mode == "header height" {
		h.Number = big.NewInt(0)
	}
	return h, nil
}
func (r boundedAttestationReader) StorageAtHash(ctx context.Context, address common.Address, slot, hash common.Hash) ([]byte, error) {
	if r.mode == "storage" && address == r.address {
		return nil, context.DeadlineExceeded
	}
	return r.Reader.StorageAtHash(ctx, address, slot, hash)
}
func (r boundedAttestationReader) CodeAtHash(ctx context.Context, address common.Address, hash common.Hash) ([]byte, error) {
	if r.mode == "code" && address == r.address {
		return nil, context.DeadlineExceeded
	}
	return r.Reader.CodeAtHash(ctx, address, hash)
}
func (r boundedAttestationReader) CallContractAtHash(ctx context.Context, msg ethereum.CallMsg, hash common.Hash) ([]byte, error) {
	if msg.To != nil && *msg.To == r.address && string(msg.Data) == string(selector(r.method)) {
		return nil, context.DeadlineExceeded
	}
	return r.Reader.CallContractAtHash(ctx, msg, hash)
}

func TestAttestationKeepsUnavailableIdentityAndPolicyIneligible(t *testing.T) {
	manifest, reader, block, policy := newEVM(t, nil, 125)
	for _, mode := range []string{"chain", "header time", "header height", "storage", "code", "implementation", "owner", "pendingOwner", "fee", "decimals", "settlement", "policy"} {
		t.Run(mode, func(t *testing.T) {
			candidate := boundedAttestationReader{Reader: reader, mode: mode}
			role := "PoolBeacon"
			switch mode {
			case "storage":
				role = "Router"
			case "code":
				role = "PoolBeacon"
			case "implementation":
				candidate.method = "implementation()"
			case "owner":
				candidate.method = "owner()"
			case "pendingOwner":
				candidate.method = "pendingOwner()"
			case "fee":
				role = "ProtocolConfig"
				candidate.method = "creationFee()"
			case "decimals":
				role = "USDL"
				candidate.method = "decimals()"
			case "settlement":
				role = "ProtocolConfig"
				candidate.method = "usdlAddress()"
			case "policy":
				role = "ProtocolConfig"
				candidate.method = "minFeeBps()"
			}
			candidate.address = common.HexToAddress(manifest.Roles[role].Address)
			att, err := Attest(context.Background(), candidate, manifest, block, policy)
			switch mode {
			case "chain", "header time", "header height":
				if err == nil {
					t.Fatal("unbound chain identity admitted")
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if att.Eligible || len(att.Issues) == 0 {
					t.Fatalf("unavailable %s admitted: %+v", mode, att)
				}
				if err := att.ValidateFor(manifest, policy); err == nil {
					t.Fatal("ineligible identity validated")
				}
			}
		})
	}
	for _, mode := range []string{"manifest", "policy"} {
		m, p := manifest, policy
		if mode == "manifest" {
			m.ChainID = 1
		} else {
			p.MaxAge = 0
		}
		if _, err := Attest(context.Background(), reader, m, block, p); err == nil {
			t.Fatal("invalid attestation configuration admitted")
		}
	}
	if _, err := headerIdentity(context.Background(), reader, common.Hash{}); err == nil {
		t.Fatal("unbound header admitted")
	}
	if _, err := wordAddress([]byte{1}); err == nil {
		t.Fatal("short address word admitted")
	}
	att, err := Attest(context.Background(), reader, manifest, block, policy)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := NewRevision(manifest, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"revision", "manifest", "attestation"} {
		m, a, r := manifest, att, revision
		switch mode {
		case "revision":
			r.RevisionSHA256 = strings.Repeat("d", 64)
		case "manifest":
			m.ChainID = 1
		case "attestation":
			a.ObservedAt = policy.Now.Add(-2 * time.Hour)
		}
		if _, err := ExportRevision(m, a, policy, PublicEndpoints{}, r, manifest.Addresses(), manifest.Addresses()); err == nil {
			t.Fatal("unbound serial export admitted")
		}
	}
	if err := revision.ValidateFor(Manifest{}, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)); err == nil {
		t.Fatal("unbound revision admitted")
	}
}
