# Interop deploy contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cli-deploy-gateway-render

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-deploy-gateway-render</code> | <code>cli_command</code> | <code>change</code> | <code>services/interop/deploy/gateway/render.py:1</code> | <code>python3 services/interop/deploy/gateway/render.py --check &lt;value&gt; --self-test &lt;value&gt; --network-id &lt;value&gt; --sequencer-public-key-file &lt;value&gt; --beta-roots-file &lt;value&gt; --out &lt;value&gt; --migration-profile-out &lt;value&gt; --repo-root &lt;value&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-deploy-gateway-render</code> | overridden_by | override.J47.13 |

## cli-deploy-mirror-deploy-ethereum-mirror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-deploy-mirror-deploy-ethereum-mirror</code> | <code>cli_command</code> | <code>change</code> | <code>services/interop/deploy/mirror/deploy-ethereum-mirror.sh:1</code> | <code>bash services/interop/deploy/mirror/deploy-ethereum-mirror.sh</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-deploy-mirror-deploy-ethereum-mirror</code> | notes | stale task 7.7.5 do_3 changes the archive contract payload, so the deployed runtime code and its runtime_code_keccak256 record change; reads environment BASH_SOURCE, CAST, CONTRACT_DIR, FORGE, LAYERX_MIRROR_CHAIN_ID, LAYERX_MIRROR_DEPLOYER_KEY_FILE, LAYERX_MIRROR_DEPLOYMENT_RECORD, LAYERX_MIRROR_FOUNDRY_BIN, LAYERX_MIRROR_PUBLISHER_ADDRESS, LAYERX_MIRROR_RPC_CA_PEM, LAYERX_MIRROR_RPC_URL, SCRIPT_DIR |

## cli-deploy-mirror-deploy-solana-mirror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-deploy-mirror-deploy-solana-mirror</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/deploy/mirror/deploy-solana-mirror.sh:1</code> | <code>bash services/interop/deploy/mirror/deploy-solana-mirror.sh</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-deploy-mirror-deploy-solana-mirror</code> | notes | reads environment BASH_SOURCE, BUILD_SBF, CONTRACT_DIR, KEYGEN, LAYERX_SOLANA_MIRROR_DEPLOYMENT_RECORD, LAYERX_SOLANA_MIRROR_KEYPAIR_FILE, LAYERX_SOLANA_MIRROR_PROGRAM_KEYPAIR_FILE, LAYERX_SOLANA_MIRROR_RPC_URL, LAYERX_SOLANA_MIRROR_TOOLCHAIN_BIN, SCRIPT_DIR, SOLANA, UPGRADEABLE_LOADER |

## cli-deploy-mirror-render-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-deploy-mirror-render-config</code> | <code>cli_command</code> | <code>change</code> | <code>services/interop/deploy/mirror/render-config.py:1</code> | <code>python3 services/interop/deploy/mirror/render-config.py --output &lt;value&gt; --state-directory &lt;value&gt; --first-batch-number &lt;value&gt; --status-listen &lt;value&gt; --paxeer-endpoint &lt;value&gt; --paxeer-chain-id &lt;value&gt; --paxeer-genesis-hash &lt;value&gt; --paxeer-confirmations &lt;value&gt; --sequencer-public-key &lt;value&gt; --ethereum-endpoint &lt;value&gt; --ethereum-chain-id &lt;value&gt; --ethereum-genesis-hash &lt;value&gt; --ethereum-archive-contract &lt;value&gt; --ethereum-archive-code-hash &lt;value&gt; --ethereum-signer-key-handle &lt;value&gt; --ethereum-signer-public-key &lt;value&gt; --ethereum-signer-socket &lt;value&gt; --solana-endpoint &lt;value&gt; --solana-genesis-hash &lt;value&gt; --solana-archive-program &lt;value&gt; --solana-upgradeable-loader &lt;value&gt; --solana-program-data-account &lt;value&gt; --solana-program-code-hash &lt;value&gt; --solana-signer-key-handle &lt;value&gt; --solana-signer-public-key &lt;value&gt; --solana-signer-socket &lt;value&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-deploy-mirror-render-config</code> | notes | stale task 11.5.119 do_2 and 7.7.5: the --lni-socket node input and checkpoint policy inputs follow the mirror input change |

## cli-deploy-mirror-tests-render-config-check

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-deploy-mirror-tests-render-config-check</code> | <code>cli_command</code> | <code>change</code> | <code>services/interop/deploy/mirror/tests/render-config-check.sh:1</code> | <code>bash services/interop/deploy/mirror/tests/render-config-check.sh</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-deploy-mirror-tests-render-config-check</code> | notes | stale task 11.5.119 do_2 and 7.7.5 change the rendered node and checkpoint inputs this check pins; reads environment BASH_SOURCE, CARGO_TARGET_DIR, ETHEREUM_PUBLIC, LAYERX_MIRROR_CHECK_PORT, PROGRAM, PUBLISHER, RENDERER, REPO_ROOT, SCRIPT_DIR, SOLANA_PUBLIC, STATUS_PORT, WORK; deployment check script |

