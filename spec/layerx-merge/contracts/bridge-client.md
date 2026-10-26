# Bridge client contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## amounts

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.483.1</code> | req | 14483.1 |
| <code>acceptance.14.483.1</code> | scenario | Given parseAmount at apps/bridge/src/lib/bridge/amount.ts, when it is called with "1,5" and 18 decimals, then it returns undefined |
| <code>acceptance.14.483.1</code> | test | apps/bridge/src/lib/bridge/amount.spec.ts::parseAmountRefusesACommaDecimalSeparator (planned) |
| <code>acceptance.14.483.1</code> | qualification | UNRUN |
| <code>req.14483</code> | ac_1 | parseAmount("1,5", 18) returns undefined; at HEAD apps/bridge/src/lib/bridge/amount.ts:11 strips every comma and returns 15 units. |
| <code>acceptance.14.483.2</code> | req | 14483.2 |
| <code>acceptance.14.483.2</code> | scenario | Given parseAmount, when it is called with "1,234.5" and 6 decimals, then it returns 1234500000n, and with "12,34" it returns undefined |
| <code>acceptance.14.483.2</code> | test | apps/bridge/src/lib/bridge/amount.spec.ts::parseAmountAcceptsGroupedThousands (planned) |
| <code>acceptance.14.483.2</code> | qualification | UNRUN |
| <code>req.14483</code> | ac_2 | parseAmount("1,234.5", 6) returns 1234500000n: commas are accepted only as thousands separators in groups of three before the decimal point (apps/bridge/src/lib/bridge/amount.ts:10). |
| <code>capability.14.42.1</code> | title | Refuse a comma decimal separator in parseAmount instead of reading 1,5 as 15 |
| <code>capability.14.42.1</code> | reqs | 14483.1<br>14483.2 |

## app

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-app-layout-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/app/layout.tsx:36</code> | <code>export default function RootLayout({ children }: LayoutProps&lt;"/"&gt;)</code> |
| <code>interface.14.ts-bridge-app-layout-metadata</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/app/layout.tsx:17</code> | <code>export const metadata: Metadata =</code> |
| <code>interface.14.ts-bridge-app-layout-viewport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/app/layout.tsx:24</code> | <code>export const viewport: Viewport =</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-app-layout-default</code> | notes | Next.js framework entry (layout) for / |
| <code>interface.14.ts-bridge-app-layout-metadata</code> | notes | Next.js framework entry (layout) for / |
| <code>interface.14.ts-bridge-app-layout-viewport</code> | notes | Next.js framework entry (layout) for / |

## attestation-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.fmt-bridge-attestation-bundle-fixture-j55</code> | <code>file_format</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/fixtures/deposit-ready.json</code> (planned) | <code>{chainId, vault, txHash, logIndex, recipient, asset, amount, digest, signatures[], status}</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationpollinterval-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> (planned) | <code>export function attestationPollInterval(data: { status: AttestationStatus; signatures: readonly Hex[] } &#124; undefined, periodMs: number): number &#124; false</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-depositattestationpath-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> (planned) | <code>export function depositAttestationPath(args: { chainId: bigint; txHash: Hex; logIndex: bigint }): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-parsedepositattestation-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> (planned) | <code>export function parseDepositAttestation(request: { chainId: bigint; txHash: Hex; logIndex: bigint }, body: unknown): DepositAttestation</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-parsereleaseattestation-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> (planned) | <code>export function parseReleaseAttestation(request: { chainId: bigint; paxeerTxHash: Hex; nonce: bigint }, body: unknown): ReleaseAttestation</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-releaseattestationpath-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> (planned) | <code>export function releaseAttestationPath(args: { chainId: bigint; paxeerTxHash: Hex; paxeerNonce: bigint }): string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.fmt-bridge-attestation-bundle-fixture-j55</code> | notes | created by task 14.44.4; response shape the relayer serves |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationpollinterval-j55</code> | notes | created by task 14.44.3; polling rule shared by both attestation hooks |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-depositattestationpath-j55</code> | notes | created by task 14.44.4; GET route of a deposit bundle |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-parsedepositattestation-j55</code> | notes | created by task 14.44.1; refuses a body for another deposit |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-parsereleaseattestation-j55</code> | notes | created by task 14.44.1; refuses a body for another release |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-releaseattestationpath-j55</code> | notes | created by task 14.44.4; GET route of a release bundle |

Contract context <code>contract_context.14.bridge-client</code>:

| Field | Frozen rule |
|---|---|
| interface | apps/bridge client: deposits call PaxeerXVault.deposit and depositNative on the foreign chain, withdrawals call bridgeOut on 0x1016, claims and releases are built from attestation bundles fetched from /v1/deposit and /v1/release and verified locally before the wallet opens. |
| storage | Browser localStorage key paxeer-bridge:transfers:v1 holding BridgeTransfer rows (apps/bridge/src/lib/bridge/transfers.ts:47); no server state. |
| errors | LXP_BRIDGE_UNAUTHORIZED, LXP_BRIDGE_UNKNOWN_CHAIN, LXP_BRIDGE_CHAIN_DISABLED, LXP_BRIDGE_UNKNOWN_ASSET, LXP_BRIDGE_PAUSED, LXP_BRIDGE_NULLIFIED, LXP_BRIDGE_CAP_EXCEEDED, LXP_BRIDGE_BAD_SIGNATURE, LXP_BRIDGE_BELOW_THRESHOLD, LXP_BRIDGE_VAULT_MISMATCH, LXP_BRIDGE_INVALID_REQUEST mapped to sentences; vault custom errors decoded with paxeerXVaultAbi. |
| events | Reads BridgeDeposit and BridgeRelease of PaxeerXVault and BridgeOut of 0x1016 from receipts. |
| gas | None set by the client; the wallet estimates gas. The Max amount keeps a native gas reserve. |
| activation | Routes open per chain when NEXT_PUBLIC_BRIDGE_VAULT_&lt;CHAIN&gt; is set and the chain is registered and enabled on 0x1016; claim and release need NEXT_PUBLIC_BRIDGE_API_URL. |
| invariants | The client never submits a claim or release whose digest it has not recomputed and whose signatures do not meet the threshold; the attested recipient, asset and chain equal the local record. |
| pause | A paused precompile or vault disables submission and shows the paused blocker. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.485.1</code> | req | 14485.1 |
| <code>acceptance.14.485.1</code> | scenario | Given a deposit request for chain 1 and a body with chainId "8453", when parseDepositAttestation runs, then it throws AttestationApiError naming chainId |
| <code>acceptance.14.485.1</code> | test | apps/bridge/src/lib/bridge/attestation-api.spec.ts::parseDepositAttestationRefusesAnotherChainId (planned) |
| <code>acceptance.14.485.1</code> | qualification | UNRUN |
| <code>req.14485</code> | ac_1 | parseDepositAttestation(request, body) throws AttestationApiError when body.chainId differs from request.chainId; at HEAD apps/bridge/src/lib/bridge/attestation-api.ts:141-154 copies the body without comparing it. |
| <code>acceptance.14.485.2</code> | req | 14485.2 |
| <code>acceptance.14.485.2</code> | scenario | Given a deposit request with logIndex 7 and a body with logIndex "8", when parseDepositAttestation runs, then it throws AttestationApiError naming logIndex |
| <code>acceptance.14.485.2</code> | test | apps/bridge/src/lib/bridge/attestation-api.spec.ts::parseDepositAttestationRefusesAnotherLogIndex (planned) |
| <code>acceptance.14.485.2</code> | qualification | UNRUN |
| <code>req.14485</code> | ac_2 | parseDepositAttestation throws when body.txHash or body.logIndex differs from the request (apps/bridge/src/lib/bridge/attestation-api.ts:132). |
| <code>acceptance.14.485.3</code> | req | 14485.3 |
| <code>acceptance.14.485.3</code> | scenario | Given a release request with nonce 3 and a body with paxeerNonce "4", when parseReleaseAttestation runs, then it throws AttestationApiError naming nonce |
| <code>acceptance.14.485.3</code> | test | apps/bridge/src/lib/bridge/attestation-api.spec.ts::parseReleaseAttestationRefusesAnotherNonce (planned) |
| <code>acceptance.14.485.3</code> | qualification | UNRUN |
| <code>req.14485</code> | ac_3 | parseReleaseAttestation throws when body.chainId, body.paxeerTxHash or body.paxeerNonce differs from the request; at HEAD apps/bridge/src/lib/bridge/attestation-api.ts:167-179 copies the body unchecked. |
| <code>acceptance.14.486.1</code> | req | 14486.1 |
| <code>acceptance.14.486.1</code> | scenario | Given a DepositAttestation with status ready and signatures [], when attestationPollInterval runs, then it returns 12000, the POLL_MS at apps/bridge/src/lib/bridge/use-bridge-state.ts:28 |
| <code>acceptance.14.486.1</code> | test | apps/bridge/src/lib/bridge/attestation-api.spec.ts::pollIntervalContinuesForAReadyBundleWithoutSignatures (planned) |
| <code>acceptance.14.486.1</code> | qualification | UNRUN |
| <code>req.14486</code> | ac_1 | attestationPollInterval returns the poll period for status ready with an empty signatures list; at HEAD apps/bridge/src/lib/bridge/use-bridge-state.ts:230 stops on status ready alone. |
| <code>acceptance.14.486.2</code> | req | 14486.2 |
| <code>acceptance.14.486.2</code> | scenario | Given a ReleaseAttestation with status ready and one signature, when attestationPollInterval runs, then it returns false |
| <code>acceptance.14.486.2</code> | test | apps/bridge/src/lib/bridge/attestation-api.spec.ts::pollIntervalStopsForASignedReadyBundle (planned) |
| <code>acceptance.14.486.2</code> | qualification | UNRUN |
| <code>req.14486</code> | ac_2 | attestationPollInterval returns false for status ready with at least one signature (apps/bridge/src/lib/bridge/use-bridge-state.ts:250). |
| <code>acceptance.14.487.1</code> | req | 14487.1 |
| <code>acceptance.14.487.1</code> | scenario | Given chainId 8453, an upper-case hash and logIndex 3, when depositAttestationPath runs, then it returns /v1/deposit/8453/&lt;lowercase hash&gt;/3 |
| <code>acceptance.14.487.1</code> | test | apps/bridge/src/lib/bridge/attestation-routes.spec.ts::depositAttestationPathLowercasesTheHash (planned) |
| <code>acceptance.14.487.1</code> | qualification | UNRUN |
| <code>req.14487</code> | ac_1 | depositAttestationPath({chainId, txHash, logIndex}) returns /v1/deposit/&lt;chainId&gt;/&lt;lowercase txHash&gt;/&lt;logIndex&gt;, the path built inline at apps/bridge/src/lib/bridge/attestation-api.ts:137-139. |
| <code>acceptance.14.487.2</code> | req | 14487.2 |
| <code>acceptance.14.487.2</code> | scenario | Given the two fixture files under src/lib/bridge/fixtures, when they are parsed, then both return status ready and a non-empty signatures list |
| <code>acceptance.14.487.2</code> | test | apps/bridge/src/lib/bridge/attestation-routes.spec.ts::theCommittedFixturesParseAsReadyBundles (planned) |
| <code>acceptance.14.487.2</code> | qualification | UNRUN |
| <code>req.14487</code> | ac_2 | The committed fixtures deposit-ready.json and release-ready.json parse through the deposit and release parsers of apps/bridge/src/lib/bridge/attestation-api.ts:131-170 into status ready bundles with at least one signature. |
| <code>capability.14.44.1</code> | title | Parse attestation responses through exported parsers that refuse another deposit or release |
| <code>capability.14.44.1</code> | reqs | 14485.1<br>14485.2<br>14485.3 |
| <code>capability.14.44.3</code> | title | Keep polling a ready attestation bundle until it carries signatures |
| <code>capability.14.44.3</code> | reqs | 14486.1<br>14486.2 |
| <code>capability.14.44.4</code> | title | Freeze the attestation API routes and a response fixture the relayer must serve |
| <code>capability.14.44.4</code> | reqs | 14487.1<br>14487.2 |

