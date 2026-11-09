use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io;
use std::path::Path;

const RETIRED_SURFACES: &str = r#"interface.9.c-lni-abi-layerx-lni-abi-version	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-abi-version-2	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-abi-version-major	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-abi-version-minor	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-close	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-handle	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-receive	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-release	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-send	services/agent/include/layerx_lni_abi.h	file	
interface.9.c-lni-abi-layerx-lni-status	services/agent/include/layerx_lni_abi.h	file	
interface.9.cli-agent-fuzz-lni-frame-j44	services/agent/fuzz/Cargo.toml	text	lni_frame
interface.9.cli-agent-qualification-gates-wire	services/agent/tests/qualify/src/main.rs	wire_cli	
interface.9.cli-agent-wire-differential	services/agent/tools/wire-differential/src/main.rs	file	
interface.9.env-layerx-agent-genesis-trust	services/agent/crates/layerx-agentd/src/main.rs	env	LAYERX_AGENT_GENESIS_TRUST
interface.9.env-layerx-agent-handover-finality	services/agent/crates/layerx-agentd/src/main.rs	env	LAYERX_AGENT_HANDOVER_FINALITY
interface.9.env-layerx-agent-human-export-settlement-domain	services/agent/crates/layerx-agentd/src/main.rs	env	LAYERX_AGENT_HUMAN_EXPORT_SETTLEMENT_DOMAIN
interface.9.env-layerx-agent-sequencer-trust-history	services/agent/crates/layerx-agentd/src/main.rs	env	LAYERX_AGENT_SEQUENCER_TRUST_HISTORY
interface.9.env-layerx-c-reference	services/agent/tools/wire-differential/src/main.rs	env	LAYERX_C_REFERENCE
interface.9.env-layerx-repository-root	services/agent/tools/wire-differential/src/main.rs	env	LAYERX_REPOSITORY_ROOT
interface.9.err-reason-export-settlement-anchoring-unavailable	services/agent/crates/layerx-agentd/src/human.rs	text	export.settlement_anchoring_unavailable
interface.9.fmt-lni-v1-capabilities	services/agent/schema/lni/v1.kvx	file	
interface.9.fmt-lni-v1-frame	services/agent/schema/lni/v1.kvx	file	
interface.9.http-rpc-faucet-claim	services/agent/schema/agent-api/identity.kvx	text	faucet.claim
interface.9.proto-agent-api-v1-type-settlementdomain	services/agent/schema/agent-api/v1.kvx	text	[type.SettlementDomain]
interface.9.proto-lni-v1-1-nodeinforequest	services/agent/schema/lni/v1.kvx	text	NodeInfoRequest
interface.9.proto-lni-v1-10-historyitem	services/agent/schema/lni/v1.kvx	text	HistoryItem
interface.9.proto-lni-v1-11-historyend	services/agent/schema/lni/v1.kvx	text	HistoryEnd
interface.9.proto-lni-v1-12-batchheaderrequest	services/agent/schema/lni/v1.kvx	text	BatchHeaderRequest
interface.9.proto-lni-v1-13-batchheaderresponse	services/agent/schema/lni/v1.kvx	text	BatchHeaderResponse
interface.9.proto-lni-v1-14-checkpointrequest	services/agent/schema/lni/v1.kvx	text	CheckpointRequest
interface.9.proto-lni-v1-15-checkpointresponse	services/agent/schema/lni/v1.kvx	text	CheckpointResponse
interface.9.proto-lni-v1-16-proofbundlerequest	services/agent/schema/lni/v1.kvx	text	ProofBundleRequest
interface.9.proto-lni-v1-17-proofbundleresponse	services/agent/schema/lni/v1.kvx	text	ProofBundleResponse
interface.9.proto-lni-v1-18-availabilityfetchrequest	services/agent/schema/lni/v1.kvx	text	AvailabilityFetchRequest
interface.9.proto-lni-v1-19-availabilitychunk	services/agent/schema/lni/v1.kvx	text	AvailabilityChunk
interface.9.proto-lni-v1-2-nodeinforesponse	services/agent/schema/lni/v1.kvx	text	NodeInfoResponse
interface.9.proto-lni-v1-20-availabilityend	services/agent/schema/lni/v1.kvx	text	AvailabilityEnd
interface.9.proto-lni-v1-21-eventsubscriberequest	services/agent/schema/lni/v1.kvx	text	EventSubscribeRequest
interface.9.proto-lni-v1-22-eventrecord	services/agent/schema/lni/v1.kvx	text	EventRecord
interface.9.proto-lni-v1-23-eventgap	services/agent/schema/lni/v1.kvx	text	EventGap
interface.9.proto-lni-v1-24-eventheartbeat	services/agent/schema/lni/v1.kvx	text	EventHeartbeat
interface.9.proto-lni-v1-25-errorresponse	services/agent/schema/lni/v1.kvx	text	ErrorResponse
interface.9.proto-lni-v1-26-preparationstaterequest	services/agent/schema/lni/v1.kvx	text	PreparationStateRequest
interface.9.proto-lni-v1-27-preparationstateresponse	services/agent/schema/lni/v1.kvx	text	PreparationStateResponse
interface.9.proto-lni-v1-28-finalityevidenceregisterrequest	services/agent/schema/lni/v1.kvx	text	FinalityEvidenceRegisterRequest
interface.9.proto-lni-v1-29-finalityevidenceregisterresponse	services/agent/schema/lni/v1.kvx	text	FinalityEvidenceRegisterResponse
interface.9.proto-lni-v1-3-submitrequest	services/agent/schema/lni/v1.kvx	text	SubmitRequest
interface.9.proto-lni-v1-30-simulaterequest	services/agent/schema/lni/v1.kvx	text	SimulateRequest
interface.9.proto-lni-v1-31-simulateresponse	services/agent/schema/lni/v1.kvx	text	SimulateResponse
interface.9.proto-lni-v1-32-assetreadrequest	services/agent/schema/lni/v1.kvx	text	AssetReadRequest
interface.9.proto-lni-v1-33-assetreadresponse	services/agent/schema/lni/v1.kvx	text	AssetReadResponse
interface.9.proto-lni-v1-34-feeestimaterequest	services/agent/schema/lni/v1.kvx	text	FeeEstimateRequest
interface.9.proto-lni-v1-35-feeestimateresponse	services/agent/schema/lni/v1.kvx	text	FeeEstimateResponse
interface.9.proto-lni-v1-36-sessionfeestaterequest	services/agent/schema/lni/v1.kvx	text	SessionFeeStateRequest
interface.9.proto-lni-v1-37-sessionfeestateresponse	services/agent/schema/lni/v1.kvx	text	SessionFeeStateResponse
interface.9.proto-lni-v1-38-programreadrequest	services/agent/schema/lni/v1.kvx	text	ProgramReadRequest
interface.9.proto-lni-v1-39-programreadresponse	services/agent/schema/lni/v1.kvx	text	ProgramReadResponse
interface.9.proto-lni-v1-4-submitresponse	services/agent/schema/lni/v1.kvx	text	SubmitResponse
interface.9.proto-lni-v1-40-programheadattestrequest	services/agent/schema/lni/v1.kvx	text	ProgramHeadAttestRequest
interface.9.proto-lni-v1-41-programheadattestresponse	services/agent/schema/lni/v1.kvx	text	ProgramHeadAttestResponse
interface.9.proto-lni-v1-42-capsdiscoveryrequest	services/agent/schema/lni/v1.kvx	text	CapsDiscoveryRequest
interface.9.proto-lni-v1-43-capsdiscoveryresponse	services/agent/schema/lni/v1.kvx	text	CapsDiscoveryResponse
interface.9.proto-lni-v1-44-executionprestaterequest	services/agent/schema/lni/v1.kvx	text	ExecutionPrestateRequest
interface.9.proto-lni-v1-45-executionprestateresponse	services/agent/schema/lni/v1.kvx	text	ExecutionPrestateResponse
interface.9.proto-lni-v1-5-receiptlookuprequest	services/agent/schema/lni/v1.kvx	text	ReceiptLookupRequest
interface.9.proto-lni-v1-6-receiptlookupresponse	services/agent/schema/lni/v1.kvx	text	ReceiptLookupResponse
interface.9.proto-lni-v1-7-accountreadrequest	services/agent/schema/lni/v1.kvx	text	AccountReadRequest
interface.9.proto-lni-v1-8-accountreadresponse	services/agent/schema/lni/v1.kvx	text	AccountReadResponse
interface.9.proto-lni-v1-9-historyrangerequest	services/agent/schema/lni/v1.kvx	text	HistoryRangeRequest
interface.9.rs-client-evidence-asset-record-assetevidenceerror-j44	services/agent/crates/layerx-client/src/evidence/asset_record.rs	rust	enum AssetEvidenceError
interface.9.rs-client-evidence-asset-record-assetrecordmetadata-j44	services/agent/crates/layerx-client/src/evidence/asset_record.rs	rust	struct AssetRecordMetadata
interface.9.rs-client-evidence-asset-record-assetsourcekind-j44	services/agent/crates/layerx-client/src/evidence/asset_record.rs	rust	enum AssetSourceKind
interface.9.rs-client-evidence-asset-record-verifiedeffectiveasset	services/agent/crates/layerx-client/src/evidence/asset_record.rs	rust	struct VerifiedEffectiveAsset
interface.9.rs-client-evidence-asset-record-verifiedissuance-j44	services/agent/crates/layerx-client/src/evidence/asset_record.rs	rust	struct VerifiedIssuance
interface.9.rs-client-evidence-module-state-verifiedmoduleevidence-j44	services/agent/crates/layerx-client/src/evidence/module_state.rs	rust	struct VerifiedModuleEvidence
interface.9.rs-client-evidence-module-state-verify-module-evidence	services/agent/crates/layerx-client/src/evidence/module_state.rs	rust	fn verify_module_evidence
interface.9.rs-client-lni-abi-abiincompatible-j44	services/agent/crates/layerx-client/src/lni/abi.rs	rust	struct AbiIncompatible
interface.9.rs-client-lni-abi-abiopenerror-j44	services/agent/crates/layerx-client/src/lni/abi.rs	rust	enum AbiOpenError
interface.9.rs-client-lni-abi-abiversion-j44	services/agent/crates/layerx-client/src/lni/abi.rs	rust	struct AbiVersion
interface.9.rs-client-lni-abi-fn-j44	services/agent/crates/layerx-client/src/lni/abi.rs	rust	fn negotiate
interface.9.rs-client-lni-abi-functions-j44	services/agent/crates/layerx-client/src/lni/abi.rs	rust	struct Functions
interface.9.rs-client-lni-abi-handle-j44	services/agent/crates/layerx-client/src/lni/abi.rs	rust	struct Handle
interface.9.rs-client-lni-capabilities-capabilities	services/agent/crates/layerx-client/src/lni/capabilities.rs	rust	struct Capabilities
interface.9.rs-client-lni-framing-decode-frame	services/agent/crates/layerx-client/src/lni/framing.rs	rust	fn decode_frame
interface.9.rs-client-lni-framing-decodedframe-j44	services/agent/crates/layerx-client/src/lni/framing.rs	rust	enum DecodedFrame
interface.9.rs-client-lni-framing-length-prefix-bytes-j44	services/agent/crates/layerx-client/src/lni/framing.rs	rust	const LENGTH_PREFIX_BYTES
interface.9.rs-client-lni-framing-read-frame	services/agent/crates/layerx-client/src/lni/framing.rs	rust	fn read_frame
interface.9.rs-client-lni-framing-write-frame	services/agent/crates/layerx-client/src/lni/framing.rs	rust	fn write_frame
interface.9.rs-client-lni-handshake-decode-node-info	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	fn decode_node_info
interface.9.rs-client-lni-handshake-encode-node-info	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	fn encode_node_info
interface.9.rs-client-lni-handshake-handshake	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	struct Handshake
interface.9.rs-client-lni-handshake-handshakeconfig	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	struct HandshakeConfig
interface.9.rs-client-lni-handshake-handshakeerror	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	enum HandshakeError
interface.9.rs-client-lni-handshake-nodeinfo	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	struct NodeInfo
interface.9.rs-client-lni-handshake-noderole	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	enum NodeRole
interface.9.rs-client-lni-handshake-perform	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	fn perform
interface.9.rs-client-lni-handshake-perform-with-schema-j44	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	fn perform_with_schema
interface.9.rs-client-lni-handshake-sequencerkeychange-j44	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	struct SequencerKeyChange
interface.9.rs-client-lni-handshake-validate	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	fn validate
interface.9.rs-client-lni-handshake-validate-with-schema-j44	services/agent/crates/layerx-client/src/lni/handshake.rs	rust	fn validate_with_schema
interface.9.rs-client-lni-head-attestation-attest-program-head	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	fn attest_program_head
interface.9.rs-client-lni-head-attestation-decode-program-head-attestation	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	fn decode_program_head_attestation
interface.9.rs-client-lni-head-attestation-encode-program-head-attest-request	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	fn encode_program_head_attest_request
interface.9.rs-client-lni-head-attestation-encode-program-head-attestation	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	fn encode_program_head_attestation
interface.9.rs-client-lni-head-attestation-program-discovery-proof-digest	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	fn program_discovery_proof_digest
interface.9.rs-client-lni-head-attestation-program-discovery-proof-domain	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	const PROGRAM_DISCOVERY_PROOF_DOMAIN
interface.9.rs-client-lni-head-attestation-program-head-attest-payload-bytes	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	const PROGRAM_HEAD_ATTEST_PAYLOAD_BYTES
interface.9.rs-client-lni-head-attestation-program-head-attest-proof-bytes	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	const PROGRAM_HEAD_ATTEST_PROOF_BYTES
interface.9.rs-client-lni-head-attestation-program-head-attest-request-bytes-j44	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	const PROGRAM_HEAD_ATTEST_REQUEST_BYTES
interface.9.rs-client-lni-head-attestation-program-head-attest-request-tag	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	const PROGRAM_HEAD_ATTEST_REQUEST_TAG
interface.9.rs-client-lni-head-attestation-program-head-attest-response-tag	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	const PROGRAM_HEAD_ATTEST_RESPONSE_TAG
interface.9.rs-client-lni-head-attestation-programdiscoveryhead	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	struct ProgramDiscoveryHead
interface.9.rs-client-lni-head-attestation-programheadattestation	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	struct ProgramHeadAttestation
interface.9.rs-client-lni-head-attestation-programheadattestcontext	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	struct ProgramHeadAttestContext
interface.9.rs-client-lni-head-attestation-programheadattesterror	services/agent/crates/layerx-client/src/lni/head_attestation.rs	rust	enum ProgramHeadAttestError
interface.9.rs-client-lni-preparation-decode-preparation-response-j44	services/agent/crates/layerx-client/src/lni/preparation.rs	rust	fn decode_preparation_response
interface.9.rs-client-lni-preparation-encode-preparation-request-j44	services/agent/crates/layerx-client/src/lni/preparation.rs	rust	fn encode_preparation_request
interface.9.rs-client-lni-preparation-preparation-state	services/agent/crates/layerx-client/src/lni/preparation.rs	rust	fn preparation_state
interface.9.rs-client-lni-preparation-preparationstate	services/agent/crates/layerx-client/src/lni/preparation.rs	rust	struct PreparationState
interface.9.rs-client-lni-preparation-preparationstatecontext	services/agent/crates/layerx-client/src/lni/preparation.rs	rust	struct PreparationStateContext
interface.9.rs-client-lni-preparation-preparationstateerror	services/agent/crates/layerx-client/src/lni/preparation.rs	rust	enum PreparationStateError
interface.9.rs-client-lni-program-read-program-read-request-tag-j44	services/agent/crates/layerx-client/src/lni/program_read.rs	rust	const PROGRAM_READ_REQUEST_TAG
interface.9.rs-client-lni-program-read-program-read-response-tag-j44	services/agent/crates/layerx-client/src/lni/program_read.rs	rust	const PROGRAM_READ_RESPONSE_TAG
interface.9.rs-client-lni-program-read-programreadcontext	services/agent/crates/layerx-client/src/lni/program_read.rs	rust	struct ProgramReadContext
interface.9.rs-client-lni-program-read-programreaderror	services/agent/crates/layerx-client/src/lni/program_read.rs	rust	enum ProgramReadError
interface.9.rs-client-lni-program-read-programreadresult	services/agent/crates/layerx-client/src/lni/program_read.rs	rust	struct ProgramReadResult
interface.9.rs-client-lni-program-read-programreadsnapshot-j44	services/agent/crates/layerx-client/src/lni/program_read.rs	rust	struct ProgramReadSnapshot
interface.9.rs-client-lni-program-read-read-program	services/agent/crates/layerx-client/src/lni/program_read.rs	rust	fn read_program
interface.9.rs-client-lni-refusal-corerefusal	services/agent/crates/layerx-client/src/lni/refusal.rs	rust	struct CoreRefusal
interface.9.rs-client-lni-refusal-decode-core-refusal	services/agent/crates/layerx-client/src/lni/refusal.rs	rust	fn decode_core_refusal
interface.9.rs-client-lni-report-capability-report	services/agent/crates/layerx-client/src/lni/report.rs	rust	fn capability_report
interface.9.rs-client-lni-report-capability-report-with-schema-j44	services/agent/crates/layerx-client/src/lni/report.rs	rust	fn capability_report_with_schema
interface.9.rs-client-lni-report-capabilityreport	services/agent/crates/layerx-client/src/lni/report.rs	rust	struct CapabilityReport
interface.9.rs-client-lni-report-capabilitystatus-j44	services/agent/crates/layerx-client/src/lni/report.rs	rust	struct CapabilityStatus
interface.9.rs-client-lni-schema-capability	services/agent/crates/layerx-client/src/lni/schema.rs	rust	enum Capability
interface.9.rs-client-lni-schema-decode-envelope	services/agent/crates/layerx-client/src/lni/schema.rs	rust	fn decode_envelope
interface.9.rs-client-lni-schema-decode-envelope-with-schema-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	fn decode_envelope_with_schema
interface.9.rs-client-lni-schema-encode-envelope	services/agent/crates/layerx-client/src/lni/schema.rs	rust	fn encode_envelope
interface.9.rs-client-lni-schema-encode-envelope-with-schema-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	fn encode_envelope_with_schema
interface.9.rs-client-lni-schema-envelope	services/agent/crates/layerx-client/src/lni/schema.rs	rust	struct Envelope
interface.9.rs-client-lni-schema-fn-2-3-4-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	fn lni_golden_vectors
interface.9.rs-client-lni-schema-fn-2-3-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	fn lni_schema_arbiter_admission_v3
interface.9.rs-client-lni-schema-fn-2-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	fn lni_schema_arbiter_prestate_v2
interface.9.rs-client-lni-schema-fn-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	fn lni_schema_v1
interface.9.rs-client-lni-schema-goldenvector-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	struct GoldenVector
interface.9.rs-client-lni-schema-lni-v1-source-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	const LNI_V1_SOURCE
interface.9.rs-client-lni-schema-messagedescriptor-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	struct MessageDescriptor
interface.9.rs-client-lni-schema-messagekind-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	enum MessageKind
interface.9.rs-client-lni-schema-schema-j44	services/agent/crates/layerx-client/src/lni/schema.rs	rust	struct Schema
interface.9.rs-client-lni-schema-schemaerror	services/agent/crates/layerx-client/src/lni/schema.rs	rust	enum SchemaError
interface.9.rs-client-lni-schema-version	services/agent/crates/layerx-client/src/lni/schema.rs	rust	struct Version
interface.9.rs-client-lni-simulate-decode-simulation-evidence-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	fn decode_simulation_evidence
interface.9.rs-client-lni-simulate-decode-simulation-payload-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	fn decode_simulation_payload
interface.9.rs-client-lni-simulate-encode-simulation-evidence-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	fn encode_simulation_evidence
interface.9.rs-client-lni-simulate-encode-simulation-payload-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	fn encode_simulation_payload
interface.9.rs-client-lni-simulate-simulate	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	fn simulate
interface.9.rs-client-lni-simulate-simulate-request-tag-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	const SIMULATE_REQUEST_TAG
interface.9.rs-client-lni-simulate-simulate-response-tag-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	const SIMULATE_RESPONSE_TAG
interface.9.rs-client-lni-simulate-simulatecontext	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	struct SimulateContext
interface.9.rs-client-lni-simulate-simulatedexecution	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	struct SimulatedExecution
interface.9.rs-client-lni-simulate-simulateerror	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	enum SimulateError
interface.9.rs-client-lni-simulate-simulation	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	struct Simulation
interface.9.rs-client-lni-simulate-simulation-boundary-domain-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	const SIMULATION_BOUNDARY_DOMAIN
interface.9.rs-client-lni-simulate-simulation-boundary-id	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	fn simulation_boundary_id
interface.9.rs-client-lni-simulate-simulation-evidence-digest	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	fn simulation_evidence_digest
interface.9.rs-client-lni-simulate-simulation-evidence-domain-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	const SIMULATION_EVIDENCE_DOMAIN
interface.9.rs-client-lni-simulate-simulationevidence	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	struct SimulationEvidence
interface.9.rs-client-lni-simulate-verify-simulation-j44	services/agent/crates/layerx-client/src/lni/simulate.rs	rust	fn verify_simulation
interface.9.rs-client-lni-transport-connectiongate	services/agent/crates/layerx-client/src/lni/transport.rs	rust	struct ConnectionGate
interface.9.rs-client-lni-transport-connectionpermit	services/agent/crates/layerx-client/src/lni/transport.rs	rust	struct ConnectionPermit
interface.9.rs-client-lni-transport-frametransport	services/agent/crates/layerx-client/src/lni/transport.rs	rust	trait FrameTransport
interface.9.rs-client-lni-transport-frameviolation	services/agent/crates/layerx-client/src/lni/transport.rs	rust	enum FrameViolation
interface.9.rs-client-lni-transport-limits	services/agent/crates/layerx-client/src/lni/transport.rs	rust	struct Limits
interface.9.rs-client-lni-transport-multiplexer-j44	services/agent/crates/layerx-client/src/lni/transport.rs	rust	struct Multiplexer
interface.9.rs-client-lni-transport-mutualtlsconfig	services/agent/crates/layerx-client/src/lni/transport.rs	rust	struct MutualTlsConfig
interface.9.rs-client-lni-transport-outboundframe-j44	services/agent/crates/layerx-client/src/lni/transport.rs	rust	struct OutboundFrame
interface.9.rs-client-lni-transport-tls	services/agent/crates/layerx-client/src/lni/transport.rs	rust	struct Tls
interface.9.rs-client-lni-transport-trafficclass-j44	services/agent/crates/layerx-client/src/lni/transport.rs	rust	enum TrafficClass
interface.9.rs-client-lni-transport-transporterror	services/agent/crates/layerx-client/src/lni/transport.rs	rust	enum TransportError
interface.9.rs-client-lni-transport-uds	services/agent/crates/layerx-client/src/lni/transport.rs	rust	struct Uds
interface.9.rs-proof-attestation-attested-at-ms-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn attested_at_ms
interface.9.rs-proof-attestation-canonical-statement-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn canonical_statement
interface.9.rs-proof-attestation-guarantor-id-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn guarantor_id
interface.9.rs-proof-attestation-new-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn new
interface.9.rs-proof-attestation-signature-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn signature
interface.9.rs-proof-attestation-signer-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn signer
interface.9.rs-proof-certificate-attestations-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn attestations
interface.9.rs-proof-certificate-new-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn new
interface.9.rs-proof-certificate-settlement-reference-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn settlement_reference
interface.9.rs-proof-certificate-threshold-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn threshold
interface.9.rs-proof-checkpoint-attestation	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	struct Attestation
interface.9.rs-proof-checkpoint-certificate	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	struct Certificate
interface.9.rs-proof-checkpoint-checkpoint	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	struct Checkpoint
interface.9.rs-proof-checkpoint-checkpoint-id	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn checkpoint_id
interface.9.rs-proof-checkpoint-checkpointerror	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	enum CheckpointError
interface.9.rs-proof-checkpoint-guarantorkey	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	struct GuarantorKey
interface.9.rs-proof-checkpoint-header-bytes-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn header_bytes
interface.9.rs-proof-checkpoint-new-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn new
interface.9.rs-proof-checkpoint-settlementdomain	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	struct SettlementDomain
interface.9.rs-proof-checkpoint-thresholdreport	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	struct ThresholdReport
interface.9.rs-proof-checkpoint-validity-proof-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn validity_proof
interface.9.rs-proof-checkpoint-verify-certificate	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn verify_certificate
interface.9.rs-proof-checkpoint-verify-declared-certificate	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn verify_declared_certificate
interface.9.rs-proof-declareddomain-network-id-j70	services/agent/crates/layerx-proof/src/settlement.rs	rust	fn network_id
interface.9.rs-proof-evidence-settlement-reference-j70	services/agent/crates/layerx-proof/src/evidence.rs	rust	fn settlement_reference
interface.9.rs-proof-export-checkpointfact	services/agent/crates/layerx-proof/src/export.rs	rust	struct CheckpointFact
interface.9.rs-proof-export-codec-checkpointrecord	services/agent/crates/layerx-proof/src/export_codec.rs	rust	struct CheckpointRecord
interface.9.rs-proof-export-codec-max-checkpoint-guarantors-j44	services/agent/crates/layerx-proof/src/export_codec.rs	rust	const MAX_CHECKPOINT_GUARANTORS
interface.9.rs-proof-export-trustedcheckpointmembership	services/agent/crates/layerx-proof/src/export.rs	rust	struct TrustedCheckpointMembership
interface.9.rs-proof-guarantorkey-guarantor-id-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn guarantor_id
interface.9.rs-proof-guarantorkey-new-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn new
interface.9.rs-proof-guarantorkey-public-key-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn public_key
interface.9.rs-proof-receipt-owner-module-nativeowneroutcomecontext	services/agent/crates/layerx-proof/src/receipt/owner_module.rs	rust	struct NativeOwnerOutcomeContext
interface.9.rs-proof-receipt-owner-module-nativeowneroutcomefailure	services/agent/crates/layerx-proof/src/receipt/owner_module.rs	rust	enum NativeOwnerOutcomeFailure
interface.9.rs-proof-receipt-owner-module-verify-native-owner-outcome	services/agent/crates/layerx-proof/src/receipt/owner_module.rs	rust	fn verify_native_owner_outcome
interface.9.rs-proof-receipt-withdrawal-verify	services/agent/crates/layerx-proof/src/receipt/withdrawal.rs	rust	fn verify
interface.9.rs-proof-settlement-declared	services/agent/crates/layerx-proof/src/settlement.rs	rust	fn declared
interface.9.rs-proof-settlement-declared-checkpoint-settlement-j44	services/agent/crates/layerx-proof/src/settlement.rs	rust	const DECLARED_CHECKPOINT_SETTLEMENT
interface.9.rs-proof-settlement-declared-domain	services/agent/crates/layerx-proof/src/settlement.rs	rust	fn declared_domain
interface.9.rs-proof-settlement-declareddomain	services/agent/crates/layerx-proof/src/settlement.rs	rust	struct DeclaredDomain
interface.9.rs-proof-settlement-maximum-attestation-delay-ms	services/agent/crates/layerx-proof/src/settlement.rs	rust	fn maximum_attestation_delay_ms
interface.9.rs-proof-settlementdomain-new-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn new
interface.9.rs-proof-settlementdomain-paxeer-chain-id-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn paxeer_chain_id
interface.9.rs-proof-settlementdomain-settlement-contract-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn settlement_contract
interface.9.rs-proof-thresholdreport-batch-number-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn batch_number
interface.9.rs-proof-thresholdreport-data-availability-root-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn data_availability_root
interface.9.rs-proof-thresholdreport-evidence-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn evidence
interface.9.rs-proof-thresholdreport-first-sequence-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn first_sequence
interface.9.rs-proof-thresholdreport-last-sequence-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn last_sequence
interface.9.rs-proof-thresholdreport-level-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn level
interface.9.rs-proof-thresholdreport-network-id-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn network_id
interface.9.rs-proof-thresholdreport-protocol-version-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn protocol_version
interface.9.rs-proof-thresholdreport-record-roots-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn record_roots
interface.9.rs-proof-thresholdreport-resulting-state-root-j70	services/agent/crates/layerx-proof/src/checkpoint.rs	rust	fn resulting_state_root
interface.9.rs-types-declaredcheckpointsettlement-checkpoint-certificate-domain-j70	services/agent/crates/layerx-types/src/settlement.rs	rust	fn checkpoint_certificate_domain
interface.9.rs-types-declaredcheckpointsettlement-domain-j70	services/agent/crates/layerx-types/src/settlement.rs	rust	fn domain
interface.9.rs-types-declaredcheckpointsettlement-finality-policy-j70	services/agent/crates/layerx-types/src/settlement.rs	rust	fn finality_policy
interface.9.rs-types-declaredcheckpointsettlement-guarantor-attestation-domain-j70	services/agent/crates/layerx-types/src/settlement.rs	rust	fn guarantor_attestation_domain
interface.9.rs-types-declaredcheckpointsettlement-header-encoding-prefix-j70	services/agent/crates/layerx-types/src/settlement.rs	rust	fn header_encoding_prefix
interface.9.rs-types-declaredcheckpointsettlement-header-length-j70	services/agent/crates/layerx-types/src/settlement.rs	rust	fn header_length
interface.9.rs-types-declaredcheckpointsettlement-parse-j70	services/agent/crates/layerx-types/src/settlement.rs	rust	fn parse
interface.9.rs-types-declaredcheckpointsettlement-protocol-version-j70	services/agent/crates/layerx-types/src/settlement.rs	rust	fn protocol_version
interface.9.rs-types-settlement-checkpoint-settlement-path	services/agent/crates/layerx-types/src/settlement.rs	rust	const CHECKPOINT_SETTLEMENT_PATH
interface.9.rs-types-settlement-checkpoint-settlement-schema-j44	services/agent/crates/layerx-types/src/settlement.rs	rust	const CHECKPOINT_SETTLEMENT_SCHEMA
interface.9.rs-types-settlement-declaredcheckpointsettlement	services/agent/crates/layerx-types/src/settlement.rs	rust	struct DeclaredCheckpointSettlement
interface.9.rs-types-settlement-declaredfinalitypolicy-j44	services/agent/crates/layerx-types/src/settlement.rs	rust	struct DeclaredFinalityPolicy
interface.9.rs-types-settlement-declaredguarantor-j44	services/agent/crates/layerx-types/src/settlement.rs	rust	struct DeclaredGuarantor
interface.9.rs-types-settlement-declaredsettlementdomain-j44	services/agent/crates/layerx-types/src/settlement.rs	rust	struct DeclaredSettlementDomain
interface.9.rs-types-settlement-settlementerror	services/agent/crates/layerx-types/src/settlement.rs	rust	enum SettlementError
interface.9.rs-wire-activity-account-sequence-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn account_sequence
interface.9.rs-wire-activity-activity	services/agent/crates/layerx-wire/src/activity.rs	rust	struct Activity
interface.9.rs-wire-activity-activity-type-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn activity_type
interface.9.rs-wire-activity-actor-did-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn actor_did
interface.9.rs-wire-activity-authority-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn authority
interface.9.rs-wire-activity-decode-signed	services/agent/crates/layerx-wire/src/activity.rs	rust	fn decode_signed
interface.9.rs-wire-activity-decode-unsigned	services/agent/crates/layerx-wire/src/activity.rs	rust	fn decode_unsigned
interface.9.rs-wire-activity-encode-signed	services/agent/crates/layerx-wire/src/activity.rs	rust	fn encode_signed
interface.9.rs-wire-activity-encode-signed-envelope	services/agent/crates/layerx-wire/src/activity.rs	rust	fn encode_signed_envelope
interface.9.rs-wire-activity-encode-unsigned	services/agent/crates/layerx-wire/src/activity.rs	rust	fn encode_unsigned
interface.9.rs-wire-activity-encode-unsigned-envelope	services/agent/crates/layerx-wire/src/activity.rs	rust	fn encode_unsigned_envelope
interface.9.rs-wire-activity-fee-limit-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn fee_limit
interface.9.rs-wire-activity-idempotency-key-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn idempotency_key
interface.9.rs-wire-activity-network-id-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn network_id
interface.9.rs-wire-activity-payload-hash-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn payload_hash
interface.9.rs-wire-activity-payload-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn payload
interface.9.rs-wire-activity-protocol-version-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn protocol_version
interface.9.rs-wire-activity-signature-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn signature
interface.9.rs-wire-activity-signing-bytes	services/agent/crates/layerx-wire/src/activity.rs	rust	fn signing_bytes
interface.9.rs-wire-activity-signingbytes-j44	services/agent/crates/layerx-wire/src/activity.rs	rust	struct SigningBytes
interface.9.rs-wire-activity-timestamp-bound-j70	services/agent/crates/layerx-wire/src/activity.rs	rust	fn timestamp_bound
interface.9.rs-wire-activity-timestampbound	services/agent/crates/layerx-wire/src/activity.rs	rust	struct TimestampBound
interface.9.rs-wire-hash-activity-id	services/agent/crates/layerx-wire/src/hash.rs	rust	fn activity_id
interface.9.rs-wire-hash-canonical-activity-j44	services/agent/crates/layerx-wire/src/hash.rs	rust	fn canonical_activity
interface.9.rs-wire-hash-canonical-payload-j44	services/agent/crates/layerx-wire/src/hash.rs	rust	fn canonical_payload
interface.9.rs-wire-hash-payload-hash	services/agent/crates/layerx-wire/src/hash.rs	rust	fn payload_hash
interface.9.rs-wire-sign-preimage	services/agent/crates/layerx-wire/src/sign.rs	rust	fn preimage
interface.9.rs-wire-sign-preimage-unsigned	services/agent/crates/layerx-wire/src/sign.rs	rust	fn preimage_unsigned
interface.9.rs-wire-sign-signingpreimage-j44	services/agent/crates/layerx-wire/src/sign.rs	rust	struct SigningPreimage
interface.9.rs-wire-signingpreimage-as-bytes-j70	services/agent/crates/layerx-wire/src/sign.rs	rust	fn as_bytes"#;

