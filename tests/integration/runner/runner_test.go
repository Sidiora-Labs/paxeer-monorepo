//go:build yaml_integration

// Package runner_test wires the existing YAML test files into standard `go test`.
//
// Requires a running Docker cluster (`make docker-cluster-start`).
//
// Run a single module:
//
//	go test -tags yaml_integration -v ./tests/integration/runner/... -run TestBankModule
//
// Run all modules:
//
//	go test -tags yaml_integration -v ./tests/integration/runner/...
//
// Each YAML test case becomes a named subtest, so -run accepts the full path:
//
//	-run TestBankModule/Test_sending_funds
package runner_test

import (
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/integration/runner"
)

func runSuite(t *testing.T, name string) {
	t.Helper()
	for _, suite := range runner.Suites {
		if suite.Name == name {
			for _, file := range suite.Files {
				runner.RunFile(t, filepath.Join("..", filepath.FromSlash(file)))
			}
			return
		}
	}
	t.Fatalf("YAML suite %q is not registered", name)
}

func TestStartup(t *testing.T) {
	runSuite(t, "TestStartup")
}

// Tests are declared in the order the CI matrix ran them as Python scripts
// (go test executes tests in declaration order): staking, then bank, then mint.

func TestStakingModule(t *testing.T) {
	runSuite(t, "TestStakingModule")
}

func TestBankModule(t *testing.T) {
	runSuite(t, "TestBankModule")
}

func TestMintModule(t *testing.T) {
	runSuite(t, "TestMintModule")
}

func TestGovModule(t *testing.T) {
	runSuite(t, "TestGovModule")
}

func TestOracleModule(t *testing.T) {
	runSuite(t, "TestOracleModule")
}

func TestDistributionModule(t *testing.T) {
	runSuite(t, "TestDistributionModule")
}

func TestTokenFactoryModule(t *testing.T) {
	runSuite(t, "TestTokenFactoryModule")
}

func TestAuthzModule(t *testing.T) {
	runSuite(t, "TestAuthzModule")
}

func TestWasmModule(t *testing.T) {
	runSuite(t, "TestWasmModule")
}

func TestPaxDB(t *testing.T) {
	runSuite(t, "TestPaxDB")
}

func TestChainOperation(t *testing.T) {
	runSuite(t, "TestChainOperation")
}

// TestStateSync requires the pax-rpc-node container, which is only present in
// the CI RPC-node cluster (make run-rpc-node-integration-ci).
func TestStateSync(t *testing.T) {
	runSuite(t, "TestStateSync")
}
