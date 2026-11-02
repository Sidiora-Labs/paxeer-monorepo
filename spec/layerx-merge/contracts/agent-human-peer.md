# Agent human peer contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## account-sequence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-account-sequence</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:88</code> | <code>human peer opcode 13 ACCOUNT_SEQUENCE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-account-sequence</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## account-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-account-state</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:93</code> | <code>human peer opcode 42 ACCOUNT_STATE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-account-state</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-archive

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-archive</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:78</code> | <code>human peer opcode 29 AGENT_ARCHIVE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-archive</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-budget-proof-v4

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-budget-proof-v4</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:112</code> | <code>human peer opcode 57 AGENT_BUDGET_PROOF_V4</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-budget-proof-v4</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 57 AGENT_BUDGET_PROOF. |

## agent-budget-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-budget-state</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:81</code> | <code>human peer opcode 32 AGENT_BUDGET_STATE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-budget-state</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-context

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-context</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:80</code> | <code>human peer opcode 31 AGENT_CONTEXT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-context</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-control

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-control</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:74</code> | <code>human peer opcode 26 AGENT_CONTROL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-control</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-get</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:73</code> | <code>human peer opcode 25 AGENT_GET</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-get</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-journey

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-journey</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:77</code> | <code>human peer opcode 28 AGENT_JOURNEY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-journey</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-key-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-key-policy</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:82</code> | <code>human peer opcode 33 AGENT_KEY_POLICY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-key-policy</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-lifecycle-publish

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-lifecycle-publish</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:86</code> | <code>human peer opcode 37 AGENT_LIFECYCLE_PUBLISH</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-lifecycle-publish</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-limit</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:75</code> | <code>human peer opcode 27 AGENT_LIMIT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-limit</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-list</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:72</code> | <code>human peer opcode 24 AGENT_LIST</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-list</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-owner-rotated

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-owner-rotated</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:76</code> | <code>human peer opcode 43 AGENT_OWNER_ROTATED</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-owner-rotated</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service client (services/human/crates/layerx-human-service/src/server/agent_runtime.rs) defines no opcode 43. |

## agent-session-bind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-bind</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:85</code> | <code>human peer opcode 36 AGENT_SESSION_BIND</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-bind</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-session-restrict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-restrict</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:87</code> | <code>human peer opcode 38 AGENT_SESSION_RESTRICT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-restrict</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-session-snapshot

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-snapshot</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:83</code> | <code>human peer opcode 34 AGENT_SESSION_SNAPSHOT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-snapshot</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## agent-session-suspend

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-suspend</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:84</code> | <code>human peer opcode 35 AGENT_SESSION_SUSPEND</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-agent-session-suspend</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## approval-approve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-approval-approve</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:64</code> | <code>human peer opcode 11 APPROVAL_APPROVE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-approve</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## approval-budget-after-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-approval-budget-after-v2</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:98</code> | <code>human peer opcode 48 APPROVAL_BUDGET_AFTER_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-budget-after-v2</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 48 APPROVAL_BUDGET_AFTER. |

## approval-get

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-approval-get</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:63</code> | <code>human peer opcode 10 APPROVAL_GET</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-get</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## approval-get-facts-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-approval-get-facts-v2</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:97</code> | <code>human peer opcode 47 APPROVAL_GET_FACTS_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-get-facts-v2</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 47 APPROVAL_GET_FACTS. |

## approval-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-approval-list</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:62</code> | <code>human peer opcode 9 APPROVAL_LIST</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-list</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## approval-list-facts-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-approval-list-facts-v2</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:96</code> | <code>human peer opcode 46 APPROVAL_LIST_FACTS_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-list-facts-v2</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 46 APPROVAL_LIST_FACTS. |

## approval-reject

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-approval-reject</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:65</code> | <code>human peer opcode 12 APPROVAL_REJECT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-approval-reject</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## balance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-balance</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:89</code> | <code>human peer opcode 6 BALANCE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-balance</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## capability-install

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-capability-install</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:79</code> | <code>human peer opcode 30 CAPABILITY_INSTALL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-capability-install</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-evidence</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:119</code> | <code>human peer opcode 8 EVIDENCE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-evidence</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## head

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-head</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:118</code> | <code>human peer opcode 7 HEAD</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-head</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## identity-resolve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-identity-resolve</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:68</code> | <code>human peer opcode 20 IDENTITY_RESOLVE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-identity-resolve</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## lease-map

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-lease-map</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:69</code> | <code>human peer opcode 21 LEASE_MAP</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-lease-map</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## managed-evidence-by-digest-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-managed-evidence-by-digest-v2</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:99</code> | <code>human peer opcode 49 MANAGED_EVIDENCE_BY_DIGEST_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-managed-evidence-by-digest-v2</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 49 MANAGED_EVIDENCE. |

## native-approval-get-facts-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-approval-get-facts-v2</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:101</code> | <code>human peer opcode 51 NATIVE_APPROVAL_GET_FACTS_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-approval-get-facts-v2</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 51 NATIVE_APPROVAL_GET_FACTS. |

## native-approval-list-facts-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-approval-list-facts-v2</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:100</code> | <code>human peer opcode 50 NATIVE_APPROVAL_LIST_FACTS_V2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-approval-list-facts-v2</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 50 NATIVE_APPROVAL_LIST_FACTS. |

## native-effect-approval-budget-v4

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-budget-v4</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:111</code> | <code>human peer opcode 56 NATIVE_EFFECT_APPROVAL_BUDGET_V4</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-budget-v4</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 56 NATIVE_EFFECT_APPROVAL_BUDGET. |

## native-effect-approval-decide-v3

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-decide-v3</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:109</code> | <code>human peer opcode 54 NATIVE_EFFECT_APPROVAL_DECIDE_V3</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-decide-v3</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 54 NATIVE_EFFECT_APPROVAL_DECIDE. |

