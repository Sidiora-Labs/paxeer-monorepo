package scenarios

import (
	"github.com/paxeer-network/pax-load/config"
	"strings"
)

var scenarioFactories = map[string]ScenarioFactory{
	// DO NOT EDIT BELOW THIS LINE - AUTO-GENERATED CONTENT
	ERC20Noop: NewERC20NoopScenario,

	// DO NOT EDIT ABOVE THIS LINE - AUTO-GENERATED CONTENT
}

func CreateScenario(cfg config.Scenario) TxGenerator {
	return scenarioFactories[strings.ToLower(cfg.Name)](cfg)
}
