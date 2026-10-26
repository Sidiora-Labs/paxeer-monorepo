# Agent lni contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## c-abi

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.c-lni-abi-layerx-lni-abi-version</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:9</code> | <code>#define LAYERX_LNI_ABI_VERSION ((MAJOR &lt;&lt; 16u) &#124; MINOR)</code> |
| <code>interface.9.c-lni-abi-layerx-lni-abi-version-2</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:21</code> | <code>uint32_t layerx_lni_abi_version(void);</code> |
| <code>interface.9.c-lni-abi-layerx-lni-abi-version-major</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:7</code> | <code>#define LAYERX_LNI_ABI_VERSION_MAJOR 1u</code> |
| <code>interface.9.c-lni-abi-layerx-lni-abi-version-minor</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:8</code> | <code>#define LAYERX_LNI_ABI_VERSION_MINOR 0u</code> |
| <code>interface.9.c-lni-abi-layerx-lni-close</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:25</code> | <code>void layerx_lni_close(layerx_lni_handle *handle);</code> |
| <code>interface.9.c-lni-abi-layerx-lni-handle</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:11</code> | <code>typedef struct layerx_lni_handle layerx_lni_handle</code> |
| <code>interface.9.c-lni-abi-layerx-lni-receive</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:23</code> | <code>int32_t layerx_lni_receive(layerx_lni_handle *handle, const uint8_t **canonical_frame, size_t *frame_len);</code> |
| <code>interface.9.c-lni-abi-layerx-lni-release</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:24</code> | <code>void layerx_lni_release(layerx_lni_handle *handle, const uint8_t *canonical_frame, size_t frame_len);</code> |
| <code>interface.9.c-lni-abi-layerx-lni-send</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:22</code> | <code>int32_t layerx_lni_send(layerx_lni_handle *handle, const uint8_t *canonical_frame, size_t frame_len);</code> |
| <code>interface.9.c-lni-abi-layerx-lni-status</code> | <code>c_symbol</code> | <code>retire</code> | <code>services/agent/include/layerx_lni_abi.h:13</code> | <code>enum layerx_lni_status { LAYERX_LNI_OK = 0, LAYERX_LNI_DEADLINE = 1, LAYERX_LNI_PEER_SHUTDOWN = 2, LAYERX_LNI_CONNECTION_FAILURE = 3, LAYERX_LNI_FRAME_VIOLATION = 4 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.c-lni-abi-layerx-lni-abi-version</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. No in-repo C or Rust code calls this symbol; only the header is referenced. |
| <code>interface.9.c-lni-abi-layerx-lni-abi-version-2</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. No in-repo C or Rust code calls this symbol; only the header is referenced. |
| <code>interface.9.c-lni-abi-layerx-lni-abi-version-major</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. |
| <code>interface.9.c-lni-abi-layerx-lni-abi-version-minor</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. No in-repo C or Rust code calls this symbol; only the header is referenced. |
| <code>interface.9.c-lni-abi-layerx-lni-close</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. No in-repo C or Rust code calls this symbol; only the header is referenced. |
| <code>interface.9.c-lni-abi-layerx-lni-handle</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. |
| <code>interface.9.c-lni-abi-layerx-lni-receive</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. No in-repo C or Rust code calls this symbol; only the header is referenced. |
| <code>interface.9.c-lni-abi-layerx-lni-release</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. No in-repo C or Rust code calls this symbol; only the header is referenced. |
| <code>interface.9.c-lni-abi-layerx-lni-send</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. No in-repo C or Rust code calls this symbol; only the header is referenced. |
| <code>interface.9.c-lni-abi-layerx-lni-status</code> | notes | Stale task 11.7.29 deletes layerx_lni_abi.h with the LNI transport. No in-repo C or Rust code calls this symbol; only the header is referenced. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.65.1</code> | req | 9065.1 |
| <code>acceptance.9.65.1</code> | scenario | Given the agent tree after this task, when the test checks those three paths, then none exists. |
| <code>acceptance.9.65.1</code> | test | services/agent/tools/boundary-check/tests/workspace.rs::agent_tree_has_no_lni_artifacts (planned) |
| <code>acceptance.9.65.1</code> | qualification | UNRUN |
| <code>req.9065</code> | ac_1 | services/agent contains no include/layerx_lni_abi.h, no schema/lni directory and no stable-abi-allowlist.toml. |
| <code>acceptance.9.65.2</code> | req | 9065.2 |
| <code>acceptance.9.65.2</code> | scenario | Given services/agent/unsafe-allowlist.toml after this task, when the test parses it, then it has no [[exceptions]] table. |
| <code>acceptance.9.65.2</code> | test | services/agent/tools/boundary-check/tests/workspace.rs::unsafe_allowlist_holds_no_exception (planned) |
| <code>acceptance.9.65.2</code> | qualification | UNRUN |
| <code>req.9065</code> | ac_2 | services/agent/unsafe-allowlist.toml holds version 1 and no exceptions entry. |
| <code>capability.9.19.1</code> | title | Delete the LNI ABI header, LNI schema and stable ABI allowlist |
| <code>capability.9.19.1</code> | reqs | 9065.1<br>9065.2 |

## message-1

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-1-nodeinforequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:14</code> | <code>LNI v1 message tag 1 NodeInfoRequest capability node_info</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-1-nodeinforequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-10

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-10-historyitem</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:86</code> | <code>LNI v1 message tag 10 HistoryItem capability history_range</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-10-historyitem</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-11

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-11-historyend</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:94</code> | <code>LNI v1 message tag 11 HistoryEnd capability history_range</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-11-historyend</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-12

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-12-batchheaderrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:102</code> | <code>LNI v1 message tag 12 BatchHeaderRequest capability batch_header</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-12-batchheaderrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-13

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-13-batchheaderresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:110</code> | <code>LNI v1 message tag 13 BatchHeaderResponse capability batch_header</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-13-batchheaderresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-14

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-14-checkpointrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:118</code> | <code>LNI v1 message tag 14 CheckpointRequest capability checkpoint</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-14-checkpointrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-15

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-15-checkpointresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:126</code> | <code>LNI v1 message tag 15 CheckpointResponse capability checkpoint</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-15-checkpointresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-16

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-16-proofbundlerequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:134</code> | <code>LNI v1 message tag 16 ProofBundleRequest capability proof_bundle</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-16-proofbundlerequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-17

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-17-proofbundleresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:142</code> | <code>LNI v1 message tag 17 ProofBundleResponse capability proof_bundle</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-17-proofbundleresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-18

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-18-availabilityfetchrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:150</code> | <code>LNI v1 message tag 18 AvailabilityFetchRequest capability availability_fetch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-18-availabilityfetchrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-19

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-19-availabilitychunk</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:158</code> | <code>LNI v1 message tag 19 AvailabilityChunk capability availability_fetch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-19-availabilitychunk</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-2-nodeinforesponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:22</code> | <code>LNI v1 message tag 2 NodeInfoResponse capability node_info</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-2-nodeinforesponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-20

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-20-availabilityend</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:166</code> | <code>LNI v1 message tag 20 AvailabilityEnd capability availability_fetch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-20-availabilityend</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-21

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-21-eventsubscriberequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:174</code> | <code>LNI v1 message tag 21 EventSubscribeRequest capability event_subscribe</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-21-eventsubscriberequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-22

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-22-eventrecord</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:182</code> | <code>LNI v1 message tag 22 EventRecord capability event_subscribe</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-22-eventrecord</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-23

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-23-eventgap</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:190</code> | <code>LNI v1 message tag 23 EventGap capability event_subscribe</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-23-eventgap</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-24

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-24-eventheartbeat</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:198</code> | <code>LNI v1 message tag 24 EventHeartbeat capability event_subscribe</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-24-eventheartbeat</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-25

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-25-errorresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:206</code> | <code>LNI v1 message tag 25 ErrorResponse capability node_info</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-25-errorresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-26

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-26-preparationstaterequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:214</code> | <code>LNI v1 message tag 26 PreparationStateRequest capability preparation_state</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-26-preparationstaterequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-27

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-27-preparationstateresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:222</code> | <code>LNI v1 message tag 27 PreparationStateResponse capability preparation_state</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-27-preparationstateresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-28

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-28-finalityevidenceregisterrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:230</code> | <code>LNI v1 message tag 28 FinalityEvidenceRegisterRequest capability finality_evidence_register</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-28-finalityevidenceregisterrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-29

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-29-finalityevidenceregisterresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:238</code> | <code>LNI v1 message tag 29 FinalityEvidenceRegisterResponse capability finality_evidence_register</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-29-finalityevidenceregisterresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-3

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-3-submitrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:30</code> | <code>LNI v1 message tag 3 SubmitRequest capability submit</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-3-submitrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-30

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-30-simulaterequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:246</code> | <code>LNI v1 message tag 30 SimulateRequest capability simulate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-30-simulaterequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-31

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-31-simulateresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:254</code> | <code>LNI v1 message tag 31 SimulateResponse capability simulate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-31-simulateresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-32

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-32-assetreadrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:262</code> | <code>LNI v1 message tag 32 AssetReadRequest capability asset_read</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-32-assetreadrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-33

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-33-assetreadresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:270</code> | <code>LNI v1 message tag 33 AssetReadResponse capability asset_read</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-33-assetreadresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-34

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-34-feeestimaterequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:278</code> | <code>LNI v1 message tag 34 FeeEstimateRequest capability fee_estimate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-34-feeestimaterequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-35

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-35-feeestimateresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:286</code> | <code>LNI v1 message tag 35 FeeEstimateResponse capability fee_estimate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-35-feeestimateresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-36

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-36-sessionfeestaterequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:294</code> | <code>LNI v1 message tag 36 SessionFeeStateRequest capability session_fee_state</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-36-sessionfeestaterequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-37

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-37-sessionfeestateresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:302</code> | <code>LNI v1 message tag 37 SessionFeeStateResponse capability session_fee_state</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-37-sessionfeestateresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-38

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-38-programreadrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:310</code> | <code>LNI v1 message tag 38 ProgramReadRequest capability program_read</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-38-programreadrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-39-programreadresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:318</code> | <code>LNI v1 message tag 39 ProgramReadResponse capability program_read</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-39-programreadresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-4

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-4-submitresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:38</code> | <code>LNI v1 message tag 4 SubmitResponse capability submit</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-4-submitresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-40

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-40-programheadattestrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:326</code> | <code>LNI v1 message tag 40 ProgramHeadAttestRequest capability program_head_attest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-40-programheadattestrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-41

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-41-programheadattestresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:334</code> | <code>LNI v1 message tag 41 ProgramHeadAttestResponse capability program_head_attest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-41-programheadattestresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-42

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-42-capsdiscoveryrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:342</code> | <code>LNI v1 message tag 42 CapsDiscoveryRequest capability caps_discovery</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-42-capsdiscoveryrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-43

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-43-capsdiscoveryresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:350</code> | <code>LNI v1 message tag 43 CapsDiscoveryResponse capability caps_discovery</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-43-capsdiscoveryresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-44

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-44-executionprestaterequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:374</code> | <code>LNI v1 message tag 44 ExecutionPrestateRequest capability execution_prestate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-44-executionprestaterequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-45

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-45-executionprestateresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:382</code> | <code>LNI v1 message tag 45 ExecutionPrestateResponse capability execution_prestate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-45-executionprestateresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-5

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-5-receiptlookuprequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:46</code> | <code>LNI v1 message tag 5 ReceiptLookupRequest capability receipt_lookup</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-5-receiptlookuprequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-6

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-6-receiptlookupresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:54</code> | <code>LNI v1 message tag 6 ReceiptLookupResponse capability receipt_lookup</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-6-receiptlookupresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-7

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-7-accountreadrequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:62</code> | <code>LNI v1 message tag 7 AccountReadRequest capability account_read</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-7-accountreadrequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-8

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-8-accountreadresponse</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:70</code> | <code>LNI v1 message tag 8 AccountReadResponse capability account_read</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-8-accountreadresponse</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |

## message-9

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.proto-lni-v1-9-historyrangerequest</code> | <code>proto_message</code> | <code>retire</code> | <code>services/agent/schema/lni/v1.kvx:78</code> | <code>LNI v1 message tag 9 HistoryRangeRequest capability history_range</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.proto-lni-v1-9-historyrangerequest</code> | notes | LayerX Node Interface v1; stale task 11.7.29 deletes the LNI transport and 11.5.47 re-targets this schema. |