## cli-deploy-x-websearch-tests-dry-run-check

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-deploy-x-websearch-tests-dry-run-check</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/tests/dry-run-check.sh:1</code> | <code>bash services/interop/deploy/x-websearch/tests/dry-run-check.sh</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-deploy-x-websearch-tests-dry-run-check</code> | notes | reads environment ABI, ANVIL_PORT, API_PORT, BINARY, CALLBACK_GAS, CARGO_TARGET_DIR, CHAIN_ID, CONSUMER, CONSUMER_ADDRESS, CONTEXT, FEE, FIXTURE, FRONT_PORT, HARNESS, MODE, PIDS, REPO_ROOT, SCRIPT_DIR, SIDECAR_PORTS, SITE, SITE_PORT, THRESHOLD, WORK; deployment check script |

## cli-deploy-x-websearch-tests-packaging-check

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-deploy-x-websearch-tests-packaging-check</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/tests/packaging-check.sh:1</code> | <code>bash services/interop/deploy/x-websearch/tests/packaging-check.sh</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-deploy-x-websearch-tests-packaging-check</code> | notes | reads environment BASH_SOURCE, BUILD_LINE, BUILD_STAGE, CARGO_TARGET_DIR, COMPOSE, COMPOSE_ENTRY, DEBIAN_BASE, DEPLOY_DIR, EXAMPLE, GROUPID, IMAGE_DOCKERFILE, IMAGE_TEXT, KEY_NAMES, KEY_VARIABLES, LAST_USER, LOADER, NODE_DOCKERFILE, NODE_RUNTIME, NODE_STAGE, REPO_ROOT, RUNTIME_STAGE, RUST_BASE, SCRIPT_DIR, SERVICE, SERVICES, UNIT, USERID, VALID_FIXTURE, WORK, X_WEBSEARCH_CHECK_IMAGE, X_WEBSEARCH_HOME; deployment check script |

