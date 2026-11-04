package genutil

// DONTCOVER

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	cfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankexported "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/exported"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

// GenAppStateFromConfig gets the genesis app state from the config
func GenAppStateFromConfig(cdc codec.JSONCodec, txEncodingConfig client.TxEncodingConfig,
	config *cfg.Config, initCfg types.InitConfig, genDoc tmtypes.GenesisDoc, genBalIterator types.GenesisBalancesIterator,
) (appState json.RawMessage, err error) {

	// process genesis transactions, else create default genesis.json
	appGenTxs, persistentPeers, err := CollectTxs(
		cdc, txEncodingConfig.TxJSONDecoder(), config.Moniker, initCfg.GenTxsDir, genDoc, genBalIterator,
	)
	if err != nil {
		return appState, err
	}

	config.P2P.PersistentPeers = persistentPeers
	if err := cfg.WriteConfigFile(config.RootDir, config); err != nil {
		return nil, err
	}

	// if there are no gen txs to be processed, return the default empty state
	if len(appGenTxs) == 0 {
		return appState, errors.New("there must be at least one genesis tx")
	}

	// create the app state
	appGenesisState, err := types.GenesisStateFromGenDoc(genDoc)
	if err != nil {
		return appState, err
	}

	appGenesisState, err = SetGenTxsInAppGenesisState(cdc, txEncodingConfig.TxJSONEncoder(), appGenesisState, appGenTxs)
	if err != nil {
		return appState, err
	}

	appState, err = json.MarshalIndent(appGenesisState, "", "  ")
	if err != nil {
		return appState, err
	}

	genDoc.AppState = appState
	err = ExportGenesisFile(&genDoc, config.GenesisFile())

	return appState, err
}

