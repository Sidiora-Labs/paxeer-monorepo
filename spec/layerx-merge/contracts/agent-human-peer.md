# Agent human peer contract

This page describes the authenticated local peer between the Human service and `layerx-agentd` in the merged Paxeer network.

Paxeer is one BFT network whose kernel participates in block proposal and execution. A receipt is final when its block is committed. See the [network audit brief](../programme/audit/BRIEF.kvx) and [public node endpoints](../../../docs/site/docs/reference/public-rpc.md) for network access; the opcodes below describe the local Human peer protocol.

## Framing and transport

The agent declares the opcodes in [human.rs](../../../services/agent/crates/layerx-agentd/src/human.rs), and the Human service emits requests through [AgentRuntime](../../../services/human/crates/layerx-human-service/src/server/agent_runtime.rs). An ordinary request begins with `LXHAGT01` and a one-byte opcode. The `LXHAGT02` trace envelope carries a 36-byte trace identifier (`trc_` plus 32 lowercase hexadecimal characters) and a length-prefixed inner `LXHAGT01` request; `trace_envelope` validates it before decoding.

[human_frame.rs](../../../services/agent/crates/layerx-agentd/src/human_frame.rs) re-exports the shared `layerx-client` framing functions and aliases its `FrameTransport` trait as `HumanFrameTransport`. The [shared framing implementation](../../../services/agent/crates/layerx-client/src/frame/framing.rs) uses a four-byte big-endian payload length and enforces the configured maximum before allocating the body. `HumanUnixServer` authenticates Unix peer credentials against its configured UID-to-principal-and-tenant mapping before processing request frames. `AgentRuntime` opens a bounded Unix-socket exchange for each call.

The catalogue lists wire constants and selected producer/decoder behavior. Payload types and validation are defined by `HumanRequest`, `decode_request`, and the `decode_operation` functions in `human.rs`. Payload types and validation apply independently of whether a producer uses an opcode.

## account-sequence

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-account-sequence</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 13 ACCOUNT_SEQUENCE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-account-sequence</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## account-state

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-account-state</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 42 ACCOUNT_STATE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-account-state</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-archive

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-archive</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 29 AGENT_ARCHIVE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-archive</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-budget-proof-v4

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-budget-proof-v4</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 57 AGENT_BUDGET_PROOF_V4</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-budget-proof-v4</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 57 AGENT_BUDGET_PROOF. |

