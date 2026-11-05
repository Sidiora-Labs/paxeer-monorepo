package tasks

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	tmcfg "github.com/sei-protocol/sei-chain/sei-tendermint/config"
	tmtypes "github.com/sei-protocol/sei-chain/sei-tendermint/types"

	"github.com/sei-protocol/sei-chain/sei-cosmos/client"
	"github.com/sei-protocol/sei-chain/sei-cosmos/client/tx"
	"github.com/sei-protocol/sei-chain/sei-cosmos/codec"
	"github.com/sei-protocol/sei-chain/sei-cosmos/codec/legacy"
	codectypes "github.com/sei-protocol/sei-chain/sei-cosmos/codec/types"
	cryptocodec "github.com/sei-protocol/sei-chain/sei-cosmos/crypto/codec"
	"github.com/sei-protocol/sei-chain/sei-cosmos/crypto/hd"
	"github.com/sei-protocol/sei-chain/sei-cosmos/crypto/keyring"
	sdk "github.com/sei-protocol/sei-chain/sei-cosmos/types"
	authclient "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/client"
	authtx "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/tx"
	authtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/types"
	vestingtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/vesting/types"
	banktypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/bank/types"
	stakingtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/staking/types"

	"github.com/sei-protocol/sei-chain/sei-cosmos/x/genutil"
	genutiltypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/genutil/types"
	stakingcli "github.com/sei-protocol/sei-chain/sei-cosmos/x/staking/client/cli"

	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seilog"
)

var gentxLog = seilog.NewLogger("seictl", "task", "generate-gentx")

const (
	gentxMarkerFile  = ".sei-sidecar-gentx-done"
	validatorKeyName = "validator"
)

// GenerateGentxRequest holds the typed parameters for the generate-gentx task.
type GenerateGentxRequest struct {
	ChainID        string `json:"chainId"`
	StakingAmount  string `json:"stakingAmount"`
	AccountBalance string `json:"accountBalance"`
}

// GentxGenerator produces a genesis transaction by calling the same SDK
// functions as seid keys add -> seid add-genesis-account -> seid gentx.
type GentxGenerator struct {
	homeDir string
}

// NewGentxGenerator creates a generator targeting the given home directory.
func NewGentxGenerator(homeDir string) *GentxGenerator {
	return &GentxGenerator{homeDir: homeDir}
}

// Handler returns an engine.TaskHandler for the generate-gentx task type.
//
// Expected params:
//
//	{
//	  "chainId":        "my-chain",
//	  "stakingAmount":  "1000000usei",
//	  "accountBalance": "10000000usei"
//	}
func (g *GentxGenerator) Handler() engine.TaskHandler {
	return engine.TypedHandler(func(ctx context.Context, params GenerateGentxRequest) error {

		if params.ChainID == "" {
			return fmt.Errorf("generate-gentx: missing required param 'chainId'")
		}
		if params.StakingAmount == "" {
			return fmt.Errorf("generate-gentx: missing required param 'stakingAmount'")
		}
		if params.AccountBalance == "" {
			return fmt.Errorf("generate-gentx: missing required param 'accountBalance'")
		}

		unlock, err := lockRecovery(ctx, g.homeDir, ".sei-sidecar-gentx-lock")
		if err != nil {
			return err
		}
		defer unlock()
		return g.resumeGentx(params)

	})
}

// addValidatorKey creates a local key and returns its bech32 address.
// Same path as: seid keys add validator --keyring-backend test
//
// Genesis ceremonies use throwaway keys; the production-keyring backend
// configured via SEI_KEYRING_BACKEND is intentionally not consumed here.
// Mixing the two would let a gentx run mutate an operator's production
// keyring (or surface its passphrase prompt) — both unacceptable.
func (g *GentxGenerator) addValidatorKey(cdc codec.Codec) (string, error) {
	gentxLog.Info("creating validator key")

	kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, g.homeDir, os.Stdin)
	if err != nil {
		return "", fmt.Errorf("generate-gentx: creating keyring: %w", err)
	}

	info, _, err := kb.NewMnemonic(
		validatorKeyName,
		keyring.English,
		sdk.GetConfig().GetFullBIP44Path(),
		"",
		hd.Secp256k1,
	)
	if err != nil {
		return "", fmt.Errorf("generate-gentx: keys add: %w", err)
	}

	addr := info.GetAddress().String()
	gentxLog.Info("validator key created", "address", addr)
	return addr, nil
}

