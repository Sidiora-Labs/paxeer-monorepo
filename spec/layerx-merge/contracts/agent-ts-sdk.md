# Agent TypeScript SDK contract

Source paths are relative to the repository root. Declaration tables identify the interfaces in the source files; runtime behavior is defined by their implementations.

Agent applications use the merged Paxeer BFT network, EVM chain ID `125` (`0x7d`). The kernel participates in block proposal and execution. Block commit establishes finality. Node JSON-RPC uses the root path; hosted methods are described by the [gateway contract](../../../services/platform/hosted/gateway/openrpc.json).

## account-derivation

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-domain-name</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const ACCOUNT_DERIVATION_DOMAIN_NAME = "Paxeer X Network";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-domain-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const ACCOUNT_DERIVATION_DOMAIN_VERSION = "1";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-hkdf-info-prefix</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const ACCOUNT_DERIVATION_HKDF_INFO_PREFIX = "LX:ACCOUNT-KEY:v1";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-hkdf-salt</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const ACCOUNT_DERIVATION_HKDF_SALT = "paxeer-x-network/layerx-account-key/v1";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-origin</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const ACCOUNT_DERIVATION_ORIGIN = "https://paxportwallet.com";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-purpose</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const ACCOUNT_DERIVATION_PURPOSE = "Derive your LayerX account key";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const ACCOUNT_DERIVATION_VERSION = 1;</code> |
| <code>interface.9.ts-sdk-account-derivation-accountderivationerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export class AccountDerivationError extends Error</code> |
| <code>interface.9.ts-sdk-account-derivation-accountderivationerrorcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export type AccountDerivationErrorCode =</code> |
| <code>interface.9.ts-sdk-account-derivation-addr-precompile-address</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const ADDR_PRECOMPILE_ADDRESS = "0x0000000000000000000000000000000000001004";</code> |
| <code>interface.9.ts-sdk-account-derivation-autobindlayerx</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export async function autoBindLayerX(</code> |
| <code>interface.9.ts-sdk-account-derivation-bindcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export interface BindCall</code> |
| <code>interface.9.ts-sdk-account-derivation-bindfees</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export interface BindFees</code> |
| <code>interface.9.ts-sdk-account-derivation-bindnoncecall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function bindNonceCall(address: string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-bindplan</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export type BindPlan = Readonly&lt;{ action: "already_bound" }&gt; &#124; Readonly&lt;{ action: "bind"; call: BindCall }&gt;;</code> |
| <code>interface.9.ts-sdk-account-derivation-bindtransactionrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function bindTransactionRequest</code> |
| <code>interface.9.ts-sdk-account-derivation-bounddidcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function boundDidCall(address: string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-checksumaddress</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function checksumAddress(address: Uint8Array &#124; string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-decodebindnonce</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function decodeBindNonce(answer: string): bigint</code> |
| <code>interface.9.ts-sdk-account-derivation-decodebounddid</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function decodeBoundDid(answer: string): string &#124; null</code> |
| <code>interface.9.ts-sdk-account-derivation-decodelayerxdid</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function decodeLayerXDid(answer: string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-derivedaccount</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export interface DerivedAccount</code> |
| <code>interface.9.ts-sdk-account-derivation-derivefrombrowserwallet</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export async function deriveFromBrowserWallet(</code> |
| <code>interface.9.ts-sdk-account-derivation-derivefrommnemonic</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function deriveFromMnemonic(</code> |
| <code>interface.9.ts-sdk-account-derivation-derivefromseed</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function deriveFromSeed(seed: Uint8Array, index = 0): DerivedAccount</code> |
| <code>interface.9.ts-sdk-account-derivation-derivefromwalletsignature</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function deriveFromWalletSignature(</code> |
| <code>interface.9.ts-sdk-account-derivation-eip1193requester</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export interface Eip1193Requester</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationhash</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function keyDerivationHash(parameters: KeyDerivationParameters): Uint8Array</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationparameters</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export interface KeyDerivationParameters</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function keyDerivationRequest(parameters: KeyDerivationParameters): WalletRequest</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationtypeddata</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export interface KeyDerivationTypedData</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationtypeddata-2</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function keyDerivationTypedData(parameters: KeyDerivationParameters): KeyDerivationTypedData</code> |
| <code>interface.9.ts-sdk-account-derivation-layerx-coin-type</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export const LAYERX_COIN_TYPE = 19544;</code> |
| <code>interface.9.ts-sdk-account-derivation-layerxbindmessage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function layerXBindMessage(chainId: number &#124; bigint, address: string, nonce: bigint): Uint8Array</code> |
| <code>interface.9.ts-sdk-account-derivation-layerxbindstate</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export interface LayerXBindState</code> |
| <code>interface.9.ts-sdk-account-derivation-layerxdidcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function layerXDidCall(address: string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-normalizewalletsignature</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function normalizeWalletSignature(signature: Uint8Array &#124; string): Uint8Array</code> |
| <code>interface.9.ts-sdk-account-derivation-planlayerxbind</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function planLayerXBind(derived: DerivedAccount, chainId: number &#124; bigint, state: LayerXBindState): BindPlan</code> |
| <code>interface.9.ts-sdk-account-derivation-readlayerxbindstate</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export async function readLayerXBindState(wallet: Eip1193Requester, address: string): Promise&lt;LayerXBindState&gt;</code> |
| <code>interface.9.ts-sdk-account-derivation-requirelayerxbinding</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function requireLayerXBinding(derived: Pick&lt;DerivedAccount, "layerxPublicKey"&gt;, state: LayerXBindState): void</code> |
| <code>interface.9.ts-sdk-account-derivation-sendboundlayerxcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export async function sendBoundLayerXCall(</code> |
| <code>interface.9.ts-sdk-account-derivation-signbindtransaction</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function signBindTransaction(</code> |
| <code>interface.9.ts-sdk-account-derivation-slip10ed25519</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export function slip10Ed25519(seed: Uint8Array, path: readonly number[]): Uint8Array</code> |
| <code>interface.9.ts-sdk-account-derivation-walletrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts</code> | <code>export interface WalletRequest</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9357</code> | bindTransactionRequest returns a type 2 request on chain 125 to the addr precompile. |
| <code>req.9357</code> | bindTransaction signs the bind call with the derived EVM key. |

## agent-http

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-agent-http-agent-daemon-rpc-path</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export const AGENT_DAEMON_RPC_PATH = null;</code> |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-operation-names</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export const AGENT_ENVELOPE_OPERATION_NAMES: readonly AgentOperation[] = Object.freeze&lt;AgentOperation[]&gt;([</code> |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-path</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export const AGENT_ENVELOPE_PATH = "/v1/agent/rpc" as const;</code> |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export const AGENT_ENVELOPE_VERSION = 1 as const;</code> |
| <code>interface.9.ts-sdk-agent-http-agentenvelopesuccess</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export interface AgentEnvelopeSuccess&lt;TValue = unknown&gt;</code> |
| <code>interface.9.ts-sdk-agent-http-agentenvelopesurface</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export type AgentEnvelopeSurface = "gateway";</code> |
| <code>interface.9.ts-sdk-agent-http-agentenvelopetransport</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export class AgentEnvelopeTransport implements ProductionTransport</code> |
| <code>interface.9.ts-sdk-agent-http-agentenvelopetransportoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export interface AgentEnvelopeTransportOptions</code> |
| <code>interface.9.ts-sdk-agent-http-agenthttptransport</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export class AgentHttpTransport implements ProductionTransport</code> |
| <code>interface.9.ts-sdk-agent-http-agenthttptransportoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export interface AgentHttpTransportOptions</code> |
| <code>interface.9.ts-sdk-agent-http-agentsessioncredential</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export class AgentSessionCredential</code> |
| <code>interface.9.ts-sdk-agent-http-checkproofbundleresponse</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function checkProofBundleResponse(request: ProofBundleRequest, response: AgentEnvelopeSuccess): ProofBundleRecord</code> |
| <code>interface.9.ts-sdk-agent-http-checktenantreadinessresponse</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function checkTenantReadinessResponse(response: AgentEnvelopeSuccess): TenantReadinessV1</code> |
| <code>interface.9.ts-sdk-agent-http-decodeagentenveloperesponse</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function decodeAgentEnvelopeResponse</code> |
| <code>interface.9.ts-sdk-agent-http-decodeenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function decodeEnvelope(status: number, encoded: Buffer, operation: AgentOperation): unknown</code> |
| <code>interface.9.ts-sdk-agent-http-decodeprogramboundaryerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function decodeProgramBoundaryError(status: number, value: unknown): ProgramBoundaryError</code> |
| <code>interface.9.ts-sdk-agent-http-decodeproofbundlerecord</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function decodeProofBundleRecord(value: unknown): ProofBundleRecord</code> |
| <code>interface.9.ts-sdk-agent-http-decodetenantreadiness</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function decodeTenantReadiness(value: unknown): TenantReadinessV1</code> |
| <code>interface.9.ts-sdk-agent-http-encodeagentenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function encodeAgentEnvelope(</code> |
| <code>interface.9.ts-sdk-agent-http-encodeprogrammutationbody</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function encodeProgramMutationBody(signed: unknown): Buffer</code> |
| <code>interface.9.ts-sdk-agent-http-encodeproofbundlerequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export function encodeProofBundleRequest(value: unknown): ProofBundleRequest</code> |
| <code>interface.9.ts-sdk-agent-http-layerxkeycredential</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export class LayerXKeyCredential</code> |
| <code>interface.9.ts-sdk-agent-http-programboundaryerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export class ProgramBoundaryError extends PlatformSdkError</code> |
| <code>interface.9.ts-sdk-agent-http-proofbundleread</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export interface ProofBundleRead</code> |
| <code>interface.9.ts-sdk-agent-http-proofbundlerecord</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export type ProofBundleRecord =</code> |
| <code>interface.9.ts-sdk-agent-http-proofbundlerequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export interface ProofBundleRequest</code> |
| <code>interface.9.ts-sdk-agent-http-proofbundleverificationlevel</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export type ProofBundleVerificationLevel = Rung;</code> |
| <code>interface.9.ts-sdk-agent-http-tenantreadinessv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export interface TenantReadinessV1</code> |
| <code>interface.9.ts-sdk-agent-http-tenantrecoveryreasonv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/agent-http.ts</code> | <code>export type TenantRecoveryReasonV1 = "recovery_pending" &#124; "store_unavailable" &#124; "store_refused"</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9359</code> | Every envelope request carries contract major 2. |
| <code>req.9359</code> | A proof bundle reply reports its verification as a rung. |

## ai-market

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-ai-market-activityidof</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function activityIdOf(signed: Uint8Array): Hex32</code> |
| <code>interface.9.ts-sdk-ai-market-ai-application-errors</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const AI_APPLICATION_ERRORS = Object.freeze(</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export class AiMarketError extends Error</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketerrorcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type AiMarketErrorCode =</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketerrordetail</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface AiMarketErrorDetail</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketviews</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export class AiMarketViews</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketviewsoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface AiMarketViewsOptions</code> |
| <code>interface.9.ts-sdk-ai-market-approval</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export class Approval</code> |
| <code>interface.9.ts-sdk-ai-market-approvalterms</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface ApprovalTerms extends BindFees</code> |
| <code>interface.9.ts-sdk-ai-market-availability</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type Availability =</code> |
| <code>interface.9.ts-sdk-ai-market-callboundary</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type CallBoundary = "mutation" &#124; "program-read";</code> |
| <code>interface.9.ts-sdk-ai-market-claim</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const CLAIM = 0x0602;</code> |
| <code>interface.9.ts-sdk-ai-market-claimrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface ClaimRequest</code> |
| <code>interface.9.ts-sdk-ai-market-components</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type Components = Readonly&lt;Record&lt;FeatureName, Availability&gt;&gt;;</code> |
| <code>interface.9.ts-sdk-ai-market-decodeclaimrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeClaimRequest(bytes: Uint8Array): ClaimRequest</code> |
| <code>interface.9.ts-sdk-ai-market-decodedenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface DecodedEnvelope</code> |
| <code>interface.9.ts-sdk-ai-market-decodeepochpage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeEpochPage(</code> |
| <code>interface.9.ts-sdk-ai-market-decodefundrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeFundRequest(bytes: Uint8Array): FundRequest</code> |
| <code>interface.9.ts-sdk-ai-market-decodemarketview</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeMarketView(value: unknown, market: Hex32): MarketView</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramcallactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeProgramCallActivity(</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramscallresult</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeProgramsCallResult(</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsreceipt</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>async function decodeProgramsReceipt</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsrpcrevert</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeProgramsRpcRevert(</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsstatehead</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeProgramsStateHead(</code> |
| <code>interface.9.ts-sdk-ai-market-decoderefundrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeRefundRequest(bytes: Uint8Array): RefundRequest</code> |
| <code>interface.9.ts-sdk-ai-market-decoderequestenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function decodeRequestEnvelope(input: Uint8Array): DecodedEnvelope</code> |
| <code>interface.9.ts-sdk-ai-market-domainhash</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function domainHash(domain: string, bytes: Uint8Array): Hex32</code> |
| <code>interface.9.ts-sdk-ai-market-domainstatus</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type DomainStatus = "pending" &#124; "completed";</code> |
| <code>interface.9.ts-sdk-ai-market-encodeclaimrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function encodeClaimRequest(value: ClaimRequest): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-encodefundrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function encodeFundRequest(value: FundRequest): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-encodeprogramcallactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function encodeProgramCallActivity(</code> |
| <code>interface.9.ts-sdk-ai-market-encodeprogramsstateheadcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function encodeProgramsStateHeadCall(program: Hex32): PrecompileCall</code> |
| <code>interface.9.ts-sdk-ai-market-encoderefundrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function encodeRefundRequest(value: RefundRequest): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-encoderequestenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function encodeRequestEnvelope(envelope: RequestEnvelope): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-encodesnapshotbinding</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function encodeSnapshotBinding(binding: SnapshotBinding): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-entrypoint</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const ENTRYPOINT = "layerx_call";</code> |
| <code>interface.9.ts-sdk-ai-market-epochentry</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface EpochEntry</code> |
| <code>interface.9.ts-sdk-ai-market-epochpage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface EpochPage</code> |
| <code>interface.9.ts-sdk-ai-market-epochstatus</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type EpochStatus =</code> |
| <code>interface.9.ts-sdk-ai-market-featurename</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type FeatureName =</code> |
| <code>interface.9.ts-sdk-ai-market-freshness</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type Freshness =</code> |
| <code>interface.9.ts-sdk-ai-market-freshnessof</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function freshnessOf(</code> |
| <code>interface.9.ts-sdk-ai-market-fund</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const FUND = 0x0601;</code> |
| <code>interface.9.ts-sdk-ai-market-funding-policy-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const FUNDING_POLICY_VERSION = 1n;</code> |
| <code>interface.9.ts-sdk-ai-market-fundrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface FundRequest</code> |
| <code>interface.9.ts-sdk-ai-market-guest-abi</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const GUEST_ABI: number = 5;</code> |
| <code>interface.9.ts-sdk-ai-market-hex32</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type Hex32 = string;</code> |
| <code>interface.9.ts-sdk-ai-market-kindfilter</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type KindFilter = "all" &#124; ParticipantKind;</code> |
| <code>interface.9.ts-sdk-ai-market-marketview</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface MarketView</code> |
| <code>interface.9.ts-sdk-ai-market-native-call-protocol-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const NATIVE_CALL_PROTOCOL_VERSION = 2;</code> |
| <code>interface.9.ts-sdk-ai-market-nativeterms</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface NativeTerms extends Omit&lt;BindFees, "evmNonce"&gt;</code> |
| <code>interface.9.ts-sdk-ai-market-operationeffect</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type OperationEffect =</code> |
| <code>interface.9.ts-sdk-ai-market-operationjournal</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export class OperationJournal</code> |
| <code>interface.9.ts-sdk-ai-market-operationrecord</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export class OperationRecord implements RecordFields</code> |
| <code>interface.9.ts-sdk-ai-market-operationrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface OperationRequest</code> |
| <code>interface.9.ts-sdk-ai-market-operationsigner</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface OperationSigner</code> |
| <code>interface.9.ts-sdk-ai-market-operationspec</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface OperationSpec</code> |
| <code>interface.9.ts-sdk-ai-market-operationstate</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type OperationState =</code> |
| <code>interface.9.ts-sdk-ai-market-parsedecimalu128</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function parseDecimalU128(text: string): bigint</code> |
| <code>interface.9.ts-sdk-ai-market-parsedecimalu64</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function parseDecimalU64(text: string): bigint</code> |
| <code>interface.9.ts-sdk-ai-market-participantcursor</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export class ParticipantCursor</code> |
| <code>interface.9.ts-sdk-ai-market-participantkind</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type ParticipantKind = "worker" &#124; "evaluator";</code> |
| <code>interface.9.ts-sdk-ai-market-participantpage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface ParticipantPage</code> |
| <code>interface.9.ts-sdk-ai-market-participantquery</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface ParticipantQuery</code> |
| <code>interface.9.ts-sdk-ai-market-participantrow</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface ParticipantRow</code> |
| <code>interface.9.ts-sdk-ai-market-paxai-limits</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const PAXAI_LIMITS = Object.freeze(</code> |
| <code>interface.9.ts-sdk-ai-market-paxai-operations</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const PAXAI_OPERATIONS: ReadonlyMap&lt;number, OperationSpec&gt; = new Map(</code> |
| <code>interface.9.ts-sdk-ai-market-preparedoperation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export class PreparedOperation</code> |
| <code>interface.9.ts-sdk-ai-market-prepareoperation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function prepareOperation(</code> |
| <code>interface.9.ts-sdk-ai-market-program-call-ordinal</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const PROGRAM_CALL_ORDINAL = 3;</code> |
| <code>interface.9.ts-sdk-ai-market-programcallactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface ProgramCallActivity</code> |
| <code>interface.9.ts-sdk-ai-market-programs-module</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const PROGRAMS_MODULE = 9;</code> |
| <code>interface.9.ts-sdk-ai-market-programs-precompile</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const PROGRAMS_PRECOMPILE = "0x000000000000000000000000000000000000101d";</code> |
| <code>interface.9.ts-sdk-ai-market-projectionstate</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type ProjectionState =</code> |
| <code>interface.9.ts-sdk-ai-market-refund-free</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export const REFUND_FREE = 0x0604;</code> |
| <code>interface.9.ts-sdk-ai-market-refundrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface RefundRequest</code> |
| <code>interface.9.ts-sdk-ai-market-requestenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface RequestEnvelope</code> |
| <code>interface.9.ts-sdk-ai-market-requestintent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function requestIntent(unsigned: Uint8Array): Hex32</code> |
| <code>interface.9.ts-sdk-ai-market-requirecurrent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function requireCurrent(</code> |
| <code>interface.9.ts-sdk-ai-market-requireretainedepoch</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function requireRetainedEpoch(entry: EpochEntry): Hex32</code> |
| <code>interface.9.ts-sdk-ai-market-review</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export class Review</code> |
| <code>interface.9.ts-sdk-ai-market-reviewfield</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type ReviewField =</code> |
| <code>interface.9.ts-sdk-ai-market-scorestatus</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export type ScoreStatus =</code> |
| <code>interface.9.ts-sdk-ai-market-signingpreimage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function signingPreimage(unsigned: Uint8Array): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-snapshotbinding</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export interface SnapshotBinding</code> |
| <code>interface.9.ts-sdk-ai-market-snapshotidof</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/ai_market.ts</code> | <code>export function snapshotIdOf(binding: SnapshotBinding): Hex32</code> |

## bridge

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-bridge-bridge-events</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/bridge.ts</code> | <code>export const BRIDGE_EVENTS: readonly PrecompileEventSpec[] = [</code> |
| <code>interface.9.ts-sdk-bridge-bridgeinattestation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/bridge.ts</code> | <code>export interface BridgeInAttestation</code> |
| <code>interface.9.ts-sdk-bridge-bridgeincall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/bridge.ts</code> | <code>export function bridgeInCall(attestation: BridgeInAttestation): PrecompileCall</code> |
| <code>interface.9.ts-sdk-bridge-bridgeoutcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/bridge.ts</code> | <code>export function bridgeOutCall(chain: bigint, asset: string, amount: bigint, recipient: string): PrecompileCall</code> |
| <code>interface.9.ts-sdk-bridge-decodebridgeevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/bridge.ts</code> | <code>export function decodeBridgeEvent(log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-bridge-layerx-bridge-precompile</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/bridge.ts</code> | <code>export const LAYERX_BRIDGE_PRECOMPILE = "0x0000000000000000000000000000000000001016";</code> |
| <code>interface.9.ts-sdk-bridge-sendbridgein</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/bridge.ts</code> | <code>export const sendBridgeIn = (wallet: Eip1193Requester, from: string, attestation: BridgeInAttestation): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-bridge-sendbridgeout</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/bridge.ts</code> | <code>export const sendBridgeOut = (</code> |

## exchange

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-exchange-abiargument</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export type AbiArgument = bigint &#124; boolean &#124; string &#124; readonly AbiArgument[];</code> |
| <code>interface.9.ts-sdk-exchange-abieventtopic</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function abiEventTopic(signature: string): string</code> |
| <code>interface.9.ts-sdk-exchange-abiselector</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function abiSelector(signature: string): string</code> |
| <code>interface.9.ts-sdk-exchange-abisignature</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function abiSignature(name: string, types: readonly PrecompileAbiType[]): string</code> |
| <code>interface.9.ts-sdk-exchange-decodedprecompileevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export interface DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-exchange-decodeeventfrom</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function decodeEventFrom(specs: readonly PrecompileEventSpec[], log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-exchange-decodeeventwithspec</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function decodeEventWithSpec(spec: PrecompileEventSpec, log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-exchange-decodeexchangeevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function decodeExchangeEvent(log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-exchange-encodeabicall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function encodeAbiCall(</code> |
| <code>interface.9.ts-sdk-exchange-exchange-events</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export const EXCHANGE_EVENTS: readonly PrecompileEventSpec[] = [</code> |
| <code>interface.9.ts-sdk-exchange-exchangecancelordercall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function exchangeCancelOrderCall(orderId: string): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-exchangedepositmargincall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function exchangeDepositMarginCall(account: string, amountWei: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-exchangedepositmargintokencall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function exchangeDepositMarginTokenCall(pointer: string, amount: bigint, account: string): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-exchangeplaceordercall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function exchangePlaceOrderCall(order:</code> |
| <code>interface.9.ts-sdk-exchange-exchangerequestsettlementcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function exchangeRequestSettlementCall(positionId: string): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-exchangewithdrawmargincall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function exchangeWithdrawMarginCall(account: string, assetId: string, amount: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-layerx-exchange-precompile</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export const LAYERX_EXCHANGE_PRECOMPILE = "0x0000000000000000000000000000000000001015";</code> |
| <code>interface.9.ts-sdk-exchange-precompileabierror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export class PrecompileAbiError extends Error</code> |
| <code>interface.9.ts-sdk-exchange-precompileabierrorcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export type PrecompileAbiErrorCode =</code> |
| <code>interface.9.ts-sdk-exchange-precompileabitype</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export type PrecompileAbiType =</code> |
| <code>interface.9.ts-sdk-exchange-precompileabivalue</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export type PrecompileAbiValue = bigint &#124; boolean &#124; string &#124; readonly string[];</code> |
| <code>interface.9.ts-sdk-exchange-precompilecall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export interface PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-precompileeventfields</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export type PrecompileEventFields = Readonly&lt;Record&lt;string, bigint &#124; boolean &#124; string&gt;&gt;;</code> |
| <code>interface.9.ts-sdk-exchange-precompileeventinput</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export interface PrecompileEventInput</code> |
| <code>interface.9.ts-sdk-exchange-precompileeventsignature</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function precompileEventSignature(spec: PrecompileEventSpec): string</code> |
| <code>interface.9.ts-sdk-exchange-precompileeventspec</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export interface PrecompileEventSpec</code> |
| <code>interface.9.ts-sdk-exchange-precompilelog</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export interface PrecompileLog</code> |
| <code>interface.9.ts-sdk-exchange-precompiletransactionrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function precompileTransactionRequest(from: string, call: PrecompileCall): WalletRequest</code> |
| <code>interface.9.ts-sdk-exchange-precompilewritecall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export function precompileWriteCall(</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangecancelorder</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export const sendExchangeCancelOrder = (wallet: Eip1193Requester, from: string, orderId: string): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangedepositmargin</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export const sendExchangeDepositMargin = (</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangedepositmargintoken</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export const sendExchangeDepositMarginToken = (</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangeplaceorder</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export const sendExchangePlaceOrder = (</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangerequestsettlement</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export const sendExchangeRequestSettlement = (wallet: Eip1193Requester, from: string, positionId: string): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangewithdrawmargin</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export const sendExchangeWithdrawMargin = (</code> |
| <code>interface.9.ts-sdk-exchange-sendprecompilecall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/exchange.ts</code> | <code>export async function sendPrecompileCall(wallet: Eip1193Requester, from: string, call: PrecompileCall): Promise&lt;string&gt;</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9360</code> | exchangePlaceOrderCall encodes the v2 selector with the account id. |
| <code>req.9360</code> | sendExchangeRequestSettlement asks the wallet for a type 2 transaction on chain 125. |

## gas-station

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-gas-station-assembleeip7702authorization</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function assembleEip7702Authorization(</code> |
| <code>interface.9.ts-sdk-gas-station-discoversponsorednonce</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export async function discoverSponsoredNonce(</code> |
| <code>interface.9.ts-sdk-gas-station-eip7702authorization</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface Eip7702Authorization</code> |
| <code>interface.9.ts-sdk-gas-station-eip7702authorizationdigest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function eip7702AuthorizationDigest(</code> |
| <code>interface.9.ts-sdk-gas-station-gas-station-quote-url</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export const GAS_STATION_QUOTE_URL =</code> |
| <code>interface.9.ts-sdk-gas-station-gasquote</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface GasQuote</code> |
| <code>interface.9.ts-sdk-gas-station-gasquotedigest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function gasQuoteDigest(</code> |
| <code>interface.9.ts-sdk-gas-station-gasquoterequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface GasQuoteRequest</code> |
| <code>interface.9.ts-sdk-gas-station-gasrefusal</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface GasRefusal</code> |
| <code>interface.9.ts-sdk-gas-station-gasrefusalcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export type GasRefusalCode =</code> |
| <code>interface.9.ts-sdk-gas-station-gasresult</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export type GasResult&lt;T&gt; =</code> |
| <code>interface.9.ts-sdk-gas-station-gasstationconfig</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface GasStationConfig</code> |
| <code>interface.9.ts-sdk-gas-station-gasstationquoteurl</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function gasStationQuoteUrl(</code> |
| <code>interface.9.ts-sdk-gas-station-gassubmissionidentity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface GasSubmissionIdentity</code> |
| <code>interface.9.ts-sdk-gas-station-gassubmissionoutcome</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export type GasSubmissionOutcome =</code> |
| <code>interface.9.ts-sdk-gas-station-gassubmissionstatus</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface GasSubmissionStatus</code> |
| <code>interface.9.ts-sdk-gas-station-gassubmissiontransaction</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface GasSubmissionTransaction</code> |
| <code>interface.9.ts-sdk-gas-station-nativefeepreference</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export type NativeFeePreference =</code> |
| <code>interface.9.ts-sdk-gas-station-readnativefeepreference</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export async function readNativeFeePreference(</code> |
| <code>interface.9.ts-sdk-gas-station-requestgasquote</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export async function requestGasQuote(</code> |
| <code>interface.9.ts-sdk-gas-station-requestgassubmissionstatus</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function requestGasSubmissionStatus(</code> |
| <code>interface.9.ts-sdk-gas-station-retrygassubmission</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function retryGasSubmission(</code> |
| <code>interface.9.ts-sdk-gas-station-sidiora-decimals</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export const SIDIORA_DECIMALS = 6;</code> |
| <code>interface.9.ts-sdk-gas-station-sidiora-token</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export const SIDIORA_TOKEN = "0x21f7b20a555199fa73A238B1a91FD0f549068fEe";</code> |
| <code>interface.9.ts-sdk-gas-station-signedeip7702authorization</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface SignedEip7702Authorization extends Eip7702Authorization</code> |
| <code>interface.9.ts-sdk-gas-station-signedgasquote</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface SignedGasQuote</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatch</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface SponsoredBatch</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatchcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function sponsoredBatchCall(</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatchdigest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function sponsoredBatchDigest(batch: SponsoredBatch): GasResult&lt;string&gt;</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredconsent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export interface SponsoredConsent</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredconsent-2</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export function sponsoredConsent(</code> |
| <code>interface.9.ts-sdk-gas-station-submitsponsoredgasbatch</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/gas-station.ts</code> | <code>export async function submitSponsoredGasBatch(</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9361</code> | discoverSponsoredNonce calls eth_call with the pending tag. |
| <code>req.9361</code> | readNativeFeePreference returns the denomination from feetoken getFeeDenom. |
| <code>req.9361</code> | A sponsored batch accepted by the station reports its transaction hash at the submitted rung. |

## generated-client

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-generated-client-amount</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type Amount = bigint;</code> |
| <code>interface.9.ts-sdk-generated-client-apierror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApiError</code> |
| <code>interface.9.ts-sdk-generated-client-approval-contract-introduced</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export const APPROVAL_CONTRACT_INTRODUCED = "1.1" as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approval-decision-outcomes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export const APPROVAL_DECISION_OUTCOMES = ["Granted", "Rejected", "Expired", "Defective", "AlreadyDecided", "Conflict"] as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approval-enforcement-notice</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export const APPROVAL_ENFORCEMENT_NOTICE = "An approval hold is a daemon-enforced restriction. It confers no protocol authority, and bypassing the daemon bypasses the restriction." as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approval-event-kinds</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export const APPROVAL_EVENT_KINDS = ["Created", "Granted", "Rejected", "Expired", "Defective"] as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approval-states</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export const APPROVAL_STATES = ["Held", "Granted", "Rejected", "Expired", "Defective"] as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approvalapproverequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApprovalApproveRequest { tenant: string; approvalId: string; idempotencyKey: string }</code> |
| <code>interface.9.ts-sdk-generated-client-approvaldecision</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApprovalDecision</code> |
| <code>interface.9.ts-sdk-generated-client-approvaldecisionoutcome</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type ApprovalDecisionOutcome = (typeof APPROVAL_DECISION_OUTCOMES)[number];</code> |
| <code>interface.9.ts-sdk-generated-client-approvaleventkind</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type ApprovalEventKind = (typeof APPROVAL_EVENT_KINDS)[number];</code> |
| <code>interface.9.ts-sdk-generated-client-approvalgetrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApprovalGetRequest { tenant: string; approvalId: string }</code> |
| <code>interface.9.ts-sdk-generated-client-approvallifecycleevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApprovalLifecycleEvent</code> |
| <code>interface.9.ts-sdk-generated-client-approvallistrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApprovalListRequest { tenant: string; cursor: string &#124; null; pageLimit: number }</code> |
| <code>interface.9.ts-sdk-generated-client-approvalpage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApprovalPage { approvals: readonly ApprovalRecord[]; nextCursor: string &#124; null }</code> |
| <code>interface.9.ts-sdk-generated-client-approvalrecord</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApprovalRecord</code> |
| <code>interface.9.ts-sdk-generated-client-approvalrejectrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface ApprovalRejectRequest { tenant: string; approvalId: string; idempotencyKey: string; reason: string }</code> |
| <code>interface.9.ts-sdk-generated-client-approvalstate</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type ApprovalState = (typeof APPROVAL_STATES)[number];</code> |
| <code>interface.9.ts-sdk-generated-client-budgetlimit</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type BudgetLimit = bigint;</code> |
| <code>interface.9.ts-sdk-generated-client-client</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export class Client</code> |
| <code>interface.9.ts-sdk-generated-client-decodenativeapprovallistresult</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function decodeNativeApprovalListResult(value: unknown): NativeApprovalListResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-decodenativeapprovalresult</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function decodeNativeApprovalResult(value: unknown): NativeApprovalResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-decodenativeprepareresult</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function decodeNativePrepareResult(value: unknown): NativePrepareResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-encodenativeapprovaldecision</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function encodeNativeApprovalDecision(value: NativeApprovalDecisionV1): NativeApprovalDecisionV1</code> |
| <code>interface.9.ts-sdk-generated-client-encodenativeapprovalget</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function encodeNativeApprovalGet(approval_id: string): {variant:"native_v1";approval_id:string}</code> |
| <code>interface.9.ts-sdk-generated-client-encodenativepreparerequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function encodeNativePrepareRequest(value: NativePrepareRequestV1): NativePrepareRequestV1</code> |
| <code>interface.9.ts-sdk-generated-client-errorclass</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type ErrorClass = "TransportFailure" &#124; "Deadline" &#124; "ProtocolIncompatibility" &#124; "UnavailableCapability" &#124; "CoreRejection" &#124; "VerificationFailure" &#124; "PolicyRefusal" &#124; "CapabilityRefusal" &#124; "BudgetRefusal" &#124; "RateLimit" &#124; "IdempotencyConflict" &#124; "InternalFault";</code> |
| <code>interface.9.ts-sdk-generated-client-holdreason</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface HoldReason { code: string; message: string }</code> |
| <code>interface.9.ts-sdk-generated-client-idempotentmutation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface IdempotentMutation&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-generated-client-layerx-sdk-ts-package</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function layerx_sdk_ts_package(): typeof PACKAGE_METADATA</code> |
| <code>interface.9.ts-sdk-generated-client-nativeactivityv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface NativeActivityV1 { version: "1"; module: string; ordinal: string }</code> |
| <code>interface.9.ts-sdk-generated-client-nativeapprovaldecisionv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface NativeApprovalDecisionV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativeapprovallistresultv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface NativeApprovalListResultV1 { version: "1"; approvals: NativeApprovalResultV1[] }</code> |
| <code>interface.9.ts-sdk-generated-client-nativeapprovalresultv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface NativeApprovalResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativelocalgrantv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface NativeLocalGrantV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativepreparerequestv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface NativePrepareRequestV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativeprepareresultv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface NativePrepareResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativepurposev1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface NativePurposeV1</code> |
| <code>interface.9.ts-sdk-generated-client-operation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type Operation = "agent.register" &#124; "approval.approve" &#124; "approval.get" &#124; "approval.list" &#124; "approval.reject" &#124; "availability.fetch" &#124; "budget.create" &#124; "budget.fund" &#124; "budget.list" &#124; "budget.reconciliation" &#124; "budget.revoke" &#124; "budget.state" &#124; "capability.attenuate" &#124; "capability.create" &#124; </code> |
| <code>interface.9.ts-sdk-generated-client-parseamount</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function parseAmount(value: string): Amount</code> |
| <code>interface.9.ts-sdk-generated-client-parsebudgetlimit</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function parseBudgetLimit(value: string): BudgetLimit</code> |
| <code>interface.9.ts-sdk-generated-client-parsesequence</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function parseSequence(value: string): Sequence</code> |
| <code>interface.9.ts-sdk-generated-client-parsetimestampseconds</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function parseTimestampSeconds(value: string): TimestampSeconds</code> |
| <code>interface.9.ts-sdk-generated-client-requireverified</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export function requireVerified&lt;T&gt;(requested: Rung, read: VerifiedRead&lt;T&gt;): VerifiedRead&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-generated-client-rung</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export enum Rung</code> |
| <code>interface.9.ts-sdk-generated-client-sequence</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type Sequence = bigint;</code> |
| <code>interface.9.ts-sdk-generated-client-structuredactivitydisclosure</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface StructuredActivityDisclosure</code> |
| <code>interface.9.ts-sdk-generated-client-submissionstate</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type SubmissionState =</code> |
| <code>interface.9.ts-sdk-generated-client-timestampseconds</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export type TimestampSeconds = bigint;</code> |
| <code>interface.9.ts-sdk-generated-client-transport</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface Transport</code> |
| <code>interface.9.ts-sdk-generated-client-verifiedread</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/client.ts</code> | <code>export interface VerifiedRead&lt;T&gt;</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9358</code> | The generated SubmissionState variants are Submitted, Receipted, Committed and Reverted. |

## generated-mirror

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-generated-mirror-mirror-archive-magic</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts</code> | <code>export const MIRROR_ARCHIVE_MAGIC = "4c584d4952524f52" as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-archive-bytes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts</code> | <code>export const MIRROR_MAX_ARCHIVE_BYTES = 67_108_864 as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-json-depth</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts</code> | <code>export const MIRROR_MAX_JSON_DEPTH = 32 as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-sources</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts</code> | <code>export const MIRROR_MAX_SOURCES = 8 as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirror-schema-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts</code> | <code>export const MIRROR_SCHEMA_VERSION = 2 as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirrorerrorcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts</code> | <code>export type MirrorErrorCode =</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirrorpolicykind</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts</code> | <code>export type MirrorPolicyKind = "exact" &#124; "ordered-preference" &#124; "agreement";</code> |

## generated-precompile-writes

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-generated-precompile-writes-launchpad-events</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export const LAUNCHPAD_EVENTS = [</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-launchpad-precompile</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export const LAUNCHPAD_PRECOMPILE = "0x0000000000000000000000000000000000001017";</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-launchpad-writes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export const LAUNCHPAD_WRITES =</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-perps-events</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export const PERPS_EVENTS = [</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-perps-precompile</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export const PERPS_PRECOMPILE = "0x0000000000000000000000000000000000001015";</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-perps-writes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export const PERPS_WRITES =</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-precompilewrite</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export interface PrecompileWrite</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-spot-precompiles</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export const SPOT_PRECOMPILES =</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-spot-writes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts</code> | <code>export const SPOT_WRITES =</code> |

## generated-receipt

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts</code> | <code>export const PROGRAM_ABI_V1 = 1;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v2</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts</code> | <code>export const PROGRAM_ABI_V2 = 2;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v3</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts</code> | <code>export const PROGRAM_ABI_V3 = 3;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v4</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts</code> | <code>export const PROGRAM_ABI_V4 = 4;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v5</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts</code> | <code>export const PROGRAM_ABI_V5 = 5;</code> |
| <code>interface.9.ts-sdk-generated-receipt-receiptfailurecode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts</code> | <code>export enum ReceiptFailureCode</code> |
| <code>interface.9.ts-sdk-generated-receipt-supportsprogramguestabi</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts</code> | <code>export function supportsProgramGuestAbi(version: number): boolean</code> |

## history

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-history-decodehistoryassetmetadata</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export function decodeHistoryAssetMetadata</code> |
| <code>interface.9.ts-sdk-history-decodehistorypage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export function decodeHistoryPage</code> |
| <code>interface.9.ts-sdk-history-decodehistoryrow</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export function decodeHistoryRow(value: unknown, unified: boolean): HistoryRow</code> |
| <code>interface.9.ts-sdk-history-decodepaxeerhistorypage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export function decodePaxeerHistoryPage</code> |
| <code>interface.9.ts-sdk-history-decodeunifiedhistorypage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export function decodeUnifiedHistoryPage</code> |
| <code>interface.9.ts-sdk-history-history-default-limit</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export const HISTORY_DEFAULT_LIMIT = 50;</code> |
| <code>interface.9.ts-sdk-history-history-maximum-accounts</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export const HISTORY_MAXIMUM_ACCOUNTS = 16;</code> |
| <code>interface.9.ts-sdk-history-history-maximum-limit</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export const HISTORY_MAXIMUM_LIMIT = 100;</code> |
| <code>interface.9.ts-sdk-history-historyaccount</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export function historyAccount(account: string): string</code> |
| <code>interface.9.ts-sdk-history-historyassetmetadata</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export interface HistoryAssetMetadata</code> |
| <code>interface.9.ts-sdk-history-historychain</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export type HistoryChain = "layerx" &#124; "paxeer";</code> |
| <code>interface.9.ts-sdk-history-historydirection</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export type HistoryDirection = "in" &#124; "out";</code> |
| <code>interface.9.ts-sdk-history-historyoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export interface HistoryOptions</code> |
| <code>interface.9.ts-sdk-history-historypage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export interface HistoryPage</code> |
| <code>interface.9.ts-sdk-history-historyparams</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export function historyParams</code> |
| <code>interface.9.ts-sdk-history-historyrow</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export interface HistoryRow</code> |
| <code>interface.9.ts-sdk-history-historyside</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export interface HistorySide</code> |
| <code>interface.9.ts-sdk-history-unifiedhistorypage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/history.ts</code> | <code>export interface UnifiedHistoryPage</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9353</code> | historyCursor and historyParams agree on the block position cursor. |
| <code>req.9353</code> | decodeHistoryPage keeps each row's transaction hash and refuses a row without one. |

## intent

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-intent-decodehumanenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export function decodeHumanEnvelope(status: number, encoded: Buffer): unknown</code> |
| <code>interface.9.ts-sdk-intent-decodehumanerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export function decodeHumanError(status: number, value: unknown, trace: string): HumanIntentError</code> |
| <code>interface.9.ts-sdk-intent-decodeintentplan</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export function decodeIntentPlan(value: unknown): IntentPlan</code> |
| <code>interface.9.ts-sdk-intent-decodeintentsubmission</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export function decodeIntentSubmission(value: unknown): IntentSubmission</code> |
| <code>interface.9.ts-sdk-intent-encodeplanintentrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export function encodePlanIntentRequest(request: PlanIntentRequest): Readonly&lt;Record&lt;string, unknown&gt;&gt;</code> |
| <code>interface.9.ts-sdk-intent-encodesubmitplanrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export function encodeSubmitPlanRequest(request: SubmitPlanRequest): Readonly&lt;Record&lt;string, unknown&gt;&gt;</code> |
| <code>interface.9.ts-sdk-intent-human-error-codes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export const HUMAN_ERROR_CODES = [</code> |
| <code>interface.9.ts-sdk-intent-human-retriabilities</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export const HUMAN_RETRIABILITIES = ["retriable", "retriable-after", "structural", "final"] as const;</code> |
| <code>interface.9.ts-sdk-intent-humanerrorcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export type HumanErrorCode = (typeof HUMAN_ERROR_CODES)[number];</code> |
| <code>interface.9.ts-sdk-intent-humanintentclient</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export class HumanIntentClient</code> |
| <code>interface.9.ts-sdk-intent-humanintentclientoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface HumanIntentClientOptions</code> |
| <code>interface.9.ts-sdk-intent-humanintenterror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export class HumanIntentError extends PlatformSdkError</code> |
| <code>interface.9.ts-sdk-intent-humanintenterrordetails</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface HumanIntentErrorDetails</code> |
| <code>interface.9.ts-sdk-intent-humanretriability</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export type HumanRetriability = (typeof HUMAN_RETRIABILITIES)[number];</code> |
| <code>interface.9.ts-sdk-intent-intent-domains</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export const INTENT_DOMAINS = ["paxeer", "layerx"] as const;</code> |
| <code>interface.9.ts-sdk-intent-intent-endpoint-kinds</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export const INTENT_ENDPOINT_KINDS = ["paxeer-wallet", "human", "agent", "agent-budget"] as const;</code> |
| <code>interface.9.ts-sdk-intent-intentconstraints</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface IntentConstraints</code> |
| <code>interface.9.ts-sdk-intent-intentdomain</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export type IntentDomain = (typeof INTENT_DOMAINS)[number];</code> |
| <code>interface.9.ts-sdk-intent-intentendpointkind</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export type IntentEndpointKind = (typeof INTENT_ENDPOINT_KINDS)[number];</code> |
| <code>interface.9.ts-sdk-intent-intentendpointref</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface IntentEndpointRef</code> |
| <code>interface.9.ts-sdk-intent-intentleg</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface IntentLeg</code> |
| <code>interface.9.ts-sdk-intent-intentlegbinding</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface IntentLegBinding</code> |
| <code>interface.9.ts-sdk-intent-intentmoney</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface IntentMoney</code> |
| <code>interface.9.ts-sdk-intent-intentoperation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export type IntentOperation = keyof typeof INTENT_ROUTES;</code> |
| <code>interface.9.ts-sdk-intent-intentplan</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface IntentPlan</code> |
| <code>interface.9.ts-sdk-intent-intentsigningrequirement</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface IntentSigningRequirement</code> |
| <code>interface.9.ts-sdk-intent-intentsubmission</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface IntentSubmission</code> |
| <code>interface.9.ts-sdk-intent-planintentrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface PlanIntentRequest</code> |
| <code>interface.9.ts-sdk-intent-submitplanrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/intent.ts</code> | <code>export interface SubmitPlanRequest</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9362</code> | A decoded plan exposes each leg's call and nonce. |
| <code>req.9362</code> | A non-sponsored plan with a missing leg nonce is refused. |

## launchpad

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-launchpad-decodelaunchpadevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function decodeLaunchpadEvent(log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadapproval</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export async function launchpadApproval(rpc: Eip1193Requester, owner: string, spend: LaunchpadSpend): Promise&lt;PrecompileCall &#124; undefined&gt;</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadapprovecall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function launchpadApproveCall(token: string, amount: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadbuycall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function launchpadBuyCall(order: LaunchpadSwapOrder): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function launchpadCall(method: LaunchpadWrite, args: readonly AbiArgument[], value = 0n): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadclaimfeescall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function launchpadClaimFeesCall(nftId: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadcreatemarketcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function launchpadCreateMarketCall(name: string, symbol: string, feeStrategy: number, optical: string = NO_OPTICAL): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadnftcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function launchpadNftCall(write: LaunchpadNftWrite, nftId: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadnftwrite</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export type LaunchpadNftWrite = "claimAirdrop" &#124; "executeAirdrop" &#124; "executeBurn" &#124; "executeLpRewards";</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadsellcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function launchpadSellCall(order: LaunchpadSwapOrder): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadsetfeestrategycall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export function launchpadSetFeeStrategyCall(nftId: bigint, feeStrategy: number): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadspend</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export type LaunchpadSpend =</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadswaporder</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export interface LaunchpadSwapOrder</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadwrite</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export type LaunchpadWrite = keyof typeof LAUNCHPAD_WRITES;</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadbuy</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export const sendLaunchpadBuy = (wallet: Eip1193Requester, from: string, order: LaunchpadSwapOrder): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadclaimfees</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export const sendLaunchpadClaimFees = (wallet: Eip1193Requester, from: string, nftId: bigint): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadcreatemarket</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export const sendLaunchpadCreateMarket = (</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadnftwrite</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export const sendLaunchpadNftWrite = (</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadsell</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export const sendLaunchpadSell = (wallet: Eip1193Requester, from: string, order: LaunchpadSwapOrder): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadsetfeestrategy</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export const sendLaunchpadSetFeeStrategy = (</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadwithapproval</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export async function sendLaunchpadWithApproval(</code> |
| <code>interface.9.ts-sdk-launchpad-waitlaunchpadapproval</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/launchpad.ts</code> | <code>export async function waitLaunchpadApproval(rpc: Eip1193Requester, hash: string, attempts = 60, intervalMs = 1_000): Promise&lt;void&gt;</code> |

## mirror

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-mirror-mirrorcandidate</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/mirror.ts</code> | <code>export interface MirrorCandidate</code> |
| <code>interface.9.ts-sdk-mirror-mirrorlag</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/mirror.ts</code> | <code>export type MirrorLag =</code> |
| <code>interface.9.ts-sdk-mirror-mirrorobservation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/mirror.ts</code> | <code>export interface MirrorObservation</code> |
| <code>interface.9.ts-sdk-mirror-mirrorprovenance</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/mirror.ts</code> | <code>export type MirrorProvenance = "canonical" &#124; "reorged";</code> |
| <code>interface.9.ts-sdk-mirror-mirrorreadpolicy</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/mirror.ts</code> | <code>export type MirrorReadPolicy =</code> |
| <code>interface.9.ts-sdk-mirror-mirrortarget</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/mirror.ts</code> | <code>export interface MirrorTarget</code> |

## native-capabilities

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-native-capabilities-decodenativecapabilityset</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts</code> | <code>export async function decodeNativeCapabilitySet(bytes: Uint8Array): Promise&lt;readonly NativeCapability[]&gt;</code> |
| <code>interface.9.ts-sdk-native-capabilities-derivenativeprogramaccount</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts</code> | <code>export async function deriveNativeProgramAccount(owner: Uint8Array, seed: Uint8Array): Promise&lt;Uint8Array&gt;</code> |
| <code>interface.9.ts-sdk-native-capabilities-encodenativecapabilityset</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts</code> | <code>export async function encodeNativeCapabilitySet(grants: readonly NativeCapability[]): Promise&lt;Uint8Array&gt;</code> |
| <code>interface.9.ts-sdk-native-capabilities-max-native-balance-views</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts</code> | <code>export const MAX_NATIVE_BALANCE_VIEWS = 32;</code> |
| <code>interface.9.ts-sdk-native-capabilities-max-native-capabilities</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts</code> | <code>export const MAX_NATIVE_CAPABILITIES = 238;</code> |
| <code>interface.9.ts-sdk-native-capabilities-max-native-capability-bytes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts</code> | <code>export const MAX_NATIVE_CAPABILITY_BYTES = 65_452;</code> |
| <code>interface.9.ts-sdk-native-capabilities-narrownativecapabilityset</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts</code> | <code>export async function narrowNativeCapabilitySet(parent: readonly NativeCapability[], requested: readonly NativeCapability[]): Promise&lt;readonly NativeCapability[]&gt;</code> |
| <code>interface.9.ts-sdk-native-capabilities-nativecapability</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts</code> | <code>export type NativeCapability =</code> |

## native-effect

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-native-effect-encodenativeeffectpreparerequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-effect.ts</code> | <code>export function encodeNativeEffectPrepareRequest(value: NativeEffectPrepareRequestV1): NativeEffectPrepareRequestV1</code> |
| <code>interface.9.ts-sdk-native-effect-nativeeffectpreparerequestdigest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-effect.ts</code> | <code>export function nativeEffectPrepareRequestDigest(value: NativeEffectPrepareRequestV1): string</code> |
| <code>interface.9.ts-sdk-native-effect-nativeeffectpreparerequestv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-effect.ts</code> | <code>export interface NativeEffectPrepareRequestV1 extends Omit&lt;NativePrepareRequestV1, "variant"&gt;</code> |

## native-program-call

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-native-program-call-decodenativeprogramcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts</code> | <code>export function decodeNativeProgramCall(payload: Uint8Array): NativeProgramCall</code> |
| <code>interface.9.ts-sdk-native-program-call-decodenativeprogramcallv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts</code> | <code>export function decodeNativeProgramCallV1(payload: Uint8Array): NativeProgramCallV1</code> |
| <code>interface.9.ts-sdk-native-program-call-encodenativeprogramcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts</code> | <code>export function encodeNativeProgramCall(call: NativeProgramCall): Uint8Array</code> |
| <code>interface.9.ts-sdk-native-program-call-encodenativeprogramcallv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts</code> | <code>export function encodeNativeProgramCallV1(call: NativeProgramCallV1): Uint8Array</code> |
| <code>interface.9.ts-sdk-native-program-call-nativeprogramcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts</code> | <code>export interface NativeProgramCall</code> |
| <code>interface.9.ts-sdk-native-program-call-nativeprogramcallv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts</code> | <code>export type NativeProgramCallV1 = Omit&lt;NativeProgramCall, "guestAbi"&gt; &amp; Readonly&lt;{ guestAbi: 1 &#124; 2 &#124; 3 &#124; 4 &#124; 5 }&gt;;</code> |

## node-mirror

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-node-mirror-decodemirrorprocessresult</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/node-mirror.ts</code> | <code>export function decodeMirrorProcessResult(code:number&#124;null,signal:NodeJS.Signals&#124;null,output:string,batchNumber:bigint):ExecutableMirrorVerification</code> |
| <code>interface.9.ts-sdk-node-mirror-executablemirrorerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/node-mirror.ts</code> | <code>export class ExecutableMirrorError extends Error{public constructor(public readonly code:string){super(`mirror verification refused: ${code}`);this.name="ExecutableMirrorError";}}</code> |
| <code>interface.9.ts-sdk-node-mirror-executablemirrorverification</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/node-mirror.ts</code> | <code>export interface ExecutableMirrorVerification {readonly level:string;readonly batchNumber:bigint;readonly headerDigest:Uint8Array;readonly evidenceDigest:Uint8Array;readonly sourceId:string;readonly target:string;readonly canonicalPosition:string;readonly provenance:"Canonical"&#124;"Reorged";readonly la</code> |
| <code>interface.9.ts-sdk-node-mirror-localmirrorexecutableverifier</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/node-mirror.ts</code> | <code>export class LocalMirrorExecutableVerifier</code> |

## perps

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-perps-decodeperpsactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function decodePerpsActivity(</code> |
| <code>interface.9.ts-sdk-perps-decodeperpsevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function decodePerpsEvent(log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-perps-encodeperpsactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function encodePerpsActivity(activity: PerpsActivity): Uint8Array</code> |
| <code>interface.9.ts-sdk-perps-perps-activity-types</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export const PERPS_ACTIVITY_TYPES =</code> |
| <code>interface.9.ts-sdk-perps-perps-module-id</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export const PERPS_MODULE_ID = 6;</code> |
| <code>interface.9.ts-sdk-perps-perpsactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export type PerpsActivity =</code> |
| <code>interface.9.ts-sdk-perps-perpsactivityname</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export type PerpsActivityName = keyof typeof PERPS_ACTIVITY_TYPES;</code> |
| <code>interface.9.ts-sdk-perps-perpsactivitytype</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function perpsActivityType(activity: PerpsActivity): number</code> |
| <code>interface.9.ts-sdk-perps-perpscall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function perpsCall(method: PerpsWrite, args: readonly AbiArgument[]): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpscancelordercall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function perpsCancelOrderCall(orderId: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpsclosepositioncall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function perpsClosePositionCall(positionId: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpsdepositcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function perpsDepositCall(token: string, amount: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpsmarket</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export interface PerpsMarket</code> |
| <code>interface.9.ts-sdk-perps-perpsopenposition</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export interface PerpsOpenPosition</code> |
| <code>interface.9.ts-sdk-perps-perpsopenpositioncall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function perpsOpenPositionCall(order: PerpsOpenPosition): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpswithdrawcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export function perpsWithdrawCall(token: string, amount: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpswrite</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export type PerpsWrite = keyof typeof PERPS_WRITES;</code> |
| <code>interface.9.ts-sdk-perps-tradeside</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export type TradeSide = 1 &#124; 2;</code> |
| <code>interface.9.ts-sdk-perps-tradingid</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export type TradingId = string;</code> |
| <code>interface.9.ts-sdk-perps-tradingpayloaderror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export class TradingPayloadError extends Error</code> |
| <code>interface.9.ts-sdk-perps-tradingpayloaderrorcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export type TradingPayloadErrorCode =</code> |
| <code>interface.9.ts-sdk-perps-tradingreader</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export class TradingReader</code> |
| <code>interface.9.ts-sdk-perps-tradingwriter</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/perps.ts</code> | <code>export class TradingWriter</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9363</code> | perpsOpenPositionCall encodes the v2 openPosition selector with the account id. |
| <code>req.9363</code> | perpsCancelOrderCall encodes cancelOrder(bytes32,bytes32). |

## production

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-production-agent-operations</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export const AGENT_OPERATIONS = [</code> |
| <code>interface.9.ts-sdk-production-calloptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export interface CallOptions</code> |
| <code>interface.9.ts-sdk-production-human-operations</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export const HUMAN_OPERATIONS = [</code> |
| <code>interface.9.ts-sdk-production-humanoperation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export type HumanOperation = (typeof HUMAN_OPERATIONS)[number];</code> |
| <code>interface.9.ts-sdk-production-idempotencykey</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export type IdempotencyKey = string &amp; { readonly [idempotencyKeyBrand]: true };</code> |
| <code>interface.9.ts-sdk-production-idempotencykey-2</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export function idempotencyKey(value: string): IdempotencyKey</code> |
| <code>interface.9.ts-sdk-production-platform-sdk-typescript</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export function platform_sdk_typescript(): typeof PACKAGE_METADATA</code> |
| <code>interface.9.ts-sdk-production-platformplane</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export type PlatformPlane = "agent" &#124; "human";</code> |
| <code>interface.9.ts-sdk-production-platformsdkerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export class PlatformSdkError extends Error</code> |
| <code>interface.9.ts-sdk-production-productionclient</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export class ProductionClient</code> |
| <code>interface.9.ts-sdk-production-productiontransport</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export interface ProductionTransport</code> |
| <code>interface.9.ts-sdk-production-protocolamount</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export type ProtocolAmount = bigint &amp; { readonly [protocolAmountBrand]: true };</code> |
| <code>interface.9.ts-sdk-production-protocolamount-2</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export function protocolAmount(value: bigint &#124; string): ProtocolAmount</code> |
| <code>interface.9.ts-sdk-production-retryclass</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export type RetryClass = "never" &#124; "safe" &#124; "after" &#124; "unknown-outcome";</code> |
| <code>interface.9.ts-sdk-production-safeerrordetails</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export interface SafeErrorDetails</code> |
| <code>interface.9.ts-sdk-production-sdk-error-codes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export const SDK_ERROR_CODES = [</code> |
| <code>interface.9.ts-sdk-production-sdkerrorcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export type SdkErrorCode = (typeof SDK_ERROR_CODES)[number];</code> |
| <code>interface.9.ts-sdk-production-sdktelemetry</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export type SdkTelemetry = (event: SdkTelemetryEvent) =&gt; void;</code> |
| <code>interface.9.ts-sdk-production-sdktelemetryevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export interface SdkTelemetryEvent</code> |
| <code>interface.9.ts-sdk-production-secretbytes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export class SecretBytes</code> |
| <code>interface.9.ts-sdk-production-transportcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/production.ts</code> | <code>export interface TransportCall&lt;TRequest&gt;</code> |

## program-lifecycle

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-program-lifecycle-decodenativeprogramdeploy</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export function decodeNativeProgramDeploy(payload: Uint8Array): NativeProgramDeploy</code> |
| <code>interface.9.ts-sdk-program-lifecycle-decodenativeprogramupgrade</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export function decodeNativeProgramUpgrade(payload: Uint8Array): NativeProgramUpgrade</code> |
| <code>interface.9.ts-sdk-program-lifecycle-decodenativeprogramwinddown</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export function decodeNativeProgramWindDown(payload: Uint8Array): NativeProgramWindDown</code> |
| <code>interface.9.ts-sdk-program-lifecycle-encodenativeprogramdeploy</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export function encodeNativeProgramDeploy(value: NativeProgramDeploy): Uint8Array</code> |
| <code>interface.9.ts-sdk-program-lifecycle-encodenativeprogramupgrade</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export function encodeNativeProgramUpgrade(value: NativeProgramUpgrade): Uint8Array</code> |
| <code>interface.9.ts-sdk-program-lifecycle-encodenativeprogramwinddown</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export function encodeNativeProgramWindDown(value: NativeProgramWindDown): Uint8Array</code> |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramdeploy</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export interface NativeProgramDeploy</code> |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramlifecyclerequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export class NativeProgramLifecycleRequest</code> |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramupgrade</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export interface NativeProgramUpgrade</code> |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramwinddown</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export type NativeProgramWindDown = Readonly&lt;{ programId: Uint8Array }&gt; &amp; (</code> |
| <code>interface.9.ts-sdk-program-lifecycle-programlifecyclesubmission</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts</code> | <code>export type ProgramLifecycleSubmission = Readonly&lt;{ state: "acknowledged"; activity_id: string; receipt: string; result_code: number }&gt;</code> |

## program-wire

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-program-wire-assertfreshsimulationobservation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-wire.ts</code> | <code>export function assertFreshSimulationObservation(</code> |
| <code>interface.9.ts-sdk-program-wire-bindretainedprogramcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-wire.ts</code> | <code>export async function bindRetainedProgramCall(</code> |
| <code>interface.9.ts-sdk-program-wire-bindsignedprogramlifecycle</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-wire.ts</code> | <code>export async function bindSignedProgramLifecycle(canonical: Uint8Array, payload: Uint8Array &#124; undefined, ordinal: 1 &#124; 2 &#124; 7, expectedIdempotencyKey?: string): Promise&lt;DecodedSignedProgramCall&gt;</code> |
| <code>interface.9.ts-sdk-program-wire-decodedprogramterminal</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-wire.ts</code> | <code>export interface DecodedProgramTerminal</code> |
| <code>interface.9.ts-sdk-program-wire-decodedsignedprogramcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-wire.ts</code> | <code>export interface DecodedSignedProgramCall</code> |
| <code>interface.9.ts-sdk-program-wire-decodesignedprogramcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-wire.ts</code> | <code>export async function decodeSignedProgramCall(</code> |
| <code>interface.9.ts-sdk-program-wire-occupancypayer</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/program-wire.ts</code> | <code>export interface OccupancyPayer</code> |

## programs

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-programs-gatewayprogramdiscovery</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface GatewayProgramDiscovery extends ProgramDiscovery</code> |
| <code>interface.9.ts-sdk-programs-nativeprogramrequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export class NativeProgramRequest implements ProgramCall</code> |
| <code>interface.9.ts-sdk-programs-nativeprogramrequestv1</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export class NativeProgramRequestV1 implements ProgramCall</code> |
| <code>interface.9.ts-sdk-programs-parseprogramexecutiondocument</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export function parseProgramExecutionDocument(value: unknown): ProgramExecutionDocument</code> |
| <code>interface.9.ts-sdk-programs-parseprogramexecutiondocumentv5</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export function parseProgramExecutionDocumentV5(value: unknown): ProgramExecutionDocument</code> |
| <code>interface.9.ts-sdk-programs-platform-sdk-programs</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export function platform_sdk_programs(): string { return "receipt-verified-program-operations-v1"; }</code> |
| <code>interface.9.ts-sdk-programs-programauthoritydocument</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramAuthorityDocument { readonly batch_id: string; readonly asset: string; readonly previous_state_root: string; readonly resulting_state_root: string; readonly sequencer_public_key: string }</code> |
| <code>interface.9.ts-sdk-programs-programbudget</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramBudget { readonly fuel: bigint; readonly feeLimit: bigint }</code> |
| <code>interface.9.ts-sdk-programs-programcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramCall</code> |
| <code>interface.9.ts-sdk-programs-programcapability</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export type ProgramCapability = keyof typeof CAPABILITY_ORDER;</code> |
| <code>interface.9.ts-sdk-programs-programdiscovery</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramDiscovery { readonly program_id: string; readonly lifecycle: "active" &#124; "deprecated" &#124; "tombstoned"; readonly version: number; readonly code_hash: string; readonly abi_version: number; readonly receipt_digest: string; readonly state_root: string; readonly observed_sequence: s</code> |
| <code>interface.9.ts-sdk-programs-programexecutiondocument</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramExecutionDocument { readonly state: "executed" &#124; "refused" &#124; "simulated"; readonly activity_id: string; readonly program_id: string; readonly guest_abi_version: number; readonly module_version: number; readonly batch_id: string; readonly global_sequence: string; readonly resu</code> |
| <code>interface.9.ts-sdk-programs-programfailure</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export type ProgramFailure = Readonly&lt;{ kind: "unknown_program" &#124; "reentrancy" &#124; "depth_exceeded" &#124; "fanout_exceeded" &#124; "guest_refused" &#124; "authority" &#124; "resource" &#124; "response" &#124; "fault"; code?: number; limit?: number; attempted?: number }&gt;;</code> |
| <code>interface.9.ts-sdk-programs-programinterface</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramInterface { readonly program_id: string; readonly version: number; readonly code_hash: string; readonly abi_version: number; readonly interface: string; readonly interface_digest: string; readonly receipt_digest: string; readonly state_root: string; readonly observed_sequence</code> |
| <code>interface.9.ts-sdk-programs-programoperations</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export class ProgramOperations</code> |
| <code>interface.9.ts-sdk-programs-programoutcome</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export type ProgramOutcome = Readonly&lt;{ kind: "completed"; code: number; response: string }&gt; &#124; Readonly&lt;{ kind: "legacy_completed"; code: number; values: readonly unknown[] }&gt; &#124; Readonly&lt;{ kind: "refused"; failure: ProgramFailure }&gt;;</code> |
| <code>interface.9.ts-sdk-programs-programsimulation</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramSimulation { readonly committed: false; readonly execution: ProgramExecutionDocument &amp; Readonly&lt;{ state: "simulated" }&gt;; readonly simulation_evidence: ProgramSimulationEvidence }</code> |
| <code>interface.9.ts-sdk-programs-programsimulationevidence</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramSimulationEvidence { readonly boundary_id: string; readonly activity_id: string; readonly previous_state_root: string; readonly hypothetical_state_root: string; readonly observed_sequence: string; readonly observed_at: string; readonly committed: false; readonly public_key: s</code> |
| <code>interface.9.ts-sdk-programs-programsource</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export type ProgramSource = Readonly&lt;{ status: "unpublished" }&gt; &#124; Readonly&lt;{ status: "verified"; source_digest: string; environment_digest: string; pipeline: "sha256-source-artifact-reproducible-build-v1" }&gt; &#124; Readonly&lt;{ status: "mismatch"; expected_code_hash: string; reproduced_artifact_digest: str</code> |
| <code>interface.9.ts-sdk-programs-programsubmission</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export type ProgramSubmission = ProgramUnknownSubmission &#124; (ProgramExecutionDocument &amp; Readonly&lt;{ state: "executed" &#124; "refused" }&gt;);</code> |
| <code>interface.9.ts-sdk-programs-programtrustcontext</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export class ProgramTrustContext</code> |
| <code>interface.9.ts-sdk-programs-programunknownsubmission</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramUnknownSubmission { readonly state: "unknown"; readonly activity_id: string; readonly idempotency_key: string; readonly retained_signed_activity?: string }</code> |
| <code>interface.9.ts-sdk-programs-programusage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export interface ProgramUsage { readonly cpu_fuel: string; readonly memory_bytes: string; readonly storage_read_bytes: string; readonly storage_write_bytes: string; readonly output_values: number; readonly output_bytes: string; readonly fee_units: string }</code> |
| <code>interface.9.ts-sdk-programs-resolvelifecyclefailure</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export function resolveLifecycleFailure(error: unknown, binding: DecodedSignedProgramCall): ProgramLifecycleSubmission</code> |
| <code>interface.9.ts-sdk-programs-verifygatewayprogramdiscovery</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export async function verifyGatewayProgramDiscovery(value: unknown, programId: string, trust: ProgramTrustContext): Promise&lt;GatewayProgramDiscovery&gt;</code> |
| <code>interface.9.ts-sdk-programs-verifygatewayprograminterface</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/programs.ts</code> | <code>export async function verifyGatewayProgramInterface(value: unknown, programId: string, trust: ProgramTrustContext,</code> |

## px

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-px-decodepxaccountbalances</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function decodePxAccountBalances(value: unknown): PxAccountBalances</code> |
| <code>interface.9.ts-sdk-px-decodepxaccountjoin</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function decodePxAccountJoin(value: unknown): PxAccountJoin</code> |
| <code>interface.9.ts-sdk-px-decodepxassetbalance</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function decodePxAssetBalance(value: unknown): PxAssetBalance</code> |
| <code>interface.9.ts-sdk-px-decodepxassettable</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function decodePxAssetTable(value: unknown): PxAssetTable</code> |
| <code>interface.9.ts-sdk-px-decodepxcustodyasset</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function decodePxCustodyAsset(value: unknown): PxCustodyAsset &#124; null</code> |
| <code>interface.9.ts-sdk-px-decodepxnetworkhead</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function decodePxNetworkHead(value: unknown): PxNetworkHead</code> |
| <code>interface.9.ts-sdk-px-decodepxreceiptkeys</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function decodePxReceiptKeys(</code> |
| <code>interface.9.ts-sdk-px-decodepxresolvedidentities</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function decodePxResolvedIdentities(</code> |
| <code>interface.9.ts-sdk-px-px-maximum-joined-assets</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export const PX_MAXIMUM_JOINED_ASSETS = 1024;</code> |
| <code>interface.9.ts-sdk-px-px-methods</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export const PX_METHODS = [</code> |
| <code>interface.9.ts-sdk-px-pxaccountbalances</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxAccountBalances</code> |
| <code>interface.9.ts-sdk-px-pxaccountjoin</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxAccountJoin</code> |
| <code>interface.9.ts-sdk-px-pxaccountkey</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function pxAccountKey(account: string): string</code> |
| <code>interface.9.ts-sdk-px-pxassetbalance</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxAssetBalance</code> |
| <code>interface.9.ts-sdk-px-pxassetentry</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxAssetEntry</code> |
| <code>interface.9.ts-sdk-px-pxassettable</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxAssetTable</code> |
| <code>interface.9.ts-sdk-px-pxclient</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export class PxClient</code> |
| <code>interface.9.ts-sdk-px-pxclientoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxClientOptions</code> |
| <code>interface.9.ts-sdk-px-pxcustodyasset</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxCustodyAsset</code> |
| <code>interface.9.ts-sdk-px-pxmethod</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export type PxMethod = (typeof PX_METHODS)[number];</code> |
| <code>interface.9.ts-sdk-px-pxnetworkhead</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxNetworkHead</code> |
| <code>interface.9.ts-sdk-px-pxoptionalquantity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function pxOptionalQuantity(value: unknown): bigint &#124; null</code> |
| <code>interface.9.ts-sdk-px-pxpaxeeraccount</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxPaxeerAccount</code> |
| <code>interface.9.ts-sdk-px-pxpaxeerassetbalance</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxPaxeerAssetBalance</code> |
| <code>interface.9.ts-sdk-px-pxpaxeerhead</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxPaxeerHead</code> |
| <code>interface.9.ts-sdk-px-pxquantity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export function pxQuantity(value: unknown): bigint</code> |
| <code>interface.9.ts-sdk-px-pxreceiptkeyentry</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxReceiptKeyEntry</code> |
| <code>interface.9.ts-sdk-px-pxresolvedidentities</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/px.ts</code> | <code>export interface PxResolvedIdentities</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9355</code> | decodePxNetworkHead returns latestCommittedHeight and refuses an anchor field. |
| <code>req.9355</code> | PX_METHODS lists exactly the eight px methods. |

## revert

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-revert-decoderevertreason</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/revert.ts</code> | <code>export function decodeRevertReason(returnData: string &#124; Uint8Array): RevertReasonError</code> |
| <code>interface.9.ts-sdk-revert-lxp-retryable</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/revert.ts</code> | <code>export const LXP_RETRYABLE: ReadonlySet&lt;string&gt; = new Set([</code> |
| <code>interface.9.ts-sdk-revert-revertreasonerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/revert.ts</code> | <code>export class RevertReasonError extends Error</code> |

## rpc

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-rpc-assetlistsnapshot</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export interface AssetListSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-assetmetadata</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export interface AssetMetadata</code> |
| <code>interface.9.ts-sdk-rpc-assetsnapshot</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export interface AssetSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-commitment</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export type Commitment = "submitted" &#124; "receipted" &#124; "committed";</code> |
| <code>interface.9.ts-sdk-rpc-decodeassetlistsnapshot</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export function decodeAssetListSnapshot(value: Record&lt;string, unknown&gt;): AssetListSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-decodeassetsnapshot</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export function decodeAssetSnapshot(value: Record&lt;string, unknown&gt;): AssetSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-decodeidentitysequencesnapshot</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export function decodeIdentitySequenceSnapshot(value: Record&lt;string, unknown&gt;): IdentitySequenceSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-decodejsonrpcresponse</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export function decodeJsonRpcResponse(value: unknown, id: string): Record&lt;string, unknown&gt;</code> |
| <code>interface.9.ts-sdk-rpc-identitysequencesnapshot</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export interface IdentitySequenceSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-jsonrpcclient</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export class JsonRpcClient</code> |
| <code>interface.9.ts-sdk-rpc-jsonrpcerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export class JsonRpcError extends Error</code> |
| <code>interface.9.ts-sdk-rpc-list-assets-default-page</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export const LIST_ASSETS_DEFAULT_PAGE = 64;</code> |
| <code>interface.9.ts-sdk-rpc-list-assets-max-page</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export const LIST_ASSETS_MAX_PAGE = 256;</code> |
| <code>interface.9.ts-sdk-rpc-listassetsparams</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export function listAssetsParams(cursor?: string, limit?: number): readonly RpcParam[]</code> |
| <code>interface.9.ts-sdk-rpc-rpcparam</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export type RpcParam = string &#124; number &#124; null;</code> |
| <code>interface.9.ts-sdk-rpc-walletaccount</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc.ts</code> | <code>export function walletAccount(did: string, asset: string, nativeAsset: string): string</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9352</code> | Every JsonRpcClient read sends an eth_ or px_ method. |
| <code>req.9352</code> | sendRawTransaction sends the 0x-prefixed raw bytes and returns the hash. |
| <code>req.9352</code> | getIdentitySequence returns the pending transaction count. |

## rpc-subscription

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-rpc-subscription-subscriberpc</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export async function* subscribeRpc(endpoint: URL, authorization: string &#124; undefined, requestId: string, topic: SubscriptionTopic, account?: string, cursor?: bigint, signal?: AbortSignal): AsyncGenerator&lt;SubscriptionEvent&gt;</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionacknowledgement</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export function subscriptionAcknowledgement(value: unknown, requestId: string): string</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptioncontinuity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export class SubscriptionContinuity</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptioncursor</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export function subscriptionCursor(value: unknown): bigint</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export interface SubscriptionEvent</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionnotification</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export function subscriptionNotification(value: unknown, subscription: string): SubscriptionEvent</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionselector</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export function subscriptionSelector</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptiontopic</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export type SubscriptionTopic = "heads" &#124; "account";</code> |
| <code>interface.9.ts-sdk-rpc-subscription-unsubscribeacknowledgement</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts</code> | <code>export function unsubscribeAcknowledgement(value: unknown, requestId: string): true</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9354</code> | subscribeRpc opens the stream with eth_subscribe newHeads. |
| <code>req.9354</code> | The account topic subscribes to logs filtered by the account address. |
| <code>req.9354</code> | A newHeads notification that skips a height ends the stream with an error. |

## spot

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-spot-decodespotactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export function decodeSpotActivity(</code> |
| <code>interface.9.ts-sdk-spot-encodespotactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export function encodeSpotActivity(activity: SpotActivity): Uint8Array</code> |
| <code>interface.9.ts-sdk-spot-spot-activity-types</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export const SPOT_ACTIVITY_TYPES =</code> |
| <code>interface.9.ts-sdk-spot-spot-module-id</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export const SPOT_MODULE_ID = 10;</code> |
| <code>interface.9.ts-sdk-spot-spotactivity</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export type SpotActivity =</code> |
| <code>interface.9.ts-sdk-spot-spotactivitytype</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export function spotActivityType(activity: SpotActivity): number</code> |
| <code>interface.9.ts-sdk-spot-spotcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export function spotCall&lt;C extends SpotContract&gt;(</code> |
| <code>interface.9.ts-sdk-spot-spotcontract</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export type SpotContract = keyof typeof SPOT_PRECOMPILES;</code> |
| <code>interface.9.ts-sdk-spot-spotorderkind</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export type SpotOrderKind = 1 &#124; 2;</code> |
| <code>interface.9.ts-sdk-spot-spottimeinforce</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export type SpotTimeInForce = 1 &#124; 2;</code> |
| <code>interface.9.ts-sdk-spot-spotwrite</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/spot.ts</code> | <code>export type SpotWrite&lt;C extends SpotContract&gt; = keyof (typeof SPOT_WRITES)[C] &amp; string;</code> |

## stream

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-stream-resumablestream</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/stream.ts</code> | <code>export class ResumableStream&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-stream-streamcursor</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/stream.ts</code> | <code>export type StreamCursor = string &amp; { readonly [streamCursorBrand]: true };</code> |
| <code>interface.9.ts-sdk-stream-streamcursor-2</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/stream.ts</code> | <code>export function streamCursor(value: string): StreamCursor</code> |
| <code>interface.9.ts-sdk-stream-streamevent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/stream.ts</code> | <code>export interface StreamEvent&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-stream-streampage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/stream.ts</code> | <code>export interface StreamPage&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-stream-streampagesource</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/stream.ts</code> | <code>export type StreamPageSource&lt;T&gt; = (</code> |

## verifier

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-verifier-default-protocol-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export const DEFAULT_PROTOCOL_VERSION = 2;</code> |
| <code>interface.9.ts-sdk-verifier-isselectableprotocolversion</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export function isSelectableProtocolVersion(version: number): version is SelectableProtocolVersion</code> |
| <code>interface.9.ts-sdk-verifier-legacy-protocol-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export const LEGACY_PROTOCOL_VERSION = 1;</code> |
| <code>interface.9.ts-sdk-verifier-protocolreceipt</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export interface ProtocolReceipt</code> |
| <code>interface.9.ts-sdk-verifier-protocolselection</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export interface ProtocolSelection</code> |
| <code>interface.9.ts-sdk-verifier-protocolversionusesoccupancy</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export function protocolVersionUsesOccupancy(version: number): boolean</code> |
| <code>interface.9.ts-sdk-verifier-receipteffect</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export interface ReceiptEffect</code> |
| <code>interface.9.ts-sdk-verifier-receiptverification</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export interface ReceiptVerification</code> |
| <code>interface.9.ts-sdk-verifier-receiptverificationerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export class ReceiptVerificationError extends PlatformSdkError</code> |
| <code>interface.9.ts-sdk-verifier-selectableprotocolversion</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export type SelectableProtocolVersion = typeof DEFAULT_PROTOCOL_VERSION &#124; typeof STATE_COMMITMENT_PROTOCOL_VERSION;</code> |
| <code>interface.9.ts-sdk-verifier-selectedprotocolversion</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export function selectedProtocolVersion(selection?: ProtocolSelection): SelectableProtocolVersion</code> |
| <code>interface.9.ts-sdk-verifier-sequencerauthorization</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export interface SequencerAuthorization</code> |
| <code>interface.9.ts-sdk-verifier-state-commitment-protocol-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export const STATE_COMMITMENT_PROTOCOL_VERSION = 3;</code> |
| <code>interface.9.ts-sdk-verifier-supportedprogramguestabi</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/verifier.ts</code> | <code>export function supportedProgramGuestAbi(abiVersion: number): boolean</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9372</code> | verifyReceipt marks a receipt verified under a caller-supplied key as submitted. |

## wallet

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-wallet-encodenativeregistration</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/wallet.ts</code> | <code>export function encodeNativeRegistration(registration:NativeTokenRegistration,actor:string):Uint8Array</code> |
| <code>interface.9.ts-sdk-wallet-nativetokenregistration</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/wallet.ts</code> | <code>export interface NativeTokenRegistration</code> |
| <code>interface.9.ts-sdk-wallet-verifiedwalletreceipt</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/wallet.ts</code> | <code>export interface VerifiedWalletReceipt</code> |
| <code>interface.9.ts-sdk-wallet-walletexecutionoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/wallet.ts</code> | <code>export interface WalletExecutionOptions</code> |
| <code>interface.9.ts-sdk-wallet-walletpendingerror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/wallet.ts</code> | <code>export class WalletPendingError extends Error { public constructor(public readonly activityId:string){super("Activity remains pending");} }</code> |
| <code>interface.9.ts-sdk-wallet-walletrpc</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/wallet.ts</code> | <code>export class WalletRpc</code> |
| <code>interface.9.ts-sdk-wallet-walletrpcconfiguration</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/wallet.ts</code> | <code>export interface WalletRpcConfiguration</code> |

## web-search

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-web-search-contentdigest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export function contentDigest(canonical: Uint8Array): string</code> |
| <code>interface.9.ts-sdk-web-search-decodepayergrant</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export function decodePayerGrant(canonical: Uint8Array): PayerGrant</code> |
| <code>interface.9.ts-sdk-web-search-decodewebcontent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export function decodeWebContent(canonical: Uint8Array): WebContent</code> |
| <code>interface.9.ts-sdk-web-search-web-content-domain</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export const WEB_CONTENT_DOMAIN = "PAXEERX_WEB_CONTENT_V1";</code> |
| <code>interface.9.ts-sdk-web-search-web-content-fetch</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export const WEB_CONTENT_FETCH = 1;</code> |
| <code>interface.9.ts-sdk-web-search-web-content-search</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export const WEB_CONTENT_SEARCH = 2;</code> |
| <code>interface.9.ts-sdk-web-search-web-search-currencies</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export const WEB_SEARCH_CURRENCIES = ["SID", "PAX", "USDC", "USDL"] as const;</code> |
| <code>interface.9.ts-sdk-web-search-web-search-max-results</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export const WEB_SEARCH_MAX_RESULTS = 10;</code> |
| <code>interface.9.ts-sdk-web-search-web-search-payer-header</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export const WEB_SEARCH_PAYER_HEADER = "LAYERX-PAYER-ADDRESS";</code> |
| <code>interface.9.ts-sdk-web-search-webcontent</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebContent</code> |
| <code>interface.9.ts-sdk-web-search-webcontentbytes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export function webContentBytes(content: { readonly kind: WebContentKind; readonly payload: Uint8Array &#124; string; readonly mediaType: string; readonly text: string }): Uint8Array</code> |
| <code>interface.9.ts-sdk-web-search-webcontentkind</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export type WebContentKind = typeof WEB_CONTENT_FETCH &#124; typeof WEB_CONTENT_SEARCH;</code> |
| <code>interface.9.ts-sdk-web-search-webcontentresponse</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebContentResponse</code> |
| <code>interface.9.ts-sdk-web-search-webfetchresponse</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebFetchResponse</code> |
| <code>interface.9.ts-sdk-web-search-websearchassetterms</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchAssetTerms</code> |
| <code>interface.9.ts-sdk-web-search-websearchclient</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export class WebSearchClient</code> |
| <code>interface.9.ts-sdk-web-search-websearchclientoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchClientOptions</code> |
| <code>interface.9.ts-sdk-web-search-websearchcurrency</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export type WebSearchCurrency = typeof WEB_SEARCH_CURRENCIES[number];</code> |
| <code>interface.9.ts-sdk-web-search-websearcherror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export class WebSearchError extends Error</code> |
| <code>interface.9.ts-sdk-web-search-websearchexactpayment</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchExactPayment</code> |
| <code>interface.9.ts-sdk-web-search-websearchmeteredpayment</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchMeteredPayment</code> |
| <code>interface.9.ts-sdk-web-search-websearchoffer</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchOffer</code> |
| <code>interface.9.ts-sdk-web-search-websearchpayer</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchPayer</code> |
| <code>interface.9.ts-sdk-web-search-websearchpreference</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchPreference</code> |
| <code>interface.9.ts-sdk-web-search-websearchresponse</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchResponse</code> |
| <code>interface.9.ts-sdk-web-search-websearchresult</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSearchResult</code> |
| <code>interface.9.ts-sdk-web-search-websearchscheme</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export type WebSearchScheme = "metered" &#124; "exact";</code> |
| <code>interface.9.ts-sdk-web-search-websettlement</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/web-search.ts</code> | <code>export interface WebSettlement</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9373</code> | WebSettlement.transaction is the draw transaction hash. |
| <code>req.9373</code> | The client refuses a settlement receipt signed by an unpinned key. |

## x402-activity

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-x402-activity-binddrawtransaction-j44</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/activity.ts</code> | <code>export function bindDrawTransaction</code> |

## x402-draw

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-x402-draw-preparedgrantdraws</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/draw.ts</code> | <code>export class PreparedGrantDraws</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9375</code> | PreparedGrantDraws.execute submits the registered raw draw transaction once. |
| <code>req.9375</code> | validateGrantDraw refuses draw calldata whose amount exceeds the offer. |

## x402-grant

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-x402-grant-grantoffer</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/grant.ts</code> | <code>export interface GrantOffer</code> |
| <code>interface.9.ts-sdk-x402-grant-validategrantdraw</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/grant.ts</code> | <code>export function validateGrantDraw</code> |

## x402-receive

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-x402-receive-assetregistration</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export interface AssetRegistration</code> |
| <code>interface.9.ts-sdk-x402-receive-decodereceive</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function decodeReceive(bytes: Uint8Array): Receive</code> |
| <code>interface.9.ts-sdk-x402-receive-derivenativeassetid</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function deriveNativeAssetId(issuerDidId32: string, salt: string): string</code> |
| <code>interface.9.ts-sdk-x402-receive-encodeaccountopen</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function encodeAccountOpen(asset: string): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodeassetregister</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function encodeAssetRegister(issuerDidId32: string, registration: AssetRegistration): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodeassetsupply</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function encodeAssetSupply(asset: string, account: string, amount: string): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodegrant</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function encodeGrant(grant: PayerGrant): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodegrantrevoke</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function encodeGrantRevoke(grant_id: string, revocation_sequence: string): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodereceive</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function encodeReceive(receive: Receive): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-grantauthorizationmessage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function grantAuthorizationMessage(grant: PayerGrant): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-payergrant</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export interface PayerGrant</code> |
| <code>interface.9.ts-sdk-x402-receive-receive</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export interface Receive</code> |
| <code>interface.9.ts-sdk-x402-receive-receiveauthorizationmessage</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export function receiveAuthorizationMessage(receive: Receive): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-receiverauthorization</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts</code> | <code>export interface ReceiverAuthorization</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9374</code> | issueGrantCall wraps the encoded payer grant in issueGrant(bytes). |
| <code>req.9374</code> | openAccountCall and registerAssetCall encode the layerxledger selectors. |
| <code>req.9374</code> | bindDrawTransaction accepts the matching draw and refuses a changed amount. |

## xweb-api

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.ts-sdk-xweb-api-buildxwebapirequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function buildXWebApiRequest(</code> |
| <code>interface.9.ts-sdk-xweb-api-decodexwebapipayload</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function decodeXWebApiPayload(raw: Uint8Array): XWebApiPayload</code> |
| <code>interface.9.ts-sdk-xweb-api-decodexwebattestors</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function decodeXWebAttestors(answer: string): XWebAttestorSet</code> |
| <code>interface.9.ts-sdk-xweb-api-encodexwebapipayload</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function encodeXWebApiPayload(payload: XWebApiPayload): Uint8Array</code> |
| <code>interface.9.ts-sdk-xweb-api-encodexwebcredential</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function encodeXWebCredential(headers: readonly XWebApiHeader[], publicHeaders: readonly XWebApiHeader[] = []): Uint8Array</code> |
| <code>interface.9.ts-sdk-xweb-api-sealxwebenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function sealXWebEnvelope(</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-api-limits</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_API_LIMITS =</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-api-version</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_API_VERSION = 1;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-default-max-payload-bytes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_DEFAULT_MAX_PAYLOAD_BYTES = 8192;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-info</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_ENVELOPE_INFO = "PAXEERX_WEB_API_ENVELOPE_V1";</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-key-length</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_ENVELOPE_KEY_LENGTH = 33;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-nonce-length</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_ENVELOPE_NONCE_LENGTH = 12;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-overhead</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_ENVELOPE_OVERHEAD = 20 + XWEB_ENVELOPE_KEY_LENGTH + XWEB_ENVELOPE_NONCE_LENGTH + XWEB_ENVELOPE_TAG_LENGTH;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-tag-length</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_ENVELOPE_TAG_LENGTH = 16;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-kind-api</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_KIND_API = 3;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-level-majority</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_LEVEL_MAJORITY = 0;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-level-single</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_LEVEL_SINGLE = 1;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-max-envelope-bytes</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_MAX_ENVELOPE_BYTES = XWEB_ENVELOPE_OVERHEAD + XWEB_API_LIMITS.credentialBytes;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-method-get</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_METHOD_GET = 1;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-method-post</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_METHOD_POST = 2;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-precompile</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export const XWEB_PRECOMPILE = "0x0000000000000000000000000000000000001019";</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapibuildoptions</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebApiBuildOptions</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapicall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebApiCall</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapierror</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export class XWebApiError extends Error</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapierrorcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export type XWebApiErrorCode =</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapiheader</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebApiHeader</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapimethod</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export type XWebApiMethod = "GET" &#124; "POST";</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapiorigin</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function xwebApiOrigin(url: string): string</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapipayload</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebApiPayload</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapirequest</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebApiRequest</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapirequestcall</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function xwebApiRequestCall(payload: Uint8Array, callbackGas: bigint, fee: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebattestor</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebAttestor</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebattestoraddress</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function xwebAttestorAddress(publicKey: Uint8Array &#124; string): string</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebattestorset</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebAttestorSet</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebenveloperandomness</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebEnvelopeRandomness</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebgetattestorscalldata</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function xwebGetAttestorsCallData(): string</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebmethodcode</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export function xwebMethodCode(method: XWebApiMethod): number</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebsealedenvelope</code> | <code>ts_export</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts</code> | <code>export interface XWebSealedEnvelope</code> |