## deploy-gateway-template

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-deploy-gateway-beta-conformance-j72</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/deploy/gateway/render.py:104</code> | <code>LAYERX_BETA_INTEROP_CONFORMANCE_&lt;X402&#124;AP2&#124;UCP&#124;VISA_TAP&#124;FIAT&#124;HTTP&#124;MCP&#124;A2A&gt;: optional &lt;suite-identifier&gt;,&lt;vector-count&gt;,&lt;suite-sha256&gt; overriding the first-party conformance suite of one adapter or transport</code> |
| <code>interface.11.env-deploy-gateway-beta-manifest-file-j72</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/deploy/gateway/render.py:103</code> | <code>LAYERX_BETA_INTEROP_MANIFEST_FILE: optional path to a JSON object that replaces the rendered interop manifest; unknown fields refused</code> |
| <code>interface.11.env-deploy-gateway-beta-roots-j72</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/deploy/gateway/render.py:112</code> | <code>LAYERX_BETA_INTEROP_&lt;X402_SUPPORTED&#124;AP2_KEYS&#124;AP2_ASSETS&#124;UCP_PAYMENT_HANDLER&#124;VISA_AGENTS&#124;VISA_TARGETS&#124;FIAT_PROVIDERS&gt;: optional JSON trust roots replacing the cluster defaults</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-deploy-gateway-beta-conformance-j72</code> | notes | names built from ADAPTERS and TRANSPORTS at services/interop/deploy/gateway/render.py:36-37 |
| <code>interface.11.env-deploy-gateway-beta-manifest-file-j72</code> | notes | read through OVERRIDE_VARIABLE; refusals at services/interop/deploy/gateway/render.py:447-453 |
| <code>interface.11.env-deploy-gateway-beta-roots-j72</code> | notes | names built from ROOTS at services/interop/deploy/gateway/render.py:48-56 |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.46.1</code> | req | 11046.1 |
| <code>acceptance.11.46.1</code> | scenario | Given config.rs and railway.env.example, when the test collects every LAYERX_INTEROP_ name, then each read name appears in the template. |
| <code>acceptance.11.46.1</code> | test | services/interop/deploy/gateway/tests/test_env_template.py::test_template_names_every_variable_the_service_reads (planned) |
| <code>acceptance.11.46.1</code> | qualification | UNRUN |
| <code>req.11046</code> | ac_1 | Every LAYERX_INTEROP_ variable config.rs reads is named in the template (planned) (services/interop/crates/layerx-interop-service/src/config.rs:254). |
| <code>acceptance.11.51.1</code> | req | 11051.1 |
| <code>acceptance.11.51.1</code> | scenario | Given a keys file with two overlapping ranges, when render.py runs, then it exits 2 naming both key ids. |
| <code>acceptance.11.51.1</code> | test | services/interop/deploy/gateway/tests/test_render_receipt_keys.py::test_overlapping_receipt_keys_are_refused (planned) |
| <code>acceptance.11.51.1</code> | qualification | UNRUN |
| <code>req.11051</code> | ac_1 | render.py refuses a receipt keys file with overlapping ranges (planned) (services/interop/deploy/gateway/render.py:835). |
| <code>acceptance.11.53.1</code> | req | 11053.1 |
| <code>acceptance.11.53.1</code> | scenario | Given config.example.json, when the test parses it, then gateway.receipt_keys is a non-empty array and kernel_network_id is absent. |
| <code>acceptance.11.53.1</code> | test | services/interop/deploy/x-websearch/tests/test_config_example.py::test_example_pins_receipt_keys (planned) |
| <code>acceptance.11.53.1</code> | qualification | UNRUN |
| <code>req.11053</code> | ac_1 | The x-websearch example names receipt_keys and no kernel_network_id (planned) (services/interop/deploy/x-websearch/config.example.json:49). |
| <code>acceptance.11.62.1</code> | req | 11062.1 |
| <code>acceptance.11.62.1</code> | scenario | Given the deploy tree, when the test lists templates, then none names INGESTER_ variables. |
| <code>acceptance.11.62.1</code> | test | services/interop/deploy/tests/test_deploy_templates.py::test_no_template_names_the_intent_ingester (planned) |
| <code>acceptance.11.62.1</code> | qualification | UNRUN |
| <code>req.11062</code> | ac_1 | No deploy template names the intent ingester (planned) (services/interop/deploy/intent-ingester/railway.env.example:1). |
| <code>acceptance.11.64.1</code> | req | 11064.1 |
| <code>acceptance.11.64.1</code> | scenario | Given the retired argument, when render.py runs, then it exits 2. |
| <code>acceptance.11.64.1</code> | test | services/interop/deploy/gateway/tests/test_render_receipt_keys.py::test_single_key_argument_is_refused (planned) |
| <code>acceptance.11.64.1</code> | qualification | UNRUN |
| <code>req.11064</code> | ac_1 | render.py refuses --sequencer-public-key-file (planned) (services/interop/deploy/gateway/render.py:840). |
| <code>acceptance.11.999.1</code> | req | 11999.1 |
| <code>acceptance.11.999.1</code> | scenario | Given the candidate after every domain-11 wave-6 task, when test_interop_completion_retired_surfaces_absent runs, then no retired surface remains and every planned path exists |
| <code>acceptance.11.999.1</code> | test | services/interop/deploy/gateway/tests/test_completion_gate.py::test_interop_completion_retired_surfaces_absent (planned) |
| <code>acceptance.11.999.1</code> | qualification | UNRUN |
| <code>req.11999</code> | ac_1 | test_interop_completion_retired_surfaces_absent passes on the merged candidate: no retired domain-11 surface remains and every planned domain-11 wave-6 path exists. |
| <code>capability.11.0.9</code> | title | Domain 11 completion gate |
| <code>capability.11.0.9</code> | reqs | 11999.1 |
| <code>capability.11.13.2</code> | title | Document the sequencer variables in the gateway deploy template and README |
| <code>capability.11.13.2</code> | reqs | 11046.1 |
| <code>capability.11.13.3</code> | title | Render the receipt keys file from the gateway deploy renderer |
| <code>capability.11.13.3</code> | reqs | 11051.1 |
| <code>capability.11.13.5</code> | title | Pin receipt keys in the x-websearch example config instead of the kernel network id |
| <code>capability.11.13.5</code> | reqs | 11053.1 |
| <code>capability.11.13.6</code> | title | Retire the intent-ingester deploy template |
| <code>capability.11.13.6</code> | reqs | 11062.1 |
| <code>capability.11.13.8</code> | title | Retire the single sequencer key argument from the gateway deploy renderer |
| <code>capability.11.13.8</code> | reqs | 11064.1 |

## deploy-mirror

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.6.1</code> | req | 11006.1 |
| <code>acceptance.11.6.1</code> | scenario | Given the flags in render-config-check.sh and the add_argument names in render-config.py, when the test compares them, then the check uses no undeclared flag. |
| <code>acceptance.11.6.1</code> | test | services/interop/deploy/mirror/tests/test_render_check_arguments.py::test_check_passes_only_arguments_the_renderer_declares (planned) |
| <code>acceptance.11.6.1</code> | qualification | UNRUN |
| <code>req.11006</code> | ac_1 | Every flag the check passes to render-config.py is declared by its argument parser (services/interop/deploy/mirror/render-config.py:152) (planned). |
| <code>acceptance.11.63.1</code> | req | 11063.1 |
| <code>acceptance.11.63.1</code> | scenario | Given a receipt keys file with two non-overlapping entries, when render-config.py runs with --receipt-keys-file and --first-height, then the output keys equal those of config.example.json, paxeer.receipt_keys lists both entries in order, and neither node.checkpoint_policy nor checkpoint_freshness_budget_batches is rendered. |
| <code>acceptance.11.63.1</code> | test | services/interop/deploy/mirror/tests/test_render_check_arguments.py::test_rendered_keys_match_the_example (planned) |
| <code>acceptance.11.63.1</code> | qualification | UNRUN |
| <code>req.11063</code> | ac_1 | The rendered mirror config has the key set of services/interop/deploy/mirror/config.example.json (planned rewrite), with first_height, freshness_budget_blocks and a paxeer section whose receipt_keys equal the keys file entries, and no node.checkpoint_policy as rendered today at services/interop/deploy/mirror/render-config.py:238. |
| <code>capability.11.13.1</code> | title | Align the mirror render check with the renderer's accepted arguments |
| <code>capability.11.13.1</code> | reqs | 11006.1 |
| <code>capability.11.13.7</code> | title | Render the mirror paxeer section and receipt keys from the pinned receipt keys file |
| <code>capability.11.13.7</code> | reqs | 11063.1 |

