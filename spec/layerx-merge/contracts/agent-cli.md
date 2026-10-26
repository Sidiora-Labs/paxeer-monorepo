# Agent cli contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## agent-boundary-conformance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agent-boundary-conformance</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tests/boundary/src/main.rs:8</code> | <code>agent-boundary-conformance NODE REPOSITORY</code> |
| <code>interface.9.cli-agent-boundary-conformance-production-duplicate</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tests/boundary/src/main.rs:16</code> | <code>agent-boundary-conformance --production-duplicate</code> |
| <code>interface.9.cli-agent-boundary-conformance-production-peer-refusal</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tests/boundary/src/main.rs:19</code> | <code>agent-boundary-conformance --production-peer-refusal</code> |
| <code>interface.9.cli-agent-boundary-conformance-production-recovery</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tests/boundary/src/main.rs:15</code> | <code>agent-boundary-conformance --production-recovery</code> |
| <code>interface.9.cli-agent-boundary-conformance-production-required</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tests/boundary/src/main.rs:14</code> | <code>agent-boundary-conformance --production-required</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-agent-boundary-conformance</code> | notes | Drives the LNI test node build/agent/layerxd-lni (Makefile:2732); stale task 12.5.4 re-targets cases.rs at the merged chain and 11.7.29 deletes the LNI transport. |
| <code>interface.9.cli-agent-boundary-conformance-production-duplicate</code> | notes | Production boundary mode over the LNI client (services/agent/tests/boundary/src/production.rs); stale task 11.7.29 deletes the LNI transport. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.cli-agent-boundary-conformance-production-peer-refusal</code> | notes | Production boundary mode over the LNI client (services/agent/tests/boundary/src/production.rs); stale task 11.7.29 deletes the LNI transport. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.cli-agent-boundary-conformance-production-recovery</code> | notes | Production boundary mode over the LNI client (services/agent/tests/boundary/src/production.rs); stale task 11.7.29 deletes the LNI transport. No in-repo reference outside the defining line at this revision. |
| <code>interface.9.cli-agent-boundary-conformance-production-required</code> | notes | Production boundary mode over the LNI client (services/agent/tests/boundary/src/production.rs); stale task 11.7.29 deletes the LNI transport. No in-repo reference outside the defining line at this revision. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.58.1</code> | req | 9058.1 |
| <code>acceptance.9.58.1</code> | scenario | Given the conformance binary and an environment without LAYERX_QUALIFY_NODE_RPC_URL, when it runs with --production-required, then it exits nonzero and stderr names LAYERX_QUALIFY_NODE_RPC_URL. |
| <code>acceptance.9.58.1</code> | test | services/agent/tests/boundary/tests/modes.rs::production_mode_requires_the_node_rpc_url (planned) |
| <code>acceptance.9.58.1</code> | qualification | UNRUN |
| <code>req.9058</code> | ac_1 | Each production mode of agent-boundary-conformance exits nonzero naming LAYERX_QUALIFY_NODE_RPC_URL when it is unset. |
| <code>acceptance.9.58.2</code> | req | 9058.2 |
| <code>acceptance.9.58.2</code> | scenario | Given the conformance binary and LAYERX_QUALIFY_LNI_SOCKET set, when it runs with --production-required, then it exits nonzero naming LAYERX_QUALIFY_LNI_SOCKET as retired. |
| <code>acceptance.9.58.2</code> | test | services/agent/tests/boundary/tests/modes.rs::retired_lni_socket_variable_is_refused (planned) |
| <code>acceptance.9.58.2</code> | qualification | UNRUN |
| <code>req.9058</code> | ac_2 | Setting LAYERX_QUALIFY_LNI_SOCKET makes agent-boundary-conformance exit nonzero naming it as retired. |
| <code>capability.9.18.1</code> | title | Re-target the boundary conformance harness at node JSON-RPC |
| <code>capability.9.18.1</code> | reqs | 9058.1<br>9058.2 |

## agent-isolation-suite

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agent-isolation-suite</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tests/isolation/src/main.rs:344</code> | <code>agent-isolation-suite</code> |

