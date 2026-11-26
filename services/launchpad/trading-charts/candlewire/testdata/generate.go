package main

import (
	"encoding/json"
	"os"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire/testfixture"
)

func main() {
	records, err := testfixture.Records()
	if err != nil {
		panic(err)
	}
	raw, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile("candlewire/testdata/candle_identity.json", append(raw, '\n'), 0600); err != nil {
		panic(err)
	}
}
