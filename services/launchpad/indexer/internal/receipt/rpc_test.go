package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

func realReceiptNode(t *testing.T, extraArgs ...string) (context.Context, *ethclient.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	ipc := filepath.Join(t.TempDir(), "node.ipc")
	args := append([]string{"--host", "127.0.0.1", "--port", "0", "--ipc", ipc, "--threads", "2", "--chain-id", "125", "--accounts", "1", "--silent"}, extraArgs...)
	command := exec.CommandContext(ctx, "anvil", args...)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	})
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		connection, err := rpc.DialIPC(ctx, ipc)
		if err == nil {
			client := ethclient.NewClient(connection)
			t.Cleanup(client.Close)
			return ctx, client
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}
func TestRPCAuthenticReceiptMembership(t *testing.T) {
	ctx, client := realReceiptNode(t)
	key, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, sender)
	if err != nil {
		t.Fatal(err)
	}
	price, err := client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := types.SignTx(types.NewTransaction(nonce, common.HexToAddress("0x0000000000000000000000000000000000000125"), big.NewInt(12345), 21000, price, nil), types.LatestSignerForChainID(big.NewInt(125)), key)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.SendTransaction(ctx, tx); err != nil {
		t.Fatal(err)
	}
	var mined *types.Receipt
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for mined == nil {
		mined, err = client.TransactionReceipt(ctx, tx.Hash())
		if err != nil && !errors.Is(err, ethereum.NotFound) {
			t.Fatal(err)
		}
		if mined == nil {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-ticker.C:
			}
		}
	}
	authority := &RPCQualification{client: client, options: RPCOptions{Timeout: 15 * time.Second}}
	block, receipts, err := authority.pinnedBlock(ctx, mined.BlockHash)
	if err != nil {
		t.Fatal(err)
	}
	if block.Hash != mined.BlockHash || len(receipts) != 1 || !receiptEqual(receipts[0], mined) {
		t.Fatal("authentic node receipt lost block membership")
	}
	bundle, err := authority.Bundle(ctx, tx.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if err = authority.ValidateReceipt(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	changed, err := clone(bundle)
	if err != nil {
		t.Fatal(err)
	}
	changed.Receipt.GasUsed++
	if err = authority.ValidateReceipt(ctx, changed); err == nil {
		t.Fatal("altered receipt accepted")
	}
	changed, err = clone(bundle)
	if err != nil {
		t.Fatal(err)
	}
	changed.Header.Extra = append(changed.Header.Extra, 1)
	if err = authority.ValidateReceipt(ctx, changed); err == nil {
		t.Fatal("altered header accepted")
	}
	changed, err = clone(bundle)
	if err != nil {
		t.Fatal(err)
	}
	changed.Receipt.TransactionIndex++
	if err = authority.ValidateReceipt(ctx, changed); err == nil {
		t.Fatal("altered transaction position accepted")
	}
}

func receiptManifestOptions(t *testing.T, manifest deployment.Manifest, anchor BlockAnchor) RPCOptions {
	t.Helper()
	bundles, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	reviewed := decode.Catalog{SchemaVersion: 1, ChainID: 125, ManifestSHA256: manifest.SourceManifestSHA256}
	revisions := map[string]string{}
	for _, bundle := range bundles {
		reviewed.Contracts = append(reviewed.Contracts, decode.CatalogContract{
			Name: bundle.Name, ABIRevision: bundle.ABIHash, SourceDigest: bundle.SourceHash,
			CompilerDigest: bundle.BuildHash, CompilerVersion: bundle.Compiler, ABI: bundle.ABI,
		})
		revisions[bundle.Name] = bundle.ABIHash
	}
	raw, err := json.Marshal(reviewed)
	if err != nil {
		t.Fatal(err)
	}
	o := RPCOptions{Manifest: manifest, CatalogJSON: raw, Anchor: &anchor}
	for _, role := range []string{"EventEmitter", "Router", "SidioraFactory", "PoolRegistry", "ProtocolConfig", "USDL"} {
		identity := manifest.Roles[role]
		name := role
		if role == "USDL" {
			name = "SidioraERC20"
		}
		if revisions[name] == "" {
			t.Fatalf("reviewed ABI absent for %s", name)
		}
		o.Epochs = append(o.Epochs, RPCCodeEpoch{
			Deployment: decode.Deployment{Address: common.HexToAddress(identity.Address), Role: name,
				ABIRevision: revisions[name], From: decode.Order{Block: manifest.ActivationHeight}},
			Identity: identity,
		})
	}
	return o
}

func TestReceiptQualificationAcceptsHistoricalManifest(t *testing.T) {
	historical, ok := deployment.Approved(32226215)
	if !ok || historical.DeploymentVersion != "paxeer125-a35ff7c6f532d2f9" {
		t.Fatal("reviewed historical manifest unavailable")
	}
	current := deployment.Current()
	approvedCurrent, ok := deployment.Approved(current.ActivationHeight)
	if !ok || !reflect.DeepEqual(approvedCurrent, current) {
		t.Fatal("current manifest absent from approved history")
	}
	next, ok := deployment.NextActivationHeight(historical.ActivationHeight)
	if !ok || next != current.ActivationHeight {
		t.Fatal("historical generation has no expected successor")
	}
	for _, height := range []uint64{historical.ActivationHeight, historical.ActivationHeight + 1, next - 1, current.ActivationHeight, current.ActivationHeight + 1} {
		t.Run(strconv.FormatUint(height, 10), func(t *testing.T) {
			manifest := historical
			if height >= current.ActivationHeight {
				manifest = current
			}
			ctx, client := realReceiptNode(t, "--number", strconv.FormatUint(height, 10))
			header, err := client.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
			if err != nil {
				t.Fatal(err)
			}
			o := receiptManifestOptions(t, manifest, BlockAnchor{Height: height, Hash: header.Hash()})
			authority, err := NewRPCQualificationWithAttestation(ctx, client, o, nil)
			if err != nil {
				t.Fatalf("approved manifest at height %d rejected: %v", height, err)
			}
			wantHash, err := manifest.Hash()
			if err != nil {
				t.Fatal(err)
			}
			if authority == nil || authority.IsPrivateFixture() || authority.manifestHash != wantHash || !reflect.DeepEqual(authority.options.Manifest, manifest) || *authority.options.Anchor != *o.Anchor {
				t.Fatal("qualification did not retain the approved manifest and anchor")
			}
		})
	}
	role := historical.Roles["Router"]
	role.Address = "0x0000000000000000000000000000000000000125"
	historical.Roles["Router"] = role
	fresh, ok := deployment.Approved(historical.ActivationHeight)
	if !ok || fresh.Roles["Router"] == role {
		t.Fatal("caller mutation changed approved history")
	}
}

func TestReceiptQualificationRefusesManifestOutsideHistory(t *testing.T) {
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("reviewed historical manifest unavailable")
	}
	ctx, client := realReceiptNode(t, "--number", strconv.FormatUint(manifest.ActivationHeight, 10))
	header, err := client.HeaderByNumber(ctx, new(big.Int).SetUint64(manifest.ActivationHeight))
	if err != nil {
		t.Fatal(err)
	}
	baseline := receiptManifestOptions(t, manifest, BlockAnchor{Height: manifest.ActivationHeight, Hash: header.Hash()})
	if _, err := NewRPCQualificationWithAttestation(ctx, client, baseline, nil); err != nil {
		t.Fatalf("unmodified historical manifest rejected: %v", err)
	}
	next, ok := deployment.NextActivationHeight(manifest.ActivationHeight)
	if !ok {
		t.Fatal("historical generation has no successor")
	}
	cases := map[string]func(*RPCOptions){
		"changed role address": func(o *RPCOptions) {
			role := o.Manifest.Roles["Router"]
			role.Address = "0x0000000000000000000000000000000000000125"
			o.Manifest.Roles["Router"] = role
		},
		"unknown activation height":  func(o *RPCOptions) { o.Manifest.ActivationHeight++ },
		"changed deployment version": func(o *RPCOptions) { o.Manifest.DeploymentVersion += "-altered" },
		"changed source digest":      func(o *RPCOptions) { o.Manifest.SourceManifestSHA256 = "b" + o.Manifest.SourceManifestSHA256[1:] },
		"changed status":             func(o *RPCOptions) { o.Manifest.Status = "eligible" },
		"changed governance owner": func(o *RPCOptions) {
			o.Manifest.Governance.ExpectedBeaconOwner = "0x0000000000000000000000000000000000000125"
		},
		"anchor before activation":     func(o *RPCOptions) { o.Anchor.Height = manifest.ActivationHeight - 1 },
		"anchor at next activation":    func(o *RPCOptions) { o.Anchor.Height = next },
		"anchor after next activation": func(o *RPCOptions) { o.Anchor.Height = next + 1 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			o, err := clone(baseline)
			if err != nil {
				t.Fatal(err)
			}
			change(&o)
			if err := o.Manifest.Validate(); err != nil {
				t.Fatalf("candidate must be a valid manifest before history admission: %v", err)
			}
			if name == "unknown activation height" {
				if _, ok := deployment.Approved(o.Manifest.ActivationHeight); ok {
					t.Fatal("unknown height unexpectedly approved")
				}
			}
			o = receiptManifestOptions(t, o.Manifest, *o.Anchor)
			ctx, client := realReceiptNode(t, "--number", strconv.FormatUint(o.Anchor.Height, 10))
			header, err := client.HeaderByNumber(ctx, new(big.Int).SetUint64(o.Anchor.Height))
			if err != nil {
				t.Fatal(err)
			}
			o.Anchor.Hash = header.Hash()
			authority, err := NewRPCQualificationWithAttestation(ctx, client, o, nil)
			if authority != nil || !errors.Is(err, ErrQualification) {
				t.Fatalf("outside-history qualification: authority=%v error=%v", authority, err)
			}
		})
	}
}
