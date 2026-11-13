# Web client contract

The [web application](../../../apps/web) provides application routes, authenticated clients, explorer views and wallet settings. API, state and UI declarations below belong to the current source tree. Chain transactions use the Paxeer network; finality follows block commitment.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## api

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export type AbiReadField = Readonly<{ name: string; type: AbiReadType }>;` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export interface AbiReadRecord` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export type AbiReadScalar = "address" \| "bool" \| "bytes" \| "bytes32" \| "string" \| AbiReadUint;` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export type AbiReadType =` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export type AbiReadUint = "uint8" \| "uint32" \| "uint64" \| "uint256";` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export type AbiReadValue = bigint \| boolean \| string \| readonly AbiReadValue[] \| AbiReadRecord;` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export function decodeAbiResult(returnData: string, outputs: readonly AbiReadField[]): AbiReadRecord` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export function readFlag(value: AbiReadValue \| undefined, label: string): boolean` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export function readList(value: AbiReadValue \| undefined, label: string): readonly AbiReadValue[]` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export function readRecord(value: AbiReadValue \| undefined, label: string): AbiReadRecord` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export function readText(value: AbiReadValue \| undefined, label: string): string` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export function readUint(value: AbiReadValue \| undefined, label: string): bigint` | [apps/web/src/api/abi-read.ts](../../../apps/web/src/api/abi-read.ts) |
| `ts_export` | `export type AccountDidOutcome =` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `ts_export` | `export type GasQuoteOutcome =` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `ts_export` | `export async function requestSidioraGasQuote(request: GasQuoteRequest, chainId: bigint): Promise<GasQuoteOutcome>` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `ts_export` | `export async function sendSponsoredBatch( quoted: SidioraGasQuote, accountSignature: string, authorizationNonce: bigint, authorizationSignature: string, ): Promise<PrecompileSendOutcome>` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `ts_export` | `export async function sidioraAccountDid(account: string): Promise<AccountDidOutcome>` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `ts_export` | `export interface SidioraGasQuote` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `ts_export` | `export async function sidioraStationMetadata(): Promise<Readonly<` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `ts_export` | `export async function ethBlockNumber(): Promise<bigint>` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export async function ethCall(` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export async function ethGetLogs(` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export async function ethTransactionReceipt(hash: string): Promise<GatewayReceipt \| undefined>` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export type ForkSurface = "exchange" \| "bridge" \| "launchpad";` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export interface GatewayLog extends PrecompileLog` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export class GatewayRpcError extends Error` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export class GatewayUnavailableError extends Error` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export const HISTORY_PAGE_LIMIT = 25;` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export type HistoryChain = "layerx" \| "paxeer";` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export interface HistoryOptions` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export interface HistoryPage` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export interface HistoryRow` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export interface NetworkCapabilities` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export async function paxeerHistory(account: string, options: HistoryOptions = {}): Promise<HistoryPage>` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export type PrecompileView = Readonly<Pick<PrecompileCall, "to" \| "data">>;` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export async function pxGetCapabilities(): Promise<NetworkCapabilities>` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export async function pxResolveAccount(account: string): Promise<ResolvedAccount>` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export interface ResolvedAccount` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export const SURFACE_UNAVAILABLE_CODE = -32003;` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export type SurfaceCapability = Readonly<{ live: boolean; detail: string \| null }>;` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export async function surfaceCapability(surface: ForkSurface): Promise<SurfaceCapability>` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export async function unifiedHistory(account: string, options: HistoryOptions = {}): Promise<HistoryPage>` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `ts_export` | `export const conformance: { readonly [name in OperationName]: (run: ConformanceRun) => Promise<JsonValue> } =` | [apps/web/src/api/generated/conformance.ts](../../../apps/web/src/api/generated/conformance.ts) |
| `ts_export` | `export interface ConformanceRun` | [apps/web/src/api/generated/conformance.ts](../../../apps/web/src/api/generated/conformance.ts) |
| `ts_export` | `export function runBody(run: ConformanceRun): JsonValue` | [apps/web/src/api/generated/conformance.ts](../../../apps/web/src/api/generated/conformance.ts) |
| `ts_export` | `export function runKey(run: ConformanceRun): string` | [apps/web/src/api/generated/conformance.ts](../../../apps/web/src/api/generated/conformance.ts) |
| `ts_export` | `export function runParam(run: ConformanceRun, name: string): string` | [apps/web/src/api/generated/conformance.ts](../../../apps/web/src/api/generated/conformance.ts) |
| `ts_export` | `export interface AccountBalance` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AccountCreateRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AccountCreation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type AccountId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ActivityEntry` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ActivityEntryDetail` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ActivityEntryId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ActivityEntryKind = "deposit" \| "withdrawal" \| "movement" \| "agent-action" \| "approval" \| "security-event";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const activityEntryKindVariants: readonly ActivityEntryKind[] = ["deposit", "withdrawal", "movement", "agent-action", "approval", "security-event"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ActivityFilter` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ActivityGroup` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ActivityPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ActivityQueryRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface Agent` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AgentArchiveRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AgentCreateRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type AgentId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AgentLimitRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AgentPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AgentReclaimRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AgentSpend` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type AgentState = "creating" \| "active" \| "paused" \| "archiving" \| "archived";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const agentStateVariants: readonly AgentState[] = ["creating", "active", "paused", "archiving", "archived"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type Amount = bigint;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ApiError` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ApprovalApproveRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ApprovalDecision` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ApprovalDetail` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ApprovalFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ApprovalId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ApprovalPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ApprovalState = "pending" \| "approved" \| "rejected" \| "expired" \| "defective";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const approvalStateVariants: readonly ApprovalState[] = ["pending", "approved", "rejected", "expired", "defective"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ApprovalSummary` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type AssertionId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AuthenticatorDisable` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type AuthenticatorId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AuthenticatorMethod` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AuthenticatorSetupBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AuthenticatorSetupChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AuthenticatorSetupFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type AuthenticatorSetupId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AuthenticatorSetupResult` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface AuthenticatorStatus` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface BackupCodeRotation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface BackupCodeSet` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface BindingRebindAction` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type BindingState = "none" \| "binding" \| "bound" \| "rebinding";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface BindingStatement` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface BindingStatementRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const bindingStateVariants: readonly BindingState[] = ["none", "binding", "bound", "rebinding"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface BindingSubmission` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ChannelPreference` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ClassToggle` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type CopyKey = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function createHumanApiClient(options: HumanApiClientOptions = {}): HumanApiClient` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type CurrencyCode = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type Cursor = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAccountBalance(value: JsonValue \| undefined, at: string): AccountBalance` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAccountCreateRequest(value: JsonValue \| undefined, at: string): AccountCreateRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAccountCreation(value: JsonValue \| undefined, at: string): AccountCreation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeActivityEntry(value: JsonValue \| undefined, at: string): ActivityEntry` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeActivityEntryDetail(value: JsonValue \| undefined, at: string): ActivityEntryDetail` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeActivityEntryKind(value: JsonValue \| undefined, at: string): ActivityEntryKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeActivityFilter(value: JsonValue \| undefined, at: string): ActivityFilter` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeActivityGroup(value: JsonValue \| undefined, at: string): ActivityGroup` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeActivityPage(value: JsonValue \| undefined, at: string): ActivityPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeActivityQueryRequest(value: JsonValue \| undefined, at: string): ActivityQueryRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAgent(value: JsonValue \| undefined, at: string): Agent` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAgentArchiveRequest(value: JsonValue \| undefined, at: string): AgentArchiveRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAgentCreateRequest(value: JsonValue \| undefined, at: string): AgentCreateRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAgentLimitRequest(value: JsonValue \| undefined, at: string): AgentLimitRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAgentPage(value: JsonValue \| undefined, at: string): AgentPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAgentReclaimRequest(value: JsonValue \| undefined, at: string): AgentReclaimRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAgentSpend(value: JsonValue \| undefined, at: string): AgentSpend` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAgentState(value: JsonValue \| undefined, at: string): AgentState` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeApiError(value: JsonValue \| undefined, at: string): ApiError` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeApprovalApproveRequest(value: JsonValue \| undefined, at: string): ApprovalApproveRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeApprovalDecision(value: JsonValue \| undefined, at: string): ApprovalDecision` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeApprovalDetail(value: JsonValue \| undefined, at: string): ApprovalDetail` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeApprovalFacts(value: JsonValue \| undefined, at: string): ApprovalFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeApprovalPage(value: JsonValue \| undefined, at: string): ApprovalPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeApprovalState(value: JsonValue \| undefined, at: string): ApprovalState` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeApprovalSummary(value: JsonValue \| undefined, at: string): ApprovalSummary` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeArray<Item>( value: JsonValue \| undefined, at: string, item: (value: JsonValue \| undefined, at: string) => Item, ): Item[]` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAuthenticatorDisable(value: JsonValue \| undefined, at: string): AuthenticatorDisable` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAuthenticatorMethod(value: JsonValue \| undefined, at: string): AuthenticatorMethod` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAuthenticatorSetupBegin(value: JsonValue \| undefined, at: string): AuthenticatorSetupBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAuthenticatorSetupChallenge(value: JsonValue \| undefined, at: string): AuthenticatorSetupChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAuthenticatorSetupFinish(value: JsonValue \| undefined, at: string): AuthenticatorSetupFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAuthenticatorSetupResult(value: JsonValue \| undefined, at: string): AuthenticatorSetupResult` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeAuthenticatorStatus(value: JsonValue \| undefined, at: string): AuthenticatorStatus` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeBackupCodeRotation(value: JsonValue \| undefined, at: string): BackupCodeRotation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeBackupCodeSet(value: JsonValue \| undefined, at: string): BackupCodeSet` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeBindingRebindAction(value: JsonValue \| undefined, at: string): BindingRebindAction` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeBindingState(value: JsonValue \| undefined, at: string): BindingState` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeBindingStatement(value: JsonValue \| undefined, at: string): BindingStatement` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeBindingStatementRequest(value: JsonValue \| undefined, at: string): BindingStatementRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeBindingSubmission(value: JsonValue \| undefined, at: string): BindingSubmission` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeChannelPreference(value: JsonValue \| undefined, at: string): ChannelPreference` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeClassToggle(value: JsonValue \| undefined, at: string): ClassToggle` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeConsensusInteger(value: JsonValue \| undefined, at: string): bigint` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeDepositConfirmRequest(value: JsonValue \| undefined, at: string): DepositConfirmRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeDepositStartRequest(value: JsonValue \| undefined, at: string): DepositStartRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeDevice(value: JsonValue \| undefined, at: string): Device` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeEmpty(value: JsonValue \| undefined, at: string): Empty` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeErrorCode(value: JsonValue \| undefined, at: string): ErrorCode` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeEvidenceClass(value: JsonValue \| undefined, at: string): EvidenceClass` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeEvidenceMaterial(value: JsonValue \| undefined, at: string): EvidenceMaterial` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeEvidenceRef(value: JsonValue \| undefined, at: string): EvidenceRef` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeExitEligibility(value: JsonValue \| undefined, at: string): ExitEligibility` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeExitStartRequest(value: JsonValue \| undefined, at: string): ExitStartRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeExportArtefact(value: JsonValue \| undefined, at: string): ExportArtefact` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeExportedPrimaryKey(value: JsonValue \| undefined, at: string): ExportedPrimaryKey` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeExportEvidenceRequest(value: JsonValue \| undefined, at: string): ExportEvidenceRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeExportKind(value: JsonValue \| undefined, at: string): ExportKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeExportStatementRequest(value: JsonValue \| undefined, at: string): ExportStatementRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeHomeSummary(value: JsonValue \| undefined, at: string): HomeSummary` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentConstraints(value: JsonValue \| undefined, at: string): IntentConstraints` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentDomain(value: JsonValue \| undefined, at: string): IntentDomain` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentEndpoint(value: JsonValue \| undefined, at: string): IntentEndpoint` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentEndpointKind(value: JsonValue \| undefined, at: string): IntentEndpointKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentLeg(value: JsonValue \| undefined, at: string): IntentLeg` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentLegBinding(value: JsonValue \| undefined, at: string): IntentLegBinding` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentPlan(value: JsonValue \| undefined, at: string): IntentPlan` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentSigningRequirement(value: JsonValue \| undefined, at: string): IntentSigningRequirement` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeIntentSubmission(value: JsonValue \| undefined, at: string): IntentSubmission` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeJourney(value: JsonValue \| undefined, at: string): Journey` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeJourneyKind(value: JsonValue \| undefined, at: string): JourneyKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeJourneyPage(value: JsonValue \| undefined, at: string): JourneyPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeJourneyStage(value: JsonValue \| undefined, at: string): JourneyStage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeJourneyState(value: JsonValue \| undefined, at: string): JourneyState` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeKeyChallenge(value: JsonValue \| undefined, at: string): KeyChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeKeyChallengeKind(value: JsonValue \| undefined, at: string): KeyChallengeKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeKeyExportBegin(value: JsonValue \| undefined, at: string): KeyExportBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeKeyExportChallenge(value: JsonValue \| undefined, at: string): KeyExportChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeKeyExportFinish(value: JsonValue \| undefined, at: string): KeyExportFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeLimitEnforcement(value: JsonValue \| undefined, at: string): LimitEnforcement` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeMoney(value: JsonValue \| undefined, at: string): Money` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeMoneyDirection(value: JsonValue \| undefined, at: string): MoneyDirection` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeMoveCommitRequest(value: JsonValue \| undefined, at: string): MoveCommitRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeMoveMechanism(value: JsonValue \| undefined, at: string): MoveMechanism` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeMoveQuote(value: JsonValue \| undefined, at: string): MoveQuote` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeMoveQuoteRequest(value: JsonValue \| undefined, at: string): MoveQuoteRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNativeFeeAsset(value: JsonValue \| undefined, at: string): NativeFeeAsset` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNativeFeeBudget(value: JsonValue \| undefined, at: string): NativeFeeBudget` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNativeSendAccessConfirmation(value: JsonValue \| undefined, at: string): NativeSendAccessConfirmation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNativeSendAccessDisclosure(value: JsonValue \| undefined, at: string): NativeSendAccessDisclosure` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNativeSendAccessRequest(value: JsonValue \| undefined, at: string): NativeSendAccessRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNativeSendAccessResult(value: JsonValue \| undefined, at: string): NativeSendAccessResult` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNotificationClass(value: JsonValue \| undefined, at: string): NotificationClass` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNotificationDetailLevel(value: JsonValue \| undefined, at: string): NotificationDetailLevel` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNotificationGroup(value: JsonValue \| undefined, at: string): NotificationGroup` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNotificationPage(value: JsonValue \| undefined, at: string): NotificationPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNotificationPreferences(value: JsonValue \| undefined, at: string): NotificationPreferences` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNotificationRecency(value: JsonValue \| undefined, at: string): NotificationRecency` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeNotificationSummary(value: JsonValue \| undefined, at: string): NotificationSummary` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeOwnerRotationDisclosureRequest(value: JsonValue \| undefined, at: string): OwnerRotationDisclosureRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeOwnerRotationRequest(value: JsonValue \| undefined, at: string): OwnerRotationRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskey(value: JsonValue \| undefined, at: string): Passkey` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskeyAssertion(value: JsonValue \| undefined, at: string): PasskeyAssertion` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskeyAssertionBegin(value: JsonValue \| undefined, at: string): PasskeyAssertionBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskeyAssertionChallenge(value: JsonValue \| undefined, at: string): PasskeyAssertionChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskeyAssertionFinish(value: JsonValue \| undefined, at: string): PasskeyAssertionFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskeyList(value: JsonValue \| undefined, at: string): PasskeyList` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskeyRegistrationBegin(value: JsonValue \| undefined, at: string): PasskeyRegistrationBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskeyRegistrationChallenge(value: JsonValue \| undefined, at: string): PasskeyRegistrationChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePasskeyRegistrationFinish(value: JsonValue \| undefined, at: string): PasskeyRegistrationFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodePlanIntentRequest(value: JsonValue \| undefined, at: string): PlanIntentRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProfile(value: JsonValue \| undefined, at: string): Profile` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProfileUpdate(value: JsonValue \| undefined, at: string): ProfileUpdate` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalBudget(value: JsonValue \| undefined, at: string): ProgramApprovalBudget` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalDecision(value: JsonValue \| undefined, at: string): ProgramApprovalDecision` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalDecisionKind(value: JsonValue \| undefined, at: string): ProgramApprovalDecisionKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalDecisionRequest(value: JsonValue \| undefined, at: string): ProgramApprovalDecisionRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalDetail(value: JsonValue \| undefined, at: string): ProgramApprovalDetail` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalDisclosure(value: JsonValue \| undefined, at: string): ProgramApprovalDisclosure` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalDisclosureRequest(value: JsonValue \| undefined, at: string): ProgramApprovalDisclosureRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalMaterial(value: JsonValue \| undefined, at: string): ProgramApprovalMaterial` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalOperation(value: JsonValue \| undefined, at: string): ProgramApprovalOperation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalPage(value: JsonValue \| undefined, at: string): ProgramApprovalPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalSemantics(value: JsonValue \| undefined, at: string): ProgramApprovalSemantics` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalState(value: JsonValue \| undefined, at: string): ProgramApprovalState` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramApprovalSummary(value: JsonValue \| undefined, at: string): ProgramApprovalSummary` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramAuthorizedLimit(value: JsonValue \| undefined, at: string): ProgramAuthorizedLimit` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramBytesCommitment(value: JsonValue \| undefined, at: string): ProgramBytesCommitment` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramCallFacts(value: JsonValue \| undefined, at: string): ProgramCallFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramChargeKind(value: JsonValue \| undefined, at: string): ProgramChargeKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramDeployFacts(value: JsonValue \| undefined, at: string): ProgramDeployFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramMaterialProvenance(value: JsonValue \| undefined, at: string): ProgramMaterialProvenance` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramOperationKind(value: JsonValue \| undefined, at: string): ProgramOperationKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramUpgradeFacts(value: JsonValue \| undefined, at: string): ProgramUpgradeFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramUpgradePolicyFacts(value: JsonValue \| undefined, at: string): ProgramUpgradePolicyFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramUpgradePolicyKind(value: JsonValue \| undefined, at: string): ProgramUpgradePolicyKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramWindDownFacts(value: JsonValue \| undefined, at: string): ProgramWindDownFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProgramWindDownKind(value: JsonValue \| undefined, at: string): ProgramWindDownKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeProtocolFreshness(value: JsonValue \| undefined, at: string): ProtocolFreshness` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeRebindingSubmission(value: JsonValue \| undefined, at: string): RebindingSubmission` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeRefusal(value: JsonValue \| undefined, at: string): Refusal` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeRefusedBy(value: JsonValue \| undefined, at: string): RefusedBy` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeResponseEnvelope(value: JsonValue \| undefined, at: string): ResponseEnvelope` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeRetriability(value: JsonValue \| undefined, at: string): Retriability` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSchemaVersion(value: JsonValue \| undefined, at: string): SchemaVersion` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSecurityAction(value: JsonValue \| undefined, at: string): SecurityAction` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSecurityActionKind(value: JsonValue \| undefined, at: string): SecurityActionKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSecurityActionRequest(value: JsonValue \| undefined, at: string): SecurityActionRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSecurityPasskeyRegistrationBegin(value: JsonValue \| undefined, at: string): SecurityPasskeyRegistrationBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSecurityPasskeyRegistrationFinish(value: JsonValue \| undefined, at: string): SecurityPasskeyRegistrationFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSecurityPasskeyRevocation(value: JsonValue \| undefined, at: string): SecurityPasskeyRevocation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSecurityRecoveryReveal(value: JsonValue \| undefined, at: string): SecurityRecoveryReveal` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSecuritySessionRevocation(value: JsonValue \| undefined, at: string): SecuritySessionRevocation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSession(value: JsonValue \| undefined, at: string): Session` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSessionDevice(value: JsonValue \| undefined, at: string): SessionDevice` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSessionList(value: JsonValue \| undefined, at: string): SessionList` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSessionOpenRequest(value: JsonValue \| undefined, at: string): SessionOpenRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSessionRevocation(value: JsonValue \| undefined, at: string): SessionRevocation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSettlementDomain(value: JsonValue \| undefined, at: string): SettlementDomain` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSpendLimit(value: JsonValue \| undefined, at: string): SpendLimit` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeStepUpChallenge(value: JsonValue \| undefined, at: string): StepUpChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeStepUpEvidence(value: JsonValue \| undefined, at: string): StepUpEvidence` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeStepUpFinish(value: JsonValue \| undefined, at: string): StepUpFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeStepUpRequest(value: JsonValue \| undefined, at: string): StepUpRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeStreamEvent(value: JsonValue \| undefined, at: string): StreamEvent` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeStreamEventKind(value: JsonValue \| undefined, at: string): StreamEventKind` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeStreamPage(value: JsonValue \| undefined, at: string): StreamPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeStreamPosition(value: JsonValue \| undefined, at: string): StreamPosition` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSubmitPlanRequest(value: JsonValue \| undefined, at: string): SubmitPlanRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportAuthor(value: JsonValue \| undefined, at: string): SupportAuthor` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportConversation(value: JsonValue \| undefined, at: string): SupportConversation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportConversationPage(value: JsonValue \| undefined, at: string): SupportConversationPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportConversationState(value: JsonValue \| undefined, at: string): SupportConversationState` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportConversationStatus(value: JsonValue \| undefined, at: string): SupportConversationStatus` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportCreateRequest(value: JsonValue \| undefined, at: string): SupportCreateRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportFeedback(value: JsonValue \| undefined, at: string): SupportFeedback` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportFeedbackRequest(value: JsonValue \| undefined, at: string): SupportFeedbackRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportMessage(value: JsonValue \| undefined, at: string): SupportMessage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportReadRequest(value: JsonValue \| undefined, at: string): SupportReadRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportReplyRequest(value: JsonValue \| undefined, at: string): SupportReplyRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportShell(value: JsonValue \| undefined, at: string): SupportShell` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeSupportTopic(value: JsonValue \| undefined, at: string): SupportTopic` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeTimedSecret(value: JsonValue \| undefined, at: string): TimedSecret` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeVerificationLevel(value: JsonValue \| undefined, at: string): VerificationLevel` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeVerifiedMoney(value: JsonValue \| undefined, at: string): VerifiedMoney` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeVersionInfo(value: JsonValue \| undefined, at: string): VersionInfo` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeWalletBinding(value: JsonValue \| undefined, at: string): WalletBinding` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeWalletSignRequest(value: JsonValue \| undefined, at: string): WalletSignRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeWithdrawClaimRequest(value: JsonValue \| undefined, at: string): WithdrawClaimRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function decodeWithdrawStartRequest(value: JsonValue \| undefined, at: string): WithdrawStartRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface DepositConfirmRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface DepositStartRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface Device` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type DeviceId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type EmailAddress = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type Empty = Record<string, never>;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAccountBalance(value: AccountBalance): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAccountCreateRequest(value: AccountCreateRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAccountCreation(value: AccountCreation): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeActivityEntry(value: ActivityEntry): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeActivityEntryDetail(value: ActivityEntryDetail): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeActivityFilter(value: ActivityFilter): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeActivityGroup(value: ActivityGroup): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeActivityPage(value: ActivityPage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeActivityQueryRequest(value: ActivityQueryRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAgent(value: Agent): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAgentArchiveRequest(value: AgentArchiveRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAgentCreateRequest(value: AgentCreateRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAgentLimitRequest(value: AgentLimitRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAgentPage(value: AgentPage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAgentReclaimRequest(value: AgentReclaimRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAgentSpend(value: AgentSpend): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeApiError(value: ApiError): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeApprovalApproveRequest(value: ApprovalApproveRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeApprovalDecision(value: ApprovalDecision): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeApprovalDetail(value: ApprovalDetail): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeApprovalFacts(value: ApprovalFacts): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeApprovalPage(value: ApprovalPage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeApprovalSummary(value: ApprovalSummary): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAuthenticatorDisable(value: AuthenticatorDisable): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAuthenticatorMethod(value: AuthenticatorMethod): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAuthenticatorSetupBegin(value: AuthenticatorSetupBegin): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAuthenticatorSetupChallenge(value: AuthenticatorSetupChallenge): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAuthenticatorSetupFinish(value: AuthenticatorSetupFinish): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAuthenticatorSetupResult(value: AuthenticatorSetupResult): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeAuthenticatorStatus(value: AuthenticatorStatus): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeBackupCodeRotation(value: BackupCodeRotation): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeBackupCodeSet(value: BackupCodeSet): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeBindingRebindAction(value: BindingRebindAction): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeBindingStatement(value: BindingStatement): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeBindingStatementRequest(value: BindingStatementRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeBindingSubmission(value: BindingSubmission): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeChannelPreference(value: ChannelPreference): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeClassToggle(value: ClassToggle): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeConsensusInteger(value: bigint): string` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeDepositConfirmRequest(value: DepositConfirmRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeDepositStartRequest(value: DepositStartRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeDevice(value: Device): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeEmpty(value: Empty): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeEvidenceMaterial(value: EvidenceMaterial): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeEvidenceRef(value: EvidenceRef): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeExitEligibility(value: ExitEligibility): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeExitStartRequest(value: ExitStartRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeExportArtefact(value: ExportArtefact): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeExportedPrimaryKey(value: ExportedPrimaryKey): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeExportEvidenceRequest(value: ExportEvidenceRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeExportStatementRequest(value: ExportStatementRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeHomeSummary(value: HomeSummary): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeIntentConstraints(value: IntentConstraints): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeIntentEndpoint(value: IntentEndpoint): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeIntentLeg(value: IntentLeg): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeIntentLegBinding(value: IntentLegBinding): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeIntentPlan(value: IntentPlan): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeIntentSigningRequirement(value: IntentSigningRequirement): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeIntentSubmission(value: IntentSubmission): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeJourney(value: Journey): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeJourneyPage(value: JourneyPage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeJourneyStage(value: JourneyStage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeKeyChallenge(value: KeyChallenge): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeKeyExportBegin(value: KeyExportBegin): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeKeyExportChallenge(value: KeyExportChallenge): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeKeyExportFinish(value: KeyExportFinish): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeMoney(value: Money): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeMoveCommitRequest(value: MoveCommitRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeMoveQuote(value: MoveQuote): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeMoveQuoteRequest(value: MoveQuoteRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNativeFeeAsset(value: NativeFeeAsset): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNativeFeeBudget(value: NativeFeeBudget): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNativeSendAccessConfirmation(value: NativeSendAccessConfirmation): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNativeSendAccessDisclosure(value: NativeSendAccessDisclosure): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNativeSendAccessRequest(value: NativeSendAccessRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNativeSendAccessResult(value: NativeSendAccessResult): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNotificationGroup(value: NotificationGroup): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNotificationPage(value: NotificationPage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNotificationPreferences(value: NotificationPreferences): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeNotificationSummary(value: NotificationSummary): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeOwnerRotationDisclosureRequest(value: OwnerRotationDisclosureRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeOwnerRotationRequest(value: OwnerRotationRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskey(value: Passkey): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskeyAssertion(value: PasskeyAssertion): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskeyAssertionBegin(value: PasskeyAssertionBegin): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskeyAssertionChallenge(value: PasskeyAssertionChallenge): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskeyAssertionFinish(value: PasskeyAssertionFinish): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskeyList(value: PasskeyList): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskeyRegistrationBegin(value: PasskeyRegistrationBegin): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskeyRegistrationChallenge(value: PasskeyRegistrationChallenge): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePasskeyRegistrationFinish(value: PasskeyRegistrationFinish): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodePlanIntentRequest(value: PlanIntentRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProfile(value: Profile): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProfileUpdate(value: ProfileUpdate): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalBudget(value: ProgramApprovalBudget): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalDecision(value: ProgramApprovalDecision): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalDecisionRequest(value: ProgramApprovalDecisionRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalDetail(value: ProgramApprovalDetail): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalDisclosure(value: ProgramApprovalDisclosure): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalDisclosureRequest(value: ProgramApprovalDisclosureRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalMaterial(value: ProgramApprovalMaterial): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalOperation(value: ProgramApprovalOperation): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalPage(value: ProgramApprovalPage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramApprovalSummary(value: ProgramApprovalSummary): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramAuthorizedLimit(value: ProgramAuthorizedLimit): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramBytesCommitment(value: ProgramBytesCommitment): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramCallFacts(value: ProgramCallFacts): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramDeployFacts(value: ProgramDeployFacts): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramUpgradeFacts(value: ProgramUpgradeFacts): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramUpgradePolicyFacts(value: ProgramUpgradePolicyFacts): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProgramWindDownFacts(value: ProgramWindDownFacts): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeProtocolFreshness(value: ProtocolFreshness): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeRebindingSubmission(value: RebindingSubmission): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeRefusal(value: Refusal): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeResponseEnvelope(value: ResponseEnvelope): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSchemaVersion(value: SchemaVersion): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSecurityAction(value: SecurityAction): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSecurityActionRequest(value: SecurityActionRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSecurityPasskeyRegistrationBegin(value: SecurityPasskeyRegistrationBegin): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSecurityPasskeyRegistrationFinish(value: SecurityPasskeyRegistrationFinish): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSecurityPasskeyRevocation(value: SecurityPasskeyRevocation): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSecurityRecoveryReveal(value: SecurityRecoveryReveal): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSecuritySessionRevocation(value: SecuritySessionRevocation): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSession(value: Session): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSessionDevice(value: SessionDevice): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSessionList(value: SessionList): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSessionOpenRequest(value: SessionOpenRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSessionRevocation(value: SessionRevocation): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSpendLimit(value: SpendLimit): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeStepUpChallenge(value: StepUpChallenge): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeStepUpEvidence(value: StepUpEvidence): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeStepUpFinish(value: StepUpFinish): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeStepUpRequest(value: StepUpRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeStreamEvent(value: StreamEvent): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeStreamPage(value: StreamPage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeStreamPosition(value: StreamPosition): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSubmitPlanRequest(value: SubmitPlanRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportConversation(value: SupportConversation): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportConversationPage(value: SupportConversationPage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportConversationStatus(value: SupportConversationStatus): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportCreateRequest(value: SupportCreateRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportFeedback(value: SupportFeedback): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportFeedbackRequest(value: SupportFeedbackRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportMessage(value: SupportMessage): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportReadRequest(value: SupportReadRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeSupportReplyRequest(value: SupportReplyRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeTimedSecret(value: TimedSecret): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeVerifiedMoney(value: VerifiedMoney): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeVersionInfo(value: VersionInfo): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeWalletBinding(value: WalletBinding): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeWalletSignRequest(value: WalletSignRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeWithdrawClaimRequest(value: WithdrawClaimRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function encodeWithdrawStartRequest(value: WithdrawStartRequest): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `ErrorCode` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `errorCodeVariants` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type EvidenceClass = "local-journey-state" \| "submission-record" \| "layerx-receipt" \| "checkpoint-proof" \| "paxeer-finality" \| "typed-refusal" \| "approval-hold" \| "wallet-ack";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const evidenceClassVariants: readonly EvidenceClass[] = ["local-journey-state", "submission-record", "layerx-receipt", "checkpoint-proof", "paxeer-finality", "typed-refusal", "approval-hold", "wallet-ack"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type EvidenceId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface EvidenceMaterial` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface EvidenceRef` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type EvmAddress = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type EvmSignature = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ExitEligibility` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ExitStartRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function expectArray(value: JsonValue \| undefined, at: string): JsonValue[]` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function expectBoolean(value: JsonValue \| undefined, at: string): boolean` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function expectInteger(value: JsonValue \| undefined, at: string): number` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function expectObject(value: JsonValue \| undefined, at: string): JsonObject` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function expectString(value: JsonValue \| undefined, at: string): string` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ExportArtefact` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ExportedPrimaryKey` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ExportEvidenceRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ExportId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ExportKind = "statement" \| "evidence-bundle";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const exportKindVariants: readonly ExportKind[] = ["statement", "evidence-bundle"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ExportStatementRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type FetchLike = (input: string, init: RequestInit) => Promise<Response>;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface HomeSummary` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type HttpMethod = "DELETE" \| "GET" \| "PATCH" \| "POST" \| "PUT";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface HumanApiClient` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface HumanApiClientOptions` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export class HumanApiDecodeError extends Error` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export class HumanApiError extends Error` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface IntentConstraints` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type IntentDomain = "paxeer" \| "layerx";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const intentDomainVariants: readonly IntentDomain[] = ["paxeer", "layerx"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface IntentEndpoint` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type IntentEndpointKind = "paxeer-wallet" \| "human" \| "agent" \| "agent-budget";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const intentEndpointKindVariants: readonly IntentEndpointKind[] = ["paxeer-wallet", "human", "agent", "agent-budget"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface IntentLeg` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface IntentLegBinding` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface IntentPlan` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface IntentSigningRequirement` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface IntentSubmission` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface Journey` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type JourneyId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type JourneyKind = "onboarding" \| "wallet-binding" \| "deposit" \| "withdraw" \| "exit" \| "move" \| "agent-create" \| "agent-fund" \| "agent-pause" \| "agent-retire";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const journeyKindVariants: readonly JourneyKind[] = ["onboarding", "wallet-binding", "deposit", "withdraw", "exit", "move", "agent-create", "agent-fund", "agent-pause", "agent-retire"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface JourneyPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface JourneyStage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type JourneyState = "getting-ready" \| "sending" \| "processing" \| "done" \| "done-finalised" \| "still-checking" \| "refused" \| "waiting-for-you" \| "closed" \| "awaiting-approval";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const journeyStateVariants: readonly JourneyState[] = ["getting-ready", "sending", "processing", "done", "done-finalised", "still-checking", "refused", "waiting-for-you", "closed", "awaiting-approval"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface JsonObject` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type JsonValue = null \| boolean \| number \| string \| JsonValue[] \| JsonObject;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface KeyChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type KeyChallengeKind = "rotate" \| "recover";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const keyChallengeKindVariants: readonly KeyChallengeKind[] = ["rotate", "recover"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface KeyExportBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface KeyExportChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface KeyExportFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type LimitEnforcement = "protocol" \| "app";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const limitEnforcementVariants: readonly LimitEnforcement[] = ["protocol", "app"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type Milliseconds = bigint;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface Money` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type MoneyDirection = "in" \| "out";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const moneyDirectionVariants: readonly MoneyDirection[] = ["in", "out"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface MoveCommitRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type MoveMechanism = "fund" \| "allocate" \| "return" \| "transfer";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const moveMechanismVariants: readonly MoveMechanism[] = ["fund", "allocate", "return", "transfer"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface MoveQuote` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface MoveQuoteRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NativeFeeAsset` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NativeFeeBudget` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NativeSendAccessConfirmation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NativeSendAccessDisclosure` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NativeSendAccessRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NativeSendAccessResult` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type NativeSendCount = bigint;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type NativeSendUnixSeconds = bigint;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type NotificationClass = "approval-waiting" \| "money-arrived" \| "journey-finished" \| "claim-ready" \| "security-new-device" \| "security-recovery" \| "security-wallet-rebinding" \| "security-key-rotation" \| "service-status";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const notificationClassVariants: readonly NotificationClass[] = ["approval-waiting", "money-arrived", "journey-finished", "claim-ready", "security-new-device", "security-recovery", "security-wallet-rebinding", "security-key-rotation", "service-status"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type NotificationDetailLevel = "full" \| "summary" \| "minimal";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const notificationDetailLevelVariants: readonly NotificationDetailLevel[] = ["full", "summary", "minimal"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NotificationGroup` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type NotificationId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NotificationPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NotificationPreferences` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type NotificationRecency = "today" \| "yesterday" \| "this-week" \| "earlier";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const notificationRecencyVariants: readonly NotificationRecency[] = ["today", "yesterday", "this-week", "earlier"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface NotificationSummary` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type OpaqueCredential = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type OperationDigest = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type OperationName = (typeof operationNames)[number];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const operationNames = [` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const operations: { readonly [name in OperationName]: OperationShape } =` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface OperationShape` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface OwnerRotationDisclosureRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface OwnerRotationRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface Passkey` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PasskeyAssertion` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PasskeyAssertionBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PasskeyAssertionChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PasskeyAssertionFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type PasskeyId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PasskeyList` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PasskeyRegistrationBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PasskeyRegistrationChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PasskeyRegistrationFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface PlanIntentRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface Profile` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProfileUpdate` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalBudget` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalDecision` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ProgramApprovalDecisionKind = "approve" \| "reject";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const programApprovalDecisionKindVariants: readonly ProgramApprovalDecisionKind[] = ["approve", "reject"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalDecisionRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalDetail` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalDisclosure` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalDisclosureRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalMaterial` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalOperation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ProgramApprovalSemantics = "operation-only" \| "authorized-limits";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const programApprovalSemanticsVariants: readonly ProgramApprovalSemantics[] = ["operation-only", "authorized-limits"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ProgramApprovalState = "pending" \| "approved" \| "rejected" \| "expired" \| "defective" \| "not-required";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const programApprovalStateVariants: readonly ProgramApprovalState[] = ["pending", "approved", "rejected", "expired", "defective", "not-required"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramApprovalSummary` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramAuthorizedLimit` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramBytesCommitment` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramCallFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ProgramChargeKind = "principal" \| "program-spend";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const programChargeKindVariants: readonly ProgramChargeKind[] = ["principal", "program-spend"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramDeployFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ProgramMaterialProvenance = "local-owned";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const programMaterialProvenanceVariants: readonly ProgramMaterialProvenance[] = ["local-owned"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ProgramOperationKind = "deploy" \| "upgrade" \| "call" \| "wind-down";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const programOperationKindVariants: readonly ProgramOperationKind[] = ["deploy", "upgrade", "call", "wind-down"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramUpgradeFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramUpgradePolicyFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ProgramUpgradePolicyKind = "immutable" \| "authority";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const programUpgradePolicyKindVariants: readonly ProgramUpgradePolicyKind[] = ["immutable", "authority"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProgramWindDownFacts` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type ProgramWindDownKind = "route" \| "deprecate" \| "tombstone" \| "exit" \| "bounded-exit";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const programWindDownKindVariants: readonly ProgramWindDownKind[] = ["route", "deprecate", "tombstone", "exit", "bounded-exit"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ProtocolFreshness` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type QuoteId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface RebindingSubmission` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface Refusal` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type RefusedBy = "policy" \| "budget" \| "capability" \| "protocol" \| "limit";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const refusedByVariants: readonly RefusedBy[] = ["policy", "budget", "capability", "protocol", "limit"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type RegistrationId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function requireValue(value: JsonValue \| undefined, at: string): JsonValue` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface ResponseEnvelope` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type Retriability = "retriable" \| "retriable-after" \| "structural" \| "final";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const retriabilityVariants: readonly Retriability[] = ["retriable", "retriable-after", "structural", "final"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `schemaVersion` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SchemaVersion` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SecurityAction` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SecurityActionKind = "add-passkey" \| "revoke-passkey" \| "revoke-session" \| "revoke-all-sessions" \| "add-authenticator" \| "disable-authenticator" \| "rotate-backup-codes" \| "reveal-recovery-evidence" \| "export-primary-key";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const securityActionKindVariants: readonly SecurityActionKind[] = ["add-passkey", "revoke-passkey", "revoke-session", "revoke-all-sessions", "add-authenticator", "disable-authenticator", "rotate-backup-codes", "reveal-recovery-evidence", "export-primary-key"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SecurityActionRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SecurityPasskeyRegistrationBegin` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SecurityPasskeyRegistrationFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SecurityPasskeyRevocation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SecurityRecoveryReveal` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SecuritySessionRevocation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface Session` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SessionDevice` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SessionId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SessionList` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SessionOpenRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SessionRevocation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SettlementDomain = "paxeer";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const settlementDomainVariants: readonly SettlementDomain[] = ["paxeer"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SpendLimit` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type StageId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface StepUpChallenge` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type StepUpChallengeId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface StepUpEvidence` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface StepUpFinish` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface StepUpRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface StreamEvent` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type StreamEventKind = "journey-progress" \| "approval-created" \| "approval-approved" \| "approval-rejected" \| "approval-expired" \| "notification" \| "program-approval-created" \| "program-approval-approved" \| "program-approval-rejected" \| "program-approval-expired";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const streamEventKindVariants: readonly StreamEventKind[] = ["journey-progress", "approval-created", "approval-approved", "approval-rejected", "approval-expired", "notification", "program-approval-created", "program-approval-approved", "program-approval-rejected", "program-approval-expired"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface StreamPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface StreamPosition` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const streamPushProfile =` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface StreamSubscribeOptions` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SubmitPlanRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SupportAuthor = "you" \| "support";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const supportAuthorVariants: readonly SupportAuthor[] = ["you", "support"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportConversation` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SupportConversationId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportConversationPage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SupportConversationState = "waiting-for-support" \| "waiting-for-you" \| "resolved";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const supportConversationStateVariants: readonly SupportConversationState[] = ["waiting-for-support", "waiting-for-you", "resolved"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportConversationStatus` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportCreateRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportFeedback` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportFeedbackRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportMessage` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SupportMessageId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportReadRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface SupportReplyRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SupportShell = "mobile" \| "desktop";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const supportShellVariants: readonly SupportShell[] = ["mobile", "desktop"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type SupportTopic = "deposit" \| "withdrawal" \| "agents" \| "account" \| "report";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const supportTopicVariants: readonly SupportTopic[] = ["deposit", "withdrawal", "agents", "account", "report"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface TimedSecret` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type Timestamp = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type TraceId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type VerificationLevel = "unverified" \| "receipt-verified" \| "checkpoint-finalised" \| "paxeer-finalised" \| "settlement-anchored";` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export const verificationLevelVariants: readonly VerificationLevel[] = ["unverified", "receipt-verified", "checkpoint-finalised", "paxeer-finalised", "settlement-anchored"];` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface VerifiedMoney` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface VersionInfo` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type WalletAddress = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface WalletBinding` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type WalletSignature = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface WalletSignRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export type WalletTxId = string;` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface WithdrawClaimRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export interface WithdrawStartRequest` | [apps/web/src/api/generated/index.ts](../../../apps/web/src/api/generated/index.ts) |
| `ts_export` | `export function humanApi(options: HumanApiClientOptions = {}): HumanApiClient` | [apps/web/src/api/index.ts](../../../apps/web/src/api/index.ts) |
| `ts_export` | `export function submitWebVital( route: PerformanceRoute, metric: WebVitalName, observed: number, ): void` | [apps/web/src/api/performance-client.ts](../../../apps/web/src/api/performance-client.ts) |
| `ts_export` | `export interface PerformanceSinkHealth` | [apps/web/src/api/performance-sink.ts](../../../apps/web/src/api/performance-sink.ts) |
| `ts_export` | `export interface PerformanceSinkResult` | [apps/web/src/api/performance-sink.ts](../../../apps/web/src/api/performance-sink.ts) |
| `ts_export` | `export interface PerformanceSinkSnapshot` | [apps/web/src/api/performance-sink.ts](../../../apps/web/src/api/performance-sink.ts) |
| `ts_export` | `export async function performanceSinkSnapshot(): Promise<PerformanceSinkSnapshot>` | [apps/web/src/api/performance-sink.ts](../../../apps/web/src/api/performance-sink.ts) |
| `ts_export` | `export async function persistPerformanceObservation( observation: RedactedWebVital, ): Promise<PerformanceSinkResult>` | [apps/web/src/api/performance-sink.ts](../../../apps/web/src/api/performance-sink.ts) |
| `ts_export` | `export async function bridgeActivity(recipient: string): Promise<readonly ObservedEvent[]>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface BridgeAttestors` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function bridgeAttestors(): Promise<BridgeAttestors>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface BridgeCap` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function bridgeCap(chain: bigint, asset: string): Promise<BridgeCap>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface BridgeChain` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function bridgeChain(chain: bigint): Promise<BridgeChain>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function bridgeNullified(chain: bigint, txHash: string, logIndex: bigint): Promise<boolean>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function bridgePaused(): Promise<boolean>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface LaunchpadConfig` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function launchpadConfig(): Promise<LaunchpadConfig>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface LaunchpadMarket` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function launchpadMarketCount(): Promise<bigint>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function launchpadMarkets(offset: bigint, limit: bigint): Promise<readonly LaunchpadMarketRow[]> {` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface LaunchpadQuote` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function launchpadQuote(token: string, side: "buy" \| "sell", amountIn: bigint): Promise<LaunchpadQuote>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface ObservedEvent` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export const PERPS_COLLATERAL_DECIMALS = 6;` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export const PERPS_COLLATERAL_TOKEN = "0x85fcd13735f4309833a503ee804ea32395851479";` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export const PERPS_ERRORS = [` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export const PERPS_MAX_MARKETS = 1024n;` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export const PERPS_USD_DECIMALS = 18;` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface PerpsAccount` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function perpsAccount(owner: string): Promise<PerpsAccount>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface PerpsMarket` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function perpsMarkets(): Promise<readonly PerpsMarket[]>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface PerpsOrder` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface PerpsPosition` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export async function perpsPreflight(from: string, data: string): Promise<string \| undefined>` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export function perpsRevertReason(data: string \| undefined): string \| undefined` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export const PRECOMPILE_LOG_WINDOW = 50_000n;` | [apps/web/src/api/precompiles.ts](../../../apps/web/src/api/precompiles.ts) |
| `ts_export` | `export interface Eip1193Provider` | [apps/web/src/api/provider.ts](../../../apps/web/src/api/provider.ts) |
| `ts_export` | `export function windowWalletProvider(): Eip1193Provider \| undefined` | [apps/web/src/api/provider.ts](../../../apps/web/src/api/provider.ts) |
| `ts_export` | `export function humanApiForServer(context: ServerHumanApiContext): HumanApiClient` | [apps/web/src/api/server.ts](../../../apps/web/src/api/server.ts) |
| `ts_export` | `export interface ServerHumanApiContext` | [apps/web/src/api/server.ts](../../../apps/web/src/api/server.ts) |
| `ts_export` | `export interface HumanStreamObserver` | [apps/web/src/api/stream.ts](../../../apps/web/src/api/stream.ts) |
| `ts_export` | `export async function observeHumanStream( client: HumanApiClient, observer: HumanStreamObserver, ): Promise<void>` | [apps/web/src/api/stream.ts](../../../apps/web/src/api/stream.ts) |
| `ts_export` | `export function chooseWalletFee(` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export async function connectWalletAccount(): Promise<string \| undefined>` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export type PrecompileSendOutcome =` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export async function resumeWalletSponsoredSubmission(` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export async function sendWalletPrecompileCall(` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export async function sendWalletSponsoredBatch(` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export function sidioraAmount(amount: bigint): string` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export function sidioraQuotePresentation(` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export function sponsoredWalletModule(` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `WalletFeePreference` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export function walletFeePreference(): WalletFeePreference` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |
| `ts_export` | `export function walletSendFailure(error: unknown): PrecompileSendOutcome` | [apps/web/src/api/wallet.ts](../../../apps/web/src/api/wallet.ts) |

## app

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export default function AgentDetailRouteError({ error, reset, }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>)` | [apps/web/src/app/app/agents/[agentId]/error.tsx](../../../apps/web/src/app/app/agents/[agentId]/error.tsx) |
| `ts_export` | `export default function AgentDetailRouteLoading()` | [apps/web/src/app/app/agents/[agentId]/loading.tsx](../../../apps/web/src/app/app/agents/[agentId]/loading.tsx) |
| `ts_export` | `export default function AgentsRouteError({ error, reset, }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>)` | [apps/web/src/app/app/agents/error.tsx](../../../apps/web/src/app/app/agents/error.tsx) |
| `ts_export` | `export default function AgentsRouteLoading()` | [apps/web/src/app/app/agents/loading.tsx](../../../apps/web/src/app/app/agents/loading.tsx) |
| `ts_export` | `export default function AgentCreateRouteError({ error, reset, }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>)` | [apps/web/src/app/app/agents/new/error.tsx](../../../apps/web/src/app/app/agents/new/error.tsx) |
| `ts_export` | `export default function AgentCreateRouteLoading()` | [apps/web/src/app/app/agents/new/loading.tsx](../../../apps/web/src/app/app/agents/new/loading.tsx) |
| `ts_export` | `export default function AppRouteError({ error, reset, }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>)` | [apps/web/src/app/app/error.tsx](../../../apps/web/src/app/app/error.tsx) |
| `ts_export` | `export default async function AppPlaneLayout({ children }: Readonly<{ children: ReactNode }>)` | [apps/web/src/app/app/layout.tsx](../../../apps/web/src/app/app/layout.tsx) |
| `ts_export` | `export default function AppLoading()` | [apps/web/src/app/app/loading.tsx](../../../apps/web/src/app/app/loading.tsx) |
| `ts_export` | `export default function SupportRouteError({ error, reset, }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>)` | [apps/web/src/app/app/support/error.tsx](../../../apps/web/src/app/app/support/error.tsx) |
| `ts_export` | `export default function SupportRouteLoading()` | [apps/web/src/app/app/support/loading.tsx](../../../apps/web/src/app/app/support/loading.tsx) |
| `ts_export` | `export function BridgeOutForm({ account, chain, asset, closedReason, maxPerTx, headroom, }: Readonly<{ account: string \| undefined; chain: string; asset: string \| undefined; closedReason: string \| undefined;` | [apps/web/src/app/bridge/bridge-client.tsx](../../../apps/web/src/app/bridge/bridge-client.tsx) |
| `ts_export` | `export function BridgeStatusPolling({ active }: Readonly<{ active: boolean }>)` | [apps/web/src/app/bridge/bridge-client.tsx](../../../apps/web/src/app/bridge/bridge-client.tsx) |
| `ts_export` | `export function BridgeTrackForm({ account, chain, asset, }: Readonly<{ account: string \| undefined; chain: string; asset: string \| undefined }>)` | [apps/web/src/app/bridge/bridge-client.tsx](../../../apps/web/src/app/bridge/bridge-client.tsx) |
| `ts_export` | `export default function RootRouteError({ error, reset, }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>)` | [apps/web/src/app/error.tsx](../../../apps/web/src/app/error.tsx) |
| `ts_export` | `export type PerpsPreflightAnswer = Readonly<{ ok: true }> \| Readonly<{ ok: false; detail: string }>;` | [apps/web/src/app/exchange/actions.ts](../../../apps/web/src/app/exchange/actions.ts) |
| `ts_export` | `export async function preflightPerpsWrite(from: string, data: string): Promise<PerpsPreflightAnswer>` | [apps/web/src/app/exchange/actions.ts](../../../apps/web/src/app/exchange/actions.ts) |
| `ts_export` | `export function ExchangeActions(props: Parameters<typeof AccountExchangeActions>[0]) {` | [apps/web/src/app/exchange/exchange-client.tsx](../../../apps/web/src/app/exchange/exchange-client.tsx) |
| `ts_export` | `export type PerpsCollateral = Readonly<{ token: string; decimals: number; symbol: string }>;` | [apps/web/src/app/exchange/exchange-client.tsx](../../../apps/web/src/app/exchange/exchange-client.tsx) |
| `ts_export` | `export async function sendPerpsWrite(` | [apps/web/src/app/exchange/exchange-client.tsx](../../../apps/web/src/app/exchange/exchange-client.tsx) |
| `ts_export` | `export default function ExplorerRouteError({ error, reset, }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>)` | [apps/web/src/app/explorer/error.tsx](../../../apps/web/src/app/explorer/error.tsx) |
| `ts_export` | `export default function ExplorerPlaneLayout({ children }: Readonly<{ children: ReactNode }>)` | [apps/web/src/app/explorer/layout.tsx](../../../apps/web/src/app/explorer/layout.tsx) |
| `ts_export` | `export default function ExplorerLoading()` | [apps/web/src/app/explorer/loading.tsx](../../../apps/web/src/app/explorer/loading.tsx) |
| `ts_export` | `export default function GlobalError({ error, reset, }: Readonly<{ error: Error & { digest?: string }; reset: () => void }>)` | [apps/web/src/app/global-error.tsx](../../../apps/web/src/app/global-error.tsx) |
| `ts_export` | `export type LaunchpadQuoteAnswer =` | [apps/web/src/app/launchpad/actions.ts](../../../apps/web/src/app/launchpad/actions.ts) |
| `ts_export` | `export async function quoteLaunchpadSwap(token: string, side: string, amountIn: string): Promise<LaunchpadQuoteAnswer>` | [apps/web/src/app/launchpad/actions.ts](../../../apps/web/src/app/launchpad/actions.ts) |
| `ts_export` | `export function LaunchpadActions({` | [apps/web/src/app/launchpad/launchpad-client.tsx](../../../apps/web/src/app/launchpad/launchpad-client.tsx) |
| `ts_export` | `export async function sendWithApproval( from: string, spend: LaunchpadSpend, build: () => PrecompileCall, onApproving: (active: boolean) => void, ): Promise<PrecompileSendOutcome>` | [apps/web/src/app/launchpad/launchpad-client.tsx](../../../apps/web/src/app/launchpad/launchpad-client.tsx) |
| `ts_export` | `export const SPONSORED_APPROVE_UNSUPPORTED = "the allowance approve cannot be sponsored; choose the PAX gas fee for it";` | [apps/web/src/app/launchpad/launchpad-client.tsx](../../../apps/web/src/app/launchpad/launchpad-client.tsx) |
| `ts_export` | `export default function RootLayout({ children }: Readonly<{ children: ReactNode }>)` | [apps/web/src/app/layout.tsx](../../../apps/web/src/app/layout.tsx) |
| `ts_export` | `export const metadata: Metadata =` | [apps/web/src/app/layout.tsx](../../../apps/web/src/app/layout.tsx) |
| `ts_export` | `export const viewport: Viewport =` | [apps/web/src/app/layout.tsx](../../../apps/web/src/app/layout.tsx) |
| `ts_export` | `export default function RootRouteLoading()` | [apps/web/src/app/loading.tsx](../../../apps/web/src/app/loading.tsx) |
| `ts_export` | `export async function ActivityFeed({ route, account, side, cursor, kind, }: Readonly<{ route: string; account: string \| undefined; side: ActivitySide; cursor: string \| undefined; kind: string \| undefined;` | [apps/web/src/app/_markets/activity-feed.tsx](../../../apps/web/src/app/_markets/activity-feed.tsx) |
| `ts_export` | `ActivitySide` | [apps/web/src/app/_markets/activity-feed.tsx](../../../apps/web/src/app/_markets/activity-feed.tsx) |
| `ts_export` | `export function availability(...reasons: readonly (string \| false \| undefined)[]): ControlAvailability` | [apps/web/src/app/_markets/availability.ts](../../../apps/web/src/app/_markets/availability.ts) |
| `ts_export` | `export async function readSurfaceCapability(surface: string): Promise<SurfaceCapability>` | [apps/web/src/app/_markets/capability-action.ts](../../../apps/web/src/app/_markets/capability-action.ts) |
| `ts_export` | `export function SurfaceCapabilityGate({ surface, live, detail, }: Readonly<{ surface: ForkSurface; live: boolean; detail: string \| null }>)` | [apps/web/src/app/_markets/capability.tsx](../../../apps/web/src/app/_markets/capability.tsx) |
| `ts_export` | `export function firstParam(value: string \| string[] \| undefined): string \| undefined` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export function formatUnits(amount: bigint, decimals: number): string` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export function isBytes32(value: string \| undefined): value is string` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export function isEvmAddress(value: string \| undefined): value is string` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export const LAUNCHPAD_DECIMALS = 6;` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export type MarketSearchParams = Readonly<Record<string, string \| string[] \| undefined>>;` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export function parseUnits(text: string, decimals: number): bigint \| undefined` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export function shortId(value: string): string` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export const WEI_DECIMALS = 18;` | [apps/web/src/app/_markets/format.ts](../../../apps/web/src/app/_markets/format.ts) |
| `ts_export` | `export function MarketFrame({ title, description, account, children, }: Readonly<{ title: string; description: string; account: string \| undefined; children: ReactNode }>)` | [apps/web/src/app/_markets/frame.tsx](../../../apps/web/src/app/_markets/frame.tsx) |
| `ts_export` | `export function MarketUnavailable({ detail }: Readonly<{ detail: string }>)` | [apps/web/src/app/_markets/frame.tsx](../../../apps/web/src/app/_markets/frame.tsx) |
| `ts_export` | `export const MARKET_ROUTES = [` | [apps/web/src/app/_markets/navigation.tsx](../../../apps/web/src/app/_markets/navigation.tsx) |
| `ts_export` | `export function MarketNavigation({ account }: Readonly<{ account: string \| undefined }>)` | [apps/web/src/app/_markets/navigation.tsx](../../../apps/web/src/app/_markets/navigation.tsx) |
| `ts_export` | `export function ChoiceField({ label, value, options, onChange, }: Readonly<{ label: string; value: string; options: readonly Readonly<{ value: string; label: string; disabled?: boolean }>[]; onChange: (value: string) => void; }>)` | [apps/web/src/app/_markets/select.tsx](../../../apps/web/src/app/_markets/select.tsx) |
| `ts_export` | `export function SendOutcome({ outcome }: Readonly<{ outcome: PrecompileSendOutcome \| undefined }>)` | [apps/web/src/app/_markets/send-outcome.tsx](../../../apps/web/src/app/_markets/send-outcome.tsx) |
| `ts_export` | `export function WalletConnect({ account }: Readonly<{ account: string \| undefined }>)` | [apps/web/src/app/_markets/wallet-connect.tsx](../../../apps/web/src/app/_markets/wallet-connect.tsx) |
| `ts_export` | `export function human_web_app_scaffold()` | [apps/web/src/app/scaffold.ts](../../../apps/web/src/app/scaffold.ts) |

## auth

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export interface FeeLimitInput` | [apps/web/src/auth/native-fee-budget.ts](../../../apps/web/src/auth/native-fee-budget.ts) |
| `ts_export` | `export function NativeFeeBudgetFields({ control, disabled, }: Readonly<{ control: ReturnType<typeof useNativeFeeBudget>; disabled: boolean }>)` | [apps/web/src/auth/native-fee-budget-fields.tsx](../../../apps/web/src/auth/native-fee-budget-fields.tsx) |
| `ts_export` | `export function useNativeFeeBudget(client: HumanApiClient)` | [apps/web/src/auth/native-fee-budget-fields.tsx](../../../apps/web/src/auth/native-fee-budget-fields.tsx) |
| `ts_export` | `export function nativeFeeBudget( input: FeeLimitInput, asset: NativeFeeAsset \| undefined, ): NativeFeeBudget \| undefined` | [apps/web/src/auth/native-fee-budget.ts](../../../apps/web/src/auth/native-fee-budget.ts) |
| `ts_export` | `export function nativeFeeBudgetIdentity(budget: NativeFeeBudget \| undefined): string` | [apps/web/src/auth/native-fee-budget.ts](../../../apps/web/src/auth/native-fee-budget.ts) |
| `ts_export` | `export function parseFeeAmount(input: string, decimals: number): bigint \| undefined` | [apps/web/src/auth/native-fee-budget.ts](../../../apps/web/src/auth/native-fee-budget.ts) |
| `ts_export` | `export function validNativeFeeAsset(asset: NativeFeeAsset): boolean` | [apps/web/src/auth/native-fee-budget.ts](../../../apps/web/src/auth/native-fee-budget.ts) |
| `ts_export` | `export function validRequestCsrf(request: Request): boolean` | [apps/web/src/auth/server-session.ts](../../../apps/web/src/auth/server-session.ts) |
| `ts_export` | `export async function verifiedWebSession(cookie: string): Promise<VerifiedSessionIdentity \| undefined>` | [apps/web/src/auth/server-session.ts](../../../apps/web/src/auth/server-session.ts) |
| `ts_export` | `export const ACTIVE_ACCOUNT_STORAGE_KEY = "layerx.account";` | [apps/web/src/auth/session.ts](../../../apps/web/src/auth/session.ts) |
| `ts_export` | `export const APP_CSRF_COOKIE = "__Host-layerx_csrf";` | [apps/web/src/auth/session.ts](../../../apps/web/src/auth/session.ts) |
| `ts_export` | `export const APP_SESSION_COOKIE = "__Host-layerx-session";` | [apps/web/src/auth/session.ts](../../../apps/web/src/auth/session.ts) |
| `ts_export` | `export function csrfTokenFromCookie(cookie: string): string \| undefined` | [apps/web/src/auth/session.ts](../../../apps/web/src/auth/session.ts) |
| `ts_export` | `export function singleCurrentSessionId( sessions: readonly Readonly<{ session_id: string; current: boolean }>[], ): string \| undefined` | [apps/web/src/auth/session.ts](../../../apps/web/src/auth/session.ts) |
| `ts_export` | `export interface VerifiedSessionIdentity` | [apps/web/src/auth/session.ts](../../../apps/web/src/auth/session.ts) |
| `ts_export` | `export function useActiveAccountId(initial?: string): string \| undefined` | [apps/web/src/auth/use-active-account.ts](../../../apps/web/src/auth/use-active-account.ts) |

## design

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function human_design_tokens()` | [apps/web/src/design/tokens.ts](../../../apps/web/src/design/tokens.ts) |
| `ts_export` | `export const layerxUiContract = Object.freeze(` | [apps/web/src/design/tokens.ts](../../../apps/web/src/design/tokens.ts) |

## env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_GATEWAY_URL` | [apps/web/src/api/gateway.ts](../../../apps/web/src/api/gateway.ts) |
| `env_var` | `LAYERX_HUMAN_WEB_ORIGIN` | [apps/web/src/api/server.ts](../../../apps/web/src/api/server.ts) |
| `env_var` | `LAYERX_RUM_DEVELOPMENT_FALLBACK` | [apps/web/src/api/performance-sink.ts](../../../apps/web/src/api/performance-sink.ts) |
| `env_var` | `LAYERX_RUM_STORAGE_DIRECTORY` | [apps/web/src/api/performance-sink.ts](../../../apps/web/src/api/performance-sink.ts) |
| `env_var` | `LAYERX_SUPPORT_REPORT_BEARER_TOKEN` | [apps/web/src/server/support-reports.ts](../../../apps/web/src/server/support-reports.ts) |
| `env_var` | `LAYERX_SUPPORT_REPORT_DIR` | [apps/web/src/server/support-reports.ts](../../../apps/web/src/server/support-reports.ts) |
| `env_var` | `LAYERX_SUPPORT_REPORT_RETENTION` | [apps/web/src/server/support-reports.ts](../../../apps/web/src/server/support-reports.ts) |
| `env_var` | `PAXEER_GAS_STATION_CHAIN_ID` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `env_var` | `PAXEER_GAS_STATION_PAYMASTER` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `env_var` | `PAXEER_GAS_STATION_QUOTE_URL` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `env_var` | `PAXEER_GAS_STATION_SPONSOR` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `env_var` | `PAXEER_GAS_STATION_SUBMIT_URL` | [apps/web/src/api/gas-station.ts](../../../apps/web/src/api/gas-station.ts) |
| `env_var` | `PAXEER_X_EXPLORER_BASE_URL` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |

## explorer

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export async function accountActivityPage(identifier: string, cursor?: string, retiredLimit?: string) {` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export function explorerLink(target: ExplorerLinkTarget): string` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export class ExplorerUnavailableError extends Error` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export async function programRecord(identifier: string): Promise<ProgramRecord \| undefined>` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export async function receiptRecord(identifier: string): Promise<ExplorerRecord<ReceiptRecord>> {` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export async function resolveName(name: string): Promise<NameResolutionRecord \| undefined>` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export async function unifiedAccount(` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export function unifiedAccountAvailability(` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export class UnifiedAccountUnavailableError extends ExplorerUnavailableError` | [apps/web/src/explorer/client.ts](../../../apps/web/src/explorer/client.ts) |
| `ts_export` | `export { EXPLORER_NAVIGATION };` | [apps/web/src/explorer/components.tsx](../../../apps/web/src/explorer/components.tsx) |
| `ts_export` | `export function ExplorerFrame({ title, description, children, }: Readonly<{ title: ReactNode; description?: ReactNode; children: ReactNode }>)` | [apps/web/src/explorer/components.tsx](../../../apps/web/src/explorer/components.tsx) |
| `ts_export` | `export function ExplorerNotFound({ title, freshness, }: Readonly<{ title: string; freshness: ExplorerFreshness }>)` | [apps/web/src/explorer/components.tsx](../../../apps/web/src/explorer/components.tsx) |
| `ts_export` | `export function explorerReceiptPath(identifier: string): string` | [apps/web/src/explorer/components.tsx](../../../apps/web/src/explorer/components.tsx) |
| `ts_export` | `export function ExplorerUnavailable()` | [apps/web/src/explorer/components.tsx](../../../apps/web/src/explorer/components.tsx) |
| `ts_export` | `export function FreshnessDisplay({` | [apps/web/src/explorer/components.tsx](../../../apps/web/src/explorer/components.tsx) |
| `ts_export` | `export function verificationLabel(level: string): string {` | [apps/web/src/explorer/components.tsx](../../../apps/web/src/explorer/components.tsx) |
| `ts_export` | `export const EXPLORER_ANCHOR_PATH = "/paxeer-x/anchors";` | [apps/web/src/explorer/links.ts](../../../apps/web/src/explorer/links.ts) |
| `ts_export` | `export const EXPLORER_LINK_KINDS = [` | [apps/web/src/explorer/links.ts](../../../apps/web/src/explorer/links.ts) |
| `ts_export` | `export const EXPLORER_NAVIGATION = [` | [apps/web/src/explorer/links.ts](../../../apps/web/src/explorer/links.ts) |
| `ts_export` | `export type ExplorerLinkKind = (typeof EXPLORER_LINK_KINDS)[number];` | [apps/web/src/explorer/links.ts](../../../apps/web/src/explorer/links.ts) |
| `ts_export` | `export function explorerLinkPath(target: ExplorerLinkTarget): string` | [apps/web/src/explorer/links.ts](../../../apps/web/src/explorer/links.ts) |
| `ts_export` | `export type ExplorerLinkTarget =` | [apps/web/src/explorer/links.ts](../../../apps/web/src/explorer/links.ts) |
| `ts_export` | `export function parseExplorerBaseUrl(configured: string \| undefined): URL \| undefined` | [apps/web/src/explorer/links.ts](../../../apps/web/src/explorer/links.ts) |
| `ts_export` | `type AccountActivityRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export interface AccountIdentifier` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export type AccountIdentifierKind = "evm" \| "did" \| "account";` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function accountIdentifierPath(canonical: string): string` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type BatchRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type CheckpointRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function decodeNameResolution(value: unknown, at = "name"): NameResolutionRecord` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function decodeProgram(value: unknown, at = "program"): ProgramRecord` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function decodeProgramForIdentifier(value: unknown, identifier: string): ProgramRecord` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type EvidenceVerificationReport,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function explorer(): Readonly<` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type ExplorerFreshness,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type ExplorerPage,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type ExplorerRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type ExplorerVerificationLevel,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function human_web_explorer()` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type MirrorVerificationProvenance,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export interface NameResolutionRecord` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function parseAccountIdentifier(value: string): AccountIdentifier \| undefined` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type PaxeerActivityEvent,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type PaxeerActivityRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type PaxeerActivityWindow,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export type ProgramLifecycle = "active" \| "deprecated" \| "tombstoned";` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export interface ProgramRecord` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export type ProgramSourceStatus =` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export type ProgramUpgradePolicy =` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export interface ProgramValueAccountRecord` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export interface ProgramVersionRecord` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type ReceiptRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type ReportedAvailability,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type UnifiedAccountAvailabilityRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type UnifiedAccountEvidence,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type UnifiedAccountRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type UnifiedBalanceRecord,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type UnifiedIdentities,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `type UnifiedSettlement,` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function validExplorerCoordinate(value: string): boolean` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function validExplorerIdentifier(value: string): boolean` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |
| `ts_export` | `export function validExplorerName(value: string): boolean` | [apps/web/src/explorer/model.ts](../../../apps/web/src/explorer/model.ts) |

## journeys

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function Activity()` | [apps/web/src/journeys/activity/activity.tsx](../../../apps/web/src/journeys/activity/activity.tsx) |
| `ts_export` | `export function ActivityErrorSurface({ failure, onRetry }: Readonly<{ failure: ActivityFailure; onRetry: () => void }>)` | [apps/web/src/journeys/activity/activity.tsx](../../../apps/web/src/journeys/activity/activity.tsx) |
| `ts_export` | `export function human_web_journeys_Activity()` | [apps/web/src/journeys/activity/activity.tsx](../../../apps/web/src/journeys/activity/activity.tsx) |
| `ts_export` | `export function ActivityDetail({ entryId }: Readonly<{ entryId: string }>)` | [apps/web/src/journeys/activity/detail.tsx](../../../apps/web/src/journeys/activity/detail.tsx) |
| `ts_export` | `export function activityDate(timestamp: string): Date` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export type ActivityFailure = Readonly<` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function activityFailure(error: unknown): ActivityFailure` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function agentFilterOptions( agents: readonly Pick<Agent, "agent_id" \| "name">[], ): readonly FeedFilterOption[]` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function allEntryIds(page: ActivityPage): readonly string[]` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function detailUnresolved(detail: Readonly<Pick<ActivityEntryDetail, "state">>): boolean` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function emptyFilterValues(): FeedFilterValues` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function entryStatusKey(state: JourneyState): StatusKey` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function entryStatusLabel(state: JourneyState): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function entryUnresolved(state: JourneyState): boolean` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function entryVerification(evidence: readonly EvidenceRef[]): VerificationLevel` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function evidenceClassLabel(evidenceClass: EvidenceClass): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function explorerPath(` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export interface FeedDateRange` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export interface FeedFilterDef` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function feedFilterDefs(agents: readonly FeedFilterOption[]): FeedFilterDef[]` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export interface FeedFilterOption` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export type FeedFilterValues = Readonly<Record<string, string \| FeedDateRange \| undefined>>;` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function feedGroups(page: ActivityPage): FeedGroupView[]` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export interface FeedGroupView` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function feedIsEmpty(page: ActivityPage): boolean` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export interface FeedRowView` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function filterEchoLines( filter: ActivityFilter, agentNames: ReadonlyMap<string, string> = new Map(), ): readonly string[]` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function formatEntryDate(timestamp: string): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function formatEntryTimestamp(timestamp: string): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function kindLabel(kind: ActivityEntryKind): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export async function loadActivity( client: HumanApiClient, filter: ActivityFilter = {}, ): Promise<Readonly<` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function mergePages(current: ActivityPage, incoming: ActivityPage): ActivityPage` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function monthLabel(month: string): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function newExportKey(): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function plainSentence( entry: Readonly<Pick<ActivityEntry, "kind" \| "state" \| "summary_copy_key">>, ): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function receiptBacked(reference: EvidenceRef): boolean` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function safeExportArtefact(artefact: ExportArtefact): ExportArtefact` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function sameFilterValues(first: FeedFilterValues, second: FeedFilterValues): boolean` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function signedBaseUnits( money: Money \| undefined, direction: MoneyDirection \| undefined, ): ProtocolAmount` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function stageView(stage: JourneyStage): TimelineStageView` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export const STATUS_KEY_BY_STATE: Readonly<Record<JourneyState, StatusKey>> = Object.freeze(` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function stillCheckingLockReason(): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export interface TimelineStageView` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function toKitDirection(direction: MoneyDirection \| undefined): "inbound" \| "outbound" \| "other"` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function toWireFilter(values: FeedFilterValues): ActivityFilter` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function unsignedBaseUnits(money: Money): ProtocolAmount` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function validatedDetail(detail: ActivityEntryDetail): ActivityEntryDetail` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function verificationLabel(level: VerificationLevel): string` | [apps/web/src/journeys/activity/model.ts](../../../apps/web/src/journeys/activity/model.ts) |
| `ts_export` | `export function AgentControls(props: AgentControlsProps)` | [apps/web/src/journeys/agents/controls.tsx](../../../apps/web/src/journeys/agents/controls.tsx) |
| `ts_export` | `export function AgentCreateJourney()` | [apps/web/src/journeys/agents/create.tsx](../../../apps/web/src/journeys/agents/create.tsx) |
| `ts_export` | `export function AgentDetailScreen(props: AgentDetailProps)` | [apps/web/src/journeys/agents/detail.tsx](../../../apps/web/src/journeys/agents/detail.tsx) |
| `ts_export` | `export const AGENT_CURRENCY = "LXP";` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export const AGENT_LOCALE = "en-GB";` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface AgentControl` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface AgentControlContext` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export type AgentControlId =` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function agentListItems(page: AgentPage, locale: string): readonly AgentListItemView[]` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface AgentListItemView` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface AgentPresentation` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function agentPresentation(agent: Agent): AgentPresentation` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export class Agents` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export type AgentsLayout = "stacked" \| "master-detail";` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function agentsLayout(shell: AgentsShell): AgentsLayout` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface AgentsOptions` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export type AgentsShell = "mobile" \| "desktop";` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function agentStateVerified(agent: Agent): boolean` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export type AgentTone = "neutral" \| "accent" \| "success" \| "warning" \| "destructive";` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function apiErrorCode(error: unknown): string \| undefined` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function apiErrorSentence(error: unknown): string` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function archiveDispositionReady(agent: Agent): boolean` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function catalogSentence(copyKey: string): string` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function controlsFor(agent: Agent, context: AgentControlContext = {}): readonly AgentControl[]` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export const CREATION_DECISIONS = 3;` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface CreationDraft` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function creationHeadlineKey(progress: JourneyProgress): "agent.create.ready" \| "agent.create.partial"` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function creationReady(draft: CreationDraft): boolean` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface CreationStep` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function creationSteps(draft: CreationDraft): readonly [CreationStep, CreationStep, CreationStep]` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function evidenceReceiptVerified(evidence: readonly EvidenceRef[]): boolean` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function formatMoney(money: Money, locale: string): string` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function formatPlainDelay(seconds: number): string` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function formatPlainTimestamp(timestamp: string, locale: string): string` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface JourneyProgress` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function journeyProgress(journey: Journey): JourneyProgress` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function journeyStatusKey(state: JourneyState): StatusKey` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface KeyChallengePresentation` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function keyChallengePresentation(challenge: KeyChallenge, locale: string): KeyChallengePresentation` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function mutationOutcomeUnknown(error: unknown): boolean` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function parseMonthlyLimit(input: string, currency: string): Money \| undefined` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function parseRotationTiming(delay: string, window: string): RotationTiming \| undefined` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface QuotePresentation` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function quotePresentation(quote: MoveQuote, locale: string): QuotePresentation` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface RotationTiming` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface SpendPresentation` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function spendPresentation(agent: Agent, locale: string): SpendPresentation` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function stageReceiptVerified(stage: JourneyStage): boolean` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export interface StageView` | [apps/web/src/journeys/agents/model.ts](../../../apps/web/src/journeys/agents/model.ts) |
| `ts_export` | `export function JourneyStages({ progress }: Readonly<{ progress: JourneyProgress }>)` | [apps/web/src/journeys/agents/progress.tsx](../../../apps/web/src/journeys/agents/progress.tsx) |
| `ts_export` | `export function useAgentsShell(): AgentsShell` | [apps/web/src/journeys/agents/shell.ts](../../../apps/web/src/journeys/agents/shell.ts) |
| `ts_export` | `export function AgentsSurface({ ownerAccount, }: Readonly<{ ownerAccount?: string }> = {})` | [apps/web/src/journeys/agents/surface.tsx](../../../apps/web/src/journeys/agents/surface.tsx) |
| `ts_export` | `export function base64UrlFromBytes(bytes: Uint8Array): string` | [apps/web/src/journeys/approvals/ceremony.ts](../../../apps/web/src/journeys/approvals/ceremony.ts) |
| `ts_export` | `export function base64UrlFromText(value: string): string` | [apps/web/src/journeys/approvals/ceremony.ts](../../../apps/web/src/journeys/approvals/ceremony.ts) |
| `ts_export` | `export function browserPasskeyAuthenticator(): PasskeyAuthenticator` | [apps/web/src/journeys/approvals/ceremony.ts](../../../apps/web/src/journeys/approvals/ceremony.ts) |
| `ts_export` | `export function bytesFromBase64Url(value: string): Uint8Array` | [apps/web/src/journeys/approvals/ceremony.ts](../../../apps/web/src/journeys/approvals/ceremony.ts) |
| `ts_export` | `export type PasskeyAuthenticator = (ceremony: OpaqueCredential) => Promise<OpaqueCredential>;` | [apps/web/src/journeys/approvals/ceremony.ts](../../../apps/web/src/journeys/approvals/ceremony.ts) |
| `ts_export` | `export async function performStepUp( client: HumanApiClient, confirms: OperationDigest, authenticator: PasskeyAuthenticator, ): Promise<StepUpEvidence>` | [apps/web/src/journeys/approvals/ceremony.ts](../../../apps/web/src/journeys/approvals/ceremony.ts) |
| `ts_export` | `export function textFromBase64Url(value: string): string` | [apps/web/src/journeys/approvals/ceremony.ts](../../../apps/web/src/journeys/approvals/ceremony.ts) |
| `ts_export` | `export class Approvals` | [apps/web/src/journeys/approvals/controller.ts](../../../apps/web/src/journeys/approvals/controller.ts) |
| `ts_export` | `export interface ApprovalsOptions` | [apps/web/src/journeys/approvals/controller.ts](../../../apps/web/src/journeys/approvals/controller.ts) |
| `ts_export` | `export async function loadProgramApprovals(client: HumanApiClient): Promise<ProgramApprovalSummary[]>` | [apps/web/src/journeys/approvals/controller.ts](../../../apps/web/src/journeys/approvals/controller.ts) |
| `ts_export` | `export function ApprovalsJourneyScreen({ approvalId, program = false, client: suppliedClient, }: Readonly<{ approvalId?: string; program?: boolean; client?: HumanApiClient; }>)` | [apps/web/src/journeys/approvals/journey-screen.tsx](../../../apps/web/src/journeys/approvals/journey-screen.tsx) |
| `ts_export` | `export function activityRoute(entryId: string): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export interface ApprovalInboxItem` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function approvalInboxItem(summary: ApprovalSummary, at: Date): ApprovalInboxItem` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export type ApprovalOutcome =` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function approvalRoute(approvalId: string): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export const APPROVALS_ROUTE = "/app/approvals";` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function approvalStateCopyKey(state: ApprovalState): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export interface ApprovalStatePresentation` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function approvalStatePresentation( state: ApprovalState, stateCopyKey: string = approvalStateCopyKey(state), ): ApprovalStatePresentation` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export type ApprovalTone = "destructive" \| "neutral" \| "success" \| "warning";` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function approveConsequence(detail: ApprovalDetail): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function canApprove(detail: ApprovalDetail, at: Date): boolean` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function canDecideProgram(detail: ProgramApprovalDetail, at: Date): boolean` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function catalogMessage(copyKey: string, fallbackKey: string): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function convergedOutcome(detail: ApprovalDetail): ApprovalOutcome` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function decidedOutcome(decision: ApprovalDecision): ApprovalOutcome` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function decisionKey(): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function defectiveOutcome(): ApprovalOutcome` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export interface ExpiryCountdown` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function expiryCountdown(expiresAt: Timestamp, at: Date): ExpiryCountdown` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function failureOutcome(detail: ApiError): ApprovalOutcome \| undefined` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function heldDigest(detail: ApprovalDetail): OperationDigest \| undefined` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function journeyStatus(state: JourneyState): StatusKey` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export const MONEY_LOCALE = "en-US";` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function moneyLabel(money: Money): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function pendingApprovalCount(summaries: readonly ApprovalSummary[]): number` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function programApprovalRoute(approvalId: string): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function programApprovalState(state: ProgramApprovalState): ApprovalStatePresentation` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function programApprovalTitle(summary: ProgramApprovalSummary): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function programDecisionMessage(decision: ProgramApprovalDecision): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function protocolMoney(money: Money): Readonly<` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function releasedActivity(page: ActivityPage, approvalId: string): ActivityEntry \| undefined` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function requestedAtLabel(createdAt: Timestamp): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function stepUpEvidenceReference(evidence: StepUpEvidence): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function validateProgramApproval(summary: ProgramApprovalSummary): void` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function validateProgramBudget(detail: ProgramApprovalDetail, budget: ProgramApprovalBudget): void` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function verificationLabel(level: string): string {` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function verifiedMoneyLabel(budget: VerifiedMoney): string` | [apps/web/src/journeys/approvals/model.ts](../../../apps/web/src/journeys/approvals/model.ts) |
| `ts_export` | `export function ProgramApprovalDetailCard({ detail, controller, at, shell, onChanged }: Readonly<{ detail: ProgramApprovalDetail; controller: Approvals; at: Date; shell: Shell; onChanged: () => Promise<void>; }>)` | [apps/web/src/journeys/approvals/programs.tsx](../../../apps/web/src/journeys/approvals/programs.tsx) |
| `ts_export` | `export function ProgramApprovalInboxRows({ approvals, at, onOpen, selectedId }: Readonly<{ approvals: readonly ProgramApprovalSummary[]; at: Date; onOpen?: ((id: string) => void) \| undefined; selectedId?: string \| undefined; }>)` | [apps/web/src/journeys/approvals/programs.tsx](../../../apps/web/src/journeys/approvals/programs.tsx) |
| `ts_export` | `export function ApprovalCountdown({ expiresAt, at, }: Readonly<{ expiresAt: Timestamp; at: Date; }>)` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function ApprovalDecisionConfirmations({ detail, shell, approveOpen, rejectOpen, onApproveOpenChange, onRejectOpenChange, onConfirmApprove, onConfirmReject, deciding = false, }: Readonly<{ detail: ApprovalDetail;` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function ApprovalDetailCard({ detail, at, onApprove, onReject, outcome, released, deciding = false, shell, }: Readonly<{ detail: ApprovalDetail; at: Date;` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function ApprovalInboxList({ approvals, programApprovals = [], onOpenProgram, at, onOpen, selectedId, }: Readonly<{ approvals: readonly ApprovalSummary[]; programApprovals?: readonly ProgramApprovalSummary[]; onOpenProgram?: ((approvalId: string) => void) \| undefined; at: Date;` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function ApprovalOutcomePanel({ outcome, released, }: Readonly<{ outcome: ApprovalOutcome; released?: ActivityEntryDetail \| undefined; }>)` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function ApprovalStateBadge({ state, stateCopyKey, }: Readonly<{ state: ApprovalState; stateCopyKey?: string; }>)` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function DesktopApprovalSplit({ approvals, programApprovals = [], onOpenProgram, at, onOpen, selectedId, children, }: Readonly<{ approvals: readonly ApprovalSummary[]; programApprovals?: readonly ProgramApprovalSummary[]; onOpenProgram?: ((approvalId: string) => void) \| undefined;` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function MobileApprovalInbox({ approvals, programApprovals = [], onOpenProgram, at, onOpen, }: Readonly<{ approvals: readonly ApprovalSummary[]; programApprovals?: readonly ProgramApprovalSummary[]; onOpenProgram?: ((approvalId: string) => void) \| undefined; at: Date; onOpen?: ((approvalId: string) => void) \| undefined;` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function ReleasedActivitySection({ entry }: Readonly<{ entry?: ActivityEntryDetail \| undefined; }>)` | [apps/web/src/journeys/approvals/screens.tsx](../../../apps/web/src/journeys/approvals/screens.tsx) |
| `ts_export` | `export function Home()` | [apps/web/src/journeys/home/home.tsx](../../../apps/web/src/journeys/home/home.tsx) |
| `ts_export` | `export function agentsSummary(agents: readonly Agent[]): string` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export function approvalBadge( approvals: readonly ApprovalSummary[], programs: readonly ProgramApprovalSummary[] = [], ): Readonly<` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export function classifyHomeFailure(error: unknown): HomeLoadFailure` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export const HOME_ACTIVITY_LIMIT = 5;` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export const HOME_DESTINATIONS = Object.freeze(` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export interface HomeActionItem` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export function homeActions(): readonly [HomeActionItem, HomeActionItem, HomeActionItem]` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export interface HomeActivityRow` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export function homeActivityRows(entries: readonly ActivityEntryDetail[]): readonly HomeActivityRow[]` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export interface HomeAgentRow` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export function homeAgentRows(agents: readonly Agent[]): readonly HomeAgentRow[]` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export type HomeBalance =` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export function homeBalance(balance?: AccountBalance): HomeBalance` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export interface HomeData` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export type HomeLoadFailure = "offline" \| "error";` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export async function loadHome(client: HumanApiClient): Promise<HomeData>` | [apps/web/src/journeys/home/model.ts](../../../apps/web/src/journeys/home/model.ts) |
| `ts_export` | `export const ACCOUNT_STORAGE_KEY = ACTIVE_ACCOUNT_STORAGE_KEY;` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export const AMOUNT_LOCALE = "en";` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export async function authorizeMoveNativeAccess( client: HumanApiClient, request: NativeSendAccessRequest, ): Promise<string>` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function clearPendingMove(storage: Pick<Storage, "removeItem">): void` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export async function commitMove( client: HumanApiClient, quoteId: string, attemptKey: string, nativeAccessId?: string, ): Promise<MoveCommitOutcome>` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function destinationOptions(agents: readonly Agent[]): readonly MoveDestinationOption[]` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export const HOME_ROUTE = "/app";` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function moneyAmountNumber(money: Money): ProtocolAmount` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function moneyWords(money: Money, locale: string = AMOUNT_LOCALE): string` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export const MOVE_CURRENCY = "LXP";` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export const MOVE_DECISIONS = 3;` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export const MOVE_ROUTE = "/app/move";` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export const MOVE_STEP_IDS = ["who", "how-much", "review"] as const;` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export type MoveCommitOutcome =` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export interface MoveDestinationOption` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export type MoveFailure =` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function moveFailure(error: unknown): MoveFailure` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export interface MoveQuoteInput` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export type MoveQuoteOutcome =` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export type MoveResult =` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function moveResult(journey: Journey): MoveResult` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export interface MoveReview` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function moveReview(quote: MoveQuote): MoveReview` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export interface MoveStagePresentation` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function moveStages(journey: Journey): readonly MoveStagePresentation[]` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export interface MoveStepCopy` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function moveSteps(): readonly [MoveStepCopy, MoveStepCopy, MoveStepCopy]` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function moveSummary(draft: MoveSummaryDraft, quote?: MoveQuote): readonly MoveSummaryItem[]` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export interface MoveSummaryDraft` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export interface MoveSummaryItem` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function newMoveAttemptKey(): string` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export const OTHER_ACCOUNT_OPTION = "other-account";` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function parseMoveAmount(input: string): bigint \| undefined` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export const PENDING_MOVE_STORAGE_KEY = "layerx.move.pending.v1";` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export interface PendingMoveAttempt` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function pendingMoveAttempt( storage: Pick<Storage, "getItem" \| "removeItem">, source: string \| undefined, ): PendingMoveAttempt \| undefined` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function persistPendingMove( storage: Pick<Storage, "setItem">, attempt: PendingMoveAttempt, ): void` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function quoteExpired(quote: MoveQuote, now: Date = new Date()): boolean` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function receiptBacked(journey: Journey): boolean` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function receiptEvidence(journey: Journey): readonly EvidenceRef[]` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export async function requestMoveQuote( client: HumanApiClient, input: MoveQuoteInput, ): Promise<MoveQuoteOutcome>` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function stillCheckingLookup( client: HumanApiClient, journeyId: string, onJourney: (journey: Journey) => void, ): () => Promise<"pending" \| "resolved">` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function storedAccount(storage: Pick<Storage, "getItem">): string \| undefined` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function timestampWords(timestamp: string): string` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function verifiedOutcomeEvidence(item: EvidenceRef): boolean` | [apps/web/src/journeys/move/model.ts](../../../apps/web/src/journeys/move/model.ts) |
| `ts_export` | `export function MoveMoney()` | [apps/web/src/journeys/move/move.tsx](../../../apps/web/src/journeys/move/move.tsx) |
| `ts_export` | `export interface NotificationLanding` | [apps/web/src/journeys/notifications/controller.ts](../../../apps/web/src/journeys/notifications/controller.ts) |
| `ts_export` | `export class Notifications` | [apps/web/src/journeys/notifications/controller.ts](../../../apps/web/src/journeys/notifications/controller.ts) |
| `ts_export` | `export interface NotificationsOptions` | [apps/web/src/journeys/notifications/controller.ts](../../../apps/web/src/journeys/notifications/controller.ts) |
| `ts_export` | `export function notificationItems( notifications: readonly PresentedNotification[], ): KitNotificationItem[]` | [apps/web/src/journeys/notifications/model.ts](../../../apps/web/src/journeys/notifications/model.ts) |
| `ts_export` | `export const NOTIFICATIONS_ROUTE = "/app/notifications";` | [apps/web/src/journeys/notifications/model.ts](../../../apps/web/src/journeys/notifications/model.ts) |
| `ts_export` | `export interface PresentedNotification` | [apps/web/src/journeys/notifications/model.ts](../../../apps/web/src/journeys/notifications/model.ts) |
| `ts_export` | `export function presentedNotifications(page: NotificationPage): readonly PresentedNotification[]` | [apps/web/src/journeys/notifications/model.ts](../../../apps/web/src/journeys/notifications/model.ts) |
| `ts_export` | `export function safeDeepLink(value: string): string \| undefined` | [apps/web/src/journeys/notifications/model.ts](../../../apps/web/src/journeys/notifications/model.ts) |
| `ts_export` | `export function unreadNotificationCount(notifications: readonly PresentedNotification[]): number` | [apps/web/src/journeys/notifications/model.ts](../../../apps/web/src/journeys/notifications/model.ts) |
| `ts_export` | `export function notificationById( notifications: readonly PresentedNotification[], id: string, ): PresentedNotification \| undefined` | [apps/web/src/journeys/notifications/screens.tsx](../../../apps/web/src/journeys/notifications/screens.tsx) |
| `ts_export` | `export function NotificationsArchiveScreen()` | [apps/web/src/journeys/notifications/screens.tsx](../../../apps/web/src/journeys/notifications/screens.tsx) |
| `ts_export` | `export function NotificationCenterProvider({ children, client: suppliedClient, }: Readonly<{ children: ReactNode; client?: HumanApiClient; }>)` | [apps/web/src/journeys/notifications/store.tsx](../../../apps/web/src/journeys/notifications/store.tsx) |
| `ts_export` | `export type NotificationCenterState =` | [apps/web/src/journeys/notifications/store.tsx](../../../apps/web/src/journeys/notifications/store.tsx) |
| `ts_export` | `export function useNotificationCenter(): NotificationCenterValue` | [apps/web/src/journeys/notifications/store.tsx](../../../apps/web/src/journeys/notifications/store.tsx) |
| `ts_export` | `export class CeremonyCancelled extends Error` | [apps/web/src/journeys/onboarding/ceremony.ts](../../../apps/web/src/journeys/onboarding/ceremony.ts) |
| `ts_export` | `export async function performAssertionCeremony(ceremony: string): Promise<string>` | [apps/web/src/journeys/onboarding/ceremony.ts](../../../apps/web/src/journeys/onboarding/ceremony.ts) |
| `ts_export` | `export async function performRegistrationCeremony(ceremony: string): Promise<string>` | [apps/web/src/journeys/onboarding/ceremony.ts](../../../apps/web/src/journeys/onboarding/ceremony.ts) |
| `ts_export` | `export function DeviceList()` | [apps/web/src/journeys/onboarding/devices.tsx](../../../apps/web/src/journeys/onboarding/devices.tsx) |
| `ts_export` | `export function accountActive(journey: Journey): boolean` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export function completedStageIds(journey: Journey): ReadonlySet<string>` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export const CREATE_ACCOUNT_DECISIONS = 2;` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export function journeyStatusKey(state: JourneyState): StatusKey` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export const ONBOARDING_DECISION_LIMIT = 3;` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export const PROTOCOL_IDENTITY_STAGE_KEY = "onboarding.stage.setting-up-your-protocol-identity";` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export const SIGN_IN_DECISIONS = 1;` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export interface StagePresentation` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export function stagePresentation(stage: JourneyStage): StagePresentation` | [apps/web/src/journeys/onboarding/model.ts](../../../apps/web/src/journeys/onboarding/model.ts) |
| `ts_export` | `export function Onboarding({ initialSelection, initiallyAuthenticated = false, returnTo, }: Readonly<{ initialSelection: ShellSelection; initiallyAuthenticated?: boolean; returnTo?: string \| undefined; }>)` | [apps/web/src/journeys/onboarding/onboarding.tsx](../../../apps/web/src/journeys/onboarding/onboarding.tsx) |

## kit

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function a11y()` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export function assertSemanticTokenContrast(stylesheet: string): void` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export function contrastRatio(foreground: string, background: string): number` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export interface ExplicitCurrencyAmountOptions` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export function formatExplicitCurrencyAmount( value: number, { currency, locale, decimals = 2, signed = true }: ExplicitCurrencyAmountOptions, ): string` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export function LiveRegion({ children, priority = "polite", className, ...props }: LiveRegionProps)` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export interface LiveRegionProps extends Omit<HTMLAttributes<HTMLSpanElement>, "role">` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export const MINIMUM_TEXT_CONTRAST = 4.5;` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export const SEMANTIC_CONTRAST_PAIRS = [` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export interface SemanticContrastResult` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export function semanticContrastResults(stylesheet: string): readonly SemanticContrastResult[]` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export function semanticTokenValues(stylesheet: string): ReadonlyMap<string, string>` | [apps/web/src/kit/a11y.ts](../../../apps/web/src/kit/a11y.ts) |
| `ts_export` | `export function ActivityEvidenceBadge({ children }: Readonly<{ children: ReactNode }>)` | [apps/web/src/kit/activity.tsx](../../../apps/web/src/kit/activity.tsx) |
| `ts_export` | `export interface ActivityFeedGroup` | [apps/web/src/kit/activity.tsx](../../../apps/web/src/kit/activity.tsx) |
| `ts_export` | `export interface ActivityFeedProps` | [apps/web/src/kit/activity.tsx](../../../apps/web/src/kit/activity.tsx) |
| `ts_export` | `export interface ActivityFeedRow` | [apps/web/src/kit/activity.tsx](../../../apps/web/src/kit/activity.tsx) |
| `ts_export` | `export function DesktopActivityFeed({ groups, onSelect, dateLabel, columns, amountSortEnabled }: ActivityFeedProps)` | [apps/web/src/kit/activity.tsx](../../../apps/web/src/kit/activity.tsx) |
| `ts_export` | `export function MobileActivityFeed({ groups, onSelect, dateLabel }: ActivityFeedProps)` | [apps/web/src/kit/activity.tsx](../../../apps/web/src/kit/activity.tsx) |
| `ts_export` | `export type ConfirmationProps =` | [apps/web/src/kit/confirm.tsx](../../../apps/web/src/kit/confirm.tsx) |
| `ts_export` | `export function DesktopConfirmation(props: ConfirmationProps)` | [apps/web/src/kit/confirm.tsx](../../../apps/web/src/kit/confirm.tsx) |
| `ts_export` | `export function MobileConfirmation(props: ConfirmationProps)` | [apps/web/src/kit/confirm.tsx](../../../apps/web/src/kit/confirm.tsx) |
| `ts_export` | `export interface TypedConfirmation` | [apps/web/src/kit/confirm.tsx](../../../apps/web/src/kit/confirm.tsx) |
| `ts_export` | `export type ControlAvailability =` | [apps/web/src/kit/control.tsx](../../../apps/web/src/kit/control.tsx) |
| `ts_export` | `export function DisabledReason({ id, children }: Readonly<{ id: string; children?: ReactNode }>)` | [apps/web/src/kit/control.tsx](../../../apps/web/src/kit/control.tsx) |
| `ts_export` | `export function KitButton({ disabled = false, disabledReason, className, "aria-describedby": describedBy, ...props }: KitButtonProps)` | [apps/web/src/kit/control.tsx](../../../apps/web/src/kit/control.tsx) |
| `ts_export` | `export type KitButtonProps = Omit<ButtonProps, "disabled"> & ControlAvailability;` | [apps/web/src/kit/control.tsx](../../../apps/web/src/kit/control.tsx) |
| `ts_export` | `export const REDUCED_MOTION_CLASS =` | [apps/web/src/kit/control.tsx](../../../apps/web/src/kit/control.tsx) |
| `ts_export` | `export interface DeviceListItem` | [apps/web/src/kit/device-list.tsx](../../../apps/web/src/kit/device-list.tsx) |
| `ts_export` | `export function DeviceSessionList({ items }: Readonly<{ items: readonly DeviceListItem[] }>)` | [apps/web/src/kit/device-list.tsx](../../../apps/web/src/kit/device-list.tsx) |
| `ts_export` | `export function ActionGrid({ actions, onAction, className }: ActionGridProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type ActionGridIcon = keyof typeof ACTION_ICONS;` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export interface ActionGridItem` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export interface ActionGridProps` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function BalanceSummary({ label, value, currency, hidden, onHiddenChange, className, }: BalanceSummaryProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export interface BalanceSummaryProps` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function CountBadge({ label, ...props }: CountBadgeProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type CountBadgeProps = Omit<BadgeProps, "children"> & Readonly<{ label: string }>;` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function KitIconTile(props: KitIconTileProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type KitIconTileProps = ComponentProps<typeof IconTile>;` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function KitList(props: KitListProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function KitListItem(props: KitListItemProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type KitListItemProps = ListItemProps;` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type KitListProps = ComponentProps<typeof List>;` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function KitOptionList(props: KitOptionListProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type KitOptionListProps = ComponentProps<typeof OptionList>;` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function KitSectionHeader(props: KitSectionHeaderProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type KitSectionHeaderProps = ComponentProps<typeof SectionHeader>;` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function KitTextField({ label, ...props }: KitTextFieldProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type KitTextFieldProps = Omit<InputProps, "id" \| "placeholder"> &` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function KitViewAllChip(props: KitViewAllChipProps)` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type KitViewAllChipProps = ComponentProps<typeof ViewAllChip>;` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export type { OptionListItem };` | [apps/web/src/kit/display.tsx](../../../apps/web/src/kit/display.tsx) |
| `ts_export` | `export function ExplorerEvidenceInput({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>)` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function ExplorerExternalLink({ href, children, }: Readonly<{ href: string; children: ReactNode }>)` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function ExplorerFreshness({ title, description, current, }: Readonly<{ title: string; description: string; current: boolean }>)` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function ExplorerLink({ href, children }: Readonly<{ href: string; children: ReactNode }>)` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function ExplorerLookupForm({ action, kind, label, placeholder, submitLabel, }: Readonly<{ action: string; kind: "receipt" \| "account" \| "program"; label: string; placeholder: string; submitLabel: string;` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function ExplorerNavigation({ label, items, }: Readonly<{ label: string; items: readonly Readonly<{ href: string; label: string }>[] }>)` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function ExplorerPanel({ title, children, }: Readonly<{ title: ReactNode; children: ReactNode }>)` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function ExplorerTable({ caption, columns, rows, }: Readonly<{ caption: string; columns: readonly string[]; rows: readonly Readonly<{ id: string; cells: readonly ReactNode[] }>[]; }>)` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function ExplorerVerificationBadge({ label, unverified = false, }: Readonly<{ label: string; unverified?: boolean }>)` | [apps/web/src/kit/explorer.tsx](../../../apps/web/src/kit/explorer.tsx) |
| `ts_export` | `export function TextField({ label, errorMessage, error, className, ...props }: TextFieldProps)` | [apps/web/src/kit/field.tsx](../../../apps/web/src/kit/field.tsx) |
| `ts_export` | `export type TextFieldProps = InputBaseProps & Readonly<` | [apps/web/src/kit/field.tsx](../../../apps/web/src/kit/field.tsx) |
| `ts_export` | `export type ConfirmationKind = "reversible" \| "destructive" \| "irreversible";` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function confirmationVariant(kind: ConfirmationKind): "primary" \| "destructive"` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function directionWord(direction: MoneyDirection): string` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function feeMathTotal(steps: readonly Readonly<{ amount: ProtocolAmount }>[]): ProtocolAmount` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function kit(): Readonly<` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export type MoneyDirection = "inbound" \| "outbound" \| "other";` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export const PATTERN_PAIRS = [` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export type ProtocolAmount = number & Readonly<{ [protocolAmountBrand]: "ProtocolAmount" }>;` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function protocolAmount(value: number): ProtocolAmount` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export const STATUS_PRESENTATIONS = Object.freeze(` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export type StatusKey = keyof typeof STATUS_PRESENTATIONS;` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function statusKeyFromCopyKey(copyKey: string): StatusKey` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function statusPresentation(status: StatusKey): Readonly<` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export type StatusTone = (typeof STATUS_PRESENTATIONS)[StatusKey]["tone"];` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function typedConfirmationReady( kind: ConfirmationKind, typed?: Readonly<{ expectedValue: string; value: string }>, ): boolean` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export const VERIFICATION_PRESENTATIONS = Object.freeze(` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export type VerificationKey = keyof typeof VERIFICATION_PRESENTATIONS;` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function verificationRung(level: string): VerificationRung` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function verificationWord(level: string): string {` | [apps/web/src/kit/model.ts](../../../apps/web/src/kit/model.ts) |
| `ts_export` | `export function CopyableIdentifier({ label, value, onCopied, className, }: Readonly<{ label: string; value: string; onCopied?: (value: string) => void; className?: string }>)` | [apps/web/src/kit/money.tsx](../../../apps/web/src/kit/money.tsx) |
| `ts_export` | `export function FeeMathDisclosure({ open, onOpenChange, steps, title = copyEntry("fee.disclosure.title").message, totalLabel = copyEntry("fee.disclosure.total").message, currency, locale, symbol, decimals, platform, portalContainer,` | [apps/web/src/kit/money.tsx](../../../apps/web/src/kit/money.tsx) |
| `ts_export` | `export interface FeeMathDisclosureProps` | [apps/web/src/kit/money.tsx](../../../apps/web/src/kit/money.tsx) |
| `ts_export` | `export interface FeeMathStep` | [apps/web/src/kit/money.tsx](../../../apps/web/src/kit/money.tsx) |
| `ts_export` | `export function LabelValue({ label, value, className, }: Readonly<{ label: ReactNode; value: ReactNode; className?: string }>)` | [apps/web/src/kit/money.tsx](../../../apps/web/src/kit/money.tsx) |
| `ts_export` | `export function SignedWordedAmount({ value, currency, locale, decimals, direction, className, ...props }: SignedWordedAmountProps)` | [apps/web/src/kit/money.tsx](../../../apps/web/src/kit/money.tsx) |
| `ts_export` | `export type SignedWordedAmountProps = Omit<` | [apps/web/src/kit/money.tsx](../../../apps/web/src/kit/money.tsx) |
| `ts_export` | `export function StatusPill({ status, className }: Readonly<{ status: StatusKey; className?: string }>)` | [apps/web/src/kit/money.tsx](../../../apps/web/src/kit/money.tsx) |
| `ts_export` | `export type CodeEntryProps = Omit<ComponentProps<typeof CodeEntry>, "platform">;` | [apps/web/src/kit/pattern-code-entry.tsx](../../../apps/web/src/kit/pattern-code-entry.tsx) |
| `ts_export` | `export function DesktopCodeEntry(props: CodeEntryProps)` | [apps/web/src/kit/pattern-code-entry.tsx](../../../apps/web/src/kit/pattern-code-entry.tsx) |
| `ts_export` | `export function MobileCodeEntry(props: CodeEntryProps)` | [apps/web/src/kit/pattern-code-entry.tsx](../../../apps/web/src/kit/pattern-code-entry.tsx) |
| `ts_export` | `export function DesktopDetail(props: DetailProps)` | [apps/web/src/kit/pattern-detail.tsx](../../../apps/web/src/kit/pattern-detail.tsx) |
| `ts_export` | `export type DetailProps = Omit<ComponentProps<typeof DetailDisclosure>, "platform">;` | [apps/web/src/kit/pattern-detail.tsx](../../../apps/web/src/kit/pattern-detail.tsx) |
| `ts_export` | `export function MobileDetail(props: DetailProps)` | [apps/web/src/kit/pattern-detail.tsx](../../../apps/web/src/kit/pattern-detail.tsx) |
| `ts_export` | `export function DesktopFilters(props: FiltersProps)` | [apps/web/src/kit/pattern-filters.tsx](../../../apps/web/src/kit/pattern-filters.tsx) |
| `ts_export` | `export type FiltersProps = Omit<ComponentProps<typeof FilterBar>, "platform">;` | [apps/web/src/kit/pattern-filters.tsx](../../../apps/web/src/kit/pattern-filters.tsx) |
| `ts_export` | `export function MobileFilters(props: FiltersProps)` | [apps/web/src/kit/pattern-filters.tsx](../../../apps/web/src/kit/pattern-filters.tsx) |
| `ts_export` | `export function DesktopMoneyList(props: MoneyListProps)` | [apps/web/src/kit/pattern-money-list.tsx](../../../apps/web/src/kit/pattern-money-list.tsx) |
| `ts_export` | `export function MobileMoneyList(props: MoneyListProps)` | [apps/web/src/kit/pattern-money-list.tsx](../../../apps/web/src/kit/pattern-money-list.tsx) |
| `ts_export` | `export type MoneyListProps = Omit<ComponentProps<typeof MoneyList>, "platform">;` | [apps/web/src/kit/pattern-money-list.tsx](../../../apps/web/src/kit/pattern-money-list.tsx) |
| `ts_export` | `export function DesktopNavigation(props: NavigationProps)` | [apps/web/src/kit/pattern-navigation.tsx](../../../apps/web/src/kit/pattern-navigation.tsx) |
| `ts_export` | `export function MobileNavigation(props: NavigationProps)` | [apps/web/src/kit/pattern-navigation.tsx](../../../apps/web/src/kit/pattern-navigation.tsx) |
| `ts_export` | `export type NavigationProps = Omit<ComponentProps<typeof AppShell>, "platform">;` | [apps/web/src/kit/pattern-navigation.tsx](../../../apps/web/src/kit/pattern-navigation.tsx) |
| `ts_export` | `export function DesktopNotifications({ view, ...props }: DesktopNotificationsProps)` | [apps/web/src/kit/pattern-notifications.tsx](../../../apps/web/src/kit/pattern-notifications.tsx) |
| `ts_export` | `export type DesktopNotificationsProps =` | [apps/web/src/kit/pattern-notifications.tsx](../../../apps/web/src/kit/pattern-notifications.tsx) |
| `ts_export` | `export type KitNotificationItem = MobileNotificationsPrimitiveProps["items"][number];` | [apps/web/src/kit/pattern-notifications.tsx](../../../apps/web/src/kit/pattern-notifications.tsx) |
| `ts_export` | `export function MobileNotifications(props: MobileNotificationsProps)` | [apps/web/src/kit/pattern-notifications.tsx](../../../apps/web/src/kit/pattern-notifications.tsx) |
| `ts_export` | `export type MobileNotificationsProps = MobileNotificationsPrimitiveProps;` | [apps/web/src/kit/pattern-notifications.tsx](../../../apps/web/src/kit/pattern-notifications.tsx) |
| `ts_export` | `export function DesktopPrimaryAction(props: KitPrimaryActionProps)` | [apps/web/src/kit/pattern-primary-action.tsx](../../../apps/web/src/kit/pattern-primary-action.tsx) |
| `ts_export` | `export type KitPrimaryActionProps = PrimaryActionBaseProps & ControlAvailability;` | [apps/web/src/kit/pattern-primary-action.tsx](../../../apps/web/src/kit/pattern-primary-action.tsx) |
| `ts_export` | `export function MobilePrimaryAction(props: KitPrimaryActionProps)` | [apps/web/src/kit/pattern-primary-action.tsx](../../../apps/web/src/kit/pattern-primary-action.tsx) |
| `ts_export` | `export function DesktopSearch(props: SearchProps)` | [apps/web/src/kit/pattern-search.tsx](../../../apps/web/src/kit/pattern-search.tsx) |
| `ts_export` | `export function MobileSearch(props: SearchProps)` | [apps/web/src/kit/pattern-search.tsx](../../../apps/web/src/kit/pattern-search.tsx) |
| `ts_export` | `export type SearchProps = Omit<ComponentProps<typeof GlobalSearch>, "platform">;` | [apps/web/src/kit/pattern-search.tsx](../../../apps/web/src/kit/pattern-search.tsx) |
| `ts_export` | `export function DesktopWizard(props: WizardProps)` | [apps/web/src/kit/pattern-wizard.tsx](../../../apps/web/src/kit/pattern-wizard.tsx) |
| `ts_export` | `export function MobileWizard(props: WizardProps)` | [apps/web/src/kit/pattern-wizard.tsx](../../../apps/web/src/kit/pattern-wizard.tsx) |
| `ts_export` | `export type WizardProps = Omit<ComponentProps<typeof Wizard>, "platform">;` | [apps/web/src/kit/pattern-wizard.tsx](../../../apps/web/src/kit/pattern-wizard.tsx) |
| `ts_export` | `export function PerformanceLoadingCard({ plane, label, }: Readonly<{ plane: "app" \| "explorer"; label: ReactNode }>)` | [apps/web/src/kit/performance.tsx](../../../apps/web/src/kit/performance.tsx) |
| `ts_export` | `export function PlaneRouteAction({ destination, children, }: Readonly<{ destination: "/app" \| "/explorer"; children: ReactNode }>)` | [apps/web/src/kit/plane-route-action.tsx](../../../apps/web/src/kit/plane-route-action.tsx) |
| `ts_export` | `export function AddressQrCode({ value, label }: Readonly<{ value: string; label: string }>)` | [apps/web/src/kit/qr-code.tsx](../../../apps/web/src/kit/qr-code.tsx) |
| `ts_export` | `export function SettingsRow(props: SettingsRowProps)` | [apps/web/src/kit/settings.tsx](../../../apps/web/src/kit/settings.tsx) |
| `ts_export` | `export type SettingsRowProps = ListItemProps;` | [apps/web/src/kit/settings.tsx](../../../apps/web/src/kit/settings.tsx) |
| `ts_export` | `export function SettingsSection({ title, children, }: Readonly<{ title: ReactNode; children: ReactNode }>)` | [apps/web/src/kit/settings.tsx](../../../apps/web/src/kit/settings.tsx) |
| `ts_export` | `export function SettingsSegmentedControl(props: SegmentedControlProps)` | [apps/web/src/kit/settings.tsx](../../../apps/web/src/kit/settings.tsx) |
| `ts_export` | `export function SettingsSwitch( props: ComponentProps<typeof Switch> & Readonly<{ label: string }>, )` | [apps/web/src/kit/settings.tsx](../../../apps/web/src/kit/settings.tsx) |
| `ts_export` | `export function SettingsTextInput(props: InputProps)` | [apps/web/src/kit/settings.tsx](../../../apps/web/src/kit/settings.tsx) |
| `ts_export` | `export function InlineNotice({ children, tone = "neutral", role = "status", }: Readonly<{ children: ReactNode; tone?: StateTone; role?: "alert" \| "status" }>)` | [apps/web/src/kit/surface.tsx](../../../apps/web/src/kit/surface.tsx) |
| `ts_export` | `export function ScreenCard({ title, description, children, landmark = "main", dataApplication, }: Readonly<{ title: ReactNode; description?: ReactNode; children?: ReactNode; landmark?: "main" \| "section"; dataApplication?: string;` | [apps/web/src/kit/surface.tsx](../../../apps/web/src/kit/surface.tsx) |
| `ts_export` | `export function StateEmpty({ title, description, action, }: Readonly<{ title: ReactNode; description?: ReactNode; action?: ReactNode }>)` | [apps/web/src/kit/surface.tsx](../../../apps/web/src/kit/surface.tsx) |
| `ts_export` | `export function StateFrame({ title, description, children, tone = "neutral", busy = false, role, }: Readonly<{ title: ReactNode; description?: ReactNode; children?: ReactNode; tone?: StateTone;` | [apps/web/src/kit/surface.tsx](../../../apps/web/src/kit/surface.tsx) |
| `ts_export` | `export function StateSkeleton({ rows = 3 }: Readonly<{ rows?: number }>)` | [apps/web/src/kit/surface.tsx](../../../apps/web/src/kit/surface.tsx) |
| `ts_export` | `export type StateTone = "neutral" \| "warning" \| "danger" \| "success";` | [apps/web/src/kit/surface.tsx](../../../apps/web/src/kit/surface.tsx) |

## perf

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export type BudgetedApplicationRoute = keyof typeof ROUTE_SCRIPT_BUDGETS;` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export function classifyPerformanceRoute(pathname: string): PerformanceRoute \| undefined` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export function isPerformanceRoute(value: string): value is PerformanceRoute` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export function isWebVitalName(value: string): value is WebVitalName` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export function normalizedBudget(name: WebVitalName): number` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export function normalizeWebVital(name: WebVitalName, observed: number): number \| undefined` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export function percentile75(values: readonly number[]): number` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export function perf()` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export const PERFORMANCE_ROUTES = ["root", "explorer", "app", "bridge", "exchange", "launchpad"] as const;` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export const PERFORMANCE_SAMPLE_COUNT = 4;` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export type PerformanceRoute = (typeof PERFORMANCE_ROUTES)[number];` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export const ROUTE_SCRIPT_BUDGETS = Object.freeze(` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export const RUM_SERIES_CAPACITY = 256;` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export const THREE_G_PROFILE = Object.freeze(` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export const WEB_VITAL_BUDGETS = Object.freeze(` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export const WEB_VITAL_NAMES = ["LCP", "INP", "CLS"] as const;` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export type WebVitalName = (typeof WEB_VITAL_NAMES)[number];` | [apps/web/src/perf/budgets.ts](../../../apps/web/src/perf/budgets.ts) |
| `ts_export` | `export function parseRedactedWebVital(value: unknown): RedactedWebVital \| undefined` | [apps/web/src/perf/rum-store.ts](../../../apps/web/src/perf/rum-store.ts) |
| `ts_export` | `export function recordWebVital(observation: RedactedWebVital): void` | [apps/web/src/perf/rum-store.ts](../../../apps/web/src/perf/rum-store.ts) |
| `ts_export` | `export interface RedactedWebVital` | [apps/web/src/perf/rum-store.ts](../../../apps/web/src/perf/rum-store.ts) |
| `ts_export` | `export interface RumAggregate` | [apps/web/src/perf/rum-store.ts](../../../apps/web/src/perf/rum-store.ts) |
| `ts_export` | `export function rumSnapshot(): readonly RumAggregate[]` | [apps/web/src/perf/rum-store.ts](../../../apps/web/src/perf/rum-store.ts) |
| `ts_export` | `export function WebVitalsRum()` | [apps/web/src/perf/web-vitals-rum.tsx](../../../apps/web/src/perf/web-vitals-rum.tsx) |

## proxy

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function proxy(request: NextRequest)` | [apps/web/src/proxy.ts](../../../apps/web/src/proxy.ts) |

## routes

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/api/performance/vitals/route.ts](../../../apps/web/src/app/api/performance/vitals/route.ts) |
| `config_key` | `export const runtime = "nodejs";` | [apps/web/src/app/api/performance/vitals/route.ts](../../../apps/web/src/app/api/performance/vitals/route.ts) |
| `config_key` | `export const runtime = "nodejs";` | [apps/web/src/app/api/support/reports/route.ts](../../../apps/web/src/app/api/support/reports/route.ts) |
| `config_key` | `export const runtime = "nodejs";` | [apps/web/src/app/api/support/reports/[traceId]/route.ts](../../../apps/web/src/app/api/support/reports/[traceId]/route.ts) |
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/app/agents/[agentId]/page.tsx](../../../apps/web/src/app/app/agents/[agentId]/page.tsx) |
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/app/agents/new/page.tsx](../../../apps/web/src/app/app/agents/new/page.tsx) |
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/app/agents/page.tsx](../../../apps/web/src/app/app/agents/page.tsx) |
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/app/layout.tsx](../../../apps/web/src/app/app/layout.tsx) |
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/app/support/page.tsx](../../../apps/web/src/app/app/support/page.tsx) |
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/bridge/page.tsx](../../../apps/web/src/app/bridge/page.tsx) |
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/exchange/page.tsx](../../../apps/web/src/app/exchange/page.tsx) |
| `config_key` | `export const dynamic = "force-static";` | [apps/web/src/app/explorer/layout.tsx](../../../apps/web/src/app/explorer/layout.tsx) |
| `config_key` | `export const revalidate = 60;` | [apps/web/src/app/explorer/layout.tsx](../../../apps/web/src/app/explorer/layout.tsx) |
| `config_key` | `export const dynamic = "force-dynamic";` | [apps/web/src/app/launchpad/page.tsx](../../../apps/web/src/app/launchpad/page.tsx) |
| `config_key` | `export const config = { matcher: ["/app/:path*"], };` | [apps/web/src/proxy.ts](../../../apps/web/src/proxy.ts) |
| source | — | [apps/web/src/app/page.tsx](../../../apps/web/src/app/page.tsx) |
| source | — | [apps/web/src/app/app/page.tsx](../../../apps/web/src/app/app/page.tsx) |
| source | — | [apps/web/src/app/app/activity/page.tsx](../../../apps/web/src/app/app/activity/page.tsx) |
| source | — | [apps/web/src/app/app/activity/[entryId]/page.tsx](../../../apps/web/src/app/app/activity/[entryId]/page.tsx) |
| source | — | [apps/web/src/app/app/approvals/page.tsx](../../../apps/web/src/app/app/approvals/page.tsx) |
| source | — | [apps/web/src/app/app/approvals/[approvalId]/page.tsx](../../../apps/web/src/app/app/approvals/[approvalId]/page.tsx) |
| source | — | [apps/web/src/app/app/move/page.tsx](../../../apps/web/src/app/app/move/page.tsx) |
| source | — | [apps/web/src/app/app/notifications/page.tsx](../../../apps/web/src/app/app/notifications/page.tsx) |
| source | — | [apps/web/src/app/app/security/devices/page.tsx](../../../apps/web/src/app/app/security/devices/page.tsx) |
| source | — | [apps/web/src/app/app/security/recovery/page.tsx](../../../apps/web/src/app/app/security/recovery/page.tsx) |
| source | — | [apps/web/src/app/app/settings/page.tsx](../../../apps/web/src/app/app/settings/page.tsx) |
| source | — | [apps/web/src/app/app/settings/devices/page.tsx](../../../apps/web/src/app/app/settings/devices/page.tsx) |
| source | — | [apps/web/src/app/app/settings/security/page.tsx](../../../apps/web/src/app/app/settings/security/page.tsx) |
| source | — | [apps/web/src/app/app/settings/wallet/page.tsx](../../../apps/web/src/app/app/settings/wallet/page.tsx) |
| source | — | [apps/web/src/app/explorer/page.tsx](../../../apps/web/src/app/explorer/page.tsx) |
| source | — | [apps/web/src/app/explorer/accounts/[accountId]/page.tsx](../../../apps/web/src/app/explorer/accounts/[accountId]/page.tsx) |
| source | — | [apps/web/src/app/explorer/lookup/route.ts](../../../apps/web/src/app/explorer/lookup/route.ts) |
| source | — | [apps/web/src/app/explorer/programs/[programId]/page.tsx](../../../apps/web/src/app/explorer/programs/[programId]/page.tsx) |
| source | — | [apps/web/src/app/explorer/receipts/[receiptId]/page.tsx](../../../apps/web/src/app/explorer/receipts/[receiptId]/page.tsx) |

## security

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function appContentSecurityPolicy(nonce: string, development: boolean): string` | [apps/web/src/security/csp.ts](../../../apps/web/src/security/csp.ts) |
| `ts_export` | `export function requestHasWebOrigin(request: Request, configuredOrigin: string \| undefined): boolean` | [apps/web/src/security/origin.ts](../../../apps/web/src/security/origin.ts) |

## server

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export class SupportReportConfigurationError extends Error {}` | [apps/web/src/server/support-reports.ts](../../../apps/web/src/server/support-reports.ts) |
| `ts_export` | `export class SupportReportConflictError extends Error {}` | [apps/web/src/server/support-reports.ts](../../../apps/web/src/server/support-reports.ts) |
| `ts_export` | `export class SupportReportRepository` | [apps/web/src/server/support-reports.ts](../../../apps/web/src/server/support-reports.ts) |
| `ts_export` | `export function supportReportRepositoryFromEnvironment(): SupportReportRepository` | [apps/web/src/server/support-reports.ts](../../../apps/web/src/server/support-reports.ts) |
| `ts_export` | `export function supportRetrievalAuthorized(request: Request): boolean` | [apps/web/src/server/support-reports.ts](../../../apps/web/src/server/support-reports.ts) |

## settings

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function classEnabled( preferences: NotificationPreferences, channel: NotificationChannel, notificationClass: NotificationClass, ): boolean` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function cloneNotificationPreferences( preferences: NotificationPreferences, ): NotificationPreferences` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function fullySuppressesSecurityClass(preferences: NotificationPreferences): boolean` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function human_web_settings()` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export const NON_SUPPRESSIBLE_NOTIFICATION_CLASSES = new Set<NotificationClass>([` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function normalizedNotificationClasses( channel: ChannelPreference, ): readonly Readonly<` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export const NOTIFICATION_CHANNELS = ["push", "email", "in_app"] as const;` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export type NotificationChannel = (typeof NOTIFICATION_CHANNELS)[number];` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function settings(): Readonly<` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function withChannelEnabled( preferences: NotificationPreferences, channel: NotificationChannel, enabled: boolean, ): NotificationPreferences \| undefined` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function withClassEnabled( preferences: NotificationPreferences, channel: NotificationChannel, notificationClass: NotificationClass, enabled: boolean, ): NotificationPreferences \| undefined` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function withDetailLevel( preferences: NotificationPreferences, detail: NotificationDetailLevel, ): NotificationPreferences` | [apps/web/src/settings/model.ts](../../../apps/web/src/settings/model.ts) |
| `ts_export` | `export function PrivacyModeProvider({ principalScope, children, }: Readonly<{ principalScope: string; children: ReactNode }>)` | [apps/web/src/settings/privacy.tsx](../../../apps/web/src/settings/privacy.tsx) |
| `ts_export` | `export function PrivateFigure({ children, className, }: Readonly<{ children: ReactNode; className?: string }>)` | [apps/web/src/settings/privacy.tsx](../../../apps/web/src/settings/privacy.tsx) |
| `ts_export` | `export function usePrivacyMode(): PrivacyModeContextValue` | [apps/web/src/settings/privacy.tsx](../../../apps/web/src/settings/privacy.tsx) |
| `ts_export` | `export function formatLastActive(timestamp: string): string` | [apps/web/src/settings/security/model.ts](../../../apps/web/src/settings/security/model.ts) |
| `ts_export` | `export function human_web_security()` | [apps/web/src/settings/security/model.ts](../../../apps/web/src/settings/security/model.ts) |
| `ts_export` | `export interface RecoveryPresentation` | [apps/web/src/settings/security/model.ts](../../../apps/web/src/settings/security/model.ts) |
| `ts_export` | `export function recoveryPresentation(journey: Journey): RecoveryPresentation` | [apps/web/src/settings/security/model.ts](../../../apps/web/src/settings/security/model.ts) |
| `ts_export` | `export function secretExpiry(secret: TimedSecret): number` | [apps/web/src/settings/security/model.ts](../../../apps/web/src/settings/security/model.ts) |
| `ts_export` | `export const security = Object.freeze(` | [apps/web/src/settings/security/model.ts](../../../apps/web/src/settings/security/model.ts) |
| `ts_export` | `export async function securityStepUp( client: HumanApiClient, action: SecurityActionKind, targetId?: string, authenticator: PasskeyAuthenticator = browserPasskeyAuthenticator(), ): Promise<StepUpEvidence>` | [apps/web/src/settings/security/model.ts](../../../apps/web/src/settings/security/model.ts) |
| `ts_export` | `export function BackupCodesView({ codes, remaskAt, copyable, onExpired }: BackupCodesViewProps)` | [apps/web/src/settings/security/secret.tsx](../../../apps/web/src/settings/security/secret.tsx) |
| `ts_export` | `export function TimedSecretView({ label, secret, onExpired }: TimedSecretViewProps)` | [apps/web/src/settings/security/secret.tsx](../../../apps/web/src/settings/security/secret.tsx) |
| `ts_export` | `export function SecurityScreen({ client: suppliedClient, authenticator: suppliedAuthenticator }: SecurityScreenProps)` | [apps/web/src/settings/security/security-screen.tsx](../../../apps/web/src/settings/security/security-screen.tsx) |
| `ts_export` | `export function privacyPrincipalScope(verifiedScope: string): string` | [apps/web/src/settings/server.ts](../../../apps/web/src/settings/server.ts) |
| `ts_export` | `export async function loadSettingsSnapshot(client: HumanApiClient): Promise<SettingsSnapshot>` | [apps/web/src/settings/settings-screen.tsx](../../../apps/web/src/settings/settings-screen.tsx) |
| `ts_export` | `export function SettingsScreen({ client: suppliedClient, }: Readonly<{ client?: HumanApiClient }>)` | [apps/web/src/settings/settings-screen.tsx](../../../apps/web/src/settings/settings-screen.tsx) |
| `ts_export` | `export function newestWalletSecurityNotification( notifications: readonly NotificationSummary[], ): NotificationSummary \| undefined` | [apps/web/src/settings/wallet/model.ts](../../../apps/web/src/settings/wallet/model.ts) |
| `ts_export` | `export class WalletFeeController` | [apps/web/src/settings/wallet/model.ts](../../../apps/web/src/settings/wallet/model.ts) |
| `ts_export` | `export interface WalletFeeSnapshot` | [apps/web/src/settings/wallet/model.ts](../../../apps/web/src/settings/wallet/model.ts) |
| `ts_export` | `export function WalletFeeChoice({ snapshot, onChange }: Readonly<{ snapshot: WalletFeeSnapshot; onChange: (currency: WalletFeeSnapshot["currency"], maximum: string) => void; }>)` | [apps/web/src/settings/wallet/wallet-screen.tsx](../../../apps/web/src/settings/wallet/wallet-screen.tsx) |
| `ts_export` | `export function WalletFeeSettings()` | [apps/web/src/settings/wallet/wallet-screen.tsx](../../../apps/web/src/settings/wallet/wallet-screen.tsx) |

## shell

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function AuthenticatedShell({ initialSelection, children, }: Readonly<{ initialSelection: ShellSelection; children: ReactNode }>)` | [apps/web/src/shell/app-shell.tsx](../../../apps/web/src/shell/app-shell.tsx) |
| `ts_export` | `export function useAuthenticatedShell(): ShellSelection` | [apps/web/src/shell/app-shell.tsx](../../../apps/web/src/shell/app-shell.tsx) |
| `ts_export` | `export function useShellSelection(): ShellSelection` | [apps/web/src/shell/app-shell.tsx](../../../apps/web/src/shell/app-shell.tsx) |
| `ts_export` | `export interface DeepLinkResolution` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export type PointerCapability = "coarse" \| "fine" \| "none";` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export type Shell = "mobile" \| "desktop";` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export const SHELL_HINT_COOKIE = "__Host-layerx-shell";` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export interface ShellConfirmation` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export interface ShellHints` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export interface ShellSelection` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export class ShellSelector` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export const TABLET_MIN_WIDTH = 768;` | [apps/web/src/shell/selector.ts](../../../apps/web/src/shell/selector.ts) |
| `ts_export` | `export function selectServerShell(headers: Headers, cookies: RequestCookies): ShellSelection` | [apps/web/src/shell/server.ts](../../../apps/web/src/shell/server.ts) |

## states

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function ApplicationStateBoundary({ children }: Readonly<{ children: ReactNode }>)` | [apps/web/src/states/application.tsx](../../../apps/web/src/states/application.tsx) |
| `ts_export` | `export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState>` | [apps/web/src/states/error.tsx](../../../apps/web/src/states/error.tsx) |
| `ts_export` | `export interface ErrorPresentation` | [apps/web/src/states/error.tsx](../../../apps/web/src/states/error.tsx) |
| `ts_export` | `export function errorPresentation(error: unknown, traceId = generatedTraceId()): ErrorPresentation` | [apps/web/src/states/error.tsx](../../../apps/web/src/states/error.tsx) |
| `ts_export` | `export function ErrorSurface({` | [apps/web/src/states/error.tsx](../../../apps/web/src/states/error.tsx) |
| `ts_export` | `export function RouteErrorBoundary({ error, reset, route, }: Readonly<{ error: Error & { digest?: string }; reset: () => void; route: string }>)` | [apps/web/src/states/error.tsx](../../../apps/web/src/states/error.tsx) |
| `ts_export` | `export const CURRENT_ROUTES = Object.freeze(STATE_ROUTE_MANIFEST.map(([, route]) => route));` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export const CURRENT_SHELLS = ["authenticated"] as const;` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export function human_state_matrix_registry(): StateMatrix` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export type HumanShell = "mobile" \| "desktop";` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export function humanStateMatrix(): StateMatrix` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export const REQUIRED_SCREEN_STATES = [` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export type ScreenState = (typeof REQUIRED_SCREEN_STATES)[number];` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export interface ScreenStateEntry` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export class StateMatrix` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export interface StatePresentation` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export type StateTargetKind = "route" \| "shell";` | [apps/web/src/states/matrix.ts](../../../apps/web/src/states/matrix.ts) |
| `ts_export` | `export function OfflineBanner()` | [apps/web/src/states/offline.tsx](../../../apps/web/src/states/offline.tsx) |
| `ts_export` | `export function QueuedActionNotice({ children }: Readonly<{ children?: ReactNode }>)` | [apps/web/src/states/offline.tsx](../../../apps/web/src/states/offline.tsx) |
| `ts_export` | `export interface MoneyOfflineAction` | [apps/web/src/states/queue.ts](../../../apps/web/src/states/queue.ts) |
| `ts_export` | `export class OfflineActionQueue` | [apps/web/src/states/queue.ts](../../../apps/web/src/states/queue.ts) |
| `ts_export` | `export interface QueueableOfflineAction` | [apps/web/src/states/queue.ts](../../../apps/web/src/states/queue.ts) |
| `ts_export` | `export type QueueDecision = "queued" \| "duplicate" \| "money-rejected";` | [apps/web/src/states/queue.ts](../../../apps/web/src/states/queue.ts) |
| `ts_export` | `export class BrowserSupportReportOutbox implements ErrorReporter` | [apps/web/src/states/report.ts](../../../apps/web/src/states/report.ts) |
| `ts_export` | `export const browserSupportReportOutbox = new BrowserSupportReportOutbox();` | [apps/web/src/states/report.ts](../../../apps/web/src/states/report.ts) |
| `ts_export` | `export interface ErrorReportDraft` | [apps/web/src/states/report.ts](../../../apps/web/src/states/report.ts) |
| `ts_export` | `export interface ErrorReporter` | [apps/web/src/states/report.ts](../../../apps/web/src/states/report.ts) |
| `ts_export` | `export type ErrorReportReceipt = SavedReportReceipt \| Readonly<` | [apps/web/src/states/report.ts](../../../apps/web/src/states/report.ts) |
| `ts_export` | `export function redactErrorReport(draft: ErrorReportDraft): SupportReportRequest` | [apps/web/src/states/report.ts](../../../apps/web/src/states/report.ts) |
| `ts_export` | `export function parseSavedReportReceipt(value: unknown): SavedReportReceipt` | [apps/web/src/states/report-schema.ts](../../../apps/web/src/states/report-schema.ts) |
| `ts_export` | `export function parseStoredSupportReport(value: unknown): StoredSupportReport` | [apps/web/src/states/report-schema.ts](../../../apps/web/src/states/report-schema.ts) |
| `ts_export` | `export function parseSupportReportRequest(value: unknown): SupportReportRequest` | [apps/web/src/states/report-schema.ts](../../../apps/web/src/states/report-schema.ts) |
| `ts_export` | `export type ReportShell = "mobile" \| "desktop";` | [apps/web/src/states/report-schema.ts](../../../apps/web/src/states/report-schema.ts) |
| `ts_export` | `export interface SavedReportReceipt` | [apps/web/src/states/report-schema.ts](../../../apps/web/src/states/report-schema.ts) |
| `ts_export` | `export interface StoredSupportReport extends SupportReportRequest` | [apps/web/src/states/report-schema.ts](../../../apps/web/src/states/report-schema.ts) |
| `ts_export` | `export interface SupportReportRequest` | [apps/web/src/states/report-schema.ts](../../../apps/web/src/states/report-schema.ts) |
| `ts_export` | `export function supportReportTrace(value: string): string` | [apps/web/src/states/report-schema.ts](../../../apps/web/src/states/report-schema.ts) |
| `ts_export` | `export function DegradedSurface({ age, referencePoint, children, onReload, }: Readonly<{ age: string; referencePoint: string; children?: ReactNode; onReload?: () => void; }>)` | [apps/web/src/states/surfaces.tsx](../../../apps/web/src/states/surfaces.tsx) |
| `ts_export` | `export function EmptySurface({ action, actionLabelKey, }: Readonly<{ action?: () => void; actionLabelKey?: string }>)` | [apps/web/src/states/surfaces.tsx](../../../apps/web/src/states/surfaces.tsx) |
| `ts_export` | `export function LoadingSurface({ rows = 3 }: Readonly<{ rows?: number }>)` | [apps/web/src/states/surfaces.tsx](../../../apps/web/src/states/surfaces.tsx) |
| `ts_export` | `export function OfflineSurface({ onRetry }: Readonly<{ onRetry: () => void }>)` | [apps/web/src/states/surfaces.tsx](../../../apps/web/src/states/surfaces.tsx) |
| `ts_export` | `export function StillCheckingSurface({ lookupOutcome, onResolved, pollIntervalMs = 3_000, children, }: StillCheckingSurfaceProps)` | [apps/web/src/states/surfaces.tsx](../../../apps/web/src/states/surfaces.tsx) |
| `ts_export` | `export interface StillCheckingSurfaceProps` | [apps/web/src/states/surfaces.tsx](../../../apps/web/src/states/surfaces.tsx) |

## support

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function DesktopSupportChat({ traceId }: Readonly<{ traceId?: string }>)` | [apps/web/src/support/chat.tsx](../../../apps/web/src/support/chat.tsx) |
| `ts_export` | `export function MobileSupportChat({ traceId }: Readonly<{ traceId?: string }>)` | [apps/web/src/support/chat.tsx](../../../apps/web/src/support/chat.tsx) |
| `ts_export` | `export function SupportChat({ platform, traceId, }: Readonly<{ platform: "mobile" \| "desktop"; traceId?: TraceId }>)` | [apps/web/src/support/chat.tsx](../../../apps/web/src/support/chat.tsx) |
| `ts_export` | `export type SupportChatConnection = "online" \| "offline";` | [apps/web/src/support/conversation.ts](../../../apps/web/src/support/conversation.ts) |
| `ts_export` | `export type SupportChatDelivery = "sending" \| "sent" \| "failed";` | [apps/web/src/support/conversation.ts](../../../apps/web/src/support/conversation.ts) |
| `ts_export` | `export interface SupportChatEntry extends SupportMessage` | [apps/web/src/support/conversation.ts](../../../apps/web/src/support/conversation.ts) |
| `ts_export` | `export type SupportChatLoad = "loading" \| "ready" \| "failed";` | [apps/web/src/support/conversation.ts](../../../apps/web/src/support/conversation.ts) |
| `ts_export` | `export class SupportChatSession` | [apps/web/src/support/conversation.ts](../../../apps/web/src/support/conversation.ts) |
| `ts_export` | `export interface SupportChatSessionOptions` | [apps/web/src/support/conversation.ts](../../../apps/web/src/support/conversation.ts) |
| `ts_export` | `export interface SupportChatSnapshot` | [apps/web/src/support/conversation.ts](../../../apps/web/src/support/conversation.ts) |
| `ts_export` | `export type SupportFeedbackDelivery = "idle" \| "sending" \| "saved" \| "failed";` | [apps/web/src/support/conversation.ts](../../../apps/web/src/support/conversation.ts) |
| `ts_export` | `export function human_web_support()` | [apps/web/src/support/model.ts](../../../apps/web/src/support/model.ts) |
| `ts_export` | `export function support(): Readonly<` | [apps/web/src/support/model.ts](../../../apps/web/src/support/model.ts) |
| `ts_export` | `export const SUPPORT_TOPICS: readonly SupportTopic[] = Object.freeze([` | [apps/web/src/support/topics.ts](../../../apps/web/src/support/topics.ts) |
| `ts_export` | `export interface SupportTopic` | [apps/web/src/support/topics.ts](../../../apps/web/src/support/topics.ts) |
| `ts_export` | `export function supportTopic(id: SupportTopicId): SupportTopic` | [apps/web/src/support/topics.ts](../../../apps/web/src/support/topics.ts) |
