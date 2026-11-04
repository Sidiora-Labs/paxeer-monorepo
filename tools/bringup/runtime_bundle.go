package bringup

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

//go:embed runtime-bundle.json
var runtimeBundleDeclaration []byte

type RuntimePlan struct {
	Version   int      `json:"version"`
	Bundle    string   `json:"bundle"`
	Names     []string `json:"names"`
	GoPaths   []string `json:"go_paths"`
	Libraries []string `json:"libraries"`
}

func ReadRuntimePlan(r io.Reader) (RuntimePlan, error) {
	var plan RuntimePlan
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return plan, err
	}
	if plan.Version != 2 || plan.Bundle != "merged-runtime" || len(plan.Names) != 3 || plan.Names[0] != "paxd" || plan.Names[1] != "attestor" || plan.Names[2] != "bringup-inspect" || len(plan.Libraries) != 3 {
		return plan, errors.New("retired or incomplete runtime bundle")
	}
	if len(plan.GoPaths) == 0 {
		return plan, errors.New("missing runtime source closure")
	}
	for _, path := range plan.GoPaths {
		if path == "custodyproof" || path == "layerxproof" || strings.Contains(path, "checkpoint-settlement") {
			return plan, errors.New("retired runtime source")
		}
	}
	return plan, nil
}

func RuntimeBundlePlan() (RuntimePlan, error) {
	return ReadRuntimePlan(strings.NewReader(string(runtimeBundleDeclaration)))
}
