import { supportsProgramGuestAbi } from "./generated/receipt.js";
import { encodeNativeProgramCall, encodeNativeProgramCallV1, type NativeProgramCall, type NativeProgramCallV1 } from "./native-program-call.js";
import type { SelectableProtocolVersion } from "./verifier.js";
import { DEFAULT_PROTOCOL_VERSION, isSelectableProtocolVersion } from "./verifier.js";
import { PlatformSdkError, type ProductionClient } from "./production.js";
import type { DecodedSignedProgramCall } from "./program-wire.js";
import type { ProgramLifecycleSubmission } from "./program-lifecycle.js";

export type { OccupancyPayer } from "./program-wire.js";

const HEX32 = /^[0-9a-f]{64}$/u;
const MAX_U128 = 340282366920938463463374607431768211455n;
const MAX_CALLDATA = 1_048_576;
const DEFAULT_MAXIMUM_SIMULATION_AGE_MILLISECONDS = 300_000n;
const CAPABILITY_ORDER = Object.freeze({
  storage_read: 1,
  storage_write: 2,
  transfer: 3,
  emit_event: 4,
  compose: 5,
} as const);

export interface ProgramBudget { readonly fuel: bigint; readonly feeLimit: bigint }
export type ProgramCapability = keyof typeof CAPABILITY_ORDER;
export interface ProgramCall {
  readonly nativeCall?: NativeProgramCallV1;
  readonly programId: string;
  readonly calldata: Uint8Array;
  readonly budget: ProgramBudget;
  readonly capabilities: readonly ProgramCapability[];
  readonly signedActivity: Uint8Array;
}

export class NativeProgramRequest implements ProgramCall {
  readonly programId: string;
  readonly calldata: Uint8Array;
  readonly budget: ProgramBudget;
  readonly capabilities: readonly ProgramCapability[] = [];
  constructor(readonly nativeCall: NativeProgramCall, feeLimit: bigint, readonly signedActivity: Uint8Array) {
    encodeNativeProgramCall(nativeCall);
    this.programId = hex(nativeCall.programId); this.calldata = nativeCall.calldata;
    this.budget = { fuel: nativeCall.resources[0], feeLimit };
  }
}

export class NativeProgramRequestV1 implements ProgramCall {
  readonly programId: string;
  readonly calldata: Uint8Array;
  readonly budget: ProgramBudget;
  readonly capabilities: readonly ProgramCapability[] = [];
  readonly nativeCall: NativeProgramCallV1;
  readonly signedActivity: Uint8Array;
  constructor(call: NativeProgramCallV1, feeLimit: bigint, signedActivity: Uint8Array) {
    const snapshot = { ...call, programId: new Uint8Array(call.programId), calldata: new Uint8Array(call.calldata),
      capabilities: new Uint8Array(call.capabilities), accessDeclaration: new Uint8Array(call.accessDeclaration),
      resources: [...call.resources] as NativeProgramCallV1["resources"] };
    encodeNativeProgramCallV1(snapshot);
    this.nativeCall = snapshot; this.programId = hex(snapshot.programId); this.calldata = snapshot.calldata;
    this.budget = { fuel: snapshot.resources[0], feeLimit }; this.signedActivity = new Uint8Array(signedActivity);
  }
}

