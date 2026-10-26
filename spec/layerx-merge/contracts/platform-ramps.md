# Platform ramps contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## ramp-container-environment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-ramps-env-init-ramp-actor-did-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:96</code> | <code>env.RAMP_ACTOR_DID read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-client-timeout-seconds-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:104</code> | <code>env.RAMP_CLIENT_TIMEOUT_SECONDS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-compliance-endpoint-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:107</code> | <code>env.RAMP_COMPLIANCE_ENDPOINT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-compliance-public-key-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:107</code> | <code>env.RAMP_COMPLIANCE_PUBLIC_KEY read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-custody-endpoint-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:132</code> | <code>env.RAMP_CUSTODY_ENDPOINT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-delayed-after-polls-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:146</code> | <code>env.RAMP_DELAYED_AFTER_POLLS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-fee-limit-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:124</code> | <code>env.RAMP_FEE_LIMIT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-gas-limit-cap-j41</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/ramps/env-init.sh</code> (planned) | <code>env.RAMP_GAS_LIMIT_CAP read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-gateway-endpoint-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:115</code> | <code>env.RAMP_GATEWAY_ENDPOINT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-identity-audience-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:106</code> | <code>env.RAMP_IDENTITY_AUDIENCE read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-identity-endpoint-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:106</code> | <code>env.RAMP_IDENTITY_ENDPOINT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-layerx-signer-key-handle-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:97</code> | <code>env.RAMP_LAYERX_SIGNER_KEY_HANDLE read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-layerx-signer-public-key-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:125</code> | <code>env.RAMP_LAYERX_SIGNER_PUBLIC_KEY read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-lease-seconds-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:92</code> | <code>env.RAMP_LEASE_SECONDS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-listen-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:88</code> | <code>env.RAMP_LISTEN read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-max-fee-per-gas-j41</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/ramps/env-init.sh</code> (planned) | <code>env.RAMP_MAX_FEE_PER_GAS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-max-priority-fee-per-gas-j41</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/ramps/env-init.sh</code> (planned) | <code>env.RAMP_MAX_PRIORITY_FEE_PER_GAS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-network-id-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:123</code> | <code>env.RAMP_NETWORK_ID read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-operator-address-j41</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/ramps/env-init.sh</code> (planned) | <code>env.RAMP_OPERATOR_ADDRESS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-operator-principal-id-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:95</code> | <code>env.RAMP_OPERATOR_PRINCIPAL_ID read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-paxeer-signer-key-handle-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:139</code> | <code>env.RAMP_PAXEER_SIGNER_KEY_HANDLE read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-poll-cadence-seconds-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:145</code> | <code>env.RAMP_POLL_CADENCE_SECONDS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-protocol-version-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:122</code> | <code>env.RAMP_PROTOCOL_VERSION read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-provider-callback-public-key-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:148</code> | <code>env.RAMP_PROVIDER_CALLBACK_PUBLIC_KEY read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-provider-endpoint-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:109</code> | <code>env.RAMP_PROVIDER_ENDPOINT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-receipt-authority-endpoint-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:116</code> | <code>env.RAMP_RECEIPT_AUTHORITY_ENDPOINT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-receipt-keys-json-j41</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/ramps/env-init.sh</code> (planned) | <code>env.RAMP_RECEIPT_KEYS_JSON read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-reconcile-seconds-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:93</code> | <code>env.RAMP_RECONCILE_SECONDS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-required-confirmations-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:144</code> | <code>env.RAMP_REQUIRED_CONFIRMATIONS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-rpc-chain-id-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:142</code> | <code>env.RAMP_RPC_CHAIN_ID read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-rpc-minimum-agreement-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:143</code> | <code>env.RAMP_RPC_MINIMUM_AGREEMENT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-rpc-names-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:65</code> | <code>env.RAMP_RPC_NAMES read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-first-batch-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:128</code> | <code>env.RAMP_SEQUENCER_FIRST_BATCH read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-id-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:126</code> | <code>env.RAMP_SEQUENCER_ID read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-last-batch-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:129</code> | <code>env.RAMP_SEQUENCER_LAST_BATCH read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-public-key-j41</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/env-init.sh:127</code> | <code>env.RAMP_SEQUENCER_PUBLIC_KEY read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-rpc-endpoint-j41</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/ramps/env-init.sh</code> (planned) | <code>env.RAMP_SEQUENCER_RPC_ENDPOINT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-signer-endpoint-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:117</code> | <code>env.RAMP_SIGNER_ENDPOINT read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-vault-id-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:138</code> | <code>env.RAMP_VAULT_ID read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-wallet-address-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:137</code> | <code>env.RAMP_WALLET_ADDRESS read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-env-init-ramp-worker-id-j41</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/env-init.sh:91</code> | <code>env.RAMP_WORKER_ID read by the ramp container initialiser</code> |
| <code>interface.8.env-ramps-layerx-ramp-expected-revision-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:354</code> | <code>expected_revision = env("LAYERX_RAMP_EXPECTED_REVISION")</code> |
| <code>interface.8.env-ramps-layerx-ramp-expected-source-digest-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:355</code> | <code>expected_digest = env("LAYERX_RAMP_EXPECTED_SOURCE_DIGEST")</code> |
| <code>interface.8.env-ramps-layerx-ramp-receipt-verifier-binary-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:248</code> | <code>[env("LAYERX_RAMP_RECEIPT_VERIFIER_BINARY"), "--config-file",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-ramps-env-init-ramp-actor-did-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-client-timeout-seconds-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-compliance-endpoint-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-compliance-public-key-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-custody-endpoint-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-delayed-after-polls-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-fee-limit-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-gateway-endpoint-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-identity-audience-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-identity-endpoint-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-layerx-signer-key-handle-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-layerx-signer-public-key-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-lease-seconds-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-listen-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-network-id-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-operator-principal-id-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-paxeer-signer-key-handle-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-poll-cadence-seconds-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-protocol-version-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-provider-callback-public-key-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-provider-endpoint-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-receipt-authority-endpoint-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-reconcile-seconds-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-required-confirmations-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-rpc-chain-id-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-rpc-minimum-agreement-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-rpc-names-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-first-batch-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-id-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-last-batch-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-sequencer-public-key-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-signer-endpoint-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-vault-id-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-wallet-address-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-env-init-ramp-worker-id-j41</code> | notes | read by services/platform/ramps/env-init.sh and listed in the ramp deployment env example; no other in-repo caller |
| <code>interface.8.env-ramps-layerx-ramp-expected-revision-j69</code> | notes | read at services/platform/ramps/sandbox-journey.sh:354 |
| <code>interface.8.env-ramps-layerx-ramp-expected-source-digest-j69</code> | notes | read at services/platform/ramps/sandbox-journey.sh:355 |
| <code>interface.8.env-ramps-layerx-ramp-receipt-verifier-binary-j69</code> | notes | read at services/platform/ramps/sandbox-journey.sh:248 |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.277.1</code> | req | 8277.1 |
| <code>acceptance.8.277.1</code> | scenario | Given config.example.json, when its layerx object is parsed with serde_json into LayerxConfig, then parsing succeeds. |
| <code>acceptance.8.277.1</code> | test | services/platform/ramps/toolkit/tests/config_example.rs::config_example_layerx_section_parses (planned) |
| <code>acceptance.8.277.1</code> | qualification | UNRUN |
| <code>req.8277</code> | ac_1 | The layerx object of services/platform/ramps/config.example.json deserialises into LayerxConfig without error. |
| <code>acceptance.8.277.2</code> | req | 8277.2 |
| <code>acceptance.8.277.2</code> | scenario | Given config.example.json, when the paxeer object keys are read, then none of the four retired keys is present. |
| <code>acceptance.8.277.2</code> | test | services/platform/ramps/toolkit/tests/config_example.rs::config_example_has_no_paxeer_rpc_agreement (planned) |
| <code>acceptance.8.277.2</code> | qualification | UNRUN |
| <code>req.8277</code> | ac_2 | The paxeer object of config.example.json holds none of rpc_endpoints, rpc_minimum_agreement, required_confirmations or delayed_after_polls. |
| <code>capability.8.30.4</code> | title | Ship ramp example configuration and container environment for the merged chain |
| <code>capability.8.30.4</code> | reqs | 8277.1<br>8277.2 |

## ramp-dependencies

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.279.1</code> | req | 8279.1 |
| <code>acceptance.8.279.1</code> | scenario | Given the platform workspace, when cargo metadata is read, then neither ramp package lists layerx-paxeer-client or layerx-intents among its dependencies. |
| <code>acceptance.8.279.1</code> | test | services/platform/tests/test_ramp_dependencies.py::test_ramp_crates_do_not_depend_on_paxeer_client (planned) |
| <code>acceptance.8.279.1</code> | qualification | UNRUN |
| <code>req.8279</code> | ac_1 | cargo metadata for the platform workspace lists no layerx-paxeer-client or layerx-intents dependency of layerx-ramp-toolkit or layerx-reference-ramp. |
| <code>acceptance.8.279.2</code> | req | 8279.2 |
| <code>acceptance.8.279.2</code> | scenario | Given a metadata file listing a package named layerx-paxeer-client, when dependency-policy.sh checks it, then it exits non-zero. |
| <code>acceptance.8.279.2</code> | test | services/platform/tests/test_ramp_dependencies.py::test_dependency_policy_bans_paxeer_client (planned) |
| <code>acceptance.8.279.2</code> | qualification | UNRUN |
| <code>req.8279</code> | ac_2 | services/platform/tools/dependency-policy.sh refuses a metadata document whose packages include layerx-paxeer-client. |
| <code>capability.8.31.2</code> | title | Drop the Paxeer client and kernel intent compiler from the ramp crates |
| <code>capability.8.31.2</code> | reqs | 8279.1<br>8279.2 |

## ramp-layerx-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-chain-id-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/clients.rs</code> (planned) | <code>pub chain_id: u64</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-gas-limit-cap-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/clients.rs</code> (planned) | <code>pub gas_limit_cap: u64</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-max-fee-per-gas-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/clients.rs</code> (planned) | <code>pub max_fee_per_gas: u128</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-max-priority-fee-per-gas-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/clients.rs</code> (planned) | <code>pub max_priority_fee_per_gas: u128</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-operator-address-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/clients.rs</code> (planned) | <code>pub operator_address: String</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-receipt-keys-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/clients.rs</code> (planned) | <code>pub receipt_keys: Vec&lt;ReceiptKeyConfig&gt;</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-rpc-endpoint-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/clients.rs</code> (planned) | <code>pub rpc_endpoint: String</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-rpc-trust-anchor-der-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/clients.rs</code> (planned) | <code>pub rpc_trust_anchor_der: Option&lt;PathBuf&gt;</code> |
| <code>interface.8.cfg-ramps-layerx-chain-id-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/config.example.json</code> (planned) | <code>"layerx": { "chain_id": ... }</code> |
| <code>interface.8.cfg-ramps-layerx-gas-limit-cap-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/config.example.json</code> (planned) | <code>"layerx": { "gas_limit_cap": ... }</code> |
| <code>interface.8.cfg-ramps-layerx-max-fee-per-gas-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/config.example.json</code> (planned) | <code>"layerx": { "max_fee_per_gas": ... }</code> |
| <code>interface.8.cfg-ramps-layerx-max-priority-fee-per-gas-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/config.example.json</code> (planned) | <code>"layerx": { "max_priority_fee_per_gas": ... }</code> |
| <code>interface.8.cfg-ramps-layerx-operator-address-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/config.example.json</code> (planned) | <code>"layerx": { "operator_address": ... }</code> |
| <code>interface.8.cfg-ramps-layerx-receipt-keys-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/config.example.json</code> (planned) | <code>"layerx": { "receipt_keys": ... }</code> |
| <code>interface.8.cfg-ramps-layerx-rpc-endpoint-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/config.example.json</code> (planned) | <code>"layerx": { "rpc_endpoint": ... }</code> |
| <code>interface.8.cfg-ramps-layerx-rpc-trust-anchor-der-j41</code> | <code>config_key</code> | <code>new</code> | <code>services/platform/ramps/config.example.json</code> (planned) | <code>"layerx": { "rpc_trust_anchor_der": ... }</code> |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.274.1</code> | req | 8274.1 |
| <code>acceptance.8.274.1</code> | scenario | Given a config valid for the merged chain plus paxeer.rpc_endpoints, when the binary starts, then it exits non-zero before listening. |
| <code>acceptance.8.274.1</code> | test | services/platform/ramps/reference/tests/config_validation.rs::reference_ramp_refuses_retired_paxeer_rpc_fields (planned) |
| <code>acceptance.8.274.1</code> | qualification | UNRUN |
| <code>req.8274</code> | ac_1 | The reference ramp exits non-zero at startup when the paxeer section still holds rpc_endpoints. |
| <code>acceptance.8.274.2</code> | req | 8274.2 |
| <code>acceptance.8.274.2</code> | scenario | Given a config whose operator_address is 0x1234, when the binary starts, then it exits non-zero naming operator_address. |
| <code>acceptance.8.274.2</code> | test | services/platform/ramps/reference/tests/config_validation.rs::reference_ramp_refuses_operator_address_mismatch (planned) |
| <code>acceptance.8.274.2</code> | qualification | UNRUN |
| <code>req.8274</code> | ac_2 | The reference ramp exits non-zero at startup when layerx.operator_address is not a 20-byte hex address. |
| <code>capability.8.30.1</code> | title | Validate the reference ramp configuration for the merged chain |
| <code>capability.8.30.1</code> | reqs | 8274.1<br>8274.2 |

## ramp-reference-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-ramp-reference-main-rs-verify-receipt</code> | <code>cli_command</code> | <code>change</code> | <code>services/platform/ramps/reference/src/main.rs:203</code> | <code>if std::env::args_os().nth(1).as_deref() == Some(std::ffi::OsStr::new("--verify-receipt")) {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-ramp-reference-main-rs-verify-receipt</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-ramp-reference-main-rs-verify-receipt</code> | overridden_by | override.J41.64 |

## ramp-reference-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-ramp-reference-complianceconfig-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:97</code> | <code>endpoint: String,</code> |
| <code>interface.8.cfg-ramp-reference-complianceconfig-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:99</code> | <code>public_key: String,</code> |
| <code>interface.8.cfg-ramp-reference-complianceconfig-service-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:98</code> | <code>service_token_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-reference-config-client-tls</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:54</code> | <code>client_tls: ClientTls,</code> |
| <code>interface.8.cfg-ramp-reference-config-compliance</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:56</code> | <code>compliance: ComplianceConfig,</code> |
| <code>interface.8.cfg-ramp-reference-config-identity</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:55</code> | <code>identity: IdentityConfig,</code> |
| <code>interface.8.cfg-ramp-reference-config-journal-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:46</code> | <code>journal_path: PathBuf,</code> |
| <code>interface.8.cfg-ramp-reference-config-layerx</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:58</code> | <code>layerx: LayerxConfig,</code> |
| <code>interface.8.cfg-ramp-reference-config-lease-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:48</code> | <code>lease_seconds: u64,</code> |
| <code>interface.8.cfg-ramp-reference-config-listen</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:44</code> | <code>listen: String,</code> |
| <code>interface.8.cfg-ramp-reference-config-listener</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:45</code> | <code>listener: Option&lt;String&gt;,</code> |
| <code>interface.8.cfg-ramp-reference-config-operator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:50</code> | <code>operator: OperatorIdentity,</code> |
| <code>interface.8.cfg-ramp-reference-config-operator-control-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:61</code> | <code>operator_control_token_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-reference-config-paxeer</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/ramps/reference/src/main.rs:59</code> | <code>paxeer: PaxeerConfig,</code> |
| <code>interface.8.cfg-ramp-reference-config-provider</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:57</code> | <code>provider: ProviderConfig,</code> |
| <code>interface.8.cfg-ramp-reference-config-provider-callback-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:60</code> | <code>provider_callback_public_key: String,</code> |
| <code>interface.8.cfg-ramp-reference-config-quotes</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:51</code> | <code>quotes: Vec&lt;QuoteTerms&gt;,</code> |
| <code>interface.8.cfg-ramp-reference-config-reconcile-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:49</code> | <code>reconcile_seconds: u64,</code> |
| <code>interface.8.cfg-ramp-reference-config-server-identity-password-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:53</code> | <code>server_identity_password_file: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cfg-ramp-reference-config-server-identity-pkcs12</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:52</code> | <code>server_identity_pkcs12: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cfg-ramp-reference-config-source-settlement</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:62</code> | <code>source_settlement: Option&lt;SourceIntakeConfig&gt;,</code> |
| <code>interface.8.cfg-ramp-reference-config-worker-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:47</code> | <code>worker_id: String,</code> |
| <code>interface.8.cfg-ramp-reference-identityconfig-audience</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:91</code> | <code>audience: String,</code> |
| <code>interface.8.cfg-ramp-reference-identityconfig-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:89</code> | <code>endpoint: String,</code> |
| <code>interface.8.cfg-ramp-reference-identityconfig-service-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:90</code> | <code>service_token_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-broadcast-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:116</code> | <code>broadcast_path: String,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-custody-credential-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:115</code> | <code>custody_credential_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-custody-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:114</code> | <code>custody_endpoint: String,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-delayed-after-polls</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/reference/src/main.rs:128</code> | <code>delayed_after_polls: u64,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-operator-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:118</code> | <code>operator_account: String,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-poll-cadence-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:127</code> | <code>poll_cadence_seconds: u64,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-required-confirmations</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/reference/src/main.rs:126</code> | <code>required_confirmations: u64,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-rpc-chain-id</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/reference/src/main.rs:124</code> | <code>rpc_chain_id: u64,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-rpc-endpoints</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/reference/src/main.rs:122</code> | <code>rpc_endpoints: Vec&lt;String&gt;,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-rpc-minimum-agreement</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/reference/src/main.rs:125</code> | <code>rpc_minimum_agreement: usize,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-rpc-trust-anchor-der</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/reference/src/main.rs:123</code> | <code>rpc_trust_anchor_der: PathBuf,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-signer-key-handle</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:121</code> | <code>signer_key_handle: String,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-status-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:117</code> | <code>status_path: String,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-vault-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:120</code> | <code>vault_id: String,</code> |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-wallet-address</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:119</code> | <code>wallet_address: String,</code> |
| <code>interface.8.cfg-ramp-reference-providerconfig-credential-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:106</code> | <code>credential_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-reference-providerconfig-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:105</code> | <code>endpoint: String,</code> |
| <code>interface.8.cfg-ramp-reference-providerconfig-settlement-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:107</code> | <code>settlement_path: String,</code> |
| <code>interface.8.cfg-ramp-reference-providerconfig-status-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:108</code> | <code>status_path: String,</code> |
| <code>interface.8.cfg-ramp-reference-receiptverifierconfig-client-tls</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:219</code> | <code>client_tls: ClientTls,</code> |
| <code>interface.8.cfg-ramp-reference-receiptverifierconfig-layerx</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:220</code> | <code>layerx: LayerxConfig,</code> |
| <code>interface.8.cfg-ramp-reference-sourceintakeconfig-migration-service-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:68</code> | <code>migration_service_token_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-reference-sourceintakeconfig-verification</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:69</code> | <code>verification: SourceSettlementConfig,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-ramp-reference-complianceconfig-endpoint</code> | notes | 490 callers total |
| <code>interface.8.cfg-ramp-reference-complianceconfig-public-key</code> | notes | 431 callers total |
| <code>interface.8.cfg-ramp-reference-config-identity</code> | notes | 632 callers total |
| <code>interface.8.cfg-ramp-reference-config-layerx</code> | notes | 361 callers total |
| <code>interface.8.cfg-ramp-reference-config-listen</code> | notes | 75 callers total |
| <code>interface.8.cfg-ramp-reference-config-listener</code> | notes | 45 callers total |
| <code>interface.8.cfg-ramp-reference-config-operator</code> | notes | 247 callers total |
| <code>interface.8.cfg-ramp-reference-config-paxeer</code> | notes | 257 callers total |
| <code>interface.8.cfg-ramp-reference-config-paxeer</code> | overridden_by | override.J41.57 |
| <code>interface.8.cfg-ramp-reference-config-provider</code> | notes | 221 callers total |
| <code>interface.8.cfg-ramp-reference-config-quotes</code> | notes | 44 callers total |
| <code>interface.8.cfg-ramp-reference-identityconfig-endpoint</code> | notes | 490 callers total |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-delayed-after-polls</code> | overridden_by | override.J41.58 |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-required-confirmations</code> | overridden_by | override.J41.59 |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-rpc-chain-id</code> | overridden_by | override.J41.60 |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-rpc-endpoints</code> | overridden_by | override.J41.61 |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-rpc-minimum-agreement</code> | overridden_by | override.J41.62 |
| <code>interface.8.cfg-ramp-reference-paxeerconfig-rpc-trust-anchor-der</code> | overridden_by | override.J41.63 |
| <code>interface.8.cfg-ramp-reference-providerconfig-credential-file</code> | notes | 46 callers total |
| <code>interface.8.cfg-ramp-reference-providerconfig-endpoint</code> | notes | 490 callers total |
| <code>interface.8.cfg-ramp-reference-receiptverifierconfig-layerx</code> | notes | 361 callers total |
| <code>interface.8.cfg-ramp-reference-sourceintakeconfig-migration-service-token-file</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ramp-reference-sourceintakeconfig-verification</code> | notes | 346 callers total |

## ramp-reference-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-ramp-reference-layerx-ramp-build-revision</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:764</code> | <code>"build_revision": option_env!("LAYERX_RAMP_BUILD_REVISION"),</code> |
| <code>interface.8.env-ramp-reference-layerx-ramp-build-source-digest</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:765</code> | <code>"build_source_digest": option_env!("LAYERX_RAMP_BUILD_SOURCE_DIGEST"),</code> |

## ramp-reference-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-ramp-reference-authentication-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1225</code> | <code>.map_err(&#124;_&#124; error(401, "authentication_refused"))</code> |
| <code>interface.8.err-ramp-reference-authentication-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1222</code> | <code>.ok_or_else(&#124;&#124; error(401, "authentication_required"))?,</code> |
| <code>interface.8.err-ramp-reference-body-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:723</code> | <code>return Err(error(400, "body_too_large"));</code> |
| <code>interface.8.err-ramp-reference-callback-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:806</code> | <code>serde_json::from_slice(&amp;request.body).map_err(&#124;_&#124; error(400, "callback_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-compliance-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1411</code> | <code>RampError::Compliance =&gt; error(503, "compliance_unavailable"),</code> |
| <code>interface.8.err-ramp-reference-content-length-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:721</code> | <code>.map_err(&#124;_&#124; error(400, "content_length_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-customer-authentication-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:926</code> | <code>.map_err(&#124;_&#124; error(401, "customer_authentication_refused"))?;</code> |
| <code>interface.8.err-ramp-reference-customer-authentication-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:922</code> | <code>.ok_or_else(&#124;&#124; error(401, "customer_authentication_required"))?;</code> |
| <code>interface.8.err-ramp-reference-customer-binding-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:936</code> | <code>return Err(error(400, "customer_binding_invalid"));</code> |
| <code>interface.8.err-ramp-reference-customer-binding-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:930</code> | <code>.ok_or_else(&#124;&#124; error(400, "customer_binding_required"))?;</code> |
| <code>interface.8.err-ramp-reference-duplicate-header</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:715</code> | <code>return Err(error(400, "duplicate_header"));</code> |
| <code>interface.8.err-ramp-reference-headers-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:699</code> | <code>std::str::from_utf8(&amp;bytes[..boundary]).map_err(&#124;_&#124; error(400, "headers_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-headers-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:691</code> | <code>return Err(error(400, "headers_too_large"));</code> |
| <code>interface.8.err-ramp-reference-idempotency-key-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:855</code> | <code>parse_hex32(idempotency).map_err(&#124;_&#124; error(400, "idempotency_key_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-journal-busy</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:781</code> | <code>.map_err(&#124;_&#124; error(503, "journal_busy"))?;</code> |
| <code>interface.8.err-ramp-reference-journal-recovery-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:795</code> | <code>return Err(error(503, "journal_recovery_required"));</code> |
| <code>interface.8.err-ramp-reference-journal-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:810</code> | <code>.map_err(&#124;_&#124; error(503, "journal_unavailable"))?;</code> |
| <code>interface.8.err-ramp-reference-layerx-binding-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1409</code> | <code>error(400, "layerx_binding_refused")</code> |
| <code>interface.8.err-ramp-reference-layerx-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1413</code> | <code>RampError::Layerx &#124; RampError::Receipt(_) =&gt; error(503, "layerx_unavailable"),</code> |
| <code>interface.8.err-ramp-reference-migration-authentication-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:917</code> | <code>return Err(error(403, "migration_authentication_refused"));</code> |
| <code>interface.8.err-ramp-reference-migration-authentication-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:910</code> | <code>.ok_or_else(&#124;&#124; error(401, "migration_authentication_required"))?;</code> |
| <code>interface.8.err-ramp-reference-native-payer-grant-authorization-required</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/ramps/reference/src/main.rs:1050</code> | <code>return Err(error(400, "native_payer_grant_authorization_required"));</code> |
| <code>interface.8.err-ramp-reference-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:822</code> | <code>return Err(error(404, "not_found"));</code> |
| <code>interface.8.err-ramp-reference-operation-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1406</code> | <code>error(409, "operation_conflict")</code> |
| <code>interface.8.err-ramp-reference-operator-authentication-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1240</code> | <code>return Err(error(403, "operator_authentication_refused"));</code> |
| <code>interface.8.err-ramp-reference-operator-authentication-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1233</code> | <code>.ok_or_else(&#124;&#124; error(401, "operator_authentication_required"))?;</code> |
| <code>interface.8.err-ramp-reference-order-digest-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:825</code> | <code>let digest = parse_hex32(digest).map_err(&#124;_&#124; error(400, "order_digest_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-order-id-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:884</code> | <code>return Err(error(409, "order_id_conflict"));</code> |
| <code>interface.8.err-ramp-reference-order-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:871</code> | <code>serde_json::from_slice(&amp;request.body).map_err(&#124;_&#124; error(400, "order_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-order-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:832</code> | <code>.ok_or_else(&#124;&#124; error(404, "order_not_found"))?;</code> |
| <code>interface.8.err-ramp-reference-paxeer-tracker-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/ramps/reference/src/main.rs:1137</code> | <code>.map_err(&#124;_&#124; error(503, "paxeer_tracker_unavailable"))?;</code> |
| <code>interface.8.err-ramp-reference-paxeer-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1414</code> | <code>RampError::Paxeer =&gt; error(503, "paxeer_unavailable"),</code> |
| <code>interface.8.err-ramp-reference-provider-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1412</code> | <code>RampError::Provider =&gt; error(503, "provider_unavailable"),</code> |
| <code>interface.8.err-ramp-reference-quote-binding-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:958</code> | <code>return Err(error(409, "quote_binding_conflict"));</code> |
| <code>interface.8.err-ramp-reference-quote-expired</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:993</code> | <code>return Err(error(409, "quote_expired"));</code> |
| <code>interface.8.err-ramp-reference-quote-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:892</code> | <code>.ok_or_else(&#124;&#124; error(404, "quote_not_found"))?;</code> |
| <code>interface.8.err-ramp-reference-rebalance-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1101</code> | <code>serde_json::from_slice(&amp;request.body).map_err(&#124;_&#124; error(400, "rebalance_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-rebalance-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:862</code> | <code>.ok_or_else(&#124;&#124; error(404, "rebalance_not_found"))?;</code> |
| <code>interface.8.err-ramp-reference-rebalance-transaction-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1150</code> | <code>return Err(error(409, "rebalance_transaction_conflict"));</code> |
| <code>interface.8.err-ramp-reference-request-incomplete</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:687</code> | <code>return Err(error(400, "request_incomplete"));</code> |
| <code>interface.8.err-ramp-reference-request-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:701</code> | <code>let request_line = lines.next().ok_or_else(&#124;&#124; error(400, "request_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-request-read-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:685</code> | <code>.map_err(&#124;_&#124; error(400, "request_read_failed"))?;</code> |
| <code>interface.8.err-ramp-reference-request-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1403</code> | <code>error(400, "request_refused")</code> |
| <code>interface.8.err-ramp-reference-request-timeout</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:678</code> | <code>.map_err(&#124;_&#124; error(503, "request_timeout"))?;</code> |
| <code>interface.8.err-ramp-reference-service-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1415</code> | <code>RampError::Journal &#124; RampError::Configuration =&gt; error(503, "service_unavailable"),</code> |
| <code>interface.8.err-ramp-reference-source-claim-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:962</code> | <code>return Err(error(409, "source_claim_conflict"));</code> |
| <code>interface.8.err-ramp-reference-source-settlement-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:939</code> | <code>.map_err(&#124;_&#124; error(400, "source_settlement_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-source-settlement-not-admitted</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:990</code> | <code>return Err(error(409, "source_settlement_not_admitted"));</code> |
| <code>interface.8.err-ramp-reference-source-settlement-unconfigured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:905</code> | <code>.ok_or_else(&#124;&#124; error(503, "source_settlement_unconfigured"))?;</code> |
| <code>interface.8.err-ramp-reference-source-state-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:979</code> | <code>_ =&gt; return Err(error(409, "source_state_conflict")),</code> |
| <code>interface.8.err-ramp-reference-transaction-hash-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1133</code> | <code>.map_err(&#124;_&#124; error(400, "transaction_hash_invalid"))?;</code> |
| <code>interface.8.err-ramp-reference-work-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1030</code> | <code>serde_json::from_slice(&amp;request.body).map_err(&#124;_&#124; error(400, "work_invalid"))?;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-ramp-reference-authentication-refused</code> | notes | HTTP 401; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-authentication-required</code> | notes | HTTP 401 |
| <code>interface.8.err-ramp-reference-body-too-large</code> | notes | HTTP 400 |
| <code>interface.8.err-ramp-reference-callback-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-compliance-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-content-length-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-customer-authentication-refused</code> | notes | HTTP 401; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-customer-authentication-required</code> | notes | HTTP 401; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-customer-binding-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-customer-binding-required</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-duplicate-header</code> | notes | HTTP 400 |
| <code>interface.8.err-ramp-reference-headers-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-headers-too-large</code> | notes | HTTP 400 |
| <code>interface.8.err-ramp-reference-idempotency-key-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-journal-busy</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-journal-recovery-required</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-journal-unavailable</code> | notes | HTTP 503; 36 callers total |
| <code>interface.8.err-ramp-reference-layerx-binding-refused</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-layerx-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-migration-authentication-refused</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-migration-authentication-required</code> | notes | HTTP 401; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-native-payer-grant-authorization-required</code> | notes | HTTP 400 |
| <code>interface.8.err-ramp-reference-native-payer-grant-authorization-required</code> | overridden_by | override.J41.65 |
| <code>interface.8.err-ramp-reference-not-found</code> | notes | HTTP 404; 134 callers total |
| <code>interface.8.err-ramp-reference-operation-conflict</code> | notes | HTTP 409 |
| <code>interface.8.err-ramp-reference-operator-authentication-refused</code> | notes | HTTP 403 |
| <code>interface.8.err-ramp-reference-operator-authentication-required</code> | notes | HTTP 401 |
| <code>interface.8.err-ramp-reference-order-digest-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-order-id-conflict</code> | notes | HTTP 409 |
| <code>interface.8.err-ramp-reference-order-invalid</code> | notes | HTTP 400 |
| <code>interface.8.err-ramp-reference-order-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-ramp-reference-paxeer-tracker-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-paxeer-tracker-unavailable</code> | overridden_by | override.J41.67 |
| <code>interface.8.err-ramp-reference-paxeer-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-ramp-reference-provider-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-ramp-reference-quote-binding-conflict</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-quote-expired</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-quote-not-found</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-rebalance-invalid</code> | notes | HTTP 400 |
| <code>interface.8.err-ramp-reference-rebalance-not-found</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-rebalance-transaction-conflict</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-request-incomplete</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-request-invalid</code> | notes | HTTP 400 |
| <code>interface.8.err-ramp-reference-request-read-failed</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-request-refused</code> | notes | HTTP 400 |
| <code>interface.8.err-ramp-reference-request-timeout</code> | notes | HTTP 503 |
| <code>interface.8.err-ramp-reference-service-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-source-claim-conflict</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-source-settlement-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-source-settlement-not-admitted</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-source-settlement-unconfigured</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-source-state-conflict</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-transaction-hash-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-ramp-reference-work-invalid</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |

## ramp-reference-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-ramp-reference-any-v1-orders</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:820</code> | <code>if let Some(digest) = request.path.strip_prefix("/v1/orders/") {</code> |
| <code>interface.8.http-ramp-reference-get-internal-v1-journal</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:776</code> | <code>if request.path == "/internal/v1/journal" &amp;&amp; request.method == "GET" {</code> |
| <code>interface.8.http-ramp-reference-get-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:746</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/livez" {</code> |
| <code>interface.8.http-ramp-reference-get-readyz</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/ramps/reference/src/main.rs:749</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/readyz" {</code> |
| <code>interface.8.http-ramp-reference-post-internal-v1-journal-recover</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:784</code> | <code>if request.path == "/internal/v1/journal/recover" &amp;&amp; request.method == "POST" {</code> |
| <code>interface.8.http-ramp-reference-post-internal-v1-rebalances</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/ramps/reference/src/main.rs:846</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/internal/v1/rebalances" {</code> |
| <code>interface.8.http-ramp-reference-post-internal-v1-rebalances-2</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:849</code> | <code>if let Some(idempotency) = request.path.strip_prefix("/internal/v1/rebalances/") {</code> |
| <code>interface.8.http-ramp-reference-post-internal-v1-work</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/ramps/reference/src/main.rs:843</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/internal/v1/work" {</code> |
| <code>interface.8.http-ramp-reference-post-internal-v2-source-settlements</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:801</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/internal/v2/source-settlements" {</code> |
| <code>interface.8.http-ramp-reference-post-v1-orders</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:798</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/v1/orders" {</code> |
| <code>interface.8.http-ramp-reference-post-v1-provider-callbacks</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:804</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/v1/provider-callbacks" {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-ramp-reference-get-livez</code> | notes | 119 callers total |
| <code>interface.8.http-ramp-reference-get-readyz</code> | notes | 324 callers total |
| <code>interface.8.http-ramp-reference-get-readyz</code> | overridden_by | override.J41.68 |
| <code>interface.8.http-ramp-reference-post-internal-v1-rebalances</code> | overridden_by | override.J41.69 |
| <code>interface.8.http-ramp-reference-post-internal-v1-work</code> | overridden_by | override.J41.66 |

## ramp-reference-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-ramp-reference-main-fn-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/reference/src/main.rs:1420</code> | <code>pub const fn platform_reference_ramp() -&gt; &amp;'static str {</code> |

## ramp-toolkit-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-activity-id</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:55</code> | <code>&#124;&#124; args[4] != "--activity-id" &#124;&#124; args[6] != "--output-file" { return Err(2); }</code> |
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-config-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:54</code> | <code>if args.len() != 8 &#124;&#124; args[0] != "--config-file" &#124;&#124; args[2] != "--order-file"</code> |
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-order-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:54</code> | <code>if args.len() != 8 &#124;&#124; args[0] != "--config-file" &#124;&#124; args[2] != "--order-file"</code> |
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-output-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:55</code> | <code>&#124;&#124; args[4] != "--activity-id" &#124;&#124; args[6] != "--output-file" { return Err(2); }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-activity-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-activity-id</code> | overridden_by | override.J41.50 |
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-config-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-order-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-output-file</code> | notes | no in-repo invocation outside the defining file |

## ramp-toolkit-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-ramp-toolkit-config-client-tls</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:28</code> | <code>client_tls: TlsConfig,</code> |
| <code>interface.8.cfg-ramp-toolkit-config-layerx</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:29</code> | <code>layerx: LayerxConfig,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-actor-did</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:883</code> | <code>pub actor_did: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-authority-token-file</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:881</code> | <code>pub authority_token_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-fee-limit</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:886</code> | <code>pub fee_limit: u128,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-gateway-endpoint</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:877</code> | <code>pub gateway_endpoint: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-gateway-key-file</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:880</code> | <code>pub gateway_key_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-network-id</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:885</code> | <code>pub network_id: u32,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-protocol-version</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:884</code> | <code>pub protocol_version: u16,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-receipt-authority-endpoint</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:878</code> | <code>pub receipt_authority_endpoint: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-first-batch</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:890</code> | <code>pub sequencer_first_batch: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-id</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:888</code> | <code>pub sequencer_id: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-last-batch</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:891</code> | <code>pub sequencer_last_batch: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-public-key</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:889</code> | <code>pub sequencer_public_key: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-signer-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:879</code> | <code>pub signer_endpoint: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-signer-public-key</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:887</code> | <code>pub signer_public_key: String,</code> |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-signer-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:882</code> | <code>pub signer_token_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-toolkit-sourcesettlementconfig-ethereum</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:35</code> | <code>pub ethereum: Option&lt;EthereumConfig&gt;,</code> |
| <code>interface.8.cfg-ramp-toolkit-sourcesettlementconfig-quotes</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:37</code> | <code>pub quotes: Vec&lt;SourceQuoteBinding&gt;,</code> |
| <code>interface.8.cfg-ramp-toolkit-sourcesettlementconfig-solana</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:36</code> | <code>pub solana: Option&lt;SolanaConfig&gt;,</code> |
| <code>interface.8.cfg-ramp-toolkit-tlsconfig-ca-pem</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:19</code> | <code>ca_pem: PathBuf,</code> |
| <code>interface.8.cfg-ramp-toolkit-tlsconfig-identity-password-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:21</code> | <code>identity_password_file: PathBuf,</code> |
| <code>interface.8.cfg-ramp-toolkit-tlsconfig-identity-pkcs12</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:20</code> | <code>identity_pkcs12: PathBuf,</code> |
| <code>interface.8.cfg-ramp-toolkit-tlsconfig-timeout-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:22</code> | <code>timeout_seconds: u64,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-ramp-toolkit-config-layerx</code> | notes | 361 callers total |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-authority-token-file</code> | overridden_by | override.J41.35 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-fee-limit</code> | notes | 88 callers total |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-fee-limit</code> | overridden_by | override.J41.36 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-gateway-endpoint</code> | overridden_by | override.J41.37 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-gateway-key-file</code> | overridden_by | override.J41.38 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-network-id</code> | notes | 336 callers total |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-network-id</code> | overridden_by | override.J41.39 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-protocol-version</code> | notes | 171 callers total |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-protocol-version</code> | overridden_by | override.J41.40 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-receipt-authority-endpoint</code> | overridden_by | override.J41.41 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-first-batch</code> | overridden_by | override.J41.42 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-id</code> | notes | 139 callers total |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-id</code> | overridden_by | override.J41.43 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-last-batch</code> | overridden_by | override.J41.44 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-public-key</code> | notes | 219 callers total |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-sequencer-public-key</code> | overridden_by | override.J41.45 |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-signer-public-key</code> | notes | 34 callers total |
| <code>interface.8.cfg-ramp-toolkit-layerxconfig-signer-public-key</code> | overridden_by | override.J41.46 |
| <code>interface.8.cfg-ramp-toolkit-sourcesettlementconfig-ethereum</code> | notes | 97 callers total |
| <code>interface.8.cfg-ramp-toolkit-sourcesettlementconfig-quotes</code> | notes | 44 callers total |
| <code>interface.8.cfg-ramp-toolkit-sourcesettlementconfig-solana</code> | notes | 81 callers total |
| <code>interface.8.cfg-ramp-toolkit-tlsconfig-timeout-seconds</code> | notes | 635 callers total |

## ramp-toolkit-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-ramp-toolkit-layerx-ramp-build-revision</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:97</code> | <code>"build_revision": option_env!("LAYERX_RAMP_BUILD_REVISION"),</code> |
| <code>interface.8.env-ramp-toolkit-layerx-ramp-build-source-digest</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs:98</code> | <code>"build_source_digest": option_env!("LAYERX_RAMP_BUILD_SOURCE_DIGEST")</code> |

## ramp-toolkit-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-ramp-toolkit-journal</code> | <code>file_format</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/journal.rs:12</code> | <code>const JOURNAL_DOMAIN: &amp;[u8] = b"LXP/market-maker-ramp/journal/v1\0";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-ramp-toolkit-journal</code> | notes | ramp journal record domain |
| <code>interface.8.fmt-ramp-toolkit-journal</code> | overridden_by | override.J41.51 |

## ramp-toolkit-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-ramp-toolkit-clients-activityconfig-j41</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:795</code> | <code>pub struct ActivityConfig {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-callback-evidence-digest-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:1637</code> | <code>pub fn callback_evidence_digest(callback: &amp;ProviderCallback) -&gt; Result&lt;[u8; 32], RampError&gt; {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-compliancedecision-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:404</code> | <code>pub struct ComplianceDecision {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-complianceoutcome-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:418</code> | <code>pub enum ComplianceOutcome {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-configured-sequencer-j41</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:1649</code> | <code>pub fn configured_sequencer(</code> |
| <code>interface.8.rs-ramp-toolkit-clients-decode-hex-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:1533</code> | <code>pub fn decode_hex(value: &amp;str, maximum: usize) -&gt; Result&lt;Vec&lt;u8&gt;, RampError&gt; {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-httprequest-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:131</code> | <code>pub struct HttpRequest&lt;'a&gt; {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-httpresponse-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:279</code> | <code>pub struct HttpResponse {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-maintainedbatchdocument-j41</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/clients.rs:1672</code> | <code>pub struct MaintainedBatchDocument {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-paxeersubmission-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/clients.rs:1370</code> | <code>pub struct PaxeerSubmission {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-preparedlayerx-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/clients.rs:986</code> | <code>pub struct PreparedLayerx {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-providerresult-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:590</code> | <code>pub struct ProviderResult {</code> |
| <code>interface.8.rs-ramp-toolkit-clients-providerstate-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:579</code> | <code>pub enum ProviderState {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-callbackidentity-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:251</code> | <code>pub struct CallbackIdentity {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-event-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:117</code> | <code>pub enum Event {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-journalhealth-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:756</code> | <code>pub struct JournalHealth {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-ordersnapshot-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:180</code> | <code>pub struct OrderSnapshot {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-paxeerobservation-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/journal.rs:65</code> | <code>pub struct PaxeerObservation&lt;'a&gt; {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-projection-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:258</code> | <code>pub struct Projection {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-providercallbackwrite-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:54</code> | <code>pub struct ProviderCallbackWrite&lt;'a&gt; {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-stagedcallback-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:627</code> | <code>pub struct StagedCallback {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-torntail-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:738</code> | <code>pub struct TornTail {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-transitionevidence-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/journal.rs:43</code> | <code>pub struct TransitionEvidence {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-writefault-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:732</code> | <code>pub enum WriteFault {</code> |
| <code>interface.8.rs-ramp-toolkit-journal-writestep-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:713</code> | <code>pub enum WriteStep {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-authenticatedprincipal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:41</code> | <code>pub struct AuthenticatedPrincipal {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-complianceclient</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:470</code> | <code>pub struct ComplianceClient {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-endpoint</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:78</code> | <code>pub struct Endpoint {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:1513</code> | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-identityclient</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:477</code> | <code>pub struct IdentityClient {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-layerxclient</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/clients.rs:952</code> | <code>pub struct LayerxClient {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-layerxconfig</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/clients.rs:876</code> | <code>pub struct LayerxConfig {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-layerxsubmission-verified</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/clients.rs:965</code> | <code>pub enum LayerxSubmission {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-mutualtlsclient</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:140</code> | <code>pub struct MutualTlsClient {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-mutualtlsfiles</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:71</code> | <code>pub struct MutualTlsFiles {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-parse-hex32</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:1525</code> | <code>pub fn parse_hex32(value: &amp;str) -&gt; Result&lt;[u8; 32], RampError&gt; {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-paxeercustodyclient</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/clients.rs:1381</code> | <code>pub struct PaxeerCustodyClient {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-providercallback</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:767</code> | <code>pub struct ProviderCallback {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-providerclient</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:646</code> | <code>pub struct ProviderClient {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-secretfile</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/clients.rs:35</code> | <code>pub struct SecretFile(PathBuf);</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-verify-recovery-settlement</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/clients.rs:2369</code> | <code>pub fn verify_recovery_settlement(</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-compliance-contract-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:25</code> | <code>pub const COMPLIANCE_CONTRACT_VERSION: &amp;str = "layerx-ramp-compliance-v1";</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-createorder</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:110</code> | <code>pub struct CreateOrder {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-engine-inventoryrebalancer</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/engine.rs:406</code> | <code>pub struct InventoryRebalancer&lt;'a&gt; {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-engine-rampengine</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/engine.rs:14</code> | <code>pub struct RampEngine&lt;'a&gt; {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-external-custody-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:21</code> | <code>pub const EXTERNAL_CUSTODY_LABEL: &amp;str =</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-journal-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:767</code> | <code>pub struct Journal {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-journal-paxeersnapshot</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/journal.rs:188</code> | <code>pub struct PaxeerSnapshot {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-journal-workflowstage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/journal.rs:18</code> | <code>pub enum WorkflowStage {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-source-error-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:200</code> | <code>pub const fn source_error_status(error: SourceVerificationError) -&gt; u16 {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-source-settlement-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:14</code> | <code>pub const SOURCE_SETTLEMENT_VERSION: &amp;str = "layerx-migration-source-v2";</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-source-trace</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:194</code> | <code>pub fn source_trace(header: Option&lt;&amp;str&gt;, order_digest: [u8; 32]) -&gt; TraceId {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourcechainkind-ethereum</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:19</code> | <code>pub enum SourceChainKind {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourcesettlementconfig</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:34</code> | <code>pub struct SourceSettlementConfig {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourcesettlementrequest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:42</code> | <code>pub struct SourceSettlementRequest {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourcesettlementresult</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:61</code> | <code>pub struct SourceSettlementResult {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourcesettlementservice</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:213</code> | <code>pub struct SourceSettlementService {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourcesettlementstate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:51</code> | <code>pub enum SourceSettlementState {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourceverificationerror-sourcepending</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:12</code> | <code>pub use layerx_migrate::MigrationError as SourceVerificationError;</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-operatoridentity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:48</code> | <code>pub struct OperatorIdentity {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-paxeer-contract-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:26</code> | <code>pub const PAXEER_CONTRACT_VERSION: &amp;str = "layerx-ramp-paxeer-v1";</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-platform-ramp-toolkit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:506</code> | <code>pub const fn platform_ramp_toolkit() -&gt; &amp;'static str {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-provider-contract-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:24</code> | <code>pub const PROVIDER_CONTRACT_VERSION: &amp;str = "layerx-ramp-provider-v1";</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-quoteterms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:64</code> | <code>pub struct QuoteTerms {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-rampdirection</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:34</code> | <code>pub enum RampDirection {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-ramperror</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:485</code> | <code>pub enum RampError {</code> |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-ramporder</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:118</code> | <code>pub struct RampOrder {</code> |
| <code>interface.8.rs-ramp-toolkit-lib-aggregatestatus-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:463</code> | <code>pub enum AggregateStatus {</code> |
| <code>interface.8.rs-ramp-toolkit-lib-compile-operator-send-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/lib.rs:349</code> | <code>pub fn compile_operator_send(</code> |
| <code>interface.8.rs-ramp-toolkit-lib-compile-payer-grant-draw-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/lib.rs:279</code> | <code>pub fn compile_payer_grant_draw(</code> |
| <code>interface.8.rs-ramp-toolkit-lib-compiledpayment-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/lib.rs:269</code> | <code>pub struct CompiledPayment {</code> |
| <code>interface.8.rs-ramp-toolkit-lib-operator-send-authorization-message-j41</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/ramps/toolkit/src/lib.rs:306</code> | <code>pub fn operator_send_authorization_message(</code> |
| <code>interface.8.rs-ramp-toolkit-lib-order-digest-domain-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:23</code> | <code>pub const ORDER_DIGEST_DOMAIN: &amp;[u8] = b"LXP/market-maker-ramp/order/v1\0";</code> |
| <code>interface.8.rs-ramp-toolkit-lib-ramppresentation-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/lib.rs:473</code> | <code>pub struct RampPresentation {</code> |
| <code>interface.8.rs-ramp-toolkit-lib-receiptevidence-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/lib.rs:393</code> | <code>pub struct ReceiptEvidence {</code> |
| <code>interface.8.rs-ramp-toolkit-lib-verifiedlayerxleg-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/lib.rs:400</code> | <code>pub struct VerifiedLayerxLeg {</code> |
| <code>interface.8.rs-ramp-toolkit-lib-verify-order-receipt-j41</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/ramps/toolkit/src/lib.rs:409</code> | <code>pub fn verify_order_receipt(</code> |
| <code>interface.8.rs-ramp-toolkit-migration-sourcequotebinding-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:26</code> | <code>pub struct SourceQuoteBinding {</code> |
| <code>interface.8.rs-ramp-toolkit-migration-sourcesettlementrecordv2-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:71</code> | <code>pub struct SourceSettlementRecordV2 {</code> |
| <code>interface.8.rs-ramp-toolkit-migration-verifiedsourcesettlement-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/ramps/toolkit/src/migration.rs:115</code> | <code>pub struct VerifiedSourceSettlement(SourceSettlementRecordV2);</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-layerxclient</code> | overridden_by | override.J41.47 |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-layerxconfig</code> | overridden_by | override.J41.48 |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-layerxsubmission-verified</code> | notes | Verified is a variant or associated item of LayerxSubmission |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-layerxsubmission-verified</code> | overridden_by | override.J41.49 |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-paxeercustodyclient</code> | overridden_by | override.J41.55 |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-clients-verify-recovery-settlement</code> | overridden_by | override.J41.56 |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-engine-inventoryrebalancer</code> | overridden_by | override.J41.53 |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-engine-rampengine</code> | overridden_by | override.J41.52 |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-journal-paxeersnapshot</code> | overridden_by | override.J41.54 |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourcechainkind-ethereum</code> | notes | Ethereum is a variant or associated item of SourceChainKind |
| <code>interface.8.rs-ramp-toolkit-layerx-ramp-toolkit-migration-sourceverificationerror-sourcepending</code> | notes | SourcePending is a variant or associated item of SourceVerificationError |

## ramp-verify-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-ramp-toolkit-ramp-verify-receipt-rs-transaction-hash-j41</code> | <code>cli_command</code> | <code>new</code> | <code>services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs</code> (planned) | <code>ramp-verify-receipt --config-file F --order-file F --transaction-hash 0xHASH --output-file F</code> |

## ramps-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-ramps-client-tls</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:98</code> | <code>"client_tls": {</code> |
| <code>interface.8.cfg-ramps-client-tls-ca-pem</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:99</code> | <code>"ca_pem": "/run/secrets/outbound-ca.pem",</code> |
| <code>interface.8.cfg-ramps-client-tls-identity-password-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:101</code> | <code>"identity_password_file": "/run/secrets/outbound-identity-password",</code> |
| <code>interface.8.cfg-ramps-client-tls-identity-pkcs12</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:100</code> | <code>"identity_pkcs12": "/run/secrets/outbound-identity.p12",</code> |
| <code>interface.8.cfg-ramps-client-tls-timeout-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:102</code> | <code>"timeout_seconds": 8</code> |
| <code>interface.8.cfg-ramps-compliance</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:109</code> | <code>"compliance": {</code> |
| <code>interface.8.cfg-ramps-compliance-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:105</code> | <code>"endpoint": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-ramps-compliance-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:112</code> | <code>"public_key": "REPLACE_64_HEX_ED25519_PUBLIC_KEY"</code> |
| <code>interface.8.cfg-ramps-compliance-service-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:106</code> | <code>"service_token_file": "/run/secrets/identity-token",</code> |
| <code>interface.8.cfg-ramps-identity</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:104</code> | <code>"identity": {</code> |
| <code>interface.8.cfg-ramps-identity-audience</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:107</code> | <code>"audience": "layerx-ramp"</code> |
| <code>interface.8.cfg-ramps-identity-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:105</code> | <code>"endpoint": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-ramps-identity-service-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:106</code> | <code>"service_token_file": "/run/secrets/identity-token",</code> |
| <code>interface.8.cfg-ramps-journal-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:3</code> | <code>"journal_path": "/var/lib/layerx-ramp/journal.jsonl",</code> |
| <code>interface.8.cfg-ramps-layerx</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:120</code> | <code>"layerx": {</code> |
| <code>interface.8.cfg-ramps-layerx-actor-did</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:125</code> | <code>"actor_did": "did:layerx:market-maker-eu-1",</code> |
| <code>interface.8.cfg-ramps-layerx-fee-limit</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:128</code> | <code>"fee_limit": 1000,</code> |
| <code>interface.8.cfg-ramps-layerx-gateway-endpoint</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:121</code> | <code>"gateway_endpoint": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-ramps-layerx-gateway-key-file</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:123</code> | <code>"gateway_key_file": "/run/secrets/gateway-key",</code> |
| <code>interface.8.cfg-ramps-layerx-network-id</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:127</code> | <code>"network_id": 1,</code> |
| <code>interface.8.cfg-ramps-layerx-protocol-version</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:126</code> | <code>"protocol_version": 1,</code> |
| <code>interface.8.cfg-ramps-layerx-signer-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:122</code> | <code>"signer_endpoint": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-ramps-layerx-signer-public-key</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:129</code> | <code>"signer_public_key": "REPLACE_64_HEX_ED25519_PUBLIC_KEY"</code> |
| <code>interface.8.cfg-ramps-layerx-signer-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:124</code> | <code>"signer_token_file": "/run/secrets/kms-token",</code> |
| <code>interface.8.cfg-ramps-lease-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:5</code> | <code>"lease_seconds": 60,</code> |
| <code>interface.8.cfg-ramps-listen</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:2</code> | <code>"listen": "&lt;addr&gt;",</code> |
| <code>interface.8.cfg-ramps-operator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:7</code> | <code>"operator": {</code> |
| <code>interface.8.cfg-ramps-operator-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:9</code> | <code>"account": "agent:did:layerx:market-maker-eu-1:main",</code> |
| <code>interface.8.cfg-ramps-operator-control-token-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:152</code> | <code>"operator_control_token_file": "/run/secrets/operator-control-token"</code> |
| <code>interface.8.cfg-ramps-operator-principal-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:8</code> | <code>"principal_id": "market-maker-eu-1",</code> |
| <code>interface.8.cfg-ramps-operator-signer-key-handle</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:10</code> | <code>"signer_key_handle": "&lt;url&gt;"</code> |
| <code>interface.8.cfg-ramps-paxeer</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:131</code> | <code>"paxeer": {</code> |
| <code>interface.8.cfg-ramps-paxeer-broadcast-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:134</code> | <code>"broadcast_path": "/layerx-paxeer-v1/rebalances",</code> |
| <code>interface.8.cfg-ramps-paxeer-custody-credential-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:133</code> | <code>"custody_credential_file": "/run/secrets/paxeer-custody-token",</code> |
| <code>interface.8.cfg-ramps-paxeer-custody-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:132</code> | <code>"custody_endpoint": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-ramps-paxeer-delayed-after-polls</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:149</code> | <code>"delayed_after_polls": 12</code> |
| <code>interface.8.cfg-ramps-paxeer-operator-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:136</code> | <code>"operator_account": "agent:did:layerx:market-maker-eu-1:main",</code> |
| <code>interface.8.cfg-ramps-paxeer-poll-cadence-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:148</code> | <code>"poll_cadence_seconds": 5,</code> |
| <code>interface.8.cfg-ramps-paxeer-required-confirmations</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:147</code> | <code>"required_confirmations": 12,</code> |
| <code>interface.8.cfg-ramps-paxeer-rpc-chain-id</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:145</code> | <code>"rpc_chain_id": 0,</code> |
| <code>interface.8.cfg-ramps-paxeer-rpc-endpoints</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:140</code> | <code>"rpc_endpoints": [</code> |
| <code>interface.8.cfg-ramps-paxeer-rpc-minimum-agreement</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:146</code> | <code>"rpc_minimum_agreement": 2,</code> |
| <code>interface.8.cfg-ramps-paxeer-rpc-trust-anchor-der</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/ramps/config.example.json:144</code> | <code>"rpc_trust_anchor_der": "/run/secrets/paxeer-rpc-ca.der",</code> |
| <code>interface.8.cfg-ramps-paxeer-signer-key-handle</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:10</code> | <code>"signer_key_handle": "&lt;url&gt;"</code> |
| <code>interface.8.cfg-ramps-paxeer-status-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:118</code> | <code>"status_path": "/layerx-ramp-v1/settlements"</code> |
| <code>interface.8.cfg-ramps-paxeer-vault-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:138</code> | <code>"vault_id": "REPLACE_OPERATOR_VAULT",</code> |
| <code>interface.8.cfg-ramps-paxeer-wallet-address</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:137</code> | <code>"wallet_address": "REPLACE_OPERATOR_WALLET",</code> |
| <code>interface.8.cfg-ramps-provider</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:114</code> | <code>"provider": {</code> |
| <code>interface.8.cfg-ramps-provider-callback-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:151</code> | <code>"provider_callback_public_key": "REPLACE_64_HEX_ED25519_PUBLIC_KEY",</code> |
| <code>interface.8.cfg-ramps-provider-credential-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:116</code> | <code>"credential_file": "/run/secrets/provider-token",</code> |
| <code>interface.8.cfg-ramps-provider-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:105</code> | <code>"endpoint": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-ramps-provider-settlement-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:117</code> | <code>"settlement_path": "/layerx-ramp-v1/settlements",</code> |
| <code>interface.8.cfg-ramps-provider-status-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:118</code> | <code>"status_path": "/layerx-ramp-v1/settlements"</code> |
| <code>interface.8.cfg-ramps-quotes</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:12</code> | <code>"quotes": [</code> |
| <code>interface.8.cfg-ramps-quotes-context</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:57</code> | <code>"context": [</code> |
| <code>interface.8.cfg-ramps-quotes-direction</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:15</code> | <code>"direction": "on_ramp",</code> |
| <code>interface.8.cfg-ramps-quotes-expires-at</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:93</code> | <code>"expires_at": 2000000000</code> |
| <code>interface.8.cfg-ramps-quotes-external-amount-minor</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:52</code> | <code>"external_amount_minor": 100,</code> |
| <code>interface.8.cfg-ramps-quotes-external-currency</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:51</code> | <code>"external_currency": "EUR",</code> |
| <code>interface.8.cfg-ramps-quotes-fee-minor</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:55</code> | <code>"fee_minor": 2,</code> |
| <code>interface.8.cfg-ramps-quotes-layerx-amount</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:50</code> | <code>"layerx_amount": 1000000,</code> |
| <code>interface.8.cfg-ramps-quotes-layerx-asset</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:16</code> | <code>"layerx_asset": [</code> |
| <code>interface.8.cfg-ramps-quotes-maximum-slippage-bps</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:56</code> | <code>"maximum_slippage_bps": 25,</code> |
| <code>interface.8.cfg-ramps-quotes-payout-token</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:92</code> | <code>"payout_token": "REPLACE_TOKENIZED_BENEFICIARY",</code> |
| <code>interface.8.cfg-ramps-quotes-provider-token</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:91</code> | <code>"provider_token": "REPLACE_PROVIDER_PRODUCT_TOKEN",</code> |
| <code>interface.8.cfg-ramps-quotes-quote-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:14</code> | <code>"quote_id": "REPLACE_ON_RAMP_QUOTE",</code> |
| <code>interface.8.cfg-ramps-quotes-rate-denominator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:54</code> | <code>"rate_denominator": 100,</code> |
| <code>interface.8.cfg-ramps-quotes-rate-numerator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:53</code> | <code>"rate_numerator": 1000000,</code> |
| <code>interface.8.cfg-ramps-reconcile-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:6</code> | <code>"reconcile_seconds": 5,</code> |
| <code>interface.8.cfg-ramps-server-identity-password-file</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:97</code> | <code>"server_identity_password_file": "/run/secrets/server-identity-password",</code> |
| <code>interface.8.cfg-ramps-server-identity-pkcs12</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:96</code> | <code>"server_identity_pkcs12": "/run/secrets/server-identity.p12",</code> |
| <code>interface.8.cfg-ramps-worker-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/ramps/config.example.json:4</code> | <code>"worker_id": "ramp-worker-eu-1",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-ramps-client-tls-timeout-seconds</code> | notes | 634 callers total |
| <code>interface.8.cfg-ramps-compliance-endpoint</code> | notes | 487 callers total |
| <code>interface.8.cfg-ramps-compliance-public-key</code> | notes | 430 callers total |
| <code>interface.8.cfg-ramps-identity</code> | notes | 631 callers total |
| <code>interface.8.cfg-ramps-identity-endpoint</code> | notes | 487 callers total |
| <code>interface.8.cfg-ramps-layerx</code> | notes | 360 callers total |
| <code>interface.8.cfg-ramps-layerx-fee-limit</code> | notes | 87 callers total |
| <code>interface.8.cfg-ramps-layerx-fee-limit</code> | overridden_by | override.J41.70 |
| <code>interface.8.cfg-ramps-layerx-gateway-endpoint</code> | overridden_by | override.J41.71 |
| <code>interface.8.cfg-ramps-layerx-gateway-key-file</code> | overridden_by | override.J41.72 |
| <code>interface.8.cfg-ramps-layerx-network-id</code> | notes | 335 callers total |
| <code>interface.8.cfg-ramps-layerx-network-id</code> | overridden_by | override.J41.73 |
| <code>interface.8.cfg-ramps-layerx-protocol-version</code> | notes | 170 callers total |
| <code>interface.8.cfg-ramps-layerx-protocol-version</code> | overridden_by | override.J41.74 |
| <code>interface.8.cfg-ramps-layerx-signer-public-key</code> | notes | 33 callers total |
| <code>interface.8.cfg-ramps-layerx-signer-public-key</code> | overridden_by | override.J41.75 |
| <code>interface.8.cfg-ramps-listen</code> | notes | 74 callers total |
| <code>interface.8.cfg-ramps-operator</code> | notes | 246 callers total |
| <code>interface.8.cfg-ramps-operator-account</code> | notes | 1391 callers total |
| <code>interface.8.cfg-ramps-paxeer</code> | notes | 256 callers total |
| <code>interface.8.cfg-ramps-paxeer-delayed-after-polls</code> | overridden_by | override.J41.76 |
| <code>interface.8.cfg-ramps-paxeer-required-confirmations</code> | overridden_by | override.J41.77 |
| <code>interface.8.cfg-ramps-paxeer-rpc-chain-id</code> | overridden_by | override.J41.78 |
| <code>interface.8.cfg-ramps-paxeer-rpc-endpoints</code> | overridden_by | override.J41.79 |
| <code>interface.8.cfg-ramps-paxeer-rpc-minimum-agreement</code> | overridden_by | override.J41.80 |
| <code>interface.8.cfg-ramps-paxeer-rpc-trust-anchor-der</code> | overridden_by | override.J41.81 |
| <code>interface.8.cfg-ramps-provider</code> | notes | 220 callers total |
| <code>interface.8.cfg-ramps-provider-credential-file</code> | notes | 45 callers total |
| <code>interface.8.cfg-ramps-provider-endpoint</code> | notes | 487 callers total |
| <code>interface.8.cfg-ramps-quotes</code> | notes | 43 callers total |
| <code>interface.8.cfg-ramps-quotes-context</code> | notes | 379 callers total |
| <code>interface.8.cfg-ramps-quotes-direction</code> | notes | 278 callers total |
| <code>interface.8.cfg-ramps-quotes-expires-at</code> | notes | 167 callers total |
| <code>interface.8.cfg-ramps-quotes-external-amount-minor</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ramps-quotes-external-currency</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ramps-quotes-fee-minor</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ramps-quotes-maximum-slippage-bps</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ramps-quotes-payout-token</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ramps-quotes-provider-token</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ramps-quotes-rate-denominator</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ramps-quotes-rate-numerator</code> | notes | no in-repo config file sets the key outside the defining file |

## ramps-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-ramps-layerx-ramp-ca-pem</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:519</code> | <code>: "${LAYERX_RAMP_CA_PEM:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-customer-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:520</code> | <code>: "${LAYERX_RAMP_CUSTOMER_TOKEN:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-off-grant-json</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:525</code> | <code>: "${LAYERX_RAMP_OFF_GRANT_JSON:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-off-quote-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:524</code> | <code>: "${LAYERX_RAMP_OFF_QUOTE_ID:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-off-receiver-sequence</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:527</code> | <code>: "${LAYERX_RAMP_OFF_RECEIVER_SEQUENCE:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-on-account-sequence</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/ramps/sandbox-journey.sh:526</code> | <code>: "${LAYERX_RAMP_ON_ACCOUNT_SEQUENCE:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-on-quote-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:523</code> | <code>: "${LAYERX_RAMP_ON_QUOTE_ID:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-operator-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:522</code> | <code>: "${LAYERX_RAMP_OPERATOR_TOKEN:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-operator-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:521</code> | <code>: "${LAYERX_RAMP_OPERATOR_URL:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-receipt-verifier-config</code> | <code>env_var</code> | <code>change</code> | <code>services/platform/ramps/sandbox-journey.sh:514</code> | <code>: "${LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-reference-bin</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:513</code> | <code>: "${LAYERX_RAMP_REFERENCE_BIN:?}"</code> |
| <code>interface.8.env-ramps-layerx-ramp-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:518</code> | <code>: "${LAYERX_RAMP_URL:?}"</code> |
| <code>interface.8.env-ramps-paxeer-x-ramp-full-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:3</code> | <code>if [ "${PAXEER_X_RAMP_FULL_CONTRACT:-0}" = 1 ]; then</code> |
| <code>interface.8.env-ramps-vendor-run-attempt</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:543</code> | <code>order_id="sandbox-${direction}-${&lt;vendor&gt;_RUN_ID:-manual}-${&lt;vendor&gt;_RUN_ATTEMPT:-1}"</code> |
| <code>interface.8.env-ramps-vendor-run-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/ramps/sandbox-journey.sh:543</code> | <code>order_id="sandbox-${direction}-${&lt;vendor&gt;_RUN_ID:-manual}-${&lt;vendor&gt;_RUN_ATTEMPT:-1}"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-ramps-layerx-ramp-on-account-sequence</code> | overridden_by | override.J41.82 |
| <code>interface.8.env-ramps-layerx-ramp-receipt-verifier-config</code> | overridden_by | override.J41.83 |
