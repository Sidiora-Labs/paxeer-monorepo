package tasks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	seiconfig "github.com/sei-protocol/sei-config"
	"github.com/sei-protocol/seictl/sidecar/engine"
	seis3 "github.com/sei-protocol/seictl/sidecar/s3"
	"github.com/sei-protocol/seilog"
)

var genesisLog = seilog.NewLogger("seictl", "task", "genesis")

const genesisMarkerFile = ".sei-sidecar-genesis-done"

// S3GetObjectAPI abstracts a single-object S3 download for small files.
type S3GetObjectAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// S3ClientFactory builds an S3GetObjectAPI for a given region.
type S3ClientFactory func(ctx context.Context, region string) (S3GetObjectAPI, error)

// DefaultS3ClientFactory creates a real S3 client using default credentials.
func DefaultS3ClientFactory(ctx context.Context, region string) (S3GetObjectAPI, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return s3.NewFromConfig(cfg), nil
}

// GenesisS3Config holds S3 coordinates for genesis.json download.
type GenesisS3Config struct {
	Bucket              string
	Key                 string
	Region              string
	ExpectedGenesisHash string
}

// ConfigureGenesisRequest holds the typed parameters for the configure-genesis task.
// The fetcher resolves genesis from the chain ID using embedded config or S3 fallback.
//
// ExpectedGenesisHash is the bare SHA-256 hex digest (no "sha256:" prefix) the
// downloaded genesis.json must match. Every S3 download requires this digest.
type ConfigureGenesisRequest struct {
	ExpectedGenesisHash string `json:"expectedGenesisHash,omitempty"`
}

// GenesisFetcher writes genesis.json to the config directory. It first checks
// for an embedded genesis in sei-config for the chain ID. If not found, it
// falls back to downloading from S3 at {bucket}/{chainID}/genesis.json.
type GenesisFetcher struct {
	homeDir         string
	chainID         string
	genesisBucket   string
	genesisRegion   string
	s3ClientFactory S3ClientFactory
}

// NewGenesisFetcher creates a fetcher targeting the given home directory.
// chainID is the chain this sidecar is running for (typically from SEI_CHAIN_ID).
// genesisBucket and genesisRegion configure the S3 fallback location when the
// chain is not embedded in sei-config.
func NewGenesisFetcher(homeDir, chainID, genesisBucket, genesisRegion string, factory S3ClientFactory) *GenesisFetcher {
	if factory == nil {
		factory = DefaultS3ClientFactory
	}
	return &GenesisFetcher{
		homeDir:         homeDir,
		chainID:         chainID,
		genesisBucket:   genesisBucket,
		genesisRegion:   genesisRegion,
		s3ClientFactory: factory,
	}
}

// Handler returns an engine.TaskHandler that resolves genesis from embedded
// config or S3 fallback. No task parameters are required.
func (g *GenesisFetcher) Handler() engine.TaskHandler {
	return engine.TypedHandler(func(ctx context.Context, req ConfigureGenesisRequest) error {

		// Try embedded genesis first.
		if _, err := seiconfig.GenesisForChain(g.chainID); err == nil {
			return g.writeEmbeddedGenesis()
		}

		// Fall back to S3.
		if g.genesisBucket == "" || g.genesisRegion == "" {
			return fmt.Errorf("configure-genesis: chain %q is not embedded and SEI_GENESIS_BUCKET/SEI_GENESIS_REGION are not set", g.chainID)
		}

		key := g.chainID + "/genesis.json"
		genesisLog.Info("chain not embedded, fetching from S3", "chainId", g.chainID, "bucket", g.genesisBucket, "key", key)
		return g.fetchFromS3(ctx, GenesisS3Config{Bucket: g.genesisBucket, Key: key, Region: g.genesisRegion}, req.ExpectedGenesisHash)
	})
}

// Fetch downloads genesis.json from S3, skipping if the marker file exists.
// Direct callers must provide the authenticated ExpectedGenesisHash.
func (g *GenesisFetcher) Fetch(ctx context.Context, cfg GenesisS3Config) error {
	return g.fetchFromS3(ctx, cfg, cfg.ExpectedGenesisHash)
}

const maxGenesisBytes = 128 << 20

type genesisCompletion struct {
	ChainID string `json:"chainId"`
	Digest  string `json:"digest"`
}

func canonicalGenesisHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	b, err := hex.DecodeString(hash)
	return err == nil && hex.EncodeToString(b) == hash
}

