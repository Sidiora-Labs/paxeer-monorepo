package decode

import (
	_ "embed"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed chainlaunch_abi.json
var chainLaunchABIJSON string

var chainLaunchEvents = mustChainLaunchEvents()

func mustChainLaunchEvents() map[common.Hash]abi.Event {
	parsed, err := abi.JSON(strings.NewReader(chainLaunchABIJSON))
	if err != nil {
		panic(err)
	}
	events := make(map[common.Hash]abi.Event, len(parsed.Events))
	for _, event := range parsed.Events {
		events[event.ID] = event
	}
	return events
}

func isChainLaunchEvent(name string) bool {
	switch name {
	case "ChainLaunched", "ChainRevenueEpochOpened", "ChainRevenueClaimed", "LaunchedChainRetired":
		return true
	}
	return false
}
