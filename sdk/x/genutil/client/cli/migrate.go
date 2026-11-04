package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/version"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
)

const flagGenesisTime = "genesis-time"

var (
	// Applications register their own targets; upstream SDK version migrations
	// are not installed by default in this fork.
	migrationMap   = types.MigrationMap{}
	migrationMutex sync.RWMutex
)

// RegisterMigration registers an application migration before MigrateGenesisCmd
// is constructed. Duplicate targets are rejected to prevent replacing a migration.
func RegisterMigration(version string, callback types.MigrationCallback) error {
	if version == "" || strings.IndexFunc(version, unicode.IsSpace) >= 0 {
		return fmt.Errorf("migration target must be non-empty and contain no whitespace")
	}
	if callback == nil {
		return fmt.Errorf("migration callback for version %s must not be nil", version)
	}

	migrationMutex.Lock()
	defer migrationMutex.Unlock()
	if _, exists := migrationMap[version]; exists {
		return fmt.Errorf("migration already registered for version: %s", version)
	}
	migrationMap[version] = callback
	return nil
}

// GetMigrationCallback returns a MigrationCallback for a given version.
func GetMigrationCallback(version string) types.MigrationCallback {
	migrationMutex.RLock()
	defer migrationMutex.RUnlock()
	return migrationMap[version]
}

// GetMigrationVersions get all migration version in a sorted slice.
func GetMigrationVersions() []string {
	migrationMutex.RLock()
	defer migrationMutex.RUnlock()
	return migrationVersions(migrationMap)
}

func migrationVersions(migrations types.MigrationMap) []string {
	versions := make([]string, 0, len(migrations))
	for version := range migrations {
		versions = append(versions, version)
	}

	sort.Strings(versions)

	return versions
}

// MigrateGenesisCmd returns a command to execute genesis state migration.
func MigrateGenesisCmd() *cobra.Command {
	// Keep the advertised targets and executable callbacks on the same snapshot.
	migrationMutex.RLock()
	migrations := make(types.MigrationMap, len(migrationMap))
	for version, callback := range migrationMap {
		migrations[version] = callback
	}
	migrationMutex.RUnlock()
	versions := migrationVersions(migrations)
	supported := strings.Join(versions, ", ")
	if len(versions) == 0 {
		supported = "none (this application has not registered any genesis migrations)"
	}

	cmd := &cobra.Command{
		Use:   "migrate [target-version] [genesis-file]",
		Short: "Migrate genesis to a specified target version",
		Long: fmt.Sprintf(`Migrate the source genesis into the target version and print to STDOUT.

Supported target versions: %s

Example:
$ %s migrate [target-version] /path/to/genesis.json --chain-id=cosmoshub-3 --genesis-time=2019-04-22T17:00:00Z
`, supported, version.AppName),
		ValidArgs: versions,
		Args:      cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			var err error

			target := args[0]
			importGenesis := args[1]

			genDoc, err := validateGenDoc(importGenesis)
			if err != nil {
				return err
			}

			// Since some default values are valid values, we just print to
			// make sure the user didn't forget to update these values.
			if genDoc.ConsensusParams.Evidence.MaxBytes == 0 {
				fmt.Printf("Warning: consensus_params.evidence.max_bytes is set to 0. If this is"+
					" deliberate, feel free to ignore this warning. If not, please have a look at the chain"+
					" upgrade guide at %s.\n", chainUpgradeGuide)
			}

			var initialState types.AppMap
			if err := json.Unmarshal(genDoc.AppState, &initialState); err != nil {
				return errors.Wrap(err, "failed to JSON unmarshal initial genesis state")
			}

			migrationFunc := migrations[target]
			if migrationFunc == nil {
				return fmt.Errorf("unknown migration function for version: %s; supported target versions: %s", target, supported)
			}

			// TODO: handler error from migrationFunc call
			newGenState := migrationFunc(initialState, clientCtx)

			genDoc.AppState, err = json.Marshal(newGenState)
			if err != nil {
				return errors.Wrap(err, "failed to JSON marshal migrated genesis state")
			}

			genesisTime, _ := cmd.Flags().GetString(flagGenesisTime)
			if genesisTime != "" {
				var t time.Time

				err := t.UnmarshalText([]byte(genesisTime))
				if err != nil {
					return errors.Wrap(err, "failed to unmarshal genesis time")
				}

				genDoc.GenesisTime = t
			}

			chainID, _ := cmd.Flags().GetString(flags.FlagChainID)
			if chainID != "" {
				genDoc.ChainID = chainID
			}

			bz, err := json.Marshal(genDoc)
			if err != nil {
				return errors.Wrap(err, "failed to marshal genesis doc")
			}

			sortedBz, err := sdk.SortJSON(bz)
			if err != nil {
				return errors.Wrap(err, "failed to sort JSON genesis doc")
			}

			cmd.Println(string(sortedBz))
			return nil
		},
	}

	cmd.Flags().String(flagGenesisTime, "", "override genesis_time with this flag")
	cmd.Flags().String(flags.FlagChainID, "", "override chain_id with this flag")

	return cmd
}