## deploy-search-front

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.52.1</code> | req | 11052.1 |
| <code>acceptance.11.52.1</code> | scenario | Given nginx.conf, when the test reads the search location block, then it sets LAYERX-PAYER-DID to empty and keeps proxy_pass_request_headers on. |
| <code>acceptance.11.52.1</code> | test | services/interop/deploy/search-front/tests/test_nginx_headers.py::test_search_location_clears_the_payer_did (planned) |
| <code>acceptance.11.52.1</code> | qualification | UNRUN |
| <code>req.11052</code> | ac_1 | The search location clears LAYERX-PAYER-DID (planned) (services/interop/deploy/search-front/nginx.conf:94). |
| <code>capability.11.13.4</code> | title | Stop forwarding LAYERX-PAYER-DID through the search front |
| <code>capability.11.13.4</code> | reqs | 11052.1 |

## deploy-workspace

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.65.1</code> | req | 11065.1 |
| <code>acceptance.11.65.1</code> | scenario | Given the in-scope crate sources, when the test scans them, then neither name appears. |
| <code>acceptance.11.65.1</code> | test | services/interop/crates/layerx-interop-gateway/tests/workspace_lock.rs::no_member_settles_on_authorized_batch (planned) |
| <code>acceptance.11.65.1</code> | qualification | UNRUN |
| <code>req.11065</code> | ac_1 | No in-scope crate names AuthorizedBatch or settle_with_receipt (planned) (services/interop/crates/layerx-interop-gateway/src/gateway.rs:506). |
| <code>acceptance.11.7.1</code> | req | 11007.1 |
| <code>acceptance.11.7.1</code> | scenario | Given the workspace manifest, every member manifest and Cargo.lock, when the test resolves each non-optional dependency name, then each name has a [[package]] entry and layerx-committed-proof is listed as a member package. |
| <code>acceptance.11.7.1</code> | test | services/interop/crates/layerx-interop-gateway/tests/workspace_lock.rs::lock_lists_every_member_dependency (planned) |
| <code>acceptance.11.7.1</code> | qualification | UNRUN |
| <code>req.11007</code> | ac_1 | Every dependency declared by a member of services/interop/Cargo.toml:2-18 has a package entry in services/interop/Cargo.lock:1 (planned). |
| <code>capability.11.15.1</code> | title | Add the wave-0 dependency edges and the committed-proof member in one lock update |
| <code>capability.11.15.1</code> | reqs | 11007.1 |
| <code>capability.11.15.2</code> | title | Guard the workspace against the retired batch settlement path |
| <code>capability.11.15.2</code> | reqs | 11065.1 |
| <code>resolution.17.878</code> | title | decision.1101 binds: Stale finding 5.0.8 is not reproduced at HEAD |
| <code>resolution.17.878</code> | applies_to | decision.1101<br>task.11.15.1<br>task.11.15.2 |
| <code>resolution.17.878</code> | source | decision.1101 in fragments/J47/decision.kvx; tasks from affects |

## fmt-deploy-bridge-relayer-relayer-1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-1</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/bridge-relayer/relayer-1.toml:1</code> | <code>TOML sections [build] [env] [[files]] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, LAYERX_BRIDGE_OPERATOR_INVENTORY, LAYERX_BRIDGE_RELAYER_CONFIG, LAYERX_BRIDGE_ATTESTOR_INDEX, LAYERX_BRIDGE_ATTESTOR_SET_SIZE, LAYERX_BRIDGE_ATTESTOR_THRESHOLD, LAYERX_BRIDGE_ATTESTOR_KEY_FILE, LAYERX_BRIDGE_COSIGN_CA, LAYERX_BRIDGE_APPROVED_MEMBERSHIP, guest_path, secret_name, source, destination, initial_size, size, memory, cpus</code> |

## fmt-deploy-bridge-relayer-relayer-2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-2</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/bridge-relayer/relayer-2.toml:1</code> | <code>TOML sections [build] [env] [[files]] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, LAYERX_BRIDGE_OPERATOR_INVENTORY, LAYERX_BRIDGE_RELAYER_CONFIG, LAYERX_BRIDGE_ATTESTOR_INDEX, LAYERX_BRIDGE_ATTESTOR_SET_SIZE, LAYERX_BRIDGE_ATTESTOR_THRESHOLD, LAYERX_BRIDGE_ATTESTOR_KEY_FILE, LAYERX_BRIDGE_COSIGN_CA, LAYERX_BRIDGE_APPROVED_MEMBERSHIP, guest_path, secret_name, source, destination, initial_size, size, memory, cpus</code> |

