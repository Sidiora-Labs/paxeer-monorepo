package scenarios

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator/bindings"
	"github.com/paxeer-network/pax-load/types"
)

const Disperse = "disperse"

const DisperseBatchSize = 4

// A fresh recipient costs 25000 for account creation, 9000 for value transfer,
// plus cold access and loop/calldata overhead. Reserve intrinsic gas separately.
const disperseRecipientGas = 38000
const disperseBaseGas = 30000

// DisperseScenario implements the TxGenerator interface for Disperse contract operations
type DisperseScenario struct {
	*ContractScenarioBase[bindings.Disperse]
	contract         *bindings.Disperse
	pool             types.AccountPool
	fixedEtherAmount *big.Int
}

// NewDisperseScenario creates a new Disperse scenario
func NewDisperseScenario(cfg config.Scenario) TxGenerator {
	scenario := &DisperseScenario{fixedEtherAmount: new(big.Int).Set(bigOne)}
	scenario.ContractScenarioBase = NewContractScenarioBase[bindings.Disperse](scenario, cfg)
	var err error
	scenario.pool, err = types.NewAccountPool(&types.AccountConfig{
		NewAccountRate: 1.0,
	})
	if err != nil {
		panic(err)
	} // The fresh-only pool above is a constant valid configuration.
	return scenario
}

// Name returns the name of the scenario.
func (s *DisperseScenario) Name() string {
	return Disperse
}

// DeployContract implements ContractDeployer interface - deploys Disperse with specific constructor args
func (s *DisperseScenario) DeployContract(opts *bind.TransactOpts, client *ethclient.Client) (common.Address, *ethtypes.Transaction, error) {
	address, tx, _, err := bindings.DeployDisperse(opts, client, bigOne, bigOne)
	return address, tx, err
}

// GetBindFunc implements ContractDeployer interface - returns the binding function
func (s *DisperseScenario) GetBindFunc() ContractBindFunc[bindings.Disperse] {
	return bindings.NewDisperse
}

// SetContract implements ContractDeployer interface - stores the contract instance
func (s *DisperseScenario) SetContract(contract *bindings.Disperse) {
	s.contract = contract
}

func (s *DisperseScenario) InitializeContract(ctx context.Context) error {
	amount, err := s.contract.FixedEtherAmount(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}
	if amount.Sign() <= 0 {
		return fmt.Errorf("disperse fixed ether amount must be positive")
	}
	s.fixedEtherAmount = new(big.Int).Set(amount)
	return nil
}

// Attach implements TxGenerator interface - attaches to an existing contract
func (s *DisperseScenario) Attach(config *config.LoadConfig, address common.Address) error {
	return s.ContractScenarioBase.Attach(config, address)
}

// CreateContractTransaction implements ContractDeployer interface - creates Disperse transaction
func (s *DisperseScenario) CreateContractTransaction(auth *bind.TransactOpts, scenario *types.TxScenario) (*ethtypes.Transaction, error) {
	if auth.GasLimit < disperseBaseGas+disperseRecipientGas {
		return nil, fmt.Errorf("disperse gas limit %d cannot fund one recipient (minimum %d)", auth.GasLimit, disperseBaseGas+disperseRecipientGas)
	}
	batchSize := min(uint64(DisperseBatchSize), (auth.GasLimit-disperseBaseGas)/disperseRecipientGas)
	targets := make([]common.Address, 0, batchSize)
	for range batchSize {
		targets = append(targets, s.pool.NextAccount().Address)
	}
	auth.Value = new(big.Int).Mul(s.fixedEtherAmount, new(big.Int).SetUint64(batchSize))
	return s.contract.DisperseEtherFixed(auth, targets)
}
