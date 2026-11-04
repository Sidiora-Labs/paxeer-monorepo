package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
)

const (
	chainUpgradeGuide = "https://docs.cosmos.network/master/migrations/chain-upgrade-guide-040.html"
	flagStreaming     = "streaming"
)

// ValidateGenesisCmd takes a genesis file, and makes sure that it is valid.
func ValidateGenesisCmd(mbm module.BasicManager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-genesis [file]",
		Args:  cobra.RangeArgs(0, 1),
		Short: "validates the genesis file at the default location or at the location passed as an arg",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			serverCtx := server.GetServerContextFromCmd(cmd)
			clientCtx := client.GetClientContextFromCmd(cmd)

			cdc := clientCtx.Codec

			isStream, err := cmd.Flags().GetBool(flagStreaming)
			if err != nil {
				panic(err)
			}

			if isStream {
				return validateGenesisStream(mbm, cmd, args)
			}

			// Load default if passed no args, otherwise load passed file
			var genesis string
			if len(args) == 0 {
				genesis = serverCtx.Config.GenesisFile()
			} else {
				genesis = args[0]
			}

			genDoc, err := validateGenDoc(genesis)
			if err != nil {
				return err
			}

			var genState map[string]json.RawMessage
			if err = json.Unmarshal(genDoc.AppState, &genState); err != nil {
				return fmt.Errorf("error unmarshalling genesis doc %s: %s", genesis, err.Error())
			}

			if err = mbm.ValidateGenesis(cdc, clientCtx.TxConfig, genState); err != nil {
				return fmt.Errorf("error validating genesis file %s: %s", genesis, err.Error())
			}

			fmt.Printf("File at %s is a valid genesis file\n", genesis)
			return nil
		},
	}
	cmd.Flags().Bool(flagStreaming, false, "turn on streaming mode with this flag")
	return cmd
}

type AppState struct {
	Module string          `json:"module"`
	Data   json.RawMessage `json:"data"`
}

type ModuleState struct {
	AppState AppState `json:"app_state"`
}

func parseModule(jsonStr string) (*ModuleState, error) {
	var module ModuleState
	err := json.Unmarshal([]byte(jsonStr), &module)
	if err != nil {
		return nil, err
	}
	if module.AppState.Module == "" {
		return nil, fmt.Errorf("module name is empty")
	}
	return &module, nil
}

func validateGenesisStream(mbm module.BasicManager, cmd *cobra.Command, args []string) (err error) {
	serverCtx := server.GetServerContextFromCmd(cmd)
	clientCtx := client.GetClientContextFromCmd(cmd)

	var genesis string
	if len(args) == 0 {
		genesis = serverCtx.Config.GenesisFile()
	} else {
		genesis = args[0]
	}

	file, err := os.Open(filepath.Clean(genesis))
	if err != nil {
		return fmt.Errorf("error opening genesis file %s: %w", genesis, err)
	}
	defer func() { _ = file.Close() }()

	var genesisCh chan json.RawMessage
	var resultCh chan error
	finishModule := func() error {
		if genesisCh == nil {
			return nil
		}
		close(genesisCh)
		validationErr := <-resultCh
		genesisCh = nil
		resultCh = nil
		return validationErr
	}
	defer func() {
		if validationErr := finishModule(); err == nil {
			err = validationErr
		}
	}()

	seenModules := make(map[string]bool)
	prevModule := ""
	headerValidated := false
	reader := bufio.NewReader(file)
	for {
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		line, readErr := reader.ReadBytes('\n')
		if readErr != nil && readErr != io.EOF {
			return fmt.Errorf("error reading genesis file %s: %w", genesis, readErr)
		}
		if len(line) > 0 {
			moduleState, parseErr := parseModule(string(line))
			if parseErr != nil {
				if _, err := tmtypes.GenesisDocFromJSON(line); err != nil {
					return fmt.Errorf("error validating genesis doc %s: %w", genesis, err)
				}
				if headerValidated {
					return fmt.Errorf("module genesisDoc seen twice in genesis file")
				}
				if err := finishModule(); err != nil {
					return err
				}
				headerValidated = true
				prevModule = ""
			} else {
				moduleName := moduleState.AppState.Module
				if prevModule != moduleName {
					if err := finishModule(); err != nil {
						return err
					}
					if seenModules[moduleName] {
						return fmt.Errorf("module %s seen twice in genesis file", moduleName)
					}
					seenModules[moduleName] = true
					prevModule = moduleName
					if validator, ok := mbm[moduleName]; ok {
						genesisCh = make(chan json.RawMessage)
						resultCh = make(chan error, 1)
						go func(ch <-chan json.RawMessage, result chan<- error) {
							result <- validator.ValidateGenesisStream(clientCtx.Codec, clientCtx.TxConfig, ch)
						}(genesisCh, resultCh)
					}
				}
				if genesisCh != nil {
					select {
					case genesisCh <- moduleState.AppState.Data:
					case validationErr := <-resultCh:
						close(genesisCh)
						genesisCh = nil
						resultCh = nil
						if validationErr == nil {
							return fmt.Errorf("module %s validator stopped before input completed", moduleName)
						}
						return validationErr
					case <-cmd.Context().Done():
						return cmd.Context().Err()
					}
				}
			}
		}
		if readErr == io.EOF {
			break
		}
	}
	if !headerValidated {
		return fmt.Errorf("genesis file %s is missing a valid genesis header", genesis)
	}
	if err := finishModule(); err != nil {
		return err
	}
	if err := cmd.Context().Err(); err != nil {
		return err
	}
	cmd.Printf("File at %s is a valid genesis file\n", genesis)
	return nil
}

// validateGenDoc reads a genesis file and validates that it is a correct
// Tendermint GenesisDoc. This function does not do any cosmos-related
// validation.
func validateGenDoc(importGenesisFile string) (*tmtypes.GenesisDoc, error) {
	genDoc, err := tmtypes.GenesisDocFromFile(importGenesisFile)
	if err != nil {
		return nil, fmt.Errorf("%s. Make sure that"+
			" you have correctly migrated all Tendermint consensus params, please see the"+
			" chain migration guide at %s for more info",
			err.Error(), chainUpgradeGuide,
		)
	}

	return genDoc, nil
}