const PLANNED_PATHS: &[&str] = &[
    "services/agent/crates/layerx-agent-api/tests/schema_retired.rs",
    "services/agent/crates/layerx-agentd/host-tests/retired_env.rs",
    "services/agent/crates/layerx-agentd/tests/faucet_claim_retired.rs",
    "services/agent/crates/layerx-agentd/tests/human_peer_retired.rs",
    "services/agent/crates/layerx-agentd/tests/managed_agent_evm_tx.rs",
    "services/agent/crates/layerx-agentd/tests/subscription_ops_cursor.rs",
    "services/agent/crates/layerx-mcp/tests/session_tools_major_two.rs",
    "services/agent/crates/layerx-proof/tests/offline_export.rs",
    "services/agent/fuzz/corpus/envelope_decode/eip1559-transfer-1",
    "services/agent/fuzz/corpus/roundtrip/eip1559-transfer-1",
    "services/agent/fuzz/fuzz_targets/codec.rs",
    "services/agent/fuzz/tests/codec.rs",
    "services/agent/qualification/pay2/tests/build_inputs.rs",
    "services/agent/sdk/python/tests/test_receipt_generated_current.py",
    "services/agent/sdk/python/tests/test_revert_retryable.py",
    "services/agent/sdk/typescript/test/agent-http-major2.test.ts",
    "services/agent/sdk/typescript/test/revert-retryable.test.ts",
    "services/agent/tests/qualify/tests/boundary_gate.rs",
    "services/agent/tests/qualify/tests/gates.rs",
    "services/agent/tests/qualify/tests/real_node_script.rs",
    "services/agent/tests/retired/test_boundary_node_retired.py",
    "services/agent/tools/boundary-check/tests/lni_retired.rs",
    "services/agent/tools/boundary-check/tests/workspace.rs",
];

