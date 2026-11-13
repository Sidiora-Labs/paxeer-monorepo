# Bridge client contract

Source paths are relative to the repository root. Declaration tables identify the interfaces in the source files; runtime behavior is defined by their implementations.

## amounts

| Requirement | Behavior |
| --- | --- |
| <code>req.14483</code> | parseAmount("1,234.5", 6) returns 1234500000n: commas are accepted only as thousands separators in groups of three before the decimal point (apps/bridge/src/lib/bridge/amount.ts). |

## app

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-app-layout-default</code> | <code>ts_export</code> | <code>apps/bridge/src/app/layout.tsx</code> | <code>export default function RootLayout</code> |
| <code>interface.14.ts-bridge-app-layout-metadata</code> | <code>ts_export</code> | <code>apps/bridge/src/app/layout.tsx</code> | <code>export const metadata: Metadata =</code> |
| <code>interface.14.ts-bridge-app-layout-viewport</code> | <code>ts_export</code> | <code>apps/bridge/src/app/layout.tsx</code> | <code>export const viewport: Viewport =</code> |

## attestation-api

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.fmt-bridge-attestation-bundle-fixture-j55</code> | <code>file_format</code> | <code>apps/bridge/src/lib/bridge/fixtures/deposit-ready.json</code> | <code>{chainId, vault, txHash, logIndex, recipient, asset, amount, digest, signatures[], status}</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationpollinterval-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export function attestationPollInterval</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-depositattestationpath-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export function depositAttestationPath</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-parsedepositattestation-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export function parseDepositAttestation</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-parsereleaseattestation-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export function parseReleaseAttestation</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-releaseattestationpath-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export function releaseAttestationPath</code> |

| Field | Rule |
|---|---|
| interface | apps/bridge client: deposits call PaxeerXVault.deposit and depositNative on the foreign chain, withdrawals call bridgeOut on 0x1016, claims and releases are built from attestation bundles fetched from /v1/deposit and /v1/release and verified locally before the wallet opens. |
| storage | Browser localStorage key paxeer-bridge:transfers:v1 holding BridgeTransfer rows (apps/bridge/src/lib/bridge/transfers.ts); no server state. |
| errors | LXP_BRIDGE_UNAUTHORIZED, LXP_BRIDGE_UNKNOWN_CHAIN, LXP_BRIDGE_CHAIN_DISABLED, LXP_BRIDGE_UNKNOWN_ASSET, LXP_BRIDGE_PAUSED, LXP_BRIDGE_NULLIFIED, LXP_BRIDGE_CAP_EXCEEDED, LXP_BRIDGE_BAD_SIGNATURE, LXP_BRIDGE_BELOW_THRESHOLD, LXP_BRIDGE_VAULT_MISMATCH, LXP_BRIDGE_INVALID_REQUEST mapped to sentences; vault custom errors decoded with paxeerXVaultAbi. |
| events | Reads BridgeDeposit and BridgeRelease of PaxeerXVault and BridgeOut of 0x1016 from receipts. |
| gas | None set by the client; the wallet estimates gas. The Max amount keeps a native gas reserve. |
| activation | Routes open per chain when NEXT_PUBLIC_BRIDGE_VAULT_&lt;CHAIN&gt; is set and the chain is registered and enabled on 0x1016; claim and release need NEXT_PUBLIC_BRIDGE_API_URL. |
| invariants | The client never submits a claim or release whose digest it has not recomputed and whose signatures do not meet the threshold; the attested recipient, asset and chain equal the local record. |
| pause | A paused precompile or vault disables submission and shows the paused blocker. |

| Requirement | Behavior |
| --- | --- |
| <code>req.14485</code> | parseDepositAttestation throws when body.txHash or body.logIndex differs from the request (apps/bridge/src/lib/bridge/attestation-api.ts). |
| <code>req.14486</code> | attestationPollInterval returns false for status ready with at least one signature (apps/bridge/src/lib/bridge/attestation-api.ts). |
| <code>req.14487</code> | depositAttestationPath({chainId, txHash, logIndex}) returns /v1/deposit/<chainId>/<lowercase txHash>/<logIndex>, the path built inline at apps/bridge/src/lib/bridge/attestation-api.ts. |
| <code>req.14487</code> | The committed fixtures deposit-ready.json and release-ready.json parse through the deposit and release parsers of apps/bridge/src/lib/bridge/attestation-api.ts into status ready bundles with at least one signature. |

