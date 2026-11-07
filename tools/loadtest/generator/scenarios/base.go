package scenarios

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/types"
)

// bigOne is 1 in big.Int.
var bigOne = big.NewInt(1)

// TxGenerator defines the interface for generating transactions.
type TxGenerator interface {
	Name() string
	Generate(scenario *types.TxScenario) *types.LoadTx
	Attach(config *config.LoadConfig, address common.Address) error
	Deploy(config *config.LoadConfig, deployer *types.Account) common.Address
}

// ContextTxGenerator is the error-returning startup and signing interface.
// The legacy TxGenerator interface is kept for callers outside this package.
type ContextTxGenerator interface {
	TxGenerator
	DeployContext(context.Context, *config.LoadConfig, *types.Account) (common.Address, error)
	AttachContext(context.Context, *config.LoadConfig, common.Address) error
	GenerateWithError(*types.TxScenario) (*types.LoadTx, error)
}

// ScenarioDeployer defines the interface for scenario-specific deployment logic
// This can be implemented by both contract and non-contract scenarios
type ScenarioDeployer interface {
	// DeployScenario handles any setup required for the scenario
	// For contracts: deploys the contract and returns its address
	// For non-contracts: performs any initialization and returns zero address
	DeployScenario(config *config.LoadConfig, deployer *types.Account) common.Address

	// AttachScenario connects to an existing contract.
	AttachScenario(config *config.LoadConfig, address common.Address) common.Address

	// CreateTransaction creates a transaction for this scenario
	CreateTransaction(config *config.LoadConfig, scenario *types.TxScenario) (*ethtypes.Transaction, error)
}

// ContractBindFunc defines a function that creates a contract instance from an address
type ContractBindFunc[T any] func(address common.Address, backend bind.ContractBackend) (*T, error)

// ContractDeployer defines the interface for contract-specific deployment logic
// This extends ScenarioDeployer with contract-specific methods
type ContractDeployer[T any] interface {
	ScenarioDeployer

	// DeployContract deploys the contract with specific constructor arguments
	DeployContract(opts *bind.TransactOpts, client *ethclient.Client) (common.Address, *ethtypes.Transaction, error)

	// GetBindFunc returns the function to bind the contract instance
	GetBindFunc() ContractBindFunc[T]

	// SetContract stores the bound contract instance
	SetContract(contract *T)

	// CreateContractTransaction creates a contract interaction transaction
	CreateContractTransaction(auth *bind.TransactOpts, scenario *types.TxScenario) (*ethtypes.Transaction, error)
}

// ScenarioBase provides common functionality for all scenarios
type ScenarioBase struct {
	config   *config.LoadConfig
	deployed bool
	address  common.Address
	deployer ScenarioDeployer

	scenarioConfig config.Scenario
}

// NewScenarioBase creates a new base scenario with the given deployer
func NewScenarioBase(deployer ScenarioDeployer, cfg config.Scenario) *ScenarioBase {
	return &ScenarioBase{
		deployer:       deployer,
		scenarioConfig: cfg,
	}
}

// Deploy handles the common deployment flow
func (s *ScenarioBase) Deploy(config *config.LoadConfig, deployer *types.Account) common.Address {
	address, err := s.DeployContext(context.Background(), config, deployer)
	if err != nil {
		panic(err) // Legacy interface; callers that can return errors use DeployContext.
	}
	return address
}

// DeployContext owns startup cancellation and returns deployment errors to callers.
func (s *ScenarioBase) DeployContext(ctx context.Context, cfg *config.LoadConfig, deployer *types.Account) (common.Address, error) {
	s.deployed = false
	if err := s.admit(ctx, cfg); err != nil {
		return common.Address{}, err
	}
	var address common.Address
	var err error
	if d, ok := s.deployer.(interface {
		DeployScenarioContext(context.Context, *config.LoadConfig, *types.Account) (common.Address, error)
	}); ok {
		address, err = d.DeployScenarioContext(ctx, cfg, deployer)
	} else {
		address = s.deployer.DeployScenario(cfg, deployer)
	}
	if err != nil {
		return common.Address{}, err
	}
	s.config, s.address, s.deployed = cfg, address, true
	return address, nil
}

// Attach connects to an existing contract.
func (s *ScenarioBase) Attach(config *config.LoadConfig, address common.Address) error {
	return s.AttachContext(context.Background(), config, address)
}

// AttachContext validates options before a generator can start signing transactions.
func (s *ScenarioBase) AttachContext(ctx context.Context, cfg *config.LoadConfig, address common.Address) error {
	s.deployed = false
	if err := s.admit(ctx, cfg); err != nil {
		return err
	}
	if d, ok := s.deployer.(interface {
		AttachScenarioContext(context.Context, *config.LoadConfig, common.Address) (common.Address, error)
	}); ok {
		var err error
		address, err = d.AttachScenarioContext(ctx, cfg, address)
		if err != nil {
			return err
		}
	} else {
		address = s.deployer.AttachScenario(cfg, address)
	}
	s.config, s.address, s.deployed = cfg, address, true
	return nil
}

