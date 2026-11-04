package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"
)

type rebuildOptions struct {
	manifestPath, schema, pool, id string
	from, to                       int64
	promote                        bool
}

func parseRebuildOptions(args []string) (rebuildOptions, error) {
	var o rebuildOptions
	flags := flag.NewFlagSet("charts-rebuild", flag.ContinueOnError)
	flags.StringVar(&o.manifestPath, "manifests", "", "approved raw-archive-v2 manifest JSON array")
	flags.StringVar(&o.schema, "schema", "candles", "admitted chart-owned schema")
	flags.StringVar(&o.pool, "pool", "", "chain125 pool address")
	flags.StringVar(&o.id, "run-id", "", "unique reconstruction ID")
	flags.Int64Var(&o.from, "from", 0, "inclusive Unix timestamp")
	flags.Int64Var(&o.to, "to", 0, "exclusive Unix timestamp")
	flags.BoolVar(&o.promote, "promote", false, "atomically promote after economic parity")
	err := flags.Parse(args)
	return o, err
}

func loadRebuildConfig(schema string) (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, err
	}
	if cfg.NodeEnv != "production" || (cfg.RuntimeProfile != "container" && cfg.RuntimeProfile != "owned_local") {
		return config.Config{}, errors.New("rebuild requires an admitted production runtime profile")
	}
	if cfg.RuntimeProfile == "container" && (cfg.Port != 8080 || cfg.BindAddress != "0.0.0.0") {
		return config.Config{}, errors.New("container_bind_mismatch")
	}
	if cfg.RuntimeProfile == "owned_local" && cfg.BindAddress != "127.0.0.1" {
		return config.Config{}, errors.New("owned_loopback_required")
	}
	if schema != "candles" {
		return config.Config{}, errors.New("rebuild schema must match the admitted candles schema")
	}
	var delivery map[string]json.RawMessage
	if err = config.ReadBoundDocument(cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256, &delivery); err != nil {
		return config.Config{}, fmt.Errorf("rebuild delivery binding: %w", err)
	}
	var admitted string
	if err = json.Unmarshal(delivery["schema"], &admitted); err != nil || admitted != schema {
		return config.Config{}, errors.New("rebuild delivery schema binding mismatch")
	}
	var expected dbproduction.ExpectedSchema
	if err = config.ReadBoundDocument(cfg.ExpectedSchemaFile, cfg.ExpectedSchemaSHA256, &expected); err != nil {
		return config.Config{}, fmt.Errorf("rebuild expected schema binding: %w", err)
	}
	if expected.Service != "core/trading-charts" || expected.Schemas[schema] == "" {
		return config.Config{}, errors.New("rebuild runtime schema binding mismatch")
	}
	return cfg, nil
}

func runArgs(args []string) error {
	o, err := parseRebuildOptions(args)
	if err != nil {
		return err
	}
	manifestPath, schema, pool, id := o.manifestPath, o.schema, o.pool, o.id
	from, to, promote := o.from, o.to, o.promote
	if !promote || manifestPath == "" || !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`).MatchString(id) || !regexp.MustCompile(`^0x[0-9a-f]{40}$`).MatchString(pool) || from < 0 || to <= from || os.Getenv("DATABASE_URL") == "" || os.Getenv("RPC_URL") == "" {
		return errors.New("explicit approved manifests, pool, bounded timestamps, unique run-id, --promote, DATABASE_URL and RPC_URL required")
	}
	cfg, err := loadRebuildConfig(schema)
	if err != nil {
		return err
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 16<<20+1))
	if err != nil || len(raw) > 16<<20 {
		return errors.New("manifest file exceeds bound")
	}
	var manifests []store.ArchiveManifest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifests); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("trailing manifest data")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	database, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("chart database configuration unavailable")
	}
	defer database.Close()
	if err = cfg.CheckRuntimeSchema(ctx, database); err != nil {
		return fmt.Errorf("rebuild runtime schema readiness: %w", err)
	}
	exact, err := store.NewExact(database, schema)
	if err != nil {
		return err
	}
	rpc, err := ethclient.DialContext(ctx, cfg.RPCURL)
	if err != nil {
		return errors.New("canonical RPC unavailable")
	}
	defer rpc.Close()
	chain, err := rpc.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(big.NewInt(125)) != 0 {
		return errors.New("canonical chain125 RPC required")
	}
	objects, err := storage.New(ctx, storage.Config{Endpoint: os.Getenv("S3_ENDPOINT"), Region: os.Getenv("S3_REGION"), AccessKeyID: os.Getenv("S3_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"), Bucket: os.Getenv("S3_BUCKET")})
	if err != nil {
		return errors.New("archive object reader configuration unavailable")
	}
	reader := store.ArchiveReader{Objects: objects, Bucket: objects.Bucket(), Exact: exact, BlockTime: func(ctx context.Context, height int64, expectedHash string) (int64, error) {
		bounded, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		block, err := (chainread.Reader{RPC: rpc}).Block(bounded, height)
		if err != nil {
			return 0, errors.New("archive boundary header unavailable")
		}
		if !strings.EqualFold(block.Hash, expectedHash) {
			return 0, store.ErrIntegrity
		}
		return block.Time, nil
	}}
	report, err := engine.ReconstructArchive(ctx, exact, reader, id, pool, from, to, manifests)
	if err != nil {
		return fmt.Errorf("reconstruction failed: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
func run() error {
	return runArgs(os.Args[1:])
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
