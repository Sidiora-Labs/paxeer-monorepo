package config

import (
	"fmt"

	"github.com/spf13/cast"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
)

func ParseFlatKVConfig(opts interface{ Get(string) interface{} }) (config.Config, error) {
	cfg := *config.DefaultConfig()

	key := "state-commit.flatkv.fsync"
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
		parsed, err := cast.ToUint32E(value)
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", key, err)
		}
		cfg.SnapshotInterval = parsed
	}

	key = "state-commit.flatkv.snapshot-keep-recent"
	if value := opts.Get(key); value != nil {
		parsed, err := cast.ToUint32E(value)
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", key, err)
		}
		cfg.SnapshotKeepRecent = parsed
	}

	return cfg, cfg.Validate()
}