## fmt-deploy-bridge-relayer-relayer-3

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-3</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/bridge-relayer/relayer-3.toml:1</code> | <code>TOML sections [build] [env] [[files]] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, LAYERX_BRIDGE_OPERATOR_INVENTORY, LAYERX_BRIDGE_RELAYER_CONFIG, LAYERX_BRIDGE_ATTESTOR_INDEX, LAYERX_BRIDGE_ATTESTOR_SET_SIZE, LAYERX_BRIDGE_ATTESTOR_THRESHOLD, LAYERX_BRIDGE_ATTESTOR_KEY_FILE, LAYERX_BRIDGE_COSIGN_CA, LAYERX_BRIDGE_APPROVED_MEMBERSHIP, guest_path, secret_name, source, destination, initial_size, size, memory, cpus</code> |

## fmt-deploy-bridge-relayer-relayer-4

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-4</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/bridge-relayer/relayer-4.toml:1</code> | <code>TOML sections [build] [env] [[files]] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, LAYERX_BRIDGE_OPERATOR_INVENTORY, LAYERX_BRIDGE_RELAYER_CONFIG, LAYERX_BRIDGE_ATTESTOR_INDEX, LAYERX_BRIDGE_ATTESTOR_SET_SIZE, LAYERX_BRIDGE_ATTESTOR_THRESHOLD, LAYERX_BRIDGE_ATTESTOR_KEY_FILE, LAYERX_BRIDGE_COSIGN_CA, LAYERX_BRIDGE_APPROVED_MEMBERSHIP, guest_path, secret_name, source, destination, initial_size, size, memory, cpus</code> |

## fmt-deploy-bridge-relayer-relayer-5

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-relayer-5</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/bridge-relayer/relayer-5.toml:1</code> | <code>TOML sections [build] [env] [[files]] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG, LAYERX_BRIDGE_OPERATOR_INVENTORY, LAYERX_BRIDGE_RELAYER_CONFIG, LAYERX_BRIDGE_ATTESTOR_INDEX, LAYERX_BRIDGE_ATTESTOR_SET_SIZE, LAYERX_BRIDGE_ATTESTOR_THRESHOLD, LAYERX_BRIDGE_ATTESTOR_KEY_FILE, LAYERX_BRIDGE_COSIGN_CA, LAYERX_BRIDGE_APPROVED_MEMBERSHIP, guest_path, secret_name, source, destination, initial_size, size, memory, cpus</code> |

## fmt-deploy-bridge-relayer-signer-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-bridge-relayer-signer-keys</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/bridge-relayer/signer-keys.json:1</code> | <code>JSON object {keys}</code> |

## fmt-deploy-gas-station-hosted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-gas-station-hosted</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/gas-station/railway.env.example:1</code> | <code>env file KEY=value lines: PORT, GAS_STATION_LISTEN, GAS_STATION_CHAIN_ID, GAS_STATION_ENDPOINTS, GAS_STATION_TOKEN, GAS_STATION_DECIMALS, GAS_STATION_SPREAD_BPS, GAS_STATION_RELAYER_KEY_ENV, GAS_STATION_RATE_OWNER_KEY_ENV, GAS_STATION_RATE_CADENCE_SECONDS, GAS_STATION_RATE_CONFIRMATION_RETRY_SECONDS, GAS_STATION_RATE_MAX_FEE_PER_GAS, GAS_STATION_PAYMASTER, GAS_STATION_MAX_RATE_AGE, GAS_STATION_MARGIN_BPS, GAS_STATION_PER_QUOTE_LIMIT, GAS_STATION_PER_ACCOUNT_LIMIT, GAS_STATION_PER_INTERVAL_LIMIT, GAS_STATION_INTERVAL_SECONDS, GAS_STATION_BALANCE_FLOOR, GAS_STATION_GAS_LIMIT, GAS_STATION_MAX_PRIORITY_FEE_PER_GAS, GAS_STATION_DID_GAS_LIMIT, GAS_STATION_RATE_GAS_BUDGET_PER_DAY, GAS_STATION_RATE_BALANCE_FLOOR, GAS_STATION_BROWSER_ORIGINS, GAS_STATION_RELAYER_KEY, GAS_STATION_RATE_OWNER_KEY</code> |

## fmt-deploy-gateway-config-example

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-config-example</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/gateway/config.example.json:1</code> | <code>JSON object {adapters, transports, x402_supported, ap2_keys, ap2_assets, ucp_payment_handler, visa_agents, visa_targets, fiat_providers}</code> |

