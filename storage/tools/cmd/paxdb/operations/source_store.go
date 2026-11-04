package operations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb/mvcc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/ktype"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/tools/cmd/paxdb/benchmark"
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
)

// existingStateStoreConfig treats dbDir as the database itself, never a node home.
func existingStateStoreConfig(dbBackend, dbDir string) (config.StateStoreConfig, error) {
	cfg := config.DefaultStateStoreConfig()
	if !benchmark.ValidDBBackends[dbBackend] {
		return cfg, fmt.Errorf("unsupported db backend: %s", dbBackend)
	}
	if dbDir == "" {
		return cfg, fmt.Errorf("must provide database dir")
	}
	info, err := os.Stat(dbDir)
	if err != nil {
		return cfg, fmt.Errorf("source database: %w", err)
	}
	if !info.IsDir() {
		return cfg, fmt.Errorf("source database %q is not a directory", dbDir)
	}
	// Pebble's active manifest moved from CURRENT to format-version markers.
	// Peek understands both layouts without opening or creating a database.
	manifestPath := filepath.Join(dbDir, "CURRENT")
	if dbBackend == config.PebbleDBBackend {
		desc, err := pebble.Peek(dbDir, vfs.Default)
		if err != nil {
			return cfg, fmt.Errorf("inspect source database: %w", err)
		}
		if !desc.Exists {
			return cfg, fmt.Errorf("source database %q does not exist", dbDir)
		}
		manifestPath = desc.ManifestFilename
	}
	info, err = os.Stat(manifestPath)
	if err != nil {
		return cfg, fmt.Errorf("source database manifest: %w", err)
	}
	if !info.Mode().IsRegular() {
		return cfg, fmt.Errorf("source database manifest is not a regular file")
	}
	cfg.Backend = dbBackend
	cfg.DBDirectory = dbDir
	cfg.KeepRecent = 0
	return cfg, nil
}

func persistedPruneStores(cfg config.StateStoreConfig) (stores []*proto.NamedChangeSet, retErr error) {
	comparer := mvcc.MVCCComparer
	if cfg.UseDefaultComparer {
		comparer = pebble.DefaultComparer
	}
	db, err := pebble.Open(cfg.DBDirectory, &pebble.Options{Comparer: comparer, ReadOnly: true, ErrorIfNotExists: true})
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, db.Close()) }()
	iter, err := db.NewIter(&pebble.IterOptions{LowerBound: []byte(mvcc.PrefixStore), UpperBound: []byte("s/k;")})
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, iter.Close()) }()
	seen := make(map[string]bool)
	for iter.First(); iter.Valid(); iter.Next() {
		key, _, ok := mvcc.SplitMVCCKey(iter.Key())
		if !ok || len(key) < len(mvcc.PrefixStore) {
			return nil, fmt.Errorf("invalid persisted store key %X", iter.Key())
		}
		name, _, err := ktype.StripModulePrefix(key[len(mvcc.PrefixStore):])
		if err != nil || name == "" {
			return nil, fmt.Errorf("invalid persisted store name in %X", key)
		}
		if !seen[name] {
			seen[name] = true
			stores = append(stores, &proto.NamedChangeSet{Name: name})
		}
	}
	return stores, iter.Error()
}