## components

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-components-bridge-bridge-card-bridgecard</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/components/bridge/bridge-card.tsx:61</code> | <code>export function BridgeCard({ onTransferSubmitted }: { onTransferSubmitted: () =&gt; void })</code> |
| <code>interface.14.ts-bridge-components-bridge-how-it-works-howitworks</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/how-it-works.tsx:30</code> | <code>export function HowItWorks()</code> |
| <code>interface.14.ts-bridge-components-bridge-providers-providers</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/providers.tsx:12</code> | <code>export function Providers({ children }: { children: ReactNode })</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-amountinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx:170</code> | <code>export function AmountInput({ id, value, onChange, symbol, balance, decimals, balanceLoading, invalid, disabled, onMax, }:</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-assetpicker</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx:268</code> | <code>export function AssetPicker({ symbols, value, onChange, label, }: { symbols: string[]; value: string; onChange: (symbol: string) =&gt; void; label: string; })</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-receivepreview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx:236</code> | <code>export function ReceivePreview({ symbol, amountText, denom, }: { symbol: string; amountText?: string; denom?: string; })</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-routeoption</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx:17</code> | <code>export type RouteOption =</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-routepanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx:54</code> | <code>export function RoutePanel({ label, option, options, onSelect, pickerTitle, children, }: { label: string; option: RouteOption; options: RouteOption[]; onSelect: (key: string) =&gt; void;</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-swapdirectionbutton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx:153</code> | <code>export function SwapDirectionButton({ onClick, disabled }: { onClick: () =&gt; void; disabled?: boolean })</code> |
| <code>interface.14.ts-bridge-components-bridge-site-header-siteheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/site-header.tsx:175</code> | <code>export function SiteHeader({ historyBadge }: { historyBadge?: number })</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-bridgeoutnoncefromreceipt</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts:251</code> | <code>export function bridgeOutNonceFromReceipt(logs: readonly { address: string; topics: readonly string[]; data: string }[]): bigint &#124; undefined</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-buildverifiedclaim</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts:84</code> | <code>export async function buildVerifiedClaim( transfer: BridgeTransfer, attestation: DepositAttestation, ): Promise&lt;VerifiedClaim&gt;</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-buildverifiedrelease</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts:168</code> | <code>export async function buildVerifiedRelease( transfer: BridgeTransfer, attestation: ReleaseAttestation, ): Promise&lt;VerifiedRelease&gt;</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-readtransferlive</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts:37</code> | <code>export async function readTransferLive(transfer: BridgeTransfer): Promise&lt;TransferLive&gt;</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-transferlive</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts:28</code> | <code>export type TransferLive =</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-transferphase</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts:20</code> | <code>export type TransferPhase =</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-verifiedclaim</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts:74</code> | <code>export type VerifiedClaim =</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-verifiedrelease</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts:158</code> | <code>export type VerifiedRelease =</code> |
| <code>interface.14.ts-bridge-components-bridge-transfers-panel-transferspanel</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/components/bridge/transfers-panel.tsx:261</code> | <code>export function TransfersPanel({ refreshKey }: { refreshKey: number })</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-createthemetransition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx:68</code> | <code>function createThemeTransition({ variant = "circle", start = "pointer", blur = false, duration, gifUrl }: ThemeTransitionOptions, origin = "50% 50%")</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetoggle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx:256</code> | <code>function ThemeToggle({ icon = "sun", appearance = "ghost", variant = "circle", start = "pointer", blur, duration, gifUrl, label = "Toggle theme", className, onClick, ...props</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetoggleicon</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx:244</code> | <code>type ThemeToggleIcon = keyof typeof icons;</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetoggleprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx:248</code> | <code>type ThemeToggleProps = Omit&lt;React.ComponentProps&lt;"button"&gt;, "children"&gt; &amp;</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx:32</code> | <code>type ThemeTransitionOptions =</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionstart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx:18</code> | <code>type ThemeTransitionStart =</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionvariant</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx:17</code> | <code>type ThemeTransitionVariant = "none" &#124; "circle" &#124; "circle-blur" &#124; "rectangle" &#124; "polygon" &#124; "gif";</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-usethemetransition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx:111</code> | <code>function useThemeTransition(options: ThemeTransitionOptions = {})</code> |
| <code>interface.14.ts-bridge-components-theme-provider-themeprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/theme-provider.tsx:22</code> | <code>export function ThemeProvider({ children }: { children: React.ReactNode })</code> |
| <code>interface.14.ts-bridge-components-theme-provider-usetheme</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/theme-provider.tsx:67</code> | <code>export function useTheme()</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx:50</code> | <code>export function ChainList({ chains, value, onSelect, recent = [], tab, defaultTab, onTabChange, wrongNetwork, variant = "wide", className }: ChainListProps)</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainlistprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx:32</code> | <code>export interface ChainListProps</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainoption</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx:21</code> | <code>export type ChainOption =</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainselector</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx:161</code> | <code>export function ChainSelector({ chains, value, defaultValue, onValueChange, open, defaultOpen = false, onOpenChange, wrongNetwork, loading, layout = "popover", showName = false,</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainselectorprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx:140</code> | <code>export interface ChainSelectorProps extends Omit&lt;ChainListProps, "onSelect" &#124; "value" &#124; "variant" &#124; "className"&gt;</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chaintab</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx:30</code> | <code>export type ChainTab = "mainnet" &#124; "testnet";</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-depositstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx:128</code> | <code>export function DepositStatus({ items, open, defaultOpen = false, onOpenChange, onDismiss, explorerUrl, footer, className }: DepositStatusProps)</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-depositstatusprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx:43</code> | <code>export interface DepositStatusProps</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstatusgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx:41</code> | <code>export type TransferStatusGroup = "pending" &#124; "failed" &#124; "completed";</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstatusitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx:23</code> | <code>export type TransferStatusItem =</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstep</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx:21</code> | <code>export type TransferStep = "submitted" &#124; "confirming" &#124; "credited" &#124; "failed";</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transfersteps</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx:74</code> | <code>export function TransferSteps({ item }: { item: TransferStatusItem })</code> |
| <code>interface.14.ts-bridge-components-trading-numeral-numeral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/numeral.tsx:36</code> | <code>export function Numeral({ value, rule = "number", dp, tick, coloring, showSign, pad, prefix, unit, placeholder = "-", hidden,</code> |
| <code>interface.14.ts-bridge-components-trading-numeral-numeralprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/numeral.tsx:6</code> | <code>export interface NumeralProps</code> |
| <code>interface.14.ts-bridge-components-trading-numeral-statistic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/numeral.tsx:77</code> | <code>export function Statistic({ label, children, align = "start", className }: StatisticProps)</code> |
| <code>interface.14.ts-bridge-components-trading-numeral-statisticprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/numeral.tsx:68</code> | <code>export interface StatisticProps</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-chainicon</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/token-icon.tsx:67</code> | <code>export function ChainIcon({ name, ...props }: ChainIconProps)</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-chainiconprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/token-icon.tsx:61</code> | <code>export interface ChainIconProps extends Omit&lt;TokenIconProps, "symbol" &#124; "shape"&gt;</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-symbolbadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/token-icon.tsx:102</code> | <code>export function SymbolBadge({ children, className }: { children: ReactNode; className?: string })</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-symboldisplay</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/token-icon.tsx:86</code> | <code>export function SymbolDisplay({ symbol, format = "base-type", showIcon = true, iconSrc, size = "xs", badge, className }: SymbolDisplayProps)</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-symboldisplayprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/token-icon.tsx:71</code> | <code>export interface SymbolDisplayProps</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-tokenicon</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/token-icon.tsx:36</code> | <code>export function TokenIcon({ symbol, src, size = "sm", shape = "circle", className }: TokenIconProps)</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-tokeniconprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/token-icon.tsx:24</code> | <code>export interface TokenIconProps</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-tokeniconsize</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/token-icon.tsx:15</code> | <code>export type TokenIconSize = keyof typeof SIZES;</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferhistory</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx:142</code> | <code>export function TransferHistory({ records, filters = ["all", "deposit", "withdraw", "transfer"], filter, defaultFilter = "all", onFilterChange, filterLocally = true, explorerUrl, onCopy, formatTime = defaultFormatTime, loading, emptyText = "No transfers yet.",</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferhistoryfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx:24</code> | <code>export type TransferHistoryFilter = "all" &#124; TransferRecordType;</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferhistoryprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx:50</code> | <code>export interface TransferHistoryProps</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx:26</code> | <code>export type TransferRecord =</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferrecordstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx:23</code> | <code>export type TransferRecordStatus = "pending" &#124; "processing" &#124; "completed" &#124; "failed" &#124; "cancelled";</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferrecordtype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx:22</code> | <code>export type TransferRecordType = "deposit" &#124; "withdraw" &#124; "transfer";</code> |
| <code>interface.14.ts-bridge-components-ui-alert-alert</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/alert.tsx:23</code> | <code>function Alert({ className, variant, ...props }: React.ComponentProps&lt;"div"&gt; &amp; VariantProps&lt;typeof alertVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-alert-alertdescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/alert.tsx:33</code> | <code>function AlertDescription({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-alert-alerttitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/alert.tsx:27</code> | <code>function AlertTitle({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-avatar-avatar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/avatar.tsx:6</code> | <code>function Avatar({ className, ...props }: AvatarPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-avatar-avatarfallback</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/avatar.tsx:25</code> | <code>function AvatarFallback({ className, ...props }: AvatarPrimitive.Fallback.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-avatar-avatargroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/avatar.tsx:35</code> | <code>function AvatarGroup({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-avatar-avatarimage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/avatar.tsx:19</code> | <code>function AvatarImage({ className, ...props }: AvatarPrimitive.Image.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-badge-badge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/badge.tsx:28</code> | <code>function Badge({ className, variant, render, ...props }: BadgeProps)</code> |
| <code>interface.14.ts-bridge-components-ui-badge-badgevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/badge.tsx:8</code> | <code>const badgeVariants = cva(</code> |
| <code>interface.14.ts-bridge-components-ui-button-button</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/button.tsx:49</code> | <code>function Button({ className, variant, size, render, nativeButton, ...props }: ButtonProps)</code> |
| <code>interface.14.ts-bridge-components-ui-button-buttonprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/button.tsx:41</code> | <code>type ButtonProps = ButtonPrimitive.Props &amp; VariantProps&lt;typeof buttonVariants&gt;;</code> |
| <code>interface.14.ts-bridge-components-ui-button-buttonvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/button.tsx:7</code> | <code>const buttonVariants = cva(</code> |
| <code>interface.14.ts-bridge-components-ui-card-card</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/card.tsx:10</code> | <code>function Card({ className, variant = "outline", ...props }: CardProps)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/card.tsx:56</code> | <code>function CardAction({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/card.tsx:66</code> | <code>function CardContent({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-carddescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/card.tsx:50</code> | <code>function CardDescription({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardfooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/card.tsx:70</code> | <code>function CardFooter({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/card.tsx:27</code> | <code>function CardHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardtitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/card.tsx:40</code> | <code>function CardTitle({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsible</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/collapsible.tsx:5</code> | <code>function Collapsible(props: CollapsiblePrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsiblecontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/collapsible.tsx:13</code> | <code>function CollapsibleContent({ className, ...props }: CollapsiblePrimitive.Panel.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsibletrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/collapsible.tsx:9</code> | <code>function CollapsibleTrigger(props: CollapsiblePrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialog</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:7</code> | <code>function Dialog(props: DialogPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogclose</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:19</code> | <code>function DialogClose(props: DialogPrimitive.Close.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:36</code> | <code>function DialogContent({ className, children, showCloseButton = true, ...props }: DialogPrimitive.Popup.Props &amp; { showCloseButton?: boolean })</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogdescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:94</code> | <code>function DialogDescription({ className, ...props }: DialogPrimitive.Description.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogfooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:74</code> | <code>function DialogFooter({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:70</code> | <code>function DialogHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogoverlay</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:23</code> | <code>function DialogOverlay({ className, ...props }: DialogPrimitive.Backdrop.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogportal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:15</code> | <code>function DialogPortal(props: DialogPrimitive.Portal.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogtitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:84</code> | <code>function DialogTitle({ className, ...props }: DialogPrimitive.Title.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogtrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dialog.tsx:11</code> | <code>function DialogTrigger(props: DialogPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenu</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:12</code> | <code>function DropdownMenu(props: MenuPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenucheckboxitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:67</code> | <code>function DropdownMenuCheckboxItem({ className, children, ...props }: MenuPrimitive.CheckboxItem.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenucontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:20</code> | <code>function DropdownMenuContent({ className, align = "start", side = "bottom", sideOffset = 6, alignOffset = 0, ...props }: MenuPrimitive.Popup.Props &amp; Pick&lt;MenuPrimitive.Positioner.Props, "align" &#124; "side" &#124; "sideOffset" &#124; "alignOffset"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenugroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:37</code> | <code>function DropdownMenuGroup(props: MenuPrimitive.Group.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:51</code> | <code>function DropdownMenuItem({ className, inset, variant = "default", ...props }: MenuPrimitive.Item.Props &amp; { inset?: boolean; variant?: "default" &#124; "destructive" })</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenulabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:41</code> | <code>function DropdownMenuLabel({ className, inset, ...props }: MenuPrimitive.GroupLabel.Props &amp; { inset?: boolean })</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuradiogroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:80</code> | <code>function DropdownMenuRadioGroup(props: MenuPrimitive.RadioGroup.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuradioitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:84</code> | <code>function DropdownMenuRadioItem({ className, children, ...props }: MenuPrimitive.RadioItem.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuseparator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:97</code> | <code>function DropdownMenuSeparator({ className, ...props }: MenuPrimitive.Separator.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenushortcut</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:101</code> | <code>function DropdownMenuShortcut({ className, ...props }: React.ComponentProps&lt;"span"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusub</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:111</code> | <code>function DropdownMenuSub(props: MenuPrimitive.SubmenuRoot.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusubcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:128</code> | <code>function DropdownMenuSubContent({ className, sideOffset = 4, alignOffset = -6, ...props }: MenuPrimitive.Popup.Props &amp; Pick&lt;MenuPrimitive.Positioner.Props, "sideOffset" &#124; "alignOffset"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusubtrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:115</code> | <code>function DropdownMenuSubTrigger({ className, inset, children, ...props }: MenuPrimitive.SubmenuTrigger.Props &amp; { inset?: boolean })</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenutrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:16</code> | <code>function DropdownMenuTrigger(props: MenuPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-menuitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:9</code> | <code>const menuItem =</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-menupopup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx:7</code> | <code>const menuPopup =</code> |
| <code>interface.14.ts-bridge-components-ui-empty-empty</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/empty.tsx:5</code> | <code>function Empty({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptycontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/empty.tsx:44</code> | <code>function EmptyContent({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptydescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/empty.tsx:40</code> | <code>function EmptyDescription({ className, ...props }: React.ComponentProps&lt;"p"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptyheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/empty.tsx:18</code> | <code>function EmptyHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptymedia</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/empty.tsx:22</code> | <code>function EmptyMedia({ className, variant = "icon", ...props }: React.ComponentProps&lt;"div"&gt; &amp; { variant?: "default" &#124; "icon" })</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptytitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/empty.tsx:36</code> | <code>function EmptyTitle({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-field-field</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:18</code> | <code>function Field({ className, orientation, ...props }: FieldPrimitive.Root.Props &amp; VariantProps&lt;typeof fieldVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldcontrol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:39</code> | <code>function FieldControl({ className, ...props }: FieldPrimitive.Control.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fielddescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:52</code> | <code>function FieldDescription({ className, ...props }: FieldPrimitive.Description.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fielderror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:56</code> | <code>function FieldError({ className, ...props }: FieldPrimitive.Error.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:68</code> | <code>function FieldGroup({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldlabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:29</code> | <code>function FieldLabel({ className, ...props }: FieldPrimitive.Label.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldlegend</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:64</code> | <code>function FieldLegend({ className, ...props }: FieldsetPrimitive.Legend.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldseparator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:72</code> | <code>function FieldSeparator({ className, children, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/field.tsx:60</code> | <code>function FieldSet({ className, ...props }: FieldsetPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/hover-card.tsx:5</code> | <code>function HoverCard(props: PreviewCardPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercardcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/hover-card.tsx:13</code> | <code>function HoverCardContent({ className, align = "center", side = "bottom", sideOffset = 8, ...props }: PreviewCardPrimitive.Popup.Props &amp; Pick&lt;PreviewCardPrimitive.Positioner.Props, "align" &#124; "side" &#124; "sideOffset"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercardtrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/hover-card.tsx:9</code> | <code>function HoverCardTrigger(props: PreviewCardPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/input-group.tsx:5</code> | <code>function InputGroup({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgroupaddon</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/input-group.tsx:22</code> | <code>function InputGroupAddon({ className, align = "inline-start", ...props }: React.ComponentProps&lt;"div"&gt; &amp; { align?: "inline-start" &#124; "inline-end" &#124; "block-start" &#124; "block-end" })</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgroupinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/input-group.tsx:50</code> | <code>function InputGroupInput({ className, ...props }: React.ComponentProps&lt;"input"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgrouptext</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/input-group.tsx:73</code> | <code>function InputGroupText({ className, ...props }: React.ComponentProps&lt;"span"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgrouptextarea</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/input-group.tsx:63</code> | <code>function InputGroupTextarea({ className, ...props }: React.ComponentProps&lt;"textarea"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-input-input</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/input.tsx:5</code> | <code>function Input({ className, type, ...props }: InputPrimitive.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-kbd-kbd</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/kbd.tsx:5</code> | <code>function Kbd({ className, ...props }: React.ComponentProps&lt;"kbd"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-kbd-kbdgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/kbd.tsx:18</code> | <code>function KbdGroup({ className, ...props }: React.ComponentProps&lt;"kbd"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-label-label</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/label.tsx:5</code> | <code>function Label({ className, ...props }: React.ComponentProps&lt;"label"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popover</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/popover.tsx:6</code> | <code>function Popover(props: PopoverPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popoverclose</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/popover.tsx:50</code> | <code>const PopoverClose = PopoverPrimitive.Close;</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popovercontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/popover.tsx:14</code> | <code>function PopoverContent({ className, align = "center", side = "bottom", sideOffset = 8, ...props }: PopoverPrimitive.Popup.Props &amp; Pick&lt;PopoverPrimitive.Positioner.Props, "align" &#124; "side" &#124; "sideOffset" &#124; "alignOffset"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popoverdescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/popover.tsx:46</code> | <code>function PopoverDescription({ className, ...props }: PopoverPrimitive.Description.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popoverheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/popover.tsx:38</code> | <code>function PopoverHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popovertitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/popover.tsx:42</code> | <code>function PopoverTitle({ className, ...props }: PopoverPrimitive.Title.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popovertrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/popover.tsx:10</code> | <code>function PopoverTrigger(props: PopoverPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-progress-progress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/progress.tsx:5</code> | <code>function Progress({ className, children, ...props }: ProgressPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-progress-progresslabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/progress.tsx:19</code> | <code>function ProgressLabel({ className, ...props }: ProgressPrimitive.Label.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-progress-progressvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/progress.tsx:23</code> | <code>function ProgressValue({ className, ...props }: ProgressPrimitive.Value.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-scroll-area-scrollarea</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/scroll-area.tsx:20</code> | <code>function ScrollArea({ className, children, fade, ...props }: ScrollAreaProps)</code> |
| <code>interface.14.ts-bridge-components-ui-scroll-area-scrollbar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/scroll-area.tsx:37</code> | <code>function ScrollBar({ className, orientation = "vertical", ...props }: ScrollAreaPrimitive.Scrollbar.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-select</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/select.tsx:6</code> | <code>const Select = SelectPrimitive.Root;</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/select.tsx:35</code> | <code>function SelectContent({ className, children, side = "bottom", sideOffset = 6, align = "center", alignItemWithTrigger = true, ...props }: SelectPrimitive.Popup.Props &amp; Pick&lt;SelectPrimitive.Positioner.Props, "align" &#124; "side" &#124; "sideOffset" &#124; "alignItemWithTrigger"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/select.tsx:8</code> | <code>function SelectGroup({ className, ...props }: SelectPrimitive.Group.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/select.tsx:78</code> | <code>function SelectItem({ className, children, ...props }: SelectPrimitive.Item.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectlabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/select.tsx:74</code> | <code>function SelectLabel({ className, ...props }: SelectPrimitive.GroupLabel.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectseparator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/select.tsx:96</code> | <code>function SelectSeparator({ className, ...props }: SelectPrimitive.Separator.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selecttrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/select.tsx:16</code> | <code>function SelectTrigger({ className, size = "default", children, ...props }: SelectPrimitive.Trigger.Props &amp; { size?: "sm" &#124; "default" })</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/select.tsx:12</code> | <code>function SelectValue({ className, ...props }: SelectPrimitive.Value.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-separator-separator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/separator.tsx:5</code> | <code>function Separator({ className, orientation = "horizontal", ...props }: SeparatorPrimitive.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sheet.tsx:7</code> | <code>function Sheet(props: SheetPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetclose</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sheet.tsx:15</code> | <code>function SheetClose(props: SheetPrimitive.Close.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sheet.tsx:27</code> | <code>function SheetContent({ className, children, side = "right", showCloseButton = true, ...props }: SheetPrimitive.Popup.Props &amp; { side?: keyof typeof sideClasses; showCloseButton?: boolean })</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetdescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sheet.tsx:80</code> | <code>function SheetDescription({ className, ...props }: SheetPrimitive.Description.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetfooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sheet.tsx:66</code> | <code>function SheetFooter({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sheet.tsx:62</code> | <code>function SheetHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheettitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sheet.tsx:70</code> | <code>function SheetTitle({ className, ...props }: SheetPrimitive.Title.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheettrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sheet.tsx:11</code> | <code>function SheetTrigger(props: SheetPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-skeleton-skeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/skeleton.tsx:5</code> | <code>function Skeleton({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-sonner-toast</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sonner.tsx:34</code> | <code>export { Toaster, toast };</code> |
| <code>interface.14.ts-bridge-components-ui-sonner-toaster</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/sonner.tsx:6</code> | <code>function Toaster(props: ToasterProps)</code> |
| <code>interface.14.ts-bridge-components-ui-spinner-spinner</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/spinner.tsx:6</code> | <code>function Spinner({ className, ...props }: React.ComponentProps&lt;"output"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-table</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/table.tsx:5</code> | <code>function Table({ className, ...props }: React.ComponentProps&lt;"table"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablebody</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/table.tsx:17</code> | <code>function TableBody({ className, ...props }: React.ComponentProps&lt;"tbody"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablecaption</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/table.tsx:52</code> | <code>function TableCaption({ className, ...props }: React.ComponentProps&lt;"caption"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablecell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/table.tsx:48</code> | <code>function TableCell({ className, ...props }: React.ComponentProps&lt;"td"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablefooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/table.tsx:21</code> | <code>function TableFooter({ className, ...props }: React.ComponentProps&lt;"tfoot"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablehead</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/table.tsx:35</code> | <code>function TableHead({ className, ...props }: React.ComponentProps&lt;"th"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tableheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/table.tsx:13</code> | <code>function TableHeader({ className, ...props }: React.ComponentProps&lt;"thead"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablerow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/table.tsx:25</code> | <code>function TableRow({ className, ...props }: React.ComponentProps&lt;"tr"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tabs.tsx:6</code> | <code>function Tabs({ className, ...props }: TabsPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabscontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tabs.tsx:67</code> | <code>function TabsContent({ className, ...props }: TabsPrimitive.Panel.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabslist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tabs.tsx:26</code> | <code>function TabsList({ className, variant = "default", children, ...props }: TabsPrimitive.List.Props &amp; VariantProps&lt;typeof tabsListVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabslistvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tabs.tsx:16</code> | <code>const tabsListVariants = cva("relative z-0 inline-flex w-fit items-center data-[orientation=vertical]:flex-col",</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabstrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tabs.tsx:53</code> | <code>function TabsTrigger({ className, ...props }: TabsPrimitive.Tab.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-toggle-group-togglegroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/toggle-group.tsx:11</code> | <code>function ToggleGroup({ className, variant, size, children, ...props }: ToggleGroupPrimitive.Props &amp; VariantProps&lt;typeof toggleVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-toggle-group-togglegroupitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/toggle-group.tsx:34</code> | <code>function ToggleGroupItem({ className, variant: _variant, size, ...props }: TogglePrimitive.Props &amp; VariantProps&lt;typeof toggleVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-toggle-toggle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/toggle.tsx:24</code> | <code>function Toggle({ className, variant, size, ...props }: TogglePrimitive.Props &amp; VariantProps&lt;typeof toggleVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-toggle-togglevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/toggle.tsx:6</code> | <code>const toggleVariants = cva(</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltip</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tooltip.tsx:11</code> | <code>function Tooltip(props: TooltipPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tooltip.tsx:51</code> | <code>function TooltipContent({ className, side = "top", sideOffset, align = "center", variant = "inverted", arrow = false, children, ...props }: TooltipPrimitive.Popup.Props &amp; Pick&lt;TooltipPrimitive.Positioner.Props, "side" &#124; "sideOffset" &#124; "align"&gt; &amp; VariantProps&lt;typeof tooltipVariants&gt; &amp;</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipcontentprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tooltip.tsx:92</code> | <code>export type TooltipContentProps = React.ComponentProps&lt;typeof TooltipContent&gt;;</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tooltip.tsx:7</code> | <code>function TooltipProvider({ delay = 300, ...props }: TooltipPrimitive.Provider.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltiptrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tooltip.tsx:15</code> | <code>function TooltipTrigger(props: TooltipPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/components/ui/tooltip.tsx:19</code> | <code>const tooltipVariants = cva(</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-components-bridge-bridge-card-bridgecard</code> | overridden_by | override.J55.15 |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-buildverifiedclaim</code> | overridden_by | override.J55.11 |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-buildverifiedrelease</code> | overridden_by | override.J55.12 |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-readtransferlive</code> | overridden_by | override.J55.13 |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-transferlive</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-transferlive</code> | overridden_by | override.J55.14 |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-verifiedclaim</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-verifiedrelease</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-bridge-transfers-panel-transferspanel</code> | overridden_by | override.J55.17 |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-createthemetransition</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetoggleicon</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetoggleprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionstart</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionvariant</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-usethemetransition</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainlistprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainselector</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainselectorprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chaintab</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-deposit-status-depositstatusprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstatusgroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstep</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstep</code> | overridden_by | override.J55.22 |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transfersteps</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-numeral-numeralprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-numeral-statistic</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-numeral-statisticprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-token-icon-chainiconprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-token-icon-symbolbadge</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-token-icon-symboldisplay</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-token-icon-symboldisplayprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-token-icon-tokeniconprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-token-icon-tokeniconsize</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferhistoryfilter</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferhistoryprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferrecordstatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferrecordtype</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-avatar-avatar</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-avatar-avatarfallback</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-avatar-avatargroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-avatar-avatarimage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-badge-badgevariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-button-buttonprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-button-buttonvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-card-cardaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-card-cardfooter</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsible</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsiblecontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsibletrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogclose</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogfooter</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogoverlay</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogportal</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogtrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenu</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenucheckboxitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenucontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenugroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenulabel</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuradiogroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuradioitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuseparator</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenushortcut</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusub</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusubcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusubtrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenutrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-menuitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-menupopup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-empty-empty</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-empty-emptycontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-empty-emptydescription</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-empty-emptyheader</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-empty-emptymedia</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-empty-emptytitle</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-field-fieldcontrol</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-field-fielderror</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-field-fieldgroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-field-fieldlegend</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-field-fieldseparator</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-field-fieldset</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercard</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercardcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercardtrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgrouptextarea</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-kbd-kbd</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-kbd-kbdgroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-label-label</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-popover-popoverclose</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-popover-popoverdescription</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-popover-popoverheader</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-popover-popovertitle</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-progress-progress</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-progress-progresslabel</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-progress-progressvalue</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-scroll-area-scrollarea</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-scroll-area-scrollbar</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-select-select</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-select-selectcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-select-selectgroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-select-selectitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-select-selectlabel</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-select-selectseparator</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-select-selecttrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-select-selectvalue</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-sheet-sheet</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetclose</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetdescription</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetfooter</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetheader</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-sheet-sheettitle</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-sheet-sheettrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-table-table</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-table-tablebody</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-table-tablecaption</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-table-tablecell</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-table-tablefooter</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-table-tablehead</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-table-tableheader</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-table-tablerow</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-tabs-tabslistvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-toggle-group-togglegroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-toggle-group-togglegroupitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-toggle-toggle</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipcontentprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipvariants</code> | notes | no in-repo importer at HEAD |

Contract context <code>contract_context.14.bridge-client</code>:

| Field | Frozen rule |
|---|---|
| interface | apps/bridge client: deposits call PaxeerXVault.deposit and depositNative on the foreign chain, withdrawals call bridgeOut on 0x1016, claims and releases are built from attestation bundles fetched from /v1/deposit and /v1/release and verified locally before the wallet opens. |
| storage | Browser localStorage key paxeer-bridge:transfers:v1 holding BridgeTransfer rows (apps/bridge/src/lib/bridge/transfers.ts:47); no server state. |
| errors | LXP_BRIDGE_UNAUTHORIZED, LXP_BRIDGE_UNKNOWN_CHAIN, LXP_BRIDGE_CHAIN_DISABLED, LXP_BRIDGE_UNKNOWN_ASSET, LXP_BRIDGE_PAUSED, LXP_BRIDGE_NULLIFIED, LXP_BRIDGE_CAP_EXCEEDED, LXP_BRIDGE_BAD_SIGNATURE, LXP_BRIDGE_BELOW_THRESHOLD, LXP_BRIDGE_VAULT_MISMATCH, LXP_BRIDGE_INVALID_REQUEST mapped to sentences; vault custom errors decoded with paxeerXVaultAbi. |
| events | Reads BridgeDeposit and BridgeRelease of PaxeerXVault and BridgeOut of 0x1016 from receipts. |
| gas | None set by the client; the wallet estimates gas. The Max amount keeps a native gas reserve. |
| activation | Routes open per chain when NEXT_PUBLIC_BRIDGE_VAULT_&lt;CHAIN&gt; is set and the chain is registered and enabled on 0x1016; claim and release need NEXT_PUBLIC_BRIDGE_API_URL. |
| invariants | The client never submits a claim or release whose digest it has not recomputed and whose signatures do not meet the threshold; the attested recipient, asset and chain equal the local record. |
| pause | A paused precompile or vault disables submission and shows the paused blocker. |

## config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-bridge-next-public-bridge-rpc-solana-j55</code> | <code>env_var</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> (planned) | <code>NEXT_PUBLIC_BRIDGE_RPC_SOLANA</code> |
| <code>interface.14.env-bridge-next-public-bridge-solana-program-id-j55</code> | <code>env_var</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> (planned) | <code>NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-buildbridgechains-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> (planned) | <code>export function buildBridgeChains(config: BridgeConfig): BridgeChain[]</code> |
| <code>interface.14.ts-bridge-lib-bridge-config-readconfig-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> (planned) | <code>export function readConfig(env: Record&lt;string, string &#124; undefined&gt;): BridgeConfig</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeeraddressurlfrom-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> (planned) | <code>export function paxeerAddressUrlFrom(base: string &#124; undefined, address: Address): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeertxurlfrom-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> (planned) | <code>export function paxeerTxUrlFrom(base: string &#124; undefined, hash: Hex): string &#124; undefined</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-bridge-next-public-bridge-rpc-solana-j55</code> | notes | created by task 14.50.1; Solana JSON-RPC endpoint used for recipient record reads |
| <code>interface.14.env-bridge-next-public-bridge-solana-program-id-j55</code> | notes | created by task 14.50.1; Solana custody program id, from the Solana deployment record program_id |
| <code>interface.14.ts-bridge-lib-bridge-chains-buildbridgechains-j55</code> | notes | created by task 14.50.2; route table from configuration |
| <code>interface.14.ts-bridge-lib-bridge-config-readconfig-j55</code> | notes | created by task 14.50.1; pure reader behind bridgeConfig |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeeraddressurlfrom-j55</code> | notes | created by task 14.57.1; Paxeer address link from an explicit base |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeertxurlfrom-j55</code> | notes | created by task 14.57.1; Paxeer transaction link from an explicit base |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.500.1</code> | req | 14500.1 |
| <code>acceptance.14.500.1</code> | scenario | Given env with NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID set to a valid base58 key, when readConfig runs, then solanaProgramId equals that key |
| <code>acceptance.14.500.1</code> | test | apps/bridge/src/lib/bridge/config.spec.ts::readConfigReadsTheSolanaProgramId (planned) |
| <code>acceptance.14.500.1</code> | qualification | UNRUN |
| <code>req.14500</code> | ac_1 | readConfig(env) returns solanaProgramId from NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID when it is a 32-byte base58 key; at HEAD apps/bridge/src/lib/bridge/config.ts:30-58 has no Solana input. |
| <code>acceptance.14.500.2</code> | req | 14500.2 |
| <code>acceptance.14.500.2</code> | scenario | Given NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID set to "abc", when readConfig runs, then solanaProgramId is undefined |
| <code>acceptance.14.500.2</code> | test | apps/bridge/src/lib/bridge/config.spec.ts::readConfigRefusesAShortProgramId (planned) |
| <code>acceptance.14.500.2</code> | qualification | UNRUN |
| <code>req.14500</code> | ac_2 | readConfig leaves solanaProgramId undefined for a value that does not decode to 32 bytes (apps/bridge/src/lib/bridge/solana.ts:50). |
| <code>acceptance.14.500.3</code> | req | 14500.3 |
| <code>acceptance.14.500.3</code> | scenario | Given NEXT_PUBLIC_BRIDGE_VAULT_BASE set to 0x0000000000000000000000000000000000000000, when readConfig runs, then vaults.base is undefined |
| <code>acceptance.14.500.3</code> | test | apps/bridge/src/lib/bridge/config.spec.ts::readConfigDropsAZeroVault (planned) |
| <code>acceptance.14.500.3</code> | qualification | UNRUN |
| <code>req.14500</code> | ac_3 | readConfig leaves a vault undefined when its variable holds the zero address; at HEAD readAddress at apps/bridge/src/lib/bridge/config.ts:25 accepts it. |
| <code>acceptance.14.501.1</code> | req | 14501.1 |
| <code>acceptance.14.501.1</code> | scenario | Given readConfig output with a Solana program id and an attestation API URL, when buildBridgeChains runs, then the solana entry has state configured |
| <code>acceptance.14.501.1</code> | test | apps/bridge/src/lib/bridge/chains.spec.ts::buildBridgeChainsMarksSolanaConfiguredWithProgramAndApi (planned) |
| <code>acceptance.14.501.1</code> | qualification | UNRUN |
| <code>req.14501</code> | ac_1 | buildBridgeChains(config) marks solana configured when solanaProgramId and attestationApiUrl are both set; at HEAD apps/bridge/src/lib/bridge/chains.ts:219 hard-codes awaiting-backend. |
| <code>acceptance.14.501.2</code> | req | 14501.2 |
| <code>acceptance.14.501.2</code> | scenario | Given readConfig output without a Solana program id, when buildBridgeChains runs, then the solana entry has state awaiting-backend |
| <code>acceptance.14.501.2</code> | test | apps/bridge/src/lib/bridge/chains.spec.ts::buildBridgeChainsMarksSolanaAwaitingWithoutAProgramId (planned) |
| <code>acceptance.14.501.2</code> | qualification | UNRUN |
| <code>req.14501</code> | ac_2 | buildBridgeChains marks solana awaiting-backend when solanaProgramId is unset (apps/bridge/src/lib/bridge/chains.ts:212-219). |
| <code>acceptance.14.518.1</code> | req | 14518.1 |
| <code>acceptance.14.518.1</code> | scenario | Given a config without paxeerExplorerUrl, when paxeerTxUrl and paxeerAddressUrl run, then both return undefined |
| <code>acceptance.14.518.1</code> | test | apps/bridge/src/lib/bridge/explorer.spec.ts::paxeerLinksAreUndefinedWithoutAConfiguredExplorer (planned) |
| <code>acceptance.14.518.1</code> | qualification | UNRUN |
| <code>req.14518</code> | ac_1 | paxeerTxUrl and paxeerAddressUrl return undefined when NEXT_PUBLIC_PAXEER_EXPLORER_URL is unset; at HEAD apps/bridge/src/lib/bridge/config.ts:33 substitutes a hard-coded explorer, contradicting the rule at apps/bridge/src/lib/bridge/config.ts:5-7 that nothing is invented. |
| <code>acceptance.14.518.2</code> | req | 14518.2 |
| <code>acceptance.14.518.2</code> | scenario | Given a configured explorer with a trailing slash, when paxeerAddressUrl runs, then the result has exactly one slash before address |
| <code>acceptance.14.518.2</code> | test | apps/bridge/src/lib/bridge/explorer.spec.ts::paxeerLinksUseTheConfiguredExplorer (planned) |
| <code>acceptance.14.518.2</code> | qualification | UNRUN |
| <code>req.14518</code> | ac_2 | paxeerAddressUrl returns &lt;explorer&gt;/address/&lt;address&gt; with trailing slashes trimmed when an explorer is configured (apps/bridge/src/lib/bridge/explorer.ts:29-31). |
| <code>capability.14.50.1</code> | title | Make the client config a pure reader with the Solana program id and Solana RPC inputs |
| <code>capability.14.50.1</code> | reqs | 14500.1<br>14500.2<br>14500.3 |
| <code>capability.14.50.2</code> | title | Derive the Solana route state from configuration instead of a hard-coded awaiting-backend |
| <code>capability.14.50.2</code> | reqs | 14501.1<br>14501.2 |
| <code>capability.14.57.1</code> | title | Stop inventing a Paxeer explorer and return no link when none is configured |
| <code>capability.14.57.1</code> | reqs | 14518.1<br>14518.2 |
| <code>resolution.17.956</code> | title | decision.1442 binds: The bridge app invents no Paxeer explorer |
| <code>resolution.17.956</code> | applies_to | decision.1442<br>task.14.57.1 |
| <code>resolution.17.956</code> | source | decision.1442 in fragments/J55/decision.kvx; tasks from affects |

## conformance

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.512.1</code> | req | 14512.1 |
| <code>acceptance.14.512.1</code> | scenario | Given the wrapped SOL mint, when solanaRecipientHandle runs, then the result equals the committed SOL asset id in both files |
| <code>acceptance.14.512.1</code> | test | apps/bridge/src/lib/bridge/solana.spec.ts::solanaHandleOfTheWrappedSolMintMatchesTheConfig (planned) |
| <code>acceptance.14.512.1</code> | qualification | UNRUN |
| <code>req.14512</code> | ac_1 | solanaRecipientHandle("So11111111111111111111111111111111111111112") (services/bridge/deploy/chainconfig/config.go:67) equals, ignoring case, the SOL asset_id at contracts/bridge/solana/chains/solana/config.json:32 and the sol assetId in apps/bridge/src/lib/bridge/chains.ts:212-230. |
| <code>acceptance.14.513.1</code> | req | 14513.1 |
| <code>acceptance.14.513.1</code> | scenario | Given precompiles/layerxbridge/abi.json read from the repository root, when it is compared with layerxBridgeAbi, then they are deep-equal |
| <code>acceptance.14.513.1</code> | test | apps/bridge/src/lib/bridge/contracts.spec.ts::layerxBridgeAbiEqualsThePrecompileAbi (planned) |
| <code>acceptance.14.513.1</code> | qualification | UNRUN |
| <code>req.14513</code> | ac_1 | layerxBridgeAbi (apps/bridge/src/lib/bridge/contracts.ts:19) deep-equals precompiles/layerxbridge/abi.json:1; the two files are identical at HEAD and nothing keeps them so. |
| <code>acceptance.14.513.2</code> | req | 14513.2 |
| <code>acceptance.14.513.2</code> | scenario | Given PaxeerXVault.sol read from the repository root, when each paxeerXVaultAbi item signature is looked up among the parsed declarations, then none is missing |
| <code>acceptance.14.513.2</code> | test | apps/bridge/src/lib/bridge/contracts.spec.ts::paxeerXVaultAbiSelectorsExistInTheVaultSource (planned) |
| <code>acceptance.14.513.2</code> | qualification | UNRUN |
| <code>req.14513</code> | ac_2 | Every function, event and error of paxeerXVaultAbi (apps/bridge/src/lib/bridge/contracts.ts:22-63) has a declaration with the same name and parameter types in contracts/bridge/evm/src/PaxeerXVault.sol:40-213. |
| <code>acceptance.14.514.1</code> | req | 14514.1 |
| <code>acceptance.14.514.1</code> | scenario | Given the eight committed EVM chain configurations, when each is compared with its BRIDGE_CHAINS entry, then chain id and finality depth are equal |
| <code>acceptance.14.514.1</code> | test | apps/bridge/src/lib/bridge/registry.spec.ts::evmChainsMatchTheCommittedConfigs (planned) |
| <code>acceptance.14.514.1</code> | qualification | UNRUN |
| <code>req.14514</code> | ac_1 | For each EVM entry of BRIDGE_CHAINS (apps/bridge/src/lib/bridge/chains.ts:130) chainId and finalityDepth equal chain_id and finality_depth of contracts/bridge/evm/chains/&lt;key&gt;/config.json (for example contracts/bridge/evm/chains/base/config.json:4 and contracts/bridge/evm/chains/base/config.json:23). |
| <code>acceptance.14.514.2</code> | req | 14514.2 |
| <code>acceptance.14.514.2</code> | scenario | Given contracts/bridge/solana/chains/solana/config.json, when it is compared with bridgeChainByKey("solana"), then chain id and finality depth are equal |
| <code>acceptance.14.514.2</code> | test | apps/bridge/src/lib/bridge/registry.spec.ts::solanaChainMatchesTheCommittedConfig (planned) |
| <code>acceptance.14.514.2</code> | qualification | UNRUN |
| <code>req.14514</code> | ac_2 | The solana entry has chainId 91600046870081 and finalityDepth 32 as contracts/bridge/solana/chains/solana/config.json:4 and contracts/bridge/solana/chains/solana/config.json:23 state (apps/bridge/src/lib/bridge/chains.ts:216). |
| <code>acceptance.14.515.1</code> | req | 14515.1 |
| <code>acceptance.14.515.1</code> | scenario | Given layerxbridge.go read from the repository root, when the test extracts every quoted LXP_BRIDGE_ code and compares the set with the PRECOMPILE_CODES keys, then the sets are equal |
| <code>acceptance.14.515.1</code> | test | apps/bridge/src/lib/bridge/errors-registry.spec.ts::everyPrecompileRevertCodeHasASentence (planned) |
| <code>acceptance.14.515.1</code> | qualification | UNRUN |
| <code>req.14515</code> | ac_1 | Every LXP_BRIDGE_ code listed in revertCodes at precompiles/layerxbridge/layerxbridge.go:97-112 is a key of PRECOMPILE_CODES in apps/bridge/src/lib/bridge/errors.ts, and PRECOMPILE_CODES has no other key. |
| <code>acceptance.14.516.1</code> | req | 14516.1 |
| <code>acceptance.14.516.1</code> | scenario | Given RelayerAbiParity.t.sol read from the repository root, when the two bytes32 constants are extracted and compared with the client event selectors, then both are equal |
| <code>acceptance.14.516.1</code> | test | apps/bridge/src/lib/bridge/topics.spec.ts::vaultEventTopicsMatchTheRelayerParityConstants (planned) |
| <code>acceptance.14.516.1</code> | qualification | UNRUN |
| <code>req.14516</code> | ac_1 | toEventSelector of BridgeDeposit and BridgeRelease in paxeerXVaultAbi (apps/bridge/src/lib/bridge/contracts.ts:39-40) equal BRIDGE_DEPOSIT_TOPIC at contracts/bridge/evm/test/RelayerAbiParity.t.sol:14 and BRIDGE_RELEASE_TOPIC at contracts/bridge/evm/test/RelayerAbiParity.t.sol:30. |
| <code>capability.14.54.4</code> | title | Pin the client Solana handle derivation to the committed SOL asset id |
| <code>capability.14.54.4</code> | reqs | 14512.1 |
| <code>capability.14.55.1</code> | title | Pin the app's bridge and vault ABIs to the precompile ABI and the vault source |
| <code>capability.14.55.1</code> | reqs | 14513.1<br>14513.2 |
| <code>capability.14.55.2</code> | title | Pin the client chain registry to the committed bridge chain configurations |
| <code>capability.14.55.2</code> | reqs | 14514.1<br>14514.2 |
| <code>capability.14.55.3</code> | title | Pin the client refusal-code table to the precompile revert code registry |
| <code>capability.14.55.3</code> | reqs | 14515.1 |
| <code>capability.14.55.4</code> | title | Pin the client vault event topics to the relayer parity constants |
| <code>capability.14.55.4</code> | reqs | 14516.1 |

## env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-bridge-next-public-bridge-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:35</code> | <code>NEXT_PUBLIC_BRIDGE_API_URL</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-arbitrum</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:51</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-avalanche</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:55</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-base</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:50</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_BASE</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-bnb</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:53</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_BNB</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-ethereum</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:49</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-hyperevm</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:56</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-optimism</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:52</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-polygon</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:54</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_POLYGON</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-arbitrum</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:40</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-avalanche</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:44</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-base</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:39</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_BASE</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-bnb</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:42</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_BNB</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-ethereum</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:38</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-hyperevm</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:45</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-optimism</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:41</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-polygon</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:43</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_POLYGON</code> |
| <code>interface.14.env-bridge-next-public-paxeer-explorer-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:33</code> | <code>NEXT_PUBLIC_PAXEER_EXPLORER_URL</code> |
| <code>interface.14.env-bridge-next-public-paxeer-rpc-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/config.ts:32</code> | <code>NEXT_PUBLIC_PAXEER_RPC_URL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-bridge-next-public-paxeer-rpc-url</code> | notes | read by bridge, perps |

## errors

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.499.1</code> | req | 14499.1 |
| <code>acceptance.14.499.1</code> | scenario | Given a viem ContractFunctionRevertedError with reason LXP_BRIDGE_PAUSED: bridge is paused, when describeWriteError runs, then it returns the bridge-paused sentence |
| <code>acceptance.14.499.1</code> | test | apps/bridge/src/lib/bridge/errors.spec.ts::describeWriteErrorMapsThePausedCode (planned) |
| <code>acceptance.14.499.1</code> | qualification | UNRUN |
| <code>req.14499</code> | ac_1 | describeWriteError for a revert reason "LXP_BRIDGE_PAUSED: bridge is paused" returns the paused sentence; the codes are listed at precompiles/layerxbridge/layerxbridge.go:97-112 and at HEAD apps/bridge/src/lib/bridge/errors.ts:10 only trims the raw reason. |
| <code>acceptance.14.499.2</code> | req | 14499.2 |
| <code>acceptance.14.499.2</code> | scenario | Given a revert reason LXP_BRIDGE_NULLIFIED: deposit already credited, when describeWriteError runs, then it returns the already-credited sentence |
| <code>acceptance.14.499.2</code> | test | apps/bridge/src/lib/bridge/errors.spec.ts::describeWriteErrorMapsTheNullifiedCode (planned) |
| <code>acceptance.14.499.2</code> | qualification | UNRUN |
| <code>req.14499</code> | ac_2 | describeWriteError for LXP_BRIDGE_NULLIFIED returns the already-credited sentence (precompiles/layerxbridge/layerxbridge.go:97-112). |
| <code>acceptance.14.499.3</code> | req | 14499.3 |
| <code>acceptance.14.499.3</code> | scenario | Given a revert reason "vault mismatch on route", when describeWriteError runs, then it returns "The contract rejected this: vault mismatch on route." |
| <code>acceptance.14.499.3</code> | test | apps/bridge/src/lib/bridge/errors.spec.ts::describeWriteErrorKeepsAnUnregisteredReason (planned) |
| <code>acceptance.14.499.3</code> | qualification | UNRUN |
| <code>req.14499</code> | ac_3 | describeWriteError for a reason without a registered LXP_BRIDGE_ code returns the reason text with a trailing period, as apps/bridge/src/lib/bridge/errors.ts:10-14 does today. |
| <code>acceptance.14.499.4</code> | req | 14499.4 |
| <code>acceptance.14.499.4</code> | scenario | Given new UserRejectedRequestError(new Error()) and new Error("nonce rejected by node"), when describeWriteError runs on each, then only the first returns the rejection sentence |
| <code>acceptance.14.499.4</code> | test | apps/bridge/src/lib/bridge/errors.spec.ts::describeWriteErrorDetectsAWalletRejectionByType (planned) |
| <code>acceptance.14.499.4</code> | qualification | UNRUN |
| <code>req.14499</code> | ac_4 | describeWriteError returns the rejection sentence for a viem UserRejectedRequestError and not for an Error whose message merely contains rejected; at HEAD apps/bridge/src/lib/bridge/errors.ts:36 tests the message text. |
| <code>capability.14.49.1</code> | title | Map every LXP_BRIDGE refusal code to a sentence and detect wallet rejection by error type |
| <code>capability.14.49.1</code> | reqs | 14499.1<br>14499.2<br>14499.3<br>14499.4 |

## harness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cli-bridge-pnpm-test-j55</code> | <code>cli_command</code> | <code>new</code> | <code>apps/bridge/package.json</code> (planned) | <code>pnpm --dir apps/bridge test</code> |
| <code>interface.14.cli-bridge-pnpm-typecheck-j55</code> | <code>cli_command</code> | <code>new</code> | <code>apps/bridge/package.json</code> (planned) | <code>pnpm --dir apps/bridge typecheck</code> |
| <code>interface.14.env-bridge-next-public-app-host-j55</code> | <code>env_var</code> | <code>retire</code> | <code>apps/bridge/instrumentation.node.ts:25</code> | <code>NEXT_PUBLIC_APP_HOST</code> |
| <code>interface.14.env-bridge-next-public-app-instance-j55</code> | <code>env_var</code> | <code>retire</code> | <code>apps/bridge/instrumentation.node.ts:24</code> | <code>NEXT_PUBLIC_APP_INSTANCE</code> |
| <code>interface.14.env-bridge-next-public-git-commit-sha-j55</code> | <code>env_var</code> | <code>retire</code> | <code>apps/bridge/instrumentation.node.ts:22</code> | <code>NEXT_PUBLIC_GIT_COMMIT_SHA</code> |
| <code>interface.14.env-bridge-next-public-git-tag-j55</code> | <code>env_var</code> | <code>retire</code> | <code>apps/bridge/instrumentation.node.ts:22</code> | <code>NEXT_PUBLIC_GIT_TAG</code> |
| <code>interface.14.env-bridge-otel-sdk-enabled-j55</code> | <code>env_var</code> | <code>retire</code> | <code>apps/bridge/instrumentation.node.ts:61</code> | <code>OTEL_SDK_ENABLED</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cli-bridge-pnpm-test-j55</code> | notes | created by task 14.41.2; runs vitest run over src/**/*.spec.ts |
| <code>interface.14.cli-bridge-pnpm-typecheck-j55</code> | notes | created by task 14.41.2; runs tsc --noEmit -p tsconfig.json |
| <code>interface.14.env-bridge-next-public-app-host-j55</code> | notes | removed by task 14.56.1; service instance fallback of the unregistered hook |
| <code>interface.14.env-bridge-next-public-app-instance-j55</code> | notes | removed by task 14.56.1; service instance of the unregistered hook |
| <code>interface.14.env-bridge-next-public-git-commit-sha-j55</code> | notes | removed by task 14.56.1; service version fallback of the unregistered hook |
| <code>interface.14.env-bridge-next-public-git-tag-j55</code> | notes | removed by task 14.56.1; service version of the unregistered hook |
| <code>interface.14.env-bridge-otel-sdk-enabled-j55</code> | notes | removed by task 14.56.1; read only by the unregistered hook |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.481.1</code> | req | 14481.1 |
| <code>acceptance.14.481.1</code> | scenario | Given apps/bridge/tsconfig.json with paths {"@/*": ["./src/*"]} and the tsconfigPaths plugin in vitest.config.ts, when harness.spec.ts imports parseAmount from @/lib/bridge/amount, then parseAmount("1", 18) returns 1000000000000000000n |
| <code>acceptance.14.481.1</code> | test | apps/bridge/src/test/harness.spec.ts::resolvesTheSrcAlias (planned) |
| <code>acceptance.14.481.1</code> | qualification | UNRUN |
| <code>req.14481</code> | ac_1 | An import of @/lib/bridge/amount from a test under apps/bridge/src resolves to apps/bridge/src/lib/bridge/amount.ts because apps/bridge/tsconfig.json:17 sets baseUrl without a paths entry today. |
| <code>acceptance.14.481.2</code> | req | 14481.2 |
| <code>acceptance.14.481.2</code> | scenario | Given vitest.config.ts without globalSetup and setupFiles and with test.environment node, when harness.spec.ts runs, then typeof window is "undefined" and the run reports no missing setup module |
| <code>acceptance.14.481.2</code> | test | apps/bridge/src/test/harness.spec.ts::defaultsToTheNodeEnvironment (planned) |
| <code>acceptance.14.481.2</code> | qualification | UNRUN |
| <code>req.14481</code> | ac_2 | vitest run in apps/bridge starts with environment node and no globalSetup or setupFiles entry, replacing the missing files named at apps/bridge/vitest.config.ts:16-17. |
| <code>acceptance.14.482.1</code> | req | 14482.1 |
| <code>acceptance.14.482.1</code> | scenario | Given apps/bridge/package.json after the change, when manifest.spec.ts collects every import specifier under src and next.config.ts, then each package name is declared |
| <code>acceptance.14.482.1</code> | test | apps/bridge/src/test/manifest.spec.ts::everyImportedPackageIsDeclared (planned) |
| <code>acceptance.14.482.1</code> | qualification | UNRUN |
| <code>req.14482</code> | ac_1 | Every bare module specifier imported under apps/bridge/src, including motion, sonner, tailwindcss, @tailwindcss/turbopack (apps/bridge/next.config.ts:9-13) and @noble/curves, is a key of dependencies or devDependencies in apps/bridge/package.json. |
| <code>acceptance.14.482.2</code> | req | 14482.2 |
| <code>acceptance.14.482.2</code> | scenario | Given apps/bridge/package.json after the change, when manifest.spec.ts compares dependencies with the collected specifiers, then the unused list is empty |
| <code>acceptance.14.482.2</code> | test | apps/bridge/src/test/manifest.spec.ts::noDeclaredRuntimeDependencyIsUnused (planned) |
| <code>acceptance.14.482.2</code> | qualification | UNRUN |
| <code>req.14482</code> | ac_2 | No entry of dependencies in apps/bridge/package.json is unimported by apps/bridge sources or configuration files; the copied entries starting at apps/bridge/package.json:12 are removed. |
| <code>acceptance.14.482.3</code> | req | 14482.3 |
| <code>acceptance.14.482.3</code> | scenario | Given apps/bridge/package.json, when manifest.spec.ts reads scripts, then scripts.test is "vitest run" and scripts.typecheck is "tsc --noEmit -p tsconfig.json" |
| <code>acceptance.14.482.3</code> | test | apps/bridge/src/test/manifest.spec.ts::declaresTestAndTypecheckScripts (planned) |
| <code>acceptance.14.482.3</code> | qualification | UNRUN |
| <code>req.14482</code> | ac_3 | apps/bridge/package.json scripts (apps/bridge/package.json:5-10) gain test = "vitest run" and typecheck = "tsc --noEmit -p tsconfig.json". |
| <code>acceptance.14.517.1</code> | req | 14517.1 |
| <code>acceptance.14.517.1</code> | scenario | Given apps/bridge/package.json after the change, when manifest.spec.ts lists dependency names, then none starts with @opentelemetry/ |
| <code>acceptance.14.517.1</code> | test | apps/bridge/src/test/manifest.spec.ts::declaresNoTelemetryPackages (planned) |
| <code>acceptance.14.517.1</code> | qualification | UNRUN |
| <code>req.14517</code> | ac_1 | apps/bridge/package.json declares no @opentelemetry/* package once apps/bridge/instrumentation.node.ts is deleted; at HEAD apps/bridge/package.json:27-36 lists ten of them and the file is never registered, since the app has no instrumentation.ts and it ignores /explorer/ asset paths (apps/bridge/instrumentation.node.ts:48-49). |
| <code>capability.14.41.1</code> | title | Resolve the @/ alias and run the bridge app tests in node without missing setup files |
| <code>capability.14.41.1</code> | reqs | 14481.1<br>14481.2 |
| <code>capability.14.41.2</code> | title | Declare every imported package, drop the copied unused ones and add test and typecheck scripts |
| <code>capability.14.41.2</code> | reqs | 14482.1<br>14482.2<br>14482.3 |
| <code>capability.14.56.1</code> | title | Retire the copied explorer telemetry hook and its unused OpenTelemetry packages |
| <code>capability.14.56.1</code> | reqs | 14517.1 |
| <code>resolution.17.957</code> | title | decision.1443 binds: The copied telemetry hook is retired, not registered |
| <code>resolution.17.957</code> | applies_to | decision.1443<br>task.14.56.1 |
| <code>resolution.17.957</code> | source | decision.1443 in fragments/J55/decision.kvx; tasks from affects |

## ledger

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-components-bridge-import-transfer-importtransfer-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/components/bridge/import-transfer.tsx</code> (planned) | <code>export function ImportTransfer({ onImported }: { onImported: () =&gt; void })</code> |
| <code>interface.14.ts-bridge-lib-bridge-enrich-enrichrecord-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/enrich.ts</code> (planned) | <code>export function enrichRecord(transfer: BridgeTransfer, receipt: TransactionReceipt): BridgeTransfer</code> |
| <code>interface.14.ts-bridge-lib-bridge-import-importdeposit-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/import.ts</code> (planned) | <code>export function importDeposit(chain: BridgeChain, receipt: TransactionReceipt): BridgeTransfer</code> |
| <code>interface.14.ts-bridge-lib-bridge-import-importwithdrawal-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/import.ts</code> (planned) | <code>export function importWithdrawal(receipt: TransactionReceipt): BridgeTransfer</code> |
| <code>interface.14.ts-bridge-lib-bridge-submitted-depositrecord-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/submitted.ts</code> (planned) | <code>export function depositRecord(input: DepositRecordInput): BridgeTransfer</code> |
| <code>interface.14.ts-bridge-lib-bridge-submitted-withdrawalrecord-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/submitted.ts</code> (planned) | <code>export function withdrawalRecord(input: WithdrawalRecordInput): BridgeTransfer</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-transferidof-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/transfers.ts</code> (planned) | <code>export function transferIdOf(direction: TransferDirection, sourceTxHash: Hex): string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-components-bridge-import-transfer-importtransfer-j55</code> | notes | created by task 14.48.3; import form in the transfers panel |
| <code>interface.14.ts-bridge-lib-bridge-enrich-enrichrecord-j55</code> | notes | created by task 14.48.4; receipt enrichment of one ledger record |
| <code>interface.14.ts-bridge-lib-bridge-import-importdeposit-j55</code> | notes | created by task 14.48.3; ledger record from a vault deposit receipt |
| <code>interface.14.ts-bridge-lib-bridge-import-importwithdrawal-j55</code> | notes | created by task 14.48.3; ledger record from a bridgeOut receipt |
| <code>interface.14.ts-bridge-lib-bridge-submitted-depositrecord-j55</code> | notes | created by task 14.47.2; ledger record of a submitted deposit |
| <code>interface.14.ts-bridge-lib-bridge-submitted-withdrawalrecord-j55</code> | notes | created by task 14.47.2; ledger record of a submitted bridgeOut |
| <code>interface.14.ts-bridge-lib-bridge-transfers-transferidof-j55</code> | notes | created by task 14.45.1; id of a ledger record |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.488.1</code> | req | 14488.1 |
| <code>acceptance.14.488.1</code> | scenario | Given localStorage holding [null, {"id": 1}, a valid BridgeTransfer], when listTransfers is called for that transfer's account, then it returns only the valid transfer and does not throw |
| <code>acceptance.14.488.1</code> | test | apps/bridge/src/lib/bridge/transfers.spec.ts::listTransfersDropsMalformedEntries (planned) |
| <code>acceptance.14.488.1</code> | qualification | UNRUN |
| <code>req.14488</code> | ac_1 | listTransfers(account) skips stored rows that are not BridgeTransfer objects; at HEAD readAll at apps/bridge/src/lib/bridge/transfers.ts:55 trusts any array and apps/bridge/src/lib/bridge/transfers.ts:72 throws on a row without account. |
| <code>acceptance.14.488.2</code> | req | 14488.2 |
| <code>acceptance.14.488.2</code> | scenario | Given transferIdOf, when it is called with "in" and 0xABC... , then it returns "0xabc...:in" |
| <code>acceptance.14.488.2</code> | test | apps/bridge/src/lib/bridge/transfers.spec.ts::transferIdOfKeysBySourceHashAndDirection (planned) |
| <code>acceptance.14.488.2</code> | qualification | UNRUN |
| <code>req.14488</code> | ac_2 | transferIdOf(direction, sourceTxHash) returns `${lowercased hash}:${direction}`, replacing depositTransferId and withdrawalTransferId at apps/bridge/src/lib/bridge/transfers.ts:84-90. |
| <code>acceptance.14.488.3</code> | req | 14488.3 |
| <code>acceptance.14.488.3</code> | scenario | Given 50 stored transfers of account A and 1 of account B, when saveTransfer adds one more for A, then A has 50 records without its oldest and B keeps its record |
| <code>acceptance.14.488.3</code> | test | apps/bridge/src/lib/bridge/transfers.spec.ts::saveTransferKeepsFiftyRecordsPerAccount (planned) |
| <code>acceptance.14.488.3</code> | qualification | UNRUN |
| <code>req.14488</code> | ac_3 | saveTransfer keeps at most 50 records per account and evicts the oldest of that account only; at HEAD MAX_TRANSFERS at apps/bridge/src/lib/bridge/transfers.ts:48 is one global cap across accounts. |
| <code>acceptance.14.492.1</code> | req | 14492.1 |
| <code>acceptance.14.492.1</code> | scenario | Given a Base ETH deposit of 1 ETH with hash 0xab.. from account A to recipient R, when depositRecord runs, then the record has id transferIdOf("in", hash), vault set, amount "1000000000000000000" and requiredConfirmations 1800 |
| <code>acceptance.14.492.1</code> | test | apps/bridge/src/lib/bridge/submitted.spec.ts::depositRecordCarriesTheVaultAndFinalityDepth (planned) |
| <code>acceptance.14.492.1</code> | qualification | UNRUN |
| <code>req.14492</code> | ac_1 | depositRecord(input) returns a BridgeTransfer with direction in, the transferIdOf id, the vault, the recipient, amount as a decimal string and requiredConfirmations equal to the finality depth shown at apps/bridge/src/components/bridge/bridge-card.tsx:323; at HEAD saveTransfer (apps/bridge/src/lib/bridge/transfers.ts:75) has no caller, so nothing the card submits is ever listed. |
| <code>acceptance.14.492.2</code> | req | 14492.2 |
| <code>acceptance.14.492.2</code> | scenario | Given a withdrawal of 2 units to a Solana destination, when withdrawalRecord runs, then recipient equals the typed base58 destination and direction is out |
| <code>acceptance.14.492.2</code> | test | apps/bridge/src/lib/bridge/submitted.spec.ts::withdrawalRecordKeepsTheTypedDestination (planned) |
| <code>acceptance.14.492.2</code> | qualification | UNRUN |
| <code>req.14492</code> | ac_2 | withdrawalRecord(input) stores the destination exactly as typed in recipient, the foreign chain id as a decimal string and no vault, matching the BridgeTransfer fields at apps/bridge/src/lib/bridge/transfers.ts:11-45. |
| <code>acceptance.14.497.1</code> | req | 14497.1 |
| <code>acceptance.14.497.1</code> | scenario | Given a Base receipt holding one BridgeDeposit log of the configured vault, when importDeposit runs, then the record has direction in, the log's logIndex and nonce and the decoded paxeerRecipient |
| <code>acceptance.14.497.1</code> | test | apps/bridge/src/lib/bridge/import.spec.ts::importDepositBuildsARecordFromTheVaultLog (planned) |
| <code>acceptance.14.497.1</code> | qualification | UNRUN |
| <code>req.14497</code> | ac_1 | importDeposit(chain, receipt) returns a BridgeTransfer with logIndex, nonce, asset, amount and recipient read from the BridgeDeposit log of the configured vault (contracts/bridge/evm/src/PaxeerXVault.sol:40); the copy at apps/bridge/src/components/bridge/transfers-panel.tsx:308-309 promises that such a transfer is claimable but no import path exists at HEAD. |
| <code>acceptance.14.497.2</code> | req | 14497.2 |
| <code>acceptance.14.497.2</code> | scenario | Given a receipt whose only log comes from another address, when importDeposit runs, then it throws naming the configured vault |
| <code>acceptance.14.497.2</code> | test | apps/bridge/src/lib/bridge/import.spec.ts::importDepositRefusesAReceiptWithoutAVaultLog (planned) |
| <code>acceptance.14.497.2</code> | qualification | UNRUN |
| <code>req.14497</code> | ac_2 | importDeposit throws naming the vault when the receipt holds no BridgeDeposit log emitted by the configured vault address (apps/bridge/src/lib/bridge/chains.ts:130). |
| <code>acceptance.14.497.3</code> | req | 14497.3 |
| <code>acceptance.14.497.3</code> | scenario | Given a Paxeer receipt holding one BridgeOut log, when importWithdrawal runs, then the record has direction out and the log's nonce |
| <code>acceptance.14.497.3</code> | test | apps/bridge/src/lib/bridge/import.spec.ts::importWithdrawalReadsTheBridgeOutLog (planned) |
| <code>acceptance.14.497.3</code> | qualification | UNRUN |
| <code>req.14497</code> | ac_3 | importWithdrawal(receipt) returns a BridgeTransfer with bridgeOutNonce, chain, asset, amount and recipient read from the BridgeOut event at precompiles/layerxbridge/LayerXBridge.sol:41 emitted by 0x1016. |
| <code>acceptance.14.498.1</code> | req | 14498.1 |
| <code>acceptance.14.498.1</code> | scenario | Given a recorded deposit to recipient R and a receipt whose BridgeDeposit log has the same amount and sender but recipient S, when enrichRecord runs, then logIndex stays undefined |
| <code>acceptance.14.498.1</code> | test | apps/bridge/src/lib/bridge/enrich.spec.ts::enrichRecordIgnoresADepositLogForAnotherRecipient (planned) |
| <code>acceptance.14.498.1</code> | qualification | UNRUN |
| <code>req.14498</code> | ac_1 | enrichRecord(transfer, receipt) leaves logIndex unset when the only same-amount, same-sender BridgeDeposit log has another paxeerRecipient; at HEAD the matcher at apps/bridge/src/components/bridge/transfers-panel.tsx:73-77 compares amount and sender only. |
| <code>acceptance.14.498.2</code> | req | 14498.2 |
| <code>acceptance.14.498.2</code> | scenario | Given a recorded withdrawal and a receipt holding one BridgeOut log with nonce 7, when enrichRecord runs, then bridgeOutNonce is "7" |
| <code>acceptance.14.498.2</code> | test | apps/bridge/src/lib/bridge/enrich.spec.ts::enrichRecordReadsTheBridgeOutNonce (planned) |
| <code>acceptance.14.498.2</code> | qualification | UNRUN |
| <code>req.14498</code> | ac_2 | enrichRecord sets bridgeOutNonce from the BridgeOut log of 0x1016 for a withdrawal (apps/bridge/src/components/bridge/transfers-panel.tsx:85-88). |
| <code>capability.14.45.1</code> | title | Harden the local transfer ledger: drop malformed rows, key by source hash, cap per account |
| <code>capability.14.45.1</code> | reqs | 14488.1<br>14488.2<br>14488.3 |
| <code>capability.14.47.2</code> | title | Record every submitted deposit and withdrawal in the local ledger |
| <code>capability.14.47.2</code> | reqs | 14492.1<br>14492.2 |
| <code>capability.14.48.3</code> | title | Import a transfer started elsewhere from its source transaction hash |
| <code>capability.14.48.3</code> | reqs | 14497.1<br>14497.2<br>14497.3 |
| <code>capability.14.48.4</code> | title | Enrich ledger records through the asset- and recipient-bound log matchers |
| <code>capability.14.48.4</code> | reqs | 14498.1<br>14498.2 |
| <code>resolution.17.958</code> | title | decision.1444 binds: Submitted transfers are keyed by source hash and direction |
| <code>resolution.17.958</code> | applies_to | decision.1444<br>task.14.45.1<br>task.14.47.2<br>task.14.48.3 |
| <code>resolution.17.958</code> | source | decision.1444 in fragments/J55/decision.kvx; tasks from affects |

## lib

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-lib-bridge-amount-cappercent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/amount.ts:52</code> | <code>export function capPercent(part: bigint, whole: bigint): number</code> |
| <code>interface.14.ts-bridge-lib-bridge-amount-formatamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/amount.ts:31</code> | <code>export function formatAmount(value: bigint, decimals: number): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-amount-formatamountdisplay</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/amount.ts:42</code> | <code>export function formatAmountDisplay(value: bigint, decimals: number): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-amount-hasexcessprecision</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/amount.ts:22</code> | <code>export function hasExcessPrecision(input: string, decimals: number): boolean</code> |
| <code>interface.14.ts-bridge-lib-bridge-amount-parseamount</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/amount.ts:10</code> | <code>export function parseAmount(input: string, decimals: number): bigint &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationapierror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts:43</code> | <code>export class AttestationApiError extends Error {}</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts:15</code> | <code>export type AttestationStatus = "observed" &#124; "finalizing" &#124; "attesting" &#124; "ready" &#124; "unknown";</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-depositattestation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts:17</code> | <code>export type DepositAttestation =</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-fetchdepositattestation</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts:132</code> | <code>export async function fetchDepositAttestation(args: { chainId: bigint; txHash: Hex; logIndex: bigint; }): Promise&lt;DepositAttestation &#124; null&gt;</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-fetchreleaseattestation</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts:158</code> | <code>export async function fetchReleaseAttestation(args: { chainId: bigint; paxeerTxHash: Hex; paxeerNonce: bigint; }): Promise&lt;ReleaseAttestation &#124; null&gt;</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-releaseattestation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts:30</code> | <code>export type ReleaseAttestation =</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridge-chains</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/chains.ts:130</code> | <code>export const BRIDGE_CHAINS: BridgeChain[] = [</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgeasset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:47</code> | <code>export type BridgeAsset =</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgechain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:67</code> | <code>export type BridgeChain =</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgechainbyid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:251</code> | <code>export function bridgeChainById(chainId: bigint): BridgeChain &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgechainbykey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:247</code> | <code>export function bridgeChainByKey(key: string): BridgeChain &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-finalityestimate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:256</code> | <code>export function finalityEstimate(chain: BridgeChain): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-paxeer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:35</code> | <code>export const paxeer = defineChain(</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-paxeer-chain-id</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:33</code> | <code>export const PAXEER_CHAIN_ID = 125;</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-paxeer-route-key</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:245</code> | <code>export const PAXEER_ROUTE_KEY = "paxeer";</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-routestate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/chains.ts:65</code> | <code>export type RouteState = "configured" &#124; "not-deployed" &#124; "awaiting-backend";</code> |
| <code>interface.14.ts-bridge-lib-bridge-config-bridgeconfig</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/config.ts:30</code> | <code>export const bridgeConfig =</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-bank-address</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/contracts.ts:17</code> | <code>export const BANK_ADDRESS = "0x0000000000000000000000000000000000001001" as const;</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-bankabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/contracts.ts:66</code> | <code>export const bankAbi = parseAbi([</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-erc20abi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/contracts.ts:75</code> | <code>export const erc20Abi = parseAbi([</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-layerx-bridge-address</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/contracts.ts:16</code> | <code>export const LAYERX_BRIDGE_ADDRESS = "0x0000000000000000000000000000000000001016" as const;</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-layerxbridgeabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/contracts.ts:19</code> | <code>export const layerxBridgeAbi = layerxBridgeAbiJson as unknown as Abi;</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-paxeerxvaultabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/contracts.ts:22</code> | <code>export const paxeerXVaultAbi = parseAbi([</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-domain-in</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:29</code> | <code>export const DOMAIN_IN = "PAXEERX_BRIDGE_IN_V1";</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-domain-out</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:31</code> | <code>export const DOMAIN_OUT = "PAXEERX_BRIDGE_OUT_V1";</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-inbounddigest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:69</code> | <code>export function inboundDigest(args: Parameters&lt;typeof inboundPreimage&gt;[0]): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-inboundpreimage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:45</code> | <code>export function inboundPreimage(args: { chainId: bigint; vault: Address; txHash: Hex; logIndex: bigint; recipient32: Hex; asset: Address; amount: bigint; }): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-outbounddigest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:104</code> | <code>export function outboundDigest(args: Parameters&lt;typeof outboundPreimage&gt;[0]): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-outboundpreimage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:80</code> | <code>export function outboundPreimage(args: { chainId: bigint; vault: Address; paxeerTxHash: Hex; paxeerNonce: bigint; recipient: Address; asset: Address; amount: bigint; }): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-paxeerrecipient</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:36</code> | <code>export function paxeerRecipient(address: Address): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-releasenullifier</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:109</code> | <code>export function releaseNullifier(paxeerTxHash: Hex, paxeerNonce: bigint): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-signaturecheck</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/digests.ts:113</code> | <code>export type SignatureCheck =</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-verifythresholdsignatures</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/digests.ts:122</code> | <code>export async function verifyThresholdSignatures(args: { digest: Hex; signatures: readonly Hex[]; attestors: readonly Address[]; threshold: number; }): Promise&lt;SignatureCheck&gt;</code> |
| <code>interface.14.ts-bridge-lib-bridge-errors-describewriteerror</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/errors.ts:34</code> | <code>export function describeWriteError(error: unknown): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-foreignaddressurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/explorer.ts:33</code> | <code>export function foreignAddressUrl(chain: BridgeChain, address: Address): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-foreigntxurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/explorer.ts:20</code> | <code>export function foreignTxUrl(chain: BridgeChain, hash: Hex): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/explorer.ts:46</code> | <code>export { paxeer };</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeeraddressurl</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/explorer.ts:29</code> | <code>export function paxeerAddressUrl(address: Address): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeertxurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/explorer.ts:25</code> | <code>export function paxeerTxUrl(hash: Hex): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-txurlbyroutekey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/explorer.ts:40</code> | <code>export function txUrlByRouteKey(routeKey: string, hash: Hex): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-base58decode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/solana.ts:18</code> | <code>export function base58Decode(input: string): Uint8Array &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-bytestohex</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/solana.ts:45</code> | <code>export function bytesToHex(bytes: Uint8Array): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-issolanaaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/solana.ts:50</code> | <code>export function isSolanaAddress(input: string): boolean</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-solanarecipienthandle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/solana.ts:56</code> | <code>export function solanaRecipientHandle(base58Address: string): Address &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-bridgetransfer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/transfers.ts:11</code> | <code>export type BridgeTransfer =</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-deposittransferid</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/bridge/src/lib/bridge/transfers.ts:84</code> | <code>export function depositTransferId(txHash: Hex, logIndex: bigint): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-listtransfers</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/transfers.ts:69</code> | <code>export function listTransfers(account?: Address): BridgeTransfer[]</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-savetransfer</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/transfers.ts:75</code> | <code>export function saveTransfer(transfer: BridgeTransfer): void</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-transferdirection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/transfers.ts:9</code> | <code>export type TransferDirection = "in" &#124; "out";</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-updatetransfer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/transfers.ts:80</code> | <code>export function updateTransfer(id: string, patch: Partial&lt;BridgeTransfer&gt;): void</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-withdrawaltransferid</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/bridge/src/lib/bridge/transfers.ts:88</code> | <code>export function withdrawalTransferId(txHash: Hex, nonce: bigint): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-attestationapiconfigured</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:25</code> | <code>export const attestationApiConfigured = bridgeConfig.attestationApiUrl !== undefined;</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-paxeerconfigured</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:24</code> | <code>export const paxeerConfigured = bridgeConfig.paxeerRpcUrl !== undefined;</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-paxeerpairstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:30</code> | <code>export type PaxeerPairState =</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usebridgedbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:199</code> | <code>export function useBridgedBalance(denom: string &#124; undefined)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usedepositattestation</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:215</code> | <code>export function useDepositAttestation(transfer: BridgeTransfer &#124; undefined, settled: boolean)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usedepositnullified</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:255</code> | <code>export function useDepositNullified(transfer: BridgeTransfer &#124; undefined)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-useforeignbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:188</code> | <code>export function useForeignBalance(chain: BridgeChain, assetAddress: Address, isNative: boolean)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usepaxeerpairstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:46</code> | <code>export function usePaxeerPairState(chain: BridgeChain, assetId: Address): PaxeerPairState</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usereleaseattestation</code> | <code>ts_export</code> | <code>change</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:235</code> | <code>export function useReleaseAttestation(transfer: BridgeTransfer &#124; undefined, settled: boolean)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usevaultstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:121</code> | <code>export function useVaultState(chain: BridgeChain, assetAddress: Address): VaultState</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-vaultstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts:110</code> | <code>export type VaultState =</code> |
| <code>interface.14.ts-bridge-lib-bridge-wagmi-wagmiconfig</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/bridge/wagmi.ts:16</code> | <code>export const wagmiConfig = createConfig(</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-drawer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:8</code> | <code>export const EASE_DRAWER = [0.32, 0.72, 0, 1] as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-drawer-css</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:12</code> | <code>export const EASE_DRAWER_CSS = "cubic-bezier(0.32, 0.72, 0, 1)";</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-in-out</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:7</code> | <code>export const EASE_IN_OUT = [0.77, 0, 0.175, 1] as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-in-out-css</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:11</code> | <code>export const EASE_IN_OUT_CSS = "cubic-bezier(0.77, 0, 0.175, 1)";</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-out</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:6</code> | <code>export const EASE_OUT = [0.23, 1, 0.32, 1] as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-out-css</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:10</code> | <code>export const EASE_OUT_CSS = "cubic-bezier(0.23, 1, 0.32, 1)";</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-glide</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:25</code> | <code>export const SPRING_GLIDE = { stiffness: 340, damping: 34, mass: 0.6 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-layout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:15</code> | <code>export const SPRING_LAYOUT = { type: "spring", stiffness: 420, damping: 38, mass: 0.9 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-mouse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:23</code> | <code>export const SPRING_MOUSE = { stiffness: 260, damping: 22, mass: 0.5 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-panel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:19</code> | <code>export const SPRING_PANEL = { type: "spring", stiffness: 380, damping: 34, mass: 1 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-press</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:17</code> | <code>export const SPRING_PRESS = { type: "spring", stiffness: 600, damping: 30, mass: 0.6 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-swap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/ease.ts:21</code> | <code>export const SPRING_SWAP = { type: "spring", stiffness: 520, damping: 36, mass: 0.7 } as const;</code> |
| <code>interface.14.ts-bridge-lib-trading-decimalsfromtick</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:44</code> | <code>export function decimalsFromTick(tick: number)</code> |
| <code>interface.14.ts-bridge-lib-trading-formatcompact</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:109</code> | <code>export const formatCompact = (value: number, dp = 2) =&gt; formatNumber(value, "compact", { dp });</code> |
| <code>interface.14.ts-bridge-lib-trading-formatcountdown</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:147</code> | <code>export function formatCountdown(ms: number)</code> |
| <code>interface.14.ts-bridge-lib-trading-formatnumber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:83</code> | <code>export function formatNumber(value: number, rule: NumberRule = "number", { dp, tick, sign, pad = true }: FormatOptions = {})</code> |
| <code>interface.14.ts-bridge-lib-trading-formatoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:65</code> | <code>export type FormatOptions =</code> |
| <code>interface.14.ts-bridge-lib-trading-formatpercent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:108</code> | <code>export const formatPercent = (fraction: number, sign = true, dp = 2) =&gt; formatNumber(fraction, "percent", { sign, dp });</code> |
| <code>interface.14.ts-bridge-lib-trading-formatprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:107</code> | <code>export const formatPrice = (value: number, tick?: number) =&gt; formatNumber(value, "price", { tick });</code> |
| <code>interface.14.ts-bridge-lib-trading-formatusd</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:110</code> | <code>export const formatUsd = (value: number, dp = 2) =&gt; formatNumber(value, "usd", { dp });</code> |
| <code>interface.14.ts-bridge-lib-trading-numberrule</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:63</code> | <code>export type NumberRule = "price" &#124; "number" &#124; "percent" &#124; "compact" &#124; "usd";</code> |
| <code>interface.14.ts-bridge-lib-trading-orderside</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:37</code> | <code>export type OrderSide = "buy" &#124; "sell";</code> |
| <code>interface.14.ts-bridge-lib-trading-parsedsymbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:114</code> | <code>export type ParsedSymbol = { base: string; quote: string; type: "PERP" &#124; "SPOT" &#124; string };</code> |
| <code>interface.14.ts-bridge-lib-trading-parsesymbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:120</code> | <code>export function parseSymbol(symbol: string): ParsedSymbol</code> |
| <code>interface.14.ts-bridge-lib-trading-shortenaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:138</code> | <code>export function shortenAddress(address: string, head = 6, tail = 4)</code> |
| <code>interface.14.ts-bridge-lib-trading-side-fill</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:32</code> | <code>export const SIDE_FILL =</code> |
| <code>interface.14.ts-bridge-lib-trading-sidetone</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:39</code> | <code>export const sideTone = (side: OrderSide): Tone =&gt; (side === "buy" ? "up" : "down");</code> |
| <code>interface.14.ts-bridge-lib-trading-tone</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:9</code> | <code>export type Tone = "up" &#124; "down" &#124; "flat";</code> |
| <code>interface.14.ts-bridge-lib-trading-tone-bg</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:25</code> | <code>export const TONE_BG: Record&lt;Tone, string&gt; =</code> |
| <code>interface.14.ts-bridge-lib-trading-tone-text</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:18</code> | <code>export const TONE_TEXT: Record&lt;Tone, string&gt; =</code> |
| <code>interface.14.ts-bridge-lib-trading-toneof</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:12</code> | <code>export function toneOf(value: number &#124; null &#124; undefined): Tone</code> |
| <code>interface.14.ts-bridge-lib-trading-usecountdown</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/trading.ts:157</code> | <code>export function useCountdown(target: number &#124; Date &#124; null &#124; undefined)</code> |
| <code>interface.14.ts-bridge-lib-utils-cn</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/bridge/src/lib/utils.ts:4</code> | <code>export function cn(...inputs: ClassValue[])</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-lib-bridge-amount-parseamount</code> | overridden_by | override.J55.1 |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationapierror</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationstatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-fetchdepositattestation</code> | overridden_by | override.J55.3 |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-fetchreleaseattestation</code> | overridden_by | override.J55.4 |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridge-chains</code> | overridden_by | override.J55.20 |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgeasset</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgechainbyid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-chains-paxeer-chain-id</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-chains-routestate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-config-bridgeconfig</code> | overridden_by | override.J55.19 |
| <code>interface.14.ts-bridge-lib-bridge-digests-domain-in</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-digests-domain-out</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-digests-inboundpreimage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-digests-outboundpreimage</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-digests-releasenullifier</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-digests-signaturecheck</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-digests-verifythresholdsignatures</code> | overridden_by | override.J55.2 |
| <code>interface.14.ts-bridge-lib-bridge-errors-describewriteerror</code> | overridden_by | override.J55.18 |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeer</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeeraddressurl</code> | overridden_by | override.J55.21 |
| <code>interface.14.ts-bridge-lib-bridge-explorer-txurlbyroutekey</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-solana-base58decode</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-solana-bytestohex</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-transfers-deposittransferid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-transfers-deposittransferid</code> | overridden_by | override.J55.9 |
| <code>interface.14.ts-bridge-lib-bridge-transfers-listtransfers</code> | overridden_by | override.J55.7 |
| <code>interface.14.ts-bridge-lib-bridge-transfers-savetransfer</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-transfers-savetransfer</code> | overridden_by | override.J55.8 |
| <code>interface.14.ts-bridge-lib-bridge-transfers-transferdirection</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-transfers-withdrawaltransferid</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-transfers-withdrawaltransferid</code> | overridden_by | override.J55.10 |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-paxeerpairstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usedepositattestation</code> | overridden_by | override.J55.5 |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usereleaseattestation</code> | overridden_by | override.J55.6 |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-vaultstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-ease-drawer</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-ease-drawer-css</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-ease-in-out</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-ease-in-out-css</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-ease-out</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-ease-out-css</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-spring-glide</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-spring-mouse</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-spring-panel</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-spring-press</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-ease-spring-swap</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-decimalsfromtick</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-formatcompact</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-formatcountdown</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-formatoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-formatpercent</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-formatprice</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-formatusd</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-orderside</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-parsedsymbol</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-side-fill</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-sidetone</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-tone</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-tone-bg</code> | notes | no in-repo importer at HEAD |
| <code>interface.14.ts-bridge-lib-trading-usecountdown</code> | notes | no in-repo importer at HEAD |

## routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-bridge-get</code> | <code>http_route</code> | <code>stable</code> | <code>apps/bridge/src/app/page.tsx:12</code> | <code>GET / (page)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-bridge-get</code> | notes | site root |

## security-headers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-lib-bridge-csp-contentsecuritypolicy-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/csp.ts</code> (planned) | <code>export function contentSecurityPolicy(config: BridgeConfig): string</code> |
| <code>interface.14.ts-bridge-middleware-config-j55</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/bridge/middleware.ts:12</code> | <code>export const config = { matcher: [...] }</code> |
| <code>interface.14.ts-bridge-middleware-middleware-j55</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/bridge/middleware.ts:3</code> | <code>export function middleware(request: NextRequest)</code> |
| <code>interface.14.ts-bridge-proxy-config-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/proxy.ts</code> (planned) | <code>export const config = { matcher: ["/((?!_next/static&#124;_next/image&#124;favicon.ico).*)"] }</code> |
| <code>interface.14.ts-bridge-proxy-proxy-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/proxy.ts</code> (planned) | <code>export function proxy(request: NextRequest): NextResponse</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-lib-bridge-csp-contentsecuritypolicy-j55</code> | notes | created by task 14.52.1; CSP from configured origins |
| <code>interface.14.ts-bridge-middleware-config-j55</code> | notes | removed by task 14.52.1; moves to src/proxy.ts |
| <code>interface.14.ts-bridge-middleware-middleware-j55</code> | notes | removed by task 14.52.1; outside src, never run by the app router |
| <code>interface.14.ts-bridge-proxy-config-j55</code> | notes | created by task 14.52.1; proxy matcher |
| <code>interface.14.ts-bridge-proxy-proxy-j55</code> | notes | created by task 14.52.1; request proxy that sets the response security headers |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.502.1</code> | req | 14502.1 |
| <code>acceptance.14.502.1</code> | scenario | Given src/proxy.ts, when proxy is called with a NextRequest for /, then the five headers are present on the response |
| <code>acceptance.14.502.1</code> | test | apps/bridge/src/proxy.spec.ts::proxySetsTheSecurityHeaders (planned) |
| <code>acceptance.14.502.1</code> | qualification | UNRUN |
| <code>req.14502</code> | ac_1 | proxy(request) returns a response with X-Content-Type-Options nosniff, X-Frame-Options DENY, Referrer-Policy strict-origin-when-cross-origin, the Permissions-Policy of apps/bridge/middleware.ts:5-8 and a Content-Security-Policy header; at HEAD apps/bridge/middleware.ts:3 sits outside apps/bridge/src, which holds the app router, so it never runs. |
| <code>acceptance.14.502.2</code> | req | 14502.2 |
| <code>acceptance.14.502.2</code> | scenario | Given a config with a Paxeer RPC URL and an attestation API URL, when contentSecurityPolicy runs, then connect-src lists exactly self and those two origins and frame-ancestors is 'none' |
| <code>acceptance.14.502.2</code> | test | apps/bridge/src/proxy.spec.ts::contentSecurityPolicyAllowsOnlyConfiguredOrigins (planned) |
| <code>acceptance.14.502.2</code> | qualification | UNRUN |
| <code>req.14502</code> | ac_2 | contentSecurityPolicy(config) lists in connect-src only self, the Paxeer RPC origin, the attestation API origin and the configured foreign RPC origins read by readConfig (apps/bridge/src/lib/bridge/config.ts:30), and sets frame-ancestors 'none'. |
| <code>capability.14.52.1</code> | title | Move the ignored root middleware to src/proxy.ts and add a configured-origin CSP |
| <code>capability.14.52.1</code> | reqs | 14502.1<br>14502.2 |

## settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-releasecheckof-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> (planned) | <code>export function releaseCheckOf(transfer: BridgeTransfer): ReadContractParameters &#124; undefined</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-settledrefusal-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> (planned) | <code>export function settledRefusal(transfer: BridgeTransfer, live: TransferLive): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-import-matchdepositlog-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/import.ts</code> (planned) | <code>export function matchDepositLog(transfer: BridgeTransfer, logs: readonly Log[]): { logIndex: bigint; nonce: bigint } &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-import-noncefrombridgeoutlogs-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/import.ts</code> (planned) | <code>export function nonceFromBridgeOutLogs(logs: readonly Log[]): bigint &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-phase-historystatusof-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/phase.ts</code> (planned) | <code>export function historyStatusOf(phase: TransferPhase): TransferRecordStatus</code> |
| <code>interface.14.ts-bridge-lib-bridge-phase-stepof-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/phase.ts</code> (planned) | <code>export function stepOf(phase: TransferPhase): TransferStep</code> |
| <code>interface.14.ts-bridge-lib-bridge-settlement-phaseof-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/settlement.ts</code> (planned) | <code>export function phaseOf(transfer: BridgeTransfer, live: TransferLive &#124; undefined, attestationReady: boolean): TransferPhase</code> |
| <code>interface.14.ts-bridge-lib-bridge-settlement-settlementpatch-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/settlement.ts</code> (planned) | <code>export function settlementPatch(transfer: BridgeTransfer, live: TransferLive, now: number): Partial&lt;BridgeTransfer&gt; &#124; undefined</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-releasecheckof-j55</code> | notes | created by task 14.46.2; vault nullified read for a withdrawal |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-settledrefusal-j55</code> | notes | created by task 14.46.2; why a settled transfer offers no action |
| <code>interface.14.ts-bridge-lib-bridge-import-matchdepositlog-j55</code> | notes | created by task 14.48.1; deposit identity from a receipt |
| <code>interface.14.ts-bridge-lib-bridge-import-noncefrombridgeoutlogs-j55</code> | notes | created by task 14.48.1; BridgeOut nonce from 0x1016 logs only |
| <code>interface.14.ts-bridge-lib-bridge-phase-historystatusof-j55</code> | notes | created by task 14.59.1; history row status of a phase |
| <code>interface.14.ts-bridge-lib-bridge-phase-stepof-j55</code> | notes | created by task 14.59.1; status chip step of a phase |
| <code>interface.14.ts-bridge-lib-bridge-settlement-phaseof-j55</code> | notes | created by task 14.48.2; transfer phase from record and chain state |
| <code>interface.14.ts-bridge-lib-bridge-settlement-settlementpatch-j55</code> | notes | created by task 14.48.2; ledger patch for a chain-observed settlement |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.490.1</code> | req | 14490.1 |
| <code>acceptance.14.490.1</code> | scenario | Given a withdrawal with vault V, sourceTxHash H and bridgeOutNonce 4 on ethereum, when releaseCheckOf runs, then it returns functionName nullified, address V, args [releaseNullifier(H, 4n)] and chainId 1 |
| <code>acceptance.14.490.1</code> | test | apps/bridge/src/components/bridge/transfer-actions.spec.ts::releaseCheckOfBuildsTheVaultNullifiedCall (planned) |
| <code>acceptance.14.490.1</code> | qualification | UNRUN |
| <code>req.14490</code> | ac_1 | releaseCheckOf(transfer) returns a readContract request for nullified(releaseNullifier(sourceTxHash, bridgeOutNonce)) on transfer.vault (apps/bridge/src/lib/bridge/contracts.ts:29, apps/bridge/src/lib/bridge/digests.ts:109); at HEAD readTransferLive at apps/bridge/src/components/bridge/transfer-actions.ts:54-62 reads only isNullified for deposits. |
| <code>acceptance.14.490.2</code> | req | 14490.2 |
| <code>acceptance.14.490.2</code> | scenario | Given a withdrawal record without bridgeOutNonce, when releaseCheckOf runs, then it returns undefined |
| <code>acceptance.14.490.2</code> | test | apps/bridge/src/components/bridge/transfer-actions.spec.ts::releaseCheckOfWaitsForTheNonce (planned) |
| <code>acceptance.14.490.2</code> | qualification | UNRUN |
| <code>req.14490</code> | ac_2 | releaseCheckOf returns undefined for a withdrawal without bridgeOutNonce (apps/bridge/src/lib/bridge/transfers.ts:42). |
| <code>acceptance.14.490.3</code> | req | 14490.3 |
| <code>acceptance.14.490.3</code> | scenario | Given a deposit and a TransferLive with credited true, when settledRefusal runs, then it returns the already-credited sentence, and for credited false it returns undefined |
| <code>acceptance.14.490.3</code> | test | apps/bridge/src/components/bridge/transfer-actions.spec.ts::settledRefusalNamesTheCreditedDeposit (planned) |
| <code>acceptance.14.490.3</code> | qualification | UNRUN |
| <code>req.14490</code> | ac_3 | settledRefusal(transfer, live) returns a sentence stating the deposit is already credited when live.credited is true, so the claim button is not offered (apps/bridge/src/components/bridge/transfer-actions.ts:28). |
| <code>acceptance.14.495.1</code> | req | 14495.1 |
| <code>acceptance.14.495.1</code> | scenario | Given a receipt with two BridgeDeposit logs of equal amount and sender but different assets, when matchDepositLog runs for the second asset, then it returns the second log's logIndex |
| <code>acceptance.14.495.1</code> | test | apps/bridge/src/lib/bridge/import.spec.ts::matchDepositLogRequiresTheAssetAndRecipient (planned) |
| <code>acceptance.14.495.1</code> | qualification | UNRUN |
| <code>req.14495</code> | ac_1 | matchDepositLog(transfer, logs) returns the BridgeDeposit log whose vault address, sender, amount, asset and paxeerRecipient all equal the transfer; at HEAD apps/bridge/src/components/bridge/transfers-panel.tsx:73-77 matches by amount and sender only. |
| <code>acceptance.14.495.2</code> | req | 14495.2 |
| <code>acceptance.14.495.2</code> | scenario | Given a log list holding one BridgeOut log from 0x...1016 with nonce 9, when nonceFromBridgeOutLogs runs, then it returns 9n |
| <code>acceptance.14.495.2</code> | test | apps/bridge/src/lib/bridge/import.spec.ts::nonceFromBridgeOutLogsReadsTheNonce (planned) |
| <code>acceptance.14.495.2</code> | qualification | UNRUN |
| <code>req.14495</code> | ac_2 | nonceFromBridgeOutLogs(logs) returns the nonce of the BridgeOut event emitted by 0x0000000000000000000000000000000000001016 (apps/bridge/src/lib/bridge/contracts.ts:16). |
| <code>acceptance.14.495.3</code> | req | 14495.3 |
| <code>acceptance.14.495.3</code> | scenario | Given a log list whose only BridgeOut-shaped log comes from 0x...dead, when nonceFromBridgeOutLogs runs, then it returns undefined |
| <code>acceptance.14.495.3</code> | test | apps/bridge/src/lib/bridge/import.spec.ts::nonceFromBridgeOutLogsIgnoresAnotherAddress (planned) |
| <code>acceptance.14.495.3</code> | qualification | UNRUN |
| <code>req.14495</code> | ac_3 | nonceFromBridgeOutLogs ignores a BridgeOut-shaped log from any other address; at HEAD bridgeOutNonceFromReceipt at apps/bridge/src/components/bridge/transfer-actions.ts:253 parses every log. |
| <code>acceptance.14.496.1</code> | req | 14496.1 |
| <code>acceptance.14.496.1</code> | scenario | Given a deposit without claimTxHash and live.credited true, when phaseOf runs, then it returns settled |
| <code>acceptance.14.496.1</code> | test | apps/bridge/src/lib/bridge/settlement.spec.ts::phaseOfSettlesADepositThePrecompileNullified (planned) |
| <code>acceptance.14.496.1</code> | qualification | UNRUN |
| <code>req.14496</code> | ac_1 | phaseOf returns settled for a deposit when live.credited is true even without claimTxHash (apps/bridge/src/components/bridge/transfers-panel.tsx:102-113). |
| <code>acceptance.14.496.2</code> | req | 14496.2 |
| <code>acceptance.14.496.2</code> | scenario | Given a credited deposit without claimedAt, when settlementPatch runs twice with the first patch applied, then the first call returns {claimedAt} and the second returns undefined |
| <code>acceptance.14.496.2</code> | test | apps/bridge/src/lib/bridge/settlement.spec.ts::settlementPatchRecordsAChainObservedClaimOnce (planned) |
| <code>acceptance.14.496.2</code> | qualification | UNRUN |
| <code>req.14496</code> | ac_2 | settlementPatch(transfer, live, now) returns {claimedAt: now} for a credited deposit without claimedAt and undefined once claimedAt is set; at HEAD the effect at apps/bridge/src/components/bridge/transfers-panel.tsx:150-154 calls onChanged without writing, and onChanged is a new function each render at apps/bridge/src/components/bridge/transfers-panel.tsx:317. |
| <code>acceptance.14.520.1</code> | req | 14520.1 |
| <code>acceptance.14.520.1</code> | scenario | Given the phases confirming, attesting, ready, settled and failed, when stepOf runs on each, then it returns confirming, attesting, ready, credited and failed |
| <code>acceptance.14.520.1</code> | test | apps/bridge/src/lib/bridge/phase.spec.ts::stepOfKeepsAttestingAndReadyApart (planned) |
| <code>acceptance.14.520.1</code> | qualification | UNRUN |
| <code>req.14520</code> | ac_1 | stepOf(phase) maps attesting to attesting and ready to ready; at HEAD apps/bridge/src/components/bridge/transfers-panel.tsx:207-214 folds both into submitted because TransferStep at apps/bridge/src/components/trading/deposit-status.tsx:21 has no such members. |
| <code>acceptance.14.520.2</code> | req | 14520.2 |
| <code>acceptance.14.520.2</code> | scenario | Given phase failed, when historyStatusOf runs, then it returns failed, and for settled it returns completed |
| <code>acceptance.14.520.2</code> | test | apps/bridge/src/lib/bridge/phase.spec.ts::historyStatusOfReportsAFailedSource (planned) |
| <code>acceptance.14.520.2</code> | qualification | UNRUN |
| <code>req.14520</code> | ac_2 | historyStatusOf(phase) returns failed for a reverted source transaction; at HEAD apps/bridge/src/components/bridge/transfers-panel.tsx:286 reports every unsettled row as processing although TransferRecordStatus (apps/bridge/src/components/trading/transfer-history.tsx:23) has failed. |
| <code>capability.14.46.2</code> | title | Read withdrawal release state from the vault and name the settled refusal |
| <code>capability.14.46.2</code> | reqs | 14490.1<br>14490.2<br>14490.3 |
| <code>capability.14.48.1</code> | title | Add receipt log matchers that bind a deposit to asset and recipient and read BridgeOut from 0x1016 |
| <code>capability.14.48.1</code> | reqs | 14495.1<br>14495.2<br>14495.3 |
| <code>capability.14.48.2</code> | title | Persist chain-observed deposit claims once and stop the transfers panel refetch loop |
| <code>capability.14.48.2</code> | reqs | 14496.1<br>14496.2 |
| <code>capability.14.59.1</code> | title | Show attesting and ready steps and failed history rows in the transfers panel |
| <code>capability.14.59.1</code> | reqs | 14520.1<br>14520.2 |

## solana-route

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-lib-bridge-solana-parserecipientrecord-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> (planned) | <code>export function parseRecipientRecord(bytes: Uint8Array, handle: Address): Uint8Array &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordaddress-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> (planned) | <code>export function recipientRecordAddress(programId: string, handle: Address): { address: string; bump: number }</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordfromresponse-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> (planned) | <code>export function recipientRecordFromResponse(response: unknown, handle: Address): { registered: boolean; key?: Uint8Array }</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordrequest-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> (planned) | <code>export function recipientRecordRequest(programId: string, destination: string): JsonRpcRequest &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-route-solanawithdrawalblocker-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/solana-route.ts</code> (planned) | <code>export function solanaWithdrawalBlocker(lookup: RecipientLookup &#124; undefined, destination: string): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-userecipientrecord-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> (planned) | <code>export function useRecipientRecord(destination: string &#124; undefined)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-lib-bridge-solana-parserecipientrecord-j55</code> | notes | created by task 14.54.1; decodes the 62-byte recipient record |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordaddress-j55</code> | notes | created by task 14.54.1; recipient record PDA |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordfromresponse-j55</code> | notes | created by task 14.54.2; parses the getAccountInfo answer |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordrequest-j55</code> | notes | created by task 14.54.2; getAccountInfo body |
| <code>interface.14.ts-bridge-lib-bridge-solana-route-solanawithdrawalblocker-j55</code> | notes | created by task 14.54.3; submit blocker for a Solana withdrawal |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-userecipientrecord-j55</code> | notes | created by task 14.54.2; recipient record lookup hook |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.509.1</code> | req | 14509.1 |
| <code>acceptance.14.509.1</code> | scenario | Given a program id and a handle, when recipientRecordAddress runs, then the returned address fails ed25519 point decoding and re-deriving with the returned bump gives the same address |
| <code>acceptance.14.509.1</code> | test | apps/bridge/src/lib/bridge/solana.spec.ts::recipientRecordAddressIsOffCurve (planned) |
| <code>acceptance.14.509.1</code> | qualification | UNRUN |
| <code>req.14509</code> | ac_1 | recipientRecordAddress(programId, handle) returns the find_program_address result for seeds b"recipient" and the 20-byte handle (contracts/bridge/solana/src/state.rs:29), a point off the ed25519 curve. |
| <code>acceptance.14.509.2</code> | req | 14509.2 |
| <code>acceptance.14.509.2</code> | scenario | Given a 62-byte buffer with magic PXBRREC0, version 1, handle H and key K, when parseRecipientRecord runs with H, then it returns K |
| <code>acceptance.14.509.2</code> | test | apps/bridge/src/lib/bridge/solana.spec.ts::parseRecipientRecordReadsTheKey (planned) |
| <code>acceptance.14.509.2</code> | qualification | UNRUN |
| <code>req.14509</code> | ac_2 | parseRecipientRecord(bytes, handle) returns the 32-byte key at bytes 30..62 of a 62-byte record starting with PXBRREC0 and layout version 1 (contracts/bridge/solana/src/state.rs:22, contracts/bridge/solana/src/state.rs:43, contracts/bridge/solana/src/state.rs:338). |
| <code>acceptance.14.509.3</code> | req | 14509.3 |
| <code>acceptance.14.509.3</code> | scenario | Given a valid record for handle H, when parseRecipientRecord runs with another handle, then it returns undefined |
| <code>acceptance.14.509.3</code> | test | apps/bridge/src/lib/bridge/solana.spec.ts::parseRecipientRecordRefusesAnotherHandle (planned) |
| <code>acceptance.14.509.3</code> | qualification | UNRUN |
| <code>req.14509</code> | ac_3 | parseRecipientRecord returns undefined when bytes 10..30 differ from the expected handle, the magic differs or the length is not 62 (contracts/bridge/solana/src/state.rs:338-340). |
| <code>acceptance.14.510.1</code> | req | 14510.1 |
| <code>acceptance.14.510.1</code> | scenario | Given a program id and a base58 destination, when recipientRecordRequest runs, then method is getAccountInfo and params[0] equals recipientRecordAddress(programId, handle).address |
| <code>acceptance.14.510.1</code> | test | apps/bridge/src/lib/bridge/solana.spec.ts::recipientRecordRequestUsesGetAccountInfo (planned) |
| <code>acceptance.14.510.1</code> | qualification | UNRUN |
| <code>req.14510</code> | ac_1 | recipientRecordRequest(programId, destination) returns a JSON-RPC getAccountInfo body for the recipient record PDA of solanaRecipientHandle(destination) (apps/bridge/src/lib/bridge/solana.ts:56) with base64 encoding. |
| <code>acceptance.14.510.2</code> | req | 14510.2 |
| <code>acceptance.14.510.2</code> | scenario | Given a JSON-RPC response with result.value null, when recipientRecordFromResponse runs, then it returns registered false |
| <code>acceptance.14.510.2</code> | test | apps/bridge/src/lib/bridge/solana.spec.ts::recipientRecordFromResponseReportsAMissingRecord (planned) |
| <code>acceptance.14.510.2</code> | qualification | UNRUN |
| <code>req.14510</code> | ac_2 | recipientRecordFromResponse returns {registered: false} for a result value null, the state the relayer waits on as Recipient::Pending (services/interop/crates/layerx-bridge-relayer/src/journal.rs:362). |
| <code>acceptance.14.511.1</code> | req | 14511.1 |
| <code>acceptance.14.511.1</code> | scenario | Given a recipient lookup with registered false, when solanaWithdrawalBlocker runs, then it returns the register-first sentence |
| <code>acceptance.14.511.1</code> | test | apps/bridge/src/lib/bridge/solana-route.spec.ts::solanaWithdrawalBlockerRequiresARegisteredRecord (planned) |
| <code>acceptance.14.511.1</code> | qualification | UNRUN |
| <code>req.14511</code> | ac_1 | solanaWithdrawalBlocker({registered: false}) returns the register-first sentence; at HEAD bridge-card.tsx sends bridgeOut with solanaRecipientHandle(destination) at apps/bridge/src/components/bridge/bridge-card.tsx:274-285 without checking the record that register_recipient creates (contracts/bridge/solana/src/identity.rs:119). |
| <code>acceptance.14.511.2</code> | req | 14511.2 |
| <code>acceptance.14.511.2</code> | scenario | Given a registered record whose key is not base58Decode(destination), when solanaWithdrawalBlocker runs, then it returns the mismatch sentence, and for a matching key it returns undefined |
| <code>acceptance.14.511.2</code> | test | apps/bridge/src/lib/bridge/solana-route.spec.ts::solanaWithdrawalBlockerRefusesARecordForAnotherKey (planned) |
| <code>acceptance.14.511.2</code> | qualification | UNRUN |
| <code>req.14511</code> | ac_2 | solanaWithdrawalBlocker returns the mismatch sentence when the record's key differs from the decoded destination key (apps/bridge/src/lib/bridge/solana.ts:18). |
| <code>capability.14.54.1</code> | title | Derive and decode the Solana recipient record PDA in the client |
| <code>capability.14.54.1</code> | reqs | 14509.1<br>14509.2<br>14509.3 |
| <code>capability.14.54.2</code> | title | Look up the Solana recipient record over JSON-RPC from a hook |
| <code>capability.14.54.2</code> | reqs | 14510.1<br>14510.2 |
| <code>capability.14.54.3</code> | title | Block a Solana withdrawal until the destination's recipient record is registered |
| <code>capability.14.54.3</code> | reqs | 14511.1<br>14511.2 |

## submission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-lib-bridge-card-state-cardstate-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/card-state.ts</code> (planned) | <code>export function cardState(inputs: CardStateInputs): CardState</code> |
| <code>interface.14.ts-bridge-lib-bridge-limits-amountlimiterror-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/limits.ts</code> (planned) | <code>export function amountLimitError(args: AmountLimitArgs): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-limits-maxspendable-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/limits.ts</code> (planned) | <code>export function maxSpendable(balance: bigint, isNative: boolean, gasReserve: bigint): bigint</code> |
| <code>interface.14.ts-bridge-lib-bridge-limits-routeblockers-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/limits.ts</code> (planned) | <code>export function routeBlockers(args: RouteBlockerArgs): string[]</code> |
| <code>interface.14.ts-bridge-lib-bridge-submission-plandeposit-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/lib/bridge/submission.ts</code> (planned) | <code>export function planDeposit(args: DepositPlanArgs): DepositPlan</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-lib-bridge-card-state-cardstate-j55</code> | notes | created by task 14.47.4; derived submit state of BridgeCard |
| <code>interface.14.ts-bridge-lib-bridge-limits-amountlimiterror-j55</code> | notes | created by task 14.47.3; pure limit check used by BridgeCard |
| <code>interface.14.ts-bridge-lib-bridge-limits-maxspendable-j55</code> | notes | created by task 14.47.3; Max button amount |
| <code>interface.14.ts-bridge-lib-bridge-limits-routeblockers-j55</code> | notes | created by task 14.47.3; reasons the submit button is disabled |
| <code>interface.14.ts-bridge-lib-bridge-submission-plandeposit-j55</code> | notes | created by task 14.47.1; ordered approve and deposit requests |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.491.1</code> | req | 14491.1 |
| <code>acceptance.14.491.1</code> | scenario | Given an ERC-20 asset, amount 5 and allowance 5, when planDeposit runs, then steps is [deposit] |
| <code>acceptance.14.491.1</code> | test | apps/bridge/src/lib/bridge/submission.spec.ts::planDepositSkipsApproveWhenAllowanceCovers (planned) |
| <code>acceptance.14.491.1</code> | qualification | UNRUN |
| <code>req.14491</code> | ac_1 | planDeposit returns no approve step when allowance &gt;= amount; at HEAD needsApprove at apps/bridge/src/components/bridge/bridge-card.tsx:211 ignores the allowance. |
| <code>acceptance.14.491.2</code> | req | 14491.2 |
| <code>acceptance.14.491.2</code> | scenario | Given amount 5 and allowance 2, when planDeposit runs, then the first step is approve with args [vault, 5n] |
| <code>acceptance.14.491.2</code> | test | apps/bridge/src/lib/bridge/submission.spec.ts::planDepositApprovesTheExactAmount (planned) |
| <code>acceptance.14.491.2</code> | qualification | UNRUN |
| <code>req.14491</code> | ac_2 | planDeposit returns approve(vault, amount) with the exact amount, never maxUint256, when allowance &lt; amount (apps/bridge/src/lib/bridge/contracts.ts:75). |
| <code>acceptance.14.491.3</code> | req | 14491.3 |
| <code>acceptance.14.491.3</code> | scenario | Given the native ETH asset and amount 1 ETH, when planDeposit runs, then steps is [depositNative] with value 1 ETH |
| <code>acceptance.14.491.3</code> | test | apps/bridge/src/lib/bridge/submission.spec.ts::planDepositUsesDepositNativeForTheNativeAsset (planned) |
| <code>acceptance.14.491.3</code> | qualification | UNRUN |
| <code>req.14491</code> | ac_3 | planDeposit returns depositNative(paxeerRecipient) with value amount for the native asset and no approve (apps/bridge/src/lib/bridge/contracts.ts:24). |
| <code>acceptance.14.493.1</code> | req | 14493.1 |
| <code>acceptance.14.493.1</code> | scenario | Given direction out, outstanding 10 units and amount 11 units, when amountLimitError runs, then it returns the outstanding message |
| <code>acceptance.14.493.1</code> | test | apps/bridge/src/lib/bridge/limits.spec.ts::amountLimitErrorRefusesAWithdrawalAboveOutstanding (planned) |
| <code>acceptance.14.493.1</code> | qualification | UNRUN |
| <code>req.14493</code> | ac_1 | amountLimitError returns a message for a withdrawal larger than the vault's outstanding balance, which PaxeerXVault.release refuses with InsufficientOutstanding (contracts/bridge/evm/src/PaxeerXVault.sol:166-181); at HEAD apps/bridge/src/components/bridge/bridge-card.tsx:113-116 checks the total only for deposits. |
| <code>acceptance.14.493.2</code> | req | 14493.2 |
| <code>acceptance.14.493.2</code> | scenario | Given total 100, outstanding 95 and amount 6, when amountLimitError runs for in and for out with outstanding 95, then in returns the total-cap message and out returns undefined |
| <code>acceptance.14.493.2</code> | test | apps/bridge/src/lib/bridge/limits.spec.ts::amountLimitErrorAppliesInFlightCapOnlyToDeposits (planned) |
| <code>acceptance.14.493.2</code> | qualification | UNRUN |
| <code>req.14493</code> | ac_2 | amountLimitError applies the remaining total cap (total minus outstanding) only to direction in, matching apps/bridge/src/components/bridge/bridge-card.tsx:112-116. |
| <code>acceptance.14.493.3</code> | req | 14493.3 |
| <code>acceptance.14.493.3</code> | scenario | Given a native balance of 1 ETH and a reserve of 0.001 ETH, when maxSpendable runs, then it returns 0.999 ETH, and for an ERC-20 it returns the balance |
| <code>acceptance.14.493.3</code> | test | apps/bridge/src/lib/bridge/limits.spec.ts::maxSpendableReservesNativeGas (planned) |
| <code>acceptance.14.493.3</code> | qualification | UNRUN |
| <code>req.14493</code> | ac_3 | maxSpendable(balance, isNative, gasReserve) returns balance minus gasReserve for a native asset and the full balance otherwise; at HEAD Max at apps/bridge/src/components/bridge/bridge-card.tsx:374-378 fills the whole balance. |
| <code>acceptance.14.493.4</code> | req | 14493.4 |
| <code>acceptance.14.493.4</code> | scenario | Given a Paxeer pair state with error set, when routeBlockers runs, then the result contains the Paxeer read blocker |
| <code>acceptance.14.493.4</code> | test | apps/bridge/src/lib/bridge/limits.spec.ts::routeBlockersReportsAFailedPaxeerRead (planned) |
| <code>acceptance.14.493.4</code> | qualification | UNRUN |
| <code>req.14493</code> | ac_4 | routeBlockers lists a blocker when the Paxeer pair read errored; at HEAD the blockers at apps/bridge/src/components/bridge/bridge-card.tsx:129-153 ignore paxeerState.error. |
| <code>acceptance.14.494.1</code> | req | 14494.1 |
| <code>acceptance.14.494.1</code> | scenario | Given connected inputs on the source chain with a valid amount and a paused Paxeer pair, when cardState runs, then ready is false and blockers contains the paused sentence |
| <code>acceptance.14.494.1</code> | test | apps/bridge/src/lib/bridge/card-state.spec.ts::cardStateIsNotReadyWhileABlockerExists (planned) |
| <code>acceptance.14.494.1</code> | qualification | UNRUN |
| <code>req.14494</code> | ac_1 | cardState(inputs).ready is false whenever routeBlockers returns a blocker, the rule at apps/bridge/src/components/bridge/bridge-card.tsx:200-209 that today reads the inline list at apps/bridge/src/components/bridge/bridge-card.tsx:129-153. |
| <code>acceptance.14.494.2</code> | req | 14494.2 |
| <code>acceptance.14.494.2</code> | scenario | Given a native balance of 1 ETH and a gas reserve of 0.001 ETH, when cardState runs, then maxAmount is "0.999" |
| <code>acceptance.14.494.2</code> | test | apps/bridge/src/lib/bridge/card-state.spec.ts::cardStateMaxAmountReservesNativeGas (planned) |
| <code>acceptance.14.494.2</code> | qualification | UNRUN |
| <code>req.14494</code> | ac_2 | cardState(inputs).maxAmount is maxSpendable(balance, isNative, gasReserve) formatted without grouping, replacing the whole-balance fill at apps/bridge/src/components/bridge/bridge-card.tsx:374-378. |
| <code>capability.14.47.1</code> | title | Plan deposits from the live allowance with an exact approve on its own writer |
| <code>capability.14.47.1</code> | reqs | 14491.1<br>14491.2<br>14491.3 |
| <code>capability.14.47.3</code> | title | Add pure limit checks: vault outstanding on withdrawals, in-flight cap on deposits, gas reserve |
| <code>capability.14.47.3</code> | reqs | 14493.1<br>14493.2<br>14493.3<br>14493.4 |
| <code>capability.14.47.4</code> | title | Drive the bridge card's errors, blockers, Max and ready state from one pure card state |
| <code>capability.14.47.4</code> | reqs | 14494.1<br>14494.2 |

## verification

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-assertclaimmatches-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> (planned) | <code>export function assertClaimMatches(transfer: BridgeTransfer, attestation: DepositAttestation): void</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-assertreleasematches-j55</code> | <code>ts_export</code> | <code>new</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> (planned) | <code>export function assertReleaseMatches(transfer: BridgeTransfer, attestation: ReleaseAttestation): void</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-assertclaimmatches-j55</code> | notes | created by task 14.46.1; throws on any signed-field mismatch |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-assertreleasematches-j55</code> | notes | created by task 14.46.1; throws on any signed-field mismatch |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.484.1</code> | req | 14484.1 |
| <code>acceptance.14.484.1</code> | scenario | Given verifyThresholdSignatures at apps/bridge/src/lib/bridge/digests.ts:122, when it is called with threshold 0n, a digest and an empty signature list, then it returns ok false with a reason naming the threshold |
| <code>acceptance.14.484.1</code> | test | apps/bridge/src/lib/bridge/digests.spec.ts::verifyThresholdRefusesAZeroThreshold (planned) |
| <code>acceptance.14.484.1</code> | qualification | UNRUN |
| <code>req.14484</code> | ac_1 | verifyThresholdSignatures with threshold 0n returns ok false; at HEAD the comparison at apps/bridge/src/lib/bridge/digests.ts:128 passes with zero valid signatures. |
| <code>acceptance.14.484.2</code> | req | 14484.2 |
| <code>acceptance.14.484.2</code> | scenario | Given verifyThresholdSignatures, when it is called with attestors [] and threshold 1n, then it returns ok false with a reason naming the attestor set |
| <code>acceptance.14.484.2</code> | test | apps/bridge/src/lib/bridge/digests.spec.ts::verifyThresholdRefusesAnEmptyAttestorSet (planned) |
| <code>acceptance.14.484.2</code> | qualification | UNRUN |
| <code>req.14484</code> | ac_2 | verifyThresholdSignatures with an empty attestor list returns ok false whatever the threshold (apps/bridge/src/lib/bridge/digests.ts:122). |
| <code>acceptance.14.489.1</code> | req | 14489.1 |
| <code>acceptance.14.489.1</code> | scenario | Given a deposit transfer to recipient R and an attestation whose recipient32 encodes another address, when assertClaimMatches runs, then it throws naming the recipient |
| <code>acceptance.14.489.1</code> | test | apps/bridge/src/components/bridge/transfer-actions.spec.ts::assertClaimMatchesRefusesAnotherRecipient (planned) |
| <code>acceptance.14.489.1</code> | qualification | UNRUN |
| <code>req.14489</code> | ac_1 | assertClaimMatches throws when attestation.recipient32 differs from paxeerRecipient(transfer.recipient); the check at apps/bridge/src/components/bridge/transfer-actions.ts:106-112 compares only txHash, logIndex and amount. |
| <code>acceptance.14.489.2</code> | req | 14489.2 |
| <code>acceptance.14.489.2</code> | scenario | Given a deposit of asset X and an attestation for asset Y, when assertClaimMatches runs, then it throws naming the asset |
| <code>acceptance.14.489.2</code> | test | apps/bridge/src/components/bridge/transfer-actions.spec.ts::assertClaimMatchesRefusesAnotherAsset (planned) |
| <code>acceptance.14.489.2</code> | qualification | UNRUN |
| <code>req.14489</code> | ac_2 | assertClaimMatches throws when attestation.asset differs from transfer.assetId (apps/bridge/src/components/bridge/transfer-actions.ts:84). |
| <code>acceptance.14.489.3</code> | req | 14489.3 |
| <code>acceptance.14.489.3</code> | scenario | Given a deposit on chain 1 and an attestation for chain 10, when assertClaimMatches runs, then it throws naming the chain |
| <code>acceptance.14.489.3</code> | test | apps/bridge/src/components/bridge/transfer-actions.spec.ts::assertClaimMatchesRefusesAnotherChainId (planned) |
| <code>acceptance.14.489.3</code> | qualification | UNRUN |
| <code>req.14489</code> | ac_3 | assertClaimMatches throws when attestation.chainId differs from transfer.foreignChainId (apps/bridge/src/components/bridge/transfer-actions.ts:84). |
| <code>acceptance.14.489.4</code> | req | 14489.4 |
| <code>acceptance.14.489.4</code> | scenario | Given a withdrawal of asset X on chain 1 and a release attestation for asset Y, when assertReleaseMatches runs, then it throws naming the asset |
| <code>acceptance.14.489.4</code> | test | apps/bridge/src/components/bridge/transfer-actions.spec.ts::assertReleaseMatchesRefusesAnotherAsset (planned) |
| <code>acceptance.14.489.4</code> | qualification | UNRUN |
| <code>req.14489</code> | ac_4 | assertReleaseMatches throws when the release attestation asset or chainId differs from the withdrawal; the check at apps/bridge/src/components/bridge/transfer-actions.ts:177-187 omits both. |
| <code>capability.14.43.1</code> | title | Refuse a zero threshold and an empty attestor set in verifyThresholdSignatures |
| <code>capability.14.43.1</code> | reqs | 14484.1<br>14484.2 |
| <code>capability.14.46.1</code> | title | Check the attested recipient, asset and chain against the local record before a claim or release |
| <code>capability.14.46.1</code> | reqs | 14489.1<br>14489.2<br>14489.3<br>14489.4 |