export interface ProgramDiscovery { readonly program_id: string; readonly lifecycle: "active" | "deprecated" | "tombstoned"; readonly version: number; readonly code_hash: string; readonly abi_version: number; readonly receipt_digest: string; readonly state_root: string; readonly observed_sequence: string; readonly observed_at: string; readonly valid_through: string; readonly verification: "server-side-receipt-verification-only" }
export type ProgramSource = Readonly<{ status: "unpublished" }> | Readonly<{ status: "verified"; source_digest: string; environment_digest: string; pipeline: "sha256-source-artifact-reproducible-build-v1" }> | Readonly<{ status: "mismatch"; expected_code_hash: string; reproduced_artifact_digest: string }>;
export interface ProgramInterface { readonly program_id: string; readonly version: number; readonly code_hash: string; readonly abi_version: number; readonly interface: string; readonly interface_digest: string; readonly receipt_digest: string; readonly state_root: string; readonly observed_sequence: string; readonly observed_at: string; readonly valid_through: string; readonly source: ProgramSource; readonly verification: "server-side-receipt-verification-only" }
export type ProgramFailure = Readonly<{ kind: "unknown_program" | "reentrancy" | "depth_exceeded" | "fanout_exceeded" | "guest_refused" | "authority" | "resource" | "response" | "fault"; code?: number; limit?: number; attempted?: number }>;
export type ProgramOutcome = Readonly<{ kind: "completed"; code: number; response: string }> | Readonly<{ kind: "legacy_completed"; code: number; values: readonly unknown[] }> | Readonly<{ kind: "refused"; failure: ProgramFailure }>;
export interface ProgramUsage { readonly cpu_fuel: string; readonly memory_bytes: string; readonly storage_read_bytes: string; readonly storage_write_bytes: string; readonly output_values: number; readonly output_bytes: string; readonly fee_units: string }
export interface ProgramAuthorityDocument { readonly batch_id: string; readonly asset: string; readonly previous_state_root: string; readonly resulting_state_root: string; readonly sequencer_public_key: string }
export interface ProgramExecutionDocument { readonly state: "executed" | "refused" | "simulated"; readonly activity_id: string; readonly program_id: string; readonly guest_abi_version: number; readonly module_version: number; readonly batch_id: string; readonly global_sequence: string; readonly result_code: number; readonly state_root: string; readonly receipt: string; readonly receipt_digest: string; readonly terminal_payload: string; readonly call_graph: string; readonly authority: ProgramAuthorityDocument; readonly usage: ProgramUsage; readonly outcome: ProgramOutcome; readonly verification: "receipt-terminal-and-call-graph-verified"; readonly idempotency_key?: string; readonly retained_signed_activity?: string }
export interface ProgramUnknownSubmission { readonly state: "unknown"; readonly activity_id: string; readonly idempotency_key: string; readonly retained_signed_activity?: string }
export type ProgramSubmission = ProgramUnknownSubmission | (ProgramExecutionDocument & Readonly<{ state: "executed" | "refused" }>);
export interface ProgramSimulationEvidence { readonly boundary_id: string; readonly activity_id: string; readonly previous_state_root: string; readonly hypothetical_state_root: string; readonly observed_sequence: string; readonly observed_at: string; readonly committed: false; readonly public_key: string; readonly signature: string }
export interface ProgramSimulation { readonly committed: false; readonly execution: ProgramExecutionDocument & Readonly<{ state: "simulated" }>; readonly simulation_evidence: ProgramSimulationEvidence }

export class ProgramTrustContext {
  readonly #sequencerPublicKey: Uint8Array;
  readonly #clockMilliseconds: () => bigint;
  readonly #maximumSimulationAgeMilliseconds: bigint;
  readonly #protocolVersion: SelectableProtocolVersion;

  public constructor(
    sequencerPublicKey: Uint8Array,
    clockMilliseconds: () => bigint = () => BigInt(Date.now()),
    maximumSimulationAgeMilliseconds: bigint = DEFAULT_MAXIMUM_SIMULATION_AGE_MILLISECONDS,
    protocolVersion: SelectableProtocolVersion = DEFAULT_PROTOCOL_VERSION,
  ) {
    if (sequencerPublicKey.length !== 32 || sequencerPublicKey.every((value) => value === 0)
      || maximumSimulationAgeMilliseconds <= 0n
      || maximumSimulationAgeMilliseconds > 18446744073709551615n
      || !isSelectableProtocolVersion(protocolVersion)) throw new TypeError("invalid Programs trust context");
    this.#sequencerPublicKey = new Uint8Array(sequencerPublicKey);
    this.#clockMilliseconds = clockMilliseconds;
    this.#maximumSimulationAgeMilliseconds = maximumSimulationAgeMilliseconds;
    this.#protocolVersion = protocolVersion;
    Object.freeze(this);
  }