## agent-budget-state

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-budget-state</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 32 AGENT_BUDGET_STATE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-budget-state</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-context

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-context</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 31 AGENT_CONTEXT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-context</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-control

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-control</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 26 AGENT_CONTROL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-control</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-get

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-get</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 25 AGENT_GET</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-get</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-journey

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-journey</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 28 AGENT_JOURNEY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-journey</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The Human service emits opcode 28 from `agent_reclaim` and `agent_key_change`. The agent decoder (`human.rs`) reads a journey tag: 0 for reclaim, 1 for rotate, and 2 for recover, then validates the fields for that tag. Completed owner rotation uses [opcode 43](#agent-owner-rotated). |

## agent-key-policy

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-key-policy</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 33 AGENT_KEY_POLICY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-key-policy</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-lifecycle-publish

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-lifecycle-publish</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 37 AGENT_LIFECYCLE_PUBLISH</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-lifecycle-publish</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The Human service producer `publish_lifecycle` (`agent_runtime.rs`) sends opcode 37 with request ID, idempotency key, body digest, and lifecycle seed. The decoder (`human.rs`) constructs `HumanRequest::AgentLifecyclePublish` with a `MutationEnvelope`. |

## agent-limit

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-limit</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 27 AGENT_LIMIT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-limit</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-list

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-list</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 24 AGENT_LIST</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-list</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-owner-rotated

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-owner-rotated</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 43 AGENT_OWNER_ROTATED</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-owner-rotated</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The Human service producer `AgentRuntime::agent_owner_rotated` (`agent_runtime.rs`) writes opcode 43 with `Writer::new(43)`, followed by agent ID, custody key, signed activity bytes, and finalization evidence. The agent decoder (`human.rs`) maps it to `HumanRequest::AgentJourney` with `HumanAgentJourneyKind::OwnerRotated`. The production rotation path calls this producer from `server/production_rotation.rs:269`. |

## agent-session-bind

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-session-bind</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 36 AGENT_SESSION_BIND</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-bind</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-session-restrict

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-session-restrict</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 38 AGENT_SESSION_RESTRICT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-restrict</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-session-snapshot

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-session-snapshot</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 34 AGENT_SESSION_SNAPSHOT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-snapshot</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## agent-session-suspend

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-agent-session-suspend</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 35 AGENT_SESSION_SUSPEND</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-suspend</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## approval-approve

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-approval-approve</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 11 APPROVAL_APPROVE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-approve</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## approval-budget-after-v2

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-approval-budget-after-v2</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 48 APPROVAL_BUDGET_AFTER_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-budget-after-v2</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 48 APPROVAL_BUDGET_AFTER. |

## approval-get

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-approval-get</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 10 APPROVAL_GET</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-get</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## approval-get-facts-v2

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-approval-get-facts-v2</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 47 APPROVAL_GET_FACTS_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-get-facts-v2</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 47 APPROVAL_GET_FACTS. |

## approval-list

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-approval-list</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 9 APPROVAL_LIST</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-list</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## approval-list-facts-v2

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-approval-list-facts-v2</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 46 APPROVAL_LIST_FACTS_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-list-facts-v2</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 46 APPROVAL_LIST_FACTS. |

## approval-reject

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-approval-reject</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 12 APPROVAL_REJECT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-reject</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## balance

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-balance</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 6 BALANCE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-balance</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## capability-install

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-capability-install</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 30 CAPABILITY_INSTALL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-capability-install</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## evidence

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-evidence</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 8 EVIDENCE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-evidence</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## head

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-head</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 7 HEAD</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-head</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## identity-resolve

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-identity-resolve</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 20 IDENTITY_RESOLVE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-identity-resolve</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## lease-map

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-lease-map</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 21 LEASE_MAP</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-lease-map</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## managed-evidence-by-digest-v2

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-managed-evidence-by-digest-v2</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 49 MANAGED_EVIDENCE_BY_DIGEST_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-managed-evidence-by-digest-v2</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 49 MANAGED_EVIDENCE. |

## native-approval-get-facts-v2

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-approval-get-facts-v2</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 51 NATIVE_APPROVAL_GET_FACTS_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-approval-get-facts-v2</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 51 NATIVE_APPROVAL_GET_FACTS. |

## native-approval-list-facts-v2

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-approval-list-facts-v2</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 50 NATIVE_APPROVAL_LIST_FACTS_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-approval-list-facts-v2</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 50 NATIVE_APPROVAL_LIST_FACTS. |

## native-effect-approval-budget-v4

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-effect-approval-budget-v4</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 56 NATIVE_EFFECT_APPROVAL_BUDGET_V4</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-budget-v4</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 56 NATIVE_EFFECT_APPROVAL_BUDGET. |

## native-effect-approval-decide-v3

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-effect-approval-decide-v3</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 54 NATIVE_EFFECT_APPROVAL_DECIDE_V3</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-decide-v3</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 54 NATIVE_EFFECT_APPROVAL_DECIDE. |

## native-effect-approval-get-v3

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-effect-approval-get-v3</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 53 NATIVE_EFFECT_APPROVAL_GET_V3</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-get-v3</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 53 NATIVE_EFFECT_APPROVAL_GET. |

## native-effect-approval-list-v3

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-effect-approval-list-v3</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 52 NATIVE_EFFECT_APPROVAL_LIST_V3</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-list-v3</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 52 NATIVE_EFFECT_APPROVAL_LIST. |

## native-effect-approval-material-v4

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-effect-approval-material-v4</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 55 NATIVE_EFFECT_APPROVAL_MATERIAL_V4</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-material-v4</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 55 NATIVE_EFFECT_APPROVAL_MATERIAL. |

## native-fee-policy

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-fee-policy</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 39 NATIVE_FEE_POLICY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-fee-policy</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## native-journey-envelope-v1

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-journey-envelope-v1</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 65 NATIVE_JOURNEY_ENVELOPE_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-journey-envelope-v1</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The agent decoder (`human.rs`) accepts opcode 65 with payload version 1 and maps it to `HumanRequest::NativeJourneyEnvelopeV1`. The Human service native journey submit/admit paths emit [opcode 67](#native-send-envelope-v1). |

## native-owner-context-v1

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-owner-context-v1</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 64 NATIVE_OWNER_CONTEXT_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-owner-context-v1</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The Human service producer `native_send_owner_context` (`agent_runtime.rs`) emits opcode 64 using `Writer::new(64)`. The agent decodes it in `decode_operation` (`human.rs`). |

## native-program-approval-budget-v5

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-program-approval-budget-v5</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 61 NATIVE_PROGRAM_APPROVAL_BUDGET_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-budget-v5</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 61 NATIVE_PROGRAM_APPROVAL_BUDGET. |

## native-program-approval-decide-v5

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-program-approval-decide-v5</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 62 NATIVE_PROGRAM_APPROVAL_DECIDE_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-decide-v5</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 62 NATIVE_PROGRAM_APPROVAL_DECIDE. |

## native-program-approval-get-v5

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-program-approval-get-v5</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 59 NATIVE_PROGRAM_APPROVAL_GET_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-get-v5</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 59 NATIVE_PROGRAM_APPROVAL_GET. |

## native-program-approval-list-v5

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-program-approval-list-v5</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 58 NATIVE_PROGRAM_APPROVAL_LIST_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-list-v5</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 58 NATIVE_PROGRAM_APPROVAL_LIST. |

## native-program-approval-material-v5

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-program-approval-material-v5</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 60 NATIVE_PROGRAM_APPROVAL_MATERIAL_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-material-v5</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The human service names opcode 60 NATIVE_PROGRAM_APPROVAL_MATERIAL. |

## native-send-envelope-v1

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-send-envelope-v1</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 67 NATIVE_SEND_ENVELOPE_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-send-envelope-v1</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The Human service uses opcode 67 in `native_journey_submit` (`agent_runtime.rs`) and `native_journey_admit` (`agent_runtime.rs`) to carry a version-1 encoded agent envelope. The agent decoder (`human.rs`) accepts opcode 67 as `HumanRequest::NativeSendEnvelopeV1`. |

## native-send-owner-preview-v1

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-send-owner-preview-v1</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 66 NATIVE_SEND_OWNER_PREVIEW_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-send-owner-preview-v1</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The Human service producer `native_send_preview_v2` (`agent_runtime.rs`) emits opcode 66 and payload version 1. The agent decodes it in `decode_operation` (`human.rs`). The producer method name and the wire constant retain their respective version suffixes. |

## native-send-preview-v1

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-native-send-preview-v1</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 63 NATIVE_SEND_PREVIEW_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-send-preview-v1</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The Human service producer `native_send_preview` (`agent_runtime.rs`) emits opcode 63 using `Writer::new(63)`. The agent decodes it in `decode_operation` (`human.rs`). |

## operator

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-operator</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 45 OPERATOR</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-operator</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The agent decoder (`human.rs`) accepts opcode 45, reads the operator ID, request ID, command code, and target, validates those fields, and constructs `HumanRequest::Operator` through `operator_command`. The Human service `agent_runtime.rs` has no named opcode-45 constant; this row documents the agent decoder contract. |

## owner-install

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-owner-install</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 23 OWNER_INSTALL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-owner-install</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## owner-validate

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-owner-validate</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 22 OWNER_VALIDATE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-owner-validate</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## prepare

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-prepare</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 1 PREPARE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-prepare</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## receipt-lookup

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-receipt-lookup</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 4 RECEIPT_LOOKUP</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-receipt-lookup</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## registry

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-registry</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 5 REGISTRY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-registry</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## session-fee-state

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-session-fee-state</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 40 SESSION_FEE_STATE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-session-fee-state</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## session-seed-prepare

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-session-seed-prepare</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 41 SESSION_SEED_PREPARE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-session-seed-prepare</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## subject

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-subject</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 44 SUBJECT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-subject</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. The Human service `scoped_request` (`agent_runtime.rs`) emits opcode 44 with principal, owner, canonical account, asset ID, and the inner request. `decode_request` (`human.rs`) parses the subject wrapper, rejects a nested subject wrapper or zero asset ID, validates the owner/account relationship, and decodes the inner request. |

## submit

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-submit</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 2 SUBMIT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-submit</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |

## track

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.proto-human-peer-track</code> | <code>proto_message</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>human peer opcode 3 TRACK</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-track</code> | notes | Uses the [framing and shared transport](#framing-and-transport) described above. |
