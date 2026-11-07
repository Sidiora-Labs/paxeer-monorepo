package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/internal/scaffold"
	"github.com/paxeer-network/pax-load/sender"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/paxeer-network/pax-load/utils/service"
)

var ErrPerformanceBounds = errors.New("performance bounds violated")

// NewCommand constructs the command used by the paxload executable.
func NewCommand() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		Use:   "paxload",
		Short: "Pax Chain Load Test v2",
		Long: `A load test generator for Pax Chain.

Supports both contract and non-contract scenarios with factory
and weighted scenario selection mechanisms. Features sharded sending
to multiple endpoints with account pooling management.

Use --dry-run to test configuration and view transaction details
without actually sending requests or deploying contracts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunLoadTest(cmd.Context(), cmd, args)
		},
	}

	rootCmd.AddCommand(scaffold.NewCommand())
	rootCmd.Flags().StringP("config", "c", "", "Path to configuration file (required)")
	rootCmd.Flags().DurationP("stats-interval", "s", 0, "Interval for logging statistics")
	rootCmd.Flags().IntP("buffer-size", "b", 0, "Buffer size per worker")
	rootCmd.Flags().Float64P("tps", "t", 0, "Transactions per second (0 = no limit)")
	rootCmd.Flags().Bool("dry-run", false, "Mock deployment and requests")
	rootCmd.Flags().Bool("debug", false, "Log each request")
	rootCmd.Flags().Bool("track-receipts", false, "Track receipts")
	rootCmd.Flags().Bool("track-blocks", false, "Track blocks")
	rootCmd.Flags().Bool("prewarm", false, "Prewarm accounts with self-transactions")
	rootCmd.Flags().Bool("track-user-latency", false, "Track user latency")
	rootCmd.Flags().IntP("workers", "w", 0, "Number of workers")
	rootCmd.Flags().IntP("nodes", "n", 0, "Number of nodes/endpoints to use (0 = use all)")
	rootCmd.Flags().String("metricsListenAddr", "0.0.0.0:9090", "The ip:port on which to export prometheus metrics.")
	rootCmd.Flags().Bool("ramp-up", false, "Ramp up loadtest")
	rootCmd.Flags().String("report-path", "", "JSON report path (overrides settings.reportPath and top-level reportPath; empty disables reporting)")
	rootCmd.Flags().Float64("min-tps", 0, "Minimum average transactions per second (0 = disabled)")
	rootCmd.Flags().Duration("max-p99-latency", 0, "Maximum p99 latency per endpoint (0 = disabled)")
	rootCmd.Flags().Uint64("min-total-txs", 0, "Minimum total transactions (0 = disabled)")
	rootCmd.Flags().String("txs-dir", "", "Path to save the transactions")
	rootCmd.Flags().Uint64("target-gas", 10_000_000, "Target gas per block")
	rootCmd.Flags().Int("num-blocks-to-write", 100, "Number of blocks to write")

	// Initialize Viper with proper error handling
	if err := config.InitializeViper(rootCmd); err != nil {
		return nil, fmt.Errorf("failed to initialize configuration: %w", err)
	}

	if err := rootCmd.MarkFlagRequired("config"); err != nil {
		return nil, err
	}
	return rootCmd, nil
}

func RunLoadTest(ctx context.Context, cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}
	// Parse the config file into a config.LoadConfig struct
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Keep optional profile settings fresh; Viper supplies operational defaults.
	if cfg.Settings == nil {
		cfg.Settings = &config.Settings{}
	}
	config.MergeBounds(cfg, cfg.Settings)
	config.MergeReportPath(cfg, cfg.Settings)

	// Load profile settings before resolving flags, including explicit zeroes.
	if err := config.LoadSettings(cfg.Settings); err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// Get resolved settings from the config package
	settings := config.ResolveSettings()

	if err := settings.Validate(len(cfg.Endpoints)); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}
	for i, scenario := range cfg.Scenarios {
		if !scenarios.HasScenario(scenario.Name) {
			return fmt.Errorf("scenarios[%d].name: unknown scenario %q", i, scenario.Name)
		}
	}

	// Handle --nodes flag to limit number of endpoints
	nodes, _ := cmd.Flags().GetInt("nodes")
	if nodes < 0 || nodes > len(cfg.Endpoints) {
		return fmt.Errorf("nodes: must be in [0,%d]", len(cfg.Endpoints))
	}
	if nodes > 0 && nodes < len(cfg.Endpoints) {
		log.Printf("🔧 Limiting endpoints from %d to %d nodes", len(cfg.Endpoints), nodes)
		cfg.Endpoints = cfg.Endpoints[:nodes]
	}

	log.Printf("🚀 Starting Pax Chain Load Test v2")
	log.Printf("📁 Config file: %s", configFile)
	log.Printf("🎯 Endpoints: %d", len(cfg.Endpoints))
	log.Printf("👥 Workers per endpoint: %d", settings.Workers)
	log.Printf("🔧 Total workers: %d", len(cfg.Endpoints)*settings.Workers)
	log.Printf("📊 Scenarios: %d", len(cfg.Scenarios))
	log.Printf("⏱️  Stats interval: %v", settings.StatsInterval.ToDuration())
	log.Printf("📦 Buffer size per worker: %d", settings.BufferSize)
	if settings.TPS > 0 {
		log.Printf("📈 Transactions per second: %.2f", settings.TPS)
	}
	if settings.DryRun {
		log.Printf("📝 Dry run: enabled")
	}
	if settings.TrackReceipts {
		log.Printf("📝 Track receipts: enabled")
	}
	if settings.TrackBlocks {
		log.Printf("📝 Track blocks: enabled")
	}
	if settings.Prewarm {
		log.Printf("📝 Prewarm: enabled")
	}
	if settings.TrackUserLatency {
		log.Printf("📝 Track user latency: enabled")
	}

	// Enable mock deployment in dry-run mode
	if settings.DryRun {
		cfg.MockDeploy = true
	}

	listenAddr := cmd.Flag("metricsListenAddr").Value.String()
	log.Printf("serving metrics at %s/metrics", listenAddr)

	metricsServer, err := NewMetricsServer(listenAddr)
	if err != nil {
		return err
	}
	defer metricsServer.Close()
	// Signals are subscribed before setup so shutdown during startup is retained.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Create statistics collector and logger
	collector := stats.NewCollector()
	logger := stats.NewLogger(collector, settings.StatsInterval.ToDuration(), settings.ReportPath, settings.Debug)
	var ramper *sender.Ramper

	var writer *sender.TxsWriter
	err = service.Run(ctx, func(ctx context.Context, s service.Scope) error {
		s.SpawnBgNamed("metrics", func() error { return metricsServer.Serve(ctx) })
		// Create the generator from the config struct
		gen, err := generator.NewConfigBasedGenerator(cfg)
		if err != nil {
			return fmt.Errorf("failed to create generator: %w", err)
		}

		// Create shared rate limiter for all workers if TPS is specified
		var sharedLimiter *rate.Limiter
		if settings.TPS > 0 {
			sharedLimiter = rate.NewLimiter(rate.Limit(settings.TPS), 1)
			log.Printf("📈 Rate limiting enabled: %.2f TPS shared across all workers", settings.TPS)
		} else {
			// No rate limiting
			sharedLimiter = rate.NewLimiter(rate.Inf, 1)
		}

		// Create and start block collector if endpoints are available
		var blockCollector *stats.BlockCollector
		if len(cfg.Endpoints) > 0 && settings.TrackBlocks {
			blockCollector = stats.NewBlockCollector(cfg.PaxChainID)
			collector.SetBlockCollector(blockCollector)
			s.SpawnBgNamed("block collector", func() error {
				return blockCollector.Run(ctx, cfg.Endpoints[0])
			})
		}

		if settings.RampUp {
			ramperBlockCollector := stats.NewBlockCollector(cfg.PaxChainID)
			s.SpawnBgNamed("ramper block collector", func() error {
				return ramperBlockCollector.Run(ctx, cfg.Endpoints[0])
			})

			ramper = sender.NewRamper(
				sender.NewRampCurveStep(100, 100, 120*time.Second, 30*time.Second),
				ramperBlockCollector,
				sharedLimiter,
			)
			s.SpawnBgNamed("ramper", func() error { return ramper.Run(ctx) })
		}

		// Create and start user latency tracker if endpoints are available
		if len(cfg.Endpoints) > 0 && settings.TrackUserLatency {
			userLatencyTracker := stats.NewUserLatencyTracker(settings.StatsInterval.ToDuration())
			s.SpawnBgNamed("user latency tracker", func() error {
				return userLatencyTracker.Run(ctx, cfg.Endpoints[0])
			})
		}

		// Create the sender from the config struct
		snd, err := sender.NewShardedSender(cfg, settings.BufferSize, settings.Workers, sharedLimiter)
		if err != nil {
			return fmt.Errorf("failed to create sender: %w", err)
		}

		// Enable dry-run mode in sender if specified
		if settings.DryRun {
			snd.SetDryRun(true)
		}
		if settings.Debug {
			snd.SetDebug(true)
		}
		if settings.TrackReceipts {
			snd.SetTrackReceipts(true)
		}
		if settings.TrackBlocks {
			snd.SetTrackBlocks(true)
		}

		// Set statistics collector for sender and its workers
		snd.SetStatsCollector(collector, logger)

		// Create dispatcher
		var dispatcher *sender.Dispatcher
		if settings.TxsDir != "" {
			// get latest height
			ethclient, err := ethclient.Dial(cfg.Endpoints[0])
			if err != nil {
				return fmt.Errorf("failed to create ethclient: %w", err)
			}
			defer ethclient.Close()
			latestHeight, err := ethclient.BlockNumber(ctx)
			if err != nil {
				return fmt.Errorf("failed to get latest height: %w", err)
			}
			numBlocksToWrite := settings.NumBlocksToWrite
			writerHeight := latestHeight + 10 // some buffer
			log.Printf("🔍 Latest height: %d, writer start height: %d", latestHeight, writerHeight)
			writer = sender.NewTxsWriter(settings.TargetGas, settings.TxsDir, writerHeight, uint64(numBlocksToWrite))
			dispatcher = sender.NewDispatcher(gen, writer)
		} else {
			dispatcher = sender.NewDispatcher(gen, snd)
		}

		// Set statistics collector for dispatcher
		dispatcher.SetStatsCollector(collector)

		// Set up prewarming if enabled
		if settings.Prewarm {
			log.Printf("🔥 Creating prewarm generator...")
			prewarmGen := generator.NewPrewarmGenerator(cfg, gen)
			dispatcher.SetPrewarmGenerator(prewarmGen)
			log.Printf("✅ Prewarm generator ready")
			log.Printf("📝 Prewarm mode: Accounts will be prewarmed")
		}

		senderDone := make(chan struct{})
		if settings.TxsDir == "" {
			// Sender keeps its run context while generation is stopped and joined.
			s.SpawnBgNamed("sender", func() error { defer close(senderDone); return snd.Run(ctx) })
			log.Printf("✅ Connected to %d endpoints", snd.GetNumShards())
		} else {
			close(senderDone)
		}
		// Perform prewarming if enabled (before starting logger to avoid logging prewarm transactions)
		if settings.Prewarm {
			if err := dispatcher.Prewarm(ctx); err != nil {
				return fmt.Errorf("failed to prewarm accounts: %w", err)
			}
		}

		// Start logger (after prewarming to capture only main load test metrics)
		s.SpawnBgNamed("logger", func() error { return logger.Run(ctx) })
		log.Printf("✅ Started statistics logger")

		// Start dispatcher for main load test
		generationCtx, stopGeneration := context.WithCancel(ctx)
		defer stopGeneration()
		dispatcherDone := make(chan struct{})
		s.SpawnBgNamed("dispatcher", func() error {
			defer close(dispatcherDone)
			err := dispatcher.Run(generationCtx)
			if generationCtx.Err() != nil && service.IsCancellation(err) {
				return nil
			}
			return err
		})
		log.Printf("✅ Started dispatcher")

		log.Printf("📈 Logging statistics every %v (Press Ctrl+C to stop)", settings.StatsInterval.ToDuration())
		if settings.DryRun {
			log.Printf("📝 Dry-run mode: Simulating requests without sending")
		}
		if settings.Debug {
			log.Printf("🐛 Debug mode: Each transaction will be logged")
		}
		if settings.TrackReceipts {
			log.Printf("📝 Track receipts mode: Receipts will be tracked")
		}
		if settings.TrackBlocks {
			log.Printf("📝 Track blocks mode: Block data will be collected")
		}
		if settings.TrackUserLatency {
			log.Printf("📝 Track user latency mode: User latency will be tracked")
		}
		log.Print(strings.Repeat("=", 60))

		// Stop the producer first; only then close queues and drain accepted work.
		select {
		case <-sigChan:
			log.Print("Received shutdown signal, stopping gracefully")
		case <-dispatcherDone:
		case <-ctx.Done():
		}
		stopGeneration()
		<-dispatcherDone
		snd.CloseInput()
		drainTimer := time.NewTimer(30 * time.Second)
		defer drainTimer.Stop()
		select {
		case <-senderDone:
			return ctx.Err()
		case <-ctx.Done():
			return ctx.Err()
		case <-drainTimer.C:
			return fmt.Errorf("sender drain: %w", context.DeadlineExceeded)
		}
	})
	// All generation and sending has joined. Tail publication precedes reporting,
	// and a real finalization failure is never discarded as scope cancellation.
	if writer != nil {
		err = errors.Join(err, writer.Close())
	}

	// Print final statistics
	finalStats, reportErr := logger.LogFinalStats()

	if settings.RampUp && ramper != nil {
		ramper.LogFinalStats()
	}
	log.Printf("👋 Shutdown complete")
	return errors.Join(err, reportErr, EnforceBounds(settings, finalStats))
}

func EnforceBounds(s config.Settings, fs *stats.FinalStats) error {
	bounds := stats.Bounds{
		MinAverageTPS: s.MinTPS,
		MaxP99Latency: s.MaxP99Latency.ToDuration(),
		MinTotalTxs:   s.MinTotalTxs,
	}
	violations := bounds.Check(fs)
	if len(violations) == 0 {
		return nil
	}
	details := make([]string, 0, len(violations))
	for _, violation := range violations {
		details = append(details, fmt.Sprintf("%s: limit=%s observed=%s", violation.Bound, violation.Limit, violation.Observed))
	}
	return fmt.Errorf("%w: %s", ErrPerformanceBounds, strings.Join(details, "; "))
}

// LoadConfig reads and parses the configuration file.
func LoadConfig(filename string) (*config.LoadConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config.LoadConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config json: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