## fmt-deploy-gateway-hosted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-hosted</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/deploy/gateway/railway.env.example:1</code> | <code>env file KEY=value lines: LAYERX_INTEROP_LISTENER, PORT, LAYERX_INTEROP_LISTEN, LAYERX_INTEROP_CHAIN_ID, LAYERX_INTEROP_PROTOCOL_NETWORK_ID, LAYERX_INTEROP_WIRE_VERSION, LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS, LAYERX_INTEROP_IDEMPOTENCY_SECONDS, LAYERX_INTEROP_NETWORK_ID, LAYERX_INTEROP_READINESS_MAX_AGE_MS, LAYERX_INTEROP_ALLOW_&lt;PLATFORM&gt;_PRIVATE_NETWORK, LAYERX_INTEROP_HOSTED_GATEWAY_URL, LAYERX_INTEROP_RECEIPT_AUTHORITY_URL, LAYERX_INTEROP_REDIS_URL, LAYERX_INTEROP_CONFIG, LAYERX_INTEROP_MODULE_REGISTRY_FILE, LAYERX_INTEROP_OUTBOUND_CA_DER, LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12, LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE, LAYERX_INTEROP_RECEIPT_AUTHORITY_TOKEN_FILE, LAYERX_INTEROP_SEQUENCER_PUBLIC_KEY_FILE, LAYERX_INTEROP_SEQUENCER_ID_FILE, LAYERX_INTEROP_SEQUENCER_FIRST_BATCH_FILE, LAYERX_INTEROP_SEQUENCER_LAST_BATCH_FILE, LAYERX_INTEROP_REDIS_USERNAME_FILE, LAYERX_INTEROP_REDIS_PASSWORD_FILE, INTEROP_CONFIG, INTEROP_MODULE_REGISTRY, INTEROP_CLIENT_CA_DER, INTEROP_CLIENT_P12, INTEROP_CLIENT_PASSWORD, INTEROP_AUTHORITY_TOKEN, INTEROP_SEQUENCER_PUBLIC_KEY, INTEROP_SEQUENCER_ID, INTEROP_SEQUENCER_FIRST_BATCH, INTEROP_SEQUENCER_LAST_BATCH, INTEROP_REDIS_USERNAME, INTEROP_REDIS_PASSWORD</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-gateway-hosted</code> | notes | a hosting-platform name inside the identifier is masked as &lt;PLATFORM&gt; |
| <code>interface.11.fmt-deploy-gateway-hosted</code> | overridden_by | override.J47.14 |

## fmt-deploy-gateway-module-registry-example

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-module-registry-example</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/gateway/module-registry.example.json:1</code> | <code>JSON object {schema_version, assets, modules}</code> |

## fmt-deploy-gateway-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-readme</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/deploy/gateway/README.md:1</code> | <code>Markdown document: Interop gateway runtime additions</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-gateway-readme</code> | overridden_by | override.J47.15 |

## fmt-deploy-gateway-runtime

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-runtime</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/gateway/runtime.env.example:1</code> | <code>env file KEY=value lines: LAYERX_INTEROP_PROTOCOL_NETWORK_ID, LAYERX_INTEROP_MODULE_REGISTRY_FILE, LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS</code> |

## fmt-deploy-gateway-visa-trust-example

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-gateway-visa-trust-example</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/gateway/visa-trust.example.json:1</code> | <code>JSON object {visa_agents, visa_targets}</code> |

## fmt-deploy-intent-ingester-hosted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-intent-ingester-hosted</code> | <code>file_format</code> | <code>retire</code> | <code>services/interop/deploy/intent-ingester/railway.env.example:1</code> | <code>env file KEY=value lines: &lt;PLATFORM&gt;_RUN_UID, PORT, INGESTER_LISTEN, INGESTER_STATE_DIR, INGESTER_INTERVAL_MS, INGESTER_RPC_URL, LAYERX_INGESTER_GATEWAY_URL, LAYERX_INGESTER_GATEWAY_KEY_ID, LAYERX_INGESTER_GATEWAY_API_KEY, LAYERX_INGESTER_NETWORK_ID, LAYERX_INGESTER_FEE_LIMIT, LAYERX_INGESTER_SIGNER_KEY_FILE, LAYERX_INGESTER_MARKET_MAP, INTENT_INGESTER_SIGNER_KEY, INTENT_INGESTER_MARKET_MAP</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-intent-ingester-hosted</code> | notes | stale task 3.6.14 and 11.7.35 retire the intent ingester; 11.5.120 do_5 leaves the reference at line 10 to leave with the tree; a hosting-platform name inside the identifier is masked as &lt;PLATFORM&gt; |

## fmt-deploy-mirror-config-example

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-config-example</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/deploy/mirror/config.example.json:1</code> | <code>JSON object {state_directory, first_batch_number, poll_interval_ms, status_listen, ethereum, solana, checkpoint_freshness_budget_batches}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-mirror-config-example</code> | notes | stale task 11.5.119 do_2 moves the node section off the LNI socket transport and 7.7.5 replaces the checkpoint policy with committed header reads |

## fmt-deploy-mirror-ethereum-deployment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-ethereum-deployment</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/mirror/ethereum-deployment.json:1</code> | <code>JSON object {contract, source, constructor_arguments, required_post_deploy_records}</code> |

## fmt-deploy-mirror-ethereum-testnets-dated

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-ethereum-testnets-dated</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/mirror/ethereum-testnets-2026-08-31.json:1</code> | <code>JSON object {contract, source, source_commit, compiler, evm_version, optimizer, publisher, runtime_code_keccak256, deployments}</code> |