// addGenesisAccount adds the validator's account and balance to genesis.
// Same path as: seid add-genesis-account <addr> <balance>
func (g *GentxGenerator) addGenesisAccount(cdc codec.Codec, address, balance string) error {
	gentxLog.Info("adding genesis account", "address", address, "balance", balance)

	addr, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		return fmt.Errorf("generate-gentx: parsing address: %w", err)
	}

	coins, err := sdk.ParseCoinsNormalized(balance)
	if err != nil {
		return fmt.Errorf("generate-gentx: parsing balance: %w", err)
	}

	genFile := filepath.Join(g.homeDir, "config", "genesis.json")
	appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
	if err != nil {
		return fmt.Errorf("generate-gentx: reading genesis: %w", err)
	}

	// auth module: add account (same as seid add-genesis-account)
	authGenState := authtypes.GetGenesisStateFromAppState(cdc, appState)
	accs, err := authtypes.UnpackAccounts(authGenState.Accounts)
	if err != nil {
		return fmt.Errorf("generate-gentx: unpacking accounts: %w", err)
	}

	if accs.Contains(addr) {
		return fmt.Errorf("generate-gentx: account already exists at %s", addr)
	}

	accs = append(accs, authtypes.NewBaseAccount(addr, nil, 0, 0))
	accs = authtypes.SanitizeGenesisAccounts(accs)

	genAccs, err := authtypes.PackAccounts(accs)
	if err != nil {
		return fmt.Errorf("generate-gentx: packing accounts: %w", err)
	}
	authGenState.Accounts = genAccs

	authStateBz, err := cdc.MarshalAsJSON(&authGenState)
	if err != nil {
		return fmt.Errorf("generate-gentx: marshaling auth state: %w", err)
	}
	appState[authtypes.ModuleName] = authStateBz

	// bank module: add balance (same as seid add-genesis-account)
	bankGenState := banktypes.GetGenesisStateFromAppState(cdc, appState)
	bankGenState.Balances = append(bankGenState.Balances, banktypes.Balance{
		Address: addr.String(),
		Coins:   coins.Sort(),
	})
	bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)

	bankStateBz, err := cdc.MarshalAsJSON(bankGenState)
	if err != nil {
		return fmt.Errorf("generate-gentx: marshaling bank state: %w", err)
	}
	appState[banktypes.ModuleName] = bankStateBz

	// evm module: associate sei address with eth address.
	// Same as seid add-genesis-account when --keyring-backend=test.
	if err := g.addEVMAddressAssociation(cdc, appState, addr); err != nil {
		return fmt.Errorf("generate-gentx: adding EVM association: %w", err)
	}

	appStateJSON, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("generate-gentx: marshaling app state: %w", err)
	}
	genDoc.AppState = appStateJSON

	return genutil.ExportGenesisFile(genDoc, genFile)
}