## agent-qualification-gates

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agent-fuzz-contract-request-j44</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/fuzz/Cargo.toml:75</code> | <code>cargo fuzz run contract_request (bin contract_request, path fuzz_targets/contract_request.rs)</code> |
| <code>interface.9.cli-agent-fuzz-envelope-decode-j44</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/fuzz/Cargo.toml:33</code> | <code>cargo fuzz run envelope_decode (bin envelope_decode, path fuzz_targets/envelope_decode.rs)</code> |
| <code>interface.9.cli-agent-fuzz-harness-j44</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/fuzz/Cargo.toml:19</code> | <code>cargo fuzz run harness (bin harness, path fuzz_targets/harness.rs)</code> |
| <code>interface.9.cli-agent-fuzz-lni-frame-j44</code> | <code>cli_command</code> | <code>retire</code> | <code>services/agent/fuzz/Cargo.toml:68</code> | <code>cargo fuzz run lni_frame (bin lni_frame, path fuzz_targets/lni_frame.rs)</code> |
| <code>interface.9.cli-agent-fuzz-payload-decode-j44</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/fuzz/Cargo.toml:40</code> | <code>cargo fuzz run payload_decode (bin payload_decode, path fuzz_targets/payload_decode.rs)</code> |
| <code>interface.9.cli-agent-fuzz-policy-loader-j44</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/fuzz/Cargo.toml:82</code> | <code>cargo fuzz run policy_loader (bin policy_loader, path fuzz_targets/policy_loader.rs)</code> |
| <code>interface.9.cli-agent-fuzz-primitive-decode-j44</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/fuzz/Cargo.toml:26</code> | <code>cargo fuzz run primitive_decode (bin primitive_decode, path fuzz_targets/primitive_decode.rs)</code> |
| <code>interface.9.cli-agent-fuzz-proof-decode-j44</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/fuzz/Cargo.toml:54</code> | <code>cargo fuzz run proof_decode (bin proof_decode, path fuzz_targets/proof_decode.rs)</code> |
| <code>interface.9.cli-agent-fuzz-receipt-decode-j44</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/fuzz/Cargo.toml:47</code> | <code>cargo fuzz run receipt_decode (bin receipt_decode, path fuzz_targets/receipt_decode.rs)</code> |
| <code>interface.9.cli-agent-fuzz-roundtrip-j44</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/fuzz/Cargo.toml:61</code> | <code>cargo fuzz run roundtrip (bin roundtrip, path fuzz_targets/roundtrip.rs)</code> |
| <code>interface.9.cli-agent-fuzz-tenant-key-j44</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/fuzz/Cargo.toml:89</code> | <code>cargo fuzz run tenant_key (bin tenant_key, path fuzz_targets/tenant_key.rs)</code> |
| <code>interface.9.cli-agent-qualification-gates-boundary</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tests/qualify/src/main.rs:44</code> | <code>agent-qualification-gates boundary [ARGS]</code> |
| <code>interface.9.cli-agent-qualification-gates-fabrication</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tests/qualify/src/main.rs:62</code> | <code>agent-qualification-gates fabrication [ARGS]</code> |
| <code>interface.9.cli-agent-qualification-gates-faults</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tests/qualify/src/main.rs:72</code> | <code>agent-qualification-gates faults [ARGS]</code> |
| <code>interface.9.cli-agent-qualification-gates-fuzz</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tests/qualify/src/main.rs:85</code> | <code>agent-qualification-gates fuzz [ARGS]</code> |
| <code>interface.9.cli-agent-qualification-gates-wire</code> | <code>cli_command</code> | <code>retire</code> | <code>services/agent/tests/qualify/src/main.rs:26</code> | <code>agent-qualification-gates wire [ARGS]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-agent-fuzz-envelope-decode-j44</code> | notes | Round-trips the retired kernel activity envelope through layerx_wire::activity (services/agent/fuzz/fuzz_targets/envelope_decode.rs:5); task 9.30.1 moves it to the EIP-1559 transaction codec. |
| <code>interface.9.cli-agent-fuzz-lni-frame-j44</code> | notes | Fuzzes layerx_client::lni::framing::decode_frame (services/agent/fuzz/fuzz_targets/lni_frame.rs:5), the retired LNI transport; task 9.30.1 deletes the target. |
| <code>interface.9.cli-agent-fuzz-roundtrip-j44</code> | notes | Selectors 0 and 1 round-trip the retired kernel activity envelope (services/agent/fuzz/fuzz_targets/roundtrip.rs:5); task 9.30.1 moves them to the EIP-1559 transaction codec. |
| <code>interface.9.cli-agent-qualification-gates-boundary</code> | notes | Runs the LNI boundary harness (Makefile:16); stale task 11.7.29 deletes the LNI transport. |
| <code>interface.9.cli-agent-qualification-gates-fabrication</code> | notes | Stale task 0.58 touches tests/qualify. |
| <code>interface.9.cli-agent-qualification-gates-faults</code> | notes | Stale task 0.58 touches tests/qualify. |
| <code>interface.9.cli-agent-qualification-gates-fuzz</code> | notes | Runs services/agent/tools/run-sanitizers.sh (services/agent/tests/qualify/fuzz.rs:232). |
| <code>interface.9.cli-agent-qualification-gates-wire</code> | notes | Stale task 11.7.33 deletes the wire differential this gate runs. |

