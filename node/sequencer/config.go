package sequencer

import (
	"errors"
	"fmt"
	"net"
	"time"

	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
)

const (
	FlagEnable                 = "sequencer.enable"
	FlagMode                   = "sequencer.mode"
	FlagWALDir                 = "sequencer.wal-dir"
	FlagReceiptKeyFile         = "sequencer.receipt-key-file"
	flagReceiptForwardURL      = "sequencer.receipt-forward-url"
	flagRole                   = "sequencer.role"
	FlagStandbyAddress         = "sequencer.standby-address"
	FlagReplicationTimeout     = "sequencer.replication-timeout"
	FlagMicroBatchWindow       = "sequencer.microbatch-window"
	FlagMicroBatchMaxCount     = "sequencer.microbatch-max-count"
	FlagWALRetainBlocks        = "sequencer.wal-retain-blocks"
	FlagNonceQueueMaxPerSender = "sequencer.nonce-queue-max-per-sender"
	FlagNonceQueueMaxTotal     = "sequencer.nonce-queue-max-total"
	FlagLeaseDuration          = "sequencer.lease-duration"
)

const (
	modeSequentialName = "sequential"
	modeMicroBatchName = "microbatch"
	rolePrimaryName    = "primary"
	roleStandbyName    = "standby"
)

// Role selects whether the sequencer originates or receives WAL replication.
type Role uint8

const (
	// RolePrimary originates WAL replication and is the default role.
	RolePrimary Role = iota
	// RoleStandby listens for WAL replication at StandbyAddress.
	RoleStandby
)

// String returns the config name of the role.
func (r Role) String() string {
	switch r {
	case RolePrimary:
		return rolePrimaryName
	case RoleStandby:
		return roleStandbyName
	default:
		return fmt.Sprintf("Role(%d)", uint8(r))
	}
}

var (
	// ErrConfigMode refuses a sequencer mode the executor does not implement.
	ErrConfigMode = errors.New("sequencer config: unknown mode")
	// ErrConfigWALDir refuses an enabled sequencer section without a WAL directory.
	ErrConfigWALDir = errors.New("sequencer config: wal-dir is required")
	// ErrConfigReceiptKeyFile refuses an enabled sequencer section without a receipt key file.
	ErrConfigReceiptKeyFile = errors.New("sequencer config: receipt-key-file is required")
	// ErrConfigStandbyAddress refuses a standby address that is not host:port.
	ErrConfigStandbyAddress = errors.New("sequencer config: invalid standby-address")
	// ErrConfigReplicationTimeout refuses a negative replication timeout, or a non-positive one
	// when a standby address is set.
	ErrConfigReplicationTimeout = errors.New("sequencer config: invalid replication-timeout")
	// ErrConfigValue refuses a sequencer option with an invalid type, role or combination.
	ErrConfigValue = errors.New("sequencer config: invalid value")
)

// Config is the sequencer section of the app config.
type Config struct {
	// Enable runs this node as the sequencer.
	Enable bool
	// Role selects the replication role; RolePrimary is the default.
	Role Role
	// Mode selects the executor; Sequential is the default.
	Mode Mode
	// WALDir is the directory of the sequencer WAL.
	WALDir                 string
	WALRetainBlocks        uint64
	NonceQueueMaxPerSender int
	NonceQueueMaxTotal     int
	LeaseDuration          time.Duration
	// ReceiptKeyFile holds the ed25519 receipt signing seed, apart from the consensus key.
	ReceiptKeyFile string
	// ReceiptForwardURL forwards receipt requests when this node is not the sequencer.
	ReceiptForwardURL string
	// StandbyAddress is the replication destination on a primary and the replication
	// listen address on a standby, in host:port form.
	StandbyAddress string
	// ReplicationTimeout bounds one replication exchange with the standby. It is set at
	// activation rehearsal; there is no default.
	ReplicationTimeout time.Duration
	// MicroBatchWindow and MicroBatchMaxCount bound one MicroBatch group. They are set at
	// activation rehearsal; there is no default.
	MicroBatchWindow   time.Duration
	MicroBatchMaxCount int
}