// generateGentx builds and signs a MsgCreateValidator.
// Same path as: seid gentx validator <amount> --chain-id <chain>
func (g *GentxGenerator) buildGentx(cdc codec.Codec, txCfg client.TxConfig, chainID, stakingAmount string) (string, []byte, error) {
	gentxLog.Info("generating gentx", "chainId", chainID, "stakingAmount", stakingAmount)

	cfg := tmcfg.DefaultConfig()
	cfg.SetRoot(g.homeDir)

	nodeID, valPubKey, err := genutil.InitializeNodeValidatorFiles(cfg)
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: loading validator files: %w", err)
	}

	kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, g.homeDir, os.Stdin)
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: opening keyring: %w", err)
	}

	keyInfo, err := kb.Key(validatorKeyName)
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: looking up key: %w", err)
	}

	// Read and validate genesis
	genDoc, err := tmtypes.GenesisDocFromFile(cfg.GenesisFile())
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: reading genesis: %w", err)
	}

	var genesisState map[string]json.RawMessage
	if err := json.Unmarshal(genDoc.AppState, &genesisState); err != nil {
		return "", nil, fmt.Errorf("generate-gentx: parsing app_state: %w", err)
	}

	// Validate account has sufficient balance (same check seid gentx does)
	coins, err := sdk.ParseCoinsNormalized(stakingAmount)
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: parsing staking amount: %w", err)
	}

	genBalIterator := banktypes.GenesisBalancesIterator{}
	if err := genutil.ValidateAccountInGenesis(
		genesisState, genBalIterator, keyInfo.GetAddress(), coins, cdc,
	); err != nil {
		return "", nil, fmt.Errorf("generate-gentx: %w", err)
	}

	// Peer discovery uses controller-published DNS peers. A fixed loopback
	// memo avoids host-interface discovery changing signed bytes on restart.
	ip := "127.0.0.1"

	// Build MsgCreateValidator (same struct seid gentx populates)
	createValCfg := stakingcli.TxCreateValidatorConfig{
		ChainID:                 chainID,
		NodeID:                  nodeID,
		Moniker:                 cfg.Moniker,
		Amount:                  stakingAmount,
		CommissionRate:          "0.1",
		CommissionMaxRate:       "0.2",
		CommissionMaxChangeRate: "0.01",
		MinSelfDelegation:       "1",
		PubKey:                  valPubKey,
		IP:                      ip,
		P2PPort:                 "26656",
	}

	clientCtx := client.Context{}.
		WithKeyring(kb).
		WithCodec(cdc).
		WithTxConfig(txCfg).
		WithFromAddress(keyInfo.GetAddress())

	txFactory := tx.Factory{}.
		WithChainID(chainID).
		WithKeybase(kb).
		WithTxConfig(txCfg)

	txBldr, msg, err := stakingcli.BuildCreateValidatorMsg(clientCtx, createValCfg, txFactory, true)
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: building MsgCreateValidator: %w", err)
	}

	if err := msg.ValidateBasic(); err != nil {
		return "", nil, fmt.Errorf("generate-gentx: invalid MsgCreateValidator: %w", err)
	}

	// Round-trip through the codec — same flow as seid gentx
	w := &bytes.Buffer{}
	clientCtx = clientCtx.WithOutput(w)

	if err := authclient.PrintUnsignedStdTx(txBldr, clientCtx, []sdk.Msg{msg}); err != nil {
		return "", nil, fmt.Errorf("generate-gentx: generating unsigned tx: %w", err)
	}

	stdTx, err := txCfg.TxJSONDecoder()(w.Bytes())
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: decoding unsigned tx: %w", err)
	}

	txBuilder, err := txCfg.WrapTxBuilder(stdTx)
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: wrapping tx builder: %w", err)
	}

	if err := authclient.SignTx(txFactory, clientCtx, validatorKeyName, txBuilder, true, true); err != nil {
		return "", nil, fmt.Errorf("generate-gentx: signing: %w", err)
	}

	signedJSON, err := txCfg.TxJSONEncoder()(txBuilder.GetTx())
	if err != nil {
		return "", nil, fmt.Errorf("generate-gentx: encoding signed tx: %w", err)
	}

	return fmt.Sprintf("gentx-%s.json", nodeID), append(signedJSON, '\n'), nil
}

func (g *GentxGenerator) generateGentx(cdc codec.Codec, txCfg client.TxConfig, chainID, stakingAmount string) error {
	name, data, err := g.buildGentx(cdc, txCfg, chainID, stakingAmount)
	if err != nil {
		return err
	}
	dir := filepath.Join(g.homeDir, "config", "gentx")
	if err := ensureRecoveryDir(dir); err != nil {
		return err
	}
	return durableJSONFile(dir, name, data)
}

// addEVMAddressAssociation derives the Ethereum address from the
// validator's secp256k1 private key and writes it into the EVM module's
// genesis state. This mirrors seid add-genesis-account lines 136-148.
//
// We avoid importing x/evm (which transitively pulls in wasmvm/duckdb
// and breaks CGO_ENABLED=0) by operating on the genesis JSON directly.
// The result is identical: an AddressAssociation entry is appended.
func (g *GentxGenerator) addEVMAddressAssociation(cdc codec.Codec, appState map[string]json.RawMessage, addr sdk.AccAddress) error {
	evmRaw, ok := appState["evm"]
	if !ok {
		// No evm module in genesis — skip silently.
		return nil
	}

	kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, g.homeDir, nil)
	if err != nil {
		return err
	}

	pk, err := getPrivateKeyOfAddr(kb, addr)
	if err != nil {
		return fmt.Errorf("deriving ETH address for EVM association: %w", err)
	}

	ethAddr := ethcrypto.PubkeyToAddress(pk.PublicKey)

	// Unmarshal the evm genesis state as generic JSON, append the
	// association, and marshal back. This avoids importing x/evm/types
	// which pulls in wasmvm via its transitive dependency chain.
	var evmState map[string]json.RawMessage
	if err := json.Unmarshal(evmRaw, &evmState); err != nil {
		return fmt.Errorf("parsing evm genesis state: %w", err)
	}

	var associations []json.RawMessage
	if raw, ok := evmState["address_associations"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &associations); err != nil {
			return fmt.Errorf("parsing evm address_associations: %w", err)
		}
	}

	entry, _ := json.Marshal(map[string]string{
		"sei_address": addr.String(),
		"eth_address": ethAddr.Hex(),
	})
	associations = append(associations, entry)

	evmState["address_associations"], _ = json.Marshal(associations)
	appState["evm"], _ = json.Marshal(evmState)
	return nil
}

