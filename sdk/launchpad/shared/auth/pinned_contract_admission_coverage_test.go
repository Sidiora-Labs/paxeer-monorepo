package auth

import (
	"context"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type sequencedContractReader struct {
	ContractBackend
	reported func(context.Context, *big.Int) (ReportedBlock, error)
	chain    func(context.Context) (*big.Int, error)
	code     func(context.Context, common.Address, *big.Int) ([]byte, error)
}

func (r sequencedContractReader) ReportedBlockByNumber(ctx context.Context, n *big.Int) (ReportedBlock, error) {
	if r.reported != nil {
		return r.reported(ctx, n)
	}
	return r.ContractBackend.ReportedBlockByNumber(ctx, n)
}
func (r sequencedContractReader) ChainID(ctx context.Context) (*big.Int, error) {
	if r.chain != nil {
		return r.chain(ctx)
	}
	return r.ContractBackend.ChainID(ctx)
}
func (r sequencedContractReader) CodeAt(ctx context.Context, a common.Address, n *big.Int) ([]byte, error) {
	if r.code != nil {
		return r.code(ctx, a, n)
	}
	return r.ContractBackend.CodeAt(ctx, a, n)
}

func TestContractReadsRequireStableReportedIdentityAroundState(t *testing.T) {
	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, os.Getenv("L34_ANVIL_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	backend := NewReportedHashBackend(client.Client())
	pin, err := backend.ReportedBlockByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	denied := errors.New("chain observation unavailable")
	for _, mode := range []string{"pin", "before read", "before identity", "state", "cancellation", "after read", "after identity"} {
		t.Run(mode, func(t *testing.T) {
			candidate := pin
			calls, reads := 0, 0
			work, cancel := context.WithCancel(ctx)
			defer cancel()
			reader := sequencedContractReader{ContractBackend: backend, reported: func(ctx context.Context, n *big.Int) (ReportedBlock, error) {
				calls++
				if mode == "before read" && calls == 1 || mode == "after read" && calls == 2 {
					return ReportedBlock{}, denied
				}
				result, err := backend.ReportedBlockByNumber(ctx, n)
				if mode == "before identity" && calls == 1 || mode == "after identity" && calls == 2 {
					result.Hash = common.HexToHash("0x01")
				}
				return result, err
			}}
			if mode == "pin" {
				candidate = ReportedBlock{}
			}
			err := readPinned(work, reader, candidate, func(height *big.Int) error {
				reads++
				if height.Cmp(pin.Number()) != 0 {
					t.Fatal("state read changed pinned height")
				}
				if mode == "state" {
					return denied
				}
				if mode == "cancellation" {
					cancel()
				}
				return nil
			})
			if err == nil {
				t.Fatal("unstable state read admitted")
			}
			if strings.HasSuffix(mode, "identity") && !errors.Is(err, errPinnedBlockChanged) {
				t.Fatalf("changed block identity: %v", err)
			}
			if mode == "cancellation" && !errors.Is(err, context.Canceled) {
				t.Fatalf("state read cancellation: %v", err)
			}
			if mode == "before read" || mode == "before identity" || mode == "pin" {
				if reads != 0 {
					t.Fatal("state accessed before identity was established")
				}
			} else if reads != 1 {
				t.Fatalf("state reads: %d", reads)
			}
		})
	}
	if err := readPinned(ctx, backend, pin, func(height *big.Int) error {
		_, err := backend.CodeAt(ctx, common.HexToAddress(goldenAddr), height)
		return err
	}); err != nil {
		t.Fatalf("stable state read: %v", err)
	}
	for _, mode := range []string{"chain unavailable", "foreign chain", "latest unavailable", "latest identity", "code unavailable"} {
		t.Run(mode, func(t *testing.T) {
			reader := sequencedContractReader{ContractBackend: backend}
			switch mode {
			case "chain unavailable":
				reader.chain = func(context.Context) (*big.Int, error) { return nil, denied }
			case "foreign chain":
				reader.chain = func(context.Context) (*big.Int, error) { return big.NewInt(1), nil }
			case "latest unavailable":
				reader.reported = func(context.Context, *big.Int) (ReportedBlock, error) { return ReportedBlock{}, denied }
			case "latest identity":
				reader.reported = func(context.Context, *big.Int) (ReportedBlock, error) { return ReportedBlock{}, nil }
			case "code unavailable":
				reader.code = func(context.Context, common.Address, *big.Int) ([]byte, error) { return nil, denied }
			}
			for _, kind := range []string{valueEoaEip191, valueErc1271} {
				seen := ""
				result, err := VerifyProof(ctx, reader, WalletProof{Kind: kind, Wallet: goldenAddr, Signature: goldenSig}, goldenMsg, func(method, category string, duration time.Duration) {
					if duration <= 0 {
						t.Error("missing proof timing")
					}
					seen = category
				})
				var failure *VerificationError
				want := valueUpstream
				if mode == "foreign chain" {
					want = valueChain
				}
				if !errors.As(err, &failure) || failure.Category != want || seen != want || result.Wallet != "" {
					t.Fatalf("unverified %s proof: %+v %v observation=%s", kind, result, err, seen)
				}
			}
			if err := (WalletOperationVerifier{Backend: reader}).Verify(ctx, strings.ToLower(goldenAddr), goldenMsg, goldenSig); err == nil {
				t.Fatal("unverified operation authority admitted")
			}
		})
	}
	for _, kind := range []string{valueEoaEip191, valueErc1271} {
		if _, err := VerifyProof(ctx, nil, WalletProof{Kind: kind, Wallet: goldenAddr, Signature: goldenSig}, goldenMsg, nil); err == nil {
			t.Fatal("proof without chain authority admitted")
		}
		if _, err := VerifyProof(ctx, backend, WalletProof{Kind: kind, Wallet: "invalid", Signature: goldenSig}, goldenMsg, nil); err == nil {
			t.Fatal("invalid proof wallet admitted")
		}
	}
	for _, signature := range []string{"invalid", strings.Repeat("0", 8195)} {
		if _, err := VerifyContract(ctx, backend, goldenAddr, goldenMsg, signature, nil); err == nil {
			t.Fatal("unbounded contract signature admitted")
		}
	}
	if _, err := backend.CodeAt(ctx, common.Address{}, nil); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("unbound code read: %v", err)
	}
}
