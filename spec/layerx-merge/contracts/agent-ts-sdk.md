# Agent ts sdk contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## account-derivation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-account-derivation-account-derivation-domain-name</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:25</code> | <code>export const ACCOUNT_DERIVATION_DOMAIN_NAME = "Paxeer X Network";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-domain-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:26</code> | <code>export const ACCOUNT_DERIVATION_DOMAIN_VERSION = "1";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-hkdf-info-prefix</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:30</code> | <code>export const ACCOUNT_DERIVATION_HKDF_INFO_PREFIX = "LX:ACCOUNT-KEY:v1";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-hkdf-salt</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:29</code> | <code>export const ACCOUNT_DERIVATION_HKDF_SALT = "paxeer-x-network/layerx-account-key/v1";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-origin</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:24</code> | <code>export const ACCOUNT_DERIVATION_ORIGIN = "&lt;origin URL&gt;";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-purpose</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:27</code> | <code>export const ACCOUNT_DERIVATION_PURPOSE = "Derive your LayerX account key";</code> |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:28</code> | <code>export const ACCOUNT_DERIVATION_VERSION = 1;</code> |
| <code>interface.9.ts-sdk-account-derivation-accountderivationerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:61</code> | <code>export class AccountDerivationError extends Error</code> |
| <code>interface.9.ts-sdk-account-derivation-accountderivationerrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:45</code> | <code>export type AccountDerivationErrorCode =</code> |
| <code>interface.9.ts-sdk-account-derivation-addr-precompile-address</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:31</code> | <code>export const ADDR_PRECOMPILE_ADDRESS = "0x0000000000000000000000000000000000001004";</code> |
| <code>interface.9.ts-sdk-account-derivation-autobindlayerx</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:568</code> | <code>export async function autoBindLayerX(</code> |
| <code>interface.9.ts-sdk-account-derivation-bindcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:417</code> | <code>export interface BindCall</code> |
| <code>interface.9.ts-sdk-account-derivation-bindfees</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:586</code> | <code>export interface BindFees</code> |
| <code>interface.9.ts-sdk-account-derivation-bindnoncecall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:432</code> | <code>export function bindNonceCall(address: string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-bindplan</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:425</code> | <code>export type BindPlan = Readonly&lt;{ action: "already_bound" }&gt; &#124; Readonly&lt;{ action: "bind"; call: BindCall }&gt;;</code> |
| <code>interface.9.ts-sdk-account-derivation-bindtransactionrequest</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:560</code> | <code>export function bindTransactionRequest(derived: DerivedAccount, call: BindCall): WalletRequest</code> |
| <code>interface.9.ts-sdk-account-derivation-bounddidcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:437</code> | <code>export function boundDidCall(address: string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-checksumaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:146</code> | <code>export function checksumAddress(address: Uint8Array &#124; string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-decodebindnonce</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:462</code> | <code>export function decodeBindNonce(answer: string): bigint</code> |
| <code>interface.9.ts-sdk-account-derivation-decodebounddid</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:470</code> | <code>export function decodeBoundDid(answer: string): string &#124; null</code> |
| <code>interface.9.ts-sdk-account-derivation-decodelayerxdid</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:445</code> | <code>export function decodeLayerXDid(answer: string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-derivedaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:72</code> | <code>export interface DerivedAccount</code> |
| <code>interface.9.ts-sdk-account-derivation-derivefrombrowserwallet</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:387</code> | <code>export async function deriveFromBrowserWallet(</code> |
| <code>interface.9.ts-sdk-account-derivation-derivefrommnemonic</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:203</code> | <code>export function deriveFromMnemonic(</code> |
| <code>interface.9.ts-sdk-account-derivation-derivefromseed</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:185</code> | <code>export function deriveFromSeed(seed: Uint8Array, index = 0): DerivedAccount</code> |
| <code>interface.9.ts-sdk-account-derivation-derivefromwalletsignature</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:334</code> | <code>export function deriveFromWalletSignature(</code> |
| <code>interface.9.ts-sdk-account-derivation-eip1193requester</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:366</code> | <code>export interface Eip1193Requester</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationhash</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:270</code> | <code>export function keyDerivationHash(parameters: KeyDerivationParameters): Uint8Array</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationparameters</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:214</code> | <code>export interface KeyDerivationParameters</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:376</code> | <code>export function keyDerivationRequest(parameters: KeyDerivationParameters): WalletRequest</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationtypeddata</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:222</code> | <code>export interface KeyDerivationTypedData</code> |
| <code>interface.9.ts-sdk-account-derivation-keyderivationtypeddata-2</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:233</code> | <code>export function keyDerivationTypedData(parameters: KeyDerivationParameters): KeyDerivationTypedData</code> |
| <code>interface.9.ts-sdk-account-derivation-layerx-coin-type</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:23</code> | <code>export const LAYERX_COIN_TYPE = 19544;</code> |
| <code>interface.9.ts-sdk-account-derivation-layerxbindmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:522</code> | <code>export function layerXBindMessage(chainId: number &#124; bigint, address: string, nonce: bigint): Uint8Array</code> |
| <code>interface.9.ts-sdk-account-derivation-layerxbindstate</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:411</code> | <code>export interface LayerXBindState</code> |
| <code>interface.9.ts-sdk-account-derivation-layerxdidcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:441</code> | <code>export function layerXDidCall(address: string): string</code> |
| <code>interface.9.ts-sdk-account-derivation-normalizewalletsignature</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:306</code> | <code>export function normalizeWalletSignature(signature: Uint8Array &#124; string): Uint8Array</code> |
| <code>interface.9.ts-sdk-account-derivation-planlayerxbind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:534</code> | <code>export function planLayerXBind(derived: DerivedAccount, chainId: number &#124; bigint, state: LayerXBindState): BindPlan</code> |
| <code>interface.9.ts-sdk-account-derivation-readlayerxbindstate</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:480</code> | <code>export async function readLayerXBindState(wallet: Eip1193Requester, address: string): Promise&lt;LayerXBindState&gt;</code> |
| <code>interface.9.ts-sdk-account-derivation-requirelayerxbinding</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:499</code> | <code>export function requireLayerXBinding(derived: Pick&lt;DerivedAccount, "layerxPublicKey"&gt;, state: LayerXBindState): void</code> |
| <code>interface.9.ts-sdk-account-derivation-sendboundlayerxcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:508</code> | <code>export async function sendBoundLayerXCall(</code> |
| <code>interface.9.ts-sdk-account-derivation-signbindtransaction</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:630</code> | <code>export function signBindTransaction(</code> |
| <code>interface.9.ts-sdk-account-derivation-slip10ed25519</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:172</code> | <code>export function slip10Ed25519(seed: Uint8Array, path: readonly number[]): Uint8Array</code> |
| <code>interface.9.ts-sdk-account-derivation-walletrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/account-derivation.ts:370</code> | <code>export interface WalletRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-account-derivation-account-derivation-domain-name</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-domain-version</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-hkdf-info-prefix</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-hkdf-salt</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-origin</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-purpose</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-account-derivation-version</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-accountderivationerror</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-accountderivationerrorcode</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-addr-precompile-address</code> | notes | Named by stale pending task 10.5.13. |
| <code>interface.9.ts-sdk-account-derivation-autobindlayerx</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-bindcall</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-bindfees</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-bindnoncecall</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-bindplan</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-bindtransactionrequest</code> | notes | Named by stale pending task 10.5.13. |
| <code>interface.9.ts-sdk-account-derivation-bounddidcall</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-checksumaddress</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-decodebindnonce</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-decodebounddid</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-decodelayerxdid</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-derivedaccount</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-derivefrombrowserwallet</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-derivefrommnemonic</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-derivefromseed</code> | notes | Named by stale pending task 10.5.13. |
| <code>interface.9.ts-sdk-account-derivation-derivefromwalletsignature</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-eip1193requester</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-keyderivationhash</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-keyderivationparameters</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-keyderivationrequest</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-keyderivationtypeddata</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-keyderivationtypeddata-2</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-layerx-coin-type</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-layerxbindmessage</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-layerxbindstate</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-layerxdidcall</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-normalizewalletsignature</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-planlayerxbind</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-readlayerxbindstate</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-requirelayerxbinding</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-sendboundlayerxcall</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-account-derivation-signbindtransaction</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-slip10ed25519</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. |
| <code>interface.9.ts-sdk-account-derivation-walletrequest</code> | notes | File is touched by stale pending task 10.5.13; the export itself is not named by them. No caller in package tests, examples or consuming packages. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.357.1</code> | req | 9357.1 |
| <code>acceptance.9.357.1</code> | scenario | Given an account derived from the BIP-39 test mnemonic and its bind call, when the test builds the request with nonce 3, then params[0] has type 0x2, chainId 0x7d, nonce 0x3 and to ADDR_PRECOMPILE_ADDRESS. |
| <code>acceptance.9.357.1</code> | test | services/agent/sdk/typescript/test/account-bind-eip1559.test.ts::bind_transaction_request_is_a_type_2_transaction_on_chain_125 (planned) |
| <code>acceptance.9.357.1</code> | qualification | UNRUN |
| <code>req.9357</code> | ac_1 | bindTransactionRequest returns a type 2 request on chain 125 to the addr precompile. |
| <code>acceptance.9.357.2</code> | req | 9357.2 |
| <code>acceptance.9.357.2</code> | scenario | Given the same account and call, when the test signs it with bindTransaction and recovers the sender with recoverEip1559Sender, then the sender equals derived.evmAddress. |
| <code>acceptance.9.357.2</code> | test | services/agent/sdk/typescript/test/account-bind-eip1559.test.ts::derived_DID_key_signs_only_the_bind_message (planned) |
| <code>acceptance.9.357.2</code> | qualification | UNRUN |
| <code>req.9357</code> | ac_2 | bindTransaction signs the bind call with the derived EVM key. |
| <code>capability.9.33.7</code> | title | Send the LayerX bind call as a type 2 transaction from the derived EVM key |
| <code>capability.9.33.7</code> | reqs | 9357.1<br>9357.2 |

## agent-http

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-agent-http-agent-daemon-rpc-path</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:378</code> | <code>export const AGENT_DAEMON_RPC_PATH = "/rpc" as const;</code> |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-operation-names</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:389</code> | <code>export const AGENT_ENVELOPE_OPERATION_NAMES: readonly AgentOperation[] = Object.freeze&lt;AgentOperation[]&gt;([</code> |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-path</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:377</code> | <code>export const AGENT_ENVELOPE_PATH = "/v1/agent/rpc" as const;</code> |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-version</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:376</code> | <code>export const AGENT_ENVELOPE_VERSION = 1 as const;</code> |
| <code>interface.9.ts-sdk-agent-http-agentenvelopesuccess</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:465</code> | <code>export interface AgentEnvelopeSuccess&lt;TValue = unknown&gt;</code> |
| <code>interface.9.ts-sdk-agent-http-agentenvelopesurface</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:379</code> | <code>export type AgentEnvelopeSurface = "gateway" &#124; "daemon";</code> |
| <code>interface.9.ts-sdk-agent-http-agentenvelopetransport</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:621</code> | <code>export class AgentEnvelopeTransport implements ProductionTransport</code> |
| <code>interface.9.ts-sdk-agent-http-agentenvelopetransportoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:445</code> | <code>export interface AgentEnvelopeTransportOptions</code> |
| <code>interface.9.ts-sdk-agent-http-agenthttptransport</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:92</code> | <code>export class AgentHttpTransport implements ProductionTransport</code> |
| <code>interface.9.ts-sdk-agent-http-agenthttptransportoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:84</code> | <code>export interface AgentHttpTransportOptions</code> |
| <code>interface.9.ts-sdk-agent-http-agentsessioncredential</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:409</code> | <code>export class AgentSessionCredential</code> |
| <code>interface.9.ts-sdk-agent-http-checkproofbundleresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:583</code> | <code>export function checkProofBundleResponse(request: ProofBundleRequest, response: AgentEnvelopeSuccess): ProofBundleRecord</code> |
| <code>interface.9.ts-sdk-agent-http-checktenantreadinessresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:502</code> | <code>export function checkTenantReadinessResponse(response: AgentEnvelopeSuccess): TenantReadinessV1</code> |
| <code>interface.9.ts-sdk-agent-http-decodeagentenveloperesponse</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:890</code> | <code>export function decodeAgentEnvelopeResponse(status: number, encoded: Buffer, mutation: boolean, sentRequestId: string): AgentEnvelopeSuccess</code> |
| <code>interface.9.ts-sdk-agent-http-decodeenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:261</code> | <code>export function decodeEnvelope(status: number, encoded: Buffer, operation: AgentOperation): unknown</code> |
| <code>interface.9.ts-sdk-agent-http-decodeprogramboundaryerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:353</code> | <code>export function decodeProgramBoundaryError(status: number, value: unknown): ProgramBoundaryError</code> |
| <code>interface.9.ts-sdk-agent-http-decodeproofbundlerecord</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:557</code> | <code>export function decodeProofBundleRecord(value: unknown): ProofBundleRecord</code> |
| <code>interface.9.ts-sdk-agent-http-decodetenantreadiness</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:488</code> | <code>export function decodeTenantReadiness(value: unknown): TenantReadinessV1</code> |
| <code>interface.9.ts-sdk-agent-http-encodeagentenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:819</code> | <code>export function encodeAgentEnvelope(</code> |
| <code>interface.9.ts-sdk-agent-http-encodeprogrammutationbody</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:194</code> | <code>export function encodeProgramMutationBody(signed: unknown): Buffer</code> |
| <code>interface.9.ts-sdk-agent-http-encodeproofbundlerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:542</code> | <code>export function encodeProofBundleRequest(value: unknown): ProofBundleRequest</code> |
| <code>interface.9.ts-sdk-agent-http-layerxkeycredential</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:62</code> | <code>export class LayerXKeyCredential</code> |
| <code>interface.9.ts-sdk-agent-http-programboundaryerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:346</code> | <code>export class ProgramBoundaryError extends PlatformSdkError</code> |
| <code>interface.9.ts-sdk-agent-http-proofbundleread</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:518</code> | <code>export interface ProofBundleRead</code> |
| <code>interface.9.ts-sdk-agent-http-proofbundlerecord</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:530</code> | <code>export type ProofBundleRecord =</code> |
| <code>interface.9.ts-sdk-agent-http-proofbundlerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:513</code> | <code>export interface ProofBundleRequest</code> |
| <code>interface.9.ts-sdk-agent-http-proofbundleverificationlevel</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:511</code> | <code>export type ProofBundleVerificationLevel = "Unverified" &#124; "SequencerSigned" &#124; "BatchIncluded" &#124; "StateProven" &#124; "CheckpointFinalised" &#124; "SettlementAnchored";</code> |
| <code>interface.9.ts-sdk-agent-http-tenantreadinessv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:475</code> | <code>export interface TenantReadinessV1</code> |
| <code>interface.9.ts-sdk-agent-http-tenantrecoveryreasonv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/agent-http.ts:471</code> | <code>export type TenantRecoveryReasonV1 = "recovery_pending" &#124; "store_unavailable" &#124; "store_refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-agent-http-agent-daemon-rpc-path</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-operation-names</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-path</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-agent-envelope-version</code> | notes | Named by stale pending task 10.5.39, 6.5.30. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-agentenvelopesuccess</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-agentenvelopesurface</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-agentenvelopetransport</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-agentenvelopetransportoptions</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-agenthttptransport</code> | notes | Named by stale pending task 6.5.30. |
| <code>interface.9.ts-sdk-agent-http-agenthttptransportoptions</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-agentsessioncredential</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-checkproofbundleresponse</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-checktenantreadinessresponse</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-decodeagentenveloperesponse</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-decodeenvelope</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-decodeprogramboundaryerror</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-decodeproofbundlerecord</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-decodetenantreadiness</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-encodeagentenvelope</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-encodeprogrammutationbody</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-encodeproofbundlerequest</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-layerxkeycredential</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-agent-http-programboundaryerror</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-proofbundleread</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-proofbundlerecord</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-proofbundlerequest</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-proofbundleverificationlevel</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-tenantreadinessv1</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-agent-http-tenantrecoveryreasonv1</code> | notes | File is touched by stale pending task 10.5.39, 11.5.53, 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.359.1</code> | req | 9359.1 |
| <code>acceptance.9.359.1</code> | scenario | Given a loopback server recording request bodies, when the test calls one operation through the envelope transport, then the body carries contract_major 2. |
| <code>acceptance.9.359.1</code> | test | services/agent/sdk/typescript/test/agent-http-major2.test.ts::agent_envelope_requests_carry_contract_major_2 (planned) |
| <code>acceptance.9.359.1</code> | qualification | UNRUN |
| <code>req.9359</code> | ac_1 | Every envelope request carries contract major 2. |
| <code>acceptance.9.359.2</code> | req | 9359.2 |
| <code>acceptance.9.359.2</code> | scenario | Given the server answering a proof bundle with verification receipted, when the test reads it, then the level is Rung.Receipted and a reply with CheckpointFinalised is refused. |
| <code>acceptance.9.359.2</code> | test | services/agent/sdk/typescript/test/agent-http-major2.test.ts::proof_bundle_levels_are_rungs (planned) |
| <code>acceptance.9.359.2</code> | qualification | UNRUN |
| <code>req.9359</code> | ac_2 | A proof bundle reply reports its verification as a rung. |
| <code>capability.9.33.9</code> | title | Send contract major 2 on agent envelope requests and report proof rungs |
| <code>capability.9.33.9</code> | reqs | 9359.1<br>9359.2 |

## ai-market

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-ai-market-activityidof</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2084</code> | <code>export function activityIdOf(signed: Uint8Array): Hex32</code> |
| <code>interface.9.ts-sdk-ai-market-ai-application-errors</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:57</code> | <code>export const AI_APPLICATION_ERRORS = Object.freeze(</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:183</code> | <code>export class AiMarketError extends Error</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketerrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:147</code> | <code>export type AiMarketErrorCode =</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketerrordetail</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:172</code> | <code>export interface AiMarketErrorDetail</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketviews</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1341</code> | <code>export class AiMarketViews</code> |
| <code>interface.9.ts-sdk-ai-market-aimarketviewsoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1334</code> | <code>export interface AiMarketViewsOptions</code> |
| <code>interface.9.ts-sdk-ai-market-approval</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2587</code> | <code>export class Approval</code> |
| <code>interface.9.ts-sdk-ai-market-approvalterms</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2435</code> | <code>export interface ApprovalTerms extends BindFees</code> |
| <code>interface.9.ts-sdk-ai-market-availability</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:712</code> | <code>export type Availability =</code> |
| <code>interface.9.ts-sdk-ai-market-callboundary</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:215</code> | <code>export type CallBoundary = "mutation" &#124; "program-read";</code> |
| <code>interface.9.ts-sdk-ai-market-claim</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:459</code> | <code>export const CLAIM = 0x0602;</code> |
| <code>interface.9.ts-sdk-ai-market-claimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1707</code> | <code>export interface ClaimRequest</code> |
| <code>interface.9.ts-sdk-ai-market-components</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:745</code> | <code>export type Components = Readonly&lt;Record&lt;FeatureName, Availability&gt;&gt;;</code> |
| <code>interface.9.ts-sdk-ai-market-decodeclaimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1758</code> | <code>export function decodeClaimRequest(bytes: Uint8Array): ClaimRequest</code> |
| <code>interface.9.ts-sdk-ai-market-decodedenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1558</code> | <code>export interface DecodedEnvelope</code> |
| <code>interface.9.ts-sdk-ai-market-decodeepochpage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1207</code> | <code>export function decodeEpochPage(</code> |
| <code>interface.9.ts-sdk-ai-market-decodefundrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1747</code> | <code>export function decodeFundRequest(bytes: Uint8Array): FundRequest</code> |
| <code>interface.9.ts-sdk-ai-market-decodemarketview</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1057</code> | <code>export function decodeMarketView(value: unknown, market: Hex32): MarketView</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramcallactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1826</code> | <code>export function decodeProgramCallActivity(</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramscallresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:3158</code> | <code>export function decodeProgramsCallResult(</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:3222</code> | <code>export function decodeProgramsReceipt(</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsrpcrevert</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1282</code> | <code>export function decodeProgramsRpcRevert(</code> |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsstatehead</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1309</code> | <code>export function decodeProgramsStateHead(</code> |
| <code>interface.9.ts-sdk-ai-market-decoderefundrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1768</code> | <code>export function decodeRefundRequest(bytes: Uint8Array): RefundRequest</code> |
| <code>interface.9.ts-sdk-ai-market-decoderequestenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1634</code> | <code>export function decodeRequestEnvelope(input: Uint8Array): DecodedEnvelope</code> |
| <code>interface.9.ts-sdk-ai-market-domainhash</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:479</code> | <code>export function domainHash(domain: string, bytes: Uint8Array): Hex32</code> |
| <code>interface.9.ts-sdk-ai-market-domainstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:145</code> | <code>export type DomainStatus = "pending" &#124; "completed";</code> |
| <code>interface.9.ts-sdk-ai-market-encodeclaimrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1726</code> | <code>export function encodeClaimRequest(value: ClaimRequest): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-encodefundrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1718</code> | <code>export function encodeFundRequest(value: FundRequest): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-encodeprogramcallactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1798</code> | <code>export function encodeProgramCallActivity(</code> |
| <code>interface.9.ts-sdk-ai-market-encodeprogramsstateheadcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1300</code> | <code>export function encodeProgramsStateHeadCall(program: Hex32): PrecompileCall</code> |
| <code>interface.9.ts-sdk-ai-market-encoderefundrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1733</code> | <code>export function encodeRefundRequest(value: RefundRequest): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-encoderequestenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1610</code> | <code>export function encodeRequestEnvelope(envelope: RequestEnvelope): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-encodesnapshotbinding</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:659</code> | <code>export function encodeSnapshotBinding(binding: SnapshotBinding): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-entrypoint</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:53</code> | <code>export const ENTRYPOINT = "layerx_call";</code> |
| <code>interface.9.ts-sdk-ai-market-epochentry</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:842</code> | <code>export interface EpochEntry</code> |
| <code>interface.9.ts-sdk-ai-market-epochpage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:847</code> | <code>export interface EpochPage</code> |
| <code>interface.9.ts-sdk-ai-market-epochstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:725</code> | <code>export type EpochStatus =</code> |
| <code>interface.9.ts-sdk-ai-market-featurename</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:734</code> | <code>export type FeatureName =</code> |
| <code>interface.9.ts-sdk-ai-market-freshness</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:674</code> | <code>export type Freshness =</code> |
| <code>interface.9.ts-sdk-ai-market-freshnessof</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:680</code> | <code>export function freshnessOf(</code> |
| <code>interface.9.ts-sdk-ai-market-fund</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:458</code> | <code>export const FUND = 0x0601;</code> |
| <code>interface.9.ts-sdk-ai-market-funding-policy-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:54</code> | <code>export const FUNDING_POLICY_VERSION = 1n;</code> |
| <code>interface.9.ts-sdk-ai-market-fundrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1701</code> | <code>export interface FundRequest</code> |
| <code>interface.9.ts-sdk-ai-market-guest-abi</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:52</code> | <code>export const GUEST_ABI: number = 5;</code> |
| <code>interface.9.ts-sdk-ai-market-hex32</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:94</code> | <code>export type Hex32 = string;</code> |
| <code>interface.9.ts-sdk-ai-market-kindfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:733</code> | <code>export type KindFilter = "all" &#124; ParticipantKind;</code> |
| <code>interface.9.ts-sdk-ai-market-marketview</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:792</code> | <code>export interface MarketView</code> |
| <code>interface.9.ts-sdk-ai-market-native-call-protocol-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:49</code> | <code>export const NATIVE_CALL_PROTOCOL_VERSION = 2;</code> |
| <code>interface.9.ts-sdk-ai-market-nativeterms</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2105</code> | <code>export interface NativeTerms extends Omit&lt;BindFees, "evmNonce"&gt;</code> |
| <code>interface.9.ts-sdk-ai-market-operationeffect</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2121</code> | <code>export type OperationEffect =</code> |
| <code>interface.9.ts-sdk-ai-market-operationjournal</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2933</code> | <code>export class OperationJournal</code> |
| <code>interface.9.ts-sdk-ai-market-operationrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2706</code> | <code>export class OperationRecord implements RecordFields</code> |
| <code>interface.9.ts-sdk-ai-market-operationrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2091</code> | <code>export interface OperationRequest</code> |
| <code>interface.9.ts-sdk-ai-market-operationsigner</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2581</code> | <code>export interface OperationSigner</code> |
| <code>interface.9.ts-sdk-ai-market-operationspec</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:216</code> | <code>export interface OperationSpec</code> |
| <code>interface.9.ts-sdk-ai-market-operationstate</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:124</code> | <code>export type OperationState =</code> |
| <code>interface.9.ts-sdk-ai-market-parsedecimalu128</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:492</code> | <code>export function parseDecimalU128(text: string): bigint</code> |
| <code>interface.9.ts-sdk-ai-market-parsedecimalu64</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:488</code> | <code>export function parseDecimalU64(text: string): bigint</code> |
| <code>interface.9.ts-sdk-ai-market-participantcursor</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:863</code> | <code>export class ParticipantCursor</code> |
| <code>interface.9.ts-sdk-ai-market-participantkind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:732</code> | <code>export type ParticipantKind = "worker" &#124; "evaluator";</code> |
| <code>interface.9.ts-sdk-ai-market-participantpage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:834</code> | <code>export interface ParticipantPage</code> |
| <code>interface.9.ts-sdk-ai-market-participantquery</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:826</code> | <code>export interface ParticipantQuery</code> |
| <code>interface.9.ts-sdk-ai-market-participantrow</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:801</code> | <code>export interface ParticipantRow</code> |
| <code>interface.9.ts-sdk-ai-market-paxai-limits</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:36</code> | <code>export const PAXAI_LIMITS = Object.freeze(</code> |
| <code>interface.9.ts-sdk-ai-market-paxai-operations</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:431</code> | <code>export const PAXAI_OPERATIONS: ReadonlyMap&lt;number, OperationSpec&gt; = new Map(</code> |
| <code>interface.9.ts-sdk-ai-market-preparedoperation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2317</code> | <code>export class PreparedOperation</code> |
| <code>interface.9.ts-sdk-ai-market-prepareoperation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2217</code> | <code>export function prepareOperation(</code> |
| <code>interface.9.ts-sdk-ai-market-program-call-ordinal</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:51</code> | <code>export const PROGRAM_CALL_ORDINAL = 3;</code> |
| <code>interface.9.ts-sdk-ai-market-programcallactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1781</code> | <code>export interface ProgramCallActivity</code> |
| <code>interface.9.ts-sdk-ai-market-programs-module</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:50</code> | <code>export const PROGRAMS_MODULE = 9;</code> |
| <code>interface.9.ts-sdk-ai-market-programs-precompile</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:83</code> | <code>export const PROGRAMS_PRECOMPILE = "0x000000000000000000000000000000000000101d";</code> |
| <code>interface.9.ts-sdk-ai-market-projectionstate</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:706</code> | <code>export type ProjectionState =</code> |
| <code>interface.9.ts-sdk-ai-market-refund-free</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:460</code> | <code>export const REFUND_FREE = 0x0604;</code> |
| <code>interface.9.ts-sdk-ai-market-refundrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1712</code> | <code>export interface RefundRequest</code> |
| <code>interface.9.ts-sdk-ai-market-requestenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1542</code> | <code>export interface RequestEnvelope</code> |
| <code>interface.9.ts-sdk-ai-market-requestintent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1685</code> | <code>export function requestIntent(unsigned: Uint8Array): Hex32</code> |
| <code>interface.9.ts-sdk-ai-market-requirecurrent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:697</code> | <code>export function requireCurrent(</code> |
| <code>interface.9.ts-sdk-ai-market-requireretainedepoch</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:1243</code> | <code>export function requireRetainedEpoch(entry: EpochEntry): Hex32</code> |
| <code>interface.9.ts-sdk-ai-market-review</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2558</code> | <code>export class Review</code> |
| <code>interface.9.ts-sdk-ai-market-reviewfield</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:96</code> | <code>export type ReviewField =</code> |
| <code>interface.9.ts-sdk-ai-market-scorestatus</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:718</code> | <code>export type ScoreStatus =</code> |
| <code>interface.9.ts-sdk-ai-market-signingpreimage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:2079</code> | <code>export function signingPreimage(unsigned: Uint8Array): Uint8Array</code> |
| <code>interface.9.ts-sdk-ai-market-snapshotbinding</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:613</code> | <code>export interface SnapshotBinding</code> |
| <code>interface.9.ts-sdk-ai-market-snapshotidof</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/ai_market.ts:670</code> | <code>export function snapshotIdOf(binding: SnapshotBinding): Hex32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-ai-market-activityidof</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-ai-application-errors</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-aimarketerror</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-aimarketerrorcode</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-aimarketerrordetail</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-aimarketviews</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-aimarketviewsoptions</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-approval</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-approvalterms</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-availability</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-callboundary</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-claim</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-claimrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-components</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-decodeclaimrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-decodedenvelope</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-decodeepochpage</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-decodefundrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-decodemarketview</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-decodeprogramcallactivity</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-decodeprogramscallresult</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsreceipt</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsrpcrevert</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-decodeprogramsstatehead</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-decoderefundrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-decoderequestenvelope</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-domainhash</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-domainstatus</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-encodeclaimrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-encodefundrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-encodeprogramcallactivity</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-encodeprogramsstateheadcall</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-encoderefundrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-encoderequestenvelope</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-encodesnapshotbinding</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-entrypoint</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-epochentry</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-epochpage</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-epochstatus</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-featurename</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-freshness</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-freshnessof</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-fund</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-funding-policy-version</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-fundrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-guest-abi</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-hex32</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-kindfilter</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-marketview</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-native-call-protocol-version</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-nativeterms</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-operationeffect</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-operationjournal</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-operationrecord</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-operationrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-operationsigner</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-operationspec</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-operationstate</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-parsedecimalu128</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-parsedecimalu64</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-participantcursor</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-participantkind</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-participantpage</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-participantquery</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-participantrow</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-paxai-limits</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-paxai-operations</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-preparedoperation</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-prepareoperation</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-program-call-ordinal</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-programcallactivity</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-programs-module</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-programs-precompile</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-projectionstate</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-refund-free</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-refundrequest</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-requestenvelope</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-requestintent</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-requirecurrent</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-requireretainedepoch</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-review</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-reviewfield</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-scorestatus</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-ai-market-signingpreimage</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-snapshotbinding</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |
| <code>interface.9.ts-sdk-ai-market-snapshotidof</code> | notes | File is touched by stale pending task 4.6.2; the export itself is not named by them. Exported under the aiMarket namespace. |

## bridge

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-bridge-bridge-events</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/bridge.ts:16</code> | <code>export const BRIDGE_EVENTS: readonly PrecompileEventSpec[] = [</code> |
| <code>interface.9.ts-sdk-bridge-bridgeinattestation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/bridge.ts:48</code> | <code>export interface BridgeInAttestation</code> |
| <code>interface.9.ts-sdk-bridge-bridgeincall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/bridge.ts:60</code> | <code>export function bridgeInCall(attestation: BridgeInAttestation): PrecompileCall</code> |
| <code>interface.9.ts-sdk-bridge-bridgeoutcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/bridge.ts:82</code> | <code>export function bridgeOutCall(chain: bigint, asset: string, amount: bigint, recipient: string): PrecompileCall</code> |
| <code>interface.9.ts-sdk-bridge-decodebridgeevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/bridge.ts:44</code> | <code>export function decodeBridgeEvent(log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-bridge-layerx-bridge-precompile</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/bridge.ts:14</code> | <code>export const LAYERX_BRIDGE_PRECOMPILE = "0x0000000000000000000000000000000000001016";</code> |
| <code>interface.9.ts-sdk-bridge-sendbridgein</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/bridge.ts:90</code> | <code>export const sendBridgeIn = (wallet: Eip1193Requester, from: string, attestation: BridgeInAttestation): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-bridge-sendbridgeout</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/bridge.ts:93</code> | <code>export const sendBridgeOut = (</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-bridge-sendbridgein</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-bridge-sendbridgeout</code> | notes | No caller in package tests, examples or consuming packages. |

## exchange

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-exchange-abiargument</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:253</code> | <code>export type AbiArgument = bigint &#124; boolean &#124; string &#124; readonly AbiArgument[];</code> |
| <code>interface.9.ts-sdk-exchange-abieventtopic</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:214</code> | <code>export function abiEventTopic(signature: string): string</code> |
| <code>interface.9.ts-sdk-exchange-abiselector</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:209</code> | <code>export function abiSelector(signature: string): string</code> |
| <code>interface.9.ts-sdk-exchange-abisignature</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:204</code> | <code>export function abiSignature(name: string, types: readonly PrecompileAbiType[]): string</code> |
| <code>interface.9.ts-sdk-exchange-decodedprecompileevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:77</code> | <code>export interface DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-exchange-decodeeventfrom</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:490</code> | <code>export function decodeEventFrom(specs: readonly PrecompileEventSpec[], log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-exchange-decodeeventwithspec</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:456</code> | <code>export function decodeEventWithSpec(spec: PrecompileEventSpec, log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-exchange-decodeexchangeevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:560</code> | <code>export function decodeExchangeEvent(log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-exchange-encodeabicall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:219</code> | <code>export function encodeAbiCall(</code> |
| <code>interface.9.ts-sdk-exchange-exchange-events</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:507</code> | <code>export const EXCHANGE_EVENTS: readonly PrecompileEventSpec[] = [</code> |
| <code>interface.9.ts-sdk-exchange-exchangecancelordercall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:584</code> | <code>export function exchangeCancelOrderCall(orderId: string): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-exchangedepositmargincall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:594</code> | <code>export function exchangeDepositMarginCall(account: string, amountWei: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-exchangedepositmargintokencall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:602</code> | <code>export function exchangeDepositMarginTokenCall(pointer: string, amount: bigint, account: string): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-exchangeplaceordercall</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/exchange.ts:569</code> | <code>export function exchangePlaceOrderCall(order:</code> |
| <code>interface.9.ts-sdk-exchange-exchangerequestsettlementcall</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/exchange.ts:589</code> | <code>export function exchangeRequestSettlementCall(positionId: string): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-exchangewithdrawmargincall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:607</code> | <code>export function exchangeWithdrawMarginCall(account: string, assetId: string, amount: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-layerx-exchange-precompile</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:14</code> | <code>export const LAYERX_EXCHANGE_PRECOMPILE = "0x0000000000000000000000000000000000001015";</code> |
| <code>interface.9.ts-sdk-exchange-precompileabierror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:25</code> | <code>export class PrecompileAbiError extends Error</code> |
| <code>interface.9.ts-sdk-exchange-precompileabierrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:16</code> | <code>export type PrecompileAbiErrorCode =</code> |
| <code>interface.9.ts-sdk-exchange-precompileabitype</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:35</code> | <code>export type PrecompileAbiType =</code> |
| <code>interface.9.ts-sdk-exchange-precompileabivalue</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:47</code> | <code>export type PrecompileAbiValue = bigint &#124; boolean &#124; string &#124; readonly string[];</code> |
| <code>interface.9.ts-sdk-exchange-precompilecall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:62</code> | <code>export interface PrecompileCall</code> |
| <code>interface.9.ts-sdk-exchange-precompileeventfields</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:75</code> | <code>export type PrecompileEventFields = Readonly&lt;Record&lt;string, bigint &#124; boolean &#124; string&gt;&gt;;</code> |
| <code>interface.9.ts-sdk-exchange-precompileeventinput</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:49</code> | <code>export interface PrecompileEventInput</code> |
| <code>interface.9.ts-sdk-exchange-precompileeventsignature</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:388</code> | <code>export function precompileEventSignature(spec: PrecompileEventSpec): string</code> |
| <code>interface.9.ts-sdk-exchange-precompileeventspec</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:55</code> | <code>export interface PrecompileEventSpec</code> |
| <code>interface.9.ts-sdk-exchange-precompilelog</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:69</code> | <code>export interface PrecompileLog</code> |
| <code>interface.9.ts-sdk-exchange-precompiletransactionrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:368</code> | <code>export function precompileTransactionRequest(from: string, call: PrecompileCall): WalletRequest</code> |
| <code>interface.9.ts-sdk-exchange-precompilewritecall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:354</code> | <code>export function precompileWriteCall(</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangecancelorder</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:617</code> | <code>export const sendExchangeCancelOrder = (wallet: Eip1193Requester, from: string, orderId: string): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangedepositmargin</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:623</code> | <code>export const sendExchangeDepositMargin = (</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangedepositmargintoken</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:630</code> | <code>export const sendExchangeDepositMarginToken = (</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangeplaceorder</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:611</code> | <code>export const sendExchangePlaceOrder = (</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangerequestsettlement</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/exchange.ts:620</code> | <code>export const sendExchangeRequestSettlement = (wallet: Eip1193Requester, from: string, positionId: string): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-exchange-sendexchangewithdrawmargin</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:638</code> | <code>export const sendExchangeWithdrawMargin = (</code> |
| <code>interface.9.ts-sdk-exchange-sendprecompilecall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/exchange.ts:379</code> | <code>export async function sendPrecompileCall(wallet: Eip1193Requester, from: string, call: PrecompileCall): Promise&lt;string&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-exchange-abiargument</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-abieventtopic</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-abiselector</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-abisignature</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-decodedprecompileevent</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-decodeeventfrom</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-decodeeventwithspec</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-decodeexchangeevent</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-encodeabicall</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-exchange-events</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-exchangecancelordercall</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-exchangedepositmargincall</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-exchangedepositmargintokencall</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-exchangeplaceordercall</code> | notes | Named by stale pending task 3.6.3. |
| <code>interface.9.ts-sdk-exchange-exchangerequestsettlementcall</code> | notes | Name carries a surface the merge design retargets; file is touched by stale pending task 10.5.9, 11.5.53, 3.6.3. |
| <code>interface.9.ts-sdk-exchange-exchangewithdrawmargincall</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-layerx-exchange-precompile</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-precompileabierror</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-precompileabierrorcode</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-precompileabitype</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-precompileabivalue</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-precompilecall</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-precompileeventfields</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-precompileeventinput</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-precompileeventsignature</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-precompileeventspec</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-precompilelog</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-precompiletransactionrequest</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-precompilewritecall</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-sendexchangecancelorder</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-sendexchangedepositmargin</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |
| <code>interface.9.ts-sdk-exchange-sendexchangedepositmargintoken</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-sendexchangeplaceorder</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-sendexchangerequestsettlement</code> | notes | Name carries a surface the merge design retargets; file is touched by stale pending task 10.5.9, 11.5.53, 3.6.3. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-sendexchangewithdrawmargin</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-exchange-sendprecompilecall</code> | notes | File is touched by stale pending task 10.5.9, 11.5.53, 3.6.3; the export itself is not named by them. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.360.1</code> | req | 9360.1 |
| <code>acceptance.9.360.1</code> | scenario | Given a market id, an account id, side 1, price 5, quantity 2 and time in force 0, when the test builds the call, then data starts with the selector of placeOrder(bytes32,bytes32,uint8,uint256,uint256,uint8) followed by the market id and the account id words. |
| <code>acceptance.9.360.1</code> | test | services/agent/sdk/typescript/test/exchange-v2.test.ts::placeOrder_call_carries_the_account_id (planned) |
| <code>acceptance.9.360.1</code> | qualification | UNRUN |
| <code>req.9360</code> | ac_1 | exchangePlaceOrderCall encodes the v2 selector with the account id. |
| <code>acceptance.9.360.2</code> | req | 9360.2 |
| <code>acceptance.9.360.2</code> | scenario | Given a recording EIP-1193 requester, when the test sends a settlement request, then the request params[0] has type 0x2, chainId 0x7d and to the exchange precompile. |
| <code>acceptance.9.360.2</code> | test | services/agent/sdk/typescript/test/exchange-v2.test.ts::exchange_wallet_requests_are_type_2_on_chain_125 (planned) |
| <code>acceptance.9.360.2</code> | qualification | UNRUN |
| <code>req.9360</code> | ac_2 | sendExchangeRequestSettlement asks the wallet for a type 2 transaction on chain 125. |
| <code>capability.9.33.10</code> | title | Encode exchange v2 orders and send exchange calls as type 2 transactions |
| <code>capability.9.33.10</code> | reqs | 9360.1<br>9360.2 |

## gas-station

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-gas-station-assembleeip7702authorization</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:428</code> | <code>export function assembleEip7702Authorization(</code> |
| <code>interface.9.ts-sdk-gas-station-discoversponsorednonce</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:1056</code> | <code>export async function discoverSponsoredNonce(</code> |
| <code>interface.9.ts-sdk-gas-station-eip7702authorization</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:85</code> | <code>export interface Eip7702Authorization</code> |
| <code>interface.9.ts-sdk-gas-station-eip7702authorizationdigest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:422</code> | <code>export function eip7702AuthorizationDigest(</code> |
| <code>interface.9.ts-sdk-gas-station-gas-station-quote-url</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:13</code> | <code>export const GAS_STATION_QUOTE_URL =</code> |
| <code>interface.9.ts-sdk-gas-station-gasquote</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:16</code> | <code>export interface GasQuote</code> |
| <code>interface.9.ts-sdk-gas-station-gasquotedigest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:258</code> | <code>export function gasQuoteDigest(</code> |
| <code>interface.9.ts-sdk-gas-station-gasquoterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:50</code> | <code>export interface GasQuoteRequest</code> |
| <code>interface.9.ts-sdk-gas-station-gasrefusal</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:76</code> | <code>export interface GasRefusal</code> |
| <code>interface.9.ts-sdk-gas-station-gasrefusalcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:59</code> | <code>export type GasRefusalCode =</code> |
| <code>interface.9.ts-sdk-gas-station-gasresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:81</code> | <code>export type GasResult&lt;T&gt; =</code> |
| <code>interface.9.ts-sdk-gas-station-gasstationconfig</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:27</code> | <code>export interface GasStationConfig</code> |
| <code>interface.9.ts-sdk-gas-station-gasstationquoteurl</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:106</code> | <code>export function gasStationQuoteUrl(</code> |
| <code>interface.9.ts-sdk-gas-station-gassubmissionidentity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:648</code> | <code>export interface GasSubmissionIdentity</code> |
| <code>interface.9.ts-sdk-gas-station-gassubmissionoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:660</code> | <code>export type GasSubmissionOutcome =</code> |
| <code>interface.9.ts-sdk-gas-station-gassubmissionstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:676</code> | <code>export interface GasSubmissionStatus</code> |
| <code>interface.9.ts-sdk-gas-station-gassubmissiontransaction</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:655</code> | <code>export interface GasSubmissionTransaction</code> |
| <code>interface.9.ts-sdk-gas-station-nativefeepreference</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:1121</code> | <code>export type NativeFeePreference =</code> |
| <code>interface.9.ts-sdk-gas-station-readnativefeepreference</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:1171</code> | <code>export async function readNativeFeePreference(</code> |
| <code>interface.9.ts-sdk-gas-station-requestgasquote</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:523</code> | <code>export async function requestGasQuote(</code> |
| <code>interface.9.ts-sdk-gas-station-requestgassubmissionstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:854</code> | <code>export function requestGasSubmissionStatus(</code> |
| <code>interface.9.ts-sdk-gas-station-retrygassubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:872</code> | <code>export function retryGasSubmission(</code> |
| <code>interface.9.ts-sdk-gas-station-sidiora-decimals</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:12</code> | <code>export const SIDIORA_DECIMALS = 6;</code> |
| <code>interface.9.ts-sdk-gas-station-sidiora-token</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:11</code> | <code>export const SIDIORA_TOKEN = "0x21f7b20a555199fa73A238B1a91FD0f549068fEe";</code> |
| <code>interface.9.ts-sdk-gas-station-signedeip7702authorization</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:91</code> | <code>export interface SignedEip7702Authorization extends Eip7702Authorization</code> |
| <code>interface.9.ts-sdk-gas-station-signedgasquote</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:44</code> | <code>export interface SignedGasQuote</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatch</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:36</code> | <code>export interface SponsoredBatch</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatchcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:347</code> | <code>export function sponsoredBatchCall(</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatchdigest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:277</code> | <code>export function sponsoredBatchDigest(batch: SponsoredBatch): GasResult&lt;string&gt;</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredconsent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:1000</code> | <code>export interface SponsoredConsent</code> |
| <code>interface.9.ts-sdk-gas-station-sponsoredconsent-2</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:1017</code> | <code>export function sponsoredConsent(</code> |
| <code>interface.9.ts-sdk-gas-station-submitsponsoredgasbatch</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/gas-station.ts:889</code> | <code>export async function submitSponsoredGasBatch(</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-gas-station-assembleeip7702authorization</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-discoversponsorednonce</code> | notes | Named by stale pending task 10.5.8. |
| <code>interface.9.ts-sdk-gas-station-eip7702authorization</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-eip7702authorizationdigest</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gas-station-quote-url</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-gasquote</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gasquotedigest</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gasquoterequest</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gasrefusal</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gasrefusalcode</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gasresult</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gasstationconfig</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gasstationquoteurl</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-gassubmissionidentity</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-gassubmissionoutcome</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-gassubmissionstatus</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-gassubmissiontransaction</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-nativefeepreference</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-readnativefeepreference</code> | notes | Named by stale pending task 10.5.8. |
| <code>interface.9.ts-sdk-gas-station-requestgasquote</code> | notes | Named by stale pending task 10.5.8. |
| <code>interface.9.ts-sdk-gas-station-requestgassubmissionstatus</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-retrygassubmission</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-sidiora-decimals</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-sidiora-token</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-signedeip7702authorization</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-gas-station-signedgasquote</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatch</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatchcall</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-sponsoredbatchdigest</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-sponsoredconsent</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-sponsoredconsent-2</code> | notes | File is touched by stale pending task 10.5.8, 6.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-gas-station-submitsponsoredgasbatch</code> | notes | Named by stale pending task 10.5.8, 6.5.41. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.361.1</code> | req | 9361.1 |
| <code>acceptance.9.361.1</code> | scenario | Given a recording request answering eth_call with 0x05, when the test discovers the nonce, then the eth_call block parameter is pending and the nonce is 5n. |
| <code>acceptance.9.361.1</code> | test | services/agent/sdk/typescript/test/gas-station-pending.test.ts::sponsored_nonce_is_read_at_the_pending_tag (planned) |
| <code>acceptance.9.361.1</code> | qualification | UNRUN |
| <code>req.9361</code> | ac_1 | discoverSponsoredNonce calls eth_call with the pending tag. |
| <code>acceptance.9.361.2</code> | req | 9361.2 |
| <code>acceptance.9.361.2</code> | scenario | Given a recording request answering eth_call to the feetoken precompile with an ABI string usid, when the test reads the preference, then the call data is getFeeDenom(address) of the account and the preference denom is usid. |
| <code>acceptance.9.361.2</code> | test | services/agent/sdk/typescript/test/gas-station-pending.test.ts::native_fee_preference_reads_feetoken_getFeeDenom (planned) |
| <code>acceptance.9.361.2</code> | qualification | UNRUN |
| <code>req.9361</code> | ac_2 | readNativeFeePreference returns the denomination from feetoken getFeeDenom. |
| <code>acceptance.9.361.3</code> | req | 9361.3 |
| <code>acceptance.9.361.3</code> | scenario | Given a fetch stub answering /submit with a transaction hash, when the test submits a signed batch, then the result carries that hash and rung submitted. |
| <code>acceptance.9.361.3</code> | test | services/agent/sdk/typescript/test/gas-station-pending.test.ts::submitted_sponsored_batch_reports_the_transaction_hash_at_the_submitted_rung (planned) |
| <code>acceptance.9.361.3</code> | qualification | UNRUN |
| <code>req.9361</code> | ac_3 | A sponsored batch accepted by the station reports its transaction hash at the submitted rung. |
| <code>capability.9.33.11</code> | title | Read sponsored nonces at pending and fee denominations from feetoken |
| <code>capability.9.33.11</code> | reqs | 9361.1<br>9361.2<br>9361.3 |

## generated-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-generated-client-amount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:13</code> | <code>export type Amount = bigint;</code> |
| <code>interface.9.ts-sdk-generated-client-apierror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:78</code> | <code>export interface ApiError</code> |
| <code>interface.9.ts-sdk-generated-client-approval-contract-introduced</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:87</code> | <code>export const APPROVAL_CONTRACT_INTRODUCED = "1.1" as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approval-decision-outcomes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:90</code> | <code>export const APPROVAL_DECISION_OUTCOMES = ["Granted", "Rejected", "Expired", "Defective", "AlreadyDecided", "Conflict"] as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approval-enforcement-notice</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:88</code> | <code>export const APPROVAL_ENFORCEMENT_NOTICE = "An approval hold is a daemon-enforced restriction. It confers no protocol authority, and bypassing the daemon bypasses the restriction." as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approval-event-kinds</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:91</code> | <code>export const APPROVAL_EVENT_KINDS = ["Created", "Granted", "Rejected", "Expired", "Defective"] as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approval-states</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:89</code> | <code>export const APPROVAL_STATES = ["Held", "Granted", "Rejected", "Expired", "Defective"] as const;</code> |
| <code>interface.9.ts-sdk-generated-client-approvalapproverequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:126</code> | <code>export interface ApprovalApproveRequest { tenant: string; approvalId: string; idempotencyKey: string }</code> |
| <code>interface.9.ts-sdk-generated-client-approvaldecision</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:128</code> | <code>export interface ApprovalDecision</code> |
| <code>interface.9.ts-sdk-generated-client-approvaldecisionoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:94</code> | <code>export type ApprovalDecisionOutcome = (typeof APPROVAL_DECISION_OUTCOMES)[number];</code> |
| <code>interface.9.ts-sdk-generated-client-approvaleventkind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:95</code> | <code>export type ApprovalEventKind = (typeof APPROVAL_EVENT_KINDS)[number];</code> |
| <code>interface.9.ts-sdk-generated-client-approvalgetrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:125</code> | <code>export interface ApprovalGetRequest { tenant: string; approvalId: string }</code> |
| <code>interface.9.ts-sdk-generated-client-approvallifecycleevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:135</code> | <code>export interface ApprovalLifecycleEvent</code> |
| <code>interface.9.ts-sdk-generated-client-approvallistrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:124</code> | <code>export interface ApprovalListRequest { tenant: string; cursor: string &#124; null; pageLimit: number }</code> |
| <code>interface.9.ts-sdk-generated-client-approvalpage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:123</code> | <code>export interface ApprovalPage { approvals: readonly ApprovalRecord[]; nextCursor: string &#124; null }</code> |
| <code>interface.9.ts-sdk-generated-client-approvalrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:111</code> | <code>export interface ApprovalRecord</code> |
| <code>interface.9.ts-sdk-generated-client-approvalrejectrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:127</code> | <code>export interface ApprovalRejectRequest { tenant: string; approvalId: string; idempotencyKey: string; reason: string }</code> |
| <code>interface.9.ts-sdk-generated-client-approvalstate</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:93</code> | <code>export type ApprovalState = (typeof APPROVAL_STATES)[number];</code> |
| <code>interface.9.ts-sdk-generated-client-budgetlimit</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:20</code> | <code>export type BudgetLimit = bigint;</code> |
| <code>interface.9.ts-sdk-generated-client-client</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:266</code> | <code>export class Client</code> |
| <code>interface.9.ts-sdk-generated-client-decodenativeapprovallistresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:255</code> | <code>export function decodeNativeApprovalListResult(value: unknown): NativeApprovalListResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-decodenativeapprovalresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:249</code> | <code>export function decodeNativeApprovalResult(value: unknown): NativeApprovalResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-decodenativeprepareresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:242</code> | <code>export function decodeNativePrepareResult(value: unknown): NativePrepareResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-encodenativeapprovaldecision</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:234</code> | <code>export function encodeNativeApprovalDecision(value: NativeApprovalDecisionV1): NativeApprovalDecisionV1</code> |
| <code>interface.9.ts-sdk-generated-client-encodenativeapprovalget</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:239</code> | <code>export function encodeNativeApprovalGet(approval_id: string): {variant:"native_v1";approval_id:string}</code> |
| <code>interface.9.ts-sdk-generated-client-encodenativepreparerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:207</code> | <code>export function encodeNativePrepareRequest(value: NativePrepareRequestV1): NativePrepareRequestV1</code> |
| <code>interface.9.ts-sdk-generated-client-errorclass</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:77</code> | <code>export type ErrorClass = "TransportFailure" &#124; "Deadline" &#124; "ProtocolIncompatibility" &#124; "UnavailableCapability" &#124; "CoreRejection" &#124; "VerificationFailure" &#124; "PolicyRefusal" &#124; "CapabilityRefusal" &#124; "BudgetRefusal" &#124; "RateLimit" &#124; "IdempotencyConflict" &#124; "InternalFault";</code> |
| <code>interface.9.ts-sdk-generated-client-holdreason</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:110</code> | <code>export interface HoldReason { code: string; message: string }</code> |
| <code>interface.9.ts-sdk-generated-client-idempotentmutation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:70</code> | <code>export interface IdempotentMutation&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-generated-client-layerx-sdk-ts-package</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:9</code> | <code>export function layerx_sdk_ts_package(): typeof PACKAGE_METADATA</code> |
| <code>interface.9.ts-sdk-generated-client-nativeactivityv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:150</code> | <code>export interface NativeActivityV1 { version: "1"; module: string; ordinal: string }</code> |
| <code>interface.9.ts-sdk-generated-client-nativeapprovaldecisionv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:169</code> | <code>export interface NativeApprovalDecisionV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativeapprovallistresultv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:177</code> | <code>export interface NativeApprovalListResultV1 { version: "1"; approvals: NativeApprovalResultV1[] }</code> |
| <code>interface.9.ts-sdk-generated-client-nativeapprovalresultv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:172</code> | <code>export interface NativeApprovalResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativelocalgrantv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:155</code> | <code>export interface NativeLocalGrantV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativepreparerequestv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:158</code> | <code>export interface NativePrepareRequestV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativeprepareresultv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:165</code> | <code>export interface NativePrepareResultV1</code> |
| <code>interface.9.ts-sdk-generated-client-nativepurposev1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:151</code> | <code>export interface NativePurposeV1</code> |
| <code>interface.9.ts-sdk-generated-client-operation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:85</code> | <code>export type Operation = "agent.register" &#124; "approval.approve" &#124; "approval.get" &#124; "approval.list" &#124; "approval.reject" &#124; "availability.fetch" &#124; "budget.create" &#124; "budget.fund" &#124; "budget.list" &#124; "budget.reconciliation" &#124; "budget.revoke" &#124; "budget.state" &#124; "capability.attenuate" &#124; "capability.create" &#124; </code> |
| <code>interface.9.ts-sdk-generated-client-parseamount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:14</code> | <code>export function parseAmount(value: string): Amount</code> |
| <code>interface.9.ts-sdk-generated-client-parsebudgetlimit</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:21</code> | <code>export function parseBudgetLimit(value: string): BudgetLimit</code> |
| <code>interface.9.ts-sdk-generated-client-parsesequence</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:28</code> | <code>export function parseSequence(value: string): Sequence</code> |
| <code>interface.9.ts-sdk-generated-client-parsetimestampseconds</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:35</code> | <code>export function parseTimestampSeconds(value: string): TimestampSeconds</code> |
| <code>interface.9.ts-sdk-generated-client-requireverified</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:60</code> | <code>export function requireVerified&lt;T&gt;(requested: Rung, read: VerifiedRead&lt;T&gt;): VerifiedRead&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-generated-client-rung</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:42</code> | <code>export enum Rung</code> |
| <code>interface.9.ts-sdk-generated-client-sequence</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:27</code> | <code>export type Sequence = bigint;</code> |
| <code>interface.9.ts-sdk-generated-client-structuredactivitydisclosure</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:97</code> | <code>export interface StructuredActivityDisclosure</code> |
| <code>interface.9.ts-sdk-generated-client-submissionstate</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:48</code> | <code>export type SubmissionState =</code> |
| <code>interface.9.ts-sdk-generated-client-timestampseconds</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:34</code> | <code>export type TimestampSeconds = bigint;</code> |
| <code>interface.9.ts-sdk-generated-client-transport</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:262</code> | <code>export interface Transport</code> |
| <code>interface.9.ts-sdk-generated-client-verifiedread</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/client.ts:54</code> | <code>export interface VerifiedRead&lt;T&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-generated-client-amount</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-apierror</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-approval-contract-introduced</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approval-decision-outcomes</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approval-enforcement-notice</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approval-event-kinds</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approval-states</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approvalapproverequest</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approvaldecision</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-approvaldecisionoutcome</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-approvaleventkind</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-approvalgetrequest</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approvallifecycleevent</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-approvallistrequest</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approvalpage</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-approvalrecord</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-approvalrejectrequest</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-approvalstate</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-budgetlimit</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-client</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-decodenativeapprovallistresult</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-decodenativeapprovalresult</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-decodenativeprepareresult</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-encodenativeapprovaldecision</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-encodenativeapprovalget</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-encodenativepreparerequest</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-errorclass</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-holdreason</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-idempotentmutation</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-layerx-sdk-ts-package</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-nativeactivityv1</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-nativeapprovaldecisionv1</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-nativeapprovallistresultv1</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-nativeapprovalresultv1</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-nativelocalgrantv1</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-nativepreparerequestv1</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-nativeprepareresultv1</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-nativepurposev1</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-operation</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-parseamount</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-parsebudgetlimit</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-parsesequence</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-parsetimestampseconds</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-requireverified</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-rung</code> | notes | Named by stale pending task 0.58, 10.5.2, 6.5.13. |
| <code>interface.9.ts-sdk-generated-client-sequence</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-structuredactivitydisclosure</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-submissionstate</code> | notes | Named by stale pending task 6.5.4. |
| <code>interface.9.ts-sdk-generated-client-timestampseconds</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-client-transport</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |
| <code>interface.9.ts-sdk-generated-client-verifiedread</code> | notes | File is touched by stale pending task 6.5.30; the export itself is not named by them. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.358.1</code> | req | 9358.1 |
| <code>acceptance.9.358.1</code> | scenario | Given the regenerated client module, when the test builds one value of each variant and switches on kind, then the switch covers exactly the four names and Rung has Submitted 0, Receipted 1 and Committed 2. |
| <code>acceptance.9.358.1</code> | test | services/agent/sdk/typescript/test/generated-rung.test.ts::generated_submission_state_names_the_rungs (planned) |
| <code>acceptance.9.358.1</code> | qualification | UNRUN |
| <code>req.9358</code> | ac_1 | The generated SubmissionState variants are Submitted, Receipted, Committed and Reverted. |
| <code>capability.9.33.8</code> | title | Regenerate the TypeScript and Python client types with rung submission states |
| <code>capability.9.33.8</code> | reqs | 9358.1 |

## generated-mirror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-generated-mirror-mirror-archive-magic</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts:3</code> | <code>export const MIRROR_ARCHIVE_MAGIC = "4c584d4952524f52" as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-archive-bytes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts:4</code> | <code>export const MIRROR_MAX_ARCHIVE_BYTES = 67_108_864 as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-json-depth</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts:6</code> | <code>export const MIRROR_MAX_JSON_DEPTH = 32 as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-sources</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts:5</code> | <code>export const MIRROR_MAX_SOURCES = 8 as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirror-schema-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts:2</code> | <code>export const MIRROR_SCHEMA_VERSION = 2 as const;</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirrorerrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts:8</code> | <code>export type MirrorErrorCode =</code> |
| <code>interface.9.ts-sdk-generated-mirror-mirrorpolicykind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/mirror.ts:7</code> | <code>export type MirrorPolicyKind = "exact" &#124; "ordered-preference" &#124; "agreement";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-generated-mirror-mirror-archive-magic</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-archive-bytes</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-json-depth</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-mirror-mirror-max-sources</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-mirror-mirror-schema-version</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-mirror-mirrorerrorcode</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-mirror-mirrorpolicykind</code> | notes | No caller in package tests, examples or consuming packages. |

## generated-precompile-writes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-generated-precompile-writes-launchpad-events</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:229</code> | <code>export const LAUNCHPAD_EVENTS = [</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-launchpad-precompile</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:176</code> | <code>export const LAUNCHPAD_PRECOMPILE = "0x0000000000000000000000000000000000001017";</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-launchpad-writes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:178</code> | <code>export const LAUNCHPAD_WRITES =</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-perps-events</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:42</code> | <code>export const PERPS_EVENTS = [</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-perps-precompile</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:12</code> | <code>export const PERPS_PRECOMPILE = "0x0000000000000000000000000000000000001015";</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-perps-writes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:14</code> | <code>export const PERPS_WRITES =</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-precompilewrite</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:6</code> | <code>export interface PrecompileWrite</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-spot-precompiles</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:76</code> | <code>export const SPOT_PRECOMPILES =</code> |
| <code>interface.9.ts-sdk-generated-precompile-writes-spot-writes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/precompile-writes.ts:84</code> | <code>export const SPOT_WRITES =</code> |

## generated-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:38</code> | <code>export const PROGRAM_ABI_V1 = 1;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v2</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:39</code> | <code>export const PROGRAM_ABI_V2 = 2;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v3</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:40</code> | <code>export const PROGRAM_ABI_V3 = 3;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v4</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:41</code> | <code>export const PROGRAM_ABI_V4 = 4;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v5</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:42</code> | <code>export const PROGRAM_ABI_V5 = 5;</code> |
| <code>interface.9.ts-sdk-generated-receipt-program-outcome-tags</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:4</code> | <code>export const PROGRAM_OUTCOME_TAGS = [0x50524731, 0x50524732, 0x50524733, 0x50524734] as const;</code> |
| <code>interface.9.ts-sdk-generated-receipt-programs-module-id</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:3</code> | <code>export const PROGRAMS_MODULE_ID = 9;</code> |
| <code>interface.9.ts-sdk-generated-receipt-receiptfailurecode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:6</code> | <code>export enum ReceiptFailureCode</code> |
| <code>interface.9.ts-sdk-generated-receipt-required-nonzero-checks</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:29</code> | <code>export const REQUIRED_NONZERO_CHECKS = Object.freeze([</code> |
| <code>interface.9.ts-sdk-generated-receipt-supportsprogramguestabi</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/generated/receipt.ts:43</code> | <code>export function supportsProgramGuestAbi(version: number): boolean</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v1</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v2</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v3</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-receipt-program-abi-v4</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-receipt-program-outcome-tags</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-receipt-programs-module-id</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-generated-receipt-required-nonzero-checks</code> | notes | No caller in package tests, examples or consuming packages. |

## history

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-history-decodehistoryassetmetadata</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:164</code> | <code>export function decodeHistoryAssetMetadata(value: unknown, asset?: string): HistoryAssetMetadata &#124; null</code> |
| <code>interface.9.ts-sdk-history-decodehistorypage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:101</code> | <code>export function decodeHistoryPage(value: unknown, pattern: RegExp, limit = HISTORY_MAXIMUM_LIMIT): HistoryPage</code> |
| <code>interface.9.ts-sdk-history-decodehistoryrow</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:131</code> | <code>export function decodeHistoryRow(value: unknown, unified: boolean): HistoryRow</code> |
| <code>interface.9.ts-sdk-history-decodepaxeerhistorypage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:110</code> | <code>export function decodePaxeerHistoryPage(value: unknown, limit?: number): HistoryPage</code> |
| <code>interface.9.ts-sdk-history-decodeunifiedhistorypage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:114</code> | <code>export function decodeUnifiedHistoryPage(value: unknown, limit = HISTORY_MAXIMUM_LIMIT): UnifiedHistoryPage</code> |
| <code>interface.9.ts-sdk-history-history-default-limit</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:4</code> | <code>export const HISTORY_DEFAULT_LIMIT = 50;</code> |
| <code>interface.9.ts-sdk-history-history-maximum-accounts</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:8</code> | <code>export const HISTORY_MAXIMUM_ACCOUNTS = 16;</code> |
| <code>interface.9.ts-sdk-history-history-maximum-limit</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:6</code> | <code>export const HISTORY_MAXIMUM_LIMIT = 100;</code> |
| <code>interface.9.ts-sdk-history-historyaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:95</code> | <code>export function historyAccount(account: string): string</code> |
| <code>interface.9.ts-sdk-history-historyassetmetadata</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:31</code> | <code>export interface HistoryAssetMetadata</code> |
| <code>interface.9.ts-sdk-history-historychain</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:20</code> | <code>export type HistoryChain = "layerx" &#124; "paxeer";</code> |
| <code>interface.9.ts-sdk-history-historydirection</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:21</code> | <code>export type HistoryDirection = "in" &#124; "out";</code> |
| <code>interface.9.ts-sdk-history-historyoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:23</code> | <code>export interface HistoryOptions</code> |
| <code>interface.9.ts-sdk-history-historypage</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/history.ts:67</code> | <code>export interface HistoryPage</code> |
| <code>interface.9.ts-sdk-history-historyparams</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/history.ts:86</code> | <code>export function historyParams(account: string, options: HistoryOptions = {}): readonly RpcParam[]</code> |
| <code>interface.9.ts-sdk-history-historyrow</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:44</code> | <code>export interface HistoryRow</code> |
| <code>interface.9.ts-sdk-history-historyside</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:73</code> | <code>export interface HistorySide</code> |
| <code>interface.9.ts-sdk-history-unifiedhistorypage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/history.ts:78</code> | <code>export interface UnifiedHistoryPage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-history-decodehistoryassetmetadata</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. |
| <code>interface.9.ts-sdk-history-decodehistorypage</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-decodehistoryrow</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-decodepaxeerhistorypage</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. |
| <code>interface.9.ts-sdk-history-decodeunifiedhistorypage</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. |
| <code>interface.9.ts-sdk-history-history-default-limit</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. |
| <code>interface.9.ts-sdk-history-history-maximum-accounts</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-history-maximum-limit</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-historyaccount</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. |
| <code>interface.9.ts-sdk-history-historyassetmetadata</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-historychain</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-historydirection</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-historyoptions</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-historypage</code> | notes | Named by stale pending task 6.6.3. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-historyparams</code> | notes | Named by stale pending task 10.5.6. |
| <code>interface.9.ts-sdk-history-historyrow</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-historyside</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-history-unifiedhistorypage</code> | notes | File is touched by stale pending task 10.5.6, 11.5.54; the export itself is not named by them. No caller in package tests, examples or consuming packages. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.353.1</code> | req | 9353.1 |
| <code>acceptance.9.353.1</code> | scenario | Given block height 7, tx index 2 and log index 1, when the test builds the cursor and passes it to historyParams, then the cursor is 0000000000000007 00000002 00000001 without spaces and params[1] equals it. |
| <code>acceptance.9.353.1</code> | test | services/agent/sdk/typescript/test/history-cursor.test.ts::history_cursor_encodes_block_height_tx_index_and_log_index (planned) |
| <code>acceptance.9.353.1</code> | qualification | UNRUN |
| <code>req.9353</code> | ac_1 | historyCursor and historyParams agree on the block position cursor. |
| <code>acceptance.9.353.2</code> | req | 9353.2 |
| <code>acceptance.9.353.2</code> | scenario | Given a page with one row carrying tx_hash and one row without it, when the test decodes each page, then the first row has txHash set and the second page throws. |
| <code>acceptance.9.353.2</code> | test | services/agent/sdk/typescript/test/history-cursor.test.ts::history_rows_carry_the_transaction_hash (planned) |
| <code>acceptance.9.353.2</code> | qualification | UNRUN |
| <code>req.9353</code> | ac_2 | decodeHistoryPage keeps each row's transaction hash and refuses a row without one. |
| <code>capability.9.33.3</code> | title | Key TypeScript history pages by block position and transaction hash |
| <code>capability.9.33.3</code> | reqs | 9353.1<br>9353.2 |

## intent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-intent-decodehumanenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:373</code> | <code>export function decodeHumanEnvelope(status: number, encoded: Buffer): unknown</code> |
| <code>interface.9.ts-sdk-intent-decodehumanerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:390</code> | <code>export function decodeHumanError(status: number, value: unknown, trace: string): HumanIntentError</code> |
| <code>interface.9.ts-sdk-intent-decodeintentplan</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:419</code> | <code>export function decodeIntentPlan(value: unknown): IntentPlan</code> |
| <code>interface.9.ts-sdk-intent-decodeintentsubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:441</code> | <code>export function decodeIntentSubmission(value: unknown): IntentSubmission</code> |
| <code>interface.9.ts-sdk-intent-encodeplanintentrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:333</code> | <code>export function encodePlanIntentRequest(request: PlanIntentRequest): Readonly&lt;Record&lt;string, unknown&gt;&gt;</code> |
| <code>interface.9.ts-sdk-intent-encodesubmitplanrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:347</code> | <code>export function encodeSubmitPlanRequest(request: SubmitPlanRequest): Readonly&lt;Record&lt;string, unknown&gt;&gt;</code> |
| <code>interface.9.ts-sdk-intent-human-error-codes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:38</code> | <code>export const HUMAN_ERROR_CODES = [</code> |
| <code>interface.9.ts-sdk-intent-human-retriabilities</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:71</code> | <code>export const HUMAN_RETRIABILITIES = ["retriable", "retriable-after", "structural", "final"] as const;</code> |
| <code>interface.9.ts-sdk-intent-humanerrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:69</code> | <code>export type HumanErrorCode = (typeof HUMAN_ERROR_CODES)[number];</code> |
| <code>interface.9.ts-sdk-intent-humanintentclient</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:242</code> | <code>export class HumanIntentClient</code> |
| <code>interface.9.ts-sdk-intent-humanintentclientoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:234</code> | <code>export interface HumanIntentClientOptions</code> |
| <code>interface.9.ts-sdk-intent-humanintenterror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:211</code> | <code>export class HumanIntentError extends PlatformSdkError</code> |
| <code>interface.9.ts-sdk-intent-humanintenterrordetails</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:200</code> | <code>export interface HumanIntentErrorDetails</code> |
| <code>interface.9.ts-sdk-intent-humanretriability</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:72</code> | <code>export type HumanRetriability = (typeof HUMAN_RETRIABILITIES)[number];</code> |
| <code>interface.9.ts-sdk-intent-intent-domains</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:35</code> | <code>export const INTENT_DOMAINS = ["paxeer", "layerx"] as const;</code> |
| <code>interface.9.ts-sdk-intent-intent-endpoint-kinds</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:32</code> | <code>export const INTENT_ENDPOINT_KINDS = ["paxeer-wallet", "human", "agent", "agent-budget"] as const;</code> |
| <code>interface.9.ts-sdk-intent-intentconstraints</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:136</code> | <code>export interface IntentConstraints</code> |
| <code>interface.9.ts-sdk-intent-intentdomain</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:36</code> | <code>export type IntentDomain = (typeof INTENT_DOMAINS)[number];</code> |
| <code>interface.9.ts-sdk-intent-intentendpointkind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:33</code> | <code>export type IntentEndpointKind = (typeof INTENT_ENDPOINT_KINDS)[number];</code> |
| <code>interface.9.ts-sdk-intent-intentendpointref</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:131</code> | <code>export interface IntentEndpointRef</code> |
| <code>interface.9.ts-sdk-intent-intentleg</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:150</code> | <code>export interface IntentLeg</code> |
| <code>interface.9.ts-sdk-intent-intentlegbinding</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:175</code> | <code>export interface IntentLegBinding</code> |
| <code>interface.9.ts-sdk-intent-intentmoney</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:126</code> | <code>export interface IntentMoney</code> |
| <code>interface.9.ts-sdk-intent-intentoperation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:124</code> | <code>export type IntentOperation = keyof typeof INTENT_ROUTES;</code> |
| <code>interface.9.ts-sdk-intent-intentplan</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/intent.ts:167</code> | <code>export interface IntentPlan</code> |
| <code>interface.9.ts-sdk-intent-intentsigningrequirement</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:160</code> | <code>export interface IntentSigningRequirement</code> |
| <code>interface.9.ts-sdk-intent-intentsubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:193</code> | <code>export interface IntentSubmission</code> |
| <code>interface.9.ts-sdk-intent-planintentrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:142</code> | <code>export interface PlanIntentRequest</code> |
| <code>interface.9.ts-sdk-intent-submitplanrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/intent.ts:187</code> | <code>export interface SubmitPlanRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-intent-decodehumanenvelope</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-decodehumanerror</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-decodeintentplan</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-decodeintentsubmission</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-encodeplanintentrequest</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-encodesubmitplanrequest</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-human-error-codes</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-human-retriabilities</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-humanerrorcode</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-humanintentclient</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-humanintentclientoptions</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-humanintenterror</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-humanintenterrordetails</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-humanretriability</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intent-domains</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intent-endpoint-kinds</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentconstraints</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentdomain</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentendpointkind</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentendpointref</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentleg</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-intentlegbinding</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentmoney</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentoperation</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentplan</code> | notes | Named by stale pending task 6.5.36. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentsigningrequirement</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-intentsubmission</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-intent-planintentrequest</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |
| <code>interface.9.ts-sdk-intent-submitplanrequest</code> | notes | File is touched by stale pending task 10.5.38, 6.5.36; the export itself is not named by them. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.362.1</code> | req | 9362.1 |
| <code>acceptance.9.362.1</code> | scenario | Given a plan document with two legs carrying calls to 0x101A and nonces 4 and 5, when the test decodes it, then legs[1].call.to is 0x101A and legs[1].nonce is 5n. |
| <code>acceptance.9.362.1</code> | test | services/agent/sdk/typescript/test/intent-legs.test.ts::intent_plan_legs_carry_an_EVM_call_and_nonce (planned) |
| <code>acceptance.9.362.1</code> | qualification | UNRUN |
| <code>req.9362</code> | ac_1 | A decoded plan exposes each leg's call and nonce. |
| <code>acceptance.9.362.2</code> | req | 9362.2 |
| <code>acceptance.9.362.2</code> | scenario | Given the same plan with nonce null on the second leg and sponsored false, when the test decodes it, then it throws HumanIntentError. |
| <code>acceptance.9.362.2</code> | test | services/agent/sdk/typescript/test/intent-legs.test.ts::a_plan_leg_without_a_nonce_is_refused (planned) |
| <code>acceptance.9.362.2</code> | qualification | UNRUN |
| <code>req.9362</code> | ac_2 | A non-sponsored plan with a missing leg nonce is refused. |
| <code>capability.9.33.12</code> | title | Carry per-leg EVM calls and nonces in TypeScript intent plans |
| <code>capability.9.33.12</code> | reqs | 9362.1<br>9362.2 |

## launchpad

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-launchpad-decodelaunchpadevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:26</code> | <code>export function decodeLaunchpadEvent(log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadapproval</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:106</code> | <code>export async function launchpadApproval(rpc: Eip1193Requester, owner: string, spend: LaunchpadSpend): Promise&lt;PrecompileCall &#124; undefined&gt;</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadapprovecall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:87</code> | <code>export function launchpadApproveCall(token: string, amount: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadbuycall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:46</code> | <code>export function launchpadBuyCall(order: LaunchpadSwapOrder): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:33</code> | <code>export function launchpadCall(method: LaunchpadWrite, args: readonly AbiArgument[], value = 0n): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadclaimfeescall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:66</code> | <code>export function launchpadClaimFeesCall(nftId: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadcreatemarketcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:56</code> | <code>export function launchpadCreateMarketCall(name: string, symbol: string, feeStrategy: number, optical: string = NO_OPTICAL): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadnftcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:73</code> | <code>export function launchpadNftCall(write: LaunchpadNftWrite, nftId: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadnftwrite</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:70</code> | <code>export type LaunchpadNftWrite = "claimAirdrop" &#124; "executeAirdrop" &#124; "executeBurn" &#124; "executeLpRewards";</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadsellcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:51</code> | <code>export function launchpadSellCall(order: LaunchpadSwapOrder): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadsetfeestrategycall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:61</code> | <code>export function launchpadSetFeeStrategyCall(nftId: bigint, feeStrategy: number): PrecompileCall</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadspend</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:81</code> | <code>export type LaunchpadSpend =</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadswaporder</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:38</code> | <code>export interface LaunchpadSwapOrder</code> |
| <code>interface.9.ts-sdk-launchpad-launchpadwrite</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:30</code> | <code>export type LaunchpadWrite = keyof typeof LAUNCHPAD_WRITES;</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:155</code> | <code>export const sendLaunchpadBuy = (wallet: Eip1193Requester, from: string, order: LaunchpadSwapOrder): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadclaimfees</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:177</code> | <code>export const sendLaunchpadClaimFees = (wallet: Eip1193Requester, from: string, nftId: bigint): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadcreatemarket</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:161</code> | <code>export const sendLaunchpadCreateMarket = (</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadnftwrite</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:180</code> | <code>export const sendLaunchpadNftWrite = (</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadsell</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:158</code> | <code>export const sendLaunchpadSell = (wallet: Eip1193Requester, from: string, order: LaunchpadSwapOrder): Promise&lt;string&gt; =&gt;</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadsetfeestrategy</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:170</code> | <code>export const sendLaunchpadSetFeeStrategy = (</code> |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadwithapproval</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:140</code> | <code>export async function sendLaunchpadWithApproval(</code> |
| <code>interface.9.ts-sdk-launchpad-waitlaunchpadapproval</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/launchpad.ts:121</code> | <code>export async function waitLaunchpadApproval(rpc: Eip1193Requester, hash: string, attempts = 60, intervalMs = 1_000): Promise&lt;void&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-launchpad-launchpadwrite</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadclaimfees</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadsetfeestrategy</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-launchpad-sendlaunchpadwithapproval</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-launchpad-waitlaunchpadapproval</code> | notes | No caller in package tests, examples or consuming packages. |

## mirror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-mirror-mirrorcandidate</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/mirror.ts:27</code> | <code>export interface MirrorCandidate</code> |
| <code>interface.9.ts-sdk-mirror-mirrorlag</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/mirror.ts:1</code> | <code>export type MirrorLag =</code> |
| <code>interface.9.ts-sdk-mirror-mirrorobservation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/mirror.ts:15</code> | <code>export interface MirrorObservation</code> |
| <code>interface.9.ts-sdk-mirror-mirrorprovenance</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/mirror.ts:5</code> | <code>export type MirrorProvenance = "canonical" &#124; "reorged";</code> |
| <code>interface.9.ts-sdk-mirror-mirrorreadpolicy</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/mirror.ts:32</code> | <code>export type MirrorReadPolicy =</code> |
| <code>interface.9.ts-sdk-mirror-mirrortarget</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/mirror.ts:7</code> | <code>export interface MirrorTarget</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-mirror-mirrorcandidate</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-mirror-mirrorlag</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-mirror-mirrorobservation</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-mirror-mirrorprovenance</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-mirror-mirrorreadpolicy</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-mirror-mirrortarget</code> | notes | No caller in package tests, examples or consuming packages. |

## native-capabilities

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-native-capabilities-decodenativecapabilityset</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts:128</code> | <code>export async function decodeNativeCapabilitySet(bytes: Uint8Array): Promise&lt;readonly NativeCapability[]&gt;</code> |
| <code>interface.9.ts-sdk-native-capabilities-derivenativeprogramaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts:62</code> | <code>export async function deriveNativeProgramAccount(owner: Uint8Array, seed: Uint8Array): Promise&lt;Uint8Array&gt;</code> |
| <code>interface.9.ts-sdk-native-capabilities-encodenativecapabilityset</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts:90</code> | <code>export async function encodeNativeCapabilitySet(grants: readonly NativeCapability[]): Promise&lt;Uint8Array&gt;</code> |
| <code>interface.9.ts-sdk-native-capabilities-max-native-balance-views</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts:2</code> | <code>export const MAX_NATIVE_BALANCE_VIEWS = 32;</code> |
| <code>interface.9.ts-sdk-native-capabilities-max-native-capabilities</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts:1</code> | <code>export const MAX_NATIVE_CAPABILITIES = 238;</code> |
| <code>interface.9.ts-sdk-native-capabilities-max-native-capability-bytes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts:3</code> | <code>export const MAX_NATIVE_CAPABILITY_BYTES = 65_452;</code> |
| <code>interface.9.ts-sdk-native-capabilities-narrownativecapabilityset</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts:138</code> | <code>export async function narrowNativeCapabilitySet(parent: readonly NativeCapability[], requested: readonly NativeCapability[]): Promise&lt;readonly NativeCapability[]&gt;</code> |
| <code>interface.9.ts-sdk-native-capabilities-nativecapability</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-capabilities.ts:5</code> | <code>export type NativeCapability =</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-native-capabilities-max-native-balance-views</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-native-capabilities-max-native-capabilities</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-native-capabilities-max-native-capability-bytes</code> | notes | No caller in package tests, examples or consuming packages. |

## native-effect

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-native-effect-encodenativeeffectpreparerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-effect.ts:50</code> | <code>export function encodeNativeEffectPrepareRequest(value: NativeEffectPrepareRequestV1): NativeEffectPrepareRequestV1</code> |
| <code>interface.9.ts-sdk-native-effect-nativeeffectpreparerequestdigest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-effect.ts:79</code> | <code>export function nativeEffectPrepareRequestDigest(value: NativeEffectPrepareRequestV1): string</code> |
| <code>interface.9.ts-sdk-native-effect-nativeeffectpreparerequestv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-effect.ts:4</code> | <code>export interface NativeEffectPrepareRequestV1 extends Omit&lt;NativePrepareRequestV1, "variant"&gt;</code> |

## native-program-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-native-program-call-decodenativeprogramcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts:48</code> | <code>export function decodeNativeProgramCall(payload: Uint8Array): NativeProgramCall</code> |
| <code>interface.9.ts-sdk-native-program-call-decodenativeprogramcallv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts:52</code> | <code>export function decodeNativeProgramCallV1(payload: Uint8Array): NativeProgramCallV1</code> |
| <code>interface.9.ts-sdk-native-program-call-encodenativeprogramcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts:16</code> | <code>export function encodeNativeProgramCall(call: NativeProgramCall): Uint8Array</code> |
| <code>interface.9.ts-sdk-native-program-call-encodenativeprogramcallv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts:20</code> | <code>export function encodeNativeProgramCallV1(call: NativeProgramCallV1): Uint8Array</code> |
| <code>interface.9.ts-sdk-native-program-call-nativeprogramcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts:3</code> | <code>export interface NativeProgramCall</code> |
| <code>interface.9.ts-sdk-native-program-call-nativeprogramcallv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/native-program-call.ts:14</code> | <code>export type NativeProgramCallV1 = Omit&lt;NativeProgramCall, "guestAbi"&gt; &amp; Readonly&lt;{ guestAbi: 1 &#124; 2 &#124; 3 &#124; 4 &#124; 5 }&gt;;</code> |

## node-mirror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-node-mirror-decodemirrorprocessresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/node-mirror.ts:17</code> | <code>export function decodeMirrorProcessResult(code:number&#124;null,signal:NodeJS.Signals&#124;null,output:string,batchNumber:bigint):ExecutableMirrorVerification</code> |
| <code>interface.9.ts-sdk-node-mirror-executablemirrorerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/node-mirror.ts:8</code> | <code>export class ExecutableMirrorError extends Error{public constructor(public readonly code:string){super(`mirror verification refused: ${code}`);this.name="ExecutableMirrorError";}}</code> |
| <code>interface.9.ts-sdk-node-mirror-executablemirrorverification</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/node-mirror.ts:7</code> | <code>export interface ExecutableMirrorVerification {readonly level:string;readonly batchNumber:bigint;readonly headerDigest:Uint8Array;readonly evidenceDigest:Uint8Array;readonly sourceId:string;readonly target:string;readonly canonicalPosition:string;readonly provenance:"Canonical"&#124;"Reorged";readonly la</code> |
| <code>interface.9.ts-sdk-node-mirror-localmirrorexecutableverifier</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/node-mirror.ts:21</code> | <code>export class LocalMirrorExecutableVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-node-mirror-executablemirrorerror</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-node-mirror-executablemirrorverification</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-node-mirror-localmirrorexecutableverifier</code> | notes | No caller in package tests, examples or consuming packages. |

## perps

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-perps-decodeperpsactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:787</code> | <code>export function decodePerpsActivity(</code> |
| <code>interface.9.ts-sdk-perps-decodeperpsevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:762</code> | <code>export function decodePerpsEvent(log: PrecompileLog): DecodedPrecompileEvent</code> |
| <code>interface.9.ts-sdk-perps-encodedactivity</code> | <code>ts_export</code> | <code>retire</code> | <code>services/agent/sdk/typescript/src/perps.ts:180</code> | <code>export interface EncodedActivity</code> |
| <code>interface.9.ts-sdk-perps-encodeperpsactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:624</code> | <code>export function encodePerpsActivity(activity: PerpsActivity): Uint8Array</code> |
| <code>interface.9.ts-sdk-perps-perps-activity-types</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:49</code> | <code>export const PERPS_ACTIVITY_TYPES =</code> |
| <code>interface.9.ts-sdk-perps-perps-module-id</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:47</code> | <code>export const PERPS_MODULE_ID = 6;</code> |
| <code>interface.9.ts-sdk-perps-perpsactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:115</code> | <code>export type PerpsActivity =</code> |
| <code>interface.9.ts-sdk-perps-perpsactivityname</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:63</code> | <code>export type PerpsActivityName = keyof typeof PERPS_ACTIVITY_TYPES;</code> |
| <code>interface.9.ts-sdk-perps-perpsactivitytype</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:619</code> | <code>export function perpsActivityType(activity: PerpsActivity): number</code> |
| <code>interface.9.ts-sdk-perps-perpscall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:717</code> | <code>export function perpsCall(method: PerpsWrite, args: readonly AbiArgument[]): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpscancelordercall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:752</code> | <code>export function perpsCancelOrderCall(orderId: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpsclosepositioncall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:757</code> | <code>export function perpsClosePositionCall(positionId: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpsdepositcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:731</code> | <code>export function perpsDepositCall(token: string, amount: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpsmarket</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:88</code> | <code>export interface PerpsMarket</code> |
| <code>interface.9.ts-sdk-perps-perpsopenposition</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:722</code> | <code>export interface PerpsOpenPosition</code> |
| <code>interface.9.ts-sdk-perps-perpsopenpositioncall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:741</code> | <code>export function perpsOpenPositionCall(order: PerpsOpenPosition): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpswithdrawcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:736</code> | <code>export function perpsWithdrawCall(token: string, amount: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-perps-perpswrite</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:711</code> | <code>export type PerpsWrite = keyof typeof PERPS_WRITES;</code> |
| <code>interface.9.ts-sdk-perps-tradeside</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:83</code> | <code>export type TradeSide = 1 &#124; 2;</code> |
| <code>interface.9.ts-sdk-perps-tradingid</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:86</code> | <code>export type TradingId = string;</code> |
| <code>interface.9.ts-sdk-perps-tradingpayloaderror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:72</code> | <code>export class TradingPayloadError extends Error</code> |
| <code>interface.9.ts-sdk-perps-tradingpayloaderrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:65</code> | <code>export type TradingPayloadErrorCode =</code> |
| <code>interface.9.ts-sdk-perps-tradingreader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:409</code> | <code>export class TradingReader</code> |
| <code>interface.9.ts-sdk-perps-tradingwriter</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/perps.ts:238</code> | <code>export class TradingWriter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-perps-decodeperpsactivity</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-decodeperpsevent</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-encodedactivity</code> | notes | Named by stale pending task 10.5.10. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-encodedactivity</code> | overridden_by | override.J44.163 |
| <code>interface.9.ts-sdk-perps-encodeperpsactivity</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perps-activity-types</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perps-module-id</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-perpsactivity</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpsactivityname</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-perpsactivitytype</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpscall</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpscancelordercall</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpsclosepositioncall</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpsdepositcall</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpsmarket</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-perpsopenposition</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpsopenpositioncall</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpswithdrawcall</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-perpswrite</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-tradeside</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-tradingid</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-tradingpayloaderror</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-perps-tradingpayloaderrorcode</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-tradingreader</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-perps-tradingwriter</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.363.1</code> | req | 9363.1 |
| <code>acceptance.9.363.1</code> | scenario | Given a market id, an account id, side 1, size 4 and leverage 2, when the test builds the call, then data starts with the selector of openPosition(bytes32,bytes32,uint8,uint256,uint256) and the second word is the account id. |
| <code>acceptance.9.363.1</code> | test | services/agent/sdk/typescript/test/perps-v2.test.ts::perps_open_position_call_uses_the_v2_selector (planned) |
| <code>acceptance.9.363.1</code> | qualification | UNRUN |
| <code>req.9363</code> | ac_1 | perpsOpenPositionCall encodes the v2 openPosition selector with the account id. |
| <code>acceptance.9.363.2</code> | req | 9363.2 |
| <code>acceptance.9.363.2</code> | scenario | Given an account id and order id 9, when the test builds the call, then data is the cancelOrder(bytes32,bytes32) selector followed by the two words. |
| <code>acceptance.9.363.2</code> | test | services/agent/sdk/typescript/test/perps-v2.test.ts::perps_cancel_order_call_carries_the_account_id (planned) |
| <code>acceptance.9.363.2</code> | qualification | UNRUN |
| <code>req.9363</code> | ac_2 | perpsCancelOrderCall encodes cancelOrder(bytes32,bytes32). |
| <code>capability.9.33.13</code> | title | Encode perps writes as exchange v2 calls and drop the activity type |
| <code>capability.9.33.13</code> | reqs | 9363.1<br>9363.2 |

## production

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-production-agent-operations</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:3</code> | <code>export const AGENT_OPERATIONS = [</code> |
| <code>interface.9.ts-sdk-production-calloptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:272</code> | <code>export interface CallOptions</code> |
| <code>interface.9.ts-sdk-production-human-operations</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:54</code> | <code>export const HUMAN_OPERATIONS = [</code> |
| <code>interface.9.ts-sdk-production-humanoperation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:113</code> | <code>export type HumanOperation = (typeof HUMAN_OPERATIONS)[number];</code> |
| <code>interface.9.ts-sdk-production-idempotencykey</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:194</code> | <code>export type IdempotencyKey = string &amp; { readonly [idempotencyKeyBrand]: true };</code> |
| <code>interface.9.ts-sdk-production-idempotencykey-2</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:196</code> | <code>export function idempotencyKey(value: string): IdempotencyKey</code> |
| <code>interface.9.ts-sdk-production-platform-sdk-typescript</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:395</code> | <code>export function platform_sdk_typescript(): typeof PACKAGE_METADATA</code> |
| <code>interface.9.ts-sdk-production-platformplane</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:114</code> | <code>export type PlatformPlane = "agent" &#124; "human";</code> |
| <code>interface.9.ts-sdk-production-platformsdkerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:165</code> | <code>export class PlatformSdkError extends Error</code> |
| <code>interface.9.ts-sdk-production-productionclient</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:338</code> | <code>export class ProductionClient</code> |
| <code>interface.9.ts-sdk-production-productiontransport</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:268</code> | <code>export interface ProductionTransport</code> |
| <code>interface.9.ts-sdk-production-protocolamount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:204</code> | <code>export type ProtocolAmount = bigint &amp; { readonly [protocolAmountBrand]: true };</code> |
| <code>interface.9.ts-sdk-production-protocolamount-2</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:206</code> | <code>export function protocolAmount(value: bigint &#124; string): ProtocolAmount</code> |
| <code>interface.9.ts-sdk-production-retryclass</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:115</code> | <code>export type RetryClass = "never" &#124; "safe" &#124; "after" &#124; "unknown-outcome";</code> |
| <code>interface.9.ts-sdk-production-safeerrordetails</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:157</code> | <code>export interface SafeErrorDetails</code> |
| <code>interface.9.ts-sdk-production-sdk-error-codes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:117</code> | <code>export const SDK_ERROR_CODES = [</code> |
| <code>interface.9.ts-sdk-production-sdkerrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:136</code> | <code>export type SdkErrorCode = (typeof SDK_ERROR_CODES)[number];</code> |
| <code>interface.9.ts-sdk-production-sdktelemetry</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:283</code> | <code>export type SdkTelemetry = (event: SdkTelemetryEvent) =&gt; void;</code> |
| <code>interface.9.ts-sdk-production-sdktelemetryevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:276</code> | <code>export interface SdkTelemetryEvent</code> |
| <code>interface.9.ts-sdk-production-secretbytes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:218</code> | <code>export class SecretBytes</code> |
| <code>interface.9.ts-sdk-production-transportcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/production.ts:261</code> | <code>export interface TransportCall&lt;TRequest&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-production-agent-operations</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-production-calloptions</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-production-human-operations</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-production-humanoperation</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-production-idempotencykey-2</code> | notes | Callers list shows 62 of 115 caller files. |
| <code>interface.9.ts-sdk-production-platform-sdk-typescript</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-production-platformplane</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-production-platformsdkerror</code> | notes | Callers list shows 66 of 128 caller files. |
| <code>interface.9.ts-sdk-production-safeerrordetails</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-production-sdktelemetry</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-production-sdktelemetryevent</code> | notes | No caller in package tests, examples or consuming packages. |

## program-lifecycle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-program-lifecycle-decodenativeprogramdeploy</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:69</code> | <code>export function decodeNativeProgramDeploy(payload: Uint8Array): NativeProgramDeploy</code> |
| <code>interface.9.ts-sdk-program-lifecycle-decodenativeprogramupgrade</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:94</code> | <code>export function decodeNativeProgramUpgrade(payload: Uint8Array): NativeProgramUpgrade</code> |
| <code>interface.9.ts-sdk-program-lifecycle-decodenativeprogramwinddown</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:125</code> | <code>export function decodeNativeProgramWindDown(payload: Uint8Array): NativeProgramWindDown</code> |
| <code>interface.9.ts-sdk-program-lifecycle-encodenativeprogramdeploy</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:62</code> | <code>export function encodeNativeProgramDeploy(value: NativeProgramDeploy): Uint8Array</code> |
| <code>interface.9.ts-sdk-program-lifecycle-encodenativeprogramupgrade</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:85</code> | <code>export function encodeNativeProgramUpgrade(value: NativeProgramUpgrade): Uint8Array</code> |
| <code>interface.9.ts-sdk-program-lifecycle-encodenativeprogramwinddown</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:109</code> | <code>export function encodeNativeProgramWindDown(value: NativeProgramWindDown): Uint8Array</code> |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramdeploy</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:20</code> | <code>export interface NativeProgramDeploy</code> |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramlifecyclerequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:4</code> | <code>export class NativeProgramLifecycleRequest</code> |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramupgrade</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:29</code> | <code>export interface NativeProgramUpgrade</code> |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramwinddown</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:39</code> | <code>export type NativeProgramWindDown = Readonly&lt;{ programId: Uint8Array }&gt; &amp; (</code> |
| <code>interface.9.ts-sdk-program-lifecycle-programlifecyclesubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-lifecycle.ts:17</code> | <code>export type ProgramLifecycleSubmission = Readonly&lt;{ state: "acknowledged"; activity_id: string; receipt: string; result_code: number }&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramdeploy</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramupgrade</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-program-lifecycle-nativeprogramwinddown</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-program-lifecycle-programlifecyclesubmission</code> | notes | No caller in package tests, examples or consuming packages. |

## program-wire

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-program-wire-assertfreshsimulationobservation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-wire.ts:137</code> | <code>export function assertFreshSimulationObservation(</code> |
| <code>interface.9.ts-sdk-program-wire-bindretainedprogramcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-wire.ts:348</code> | <code>export async function bindRetainedProgramCall(</code> |
| <code>interface.9.ts-sdk-program-wire-bindsignedprogramlifecycle</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-wire.ts:67</code> | <code>export async function bindSignedProgramLifecycle(canonical: Uint8Array, payload: Uint8Array &#124; undefined, ordinal: 1 &#124; 2 &#124; 7, expectedIdempotencyKey?: string): Promise&lt;DecodedSignedProgramCall&gt;</code> |
| <code>interface.9.ts-sdk-program-wire-decodeandverifyprogramterminal</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-wire.ts:149</code> | <code>export async function decodeAndVerifyProgramTerminal(</code> |
| <code>interface.9.ts-sdk-program-wire-decodedprogramterminal</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-wire.ts:55</code> | <code>export interface DecodedProgramTerminal</code> |
| <code>interface.9.ts-sdk-program-wire-decodedsignedprogramcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-wire.ts:47</code> | <code>export interface DecodedSignedProgramCall</code> |
| <code>interface.9.ts-sdk-program-wire-decodesignedprogramcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-wire.ts:96</code> | <code>export async function decodeSignedProgramCall(</code> |
| <code>interface.9.ts-sdk-program-wire-occupancypayer</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/program-wire.ts:62</code> | <code>export interface OccupancyPayer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-program-wire-assertfreshsimulationobservation</code> | notes | File is touched by stale pending task 11.5.54; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-program-wire-bindretainedprogramcall</code> | notes | File is touched by stale pending task 11.5.54; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-program-wire-bindsignedprogramlifecycle</code> | notes | File is touched by stale pending task 11.5.54; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-program-wire-decodeandverifyprogramterminal</code> | notes | File is touched by stale pending task 11.5.54; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-program-wire-decodedprogramterminal</code> | notes | File is touched by stale pending task 11.5.54; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-program-wire-decodedsignedprogramcall</code> | notes | File is touched by stale pending task 11.5.54; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-program-wire-decodesignedprogramcall</code> | notes | File is touched by stale pending task 11.5.54; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-program-wire-occupancypayer</code> | notes | File is touched by stale pending task 11.5.54; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |

## programs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-programs-gatewayprogramdiscovery</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:562</code> | <code>export interface GatewayProgramDiscovery extends ProgramDiscovery</code> |
| <code>interface.9.ts-sdk-programs-nativeprogramrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:38</code> | <code>export class NativeProgramRequest implements ProgramCall</code> |
| <code>interface.9.ts-sdk-programs-nativeprogramrequestv1</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:50</code> | <code>export class NativeProgramRequestV1 implements ProgramCall</code> |
| <code>interface.9.ts-sdk-programs-parseprogramexecutiondocument</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:448</code> | <code>export function parseProgramExecutionDocument(value: unknown): ProgramExecutionDocument</code> |
| <code>interface.9.ts-sdk-programs-parseprogramexecutiondocumentv5</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:551</code> | <code>export function parseProgramExecutionDocumentV5(value: unknown): ProgramExecutionDocument</code> |
| <code>interface.9.ts-sdk-programs-platform-sdk-programs</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:830</code> | <code>export function platform_sdk_programs(): string { return "receipt-verified-program-operations-v1"; }</code> |
| <code>interface.9.ts-sdk-programs-programauthoritydocument</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:73</code> | <code>export interface ProgramAuthorityDocument { readonly batch_id: string; readonly asset: string; readonly previous_state_root: string; readonly resulting_state_root: string; readonly sequencer_public_key: string }</code> |
| <code>interface.9.ts-sdk-programs-programbudget</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:27</code> | <code>export interface ProgramBudget { readonly fuel: bigint; readonly feeLimit: bigint }</code> |
| <code>interface.9.ts-sdk-programs-programcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:29</code> | <code>export interface ProgramCall</code> |
| <code>interface.9.ts-sdk-programs-programcapability</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:28</code> | <code>export type ProgramCapability = keyof typeof CAPABILITY_ORDER;</code> |
| <code>interface.9.ts-sdk-programs-programdiscovery</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:67</code> | <code>export interface ProgramDiscovery { readonly program_id: string; readonly lifecycle: "active" &#124; "deprecated" &#124; "tombstoned"; readonly version: number; readonly code_hash: string; readonly abi_version: number; readonly receipt_digest: string; readonly state_root: string; readonly observed_sequence: s</code> |
| <code>interface.9.ts-sdk-programs-programexecutiondocument</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:74</code> | <code>export interface ProgramExecutionDocument { readonly state: "executed" &#124; "refused" &#124; "simulated"; readonly activity_id: string; readonly program_id: string; readonly guest_abi_version: number; readonly module_version: number; readonly batch_id: string; readonly global_sequence: string; readonly resu</code> |
| <code>interface.9.ts-sdk-programs-programfailure</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:70</code> | <code>export type ProgramFailure = Readonly&lt;{ kind: "unknown_program" &#124; "reentrancy" &#124; "depth_exceeded" &#124; "fanout_exceeded" &#124; "guest_refused" &#124; "authority" &#124; "resource" &#124; "response" &#124; "fault"; code?: number; limit?: number; attempted?: number }&gt;;</code> |
| <code>interface.9.ts-sdk-programs-programinterface</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:69</code> | <code>export interface ProgramInterface { readonly program_id: string; readonly version: number; readonly code_hash: string; readonly abi_version: number; readonly interface: string; readonly interface_digest: string; readonly receipt_digest: string; readonly state_root: string; readonly observed_sequence</code> |
| <code>interface.9.ts-sdk-programs-programoperations</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:233</code> | <code>export class ProgramOperations</code> |
| <code>interface.9.ts-sdk-programs-programoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:71</code> | <code>export type ProgramOutcome = Readonly&lt;{ kind: "completed"; code: number; response: string }&gt; &#124; Readonly&lt;{ kind: "legacy_completed"; code: number; values: readonly unknown[] }&gt; &#124; Readonly&lt;{ kind: "refused"; failure: ProgramFailure }&gt;;</code> |
| <code>interface.9.ts-sdk-programs-programsimulation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:78</code> | <code>export interface ProgramSimulation { readonly committed: false; readonly execution: ProgramExecutionDocument &amp; Readonly&lt;{ state: "simulated" }&gt;; readonly simulation_evidence: ProgramSimulationEvidence }</code> |
| <code>interface.9.ts-sdk-programs-programsimulationevidence</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:77</code> | <code>export interface ProgramSimulationEvidence { readonly boundary_id: string; readonly activity_id: string; readonly previous_state_root: string; readonly hypothetical_state_root: string; readonly observed_sequence: string; readonly observed_at: string; readonly committed: false; readonly public_key: s</code> |
| <code>interface.9.ts-sdk-programs-programsource</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:68</code> | <code>export type ProgramSource = Readonly&lt;{ status: "unpublished" }&gt; &#124; Readonly&lt;{ status: "verified"; source_digest: string; environment_digest: string; pipeline: "sha256-source-artifact-reproducible-build-v1" }&gt; &#124; Readonly&lt;{ status: "mismatch"; expected_code_hash: string; reproduced_artifact_digest: str</code> |
| <code>interface.9.ts-sdk-programs-programsubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:76</code> | <code>export type ProgramSubmission = ProgramUnknownSubmission &#124; (ProgramExecutionDocument &amp; Readonly&lt;{ state: "executed" &#124; "refused" }&gt;);</code> |
| <code>interface.9.ts-sdk-programs-programtrustcontext</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:81</code> | <code>export class ProgramTrustContext</code> |
| <code>interface.9.ts-sdk-programs-programunknownsubmission</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:75</code> | <code>export interface ProgramUnknownSubmission { readonly state: "unknown"; readonly activity_id: string; readonly idempotency_key: string; readonly retained_signed_activity?: string }</code> |
| <code>interface.9.ts-sdk-programs-programusage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:72</code> | <code>export interface ProgramUsage { readonly cpu_fuel: string; readonly memory_bytes: string; readonly storage_read_bytes: string; readonly storage_write_bytes: string; readonly output_values: number; readonly output_bytes: string; readonly fee_units: string }</code> |
| <code>interface.9.ts-sdk-programs-resolvelifecyclefailure</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:365</code> | <code>export function resolveLifecycleFailure(error: unknown, binding: DecodedSignedProgramCall): ProgramLifecycleSubmission</code> |
| <code>interface.9.ts-sdk-programs-resolvelifecycleresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:370</code> | <code>export async function resolveLifecycleResponse(result: unknown, binding: DecodedSignedProgramCall, sequencer: Uint8Array): Promise&lt;ProgramLifecycleSubmission&gt;</code> |
| <code>interface.9.ts-sdk-programs-verifiedprogramreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:79</code> | <code>export interface VerifiedProgramReceipt { readonly verification: ReceiptVerification; readonly terminalPayload: Uint8Array; readonly callGraph: Uint8Array; readonly transferVerification: "reconstructed" &#124; "recorded_terminal_root_not_locally_reconstructable"; readonly occupancyPaymentAccounts: readon</code> |
| <code>interface.9.ts-sdk-programs-verifygatewayprogramdiscovery</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:569</code> | <code>export async function verifyGatewayProgramDiscovery(value: unknown, programId: string, trust: ProgramTrustContext): Promise&lt;GatewayProgramDiscovery&gt;</code> |
| <code>interface.9.ts-sdk-programs-verifygatewayprograminterface</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:610</code> | <code>export async function verifyGatewayProgramInterface(value: unknown, programId: string, trust: ProgramTrustContext,</code> |
| <code>interface.9.ts-sdk-programs-verifylifecyclerecovery</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:389</code> | <code>export async function verifyLifecycleRecovery(result: unknown, expectedActivity: string, sequencer: Uint8Array): Promise&lt;ReceiptVerification&gt;</code> |
| <code>interface.9.ts-sdk-programs-verifyprogramreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:127</code> | <code>export async function verifyProgramReceipt(</code> |
| <code>interface.9.ts-sdk-programs-verifyprogramreceiptv5</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/programs.ts:179</code> | <code>export async function verifyProgramReceiptV5(</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-programs-gatewayprogramdiscovery</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-nativeprogramrequest</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-nativeprogramrequestv1</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-parseprogramexecutiondocument</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-parseprogramexecutiondocumentv5</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-platform-sdk-programs</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programauthoritydocument</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programbudget</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programcall</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programcapability</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programdiscovery</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programexecutiondocument</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programfailure</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programinterface</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programoperations</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-programoutcome</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programsimulation</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programsimulationevidence</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programsource</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programsubmission</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programtrustcontext</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-programunknownsubmission</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-programusage</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-resolvelifecyclefailure</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-resolvelifecycleresponse</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-verifiedprogramreceipt</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-programs-verifygatewayprogramdiscovery</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-verifygatewayprograminterface</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-verifylifecyclerecovery</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-verifyprogramreceipt</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-programs-verifyprogramreceiptv5</code> | notes | File is touched by stale pending task 11.5.54, 6.5.29; the export itself is not named by them. |

## px

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-px-decodepxaccountbalances</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:367</code> | <code>export function decodePxAccountBalances(value: unknown): PxAccountBalances</code> |
| <code>interface.9.ts-sdk-px-decodepxaccountjoin</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:356</code> | <code>export function decodePxAccountJoin(value: unknown): PxAccountJoin</code> |
| <code>interface.9.ts-sdk-px-decodepxanchorhead</code> | <code>ts_export</code> | <code>retire</code> | <code>services/agent/sdk/typescript/src/px.ts:516</code> | <code>export function decodePxAnchorHead(value: unknown): PxAnchorHead</code> |
| <code>interface.9.ts-sdk-px-decodepxassetbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:380</code> | <code>export function decodePxAssetBalance(value: unknown): PxAssetBalance</code> |
| <code>interface.9.ts-sdk-px-decodepxassettable</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:435</code> | <code>export function decodePxAssetTable(value: unknown): PxAssetTable</code> |
| <code>interface.9.ts-sdk-px-decodepxcustodyasset</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:402</code> | <code>export function decodePxCustodyAsset(value: unknown): PxCustodyAsset &#124; null</code> |
| <code>interface.9.ts-sdk-px-decodepxnetworkhead</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:456</code> | <code>export function decodePxNetworkHead(value: unknown): PxNetworkHead</code> |
| <code>interface.9.ts-sdk-px-decodepxreceiptkeys</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:479</code> | <code>export function decodePxReceiptKeys(</code> |
| <code>interface.9.ts-sdk-px-decodepxresolvedidentities</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:334</code> | <code>export function decodePxResolvedIdentities(</code> |
| <code>interface.9.ts-sdk-px-px-maximum-joined-assets</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:29</code> | <code>export const PX_MAXIMUM_JOINED_ASSETS = 1024;</code> |
| <code>interface.9.ts-sdk-px-px-methods</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/px.ts:31</code> | <code>export const PX_METHODS = [</code> |
| <code>interface.9.ts-sdk-px-pxaccountbalances</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:89</code> | <code>export interface PxAccountBalances</code> |
| <code>interface.9.ts-sdk-px-pxaccountjoin</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:57</code> | <code>export interface PxAccountJoin</code> |
| <code>interface.9.ts-sdk-px-pxaccountkey</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:291</code> | <code>export function pxAccountKey(account: string): string</code> |
| <code>interface.9.ts-sdk-px-pxanchorhead</code> | <code>ts_export</code> | <code>retire</code> | <code>services/agent/sdk/typescript/src/px.ts:106</code> | <code>export interface PxAnchorHead</code> |
| <code>interface.9.ts-sdk-px-pxassetbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:81</code> | <code>export interface PxAssetBalance</code> |
| <code>interface.9.ts-sdk-px-pxassetentry</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:95</code> | <code>export interface PxAssetEntry</code> |
| <code>interface.9.ts-sdk-px-pxassettable</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:101</code> | <code>export interface PxAssetTable</code> |
| <code>interface.9.ts-sdk-px-pxclient</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:140</code> | <code>export class PxClient</code> |
| <code>interface.9.ts-sdk-px-pxclientoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:132</code> | <code>export interface PxClientOptions</code> |
| <code>interface.9.ts-sdk-px-pxcustodyasset</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:63</code> | <code>export interface PxCustodyAsset</code> |
| <code>interface.9.ts-sdk-px-pxmethod</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:41</code> | <code>export type PxMethod = (typeof PX_METHODS)[number];</code> |
| <code>interface.9.ts-sdk-px-pxnetworkhead</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:118</code> | <code>export interface PxNetworkHead</code> |
| <code>interface.9.ts-sdk-px-pxoptionalquantity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:320</code> | <code>export function pxOptionalQuantity(value: unknown): bigint &#124; null</code> |
| <code>interface.9.ts-sdk-px-pxpaxeeraccount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:51</code> | <code>export interface PxPaxeerAccount</code> |
| <code>interface.9.ts-sdk-px-pxpaxeerassetbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:76</code> | <code>export interface PxPaxeerAssetBalance</code> |
| <code>interface.9.ts-sdk-px-pxpaxeerhead</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:113</code> | <code>export interface PxPaxeerHead</code> |
| <code>interface.9.ts-sdk-px-pxquantity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:301</code> | <code>export function pxQuantity(value: unknown): bigint</code> |
| <code>interface.9.ts-sdk-px-pxreceiptkeyentry</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:125</code> | <code>export interface PxReceiptKeyEntry</code> |
| <code>interface.9.ts-sdk-px-pxresolvedidentities</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/px.ts:43</code> | <code>export interface PxResolvedIdentities</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-px-decodepxaccountbalances</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-decodepxaccountjoin</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-decodepxanchorhead</code> | notes | Named by stale pending task 10.5.7. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-decodepxanchorhead</code> | overridden_by | override.J44.162 |
| <code>interface.9.ts-sdk-px-decodepxassetbalance</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-decodepxassettable</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-decodepxcustodyasset</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-decodepxnetworkhead</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-decodepxreceiptkeys</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-decodepxresolvedidentities</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-px-maximum-joined-assets</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-px-methods</code> | notes | Named by stale pending task 10.5.21. |
| <code>interface.9.ts-sdk-px-pxaccountbalances</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxaccountjoin</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxaccountkey</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-pxanchorhead</code> | notes | Named by stale pending task 11.5.34, 11.5.41, 11.5.49, 11.5.51, 11.5.55. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxanchorhead</code> | overridden_by | override.J44.161 |
| <code>interface.9.ts-sdk-px-pxassetbalance</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxassetentry</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxassettable</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxclient</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-pxclientoptions</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxcustodyasset</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxmethod</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxnetworkhead</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxoptionalquantity</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-pxpaxeeraccount</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxpaxeerassetbalance</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxpaxeerhead</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxquantity</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. |
| <code>interface.9.ts-sdk-px-pxreceiptkeyentry</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-px-pxresolvedidentities</code> | notes | File is touched by stale pending task 10.5.7, 11.5.55; the export itself is not named by them. No caller in package tests, examples or consuming packages. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.355.1</code> | req | 9355.1 |
| <code>acceptance.9.355.1</code> | scenario | Given a px_getNetwork result with latest_committed_height 9 and another with an anchor member, when the test decodes both, then the first has latestCommittedHeight 9n and the second throws. |
| <code>acceptance.9.355.1</code> | test | services/agent/sdk/typescript/test/px.test.ts::px_network_head_carries_the_commit_finality_and_no_anchor (planned) |
| <code>acceptance.9.355.1</code> | qualification | UNRUN |
| <code>req.9355</code> | ac_1 | decodePxNetworkHead returns latestCommittedHeight and refuses an anchor field. |
| <code>acceptance.9.355.2</code> | req | 9355.2 |
| <code>acceptance.9.355.2</code> | scenario | Given the exported PX_METHODS, when the test compares it with px_resolveAccount, px_getAccount, px_getBalances, px_listAssets, px_getNetwork, px_getReceiptKeys, px_getHistory and px_getUnifiedHistory, then they are equal in order. |
| <code>acceptance.9.355.2</code> | test | services/agent/sdk/typescript/test/px.test.ts::px_methods_list_is_the_router_px_set (planned) |
| <code>acceptance.9.355.2</code> | qualification | UNRUN |
| <code>req.9355</code> | ac_2 | PX_METHODS lists exactly the eight px methods. |
| <code>capability.9.33.5</code> | title | Drop the anchor head from the TypeScript px client |
| <code>capability.9.33.5</code> | reqs | 9355.1<br>9355.2 |
| <code>resolution.17.851</code> | title | decision.926 binds: The TypeScript and Python SDKs verify sequencer receipts at the receipted and committed rungs and drop anchor and checkpoint verification |
| <code>resolution.17.851</code> | applies_to | decision.926<br>task.9.33.5<br>task.9.34.2<br>task.9.36.1 |
| <code>resolution.17.851</code> | source | decision.926 in fragments/J44/A-decision.kvx; tasks from affects |

## revert

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-revert-decoderevertreason</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/revert.ts:74</code> | <code>export function decodeRevertReason(returnData: string &#124; Uint8Array): RevertReasonError</code> |
| <code>interface.9.ts-sdk-revert-lxp-retryable</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/revert.ts:4</code> | <code>export const LXP_RETRYABLE: ReadonlySet&lt;string&gt; = new Set([</code> |
| <code>interface.9.ts-sdk-revert-revertreasonerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/revert.ts:54</code> | <code>export class RevertReasonError extends Error</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-revert-decoderevertreason</code> | notes | Named by stale pending task 0.36. |
| <code>interface.9.ts-sdk-revert-decoderevertreason</code> | overridden_by | override.J44.158 |

## rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-rpc-assetlistsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:37</code> | <code>export interface AssetListSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-assetmetadata</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:18</code> | <code>export interface AssetMetadata</code> |
| <code>interface.9.ts-sdk-rpc-assetsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:32</code> | <code>export interface AssetSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-commitment</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:8</code> | <code>export type Commitment = "executed" &#124; "batched" &#124; "finalised";</code> |
| <code>interface.9.ts-sdk-rpc-decodeassetlistsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:207</code> | <code>export function decodeAssetListSnapshot(value: Record&lt;string, unknown&gt;): AssetListSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-decodeassetsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:198</code> | <code>export function decodeAssetSnapshot(value: Record&lt;string, unknown&gt;): AssetSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-decodeidentitysequencesnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:216</code> | <code>export function decodeIdentitySequenceSnapshot(value: Record&lt;string, unknown&gt;): IdentitySequenceSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-decodejsonrpcresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:149</code> | <code>export function decodeJsonRpcResponse(value: unknown, id: string): Record&lt;string, unknown&gt;</code> |
| <code>interface.9.ts-sdk-rpc-identitysequencesnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:12</code> | <code>export interface IdentitySequenceSnapshot</code> |
| <code>interface.9.ts-sdk-rpc-jsonrpcclient</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/rpc.ts:47</code> | <code>export class JsonRpcClient</code> |
| <code>interface.9.ts-sdk-rpc-jsonrpcerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:43</code> | <code>export class JsonRpcError extends Error</code> |
| <code>interface.9.ts-sdk-rpc-list-assets-default-page</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:10</code> | <code>export const LIST_ASSETS_DEFAULT_PAGE = 64;</code> |
| <code>interface.9.ts-sdk-rpc-list-assets-max-page</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:11</code> | <code>export const LIST_ASSETS_MAX_PAGE = 256;</code> |
| <code>interface.9.ts-sdk-rpc-listassetsparams</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:201</code> | <code>export function listAssetsParams(cursor?: string, limit?: number): readonly RpcParam[]</code> |
| <code>interface.9.ts-sdk-rpc-rpcparam</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:9</code> | <code>export type RpcParam = string &#124; number &#124; null;</code> |
| <code>interface.9.ts-sdk-rpc-walletaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc.ts:224</code> | <code>export function walletAccount(did: string, asset: string, nativeAsset: string): string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-rpc-assetlistsnapshot</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-assetmetadata</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-assetsnapshot</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-commitment</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-decodeassetlistsnapshot</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-decodeassetsnapshot</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-decodeidentitysequencesnapshot</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-decodejsonrpcresponse</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-identitysequencesnapshot</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-jsonrpcclient</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-jsonrpcclient</code> | overridden_by | override.J44.159 |
| <code>interface.9.ts-sdk-rpc-jsonrpcerror</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-list-assets-default-page</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-list-assets-max-page</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-listassetsparams</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |
| <code>interface.9.ts-sdk-rpc-rpcparam</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-walletaccount</code> | notes | File is touched by stale pending task 10.5.6, 11.5.55, 6.5.31; the export itself is not named by them. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.352.1</code> | req | 9352.1 |
| <code>acceptance.9.352.1</code> | scenario | Given a loopback server answering each method with a fixture result, when the test calls each read, then every recorded method name starts with eth_ or px_. |
| <code>acceptance.9.352.1</code> | test | services/agent/sdk/typescript/test/rpc-node.test.ts::JsonRpcClient_reads_use_only_eth_and_px_methods (planned) |
| <code>acceptance.9.352.1</code> | qualification | UNRUN |
| <code>req.9352</code> | ac_1 | Every JsonRpcClient read sends an eth_ or px_ method. |
| <code>acceptance.9.352.2</code> | req | 9352.2 |
| <code>acceptance.9.352.2</code> | scenario | Given a server answering eth_sendRawTransaction with a 32-byte hash, when the test sends the evm test raw bytes, then params[0] is their hex and the result is the hash. |
| <code>acceptance.9.352.2</code> | test | services/agent/sdk/typescript/test/rpc-node.test.ts::sendRawTransaction_posts_eth_sendRawTransaction_and_returns_the_hash (planned) |
| <code>acceptance.9.352.2</code> | qualification | UNRUN |
| <code>req.9352</code> | ac_2 | sendRawTransaction sends the 0x-prefixed raw bytes and returns the hash. |
| <code>acceptance.9.352.3</code> | req | 9352.3 |
| <code>acceptance.9.352.3</code> | scenario | Given a server answering eth_getTransactionCount with 0x2a, when the test reads the sequence of an address, then params are the address and pending and the value is 42n. |
| <code>acceptance.9.352.3</code> | test | services/agent/sdk/typescript/test/rpc-node.test.ts::identity_sequence_reads_the_pending_transaction_count (planned) |
| <code>acceptance.9.352.3</code> | qualification | UNRUN |
| <code>req.9352</code> | ac_3 | getIdentitySequence returns the pending transaction count. |
| <code>capability.9.33.2</code> | title | Move JsonRpcClient reads and submission from lx_ methods to eth_ and px_ |
| <code>capability.9.33.2</code> | reqs | 9352.1<br>9352.2<br>9352.3 |

## rpc-subscription

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-rpc-subscription-subscriberpc</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:80</code> | <code>export async function* subscribeRpc(endpoint: URL, authorization: string &#124; undefined, requestId: string, topic: SubscriptionTopic, account?: string, cursor?: bigint, signal?: AbortSignal): AsyncGenerator&lt;SubscriptionEvent&gt;</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionacknowledgement</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:31</code> | <code>export function subscriptionAcknowledgement(value: unknown, requestId: string): string</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptioncontinuity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:57</code> | <code>export class SubscriptionContinuity</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptioncursor</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:24</code> | <code>export function subscriptionCursor(value: unknown): bigint</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:7</code> | <code>export interface SubscriptionEvent</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionnotification</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:45</code> | <code>export function subscriptionNotification(value: unknown, subscription: string): SubscriptionEvent</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionselector</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:50</code> | <code>export function subscriptionSelector(topic: SubscriptionTopic, account?: string, cursor?: bigint): string[]</code> |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptiontopic</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:5</code> | <code>export type SubscriptionTopic = "receipts" &#124; "checkpoints" &#124; "account";</code> |
| <code>interface.9.ts-sdk-rpc-subscription-unsubscribeacknowledgement</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/rpc-subscription.ts:38</code> | <code>export function unsubscribeAcknowledgement(value: unknown, requestId: string): true</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-rpc-subscription-subscriberpc</code> | notes | Named by stale pending task 10.5.7. Not re-exported from the package entry points index.ts or browser.ts. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionacknowledgement</code> | notes | File is touched by stale pending task 10.5.7, 6.5.31; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptioncontinuity</code> | notes | File is touched by stale pending task 10.5.7, 6.5.31; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptioncursor</code> | notes | File is touched by stale pending task 10.5.7, 6.5.31; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionevent</code> | notes | File is touched by stale pending task 10.5.7, 6.5.31; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionnotification</code> | notes | File is touched by stale pending task 10.5.7, 6.5.31; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptionselector</code> | notes | File is touched by stale pending task 10.5.7, 6.5.31; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptiontopic</code> | notes | File is touched by stale pending task 10.5.7, 6.5.31; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-rpc-subscription-subscriptiontopic</code> | overridden_by | override.J44.160 |
| <code>interface.9.ts-sdk-rpc-subscription-unsubscribeacknowledgement</code> | notes | File is touched by stale pending task 10.5.7, 6.5.31; the export itself is not named by them. Not re-exported from the package entry points index.ts or browser.ts. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.354.1</code> | req | 9354.1 |
| <code>acceptance.9.354.1</code> | scenario | Given a ws loopback server, when the test starts subscribeRpc on the heads topic, then the first frame is method eth_subscribe with params ["newHeads"]. |
| <code>acceptance.9.354.1</code> | test | services/agent/sdk/typescript/test/rpc-subscription-eth.test.ts::subscribeRpc_sends_eth_subscribe_newHeads (planned) |
| <code>acceptance.9.354.1</code> | qualification | UNRUN |
| <code>req.9354</code> | ac_1 | subscribeRpc opens the stream with eth_subscribe newHeads. |
| <code>acceptance.9.354.2</code> | req | 9354.2 |
| <code>acceptance.9.354.2</code> | scenario | Given the same server, when the test subscribes the account topic for an address, then params are ["logs", {address: [address]}] with the account topic filter. |
| <code>acceptance.9.354.2</code> | test | services/agent/sdk/typescript/test/rpc-subscription-eth.test.ts::account_topic_subscribes_to_logs_for_the_account (planned) |
| <code>acceptance.9.354.2</code> | qualification | UNRUN |
| <code>req.9354</code> | ac_2 | The account topic subscribes to logs filtered by the account address. |
| <code>acceptance.9.354.3</code> | req | 9354.3 |
| <code>acceptance.9.354.3</code> | scenario | Given the server pushing heads 10 and then 12, when the test reads the stream, then the second read throws a continuity error. |
| <code>acceptance.9.354.3</code> | test | services/agent/sdk/typescript/test/rpc-subscription-eth.test.ts::block_height_continuity_refuses_a_gap (planned) |
| <code>acceptance.9.354.3</code> | qualification | UNRUN |
| <code>req.9354</code> | ac_3 | A newHeads notification that skips a height ends the stream with an error. |
| <code>capability.9.33.4</code> | title | Subscribe through eth_subscribe newHeads and logs instead of lx_subscribe |
| <code>capability.9.33.4</code> | reqs | 9354.1<br>9354.2<br>9354.3 |

## spot

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-spot-decodespotactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:177</code> | <code>export function decodeSpotActivity(</code> |
| <code>interface.9.ts-sdk-spot-encodespotactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:100</code> | <code>export function encodeSpotActivity(activity: SpotActivity): Uint8Array</code> |
| <code>interface.9.ts-sdk-spot-spot-activity-types</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:27</code> | <code>export const SPOT_ACTIVITY_TYPES =</code> |
| <code>interface.9.ts-sdk-spot-spot-module-id</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:25</code> | <code>export const SPOT_MODULE_ID = 10;</code> |
| <code>interface.9.ts-sdk-spot-spotactivity</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:41</code> | <code>export type SpotActivity =</code> |
| <code>interface.9.ts-sdk-spot-spotactivitytype</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:95</code> | <code>export function spotActivityType(activity: SpotActivity): number</code> |
| <code>interface.9.ts-sdk-spot-spotcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:154</code> | <code>export function spotCall&lt;C extends SpotContract&gt;(</code> |
| <code>interface.9.ts-sdk-spot-spotcontract</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:145</code> | <code>export type SpotContract = keyof typeof SPOT_PRECOMPILES;</code> |
| <code>interface.9.ts-sdk-spot-spotorderkind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:36</code> | <code>export type SpotOrderKind = 1 &#124; 2;</code> |
| <code>interface.9.ts-sdk-spot-spottimeinforce</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:39</code> | <code>export type SpotTimeInForce = 1 &#124; 2;</code> |
| <code>interface.9.ts-sdk-spot-spotwrite</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/spot.ts:147</code> | <code>export type SpotWrite&lt;C extends SpotContract&gt; = keyof (typeof SPOT_WRITES)[C] &amp; string;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-spot-decodespotactivity</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-spot-encodespotactivity</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-spot-spot-activity-types</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-spot-spot-module-id</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-spot-spotactivity</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-spot-spotactivitytype</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-spot-spotcall</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-spot-spotcontract</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |
| <code>interface.9.ts-sdk-spot-spotorderkind</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-spot-spottimeinforce</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-spot-spotwrite</code> | notes | File is touched by stale pending task 10.5.10, 3.6.4; the export itself is not named by them. |

## stream

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-stream-resumablestream</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/stream.ts:31</code> | <code>export class ResumableStream&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-stream-streamcursor</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/stream.ts:4</code> | <code>export type StreamCursor = string &amp; { readonly [streamCursorBrand]: true };</code> |
| <code>interface.9.ts-sdk-stream-streamcursor-2</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/stream.ts:6</code> | <code>export function streamCursor(value: string): StreamCursor</code> |
| <code>interface.9.ts-sdk-stream-streamevent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/stream.ts:13</code> | <code>export interface StreamEvent&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-stream-streampage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/stream.ts:20</code> | <code>export interface StreamPage&lt;T&gt;</code> |
| <code>interface.9.ts-sdk-stream-streampagesource</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/stream.ts:26</code> | <code>export type StreamPageSource&lt;T&gt; = (</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-stream-streamevent</code> | notes | No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-stream-streampagesource</code> | notes | No caller in package tests, examples or consuming packages. |

## verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-verifier-authorizedreceiptbatch</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/verifier.ts:239</code> | <code>export interface AuthorizedReceiptBatch</code> |
| <code>interface.9.ts-sdk-verifier-batchheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:73</code> | <code>export interface BatchHeader</code> |
| <code>interface.9.ts-sdk-verifier-checkpointattestation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:107</code> | <code>export interface CheckpointAttestation</code> |
| <code>interface.9.ts-sdk-verifier-checkpointcertificate</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:133</code> | <code>export interface CheckpointCertificate</code> |
| <code>interface.9.ts-sdk-verifier-checkpointverification</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:151</code> | <code>export interface CheckpointVerification</code> |
| <code>interface.9.ts-sdk-verifier-checkpointverificationinput</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:141</code> | <code>export interface CheckpointVerificationInput</code> |
| <code>interface.9.ts-sdk-verifier-decodebatchheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:457</code> | <code>export function decodeBatchHeader(canonicalHeader: Uint8Array): BatchHeader</code> |
| <code>interface.9.ts-sdk-verifier-decodeprogramreceiptoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:833</code> | <code>export function decodeProgramReceiptOutcome(canonicalOutcome: Uint8Array, protocolVersion: number): ProgramReceiptOutcome</code> |
| <code>interface.9.ts-sdk-verifier-default-protocol-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:21</code> | <code>export const DEFAULT_PROTOCOL_VERSION = 2;</code> |
| <code>interface.9.ts-sdk-verifier-guarantorkey</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:127</code> | <code>export interface GuarantorKey</code> |
| <code>interface.9.ts-sdk-verifier-inclusionkind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:98</code> | <code>export type InclusionKind = "activity" &#124; "receipt" &#124; "event" &#124; "state";</code> |
| <code>interface.9.ts-sdk-verifier-inclusionverification</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:100</code> | <code>export interface InclusionVerification</code> |
| <code>interface.9.ts-sdk-verifier-isselectableprotocolversion</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:33</code> | <code>export function isSelectableProtocolVersion(version: number): version is SelectableProtocolVersion</code> |
| <code>interface.9.ts-sdk-verifier-legacy-protocol-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:20</code> | <code>export const LEGACY_PROTOCOL_VERSION = 1;</code> |
| <code>interface.9.ts-sdk-verifier-localsignatureverifier</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:159</code> | <code>export interface LocalSignatureVerifier</code> |
| <code>interface.9.ts-sdk-verifier-merkleproof</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:67</code> | <code>export interface MerkleProof</code> |
| <code>interface.9.ts-sdk-verifier-programreceiptoutcome</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:179</code> | <code>export interface ProgramReceiptOutcome</code> |
| <code>interface.9.ts-sdk-verifier-programsmoduleversionforprotocol</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:49</code> | <code>export function programsModuleVersionForProtocol(</code> |
| <code>interface.9.ts-sdk-verifier-protocolreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:206</code> | <code>export interface ProtocolReceipt</code> |
| <code>interface.9.ts-sdk-verifier-protocolselection</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:29</code> | <code>export interface ProtocolSelection</code> |
| <code>interface.9.ts-sdk-verifier-protocolversionusesoccupancy</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:37</code> | <code>export function protocolVersionUsesOccupancy(version: number): boolean</code> |
| <code>interface.9.ts-sdk-verifier-receipteffect</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:169</code> | <code>export interface ReceiptEffect</code> |
| <code>interface.9.ts-sdk-verifier-receiptverification</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:247</code> | <code>export interface ReceiptVerification</code> |
| <code>interface.9.ts-sdk-verifier-receiptverificationerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:254</code> | <code>export class ReceiptVerificationError extends PlatformSdkError</code> |
| <code>interface.9.ts-sdk-verifier-selectableprotocolversion</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:27</code> | <code>export type SelectableProtocolVersion = typeof DEFAULT_PROTOCOL_VERSION &#124; typeof STATE_COMMITMENT_PROTOCOL_VERSION;</code> |
| <code>interface.9.ts-sdk-verifier-selectedprotocolversion</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:41</code> | <code>export function selectedProtocolVersion(selection?: ProtocolSelection): SelectableProtocolVersion</code> |
| <code>interface.9.ts-sdk-verifier-sequencerauthorization</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:91</code> | <code>export interface SequencerAuthorization</code> |
| <code>interface.9.ts-sdk-verifier-state-commitment-protocol-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:22</code> | <code>export const STATE_COMMITMENT_PROTOCOL_VERSION = 3;</code> |
| <code>interface.9.ts-sdk-verifier-supportedprogramguestabi</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:63</code> | <code>export function supportedProgramGuestAbi(abiVersion: number): boolean</code> |
| <code>interface.9.ts-sdk-verifier-verifybatchinclusion</code> | <code>ts_export</code> | <code>retire</code> | <code>services/agent/sdk/typescript/src/verifier.ts:552</code> | <code>export async function verifyBatchInclusion(</code> |
| <code>interface.9.ts-sdk-verifier-verifycheckpoint</code> | <code>ts_export</code> | <code>retire</code> | <code>services/agent/sdk/typescript/src/verifier.ts:633</code> | <code>export async function verifyCheckpoint(</code> |
| <code>interface.9.ts-sdk-verifier-verifymerkleinclusion</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:330</code> | <code>export async function verifyMerkleInclusion(</code> |
| <code>interface.9.ts-sdk-verifier-verifynativeprogramreceiptoutcomev5</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:1122</code> | <code>export async function verifyNativeProgramReceiptOutcomeV5(</code> |
| <code>interface.9.ts-sdk-verifier-verifyprogramlifecyclereceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/verifier.ts:1064</code> | <code>export async function verifyProgramLifecycleReceipt(canonicalReceipt: Uint8Array, expectedActivity: Uint8Array, sequencerPublicKey: Uint8Array): Promise&lt;ReceiptVerification&gt;</code> |
| <code>interface.9.ts-sdk-verifier-verifyreceipt</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/verifier.ts:1168</code> | <code>export async function verifyReceipt(</code> |
| <code>interface.9.ts-sdk-verifier-verifyreceiptoutcome</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/verifier.ts:1076</code> | <code>export async function verifyReceiptOutcome(</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-verifier-authorizedreceiptbatch</code> | notes | Named by stale pending task 11.5.14, 11.5.48, 11.5.50, 11.5.51, 11.5.52, 11.5.54, 11.5.55, 11.5.56. |
| <code>interface.9.ts-sdk-verifier-batchheader</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-checkpointattestation</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-checkpointcertificate</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-checkpointverification</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-checkpointverificationinput</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-decodebatchheader</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-decodeprogramreceiptoutcome</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-default-protocol-version</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-guarantorkey</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-inclusionkind</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-inclusionverification</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-isselectableprotocolversion</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-legacy-protocol-version</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-localsignatureverifier</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-merkleproof</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-programreceiptoutcome</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-programsmoduleversionforprotocol</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-protocolreceipt</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-protocolselection</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-protocolversionusesoccupancy</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-receipteffect</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-receiptverification</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-receiptverificationerror</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-selectableprotocolversion</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-selectedprotocolversion</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-sequencerauthorization</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-state-commitment-protocol-version</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-supportedprogramguestabi</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-verifybatchinclusion</code> | notes | Named by stale pending task 10.5.15. |
| <code>interface.9.ts-sdk-verifier-verifybatchinclusion</code> | overridden_by | override.J44.165 |
| <code>interface.9.ts-sdk-verifier-verifycheckpoint</code> | notes | Named by stale pending task 10.5.15. |
| <code>interface.9.ts-sdk-verifier-verifycheckpoint</code> | overridden_by | override.J44.166 |
| <code>interface.9.ts-sdk-verifier-verifymerkleinclusion</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-verifier-verifynativeprogramreceiptoutcomev5</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-verifyprogramlifecyclereceipt</code> | notes | File is touched by stale pending task 10.5.15, 11.5.55, 6.5.29; the export itself is not named by them. |
| <code>interface.9.ts-sdk-verifier-verifyreceipt</code> | notes | Named by stale pending task 10.5.15. |
| <code>interface.9.ts-sdk-verifier-verifyreceiptoutcome</code> | notes | Named by stale pending task 0.36, 10.5.15, 6.5.29. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.372.1</code> | req | 9372.1 |
| <code>acceptance.9.372.1</code> | scenario | Given the receipt fixture and its authorized batch, when the test calls verifyReceipt, then the result rung is Rung.Submitted and the receipt fields are unchanged. |
| <code>acceptance.9.372.1</code> | test | services/agent/sdk/typescript/test/receipt-fixture.test.ts::verifyReceipt_reports_the_submitted_rung_under_an_authority_key (planned) |
| <code>acceptance.9.372.1</code> | qualification | UNRUN |
| <code>req.9372</code> | ac_1 | verifyReceipt marks a receipt verified under a caller-supplied key as submitted. |
| <code>capability.9.34.2</code> | title | Delete the TypeScript batch inclusion and checkpoint verifiers |
| <code>capability.9.34.2</code> | reqs | 9372.1 |

## wallet

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-wallet-encodenativeregistration</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/wallet.ts:122</code> | <code>export function encodeNativeRegistration(registration:NativeTokenRegistration,actor:string):Uint8Array</code> |
| <code>interface.9.ts-sdk-wallet-nativetokenregistration</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/wallet.ts:31</code> | <code>export interface NativeTokenRegistration</code> |
| <code>interface.9.ts-sdk-wallet-verifiedwalletreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/wallet.ts:38</code> | <code>export interface VerifiedWalletReceipt</code> |
| <code>interface.9.ts-sdk-wallet-walletexecutionoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/wallet.ts:6</code> | <code>export interface WalletExecutionOptions</code> |
| <code>interface.9.ts-sdk-wallet-walletpendingerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/wallet.ts:46</code> | <code>export class WalletPendingError extends Error { public constructor(public readonly activityId:string){super("Activity remains pending");} }</code> |
| <code>interface.9.ts-sdk-wallet-walletrpc</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/wallet.ts:48</code> | <code>export class WalletRpc</code> |
| <code>interface.9.ts-sdk-wallet-walletrpcconfiguration</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/wallet.ts:15</code> | <code>export interface WalletRpcConfiguration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-wallet-encodenativeregistration</code> | notes | File is touched by stale pending task 10.5.41, 6.5.26; the export itself is not named by them. |
| <code>interface.9.ts-sdk-wallet-nativetokenregistration</code> | notes | File is touched by stale pending task 10.5.41, 6.5.26; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-wallet-verifiedwalletreceipt</code> | notes | File is touched by stale pending task 10.5.41, 6.5.26; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-wallet-walletexecutionoptions</code> | notes | File is touched by stale pending task 10.5.41, 6.5.26; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-wallet-walletpendingerror</code> | notes | File is touched by stale pending task 10.5.41, 6.5.26; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-wallet-walletrpc</code> | notes | File is touched by stale pending task 10.5.41, 6.5.26; the export itself is not named by them. |
| <code>interface.9.ts-sdk-wallet-walletrpcconfiguration</code> | notes | File is touched by stale pending task 10.5.41, 6.5.26; the export itself is not named by them. No caller in package tests, examples or consuming packages. |

## web-search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-web-search-contentdigest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:201</code> | <code>export function contentDigest(canonical: Uint8Array): string</code> |
| <code>interface.9.ts-sdk-web-search-decodepayergrant</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:237</code> | <code>export function decodePayerGrant(canonical: Uint8Array): PayerGrant</code> |
| <code>interface.9.ts-sdk-web-search-decodewebcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:205</code> | <code>export function decodeWebContent(canonical: Uint8Array): WebContent</code> |
| <code>interface.9.ts-sdk-web-search-web-content-domain</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:9</code> | <code>export const WEB_CONTENT_DOMAIN = "PAXEERX_WEB_CONTENT_V1";</code> |
| <code>interface.9.ts-sdk-web-search-web-content-fetch</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:10</code> | <code>export const WEB_CONTENT_FETCH = 1;</code> |
| <code>interface.9.ts-sdk-web-search-web-content-search</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:11</code> | <code>export const WEB_CONTENT_SEARCH = 2;</code> |
| <code>interface.9.ts-sdk-web-search-web-search-currencies</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:12</code> | <code>export const WEB_SEARCH_CURRENCIES = ["SID", "PAX", "USDC", "USDL"] as const;</code> |
| <code>interface.9.ts-sdk-web-search-web-search-max-results</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:14</code> | <code>export const WEB_SEARCH_MAX_RESULTS = 10;</code> |
| <code>interface.9.ts-sdk-web-search-web-search-payer-header</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:13</code> | <code>export const WEB_SEARCH_PAYER_HEADER = "LAYERX-PAYER-DID";</code> |
| <code>interface.9.ts-sdk-web-search-webcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:27</code> | <code>export interface WebContent</code> |
| <code>interface.9.ts-sdk-web-search-webcontentbytes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:190</code> | <code>export function webContentBytes(content: { readonly kind: WebContentKind; readonly payload: Uint8Array &#124; string; readonly mediaType: string; readonly text: string }): Uint8Array</code> |
| <code>interface.9.ts-sdk-web-search-webcontentkind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:18</code> | <code>export type WebContentKind = typeof WEB_CONTENT_FETCH &#124; typeof WEB_CONTENT_SEARCH;</code> |
| <code>interface.9.ts-sdk-web-search-webcontentresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:116</code> | <code>export interface WebContentResponse</code> |
| <code>interface.9.ts-sdk-web-search-webfetchresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:107</code> | <code>export interface WebFetchResponse</code> |
| <code>interface.9.ts-sdk-web-search-websearchassetterms</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:68</code> | <code>export interface WebSearchAssetTerms</code> |
| <code>interface.9.ts-sdk-web-search-websearchclient</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/web-search.ts:312</code> | <code>export class WebSearchClient</code> |
| <code>interface.9.ts-sdk-web-search-websearchclientoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:73</code> | <code>export interface WebSearchClientOptions</code> |
| <code>interface.9.ts-sdk-web-search-websearchcurrency</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:16</code> | <code>export type WebSearchCurrency = typeof WEB_SEARCH_CURRENCIES[number];</code> |
| <code>interface.9.ts-sdk-web-search-websearcherror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:20</code> | <code>export class WebSearchError extends Error</code> |
| <code>interface.9.ts-sdk-web-search-websearchexactpayment</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:57</code> | <code>export interface WebSearchExactPayment</code> |
| <code>interface.9.ts-sdk-web-search-websearchmeteredpayment</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:52</code> | <code>export interface WebSearchMeteredPayment</code> |
| <code>interface.9.ts-sdk-web-search-websearchoffer</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:34</code> | <code>export interface WebSearchOffer</code> |
| <code>interface.9.ts-sdk-web-search-websearchpayer</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:61</code> | <code>export interface WebSearchPayer</code> |
| <code>interface.9.ts-sdk-web-search-websearchpreference</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:47</code> | <code>export interface WebSearchPreference</code> |
| <code>interface.9.ts-sdk-web-search-websearchresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:102</code> | <code>export interface WebSearchResponse</code> |
| <code>interface.9.ts-sdk-web-search-websearchresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:96</code> | <code>export interface WebSearchResult</code> |
| <code>interface.9.ts-sdk-web-search-websearchscheme</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/web-search.ts:17</code> | <code>export type WebSearchScheme = "metered" &#124; "exact";</code> |
| <code>interface.9.ts-sdk-web-search-websettlement</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/web-search.ts:85</code> | <code>export interface WebSettlement</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-web-search-contentdigest</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. |
| <code>interface.9.ts-sdk-web-search-decodepayergrant</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. |
| <code>interface.9.ts-sdk-web-search-decodewebcontent</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. |
| <code>interface.9.ts-sdk-web-search-web-content-domain</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-web-content-fetch</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-web-content-search</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-web-search-currencies</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-web-search-max-results</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-web-search-payer-header</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-webcontent</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-webcontentbytes</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. |
| <code>interface.9.ts-sdk-web-search-webcontentkind</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-webcontentresponse</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-webfetchresponse</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websearchassetterms</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websearchclient</code> | notes | Named by stale pending task 5.5.3, 6.5.34. |
| <code>interface.9.ts-sdk-web-search-websearchclientoptions</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websearchcurrency</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. |
| <code>interface.9.ts-sdk-web-search-websearcherror</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. |
| <code>interface.9.ts-sdk-web-search-websearchexactpayment</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websearchmeteredpayment</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websearchoffer</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. |
| <code>interface.9.ts-sdk-web-search-websearchpayer</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. |
| <code>interface.9.ts-sdk-web-search-websearchpreference</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websearchresponse</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websearchresult</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websearchscheme</code> | notes | File is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-web-search-websettlement</code> | notes | Name carries a surface the merge design retargets; file is touched by stale pending task 10.5.14, 11.5.56, 5.5.3, 6.5.29, 6.5.34. No caller in package tests, examples or consuming packages. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.373.1</code> | req | 9373.1 |
| <code>acceptance.9.373.1</code> | scenario | Given a seller stub answering a metered search with a signed sequencer receipt for a draw transaction, when the test runs a search, then settlement.transaction equals keccak256 of the raw draw transaction. |
| <code>acceptance.9.373.1</code> | test | services/agent/sdk/typescript/test/web-search-sequencer.test.ts::web_settlement_transaction_is_the_draw_transaction_hash (planned) |
| <code>acceptance.9.373.1</code> | qualification | UNRUN |
| <code>req.9373</code> | ac_1 | WebSettlement.transaction is the draw transaction hash. |
| <code>acceptance.9.373.2</code> | req | 9373.2 |
| <code>acceptance.9.373.2</code> | scenario | Given the same stub signing with a key outside receiptKeys, when the test runs a search, then it throws WebSearchError. |
| <code>acceptance.9.373.2</code> | test | services/agent/sdk/typescript/test/web-search-sequencer.test.ts::web_search_client_verifies_the_payment_with_pinned_receipt_keys (planned) |
| <code>acceptance.9.373.2</code> | qualification | UNRUN |
| <code>req.9373</code> | ac_2 | The client refuses a settlement receipt signed by an unpinned key. |
| <code>capability.9.34.3</code> | title | Settle web search payments with draw transactions and sequencer receipts |
| <code>capability.9.34.3</code> | reqs | 9373.1<br>9373.2 |

## x402-activity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-x402-activity-binddrawtransaction-j44</code> | <code>ts_export</code> | <code>new</code> | <code>services/agent/sdk/typescript/src/x402/activity.ts:1</code> | <code>export function bindDrawTransaction(raw: Uint8Array, expected: { from: string; assetId: string; amount: bigint; grantId: string; idempotencyKey: string; chainId: bigint }): string</code> |
| <code>interface.9.ts-sdk-x402-activity-bindreceiveactivity</code> | <code>ts_export</code> | <code>retire</code> | <code>services/agent/sdk/typescript/src/x402/activity.ts:3</code> | <code>export function bindReceiveActivity(canonical: Uint8Array, receive: Uint8Array, actor: string, network: number, key: string): string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-x402-activity-binddrawtransaction-j44</code> | notes | Planned; no caller exists at HEAD. |
| <code>interface.9.ts-sdk-x402-activity-bindreceiveactivity</code> | notes | Named by stale pending task 10.5.12, 6.5.33. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-activity-bindreceiveactivity</code> | overridden_by | override.J44.164 |

## x402-draw

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-x402-draw-preparedgrantdraws</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/x402/draw.ts:6</code> | <code>export class PreparedGrantDraws</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-x402-draw-preparedgrantdraws</code> | notes | Named by stale pending task 10.5.11, 10.5.20, 6.5.33. No caller in package tests, examples or consuming packages. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.375.1</code> | req | 9375.1 |
| <code>acceptance.9.375.1</code> | scenario | Given a registered raw draw transaction and an rpc stub answering sendRawTransaction and sequencerReceipt with a pinned-key receipt, when the test executes the draw twice with the same idempotency key, then sendRawTransaction is called once and both results carry the same hash. |
| <code>acceptance.9.375.1</code> | test | services/agent/sdk/typescript/test/x402-draw-sequencer.test.ts::prepared_draw_submits_a_raw_draw_transaction (planned) |
| <code>acceptance.9.375.1</code> | qualification | UNRUN |
| <code>req.9375</code> | ac_1 | PreparedGrantDraws.execute submits the registered raw draw transaction once. |
| <code>acceptance.9.375.2</code> | req | 9375.2 |
| <code>acceptance.9.375.2</code> | scenario | Given an offer of at most 10 units and draw calldata for 11, when the test validates it, then it throws. |
| <code>acceptance.9.375.2</code> | test | services/agent/sdk/typescript/test/x402-draw-sequencer.test.ts::grant_draw_validation_checks_the_draw_calldata_against_the_offer (planned) |
| <code>acceptance.9.375.2</code> | qualification | UNRUN |
| <code>req.9375</code> | ac_2 | validateGrantDraw refuses draw calldata whose amount exceeds the offer. |
| <code>capability.9.34.5</code> | title | Submit x402 grant draws as raw transactions verified by sequencer receipts |
| <code>capability.9.34.5</code> | reqs | 9375.1<br>9375.2 |

## x402-grant

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-x402-grant-grantoffer</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/grant.ts:3</code> | <code>export interface GrantOffer</code> |
| <code>interface.9.ts-sdk-x402-grant-validategrantdraw</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/x402/grant.ts:16</code> | <code>export function validateGrantDraw(wire: Uint8Array, offer: GrantOffer, idempotencyKey: string, networkId: number, now: bigint): Receive</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-x402-grant-grantoffer</code> | notes | File is touched by stale pending task 10.5.11, 6.5.33; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-grant-validategrantdraw</code> | notes | Named by stale pending task 10.5.11, 6.5.33. No caller in package tests, examples or consuming packages. |

## x402-receive

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-x402-receive-assetregistration</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:44</code> | <code>export interface AssetRegistration</code> |
| <code>interface.9.ts-sdk-x402-receive-decodereceive</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:172</code> | <code>export function decodeReceive(bytes: Uint8Array): Receive</code> |
| <code>interface.9.ts-sdk-x402-receive-derivenativeassetid</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:114</code> | <code>export function deriveNativeAssetId(issuerDidId32: string, salt: string): string</code> |
| <code>interface.9.ts-sdk-x402-receive-encodeaccountopen</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:191</code> | <code>export function encodeAccountOpen(asset: string): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodeassetregister</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:122</code> | <code>export function encodeAssetRegister(issuerDidId32: string, registration: AssetRegistration): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodeassetsupply</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:199</code> | <code>export function encodeAssetSupply(asset: string, account: string, amount: string): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodegrant</code> | <code>ts_export</code> | <code>change</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:151</code> | <code>export function encodeGrant(grant: PayerGrant): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodegrantrevoke</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:195</code> | <code>export function encodeGrantRevoke(grant_id: string, revocation_sequence: string): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-encodereceive</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:161</code> | <code>export function encodeReceive(receive: Receive): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-grantauthorizationmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:156</code> | <code>export function grantAuthorizationMessage(grant: PayerGrant): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-payergrant</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:3</code> | <code>export interface PayerGrant</code> |
| <code>interface.9.ts-sdk-x402-receive-receive</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:31</code> | <code>export interface Receive</code> |
| <code>interface.9.ts-sdk-x402-receive-receiveauthorizationmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:167</code> | <code>export function receiveAuthorizationMessage(receive: Receive): Uint8Array</code> |
| <code>interface.9.ts-sdk-x402-receive-receiverauthorization</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/x402/receive.ts:21</code> | <code>export interface ReceiverAuthorization</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-x402-receive-assetregistration</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-decodereceive</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-derivenativeassetid</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-encodeaccountopen</code> | notes | Named by stale pending task 10.5.12. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-encodeassetregister</code> | notes | Named by stale pending task 10.5.12. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-encodeassetsupply</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-encodegrant</code> | notes | Named by stale pending task 10.5.12. |
| <code>interface.9.ts-sdk-x402-receive-encodegrantrevoke</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-encodereceive</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-grantauthorizationmessage</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-payergrant</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-receive</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-receiveauthorizationmessage</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-x402-receive-receiverauthorization</code> | notes | File is touched by stale pending task 10.5.12; the export itself is not named by them. No caller in package tests, examples or consuming packages. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.374.1</code> | req | 9374.1 |
| <code>acceptance.9.374.1</code> | scenario | Given a fixture PayerGrant, when the test builds the call, then to is 0x101A and data ABI-decodes to encodeGrant of the grant. |
| <code>acceptance.9.374.1</code> | test | services/agent/sdk/typescript/test/x402-ledger-calls.test.ts::grant_encodes_issueGrant_calldata_over_the_payer_grant (planned) |
| <code>acceptance.9.374.1</code> | qualification | UNRUN |
| <code>req.9374</code> | ac_1 | issueGrantCall wraps the encoded payer grant in issueGrant(bytes). |
| <code>acceptance.9.374.2</code> | req | 9374.2 |
| <code>acceptance.9.374.2</code> | scenario | Given an asset id and a registration, when the test builds both calls, then their data start with the openAccount(bytes32) and register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes) selectors. |
| <code>acceptance.9.374.2</code> | test | services/agent/sdk/typescript/test/x402-ledger-calls.test.ts::account_open_and_asset_register_encode_layerxledger_calldata (planned) |
| <code>acceptance.9.374.2</code> | qualification | UNRUN |
| <code>req.9374</code> | ac_2 | openAccountCall and registerAssetCall encode the layerxledger selectors. |
| <code>acceptance.9.374.3</code> | req | 9374.3 |
| <code>acceptance.9.374.3</code> | scenario | Given a raw draw transaction for amount 10 and expectations for amounts 10 and 11, when the test binds each, then the first returns keccak256 of the raw bytes and the second throws. |
| <code>acceptance.9.374.3</code> | test | services/agent/sdk/typescript/test/x402-ledger-calls.test.ts::bindDrawTransaction_binds_a_raw_draw_transaction_and_returns_its_hash (planned) |
| <code>acceptance.9.374.3</code> | qualification | UNRUN |
| <code>req.9374</code> | ac_3 | bindDrawTransaction accepts the matching draw and refuses a changed amount. |
| <code>capability.9.34.4</code> | title | Build x402 grants, account opens and asset registrations as layerxledger calldata |
| <code>capability.9.34.4</code> | reqs | 9374.1<br>9374.2<br>9374.3 |

## xweb-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.ts-sdk-xweb-api-buildxwebapirequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:659</code> | <code>export function buildXWebApiRequest(</code> |
| <code>interface.9.ts-sdk-xweb-api-decodexwebapipayload</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:532</code> | <code>export function decodeXWebApiPayload(raw: Uint8Array): XWebApiPayload</code> |
| <code>interface.9.ts-sdk-xweb-api-decodexwebattestors</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:785</code> | <code>export function decodeXWebAttestors(answer: string): XWebAttestorSet</code> |
| <code>interface.9.ts-sdk-xweb-api-encodexwebapipayload</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:465</code> | <code>export function encodeXWebApiPayload(payload: XWebApiPayload): Uint8Array</code> |
| <code>interface.9.ts-sdk-xweb-api-encodexwebcredential</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:565</code> | <code>export function encodeXWebCredential(headers: readonly XWebApiHeader[], publicHeaders: readonly XWebApiHeader[] = []): Uint8Array</code> |
| <code>interface.9.ts-sdk-xweb-api-sealxwebenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:593</code> | <code>export function sealXWebEnvelope(</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-api-limits</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:31</code> | <code>export const XWEB_API_LIMITS =</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-api-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:23</code> | <code>export const XWEB_API_VERSION = 1;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-default-max-payload-bytes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:29</code> | <code>export const XWEB_DEFAULT_MAX_PAYLOAD_BYTES = 8192;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-info</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:28</code> | <code>export const XWEB_ENVELOPE_INFO = "PAXEERX_WEB_API_ENVELOPE_V1";</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-key-length</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:44</code> | <code>export const XWEB_ENVELOPE_KEY_LENGTH = 33;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-nonce-length</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:45</code> | <code>export const XWEB_ENVELOPE_NONCE_LENGTH = 12;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-overhead</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:47</code> | <code>export const XWEB_ENVELOPE_OVERHEAD = 20 + XWEB_ENVELOPE_KEY_LENGTH + XWEB_ENVELOPE_NONCE_LENGTH + XWEB_ENVELOPE_TAG_LENGTH;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-tag-length</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:46</code> | <code>export const XWEB_ENVELOPE_TAG_LENGTH = 16;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-kind-api</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:22</code> | <code>export const XWEB_KIND_API = 3;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-level-majority</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:26</code> | <code>export const XWEB_LEVEL_MAJORITY = 0;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-level-single</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:27</code> | <code>export const XWEB_LEVEL_SINGLE = 1;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-max-envelope-bytes</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:48</code> | <code>export const XWEB_MAX_ENVELOPE_BYTES = XWEB_ENVELOPE_OVERHEAD + XWEB_API_LIMITS.credentialBytes;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-method-get</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:24</code> | <code>export const XWEB_METHOD_GET = 1;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-method-post</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:25</code> | <code>export const XWEB_METHOD_POST = 2;</code> |
| <code>interface.9.ts-sdk-xweb-api-xweb-precompile</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:21</code> | <code>export const XWEB_PRECOMPILE = "0x0000000000000000000000000000000000001019";</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapibuildoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:117</code> | <code>export interface XWebApiBuildOptions</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapicall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:108</code> | <code>export interface XWebApiCall</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapierror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:59</code> | <code>export class XWebApiError extends Error</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapierrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:50</code> | <code>export type XWebApiErrorCode =</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapiheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:71</code> | <code>export interface XWebApiHeader</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapimethod</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:69</code> | <code>export type XWebApiMethod = "GET" &#124; "POST";</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapiorigin</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:240</code> | <code>export function xwebApiOrigin(url: string): string</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapipayload</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:77</code> | <code>export interface XWebApiPayload</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapirequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:131</code> | <code>export interface XWebApiRequest</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebapirequestcall</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:720</code> | <code>export function xwebApiRequestCall(payload: Uint8Array, callbackGas: bigint, fee: bigint): PrecompileCall</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebattestor</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:89</code> | <code>export interface XWebAttestor</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebattestoraddress</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:368</code> | <code>export function xwebAttestorAddress(publicKey: Uint8Array &#124; string): string</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebattestorset</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:96</code> | <code>export interface XWebAttestorSet</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebenveloperandomness</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:102</code> | <code>export interface XWebEnvelopeRandomness</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebgetattestorscalldata</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:740</code> | <code>export function xwebGetAttestorsCallData(): string</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebmethodcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:206</code> | <code>export function xwebMethodCode(method: XWebApiMethod): number</code> |
| <code>interface.9.ts-sdk-xweb-api-xwebsealedenvelope</code> | <code>ts_export</code> | <code>stable</code> | <code>services/agent/sdk/typescript/src/xweb-api.ts:126</code> | <code>export interface XWebSealedEnvelope</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.ts-sdk-xweb-api-buildxwebapirequest</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-decodexwebapipayload</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-decodexwebattestors</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-encodexwebapipayload</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-encodexwebcredential</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-sealxwebenvelope</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xweb-api-limits</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-api-version</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-default-max-payload-bytes</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-info</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-key-length</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-nonce-length</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-overhead</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-envelope-tag-length</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-kind-api</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xweb-level-majority</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xweb-level-single</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xweb-max-envelope-bytes</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-method-get</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-method-post</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xweb-precompile</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebapibuildoptions</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebapicall</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebapierror</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebapierrorcode</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xwebapiheader</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebapimethod</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xwebapiorigin</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebapipayload</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xwebapirequest</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebapirequestcall</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebattestor</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xwebattestoraddress</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebattestorset</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebenveloperandomness</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebgetattestorscalldata</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. |
| <code>interface.9.ts-sdk-xweb-api-xwebmethodcode</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
| <code>interface.9.ts-sdk-xweb-api-xwebsealedenvelope</code> | notes | File is touched by stale pending task 10.5.41; the export itself is not named by them. No caller in package tests, examples or consuming packages. |