// DefaultConfig returns the sequencer section of a node that is not the sequencer.
func DefaultConfig() Config {
	queue := NonceQueueConfig{}
	cfg := Config{
		Mode:                   ModeSequential,
		Role:                   RolePrimary,
		WALRetainBlocks:        2,
		NonceQueueMaxPerSender: queue.PerSender,
		NonceQueueMaxTotal:     queue.Total,
	}
	cfg.LeaseDuration = 3 * cfg.ReplicationTimeout
	return cfg
}

// AddStartFlags registers the sequencer section on the node's start command.
func AddStartFlags(cmd *cobra.Command) {
	cfg := DefaultConfig()
	flags := cmd.Flags()
	flags.Bool(FlagEnable, cfg.Enable, "Run this node as the sequencer")
	flags.String(FlagMode, cfg.Mode.String(), "Sequencer executor: sequential or microbatch")
	flags.String(FlagWALDir, cfg.WALDir, "Sequencer write-ahead log directory")
	flags.String(FlagReceiptKeyFile, cfg.ReceiptKeyFile, "Sequencer receipt signing seed file")
	flags.String(flagReceiptForwardURL, cfg.ReceiptForwardURL, "Forward receipt requests to this URL when disabled")
	flags.String(flagRole, cfg.Role.String(), "Sequencer replication role: primary or standby")
	flags.String(FlagStandbyAddress, cfg.StandbyAddress, "Standby replication destination or listen address")
	flags.Duration(FlagReplicationTimeout, cfg.ReplicationTimeout, "Timeout for one standby replication exchange")
	flags.Duration(FlagMicroBatchWindow, cfg.MicroBatchWindow, "Maximum wait for a microbatch group")
	flags.Int(FlagMicroBatchMaxCount, cfg.MicroBatchMaxCount, "Maximum transactions in a microbatch group")
	flags.Uint64(FlagWALRetainBlocks, cfg.WALRetainBlocks, "Number of committed blocks to retain in the WAL")
	flags.Int(FlagNonceQueueMaxPerSender, cfg.NonceQueueMaxPerSender, "Maximum future nonces held per sender; set at activation")
	flags.Int(FlagNonceQueueMaxTotal, cfg.NonceQueueMaxTotal, "Maximum future nonces held in total; set at activation")
	flags.Duration(FlagLeaseDuration, cfg.LeaseDuration, "Sequencer lease duration; defaults to three times replication-timeout")
}

// String returns the config name of the mode.
func (m Mode) String() string {
	switch m {
	case ModeSequential:
		return modeSequentialName
	case ModeMicroBatch:
		return modeMicroBatchName
	default:
		return fmt.Sprintf("Mode(%d)", uint8(m))
	}
}

// ParseMode maps a config mode name to its Mode.
func ParseMode(name string) (Mode, error) {
	switch name {
	case modeSequentialName:
		return ModeSequential, nil
	case modeMicroBatchName:
		return ModeMicroBatch, nil
	default:
		return 0, fmt.Errorf("%w %q", ErrConfigMode, name)
	}
}