Contract context <code>contract_context.9.agent-fuzz-targets</code>:

| Field | Frozen rule |
|---|---|
| interface | cargo fuzz targets of the layerx-agent-fuzz package at services/agent/fuzz |
| storage | seed corpora under services/agent/fuzz/corpus |
| errors | a failed round-trip assertion aborts the target |
| events | none |
| gas | none |
| activation | lni_frame retires and envelope_decode and roundtrip move to the EIP-1559 codec in wave 6 |
| invariants | every target accepts hex: prefixed seeds; decoded inputs re-encode byte for byte |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.313.1</code> | req | 9313.1 |
| <code>acceptance.9.313.1</code> | scenario | Given services/agent/fuzz/corpus/envelope_decode/eip1559-transfer-1 holds one signed EIP-1559 transfer, when envelope_decode_round_trips_signed_eip1559_transactions runs codec::envelope_round_trip on it, then evm_transaction::decode_signed succeeds and assemble returns the seed bytes exactly. |
| <code>acceptance.9.313.1</code> | test | services/agent/fuzz/tests/codec.rs::envelope_decode_round_trips_signed_eip1559_transactions (planned) |
| <code>acceptance.9.313.1</code> | qualification | UNRUN |
| <code>req.9313</code> | ac_1 | The envelope_decode codec decodes every envelope_decode seed as a signed EIP-1559 transaction and re-assembles it byte for byte (services/agent/fuzz/fuzz_targets/envelope_decode.rs:8). |
| <code>acceptance.9.313.2</code> | req | 9313.2 |
| <code>acceptance.9.313.2</code> | scenario | Given the files under services/agent/fuzz/corpus/roundtrip after the activity seed is replaced, when roundtrip_corpus_seeds_round_trip_through_their_codecs runs codec::roundtrip_case on each, then no assertion fails and the selector-0 seed decodes as a signed EIP-1559 transaction. |
| <code>acceptance.9.313.2</code> | test | services/agent/fuzz/tests/codec.rs::roundtrip_corpus_seeds_round_trip_through_their_codecs (planned) |
| <code>acceptance.9.313.2</code> | qualification | UNRUN |
| <code>req.9313</code> | ac_2 | Every roundtrip seed passes codec::roundtrip_case and the transaction seed decodes successfully (services/agent/fuzz/fuzz_targets/roundtrip.rs:12). |
| <code>acceptance.9.59.1</code> | req | 9059.1 |
| <code>acceptance.9.59.1</code> | scenario | Given the qualification binary, when it runs with the wire command and three paths, then it exits nonzero and stderr says the wire gate is retired. |
| <code>acceptance.9.59.1</code> | test | services/agent/tests/qualify/tests/gates.rs::wire_gate_is_refused_as_retired (planned) |
| <code>acceptance.9.59.1</code> | qualification | UNRUN |
| <code>req.9059</code> | ac_1 | agent-qualification-gates wire exits nonzero naming the wire gate as retired. |
| <code>acceptance.9.62.1</code> | req | 9062.1 |
| <code>acceptance.9.62.1</code> | scenario | Given the qualification binary, when it runs boundary with a repository path and /usr/bin/true, then it exits nonzero naming the expected node JSON-RPC URL. |
| <code>acceptance.9.62.1</code> | test | services/agent/tests/qualify/tests/boundary_gate.rs::boundary_gate_requires_a_node_rpc_url (planned) |
| <code>acceptance.9.62.1</code> | qualification | UNRUN |
| <code>req.9062</code> | ac_1 | agent-qualification-gates boundary refuses a NODE argument that is not an http or https URL. |
| <code>acceptance.9.62.2</code> | req | 9062.2 |
| <code>acceptance.9.62.2</code> | scenario | Given the repository after this task, when the test lists services/agent/tests/boundary/node, then none of the four LNI files exists. |
| <code>acceptance.9.62.2</code> | test | services/agent/tests/qualify/tests/boundary_gate.rs::boundary_harness_tree_has_no_lni_node (planned) |
| <code>acceptance.9.62.2</code> | qualification | UNRUN |
| <code>req.9062</code> | ac_2 | services/agent/tests/boundary/node holds no layerxd_lni.c, preparation_state.c or production-lni files. |
| <code>acceptance.9.64.1</code> | req | 9064.1 |
| <code>acceptance.9.64.1</code> | scenario | Given the fuzz gate target list FUZZ_TARGETS at services/agent/tests/qualify/fuzz.rs:6, when the unit test reads it, then lni_frame is absent and contract_request and policy_loader remain. |
| <code>acceptance.9.64.1</code> | test | services/agent/tests/qualify/fuzz.rs::fuzz_gate_requires_no_lni_frame_target (planned) |
| <code>acceptance.9.64.1</code> | qualification | UNRUN |
| <code>req.9064</code> | ac_1 | The qualification fuzz gate's required target list holds no lni_frame entry. |
| <code>capability.9.18.2</code> | title | Retire the wire differential and the qualification wire gate |
| <code>capability.9.18.2</code> | reqs | 9059.1 |
| <code>capability.9.18.5</code> | title | Run the qualification boundary gate against a node JSON-RPC URL and delete the LNI test node |
| <code>capability.9.18.5</code> | reqs | 9062.1<br>9062.2 |
| <code>capability.9.18.7</code> | title | Drop the LNI frame target from the qualification fuzz gate |
| <code>capability.9.18.7</code> | reqs | 9064.1 |
| <code>capability.9.30.1</code> | title | Drop the lni_frame fuzz target and fuzz the EIP-1559 transaction codec |
| <code>capability.9.30.1</code> | reqs | 9313.1<br>9313.2 |