// getPrivateKeyOfAddr extracts the secp256k1 private key for the given
// address from the test keyring. Mirrors seid's getPrivateKeyOfAddr
// in cmd/seid/cmd/genaccounts.go:212-238.
func getPrivateKeyOfAddr(kb keyring.Keyring, addr sdk.Address) (*ecdsa.PrivateKey, error) {
	keys, err := kb.List()
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		localInfo, ok := key.(keyring.LocalInfo)
		if !ok {
			continue
		}
		if localInfo.GetAddress().Equals(addr) {
			priv, err := legacy.PrivKeyFromBytes([]byte(localInfo.PrivKeyArmor))
			if err != nil {
				return nil, err
			}
			privKey, err := ethcrypto.HexToECDSA(hex.EncodeToString(priv.Bytes()))
			if err != nil {
				return nil, err
			}
			return privKey, nil
		}
	}
	return nil, fmt.Errorf("key not found for address %s", addr)
}

// makeCodec builds a proto codec with the minimum interface types
// registered for genesis ceremony operations. vestingtypes is required to
// marshal/unmarshal a ContinuousVestingAccount or DelayedVestingAccount —
// addExternalGenesisAccounts can produce either when a GenesisAccountEntry
// sets Vesting, so this codec must resolve their type URLs even though no
// gentx itself is ever a vesting account.
func makeCodec() (codec.Codec, client.TxConfig) {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	vestingtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	return cdc, txConfig
}

const gentxOperationFile = ".sei-sidecar-gentx-operation.json"
const gentxStageDir = ".sei-sidecar-gentx-stage"

// This private, mode-0600 journal has the same trust boundary as the test
// keyring. Persisting the seed before key creation closes the key-write crash
// window; it is never included in public ceremony artifacts.
type gentxOperation struct {
	Request   GenerateGentxRequest `json:"request"`
	Mnemonic  string               `json:"mnemonic"`
	Address   string               `json:"address"`
	Input     []byte               `json:"input"`
	Account   []byte               `json:"account,omitempty"`
	GentxName string               `json:"gentxName,omitempty"`
	Signed    []byte               `json:"signed,omitempty"`
}