  public sequencerPublicKey(): Uint8Array { return new Uint8Array(this.#sequencerPublicKey); }
  public protocolVersion(): SelectableProtocolVersion { return this.#protocolVersion; }
  public nowMilliseconds(): bigint {
    const value = this.#clockMilliseconds();
    if (value < 0n || value > 18446744073709551615n) throw new TypeError("invalid trust clock");
    return value;
  }
  public maximumSimulationAgeMilliseconds(): bigint { return this.#maximumSimulationAgeMilliseconds; }
}

export class ProgramOperations {
  readonly #heads = new Map<string, ProgramHeadObservation>();
  readonly #gatewayDiscoveries = new Map<string, GatewayProgramDiscovery>();

  public constructor(private readonly client: ProductionClient, private readonly trust: ProgramTrustContext) {}

  public async discover(programId: string): Promise<ProgramDiscovery> {
    if (!HEX32.test(programId)) throw new TypeError("invalid program id");
    const value = await this.client.agent<unknown, unknown>("program.discover", { program_id: programId, requested_verification_level: "sequencer-signed" });
    const result = "deployment_receipt_digest" in object(value)
      ? await verifyGatewayProgramDiscovery(value, programId, this.trust)
      : discovery(value, programId, this.trust.nowMilliseconds());
    this.#rememberHead(programId, head(result));
    if ("deployment_receipt_digest" in result) this.#gatewayDiscoveries.set(programId, result as GatewayProgramDiscovery);
    return result;
  }

  public async interface(programId: string): Promise<ProgramInterface> {
    if (!HEX32.test(programId)) throw new TypeError("invalid program id");
    const value = await this.client.agent<unknown, unknown>("program.interface", { program_id: programId, requested_verification_level: "sequencer-signed" });
    const observed = this.#gatewayDiscoveries.get(programId);
    const result = observed === undefined
      ? await programInterface(value, programId, this.trust.nowMilliseconds())
      : await verifyGatewayProgramInterface(value, programId, this.trust, observed);
    this.#rememberHead(programId, head(result));
    return result;
  }

  #rememberHead(programId: string, candidate: ProgramHeadObservation): void {
    const current = this.#heads.get(programId);
    if (current !== undefined && (candidate.sequence < current.sequence
      || candidate.observedAt < current.observedAt
      || (candidate.sequence === current.sequence && (candidate.stateRoot !== current.stateRoot
        || candidate.validThrough < current.validThrough)))) throw new TypeError("program head rollback or conflict");
    this.#heads.set(programId, candidate);
  }
}

function unknownLifecycleSubmission(binding: DecodedSignedProgramCall): ProgramLifecycleSubmission {
  return Object.freeze({ state: "unknown", activity_id: binding.activityId, idempotency_key: binding.idempotencyKey, retained_signed_activity: hex(binding.canonicalBytes) });
}

export function resolveLifecycleFailure(error: unknown, binding: DecodedSignedProgramCall): ProgramLifecycleSubmission {
  if (error instanceof PlatformSdkError && ["unknown-outcome", "decode-failure", "verification-failure"].includes(error.code)) return unknownLifecycleSubmission(binding);
  throw error;
}

interface ProgramHeadObservation { readonly stateRoot: string; readonly sequence: bigint; readonly observedAt: bigint; readonly validThrough: bigint }

export function parseProgramExecutionDocument(value: unknown): ProgramExecutionDocument {
  const candidate = object(value);
  if (candidate.state !== "executed" && candidate.state !== "refused" && candidate.state !== "simulated") throw new TypeError("invalid program execution state");
  return executionDocument(candidate, candidate.state);
}

function executionDocument(candidate: Readonly<Record<string, unknown>>, state: "executed" | "refused" | "simulated"): ProgramExecutionDocument {
  if (candidate.state !== state) throw new TypeError("invalid program execution state");
  exactKeys(candidate, ["state", "activity_id", "program_id", "guest_abi_version", "module_version", "batch_id",
    "global_sequence", "result_code", "state_root", "receipt", "receipt_digest", "terminal_payload", "call_graph",
    "authority", "usage", "outcome", "verification"], ["idempotency_key", "retained_signed_activity"]);
  const usage = object(candidate.usage);
  const authority = object(candidate.authority);
  exactKeys(authority, ["batch_id", "asset", "previous_state_root", "resulting_state_root", "sequencer_public_key"]);
  exactKeys(usage, ["cpu_fuel", "memory_bytes", "storage_read_bytes", "storage_write_bytes", "output_values", "output_bytes", "fee_units"]);
  const result: ProgramExecutionDocument = Object.freeze({
    state,
    activity_id: requiredHex32(candidate, "activity_id"),
    program_id: requiredHex32(candidate, "program_id"),
    guest_abi_version: exactInteger(candidate.guest_abi_version, 1, 2),
    module_version: exactInteger(candidate.module_version, 1, 4),
    batch_id: requiredHex32(candidate, "batch_id"),
    global_sequence: decimal(candidate.global_sequence),
    result_code: exactInteger(candidate.result_code, -2147483648, 2147483647),
    state_root: requiredHex32(candidate, "state_root"),
    receipt: requiredHex(candidate, "receipt", MAX_CALLDATA),
    receipt_digest: requiredHex32(candidate, "receipt_digest"),
    terminal_payload: requiredHex(candidate, "terminal_payload", MAX_CALLDATA),
    call_graph: requiredHex(candidate, "call_graph", MAX_CALLDATA),
    authority: Object.freeze({
      batch_id: requiredHex32(authority, "batch_id"),
      asset: requiredHex32(authority, "asset"),
      previous_state_root: requiredHex32(authority, "previous_state_root"),
      resulting_state_root: requiredHex32(authority, "resulting_state_root"),
      sequencer_public_key: requiredHex32(authority, "sequencer_public_key"),
    }),
    usage: Object.freeze({
      cpu_fuel: decimal(usage.cpu_fuel), memory_bytes: decimal(usage.memory_bytes),
      storage_read_bytes: decimal(usage.storage_read_bytes), storage_write_bytes: decimal(usage.storage_write_bytes),
      output_values: exactInteger(usage.output_values, 0, 0xffff_ffff), output_bytes: decimal(usage.output_bytes), fee_units: decimal(usage.fee_units, true),
    }),
    outcome: programOutcome(candidate.outcome),
    verification: candidate.verification === "receipt-terminal-and-call-graph-verified"
      ? candidate.verification : (() => { throw new TypeError("invalid program verification status"); })(),
    ...(candidate.idempotency_key === undefined ? {} : { idempotency_key: requiredHex32(candidate, "idempotency_key") }),
    ...(candidate.retained_signed_activity === undefined ? {} : { retained_signed_activity: requiredHex(candidate, "retained_signed_activity", MAX_CALLDATA) }),
  });
  if ((state === "refused" && result.outcome.kind !== "refused")
    || (state === "executed" && result.outcome.kind === "refused")) throw new TypeError("program state/outcome mismatch");
  return result;
}

function executionDocumentV5(candidate: Readonly<Record<string, unknown>>, state: "executed" | "refused" | "simulated"): ProgramExecutionDocument {
  if (candidate.state !== state) throw new TypeError("invalid program execution state");
  exactKeys(candidate, ["state", "activity_id", "program_id", "guest_abi_version", "module_version", "batch_id",
    "global_sequence", "result_code", "state_root", "receipt", "receipt_digest", "terminal_payload", "call_graph",
    "authority", "usage", "outcome", "verification"], ["idempotency_key", "retained_signed_activity"]);
  const usage = object(candidate.usage);
  const authority = object(candidate.authority);
  exactKeys(authority, ["batch_id", "asset", "previous_state_root", "resulting_state_root", "sequencer_public_key"]);
  exactKeys(usage, ["cpu_fuel", "memory_bytes", "storage_read_bytes", "storage_write_bytes", "output_values", "output_bytes", "fee_units"]);
  const result: ProgramExecutionDocument = Object.freeze({
    state,
    activity_id: requiredHex32(candidate, "activity_id"),
    program_id: requiredHex32(candidate, "program_id"),
    guest_abi_version: exactInteger(candidate.guest_abi_version, 3, 5),
    module_version: exactInteger(candidate.module_version, 1, 5),
    batch_id: requiredHex32(candidate, "batch_id"),
    global_sequence: decimal(candidate.global_sequence),
    result_code: exactInteger(candidate.result_code, -2147483648, 2147483647),
    state_root: requiredHex32(candidate, "state_root"),
    receipt: requiredHex(candidate, "receipt", MAX_CALLDATA),
    receipt_digest: requiredHex32(candidate, "receipt_digest"),
    terminal_payload: requiredHex(candidate, "terminal_payload", MAX_CALLDATA),
    call_graph: requiredHex(candidate, "call_graph", MAX_CALLDATA),
    authority: Object.freeze({
      batch_id: requiredHex32(authority, "batch_id"),
      asset: requiredHex32(authority, "asset"),
      previous_state_root: requiredHex32(authority, "previous_state_root"),
      resulting_state_root: requiredHex32(authority, "resulting_state_root"),
      sequencer_public_key: requiredHex32(authority, "sequencer_public_key"),
    }),
    usage: Object.freeze({
      cpu_fuel: decimal(usage.cpu_fuel), memory_bytes: decimal(usage.memory_bytes),
      storage_read_bytes: decimal(usage.storage_read_bytes), storage_write_bytes: decimal(usage.storage_write_bytes),
      output_values: exactInteger(usage.output_values, 0, 0xffff_ffff), output_bytes: decimal(usage.output_bytes), fee_units: decimal(usage.fee_units, true),
    }),
    outcome: programOutcome(candidate.outcome),
    verification: candidate.verification === "receipt-terminal-and-call-graph-verified"
      ? candidate.verification : (() => { throw new TypeError("invalid program verification status"); })(),
    ...(candidate.idempotency_key === undefined ? {} : { idempotency_key: requiredHex32(candidate, "idempotency_key") }),
    ...(candidate.retained_signed_activity === undefined ? {} : { retained_signed_activity: requiredHex(candidate, "retained_signed_activity", MAX_CALLDATA) }),
  });
  if ((state === "refused" && result.outcome.kind !== "refused")
    || (state === "executed" && result.outcome.kind === "refused")) throw new TypeError("program state/outcome mismatch");
  return result;
}

export function parseProgramExecutionDocumentV5(value: unknown): ProgramExecutionDocument {
  const candidate = object(value);
  if (candidate.state !== "executed" && candidate.state !== "refused" && candidate.state !== "simulated") throw new TypeError("invalid program execution state");
  return executionDocumentV5(candidate, candidate.state);
}

export interface GatewayProgramDiscovery extends ProgramDiscovery {
  readonly deployment_receipt_digest: string;
  readonly discovery_verification: "sequencer-signed" | "server-side-receipt-verification-only";
  readonly discovery_public_key?: string;
  readonly discovery_signature?: string;
}

export async function verifyGatewayProgramDiscovery(value: unknown, programId: string, trust: ProgramTrustContext): Promise<GatewayProgramDiscovery> {
  const candidate = object(structuredClone(value));
  exactKeys(candidate, ["program_id", "lifecycle", "version", "code_hash", "abi_version", "receipt_digest",
    "deployment_receipt_digest", "state_root", "observed_sequence", "observed_at", "valid_through", "verification"],
    ["discovery_public_key", "discovery_signature", "value_accounts"]);
  if (!HEX32.test(programId) || programId === "0".repeat(64) || requiredHex32(candidate, "program_id") !== programId
    || candidate.verification !== "registry-receipt-and-current-head-verified"
    || trust.protocolVersion() !== 3 || !supportsProgramGuestAbi(candidate.abi_version as number)
    || (candidate.lifecycle !== "active" && candidate.lifecycle !== "deprecated" && candidate.lifecycle !== "tombstoned")) {
    throw new TypeError("invalid native gateway discovery");
  }
  if ("value_accounts" in candidate) object(candidate.value_accounts);
  const version = exactInteger(candidate.version, 1, 0xffff_ffff);
  const abi = candidate.abi_version as 1 | 2 | 3 | 4 | 5;
  const integers = new Uint8Array(7); const view = new DataView(integers.buffer);
  integers[0] = 1; view.setUint32(1, version); view.setUint16(5, abi);
  const codeHash = requiredHex32(candidate, "code_hash"), stateRoot = requiredHex32(candidate, "state_root");
  const sequence = decimal(candidate.observed_sequence), observed = decimal(candidate.observed_at), through = decimal(candidate.valid_through);
  const digestBytes = await digest(concat(new TextEncoder().encode("LayerX/program-discovery-proof/v1\0"),
    decodeHex(programId, 32), integers.subarray(0, 5), decodeHex(codeHash, 32), integers.subarray(5),
    u64(BigInt(sequence)), u64(BigInt(observed)), u64(BigInt(through)), decodeHex(stateRoot, 32)));
  if (hex(digestBytes) !== requiredHex32(candidate, "receipt_digest")) throw new TypeError("native discovery digest mismatch");
  const signed = "discovery_public_key" in candidate || "discovery_signature" in candidate;
  if (abi >= 3 && !signed) throw new TypeError("native ABI requires signed discovery");
  let proof: Readonly<{ discovery_public_key: string; discovery_signature: string }> | undefined;
  if (signed) {
    const publicKey = requiredHex32(candidate, "discovery_public_key"), signature = requiredHex(candidate, "discovery_signature", 64);
    if (signature.length !== 128 || publicKey !== hex(trust.sequencerPublicKey())) throw new TypeError("discovery sequencer key does not match pin");
    const key = await globalThis.crypto.subtle.importKey("raw", buffer(decodeHex(publicKey, 32)), { name: "Ed25519" }, false, ["verify"]);
    if (!await globalThis.crypto.subtle.verify("Ed25519", key, buffer(decodeHex(signature, 64)), buffer(digestBytes))) throw new TypeError("discovery signature mismatch");
    proof = { discovery_public_key: publicKey, discovery_signature: signature };
  }
  const result: GatewayProgramDiscovery = Object.freeze({ program_id: programId, lifecycle: candidate.lifecycle, version,
    code_hash: codeHash, abi_version: abi, receipt_digest: hex(digestBytes), deployment_receipt_digest: requiredHex32(candidate, "deployment_receipt_digest"),
    state_root: stateRoot, observed_sequence: sequence, observed_at: observed, valid_through: through,
    verification: "server-side-receipt-verification-only", discovery_verification: signed ? "sequencer-signed" : "server-side-receipt-verification-only",
    ...(proof ?? {}) });
  requireFreshHead(head(result), trust.nowMilliseconds());
  return result;
}

export async function verifyGatewayProgramInterface(value: unknown, programId: string, trust: ProgramTrustContext,
  observed: GatewayProgramDiscovery): Promise<ProgramInterface> {
  observed = Object.freeze(structuredClone(observed));
  const candidate = object(structuredClone(value));
  const { discovery_verification: _presentation, ...packet } = observed;
  await verifyGatewayProgramDiscovery({ ...packet, verification: "registry-receipt-and-current-head-verified" }, programId, trust);
  exactKeys(candidate, ["program_id", "version", "code_hash", "abi_version", "interface", "interface_digest",
    "receipt_digest", "state_root", "observed_sequence", "observed_at", "valid_through", "source", "verification"]);
  if (programId !== observed.program_id || requiredHex32(candidate, "program_id") !== programId
    || candidate.verification !== "deployment-interface-and-current-head-verified" || trust.protocolVersion() !== 3
    || candidate.abi_version !== observed.abi_version || candidate.version !== observed.version
    || candidate.code_hash !== observed.code_hash || candidate.state_root !== observed.state_root
    || candidate.observed_sequence !== observed.observed_sequence || candidate.observed_at !== observed.observed_at
    || candidate.valid_through !== observed.valid_through || candidate.receipt_digest !== observed.deployment_receipt_digest
    || observed.abi_version >= 3 && observed.discovery_verification !== "sequencer-signed") throw new TypeError("interface does not match discovered deployment and head");
  requireFreshHead(head(observed), trust.nowMilliseconds());
  const iface = requiredHex(candidate, "interface", 952);
  const ifaceDigest = requiredHex32(candidate, "interface_digest");
  if (iface.length === 0 || hex(await digest(decodeHex(iface, 952))) !== ifaceDigest) throw new TypeError("program interface digest mismatch");
  return Object.freeze({ program_id: programId, version: observed.version, code_hash: observed.code_hash, abi_version: observed.abi_version,
    interface: iface, interface_digest: ifaceDigest, receipt_digest: observed.deployment_receipt_digest, state_root: observed.state_root,
    observed_sequence: observed.observed_sequence, observed_at: observed.observed_at, valid_through: observed.valid_through,
    source: programSource(candidate.source), verification: "server-side-receipt-verification-only" });
}

function discovery(value: unknown, programId: string, now: bigint): ProgramDiscovery {
  const candidate = object(value);
  exactKeys(candidate, ["program_id", "lifecycle", "version", "code_hash", "abi_version", "receipt_digest",
    "state_root", "observed_sequence", "observed_at", "valid_through", "verification"]);
  if (requiredHex32(candidate, "program_id") !== programId || candidate.verification !== "registry-receipt-and-current-head-verified") throw new TypeError("unverified program discovery");
  if (candidate.lifecycle !== "active" && candidate.lifecycle !== "deprecated" && candidate.lifecycle !== "tombstoned") throw new TypeError("invalid program lifecycle");
  const result: ProgramDiscovery = Object.freeze({
    program_id: programId, lifecycle: candidate.lifecycle, version: exactInteger(candidate.version, 1, 0xffff_ffff),
    code_hash: requiredHex32(candidate, "code_hash"), abi_version: exactInteger(candidate.abi_version, 1, 2),
    receipt_digest: requiredHex32(candidate, "receipt_digest"), state_root: requiredHex32(candidate, "state_root"),
    observed_sequence: decimal(candidate.observed_sequence), observed_at: decimal(candidate.observed_at), valid_through: decimal(candidate.valid_through),
    verification: "server-side-receipt-verification-only",
  });
  requireFreshHead(head(result), now);
  return result;
}

async function programInterface(value: unknown, programId: string, now: bigint): Promise<ProgramInterface> {
  const candidate = object(value);
  exactKeys(candidate, ["program_id", "version", "code_hash", "abi_version", "interface", "interface_digest",
    "receipt_digest", "state_root", "observed_sequence", "observed_at", "valid_through", "source", "verification"]);
  if (requiredHex32(candidate, "program_id") !== programId || candidate.verification !== "deployment-interface-and-current-head-verified") throw new TypeError("unverified program interface");
  const encodedInterface = requiredHex(candidate, "interface", 952);
  if (encodedInterface.length === 0 || hex(await digest(decodeHex(encodedInterface, 952))) !== requiredHex32(candidate, "interface_digest")) throw new TypeError("program interface digest mismatch");
  const result: ProgramInterface = Object.freeze({
    program_id: programId, version: exactInteger(candidate.version, 1, 0xffff_ffff), code_hash: requiredHex32(candidate, "code_hash"),
    abi_version: exactInteger(candidate.abi_version, 1, 2), interface: encodedInterface, interface_digest: requiredHex32(candidate, "interface_digest"),
    receipt_digest: requiredHex32(candidate, "receipt_digest"), state_root: requiredHex32(candidate, "state_root"),
    observed_sequence: decimal(candidate.observed_sequence), observed_at: decimal(candidate.observed_at), valid_through: decimal(candidate.valid_through),
    source: programSource(candidate.source), verification: "server-side-receipt-verification-only",
  });
  requireFreshHead(head(result), now);
  return result;
}

function programSource(value: unknown): ProgramSource {
  const source = object(value);
  if (source.status === "unpublished") {
    exactKeys(source, ["status"]);
    return Object.freeze({ status: "unpublished" });
  }
  if (source.status === "verified") {
    exactKeys(source, ["status", "source_digest", "environment_digest", "pipeline"]);
    if (source.pipeline !== "sha256-source-artifact-reproducible-build-v1") throw new TypeError("invalid program source pipeline");
    return Object.freeze({ status: "verified", source_digest: requiredHex32(source, "source_digest"),
      environment_digest: requiredHex32(source, "environment_digest"), pipeline: source.pipeline });
  }
  if (source.status === "mismatch") {
    exactKeys(source, ["status", "expected_code_hash", "reproduced_artifact_digest"]);
    return Object.freeze({ status: "mismatch", expected_code_hash: requiredHex32(source, "expected_code_hash"),
      reproduced_artifact_digest: requiredHex32(source, "reproduced_artifact_digest") });
  }
  throw new TypeError("invalid program source status");
}

function programOutcome(value: unknown): ProgramOutcome {
  const outcome = object(value);
  if (outcome.kind === "completed") {
    exactKeys(outcome, ["kind", "code", "response"]);
    return Object.freeze({ kind: "completed", code: exactInteger(outcome.code, 0, 0x7fff_ffff),
      response: requiredHex(outcome, "response", MAX_CALLDATA) });
  }
  if (outcome.kind === "legacy_completed") {
    exactKeys(outcome, ["kind", "code", "values"]);
    if (!Array.isArray(outcome.values) || outcome.values.length > Math.floor(MAX_CALLDATA / 5)) throw new TypeError("invalid legacy program values");
    const values = outcome.values.map((raw) => {
      const valueDocument = object(raw);
      exactKeys(valueDocument, ["type", "value"]);
      if (valueDocument.type === "i32") return Object.freeze({ type: "i32", value: exactInteger(valueDocument.value, -2147483648, 2147483647) });
      if (valueDocument.type === "i64" && typeof valueDocument.value === "string" && /^-?(0|[1-9][0-9]*)$/u.test(valueDocument.value)) {
        const parsed = BigInt(valueDocument.value);
        if (parsed >= -9223372036854775808n && parsed <= 9223372036854775807n) return Object.freeze({ type: "i64", value: valueDocument.value });
      }
      throw new TypeError("invalid legacy program value");
    });
    return Object.freeze({ kind: "legacy_completed", code: exactInteger(outcome.code, -2147483648, 2147483647), values: Object.freeze(values) });
  }
  if (outcome.kind === "refused") {
    exactKeys(outcome, ["kind", "failure"]);
    return Object.freeze({ kind: "refused", failure: programFailure(outcome.failure) });
  }
  throw new TypeError("invalid program outcome");
}

function programFailure(value: unknown): ProgramFailure {
  const failure = object(value);
  const kind = failure.kind;
  if (kind === "depth_exceeded" || kind === "fanout_exceeded") {
    exactKeys(failure, ["kind", "limit", "attempted"]);
    return Object.freeze({ kind, limit: exactInteger(failure.limit, 0, 0xffff_ffff),
      attempted: exactInteger(failure.attempted, 0, 0xffff_ffff) });
  }
  if (kind === "guest_refused") {
    exactKeys(failure, ["kind", "code"]);
    return Object.freeze({ kind, code: exactInteger(failure.code, -2147483648, 2147483647) });
  }
  if (kind === "unknown_program" || kind === "reentrancy" || kind === "authority" || kind === "resource"
    || kind === "response" || kind === "fault") {
    exactKeys(failure, ["kind"]);
    return Object.freeze({ kind });
  }
  throw new TypeError("invalid program failure");
}

function head(value: ProgramDiscovery | ProgramInterface): ProgramHeadObservation {
  return Object.freeze({ stateRoot: value.state_root, sequence: BigInt(value.observed_sequence),
    observedAt: BigInt(value.observed_at), validThrough: BigInt(value.valid_through) });
}

function requireFreshHead(value: ProgramHeadObservation, now: bigint): void {
  if (value.observedAt > now || now > value.validThrough || value.validThrough < value.observedAt) throw new TypeError("stale program head");
}

function object(value: unknown): Readonly<Record<string, unknown>> { if (value === null || typeof value !== "object" || Array.isArray(value)) throw new TypeError("invalid program document"); return value as Readonly<Record<string, unknown>>; }
function exactKeys(value: Readonly<Record<string, unknown>>, required: readonly string[], optional: readonly string[] = []): void { const allowed = new Set([...required, ...optional]); if (required.some((key) => !(key in value)) || Object.keys(value).some((key) => !allowed.has(key))) throw new TypeError("invalid program document fields"); }
function requiredHex32(value: Readonly<Record<string, unknown>>, field: string): string { const candidate = value[field]; if (typeof candidate !== "string" || !HEX32.test(candidate)) throw new TypeError("invalid program identity"); return candidate; }
function requiredHex(value: Readonly<Record<string, unknown>>, field: string, maximum: number, exact?: number): string { const candidate = value[field]; if (typeof candidate !== "string" || candidate.length % 2 !== 0 || candidate.length > maximum * 2 || (exact !== undefined && candidate.length !== exact * 2) || !/^[0-9a-f]*$/u.test(candidate)) throw new TypeError("invalid program hexadecimal field"); return candidate; }
function decimal(value: unknown, u128 = false): string { if (typeof value !== "string" || !/^(0|[1-9][0-9]*)$/u.test(value)) throw new TypeError("invalid program decimal"); const parsed = BigInt(value); if (parsed > (u128 ? MAX_U128 : 18446744073709551615n)) throw new TypeError("program decimal overflow"); return value; }
function exactInteger(value: unknown, minimum: number, maximum: number): number { if (typeof value !== "number" || !Number.isSafeInteger(value) || value < minimum || value > maximum) throw new TypeError("invalid program integer"); return value; }
function hex(value: Uint8Array): string { return Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join(""); }
function decodeHex(value: string, maximum: number): Uint8Array { if (value.length % 2 !== 0 || value.length > maximum * 2 || !/^[0-9a-f]*$/u.test(value)) throw new TypeError("invalid hexadecimal evidence"); return Uint8Array.from(value.match(/.{2}/gu) ?? [], (pair) => Number.parseInt(pair, 16)); }
async function digest(value: Uint8Array): Promise<Uint8Array> { const copy = new Uint8Array(value); return new Uint8Array(await globalThis.crypto.subtle.digest("SHA-256", copy.buffer)); }
function concat(...values: readonly Uint8Array[]): Uint8Array { const output = new Uint8Array(values.reduce((length, value) => length + value.length, 0)); let offset = 0; for (const value of values) { output.set(value, offset); offset += value.length; } return output; }
function u64(value: bigint): Uint8Array { if (value < 0n || value > 18446744073709551615n) throw new TypeError("u64 overflow"); const encoded = new Uint8Array(8); let remaining = value; for (let index = 7; index >= 0; index -= 1) { encoded[index] = Number(remaining & 0xffn); remaining >>= 8n; } return encoded; }
function buffer(value: Uint8Array): ArrayBuffer { const encoded = new ArrayBuffer(value.length); new Uint8Array(encoded).set(value); return encoded; }

export function platform_sdk_programs(): string { return "receipt-verified-program-operations-v1"; }