## agent-sdk-doc-check

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agent-sdk-doc-check</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tools/doc-check/src/main.rs:187</code> | <code>agent-sdk-doc-check [REPOSITORY]</code> |

## agent-sdk-gen

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agent-sdk-gen-check</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tools/sdk-gen/src/main.rs:969</code> | <code>agent-sdk-gen --check [SCHEMA_ROOT] [OUTPUT_ROOT] (default mode)</code> |
| <code>interface.9.cli-agent-sdk-gen-write</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tools/sdk-gen/src/main.rs:965</code> | <code>agent-sdk-gen --write [SCHEMA_ROOT] [OUTPUT_ROOT]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-agent-sdk-gen-check</code> | notes | Stale task 6.5.30 (in_progress). |
| <code>interface.9.cli-agent-sdk-gen-write</code> | notes | Stale task 6.5.30 (in_progress) regenerates the TS and Python envelopes at major 2; 6.5.53 regenerates goldens and the six SDKs. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.61.1</code> | req | 9061.1 |
| <code>acceptance.9.61.1</code> | scenario | Given the canonical schema, when agent_sdk_generator runs, then the TypeScript output contains contractMajor: 2 and a rung enum of submitted, receipted and committed. |
| <code>acceptance.9.61.1</code> | test | services/agent/tools/sdk-gen/tests/major_two.rs::generated_typescript_declares_contract_major_two_and_rungs (planned) |
| <code>acceptance.9.61.1</code> | qualification | UNRUN |
| <code>req.9061</code> | ac_1 | The generated TypeScript declares contractMajor 2 and exactly the rungs submitted, receipted and committed. |
| <code>acceptance.9.61.2</code> | req | 9061.2 |
| <code>acceptance.9.61.2</code> | scenario | Given the canonical schema, when agent_sdk_generator runs, then the Signature type in both outputs is fixed at 65 bytes and the Address type at 20. |
| <code>acceptance.9.61.2</code> | test | services/agent/tools/sdk-gen/tests/major_two.rs::generated_bindings_use_65_byte_signatures (planned) |
| <code>acceptance.9.61.2</code> | qualification | UNRUN |
| <code>req.9061</code> | ac_2 | The generated TypeScript and Python signature types are 65 bytes and address types 20 bytes. |
| <code>capability.9.18.4</code> | title | Generate SDK bindings from the major-2 schema with EVM write bindings |
| <code>capability.9.18.4</code> | reqs | 9061.1<br>9061.2 |

