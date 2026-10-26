# Platform hosted contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## agent-boundary-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-chain-rpc-url-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/agent-boundary/src/chain_receipt.rs</code> (planned) | <code>LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-client-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:462</code> | <code>let config = match env::var("LAYERX_AGENT_BOUNDARY_CLIENT_CA_DER") {</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-gateway-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:549</code> | <code>let gateway_token = read_secret("LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE")?;</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:545</code> | <code>let listen = env::var("LAYERX_AGENT_BOUNDARY_LISTEN")</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-lni-deadline-ms-j69</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:570</code> | <code>let lni_deadline_ms = parse_u64("LAYERX_AGENT_BOUNDARY_LNI_DEADLINE_MS", 10_000)?;</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-lni-socket</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:567</code> | <code>env::var("LAYERX_AGENT_BOUNDARY_LNI_SOCKET")</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-module-registry-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:487</code> | <code>let declarations = match env::var("LAYERX_AGENT_BOUNDARY_MODULE_REGISTRY_FILE") {</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:585</code> | <code>let network_name = env::var("LAYERX_AGENT_BOUNDARY_NETWORK_ID")</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-node-bearer-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:608</code> | <code>node_token: read_secret("LAYERX_AGENT_BOUNDARY_NODE_BEARER_TOKEN_FILE")?,</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-node-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:605</code> | <code>&amp;env::var("LAYERX_AGENT_BOUNDARY_NODE_URL")</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-protocol-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:578</code> | <code>let protocol_network_id = env::var("LAYERX_AGENT_BOUNDARY_PROTOCOL_NETWORK_ID")</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-receipt-keys-file-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/agent-boundary/src/chain_receipt.rs</code> (planned) | <code>LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-receipt-wait-ms-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:574</code> | <code>let receipt_wait_ms = parse_u64("LAYERX_AGENT_BOUNDARY_RECEIPT_WAIT_MS", 5_000)?;</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-registry-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:550</code> | <code>let registry_token = read_secret("LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE")?;</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-state-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:591</code> | <code>env::var("LAYERX_AGENT_BOUNDARY_STATE_DIR")</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-tls-cert-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:453</code> | <code>let certificate_path = env::var("LAYERX_AGENT_BOUNDARY_TLS_CERT_DER")</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-tls-key-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:455</code> | <code>let key_path = env::var("LAYERX_AGENT_BOUNDARY_TLS_KEY_DER")</code> |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-webhook-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:559</code> | <code>let webhook_token = read_secret("LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE")?;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-chain-rpc-url-j40</code> | notes | planned chain JSON-RPC endpoint for pax_sequencerReceipt reads; task 8.8.1 |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-gateway-token-file-j69</code> | notes | read at services/platform/hosted/agent-boundary/src/main.rs:549; 11 in-repo references, first 8 listed |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-lni-deadline-ms-j69</code> | notes | read at services/platform/hosted/agent-boundary/src/main.rs:570; kernel LNI deadline; deleted by task 8.8.4 |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-lni-socket</code> | notes | LNI transport retired; reads move to eth_call and gRPC queries (spec/layerx-merge/spec.kvx:60434) |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-module-registry-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-node-bearer-token-file-j69</code> | notes | read at services/platform/hosted/agent-boundary/src/main.rs:608; 9 in-repo references, first 8 listed |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-receipt-keys-file-j40</code> | notes | planned pinned sequencer receipt key list in the gateway ReceiptKeyList format; task 8.8.1 |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-receipt-wait-ms-j69</code> | notes | read at services/platform/hosted/agent-boundary/src/main.rs:574 |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-registry-token-file-j69</code> | notes | read at services/platform/hosted/agent-boundary/src/main.rs:550; 13 in-repo references, first 8 listed |
| <code>interface.8.env-agent-boundary-layerx-agent-boundary-webhook-token-file-j69</code> | notes | read at services/platform/hosted/agent-boundary/src/main.rs:559; 11 in-repo references, first 8 listed |

Contract context <code>contract_context.8.agent-boundary-chain</code>:

| Field | Frozen rule |
|---|---|
| interface | HTTPS routes of the agent boundary on the gateway, registry and webhook planes; receipts and deployment proofs read from pax_sequencerReceipt |
| storage | state directory journal and activities records only; no kernel session after task 8.8.4 |
| errors | invalid_activity_id 400, receipt_not_found 404, entitlement_denied 403, not_found 404, node_unavailable 503 |
| events | none |
| gas | none; the boundary only reads the chain |
| activation | receipts move to the chain in wave 3, program routes retire in waves 4 to 6 |
| invariants | every receipt answered verifies against the pinned key list; no route submits a kernel activity after task 8.8.3 |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.80.1</code> | req | 8080.1 |
| <code>acceptance.8.80.1</code> | scenario | Given an environment without any LNI variable and a chain listener, when the boundary starts and is asked for a receipt and a head attestation, then it is live, the receipt answers 200 and the attestation answers 404 |
| <code>acceptance.8.80.1</code> | test | services/platform/hosted/agent-boundary/tests/chain_receipt.rs::boundary_starts_without_an_lni_socket (planned) |
| <code>acceptance.8.80.1</code> | qualification | UNRUN |
| <code>req.8080</code> | ac_1 | The agent boundary starts with no LAYERX_AGENT_BOUNDARY_LNI_SOCKET set, serves a receipt read from the chain and answers 404 not_found for the head attestation path. |
| <code>capability.8.8.4</code> | title | Remove the kernel session and the head attestation route from the agent boundary |
| <code>capability.8.8.4</code> | reqs | 8080.1 |

## agent-boundary-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-agent-boundary-activity-not-journaled</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1926</code> | <code>return refusal(404, "activity_not_journaled", None)</code> |
| <code>interface.8.err-agent-boundary-asset-ordinal-reserved</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:797</code> | <code>Ok(Some(terminal(422, "asset_ordinal_reserved")))</code> |
| <code>interface.8.err-agent-boundary-bad-signature</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:860</code> | <code>Ok(SubmitOutcome::Refused(terminal(422, "bad_signature")))</code> |
| <code>interface.8.err-agent-boundary-capability-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/head_attestation.rs:34</code> | <code>return Ok(Err(refusal(503, "capability_unavailable", Some(60))));</code> |
| <code>interface.8.err-agent-boundary-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1449</code> | <code>return refusal(400, "content_type_required", None);</code> |
| <code>interface.8.err-agent-boundary-entitlement-denied</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/deployment.rs:105</code> | <code>return Some(refusal(403, "entitlement_denied", None));</code> |
| <code>interface.8.err-agent-boundary-head-stale</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/head_attestation.rs:45</code> | <code>Err(ProgramHeadAttestError::HeadStale) =&gt; Ok(Err(refusal(503, "head_stale", Some(1)))),</code> |
| <code>interface.8.err-agent-boundary-idempotency-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1762</code> | <code>return refusal(409, "idempotency_conflict", None);</code> |
| <code>interface.8.err-agent-boundary-idempotency-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1736</code> | <code>return refusal(400, "idempotency_key_required", None);</code> |
| <code>interface.8.err-agent-boundary-identity-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2121</code> | <code>return Err(refusal(401, "identity_required", None));</code> |
| <code>interface.8.err-agent-boundary-invalid-activity-id</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/deployment.rs:35</code> | <code>return refusal(400, "invalid_activity_id", None);</code> |
| <code>interface.8.err-agent-boundary-invalid-activity-length</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1452</code> | <code>return refusal(400, "invalid_activity_length", None);</code> |
| <code>interface.8.err-agent-boundary-invalid-asset-activity</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:800</code> | <code>Ok(Some(terminal(400, "invalid_asset_activity")))</code> |
| <code>interface.8.err-agent-boundary-invalid-deployment-body</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/deployment.rs:14</code> | <code>return refusal(400, "invalid_deployment_body", None);</code> |
| <code>interface.8.err-agent-boundary-invalid-expected-state-root</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1587</code> | <code>return Err(refusal(400, "invalid_expected_state_root", None));</code> |
| <code>interface.8.err-agent-boundary-invalid-idempotency-key</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1739</code> | <code>return refusal(400, "invalid_idempotency_key", None);</code> |
| <code>interface.8.err-agent-boundary-invalid-minimum-sequence</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1576</code> | <code>return Err(refusal(400, "invalid_minimum_sequence", None));</code> |
| <code>interface.8.err-agent-boundary-invalid-program-id</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/head_attestation.rs:94</code> | <code>return Some(refusal(400, "invalid_program_id", None));</code> |
| <code>interface.8.err-agent-boundary-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2391</code> | <code>&#124;_&#124; refusal(400, "invalid_request", None),</code> |
| <code>interface.8.err-agent-boundary-invalid-staleness-ms</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/head_attestation.rs:100</code> | <code>return Some(refusal(400, "invalid_staleness_ms", None));</code> |
| <code>interface.8.err-agent-boundary-lifecycle-receipt-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1245</code> | <code>return refusal(503, "lifecycle_receipt_invalid", Some(5));</code> |
| <code>interface.8.err-agent-boundary-malformed-activity</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:773</code> | <code>return Ok(Some(terminal(400, "malformed_activity")));</code> |
| <code>interface.8.err-agent-boundary-malformed-program-call</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:977</code> | <code>.map_err(&#124;_&#124; refusal(400, "malformed_program_call", None))?;</code> |
| <code>interface.8.err-agent-boundary-malformed-program-deploy</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:993</code> | <code>.map_err(&#124;_&#124; refusal(400, "malformed_program_deploy", None))?;</code> |
| <code>interface.8.err-agent-boundary-malformed-program-upgrade</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:998</code> | <code>.map_err(&#124;_&#124; refusal(400, "malformed_program_upgrade", None))?;</code> |
| <code>interface.8.err-agent-boundary-malformed-program-wasm</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1009</code> | <code>return Err(refusal(400, "malformed_program_wasm", None));</code> |
| <code>interface.8.err-agent-boundary-malformed-program-wind-down</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1004</code> | <code>.map_err(&#124;_&#124; refusal(400, "malformed_program_wind_down", None))</code> |
| <code>interface.8.err-agent-boundary-node-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2071</code> | <code>return refusal(503, "node_invalid", Some(5));</code> |
| <code>interface.8.err-agent-boundary-node-transport-lost</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:320</code> | <code>refusal(503, "node_transport_lost", Some(5))</code> |
| <code>interface.8.err-agent-boundary-node-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:316</code> | <code>refusal(503, "node_unavailable", Some(5))</code> |
| <code>interface.8.err-agent-boundary-non-canonical-activity</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:941</code> | <code>return Err(refusal(400, "non_canonical_activity", None));</code> |
| <code>interface.8.err-agent-boundary-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/deployment.rs:108</code> | <code>return Some(refusal(404, "not_found", None));</code> |
| <code>interface.8.err-agent-boundary-not-program-call</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1459</code> | <code>return refusal(400, "not_program_call", None);</code> |
| <code>interface.8.err-agent-boundary-persistence-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1311</code> | <code>return refusal(503, "persistence_invalid", Some(5));</code> |
| <code>interface.8.err-agent-boundary-persistence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1347</code> | <code>return refusal(503, "persistence_unavailable", Some(5));</code> |
| <code>interface.8.err-agent-boundary-program-artifacts-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1127</code> | <code>let invalid = &#124;&#124; refusal(503, "program_artifacts_invalid", Some(5));</code> |
| <code>interface.8.err-agent-boundary-program-artifacts-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1138</code> | <code>return Err(refusal(503, "program_artifacts_unavailable", Some(5)));</code> |
| <code>interface.8.err-agent-boundary-program-not-registered</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/head_attestation.rs:47</code> | <code>Ok(Err(refusal(404, "program_not_registered", None)))</code> |
| <code>interface.8.err-agent-boundary-program-payload-hash-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1013</code> | <code>return Err(refusal(400, "program_payload_hash_mismatch", None));</code> |
| <code>interface.8.err-agent-boundary-receipt-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1064</code> | <code>return refusal(503, "receipt_invalid", Some(5));</code> |
| <code>interface.8.err-agent-boundary-receipt-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1823</code> | <code>Ok(Lookup::Absent) =&gt; refusal(404, "receipt_not_found", None),</code> |
| <code>interface.8.err-agent-boundary-receipt-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1905</code> | <code>return refusal(503, "receipt_unavailable", Some(5));</code> |
| <code>interface.8.err-agent-boundary-snapshot-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1686</code> | <code>Ok(Err(refusal(409, "snapshot_mismatch", None)))</code> |
| <code>interface.8.err-agent-boundary-snapshot-stale</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:1683</code> | <code>Ok(Err(refusal(409, "snapshot_stale", Some(1))))</code> |
| <code>interface.8.err-agent-boundary-unsupported-authority</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:952</code> | <code>.map_err(&#124;_&#124; refusal(422, "unsupported_authority", None))?;</code> |
| <code>interface.8.err-agent-boundary-version-unsupported</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:863</code> | <code>Ok(SubmitOutcome::Refused(terminal(422, "version_unsupported")))</code> |
| <code>interface.8.err-agent-boundary-withdrawal-asset-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:803</code> | <code>Ok(Some(terminal(422, "withdrawal_asset_unavailable")))</code> |
| <code>interface.8.err-agent-boundary-wrong-network</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:866</code> | <code>Ok(SubmitOutcome::Refused(terminal(422, "wrong_network")))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-agent-boundary-activity-not-journaled</code> | notes | HTTP 404 |
| <code>interface.8.err-agent-boundary-asset-ordinal-reserved</code> | notes | HTTP 422 |
| <code>interface.8.err-agent-boundary-bad-signature</code> | notes | HTTP 422 |
| <code>interface.8.err-agent-boundary-capability-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-agent-boundary-capability-unavailable</code> | overridden_by | override.J40.10 |
| <code>interface.8.err-agent-boundary-content-type-required</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-entitlement-denied</code> | notes | HTTP 403 |
| <code>interface.8.err-agent-boundary-head-stale</code> | notes | HTTP 503 |
| <code>interface.8.err-agent-boundary-head-stale</code> | overridden_by | override.J40.11 |
| <code>interface.8.err-agent-boundary-idempotency-conflict</code> | notes | HTTP 409; 30 callers total |
| <code>interface.8.err-agent-boundary-idempotency-key-required</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-identity-required</code> | notes | HTTP 401; 27 callers total |
| <code>interface.8.err-agent-boundary-invalid-activity-id</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-invalid-activity-length</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-invalid-asset-activity</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-invalid-deployment-body</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-agent-boundary-invalid-expected-state-root</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-invalid-idempotency-key</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-invalid-minimum-sequence</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-invalid-program-id</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-invalid-program-id</code> | overridden_by | override.J40.13 |
| <code>interface.8.err-agent-boundary-invalid-request</code> | notes | HTTP 400; 78 callers total |
| <code>interface.8.err-agent-boundary-invalid-staleness-ms</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-invalid-staleness-ms</code> | overridden_by | override.J40.14 |
| <code>interface.8.err-agent-boundary-lifecycle-receipt-invalid</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-agent-boundary-malformed-activity</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-malformed-program-call</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-malformed-program-deploy</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-malformed-program-upgrade</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-malformed-program-wasm</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-agent-boundary-malformed-program-wind-down</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-node-invalid</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-agent-boundary-node-transport-lost</code> | notes | HTTP 503 |
| <code>interface.8.err-agent-boundary-node-unavailable</code> | notes | HTTP 503; 68 callers total |
| <code>interface.8.err-agent-boundary-non-canonical-activity</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-not-found</code> | notes | HTTP 404; 135 callers total |
| <code>interface.8.err-agent-boundary-not-program-call</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-persistence-invalid</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-agent-boundary-persistence-unavailable</code> | notes | HTTP 503; 62 callers total |
| <code>interface.8.err-agent-boundary-program-artifacts-invalid</code> | notes | HTTP 503 |
| <code>interface.8.err-agent-boundary-program-artifacts-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-agent-boundary-program-not-registered</code> | notes | HTTP 404 |
| <code>interface.8.err-agent-boundary-program-not-registered</code> | overridden_by | override.J40.12 |
| <code>interface.8.err-agent-boundary-program-payload-hash-mismatch</code> | notes | HTTP 400 |
| <code>interface.8.err-agent-boundary-receipt-invalid</code> | notes | HTTP 503 |
| <code>interface.8.err-agent-boundary-receipt-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-agent-boundary-receipt-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-agent-boundary-snapshot-mismatch</code> | notes | HTTP 409 |
| <code>interface.8.err-agent-boundary-snapshot-stale</code> | notes | HTTP 409 |
| <code>interface.8.err-agent-boundary-unsupported-authority</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-agent-boundary-version-unsupported</code> | notes | HTTP 422 |
| <code>interface.8.err-agent-boundary-withdrawal-asset-unavailable</code> | notes | HTTP 422 |
| <code>interface.8.err-agent-boundary-wrong-network</code> | notes | HTTP 422 |

## agent-boundary-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-agent-boundary-any-internal-v1-receipts</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2185</code> | <code>if let Some(activity) = path.strip_prefix("/internal/v1/receipts/") {</code> |
| <code>interface.8.http-agent-boundary-any-v1</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2194</code> | <code>if !path.starts_with("/v1/") &#124;&#124; query.is_some() {</code> |
| <code>interface.8.http-agent-boundary-any-v1-batches</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2035</code> | <code>if let Some(rest) = path.strip_prefix("/v1/batches/") {</code> |
| <code>interface.8.http-agent-boundary-any-v1-programs</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2011</code> | <code>if let Some(rest) = path.strip_prefix("/v1/programs/") {</code> |
| <code>interface.8.http-agent-boundary-any-v1-programs-account-state-changes</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2006</code> | <code>if path == "/v1/programs/account-state/changes" {</code> |
| <code>interface.8.http-agent-boundary-any-v1-programs-activities</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2208</code> | <code>.or_else(&#124;&#124; path.strip_prefix("/v1/programs/activities/"))</code> |
| <code>interface.8.http-agent-boundary-any-v1-protocol-account-state-head</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2000</code> | <code>if path == "/v1/protocol/account-state/head" {</code> |
| <code>interface.8.http-agent-boundary-get-internal-v1-deployment-proof</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/deployment.rs:113</code> | <code>("GET", target) if target.starts_with("/internal/v1/deployment-proof/") =&gt; {</code> |
| <code>interface.8.http-agent-boundary-get-internal-v1-programs-head-attestation-j40</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/head_attestation.rs:12</code> | <code>GET /internal/v1/programs/{program_id}/head-attestation</code> |
| <code>interface.8.http-agent-boundary-get-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2160</code> | <code>if request.method == "GET" &amp;&amp; path == "/livez" &amp;&amp; query.is_none() {</code> |
| <code>interface.8.http-agent-boundary-get-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2163</code> | <code>if request.method == "GET" &amp;&amp; path == "/readyz" &amp;&amp; query.is_none() {</code> |
| <code>interface.8.http-agent-boundary-get-v1-programs-receipts-by-idempotency</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2229</code> | <code>if let Some(key) = target.strip_prefix("/v1/programs/receipts/by-idempotency/") {</code> |
| <code>interface.8.http-agent-boundary-get-v1-receipts</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2231</code> | <code>} else if let Some(activity) = target.strip_prefix("/v1/receipts/") {</code> |
| <code>interface.8.http-agent-boundary-post-internal-v1-programs-deploy</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/deployment.rs:111</code> | <code>("POST", "/internal/v1/programs/deploy") =&gt; submit(config, request, Route::ProgramDeploy),</code> |
| <code>interface.8.http-agent-boundary-post-internal-v1-programs-upgrade</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/deployment.rs:112</code> | <code>("POST", "/internal/v1/programs/upgrade") =&gt; submit(config, request, Route::ProgramUpgrade),</code> |
| <code>interface.8.http-agent-boundary-post-v1-activities</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2221</code> | <code>("POST", "/v1/activities") =&gt; submit_route(config, request, Route::Activities),</code> |
| <code>interface.8.http-agent-boundary-post-v1-programs-call</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2222</code> | <code>("POST", "/v1/programs/call") =&gt; submit_route(config, request, Route::ProgramCall),</code> |
| <code>interface.8.http-agent-boundary-post-v1-programs-deploy</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2223</code> | <code>("POST", "/v1/programs/deploy") =&gt; submit_route(config, request, Route::ProgramDeploy),</code> |
| <code>interface.8.http-agent-boundary-post-v1-programs-read</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2226</code> | <code>("POST", "/v1/programs/read") =&gt; program_read_route(config, request),</code> |
| <code>interface.8.http-agent-boundary-post-v1-programs-simulate</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2227</code> | <code>("POST", "/v1/programs/simulate") =&gt; simulate_route(config, request),</code> |
| <code>interface.8.http-agent-boundary-post-v1-programs-upgrade</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2224</code> | <code>("POST", "/v1/programs/upgrade") =&gt; submit_route(config, request, Route::ProgramUpgrade),</code> |
| <code>interface.8.http-agent-boundary-post-v1-programs-wind-down</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/agent-boundary/src/main.rs:2225</code> | <code>("POST", "/v1/programs/wind-down") =&gt; submit_route(config, request, Route::ProgramWindDown),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-agent-boundary-any-v1</code> | notes | path is too generic to attribute callers |
| <code>interface.8.http-agent-boundary-any-v1-batches</code> | notes | 45 callers total |
| <code>interface.8.http-agent-boundary-any-v1-programs</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 632 callers total |
| <code>interface.8.http-agent-boundary-any-v1-programs-account-state-changes</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439) |
| <code>interface.8.http-agent-boundary-any-v1-programs-activities</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 56 callers total |
| <code>interface.8.http-agent-boundary-any-v1-protocol-account-state-head</code> | notes | 28 callers total |
| <code>interface.8.http-agent-boundary-get-internal-v1-programs-head-attestation-j40</code> | notes | registry-plane relay of the kernel program head attestation; the registry reads 0x101D instead (task 8.12.1); retired by task 8.8.4 |
| <code>interface.8.http-agent-boundary-get-livez</code> | notes | 119 callers total |
| <code>interface.8.http-agent-boundary-get-readyz</code> | notes | 324 callers total |
| <code>interface.8.http-agent-boundary-get-v1-programs-receipts-by-idempotency</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 43 callers total |
| <code>interface.8.http-agent-boundary-get-v1-receipts</code> | notes | 56 callers total |
| <code>interface.8.http-agent-boundary-post-internal-v1-programs-deploy</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439) |
| <code>interface.8.http-agent-boundary-post-internal-v1-programs-upgrade</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); no in-repo client cites the path outside the defining file |
| <code>interface.8.http-agent-boundary-post-v1-activities</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 103 callers total |
| <code>interface.8.http-agent-boundary-post-v1-programs-call</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 82 callers total |
| <code>interface.8.http-agent-boundary-post-v1-programs-deploy</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 62 callers total |
| <code>interface.8.http-agent-boundary-post-v1-programs-read</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439) |
| <code>interface.8.http-agent-boundary-post-v1-programs-simulate</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 47 callers total |
| <code>interface.8.http-agent-boundary-post-v1-programs-upgrade</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 52 callers total |
| <code>interface.8.http-agent-boundary-post-v1-programs-wind-down</code> | notes | stale design removes the programs REST mapping in the agent boundary (spec/layerx-merge/spec.kvx:60435, spec/layerx-merge/spec.kvx:60439); 57 callers total |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.225.1</code> | req | 8225.1 |
| <code>acceptance.8.225.1</code> | scenario | Given the agent boundary tests directory, when it is listed and its files are searched, then the kernel node harness and both retired names are absent |
| <code>acceptance.8.225.1</code> | test | services/platform/hosted/tests/test_retired_agent_boundary_node_tests.py::test_agent_boundary_has_no_kernel_node_test (planned) |
| <code>acceptance.8.225.1</code> | qualification | UNRUN |
| <code>req.8225</code> | ac_1 | The agent boundary tests directory holds no real_node.rs and no real_node directory, and no remaining test file names layerx_wire or tests/bridge. |
| <code>acceptance.8.77.1</code> | req | 8077.1 |
| <code>acceptance.8.77.1</code> | scenario | Given a chain listener answering pax_sequencerReceipt with a receipt signed by the pinned key, when the gateway plane gets /v1/receipts for that transaction, then the answer is 200 with the same receipt bytes |
| <code>acceptance.8.77.1</code> | test | services/platform/hosted/agent-boundary/tests/chain_receipt.rs::receipt_route_reads_the_sequencer_receipt_of_the_transaction (planned) |
| <code>acceptance.8.77.1</code> | qualification | UNRUN |
| <code>req.8077</code> | ac_1 | GET /v1/receipts/&lt;tx hash&gt; on the gateway plane answers the signed receipt pax_sequencerReceipt returned once its signature verifies against the pinned key list. |
| <code>acceptance.8.77.2</code> | req | 8077.2 |
| <code>acceptance.8.77.2</code> | scenario | Given a chain listener answering null for one hash and then a closed endpoint, when the webhook plane gets the internal receipt route twice, then it answers 404 receipt_not_found and then 503 node_unavailable |
| <code>acceptance.8.77.2</code> | test | services/platform/hosted/agent-boundary/tests/chain_receipt.rs::internal_receipt_route_refuses_an_unknown_transaction (planned) |
| <code>acceptance.8.77.2</code> | qualification | UNRUN |
| <code>req.8077</code> | ac_2 | GET /internal/v1/receipts/&lt;tx hash&gt; answers 404 receipt_not_found for a transaction the chain has no receipt for and 503 node_unavailable when the chain endpoint is down. |
| <code>acceptance.8.78.1</code> | req | 8078.1 |
| <code>acceptance.8.78.1</code> | scenario | Given a chain listener holding the receipt of a deployment transaction, when the registry plane gets its deployment proof and posts a deploy body, then the proof answers the receipt and the deploy answers 404 |
| <code>acceptance.8.78.1</code> | test | services/platform/hosted/agent-boundary/tests/chain_receipt.rs::deployment_proof_reads_the_deployment_receipt (planned) |
| <code>acceptance.8.78.1</code> | qualification | UNRUN |
| <code>req.8078</code> | ac_1 | GET /internal/v1/deployment-proof/&lt;tx hash&gt; on the registry plane answers the sequencer receipt of the deployment transaction, and POST /internal/v1/programs/deploy answers 404 not_found. |
| <code>acceptance.8.79.1</code> | req | 8079.1 |
| <code>acceptance.8.79.1</code> | scenario | Given the gateway plane, when each retired method and path is routed, then each answers 404 not_found and a receipt read reaches receipt_route |
| <code>acceptance.8.79.1</code> | test | services/platform/hosted/agent-boundary/src/lifecycle_tests.rs::retired_program_routes_answer_not_found (planned) |
| <code>acceptance.8.79.1</code> | qualification | UNRUN |
| <code>req.8079</code> | ac_1 | Every retired gateway-plane path (POST /v1/activities, the six POST /v1/programs routes and the program activity, idempotency and account-state change reads) answers 404 not_found while GET /v1/receipts/&lt;id&gt; is still routed. |
| <code>capability.8.19.6</code> | title | Delete the kernel node tests of the agent boundary |
| <code>capability.8.19.6</code> | reqs | 8225.1 |
| <code>capability.8.8.1</code> | title | Answer agent-boundary receipts from pax_sequencerReceipt |
| <code>capability.8.8.1</code> | reqs | 8077.1<br>8077.2 |
| <code>capability.8.8.2</code> | title | Retire agent-boundary program deployment submission and read deployment proofs from the chain |
| <code>capability.8.8.2</code> | reqs | 8078.1 |
| <code>capability.8.8.3</code> | title | Retire the kernel activity and program routes of the agent-boundary gateway plane |
| <code>capability.8.8.3</code> | reqs | 8079.1 |
| <code>resolution.17.802</code> | title | decision.807 binds: Agent-boundary receipt identifiers are EVM transaction hashes |
| <code>resolution.17.802</code> | applies_to | decision.807<br>task.8.8.1<br>task.8.8.2 |
| <code>resolution.17.802</code> | source | decision.807 in fragments/J40/decision.kvx; tasks from affects |
| <code>resolution.17.807</code> | title | decision.812 binds: Hosted readers pin receipt keys from a file, not from checkpoints |
| <code>resolution.17.807</code> | applies_to | decision.812<br>task.8.8.1<br>task.8.11.1<br>task.8.11.2 |
| <code>resolution.17.807</code> | source | decision.812 in fragments/J40/decision.kvx; tasks from affects |
| <code>service.platform-agent-boundary</code> | purpose | Authenticated boundary between the gateway, registry and webhook planes and the chain for receipt and deployment-proof reads |
| <code>service.platform-agent-boundary</code> | route | GET /v1/receipts/{id}, GET /internal/v1/receipts/{id}, GET /internal/v1/deployment-proof/{id}, GET /v1/protocol/account-state/head, /livez, /readyz |
| <code>service.platform-agent-boundary</code> | inputs | plane bearer tokens, LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL, LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE, LAYERX_AGENT_BOUNDARY_NODE_URL |
| <code>service.platform-agent-boundary</code> | outputs | signed sequencer receipts and relayed account-state answers |
| <code>service.platform-agent-boundary</code> | state | state directory with journal and activity records |
| <code>service.platform-agent-boundary</code> | dependencies | chain JSON-RPC, local node relay |
| <code>service.platform-agent-boundary</code> | failure_policy | an unreachable chain answers node_unavailable 503 with retry; an unverifiable receipt is never relayed |
| <code>service.platform-agent-boundary</code> | completion_task | 8.0.9 |
| <code>service.platform-agent-boundary</code> | operation_task | 8.8.4 |
| <code>service.platform-agent-boundary</code> | domain | 8 |

## agentd-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-agentd-probe-sh-bearer-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:44</code> | <code>--bearer-file) bearer_file=${2:?probe.sh: --bearer-file needs a value}; shift 2 ;;</code> |
| <code>interface.8.cli-agentd-probe-sh-ca</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:41</code> | <code>--ca) ca=${2:?probe.sh: --ca needs a value}; shift 2 ;;</code> |
| <code>interface.8.cli-agentd-probe-sh-client-cert</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:42</code> | <code>--client-cert) client_cert=${2:?probe.sh: --client-cert needs a value}; shift 2 ;;</code> |
| <code>interface.8.cli-agentd-probe-sh-client-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:43</code> | <code>--client-key) client_key=${2:?probe.sh: --client-key needs a value}; shift 2 ;;</code> |
| <code>interface.8.cli-agentd-probe-sh-credential-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:48</code> | <code>--credential-file) credential_file=${2:?probe.sh: --credential-file needs a value}; shift 2 ;;</code> |
| <code>interface.8.cli-agentd-probe-sh-gateway-key-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:47</code> | <code>--gateway-key-file) gateway_key_file=${2:?probe.sh: --gateway-key-file needs a value}; shift 2 ;;</code> |
| <code>interface.8.cli-agentd-probe-sh-journey</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:45</code> | <code>--journey) journey=${2:?probe.sh: --journey needs a value}; shift 2 ;;</code> |
| <code>interface.8.cli-agentd-probe-sh-rpc-url</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:46</code> | <code>--rpc-url) rpc_url=${2:?probe.sh: --rpc-url needs a value}; shift 2 ;;</code> |
| <code>interface.8.cli-agentd-probe-sh-url</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/agentd/probe.sh:40</code> | <code>--url) url=${2:?probe.sh: --url needs a value}; shift 2 ;;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-agentd-probe-sh-credential-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-agentd-probe-sh-gateway-key-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-agentd-probe-sh-journey</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-agentd-probe-sh-rpc-url</code> | notes | no in-repo invocation outside the defining file |

## agentd-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-agentd-human-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agentd/manifest_check.py:186</code> | <code>container["env"].append({"name": "LAYERX_AGENTD_OWNER_SOCKET", "value": values["HUMAN_SOCKET"]})</code> |
| <code>interface.8.env-agentd-layerx-agentd-bearer-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agentd/live-check.sh:59</code> | <code>if [ -z "${LAYERX_AGENTD_BEARER_FILE:-}" ] &amp;&amp; [ "$env_read" = 1 ]; then</code> |
| <code>interface.8.env-agentd-layerx-agentd-owner-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agentd/manifest_check.py:186</code> | <code>container["env"].append({"name": "LAYERX_AGENTD_OWNER_SOCKET", "value": values["HUMAN_SOCKET"]})</code> |
| <code>interface.8.env-agentd-layerx-agentd-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agentd/live-check.sh:52</code> | <code>case "${LAYERX_AGENTD_URL:-}" in</code> |
| <code>interface.8.env-agentd-platform-beta-cluster-env</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/agentd/live-check.sh:44</code> | <code>env_file=${PLATFORM_BETA_CLUSTER_ENV:-build/beta-cluster/env}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-agentd-layerx-agentd-bearer-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-agentd-layerx-agentd-owner-socket</code> | notes | no in-repo setter or reader outside the defining file |

## ai-artifacts-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-ai-artifacts-main-rs-config</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:1715</code> | <code>(arg("--data-dir"), arg("--config"), arg("--key-file"))</code> |
| <code>interface.8.cli-ai-artifacts-main-rs-data-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:1715</code> | <code>(arg("--data-dir"), arg("--config"), arg("--key-file"))</code> |
| <code>interface.8.cli-ai-artifacts-main-rs-key-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:1715</code> | <code>(arg("--data-dir"), arg("--config"), arg("--key-file"))</code> |
| <code>interface.8.cli-ai-artifacts-main-rs-listen</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:1720</code> | <code>let listen = arg("--listen").unwrap_or_else(&#124;&#124; "&lt;addr&gt;".to_string());</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-ai-artifacts-main-rs-config</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-ai-artifacts-main-rs-data-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-ai-artifacts-main-rs-key-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-ai-artifacts-main-rs-listen</code> | notes | no in-repo invocation outside the defining file |

## ai-artifacts-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-ai-artifacts-config-capacity-bytes</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/authority.rs:17</code> | <code>pub capacity_bytes: u64,</code> |
| <code>interface.8.cfg-ai-artifacts-config-commit-readers</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/authority.rs:30</code> | <code>pub commit_readers: Vec&lt;String&gt;,</code> |
| <code>interface.8.cfg-ai-artifacts-config-listener</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/authority.rs:18</code> | <code>pub listener: Listener,</code> |
| <code>interface.8.cfg-ai-artifacts-config-publishers</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/authority.rs:32</code> | <code>pub publishers: Vec&lt;Publisher&gt;,</code> |
| <code>interface.8.cfg-ai-artifacts-config-retention-mark</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/authority.rs:24</code> | <code>pub retention_mark: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cfg-ai-artifacts-config-revocation-mark</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/authority.rs:20</code> | <code>pub revocation_mark: PathBuf,</code> |
| <code>interface.8.cfg-ai-artifacts-config-tenants</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/authority.rs:31</code> | <code>pub tenants: Vec&lt;Tenant&gt;,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-ai-artifacts-config-commit-readers</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ai-artifacts-config-listener</code> | notes | 45 callers total |
| <code>interface.8.cfg-ai-artifacts-config-publishers</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ai-artifacts-config-retention-mark</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ai-artifacts-config-revocation-mark</code> | notes | no in-repo config file sets the key outside the defining file |

## ai-artifacts-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-ai-artifacts-events-log</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/store.rs:423</code> | <code>.open(self.dir.join("events.log"))</code> |
| <code>interface.8.fmt-ai-artifacts-retention-json</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/retention.rs:444</code> | <code>let path = dir.join("retention.json");</code> |
| <code>interface.8.fmt-ai-artifacts-state-json</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/store.rs:255</code> | <code>let path = self.dir.join("state.json");</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-ai-artifacts-events-log</code> | notes | artifact store event log |
| <code>interface.8.fmt-ai-artifacts-retention-json</code> | notes | retention state file; read and written only by the defining module |
| <code>interface.8.fmt-ai-artifacts-state-json</code> | notes | artifact store state file; 71 callers total |

## ai-artifacts-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-ai-artifacts-any-v1-content</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:549</code> | <code>"/v1/content" =&gt; return self.content(tenant, body),</code> |
| <code>interface.8.http-ai-artifacts-any-v1-deliveries</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:550</code> | <code>"/v1/deliveries" =&gt; self.delivery_status(tenant, body),</code> |
| <code>interface.8.http-ai-artifacts-any-v1-finalize</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:457</code> | <code>"/v1/finalize" =&gt; self.with_id::&lt;FinalizeReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-grants</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:464</code> | <code>"/v1/grants" =&gt; self.with_id::&lt;GrantReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-import</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:456</code> | <code>"/v1/import" =&gt; reply(self.import(tenant, body)),</code> |
| <code>interface.8.http-ai-artifacts-any-v1-locators-check</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:551</code> | <code>"/v1/locators/check" =&gt; reply(parse::&lt;LocatorReq&gt;(body).and_then(&#124;r&#124; {</code> |
| <code>interface.8.http-ai-artifacts-any-v1-prepare</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:446</code> | <code>"/v1/prepare" =&gt; {</code> |
| <code>interface.8.http-ai-artifacts-any-v1-publishers-revoke</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:492</code> | <code>"/v1/publishers/revoke" =&gt; self.with_id::&lt;PublisherRevokeReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-retention-binding</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:541</code> | <code>"/v1/retention/binding" =&gt; self.with_id::&lt;StateHeadReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-retention-binding-intent</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:520</code> | <code>"/v1/retention/binding-intent" =&gt; self.with_id::&lt;RootReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-retention-hold</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:499</code> | <code>"/v1/retention/hold" =&gt; self.with_id::&lt;HoldReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-retention-orphan</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:527</code> | <code>"/v1/retention/orphan" =&gt; self.with_id::&lt;RootReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-retention-projection</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:548</code> | <code>"/v1/retention/projection" =&gt; reply(self.projection(body)),</code> |
| <code>interface.8.http-ai-artifacts-any-v1-retention-reinstate</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:513</code> | <code>"/v1/retention/reinstate" =&gt; self.with_id::&lt;ReinstateReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-retention-release</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:506</code> | <code>"/v1/retention/release" =&gt; self.with_id::&lt;HoldReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-retention-terminal</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:534</code> | <code>"/v1/retention/terminal" =&gt; self.with_id::&lt;StateHeadReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-revoke</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:471</code> | <code>"/v1/revoke" =&gt; self.with_id::&lt;RevokeReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-rotate</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:478</code> | <code>"/v1/rotate" =&gt; self.with_id::&lt;RootReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-any-v1-tombstone</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:485</code> | <code>"/v1/tombstone" =&gt; self.with_id::&lt;RootReq, _&gt;(</code> |
| <code>interface.8.http-ai-artifacts-get-v1-usage</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:438</code> | <code>if req.method == "GET" &amp;&amp; path == "/v1/usage" {</code> |
| <code>interface.8.http-ai-artifacts-post-v1-resolve</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:417</code> | <code>if req.method != "POST" &#124;&#124; req.path != "/v1/resolve" {</code> |
| <code>interface.8.http-ai-artifacts-put-v1-upload</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-artifacts/src/main.rs:435</code> | <code>if let ("PUT", Some(rest)) = (req.method.as_str(), path.strip_prefix("/v1/upload/")) {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-ai-artifacts-post-v1-resolve</code> | notes | 28 callers total |

## ai-evaluator-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-ai-evaluator-config-access-declaration</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:910</code> | <code>access_declaration: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-assessments-dir</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:917</code> | <code>assessments_dir: PathBuf,</code> |
| <code>interface.8.cfg-ai-evaluator-config-budget</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:908</code> | <code>budget: [u64; 7],</code> |
| <code>interface.8.cfg-ai-evaluator-config-capabilities</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:909</code> | <code>capabilities: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-chain</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:911</code> | <code>chain: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-evaluator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:913</code> | <code>evaluator: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-evm-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:903</code> | <code>evm_chain_id: u64,</code> |
| <code>interface.8.cfg-ai-evaluator-config-fee-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:906</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-gas-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:904</code> | <code>gas_limit: u64,</code> |
| <code>interface.8.cfg-ai-evaluator-config-near-end-heights</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:919</code> | <code>near_end_heights: u64,</code> |
| <code>interface.8.cfg-ai-evaluator-config-owner</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:914</code> | <code>owner: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-poll-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:918</code> | <code>poll_ms: u64,</code> |
| <code>interface.8.cfg-ai-evaluator-config-program</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:912</code> | <code>program: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-reader</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:915</code> | <code>reader: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-receipt-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:920</code> | <code>receipt_timeout_ms: u64,</code> |
| <code>interface.8.cfg-ai-evaluator-config-response-capacity</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:907</code> | <code>response_capacity: u32,</code> |
| <code>interface.8.cfg-ai-evaluator-config-rpc-url</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:902</code> | <code>rpc_url: String,</code> |
| <code>interface.8.cfg-ai-evaluator-config-state-dir</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:916</code> | <code>state_dir: PathBuf,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-ai-evaluator-config-assessments-dir</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ai-evaluator-config-budget</code> | notes | 103 callers total |
| <code>interface.8.cfg-ai-evaluator-config-capabilities</code> | notes | 174 callers total |
| <code>interface.8.cfg-ai-evaluator-config-chain</code> | notes | 451 callers total |
| <code>interface.8.cfg-ai-evaluator-config-fee-limit</code> | notes | 88 callers total |
| <code>interface.8.cfg-ai-evaluator-config-gas-limit</code> | notes | 217 callers total |
| <code>interface.8.cfg-ai-evaluator-config-near-end-heights</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ai-evaluator-config-owner</code> | notes | 1260 callers total |
| <code>interface.8.cfg-ai-evaluator-config-poll-ms</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ai-evaluator-config-program</code> | notes | 120 callers total |
| <code>interface.8.cfg-ai-evaluator-config-reader</code> | notes | 51 callers total |
| <code>interface.8.cfg-ai-evaluator-config-receipt-timeout-ms</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-ai-evaluator-config-rpc-url</code> | notes | 48 callers total |

## ai-evaluator-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-ai-evaluator-layerx-ai-evaluator-config</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:75</code> | <code>const CONFIG_VARIABLE: &amp;str = "LAYERX_AI_EVALUATOR_CONFIG";</code> |
| <code>interface.8.env-ai-evaluator-layerx-ai-evaluator-evm-sender</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:78</code> | <code>const EVM_SENDER_PREFIX: &amp;str = "LAYERX_AI_EVALUATOR_EVM_SENDER";</code> |
| <code>interface.8.env-ai-evaluator-layerx-ai-evaluator-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:76</code> | <code>const KEY_VARIABLE: &amp;str = "LAYERX_AI_EVALUATOR_KEY_FILE";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-ai-evaluator-layerx-ai-evaluator-config</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-ai-evaluator-layerx-ai-evaluator-evm-sender</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-ai-evaluator-layerx-ai-evaluator-key-file</code> | notes | no in-repo setter or reader outside the defining file |

## ai-evaluator-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-ai-evaluator-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:888</code> | <code>&#124;_&#124; refusal(400, "invalid_request", None),</code> |
| <code>interface.8.err-ai-evaluator-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:857</code> | <code>_ =&gt; refusal(404, "not_found", None),</code> |
| <code>interface.8.err-ai-evaluator-not-ready</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:855</code> | <code>("GET", "/readyz") =&gt; refusal(503, "not_ready", None),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-ai-evaluator-invalid-request</code> | notes | HTTP 400; 78 callers total |
| <code>interface.8.err-ai-evaluator-not-found</code> | notes | HTTP 404; 136 callers total |
| <code>interface.8.err-ai-evaluator-not-ready</code> | notes | HTTP 503 |

## ai-evaluator-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-ai-evaluator-get-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:853</code> | <code>("GET", "/healthz") =&gt; ok(r#"{"status":"ok"}"#.to_owned()),</code> |
| <code>interface.8.http-ai-evaluator-get-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:856</code> | <code>("GET", "/metrics") =&gt; ok(metrics.render()),</code> |
| <code>interface.8.http-ai-evaluator-get-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:854</code> | <code>("GET", "/readyz") if ready =&gt; ok(r#"{"status":"ready"}"#.to_owned()),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-ai-evaluator-get-healthz</code> | notes | 331 callers total |
| <code>interface.8.http-ai-evaluator-get-metrics</code> | notes | 323 callers total |
| <code>interface.8.http-ai-evaluator-get-readyz</code> | notes | 323 callers total |

## ai-evaluator-metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.met-ai-evaluator-paxai-evaluator-active</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:814</code> | <code>("active", "gauge", count(self.active)),</code> |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-applied-total</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:823</code> | <code>("applied_total", "counter", count(self.applied)),</code> |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-below-quorum</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:816</code> | <code>("below_quorum", "gauge", u64::from(self.active &lt; QUORUM)),</code> |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-epoch</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:813</code> | <code>("epoch", "gauge", self.epoch),</code> |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-forfeited-total</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:825</code> | <code>("forfeited_total", "counter", count(self.forfeited)),</code> |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-quorum</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:815</code> | <code>("quorum", "gauge", count(QUORUM)),</code> |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-refused-total</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:824</code> | <code>("refused_total", "counter", count(self.refused)),</code> |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-revealed</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:817</code> | <code>("revealed", "gauge", count(self.revealed)),</code> |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-unrevealed-near-end</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/ai-evaluator/src/main.rs:819</code> | <code>"unrevealed_near_end",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.met-ai-evaluator-paxai-evaluator-active</code> | notes | Prometheus text exposition from Metrics::render |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-applied-total</code> | notes | Prometheus text exposition from Metrics::render |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-below-quorum</code> | notes | Prometheus text exposition from Metrics::render |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-epoch</code> | notes | Prometheus text exposition from Metrics::render |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-forfeited-total</code> | notes | Prometheus text exposition from Metrics::render |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-quorum</code> | notes | Prometheus text exposition from Metrics::render |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-refused-total</code> | notes | Prometheus text exposition from Metrics::render; no in-repo scraper or dashboard cites it |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-revealed</code> | notes | Prometheus text exposition from Metrics::render |
| <code>interface.8.met-ai-evaluator-paxai-evaluator-unrevealed-near-end</code> | notes | Prometheus text exposition from Metrics::render |

## authority-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-authority-main-rs-help</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1412</code> | <code>.any(&#124;argument&#124; argument == "--help" &#124;&#124; argument == "-h")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-authority-main-rs-help</code> | notes | no in-repo invocation outside the defining file |

## authority-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-authority-keypolicy-effective-sequence</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:91</code> | <code>effective_sequence: u64,</code> |
| <code>interface.8.cfg-authority-keypolicy-evidence</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:92</code> | <code>evidence: EvidenceReference,</code> |
| <code>interface.8.cfg-authority-keypolicy-maximum-delay-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:90</code> | <code>maximum_delay_seconds: u64,</code> |
| <code>interface.8.cfg-authority-keypolicy-policy-revision</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:88</code> | <code>policy_revision: u64,</code> |
| <code>interface.8.cfg-authority-keypolicy-required-delay-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:89</code> | <code>required_delay_seconds: u64,</code> |
| <code>interface.8.cfg-authority-policy-principals</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:26</code> | <code>principals: Vec&lt;PrincipalPolicy&gt;,</code> |
| <code>interface.8.cfg-authority-principalpolicy-account-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:34</code> | <code>account_id: String,</code> |
| <code>interface.8.cfg-authority-principalpolicy-activities</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:36</code> | <code>activities: Vec&lt;String&gt;,</code> |
| <code>interface.8.cfg-authority-principalpolicy-asset-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:35</code> | <code>asset_id: String,</code> |
| <code>interface.8.cfg-authority-principalpolicy-budgets</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:37</code> | <code>budgets: Vec&lt;String&gt;,</code> |
| <code>interface.8.cfg-authority-principalpolicy-identities</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:40</code> | <code>identities: Vec&lt;Identity&gt;,</code> |
| <code>interface.8.cfg-authority-principalpolicy-maximum-age-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:38</code> | <code>maximum_age_seconds: u64,</code> |
| <code>interface.8.cfg-authority-principalpolicy-maximum-age-sequences</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:39</code> | <code>maximum_age_sequences: u64,</code> |
| <code>interface.8.cfg-authority-principalpolicy-principal</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:33</code> | <code>principal: String,</code> |
| <code>interface.8.cfg-authority-principalpolicy-tenant</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:32</code> | <code>tenant: String,</code> |
| <code>interface.8.cfg-authority-registry-assets</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:99</code> | <code>assets: Vec&lt;CurrencyMetadata&gt;,</code> |
| <code>interface.8.cfg-authority-registry-modules</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:100</code> | <code>modules: Vec&lt;Module&gt;,</code> |
| <code>interface.8.cfg-authority-registry-schema-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:98</code> | <code>schema_version: u16,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-authority-keypolicy-evidence</code> | notes | 951 callers total |
| <code>interface.8.cfg-authority-policy-principals</code> | notes | 36 callers total |
| <code>interface.8.cfg-authority-principalpolicy-account-id</code> | notes | 108 callers total |
| <code>interface.8.cfg-authority-principalpolicy-activities</code> | notes | 41 callers total |
| <code>interface.8.cfg-authority-principalpolicy-asset-id</code> | notes | 271 callers total |
| <code>interface.8.cfg-authority-principalpolicy-budgets</code> | notes | 35 callers total |
| <code>interface.8.cfg-authority-principalpolicy-identities</code> | notes | 48 callers total |
| <code>interface.8.cfg-authority-principalpolicy-principal</code> | notes | 185 callers total |
| <code>interface.8.cfg-authority-principalpolicy-tenant</code> | notes | 194 callers total |
| <code>interface.8.cfg-authority-registry-assets</code> | notes | 185 callers total |
| <code>interface.8.cfg-authority-registry-modules</code> | notes | 106 callers total |
| <code>interface.8.cfg-authority-registry-schema-version</code> | notes | 140 callers total |

## authority-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-authority-layerx-authority-chain-rpc-url-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/authority/src/chain_receipt.rs</code> (planned) | <code>LAYERX_AUTHORITY_CHAIN_RPC_URL</code> |
| <code>interface.8.env-authority-layerx-authority-client-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:224</code> | <code>let config = if env::var_os("LAYERX_AUTHORITY_CLIENT_CA_DER").is_some() {</code> |
| <code>interface.8.env-authority-layerx-authority-core-clock-horizon-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:307</code> | <code>let horizon = required("LAYERX_AUTHORITY_CORE_CLOCK_HORIZON")?</code> |
| <code>interface.8.env-authority-layerx-authority-evidence-read-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:155</code> | <code>let Some(path) = env::var_os("LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE") else {</code> |
| <code>interface.8.env-authority-layerx-authority-genesis-trust</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/trust.rs:58</code> | <code>let genesis = env::var_os("LAYERX_AUTHORITY_GENESIS_TRUST");</code> |
| <code>interface.8.env-authority-layerx-authority-handover-finality</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/trust.rs:59</code> | <code>let finality = env::var_os("LAYERX_AUTHORITY_HANDOVER_FINALITY");</code> |
| <code>interface.8.env-authority-layerx-authority-human-agent-principal-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:317</code> | <code>let principal = required("LAYERX_AUTHORITY_HUMAN_AGENT_PRINCIPAL")?;</code> |
| <code>interface.8.env-authority-layerx-authority-human-agent-tenant-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:316</code> | <code>let tenant = required("LAYERX_AUTHORITY_HUMAN_AGENT_TENANT")?;</code> |
| <code>interface.8.env-authority-layerx-authority-human-agent-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:266</code> | <code>if std::env::var_os("LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE").is_none() {</code> |
| <code>interface.8.env-authority-layerx-authority-identity-binding-gid-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:35</code> | <code>"LAYERX_AUTHORITY_IDENTITY_BINDING_GID",</code> |
| <code>interface.8.env-authority-layerx-authority-identity-binding-socket-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:33</code> | <code>"LAYERX_AUTHORITY_IDENTITY_BINDING_SOCKET",</code> |
| <code>interface.8.env-authority-layerx-authority-identity-binding-uid-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:34</code> | <code>"LAYERX_AUTHORITY_IDENTITY_BINDING_UID",</code> |
| <code>interface.8.env-authority-layerx-authority-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:247</code> | <code>let listen = env::var("LAYERX_AUTHORITY_LISTEN")</code> |
| <code>interface.8.env-authority-layerx-authority-lni-socket</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/main.rs:281</code> | <code>let lni_socket = env::var("LAYERX_AUTHORITY_LNI_SOCKET")</code> |
| <code>interface.8.env-authority-layerx-authority-module-registry-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:330</code> | <code>registry_path: PathBuf::from(required("LAYERX_AUTHORITY_MODULE_REGISTRY_FILE")?),</code> |
| <code>interface.8.env-authority-layerx-authority-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:294</code> | <code>let network_id = env::var("LAYERX_AUTHORITY_NETWORK_ID")</code> |
| <code>interface.8.env-authority-layerx-authority-principal-policy-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:299</code> | <code>let policy_path = PathBuf::from(required("LAYERX_AUTHORITY_PRINCIPAL_POLICY_FILE")?);</code> |
| <code>interface.8.env-authority-layerx-authority-protocol-network-id-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:287</code> | <code>let protocol_network_id = parse_u64("LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID")?;</code> |
| <code>interface.8.env-authority-layerx-authority-receipt-keys-file-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/authority/src/chain_receipt.rs</code> (planned) | <code>LAYERX_AUTHORITY_RECEIPT_KEYS_FILE</code> |
| <code>interface.8.env-authority-layerx-authority-replica-bearer-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:268</code> | <code>let replica_token = read_secret("LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE")?;</code> |
| <code>interface.8.env-authority-layerx-authority-replica-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:261</code> | <code>let (replica_host, replica_address) = env::var("LAYERX_AUTHORITY_REPLICA_URL")</code> |
| <code>interface.8.env-authority-layerx-authority-state-root-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:313</code> | <code>let state_root = PathBuf::from(required("LAYERX_AUTHORITY_STATE_ROOT")?);</code> |
| <code>interface.8.env-authority-layerx-authority-tls-cert-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:220</code> | <code>let certificate = CertificateDer::from(read_bounded("LAYERX_AUTHORITY_TLS_CERT_DER")?);</code> |
| <code>interface.8.env-authority-layerx-authority-tls-key-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:221</code> | <code>let private_key = PrivateKeyDer::try_from(read_bounded("LAYERX_AUTHORITY_TLS_KEY_DER")?)</code> |
| <code>interface.8.env-authority-layerx-authority-token-files</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:252</code> | <code>let token_files = env::var("LAYERX_AUTHORITY_TOKEN_FILES")</code> |
| <code>interface.8.env-authority-layerx-authority-wire-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:297</code> | <code>env::var("LAYERX_AUTHORITY_WIRE_VERSION").unwrap_or_else(&#124;_&#124; PROTOCOL_VERSION.to_string());</code> |
| <code>interface.8.env-authority-paxeer-x-lni-capture-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1435</code> | <code>fs::read(env::var("PAXEER_X_LNI_CAPTURE").expect("actual LNI capture required"))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-authority-layerx-authority-chain-rpc-url-j40</code> | notes | planned chain JSON-RPC endpoint for pax_sequencerReceipt lookups; task 8.11.1 |
| <code>interface.8.env-authority-layerx-authority-core-clock-horizon-j69</code> | notes | read at services/platform/hosted/authority/src/human.rs:307; 10 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-human-agent-principal-j69</code> | notes | read at services/platform/hosted/authority/src/human.rs:317; 10 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-human-agent-tenant-j69</code> | notes | read at services/platform/hosted/authority/src/human.rs:316; 10 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-identity-binding-gid-j69</code> | notes | read at services/platform/hosted/authority/src/human/dynamic.rs:35 |
| <code>interface.8.env-authority-layerx-authority-identity-binding-socket-j69</code> | notes | read at services/platform/hosted/authority/src/human/dynamic.rs:33 |
| <code>interface.8.env-authority-layerx-authority-identity-binding-uid-j69</code> | notes | read at services/platform/hosted/authority/src/human/dynamic.rs:34 |
| <code>interface.8.env-authority-layerx-authority-lni-socket</code> | notes | LNI transport retired; reads move to eth_call and gRPC queries (spec/layerx-merge/spec.kvx:60434) |
| <code>interface.8.env-authority-layerx-authority-module-registry-file-j69</code> | notes | read at services/platform/hosted/authority/src/human.rs:330; 9 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-principal-policy-file-j69</code> | notes | read at services/platform/hosted/authority/src/human.rs:299; 9 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-protocol-network-id-j69</code> | notes | read at services/platform/hosted/authority/src/main.rs:287; 22 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-receipt-keys-file-j40</code> | notes | planned pinned sequencer receipt key list; task 8.11.1 |
| <code>interface.8.env-authority-layerx-authority-replica-bearer-token-file-j69</code> | notes | read at services/platform/hosted/authority/src/main.rs:268; 18 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-state-root-j69</code> | notes | read at services/platform/hosted/authority/src/human.rs:313; 9 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-tls-cert-der-j69</code> | notes | read at services/platform/hosted/authority/src/main.rs:220; 18 in-repo references, first 8 listed |
| <code>interface.8.env-authority-layerx-authority-tls-key-der-j69</code> | notes | read at services/platform/hosted/authority/src/main.rs:221; 19 in-repo references, first 8 listed |
| <code>interface.8.env-authority-paxeer-x-lni-capture-j69</code> | notes | read at services/platform/hosted/authority/src/main.rs:1435 |

Contract context <code>contract_context.8.hosted-chain-readers</code>:

| Field | Frozen rule |
|---|---|
| interface | Chain JSON-RPC clients of the core boundary funding path, the receipt authority and the program registry |
| storage | core funding journal; authority receipt store; registry journal |
| errors | treasury_unavailable 503, identity_binding_unavailable 409, idempotency_conflict 409, deployment_proof_refused 422, protocol_state_unavailable 503 |
| events | none |
| gas | one value transfer per funding command, paid by the funding sender |
| activation | wave 3 moves the readers, wave 6 removes the kernel sockets |
| invariants | a funding command signs at most one transaction; every admitted receipt verifies under a pinned key; every registry head is read at one block height |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.113.1</code> | req | 8113.1 |
| <code>acceptance.8.113.1</code> | scenario | Given an environment without the LNI variable and a chain listener, when the authority starts and a receipt is admitted, then it is ready and the receipt is admitted |
| <code>acceptance.8.113.1</code> | test | services/platform/hosted/authority/tests/chain_receipts.rs::authority_starts_without_an_lni_socket (planned) |
| <code>acceptance.8.113.1</code> | qualification | UNRUN |
| <code>req.8113</code> | ac_1 | The receipt authority starts with no LAYERX_AUTHORITY_LNI_SOCKET, reports ready against the chain endpoint and admits a chain receipt. |
| <code>capability.8.11.3</code> | title | Remove the kernel socket from the receipt authority |
| <code>capability.8.11.3</code> | reqs | 8113.1 |

## authority-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-authority-activity-not-bound</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:625</code> | <code>return Err(refusal(403, "activity_not_bound", None));</code> |
| <code>interface.8.err-authority-budget-not-bound</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:632</code> | <code>return Err(refusal(404, "budget_not_bound", None));</code> |
| <code>interface.8.err-authority-capability-not-bound</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:858</code> | <code>.ok_or_else(&#124;&#124; refusal(403, "capability_not_bound", None))?;</code> |
| <code>interface.8.err-authority-child-not-bound</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:241</code> | <code>.map_err(&#124;_&#124; refusal(403, "child_not_bound", None))?;</code> |
| <code>interface.8.err-authority-did-not-bound</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:846</code> | <code>.ok_or_else(&#124;&#124; refusal(404, "did_not_bound", None))?;</code> |
| <code>interface.8.err-authority-duplicate-query</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:474</code> | <code>return Err(refusal(400, "duplicate_query", None));</code> |
| <code>interface.8.err-authority-evidence-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:843</code> | <code>refusal(502, "evidence_refused", None)</code> |
| <code>interface.8.err-authority-identity-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:513</code> | <code>.ok_or_else(&#124;&#124; refusal(401, "identity_required", None))?;</code> |
| <code>interface.8.err-authority-invalid-activity-id</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:622</code> | <code>return Err(refusal(400, "invalid_activity_id", None));</code> |
| <code>interface.8.err-authority-invalid-budget-id</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/budget_state.rs:464</code> | <code>let id = hex::decode32(id).map_err(&#124;_&#124; refusal(400, "invalid_budget_id", None))?;</code> |
| <code>interface.8.err-authority-invalid-query</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:470</code> | <code>.ok_or_else(&#124;&#124; refusal(400, "invalid_query", None))?;</code> |
| <code>interface.8.err-authority-invalid-recovery</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:864</code> | <code>_ =&gt; return Err(refusal(400, "invalid_recovery", None)),</code> |
| <code>interface.8.err-authority-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1078</code> | <code>return refusal(400, "invalid_request", None);</code> |
| <code>interface.8.err-authority-method-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1249</code> | <code>return refusal(405, "method_not_allowed", None);</code> |
| <code>interface.8.err-authority-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:527</code> | <code>.ok_or_else(&#124;&#124; refusal(404, "not_found", None))?;</code> |
| <code>interface.8.err-authority-original-activity-authority-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:459</code> | <code>return Err(refusal(403, "original_activity_authority_mismatch", None));</code> |
| <code>interface.8.err-authority-original-activity-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:183</code> | <code>let refused = &#124;&#124; refusal(403, "original_activity_invalid", None);</code> |
| <code>interface.8.err-authority-original-activity-receipt-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:475</code> | <code>return Err(refusal(403, "original_activity_receipt_mismatch", None));</code> |
| <code>interface.8.err-authority-principal-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:521</code> | <code>return Err(refusal(403, "principal_mismatch", None));</code> |
| <code>interface.8.err-authority-principal-not-provisioned</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:349</code> | <code>.ok_or_else(&#124;&#124; refusal(404, "principal_not_provisioned", None))</code> |
| <code>interface.8.err-authority-query-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human.rs:465</code> | <code>.ok_or_else(&#124;&#124; refusal(400, "query_required", None))?</code> |
| <code>interface.8.err-authority-receipt-network-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:929</code> | <code>return refusal(503, "receipt_network_mismatch", Some(5));</code> |
| <code>interface.8.err-authority-receipt-source-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:896</code> | <code>return refusal(503, "receipt_source_unavailable", Some(5));</code> |
| <code>interface.8.err-authority-replica-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:914</code> | <code>return refusal(503, "replica_evidence_unavailable", Some(1));</code> |
| <code>interface.8.err-authority-replica-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:918</code> | <code>return refusal(503, "replica_unavailable", Some(5));</code> |
| <code>interface.8.err-authority-sequencer-key-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:892</code> | <code>return refusal(502, "sequencer_key_mismatch", None);</code> |
| <code>interface.8.err-authority-state-persistence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:941</code> | <code>return refusal(503, "state_persistence_unavailable", Some(5));</code> |
| <code>interface.8.err-authority-subject-not-bound</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:70</code> | <code>let refused = &#124;&#124; refusal(403, "subject_not_bound", None);</code> |
| <code>interface.8.err-authority-unexpected-registration</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/dynamic.rs:263</code> | <code>return Err(refusal(400, "unexpected_registration", None));</code> |
| <code>interface.8.err-authority-unknown-activity</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:889</code> | <code>ReceiptSource::Unknown =&gt; return refusal(404, "unknown_activity", None),</code> |
| <code>interface.8.err-authority-withdrawal-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:934</code> | <code>return refusal(503, "withdrawal_evidence_unavailable", Some(5));</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-authority-activity-not-bound</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-budget-not-bound</code> | notes | HTTP 404 |
| <code>interface.8.err-authority-capability-not-bound</code> | notes | HTTP 403 |
| <code>interface.8.err-authority-child-not-bound</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-did-not-bound</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-duplicate-query</code> | notes | HTTP 400 |
| <code>interface.8.err-authority-evidence-refused</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-identity-required</code> | notes | HTTP 401; 28 callers total |
| <code>interface.8.err-authority-invalid-activity-id</code> | notes | HTTP 400 |
| <code>interface.8.err-authority-invalid-budget-id</code> | notes | HTTP 400 |
| <code>interface.8.err-authority-invalid-query</code> | notes | HTTP 400 |
| <code>interface.8.err-authority-invalid-recovery</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-invalid-request</code> | notes | HTTP 400; 76 callers total |
| <code>interface.8.err-authority-method-not-allowed</code> | notes | HTTP 405; 48 callers total |
| <code>interface.8.err-authority-not-found</code> | notes | HTTP 404; 134 callers total |
| <code>interface.8.err-authority-original-activity-authority-mismatch</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-original-activity-invalid</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-original-activity-receipt-mismatch</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-principal-mismatch</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-principal-not-provisioned</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-query-required</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-receipt-network-mismatch</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-receipt-source-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-authority-replica-evidence-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-authority-replica-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-authority-sequencer-key-mismatch</code> | notes | HTTP 502 |
| <code>interface.8.err-authority-state-persistence-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-subject-not-bound</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-unexpected-registration</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-authority-unknown-activity</code> | notes | HTTP 404 |
| <code>interface.8.err-authority-withdrawal-evidence-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |

## authority-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-authority-ai-storage-admission</code> | <code>file_format</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:12</code> | <code>pub const FORMAT_VERSION: u16 = 1;</code> |
| <code>interface.8.fmt-authority-lxgi1-identity-state</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:535</code> | <code>&#124;&#124; &amp;states[0].body()[..5] != b"LXGI1"</code> |
| <code>interface.8.fmt-authority-lxgs2-session-summary</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/human/session_membership.rs:296</code> | <code>let mut summary = b"LXGS2".to_vec();</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-authority-ai-storage-admission</code> | notes | storage admission record version |
| <code>interface.8.fmt-authority-ai-storage-admission</code> | overridden_by | override.J40.200 |
| <code>interface.8.fmt-authority-lxgi1-identity-state</code> | notes | identity state record prefix; 29 callers total |
| <code>interface.8.fmt-authority-lxgs2-session-summary</code> | notes | session membership summary prefix |

## authority-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-authority-any-internal-v1-activities</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1284</code> | <code>path.strip_prefix("/internal/v1/activities/")</code> |
| <code>interface.8.http-authority-any-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1252</code> | <code>if path == "/livez" {</code> |
| <code>interface.8.http-authority-any-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1255</code> | <code>if path == "/readyz" {</code> |
| <code>interface.8.http-authority-any-v1-agent</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1258</code> | <code>if path.starts_with("/v1/agent/") {</code> |
| <code>interface.8.http-authority-any-v1-authorized-batches-wait-by-activity</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/main.rs:1280</code> | <code>path.strip_prefix("/v1/authorized-batches/wait-by-activity/")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-authority-any-livez</code> | notes | 118 callers total |
| <code>interface.8.http-authority-any-readyz</code> | notes | 323 callers total |
| <code>interface.8.http-authority-any-v1-agent</code> | notes | 248 callers total |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.111.1</code> | req | 8111.1 |
| <code>acceptance.8.111.1</code> | scenario | Given a chain listener answering one signed receipt and one null, when the authority looks up both hashes, then the first is present with the same bytes and the second is absent |
| <code>acceptance.8.111.1</code> | test | services/platform/hosted/authority/tests/chain_receipts.rs::receipt_lookup_reads_the_sequencer_receipt (planned) |
| <code>acceptance.8.111.1</code> | qualification | UNRUN |
| <code>req.8111</code> | ac_1 | The authority admits a receipt for a transaction hash when pax_sequencerReceipt answers a receipt signed by a pinned key, and answers absent for a null answer. |
| <code>acceptance.8.111.2</code> | req | 8111.2 |
| <code>acceptance.8.111.2</code> | scenario | Given a chain listener answering a receipt signed by a fresh key, when the authority looks it up, then the answer is the key mismatch refusal and the authority store holds nothing for it |
| <code>acceptance.8.111.2</code> | test | services/platform/hosted/authority/tests/chain_receipts.rs::receipt_signed_by_an_unpinned_key_is_refused (planned) |
| <code>acceptance.8.111.2</code> | qualification | UNRUN |
| <code>req.8111</code> | ac_2 | A receipt whose sequencer signature verifies only under a key absent from the pinned list is refused as key_mismatch and never stored. |
| <code>acceptance.8.112.1</code> | req | 8112.1 |
| <code>acceptance.8.112.1</code> | scenario | Given a key list of two keys and no kernel socket, when receipts signed by each pinned key and by a third key are admitted, then the first two are admitted and the third is refused |
| <code>acceptance.8.112.1</code> | test | services/platform/hosted/authority/tests/chain_receipts.rs::trust_snapshot_is_the_pinned_key_list (planned) |
| <code>acceptance.8.112.1</code> | qualification | UNRUN |
| <code>req.8112</code> | ac_1 | The authority accepts a receipt signed by any pinned key at any height and refuses a claimed key outside the list without opening a kernel socket. |
| <code>acceptance.8.226.1</code> | req | 8226.1 |
| <code>acceptance.8.226.1</code> | scenario | Given the authority tests directory, when it is listed and searched for the daemon fixture path, then neither is found |
| <code>acceptance.8.226.1</code> | test | services/platform/hosted/tests/test_retired_authority_node_tests.py::test_authority_has_no_kernel_node_test (planned) |
| <code>acceptance.8.226.1</code> | qualification | UNRUN |
| <code>req.8226</code> | ac_1 | authority/tests/real_node.rs is absent and no authority test names tests/daemon/fixtures. |
| <code>capability.8.11.1</code> | title | Look up receipts for the authority from pax_sequencerReceipt |
| <code>capability.8.11.1</code> | reqs | 8111.1<br>8111.2 |
| <code>capability.8.11.2</code> | title | Take the authority's sequencer trust from the pinned receipt keys |
| <code>capability.8.11.2</code> | reqs | 8112.1 |
| <code>capability.8.19.7</code> | title | Delete the kernel node test of the receipt authority |
| <code>capability.8.19.7</code> | reqs | 8226.1 |
| <code>service.platform-receipt-authority</code> | purpose | Admits and serves sequencer receipts for hosted clients after verifying them against pinned keys |
| <code>service.platform-receipt-authority</code> | route | receipt admission and lookup routes of layerx-receipt-authority, /livez, /readyz |
| <code>service.platform-receipt-authority</code> | inputs | LAYERX_AUTHORITY_CHAIN_RPC_URL, LAYERX_AUTHORITY_RECEIPT_KEYS_FILE |
| <code>service.platform-receipt-authority</code> | outputs | verified receipts and refusals |
| <code>service.platform-receipt-authority</code> | state | authority receipt store |
| <code>service.platform-receipt-authority</code> | dependencies | chain JSON-RPC pax_sequencerReceipt |
| <code>service.platform-receipt-authority</code> | failure_policy | an unreachable chain answers unavailable; an unpinned key is refused, never admitted |
| <code>service.platform-receipt-authority</code> | completion_task | 8.0.9 |
| <code>service.platform-receipt-authority</code> | operation_task | 8.11.3 |
| <code>service.platform-receipt-authority</code> | domain | 8 |

## authority-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-admissionerror</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:45</code> | <code>pub enum AdmissionError {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-cancel-request</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:28</code> | <code>pub const CANCEL_REQUEST: u16 = 56;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-cancel-response</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:29</code> | <code>pub const CANCEL_RESPONSE: u16 = 57;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-decode-reserve-answer</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:629</code> | <code>pub fn decode_reserve_answer(payload: &amp;[u8]) -&gt; Result&lt;(bool, Reservation), AdmissionError&gt; {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-demand</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:77</code> | <code>pub struct Demand {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-install-request</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:30</code> | <code>pub const INSTALL_REQUEST: u16 = 58;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-install-response</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:31</code> | <code>pub const INSTALL_RESPONSE: u16 = 59;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-kind</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:168</code> | <code>pub enum Kind {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observation</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:330</code> | <code>pub struct Observation {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observation-bytes</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:18</code> | <code>pub const OBSERVATION_BYTES: usize = 2 + 2 + 32 + 8 + 32 + 6 * DEMAND_BYTES + 4 + 8;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observe-body</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:612</code> | <code>pub fn observe_body() -&gt; Vec&lt;u8&gt; {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observe-request</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:22</code> | <code>pub const OBSERVE_REQUEST: u16 = 50;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observe-response</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:23</code> | <code>pub const OBSERVE_RESPONSE: u16 = 51;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-profile</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:111</code> | <code>pub struct Profile {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-profile-bytes</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:14</code> | <code>pub const PROFILE_BYTES: usize = 2 + 2 + 32 + DEMAND_BYTES + 8;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-proof-bytes</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:19</code> | <code>pub const PROOF_BYTES: usize = 32 + 64;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reconcile-request</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:26</code> | <code>pub const RECONCILE_REQUEST: u16 = 54;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reconcile-response</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:27</code> | <code>pub const RECONCILE_RESPONSE: u16 = 55;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-record-bytes</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:16</code> | <code>pub const RECORD_BYTES: usize =</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-request</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:225</code> | <code>pub struct Request {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-request-bytes</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:15</code> | <code>pub const REQUEST_BYTES: usize = 2 + 1 + 32 + 32 + 2 + 255 + DEMAND_BYTES + 8 + 32 + 8 + 8;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-request-digest</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:321</code> | <code>pub fn request_digest(body: &amp;[u8]) -&gt; [u8; 32] {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-request-id-body</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:618</code> | <code>pub fn request_id_body(request_id: u64) -&gt; Vec&lt;u8&gt; {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reservation</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:473</code> | <code>pub struct Reservation {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reserve-request</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:24</code> | <code>pub const RESERVE_REQUEST: u16 = 52;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reserve-response</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:25</code> | <code>pub const RESERVE_RESPONSE: u16 = 53;</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-verify-answer</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/platform/hosted/authority/src/ai_storage_admission.rs:583</code> | <code>pub fn verify_answer(</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-authorityfacts</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:172</code> | <code>pub struct AuthorityFacts {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-authorized-batch-by-activity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:389</code> | <code>pub fn authorized_batch_by_activity(</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-batchevidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:98</code> | <code>pub struct BatchEvidence {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-batchidentityevidence-batchmaintenancev1</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:110</code> | <code>pub enum BatchIdentityEvidence {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-evidencerefusal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:143</code> | <code>pub enum EvidenceRefusal {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:33</code> | <code>pub mod hex {</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-parse-replica-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:269</code> | <code>pub fn parse_replica_evidence(</code> |
| <code>interface.8.rs-authority-layerx-platform-authority-receipt-locator</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/authority/src/lib.rs:250</code> | <code>pub fn receipt_locator(receipt_bytes: &amp;[u8]) -&gt; Result&lt;ReceiptLocator, EvidenceRefusal&gt; {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-admissionerror</code> | overridden_by | override.J40.216 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-cancel-request</code> | overridden_by | override.J40.212 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-cancel-response</code> | overridden_by | override.J40.213 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-decode-reserve-answer</code> | overridden_by | override.J40.227 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-demand</code> | overridden_by | override.J40.217 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-install-request</code> | overridden_by | override.J40.214 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-install-response</code> | overridden_by | override.J40.215 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-kind</code> | overridden_by | override.J40.219 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observation</code> | overridden_by | override.J40.222 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observation-bytes</code> | overridden_by | override.J40.204 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observe-body</code> | overridden_by | override.J40.225 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observe-request</code> | overridden_by | override.J40.206 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-observe-response</code> | overridden_by | override.J40.207 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-profile</code> | overridden_by | override.J40.218 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-profile-bytes</code> | overridden_by | override.J40.201 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-proof-bytes</code> | overridden_by | override.J40.205 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reconcile-request</code> | overridden_by | override.J40.210 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reconcile-response</code> | overridden_by | override.J40.211 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-record-bytes</code> | overridden_by | override.J40.203 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-request</code> | overridden_by | override.J40.220 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-request-bytes</code> | overridden_by | override.J40.202 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-request-digest</code> | overridden_by | override.J40.221 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-request-id-body</code> | overridden_by | override.J40.226 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reservation</code> | overridden_by | override.J40.223 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reserve-request</code> | overridden_by | override.J40.208 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-reserve-response</code> | overridden_by | override.J40.209 |
| <code>interface.8.rs-authority-layerx-platform-authority-ai-storage-admission-verify-answer</code> | overridden_by | override.J40.224 |
| <code>interface.8.rs-authority-layerx-platform-authority-batchidentityevidence-batchmaintenancev1</code> | notes | BatchMaintenanceV1 is a variant or associated item of BatchIdentityEvidence |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.189.1</code> | req | 8189.1 |
| <code>acceptance.8.189.1</code> | scenario | Given the authority sources and every Rust file under services/platform, when they are listed and searched, then the module and its name are absent |
| <code>acceptance.8.189.1</code> | test | services/platform/hosted/tests/test_retired_storage_admission.py::test_authority_has_no_storage_admission (planned) |
| <code>acceptance.8.189.1</code> | qualification | UNRUN |
| <code>req.8189</code> | ac_1 | authority/src/ai_storage_admission.rs is absent and no Rust source under services/platform names ai_storage_admission. |
| <code>capability.8.17.2</code> | title | Delete the storage admission module of the authority |
| <code>capability.8.17.2</code> | reqs | 8189.1 |

## core-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-core-layerx-core-admin-listen-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:336</code> | <code>admin_listen: parse_listen("LAYERX_CORE_ADMIN_LISTEN", "0.0.0.0:9444")?,</code> |
| <code>interface.8.env-core-layerx-core-admin-tls-cert-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:343</code> | <code>"LAYERX_CORE_ADMIN_TLS_CERT_DER",</code> |
| <code>interface.8.env-core-layerx-core-admin-tls-key-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:344</code> | <code>"LAYERX_CORE_ADMIN_TLS_KEY_DER",</code> |
| <code>interface.8.env-core-layerx-core-admin-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:359</code> | <code>admin_token: read_secret("LAYERX_CORE_ADMIN_TOKEN_FILE")?,</code> |
| <code>interface.8.env-core-layerx-core-chain-rpc-url-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/core/src/chain_fund.rs</code> (planned) | <code>LAYERX_CORE_CHAIN_RPC_URL</code> |
| <code>interface.8.env-core-layerx-core-client-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:295</code> | <code>let client_ca = match env::var("LAYERX_CORE_CLIENT_CA_DER") {</code> |
| <code>interface.8.env-core-layerx-core-fee-limit-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:366</code> | <code>fee_limit: u128::from(parse_u64("LAYERX_CORE_FEE_LIMIT", 1_000)?),</code> |
| <code>interface.8.env-core-layerx-core-funding-sender-key-file-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/core/src/chain_fund.rs</code> (planned) | <code>LAYERX_CORE_FUNDING_SENDER_KEY_FILE</code> |
| <code>interface.8.env-core-layerx-core-funding-sender-kms-key-id-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/core/src/chain_fund.rs</code> (planned) | <code>LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID</code> |
| <code>interface.8.env-core-layerx-core-listen-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:335</code> | <code>listen: parse_listen("LAYERX_CORE_LISTEN", "0.0.0.0:9443")?,</code> |
| <code>interface.8.env-core-layerx-core-lni-socket-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:347</code> | <code>lni_socket: PathBuf::from(required("LAYERX_CORE_LNI_SOCKET")?),</code> |
| <code>interface.8.env-core-layerx-core-network-id-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:299</code> | <code>let network_id = required("LAYERX_CORE_NETWORK_ID")?</code> |
| <code>interface.8.env-core-layerx-core-node-bearer-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:350</code> | <code>node_token: read_secret("LAYERX_CORE_NODE_BEARER_TOKEN_FILE")?,</code> |
| <code>interface.8.env-core-layerx-core-node-url-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:349</code> | <code>node: parse_node_url(&amp;required("LAYERX_CORE_NODE_URL")?)?,</code> |
| <code>interface.8.env-core-layerx-core-receipt-deadline-ms-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:368</code> | <code>"LAYERX_CORE_RECEIPT_DEADLINE_MS",</code> |
| <code>interface.8.env-core-layerx-core-receipt-events-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:354</code> | <code>receipt_events_token: env::var_os("LAYERX_CORE_RECEIPT_EVENTS_TOKEN_FILE")</code> |
| <code>interface.8.env-core-layerx-core-replica-bearer-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:358</code> | <code>replica_token: read_secret("LAYERX_CORE_REPLICA_BEARER_TOKEN_FILE")?,</code> |
| <code>interface.8.env-core-layerx-core-replica-url-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:357</code> | <code>replica: parse_node_url(&amp;required("LAYERX_CORE_REPLICA_URL")?)?,</code> |
| <code>interface.8.env-core-layerx-core-state-dir-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:319</code> | <code>let state_dir = PathBuf::from(required("LAYERX_CORE_STATE_DIR")?);</code> |
| <code>interface.8.env-core-layerx-core-supervisor-socket-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:364</code> | <code>supervisor_socket: PathBuf::from(required("LAYERX_CORE_SUPERVISOR_SOCKET")?),</code> |
| <code>interface.8.env-core-layerx-core-tls-cert-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:338</code> | <code>"LAYERX_CORE_TLS_CERT_DER",</code> |
| <code>interface.8.env-core-layerx-core-tls-key-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:339</code> | <code>"LAYERX_CORE_TLS_KEY_DER",</code> |
| <code>interface.8.env-core-layerx-core-wallet-caps-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:351</code> | <code>wallet_caps_token: env::var_os("LAYERX_CORE_WALLET_CAPS_TOKEN_FILE")</code> |
| <code>interface.8.env-core-layerx-pay-timing</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:64</code> | <code>if std::env::var_os("LAYERX_PAY_TIMING").is_some() {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-core-layerx-core-admin-listen-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:336 |
| <code>interface.8.env-core-layerx-core-admin-tls-cert-der-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:343 |
| <code>interface.8.env-core-layerx-core-admin-tls-key-der-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:344 |
| <code>interface.8.env-core-layerx-core-admin-token-file-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:359 |
| <code>interface.8.env-core-layerx-core-chain-rpc-url-j40</code> | notes | planned chain JSON-RPC endpoint for treasury funding sends; task 8.10.1 |
| <code>interface.8.env-core-layerx-core-fee-limit-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:366 |
| <code>interface.8.env-core-layerx-core-funding-sender-key-file-j40</code> | notes | planned file holding the funding sender secp256k1 key read by EvmSender::from_env; task 8.10.1 |
| <code>interface.8.env-core-layerx-core-funding-sender-kms-key-id-j40</code> | notes | planned KMS key handle alternative to the key file for the funding sender; task 8.10.1 |
| <code>interface.8.env-core-layerx-core-listen-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:335 |
| <code>interface.8.env-core-layerx-core-lni-socket-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:347; named in the do text of task 8.18.1 |
| <code>interface.8.env-core-layerx-core-network-id-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:299 |
| <code>interface.8.env-core-layerx-core-node-bearer-token-file-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:350 |
| <code>interface.8.env-core-layerx-core-node-url-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:349; 13 in-repo references, first 8 listed |
| <code>interface.8.env-core-layerx-core-receipt-deadline-ms-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:368 |
| <code>interface.8.env-core-layerx-core-replica-bearer-token-file-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:358 |
| <code>interface.8.env-core-layerx-core-replica-url-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:357 |
| <code>interface.8.env-core-layerx-core-state-dir-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:319; 11 in-repo references, first 8 listed |
| <code>interface.8.env-core-layerx-core-supervisor-socket-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:364 |
| <code>interface.8.env-core-layerx-core-tls-cert-der-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:338 |
| <code>interface.8.env-core-layerx-core-tls-key-der-j69</code> | notes | read at services/platform/hosted/core/src/main.rs:339 |
| <code>interface.8.env-core-layerx-core-wallet-caps-token-file</code> | notes | no in-repo setter or reader outside the defining file |

Contract context <code>contract_context.8.hosted-chain-readers</code>:

| Field | Frozen rule |
|---|---|
| interface | Chain JSON-RPC clients of the core boundary funding path, the receipt authority and the program registry |
| storage | core funding journal; authority receipt store; registry journal |
| errors | treasury_unavailable 503, identity_binding_unavailable 409, idempotency_conflict 409, deployment_proof_refused 422, protocol_state_unavailable 503 |
| events | none |
| gas | one value transfer per funding command, paid by the funding sender |
| activation | wave 3 moves the readers, wave 6 removes the kernel sockets |
| invariants | a funding command signs at most one transaction; every admitted receipt verifies under a pinned key; every registry head is read at one block height |
| pause | none |

## core-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-core-account-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:28</code> | <code>return refusal(503, "account_evidence_unavailable", Some(5));</code> |
| <code>interface.8.err-core-activity-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1255</code> | <code>return refusal(415, "activity_content_type_required", None);</code> |
| <code>interface.8.err-core-admin-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2945</code> | <code>return refusal(503, "admin_unavailable", Some(5));</code> |
| <code>interface.8.err-core-asset-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:109</code> | <code>Err(_) =&gt; refusal(503, "asset_evidence_unavailable", Some(5)),</code> |
| <code>interface.8.err-core-asset-ordinal-reserved</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:822</code> | <code>Self::AssetOrdinalReserved =&gt; refusal(422, "asset_ordinal_reserved", None),</code> |
| <code>interface.8.err-core-authority-unsupported</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:898</code> | <code>.ok_or_else(&#124;&#124; refusal(400, "authority_unsupported", None))?;</code> |
| <code>interface.8.err-core-batch-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:431</code> | <code>Err(_) =&gt; refusal(503, "batch_evidence_unavailable", Some(5)),</code> |
| <code>interface.8.err-core-capability-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:916</code> | <code>SubmitError::UnavailableCapability =&gt; refusal(503, "capability_unavailable", Some(30)),</code> |
| <code>interface.8.err-core-caps-account-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:20</code> | <code>return refusal(403, "caps_account_refused", None);</code> |
| <code>interface.8.err-core-caps-evidence-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:44</code> | <code>CapsProgress::Refused(_) =&gt; return refusal(502, "caps_evidence_refused", None),</code> |
| <code>interface.8.err-core-caps-not-configured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:11</code> | <code>let Some(token) = &amp;config.wallet_caps_token else { return refusal(503, "caps_not_configured", Some(5)); };</code> |
| <code>interface.8.err-core-caps-response-bound</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:76</code> | <code>if value.to_string().len() &gt; 4 * 1024 * 1024 { return refusal(503, "caps_response_bound", None); }</code> |
| <code>interface.8.err-core-caps-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:22</code> | <code>let Ok((mut transport, handshake)) = connect(config) else { return refusal(503, "caps_unavailable", Some(5)); };</code> |
| <code>interface.8.err-core-checkpoint-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:449</code> | <code>Err(_) =&gt; refusal(503, "checkpoint_evidence_unavailable", Some(5)),</code> |
| <code>interface.8.err-core-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1214</code> | <code>_ =&gt; return refusal(400, "content_type_required", None),</code> |
| <code>interface.8.err-core-did-account-listing-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:148</code> | <code>return refusal(503, "did_account_listing_unavailable", Some(5));</code> |
| <code>interface.8.err-core-fee-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:278</code> | <code>Err(_) =&gt; refusal(503, "fee_evidence_unavailable", Some(5)),</code> |
| <code>interface.8.err-core-fee-execution-meter-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:266</code> | <code>return refusal(503, "fee_execution_meter_unavailable", Some(5));</code> |
| <code>interface.8.err-core-gateway-readiness-not-configured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1582</code> | <code>return refusal(503, "gateway_readiness_not_configured", None);</code> |
| <code>interface.8.err-core-idempotency-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1913</code> | <code>Ok(Some(_)) =&gt; return refusal(409, "idempotency_conflict", None),</code> |
| <code>interface.8.err-core-idempotency-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1275</code> | <code>return refusal(400, "idempotency_key_required", None);</code> |
| <code>interface.8.err-core-insufficient-treasury-balance</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2054</code> | <code>return Err(refusal(422, "insufficient_treasury_balance", Some(60)));</code> |
| <code>interface.8.err-core-invalid-account-evidence</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:34</code> | <code>return refusal(502, "invalid_account_evidence", None);</code> |
| <code>interface.8.err-core-invalid-account-id</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:18</code> | <code>let Ok(account_id) = fixed_hex::&lt;32&gt;("account_id", &amp;selection.account_id) else { return refusal(400, "invalid_account_id", None); };</code> |
| <code>interface.8.err-core-invalid-account-name</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:37</code> | <code>return refusal(502, "invalid_account_name", None);</code> |
| <code>interface.8.err-core-invalid-activity</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:878</code> | <code>.map_err(&#124;_&#124; refusal(400, "invalid_activity", None))?;</code> |
| <code>interface.8.err-core-invalid-activity-domain</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:244</code> | <code>return refusal(400, "invalid_activity_domain", None);</code> |
| <code>interface.8.err-core-invalid-argument</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1207</code> | <code>return refusal(400, "invalid_argument", None);</code> |
| <code>interface.8.err-core-invalid-asset-activity</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:823</code> | <code>Self::InvalidAssetActivity =&gt; refusal(400, "invalid_asset_activity", None),</code> |
| <code>interface.8.err-core-invalid-asset-id</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:294</code> | <code>refusal(400, "invalid_asset_id", None)</code> |
| <code>interface.8.err-core-invalid-asset-symbol</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:70</code> | <code>Self::InvalidSymbol =&gt; refusal(502, "invalid_asset_symbol", None),</code> |
| <code>interface.8.err-core-invalid-batch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:415</code> | <code>return refusal(400, "invalid_batch", None);</code> |
| <code>interface.8.err-core-invalid-caps-selection</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:15</code> | <code>if request.query.is_some() &#124;&#124; request.body.len() &gt; 1024 { return refusal(400, "invalid_caps_selection", None); }</code> |
| <code>interface.8.err-core-invalid-checkpoint</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:435</code> | <code>return refusal(400, "invalid_checkpoint", None);</code> |
| <code>interface.8.err-core-invalid-did</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:17</code> | <code>let Ok(did) = layerx_types::ids::Did::new(selection.did.as_bytes()) else { return refusal(400, "invalid_did", None); };</code> |
| <code>interface.8.err-core-invalid-expected-state-root</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1070</code> | <code>return Err(refusal(400, "invalid_expected_state_root", None));</code> |
| <code>interface.8.err-core-invalid-fee-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:224</code> | <code>return refusal(400, "invalid_fee_request", None);</code> |
| <code>interface.8.err-core-invalid-idempotency-key</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1329</code> | <code>return refusal(400, "invalid_idempotency_key", None);</code> |
| <code>interface.8.err-core-invalid-minimum-sequence</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1059</code> | <code>return Err(refusal(400, "invalid_minimum_sequence", None));</code> |
| <code>interface.8.err-core-invalid-program-account-operation</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:826</code> | <code>refusal(400, "invalid_program_account_operation", None)</code> |
| <code>interface.8.err-core-invalid-program-call</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:824</code> | <code>Self::InvalidProgramCall =&gt; refusal(400, "invalid_program_call", None),</code> |
| <code>interface.8.err-core-invalid-program-lifecycle</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:828</code> | <code>Self::InvalidProgramLifecycle =&gt; refusal(400, "invalid_program_lifecycle", None),</code> |
| <code>interface.8.err-core-invalid-proof-selector</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:474</code> | <code>return refusal(400, "invalid_proof_selector", None);</code> |
| <code>interface.8.err-core-invalid-receipt-event</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:568</code> | <code>_ =&gt; refusal(502, "invalid_receipt_event", None),</code> |
| <code>interface.8.err-core-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1594</code> | <code>return refusal(400, "invalid_request", None);</code> |
| <code>interface.8.err-core-invalid-sequence</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:548</code> | <code>return refusal(400, "invalid_sequence", None);</code> |
| <code>interface.8.err-core-journal-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1385</code> | <code>return refusal(503, "journal_unavailable", Some(5));</code> |
| <code>interface.8.err-core-method-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:14</code> | <code>if request.method != "POST" { return refusal(405, "method_not_allowed", None); }</code> |
| <code>interface.8.err-core-node-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:902</code> | <code>refusal(503, "node_unavailable", Some(5))</code> |
| <code>interface.8.err-core-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1309</code> | <code>Ok(None) =&gt; refusal(404, "not_found", None),</code> |
| <code>interface.8.err-core-not-program-call</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:965</code> | <code>return Err(refusal(400, "not_program_call", None));</code> |
| <code>interface.8.err-core-outcome-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1937</code> | <code>refusal(409, "outcome_unknown", Some(5))</code> |
| <code>interface.8.err-core-program-read-refused</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1164</code> | <code>refusal(422, "program_read_refused", None)</code> |
| <code>interface.8.err-core-program-route-mismatch</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:884</code> | <code>return Err(refusal(400, "program_route_mismatch", None));</code> |
| <code>interface.8.err-core-proof-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:506</code> | <code>return refusal(503, "proof_evidence_unavailable", Some(5));</code> |
| <code>interface.8.err-core-protocol-idempotency-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1284</code> | <code>return refusal(409, "protocol_idempotency_mismatch", None);</code> |
| <code>interface.8.err-core-receipt-events-not-configured</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:534</code> | <code>return refusal(503, "receipt_events_not_configured", None);</code> |
| <code>interface.8.err-core-receipt-events-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:572</code> | <code>Err(_) =&gt; refusal(503, "receipt_events_unavailable", Some(5)),</code> |
| <code>interface.8.err-core-receipt-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1339</code> | <code>Err(_) =&gt; return refusal(502, "receipt_invalid", None),</code> |
| <code>interface.8.err-core-receipt-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1334</code> | <code>Ok((404, _)) =&gt; return refusal(404, "receipt_not_found", None),</code> |
| <code>interface.8.err-core-receipt-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:949</code> | <code>Err(refusal(503, "receipt_unavailable", Some(5)))</code> |
| <code>interface.8.err-core-registry-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:876</code> | <code>submission_decode_registry().map_err(&#124;_&#124; refusal(503, "registry_unavailable", Some(5)))?;</code> |
| <code>interface.8.err-core-replica-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1382</code> | <code>return refusal(503, "replica_unavailable", Some(5));</code> |
| <code>interface.8.err-core-reset-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2855</code> | <code>refusal(503, "reset_failed", Some(30))</code> |
| <code>interface.8.err-core-send-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2698</code> | <code>return Err(refusal(422, "send_refused", None));</code> |
| <code>interface.8.err-core-send-unbuildable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2018</code> | <code>refusal(422, "send_unbuildable", None)</code> |
| <code>interface.8.err-core-sequence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:467</code> | <code>Err(_) =&gt; refusal(503, "sequence_unavailable", Some(5)),</code> |
| <code>interface.8.err-core-simulation-refused</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:987</code> | <code>refusal(422, "simulation_refused", None)</code> |
| <code>interface.8.err-core-snapshot-mismatch</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1155</code> | <code>ProgramReadError::SnapshotMismatch =&gt; refusal(409, "snapshot_mismatch", None),</code> |
| <code>interface.8.err-core-snapshot-stale</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1154</code> | <code>ProgramReadError::SnapshotStale =&gt; refusal(409, "snapshot_stale", Some(1)),</code> |
| <code>interface.8.err-core-submission-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:914</code> | <code>refusal(422, "submission_refused", None)</code> |
| <code>interface.8.err-core-supervisor-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2864</code> | <code>refusal(503, "supervisor_unavailable", Some(30))</code> |
| <code>interface.8.err-core-treasury-account-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2040</code> | <code>refusal(422, "treasury_account_unavailable", Some(60))</code> |
| <code>interface.8.err-core-treasury-identity-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2105</code> | <code>refusal(503, "treasury_identity_unavailable", Some(5))</code> |
| <code>interface.8.err-core-treasury-signer-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2014</code> | <code>refusal(503, "treasury_signer_unavailable", Some(5))</code> |
| <code>interface.8.err-core-treasury-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2025</code> | <code>.map_err(&#124;_&#124; refusal(503, "treasury_unavailable", Some(60)))?;</code> |
| <code>interface.8.err-core-unauthorized</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/core/src/caps_reads.rs:13</code> | <code>if !bool::from(supplied.as_bytes().ct_eq(token.as_bytes())) { return refusal(401, "unauthorized", None); }</code> |
| <code>interface.8.err-core-withdrawal-asset-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/withdrawal.rs:32</code> | <code>refusal(422, "withdrawal_asset_unavailable", None)</code> |
| <code>interface.8.err-core-withdrawal-configuration-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/platform/hosted/core/src/withdrawal.rs:10</code> | <code>.map_err(&#124;_&#124; refusal(503, "withdrawal_configuration_unavailable", Some(5)))?;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-core-account-evidence-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-activity-content-type-required</code> | notes | HTTP 415 |
| <code>interface.8.err-core-admin-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-asset-evidence-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-asset-ordinal-reserved</code> | notes | HTTP 422 |
| <code>interface.8.err-core-asset-ordinal-reserved</code> | overridden_by | override.J40.747 |
| <code>interface.8.err-core-authority-unsupported</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-authority-unsupported</code> | overridden_by | override.J40.753 |
| <code>interface.8.err-core-batch-evidence-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-capability-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-caps-account-refused</code> | notes | HTTP 403 |
| <code>interface.8.err-core-caps-evidence-refused</code> | notes | HTTP 502 |
| <code>interface.8.err-core-caps-not-configured</code> | notes | HTTP 503 |
| <code>interface.8.err-core-caps-response-bound</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-caps-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-checkpoint-evidence-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-content-type-required</code> | notes | HTTP 400 |
| <code>interface.8.err-core-did-account-listing-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-fee-evidence-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-fee-execution-meter-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-gateway-readiness-not-configured</code> | notes | HTTP 503 |
| <code>interface.8.err-core-idempotency-conflict</code> | notes | HTTP 409; 27 callers total |
| <code>interface.8.err-core-idempotency-key-required</code> | notes | HTTP 400 |
| <code>interface.8.err-core-insufficient-treasury-balance</code> | notes | HTTP 422 |
| <code>interface.8.err-core-invalid-account-evidence</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-invalid-account-id</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-account-name</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-invalid-activity</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-activity-domain</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-invalid-argument</code> | notes | HTTP 400; 65 callers total |
| <code>interface.8.err-core-invalid-asset-activity</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-asset-activity</code> | overridden_by | override.J40.748 |
| <code>interface.8.err-core-invalid-asset-id</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-invalid-asset-symbol</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-invalid-batch</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-caps-selection</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-invalid-checkpoint</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-did</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-expected-state-root</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-expected-state-root</code> | overridden_by | override.J40.757 |
| <code>interface.8.err-core-invalid-fee-request</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-invalid-idempotency-key</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-minimum-sequence</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-minimum-sequence</code> | overridden_by | override.J40.756 |
| <code>interface.8.err-core-invalid-program-account-operation</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-program-account-operation</code> | overridden_by | override.J40.750 |
| <code>interface.8.err-core-invalid-program-call</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-program-call</code> | overridden_by | override.J40.749 |
| <code>interface.8.err-core-invalid-program-lifecycle</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-program-lifecycle</code> | overridden_by | override.J40.751 |
| <code>interface.8.err-core-invalid-proof-selector</code> | notes | HTTP 400 |
| <code>interface.8.err-core-invalid-receipt-event</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-invalid-request</code> | notes | HTTP 400; 72 callers total |
| <code>interface.8.err-core-invalid-sequence</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-journal-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-method-not-allowed</code> | notes | HTTP 405; 48 callers total |
| <code>interface.8.err-core-node-unavailable</code> | notes | HTTP 503; 47 callers total |
| <code>interface.8.err-core-not-found</code> | notes | HTTP 404; 133 callers total |
| <code>interface.8.err-core-not-program-call</code> | notes | HTTP 400 |
| <code>interface.8.err-core-not-program-call</code> | overridden_by | override.J40.754 |
| <code>interface.8.err-core-outcome-unknown</code> | notes | HTTP 409 |
| <code>interface.8.err-core-program-read-refused</code> | notes | HTTP 422 |
| <code>interface.8.err-core-program-read-refused</code> | overridden_by | override.J40.760 |
| <code>interface.8.err-core-program-route-mismatch</code> | notes | HTTP 400 |
| <code>interface.8.err-core-program-route-mismatch</code> | overridden_by | override.J40.752 |
| <code>interface.8.err-core-proof-evidence-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-protocol-idempotency-mismatch</code> | notes | HTTP 409 |
| <code>interface.8.err-core-receipt-events-not-configured</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-receipt-events-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-receipt-invalid</code> | notes | HTTP 502 |
| <code>interface.8.err-core-receipt-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-core-receipt-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-registry-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-replica-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-reset-failed</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-send-refused</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-send-unbuildable</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-sequence-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-simulation-refused</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-simulation-refused</code> | overridden_by | override.J40.755 |
| <code>interface.8.err-core-snapshot-mismatch</code> | notes | HTTP 409 |
| <code>interface.8.err-core-snapshot-mismatch</code> | overridden_by | override.J40.759 |
| <code>interface.8.err-core-snapshot-stale</code> | notes | HTTP 409 |
| <code>interface.8.err-core-snapshot-stale</code> | overridden_by | override.J40.758 |
| <code>interface.8.err-core-submission-refused</code> | notes | HTTP 422 |
| <code>interface.8.err-core-supervisor-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-core-treasury-account-unavailable</code> | notes | HTTP 422 |
| <code>interface.8.err-core-treasury-identity-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-treasury-signer-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-treasury-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-unauthorized</code> | notes | HTTP 401; 96 callers total |
| <code>interface.8.err-core-withdrawal-asset-unavailable</code> | notes | HTTP 422 |
| <code>interface.8.err-core-withdrawal-asset-unavailable</code> | overridden_by | override.J40.762 |
| <code>interface.8.err-core-withdrawal-configuration-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-core-withdrawal-configuration-unavailable</code> | overridden_by | override.J40.761 |

## core-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-core-lxbc4-profile</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:542</code> | <code>if &amp;profile[..5] != b"LXBC4" &#124;&#124; custody_u64(&amp;profile[5..13]) != 125</code> |
| <code>interface.8.fmt-core-lxbr1-custody-registry</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:442</code> | <code>if marker != b"LXBR1" { return Err("custody registry marker is invalid".into()); }</code> |
| <code>interface.8.fmt-core-lxlt1-trust</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:478</code> | <code>if trust.len() != 89 &#124;&#124; &amp;trust[..5] != b"LXLT1" {</code> |
| <code>interface.8.fmt-core-lxpa1-program-account</code> | <code>file_format</code> | <code>retire</code> | <code>services/platform/hosted/core/src/program_accounts.rs:14</code> | <code>if payload.get(32..37) != Some(b"LXPA1") &#124;&#124; payload.get(37..69).ok_or(())? == [0; 32] {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-core-lxbc4-profile</code> | notes | custody profile record |
| <code>interface.8.fmt-core-lxbr1-custody-registry</code> | notes | custody registry marker |
| <code>interface.8.fmt-core-lxlt1-trust</code> | notes | 89-byte trust record |
| <code>interface.8.fmt-core-lxpa1-program-account</code> | notes | program account payload marker; 26 callers total |
| <code>interface.8.fmt-core-lxpa1-program-account</code> | overridden_by | override.J40.763 |

## core-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-core-any-internal-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1600</code> | <code>if request.path == "/internal/readyz" {</code> |
| <code>interface.8.http-core-any-internal-v1-receipt-events</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:331</code> | <code>if let Some(sequence) = request.path.strip_prefix("/internal/v1/receipt-events/") {</code> |
| <code>interface.8.http-core-any-internal-v1-wallet-caps</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:307</code> | <code>if request.path == "/internal/v1/wallet-caps" {</code> |
| <code>interface.8.http-core-any-v1-accounts</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1573</code> | <code>path == "/v1/accounts"</code> |
| <code>interface.8.http-core-any-v1-activities</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1707</code> | <code>&#124; "/v1/activities"</code> |
| <code>interface.8.http-core-any-v1-assets</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:287</code> | <code>} else if request.path != "/v1/assets"</code> |
| <code>interface.8.http-core-any-v1-assets-2</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:310</code> | <code>if request.path == "/v1/assets" &#124;&#124; request.path.starts_with("/v1/assets/") {</code> |
| <code>interface.8.http-core-any-v1-fees-estimate</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:313</code> | <code>if request.path == "/v1/fees/estimate" {</code> |
| <code>interface.8.http-core-any-v1-node-info</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/public_reads.rs:322</code> | <code>if request.path == "/v1/node-info" {</code> |
| <code>interface.8.http-core-any-v1-programs-activities</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1576</code> | <code>&#124;&#124; path.starts_with("/v1/programs/activities/")</code> |
| <code>interface.8.http-core-any-v1-programs-receipts-by-idempotency</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1577</code> | <code>&#124;&#124; path.starts_with("/v1/programs/receipts/by-idempotency/")</code> |
| <code>interface.8.http-core-any-v1-programs-registry</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1574</code> | <code>&#124;&#124; path == "/v1/programs/registry"</code> |
| <code>interface.8.http-core-any-v1-programs-registry-2</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1575</code> | <code>&#124;&#124; path.starts_with("/v1/programs/registry/")</code> |
| <code>interface.8.http-core-any-v1-programs-upgrade</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1651</code> | <code>&#124; "/v1/programs/upgrade"</code> |
| <code>interface.8.http-core-any-v1-programs-wind-down</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1652</code> | <code>&#124; "/v1/programs/wind-down",</code> |
| <code>interface.8.http-core-get-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1643</code> | <code>("GET", "/livez") =&gt; json_response(200, &amp;serde_json::json!({ "live": true })),</code> |
| <code>interface.8.http-core-get-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1644</code> | <code>("GET", "/readyz") =&gt; readiness(config),</code> |
| <code>interface.8.http-core-get-v1-sequencer</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1645</code> | <code>("GET", "/v1/sequencer") =&gt; sequencer_route(config),</code> |
| <code>interface.8.http-core-get-v1-state</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:1689</code> | <code>("GET", "/v1/state") =&gt; wrapped_relay_route(</code> |
| <code>interface.8.http-core-post-admin-v1-testnet-fund</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2929</code> | <code>"/admin/v1/testnet/fund" &#124; "/admin/v1/testnet/reset" =&gt; {</code> |
| <code>interface.8.http-core-post-admin-v1-testnet-reset</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/core/src/main.rs:2929</code> | <code>"/admin/v1/testnet/fund" &#124; "/admin/v1/testnet/reset" =&gt; {</code> |
| <code>interface.8.http-core-post-v1-programs-call</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1649</code> | <code>&#124; "/v1/programs/call"</code> |
| <code>interface.8.http-core-post-v1-programs-deploy</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1650</code> | <code>&#124; "/v1/programs/deploy"</code> |
| <code>interface.8.http-core-post-v1-programs-read</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1687</code> | <code>("POST", "/v1/programs/read") =&gt; program_read_route(config, request),</code> |
| <code>interface.8.http-core-post-v1-programs-simulate</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/core/src/main.rs:1688</code> | <code>("POST", "/v1/programs/simulate") =&gt; simulate_route(config, request),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-core-any-v1-activities</code> | notes | 103 callers total |
| <code>interface.8.http-core-any-v1-activities</code> | overridden_by | override.J40.746 |
| <code>interface.8.http-core-any-v1-programs-activities</code> | notes | 58 callers total |
| <code>interface.8.http-core-any-v1-programs-receipts-by-idempotency</code> | notes | 43 callers total |
| <code>interface.8.http-core-any-v1-programs-registry-2</code> | notes | 100 callers total |
| <code>interface.8.http-core-any-v1-programs-upgrade</code> | notes | 52 callers total |
| <code>interface.8.http-core-any-v1-programs-upgrade</code> | overridden_by | override.J40.742 |
| <code>interface.8.http-core-any-v1-programs-wind-down</code> | notes | 57 callers total |
| <code>interface.8.http-core-any-v1-programs-wind-down</code> | overridden_by | override.J40.743 |
| <code>interface.8.http-core-get-livez</code> | notes | 117 callers total |
| <code>interface.8.http-core-get-readyz</code> | notes | 321 callers total |
| <code>interface.8.http-core-get-v1-state</code> | notes | 36 callers total |
| <code>interface.8.http-core-post-admin-v1-testnet-fund</code> | notes | 37 callers total |
| <code>interface.8.http-core-post-v1-programs-call</code> | notes | 81 callers total |
| <code>interface.8.http-core-post-v1-programs-call</code> | overridden_by | override.J40.740 |
| <code>interface.8.http-core-post-v1-programs-deploy</code> | notes | 62 callers total |
| <code>interface.8.http-core-post-v1-programs-deploy</code> | overridden_by | override.J40.741 |
| <code>interface.8.http-core-post-v1-programs-read</code> | overridden_by | override.J40.744 |
| <code>interface.8.http-core-post-v1-programs-simulate</code> | notes | 47 callers total |
| <code>interface.8.http-core-post-v1-programs-simulate</code> | overridden_by | override.J40.745 |

Contract context <code>contract_context.8.kernel-harness-retirement</code>:

| Field | Frozen rule |
|---|---|
| interface | Hosted node, guarantor, custody provisioning and core activity surfaces retired with the kernel |
| storage | none |
| errors | retired routes answer 404 not_found |
| events | none |
| gas | none |
| activation | movement intervals, handover verify, relay archive and verifier dependency in wave 5; scripts, harnesses and routes in wave 6 |
| invariants | no hosted file names a deleted kernel binary, script or settlement document; writes reach the chain only as EIP-1559 transactions |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.109.1</code> | req | 8109.1 |
| <code>acceptance.8.109.1</code> | scenario | Given a chain listener binding the DID key to one address and answering a successful receipt, when the admin plane posts a fund command, then the listener saw one eth_sendRawTransaction whose recovered sender is the funding key, recipient the bound address and value the amount |
| <code>acceptance.8.109.1</code> | test | services/platform/hosted/core/tests/chain_fund.rs::testnet_fund_sends_one_bank_transfer_to_the_bound_address (planned) |
| <code>acceptance.8.109.1</code> | qualification | UNRUN |
| <code>req.8109</code> | ac_1 | POST /admin/v1/testnet/fund sends exactly one signed EIP-1559 value transfer of the requested amount to the address getEvmAddrByLayerX returns for the DID's key and answers its transaction hash. |
| <code>acceptance.8.109.2</code> | req | 8109.2 |
| <code>acceptance.8.109.2</code> | scenario | Given a fund command whose first send timed out after journaling, when it is retried with the same key, then the listener receives the identical raw transaction and the answer carries the same hash |
| <code>acceptance.8.109.2</code> | test | services/platform/hosted/core/tests/chain_fund.rs::testnet_fund_replays_the_journaled_transaction (planned) |
| <code>acceptance.8.109.2</code> | qualification | UNRUN |
| <code>req.8109</code> | ac_2 | A retried fund command with the same idempotency key resends the journaled transaction bytes and never signs a second transfer. |
| <code>acceptance.8.110.1</code> | req | 8110.1 |
| <code>acceptance.8.110.1</code> | scenario | Given a core boundary with a funding sender and a chain listener that answers and then closes, when /readyz is read twice, then the first funding member is ready with chain id and sender and the second names chain_unavailable |
| <code>acceptance.8.110.1</code> | test | services/platform/hosted/core/tests/chain_fund.rs::readyz_reports_the_funding_chain_and_sender (planned) |
| <code>acceptance.8.110.1</code> | qualification | UNRUN |
| <code>req.8110</code> | ac_1 | GET /readyz of the core boundary carries funding.ready true with the chain id and sender address when the chain answers and funding.ready false with chain_unavailable when it does not. |
| <code>acceptance.8.228.1</code> | req | 8228.1 |
| <code>acceptance.8.228.1</code> | scenario | Given a started core boundary, when each retired route is posted and the sequencer route is read, then every retired route answers 404 not_found and the sequencer route answers 200 |
| <code>acceptance.8.228.1</code> | test | services/platform/hosted/core/tests/chain_fund.rs::retired_activity_and_program_routes_answer_not_found (planned) |
| <code>acceptance.8.228.1</code> | qualification | UNRUN |
| <code>req.8228</code> | ac_1 | POST /v1/activities, /v1/programs/call, /v1/programs/deploy, /v1/programs/upgrade, /v1/programs/wind-down, /v1/programs/read and /v1/programs/simulate each answer 404 not_found, and GET /v1/sequencer still answers. |
| <code>acceptance.8.229.1</code> | req | 8229.1 |
| <code>acceptance.8.229.1</code> | scenario | Given the core tests directory, when the five files are checked and the rest are searched for the two retired names, then nothing is found |
| <code>acceptance.8.229.1</code> | test | services/platform/hosted/tests/test_retired_core_kernel_tests.py::test_core_has_no_kernel_daemon_test (planned) |
| <code>acceptance.8.229.1</code> | qualification | UNRUN |
| <code>req.8229</code> | ac_1 | boundary.rs, custody_asset_send.rs, core_chain.py, qualify.py and receipt_retention.py are absent from the core tests and no remaining core test names layerxd or checkpoint-settlement.json. |
| <code>capability.8.10.1</code> | title | Send testnet funding as one EVM bank transfer from the core treasury |
| <code>capability.8.10.1</code> | reqs | 8109.1<br>8109.2 |
| <code>capability.8.10.2</code> | title | Report the funding chain and sender in core readiness |
| <code>capability.8.10.2</code> | reqs | 8110.1 |
| <code>capability.8.20.1</code> | title | Retire the activity and program routes of the core boundary |
| <code>capability.8.20.1</code> | reqs | 8228.1 |
| <code>capability.8.20.2</code> | title | Delete the kernel daemon tests of the core boundary |
| <code>capability.8.20.2</code> | reqs | 8229.1 |
| <code>resolution.17.805</code> | title | decision.810 binds: Testnet funding pays the EVM address bound to the funded DID |
| <code>resolution.17.805</code> | applies_to | decision.810<br>task.8.10.1<br>task.8.10.2 |
| <code>resolution.17.805</code> | source | decision.810 in fragments/J40/decision.kvx; tasks from affects |
| <code>resolution.17.806</code> | title | decision.811 binds: The core boundary keeps its kernel routes in this card |
| <code>resolution.17.806</code> | applies_to | decision.811<br>task.8.10.1<br>task.8.10.2<br>task.8.20.1 |
| <code>resolution.17.806</code> | source | decision.811 in fragments/J40/decision.kvx; tasks from affects |
| <code>service.platform-core-boundary</code> | purpose | Hosted core boundary: sequencer and state reads, receipts and the admin testnet fund and reset plane |
| <code>service.platform-core-boundary</code> | route | /livez, /readyz, GET /v1/sequencer, GET /v1/state, GET /v1/receipts/{id}, POST /admin/v1/testnet/fund, POST /admin/v1/testnet/reset |
| <code>service.platform-core-boundary</code> | inputs | LAYERX_CORE_CHAIN_RPC_URL, LAYERX_CORE_FUNDING_SENDER_KEY_FILE, LAYERX_CORE_CLIENT_CA_DER, admin bearer tokens |
| <code>service.platform-core-boundary</code> | outputs | sequencer, state and receipt answers; funding transaction hashes |
| <code>service.platform-core-boundary</code> | state | journal directory of fund and reset transitions |
| <code>service.platform-core-boundary</code> | dependencies | paxeer chain JSON-RPC, local node relay |
| <code>service.platform-core-boundary</code> | failure_policy | an unreachable chain or sender answers treasury_unavailable 503 with retry; a retried fund replays the journaled transaction |
| <code>service.platform-core-boundary</code> | completion_task | 8.0.9 |
| <code>service.platform-core-boundary</code> | operation_task | 8.20.1 |
| <code>service.platform-core-boundary</code> | domain | 8 |

## core-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-core-layerx-platform-core-asset-registry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:75</code> | <code>pub fn asset_registry() -&gt; Result&lt;(ModuleRegistry, ActivityType), String&gt; {</code> |
| <code>interface.8.rs-core-layerx-platform-core-build-asset-send-with-identity-sequence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:578</code> | <code>pub fn build_asset_send_with_identity_sequence(</code> |
| <code>interface.8.rs-core-layerx-platform-core-build-send</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:112</code> | <code>pub fn build_send(seed: &amp;[u8; 32], request: &amp;SendRequest) -&gt; Result&lt;SignedSend, String&gt; {</code> |
| <code>interface.8.rs-core-layerx-platform-core-domain-hash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:99</code> | <code>pub fn domain_hash(domain: Domain, bytes: &amp;[u8]) -&gt; [u8; 32] {</code> |
| <code>interface.8.rs-core-layerx-platform-core-hex-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:342</code> | <code>pub fn hex_decode(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, String&gt; {</code> |
| <code>interface.8.rs-core-layerx-platform-core-hex-encode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:327</code> | <code>pub fn hex_encode(bytes: &amp;[u8]) -&gt; String {</code> |
| <code>interface.8.rs-core-layerx-platform-core-main-account</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:60</code> | <code>pub fn main_account(did: &amp;str) -&gt; Result&lt;[u8; 32], String&gt; {</code> |
| <code>interface.8.rs-core-layerx-platform-core-read-custody-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:468</code> | <code>pub fn read_custody_asset(</code> |
| <code>interface.8.rs-core-layerx-platform-core-read-custody-profile</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:428</code> | <code>pub fn read_custody_profile(</code> |
| <code>interface.8.rs-core-layerx-platform-core-send-context-hash</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:87</code> | <code>pub fn send_context_hash(</code> |
| <code>interface.8.rs-core-layerx-platform-core-sendrequest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:31</code> | <code>pub struct SendRequest {</code> |
| <code>interface.8.rs-core-layerx-platform-core-signedsend</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:46</code> | <code>pub struct SignedSend {</code> |
| <code>interface.8.rs-core-layerx-platform-core-treasury-did</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/core/src/lib.rs:377</code> | <code>pub fn treasury_did(seed: &amp;[u8; 32]) -&gt; String {</code> |

## dashboard-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-dashboard-layerx-dashboard-gateway-redis-password-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/gateway.rs:73</code> | <code>password: read_secret("LAYERX_DASHBOARD_GATEWAY_REDIS_PASSWORD_FILE")?,</code> |
| <code>interface.8.env-dashboard-layerx-dashboard-gateway-redis-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/gateway.rs:58</code> | <code>&amp;env::var("LAYERX_DASHBOARD_GATEWAY_REDIS_URL")</code> |
| <code>interface.8.env-dashboard-layerx-dashboard-gateway-redis-username-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/gateway.rs:72</code> | <code>username: read_secret("LAYERX_DASHBOARD_GATEWAY_REDIS_USERNAME_FILE")?,</code> |
| <code>interface.8.env-dashboard-layerx-dashboard-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:88</code> | <code>listen: env::var("LAYERX_DASHBOARD_LISTEN")</code> |
| <code>interface.8.env-dashboard-layerx-dashboard-listener</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:49</code> | <code>match env::var("LAYERX_DASHBOARD_LISTENER") {</code> |
| <code>interface.8.env-dashboard-layerx-dashboard-redis-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/gateway.rs:63</code> | <code>env::var("LAYERX_DASHBOARD_REDIS_CA_DER")</code> |
| <code>interface.8.env-dashboard-layerx-dashboard-tls-cert-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:67</code> | <code>env::var("LAYERX_DASHBOARD_TLS_CERT_DER")</code> |
| <code>interface.8.env-dashboard-layerx-dashboard-tls-key-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:74</code> | <code>env::var("LAYERX_DASHBOARD_TLS_KEY_DER")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-dashboard-layerx-dashboard-gateway-redis-password-file-j69</code> | notes | read at services/platform/hosted/dashboard/src/gateway.rs:73 |
| <code>interface.8.env-dashboard-layerx-dashboard-gateway-redis-username-file-j69</code> | notes | read at services/platform/hosted/dashboard/src/gateway.rs:72 |

## dashboard-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-dashboard-encoding-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:112</code> | <code>&#124;_&#124; Reply::refusal(503, "encoding_failed", Some(5)),</code> |
| <code>interface.8.err-dashboard-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:100</code> | <code>DashboardError::InvalidRequest =&gt; Reply::refusal(400, "invalid_request", None),</code> |
| <code>interface.8.err-dashboard-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:137</code> | <code>return Reply::refusal(404, "not_found", None);</code> |
| <code>interface.8.err-dashboard-principal-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:102</code> | <code>DashboardError::Gateway(_) =&gt; Reply::refusal(403, "principal_refused", None),</code> |
| <code>interface.8.err-dashboard-receipt-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:101</code> | <code>DashboardError::UnknownReceipt =&gt; Reply::refusal(404, "receipt_not_found", None),</code> |
| <code>interface.8.err-dashboard-session-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:202</code> | <code>Err(_) =&gt; Reply::refusal(401, "session_required", None),</code> |
| <code>interface.8.err-dashboard-state-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:106</code> | <code>&#124; DashboardError::Io(_) =&gt; Reply::refusal(503, "state_unavailable", Some(5)),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-dashboard-encoding-failed</code> | notes | HTTP 503 |
| <code>interface.8.err-dashboard-invalid-request</code> | notes | HTTP 400; 77 callers total |
| <code>interface.8.err-dashboard-not-found</code> | notes | HTTP 404; 135 callers total |
| <code>interface.8.err-dashboard-principal-refused</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-dashboard-receipt-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-dashboard-session-required</code> | notes | HTTP 401 |
| <code>interface.8.err-dashboard-state-unavailable</code> | notes | HTTP 503 |

## dashboard-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-dashboard-get-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/dashboard/src/main.rs:193</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/healthz" {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-dashboard-get-healthz</code> | notes | 331 callers total |

## env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-docs-layerx-amount-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/first-payment-typescript/index.mjs:25</code> | <code>{ amount: required("LAYERX_AMOUNT"), currency: required("LAYERX_CURRENCY") },</code> |
| <code>interface.8.env-docs-layerx-api-url-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/first-payment-typescript/index.mjs:20</code> | <code>const layerx = openLayerX(required("LAYERX_API_URL"), required("LAYERX_API_TOKEN"));</code> |
| <code>interface.8.env-docs-layerx-batch-id-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/verify-receipt-rust/src/main.rs:49</code> | <code>batch_id: fixed_hex("LAYERX_BATCH_ID")?,</code> |
| <code>interface.8.env-docs-layerx-destination-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/first-payment-typescript/index.mjs:24</code> | <code>required("LAYERX_DESTINATION"),</code> |
| <code>interface.8.env-docs-layerx-fulfillment-dir-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/paid-endpoint-fastapi/app.py:95</code> | <code>fulfillment_directory = Path(environ.get("LAYERX_FULFILLMENT_DIR", "./fulfillments"))</code> |
| <code>interface.8.env-docs-layerx-payment-key-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/first-payment-typescript/index.mjs:26</code> | <code>required("LAYERX_PAYMENT_KEY"),</code> |
| <code>interface.8.env-docs-layerx-previous-state-root-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/verify-receipt-rust/src/main.rs:51</code> | <code>previous_state_root: fixed_hex("LAYERX_PREVIOUS_STATE_ROOT")?,</code> |
| <code>interface.8.env-docs-layerx-receipt-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/verify-receipt-rust/src/main.rs:46</code> | <code>let path = required("LAYERX_RECEIPT_FILE")?;</code> |
| <code>interface.8.env-docs-layerx-resource-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/paid-endpoint-fastapi/app.py:94</code> | <code>report_body = Path(environ.get("LAYERX_RESOURCE_FILE", "./resource.json")).read_bytes()</code> |
| <code>interface.8.env-docs-layerx-resulting-state-root-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/verify-receipt-rust/src/main.rs:52</code> | <code>resulting_state_root: fixed_hex("LAYERX_RESULTING_STATE_ROOT")?,</code> |
| <code>interface.8.env-docs-layerx-sequencer-public-key-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/verify-receipt-rust/src/main.rs:53</code> | <code>sequencer_public_key: fixed_hex("LAYERX_SEQUENCER_PUBLIC_KEY")?,</code> |
| <code>interface.8.env-docs-layerx-source-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/docs/samples/first-payment-typescript/index.mjs:23</code> | <code>required("LAYERX_SOURCE"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-docs-layerx-amount-j69</code> | notes | read at services/platform/docs/samples/first-payment-typescript/index.mjs:25; 36 in-repo references, first 8 listed |
| <code>interface.8.env-docs-layerx-api-url-j69</code> | notes | read at services/platform/docs/samples/first-payment-typescript/index.mjs:20; 35 in-repo references, first 8 listed |
| <code>interface.8.env-docs-layerx-batch-id-j69</code> | notes | read at services/platform/docs/samples/verify-receipt-rust/src/main.rs:49 |
| <code>interface.8.env-docs-layerx-destination-j69</code> | notes | read at services/platform/docs/samples/first-payment-typescript/index.mjs:24; 36 in-repo references, first 8 listed |
| <code>interface.8.env-docs-layerx-fulfillment-dir-j69</code> | notes | read at services/platform/docs/samples/paid-endpoint-fastapi/app.py:95 |
| <code>interface.8.env-docs-layerx-payment-key-j69</code> | notes | read at services/platform/docs/samples/first-payment-typescript/index.mjs:26; 36 in-repo references, first 8 listed |
| <code>interface.8.env-docs-layerx-previous-state-root-j69</code> | notes | read at services/platform/docs/samples/verify-receipt-rust/src/main.rs:51 |
| <code>interface.8.env-docs-layerx-receipt-file-j69</code> | notes | read at services/platform/docs/samples/verify-receipt-rust/src/main.rs:46 |
| <code>interface.8.env-docs-layerx-resource-file-j69</code> | notes | read at services/platform/docs/samples/paid-endpoint-fastapi/app.py:94 |
| <code>interface.8.env-docs-layerx-resulting-state-root-j69</code> | notes | read at services/platform/docs/samples/verify-receipt-rust/src/main.rs:52 |
| <code>interface.8.env-docs-layerx-sequencer-public-key-j69</code> | notes | read at services/platform/docs/samples/verify-receipt-rust/src/main.rs:53; 11 in-repo references, first 8 listed |
| <code>interface.8.env-docs-layerx-source-j69</code> | notes | read at services/platform/docs/samples/first-payment-typescript/index.mjs:23; 49 in-repo references, first 8 listed |

## faucet-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-faucet-layerx-faucet-address-limit-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:278</code> | <code>let address_limit = parse_u64("LAYERX_FAUCET_ADDRESS_LIMIT", 10_000_000)?;</code> |
| <code>interface.8.env-faucet-layerx-faucet-claim-amount-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:276</code> | <code>let amount = parse_u64("LAYERX_FAUCET_CLAIM_AMOUNT", 1_000_000)?;</code> |
| <code>interface.8.env-faucet-layerx-faucet-idempotency-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:275</code> | <code>let idempotency_seconds = parse_u64("LAYERX_FAUCET_IDEMPOTENCY_SECONDS", 604_800)?;</code> |
| <code>interface.8.env-faucet-layerx-faucet-identity-limit-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:277</code> | <code>let identity_limit = parse_u64("LAYERX_FAUCET_IDENTITY_LIMIT", 10_000_000)?;</code> |
| <code>interface.8.env-faucet-layerx-faucet-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:287</code> | <code>let listen = env::var("LAYERX_FAUCET_LISTEN")</code> |
| <code>interface.8.env-faucet-layerx-faucet-network-limit-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:279</code> | <code>let network_limit = parse_u64("LAYERX_FAUCET_NETWORK_LIMIT", 50_000_000)?;</code> |
| <code>interface.8.env-faucet-layerx-faucet-network-request-limit-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:325</code> | <code>network_request_limit: parse_u64("LAYERX_FAUCET_NETWORK_REQUEST_LIMIT", 60)?,</code> |
| <code>interface.8.env-faucet-layerx-faucet-network-request-window-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:327</code> | <code>"LAYERX_FAUCET_NETWORK_REQUEST_WINDOW_SECONDS",</code> |
| <code>interface.8.env-faucet-layerx-faucet-redis-password-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:321</code> | <code>redis_password: read_secret("LAYERX_FAUCET_REDIS_PASSWORD_FILE")?,</code> |
| <code>interface.8.env-faucet-layerx-faucet-redis-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:316</code> | <code>&amp;env::var("LAYERX_FAUCET_REDIS_URL")</code> |
| <code>interface.8.env-faucet-layerx-faucet-redis-username-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:320</code> | <code>redis_username: read_secret("LAYERX_FAUCET_REDIS_USERNAME_FILE")?,</code> |
| <code>interface.8.env-faucet-layerx-faucet-service-claim-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:308</code> | <code>service_claim_token: read_optional_secret("LAYERX_FAUCET_SERVICE_CLAIM_TOKEN_FILE")?,</code> |
| <code>interface.8.env-faucet-layerx-faucet-tls-cert-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:259</code> | <code>env::var("LAYERX_FAUCET_TLS_CERT_DER").map_err(&#124;_&#124; "TLS certificate is required")?;</code> |
| <code>interface.8.env-faucet-layerx-faucet-tls-key-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:261</code> | <code>env::var("LAYERX_FAUCET_TLS_KEY_DER").map_err(&#124;_&#124; "TLS private key is required")?;</code> |
| <code>interface.8.env-faucet-layerx-faucet-window-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:274</code> | <code>let window_seconds = parse_u64("LAYERX_FAUCET_WINDOW_SECONDS", 86_400)?;</code> |
| <code>interface.8.env-faucet-layerx-identity-introspection-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:303</code> | <code>&amp;env::var("LAYERX_IDENTITY_INTROSPECTION_URL")</code> |
| <code>interface.8.env-faucet-layerx-identity-service-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:307</code> | <code>identity_service_token: read_secret("LAYERX_IDENTITY_SERVICE_TOKEN_FILE")?,</code> |
| <code>interface.8.env-faucet-layerx-outbound-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:296</code> | <code>env::var("LAYERX_OUTBOUND_CA_DER")</code> |
| <code>interface.8.env-faucet-layerx-testnet-admin-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:314</code> | <code>funding_admin_token: read_secret("LAYERX_TESTNET_ADMIN_TOKEN_FILE")?,</code> |
| <code>interface.8.env-faucet-layerx-testnet-funding-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:310</code> | <code>&amp;env::var("LAYERX_TESTNET_FUNDING_URL")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-faucet-layerx-faucet-address-limit-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:278 |
| <code>interface.8.env-faucet-layerx-faucet-claim-amount-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:276 |
| <code>interface.8.env-faucet-layerx-faucet-idempotency-seconds-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:275 |
| <code>interface.8.env-faucet-layerx-faucet-identity-limit-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:277 |
| <code>interface.8.env-faucet-layerx-faucet-network-limit-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:279 |
| <code>interface.8.env-faucet-layerx-faucet-network-request-limit-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:325 |
| <code>interface.8.env-faucet-layerx-faucet-network-request-window-seconds-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:327 |
| <code>interface.8.env-faucet-layerx-faucet-redis-password-file-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:321 |
| <code>interface.8.env-faucet-layerx-faucet-redis-username-file-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:320 |
| <code>interface.8.env-faucet-layerx-faucet-service-claim-token-file-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:308 |
| <code>interface.8.env-faucet-layerx-faucet-window-seconds-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:274 |
| <code>interface.8.env-faucet-layerx-identity-service-token-file-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:307 |
| <code>interface.8.env-faucet-layerx-testnet-admin-token-file-j69</code> | notes | read at services/platform/hosted/faucet/src/main.rs:314 |

## faucet-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-faucet-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:950</code> | <code>return Err(refusal(400, "content_type_required", None));</code> |
| <code>interface.8.err-faucet-dependency-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1044</code> | <code>_ =&gt; refusal(503, "dependency_unavailable", Some(10)),</code> |
| <code>interface.8.err-faucet-funding-rejected</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1107</code> | <code>refusal(503, "funding_rejected", Some(10))</code> |
| <code>interface.8.err-faucet-idempotency-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1115</code> | <code>Reservation::Conflict =&gt; refusal(409, "idempotency_conflict", None),</code> |
| <code>interface.8.err-faucet-idempotency-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:972</code> | <code>return Err(refusal(400, "idempotency_key_required", None));</code> |
| <code>interface.8.err-faucet-identity-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:501</code> | <code>return Err(refusal(401, "identity_required", None));</code> |
| <code>interface.8.err-faucet-identity-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:508</code> | <code>.map_err(&#124;_&#124; refusal(503, "identity_unavailable", Some(10)))?,</code> |
| <code>interface.8.err-faucet-invalid-argument</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1020</code> | <code>return refusal(400, "invalid_argument", None);</code> |
| <code>interface.8.err-faucet-invalid-idempotency-key</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:975</code> | <code>return Err(refusal(400, "invalid_idempotency_key", None));</code> |
| <code>interface.8.err-faucet-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1192</code> | <code>&#124;_&#124; refusal(400, "invalid_request", None),</code> |
| <code>interface.8.err-faucet-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1006</code> | <code>return refusal(404, "not_found", None);</code> |
| <code>interface.8.err-faucet-persistence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:969</code> | <code>Err(_) =&gt; return Err(refusal(503, "persistence_unavailable", Some(10))),</code> |
| <code>interface.8.err-faucet-service-token-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1017</code> | <code>return refusal(401, "service_token_required", None);</code> |
| <code>interface.8.err-faucet-untrusted-identity-header</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:958</code> | <code>return Err(refusal(400, "untrusted_identity_header", None));</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-faucet-content-type-required</code> | notes | HTTP 400 |
| <code>interface.8.err-faucet-dependency-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-faucet-funding-rejected</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-faucet-idempotency-conflict</code> | notes | HTTP 409; 30 callers total |
| <code>interface.8.err-faucet-idempotency-key-required</code> | notes | HTTP 400 |
| <code>interface.8.err-faucet-identity-required</code> | notes | HTTP 401; 26 callers total |
| <code>interface.8.err-faucet-identity-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-faucet-invalid-argument</code> | notes | HTTP 400; 77 callers total |
| <code>interface.8.err-faucet-invalid-idempotency-key</code> | notes | HTTP 400 |
| <code>interface.8.err-faucet-invalid-request</code> | notes | HTTP 400; 78 callers total |
| <code>interface.8.err-faucet-not-found</code> | notes | HTTP 404; 135 callers total |
| <code>interface.8.err-faucet-persistence-unavailable</code> | notes | HTTP 503; 67 callers total |
| <code>interface.8.err-faucet-service-token-required</code> | notes | HTTP 401 |
| <code>interface.8.err-faucet-untrusted-identity-header</code> | notes | HTTP 400 |

## faucet-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-faucet-any</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:48</code> | <code>let path = if path == "/" {</code> |
| <code>interface.8.http-faucet-get-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1036</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/livez" {</code> |
| <code>interface.8.http-faucet-get-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1039</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/readyz" {</code> |
| <code>interface.8.http-faucet-post-v1-faucet-claims</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1050</code> | <code>if request.method != "POST" &#124;&#124; request.path != "/v1/faucet/claims" {</code> |
| <code>interface.8.http-faucet-post-v1-faucet-service-claims</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/faucet/src/main.rs:1047</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/v1/faucet/service-claims" {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-faucet-any</code> | notes | path is too generic to attribute callers |
| <code>interface.8.http-faucet-get-livez</code> | notes | 119 callers total |
| <code>interface.8.http-faucet-get-readyz</code> | notes | 324 callers total |

## feeder-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-feeder-main-rs-check-config</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:226</code> | <code>[flag] if flag == "--check-config" =&gt; true,</code> |
| <code>interface.8.cli-feeder-main-rs-version</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:227</code> | <code>[flag] if flag == "--version" =&gt; {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-feeder-main-rs-check-config</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-feeder-main-rs-version</code> | notes | no in-repo invocation outside the defining file |

## feeder-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-feeder-chainflowconfig-api-key-env</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/chainflow.rs:28</code> | <code>pub api_key_env: String,</code> |
| <code>interface.8.cfg-feeder-chainflowconfig-base-url</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/chainflow.rs:26</code> | <code>pub base_url: String,</code> |
| <code>interface.8.cfg-feeder-chainflowconfig-max-quote-age-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/chainflow.rs:30</code> | <code>pub max_quote_age_ms: u64,</code> |
| <code>interface.8.cfg-feeder-chainflowconfig-timeout-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/chainflow.rs:29</code> | <code>pub timeout_ms: u64,</code> |
| <code>interface.8.cfg-feeder-config-cadence-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:46</code> | <code>pub cadence_ms: u64,</code> |
| <code>interface.8.cfg-feeder-config-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:39</code> | <code>pub chain_id: String,</code> |
| <code>interface.8.cfg-feeder-config-chainflow</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:49</code> | <code>pub chainflow: ChainflowConfig,</code> |
| <code>interface.8.cfg-feeder-config-feed-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:43</code> | <code>pub feed_account: String,</code> |
| <code>interface.8.cfg-feeder-config-gas-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:44</code> | <code>pub gas_limit: u64,</code> |
| <code>interface.8.cfg-feeder-config-markets</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:50</code> | <code>pub markets: Vec&lt;MarketConfig&gt;,</code> |
| <code>interface.8.cfg-feeder-config-retry-backoff-ms</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:47</code> | <code>pub retry_backoff_ms: u64,</code> |
| <code>interface.8.cfg-feeder-config-retry-max-attempts</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:48</code> | <code>pub retry_max_attempts: u32,</code> |
| <code>interface.8.cfg-feeder-config-validator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:41</code> | <code>pub validator: String,</code> |
| <code>interface.8.cfg-feeder-marketconfig-market-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:31</code> | <code>pub market_id: u64,</code> |
| <code>interface.8.cfg-feeder-marketconfig-symbol</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:32</code> | <code>pub symbol: String,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-feeder-chainflowconfig-base-url</code> | notes | 62 callers total |
| <code>interface.8.cfg-feeder-config-chain-id</code> | notes | 1164 callers total |
| <code>interface.8.cfg-feeder-config-gas-limit</code> | notes | 217 callers total |
| <code>interface.8.cfg-feeder-config-markets</code> | notes | 126 callers total |
| <code>interface.8.cfg-feeder-config-validator</code> | notes | 217 callers total |
| <code>interface.8.cfg-feeder-marketconfig-market-id</code> | notes | 88 callers total |
| <code>interface.8.cfg-feeder-marketconfig-symbol</code> | notes | 1421 callers total |

## feeder-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-feeder-price-sidecar-config</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:187</code> | <code>let config = Config::load(&amp;PathBuf::from(env("PRICE_SIDECAR_CONFIG")?))?;</code> |
| <code>interface.8.env-feeder-price-sidecar-consensus-rpc-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:200</code> | <code>env("PRICE_SIDECAR_CONSENSUS_RPC_URL")?,</code> |
| <code>interface.8.env-feeder-price-sidecar-evm-rpc-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:201</code> | <code>env("PRICE_SIDECAR_EVM_RPC_URL")?,</code> |
| <code>interface.8.env-feeder-price-sidecar-feed-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:192</code> | <code>let secret = load_secret(&amp;PathBuf::from(env("PRICE_SIDECAR_FEED_KEY_FILE")?))?;</code> |
| <code>interface.8.env-feeder-price-sidecar-health-addr</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:195</code> | <code>let health = std::env::var("PRICE_SIDECAR_HEALTH_ADDR")</code> |
| <code>interface.8.env-feeder-price-sidecar-state-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/main.rs:194</code> | <code>let state_dir = PathBuf::from(env("PRICE_SIDECAR_STATE_DIR")?);</code> |

## feeder-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-feeder-journal-json</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/lib.rs:86</code> | <code>dir.join("journal.json")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-feeder-journal-json</code> | notes | feeder journal file; read and written only by the defining module |

## feeder-metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.met-feeder-price-sidecar-fetch-errors-total</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/health.rs:86</code> | <code>let _ = writeln!(out, "# TYPE price_sidecar_fetch_errors_total counter");</code> |
| <code>interface.8.met-feeder-price-sidecar-last-accepted-height</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/health.rs:101</code> | <code>let _ = writeln!(out, "# TYPE price_sidecar_last_accepted_height gauge");</code> |
| <code>interface.8.met-feeder-price-sidecar-market-age-ms</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/health.rs:93</code> | <code>let _ = writeln!(out, "# TYPE price_sidecar_market_age_ms gauge");</code> |
| <code>interface.8.met-feeder-price-sidecar-rounds-total</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/health.rs:77</code> | <code>let _ = writeln!(out, "# TYPE price_sidecar_rounds_total counter");</code> |
| <code>interface.8.met-feeder-price-sidecar-submissions-total</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/bin/price-sidecar/health.rs:79</code> | <code>let _ = writeln!(out, "# TYPE price_sidecar_submissions_total counter");</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.met-feeder-price-sidecar-last-accepted-height</code> | notes | no in-repo scraper or dashboard cites it |
| <code>interface.8.met-feeder-price-sidecar-rounds-total</code> | notes | no in-repo scraper or dashboard cites it |
| <code>interface.8.met-feeder-price-sidecar-submissions-total</code> | notes | no in-repo scraper or dashboard cites it |

## feeder-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-feeder-layerx-oracle-feeder-http-agent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/lib.rs:132</code> | <code>pub fn http_agent() -&gt; Result&lt;ureq::Agent, String&gt; {</code> |
| <code>interface.8.rs-feeder-layerx-oracle-feeder-sequencer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/feeder/src/lib.rs:193</code> | <code>pub struct Sequencer {</code> |

## human-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-human-browser-onboarding-py-resume</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/browser_onboarding.py:218</code> | <code>parser.add_argument('--resume', action='store_true')</code> |
| <code>interface.8.cli-human-guardians-py-epoch</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/guardians.py:138</code> | <code>enrollment.add_argument('--epoch', type=int, default=1)</code> |
| <code>interface.8.cli-human-guardians-py-identity</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/guardians.py:137</code> | <code>enrollment.add_argument('--identity', required=True)</code> |
| <code>interface.8.cli-human-guardians-py-output-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/guardians.py:146</code> | <code>assembly.add_argument('--output-dir')</code> |
| <code>interface.8.cli-human-guardians-py-role</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/guardians.py:136</code> | <code>enrollment.add_argument('--role', required=True)</code> |
| <code>interface.8.cli-human-guardians-py-secrets-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/guardians.py:135</code> | <code>enrollment.add_argument('--secrets-dir', required=True)</code> |
| <code>interface.8.cli-human-guardians-py-work-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/guardians.py:134</code> | <code>enrollment.add_argument('--work-dir', required=True)</code> |
| <code>interface.8.cli-human-material-py-assemble</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1613</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--assemble':</code> |
| <code>interface.8.cli-human-material-py-export-registry-material</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1593</code> | <code>if len(sys.argv) == 3 and sys.argv[1] == '--export-registry-material':</code> |
| <code>interface.8.cli-human-material-py-genesis-binding</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1623</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--genesis-binding':</code> |
| <code>interface.8.cli-human-material-py-hosted-registry-material-produce</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1597</code> | <code>elif len(sys.argv) == 4 and sys.argv[1] == '--hosted-registry-material-produce':</code> |
| <code>interface.8.cli-human-material-py-import-registry-material</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1595</code> | <code>elif len(sys.argv) == 7 and sys.argv[1] == '--import-registry-material':</code> |
| <code>interface.8.cli-human-material-py-import-secret</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1625</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--import-secret':</code> |
| <code>interface.8.cli-human-material-py-kernel-registry-material-produce</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1609</code> | <code>elif len(sys.argv) in (6, 7) and sys.argv[1] == '--kernel-registry-material-produce':</code> |
| <code>interface.8.cli-human-material-py-kms-prerequisite</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1611</code> | <code>elif len(sys.argv) == 8 and sys.argv[1] == '--kms-prerequisite':</code> |
| <code>interface.8.cli-human-material-py-policy-graph-status</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1599</code> | <code>elif len(sys.argv) == 11 and sys.argv[1] == '--policy-graph-status':</code> |
| <code>interface.8.cli-human-material-py-publish-authority-material</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1601</code> | <code>elif len(sys.argv) in (7, 8) and sys.argv[1] == '--publish-authority-material':</code> |
| <code>interface.8.cli-human-material-py-relocate-bundle</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1617</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--relocate-bundle':</code> |
| <code>interface.8.cli-human-material-py-seal-material</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1619</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--seal-material':</code> |
| <code>interface.8.cli-human-material-py-secret-arguments</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1629</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--secret-arguments':</code> |
| <code>interface.8.cli-human-material-py-verify-authority-material</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1604</code> | <code>elif len(sys.argv) == 3 and sys.argv[1] == '--verify-authority-material':</code> |
| <code>interface.8.cli-human-material-py-verify-bundle</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1615</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--verify-bundle':</code> |
| <code>interface.8.cli-human-material-py-verify-material</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1621</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--verify-material':</code> |
| <code>interface.8.cli-human-material-py-verify-projected-material</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1627</code> | <code>elif len(sys.argv) &gt; 1 and sys.argv[1] == '--verify-projected-material':</code> |
| <code>interface.8.cli-human-material-py-verify-registry-material</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/material.py:1606</code> | <code>elif len(sys.argv) == 3 and sys.argv[1] == '--verify-registry-material':</code> |
| <code>interface.8.cli-human-onboarding-bootstrap-py-native-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/onboarding_bootstrap.py:21</code> | <code>parser.add_argument('--native-asset', required=True)</code> |
| <code>interface.8.cli-human-onboarding-bootstrap-py-tenant</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/onboarding_bootstrap.py:20</code> | <code>parser.add_argument('--tenant', required=True)</code> |
| <code>interface.8.cli-human-owner-custody-py-amount</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:146</code> | <code>parser.add_argument('--amount', type=int, default=1000000000000000000)</code> |
| <code>interface.8.cli-human-owner-custody-py-asset</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:145</code> | <code>parser.add_argument('--asset', required=True)</code> |
| <code>interface.8.cli-human-owner-custody-py-ca-bundle</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:137</code> | <code>parser.add_argument('--ca-bundle')</code> |
| <code>interface.8.cli-human-owner-custody-py-comet-rpc</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:140</code> | <code>parser.add_argument('--comet-rpc', required=True)</code> |
| <code>interface.8.cli-human-owner-custody-py-disposable-identity</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:138</code> | <code>parser.add_argument('--disposable-identity')</code> |
| <code>interface.8.cli-human-owner-custody-py-key-file</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:143</code> | <code>parser.add_argument('--key-file', required=True)</code> |
| <code>interface.8.cli-human-owner-custody-py-network-id</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:144</code> | <code>parser.add_argument('--network-id', type=int, required=True)</code> |
| <code>interface.8.cli-human-owner-custody-py-rpc</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:136</code> | <code>parser.add_argument('--rpc', action='append', required=True)</code> |
| <code>interface.8.cli-human-owner-custody-py-trusted-height</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:141</code> | <code>parser.add_argument('--trusted-height', type=int)</code> |
| <code>interface.8.cli-human-owner-custody-py-trusting-period-seconds</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:142</code> | <code>parser.add_argument('--trusting-period-seconds', type=int)</code> |
| <code>interface.8.cli-human-owner-custody-py-vault-artifact</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:139</code> | <code>parser.add_argument('--vault-artifact')</code> |
| <code>interface.8.cli-human-owner-custody-py-work-dir</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/human/owner_custody.py:135</code> | <code>parser.add_argument('--work-dir', required=True)</code> |
| <code>interface.8.cli-human-owner-kms-py-config-directory</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/owner_kms.py:103</code> | <code>parser.add_argument('--config-directory', required=True)</code> |
| <code>interface.8.cli-human-owner-kms-py-executable</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/owner_kms.py:102</code> | <code>parser.add_argument('--executable', required=True)</code> |
| <code>interface.8.cli-human-owner-kms-py-work-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/owner_kms.py:101</code> | <code>parser.add_argument('--work-dir', required=True)</code> |
| <code>interface.8.cli-human-provision-py-account-requests</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1074</code> | <code>mode.add_argument('--account-requests', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-assemble</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1070</code> | <code>mode.add_argument('--assemble', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1084</code> | <code>parser.add_argument('--asset')</code> |
| <code>interface.8.cli-human-provision-py-catalog</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1069</code> | <code>mode.add_argument('--catalog', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-chain</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1088</code> | <code>parser.add_argument('--chain', type=int)</code> |
| <code>interface.8.cli-human-provision-py-journal</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1085</code> | <code>parser.add_argument('--journal', type=Path)</code> |
| <code>interface.8.cli-human-provision-py-materialize-journal</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1065</code> | <code>mode.add_argument('--materialize-journal', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-movement-source</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1072</code> | <code>mode.add_argument('--movement-source', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-network</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1087</code> | <code>parser.add_argument('--network', type=int)</code> |
| <code>interface.8.cli-human-provision-py-output</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1091</code> | <code>parser.add_argument('--output', type=Path)</code> |
| <code>interface.8.cli-human-provision-py-prepare-explorer-read-funding</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1078</code> | <code>mode.add_argument('--prepare-explorer-read-funding', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-prepare-owner-admission</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1068</code> | <code>mode.add_argument('--prepare-owner-admission', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-prepare-owner-request</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1063</code> | <code>mode.add_argument('--prepare-owner-request', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-preserve-binding</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1077</code> | <code>mode.add_argument('--preserve-binding', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-produce-owner-registration</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1067</code> | <code>mode.add_argument('--produce-owner-registration', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-producer-manifest</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1071</code> | <code>mode.add_argument('--producer-manifest', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-qualify-generated-set</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1073</code> | <code>mode.add_argument('--qualify-generated-set', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-registry</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1081</code> | <code>parser.add_argument('--registry', type=Path)</code> |
| <code>interface.8.cli-human-provision-py-request</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1089</code> | <code>parser.add_argument('--request', type=Path)</code> |
| <code>interface.8.cli-human-provision-py-response</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1090</code> | <code>parser.add_argument('--response', type=Path)</code> |
| <code>interface.8.cli-human-provision-py-secrets-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1086</code> | <code>parser.add_argument('--secrets-dir', type=Path)</code> |
| <code>interface.8.cli-human-provision-py-sequencer</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1083</code> | <code>parser.add_argument('--sequencer', type=Path)</code> |
| <code>interface.8.cli-human-provision-py-sign-explorer-read-credit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1079</code> | <code>mode.add_argument('--sign-explorer-read-credit', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-submit-explorer-read-credit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1080</code> | <code>mode.add_argument('--submit-explorer-read-credit', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-treasury</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1082</code> | <code>parser.add_argument('--treasury', type=Path)</code> |
| <code>interface.8.cli-human-provision-py-validate-evidence-inputs</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1064</code> | <code>mode.add_argument('--validate-evidence-inputs', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-validate-job-input</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1075</code> | <code>mode.add_argument('--validate-job-input', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-validate-owner-registration</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1066</code> | <code>mode.add_argument('--validate-owner-registration', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-validate-owner-result</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1076</code> | <code>mode.add_argument('--validate-owner-result', action='store_true')</code> |
| <code>interface.8.cli-human-provision-py-work-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.py:1092</code> | <code>parser.add_argument('--work-dir', type=Path, required=True)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-human-browser-onboarding-py-resume</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-guardians-py-epoch</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-guardians-py-identity</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-guardians-py-output-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-guardians-py-role</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-guardians-py-secrets-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-guardians-py-work-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-material-py-import-registry-material</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-material-py-verify-authority-material</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-onboarding-bootstrap-py-native-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-onboarding-bootstrap-py-tenant</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-amount</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-amount</code> | overridden_by | override.J40.731 |
| <code>interface.8.cli-human-owner-custody-py-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-asset</code> | overridden_by | override.J40.730 |
| <code>interface.8.cli-human-owner-custody-py-ca-bundle</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-ca-bundle</code> | overridden_by | override.J40.722 |
| <code>interface.8.cli-human-owner-custody-py-comet-rpc</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-comet-rpc</code> | overridden_by | override.J40.725 |
| <code>interface.8.cli-human-owner-custody-py-disposable-identity</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-disposable-identity</code> | overridden_by | override.J40.723 |
| <code>interface.8.cli-human-owner-custody-py-key-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-key-file</code> | overridden_by | override.J40.728 |
| <code>interface.8.cli-human-owner-custody-py-network-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-network-id</code> | overridden_by | override.J40.729 |
| <code>interface.8.cli-human-owner-custody-py-rpc</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-rpc</code> | overridden_by | override.J40.721 |
| <code>interface.8.cli-human-owner-custody-py-trusted-height</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-trusted-height</code> | overridden_by | override.J40.726 |
| <code>interface.8.cli-human-owner-custody-py-trusting-period-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-trusting-period-seconds</code> | overridden_by | override.J40.727 |
| <code>interface.8.cli-human-owner-custody-py-vault-artifact</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-vault-artifact</code> | overridden_by | override.J40.724 |
| <code>interface.8.cli-human-owner-custody-py-work-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-custody-py-work-dir</code> | overridden_by | override.J40.720 |
| <code>interface.8.cli-human-owner-kms-py-config-directory</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-kms-py-executable</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-owner-kms-py-work-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-assemble</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-catalog</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-chain</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-journal</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-movement-source</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-network</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-output</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-prepare-explorer-read-funding</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-prepare-owner-admission</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-preserve-binding</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-producer-manifest</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-qualify-generated-set</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-registry</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-request</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-response</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-secrets-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-sequencer</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-sign-explorer-read-credit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-treasury</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-human-provision-py-validate-evidence-inputs</code> | notes | no in-repo invocation outside the defining file |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.220.1</code> | req | 8220.1 |
| <code>acceptance.8.220.1</code> | scenario | Given the human provisioning sources, when provision.sh is searched, provision.py is run with --prepare-explorer-read-funding and the owner custody path is checked, then the steps are absent, the flag is refused and the file is missing |
| <code>acceptance.8.220.1</code> | test | services/platform/hosted/human/test_provision.py::test_provisioning_makes_no_custody_deposit (planned) |
| <code>acceptance.8.220.1</code> | qualification | UNRUN |
| <code>req.8220</code> | ac_1 | provision.sh defines no human_custody_step or explorer_read_principal_fund, provision.py accepts no --prepare-explorer-read-funding flag and owner_custody.py is absent. |
| <code>acceptance.8.221.1</code> | req | 8221.1 |
| <code>acceptance.8.221.1</code> | scenario | Given the shipped owner policy and a copy with a movement object added, when owner_policy reads each, then the first loads and the second is refused |
| <code>acceptance.8.221.1</code> | test | services/platform/hosted/human/test_provision.py::test_owner_policy_has_no_movement_intervals (planned) |
| <code>acceptance.8.221.1</code> | qualification | UNRUN |
| <code>req.8221</code> | ac_1 | owner_policy loads beta-owner-policy.json without a movement member and refuses a policy file that still carries one as an unknown field. |
| <code>acceptance.8.222.1</code> | req | 8222.1 |
| <code>acceptance.8.222.1</code> | scenario | Given material.py, when the source of hosted_registry_material_produce is inspected, then no layerx-handover invocation remains |
| <code>acceptance.8.222.1</code> | test | services/platform/hosted/human/test_material.py::test_registry_material_runs_no_handover_verify (planned) |
| <code>acceptance.8.222.1</code> | qualification | UNRUN |
| <code>req.8222</code> | ac_1 | hosted_registry_material_produce invokes no layerx-handover binary and material.py names no /usr/local/bin/layerx-handover. |
| <code>capability.8.19.10</code> | title | Drop the movement intervals from the human owner policy |
| <code>capability.8.19.10</code> | reqs | 8221.1 |
| <code>capability.8.19.4</code> | title | Remove the custody deposits from human provisioning |
| <code>capability.8.19.4</code> | reqs | 8220.1 |
| <code>capability.8.19.9</code> | title | Stop the hosted registry material from running the handover verifier |
| <code>capability.8.19.9</code> | reqs | 8222.1 |
| <code>resolution.17.815</code> | title | decision.820 binds: Human provisioning funds accounts by chain transfer, not custody deposit |
| <code>resolution.17.815</code> | applies_to | decision.820<br>task.8.19.4<br>task.8.19.10 |
| <code>resolution.17.815</code> | source | decision.820 in fragments/J40/decision.kvx; tasks from affects |

## human-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-human-human-web-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/material.sh:78</code> | <code>"$NODE_NETWORK_ID" "$PAXEER_CHAIN_ID" "$LAYERX_BETA_HUMAN_POLICY_FILE" "${HUMAN_WEB_URL:-}"</code> |
| <code>interface.8.env-human-layerx-authority-human-agent-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/bootstrap.py:19</code> | <code>human_env = {'LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE',</code> |
| <code>interface.8.env-human-layerx-beta-human-initial-funding</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/onboarding_provision.sh:8</code> | <code>local funding="${LAYERX_BETA_HUMAN_INITIAL_FUNDING:-1000000000000}" tenant</code> |
| <code>interface.8.env-human-layerx-beta-owner-custody</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.sh:217</code> | <code>case "${LAYERX_BETA_OWNER_CUSTODY:-kms}" in</code> |
| <code>interface.8.env-human-layerx-beta-retain-material</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/onboarding_provision.sh:75</code> | <code>if [ "${LAYERX_BETA_RETAIN_MATERIAL:-0}" = 1 ]; then</code> |
| <code>interface.8.env-human-layerx-human-onboarding-initial-funding</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/owner_kms.py:26</code> | <code>funding = int(os.environ['LAYERX_HUMAN_ONBOARDING_INITIAL_FUNDING'])</code> |
| <code>interface.8.env-human-paxeer-trusted-height</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.sh:328</code> | <code>--comet-rpc "$PAXEER_URL/comet" --trusted-height "${PAXEER_TRUSTED_HEIGHT:-1}" \</code> |
| <code>interface.8.env-human-paxeer-trusting-period-seconds</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/provision.sh:329</code> | <code>--trusting-period-seconds "${PAXEER_TRUSTING_PERIOD_SECONDS:?Paxeer trusting period is required}" \</code> |
| <code>interface.8.env-human-store-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/human/onboarding_material.py:71</code> | <code>env=dict(os.environ, LAYERX_HUMAN_STORE_ROOT=config['STORE_ROOT']))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-human-layerx-beta-human-initial-funding</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-human-layerx-beta-owner-custody</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-human-paxeer-trusted-height</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-human-paxeer-trusting-period-seconds</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-human-store-root</code> | notes | no in-repo setter or reader outside the defining file |

## human-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-human-onboarding-request-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/human/onboarding_socket.py:118</code> | <code>send(connection, dict(version=1, error='onboarding_request_refused'))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-human-onboarding-request-refused</code> | notes | no in-repo consumer quotes the code outside the defining file |

## identity-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-identity-layerx-identity-client-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:408</code> | <code>let config = match env::var("LAYERX_IDENTITY_CLIENT_CA_DER") {</code> |
| <code>interface.8.env-identity-layerx-identity-disposable-domains-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:484</code> | <code>let disposable_domains = match env::var("LAYERX_IDENTITY_DISPOSABLE_DOMAINS_FILE") {</code> |
| <code>interface.8.env-identity-layerx-identity-health-addr-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:441</code> | <code>let health = listen_address("LAYERX_IDENTITY_HEALTH_ADDR", "[::]:8080")?;</code> |
| <code>interface.8.env-identity-layerx-identity-listen-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:440</code> | <code>let listen = listen_address("LAYERX_IDENTITY_LISTEN", "[::]:9443")?;</code> |
| <code>interface.8.env-identity-layerx-identity-service-tokens-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:447</code> | <code>env::var("LAYERX_IDENTITY_SERVICE_TOKENS_DIR")</code> |
| <code>interface.8.env-identity-layerx-identity-session-ttl-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:450</code> | <code>let default_ttl_seconds = parse_u64("LAYERX_IDENTITY_SESSION_TTL_SECONDS", 86_400)?;</code> |
| <code>interface.8.env-identity-layerx-identity-signup-daily-cap-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:470</code> | <code>"LAYERX_IDENTITY_SIGNUP_DAILY_CAP",</code> |
| <code>interface.8.env-identity-layerx-identity-signup-outbox-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:492</code> | <code>let outbox_dir = env::var("LAYERX_IDENTITY_SIGNUP_OUTBOX_DIR")</code> |
| <code>interface.8.env-identity-layerx-identity-signup-per-email-per-hour-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:465</code> | <code>"LAYERX_IDENTITY_SIGNUP_PER_EMAIL_PER_HOUR",</code> |
| <code>interface.8.env-identity-layerx-identity-signup-per-ip-per-hour-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:460</code> | <code>"LAYERX_IDENTITY_SIGNUP_PER_IP_PER_HOUR",</code> |
| <code>interface.8.env-identity-layerx-identity-signup-token-ttl-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:475</code> | <code>"LAYERX_IDENTITY_SIGNUP_TOKEN_TTL_SECONDS",</code> |
| <code>interface.8.env-identity-layerx-identity-smtp-from</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:497</code> | <code>&amp;env::var("LAYERX_IDENTITY_SMTP_FROM").map_err(&#124;_&#124; {</code> |
| <code>interface.8.env-identity-layerx-identity-smtp-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:494</code> | <code>let smtp = match env::var("LAYERX_IDENTITY_SMTP_URL") {</code> |
| <code>interface.8.env-identity-layerx-identity-state-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:443</code> | <code>env::var("LAYERX_IDENTITY_STATE_DIR")</code> |
| <code>interface.8.env-identity-layerx-identity-store-key-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:456</code> | <code>let store_secret = read_secret("LAYERX_IDENTITY_STORE_KEY_FILE")?;</code> |
| <code>interface.8.env-identity-layerx-identity-tls-cert-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:400</code> | <code>env::var("LAYERX_IDENTITY_TLS_CERT_DER").map_err(&#124;_&#124; "TLS certificate is required")?;</code> |
| <code>interface.8.env-identity-layerx-identity-tls-key-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:402</code> | <code>env::var("LAYERX_IDENTITY_TLS_KEY_DER").map_err(&#124;_&#124; "TLS private key is required")?;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-identity-layerx-identity-disposable-domains-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-identity-layerx-identity-health-addr-j69</code> | notes | read at services/platform/hosted/identity/src/main.rs:441 |
| <code>interface.8.env-identity-layerx-identity-listen-j69</code> | notes | read at services/platform/hosted/identity/src/main.rs:440; 9 in-repo references, first 8 listed |
| <code>interface.8.env-identity-layerx-identity-session-ttl-seconds-j69</code> | notes | read at services/platform/hosted/identity/src/main.rs:450; 10 in-repo references, first 8 listed |
| <code>interface.8.env-identity-layerx-identity-signup-daily-cap-j69</code> | notes | read at services/platform/hosted/identity/src/main.rs:470 |
| <code>interface.8.env-identity-layerx-identity-signup-outbox-dir</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-identity-layerx-identity-signup-per-email-per-hour-j69</code> | notes | read at services/platform/hosted/identity/src/main.rs:465 |
| <code>interface.8.env-identity-layerx-identity-signup-per-ip-per-hour-j69</code> | notes | read at services/platform/hosted/identity/src/main.rs:460 |
| <code>interface.8.env-identity-layerx-identity-signup-token-ttl-seconds-j69</code> | notes | read at services/platform/hosted/identity/src/main.rs:475 |
| <code>interface.8.env-identity-layerx-identity-store-key-file-j69</code> | notes | read at services/platform/hosted/identity/src/main.rs:456; 12 in-repo references, first 8 listed |

## identity-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-identity-audience-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:731</code> | <code>(Service::Ramp, _) =&gt; return refusal(400, "audience_required", None),</code> |
| <code>interface.8.err-identity-clock-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:739</code> | <code>return refusal(503, "clock_unavailable", Some(5));</code> |
| <code>interface.8.err-identity-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:721</code> | <code>return refusal(400, "content_type_required", None);</code> |
| <code>interface.8.err-identity-email-domain-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:53</code> | <code>Self::DisposableDomain =&gt; (400, "email_domain_refused", None),</code> |
| <code>interface.8.err-identity-email-registered</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:57</code> | <code>Self::EmailRegistered =&gt; (409, "email_registered", None),</code> |
| <code>interface.8.err-identity-encoding-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:710</code> | <code>&#124;_&#124; refusal(503, "encoding_failed", Some(5)),</code> |
| <code>interface.8.err-identity-entropy-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:931</code> | <code>return refusal(503, "entropy_unavailable", Some(5));</code> |
| <code>interface.8.err-identity-invalid-argument</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:725</code> | <code>Err(_) =&gt; return refusal(400, "invalid_argument", None),</code> |
| <code>interface.8.err-identity-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1417</code> | <code>&#124;_&#124; refusal(400, "invalid_request", None),</code> |
| <code>interface.8.err-identity-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1284</code> | <code>return refusal(404, "not_found", None);</code> |
| <code>interface.8.err-identity-principal-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:940</code> | <code>return refusal(404, "principal_not_found", None);</code> |
| <code>interface.8.err-identity-publication-key-binding-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1244</code> | <code>Err(_) =&gt; refusal(409, "publication_key_binding_refused", None),</code> |
| <code>interface.8.err-identity-publication-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1194</code> | <code>.ok_or_else(&#124;&#124; refusal(400, "publication_key_required", None))?;</code> |
| <code>interface.8.err-identity-service-not-permitted</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:833</code> | <code>refusal(403, "service_not_permitted", None)</code> |
| <code>interface.8.err-identity-service-token-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:624</code> | <code>return Err(refusal(401, "service_token_required", None));</code> |
| <code>interface.8.err-identity-session-bound-reached</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:963</code> | <code>refusal(429, "session_bound_reached", Some(60))</code> |
| <code>interface.8.err-identity-session-history-capacity-reached</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:961</code> | <code>refusal(429, "session_history_capacity_reached", None)</code> |
| <code>interface.8.err-identity-session-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:981</code> | <code>return refusal(404, "session_not_found", None);</code> |
| <code>interface.8.err-identity-signup-daily-cap-reached</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:56</code> | <code>Self::DailyCapReached(after) =&gt; (429, "signup_daily_cap_reached", Some(after)),</code> |
| <code>interface.8.err-identity-signup-email-rate-limited</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:55</code> | <code>Self::EmailRateLimited(after) =&gt; (429, "signup_email_rate_limited", Some(after)),</code> |
| <code>interface.8.err-identity-signup-ip-rate-limited</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:54</code> | <code>Self::IpRateLimited(after) =&gt; (429, "signup_ip_rate_limited", Some(after)),</code> |
| <code>interface.8.err-identity-starter-credit-already-claimed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:62</code> | <code>Self::AlreadyCredited =&gt; (409, "starter_credit_already_claimed", None),</code> |
| <code>interface.8.err-identity-starter-credit-grant-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:63</code> | <code>Self::GrantMismatch =&gt; (409, "starter_credit_grant_mismatch", None),</code> |
| <code>interface.8.err-identity-store-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:652</code> | <code>.map_err(&#124;_&#124; refusal(503, "store_unavailable", Some(5)))?;</code> |
| <code>interface.8.err-identity-subject-tenant-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:884</code> | <code>refusal(409, "subject_tenant_conflict", None)</code> |
| <code>interface.8.err-identity-untrusted-identity-header</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1261</code> | <code>return refusal(400, "untrusted_identity_header", None);</code> |
| <code>interface.8.err-identity-verification-delivery-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1080</code> | <code>return refusal(503, "verification_delivery_unavailable", Some(30));</code> |
| <code>interface.8.err-identity-verification-expired</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:60</code> | <code>Self::TokenExpired =&gt; (410, "verification_expired", None),</code> |
| <code>interface.8.err-identity-verification-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1115</code> | <code>return refusal(404, "verification_not_found", None);</code> |
| <code>interface.8.err-identity-verification-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/signup.rs:61</code> | <code>Self::NotVerified =&gt; (403, "verification_required", None),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-identity-audience-required</code> | notes | HTTP 400 |
| <code>interface.8.err-identity-clock-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-identity-content-type-required</code> | notes | HTTP 400 |
| <code>interface.8.err-identity-email-domain-refused</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-email-registered</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-encoding-failed</code> | notes | HTTP 503 |
| <code>interface.8.err-identity-entropy-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-identity-invalid-argument</code> | notes | HTTP 400; 66 callers total |
| <code>interface.8.err-identity-invalid-request</code> | notes | HTTP 400; 77 callers total |
| <code>interface.8.err-identity-not-found</code> | notes | HTTP 404; 134 callers total |
| <code>interface.8.err-identity-principal-not-found</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-publication-key-binding-refused</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-publication-key-required</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-service-not-permitted</code> | notes | HTTP 403; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-service-token-required</code> | notes | HTTP 401 |
| <code>interface.8.err-identity-session-bound-reached</code> | notes | HTTP 429 |
| <code>interface.8.err-identity-session-history-capacity-reached</code> | notes | HTTP 429 |
| <code>interface.8.err-identity-session-not-found</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-signup-daily-cap-reached</code> | notes | HTTP 429; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-signup-email-rate-limited</code> | notes | HTTP 429; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-signup-ip-rate-limited</code> | notes | HTTP 429 |
| <code>interface.8.err-identity-starter-credit-already-claimed</code> | notes | HTTP 409 |
| <code>interface.8.err-identity-starter-credit-grant-mismatch</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-store-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-identity-subject-tenant-conflict</code> | notes | HTTP 409 |
| <code>interface.8.err-identity-untrusted-identity-header</code> | notes | HTTP 400 |
| <code>interface.8.err-identity-verification-delivery-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-verification-expired</code> | notes | HTTP 410; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-identity-verification-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-identity-verification-required</code> | notes | HTTP 403 |

## identity-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-identity-audit-log</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/store.rs:11</code> | <code>const AUDIT_FILE: &amp;str = "audit.log";</code> |
| <code>interface.8.fmt-identity-journal-log</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/store.rs:9</code> | <code>const JOURNAL_FILE: &amp;str = "journal.log";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-identity-audit-log</code> | notes | identity audit file; read and written only by the defining module |
| <code>interface.8.fmt-identity-journal-log</code> | notes | identity journal file |

## identity-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-identity-any-v1-signup</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1272</code> | <code>&#124; "/v1/signup"</code> |
| <code>interface.8.http-identity-any-v1-starter-credits-settle</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1275</code> | <code>&#124; "/v1/starter-credits/settle"</code> |
| <code>interface.8.http-identity-delete-v1-sessions</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1282</code> | <code>&#124;&#124; (request.method == "DELETE" &amp;&amp; request.path.starts_with("/v1/sessions/"));</code> |
| <code>interface.8.http-identity-get-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1435</code> | <code>Ok(request) if request.method == "GET" &amp;&amp; request.path == "/healthz" =&gt; readiness(shared),</code> |
| <code>interface.8.http-identity-get-internal-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1280</code> | <code>"/internal/v1/principal" &#124; "/internal/readyz"</code> |
| <code>interface.8.http-identity-get-internal-v1-principal</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1280</code> | <code>"/internal/v1/principal" &#124; "/internal/readyz"</code> |
| <code>interface.8.http-identity-get-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1249</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/livez" {</code> |
| <code>interface.8.http-identity-get-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1252</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/readyz" {</code> |
| <code>interface.8.http-identity-post-v1-introspect</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1268</code> | <code>&#124; "/v1/introspect"</code> |
| <code>interface.8.http-identity-post-v1-principals</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1269</code> | <code>&#124; "/v1/principals"</code> |
| <code>interface.8.http-identity-post-v1-publication-keys</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1307</code> | <code>("POST", "/v1/publication-keys") =&gt; {</code> |
| <code>interface.8.http-identity-post-v1-sessions</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1341</code> | <code>("POST", "/v1/sessions") =&gt; {</code> |
| <code>interface.8.http-identity-post-v1-sessions-introspect</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1329</code> | <code>("POST", "/v1/sessions/introspect" &#124; "/v1/introspect") =&gt; {</code> |
| <code>interface.8.http-identity-post-v1-signup-verify</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1316</code> | <code>&#124; "/v1/signup/verify"</code> |
| <code>interface.8.http-identity-post-v1-starter-credits</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/identity/src/main.rs:1317</code> | <code>&#124; "/v1/starter-credits"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-identity-delete-v1-sessions</code> | notes | 84 callers total |
| <code>interface.8.http-identity-get-healthz</code> | notes | 331 callers total |
| <code>interface.8.http-identity-get-livez</code> | notes | 116 callers total |
| <code>interface.8.http-identity-get-readyz</code> | notes | 322 callers total |
| <code>interface.8.http-identity-post-v1-sessions</code> | notes | 59 callers total |

## indexer-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-indexer-config-rs-cutover-height</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:136</code> | <code>"--cutover-height" =&gt; cutover = Some(parse(value()?)?),</code> |
| <code>interface.8.cli-indexer-config-rs-range-blocks</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:137</code> | <code>"--range-blocks" =&gt; range_blocks = parse(value()?)?,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-indexer-config-rs-range-blocks</code> | notes | no in-repo invocation outside the defining file |

## indexer-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-indexer-layerx-indexer-abi-dir-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:350</code> | <code>abi_dir: lookup("LAYERX_INDEXER_ABI_DIR").map(PathBuf::from),</code> |
| <code>interface.8.env-indexer-layerx-indexer-allow-remote-plaintext-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:262</code> | <code>let allow_remote = lookup("LAYERX_INDEXER_ALLOW_REMOTE_PLAINTEXT").as_deref() == Some("1");</code> |
| <code>interface.8.env-indexer-layerx-indexer-backfill-range-blocks-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:115</code> | <code>"LAYERX_INDEXER_BACKFILL_RANGE_BLOCKS",</code> |
| <code>interface.8.env-indexer-layerx-indexer-batch-interval-ms-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/config.rs:303</code> | <code>LAYERX_INDEXER_BATCH_INTERVAL_MS</code> |
| <code>interface.8.env-indexer-layerx-indexer-block-time-ms-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:327</code> | <code>policy: policy("LAYERX_INDEXER_BLOCK_TIME_MS", DEFAULT_BLOCK_TIME_MS)?,</code> |
| <code>interface.8.env-indexer-layerx-indexer-challenge-window-seconds-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/config.rs:278</code> | <code>LAYERX_INDEXER_CHALLENGE_WINDOW_SECONDS</code> |
| <code>interface.8.env-indexer-layerx-indexer-comet-attributes-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:309</code> | <code>let encoding = match lookup("LAYERX_INDEXER_COMET_ATTRIBUTES").as_deref() {</code> |
| <code>interface.8.env-indexer-layerx-indexer-comet-ca-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:318</code> | <code>let comet = endpoint("LAYERX_INDEXER_COMET_URL", "LAYERX_INDEXER_COMET_CA_DER")?;</code> |
| <code>interface.8.env-indexer-layerx-indexer-comet-url-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:318</code> | <code>let comet = endpoint("LAYERX_INDEXER_COMET_URL", "LAYERX_INDEXER_COMET_CA_DER")?;</code> |
| <code>interface.8.env-indexer-layerx-indexer-db-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:250</code> | <code>let database = lookup("LAYERX_INDEXER_DB")</code> |
| <code>interface.8.env-indexer-layerx-indexer-evm-ca-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:319</code> | <code>let paxeer = endpoint("LAYERX_INDEXER_EVM_URL", "LAYERX_INDEXER_EVM_CA_DER")?</code> |
| <code>interface.8.env-indexer-layerx-indexer-evm-chain-id-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:325</code> | <code>chain_id: optional_number(lookup, "LAYERX_INDEXER_EVM_CHAIN_ID")?,</code> |
| <code>interface.8.env-indexer-layerx-indexer-evm-url-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:319</code> | <code>let paxeer = endpoint("LAYERX_INDEXER_EVM_URL", "LAYERX_INDEXER_EVM_CA_DER")?</code> |
| <code>interface.8.env-indexer-layerx-indexer-explorer-ca-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:333</code> | <code>"LAYERX_INDEXER_EXPLORER_CA_DER",</code> |
| <code>interface.8.env-indexer-layerx-indexer-explorer-url-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:332</code> | <code>"LAYERX_INDEXER_EXPLORER_URL",</code> |
| <code>interface.8.env-indexer-layerx-indexer-finality-depth-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:281</code> | <code>let fixed_depth = optional_number(lookup, "LAYERX_INDEXER_FINALITY_DEPTH")?;</code> |
| <code>interface.8.env-indexer-layerx-indexer-listen-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:253</code> | <code>let listen = lookup("LAYERX_INDEXER_LISTEN")</code> |
| <code>interface.8.env-indexer-layerx-indexer-max-units-per-step-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:284</code> | <code>"LAYERX_INDEXER_MAX_UNITS_PER_STEP",</code> |
| <code>interface.8.env-indexer-layerx-indexer-paxscan-cert-sha256-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:151</code> | <code>lookup("LAYERX_INDEXER_PAXSCAN_CERT_SHA256"),</code> |
| <code>interface.8.env-indexer-layerx-indexer-paxscan-tls-unauthenticated-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:152</code> | <code>lookup("LAYERX_INDEXER_PAXSCAN_TLS_UNAUTHENTICATED").as_deref(),</code> |
| <code>interface.8.env-indexer-layerx-indexer-pointers-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:353</code> | <code>pointers: parse_pointers(&amp;lookup("LAYERX_INDEXER_POINTERS").unwrap_or_default())?,</code> |
| <code>interface.8.env-indexer-layerx-indexer-poll-ms-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:355</code> | <code>number(lookup, "LAYERX_INDEXER_POLL_MS", DEFAULT_POLL_MS)?.max(1),</code> |
| <code>interface.8.env-indexer-layerx-indexer-relay-ca-der-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/config.rs:297</code> | <code>LAYERX_INDEXER_RELAY_CA_DER</code> |
| <code>interface.8.env-indexer-layerx-indexer-relay-url-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/config.rs:297</code> | <code>LAYERX_INDEXER_RELAY_URL</code> |
| <code>interface.8.env-indexer-layerx-indexer-settlement-genesis-trust-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/settlement.rs:97</code> | <code>LAYERX_INDEXER_GENESIS_TRUST</code> |
| <code>interface.8.env-indexer-layerx-indexer-settlement-initial-sequencer-key-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/settlement.rs:98</code> | <code>LAYERX_INDEXER_INITIAL_SEQUENCER_KEY</code> |
| <code>interface.8.env-indexer-layerx-indexer-settlement-lni-socket-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/settlement.rs:99</code> | <code>LAYERX_INDEXER_LNI_SOCKET</code> |
| <code>interface.8.env-indexer-layerx-indexer-settlement-policy-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/settlement.rs:96</code> | <code>LAYERX_INDEXER_SETTLEMENT_POLICY</code> |
| <code>interface.8.env-indexer-layerx-indexer-stall-secs</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/main.rs:127</code> | <code>let stall_after_secs = match std::env::var("LAYERX_INDEXER_STALL_SECS") {</code> |
| <code>interface.8.env-indexer-layerx-indexer-start-batch-j40</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/indexer/src/config.rs:301</code> | <code>LAYERX_INDEXER_START_BATCH</code> |
| <code>interface.8.env-indexer-layerx-indexer-start-block-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:324</code> | <code>start_block: number(lookup, "LAYERX_INDEXER_START_BLOCK", 1)?,</code> |
| <code>interface.8.env-indexer-layerx-indexer-timeout-ms-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:259</code> | <code>"LAYERX_INDEXER_TIMEOUT_MS",</code> |
| <code>interface.8.env-indexer-layerx-indexer-tls-cert-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:349</code> | <code>tls: lookup("LAYERX_INDEXER_TLS_CERT_DER").is_some(),</code> |
| <code>interface.8.env-indexer-paxscan-database-public-url-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/config.rs:147</code> | <code>let paxscan_url = lookup("PAXSCAN_DATABASE_PUBLIC_URL").ok_or_else(&#124;&#124; {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-indexer-layerx-indexer-abi-dir-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:350 |
| <code>interface.8.env-indexer-layerx-indexer-allow-remote-plaintext-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:262 |
| <code>interface.8.env-indexer-layerx-indexer-backfill-range-blocks-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:115 |
| <code>interface.8.env-indexer-layerx-indexer-batch-interval-ms-j40</code> | notes | kernel batch interval for the relay finality depth; task 8.16.7 |
| <code>interface.8.env-indexer-layerx-indexer-block-time-ms-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:327 |
| <code>interface.8.env-indexer-layerx-indexer-challenge-window-seconds-j40</code> | notes | anchor challenge window the finality depth was derived from; task 8.16.7 |
| <code>interface.8.env-indexer-layerx-indexer-comet-attributes-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:309 |
| <code>interface.8.env-indexer-layerx-indexer-comet-ca-der-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:318 |
| <code>interface.8.env-indexer-layerx-indexer-comet-url-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:318 |
| <code>interface.8.env-indexer-layerx-indexer-db-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:250 |
| <code>interface.8.env-indexer-layerx-indexer-evm-ca-der-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:319 |
| <code>interface.8.env-indexer-layerx-indexer-evm-chain-id-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:325 |
| <code>interface.8.env-indexer-layerx-indexer-evm-url-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:319; named in the do text of task 8.16.7 |
| <code>interface.8.env-indexer-layerx-indexer-explorer-ca-der-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:333 |
| <code>interface.8.env-indexer-layerx-indexer-explorer-url-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:332 |
| <code>interface.8.env-indexer-layerx-indexer-finality-depth-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:281; named in the do text of task 8.16.2 |
| <code>interface.8.env-indexer-layerx-indexer-listen-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:253 |
| <code>interface.8.env-indexer-layerx-indexer-max-units-per-step-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:284 |
| <code>interface.8.env-indexer-layerx-indexer-paxscan-cert-sha256-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:151 |
| <code>interface.8.env-indexer-layerx-indexer-paxscan-tls-unauthenticated-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:152 |
| <code>interface.8.env-indexer-layerx-indexer-pointers-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:353 |
| <code>interface.8.env-indexer-layerx-indexer-poll-ms-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:355 |
| <code>interface.8.env-indexer-layerx-indexer-relay-ca-der-j40</code> | notes | trust anchor of the kernel relay endpoint; task 8.16.7 |
| <code>interface.8.env-indexer-layerx-indexer-relay-url-j40</code> | notes | kernel relay archive endpoint; task 8.16.7 |
| <code>interface.8.env-indexer-layerx-indexer-settlement-genesis-trust-j40</code> | notes | kernel settlement genesis trust; task 8.16.4 |
| <code>interface.8.env-indexer-layerx-indexer-settlement-initial-sequencer-key-j40</code> | notes | kernel settlement initial sequencer key; task 8.16.4 |
| <code>interface.8.env-indexer-layerx-indexer-settlement-lni-socket-j40</code> | notes | kernel node socket for settlement reads; task 8.16.4 |
| <code>interface.8.env-indexer-layerx-indexer-settlement-policy-j40</code> | notes | kernel checkpoint settlement policy file; task 8.16.4 |
| <code>interface.8.env-indexer-layerx-indexer-start-batch-j40</code> | notes | first kernel batch to ingest; task 8.16.7 |
| <code>interface.8.env-indexer-layerx-indexer-start-block-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:324 |
| <code>interface.8.env-indexer-layerx-indexer-timeout-ms-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:259 |
| <code>interface.8.env-indexer-layerx-indexer-tls-cert-der-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:349 |
| <code>interface.8.env-indexer-paxscan-database-public-url-j69</code> | notes | read at services/platform/hosted/indexer/src/config.rs:147; 11 in-repo references, first 8 listed |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.182.1</code> | req | 8182.1 |
| <code>acceptance.8.182.1</code> | scenario | Given an environment with an EVM endpoint and a 500 ms block time, when it is parsed with and without a fixed depth of 3, then the paxeer depth is 0 and then 3 |
| <code>acceptance.8.182.1</code> | test | services/platform/hosted/indexer/src/config.rs::paxeer_source_is_final_at_block_commit (planned) |
| <code>acceptance.8.182.1</code> | qualification | UNRUN |
| <code>req.8182</code> | ac_1 | With no LAYERX_INDEXER_FINALITY_DEPTH the paxeer source has finality depth 0 whatever the block time, and with LAYERX_INDEXER_FINALITY_DEPTH=3 it has depth 3. |
| <code>acceptance.8.182.2</code> | req | 8182.2 |
| <code>acceptance.8.182.2</code> | scenario | Given relay and EVM endpoints with a 30 second window, when the environment is parsed, then both sources exist and the paxeer depth is 0 |
| <code>acceptance.8.182.2</code> | test | services/platform/hosted/indexer/src/config.rs::environment_builds_both_sources (existing) |
| <code>acceptance.8.182.2</code> | qualification | UNRUN |
| <code>req.8182</code> | ac_2 | An environment naming both sources still builds both, with the paxeer depth 0 and the relay depth derived from its window. |
| <code>acceptance.8.186.1</code> | req | 8186.1 |
| <code>acceptance.8.186.1</code> | scenario | Given an EVM endpoint and malformed LAYERX_INDEXER_SETTLEMENT_* values, when the environment is parsed, then the configuration builds |
| <code>acceptance.8.186.1</code> | test | services/platform/hosted/indexer/src/config.rs::settlement_variables_are_not_read (planned) |
| <code>acceptance.8.186.1</code> | qualification | UNRUN |
| <code>req.8186</code> | ac_1 | Config::from_lookup builds an EVM-only configuration even when the four settlement variables hold malformed values. |
| <code>capability.8.16.2</code> | title | Follow the paxeer source at block-commit finality |
| <code>capability.8.16.2</code> | reqs | 8182.1<br>8182.2 |
| <code>capability.8.16.4</code> | title | Retire kernel checkpoint settlement verification in the indexer |
| <code>capability.8.16.4</code> | reqs | 8186.1 |
| <code>resolution.17.811</code> | title | decision.816 binds: Indexer finality on the merged chain is the committed block |
| <code>resolution.17.811</code> | applies_to | decision.816<br>task.8.16.2<br>task.8.16.7 |
| <code>resolution.17.811</code> | source | decision.816 in fragments/J40/decision.kvx; tasks from affects |
| <code>resolution.17.812</code> | title | decision.817 binds: Kernel settlement and relay ingestion retire from the indexer |
| <code>resolution.17.812</code> | applies_to | decision.817<br>task.8.16.4<br>task.8.16.7 |
| <code>resolution.17.812</code> | source | decision.817 in fragments/J40/decision.kvx; tasks from affects |

## indexer-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-indexer-asset-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:322</code> | <code>Ok(None) =&gt; refusal(404, "asset_not_found", None),</code> |
| <code>interface.8.err-indexer-duplicate-query-parameter</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:67</code> | <code>return Err(refusal(400, "duplicate_query_parameter", None));</code> |
| <code>interface.8.err-indexer-explorer-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:339</code> | <code>return refusal(503, "explorer_unavailable", Some(1));</code> |
| <code>interface.8.err-indexer-invalid-cursor</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:77</code> | <code>.ok_or_else(&#124;&#124; refusal(400, "invalid_cursor", None))?,</code> |
| <code>interface.8.err-indexer-invalid-kind</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:101</code> | <code>return Err(refusal(400, "invalid_kind", None));</code> |
| <code>interface.8.err-indexer-invalid-limit</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:92</code> | <code>.ok_or_else(&#124;&#124; refusal(400, "invalid_limit", None))?;</code> |
| <code>interface.8.err-indexer-invalid-path</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:152</code> | <code>let decoded = percent_decode(segment, false).map_err(&#124;_&#124; refusal(400, "invalid_path", None))?;</code> |
| <code>interface.8.err-indexer-invalid-query</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:64</code> | <code>let name = percent_decode(name, true).map_err(&#124;_&#124; refusal(400, "invalid_query", None))?;</code> |
| <code>interface.8.err-indexer-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:389</code> | <code>_ =&gt; refusal(400, "invalid_request", None),</code> |
| <code>interface.8.err-indexer-method-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:280</code> | <code>return refusal(405, "method_not_allowed", None);</code> |
| <code>interface.8.err-indexer-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:329</code> | <code>Ok(_) =&gt; return refusal(404, "not_found", None),</code> |
| <code>interface.8.err-indexer-store-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:148</code> | <code>refusal(503, "store_unavailable", Some(1))</code> |
| <code>interface.8.err-indexer-unknown-query-parameter</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:105</code> | <code>_ =&gt; return Err(refusal(400, "unknown_query_parameter", None)),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-indexer-asset-not-found</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-indexer-duplicate-query-parameter</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-indexer-explorer-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-indexer-invalid-cursor</code> | notes | HTTP 400 |
| <code>interface.8.err-indexer-invalid-kind</code> | notes | HTTP 400 |
| <code>interface.8.err-indexer-invalid-limit</code> | notes | HTTP 400 |
| <code>interface.8.err-indexer-invalid-path</code> | notes | HTTP 400 |
| <code>interface.8.err-indexer-invalid-query</code> | notes | HTTP 400 |
| <code>interface.8.err-indexer-invalid-request</code> | notes | HTTP 400; 77 callers total |
| <code>interface.8.err-indexer-method-not-allowed</code> | notes | HTTP 405; 48 callers total |
| <code>interface.8.err-indexer-not-found</code> | notes | HTTP 404; 134 callers total |
| <code>interface.8.err-indexer-store-unavailable</code> | notes | HTTP 503; 39 callers total |
| <code>interface.8.err-indexer-unknown-query-parameter</code> | notes | HTTP 400 |

## indexer-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-indexer-ai-health-schema</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_health.rs:55</code> | <code>const SCHEMA: &amp;str = "</code> |
| <code>interface.8.fmt-indexer-ai-market-schema</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:78</code> | <code>const SCHEMA: &amp;str = "</code> |
| <code>interface.8.fmt-indexer-ai-reputation-schema</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_reputation.rs:35</code> | <code>const SCHEMA: &amp;str = "</code> |
| <code>interface.8.fmt-indexer-store-schema</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/store.rs:23</code> | <code>const SCHEMA: &amp;str = "</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-indexer-ai-health-schema</code> | notes | AI health SQL schema; read and written only by the defining module |
| <code>interface.8.fmt-indexer-ai-market-schema</code> | notes | AI market SQL schema; read and written only by the defining module |
| <code>interface.8.fmt-indexer-ai-reputation-schema</code> | notes | AI reputation SQL schema; read and written only by the defining module |
| <code>interface.8.fmt-indexer-store-schema</code> | notes | indexer SQL schema; read and written only by the defining module |

## indexer-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-indexer-any-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:285</code> | <code>if path == "/healthz" {</code> |
| <code>interface.8.http-indexer-any-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:291</code> | <code>if path == "/readyz" {</code> |
| <code>interface.8.http-indexer-any-v1-assets</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:303</code> | <code>if path == "/v1/assets" {</code> |
| <code>interface.8.http-indexer-any-v1-assets-2</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:312</code> | <code>if let Some(segment) = path.strip_prefix("/v1/assets/") {</code> |
| <code>interface.8.http-indexer-any-v1-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:326</code> | <code>if let Some(segment) = path.strip_prefix("/v1/history/") {</code> |
| <code>interface.8.http-indexer-any-v1-holders</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:348</code> | <code>if let Some(segment) = path.strip_prefix("/v1/holders/") {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-store-stability-source-j40</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/hosted/indexer/src/store.rs:179</code> | <code>pub const STABILITY_SOURCE: &amp;str = "local_finality_depth";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-indexer-any-healthz</code> | notes | 330 callers total |
| <code>interface.8.http-indexer-any-readyz</code> | notes | 321 callers total |
| <code>interface.8.http-indexer-any-v1-history</code> | notes | 72 callers total |
| <code>interface.8.rs-indexer-layerx-indexer-store-stability-source-j40</code> | notes | the final_basis word of history rows becomes block_commit in task 8.16.7 |

Contract context <code>contract_context.8.indexer-chain-history</code>:

| Field | Frozen rule |
|---|---|
| interface | Indexer history, assets and holders routes over the paxeer chain |
| storage | SQLite store of units, events, transfers, holders and assets |
| errors | invalid_query, invalid_cursor, store_unavailable, explorer_unavailable |
| events | none; the indexer decodes chain logs and consensus events |
| gas | none |
| activation | new construct addresses in wave 1, block-commit finality in wave 3, kernel sources retired in waves 5 and 6 |
| invariants | a row is final once its block is committed; an ABI is assigned only the address its precompile declares |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.187.1</code> | req | 8187.1 |
| <code>acceptance.8.187.1</code> | scenario | Given a relay-only environment and an EVM environment with malformed relay variables, when each is parsed, then the first is refused naming LAYERX_INDEXER_EVM_URL and the second builds |
| <code>acceptance.8.187.1</code> | test | services/platform/hosted/indexer/src/config.rs::relay_variables_are_not_read (planned) |
| <code>acceptance.8.187.1</code> | qualification | UNRUN |
| <code>req.8187</code> | ac_1 | An environment with only LAYERX_INDEXER_RELAY_URL is refused for naming no EVM source, and an EVM environment with malformed relay, batch and window values builds. |
| <code>acceptance.8.187.2</code> | req | 8187.2 |
| <code>acceptance.8.187.2</code> | scenario | Given node rows above and explorer rows below the floor, when the history is merged, then the node row final_basis is block_commit |
| <code>acceptance.8.187.2</code> | test | services/platform/hosted/indexer/src/explorer.rs::evm_history_merges_node_rows_with_explorer_rows_below_the_floor (existing) |
| <code>acceptance.8.187.2</code> | qualification | UNRUN |
| <code>req.8187</code> | ac_2 | Node rows of the EVM history carry final_basis block_commit while explorer rows below the floor keep explorer_api. |
| <code>capability.8.16.7</code> | title | Retire the kernel relay source of the indexer |
| <code>capability.8.16.7</code> | reqs | 8187.1<br>8187.2 |
| <code>service.platform-indexer</code> | purpose | Decodes paxeer chain blocks, logs and consensus events into account history, assets and holder records |
| <code>service.platform-indexer</code> | route | /healthz, /readyz, /v1/history, /v1/assets, /v1/assets/{id}, /v1/holders |
| <code>service.platform-indexer</code> | inputs | LAYERX_INDEXER_EVM_URL, LAYERX_INDEXER_COMET_URL, LAYERX_INDEXER_EXPLORER_URL, the precompiles ABI directory |
| <code>service.platform-indexer</code> | outputs | paginated history, asset and holder answers |
| <code>service.platform-indexer</code> | state | SQLite store |
| <code>service.platform-indexer</code> | dependencies | paxeer chain JSON-RPC, consensus RPC, explorer API |
| <code>service.platform-indexer</code> | failure_policy | a stalled source makes readiness fail; a reorganised block is rolled back before its replacement is stored |
| <code>service.platform-indexer</code> | completion_task | 8.0.9 |
| <code>service.platform-indexer</code> | operation_task | 8.16.7 |
| <code>service.platform-indexer</code> | domain | 8 |

## indexer-metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.met-indexer-paxai-health-latency-p95-ms</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_health.rs:1107</code> | <code>writeln!(formatter, "# TYPE paxai_health_latency_p95_ms histogram")?;</code> |
| <code>interface.8.met-indexer-paxai-health-success-ppm</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_health.rs:1098</code> | <code>writeln!(formatter, "# TYPE paxai_health_success_ppm gauge")?;</code> |
| <code>interface.8.met-indexer-paxai-health-withheld-observations-total</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_health.rs:1134</code> | <code>"# TYPE paxai_health_withheld_observations_total counter"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.met-indexer-paxai-health-latency-p95-ms</code> | notes | no in-repo scraper or dashboard cites it |

## indexer-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-indexer-layerx-indexer-abi-keccak</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/abi.rs:54</code> | <code>pub fn keccak(bytes: &amp;[u8]) -&gt; [u8; 32] {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-abi-known-precompiles-j40</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/hosted/indexer/src/abi.rs:22</code> | <code>pub const KNOWN_PRECOMPILES: [(&amp;str, u16); 19] = [</code> |
| <code>interface.8.rs-indexer-layerx-indexer-abi-nested-precompiles-j40</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/platform/hosted/indexer/src/abi.rs</code> (planned) | <code>pub const NESTED_PRECOMPILES: [(&amp;str, &amp;str, u16); N]</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-availability</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/lib.rs:16</code> | <code>pub mod ai_market;</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-cursorkeyring</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:589</code> | <code>pub struct CursorKeyring {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-epochstatus</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:424</code> | <code>pub enum EpochStatus {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-freshness</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:389</code> | <code>pub enum Freshness {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-pagerequest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:450</code> | <code>pub struct PageRequest {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-parse-epoch-query</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:548</code> | <code>pub fn parse_epoch_query(market: &amp;str, query: Option&lt;&amp;str&gt;) -&gt; ViewResult&lt;(MarketId, u64, u8)&gt; {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-parse-snapshot-query</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:528</code> | <code>pub fn parse_snapshot_query(</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-presence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:42</code> | <code>pub use layerx_programs_ai_market::types::Presence;</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-projectionstate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:242</code> | <code>pub enum ProjectionState {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-projectionstore</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:712</code> | <code>pub struct ProjectionStore {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-querylimiter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:649</code> | <code>pub struct QueryLimiter {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-viewer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:563</code> | <code>pub struct Viewer {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-viewerror</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/ai_market.rs:145</code> | <code>pub enum ViewError {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-api-serve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/api.rs:439</code> | <code>pub fn serve(listener: &amp;TcpListener, store: &amp;Arc&lt;Store&gt;, tls: Option&lt;&amp;Arc&lt;ServerConfig&gt;&gt;) {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-codec-hex0x</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/codec.rs:20</code> | <code>pub fn hex0x(bytes: &amp;[u8]) -&gt; String {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-store-assetrow</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/store.rs:300</code> | <code>pub struct AssetRow {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-store-holderrow</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/store.rs:284</code> | <code>pub struct HolderRow {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-store-position</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/store.rs:196</code> | <code>pub struct Position {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-store-store</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/store.rs:376</code> | <code>pub struct Store {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-store-transferrow</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/store.rs:252</code> | <code>pub struct TransferRow {</code> |
| <code>interface.8.rs-indexer-layerx-indexer-store-unit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/indexer/src/store.rs:311</code> | <code>pub struct Unit {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rs-indexer-layerx-indexer-abi-known-precompiles-j40</code> | notes | gains the merged construct rows in task 8.16.1, swaps 0x1015 to perps in task 8.16.5 and drops the kernel rows in task 8.16.3 |
| <code>interface.8.rs-indexer-layerx-indexer-abi-nested-precompiles-j40</code> | notes | planned table of precompile ABIs kept below a nested abi directory; tasks 8.16.5 and 8.16.6 |
| <code>interface.8.rs-indexer-layerx-indexer-ai-market-availability</code> | notes | Availability is a variant or associated item of ai_market |

Contract context <code>contract_context.8.indexer-chain-history</code>:

| Field | Frozen rule |
|---|---|
| interface | Indexer history, assets and holders routes over the paxeer chain |
| storage | SQLite store of units, events, transfers, holders and assets |
| errors | invalid_query, invalid_cursor, store_unavailable, explorer_unavailable |
| events | none; the indexer decodes chain logs and consensus events |
| gas | none |
| activation | new construct addresses in wave 1, block-commit finality in wave 3, kernel sources retired in waves 5 and 6 |
| invariants | a row is final once its block is committed; an ABI is assigned only the address its precompile declares |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.181.1</code> | req | 8181.1 |
| <code>acceptance.8.181.1</code> | scenario | Given the repository precompiles directory, when it is loaded with load_dir, then each of the four ABIs is registered at its fixed precompile address |
| <code>acceptance.8.181.1</code> | test | services/platform/hosted/indexer/src/abi.rs::merged_construct_precompiles_are_known (planned) |
| <code>acceptance.8.181.1</code> | qualification | UNRUN |
| <code>req.8181</code> | ac_1 | Loading the repository precompiles directory gives feetoken, xweb, layerxpay and layerxprograms the addresses 0x1018, 0x1019, 0x101B and 0x101D. |
| <code>acceptance.8.183.1</code> | req | 8183.1 |
| <code>acceptance.8.183.1</code> | scenario | Given the repository precompiles directory, when it is loaded and CollateralDeposited and FeedKeyRegistered are looked up at 0x1015 and 0x1020, then both resolve to perps and pricefeed |
| <code>acceptance.8.183.1</code> | test | services/platform/hosted/indexer/src/abi.rs::perps_and_pricefeed_abis_load_at_their_addresses (planned) |
| <code>acceptance.8.183.1</code> | qualification | UNRUN |
| <code>req.8183</code> | ac_1 | The repository precompiles load perps at 0x1015 with CollateralDeposited registered and pricefeed at 0x1020 with FeedKeyRegistered registered, and no ABI named layerxexchange holds 0x1015. |
| <code>acceptance.8.184.1</code> | req | 8184.1 |
| <code>acceptance.8.184.1</code> | scenario | Given the repository precompiles directory, when the four event topics are looked up at their addresses, then each resolves to pecor, orders, stoporders and vault |
| <code>acceptance.8.184.1</code> | test | services/platform/hosted/indexer/src/abi.rs::spot_contract_events_decode_at_their_addresses (planned) |
| <code>acceptance.8.184.1</code> | qualification | UNRUN |
| <code>req.8184</code> | ac_1 | MarketOrderExecuted resolves at 0x1021, LimitOrderCreated at 0x1022, StopLimitOrderCreated at 0x1023 and Deposit at 0x1025 in the repository registry. |
| <code>acceptance.8.185.1</code> | req | 8185.1 |
| <code>acceptance.8.185.1</code> | scenario | Given the precompile table and the repository registry, when 0x1012, 0x1013 and 0x1014 are searched, then neither holds an entry |
| <code>acceptance.8.185.1</code> | test | services/platform/hosted/indexer/src/abi.rs::retired_kernel_precompiles_have_no_address (planned) |
| <code>acceptance.8.185.1</code> | qualification | UNRUN |
| <code>req.8185</code> | ac_1 | KNOWN_PRECOMPILES has no entry for 0x1012, 0x1013 or 0x1014 and no loaded ABI is registered at those addresses. |
| <code>acceptance.8.185.2</code> | req | 8185.2 |
| <code>acceptance.8.185.2</code> | scenario | Given the paxeer block, log and tx_search fixtures, when they are ingested and rolled back, then every remaining assertion holds and no custody leg is recorded |
| <code>acceptance.8.185.2</code> | test | services/platform/hosted/indexer/tests/ingest.rs::paxeer_blocks_logs_and_tx_search_decode_paginate_and_roll_back (existing) |
| <code>acceptance.8.185.2</code> | qualification | UNRUN |
| <code>req.8185</code> | ac_2 | The paxeer fixture still decodes, paginates and rolls back with no custody_deposit transfer and no CustodyDeposit event. |
| <code>capability.8.16.1</code> | title | Know the addresses of the merged construct precompiles in the indexer |
| <code>capability.8.16.1</code> | reqs | 8181.1 |
| <code>capability.8.16.3</code> | title | Drop the kernel verify, custody and anchor precompiles from the indexer |
| <code>capability.8.16.3</code> | reqs | 8185.1<br>8185.2 |
| <code>capability.8.16.5</code> | title | Load the perps and price feed ABIs from their nested directories |
| <code>capability.8.16.5</code> | reqs | 8183.1 |
| <code>capability.8.16.6</code> | title | Register the spot order book contracts in the indexer |
| <code>capability.8.16.6</code> | reqs | 8184.1 |

## internal-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-internal-layerx-event-source-rs-empty-enrollment-mac</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-event-source.rs:43</code> | <code>if arguments.as_slice() == ["--empty-enrollment-mac"] {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-internal-layerx-event-source-rs-empty-enrollment-mac</code> | notes | no in-repo invocation outside the defining file |

## internal-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-internal-layerx-events-bootstrap-credentials-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1090</code> | <code>bootstrap: std::env::var_os("LAYERX_EVENTS_BOOTSTRAP_CREDENTIALS_FILE")</code> |
| <code>interface.8.env-internal-layerx-events-credentials-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-event-source.rs:61</code> | <code>let credentials = secret::required_env("LAYERX_EVENTS_CREDENTIALS_FILE")?;</code> |
| <code>interface.8.env-internal-layerx-events-enrollment-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-event-source.rs:45</code> | <code>let path = secret::required_env("LAYERX_EVENTS_ENROLLMENT_KEY_FILE")?;</code> |
| <code>interface.8.env-internal-layerx-events-kind</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-event-source.rs:44</code> | <code>let kind = events::Kind::parse(&amp;secret::required_env("LAYERX_EVENTS_KIND")?)?;</code> |
| <code>interface.8.env-internal-layerx-events-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-event-source.rs:54</code> | <code>let listen = secret::required_env("LAYERX_EVENTS_LISTEN")?</code> |
| <code>interface.8.env-internal-layerx-events-producers-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-event-source.rs:18</code> | <code>let Ok(path) = std::env::var("LAYERX_EVENTS_PRODUCERS_FILE") else {</code> |
| <code>interface.8.env-internal-layerx-events-state-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-event-source.rs:62</code> | <code>let state = secret::required_env("LAYERX_EVENTS_STATE_DIR")?;</code> |
| <code>interface.8.env-internal-layerx-events-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-event-source.rs:66</code> | <code>let token = secret::read_token("LAYERX_EVENTS_TOKEN_FILE")?;</code> |
| <code>interface.8.env-internal-layerx-events-webhooks</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:665</code> | <code>Upstream::from_environment("LAYERX_EVENTS_WEBHOOKS")?,</code> |
| <code>interface.8.env-internal-layerx-health-addr</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/env-init.sh:185</code> | <code>export LAYERX_HEALTH_ADDR="${LAYERX_HEALTH_ADDR:-&lt;addr&gt;}"</code> |
| <code>interface.8.env-internal-layerx-internal-upstream-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/env-init.sh:106</code> | <code>upstream_root="${LAYERX_INTERNAL_UPSTREAM_ROOT:-}"</code> |
| <code>interface.8.env-internal-layerx-internal-upstream-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/env-init.sh:105</code> | <code>upstream_url="${LAYERX_INTERNAL_UPSTREAM_URL:-}"</code> |
| <code>interface.8.env-internal-layerx-kms-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-kms.rs:8</code> | <code>let listen = secret::required_env("LAYERX_KMS_LISTEN")?</code> |
| <code>interface.8.env-internal-layerx-kms-seal-secret-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-kms.rs:12</code> | <code>let seal_secret = secret::read_token("LAYERX_KMS_SEAL_SECRET_FILE")?;</code> |
| <code>interface.8.env-internal-layerx-kms-state-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-kms.rs:13</code> | <code>let state = secret::required_env("LAYERX_KMS_STATE_DIR")?;</code> |
| <code>interface.8.env-internal-layerx-kms-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/bin/layerx-kms.rs:11</code> | <code>let token = secret::read_token("LAYERX_KMS_TOKEN_FILE")?;</code> |
| <code>interface.8.env-internal-layerx-role</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/env-init.sh:24</code> | <code>group="${LAYERX_ROLE:-}"</code> |
| <code>interface.8.env-internal-layerx-test-webhooks-bin</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer_test_support.rs:233</code> | <code>let executable = std::env::var_os("LAYERX_TEST_WEBHOOKS_BIN")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-internal-layerx-events-token-file-j69</code> | notes | read at services/platform/hosted/internal/src/bin/layerx-event-source.rs:66; 15 in-repo references, first 8 listed |
| <code>interface.8.env-internal-layerx-internal-upstream-root</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-internal-layerx-internal-upstream-url</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-internal-layerx-kms-seal-secret-file-j69</code> | notes | read at services/platform/hosted/internal/src/bin/layerx-kms.rs:12 |
| <code>interface.8.env-internal-layerx-kms-token-file-j69</code> | notes | read at services/platform/hosted/internal/src/bin/layerx-kms.rs:11; 9 in-repo references, first 8 listed |
| <code>interface.8.env-internal-layerx-role</code> | notes | 14 callers total |

## internal-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-internal-admission-deadline-exceeded</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1403</code> | <code>return refusal(503, "admission_deadline_exceeded", None);</code> |
| <code>interface.8.err-internal-algorithm-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:586</code> | <code>return refusal(422, "algorithm_mismatch", None);</code> |
| <code>interface.8.err-internal-client-certificate-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:475</code> | <code>return Some(refusal(401, "client_certificate_required", None));</code> |
| <code>interface.8.err-internal-dependency-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:523</code> | <code>return refusal(503, "dependency_unavailable", Some(5));</code> |
| <code>interface.8.err-internal-enrollment-credential-reused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:974</code> | <code>return Err((409, "enrollment_credential_reused"));</code> |
| <code>interface.8.err-internal-enrollment-generation-exhausted</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1020</code> | <code>.ok_or((503, "enrollment_generation_exhausted"))?;</code> |
| <code>interface.8.err-internal-enrollment-issuer-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:945</code> | <code>return Err((409, "enrollment_issuer_conflict"));</code> |
| <code>interface.8.err-internal-enrollment-malformed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:829</code> | <code>return refusal(400, "enrollment_malformed", None);</code> |
| <code>interface.8.err-internal-enrollment-not-adopted</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:879</code> | <code>return Err((503, "enrollment_not_adopted"));</code> |
| <code>interface.8.err-internal-enrollment-oversized</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:979</code> | <code>return Err((413, "enrollment_oversized"));</code> |
| <code>interface.8.err-internal-enrollment-principal-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:919</code> | <code>return Err((403, "enrollment_principal_mismatch"));</code> |
| <code>interface.8.err-internal-enrollment-removed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:956</code> | <code>return Err((409, "enrollment_removed"));</code> |
| <code>interface.8.err-internal-enrollment-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:846</code> | <code>.map_err(&#124;_&#124; (503, "enrollment_unavailable"))?;</code> |
| <code>interface.8.err-internal-enrollment-upstream-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:913</code> | <code>.map_err(&#124;_&#124; (503, "enrollment_upstream_unavailable"))?;</code> |
| <code>interface.8.err-internal-event-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1536</code> | <code>&#124;&#124; refusal(404, "event_not_found", None),</code> |
| <code>interface.8.err-internal-idempotency-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:528</code> | <code>Ok(Created::Conflict) =&gt; refusal(409, "idempotency_conflict", None),</code> |
| <code>interface.8.err-internal-idempotency-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:506</code> | <code>return refusal(400, "idempotency_key_required", None);</code> |
| <code>interface.8.err-internal-invalid-message</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:574</code> | <code>return refusal(400, "invalid_message", None);</code> |
| <code>interface.8.err-internal-invalid-purpose</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:518</code> | <code>return refusal(422, "invalid_purpose", None);</code> |
| <code>interface.8.err-internal-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:279</code> | <code>&#124;_&#124; refusal(400, "invalid_request", None),</code> |
| <code>interface.8.err-internal-lxp-wire-version</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer_test_support.rs:263</code> | <code>("LXP_WIRE_VERSION", "3".to_owned()),</code> |
| <code>interface.8.err-internal-message-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:577</code> | <code>return refusal(413, "message_too_large", None);</code> |
| <code>interface.8.err-internal-method-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:349</code> | <code>Ok(request) if request.method != "GET" =&gt; refusal(405, "method_not_allowed", None),</code> |
| <code>interface.8.err-internal-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1547</code> | <code>refusal(404, "not_found", None)</code> |
| <code>interface.8.err-internal-serialization-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:88</code> | <code>&#124;_&#124; refusal(500, "serialization_failed", None),</code> |
| <code>interface.8.err-internal-source-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1518</code> | <code>&#124;_&#124; refusal(503, "source_unavailable", Some(5)),</code> |
| <code>interface.8.err-internal-unauthorized</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:826</code> | <code>return refusal(401, "unauthorized", None);</code> |
| <code>interface.8.err-internal-unknown-key</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:541</code> | <code>return refusal(404, "unknown_key", None);</code> |
| <code>interface.8.err-internal-unsupported-algorithm</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:515</code> | <code>return refusal(422, "unsupported_algorithm", None);</code> |
| <code>interface.8.err-internal-waiting-principals</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:797</code> | <code>refusal(503, "waiting_principals", Some(PRINCIPAL_POLL.as_secs()))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-internal-admission-deadline-exceeded</code> | notes | HTTP 503 |
| <code>interface.8.err-internal-algorithm-mismatch</code> | notes | HTTP 422 |
| <code>interface.8.err-internal-client-certificate-required</code> | notes | HTTP 401; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-internal-dependency-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-internal-enrollment-credential-reused</code> | notes | HTTP 409 |
| <code>interface.8.err-internal-enrollment-generation-exhausted</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-internal-enrollment-issuer-conflict</code> | notes | HTTP 409 |
| <code>interface.8.err-internal-enrollment-malformed</code> | notes | HTTP 400 |
| <code>interface.8.err-internal-enrollment-not-adopted</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-internal-enrollment-oversized</code> | notes | HTTP 413 |
| <code>interface.8.err-internal-enrollment-principal-mismatch</code> | notes | HTTP 403 |
| <code>interface.8.err-internal-enrollment-removed</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-internal-enrollment-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-internal-enrollment-upstream-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-internal-event-not-found</code> | notes | HTTP 404 |
| <code>interface.8.err-internal-idempotency-conflict</code> | notes | HTTP 409; 30 callers total |
| <code>interface.8.err-internal-idempotency-key-required</code> | notes | HTTP 400 |
| <code>interface.8.err-internal-invalid-message</code> | notes | HTTP 400 |
| <code>interface.8.err-internal-invalid-purpose</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-internal-invalid-request</code> | notes | HTTP 400; 77 callers total |
| <code>interface.8.err-internal-message-too-large</code> | notes | HTTP 413; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-internal-method-not-allowed</code> | notes | HTTP 405; 48 callers total |
| <code>interface.8.err-internal-not-found</code> | notes | HTTP 404; 136 callers total |
| <code>interface.8.err-internal-serialization-failed</code> | notes | HTTP 500; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-internal-source-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-internal-unauthorized</code> | notes | HTTP 401; 91 callers total |
| <code>interface.8.err-internal-unknown-key</code> | notes | HTTP 404 |
| <code>interface.8.err-internal-unsupported-algorithm</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-internal-waiting-principals</code> | notes | HTTP 503 |

## internal-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-internal-enrollment-snapshot</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1818</code> | <code>&amp;directory.join("enrollment-snapshot.json"),</code> |
| <code>interface.8.fmt-internal-journal-log</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/journal.rs:12</code> | <code>const JOURNAL_FILE: &amp;str = "journal.log";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-internal-enrollment-snapshot</code> | notes | enrollment snapshot file; read and written only by the defining module |
| <code>interface.8.fmt-internal-journal-log</code> | notes | internal journal file |

## internal-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-internal-any-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:348</code> | <code>Ok(request) if request.path != "/healthz" =&gt; refusal(404, "not_found", None),</code> |
| <code>interface.8.http-internal-get-internal-v1-producer-readiness</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1341</code> | <code>"/internal/v1/producer-readiness" &#124; "/internal/v1/reader-readiness"</code> |
| <code>interface.8.http-internal-get-internal-v1-reader-readiness</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1341</code> | <code>"/internal/v1/producer-readiness" &#124; "/internal/v1/reader-readiness"</code> |
| <code>interface.8.http-internal-get-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:788</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/livez" {</code> |
| <code>interface.8.http-internal-get-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:791</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/readyz" {</code> |
| <code>interface.8.http-internal-get-v1-signing-keys</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:612</code> | <code>("GET", path) =&gt; path.strip_prefix("/v1/signing-keys/").map_or_else(</code> |
| <code>interface.8.http-internal-post-internal-v1-enrollments</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1335</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/internal/v1/enrollments" {</code> |
| <code>interface.8.http-internal-post-internal-v1-observe</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:1493</code> | <code>&amp;&amp; request.path == "/internal/v1/observe"</code> |
| <code>interface.8.http-internal-post-v1-signatures</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:611</code> | <code>("POST", "/v1/signatures") =&gt; self.sign(request),</code> |
| <code>interface.8.http-internal-post-v1-signing-keys</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:610</code> | <code>("POST", "/v1/signing-keys") =&gt; self.create_key(request),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-internal-any-healthz</code> | notes | 330 callers total |
| <code>interface.8.http-internal-get-livez</code> | notes | 118 callers total |
| <code>interface.8.http-internal-get-readyz</code> | notes | 319 callers total |

## internal-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-internal-layerx-platform-internal-base64</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/lib.rs:11</code> | <code>pub mod base64;</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-base64-decode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/base64.rs:37</code> | <code>pub fn decode(value: &amp;str) -&gt; Option&lt;Vec&lt;u8&gt;&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-base64-encode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/base64.rs:8</code> | <code>pub fn encode(bytes: &amp;[u8]) -&gt; String {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-events-enrollment-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:410</code> | <code>pub fn enrollment_key(path: &amp;Path) -&gt; Result&lt;Zeroizing&lt;String&gt;, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-events-enrollment-snapshot-mac</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:426</code> | <code>pub fn enrollment_snapshot_mac(</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-events-enrollment-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:221</code> | <code>pub const ENROLLMENT_VERSION: u64 = 1;</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-events-fact</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:76</code> | <code>pub struct Fact {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-events-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:21</code> | <code>pub enum Kind {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-events-producercredential</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:105</code> | <code>pub struct ProducerCredential {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-events-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:83</code> | <code>pub struct Record {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-events-service</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/events.rs:800</code> | <code>pub struct Service {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-evm-sender-eip1559call</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/evm_sender.rs:3</code> | <code>pub use layerx_crypto::evm_transaction::{Eip1559Call, Eip7702Authorization, SignedAuthorization};</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-evm-sender-evmsender</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/evm_sender.rs:70</code> | <code>pub struct EvmSender {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-evm-sender-evmsendererror</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/evm_sender.rs:16</code> | <code>pub enum EvmSenderError {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-gateway-http</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/lib.rs:14</code> | <code>pub mod gateway_http;</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-gateway-http-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/gateway_http.rs:86</code> | <code>pub struct Client {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-gateway-http-endpoint</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/gateway_http.rs:14</code> | <code>pub struct Endpoint {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/lib.rs:15</code> | <code>pub mod http;</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-health-listen</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:339</code> | <code>pub fn health_listen() -&gt; Result&lt;SocketAddr, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-io-timeout</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:20</code> | <code>pub const IO_TIMEOUT: Duration = Duration::from_secs(8);</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:86</code> | <code>pub fn json&lt;T: Serialize&gt;(status: u16, value: &amp;T) -&gt; Response {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-ok</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:76</code> | <code>pub fn ok(body: String) -&gt; Response {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-parse-client-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:187</code> | <code>pub fn parse_client_request(stream: &amp;mut impl Read) -&gt; Result&lt;Request, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-read-http-message</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:121</code> | <code>pub fn read_http_message(stream: &amp;mut impl Read, maximum: usize) -&gt; Result&lt;Request, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-refusal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:99</code> | <code>pub fn refusal(status: u16, code: &amp;str, retry_after: Option&lt;u64&gt;) -&gt; Response {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:68</code> | <code>pub struct Response {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-http-write-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/http.rs:219</code> | <code>pub fn write_response(stream: &amp;mut impl Write, response: &amp;Response) -&gt; Result&lt;(), String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-journal-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/journal.rs:23</code> | <code>pub struct Journal {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-journal-lock-refusal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/journal.rs:20</code> | <code>pub const LOCK_REFUSAL: &amp;str = "journal lock";</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-kms-keystore</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:137</code> | <code>pub struct KeyStore {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-kms-seal-label</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:24</code> | <code>pub const SEAL_LABEL: &amp;[u8] = b"layerx-kms-ed25519-seed";</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-kms-service</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/kms.rs:456</code> | <code>pub struct Service {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-principal-principalclient</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/principal.rs:6</code> | <code>pub struct PrincipalClient {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-principal-principalclient-from-environment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/principal.rs:24</code> | <code>pub fn from_environment(prefix: &amp;str) -&gt; Result&lt;Self, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-client</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:611</code> | <code>pub struct Client {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-client-from-environment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:657</code> | <code>pub fn from_environment(kinds: &amp;[&amp;str]) -&gt; Result&lt;Self, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-client-from-environment-if-configured</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:707</code> | <code>pub fn from_environment_if_configured(kinds: &amp;[&amp;str]) -&gt; Result&lt;Option&lt;Self&gt;, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-deliveryfailure</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:220</code> | <code>pub enum DeliveryFailure {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-deliverystate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:250</code> | <code>pub struct DeliveryState {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-event-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:56</code> | <code>pub fn event_id(kind: &amp;str, resource: &amp;str, sequence: u64) -&gt; String {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-health</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:527</code> | <code>pub struct Health {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-max-delivery-attempts</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:214</code> | <code>pub const MAX_DELIVERY_ATTEMPTS: u16 = 5;</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-max-pending</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:12</code> | <code>pub const MAX_PENDING: usize = 1024;</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-max-scheduled-principals</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:216</code> | <code>pub const MAX_SCHEDULED_PRINCIPALS: usize = 10_000;</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-observation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:35</code> | <code>pub struct Observation {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-outbox</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:180</code> | <code>pub trait Outbox: Send + Sync + 'static {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-pending</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:152</code> | <code>pub struct Pending {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-producer-queuestate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/producer.rs:274</code> | <code>pub struct QueueState {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-seal-sealkey</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/seal.rs:13</code> | <code>pub struct SealKey {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-secret-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/secret.rs:148</code> | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-secret-random-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/secret.rs:190</code> | <code>pub fn random_hex(bytes: usize) -&gt; Result&lt;Zeroizing&lt;String&gt;, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-secret-read-secret-file</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/secret.rs:21</code> | <code>pub fn read_secret_file(path: &amp;Path) -&gt; Result&lt;Zeroizing&lt;String&gt;, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-secret-required-env</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/secret.rs:71</code> | <code>pub fn required_env(name: &amp;str) -&gt; Result&lt;String, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-secret-sha256-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/secret.rs:182</code> | <code>pub fn sha256_hex(value: &amp;[u8]) -&gt; String {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-secret-unhex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/secret.rs:159</code> | <code>pub fn unhex(value: &amp;str) -&gt; Option&lt;Vec&lt;u8&gt;&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-secret-unix-seconds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/secret.rs:94</code> | <code>pub fn unix_seconds() -&gt; Result&lt;u64, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-tls</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/lib.rs:22</code> | <code>pub mod tls;</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-tls-origin</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/tls.rs:158</code> | <code>pub struct Origin {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-tls-server-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/tls.rs:116</code> | <code>pub fn server_config(prefix: &amp;str) -&gt; Result&lt;Arc&lt;ServerConfig&gt;, String&gt; {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-tls-upstream</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/tls.rs:218</code> | <code>pub struct Upstream {</code> |
| <code>interface.8.rs-internal-layerx-platform-internal-tls-upstream-from-environment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/internal/src/tls.rs:250</code> | <code>pub fn from_environment(prefix: &amp;str) -&gt; Result&lt;Self, String&gt; {</code> |

## node-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-node-bootstrap-sh-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:274</code> | <code>--asset) ASSET_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-custody-profile</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:306</code> | <code>--custody-profile) CUSTODY_PROFILE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-custody-registry</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:307</code> | <code>--custody-registry) CUSTODY_REGISTRY=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-data-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:259</code> | <code>--data-dir) DATA_DIR=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-enable-module</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:297</code> | <code>--enable-module)</code> |
| <code>interface.8.cli-node-bootstrap-sh-force</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:310</code> | <code>--force) FORCE=1; shift ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-generation-authorization-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:267</code> | <code>--generation-authorization-dir)</code> |
| <code>interface.8.cli-node-bootstrap-sh-generation-run-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:264</code> | <code>--generation-run-dir)</code> |
| <code>interface.8.cli-node-bootstrap-sh-generation-target-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:261</code> | <code>--generation-target-dir)</code> |
| <code>interface.8.cli-node-bootstrap-sh-genesis-build</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:305</code> | <code>--genesis-build) GENESIS_BUILD=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-genesis-metadata</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:275</code> | <code>--genesis-metadata) GENESIS_METADATA=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-genesis-timestamp-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:296</code> | <code>--genesis-timestamp-ms) GENESIS_TIMESTAMP_MS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-handover-authority</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:276</code> | <code>--handover-authority)</code> |
| <code>interface.8.cli-node-bootstrap-sh-layerxd</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:304</code> | <code>--layerxd) LAYERXD=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-lni-gid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:292</code> | <code>--lni-gid) LNI_GID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-lni-uid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:291</code> | <code>--lni-uid) LNI_UID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-migrations</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:303</code> | <code>--migrations) MIGRATIONS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-module-fees</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:287</code> | <code>--module-fees) MODULE_FEES=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-module-profile</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:300</code> | <code>--module-profile)</code> |
| <code>interface.8.cli-node-bootstrap-sh-network-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:270</code> | <code>--network-id) NETWORK_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-perps-oracle-transport</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:283</code> | <code>--perps-oracle-transport)</code> |
| <code>interface.8.cli-node-bootstrap-sh-perps-order-tif</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:280</code> | <code>--perps-order-tif)</code> |
| <code>interface.8.cli-node-bootstrap-sh-program-port</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:289</code> | <code>--program-port) PROGRAM_PORT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-program-token-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:293</code> | <code>--program-token-file) PROGRAM_TOKEN_FILE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-replica-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:295</code> | <code>--replica-id) REPLICA_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-replica-port</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:290</code> | <code>--replica-port) REPLICA_PORT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-replica-token-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:294</code> | <code>--replica-token-file) REPLICA_TOKEN_FILE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-run-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:260</code> | <code>--run-dir) RUN_DIR=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-sequencer-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:271</code> | <code>--sequencer-key) SEQUENCER_KEY_FILE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-settlement-document</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:309</code> | <code>--settlement-document) SETTLEMENT_DOCUMENT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-settlement-env</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:308</code> | <code>--settlement-env) SETTLEMENT_ENV=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-treasury-balance</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:288</code> | <code>--treasury-balance) TREASURY_BALANCE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-treasury-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:272</code> | <code>--treasury-key) TREASURY_KEY_FILE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-treasury-signer-socket</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:273</code> | <code>--treasury-signer-socket) TREASURY_SIGNER_SOCKET=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-bootstrap-sh-withdrawal-fee</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:286</code> | <code>--withdrawal-fee) WITHDRAWAL_FEE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-client-py-expected-peer-gid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/client.py:139</code> | <code>parser.add_argument('--expected-peer-gid', type=int)</code> |
| <code>interface.8.cli-node-client-py-expected-peer-uid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/client.py:138</code> | <code>parser.add_argument('--expected-peer-uid', type=int)</code> |
| <code>interface.8.cli-node-client-py-expected-public-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/client.py:140</code> | <code>parser.add_argument('--expected-public-key')</code> |
| <code>interface.8.cli-node-client-py-socket</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/client.py:136</code> | <code>parser.add_argument('--socket', required=True)</code> |
| <code>interface.8.cli-node-client-py-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/client.py:137</code> | <code>parser.add_argument('--timeout-seconds', default=10.0, type=float)</code> |
| <code>interface.8.cli-node-generation-client-py-broker-uid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_client.py:345</code> | <code>parser.add_argument("--broker-uid", type=int, default=4020)</code> |
| <code>interface.8.cli-node-generation-client-py-socket</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_client.py:343</code> | <code>parser.add_argument("--socket", required=True)</code> |
| <code>interface.8.cli-node-generation-client-py-watch-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_client.py:344</code> | <code>parser.add_argument("--watch-seconds", type=float, default=1)</code> |
| <code>interface.8.cli-node-generation-transport-py-allowed-gid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_transport.py:294</code> | <code>parser.add_argument('--allowed-gid', type=int, required=True)</code> |
| <code>interface.8.cli-node-generation-transport-py-allowed-uid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_transport.py:293</code> | <code>parser.add_argument('--allowed-uid', type=int, required=True)</code> |
| <code>interface.8.cli-node-genesis-fees-py-check</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/genesis_fees.py:84</code> | <code>parser.add_argument('--check', action='store_true')</code> |
| <code>interface.8.cli-node-genesis-fees-py-module-fees</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/genesis_fees.py:85</code> | <code>parser.add_argument('--module-fees', type=Path)</code> |
| <code>interface.8.cli-node-reset-state-py-allowed-gid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1092</code> | <code>parser.add_argument("--allowed-gid", type=int, default=os.getegid())</code> |
| <code>interface.8.cli-node-reset-state-py-data-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1034</code> | <code>parser.add_argument("--data-dir", required=True)</code> |
| <code>interface.8.cli-node-reset-state-py-expected-phase</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1095</code> | <code>parser.add_argument("--expected-phase")</code> |
| <code>interface.8.cli-node-reset-state-py-extra</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1097</code> | <code>parser.add_argument("--extra", "--extra-json", dest="extra")</code> |
| <code>interface.8.cli-node-reset-state-py-extra-json</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1097</code> | <code>parser.add_argument("--extra", "--extra-json", dest="extra")</code> |
| <code>interface.8.cli-node-reset-state-py-generation</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1067</code> | <code>parser.add_argument("--generation", required=True, type=int)</code> |
| <code>interface.8.cli-node-reset-state-py-initial-generation</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1093</code> | <code>parser.add_argument("--initial-generation", type=int, default=1)</code> |
| <code>interface.8.cli-node-reset-state-py-network</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1091</code> | <code>parser.add_argument("--network", type=int, required=True)</code> |
| <code>interface.8.cli-node-reset-state-py-new-phase</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1096</code> | <code>parser.add_argument("--new-phase")</code> |
| <code>interface.8.cli-node-reset-state-py-reset-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1094</code> | <code>parser.add_argument("--reset-id")</code> |
| <code>interface.8.cli-node-reset-state-py-run-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1035</code> | <code>parser.add_argument("--run-dir", required=True)</code> |
| <code>interface.8.cli-node-reset-state-py-socket</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1098</code> | <code>parser.add_argument("--socket")</code> |
| <code>interface.8.cli-node-reset-state-py-state-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:1033</code> | <code>parser.add_argument("--state-dir", required=True)</code> |
| <code>interface.8.cli-node-signer-py-allowed-uid</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:289</code> | <code>parser.add_argument('--allowed-uid', required=True, type=uid_list)</code> |
| <code>interface.8.cli-node-signer-py-binding-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:297</code> | <code>parser.add_argument('--binding-policy', default='')</code> |
| <code>interface.8.cli-node-signer-py-key-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:291</code> | <code>parser.add_argument('--key-file', default='')</code> |
| <code>interface.8.cli-node-signer-py-provider</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:290</code> | <code>parser.add_argument('--provider', default='file', choices=('file', 'command'))</code> |
| <code>interface.8.cli-node-signer-py-provider-command</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:292</code> | <code>parser.add_argument('--provider-command', default='')</code> |
| <code>interface.8.cli-node-signer-py-provider-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:293</code> | <code>parser.add_argument('--provider-timeout-seconds', default=10, type=positive)</code> |
| <code>interface.8.cli-node-signer-py-public-key-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:295</code> | <code>parser.add_argument('--public-key-file', default='')</code> |
| <code>interface.8.cli-node-signer-py-request-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:296</code> | <code>parser.add_argument('--request-timeout-seconds', default=5, type=positive)</code> |
| <code>interface.8.cli-node-signer-py-socket</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:288</code> | <code>parser.add_argument('--socket', required=True)</code> |
| <code>interface.8.cli-node-signer-py-socket-group</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:294</code> | <code>parser.add_argument('--socket-group', default=None, type=int)</code> |
| <code>interface.8.cli-node-supervisor-sh-data-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:75</code> | <code>--data-dir) DATA_DIR=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-supervisor-sh-layerxd</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:78</code> | <code>--layerxd) LAYERXD=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-supervisor-sh-role</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:74</code> | <code>--role) ROLE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-supervisor-sh-run-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:76</code> | <code>--run-dir) RUN_DIR=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-supervisor-sh-socat</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:79</code> | <code>--socat) SOCAT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-node-supervisor-sh-state-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:77</code> | <code>--state-dir) STATE_DIR=$2; shift 2 ;;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-node-bootstrap-sh-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-custody-registry</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-force</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-generation-authorization-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-generation-run-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-generation-target-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-genesis-metadata</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-genesis-timestamp-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-handover-authority</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-migrations</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-module-fees</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-perps-oracle-transport</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-perps-order-tif</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-program-token-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-replica-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-replica-token-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-settlement-document</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-treasury-balance</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-treasury-signer-socket</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-bootstrap-sh-withdrawal-fee</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-client-py-expected-peer-gid</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-client-py-expected-peer-uid</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-client-py-expected-public-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-client-py-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-generation-client-py-broker-uid</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-generation-transport-py-allowed-gid</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-generation-transport-py-allowed-uid</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-genesis-fees-py-module-fees</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-allowed-gid</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-data-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-expected-phase</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-extra</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-extra-json</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-generation</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-initial-generation</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-network</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-new-phase</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-reset-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-run-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-socket</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-reset-state-py-state-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-allowed-uid</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-binding-policy</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-key-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-provider</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-provider-command</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-provider-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-public-key-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-request-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-signer-py-socket-group</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-supervisor-sh-layerxd</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-supervisor-sh-socat</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-node-supervisor-sh-state-dir</code> | notes | no in-repo invocation outside the defining file |

## node-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-node-initial-generation</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:667</code> | <code>--initial-generation "${INITIAL_GENERATION:-1}" --allowed-gid "${LNI_GID:-$(id -g)}"); then</code> |
| <code>interface.8.env-node-layerx-core-env-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_client.py:182</code> | <code>adopted["LAYERX_CORE_ENV_FILE"] = "/proc/self/fd/" + str(files["core.env"])</code> |
| <code>interface.8.env-node-layerx-generation-fd-mode</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_client.py:190</code> | <code>result_env["LAYERX_GENERATION_FD_MODE"] = "1"</code> |
| <code>interface.8.env-node-layerx-generation-number</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_client.py:191</code> | <code>result_env["LAYERX_GENERATION_NUMBER"] = str(result["generation"])</code> |
| <code>interface.8.env-node-layerx-guarantor-anchor-abi</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:48</code> | <code>anchor_abi=${LAYERX_GUARANTOR_ANCHOR_ABI:-/opt/layerx/precompiles/layerxanchor/abi.json}</code> |
| <code>interface.8.env-node-layerx-guarantor-binary</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:58</code> | <code>guarantor_binary=${LAYERX_GUARANTOR_BINARY:-/usr/local/bin/layerx-guarantor}</code> |
| <code>interface.8.env-node-layerx-guarantor-identity-dir</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:20</code> | <code>: "${LAYERX_GUARANTOR_IDENTITY_DIR:?identity directory is required}"</code> |
| <code>interface.8.env-node-layerx-guarantor-key-file</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:78</code> | <code>: "${LAYERX_GUARANTOR_KEY_FILE:?generation identity key is required}"</code> |
| <code>interface.8.env-node-layerx-guarantor-lni-socket</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:21</code> | <code>: "${LAYERX_GUARANTOR_LNI_SOCKET:?LNI socket is required}"</code> |
| <code>interface.8.env-node-layerx-guarantor-node-config</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:81</code> | <code>: "${LAYERX_GUARANTOR_NODE_CONFIG:?generation node config is required}"</code> |
| <code>interface.8.env-node-layerx-guarantor-producer-env-file</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:75</code> | <code>producer_env=${LAYERX_GUARANTOR_PRODUCER_ENV_FILE:?generation producer environment is required}</code> |
| <code>interface.8.env-node-layerx-guarantor-publication-authorization-file</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:36</code> | <code>if [ -n "${LAYERX_GUARANTOR_PUBLICATION_AUTHORIZATION_FILE:-}" ] &amp;&amp; \</code> |
| <code>interface.8.env-node-layerx-guarantor-publication-authorization-source</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:37</code> | <code>[ -z "${LAYERX_GUARANTOR_PUBLICATION_AUTHORIZATION_SOURCE:-}" ]; then</code> |
| <code>interface.8.env-node-layerx-guarantor-publication-inputs-dir</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:35</code> | <code>inputs_override=${LAYERX_GUARANTOR_PUBLICATION_INPUTS_DIR:-}</code> |
| <code>interface.8.env-node-layerx-guarantor-settlement-env</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:23</code> | <code>: "${LAYERX_GUARANTOR_SETTLEMENT_ENV:?settlement environment is required}"</code> |
| <code>interface.8.env-node-layerx-guarantor-state-dir</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:22</code> | <code>: "${LAYERX_GUARANTOR_STATE_DIR:?state directory is required}"</code> |
| <code>interface.8.env-node-layerx-guarantor-submitter-key-file</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:24</code> | <code>: "${LAYERX_GUARANTOR_SUBMITTER_KEY_FILE:?submitter key is required}"</code> |
| <code>interface.8.env-node-layerx-node-checkpoint-directory</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:207</code> | <code>"${LAYERX_NODE_CHECKPOINT_DIRECTORY:?}/da-bodies.log" "$derived")</code> |
| <code>interface.8.env-node-layerx-node-generation-transport</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:208</code> | <code>GENERATION_TRANSPORT=${LAYERX_NODE_GENERATION_TRANSPORT:-0}</code> |
| <code>interface.8.env-node-layerx-node-genesis-manifest</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:76</code> | <code>manifest=${LAYERX_NODE_GENESIS_MANIFEST:?generation manifest is required}</code> |
| <code>interface.8.env-node-layerx-node-genesis-registration</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:77</code> | <code>registration=${LAYERX_NODE_GENESIS_REGISTRATION:?generation registration is required}</code> |
| <code>interface.8.env-node-layerx-node-handover-activity</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:208</code> | <code>if [ -n "${LAYERX_NODE_HANDOVER_ACTIVITY:-}" ]; then</code> |
| <code>interface.8.env-node-layerx-node-handover-authority-public-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:481</code> | <code>if [ -n "${LAYERX_NODE_HANDOVER_AUTHORITY_PUBLIC_KEY:-}" ]; then</code> |
| <code>interface.8.env-node-layerx-node-identities</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:80</code> | <code>: "${LAYERX_NODE_IDENTITIES:?generation identity inventory is required}"</code> |
| <code>interface.8.env-node-layerx-node-registry-material</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:512</code> | <code>if [ "${LAYERX_NODE_REGISTRY_MATERIAL:-0}" = 1 ]; then</code> |
| <code>interface.8.env-node-layerx-node-reset-staged-generations</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:195</code> | <code>self.staged = os.environ.get("LAYERX_NODE_RESET_STAGED_GENERATIONS", "0") == "1"</code> |
| <code>interface.8.env-node-layerx-node-sequencer-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/sequencer-env.sh:53</code> | <code>key_file=${LAYERX_NODE_SEQUENCER_KEY_FILE:-}</code> |
| <code>interface.8.env-node-layerx-node-settlement-wait-seconds</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:248</code> | <code>limit=${LAYERX_NODE_SETTLEMENT_WAIT_SECONDS:-3600}</code> |
| <code>interface.8.env-node-layerx-node-snapshot</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:79</code> | <code>: "${LAYERX_NODE_SNAPSHOT:?generation snapshot is required}"</code> |
| <code>interface.8.env-node-layerx-node-treasury-signer-wait-seconds</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:463</code> | <code>limit=${LAYERX_NODE_TREASURY_SIGNER_WAIT_SECONDS:-600}</code> |
| <code>interface.8.env-node-layerx-paxeer-settlement-json</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/bootstrap.sh:242</code> | <code>SETTLEMENT_DOCUMENT=${LAYERX_PAXEER_SETTLEMENT_JSON:-}</code> |
| <code>interface.8.env-node-layerx-reset-owner-fd</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/reset_state.py:925</code> | <code>raw = os.environ.get("LAYERX_RESET_OWNER_FD", "")</code> |
| <code>interface.8.env-node-lni-gid</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:667</code> | <code>--initial-generation "${INITIAL_GENERATION:-1}" --allowed-gid "${LNI_GID:-$(id -g)}"); then</code> |
| <code>interface.8.env-node-pythonpath</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/node/guarantor.sh:125</code> | <code>export PYTHONPATH=${PYTHONPATH:-/opt/layerx:/opt/layerx/human}</code> |
| <code>interface.8.env-node-role</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:59</code> | <code>log() { printf 'supervisor[%s]: %s\n' "${ROLE:-handler}" "$*" &gt;&amp;2; }</code> |
| <code>interface.8.env-node-settlement-lines</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:202</code> | <code>if [ -n "${SETTLEMENT_LINES:-}" ]; then</code> |
| <code>interface.8.env-node-socat</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:122</code> | <code>SOCAT=${SOCAT:-$(command -v socat &#124;&#124; true)}</code> |
| <code>interface.8.env-node-state-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/node/supervisor.sh:90</code> | <code>STATE_DIR=${STATE_DIR:-$(dirname "$DATA_DIR")/supervisor-state}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-node-initial-generation</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-node-layerx-core-env-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-node-layerx-generation-number</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-node-layerx-guarantor-anchor-abi</code> | overridden_by | override.J40.707 |
| <code>interface.8.env-node-layerx-guarantor-binary</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-node-layerx-guarantor-binary</code> | overridden_by | override.J40.708 |
| <code>interface.8.env-node-layerx-guarantor-identity-dir</code> | overridden_by | override.J40.700 |
| <code>interface.8.env-node-layerx-guarantor-key-file</code> | overridden_by | override.J40.712 |
| <code>interface.8.env-node-layerx-guarantor-lni-socket</code> | notes | LNI transport retired; reads move to eth_call and gRPC queries (spec/layerx-merge/spec.kvx:60434) |
| <code>interface.8.env-node-layerx-guarantor-node-config</code> | overridden_by | override.J40.715 |
| <code>interface.8.env-node-layerx-guarantor-producer-env-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-node-layerx-guarantor-producer-env-file</code> | overridden_by | override.J40.709 |
| <code>interface.8.env-node-layerx-guarantor-publication-authorization-file</code> | overridden_by | override.J40.705 |
| <code>interface.8.env-node-layerx-guarantor-publication-authorization-source</code> | overridden_by | override.J40.706 |
| <code>interface.8.env-node-layerx-guarantor-publication-inputs-dir</code> | overridden_by | override.J40.704 |
| <code>interface.8.env-node-layerx-guarantor-settlement-env</code> | overridden_by | override.J40.702 |
| <code>interface.8.env-node-layerx-guarantor-state-dir</code> | overridden_by | override.J40.701 |
| <code>interface.8.env-node-layerx-guarantor-submitter-key-file</code> | overridden_by | override.J40.703 |
| <code>interface.8.env-node-layerx-node-genesis-manifest</code> | overridden_by | override.J40.710 |
| <code>interface.8.env-node-layerx-node-genesis-registration</code> | overridden_by | override.J40.711 |
| <code>interface.8.env-node-layerx-node-handover-activity</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-node-layerx-node-identities</code> | overridden_by | override.J40.714 |
| <code>interface.8.env-node-layerx-node-settlement-wait-seconds</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-node-layerx-node-snapshot</code> | overridden_by | override.J40.713 |
| <code>interface.8.env-node-layerx-node-treasury-signer-wait-seconds</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-node-pythonpath</code> | overridden_by | override.J40.716 |
| <code>interface.8.env-node-role</code> | notes | 113 callers total |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.190.1</code> | req | 8190.1 |
| <code>acceptance.8.190.1</code> | scenario | Given the node deployment manifest, when it is parsed and every container and environment entry is listed, then no guarantor or boundary LNI entry remains and the core socket is present |
| <code>acceptance.8.190.1</code> | test | services/platform/hosted/node/tests/test_node_without_guarantor.py::test_node_deployment_runs_no_guarantor (planned) |
| <code>acceptance.8.190.1</code> | qualification | UNRUN |
| <code>req.8190</code> | ac_1 | The layerx-node StatefulSet has no container or init container named guarantor-*, no LAYERX_GUARANTOR_* variable and no authority or agent boundary LNI socket, while core-boundary keeps LAYERX_CORE_LNI_SOCKET. |
| <code>acceptance.8.217.1</code> | req | 8217.1 |
| <code>acceptance.8.217.1</code> | scenario | Given the hosted tree, when the four paths are checked and every file is searched for the two script names, then nothing is found |
| <code>acceptance.8.217.1</code> | test | services/platform/hosted/tests/test_retired_guarantor_scripts.py::test_hosted_tree_ships_no_guarantor_script (planned) |
| <code>acceptance.8.217.1</code> | qualification | UNRUN |
| <code>req.8217</code> | ac_1 | guarantor.sh, checkpoint-authority.py, guarantor-provision.sh and publication-authorization-test.py are absent and no file under services/platform/hosted names guarantor.sh or checkpoint-authority.py. |
| <code>acceptance.8.217.2</code> | req | 8217.2 |
| <code>acceptance.8.217.2</code> | scenario | Given an empty directory, when publication-policy.py runs the authorization and then the treasury operation, then the first is refused without output and the second writes a version 1 policy |
| <code>acceptance.8.217.2</code> | test | services/platform/hosted/tests/test_retired_guarantor_scripts.py::test_publication_policy_refuses_the_authorization_operation (planned) |
| <code>acceptance.8.217.2</code> | qualification | UNRUN |
| <code>req.8217</code> | ac_2 | publication-policy.py authorization with ten arguments exits non-zero with publication policy creation refused and writes no file, while the treasury operation still writes its policy. |
| <code>acceptance.8.218.1</code> | req | 8218.1 |
| <code>acceptance.8.218.1</code> | scenario | Given the node tests directory, when the four drivers are checked and the remaining files are searched for the three retired names, then nothing is found |
| <code>acceptance.8.218.1</code> | test | services/platform/hosted/node/tests/test_retired_node_drivers.py::test_kernel_node_drivers_are_absent (planned) |
| <code>acceptance.8.218.1</code> | qualification | UNRUN |
| <code>req.8218</code> | ac_1 | reset_recovery.py, node-test.sh, bootstrap-test.py and node-artifacts.py are absent from the node tests and no remaining node test names checkpoint-settlement.json, layerx-custody-proof or tests/bridge. |
| <code>capability.8.18.1</code> | title | Run the hosted node without guarantors and kernel boundary sockets |
| <code>capability.8.18.1</code> | reqs | 8190.1 |
| <code>capability.8.19.1</code> | title | Delete the hosted guarantor launcher and its checkpoint authority helper |
| <code>capability.8.19.1</code> | reqs | 8217.1<br>8217.2 |
| <code>capability.8.19.2</code> | title | Delete the kernel node regression drivers |
| <code>capability.8.19.2</code> | reqs | 8218.1 |
| <code>resolution.17.814</code> | title | decision.819 binds: The hosted node launcher stays until a merged-chain pod replaces it |
| <code>resolution.17.814</code> | applies_to | decision.819<br>task.8.19.1<br>task.8.19.2<br>task.8.19.3 |
| <code>resolution.17.814</code> | source | decision.819 in fragments/J40/decision.kvx; tasks from affects |

## node-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-node-binding-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:210</code> | <code>return {'error': {'code': 'binding_refused', 'retry': 'never'}}</code> |
| <code>interface.8.err-node-digest-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:223</code> | <code>return {'error': {'code': 'digest_refused', 'retry': 'never'}}</code> |
| <code>interface.8.err-node-generation-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/node/generation_transport.py:247</code> | <code>connection.sendall(canonical({'error': {'code': 'generation_unavailable'}}) + b'\n')</code> |
| <code>interface.8.err-node-peer-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:171</code> | <code>self.reply({'error': {'code': 'peer_refused', 'retry': 'never'}})</code> |
| <code>interface.8.err-node-signature-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:227</code> | <code>return {'error': {'code': 'signature_refused', 'retry': 'never'}}</code> |
| <code>interface.8.err-node-unknown-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/node/signer/signer.py:39</code> | <code>#   anything else    -&gt; {"error":{"code":"unknown_request","retry":"never"}}</code> |

## paxai-storage-keeper-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-paxai-storage-keeper-keeper-log</code> | <code>file_format</code> | <code>retire</code> | <code>services/platform/hosted/paxai-storage-keeper/src/store.rs:15</code> | <code>const MAGIC: &amp;[u8; 8] = b"LXKEEPR1";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-paxai-storage-keeper-keeper-log</code> | notes | capacity keeper log magic; stale tasks 4.7.4 (spec/layerx-merge/spec.kvx:16625) and 11.7.38 (spec/layerx-merge/spec.kvx:64619) retire the storage keeper; present at HEAD; read and written only by the defining module |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.188.1</code> | req | 8188.1 |
| <code>acceptance.8.188.1</code> | scenario | Given the hosted tree, when it is listed and every Cargo.toml under services/platform is searched for the keeper name, then nothing is found |
| <code>acceptance.8.188.1</code> | test | services/platform/hosted/tests/test_retired_storage_keeper.py::test_storage_keeper_crate_is_absent (planned) |
| <code>acceptance.8.188.1</code> | qualification | UNRUN |
| <code>req.8188</code> | ac_1 | services/platform/hosted/paxai-storage-keeper is absent and no Cargo.toml under services/platform names paxai-storage-keeper. |
| <code>capability.8.17.1</code> | title | Delete the storage keeper crate |
| <code>capability.8.17.1</code> | reqs | 8188.1 |
| <code>resolution.17.813</code> | title | decision.818 binds: The storage keeper and the hosted guarantors retire with the kernel checkpoint path |
| <code>resolution.17.813</code> | applies_to | decision.818<br>task.8.17.1<br>task.8.17.2<br>task.8.18.1<br>task.8.19.1 |
| <code>resolution.17.813</code> | source | decision.818 in fragments/J40/decision.kvx; tasks from affects |

## paxai-worker-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-paxai-worker-main-rs-config</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:239</code> | <code>let (Some("--config"), Some(path), None) = (args.next().as_deref(), args.next(), args.next())</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-paxai-worker-main-rs-config</code> | notes | no in-repo invocation outside the defining file |

## paxai-worker-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-paxai-worker-config-certificate-chain</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:41</code> | <code>certificate_chain: PathBuf,</code> |
| <code>interface.8.cfg-paxai-worker-config-chain</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:45</code> | <code>chain: String,</code> |
| <code>interface.8.cfg-paxai-worker-config-delegate-seed</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:43</code> | <code>delegate_seed: PathBuf,</code> |
| <code>interface.8.cfg-paxai-worker-config-listen</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:40</code> | <code>listen: String,</code> |
| <code>interface.8.cfg-paxai-worker-config-market</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:47</code> | <code>market: String,</code> |
| <code>interface.8.cfg-paxai-worker-config-owner</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:49</code> | <code>owner: String,</code> |
| <code>interface.8.cfg-paxai-worker-config-private-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:42</code> | <code>private_key: PathBuf,</code> |
| <code>interface.8.cfg-paxai-worker-config-program</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:46</code> | <code>program: String,</code> |
| <code>interface.8.cfg-paxai-worker-config-signed-metadata</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:44</code> | <code>signed_metadata: PathBuf,</code> |
| <code>interface.8.cfg-paxai-worker-config-worker</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/main.rs:48</code> | <code>worker: String,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-paxai-worker-config-certificate-chain</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-paxai-worker-config-chain</code> | notes | 451 callers total |
| <code>interface.8.cfg-paxai-worker-config-delegate-seed</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-paxai-worker-config-listen</code> | notes | 75 callers total |
| <code>interface.8.cfg-paxai-worker-config-market</code> | notes | 523 callers total |
| <code>interface.8.cfg-paxai-worker-config-owner</code> | notes | 1260 callers total |
| <code>interface.8.cfg-paxai-worker-config-program</code> | notes | 120 callers total |
| <code>interface.8.cfg-paxai-worker-config-signed-metadata</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-paxai-worker-config-worker</code> | notes | 65 callers total |

## paxai-worker-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-paxai-worker-paxai-runner-models</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/bin/paxai-runner.rs:34</code> | <code>const MODELS_ENV: &amp;str = "PAXAI_RUNNER_MODELS";</code> |
| <code>interface.8.env-paxai-worker-paxai-runner-state</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/bin/paxai-runner.rs:33</code> | <code>const STATE_ENV: &amp;str = "PAXAI_RUNNER_STATE";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-paxai-worker-paxai-runner-models</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxai-worker-paxai-runner-state</code> | notes | no in-repo setter or reader outside the defining file |

## paxai-worker-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-paxai-worker-manifest-schema</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/metadata.rs:10</code> | <code>pub const MANIFEST_SCHEMA: u16 = 1;</code> |
| <code>interface.8.fmt-paxai-worker-membership</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/membership.rs:28</code> | <code>const MAGIC: &amp;[u8; 8] = b"PXMEMB01";</code> |
| <code>interface.8.fmt-paxai-worker-runner</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/runner.rs:31</code> | <code>pub const MAGIC: &amp;[u8; 7] = b"PAXRUN1";</code> |
| <code>interface.8.fmt-paxai-worker-service-auth</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/auth.rs:34</code> | <code>pub const SERVICE_MAGIC: &amp;[u8; 7] = b"PAXAIS1";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-paxai-worker-manifest-schema</code> | notes | worker manifest schema version; read and written only by the defining module |
| <code>interface.8.fmt-paxai-worker-membership</code> | notes | membership record magic; read and written only by the defining module |
| <code>interface.8.fmt-paxai-worker-runner</code> | notes | runner frame magic |
| <code>interface.8.fmt-paxai-worker-service-auth</code> | notes | service authentication frame magic |

## paxai-worker-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-paxai-worker-any-paxai-v1</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/paxai-worker/src/discovery.rs:707</code> | <code>if !path.starts_with("/paxai/v1/") &#124;&#124; body.len() &gt; MAX_ENVELOPE_BYTES {</code> |

## paxeer-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-paxeer-anchor-genesis-py-anchor</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:117</code> | <code>parser.add_argument('--anchor', type=anchor, default=dict(set=False, batch_number=0, last_sequence=0, state_root='0' * 64))</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-authority</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:101</code> | <code>authority.add_argument('--authority')</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-authority-evm</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:102</code> | <code>authority.add_argument('--authority-evm')</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-bond-denom</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:107</code> | <code>parser.add_argument('--bond-denom', default='uhpx')</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-challenge-bond</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:108</code> | <code>parser.add_argument('--challenge-bond', type=int, default=1_000_000)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-challenge-window-seconds</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:109</code> | <code>parser.add_argument('--challenge-window-seconds', type=int, default=0)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-first-batch</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:114</code> | <code>parser.add_argument('--first-batch', type=int, default=1)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-guarantor</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:116</code> | <code>parser.add_argument('--guarantor', action='append', default=[], type=guarantor)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-last-batch</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:115</code> | <code>parser.add_argument('--last-batch', type=int, default=OPEN_ENDED_BATCH)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-max-attestation-delay-ms</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:111</code> | <code>parser.add_argument('--max-attestation-delay-ms', type=int, default=3_600_000)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-min-bond</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:106</code> | <code>parser.add_argument('--min-bond', type=int, default=1_000_000)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-network-id</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:104</code> | <code>parser.add_argument('--network-id', type=int, required=True)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-output</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:119</code> | <code>parser.add_argument('--output', required=True)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-paxeer-chain-id</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:103</code> | <code>parser.add_argument('--paxeer-chain-id', type=int, required=True)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-permissionless-registration</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:118</code> | <code>parser.add_argument('--permissionless-registration', action='store_true')</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-sequencer-id</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:112</code> | <code>parser.add_argument('--sequencer-id', required=True)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-sequencer-public-key</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:113</code> | <code>parser.add_argument('--sequencer-public-key', required=True)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-threshold</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:105</code> | <code>parser.add_argument('--threshold', type=int, required=True)</code> |
| <code>interface.8.cli-paxeer-anchor-genesis-py-unbonding-delay-seconds</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-genesis.py:110</code> | <code>parser.add_argument('--unbonding-delay-seconds', type=int, default=21 * 24 * 60 * 60)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:64</code> | <code>parser.add_argument('--asset', action='append', required=True, type=asset)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-authority</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:62</code> | <code>parser.add_argument('--authority', default='')</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-checkpoint</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:65</code> | <code>parser.add_argument('--checkpoint', action='append', default=[], type=checkpoint)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-deposit-root-authority</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:63</code> | <code>parser.add_argument('--deposit-root-authority', required=True)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-first-batch</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:57</code> | <code>parser.add_argument('--first-batch', type=int, default=1)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-forced-exit-delay-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:60</code> | <code>parser.add_argument('--forced-exit-delay-seconds', type=int, default=0)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-last-batch</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:58</code> | <code>parser.add_argument('--last-batch', type=int, default=1 &lt;&lt; 40)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-liveness-bound-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:61</code> | <code>parser.add_argument('--liveness-bound-seconds', type=int, default=86400)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-network-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:54</code> | <code>parser.add_argument('--network-id', type=int, required=True)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-output</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:66</code> | <code>parser.add_argument('--output', required=True)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-sequencer-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:55</code> | <code>parser.add_argument('--sequencer-id', required=True)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-sequencer-public-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:56</code> | <code>parser.add_argument('--sequencer-public-key', required=True)</code> |
| <code>interface.8.cli-paxeer-custody-genesis-py-withdrawal-delay-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/custody-genesis.py:59</code> | <code>parser.add_argument('--withdrawal-delay-seconds', type=int, default=3600)</code> |
| <code>interface.8.cli-paxeer-evm-py-ca</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/evm.py:425</code> | <code>options, rest = _options(arguments[1:], ('--rpc', '--ca', '--chain', '--key-file', '--value', '--timeout'))</code> |
| <code>interface.8.cli-paxeer-evm-py-chain</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/evm.py:425</code> | <code>options, rest = _options(arguments[1:], ('--rpc', '--ca', '--chain', '--key-file', '--value', '--timeout'))</code> |
| <code>interface.8.cli-paxeer-evm-py-key-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/evm.py:425</code> | <code>options, rest = _options(arguments[1:], ('--rpc', '--ca', '--chain', '--key-file', '--value', '--timeout'))</code> |
| <code>interface.8.cli-paxeer-evm-py-rpc</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/evm.py:425</code> | <code>options, rest = _options(arguments[1:], ('--rpc', '--ca', '--chain', '--key-file', '--value', '--timeout'))</code> |
| <code>interface.8.cli-paxeer-evm-py-timeout</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/evm.py:425</code> | <code>options, rest = _options(arguments[1:], ('--rpc', '--ca', '--chain', '--key-file', '--value', '--timeout'))</code> |
| <code>interface.8.cli-paxeer-evm-py-value</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/evm.py:425</code> | <code>options, rest = _options(arguments[1:], ('--rpc', '--ca', '--chain', '--key-file', '--value', '--timeout'))</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-chain-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:163</code> | <code>--chain-id) need_value "$@"; COSMOS_CHAIN_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-check</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:186</code> | <code>--check) CHECK=1; shift ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-env-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:161</code> | <code>--env-file) need_value "$@"; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-evm-chain-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:164</code> | <code>--evm-chain-id) need_value "$@"; EVM_CHAIN_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-fees</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:173</code> | <code>--fees) need_value "$@"; FEES=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-gas</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:174</code> | <code>--gas) need_value "$@"; GAS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-module</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:176</code> | <code>--module)</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-new-bin</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:168</code> | <code>--new-bin) need_value "$@"; NEW_BIN=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-old-bin</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:167</code> | <code>--old-bin) need_value "$@"; OLD_BIN=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-port-base</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:171</code> | <code>--port-base) need_value "$@"; PORT_BASE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-source-data</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:165</code> | <code>--source-data) need_value "$@"; SOURCE_DATA=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-source-genesis</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:166</code> | <code>--source-genesis) need_value "$@"; SOURCE_GENESIS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-upgrade-height-offset</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:170</code> | <code>--upgrade-height-offset) need_value "$@"; OFFSET=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-upgrade-name</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:162</code> | <code>--upgrade-name) need_value "$@"; UPGRADE_NAME=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-view</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:180</code> | <code>--view)</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-voting-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:172</code> | <code>--voting-seconds) need_value "$@"; VOTING_SECONDS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-wait-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:175</code> | <code>--wait-seconds) need_value "$@"; WAIT_SECONDS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-work</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:169</code> | <code>--work) need_value "$@"; WORK=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-bin</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:103</code> | <code>--bin) need_value "$@"; PAXD_BIN=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-bypass-commit-timeout</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:84</code> | <code>--bypass-commit-timeout) need_value "$@"; BYPASS_COMMIT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-chain-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:98</code> | <code>--chain-id) need_value "$@"; COSMOS_CHAIN_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-deposit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:89</code> | <code>--deposit) need_value "$@"; DEPOSIT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-description</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:92</code> | <code>--description) need_value "$@"; DESCRIPTION=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-execute</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:104</code> | <code>--execute) EXECUTE=1; shift ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-expedited</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:90</code> | <code>--expedited) EXPEDITED=1; shift ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-fees</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:99</code> | <code>--fees) need_value "$@"; FEES=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-gas</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:100</code> | <code>--gas) need_value "$@"; GAS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-home</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:102</code> | <code>--home) need_value "$@"; HOME_DIR=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:82</code> | <code>--key) need_value "$@"; KEY_NAME=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-keyring-backend</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:101</code> | <code>--keyring-backend) need_value "$@"; KEYRING_BACKEND=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-margin-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:88</code> | <code>--margin-seconds) need_value "$@"; MARGIN_SECONDS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-now</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:97</code> | <code>--now) need_value "$@"; NOW_OVERRIDE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-proposal-file</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:93</code> | <code>--proposal-file) need_value "$@"; PROPOSAL_FILE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-proposal-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:95</code> | <code>--proposal-id) need_value "$@"; PROPOSAL_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-rpc</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:81</code> | <code>--rpc) need_value "$@"; RPC=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-seconds-per-block</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:87</code> | <code>--seconds-per-block) need_value "$@"; SPB_OVERRIDE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-target-utc</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:86</code> | <code>--target-utc) need_value "$@"; TARGET_UTC=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-timeout-commit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:83</code> | <code>--timeout-commit) need_value "$@"; TIMEOUT_COMMIT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-timeout-vote</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:85</code> | <code>--timeout-vote) need_value "$@"; TIMEOUT_VOTE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-title</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:91</code> | <code>--title) need_value "$@"; TITLE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-voter</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:94</code> | <code>--voter) need_value "$@"; VOTERS+=("$2"); shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-wait-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-consensus-params.sh:96</code> | <code>--wait-seconds) need_value "$@"; WAIT_SECONDS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-anchor</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:105</code> | <code>--anchor) need_value "$@"; ANCHOR=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-bin</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:115</code> | <code>--bin) need_value "$@"; PAXD_BIN=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-chain-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:110</code> | <code>--chain-id) need_value "$@"; COSMOS_CHAIN_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-deposit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:98</code> | <code>--deposit) need_value "$@"; DEPOSIT=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-description</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:97</code> | <code>--description) need_value "$@"; DESCRIPTION=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-execute</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:116</code> | <code>--execute) EXECUTE=1; shift ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-expedited</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:99</code> | <code>--expedited) EXPEDITED=1; shift ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-fees</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:111</code> | <code>--fees) need_value "$@"; FEES=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-gas</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:112</code> | <code>--gas) need_value "$@"; GAS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-halt-height</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:106</code> | <code>--halt-height) need_value "$@"; HALT_HEIGHT_OVERRIDE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-home</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:114</code> | <code>--home) need_value "$@"; HOME_DIR=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:92</code> | <code>--key) need_value "$@"; KEY_NAME=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-keyring-backend</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:113</code> | <code>--keyring-backend) need_value "$@"; KEYRING_BACKEND=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-lookback-blocks</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:104</code> | <code>--lookback-blocks) need_value "$@"; LOOKBACK_BLOCKS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-margin-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:101</code> | <code>--margin-seconds) need_value "$@"; MARGIN_SECONDS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-measure-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:102</code> | <code>--measure-seconds) need_value "$@"; MEASURE_SECONDS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-now</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:107</code> | <code>--now) need_value "$@"; NOW_OVERRIDE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-proposal-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:108</code> | <code>--proposal-id) need_value "$@"; PROPOSAL_ID=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-rpc</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:91</code> | <code>--rpc) need_value "$@"; RPC=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-seconds-per-block</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:103</code> | <code>--seconds-per-block) need_value "$@"; SPB_OVERRIDE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-target-utc</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:93</code> | <code>--target-utc) need_value "$@"; TARGET_UTC=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-title</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:96</code> | <code>--title) need_value "$@"; TITLE=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-upgrade-info</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:95</code> | <code>--upgrade-info) need_value "$@"; UPGRADE_INFO=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-upgrade-name</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:94</code> | <code>--upgrade-name) need_value "$@"; UPGRADE_NAME=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-voter</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:100</code> | <code>--voter) need_value "$@"; VOTERS+=("$2"); shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-wait-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-upgrade.sh:109</code> | <code>--wait-seconds) need_value "$@"; WAIT_SECONDS=$2; shift 2 ;;</code> |
| <code>interface.8.cli-paxeer-prepare-beta-py-asset-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/prepare-beta.py:90</code> | <code>parser.add_argument('--asset-id', default=BETA_ASSET_ID)</code> |
| <code>interface.8.cli-paxeer-prepare-beta-py-genesis-metadata</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/prepare-beta.py:91</code> | <code>parser.add_argument('--genesis-metadata', type=Path, required=True)</code> |
| <code>interface.8.cli-paxeer-prepare-beta-py-module-fees</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/prepare-beta.py:93</code> | <code>parser.add_argument('--module-fees', type=Path, default=ROOT.parent / 'node/genesis-module-fees.json')</code> |
| <code>interface.8.cli-paxeer-prepare-beta-py-network-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/prepare-beta.py:89</code> | <code>parser.add_argument('--network-id', type=int, default=402)</code> |
| <code>interface.8.cli-paxeer-prepare-beta-py-timestamp-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/prepare-beta.py:94</code> | <code>parser.add_argument('--timestamp-ms', type=int, default=int(time.time() * 1000))</code> |
| <code>interface.8.cli-paxeer-prepare-beta-py-withdrawal-fee</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/prepare-beta.py:92</code> | <code>parser.add_argument('--withdrawal-fee', type=int, default=0)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-paxeer-anchor-genesis-py-anchor</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-anchor</code> | overridden_by | override.J40.126 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-authority</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-authority</code> | overridden_by | override.J40.110 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-authority-evm</code> | overridden_by | override.J40.111 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-bond-denom</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-bond-denom</code> | overridden_by | override.J40.116 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-challenge-bond</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-challenge-bond</code> | overridden_by | override.J40.117 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-challenge-window-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-challenge-window-seconds</code> | overridden_by | override.J40.118 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-first-batch</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-first-batch</code> | overridden_by | override.J40.123 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-guarantor</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-guarantor</code> | overridden_by | override.J40.125 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-last-batch</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-last-batch</code> | overridden_by | override.J40.124 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-max-attestation-delay-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-max-attestation-delay-ms</code> | overridden_by | override.J40.120 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-min-bond</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-min-bond</code> | overridden_by | override.J40.115 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-network-id</code> | overridden_by | override.J40.113 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-output</code> | overridden_by | override.J40.128 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-paxeer-chain-id</code> | overridden_by | override.J40.112 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-permissionless-registration</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-permissionless-registration</code> | overridden_by | override.J40.127 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-sequencer-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-sequencer-id</code> | overridden_by | override.J40.121 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-sequencer-public-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-sequencer-public-key</code> | overridden_by | override.J40.122 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-threshold</code> | overridden_by | override.J40.114 |
| <code>interface.8.cli-paxeer-anchor-genesis-py-unbonding-delay-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-anchor-genesis-py-unbonding-delay-seconds</code> | overridden_by | override.J40.119 |
| <code>interface.8.cli-paxeer-custody-genesis-py-authority</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-custody-genesis-py-checkpoint</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-custody-genesis-py-first-batch</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-custody-genesis-py-forced-exit-delay-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-custody-genesis-py-last-batch</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-custody-genesis-py-liveness-bound-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-custody-genesis-py-sequencer-public-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-custody-genesis-py-withdrawal-delay-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-evm-py-ca</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-evm-py-timeout</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-chain-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-check</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-env-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-evm-chain-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-fees</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-gas</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-module</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-new-bin</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-old-bin</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-port-base</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-source-data</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-source-genesis</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-upgrade-height-offset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-upgrade-name</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-view</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-voting-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-wait-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-fork-rehearsal-sh-work</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-bin</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-bypass-commit-timeout</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-chain-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-deposit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-description</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-execute</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-expedited</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-fees</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-gas</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-home</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-keyring-backend</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-margin-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-now</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-proposal-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-proposal-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-rpc</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-seconds-per-block</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-target-utc</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-timeout-commit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-timeout-vote</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-title</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-voter</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-consensus-params-sh-wait-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-anchor</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-bin</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-chain-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-deposit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-description</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-execute</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-expedited</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-fees</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-gas</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-halt-height</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-home</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-keyring-backend</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-lookback-blocks</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-margin-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-measure-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-now</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-proposal-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-rpc</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-seconds-per-block</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-target-utc</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-title</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-upgrade-info</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-upgrade-name</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-voter</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-gov-upgrade-sh-wait-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-prepare-beta-py-asset-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-prepare-beta-py-genesis-metadata</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-prepare-beta-py-module-fees</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-prepare-beta-py-network-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-prepare-beta-py-timestamp-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-paxeer-prepare-beta-py-withdrawal-fee</code> | notes | no in-repo invocation outside the defining file |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.148.1</code> | req | 8148.1 |
| <code>acceptance.8.148.1</code> | scenario | Given the paxeer deployment directory and the beta cluster script, when they are listed and searched for the two script names and paxeer-anchor-genesis, then nothing is found |
| <code>acceptance.8.148.1</code> | test | services/platform/hosted/tests/test_retired_kernel_scripts.py::test_paxeer_deployment_ships_no_anchor_genesis (planned) |
| <code>acceptance.8.148.1</code> | qualification | UNRUN |
| <code>req.8148</code> | ac_1 | anchor-genesis.py and anchor-guarantors.sh are absent from the paxeer deployment and neither the deployment nor the beta cluster script names them or the anchor genesis file. |
| <code>capability.8.14.2</code> | title | Remove the anchor genesis and guarantor registration of the paxeer deployment |
| <code>capability.8.14.2</code> | reqs | 8148.1 |

## paxeer-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-paxeer-bash-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-lib.sh:15</code> | <code>[ -n "${BASH_VERSION:-}" ] &#124;&#124; { echo "gov-lib.sh must be sourced from bash" &gt;&amp;2; exit 1; }</code> |
| <code>interface.8.env-paxeer-gov-fixture-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-lib.sh:18</code> | <code>GOV_FIXTURE_DIR=${GOV_FIXTURE_DIR:-}</code> |
| <code>interface.8.env-paxeer-gov-tool</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-lib.sh:17</code> | <code>GOV_TOOL=${GOV_TOOL:-gov}</code> |
| <code>interface.8.env-paxeer-guarantors</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:58</code> | <code>[ -r "$GUARANTORS" ] &#124;&#124; fail "guarantor list ${GUARANTORS:-LAYERX_PAXEER_GUARANTORS} is not readable"</code> |
| <code>interface.8.env-paxeer-home-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-lib.sh:256</code> | <code>[ -z "${HOME_DIR:-}" ] &#124;&#124; printf -- ' --home %q' "$HOME_DIR"</code> |
| <code>interface.8.env-paxeer-jq</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:41</code> | <code>JQ=${JQ:-jq}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-anchor-genesis</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:43</code> | <code>ANCHOR_GENESIS=${LAYERX_PAXEER_ANCHOR_GENESIS:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-anchor-genesis-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:63</code> | <code>ANCHOR_GENESIS=${LAYERX_PAXEER_ANCHOR_GENESIS_FILE:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-boundary-ca-der</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:40</code> | <code>BOUNDARY_CA=${LAYERX_PAXEER_BOUNDARY_CA_DER:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-boundary-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:150</code> | <code>let listen = env::var("LAYERX_PAXEER_BOUNDARY_LISTEN")</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-boundary-tls-cert-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:134</code> | <code>let certificate_path = env::var("LAYERX_PAXEER_BOUNDARY_TLS_CERT_DER")</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-boundary-tls-key-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:136</code> | <code>let key_path = env::var("LAYERX_PAXEER_BOUNDARY_TLS_KEY_DER")</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-boundary-url</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:39</code> | <code>BOUNDARY_URL=${LAYERX_PAXEER_BOUNDARY_URL:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-chain-id</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:41</code> | <code>CHAIN_ID=${LAYERX_PAXEER_CHAIN_ID:-125}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-checkpoint-submitter-address</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:99</code> | <code>SUBMITTER_ADDRESS=${LAYERX_PAXEER_CHECKPOINT_SUBMITTER_ADDRESS:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-checkpoint-submitter-address-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:94</code> | <code>if [ -n "${LAYERX_PAXEER_CHECKPOINT_SUBMITTER_ADDRESS_FILE:-}" ]; then</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-checkpoint-submitter-funding</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:51</code> | <code>SUBMITTER_FUNDING=${LAYERX_PAXEER_CHECKPOINT_SUBMITTER_FUNDING:-1000000000000uhpx}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-checkpoint-submitter-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/deploy-contracts.sh:71</code> | <code>CHECKPOINT_SUBMITTER_KEY_FILE=${LAYERX_PAXEER_CHECKPOINT_SUBMITTER_KEY_FILE:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-comet-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:166</code> | <code>comet: env::var("LAYERX_PAXEER_COMET_URL").map_or_else(</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-commit-timeout-nanoseconds</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:61</code> | <code>COMMIT_TIMEOUT_NANOSECONDS=${LAYERX_PAXEER_COMMIT_TIMEOUT_NANOSECONDS:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-controller-gas-wei</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:49</code> | <code>GAS_WEI=${LAYERX_PAXEER_CONTROLLER_GAS_WEI:-1000000000000000000}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-custody-genesis-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:62</code> | <code>CUSTODY_GENESIS=${LAYERX_PAXEER_CUSTODY_GENESIS_FILE:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-deployer-address</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:86</code> | <code>DEPLOYER_ADDRESS=${LAYERX_PAXEER_DEPLOYER_ADDRESS:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-deployer-address-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:83</code> | <code>if [ -n "${LAYERX_PAXEER_DEPLOYER_ADDRESS_FILE:-}" ]; then</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-deployer-funding</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:49</code> | <code>DEPLOYER_FUNDING=${LAYERX_PAXEER_DEPLOYER_FUNDING:-1000000000000000000000000uhpx}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-deployer-key-file</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:42</code> | <code>DEPLOYER_KEY_FILE=${LAYERX_PAXEER_DEPLOYER_KEY_FILE:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-deployment-input</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/deploy-contracts.sh:73</code> | <code>INPUT_JSON=${LAYERX_PAXEER_DEPLOYMENT_INPUT:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-deployment-record</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/deploy-contracts.sh:76</code> | <code>RECORD=${LAYERX_PAXEER_DEPLOYMENT_RECORD:-$SCRIPT_DIR/deployments/beta.json}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-deposit-root-authority</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:64</code> | <code>DEPOSIT_ROOT_AUTHORITY=${LAYERX_PAXEER_DEPOSIT_ROOT_AUTHORITY:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-deposit-root-authority-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:114</code> | <code>if [ -n "${LAYERX_PAXEER_DEPOSIT_ROOT_AUTHORITY_FILE:-}" ]; then</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-evm-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:55</code> | <code>EVM_PORT=${LAYERX_PAXEER_EVM_PORT:-8545}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-evm-ws-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:56</code> | <code>EVM_WS_PORT=${LAYERX_PAXEER_EVM_WS_PORT:-8546}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-foundry-bin</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/deploy-contracts.sh:61</code> | <code>FOUNDRY_BIN=${LAYERX_PAXEER_FOUNDRY_BIN:-/root/.foundry/bin}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-genesis-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/deploy-contracts.sh:72</code> | <code>GENESIS_DIR=${LAYERX_PAXEER_GENESIS_DIR:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-grpc-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:59</code> | <code>GRPC_PORT=${LAYERX_PAXEER_GRPC_PORT:-9090}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-grpc-web-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:60</code> | <code>GRPC_WEB_PORT=${LAYERX_PAXEER_GRPC_WEB_PORT:-9091}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-guarantor-keys-dir</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:45</code> | <code>KEYS_DIR=${LAYERX_PAXEER_GUARANTOR_KEYS_DIR:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-guarantors</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:44</code> | <code>GUARANTORS=${LAYERX_PAXEER_GUARANTORS:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-home</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:42</code> | <code>HOME_DIR=${LAYERX_PAXEER_HOME:-/var/lib/paxeer}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-moniker</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:44</code> | <code>MONIKER=${LAYERX_PAXEER_MONIKER:-paxeer-beta}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-node-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:162</code> | <code>&amp;env::var("LAYERX_PAXEER_NODE_URL")</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-p2p-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:58</code> | <code>P2P_PORT=${LAYERX_PAXEER_P2P_PORT:-26656}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-protocol-version</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:48</code> | <code>PROTOCOL_VERSION=${LAYERX_PAXEER_PROTOCOL_VERSION:-3}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-rpc-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:57</code> | <code>RPC_PORT=${LAYERX_PAXEER_RPC_PORT:-26657}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-settlement-domain</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:47</code> | <code>DOMAIN=${LAYERX_PAXEER_SETTLEMENT_DOMAIN:-beta}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-settlement-json</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/anchor-guarantors.sh:46</code> | <code>SETTLEMENT_JSON=${LAYERX_PAXEER_SETTLEMENT_JSON:-}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-usdl-runtime</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:54</code> | <code>USDL_RUNTIME=${LAYERX_PAXEER_USDL_RUNTIME:-$SCRIPT_DIR/contracts/BetaUsdl.runtime.hex}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-validator-funding</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:46</code> | <code>VALIDATOR_FUNDING=${LAYERX_PAXEER_VALIDATOR_FUNDING:-100000000000000000000uhpx}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-validator-key-name</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:45</code> | <code>VALIDATOR_KEY=${LAYERX_PAXEER_VALIDATOR_KEY_NAME:-validator}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-validator-power</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:48</code> | <code>VALIDATOR_POWER=${LAYERX_PAXEER_VALIDATOR_POWER:-7000000000}</code> |
| <code>interface.8.env-paxeer-layerx-paxeer-validator-stake</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:47</code> | <code>VALIDATOR_STAKE=${LAYERX_PAXEER_VALIDATOR_STAKE:-7000000000000000uhpx}</code> |
| <code>interface.8.env-paxeer-paxd</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/init-chain.sh:40</code> | <code>PAXD=${PAXD:-paxd}</code> |
| <code>interface.8.env-paxeer-paxd-bin</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-lib.sh:19</code> | <code>PAXD_BIN=${PAXD_BIN:-paxd}</code> |
| <code>interface.8.env-paxeer-paxeer-fork-env</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/fork-rehearsal.sh:40</code> | <code>ENV_FILE=${PAXEER_FORK_ENV:-}</code> |
| <code>interface.8.env-paxeer-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/gov-lib.sh:20</code> | <code>RPC=${RPC:-}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-paxeer-bash-version</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-guarantors</code> | overridden_by | override.J40.140 |
| <code>interface.8.env-paxeer-home-dir</code> | notes | 73 callers total |
| <code>interface.8.env-paxeer-jq</code> | notes | 105 callers total |
| <code>interface.8.env-paxeer-layerx-paxeer-anchor-genesis</code> | overridden_by | override.J40.133 |
| <code>interface.8.env-paxeer-layerx-paxeer-boundary-ca-der</code> | overridden_by | override.J40.130 |
| <code>interface.8.env-paxeer-layerx-paxeer-boundary-url</code> | overridden_by | override.J40.129 |
| <code>interface.8.env-paxeer-layerx-paxeer-chain-id</code> | notes | 27 callers total |
| <code>interface.8.env-paxeer-layerx-paxeer-chain-id</code> | overridden_by | override.J40.131 |
| <code>interface.8.env-paxeer-layerx-paxeer-checkpoint-submitter-address-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-checkpoint-submitter-funding</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-checkpoint-submitter-key-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-controller-gas-wei</code> | overridden_by | override.J40.139 |
| <code>interface.8.env-paxeer-layerx-paxeer-deployer-funding</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-deployer-key-file</code> | overridden_by | override.J40.132 |
| <code>interface.8.env-paxeer-layerx-paxeer-deposit-root-authority-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-foundry-bin</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-guarantor-keys-dir</code> | overridden_by | override.J40.135 |
| <code>interface.8.env-paxeer-layerx-paxeer-guarantors</code> | overridden_by | override.J40.134 |
| <code>interface.8.env-paxeer-layerx-paxeer-moniker</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-protocol-version</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-protocol-version</code> | overridden_by | override.J40.138 |
| <code>interface.8.env-paxeer-layerx-paxeer-settlement-domain</code> | overridden_by | override.J40.137 |
| <code>interface.8.env-paxeer-layerx-paxeer-settlement-json</code> | overridden_by | override.J40.136 |
| <code>interface.8.env-paxeer-layerx-paxeer-validator-funding</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-validator-key-name</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-validator-power</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-layerx-paxeer-validator-stake</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-paxeer-paxd</code> | notes | 83 callers total |
| <code>interface.8.env-paxeer-paxd-bin</code> | notes | 41 callers total |
| <code>interface.8.env-paxeer-rpc</code> | notes | 4186 callers total; no in-repo setter or reader outside the defining file |

## paxeer-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-paxeer-32600</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:90</code> | <code>return Err(rpc_error(&amp;Value::Null, -32600, "invalid Comet request"));</code> |
| <code>interface.8.err-paxeer-32601</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:427</code> | <code>return rpc_error(&amp;id, -32601, "method is not relayed by the boundary");</code> |
| <code>interface.8.err-paxeer-32602</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:101</code> | <code>let invalid = &#124;&#124; rpc_error(&amp;id, -32602, "invalid Comet parameters");</code> |
| <code>interface.8.err-paxeer-batch-not-supported</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:409</code> | <code>return refusal(400, "batch_not_supported", None);</code> |
| <code>interface.8.err-paxeer-body-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:82</code> | <code>return Err(refusal(413, "body_too_large", None));</code> |
| <code>interface.8.err-paxeer-chain-id-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:473</code> | <code>Ok(_) =&gt; refusal(503, "chain_id_mismatch", Some(10)),</code> |
| <code>interface.8.err-paxeer-comet-evidence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:408</code> | <code>Err(Failure::Unavailable) =&gt; refusal(503, "comet_evidence_unavailable", Some(5)),</code> |
| <code>interface.8.err-paxeer-comet-proof-unsupported</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:409</code> | <code>Err(Failure::ProofUnsupported) =&gt; refusal(502, "comet_proof_unsupported", None),</code> |
| <code>interface.8.err-paxeer-comet-response-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:407</code> | <code>Err(Failure::Node(NodeFailure::Invalid)) =&gt; refusal(502, "comet_response_invalid", Some(5)),</code> |
| <code>interface.8.err-paxeer-comet-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:406</code> | <code>Err(Failure::Node(NodeFailure::Unreachable)) =&gt; refusal(503, "comet_unavailable", Some(5)),</code> |
| <code>interface.8.err-paxeer-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:393</code> | <code>return refusal(400, "content_type_required", None);</code> |
| <code>interface.8.err-paxeer-invalid-json</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:406</code> | <code>return refusal(400, "invalid_json", None);</code> |
| <code>interface.8.err-paxeer-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:570</code> | <code>&#124;_&#124; refusal(400, "invalid_request", None),</code> |
| <code>interface.8.err-paxeer-node-response-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:443</code> | <code>_ =&gt; refusal(502, "node_response_invalid", Some(5)),</code> |
| <code>interface.8.err-paxeer-node-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:446</code> | <code>Err(NodeFailure::Unreachable) =&gt; refusal(503, "node_unavailable", Some(5)),</code> |
| <code>interface.8.err-paxeer-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:490</code> | <code>.map_or_else(&#124;&#124; refusal(404, "not_found", None), genesis::response);</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-paxeer-32600</code> | notes | 38 callers total |
| <code>interface.8.err-paxeer-32601</code> | notes | 116 callers total |
| <code>interface.8.err-paxeer-32602</code> | notes | 444 callers total |
| <code>interface.8.err-paxeer-batch-not-supported</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-paxeer-body-too-large</code> | notes | HTTP 413 |
| <code>interface.8.err-paxeer-chain-id-mismatch</code> | notes | HTTP 503 |
| <code>interface.8.err-paxeer-comet-evidence-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-paxeer-comet-proof-unsupported</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-paxeer-comet-response-invalid</code> | notes | HTTP 502 |
| <code>interface.8.err-paxeer-comet-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-paxeer-content-type-required</code> | notes | HTTP 400 |
| <code>interface.8.err-paxeer-invalid-json</code> | notes | HTTP 400 |
| <code>interface.8.err-paxeer-invalid-request</code> | notes | HTTP 400; 78 callers total |
| <code>interface.8.err-paxeer-node-response-invalid</code> | notes | HTTP 502; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-paxeer-node-unavailable</code> | notes | HTTP 503; 71 callers total |
| <code>interface.8.err-paxeer-not-found</code> | notes | HTTP 404; 134 callers total |

## paxeer-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-paxeer-any-store-evm-key</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/comet.rs:155</code> | <code>if params.path != "/store/evm/key"</code> |
| <code>interface.8.http-paxeer-any-store-layerxcustody-key</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/hosted/paxeer/src/comet.rs:139</code> | <code>if params.path == "/store/layerxcustody/key" {</code> |
| <code>interface.8.http-paxeer-get-genesis</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:486</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/genesis" {</code> |
| <code>interface.8.http-paxeer-get-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:480</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/livez" {</code> |
| <code>interface.8.http-paxeer-get-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:483</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/readyz" {</code> |
| <code>interface.8.http-paxeer-post</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:492</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/" {</code> |
| <code>interface.8.http-paxeer-post-comet</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:495</code> | <code>if request.method == "POST" &amp;&amp; request.path == "/comet" {</code> |
| <code>interface.8.rpc-paxeer-pax-sequencerreceipt-j40</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>services/platform/hosted/paxeer/src/main.rs</code> (planned) | <code>pax_sequencerReceipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-paxeer-any-store-layerxcustody-key</code> | notes | no in-repo client cites the path outside the defining file |
| <code>interface.8.http-paxeer-any-store-layerxcustody-key</code> | overridden_by | override.J40.101 |
| <code>interface.8.http-paxeer-get-genesis</code> | notes | 264 callers total |
| <code>interface.8.http-paxeer-get-livez</code> | notes | 119 callers total |
| <code>interface.8.http-paxeer-get-readyz</code> | notes | 324 callers total |
| <code>interface.8.http-paxeer-post</code> | notes | path is too generic to attribute callers |
| <code>interface.8.rpc-paxeer-pax-sequencerreceipt-j40</code> | notes | planned admission of the sequencer receipt read on the paxeer chain relay; task 8.14.5 |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.147.1</code> | req | 8147.1 |
| <code>acceptance.8.147.1</code> | scenario | Given a proved custody store query and a proved EVM store query at height 17, when both bodies are validated, then the custody query is refused and the EVM query is admitted as Method::Query(17) |
| <code>acceptance.8.147.1</code> | test | services/platform/hosted/paxeer/src/comet.rs::custody_store_query_is_refused (planned) |
| <code>acceptance.8.147.1</code> | qualification | UNRUN |
| <code>req.8147</code> | ac_1 | An abci_query for /store/layerxcustody/key with prove true is refused with -32602 while a proved /store/evm/key query is still admitted. |
| <code>acceptance.8.147.2</code> | req | 8147.2 |
| <code>acceptance.8.147.2</code> | scenario | Given status bodies with empty and null params and a proved EVM store query, when each is validated, then status is Method::Status and the query is Method::Query with its height |
| <code>acceptance.8.147.2</code> | test | services/platform/hosted/paxeer/src/comet.rs::status_and_evm_store_proofs_are_admitted (planned) |
| <code>acceptance.8.147.2</code> | qualification | UNRUN |
| <code>req.8147</code> | ac_2 | The Comet boundary admits status and proved /store/evm/key queries exactly as before the custody path was removed. |
| <code>acceptance.8.151.1</code> | req | 8151.1 |
| <code>acceptance.8.151.1</code> | scenario | Given the sequencer receipt method, two admitted names, an unlisted pax_ name and two denied eth_ names, when each is checked by method_allowed, then only the three admitted names pass |
| <code>acceptance.8.151.1</code> | test | services/platform/hosted/paxeer/src/main.rs::sequencer_receipt_method_is_allowed (planned) |
| <code>acceptance.8.151.1</code> | qualification | UNRUN |
| <code>req.8151</code> | ac_1 | method_allowed admits pax_sequencerReceipt, eth_getTransactionReceipt and net_version and refuses pax_unknownMethod, eth_sendTransaction and eth_sign. |
| <code>capability.8.14.1</code> | title | Refuse custody store proofs at the paxeer Comet boundary |
| <code>capability.8.14.1</code> | reqs | 8147.1<br>8147.2 |
| <code>capability.8.14.5</code> | title | Admit the sequencer receipt read at the paxeer chain boundary |
| <code>capability.8.14.5</code> | reqs | 8151.1 |

## paxeer-jsonrpc-methods

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rpc-paxeer-net-version</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:31</code> | <code>const ALLOWED_METHODS: [&amp;str; 2] = ["net_version", "web3_clientVersion"];</code> |
| <code>interface.8.rpc-paxeer-web3-clientversion</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>services/platform/hosted/paxeer/src/main.rs:31</code> | <code>const ALLOWED_METHODS: [&amp;str; 2] = ["net_version", "web3_clientVersion"];</code> |

## platform-docs

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.231.1</code> | req | 8231.1 |
| <code>acceptance.8.231.1</code> | scenario | Given beta.md, when its surface table rows and funding sentence are parsed, then none names a retired kernel surface |
| <code>acceptance.8.231.1</code> | test | services/platform/docs/build/test_beta_surfaces.py::test_beta_surfaces_name_no_retired_kernel_surface (planned) |
| <code>acceptance.8.231.1</code> | qualification | UNRUN |
| <code>req.8231</code> | ac_1 | The beta surface table has no native-core or native-daemon row and no row names guarantor, layerxd or custody credit. |
| <code>capability.8.20.4</code> | title | List only merged-chain surfaces on the beta page |
| <code>capability.8.20.4</code> | reqs | 8231.1 |

## platform-workspace

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.219.1</code> | req | 8219.1 |
| <code>acceptance.8.219.1</code> | scenario | Given services/platform/Makefile.inc, when its text is searched for the retired names and the listener recipe is parsed, then none is present and the recipe has four cargo test lines |
| <code>acceptance.8.219.1</code> | test | services/platform/hosted/tests/test_platform_make_targets.py::test_platform_targets_build_no_kernel_binaries (planned) |
| <code>acceptance.8.219.1</code> | qualification | UNRUN |
| <code>req.8219</code> | ac_1 | Makefile.inc names no layerxd, layerx-archive-codec, tests/relay-archive or node-artifacts.py and still declares platform-test-plain-listeners with its four cargo runs. |
| <code>acceptance.8.223.1</code> | req | 8223.1 |
| <code>acceptance.8.223.1</code> | scenario | Given the gateway local package, when its Cargo.toml is parsed with tomllib and the six files are checked, then no test target remains, the files are absent and the two binaries are still declared |
| <code>acceptance.8.223.1</code> | test | services/platform/hosted/tests/test_retired_gateway_lifecycle.py::test_gateway_local_harness_has_no_lifecycle_test (planned) |
| <code>acceptance.8.223.1</code> | qualification | UNRUN |
| <code>req.8223</code> | ac_1 | The gateway local package declares no lifecycle test and none of lifecycle.rs, events.rs, funding.rs, withdrawal.rs, registry_runtime.rs or build.rs exists, while verifier.rs and lifecycle_sdk.rs remain declared binaries. |
| <code>acceptance.8.224.1</code> | req | 8224.1 |
| <code>acceptance.8.224.1</code> | scenario | Given the gateway local harness, when the three helpers are checked and the hosted tree is searched for their names, then they are absent and registry_image.py is present |
| <code>acceptance.8.224.1</code> | test | services/platform/hosted/tests/test_retired_gateway_helpers.py::test_gateway_local_harness_ships_no_kernel_helper (planned) |
| <code>acceptance.8.224.1</code> | qualification | UNRUN |
| <code>req.8224</code> | ac_1 | checkpoint_settlement.py, custody_chain.py and registry_runtime.py are absent from the gateway local harness and no file under services/platform/hosted names them, while registry_image.py remains. |
| <code>acceptance.8.227.1</code> | req | 8227.1 |
| <code>acceptance.8.227.1</code> | scenario | Given the two beta scripts, when their text is searched for the relay archive names, then nothing is found and bash -n accepts both |
| <code>acceptance.8.227.1</code> | test | services/platform/hosted/tests/test_beta_relay_archive_retired.py::test_beta_cluster_deploys_no_relay_archive (planned) |
| <code>acceptance.8.227.1</code> | qualification | UNRUN |
| <code>req.8227</code> | ac_1 | beta-images.sh lists no layerx-relay-archive image and beta-cluster.sh names no relay_archive function, relay-archive manifest or layerx-archive-codec. |
| <code>acceptance.8.230.1</code> | req | 8230.1 |
| <code>acceptance.8.230.1</code> | scenario | Given the platform manifests, sources and lock file, when they are parsed and searched for the verifier crate, then it is absent from all three |
| <code>acceptance.8.230.1</code> | test | services/platform/hosted/tests/test_workspace_manifest.py::test_paxeer_verifier_is_not_a_platform_dependency (planned) |
| <code>acceptance.8.230.1</code> | qualification | UNRUN |
| <code>req.8230</code> | ac_1 | No Cargo.toml of the platform workspace lists layerx-paxeer-verifier, no platform Rust source names layerx_paxeer_verifier, and services/platform/Cargo.lock has no package of that name. |
| <code>acceptance.8.81.1</code> | req | 8081.1 |
| <code>acceptance.8.81.1</code> | scenario | Given the platform workspace manifest, when the members list is parsed, then its length equals the size of its set and each member directory holds a manifest |
| <code>acceptance.8.81.1</code> | test | services/platform/hosted/tests/test_workspace_manifest.py::test_workspace_members_are_unique (planned) |
| <code>acceptance.8.81.1</code> | qualification | UNRUN |
| <code>req.8081</code> | ac_1 | The members list of services/platform/Cargo.toml has no duplicate entry and every member path contains a Cargo.toml. |
| <code>acceptance.8.82.1</code> | req | 8082.1 |
| <code>acceptance.8.82.1</code> | scenario | Given the five crate manifests and their src trees, when each listed kernel crate name is searched in the sources, then every listed one is used and the retired ones are absent, and services/platform/Cargo.lock lists neither layerx-intents nor layerx-paxeer-client under layerx-ramp-toolkit |
| <code>acceptance.8.82.1</code> | test | services/platform/hosted/tests/test_workspace_manifest.py::test_retired_kernel_crates_are_not_dependencies (planned) |
| <code>acceptance.8.82.1</code> | qualification | UNRUN |
| <code>req.8082</code> | ac_1 | No manifest of the gateway, agent boundary, authority, registry or indexer lists layerx-client, layerx-programs-runtime or layerx-programs-registry unless a source file of that crate uses it. services/platform/Cargo.lock (services/platform/Cargo.lock:1361) lists neither layerx-intents nor layerx-paxeer-client as a dependency of layerx-ramp-toolkit. |
| <code>capability.8.19.11</code> | title | Delete the kernel settlement and custody helpers of the gateway local harness |
| <code>capability.8.19.11</code> | reqs | 8224.1 |
| <code>capability.8.19.3</code> | title | Stop the platform make targets from building kernel binaries |
| <code>capability.8.19.3</code> | reqs | 8219.1 |
| <code>capability.8.19.5</code> | title | Delete the kernel lifecycle test of the gateway local harness |
| <code>capability.8.19.5</code> | reqs | 8223.1 |
| <code>capability.8.19.8</code> | title | Stop the beta cluster from building and deploying the relay archive |
| <code>capability.8.19.8</code> | reqs | 8227.1 |
| <code>capability.8.20.3</code> | title | Drop the paxeer checkpoint verifier from the platform crates |
| <code>capability.8.20.3</code> | reqs | 8230.1 |
| <code>capability.8.9.1</code> | title | List the runtime clock crate once in the platform workspace |
| <code>capability.8.9.1</code> | reqs | 8081.1 |
| <code>capability.8.9.2</code> | title | Drop the kernel client and program crates from the platform manifests |
| <code>capability.8.9.2</code> | reqs | 8082.1 |
| <code>resolution.17.804</code> | title | decision.809 binds: One task per wave edits the platform lock file |
| <code>resolution.17.804</code> | applies_to | decision.809<br>task.8.9.1<br>task.8.9.2<br>task.8.20.3<br>task.8.28.1<br>task.8.96.4 |
| <code>resolution.17.804</code> | source | decision.809 in fragments/J40/decision.kvx; tasks from affects, task text naming decision.809 |
| <code>resolution.17.834</code> | title | decision.890 binds: Task 8.28.1 is the wave-0 lock-file editor |
| <code>resolution.17.834</code> | applies_to | decision.890<br>task.8.9.2<br>task.8.28.1<br>task.8.31.2 |
| <code>resolution.17.834</code> | source | decision.890 in fragments/J79/decision.kvx; tasks from affects |

## registry-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-attach-before-exec</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:95</code> | <code>.any(&#124;argument&#124; argument == "--attach-before-exec")</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-cgroup-root</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:108</code> | <code>.position(&#124;argument&#124; argument == "--cgroup-root")</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-cgroup-v2</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:92</code> | <code>if !arguments.iter().any(&#124;argument&#124; argument == "--cgroup-v2")</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-cpu-time-max-usec</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:136</code> | <code>cpu_usec: number("--cpu-time-max-usec=")?,</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-io-write-max</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:139</code> | <code>io_write: number("--io-write-max=")?,</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-kill-tree-on-exit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:98</code> | <code>.any(&#124;argument&#124; argument == "--kill-tree-on-exit")</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-memory-max</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:135</code> | <code>memory: number("--memory-max=")?,</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-pids-max</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:137</code> | <code>pids: u32::try_from(number("--pids-max=")?)</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-stopped-launcher</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:80</code> | <code>if arguments.first().map(String::as_str) == Some("--stopped-launcher") {</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-wall-time-max-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:140</code> | <code>wall_ms: number("--wall-time-max-ms=")?,</code> |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-workspace-device-path</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/bin/layerx-cgroup-exec.rs:116</code> | <code>.position(&#124;argument&#124; argument == "--workspace-device-path")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-attach-before-exec</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-cgroup-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-cgroup-v2</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-cpu-time-max-usec</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-io-write-max</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-kill-tree-on-exit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-memory-max</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-pids-max</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-stopped-launcher</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-wall-time-max-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-registry-layerx-cgroup-exec-rs-workspace-device-path</code> | notes | no in-repo invocation outside the defining file |

## registry-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-registry-layerx-registry-attempts-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:471</code> | <code>attempts: parse_u32("LAYERX_REGISTRY_ATTEMPTS", 2)?,</code> |
| <code>interface.8.env-registry-layerx-registry-build-quota-bytes</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/node-provision-build-boundary.sh:6</code> | <code>BYTES=${LAYERX_REGISTRY_BUILD_QUOTA_BYTES:?missing build byte quota}</code> |
| <code>interface.8.env-registry-layerx-registry-build-quota-inodes</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/node-provision-build-boundary.sh:7</code> | <code>INODES=${LAYERX_REGISTRY_BUILD_QUOTA_INODES:?missing build inode quota}</code> |
| <code>interface.8.env-registry-layerx-registry-builder-entrypoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:444</code> | <code>builder_entrypoint: env::var("LAYERX_REGISTRY_BUILDER_ENTRYPOINT")</code> |
| <code>interface.8.env-registry-layerx-registry-builder-environment-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:441</code> | <code>env::var("LAYERX_REGISTRY_BUILDER_ENVIRONMENT_ROOT")</code> |
| <code>interface.8.env-registry-layerx-registry-builder-image-digest</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:418</code> | <code>let digest = env::var("LAYERX_REGISTRY_BUILDER_IMAGE_DIGEST")</code> |
| <code>interface.8.env-registry-layerx-registry-builder-isolation-runtime</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:447</code> | <code>env::var("LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME")</code> |
| <code>interface.8.env-registry-layerx-registry-builder-isolation-runtime-digest</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:451</code> | <code>&amp;env::var("LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME_DIGEST").map_err(&#124;_&#124; {</code> |
| <code>interface.8.env-registry-layerx-registry-builder-job-supervisor</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:457</code> | <code>env::var("LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR")</code> |
| <code>interface.8.env-registry-layerx-registry-builder-job-supervisor-digest</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:461</code> | <code>&amp;env::var("LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR_DIGEST").map_err(&#124;_&#124; {</code> |
| <code>interface.8.env-registry-layerx-registry-chain-rpc-url-j40</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/hosted/registry/src/programs_chain.rs</code> (planned) | <code>LAYERX_REGISTRY_CHAIN_RPC_URL</code> |
| <code>interface.8.env-registry-layerx-registry-client-identity-password-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:404</code> | <code>const PASSWORD: &amp;str = "LAYERX_REGISTRY_CLIENT_IDENTITY_PASSWORD_FILE";</code> |
| <code>interface.8.env-registry-layerx-registry-client-identity-pkcs12</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:403</code> | <code>const IDENTITY: &amp;str = "LAYERX_REGISTRY_CLIENT_IDENTITY_PKCS12";</code> |
| <code>interface.8.env-registry-layerx-registry-health-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:782</code> | <code>match env::var("LAYERX_REGISTRY_HEALTH_LISTEN") {</code> |
| <code>interface.8.env-registry-layerx-registry-host-cgroup-mount</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:185</code> | <code>let mount = env::var("LAYERX_REGISTRY_HOST_CGROUP_MOUNT")</code> |
| <code>interface.8.env-registry-layerx-registry-identity-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1436</code> | <code>"LAYERX_REGISTRY_IDENTITY",</code> |
| <code>interface.8.env-registry-layerx-registry-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:432</code> | <code>listen: env::var("LAYERX_REGISTRY_LISTEN").unwrap_or_else(&#124;_&#124; DEFAULT_LISTEN.to_owned()),</code> |
| <code>interface.8.env-registry-layerx-registry-lni-socket</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/registry/src/main.rs:434</code> | <code>deployment_lni_socket: std::env::var_os("LAYERX_REGISTRY_LNI_SOCKET").map(PathBuf::from),</code> |
| <code>interface.8.env-registry-layerx-registry-max-builds</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/node-provision-build-boundary.sh:5</code> | <code>SLOTS=${LAYERX_REGISTRY_MAX_BUILDS:?missing build slot count}</code> |
| <code>interface.8.env-registry-layerx-registry-max-staleness-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:472</code> | <code>staleness_ms: parse_u64("LAYERX_REGISTRY_MAX_STALENESS_SECONDS", 300)?</code> |
| <code>interface.8.env-registry-layerx-registry-node-authorization</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:477</code> | <code>node_authorization: env::var("LAYERX_REGISTRY_NODE_AUTHORIZATION")</code> |
| <code>interface.8.env-registry-layerx-registry-node-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:475</code> | <code>node_endpoint: env::var("LAYERX_REGISTRY_NODE_ENDPOINT")</code> |
| <code>interface.8.env-registry-layerx-registry-node-lock</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/node-provision-build-boundary.sh:10</code> | <code>exec 9&gt;"${LAYERX_REGISTRY_NODE_LOCK:-/run/lock/layerx-program-registry-boundary.lock}"</code> |
| <code>interface.8.env-registry-layerx-registry-node-quota-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/node-provision-build-boundary.sh:4</code> | <code>QUOTA_ROOT=${LAYERX_REGISTRY_NODE_QUOTA_ROOT:-/var/lib/layerx-program-registry-builds}</code> |
| <code>interface.8.env-registry-layerx-registry-outbound-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:480</code> | <code>env::var("LAYERX_REGISTRY_OUTBOUND_CA_DER")</code> |
| <code>interface.8.env-registry-layerx-registry-receipt-authority-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:485</code> | <code>receipt_authority_endpoint: env::var("LAYERX_REGISTRY_RECEIPT_AUTHORITY_ENDPOINT")</code> |
| <code>interface.8.env-registry-layerx-registry-receipt-authority-replica-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:492</code> | <code>&amp;env::var("LAYERX_REGISTRY_RECEIPT_AUTHORITY_REPLICA_ID").map_err(&#124;_&#124; {</code> |
| <code>interface.8.env-registry-layerx-registry-sequencer-trust-history</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:500</code> | <code>env::var("LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY")</code> |
| <code>interface.8.env-registry-layerx-registry-source-mirror-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:435</code> | <code>mirror: parse_path("LAYERX_REGISTRY_SOURCE_MIRROR", root.join("sources")),</code> |
| <code>interface.8.env-registry-layerx-registry-test-delegation-child-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:1557</code> | <code>if let Some(root) = env::var_os("LAYERX_REGISTRY_TEST_DELEGATION_CHILD") {</code> |
| <code>interface.8.env-registry-layerx-registry-verified-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:436</code> | <code>verified: parse_path("LAYERX_REGISTRY_VERIFIED", root.join("verified")),</code> |
| <code>interface.8.env-registry-lc-all</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/builder.rs:452</code> | <code>.args(["--setenv", "TZ", "UTC", "--setenv", "LC_ALL", "C"])</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-registry-layerx-registry-attempts-j69</code> | notes | read at services/platform/hosted/registry/src/main.rs:471 |
| <code>interface.8.env-registry-layerx-registry-chain-rpc-url-j40</code> | notes | planned chain JSON-RPC endpoint for 0x101D getStateHead and getProgram reads; task 8.12.1 |
| <code>interface.8.env-registry-layerx-registry-identity-j69</code> | notes | read at services/platform/hosted/registry/src/routes.rs:1436; 24 in-repo references, first 8 listed |
| <code>interface.8.env-registry-layerx-registry-lni-socket</code> | notes | LNI transport retired; reads move to eth_call and gRPC queries (spec/layerx-merge/spec.kvx:60434) |
| <code>interface.8.env-registry-layerx-registry-max-staleness-seconds-j69</code> | notes | read at services/platform/hosted/registry/src/main.rs:472 |
| <code>interface.8.env-registry-layerx-registry-source-mirror-j69</code> | notes | read at services/platform/hosted/registry/src/main.rs:435 |
| <code>interface.8.env-registry-layerx-registry-test-delegation-child-j69</code> | notes | read at services/platform/hosted/registry/src/main.rs:1557 |
| <code>interface.8.env-registry-layerx-registry-verified-j69</code> | notes | read at services/platform/hosted/registry/src/main.rs:436 |
| <code>interface.8.env-registry-lc-all</code> | notes | 70 callers total |

Contract context <code>contract_context.8.hosted-chain-readers</code>:

| Field | Frozen rule |
|---|---|
| interface | Chain JSON-RPC clients of the core boundary funding path, the receipt authority and the program registry |
| storage | core funding journal; authority receipt store; registry journal |
| errors | treasury_unavailable 503, identity_binding_unavailable 409, idempotency_conflict 409, deployment_proof_refused 422, protocol_state_unavailable 503 |
| events | none |
| gas | one value transfer per funding command, paid by the funding sender |
| activation | wave 3 moves the readers, wave 6 removes the kernel sockets |
| invariants | a funding command signs at most one transaction; every admitted receipt verifies under a pinned key; every registry head is read at one block height |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.116.1</code> | req | 8116.1 |
| <code>acceptance.8.116.1</code> | scenario | Given an environment without the LNI variable and a chain listener, when the registry starts and ingests a head and a deployment, then both answer 200 |
| <code>acceptance.8.116.1</code> | test | services/platform/hosted/registry/tests/programs_chain.rs::registry_starts_without_an_lni_socket (planned) |
| <code>acceptance.8.116.1</code> | qualification | UNRUN |
| <code>req.8116</code> | ac_1 | The program registry starts with no LAYERX_REGISTRY_LNI_SOCKET and records a head and a deployment from the chain. |
| <code>capability.8.12.3</code> | title | Remove the kernel socket and node-state proofs from the program registry |
| <code>capability.8.12.3</code> | reqs | 8116.1 |

## registry-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-registry-build-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1582</code> | <code>BuildRefusal::BuilderFailed { reason } =&gt; refusal(422, "build_failed", reason),</code> |
| <code>interface.8.err-registry-build-not-reproducible</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1584</code> | <code>refusal(422, "build_not_reproducible", &amp;refused.to_string())</code> |
| <code>interface.8.err-registry-build-queue-full</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:1138</code> | <code>return refusal(503, "build_queue_full", "the bounded build queue is full");</code> |
| <code>interface.8.err-registry-builder-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1581</code> | <code>BuildRefusal::SandboxUnavailable { reason } =&gt; refusal(503, "builder_unavailable", reason),</code> |
| <code>interface.8.err-registry-deployment-head-unverified</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1220</code> | <code>.map_err(&#124;error&#124; refusal(503, "deployment_head_unverified", &amp;error))?;</code> |
| <code>interface.8.err-registry-deployment-projection-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1344</code> | <code>return refusal(422, "deployment_projection_refused", &amp;error.to_string());</code> |
| <code>interface.8.err-registry-deployment-proof-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1306</code> | <code>Err(error) =&gt; return refusal(422, "deployment_proof_refused", &amp;error),</code> |
| <code>interface.8.err-registry-deployment-proof-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1322</code> | <code>Err(error) =&gt; return refusal(503, "deployment_proof_unavailable", &amp;error),</code> |
| <code>interface.8.err-registry-idempotency-record-corrupt</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1843</code> | <code>JournalRefusal::Corrupt(defect) =&gt; refusal(503, "idempotency_record_corrupt", defect),</code> |
| <code>interface.8.err-registry-idempotency-store-corrupt</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1007</code> | <code>Err(error) =&gt; return refusal(503, "idempotency_store_corrupt", &amp;error),</code> |
| <code>interface.8.err-registry-idempotency-store-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:909</code> | <code>return refusal(503, "idempotency_store_unavailable", &amp;error)</code> |
| <code>interface.8.err-registry-interface-proof-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:820</code> | <code>Err(error) =&gt; return refusal(503, "interface_proof_unavailable", &amp;error),</code> |
| <code>interface.8.err-registry-invalid-argument</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1477</code> | <code>Err(error) =&gt; refusal(400, "invalid_argument", &amp;error),</code> |
| <code>interface.8.err-registry-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:855</code> | <code>&#124;_&#124; refusal(400, "invalid_request", "request could not be parsed"),</code> |
| <code>interface.8.err-registry-journal-export-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1309</code> | <code>return refusal(503, "journal_export_unavailable", &amp;error);</code> |
| <code>interface.8.err-registry-journal-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1297</code> | <code>Err(error) =&gt; return refusal(503, "journal_unavailable", &amp;error),</code> |
| <code>interface.8.err-registry-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:481</code> | <code>return refusal(404, "not_found", "route does not exist");</code> |
| <code>interface.8.err-registry-persistence-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1050</code> | <code>return refusal(503, "persistence_unavailable", &amp;error);</code> |
| <code>interface.8.err-registry-protocol-state-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:502</code> | <code>return refusal(503, "protocol_state_unavailable", &amp;error);</code> |
| <code>interface.8.err-registry-read-unverifiable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:505</code> | <code>return refusal(503, "read_unverifiable", &amp;error.to_string());</code> |
| <code>interface.8.err-registry-receipt-authority-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1340</code> | <code>return refusal(503, "receipt_authority_unavailable", &amp;error);</code> |
| <code>interface.8.err-registry-source-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1620</code> | <code>"code": "source_mismatch",</code> |
| <code>interface.8.err-registry-source-not-mirrored</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1236</code> | <code>return refusal(404, "source_not_mirrored", &amp;refused.to_string())</code> |
| <code>interface.8.err-registry-source-unverifiable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1238</code> | <code>Err(refused) =&gt; return refusal(422, "source_unverifiable", &amp;refused.to_string()),</code> |
| <code>interface.8.err-registry-stale-read</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:515</code> | <code>return refusal(503, "stale_read", "protocol head freshness overflowed");</code> |
| <code>interface.8.err-registry-unverified-read</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:568</code> | <code>Err(error) =&gt; refusal(502, "unverified_read", &amp;error.to_string()),</code> |
| <code>interface.8.err-registry-verification-pending</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:1065</code> | <code>return refusal(503, "verification_pending", &amp;error.to_string());</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-registry-build-failed</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-build-not-reproducible</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-build-queue-full</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-builder-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-registry-deployment-head-unverified</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-deployment-projection-refused</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-deployment-proof-refused</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-deployment-proof-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-registry-idempotency-record-corrupt</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-idempotency-store-corrupt</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-idempotency-store-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-interface-proof-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-invalid-argument</code> | notes | HTTP 400; 69 callers total |
| <code>interface.8.err-registry-invalid-request</code> | notes | HTTP 400; 77 callers total |
| <code>interface.8.err-registry-journal-export-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-journal-unavailable</code> | notes | HTTP 503; 41 callers total |
| <code>interface.8.err-registry-not-found</code> | notes | HTTP 404; 129 callers total |
| <code>interface.8.err-registry-persistence-unavailable</code> | notes | HTTP 503; 68 callers total |
| <code>interface.8.err-registry-protocol-state-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-read-unverifiable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-receipt-authority-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-source-not-mirrored</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-source-unverifiable</code> | notes | HTTP 422 |
| <code>interface.8.err-registry-stale-read</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-registry-unverified-read</code> | notes | HTTP 502 |
| <code>interface.8.err-registry-verification-pending</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |

## registry-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-registry-build-log</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/builder.rs:22</code> | <code>const BUILD_LOG: &amp;str = "layerx-build.log";</code> |
| <code>interface.8.fmt-registry-owner-lock</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/verified.rs:192</code> | <code>const OWNER_LOCK: &amp;str = "owner.lock";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-registry-build-log</code> | notes | build log file |
| <code>interface.8.fmt-registry-owner-lock</code> | notes | verified store owner lock; read and written only by the defining module |

## registry-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-registry-get-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:300</code> | <code>("GET", "/healthz") =&gt; Response {</code> |
| <code>interface.8.http-registry-get-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:1113</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/metrics" {</code> |
| <code>interface.8.http-registry-get-v1-programs-registry</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:312</code> | <code>("GET", "/v1/programs/registry") =&gt; self.list(now),</code> |
| <code>interface.8.http-registry-post-registry-deployments</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/hosted/registry/src/routes.rs:304</code> | <code>("POST", "/__registry/deployments") =&gt; self.ingest_deployment(&amp;request.body, deadline),</code> |
| <code>interface.8.http-registry-post-registry-head</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/hosted/registry/src/routes.rs:305</code> | <code>("POST", "/__registry/head") =&gt; self.ingest_head(now),</code> |
| <code>interface.8.http-registry-post-registry-sources</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/routes.rs:306</code> | <code>("POST", "/__registry/sources") =&gt; {</code> |
| <code>interface.8.http-registry-post-v1-programs-registry</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:1133</code> | <code>&amp;&amp; request.path.starts_with("/v1/programs/registry/")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-registry-get-healthz</code> | notes | 329 callers total |
| <code>interface.8.http-registry-get-metrics</code> | notes | 323 callers total |
| <code>interface.8.http-registry-post-registry-deployments</code> | notes | registry native deploy and head attestation retire as one unit (spec/layerx-merge/spec.kvx:559); deployment answers as a 0x101D transaction |
| <code>interface.8.http-registry-post-registry-head</code> | notes | registry native deploy and head attestation retire as one unit (spec/layerx-merge/spec.kvx:559); deployment answers as a 0x101D transaction |
| <code>interface.8.http-registry-post-v1-programs-registry</code> | notes | 100 callers total |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.114.1</code> | req | 8114.1 |
| <code>acceptance.8.114.1</code> | scenario | Given a chain listener answering eth_blockNumber 0x20 and a state record for the program, when the registry head is ingested, then the answer has sequence 32 and the record's keccak and every eth_call named block 0x20 |
| <code>acceptance.8.114.1</code> | test | services/platform/hosted/registry/tests/programs_chain.rs::registry_head_reads_the_programs_state_head (planned) |
| <code>acceptance.8.114.1</code> | qualification | UNRUN |
| <code>req.8114</code> | ac_1 | POST /__registry/head answers observed true with the block height as sequence and the keccak of the getStateHead record as state_root, all read at one height. |
| <code>acceptance.8.115.1</code> | req | 8115.1 |
| <code>acceptance.8.115.1</code> | scenario | Given a chain listener holding one successful and one reverted deployment receipt, when both hashes are ingested, then the first journals code hash and version from getProgram and the second answers 422 |
| <code>acceptance.8.115.1</code> | test | services/platform/hosted/registry/tests/programs_chain.rs::registry_deployment_reads_the_program_record (planned) |
| <code>acceptance.8.115.1</code> | qualification | UNRUN |
| <code>req.8115</code> | ac_1 | POST /__registry/deployments with a deployment transaction hash journals the getProgram record of the deployed program and refuses a failed transaction with deployment_proof_refused. |
| <code>capability.8.12.1</code> | title | Observe program heads from the programs construct getStateHead |
| <code>capability.8.12.1</code> | reqs | 8114.1 |
| <code>capability.8.12.2</code> | title | Record deployments from the programs construct getProgram |
| <code>capability.8.12.2</code> | reqs | 8115.1 |
| <code>service.platform-program-registry</code> | purpose | Publishes verified program records and heads read from the programs construct |
| <code>service.platform-program-registry</code> | route | POST /__registry/head, POST /__registry/deployments, registry read routes |
| <code>service.platform-program-registry</code> | inputs | LAYERX_REGISTRY_CHAIN_RPC_URL, source mirror and builder configuration |
| <code>service.platform-program-registry</code> | outputs | registry documents with heads and deployment records |
| <code>service.platform-program-registry</code> | state | registry journal, source mirror, verified builds |
| <code>service.platform-program-registry</code> | dependencies | chain JSON-RPC eth_call at 0x101D |
| <code>service.platform-program-registry</code> | failure_policy | an unreadable head answers protocol_state_unavailable 503 and keeps the last journaled head |
| <code>service.platform-program-registry</code> | completion_task | 8.0.9 |
| <code>service.platform-program-registry</code> | operation_task | 8.12.3 |
| <code>service.platform-program-registry</code> | domain | 8 |

## registry-metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.met-registry-event-producer-failures</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:1078</code> | <code>"event_producer_failures": failures,</code> |
| <code>interface.8.met-registry-event-producer-overflow</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:1079</code> | <code>"event_producer_overflow": overflow,</code> |
| <code>interface.8.met-registry-program-facts-unbound</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/hosted/registry/src/main.rs:1077</code> | <code>"program_facts_unbound": unbound,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.met-registry-event-producer-failures</code> | notes | JSON body of GET /metrics |
| <code>interface.8.met-registry-event-producer-overflow</code> | notes | JSON body of GET /metrics |
| <code>interface.8.met-registry-program-facts-unbound</code> | notes | JSON body of GET /metrics; no in-repo scraper or dashboard cites it |

## runtime-clock-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-runtime-clock-main-rs-runtime-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/hosted/runtime-clock/src/main.rs:35</code> | <code>Some(value) if value == "--runtime-dir" =&gt; {</code> |

## runtime-clock-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-runtime-clock-layerx-runtime-clock-pid</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/runtime-clock/src/main.rs:64</code> | <code>.env("LAYERX_RUNTIME_CLOCK_PID", std::process::id().to_string())</code> |
| <code>interface.8.env-runtime-clock-layerx-runtime-clock-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/runtime-clock/src/main.rs:63</code> | <code>.env("LAYERX_RUNTIME_CLOCK_SOCKET", server.socket())</code> |

## testnet-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-testnet-layerx-outbound-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:16</code> | <code>: "${LAYERX_OUTBOUND_CA_DER:?LAYERX_OUTBOUND_CA_DER is required}"</code> |
| <code>interface.8.env-testnet-layerx-pending-package-semver</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:3</code> | <code>: "${LAYERX_PENDING_PACKAGE_SEMVER:?LAYERX_PENDING_PACKAGE_SEMVER is required}"</code> |
| <code>interface.8.env-testnet-layerx-pending-wire-protocol-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:4</code> | <code>: "${LAYERX_PENDING_WIRE_PROTOCOL_VERSION:?LAYERX_PENDING_WIRE_PROTOCOL_VERSION is required}"</code> |
| <code>interface.8.env-testnet-layerx-status-publish-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/render-status.sh:6</code> | <code>: "${LAYERX_STATUS_PUBLISH_URL:?LAYERX_STATUS_PUBLISH_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-status-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/render-status.sh:7</code> | <code>: "${LAYERX_STATUS_TOKEN_FILE:?LAYERX_STATUS_TOKEN_FILE is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-admin-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:812</code> | <code>admin_listen: env::var("LAYERX_TESTNET_ADMIN_LISTEN")</code> |
| <code>interface.8.env-testnet-layerx-testnet-admin-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/reset-testnet.sh:5</code> | <code>: "${LAYERX_TESTNET_ADMIN_URL:?LAYERX_TESTNET_ADMIN_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-backend-admin-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:17</code> | <code>: "${LAYERX_TESTNET_BACKEND_ADMIN_TOKEN_FILE:?LAYERX_TESTNET_BACKEND_ADMIN_TOKEN_FILE is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-ca-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/render-status.sh:8</code> | <code>: "${LAYERX_TESTNET_CA_FILE:?LAYERX_TESTNET_CA_FILE is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-control-admin-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:18</code> | <code>: "${LAYERX_TESTNET_CONTROL_ADMIN_TOKEN_FILE:?LAYERX_TESTNET_CONTROL_ADMIN_TOKEN_FILE is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-core-admin-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:6</code> | <code>: "${LAYERX_TESTNET_CORE_ADMIN_URL:?LAYERX_TESTNET_CORE_ADMIN_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-core-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:5</code> | <code>: "${LAYERX_TESTNET_CORE_URL:?LAYERX_TESTNET_CORE_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-faucet-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:10</code> | <code>: "${LAYERX_TESTNET_FAUCET_URL:?LAYERX_TESTNET_FAUCET_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-gateway-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:7</code> | <code>: "${LAYERX_TESTNET_GATEWAY_URL:?LAYERX_TESTNET_GATEWAY_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-identity-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:9</code> | <code>: "${LAYERX_TESTNET_IDENTITY_URL:?LAYERX_TESTNET_IDENTITY_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-paxeer-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:8</code> | <code>: "${LAYERX_TESTNET_PAXEER_URL:?LAYERX_TESTNET_PAXEER_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-public-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:808</code> | <code>public_listen: env::var("LAYERX_TESTNET_PUBLIC_LISTEN")</code> |
| <code>interface.8.env-testnet-layerx-testnet-receipt-authority-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:11</code> | <code>: "${LAYERX_TESTNET_RECEIPT_AUTHORITY_URL:?LAYERX_TESTNET_RECEIPT_AUTHORITY_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-redis-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:13</code> | <code>: "${LAYERX_TESTNET_REDIS_URL:?LAYERX_TESTNET_REDIS_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-registry-url</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/hosted/testnet/run-testnet.sh:12</code> | <code>: "${LAYERX_TESTNET_REGISTRY_URL:?LAYERX_TESTNET_REGISTRY_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-reset-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/reset-testnet.sh:6</code> | <code>: "${LAYERX_TESTNET_RESET_TOKEN_FILE:?LAYERX_TESTNET_RESET_TOKEN_FILE is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-status-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/render-status.sh:5</code> | <code>: "${LAYERX_TESTNET_STATUS_URL:?LAYERX_TESTNET_STATUS_URL is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-tls-cert-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:14</code> | <code>: "${LAYERX_TESTNET_TLS_CERT_DER:?LAYERX_TESTNET_TLS_CERT_DER is required}"</code> |
| <code>interface.8.env-testnet-layerx-testnet-tls-key-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/testnet/run-testnet.sh:15</code> | <code>: "${LAYERX_TESTNET_TLS_KEY_DER:?LAYERX_TESTNET_TLS_KEY_DER is required}"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-testnet-layerx-testnet-registry-url</code> | overridden_by | override.J40.102 |

## testnet-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-testnet-content-type-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1219</code> | <code>serde_json::json!({ "error": { "code": "content_type_required" } }),</code> |
| <code>interface.8.err-testnet-core-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1287</code> | <code>serde_json::json!({ "error": { "code": "core_unavailable", "retry": "after" } }),</code> |
| <code>interface.8.err-testnet-idempotency-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1225</code> | <code>serde_json::json!({ "error": { "code": "idempotency_key_required" } }),</code> |
| <code>interface.8.err-testnet-invalid-argument</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1239</code> | <code>serde_json::json!({ "error": { "code": "invalid_argument" } }),</code> |
| <code>interface.8.err-testnet-invalid-idempotency-key</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1231</code> | <code>serde_json::json!({ "error": { "code": "invalid_idempotency_key" } }),</code> |
| <code>interface.8.err-testnet-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1345</code> | <code>serde_json::json!({ "error": { "code": "invalid_request" } }),</code> |
| <code>interface.8.err-testnet-journey-degraded</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:430</code> | <code>"code": "journey_degraded",</code> |
| <code>interface.8.err-testnet-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1150</code> | <code>return json_response(404, serde_json::json!({ "error": { "code": "not_found" } }));</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-testnet-invalid-argument</code> | notes | 77 callers total |
| <code>interface.8.err-testnet-invalid-request</code> | notes | 78 callers total |
| <code>interface.8.err-testnet-journey-degraded</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-testnet-not-found</code> | notes | 134 callers total |

## testnet-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-testnet-any-livez</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1153</code> | <code>"/livez" =&gt; json_response(200, serde_json::json!({ "status": "live" })),</code> |
| <code>interface.8.http-testnet-any-readyz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1154</code> | <code>"/readyz" =&gt; {</code> |
| <code>interface.8.http-testnet-any-v1-parameters</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1162</code> | <code>"/v1/parameters" =&gt; json_response(</code> |
| <code>interface.8.http-testnet-any-v1-status</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1161</code> | <code>"/v1/status" =&gt; json_response(200, status(config).public_status(&amp;config.release)),</code> |
| <code>interface.8.http-testnet-post-admin-v1-testnet-fund</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1235</code> | <code>("POST", "/admin/v1/testnet/fund") =&gt; {</code> |
| <code>interface.8.http-testnet-post-admin-v1-testnet-reset</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/testnet/src/main.rs:1264</code> | <code>("POST", "/admin/v1/testnet/reset") =&gt; {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-testnet-any-livez</code> | notes | 119 callers total |
| <code>interface.8.http-testnet-any-readyz</code> | notes | 320 callers total |
| <code>interface.8.http-testnet-post-admin-v1-testnet-fund</code> | notes | 38 callers total |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.149.1</code> | req | 8149.1 |
| <code>acceptance.8.149.1</code> | scenario | Given the journey scripts, when every rpc call's method literal is collected from their syntax trees, then the set is non-empty and holds no name starting with lx_ |
| <code>acceptance.8.149.1</code> | test | services/platform/hosted/testnet/tests/test_journeys.py::test_payment_journey_calls_no_lx_method (planned) |
| <code>acceptance.8.149.1</code> | qualification | UNRUN |
| <code>req.8149</code> | ac_1 | Every JSON-RPC method the payment journey and journey credential scripts call is a px_, eth_ or pax_ name and none is an lx_ name. |
| <code>acceptance.8.150.1</code> | req | 8150.1 |
| <code>acceptance.8.150.1</code> | scenario | Given every testnet journey, when their dependency sets are listed, then there are three journeys and the registry dependency appears in none |
| <code>acceptance.8.150.1</code> | test | services/platform/hosted/testnet/src/main.rs::no_journey_depends_on_the_program_registry (planned) |
| <code>acceptance.8.150.1</code> | qualification | UNRUN |
| <code>req.8150</code> | ac_1 | Journey::ALL holds Funding, Payment and ReceiptInspection and no journey's dependency set names the program registry. |
| <code>acceptance.8.150.2</code> | req | 8150.2 |
| <code>acceptance.8.150.2</code> | scenario | Given the three journeys, when each dependency set is compared with its declared list, then each matches exactly |
| <code>acceptance.8.150.2</code> | test | services/platform/hosted/testnet/src/main.rs::journeys_declare_their_dependency_sets (existing) |
| <code>acceptance.8.150.2</code> | qualification | UNRUN |
| <code>req.8150</code> | ac_2 | The Funding, Payment and ReceiptInspection journeys declare exactly their dependency sets after the programs journey is removed. |
| <code>capability.8.14.3</code> | title | Run the testnet payment journey on the chain methods |
| <code>capability.8.14.3</code> | reqs | 8149.1 |
| <code>capability.8.14.4</code> | title | Retire the programs journey and the registry dependency of testnet control |
| <code>capability.8.14.4</code> | reqs | 8150.1<br>8150.2 |
| <code>resolution.17.809</code> | title | decision.814 binds: Testnet journeys keep their scripts and change their methods |
| <code>resolution.17.809</code> | applies_to | decision.814<br>task.8.14.3<br>task.8.14.4 |
| <code>resolution.17.809</code> | source | decision.814 in fragments/J40/decision.kvx; tasks from affects |
| <code>service.platform-testnet-control</code> | purpose | Testnet control plane reporting journey readiness and forwarding funding and reset commands |
| <code>service.platform-testnet-control</code> | route | /livez, /readyz, /v1/status, /v1/parameters, POST /admin/v1/testnet/fund, POST /admin/v1/testnet/reset |
| <code>service.platform-testnet-control</code> | inputs | dependency endpoints and admin bearer token |
| <code>service.platform-testnet-control</code> | outputs | journey readiness documents and forwarded admin answers |
| <code>service.platform-testnet-control</code> | state | none |
| <code>service.platform-testnet-control</code> | dependencies | identity, faucet, core boundary, receipt authority, gateway, paxeer chain, Redis |
| <code>service.platform-testnet-control</code> | failure_policy | a degraded journey names every failing dependency once and refuses its admin command |
| <code>service.platform-testnet-control</code> | completion_task | 8.0.9 |
| <code>service.platform-testnet-control</code> | operation_task | 8.14.4 |
| <code>service.platform-testnet-control</code> | domain | 8 |

## webhooks-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-webhooks-retrypolicy-base-delay-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/endpoints.rs:13</code> | <code>pub base_delay_seconds: u64,</code> |
| <code>interface.8.cfg-webhooks-retrypolicy-in-flight-timeout-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/endpoints.rs:23</code> | <code>pub in_flight_timeout_seconds: u64,</code> |
| <code>interface.8.cfg-webhooks-retrypolicy-maximum-attempts</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/endpoints.rs:17</code> | <code>pub maximum_attempts: u32,</code> |
| <code>interface.8.cfg-webhooks-retrypolicy-maximum-delay-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/endpoints.rs:15</code> | <code>pub maximum_delay_seconds: u64,</code> |
| <code>interface.8.cfg-webhooks-retrypolicy-spread-percent</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/endpoints.rs:19</code> | <code>pub spread_percent: u8,</code> |
| <code>interface.8.cfg-webhooks-retrypolicy-suspend-after-dead-letters</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/endpoints.rs:21</code> | <code>pub suspend_after_dead_letters: u32,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-webhooks-retrypolicy-base-delay-seconds</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-webhooks-retrypolicy-in-flight-timeout-seconds</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-webhooks-retrypolicy-maximum-attempts</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-webhooks-retrypolicy-spread-percent</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-webhooks-retrypolicy-suspend-after-dead-letters</code> | notes | no in-repo config file sets the key outside the defining file |

## webhooks-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-webhooks-layerx-dashboard-client-identity-password-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:503</code> | <code>let password = read_secret("LAYERX_DASHBOARD_CLIENT_IDENTITY_PASSWORD_FILE")?;</code> |
| <code>interface.8.env-webhooks-layerx-dashboard-client-identity-pkcs12</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:506</code> | <code>env::var("LAYERX_DASHBOARD_CLIENT_IDENTITY_PKCS12")</code> |
| <code>interface.8.env-webhooks-layerx-dashboard-identity-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:519</code> | <code>token: read_secret("LAYERX_DASHBOARD_IDENTITY_TOKEN_FILE")?,</code> |
| <code>interface.8.env-webhooks-layerx-dashboard-identity-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:516</code> | <code>&amp;env::var("LAYERX_DASHBOARD_IDENTITY_URL")</code> |
| <code>interface.8.env-webhooks-layerx-dashboard-internal-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:497</code> | <code>env::var("LAYERX_DASHBOARD_INTERNAL_CA_DER")</code> |
| <code>interface.8.env-webhooks-layerx-health-addr</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/env-init.sh:27</code> | <code>if [ -n "${LAYERX_HEALTH_ADDR:-}" ]; then</code> |
| <code>interface.8.env-webhooks-layerx-role</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/env-init.sh:35</code> | <code>case "${LAYERX_ROLE:-}" in</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-authority-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:258</code> | <code>authority_token: read_secret("LAYERX_WEBHOOKS_AUTHORITY_TOKEN_FILE")?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-authority-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:255</code> | <code>&amp;env::var("LAYERX_WEBHOOKS_AUTHORITY_URL")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-base-delay-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:685</code> | <code>base_delay_seconds: number("LAYERX_WEBHOOKS_BASE_DELAY_SECONDS", 10)?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-client-identity-password-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:738</code> | <code>let identity_password = read_secret("LAYERX_WEBHOOKS_CLIENT_IDENTITY_PASSWORD_FILE")?;</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-client-identity-pkcs12</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:214</code> | <code>env::var("LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-component-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:253</code> | <code>component_token: read_secret("LAYERX_WEBHOOKS_COMPONENT_TOKEN_FILE")?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-component-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:250</code> | <code>&amp;env::var("LAYERX_WEBHOOKS_COMPONENT_URL")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-cursor-key-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:747</code> | <code>let cursor = read_secret("LAYERX_WEBHOOKS_CURSOR_KEY_FILE")?;</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-dispatch-budget-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:237</code> | <code>dispatch_budget: u32::try_from(number("LAYERX_WEBHOOKS_DISPATCH_BUDGET", 64)?)</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-dispatch-interval-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:234</code> | <code>"LAYERX_WEBHOOKS_DISPATCH_INTERVAL_SECONDS",</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-health-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:221</code> | <code>health_listen: match env::var("LAYERX_WEBHOOKS_HEALTH_LISTEN") {</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-identity-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:106</code> | <code>const INGRESS_FOREIGN_VARIABLES: [&amp;str; 1] = ["LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE"];</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-identity-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:485</code> | <code>&amp;env::var("LAYERX_WEBHOOKS_IDENTITY_URL")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-ingress-client-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:136</code> | <code>let client_ca = env::var("LAYERX_WEBHOOKS_INGRESS_CLIENT_CA_DER")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-instance-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:773</code> | <code>let instance = env::var("LAYERX_WEBHOOKS_INSTANCE_ID")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-internal-ca-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:205</code> | <code>env::var("LAYERX_WEBHOOKS_INTERNAL_CA_DER")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-key-overlap-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:794</code> | <code>key_overlap_seconds: number("LAYERX_WEBHOOKS_KEY_OVERLAP_SECONDS", 86_400)?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-kms-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:771</code> | <code>token: read_secret("LAYERX_WEBHOOKS_KMS_TOKEN_FILE")?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-kms-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:768</code> | <code>&amp;env::var("LAYERX_WEBHOOKS_KMS_URL")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-lease-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:696</code> | <code>in_flight_timeout_seconds: number("LAYERX_WEBHOOKS_LEASE_SECONDS", 120)?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-listen</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/env-init.sh:37</code> | <code>export LAYERX_WEBHOOKS_ROLE=public LAYERX_WEBHOOKS_LISTENER=tls LAYERX_WEBHOOKS_LISTEN="${LAYERX_WEBHOOKS_LISTEN:-&lt;addr&gt;}" \</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-listener</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:134</code> | <code>return match env::var("LAYERX_WEBHOOKS_LISTENER").as_deref() {</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-lxp-wire-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:241</code> | <code>let wire_version = bounded_env("LAYERX_WEBHOOKS_LXP_WIRE_VERSION", 32)?;</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-maximum-attempts-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:687</code> | <code>maximum_attempts: u32::try_from(number("LAYERX_WEBHOOKS_MAXIMUM_ATTEMPTS", 8)?)</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-maximum-delay-seconds-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:686</code> | <code>maximum_delay_seconds: number("LAYERX_WEBHOOKS_MAXIMUM_DELAY_SECONDS", 3_600)?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:266</code> | <code>network_id: bounded_env("LAYERX_WEBHOOKS_NETWORK_ID", 64)?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-operator-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:610</code> | <code>token: read_secret("LAYERX_WEBHOOKS_OPERATOR_TOKEN_FILE")?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-public-ca-der-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:745</code> | <code>let public_ca = Certificate::from_der(&amp;read_required("LAYERX_WEBHOOKS_PUBLIC_CA_DER")?)</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-redis-password-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:708</code> | <code>password: read_secret("LAYERX_WEBHOOKS_REDIS_PASSWORD_FILE")?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-redis-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:703</code> | <code>&amp;env::var("LAYERX_WEBHOOKS_REDIS_URL")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-redis-username-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:707</code> | <code>username: read_secret("LAYERX_WEBHOOKS_REDIS_USERNAME_FILE")?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-retention-events-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:239</code> | <code>retention_events: usize::try_from(number("LAYERX_WEBHOOKS_RETENTION_EVENTS", 10_000)?)</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-role</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:109</code> | <code>let ingress = match env::var("LAYERX_WEBHOOKS_ROLE").as_deref() {</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-sequencer-first-batch-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:262</code> | <code>read_secret("LAYERX_WEBHOOKS_SEQUENCER_FIRST_BATCH_FILE")?.as_str(),</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-sequencer-id-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:260</code> | <code>read_secret("LAYERX_WEBHOOKS_SEQUENCER_ID_FILE")?.as_str(),</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-sequencer-last-batch-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:263</code> | <code>read_secret("LAYERX_WEBHOOKS_SEQUENCER_LAST_BATCH_FILE")?.as_str(),</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-sequencer-public-key-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:240</code> | <code>let trusted = read_secret("LAYERX_WEBHOOKS_SEQUENCER_PUBLIC_KEY_FILE")?;</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-source-trigger-token-file-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:602</code> | <code>token: read_secret("LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE")?,</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-spread-percent-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:689</code> | <code>spread_percent: u8::try_from(number("LAYERX_WEBHOOKS_SPREAD_PERCENT", 20)?)</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-suspend-after-dead-letters-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:692</code> | <code>"LAYERX_WEBHOOKS_SUSPEND_AFTER_DEAD_LETTERS",</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-tls-cert-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:163</code> | <code>env::var("LAYERX_WEBHOOKS_TLS_CERT_DER")</code> |
| <code>interface.8.env-webhooks-layerx-webhooks-tls-key-der</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:170</code> | <code>env::var("LAYERX_WEBHOOKS_TLS_KEY_DER").map_err(&#124;_&#124; "webhook TLS key is required")?,</code> |
| <code>interface.8.env-webhooks-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/env-init.sh:29</code> | <code>elif [ -n "${PORT:-}" ]; then</code> |
| <code>interface.8.env-webhooks-vendor-replica-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/env-init.sh:20</code> | <code>LAYERX_WEBHOOKS_INSTANCE_ID=${&lt;vendor&gt;_REPLICA_ID:-$(hostname)}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-webhooks-layerx-dashboard-client-identity-password-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:503 |
| <code>interface.8.env-webhooks-layerx-dashboard-identity-token-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:519 |
| <code>interface.8.env-webhooks-layerx-role</code> | notes | 16 callers total |
| <code>interface.8.env-webhooks-layerx-webhooks-authority-token-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:258; 10 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-base-delay-seconds-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:685 |
| <code>interface.8.env-webhooks-layerx-webhooks-client-identity-password-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:738; 9 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-component-token-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:253; 11 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-cursor-key-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:747 |
| <code>interface.8.env-webhooks-layerx-webhooks-dispatch-budget-j69</code> | notes | read at services/platform/hosted/webhooks/src/main.rs:237 |
| <code>interface.8.env-webhooks-layerx-webhooks-dispatch-interval-seconds-j69</code> | notes | read at services/platform/hosted/webhooks/src/main.rs:234 |
| <code>interface.8.env-webhooks-layerx-webhooks-key-overlap-seconds-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:794 |
| <code>interface.8.env-webhooks-layerx-webhooks-kms-token-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:771; 11 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-lease-seconds-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:696 |
| <code>interface.8.env-webhooks-layerx-webhooks-maximum-attempts-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:687 |
| <code>interface.8.env-webhooks-layerx-webhooks-maximum-delay-seconds-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:686 |
| <code>interface.8.env-webhooks-layerx-webhooks-operator-token-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:610; 14 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-public-ca-der-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:745; 9 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-redis-password-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:708; 10 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-redis-username-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:707; 10 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-retention-events-j69</code> | notes | read at services/platform/hosted/webhooks/src/main.rs:239 |
| <code>interface.8.env-webhooks-layerx-webhooks-role</code> | notes | 27 callers total |
| <code>interface.8.env-webhooks-layerx-webhooks-sequencer-first-batch-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:262 |
| <code>interface.8.env-webhooks-layerx-webhooks-sequencer-id-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:260 |
| <code>interface.8.env-webhooks-layerx-webhooks-sequencer-last-batch-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:263 |
| <code>interface.8.env-webhooks-layerx-webhooks-sequencer-public-key-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:240; 9 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-source-trigger-token-file-j69</code> | notes | read at services/platform/hosted/webhooks/src/trusted.rs:602; 15 in-repo references, first 8 listed |
| <code>interface.8.env-webhooks-layerx-webhooks-spread-percent-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:689 |
| <code>interface.8.env-webhooks-layerx-webhooks-suspend-after-dead-letters-j69</code> | notes | read at services/platform/hosted/webhooks/src/hosted.rs:692 |
| <code>interface.8.env-webhooks-port</code> | notes | system variable; callers not enumerated |
| <code>interface.8.env-webhooks-vendor-replica-id</code> | notes | no in-repo setter or reader outside the defining file |

## webhooks-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-webhooks-admission-deadline-exceeded</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:482</code> | <code>return Reply::refusal(503, "admission_deadline_exceeded", None);</code> |
| <code>interface.8.err-webhooks-body-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:467</code> | <code>return Reply::refusal(400, "body_not_allowed", None);</code> |
| <code>interface.8.err-webhooks-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:252</code> | <code>WebhookError::EventConflict =&gt; (409, "conflict", None),</code> |
| <code>interface.8.err-webhooks-cursor-expired</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:255</code> | <code>WebhookError::CursorExpired =&gt; (410, "cursor_expired", None),</code> |
| <code>interface.8.err-webhooks-delivery-state-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:504</code> | <code>Ok(_) =&gt; Reply::refusal(503, "delivery_state_unavailable", None),</code> |
| <code>interface.8.err-webhooks-dependency-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:263</code> | <code>(503, "dependency_unavailable", Some(5))</code> |
| <code>interface.8.err-webhooks-encoding-failed</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:272</code> | <code>&#124;_&#124; Reply::refusal(503, "encoding_failed", Some(5)),</code> |
| <code>interface.8.err-webhooks-endpoint-suspended</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:251</code> | <code>WebhookError::EndpointSuspended =&gt; (409, "endpoint_suspended", None),</code> |
| <code>interface.8.err-webhooks-entropy-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:261</code> | <code>WebhookError::Entropy =&gt; (503, "entropy_unavailable", Some(5)),</code> |
| <code>interface.8.err-webhooks-idempotency-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:312</code> | <code>return Reply::refusal(400, "idempotency_key_required", None);</code> |
| <code>interface.8.err-webhooks-invalid-admission-deadline</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:479</code> | <code>return Reply::refusal(400, "invalid_admission_deadline", None);</code> |
| <code>interface.8.err-webhooks-invalid-cursor</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:254</code> | <code>WebhookError::InvalidCursor =&gt; (400, "invalid_cursor", None),</code> |
| <code>interface.8.err-webhooks-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:247</code> | <code>WebhookError::InvalidRequest =&gt; (400, "invalid_request", None),</code> |
| <code>interface.8.err-webhooks-not-dead-lettered</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:250</code> | <code>WebhookError::NotDeadLettered =&gt; (409, "not_dead_lettered", None),</code> |
| <code>interface.8.err-webhooks-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:309</code> | <code>return Reply::refusal(404, "not_found", None);</code> |
| <code>interface.8.err-webhooks-operator-authentication-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:525</code> | <code>return Reply::refusal(401, "operator_authentication_required", None);</code> |
| <code>interface.8.err-webhooks-operator-role-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:522</code> | <code>return Reply::refusal(403, "operator_role_required", None);</code> |
| <code>interface.8.err-webhooks-order-violation</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:253</code> | <code>WebhookError::OrderViolation =&gt; (409, "order_violation", None),</code> |
| <code>interface.8.err-webhooks-peer-role-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:616</code> | <code>return Reply::refusal(403, "peer_role_refused", None);</code> |
| <code>interface.8.err-webhooks-producer-role-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:461</code> | <code>return Reply::refusal(403, "producer_role_required", None);</code> |
| <code>interface.8.err-webhooks-replay-capacity</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:260</code> | <code>WebhookError::ReplayCapacity =&gt; (503, "replay_capacity", Some(10)),</code> |
| <code>interface.8.err-webhooks-replay-rejected</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:258</code> | <code>WebhookError::ReplayRejected =&gt; (409, "replay_rejected", None),</code> |
| <code>interface.8.err-webhooks-session-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:604</code> | <code>Err(_) =&gt; Reply::refusal(401, "session_required", None),</code> |
| <code>interface.8.err-webhooks-signature-rejected</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:257</code> | <code>WebhookError::SignatureRejected =&gt; (401, "signature_rejected", None),</code> |
| <code>interface.8.err-webhooks-source-authentication-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:464</code> | <code>return Reply::refusal(401, "source_authentication_required", None);</code> |
| <code>interface.8.err-webhooks-stale-timestamp</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:259</code> | <code>WebhookError::StaleTimestamp =&gt; (400, "stale_timestamp", None),</code> |
| <code>interface.8.err-webhooks-unknown-delivery</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:249</code> | <code>WebhookError::UnknownDelivery =&gt; (404, "unknown_delivery", None),</code> |
| <code>interface.8.err-webhooks-unknown-endpoint</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:248</code> | <code>WebhookError::UnknownEndpoint =&gt; (404, "unknown_endpoint", None),</code> |
| <code>interface.8.err-webhooks-verification-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:265</code> | <code>WebhookError::Gateway(_) =&gt; (422, "verification_refused", None),</code> |
| <code>interface.8.err-webhooks-verification-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:256</code> | <code>WebhookError::VerificationRequired =&gt; (422, "verification_required", None),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-webhooks-admission-deadline-exceeded</code> | notes | HTTP 503 |
| <code>interface.8.err-webhooks-body-not-allowed</code> | notes | HTTP 400 |
| <code>interface.8.err-webhooks-conflict</code> | notes | HTTP 409; 135 callers total |
| <code>interface.8.err-webhooks-cursor-expired</code> | notes | HTTP 410; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-delivery-state-unavailable</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-dependency-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-webhooks-encoding-failed</code> | notes | HTTP 503 |
| <code>interface.8.err-webhooks-endpoint-suspended</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-entropy-unavailable</code> | notes | HTTP 503 |
| <code>interface.8.err-webhooks-idempotency-key-required</code> | notes | HTTP 400 |
| <code>interface.8.err-webhooks-invalid-admission-deadline</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-invalid-cursor</code> | notes | HTTP 400 |
| <code>interface.8.err-webhooks-invalid-request</code> | notes | HTTP 400; 76 callers total |
| <code>interface.8.err-webhooks-not-dead-lettered</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-not-found</code> | notes | HTTP 404; 129 callers total |
| <code>interface.8.err-webhooks-operator-authentication-required</code> | notes | HTTP 401 |
| <code>interface.8.err-webhooks-operator-role-required</code> | notes | HTTP 403 |
| <code>interface.8.err-webhooks-order-violation</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-peer-role-refused</code> | notes | HTTP 403 |
| <code>interface.8.err-webhooks-producer-role-required</code> | notes | HTTP 403 |
| <code>interface.8.err-webhooks-replay-capacity</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-replay-rejected</code> | notes | HTTP 409; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-session-required</code> | notes | HTTP 401 |
| <code>interface.8.err-webhooks-signature-rejected</code> | notes | HTTP 401; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-source-authentication-required</code> | notes | HTTP 401 |
| <code>interface.8.err-webhooks-stale-timestamp</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-unknown-delivery</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-unknown-endpoint</code> | notes | HTTP 404; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-verification-refused</code> | notes | HTTP 422; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-webhooks-verification-required</code> | notes | HTTP 422 |

## webhooks-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-webhooks-ai-market-state-lock</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/ai_market.rs:69</code> | <code>const LOCK_FILE: &amp;str = "state.lock";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-webhooks-ai-market-state-lock</code> | notes | AI market state lock; read and written only by the defining module |

## webhooks-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-webhooks-any-internal</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:612</code> | <code>if !request.path.starts_with("/internal/") {</code> |
| <code>interface.8.http-webhooks-any-v1-webhooks</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:599</code> | <code>if !request.path.starts_with("/v1/webhooks/") {</code> |
| <code>interface.8.http-webhooks-get-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:580</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/healthz" {</code> |
| <code>interface.8.http-webhooks-get-v1-webhooks-scheme</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/main.rs:588</code> | <code>if request.method == "GET" &amp;&amp; request.path == "/v1/webhooks/scheme" {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-webhooks-any-internal</code> | notes | 4450 callers total |
| <code>interface.8.http-webhooks-any-v1-webhooks</code> | notes | 54 callers total |
| <code>interface.8.http-webhooks-get-healthz</code> | notes | 330 callers total |

## webhooks-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-deliveries-deliveryrecord</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/deliveries.rs:237</code> | <code>pub struct DeliveryRecord {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-encoding-hex-encode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/encoding.rs:13</code> | <code>pub fn hex_encode(bytes: &amp;[u8]) -&gt; String {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-endpoints-endpointhealth</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/endpoints.rs:81</code> | <code>pub struct EndpointHealth {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-error-webhookerror</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/error.rs:11</code> | <code>pub enum WebhookError {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-events-endpointid</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/events.rs:195</code> | <code>pub struct EndpointId(String);</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-events-eventkind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/events.rs:87</code> | <code>pub enum EventKind {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-events-principal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/events.rs:149</code> | <code>pub struct Principal(String);</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-events-protocolevent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/events.rs:463</code> | <code>pub struct ProtocolEvent {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-events-protocolfact</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/events.rs:339</code> | <code>pub struct ProtocolFact {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-events-verification</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/platform/hosted/webhooks/src/events.rs:20</code> | <code>pub enum Verification {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-hostedreader</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/hosted.rs:673</code> | <code>pub struct HostedReader {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-http</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/lib.rs:22</code> | <code>pub mod http;</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-http-reply</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/http.rs:53</code> | <code>pub struct Reply {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-http-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/http.rs:12</code> | <code>pub struct Request {</code> |
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-trusted-developeridentity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/hosted/webhooks/src/trusted.rs:131</code> | <code>pub struct DeveloperIdentity {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rs-webhooks-layerx-platform-webhooks-events-verification</code> | overridden_by | override.J40.100 |

Contract context <code>contract_context.8.webhook-verification</code>:

| Field | Frozen rule |
|---|---|
| interface | Verification word of webhook events and subscription minimums |
| storage | none |
| errors | invalid_request for a word outside the vocabulary |
| events | webhook deliveries carry the verification word |
| gas | none |
| activation | block-committed accepted in wave 1; kernel words refused in wave 6 |
| invariants | a value never claims a level stronger than its weakest evidence; the lattice order is unverified &lt; receipt-verified &lt; block-committed |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.145.1</code> | req | 8145.1 |
| <code>acceptance.8.145.1</code> | scenario | Given every Verification value, when each is rendered, parsed and compared with at_least, then block-committed round-trips and sits strictly between receipt-verified and checkpoint-finalised |
| <code>acceptance.8.145.1</code> | test | services/platform/hosted/webhooks/src/events.rs::block_committed_orders_between_receipt_and_checkpoint (planned) |
| <code>acceptance.8.145.1</code> | qualification | UNRUN |
| <code>req.8145</code> | ac_1 | Verification::BlockCommitted round-trips through as_str and parse as block-committed and orders above ReceiptVerified and below CheckpointFinalised, so at_least keeps its lattice meaning. |
| <code>acceptance.8.146.1</code> | req | 8146.1 |
| <code>acceptance.8.146.1</code> | scenario | Given the two retired words and the wire ranks 0 to 6, when each is parsed or projected, then both words are refused, ranks 4 and 5 are block-committed, 1 to 3 receipt-verified and the rest unverified |
| <code>acceptance.8.146.1</code> | test | services/platform/hosted/webhooks/src/events.rs::kernel_finality_words_are_refused (planned) |
| <code>acceptance.8.146.1</code> | qualification | UNRUN |
| <code>req.8146</code> | ac_1 | Verification::parse refuses checkpoint-finalised and paxeer-finalised, and from_wire_rank maps ranks 4 and 5 to BlockCommitted and ranks 1 to 3 to ReceiptVerified. |
| <code>capability.8.13.1</code> | title | Add the block-committed verification word to webhook events |
| <code>capability.8.13.1</code> | reqs | 8145.1 |
| <code>capability.8.13.2</code> | title | Retire the checkpoint and anchor finality words of webhook events |
| <code>capability.8.13.2</code> | reqs | 8146.1 |
| <code>resolution.17.808</code> | title | decision.813 binds: Webhook finality on the merged chain is the committed block |
| <code>resolution.17.808</code> | applies_to | decision.813<br>task.8.13.1<br>task.8.13.2 |
| <code>resolution.17.808</code> | source | decision.813 in fragments/J40/decision.kvx; tasks from affects |
| <code>service.platform-webhooks</code> | purpose | Signs and delivers protocol event webhooks to registered endpoints |
| <code>service.platform-webhooks</code> | route | webhook endpoint management and delivery routes of layerx-webhooks |
| <code>service.platform-webhooks</code> | inputs | protocol facts with verification levels, endpoint registrations |
| <code>service.platform-webhooks</code> | outputs | signed webhook deliveries |
| <code>service.platform-webhooks</code> | state | delivery and endpoint store |
| <code>service.platform-webhooks</code> | dependencies | agent boundary internal receipts, KMS signing |
| <code>service.platform-webhooks</code> | failure_policy | a delivery below the endpoint's minimum verification is held, never sent |
| <code>service.platform-webhooks</code> | completion_task | 8.0.9 |
| <code>service.platform-webhooks</code> | operation_task | 8.13.2 |
| <code>service.platform-webhooks</code> | domain | 8 |
