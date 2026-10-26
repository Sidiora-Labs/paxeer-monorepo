# Web client contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-api-abi-read-abireadfield</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:13</code> | <code>export type AbiReadField = Readonly&lt;{ name: string; type: AbiReadType }&gt;;</code> |
| <code>interface.14.ts-web-api-abi-read-abireadrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:17</code> | <code>export interface AbiReadRecord</code> |
| <code>interface.14.ts-web-api-abi-read-abireadscalar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:5</code> | <code>export type AbiReadScalar = "address" &#124; "bool" &#124; "bytes" &#124; "bytes32" &#124; "string" &#124; AbiReadUint;</code> |
| <code>interface.14.ts-web-api-abi-read-abireadtype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:7</code> | <code>export type AbiReadType =</code> |
| <code>interface.14.ts-web-api-abi-read-abireaduint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:3</code> | <code>export type AbiReadUint = "uint8" &#124; "uint32" &#124; "uint64" &#124; "uint256";</code> |
| <code>interface.14.ts-web-api-abi-read-abireadvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:15</code> | <code>export type AbiReadValue = bigint &#124; boolean &#124; string &#124; readonly AbiReadValue[] &#124; AbiReadRecord;</code> |
| <code>interface.14.ts-web-api-abi-read-decodeabiresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:196</code> | <code>export function decodeAbiResult(returnData: string, outputs: readonly AbiReadField[]): AbiReadRecord</code> |
| <code>interface.14.ts-web-api-abi-read-readflag</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:214</code> | <code>export function readFlag(value: AbiReadValue &#124; undefined, label: string): boolean</code> |
| <code>interface.14.ts-web-api-abi-read-readlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:221</code> | <code>export function readList(value: AbiReadValue &#124; undefined, label: string): readonly AbiReadValue[]</code> |
| <code>interface.14.ts-web-api-abi-read-readrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:228</code> | <code>export function readRecord(value: AbiReadValue &#124; undefined, label: string): AbiReadRecord</code> |
| <code>interface.14.ts-web-api-abi-read-readtext</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:207</code> | <code>export function readText(value: AbiReadValue &#124; undefined, label: string): string</code> |
| <code>interface.14.ts-web-api-abi-read-readuint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/abi-read.ts:200</code> | <code>export function readUint(value: AbiReadValue &#124; undefined, label: string): bigint</code> |
| <code>interface.14.ts-web-api-gas-station-accountdidoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gas-station.ts:46</code> | <code>export type AccountDidOutcome =</code> |
| <code>interface.14.ts-web-api-gas-station-gasquoteoutcome</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/gas-station.ts:21</code> | <code>export type GasQuoteOutcome =</code> |
| <code>interface.14.ts-web-api-gas-station-requestsidioragasquote</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/gas-station.ts:62</code> | <code>export async function requestSidioraGasQuote(request: GasQuoteRequest, chainId: bigint): Promise&lt;GasQuoteOutcome&gt;</code> |
| <code>interface.14.ts-web-api-gas-station-sendsponsoredbatch</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/gas-station.ts:81</code> | <code>export async function sendSponsoredBatch( quoted: SidioraGasQuote, accountSignature: string, authorizationNonce: bigint, authorizationSignature: string, ): Promise&lt;PrecompileSendOutcome&gt;</code> |
| <code>interface.14.ts-web-api-gas-station-sidioraaccountdid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gas-station.ts:50</code> | <code>export async function sidioraAccountDid(account: string): Promise&lt;AccountDidOutcome&gt;</code> |
| <code>interface.14.ts-web-api-gas-station-sidioragasquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gas-station.ts:15</code> | <code>export interface SidioraGasQuote</code> |
| <code>interface.14.ts-web-api-gas-station-sidiorastationmetadata</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gas-station.ts:40</code> | <code>export async function sidioraStationMetadata(): Promise&lt;Readonly&lt;</code> |
| <code>interface.14.ts-web-api-gateway-ethblocknumber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:125</code> | <code>export async function ethBlockNumber(): Promise&lt;bigint&gt;</code> |
| <code>interface.14.ts-web-api-gateway-ethcall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:116</code> | <code>export async function ethCall(call: Readonly&lt;{ to: string; data: string; from?: string }&gt;): Promise&lt;string&gt;</code> |
| <code>interface.14.ts-web-api-gateway-ethgetlogs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:139</code> | <code>export async function ethGetLogs( filter: Readonly&lt;{ address: string; topics: readonly (string &#124; null)[]; fromBlock: bigint }&gt;, ): Promise&lt;readonly GatewayLog[]&gt;</code> |
| <code>interface.14.ts-web-api-gateway-ethtransactionreceipt-j53</code> | <code>ts_export</code> | <code>new</code> | <code>apps/web/src/api/gateway.ts</code> (planned) | <code>export async function ethTransactionReceipt(hash: string): Promise&lt;GatewayReceipt &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-api-gateway-forksurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:186</code> | <code>export type ForkSurface = "exchange" &#124; "bridge" &#124; "launchpad";</code> |
| <code>interface.14.ts-web-api-gateway-gatewaylog</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:133</code> | <code>export interface GatewayLog extends PrecompileLog</code> |
| <code>interface.14.ts-web-api-gateway-gatewayrpcerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:28</code> | <code>export class GatewayRpcError extends Error</code> |
| <code>interface.14.ts-web-api-gateway-gatewayunavailableerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:21</code> | <code>export class GatewayUnavailableError extends Error</code> |
| <code>interface.14.ts-web-api-gateway-history-page-limit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:19</code> | <code>export const HISTORY_PAGE_LIMIT = 25;</code> |
| <code>interface.14.ts-web-api-gateway-historychain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:265</code> | <code>export type HistoryChain = "layerx" &#124; "paxeer";</code> |
| <code>interface.14.ts-web-api-gateway-historyoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:294</code> | <code>export interface HistoryOptions</code> |
| <code>interface.14.ts-web-api-gateway-historypage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:287</code> | <code>export interface HistoryPage</code> |
| <code>interface.14.ts-web-api-gateway-historyrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:267</code> | <code>export interface HistoryRow</code> |
| <code>interface.14.ts-web-api-gateway-networkcapabilities</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:191</code> | <code>export interface NetworkCapabilities</code> |
| <code>interface.14.ts-web-api-gateway-paxeerhistory</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:395</code> | <code>export async function paxeerHistory(account: string, options: HistoryOptions = {}): Promise&lt;HistoryPage&gt;</code> |
| <code>interface.14.ts-web-api-gateway-precompileview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:239</code> | <code>export type PrecompileView = Readonly&lt;Pick&lt;PrecompileCall, "to" &#124; "data"&gt;&gt;;</code> |
| <code>interface.14.ts-web-api-gateway-pxgetcapabilities</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:200</code> | <code>export async function pxGetCapabilities(): Promise&lt;NetworkCapabilities&gt;</code> |
| <code>interface.14.ts-web-api-gateway-pxresolveaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:252</code> | <code>export async function pxResolveAccount(account: string): Promise&lt;ResolvedAccount&gt;</code> |
| <code>interface.14.ts-web-api-gateway-resolvedaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:241</code> | <code>export interface ResolvedAccount</code> |
| <code>interface.14.ts-web-api-gateway-surface-unavailable-code</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:189</code> | <code>export const SURFACE_UNAVAILABLE_CODE = -32003;</code> |
| <code>interface.14.ts-web-api-gateway-surfacecapability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:224</code> | <code>export type SurfaceCapability = Readonly&lt;{ live: boolean; detail: string &#124; null }&gt;;</code> |
| <code>interface.14.ts-web-api-gateway-surfacecapability-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:227</code> | <code>export async function surfaceCapability(surface: ForkSurface): Promise&lt;SurfaceCapability&gt;</code> |
| <code>interface.14.ts-web-api-gateway-unifiedhistory</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:404</code> | <code>export async function unifiedHistory(account: string, options: HistoryOptions = {}): Promise&lt;HistoryPage&gt;</code> |
| <code>interface.14.ts-web-api-generated-conformance-conformance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/conformance.ts:150</code> | <code>export const conformance: { readonly [name in OperationName]: (run: ConformanceRun) =&gt; Promise&lt;JsonValue&gt; } =</code> |
| <code>interface.14.ts-web-api-generated-conformance-conformancerun</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/conformance.ts:121</code> | <code>export interface ConformanceRun</code> |
| <code>interface.14.ts-web-api-generated-conformance-runbody</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/conformance.ts:136</code> | <code>export function runBody(run: ConformanceRun): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-conformance-runkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/conformance.ts:143</code> | <code>export function runKey(run: ConformanceRun): string</code> |
| <code>interface.14.ts-web-api-generated-conformance-runparam</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/conformance.ts:128</code> | <code>export function runParam(run: ConformanceRun, name: string): string</code> |
| <code>interface.14.ts-web-api-generated-index-accountbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:669</code> | <code>export interface AccountBalance</code> |
| <code>interface.14.ts-web-api-generated-index-accountcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:700</code> | <code>export interface AccountCreateRequest</code> |
| <code>interface.14.ts-web-api-generated-index-accountcreation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:722</code> | <code>export interface AccountCreation</code> |
| <code>interface.14.ts-web-api-generated-index-accountid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:91</code> | <code>export type AccountId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-activityentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:744</code> | <code>export interface ActivityEntry</code> |
| <code>interface.14.ts-web-api-generated-index-activityentrydetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:813</code> | <code>export interface ActivityEntryDetail</code> |
| <code>interface.14.ts-web-api-generated-index-activityentryid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:93</code> | <code>export type ActivityEntryId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-activityentrykind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:165</code> | <code>export type ActivityEntryKind = "deposit" &#124; "withdrawal" &#124; "movement" &#124; "agent-action" &#124; "approval" &#124; "security-event";</code> |
| <code>interface.14.ts-web-api-generated-index-activityentrykindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:167</code> | <code>export const activityEntryKindVariants: readonly ActivityEntryKind[] = ["deposit", "withdrawal", "movement", "agent-action", "approval", "security-event"];</code> |
| <code>interface.14.ts-web-api-generated-index-activityfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:895</code> | <code>export interface ActivityFilter</code> |
| <code>interface.14.ts-web-api-generated-index-activitygroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:937</code> | <code>export interface ActivityGroup</code> |
| <code>interface.14.ts-web-api-generated-index-activitypage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:965</code> | <code>export interface ActivityPage</code> |
| <code>interface.14.ts-web-api-generated-index-activityqueryrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:990</code> | <code>export interface ActivityQueryRequest</code> |
| <code>interface.14.ts-web-api-generated-index-agent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1025</code> | <code>export interface Agent</code> |
| <code>interface.14.ts-web-api-generated-index-agentarchiverequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1078</code> | <code>export interface AgentArchiveRequest</code> |
| <code>interface.14.ts-web-api-generated-index-agentcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1097</code> | <code>export interface AgentCreateRequest</code> |
| <code>interface.14.ts-web-api-generated-index-agentid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:95</code> | <code>export type AgentId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-agentlimitrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1129</code> | <code>export interface AgentLimitRequest</code> |
| <code>interface.14.ts-web-api-generated-index-agentpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1148</code> | <code>export interface AgentPage</code> |
| <code>interface.14.ts-web-api-generated-index-agentreclaimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1170</code> | <code>export interface AgentReclaimRequest</code> |
| <code>interface.14.ts-web-api-generated-index-agentspend</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1189</code> | <code>export interface AgentSpend</code> |
| <code>interface.14.ts-web-api-generated-index-agentstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:179</code> | <code>export type AgentState = "creating" &#124; "active" &#124; "paused" &#124; "archiving" &#124; "archived";</code> |
| <code>interface.14.ts-web-api-generated-index-agentstatevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:181</code> | <code>export const agentStateVariants: readonly AgentState[] = ["creating", "active", "paused", "archiving", "archived"];</code> |
| <code>interface.14.ts-web-api-generated-index-amount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:97</code> | <code>export type Amount = bigint;</code> |
| <code>interface.14.ts-web-api-generated-index-apierror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1227</code> | <code>export interface ApiError</code> |
| <code>interface.14.ts-web-api-generated-index-approvalapproverequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1266</code> | <code>export interface ApprovalApproveRequest</code> |
| <code>interface.14.ts-web-api-generated-index-approvaldecision</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1285</code> | <code>export interface ApprovalDecision</code> |
| <code>interface.14.ts-web-api-generated-index-approvaldetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1319</code> | <code>export interface ApprovalDetail</code> |
| <code>interface.14.ts-web-api-generated-index-approvalfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1365</code> | <code>export interface ApprovalFacts</code> |
| <code>interface.14.ts-web-api-generated-index-approvalid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:99</code> | <code>export type ApprovalId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-approvalpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1396</code> | <code>export interface ApprovalPage</code> |
| <code>interface.14.ts-web-api-generated-index-approvalstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:193</code> | <code>export type ApprovalState = "pending" &#124; "approved" &#124; "rejected" &#124; "expired" &#124; "defective";</code> |
| <code>interface.14.ts-web-api-generated-index-approvalstatevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:195</code> | <code>export const approvalStateVariants: readonly ApprovalState[] = ["pending", "approved", "rejected", "expired", "defective"];</code> |
| <code>interface.14.ts-web-api-generated-index-approvalsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1418</code> | <code>export interface ApprovalSummary</code> |
| <code>interface.14.ts-web-api-generated-index-assertionid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:101</code> | <code>export type AssertionId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatordisable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1461</code> | <code>export interface AuthenticatorDisable</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatorid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:103</code> | <code>export type AuthenticatorId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatormethod</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1480</code> | <code>export interface AuthenticatorMethod</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1512</code> | <code>export interface AuthenticatorSetupBegin</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1534</code> | <code>export interface AuthenticatorSetupChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1562</code> | <code>export interface AuthenticatorSetupFinish</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:105</code> | <code>export type AuthenticatorSetupId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1584</code> | <code>export interface AuthenticatorSetupResult</code> |
| <code>interface.14.ts-web-api-generated-index-authenticatorstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1606</code> | <code>export interface AuthenticatorStatus</code> |
| <code>interface.14.ts-web-api-generated-index-backupcoderotation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1628</code> | <code>export interface BackupCodeRotation</code> |
| <code>interface.14.ts-web-api-generated-index-backupcodeset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1647</code> | <code>export interface BackupCodeSet</code> |
| <code>interface.14.ts-web-api-generated-index-bindingrebindaction</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:1672</code> | <code>export interface BindingRebindAction</code> |
| <code>interface.14.ts-web-api-generated-index-bindingstate</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:207</code> | <code>export type BindingState = "none" &#124; "binding" &#124; "bound" &#124; "rebinding";</code> |
| <code>interface.14.ts-web-api-generated-index-bindingstatement</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:1694</code> | <code>export interface BindingStatement</code> |
| <code>interface.14.ts-web-api-generated-index-bindingstatementrequest</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:1719</code> | <code>export interface BindingStatementRequest</code> |
| <code>interface.14.ts-web-api-generated-index-bindingstatevariants</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:209</code> | <code>export const bindingStateVariants: readonly BindingState[] = ["none", "binding", "bound", "rebinding"];</code> |
| <code>interface.14.ts-web-api-generated-index-bindingsubmission</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:1738</code> | <code>export interface BindingSubmission</code> |
| <code>interface.14.ts-web-api-generated-index-channelpreference</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1763</code> | <code>export interface ChannelPreference</code> |
| <code>interface.14.ts-web-api-generated-index-classtoggle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1785</code> | <code>export interface ClassToggle</code> |
| <code>interface.14.ts-web-api-generated-index-copykey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:107</code> | <code>export type CopyKey = string;</code> |
| <code>interface.14.ts-web-api-generated-index-createhumanapiclient</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5662</code> | <code>export function createHumanApiClient(options: HumanApiClientOptions = {}): HumanApiClient</code> |
| <code>interface.14.ts-web-api-generated-index-currencycode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:109</code> | <code>export type CurrencyCode = string;</code> |
| <code>interface.14.ts-web-api-generated-index-cursor</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:111</code> | <code>export type Cursor = string;</code> |
| <code>interface.14.ts-web-api-generated-index-decodeaccountbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:677</code> | <code>export function decodeAccountBalance(value: JsonValue &#124; undefined, at: string): AccountBalance</code> |
| <code>interface.14.ts-web-api-generated-index-decodeaccountcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:705</code> | <code>export function decodeAccountCreateRequest(value: JsonValue &#124; undefined, at: string): AccountCreateRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeaccountcreation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:727</code> | <code>export function decodeAccountCreation(value: JsonValue &#124; undefined, at: string): AccountCreation</code> |
| <code>interface.14.ts-web-api-generated-index-decodeactivityentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:758</code> | <code>export function decodeActivityEntry(value: JsonValue &#124; undefined, at: string): ActivityEntry</code> |
| <code>interface.14.ts-web-api-generated-index-decodeactivityentrydetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:830</code> | <code>export function decodeActivityEntryDetail(value: JsonValue &#124; undefined, at: string): ActivityEntryDetail</code> |
| <code>interface.14.ts-web-api-generated-index-decodeactivityentrykind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:169</code> | <code>export function decodeActivityEntryKind(value: JsonValue &#124; undefined, at: string): ActivityEntryKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodeactivityfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:902</code> | <code>export function decodeActivityFilter(value: JsonValue &#124; undefined, at: string): ActivityFilter</code> |
| <code>interface.14.ts-web-api-generated-index-decodeactivitygroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:944</code> | <code>export function decodeActivityGroup(value: JsonValue &#124; undefined, at: string): ActivityGroup</code> |
| <code>interface.14.ts-web-api-generated-index-decodeactivitypage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:971</code> | <code>export function decodeActivityPage(value: JsonValue &#124; undefined, at: string): ActivityPage</code> |
| <code>interface.14.ts-web-api-generated-index-decodeactivityqueryrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:996</code> | <code>export function decodeActivityQueryRequest(value: JsonValue &#124; undefined, at: string): ActivityQueryRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeagent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1039</code> | <code>export function decodeAgent(value: JsonValue &#124; undefined, at: string): Agent</code> |
| <code>interface.14.ts-web-api-generated-index-decodeagentarchiverequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1082</code> | <code>export function decodeAgentArchiveRequest(value: JsonValue &#124; undefined, at: string): AgentArchiveRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeagentcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1104</code> | <code>export function decodeAgentCreateRequest(value: JsonValue &#124; undefined, at: string): AgentCreateRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeagentlimitrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1133</code> | <code>export function decodeAgentLimitRequest(value: JsonValue &#124; undefined, at: string): AgentLimitRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeagentpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1153</code> | <code>export function decodeAgentPage(value: JsonValue &#124; undefined, at: string): AgentPage</code> |
| <code>interface.14.ts-web-api-generated-index-decodeagentreclaimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1174</code> | <code>export function decodeAgentReclaimRequest(value: JsonValue &#124; undefined, at: string): AgentReclaimRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeagentspend</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1198</code> | <code>export function decodeAgentSpend(value: JsonValue &#124; undefined, at: string): AgentSpend</code> |
| <code>interface.14.ts-web-api-generated-index-decodeagentstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:183</code> | <code>export function decodeAgentState(value: JsonValue &#124; undefined, at: string): AgentState</code> |
| <code>interface.14.ts-web-api-generated-index-decodeapierror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1235</code> | <code>export function decodeApiError(value: JsonValue &#124; undefined, at: string): ApiError</code> |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalapproverequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1270</code> | <code>export function decodeApprovalApproveRequest(value: JsonValue &#124; undefined, at: string): ApprovalApproveRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeapprovaldecision</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1294</code> | <code>export function decodeApprovalDecision(value: JsonValue &#124; undefined, at: string): ApprovalDecision</code> |
| <code>interface.14.ts-web-api-generated-index-decodeapprovaldetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1332</code> | <code>export function decodeApprovalDetail(value: JsonValue &#124; undefined, at: string): ApprovalDetail</code> |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1373</code> | <code>export function decodeApprovalFacts(value: JsonValue &#124; undefined, at: string): ApprovalFacts</code> |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1401</code> | <code>export function decodeApprovalPage(value: JsonValue &#124; undefined, at: string): ApprovalPage</code> |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:197</code> | <code>export function decodeApprovalState(value: JsonValue &#124; undefined, at: string): ApprovalState</code> |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1430</code> | <code>export function decodeApprovalSummary(value: JsonValue &#124; undefined, at: string): ApprovalSummary</code> |
| <code>interface.14.ts-web-api-generated-index-decodearray</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:66</code> | <code>export function decodeArray&lt;Item&gt;( value: JsonValue &#124; undefined, at: string, item: (value: JsonValue &#124; undefined, at: string) =&gt; Item, ): Item[]</code> |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatordisable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1465</code> | <code>export function decodeAuthenticatorDisable(value: JsonValue &#124; undefined, at: string): AuthenticatorDisable</code> |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatormethod</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1487</code> | <code>export function decodeAuthenticatorMethod(value: JsonValue &#124; undefined, at: string): AuthenticatorMethod</code> |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatorsetupbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1517</code> | <code>export function decodeAuthenticatorSetupBegin(value: JsonValue &#124; undefined, at: string): AuthenticatorSetupBegin</code> |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatorsetupchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1541</code> | <code>export function decodeAuthenticatorSetupChallenge(value: JsonValue &#124; undefined, at: string): AuthenticatorSetupChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatorsetupfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1567</code> | <code>export function decodeAuthenticatorSetupFinish(value: JsonValue &#124; undefined, at: string): AuthenticatorSetupFinish</code> |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatorsetupresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1589</code> | <code>export function decodeAuthenticatorSetupResult(value: JsonValue &#124; undefined, at: string): AuthenticatorSetupResult</code> |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatorstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1611</code> | <code>export function decodeAuthenticatorStatus(value: JsonValue &#124; undefined, at: string): AuthenticatorStatus</code> |
| <code>interface.14.ts-web-api-generated-index-decodebackupcoderotation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1632</code> | <code>export function decodeBackupCodeRotation(value: JsonValue &#124; undefined, at: string): BackupCodeRotation</code> |
| <code>interface.14.ts-web-api-generated-index-decodebackupcodeset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1653</code> | <code>export function decodeBackupCodeSet(value: JsonValue &#124; undefined, at: string): BackupCodeSet</code> |
| <code>interface.14.ts-web-api-generated-index-decodebindingrebindaction</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:1677</code> | <code>export function decodeBindingRebindAction(value: JsonValue &#124; undefined, at: string): BindingRebindAction</code> |
| <code>interface.14.ts-web-api-generated-index-decodebindingstate</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:211</code> | <code>export function decodeBindingState(value: JsonValue &#124; undefined, at: string): BindingState</code> |
| <code>interface.14.ts-web-api-generated-index-decodebindingstatement</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:1700</code> | <code>export function decodeBindingStatement(value: JsonValue &#124; undefined, at: string): BindingStatement</code> |
| <code>interface.14.ts-web-api-generated-index-decodebindingstatementrequest</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:1723</code> | <code>export function decodeBindingStatementRequest(value: JsonValue &#124; undefined, at: string): BindingStatementRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodebindingsubmission</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:1744</code> | <code>export function decodeBindingSubmission(value: JsonValue &#124; undefined, at: string): BindingSubmission</code> |
| <code>interface.14.ts-web-api-generated-index-decodechannelpreference</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1768</code> | <code>export function decodeChannelPreference(value: JsonValue &#124; undefined, at: string): ChannelPreference</code> |
| <code>interface.14.ts-web-api-generated-index-decodeclasstoggle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1790</code> | <code>export function decodeClassToggle(value: JsonValue &#124; undefined, at: string): ClassToggle</code> |
| <code>interface.14.ts-web-api-generated-index-decodeconsensusinteger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:76</code> | <code>export function decodeConsensusInteger(value: JsonValue &#124; undefined, at: string): bigint</code> |
| <code>interface.14.ts-web-api-generated-index-decodedepositconfirmrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1812</code> | <code>export function decodeDepositConfirmRequest(value: JsonValue &#124; undefined, at: string): DepositConfirmRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodedepositstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1838</code> | <code>export function decodeDepositStartRequest(value: JsonValue &#124; undefined, at: string): DepositStartRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodedevice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1865</code> | <code>export function decodeDevice(value: JsonValue &#124; undefined, at: string): Device</code> |
| <code>interface.14.ts-web-api-generated-index-decodeempty</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1886</code> | <code>export function decodeEmpty(value: JsonValue &#124; undefined, at: string): Empty</code> |
| <code>interface.14.ts-web-api-generated-index-decodeerrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:225</code> | <code>export function decodeErrorCode(value: JsonValue &#124; undefined, at: string): ErrorCode</code> |
| <code>interface.14.ts-web-api-generated-index-decodeevidenceclass</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:239</code> | <code>export function decodeEvidenceClass(value: JsonValue &#124; undefined, at: string): EvidenceClass</code> |
| <code>interface.14.ts-web-api-generated-index-decodeevidencematerial</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1904</code> | <code>export function decodeEvidenceMaterial(value: JsonValue &#124; undefined, at: string): EvidenceMaterial</code> |
| <code>interface.14.ts-web-api-generated-index-decodeevidenceref</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1940</code> | <code>export function decodeEvidenceRef(value: JsonValue &#124; undefined, at: string): EvidenceRef</code> |
| <code>interface.14.ts-web-api-generated-index-decodeexiteligibility</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1972</code> | <code>export function decodeExitEligibility(value: JsonValue &#124; undefined, at: string): ExitEligibility</code> |
| <code>interface.14.ts-web-api-generated-index-decodeexitstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2006</code> | <code>export function decodeExitStartRequest(value: JsonValue &#124; undefined, at: string): ExitStartRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeexportartefact</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2036</code> | <code>export function decodeExportArtefact(value: JsonValue &#124; undefined, at: string): ExportArtefact</code> |
| <code>interface.14.ts-web-api-generated-index-decodeexportedprimarykey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2116</code> | <code>export function decodeExportedPrimaryKey(value: JsonValue &#124; undefined, at: string): ExportedPrimaryKey</code> |
| <code>interface.14.ts-web-api-generated-index-decodeexportevidencerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2066</code> | <code>export function decodeExportEvidenceRequest(value: JsonValue &#124; undefined, at: string): ExportEvidenceRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeexportkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:253</code> | <code>export function decodeExportKind(value: JsonValue &#124; undefined, at: string): ExportKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodeexportstatementrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2093</code> | <code>export function decodeExportStatementRequest(value: JsonValue &#124; undefined, at: string): ExportStatementRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodehomesummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2143</code> | <code>export function decodeHomeSummary(value: JsonValue &#124; undefined, at: string): HomeSummary</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentconstraints</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2176</code> | <code>export function decodeIntentConstraints(value: JsonValue &#124; undefined, at: string): IntentConstraints</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentdomain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:267</code> | <code>export function decodeIntentDomain(value: JsonValue &#124; undefined, at: string): IntentDomain</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentendpoint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2200</code> | <code>export function decodeIntentEndpoint(value: JsonValue &#124; undefined, at: string): IntentEndpoint</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentendpointkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:281</code> | <code>export function decodeIntentEndpointKind(value: JsonValue &#124; undefined, at: string): IntentEndpointKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentleg</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2231</code> | <code>export function decodeIntentLeg(value: JsonValue &#124; undefined, at: string): IntentLeg</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentlegbinding</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:2270</code> | <code>export function decodeIntentLegBinding(value: JsonValue &#124; undefined, at: string): IntentLegBinding</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2309</code> | <code>export function decodeIntentPlan(value: JsonValue &#124; undefined, at: string): IntentPlan</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentsigningrequirement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2339</code> | <code>export function decodeIntentSigningRequirement(value: JsonValue &#124; undefined, at: string): IntentSigningRequirement</code> |
| <code>interface.14.ts-web-api-generated-index-decodeintentsubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2367</code> | <code>export function decodeIntentSubmission(value: JsonValue &#124; undefined, at: string): IntentSubmission</code> |
| <code>interface.14.ts-web-api-generated-index-decodejourney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2401</code> | <code>export function decodeJourney(value: JsonValue &#124; undefined, at: string): Journey</code> |
| <code>interface.14.ts-web-api-generated-index-decodejourneykind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:295</code> | <code>export function decodeJourneyKind(value: JsonValue &#124; undefined, at: string): JourneyKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodejourneypage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2447</code> | <code>export function decodeJourneyPage(value: JsonValue &#124; undefined, at: string): JourneyPage</code> |
| <code>interface.14.ts-web-api-generated-index-decodejourneystage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2471</code> | <code>export function decodeJourneyStage(value: JsonValue &#124; undefined, at: string): JourneyStage</code> |
| <code>interface.14.ts-web-api-generated-index-decodejourneystate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:309</code> | <code>export function decodeJourneyState(value: JsonValue &#124; undefined, at: string): JourneyState</code> |
| <code>interface.14.ts-web-api-generated-index-decodekeychallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2501</code> | <code>export function decodeKeyChallenge(value: JsonValue &#124; undefined, at: string): KeyChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-decodekeychallengekind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:323</code> | <code>export function decodeKeyChallengeKind(value: JsonValue &#124; undefined, at: string): KeyChallengeKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodekeyexportbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2530</code> | <code>export function decodeKeyExportBegin(value: JsonValue &#124; undefined, at: string): KeyExportBegin</code> |
| <code>interface.14.ts-web-api-generated-index-decodekeyexportchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2551</code> | <code>export function decodeKeyExportChallenge(value: JsonValue &#124; undefined, at: string): KeyExportChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-decodekeyexportfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2575</code> | <code>export function decodeKeyExportFinish(value: JsonValue &#124; undefined, at: string): KeyExportFinish</code> |
| <code>interface.14.ts-web-api-generated-index-decodelimitenforcement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:337</code> | <code>export function decodeLimitEnforcement(value: JsonValue &#124; undefined, at: string): LimitEnforcement</code> |
| <code>interface.14.ts-web-api-generated-index-decodemoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2597</code> | <code>export function decodeMoney(value: JsonValue &#124; undefined, at: string): Money</code> |
| <code>interface.14.ts-web-api-generated-index-decodemoneydirection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:351</code> | <code>export function decodeMoneyDirection(value: JsonValue &#124; undefined, at: string): MoneyDirection</code> |
| <code>interface.14.ts-web-api-generated-index-decodemovecommitrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2619</code> | <code>export function decodeMoveCommitRequest(value: JsonValue &#124; undefined, at: string): MoveCommitRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodemovemechanism</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:365</code> | <code>export function decodeMoveMechanism(value: JsonValue &#124; undefined, at: string): MoveMechanism</code> |
| <code>interface.14.ts-web-api-generated-index-decodemovequote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2653</code> | <code>export function decodeMoveQuote(value: JsonValue &#124; undefined, at: string): MoveQuote</code> |
| <code>interface.14.ts-web-api-generated-index-decodemovequoterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2700</code> | <code>export function decodeMoveQuoteRequest(value: JsonValue &#124; undefined, at: string): MoveQuoteRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodenativefeeasset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2725</code> | <code>export function decodeNativeFeeAsset(value: JsonValue &#124; undefined, at: string): NativeFeeAsset</code> |
| <code>interface.14.ts-web-api-generated-index-decodenativefeebudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2752</code> | <code>export function decodeNativeFeeBudget(value: JsonValue &#124; undefined, at: string): NativeFeeBudget</code> |
| <code>interface.14.ts-web-api-generated-index-decodenativesendaccessconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2789</code> | <code>export function decodeNativeSendAccessConfirmation(value: JsonValue &#124; undefined, at: string): NativeSendAccessConfirmation</code> |
| <code>interface.14.ts-web-api-generated-index-decodenativesendaccessdisclosure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2833</code> | <code>export function decodeNativeSendAccessDisclosure(value: JsonValue &#124; undefined, at: string): NativeSendAccessDisclosure</code> |
| <code>interface.14.ts-web-api-generated-index-decodenativesendaccessrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2863</code> | <code>export function decodeNativeSendAccessRequest(value: JsonValue &#124; undefined, at: string): NativeSendAccessRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodenativesendaccessresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2904</code> | <code>export function decodeNativeSendAccessResult(value: JsonValue &#124; undefined, at: string): NativeSendAccessResult</code> |
| <code>interface.14.ts-web-api-generated-index-decodenotificationclass</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:379</code> | <code>export function decodeNotificationClass(value: JsonValue &#124; undefined, at: string): NotificationClass</code> |
| <code>interface.14.ts-web-api-generated-index-decodenotificationdetaillevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:393</code> | <code>export function decodeNotificationDetailLevel(value: JsonValue &#124; undefined, at: string): NotificationDetailLevel</code> |
| <code>interface.14.ts-web-api-generated-index-decodenotificationgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2924</code> | <code>export function decodeNotificationGroup(value: JsonValue &#124; undefined, at: string): NotificationGroup</code> |
| <code>interface.14.ts-web-api-generated-index-decodenotificationpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2947</code> | <code>export function decodeNotificationPage(value: JsonValue &#124; undefined, at: string): NotificationPage</code> |
| <code>interface.14.ts-web-api-generated-index-decodenotificationpreferences</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2973</code> | <code>export function decodeNotificationPreferences(value: JsonValue &#124; undefined, at: string): NotificationPreferences</code> |
| <code>interface.14.ts-web-api-generated-index-decodenotificationrecency</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:407</code> | <code>export function decodeNotificationRecency(value: JsonValue &#124; undefined, at: string): NotificationRecency</code> |
| <code>interface.14.ts-web-api-generated-index-decodenotificationsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3009</code> | <code>export function decodeNotificationSummary(value: JsonValue &#124; undefined, at: string): NotificationSummary</code> |
| <code>interface.14.ts-web-api-generated-index-decodeownerrotationdisclosurerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3072</code> | <code>export function decodeOwnerRotationDisclosureRequest(value: JsonValue &#124; undefined, at: string): OwnerRotationDisclosureRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeownerrotationrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3097</code> | <code>export function decodeOwnerRotationRequest(value: JsonValue &#124; undefined, at: string): OwnerRotationRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3123</code> | <code>export function decodePasskey(value: JsonValue &#124; undefined, at: string): Passkey</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyassertion</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3155</code> | <code>export function decodePasskeyAssertion(value: JsonValue &#124; undefined, at: string): PasskeyAssertion</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyassertionbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3180</code> | <code>export function decodePasskeyAssertionBegin(value: JsonValue &#124; undefined, at: string): PasskeyAssertionBegin</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyassertionchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3203</code> | <code>export function decodePasskeyAssertionChallenge(value: JsonValue &#124; undefined, at: string): PasskeyAssertionChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyassertionfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3226</code> | <code>export function decodePasskeyAssertionFinish(value: JsonValue &#124; undefined, at: string): PasskeyAssertionFinish</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskeylist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3245</code> | <code>export function decodePasskeyList(value: JsonValue &#124; undefined, at: string): PasskeyList</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyregistrationbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3264</code> | <code>export function decodePasskeyRegistrationBegin(value: JsonValue &#124; undefined, at: string): PasskeyRegistrationBegin</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyregistrationchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3285</code> | <code>export function decodePasskeyRegistrationChallenge(value: JsonValue &#124; undefined, at: string): PasskeyRegistrationChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyregistrationfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3308</code> | <code>export function decodePasskeyRegistrationFinish(value: JsonValue &#124; undefined, at: string): PasskeyRegistrationFinish</code> |
| <code>interface.14.ts-web-api-generated-index-decodeplanintentrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3331</code> | <code>export function decodePlanIntentRequest(value: JsonValue &#124; undefined, at: string): PlanIntentRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprofile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3359</code> | <code>export function decodeProfile(value: JsonValue &#124; undefined, at: string): Profile</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprofileupdate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3385</code> | <code>export function decodeProfileUpdate(value: JsonValue &#124; undefined, at: string): ProfileUpdate</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalbudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3427</code> | <code>export function decodeProgramApprovalBudget(value: JsonValue &#124; undefined, at: string): ProgramApprovalBudget</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldecision</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3483</code> | <code>export function decodeProgramApprovalDecision(value: JsonValue &#124; undefined, at: string): ProgramApprovalDecision</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldecisionkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:421</code> | <code>export function decodeProgramApprovalDecisionKind(value: JsonValue &#124; undefined, at: string): ProgramApprovalDecisionKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldecisionrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3521</code> | <code>export function decodeProgramApprovalDecisionRequest(value: JsonValue &#124; undefined, at: string): ProgramApprovalDecisionRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3558</code> | <code>export function decodeProgramApprovalDetail(value: JsonValue &#124; undefined, at: string): ProgramApprovalDetail</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldisclosure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3621</code> | <code>export function decodeProgramApprovalDisclosure(value: JsonValue &#124; undefined, at: string): ProgramApprovalDisclosure</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldisclosurerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3650</code> | <code>export function decodeProgramApprovalDisclosureRequest(value: JsonValue &#124; undefined, at: string): ProgramApprovalDisclosureRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalmaterial</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3679</code> | <code>export function decodeProgramApprovalMaterial(value: JsonValue &#124; undefined, at: string): ProgramApprovalMaterial</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaloperation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3715</code> | <code>export function decodeProgramApprovalOperation(value: JsonValue &#124; undefined, at: string): ProgramApprovalOperation</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3761</code> | <code>export function decodeProgramApprovalPage(value: JsonValue &#124; undefined, at: string): ProgramApprovalPage</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalsemantics</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:435</code> | <code>export function decodeProgramApprovalSemantics(value: JsonValue &#124; undefined, at: string): ProgramApprovalSemantics</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:449</code> | <code>export function decodeProgramApprovalState(value: JsonValue &#124; undefined, at: string): ProgramApprovalState</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3793</code> | <code>export function decodeProgramApprovalSummary(value: JsonValue &#124; undefined, at: string): ProgramApprovalSummary</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramauthorizedlimit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3838</code> | <code>export function decodeProgramAuthorizedLimit(value: JsonValue &#124; undefined, at: string): ProgramAuthorizedLimit</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogrambytescommitment</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3870</code> | <code>export function decodeProgramBytesCommitment(value: JsonValue &#124; undefined, at: string): ProgramBytesCommitment</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramcallfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3897</code> | <code>export function decodeProgramCallFacts(value: JsonValue &#124; undefined, at: string): ProgramCallFacts</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramchargekind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:463</code> | <code>export function decodeProgramChargeKind(value: JsonValue &#124; undefined, at: string): ProgramChargeKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramdeployfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3932</code> | <code>export function decodeProgramDeployFacts(value: JsonValue &#124; undefined, at: string): ProgramDeployFacts</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogrammaterialprovenance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:477</code> | <code>export function decodeProgramMaterialProvenance(value: JsonValue &#124; undefined, at: string): ProgramMaterialProvenance</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramoperationkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:491</code> | <code>export function decodeProgramOperationKind(value: JsonValue &#124; undefined, at: string): ProgramOperationKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramupgradefacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3969</code> | <code>export function decodeProgramUpgradeFacts(value: JsonValue &#124; undefined, at: string): ProgramUpgradeFacts</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramupgradepolicyfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4005</code> | <code>export function decodeProgramUpgradePolicyFacts(value: JsonValue &#124; undefined, at: string): ProgramUpgradePolicyFacts</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramupgradepolicykind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:505</code> | <code>export function decodeProgramUpgradePolicyKind(value: JsonValue &#124; undefined, at: string): ProgramUpgradePolicyKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramwinddownfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4037</code> | <code>export function decodeProgramWindDownFacts(value: JsonValue &#124; undefined, at: string): ProgramWindDownFacts</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprogramwinddownkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:519</code> | <code>export function decodeProgramWindDownKind(value: JsonValue &#124; undefined, at: string): ProgramWindDownKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodeprotocolfreshness</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4102</code> | <code>export function decodeProtocolFreshness(value: JsonValue &#124; undefined, at: string): ProtocolFreshness</code> |
| <code>interface.14.ts-web-api-generated-index-decoderebindingsubmission</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:4136</code> | <code>export function decodeRebindingSubmission(value: JsonValue &#124; undefined, at: string): RebindingSubmission</code> |
| <code>interface.14.ts-web-api-generated-index-decoderefusal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4164</code> | <code>export function decodeRefusal(value: JsonValue &#124; undefined, at: string): Refusal</code> |
| <code>interface.14.ts-web-api-generated-index-decoderefusedby</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:533</code> | <code>export function decodeRefusedBy(value: JsonValue &#124; undefined, at: string): RefusedBy</code> |
| <code>interface.14.ts-web-api-generated-index-decoderesponseenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4196</code> | <code>export function decodeResponseEnvelope(value: JsonValue &#124; undefined, at: string): ResponseEnvelope</code> |
| <code>interface.14.ts-web-api-generated-index-decoderetriability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:547</code> | <code>export function decodeRetriability(value: JsonValue &#124; undefined, at: string): Retriability</code> |
| <code>interface.14.ts-web-api-generated-index-decodeschemaversion</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4226</code> | <code>export function decodeSchemaVersion(value: JsonValue &#124; undefined, at: string): SchemaVersion</code> |
| <code>interface.14.ts-web-api-generated-index-decodesecurityaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4247</code> | <code>export function decodeSecurityAction(value: JsonValue &#124; undefined, at: string): SecurityAction</code> |
| <code>interface.14.ts-web-api-generated-index-decodesecurityactionkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:561</code> | <code>export function decodeSecurityActionKind(value: JsonValue &#124; undefined, at: string): SecurityActionKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodesecurityactionrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4267</code> | <code>export function decodeSecurityActionRequest(value: JsonValue &#124; undefined, at: string): SecurityActionRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodesecuritypasskeyregistrationbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4293</code> | <code>export function decodeSecurityPasskeyRegistrationBegin(value: JsonValue &#124; undefined, at: string): SecurityPasskeyRegistrationBegin</code> |
| <code>interface.14.ts-web-api-generated-index-decodesecuritypasskeyregistrationfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4315</code> | <code>export function decodeSecurityPasskeyRegistrationFinish(value: JsonValue &#124; undefined, at: string): SecurityPasskeyRegistrationFinish</code> |
| <code>interface.14.ts-web-api-generated-index-decodesecuritypasskeyrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4336</code> | <code>export function decodeSecurityPasskeyRevocation(value: JsonValue &#124; undefined, at: string): SecurityPasskeyRevocation</code> |
| <code>interface.14.ts-web-api-generated-index-decodesecurityrecoveryreveal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4356</code> | <code>export function decodeSecurityRecoveryReveal(value: JsonValue &#124; undefined, at: string): SecurityRecoveryReveal</code> |
| <code>interface.14.ts-web-api-generated-index-decodesecuritysessionrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4377</code> | <code>export function decodeSecuritySessionRevocation(value: JsonValue &#124; undefined, at: string): SecuritySessionRevocation</code> |
| <code>interface.14.ts-web-api-generated-index-decodesession</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4400</code> | <code>export function decodeSession(value: JsonValue &#124; undefined, at: string): Session</code> |
| <code>interface.14.ts-web-api-generated-index-decodesessiondevice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4428</code> | <code>export function decodeSessionDevice(value: JsonValue &#124; undefined, at: string): SessionDevice</code> |
| <code>interface.14.ts-web-api-generated-index-decodesessionlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4449</code> | <code>export function decodeSessionList(value: JsonValue &#124; undefined, at: string): SessionList</code> |
| <code>interface.14.ts-web-api-generated-index-decodesessionopenrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4469</code> | <code>export function decodeSessionOpenRequest(value: JsonValue &#124; undefined, at: string): SessionOpenRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodesessionrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4495</code> | <code>export function decodeSessionRevocation(value: JsonValue &#124; undefined, at: string): SessionRevocation</code> |
| <code>interface.14.ts-web-api-generated-index-decodesettlementdomain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:575</code> | <code>export function decodeSettlementDomain(value: JsonValue &#124; undefined, at: string): SettlementDomain</code> |
| <code>interface.14.ts-web-api-generated-index-decodespendlimit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4518</code> | <code>export function decodeSpendLimit(value: JsonValue &#124; undefined, at: string): SpendLimit</code> |
| <code>interface.14.ts-web-api-generated-index-decodestepupchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4544</code> | <code>export function decodeStepUpChallenge(value: JsonValue &#124; undefined, at: string): StepUpChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-decodestepupevidence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4573</code> | <code>export function decodeStepUpEvidence(value: JsonValue &#124; undefined, at: string): StepUpEvidence</code> |
| <code>interface.14.ts-web-api-generated-index-decodestepupfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4600</code> | <code>export function decodeStepUpFinish(value: JsonValue &#124; undefined, at: string): StepUpFinish</code> |
| <code>interface.14.ts-web-api-generated-index-decodestepuprequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4619</code> | <code>export function decodeStepUpRequest(value: JsonValue &#124; undefined, at: string): StepUpRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodestreamevent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4644</code> | <code>export function decodeStreamEvent(value: JsonValue &#124; undefined, at: string): StreamEvent</code> |
| <code>interface.14.ts-web-api-generated-index-decodestreameventkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:589</code> | <code>export function decodeStreamEventKind(value: JsonValue &#124; undefined, at: string): StreamEventKind</code> |
| <code>interface.14.ts-web-api-generated-index-decodestreampage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4692</code> | <code>export function decodeStreamPage(value: JsonValue &#124; undefined, at: string): StreamPage</code> |
| <code>interface.14.ts-web-api-generated-index-decodestreamposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4713</code> | <code>export function decodeStreamPosition(value: JsonValue &#124; undefined, at: string): StreamPosition</code> |
| <code>interface.14.ts-web-api-generated-index-decodesubmitplanrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4735</code> | <code>export function decodeSubmitPlanRequest(value: JsonValue &#124; undefined, at: string): SubmitPlanRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportauthor</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:603</code> | <code>export function decodeSupportAuthor(value: JsonValue &#124; undefined, at: string): SupportAuthor</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportconversation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4771</code> | <code>export function decodeSupportConversation(value: JsonValue &#124; undefined, at: string): SupportConversation</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportconversationpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4808</code> | <code>export function decodeSupportConversationPage(value: JsonValue &#124; undefined, at: string): SupportConversationPage</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportconversationstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:617</code> | <code>export function decodeSupportConversationState(value: JsonValue &#124; undefined, at: string): SupportConversationState</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportconversationstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4830</code> | <code>export function decodeSupportConversationStatus(value: JsonValue &#124; undefined, at: string): SupportConversationStatus</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4858</code> | <code>export function decodeSupportCreateRequest(value: JsonValue &#124; undefined, at: string): SupportCreateRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportfeedback</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4893</code> | <code>export function decodeSupportFeedback(value: JsonValue &#124; undefined, at: string): SupportFeedback</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportfeedbackrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4917</code> | <code>export function decodeSupportFeedbackRequest(value: JsonValue &#124; undefined, at: string): SupportFeedbackRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4943</code> | <code>export function decodeSupportMessage(value: JsonValue &#124; undefined, at: string): SupportMessage</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportreadrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4976</code> | <code>export function decodeSupportReadRequest(value: JsonValue &#124; undefined, at: string): SupportReadRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportreplyrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4995</code> | <code>export function decodeSupportReplyRequest(value: JsonValue &#124; undefined, at: string): SupportReplyRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupportshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:631</code> | <code>export function decodeSupportShell(value: JsonValue &#124; undefined, at: string): SupportShell</code> |
| <code>interface.14.ts-web-api-generated-index-decodesupporttopic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:645</code> | <code>export function decodeSupportTopic(value: JsonValue &#124; undefined, at: string): SupportTopic</code> |
| <code>interface.14.ts-web-api-generated-index-decodetimedsecret</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5016</code> | <code>export function decodeTimedSecret(value: JsonValue &#124; undefined, at: string): TimedSecret</code> |
| <code>interface.14.ts-web-api-generated-index-decodeverificationlevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:659</code> | <code>export function decodeVerificationLevel(value: JsonValue &#124; undefined, at: string): VerificationLevel</code> |
| <code>interface.14.ts-web-api-generated-index-decodeverifiedmoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5040</code> | <code>export function decodeVerifiedMoney(value: JsonValue &#124; undefined, at: string): VerifiedMoney</code> |
| <code>interface.14.ts-web-api-generated-index-decodeversioninfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5062</code> | <code>export function decodeVersionInfo(value: JsonValue &#124; undefined, at: string): VersionInfo</code> |
| <code>interface.14.ts-web-api-generated-index-decodewalletbinding</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:5086</code> | <code>export function decodeWalletBinding(value: JsonValue &#124; undefined, at: string): WalletBinding</code> |
| <code>interface.14.ts-web-api-generated-index-decodewalletsignrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5127</code> | <code>export function decodeWalletSignRequest(value: JsonValue &#124; undefined, at: string): WalletSignRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodewithdrawclaimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5159</code> | <code>export function decodeWithdrawClaimRequest(value: JsonValue &#124; undefined, at: string): WithdrawClaimRequest</code> |
| <code>interface.14.ts-web-api-generated-index-decodewithdrawstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5186</code> | <code>export function decodeWithdrawStartRequest(value: JsonValue &#124; undefined, at: string): WithdrawStartRequest</code> |
| <code>interface.14.ts-web-api-generated-index-depositconfirmrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1807</code> | <code>export interface DepositConfirmRequest</code> |
| <code>interface.14.ts-web-api-generated-index-depositstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1833</code> | <code>export interface DepositStartRequest</code> |
| <code>interface.14.ts-web-api-generated-index-device</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1859</code> | <code>export interface Device</code> |
| <code>interface.14.ts-web-api-generated-index-deviceid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:113</code> | <code>export type DeviceId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-emailaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:115</code> | <code>export type EmailAddress = string;</code> |
| <code>interface.14.ts-web-api-generated-index-empty</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1884</code> | <code>export type Empty = Record&lt;string, never&gt;;</code> |
| <code>interface.14.ts-web-api-generated-index-encodeaccountbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:689</code> | <code>export function encodeAccountBalance(value: AccountBalance): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeaccountcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:714</code> | <code>export function encodeAccountCreateRequest(value: AccountCreateRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeaccountcreation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:736</code> | <code>export function encodeAccountCreation(value: AccountCreation): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeactivityentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:786</code> | <code>export function encodeActivityEntry(value: ActivityEntry): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeactivityentrydetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:863</code> | <code>export function encodeActivityEntryDetail(value: ActivityEntryDetail): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeactivityfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:920</code> | <code>export function encodeActivityFilter(value: ActivityFilter): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeactivitygroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:955</code> | <code>export function encodeActivityGroup(value: ActivityGroup): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeactivitypage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:981</code> | <code>export function encodeActivityPage(value: ActivityPage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeactivityqueryrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1011</code> | <code>export function encodeActivityQueryRequest(value: ActivityQueryRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeagent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1059</code> | <code>export function encodeAgent(value: Agent): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeagentarchiverequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1090</code> | <code>export function encodeAgentArchiveRequest(value: AgentArchiveRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeagentcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1117</code> | <code>export function encodeAgentCreateRequest(value: AgentCreateRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeagentlimitrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1141</code> | <code>export function encodeAgentLimitRequest(value: AgentLimitRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeagentpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1162</code> | <code>export function encodeAgentPage(value: AgentPage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeagentreclaimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1182</code> | <code>export function encodeAgentReclaimRequest(value: AgentReclaimRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeagentspend</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1213</code> | <code>export function encodeAgentSpend(value: AgentSpend): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeapierror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1251</code> | <code>export function encodeApiError(value: ApiError): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeapprovalapproverequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1278</code> | <code>export function encodeApprovalApproveRequest(value: ApprovalApproveRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeapprovaldecision</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1307</code> | <code>export function encodeApprovalDecision(value: ApprovalDecision): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeapprovaldetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1349</code> | <code>export function encodeApprovalDetail(value: ApprovalDetail): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeapprovalfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1385</code> | <code>export function encodeApprovalFacts(value: ApprovalFacts): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeapprovalpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1410</code> | <code>export function encodeApprovalPage(value: ApprovalPage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeapprovalsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1446</code> | <code>export function encodeApprovalSummary(value: ApprovalSummary): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatordisable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1473</code> | <code>export function encodeAuthenticatorDisable(value: AuthenticatorDisable): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatormethod</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1500</code> | <code>export function encodeAuthenticatorMethod(value: AuthenticatorMethod): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatorsetupbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1526</code> | <code>export function encodeAuthenticatorSetupBegin(value: AuthenticatorSetupBegin): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatorsetupchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1552</code> | <code>export function encodeAuthenticatorSetupChallenge(value: AuthenticatorSetupChallenge): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatorsetupfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1576</code> | <code>export function encodeAuthenticatorSetupFinish(value: AuthenticatorSetupFinish): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatorsetupresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1598</code> | <code>export function encodeAuthenticatorSetupResult(value: AuthenticatorSetupResult): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatorstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1620</code> | <code>export function encodeAuthenticatorStatus(value: AuthenticatorStatus): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodebackupcoderotation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1640</code> | <code>export function encodeBackupCodeRotation(value: BackupCodeRotation): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodebackupcodeset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1663</code> | <code>export function encodeBackupCodeSet(value: BackupCodeSet): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodebindingrebindaction</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:1686</code> | <code>export function encodeBindingRebindAction(value: BindingRebindAction): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodebindingstatement</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:1710</code> | <code>export function encodeBindingStatement(value: BindingStatement): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodebindingstatementrequest</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:1731</code> | <code>export function encodeBindingStatementRequest(value: BindingStatementRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodebindingsubmission</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:1754</code> | <code>export function encodeBindingSubmission(value: BindingSubmission): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodechannelpreference</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1777</code> | <code>export function encodeChannelPreference(value: ChannelPreference): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeclasstoggle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1799</code> | <code>export function encodeClassToggle(value: ClassToggle): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeconsensusinteger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:84</code> | <code>export function encodeConsensusInteger(value: bigint): string</code> |
| <code>interface.14.ts-web-api-generated-index-encodedepositconfirmrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1823</code> | <code>export function encodeDepositConfirmRequest(value: DepositConfirmRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodedepositstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1849</code> | <code>export function encodeDepositStartRequest(value: DepositStartRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodedevice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1875</code> | <code>export function encodeDevice(value: Device): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeempty</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1891</code> | <code>export function encodeEmpty(value: Empty): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeevidencematerial</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1919</code> | <code>export function encodeEvidenceMaterial(value: EvidenceMaterial): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeevidenceref</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1953</code> | <code>export function encodeEvidenceRef(value: EvidenceRef): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeexiteligibility</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1987</code> | <code>export function encodeExitEligibility(value: ExitEligibility): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeexitstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2017</code> | <code>export function encodeExitStartRequest(value: ExitStartRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeexportartefact</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2049</code> | <code>export function encodeExportArtefact(value: ExportArtefact): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeexportedprimarykey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2126</code> | <code>export function encodeExportedPrimaryKey(value: ExportedPrimaryKey): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeexportevidencerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2078</code> | <code>export function encodeExportEvidenceRequest(value: ExportEvidenceRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeexportstatementrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2102</code> | <code>export function encodeExportStatementRequest(value: ExportStatementRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodehomesummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2157</code> | <code>export function encodeHomeSummary(value: HomeSummary): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeintentconstraints</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2186</code> | <code>export function encodeIntentConstraints(value: IntentConstraints): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeintentendpoint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2211</code> | <code>export function encodeIntentEndpoint(value: IntentEndpoint): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeintentleg</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2245</code> | <code>export function encodeIntentLeg(value: IntentLeg): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeintentlegbinding</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:2286</code> | <code>export function encodeIntentLegBinding(value: IntentLegBinding): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeintentplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2321</code> | <code>export function encodeIntentPlan(value: IntentPlan): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeintentsigningrequirement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2350</code> | <code>export function encodeIntentSigningRequirement(value: IntentSigningRequirement): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeintentsubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2378</code> | <code>export function encodeIntentSubmission(value: IntentSubmission): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodejourney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2422</code> | <code>export function encodeJourney(value: Journey): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodejourneypage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2456</code> | <code>export function encodeJourneyPage(value: JourneyPage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodejourneystage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2482</code> | <code>export function encodeJourneyStage(value: JourneyStage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodekeychallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2514</code> | <code>export function encodeKeyChallenge(value: KeyChallenge): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodekeyexportbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2538</code> | <code>export function encodeKeyExportBegin(value: KeyExportBegin): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodekeyexportchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2561</code> | <code>export function encodeKeyExportChallenge(value: KeyExportChallenge): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodekeyexportfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2584</code> | <code>export function encodeKeyExportFinish(value: KeyExportFinish): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodemoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2606</code> | <code>export function encodeMoney(value: Money): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodemovecommitrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2630</code> | <code>export function encodeMoveCommitRequest(value: MoveCommitRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodemovequote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2674</code> | <code>export function encodeMoveQuote(value: MoveQuote): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodemovequoterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2710</code> | <code>export function encodeMoveQuoteRequest(value: MoveQuoteRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenativefeeasset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2735</code> | <code>export function encodeNativeFeeAsset(value: NativeFeeAsset): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenativefeebudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2764</code> | <code>export function encodeNativeFeeBudget(value: NativeFeeBudget): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenativesendaccessconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2809</code> | <code>export function encodeNativeSendAccessConfirmation(value: NativeSendAccessConfirmation): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenativesendaccessdisclosure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2842</code> | <code>export function encodeNativeSendAccessDisclosure(value: NativeSendAccessDisclosure): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenativesendaccessrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2882</code> | <code>export function encodeNativeSendAccessRequest(value: NativeSendAccessRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenativesendaccessresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2912</code> | <code>export function encodeNativeSendAccessResult(value: NativeSendAccessResult): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenotificationgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2933</code> | <code>export function encodeNotificationGroup(value: NotificationGroup): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenotificationpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2957</code> | <code>export function encodeNotificationPage(value: NotificationPage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenotificationpreferences</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2984</code> | <code>export function encodeNotificationPreferences(value: NotificationPreferences): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodenotificationsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3038</code> | <code>export function encodeNotificationSummary(value: NotificationSummary): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeownerrotationdisclosurerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3082</code> | <code>export function encodeOwnerRotationDisclosureRequest(value: OwnerRotationDisclosureRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeownerrotationrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3107</code> | <code>export function encodeOwnerRotationRequest(value: OwnerRotationRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3136</code> | <code>export function encodePasskey(value: Passkey): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyassertion</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3166</code> | <code>export function encodePasskeyAssertion(value: PasskeyAssertion): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyassertionbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3189</code> | <code>export function encodePasskeyAssertionBegin(value: PasskeyAssertionBegin): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyassertionchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3213</code> | <code>export function encodePasskeyAssertionChallenge(value: PasskeyAssertionChallenge): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyassertionfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3234</code> | <code>export function encodePasskeyAssertionFinish(value: PasskeyAssertionFinish): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskeylist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3253</code> | <code>export function encodePasskeyList(value: PasskeyList): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyregistrationbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3272</code> | <code>export function encodePasskeyRegistrationBegin(value: PasskeyRegistrationBegin): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyregistrationchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3295</code> | <code>export function encodePasskeyRegistrationChallenge(value: PasskeyRegistrationChallenge): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyregistrationfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3316</code> | <code>export function encodePasskeyRegistrationFinish(value: PasskeyRegistrationFinish): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeplanintentrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3343</code> | <code>export function encodePlanIntentRequest(value: PlanIntentRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprofile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3370</code> | <code>export function encodeProfile(value: Profile): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprofileupdate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3397</code> | <code>export function encodeProfileUpdate(value: ProfileUpdate): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovalbudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3450</code> | <code>export function encodeProgramApprovalBudget(value: ProgramApprovalBudget): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaldecision</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3500</code> | <code>export function encodeProgramApprovalDecision(value: ProgramApprovalDecision): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaldecisionrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3530</code> | <code>export function encodeProgramApprovalDecisionRequest(value: ProgramApprovalDecisionRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaldetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3586</code> | <code>export function encodeProgramApprovalDetail(value: ProgramApprovalDetail): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaldisclosure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3633</code> | <code>export function encodeProgramApprovalDisclosure(value: ProgramApprovalDisclosure): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaldisclosurerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3660</code> | <code>export function encodeProgramApprovalDisclosureRequest(value: ProgramApprovalDisclosureRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovalmaterial</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3693</code> | <code>export function encodeProgramApprovalMaterial(value: ProgramApprovalMaterial): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaloperation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3736</code> | <code>export function encodeProgramApprovalOperation(value: ProgramApprovalOperation): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovalpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3770</code> | <code>export function encodeProgramApprovalPage(value: ProgramApprovalPage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovalsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3812</code> | <code>export function encodeProgramApprovalSummary(value: ProgramApprovalSummary): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramauthorizedlimit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3852</code> | <code>export function encodeProgramAuthorizedLimit(value: ProgramAuthorizedLimit): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogrambytescommitment</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3879</code> | <code>export function encodeProgramBytesCommitment(value: ProgramBytesCommitment): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramcallfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3911</code> | <code>export function encodeProgramCallFacts(value: ProgramCallFacts): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramdeployfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3946</code> | <code>export function encodeProgramDeployFacts(value: ProgramDeployFacts): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramupgradefacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3985</code> | <code>export function encodeProgramUpgradeFacts(value: ProgramUpgradeFacts): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramupgradepolicyfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4016</code> | <code>export function encodeProgramUpgradePolicyFacts(value: ProgramUpgradePolicyFacts): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprogramwinddownfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4066</code> | <code>export function encodeProgramWindDownFacts(value: ProgramWindDownFacts): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeprotocolfreshness</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4116</code> | <code>export function encodeProtocolFreshness(value: ProtocolFreshness): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encoderebindingsubmission</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:4147</code> | <code>export function encodeRebindingSubmission(value: RebindingSubmission): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encoderefusal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4177</code> | <code>export function encodeRefusal(value: Refusal): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encoderesponseenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4209</code> | <code>export function encodeResponseEnvelope(value: ResponseEnvelope): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeschemaversion</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4235</code> | <code>export function encodeSchemaVersion(value: SchemaVersion): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesecurityaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4255</code> | <code>export function encodeSecurityAction(value: SecurityAction): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesecurityactionrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4278</code> | <code>export function encodeSecurityActionRequest(value: SecurityActionRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesecuritypasskeyregistrationbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4302</code> | <code>export function encodeSecurityPasskeyRegistrationBegin(value: SecurityPasskeyRegistrationBegin): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesecuritypasskeyregistrationfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4324</code> | <code>export function encodeSecurityPasskeyRegistrationFinish(value: SecurityPasskeyRegistrationFinish): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesecuritypasskeyrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4344</code> | <code>export function encodeSecurityPasskeyRevocation(value: SecurityPasskeyRevocation): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesecurityrecoveryreveal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4365</code> | <code>export function encodeSecurityRecoveryReveal(value: SecurityRecoveryReveal): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesecuritysessionrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4385</code> | <code>export function encodeSecuritySessionRevocation(value: SecuritySessionRevocation): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesession</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4412</code> | <code>export function encodeSession(value: Session): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesessiondevice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4437</code> | <code>export function encodeSessionDevice(value: SessionDevice): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesessionlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4457</code> | <code>export function encodeSessionList(value: SessionList): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesessionopenrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4480</code> | <code>export function encodeSessionOpenRequest(value: SessionOpenRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesessionrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4504</code> | <code>export function encodeSessionRevocation(value: SessionRevocation): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodespendlimit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4528</code> | <code>export function encodeSpendLimit(value: SpendLimit): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodestepupchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4555</code> | <code>export function encodeStepUpChallenge(value: StepUpChallenge): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodestepupevidence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4585</code> | <code>export function encodeStepUpEvidence(value: StepUpEvidence): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodestepupfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4608</code> | <code>export function encodeStepUpFinish(value: StepUpFinish): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodestepuprequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4627</code> | <code>export function encodeStepUpRequest(value: StepUpRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodestreamevent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4666</code> | <code>export function encodeStreamEvent(value: StreamEvent): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodestreampage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4701</code> | <code>export function encodeStreamPage(value: StreamPage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodestreamposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4721</code> | <code>export function encodeStreamPosition(value: StreamPosition): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesubmitplanrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4748</code> | <code>export function encodeSubmitPlanRequest(value: SubmitPlanRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportconversation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4788</code> | <code>export function encodeSupportConversation(value: SupportConversation): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportconversationpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4816</code> | <code>export function encodeSupportConversationPage(value: SupportConversationPage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportconversationstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4841</code> | <code>export function encodeSupportConversationStatus(value: SupportConversationStatus): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4873</code> | <code>export function encodeSupportCreateRequest(value: SupportCreateRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportfeedback</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4903</code> | <code>export function encodeSupportFeedback(value: SupportFeedback): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportfeedbackrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4926</code> | <code>export function encodeSupportFeedbackRequest(value: SupportFeedbackRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4958</code> | <code>export function encodeSupportMessage(value: SupportMessage): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportreadrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4984</code> | <code>export function encodeSupportReadRequest(value: SupportReadRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodesupportreplyrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5003</code> | <code>export function encodeSupportReplyRequest(value: SupportReplyRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodetimedsecret</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5026</code> | <code>export function encodeTimedSecret(value: TimedSecret): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeverifiedmoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5049</code> | <code>export function encodeVerifiedMoney(value: VerifiedMoney): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodeversioninfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5071</code> | <code>export function encodeVersionInfo(value: VersionInfo): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodewalletbinding</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:5103</code> | <code>export function encodeWalletBinding(value: WalletBinding): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodewalletsignrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5141</code> | <code>export function encodeWalletSignRequest(value: WalletSignRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodewithdrawclaimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5170</code> | <code>export function encodeWithdrawClaimRequest(value: WithdrawClaimRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-encodewithdrawstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5198</code> | <code>export function encodeWithdrawStartRequest(value: WithdrawStartRequest): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-errorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:221</code> | <code>export type ErrorCode = "unauthenticated" &#124; "session-expired" &#124; "step-up-required" &#124; "forbidden" &#124; "not-found" &#124; "invalid-request" &#124; "conflict" &#124; "rate-limited" &#124; "cursor-expired" &#124; "unavailable" &#124; "upstream-degraded" &#124; "challenge-expired" &#124; "refused-by-policy" &#124; "refused-by-budget" &#124; "refused-by-capability" &#124; "refused-by-protocol" &#124; "refused-by-limit" &#124; "quote-expired" &#124; "wallet-not-bound" &#124; "exit-unavailable" &#124; "already-decided" &#124; "hold-expired" &#124; "hold-defective" &#124; "archive-needs-disposition" ...</code> |
| <code>interface.14.ts-web-api-generated-index-errorcodevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:223</code> | <code>export const errorCodeVariants: readonly ErrorCode[] = ["unauthenticated", "session-expired", "step-up-required", "forbidden", "not-found", "invalid-request", "conflict", "rate-limited", "cursor-expired", "unavailable", "upstream-degraded", "challenge-expired", "refused-by-policy", "refused-by-budget", "refused-by-capability", "refused-by-protocol", "refused-by-limit", "quote-expired", "wallet-not-bound", "exit-unavailable", "already-decided", "hold-expired", "hold-defective", "archive-needs-dis ...</code> |
| <code>interface.14.ts-web-api-generated-index-evidenceclass</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:235</code> | <code>export type EvidenceClass = "local-journey-state" &#124; "submission-record" &#124; "layerx-receipt" &#124; "checkpoint-proof" &#124; "paxeer-finality" &#124; "typed-refusal" &#124; "approval-hold" &#124; "wallet-ack";</code> |
| <code>interface.14.ts-web-api-generated-index-evidenceclassvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:237</code> | <code>export const evidenceClassVariants: readonly EvidenceClass[] = ["local-journey-state", "submission-record", "layerx-receipt", "checkpoint-proof", "paxeer-finality", "typed-refusal", "approval-hold", "wallet-ack"];</code> |
| <code>interface.14.ts-web-api-generated-index-evidenceid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:117</code> | <code>export type EvidenceId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-evidencematerial</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1895</code> | <code>export interface EvidenceMaterial</code> |
| <code>interface.14.ts-web-api-generated-index-evidenceref</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1933</code> | <code>export interface EvidenceRef</code> |
| <code>interface.14.ts-web-api-generated-index-evmaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:119</code> | <code>export type EvmAddress = string;</code> |
| <code>interface.14.ts-web-api-generated-index-evmsignature</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:121</code> | <code>export type EvmSignature = string;</code> |
| <code>interface.14.ts-web-api-generated-index-exiteligibility</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:1965</code> | <code>export interface ExitEligibility</code> |
| <code>interface.14.ts-web-api-generated-index-exitstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2001</code> | <code>export interface ExitStartRequest</code> |
| <code>interface.14.ts-web-api-generated-index-expectarray</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:34</code> | <code>export function expectArray(value: JsonValue &#124; undefined, at: string): JsonValue[]</code> |
| <code>interface.14.ts-web-api-generated-index-expectboolean</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:50</code> | <code>export function expectBoolean(value: JsonValue &#124; undefined, at: string): boolean</code> |
| <code>interface.14.ts-web-api-generated-index-expectinteger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:58</code> | <code>export function expectInteger(value: JsonValue &#124; undefined, at: string): number</code> |
| <code>interface.14.ts-web-api-generated-index-expectobject</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:26</code> | <code>export function expectObject(value: JsonValue &#124; undefined, at: string): JsonObject</code> |
| <code>interface.14.ts-web-api-generated-index-expectstring</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:42</code> | <code>export function expectString(value: JsonValue &#124; undefined, at: string): string</code> |
| <code>interface.14.ts-web-api-generated-index-exportartefact</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2027</code> | <code>export interface ExportArtefact</code> |
| <code>interface.14.ts-web-api-generated-index-exportedprimarykey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2110</code> | <code>export interface ExportedPrimaryKey</code> |
| <code>interface.14.ts-web-api-generated-index-exportevidencerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2061</code> | <code>export interface ExportEvidenceRequest</code> |
| <code>interface.14.ts-web-api-generated-index-exportid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:123</code> | <code>export type ExportId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-exportkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:249</code> | <code>export type ExportKind = "statement" &#124; "evidence-bundle";</code> |
| <code>interface.14.ts-web-api-generated-index-exportkindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:251</code> | <code>export const exportKindVariants: readonly ExportKind[] = ["statement", "evidence-bundle"];</code> |
| <code>interface.14.ts-web-api-generated-index-exportstatementrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2089</code> | <code>export interface ExportStatementRequest</code> |
| <code>interface.14.ts-web-api-generated-index-fetchlike</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5433</code> | <code>export type FetchLike = (input: string, init: RequestInit) =&gt; Promise&lt;Response&gt;;</code> |
| <code>interface.14.ts-web-api-generated-index-homesummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2135</code> | <code>export interface HomeSummary</code> |
| <code>interface.14.ts-web-api-generated-index-httpmethod</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5223</code> | <code>export type HttpMethod = "DELETE" &#124; "GET" &#124; "PATCH" &#124; "POST" &#124; "PUT";</code> |
| <code>interface.14.ts-web-api-generated-index-humanapiclient</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5563</code> | <code>export interface HumanApiClient</code> |
| <code>interface.14.ts-web-api-generated-index-humanapiclientoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5435</code> | <code>export interface HumanApiClientOptions</code> |
| <code>interface.14.ts-web-api-generated-index-humanapidecodeerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:12</code> | <code>export class HumanApiDecodeError extends Error</code> |
| <code>interface.14.ts-web-api-generated-index-humanapierror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5209</code> | <code>export class HumanApiError extends Error</code> |
| <code>interface.14.ts-web-api-generated-index-intentconstraints</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2170</code> | <code>export interface IntentConstraints</code> |
| <code>interface.14.ts-web-api-generated-index-intentdomain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:263</code> | <code>export type IntentDomain = "paxeer" &#124; "layerx";</code> |
| <code>interface.14.ts-web-api-generated-index-intentdomainvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:265</code> | <code>export const intentDomainVariants: readonly IntentDomain[] = ["paxeer", "layerx"];</code> |
| <code>interface.14.ts-web-api-generated-index-intentendpoint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2195</code> | <code>export interface IntentEndpoint</code> |
| <code>interface.14.ts-web-api-generated-index-intentendpointkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:277</code> | <code>export type IntentEndpointKind = "paxeer-wallet" &#124; "human" &#124; "agent" &#124; "agent-budget";</code> |
| <code>interface.14.ts-web-api-generated-index-intentendpointkindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:279</code> | <code>export const intentEndpointKindVariants: readonly IntentEndpointKind[] = ["paxeer-wallet", "human", "agent", "agent-budget"];</code> |
| <code>interface.14.ts-web-api-generated-index-intentleg</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2221</code> | <code>export interface IntentLeg</code> |
| <code>interface.14.ts-web-api-generated-index-intentlegbinding</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:2258</code> | <code>export interface IntentLegBinding</code> |
| <code>interface.14.ts-web-api-generated-index-intentplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2301</code> | <code>export interface IntentPlan</code> |
| <code>interface.14.ts-web-api-generated-index-intentsigningrequirement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2332</code> | <code>export interface IntentSigningRequirement</code> |
| <code>interface.14.ts-web-api-generated-index-intentsubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2360</code> | <code>export interface IntentSubmission</code> |
| <code>interface.14.ts-web-api-generated-index-journey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2388</code> | <code>export interface Journey</code> |
| <code>interface.14.ts-web-api-generated-index-journeyid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:125</code> | <code>export type JourneyId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-journeykind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:291</code> | <code>export type JourneyKind = "onboarding" &#124; "wallet-binding" &#124; "deposit" &#124; "withdraw" &#124; "exit" &#124; "move" &#124; "agent-create" &#124; "agent-fund" &#124; "agent-pause" &#124; "agent-retire";</code> |
| <code>interface.14.ts-web-api-generated-index-journeykindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:293</code> | <code>export const journeyKindVariants: readonly JourneyKind[] = ["onboarding", "wallet-binding", "deposit", "withdraw", "exit", "move", "agent-create", "agent-fund", "agent-pause", "agent-retire"];</code> |
| <code>interface.14.ts-web-api-generated-index-journeypage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2442</code> | <code>export interface JourneyPage</code> |
| <code>interface.14.ts-web-api-generated-index-journeystage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2464</code> | <code>export interface JourneyStage</code> |
| <code>interface.14.ts-web-api-generated-index-journeystate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:305</code> | <code>export type JourneyState = "getting-ready" &#124; "sending" &#124; "processing" &#124; "done" &#124; "done-finalised" &#124; "still-checking" &#124; "refused" &#124; "waiting-for-you";</code> |
| <code>interface.14.ts-web-api-generated-index-journeystatevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:307</code> | <code>export const journeyStateVariants: readonly JourneyState[] = ["getting-ready", "sending", "processing", "done", "done-finalised", "still-checking", "refused", "waiting-for-you"];</code> |
| <code>interface.14.ts-web-api-generated-index-jsonobject</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:8</code> | <code>export interface JsonObject</code> |
| <code>interface.14.ts-web-api-generated-index-jsonvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:6</code> | <code>export type JsonValue = null &#124; boolean &#124; number &#124; string &#124; JsonValue[] &#124; JsonObject;</code> |
| <code>interface.14.ts-web-api-generated-index-keychallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2492</code> | <code>export interface KeyChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-keychallengekind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:319</code> | <code>export type KeyChallengeKind = "rotate" &#124; "recover";</code> |
| <code>interface.14.ts-web-api-generated-index-keychallengekindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:321</code> | <code>export const keyChallengeKindVariants: readonly KeyChallengeKind[] = ["rotate", "recover"];</code> |
| <code>interface.14.ts-web-api-generated-index-keyexportbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2526</code> | <code>export interface KeyExportBegin</code> |
| <code>interface.14.ts-web-api-generated-index-keyexportchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2545</code> | <code>export interface KeyExportChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-keyexportfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2570</code> | <code>export interface KeyExportFinish</code> |
| <code>interface.14.ts-web-api-generated-index-limitenforcement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:333</code> | <code>export type LimitEnforcement = "protocol" &#124; "app";</code> |
| <code>interface.14.ts-web-api-generated-index-limitenforcementvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:335</code> | <code>export const limitEnforcementVariants: readonly LimitEnforcement[] = ["protocol", "app"];</code> |
| <code>interface.14.ts-web-api-generated-index-milliseconds</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:127</code> | <code>export type Milliseconds = bigint;</code> |
| <code>interface.14.ts-web-api-generated-index-money</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2592</code> | <code>export interface Money</code> |
| <code>interface.14.ts-web-api-generated-index-moneydirection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:347</code> | <code>export type MoneyDirection = "in" &#124; "out";</code> |
| <code>interface.14.ts-web-api-generated-index-moneydirectionvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:349</code> | <code>export const moneyDirectionVariants: readonly MoneyDirection[] = ["in", "out"];</code> |
| <code>interface.14.ts-web-api-generated-index-movecommitrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2614</code> | <code>export interface MoveCommitRequest</code> |
| <code>interface.14.ts-web-api-generated-index-movemechanism</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:361</code> | <code>export type MoveMechanism = "fund" &#124; "allocate" &#124; "return" &#124; "transfer";</code> |
| <code>interface.14.ts-web-api-generated-index-movemechanismvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:363</code> | <code>export const moveMechanismVariants: readonly MoveMechanism[] = ["fund", "allocate", "return", "transfer"];</code> |
| <code>interface.14.ts-web-api-generated-index-movequote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2640</code> | <code>export interface MoveQuote</code> |
| <code>interface.14.ts-web-api-generated-index-movequoterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2694</code> | <code>export interface MoveQuoteRequest</code> |
| <code>interface.14.ts-web-api-generated-index-nativefeeasset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2719</code> | <code>export interface NativeFeeAsset</code> |
| <code>interface.14.ts-web-api-generated-index-nativefeebudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2744</code> | <code>export interface NativeFeeBudget</code> |
| <code>interface.14.ts-web-api-generated-index-nativesendaccessconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2775</code> | <code>export interface NativeSendAccessConfirmation</code> |
| <code>interface.14.ts-web-api-generated-index-nativesendaccessdisclosure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2828</code> | <code>export interface NativeSendAccessDisclosure</code> |
| <code>interface.14.ts-web-api-generated-index-nativesendaccessrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2850</code> | <code>export interface NativeSendAccessRequest</code> |
| <code>interface.14.ts-web-api-generated-index-nativesendaccessresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2900</code> | <code>export interface NativeSendAccessResult</code> |
| <code>interface.14.ts-web-api-generated-index-nativesendcount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:129</code> | <code>export type NativeSendCount = bigint;</code> |
| <code>interface.14.ts-web-api-generated-index-nativesendunixseconds</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:131</code> | <code>export type NativeSendUnixSeconds = bigint;</code> |
| <code>interface.14.ts-web-api-generated-index-notificationclass</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:375</code> | <code>export type NotificationClass = "approval-waiting" &#124; "money-arrived" &#124; "journey-finished" &#124; "claim-ready" &#124; "security-new-device" &#124; "security-recovery" &#124; "security-wallet-rebinding" &#124; "security-key-rotation" &#124; "service-status";</code> |
| <code>interface.14.ts-web-api-generated-index-notificationclassvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:377</code> | <code>export const notificationClassVariants: readonly NotificationClass[] = ["approval-waiting", "money-arrived", "journey-finished", "claim-ready", "security-new-device", "security-recovery", "security-wallet-rebinding", "security-key-rotation", "service-status"];</code> |
| <code>interface.14.ts-web-api-generated-index-notificationdetaillevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:389</code> | <code>export type NotificationDetailLevel = "full" &#124; "summary" &#124; "minimal";</code> |
| <code>interface.14.ts-web-api-generated-index-notificationdetaillevelvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:391</code> | <code>export const notificationDetailLevelVariants: readonly NotificationDetailLevel[] = ["full", "summary", "minimal"];</code> |
| <code>interface.14.ts-web-api-generated-index-notificationgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2919</code> | <code>export interface NotificationGroup</code> |
| <code>interface.14.ts-web-api-generated-index-notificationid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:133</code> | <code>export type NotificationId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-notificationpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2941</code> | <code>export interface NotificationPage</code> |
| <code>interface.14.ts-web-api-generated-index-notificationpreferences</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2966</code> | <code>export interface NotificationPreferences</code> |
| <code>interface.14.ts-web-api-generated-index-notificationrecency</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:403</code> | <code>export type NotificationRecency = "today" &#124; "yesterday" &#124; "this-week" &#124; "earlier";</code> |
| <code>interface.14.ts-web-api-generated-index-notificationrecencyvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:405</code> | <code>export const notificationRecencyVariants: readonly NotificationRecency[] = ["today", "yesterday", "this-week", "earlier"];</code> |
| <code>interface.14.ts-web-api-generated-index-notificationsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:2994</code> | <code>export interface NotificationSummary</code> |
| <code>interface.14.ts-web-api-generated-index-opaquecredential</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:135</code> | <code>export type OpaqueCredential = string;</code> |
| <code>interface.14.ts-web-api-generated-index-operationdigest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:137</code> | <code>export type OperationDigest = string;</code> |
| <code>interface.14.ts-web-api-generated-index-operationname</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5333</code> | <code>export type OperationName = (typeof operationNames)[number];</code> |
| <code>interface.14.ts-web-api-generated-index-operationnames</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5235</code> | <code>export const operationNames = [</code> |
| <code>interface.14.ts-web-api-generated-index-operations</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5335</code> | <code>export const operations: { readonly [name in OperationName]: OperationShape } =</code> |
| <code>interface.14.ts-web-api-generated-index-operationshape</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5225</code> | <code>export interface OperationShape</code> |
| <code>interface.14.ts-web-api-generated-index-ownerrotationdisclosurerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3066</code> | <code>export interface OwnerRotationDisclosureRequest</code> |
| <code>interface.14.ts-web-api-generated-index-ownerrotationrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3091</code> | <code>export interface OwnerRotationRequest</code> |
| <code>interface.14.ts-web-api-generated-index-passkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3116</code> | <code>export interface Passkey</code> |
| <code>interface.14.ts-web-api-generated-index-passkeyassertion</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3148</code> | <code>export interface PasskeyAssertion</code> |
| <code>interface.14.ts-web-api-generated-index-passkeyassertionbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3176</code> | <code>export interface PasskeyAssertionBegin</code> |
| <code>interface.14.ts-web-api-generated-index-passkeyassertionchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3197</code> | <code>export interface PasskeyAssertionChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-passkeyassertionfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3222</code> | <code>export interface PasskeyAssertionFinish</code> |
| <code>interface.14.ts-web-api-generated-index-passkeyid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:139</code> | <code>export type PasskeyId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-passkeylist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3241</code> | <code>export interface PasskeyList</code> |
| <code>interface.14.ts-web-api-generated-index-passkeyregistrationbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3260</code> | <code>export interface PasskeyRegistrationBegin</code> |
| <code>interface.14.ts-web-api-generated-index-passkeyregistrationchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3279</code> | <code>export interface PasskeyRegistrationChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-passkeyregistrationfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3304</code> | <code>export interface PasskeyRegistrationFinish</code> |
| <code>interface.14.ts-web-api-generated-index-planintentrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3323</code> | <code>export interface PlanIntentRequest</code> |
| <code>interface.14.ts-web-api-generated-index-profile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3354</code> | <code>export interface Profile</code> |
| <code>interface.14.ts-web-api-generated-index-profileupdate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3380</code> | <code>export interface ProfileUpdate</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovalbudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3408</code> | <code>export interface ProgramApprovalBudget</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovaldecision</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3472</code> | <code>export interface ProgramApprovalDecision</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovaldecisionkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:417</code> | <code>export type ProgramApprovalDecisionKind = "approve" &#124; "reject";</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovaldecisionkindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:419</code> | <code>export const programApprovalDecisionKindVariants: readonly ProgramApprovalDecisionKind[] = ["approve", "reject"];</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovaldecisionrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3516</code> | <code>export interface ProgramApprovalDecisionRequest</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovaldetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3538</code> | <code>export interface ProgramApprovalDetail</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovaldisclosure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3613</code> | <code>export interface ProgramApprovalDisclosure</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovaldisclosurerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3644</code> | <code>export interface ProgramApprovalDisclosureRequest</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovalmaterial</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3669</code> | <code>export interface ProgramApprovalMaterial</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovaloperation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3706</code> | <code>export interface ProgramApprovalOperation</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovalpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3756</code> | <code>export interface ProgramApprovalPage</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovalsemantics</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:431</code> | <code>export type ProgramApprovalSemantics = "operation-only" &#124; "authorized-limits";</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovalsemanticsvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:433</code> | <code>export const programApprovalSemanticsVariants: readonly ProgramApprovalSemantics[] = ["operation-only", "authorized-limits"];</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovalstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:445</code> | <code>export type ProgramApprovalState = "pending" &#124; "approved" &#124; "rejected" &#124; "expired" &#124; "defective" &#124; "not-required";</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovalstatevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:447</code> | <code>export const programApprovalStateVariants: readonly ProgramApprovalState[] = ["pending", "approved", "rejected", "expired", "defective", "not-required"];</code> |
| <code>interface.14.ts-web-api-generated-index-programapprovalsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3778</code> | <code>export interface ProgramApprovalSummary</code> |
| <code>interface.14.ts-web-api-generated-index-programauthorizedlimit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3830</code> | <code>export interface ProgramAuthorizedLimit</code> |
| <code>interface.14.ts-web-api-generated-index-programbytescommitment</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3865</code> | <code>export interface ProgramBytesCommitment</code> |
| <code>interface.14.ts-web-api-generated-index-programcallfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3887</code> | <code>export interface ProgramCallFacts</code> |
| <code>interface.14.ts-web-api-generated-index-programchargekind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:459</code> | <code>export type ProgramChargeKind = "principal" &#124; "program-spend";</code> |
| <code>interface.14.ts-web-api-generated-index-programchargekindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:461</code> | <code>export const programChargeKindVariants: readonly ProgramChargeKind[] = ["principal", "program-spend"];</code> |
| <code>interface.14.ts-web-api-generated-index-programdeployfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3924</code> | <code>export interface ProgramDeployFacts</code> |
| <code>interface.14.ts-web-api-generated-index-programmaterialprovenance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:473</code> | <code>export type ProgramMaterialProvenance = "local-owned";</code> |
| <code>interface.14.ts-web-api-generated-index-programmaterialprovenancevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:475</code> | <code>export const programMaterialProvenanceVariants: readonly ProgramMaterialProvenance[] = ["local-owned"];</code> |
| <code>interface.14.ts-web-api-generated-index-programoperationkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:487</code> | <code>export type ProgramOperationKind = "deploy" &#124; "upgrade" &#124; "call" &#124; "wind-down";</code> |
| <code>interface.14.ts-web-api-generated-index-programoperationkindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:489</code> | <code>export const programOperationKindVariants: readonly ProgramOperationKind[] = ["deploy", "upgrade", "call", "wind-down"];</code> |
| <code>interface.14.ts-web-api-generated-index-programupgradefacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:3959</code> | <code>export interface ProgramUpgradeFacts</code> |
| <code>interface.14.ts-web-api-generated-index-programupgradepolicyfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4000</code> | <code>export interface ProgramUpgradePolicyFacts</code> |
| <code>interface.14.ts-web-api-generated-index-programupgradepolicykind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:501</code> | <code>export type ProgramUpgradePolicyKind = "immutable" &#124; "authority";</code> |
| <code>interface.14.ts-web-api-generated-index-programupgradepolicykindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:503</code> | <code>export const programUpgradePolicyKindVariants: readonly ProgramUpgradePolicyKind[] = ["immutable", "authority"];</code> |
| <code>interface.14.ts-web-api-generated-index-programwinddownfacts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4026</code> | <code>export interface ProgramWindDownFacts</code> |
| <code>interface.14.ts-web-api-generated-index-programwinddownkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:515</code> | <code>export type ProgramWindDownKind = "route" &#124; "deprecate" &#124; "tombstone" &#124; "exit" &#124; "bounded-exit";</code> |
| <code>interface.14.ts-web-api-generated-index-programwinddownkindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:517</code> | <code>export const programWindDownKindVariants: readonly ProgramWindDownKind[] = ["route", "deprecate", "tombstone", "exit", "bounded-exit"];</code> |
| <code>interface.14.ts-web-api-generated-index-protocolfreshness</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4094</code> | <code>export interface ProtocolFreshness</code> |
| <code>interface.14.ts-web-api-generated-index-quoteid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:141</code> | <code>export type QuoteId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-rebindingsubmission</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/api/generated/index.ts:4129</code> | <code>export interface RebindingSubmission</code> |
| <code>interface.14.ts-web-api-generated-index-refusal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4157</code> | <code>export interface Refusal</code> |
| <code>interface.14.ts-web-api-generated-index-refusedby</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:529</code> | <code>export type RefusedBy = "policy" &#124; "budget" &#124; "capability" &#124; "protocol" &#124; "limit";</code> |
| <code>interface.14.ts-web-api-generated-index-refusedbyvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:531</code> | <code>export const refusedByVariants: readonly RefusedBy[] = ["policy", "budget", "capability", "protocol", "limit"];</code> |
| <code>interface.14.ts-web-api-generated-index-registrationid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:143</code> | <code>export type RegistrationId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-requirevalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:19</code> | <code>export function requireValue(value: JsonValue &#124; undefined, at: string): JsonValue</code> |
| <code>interface.14.ts-web-api-generated-index-responseenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4189</code> | <code>export interface ResponseEnvelope</code> |
| <code>interface.14.ts-web-api-generated-index-retriability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:543</code> | <code>export type Retriability = "retriable" &#124; "retriable-after" &#124; "structural" &#124; "final";</code> |
| <code>interface.14.ts-web-api-generated-index-retriabilityvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:545</code> | <code>export const retriabilityVariants: readonly Retriability[] = ["retriable", "retriable-after", "structural", "final"];</code> |
| <code>interface.14.ts-web-api-generated-index-schemaversion</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4</code> | <code>export const schemaVersion = { major: 1, minor: 9 } as const;</code> |
| <code>interface.14.ts-web-api-generated-index-schemaversion-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4221</code> | <code>export interface SchemaVersion</code> |
| <code>interface.14.ts-web-api-generated-index-securityaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4243</code> | <code>export interface SecurityAction</code> |
| <code>interface.14.ts-web-api-generated-index-securityactionkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:557</code> | <code>export type SecurityActionKind = "add-passkey" &#124; "revoke-passkey" &#124; "revoke-session" &#124; "revoke-all-sessions" &#124; "add-authenticator" &#124; "disable-authenticator" &#124; "rotate-backup-codes" &#124; "reveal-recovery-evidence" &#124; "export-primary-key";</code> |
| <code>interface.14.ts-web-api-generated-index-securityactionkindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:559</code> | <code>export const securityActionKindVariants: readonly SecurityActionKind[] = ["add-passkey", "revoke-passkey", "revoke-session", "revoke-all-sessions", "add-authenticator", "disable-authenticator", "rotate-backup-codes", "reveal-recovery-evidence", "export-primary-key"];</code> |
| <code>interface.14.ts-web-api-generated-index-securityactionrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4262</code> | <code>export interface SecurityActionRequest</code> |
| <code>interface.14.ts-web-api-generated-index-securitypasskeyregistrationbegin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4288</code> | <code>export interface SecurityPasskeyRegistrationBegin</code> |
| <code>interface.14.ts-web-api-generated-index-securitypasskeyregistrationfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4310</code> | <code>export interface SecurityPasskeyRegistrationFinish</code> |
| <code>interface.14.ts-web-api-generated-index-securitypasskeyrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4332</code> | <code>export interface SecurityPasskeyRevocation</code> |
| <code>interface.14.ts-web-api-generated-index-securityrecoveryreveal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4351</code> | <code>export interface SecurityRecoveryReveal</code> |
| <code>interface.14.ts-web-api-generated-index-securitysessionrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4373</code> | <code>export interface SecuritySessionRevocation</code> |
| <code>interface.14.ts-web-api-generated-index-session</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4392</code> | <code>export interface Session</code> |
| <code>interface.14.ts-web-api-generated-index-sessiondevice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4423</code> | <code>export interface SessionDevice</code> |
| <code>interface.14.ts-web-api-generated-index-sessionid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:145</code> | <code>export type SessionId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-sessionlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4445</code> | <code>export interface SessionList</code> |
| <code>interface.14.ts-web-api-generated-index-sessionopenrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4464</code> | <code>export interface SessionOpenRequest</code> |
| <code>interface.14.ts-web-api-generated-index-sessionrevocation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4490</code> | <code>export interface SessionRevocation</code> |
| <code>interface.14.ts-web-api-generated-index-settlementdomain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:571</code> | <code>export type SettlementDomain = "paxeer";</code> |
| <code>interface.14.ts-web-api-generated-index-settlementdomainvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:573</code> | <code>export const settlementDomainVariants: readonly SettlementDomain[] = ["paxeer"];</code> |
| <code>interface.14.ts-web-api-generated-index-spendlimit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4512</code> | <code>export interface SpendLimit</code> |
| <code>interface.14.ts-web-api-generated-index-stageid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:147</code> | <code>export type StageId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-stepupchallenge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4537</code> | <code>export interface StepUpChallenge</code> |
| <code>interface.14.ts-web-api-generated-index-stepupchallengeid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:149</code> | <code>export type StepUpChallengeId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-stepupevidence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4565</code> | <code>export interface StepUpEvidence</code> |
| <code>interface.14.ts-web-api-generated-index-stepupfinish</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4596</code> | <code>export interface StepUpFinish</code> |
| <code>interface.14.ts-web-api-generated-index-stepuprequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4615</code> | <code>export interface StepUpRequest</code> |
| <code>interface.14.ts-web-api-generated-index-streamevent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4634</code> | <code>export interface StreamEvent</code> |
| <code>interface.14.ts-web-api-generated-index-streameventkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:585</code> | <code>export type StreamEventKind = "journey-progress" &#124; "approval-created" &#124; "approval-approved" &#124; "approval-rejected" &#124; "approval-expired" &#124; "notification" &#124; "program-approval-created" &#124; "program-approval-approved" &#124; "program-approval-rejected" &#124; "program-approval-expired";</code> |
| <code>interface.14.ts-web-api-generated-index-streameventkindvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:587</code> | <code>export const streamEventKindVariants: readonly StreamEventKind[] = ["journey-progress", "approval-created", "approval-approved", "approval-rejected", "approval-expired", "notification", "program-approval-created", "program-approval-approved", "program-approval-rejected", "program-approval-expired"];</code> |
| <code>interface.14.ts-web-api-generated-index-streampage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4687</code> | <code>export interface StreamPage</code> |
| <code>interface.14.ts-web-api-generated-index-streamposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4709</code> | <code>export interface StreamPosition</code> |
| <code>interface.14.ts-web-api-generated-index-streampushprofile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5448</code> | <code>export const streamPushProfile =</code> |
| <code>interface.14.ts-web-api-generated-index-streamsubscribeoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5444</code> | <code>export interface StreamSubscribeOptions</code> |
| <code>interface.14.ts-web-api-generated-index-submitplanrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4728</code> | <code>export interface SubmitPlanRequest</code> |
| <code>interface.14.ts-web-api-generated-index-supportauthor</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:599</code> | <code>export type SupportAuthor = "you" &#124; "support";</code> |
| <code>interface.14.ts-web-api-generated-index-supportauthorvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:601</code> | <code>export const supportAuthorVariants: readonly SupportAuthor[] = ["you", "support"];</code> |
| <code>interface.14.ts-web-api-generated-index-supportconversation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4760</code> | <code>export interface SupportConversation</code> |
| <code>interface.14.ts-web-api-generated-index-supportconversationid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:151</code> | <code>export type SupportConversationId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-supportconversationpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4804</code> | <code>export interface SupportConversationPage</code> |
| <code>interface.14.ts-web-api-generated-index-supportconversationstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:613</code> | <code>export type SupportConversationState = "waiting-for-support" &#124; "waiting-for-you" &#124; "resolved";</code> |
| <code>interface.14.ts-web-api-generated-index-supportconversationstatevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:615</code> | <code>export const supportConversationStateVariants: readonly SupportConversationState[] = ["waiting-for-support", "waiting-for-you", "resolved"];</code> |
| <code>interface.14.ts-web-api-generated-index-supportconversationstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4823</code> | <code>export interface SupportConversationStatus</code> |
| <code>interface.14.ts-web-api-generated-index-supportcreaterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4851</code> | <code>export interface SupportCreateRequest</code> |
| <code>interface.14.ts-web-api-generated-index-supportfeedback</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4887</code> | <code>export interface SupportFeedback</code> |
| <code>interface.14.ts-web-api-generated-index-supportfeedbackrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4912</code> | <code>export interface SupportFeedbackRequest</code> |
| <code>interface.14.ts-web-api-generated-index-supportmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4934</code> | <code>export interface SupportMessage</code> |
| <code>interface.14.ts-web-api-generated-index-supportmessageid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:153</code> | <code>export type SupportMessageId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-supportreadrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4972</code> | <code>export interface SupportReadRequest</code> |
| <code>interface.14.ts-web-api-generated-index-supportreplyrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:4991</code> | <code>export interface SupportReplyRequest</code> |
| <code>interface.14.ts-web-api-generated-index-supportshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:627</code> | <code>export type SupportShell = "mobile" &#124; "desktop";</code> |
| <code>interface.14.ts-web-api-generated-index-supportshellvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:629</code> | <code>export const supportShellVariants: readonly SupportShell[] = ["mobile", "desktop"];</code> |
| <code>interface.14.ts-web-api-generated-index-supporttopic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:641</code> | <code>export type SupportTopic = "deposit" &#124; "withdrawal" &#124; "agents" &#124; "account" &#124; "report";</code> |
| <code>interface.14.ts-web-api-generated-index-supporttopicvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:643</code> | <code>export const supportTopicVariants: readonly SupportTopic[] = ["deposit", "withdrawal", "agents", "account", "report"];</code> |
| <code>interface.14.ts-web-api-generated-index-timedsecret</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5010</code> | <code>export interface TimedSecret</code> |
| <code>interface.14.ts-web-api-generated-index-timestamp</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:155</code> | <code>export type Timestamp = string;</code> |
| <code>interface.14.ts-web-api-generated-index-traceid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:157</code> | <code>export type TraceId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-verificationlevel</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:655</code> | <code>export type VerificationLevel = "unverified" &#124; "receipt-verified" &#124; "checkpoint-finalised" &#124; "paxeer-finalised" &#124; "settlement-anchored";</code> |
| <code>interface.14.ts-web-api-generated-index-verificationlevelvariants</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:657</code> | <code>export const verificationLevelVariants: readonly VerificationLevel[] = ["unverified", "receipt-verified", "checkpoint-finalised", "paxeer-finalised", "settlement-anchored"];</code> |
| <code>interface.14.ts-web-api-generated-index-verifiedmoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5035</code> | <code>export interface VerifiedMoney</code> |
| <code>interface.14.ts-web-api-generated-index-versioninfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5057</code> | <code>export interface VersionInfo</code> |
| <code>interface.14.ts-web-api-generated-index-walletaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:159</code> | <code>export type WalletAddress = string;</code> |
| <code>interface.14.ts-web-api-generated-index-walletbinding</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/generated/index.ts:5079</code> | <code>export interface WalletBinding</code> |
| <code>interface.14.ts-web-api-generated-index-walletsignature</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:161</code> | <code>export type WalletSignature = string;</code> |
| <code>interface.14.ts-web-api-generated-index-walletsignrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5119</code> | <code>export interface WalletSignRequest</code> |
| <code>interface.14.ts-web-api-generated-index-wallettxid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:163</code> | <code>export type WalletTxId = string;</code> |
| <code>interface.14.ts-web-api-generated-index-withdrawclaimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5154</code> | <code>export interface WithdrawClaimRequest</code> |
| <code>interface.14.ts-web-api-generated-index-withdrawstartrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/generated/index.ts:5180</code> | <code>export interface WithdrawStartRequest</code> |
| <code>interface.14.ts-web-api-index-humanapi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/index.ts:6</code> | <code>export function humanApi(options: HumanApiClientOptions = {}): HumanApiClient</code> |
| <code>interface.14.ts-web-api-performance-client-submitwebvital</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/performance-client.ts:5</code> | <code>export function submitWebVital( route: PerformanceRoute, metric: WebVitalName, observed: number, ): void</code> |
| <code>interface.14.ts-web-api-performance-sink-performancesinkhealth</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/performance-sink.ts:49</code> | <code>export interface PerformanceSinkHealth</code> |
| <code>interface.14.ts-web-api-performance-sink-performancesinkresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/performance-sink.ts:62</code> | <code>export interface PerformanceSinkResult</code> |
| <code>interface.14.ts-web-api-performance-sink-performancesinksnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/performance-sink.ts:57</code> | <code>export interface PerformanceSinkSnapshot</code> |
| <code>interface.14.ts-web-api-performance-sink-performancesinksnapshot-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/performance-sink.ts:384</code> | <code>export async function performanceSinkSnapshot(): Promise&lt;PerformanceSinkSnapshot&gt;</code> |
| <code>interface.14.ts-web-api-performance-sink-persistperformanceobservation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/performance-sink.ts:363</code> | <code>export async function persistPerformanceObservation( observation: RedactedWebVital, ): Promise&lt;PerformanceSinkResult&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-bridgeactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:341</code> | <code>export async function bridgeActivity(recipient: string): Promise&lt;readonly ObservedEvent[]&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-bridgeattestors</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:264</code> | <code>export interface BridgeAttestors</code> |
| <code>interface.14.ts-web-api-precompiles-bridgeattestors-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:310</code> | <code>export async function bridgeAttestors(): Promise&lt;BridgeAttestors&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-bridgecap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:257</code> | <code>export interface BridgeCap</code> |
| <code>interface.14.ts-web-api-precompiles-bridgecap-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:291</code> | <code>export async function bridgeCap(chain: bigint, asset: string): Promise&lt;BridgeCap&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-bridgechain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:250</code> | <code>export interface BridgeChain</code> |
| <code>interface.14.ts-web-api-precompiles-bridgechain-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:276</code> | <code>export async function bridgeChain(chain: bigint): Promise&lt;BridgeChain&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-bridgenullified</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:323</code> | <code>export async function bridgeNullified(chain: bigint, txHash: string, logIndex: bigint): Promise&lt;boolean&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-bridgepaused</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:269</code> | <code>export async function bridgePaused(): Promise&lt;boolean&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-launchpadconfig</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:121</code> | <code>export interface LaunchpadConfig</code> |
| <code>interface.14.ts-web-api-precompiles-launchpadconfig-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:201</code> | <code>export async function launchpadConfig(): Promise&lt;LaunchpadConfig&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-launchpadmarket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:106</code> | <code>export interface LaunchpadMarket</code> |
| <code>interface.14.ts-web-api-precompiles-launchpadmarketcount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:150</code> | <code>export async function launchpadMarketCount(): Promise&lt;bigint&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-launchpadmarkets</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:189</code> | <code>export async function launchpadMarkets(offset: bigint, limit: bigint): Promise&lt;readonly LaunchpadMarket[]&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-launchpadquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:212</code> | <code>export interface LaunchpadQuote</code> |
| <code>interface.14.ts-web-api-precompiles-launchpadquote-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:219</code> | <code>export async function launchpadQuote(token: string, side: "buy" &#124; "sell", amountIn: bigint): Promise&lt;LaunchpadQuote&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-observedevent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:50</code> | <code>export interface ObservedEvent</code> |
| <code>interface.14.ts-web-api-precompiles-perps-collateral-decimals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:360</code> | <code>export const PERPS_COLLATERAL_DECIMALS = 6;</code> |
| <code>interface.14.ts-web-api-precompiles-perps-collateral-token</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:358</code> | <code>export const PERPS_COLLATERAL_TOKEN = "0x85fcd13735f4309833a503ee804ea32395851479";</code> |
| <code>interface.14.ts-web-api-precompiles-perps-errors</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:367</code> | <code>export const PERPS_ERRORS = [</code> |
| <code>interface.14.ts-web-api-precompiles-perps-max-markets</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:364</code> | <code>export const PERPS_MAX_MARKETS = 1024n;</code> |
| <code>interface.14.ts-web-api-precompiles-perps-usd-decimals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:362</code> | <code>export const PERPS_USD_DECIMALS = 18;</code> |
| <code>interface.14.ts-web-api-precompiles-perpsaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:502</code> | <code>export interface PerpsAccount</code> |
| <code>interface.14.ts-web-api-precompiles-perpsaccount-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:550</code> | <code>export async function perpsAccount(owner: string): Promise&lt;PerpsAccount&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-perpsmarket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:439</code> | <code>export interface PerpsMarket</code> |
| <code>interface.14.ts-web-api-precompiles-perpsmarkets</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:459</code> | <code>export async function perpsMarkets(): Promise&lt;readonly PerpsMarket[]&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-perpsorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:491</code> | <code>export interface PerpsOrder</code> |
| <code>interface.14.ts-web-api-precompiles-perpsposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:481</code> | <code>export interface PerpsPosition</code> |
| <code>interface.14.ts-web-api-precompiles-perpspreflight</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:426</code> | <code>export async function perpsPreflight(from: string, data: string): Promise&lt;string &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-api-precompiles-perpsrevertreason</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:386</code> | <code>export function perpsRevertReason(data: string &#124; undefined): string &#124; undefined</code> |
| <code>interface.14.ts-web-api-precompiles-precompile-log-window</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/precompiles.ts:29</code> | <code>export const PRECOMPILE_LOG_WINDOW = 50_000n;</code> |
| <code>interface.14.ts-web-api-provider-eip1193provider-j53</code> | <code>ts_export</code> | <code>new</code> | <code>apps/web/src/api/provider.ts</code> (planned) | <code>export interface Eip1193Provider</code> |
| <code>interface.14.ts-web-api-provider-windowwalletprovider-j53</code> | <code>ts_export</code> | <code>new</code> | <code>apps/web/src/api/provider.ts</code> (planned) | <code>export function windowWalletProvider(): Eip1193Provider &#124; undefined</code> |
| <code>interface.14.ts-web-api-server-humanapiforserver</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/server.ts:11</code> | <code>export function humanApiForServer(context: ServerHumanApiContext): HumanApiClient</code> |
| <code>interface.14.ts-web-api-server-serverhumanapicontext</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/server.ts:5</code> | <code>export interface ServerHumanApiContext</code> |
| <code>interface.14.ts-web-api-stream-humanstreamobserver</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/stream.ts:9</code> | <code>export interface HumanStreamObserver</code> |
| <code>interface.14.ts-web-api-stream-observehumanstream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/stream.ts:34</code> | <code>export async function observeHumanStream( client: HumanApiClient, observer: HumanStreamObserver, ): Promise&lt;void&gt;</code> |
| <code>interface.14.ts-web-api-wallet-choosewalletfee</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/wallet.ts:109</code> | <code>export function chooseWalletFee(currency: WalletFeePreference["currency"], maximum: string): boolean</code> |
| <code>interface.14.ts-web-api-wallet-connectwalletaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/wallet.ts:42</code> | <code>export async function connectWalletAccount(): Promise&lt;string &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-api-wallet-precompilesendoutcome</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/wallet.ts:12</code> | <code>export type PrecompileSendOutcome =</code> |
| <code>interface.14.ts-web-api-wallet-resumewalletsponsoredsubmission</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/wallet.ts:178</code> | <code>export async function resumeWalletSponsoredSubmission(account: string, call?: PrecompileCall): Promise&lt;PrecompileSendOutcome &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-api-wallet-sendwalletprecompilecall</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/wallet.ts:53</code> | <code>export async function sendWalletPrecompileCall(from: string, build: () =&gt; PrecompileCall): Promise&lt;PrecompileSendOutcome&gt;</code> |
| <code>interface.14.ts-web-api-wallet-sendwalletsponsoredbatch</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/api/wallet.ts:206</code> | <code>export async function sendWalletSponsoredBatch(quoted: SidioraGasQuote, consentIdentity: string &#124; undefined): Promise&lt;PrecompileSendOutcome&gt;</code> |
| <code>interface.14.ts-web-api-wallet-sidioraamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/wallet.ts:118</code> | <code>export function sidioraAmount(amount: bigint): string</code> |
| <code>interface.14.ts-web-api-wallet-sidioraquotepresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/wallet.ts:123</code> | <code>export function sidioraQuotePresentation(quoted: SidioraGasQuote, now = BigInt(Math.floor(Date.now() / 1000)))</code> |
| <code>interface.14.ts-web-api-wallet-sponsoredwalletmodule</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/wallet.ts:145</code> | <code>export function sponsoredWalletModule(quoted: SidioraGasQuote, wallet: NonNullable&lt;ReturnType&lt;typeof windowWalletProvider&gt;&gt;, submissionStore?: GasSubmissionStore)</code> |
| <code>interface.14.ts-web-api-wallet-walletfeepreference</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/wallet.ts:92</code> | <code>export type WalletFeePreference = Readonly&lt;{ currency: "paxeer" &#124; "sidiora"; maximum?: bigint }&gt;;</code> |
| <code>interface.14.ts-web-api-wallet-walletfeepreference-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/wallet.ts:96</code> | <code>export function walletFeePreference(): WalletFeePreference</code> |
| <code>interface.14.ts-web-api-wallet-walletsendfailure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/api/wallet.ts:22</code> | <code>export function walletSendFailure(error: unknown): PrecompileSendOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-api-abi-read-abireadscalar</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-abi-read-abireadtype</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-abi-read-abireaduint</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-abi-read-abireadvalue</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-gas-station-accountdidoutcome</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-gas-station-gasquoteoutcome</code> | notes | stale task 8.5.20 do_4 adds a refusal outcome carrying budget_exhausted |
| <code>interface.14.ts-web-api-gas-station-requestsidioragasquote</code> | notes | stale task 8.5.20 do_4 maps the gateway refusal budget_exhausted to a refusal outcome carrying the code; GasQuoteOutcome (apps/web/src/api/gas-station.ts:21) has no such outcome |
| <code>interface.14.ts-web-api-gas-station-sendsponsoredbatch</code> | notes | returns PrecompileSendOutcome with transactionHash only; stale task 8.5.20 do_4 requires the hash with the verified receipt |
| <code>interface.14.ts-web-api-gateway-ethtransactionreceipt-j53</code> | notes | eth_getTransactionReceipt through the gateway relay; no caller until tasks 14.4.1-14.4.3 |
| <code>interface.14.ts-web-api-generated-conformance-runbody</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-conformance-runkey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-conformance-runparam</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-accountcreaterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-accountcreation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-accountid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-activityentryid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-activityqueryrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-agentarchiverequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-agentcreaterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-agentid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-agentlimitrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-agentreclaimrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-agentspend</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-agentstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-agentstatevariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-amount</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-approvalapproverequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-approvalfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-approvalid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-approvalpage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-approvalstatevariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-assertionid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-authenticatordisable</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-authenticatorid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-authenticatormethod</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-authenticatorsetupresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-backupcoderotation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-bindingrebindaction</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-bindingstate</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-bindingstatement</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it; still used at apps/web/src/settings/wallet/model.ts:2 |
| <code>interface.14.ts-web-api-generated-index-bindingstatementrequest</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-bindingstatevariants</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-bindingsubmission</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-classtoggle</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-copykey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-currencycode</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-cursor</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeaccountbalance</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeaccountcreation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeactivityentry</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeactivityentrydetail</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeactivityentrykind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeactivityfilter</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeactivitygroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeactivitypage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeagentspend</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeagentstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeapierror</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeapprovaldecision</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeapprovaldetail</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalpage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeapprovalsummary</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodearray</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatormethod</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatorsetupchallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatorsetupresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeauthenticatorstatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodebackupcodeset</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodebindingrebindaction</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-decodebindingstate</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-decodebindingstatement</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-decodebindingstatementrequest</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it; still used at apps/web/src/api/generated/conformance.ts:4 |
| <code>interface.14.ts-web-api-generated-index-decodebindingsubmission</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it; still used at apps/web/src/api/generated/conformance.ts:4 |
| <code>interface.14.ts-web-api-generated-index-decodechannelpreference</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeclasstoggle</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeconsensusinteger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodedevice</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeempty</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeerrorcode</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeevidenceclass</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeevidencematerial</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeevidenceref</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeexiteligibility</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeexportartefact</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeexportedprimarykey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeexportkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodehomesummary</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeintentconstraints</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeintentdomain</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeintentendpoint</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeintentendpointkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeintentleg</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeintentlegbinding</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-decodeintentplan</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeintentsigningrequirement</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeintentsubmission</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodejourneykind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodejourneypage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodejourneystage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodejourneystate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodekeychallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodekeychallengekind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodekeyexportchallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodelimitenforcement</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodemoneydirection</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodemovemechanism</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodemovequote</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodenativesendaccessdisclosure</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodenativesendaccessresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodenotificationclass</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodenotificationdetaillevel</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodenotificationgroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodenotificationpage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodenotificationrecency</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodenotificationsummary</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodepasskey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyassertion</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyassertionchallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodepasskeylist</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodepasskeyregistrationchallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalbudget</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldecision</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldecisionkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldetail</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaldisclosure</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalmaterial</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovaloperation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalpage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalsemantics</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramapprovalsummary</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramauthorizedlimit</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogrambytescommitment</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramcallfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramchargekind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramdeployfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogrammaterialprovenance</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramoperationkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramupgradefacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramupgradepolicyfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramupgradepolicykind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramwinddownfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprogramwinddownkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeprotocolfreshness</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decoderebindingsubmission</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it; still used at apps/web/src/api/generated/conformance.ts:4 |
| <code>interface.14.ts-web-api-generated-index-decoderefusal</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decoderefusedby</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decoderesponseenvelope</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decoderetriability</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeschemaversion</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesecurityaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesecurityactionkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesession</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesessiondevice</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesessionlist</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesessionrevocation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesettlementdomain</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodespendlimit</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodestepupchallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodestepupevidence</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodestreamevent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodestreameventkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodestreampage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodestreamposition</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupportauthor</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupportconversation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupportconversationpage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupportconversationstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupportconversationstatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupportfeedback</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupportmessage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupportshell</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodesupporttopic</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodetimedsecret</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeverificationlevel</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeverifiedmoney</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodeversioninfo</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-decodewalletbinding</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-decodewalletsignrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-depositconfirmrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-depositstartrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-device</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-deviceid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-emailaddress</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-empty</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeaccountcreaterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeactivityentry</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeactivitygroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeactivityqueryrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeagentarchiverequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeagentcreaterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeagentlimitrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeagentreclaimrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeagentspend</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeapierror</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeapprovalapproverequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeapprovalfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeapprovalsummary</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatordisable</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatormethod</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatorsetupbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeauthenticatorsetupfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodebackupcoderotation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodebindingrebindaction</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it; still used at apps/web/src/api/generated/conformance.ts:4 |
| <code>interface.14.ts-web-api-generated-index-encodebindingstatement</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it; still used at apps/web/src/api/generated/conformance.ts:4 |
| <code>interface.14.ts-web-api-generated-index-encodebindingstatementrequest</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-encodebindingsubmission</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-encodechannelpreference</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeclasstoggle</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeconsensusinteger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodedepositconfirmrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodedepositstartrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodedevice</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeempty</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeevidenceref</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeexitstartrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeexportevidencerequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeexportstatementrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeintentconstraints</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeintentendpoint</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeintentleg</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeintentlegbinding</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-encodeintentsigningrequirement</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodejourneystage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodekeyexportbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodekeyexportfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodemovecommitrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodemovequoterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodenativesendaccessconfirmation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodenativesendaccessrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodenotificationgroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeownerrotationdisclosurerequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeownerrotationrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyassertionbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyassertionfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyregistrationbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodepasskeyregistrationfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeplanintentrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaldecisionrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaldisclosurerequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovaloperation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramapprovalsummary</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramauthorizedlimit</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogrambytescommitment</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramcallfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramdeployfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramupgradefacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramupgradepolicyfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprogramwinddownfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeprotocolfreshness</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encoderebindingsubmission</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-encoderefusal</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encoderesponseenvelope</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeschemaversion</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesecurityactionrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesecuritypasskeyregistrationbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesecuritypasskeyregistrationfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesecuritypasskeyrevocation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesecurityrecoveryreveal</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesecuritysessionrevocation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesessiondevice</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesessionopenrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodespendlimit</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodestepupfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodestepuprequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodestreamevent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesubmitplanrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesupportcreaterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesupportfeedback</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesupportfeedbackrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesupportmessage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesupportreadrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodesupportreplyrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodeverifiedmoney</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodewalletbinding</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it; still used at apps/web/src/api/generated/conformance.ts:4 |
| <code>interface.14.ts-web-api-generated-index-encodewalletsignrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodewithdrawclaimrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-encodewithdrawstartrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-evidenceclassvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-evidenceid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-exitstartrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-expectarray</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-expectboolean</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-expectinteger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-expectobject</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-expectstring</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-exportedprimarykey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-exportevidencerequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-exportid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-exportkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-exportkindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-exportstatementrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-homesummary</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-httpmethod</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentconstraints</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentdomain</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentdomainvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentendpoint</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentendpointkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentendpointkindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentleg</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentlegbinding</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-intentplan</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentsigningrequirement</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-intentsubmission</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-journeyid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-journeykindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-journeypage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-journeystatevariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-jsonobject</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-keychallengekind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-keychallengekindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-keyexportbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-keyexportchallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-keyexportfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-limitenforcementvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-milliseconds</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-moneydirectionvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-movecommitrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-movemechanism</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-movemechanismvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-movequoterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-nativesendaccessconfirmation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-nativesendaccessdisclosure</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-nativesendaccessresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-nativesendcount</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-nativesendunixseconds</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-notificationdetaillevelvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-notificationgroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-notificationid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-notificationrecency</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-notificationrecencyvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-ownerrotationdisclosurerequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-passkeyassertion</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-passkeyassertionbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-passkeyassertionchallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-passkeyassertionfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-passkeylist</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-passkeyregistrationbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-passkeyregistrationfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-planintentrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-profileupdate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovaldecisionkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovaldecisionkindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovaldecisionrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovaldisclosure</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovaldisclosurerequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovaloperation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovalpage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovalsemantics</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovalsemanticsvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programapprovalstatevariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programauthorizedlimit</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programbytescommitment</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programcallfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programchargekind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programchargekindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programdeployfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programmaterialprovenance</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programmaterialprovenancevariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programoperationkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programoperationkindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programupgradefacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programupgradepolicyfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programupgradepolicykind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programupgradepolicykindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programwinddownfacts</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programwinddownkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-programwinddownkindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-protocolfreshness</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-quoteid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-rebindingsubmission</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it |
| <code>interface.14.ts-web-api-generated-index-refusedby</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-refusedbyvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-registrationid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-requirevalue</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-responseenvelope</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-retriability</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-retriabilityvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-schemaversion</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-schemaversion-2</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-securityaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-securityactionkindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-securityactionrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-securitypasskeyregistrationbegin</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-securitypasskeyregistrationfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-securitypasskeyrevocation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-securityrecoveryreveal</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-securitysessionrevocation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-sessiondevice</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-sessionid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-sessionlist</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-sessionopenrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-sessionrevocation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-settlementdomain</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-settlementdomainvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-spendlimit</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-stageid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-stepupchallenge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-stepupfinish</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-stepuprequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-streameventkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-streameventkindvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-streampage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-streamposition</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-streampushprofile</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-streamsubscribeoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-submitplanrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportauthor</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportauthorvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportconversationid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportconversationpage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportconversationstatevariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportconversationstatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportcreaterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportfeedback</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportfeedbackrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportmessageid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportreadrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportreplyrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supportshellvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-supporttopicvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-verificationlevel</code> | notes | feature 8 scope (spec/layerx-merge/spec.kvx:32248) reports the three rungs submitted, receipted, committed; this surface still carries checkpoint-finalised or settlement-anchored levels |
| <code>interface.14.ts-web-api-generated-index-verificationlevelvariants</code> | notes | feature 8 scope (spec/layerx-merge/spec.kvx:32248) reports the three rungs submitted, receipted, committed; this surface still carries checkpoint-finalised or settlement-anchored levels |
| <code>interface.14.ts-web-api-generated-index-versioninfo</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-walletaddress</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-walletbinding</code> | notes | stale req.8043 removes the wallet payout binding; the generated binding operations and types go with it; still used at apps/web/e2e/custody.spec.ts:25, apps/web/src/settings/settings-screen.tsx:8, apps/web/src/settings/wallet/model.ts:2 |
| <code>interface.14.ts-web-api-generated-index-walletsignature</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-wallettxid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-withdrawclaimrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-generated-index-withdrawstartrequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-performance-sink-performancesinkhealth</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-performance-sink-performancesinkresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-performance-sink-performancesinksnapshot</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-precompiles-perps-errors</code> | notes | decision.6 (spec/layerx-merge/spec.kvx:43) requires Error(string) with LXP_&lt;code&gt; on 0x1015, while the perps precompile reverts with 4-byte custom errors that this surface decodes |
| <code>interface.14.ts-web-api-precompiles-perps-errors</code> | overridden_by | override.J53.1 |
| <code>interface.14.ts-web-api-precompiles-perpspreflight</code> | notes | decision.6 (spec/layerx-merge/spec.kvx:43) requires Error(string) with LXP_&lt;code&gt; on 0x1015, while the perps precompile reverts with 4-byte custom errors that this surface decodes |
| <code>interface.14.ts-web-api-precompiles-perpspreflight</code> | overridden_by | override.J53.3 |
| <code>interface.14.ts-web-api-precompiles-perpsrevertreason</code> | notes | decision.6 (spec/layerx-merge/spec.kvx:43) requires Error(string) with LXP_&lt;code&gt; on 0x1015, while the perps precompile reverts with 4-byte custom errors that this surface decodes |
| <code>interface.14.ts-web-api-precompiles-perpsrevertreason</code> | overridden_by | override.J53.2 |
| <code>interface.14.ts-web-api-provider-eip1193provider-j53</code> | notes | moved from apps/web/src/journeys/custody/handoff.ts:89 with the request method only |
| <code>interface.14.ts-web-api-provider-windowwalletprovider-j53</code> | notes | moved from apps/web/src/journeys/custody/handoff.ts:278 so the custody module can be deleted; callers listed import it from handoff at HEAD |
| <code>interface.14.ts-web-api-server-serverhumanapicontext</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-stream-humanstreamobserver</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-api-wallet-precompilesendoutcome</code> | notes | returns only transactionHash (apps/web/src/api/wallet.ts:12); stale task 8.5.20 do_4/do_5 and req.8031 ac_3 require the verified receipt and logs beside the hash |
| <code>interface.14.ts-web-api-wallet-resumewalletsponsoredsubmission</code> | notes | stale task 8.5.20 do_5 makes resume read pax_sequencerReceipt for the retained hash; at HEAD it returns PrecompileSendOutcome with transactionHash only |
| <code>interface.14.ts-web-api-wallet-sendwalletprecompilecall</code> | notes | returns only transactionHash (apps/web/src/api/wallet.ts:12); stale task 8.5.20 do_4/do_5 and req.8031 ac_3 require the verified receipt and logs beside the hash |
| <code>interface.14.ts-web-api-wallet-sendwalletsponsoredbatch</code> | notes | returns only transactionHash (apps/web/src/api/wallet.ts:12); stale task 8.5.20 do_4/do_5 and req.8031 ac_3 require the verified receipt and logs beside the hash |

Contract context <code>contract_context.14.web-gateway-receipts</code>:

| Field | Frozen rule |
|---|---|
| interface | ethTransactionReceipt and the receipt and reverted arms of PrecompileSendOutcome in apps/web/src/api |
| storage | none |
| errors | GatewayUnavailableError for relay failures and malformed receipts; status 0x0 is a reverted outcome, not an error |
| events | none |
| gas | none |
| activation | wave 2 gateway read, wave 3 wallet and gas-station callers, wave 5 rendering |
| invariants | a sent outcome never claims success before a receipt with status 0x1; receipts are read only through the eth_ relay |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.3.1</code> | req | 14003.1 |
| <code>acceptance.14.3.1</code> | scenario | Given a window whose ethereum object has a request function, when window_wallet_provider_reads_window_ethereum calls windowWalletProvider, then that object is returned |
| <code>acceptance.14.3.1</code> | test | apps/web/e2e/wallet-provider.test.ts::window_wallet_provider_reads_window_ethereum (planned) |
| <code>acceptance.14.3.1</code> | qualification | UNRUN |
| <code>req.14003</code> | ac_1 | windowWalletProvider from apps/web/src/api/provider.ts (planned) returns window.ethereum when it exposes a request function. |
| <code>acceptance.14.3.2</code> | req | 14003.2 |
| <code>acceptance.14.3.2</code> | scenario | Given no window, and then an ethereum object without a callable request, when window_wallet_provider_is_undefined_without_request calls windowWalletProvider, then undefined is returned both times |
| <code>acceptance.14.3.2</code> | test | apps/web/e2e/wallet-provider.test.ts::window_wallet_provider_is_undefined_without_request (planned) |
| <code>acceptance.14.3.2</code> | qualification | UNRUN |
| <code>req.14003</code> | ac_2 | windowWalletProvider returns undefined when window or window.ethereum.request is missing, as the moved code at apps/web/src/journeys/custody/handoff.ts:278 does. |
| <code>acceptance.14.34.1</code> | req | 14034.1 |
| <code>acceptance.14.34.1</code> | scenario | Given a loopback gateway answering a receipt with status 0x1, blockNumber 0x10 and two logs, when eth_transaction_receipt_decodes_status_and_logs reads it, then status is success, blockNumber is 16n and two logs come back |
| <code>acceptance.14.34.1</code> | test | apps/web/e2e/gateway-receipt.test.ts::eth_transaction_receipt_decodes_status_and_logs (planned) |
| <code>acceptance.14.34.1</code> | qualification | UNRUN |
| <code>req.14034</code> | ac_1 | ethTransactionReceipt (apps/web/src/api/gateway.ts (planned)) sends eth_getTransactionReceipt to LAYERX_GATEWAY_URL and returns status success, the block number as bigint and every log, because the hosted gateway relays the eth_ namespace (services/platform/hosted/gateway/src/evm.rs:12). |
| <code>acceptance.14.34.2</code> | req | 14034.2 |
| <code>acceptance.14.34.2</code> | scenario | Given a gateway answering null and then a receipt with status 0x2, when eth_transaction_receipt_is_undefined_while_pending reads both, then the first is undefined and the second throws GatewayUnavailableError |
| <code>acceptance.14.34.2</code> | test | apps/web/e2e/gateway-receipt.test.ts::eth_transaction_receipt_is_undefined_while_pending (planned) |
| <code>acceptance.14.34.2</code> | qualification | UNRUN |
| <code>req.14034</code> | ac_2 | ethTransactionReceipt (planned) returns undefined for a null result and throws GatewayUnavailableError (apps/web/src/api/gateway.ts:21) for a status of 0x2. |
| <code>acceptance.14.35.1</code> | req | 14035.1 |
| <code>acceptance.14.35.1</code> | scenario | Given a wallet stub returning hash 0xab..ab and a gateway answering a success receipt with one log, when wallet_send_reports_receipt_and_logs sends a precompile call, then the outcome is sent and receipt.logs has one entry |
| <code>acceptance.14.35.1</code> | test | apps/web/e2e/wallet-receipt.test.ts::wallet_send_reports_receipt_and_logs (planned) |
| <code>acceptance.14.35.1</code> | qualification | UNRUN |
| <code>req.14035</code> | ac_1 | sendWalletPrecompileCall (apps/web/src/api/wallet.ts:53) returns outcome sent with the transaction hash and the receipt logs the gateway reports for it, as stale req.8031 ac_3 requires. |
| <code>acceptance.14.35.2</code> | req | 14035.2 |
| <code>acceptance.14.35.2</code> | scenario | Given a gateway answering a receipt with status 0x0, when wallet_send_reports_reverted_receipt sends a call, then the outcome is reverted and receipt.status is reverted |
| <code>acceptance.14.35.2</code> | test | apps/web/e2e/wallet-receipt.test.ts::wallet_send_reports_reverted_receipt (planned) |
| <code>acceptance.14.35.2</code> | qualification | UNRUN |
| <code>req.14035</code> | ac_2 | A receipt with status 0x0 yields outcome reverted carrying the receipt instead of the sent outcome at apps/web/src/api/wallet.ts:85. |
| <code>acceptance.14.36.1</code> | req | 14036.1 |
| <code>acceptance.14.36.1</code> | scenario | Given a quote server answering 429 with error budget_exhausted, when gas_quote_reports_budget_exhausted requests a quote, then the outcome carries refusal budget_exhausted |
| <code>acceptance.14.36.1</code> | test | apps/web/e2e/gas-station-receipt.test.ts::gas_quote_reports_budget_exhausted (planned) |
| <code>acceptance.14.36.1</code> | qualification | UNRUN |
| <code>req.14036</code> | ac_1 | requestSidioraGasQuote returns ok false with refusal budget_exhausted when the station answers {"error":"budget_exhausted"}, as services/agent/sdk/typescript/src/gas-station.ts:500 classifies it. |
| <code>acceptance.14.36.2</code> | req | 14036.2 |
| <code>acceptance.14.36.2</code> | scenario | Given a submit server answering a transactionHash and a gateway answering its success receipt, when sponsored_submit_returns_receipt submits, then the outcome is sent with receipt.status success |
| <code>acceptance.14.36.2</code> | test | apps/web/e2e/gas-station-receipt.test.ts::sponsored_submit_returns_receipt (planned) |
| <code>acceptance.14.36.2</code> | qualification | UNRUN |
| <code>req.14036</code> | ac_2 | sendSponsoredBatch (apps/web/src/api/gas-station.ts:81) returns outcome sent with the submit answer's transactionHash and the gateway receipt for it. |
| <code>acceptance.14.37.1</code> | req | 14037.1 |
| <code>acceptance.14.37.1</code> | scenario | Given a wallet module stub returning a hash and a gateway answering its receipt, when sponsored_batch_reports_receipt submits a quoted batch, then the outcome carries the receipt |
| <code>acceptance.14.37.1</code> | test | apps/web/e2e/wallet-sponsored-receipt.test.ts::sponsored_batch_reports_receipt (planned) |
| <code>acceptance.14.37.1</code> | qualification | UNRUN |
| <code>req.14037</code> | ac_1 | sendWalletSponsoredBatch (apps/web/src/api/wallet.ts:206) returns sent with the receipt of the submitted hash. |
| <code>acceptance.14.37.2</code> | req | 14037.2 |
| <code>acceptance.14.37.2</code> | scenario | Given a retained submission whose resume report is confirmed, when resumed_submission_reports_receipt resumes it, then the outcome is sent with the gateway receipt |
| <code>acceptance.14.37.2</code> | test | apps/web/e2e/wallet-sponsored-receipt.test.ts::resumed_submission_reports_receipt (planned) |
| <code>acceptance.14.37.2</code> | qualification | UNRUN |
| <code>req.14037</code> | ac_2 | resumeWalletSponsoredSubmission (apps/web/src/api/wallet.ts:178) returns sent with the receipt once the retained submission is confirmed. |
| <code>acceptance.14.4.1</code> | req | 14004.1 |
| <code>acceptance.14.4.1</code> | scenario | Given the edited sources, when no_client_module_imports_custody_handoff lists the import specifiers of every .ts and .tsx file under apps/web/src/app and apps/web/src/settings, then none ends in custody/handoff or custody/handoff.ts |
| <code>acceptance.14.4.1</code> | test | apps/web/e2e/provider-imports.test.ts::no_client_module_imports_custody_handoff (planned) |
| <code>acceptance.14.4.1</code> | qualification | UNRUN |
| <code>req.14004</code> | ac_1 | No module under apps/web/src/app or apps/web/src/settings imports journeys/custody/handoff, which apps/web/src/app/exchange/exchange-client.tsx:17 does at HEAD. |
| <code>acceptance.14.4.2</code> | req | 14004.2 |
| <code>acceptance.14.4.2</code> | scenario | Given the edited sources, when exchange_and_launchpad_clients_load_without_custody walks the relative imports from both client modules, then no visited path lies under journeys/custody |
| <code>acceptance.14.4.2</code> | test | apps/web/e2e/provider-imports.test.ts::exchange_and_launchpad_clients_load_without_custody (planned) |
| <code>acceptance.14.4.2</code> | qualification | UNRUN |
| <code>req.14004</code> | ac_2 | The import graph of apps/web/src/app/exchange/exchange-client.tsx:1 and apps/web/src/app/launchpad/launchpad-client.tsx:1 contains no file under apps/web/src/journeys/custody. |
| <code>acceptance.14.54.1</code> | req | 14054.1 |
| <code>acceptance.14.54.1</code> | scenario | Given the regenerated client, when generated_client_reports_schema_minor_10 imports schemaVersion, then it deep-equals major 1 minor 10 |
| <code>acceptance.14.54.1</code> | test | apps/web/e2e/api-schema-version.test.ts::generated_client_reports_schema_minor_10 (planned) |
| <code>acceptance.14.54.1</code> | qualification | UNRUN |
| <code>req.14054</code> | ac_1 | schemaVersion exported by apps/web/src/api/generated/index.ts:4 equals { major: 1, minor: 10 }. |
| <code>acceptance.14.54.2</code> | req | 14054.2 |
| <code>acceptance.14.54.2</code> | scenario | Given the regenerated client, when generated_client_has_no_binding_operations calls createHumanApiClient with a stub fetch, then none of the seven method names is a property of the client |
| <code>acceptance.14.54.2</code> | test | apps/web/e2e/api-schema-version.test.ts::generated_client_has_no_binding_operations (planned) |
| <code>acceptance.14.54.2</code> | qualification | UNRUN |
| <code>req.14054</code> | ac_2 | The client from createHumanApiClient (apps/web/src/api/generated/index.ts:5662) has no bindingSubmit, bindingStatus, bindingStatement, bindingRebind, depositStart, withdrawStart or exitStart method, the operations retired by the minor-10 schema. |
| <code>capability.14.0.2</code> | title | Gateway receipt read |
| <code>capability.14.0.2</code> | reqs | 14034.1<br>14034.2 |
| <code>capability.14.0.4</code> | title | Generated client at schema minor 10 |
| <code>capability.14.0.4</code> | reqs | 14054.1<br>14054.2 |
| <code>capability.14.11.1</code> | title | Wallet provider in the api layer |
| <code>capability.14.11.1</code> | reqs | 14003.1<br>14003.2 |
| <code>capability.14.11.2</code> | title | Clients on the api wallet provider |
| <code>capability.14.11.2</code> | reqs | 14004.1<br>14004.2 |
| <code>capability.14.4.1</code> | title | Wallet send receipts |
| <code>capability.14.4.1</code> | reqs | 14035.1<br>14035.2 |
| <code>capability.14.4.2</code> | title | Gas-station refusals and receipts |
| <code>capability.14.4.2</code> | reqs | 14036.1<br>14036.2 |
| <code>capability.14.4.3</code> | title | Sponsored submission receipts |
| <code>capability.14.4.3</code> | reqs | 14037.1<br>14037.2 |
| <code>resolution.17.941</code> | title | decision.1403 binds: Send receipts are read with eth_getTransactionReceipt |
| <code>resolution.17.941</code> | applies_to | decision.1403<br>task.14.0.2<br>task.14.0.4<br>task.14.4.1<br>task.14.4.2<br>task.14.4.3 |
| <code>resolution.17.941</code> | source | decision.1403 in fragments/J53/decision.kvx; tasks from affects |
| <code>resolution.17.944</code> | title | decision.1406 binds: Custody journeys and wallet binding leave the web app |
| <code>resolution.17.944</code> | applies_to | decision.1406<br>task.14.0.3<br>task.14.7.1<br>task.14.7.2<br>task.14.7.3<br>task.14.7.4<br>task.14.7.5<br>task.14.8.1<br>task.14.8.2<br>task.14.10.2<br>task.14.11.1 |
| <code>resolution.17.944</code> | source | decision.1406 in fragments/J53/decision.kvx; tasks from affects |

## app

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-app-app-agents-agentid-error-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/[agentId]/error.tsx:5</code> | <code>export default function AgentDetailRouteError({ error, reset, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-app-app-agents-agentid-loading-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/[agentId]/loading.tsx:3</code> | <code>export default function AgentDetailRouteLoading()</code> |
| <code>interface.14.ts-web-app-app-agents-error-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/error.tsx:5</code> | <code>export default function AgentsRouteError({ error, reset, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-app-app-agents-loading-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/loading.tsx:3</code> | <code>export default function AgentsRouteLoading()</code> |
| <code>interface.14.ts-web-app-app-agents-new-error-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/new/error.tsx:5</code> | <code>export default function AgentCreateRouteError({ error, reset, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-app-app-agents-new-loading-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/new/loading.tsx:3</code> | <code>export default function AgentCreateRouteLoading()</code> |
| <code>interface.14.ts-web-app-app-error-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/error.tsx:5</code> | <code>export default function AppRouteError({ error, reset, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-app-app-layout-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/layout.tsx:14</code> | <code>export default async function AppPlaneLayout({ children }: Readonly&lt;{ children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-app-app-loading-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/loading.tsx:4</code> | <code>export default function AppLoading()</code> |
| <code>interface.14.ts-web-app-app-support-error-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/support/error.tsx:5</code> | <code>export default function SupportRouteError({ error, reset, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-app-app-support-loading-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/app/support/loading.tsx:3</code> | <code>export default function SupportRouteLoading()</code> |
| <code>interface.14.ts-web-app-bridge-bridge-client-bridgeoutform</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/bridge/bridge-client.tsx:82</code> | <code>export function BridgeOutForm({ account, chain, asset, closedReason, maxPerTx, headroom, }: Readonly&lt;{ account: string &#124; undefined; chain: string; asset: string &#124; undefined; closedReason: string &#124; undefined;</code> |
| <code>interface.14.ts-web-app-bridge-bridge-client-bridgestatuspolling</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/bridge/bridge-client.tsx:19</code> | <code>export function BridgeStatusPolling({ active }: Readonly&lt;{ active: boolean }&gt;)</code> |
| <code>interface.14.ts-web-app-bridge-bridge-client-bridgetrackform</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/bridge/bridge-client.tsx:35</code> | <code>export function BridgeTrackForm({ account, chain, asset, }: Readonly&lt;{ account: string &#124; undefined; chain: string; asset: string &#124; undefined }&gt;)</code> |
| <code>interface.14.ts-web-app-error-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/error.tsx:5</code> | <code>export default function RootRouteError({ error, reset, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-app-exchange-actions-perpspreflightanswer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/exchange/actions.ts:6</code> | <code>export type PerpsPreflightAnswer = Readonly&lt;{ ok: true }&gt; &#124; Readonly&lt;{ ok: false; detail: string }&gt;;</code> |
| <code>interface.14.ts-web-app-exchange-actions-preflightperpswrite</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/exchange/actions.ts:12</code> | <code>export async function preflightPerpsWrite(from: string, data: string): Promise&lt;PerpsPreflightAnswer&gt;</code> |
| <code>interface.14.ts-web-app-exchange-exchange-client-exchangeactions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/exchange/exchange-client.tsx:72</code> | <code>export function ExchangeActions({ account, collateral, markets, positions, orders, }: Readonly&lt;{ account: string &#124; undefined; collateral: PerpsCollateral; markets: readonly Choice[]; positions: readonly Choice[]; orders: readonly Choice[];</code> |
| <code>interface.14.ts-web-app-exchange-exchange-client-perpscollateral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/exchange/exchange-client.tsx:29</code> | <code>export type PerpsCollateral = Readonly&lt;{ token: string; decimals: number; symbol: string }&gt;;</code> |
| <code>interface.14.ts-web-app-exchange-exchange-client-sendperpswrite</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/exchange/exchange-client.tsx:36</code> | <code>export async function sendPerpsWrite( from: string, collateral: PerpsCollateral, pull: bigint, call: PrecompileCall, onApproving: (active: boolean) =&gt; void, ): Promise&lt;PrecompileSendOutcome&gt;</code> |
| <code>interface.14.ts-web-app-explorer-error-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/explorer/error.tsx:5</code> | <code>export default function ExplorerRouteError({ error, reset, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-app-explorer-layout-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/explorer/layout.tsx:6</code> | <code>export default function ExplorerPlaneLayout({ children }: Readonly&lt;{ children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-app-explorer-loading-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/explorer/loading.tsx:4</code> | <code>export default function ExplorerLoading()</code> |
| <code>interface.14.ts-web-app-global-error-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/global-error.tsx:5</code> | <code>export default function GlobalError({ error, reset, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-app-launchpad-actions-launchpadquoteanswer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/launchpad/actions.ts:6</code> | <code>export type LaunchpadQuoteAnswer =</code> |
| <code>interface.14.ts-web-app-launchpad-actions-quotelaunchpadswap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/launchpad/actions.ts:18</code> | <code>export async function quoteLaunchpadSwap(token: string, side: string, amountIn: string): Promise&lt;LaunchpadQuoteAnswer&gt;</code> |
| <code>interface.14.ts-web-app-launchpad-launchpad-client-launchpadactions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/launchpad/launchpad-client.tsx:79</code> | <code>export function LaunchpadActions({ account, quoteDenom, markets, feeStrategies, }: Readonly&lt;{ account: string &#124; undefined; quoteDenom: string; markets: readonly Readonly&lt;{ token: string; pool: string; label: string; paused: boolean }&gt;[]; feeStrategies: readonly string[]; }&gt;)</code> |
| <code>interface.14.ts-web-app-launchpad-launchpad-client-sendwithapproval</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/launchpad/launchpad-client.tsx:48</code> | <code>export async function sendWithApproval( from: string, spend: LaunchpadSpend, build: () =&gt; PrecompileCall, onApproving: (active: boolean) =&gt; void, ): Promise&lt;PrecompileSendOutcome&gt;</code> |
| <code>interface.14.ts-web-app-launchpad-launchpad-client-sponsored-approve-unsupported</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/launchpad/launchpad-client.tsx:42</code> | <code>export const SPONSORED_APPROVE_UNSUPPORTED = "the allowance approve cannot be sponsored; choose the PAX gas fee for it";</code> |
| <code>interface.14.ts-web-app-layout-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/layout.tsx:21</code> | <code>export default function RootLayout({ children }: Readonly&lt;{ children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-app-layout-metadata</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/layout.tsx:10</code> | <code>export const metadata: Metadata =</code> |
| <code>interface.14.ts-web-app-layout-viewport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/layout.tsx:14</code> | <code>export const viewport: Viewport =</code> |
| <code>interface.14.ts-web-app-loading-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/loading.tsx:3</code> | <code>export default function RootRouteLoading()</code> |
| <code>interface.14.ts-web-app-markets-activity-feed-activityfeed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/activity-feed.tsx:41</code> | <code>export async function ActivityFeed({ route, account, side, cursor, kind, }: Readonly&lt;{ route: string; account: string &#124; undefined; side: ActivitySide; cursor: string &#124; undefined; kind: string &#124; undefined;</code> |
| <code>interface.14.ts-web-app-markets-activity-feed-activityside</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/activity-feed.tsx:15</code> | <code>export type ActivitySide = "unified" &#124; "paxeer";</code> |
| <code>interface.14.ts-web-app-markets-activity-feed-activityside-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/activity-feed.tsx:17</code> | <code>export function activitySide(value: string &#124; undefined): ActivitySide</code> |
| <code>interface.14.ts-web-app-markets-availability-availability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/availability.ts:4</code> | <code>export function availability(...reasons: readonly (string &#124; false &#124; undefined)[]): ControlAvailability</code> |
| <code>interface.14.ts-web-app-markets-capability-action-readsurfacecapability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/capability-action.ts:10</code> | <code>export async function readSurfaceCapability(surface: string): Promise&lt;SurfaceCapability&gt;</code> |
| <code>interface.14.ts-web-app-markets-capability-surfacecapabilitygate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/capability.tsx:23</code> | <code>export function SurfaceCapabilityGate({ surface, live, detail, }: Readonly&lt;{ surface: ForkSurface; live: boolean; detail: string &#124; null }&gt;)</code> |
| <code>interface.14.ts-web-app-markets-format-firstparam</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:49</code> | <code>export function firstParam(value: string &#124; string[] &#124; undefined): string &#124; undefined</code> |
| <code>interface.14.ts-web-app-markets-format-formatunits</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:11</code> | <code>export function formatUnits(amount: bigint, decimals: number): string</code> |
| <code>interface.14.ts-web-app-markets-format-isbytes32</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:45</code> | <code>export function isBytes32(value: string &#124; undefined): value is string</code> |
| <code>interface.14.ts-web-app-markets-format-isevmaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:41</code> | <code>export function isEvmAddress(value: string &#124; undefined): value is string</code> |
| <code>interface.14.ts-web-app-markets-format-launchpad-decimals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:8</code> | <code>export const LAUNCHPAD_DECIMALS = 6;</code> |
| <code>interface.14.ts-web-app-markets-format-marketsearchparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:53</code> | <code>export type MarketSearchParams = Readonly&lt;Record&lt;string, string &#124; string[] &#124; undefined&gt;&gt;;</code> |
| <code>interface.14.ts-web-app-markets-format-parseunits</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:24</code> | <code>export function parseUnits(text: string, decimals: number): bigint &#124; undefined</code> |
| <code>interface.14.ts-web-app-markets-format-shortid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:37</code> | <code>export function shortId(value: string): string</code> |
| <code>interface.14.ts-web-app-markets-format-wei-decimals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/format.ts:5</code> | <code>export const WEI_DECIMALS = 18;</code> |
| <code>interface.14.ts-web-app-markets-frame-marketframe</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/frame.tsx:7</code> | <code>export function MarketFrame({ title, description, account, children, }: Readonly&lt;{ title: string; description: string; account: string &#124; undefined; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-app-markets-frame-marketunavailable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/frame.tsx:24</code> | <code>export function MarketUnavailable({ detail }: Readonly&lt;{ detail: string }&gt;)</code> |
| <code>interface.14.ts-web-app-markets-navigation-market-routes</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/navigation.tsx:3</code> | <code>export const MARKET_ROUTES = [</code> |
| <code>interface.14.ts-web-app-markets-navigation-marketnavigation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/navigation.tsx:9</code> | <code>export function MarketNavigation({ account }: Readonly&lt;{ account: string &#124; undefined }&gt;)</code> |
| <code>interface.14.ts-web-app-markets-select-choicefield</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/select.tsx:3</code> | <code>export function ChoiceField({ label, value, options, onChange, }: Readonly&lt;{ label: string; value: string; options: readonly Readonly&lt;{ value: string; label: string; disabled?: boolean }&gt;[]; onChange: (value: string) =&gt; void; }&gt;)</code> |
| <code>interface.14.ts-web-app-markets-send-outcome-sendoutcome</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/app/_markets/send-outcome.tsx:4</code> | <code>export function SendOutcome({ outcome }: Readonly&lt;{ outcome: PrecompileSendOutcome &#124; undefined }&gt;)</code> |
| <code>interface.14.ts-web-app-markets-wallet-connect-walletconnect</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/_markets/wallet-connect.tsx:11</code> | <code>export function WalletConnect({ account }: Readonly&lt;{ account: string &#124; undefined }&gt;)</code> |
| <code>interface.14.ts-web-app-scaffold-human-web-app-scaffold</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/app/scaffold.ts:3</code> | <code>export function human_web_app_scaffold()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-app-app-agents-agentid-error-default</code> | notes | Next.js framework entry (error) for /app/agents/{agentId} |
| <code>interface.14.ts-web-app-app-agents-agentid-loading-default</code> | notes | Next.js framework entry (loading) for /app/agents/{agentId} |
| <code>interface.14.ts-web-app-app-agents-error-default</code> | notes | Next.js framework entry (error) for /app/agents |
| <code>interface.14.ts-web-app-app-agents-loading-default</code> | notes | Next.js framework entry (loading) for /app/agents |
| <code>interface.14.ts-web-app-app-agents-new-error-default</code> | notes | Next.js framework entry (error) for /app/agents/new |
| <code>interface.14.ts-web-app-app-agents-new-loading-default</code> | notes | Next.js framework entry (loading) for /app/agents/new |
| <code>interface.14.ts-web-app-app-error-default</code> | notes | Next.js framework entry (error) for /app |
| <code>interface.14.ts-web-app-app-layout-default</code> | notes | Next.js framework entry (layout) for /app |
| <code>interface.14.ts-web-app-app-loading-default</code> | notes | Next.js framework entry (loading) for /app |
| <code>interface.14.ts-web-app-app-support-error-default</code> | notes | Next.js framework entry (error) for /app/support |
| <code>interface.14.ts-web-app-app-support-loading-default</code> | notes | Next.js framework entry (loading) for /app/support |
| <code>interface.14.ts-web-app-error-default</code> | notes | Next.js framework entry (error) for / |
| <code>interface.14.ts-web-app-exchange-actions-perpspreflightanswer</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-app-explorer-error-default</code> | notes | Next.js framework entry (error) for /explorer |
| <code>interface.14.ts-web-app-explorer-layout-default</code> | notes | Next.js framework entry (layout) for /explorer |
| <code>interface.14.ts-web-app-explorer-loading-default</code> | notes | Next.js framework entry (loading) for /explorer |
| <code>interface.14.ts-web-app-global-error-default</code> | notes | Next.js framework entry (global-error) for / |
| <code>interface.14.ts-web-app-layout-default</code> | notes | Next.js framework entry (layout) for / |
| <code>interface.14.ts-web-app-layout-metadata</code> | notes | Next.js framework entry (layout) for / |
| <code>interface.14.ts-web-app-layout-viewport</code> | notes | Next.js framework entry (layout) for / |
| <code>interface.14.ts-web-app-loading-default</code> | notes | Next.js framework entry (loading) for / |
| <code>interface.14.ts-web-app-markets-activity-feed-activityside</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-app-markets-send-outcome-sendoutcome</code> | overridden_by | override.J53.5 |
| <code>interface.14.ts-web-app-scaffold-human-web-app-scaffold</code> | notes | no in-repo importer at HEAD |

Contract context <code>contract_context.14.web-gateway-receipts</code>:

| Field | Frozen rule |
|---|---|
| interface | ethTransactionReceipt and the receipt and reverted arms of PrecompileSendOutcome in apps/web/src/api |
| storage | none |
| errors | GatewayUnavailableError for relay failures and malformed receipts; status 0x0 is a reverted outcome, not an error |
| events | none |
| gas | none |
| activation | wave 2 gateway read, wave 3 wallet and gas-station callers, wave 5 rendering |
| invariants | a sent outcome never claims success before a receipt with status 0x1; receipts are read only through the eth_ relay |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.2.1</code> | req | 14002.1 |
| <code>acceptance.14.2.1</code> | scenario | Given the catalog with the new key, when copy_has_operation_retired_key resolves error.operation.retired, then the message is non-empty and contains no BANNED_VOCABULARY word |
| <code>acceptance.14.2.1</code> | test | apps/web/e2e/copy-retired-operation.test.ts::copy_has_operation_retired_key (planned) |
| <code>acceptance.14.2.1</code> | qualification | UNRUN |
| <code>req.14002</code> | ac_1 | copyEntry('error.operation.retired') resolves to the sentence added after apps/web/copy/catalog.ts:102, free of banned vocabulary. |
| <code>acceptance.14.2.2</code> | req | 14002.2 |
| <code>acceptance.14.2.2</code> | scenario | Given the regenerated runtime table, when copy_runtime_matches_catalog_after_retired_key compares it with copyEntries(), then the key sets are equal and include error.operation.retired |
| <code>acceptance.14.2.2</code> | test | apps/web/e2e/copy-retired-operation.test.ts::copy_runtime_matches_catalog_after_retired_key (planned) |
| <code>acceptance.14.2.2</code> | qualification | UNRUN |
| <code>req.14002</code> | ac_2 | apps/web/copy/messages.generated.ts:1 lists exactly the keys of copyEntries() once the key is added. |
| <code>acceptance.14.38.1</code> | req | 14038.1 |
| <code>acceptance.14.38.1</code> | scenario | Given a sent outcome whose receipt has blockNumber 42n and two logs, when send_outcome_renders_receipt_block renders it, then the markup contains 42 and the markets.send.receipt copy |
| <code>acceptance.14.38.1</code> | test | apps/web/e2e/send-outcome.test.ts::send_outcome_renders_receipt_block (planned) |
| <code>acceptance.14.38.1</code> | qualification | UNRUN |
| <code>req.14038</code> | ac_1 | The send outcome view of apps/web/src/app/_markets/send-outcome.tsx:1 renders the receipt block number for a sent outcome with a receipt. |
| <code>acceptance.14.38.2</code> | req | 14038.2 |
| <code>acceptance.14.38.2</code> | scenario | Given a reverted outcome, when send_outcome_renders_reverted renders it, then the markup contains the reverted copy and not the sent copy |
| <code>acceptance.14.38.2</code> | test | apps/web/e2e/send-outcome.test.ts::send_outcome_renders_reverted (planned) |
| <code>acceptance.14.38.2</code> | qualification | UNRUN |
| <code>req.14038</code> | ac_2 | SendOutcome (apps/web/src/app/_markets/send-outcome.tsx:4) renders the markets.send.reverted copy and no success state for a reverted outcome. |
| <code>acceptance.14.47.1</code> | req | 14047.1 |
| <code>acceptance.14.47.1</code> | scenario | Given the edited matrix and budgets, when state_matrix_has_no_custody_routes looks up the six routes, then no matrix row exists and classifyPerformanceRoute returns undefined for each |
| <code>acceptance.14.47.1</code> | test | apps/web/e2e/retired-routes.test.ts::state_matrix_has_no_custody_routes (planned) |
| <code>acceptance.14.47.1</code> | qualification | UNRUN |
| <code>req.14047</code> | ac_1 | Neither the state matrix (apps/web/src/states/matrix.ts:106-110 at HEAD) nor classifyPerformanceRoute (apps/web/src/perf/budgets.ts:36) knows /app/deposit, /app/withdraw, /app/move/withdraw, /app/journeys/[journeyId], its claim route or /app/settings/exit. |
| <code>acceptance.14.61.1</code> | req | 14061.1 |
| <code>acceptance.14.61.1</code> | scenario | Given the edited catalogue, when binding_copy_keys_are_gone looks up the four keys, then the three binding keys throw and settings.wallet.title resolves |
| <code>acceptance.14.61.1</code> | test | apps/web/e2e/copy-binding-retired.test.ts::binding_copy_keys_are_gone (planned) |
| <code>acceptance.14.61.1</code> | qualification | UNRUN |
| <code>req.14061</code> | ac_1 | copyEntry from apps/web/copy/runtime.ts:1 throws for settings.wallet.bound, settings.wallet.rebinding and settings.wallet.handoff.failed.body and returns the Payout wallet message for settings.wallet.title. |
| <code>acceptance.14.61.2</code> | req | 14061.2 |
| <code>acceptance.14.61.2</code> | scenario | Given the edited source and catalogue, when every_settings_wallet_key_in_source_is_catalogued scans apps/web/src for settings.wallet. literals, then copyEntry resolves each one |
| <code>acceptance.14.61.2</code> | test | apps/web/e2e/copy-binding-retired.test.ts::every_settings_wallet_key_in_source_is_catalogued (planned) |
| <code>acceptance.14.61.2</code> | qualification | UNRUN |
| <code>req.14061</code> | ac_2 | Every settings.wallet.* string literal left under apps/web/src (apps/web/src/settings/settings-screen.tsx:380 at HEAD) resolves through copyEntry. |
| <code>acceptance.14.62.1</code> | req | 14062.1 |
| <code>acceptance.14.62.1</code> | scenario | Given the edited catalogue, when retired_verification_copy_keys_are_gone looks up the four keys, then each lookup throws |
| <code>acceptance.14.62.1</code> | test | apps/web/e2e/copy-verify-retired.test.ts::retired_verification_copy_keys_are_gone (planned) |
| <code>acceptance.14.62.1</code> | qualification | UNRUN |
| <code>req.14062</code> | ac_1 | copyEntry from apps/web/copy/runtime.ts:1 throws for explorer.verify.title, explorer.verification.settlement_anchored, verification.paxeer_finalised and activity.verification.paxeer_finalised. |
| <code>acceptance.14.62.2</code> | req | 14062.2 |
| <code>acceptance.14.62.2</code> | scenario | Given the edited catalogue, when explorer_verify_unavailable_key_is_kept looks up both keys, then both resolve to non-empty messages |
| <code>acceptance.14.62.2</code> | test | apps/web/e2e/copy-verify-retired.test.ts::explorer_verify_unavailable_key_is_kept (planned) |
| <code>acceptance.14.62.2</code> | qualification | UNRUN |
| <code>req.14062</code> | ac_2 | copyEntry resolves explorer.verify.unavailable, read at apps/web/src/app/explorer/accounts/[accountId]/page.tsx:137, and verification.receipt_verified. |
| <code>acceptance.14.999.1</code> | req | 14999.1 |
| <code>acceptance.14.999.1</code> | scenario | Given the candidate after every domain-14 wave-6 task, when web_completion_retired_surfaces_absent runs, then no retired surface remains and every planned path exists |
| <code>acceptance.14.999.1</code> | test | apps/web/e2e/completion-gate.test.ts::web_completion_retired_surfaces_absent (planned) |
| <code>acceptance.14.999.1</code> | qualification | UNRUN |
| <code>req.14999</code> | ac_1 | web_completion_retired_surfaces_absent passes on the merged candidate: no retired domain-14 surface remains and every planned domain-14 wave-6 path exists. |
| <code>capability.14.0.3</code> | title | Market pages show receipts |
| <code>capability.14.0.3</code> | reqs | 14038.1<br>14038.2 |
| <code>capability.14.0.9</code> | title | Domain 14 completion gate |
| <code>capability.14.0.9</code> | reqs | 14999.1 |
| <code>capability.14.10.1</code> | title | Operation-retired copy key |
| <code>capability.14.10.1</code> | reqs | 14002.1<br>14002.2 |
| <code>capability.14.10.2</code> | title | Binding copy removed |
| <code>capability.14.10.2</code> | reqs | 14061.1<br>14061.2 |
| <code>capability.14.10.3</code> | title | Retired verification copy removed |
| <code>capability.14.10.3</code> | reqs | 14062.1<br>14062.2 |
| <code>capability.14.7.1</code> | title | Custody routes removed |
| <code>capability.14.7.1</code> | reqs | 14047.1 |

## auth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-auth-native-fee-budget-feelimitinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/native-fee-budget.ts:3</code> | <code>export interface FeeLimitInput</code> |
| <code>interface.14.ts-web-auth-native-fee-budget-fields-nativefeebudgetfields</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/native-fee-budget-fields.tsx:34</code> | <code>export function NativeFeeBudgetFields({ control, disabled, }: Readonly&lt;{ control: ReturnType&lt;typeof useNativeFeeBudget&gt;; disabled: boolean }&gt;)</code> |
| <code>interface.14.ts-web-auth-native-fee-budget-fields-usenativefeebudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/native-fee-budget-fields.tsx:12</code> | <code>export function useNativeFeeBudget(client: HumanApiClient)</code> |
| <code>interface.14.ts-web-auth-native-fee-budget-nativefeebudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/native-fee-budget.ts:29</code> | <code>export function nativeFeeBudget( input: FeeLimitInput, asset: NativeFeeAsset &#124; undefined, ): NativeFeeBudget &#124; undefined</code> |
| <code>interface.14.ts-web-auth-native-fee-budget-nativefeebudgetidentity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/native-fee-budget.ts:48</code> | <code>export function nativeFeeBudgetIdentity(budget: NativeFeeBudget &#124; undefined): string</code> |
| <code>interface.14.ts-web-auth-native-fee-budget-parsefeeamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/native-fee-budget.ts:18</code> | <code>export function parseFeeAmount(input: string, decimals: number): bigint &#124; undefined</code> |
| <code>interface.14.ts-web-auth-native-fee-budget-validnativefeeasset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/native-fee-budget.ts:11</code> | <code>export function validNativeFeeAsset(asset: NativeFeeAsset): boolean</code> |
| <code>interface.14.ts-web-auth-server-session-validrequestcsrf</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/server-session.ts:28</code> | <code>export function validRequestCsrf(request: Request): boolean</code> |
| <code>interface.14.ts-web-auth-server-session-verifiedwebsession</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/server-session.ts:8</code> | <code>export async function verifiedWebSession(cookie: string): Promise&lt;VerifiedSessionIdentity &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-auth-session-active-account-storage-key</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/session.ts:3</code> | <code>export const ACTIVE_ACCOUNT_STORAGE_KEY = "layerx.account";</code> |
| <code>interface.14.ts-web-auth-session-app-csrf-cookie</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/session.ts:2</code> | <code>export const APP_CSRF_COOKIE = "__Host-layerx_csrf";</code> |
| <code>interface.14.ts-web-auth-session-app-session-cookie</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/session.ts:1</code> | <code>export const APP_SESSION_COOKIE = "__Host-layerx-session";</code> |
| <code>interface.14.ts-web-auth-session-csrftokenfromcookie</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/session.ts:18</code> | <code>export function csrfTokenFromCookie(cookie: string): string &#124; undefined</code> |
| <code>interface.14.ts-web-auth-session-singlecurrentsessionid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/session.ts:11</code> | <code>export function singleCurrentSessionId( sessions: readonly Readonly&lt;{ session_id: string; current: boolean }&gt;[], ): string &#124; undefined</code> |
| <code>interface.14.ts-web-auth-session-verifiedsessionidentity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/session.ts:5</code> | <code>export interface VerifiedSessionIdentity</code> |
| <code>interface.14.ts-web-auth-use-active-account-useactiveaccountid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/auth/use-active-account.ts:16</code> | <code>export function useActiveAccountId(initial?: string): string &#124; undefined</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-auth-session-app-csrf-cookie</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-auth-session-app-session-cookie</code> | notes | no in-repo importer at HEAD |

## design

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-design-tokens-human-design-tokens</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/design/tokens.ts:34</code> | <code>export function human_design_tokens()</code> |
| <code>interface.14.ts-web-design-tokens-layerxuicontract</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/design/tokens.ts:1</code> | <code>export const layerxUiContract = Object.freeze(</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-design-tokens-layerxuicontract</code> | notes | no in-repo importer at HEAD |

## env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-web-human-challenge-window-seconds</code> | <code>env_var</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:30</code> | <code>HUMAN_CHALLENGE_WINDOW_SECONDS</code> |
| <code>interface.14.env-web-human-deposit-delayed-after-seconds</code> | <code>env_var</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:31</code> | <code>HUMAN_DEPOSIT_DELAYED_AFTER_SECONDS</code> |
| <code>interface.14.env-web-human-settlement-checkpoint-interval-seconds</code> | <code>env_var</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:27</code> | <code>HUMAN_SETTLEMENT_CHECKPOINT_INTERVAL_SECONDS</code> |
| <code>interface.14.env-web-human-settlement-paxeer-block-seconds</code> | <code>env_var</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:28</code> | <code>HUMAN_SETTLEMENT_PAXEER_BLOCK_SECONDS</code> |
| <code>interface.14.env-web-human-settlement-required-confirmations</code> | <code>env_var</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:29</code> | <code>HUMAN_SETTLEMENT_REQUIRED_CONFIRMATIONS</code> |
| <code>interface.14.env-web-layerx-explorer-api-origin</code> | <code>env_var</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:59</code> | <code>LAYERX_EXPLORER_API_ORIGIN</code> |
| <code>interface.14.env-web-layerx-explorer-naming-program</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/explorer/client.ts:299</code> | <code>LAYERX_EXPLORER_NAMING_PROGRAM</code> |
| <code>interface.14.env-web-layerx-explorer-program-api-origin</code> | <code>env_var</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:40</code> | <code>LAYERX_EXPLORER_PROGRAM_API_ORIGIN</code> |
| <code>interface.14.env-web-layerx-explorer-program-bearer-token</code> | <code>env_var</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:41</code> | <code>LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN</code> |
| <code>interface.14.env-web-layerx-gateway-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/gateway.ts:41</code> | <code>LAYERX_GATEWAY_URL</code> |
| <code>interface.14.env-web-layerx-human-service-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/server.ts:12</code> | <code>LAYERX_HUMAN_SERVICE_URL</code> |
| <code>interface.14.env-web-layerx-human-web-origin</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/server.ts:27</code> | <code>LAYERX_HUMAN_WEB_ORIGIN</code> |
| <code>interface.14.env-web-layerx-mirror-verifier-config</code> | <code>env_var</code> | <code>change</code> | <code>apps/web/src/explorer/mirror-server.ts:34</code> | <code>LAYERX_MIRROR_VERIFIER_CONFIG</code> |
| <code>interface.14.env-web-layerx-mirror-verifier-executable</code> | <code>env_var</code> | <code>change</code> | <code>apps/web/src/explorer/mirror-server.ts:34</code> | <code>LAYERX_MIRROR_VERIFIER_EXECUTABLE</code> |
| <code>interface.14.env-web-layerx-mirror-verifier-prlimit-executable</code> | <code>env_var</code> | <code>change</code> | <code>apps/web/src/explorer/mirror-server.ts:34</code> | <code>LAYERX_MIRROR_VERIFIER_PRLIMIT_EXECUTABLE</code> |
| <code>interface.14.env-web-layerx-rum-development-fallback</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/performance-sink.ts:117</code> | <code>LAYERX_RUM_DEVELOPMENT_FALLBACK</code> |
| <code>interface.14.env-web-layerx-rum-storage-directory</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/performance-sink.ts:108</code> | <code>LAYERX_RUM_STORAGE_DIRECTORY</code> |
| <code>interface.14.env-web-layerx-support-report-bearer-token</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/server/support-reports.ts:198</code> | <code>LAYERX_SUPPORT_REPORT_BEARER_TOKEN</code> |
| <code>interface.14.env-web-layerx-support-report-dir</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/server/support-reports.ts:48</code> | <code>LAYERX_SUPPORT_REPORT_DIR</code> |
| <code>interface.14.env-web-layerx-support-report-retention</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/server/support-reports.ts:36</code> | <code>LAYERX_SUPPORT_REPORT_RETENTION</code> |
| <code>interface.14.env-web-paxeer-gas-station-chain-id</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/gas-station.ts:27</code> | <code>PAXEER_GAS_STATION_CHAIN_ID</code> |
| <code>interface.14.env-web-paxeer-gas-station-paymaster</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/gas-station.ts:29</code> | <code>PAXEER_GAS_STATION_PAYMASTER</code> |
| <code>interface.14.env-web-paxeer-gas-station-quote-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/gas-station.ts:26</code> | <code>PAXEER_GAS_STATION_QUOTE_URL</code> |
| <code>interface.14.env-web-paxeer-gas-station-sponsor</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/api/gas-station.ts:28</code> | <code>PAXEER_GAS_STATION_SPONSOR</code> |
| <code>interface.14.env-web-paxeer-gas-station-submit-url</code> | <code>env_var</code> | <code>change</code> | <code>apps/web/src/api/gas-station.ts:88</code> | <code>PAXEER_GAS_STATION_SUBMIT_URL</code> |
| <code>interface.14.env-web-paxeer-x-explorer-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/web/src/explorer/client.ts:81</code> | <code>PAXEER_X_EXPLORER_BASE_URL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-web-human-challenge-window-seconds</code> | notes | read only inside the retired journey set (stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013)) |
| <code>interface.14.env-web-human-deposit-delayed-after-seconds</code> | notes | read only inside the retired journey set (stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013)) |
| <code>interface.14.env-web-human-settlement-checkpoint-interval-seconds</code> | notes | read only inside the retired journey set (stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013)) |
| <code>interface.14.env-web-human-settlement-paxeer-block-seconds</code> | notes | read only inside the retired journey set (stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013)) |
| <code>interface.14.env-web-human-settlement-required-confirmations</code> | notes | read only inside the retired journey set (stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013)) |
| <code>interface.14.env-web-layerx-explorer-api-origin</code> | notes | stale req.8045 ac_1 makes PAXEER_X_EXPLORER_BASE_URL (apps/web/src/explorer/client.ts:81) the only origin; this surface reads LAYERX_EXPLORER_API_ORIGIN or LAYERX_EXPLORER_PROGRAM_API_ORIGIN |
| <code>interface.14.env-web-layerx-explorer-program-api-origin</code> | notes | stale req.8045 ac_1 makes PAXEER_X_EXPLORER_BASE_URL (apps/web/src/explorer/client.ts:81) the only origin; this surface reads LAYERX_EXPLORER_API_ORIGIN or LAYERX_EXPLORER_PROGRAM_API_ORIGIN |
| <code>interface.14.env-web-layerx-explorer-program-bearer-token</code> | notes | authenticates the program explorer origin that stale req.8045 ac_1 removes in favour of PAXEER_X_EXPLORER_BASE_URL |
| <code>interface.14.env-web-layerx-mirror-verifier-config</code> | notes | read only by the mirror verifier that stale req.8045 ac_3 removes |
| <code>interface.14.env-web-layerx-mirror-verifier-executable</code> | notes | read only by the mirror verifier that stale req.8045 ac_3 removes |
| <code>interface.14.env-web-layerx-mirror-verifier-prlimit-executable</code> | notes | read only by the mirror verifier that stale req.8045 ac_3 removes |
| <code>interface.14.env-web-paxeer-gas-station-submit-url</code> | notes | the submit answer is read for transactionHash only; stale task 8.5.20 do_4 requires the verified receipt |

## explorer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-explorer-client-accountactivitypage</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:138</code> | <code>export async function accountActivityPage( identifier: string, before?: string, limit = "25", ): Promise&lt;ExplorerPage&lt;AccountActivityRecord&gt;&gt;</code> |
| <code>interface.14.ts-web-explorer-client-explorerlink</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/client.ts:88</code> | <code>export function explorerLink(target: ExplorerLinkTarget): string</code> |
| <code>interface.14.ts-web-explorer-client-explorerunavailableerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/client.ts:32</code> | <code>export class ExplorerUnavailableError extends Error</code> |
| <code>interface.14.ts-web-explorer-client-programrecord</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:160</code> | <code>export async function programRecord(identifier: string): Promise&lt;ProgramRecord &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-explorer-client-receiptrecord</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:125</code> | <code>export async function receiptRecord( identifier: string, ): Promise&lt;ExplorerRecord&lt;ReceiptRecord&gt;&gt;</code> |
| <code>interface.14.ts-web-explorer-client-resolvename</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:306</code> | <code>export async function resolveName(name: string): Promise&lt;NameResolutionRecord &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-explorer-client-unifiedaccount</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:192</code> | <code>export async function unifiedAccount( identifier: string, beforeBlock?: string, before?: string, limit = 25, ): Promise&lt;UnifiedAccountRecord &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-explorer-client-unifiedaccountavailability</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/client.ts:240</code> | <code>export async function unifiedAccountAvailability( identifier: string, beforeBlock?: string, before?: string, limit = 25, ): Promise&lt;UnifiedAccountAvailabilityRecord &#124; undefined&gt;</code> |
| <code>interface.14.ts-web-explorer-client-unifiedaccountunavailableerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/client.ts:291</code> | <code>export class UnifiedAccountUnavailableError extends ExplorerUnavailableError</code> |
| <code>interface.14.ts-web-explorer-client-verifyevidenceupstream</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/explorer/client.ts:343</code> | <code>export async function verifyEvidenceUpstream( request: Readonly&lt;Record&lt;string, unknown&gt;&gt;, ): Promise&lt;EvidenceVerificationReport&gt;</code> |
| <code>interface.14.ts-web-explorer-components-explorer-navigation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/components.tsx:10</code> | <code>export { EXPLORER_NAVIGATION };</code> |
| <code>interface.14.ts-web-explorer-components-explorerframe</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/components.tsx:12</code> | <code>export function ExplorerFrame({ title, description, children, }: Readonly&lt;{ title: ReactNode; description?: ReactNode; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-explorer-components-explorernotfound</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/components.tsx:102</code> | <code>export function ExplorerNotFound({ title, freshness, }: Readonly&lt;{ title: string; freshness: ExplorerFreshness }&gt;)</code> |
| <code>interface.14.ts-web-explorer-components-explorerreceiptpath</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/components.tsx:114</code> | <code>export function explorerReceiptPath(identifier: string): string</code> |
| <code>interface.14.ts-web-explorer-components-explorerunavailable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/components.tsx:91</code> | <code>export function ExplorerUnavailable()</code> |
| <code>interface.14.ts-web-explorer-components-freshnessdisplay</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/components.tsx:37</code> | <code>export function FreshnessDisplay({ freshness }: Readonly&lt;{ freshness?: ExplorerFreshness }&gt;)</code> |
| <code>interface.14.ts-web-explorer-components-mirrorfreshnessdisplay</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/components.tsx:63</code> | <code>export function MirrorFreshnessDisplay({ mirror }: Readonly&lt;{ mirror: MirrorVerificationProvenance }&gt;)</code> |
| <code>interface.14.ts-web-explorer-components-verificationlabel</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/components.tsx:33</code> | <code>export function verificationLabel(level: ExplorerVerificationLevel): string</code> |
| <code>interface.14.ts-web-explorer-links-explorer-anchor-path</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/links.ts:26</code> | <code>export const EXPLORER_ANCHOR_PATH = "/paxeer-x/anchors";</code> |
| <code>interface.14.ts-web-explorer-links-explorer-link-kinds</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/links.ts:7</code> | <code>export const EXPLORER_LINK_KINDS = [</code> |
| <code>interface.14.ts-web-explorer-links-explorer-navigation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/links.ts:86</code> | <code>export const EXPLORER_NAVIGATION = [</code> |
| <code>interface.14.ts-web-explorer-links-explorerlinkkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/links.ts:16</code> | <code>export type ExplorerLinkKind = (typeof EXPLORER_LINK_KINDS)[number];</code> |
| <code>interface.14.ts-web-explorer-links-explorerlinkpath</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/links.ts:31</code> | <code>export function explorerLinkPath(target: ExplorerLinkTarget): string</code> |
| <code>interface.14.ts-web-explorer-links-explorerlinktarget</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/links.ts:18</code> | <code>export type ExplorerLinkTarget =</code> |
| <code>interface.14.ts-web-explorer-links-parseexplorerbaseurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/links.ts:59</code> | <code>export function parseExplorerBaseUrl(configured: string &#124; undefined): URL &#124; undefined</code> |
| <code>interface.14.ts-web-explorer-mirror-admission-mirroroverloadederror</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/mirror-admission.ts:1</code> | <code>export class MirrorOverloadedError extends Error</code> |
| <code>interface.14.ts-web-explorer-mirror-admission-mirrorverificationadmission</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/mirror-admission.ts:8</code> | <code>export class MirrorVerificationAdmission</code> |
| <code>interface.14.ts-web-explorer-mirror-server-mirrordivergenterror</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/explorer/mirror-server.ts:21</code> | <code>export class MirrorDivergentError extends Error { public constructor(){super("mirror divergent");this.name="MirrorDivergentError";} }</code> |
| <code>interface.14.ts-web-explorer-mirror-server-mirrorrefusederror</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/explorer/mirror-server.ts:22</code> | <code>export class MirrorRefusedError extends Error { public constructor(){super("mirror refused");this.name="MirrorRefusedError";} }</code> |
| <code>interface.14.ts-web-explorer-mirror-server-mirrorunavailableerror</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/explorer/mirror-server.ts:20</code> | <code>export class MirrorUnavailableError extends Error { public constructor(){super("mirror unavailable");this.name="MirrorUnavailableError";} }</code> |
| <code>interface.14.ts-web-explorer-mirror-server-verifyevidencefrommirrors</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/explorer/mirror-server.ts:55</code> | <code>export async function verifyEvidenceFromMirrors(input:Readonly&lt;{kind:"receipt"&#124;"state-inclusion";evidence:string}&gt;,signal?:AbortSignal):Promise&lt;EvidenceVerificationReport&gt;</code> |
| <code>interface.14.ts-web-explorer-model-accountactivityrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:47</code> | <code>export interface AccountActivityRecord</code> |
| <code>interface.14.ts-web-explorer-model-accountidentifier</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:483</code> | <code>export interface AccountIdentifier</code> |
| <code>interface.14.ts-web-explorer-model-accountidentifierkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:480</code> | <code>export type AccountIdentifierKind = "evm" &#124; "did" &#124; "account";</code> |
| <code>interface.14.ts-web-explorer-model-accountidentifierpath</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:507</code> | <code>export function accountIdentifierPath(canonical: string): string</code> |
| <code>interface.14.ts-web-explorer-model-batchrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:29</code> | <code>export interface BatchRecord</code> |
| <code>interface.14.ts-web-explorer-model-checkpointrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:19</code> | <code>export interface CheckpointRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodeaccountactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:270</code> | <code>export function decodeAccountActivity( value: unknown, at = "account_activity", ): AccountActivityRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodebatch</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:238</code> | <code>export function decodeBatch(value: unknown, at = "batch"): BatchRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodecheckpoint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:225</code> | <code>export function decodeCheckpoint(value: unknown, at = "checkpoint"): CheckpointRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodefreshness</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:206</code> | <code>export function decodeFreshness(value: unknown, at = "freshness"): ExplorerFreshness</code> |
| <code>interface.14.ts-web-explorer-model-decodemirrorprovenance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:452</code> | <code>export function decodeMirrorProvenance(value:unknown,at="mirror"):MirrorVerificationProvenance</code> |
| <code>interface.14.ts-web-explorer-model-decodenameresolution</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:314</code> | <code>export function decodeNameResolution(value: unknown, at = "name"): NameResolutionRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodepage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:392</code> | <code>export function decodePage&lt;T&gt;( value: unknown, decodeItem: (value: unknown, at: string) =&gt; T, at = "page", ): ExplorerPage&lt;T&gt;</code> |
| <code>interface.14.ts-web-explorer-model-decodeprogram</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:327</code> | <code>export function decodeProgram(value: unknown, at = "program"): ProgramRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodeprogramforidentifier</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:381</code> | <code>export function decodeProgramForIdentifier(value: unknown, identifier: string): ProgramRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodereceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:255</code> | <code>export function decodeReceipt(value: unknown, at = "receipt"): ReceiptRecord</code> |
| <code>interface.14.ts-web-explorer-model-decoderecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:410</code> | <code>export function decodeRecord&lt;T&gt;( value: unknown, decodeValue: (value: unknown, at: string) =&gt; T, at = "record", ): ExplorerRecord&lt;T&gt;</code> |
| <code>interface.14.ts-web-explorer-model-decodeunifiedaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:729</code> | <code>export function decodeUnifiedAccount(value: unknown, at = "unified_account"): UnifiedAccountRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodeunifiedaccountavailability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:820</code> | <code>export function decodeUnifiedAccountAvailability(value: unknown, at = "unified_account_v2"): UnifiedAccountAvailabilityRecord</code> |
| <code>interface.14.ts-web-explorer-model-decodeunifiedaccountunavailable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:806</code> | <code>export function decodeUnifiedAccountUnavailable(value: unknown): "gateway_unavailable" &#124; "index_incomplete"</code> |
| <code>interface.14.ts-web-explorer-model-decodeverificationreport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:424</code> | <code>export function decodeVerificationReport( value: unknown, at = "verification", ): EvidenceVerificationReport</code> |
| <code>interface.14.ts-web-explorer-model-encodefreshness</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:454</code> | <code>export function encodeFreshness(freshness: ExplorerFreshness): Readonly&lt;Record&lt;string, unknown&gt;&gt;</code> |
| <code>interface.14.ts-web-explorer-model-encodeverificationreport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:466</code> | <code>export function encodeVerificationReport( report: EvidenceVerificationReport, ): Readonly&lt;Record&lt;string, unknown&gt;&gt;</code> |
| <code>interface.14.ts-web-explorer-model-evidenceverificationreport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:117</code> | <code>export interface EvidenceVerificationReport</code> |
| <code>interface.14.ts-web-explorer-model-explorer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:925</code> | <code>export function explorer(): Readonly&lt;</code> |
| <code>interface.14.ts-web-explorer-model-explorerfreshness</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:9</code> | <code>export interface ExplorerFreshness</code> |
| <code>interface.14.ts-web-explorer-model-explorerpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:106</code> | <code>export interface ExplorerPage&lt;T&gt;</code> |
| <code>interface.14.ts-web-explorer-model-explorerrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:112</code> | <code>export interface ExplorerRecord&lt;T&gt;</code> |
| <code>interface.14.ts-web-explorer-model-explorerverificationlevel</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/model.ts:1</code> | <code>export type ExplorerVerificationLevel =</code> |
| <code>interface.14.ts-web-explorer-model-human-web-explorer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:929</code> | <code>export function human_web_explorer()</code> |
| <code>interface.14.ts-web-explorer-model-mirrorverificationprovenance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:127</code> | <code>export interface MirrorVerificationProvenance</code> |
| <code>interface.14.ts-web-explorer-model-nameresolutionrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:100</code> | <code>export interface NameResolutionRecord</code> |
| <code>interface.14.ts-web-explorer-model-parseaccountidentifier</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:490</code> | <code>export function parseAccountIdentifier(value: string): AccountIdentifier &#124; undefined</code> |
| <code>interface.14.ts-web-explorer-model-paxeeractivityevent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:513</code> | <code>export type PaxeerActivityEvent =</code> |
| <code>interface.14.ts-web-explorer-model-paxeeractivityrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:548</code> | <code>export interface PaxeerActivityRecord</code> |
| <code>interface.14.ts-web-explorer-model-paxeeractivitywindow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:559</code> | <code>export interface PaxeerActivityWindow</code> |
| <code>interface.14.ts-web-explorer-model-programlifecycle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:62</code> | <code>export type ProgramLifecycle = "active" &#124; "deprecated" &#124; "tombstoned";</code> |
| <code>interface.14.ts-web-explorer-model-programrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:88</code> | <code>export interface ProgramRecord</code> |
| <code>interface.14.ts-web-explorer-model-programsourcestatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:68</code> | <code>export type ProgramSourceStatus =</code> |
| <code>interface.14.ts-web-explorer-model-programupgradepolicy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:64</code> | <code>export type ProgramUpgradePolicy =</code> |
| <code>interface.14.ts-web-explorer-model-programvalueaccountrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:81</code> | <code>export interface ProgramValueAccountRecord</code> |
| <code>interface.14.ts-web-explorer-model-programversionrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:73</code> | <code>export interface ProgramVersionRecord</code> |
| <code>interface.14.ts-web-explorer-model-receiptrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:39</code> | <code>export interface ReceiptRecord</code> |
| <code>interface.14.ts-web-explorer-model-reportedavailability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:581</code> | <code>export type ReportedAvailability&lt;T&gt; =</code> |
| <code>interface.14.ts-web-explorer-model-unifiedaccountavailabilityrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:585</code> | <code>export interface UnifiedAccountAvailabilityRecord extends Omit&lt;UnifiedAccountRecord, "balances" &#124; "settlement"&gt;</code> |
| <code>interface.14.ts-web-explorer-model-unifiedaccountevidence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:511</code> | <code>export type UnifiedAccountEvidence = "gateway-reported";</code> |
| <code>interface.14.ts-web-explorer-model-unifiedaccountrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:566</code> | <code>export interface UnifiedAccountRecord</code> |
| <code>interface.14.ts-web-explorer-model-unifiedbalancerecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:530</code> | <code>export interface UnifiedBalanceRecord</code> |
| <code>interface.14.ts-web-explorer-model-unifiedidentities</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:522</code> | <code>export interface UnifiedIdentities</code> |
| <code>interface.14.ts-web-explorer-model-unifiedsettlement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:538</code> | <code>export interface UnifiedSettlement</code> |
| <code>interface.14.ts-web-explorer-model-validexplorercoordinate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:921</code> | <code>export function validExplorerCoordinate(value: string): boolean</code> |
| <code>interface.14.ts-web-explorer-model-validexploreridentifier</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:911</code> | <code>export function validExplorerIdentifier(value: string): boolean</code> |
| <code>interface.14.ts-web-explorer-model-validexplorername</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/explorer/model.ts:917</code> | <code>export function validExplorerName(value: string): boolean</code> |
| <code>interface.14.ts-web-explorer-verifier-evidenceverifier</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/explorer/verifier.tsx:41</code> | <code>export function EvidenceVerifier()</code> |
| <code>interface.14.ts-web-explorer-verifier-state-explorer-verifier-refused-statuses</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/verifier-state.ts:11</code> | <code>export const EXPLORER_VERIFIER_REFUSED_STATUSES: readonly number[] = Object.freeze([400, 413, 422]);</code> |
| <code>interface.14.ts-web-explorer-verifier-state-explorerverifierfailure</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/explorer/verifier-state.ts:5</code> | <code>export type ExplorerVerifierFailure =</code> |
| <code>interface.14.ts-web-explorer-verifier-state-explorerverifierfailure-2</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/verifier-state.ts:34</code> | <code>export function explorerVerifierFailure( status: number, retryAfterHeader: string &#124; null, now: number = Date.now(), ): ExplorerVerifierFailure</code> |
| <code>interface.14.ts-web-explorer-verifier-state-explorerverifierretryafter</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/explorer/verifier-state.ts:1</code> | <code>export type ExplorerVerifierRetryAfter =</code> |
| <code>interface.14.ts-web-explorer-verifier-state-explorerverifierretryafter-2</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/explorer/verifier-state.ts:18</code> | <code>export function explorerVerifierRetryAfter(header: string &#124; null, now: number): ExplorerVerifierRetryAfter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-explorer-client-accountactivitypage</code> | notes | stale req.8045 ac_1 makes PAXEER_X_EXPLORER_BASE_URL (apps/web/src/explorer/client.ts:81) the only origin; this surface reads LAYERX_EXPLORER_API_ORIGIN or LAYERX_EXPLORER_PROGRAM_API_ORIGIN |
| <code>interface.14.ts-web-explorer-client-explorerunavailableerror</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-client-programrecord</code> | notes | stale req.8045 ac_1 makes PAXEER_X_EXPLORER_BASE_URL (apps/web/src/explorer/client.ts:81) the only origin; this surface reads LAYERX_EXPLORER_API_ORIGIN or LAYERX_EXPLORER_PROGRAM_API_ORIGIN |
| <code>interface.14.ts-web-explorer-client-receiptrecord</code> | notes | stale req.8045 ac_1 makes PAXEER_X_EXPLORER_BASE_URL (apps/web/src/explorer/client.ts:81) the only origin; this surface reads LAYERX_EXPLORER_API_ORIGIN or LAYERX_EXPLORER_PROGRAM_API_ORIGIN |
| <code>interface.14.ts-web-explorer-client-resolvename</code> | notes | stale req.8045 ac_1 makes PAXEER_X_EXPLORER_BASE_URL (apps/web/src/explorer/client.ts:81) the only origin; this surface reads LAYERX_EXPLORER_API_ORIGIN or LAYERX_EXPLORER_PROGRAM_API_ORIGIN |
| <code>interface.14.ts-web-explorer-client-unifiedaccount</code> | notes | stale req.8045 ac_1 makes PAXEER_X_EXPLORER_BASE_URL (apps/web/src/explorer/client.ts:81) the only origin; this surface reads LAYERX_EXPLORER_API_ORIGIN or LAYERX_EXPLORER_PROGRAM_API_ORIGIN |
| <code>interface.14.ts-web-explorer-client-unifiedaccountavailability</code> | notes | stale req.8045 ac_1 makes PAXEER_X_EXPLORER_BASE_URL (apps/web/src/explorer/client.ts:81) the only origin; this surface reads LAYERX_EXPLORER_API_ORIGIN or LAYERX_EXPLORER_PROGRAM_API_ORIGIN |
| <code>interface.14.ts-web-explorer-client-unifiedaccountunavailableerror</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-client-verifyevidenceupstream</code> | notes | stale req.8045 ac_3 and feature 8 scope (spec/layerx-merge/spec.kvx:32248) retire the verify interface /v1/explorer/verify |
| <code>interface.14.ts-web-explorer-components-explorer-navigation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-components-explorerreceiptpath</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-components-verificationlabel</code> | overridden_by | override.J53.30 |
| <code>interface.14.ts-web-explorer-links-explorer-anchor-path</code> | notes | feature 8 scope (spec/layerx-merge/spec.kvx:32248) retires the anchor interfaces; this surface links anchor, batch or checkpoint targets |
| <code>interface.14.ts-web-explorer-links-explorerlinkkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-links-explorerlinkpath</code> | notes | feature 8 scope (spec/layerx-merge/spec.kvx:32248) retires the anchor interfaces; this surface links anchor, batch or checkpoint targets |
| <code>interface.14.ts-web-explorer-links-explorerlinktarget</code> | notes | feature 8 scope (spec/layerx-merge/spec.kvx:32248) retires the anchor interfaces; this surface links anchor, batch or checkpoint targets |
| <code>interface.14.ts-web-explorer-mirror-admission-mirroroverloadederror</code> | notes | stale req.8045 ac_3 removes verifyEvidenceFromMirrors and its verifier input; relocate before removal: used outside the retired set at apps/web/e2e/explorer-admission.test.ts:9 |
| <code>interface.14.ts-web-explorer-mirror-admission-mirrorverificationadmission</code> | notes | stale req.8045 ac_3 removes verifyEvidenceFromMirrors and its verifier input; relocate before removal: used outside the retired set at apps/web/e2e/explorer-admission.test.ts:9 |
| <code>interface.14.ts-web-explorer-mirror-server-mirrordivergenterror</code> | notes | stale req.8045 ac_3 removes verifyEvidenceFromMirrors and its verifier input |
| <code>interface.14.ts-web-explorer-mirror-server-mirrorrefusederror</code> | notes | stale req.8045 ac_3 removes verifyEvidenceFromMirrors and its verifier input |
| <code>interface.14.ts-web-explorer-mirror-server-mirrorunavailableerror</code> | notes | stale req.8045 ac_3 removes verifyEvidenceFromMirrors and its verifier input |
| <code>interface.14.ts-web-explorer-mirror-server-verifyevidencefrommirrors</code> | notes | stale req.8045 ac_3 removes verifyEvidenceFromMirrors and its verifier input |
| <code>interface.14.ts-web-explorer-model-accountidentifier</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-accountidentifierkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-batchrecord</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-checkpointrecord</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-decodebatch</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-decodecheckpoint</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-decodefreshness</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-decodemirrorprovenance</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-decodeprogram</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-encodefreshness</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-explorer</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-explorerverificationlevel</code> | notes | feature 8 scope (spec/layerx-merge/spec.kvx:32248) reports the three rungs submitted, receipted, committed; this surface still carries checkpoint-finalised or settlement-anchored levels |
| <code>interface.14.ts-web-explorer-model-human-web-explorer</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-paxeeractivityrecord</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-paxeeractivitywindow</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-programlifecycle</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-programsourcestatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-programupgradepolicy</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-programvalueaccountrecord</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-programversionrecord</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-reportedavailability</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-unifiedaccountevidence</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-unifiedbalancerecord</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-unifiedidentities</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-model-unifiedsettlement</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-explorer-verifier-evidenceverifier</code> | notes | stale req.8045 ac_3 removes mirror verify with the verifier |
| <code>interface.14.ts-web-explorer-verifier-state-explorer-verifier-refused-statuses</code> | notes | stale req.8045 ac_3 removes mirror verify with the verifier; relocate before removal: used outside the retired set at apps/web/e2e/explorer-verifier.test.ts:6 |
| <code>interface.14.ts-web-explorer-verifier-state-explorerverifierfailure</code> | notes | stale req.8045 ac_3 removes mirror verify with the verifier |
| <code>interface.14.ts-web-explorer-verifier-state-explorerverifierfailure-2</code> | notes | stale req.8045 ac_3 removes mirror verify with the verifier; relocate before removal: used outside the retired set at apps/web/e2e/explorer-verifier.test.ts:6 |
| <code>interface.14.ts-web-explorer-verifier-state-explorerverifierretryafter</code> | notes | stale req.8045 ac_3 removes mirror verify with the verifier |
| <code>interface.14.ts-web-explorer-verifier-state-explorerverifierretryafter-2</code> | notes | stale req.8045 ac_3 removes mirror verify with the verifier; relocate before removal: used outside the retired set at apps/web/e2e/explorer-verifier.test.ts:6 |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.39.1</code> | req | 14039.1 |
| <code>acceptance.14.39.1</code> | scenario | Given the edited links module, when explorer_link_kinds_are_receipt_transaction_address reads EXPLORER_LINK_KINDS, then it deep-equals [receipt, transaction, address] |
| <code>acceptance.14.39.1</code> | test | apps/web/e2e/explorer-links.test.ts::explorer_link_kinds_are_receipt_transaction_address (planned) |
| <code>acceptance.14.39.1</code> | qualification | UNRUN |
| <code>req.14039</code> | ac_1 | EXPLORER_LINK_KINDS (apps/web/src/explorer/links.ts:7) is exactly receipt, transaction and address once the anchor surfaces retired with feature 8 are gone. |
| <code>acceptance.14.39.2</code> | req | 14039.2 |
| <code>acceptance.14.39.2</code> | scenario | Given a target {kind: 'anchor'} cast past the type, when explorer_link_path_refuses_anchor_kind calls explorerLinkPath, then a TypeError is thrown |
| <code>acceptance.14.39.2</code> | test | apps/web/e2e/explorer-links.test.ts::explorer_link_path_refuses_anchor_kind (planned) |
| <code>acceptance.14.39.2</code> | qualification | UNRUN |
| <code>req.14039</code> | ac_2 | explorerLinkPath (apps/web/src/explorer/links.ts:31) throws TypeError for a target of kind anchor instead of returning /paxeer-x/anchors. |
| <code>acceptance.14.40.1</code> | req | 14040.1 |
| <code>acceptance.14.40.1</code> | scenario | Given two explorer receipt answers with levels sequencer-signed and batch-included, when explorer_levels_collapse_to_three_rungs decodes them, then verificationLevel is receipt-verified and checkpoint-finalised |
| <code>acceptance.14.40.1</code> | test | apps/web/e2e/explorer-rungs.test.ts::explorer_levels_collapse_to_three_rungs (planned) |
| <code>acceptance.14.40.1</code> | qualification | UNRUN |
| <code>req.14040</code> | ac_1 | verificationLevel (apps/web/src/explorer/model.ts:191) decodes a receipt record whose verification_level is sequencer-signed to receipt-verified and one whose level is batch-included to checkpoint-finalised. |
| <code>acceptance.14.40.2</code> | req | 14040.2 |
| <code>acceptance.14.40.2</code> | scenario | Given an explorer receipt answer with level settlement-anchored, when explorer_settlement_anchored_is_refused decodes it, then a TypeError is thrown |
| <code>acceptance.14.40.2</code> | test | apps/web/e2e/explorer-rungs.test.ts::explorer_settlement_anchored_is_refused (planned) |
| <code>acceptance.14.40.2</code> | qualification | UNRUN |
| <code>req.14040</code> | ac_2 | A record whose verification_level is settlement-anchored is refused with TypeError by verificationLevel (apps/web/src/explorer/model.ts:191). |
| <code>acceptance.14.41.1</code> | req | 14041.1 |
| <code>acceptance.14.41.1</code> | scenario | Given PAXEER_X_EXPLORER_BASE_URL pointing at server A and LAYERX_EXPLORER_API_ORIGIN at server B, when explorer_reads_use_base_url_origin reads a receipt, then only server A receives the request |
| <code>acceptance.14.41.1</code> | test | apps/web/e2e/explorer-origin.test.ts::explorer_reads_use_base_url_origin (planned) |
| <code>acceptance.14.41.1</code> | qualification | UNRUN |
| <code>req.14041</code> | ac_1 | receiptRecord (apps/web/src/explorer/client.ts:125) requests /v1/explorer/receipts/&lt;id&gt; on the PAXEER_X_EXPLORER_BASE_URL origin even when LAYERX_EXPLORER_API_ORIGIN names another server. |
| <code>acceptance.14.41.2</code> | req | 14041.2 |
| <code>acceptance.14.41.2</code> | scenario | Given no PAXEER_X_EXPLORER_BASE_URL, when explorer_reads_fail_closed_without_base_url calls the three readers, then each throws ExplorerUnavailableError and the server sees nothing |
| <code>acceptance.14.41.2</code> | test | apps/web/e2e/explorer-origin.test.ts::explorer_reads_fail_closed_without_base_url (planned) |
| <code>acceptance.14.41.2</code> | qualification | UNRUN |
| <code>req.14041</code> | ac_2 | With PAXEER_X_EXPLORER_BASE_URL unset, receiptRecord, unifiedAccount and accountActivityPage throw ExplorerUnavailableError (apps/web/src/explorer/client.ts:32) without a request. |
| <code>acceptance.14.42.1</code> | req | 14042.1 |
| <code>acceptance.14.42.1</code> | scenario | Given a loopback server as PAXEER_X_EXPLORER_BASE_URL answering a program record, when program_record_uses_base_url_without_bearer reads it, then the record decodes and the request carried no Authorization header |
| <code>acceptance.14.42.1</code> | test | apps/web/e2e/explorer-program-origin.test.ts::program_record_uses_base_url_without_bearer (planned) |
| <code>acceptance.14.42.1</code> | qualification | UNRUN |
| <code>req.14042</code> | ac_1 | programRecord (apps/web/src/explorer/client.ts:160) requests the PAXEER_X_EXPLORER_BASE_URL origin and sends no Authorization header. |
| <code>acceptance.14.42.2</code> | req | 14042.2 |
| <code>acceptance.14.42.2</code> | scenario | Given the two retired variables pointing at server B, when program_record_ignores_retired_program_origin reads a program, then server B receives nothing |
| <code>acceptance.14.42.2</code> | test | apps/web/e2e/explorer-program-origin.test.ts::program_record_ignores_retired_program_origin (planned) |
| <code>acceptance.14.42.2</code> | qualification | UNRUN |
| <code>req.14042</code> | ac_2 | Setting LAYERX_EXPLORER_PROGRAM_API_ORIGIN and LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN has no effect on where programRecord (apps/web/src/explorer/client.ts:160) reads. |
| <code>acceptance.14.43.1</code> | req | 14043.1 |
| <code>acceptance.14.43.1</code> | scenario | Given the edited links module, when explorer_navigation_has_no_verify_entry maps EXPLORER_NAVIGATION to hrefs, then the list is [/explorer] |
| <code>acceptance.14.43.1</code> | test | apps/web/e2e/explorer-verify-unlisted.test.ts::explorer_navigation_has_no_verify_entry (planned) |
| <code>acceptance.14.43.1</code> | qualification | UNRUN |
| <code>req.14043</code> | ac_1 | EXPLORER_NAVIGATION (apps/web/src/explorer/links.ts:86) lists only /explorer once mirror verify is retired by stale req.8045 ac_3. |
| <code>acceptance.14.43.2</code> | req | 14043.2 |
| <code>acceptance.14.43.2</code> | scenario | Given the edited matrix and budgets, when state_matrix_has_no_verify_route looks up /explorer/verify, then the matrix has no row and classifyPerformanceRoute returns undefined |
| <code>acceptance.14.43.2</code> | test | apps/web/e2e/explorer-verify-unlisted.test.ts::state_matrix_has_no_verify_route (planned) |
| <code>acceptance.14.43.2</code> | qualification | UNRUN |
| <code>req.14043</code> | ac_2 | The state matrix of apps/web/src/states/matrix.ts:121 and classifyPerformanceRoute of apps/web/src/perf/budgets.ts:26 know no /explorer/verify route. |
| <code>acceptance.14.44.1</code> | req | 14044.1 |
| <code>acceptance.14.44.1</code> | scenario | Given the edited specs, when explorer_specs_open_no_verify_page reads both files, then neither contains the string /explorer/verify |
| <code>acceptance.14.44.1</code> | test | apps/web/e2e/explorer-suites.test.ts::explorer_specs_open_no_verify_page (planned) |
| <code>acceptance.14.44.1</code> | qualification | UNRUN |
| <code>req.14044</code> | ac_1 | Neither apps/web/e2e/explorer.spec.ts:32 nor apps/web/e2e/browser/explorer.spec.ts:10 names /explorer/verify after the edit. |
| <code>acceptance.14.44.2</code> | req | 14044.2 |
| <code>acceptance.14.44.2</code> | scenario | Given the edited package.json, when web_test_scripts_name_no_verifier_suite parses the scripts, then explorer-verifier.test.ts is absent and every named file exists under apps/web |
| <code>acceptance.14.44.2</code> | test | apps/web/e2e/explorer-suites.test.ts::web_test_scripts_name_no_verifier_suite (planned) |
| <code>acceptance.14.44.2</code> | qualification | UNRUN |
| <code>req.14044</code> | ac_2 | No script of apps/web/package.json:13-20 names e2e/explorer-verifier.test.ts and every file the scripts name exists. |
| <code>acceptance.14.45.1</code> | req | 14045.1 |
| <code>acceptance.14.45.1</code> | scenario | Given the deletion, when explorer_verify_page_does_not_resolve imports the three modules, then each import rejects with code ERR_MODULE_NOT_FOUND |
| <code>acceptance.14.45.1</code> | test | apps/web/e2e/explorer-verify-retired.test.ts::explorer_verify_page_does_not_resolve (planned) |
| <code>acceptance.14.45.1</code> | qualification | UNRUN |
| <code>req.14045</code> | ac_1 | Dynamic import of apps/web/src/app/explorer/verify/page.tsx:1, apps/web/src/explorer/verifier.tsx:1 and apps/web/src/explorer/verifier-state.ts:1 rejects with ERR_MODULE_NOT_FOUND. |
| <code>acceptance.14.46.1</code> | req | 14046.1 |
| <code>acceptance.14.46.1</code> | scenario | Given the deletions, when explorer_mirror_modules_do_not_resolve dynamically imports the three modules, then each import rejects with code ERR_MODULE_NOT_FOUND |
| <code>acceptance.14.46.1</code> | test | apps/web/e2e/explorer-mirror-retired.test.ts::explorer_mirror_modules_do_not_resolve (planned) |
| <code>acceptance.14.46.1</code> | qualification | UNRUN |
| <code>req.14046</code> | ac_1 | Importing apps/web/src/explorer/mirror-server.ts:1, apps/web/src/explorer/mirror-admission.ts:1 or apps/web/src/app/api/explorer/verify/route.ts:1 rejects with ERR_MODULE_NOT_FOUND, so no POST /api/explorer/verify handler and no mirror verifier process remain. |
| <code>acceptance.14.46.2</code> | req | 14046.2 |
| <code>acceptance.14.46.2</code> | scenario | Given the edited client, when explorer_client_exports_no_verify_upstream imports it, then verifyEvidenceUpstream is undefined and receiptRecord is still a function |
| <code>acceptance.14.46.2</code> | test | apps/web/e2e/explorer-mirror-retired.test.ts::explorer_client_exports_no_verify_upstream (planned) |
| <code>acceptance.14.46.2</code> | qualification | UNRUN |
| <code>req.14046</code> | ac_2 | apps/web/src/explorer/client.ts exports no verifyEvidenceUpstream (apps/web/src/explorer/client.ts:343 at HEAD), so nothing in the web app posts to /v1/explorer/verify. |
| <code>acceptance.14.60.1</code> | req | 14060.1 |
| <code>acceptance.14.60.1</code> | scenario | Given the two rung levels, when explorer_label_uses_kit_rung_words calls verificationLabel, then each result equals the kit word for that rung |
| <code>acceptance.14.60.1</code> | test | apps/web/e2e/explorer-labels.test.ts::explorer_label_uses_kit_rung_words (planned) |
| <code>acceptance.14.60.1</code> | qualification | UNRUN |
| <code>req.14060</code> | ac_1 | verificationLabel (apps/web/src/explorer/components.tsx:33) returns the verification.receipt_verified and verification.checkpoint_finalised messages for the two rungs. |
| <code>acceptance.14.60.2</code> | req | 14060.2 |
| <code>acceptance.14.60.2</code> | scenario | Given settlement-anchored, when explorer_label_for_retired_level_is_unverified calls verificationLabel, then the result equals the verification.unverified message |
| <code>acceptance.14.60.2</code> | test | apps/web/e2e/explorer-labels.test.ts::explorer_label_for_retired_level_is_unverified (planned) |
| <code>acceptance.14.60.2</code> | qualification | UNRUN |
| <code>req.14060</code> | ac_2 | verificationLabel (apps/web/src/explorer/components.tsx:33) returns the verification.unverified message for settlement-anchored and for state-proven passed unmapped. |
| <code>capability.14.5.1</code> | title | Explorer link targets |
| <code>capability.14.5.1</code> | reqs | 14039.1<br>14039.2 |
| <code>capability.14.5.2</code> | title | Explorer verification rungs |
| <code>capability.14.5.2</code> | reqs | 14040.1<br>14040.2 |
| <code>capability.14.5.3</code> | title | Single explorer origin |
| <code>capability.14.5.3</code> | reqs | 14041.1<br>14041.2 |
| <code>capability.14.5.4</code> | title | Program records on the single origin |
| <code>capability.14.5.4</code> | reqs | 14042.1<br>14042.2 |
| <code>capability.14.5.5</code> | title | Explorer labels on the kit rungs |
| <code>capability.14.5.5</code> | reqs | 14060.1<br>14060.2 |
| <code>capability.14.6.1</code> | title | Explorer verify page unlisted |
| <code>capability.14.6.1</code> | reqs | 14043.1<br>14043.2 |
| <code>capability.14.6.2</code> | title | Explorer suites without the verify page |
| <code>capability.14.6.2</code> | reqs | 14044.1<br>14044.2 |
| <code>capability.14.6.3</code> | title | Explorer verify page retired |
| <code>capability.14.6.3</code> | reqs | 14045.1 |
| <code>capability.14.6.4</code> | title | Mirror verifier retired |
| <code>capability.14.6.4</code> | reqs | 14046.1<br>14046.2 |
| <code>resolution.17.942</code> | title | decision.1404 binds: One explorer origin |
| <code>resolution.17.942</code> | applies_to | decision.1404<br>task.14.5.3<br>task.14.5.4 |
| <code>resolution.17.942</code> | source | decision.1404 in fragments/J53/decision.kvx; tasks from affects |
| <code>resolution.17.943</code> | title | decision.1405 binds: Three verification rungs in the web kit |
| <code>resolution.17.943</code> | applies_to | decision.1405<br>task.14.5.2<br>task.14.5.5<br>task.14.9.1<br>task.14.9.2<br>task.14.9.3<br>task.14.9.4<br>task.14.9.5<br>task.14.9.6<br>task.14.10.3 |
| <code>resolution.17.943</code> | source | decision.1405 in fragments/J53/decision.kvx; tasks from affects |

## journeys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-journeys-activity-activity-activity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/activity.tsx:138</code> | <code>export function Activity()</code> |
| <code>interface.14.ts-web-journeys-activity-activity-activityerrorsurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/activity.tsx:52</code> | <code>export function ActivityErrorSurface({ failure, onRetry }: Readonly&lt;{ failure: ActivityFailure; onRetry: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-journeys-activity-activity-human-web-journeys-activity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/activity.tsx:353</code> | <code>export function human_web_journeys_Activity()</code> |
| <code>interface.14.ts-web-journeys-activity-detail-activitydetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/detail.tsx:126</code> | <code>export function ActivityDetail({ entryId }: Readonly&lt;{ entryId: string }&gt;)</code> |
| <code>interface.14.ts-web-journeys-activity-model-activitydate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:267</code> | <code>export function activityDate(timestamp: string): Date</code> |
| <code>interface.14.ts-web-journeys-activity-model-activityfailure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:469</code> | <code>export type ActivityFailure = Readonly&lt;</code> |
| <code>interface.14.ts-web-journeys-activity-model-activityfailure-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:477</code> | <code>export function activityFailure(error: unknown): ActivityFailure</code> |
| <code>interface.14.ts-web-journeys-activity-model-agentfilteroptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:450</code> | <code>export function agentFilterOptions( agents: readonly Pick&lt;Agent, "agent_id" &#124; "name"&gt;[], ): readonly FeedFilterOption[]</code> |
| <code>interface.14.ts-web-journeys-activity-model-allentryids</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:523</code> | <code>export function allEntryIds(page: ActivityPage): readonly string[]</code> |
| <code>interface.14.ts-web-journeys-activity-model-detailunresolved</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:417</code> | <code>export function detailUnresolved(detail: Readonly&lt;Pick&lt;ActivityEntryDetail, "state"&gt;&gt;): boolean</code> |
| <code>interface.14.ts-web-journeys-activity-model-emptyfiltervalues</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:328</code> | <code>export function emptyFilterValues(): FeedFilterValues</code> |
| <code>interface.14.ts-web-journeys-activity-model-entrystatuskey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:72</code> | <code>export function entryStatusKey(state: JourneyState): StatusKey</code> |
| <code>interface.14.ts-web-journeys-activity-model-entrystatuslabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:76</code> | <code>export function entryStatusLabel(state: JourneyState): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-entryunresolved</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:80</code> | <code>export function entryUnresolved(state: JourneyState): boolean</code> |
| <code>interface.14.ts-web-journeys-activity-model-entryverification</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/activity/model.ts:205</code> | <code>export function entryVerification(evidence: readonly EvidenceRef[]): VerificationLevel</code> |
| <code>interface.14.ts-web-journeys-activity-model-evidenceclasslabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:201</code> | <code>export function evidenceClassLabel(evidenceClass: EvidenceClass): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-explorerpath</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:187</code> | <code>export function explorerPath(reference: EvidenceRef): string &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-activity-model-feeddaterange</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:321</code> | <code>export interface FeedDateRange</code> |
| <code>interface.14.ts-web-journeys-activity-model-feedfilterdef</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:314</code> | <code>export interface FeedFilterDef</code> |
| <code>interface.14.ts-web-journeys-activity-model-feedfilterdefs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:332</code> | <code>export function feedFilterDefs(agents: readonly FeedFilterOption[]): FeedFilterDef[]</code> |
| <code>interface.14.ts-web-journeys-activity-model-feedfilteroption</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:309</code> | <code>export interface FeedFilterOption</code> |
| <code>interface.14.ts-web-journeys-activity-model-feedfiltervalues</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:326</code> | <code>export type FeedFilterValues = Readonly&lt;Record&lt;string, string &#124; FeedDateRange &#124; undefined&gt;&gt;;</code> |
| <code>interface.14.ts-web-journeys-activity-model-feedgroups</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:275</code> | <code>export function feedGroups(page: ActivityPage): FeedGroupView[]</code> |
| <code>interface.14.ts-web-journeys-activity-model-feedgroupview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:227</code> | <code>export interface FeedGroupView</code> |
| <code>interface.14.ts-web-journeys-activity-model-feedisempty</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:279</code> | <code>export function feedIsEmpty(page: ActivityPage): boolean</code> |
| <code>interface.14.ts-web-journeys-activity-model-feedrowview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:216</code> | <code>export interface FeedRowView</code> |
| <code>interface.14.ts-web-journeys-activity-model-filterecholines</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:283</code> | <code>export function filterEchoLines( filter: ActivityFilter, agentNames: ReadonlyMap&lt;string, string&gt; = new Map(), ): readonly string[]</code> |
| <code>interface.14.ts-web-journeys-activity-model-formatentrydate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:173</code> | <code>export function formatEntryDate(timestamp: string): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-formatentrytimestamp</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:177</code> | <code>export function formatEntryTimestamp(timestamp: string): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-kindlabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:88</code> | <code>export function kindLabel(kind: ActivityEntryKind): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-loadactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:458</code> | <code>export async function loadActivity( client: HumanApiClient, filter: ActivityFilter = {}, ): Promise&lt;Readonly&lt;</code> |
| <code>interface.14.ts-web-journeys-activity-model-mergepages</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:379</code> | <code>export function mergePages(current: ActivityPage, incoming: ActivityPage): ActivityPage</code> |
| <code>interface.14.ts-web-journeys-activity-model-monthlabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:153</code> | <code>export function monthLabel(month: string): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-newexportkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:512</code> | <code>export function newExportKey(): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-plainsentence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:92</code> | <code>export function plainSentence( entry: Readonly&lt;Pick&lt;ActivityEntry, "kind" &#124; "state" &#124; "summary_copy_key"&gt;&gt;, ): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-receiptbacked</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/activity/model.ts:428</code> | <code>export function receiptBacked(reference: EvidenceRef): boolean</code> |
| <code>interface.14.ts-web-journeys-activity-model-safeexportartefact</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:516</code> | <code>export function safeExportArtefact(artefact: ExportArtefact): ExportArtefact</code> |
| <code>interface.14.ts-web-journeys-activity-model-samefiltervalues</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:373</code> | <code>export function sameFilterValues(first: FeedFilterValues, second: FeedFilterValues): boolean</code> |
| <code>interface.14.ts-web-journeys-activity-model-signedbaseunits</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:134</code> | <code>export function signedBaseUnits( money: Money &#124; undefined, direction: MoneyDirection &#124; undefined, ): ProtocolAmount</code> |
| <code>interface.14.ts-web-journeys-activity-model-stageview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:114</code> | <code>export function stageView(stage: JourneyStage): TimelineStageView</code> |
| <code>interface.14.ts-web-journeys-activity-model-status-key-by-state</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:27</code> | <code>export const STATUS_KEY_BY_STATE: Readonly&lt;Record&lt;JourneyState, StatusKey&gt;&gt; = Object.freeze(</code> |
| <code>interface.14.ts-web-journeys-activity-model-stillcheckinglockreason</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:84</code> | <code>export function stillCheckingLockReason(): string</code> |
| <code>interface.14.ts-web-journeys-activity-model-timelinestageview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:107</code> | <code>export interface TimelineStageView</code> |
| <code>interface.14.ts-web-journeys-activity-model-tokitdirection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:124</code> | <code>export function toKitDirection(direction: MoneyDirection &#124; undefined): "inbound" &#124; "outbound" &#124; "other"</code> |
| <code>interface.14.ts-web-journeys-activity-model-towirefilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:352</code> | <code>export function toWireFilter(values: FeedFilterValues): ActivityFilter</code> |
| <code>interface.14.ts-web-journeys-activity-model-unsignedbaseunits</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:149</code> | <code>export function unsignedBaseUnits(money: Money): ProtocolAmount</code> |
| <code>interface.14.ts-web-journeys-activity-model-validateddetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/activity/model.ts:432</code> | <code>export function validatedDetail(detail: ActivityEntryDetail): ActivityEntryDetail</code> |
| <code>interface.14.ts-web-journeys-activity-model-verificationlabel</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/activity/model.ts:197</code> | <code>export function verificationLabel(level: VerificationLevel): string</code> |
| <code>interface.14.ts-web-journeys-agents-controls-agentcontrols</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/controls.tsx:72</code> | <code>export function AgentControls(props: AgentControlsProps)</code> |
| <code>interface.14.ts-web-journeys-agents-create-agentcreatejourney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/create.tsx:117</code> | <code>export function AgentCreateJourney()</code> |
| <code>interface.14.ts-web-journeys-agents-detail-agentdetailscreen</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/detail.tsx:81</code> | <code>export function AgentDetailScreen(props: AgentDetailProps)</code> |
| <code>interface.14.ts-web-journeys-agents-model-agent-currency</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:38</code> | <code>export const AGENT_CURRENCY = "LXP";</code> |
| <code>interface.14.ts-web-journeys-agents-model-agent-locale</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:39</code> | <code>export const AGENT_LOCALE = "en-GB";</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentcontrol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:379</code> | <code>export interface AgentControl</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentcontrolcontext</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:390</code> | <code>export interface AgentControlContext</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentcontrolid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:369</code> | <code>export type AgentControlId =</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentlistitems</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:580</code> | <code>export function agentListItems(page: AgentPage, locale: string): readonly AgentListItemView[]</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentlistitemview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:570</code> | <code>export interface AgentListItemView</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentpresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:241</code> | <code>export interface AgentPresentation</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentpresentation-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:259</code> | <code>export function agentPresentation(agent: Agent): AgentPresentation</code> |
| <code>interface.14.ts-web-journeys-agents-model-agents</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:611</code> | <code>export class Agents</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentslayout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:36</code> | <code>export type AgentsLayout = "stacked" &#124; "master-detail";</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentslayout-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:41</code> | <code>export function agentsLayout(shell: AgentsShell): AgentsLayout</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentsoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:606</code> | <code>export interface AgentsOptions</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentsshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:35</code> | <code>export type AgentsShell = "mobile" &#124; "desktop";</code> |
| <code>interface.14.ts-web-journeys-agents-model-agentstateverified</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:249</code> | <code>export function agentStateVerified(agent: Agent): boolean</code> |
| <code>interface.14.ts-web-journeys-agents-model-agenttone</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:223</code> | <code>export type AgentTone = "neutral" &#124; "accent" &#124; "success" &#124; "warning" &#124; "destructive";</code> |
| <code>interface.14.ts-web-journeys-agents-model-apierrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:560</code> | <code>export function apiErrorCode(error: unknown): string &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-agents-model-apierrorsentence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:547</code> | <code>export function apiErrorSentence(error: unknown): string</code> |
| <code>interface.14.ts-web-journeys-agents-model-archivedispositionready</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:253</code> | <code>export function archiveDispositionReady(agent: Agent): boolean</code> |
| <code>interface.14.ts-web-journeys-agents-model-catalogsentence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:136</code> | <code>export function catalogSentence(copyKey: string): string</code> |
| <code>interface.14.ts-web-journeys-agents-model-controlsfor</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:394</code> | <code>export function controlsFor(agent: Agent, context: AgentControlContext = {}): readonly AgentControl[]</code> |
| <code>interface.14.ts-web-journeys-agents-model-creation-decisions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:45</code> | <code>export const CREATION_DECISIONS = 3;</code> |
| <code>interface.14.ts-web-journeys-agents-model-creationdraft</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:47</code> | <code>export interface CreationDraft</code> |
| <code>interface.14.ts-web-journeys-agents-model-creationheadlinekey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:219</code> | <code>export function creationHeadlineKey(progress: JourneyProgress): "agent.create.ready" &#124; "agent.create.partial"</code> |
| <code>interface.14.ts-web-journeys-agents-model-creationready</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:117</code> | <code>export function creationReady(draft: CreationDraft): boolean</code> |
| <code>interface.14.ts-web-journeys-agents-model-creationstep</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:55</code> | <code>export interface CreationStep</code> |
| <code>interface.14.ts-web-journeys-agents-model-creationsteps</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:94</code> | <code>export function creationSteps(draft: CreationDraft): readonly [CreationStep, CreationStep, CreationStep]</code> |
| <code>interface.14.ts-web-journeys-agents-model-evidencereceiptverified</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/agents/model.ts:163</code> | <code>export function evidenceReceiptVerified(evidence: readonly EvidenceRef[]): boolean</code> |
| <code>interface.14.ts-web-journeys-agents-model-formatmoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:287</code> | <code>export function formatMoney(money: Money, locale: string): string</code> |
| <code>interface.14.ts-web-journeys-agents-model-formatplaindelay</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:472</code> | <code>export function formatPlainDelay(seconds: number): string</code> |
| <code>interface.14.ts-web-journeys-agents-model-formatplaintimestamp</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:482</code> | <code>export function formatPlainTimestamp(timestamp: string, locale: string): string</code> |
| <code>interface.14.ts-web-journeys-agents-model-journeyprogress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:181</code> | <code>export interface JourneyProgress</code> |
| <code>interface.14.ts-web-journeys-agents-model-journeyprogress-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:190</code> | <code>export function journeyProgress(journey: Journey): JourneyProgress</code> |
| <code>interface.14.ts-web-journeys-agents-model-journeystatuskey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:132</code> | <code>export function journeyStatusKey(state: JourneyState): StatusKey</code> |
| <code>interface.14.ts-web-journeys-agents-model-keychallengepresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:495</code> | <code>export interface KeyChallengePresentation</code> |
| <code>interface.14.ts-web-journeys-agents-model-keychallengepresentation-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:503</code> | <code>export function keyChallengePresentation(challenge: KeyChallenge, locale: string): KeyChallengePresentation</code> |
| <code>interface.14.ts-web-journeys-agents-model-mutationoutcomeunknown</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:564</code> | <code>export function mutationOutcomeUnknown(error: unknown): boolean</code> |
| <code>interface.14.ts-web-journeys-agents-model-parsemonthlylimit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:85</code> | <code>export function parseMonthlyLimit(input: string, currency: string): Money &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-agents-model-parserotationtiming</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:67</code> | <code>export function parseRotationTiming(delay: string, window: string): RotationTiming &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-agents-model-quotepresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:526</code> | <code>export interface QuotePresentation</code> |
| <code>interface.14.ts-web-journeys-agents-model-quotepresentation-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:533</code> | <code>export function quotePresentation(quote: MoveQuote, locale: string): QuotePresentation</code> |
| <code>interface.14.ts-web-journeys-agents-model-rotationtiming</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:62</code> | <code>export interface RotationTiming</code> |
| <code>interface.14.ts-web-journeys-agents-model-spendpresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:305</code> | <code>export interface SpendPresentation</code> |
| <code>interface.14.ts-web-journeys-agents-model-spendpresentation-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:320</code> | <code>export function spendPresentation(agent: Agent, locale: string): SpendPresentation</code> |
| <code>interface.14.ts-web-journeys-agents-model-stagereceiptverified</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/agents/model.ts:167</code> | <code>export function stageReceiptVerified(stage: JourneyStage): boolean</code> |
| <code>interface.14.ts-web-journeys-agents-model-stageview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/model.ts:174</code> | <code>export interface StageView</code> |
| <code>interface.14.ts-web-journeys-agents-progress-journeystages</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/progress.tsx:9</code> | <code>export function JourneyStages({ progress }: Readonly&lt;{ progress: JourneyProgress }&gt;)</code> |
| <code>interface.14.ts-web-journeys-agents-shell-useagentsshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/shell.ts:6</code> | <code>export function useAgentsShell(): AgentsShell</code> |
| <code>interface.14.ts-web-journeys-agents-surface-agentssurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/agents/surface.tsx:58</code> | <code>export function AgentsSurface({ ownerAccount, }: Readonly&lt;{ ownerAccount?: string }&gt; = {})</code> |
| <code>interface.14.ts-web-journeys-approvals-ceremony-base64urlfrombytes</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/ceremony.ts:27</code> | <code>export function base64UrlFromBytes(bytes: Uint8Array): string</code> |
| <code>interface.14.ts-web-journeys-approvals-ceremony-base64urlfromtext</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/ceremony.ts:35</code> | <code>export function base64UrlFromText(value: string): string</code> |
| <code>interface.14.ts-web-journeys-approvals-ceremony-browserpasskeyauthenticator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/ceremony.ts:39</code> | <code>export function browserPasskeyAuthenticator(): PasskeyAuthenticator</code> |
| <code>interface.14.ts-web-journeys-approvals-ceremony-bytesfrombase64url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/ceremony.ts:18</code> | <code>export function bytesFromBase64Url(value: string): Uint8Array</code> |
| <code>interface.14.ts-web-journeys-approvals-ceremony-passkeyauthenticator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/ceremony.ts:8</code> | <code>export type PasskeyAuthenticator = (ceremony: OpaqueCredential) =&gt; Promise&lt;OpaqueCredential&gt;;</code> |
| <code>interface.14.ts-web-journeys-approvals-ceremony-performstepup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/ceremony.ts:53</code> | <code>export async function performStepUp( client: HumanApiClient, confirms: OperationDigest, authenticator: PasskeyAuthenticator, ): Promise&lt;StepUpEvidence&gt;</code> |
| <code>interface.14.ts-web-journeys-approvals-ceremony-textfrombase64url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/ceremony.ts:23</code> | <code>export function textFromBase64Url(value: string): string</code> |
| <code>interface.14.ts-web-journeys-approvals-controller-approvals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/controller.ts:61</code> | <code>export class Approvals</code> |
| <code>interface.14.ts-web-journeys-approvals-controller-approvalsoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/controller.ts:32</code> | <code>export interface ApprovalsOptions</code> |
| <code>interface.14.ts-web-journeys-approvals-controller-loadprogramapprovals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/controller.ts:37</code> | <code>export async function loadProgramApprovals(client: HumanApiClient): Promise&lt;ProgramApprovalSummary[]&gt;</code> |
| <code>interface.14.ts-web-journeys-approvals-journey-screen-approvalsjourneyscreen</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/journey-screen.tsx:75</code> | <code>export function ApprovalsJourneyScreen({ approvalId, program = false, client: suppliedClient, }: Readonly&lt;{ approvalId?: string; program?: boolean; client?: HumanApiClient; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-approvals-model-activityroute</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:39</code> | <code>export function activityRoute(entryId: string): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvalinboxitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:136</code> | <code>export interface ApprovalInboxItem</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvalinboxitem-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:150</code> | <code>export function approvalInboxItem(summary: ApprovalSummary, at: Date): ApprovalInboxItem</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvaloutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:208</code> | <code>export type ApprovalOutcome =</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvalroute</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:35</code> | <code>export function approvalRoute(approvalId: string): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvals-route</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:28</code> | <code>export const APPROVALS_ROUTE = "/app/approvals";</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvalstatecopykey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:115</code> | <code>export function approvalStateCopyKey(state: ApprovalState): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvalstatepresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:119</code> | <code>export interface ApprovalStatePresentation</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvalstatepresentation-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:124</code> | <code>export function approvalStatePresentation( state: ApprovalState, stateCopyKey: string = approvalStateCopyKey(state), ): ApprovalStatePresentation</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approvaltone</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:105</code> | <code>export type ApprovalTone = "destructive" &#124; "neutral" &#124; "success" &#124; "warning";</code> |
| <code>interface.14.ts-web-journeys-approvals-model-approveconsequence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:172</code> | <code>export function approveConsequence(detail: ApprovalDetail): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-canapprove</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:200</code> | <code>export function canApprove(detail: ApprovalDetail, at: Date): boolean</code> |
| <code>interface.14.ts-web-journeys-approvals-model-candecideprogram</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:358</code> | <code>export function canDecideProgram(detail: ProgramApprovalDetail, at: Date): boolean</code> |
| <code>interface.14.ts-web-journeys-approvals-model-catalogmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:132</code> | <code>export function catalogMessage(copyKey: string, fallbackKey: string): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-convergedoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:238</code> | <code>export function convergedOutcome(detail: ApprovalDetail): ApprovalOutcome</code> |
| <code>interface.14.ts-web-journeys-approvals-model-decidedoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:217</code> | <code>export function decidedOutcome(decision: ApprovalDecision): ApprovalOutcome</code> |
| <code>interface.14.ts-web-journeys-approvals-model-decisionkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:301</code> | <code>export function decisionKey(): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-defectiveoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:231</code> | <code>export function defectiveOutcome(): ApprovalOutcome</code> |
| <code>interface.14.ts-web-journeys-approvals-model-expirycountdown</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:64</code> | <code>export interface ExpiryCountdown</code> |
| <code>interface.14.ts-web-journeys-approvals-model-expirycountdown-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:69</code> | <code>export function expiryCountdown(expiresAt: Timestamp, at: Date): ExpiryCountdown</code> |
| <code>interface.14.ts-web-journeys-approvals-model-failureoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:290</code> | <code>export function failureOutcome(detail: ApiError): ApprovalOutcome &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-approvals-model-helddigest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:180</code> | <code>export function heldDigest(detail: ApprovalDetail): OperationDigest &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-approvals-model-journeystatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:323</code> | <code>export function journeyStatus(state: JourneyState): StatusKey</code> |
| <code>interface.14.ts-web-journeys-approvals-model-money-locale</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:29</code> | <code>export const MONEY_LOCALE = "en-US";</code> |
| <code>interface.14.ts-web-journeys-approvals-model-moneylabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:54</code> | <code>export function moneyLabel(money: Money): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-pendingapprovalcount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:168</code> | <code>export function pendingApprovalCount(summaries: readonly ApprovalSummary[]): number</code> |
| <code>interface.14.ts-web-journeys-approvals-model-programapprovalroute</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:327</code> | <code>export function programApprovalRoute(approvalId: string): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-programapprovalstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:351</code> | <code>export function programApprovalState(state: ProgramApprovalState): ApprovalStatePresentation</code> |
| <code>interface.14.ts-web-journeys-approvals-model-programapprovaltitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:346</code> | <code>export function programApprovalTitle(summary: ProgramApprovalSummary): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-programdecisionmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:375</code> | <code>export function programDecisionMessage(decision: ProgramApprovalDecision): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-protocolmoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:43</code> | <code>export function protocolMoney(money: Money): Readonly&lt;</code> |
| <code>interface.14.ts-web-journeys-approvals-model-releasedactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:305</code> | <code>export function releasedActivity(page: ActivityPage, approvalId: string): ActivityEntry &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-approvals-model-requestedatlabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:83</code> | <code>export function requestedAtLabel(createdAt: Timestamp): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-stepupevidencereference</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:189</code> | <code>export function stepUpEvidenceReference(evidence: StepUpEvidence): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-validateprogramapproval</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:331</code> | <code>export function validateProgramApproval(summary: ProgramApprovalSummary): void</code> |
| <code>interface.14.ts-web-journeys-approvals-model-validateprogrambudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:363</code> | <code>export function validateProgramBudget(detail: ProgramApprovalDetail, budget: ProgramApprovalBudget): void</code> |
| <code>interface.14.ts-web-journeys-approvals-model-verificationlabel</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/approvals/model.ts:97</code> | <code>export function verificationLabel(level: VerificationLevel): string</code> |
| <code>interface.14.ts-web-journeys-approvals-model-verifiedmoneylabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/model.ts:101</code> | <code>export function verifiedMoneyLabel(budget: VerifiedMoney): string</code> |
| <code>interface.14.ts-web-journeys-approvals-programs-programapprovaldetailcard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/programs.tsx:46</code> | <code>export function ProgramApprovalDetailCard({ detail, controller, at, shell, onChanged }: Readonly&lt;{ detail: ProgramApprovalDetail; controller: Approvals; at: Date; shell: Shell; onChanged: () =&gt; Promise&lt;void&gt;; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-approvals-programs-programapprovalinboxrows</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/programs.tsx:19</code> | <code>export function ProgramApprovalInboxRows({ approvals, at, onOpen, selectedId }: Readonly&lt;{ approvals: readonly ProgramApprovalSummary[]; at: Date; onOpen?: ((id: string) =&gt; void) &#124; undefined; selectedId?: string &#124; undefined; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-approvalcountdown</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:64</code> | <code>export function ApprovalCountdown({ expiresAt, at, }: Readonly&lt;{ expiresAt: Timestamp; at: Date; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-approvaldecisionconfirmations</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:438</code> | <code>export function ApprovalDecisionConfirmations({ detail, shell, approveOpen, rejectOpen, onApproveOpenChange, onRejectOpenChange, onConfirmApprove, onConfirmReject, deciding = false, }: Readonly&lt;{ detail: ApprovalDetail;</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-approvaldetailcard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:284</code> | <code>export function ApprovalDetailCard({ detail, at, onApprove, onReject, outcome, released, deciding = false, shell, }: Readonly&lt;{ detail: ApprovalDetail; at: Date;</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-approvalinboxlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:88</code> | <code>export function ApprovalInboxList({ approvals, programApprovals = [], onOpenProgram, at, onOpen, selectedId, }: Readonly&lt;{ approvals: readonly ApprovalSummary[]; programApprovals?: readonly ProgramApprovalSummary[]; onOpenProgram?: ((approvalId: string) =&gt; void) &#124; undefined; at: Date;</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-approvaloutcomepanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:245</code> | <code>export function ApprovalOutcomePanel({ outcome, released, }: Readonly&lt;{ outcome: ApprovalOutcome; released?: ActivityEntryDetail &#124; undefined; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-approvalstatebadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:76</code> | <code>export function ApprovalStateBadge({ state, stateCopyKey, }: Readonly&lt;{ state: ApprovalState; stateCopyKey?: string; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-desktopapprovalsplit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:172</code> | <code>export function DesktopApprovalSplit({ approvals, programApprovals = [], onOpenProgram, at, onOpen, selectedId, children, }: Readonly&lt;{ approvals: readonly ApprovalSummary[]; programApprovals?: readonly ProgramApprovalSummary[]; onOpenProgram?: ((approvalId: string) =&gt; void) &#124; undefined;</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-mobileapprovalinbox</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:148</code> | <code>export function MobileApprovalInbox({ approvals, programApprovals = [], onOpenProgram, at, onOpen, }: Readonly&lt;{ approvals: readonly ApprovalSummary[]; programApprovals?: readonly ProgramApprovalSummary[]; onOpenProgram?: ((approvalId: string) =&gt; void) &#124; undefined; at: Date; onOpen?: ((approvalId: string) =&gt; void) &#124; undefined;</code> |
| <code>interface.14.ts-web-journeys-approvals-screens-releasedactivitysection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/approvals/screens.tsx:207</code> | <code>export function ReleasedActivitySection({ entry }: Readonly&lt;{ entry?: ActivityEntryDetail &#124; undefined; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-custody-copy-custodycopykey</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/copy.ts:3</code> | <code>export function custodyCopyKey(key: string, fallbackKey: string): string</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-depositcomplete</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/evidence.ts:105</code> | <code>export function depositComplete(journey: Journey): boolean</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-exitcomplete</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/evidence.ts:119</code> | <code>export function exitComplete(journey: Journey): boolean</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-presentedjourneystate</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/evidence.ts:95</code> | <code>export function presentedJourneyState(journey: Journey, finalStageCopyKey: string): JourneyState</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-presentedstagestate</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/evidence.ts:58</code> | <code>export function presentedStageState(stage: JourneyStage): JourneyState</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-stageevidencebacked</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/evidence.ts:48</code> | <code>export function stageEvidenceBacked(stage: JourneyStage): boolean</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-stageevidencerule</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/evidence.ts:44</code> | <code>export function stageEvidenceRule(copyKey: string): StageEvidenceRule &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-stageoutcomeverified</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/evidence.ts:86</code> | <code>export function stageOutcomeVerified(journey: Journey, finalStageCopyKey: string): boolean</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-statuskeyforstate</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/evidence.ts:65</code> | <code>export function statusKeyForState(state: JourneyState): StatusKey</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-verificationatleast</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/evidence.ts:18</code> | <code>export function verificationAtLeast(level: VerificationLevel, required: VerificationLevel): boolean</code> |
| <code>interface.14.ts-web-journeys-custody-evidence-withdrawpaidout</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/evidence.ts:112</code> | <code>export function withdrawPaidOut(journey: Journey): boolean</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-browserwalletbridge</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/handoff.ts:183</code> | <code>export function browserWalletBridge( provider: () =&gt; Eip1193Provider &#124; undefined, options: CustodyBridgeOptions = {}, ): PaxeerWalletBridge</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-custodybridgeoptions</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/handoff.ts:115</code> | <code>export interface CustodyBridgeOptions</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-custodyfromwalletsignrequest</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/handoff.ts:124</code> | <code>export function custodyFromWalletSignRequest( request: WalletSignRequest, expectedChainId = BigInt(PAXEER_CHAIN_ID), ): Readonly&lt;</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-custodyhandoffrecord</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/handoff.ts:104</code> | <code>export interface CustodyHandoffRecord</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-custodyhandoffstorage</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/handoff.ts:98</code> | <code>export interface CustodyHandoffStorage</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-custodyhandoffstoragekey</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/handoff.ts:148</code> | <code>export function custodyHandoffStorageKey(request: WalletSignRequest): string</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-eip1193provider</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/handoff.ts:89</code> | <code>export interface Eip1193Provider</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-paxeerwalletbridge</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/handoff.ts:21</code> | <code>export interface PaxeerWalletBridge</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-wallethandoff</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/handoff.ts:25</code> | <code>export class WalletHandOff</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-wallethandoffphase</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/handoff.ts:5</code> | <code>export type WalletHandOffPhase =</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-walletsignoutcome</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/handoff.ts:14</code> | <code>export type WalletSignOutcome =</code> |
| <code>interface.14.ts-web-journeys-custody-handoff-windowwalletprovider</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/handoff.ts:278</code> | <code>export function windowWalletProvider(): Eip1193Provider &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-custody-journey-screen-journeyscreen</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/journey-screen.tsx:94</code> | <code>export function JourneyScreen({ journeyId, timing, }: Readonly&lt;{ journeyId: string; timing: CustodyTiming }&gt;)</code> |
| <code>interface.14.ts-web-journeys-custody-model-custody-currency</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/model.ts:9</code> | <code>export const CUSTODY_CURRENCY = "LXP";</code> |
| <code>interface.14.ts-web-journeys-custody-model-custodyapplicationpath</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:105</code> | <code>export function custodyApplicationPath(path: string): string &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-custody-model-custodyshell</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:7</code> | <code>export type CustodyShell = "mobile" &#124; "desktop";</code> |
| <code>interface.14.ts-web-journeys-custody-model-journeytimeline</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/model.ts:18</code> | <code>export function journeyTimeline(journey: Journey): readonly TimelineRow[]</code> |
| <code>interface.14.ts-web-journeys-custody-model-newidempotencykey</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/model.ts:137</code> | <code>export function newIdempotencyKey(randomBytes: RandomBytes = defaultRandomBytes): string</code> |
| <code>interface.14.ts-web-journeys-custody-model-randombytes</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:131</code> | <code>export type RandomBytes = (length: number) =&gt; Uint8Array;</code> |
| <code>interface.14.ts-web-journeys-custody-model-refusalpresentation</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:98</code> | <code>export interface RefusalPresentation</code> |
| <code>interface.14.ts-web-journeys-custody-model-refusalpresentation-2</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:116</code> | <code>export function refusalPresentation(refusal: Refusal): RefusalPresentation</code> |
| <code>interface.14.ts-web-journeys-custody-model-timelinerow</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:11</code> | <code>export interface TimelineRow</code> |
| <code>interface.14.ts-web-journeys-custody-model-validatedestinationaddress</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:153</code> | <code>export function validateDestinationAddress(input: string): string &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-custody-model-validatepositiveamount</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:145</code> | <code>export function validatePositiveAmount(input: string): bigint &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-custody-model-walletpanel</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:39</code> | <code>export function walletPanel( request: WalletSignRequest, phase: WalletHandOffPhase, fallbackSignKey: string, ): WalletPanelPlan &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-custody-model-walletpanelplan</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/model.ts:31</code> | <code>export interface WalletPanelPlan</code> |
| <code>interface.14.ts-web-journeys-custody-recovery-isjourneyoutcomeunknown</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/recovery.ts:24</code> | <code>export function isJourneyOutcomeUnknown(error: unknown): error is JourneyOutcomeUnknownError</code> |
| <code>interface.14.ts-web-journeys-custody-recovery-journeyoutcomeunknownerror</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/recovery.ts:7</code> | <code>export class JourneyOutcomeUnknownError extends Error</code> |
| <code>interface.14.ts-web-journeys-custody-recovery-mutationoutcomeisunknown</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/recovery.ts:17</code> | <code>export function mutationOutcomeIsUnknown(error: unknown): boolean</code> |
| <code>interface.14.ts-web-journeys-custody-recovery-unknownoutcomerecovery</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/recovery.ts:3</code> | <code>export type UnknownOutcomeRecovery = () =&gt; Promise&lt;</code> |
| <code>interface.14.ts-web-journeys-custody-time-custodytiming</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/time.ts:9</code> | <code>export interface CustodyTiming</code> |
| <code>interface.14.ts-web-journeys-custody-time-custodytimingfromenv</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:26</code> | <code>export function custodyTimingFromEnv(env: EnvSource): CustodyTiming</code> |
| <code>interface.14.ts-web-journeys-custody-time-elapsedseconds</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:68</code> | <code>export function elapsedSeconds(nowMs: number, sinceIso: string): number</code> |
| <code>interface.14.ts-web-journeys-custody-time-plainduration</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:52</code> | <code>export function plainDuration(seconds: number): string</code> |
| <code>interface.14.ts-web-journeys-custody-time-settlementdeclaration</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:3</code> | <code>export interface SettlementDeclaration</code> |
| <code>interface.14.ts-web-journeys-custody-time-settlementexpectationseconds</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:45</code> | <code>export function settlementExpectationSeconds(settlement: SettlementDeclaration): number</code> |
| <code>interface.14.ts-web-journeys-custody-time-stagedelayed</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/time.ts:76</code> | <code>export function stageDelayed(nowMs: number, sinceIso: string, expectationSeconds: number): boolean</code> |
| <code>interface.14.ts-web-journeys-custody-timeline-completeview</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/timeline.tsx:171</code> | <code>export function CompleteView({ titleKey, bodyKey, }: Readonly&lt;{ titleKey: string; bodyKey: string }&gt;)</code> |
| <code>interface.14.ts-web-journeys-custody-timeline-delaynotice</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/timeline.tsx:154</code> | <code>export function DelayNotice({ duration }: Readonly&lt;{ duration: string }&gt;)</code> |
| <code>interface.14.ts-web-journeys-custody-timeline-journeytechnicaldetails</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/timeline.tsx:54</code> | <code>export function JourneyTechnicalDetails({ journey }: Readonly&lt;{ journey: Journey }&gt;)</code> |
| <code>interface.14.ts-web-journeys-custody-timeline-journeytimelineview</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/custody/timeline.tsx:31</code> | <code>export function JourneyTimelineView({ rows }: Readonly&lt;{ rows: readonly TimelineRow[] }&gt;)</code> |
| <code>interface.14.ts-web-journeys-custody-timeline-refusalview</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/timeline.tsx:127</code> | <code>export function RefusalView({ refusal, onNavigate, }: Readonly&lt;{ refusal: RefusalPresentation; onNavigate: (path: string) =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-journeys-custody-timeline-safetoclosenotice</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/timeline.tsx:163</code> | <code>export function SafeToCloseNotice({ messageKey }: Readonly&lt;{ messageKey: string }&gt;)</code> |
| <code>interface.14.ts-web-journeys-custody-timeline-usecustodyshell</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/timeline.tsx:14</code> | <code>export function useCustodyShell(): CustodyShell</code> |
| <code>interface.14.ts-web-journeys-custody-timeline-walletpanelview</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/custody/timeline.tsx:98</code> | <code>export function WalletPanelView({ panel, onOpen, busy, }: Readonly&lt;{ panel: WalletPanelPlan; onOpen: () =&gt; void; busy: boolean }&gt;)</code> |
| <code>interface.14.ts-web-journeys-deposit-model-deposit-final-stage</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/deposit/model.ts:23</code> | <code>export const DEPOSIT_FINAL_STAGE = "deposit.stage.crediting";</code> |
| <code>interface.14.ts-web-journeys-deposit-model-depositcontroller</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/deposit/model.ts:153</code> | <code>export class DepositController</code> |
| <code>interface.14.ts-web-journeys-deposit-model-depositcontrolleroptions</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/deposit/model.ts:147</code> | <code>export interface DepositControllerOptions</code> |
| <code>interface.14.ts-web-journeys-deposit-model-depositplan</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/deposit/model.ts:35</code> | <code>export interface DepositPlan</code> |
| <code>interface.14.ts-web-journeys-deposit-model-depositplan-2</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/deposit/model.ts:58</code> | <code>export function depositPlan(input: DepositPlanInput): DepositPlan</code> |
| <code>interface.14.ts-web-journeys-deposit-model-depositplaninput</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/deposit/model.ts:25</code> | <code>export interface DepositPlanInput</code> |
| <code>interface.14.ts-web-journeys-deposit-screen-deposit</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/deposit/screen.tsx:34</code> | <code>export function Deposit({ timing }: Readonly&lt;{ timing: CustodyTiming }&gt;)</code> |
| <code>interface.14.ts-web-journeys-deposit-screen-depositscreen</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/deposit/screen.tsx:281</code> | <code>export const DepositScreen = Deposit;</code> |
| <code>interface.14.ts-web-journeys-exit-model-exit-confirmation-phrase</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/exit/model.ts:24</code> | <code>export const EXIT_CONFIRMATION_PHRASE = copyEntry("exit.confirmation.phrase").message;</code> |
| <code>interface.14.ts-web-journeys-exit-model-exit-final-stage</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/exit/model.ts:25</code> | <code>export const EXIT_FINAL_STAGE = "exit.stage.confirming-on-paxeer";</code> |
| <code>interface.14.ts-web-journeys-exit-model-exitconfirmationerror</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/exit/model.ts:27</code> | <code>export class ExitConfirmationError extends Error</code> |
| <code>interface.14.ts-web-journeys-exit-model-exitconfirmationready</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/exit/model.ts:34</code> | <code>export function exitConfirmationReady(typed: string): boolean</code> |
| <code>interface.14.ts-web-journeys-exit-model-exitcontroller</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/exit/model.ts:141</code> | <code>export class ExitController</code> |
| <code>interface.14.ts-web-journeys-exit-model-exitcontrolleroptions</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/exit/model.ts:135</code> | <code>export interface ExitControllerOptions</code> |
| <code>interface.14.ts-web-journeys-exit-model-exitplan</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/exit/model.ts:47</code> | <code>export interface ExitPlan</code> |
| <code>interface.14.ts-web-journeys-exit-model-exitplan-2</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/exit/model.ts:72</code> | <code>export function exitPlan(input: ExitPlanInput): ExitPlan</code> |
| <code>interface.14.ts-web-journeys-exit-model-exitplaninput</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/exit/model.ts:38</code> | <code>export interface ExitPlanInput</code> |
| <code>interface.14.ts-web-journeys-exit-screen-exit</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/exit/screen.tsx:29</code> | <code>export function Exit()</code> |
| <code>interface.14.ts-web-journeys-exit-screen-exitscreen</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/exit/screen.tsx:277</code> | <code>export const ExitScreen = Exit;</code> |
| <code>interface.14.ts-web-journeys-home-home-home</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/home.tsx:37</code> | <code>export function Home()</code> |
| <code>interface.14.ts-web-journeys-home-model-agentssummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:149</code> | <code>export function agentsSummary(agents: readonly Agent[]): string</code> |
| <code>interface.14.ts-web-journeys-home-model-approvalbadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:118</code> | <code>export function approvalBadge( approvals: readonly ApprovalSummary[], programs: readonly ProgramApprovalSummary[] = [], ): Readonly&lt;</code> |
| <code>interface.14.ts-web-journeys-home-model-classifyhomefailure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:79</code> | <code>export function classifyHomeFailure(error: unknown): HomeLoadFailure</code> |
| <code>interface.14.ts-web-journeys-home-model-home-activity-limit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:27</code> | <code>export const HOME_ACTIVITY_LIMIT = 5;</code> |
| <code>interface.14.ts-web-journeys-home-model-home-destinations</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/home/model.ts:29</code> | <code>export const HOME_DESTINATIONS = Object.freeze(</code> |
| <code>interface.14.ts-web-journeys-home-model-homeactionitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:38</code> | <code>export interface HomeActionItem</code> |
| <code>interface.14.ts-web-journeys-home-model-homeactions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:45</code> | <code>export function homeActions(): readonly [HomeActionItem, HomeActionItem, HomeActionItem]</code> |
| <code>interface.14.ts-web-journeys-home-model-homeactivityrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:153</code> | <code>export interface HomeActivityRow</code> |
| <code>interface.14.ts-web-journeys-home-model-homeactivityrows</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:173</code> | <code>export function homeActivityRows(entries: readonly ActivityEntryDetail[]): readonly HomeActivityRow[]</code> |
| <code>interface.14.ts-web-journeys-home-model-homeagentrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:126</code> | <code>export interface HomeAgentRow</code> |
| <code>interface.14.ts-web-journeys-home-model-homeagentrows</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:135</code> | <code>export function homeAgentRows(agents: readonly Agent[]): readonly HomeAgentRow[]</code> |
| <code>interface.14.ts-web-journeys-home-model-homebalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:83</code> | <code>export type HomeBalance =</code> |
| <code>interface.14.ts-web-journeys-home-model-homebalance-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:95</code> | <code>export function homeBalance(balance?: AccountBalance): HomeBalance</code> |
| <code>interface.14.ts-web-journeys-home-model-homedata</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:58</code> | <code>export interface HomeData</code> |
| <code>interface.14.ts-web-journeys-home-model-homeloadfailure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:77</code> | <code>export type HomeLoadFailure = "offline" &#124; "error";</code> |
| <code>interface.14.ts-web-journeys-home-model-loadhome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/home/model.ts:66</code> | <code>export async function loadHome(client: HumanApiClient): Promise&lt;HomeData&gt;</code> |
| <code>interface.14.ts-web-journeys-move-model-account-storage-key</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:31</code> | <code>export const ACCOUNT_STORAGE_KEY = ACTIVE_ACCOUNT_STORAGE_KEY;</code> |
| <code>interface.14.ts-web-journeys-move-model-amount-locale</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:29</code> | <code>export const AMOUNT_LOCALE = "en";</code> |
| <code>interface.14.ts-web-journeys-move-model-authorizemovenativeaccess</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:341</code> | <code>export async function authorizeMoveNativeAccess( client: HumanApiClient, request: NativeSendAccessRequest, ): Promise&lt;string&gt;</code> |
| <code>interface.14.ts-web-journeys-move-model-clearpendingmove</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:126</code> | <code>export function clearPendingMove(storage: Pick&lt;Storage, "removeItem"&gt;): void</code> |
| <code>interface.14.ts-web-journeys-move-model-commitmove</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:363</code> | <code>export async function commitMove( client: HumanApiClient, quoteId: string, attemptKey: string, nativeAccessId?: string, ): Promise&lt;MoveCommitOutcome&gt;</code> |
| <code>interface.14.ts-web-journeys-move-model-destinationoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:212</code> | <code>export function destinationOptions(agents: readonly Agent[]): readonly MoveDestinationOption[]</code> |
| <code>interface.14.ts-web-journeys-move-model-home-route</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:24</code> | <code>export const HOME_ROUTE = "/app";</code> |
| <code>interface.14.ts-web-journeys-move-model-moneyamountnumber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:141</code> | <code>export function moneyAmountNumber(money: Money): ProtocolAmount</code> |
| <code>interface.14.ts-web-journeys-move-model-moneywords</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:149</code> | <code>export function moneyWords(money: Money, locale: string = AMOUNT_LOCALE): string</code> |
| <code>interface.14.ts-web-journeys-move-model-move-currency</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:28</code> | <code>export const MOVE_CURRENCY = "LXP";</code> |
| <code>interface.14.ts-web-journeys-move-model-move-decisions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:26</code> | <code>export const MOVE_DECISIONS = 3;</code> |
| <code>interface.14.ts-web-journeys-move-model-move-route</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:25</code> | <code>export const MOVE_ROUTE = "/app/move";</code> |
| <code>interface.14.ts-web-journeys-move-model-move-step-ids</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:27</code> | <code>export const MOVE_STEP_IDS = ["who", "how-much", "review"] as const;</code> |
| <code>interface.14.ts-web-journeys-move-model-movecommitoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:336</code> | <code>export type MoveCommitOutcome =</code> |
| <code>interface.14.ts-web-journeys-move-model-movedestinationoption</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:206</code> | <code>export interface MoveDestinationOption</code> |
| <code>interface.14.ts-web-journeys-move-model-movefailure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:225</code> | <code>export type MoveFailure =</code> |
| <code>interface.14.ts-web-journeys-move-model-movefailure-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:231</code> | <code>export function moveFailure(error: unknown): MoveFailure</code> |
| <code>interface.14.ts-web-journeys-move-model-movequoteinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:256</code> | <code>export interface MoveQuoteInput</code> |
| <code>interface.14.ts-web-journeys-move-model-movequoteoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:263</code> | <code>export type MoveQuoteOutcome =</code> |
| <code>interface.14.ts-web-journeys-move-model-moveresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:432</code> | <code>export type MoveResult =</code> |
| <code>interface.14.ts-web-journeys-move-model-moveresult-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:471</code> | <code>export function moveResult(journey: Journey): MoveResult</code> |
| <code>interface.14.ts-web-journeys-move-model-movereview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:283</code> | <code>export interface MoveReview</code> |
| <code>interface.14.ts-web-journeys-move-model-movereview-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:295</code> | <code>export function moveReview(quote: MoveQuote): MoveReview</code> |
| <code>interface.14.ts-web-journeys-move-model-movestagepresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:414</code> | <code>export interface MoveStagePresentation</code> |
| <code>interface.14.ts-web-journeys-move-model-movestages</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:420</code> | <code>export function moveStages(journey: Journey): readonly MoveStagePresentation[]</code> |
| <code>interface.14.ts-web-journeys-move-model-movestepcopy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:178</code> | <code>export interface MoveStepCopy</code> |
| <code>interface.14.ts-web-journeys-move-model-movesteps</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:185</code> | <code>export function moveSteps(): readonly [MoveStepCopy, MoveStepCopy, MoveStepCopy]</code> |
| <code>interface.14.ts-web-journeys-move-model-movesummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:552</code> | <code>export function moveSummary(draft: MoveSummaryDraft, quote?: MoveQuote): readonly MoveSummaryItem[]</code> |
| <code>interface.14.ts-web-journeys-move-model-movesummarydraft</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:540</code> | <code>export interface MoveSummaryDraft</code> |
| <code>interface.14.ts-web-journeys-move-model-movesummaryitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:546</code> | <code>export interface MoveSummaryItem</code> |
| <code>interface.14.ts-web-journeys-move-model-newmoveattemptkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:332</code> | <code>export function newMoveAttemptKey(): string</code> |
| <code>interface.14.ts-web-journeys-move-model-other-account-option</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:30</code> | <code>export const OTHER_ACCOUNT_OPTION = "other-account";</code> |
| <code>interface.14.ts-web-journeys-move-model-parsemoveamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:132</code> | <code>export function parseMoveAmount(input: string): bigint &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-move-model-pending-move-storage-key</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:32</code> | <code>export const PENDING_MOVE_STORAGE_KEY = "layerx.move.pending.v1";</code> |
| <code>interface.14.ts-web-journeys-move-model-pendingmoveattempt</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:69</code> | <code>export interface PendingMoveAttempt</code> |
| <code>interface.14.ts-web-journeys-move-model-pendingmoveattempt-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:96</code> | <code>export function pendingMoveAttempt( storage: Pick&lt;Storage, "getItem" &#124; "removeItem"&gt;, source: string &#124; undefined, ): PendingMoveAttempt &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-move-model-persistpendingmove</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:117</code> | <code>export function persistPendingMove( storage: Pick&lt;Storage, "setItem"&gt;, attempt: PendingMoveAttempt, ): void</code> |
| <code>interface.14.ts-web-journeys-move-model-quoteexpired</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:327</code> | <code>export function quoteExpired(quote: MoveQuote, now: Date = new Date()): boolean</code> |
| <code>interface.14.ts-web-journeys-move-model-receiptbacked</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/move/model.ts:397</code> | <code>export function receiptBacked(journey: Journey): boolean</code> |
| <code>interface.14.ts-web-journeys-move-model-receiptevidence</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:408</code> | <code>export function receiptEvidence(journey: Journey): readonly EvidenceRef[]</code> |
| <code>interface.14.ts-web-journeys-move-model-requestmovequote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:267</code> | <code>export async function requestMoveQuote( client: HumanApiClient, input: MoveQuoteInput, ): Promise&lt;MoveQuoteOutcome&gt;</code> |
| <code>interface.14.ts-web-journeys-move-model-stillcheckinglookup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:525</code> | <code>export function stillCheckingLookup( client: HumanApiClient, journeyId: string, onJourney: (journey: Journey) =&gt; void, ): () =&gt; Promise&lt;"pending" &#124; "resolved"&gt;</code> |
| <code>interface.14.ts-web-journeys-move-model-storedaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:56</code> | <code>export function storedAccount(storage: Pick&lt;Storage, "getItem"&gt;): string &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-move-model-timestampwords</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/model.ts:158</code> | <code>export function timestampWords(timestamp: string): string</code> |
| <code>interface.14.ts-web-journeys-move-model-verifiedoutcomeevidence</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/move/model.ts:387</code> | <code>export function verifiedOutcomeEvidence(item: EvidenceRef): boolean</code> |
| <code>interface.14.ts-web-journeys-move-move-movemoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/move/move.tsx:314</code> | <code>export function MoveMoney()</code> |
| <code>interface.14.ts-web-journeys-notifications-controller-notificationlanding</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/controller.ts:19</code> | <code>export interface NotificationLanding</code> |
| <code>interface.14.ts-web-journeys-notifications-controller-notifications</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/controller.ts:24</code> | <code>export class Notifications</code> |
| <code>interface.14.ts-web-journeys-notifications-controller-notificationsoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/controller.ts:15</code> | <code>export interface NotificationsOptions</code> |
| <code>interface.14.ts-web-journeys-notifications-model-notificationitems</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/model.ts:102</code> | <code>export function notificationItems( notifications: readonly PresentedNotification[], ): KitNotificationItem[]</code> |
| <code>interface.14.ts-web-journeys-notifications-model-notifications-route</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/model.ts:12</code> | <code>export const NOTIFICATIONS_ROUTE = "/app/notifications";</code> |
| <code>interface.14.ts-web-journeys-notifications-model-presentednotification</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/model.ts:14</code> | <code>export interface PresentedNotification</code> |
| <code>interface.14.ts-web-journeys-notifications-model-presentednotifications</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/model.ts:34</code> | <code>export function presentedNotifications(page: NotificationPage): readonly PresentedNotification[]</code> |
| <code>interface.14.ts-web-journeys-notifications-model-safedeeplink</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/model.ts:60</code> | <code>export function safeDeepLink(value: string): string &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-notifications-model-unreadnotificationcount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/model.ts:56</code> | <code>export function unreadNotificationCount(notifications: readonly PresentedNotification[]): number</code> |
| <code>interface.14.ts-web-journeys-notifications-screens-notificationbyid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/screens.tsx:104</code> | <code>export function notificationById( notifications: readonly PresentedNotification[], id: string, ): PresentedNotification &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-notifications-screens-notificationsarchivescreen</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/screens.tsx:17</code> | <code>export function NotificationsArchiveScreen()</code> |
| <code>interface.14.ts-web-journeys-notifications-store-notificationcenterprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/store.tsx:39</code> | <code>export function NotificationCenterProvider({ children, client: suppliedClient, }: Readonly&lt;{ children: ReactNode; client?: HumanApiClient; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-notifications-store-notificationcenterstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/store.tsx:20</code> | <code>export type NotificationCenterState =</code> |
| <code>interface.14.ts-web-journeys-notifications-store-usenotificationcenter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/notifications/store.tsx:152</code> | <code>export function useNotificationCenter(): NotificationCenterValue</code> |
| <code>interface.14.ts-web-journeys-onboarding-ceremony-ceremonycancelled</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/ceremony.ts:1</code> | <code>export class CeremonyCancelled extends Error</code> |
| <code>interface.14.ts-web-journeys-onboarding-ceremony-performassertionceremony</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/ceremony.ts:194</code> | <code>export async function performAssertionCeremony(ceremony: string): Promise&lt;string&gt;</code> |
| <code>interface.14.ts-web-journeys-onboarding-ceremony-performregistrationceremony</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/ceremony.ts:137</code> | <code>export async function performRegistrationCeremony(ceremony: string): Promise&lt;string&gt;</code> |
| <code>interface.14.ts-web-journeys-onboarding-devices-devicelist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/devices.tsx:27</code> | <code>export function DeviceList()</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-accountactive</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/onboarding/model.ts:62</code> | <code>export function accountActive(journey: Journey): boolean</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-completedstageids</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/model.ts:67</code> | <code>export function completedStageIds(journey: Journey): ReadonlySet&lt;string&gt;</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-create-account-decisions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/model.ts:6</code> | <code>export const CREATE_ACCOUNT_DECISIONS = 2;</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-journeystatuskey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/model.ts:11</code> | <code>export function journeyStatusKey(state: JourneyState): StatusKey</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-onboarding-decision-limit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/model.ts:5</code> | <code>export const ONBOARDING_DECISION_LIMIT = 3;</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-protocol-identity-stage-key</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/model.ts:9</code> | <code>export const PROTOCOL_IDENTITY_STAGE_KEY = "onboarding.stage.setting-up-your-protocol-identity";</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-sign-in-decisions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/model.ts:7</code> | <code>export const SIGN_IN_DECISIONS = 1;</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-stagepresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/model.ts:32</code> | <code>export interface StagePresentation</code> |
| <code>interface.14.ts-web-journeys-onboarding-model-stagepresentation-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/model.ts:42</code> | <code>export function stagePresentation(stage: JourneyStage): StagePresentation</code> |
| <code>interface.14.ts-web-journeys-onboarding-onboarding-onboarding</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/journeys/onboarding/onboarding.tsx:175</code> | <code>export function Onboarding({ initialSelection, initiallyAuthenticated = false, returnTo, }: Readonly&lt;{ initialSelection: ShellSelection; initiallyAuthenticated?: boolean; returnTo?: string &#124; undefined; }&gt;)</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-challengeholdpresentation</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/model.ts:46</code> | <code>export interface ChallengeHoldPresentation</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-challengeholdpresentation-2</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/withdraw/model.ts:53</code> | <code>export function challengeHoldPresentation( journey: Journey, timing: CustodyTiming, ): ChallengeHoldPresentation &#124; undefined</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-settlementpresentation</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/model.ts:31</code> | <code>export interface SettlementPresentation</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-settlementpresentation-2</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/withdraw/model.ts:36</code> | <code>export function settlementPresentation(timing: CustodyTiming): SettlementPresentation</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-withdraw-final-stage</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/withdraw/model.ts:28</code> | <code>export const WITHDRAW_FINAL_STAGE = "withdraw.stage.paying-out";</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-withdraw-hold-stage</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/model.ts:29</code> | <code>export const WITHDRAW_HOLD_STAGE = "withdraw.stage.challenge-hold";</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawcontroller</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/withdraw/model.ts:206</code> | <code>export class WithdrawController</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawcontrolleroptions</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/model.ts:200</code> | <code>export interface WithdrawControllerOptions</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawplan</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/model.ts:86</code> | <code>export interface WithdrawPlan</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawplan-2</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/journeys/withdraw/model.ts:114</code> | <code>export function withdrawPlan(input: WithdrawPlanInput): WithdrawPlan</code> |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawplaninput</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/model.ts:76</code> | <code>export interface WithdrawPlanInput</code> |
| <code>interface.14.ts-web-journeys-withdraw-screen-challengeholdview</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/screen.tsx:48</code> | <code>export function ChallengeHoldView({ hold }: Readonly&lt;{ hold: ChallengeHoldPresentation }&gt;)</code> |
| <code>interface.14.ts-web-journeys-withdraw-screen-settlementnotice</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/screen.tsx:38</code> | <code>export function SettlementNotice({ settlement }: Readonly&lt;{ settlement: SettlementPresentation }&gt;)</code> |
| <code>interface.14.ts-web-journeys-withdraw-screen-withdraw</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/screen.tsx:68</code> | <code>export function Withdraw({ timing }: Readonly&lt;{ timing: CustodyTiming }&gt;)</code> |
| <code>interface.14.ts-web-journeys-withdraw-screen-withdrawscreen</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/journeys/withdraw/screen.tsx:338</code> | <code>export const WithdrawScreen = Withdraw;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-journeys-activity-activity-human-web-journeys-activity</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-activitydate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-allentryids</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-entrystatuslabel</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-entryunresolved</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-entryverification</code> | overridden_by | override.J53.38 |
| <code>interface.14.ts-web-journeys-activity-model-feedfilterdef</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-feedfilteroption</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-feedgroupview</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-feedisempty</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-feedrowview</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-receiptbacked</code> | overridden_by | override.J53.39 |
| <code>interface.14.ts-web-journeys-activity-model-status-key-by-state</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-timelinestageview</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-activity-model-verificationlabel</code> | overridden_by | override.J53.37 |
| <code>interface.14.ts-web-journeys-agents-model-agentcontrolid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-agentpresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-agentslayout</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-agentsoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-agenttone</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-catalogsentence</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-creation-decisions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-creationstep</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-evidencereceiptverified</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-evidencereceiptverified</code> | overridden_by | override.J53.32 |
| <code>interface.14.ts-web-journeys-agents-model-journeystatuskey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-quotepresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-rotationtiming</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-spendpresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-stagereceiptverified</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-agents-model-stagereceiptverified</code> | overridden_by | override.J53.33 |
| <code>interface.14.ts-web-journeys-agents-model-stageview</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-ceremony-base64urlfrombytes</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-ceremony-base64urlfromtext</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-ceremony-bytesfrombase64url</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-ceremony-textfrombase64url</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-controller-approvalsoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-approvalinboxitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-approvals-route</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-approvalstatecopykey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-approvalstatepresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-expirycountdown</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-money-locale</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-pendingapprovalcount</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-protocolmoney</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-model-verificationlabel</code> | overridden_by | override.J53.36 |
| <code>interface.14.ts-web-journeys-approvals-screens-approvalcountdown</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-screens-approvalinboxlist</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-screens-approvaloutcomepanel</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-screens-approvalstatebadge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-approvals-screens-releasedactivitysection</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-custody-copy-custodycopykey</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-evidence-depositcomplete</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-evidence-exitcomplete</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-evidence-presentedjourneystate</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:24 |
| <code>interface.14.ts-web-journeys-custody-evidence-presentedstagestate</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody-evidence.test.ts:21 |
| <code>interface.14.ts-web-journeys-custody-evidence-stageevidencebacked</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody-evidence.test.ts:21 |
| <code>interface.14.ts-web-journeys-custody-evidence-stageevidencerule</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody-evidence.test.ts:21 |
| <code>interface.14.ts-web-journeys-custody-evidence-stageoutcomeverified</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-evidence-statuskeyforstate</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-evidence-verificationatleast</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/src/settings/wallet/model.ts:15 |
| <code>interface.14.ts-web-journeys-custody-evidence-withdrawpaidout</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-handoff-browserwalletbridge</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody-evidence.test.ts:16 |
| <code>interface.14.ts-web-journeys-custody-handoff-custodybridgeoptions</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-handoff-custodyfromwalletsignrequest</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody-evidence.test.ts:16 |
| <code>interface.14.ts-web-journeys-custody-handoff-custodyhandoffrecord</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-handoff-custodyhandoffstorage</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-handoff-custodyhandoffstoragekey</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody-evidence.test.ts:16 |
| <code>interface.14.ts-web-journeys-custody-handoff-eip1193provider</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/src/settings/wallet/bridge.ts:2 |
| <code>interface.14.ts-web-journeys-custody-handoff-paxeerwalletbridge</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:26 |
| <code>interface.14.ts-web-journeys-custody-handoff-wallethandoff</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-handoff-wallethandoffphase</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-handoff-walletsignoutcome</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-handoff-windowwalletprovider</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/src/api/wallet.ts:3, apps/web/src/app/exchange/exchange-client.tsx:17, apps/web/src/app/launchpad/launchpad-client.tsx:16, apps/web/src/settings/wallet/wallet-screen.tsx:20 |
| <code>interface.14.ts-web-journeys-custody-journey-screen-journeyscreen</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-custody-currency</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:4 |
| <code>interface.14.ts-web-journeys-custody-model-custodyapplicationpath</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-custodyshell</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-journeytimeline</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/src/settings/wallet/wallet-screen.tsx:19 |
| <code>interface.14.ts-web-journeys-custody-model-newidempotencykey</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/src/settings/wallet/model.ts:16 |
| <code>interface.14.ts-web-journeys-custody-model-randombytes</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-refusalpresentation</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-refusalpresentation-2</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-timelinerow</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-validatedestinationaddress</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-validatepositiveamount</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-walletpanel</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-model-walletpanelplan</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-recovery-isjourneyoutcomeunknown</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-recovery-journeyoutcomeunknownerror</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-recovery-mutationoutcomeisunknown</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-recovery-unknownoutcomerecovery</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-time-custodytiming</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:27 |
| <code>interface.14.ts-web-journeys-custody-time-custodytimingfromenv</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-time-elapsedseconds</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-time-plainduration</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-time-settlementdeclaration</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-time-settlementexpectationseconds</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-time-stagedelayed</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-timeline-completeview</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-timeline-delaynotice</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-timeline-journeytechnicaldetails</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/src/settings/wallet/wallet-screen.tsx:15 |
| <code>interface.14.ts-web-journeys-custody-timeline-journeytimelineview</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013); relocate before removal: used outside the retired set at apps/web/src/settings/wallet/wallet-screen.tsx:15 |
| <code>interface.14.ts-web-journeys-custody-timeline-refusalview</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-timeline-safetoclosenotice</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-timeline-usecustodyshell</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-custody-timeline-walletpanelview</code> | notes | stale req.8033 removes the custody hand-off and deposit journey (no web path calls 0x1013) |
| <code>interface.14.ts-web-journeys-deposit-model-deposit-final-stage</code> | notes | stale req.8033 removes the deposit journey and its routes; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:5 |
| <code>interface.14.ts-web-journeys-deposit-model-depositcontroller</code> | notes | stale req.8033 removes the deposit journey and its routes; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:5 |
| <code>interface.14.ts-web-journeys-deposit-model-depositcontrolleroptions</code> | notes | stale req.8033 removes the deposit journey and its routes |
| <code>interface.14.ts-web-journeys-deposit-model-depositplan</code> | notes | stale req.8033 removes the deposit journey and its routes |
| <code>interface.14.ts-web-journeys-deposit-model-depositplan-2</code> | notes | stale req.8033 removes the deposit journey and its routes; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:5 |
| <code>interface.14.ts-web-journeys-deposit-model-depositplaninput</code> | notes | stale req.8033 removes the deposit journey and its routes |
| <code>interface.14.ts-web-journeys-deposit-screen-deposit</code> | notes | stale req.8033 removes the deposit journey and its routes |
| <code>interface.14.ts-web-journeys-deposit-screen-depositscreen</code> | notes | stale req.8033 removes the deposit journey and its routes |
| <code>interface.14.ts-web-journeys-exit-model-exit-confirmation-phrase</code> | notes | stale req.8035 removes the forced-exit journey; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:17 |
| <code>interface.14.ts-web-journeys-exit-model-exit-final-stage</code> | notes | stale req.8035 removes the forced-exit journey; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:17 |
| <code>interface.14.ts-web-journeys-exit-model-exitconfirmationerror</code> | notes | stale req.8035 removes the forced-exit journey |
| <code>interface.14.ts-web-journeys-exit-model-exitconfirmationready</code> | notes | stale req.8035 removes the forced-exit journey; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:17 |
| <code>interface.14.ts-web-journeys-exit-model-exitcontroller</code> | notes | stale req.8035 removes the forced-exit journey; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:17 |
| <code>interface.14.ts-web-journeys-exit-model-exitcontrolleroptions</code> | notes | stale req.8035 removes the forced-exit journey |
| <code>interface.14.ts-web-journeys-exit-model-exitplan</code> | notes | stale req.8035 removes the forced-exit journey |
| <code>interface.14.ts-web-journeys-exit-model-exitplan-2</code> | notes | stale req.8035 removes the forced-exit journey; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:17 |
| <code>interface.14.ts-web-journeys-exit-model-exitplaninput</code> | notes | stale req.8035 removes the forced-exit journey |
| <code>interface.14.ts-web-journeys-exit-screen-exit</code> | notes | stale req.8035 removes the forced-exit journey |
| <code>interface.14.ts-web-journeys-exit-screen-exitscreen</code> | notes | stale req.8035 removes the forced-exit journey |
| <code>interface.14.ts-web-journeys-home-model-home-activity-limit</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-home-model-home-destinations</code> | overridden_by | override.J53.6 |
| <code>interface.14.ts-web-journeys-home-model-homeactionitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-home-model-homeactivityrow</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-home-model-homeagentrow</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-home-model-homebalance</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-home-model-homeloadfailure</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-account-storage-key</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-move-decisions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-move-step-ids</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movecommitoutcome</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movedestinationoption</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movequoteinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movequoteoutcome</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-moveresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movereview</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movestagepresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movestages</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movestepcopy</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movesummarydraft</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-movesummaryitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-pending-move-storage-key</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-receiptbacked</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-receiptbacked</code> | overridden_by | override.J53.35 |
| <code>interface.14.ts-web-journeys-move-model-receiptevidence</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-move-model-verifiedoutcomeevidence</code> | overridden_by | override.J53.34 |
| <code>interface.14.ts-web-journeys-notifications-controller-notificationsoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-notifications-screens-notificationbyid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-notifications-store-notificationcenterstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-onboarding-model-accountactive</code> | overridden_by | override.J53.31 |
| <code>interface.14.ts-web-journeys-onboarding-model-completedstageids</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-onboarding-model-create-account-decisions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-onboarding-model-journeystatuskey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-onboarding-model-protocol-identity-stage-key</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-onboarding-model-sign-in-decisions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-onboarding-model-stagepresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-journeys-withdraw-model-challengeholdpresentation</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-model-challengeholdpresentation-2</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:10 |
| <code>interface.14.ts-web-journeys-withdraw-model-settlementpresentation</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-model-settlementpresentation-2</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:10 |
| <code>interface.14.ts-web-journeys-withdraw-model-withdraw-final-stage</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:10 |
| <code>interface.14.ts-web-journeys-withdraw-model-withdraw-hold-stage</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawcontroller</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:10 |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawcontrolleroptions</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawplan</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawplan-2</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut; relocate before removal: used outside the retired set at apps/web/e2e/custody.spec.ts:10 |
| <code>interface.14.ts-web-journeys-withdraw-model-withdrawplaninput</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-screen-challengeholdview</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-screen-settlementnotice</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-screen-withdraw</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |
| <code>interface.14.ts-web-journeys-withdraw-screen-withdrawscreen</code> | notes | stale req.8034 removes the withdraw journey; outward moves use bridgeOut |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.48.1</code> | req | 14048.1 |
| <code>acceptance.14.48.1</code> | scenario | Given the edited home model, when home_destinations_send_outward_moves_to_bridge reads HOME_DESTINATIONS, then add and withdraw equal /bridge |
| <code>acceptance.14.48.1</code> | test | apps/web/e2e/home-destinations.test.ts::home_destinations_send_outward_moves_to_bridge (planned) |
| <code>acceptance.14.48.1</code> | qualification | UNRUN |
| <code>req.14048</code> | ac_1 | HOME_DESTINATIONS (apps/web/src/journeys/home/model.ts:29) names /bridge for add and withdraw and no /app/deposit or /app/withdraw path. |
| <code>acceptance.14.48.2</code> | req | 14048.2 |
| <code>acceptance.14.48.2</code> | scenario | Given the edited home model, when home_actions_link_no_retired_route calls homeActions, then every href is outside the three retired prefixes |
| <code>acceptance.14.48.2</code> | test | apps/web/e2e/home-destinations.test.ts::home_actions_link_no_retired_route (planned) |
| <code>acceptance.14.48.2</code> | qualification | UNRUN |
| <code>req.14048</code> | ac_2 | No href returned by homeActions (apps/web/src/journeys/home/model.ts:45) starts with /app/deposit, /app/withdraw or /app/journeys. |
| <code>acceptance.14.49.1</code> | req | 14049.1 |
| <code>acceptance.14.49.1</code> | scenario | Given the deletions, when retired_journey_modules_do_not_resolve imports the three journey indexes, then each import rejects with code ERR_MODULE_NOT_FOUND |
| <code>acceptance.14.49.1</code> | test | apps/web/e2e/retired-journeys.test.ts::retired_journey_modules_do_not_resolve (planned) |
| <code>acceptance.14.49.1</code> | qualification | UNRUN |
| <code>req.14049</code> | ac_1 | Dynamic import of apps/web/src/journeys/deposit/index.ts:1, apps/web/src/journeys/withdraw/index.ts:1 and apps/web/src/journeys/exit/index.ts:1 rejects with ERR_MODULE_NOT_FOUND. |
| <code>acceptance.14.49.2</code> | req | 14049.2 |
| <code>acceptance.14.49.2</code> | scenario | Given the edited package.json, when web_test_list_names_no_custody_suite parses scripts.test, then no custody suite is listed and every listed file exists under apps/web |
| <code>acceptance.14.49.2</code> | test | apps/web/e2e/retired-journeys.test.ts::web_test_list_names_no_custody_suite (planned) |
| <code>acceptance.14.49.2</code> | qualification | UNRUN |
| <code>req.14049</code> | ac_2 | The test script of apps/web/package.json:13 names neither e2e/custody.spec.ts nor e2e/custody-evidence.test.ts and every file it names exists. |
| <code>acceptance.14.50.1</code> | req | 14050.1 |
| <code>acceptance.14.50.1</code> | scenario | Given the deletion, when custody_screens_do_not_resolve imports both modules, then each import rejects with code ERR_MODULE_NOT_FOUND |
| <code>acceptance.14.50.1</code> | test | apps/web/e2e/retired-custody-screens.test.ts::custody_screens_do_not_resolve (planned) |
| <code>acceptance.14.50.1</code> | qualification | UNRUN |
| <code>req.14050</code> | ac_1 | Dynamic import of apps/web/src/journeys/custody/index.ts:1 and apps/web/src/journeys/custody/journey-screen.tsx:1 rejects with ERR_MODULE_NOT_FOUND. |
| <code>acceptance.14.50.2</code> | req | 14050.2 |
| <code>acceptance.14.50.2</code> | scenario | Given no window.ethereum, when wallet_api_imports_no_custody_module imports apps/web/src/api/wallet.ts and calls connectWalletAccount, then it resolves to undefined without throwing |
| <code>acceptance.14.50.2</code> | test | apps/web/e2e/retired-custody-screens.test.ts::wallet_api_imports_no_custody_module (planned) |
| <code>acceptance.14.50.2</code> | qualification | UNRUN |
| <code>req.14050</code> | ac_2 | apps/web/src/api/wallet.ts and the exchange and launchpad clients still load and connectWalletAccount (apps/web/src/api/wallet.ts:42) returns undefined with no window wallet, now that their provider comes from apps/web/src/api/provider.ts. |
| <code>acceptance.14.51.1</code> | req | 14051.1 |
| <code>acceptance.14.51.1</code> | scenario | Given the deletion, when custody_journey_module_does_not_resolve imports the three modules, then each import rejects with code ERR_MODULE_NOT_FOUND |
| <code>acceptance.14.51.1</code> | test | apps/web/e2e/retired-custody.test.ts::custody_journey_module_does_not_resolve (planned) |
| <code>acceptance.14.51.1</code> | qualification | UNRUN |
| <code>req.14051</code> | ac_1 | Dynamic import of apps/web/src/journeys/custody/handoff.ts:1, apps/web/src/journeys/custody/model.ts:1 and apps/web/src/journeys/custody/time.ts:1 rejects with ERR_MODULE_NOT_FOUND. |
| <code>acceptance.14.51.2</code> | req | 14051.2 |
| <code>acceptance.14.51.2</code> | scenario | Given the deletion, when web_source_reads_no_human_settlement_variable scans apps/web/src for the five names, then no match is found |
| <code>acceptance.14.51.2</code> | test | apps/web/e2e/retired-custody.test.ts::web_source_reads_no_human_settlement_variable (planned) |
| <code>acceptance.14.51.2</code> | qualification | UNRUN |
| <code>req.14051</code> | ac_2 | No file under apps/web/src reads HUMAN_SETTLEMENT_CHECKPOINT_INTERVAL_SECONDS or the four other variables read at apps/web/src/journeys/custody/time.ts:27-31 at HEAD. |
| <code>acceptance.14.55.1</code> | req | 14055.1 |
| <code>acceptance.14.55.1</code> | scenario | Given a journey with a receipt-verified layerx-receipt on its protocol stage, when onboarding_account_active_on_receipt_rung calls accountActive, then it returns true |
| <code>acceptance.14.55.1</code> | test | apps/web/e2e/onboarding-rungs.test.ts::onboarding_account_active_on_receipt_rung (planned) |
| <code>acceptance.14.55.1</code> | qualification | UNRUN |
| <code>req.14055</code> | ac_1 | accountActive (apps/web/src/journeys/onboarding/model.ts:62) is true for a journey whose protocol stage carries layerx-receipt evidence at receipt-verified or checkpoint-finalised. |
| <code>acceptance.14.55.2</code> | req | 14055.2 |
| <code>acceptance.14.55.2</code> | scenario | Given a journey whose only layerx-receipt evidence is paxeer-finalised, when onboarding_account_inactive_on_retired_level calls accountActive, then it returns false |
| <code>acceptance.14.55.2</code> | test | apps/web/e2e/onboarding-rungs.test.ts::onboarding_account_inactive_on_retired_level (planned) |
| <code>acceptance.14.55.2</code> | qualification | UNRUN |
| <code>req.14055</code> | ac_2 | accountActive (apps/web/src/journeys/onboarding/model.ts:62) is false when the only receipt evidence is paxeer-finalised, a level retired by decision.1002. |
| <code>acceptance.14.56.1</code> | req | 14056.1 |
| <code>acceptance.14.56.1</code> | scenario | Given three evidence lists at receipt-verified, paxeer-finalised and settlement-anchored, when agent_evidence_verified_on_receipt_rungs_only calls evidenceReceiptVerified on each, then only the first returns true |
| <code>acceptance.14.56.1</code> | test | apps/web/e2e/agents-rungs.test.ts::agent_evidence_verified_on_receipt_rungs_only (planned) |
| <code>acceptance.14.56.1</code> | qualification | UNRUN |
| <code>req.14056</code> | ac_1 | evidenceReceiptVerified (apps/web/src/journeys/agents/model.ts:163) is true for receipt-verified layerx-receipt evidence and false for paxeer-finalised or settlement-anchored evidence. |
| <code>acceptance.14.56.2</code> | req | 14056.2 |
| <code>acceptance.14.56.2</code> | scenario | Given an agent whose spend verification is settlement-anchored, when agent_spend_sentence_for_retired_level_is_unverified builds its row, then verificationSentence equals the verification.unverified message |
| <code>acceptance.14.56.2</code> | test | apps/web/e2e/agents-rungs.test.ts::agent_spend_sentence_for_retired_level_is_unverified (planned) |
| <code>acceptance.14.56.2</code> | qualification | UNRUN |
| <code>req.14056</code> | ac_2 | The agent row built at apps/web/src/journeys/agents/model.ts:361 for a spend at settlement-anchored carries the verification.unverified sentence. |
| <code>acceptance.14.57.1</code> | req | 14057.1 |
| <code>acceptance.14.57.1</code> | scenario | Given layerx-receipt evidence at each of the two rungs, when move_outcome_verified_on_receipt_rungs calls verifiedOutcomeEvidence, then both return true |
| <code>acceptance.14.57.1</code> | test | apps/web/e2e/move-rungs.test.ts::move_outcome_verified_on_receipt_rungs (planned) |
| <code>acceptance.14.57.1</code> | qualification | UNRUN |
| <code>req.14057</code> | ac_1 | verifiedOutcomeEvidence (apps/web/src/journeys/move/model.ts:387) is true for layerx-receipt evidence at receipt-verified or checkpoint-finalised. |
| <code>acceptance.14.57.2</code> | req | 14057.2 |
| <code>acceptance.14.57.2</code> | scenario | Given a journey whose only evidence is paxeer-finality at paxeer-finalised, when move_outcome_unverified_on_paxeer_finality calls receiptBacked, then it returns false |
| <code>acceptance.14.57.2</code> | test | apps/web/e2e/move-rungs.test.ts::move_outcome_unverified_on_paxeer_finality (planned) |
| <code>acceptance.14.57.2</code> | qualification | UNRUN |
| <code>req.14057</code> | ac_2 | verifiedOutcomeEvidence (apps/web/src/journeys/move/model.ts:387) is false for paxeer-finality evidence and for layerx-receipt evidence at paxeer-finalised, and receiptBacked (apps/web/src/journeys/move/model.ts:397) is false for a journey carrying only those. |
| <code>acceptance.14.58.1</code> | req | 14058.1 |
| <code>acceptance.14.58.1</code> | scenario | Given the two rung levels, when approval_verification_label_for_rungs calls verificationLabel, then each result equals its catalogue message |
| <code>acceptance.14.58.1</code> | test | apps/web/e2e/approvals-rungs.test.ts::approval_verification_label_for_rungs (planned) |
| <code>acceptance.14.58.1</code> | qualification | UNRUN |
| <code>req.14058</code> | ac_1 | verificationLabel (apps/web/src/journeys/approvals/model.ts:97) returns the verification.receipt_verified message for receipt-verified and the verification.checkpoint_finalised message for checkpoint-finalised. |
| <code>acceptance.14.58.2</code> | req | 14058.2 |
| <code>acceptance.14.58.2</code> | scenario | Given the two retired levels, when approval_verification_label_for_retired_level calls verificationLabel, then both results equal the verification.unverified message |
| <code>acceptance.14.58.2</code> | test | apps/web/e2e/approvals-rungs.test.ts::approval_verification_label_for_retired_level (planned) |
| <code>acceptance.14.58.2</code> | qualification | UNRUN |
| <code>req.14058</code> | ac_2 | verificationLabel (apps/web/src/journeys/approvals/model.ts:97) returns the verification.unverified message for paxeer-finalised and settlement-anchored. |
| <code>acceptance.14.59.1</code> | req | 14059.1 |
| <code>acceptance.14.59.1</code> | scenario | Given two evidence lists, when activity_entry_verification_ranks_three_rungs calls entryVerification on each, then the results are checkpoint-finalised and unverified |
| <code>acceptance.14.59.1</code> | test | apps/web/e2e/activity-rungs.test.ts::activity_entry_verification_ranks_three_rungs (planned) |
| <code>acceptance.14.59.1</code> | qualification | UNRUN |
| <code>req.14059</code> | ac_1 | entryVerification (apps/web/src/journeys/activity/model.ts:205) returns checkpoint-finalised for evidence at receipt-verified and checkpoint-finalised, and unverified for evidence only at paxeer-finalised or settlement-anchored. |
| <code>acceptance.14.59.2</code> | req | 14059.2 |
| <code>acceptance.14.59.2</code> | scenario | Given layerx-receipt evidence at three levels, when activity_receipt_backed_refuses_retired_levels calls receiptBacked on each, then only receipt-verified returns true |
| <code>acceptance.14.59.2</code> | test | apps/web/e2e/activity-rungs.test.ts::activity_receipt_backed_refuses_retired_levels (planned) |
| <code>acceptance.14.59.2</code> | qualification | UNRUN |
| <code>req.14059</code> | ac_2 | receiptBacked (apps/web/src/journeys/activity/model.ts:428) is false for layerx-receipt evidence at paxeer-finalised or settlement-anchored and true at receipt-verified. |
| <code>capability.14.7.2</code> | title | Home actions to the bridge |
| <code>capability.14.7.2</code> | reqs | 14048.1<br>14048.2 |
| <code>capability.14.7.3</code> | title | Deposit, withdraw and exit journeys deleted |
| <code>capability.14.7.3</code> | reqs | 14049.1<br>14049.2 |
| <code>capability.14.7.4</code> | title | Custody journey screens deleted |
| <code>capability.14.7.4</code> | reqs | 14050.1<br>14050.2 |
| <code>capability.14.7.5</code> | title | Custody hand-off journey deleted |
| <code>capability.14.7.5</code> | reqs | 14051.1<br>14051.2 |
| <code>capability.14.9.2</code> | title | Onboarding on three rungs |
| <code>capability.14.9.2</code> | reqs | 14055.1<br>14055.2 |
| <code>capability.14.9.3</code> | title | Agents on three rungs |
| <code>capability.14.9.3</code> | reqs | 14056.1<br>14056.2 |
| <code>capability.14.9.4</code> | title | Move outcomes on three rungs |
| <code>capability.14.9.4</code> | reqs | 14057.1<br>14057.2 |
| <code>capability.14.9.5</code> | title | Approvals on three rungs |
| <code>capability.14.9.5</code> | reqs | 14058.1<br>14058.2 |
| <code>capability.14.9.6</code> | title | Activity on three rungs |
| <code>capability.14.9.6</code> | reqs | 14059.1<br>14059.2 |

## kit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-kit-a11y-a11y</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:40</code> | <code>export function a11y()</code> |
| <code>interface.14.ts-web-kit-a11y-assertsemantictokencontrast</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:123</code> | <code>export function assertSemanticTokenContrast(stylesheet: string): void</code> |
| <code>interface.14.ts-web-kit-a11y-contrastratio</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:86</code> | <code>export function contrastRatio(foreground: string, background: string): number</code> |
| <code>interface.14.ts-web-kit-a11y-explicitcurrencyamountoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:44</code> | <code>export interface ExplicitCurrencyAmountOptions</code> |
| <code>interface.14.ts-web-kit-a11y-formatexplicitcurrencyamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:134</code> | <code>export function formatExplicitCurrencyAmount( value: number, { currency, locale, decimals = 2, signed = true }: ExplicitCurrencyAmountOptions, ): string</code> |
| <code>interface.14.ts-web-kit-a11y-liveregion</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:163</code> | <code>export function LiveRegion({ children, priority = "polite", className, ...props }: LiveRegionProps)</code> |
| <code>interface.14.ts-web-kit-a11y-liveregionprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:158</code> | <code>export interface LiveRegionProps extends Omit&lt;HTMLAttributes&lt;HTMLSpanElement&gt;, "role"&gt;</code> |
| <code>interface.14.ts-web-kit-a11y-minimum-text-contrast</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:3</code> | <code>export const MINIMUM_TEXT_CONTRAST = 4.5;</code> |
| <code>interface.14.ts-web-kit-a11y-semantic-contrast-pairs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:5</code> | <code>export const SEMANTIC_CONTRAST_PAIRS = [</code> |
| <code>interface.14.ts-web-kit-a11y-semanticcontrastresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:51</code> | <code>export interface SemanticContrastResult</code> |
| <code>interface.14.ts-web-kit-a11y-semanticcontrastresults</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:107</code> | <code>export function semanticContrastResults(stylesheet: string): readonly SemanticContrastResult[]</code> |
| <code>interface.14.ts-web-kit-a11y-semantictokenvalues</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/a11y.ts:95</code> | <code>export function semanticTokenValues(stylesheet: string): ReadonlyMap&lt;string, string&gt;</code> |
| <code>interface.14.ts-web-kit-activity-activityevidencebadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/activity.tsx:163</code> | <code>export function ActivityEvidenceBadge({ children }: Readonly&lt;{ children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-activity-activityfeedgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/activity.tsx:19</code> | <code>export interface ActivityFeedGroup</code> |
| <code>interface.14.ts-web-kit-activity-activityfeedprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/activity.tsx:26</code> | <code>export interface ActivityFeedProps</code> |
| <code>interface.14.ts-web-kit-activity-activityfeedrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/activity.tsx:9</code> | <code>export interface ActivityFeedRow</code> |
| <code>interface.14.ts-web-kit-activity-desktopactivityfeed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/activity.tsx:88</code> | <code>export function DesktopActivityFeed({ groups, onSelect, dateLabel, columns, amountSortEnabled }: ActivityFeedProps)</code> |
| <code>interface.14.ts-web-kit-activity-mobileactivityfeed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/activity.tsx:39</code> | <code>export function MobileActivityFeed({ groups, onSelect, dateLabel }: ActivityFeedProps)</code> |
| <code>interface.14.ts-web-kit-confirm-confirmationprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/confirm.tsx:34</code> | <code>export type ConfirmationProps =</code> |
| <code>interface.14.ts-web-kit-confirm-desktopconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/confirm.tsx:108</code> | <code>export function DesktopConfirmation(props: ConfirmationProps)</code> |
| <code>interface.14.ts-web-kit-confirm-mobileconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/confirm.tsx:104</code> | <code>export function MobileConfirmation(props: ConfirmationProps)</code> |
| <code>interface.14.ts-web-kit-confirm-typedconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/confirm.tsx:27</code> | <code>export interface TypedConfirmation</code> |
| <code>interface.14.ts-web-kit-control-controlavailability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/control.tsx:10</code> | <code>export type ControlAvailability =</code> |
| <code>interface.14.ts-web-kit-control-disabledreason</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/control.tsx:16</code> | <code>export function DisabledReason({ id, children }: Readonly&lt;{ id: string; children?: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-control-kitbutton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/control.tsx:27</code> | <code>export function KitButton({ disabled = false, disabledReason, className, "aria-describedby": describedBy, ...props }: KitButtonProps)</code> |
| <code>interface.14.ts-web-kit-control-kitbuttonprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/control.tsx:14</code> | <code>export type KitButtonProps = Omit&lt;ButtonProps, "disabled"&gt; &amp; ControlAvailability;</code> |
| <code>interface.14.ts-web-kit-control-reduced-motion-class</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/control.tsx:7</code> | <code>export const REDUCED_MOTION_CLASS =</code> |
| <code>interface.14.ts-web-kit-device-list-devicelistitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/device-list.tsx:6</code> | <code>export interface DeviceListItem</code> |
| <code>interface.14.ts-web-kit-device-list-devicesessionlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/device-list.tsx:15</code> | <code>export function DeviceSessionList({ items }: Readonly&lt;{ items: readonly DeviceListItem[] }&gt;)</code> |
| <code>interface.14.ts-web-kit-display-actiongrid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:138</code> | <code>export function ActionGrid({ actions, onAction, className }: ActionGridProps)</code> |
| <code>interface.14.ts-web-kit-display-actiongridicon</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:124</code> | <code>export type ActionGridIcon = keyof typeof ACTION_ICONS;</code> |
| <code>interface.14.ts-web-kit-display-actiongriditem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:126</code> | <code>export interface ActionGridItem</code> |
| <code>interface.14.ts-web-kit-display-actiongridprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:132</code> | <code>export interface ActionGridProps</code> |
| <code>interface.14.ts-web-kit-display-balancesummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:68</code> | <code>export function BalanceSummary({ label, value, currency, hidden, onHiddenChange, className, }: BalanceSummaryProps)</code> |
| <code>interface.14.ts-web-kit-display-balancesummaryprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:59</code> | <code>export interface BalanceSummaryProps</code> |
| <code>interface.14.ts-web-kit-display-countbadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:51</code> | <code>export function CountBadge({ label, ...props }: CountBadgeProps)</code> |
| <code>interface.14.ts-web-kit-display-countbadgeprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:49</code> | <code>export type CountBadgeProps = Omit&lt;BadgeProps, "children"&gt; &amp; Readonly&lt;{ label: string }&gt;;</code> |
| <code>interface.14.ts-web-kit-display-kiticontile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:41</code> | <code>export function KitIconTile(props: KitIconTileProps)</code> |
| <code>interface.14.ts-web-kit-display-kiticontileprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:22</code> | <code>export type KitIconTileProps = ComponentProps&lt;typeof IconTile&gt;;</code> |
| <code>interface.14.ts-web-kit-display-kitlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:25</code> | <code>export function KitList(props: KitListProps)</code> |
| <code>interface.14.ts-web-kit-display-kitlistitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:29</code> | <code>export function KitListItem(props: KitListItemProps)</code> |
| <code>interface.14.ts-web-kit-display-kitlistitemprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:19</code> | <code>export type KitListItemProps = ListItemProps;</code> |
| <code>interface.14.ts-web-kit-display-kitlistprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:18</code> | <code>export type KitListProps = ComponentProps&lt;typeof List&gt;;</code> |
| <code>interface.14.ts-web-kit-display-kitoptionlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:37</code> | <code>export function KitOptionList(props: KitOptionListProps)</code> |
| <code>interface.14.ts-web-kit-display-kitoptionlistprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:21</code> | <code>export type KitOptionListProps = ComponentProps&lt;typeof OptionList&gt;;</code> |
| <code>interface.14.ts-web-kit-display-kitsectionheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:33</code> | <code>export function KitSectionHeader(props: KitSectionHeaderProps)</code> |
| <code>interface.14.ts-web-kit-display-kitsectionheaderprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:20</code> | <code>export type KitSectionHeaderProps = ComponentProps&lt;typeof SectionHeader&gt;;</code> |
| <code>interface.14.ts-web-kit-display-kittextfield</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:91</code> | <code>export function KitTextField({ label, ...props }: KitTextFieldProps)</code> |
| <code>interface.14.ts-web-kit-display-kittextfieldprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:88</code> | <code>export type KitTextFieldProps = Omit&lt;InputProps, "id" &#124; "placeholder"&gt; &amp;</code> |
| <code>interface.14.ts-web-kit-display-kitviewallchip</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:45</code> | <code>export function KitViewAllChip(props: KitViewAllChipProps)</code> |
| <code>interface.14.ts-web-kit-display-kitviewallchipprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:23</code> | <code>export type KitViewAllChipProps = ComponentProps&lt;typeof ViewAllChip&gt;;</code> |
| <code>interface.14.ts-web-kit-display-optionlistitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/display.tsx:16</code> | <code>export type { OptionListItem };</code> |
| <code>interface.14.ts-web-kit-explorer-explorerevidenceinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:153</code> | <code>export function ExplorerEvidenceInput({ className, ...props }: TextareaHTMLAttributes&lt;HTMLTextAreaElement&gt;)</code> |
| <code>interface.14.ts-web-kit-explorer-explorerexternallink</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:84</code> | <code>export function ExplorerExternalLink({ href, children, }: Readonly&lt;{ href: string; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-explorer-explorerfreshness</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:107</code> | <code>export function ExplorerFreshness({ title, description, current, }: Readonly&lt;{ title: string; description: string; current: boolean }&gt;)</code> |
| <code>interface.14.ts-web-kit-explorer-explorerlink</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:80</code> | <code>export function ExplorerLink({ href, children }: Readonly&lt;{ href: string; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-explorer-explorerlookupform</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:128</code> | <code>export function ExplorerLookupForm({ action, kind, label, placeholder, submitLabel, }: Readonly&lt;{ action: string; kind: "receipt" &#124; "account" &#124; "program"; label: string; placeholder: string; submitLabel: string;</code> |
| <code>interface.14.ts-web-kit-explorer-explorernavigation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:10</code> | <code>export function ExplorerNavigation({ label, items, }: Readonly&lt;{ label: string; items: readonly Readonly&lt;{ href: string; label: string }&gt;[] }&gt;)</code> |
| <code>interface.14.ts-web-kit-explorer-explorerpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:30</code> | <code>export function ExplorerPanel({ title, children, }: Readonly&lt;{ title: ReactNode; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-explorer-explorertable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:42</code> | <code>export function ExplorerTable({ caption, columns, rows, }: Readonly&lt;{ caption: string; columns: readonly string[]; rows: readonly Readonly&lt;{ id: string; cells: readonly ReactNode[] }&gt;[]; }&gt;)</code> |
| <code>interface.14.ts-web-kit-explorer-explorerverificationbadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/explorer.tsx:100</code> | <code>export function ExplorerVerificationBadge({ label, unverified = false, }: Readonly&lt;{ label: string; unverified?: boolean }&gt;)</code> |
| <code>interface.14.ts-web-kit-field-textfield</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/field.tsx:14</code> | <code>export function TextField({ label, errorMessage, error, className, ...props }: TextFieldProps)</code> |
| <code>interface.14.ts-web-kit-field-textfieldprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/field.tsx:9</code> | <code>export type TextFieldProps = InputBaseProps &amp; Readonly&lt;</code> |
| <code>interface.14.ts-web-kit-model-confirmationkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:50</code> | <code>export type ConfirmationKind = "reversible" &#124; "destructive" &#124; "irreversible";</code> |
| <code>interface.14.ts-web-kit-model-confirmationvariant</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:85</code> | <code>export function confirmationVariant(kind: ConfirmationKind): "primary" &#124; "destructive"</code> |
| <code>interface.14.ts-web-kit-model-directionword</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:81</code> | <code>export function directionWord(direction: MoneyDirection): string</code> |
| <code>interface.14.ts-web-kit-model-feemathtotal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:97</code> | <code>export function feeMathTotal(steps: readonly Readonly&lt;{ amount: ProtocolAmount }&gt;[]): ProtocolAmount</code> |
| <code>interface.14.ts-web-kit-model-kit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:17</code> | <code>export function kit(): Readonly&lt;</code> |
| <code>interface.14.ts-web-kit-model-moneydirection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:49</code> | <code>export type MoneyDirection = "inbound" &#124; "outbound" &#124; "other";</code> |
| <code>interface.14.ts-web-kit-model-pattern-pairs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:4</code> | <code>export const PATTERN_PAIRS = [</code> |
| <code>interface.14.ts-web-kit-model-protocolamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:52</code> | <code>export type ProtocolAmount = number &amp; Readonly&lt;{ [protocolAmountBrand]: "ProtocolAmount" }&gt;;</code> |
| <code>interface.14.ts-web-kit-model-protocolamount-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:54</code> | <code>export function protocolAmount(value: number): ProtocolAmount</code> |
| <code>interface.14.ts-web-kit-model-status-presentations</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:21</code> | <code>export const STATUS_PRESENTATIONS = Object.freeze(</code> |
| <code>interface.14.ts-web-kit-model-statuskey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:46</code> | <code>export type StatusKey = keyof typeof STATUS_PRESENTATIONS;</code> |
| <code>interface.14.ts-web-kit-model-statuskeyfromcopykey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:69</code> | <code>export function statusKeyFromCopyKey(copyKey: string): StatusKey</code> |
| <code>interface.14.ts-web-kit-model-statuspresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:61</code> | <code>export function statusPresentation(status: StatusKey): Readonly&lt;</code> |
| <code>interface.14.ts-web-kit-model-statustone</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:47</code> | <code>export type StatusTone = (typeof STATUS_PRESENTATIONS)[StatusKey]["tone"];</code> |
| <code>interface.14.ts-web-kit-model-typedconfirmationready</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:89</code> | <code>export function typedConfirmationReady( kind: ConfirmationKind, typed?: Readonly&lt;{ expectedValue: string; value: string }&gt;, ): boolean</code> |
| <code>interface.14.ts-web-kit-model-verification-presentations</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/kit/model.ts:38</code> | <code>export const VERIFICATION_PRESENTATIONS = Object.freeze(</code> |
| <code>interface.14.ts-web-kit-model-verificationkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/model.ts:48</code> | <code>export type VerificationKey = keyof typeof VERIFICATION_PRESENTATIONS;</code> |
| <code>interface.14.ts-web-kit-model-verificationrung-j53</code> | <code>ts_export</code> | <code>new</code> | <code>apps/web/src/kit/model.ts</code> (planned) | <code>export function verificationRung(level: string): VerificationRung</code> |
| <code>interface.14.ts-web-kit-model-verificationword</code> | <code>ts_export</code> | <code>change</code> | <code>apps/web/src/kit/model.ts:77</code> | <code>export function verificationWord(verification: VerificationKey): string</code> |
| <code>interface.14.ts-web-kit-money-copyableidentifier</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/money.tsx:97</code> | <code>export function CopyableIdentifier({ label, value, onCopied, className, }: Readonly&lt;{ label: string; value: string; onCopied?: (value: string) =&gt; void; className?: string }&gt;)</code> |
| <code>interface.14.ts-web-kit-money-feemathdisclosure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/money.tsx:164</code> | <code>export function FeeMathDisclosure({ open, onOpenChange, steps, title = copyEntry("fee.disclosure.title").message, totalLabel = copyEntry("fee.disclosure.total").message, currency, locale, symbol, decimals, platform, portalContainer,</code> |
| <code>interface.14.ts-web-kit-money-feemathdisclosureprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/money.tsx:150</code> | <code>export interface FeeMathDisclosureProps</code> |
| <code>interface.14.ts-web-kit-money-feemathstep</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/money.tsx:144</code> | <code>export interface FeeMathStep</code> |
| <code>interface.14.ts-web-kit-money-labelvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/money.tsx:69</code> | <code>export function LabelValue({ label, value, className, }: Readonly&lt;{ label: ReactNode; value: ReactNode; className?: string }&gt;)</code> |
| <code>interface.14.ts-web-kit-money-signedwordedamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/money.tsx:35</code> | <code>export function SignedWordedAmount({ value, currency, locale, decimals, direction, className, ...props }: SignedWordedAmountProps)</code> |
| <code>interface.14.ts-web-kit-money-signedwordedamountprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/money.tsx:24</code> | <code>export type SignedWordedAmountProps = Omit&lt;</code> |
| <code>interface.14.ts-web-kit-money-statuspill</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/money.tsx:82</code> | <code>export function StatusPill({ status, className }: Readonly&lt;{ status: StatusKey; className?: string }&gt;)</code> |
| <code>interface.14.ts-web-kit-pattern-code-entry-codeentryprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-code-entry.tsx:6</code> | <code>export type CodeEntryProps = Omit&lt;ComponentProps&lt;typeof CodeEntry&gt;, "platform"&gt;;</code> |
| <code>interface.14.ts-web-kit-pattern-code-entry-desktopcodeentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-code-entry.tsx:12</code> | <code>export function DesktopCodeEntry(props: CodeEntryProps)</code> |
| <code>interface.14.ts-web-kit-pattern-code-entry-mobilecodeentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-code-entry.tsx:8</code> | <code>export function MobileCodeEntry(props: CodeEntryProps)</code> |
| <code>interface.14.ts-web-kit-pattern-detail-desktopdetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-detail.tsx:12</code> | <code>export function DesktopDetail(props: DetailProps)</code> |
| <code>interface.14.ts-web-kit-pattern-detail-detailprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-detail.tsx:6</code> | <code>export type DetailProps = Omit&lt;ComponentProps&lt;typeof DetailDisclosure&gt;, "platform"&gt;;</code> |
| <code>interface.14.ts-web-kit-pattern-detail-mobiledetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-detail.tsx:8</code> | <code>export function MobileDetail(props: DetailProps)</code> |
| <code>interface.14.ts-web-kit-pattern-filters-desktopfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-filters.tsx:12</code> | <code>export function DesktopFilters(props: FiltersProps)</code> |
| <code>interface.14.ts-web-kit-pattern-filters-filtersprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-filters.tsx:6</code> | <code>export type FiltersProps = Omit&lt;ComponentProps&lt;typeof FilterBar&gt;, "platform"&gt;;</code> |
| <code>interface.14.ts-web-kit-pattern-filters-mobilefilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-filters.tsx:8</code> | <code>export function MobileFilters(props: FiltersProps)</code> |
| <code>interface.14.ts-web-kit-pattern-money-list-desktopmoneylist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-money-list.tsx:12</code> | <code>export function DesktopMoneyList(props: MoneyListProps)</code> |
| <code>interface.14.ts-web-kit-pattern-money-list-mobilemoneylist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-money-list.tsx:8</code> | <code>export function MobileMoneyList(props: MoneyListProps)</code> |
| <code>interface.14.ts-web-kit-pattern-money-list-moneylistprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-money-list.tsx:6</code> | <code>export type MoneyListProps = Omit&lt;ComponentProps&lt;typeof MoneyList&gt;, "platform"&gt;;</code> |
| <code>interface.14.ts-web-kit-pattern-navigation-desktopnavigation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-navigation.tsx:16</code> | <code>export function DesktopNavigation(props: NavigationProps)</code> |
| <code>interface.14.ts-web-kit-pattern-navigation-mobilenavigation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-navigation.tsx:12</code> | <code>export function MobileNavigation(props: NavigationProps)</code> |
| <code>interface.14.ts-web-kit-pattern-navigation-navigationprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-navigation.tsx:6</code> | <code>export type NavigationProps = Omit&lt;ComponentProps&lt;typeof AppShell&gt;, "platform"&gt;;</code> |
| <code>interface.14.ts-web-kit-pattern-notifications-desktopnotifications</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-notifications.tsx:20</code> | <code>export function DesktopNotifications({ view, ...props }: DesktopNotificationsProps)</code> |
| <code>interface.14.ts-web-kit-pattern-notifications-desktopnotificationsprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-notifications.tsx:12</code> | <code>export type DesktopNotificationsProps =</code> |
| <code>interface.14.ts-web-kit-pattern-notifications-kitnotificationitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-notifications.tsx:10</code> | <code>export type KitNotificationItem = MobileNotificationsPrimitiveProps["items"][number];</code> |
| <code>interface.14.ts-web-kit-pattern-notifications-mobilenotifications</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-notifications.tsx:16</code> | <code>export function MobileNotifications(props: MobileNotificationsProps)</code> |
| <code>interface.14.ts-web-kit-pattern-notifications-mobilenotificationsprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-notifications.tsx:11</code> | <code>export type MobileNotificationsProps = MobileNotificationsPrimitiveProps;</code> |
| <code>interface.14.ts-web-kit-pattern-primary-action-desktopprimaryaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-primary-action.tsx:41</code> | <code>export function DesktopPrimaryAction(props: KitPrimaryActionProps)</code> |
| <code>interface.14.ts-web-kit-pattern-primary-action-kitprimaryactionprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-primary-action.tsx:13</code> | <code>export type KitPrimaryActionProps = PrimaryActionBaseProps &amp; ControlAvailability;</code> |
| <code>interface.14.ts-web-kit-pattern-primary-action-mobileprimaryaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-primary-action.tsx:37</code> | <code>export function MobilePrimaryAction(props: KitPrimaryActionProps)</code> |
| <code>interface.14.ts-web-kit-pattern-search-desktopsearch</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-search.tsx:12</code> | <code>export function DesktopSearch(props: SearchProps)</code> |
| <code>interface.14.ts-web-kit-pattern-search-mobilesearch</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-search.tsx:8</code> | <code>export function MobileSearch(props: SearchProps)</code> |
| <code>interface.14.ts-web-kit-pattern-search-searchprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-search.tsx:6</code> | <code>export type SearchProps = Omit&lt;ComponentProps&lt;typeof GlobalSearch&gt;, "platform"&gt;;</code> |
| <code>interface.14.ts-web-kit-pattern-wizard-desktopwizard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-wizard.tsx:12</code> | <code>export function DesktopWizard(props: WizardProps)</code> |
| <code>interface.14.ts-web-kit-pattern-wizard-mobilewizard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-wizard.tsx:8</code> | <code>export function MobileWizard(props: WizardProps)</code> |
| <code>interface.14.ts-web-kit-pattern-wizard-wizardprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/pattern-wizard.tsx:6</code> | <code>export type WizardProps = Omit&lt;ComponentProps&lt;typeof Wizard&gt;, "platform"&gt;;</code> |
| <code>interface.14.ts-web-kit-performance-performanceloadingcard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/performance.tsx:5</code> | <code>export function PerformanceLoadingCard({ plane, label, }: Readonly&lt;{ plane: "app" &#124; "explorer"; label: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-plane-route-action-planerouteaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/plane-route-action.tsx:10</code> | <code>export function PlaneRouteAction({ destination, children, }: Readonly&lt;{ destination: "/app" &#124; "/explorer"; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-qr-code-addressqrcode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/qr-code.tsx:6</code> | <code>export function AddressQrCode({ value, label }: Readonly&lt;{ value: string; label: string }&gt;)</code> |
| <code>interface.14.ts-web-kit-settings-settingsrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/settings.tsx:24</code> | <code>export function SettingsRow(props: SettingsRowProps)</code> |
| <code>interface.14.ts-web-kit-settings-settingsrowprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/settings.tsx:22</code> | <code>export type SettingsRowProps = ListItemProps;</code> |
| <code>interface.14.ts-web-kit-settings-settingssection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/settings.tsx:10</code> | <code>export function SettingsSection({ title, children, }: Readonly&lt;{ title: ReactNode; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-settings-settingssegmentedcontrol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/settings.tsx:39</code> | <code>export function SettingsSegmentedControl(props: SegmentedControlProps)</code> |
| <code>interface.14.ts-web-kit-settings-settingsswitch</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/settings.tsx:28</code> | <code>export function SettingsSwitch( props: ComponentProps&lt;typeof Switch&gt; &amp; Readonly&lt;{ label: string }&gt;, )</code> |
| <code>interface.14.ts-web-kit-settings-settingstextinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/settings.tsx:35</code> | <code>export function SettingsTextInput(props: InputProps)</code> |
| <code>interface.14.ts-web-kit-surface-inlinenotice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/surface.tsx:95</code> | <code>export function InlineNotice({ children, tone = "neutral", role = "status", }: Readonly&lt;{ children: ReactNode; tone?: StateTone; role?: "alert" &#124; "status" }&gt;)</code> |
| <code>interface.14.ts-web-kit-surface-screencard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/surface.tsx:7</code> | <code>export function ScreenCard({ title, description, children, landmark = "main", dataApplication, }: Readonly&lt;{ title: ReactNode; description?: ReactNode; children?: ReactNode; landmark?: "main" &#124; "section"; dataApplication?: string;</code> |
| <code>interface.14.ts-web-kit-surface-stateempty</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/surface.tsx:87</code> | <code>export function StateEmpty({ title, description, action, }: Readonly&lt;{ title: ReactNode; description?: ReactNode; action?: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-kit-surface-stateframe</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/surface.tsx:43</code> | <code>export function StateFrame({ title, description, children, tone = "neutral", busy = false, role, }: Readonly&lt;{ title: ReactNode; description?: ReactNode; children?: ReactNode; tone?: StateTone;</code> |
| <code>interface.14.ts-web-kit-surface-stateskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/surface.tsx:75</code> | <code>export function StateSkeleton({ rows = 3 }: Readonly&lt;{ rows?: number }&gt;)</code> |
| <code>interface.14.ts-web-kit-surface-statetone</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/kit/surface.tsx:34</code> | <code>export type StateTone = "neutral" &#124; "warning" &#124; "danger" &#124; "success";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-kit-a11y-contrastratio</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-a11y-explicitcurrencyamountoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-a11y-liveregionprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-a11y-semantic-contrast-pairs</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-a11y-semanticcontrastresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-a11y-semantictokenvalues</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-activity-activityfeedprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-activity-activityfeedrow</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-confirm-typedconfirmation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-control-kitbuttonprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-device-list-devicelistitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-actiongridicon</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-actiongriditem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-actiongridprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-balancesummaryprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-countbadgeprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-kiticontile</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-kiticontileprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-kitlistitemprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-kitlistprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-kitoptionlistprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-kitsectionheaderprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-kittextfieldprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-kitviewallchipprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-display-optionlistitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-field-textfieldprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-model-kit</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-model-status-presentations</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-model-statustone</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-model-verification-presentations</code> | notes | feature 8 scope (spec/layerx-merge/spec.kvx:32248) reports the three rungs submitted, receipted, committed; this surface still carries checkpoint-finalised or settlement-anchored levels |
| <code>interface.14.ts-web-kit-model-verificationkey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-model-verificationrung-j53</code> | notes | maps every verification level string to unverified, receipt-verified or checkpoint-finalised; no caller until task 14.9.1 |
| <code>interface.14.ts-web-kit-model-verificationword</code> | overridden_by | override.J53.4 |
| <code>interface.14.ts-web-kit-money-feemathdisclosure</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-money-feemathdisclosureprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-money-feemathstep</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-money-signedwordedamountprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-code-entry-codeentryprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-code-entry-desktopcodeentry</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-code-entry-mobilecodeentry</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-detail-detailprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-filters-filtersprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-money-list-desktopmoneylist</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-money-list-mobilemoneylist</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-money-list-moneylistprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-notifications-desktopnotificationsprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-notifications-mobilenotificationsprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-primary-action-kitprimaryactionprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-search-desktopsearch</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-search-mobilesearch</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-pattern-search-searchprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-settings-settingsrowprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-kit-surface-statetone</code> | notes | no in-repo importer at HEAD |

Contract context <code>contract_context.14.web-verification-rungs</code>:

| Field | Frozen rule |
|---|---|
| interface | verificationRung and verificationWord in apps/web/src/kit/model.ts |
| storage | none |
| errors | unknown or retired levels map to unverified |
| events | none |
| gas | none |
| activation | wave 0 |
| invariants | the kit never renders a rung above what the record states; exactly three rungs exist |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.5.1</code> | req | 14005.1 |
| <code>acceptance.14.5.1</code> | scenario | Given the kit model, when verification_rung_keeps_three_rungs passes each of the three rungs, then the same rung and its catalog word come back |
| <code>acceptance.14.5.1</code> | test | apps/web/e2e/verification-rungs.test.ts::verification_rung_keeps_three_rungs (planned) |
| <code>acceptance.14.5.1</code> | qualification | UNRUN |
| <code>req.14005</code> | ac_1 | verificationRung maps unverified, receipt-verified and checkpoint-finalised to themselves and verificationWord (apps/web/src/kit/model.ts:77) renders verification.unverified, verification.receipt_verified and verification.checkpoint_finalised copy for them. |
| <code>acceptance.14.5.2</code> | req | 14005.2 |
| <code>acceptance.14.5.2</code> | scenario | Given the retired levels paxeer-finalised and settlement-anchored and the string bogus, when verification_rung_fails_retired_levels_closed maps them, then each yields unverified |
| <code>acceptance.14.5.2</code> | test | apps/web/e2e/verification-rungs.test.ts::verification_rung_fails_retired_levels_closed (planned) |
| <code>acceptance.14.5.2</code> | qualification | UNRUN |
| <code>req.14005</code> | ac_2 | verificationRung maps paxeer-finalised, settlement-anchored and an unknown string to unverified, and VERIFICATION_PRESENTATIONS (apps/web/src/kit/model.ts:38) has exactly three keys. |
| <code>capability.14.9.1</code> | title | Kit verification rungs |
| <code>capability.14.9.1</code> | reqs | 14005.1<br>14005.2 |

## perf

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-perf-budgets-budgetedapplicationroute</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:53</code> | <code>export type BudgetedApplicationRoute = keyof typeof ROUTE_SCRIPT_BUDGETS;</code> |
| <code>interface.14.ts-web-perf-budgets-classifyperformanceroute</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:83</code> | <code>export function classifyPerformanceRoute(pathname: string): PerformanceRoute &#124; undefined</code> |
| <code>interface.14.ts-web-perf-budgets-isperformanceroute</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:79</code> | <code>export function isPerformanceRoute(value: string): value is PerformanceRoute</code> |
| <code>interface.14.ts-web-perf-budgets-iswebvitalname</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:75</code> | <code>export function isWebVitalName(value: string): value is WebVitalName</code> |
| <code>interface.14.ts-web-perf-budgets-normalizedbudget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:114</code> | <code>export function normalizedBudget(name: WebVitalName): number</code> |
| <code>interface.14.ts-web-perf-budgets-normalizewebvital</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:105</code> | <code>export function normalizeWebVital(name: WebVitalName, observed: number): number &#124; undefined</code> |
| <code>interface.14.ts-web-perf-budgets-percentile75</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:119</code> | <code>export function percentile75(values: readonly number[]): number</code> |
| <code>interface.14.ts-web-perf-budgets-perf</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:71</code> | <code>export function perf()</code> |
| <code>interface.14.ts-web-perf-budgets-performance-routes</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:4</code> | <code>export const PERFORMANCE_ROUTES = ["root", "explorer", "app", "bridge", "exchange", "launchpad"] as const;</code> |
| <code>interface.14.ts-web-perf-budgets-performance-sample-count</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:61</code> | <code>export const PERFORMANCE_SAMPLE_COUNT = 4;</code> |
| <code>interface.14.ts-web-perf-budgets-performanceroute</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:5</code> | <code>export type PerformanceRoute = (typeof PERFORMANCE_ROUTES)[number];</code> |
| <code>interface.14.ts-web-perf-budgets-route-script-budgets</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:18</code> | <code>export const ROUTE_SCRIPT_BUDGETS = Object.freeze(</code> |
| <code>interface.14.ts-web-perf-budgets-rum-series-capacity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:62</code> | <code>export const RUM_SERIES_CAPACITY = 256;</code> |
| <code>interface.14.ts-web-perf-budgets-three-g-profile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:55</code> | <code>export const THREE_G_PROFILE = Object.freeze(</code> |
| <code>interface.14.ts-web-perf-budgets-web-vital-budgets</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:7</code> | <code>export const WEB_VITAL_BUDGETS = Object.freeze(</code> |
| <code>interface.14.ts-web-perf-budgets-web-vital-names</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:1</code> | <code>export const WEB_VITAL_NAMES = ["LCP", "INP", "CLS"] as const;</code> |
| <code>interface.14.ts-web-perf-budgets-webvitalname</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/budgets.ts:2</code> | <code>export type WebVitalName = (typeof WEB_VITAL_NAMES)[number];</code> |
| <code>interface.14.ts-web-perf-rum-store-parseredactedwebvital</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/rum-store.ts:42</code> | <code>export function parseRedactedWebVital(value: unknown): RedactedWebVital &#124; undefined</code> |
| <code>interface.14.ts-web-perf-rum-store-recordwebvital</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/rum-store.ts:67</code> | <code>export function recordWebVital(observation: RedactedWebVital): void</code> |
| <code>interface.14.ts-web-perf-rum-store-redactedwebvital</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/rum-store.ts:13</code> | <code>export interface RedactedWebVital</code> |
| <code>interface.14.ts-web-perf-rum-store-rumaggregate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/rum-store.ts:20</code> | <code>export interface RumAggregate</code> |
| <code>interface.14.ts-web-perf-rum-store-rumsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/rum-store.ts:77</code> | <code>export function rumSnapshot(): readonly RumAggregate[]</code> |
| <code>interface.14.ts-web-perf-web-vitals-rum-webvitalsrum</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/perf/web-vitals-rum.tsx:16</code> | <code>export function WebVitalsRum()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-perf-budgets-budgetedapplicationroute</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-perf-budgets-perf</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-perf-budgets-performance-routes</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-perf-budgets-web-vital-names</code> | notes | no in-repo importer at HEAD |

## proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-proxy-proxy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/proxy.ts:11</code> | <code>export function proxy(request: NextRequest)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-proxy-proxy</code> | notes | Next.js framework hook |

## routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-web-app-api-performance-vitals-route-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/api/performance/vitals/route.ts:11</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-api-performance-vitals-route-runtime</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/api/performance/vitals/route.ts:12</code> | <code>export const runtime = "nodejs";</code> |
| <code>interface.14.cfg-web-app-api-support-reports-route-runtime</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/api/support/reports/route.ts:9</code> | <code>export const runtime = "nodejs";</code> |
| <code>interface.14.cfg-web-app-api-support-reports-traceid-route-runtime</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/api/support/reports/[traceId]/route.ts:9</code> | <code>export const runtime = "nodejs";</code> |
| <code>interface.14.cfg-web-app-app-agents-agentid-page-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/[agentId]/page.tsx:3</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-agents-new-page-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/new/page.tsx:3</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-agents-page-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/page.tsx:3</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-deposit-page-dynamic</code> | <code>config_key</code> | <code>retire</code> | <code>apps/web/src/app/app/deposit/page.tsx:4</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-journeys-journeyid-claim-page-dynamic</code> | <code>config_key</code> | <code>retire</code> | <code>apps/web/src/app/app/journeys/[journeyId]/claim/page.tsx:3</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-journeys-journeyid-page-dynamic</code> | <code>config_key</code> | <code>retire</code> | <code>apps/web/src/app/app/journeys/[journeyId]/page.tsx:3</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-layout-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/app/layout.tsx:12</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-settings-exit-page-dynamic</code> | <code>config_key</code> | <code>retire</code> | <code>apps/web/src/app/app/settings/exit/page.tsx:3</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-support-page-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/app/support/page.tsx:7</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-app-withdraw-page-dynamic</code> | <code>config_key</code> | <code>retire</code> | <code>apps/web/src/app/app/withdraw/page.tsx:4</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-bridge-page-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/bridge/page.tsx:30</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-exchange-page-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/exchange/page.tsx:26</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-app-explorer-layout-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/explorer/layout.tsx:3</code> | <code>export const dynamic = "force-static";</code> |
| <code>interface.14.cfg-web-app-explorer-layout-revalidate</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/explorer/layout.tsx:4</code> | <code>export const revalidate = 60;</code> |
| <code>interface.14.cfg-web-app-launchpad-page-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/app/launchpad/page.tsx:27</code> | <code>export const dynamic = "force-dynamic";</code> |
| <code>interface.14.cfg-web-proxy-config</code> | <code>config_key</code> | <code>stable</code> | <code>apps/web/src/proxy.ts:28</code> | <code>export const config = { matcher: ["/app/:path*"], };</code> |
| <code>interface.14.http-web-get</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/page.tsx:11</code> | <code>GET / (page)</code> |
| <code>interface.14.http-web-get-api-performance-vitals</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/api/performance/vitals/route.ts:114</code> | <code>GET /api/performance/vitals</code> |
| <code>interface.14.http-web-get-api-support-reports-traceid</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/api/support/reports/[traceId]/route.ts:24</code> | <code>GET /api/support/reports/{traceId}</code> |
| <code>interface.14.http-web-get-app</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/page.tsx:3</code> | <code>GET /app (page)</code> |
| <code>interface.14.http-web-get-app-activity</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/activity/page.tsx:3</code> | <code>GET /app/activity (page)</code> |
| <code>interface.14.http-web-get-app-activity-entryid</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/activity/[entryId]/page.tsx:3</code> | <code>GET /app/activity/{entryId} (page)</code> |
| <code>interface.14.http-web-get-app-agents</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/page.tsx:5</code> | <code>GET /app/agents (page)</code> |
| <code>interface.14.http-web-get-app-agents-agentid</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/[agentId]/page.tsx:5</code> | <code>GET /app/agents/{agentId} (page)</code> |
| <code>interface.14.http-web-get-app-agents-new</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/agents/new/page.tsx:5</code> | <code>GET /app/agents/new (page)</code> |
| <code>interface.14.http-web-get-app-approvals</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/approvals/page.tsx:3</code> | <code>GET /app/approvals (page)</code> |
| <code>interface.14.http-web-get-app-approvals-approvalid</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/approvals/[approvalId]/page.tsx:5</code> | <code>GET /app/approvals/{approvalId} (page)</code> |
| <code>interface.14.http-web-get-app-deposit</code> | <code>http_route</code> | <code>retire</code> | <code>apps/web/src/app/app/deposit/page.tsx:6</code> | <code>GET /app/deposit (page)</code> |
| <code>interface.14.http-web-get-app-journeys-journeyid</code> | <code>http_route</code> | <code>retire</code> | <code>apps/web/src/app/app/journeys/[journeyId]/page.tsx:5</code> | <code>GET /app/journeys/{journeyId} (page)</code> |
| <code>interface.14.http-web-get-app-journeys-journeyid-claim</code> | <code>http_route</code> | <code>retire</code> | <code>apps/web/src/app/app/journeys/[journeyId]/claim/page.tsx:5</code> | <code>GET /app/journeys/{journeyId}/claim (page)</code> |
| <code>interface.14.http-web-get-app-move</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/move/page.tsx:3</code> | <code>GET /app/move (page)</code> |
| <code>interface.14.http-web-get-app-move-withdraw</code> | <code>http_route</code> | <code>retire</code> | <code>apps/web/src/app/app/move/withdraw/page.tsx:3</code> | <code>GET /app/move/withdraw (page)</code> |
| <code>interface.14.http-web-get-app-notifications</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/notifications/page.tsx:3</code> | <code>GET /app/notifications (page)</code> |
| <code>interface.14.http-web-get-app-security-devices</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/security/devices/page.tsx:3</code> | <code>GET /app/security/devices (page)</code> |
| <code>interface.14.http-web-get-app-security-recovery</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/security/recovery/page.tsx:3</code> | <code>GET /app/security/recovery (page)</code> |
| <code>interface.14.http-web-get-app-settings</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/settings/page.tsx:3</code> | <code>GET /app/settings (page)</code> |
| <code>interface.14.http-web-get-app-settings-devices</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/settings/devices/page.tsx:3</code> | <code>GET /app/settings/devices (page)</code> |
| <code>interface.14.http-web-get-app-settings-exit</code> | <code>http_route</code> | <code>retire</code> | <code>apps/web/src/app/app/settings/exit/page.tsx:5</code> | <code>GET /app/settings/exit (page)</code> |
| <code>interface.14.http-web-get-app-settings-security</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/settings/security/page.tsx:3</code> | <code>GET /app/settings/security (page)</code> |
| <code>interface.14.http-web-get-app-settings-wallet</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/settings/wallet/page.tsx:3</code> | <code>GET /app/settings/wallet (page)</code> |
| <code>interface.14.http-web-get-app-support</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/app/support/page.tsx:9</code> | <code>GET /app/support (page)</code> |
| <code>interface.14.http-web-get-app-withdraw</code> | <code>http_route</code> | <code>retire</code> | <code>apps/web/src/app/app/withdraw/page.tsx:6</code> | <code>GET /app/withdraw (page)</code> |
| <code>interface.14.http-web-get-bridge</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/bridge/page.tsx:49</code> | <code>GET /bridge (page)</code> |
| <code>interface.14.http-web-get-exchange</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/exchange/page.tsx:49</code> | <code>GET /exchange (page)</code> |
| <code>interface.14.http-web-get-explorer</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/explorer/page.tsx:7</code> | <code>GET /explorer (page)</code> |
| <code>interface.14.http-web-get-explorer-accounts-accountid</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/explorer/accounts/[accountId]/page.tsx:192</code> | <code>GET /explorer/accounts/{accountId} (page)</code> |
| <code>interface.14.http-web-get-explorer-lookup</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/explorer/lookup/route.ts:21</code> | <code>GET /explorer/lookup</code> |
| <code>interface.14.http-web-get-explorer-programs-programid</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/explorer/programs/[programId]/page.tsx:10</code> | <code>GET /explorer/programs/{programId} (page)</code> |
| <code>interface.14.http-web-get-explorer-receipts-receiptid</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/explorer/receipts/[receiptId]/page.tsx:12</code> | <code>GET /explorer/receipts/{receiptId} (page)</code> |
| <code>interface.14.http-web-get-explorer-verify</code> | <code>http_route</code> | <code>retire</code> | <code>apps/web/src/app/explorer/verify/page.tsx:5</code> | <code>GET /explorer/verify (page)</code> |
| <code>interface.14.http-web-get-launchpad</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/launchpad/page.tsx:32</code> | <code>GET /launchpad (page)</code> |
| <code>interface.14.http-web-post-api-explorer-verify</code> | <code>http_route</code> | <code>retire</code> | <code>apps/web/src/app/api/explorer/verify/route.ts:55</code> | <code>POST /api/explorer/verify</code> |
| <code>interface.14.http-web-post-api-performance-vitals</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/api/performance/vitals/route.ts:81</code> | <code>POST /api/performance/vitals</code> |
| <code>interface.14.http-web-post-api-support-reports</code> | <code>http_route</code> | <code>stable</code> | <code>apps/web/src/app/api/support/reports/route.ts:47</code> | <code>POST /api/support/reports</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-web-app-api-performance-vitals-route-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-api-performance-vitals-route-runtime</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-api-support-reports-route-runtime</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-api-support-reports-traceid-route-runtime</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-app-agents-agentid-page-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-app-agents-new-page-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-app-agents-page-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-app-deposit-page-dynamic</code> | notes | stale req.8033 removes the deposit journey routes |
| <code>interface.14.cfg-web-app-app-journeys-journeyid-claim-page-dynamic</code> | notes | stale req.8033-8035 remove the custody journeys this route renders |
| <code>interface.14.cfg-web-app-app-journeys-journeyid-page-dynamic</code> | notes | stale req.8033-8035 remove the custody journeys this route renders |
| <code>interface.14.cfg-web-app-app-layout-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-app-settings-exit-page-dynamic</code> | notes | stale req.8035 removes the forced-exit journey |
| <code>interface.14.cfg-web-app-app-support-page-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-app-withdraw-page-dynamic</code> | notes | stale req.8034 removes the withdraw journey routes |
| <code>interface.14.cfg-web-app-bridge-page-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-exchange-page-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-explorer-layout-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-explorer-layout-revalidate</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-app-launchpad-page-dynamic</code> | notes | Next.js route segment config read by the framework |
| <code>interface.14.cfg-web-proxy-config</code> | notes | Next.js config export read by the framework |
| <code>interface.14.http-web-get</code> | notes | site root |
| <code>interface.14.http-web-get-app-deposit</code> | notes | stale req.8033 removes the deposit journey routes |
| <code>interface.14.http-web-get-app-journeys-journeyid</code> | notes | stale req.8033-8035 remove the custody journeys this route renders; no in-scope string reference to this path |
| <code>interface.14.http-web-get-app-journeys-journeyid-claim</code> | notes | stale req.8033-8035 remove the custody journeys this route renders; no in-scope string reference to this path |
| <code>interface.14.http-web-get-app-move-withdraw</code> | notes | stale req.8034 removes the withdraw journey routes |
| <code>interface.14.http-web-get-app-settings-exit</code> | notes | stale req.8035 removes the forced-exit journey |
| <code>interface.14.http-web-get-app-withdraw</code> | notes | stale req.8034 removes the withdraw journey routes |
| <code>interface.14.http-web-get-explorer-programs-programid</code> | notes | no in-scope string reference to this path |
| <code>interface.14.http-web-get-explorer-verify</code> | notes | stale req.8045 ac_3 removes mirror verify |
| <code>interface.14.http-web-post-api-explorer-verify</code> | notes | stale req.8045 ac_3 removes mirror verify |

## security

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-security-csp-appcontentsecuritypolicy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/security/csp.ts:3</code> | <code>export function appContentSecurityPolicy(nonce: string, development: boolean): string</code> |
| <code>interface.14.ts-web-security-origin-requesthasweborigin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/security/origin.ts:1</code> | <code>export function requestHasWebOrigin(request: Request, configuredOrigin: string &#124; undefined): boolean</code> |

## server

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-server-support-reports-supportreportconfigurationerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/server/support-reports.ts:17</code> | <code>export class SupportReportConfigurationError extends Error {}</code> |
| <code>interface.14.ts-web-server-support-reports-supportreportconflicterror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/server/support-reports.ts:18</code> | <code>export class SupportReportConflictError extends Error {}</code> |
| <code>interface.14.ts-web-server-support-reports-supportreportrepository</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/server/support-reports.ts:75</code> | <code>export class SupportReportRepository</code> |
| <code>interface.14.ts-web-server-support-reports-supportreportrepositoryfromenvironment</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/server/support-reports.ts:193</code> | <code>export function supportReportRepositoryFromEnvironment(): SupportReportRepository</code> |
| <code>interface.14.ts-web-server-support-reports-supportretrievalauthorized</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/server/support-reports.ts:197</code> | <code>export function supportRetrievalAuthorized(request: Request): boolean</code> |

## settings

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-settings-model-classenabled</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:35</code> | <code>export function classEnabled( preferences: NotificationPreferences, channel: NotificationChannel, notificationClass: NotificationClass, ): boolean</code> |
| <code>interface.14.ts-web-settings-model-clonenotificationpreferences</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:24</code> | <code>export function cloneNotificationPreferences( preferences: NotificationPreferences, ): NotificationPreferences</code> |
| <code>interface.14.ts-web-settings-model-fullysuppressessecurityclass</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:45</code> | <code>export function fullySuppressesSecurityClass(preferences: NotificationPreferences): boolean</code> |
| <code>interface.14.ts-web-settings-model-human-web-settings</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:103</code> | <code>export function human_web_settings()</code> |
| <code>interface.14.ts-web-settings-model-non-suppressible-notification-classes</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:12</code> | <code>export const NON_SUPPRESSIBLE_NOTIFICATION_CLASSES = new Set&lt;NotificationClass&gt;([</code> |
| <code>interface.14.ts-web-settings-model-normalizednotificationclasses</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:84</code> | <code>export function normalizedNotificationClasses( channel: ChannelPreference, ): readonly Readonly&lt;</code> |
| <code>interface.14.ts-web-settings-model-notification-channels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:9</code> | <code>export const NOTIFICATION_CHANNELS = ["push", "email", "in_app"] as const;</code> |
| <code>interface.14.ts-web-settings-model-notificationchannel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:10</code> | <code>export type NotificationChannel = (typeof NOTIFICATION_CHANNELS)[number];</code> |
| <code>interface.14.ts-web-settings-model-settings</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:93</code> | <code>export function settings(): Readonly&lt;</code> |
| <code>interface.14.ts-web-settings-model-withchannelenabled</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:51</code> | <code>export function withChannelEnabled( preferences: NotificationPreferences, channel: NotificationChannel, enabled: boolean, ): NotificationPreferences &#124; undefined</code> |
| <code>interface.14.ts-web-settings-model-withclassenabled</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:61</code> | <code>export function withClassEnabled( preferences: NotificationPreferences, channel: NotificationChannel, notificationClass: NotificationClass, enabled: boolean, ): NotificationPreferences &#124; undefined</code> |
| <code>interface.14.ts-web-settings-model-withdetaillevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/model.ts:77</code> | <code>export function withDetailLevel( preferences: NotificationPreferences, detail: NotificationDetailLevel, ): NotificationPreferences</code> |
| <code>interface.14.ts-web-settings-privacy-privacymodeprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/privacy.tsx:34</code> | <code>export function PrivacyModeProvider({ principalScope, children, }: Readonly&lt;{ principalScope: string; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-settings-privacy-privatefigure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/privacy.tsx:108</code> | <code>export function PrivateFigure({ children, className, }: Readonly&lt;{ children: ReactNode; className?: string }&gt;)</code> |
| <code>interface.14.ts-web-settings-privacy-useprivacymode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/privacy.tsx:100</code> | <code>export function usePrivacyMode(): PrivacyModeContextValue</code> |
| <code>interface.14.ts-web-settings-security-model-formatlastactive</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/model.ts:35</code> | <code>export function formatLastActive(timestamp: string): string</code> |
| <code>interface.14.ts-web-settings-security-model-human-web-security</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/model.ts:78</code> | <code>export function human_web_security()</code> |
| <code>interface.14.ts-web-settings-security-model-recoverypresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/model.ts:17</code> | <code>export interface RecoveryPresentation</code> |
| <code>interface.14.ts-web-settings-security-model-recoverypresentation-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/model.ts:22</code> | <code>export function recoveryPresentation(journey: Journey): RecoveryPresentation</code> |
| <code>interface.14.ts-web-settings-security-model-secretexpiry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/model.ts:50</code> | <code>export function secretExpiry(secret: TimedSecret): number</code> |
| <code>interface.14.ts-web-settings-security-model-security</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/model.ts:71</code> | <code>export const security = Object.freeze(</code> |
| <code>interface.14.ts-web-settings-security-model-securitystepup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/model.ts:58</code> | <code>export async function securityStepUp( client: HumanApiClient, action: SecurityActionKind, targetId?: string, authenticator: PasskeyAuthenticator = browserPasskeyAuthenticator(), ): Promise&lt;StepUpEvidence&gt;</code> |
| <code>interface.14.ts-web-settings-security-secret-backupcodesview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/secret.tsx:81</code> | <code>export function BackupCodesView({ codes, remaskAt, copyable, onExpired }: BackupCodesViewProps)</code> |
| <code>interface.14.ts-web-settings-security-secret-timedsecretview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/secret.tsx:17</code> | <code>export function TimedSecretView({ label, secret, onExpired }: TimedSecretViewProps)</code> |
| <code>interface.14.ts-web-settings-security-security-screen-securityscreen</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/security/security-screen.tsx:69</code> | <code>export function SecurityScreen({ client: suppliedClient, authenticator: suppliedAuthenticator }: SecurityScreenProps)</code> |
| <code>interface.14.ts-web-settings-server-privacyprincipalscope</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/server.ts:1</code> | <code>export function privacyPrincipalScope(verifiedScope: string): string</code> |
| <code>interface.14.ts-web-settings-settings-screen-loadsettingssnapshot-j53</code> | <code>ts_export</code> | <code>new</code> | <code>apps/web/src/settings/settings-screen.tsx</code> (planned) | <code>export async function loadSettingsSnapshot(client: HumanApiClient): Promise&lt;SettingsSnapshot&gt;</code> |
| <code>interface.14.ts-web-settings-settings-screen-settingsscreen</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/settings-screen.tsx:165</code> | <code>export function SettingsScreen({ client: suppliedClient, }: Readonly&lt;{ client?: HumanApiClient }&gt;)</code> |
| <code>interface.14.ts-web-settings-wallet-bridge-bindingwalletbridge</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/bridge.ts:14</code> | <code>export interface BindingWalletBridge</code> |
| <code>interface.14.ts-web-settings-wallet-bridge-browserbindingwalletbridge</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/bridge.ts:47</code> | <code>export function browserBindingWalletBridge( provider: () =&gt; Eip1193Provider &#124; undefined, ): BindingWalletBridge</code> |
| <code>interface.14.ts-web-settings-wallet-bridge-walletaccountoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/wallet/bridge.ts:6</code> | <code>export type WalletAccountOutcome =</code> |
| <code>interface.14.ts-web-settings-wallet-bridge-walletbridgefailure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/wallet/bridge.ts:4</code> | <code>export type WalletBridgeFailure = "cancelled" &#124; "rejected" &#124; "unavailable" &#124; "failed";</code> |
| <code>interface.14.ts-web-settings-wallet-bridge-walletsignatureoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/wallet/bridge.ts:10</code> | <code>export type WalletSignatureOutcome =</code> |
| <code>interface.14.ts-web-settings-wallet-model-activewalletbinding</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:25</code> | <code>export interface ActiveWalletBinding</code> |
| <code>interface.14.ts-web-settings-wallet-model-human-web-wallet-binding</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:216</code> | <code>export function human_web_wallet_binding()</code> |
| <code>interface.14.ts-web-settings-wallet-model-newestwalletsecuritynotification</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/wallet/model.ts:81</code> | <code>export function newestWalletSecurityNotification( notifications: readonly NotificationSummary[], ): NotificationSummary &#124; undefined</code> |
| <code>interface.14.ts-web-settings-wallet-model-receiptbackedbinding</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:49</code> | <code>export function receiptBackedBinding(binding: WalletBinding): ActiveWalletBinding &#124; undefined</code> |
| <code>interface.14.ts-web-settings-wallet-model-validbindingstatement</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:67</code> | <code>export function validBindingStatement( statement: BindingStatement, selectedAddress: string, now = Date.now(), ): boolean</code> |
| <code>interface.14.ts-web-settings-wallet-model-wallet-binding</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:209</code> | <code>export const wallet_binding = Object.freeze(</code> |
| <code>interface.14.ts-web-settings-wallet-model-wallet-binding-domain</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:22</code> | <code>export const WALLET_BINDING_DOMAIN = "layerx-wallet-binding-v1\n";</code> |
| <code>interface.14.ts-web-settings-wallet-model-walletbindingcontroller</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:92</code> | <code>export class WalletBindingController</code> |
| <code>interface.14.ts-web-settings-wallet-model-walletbindingphase</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:31</code> | <code>export type WalletBindingPhase =</code> |
| <code>interface.14.ts-web-settings-wallet-model-walletbindingsnapshot</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/model.ts:41</code> | <code>export interface WalletBindingSnapshot</code> |
| <code>interface.14.ts-web-settings-wallet-model-walletfeecontroller</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/wallet/model.ts:227</code> | <code>export class WalletFeeController</code> |
| <code>interface.14.ts-web-settings-wallet-model-walletfeesnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/wallet/model.ts:221</code> | <code>export interface WalletFeeSnapshot</code> |
| <code>interface.14.ts-web-settings-wallet-wallet-screen-walletbindingscreen</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/web/src/settings/wallet/wallet-screen.tsx:60</code> | <code>export function WalletBindingScreen({ client: suppliedClient, }: Readonly&lt;{ client?: HumanApiClient }&gt;)</code> |
| <code>interface.14.ts-web-settings-wallet-wallet-screen-walletfeechoice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/wallet/wallet-screen.tsx:264</code> | <code>export function WalletFeeChoice({ snapshot, onChange }: Readonly&lt;{ snapshot: WalletFeeSnapshot; onChange: (currency: WalletFeeSnapshot["currency"], maximum: string) =&gt; void; }&gt;)</code> |
| <code>interface.14.ts-web-settings-wallet-wallet-screen-walletfeesettings</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/settings/wallet/wallet-screen.tsx:297</code> | <code>export function WalletFeeSettings()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-settings-model-human-web-settings</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-model-settings</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-security-model-human-web-security</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-security-model-recoverypresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-security-model-security</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-settings-screen-loadsettingssnapshot-j53</code> | notes | extracted from the parallel load at apps/web/src/settings/settings-screen.tsx:185-191 without the retired bindingStatus read; SettingsScreen is its only caller |
| <code>interface.14.ts-web-settings-wallet-bridge-bindingwalletbridge</code> | overridden_by | override.J53.27 |
| <code>interface.14.ts-web-settings-wallet-bridge-browserbindingwalletbridge</code> | overridden_by | override.J53.28 |
| <code>interface.14.ts-web-settings-wallet-bridge-walletaccountoutcome</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-bridge-walletsignatureoutcome</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-model-activewalletbinding</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-model-activewalletbinding</code> | overridden_by | override.J53.19 |
| <code>interface.14.ts-web-settings-wallet-model-human-web-wallet-binding</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-model-human-web-wallet-binding</code> | overridden_by | override.J53.26 |
| <code>interface.14.ts-web-settings-wallet-model-receiptbackedbinding</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-model-receiptbackedbinding</code> | overridden_by | override.J53.22 |
| <code>interface.14.ts-web-settings-wallet-model-validbindingstatement</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-model-validbindingstatement</code> | overridden_by | override.J53.23 |
| <code>interface.14.ts-web-settings-wallet-model-wallet-binding</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-model-wallet-binding</code> | overridden_by | override.J53.25 |
| <code>interface.14.ts-web-settings-wallet-model-wallet-binding-domain</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-model-wallet-binding-domain</code> | overridden_by | override.J53.18 |
| <code>interface.14.ts-web-settings-wallet-model-walletbindingcontroller</code> | overridden_by | override.J53.24 |
| <code>interface.14.ts-web-settings-wallet-model-walletbindingphase</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-settings-wallet-model-walletbindingphase</code> | overridden_by | override.J53.20 |
| <code>interface.14.ts-web-settings-wallet-model-walletbindingsnapshot</code> | overridden_by | override.J53.21 |
| <code>interface.14.ts-web-settings-wallet-wallet-screen-walletbindingscreen</code> | overridden_by | override.J53.29 |
| <code>interface.14.ts-web-settings-wallet-wallet-screen-walletfeesettings</code> | notes | no in-repo importer at HEAD |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.52.1</code> | req | 14052.1 |
| <code>acceptance.14.52.1</code> | scenario | Given a stub client recording calls, when settings_snapshot_loads_without_binding_status awaits loadSettingsSnapshot, then the recorded calls are the four reads and bindingStatus is absent |
| <code>acceptance.14.52.1</code> | test | apps/web/e2e/settings-without-binding.test.ts::settings_snapshot_loads_without_binding_status (planned) |
| <code>acceptance.14.52.1</code> | qualification | UNRUN |
| <code>req.14052</code> | ac_1 | loadSettingsSnapshot (planned) calls profileGet, notificationPreferencesGet, securityPasskeyList and sessionList on the client and never bindingStatus. |
| <code>acceptance.14.52.2</code> | req | 14052.2 |
| <code>acceptance.14.52.2</code> | scenario | Given a stub client whose bindingStatus rejects with status 410, when settings_snapshot_survives_operation_retired awaits loadSettingsSnapshot, then it resolves with the profile and session data |
| <code>acceptance.14.52.2</code> | test | apps/web/e2e/settings-without-binding.test.ts::settings_snapshot_survives_operation_retired (planned) |
| <code>acceptance.14.52.2</code> | qualification | UNRUN |
| <code>req.14052</code> | ac_2 | loadSettingsSnapshot (planned) resolves when bindingStatus would reject with an HTTP 410 operation-retired HumanApiError (apps/web/src/api/generated/index.ts:5373 at HEAD). |
| <code>acceptance.14.53.1</code> | req | 14053.1 |
| <code>acceptance.14.53.1</code> | scenario | Given the edited page, when wallet_settings_render_fee_choice_only renders it, then the markup contains the fee choice copy and no settings.wallet.bind copy |
| <code>acceptance.14.53.1</code> | test | apps/web/e2e/wallet-settings.test.ts::wallet_settings_render_fee_choice_only (planned) |
| <code>acceptance.14.53.1</code> | qualification | UNRUN |
| <code>req.14053</code> | ac_1 | The /app/settings/wallet page (apps/web/src/app/app/settings/wallet/page.tsx:3) renders WalletFeeSettings with the PAX and SID choices and no binding call to action. |
| <code>acceptance.14.53.2</code> | req | 14053.2 |
| <code>acceptance.14.53.2</code> | scenario | Given the edited module, when wallet_settings_module_exports_no_binding imports it, then the three binding names are undefined and WalletFeeController is a function |
| <code>acceptance.14.53.2</code> | test | apps/web/e2e/wallet-settings.test.ts::wallet_settings_module_exports_no_binding (planned) |
| <code>acceptance.14.53.2</code> | qualification | UNRUN |
| <code>req.14053</code> | ac_2 | apps/web/src/settings/wallet/index.ts:1 exports WalletFeeController and no WalletBindingController, browserBindingWalletBridge or WalletBindingScreen. |
| <code>capability.14.8.1</code> | title | Settings screen without binding reads |
| <code>capability.14.8.1</code> | reqs | 14052.1<br>14052.2 |
| <code>capability.14.8.2</code> | title | Wallet binding removed from settings |
| <code>capability.14.8.2</code> | reqs | 14053.1<br>14053.2 |

## shell

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-shell-app-shell-authenticatedshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/app-shell.tsx:90</code> | <code>export function AuthenticatedShell({ initialSelection, children, }: Readonly&lt;{ initialSelection: ShellSelection; children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-shell-app-shell-useauthenticatedshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/app-shell.tsx:220</code> | <code>export function useAuthenticatedShell(): ShellSelection</code> |
| <code>interface.14.ts-web-shell-app-shell-useshellselection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/app-shell.tsx:44</code> | <code>export function useShellSelection(): ShellSelection</code> |
| <code>interface.14.ts-web-shell-selector-deeplinkresolution</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:25</code> | <code>export interface DeepLinkResolution</code> |
| <code>interface.14.ts-web-shell-selector-pointercapability</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:5</code> | <code>export type PointerCapability = "coarse" &#124; "fine" &#124; "none";</code> |
| <code>interface.14.ts-web-shell-selector-shell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:4</code> | <code>export type Shell = "mobile" &#124; "desktop";</code> |
| <code>interface.14.ts-web-shell-selector-shell-hint-cookie</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:2</code> | <code>export const SHELL_HINT_COOKIE = "__Host-layerx-shell";</code> |
| <code>interface.14.ts-web-shell-selector-shellconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:20</code> | <code>export interface ShellConfirmation</code> |
| <code>interface.14.ts-web-shell-selector-shellhints</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:7</code> | <code>export interface ShellHints</code> |
| <code>interface.14.ts-web-shell-selector-shellselection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:12</code> | <code>export interface ShellSelection</code> |
| <code>interface.14.ts-web-shell-selector-shellselector</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:69</code> | <code>export class ShellSelector</code> |
| <code>interface.14.ts-web-shell-selector-tablet-min-width</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/selector.ts:1</code> | <code>export const TABLET_MIN_WIDTH = 768;</code> |
| <code>interface.14.ts-web-shell-server-selectservershell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/shell/server.ts:30</code> | <code>export function selectServerShell(headers: Headers, cookies: RequestCookies): ShellSelection</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-shell-selector-deeplinkresolution</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-shell-selector-shellconfirmation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-shell-selector-shellhints</code> | notes | no in-repo importer at HEAD |

## states

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-states-application-applicationstateboundary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/application.tsx:10</code> | <code>export function ApplicationStateBoundary({ children }: Readonly&lt;{ children: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-states-error-errorboundary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/error.tsx:251</code> | <code>export class ErrorBoundary extends Component&lt;ErrorBoundaryProps, ErrorBoundaryState&gt;</code> |
| <code>interface.14.ts-web-states-error-errorpresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/error.tsx:24</code> | <code>export interface ErrorPresentation</code> |
| <code>interface.14.ts-web-states-error-errorpresentation-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/error.tsx:53</code> | <code>export function errorPresentation(error: unknown, traceId = generatedTraceId()): ErrorPresentation</code> |
| <code>interface.14.ts-web-states-error-errorsurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/error.tsx:111</code> | <code>export function ErrorSurface({ error, route, platform, onRetry, onReload, reporter = browserSupportReportOutbox, }: Readonly&lt;{ error: ErrorPresentation; route: string; platform?: "mobile" &#124; "desktop"; onRetry?: () =&gt; void;</code> |
| <code>interface.14.ts-web-states-error-routeerrorboundary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/error.tsx:290</code> | <code>export function RouteErrorBoundary({ error, reset, route, }: Readonly&lt;{ error: Error &amp; { digest?: string }; reset: () =&gt; void; route: string }&gt;)</code> |
| <code>interface.14.ts-web-states-matrix-current-routes</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:12</code> | <code>export const CURRENT_ROUTES = [</code> |
| <code>interface.14.ts-web-states-matrix-current-shells</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:40</code> | <code>export const CURRENT_SHELLS = ["authenticated"] as const;</code> |
| <code>interface.14.ts-web-states-matrix-human-state-matrix-registry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:196</code> | <code>export function human_state_matrix_registry(): StateMatrix</code> |
| <code>interface.14.ts-web-states-matrix-humanshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:43</code> | <code>export type HumanShell = "mobile" &#124; "desktop";</code> |
| <code>interface.14.ts-web-states-matrix-humanstatematrix</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:192</code> | <code>export function humanStateMatrix(): StateMatrix</code> |
| <code>interface.14.ts-web-states-matrix-required-screen-states</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:3</code> | <code>export const REQUIRED_SCREEN_STATES = [</code> |
| <code>interface.14.ts-web-states-matrix-screenstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:42</code> | <code>export type ScreenState = (typeof REQUIRED_SCREEN_STATES)[number];</code> |
| <code>interface.14.ts-web-states-matrix-screenstateentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:53</code> | <code>export interface ScreenStateEntry</code> |
| <code>interface.14.ts-web-states-matrix-statematrix</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:128</code> | <code>export class StateMatrix</code> |
| <code>interface.14.ts-web-states-matrix-statepresentation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:46</code> | <code>export interface StatePresentation</code> |
| <code>interface.14.ts-web-states-matrix-statetargetkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/matrix.ts:44</code> | <code>export type StateTargetKind = "route" &#124; "shell";</code> |
| <code>interface.14.ts-web-states-offline-offlinebanner</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/offline.tsx:8</code> | <code>export function OfflineBanner()</code> |
| <code>interface.14.ts-web-states-offline-queuedactionnotice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/offline.tsx:28</code> | <code>export function QueuedActionNotice({ children }: Readonly&lt;{ children?: ReactNode }&gt;)</code> |
| <code>interface.14.ts-web-states-queue-moneyofflineaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/queue.ts:7</code> | <code>export interface MoneyOfflineAction</code> |
| <code>interface.14.ts-web-states-queue-offlineactionqueue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/queue.ts:15</code> | <code>export class OfflineActionQueue</code> |
| <code>interface.14.ts-web-states-queue-queueableofflineaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/queue.ts:1</code> | <code>export interface QueueableOfflineAction</code> |
| <code>interface.14.ts-web-states-queue-queuedecision</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/queue.ts:13</code> | <code>export type QueueDecision = "queued" &#124; "duplicate" &#124; "money-rejected";</code> |
| <code>interface.14.ts-web-states-report-browsersupportreportoutbox</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report.ts:108</code> | <code>export class BrowserSupportReportOutbox implements ErrorReporter</code> |
| <code>interface.14.ts-web-states-report-browsersupportreportoutbox-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report.ts:177</code> | <code>export const browserSupportReportOutbox = new BrowserSupportReportOutbox();</code> |
| <code>interface.14.ts-web-states-report-errorreportdraft</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report.ts:10</code> | <code>export interface ErrorReportDraft</code> |
| <code>interface.14.ts-web-states-report-errorreporter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report.ts:23</code> | <code>export interface ErrorReporter</code> |
| <code>interface.14.ts-web-states-report-errorreportreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report.ts:18</code> | <code>export type ErrorReportReceipt = SavedReportReceipt &#124; Readonly&lt;</code> |
| <code>interface.14.ts-web-states-report-redacterrorreport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report.ts:39</code> | <code>export function redactErrorReport(draft: ErrorReportDraft): SupportReportRequest</code> |
| <code>interface.14.ts-web-states-report-schema-parsesavedreportreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report-schema.ts:115</code> | <code>export function parseSavedReportReceipt(value: unknown): SavedReportReceipt</code> |
| <code>interface.14.ts-web-states-report-schema-parsestoredsupportreport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report-schema.ts:95</code> | <code>export function parseStoredSupportReport(value: unknown): StoredSupportReport</code> |
| <code>interface.14.ts-web-states-report-schema-parsesupportreportrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report-schema.ts:85</code> | <code>export function parseSupportReportRequest(value: unknown): SupportReportRequest</code> |
| <code>interface.14.ts-web-states-report-schema-reportshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report-schema.ts:3</code> | <code>export type ReportShell = "mobile" &#124; "desktop";</code> |
| <code>interface.14.ts-web-states-report-schema-savedreportreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report-schema.ts:17</code> | <code>export interface SavedReportReceipt</code> |
| <code>interface.14.ts-web-states-report-schema-storedsupportreport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report-schema.ts:12</code> | <code>export interface StoredSupportReport extends SupportReportRequest</code> |
| <code>interface.14.ts-web-states-report-schema-supportreportrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report-schema.ts:5</code> | <code>export interface SupportReportRequest</code> |
| <code>interface.14.ts-web-states-report-schema-supportreporttrace</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/report-schema.ts:47</code> | <code>export function supportReportTrace(value: string): string</code> |
| <code>interface.14.ts-web-states-surfaces-degradedsurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/surfaces.tsx:51</code> | <code>export function DegradedSurface({ age, referencePoint, children, onReload, }: Readonly&lt;{ age: string; referencePoint: string; children?: ReactNode; onReload?: () =&gt; void; }&gt;)</code> |
| <code>interface.14.ts-web-states-surfaces-emptysurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/surfaces.tsx:23</code> | <code>export function EmptySurface({ action, actionLabelKey, }: Readonly&lt;{ action?: () =&gt; void; actionLabelKey?: string }&gt;)</code> |
| <code>interface.14.ts-web-states-surfaces-loadingsurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/surfaces.tsx:10</code> | <code>export function LoadingSurface({ rows = 3 }: Readonly&lt;{ rows?: number }&gt;)</code> |
| <code>interface.14.ts-web-states-surfaces-offlinesurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/surfaces.tsx:38</code> | <code>export function OfflineSurface({ onRetry }: Readonly&lt;{ onRetry: () =&gt; void }&gt;)</code> |
| <code>interface.14.ts-web-states-surfaces-stillcheckingsurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/surfaces.tsx:85</code> | <code>export function StillCheckingSurface({ lookupOutcome, onResolved, pollIntervalMs = 3_000, children, }: StillCheckingSurfaceProps)</code> |
| <code>interface.14.ts-web-states-surfaces-stillcheckingsurfaceprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/states/surfaces.tsx:78</code> | <code>export interface StillCheckingSurfaceProps</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-states-error-errorpresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-matrix-humanshell</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-matrix-screenstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-matrix-screenstateentry</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-matrix-statematrix</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-matrix-statepresentation</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-matrix-statetargetkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-offline-queuedactionnotice</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-queue-moneyofflineaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-queue-queueableofflineaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-queue-queuedecision</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-report-browsersupportreportoutbox</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-report-errorreportdraft</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-report-errorreportreceipt</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-surfaces-degradedsurface</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-surfaces-emptysurface</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-states-surfaces-stillcheckingsurfaceprops</code> | notes | no in-repo importer at HEAD |

## support

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-web-support-chat-desktopsupportchat</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/chat.tsx:311</code> | <code>export function DesktopSupportChat({ traceId }: Readonly&lt;{ traceId?: string }&gt;)</code> |
| <code>interface.14.ts-web-support-chat-mobilesupportchat</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/chat.tsx:307</code> | <code>export function MobileSupportChat({ traceId }: Readonly&lt;{ traceId?: string }&gt;)</code> |
| <code>interface.14.ts-web-support-chat-supportchat</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/chat.tsx:153</code> | <code>export function SupportChat({ platform, traceId, }: Readonly&lt;{ platform: "mobile" &#124; "desktop"; traceId?: TraceId }&gt;)</code> |
| <code>interface.14.ts-web-support-conversation-supportchatconnection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/conversation.ts:15</code> | <code>export type SupportChatConnection = "online" &#124; "offline";</code> |
| <code>interface.14.ts-web-support-conversation-supportchatdelivery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/conversation.ts:14</code> | <code>export type SupportChatDelivery = "sending" &#124; "sent" &#124; "failed";</code> |
| <code>interface.14.ts-web-support-conversation-supportchatentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/conversation.ts:19</code> | <code>export interface SupportChatEntry extends SupportMessage</code> |
| <code>interface.14.ts-web-support-conversation-supportchatload</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/conversation.ts:16</code> | <code>export type SupportChatLoad = "loading" &#124; "ready" &#124; "failed";</code> |
| <code>interface.14.ts-web-support-conversation-supportchatsession</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/conversation.ts:62</code> | <code>export class SupportChatSession</code> |
| <code>interface.14.ts-web-support-conversation-supportchatsessionoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/conversation.ts:46</code> | <code>export interface SupportChatSessionOptions</code> |
| <code>interface.14.ts-web-support-conversation-supportchatsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/conversation.ts:23</code> | <code>export interface SupportChatSnapshot</code> |
| <code>interface.14.ts-web-support-conversation-supportfeedbackdelivery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/conversation.ts:17</code> | <code>export type SupportFeedbackDelivery = "idle" &#124; "sending" &#124; "saved" &#124; "failed";</code> |
| <code>interface.14.ts-web-support-model-human-web-support</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/model.ts:10</code> | <code>export function human_web_support()</code> |
| <code>interface.14.ts-web-support-model-support</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/model.ts:1</code> | <code>export function support(): Readonly&lt;</code> |
| <code>interface.14.ts-web-support-topics-support-topics</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/topics.ts:9</code> | <code>export const SUPPORT_TOPICS: readonly SupportTopic[] = Object.freeze([</code> |
| <code>interface.14.ts-web-support-topics-supporttopic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/topics.ts:3</code> | <code>export interface SupportTopic</code> |
| <code>interface.14.ts-web-support-topics-supporttopic-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/web/src/support/topics.ts:17</code> | <code>export function supportTopic(id: SupportTopicId): SupportTopic</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-web-support-chat-desktopsupportchat</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-chat-mobilesupportchat</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-conversation-supportchatconnection</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-conversation-supportchatdelivery</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-conversation-supportchatload</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-conversation-supportchatsessionoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-conversation-supportfeedbackdelivery</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-model-human-web-support</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-model-support</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-web-support-topics-supporttopic</code> | notes | no in-repo importer at HEAD |