## agent-sdk-parity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agent-sdk-parity</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tests/parity/src/main.rs:957</code> | <code>agent-sdk-parity NODE REPOSITORY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-agent-sdk-parity</code> | notes | Runs against the LNI test node build/agent/layerxd-lni (Makefile:3040); stale task 0.58 touches it and 11.7.29 deletes the LNI transport. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.4.1</code> | req | 9004.1 |
| <code>acceptance.9.4.1</code> | scenario | Given the parity binary built from services/agent/tests/parity, when it is run with /usr/bin/true as its node argument, then it exits nonzero with `expected a node JSON-RPC URL` and starts no process. |
| <code>acceptance.9.4.1</code> | test | services/agent/tests/parity/tests/arguments.rs::parity_requires_a_node_json_rpc_url (planned) |
| <code>acceptance.9.4.1</code> | qualification | UNRUN |
| <code>req.9004</code> | ac_1 | The parity binary refuses a filesystem path where HEAD expects a layerxd executable (services/agent/tests/parity/src/main.rs:957) and accepts only an http or https node JSON-RPC URL. |
| <code>acceptance.9.4.2</code> | req | 9004.2 |
| <code>acceptance.9.4.2</code> | scenario | Given a copy of scenarios.kvx with one observation set to sequencer_signed, when the parity binary validates scenarios through its --check-scenarios argument, then it exits nonzero naming that observation. |
| <code>acceptance.9.4.2</code> | test | services/agent/tests/parity/tests/arguments.rs::parity_scenarios_record_rungs (planned) |
| <code>acceptance.9.4.2</code> | qualification | UNRUN |
| <code>req.9004</code> | ac_2 | Every observation in services/agent/tests/parity/scenarios.kvx that the binary loads names a rung submitted, receipted or committed and the binary refuses a scenario naming a retired level. |
| <code>acceptance.9.999.1</code> | req | 9999.1 |
| <code>acceptance.9.999.1</code> | scenario | Given the candidate after every domain-9 wave-6 task, when agent_completion_retired_surfaces_absent runs, then no retired surface remains and every planned path exists |
| <code>acceptance.9.999.1</code> | test | services/agent/tests/parity/tests/completion_gate.rs::agent_completion_retired_surfaces_absent (planned) |
| <code>acceptance.9.999.1</code> | qualification | UNRUN |
| <code>req.9999</code> | ac_1 | agent_completion_retired_surfaces_absent passes on the merged candidate: no retired domain-9 surface remains and every planned domain-9 wave-6 path exists. |
| <code>capability.9.0.9</code> | title | Domain 9 completion gate |
| <code>capability.9.0.9</code> | reqs | 9999.1 |
| <code>capability.9.20.94</code> | title | Repoint the SDK parity harness at the node JSON-RPC endpoint and rungs |
| <code>capability.9.20.94</code> | reqs | 9004.1<br>9004.2 |

## agent-secret-check

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agent-secret-check</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tools/secret-check/src/main.rs:290</code> | <code>agent-secret-check [ROOT]</code> |

## agent-wire-differential

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agent-wire-differential</code> | <code>cli_command</code> | <code>retire</code> | <code>services/agent/tools/wire-differential/src/main.rs:355</code> | <code>agent-wire-differential (differential against the C reference named by LAYERX_C_REFERENCE)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-agent-wire-differential</code> | notes | Stale task 11.7.33 deletes layerx_wire::activity and the wire differential. |

## dependency-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-dependency-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tools/dependency-policy.sh:1</code> | <code>sh services/agent/tools/dependency-policy.sh</code> |

