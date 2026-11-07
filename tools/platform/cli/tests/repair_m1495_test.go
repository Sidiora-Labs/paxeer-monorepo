// Task 22.33.1495 repair record. The task's do_16 correction replaces the
// generated Go gate with repair_22_33_1495.rs; no Go subprocess runs in that gate.
package cli_test

const repairM1495Gate = "cargo test --locked --manifest-path tools/platform/cli/Cargo.toml --test repair_22_33_1495"

// Each entry names the finding and its executable Rust proving test.
var repairM1495Findings = map[string]string{
	"test_retired_endpoint_chains":     "test_retired_endpoint_chains",
	"credential_profile_collision_gap": "credential_profile_collision_gap",
	"test_retired_endpoint_faucet":     "test_retired_endpoint_faucet",
	"install_journey_a2a_contract":     "install_journey_a2a_contract",
	"test_retired_endpoint_lx_refusal": "test_retired_endpoint_lx_refusal",
	"mcp_binding_test_rewrite":         "mcp_binding_test_rewrite",
	"test_retired_endpoint_mcp_submit": "test_retired_endpoint_mcp_submit",
	"mcp_submit_path_test":             "mcp_submit_path_test",
}

// Wiring request: domain 8, tools/platform/cli/src/chains.rs: route Ethereum
// calls to the public node root instead of the retired public /rpc endpoint.
// Wiring request: domain 8, tools/platform/cli/src/rpc.rs: accept root node URLs
// and route Ethereum methods there while retaining explicit hosted service routing.
// Baseline: no prior hf-build/hf-done ledger existed; the ordered implementation-
// before-gates instruction forbids the generated do_12 baseline verification.
// Qualification revision, command, exit and private log are emitted by hf-done;
// this record makes no claim about an unexecuted gate or a live installation.

// Wiring request: domain 8, tools/platform/cli/src/a2a.rs: update its node-response
// unit fixture to expect POST / after the scoped toolset Ethereum route repair.

// Repair cycle 1: the daemon fixture needed its own std::time::Duration import.
// Failed build evidence: revision 019001892542682d722f9006460c3d3318023e4c,
// command cargo build --locked --manifest-path tools/platform/cli/Cargo.toml --tests,
// exit 101, private log /out/logs/build-1.log.

// Attempt 2 retains the production prepare_submission validation and root POST
// from 22.33.1493 while resolving the seed conflict. The wrong-chain test proves
// preflight refusal leaves the node listener untouched. The prior attempt's two
// verify failures were test_retired_endpoint_chains and test_retired_endpoint_lx_refusal;
// their required root admission is now supplied by 22.33.1492 and 22.33.1493.