func (g *GentxGenerator) resumeGentx(params GenerateGentxRequest) error {
	cdc, txCfg := makeCodec()
	ensureBech32()
	kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, g.homeDir, nil)
	if err != nil {
		return err
	}
	var op gentxOperation
	data, err := readRecoveryFile(g.homeDir, gentxOperationFile)
	if errors.Is(err, os.ErrNotExist) {
		// Never adopt a pre-existing key, account, gentx or legacy marker.
		keys, err := kb.List()
		if err != nil {
			return err
		}
		for _, key := range keys {
			if key.GetName() == validatorKeyName {
				return fmt.Errorf("generate-gentx: unbound validator key already exists")
			}
		}
		if markerExists(g.homeDir, gentxMarkerFile) {
			return fmt.Errorf("generate-gentx: unbound completion marker")
		}
		files, err := os.ReadDir(filepath.Join(g.homeDir, "config", "gentx"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if len(files) > 0 {
			return fmt.Errorf("generate-gentx: unbound gentx already exists")
		}
		input, err := os.ReadFile(filepath.Join(g.homeDir, "config", "genesis.json"))
		if err != nil {
			return err
		}
		doc, err := tmtypes.GenesisDocFromJSON(input)
		if err != nil {
			return err
		}
		if doc.ChainID != params.ChainID {
			return fmt.Errorf("generate-gentx: genesis chain does not match request")
		}
		memory := keyring.NewInMemory()
		info, mnemonic, err := memory.NewMnemonic(validatorKeyName, keyring.English, sdk.GetConfig().GetFullBIP44Path(), "", hd.Secp256k1)
		if err != nil {
			return err
		}
		op = gentxOperation{Request: params, Mnemonic: mnemonic, Address: info.GetAddress().String(), Input: input}
		if err := saveRecovery(g.homeDir, gentxOperationFile, op); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err := json.Unmarshal(data, &op); err != nil {
		return fmt.Errorf("generate-gentx: corrupt operation: %w", err)
	}
	if op.Request != params || op.Mnemonic == "" || len(op.Input) == 0 {
		return fmt.Errorf("generate-gentx: operation does not match request")
	}
	expected, err := keyring.NewInMemory().NewAccount(validatorKeyName, op.Mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
	if err != nil {
		return err
	}
	if expected.GetAddress().String() != op.Address {
		return fmt.Errorf("generate-gentx: operation key identity mismatch")
	}
	keys, err := kb.List()
	if err != nil {
		return err
	}
	found := false
	for _, key := range keys {
		if key.GetName() != validatorKeyName {
			continue
		}
		if !key.GetPubKey().Equals(expected.GetPubKey()) {
			return fmt.Errorf("generate-gentx: existing key does not match operation")
		}
		found = true
	}
	if !found {
		if _, err := kb.NewAccount(validatorKeyName, op.Mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1); err != nil {
			return err
		}
	}
	if len(op.Signed) > 0 {
		completion, completionErr := readRecoveryFile(g.homeDir, gentxMarkerFile)
		if completionErr == nil {
			var done map[string]string
			if err := json.Unmarshal(completion, &done); err != nil {
				return fmt.Errorf("generate-gentx: corrupt completion: %w", err)
			}
			if !plainFilename(op.GentxName) || done["requestHash"] != recoveryHash(mustGentxRequest(params)) || done["gentxHash"] != recoveryHash(op.Signed) {
				return fmt.Errorf("generate-gentx: completion does not match operation")
			}
			signed, err := readRecoveryFile(filepath.Join(g.homeDir, "config", "gentx"), op.GentxName)
			if err != nil {
				return err
			}
			if !bytes.Equal(signed, op.Signed) {
				return fmt.Errorf("generate-gentx: completed signed gentx does not match operation")
			}
			return nil
		}
		if !errors.Is(completionErr, os.ErrNotExist) {
			return completionErr
		}
	}
	if len(op.Account) == 0 {
		stage := GentxGenerator{homeDir: filepath.Join(g.homeDir, gentxStageDir)}
		stageConfig := filepath.Join(stage.homeDir, "config")
		if err := ensureRecoveryDir(stageConfig); err != nil {
			return err
		}
		if err := durableJSONFile(stageConfig, "genesis.json", op.Input); err != nil {
			return err
		}
		stageKB, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, stage.homeDir, nil)
		if err != nil {
			return err
		}
		stageKeys, err := stageKB.List()
		if err != nil {
			return err
		}
		stageFound := false
		for _, key := range stageKeys {
			if key.GetName() != validatorKeyName {
				continue
			}
			if !key.GetPubKey().Equals(expected.GetPubKey()) {
				return fmt.Errorf("generate-gentx: staged key does not match operation")
			}
			stageFound = true
		}
		if !stageFound {
			if _, err := stageKB.NewAccount(validatorKeyName, op.Mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1); err != nil {
				return err
			}
		}
		if err := stage.addGenesisAccount(cdc, op.Address, params.AccountBalance); err != nil {
			return err
		}
		op.Account, err = os.ReadFile(filepath.Join(stageConfig, "genesis.json"))
		if err != nil {
			return err
		}
		if err := saveRecovery(g.homeDir, gentxOperationFile, op); err != nil {
			return err
		}
	}
	current, err := os.ReadFile(filepath.Join(g.homeDir, "config", "genesis.json"))
	if err != nil {
		return err
	}
	if !bytes.Equal(current, op.Account) {
		if !bytes.Equal(current, op.Input) {
			return fmt.Errorf("generate-gentx: genesis does not match operation input or account stage")
		}
		if err := durableJSONFile(filepath.Join(g.homeDir, "config"), "genesis.json", op.Account); err != nil {
			return err
		}
	}
	if len(op.Signed) == 0 {
		op.GentxName, op.Signed, err = g.buildGentx(cdc, txCfg, params.ChainID, params.StakingAmount)
		if err != nil {
			return err
		}
		if err := saveRecovery(g.homeDir, gentxOperationFile, op); err != nil {
			return err
		}
	}
	if !plainFilename(op.GentxName) {
		return fmt.Errorf("generate-gentx: invalid staged gentx name")
	}
	gentxDir := filepath.Join(g.homeDir, "config", "gentx")
	if err := ensureRecoveryDir(gentxDir); err != nil {
		return err
	}
	existing, err := readRecoveryFile(gentxDir, op.GentxName)
	if err == nil && !bytes.Equal(existing, op.Signed) {
		return fmt.Errorf("generate-gentx: signed gentx does not match operation")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := durableJSONFile(gentxDir, op.GentxName, op.Signed); err != nil {
		return err
	}
	return saveRecovery(g.homeDir, gentxMarkerFile, map[string]string{"requestHash": recoveryHash(mustGentxRequest(params)), "gentxHash": recoveryHash(op.Signed)})
}

func mustGentxRequest(params GenerateGentxRequest) []byte {
	data, _ := json.Marshal(params)
	return data
}