## fmt-deploy-mirror-signer-protocol

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-signer-protocol</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/mirror/signer-protocol.md:1</code> | <code>Markdown document: Mirror remote signer protocol</code> |

## fmt-deploy-mirror-solana-deployment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-solana-deployment</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/mirror/solana-deployment.json:1</code> | <code>JSON object {program, source, upgrade_policy, archive_namespace, required_post_deploy_records}</code> |

## fmt-deploy-mirror-solana-devnet-dated

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-solana-devnet-dated</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/mirror/solana-devnet-2026-08-31.json:1</code> | <code>JSON object {program, source, source_commit, toolchain, network, genesis_hash, program_id, publisher_ed25519_public_key, program_data_account, upgradeable_loader_id, program_elf_bytes, program_elf_sha256, deployment_signature, deployment_finalized_slot, deployment_fee_lamports, program_data_rent_lamports, program_account_rent_lamports, finalization_signature, finalization_finalized_slot, finalization_fee_lamports, upgrade_authority, on_chain_elf_verified}</code> |

## fmt-deploy-mirror-verify-config-example

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-mirror-verify-config-example</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/mirror/verify-config.example.json:1</code> | <code>JSON object {layerx_network_id, sequencer_id_hex, sequencer_public_key_hex, first_batch_number, last_batch_number, sources}</code> |

## fmt-deploy-search-front-hosted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-search-front-hosted</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/search-front/railway.env.example:1</code> | <code>env file KEY=value lines: PORT, LAYERX_TRUSTED_PROXIES, SEARCH_FRONT_UPSTREAMS</code> |

