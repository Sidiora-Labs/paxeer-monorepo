package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/unit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
)

// BenchmarkConfig is a struct that holds the configuration for the benchmark.
type BenchmarkConfig struct {

	// Configuration for the LittDB instance.
	LittConfig *litt.Config

	// The location where the benchmark stores test metadata.
	MetadataDirectory string

	// The maximum target write throughput in MB/s.
	MaximumWriteThroughputMB float64

	// The maximum read throughput in MB/s.
	MaximumReadThroughputMB float64

	// The number of parallel write goroutines.
	WriterParallelism int

	// The number of parallel read goroutines.
	ReaderParallelism int

	// The size of the values in MB.
	ValueSizeMB float64

	// Data is written to the DB in batches and then flushed. This determines the size of those batches, in MB.
	BatchSizeMB float64

	// The frequency at which the benchmark does cohort garbage collection, in seconds
	CohortGCPeriodSeconds float64

	// The size of the write info channel. Controls the max number of keys to prepare for writing ahead of time.
	WriteInfoChanelSize uint64

	// The size of the read info channel. Controls the max number of keys to prepare for reading ahead of time.
	ReadInfoChanelSize uint64

	// The number of keys in a new cohort.
	CohortSize uint64

	// The time-to-live (TTL) for keys in the database, in hours.
	TTLHours float64

	// If data is within this many minutes of its expiration time, it will not be read.
	ReadSafetyMarginMinutes float64

	// A seed for the random number generator used to generate keys and values. When restarting the benchmark,
	// it's important to always use the same seed.
	Seed int64

	// The size of the pool of random data. Instead of generating random data for each key/value pair
	// (which is expensive), data from this pool is reused. When restarting the benchmark,
	// it's important to always use the same pool size.
	RandomPoolSize uint64

	// When the benchmark starts, it sleeps for a length of time. The average amount of time spent sleeping is equal to
	// this value, in seconds. The purpose of this sleeping to stagger the start of the workers so that they don't all
	// operate in lockstep.
	StartupSleepFactorSeconds float64

	// The frequency at which the benchmark logs metrics, in seconds. If zero, then metrics logging is disabled.
	MetricsLoggingPeriodSeconds float64

	// Retained for configuration compatibility. Integrity violations always fail the run.
	PanicOnReadFailure bool

	// If true, fsync cohort files to ensure atomicity. Can be set to false for unit tests that need to be fast.
	Fsync bool

	// If non-zero, then the benchmark will run for this many seconds and then stop. If zero,
	// the benchmark will run until it is manually stopped.
	TimeLimitSeconds float64
}

// DefaultBenchmarkConfig returns a default BenchmarkConfig.
func DefaultBenchmarkConfig() *BenchmarkConfig {

	littConfig := litt.DefaultConfigNoPaths()
	littConfig.MetricsEnabled = true

	return &BenchmarkConfig{
		LittConfig:                  littConfig,
		MetadataDirectory:           "~/benchmark",
		MaximumWriteThroughputMB:    10,
		MaximumReadThroughputMB:     10,
		WriterParallelism:           4,
		ReaderParallelism:           32,
		ValueSizeMB:                 2.0,
		BatchSizeMB:                 32,
		CohortGCPeriodSeconds:       10.0,
		WriteInfoChanelSize:         1024,
		ReadInfoChanelSize:          1024,
		CohortSize:                  1024,
		TTLHours:                    1.0,
		ReadSafetyMarginMinutes:     5.0,
		Seed:                        1337,
		RandomPoolSize:              unit.GB,
		StartupSleepFactorSeconds:   0.5,
		MetricsLoggingPeriodSeconds: 60.0,
		PanicOnReadFailure:          false,
		TimeLimitSeconds:            0.0,
	}
}

// LoadConfig loads the benchmark configuration from the json file at the given path.
func LoadConfig(path string) (*BenchmarkConfig, error) {
	config := DefaultBenchmarkConfig()

	path, err := util.SanitizePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize path: %w", err)
	}

	// Read the file
	data, err := os.ReadFile(path) //nolint:gosec // caller-supplied config path
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Create a decoder that will return an error if there are unmatched fields
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()

	// Unmarshal JSON into config struct
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid benchmark config: %w", err)
	}

	config.MetadataDirectory, err = util.SanitizePath(config.MetadataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize metadata directory: %w", err)
	}

	return config, nil
}