## native-effect-approval-get-v3

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-get-v3</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:108</code> | <code>human peer opcode 53 NATIVE_EFFECT_APPROVAL_GET_V3</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-get-v3</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 53 NATIVE_EFFECT_APPROVAL_GET. |

## native-effect-approval-list-v3

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-list-v3</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:107</code> | <code>human peer opcode 52 NATIVE_EFFECT_APPROVAL_LIST_V3</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-list-v3</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 52 NATIVE_EFFECT_APPROVAL_LIST. |

## native-effect-approval-material-v4

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-material-v4</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:110</code> | <code>human peer opcode 55 NATIVE_EFFECT_APPROVAL_MATERIAL_V4</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-effect-approval-material-v4</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 55 NATIVE_EFFECT_APPROVAL_MATERIAL. |

## native-fee-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-fee-policy</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:90</code> | <code>human peer opcode 39 NATIVE_FEE_POLICY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-fee-policy</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## native-journey-envelope-v1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-journey-envelope-v1</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:115</code> | <code>human peer opcode 65 NATIVE_JOURNEY_ENVELOPE_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-journey-envelope-v1</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service client (services/human/crates/layerx-human-service/src/server/agent_runtime.rs) defines no opcode 65. No in-repo reference outside the defining line at this revision. |

## native-owner-context-v1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-owner-context-v1</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:114</code> | <code>human peer opcode 64 NATIVE_OWNER_CONTEXT_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-owner-context-v1</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service client (services/human/crates/layerx-human-service/src/server/agent_runtime.rs) defines no opcode 64. |

## native-program-approval-budget-v5

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-budget-v5</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:105</code> | <code>human peer opcode 61 NATIVE_PROGRAM_APPROVAL_BUDGET_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-budget-v5</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 61 NATIVE_PROGRAM_APPROVAL_BUDGET. |

## native-program-approval-decide-v5

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-decide-v5</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:106</code> | <code>human peer opcode 62 NATIVE_PROGRAM_APPROVAL_DECIDE_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-decide-v5</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 62 NATIVE_PROGRAM_APPROVAL_DECIDE. |

## native-program-approval-get-v5

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-get-v5</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:103</code> | <code>human peer opcode 59 NATIVE_PROGRAM_APPROVAL_GET_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-get-v5</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 59 NATIVE_PROGRAM_APPROVAL_GET. |

## native-program-approval-list-v5

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-list-v5</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:102</code> | <code>human peer opcode 58 NATIVE_PROGRAM_APPROVAL_LIST_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-list-v5</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 58 NATIVE_PROGRAM_APPROVAL_LIST. |

## native-program-approval-material-v5

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-material-v5</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:104</code> | <code>human peer opcode 60 NATIVE_PROGRAM_APPROVAL_MATERIAL_V5</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-program-approval-material-v5</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service names opcode 60 NATIVE_PROGRAM_APPROVAL_MATERIAL. |

## native-send-envelope-v1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-send-envelope-v1</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:117</code> | <code>human peer opcode 67 NATIVE_SEND_ENVELOPE_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-send-envelope-v1</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service client (services/human/crates/layerx-human-service/src/server/agent_runtime.rs) defines no opcode 67. |

## native-send-owner-preview-v1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-send-owner-preview-v1</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:116</code> | <code>human peer opcode 66 NATIVE_SEND_OWNER_PREVIEW_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-send-owner-preview-v1</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service client (services/human/crates/layerx-human-service/src/server/agent_runtime.rs) defines no opcode 66. |

## native-send-preview-v1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-native-send-preview-v1</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:113</code> | <code>human peer opcode 63 NATIVE_SEND_PREVIEW_V1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-native-send-preview-v1</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service client (services/human/crates/layerx-human-service/src/server/agent_runtime.rs) defines no opcode 63. |

## operator

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-operator</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:95</code> | <code>human peer opcode 45 OPERATOR</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-operator</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service client (services/human/crates/layerx-human-service/src/server/agent_runtime.rs) defines no opcode 45. |

## owner-install

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-owner-install</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:71</code> | <code>human peer opcode 23 OWNER_INSTALL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-owner-install</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## owner-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-owner-validate</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:70</code> | <code>human peer opcode 22 OWNER_VALIDATE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-owner-validate</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## prepare

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-prepare</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:57</code> | <code>human peer opcode 1 PREPARE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-prepare</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## receipt-lookup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-receipt-lookup</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:60</code> | <code>human peer opcode 4 RECEIPT_LOOKUP</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-receipt-lookup</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## registry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-registry</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:61</code> | <code>human peer opcode 5 REGISTRY</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-registry</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## session-fee-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-session-fee-state</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:91</code> | <code>human peer opcode 40 SESSION_FEE_STATE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-session-fee-state</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## session-seed-prepare

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-session-seed-prepare</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:92</code> | <code>human peer opcode 41 SESSION_SEED_PREPARE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-session-seed-prepare</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## subject

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-subject</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:94</code> | <code>human peer opcode 44 SUBJECT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-subject</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). The human service client (services/human/crates/layerx-human-service/src/server/agent_runtime.rs) defines no opcode 44. No in-repo reference outside the defining line at this revision. |

## submit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-submit</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:58</code> | <code>human peer opcode 2 SUBMIT</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-submit</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |

## track

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-human-peer-track</code> | <code>proto_message</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:59</code> | <code>human peer opcode 3 TRACK</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-human-peer-track</code> | notes | Opcode of the LXHAGT01/LXHAGT02 human peer frame (services/agent/crates/layerx-agentd/src/human.rs:17-18) carried over the agentd-local HumanFrameTransport (services/agent/crates/layerx-agentd/src/human_frame.rs:19). |
