package runner

// Suite names a Go integration test and its YAML files relative to tests/integration.
type Suite struct {
	Name  string
	Files []string
}

// Suites preserves the CI execution order, including the file order within each suite.
var Suites = []Suite{
	{Name: "TestStartup", Files: []string{"startup/startup_test.yaml"}},
	{Name: "TestStakingModule", Files: []string{"staking_module/staking_test.yaml"}},
	{Name: "TestBankModule", Files: []string{
		"bank_module/send_funds_test.yaml",
		"bank_module/multi_sig_send_test.yaml",
		"bank_module/simulation_tx.yaml",
	}},
	{Name: "TestMintModule", Files: []string{"mint_module/mint_test.yaml"}},
	{Name: "TestGovModule", Files: []string{
		"gov_module/gov_proposal_test.yaml",
		"gov_module/staking_proposal_test.yaml",
	}},
	{Name: "TestOracleModule", Files: []string{
		"oracle_module/verify_penalty_counts.yaml",
		"oracle_module/set_feeder_test.yaml",
	}},
	{Name: "TestDistributionModule", Files: []string{
		"distribution_module/community_pool.yaml",
		"distribution_module/rewards.yaml",
	}},
	{Name: "TestTokenFactoryModule", Files: []string{"tokenfactory_module/create_tokenfactory_test.yaml"}},
	{Name: "TestAuthzModule", Files: []string{
		"authz_module/send_authorization_test.yaml",
		"authz_module/staking_authorization_test.yaml",
		"authz_module/generic_authorization_test.yaml",
	}},
	{Name: "TestWasmModule", Files: []string{
		"wasm_module/timelocked_token_delegation_test.yaml",
		"wasm_module/timelocked_token_admin_test.yaml",
		"wasm_module/timelocked_token_withdraw_test.yaml",
		"wasm_module/timelocked_token_emergency_withdraw_test.yaml",
	}},
	{Name: "TestPaxDB", Files: []string{
		"db/flatkv_evm_test.yaml",
		"db/state_store_test.yaml",
	}},
	{Name: "TestChainOperation", Files: []string{"chain_operation/snapshot_operation.yaml"}},
	{Name: "TestStateSync", Files: []string{"chain_operation/statesync_operation.yaml"}},
}

// ExcludedSuites records why each YAML outside Suites is deliberately excluded.
// Keys use the same paths relative to tests/integration as Suite.Files.
var ExcludedSuites = map[string]string{
	"upgrade_module/major_upgrade_test.yaml": "driven by the upgrade workflow through tests/integration/scripts/runner.py",
	"upgrade_module/minor_upgrade_test.yaml": "driven by the upgrade workflow through tests/integration/scripts/runner.py",
	"template/template_test.yaml":            "authoring template",
}