fn read_source(root: &Path, path: &str) -> Option<String> {
    match fs::read_to_string(root.join(path)) {
        Ok(source) => Some(source),
        Err(error) if error.kind() == io::ErrorKind::NotFound => None,
        Err(error) => panic!("cannot read {path}: {error}"),
    }
}

fn tokens(source: &str) -> Vec<String> {
    let bytes = source.as_bytes();
    let mut result = Vec::new();
    let mut i = 0;
    while i < bytes.len() {
        let start = i;
        if bytes[i].is_ascii_whitespace() {
            i += 1;
        } else if bytes[i..].starts_with(b"//") {
            while i < bytes.len() && bytes[i] != b'\n' {
                i += 1;
            }
        } else if bytes[i..].starts_with(b"/*") {
            i += 2;
            let mut depth = 1;
            while i < bytes.len() && depth > 0 {
                if bytes[i..].starts_with(b"/*") {
                    depth += 1;
                    i += 2;
                } else if bytes[i..].starts_with(b"*/") {
                    depth -= 1;
                    i += 2;
                } else {
                    i += 1;
                }
            }
            assert_eq!(depth, 0, "unterminated block comment");
        } else if bytes[i] == b'r' && {
            let mut quote = i + 1;
            while quote < bytes.len() && bytes[quote] == b'#' {
                quote += 1;
            }
            quote < bytes.len() && bytes[quote] == b'"'
        } {
            i += 1;
            while bytes[i] == b'#' {
                i += 1;
            }
            let hashes = i - start - 1;
            let ending = format!("\"{}", "#".repeat(hashes));
            i += 1;
            let end = source[i..].find(&ending).expect("terminated raw string");
            result.push(format!("\"{}\"", &source[i..i + end]));
            i += end + ending.len();
        } else if bytes[i] == b'\'' && character_literal_end(source, i).is_some() {
            i = character_literal_end(source, i).expect("character literal ending");
            result.push(source[start..i].to_owned());
        } else if bytes[i] == b'"' {
            i += 1;
            while i < bytes.len() {
                if bytes[i] == b'\\' {
                    i += 2;
                } else if bytes[i] == b'"' {
                    i += 1;
                    break;
                } else {
                    i += 1;
                }
            }
            result.push(source[start..i].to_owned());
        } else if bytes[i].is_ascii_alphanumeric() || bytes[i] == b'_' {
            i += 1;
            while i < bytes.len() && (bytes[i].is_ascii_alphanumeric() || bytes[i] == b'_') {
                i += 1;
            }
            result.push(source[start..i].to_owned());
        } else {
            let character = source[i..].chars().next().expect("source character");
            result.push(character.to_string());
            i += character.len_utf8();
        }
    }
    result
}