// Validate rejects configuration that cannot safely reach the benchmark's runtime consumers.
func (c *BenchmarkConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("configuration must not be null")
	}
	if c.LittConfig == nil {
		return fmt.Errorf("LittConfig must not be null")
	}
	if err := c.LittConfig.Validate(); err != nil {
		return fmt.Errorf("LittConfig: %w", err)
	}
	if len(c.LittConfig.Paths) > litt.MaxShardingFactor {
		return fmt.Errorf("LittConfig.Paths must contain at most %d paths", litt.MaxShardingFactor)
	}
	for i, path := range c.LittConfig.Paths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("LittConfig.Paths[%d] must not be empty", i)
		}
	}
	if c.LittConfig.ControlChannelSize < 1 {
		return fmt.Errorf("LittConfig.ControlChannelSize must be positive")
	}
	if c.LittConfig.GCPeriod < 1 {
		return fmt.Errorf("LittConfig.GCPeriod must be positive")
	}
	if c.LittConfig.MetricsEnabled {
		if c.LittConfig.MetricsUpdateInterval < 1 {
			return fmt.Errorf("LittConfig.MetricsUpdateInterval must be positive when metrics are enabled")
		}
		if c.LittConfig.MetricsPort < 0 || c.LittConfig.MetricsPort > 65535 {
			return fmt.Errorf("LittConfig.MetricsPort must be between 0 and 65535")
		}
	}
	if c.LittConfig.MinimumFlushInterval < 0 {
		return fmt.Errorf("LittConfig.MinimumFlushInterval must not be negative")
	}
	switch c.LittConfig.KeymapType {
	case keymap.MemKeymapType, keymap.PebbleDBKeymapType, keymap.UnsafePebbleDBKeymapType:
	default:
		return fmt.Errorf("LittConfig.KeymapType is unsupported: %q", c.LittConfig.KeymapType)
	}
	if c.WriterParallelism < 1 {
		return fmt.Errorf("WriterParallelism must be positive")
	}
	if c.ReaderParallelism < 1 {
		return fmt.Errorf("ReaderParallelism must be positive")
	}
	maxInt := uint64(^uint(0) >> 1)
	maxChannelSize := maxInt / uint64(strconv.IntSize/8)
	if c.WriteInfoChanelSize == 0 || c.WriteInfoChanelSize > maxChannelSize {
		return fmt.Errorf("WriteInfoChanelSize must be between 1 and %d", maxChannelSize)
	}
	if c.ReadInfoChanelSize == 0 || c.ReadInfoChanelSize > maxChannelSize {
		return fmt.Errorf("ReadInfoChanelSize must be between 1 and %d", maxChannelSize)
	}
	if c.CohortSize == 0 || c.CohortSize > math.MaxInt64 {
		return fmt.Errorf("CohortSize must be between 1 and %d", int64(math.MaxInt64))
	}
	if c.RandomPoolSize == 0 || c.RandomPoolSize > maxInt {
		return fmt.Errorf("RandomPoolSize must be between 1 and %d", maxInt)
	}
	for _, size := range []struct {
		name    string
		value   float64
		minimum float64
		limit   float64
	}{
		{"ValueSizeMB", c.ValueSizeMB, 1, math.Min(float64(math.MaxUint32)+1, float64(maxInt))},
		{"BatchSizeMB", c.BatchSizeMB, 1, float64(maxInt)},
		{"MaximumWriteThroughputMB", c.MaximumWriteThroughputMB, float64(c.WriterParallelism), math.Exp2(64)},
		{"MaximumReadThroughputMB", c.MaximumReadThroughputMB, float64(c.ReaderParallelism), math.Exp2(64)},
	} {
		bytes := size.value * float64(unit.MB)
		if math.IsNaN(bytes) || math.IsInf(bytes, 0) || bytes < size.minimum || bytes >= size.limit {
			return fmt.Errorf("%s must be finite and convert to at least %g and less than %g bytes", size.name, size.minimum, size.limit)
		}
	}
	for _, duration := range []struct {
		name       string
		value      float64
		unit       time.Duration
		optional   bool
		multiplier float64
	}{
		{"CohortGCPeriodSeconds", c.CohortGCPeriodSeconds, time.Second, false, 1},
		{"TTLHours", c.TTLHours, time.Hour, false, 1},
		{"ReadSafetyMarginMinutes", c.ReadSafetyMarginMinutes, time.Minute, true, 1},
		{"StartupSleepFactorSeconds", c.StartupSleepFactorSeconds, time.Second, true, 2},
		{"MetricsLoggingPeriodSeconds", c.MetricsLoggingPeriodSeconds, time.Second, true, 1},
		{"TimeLimitSeconds", c.TimeLimitSeconds, time.Second, true, 1},
	} {
		if duration.optional && duration.value == 0 {
			continue
		}
		nanoseconds := duration.value * float64(duration.unit) * duration.multiplier
		if math.IsNaN(nanoseconds) || math.IsInf(nanoseconds, 0) || nanoseconds < 1 || nanoseconds >= math.Exp2(63) {
			return fmt.Errorf("%s must be finite and convert to a positive, representable duration", duration.name)
		}
	}
	if time.Duration(c.ReadSafetyMarginMinutes*float64(time.Minute)) >= time.Duration(c.TTLHours*float64(time.Hour)) {
		return fmt.Errorf("ReadSafetyMarginMinutes must be shorter than TTLHours")
	}
	return nil
}