func (s *ScenarioBase) admit(ctx context.Context, cfg *config.LoadConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if cfg == nil || cfg.ChainID <= 0 {
		return fmt.Errorf("a positive chain ID is required")
	}
	_, err := scenarioTransactionOptions(s.scenarioConfig, 200000)
	return err
}

// Generate handles the common transaction generation flow
func (s *ScenarioBase) Generate(scenario *types.TxScenario) *types.LoadTx {
	tx, err := s.GenerateWithError(scenario)
	if err != nil {
		panic(err) // Legacy interface; error-aware generators use GenerateWithError.
	}
	return tx
}

// GenerateWithError preserves signing and picker errors without panicking.
func (s *ScenarioBase) GenerateWithError(scenario *types.TxScenario) (*types.LoadTx, error) {
	if !s.deployed {
		return nil, fmt.Errorf("scenario not deployed/initialized")
	}

	// Create transaction using scenario-specific logic
	tx, err := s.deployer.CreateTransaction(s.config, scenario)
	if err != nil {
		return nil, fmt.Errorf("create scenario transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("scenario returned no transaction")
	}
	return types.CreateTxFromEthTx(tx, scenario), nil
}

// GetConfig returns the configuration
func (s *ScenarioBase) GetConfig() *config.LoadConfig {
	return s.config
}

// GetAddress returns the deployed contract address (zero address for non-contract scenarios)
func (s *ScenarioBase) GetAddress() common.Address {
	return s.address
}

// ContractScenarioBase provides common functionality for contract scenarios
type ContractScenarioBase[T any] struct {
	*ScenarioBase
	deployer ContractDeployer[T]
}

// NewContractScenarioBase creates a new base scenario with the given contract deployer
func NewContractScenarioBase[T any](deployer ContractDeployer[T], cfg config.Scenario) *ContractScenarioBase[T] {
	base := &ContractScenarioBase[T]{
		deployer: deployer,
	}
	base.ScenarioBase = NewScenarioBase(base, cfg)
	return base
}

func dial(ctx context.Context, cfg *config.LoadConfig) (*ethclient.Client, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("deployment requires an RPC endpoint")
	}
	return ethclient.DialContext(ctx, cfg.Endpoints[0])
}

func (c *ContractScenarioBase[T]) bindContract(ctx context.Context, address common.Address, client *ethclient.Client) error {
	contract, err := c.deployer.GetBindFunc()(address, client)
	if err != nil {
		return fmt.Errorf("bind contract: %w", err)
	}
	c.deployer.SetContract(contract)
	if initializer, ok := c.deployer.(interface{ InitializeContract(context.Context) error }); ok {
		if err := initializer.InitializeContract(ctx); err != nil {
			return fmt.Errorf("read contract configuration: %w", err)
		}
	}
	// Signed NoSend calls supply nonce, gas and fees, so bindings need no live
	// backend after startup. Keep no closed client in the production binding.
	return c.bindOffline(address)
}

func (c *ContractScenarioBase[T]) bindOffline(address common.Address) error {
	contract, err := c.deployer.GetBindFunc()(address, nil)
	if err != nil {
		return fmt.Errorf("bind contract: %w", err)
	}
	c.deployer.SetContract(contract)
	return nil
}

func (c *ContractScenarioBase[T]) AttachScenario(cfg *config.LoadConfig, address common.Address) common.Address {
	result, err := c.AttachScenarioContext(context.Background(), cfg, address)
	if err != nil {
		panic(err)
	}
	return result
}