// Validate refuses unknown roles, conflicting options and an enabled section that cannot start.
func (c Config) Validate() error {
	switch c.Role {
	case RolePrimary, RoleStandby:
	default:
		return fmt.Errorf("%w: %s: unknown role %s", ErrConfigValue, flagRole, c.Role)
	}
	if c.Enable && c.ReceiptForwardURL != "" {
		return fmt.Errorf("%w: %s cannot be set when %s is true", ErrConfigValue, flagReceiptForwardURL, FlagEnable)
	}
	if !c.Enable {
		return nil
	}
	switch c.Mode {
	case ModeSequential:
	case ModeMicroBatch:
		if c.MicroBatchWindow <= 0 {
			return fmt.Errorf("%w: got %s", ErrMicroBatchWindow, c.MicroBatchWindow)
		}
		if c.MicroBatchMaxCount <= 0 {
			return fmt.Errorf("%w: got %d", ErrMicroBatchMaxCount, c.MicroBatchMaxCount)
		}
	default:
		return fmt.Errorf("%w %s", ErrConfigMode, c.Mode)
	}
	if c.WALDir == "" {
		return ErrConfigWALDir
	}
	if c.ReceiptKeyFile == "" {
		return ErrConfigReceiptKeyFile
	}
	if c.ReplicationTimeout < 0 {
		return fmt.Errorf("%w: %s is negative", ErrConfigReplicationTimeout, c.ReplicationTimeout)
	}
	if c.StandbyAddress != "" {
		if _, _, err := net.SplitHostPort(c.StandbyAddress); err != nil {
			return fmt.Errorf("%w %q: %v", ErrConfigStandbyAddress, c.StandbyAddress, err)
		}
		if c.ReplicationTimeout == 0 {
			return fmt.Errorf("%w: must be positive when standby-address is set", ErrConfigReplicationTimeout)
		}
	}
	if c.WALRetainBlocks == 0 {
		return fmt.Errorf("%w: %s must be positive", ErrConfigValue, FlagWALRetainBlocks)
	}
	if c.NonceQueueMaxPerSender <= 0 {
		return fmt.Errorf("%w: %s must be positive", ErrConfigValue, FlagNonceQueueMaxPerSender)
	}
	if c.NonceQueueMaxTotal <= 0 {
		return fmt.Errorf("%w: %s must be positive", ErrConfigValue, FlagNonceQueueMaxTotal)
	}
	if c.NonceQueueMaxTotal < c.NonceQueueMaxPerSender {
		return fmt.Errorf("%w: %s must be at least %s", ErrConfigValue, FlagNonceQueueMaxTotal, FlagNonceQueueMaxPerSender)
	}
	if c.LeaseDuration <= 0 {
		return fmt.Errorf("%w: %s must be positive", ErrConfigValue, FlagLeaseDuration)
	}
	return nil
}