// CollectTxs processes and validates application's genesis Txs and returns
// the list of appGenTxs, and persistent peers required to generate genesis.json.
func CollectTxs(cdc codec.JSONCodec, txJSONDecoder sdk.TxDecoder, moniker, genTxsDir string,
	genDoc tmtypes.GenesisDoc, genBalIterator types.GenesisBalancesIterator,
) (appGenTxs []sdk.Tx, persistentPeers string, err error) {
	// prepare a map of all balances in genesis state to then validate
	// against the validators addresses
	var appState map[string]json.RawMessage
	if err := json.Unmarshal(genDoc.AppState, &appState); err != nil {
		return appGenTxs, persistentPeers, err
	}

	var fos []os.DirEntry
	fos, err = os.ReadDir(genTxsDir)
	if err != nil {
		return appGenTxs, persistentPeers, err
	}

	balancesMap := make(map[string]bankexported.GenesisBalance)

	genBalIterator.IterateGenesisBalances(
		cdc, appState,
		func(balance bankexported.GenesisBalance) (stop bool) {
			balancesMap[balance.GetAddress().String()] = balance
			return false
		},
	)

	// addresses and IPs (and port) validator server info
	var addressesIPs []string

	for _, fo := range fos {
		if fo.IsDir() {
			continue
		}
		if !strings.HasSuffix(fo.Name(), ".json") {
			continue
		}

		// get the genTx
		jsonRawTx, err := os.ReadFile(filepath.Clean(filepath.Join(genTxsDir, fo.Name())))
		if err != nil {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: %w", fo.Name(), err)
		}

		var genTx sdk.Tx
		if genTx, err = txJSONDecoder(jsonRawTx); err != nil {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: %w", fo.Name(), err)
		}

		if genTx == nil {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: nil genesis transaction", fo.Name())
		}
		msgs := genTx.GetMsgs()
		if len(msgs) != 1 {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: must provide genesis Tx with exactly 1 CreateValidator message, got %d", fo.Name(), len(msgs))
		}
		msg, ok := msgs[0].(*stakingtypes.MsgCreateValidator)
		if !ok || msg == nil {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: expected MsgCreateValidator, got %T", fo.Name(), msgs[0])
		}
		if msg.MinSelfDelegation.IsNil() {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: invalid MsgCreateValidator: missing min_self_delegation", fo.Name())
		}
		if err := validateGenTxCommission(jsonRawTx); err != nil {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: invalid MsgCreateValidator: %w", fo.Name(), err)
		}
		for _, field := range []struct {
			name string
			rate sdk.Dec
		}{
			{"commission.rate", msg.Commission.Rate},
			{"commission.max_rate", msg.Commission.MaxRate},
			{"commission.max_change_rate", msg.Commission.MaxChangeRate},
		} {
			if field.rate.IsNil() {
				return appGenTxs, persistentPeers, fmt.Errorf("%s: invalid MsgCreateValidator: missing %s", fo.Name(), field.name)
			}
		}
		if err := msg.ValidateBasic(); err != nil {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: invalid MsgCreateValidator: %w", fo.Name(), err)
		}
		if err := genTx.ValidateBasic(); err != nil {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: invalid genesis transaction: %w", fo.Name(), err)
		}

		// the memo flag is used to store
		// the ip and node-id, for example this may be:
		// "528fd3df22b31f4969b05652bfe8f0fe921321d5@192.168.2.37:26656"

		memoTx, ok := genTx.(sdk.TxWithMemo)
		if !ok {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: expected TxWithMemo, got %T", fo.Name(), genTx)
		}
		nodeAddrIP := memoTx.GetMemo()
		if len(nodeAddrIP) == 0 {
			return appGenTxs, persistentPeers, fmt.Errorf("failed to find node's address and IP in %s", fo.Name())
		}

		// validate delegator and validator addresses and funds against the accounts in the state
		delAddr := msg.DelegatorAddress
		valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
		if err != nil {
			return appGenTxs, persistentPeers, fmt.Errorf("%s: %w", fo.Name(), err)
		}

		delBal, delOk := balancesMap[delAddr]
		if !delOk {
			_, file, no, ok := runtime.Caller(1)
			if ok {
				fmt.Printf("CollectTxs-1, called from %s#%d\n", file, no)
			}

			return appGenTxs, persistentPeers, fmt.Errorf("%s: account %s balance not in genesis state: %+v", fo.Name(), delAddr, balancesMap)
		}

		_, valOk := balancesMap[sdk.AccAddress(valAddr).String()]
		if !valOk {
			_, file, no, ok := runtime.Caller(1)
			if ok {
				fmt.Printf("CollectTxs-2, called from %s#%d - %s\n", file, no, sdk.AccAddress(msg.ValidatorAddress).String())
			}
			return appGenTxs, persistentPeers, fmt.Errorf("%s: account %s balance not in genesis state: %+v", fo.Name(), valAddr, balancesMap)
		}

		if delBal.GetCoins().AmountOf(msg.Value.Denom).LT(msg.Value.Amount) {
			return appGenTxs, persistentPeers, fmt.Errorf(
				"%s: insufficient fund for delegation %v: %v < %v",
				fo.Name(), delBal.GetAddress().String(), delBal.GetCoins().AmountOf(msg.Value.Denom), msg.Value.Amount,
			)
		}

		appGenTxs = append(appGenTxs, genTx)

		// exclude itself from persistent peers
		if msg.Description.Moniker != moniker {
			addressesIPs = append(addressesIPs, nodeAddrIP)
		}
	}

	sort.Strings(addressesIPs)
	persistentPeers = strings.Join(addressesIPs, ",")

	return appGenTxs, persistentPeers, nil
}

func validateGenTxCommission(encoded []byte) error {
	// Non-nullable protobuf decimals can acquire zero values while unpacking Any.
	// Check the ceremony input before those defaults erase field presence.
	var tx struct {
		Body struct {
			Messages []struct {
				Commission map[string]json.RawMessage `json:"commission"`
			} `json:"messages"`
		} `json:"body"`
	}
	if err := json.Unmarshal(encoded, &tx); err != nil {
		return fmt.Errorf("invalid commission: %w", err)
	}
	if len(tx.Body.Messages) != 1 || tx.Body.Messages[0].Commission == nil {
		return errors.New("missing commission")
	}
	commission := tx.Body.Messages[0].Commission
	for _, field := range []struct{ name, jsonName string }{
		{"rate", "rate"},
		{"max_rate", "maxRate"},
		{"max_change_rate", "maxChangeRate"},
	} {
		raw, ok := commission[field.name]
		if !ok {
			raw = commission[field.jsonName]
		}
		if len(raw) == 0 {
			return fmt.Errorf("missing commission.%s", field.name)
		}
		var rate *sdk.Dec
		if err := json.Unmarshal(raw, &rate); err != nil {
			return fmt.Errorf("invalid commission.%s: %w", field.name, err)
		}
		if rate == nil || rate.IsNil() {
			return fmt.Errorf("missing commission.%s", field.name)
		}
	}
	return nil
}