func (c *ContractScenarioBase[T]) AttachScenarioContext(ctx context.Context, cfg *config.LoadConfig, address common.Address) (common.Address, error) {
	if cfg.MockDeploy {
		return address, c.bindOffline(address)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := dial(ctx, cfg)
	if err != nil {
		return common.Address{}, fmt.Errorf("connect to Ethereum client: %w", err)
	}
	defer client.Close()
	if err := c.bindContract(ctx, address, client); err != nil {
		return common.Address{}, err
	}
	return address, nil
}

func (c *ContractScenarioBase[T]) DeployScenario(cfg *config.LoadConfig, deployer *types.Account) common.Address {
	address, err := c.DeployScenarioContext(context.Background(), cfg, deployer)
	if err != nil {
		panic(err)
	}
	return address
}

func (c *ContractScenarioBase[T]) DeployScenarioContext(ctx context.Context, cfg *config.LoadConfig, deployer *types.Account) (common.Address, error) {
	// The deadline exists before dialing, signing, sending, or waiting for mining.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return common.Address{}, err
	}
	client, err := dial(ctx, cfg)
	if err != nil {
		return common.Address{}, fmt.Errorf("connect to Ethereum client: %w", err)
	}
	defer client.Close()
	auth, err := keyedScenarioOpts(cfg.GetChainID(), deployer)
	if err != nil {
		return common.Address{}, fmt.Errorf("create deployment options: %w", err)
	}
	auth.Context, auth.GasLimit, auth.NoSend = ctx, 3000000, false
	address, tx, err := c.deployer.DeployContract(auth, client)
	if err != nil {
		return common.Address{}, fmt.Errorf("deploy contract: %w", err)
	}
	log.Printf("Deployment transaction sent: %s", tx.Hash())
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("wait for deployment: %w", err)
	}
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return common.Address{}, fmt.Errorf("deployment failed with status %d (tx: %s)", receipt.Status, tx.Hash())
	}
	if err := c.bindContract(ctx, address, client); err != nil {
		return common.Address{}, err
	}
	return address, nil
}

// CreateTransaction implements ScenarioDeployer interface for contract scenarios
func (c *ContractScenarioBase[T]) CreateTransaction(config *config.LoadConfig, scenario *types.TxScenario) (*ethtypes.Transaction, error) {
	auth, err := createScenarioTransactionOpts(config.GetChainID(), scenario, c.scenarioConfig, 200000)
	if err != nil {
		return nil, err
	}
	return c.deployer.CreateContractTransaction(auth, scenario)
}

// createScenarioTransactionOpts applies profile selections over scenario defaults
// before the contract binding or transfer signer consumes the options.
func createScenarioTransactionOpts(chainID *big.Int, scenario *types.TxScenario, cfg config.Scenario, defaultGas uint64) (*bind.TransactOpts, error) {
	options, err := scenarioTransactionOptions(cfg, defaultGas)
	if err != nil {
		return nil, err
	}
	if scenario == nil {
		return nil, fmt.Errorf("transaction scenario is required")
	}
	auth, err := keyedScenarioOpts(chainID, scenario.Sender)
	if err != nil {
		return nil, err
	}
	auth.GasLimit, auth.GasTipCap, auth.GasFeeCap = options.GasLimit, options.GasTipCap, options.GasFeeCap
	return auth, nil
}

func keyedScenarioOpts(chainID *big.Int, account *types.Account) (*bind.TransactOpts, error) {
	if chainID == nil || chainID.Sign() <= 0 {
		return nil, fmt.Errorf("a positive chain ID is required")
	}
	if account == nil || account.PrivKey == nil {
		return nil, fmt.Errorf("sender private key is required")
	}
	auth, err := bind.NewKeyedTransactorWithChainID(account.PrivKey, chainID)
	if err != nil {
		return nil, err
	}
	if auth.From != account.Address {
		return nil, fmt.Errorf("sender address does not match private key")
	}
	auth.Nonce = new(big.Int).SetUint64(account.GetAndIncrementNonce())
	auth.NoSend = true
	auth.GasTipCap = new(big.Int).SetUint64(2000000000)
	auth.GasFeeCap = new(big.Int).SetUint64(20000000000)
	return auth, nil
}

func scenarioTransactionOptions(cfg config.Scenario, defaultGas uint64) (*bind.TransactOpts, error) {
	auth := &bind.TransactOpts{GasLimit: defaultGas, GasTipCap: new(big.Int).SetUint64(2000000000), GasFeeCap: new(big.Int).SetUint64(20000000000)}
	for _, selection := range []struct {
		name   string
		picker *config.GasPicker
		apply  func(uint64)
	}{
		{"gasPicker", cfg.GasPicker, func(value uint64) { auth.GasLimit = value }},
		{"gasTipCapPicker", cfg.GasTipCapPicker, func(value uint64) { auth.GasTipCap = new(big.Int).SetUint64(value) }},
		{"gasFeeCapPicker", cfg.GasFeeCapPicker, func(value uint64) { auth.GasFeeCap = new(big.Int).SetUint64(value) }},
	} {
		if selection.picker == nil {
			continue
		}
		if selection.picker.Name() != "fixed" && selection.picker.Name() != "random" {
			return nil, fmt.Errorf("%s: a fixed or random picker is required", selection.name)
		}
		value, err := selection.picker.GenerateGas()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", selection.name, err)
		}
		selection.apply(value)
	}
	if auth.GasLimit == 0 {
		return nil, fmt.Errorf("gasPicker: gas limit must be greater than zero")
	}
	if auth.GasFeeCap.Cmp(auth.GasTipCap) < 0 {
		return nil, fmt.Errorf("gasFeeCapPicker: fee cap must be at least the gasTipCapPicker tip cap")
	}
	return auth, nil
}