## generate-operation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-generate-operation</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tools/generate-operation.py:50</code> | <code>python3 services/agent/tools/generate-operation.py --schema PATH --output PATH [--check]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-generate-operation</code> | notes | No Makefile or CI invocation found; operation_generated.rs in layerx-agent-api is its output. No in-repo reference outside the defining line at this revision. |

## layerx-agentd

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-layerx-agentd</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:2202</code> | <code>layerx-agentd (no arguments; configuration from LAYERX_AGENT_* environment)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-layerx-agentd</code> | notes | The layerx-agentd-host package builds this bin from ../layerx-agentd/src/main.rs. |

## layerx-agentd-mcp-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-layerx-agentd-mcp-evidence</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:2142</code> | <code>layerx-agentd --mcp-evidence ROOT TENANT KEY [TOOL]</code> |

## layerx-agentd-modes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-layerx-agentd-mode-full</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:23</code> | <code>LAYERX_AGENT_MODE=full (default when unset): full agent daemon</code> |
| <code>interface.9.cli-layerx-agentd-mode-human-owner</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human_owner_mode.rs:24</code> | <code>LAYERX_AGENT_MODE=human-owner: human-owner runtime with the authenticated GET /healthz only</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-layerx-agentd-mode-human-owner</code> | notes | Stale task 11.5.13 re-targets human_owner_mode.rs off the finality and LNI surfaces. |

## layerx-audit-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-layerx-audit-verify</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tools/audit-verify/src/main.rs:18</code> | <code>layerx-audit-verify PATH</code> |

## layerx-boundary-check

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-layerx-boundary-check</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tools/boundary-check/src/main.rs:205</code> | <code>layerx-boundary-check [ROOT]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-layerx-boundary-check</code> | notes | Stale task 11.5.58 re-targets tools/boundary-check/src/main.rs off the LNI transport. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.60.1</code> | req | 9060.1 |
| <code>acceptance.9.60.1</code> | scenario | Given a fixture crate whose lib.rs contains use layerx_client::lni::framing, when layerx-boundary-check runs, then it exits nonzero naming the file. |
| <code>acceptance.9.60.1</code> | test | services/agent/tools/boundary-check/tests/lni_retired.rs::any_lni_import_fails_the_purity_check (planned) |
| <code>acceptance.9.60.1</code> | qualification | UNRUN |
| <code>req.9060</code> | ac_1 | A crate source importing layerx_client::lni fails the boundary check, including inside the client crate. |
| <code>acceptance.9.60.2</code> | req | 9060.2 |
| <code>acceptance.9.60.2</code> | scenario | Given a clean fixture agent root without stable-abi-allowlist.toml, when layerx-boundary-check runs, then it exits zero. |
| <code>acceptance.9.60.2</code> | test | services/agent/tools/boundary-check/tests/lni_retired.rs::purity_check_runs_without_a_stable_abi_allowlist (planned) |
| <code>acceptance.9.60.2</code> | qualification | UNRUN |
| <code>req.9060</code> | ac_2 | layerx-boundary-check passes a clean fixture agent root that has no stable-abi-allowlist.toml. |
| <code>capability.9.18.3</code> | title | Refuse any LNI transport import in the boundary purity check |
| <code>capability.9.18.3</code> | reqs | 9060.1<br>9060.2 |
| <code>resolution.17.845</code> | title | decision.910 binds: The layerx-wire crate stays as the agent's canonical encoding library |
| <code>resolution.17.845</code> | applies_to | decision.910<br>task.9.18.3<br>task.9.19.1 |
| <code>resolution.17.845</code> | source | decision.910 in fragments/J43/decision.kvx; tasks from affects |

## layerx-mcp

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-layerx-mcp</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/bin/layerx-mcp.rs:14</code> | <code>layerx-mcp &lt;absolute path to the daemon binding document&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-layerx-mcp</code> | notes | Serves MCP JSON-RPC on stdio, or on the unix socket named by the binding listener section. |

## layerx-policy-harness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-layerx-policy-harness</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tools/policy-harness/src/main.rs:73</code> | <code>layerx-policy-harness</code> |

