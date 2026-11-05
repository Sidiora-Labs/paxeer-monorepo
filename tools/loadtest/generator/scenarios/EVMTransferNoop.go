package scenarios

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/paxeer-network/pax-load/config"
	types2 "github.com/paxeer-network/pax-load/types"
)

const EVMTransferNoop = "evmtransfernoop"

// EVMTransferNoopScenario implements the TxGenerator interface for simple ETH transfers of 0 value
type EVMTransferNoopScenario struct {
	*ScenarioBase
}

// NewEVMTransferNoopScenario creates a new ETH transfer scenario
func NewEVMTransferNoopScenario(cfg config.Scenario) TxGenerator {
	scenario := &EVMTransferNoopScenario{}
	scenario.ScenarioBase = NewScenarioBase(scenario, cfg)
	return scenario
}

// Name returns the name of the scenario.
func (s *EVMTransferNoopScenario) Name() string {
	return EVMTransferNoop
}

// DeployScenario implements ScenarioDeployer interface - no deployment needed for ETH transfers
func (s *EVMTransferNoopScenario) DeployScenario(config *config.LoadConfig, deployer *types2.Account) common.Address {
	// No deployment needed for simple ETH transfers
	// Return zero address to indicate no contract deployment
	return common.Address{}
}

// AttachScenario implements ScenarioDeployer interface - no attachment needed for ETH transfers.
func (s *EVMTransferNoopScenario) AttachScenario(config *config.LoadConfig, address common.Address) common.Address {
	// No attachment needed for simple ETH transfers
	// Return zero address to indicate no contract deployment
	return common.Address{}
}

// CreateTransaction implements ScenarioDeployer interface - creates ETH transfer transaction
func (s *EVMTransferNoopScenario) CreateTransaction(config *config.LoadConfig, scenario *types2.TxScenario) (*ethtypes.Transaction, error) {
	auth, err := createScenarioTransactionOpts(config.GetChainID(), scenario, s.scenarioConfig, 21000)
	if err != nil {
		return nil, err
	}
	tx := &ethtypes.DynamicFeeTx{
		Nonce:     auth.Nonce.Uint64(),
		To:        &scenario.Sender.Address,
		Value:     big.NewInt(0),
		Gas:       auth.GasLimit,
		GasTipCap: auth.GasTipCap,
		GasFeeCap: auth.GasFeeCap,
		Data:      nil,
	}

	// Sign the transaction
	signer := ethtypes.NewCancunSigner(config.GetChainID())
	signedTx, err := ethtypes.SignTx(ethtypes.NewTx(tx), signer, scenario.Sender.PrivKey)
	if err != nil {
		return nil, err
	}

	return signedTx, nil
}