func verifyGenesisBytes(data []byte, chainID, hash string) error {
	if !canonicalGenesisHash(hash) {
		return fmt.Errorf("canonical SHA-256 genesis digest required")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != hash {
		return &engine.TaskError{Task: "configure-genesis", Operation: "verify-hash", Message: "genesis SHA-256 does not match authenticated digest", Retryable: false}
	}
	var doc struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	if chainID == "" || doc.ChainID != chainID {
		return fmt.Errorf("genesis chain ID %q differs from configured %q", doc.ChainID, chainID)
	}
	return nil
}

func (g *GenesisFetcher) fetchFromS3(ctx context.Context, cfg GenesisS3Config, expectedHash string) error {
	if !canonicalGenesisHash(expectedHash) {
		return Terminal(fmt.Errorf("configure-genesis: canonical authenticated SHA-256 digest required"))
	}
	if g.chainID == "" {
		return Terminal(fmt.Errorf("configure-genesis: chain ID required"))
	}
	home, err := openConfinedDir(g.homeDir, false)
	if err != nil {
		return err
	}
	defer home.Close()
	marker, err := confinedRead(home, genesisMarkerFile, 4096)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		var completed genesisCompletion
		if err := json.Unmarshal(marker, &completed); err != nil {
			return fmt.Errorf("configure-genesis: unbound legacy marker: %w", err)
		}
		if completed.ChainID != g.chainID || completed.Digest != expectedHash {
			return Terminal(fmt.Errorf("configure-genesis: completion belongs to a different chain or digest"))
		}
		dir, err := openConfinedDir(filepath.Join(g.homeDir, "config"), false)
		if err != nil {
			return err
		}
		defer dir.Close()
		data, err := confinedRead(dir, "genesis.json", maxGenesisBytes)
		if err != nil {
			return err
		}
		return verifyGenesisBytes(data, g.chainID, expectedHash)
	}
	client, err := g.s3ClientFactory(ctx, cfg.Region)
	if err != nil {
		return fmt.Errorf("building S3 client: %w", err)
	}
	output, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(cfg.Key)})
	if err != nil {
		return seis3.ClassifyS3Error("configure-genesis", cfg.Bucket, cfg.Key, cfg.Region, err)
	}
	defer output.Body.Close()
	data, err := io.ReadAll(io.LimitReader(output.Body, maxGenesisBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxGenesisBytes {
		return fmt.Errorf("genesis exceeds size limit")
	}
	if err := verifyGenesisBytes(data, g.chainID, expectedHash); err != nil {
		return err
	}
	if err := g.writeGenesisFile(func(f *os.File) error { _, err := io.Copy(f, bytes.NewReader(data)); return err }); err != nil {
		return err
	}
	completion, err := json.Marshal(genesisCompletion{ChainID: g.chainID, Digest: expectedHash})
	if err != nil {
		return err
	}
	return confinedWrite(home, genesisMarkerFile, completion, 0600)
}

func (g *GenesisFetcher) writeEmbeddedGenesis() error {
	if g.chainID == "" {
		return fmt.Errorf("configure-genesis: no S3 params and SEI_CHAIN_ID is not set")
	}

	genesisLog.Info("writing embedded genesis", "chainId", g.chainID)
	data, err := seiconfig.GenesisForChain(g.chainID)
	if err != nil {
		return fmt.Errorf("configure-genesis: %w", err)
	}

	if err := g.writeGenesisFile(func(f *os.File) error {
		_, err := f.Write(data)
		return err
	}); err != nil {
		return err
	}

	genesisLog.Info("genesis written from embedded data", "chainId", g.chainID)
	return writeMarker(g.homeDir, genesisMarkerFile)
}

// writeGenesisFile creates the config directory and genesis.json file, calling
// writeFn to populate its contents.
func (g *GenesisFetcher) writeGenesisFile(writeFn func(*os.File) error) error {
	dir, err := openConfinedDir(filepath.Join(g.homeDir, "config"), true)
	if err != nil {
		return err
	}
	defer dir.Close()
	tmp := fmt.Sprintf(".genesis-%x", randomFileID())
	fd, err := unix.Openat(int(dir.Fd()), tmp, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), tmp)
	defer f.Close()
	defer unix.Unlinkat(int(dir.Fd()), tmp, 0)
	if err := writeFn(f); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := rejectSymlinkAt(dir, "genesis.json"); err != nil {
		return err
	}
	if err := unix.Renameat(int(dir.Fd()), tmp, int(dir.Fd()), "genesis.json"); err != nil {
		return err
	}
	return dir.Sync()
}