// ReadConfig reads the sequencer section from the app options over DefaultConfig and validates it.
func ReadConfig(opts servertypes.AppOptions) (Config, error) {
	cfg := DefaultConfig()
	var err error
	if v := opts.Get(FlagEnable); v != nil {
		if cfg.Enable, err = cast.ToBoolE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagEnable, err)
		}
	}
	if v := opts.Get(FlagMode); v != nil {
		name, err := cast.ToStringE(v)
		if err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagMode, err)
		}
		if cfg.Mode, err = ParseMode(name); err != nil {
			return cfg, err
		}
	}
	if v := opts.Get(flagRole); v != nil {
		name, err := cast.ToStringE(v)
		if err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, flagRole, err)
		}
		switch name {
		case rolePrimaryName:
			cfg.Role = RolePrimary
		case roleStandbyName:
			cfg.Role = RoleStandby
		default:
			return cfg, fmt.Errorf("%w: %s: unknown role %q", ErrConfigValue, flagRole, name)
		}
	}
	if v := opts.Get(flagReceiptForwardURL); v != nil {
		if cfg.ReceiptForwardURL, err = cast.ToStringE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, flagReceiptForwardURL, err)
		}
	}
	if v := opts.Get(FlagWALDir); v != nil {
		if cfg.WALDir, err = cast.ToStringE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagWALDir, err)
		}
	}
	if v := opts.Get(FlagReceiptKeyFile); v != nil {
		if cfg.ReceiptKeyFile, err = cast.ToStringE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagReceiptKeyFile, err)
		}
	}
	if v := opts.Get(FlagStandbyAddress); v != nil {
		if cfg.StandbyAddress, err = cast.ToStringE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagStandbyAddress, err)
		}
	}
	if v := opts.Get(FlagReplicationTimeout); v != nil {
		if cfg.ReplicationTimeout, err = cast.ToDurationE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagReplicationTimeout, err)
		}
	}
	if v := opts.Get(FlagMicroBatchWindow); v != nil {
		if cfg.MicroBatchWindow, err = cast.ToDurationE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagMicroBatchWindow, err)
		}
	}
	if v := opts.Get(FlagMicroBatchMaxCount); v != nil {
		if cfg.MicroBatchMaxCount, err = cast.ToIntE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagMicroBatchMaxCount, err)
		}
	}
	if v := opts.Get(FlagWALRetainBlocks); v != nil {
		if cfg.WALRetainBlocks, err = cast.ToUint64E(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagWALRetainBlocks, err)
		}
	}
	if v := opts.Get(FlagNonceQueueMaxPerSender); v != nil {
		if cfg.NonceQueueMaxPerSender, err = cast.ToIntE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagNonceQueueMaxPerSender, err)
		}
	}
	if v := opts.Get(FlagNonceQueueMaxTotal); v != nil {
		if cfg.NonceQueueMaxTotal, err = cast.ToIntE(v); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagNonceQueueMaxTotal, err)
		}
	}
	lease := opts.Get(FlagLeaseDuration)
	// Viper exposes bound flag defaults through Get even when the flag was not set.
	if explicit, ok := opts.(interface{ IsSet(string) bool }); ok && !explicit.IsSet(FlagLeaseDuration) {
		lease = nil
	}
	if lease != nil {
		if cfg.LeaseDuration, err = cast.ToDurationE(lease); err != nil {
			return cfg, fmt.Errorf("%w: %s: %v", ErrConfigValue, FlagLeaseDuration, err)
		}
	} else {
		if cfg.ReplicationTimeout > time.Duration(1<<63-1)/3 {
			return cfg, fmt.Errorf("%w: %s overflows three times %s", ErrConfigValue, FlagLeaseDuration, FlagReplicationTimeout)
		}
		cfg.LeaseDuration = 3 * cfg.ReplicationTimeout
	}
	return cfg, cfg.Validate()
}

// MicroBatchOptions returns the MicroBatch group bounds of the section.
func (c Config) MicroBatchOptions() MicroBatchOptions {
	return MicroBatchOptions{Window: c.MicroBatchWindow, MaxCount: c.MicroBatchMaxCount}
}

// ConfigTemplate is the sequencer section of app.toml with the DefaultConfig values.
const ConfigTemplate = `
###############################################################################
###                         Sequencer Configuration                         ###
###############################################################################

[sequencer]
# enable runs this node as the sequencer. When true, wal-dir and receipt-key-file are required.
enable = false

# role selects the replication role. Supported: "primary" (default), "standby".
role = "` + rolePrimaryName + `"

# mode selects the executor. Supported: "sequential", "microbatch".
mode = "` + modeSequentialName + `"

# wal-dir is the directory of the sequencer write-ahead log.
wal-dir = ""

# wal-retain-blocks is the positive number of committed WAL blocks to retain.
wal-retain-blocks = 2

# Future nonce queue bounds are set at activation; both must be positive when enabled.
nonce-queue-max-per-sender = 0
nonce-queue-max-total = 0

# receipt-key-file holds the hex ed25519 receipt signing seed, mode 0600, apart from the consensus key.
receipt-key-file = ""

# receipt-forward-url forwards receipt requests to the sequencer when enable is false.
# It must be empty when enable is true.
receipt-forward-url = ""

# standby-address is the replication destination on a primary and the replication
# listen address on a standby, in host:port form.
standby-address = ""

# replication-timeout bounds one replication exchange with the standby; it must be positive
# when standby-address is set.
replication-timeout = "0s"

# lease-duration must be positive when enabled; omit it to use three times replication-timeout.
lease-duration = "0s"

# microbatch-window is how long the first transaction of a MicroBatch group waits for others;
# it must be positive when mode is "microbatch".
microbatch-window = "0s"

# microbatch-max-count closes a MicroBatch group early once it holds this many transactions;
# it must be positive when mode is "microbatch".
microbatch-max-count = 0
`
