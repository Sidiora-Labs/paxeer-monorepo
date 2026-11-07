package genutil

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	cfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmos "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/os"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/cosmos/go-bip39"

	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
)

// ExportGenesisFile creates and writes the genesis configuration to disk. An
// error is returned if building or writing the configuration to file fails.
func ExportGenesisFile(genDoc *tmtypes.GenesisDoc, genFile string) error {
	if err := genDoc.ValidateAndComplete(); err != nil {
		return err
	}

	return genDoc.SaveAs(genFile)
}

// ExportGenesisFileWithTime creates and writes the genesis configuration to disk.
// An error is returned if building or writing the configuration to file fails.
func ExportGenesisFileWithTime(
	genFile, chainID string, validators []tmtypes.GenesisValidator,
	appState json.RawMessage, genTime time.Time,
) error {

	genDoc := tmtypes.GenesisDoc{
		GenesisTime: genTime,
		ChainID:     chainID,
		Validators:  validators,
		AppState:    appState,
	}

	if err := genDoc.ValidateAndComplete(); err != nil {
		return err
	}

	return genDoc.SaveAs(genFile)
}

// InitializeNodeValidatorFiles creates private validator and p2p configuration files.
func InitializeNodeValidatorFiles(config *cfg.Config) (nodeID string, valPubKey cryptotypes.PubKey, err error) {
	return InitializeNodeValidatorFilesFromMnemonic(config, "")
}

// InitializeNodeValidatorFiles creates private validator and p2p configuration files using the given mnemonic.
// If no valid mnemonic is given, a random one will be used instead.
func InitializeNodeValidatorFilesFromMnemonic(config *cfg.Config, mnemonic string) (nodeID string, valPubKey cryptotypes.PubKey, err error) {
	if len(mnemonic) > 0 && !bip39.IsMnemonicValid(mnemonic) {
		return "", nil, fmt.Errorf("invalid mnemonic")
	}

	nodeKey, err := config.LoadOrGenNodeKeyID()
	if err != nil {
		return "", nil, err
	}

	nodeID = string(nodeKey)

	pvKeyFile := config.PrivValidator.KeyFile()
	if err := tmos.EnsureDir(filepath.Dir(pvKeyFile), 0777); err != nil {
		return "", nil, err
	}

	pvStateFile := config.PrivValidator.StateFile()
	if err := tmos.EnsureDir(filepath.Dir(pvStateFile), 0777); err != nil {
		return "", nil, err
	}

	var filePV *privval.FilePV
	if len(mnemonic) == 0 {
		filePV, err = privval.LoadOrGenFilePV(pvKeyFile, pvStateFile)
		if err != nil {
			return "", nil, err
		}
	} else {
		privKey := tmed25519.TestSecretKey([]byte(mnemonic))
		_, keyErr := os.Lstat(pvKeyFile)
		if keyErr != nil && !os.IsNotExist(keyErr) {
			return "", nil, keyErr
		}
		_, stateErr := os.Lstat(pvStateFile)
		if stateErr != nil && !os.IsNotExist(stateErr) {
			return "", nil, stateErr
		}
		switch {
		case keyErr == nil && stateErr == nil:
			filePV, err = privval.LoadFilePV(pvKeyFile, pvStateFile)
			if err != nil {
				return "", nil, err
			}
			if filePV.Key.PubKey.Compare(privKey.Public()) != 0 {
				return "", nil, fmt.Errorf("mnemonic does not match existing validator key")
			}
		case os.IsNotExist(keyErr) && os.IsNotExist(stateErr):
			filePV = privval.NewFilePV(privKey, pvKeyFile, pvStateFile)
			if err := filePV.Save(); err != nil {
				return "", nil, err
			}
		default:
			return "", nil, fmt.Errorf("incomplete validator files: restore both key and signing state before mnemonic recovery")
		}
	}

	tmValPubKey, err := filePV.GetPubKey(context.Background())
	if err != nil {
		return "", nil, err
	}

	valPubKey, err = cryptocodec.FromTmPubKeyInterface(tmValPubKey)
	if err != nil {
		return "", nil, err
	}

	return nodeID, valPubKey, nil
}
