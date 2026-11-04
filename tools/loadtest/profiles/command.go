package profiles

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/config/cli"
	"github.com/spf13/cobra"
)

const chainCheckTimeout = 5 * time.Second

// NewCommand constructs the executable's CLI with a chain identity preflight.
// Validate the whole configured pool, including endpoints excluded by --nodes,
// before RunLoadTest can deploy, prewarm, sign, send, or write transactions.
func NewCommand() (*cobra.Command, error) {
	cmd, err := cli.NewCommand()
	if err != nil {
		return nil, err
	}
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		filename, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}
		cfg, err := cli.LoadConfig(filename)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return validateEndpoints(cmd.Context(), cfg)
	}
	return cmd, nil
}

func validateEndpoints(ctx context.Context, cfg *config.LoadConfig) error {
	if cfg.ChainID <= 0 {
		return fmt.Errorf("chain preflight: chainId must be positive, got %d", cfg.ChainID)
	}
	for i, endpoint := range cfg.Endpoints {
		if err := validateEndpoint(ctx, endpoint, cfg); err != nil {
			return fmt.Errorf("chain preflight: endpoint %d (%s): %w", i+1, endpoint, err)
		}
	}
	return nil
}

func validateEndpoint(ctx context.Context, endpoint string, cfg *config.LoadConfig) error {
	ctx, cancel := context.WithTimeout(ctx, chainCheckTimeout)
	defer cancel()
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("connect for eth_chainId: %w", err)
	}
	defer client.Close()
	actual, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("eth_chainId: %w", err)
	}
	if actual.Cmp(cfg.GetChainID()) != 0 {
		return fmt.Errorf("eth_chainId mismatch: configured %d, actual %s", cfg.ChainID, actual)
	}
	return nil
}