## layerx-wallet-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-layerx-wallet-rpc</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/crates/layerx-sdk/src/bin/layerx-wallet-rpc.rs:200</code> | <code>layerx-wallet-rpc (one JSON request on stdin with action accounts&#124;balance&#124;wait&#124;submit, JSON reply on stdout)</code> |
| <code>interface.9.cli-layerx-wallet-rpc-accounts</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/crates/layerx-sdk/src/bin/layerx-wallet-rpc.rs:144</code> | <code>layerx-wallet-rpc stdin action=accounts</code> |
| <code>interface.9.cli-layerx-wallet-rpc-balance</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/crates/layerx-sdk/src/bin/layerx-wallet-rpc.rs:150</code> | <code>layerx-wallet-rpc stdin action=balance</code> |
| <code>interface.9.cli-layerx-wallet-rpc-submit</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/crates/layerx-sdk/src/bin/layerx-wallet-rpc.rs:168</code> | <code>layerx-wallet-rpc stdin action=submit</code> |
| <code>interface.9.cli-layerx-wallet-rpc-wait</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/crates/layerx-sdk/src/bin/layerx-wallet-rpc.rs:156</code> | <code>layerx-wallet-rpc stdin action=wait commitment=executed&#124;batched&#124;finalised</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-layerx-wallet-rpc</code> | notes | Stale task 6.5.26 moves the binary onto the EVM signer; 10.5.4 and 11.5.36 also touch it. |
| <code>interface.9.cli-layerx-wallet-rpc-accounts</code> | notes | Stale task 6.5.26 replaces the kernel activity path with EVM transactions. |
| <code>interface.9.cli-layerx-wallet-rpc-balance</code> | notes | Stale task 6.5.26 replaces the kernel activity path with EVM transactions. |
| <code>interface.9.cli-layerx-wallet-rpc-submit</code> | notes | Stale task 6.5.26 replaces the kernel activity path with EVM transactions. |
| <code>interface.9.cli-layerx-wallet-rpc-wait</code> | notes | Stale task 6.5.26 replaces the kernel activity path with EVM transactions. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.333.1</code> | req | 9333.1 |
| <code>acceptance.9.333.1</code> | scenario | Given a loopback listener answering eth_getTransactionCount, eth_estimateGas and eth_sendRawTransaction and a submit request with a test EVM key reference, when the test pipes the request to the binary, then stdout is one JSON object whose txHash equals the listener's hash and whose rung is submitted. |
| <code>acceptance.9.333.1</code> | test | services/agent/crates/layerx-sdk/tests/wallet_rpc_bin.rs::submit_reply_carries_the_transaction_hash_and_rung (planned) |
| <code>acceptance.9.333.1</code> | qualification | UNRUN |
| <code>req.9333</code> | ac_1 | A submit request returns JSON with txHash and rung submitted. |
| <code>acceptance.9.333.2</code> | req | 9333.2 |
| <code>acceptance.9.333.2</code> | scenario | Given wait requests with rung receipted and with commitment finalised, when the test pipes each to the binary, then the first polls pax_sequencerReceipt and the second exits non-zero with the bridge error. |
| <code>acceptance.9.333.2</code> | test | services/agent/crates/layerx-sdk/tests/wallet_rpc_bin.rs::wait_accepts_rung_names_and_refuses_kernel_commitments (planned) |
| <code>acceptance.9.333.2</code> | qualification | UNRUN |
| <code>req.9333</code> | ac_2 | The wait action accepts rung names and refuses the kernel commitment names. |
| <code>capability.9.32.3</code> | title | Serve layerx-wallet-rpc over the EVM signer with hashes and rungs |
| <code>capability.9.32.3</code> | reqs | 9333.1<br>9333.2 |