## fmt-deploy-search-front-nginx

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-search-front-nginx</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/deploy/search-front/nginx.conf:1</code> | <code>nginx configuration: location = /healthz, location = /readyz, location = /_search_ready, location @not_ready, location = /xweb/health, location ~ ^/(search&#124;fetch&#124;content/), location /</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-search-front-nginx</code> | notes | stale task 5.4.4 do_1 and 11.5.125 drop the LAYERX-PAYER-DID forward (services/interop/deploy/search-front/nginx.conf:104) |

## fmt-deploy-x-websearch-attestor-1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-attestor-1</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/attestor-1.toml:1</code> | <code>TOML sections [build] [deploy] [env] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, build-target, strategy, X_WEBSEARCH_ATTESTOR_KEY_FILE, X_WEBSEARCH_SUBMITTER_KEY_FILE, X_WEBSEARCH_RECEIVER_KEY_FILE, XWEB_HOPS, source, destination, initial_size, scheduled_snapshots, snapshot_retention, size, memory, cpus, cpu_kind</code> |

## fmt-deploy-x-websearch-attestor-2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-attestor-2</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/attestor-2.toml:1</code> | <code>TOML sections [build] [deploy] [env] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, build-target, strategy, X_WEBSEARCH_ATTESTOR_KEY_FILE, X_WEBSEARCH_SUBMITTER_KEY_FILE, X_WEBSEARCH_RECEIVER_KEY_FILE, XWEB_HOPS, source, destination, initial_size, scheduled_snapshots, snapshot_retention, size, memory, cpus, cpu_kind</code> |

## fmt-deploy-x-websearch-attestor-3

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-attestor-3</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/attestor-3.toml:1</code> | <code>TOML sections [build] [deploy] [env] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, build-target, strategy, X_WEBSEARCH_ATTESTOR_KEY_FILE, X_WEBSEARCH_SUBMITTER_KEY_FILE, X_WEBSEARCH_RECEIVER_KEY_FILE, XWEB_HOPS, source, destination, initial_size, scheduled_snapshots, snapshot_retention, size, memory, cpus, cpu_kind</code> |

## fmt-deploy-x-websearch-attestor-4

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-attestor-4</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/attestor-4.toml:1</code> | <code>TOML sections [build] [deploy] [env] [[mounts]] [[vm]]; keys app, primary_region, kill_signal, kill_timeout, dockerfile, build-target, strategy, X_WEBSEARCH_ATTESTOR_KEY_FILE, X_WEBSEARCH_SUBMITTER_KEY_FILE, X_WEBSEARCH_RECEIVER_KEY_FILE, XWEB_HOPS, source, destination, initial_size, scheduled_snapshots, snapshot_retention, size, memory, cpus, cpu_kind</code> |

## fmt-deploy-x-websearch-config-example

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-config-example</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/deploy/x-websearch/config.example.json:1</code> | <code>JSON object {note, listen, data_dir, seeds, crawl, fetch, assets, gateway, evm, kernel_network_id, peers}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-config-example</code> | notes | stale task 11.5.120 and 11.5.126 remove kernel_network_id (services/interop/deploy/x-websearch/config.example.json:49) and the kernel relay fields; 5.4.3 pins receipt_keys in place of the gateway sequencer key fields (services/interop/deploy/x-websearch/config.example.json:41-42) |

Contract context <code>contract_context.11.x-websearch-config</code>:

| Field | Frozen rule |
|---|---|
| interface | x-websearch reads one JSON configuration file and refuses unknown fields. |
| storage | none |
| errors | ConfigError naming the refused field. |
| events | none |
| gas | none |
| activation | Read at start with --config. |
| invariants | Key material never appears in the configuration. |
| pause | none |

## fmt-deploy-x-websearch-nginx-search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-nginx-search</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/nginx-search.conf:1</code> | <code>nginx configuration: location ~ ^/(search&#124;fetch&#124;content/), location = /xweb/health</code> |

## fmt-deploy-x-websearch-tests-fixtures-dry-run

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-dry-run</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/tests/fixtures/dry-run.json:1</code> | <code>JSON object {attestors, callback_gas, calls, chain_id, consumer, credential_header, credential_scan, legs, pages, precompile, submitter, threshold, version, views}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-dry-run</code> | notes | deployment check fixture |

## fmt-deploy-x-websearch-tests-fixtures-site-guide

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-guide</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/tests/fixtures/site/guide.html:1</code> | <code>HTML fixture page</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-guide</code> | notes | deployment check fixture |

## fmt-deploy-x-websearch-tests-fixtures-site-index

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-index</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/tests/fixtures/site/index.html:1</code> | <code>HTML fixture page</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-index</code> | notes | deployment check fixture |

## fmt-deploy-x-websearch-tests-fixtures-site-notes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-notes</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/tests/fixtures/site/notes.txt:1</code> | <code>plain text fixture</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-notes</code> | notes | deployment check fixture |

## fmt-deploy-x-websearch-tests-fixtures-site-robots

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-robots</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/tests/fixtures/site/robots.txt:1</code> | <code>plain text fixture</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-tests-fixtures-site-robots</code> | notes | deployment check fixture |

## fmt-deploy-x-websearch-x-websearch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-x-websearch</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/x-websearch.service:1</code> | <code>systemd unit; User=...; LoadCredential=...; LoadCredential=...; LoadCredential=...; ExecStart=/usr/local/bin/x-websearch; StateDirectory=...</code> |

## fmt-deploy-x-websearch-x-websearch-serve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-deploy-x-websearch-x-websearch-serve</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/x-websearch-serve.service:1</code> | <code>systemd unit; User=...; LoadCredential=...; ExecStart=/usr/local/bin/x-websearch; StateDirectory=...</code> |

## http-search-front-default

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-search-front-default</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/deploy/search-front/nginx.conf:119</code> | <code>any other path</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-search-front-default</code> | notes | nginx; 404 |

## http-search-front-healthz

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-search-front-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/deploy/search-front/nginx.conf:33</code> | <code>GET /healthz</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-search-front-healthz</code> | notes | nginx; answers 200 ok from nginx itself |

## http-search-front-not-ready

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-search-front-not-ready</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/deploy/search-front/nginx.conf:73</code> | <code>named location @not_ready</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-search-front-not-ready</code> | notes | nginx; answers 503 {"ready":false,"error":"selected_backend_unavailable"} |

## http-search-front-readyz

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-search-front-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/deploy/search-front/nginx.conf:39</code> | <code>GET /readyz</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-search-front-readyz</code> | notes | nginx; 503 without a selected node or when not eligible, else proxied to the selected sidecar /readyz |

## http-search-front-search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-search-front-search</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/deploy/search-front/nginx.conf:94</code> | <code>GET /search &#124; /fetch &#124; /content/&lt;digest&gt; (regex ^/(search&#124;fetch&#124;content/))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-search-front-search</code> | notes | nginx; proxied unchanged to the selected sidecar with X-Search-Node; passes PAYMENT-REQUIRED, PAYMENT-RESPONSE, PAYMENT-SIGNATURE and LAYERX-PAYER-DID; stale task 5.4.4 do_1 and 11.5.125 drop the LAYERX-PAYER-DID forward (services/interop/deploy/search-front/nginx.conf:104) |

## http-search-front-search-ready

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-search-front-search-ready</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/deploy/search-front/nginx.conf:56</code> | <code>internal auth_request /_search_ready</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-search-front-search-ready</code> | notes | nginx; internal subrequest gating the search location |

## http-search-front-xweb-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-search-front-xweb-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/deploy/search-front/nginx.conf:80</code> | <code>GET /xweb/health</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-search-front-xweb-health</code> | notes | nginx; 502 without a selected node, else proxied |

## http-xws-nginx-search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-nginx-search</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/nginx-search.conf:3</code> | <code>GET /search &#124; /fetch &#124; /content/&lt;digest&gt; (include for a sidecar host)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-nginx-search</code> | notes | nginx; proxies to the local sidecar with Host, X-Forwarded-For and X-Forwarded-Proto |

## http-xws-nginx-xweb-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-xws-nginx-xweb-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/deploy/x-websearch/nginx-search.conf:12</code> | <code>GET /xweb/health (include for a sidecar host)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-xws-nginx-xweb-health</code> | notes | nginx; proxies to the local sidecar health route |
