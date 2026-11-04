// Package scenarios contains a compiled scaffold qualification fixture. Its base
// types and transaction operations are the production implementations.
package scenarios

import (
	"github.com/paxeer-network/pax-load/config"
	production "github.com/paxeer-network/pax-load/generator/scenarios"
)

type TxGenerator = production.TxGenerator
type ScenarioFactory = production.ScenarioFactory
type ContractScenarioBase[T any] = production.ContractScenarioBase[T]
type ContractBindFunc[T any] = production.ContractBindFunc[T]

func NewContractScenarioBase[T any](deployer production.ContractDeployer[T], cfg config.Scenario) *ContractScenarioBase[T] {
	return production.NewContractScenarioBase(deployer, cfg)
}
