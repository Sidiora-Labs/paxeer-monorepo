package metadata

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
)

func signEIP191(t *testing.T, message string) (wallet, signature string) {
	t.Helper()
	key, e := crypto.GenerateKey()
	if e != nil {
		t.Fatal(e)
	}
	sig, e := crypto.Sign(accounts.TextHash([]byte(message)), key)
	if e != nil {
		t.Fatal(e)
	}
	sig[64] += 27
	return strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex()), "0x" + hex.EncodeToString(sig)
}

// Against the live Paxeer RPC: the committed head is stored with the
// node-reported (CometBFT) hash, which is not go-ethereum's header hash. The
// verifier pins the proof to exactly that block by its reported hash and
// accepts genuine signatures only; the recomputed header hash is refused.
func TestProductionReportedCommittedProofVerifierOnPaxeer(t *testing.T) {
	for _, name := range []string{"NETWORK_RPC_URL"} {
		if os.Getenv(name) == "" {
			t.Skipf("protected owner fixture unavailable: %s is not set", name)
		}
	}

	endpoint := strings.TrimSpace(os.Getenv("NETWORK_RPC_URL"))
	if endpoint == "" {
		t.Fatal("NETWORK_RPC_URL (Paxeer mainnet RPC) is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client, e := gethrpc.DialContext(ctx, endpoint)
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	chain := ethclient.NewClient(client)
	head, e := chain.BlockNumber(ctx)
	if e != nil {
		t.Fatal(e)
	}
	height := new(big.Int).SetUint64(head - 3)
	block, e := sharedauth.NewReportedHashBackend(client).ReportedBlockByNumber(ctx, height)
	if e != nil {
		t.Fatal(e)
	}
	header, reported := block.Header, block.Hash
	if header.Hash() == reported {
		t.Log("node reported the Keccak header hash at", height)
	}
	admin := edgePool(t, "")
	schema := edgeSchema(t, admin, "edge_committed")
	if _, e = admin.Exec(ctx, "CREATE TABLE "+pgx.Identifier{schema, "auth_committed_blocks"}.Sanitize()+"(chain_id integer NOT NULL CHECK(chain_id=125),height numeric(20,0) NOT NULL CHECK(height>=0),block_hash text NOT NULL CHECK(block_hash ~ '^0x[0-9a-f]{64}$'),integrity text NOT NULL CHECK(integrity IN ('healthy','quarantined')),PRIMARY KEY(chain_id,height))"); e != nil {
		t.Fatal(e)
	}
	table := pgx.Identifier{schema, "auth_committed_blocks"}.Sanitize()
	if _, e = admin.Exec(ctx, "INSERT INTO "+table+" VALUES(125,$1,$2,'healthy')", height.String(), strings.ToLower(reported.Hex())); e != nil {
		t.Fatal(e)
	}
	pinned := edgePool(t, schema+",pg_catalog")
	blocks := sharedauth.PostgresCommittedBlockProvider{Pool: pinned}
	verifier := ReportedCommittedProofVerifier{RPC: client, Blocks: blocks}
	if e = verifier.CheckCommittedHead(ctx); e != nil {
		t.Fatal("committed head at the reported hash refused:", e)
	}
	message := "KindleLaunch metadata publication\nverifier qualification " + height.String()
	wallet, signature := signEIP191(t, message)
	if e = verifier.Verify(ctx, wallet, message, signature); e != nil {
		t.Fatal("genuine signature refused:", e)
	}
	other, _ := signEIP191(t, message)
	if e = verifier.Verify(ctx, other, message, signature); e == nil {
		t.Fatal("signature accepted for another wallet")
	}
	if e = verifier.Verify(ctx, wallet, message+"!", signature); e == nil {
		t.Fatal("signature accepted for another message")
	}
	// A committed hash the node does not report at that height (here the
	// locally recomputed header hash) is an upstream failure, never a pass.
	if _, e = admin.Exec(ctx, "UPDATE "+table+" SET block_hash=$1", strings.ToLower(header.Hash().Hex())); e != nil {
		t.Fatal(e)
	}
	if header.Hash() != reported {
		var failure *sharedauth.VerificationError
		if e = verifier.Verify(ctx, wallet, message, signature); !errors.As(e, &failure) || !failure.Retryable {
			t.Fatal("mismatched committed hash accepted:", e)
		}
		if verifier.CheckCommittedHead(ctx) == nil {
			t.Fatal("mismatched committed head reported ready")
		}
	}
	if _, e = admin.Exec(ctx, "UPDATE "+table+" SET block_hash=$1,integrity='quarantined'", strings.ToLower(reported.Hex())); e != nil {
		t.Fatal(e)
	}
	if verifier.Verify(ctx, wallet, message, signature) == nil || verifier.CheckCommittedHead(ctx) == nil {
		t.Fatal("quarantined committed head accepted")
	}
	if (ReportedCommittedProofVerifier{}).Verify(ctx, wallet, message, signature) == nil {
		t.Fatal("unconfigured verifier accepted")
	}
}

func TestReportedCommittedHeadTracksStoredChainHash(t *testing.T) {
	ctx := context.Background()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	chain := simulated.NewBackend(types.GenesisAlloc{}, func(n *node.Config, e *ethconfig.Config) {
		n.HTTPHost = "127.0.0.1"
		n.HTTPPort = port
		n.HTTPModules = []string{"eth"}
		e.Genesis.Config = &chainConfig
		e.NetworkId = 125
	})
	defer chain.Close()
	client, err := gethrpc.DialContext(ctx, "http://127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	block, err := sharedauth.NewReportedHashBackend(client).ReportedBlockByNumber(ctx, big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	admin, dsn := internaltest.NewPostgresURL(t)
	const schema = "committed_head"
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema+"; CREATE TABLE "+schema+".auth_committed_blocks(chain_id integer NOT NULL,height bigint NOT NULL,block_hash text NOT NULL,integrity text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",pg_catalog"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := admin.Exec(ctx, "INSERT INTO "+schema+".auth_committed_blocks VALUES(125,0,$1,'healthy')", strings.ToLower(block.Hash.Hex())); err != nil {
		t.Fatal(err)
	}
	verifier := ReportedCommittedProofVerifier{RPC: client, Blocks: sharedauth.PostgresCommittedBlockProvider{Pool: pool}}
	if err := verifier.CheckCommittedHead(ctx); err != nil {
		t.Fatalf("stored committed hash = %v", err)
	}
	if err := (ReportedCommittedProofVerifier{}).CheckCommittedHead(ctx); err != raster.ErrUnavailable {
		t.Fatalf("unconfigured committed head = %v", err)
	}
	if _, err := admin.Exec(ctx, "UPDATE "+schema+".auth_committed_blocks SET block_hash=$1", "0x"+strings.Repeat("1", 64)); err != nil {
		t.Fatal(err)
	}
	var verification *sharedauth.VerificationError
	if err := verifier.CheckCommittedHead(ctx); !errors.As(err, &verification) || !verification.Retryable {
		t.Fatalf("changed committed hash = %v", err)
	}
	if _, err := admin.Exec(ctx, "UPDATE "+schema+".auth_committed_blocks SET integrity='quarantined'"); err != nil {
		t.Fatal(err)
	}
	if err := verifier.CheckCommittedHead(ctx); !errors.As(err, &verification) || !verification.Retryable {
		t.Fatalf("quarantined committed head = %v", err)
	}
}