fn character_literal_end(source: &str, start: usize) -> Option<usize> {
    let bytes = source.as_bytes();
    let mut end = start + 1;
    if *bytes.get(end)? == b'\\' {
        end += 1;
        match *bytes.get(end)? {
            b'x' => end += 3,
            b'u' if bytes.get(end + 1) == Some(&b'{') => {
                end += 2;
                while *bytes.get(end)? != b'}' {
                    end += 1;
                }
                end += 1;
            }
            _ => end += 1,
        }
    } else {
        end += source.get(end..)?.chars().next()?.len_utf8();
    }
    (bytes.get(end) == Some(&b'\'')).then_some(end + 1)
}

fn contains_tokens(source: &[String], pattern: &[&str]) -> bool {
    source.windows(pattern.len()).any(|window| {
        window
            .iter()
            .map(String::as_str)
            .eq(pattern.iter().copied())
    })
}

fn without_historical_modules(source: &[String]) -> Vec<String> {
    let mut retained = Vec::new();
    let mut i = 0;
    while i < source.len() {
        if source[i..].starts_with(&["mod".into(), "historical".into(), "{".into()]) {
            i += 3;
            let mut depth = 1;
            while i < source.len() && depth > 0 {
                match source[i].as_str() {
                    "{" => depth += 1,
                    "}" => depth -= 1,
                    _ => {}
                }
                i += 1;
            }
            assert_eq!(depth, 0, "historical module must be closed");
        } else {
            retained.push(source[i].clone());
            i += 1;
        }
    }
    retained
}