## operator-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-agentd-operator-activity-submit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:193</code> | <code>operator command activity.submit (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-budget-create</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:199</code> | <code>operator command budget.create (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-budget-inspect</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:169</code> | <code>operator command budget.inspect (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-budget-reconcile</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:175</code> | <code>operator command budget.reconcile (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-subscription-inspect</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:157</code> | <code>operator command subscription.inspect (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-subscription-resume</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:163</code> | <code>operator command subscription.resume (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-unknown-inspect</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:145</code> | <code>operator command unknown.inspect (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-unknown-resolve</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:151</code> | <code>operator command unknown.resolve (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-verification-inspect</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:181</code> | <code>operator command verification.inspect (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |
| <code>interface.9.cli-agentd-operator-verification-retry</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/admin.rs:187</code> | <code>operator command verification.retry (admin CommandDescriptor, routed by route_operator_command at services/agent/crates/layerx-agentd/src/human_runtime.rs:17672, reached through the human peer OPERATOR opcode)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-agentd-operator-activity-submit</code> | notes | admin.rs is touched by stale task 0.58 (Rung); the command itself is not named. |
| <code>interface.9.cli-agentd-operator-budget-create</code> | notes | admin.rs is touched by stale task 0.58 (Rung); the command itself is not named. |
| <code>interface.9.cli-agentd-operator-budget-inspect</code> | notes | admin.rs is touched by stale task 0.58 (Rung); the command itself is not named. |
| <code>interface.9.cli-agentd-operator-budget-reconcile</code> | notes | admin.rs is touched by stale task 0.58 (Rung); the command itself is not named. |
| <code>interface.9.cli-agentd-operator-subscription-inspect</code> | notes | admin.rs is touched by stale task 0.58 (Rung); the command itself is not named. |
| <code>interface.9.cli-agentd-operator-subscription-resume</code> | notes | admin.rs is touched by stale task 0.58 (Rung); the command itself is not named. |
| <code>interface.9.cli-agentd-operator-unknown-inspect</code> | notes | Stale task 6.5.5 resolves unknown outcomes by transaction hash then resultByIdempotency. |
| <code>interface.9.cli-agentd-operator-unknown-resolve</code> | notes | Stale task 6.5.5 resolves unknown outcomes by transaction hash then resultByIdempotency. |
| <code>interface.9.cli-agentd-operator-verification-inspect</code> | notes | admin.rs is touched by stale task 0.58 (Rung); the command itself is not named. |
| <code>interface.9.cli-agentd-operator-verification-retry</code> | notes | admin.rs is touched by stale task 0.58 (Rung); the command itself is not named. |

## run-real-node-tests

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-run-real-node-tests-prepare</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tools/run-real-node-tests.sh:7</code> | <code>sh services/agent/tools/run-real-node-tests.sh prepare</code> |
| <code>interface.9.cli-run-real-node-tests-sanitizers</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tools/run-real-node-tests.sh:6</code> | <code>sh services/agent/tools/run-real-node-tests.sh sanitizers</code> |
| <code>interface.9.cli-run-real-node-tests-test</code> | <code>cli_command</code> | <code>change</code> | <code>services/agent/tools/run-real-node-tests.sh:5</code> | <code>sh services/agent/tools/run-real-node-tests.sh test</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.cli-run-real-node-tests-prepare</code> | notes | Stale task 11.5.59 re-targets run-real-node-tests.sh off the finality evidence path. |
| <code>interface.9.cli-run-real-node-tests-sanitizers</code> | notes | Stale task 11.5.59 re-targets run-real-node-tests.sh off the finality evidence path. |
| <code>interface.9.cli-run-real-node-tests-test</code> | notes | Stale task 11.5.59 re-targets run-real-node-tests.sh off the finality evidence path. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.63.1</code> | req | 9063.1 |
| <code>acceptance.9.63.1</code> | scenario | Given services/agent/tools/run-real-node-tests.sh after this task, when the test reads it, then the prepare section names neither layerxd nor lxp_test_daemon_finality_authority and no line imports ed25519. |
| <code>acceptance.9.63.1</code> | test | services/agent/tests/qualify/tests/real_node_script.rs::real_node_prepare_builds_no_kernel_daemon (planned) |
| <code>acceptance.9.63.1</code> | qualification | UNRUN |
| <code>req.9063</code> | ac_1 | run-real-node-tests.sh prepare names neither layerxd nor lxp_test_daemon_finality_authority and imports no ed25519 module. |
| <code>capability.9.18.6</code> | title | Prepare real-node tests without the kernel daemon |
| <code>capability.9.18.6</code> | reqs | 9063.1 |

## run-sanitizers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.cli-run-sanitizers</code> | <code>cli_command</code> | <code>stable</code> | <code>services/agent/tools/run-sanitizers.sh:1</code> | <code>sh services/agent/tools/run-sanitizers.sh</code> |
