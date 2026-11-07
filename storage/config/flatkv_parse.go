package config

import (
	"fmt"
	"strconv"

	"github.com/spf13/cast"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
)

const fsync = "state-commit.flatkv.fsync"

// ParseFlatKVConfig reads the FlatKV keys emitted by StateCommitConfigTemplate.
// Missing keys preserve defaults; malformed values return a key-qualified error.
func ParseFlatKVConfig(opts interface{ Get(string) interface{} }) (config.Config, error) {
	cfg := *config.DefaultConfig()

	key := fsync
	if value := opts.Get(key); value != nil {
		parsed, err := cast.ToBoolE(value)
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", key, err)
		}
		cfg.Fsync = parsed
	}

	key = "state-commit.flatkv.async-write-buffer"
	if value := opts.Get(key); value != nil {
		parsed, err := cast.ToIntE(value)
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", key, err)
		}
		cfg.AsyncWriteBuffer = parsed
	}

	key = "state-commit.flatkv.snapshot-interval"
	if value := opts.Get(key); value != nil {
		parsed, err := parseFlatKVSnapshotCount(value)
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", key, err)
		}
		cfg.SnapshotInterval = parsed
	}

	key = "state-commit.flatkv.snapshot-keep-recent"
	if value := opts.Get(key); value != nil {
		parsed, err := parseFlatKVSnapshotCount(value)
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", key, err)
		}
		cfg.SnapshotKeepRecent = parsed
	}

	return cfg, nil
}

func parseFlatKVSnapshotCount(value interface{}) (uint32, error) {
	// Casting a TOML int64 directly to uint32 silently wraps values above
	// MaxUint32. Parse at the destination width before converting instead.
	text, err := cast.ToStringE(value)
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(parsed), nil
}