fn root_wire_module_present(root: &Path, module: &str) -> bool {
    let source = read_source(root, "services/agent/crates/layerx-wire/src/lib.rs")
        .expect("wire crate root must exist");
    let source = without_historical_modules(&tokens(&source));
    contains_tokens(&source, &["mod", module]) || reexports(&source, module)
}

fn reexports(source: &[String], name: &str) -> bool {
    source.split(|token| token == ";").any(|statement| {
        contains_tokens(statement, &["pub", "use"])
            && statement.iter().any(|token| token == name || token == "*")
    })
}

fn setting_read(source: &str, name: &str) -> bool {
    let source = tokens(source);
    let literal = format!("\"{name}\"");
    ["var", "var_os"]
        .into_iter()
        .any(|function| contains_tokens(&source, &[function, "(", &literal]))
}

#[test]
fn agent_completion_retired_surfaces_absent() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .expect("parity crate resides below the repository root");
    assert!(root.join("spec/layerx-merge/spec.kvx").is_file());
    let mut violations = Vec::new();
    let mut sources = BTreeMap::new();
    let mut interfaces = BTreeSet::new();
    for row in RETIRED_SURFACES.lines() {
        let fields: Vec<_> = row.split('\t').collect();
        assert_eq!(fields.len(), 4, "invalid surface inventory: {row}");
        let (id, path, kind, needle) = (fields[0], fields[1], fields[2], fields[3]);
        assert!(interfaces.insert(id), "duplicate interface {id}");
        let source = sources
            .entry(path)
            .or_insert_with(|| read_source(root, path));
        let Some(source) = source else { continue };
        let present = match kind {
            "file" => true,
            "text" => source.lines().any(|line| {
                let line = line.trim();
                !line.starts_with('#') && !line.starts_with("//") && line.contains(needle)
            }),
            "env" => setting_read(source, needle),
            "rust" => {
                // Decision 3006 keeps archive codecs at historical paths only.
                let archive_module = match path {
                    "services/agent/crates/layerx-wire/src/activity.rs" => Some("activity"),
                    "services/agent/crates/layerx-wire/src/sign.rs" => Some("sign"),
                    _ => None,
                };
                let exposed =
                    archive_module.is_none_or(|module| root_wire_module_present(root, module));
                let source = tokens(source);
                let source = if path == "services/agent/crates/layerx-wire/src/hash.rs" {
                    without_historical_modules(&source)
                } else {
                    source
                };
                let pattern: Vec<_> = needle.split_whitespace().collect();
                exposed && (contains_tokens(&source, &pattern) || reexports(&source, pattern[1]))
            }
            "wire_cli" => {
                let source = tokens(source);
                contains_tokens(&source, &["mod", "wire"])
                    || source
                        .iter()
                        .any(|token| token == "agent_qualify_wire_gate")
            }
            _ => panic!("unknown surface check {kind} for {id}"),
        };
        if present {
            violations.push(format!("{id} remains at {path} ({needle})"));
        }
    }
    assert_eq!(
        interfaces.len(),
        279,
        "retirement inventory must remain complete"
    );

    // Decision 2279 permits refusal-list names, but never setting reads.
    for name in [
        "LAYERX_AGENT_GENESIS_TRUST",
        "LAYERX_AGENT_HANDOVER_FINALITY",
        "LAYERX_AGENT_SEQUENCER_TRUST_HISTORY",
        "LAYERX_AGENT_HUMAN_EXPORT_SETTLEMENT_DOMAIN",
        "LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY",
    ] {
        for path in [
            "services/agent/crates/layerx-agentd/src/main.rs",
            "services/agent/tests/boundary/src/production.rs",
        ] {
            let source = sources
                .entry(path)
                .or_insert_with(|| read_source(root, path));
            if source
                .as_ref()
                .is_some_and(|source| setting_read(source, name))
            {
                violations.push(format!("retired setting {name} is still read at {path}"));
            }
        }
    }

    assert_eq!(PLANNED_PATHS.len(), 23);
    let planned: BTreeSet<_> = PLANNED_PATHS.iter().copied().collect();
    assert_eq!(planned.len(), 23, "planned paths must be unique");
    for path in planned {
        if !root.join(path).is_file() {
            violations.push(format!("planned domain-9 wave-6 path is missing: {path}"));
        }
    }
    assert!(
        violations.is_empty(),
        "domain 9 is incomplete:\n{}",
        violations.join("\n")
    );
}