## components

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-components-bridge-bridge-card-bridgecard</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/bridge-card.tsx</code> | <code>export function BridgeCard({ onTransferSubmitted }: { onTransferSubmitted: () =&gt; void })</code> |
| <code>interface.14.ts-bridge-components-bridge-how-it-works-howitworks</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/how-it-works.tsx</code> | <code>export function HowItWorks</code> |
| <code>interface.14.ts-bridge-components-bridge-providers-providers</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/providers.tsx</code> | <code>export function Providers({ children }: { children: ReactNode })</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-amountinput</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx</code> | <code>export function AmountInput({ id, value, onChange, symbol, balance, decimals, balanceLoading, invalid, disabled, onMax, }:</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-assetpicker</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx</code> | <code>export function AssetPicker({ symbols, value, onChange, label, }: { symbols: string[]; value: string; onChange: (symbol: string) =&gt; void; label: string; })</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-receivepreview</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx</code> | <code>export function ReceivePreview({ symbol, amountText, denom, }: { symbol: string; amountText?: string; denom?: string; })</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-routeoption</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx</code> | <code>export type RouteOption =</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-routepanel</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx</code> | <code>export function RoutePanel({ label, option, options, onSelect, pickerTitle, children, }: { label: string; option: RouteOption; options: RouteOption[]; onSelect: (key: string) =&gt; void;</code> |
| <code>interface.14.ts-bridge-components-bridge-route-panels-swapdirectionbutton</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/route-panels.tsx</code> | <code>export function SwapDirectionButton({ onClick, disabled }: { onClick: () =&gt; void; disabled?: boolean })</code> |
| <code>interface.14.ts-bridge-components-bridge-site-header-siteheader</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/site-header.tsx</code> | <code>export function SiteHeader({ historyBadge }: { historyBadge?: number })</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-bridgeoutnoncefromreceipt</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export function bridgeOutNonceFromReceipt</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-buildverifiedclaim</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export async function buildVerifiedClaim( transfer: BridgeTransfer, attestation: DepositAttestation, ): Promise&lt;VerifiedClaim&gt;</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-buildverifiedrelease</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export async function buildVerifiedRelease( transfer: BridgeTransfer, attestation: ReleaseAttestation, ): Promise&lt;VerifiedRelease&gt;</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-readtransferlive</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export async function readTransferLive</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-transferlive</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export type TransferLive =</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-transferphase</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export type TransferPhase =</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-verifiedclaim</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export type VerifiedClaim =</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-verifiedrelease</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export type VerifiedRelease =</code> |
| <code>interface.14.ts-bridge-components-bridge-transfers-panel-transferspanel</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfers-panel.tsx</code> | <code>export function TransfersPanel({ refreshKey }: { refreshKey: number })</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-createthemetransition</code> | <code>ts_export</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx</code> | <code>function createThemeTransition({ variant = "circle", start = "pointer", blur = false, duration, gifUrl }: ThemeTransitionOptions, origin = "50% 50%")</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetoggle</code> | <code>ts_export</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx</code> | <code>function ThemeToggle({ icon = "sun", appearance = "ghost", variant = "circle", start = "pointer", blur, duration, gifUrl, label = "Toggle theme", className, onClick, ...props</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetoggleicon</code> | <code>ts_export</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx</code> | <code>type ThemeToggleIcon = keyof typeof icons;</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetoggleprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx</code> | <code>type ThemeToggleProps = Omit&lt;React.ComponentProps&lt;"button"&gt;, "children"&gt; &amp;</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionoptions</code> | <code>ts_export</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx</code> | <code>type ThemeTransitionOptions =</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionstart</code> | <code>ts_export</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx</code> | <code>type ThemeTransitionStart =</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-themetransitionvariant</code> | <code>ts_export</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx</code> | <code>type ThemeTransitionVariant = "none" &#124; "circle" &#124; "circle-blur" &#124; "rectangle" &#124; "polygon" &#124; "gif";</code> |
| <code>interface.14.ts-bridge-components-dx-theme-toggle-usethemetransition</code> | <code>ts_export</code> | <code>apps/bridge/src/components/dx/theme-toggle.tsx</code> | <code>function useThemeTransition(options: ThemeTransitionOptions = {})</code> |
| <code>interface.14.ts-bridge-components-theme-provider-themeprovider</code> | <code>ts_export</code> | <code>apps/bridge/src/components/theme-provider.tsx</code> | <code>export function ThemeProvider({ children }: { children: React.ReactNode })</code> |
| <code>interface.14.ts-bridge-components-theme-provider-usetheme</code> | <code>ts_export</code> | <code>apps/bridge/src/components/theme-provider.tsx</code> | <code>export function useTheme()</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainlist</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx</code> | <code>export function ChainList({ chains, value, onSelect, recent = [], tab, defaultTab, onTabChange, wrongNetwork, variant = "wide", className }: ChainListProps)</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainlistprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx</code> | <code>export interface ChainListProps</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainoption</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx</code> | <code>export type ChainOption =</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainselector</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx</code> | <code>export function ChainSelector({ chains, value, defaultValue, onValueChange, open, defaultOpen = false, onOpenChange, wrongNetwork, loading, layout = "popover", showName = false,</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chainselectorprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx</code> | <code>export interface ChainSelectorProps extends Omit&lt;ChainListProps, "onSelect" &#124; "value" &#124; "variant" &#124; "className"&gt;</code> |
| <code>interface.14.ts-bridge-components-trading-chain-selector-chaintab</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/chain-selector.tsx</code> | <code>export type ChainTab = "mainnet" &#124; "testnet";</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-depositstatus</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx</code> | <code>export function DepositStatus({ items, open, defaultOpen = false, onOpenChange, onDismiss, explorerUrl, footer, className }: DepositStatusProps)</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-depositstatusprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx</code> | <code>export interface DepositStatusProps</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstatusgroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx</code> | <code>export type TransferStatusGroup = 'pending' &#124; 'failed' &#124; 'completed';</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstatusitem</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx</code> | <code>export type TransferStatusItem =</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transferstep</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx</code> | <code>export type TransferStep = 'submitted' &#124; 'confirming' &#124; 'attesting' &#124; 'ready' &#124; 'credited' &#124; 'failed';</code> |
| <code>interface.14.ts-bridge-components-trading-deposit-status-transfersteps</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/deposit-status.tsx</code> | <code>export function TransferSteps({ item }: { item: TransferStatusItem })</code> |
| <code>interface.14.ts-bridge-components-trading-numeral-numeral</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/numeral.tsx</code> | <code>export function Numeral</code> |
| <code>interface.14.ts-bridge-components-trading-numeral-numeralprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/numeral.tsx</code> | <code>export interface NumeralProps</code> |
| <code>interface.14.ts-bridge-components-trading-numeral-statistic</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/numeral.tsx</code> | <code>export function Statistic({ label, children, align = "start", className }: StatisticProps)</code> |
| <code>interface.14.ts-bridge-components-trading-numeral-statisticprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/numeral.tsx</code> | <code>export interface StatisticProps</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-chainicon</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/token-icon.tsx</code> | <code>export function ChainIcon({ name, ...props }: ChainIconProps)</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-chainiconprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/token-icon.tsx</code> | <code>export interface ChainIconProps extends Omit&lt;TokenIconProps, "symbol" &#124; "shape"&gt;</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-symbolbadge</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/token-icon.tsx</code> | <code>export function SymbolBadge({ children, className }: { children: ReactNode; className?: string })</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-symboldisplay</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/token-icon.tsx</code> | <code>export function SymbolDisplay({ symbol, format = "base-type", showIcon = true, iconSrc, size = "xs", badge, className }: SymbolDisplayProps)</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-symboldisplayprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/token-icon.tsx</code> | <code>export interface SymbolDisplayProps</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-tokenicon</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/token-icon.tsx</code> | <code>export function TokenIcon({ symbol, src, size = "sm", shape = "circle", className }: TokenIconProps)</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-tokeniconprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/token-icon.tsx</code> | <code>export interface TokenIconProps</code> |
| <code>interface.14.ts-bridge-components-trading-token-icon-tokeniconsize</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/token-icon.tsx</code> | <code>export type TokenIconSize = keyof typeof SIZES;</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferhistory</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx</code> | <code>export function TransferHistory({ records, filters = ["all", "deposit", "withdraw", "transfer"], filter, defaultFilter = "all", onFilterChange, filterLocally = true, explorerUrl, onCopy, formatTime = defaultFormatTime, loading, emptyText = "No transfers yet.",</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferhistoryfilter</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx</code> | <code>export type TransferHistoryFilter = "all" &#124; TransferRecordType;</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferhistoryprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx</code> | <code>export interface TransferHistoryProps</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferrecord</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx</code> | <code>export type TransferRecord =</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferrecordstatus</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx</code> | <code>export type TransferRecordStatus = "pending" &#124; "processing" &#124; "completed" &#124; "failed" &#124; "cancelled";</code> |
| <code>interface.14.ts-bridge-components-trading-transfer-history-transferrecordtype</code> | <code>ts_export</code> | <code>apps/bridge/src/components/trading/transfer-history.tsx</code> | <code>export type TransferRecordType = "deposit" &#124; "withdraw" &#124; "transfer";</code> |
| <code>interface.14.ts-bridge-components-ui-alert-alert</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/alert.tsx</code> | <code>function Alert({ className, variant, ...props }: React.ComponentProps&lt;"div"&gt; &amp; VariantProps&lt;typeof alertVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-alert-alertdescription</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/alert.tsx</code> | <code>function AlertDescription({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-alert-alerttitle</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/alert.tsx</code> | <code>function AlertTitle({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-avatar-avatar</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/avatar.tsx</code> | <code>function Avatar({ className, ...props }: AvatarPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-avatar-avatarfallback</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/avatar.tsx</code> | <code>function AvatarFallback({ className, ...props }: AvatarPrimitive.Fallback.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-avatar-avatargroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/avatar.tsx</code> | <code>function AvatarGroup({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-avatar-avatarimage</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/avatar.tsx</code> | <code>function AvatarImage({ className, ...props }: AvatarPrimitive.Image.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-badge-badge</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/badge.tsx</code> | <code>function Badge({ className, variant, render, ...props }: BadgeProps)</code> |
| <code>interface.14.ts-bridge-components-ui-badge-badgevariants</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/badge.tsx</code> | <code>const badgeVariants = cva(</code> |
| <code>interface.14.ts-bridge-components-ui-button-button</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/button.tsx</code> | <code>function Button({ className, variant, size, render, nativeButton, ...props }: ButtonProps)</code> |
| <code>interface.14.ts-bridge-components-ui-button-buttonprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/button.tsx</code> | <code>type ButtonProps = ButtonPrimitive.Props &amp; VariantProps&lt;typeof buttonVariants&gt;;</code> |
| <code>interface.14.ts-bridge-components-ui-button-buttonvariants</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/button.tsx</code> | <code>const buttonVariants = cva(</code> |
| <code>interface.14.ts-bridge-components-ui-card-card</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/card.tsx</code> | <code>function Card({ className, variant = "outline", ...props }: CardProps)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardaction</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/card.tsx</code> | <code>function CardAction({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardcontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/card.tsx</code> | <code>function CardContent({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-carddescription</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/card.tsx</code> | <code>function CardDescription({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardfooter</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/card.tsx</code> | <code>function CardFooter({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardheader</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/card.tsx</code> | <code>function CardHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-card-cardtitle</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/card.tsx</code> | <code>function CardTitle({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsible</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/collapsible.tsx</code> | <code>function Collapsible(props: CollapsiblePrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsiblecontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/collapsible.tsx</code> | <code>function CollapsibleContent({ className, ...props }: CollapsiblePrimitive.Panel.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-collapsible-collapsibletrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/collapsible.tsx</code> | <code>function CollapsibleTrigger(props: CollapsiblePrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialog</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function Dialog(props: DialogPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogclose</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogClose(props: DialogPrimitive.Close.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogcontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogContent({ className, children, showCloseButton = true, ...props }: DialogPrimitive.Popup.Props &amp; { showCloseButton?: boolean })</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogdescription</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogDescription({ className, ...props }: DialogPrimitive.Description.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogfooter</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogFooter({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogheader</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogoverlay</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogOverlay({ className, ...props }: DialogPrimitive.Backdrop.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogportal</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogPortal(props: DialogPrimitive.Portal.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogtitle</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogTitle({ className, ...props }: DialogPrimitive.Title.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dialog-dialogtrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dialog.tsx</code> | <code>function DialogTrigger(props: DialogPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenu</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenu(props: MenuPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenucheckboxitem</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuCheckboxItem({ className, children, ...props }: MenuPrimitive.CheckboxItem.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenucontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuContent({ className, align = "start", side = "bottom", sideOffset = 6, alignOffset = 0, ...props }: MenuPrimitive.Popup.Props &amp; Pick&lt;MenuPrimitive.Positioner.Props, "align" &#124; "side" &#124; "sideOffset" &#124; "alignOffset"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenugroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuGroup(props: MenuPrimitive.Group.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuitem</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuItem({ className, inset, variant = "default", ...props }: MenuPrimitive.Item.Props &amp; { inset?: boolean; variant?: "default" &#124; "destructive" })</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenulabel</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuLabel({ className, inset, ...props }: MenuPrimitive.GroupLabel.Props &amp; { inset?: boolean })</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuradiogroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuRadioGroup(props: MenuPrimitive.RadioGroup.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuradioitem</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuRadioItem({ className, children, ...props }: MenuPrimitive.RadioItem.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenuseparator</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuSeparator({ className, ...props }: MenuPrimitive.Separator.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenushortcut</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuShortcut({ className, ...props }: React.ComponentProps&lt;"span"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusub</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuSub(props: MenuPrimitive.SubmenuRoot.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusubcontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuSubContent({ className, sideOffset = 4, alignOffset = -6, ...props }: MenuPrimitive.Popup.Props &amp; Pick&lt;MenuPrimitive.Positioner.Props, "sideOffset" &#124; "alignOffset"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenusubtrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuSubTrigger({ className, inset, children, ...props }: MenuPrimitive.SubmenuTrigger.Props &amp; { inset?: boolean })</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-dropdownmenutrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>function DropdownMenuTrigger(props: MenuPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-menuitem</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>const menuItem =</code> |
| <code>interface.14.ts-bridge-components-ui-dropdown-menu-menupopup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/dropdown-menu.tsx</code> | <code>const menuPopup =</code> |
| <code>interface.14.ts-bridge-components-ui-empty-empty</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/empty.tsx</code> | <code>function Empty({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptycontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/empty.tsx</code> | <code>function EmptyContent({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptydescription</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/empty.tsx</code> | <code>function EmptyDescription({ className, ...props }: React.ComponentProps&lt;"p"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptyheader</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/empty.tsx</code> | <code>function EmptyHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptymedia</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/empty.tsx</code> | <code>function EmptyMedia({ className, variant = "icon", ...props }: React.ComponentProps&lt;"div"&gt; &amp; { variant?: "default" &#124; "icon" })</code> |
| <code>interface.14.ts-bridge-components-ui-empty-emptytitle</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/empty.tsx</code> | <code>function EmptyTitle({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-field-field</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function Field({ className, orientation, ...props }: FieldPrimitive.Root.Props &amp; VariantProps&lt;typeof fieldVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldcontrol</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function FieldControl({ className, ...props }: FieldPrimitive.Control.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fielddescription</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function FieldDescription({ className, ...props }: FieldPrimitive.Description.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fielderror</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function FieldError({ className, ...props }: FieldPrimitive.Error.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldgroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function FieldGroup({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldlabel</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function FieldLabel({ className, ...props }: FieldPrimitive.Label.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldlegend</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function FieldLegend({ className, ...props }: FieldsetPrimitive.Legend.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldseparator</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function FieldSeparator({ className, children, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-field-fieldset</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/field.tsx</code> | <code>function FieldSet({ className, ...props }: FieldsetPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercard</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/hover-card.tsx</code> | <code>function HoverCard(props: PreviewCardPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercardcontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/hover-card.tsx</code> | <code>function HoverCardContent({ className, align = "center", side = "bottom", sideOffset = 8, ...props }: PreviewCardPrimitive.Popup.Props &amp; Pick&lt;PreviewCardPrimitive.Positioner.Props, "align" &#124; "side" &#124; "sideOffset"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-hover-card-hovercardtrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/hover-card.tsx</code> | <code>function HoverCardTrigger(props: PreviewCardPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/input-group.tsx</code> | <code>function InputGroup({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgroupaddon</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/input-group.tsx</code> | <code>function InputGroupAddon({ className, align = "inline-start", ...props }: React.ComponentProps&lt;"div"&gt; &amp; { align?: "inline-start" &#124; "inline-end" &#124; "block-start" &#124; "block-end" })</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgroupinput</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/input-group.tsx</code> | <code>function InputGroupInput({ className, ...props }: React.ComponentProps&lt;"input"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgrouptext</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/input-group.tsx</code> | <code>function InputGroupText({ className, ...props }: React.ComponentProps&lt;"span"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-input-group-inputgrouptextarea</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/input-group.tsx</code> | <code>function InputGroupTextarea({ className, ...props }: React.ComponentProps&lt;"textarea"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-input-input</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/input.tsx</code> | <code>function Input({ className, type, ...props }: InputPrimitive.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-kbd-kbd</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/kbd.tsx</code> | <code>function Kbd({ className, ...props }: React.ComponentProps&lt;"kbd"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-kbd-kbdgroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/kbd.tsx</code> | <code>function KbdGroup({ className, ...props }: React.ComponentProps&lt;"kbd"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-label-label</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/label.tsx</code> | <code>function Label({ className, ...props }: React.ComponentProps&lt;"label"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popover</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/popover.tsx</code> | <code>function Popover(props: PopoverPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popoverclose</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/popover.tsx</code> | <code>const PopoverClose = PopoverPrimitive.Close;</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popovercontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/popover.tsx</code> | <code>function PopoverContent({ className, align = "center", side = "bottom", sideOffset = 8, ...props }: PopoverPrimitive.Popup.Props &amp; Pick&lt;PopoverPrimitive.Positioner.Props, "align" &#124; "side" &#124; "sideOffset" &#124; "alignOffset"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popoverdescription</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/popover.tsx</code> | <code>function PopoverDescription({ className, ...props }: PopoverPrimitive.Description.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popoverheader</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/popover.tsx</code> | <code>function PopoverHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popovertitle</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/popover.tsx</code> | <code>function PopoverTitle({ className, ...props }: PopoverPrimitive.Title.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-popover-popovertrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/popover.tsx</code> | <code>function PopoverTrigger(props: PopoverPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-progress-progress</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/progress.tsx</code> | <code>function Progress({ className, children, ...props }: ProgressPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-progress-progresslabel</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/progress.tsx</code> | <code>function ProgressLabel({ className, ...props }: ProgressPrimitive.Label.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-progress-progressvalue</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/progress.tsx</code> | <code>function ProgressValue({ className, ...props }: ProgressPrimitive.Value.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-scroll-area-scrollarea</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/scroll-area.tsx</code> | <code>function ScrollArea({ className, children, fade, ...props }: ScrollAreaProps)</code> |
| <code>interface.14.ts-bridge-components-ui-scroll-area-scrollbar</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/scroll-area.tsx</code> | <code>function ScrollBar({ className, orientation = "vertical", ...props }: ScrollAreaPrimitive.Scrollbar.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-select</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/select.tsx</code> | <code>const Select = SelectPrimitive.Root;</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectcontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/select.tsx</code> | <code>function SelectContent({ className, children, side = "bottom", sideOffset = 6, align = "center", alignItemWithTrigger = true, ...props }: SelectPrimitive.Popup.Props &amp; Pick&lt;SelectPrimitive.Positioner.Props, "align" &#124; "side" &#124; "sideOffset" &#124; "alignItemWithTrigger"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectgroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/select.tsx</code> | <code>function SelectGroup({ className, ...props }: SelectPrimitive.Group.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectitem</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/select.tsx</code> | <code>function SelectItem({ className, children, ...props }: SelectPrimitive.Item.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectlabel</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/select.tsx</code> | <code>function SelectLabel({ className, ...props }: SelectPrimitive.GroupLabel.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectseparator</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/select.tsx</code> | <code>function SelectSeparator({ className, ...props }: SelectPrimitive.Separator.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-select-selecttrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/select.tsx</code> | <code>function SelectTrigger({ className, size = "default", children, ...props }: SelectPrimitive.Trigger.Props &amp; { size?: "sm" &#124; "default" })</code> |
| <code>interface.14.ts-bridge-components-ui-select-selectvalue</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/select.tsx</code> | <code>function SelectValue({ className, ...props }: SelectPrimitive.Value.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-separator-separator</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/separator.tsx</code> | <code>function Separator({ className, orientation = "horizontal", ...props }: SeparatorPrimitive.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheet</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sheet.tsx</code> | <code>function Sheet(props: SheetPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetclose</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sheet.tsx</code> | <code>function SheetClose(props: SheetPrimitive.Close.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetcontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sheet.tsx</code> | <code>function SheetContent({ className, children, side = "right", showCloseButton = true, ...props }: SheetPrimitive.Popup.Props &amp; { side?: keyof typeof sideClasses; showCloseButton?: boolean })</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetdescription</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sheet.tsx</code> | <code>function SheetDescription({ className, ...props }: SheetPrimitive.Description.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetfooter</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sheet.tsx</code> | <code>function SheetFooter({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheetheader</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sheet.tsx</code> | <code>function SheetHeader({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheettitle</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sheet.tsx</code> | <code>function SheetTitle({ className, ...props }: SheetPrimitive.Title.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-sheet-sheettrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sheet.tsx</code> | <code>function SheetTrigger(props: SheetPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-skeleton-skeleton</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/skeleton.tsx</code> | <code>function Skeleton({ className, ...props }: React.ComponentProps&lt;"div"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-sonner-toast</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sonner.tsx</code> | <code>export { Toaster, toast };</code> |
| <code>interface.14.ts-bridge-components-ui-sonner-toaster</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/sonner.tsx</code> | <code>function Toaster(props: ToasterProps)</code> |
| <code>interface.14.ts-bridge-components-ui-spinner-spinner</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/spinner.tsx</code> | <code>function Spinner({ className, ...props }: React.ComponentProps&lt;"output"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-table</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/table.tsx</code> | <code>function Table({ className, ...props }: React.ComponentProps&lt;"table"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablebody</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/table.tsx</code> | <code>function TableBody({ className, ...props }: React.ComponentProps&lt;"tbody"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablecaption</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/table.tsx</code> | <code>function TableCaption({ className, ...props }: React.ComponentProps&lt;"caption"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablecell</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/table.tsx</code> | <code>function TableCell({ className, ...props }: React.ComponentProps&lt;"td"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablefooter</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/table.tsx</code> | <code>function TableFooter({ className, ...props }: React.ComponentProps&lt;"tfoot"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablehead</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/table.tsx</code> | <code>function TableHead({ className, ...props }: React.ComponentProps&lt;"th"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tableheader</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/table.tsx</code> | <code>function TableHeader({ className, ...props }: React.ComponentProps&lt;"thead"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-table-tablerow</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/table.tsx</code> | <code>function TableRow({ className, ...props }: React.ComponentProps&lt;"tr"&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabs</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tabs.tsx</code> | <code>function Tabs({ className, ...props }: TabsPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabscontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tabs.tsx</code> | <code>function TabsContent({ className, ...props }: TabsPrimitive.Panel.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabslist</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tabs.tsx</code> | <code>function TabsList({ className, variant = "default", children, ...props }: TabsPrimitive.List.Props &amp; VariantProps&lt;typeof tabsListVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabslistvariants</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tabs.tsx</code> | <code>const tabsListVariants = cva("relative z-0 inline-flex w-fit items-center data-[orientation=vertical]:flex-col",</code> |
| <code>interface.14.ts-bridge-components-ui-tabs-tabstrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tabs.tsx</code> | <code>function TabsTrigger({ className, ...props }: TabsPrimitive.Tab.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-toggle-group-togglegroup</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/toggle-group.tsx</code> | <code>function ToggleGroup({ className, variant, size, children, ...props }: ToggleGroupPrimitive.Props &amp; VariantProps&lt;typeof toggleVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-toggle-group-togglegroupitem</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/toggle-group.tsx</code> | <code>function ToggleGroupItem({ className, variant: _variant, size, ...props }: TogglePrimitive.Props &amp; VariantProps&lt;typeof toggleVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-toggle-toggle</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/toggle.tsx</code> | <code>function Toggle({ className, variant, size, ...props }: TogglePrimitive.Props &amp; VariantProps&lt;typeof toggleVariants&gt;)</code> |
| <code>interface.14.ts-bridge-components-ui-toggle-togglevariants</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/toggle.tsx</code> | <code>const toggleVariants = cva(</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltip</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tooltip.tsx</code> | <code>function Tooltip(props: TooltipPrimitive.Root.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipcontent</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tooltip.tsx</code> | <code>function TooltipContent({ className, side = "top", sideOffset, align = "center", variant = "inverted", arrow = false, children, ...props }: TooltipPrimitive.Popup.Props &amp; Pick&lt;TooltipPrimitive.Positioner.Props, "side" &#124; "sideOffset" &#124; "align"&gt; &amp; VariantProps&lt;typeof tooltipVariants&gt; &amp;</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipcontentprops</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tooltip.tsx</code> | <code>export type TooltipContentProps = React.ComponentProps&lt;typeof TooltipContent&gt;;</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipprovider</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tooltip.tsx</code> | <code>function TooltipProvider({ delay = 300, ...props }: TooltipPrimitive.Provider.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltiptrigger</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tooltip.tsx</code> | <code>function TooltipTrigger(props: TooltipPrimitive.Trigger.Props)</code> |
| <code>interface.14.ts-bridge-components-ui-tooltip-tooltipvariants</code> | <code>ts_export</code> | <code>apps/bridge/src/components/ui/tooltip.tsx</code> | <code>const tooltipVariants = cva(</code> |

| Field | Rule |
|---|---|
| interface | apps/bridge client: deposits call PaxeerXVault.deposit and depositNative on the foreign chain, withdrawals call bridgeOut on 0x1016, claims and releases are built from attestation bundles fetched from /v1/deposit and /v1/release and verified locally before the wallet opens. |
| storage | Browser localStorage key paxeer-bridge:transfers:v1 holding BridgeTransfer rows (apps/bridge/src/lib/bridge/transfers.ts); no server state. |
| errors | LXP_BRIDGE_UNAUTHORIZED, LXP_BRIDGE_UNKNOWN_CHAIN, LXP_BRIDGE_CHAIN_DISABLED, LXP_BRIDGE_UNKNOWN_ASSET, LXP_BRIDGE_PAUSED, LXP_BRIDGE_NULLIFIED, LXP_BRIDGE_CAP_EXCEEDED, LXP_BRIDGE_BAD_SIGNATURE, LXP_BRIDGE_BELOW_THRESHOLD, LXP_BRIDGE_VAULT_MISMATCH, LXP_BRIDGE_INVALID_REQUEST mapped to sentences; vault custom errors decoded with paxeerXVaultAbi. |
| events | Reads BridgeDeposit and BridgeRelease of PaxeerXVault and BridgeOut of 0x1016 from receipts. |
| gas | None set by the client; the wallet estimates gas. The Max amount keeps a native gas reserve. |
| activation | Routes open per chain when NEXT_PUBLIC_BRIDGE_VAULT_&lt;CHAIN&gt; is set and the chain is registered and enabled on 0x1016; claim and release need NEXT_PUBLIC_BRIDGE_API_URL. |
| invariants | The client never submits a claim or release whose digest it has not recomputed and whose signatures do not meet the threshold; the attested recipient, asset and chain equal the local record. |
| pause | A paused precompile or vault disables submission and shows the paused blocker. |

## config

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.env-bridge-next-public-bridge-rpc-solana-j55</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_SOLANA</code> |
| <code>interface.14.env-bridge-next-public-bridge-solana-program-id-j55</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-buildbridgechains-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export function buildBridgeChains</code> |
| <code>interface.14.ts-bridge-lib-bridge-config-readconfig-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>export function readConfig</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeeraddressurlfrom-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> | <code>export function paxeerAddressUrlFrom(base: string &#124; undefined, address: Address): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeertxurlfrom-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> | <code>export function paxeerTxUrlFrom(base: string &#124; undefined, hash: Hex): string &#124; undefined</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.14500</code> | readConfig leaves solanaProgramId undefined for a value that does not decode to 32 bytes (apps/bridge/src/lib/bridge/solana.ts). |
| <code>req.14501</code> | buildBridgeChains marks solana awaiting-backend when solanaProgramId is unset (apps/bridge/src/lib/bridge/chains.ts). |
| <code>req.14518</code> | paxeerAddressUrl returns <explorer>/address/<address> with trailing slashes trimmed when an explorer is configured (apps/bridge/src/lib/bridge/explorer.ts). |

## conformance

| Requirement | Behavior |
| --- | --- |
| <code>req.14512</code> | solanaRecipientHandle("So11111111111111111111111111111111111111112") (services/bridge/deploy/chainconfig/config.go) equals, ignoring case, the SOL asset_id at contracts/bridge/solana/chains/solana/config.json and the sol assetId in apps/bridge/src/lib/bridge/chains.ts. |
| <code>req.14513</code> | Every function, event and error of paxeerXVaultAbi (apps/bridge/src/lib/bridge/contracts.ts) has a declaration with the same name and parameter types in contracts/bridge/evm/src/PaxeerXVault.sol:40-213. |
| <code>req.14514</code> | For each EVM entry of BRIDGE_CHAINS (apps/bridge/src/lib/bridge/chains.ts) chainId and finalityDepth equal chain_id and finality_depth of contracts/bridge/evm/chains/<key>/config.json (for example contracts/bridge/evm/chains/base/config.json and contracts/bridge/evm/chains/base/config.json). |
| <code>req.14514</code> | The solana entry has chainId 91600046870081 and finalityDepth 32 as contracts/bridge/solana/chains/solana/config.json and contracts/bridge/solana/chains/solana/config.json state (apps/bridge/src/lib/bridge/chains.ts). |
| <code>req.14515</code> | Every LXP_BRIDGE_ code listed in revertCodes at precompiles/layerxbridge/layerxbridge.go is a key of PRECOMPILE_CODES in apps/bridge/src/lib/bridge/errors.ts, and PRECOMPILE_CODES has no other key. |
| <code>req.14516</code> | toEventSelector of BridgeDeposit and BridgeRelease in paxeerXVaultAbi (apps/bridge/src/lib/bridge/contracts.ts) equal BRIDGE_DEPOSIT_TOPIC at contracts/bridge/evm/test/RelayerAbiParity.t.sol:14 and BRIDGE_RELEASE_TOPIC at contracts/bridge/evm/test/RelayerAbiParity.t.sol:30. |

## env

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.env-bridge-next-public-bridge-api-url</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_API_URL</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-arbitrum</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-avalanche</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-base</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_BASE</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-bnb</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_BNB</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-ethereum</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-hyperevm</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-optimism</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM</code> |
| <code>interface.14.env-bridge-next-public-bridge-rpc-polygon</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_RPC_POLYGON</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-arbitrum</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-avalanche</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-base</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_BASE</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-bnb</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_BNB</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-ethereum</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-hyperevm</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-optimism</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM</code> |
| <code>interface.14.env-bridge-next-public-bridge-vault-polygon</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_BRIDGE_VAULT_POLYGON</code> |
| <code>interface.14.env-bridge-next-public-paxeer-explorer-url</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_PAXEER_EXPLORER_URL</code> |
| <code>interface.14.env-bridge-next-public-paxeer-rpc-url</code> | <code>env_var</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>NEXT_PUBLIC_PAXEER_RPC_URL</code> |

## errors

| Requirement | Behavior |
| --- | --- |
| <code>req.14499</code> | describeWriteError for LXP_BRIDGE_NULLIFIED returns the already-credited sentence (precompiles/layerxbridge/layerxbridge.go). |
| <code>req.14499</code> | describeWriteError for a reason without a registered LXP_BRIDGE_ code returns the reason text with a trailing period, as defined in apps/bridge/src/lib/bridge/errors.ts. |

## harness

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.cli-bridge-pnpm-test-j55</code> | <code>cli_command</code> | <code>apps/bridge/package.json</code> | <code>pnpm --dir apps/bridge test</code> |
| <code>interface.14.cli-bridge-pnpm-typecheck-j55</code> | <code>cli_command</code> | <code>apps/bridge/package.json</code> | <code>pnpm --dir apps/bridge typecheck</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.14481</code> | An import of @/lib/bridge/amount from a test under apps/bridge/src resolves to apps/bridge/src/lib/bridge/amount.ts because apps/bridge/tsconfig.json maps @/* to ./src/* with baseUrl set to the app directory. |
| <code>req.14481</code> | vitest run in apps/bridge uses the node environment, vite-tsconfig-paths and vite-plugin-svgr, with **/*.spec.ts and **/*.spec.tsx inputs (apps/bridge/vitest.config.ts). |
| <code>req.14482</code> | Every bare module specifier imported under apps/bridge/src, including motion, sonner, tailwindcss, @tailwindcss/turbopack (apps/bridge/next.config.ts) and @noble/curves, is a key of dependencies or devDependencies in apps/bridge/package.json. |
| <code>req.14482</code> | apps/bridge/package.json declares runtime and development dependencies for the app, configuration and test tooling. |
| <code>req.14482</code> | apps/bridge/package.json defines test = "vitest run" and typecheck = "next typegen &amp;&amp; tsc --noEmit -p tsconfig.json". |

## ledger

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-components-bridge-import-transfer-importtransfer-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/import-transfer.tsx</code> | <code>export function ImportTransfer({ onImported }: { onImported: () =&gt; void })</code> |
| <code>interface.14.ts-bridge-lib-bridge-enrich-enrichrecord-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/enrich.ts</code> | <code>export function enrichRecord(transfer: BridgeTransfer, receipt: TransactionReceipt): BridgeTransfer</code> |
| <code>interface.14.ts-bridge-lib-bridge-import-importdeposit-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/import.ts</code> | <code>export function importDeposit</code> |
| <code>interface.14.ts-bridge-lib-bridge-import-importwithdrawal-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/import.ts</code> | <code>export function importWithdrawal</code> |
| <code>interface.14.ts-bridge-lib-bridge-submitted-depositrecord-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/submitted.ts</code> | <code>export function depositRecord(input: DepositRecordInput): BridgeTransfer</code> |
| <code>interface.14.ts-bridge-lib-bridge-submitted-withdrawalrecord-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/submitted.ts</code> | <code>export function withdrawalRecord(input: WithdrawalRecordInput): BridgeTransfer</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-transferidof-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/transfers.ts</code> | <code>export function transferIdOf</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.14488</code> | transferIdOf(direction, sourceTxHash) returns `${lowercased hash}:${direction}`. With chainId and an optional eventId it returns `${chainId}:${lowercased hash}:${direction}:${eventId}`, using pending for an absent eventId (apps/bridge/src/lib/bridge/transfers.ts). |
| <code>req.14492</code> | withdrawalRecord(input) stores the destination in recipient, the foreign chain id as a decimal string and no vault. For Solana it also validates the destination and stores its recipientHandle (apps/bridge/src/lib/bridge/submitted.ts). |
| <code>req.14497</code> | importDeposit requires a successful receipt and a BridgeDeposit log from the configured vault, with a canonical Paxeer recipient and a configured asset. Multiple deposits require a logIndex selection; an absent matching log produces an error naming the vault (apps/bridge/src/lib/bridge/import.ts). |
| <code>req.14497</code> | importWithdrawal requires a successful receipt and one matching BridgeOut event from the bridge precompile, using an optional nonce to select among events. It returns a BridgeTransfer with the nonce, chain, asset, amount and recipient from the event; Solana withdrawals require importWithdrawalResolved to resolve the custody recipient (apps/bridge/src/lib/bridge/import.ts). |

## lib

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-lib-bridge-amount-cappercent</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/amount.ts</code> | <code>export function capPercent(part: bigint, whole: bigint): number</code> |
| <code>interface.14.ts-bridge-lib-bridge-amount-formatamount</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/amount.ts</code> | <code>export function formatAmount(value: bigint, decimals: number): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-amount-formatamountdisplay</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/amount.ts</code> | <code>export function formatAmountDisplay(value: bigint, decimals: number): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-amount-hasexcessprecision</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/amount.ts</code> | <code>export function hasExcessPrecision(input: string, decimals: number): boolean</code> |
| <code>interface.14.ts-bridge-lib-bridge-amount-parseamount</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/amount.ts</code> | <code>export function parseAmount(input: string, decimals: number): bigint &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationapierror</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export class AttestationApiError extends Error {}</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-attestationstatus</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export type AttestationStatus = 'observed' &#124; 'finalizing' &#124; 'attesting' &#124; 'ready' &#124; 'unknown';</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-depositattestation</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export type DepositAttestation =</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-fetchdepositattestation</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export async function fetchDepositAttestation</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-fetchreleaseattestation</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export async function fetchReleaseAttestation</code> |
| <code>interface.14.ts-bridge-lib-bridge-attestation-api-releaseattestation</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/attestation-api.ts</code> | <code>export type ReleaseAttestation =</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridge-chains</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export const BRIDGE_CHAINS: BridgeChain[] = buildBridgeChains(bridgeConfig);</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgeasset</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export type BridgeAsset =</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgechain</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export type BridgeChain =</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgechainbyid</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export function bridgeChainById(chainId: bigint): BridgeChain &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-bridgechainbykey</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export function bridgeChainByKey(key: string): BridgeChain &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-finalityestimate</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export function finalityEstimate(chain: BridgeChain): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-paxeer</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export const paxeer = defineChain(</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-paxeer-chain-id</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export const PAXEER_CHAIN_ID = 125;</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-paxeer-route-key</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export const PAXEER_ROUTE_KEY = 'paxeer';</code> |
| <code>interface.14.ts-bridge-lib-bridge-chains-routestate</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/chains.ts</code> | <code>export type RouteState = 'configured' &#124; 'not-deployed' &#124; 'awaiting-backend';</code> |
| <code>interface.14.ts-bridge-lib-bridge-config-bridgeconfig</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/config.ts</code> | <code>export const bridgeConfig =</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-bank-address</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/contracts.ts</code> | <code>export const BANK_ADDRESS = "0x0000000000000000000000000000000000001001" as const;</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-bankabi</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/contracts.ts</code> | <code>export const bankAbi = parseAbi([</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-erc20abi</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/contracts.ts</code> | <code>export const erc20Abi = parseAbi([</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-layerx-bridge-address</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/contracts.ts</code> | <code>export const LAYERX_BRIDGE_ADDRESS = "0x0000000000000000000000000000000000001016" as const;</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-layerxbridgeabi</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/contracts.ts</code> | <code>export const layerxBridgeAbi = layerxBridgeAbiJson as unknown as Abi;</code> |
| <code>interface.14.ts-bridge-lib-bridge-contracts-paxeerxvaultabi</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/contracts.ts</code> | <code>export const paxeerXVaultAbi = parseAbi([</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-domain-in</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export const DOMAIN_IN = 'PAXEERX_BRIDGE_IN_V1';</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-domain-out</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export const DOMAIN_OUT = 'PAXEERX_BRIDGE_OUT_V1';</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-inbounddigest</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export function inboundDigest(args: Parameters&lt;typeof inboundPreimage&gt;[0]): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-inboundpreimage</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export function inboundPreimage(args: { chainId: bigint; vault: Address; txHash: Hex; logIndex: bigint; recipient32: Hex; asset: Address; amount: bigint; }): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-outbounddigest</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export function outboundDigest(args: Parameters&lt;typeof outboundPreimage&gt;[0]): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-outboundpreimage</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export function outboundPreimage(args: { chainId: bigint; vault: Address; paxeerTxHash: Hex; paxeerNonce: bigint; recipient: Address; asset: Address; amount: bigint; }): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-paxeerrecipient</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export function paxeerRecipient(address: Address): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-releasenullifier</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export function releaseNullifier(paxeerTxHash: Hex, paxeerNonce: bigint): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-signaturecheck</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export type SignatureCheck =</code> |
| <code>interface.14.ts-bridge-lib-bridge-digests-verifythresholdsignatures</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/digests.ts</code> | <code>export async function verifyThresholdSignatures(args: { digest: Hex; signatures: readonly Hex[]; attestors: readonly Address[]; threshold: number; }): Promise&lt;SignatureCheck&gt;</code> |
| <code>interface.14.ts-bridge-lib-bridge-errors-describewriteerror</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/errors.ts</code> | <code>export function describeWriteError(error: unknown): string</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-foreignaddressurl</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> | <code>export function foreignAddressUrl(chain: BridgeChain, address: Address): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-foreigntxurl</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> | <code>export function foreignTxUrl(chain: BridgeChain, hash: Hex): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeer</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> | <code>export { paxeer };</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeeraddressurl</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> | <code>export function paxeerAddressUrl</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-paxeertxurl</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> | <code>export function paxeerTxUrl(hash: Hex): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-explorer-txurlbyroutekey</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/explorer.ts</code> | <code>export function txUrlByRouteKey(routeKey: string, hash: Hex): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-base58decode</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> | <code>export function base58Decode(input: string): Uint8Array &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-bytestohex</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> | <code>export function bytesToHex(bytes: Uint8Array): Hex</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-issolanaaddress</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> | <code>export function isSolanaAddress(input: string): boolean</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-solanarecipienthandle</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> | <code>export function solanaRecipientHandle(base58Address: string): Address &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-bridgetransfer</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/transfers.ts</code> | <code>export type BridgeTransfer =</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-listtransfers</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/transfers.ts</code> | <code>export function listTransfers</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-savetransfer</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/transfers.ts</code> | <code>export function saveTransfer(transfer: BridgeTransfer): void</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-transferdirection</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/transfers.ts</code> | <code>export type TransferDirection = 'in' &#124; 'out';</code> |
| <code>interface.14.ts-bridge-lib-bridge-transfers-updatetransfer</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/transfers.ts</code> | <code>export function updateTransfer(id: string, patch: Partial&lt;BridgeTransfer&gt;): void</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-attestationapiconfigured</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export const attestationApiConfigured = bridgeConfig.attestationApiUrl !== undefined;</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-paxeerconfigured</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export const paxeerConfigured = bridgeConfig.paxeerRpcUrl !== undefined;</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-paxeerpairstate</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export type PaxeerPairState =</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usebridgedbalance</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export function useBridgedBalance(denom: string &#124; undefined)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usedepositattestation</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export function useDepositAttestation(transfer: BridgeTransfer &#124; undefined, settled: boolean)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usedepositnullified</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export function useDepositNullified(transfer: BridgeTransfer &#124; undefined)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-useforeignbalance</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export function useForeignBalance(chain: BridgeChain, assetAddress: Address, isNative: boolean)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usepaxeerpairstate</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export function usePaxeerPairState(chain: BridgeChain, assetId: Address): PaxeerPairState</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usereleaseattestation</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export function useReleaseAttestation(transfer: BridgeTransfer &#124; undefined, settled: boolean)</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-usevaultstate</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export function useVaultState(chain: BridgeChain, assetAddress: Address): VaultState</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-vaultstate</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export type VaultState =</code> |
| <code>interface.14.ts-bridge-lib-bridge-wagmi-wagmiconfig</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/wagmi.ts</code> | <code>export const wagmiConfig = createBridgeWagmiConfig(bridgeConfig);</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-drawer</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const EASE_DRAWER = [0.32, 0.72, 0, 1] as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-drawer-css</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const EASE_DRAWER_CSS = "cubic-bezier(0.32, 0.72, 0, 1)";</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-in-out</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const EASE_IN_OUT = [0.77, 0, 0.175, 1] as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-in-out-css</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const EASE_IN_OUT_CSS = "cubic-bezier(0.77, 0, 0.175, 1)";</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-out</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const EASE_OUT = [0.23, 1, 0.32, 1] as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-ease-out-css</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const EASE_OUT_CSS = "cubic-bezier(0.23, 1, 0.32, 1)";</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-glide</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const SPRING_GLIDE = { stiffness: 340, damping: 34, mass: 0.6 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-layout</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const SPRING_LAYOUT = { type: "spring", stiffness: 420, damping: 38, mass: 0.9 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-mouse</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const SPRING_MOUSE = { stiffness: 260, damping: 22, mass: 0.5 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-panel</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const SPRING_PANEL = { type: "spring", stiffness: 380, damping: 34, mass: 1 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-press</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const SPRING_PRESS = { type: "spring", stiffness: 600, damping: 30, mass: 0.6 } as const;</code> |
| <code>interface.14.ts-bridge-lib-ease-spring-swap</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/ease.ts</code> | <code>export const SPRING_SWAP = { type: "spring", stiffness: 520, damping: 36, mass: 0.7 } as const;</code> |
| <code>interface.14.ts-bridge-lib-trading-decimalsfromtick</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export function decimalsFromTick(tick: number)</code> |
| <code>interface.14.ts-bridge-lib-trading-formatcompact</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export const formatCompact = (value: number, dp = 2) =&gt; formatNumber(value, "compact", { dp });</code> |
| <code>interface.14.ts-bridge-lib-trading-formatcountdown</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export function formatCountdown(ms: number)</code> |
| <code>interface.14.ts-bridge-lib-trading-formatnumber</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export function formatNumber(value: number, rule: NumberRule = "number", { dp, tick, sign, pad = true }: FormatOptions = {})</code> |
| <code>interface.14.ts-bridge-lib-trading-formatoptions</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export type FormatOptions =</code> |
| <code>interface.14.ts-bridge-lib-trading-formatpercent</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export const formatPercent = (fraction: number, sign = true, dp = 2) =&gt; formatNumber(fraction, "percent", { sign, dp });</code> |
| <code>interface.14.ts-bridge-lib-trading-formatprice</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export const formatPrice = (value: number, tick?: number) =&gt; formatNumber(value, "price", { tick });</code> |
| <code>interface.14.ts-bridge-lib-trading-formatusd</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export const formatUsd = (value: number, dp = 2) =&gt; formatNumber(value, "usd", { dp });</code> |
| <code>interface.14.ts-bridge-lib-trading-numberrule</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export type NumberRule = "price" &#124; "number" &#124; "percent" &#124; "compact" &#124; "usd";</code> |
| <code>interface.14.ts-bridge-lib-trading-orderside</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export type OrderSide = "buy" &#124; "sell";</code> |
| <code>interface.14.ts-bridge-lib-trading-parsedsymbol</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export type ParsedSymbol = { base: string; quote: string; type: "PERP" &#124; "SPOT" &#124; string };</code> |
| <code>interface.14.ts-bridge-lib-trading-parsesymbol</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export function parseSymbol(symbol: string): ParsedSymbol</code> |
| <code>interface.14.ts-bridge-lib-trading-shortenaddress</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export function shortenAddress(address: string, head = 6, tail = 4)</code> |
| <code>interface.14.ts-bridge-lib-trading-side-fill</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export const SIDE_FILL =</code> |
| <code>interface.14.ts-bridge-lib-trading-sidetone</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export const sideTone = (side: OrderSide): Tone =&gt; (side === "buy" ? "up" : "down");</code> |
| <code>interface.14.ts-bridge-lib-trading-tone</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export type Tone = "up" &#124; "down" &#124; "flat";</code> |
| <code>interface.14.ts-bridge-lib-trading-tone-bg</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export const TONE_BG: Record&lt;Tone, string&gt; =</code> |
| <code>interface.14.ts-bridge-lib-trading-tone-text</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export const TONE_TEXT: Record&lt;Tone, string&gt; =</code> |
| <code>interface.14.ts-bridge-lib-trading-toneof</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export function toneOf(value: number &#124; null &#124; undefined): Tone</code> |
| <code>interface.14.ts-bridge-lib-trading-usecountdown</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/trading.ts</code> | <code>export function useCountdown(target: number &#124; Date &#124; null &#124; undefined)</code> |
| <code>interface.14.ts-bridge-lib-utils-cn</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/utils.ts</code> | <code>export function cn(...inputs: ClassValue[])</code> |

## routes

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.http-bridge-get</code> | <code>http_route</code> | <code>apps/bridge/src/app/page.tsx</code> | <code>GET / (page)</code> |

## security-headers

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-lib-bridge-csp-contentsecuritypolicy-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/csp.ts</code> | <code>export function contentSecurityPolicy(config: BridgeConfig): string</code> |
| <code>interface.14.ts-bridge-proxy-config-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/proxy.ts</code> | <code>export const config = {</code> |
| <code>interface.14.ts-bridge-proxy-proxy-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/proxy.ts</code> | <code>export function proxy</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.14502</code> | contentSecurityPolicy(config) lists in connect-src only self, the Paxeer RPC origin, the attestation API origin and the configured foreign RPC origins read by readConfig (apps/bridge/src/lib/bridge/config.ts), and sets frame-ancestors 'none'. |

## settlement

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-releasecheckof-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export function releaseCheckOf</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-settledrefusal-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export function settledRefusal</code> |
| <code>interface.14.ts-bridge-lib-bridge-import-matchdepositlog-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/import.ts</code> | <code>export function matchDepositLog</code> |
| <code>interface.14.ts-bridge-lib-bridge-import-noncefrombridgeoutlogs-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/import.ts</code> | <code>export function nonceFromBridgeOutLogs</code> |
| <code>interface.14.ts-bridge-lib-bridge-phase-historystatusof-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/phase.ts</code> | <code>export function historyStatusOf(phase: TransferPhase): TransferRecordStatus</code> |
| <code>interface.14.ts-bridge-lib-bridge-phase-stepof-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/phase.ts</code> | <code>export function stepOf(phase: TransferPhase): TransferStep</code> |
| <code>interface.14.ts-bridge-lib-bridge-settlement-phaseof-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/settlement.ts</code> | <code>export function phaseOf</code> |
| <code>interface.14.ts-bridge-lib-bridge-settlement-settlementpatch-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/settlement.ts</code> | <code>export function settlementPatch</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.14490</code> | releaseCheckOf returns undefined for a withdrawal without bridgeOutNonce (apps/bridge/src/lib/bridge/transfers.ts). |
| <code>req.14490</code> | settledRefusal(transfer, live) returns a sentence stating the deposit is already credited when live.credited is true, so the claim button is not offered (apps/bridge/src/components/bridge/transfer-actions.ts). |
| <code>req.14495</code> | nonceFromBridgeOutLogs(logs) returns the nonce of the BridgeOut event emitted by 0x0000000000000000000000000000000000001016 (apps/bridge/src/lib/bridge/contracts.ts). |

## solana-route

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-lib-bridge-solana-parserecipientrecord-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> | <code>export function parseRecipientRecord(bytes: Uint8Array, handle: Address): Uint8Array &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordaddress-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> | <code>export function recipientRecordAddress(programId: string, handle: Address): { address: string; bump: number }</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordfromresponse-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> | <code>export function recipientRecordFromResponse</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-recipientrecordrequest-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana.ts</code> | <code>export function recipientRecordRequest</code> |
| <code>interface.14.ts-bridge-lib-bridge-solana-route-solanawithdrawalblocker-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/solana-route.ts</code> | <code>export function solanaWithdrawalBlocker(lookup: RecipientLookup &#124; undefined, destination: string): string &#124; undefined</code> |
| <code>interface.14.ts-bridge-lib-bridge-use-bridge-state-userecipientrecord-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/use-bridge-state.ts</code> | <code>export function useRecipientRecord</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.14509</code> | recipientRecordAddress(programId, handle) returns the find_program_address result for seeds b"recipient" and the 20-byte handle (contracts/bridge/solana/src/state.rs), a point off the ed25519 curve. |
| <code>req.14509</code> | parseRecipientRecord(bytes, handle) returns the 32-byte key at bytes 30..62 of a 62-byte record starting with PXBRREC0 and layout version 1 (contracts/bridge/solana/src/state.rs, contracts/bridge/solana/src/state.rs, contracts/bridge/solana/src/state.rs). |
| <code>req.14509</code> | parseRecipientRecord returns undefined when bytes 10..30 differ from the expected handle, the magic differs or the length is not 62 (contracts/bridge/solana/src/state.rs). |
| <code>req.14510</code> | recipientRecordRequest(programId, destination) returns a JSON-RPC getAccountInfo body for the recipient record PDA of solanaRecipientHandle(destination) (apps/bridge/src/lib/bridge/solana.ts) with base64 encoding. |
| <code>req.14510</code> | recipientRecordFromResponse returns {registered: false} for a result value null, the state the relayer waits on as Recipient::Pending (services/interop/crates/layerx-bridge-relayer/src/journal.rs). |
| <code>req.14511</code> | solanaWithdrawalBlocker returns the mismatch sentence when the record's key differs from the decoded destination key (apps/bridge/src/lib/bridge/solana.ts). |

## submission

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-lib-bridge-card-state-cardstate-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/card-state.ts</code> | <code>export function cardState(inputs: CardStateInputs): CardState</code> |
| <code>interface.14.ts-bridge-lib-bridge-limits-amountlimiterror-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/limits.ts</code> | <code>export function amountLimitError</code> |
| <code>interface.14.ts-bridge-lib-bridge-limits-maxspendable-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/limits.ts</code> | <code>export function maxSpendable(balance: bigint, isNative: boolean, gasReserve: bigint): bigint</code> |
| <code>interface.14.ts-bridge-lib-bridge-limits-routeblockers-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/limits.ts</code> | <code>export function routeBlockers</code> |
| <code>interface.14.ts-bridge-lib-bridge-submission-plandeposit-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/lib/bridge/submission.ts</code> | <code>export function planDeposit</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.14491</code> | planDeposit returns approve(vault, amount) with the exact amount, never maxUint256, when allowance < amount (apps/bridge/src/lib/bridge/contracts.ts). |
| <code>req.14491</code> | planDeposit returns depositNative(paxeerRecipient) with value amount for the native asset and no approve (apps/bridge/src/lib/bridge/contracts.ts). |

## verification

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-assertclaimmatches-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export function assertClaimMatches(transfer: BridgeTransfer, attestation: DepositAttestation): void</code> |
| <code>interface.14.ts-bridge-components-bridge-transfer-actions-assertreleasematches-j55</code> | <code>ts_export</code> | <code>apps/bridge/src/components/bridge/transfer-actions.ts</code> | <code>export function assertReleaseMatches(transfer: BridgeTransfer, attestation: ReleaseAttestation): void</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.14484</code> | verifyThresholdSignatures with an empty attestor list returns ok false whatever the threshold (apps/bridge/src/lib/bridge/digests.ts). |
| <code>req.14489</code> | assertClaimMatches throws when attestation.recipient32 differs from paxeerRecipient(transfer.recipient) (apps/bridge/src/components/bridge/transfer-actions.ts). |
| <code>req.14489</code> | assertClaimMatches throws when attestation.asset differs from transfer.assetId (apps/bridge/src/components/bridge/transfer-actions.ts). |
| <code>req.14489</code> | assertClaimMatches throws when attestation.chainId differs from transfer.foreignChainId (apps/bridge/src/components/bridge/transfer-actions.ts). |
| <code>req.14489</code> | assertReleaseMatches throws when the release attestation asset or chainId differs from the withdrawal (apps/bridge/src/components/bridge/transfer-actions.ts). |
