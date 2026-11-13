# Perps client contract

The perpetuals application is implemented in [apps/perps](../../../apps/perps). Its components, request clients, routes and configuration are listed below. Market execution is served by the chain; application state and market-data responses are client projections.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## app

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function ConnectWalletModal()` | [apps/perps/src/app/modals/ConnectWalletModal.tsx](../../../apps/perps/src/app/modals/ConnectWalletModal.tsx) |
| `ts_export` | `export function DepositWithdrawModal({` | [apps/perps/src/app/modals/DepositWithdrawModal.tsx](../../../apps/perps/src/app/modals/DepositWithdrawModal.tsx) |
| `ts_export` | `export function SettingsModal({ isOpen, onClose }: SettingsModalProps)` | [apps/perps/src/app/modals/SettingsModal.tsx](../../../apps/perps/src/app/modals/SettingsModal.tsx) |
| `ts_export` | `export function WalletControlPanel({ open, onOpenChange, trigger, onDeposit, onWithdraw, }: WalletControlPanelProps)` | [apps/perps/src/app/modals/WalletControlPanel.tsx](../../../apps/perps/src/app/modals/WalletControlPanel.tsx) |
| `ts_export` | `export function PaxscanProvider({ children }: { children: React.ReactNode })` | [apps/perps/src/app/providers/PaxscanProvider.tsx](../../../apps/perps/src/app/providers/PaxscanProvider.tsx) |
| `ts_export` | `export function AppShell({ Component, pageProps }: AppShellProps)` | [apps/perps/src/app/shell/AppShell.tsx](../../../apps/perps/src/app/shell/AppShell.tsx) |
| `ts_export` | `export function CookieBanner()` | [apps/perps/src/app/shell/CookieBanner.tsx](../../../apps/perps/src/app/shell/CookieBanner.tsx) |
| `ts_export` | `export function DesktopSidebar()` | [apps/perps/src/app/shell/DesktopSidebar.tsx](../../../apps/perps/src/app/shell/DesktopSidebar.tsx) |
| `ts_export` | `export class ErrorBoundary extends React.Component<Props, State>` | [apps/perps/src/app/shell/ErrorBoundary.tsx](../../../apps/perps/src/app/shell/ErrorBoundary.tsx) |
| `ts_export` | `export function Footer()` | [apps/perps/src/app/shell/Footer.tsx](../../../apps/perps/src/app/shell/Footer.tsx) |
| `ts_export` | `export function parseStatusPageUrl(raw: string \| undefined): string \| null` | [apps/perps/src/app/shell/Footer.tsx](../../../apps/perps/src/app/shell/Footer.tsx) |
| `ts_export` | `export function FooterSettings({ isPaused, setIsPaused, showPrice, setShowPrice, tickerDisplayMode, setTickerDisplayMode }: FooterSettingsProps)` | [apps/perps/src/app/shell/FooterSettings.tsx](../../../apps/perps/src/app/shell/FooterSettings.tsx) |
| `ts_export` | `export function Header()` | [apps/perps/src/app/shell/Header.tsx](../../../apps/perps/src/app/shell/Header.tsx) |
| `ts_export` | `export function MobileNavDrawer({ open, onClose }: { open: boolean; onClose: () => void })` | [apps/perps/src/app/shell/MobileNav.tsx](../../../apps/perps/src/app/shell/MobileNav.tsx) |
| `ts_export` | `export function MobileNavTrigger({ onClick }: { onClick: () => void })` | [apps/perps/src/app/shell/MobileNav.tsx](../../../apps/perps/src/app/shell/MobileNav.tsx) |
| `ts_export` | `export function useMobileNav()` | [apps/perps/src/app/shell/MobileNav.tsx](../../../apps/perps/src/app/shell/MobileNav.tsx) |
| `ts_export` | `export function MobileTradeHeader({` | [apps/perps/src/app/shell/MobileTradeHeader.tsx](../../../apps/perps/src/app/shell/MobileTradeHeader.tsx) |
| `ts_export` | `export function SplashScreen()` | [apps/perps/src/app/shell/SplashScreen.tsx](../../../apps/perps/src/app/shell/SplashScreen.tsx) |

## components

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function ActiveOrders()` | [apps/perps/src/components/paxeer/ActiveOrders.tsx](../../../apps/perps/src/components/paxeer/ActiveOrders.tsx) |
| `ts_export` | `export default ActiveOrders;` | [apps/perps/src/components/paxeer/ActiveOrders.tsx](../../../apps/perps/src/components/paxeer/ActiveOrders.tsx) |
| `ts_export` | `export function AddressChip()` | [apps/perps/src/components/paxeer/AddressChip.tsx](../../../apps/perps/src/components/paxeer/AddressChip.tsx) |
| `ts_export` | `export default AddressChip;` | [apps/perps/src/components/paxeer/AddressChip.tsx](../../../apps/perps/src/components/paxeer/AddressChip.tsx) |
| `ts_export` | `export function ClosedPositions()` | [apps/perps/src/components/paxeer/ClosedPositions.tsx](../../../apps/perps/src/components/paxeer/ClosedPositions.tsx) |
| `ts_export` | `export default ClosedPositions;` | [apps/perps/src/components/paxeer/ClosedPositions.tsx](../../../apps/perps/src/components/paxeer/ClosedPositions.tsx) |
| `ts_export` | `export function CommandPalette()` | [apps/perps/src/components/paxeer/CommandPalette.tsx](../../../apps/perps/src/components/paxeer/CommandPalette.tsx) |
| `ts_export` | `export default CommandPalette;` | [apps/perps/src/components/paxeer/CommandPalette.tsx](../../../apps/perps/src/components/paxeer/CommandPalette.tsx) |
| `ts_export` | `export function ConfirmSlider({ label = "Slide to confirm swap", confirmedLabel = "Confirmed", accent = "var(--accent-solid)" })` | [apps/perps/src/components/paxeer/ConfirmSlider.tsx](../../../apps/perps/src/components/paxeer/ConfirmSlider.tsx) |
| `ts_export` | `export default ConfirmSlider;` | [apps/perps/src/components/paxeer/ConfirmSlider.tsx](../../../apps/perps/src/components/paxeer/ConfirmSlider.tsx) |
| `ts_export` | `export function CumulativePnL()` | [apps/perps/src/components/paxeer/CumulativePnL.tsx](../../../apps/perps/src/components/paxeer/CumulativePnL.tsx) |
| `ts_export` | `export default CumulativePnL;` | [apps/perps/src/components/paxeer/CumulativePnL.tsx](../../../apps/perps/src/components/paxeer/CumulativePnL.tsx) |
| `ts_export` | `export function DataInput()` | [apps/perps/src/components/paxeer/DataInput.tsx](../../../apps/perps/src/components/paxeer/DataInput.tsx) |
| `ts_export` | `export default DataInput;` | [apps/perps/src/components/paxeer/DataInput.tsx](../../../apps/perps/src/components/paxeer/DataInput.tsx) |
| `ts_export` | `export default DepthChart;` | [apps/perps/src/components/paxeer/DepthChart.tsx](../../../apps/perps/src/components/paxeer/DepthChart.tsx) |
| `ts_export` | `export function DepthChart()` | [apps/perps/src/components/paxeer/DepthChart.tsx](../../../apps/perps/src/components/paxeer/DepthChart.tsx) |
| `ts_export` | `export default EpochCountdown;` | [apps/perps/src/components/paxeer/EpochCountdown.tsx](../../../apps/perps/src/components/paxeer/EpochCountdown.tsx) |
| `ts_export` | `export function EpochCountdown()` | [apps/perps/src/components/paxeer/EpochCountdown.tsx](../../../apps/perps/src/components/paxeer/EpochCountdown.tsx) |
| `ts_export` | `export default FeesBreakdown;` | [apps/perps/src/components/paxeer/FeesBreakdown.tsx](../../../apps/perps/src/components/paxeer/FeesBreakdown.tsx) |
| `ts_export` | `export function FeesBreakdown()` | [apps/perps/src/components/paxeer/FeesBreakdown.tsx](../../../apps/perps/src/components/paxeer/FeesBreakdown.tsx) |
| `ts_export` | `export default GasWeather;` | [apps/perps/src/components/paxeer/GasWeather.tsx](../../../apps/perps/src/components/paxeer/GasWeather.tsx) |
| `ts_export` | `export function GasWeather()` | [apps/perps/src/components/paxeer/GasWeather.tsx](../../../apps/perps/src/components/paxeer/GasWeather.tsx) |
| `ts_export` | `export default GestureUnlock;` | [apps/perps/src/components/paxeer/GestureUnlock.tsx](../../../apps/perps/src/components/paxeer/GestureUnlock.tsx) |
| `ts_export` | `export function GestureUnlock()` | [apps/perps/src/components/paxeer/GestureUnlock.tsx](../../../apps/perps/src/components/paxeer/GestureUnlock.tsx) |
| `ts_export` | `export default GhostHold;` | [apps/perps/src/components/paxeer/GhostHold.tsx](../../../apps/perps/src/components/paxeer/GhostHold.tsx) |
| `ts_export` | `export function GhostHold()` | [apps/perps/src/components/paxeer/GhostHold.tsx](../../../apps/perps/src/components/paxeer/GhostHold.tsx) |
| `ts_export` | `export default Leaderboard;` | [apps/perps/src/components/paxeer/Leaderboard.tsx](../../../apps/perps/src/components/paxeer/Leaderboard.tsx) |
| `ts_export` | `export function Leaderboard()` | [apps/perps/src/components/paxeer/Leaderboard.tsx](../../../apps/perps/src/components/paxeer/Leaderboard.tsx) |
| `ts_export` | `export default LeaderboardStats;` | [apps/perps/src/components/paxeer/LeaderboardStats.tsx](../../../apps/perps/src/components/paxeer/LeaderboardStats.tsx) |
| `ts_export` | `export function LeaderboardStats()` | [apps/perps/src/components/paxeer/LeaderboardStats.tsx](../../../apps/perps/src/components/paxeer/LeaderboardStats.tsx) |
| `ts_export` | `export default LeverageDial;` | [apps/perps/src/components/paxeer/LeverageDial.tsx](../../../apps/perps/src/components/paxeer/LeverageDial.tsx) |
| `ts_export` | `export function LeverageDial()` | [apps/perps/src/components/paxeer/LeverageDial.tsx](../../../apps/perps/src/components/paxeer/LeverageDial.tsx) |
| `ts_export` | `export default LiquidStat;` | [apps/perps/src/components/paxeer/LiquidStat.tsx](../../../apps/perps/src/components/paxeer/LiquidStat.tsx) |
| `ts_export` | `export function LiquidStat()` | [apps/perps/src/components/paxeer/LiquidStat.tsx](../../../apps/perps/src/components/paxeer/LiquidStat.tsx) |
| `ts_export` | `export default MagneticSwap;` | [apps/perps/src/components/paxeer/MagneticSwap.tsx](../../../apps/perps/src/components/paxeer/MagneticSwap.tsx) |
| `ts_export` | `export function MagneticSwap()` | [apps/perps/src/components/paxeer/MagneticSwap.tsx](../../../apps/perps/src/components/paxeer/MagneticSwap.tsx) |
| `ts_export` | `export default NetworkPill;` | [apps/perps/src/components/paxeer/NetworkPill.tsx](../../../apps/perps/src/components/paxeer/NetworkPill.tsx) |
| `ts_export` | `export function NetworkPill()` | [apps/perps/src/components/paxeer/NetworkPill.tsx](../../../apps/perps/src/components/paxeer/NetworkPill.tsx) |
| `ts_export` | `export default OrbitalSelector;` | [apps/perps/src/components/paxeer/OrbitalSelector.tsx](../../../apps/perps/src/components/paxeer/OrbitalSelector.tsx) |
| `ts_export` | `export function OrbitalSelector()` | [apps/perps/src/components/paxeer/OrbitalSelector.tsx](../../../apps/perps/src/components/paxeer/OrbitalSelector.tsx) |
| `ts_export` | `export default OrderBook;` | [apps/perps/src/components/paxeer/OrderBook.tsx](../../../apps/perps/src/components/paxeer/OrderBook.tsx) |
| `ts_export` | `export function OrderBook()` | [apps/perps/src/components/paxeer/OrderBook.tsx](../../../apps/perps/src/components/paxeer/OrderBook.tsx) |
| `ts_export` | `export default OrderbookTopo;` | [apps/perps/src/components/paxeer/OrderbookTopo.tsx](../../../apps/perps/src/components/paxeer/OrderbookTopo.tsx) |
| `ts_export` | `export function OrderbookTopo()` | [apps/perps/src/components/paxeer/OrderbookTopo.tsx](../../../apps/perps/src/components/paxeer/OrderbookTopo.tsx) |
| `ts_export` | `export default OrderEntry;` | [apps/perps/src/components/paxeer/OrderEntry.tsx](../../../apps/perps/src/components/paxeer/OrderEntry.tsx) |
| `ts_export` | `export function OrderEntry()` | [apps/perps/src/components/paxeer/OrderEntry.tsx](../../../apps/perps/src/components/paxeer/OrderEntry.tsx) |
| `ts_export` | `export default Overview;` | [apps/perps/src/components/paxeer/Overview.tsx](../../../apps/perps/src/components/paxeer/Overview.tsx) |
| `ts_export` | `export function Overview()` | [apps/perps/src/components/paxeer/Overview.tsx](../../../apps/perps/src/components/paxeer/Overview.tsx) |
| `ts_export` | `export default Performance;` | [apps/perps/src/components/paxeer/Performance.tsx](../../../apps/perps/src/components/paxeer/Performance.tsx) |
| `ts_export` | `export function Performance()` | [apps/perps/src/components/paxeer/Performance.tsx](../../../apps/perps/src/components/paxeer/Performance.tsx) |
| `ts_export` | `export default PositionCard;` | [apps/perps/src/components/paxeer/PositionCard.tsx](../../../apps/perps/src/components/paxeer/PositionCard.tsx) |
| `ts_export` | `export function PositionCard({ data })` | [apps/perps/src/components/paxeer/PositionCard.tsx](../../../apps/perps/src/components/paxeer/PositionCard.tsx) |
| `ts_export` | `export default PulseFeed;` | [apps/perps/src/components/paxeer/PulseFeed.tsx](../../../apps/perps/src/components/paxeer/PulseFeed.tsx) |
| `ts_export` | `export function PulseFeed()` | [apps/perps/src/components/paxeer/PulseFeed.tsx](../../../apps/perps/src/components/paxeer/PulseFeed.tsx) |
| `ts_export` | `export default RadialAllocator;` | [apps/perps/src/components/paxeer/RadialAllocator.tsx](../../../apps/perps/src/components/paxeer/RadialAllocator.tsx) |
| `ts_export` | `export function RadialAllocator()` | [apps/perps/src/components/paxeer/RadialAllocator.tsx](../../../apps/perps/src/components/paxeer/RadialAllocator.tsx) |
| `ts_export` | `export default ReactionRail;` | [apps/perps/src/components/paxeer/ReactionRail.tsx](../../../apps/perps/src/components/paxeer/ReactionRail.tsx) |
| `ts_export` | `export function ReactionRail()` | [apps/perps/src/components/paxeer/ReactionRail.tsx](../../../apps/perps/src/components/paxeer/ReactionRail.tsx) |
| `ts_export` | `export default RecentTrades;` | [apps/perps/src/components/paxeer/RecentTrades.tsx](../../../apps/perps/src/components/paxeer/RecentTrades.tsx) |
| `ts_export` | `export function RecentTrades()` | [apps/perps/src/components/paxeer/RecentTrades.tsx](../../../apps/perps/src/components/paxeer/RecentTrades.tsx) |
| `ts_export` | `export default RiskDial;` | [apps/perps/src/components/paxeer/RiskDial.tsx](../../../apps/perps/src/components/paxeer/RiskDial.tsx) |
| `ts_export` | `export function RiskDial()` | [apps/perps/src/components/paxeer/RiskDial.tsx](../../../apps/perps/src/components/paxeer/RiskDial.tsx) |
| `ts_export` | `export default ShardStatus;` | [apps/perps/src/components/paxeer/ShardStatus.tsx](../../../apps/perps/src/components/paxeer/ShardStatus.tsx) |
| `ts_export` | `export function ShardStatus()` | [apps/perps/src/components/paxeer/ShardStatus.tsx](../../../apps/perps/src/components/paxeer/ShardStatus.tsx) |
| `ts_export` | `export function cx(...parts: Array<string \| false \| null \| undefined>): string` | [apps/perps/src/components/paxeer/_shared.tsx](../../../apps/perps/src/components/paxeer/_shared.tsx) |
| `ts_export` | `export function fmt(n: number, digits = 2): string` | [apps/perps/src/components/paxeer/_shared.tsx](../../../apps/perps/src/components/paxeer/_shared.tsx) |
| `ts_export` | `export const Icon =` | [apps/perps/src/components/paxeer/_shared.tsx](../../../apps/perps/src/components/paxeer/_shared.tsx) |
| `ts_export` | `export function shortAddr(a: string): string` | [apps/perps/src/components/paxeer/_shared.tsx](../../../apps/perps/src/components/paxeer/_shared.tsx) |
| `ts_export` | `export type Token =` | [apps/perps/src/components/paxeer/_shared.tsx](../../../apps/perps/src/components/paxeer/_shared.tsx) |
| `ts_export` | `export function TokenBadge({ token, size = 28 }: { token: Token; size?: number })` | [apps/perps/src/components/paxeer/_shared.tsx](../../../apps/perps/src/components/paxeer/_shared.tsx) |
| `ts_export` | `export const TOKENS: Token[] = [` | [apps/perps/src/components/paxeer/_shared.tsx](../../../apps/perps/src/components/paxeer/_shared.tsx) |
| `ts_export` | `export function useAnimatedNumber(target: number, duration = 600): number` | [apps/perps/src/components/paxeer/_shared.tsx](../../../apps/perps/src/components/paxeer/_shared.tsx) |
| `ts_export` | `export default SmartButton;` | [apps/perps/src/components/paxeer/SmartButton.tsx](../../../apps/perps/src/components/paxeer/SmartButton.tsx) |
| `ts_export` | `export function SmartButton()` | [apps/perps/src/components/paxeer/SmartButton.tsx](../../../apps/perps/src/components/paxeer/SmartButton.tsx) |
| `ts_export` | `export default TradePathSankey;` | [apps/perps/src/components/paxeer/TradePathSankey.tsx](../../../apps/perps/src/components/paxeer/TradePathSankey.tsx) |
| `ts_export` | `export function TradePathSankey()` | [apps/perps/src/components/paxeer/TradePathSankey.tsx](../../../apps/perps/src/components/paxeer/TradePathSankey.tsx) |
| `ts_export` | `export default TraderCard;` | [apps/perps/src/components/paxeer/TraderCard.tsx](../../../apps/perps/src/components/paxeer/TraderCard.tsx) |
| `ts_export` | `export function TraderCard()` | [apps/perps/src/components/paxeer/TraderCard.tsx](../../../apps/perps/src/components/paxeer/TraderCard.tsx) |
| `ts_export` | `export default TradesTable;` | [apps/perps/src/components/paxeer/TradesTable.tsx](../../../apps/perps/src/components/paxeer/TradesTable.tsx) |
| `ts_export` | `export function TradesTable()` | [apps/perps/src/components/paxeer/TradesTable.tsx](../../../apps/perps/src/components/paxeer/TradesTable.tsx) |
| `ts_export` | `export default TransferModal;` | [apps/perps/src/components/paxeer/TransferModal.tsx](../../../apps/perps/src/components/paxeer/TransferModal.tsx) |
| `ts_export` | `export function TransferModal({ initialMode = 'deposit' } = {})` | [apps/perps/src/components/paxeer/TransferModal.tsx](../../../apps/perps/src/components/paxeer/TransferModal.tsx) |
| `ts_export` | `export default TransfersHistory;` | [apps/perps/src/components/paxeer/TransfersHistory.tsx](../../../apps/perps/src/components/paxeer/TransfersHistory.tsx) |
| `ts_export` | `export function TransfersHistory()` | [apps/perps/src/components/paxeer/TransfersHistory.tsx](../../../apps/perps/src/components/paxeer/TransfersHistory.tsx) |
| `ts_export` | `export default TxSimulator;` | [apps/perps/src/components/paxeer/TxSimulator.tsx](../../../apps/perps/src/components/paxeer/TxSimulator.tsx) |
| `ts_export` | `export function TxSimulator()` | [apps/perps/src/components/paxeer/TxSimulator.tsx](../../../apps/perps/src/components/paxeer/TxSimulator.tsx) |

## core

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function buildGetFeeTier(account: Address) {` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetMarginTiers(marketId: bigint) {` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetMarketConfig(marketId: bigint) {` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetMarkPriceDetails(marketId: bigint) {` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export const EXPLORER_BASE_URL = PAXSCAN_BASE_URL ?? MAINNET_EXPLORER;` | [apps/perps/src/core/chain.ts](../../../apps/perps/src/core/chain.ts) |
| `ts_export` | `export const paxeerChain = defineChain(` | [apps/perps/src/core/chain.ts](../../../apps/perps/src/core/chain.ts) |
| `ts_export` | `export class PerpsDatafeed implements IBasicDataFeed` | [apps/perps/src/core/datafeed/perps.ts](../../../apps/perps/src/core/datafeed/perps.ts) |
| `ts_export` | `export const ERROR_CATALOG: Record<ErrorCode, CatalogEntry> =` | [apps/perps/src/core/errors/catalog.ts](../../../apps/perps/src/core/errors/catalog.ts) |
| `ts_export` | `export function getCatalogEntry(code: ErrorCode \| undefined \| null): CatalogEntry` | [apps/perps/src/core/errors/catalog.ts](../../../apps/perps/src/core/errors/catalog.ts) |
| `ts_export` | `export function classifyError(raw: unknown, ctx?: ErrorContext): UserFacingError` | [apps/perps/src/core/errors/classify.ts](../../../apps/perps/src/core/errors/classify.ts) |
| `ts_export` | `export function isUserFacingError(v: unknown): v is UserFacingError` | [apps/perps/src/core/errors/classify.ts](../../../apps/perps/src/core/errors/classify.ts) |
| `ts_export` | `export function makeError(code: ErrorCode, ctx?: ErrorContext, cause?: unknown): UserFacingError` | [apps/perps/src/core/errors/classify.ts](../../../apps/perps/src/core/errors/classify.ts) |
| `ts_export` | `export const PRECOMPILE_ERROR_CODES: Readonly<Record<PrecompileErrorName, PrecompileErrorEntry>> =` | [apps/perps/src/core/errors/classify.ts](../../../apps/perps/src/core/errors/classify.ts) |
| `ts_export` | `export interface PrecompileErrorEntry` | [apps/perps/src/core/errors/classify.ts](../../../apps/perps/src/core/errors/classify.ts) |
| `ts_export` | `export type PrecompileErrorName = Extract<(typeof PRECOMPILE_ABI)[number], { type: "error" }>["name"];` | [apps/perps/src/core/errors/classify.ts](../../../apps/perps/src/core/errors/classify.ts) |
| `ts_export` | `export function installGlobalErrorHandlers(): () => void` | [apps/perps/src/core/errors/global.ts](../../../apps/perps/src/core/errors/global.ts) |
| `ts_export` | `export async function leaveBreadcrumb( category: string, message: string, data?: Record<string, unknown>, ): Promise<void>` | [apps/perps/src/core/errors/report.ts](../../../apps/perps/src/core/errors/report.ts) |
| `ts_export` | `export async function reportError( err: unknown, opts: ReportErrorOptions = {}, ): Promise<void>` | [apps/perps/src/core/errors/report.ts](../../../apps/perps/src/core/errors/report.ts) |
| `ts_export` | `export type ErrorCode =` | [apps/perps/src/core/errors/types.ts](../../../apps/perps/src/core/errors/types.ts) |
| `ts_export` | `export type ErrorContext = string;` | [apps/perps/src/core/errors/types.ts](../../../apps/perps/src/core/errors/types.ts) |
| `ts_export` | `export type ErrorSeverity = "info" \| "warning" \| "error" \| "fatal";` | [apps/perps/src/core/errors/types.ts](../../../apps/perps/src/core/errors/types.ts) |
| `ts_export` | `export interface HandleErrorOptions extends ReportErrorOptions` | [apps/perps/src/core/errors/types.ts](../../../apps/perps/src/core/errors/types.ts) |
| `ts_export` | `export interface ReportErrorOptions` | [apps/perps/src/core/errors/types.ts](../../../apps/perps/src/core/errors/types.ts) |
| `ts_export` | `export interface UserFacingError` | [apps/perps/src/core/errors/types.ts](../../../apps/perps/src/core/errors/types.ts) |
| `ts_export` | `export function candleFeedUrl(` | [apps/perps/src/core/feeds.ts](../../../apps/perps/src/core/feeds.ts) |
| `ts_export` | `export const FEED_ORIGIN = feedOrigin.origin;` | [apps/perps/src/core/feeds.ts](../../../apps/perps/src/core/feeds.ts) |
| `ts_export` | `export const FEED_SYMBOLS = [` | [apps/perps/src/core/feeds.ts](../../../apps/perps/src/core/feeds.ts) |
| `ts_export` | ``export const MARKET_DATA_URL = `${FEED_ORIGIN}/api/markets`;`` | [apps/perps/src/core/feeds.ts](../../../apps/perps/src/core/feeds.ts) |
| `ts_export` | `export function normalizeFeedSymbol(symbol: string): string` | [apps/perps/src/core/feeds.ts](../../../apps/perps/src/core/feeds.ts) |
| `ts_export` | `export function gqlEndpoint(url: string, opts: Partial<GraphQLEndpoint> = {}): GraphQLEndpoint` | [apps/perps/src/core/graphql/client.ts](../../../apps/perps/src/core/graphql/client.ts) |
| `ts_export` | `export function gqlQuery<T>( endpoint: GraphQLEndpoint, query: string, variables?: Record<string, unknown>, ): Promise<T>` | [apps/perps/src/core/graphql/client.ts](../../../apps/perps/src/core/graphql/client.ts) |
| `ts_export` | `export interface GraphQLEndpoint` | [apps/perps/src/core/graphql/client.ts](../../../apps/perps/src/core/graphql/client.ts) |
| `ts_export` | `export interface PolledSubscriptionOptions<T>` | [apps/perps/src/core/graphql/client.ts](../../../apps/perps/src/core/graphql/client.ts) |
| `ts_export` | `export interface StreamSubscriptionOptions<T>` | [apps/perps/src/core/graphql/client.ts](../../../apps/perps/src/core/graphql/client.ts) |
| `ts_export` | `export function subscribeQuery<T>(opts: PolledSubscriptionOptions<T>): () => void` | [apps/perps/src/core/graphql/client.ts](../../../apps/perps/src/core/graphql/client.ts) |
| `ts_export` | `export function subscribeStream<T>(opts: StreamSubscriptionOptions<T>): () => void` | [apps/perps/src/core/graphql/client.ts](../../../apps/perps/src/core/graphql/client.ts) |
| `ts_export` | `export function createLogger(ns: string): Logger` | [apps/perps/src/core/log/logger.ts](../../../apps/perps/src/core/log/logger.ts) |
| `ts_export` | `export type LogEvent =` | [apps/perps/src/core/log/logger.ts](../../../apps/perps/src/core/log/logger.ts) |
| `ts_export` | `export interface Logger` | [apps/perps/src/core/log/logger.ts](../../../apps/perps/src/core/log/logger.ts) |
| `ts_export` | `export type LogLevel = "debug" \| "info" \| "warn" \| "error";` | [apps/perps/src/core/log/logger.ts](../../../apps/perps/src/core/log/logger.ts) |
| `ts_export` | `export type LogSink = (event: LogEvent) => void;` | [apps/perps/src/core/log/logger.ts](../../../apps/perps/src/core/log/logger.ts) |
| `ts_export` | `export function setSink(sink: LogSink): void` | [apps/perps/src/core/log/logger.ts](../../../apps/perps/src/core/log/logger.ts) |
| `ts_export` | `export function isSpotOpen( value: string \| undefined = process.env.NEXT_PUBLIC_RELEASE_SPOT_OPEN, ): boolean` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `ts_export` | `export const NAV_GROUP_LABELS: Record<NavGroup, string> =` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `ts_export` | `export const NAV_GROUP_ORDER: NavGroup[] = ["trade", "account"];` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `ts_export` | `export const NAV_ITEMS: NavItem[] = navItems();` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `ts_export` | `export type NavGroup = "trade" \| "account";` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `ts_export` | `export interface NavItem` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `ts_export` | `export function navItems(spotOpen: boolean = isSpotOpen()): NavItem[]` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `ts_export` | `export type NavSurface = "desktopPro" \| "mobileDrawer";` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `ts_export` | `export type FetchErrorKind = "timeout" \| "abort" \| "network" \| "http" \| "parse" \| "unknown";` | [apps/perps/src/core/net/fetch.ts](../../../apps/perps/src/core/net/fetch.ts) |
| `ts_export` | `export async function fetchJsonWithPolicy<T = unknown>(url: string, policy: FetchPolicy = {}): Promise<T>` | [apps/perps/src/core/net/fetch.ts](../../../apps/perps/src/core/net/fetch.ts) |
| `ts_export` | `export interface FetchPolicy extends Omit<RequestInit, "body">` | [apps/perps/src/core/net/fetch.ts](../../../apps/perps/src/core/net/fetch.ts) |
| `ts_export` | `export class FetchPolicyError extends Error` | [apps/perps/src/core/net/fetch.ts](../../../apps/perps/src/core/net/fetch.ts) |
| `ts_export` | `export async function fetchWithPolicy(input: string \| URL \| Request, policy: FetchPolicy = {}): Promise<Response>` | [apps/perps/src/core/net/fetch.ts](../../../apps/perps/src/core/net/fetch.ts) |
| `ts_export` | `export function retryAfterMs(value: string \| null, now = Date.now()): number` | [apps/perps/src/core/net/fetch.ts](../../../apps/perps/src/core/net/fetch.ts) |
| `ts_export` | `export function activePoll(task: () => unknown \| Promise<unknown>, intervalMs: number, immediate = true): CancelTask` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export type CancelTask = () => void;` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export const isPageActive = () => typeof document === 'undefined' \|\| (!document.hidden && (typeof navigator === 'undefined' \|\| navigator.onLine !== false));` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export const jitter = (ceiling: number, random = Math.random) => Math.floor(random() * Math.min(60000, Math.max(0, ceiling)));` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export function notifyStreamResync(url: string)` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export function onStreamResync(callback: (url: string) => void): CancelTask` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export function scheduleActive(task: () => void, ceiling = 1000, url = ''): CancelTask` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export function sequenceGuard()` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export function watchSocket(ws: WebSocket, onStable: () => void, ping?: unknown, requireData = false): void` | [apps/perps/src/core/net/realtime.ts](../../../apps/perps/src/core/net/realtime.ts) |
| `ts_export` | `export const FILL_CAP = 100;` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export const GAP_CLOSE_CODE = 4000;` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function isSpotVenueSymbol(symbol: string \| null \| undefined): boolean` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function isVenueSymbol(symbol: string \| null \| undefined): boolean` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export const MAX_SUBSCRIPTIONS = 200;` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function parseSpotVenueTrade(raw: unknown): SpotVenueTrade \| null` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function parseVenueBook(data: unknown, ts: number): VenueBook \| null` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function parseVenueFill(raw: unknown): VenueFill \| null` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function perpBookTopic(symbol: string): string` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function perpTradeTopic(symbol: string): string` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export const RESYNC_DEPTH = 100;` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function spotBookTopic(symbol: string): string` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function spotTradeTopic(symbol: string): string` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export interface SpotVenueTrade` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function subscribeSpotVenueBook(` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function subscribeSpotVenueTrades(` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function subscribeVenueBook(` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function subscribeVenueFills(` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function useSpotVenueBook(symbol: string \| undefined): VenueBook \| null` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function useSpotVenueTrades(` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function useVenueBook(symbol: string \| undefined): VenueBook \| null` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export function useVenueFills(symbol: string \| undefined): VenueFill[] \| null` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export interface VenueBook` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export interface VenueFill` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export type VenueFrame = { event: "subscribe" \| "unsubscribe"; topic: string };` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `export type VenueLevel = SpotBookLevel;` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |
| `ts_export` | `module.exports = { IERC20ABI };` | [apps/perps/src/core/network/abi/IERC20.js](../../../apps/perps/src/core/network/abi/IERC20.js) |
| `ts_export` | `export const IERC20ABI = [` | [apps/perps/src/core/network/abi/IERC20.ts](../../../apps/perps/src/core/network/abi/IERC20.ts) |
| `ts_export` | `module.exports = {` | [apps/perps/src/core/network/abi/index.js](../../../apps/perps/src/core/network/abi/index.js) |
| `ts_export` | `export const OrderType =` | [apps/perps/src/core/network/abi/PerpsPrecompile.ts](../../../apps/perps/src/core/network/abi/PerpsPrecompile.ts) |
| `ts_export` | `export type OrderType = (typeof OrderType)[keyof typeof OrderType];` | [apps/perps/src/core/network/abi/PerpsPrecompile.ts](../../../apps/perps/src/core/network/abi/PerpsPrecompile.ts) |
| `ts_export` | `export const PerpsPrecompileABI = [` | [apps/perps/src/core/network/abi/PerpsPrecompile.ts](../../../apps/perps/src/core/network/abi/PerpsPrecompile.ts) |
| `ts_export` | `export const TimeInForce =` | [apps/perps/src/core/network/abi/PerpsPrecompile.ts](../../../apps/perps/src/core/network/abi/PerpsPrecompile.ts) |
| `ts_export` | `export type TimeInForce = (typeof TimeInForce)[keyof typeof TimeInForce];` | [apps/perps/src/core/network/abi/PerpsPrecompile.ts](../../../apps/perps/src/core/network/abi/PerpsPrecompile.ts) |
| `ts_export` | `export type ApprovalPlan = readonly ApproveCall[];` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export type ApprovalToken = Address \| null;` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export interface ApproveCall` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export const LAUNCHPAD_SPENDER: Address = PRECOMPILES.launchpad;` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export type NativeTokens = ReadonlySet<Address>;` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export function needsApproval( token: ApprovalToken, allowance: bigint, amount: bigint, native: NativeTokens, ): boolean` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export const PERPS_SPENDER: Address = PRECOMPILES.perps;` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export async function planApproval( token: ApprovalToken, spender: Address, allowance: () => Promise<bigint>, amount: bigint, native: NativeTokens, ): Promise<ApprovalPlan>` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export const SPOT_SPENDERS =` | [apps/perps/src/core/network/approvals.ts](../../../apps/perps/src/core/network/approvals.ts) |
| `ts_export` | `export function gasWithMargin(estimate: bigint): bigint` | [apps/perps/src/core/network/gas.ts](../../../apps/perps/src/core/network/gas.ts) |
| `ts_export` | `export interface GatewayUrls` | [apps/perps/src/core/network/gateway.ts](../../../apps/perps/src/core/network/gateway.ts) |
| `ts_export` | `export function gatewayUrls(): GatewayUrls` | [apps/perps/src/core/network/gateway.ts](../../../apps/perps/src/core/network/gateway.ts) |
| `ts_export` | `export const PRECOMPILES =` | [apps/perps/src/core/network/precompiles.ts](../../../apps/perps/src/core/network/precompiles.ts) |
| `ts_export` | `export const pecorAbi = [` | [apps/perps/src/core/network/spot_abi/PECOR.ts](../../../apps/perps/src/core/network/spot_abi/PECOR.ts) |
| `ts_export` | `export const pecorOrdersAbi = [` | [apps/perps/src/core/network/spot_abi/PECOROrders.ts](../../../apps/perps/src/core/network/spot_abi/PECOROrders.ts) |
| `ts_export` | `export const pecorQuoterAbi = [` | [apps/perps/src/core/network/spot_abi/PECORQuoter.ts](../../../apps/perps/src/core/network/spot_abi/PECORQuoter.ts) |
| `ts_export` | `export const pecorStopOrdersAbi = [` | [apps/perps/src/core/network/spot_abi/PECORStopOrders.ts](../../../apps/perps/src/core/network/spot_abi/PECORStopOrders.ts) |
| `ts_export` | `export const pecorVaultAbi = [` | [apps/perps/src/core/network/spot_abi/PECORVault.ts](../../../apps/perps/src/core/network/spot_abi/PECORVault.ts) |
| `ts_export` | `export function buildUrl<R extends ResourceName>( resource: R, pathParams?: PathParams<R>, queryParams?: QueryParams<R>, ): string` | [apps/perps/src/core/paxscan/api/buildUrl.ts](../../../apps/perps/src/core/paxscan/api/buildUrl.ts) |
| `ts_export` | `export class PaxscanError extends Error` | [apps/perps/src/core/paxscan/api/fetch.ts](../../../apps/perps/src/core/paxscan/api/fetch.ts) |
| `ts_export` | `export async function paxscanFetch<T>(url: string, init?: RequestInit): Promise<T>` | [apps/perps/src/core/paxscan/api/fetch.ts](../../../apps/perps/src/core/paxscan/api/fetch.ts) |
| `ts_export` | `export interface ApiResource` | [apps/perps/src/core/paxscan/api/resources.ts](../../../apps/perps/src/core/paxscan/api/resources.ts) |
| `ts_export` | `export type PaginatedResourceName =` | [apps/perps/src/core/paxscan/api/resources.ts](../../../apps/perps/src/core/paxscan/api/resources.ts) |
| `ts_export` | `export type PathParams<R extends ResourceName> = R extends keyof PathParamsMap ? PathParamsMap[R] : never;` | [apps/perps/src/core/paxscan/api/resources.ts](../../../apps/perps/src/core/paxscan/api/resources.ts) |
| `ts_export` | `export type QueryParams<_R extends ResourceName = ResourceName> =` | [apps/perps/src/core/paxscan/api/resources.ts](../../../apps/perps/src/core/paxscan/api/resources.ts) |
| `ts_export` | `export type ResourceName = keyof typeof RESOURCES;` | [apps/perps/src/core/paxscan/api/resources.ts](../../../apps/perps/src/core/paxscan/api/resources.ts) |
| `ts_export` | `export type ResourcePayload<R extends ResourceName> = R extends keyof PayloadMap ? PayloadMap[R] : never;` | [apps/perps/src/core/paxscan/api/resources.ts](../../../apps/perps/src/core/paxscan/api/resources.ts) |
| `ts_export` | `export const RESOURCES =` | [apps/perps/src/core/paxscan/api/resources.ts](../../../apps/perps/src/core/paxscan/api/resources.ts) |
| `ts_export` | `export const PAXSCAN_API_URL = process.env.NEXT_PUBLIC_PAXSCAN_API_URL ?? '';` | [apps/perps/src/core/paxscan/config.ts](../../../apps/perps/src/core/paxscan/config.ts) |
| `ts_export` | `export const PAXSCAN_BASE_URL = parseBaseUrl(process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL);` | [apps/perps/src/core/paxscan/config.ts](../../../apps/perps/src/core/paxscan/config.ts) |
| `ts_export` | `export const PAXSCAN_WS_URL = process.env.NEXT_PUBLIC_PAXSCAN_WS_URL ?? '';` | [apps/perps/src/core/paxscan/config.ts](../../../apps/perps/src/core/paxscan/config.ts) |
| `ts_export` | `export function PaxscanSocketProvider({ children, options, url, name = 'default' }: PaxscanSocketProviderProps)` | [apps/perps/src/core/paxscan/socket/context.tsx](../../../apps/perps/src/core/paxscan/socket/context.tsx) |
| `ts_export` | `export function usePaxscanSocket(name: string = 'default')` | [apps/perps/src/core/paxscan/socket/context.tsx](../../../apps/perps/src/core/paxscan/socket/context.tsx) |
| `ts_export` | `export namespace SocketMessage` | [apps/perps/src/core/paxscan/socket/types.ts](../../../apps/perps/src/core/paxscan/socket/types.ts) |
| `ts_export` | `export type SocketMessageParams =` | [apps/perps/src/core/paxscan/socket/types.ts](../../../apps/perps/src/core/paxscan/socket/types.ts) |
| `ts_export` | `export function usePaxscanSocketChannel({ topic, params, isDisabled, onJoin, onSocketClose, onSocketError, socketName }: Params)` | [apps/perps/src/core/paxscan/socket/useSocketChannel.tsx](../../../apps/perps/src/core/paxscan/socket/useSocketChannel.tsx) |
| `ts_export` | `export function usePaxscanSocketMessage({ channel, event, handler }: SocketMessageParams)` | [apps/perps/src/core/paxscan/socket/useSocketMessage.tsx](../../../apps/perps/src/core/paxscan/socket/useSocketMessage.tsx) |
| `ts_export` | ``export const paxscanAddressUrl = (addr: string) => `${ EXPLORER_BASE_URL }/address/${ addr }`;`` | [apps/perps/src/core/paxscan/urls.ts](../../../apps/perps/src/core/paxscan/urls.ts) |
| `ts_export` | ``export const paxscanBlockUrl = (n: number \| string) => `${ EXPLORER_BASE_URL }/block/${ n }`;`` | [apps/perps/src/core/paxscan/urls.ts](../../../apps/perps/src/core/paxscan/urls.ts) |
| `ts_export` | ``export const paxscanSearchUrl = (q: string) => `${ EXPLORER_BASE_URL }/search-results?q=${ encodeURIComponent(q) }`;`` | [apps/perps/src/core/paxscan/urls.ts](../../../apps/perps/src/core/paxscan/urls.ts) |
| `ts_export` | ``export const paxscanTokenUrl = (addr: string) => `${ EXPLORER_BASE_URL }/token/${ addr }`;`` | [apps/perps/src/core/paxscan/urls.ts](../../../apps/perps/src/core/paxscan/urls.ts) |
| `ts_export` | ``export const paxscanTxUrl = (hash: string) => `${ EXPLORER_BASE_URL }/tx/${ hash }`;`` | [apps/perps/src/core/paxscan/urls.ts](../../../apps/perps/src/core/paxscan/urls.ts) |
| `ts_export` | `export function getAuthHeaders(): Record<string, string>` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export function getAuthToken(): string \| null` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export const PERFDB_GRAPHQL: GraphQLEndpoint = gqlEndpoint(` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export const PERFDB_REST_URL = PERFDB_URL;` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export const PERFDB_URL = gatewayUrls().perfdbOrigin;` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export const PERFDB_WS_URL = PERFDB_URL.replace(/^https/, "wss").replace(` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export async function perfdbQuery<T>( document: string, variables?: Record<string, unknown> ): Promise<T>` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export async function perfdbQueryAuth<T>( document: string, variables?: Record<string, unknown> ): Promise<T>` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export async function perfdbRest<T>( path: string, options?: RequestInit ): Promise<T>` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export async function perfdbRestAuth<T>( path: string, options?: RequestInit ): Promise<T>` | [apps/perps/src/core/perfdb/client.ts](../../../apps/perps/src/core/perfdb/client.ts) |
| `ts_export` | `export interface AccountView` | [apps/perps/src/core/perps/accountView.ts](../../../apps/perps/src/core/perps/accountView.ts) |
| `ts_export` | `export function accountViewFromState(accountState: AccountState): AccountView` | [apps/perps/src/core/perps/accountView.ts](../../../apps/perps/src/core/perps/accountView.ts) |
| `ts_export` | `export function pickCollateral(accountState: Pick<AccountState, "collateral">): PickedCollateral` | [apps/perps/src/core/perps/accountView.ts](../../../apps/perps/src/core/perps/accountView.ts) |
| `ts_export` | `export interface PickedCollateral` | [apps/perps/src/core/perps/accountView.ts](../../../apps/perps/src/core/perps/accountView.ts) |
| `ts_export` | `export function deriveImpliedCollateral( sizeUsd1e18: bigint, leverage1e18: bigint, collateralDecimals: number, ): bigint` | [apps/perps/src/core/perps/amounts.ts](../../../apps/perps/src/core/perps/amounts.ts) |
| `ts_export` | `export function getCollateralDecimals(tokenAddress: string): number` | [apps/perps/src/core/perps/amounts.ts](../../../apps/perps/src/core/perps/amounts.ts) |
| `ts_export` | `export function safeParseUnits(value: string \| undefined \| null, decimals: number): bigint` | [apps/perps/src/core/perps/amounts.ts](../../../apps/perps/src/core/perps/amounts.ts) |
| `ts_export` | `export function normalizeLevels(raw: unknown, side: "asks" \| "bids"): SpotBookLevel[]` | [apps/perps/src/core/perps/bookLevels.ts](../../../apps/perps/src/core/perps/bookLevels.ts) |
| `ts_export` | `export interface SpotBookLevel` | [apps/perps/src/core/perps/bookLevels.ts](../../../apps/perps/src/core/perps/bookLevels.ts) |
| `ts_export` | `export function acceptablePrice( price: bigint, isLong: boolean, action: "open" \| "close", bps: number = DEFAULT_SLIPPAGE_BPS, ): bigint` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export type AccountState = ContractFunctionReturnType<PerpsAbi, "view", "getAccount">;` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface AddSizeInput` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildAddCollateral(positionId: bigint, amount: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildAddSize(input: AddSizeInput)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildCancelByCloid(marketId: bigint, cloid: Hex)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildCancelOrder(orderId: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildClosePosition(input: ClosePositionInput)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildDeposit(amount: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export async function buildDepositPlan( amount: bigint, allowance: () => Promise<bigint>, native: NativeTokens, ): Promise<DepositPlan>` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetAccount(account: Address)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetAccountSummary(account: Address, token: Address)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetBalance(account: Address, token: Address)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetLockedBalance(account: Address, token: Address)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetOrder(orderId: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetPosition(positionId: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetPositionDetails(positionId: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetPositionMargin(positionId: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetUserOrderIds(account: Address)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildGetUserPositionIds(account: Address)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildOpenPosition(input: OpenPositionInput)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildPartialClose(input: PartialCloseInput)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildPlaceLimitOrder(input: LimitOrderInput)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildPlaceStopLimitOrder(input: StopLimitOrderInput)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildRemoveCollateral(positionId: bigint, amount: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildSetPositionTpSl(input: SetPositionTpSlInput)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildTopUpPosition(positionId: bigint, amount: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildWithdraw(amount: bigint)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function buildWithdrawAll(accountState: Pick<AccountState, "withdrawable">)` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface ClosePositionInput` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export type DepositPlan = readonly (ApproveCall \| ReturnType<typeof buildDeposit>)[];` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function ladderCloid(ladderId: bigint, rung: number): Hex` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface LimitOrderInput` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface OpenPositionInput` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface OpenQuote` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export type OrderParams = ContractFunctionArgs<PerpsAbi, "nonpayable", "placeOrder">[0];` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface PartialCloseInput` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface PerpsCall<` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface SetPositionTpSlInput` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export interface StopLimitOrderInput extends LimitOrderInput` | [apps/perps/src/core/perps/calls.ts](../../../apps/perps/src/core/perps/calls.ts) |
| `ts_export` | `export function toCollateralUnits(text: string): bigint` | [apps/perps/src/core/perps/collateralInputs.ts](../../../apps/perps/src/core/perps/collateralInputs.ts) |
| `ts_export` | `export const ANALYTICS_AXIS_FORMATTERS =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const ANALYTICS_CHART_COLORS =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export function coinLogoUrl(symbol: string): string` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const COLLATERAL_OPTIONS = Object.entries(COLLATERAL_TOKENS).map(` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const COLLATERAL_TOKENS =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const DEFAULT_SLIPPAGE_BPS = 50; // 0.5%` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const erc20Abi = [` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const EXPLORER_URL = paxeerChain.blockExplorers.default.url;` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const FEATURED_MARKET_SYMBOLS: readonly string[] = [` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const FEE_TYPE_LABELS: Record<number, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export function fromOrderlySymbol(orderlySymbol: string): string` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export function getMarketCategory(marketId: number): string` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const INDEXER_GRAPHQL: GraphQLEndpoint = gqlEndpoint(gatewayUrls().graphql.url,` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const MARGIN_MODE_LABELS: Record<number, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const MARGIN_MODE_TOGGLE_COPY =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const MARKET_CATEGORIES: Record<string, number[]> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const MARKET_CATEGORY_LABELS: Record<string, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const MARKET_LOGO_FILES: Readonly<Record<string, string>> = Object.freeze(` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const MARKET_LOGO_PLACEHOLDER = "/market-logos/generic.svg";` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const MARKET_LOGO_SYMBOLS: ReadonlySet<string> = new Set(Object.keys(MARKET_LOGO_FILES));` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export function marketLogoUrl(symbol: string): string` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const ORDER_TYPE_COLORS: Record<number, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const ORDER_TYPE_LABELS: Record<number, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const PERPS_MARKETS: PerpsMarket[] = [` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export interface PerpsMarket` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const POSITION_ACTION_COPY =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const SPOT_ORDER_TYPE_LABELS: Record<number, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const SPOT_SIDE_LABELS: Record<number, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export function toOrderlySymbol(paxeerSymbol: string): string` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const TP_SL_COPY =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const TRACK_B_STRINGS =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const TRADE_TYPE_LABELS: Record<number, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export const VAULT_TYPE_LABELS: Record<number, string> =` | [apps/perps/src/core/perps/config.ts](../../../apps/perps/src/core/perps/config.ts) |
| `ts_export` | `export function formatFundingDecimalPct(v: string \| null, dp = 4): string` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function formatFundingRatePct(rate: bigint, dp = 4): string` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function formatSignedFundingDecimalPct(v: string \| null, dp = 4): string` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export const FUNDING_LABEL_1H = "last settled funding (1h)";` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export const FUNDING_LABEL_24H = "last settled funding (1h) ×24 projection";` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export const FUNDING_LABEL_PREDICTED = "Pred. funding rate";` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export const FUNDING_RATE_DECIMALS = 18;` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function fundingDecimalSign(v: string \| null): -1 \| 0 \| 1 \| null` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function fundingFigures(hourly: bigint \| null):` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function fundingRateFromJsonNumber(v: unknown): string \| null` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function fundingRowFigure( socketRate: string \| null \| undefined, venueHourly: bigint \| null, ):` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function fundingWadDecimal(rate: bigint \| null): string \| null` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function fundingWadFigure(rate: bigint \| null):` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function parseFundingRateHourly(v: string \| null \| undefined): bigint` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function parseFundingRateWad(v: string \| null \| undefined): bigint \| null` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function projectFunding24h(hourly: bigint): bigint` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export interface VenueTicker` | [apps/perps/src/core/perps/fundingRate.ts](../../../apps/perps/src/core/perps/fundingRate.ts) |
| `ts_export` | `export function buildConfirmation(quote: OpenPositionQuote \| null, plan: OrderPlan): OrderConfirmation \| null` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function buildOrderPlan(state: OrderFormState, quote: OpenPositionQuote \| null, limits: OrderLimits): OrderPlan {` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export type ClearedInputs = Pick<` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export const MAX_SCALED_ORDERS = 20;` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export const MIN_SCALED_ORDERS = 2;` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function needsConfirmation(orderConfirm: boolean): boolean` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function needsRequote(quoteAgeMs: number): boolean` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export interface OrderConfirmation` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export interface OrderFormState extends OrderOptions` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export type OrderKind = "market" \| "limit" \| "stop-limit" \| "scaled";` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export interface OrderOptions` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export interface OrderPlan` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function parseLeverage(value: string): bigint` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export type PlaceOrderCall = ReturnType<typeof buildOpenPosition>;` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export const QUOTE_MAX_AGE_MS = 6_000;` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export type ScaledDistribution = "flat" \| "asc" \| "desc" \| "custom";` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function selectOrderOption(options: OrderOptions, option: keyof OrderOptions, on: boolean): OrderOptions` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export interface SuccessReset` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function successReset(status: "success" \| "reverted"): SuccessReset \| null` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function timeInForceOf(kind: OrderKind, options: OrderOptions): TimeInForce` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function collateralHealthColor(healthPct: bigint): string` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function computePositionPnl(` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function decimalToWad(v: string): bigint` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function formatPnlPercent(value: bigint): string` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function formatPnlValue(value: bigint): string` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function formatWadFixed(v: bigint, dp: number, group = false): string` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function markPriceWad(v: number \| string \| null \| undefined): bigint` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export interface ParsedPosition` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function parsePositionRow(row: PositionRowInput, symbol: string): ParsedPosition` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function pctWad(num: bigint, den: bigint): bigint` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function pnlColorClass(value: bigint): string` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export interface PositionPnl` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export interface PositionRowInput` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export const WAD = 10n ** 18n;` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function wadToDecimal(v: bigint): string` | [apps/perps/src/core/perps/pnlHelpers.ts](../../../apps/perps/src/core/perps/pnlHelpers.ts) |
| `ts_export` | `export function portfolioFromAccount( accountState: Pick<AccountState, "collateral" \| "equityUsd" \| "withdrawable">, positions: readonly PortfolioPosition[], ): PortfolioTotals` | [apps/perps/src/core/perps/portfolio.ts](../../../apps/perps/src/core/perps/portfolio.ts) |
| `ts_export` | `export interface PortfolioPosition` | [apps/perps/src/core/perps/portfolio.ts](../../../apps/perps/src/core/perps/portfolio.ts) |
| `ts_export` | `export interface PortfolioTotals` | [apps/perps/src/core/perps/portfolio.ts](../../../apps/perps/src/core/perps/portfolio.ts) |
| `ts_export` | `export const PARTIAL_CLOSE_BPS = [2_500, 5_000, 7_500] as const;` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export interface PartialCloseAction` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export function partialCloseAction(input: PartialCloseAction)` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export function partialCloseSize(sizeUsd: bigint, bps: number): bigint` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export interface PositionAction` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export type PositionActionKind = "close" \| "partialClose" \| "adjustCollateral" \| "topUp" \| "setTpSl" \| "cancel";` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export const positionActions: readonly PositionAction[] = [` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export function setTpSlAction(positionId: bigint, isLong: boolean, takeProfit: string, stopLoss: string)` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export function unitsFromIndexed(value: string): bigint` | [apps/perps/src/core/perps/positionActions.ts](../../../apps/perps/src/core/perps/positionActions.ts) |
| `ts_export` | `export interface AllowanceSource` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export type CloseCall = ReturnType<typeof buildClosePosition>;` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export type CollateralCall = ReturnType<typeof buildAddCollateral> \| ReturnType<typeof buildRemoveCollateral>;` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export interface CollateralInput` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export type PlaceOrderCall = ReturnType<typeof buildOpenPosition>;` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export async function planAddSize(input: AddSizeInput): Promise<WritePlan<PlaceOrderCall>> {` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export function planClose(input: ClosePositionInput): WritePlan<CloseCall>` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export async function planCollateral(input: CollateralInput, source: AllowanceSource): Promise<WritePlan<CollateralCall>>` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export async function planOpen(input: OpenPositionInput): Promise<WritePlan<PlaceOrderCall>> {` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export async function planOrder(calls: readonly PlaceOrderCall[]): Promise<WritePlan<PlaceOrderCall>> {` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export function planPartialClose(input: PartialCloseInput): WritePlan<CloseCall>` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export function positionFromTuple(tuple: PositionTuple): PositionRecord` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export interface PositionRecord` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export type PositionTuple = ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getPosition">;` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export type PositionWrite = PlaceOrderCall \| CloseCall \| CollateralCall;` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export type WritePlan<C extends PositionWrite = PositionWrite> = readonly (ApproveCall \| C)[];` | [apps/perps/src/core/perps/positionWrites.ts](../../../apps/perps/src/core/perps/positionWrites.ts) |
| `ts_export` | `export interface MarketQuote` | [apps/perps/src/core/perps/quoteFallback.ts](../../../apps/perps/src/core/perps/quoteFallback.ts) |
| `ts_export` | `export function marketQuoteFromPrecompile(result: PrecompileMarketQuote): MarketQuote` | [apps/perps/src/core/perps/quoteFallback.ts](../../../apps/perps/src/core/perps/quoteFallback.ts) |
| `ts_export` | `export interface PartialCloseQuote` | [apps/perps/src/core/perps/quoteFallback.ts](../../../apps/perps/src/core/perps/quoteFallback.ts) |
| `ts_export` | `export function partialCloseQuoteFromPrecompile(result: PrecompilePartialCloseQuote): PartialCloseQuote` | [apps/perps/src/core/perps/quoteFallback.ts](../../../apps/perps/src/core/perps/quoteFallback.ts) |
| `ts_export` | `export type PrecompileMarketQuote = ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "quoteMarket">;` | [apps/perps/src/core/perps/quoteFallback.ts](../../../apps/perps/src/core/perps/quoteFallback.ts) |
| `ts_export` | `export type PrecompilePartialCloseQuote = ContractFunctionReturnType<` | [apps/perps/src/core/perps/quoteFallback.ts](../../../apps/perps/src/core/perps/quoteFallback.ts) |
| `ts_export` | `export function buildPaxeerTransport(): Transport` | [apps/perps/src/core/rpc.ts](../../../apps/perps/src/core/rpc.ts) |
| `ts_export` | `export function getPaxeerRpcUrls(): string[]` | [apps/perps/src/core/rpc.ts](../../../apps/perps/src/core/rpc.ts) |
| `ts_export` | `export function getTokenByAddress( addressLike: string, ): SpotTokenMeta \| undefined` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export function getTokenBySymbol(symbol: string): SpotTokenMeta \| undefined` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export function isStablecoinAddress(addressLike: string): boolean` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export const NATIVE_PAX_ADDRESS: Address =` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export const PECOR_CONTRACTS = PRECOMPILES.spot;` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export const SPOT_STABLECOIN_ADDRESSES: ReadonlySet<string> = new Set(` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export const SPOT_STABLECOIN_LIST: readonly SpotTokenMeta[] = [SPOT_TOKENS.USDL];` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export const SPOT_TOKENS =` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export interface SpotTokenMeta` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export type SpotTokenSymbol = keyof typeof SPOT_TOKENS;` | [apps/perps/src/core/spot/addresses.ts](../../../apps/perps/src/core/spot/addresses.ts) |
| `ts_export` | `export function dStr(v: string \| null \| undefined, fallback = 0): number` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export function dStrToToken( raw: string \| null \| undefined, decimals: number \| null \| undefined, fallback = 0, ): number` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export function dStrToUsd(raw: string \| null \| undefined, fallback = 0): number` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export function positionUsdValue(p: SpotPosition, currentPrice: number): number` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export const spotApi =` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export class SpotApiError extends Error` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotClosedLeg` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotClosedLegsResponse` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotHoldingsResponse` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotIdentity` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotOrder` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotOrdersResponse` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export type SpotOrderStatus = "open" \| "executed" \| "cancelled" \| "activated";` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export type SpotOrderType = "limit" \| "stop_limit" \| "stop_loss";` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotPnlSummary` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotPosition` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotPositionsResponse` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotTrade` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotTradesResponse` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export interface SpotTradesStats` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export function symbolOrShort(token: string, symbol: string \| null \| undefined): string` | [apps/perps/src/core/spot/api.ts](../../../apps/perps/src/core/spot/api.ts) |
| `ts_export` | `export function buildCancelLimitOrder(orderId: bigint)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildCancelStopLimitOrder(orderId: bigint)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildCancelStopLossOrder(orderId: bigint)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildGetLimitOrder(orderId: bigint)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildGetPaxPointer()` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildGetStopLimitOrder(orderId: bigint)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildGetStopLossOrder(orderId: bigint)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildGetUserLimitOrders(user: Address)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildGetUserStopLimitOrders(user: Address)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildGetUserStopLossOrders(user: Address)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildMarketBuy(input: MarketBuyCallInput)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildMarketSell(input: MarketSellCallInput)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildModifyLimitOrder(input: { orderId: bigint; newTargetPrice: bigint; newExpiresAt: bigint })` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildPlaceLimitBuy(input: LimitCallInput & { stablecoin: Address; token: Address })` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildPlaceLimitSell(input: LimitCallInput & { token: Address; stablecoin: Address })` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildPlaceStopLimitBuy(input: StopLimitCallInput & { stablecoin: Address; token: Address })` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildPlaceStopLimitSell(input: StopLimitCallInput & { token: Address; stablecoin: Address })` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildPlaceStopLoss(input: { token: Address; stablecoin: Address; tokenAmount: bigint; /** Trigger price, 18 decimals: the sell fires when the mark is at or below it. */ triggerPrice: bigint; expiresAt: bigint; })` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function buildSwapExactInNative(input: SwapExactInNativeCallInput)` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export interface LimitCallInput` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export interface MarketBuyCallInput` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export interface MarketSellCallInput` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export interface SpotCall<` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export interface StopLimitCallInput` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export interface SwapExactInNativeCallInput` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export const WEI_PER_UHPX = 10n ** 12n;` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export const DEFAULT_SPOT_SYMBOL = "SID";` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export function getSpotMarket(symbol: string): SpotMarket \| undefined` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export const SPOT_CATEGORIES: Record<string, string[]> =` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export const SPOT_CATEGORY_LABELS: Record<string, string> =` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export const SPOT_MARKETS: SpotMarket[] = [` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export const SPOT_STABLECOINS: readonly StablecoinMeta[] = SPOT_STABLECOIN_LIST;` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export const SPOT_SYMBOLS = new Set(SPOT_MARKETS.map((m) => m.symbol));` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export function spotCoinLogoUrl(symbol: string): string` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export interface SpotMarket` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export type StablecoinMeta = SpotTokenMeta;` | [apps/perps/src/core/spot/config.ts](../../../apps/perps/src/core/spot/config.ts) |
| `ts_export` | `export type BatchQuoteRequests = ContractFunctionArgs<QuoterAbi, "view", "batchQuote">[0];` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildBatchQuote(requests: BatchQuoteRequests)` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildGetAllLiquidityInfo()` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildGetLiquidityInfo(token: Address)` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildGetTokenPrice(token: Address)` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildGetTWAP(token: Address, period: bigint)` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildQuoteExactIn(tokenIn: Address, tokenOut: Address, amountIn: bigint)` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildQuoteExactOut(tokenIn: Address, tokenOut: Address, amountOut: bigint)` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildQuoteMarketBuy(stablecoin: Address, token: Address, stablecoinAmount: bigint)` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function buildQuoteMarketSell(token: Address, stablecoin: Address, tokenAmount: bigint)` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export interface QuoterCall<` | [apps/perps/src/core/spot/quoterCalls.ts](../../../apps/perps/src/core/spot/quoterCalls.ts) |
| `ts_export` | `export function applySlippageMaxIn( expectedIn: bigint, slippageBps: number, ): bigint` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function applySlippageMinOut( expectedOut: bigint, slippageBps: number, ): bigint` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export const BPS_DENOMINATOR = 10_000n;` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function buildDeadline(secondsFromNow: number = DEFAULT_MARKET_DEADLINE_SECONDS): bigint` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function buildLimitExpiry( secondsFromNow: number = DEFAULT_LIMIT_EXPIRY_SECONDS, ): bigint` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export const DEFAULT_LIMIT_EXPIRY_SECONDS = 24 * 60 * 60; // 24 h` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export const DEFAULT_MARKET_DEADLINE_SECONDS = 60;` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | ``export function expectedOutFromPrice(params: { amountIn: bigint; price: bigint; /** Decimals of `amountIn`. */ amountInDecimals: number; /** Decimals of the output token. */ amountOutDecimals: number; }): bigint`` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function formatPrice(value: bigint): string` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function formatTokenAmount(value: bigint, decimals: number): string` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function limitMinOut( amountIn: bigint, limitPrice: bigint, isBuy: boolean, decimalsIn: number, decimalsOut: number, ): bigint` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function parsePrice(value: string \| number): bigint` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function parseTokenAmount(value: string \| number, decimals: number): bigint` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export const PRICE_DECIMALS = 18;` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export const PRICE_SCALE = 10n ** BigInt(PRICE_DECIMALS);` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function quoteDecimals( stables: readonly { readonly decimals: number }[], ): number` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function spotOrderRail( orderType: SpotOrderType, tif: SpotTimeInForce, postOnly: boolean, ): SpotRail` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export type SpotOrderType = "market" \| "limit" \| "stop-limit";` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export type SpotRail = "market" \| "limit" \| "stop";` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export type SpotTimeInForce = "GTC" \| "IOC" \| "FOK";` | [apps/perps/src/core/spot/slippage.ts](../../../apps/perps/src/core/spot/slippage.ts) |
| `ts_export` | `export function buildStableAmountMap( entries: ReadonlyArray<{ address: Address \| string; amount: bigint }>, ): StableAmountMap` | [apps/perps/src/core/spot/stableSelector.ts](../../../apps/perps/src/core/spot/stableSelector.ts) |
| `ts_export` | `export function selectStableForBuy(input: SelectStableForBuyInput): StableSelection` | [apps/perps/src/core/spot/stableSelector.ts](../../../apps/perps/src/core/spot/stableSelector.ts) |
| `ts_export` | `export interface SelectStableForBuyInput` | [apps/perps/src/core/spot/stableSelector.ts](../../../apps/perps/src/core/spot/stableSelector.ts) |
| `ts_export` | `export function selectStableForSell(input: SelectStableForSellInput): StableSelection` | [apps/perps/src/core/spot/stableSelector.ts](../../../apps/perps/src/core/spot/stableSelector.ts) |
| `ts_export` | `export interface SelectStableForSellInput` | [apps/perps/src/core/spot/stableSelector.ts](../../../apps/perps/src/core/spot/stableSelector.ts) |
| `ts_export` | `export type StableAmountMap = Readonly<Record<string, bigint>>;` | [apps/perps/src/core/spot/stableSelector.ts](../../../apps/perps/src/core/spot/stableSelector.ts) |
| `ts_export` | `export interface StableSelection` | [apps/perps/src/core/spot/stableSelector.ts](../../../apps/perps/src/core/spot/stableSelector.ts) |
| `ts_export` | `export function sumStableAmounts(map: StableAmountMap): bigint` | [apps/perps/src/core/spot/stableSelector.ts](../../../apps/perps/src/core/spot/stableSelector.ts) |
| `ts_export` | ``export function getConnectedAddress(): `0x$`` | [apps/perps/src/core/viem/clients.ts](../../../apps/perps/src/core/viem/clients.ts) |
| `ts_export` | `export function getPublicClient(): PublicClient` | [apps/perps/src/core/viem/clients.ts](../../../apps/perps/src/core/viem/clients.ts) |
| `ts_export` | ``export async function getWalletAccount(): Promise<`0x$`` | [apps/perps/src/core/viem/clients.ts](../../../apps/perps/src/core/viem/clients.ts) |
| `ts_export` | `export async function getWalletClient(): Promise<WalletClient \| null>` | [apps/perps/src/core/viem/clients.ts](../../../apps/perps/src/core/viem/clients.ts) |
| `ts_export` | `export const wagmiConfig = createConfig(` | [apps/perps/src/core/wagmi.ts](../../../apps/perps/src/core/wagmi.ts) |
| `ts_export` | `export function getAuthRedirectUrl(): string` | [apps/perps/src/core/wallet/embedded/client.ts](../../../apps/perps/src/core/wallet/embedded/client.ts) |
| `ts_export` | `export function getPaxeerWallet(): PaxeerWallet \| null` | [apps/perps/src/core/wallet/embedded/client.ts](../../../apps/perps/src/core/wallet/embedded/client.ts) |
| `ts_export` | `export interface ResolvedPaxeerConfig extends PaxeerWalletConfig` | [apps/perps/src/core/wallet/embedded/client.ts](../../../apps/perps/src/core/wallet/embedded/client.ts) |
| `ts_export` | `export function resolvePaxeerConfig(): ResolvedPaxeerConfig \| null` | [apps/perps/src/core/wallet/embedded/client.ts](../../../apps/perps/src/core/wallet/embedded/client.ts) |
| `ts_export` | `export function createPaxeerEIP1193Provider(` | [apps/perps/src/core/wallet/embedded/eip1193.ts](../../../apps/perps/src/core/wallet/embedded/eip1193.ts) |
| `ts_export` | `export type EIP1193Event =` | [apps/perps/src/core/wallet/embedded/eip1193.ts](../../../apps/perps/src/core/wallet/embedded/eip1193.ts) |
| `ts_export` | `export interface PaxeerEIP1193Provider` | [apps/perps/src/core/wallet/embedded/eip1193.ts](../../../apps/perps/src/core/wallet/embedded/eip1193.ts) |
| `ts_export` | `export interface PaxeerProviderOptions` | [apps/perps/src/core/wallet/embedded/eip1193.ts](../../../apps/perps/src/core/wallet/embedded/eip1193.ts) |
| `ts_export` | `export const PAXEER_CONNECTOR_ID = "paxeer-embedded";` | [apps/perps/src/core/wallet/embedded/paxeerConnector.ts](../../../apps/perps/src/core/wallet/embedded/paxeerConnector.ts) |
| `ts_export` | `export interface PaxeerConnectorParams` | [apps/perps/src/core/wallet/embedded/paxeerConnector.ts](../../../apps/perps/src/core/wallet/embedded/paxeerConnector.ts) |
| `ts_export` | `export function paxeerEmbeddedConnector(params: PaxeerConnectorParams)` | [apps/perps/src/core/wallet/embedded/paxeerConnector.ts](../../../apps/perps/src/core/wallet/embedded/paxeerConnector.ts) |
| `ts_export` | `export interface EmbeddedSignInResult` | [apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx](../../../apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx) |
| `ts_export` | `export function PaxeerProvider({ children }: PaxeerProviderProps)` | [apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx](../../../apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx) |
| `ts_export` | `export type SignInProvider = OAuthProvider;` | [apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx](../../../apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx) |
| `ts_export` | `export function useEmbeddedWalletAvailable(): boolean` | [apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx](../../../apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx) |
| `ts_export` | `export function usePaxeer(): PaxeerContextValue` | [apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx](../../../apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx) |
| `ts_export` | `export type OAuthProvider = "google" \| "apple" \| "twitter" \| "github" \| "discord";` | [apps/perps/src/core/wallet/embedded/sdk/index.ts](../../../apps/perps/src/core/wallet/embedded/sdk/index.ts) |
| `ts_export` | `export class PaxeerWallet` | [apps/perps/src/core/wallet/embedded/sdk/index.ts](../../../apps/perps/src/core/wallet/embedded/sdk/index.ts) |
| `ts_export` | `export function usePaxeerWallet(config: PaxeerWalletConfig): PaxeerWallet` | [apps/perps/src/core/wallet/embedded/sdk/react.ts](../../../apps/perps/src/core/wallet/embedded/sdk/react.ts) |
| `ts_export` | `export function useSession(wallet: PaxeerWallet): UseSessionResult` | [apps/perps/src/core/wallet/embedded/sdk/react.ts](../../../apps/perps/src/core/wallet/embedded/sdk/react.ts) |
| `ts_export` | `export interface UseSessionResult` | [apps/perps/src/core/wallet/embedded/sdk/react.ts](../../../apps/perps/src/core/wallet/embedded/sdk/react.ts) |
| `ts_export` | `export function useWallet(client: PaxeerWallet): UseWalletResult` | [apps/perps/src/core/wallet/embedded/sdk/react.ts](../../../apps/perps/src/core/wallet/embedded/sdk/react.ts) |
| `ts_export` | `export interface UseWalletResult` | [apps/perps/src/core/wallet/embedded/sdk/react.ts](../../../apps/perps/src/core/wallet/embedded/sdk/react.ts) |
| `ts_export` | `export interface ChainInfo` | [apps/perps/src/core/wallet/embedded/sdk/types.ts](../../../apps/perps/src/core/wallet/embedded/sdk/types.ts) |
| `ts_export` | `export interface PaxeerWalletConfig` | [apps/perps/src/core/wallet/embedded/sdk/types.ts](../../../apps/perps/src/core/wallet/embedded/sdk/types.ts) |
| `ts_export` | `export class PaxeerWalletError extends Error` | [apps/perps/src/core/wallet/embedded/sdk/types.ts](../../../apps/perps/src/core/wallet/embedded/sdk/types.ts) |
| `ts_export` | `export interface PublicWallet` | [apps/perps/src/core/wallet/embedded/sdk/types.ts](../../../apps/perps/src/core/wallet/embedded/sdk/types.ts) |
| `ts_export` | `export interface SendTxResponse` | [apps/perps/src/core/wallet/embedded/sdk/types.ts](../../../apps/perps/src/core/wallet/embedded/sdk/types.ts) |
| `ts_export` | `export interface SignMessageResponse` | [apps/perps/src/core/wallet/embedded/sdk/types.ts](../../../apps/perps/src/core/wallet/embedded/sdk/types.ts) |
| `ts_export` | `export interface SignTxResponse` | [apps/perps/src/core/wallet/embedded/sdk/types.ts](../../../apps/perps/src/core/wallet/embedded/sdk/types.ts) |
| `ts_export` | `export interface TxRequest` | [apps/perps/src/core/wallet/embedded/sdk/types.ts](../../../apps/perps/src/core/wallet/embedded/sdk/types.ts) |
| `ts_export` | `export function installPostMessageEip1193Provider()` | [apps/perps/src/core/wallet/postMessageEip1193.ts](../../../apps/perps/src/core/wallet/postMessageEip1193.ts) |
| `ts_export` | `export class PostMessageEip1193Provider` | [apps/perps/src/core/wallet/postMessageEip1193.ts](../../../apps/perps/src/core/wallet/postMessageEip1193.ts) |
| `ts_export` | `export function feeTierView(` | [apps/perps/src/core/perps/feeTier.ts](../../../apps/perps/src/core/perps/feeTier.ts) |
| `ts_export` | `export function catalogFromGateway(` | [apps/perps/src/core/perps/marketCatalog.ts](../../../apps/perps/src/core/perps/marketCatalog.ts) |
| `ts_export` | `export function decodeMarketConfig(` | [apps/perps/src/core/perps/marketConfig.ts](../../../apps/perps/src/core/perps/marketConfig.ts) |
| `ts_export` | `export function maintenanceMargin(notionalUsd: bigint, tiers: readonly MarginTier[]): bigint {` | [apps/perps/src/core/perps/marketConfig.ts](../../../apps/perps/src/core/perps/marketConfig.ts) |
| `ts_export` | `export function maxLeverageForNotional(notionalUsd: bigint, tiers: readonly MarginTier[]): bigint {` | [apps/perps/src/core/perps/marketConfig.ts](../../../apps/perps/src/core/perps/marketConfig.ts) |
| `ts_export` | `export function marketLabel(` | [apps/perps/src/core/perps/marketLabel.ts](../../../apps/perps/src/core/perps/marketLabel.ts) |
| `ts_export` | `export function markStatus(details: MarkPriceDetails, enabled: boolean): "open" \| "stale" \| "disabled" {` | [apps/perps/src/core/perps/markStatus.ts](../../../apps/perps/src/core/perps/markStatus.ts) |
| `ts_export` | `export function alignToTick(price: bigint, tickSize: bigint, side: "down" \| "up"): bigint {` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function checkOrderLimits(state: OrderFormState, limits: OrderLimits): OrderPlan \| null` | [apps/perps/src/core/perps/orderPlan.ts](../../../apps/perps/src/core/perps/orderPlan.ts) |
| `ts_export` | `export function tickerQuoteWithConfig(` | [apps/perps/src/core/perps/quoteFallback.ts](../../../apps/perps/src/core/perps/quoteFallback.ts) |
| `ts_export` | `export function buildIsTokenPaused(token: Address) {` | [apps/perps/src/core/spot/calls.ts](../../../apps/perps/src/core/spot/calls.ts) |
| `ts_export` | `export function spotTokenState(paused: boolean, info: VaultTokenInfo): SpotTokenState` | [apps/perps/src/core/spot/tokenState.ts](../../../apps/perps/src/core/spot/tokenState.ts) |
| `ts_export` | `export function registerVenueSymbols(symbols: readonly string[]): void` | [apps/perps/src/core/net/venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) |

## env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `NODE_ENV` | [apps/perps/src/core/log/logger.ts](../../../apps/perps/src/core/log/logger.ts) |
| `env_var` | `MARKET_PROXY_HOST` | [apps/perps/src/pages/api/candle/[...path].ts](../../../apps/perps/src/pages/api/candle/[...path].ts) |
| `env_var` | `MARKET_PROXY_ORIGIN` | [apps/perps/src/instrumentation.ts](../../../apps/perps/src/instrumentation.ts) |
| `env_var` | `NEXT_PUBLIC_AUTH_REDIRECT_URL` | [apps/perps/src/core/wallet/embedded/client.ts](../../../apps/perps/src/core/wallet/embedded/client.ts) |
| `env_var` | `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE` | [apps/perps/src/core/network/precompiles.ts](../../../apps/perps/src/core/network/precompiles.ts) |
| `env_var` | `NEXT_PUBLIC_LOG_LEVEL` | [apps/perps/src/core/log/logger.ts](../../../apps/perps/src/core/log/logger.ts) |
| `env_var` | `NEXT_PUBLIC_MARKET_PROXY_HOST` | [apps/perps/src/core/feeds.ts](../../../apps/perps/src/core/feeds.ts) |
| `env_var` | `NEXT_PUBLIC_PAXEER_RPC_URLS` | [apps/perps/src/core/rpc.ts](../../../apps/perps/src/core/rpc.ts) |
| `env_var` | `NEXT_PUBLIC_PAXEER_WALLET_API` | [apps/perps/src/core/wallet/embedded/client.ts](../../../apps/perps/src/core/wallet/embedded/client.ts) |
| `env_var` | `NEXT_PUBLIC_PAXSCAN_API_URL` | [apps/perps/src/core/paxscan/config.ts](../../../apps/perps/src/core/paxscan/config.ts) |
| `env_var` | `NEXT_PUBLIC_PAXSCAN_BASE_URL` | [apps/perps/src/core/paxscan/config.ts](../../../apps/perps/src/core/paxscan/config.ts) |
| `env_var` | `NEXT_PUBLIC_PAXSCAN_WS_URL` | [apps/perps/src/core/paxscan/config.ts](../../../apps/perps/src/core/paxscan/config.ts) |
| `env_var` | `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE` | [apps/perps/src/core/network/precompiles.ts](../../../apps/perps/src/core/network/precompiles.ts) |
| `env_var` | `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE` | [apps/perps/src/core/network/precompiles.ts](../../../apps/perps/src/core/network/precompiles.ts) |
| `env_var` | `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE` | [apps/perps/src/core/network/precompiles.ts](../../../apps/perps/src/core/network/precompiles.ts) |
| `env_var` | `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE` | [apps/perps/src/core/network/precompiles.ts](../../../apps/perps/src/core/network/precompiles.ts) |
| `env_var` | `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE` | [apps/perps/src/core/network/precompiles.ts](../../../apps/perps/src/core/network/precompiles.ts) |
| `env_var` | `NEXT_PUBLIC_PERPS_GATEWAY_URL` | [apps/perps/src/core/network/gateway.ts](../../../apps/perps/src/core/network/gateway.ts) |
| `env_var` | `NEXT_PUBLIC_PERPS_PRECOMPILE` | [apps/perps/src/core/network/precompiles.ts](../../../apps/perps/src/core/network/precompiles.ts) |
| `env_var` | `NEXT_PUBLIC_RELEASE_SPOT_OPEN` | [apps/perps/src/core/nav.ts](../../../apps/perps/src/core/nav.ts) |
| `env_var` | `NEXT_PUBLIC_STATUS_PAGE_URL` | [apps/perps/src/app/shell/Footer.tsx](../../../apps/perps/src/app/shell/Footer.tsx) |
| `env_var` | `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | [apps/perps/src/core/wallet/embedded/client.ts](../../../apps/perps/src/core/wallet/embedded/client.ts) |
| `env_var` | `NEXT_PUBLIC_SUPABASE_URL` | [apps/perps/src/core/wallet/embedded/client.ts](../../../apps/perps/src/core/wallet/embedded/client.ts) |
| `env_var` | `NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA` | [apps/perps/src/pages/_document.tsx](../../../apps/perps/src/pages/_document.tsx) |
| `env_var` | `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN` | [apps/perps/src/core/wallet/postMessageEip1193.ts](../../../apps/perps/src/core/wallet/postMessageEip1193.ts) |
| `env_var` | `NEXT_RUNTIME` | [apps/perps/src/instrumentation.ts](../../../apps/perps/src/instrumentation.ts) |
| `env_var` | `RAILWAY_GIT_COMMIT_SHA` | [apps/perps/src/pages/api/build-id.ts](../../../apps/perps/src/pages/api/build-id.ts) |
| `env_var` | `VERCEL_GIT_COMMIT_SHA` | [apps/perps/src/pages/api/build-id.ts](../../../apps/perps/src/pages/api/build-id.ts) |
| source | — | [apps/perps/src/env/.env.template](../../../apps/perps/src/env/.env.template) |

## hooks

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export interface EvmBlockStreamState` | [apps/perps/src/hooks/chain/useEvmBlockStream.ts](../../../apps/perps/src/hooks/chain/useEvmBlockStream.ts) |
| `ts_export` | `export function useEvmBlockStream( options: UseEvmBlockStreamOptions = {}, ): EvmBlockStreamState` | [apps/perps/src/hooks/chain/useEvmBlockStream.ts](../../../apps/perps/src/hooks/chain/useEvmBlockStream.ts) |
| `ts_export` | `export interface UseEvmBlockStreamOptions` | [apps/perps/src/hooks/chain/useEvmBlockStream.ts](../../../apps/perps/src/hooks/chain/useEvmBlockStream.ts) |
| `ts_export` | ``export const EXPLORER_WS_URL = `${FEED_ORIGIN.replace(/^http/, "ws")}/ws/explorer`;`` | [apps/perps/src/hooks/chain/useExplorerStream.ts](../../../apps/perps/src/hooks/chain/useExplorerStream.ts) |
| `ts_export` | `export interface ExplorerBlock` | [apps/perps/src/hooks/chain/useExplorerStream.ts](../../../apps/perps/src/hooks/chain/useExplorerStream.ts) |
| `ts_export` | `export interface ExplorerTx` | [apps/perps/src/hooks/chain/useExplorerStream.ts](../../../apps/perps/src/hooks/chain/useExplorerStream.ts) |
| `ts_export` | `export function useExplorerStream(): UseExplorerStreamResult` | [apps/perps/src/hooks/chain/useExplorerStream.ts](../../../apps/perps/src/hooks/chain/useExplorerStream.ts) |
| `ts_export` | `export interface UseExplorerStreamResult` | [apps/perps/src/hooks/chain/useExplorerStream.ts](../../../apps/perps/src/hooks/chain/useExplorerStream.ts) |
| `ts_export` | `export type PaxscanPageParam =` | [apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts](../../../apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts) |
| `ts_export` | `export function useBlockscoutInfiniteQuery<R extends PaginatedResourceName>( resource: R, opts: UseBlockscoutInfiniteQueryOptions<R> = {}, ): UseInfiniteQueryResult<InfiniteData<ResourcePayload<R>>, PaxscanError>` | [apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts](../../../apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts) |
| `ts_export` | `export interface UseBlockscoutInfiniteQueryOptions<R extends PaginatedResourceName>` | [apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts](../../../apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts) |
| `ts_export` | `export function useBlockscoutQuery<R extends ResourceName>( resource: R, opts: UseBlockscoutQueryOptions<R> = {}, ): UseQueryResult<ResourcePayload<R>, PaxscanError>` | [apps/perps/src/hooks/paxscan/useBlockscoutQuery.ts](../../../apps/perps/src/hooks/paxscan/useBlockscoutQuery.ts) |
| `ts_export` | `export interface UseBlockscoutQueryOptions<R extends ResourceName>` | [apps/perps/src/hooks/paxscan/useBlockscoutQuery.ts](../../../apps/perps/src/hooks/paxscan/useBlockscoutQuery.ts) |
| `ts_export` | `export async function deposit(` | [apps/perps/src/hooks/perps/useAccount.ts](../../../apps/perps/src/hooks/perps/useAccount.ts) |
| `ts_export` | `export type DepositStep = "approving" \| "executing";` | [apps/perps/src/hooks/perps/useAccount.ts](../../../apps/perps/src/hooks/perps/useAccount.ts) |
| `ts_export` | `export async function getAccount(account: Address): Promise<AccountView>` | [apps/perps/src/hooks/perps/useAccount.ts](../../../apps/perps/src/hooks/perps/useAccount.ts) |
| `ts_export` | `export async function getAllowance(account: Address): Promise<bigint>` | [apps/perps/src/hooks/perps/useAccount.ts](../../../apps/perps/src/hooks/perps/useAccount.ts) |
| `ts_export` | `export async function getWalletBalance(account: Address): Promise<bigint>` | [apps/perps/src/hooks/perps/useAccount.ts](../../../apps/perps/src/hooks/perps/useAccount.ts) |
| `ts_export` | `export function useAccount()` | [apps/perps/src/hooks/perps/useAccount.ts](../../../apps/perps/src/hooks/perps/useAccount.ts) |
| `ts_export` | `export async function withdraw(amount: bigint, account: Address, chainId: number = paxeerChain.id): Promise<WaitForTxResult> {` | [apps/perps/src/hooks/perps/useAccount.ts](../../../apps/perps/src/hooks/perps/useAccount.ts) |
| `ts_export` | `export async function withdrawAll(account: Address, chainId: number = paxeerChain.id): Promise<WaitForTxResult> {` | [apps/perps/src/hooks/perps/useAccount.ts](../../../apps/perps/src/hooks/perps/useAccount.ts) |
| `ts_export` | `export function useMarketSnapshots(marketId: number \| undefined, limit = 100)` | [apps/perps/src/hooks/perps/useAnalytics.ts](../../../apps/perps/src/hooks/perps/useAnalytics.ts) |
| `ts_export` | `export function useMarkPriceHistory(marketId: number \| undefined, limit = 100)` | [apps/perps/src/hooks/perps/useAnalytics.ts](../../../apps/perps/src/hooks/perps/useAnalytics.ts) |
| `ts_export` | `export function useOiSnapshots(marketId: number \| undefined, limit = 100)` | [apps/perps/src/hooks/perps/useAnalytics.ts](../../../apps/perps/src/hooks/perps/useAnalytics.ts) |
| `ts_export` | `export function useProtocolSnapshots(limit = 100)` | [apps/perps/src/hooks/perps/useAnalytics.ts](../../../apps/perps/src/hooks/perps/useAnalytics.ts) |
| `ts_export` | `export function useTradeSettlements(params?: { userAddress?: string; marketId?: number; positionId?: string; tradeType?: number; limit?: number; offset?: number; })` | [apps/perps/src/hooks/perps/useAnalytics.ts](../../../apps/perps/src/hooks/perps/useAnalytics.ts) |
| `ts_export` | `export function useVaultBalanceHistory( tokenAddress?: string, vaultType?: number, limit = 100, )` | [apps/perps/src/hooks/perps/useAnalytics.ts](../../../apps/perps/src/hooks/perps/useAnalytics.ts) |
| `ts_export` | `export interface AutoCollateralResult` | [apps/perps/src/hooks/perps/useAutoCollateral.ts](../../../apps/perps/src/hooks/perps/useAutoCollateral.ts) |
| `ts_export` | `export function useAutoCollateral(): AutoCollateralResult` | [apps/perps/src/hooks/perps/useAutoCollateral.ts](../../../apps/perps/src/hooks/perps/useAutoCollateral.ts) |
| `ts_export` | `export async function readContract(params: { address: Address; abi: readonly unknown[]; functionName: string; args?: readonly unknown[]; })` | [apps/perps/src/hooks/perps/useContract.ts](../../../apps/perps/src/hooks/perps/useContract.ts) |
| `ts_export` | `export function waitForTx(hash: Hash): Promise<WaitForTxResult> {` | [apps/perps/src/hooks/perps/useContract.ts](../../../apps/perps/src/hooks/perps/useContract.ts) |
| `ts_export` | `export interface WaitForTxResult` | [apps/perps/src/hooks/perps/useContract.ts](../../../apps/perps/src/hooks/perps/useContract.ts) |
| `ts_export` | ``export async function writeContract(params: { address: Address; abi: readonly unknown[]; functionName: string; args?: readonly unknown[]; value?: bigint; }): Promise<`0x$`` | [apps/perps/src/hooks/perps/useContract.ts](../../../apps/perps/src/hooks/perps/useContract.ts) |
| `ts_export` | `export interface DailyPnl` | [apps/perps/src/hooks/perps/useDailyPnl.ts](../../../apps/perps/src/hooks/perps/useDailyPnl.ts) |
| `ts_export` | `export interface DailyPnlResult` | [apps/perps/src/hooks/perps/useDailyPnl.ts](../../../apps/perps/src/hooks/perps/useDailyPnl.ts) |
| `ts_export` | `export async function fetchDailyPnl(addr: string): Promise<DailyPnlResult>` | [apps/perps/src/hooks/perps/useDailyPnl.ts](../../../apps/perps/src/hooks/perps/useDailyPnl.ts) |
| `ts_export` | `export function useDailyPnl(address: string \| undefined):` | [apps/perps/src/hooks/perps/useDailyPnl.ts](../../../apps/perps/src/hooks/perps/useDailyPnl.ts) |
| `ts_export` | `export function useEnrichedGlobalStats()` | [apps/perps/src/hooks/perps/useEnrichedStats.ts](../../../apps/perps/src/hooks/perps/useEnrichedStats.ts) |
| `ts_export` | `export function useEnrichedMarketStats(marketId: number \| undefined)` | [apps/perps/src/hooks/perps/useEnrichedStats.ts](../../../apps/perps/src/hooks/perps/useEnrichedStats.ts) |
| `ts_export` | `export function useOrderlyFundingRates()` | [apps/perps/src/hooks/perps/useEnrichedStats.ts](../../../apps/perps/src/hooks/perps/useEnrichedStats.ts) |
| `ts_export` | `export function useOrderlyOpenInterests()` | [apps/perps/src/hooks/perps/useEnrichedStats.ts](../../../apps/perps/src/hooks/perps/useEnrichedStats.ts) |
| `ts_export` | `export function useOrderlyPriceChanges()` | [apps/perps/src/hooks/perps/useEnrichedStats.ts](../../../apps/perps/src/hooks/perps/useEnrichedStats.ts) |
| `ts_export` | `export function useOrderlyTickers()` | [apps/perps/src/hooks/perps/useEnrichedStats.ts](../../../apps/perps/src/hooks/perps/useEnrichedStats.ts) |
| `ts_export` | `export function useFees(params?: { userAddress?: string; marketId?: number; positionId?: string; feeType?: number; limit?: number; offset?: number; })` | [apps/perps/src/hooks/perps/useFees.ts](../../../apps/perps/src/hooks/perps/useFees.ts) |
| `ts_export` | `export function useFundingPayments(params?: { userAddress?: string; marketId?: number; positionId?: string; limit?: number; offset?: number; })` | [apps/perps/src/hooks/perps/useFundingPayments.ts](../../../apps/perps/src/hooks/perps/useFundingPayments.ts) |
| `ts_export` | `export interface SharedPolledQueryOptions<TRaw, TPicked>` | [apps/perps/src/hooks/perps/useGraphQLQuery.ts](../../../apps/perps/src/hooks/perps/useGraphQLQuery.ts) |
| `ts_export` | `export interface SharedPolledQueryResult<T>` | [apps/perps/src/hooks/perps/useGraphQLQuery.ts](../../../apps/perps/src/hooks/perps/useGraphQLQuery.ts) |
| `ts_export` | `export function useSharedPolledQuery<TRaw = unknown, TPicked = TRaw>( opts: SharedPolledQueryOptions<TRaw, TPicked>, ): SharedPolledQueryResult<TPicked>` | [apps/perps/src/hooks/perps/useGraphQLQuery.ts](../../../apps/perps/src/hooks/perps/useGraphQLQuery.ts) |
| `ts_export` | `export interface IndexerLiquidation` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export interface IndexerOrder` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export interface IndexerPosition` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export interface IndexerPriceUpdate` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export interface IndexerStatus` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export interface IndexerTrade` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export function useIndexerStatus()` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export function useLatestPricesSubscription()` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export function useLiquidationSubscription(marketId: number \| undefined)` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export function useOrderChangedSubscription(userAddress: string \| undefined)` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export function usePositionChangedSubscription(` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export function useSingleMarketPriceSubscription(marketId: number \| undefined)` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export function useTradeCreatedSubscription(marketId: number \| undefined)` | [apps/perps/src/hooks/perps/useIndexerSubscriptions.ts](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) |
| `ts_export` | `export function useAccountSummary(` | [apps/perps/src/hooks/perps/useMarginMode.ts](../../../apps/perps/src/hooks/perps/useMarginMode.ts) |
| `ts_export` | `export function usePositionMargin(` | [apps/perps/src/hooks/perps/useMarginMode.ts](../../../apps/perps/src/hooks/perps/useMarginMode.ts) |
| `ts_export` | ``export function useTopUpPosition(userAddress: `0x${string}` \| undefined):`` | [apps/perps/src/hooks/perps/useMarginMode.ts](../../../apps/perps/src/hooks/perps/useMarginMode.ts) |
| `ts_export` | `export type ConnectionState = "idle" \| "connecting" \| "open" \| "closed";` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export function marketFundingHourly(symbol: string, field: string, v: unknown): bigint \| null` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | ``export const MARKETS_WS_URL = `${FEED_ORIGIN.replace(/^http/, "ws")}/ws/markets`;`` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export interface MarketSnapshot` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export interface MarketsTotals` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export function normalizeMarket(raw: unknown): MarketSnapshot \| null` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export function normalizeMarketsFrame(data: unknown[]): MarketSnapshot[]` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export function parseVenueFundingHourly(v: unknown): bigint \| null` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export function useMarketSnapshot(symbol: string \| undefined \| null): MarketSnapshot \| undefined` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export function useMarketsSnapshot(enabled = true): MarketSnapshot[]` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export function useMarketsStreamConnection(): ConnectionState` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export function useMarketsTotals(): MarketsTotals \| null` | [apps/perps/src/hooks/perps/useMarketsStream.ts](../../../apps/perps/src/hooks/perps/useMarketsStream.ts) |
| `ts_export` | `export interface DraftOrderLines` | [apps/perps/src/hooks/perps/useOrderLineStore.ts](../../../apps/perps/src/hooks/perps/useOrderLineStore.ts) |
| `ts_export` | `export const useOrderLineStore = create<DraftOrderLines>()((set) => (` | [apps/perps/src/hooks/perps/useOrderLineStore.ts](../../../apps/perps/src/hooks/perps/useOrderLineStore.ts) |
| `ts_export` | `export type AlertType =` | [apps/perps/src/hooks/perps/usePerfdbAlerts.ts](../../../apps/perps/src/hooks/perps/usePerfdbAlerts.ts) |
| `ts_export` | `export interface PerfdbAlert` | [apps/perps/src/hooks/perps/usePerfdbAlerts.ts](../../../apps/perps/src/hooks/perps/usePerfdbAlerts.ts) |
| `ts_export` | `export function usePerfdbAlerts(address: string \| undefined):` | [apps/perps/src/hooks/perps/usePerfdbAlerts.ts](../../../apps/perps/src/hooks/perps/usePerfdbAlerts.ts) |
| `ts_export` | `export function usePerfdbAuth(): UsePerfdbAuthReturn` | [apps/perps/src/hooks/perps/usePerfdbAuth.ts](../../../apps/perps/src/hooks/perps/usePerfdbAuth.ts) |
| `ts_export` | `export interface UsePerfdbAuthReturn extends AuthState` | [apps/perps/src/hooks/perps/usePerfdbAuth.ts](../../../apps/perps/src/hooks/perps/usePerfdbAuth.ts) |
| `ts_export` | `export interface PerfdbBalance` | [apps/perps/src/hooks/perps/usePerfdbBalances.ts](../../../apps/perps/src/hooks/perps/usePerfdbBalances.ts) |
| `ts_export` | `export function usePerfdbBalances(address: string \| undefined):` | [apps/perps/src/hooks/perps/usePerfdbBalances.ts](../../../apps/perps/src/hooks/perps/usePerfdbBalances.ts) |
| `ts_export` | `export interface Competition` | [apps/perps/src/hooks/perps/usePerfdbCompetitions.ts](../../../apps/perps/src/hooks/perps/usePerfdbCompetitions.ts) |
| `ts_export` | `export interface CompetitionDetail extends Competition` | [apps/perps/src/hooks/perps/usePerfdbCompetitions.ts](../../../apps/perps/src/hooks/perps/usePerfdbCompetitions.ts) |
| `ts_export` | `export type CompetitionMetric = "pnl" \| "volume" \| "trades" \| "roi" \| string;` | [apps/perps/src/hooks/perps/usePerfdbCompetitions.ts](../../../apps/perps/src/hooks/perps/usePerfdbCompetitions.ts) |
| `ts_export` | `export interface CompetitionRanking` | [apps/perps/src/hooks/perps/usePerfdbCompetitions.ts](../../../apps/perps/src/hooks/perps/usePerfdbCompetitions.ts) |
| `ts_export` | `export type CompetitionStatus = "upcoming" \| "active" \| "ended";` | [apps/perps/src/hooks/perps/usePerfdbCompetitions.ts](../../../apps/perps/src/hooks/perps/usePerfdbCompetitions.ts) |
| `ts_export` | `export function usePerfdbCompetitionDetail(id: string \| null):` | [apps/perps/src/hooks/perps/usePerfdbCompetitions.ts](../../../apps/perps/src/hooks/perps/usePerfdbCompetitions.ts) |
| `ts_export` | `export function usePerfdbCompetitions():` | [apps/perps/src/hooks/perps/usePerfdbCompetitions.ts](../../../apps/perps/src/hooks/perps/usePerfdbCompetitions.ts) |
| `ts_export` | `export interface ReferralTier` | [apps/perps/src/hooks/perps/usePerfdbReferrals.ts](../../../apps/perps/src/hooks/perps/usePerfdbReferrals.ts) |
| `ts_export` | `export function usePerfdbReferrals(address: string \| undefined):` | [apps/perps/src/hooks/perps/usePerfdbReferrals.ts](../../../apps/perps/src/hooks/perps/usePerfdbReferrals.ts) |
| `ts_export` | `export interface UserReferralData` | [apps/perps/src/hooks/perps/usePerfdbReferrals.ts](../../../apps/perps/src/hooks/perps/usePerfdbReferrals.ts) |
| `ts_export` | `export interface PriceUpdate` | [apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts](../../../apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts) |
| `ts_export` | `export interface TradeUpdate` | [apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts](../../../apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts) |
| `ts_export` | `export function usePriceSubscription(marketId?: number)` | [apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts](../../../apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts) |
| `ts_export` | `export function useTradeSubscription(marketId?: number)` | [apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts](../../../apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts) |
| `ts_export` | `export interface PerfdbTraderStats` | [apps/perps/src/hooks/perps/usePerfdbTraderStats.ts](../../../apps/perps/src/hooks/perps/usePerfdbTraderStats.ts) |
| `ts_export` | `export function usePerfdbTraderStats(address: string \| undefined):` | [apps/perps/src/hooks/perps/usePerfdbTraderStats.ts](../../../apps/perps/src/hooks/perps/usePerfdbTraderStats.ts) |
| `ts_export` | `export interface PerfdbUser` | [apps/perps/src/hooks/perps/usePerfdbUser.ts](../../../apps/perps/src/hooks/perps/usePerfdbUser.ts) |
| `ts_export` | `export function usePerfdbUser(address: string \| undefined):` | [apps/perps/src/hooks/perps/usePerfdbUser.ts](../../../apps/perps/src/hooks/perps/usePerfdbUser.ts) |
| `ts_export` | `export function usePerpsOrders()` | [apps/perps/src/hooks/perps/usePerpsOrders.ts](../../../apps/perps/src/hooks/perps/usePerpsOrders.ts) |
| `ts_export` | `export function usePerpsWallet()` | [apps/perps/src/hooks/perps/usePerpsWallet.ts](../../../apps/perps/src/hooks/perps/usePerpsWallet.ts) |
| `ts_export` | `export function markPositionLocallyClosed(positionId: string \| bigint)` | [apps/perps/src/hooks/perps/usePortfolioStats.ts](../../../apps/perps/src/hooks/perps/usePortfolioStats.ts) |
| `ts_export` | `export interface PortfolioStats` | [apps/perps/src/hooks/perps/usePortfolioStats.ts](../../../apps/perps/src/hooks/perps/usePortfolioStats.ts) |
| `ts_export` | `export function refreshPositions()` | [apps/perps/src/hooks/perps/usePortfolioStats.ts](../../../apps/perps/src/hooks/perps/usePortfolioStats.ts) |
| `ts_export` | `export function unmarkPositionLocallyClosed(positionId: string \| bigint)` | [apps/perps/src/hooks/perps/usePortfolioStats.ts](../../../apps/perps/src/hooks/perps/usePortfolioStats.ts) |
| `ts_export` | `export function usePortfolioStats(): PortfolioStats` | [apps/perps/src/hooks/perps/usePortfolioStats.ts](../../../apps/perps/src/hooks/perps/usePortfolioStats.ts) |
| `ts_export` | `export function perpsAllowanceSource(): AllowanceSource` | [apps/perps/src/hooks/perps/usePosition.ts](../../../apps/perps/src/hooks/perps/usePosition.ts) |
| `ts_export` | `export async function sendPlan(plan: WritePlan): Promise<WaitForTxResult>` | [apps/perps/src/hooks/perps/usePosition.ts](../../../apps/perps/src/hooks/perps/usePosition.ts) |
| `ts_export` | `export function usePosition()` | [apps/perps/src/hooks/perps/usePosition.ts](../../../apps/perps/src/hooks/perps/usePosition.ts) |
| `ts_export` | `export interface ClosePositionQuote` | [apps/perps/src/hooks/perps/useQuoter.ts](../../../apps/perps/src/hooks/perps/useQuoter.ts) |
| `ts_export` | `export type { MarketQuote, PartialCloseQuote };` | [apps/perps/src/hooks/perps/useQuoter.ts](../../../apps/perps/src/hooks/perps/useQuoter.ts) |
| `ts_export` | `export interface OpenPositionQuote` | [apps/perps/src/hooks/perps/useQuoter.ts](../../../apps/perps/src/hooks/perps/useQuoter.ts) |
| `ts_export` | `export interface QuoterMarketQuote extends MarketQuote` | [apps/perps/src/hooks/perps/useQuoter.ts](../../../apps/perps/src/hooks/perps/useQuoter.ts) |
| `ts_export` | `export function useQuoter(marketDataUrl = REST_API_URL) {` | [apps/perps/src/hooks/perps/useQuoter.ts](../../../apps/perps/src/hooks/perps/useQuoter.ts) |
| `ts_export` | `export function fetchTickers(): Promise<TickerMap>` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export interface RealtimePricesResult` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export interface TickerEntry` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export function tickerFromSnapshot(row: MarketSnapshot): TickerEntry` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export function tickerFromWire(row: TickerWire): TickerEntry` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export type TickerMap = Record<string, TickerEntry>;` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export function tickersFromWire(rows: TickerWire[]): TickerMap` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export type TickerWire = Omit<TickerEntry, "last_funding_rate_hourly"> & {` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export function useRealtimePrices({` | [apps/perps/src/hooks/perps/useRealtimePrices.ts](../../../apps/perps/src/hooks/perps/useRealtimePrices.ts) |
| `ts_export` | `export function hasDedicatedKlineStream(symbol: string \| undefined \| null): boolean` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export function hasDedicatedMarketStream(symbol: string \| undefined \| null): boolean` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export interface SidPaxBookLevel` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export interface SidPaxCandle` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export interface SidPaxKlineSubscriber` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export interface SidPaxOrderbook` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export interface SidPaxStats` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export interface SidPaxTrade` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export function subscribeKline( symbol: string \| undefined \| null, interval: string \| undefined \| null, subscriber: SidPaxKlineSubscriber, limit?: number, ): () => void` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export function subscribeOrderbook(symbol: string, cb: (book: SidPaxOrderbook \| null) => void): () => void` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export function tvResolutionToKlineInterval(resolution: string \| undefined \| null): string \| null` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export function useSidPaxOrderbook(symbol: string \| undefined): SidPaxOrderbook \| null` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export function useSidPaxStats(symbol: string \| undefined): SidPaxStats \| null` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export function useSidPaxTrades(symbol: string \| undefined): SidPaxTrade[]` | [apps/perps/src/hooks/perps/useSidPaxMarketStream.ts](../../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) |
| `ts_export` | `export const ACCOUNT_LEDGER_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ALL_ORDERS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ALL_ORDERS_V2_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const CLOSED_POSITIONS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const DASHBOARD_V2_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const DELEGATES_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ENRICHED_GLOBAL_STATS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ENRICHED_MARKET_STATS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const FEES_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const FILLED_ORDERS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const FUNDING_PAYMENTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const GLOBAL_STATS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const KEEPER_CYCLES_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const LATEST_PRICES_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const LEADERBOARD_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const LIQUIDATIONS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const MARK_PRICE_HISTORY_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const MARKET_INFO_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const MARKET_SNAPSHOTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const MARKETS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const OI_SNAPSHOTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ORDERLY_FUNDING_RATES_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ORDERLY_OPEN_INTERESTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ORDERLY_PRICE_CHANGES_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ORDERLY_TICKERS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const ORDERS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const POSITION_LEDGER_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const POSITIONS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const PROTOCOL_SNAPSHOTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const SPOT_COLLATERAL_MOVEMENTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const SPOT_MARKET_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const SPOT_MARKETS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const SPOT_ORDER_FILLS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const SPOT_ORDER_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const SPOT_ORDERS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export async function subgraphQuery<T = unknown>(` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const TPSL_ORDERS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const TRADE_SETTLEMENTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const TRADES_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const TRADING_ACCOUNT_EVENTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const USER_STATS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const USER_VAULT_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const VAULT_BALANCE_HISTORY_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export const VAULT_EVENTS_QUERY = /* GraphQL */ ` | [apps/perps/src/hooks/perps/useSubgraph.ts](../../../apps/perps/src/hooks/perps/useSubgraph.ts) |
| `ts_export` | `export interface PresenceSnapshot` | [apps/perps/src/hooks/presence/usePresence.ts](../../../apps/perps/src/hooks/presence/usePresence.ts) |
| `ts_export` | `export function usePresenceHeartbeat(market: string = "home")` | [apps/perps/src/hooks/presence/usePresence.ts](../../../apps/perps/src/hooks/presence/usePresence.ts) |
| `ts_export` | `export function usePresenceSnapshot(intervalMs: number = SNAPSHOT_INTERVAL_MS)` | [apps/perps/src/hooks/presence/usePresence.ts](../../../apps/perps/src/hooks/presence/usePresence.ts) |
| `ts_export` | `export async function approveExact(params: { token: Address; spender: Address; amount: bigint; }): Promise<WaitForTxResult>` | [apps/perps/src/hooks/spot/useSpotApprovals.ts](../../../apps/perps/src/hooks/spot/useSpotApprovals.ts) |
| `ts_export` | `export async function ensureAllowance(params: { token: ApprovalToken; owner: Address; spender: Address; amount: bigint; native?: NativeTokens; }): Promise<EnsureAllowanceResult>` | [apps/perps/src/hooks/spot/useSpotApprovals.ts](../../../apps/perps/src/hooks/spot/useSpotApprovals.ts) |
| `ts_export` | `export interface EnsureAllowanceResult` | [apps/perps/src/hooks/spot/useSpotApprovals.ts](../../../apps/perps/src/hooks/spot/useSpotApprovals.ts) |
| `ts_export` | `export async function getAllowance(params: { token: Address; owner: Address; spender: Address; }): Promise<bigint>` | [apps/perps/src/hooks/spot/useSpotApprovals.ts](../../../apps/perps/src/hooks/spot/useSpotApprovals.ts) |
| `ts_export` | `export const SPOT_NATIVE_TOKENS: NativeTokens = new Set<Address>(` | [apps/perps/src/hooks/spot/useSpotApprovals.ts](../../../apps/perps/src/hooks/spot/useSpotApprovals.ts) |
| `ts_export` | `export function useSpotApprovals()` | [apps/perps/src/hooks/spot/useSpotApprovals.ts](../../../apps/perps/src/hooks/spot/useSpotApprovals.ts) |
| `ts_export` | `export function buildSymbolWsUrl(symbol: string): string` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export function hasSpotMarketStream(symbol: string \| undefined \| null): boolean` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export interface SpotOrderbook` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export interface SpotStats` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export interface SpotTrade` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export function subscribeSpotOrderbook(symbol: string, cb: (book: SpotOrderbook \| null) => void): () => void` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export function useSpotOrderbook(symbol: string \| undefined): SpotOrderbook \| null` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export function useSpotStats(symbol: string \| undefined): SpotStats \| null` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export function useSpotTrades(symbol: string \| undefined): SpotTrade[]` | [apps/perps/src/hooks/spot/useSpotMarketStream.ts](../../../apps/perps/src/hooks/spot/useSpotMarketStream.ts) |
| `ts_export` | `export async function cancelLimitOrder(orderId: bigint): Promise<WaitForTxResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function cancelStopLimitOrder(orderId: bigint): Promise<WaitForTxResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function cancelStopLossOrder(orderId: bigint): Promise<WaitForTxResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function getLimitOrder(orderId: bigint)` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function getPaxPointer(): Promise<Address>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function getStopLimitOrder(orderId: bigint)` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function getStopLossOrder(orderId: bigint)` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function getUserLimitOrders(user: Address): Promise<readonly bigint[]>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function getUserStopLimitOrders(user: Address): Promise<readonly bigint[]>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function getUserStopLossOrders(user: Address): Promise<readonly bigint[]>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface LimitOrderResult extends SpotOrderResult` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function marketBuy(params: MarketBuyParams): Promise<MarketOrderResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface MarketBuyParams` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface MarketOrderResult extends SpotOrderResult` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function marketSell(params: MarketSellParams): Promise<MarketOrderResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface MarketSellParams` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function modifyLimitOrder(params: { orderId: bigint; newTargetPrice: bigint; newExpiresAt: bigint; }): Promise<WaitForTxResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function placeLimitBuy( params: PlaceLimitBuyParams, ): Promise<LimitOrderResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface PlaceLimitBuyParams` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function placeLimitSell( params: PlaceLimitSellParams, ): Promise<LimitOrderResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface PlaceLimitSellParams` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function placeStopLimitBuy( params: PlaceStopLimitBuyParams, ): Promise<StopLimitOrderResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface PlaceStopLimitBuyParams` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function placeStopLimitSell( params: PlaceStopLimitSellParams, ): Promise<StopLimitOrderResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface PlaceStopLimitSellParams` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export async function placeStopLoss( params: PlaceStopLossParams, ): Promise<StopLossOrderResult>` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface PlaceStopLossParams` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface SpotOrderResult` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export type StopLimitOrderResult = LimitOrderResult;` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export type StopLossOrderResult = LimitOrderResult;` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export function useSpotOrders()` | [apps/perps/src/hooks/spot/useSpotOrders.ts](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) |
| `ts_export` | `export interface SpotPortfolioState` | [apps/perps/src/hooks/spot/useSpotPortfolio.ts](../../../apps/perps/src/hooks/spot/useSpotPortfolio.ts) |
| `ts_export` | `export function useSpotPortfolio(` | [apps/perps/src/hooks/spot/useSpotPortfolio.ts](../../../apps/perps/src/hooks/spot/useSpotPortfolio.ts) |
| `ts_export` | `export async function batchQuote( requests: ReadonlyArray<BatchQuoteRequest>, ): Promise<SpotQuoteResult[]>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export interface BatchQuoteRequest` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export async function getAllLiquidityInfo(): Promise<SpotAllLiquidityRow[]>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export async function getLiquidityInfo(token: Address): Promise<SpotLiquidityInfo>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export async function getTokenPrice(token: Address): Promise<SpotTokenPrice>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export async function getTWAP(token: Address, period: bigint): Promise<bigint>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export async function quoteExactIn(params: { tokenIn: Address; tokenOut: Address; amountIn: bigint; }): Promise<SpotQuoteResult>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export async function quoteExactOut(params: { tokenIn: Address; tokenOut: Address; amountOut: bigint; }): Promise<SpotQuoteResult>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export async function quoteMarketBuy(params: { stablecoin: Address; token: Address; stablecoinAmount: bigint; }): Promise<SpotQuoteResult>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export async function quoteMarketSell(params: { token: Address; stablecoin: Address; tokenAmount: bigint; }): Promise<SpotQuoteResult>` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export interface SpotAllLiquidityRow` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export interface SpotLiquidityInfo` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export interface SpotQuoteResult` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export interface SpotTokenPrice` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export function useSpotQuoter()` | [apps/perps/src/hooks/spot/useSpotQuoter.ts](../../../apps/perps/src/hooks/spot/useSpotQuoter.ts) |
| `ts_export` | `export const SPOT_USER_WS_URL = gatewayUrls().spotUserWsUrl;` | [apps/perps/src/hooks/spot/useSpotUserStream.ts](../../../apps/perps/src/hooks/spot/useSpotUserStream.ts) |
| `ts_export` | `export type SpotUserConnectionState = "idle" \| "connecting" \| "open" \| "closed";` | [apps/perps/src/hooks/spot/useSpotUserStream.ts](../../../apps/perps/src/hooks/spot/useSpotUserStream.ts) |
| `ts_export` | `export type SpotUserFrame = Record<string, unknown>;` | [apps/perps/src/hooks/spot/useSpotUserStream.ts](../../../apps/perps/src/hooks/spot/useSpotUserStream.ts) |
| `ts_export` | `export type SpotUserStreamName =` | [apps/perps/src/hooks/spot/useSpotUserStream.ts](../../../apps/perps/src/hooks/spot/useSpotUserStream.ts) |
| `ts_export` | `export function subscribeSpotUserStream( stream: SpotUserStreamName, address: string, handler: FrameHandler, ): () => void` | [apps/perps/src/hooks/spot/useSpotUserStream.ts](../../../apps/perps/src/hooks/spot/useSpotUserStream.ts) |
| `ts_export` | `export function useSpotUserConnection(): SpotUserConnectionState` | [apps/perps/src/hooks/spot/useSpotUserStream.ts](../../../apps/perps/src/hooks/spot/useSpotUserStream.ts) |
| `ts_export` | `export function useSpotUserStream( stream: SpotUserStreamName, address: string \| null \| undefined, ): SpotUserFrame \| null` | [apps/perps/src/hooks/spot/useSpotUserStream.ts](../../../apps/perps/src/hooks/spot/useSpotUserStream.ts) |
| `ts_export` | `export function useSpotUserStreams( streams: SpotUserStreamName[], address: string \| null \| undefined, ): Partial<Record<SpotUserStreamName, SpotUserFrame>>` | [apps/perps/src/hooks/spot/useSpotUserStream.ts](../../../apps/perps/src/hooks/spot/useSpotUserStream.ts) |
| `ts_export` | `export interface CookieConsent` | [apps/perps/src/hooks/ui/useCookieConsent.ts](../../../apps/perps/src/hooks/ui/useCookieConsent.ts) |
| `ts_export` | `export const cookieConsentStore = create<CookieConsentStore>()((set, get) => (` | [apps/perps/src/hooks/ui/useCookieConsent.ts](../../../apps/perps/src/hooks/ui/useCookieConsent.ts) |
| `ts_export` | `export type CookieConsentValue = "accepted" \| "declined" \| null;` | [apps/perps/src/hooks/ui/useCookieConsent.ts](../../../apps/perps/src/hooks/ui/useCookieConsent.ts) |
| `ts_export` | `export function useCookieConsent(): CookieConsent` | [apps/perps/src/hooks/ui/useCookieConsent.ts](../../../apps/perps/src/hooks/ui/useCookieConsent.ts) |
| `ts_export` | `export interface FavoriteMarketsApi` | [apps/perps/src/hooks/ui/useFavoriteMarkets.ts](../../../apps/perps/src/hooks/ui/useFavoriteMarkets.ts) |
| `ts_export` | `export function useFavoriteMarkets(): FavoriteMarketsApi` | [apps/perps/src/hooks/ui/useFavoriteMarkets.ts](../../../apps/perps/src/hooks/ui/useFavoriteMarkets.ts) |
| `ts_export` | `export interface LayoutPreset` | [apps/perps/src/hooks/ui/useLayoutPresets.ts](../../../apps/perps/src/hooks/ui/useLayoutPresets.ts) |
| `ts_export` | `export function useLayoutPresets()` | [apps/perps/src/hooks/ui/useLayoutPresets.ts](../../../apps/perps/src/hooks/ui/useLayoutPresets.ts) |
| `ts_export` | `export function useIsMobile(): boolean` | [apps/perps/src/hooks/ui/useMobile.tsx](../../../apps/perps/src/hooks/ui/useMobile.tsx) |
| `ts_export` | `export const useSidebar = create<SidebarStore>()(` | [apps/perps/src/hooks/ui/useSidebar.ts](../../../apps/perps/src/hooks/ui/useSidebar.ts) |
| `ts_export` | `export interface ErrorHandlerApi` | [apps/perps/src/hooks/useErrorHandler.ts](../../../apps/perps/src/hooks/useErrorHandler.ts) |
| `ts_export` | `export function useErrorHandler(defaults: UseErrorHandlerOptions = {}): ErrorHandlerApi` | [apps/perps/src/hooks/useErrorHandler.ts](../../../apps/perps/src/hooks/useErrorHandler.ts) |
| `ts_export` | `export interface UseErrorHandlerOptions` | [apps/perps/src/hooks/useErrorHandler.ts](../../../apps/perps/src/hooks/useErrorHandler.ts) |
| `ts_export` | `export function useFeeTier(address: string \| undefined) {` | [apps/perps/src/hooks/perps/useFeeTier.ts](../../../apps/perps/src/hooks/perps/useFeeTier.ts) |
| `ts_export` | `export function useMarketConfig(marketId: bigint) {` | [apps/perps/src/hooks/perps/useMarketConfig.ts](../../../apps/perps/src/hooks/perps/useMarketConfig.ts) |
| `ts_export` | `export function usePerpsMarkets() {` | [apps/perps/src/hooks/perps/usePerpsMarkets.ts](../../../apps/perps/src/hooks/perps/usePerpsMarkets.ts) |
| `ts_export` | `export function useSpotTokenState(` | [apps/perps/src/hooks/spot/useSpotTokenState.ts](../../../apps/perps/src/hooks/spot/useSpotTokenState.ts) |
| `ts_export` | `export const useWalletStore = create<WalletState>()(` | [apps/perps/src/hooks/wallet/useWalletStore.ts](../../../apps/perps/src/hooks/wallet/useWalletStore.ts) |

## instrumentation

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export async function register(): Promise<void>` | [apps/perps/src/instrumentation.ts](../../../apps/perps/src/instrumentation.ts) |

## page-modules

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export default function AnalyticsPage()` | [apps/perps/src/page-modules/analytics/index.tsx](../../../apps/perps/src/page-modules/analytics/index.tsx) |
| `ts_export` | `export default function BlockRedirectShell()` | [apps/perps/src/page-modules/block-detail/index.tsx](../../../apps/perps/src/page-modules/block-detail/index.tsx) |
| `ts_export` | `export const getServerSideProps: GetServerSideProps = async (context) =>` | [apps/perps/src/page-modules/block-detail/index.tsx](../../../apps/perps/src/page-modules/block-detail/index.tsx) |
| `ts_export` | `export default function CompetitionsPage()` | [apps/perps/src/page-modules/competitions/index.tsx](../../../apps/perps/src/page-modules/competitions/index.tsx) |
| `ts_export` | `export default function ExplorerPage()` | [apps/perps/src/page-modules/explorer/index.tsx](../../../apps/perps/src/page-modules/explorer/index.tsx) |
| `ts_export` | `export default function HomePage()` | [apps/perps/src/page-modules/home/index.tsx](../../../apps/perps/src/page-modules/home/index.tsx) |
| `ts_export` | `export function compareWei(a: string, b: string): number` | [apps/perps/src/page-modules/leaderboard/index.tsx](../../../apps/perps/src/page-modules/leaderboard/index.tsx) |
| `ts_export` | `export default function LeaderboardPage()` | [apps/perps/src/page-modules/leaderboard/index.tsx](../../../apps/perps/src/page-modules/leaderboard/index.tsx) |
| `ts_export` | `export function normalizeWeiStr(v: string \| null \| undefined): string` | [apps/perps/src/page-modules/leaderboard/index.tsx](../../../apps/perps/src/page-modules/leaderboard/index.tsx) |
| `ts_export` | `export function totalPnlWei(entries: readonly { totalRealizedPnl: string }[]): bigint` | [apps/perps/src/page-modules/leaderboard/index.tsx](../../../apps/perps/src/page-modules/leaderboard/index.tsx) |
| `ts_export` | `export default function CookiePolicy()` | [apps/perps/src/page-modules/legal/CookiePolicy.tsx](../../../apps/perps/src/page-modules/legal/CookiePolicy.tsx) |
| `ts_export` | `export function LegalLayout({ title, lastUpdated, children }: LegalLayoutProps)` | [apps/perps/src/page-modules/legal/LegalLayout.tsx](../../../apps/perps/src/page-modules/legal/LegalLayout.tsx) |
| `ts_export` | `export default function PrivacyPolicy()` | [apps/perps/src/page-modules/legal/PrivacyPolicy.tsx](../../../apps/perps/src/page-modules/legal/PrivacyPolicy.tsx) |
| `ts_export` | `export default function TermsOfService()` | [apps/perps/src/page-modules/legal/TermsOfService.tsx](../../../apps/perps/src/page-modules/legal/TermsOfService.tsx) |
| `ts_export` | `export default function MarketsPage()` | [apps/perps/src/page-modules/markets/index.tsx](../../../apps/perps/src/page-modules/markets/index.tsx) |
| `ts_export` | `export default function TradingInterface()` | [apps/perps/src/page-modules/perps/index.tsx](../../../apps/perps/src/page-modules/perps/index.tsx) |
| `ts_export` | `export default function PortfolioPage()` | [apps/perps/src/page-modules/portfolio/index.tsx](../../../apps/perps/src/page-modules/portfolio/index.tsx) |
| `ts_export` | `export function overviewDailyPnlProps(result: ReturnType<typeof useDailyPnl>)` | [apps/perps/src/page-modules/portfolio/index.tsx](../../../apps/perps/src/page-modules/portfolio/index.tsx) |
| `ts_export` | `export default function ReferralsPage()` | [apps/perps/src/page-modules/referrals/index.tsx](../../../apps/perps/src/page-modules/referrals/index.tsx) |
| `ts_export` | `export default function SpotTradingInterface()` | [apps/perps/src/page-modules/spot/index.tsx](../../../apps/perps/src/page-modules/spot/index.tsx) |
| `ts_export` | `export default function TxRedirectShell()` | [apps/perps/src/page-modules/tx-detail/index.tsx](../../../apps/perps/src/page-modules/tx-detail/index.tsx) |
| `ts_export` | `export const getServerSideProps: GetServerSideProps = async (context) =>` | [apps/perps/src/page-modules/tx-detail/index.tsx](../../../apps/perps/src/page-modules/tx-detail/index.tsx) |

## pages

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function parseMarketProxyOrigin(raw: string \| undefined): string` | [apps/perps/src/pages/api/candle/[...path].ts](../../../apps/perps/src/pages/api/candle/[...path].ts) |
| `ts_export` | `export default MyApp;` | [apps/perps/src/pages/_app.tsx](../../../apps/perps/src/pages/_app.tsx) |
| `ts_export` | `export default function Document()` | [apps/perps/src/pages/_document.tsx](../../../apps/perps/src/pages/_document.tsx) |
| `ts_export` | `export const getServerSideProps = async (` | [apps/perps/src/pages/spot.tsx](../../../apps/perps/src/pages/spot.tsx) |
| `ts_export` | `export function SpotPage({ symbol }: SpotPageProps)` | [apps/perps/src/pages/spot.tsx](../../../apps/perps/src/pages/spot.tsx) |
| `ts_export` | `export interface SpotPageProps` | [apps/perps/src/pages/spot.tsx](../../../apps/perps/src/pages/spot.tsx) |

## routes

| Kind | Declaration | Source |
|---|---|---|
| source | — | [apps/perps/src/pages/api/build-id.ts](../../../apps/perps/src/pages/api/build-id.ts) |
| source | — | [apps/perps/src/pages/index.tsx](../../../apps/perps/src/pages/index.tsx) |
| source | — | [apps/perps/src/pages/analytics.tsx](../../../apps/perps/src/pages/analytics.tsx) |
| source | — | [apps/perps/src/pages/auth/callback.tsx](../../../apps/perps/src/pages/auth/callback.tsx) |
| source | — | [apps/perps/src/pages/block/[height].tsx](../../../apps/perps/src/pages/block/[height].tsx) |
| source | — | [apps/perps/src/pages/competitions.tsx](../../../apps/perps/src/pages/competitions.tsx) |
| source | — | [apps/perps/src/pages/cookies.tsx](../../../apps/perps/src/pages/cookies.tsx) |
| source | — | [apps/perps/src/pages/explorer.tsx](../../../apps/perps/src/pages/explorer.tsx) |
| source | — | [apps/perps/src/pages/api/candle/[...path].ts](../../../apps/perps/src/pages/api/candle/[...path].ts) |
| source | — | [apps/perps/src/pages/leaderboard.tsx](../../../apps/perps/src/pages/leaderboard.tsx) |
| source | — | [apps/perps/src/pages/markets.tsx](../../../apps/perps/src/pages/markets.tsx) |
| source | — | [apps/perps/src/pages/perps.tsx](../../../apps/perps/src/pages/perps.tsx) |
| source | — | [apps/perps/src/pages/portfolio.tsx](../../../apps/perps/src/pages/portfolio.tsx) |
| source | — | [apps/perps/src/pages/api/presence.ts](../../../apps/perps/src/pages/api/presence.ts) |
| source | — | [apps/perps/src/pages/privacy.tsx](../../../apps/perps/src/pages/privacy.tsx) |
| source | — | [apps/perps/src/pages/referrals.tsx](../../../apps/perps/src/pages/referrals.tsx) |
| source | — | [apps/perps/src/pages/spot.tsx](../../../apps/perps/src/pages/spot.tsx) |
| source | — | [apps/perps/src/pages/terms.tsx](../../../apps/perps/src/pages/terms.tsx) |
| source | — | [apps/perps/src/pages/tx/[hash].tsx](../../../apps/perps/src/pages/tx/[hash].tsx) |

## types

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export interface Address extends UserTags` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressBlocksValidatedResponse` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressCeloParams` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressCoinBalanceHistoryChart =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressCoinBalanceHistoryItem` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressCoinBalanceHistoryResponse` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressCollection =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressCollectionsResponse` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressCounters` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressEpochRewardsItem =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressEpochRewardsResponse =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressFromToFilter = typeof AddressFromToFilterValues[number] \| undefined;` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export const AddressFromToFilterValues = [ 'from', 'to' ] as const;` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressInternalTxsResponse` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressMudRecord =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressMudRecords =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressMudRecordsFilter =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressMudRecordsItem =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressMudRecordsSorting =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressMudTableItem =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressMudTables =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressMudTablesFilter =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressNFT = TokenInstance &` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressNFTsResponse` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressNFTTokensFilter =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressTabsCounters =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressTokenBalance` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressTokenBalancesResponse = Array<AddressTokenBalance>;` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressTokensBalancesSocketMessage` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressTokensFilter =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressTokensResponse` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressTokenTransferFilters =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressTokenTransferResponse` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressTransactionsResponse` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressTxsFilters =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressWithdrawalsItem =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressWithdrawalsResponse =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressXStarResponse =` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export interface AddressZilliqaParams` | [apps/perps/src/types/paxscan/api/address.ts](../../../apps/perps/src/types/paxscan/api/address.ts) |
| `ts_export` | `export type AddressesItem = AddressParam & { transactions_count: string; coin_balance: string \| null };` | [apps/perps/src/types/paxscan/api/addresses.ts](../../../apps/perps/src/types/paxscan/api/addresses.ts) |
| `ts_export` | `export interface AddressesMetadataSearchFilters` | [apps/perps/src/types/paxscan/api/addresses.ts](../../../apps/perps/src/types/paxscan/api/addresses.ts) |
| `ts_export` | `export interface AddressesMetadataSearchResult` | [apps/perps/src/types/paxscan/api/addresses.ts](../../../apps/perps/src/types/paxscan/api/addresses.ts) |
| `ts_export` | `export type AddressesResponse =` | [apps/perps/src/types/paxscan/api/addresses.ts](../../../apps/perps/src/types/paxscan/api/addresses.ts) |
| `ts_export` | `export interface AddressMetadataInfo` | [apps/perps/src/types/paxscan/api/addressMetadata.ts](../../../apps/perps/src/types/paxscan/api/addressMetadata.ts) |
| `ts_export` | `export interface AddressMetadataTag` | [apps/perps/src/types/paxscan/api/addressMetadata.ts](../../../apps/perps/src/types/paxscan/api/addressMetadata.ts) |
| `ts_export` | `export interface AddressMetadataTagApi extends Omit<AddressMetadataTag, 'meta'>` | [apps/perps/src/types/paxscan/api/addressMetadata.ts](../../../apps/perps/src/types/paxscan/api/addressMetadata.ts) |
| `ts_export` | `export type AddressMetadataTagType = 'name' \| 'generic' \| 'classifier' \| 'information' \| 'note' \| 'protocol';` | [apps/perps/src/types/paxscan/api/addressMetadata.ts](../../../apps/perps/src/types/paxscan/api/addressMetadata.ts) |
| `ts_export` | `export interface PublicTagType` | [apps/perps/src/types/paxscan/api/addressMetadata.ts](../../../apps/perps/src/types/paxscan/api/addressMetadata.ts) |
| `ts_export` | `export interface PublicTagTypesResponse` | [apps/perps/src/types/paxscan/api/addressMetadata.ts](../../../apps/perps/src/types/paxscan/api/addressMetadata.ts) |
| `ts_export` | `export type AddressFilecoinParams =` | [apps/perps/src/types/paxscan/api/addressParams.ts](../../../apps/perps/src/types/paxscan/api/addressParams.ts) |
| `ts_export` | `export interface AddressImplementation` | [apps/perps/src/types/paxscan/api/addressParams.ts](../../../apps/perps/src/types/paxscan/api/addressParams.ts) |
| `ts_export` | `export type AddressParam = UserTags & AddressParamBasic;` | [apps/perps/src/types/paxscan/api/addressParams.ts](../../../apps/perps/src/types/paxscan/api/addressParams.ts) |
| `ts_export` | `export type AddressParamBasic =` | [apps/perps/src/types/paxscan/api/addressParams.ts](../../../apps/perps/src/types/paxscan/api/addressParams.ts) |
| `ts_export` | `export interface AddressTag` | [apps/perps/src/types/paxscan/api/addressParams.ts](../../../apps/perps/src/types/paxscan/api/addressParams.ts) |
| `ts_export` | `export type FilecoinActorType =` | [apps/perps/src/types/paxscan/api/addressParams.ts](../../../apps/perps/src/types/paxscan/api/addressParams.ts) |
| `ts_export` | `export interface UserTags` | [apps/perps/src/types/paxscan/api/addressParams.ts](../../../apps/perps/src/types/paxscan/api/addressParams.ts) |
| `ts_export` | `export interface WatchlistName` | [apps/perps/src/types/paxscan/api/addressParams.ts](../../../apps/perps/src/types/paxscan/api/addressParams.ts) |
| `ts_export` | `export interface Block` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface BlockBaseFeeCelo` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface BlockCountdownResponse` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface BlockFilters` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface BlockInternalTransactionsResponse` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface BlocksResponse` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface BlockTransactionsResponse` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export type BlockType = 'block' \| 'reorg' \| 'uncle';` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export type BlockWithdrawalsItem =` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export type BlockWithdrawalsResponse =` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface NewBlockSocketResponse` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface ZilliqaBlockData` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface ZilliqaNestedQuorumCertificate extends ZilliqaQuorumCertificate` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface ZilliqaQuorumCertificate` | [apps/perps/src/types/paxscan/api/block.ts](../../../apps/perps/src/types/paxscan/api/block.ts) |
| `ts_export` | `export interface DecodedInput` | [apps/perps/src/types/paxscan/api/decodedInput.ts](../../../apps/perps/src/types/paxscan/api/decodedInput.ts) |
| `ts_export` | `export interface DecodedInputParams` | [apps/perps/src/types/paxscan/api/decodedInput.ts](../../../apps/perps/src/types/paxscan/api/decodedInput.ts) |
| `ts_export` | `export interface EnsAddressLookupFilters` | [apps/perps/src/types/paxscan/api/ens.ts](../../../apps/perps/src/types/paxscan/api/ens.ts) |
| `ts_export` | `export interface EnsDomainLookupFilters` | [apps/perps/src/types/paxscan/api/ens.ts](../../../apps/perps/src/types/paxscan/api/ens.ts) |
| `ts_export` | `export type EnsDomainLookupFiltersOptions = Array<'resolved_to' \| 'owned_by' \| 'with_inactive'>;` | [apps/perps/src/types/paxscan/api/ens.ts](../../../apps/perps/src/types/paxscan/api/ens.ts) |
| `ts_export` | `export interface EnsLookupSorting` | [apps/perps/src/types/paxscan/api/ens.ts](../../../apps/perps/src/types/paxscan/api/ens.ts) |
| `ts_export` | `export interface Fee` | [apps/perps/src/types/paxscan/api/fee.ts](../../../apps/perps/src/types/paxscan/api/fee.ts) |
| `ts_export` | `export type IndexingStatus =` | [apps/perps/src/types/paxscan/api/indexingStatus.ts](../../../apps/perps/src/types/paxscan/api/indexingStatus.ts) |
| `ts_export` | `export type InternalTransaction = (` | [apps/perps/src/types/paxscan/api/internalTransaction.ts](../../../apps/perps/src/types/paxscan/api/internalTransaction.ts) |
| `ts_export` | `export interface InternalTransactionFilters` | [apps/perps/src/types/paxscan/api/internalTransaction.ts](../../../apps/perps/src/types/paxscan/api/internalTransaction.ts) |
| `ts_export` | `export interface InternalTransactionsResponse` | [apps/perps/src/types/paxscan/api/internalTransaction.ts](../../../apps/perps/src/types/paxscan/api/internalTransaction.ts) |
| `ts_export` | `export type TxInternalsType = 'call' \| 'delegatecall' \| 'staticcall' \| 'create' \| 'create2' \| 'selfdestruct' \| 'reward';` | [apps/perps/src/types/paxscan/api/internalTransaction.ts](../../../apps/perps/src/types/paxscan/api/internalTransaction.ts) |
| `ts_export` | `export interface Log` | [apps/perps/src/types/paxscan/api/log.ts](../../../apps/perps/src/types/paxscan/api/log.ts) |
| `ts_export` | `export interface LogsResponseAddress` | [apps/perps/src/types/paxscan/api/log.ts](../../../apps/perps/src/types/paxscan/api/log.ts) |
| `ts_export` | `export interface LogsResponseTx` | [apps/perps/src/types/paxscan/api/log.ts](../../../apps/perps/src/types/paxscan/api/log.ts) |
| `ts_export` | `export interface RawTrace` | [apps/perps/src/types/paxscan/api/rawTrace.ts](../../../apps/perps/src/types/paxscan/api/rawTrace.ts) |
| `ts_export` | `export type RawTracesResponse = Array<RawTrace>;` | [apps/perps/src/types/paxscan/api/rawTrace.ts](../../../apps/perps/src/types/paxscan/api/rawTrace.ts) |
| `ts_export` | `export interface Reward` | [apps/perps/src/types/paxscan/api/reward.ts](../../../apps/perps/src/types/paxscan/api/reward.ts) |
| `ts_export` | `export const SEARCH_RESULT_TYPES =` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchRedirectResult` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResult` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultAddressOrContract extends SearchResultAddressData` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultBlob` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultBlock` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultCluster extends SearchResultAddressData` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultDomain extends Omit<SearchResultAddressData, 'address_hash'>` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultFilters` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export type SearchResultItem =` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultLabel` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultMetadataTag extends SearchResultAddressData` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultTacOperation` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultToken` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultTx` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export type SearchResultType = typeof SEARCH_RESULT_TYPES[keyof typeof SEARCH_RESULT_TYPES];` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface SearchResultUserOp` | [apps/perps/src/types/paxscan/api/search.ts](../../../apps/perps/src/types/paxscan/api/search.ts) |
| `ts_export` | `export interface GasPriceInfo` | [apps/perps/src/types/paxscan/api/stats.ts](../../../apps/perps/src/types/paxscan/api/stats.ts) |
| `ts_export` | `export type GasPrices =` | [apps/perps/src/types/paxscan/api/stats.ts](../../../apps/perps/src/types/paxscan/api/stats.ts) |
| `ts_export` | `export type HomeStats =` | [apps/perps/src/types/paxscan/api/stats.ts](../../../apps/perps/src/types/paxscan/api/stats.ts) |
| `ts_export` | `export type NFTTokenType = 'ERC-721' \| 'ERC-1155' \| 'ERC-404';` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type ThumbnailSize = '60x60' \| '250x250' \| '500x500' \| 'original';` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export interface TokenCounters` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenHolder = TokenHolderERC20ERC721 \| TokenHolderERC1155;` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenHolderBase =` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenHolderERC1155 = TokenHolderBase &` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenHolderERC20ERC721 = TokenHolderBase;` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export interface TokenHolders` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenHoldersPagination =` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export interface TokenInfo` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export interface TokenInstance` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export interface TokenInstanceMetadataSocketMessage` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export interface TokenInstanceTransfersCount` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenInventoryFilters =` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenInventoryPagination =` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export interface TokenInventoryResponse` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenReputation = 'ok' \| 'scam';` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenType = string;` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export type TokenVerifiedInfo = unknown;` | [apps/perps/src/types/paxscan/api/token.ts](../../../apps/perps/src/types/paxscan/api/token.ts) |
| `ts_export` | `export interface TokenInstanceTransferPagination` | [apps/perps/src/types/paxscan/api/tokens.ts](../../../apps/perps/src/types/paxscan/api/tokens.ts) |
| `ts_export` | `export interface TokenInstanceTransferResponse` | [apps/perps/src/types/paxscan/api/tokens.ts](../../../apps/perps/src/types/paxscan/api/tokens.ts) |
| `ts_export` | `export type TokensBridgedFilters = { q: string; chain_ids: Array<string> \| undefined };` | [apps/perps/src/types/paxscan/api/tokens.ts](../../../apps/perps/src/types/paxscan/api/tokens.ts) |
| `ts_export` | `export type TokensFilters = { q: string; type: Array<TokenType> \| undefined };` | [apps/perps/src/types/paxscan/api/tokens.ts](../../../apps/perps/src/types/paxscan/api/tokens.ts) |
| `ts_export` | `export type TokensResponse =` | [apps/perps/src/types/paxscan/api/tokens.ts](../../../apps/perps/src/types/paxscan/api/tokens.ts) |
| `ts_export` | `export interface TokensSorting` | [apps/perps/src/types/paxscan/api/tokens.ts](../../../apps/perps/src/types/paxscan/api/tokens.ts) |
| `ts_export` | `export type TokensSortingField = TokensSorting['sort'];` | [apps/perps/src/types/paxscan/api/tokens.ts](../../../apps/perps/src/types/paxscan/api/tokens.ts) |
| `ts_export` | ``export type TokensSortingValue = `${ TokensSortingField }-${ TokensSorting['order'] }` \| 'default';`` | [apps/perps/src/types/paxscan/api/tokens.ts](../../../apps/perps/src/types/paxscan/api/tokens.ts) |
| `ts_export` | `export type Erc1155TotalPayload =` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export type Erc20TotalPayload =` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export type Erc404TotalPayload =` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export type Erc721TotalPayload =` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export type TokenTotal = Erc20TotalPayload \| Erc721TotalPayload \| Erc1155TotalPayload;` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export type TokenTransfer = (` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export interface TokenTransferFilters` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export type TokenTransferPagination =` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export interface TokenTransferResponse` | [apps/perps/src/types/paxscan/api/tokenTransfer.ts](../../../apps/perps/src/types/paxscan/api/tokenTransfer.ts) |
| `ts_export` | `export type Transaction =` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export type TransactionRevertReason =` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export type TransactionsResponse = TransactionsResponseValidated \| TransactionsResponsePending;` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export interface TransactionsResponsePending` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export interface TransactionsResponseValidated` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export interface TransactionsResponseWatchlist` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export interface TransactionsResponseWithBlobs` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export interface TransactionsSorting` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export type TransactionsSortingField = TransactionsSorting['sort'];` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | ``export type TransactionsSortingValue = `${ TransactionsSortingField }-${ TransactionsSorting['order'] }` \| 'default';`` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export interface TransactionsStats` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export type TransactionType = 'rootstock_remasc' \|` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export interface TxAuthorization` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export type TxsResponse = TransactionsResponseValidated \| TransactionsResponsePending \| BlockTransactionsResponse;` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export type WrappedTransactionFields = 'decoded_input' \| 'fee' \| 'gas_limit' \| 'gas_price' \| 'hash' \| 'max_fee_per_gas' \|` | [apps/perps/src/types/paxscan/api/transaction.ts](../../../apps/perps/src/types/paxscan/api/transaction.ts) |
| `ts_export` | `export type TxAction =` | [apps/perps/src/types/paxscan/api/txAction.ts](../../../apps/perps/src/types/paxscan/api/txAction.ts) |
| `ts_export` | `export interface TxActionGeneral` | [apps/perps/src/types/paxscan/api/txAction.ts](../../../apps/perps/src/types/paxscan/api/txAction.ts) |
| `ts_export` | `export interface TxActionNft` | [apps/perps/src/types/paxscan/api/txAction.ts](../../../apps/perps/src/types/paxscan/api/txAction.ts) |
| `ts_export` | `export type TxStateChange = (TxStateChangeCoin \| TxStateChangeToken) &` | [apps/perps/src/types/paxscan/api/txStateChanges.ts](../../../apps/perps/src/types/paxscan/api/txStateChanges.ts) |
| `ts_export` | `export interface TxStateChangeCoin` | [apps/perps/src/types/paxscan/api/txStateChanges.ts](../../../apps/perps/src/types/paxscan/api/txStateChanges.ts) |
| `ts_export` | `export type TxStateChanges =` | [apps/perps/src/types/paxscan/api/txStateChanges.ts](../../../apps/perps/src/types/paxscan/api/txStateChanges.ts) |
| `ts_export` | `export type TxStateChangeToken = TxStateChangeTokenErc20 \| TxStateChangeTokenErc721 \| TxStateChangeTokenErc1155;` | [apps/perps/src/types/paxscan/api/txStateChanges.ts](../../../apps/perps/src/types/paxscan/api/txStateChanges.ts) |
| `ts_export` | `export interface TxStateChangeTokenErc1155` | [apps/perps/src/types/paxscan/api/txStateChanges.ts](../../../apps/perps/src/types/paxscan/api/txStateChanges.ts) |
| `ts_export` | `export interface TxStateChangeTokenErc20` | [apps/perps/src/types/paxscan/api/txStateChanges.ts](../../../apps/perps/src/types/paxscan/api/txStateChanges.ts) |
| `ts_export` | `export interface TxStateChangeTokenErc404` | [apps/perps/src/types/paxscan/api/txStateChanges.ts](../../../apps/perps/src/types/paxscan/api/txStateChanges.ts) |
| `ts_export` | `export interface TxStateChangeTokenErc721` | [apps/perps/src/types/paxscan/api/txStateChanges.ts](../../../apps/perps/src/types/paxscan/api/txStateChanges.ts) |
| `ts_export` | `export interface AccountSummary` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface EnrichedGlobalStats` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface EnrichedMarketStats` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface Fee` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface FeeTierConfig` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface FundingPayment` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface GlobalStats` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export enum MarginMode` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface MarketSnapshotRecord` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface MarketStats` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface MarkPriceRecord` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface OiSnapshot` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface OrderlyFundingRate` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface OrderlyOpenInterest` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface OrderlyPriceChange` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface OrderlyTicker` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface OrderlyVolumeStats` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface PlaceSpotOrderArgs` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface PLVQuote` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface PLVQuoteArgs` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface PLVScore` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface PoFQScore` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export type PoFQTierIndex = 0 \| 1 \| 2 \| 3;` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface PositionMargin` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface ProtocolSnapshotRecord` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotBatchClearFailure` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotBatchClearing` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotCollateralMovement` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotEpochSettlement` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotFastSettlement` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotKeeperCycle` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotMarket` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotMarketModeChange` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotOrder` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotOrderFill` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotPLV` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SgSpotPofqUpdate` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SpotBookLevel` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SpotCollateralArgs` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SpotEpochInfo` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SpotMarket` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface SpotOrder` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export enum SpotOrderType` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export enum SpotSide` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface TopUpPositionArgs` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface TraderFeeTier` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface TradeSettlement` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface TransferMarginArgs` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |
| `ts_export` | `export interface VaultBalanceRecord` | [apps/perps/src/types/perps.ts](../../../apps/perps/src/types/perps.ts) |

## ui

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `const Alert = React.forwardRef<` | [apps/perps/src/ui/atoms/alert.tsx](../../../apps/perps/src/ui/atoms/alert.tsx) |
| `ts_export` | `const AlertDescription = React.forwardRef<` | [apps/perps/src/ui/atoms/alert.tsx](../../../apps/perps/src/ui/atoms/alert.tsx) |
| `ts_export` | `const AlertTitle = React.forwardRef<` | [apps/perps/src/ui/atoms/alert.tsx](../../../apps/perps/src/ui/atoms/alert.tsx) |
| `ts_export` | `const Skiper68 = () =>` | [apps/perps/src/ui/atoms/animated-input.tsx](../../../apps/perps/src/ui/atoms/animated-input.tsx) |
| `ts_export` | `function Badge({ className, variant, ...props }: BadgeProps)` | [apps/perps/src/ui/atoms/badge.tsx](../../../apps/perps/src/ui/atoms/badge.tsx) |
| `ts_export` | `export interface BadgeProps` | [apps/perps/src/ui/atoms/badge.tsx](../../../apps/perps/src/ui/atoms/badge.tsx) |
| `ts_export` | `const badgeVariants = cva(` | [apps/perps/src/ui/atoms/badge.tsx](../../../apps/perps/src/ui/atoms/badge.tsx) |
| `ts_export` | `const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(` | [apps/perps/src/ui/atoms/button.tsx](../../../apps/perps/src/ui/atoms/button.tsx) |
| `ts_export` | `export interface ButtonProps` | [apps/perps/src/ui/atoms/button.tsx](../../../apps/perps/src/ui/atoms/button.tsx) |
| `ts_export` | `const buttonVariants = cva(` | [apps/perps/src/ui/atoms/button.tsx](../../../apps/perps/src/ui/atoms/button.tsx) |
| `ts_export` | `const Card = React.forwardRef<` | [apps/perps/src/ui/atoms/card.tsx](../../../apps/perps/src/ui/atoms/card.tsx) |
| `ts_export` | `const CardContent = React.forwardRef<` | [apps/perps/src/ui/atoms/card.tsx](../../../apps/perps/src/ui/atoms/card.tsx) |
| `ts_export` | `const CardDescription = React.forwardRef<` | [apps/perps/src/ui/atoms/card.tsx](../../../apps/perps/src/ui/atoms/card.tsx) |
| `ts_export` | `const CardFooter = React.forwardRef<` | [apps/perps/src/ui/atoms/card.tsx](../../../apps/perps/src/ui/atoms/card.tsx) |
| `ts_export` | `const CardHeader = React.forwardRef<` | [apps/perps/src/ui/atoms/card.tsx](../../../apps/perps/src/ui/atoms/card.tsx) |
| `ts_export` | `const CardTitle = React.forwardRef<` | [apps/perps/src/ui/atoms/card.tsx](../../../apps/perps/src/ui/atoms/card.tsx) |
| `ts_export` | `const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(` | [apps/perps/src/ui/atoms/checkbox.tsx](../../../apps/perps/src/ui/atoms/checkbox.tsx) |
| `ts_export` | `const Collapsible = CollapsiblePrimitive.Root` | [apps/perps/src/ui/atoms/collapsible.tsx](../../../apps/perps/src/ui/atoms/collapsible.tsx) |
| `ts_export` | `const CollapsibleContent = CollapsiblePrimitive.CollapsibleContent` | [apps/perps/src/ui/atoms/collapsible.tsx](../../../apps/perps/src/ui/atoms/collapsible.tsx) |
| `ts_export` | `const CollapsibleTrigger = CollapsiblePrimitive.CollapsibleTrigger` | [apps/perps/src/ui/atoms/collapsible.tsx](../../../apps/perps/src/ui/atoms/collapsible.tsx) |
| `ts_export` | `export const IcoAi = icon("/assets/ai.svg", "AI");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoAlert = icon("/assets/alert.svg", "Alert");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoAlertLarge = icon("/assets/alert_large.svg", "AlertLarge");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoArrowsLeftRight = icon("/assets/ArrowsLeftRight.svg", "ArrowsLeftRight");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoAvatar = icon("/assets/avatar.svg", "Avatar");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoAvatarDark = icon("/assets/avatar_dark.svg", "AvatarDark");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoBuy = icon("/assets/buy.svg", "Buy");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoCircleChain = icon("/assets/circle_chain.svg", "CircleChain");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoClock = icon("/assets/clock.svg", "Clock");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoCopy = icon("/assets/copy.svg", "Copy");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoDiscord = icon("/assets/Discord.svg", "Discord");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoDocker = icon("/assets/docker.svg", "Docker");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoDocument = icon("/assets/Document.svg", "Document");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoDownMenu = icon("/assets/downmenu.svg", "DownMenu");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoDraggable = icon("/assets/draggable.svg", "Draggable");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoDropdown = icon("/assets/dropdown.svg", "Dropdown");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoDropdownWhite = icon("/assets/dropdown_white.svg", "DropdownWhite");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoExchange = icon("/assets/exchange.svg", "Exchange");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoExclamation = icon("/assets/exclamation.svg", "Exclamation");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoExit = icon("/assets/exit.svg", "Exit");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoFilter = icon("/assets/filter.svg", "Filter");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoFire = icon("/assets/fire.svg", "Fire");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoFlag = icon("/assets/flag.svg", "Flag");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoGas = icon("/assets/gas.svg", "Gas");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoGraph = icon("/assets/graph.svg", "Graph");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoLayer = icon("/assets/layer.svg", "Layer");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoLeave = icon("/assets/leave.svg", "Leave");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoLink = icon("/assets/link.svg", "Link");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoLinkBlue = icon("/assets/link_blue.svg", "LinkBlue");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoLinkCyan = icon("/assets/link_cyan.svg", "LinkCyan");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoLinkWhite = icon("/assets/link_white.svg", "LinkWhite");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoLock = icon("/assets/lock.svg", "Lock");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoMoney = icon("/assets/money.svg", "Money");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoNextArrow = icon("/assets/nextarrow.svg", "NextArrow");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoPaint = icon("/assets/Paint.svg", "Paint");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoPeople = icon("/assets/people.svg", "People");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoPlus = icon("/assets/plus.svg", "Plus");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoPlusWhite = icon("/assets/plus_white.svg", "PlusWhite");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoPremium = icon("/assets/premium.svg", "Premium");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoQuitWhite = icon("/assets/quit_white.svg", "QuitWhite");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoRun = icon("/assets/run.svg", "Run");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoSearch = icon("/assets/search.svg", "Search");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoSet = icon("/assets/set.svg", "Set");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoSettings = icon("/assets/setting.svg", "Settings");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoShare = icon("/assets/share.svg", "Share");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoShield = icon("/assets/case.svg", "Shield");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoSpeaker = icon("/assets/Speaker.svg", "Speaker");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoStar = icon("/assets/star.svg", "Star");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoTarget = icon("/assets/target.svg", "Target");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoTrash = icon("/assets/trash.svg", "Trash");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoVolume = icon("/assets/volume.svg", "Volume");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoWallet = icon("/assets/wallet_svgrepo.svg", "Wallet");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoWarn = icon("/assets/warn.svg", "Warn");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoWidget = icon("/assets/widget.svg", "Widget");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoWind = icon("/assets/wind.svg", "Wind");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoWorld = icon("/assets/world.svg", "World");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export const IcoX = icon("/assets/X.svg", "X");` | [apps/perps/src/ui/atoms/custom-icon.tsx](../../../apps/perps/src/ui/atoms/custom-icon.tsx) |
| `ts_export` | `export default function CustomLeverageSlider({ min = 1, max = 100, step = 1, defaultValue = 10, value, onChange, className, }: CustomLeverageSliderProps)` | [apps/perps/src/ui/atoms/custom-leverage-slider.tsx](../../../apps/perps/src/ui/atoms/custom-leverage-slider.tsx) |
| `ts_export` | `export default function CustomSlider({ min = 0, max = 100, step = 1, defaultValue = 0, value, onChange, className, }: CustomSliderProps)` | [apps/perps/src/ui/atoms/custom-slider.tsx](../../../apps/perps/src/ui/atoms/custom-slider.tsx) |
| `ts_export` | `const Dialog = DialogPrimitive.Root` | [apps/perps/src/ui/atoms/dialog.tsx](../../../apps/perps/src/ui/atoms/dialog.tsx) |
| `ts_export` | `const DialogContent = React.forwardRef<` | [apps/perps/src/ui/atoms/dialog.tsx](../../../apps/perps/src/ui/atoms/dialog.tsx) |
| `ts_export` | `const DialogDescription = React.forwardRef<` | [apps/perps/src/ui/atoms/dialog.tsx](../../../apps/perps/src/ui/atoms/dialog.tsx) |
| `ts_export` | `const DialogFooter = (` | [apps/perps/src/ui/atoms/dialog.tsx](../../../apps/perps/src/ui/atoms/dialog.tsx) |
| `ts_export` | `const DialogHeader = (` | [apps/perps/src/ui/atoms/dialog.tsx](../../../apps/perps/src/ui/atoms/dialog.tsx) |
| `ts_export` | `const DialogTitle = React.forwardRef<` | [apps/perps/src/ui/atoms/dialog.tsx](../../../apps/perps/src/ui/atoms/dialog.tsx) |
| `ts_export` | `const DialogTrigger = DialogPrimitive.Trigger` | [apps/perps/src/ui/atoms/dialog.tsx](../../../apps/perps/src/ui/atoms/dialog.tsx) |
| `ts_export` | `const DropdownMenu = DropdownMenuPrimitive.Root` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuCheckboxItem = React.forwardRef<` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuContent = React.forwardRef<` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuGroup = DropdownMenuPrimitive.Group` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuItem = React.forwardRef<` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuLabel = React.forwardRef<` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuPortal = DropdownMenuPrimitive.Portal` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuRadioGroup = DropdownMenuPrimitive.RadioGroup` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuRadioItem = React.forwardRef<` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuSeparator = React.forwardRef<` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuShortcut = (` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuSub = DropdownMenuPrimitive.Sub` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuSubContent = React.forwardRef<` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuSubTrigger = React.forwardRef<` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger` | [apps/perps/src/ui/atoms/dropdown-menu.tsx](../../../apps/perps/src/ui/atoms/dropdown-menu.tsx) |
| `ts_export` | `const GlassSurface = React.forwardRef<HTMLDivElement, GlassSurfaceProps>(` | [apps/perps/src/ui/atoms/glass-surface.tsx](../../../apps/perps/src/ui/atoms/glass-surface.tsx) |
| `ts_export` | `export interface GlassSurfaceProps extends React.HTMLAttributes<HTMLDivElement>` | [apps/perps/src/ui/atoms/glass-surface.tsx](../../../apps/perps/src/ui/atoms/glass-surface.tsx) |
| `ts_export` | `export function HyperPaxWordmark({ size = "md", textOnly = false, markOnly = false, color, className, }: HyperPaxWordmarkProps)` | [apps/perps/src/ui/atoms/HyperPaxWordmark.tsx](../../../apps/perps/src/ui/atoms/HyperPaxWordmark.tsx) |
| `ts_export` | `export interface HyperPaxWordmarkProps` | [apps/perps/src/ui/atoms/HyperPaxWordmark.tsx](../../../apps/perps/src/ui/atoms/HyperPaxWordmark.tsx) |
| `ts_export` | `const Input = React.forwardRef<HTMLInputElement, InputProps>(` | [apps/perps/src/ui/atoms/input.tsx](../../../apps/perps/src/ui/atoms/input.tsx) |
| `ts_export` | `export interface InputProps` | [apps/perps/src/ui/atoms/input.tsx](../../../apps/perps/src/ui/atoms/input.tsx) |
| `ts_export` | `const Label = React.forwardRef<` | [apps/perps/src/ui/atoms/label.tsx](../../../apps/perps/src/ui/atoms/label.tsx) |
| `ts_export` | `export function PaxscanLink({ kind, value, label, iconClassName, className, title, }: PaxscanLinkProps): React.ReactElement \| null` | [apps/perps/src/ui/atoms/PaxscanLink.tsx](../../../apps/perps/src/ui/atoms/PaxscanLink.tsx) |
| `ts_export` | `export type PaxscanLinkKind = "tx" \| "block" \| "address";` | [apps/perps/src/ui/atoms/PaxscanLink.tsx](../../../apps/perps/src/ui/atoms/PaxscanLink.tsx) |
| `ts_export` | `export interface PaxscanLinkProps` | [apps/perps/src/ui/atoms/PaxscanLink.tsx](../../../apps/perps/src/ui/atoms/PaxscanLink.tsx) |
| `ts_export` | `export function PaxSpinner({ className, style, speed = 0.6, cameraZ = 4, active = true, }: PaxSpinnerProps)` | [apps/perps/src/ui/atoms/PaxSpinner.tsx](../../../apps/perps/src/ui/atoms/PaxSpinner.tsx) |
| `ts_export` | `const Popover = PopoverPrimitive.Root` | [apps/perps/src/ui/atoms/popover.tsx](../../../apps/perps/src/ui/atoms/popover.tsx) |
| `ts_export` | `const PopoverAnchor = PopoverPrimitive.Anchor` | [apps/perps/src/ui/atoms/popover.tsx](../../../apps/perps/src/ui/atoms/popover.tsx) |
| `ts_export` | `const PopoverContent = React.forwardRef<` | [apps/perps/src/ui/atoms/popover.tsx](../../../apps/perps/src/ui/atoms/popover.tsx) |
| `ts_export` | `const PopoverTrigger = PopoverPrimitive.Trigger` | [apps/perps/src/ui/atoms/popover.tsx](../../../apps/perps/src/ui/atoms/popover.tsx) |
| `ts_export` | `export function PriceFlash({ value, children, className, as: Tag = "span", durationMs = 420, }: { value: number; children: React.ReactNode; className?: string; as?: keyof JSX.IntrinsicElements; durationMs?: number;` | [apps/perps/src/ui/atoms/PriceFlash.tsx](../../../apps/perps/src/ui/atoms/PriceFlash.tsx) |
| `ts_export` | `export function ResizeHandle({ direction, targetRef, cssProp = direction === "horizontal" ? "width" : "height", min = 0, max = 9999, onResizeEnd, invert = false, className, }: ResizeHandleProps)` | [apps/perps/src/ui/atoms/resize-handle.tsx](../../../apps/perps/src/ui/atoms/resize-handle.tsx) |
| `ts_export` | `export function SegmentedControl({ options, value, onValueChange, className, size = "md", }: SegmentedControlProps)` | [apps/perps/src/ui/atoms/segmented-control.tsx](../../../apps/perps/src/ui/atoms/segmented-control.tsx) |
| `ts_export` | `export interface SegmentedControlOption` | [apps/perps/src/ui/atoms/segmented-control.tsx](../../../apps/perps/src/ui/atoms/segmented-control.tsx) |
| `ts_export` | `export interface SegmentedControlProps` | [apps/perps/src/ui/atoms/segmented-control.tsx](../../../apps/perps/src/ui/atoms/segmented-control.tsx) |
| `ts_export` | `const Select = SelectPrimitive.Root` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectContent = React.forwardRef<` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectGroup = SelectPrimitive.Group` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectItem = React.forwardRef<` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectLabel = React.forwardRef<` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectScrollDownButton = React.forwardRef<` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectScrollUpButton = React.forwardRef<` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectSeparator = React.forwardRef<` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectTrigger = React.forwardRef<` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const SelectValue = SelectPrimitive.Value` | [apps/perps/src/ui/atoms/select.tsx](../../../apps/perps/src/ui/atoms/select.tsx) |
| `ts_export` | `const Separator = React.forwardRef<` | [apps/perps/src/ui/atoms/separator.tsx](../../../apps/perps/src/ui/atoms/separator.tsx) |
| `ts_export` | `const Sheet = SheetPrimitive.Root` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetClose = SheetPrimitive.Close` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetContent = React.forwardRef<` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetDescription = React.forwardRef<` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetFooter = (` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetHeader = (` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetOverlay = React.forwardRef<` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetPortal = SheetPrimitive.Portal` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetTitle = React.forwardRef<` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const SheetTrigger = SheetPrimitive.Trigger` | [apps/perps/src/ui/atoms/sheet.tsx](../../../apps/perps/src/ui/atoms/sheet.tsx) |
| `ts_export` | `const Sidebar = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarContent = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarFooter = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarGroup = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarGroupAction = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarGroupContent = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarGroupLabel = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarHeader = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarInput = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarInset = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenu = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenuAction = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenuBadge = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenuButton = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenuItem = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenuSkeleton = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenuSub = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenuSubButton = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarMenuSubItem = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarProvider = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarRail = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarSeparator = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `const SidebarTrigger = React.forwardRef<` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `function useSidebar()` | [apps/perps/src/ui/atoms/sidebar.tsx](../../../apps/perps/src/ui/atoms/sidebar.tsx) |
| `ts_export` | `export function ChartSkeleton()` | [apps/perps/src/ui/atoms/skeleton-panel.tsx](../../../apps/perps/src/ui/atoms/skeleton-panel.tsx) |
| `ts_export` | `export function OrderbookSkeleton()` | [apps/perps/src/ui/atoms/skeleton-panel.tsx](../../../apps/perps/src/ui/atoms/skeleton-panel.tsx) |
| `ts_export` | `export function OrderFormSkeleton()` | [apps/perps/src/ui/atoms/skeleton-panel.tsx](../../../apps/perps/src/ui/atoms/skeleton-panel.tsx) |
| `ts_export` | `export function PositionsSkeleton()` | [apps/perps/src/ui/atoms/skeleton-panel.tsx](../../../apps/perps/src/ui/atoms/skeleton-panel.tsx) |
| `ts_export` | `function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>)` | [apps/perps/src/ui/atoms/skeleton.tsx](../../../apps/perps/src/ui/atoms/skeleton.tsx) |
| `ts_export` | `const Slider = React.forwardRef<` | [apps/perps/src/ui/atoms/slider.tsx](../../../apps/perps/src/ui/atoms/slider.tsx) |
| `ts_export` | `const Skiper63 = () =>` | [apps/perps/src/ui/atoms/SquiCircleFilter.tsx](../../../apps/perps/src/ui/atoms/SquiCircleFilter.tsx) |
| `ts_export` | `export const SquiCircleFilterStatic = () =>` | [apps/perps/src/ui/atoms/SquiCircleFilter.tsx](../../../apps/perps/src/ui/atoms/SquiCircleFilter.tsx) |
| `ts_export` | `const Switch = React.forwardRef<` | [apps/perps/src/ui/atoms/switch.tsx](../../../apps/perps/src/ui/atoms/switch.tsx) |
| `ts_export` | `const Table = React.forwardRef<` | [apps/perps/src/ui/atoms/table.tsx](../../../apps/perps/src/ui/atoms/table.tsx) |
| `ts_export` | `const TableBody = React.forwardRef<` | [apps/perps/src/ui/atoms/table.tsx](../../../apps/perps/src/ui/atoms/table.tsx) |
| `ts_export` | `const TableCaption = React.forwardRef<` | [apps/perps/src/ui/atoms/table.tsx](../../../apps/perps/src/ui/atoms/table.tsx) |
| `ts_export` | `const TableCell = React.forwardRef<` | [apps/perps/src/ui/atoms/table.tsx](../../../apps/perps/src/ui/atoms/table.tsx) |
| `ts_export` | `const TableFooter = React.forwardRef<` | [apps/perps/src/ui/atoms/table.tsx](../../../apps/perps/src/ui/atoms/table.tsx) |
| `ts_export` | `const TableHead = React.forwardRef<` | [apps/perps/src/ui/atoms/table.tsx](../../../apps/perps/src/ui/atoms/table.tsx) |
| `ts_export` | `const TableHeader = React.forwardRef<` | [apps/perps/src/ui/atoms/table.tsx](../../../apps/perps/src/ui/atoms/table.tsx) |
| `ts_export` | `const TableRow = React.forwardRef<` | [apps/perps/src/ui/atoms/table.tsx](../../../apps/perps/src/ui/atoms/table.tsx) |
| `ts_export` | `const Tabs = TabsPrimitive.Root` | [apps/perps/src/ui/atoms/tabs.tsx](../../../apps/perps/src/ui/atoms/tabs.tsx) |
| `ts_export` | `const TabsContent = React.forwardRef<` | [apps/perps/src/ui/atoms/tabs.tsx](../../../apps/perps/src/ui/atoms/tabs.tsx) |
| `ts_export` | `const TabsList = React.forwardRef<` | [apps/perps/src/ui/atoms/tabs.tsx](../../../apps/perps/src/ui/atoms/tabs.tsx) |
| `ts_export` | `const TabsTrigger = React.forwardRef<` | [apps/perps/src/ui/atoms/tabs.tsx](../../../apps/perps/src/ui/atoms/tabs.tsx) |
| `ts_export` | `const Toast = React.forwardRef<` | [apps/perps/src/ui/atoms/toast.tsx](../../../apps/perps/src/ui/atoms/toast.tsx) |
| `ts_export` | `export type ToastActionElement = React.ReactElement` | [apps/perps/src/ui/atoms/toast.tsx](../../../apps/perps/src/ui/atoms/toast.tsx) |
| `ts_export` | `const ToastClose = React.forwardRef<` | [apps/perps/src/ui/atoms/toast.tsx](../../../apps/perps/src/ui/atoms/toast.tsx) |
| `ts_export` | `const ToastDescription = React.forwardRef<` | [apps/perps/src/ui/atoms/toast.tsx](../../../apps/perps/src/ui/atoms/toast.tsx) |
| `ts_export` | `export type ToastProps = React.ComponentPropsWithoutRef<typeof ToastPrimitives.Root> &` | [apps/perps/src/ui/atoms/toast.tsx](../../../apps/perps/src/ui/atoms/toast.tsx) |
| `ts_export` | `const ToastProvider = ToastPrimitives.Provider` | [apps/perps/src/ui/atoms/toast.tsx](../../../apps/perps/src/ui/atoms/toast.tsx) |
| `ts_export` | `const ToastTitle = React.forwardRef<` | [apps/perps/src/ui/atoms/toast.tsx](../../../apps/perps/src/ui/atoms/toast.tsx) |
| `ts_export` | `const ToastViewport = React.forwardRef<` | [apps/perps/src/ui/atoms/toast.tsx](../../../apps/perps/src/ui/atoms/toast.tsx) |
| `ts_export` | `export function Toaster()` | [apps/perps/src/ui/atoms/toaster.tsx](../../../apps/perps/src/ui/atoms/toaster.tsx) |
| `ts_export` | `const Tooltip = TooltipPrimitive.Root` | [apps/perps/src/ui/atoms/tooltip.tsx](../../../apps/perps/src/ui/atoms/tooltip.tsx) |
| `ts_export` | `const TooltipContent = React.forwardRef<` | [apps/perps/src/ui/atoms/tooltip.tsx](../../../apps/perps/src/ui/atoms/tooltip.tsx) |
| `ts_export` | `const TooltipProvider = TooltipPrimitive.Provider` | [apps/perps/src/ui/atoms/tooltip.tsx](../../../apps/perps/src/ui/atoms/tooltip.tsx) |
| `ts_export` | `const TooltipTrigger = TooltipPrimitive.Trigger` | [apps/perps/src/ui/atoms/tooltip.tsx](../../../apps/perps/src/ui/atoms/tooltip.tsx) |
| `ts_export` | `export const reducer = (state: State, action: Action): State =>` | [apps/perps/src/ui/atoms/use-toast.tsx](../../../apps/perps/src/ui/atoms/use-toast.tsx) |
| `ts_export` | `function toast({ ...props }: Toast)` | [apps/perps/src/ui/atoms/use-toast.tsx](../../../apps/perps/src/ui/atoms/use-toast.tsx) |
| `ts_export` | `type ToastProps =` | [apps/perps/src/ui/atoms/use-toast.tsx](../../../apps/perps/src/ui/atoms/use-toast.tsx) |
| `ts_export` | `function useToast()` | [apps/perps/src/ui/atoms/use-toast.tsx](../../../apps/perps/src/ui/atoms/use-toast.tsx) |
| `ts_export` | `export function useAnimatedNumber(target: number, duration = 600): number` | [apps/perps/src/ui/hooks/useAnimatedNumber.ts](../../../apps/perps/src/ui/hooks/useAnimatedNumber.ts) |
| `ts_export` | `export default GasWeather;` | [apps/perps/src/ui/tokens/GasWeather.tsx](../../../apps/perps/src/ui/tokens/GasWeather.tsx) |
| `ts_export` | `export function GasWeather({ className, style }: GasWeatherProps)` | [apps/perps/src/ui/tokens/GasWeather.tsx](../../../apps/perps/src/ui/tokens/GasWeather.tsx) |
| `ts_export` | `export interface GasWeatherProps` | [apps/perps/src/ui/tokens/GasWeather.tsx](../../../apps/perps/src/ui/tokens/GasWeather.tsx) |
| `ts_export` | `export const Icon: Record<` | [apps/perps/src/ui/tokens/Icon.tsx](../../../apps/perps/src/ui/tokens/Icon.tsx) |
| `ts_export` | `export default LiquidStat;` | [apps/perps/src/ui/tokens/LiquidStat.tsx](../../../apps/perps/src/ui/tokens/LiquidStat.tsx) |
| `ts_export` | `export function LiquidStat({ states, width = 360, className, style }: LiquidStatProps)` | [apps/perps/src/ui/tokens/LiquidStat.tsx](../../../apps/perps/src/ui/tokens/LiquidStat.tsx) |
| `ts_export` | `export interface LiquidStatProps` | [apps/perps/src/ui/tokens/LiquidStat.tsx](../../../apps/perps/src/ui/tokens/LiquidStat.tsx) |
| `ts_export` | `export interface LiquidStatState` | [apps/perps/src/ui/tokens/LiquidStat.tsx](../../../apps/perps/src/ui/tokens/LiquidStat.tsx) |
| `ts_export` | `export default Overview;` | [apps/perps/src/ui/tokens/Overview.tsx](../../../apps/perps/src/ui/tokens/Overview.tsx) |
| `ts_export` | `export function Overview({ totalValue, totalDelta24h, unrealized, unrealizedPct, openCount, openTokens, realized, realizedTradeCount, className, style, }: OverviewProps)` | [apps/perps/src/ui/tokens/Overview.tsx](../../../apps/perps/src/ui/tokens/Overview.tsx) |
| `ts_export` | `export interface OverviewProps` | [apps/perps/src/ui/tokens/Overview.tsx](../../../apps/perps/src/ui/tokens/Overview.tsx) |
| `ts_export` | `export interface OverviewTokenDot` | [apps/perps/src/ui/tokens/Overview.tsx](../../../apps/perps/src/ui/tokens/Overview.tsx) |
| `ts_export` | `export default Performance;` | [apps/perps/src/ui/tokens/Performance.tsx](../../../apps/perps/src/ui/tokens/Performance.tsx) |
| `ts_export` | `export function Performance({ sevenDPnl, sevenDPnlVsPrevPct, sevenDVolume, sevenDTrades, sevenDRoiPct, sevenDRoiAnnualizedPct, dailyPnls, dailyLabels, className, style, }: PerformanceProps)` | [apps/perps/src/ui/tokens/Performance.tsx](../../../apps/perps/src/ui/tokens/Performance.tsx) |
| `ts_export` | `export interface PerformanceProps` | [apps/perps/src/ui/tokens/Performance.tsx](../../../apps/perps/src/ui/tokens/Performance.tsx) |
| `ts_export` | `export function TokenBadge({ token, size = 28, className }: TokenBadgeProps)` | [apps/perps/src/ui/tokens/TokenBadge.tsx](../../../apps/perps/src/ui/tokens/TokenBadge.tsx) |
| `ts_export` | `export interface TokenSpec` | [apps/perps/src/ui/tokens/TokenBadge.tsx](../../../apps/perps/src/ui/tokens/TokenBadge.tsx) |
| `ts_export` | `export function cn(...inputs: ClassValue[])` | [apps/perps/src/ui/utils/cn.ts](../../../apps/perps/src/ui/utils/cn.ts) |
| `ts_export` | `export function fmt(n: number, digits = 2): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function fmtChangePct(v: number): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function fmtCompact(n: number): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function fmtDate(v: string \| null \| undefined): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function fmtPercent(v: number, digits: number = 4): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function fmtPrice(price: number): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function fmtUsd(v: number \| string \| null \| undefined): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function fmtUsdCompact(v: number \| string \| null \| undefined): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function parse18(v: string \| null \| undefined): number` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function parseWei(v: string): bigint` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function shortAddr(a: string): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function toPlainDecimal(v: string): string` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function weiOrZero(v: string \| null \| undefined): bigint` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function weiPct(a: bigint, b: bigint): number \| null` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |
| `ts_export` | `export function weiToNumber(wei: bigint): number` | [apps/perps/src/ui/utils/format.ts](../../../apps/perps/src/ui/utils/format.ts) |

## utils

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function cn(...inputs: ClassValue[])` | [apps/perps/src/utils/cn.ts](../../../apps/perps/src/utils/cn.ts) |
| `ts_export` | `export function formatLargeNumber(value: string): string` | [apps/perps/src/utils/format.ts](../../../apps/perps/src/utils/format.ts) |

## widgets

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export function AlertsPanel({ address }: AlertsPanelProps)` | [apps/perps/src/widgets/alerts/AlertsPanel.tsx](../../../apps/perps/src/widgets/alerts/AlertsPanel.tsx) |
| `ts_export` | `export function AnalyticsLiquidStat()` | [apps/perps/src/widgets/analytics/AnalyticsLiquidStat.tsx](../../../apps/perps/src/widgets/analytics/AnalyticsLiquidStat.tsx) |
| `ts_export` | `export default AnalyticsLiquidStat;` | [apps/perps/src/widgets/analytics/AnalyticsLiquidStat.tsx](../../../apps/perps/src/widgets/analytics/AnalyticsLiquidStat.tsx) |
| `ts_export` | `export function FundingHistoryChart({ marketId, isMobile }: FundingHistoryChartProps)` | [apps/perps/src/widgets/analytics/FundingHistoryChart.tsx](../../../apps/perps/src/widgets/analytics/FundingHistoryChart.tsx) |
| `ts_export` | `export const FUNDING_LABEL_ONCHAIN_24H = "funding paid (24h, on-chain)";` | [apps/perps/src/widgets/analytics/MarketSnapshotTable.tsx](../../../apps/perps/src/widgets/analytics/MarketSnapshotTable.tsx) |
| `ts_export` | `export function MarketSnapshotTable({ marketId }: MarketSnapshotTableProps)` | [apps/perps/src/widgets/analytics/MarketSnapshotTable.tsx](../../../apps/perps/src/widgets/analytics/MarketSnapshotTable.tsx) |
| `ts_export` | `export function MarkPriceChart({ marketId, isMobile }: MarkPriceChartProps)` | [apps/perps/src/widgets/analytics/MarkPriceChart.tsx](../../../apps/perps/src/widgets/analytics/MarkPriceChart.tsx) |
| `ts_export` | `export function OiChart({ marketId, isMobile }: OiChartProps)` | [apps/perps/src/widgets/analytics/OiChart.tsx](../../../apps/perps/src/widgets/analytics/OiChart.tsx) |
| `ts_export` | `export function ProtocolTvlChart({ isMobile }: ProtocolTvlChartProps)` | [apps/perps/src/widgets/analytics/ProtocolTvlChart.tsx](../../../apps/perps/src/widgets/analytics/ProtocolTvlChart.tsx) |
| `ts_export` | `export function ErrorPanel({ error, onRetry, size = "md", bare = false, className, }: ErrorPanelProps)` | [apps/perps/src/widgets/errors/ErrorPanel.tsx](../../../apps/perps/src/widgets/errors/ErrorPanel.tsx) |
| `ts_export` | `export { AlertCircle as ErrorGlyph };` | [apps/perps/src/widgets/errors/ErrorToast.tsx](../../../apps/perps/src/widgets/errors/ErrorToast.tsx) |
| `ts_export` | `export function showErrorToast( ufe: UserFacingError, opts: ShowErrorToastOptions = {}, ):` | [apps/perps/src/widgets/errors/ErrorToast.tsx](../../../apps/perps/src/widgets/errors/ErrorToast.tsx) |
| `ts_export` | `export interface ShowErrorToastOptions` | [apps/perps/src/widgets/errors/ErrorToast.tsx](../../../apps/perps/src/widgets/errors/ErrorToast.tsx) |
| `ts_export` | `export function toastError(err: unknown, opts: ToastErrorOptions = {}): void` | [apps/perps/src/widgets/errors/ErrorToast.tsx](../../../apps/perps/src/widgets/errors/ErrorToast.tsx) |
| `ts_export` | `export interface ToastErrorOptions extends ShowErrorToastOptions` | [apps/perps/src/widgets/errors/ErrorToast.tsx](../../../apps/perps/src/widgets/errors/ErrorToast.tsx) |
| `ts_export` | `export function closeReportModal(): void` | [apps/perps/src/widgets/errors/ReportFeedbackModal.tsx](../../../apps/perps/src/widgets/errors/ReportFeedbackModal.tsx) |
| `ts_export` | `export function openReportModal(error: UserFacingError): void` | [apps/perps/src/widgets/errors/ReportFeedbackModal.tsx](../../../apps/perps/src/widgets/errors/ReportFeedbackModal.tsx) |
| `ts_export` | `export function ReportFeedbackModalHost()` | [apps/perps/src/widgets/errors/ReportFeedbackModal.tsx](../../../apps/perps/src/widgets/errors/ReportFeedbackModal.tsx) |
| `ts_export` | `export const ToastAction = React.forwardRef<` | [apps/perps/src/widgets/errors/ToastActionBridge.tsx](../../../apps/perps/src/widgets/errors/ToastActionBridge.tsx) |
| `ts_export` | `export type ToastActionElement = React.ReactElement;` | [apps/perps/src/widgets/errors/ToastActionBridge.tsx](../../../apps/perps/src/widgets/errors/ToastActionBridge.tsx) |
| `ts_export` | `export function BlockDetailPanel({ identifier, seedBlock, onClose }: BlockDetailPanelProps)` | [apps/perps/src/widgets/explorer/BlockDetailPanel.tsx](../../../apps/perps/src/widgets/explorer/BlockDetailPanel.tsx) |
| `ts_export` | `export function BlockRow({ block, isNew = false, onSelect, isSelected = false }: BlockRowProps)` | [apps/perps/src/widgets/explorer/BlockRow.tsx](../../../apps/perps/src/widgets/explorer/BlockRow.tsx) |
| `ts_export` | `export function buildExplorerUrl(focus: ExplorerFocus): string` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export type ExplorerFocus =` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export function fmtGas(v: string \| null \| undefined): string` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export function fmtGasPct(used: string \| null \| undefined, limit: string \| null \| undefined): string` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export function looksLikeBlockHeight(s: string): boolean` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export function looksLikeHash(s: string): boolean` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export function parseFocus( query: Record<string, string \| Array<string> \| undefined> \| URLSearchParams, ): ExplorerFocus` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export function toRelativeTime(input: string \| number \| Date \| null \| undefined): string` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export function truncateHash(s: string \| null \| undefined, front = 8, back = 4): string` | [apps/perps/src/widgets/explorer/explorer-utils.ts](../../../apps/perps/src/widgets/explorer/explorer-utils.ts) |
| `ts_export` | `export function LiveBlockStream({ onSelectBlock, selectedBlockHash, maxBlocks = 100, }: LiveBlockStreamProps)` | [apps/perps/src/widgets/explorer/LiveBlockStream.tsx](../../../apps/perps/src/widgets/explorer/LiveBlockStream.tsx) |
| `ts_export` | `export function TxDetailPanel({ txHash, onClose }: TxDetailPanelProps)` | [apps/perps/src/widgets/explorer/TxDetailPanel.tsx](../../../apps/perps/src/widgets/explorer/TxDetailPanel.tsx) |
| `ts_export` | `export function MarketsFilterBar({ category, onChangeCategory, search, onChangeSearch, favoriteCount, className, }: MarketsFilterBarProps)` | [apps/perps/src/widgets/markets/MarketsFilterBar.tsx](../../../apps/perps/src/widgets/markets/MarketsFilterBar.tsx) |
| `ts_export` | `export function MarketsHeaderTabs({ activeTab, onChangeTab, mode = "perps", onChangeMode, rightSlot, }: MarketsHeaderTabsProps)` | [apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx](../../../apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx) |
| `ts_export` | `export type MarketsMode = "perps" \| "spot";` | [apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx](../../../apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx) |
| `ts_export` | `export function marketsModes(spotOpen: boolean = isSpotOpen()): MarketsMode[]` | [apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx](../../../apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx) |
| `ts_export` | `export type MarketsTab = "markets" \| "funding";` | [apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx](../../../apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx) |
| `ts_export` | `export function MarketsHighlights({ rows, hasAnyLive, variant = "columns", className, }: MarketsHighlightsProps)` | [apps/perps/src/widgets/markets/MarketsHighlights.tsx](../../../apps/perps/src/widgets/markets/MarketsHighlights.tsx) |
| `ts_export` | `export function MarketsOverviewStats({ rows, layout = "column", className, }: MarketsOverviewStatsProps)` | [apps/perps/src/widgets/markets/MarketsOverviewStats.tsx](../../../apps/perps/src/widgets/markets/MarketsOverviewStats.tsx) |
| `ts_export` | `export function HighlightsPanelSkeleton({ rows = 5 }: { rows?: number })` | [apps/perps/src/widgets/markets/MarketsSkeletons.tsx](../../../apps/perps/src/widgets/markets/MarketsSkeletons.tsx) |
| `ts_export` | `export function MarketsTableSkeleton({ rows = 8, hiddenColsMd = false, hiddenColsLg = true, }: { rows?: number; hiddenColsMd?: boolean; hiddenColsLg?: boolean; })` | [apps/perps/src/widgets/markets/MarketsSkeletons.tsx](../../../apps/perps/src/widgets/markets/MarketsSkeletons.tsx) |
| `ts_export` | `export function OverviewStatsSkeleton()` | [apps/perps/src/widgets/markets/MarketsSkeletons.tsx](../../../apps/perps/src/widgets/markets/MarketsSkeletons.tsx) |
| `ts_export` | `export function MarketsTable({ rows, loading, emptyMessage, favorites, onToggleFavorite, sortKey, sortDir, onToggleSort, }: MarketsTableProps)` | [apps/perps/src/widgets/markets/MarketsTable.tsx](../../../apps/perps/src/widgets/markets/MarketsTable.tsx) |
| `ts_export` | `export function useSortedPageRows( rows: readonly MarketRow[], pageSize: number, page: number, ):` | [apps/perps/src/widgets/markets/MarketsTable.tsx](../../../apps/perps/src/widgets/markets/MarketsTable.tsx) |
| `ts_export` | `export interface MarketRow extends TradeableMarket {` | [apps/perps/src/widgets/markets/types.ts](../../../apps/perps/src/widgets/markets/types.ts) |
| `ts_export` | `export type MarketsCategoryFilter =` | [apps/perps/src/widgets/markets/types.ts](../../../apps/perps/src/widgets/markets/types.ts) |
| `ts_export` | `export type MarketsHighlight = "new" \| "gainers" \| "losers";` | [apps/perps/src/widgets/markets/types.ts](../../../apps/perps/src/widgets/markets/types.ts) |
| `ts_export` | `export type SortDir = "asc" \| "desc";` | [apps/perps/src/widgets/markets/types.ts](../../../apps/perps/src/widgets/markets/types.ts) |
| `ts_export` | `export type SortKey =` | [apps/perps/src/widgets/markets/types.ts](../../../apps/perps/src/widgets/markets/types.ts) |
| `ts_export` | `export function filterAndSort( rows: readonly MarketRow[], opts: { search?: string; category?: string; favorites?: ReadonlySet<string>; sortKey?: import("./types").SortKey; sortDir?: import("./types").SortDir; }, ): MarketRow[]` | [apps/perps/src/widgets/markets/useMarketsRows.ts](../../../apps/perps/src/widgets/markets/useMarketsRows.ts) |
| `ts_export` | `export function useMarketsRows():` | [apps/perps/src/widgets/markets/useMarketsRows.ts](../../../apps/perps/src/widgets/markets/useMarketsRows.ts) |
| `ts_export` | `export function MobileAccountView()` | [apps/perps/src/widgets/mobile/MobileAccountView.tsx](../../../apps/perps/src/widgets/mobile/MobileAccountView.tsx) |
| `ts_export` | `export function MobileAppLayout({ market, onSelectMarket }: MobileAppLayoutProps)` | [apps/perps/src/widgets/mobile/MobileAppLayout.tsx](../../../apps/perps/src/widgets/mobile/MobileAppLayout.tsx) |
| `ts_export` | `export function MobileBalancePanel()` | [apps/perps/src/widgets/mobile/MobileBalancePanel.tsx](../../../apps/perps/src/widgets/mobile/MobileBalancePanel.tsx) |
| `ts_export` | `export function MobileBottomNav({ active, onChange }: MobileBottomNavProps)` | [apps/perps/src/widgets/mobile/MobileBottomNav.tsx](../../../apps/perps/src/widgets/mobile/MobileBottomNav.tsx) |
| `ts_export` | `export type MobileGlobalTab = "markets" \| "trade" \| "account";` | [apps/perps/src/widgets/mobile/MobileBottomNav.tsx](../../../apps/perps/src/widgets/mobile/MobileBottomNav.tsx) |
| `ts_export` | `export function MobileHeader()` | [apps/perps/src/widgets/mobile/MobileHeader.tsx](../../../apps/perps/src/widgets/mobile/MobileHeader.tsx) |
| `ts_export` | `export function LanguageToggle()` | [apps/perps/src/widgets/mobile/MobileLanguageToggle.tsx](../../../apps/perps/src/widgets/mobile/MobileLanguageToggle.tsx) |
| `ts_export` | `export function MobileMarketView({ market }: MobileMarketViewProps)` | [apps/perps/src/widgets/mobile/MobileMarketView.tsx](../../../apps/perps/src/widgets/mobile/MobileMarketView.tsx) |
| `ts_export` | `export function MobilePanelToolbar({ sortValue = "Market", onSortChange, onExpandAll }: MobilePanelToolbarProps)` | [apps/perps/src/widgets/mobile/MobilePanelToolbar.tsx](../../../apps/perps/src/widgets/mobile/MobilePanelToolbar.tsx) |
| `ts_export` | `export function MobileSecondaryTabContent({ activeTab }: MobileSecondaryTabContentProps)` | [apps/perps/src/widgets/mobile/MobileSecondaryTabContent.tsx](../../../apps/perps/src/widgets/mobile/MobileSecondaryTabContent.tsx) |
| `ts_export` | `export function MobileSecondaryTabs({ active, onChange }: MobileSecondaryTabsProps)` | [apps/perps/src/widgets/mobile/MobileSecondaryTabs.tsx](../../../apps/perps/src/widgets/mobile/MobileSecondaryTabs.tsx) |
| `ts_export` | `export function MobileSpotAccountView()` | [apps/perps/src/widgets/mobile/MobileSpotAccountView.tsx](../../../apps/perps/src/widgets/mobile/MobileSpotAccountView.tsx) |
| `ts_export` | `export function MobileSpotAppLayout({ market, onSelectMarket, }: MobileSpotAppLayoutProps)` | [apps/perps/src/widgets/mobile/MobileSpotAppLayout.tsx](../../../apps/perps/src/widgets/mobile/MobileSpotAppLayout.tsx) |
| `ts_export` | `export function MobileSpotBalancePanel()` | [apps/perps/src/widgets/mobile/MobileSpotBalancePanel.tsx](../../../apps/perps/src/widgets/mobile/MobileSpotBalancePanel.tsx) |
| `ts_export` | `export function MobileSpotMarketView({ market }: MobileSpotMarketViewProps)` | [apps/perps/src/widgets/mobile/MobileSpotMarketView.tsx](../../../apps/perps/src/widgets/mobile/MobileSpotMarketView.tsx) |
| `ts_export` | `export function MobileSpotTickerBar({ market, onOpenSelector }: MobileSpotTickerBarProps)` | [apps/perps/src/widgets/mobile/MobileSpotTickerBar.tsx](../../../apps/perps/src/widgets/mobile/MobileSpotTickerBar.tsx) |
| `ts_export` | `export function MobileSpotTradeView({ market }: MobileSpotTradeViewProps)` | [apps/perps/src/widgets/mobile/MobileSpotTradeView.tsx](../../../apps/perps/src/widgets/mobile/MobileSpotTradeView.tsx) |
| `ts_export` | `export function MobileTickerBar({ market, onOpenSelector }: MobileTickerBarProps)` | [apps/perps/src/widgets/mobile/MobileTickerBar.tsx](../../../apps/perps/src/widgets/mobile/MobileTickerBar.tsx) |
| `ts_export` | `export function MobileTradeView({ market }: MobileTradeViewProps)` | [apps/perps/src/widgets/mobile/MobileTradeView.tsx](../../../apps/perps/src/widgets/mobile/MobileTradeView.tsx) |
| `ts_export` | `export function MobileVaultView()` | [apps/perps/src/widgets/mobile/MobileVaultView.tsx](../../../apps/perps/src/widgets/mobile/MobileVaultView.tsx) |
| `ts_export` | `export default NetworkPill;` | [apps/perps/src/widgets/network-pill/NetworkPill.tsx](../../../apps/perps/src/widgets/network-pill/NetworkPill.tsx) |
| `ts_export` | `export function NetworkPill({ className, style }: NetworkPillProps)` | [apps/perps/src/widgets/network-pill/NetworkPill.tsx](../../../apps/perps/src/widgets/network-pill/NetworkPill.tsx) |
| `ts_export` | `export interface NetworkPillProps` | [apps/perps/src/widgets/network-pill/NetworkPill.tsx](../../../apps/perps/src/widgets/network-pill/NetworkPill.tsx) |
| `ts_export` | `export function PaxscanSearch({` | [apps/perps/src/widgets/paxscan-search/PaxscanSearch.tsx](../../../apps/perps/src/widgets/paxscan-search/PaxscanSearch.tsx) |
| `ts_export` | `export interface PaxscanSearchProps` | [apps/perps/src/widgets/paxscan-search/PaxscanSearch.tsx](../../../apps/perps/src/widgets/paxscan-search/PaxscanSearch.tsx) |
| `ts_export` | `export function FeesBreakdownCard({ userAddress }: FeesBreakdownCardProps)` | [apps/perps/src/widgets/portfolio/FeesBreakdownCard.tsx](../../../apps/perps/src/widgets/portfolio/FeesBreakdownCard.tsx) |
| `ts_export` | `export function groupFees(data: readonly { feeType: number; feeTypeName?: string \| null; feeUsd: string \| null \| undefined }[]): FeeGroup[]` | [apps/perps/src/widgets/portfolio/FeesBreakdownCard.tsx](../../../apps/perps/src/widgets/portfolio/FeesBreakdownCard.tsx) |
| `ts_export` | `export function MobilePortfolioOverview({` | [apps/perps/src/widgets/portfolio/MobilePortfolioOverview.tsx](../../../apps/perps/src/widgets/portfolio/MobilePortfolioOverview.tsx) |
| `ts_export` | `export function OnChainHistoryPanel({ address, variant = "desktop", className, initialSubTab = "transfers", }: OnChainHistoryPanelProps)` | [apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx](../../../apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx) |
| `ts_export` | `export interface OnChainHistoryPanelProps` | [apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx](../../../apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx) |
| `ts_export` | `export type OnChainHistorySubTab = "transfers" \| "transactions";` | [apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx](../../../apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx) |
| `ts_export` | `export type OnChainHistoryVariant = "desktop" \| "mobile";` | [apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx](../../../apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx) |
| `ts_export` | `export function PerformanceCards({ dailyPnl, isLoading, }: PerformanceCardsProps)` | [apps/perps/src/widgets/portfolio/PerformanceCards.tsx](../../../apps/perps/src/widgets/portfolio/PerformanceCards.tsx) |
| `ts_export` | `export function sevenDayTotals(dailyPnl: DailyPnl[]):` | [apps/perps/src/widgets/portfolio/PerformanceCards.tsx](../../../apps/perps/src/widgets/portfolio/PerformanceCards.tsx) |
| `ts_export` | `export function PnlChart({ dailyPnl }: PnlChartProps)` | [apps/perps/src/widgets/portfolio/PnlChart.tsx](../../../apps/perps/src/widgets/portfolio/PnlChart.tsx) |
| `ts_export` | `export type AssetsTimeRange = "7D" \| "30D" \| "90D" \| "ALL";` | [apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx) |
| `ts_export` | `export function buildAssetSeries(` | [apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx) |
| `ts_export` | `export function PortfolioAssetsChart({` | [apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx) |
| `ts_export` | `PortfolioFeeTierPanel` | [apps/perps/src/widgets/portfolio/PortfolioFeeTierPanel.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioFeeTierPanel.tsx) |
| `ts_export` | `export interface PortfolioFeeTierPanelProps` | [apps/perps/src/widgets/portfolio/PortfolioFeeTierPanel.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioFeeTierPanel.tsx) |
| `ts_export` | `export function PortfolioMobileNav({ active, onChange }: PortfolioMobileNavProps)` | [apps/perps/src/widgets/portfolio/PortfolioMobileNav.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioMobileNav.tsx) |
| `ts_export` | `export interface PortfolioMobileNavProps` | [apps/perps/src/widgets/portfolio/PortfolioMobileNav.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioMobileNav.tsx) |
| `ts_export` | `export function PortfolioOverviewPanel(props: PortfolioOverviewPanelProps)` | [apps/perps/src/widgets/portfolio/PortfolioOverviewPanel.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioOverviewPanel.tsx) |
| `ts_export` | `export function buildSeries(dailyPnl: DailyPnl[], range: PerformanceRange, now: Date = new Date()): SeriesPoint[]` | [apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx) |
| `ts_export` | `export type PerformanceRange = "7D" \| "30D" \| "90D" \| "ALL";` | [apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx) |
| `ts_export` | `export function PortfolioPerformanceGrid({` | [apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx) |
| `ts_export` | `export function sliceByRange(dailyPnl: DailyPnl[], range: PerformanceRange, now: Date = new Date()): DailyPnl[]` | [apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx) |
| `ts_export` | `export function windowStartKey(range: Exclude<PerformanceRange, "ALL">, now: Date = new Date()): string` | [apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx) |
| `ts_export` | `export interface WindowTotals` | [apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx) |
| `ts_export` | `export function windowTotals(rows: DailyPnl[]): WindowTotals \| null` | [apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx) |
| `ts_export` | `export function PortfolioPositionsPanel({ positions, isConnected, hasVault, onConnect, }: PortfolioPositionsPanelProps)` | [apps/perps/src/widgets/portfolio/PortfolioPositionsPanel.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPositionsPanel.tsx) |
| `ts_export` | `export interface PortfolioPositionsPanelProps` | [apps/perps/src/widgets/portfolio/PortfolioPositionsPanel.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPositionsPanel.tsx) |
| `ts_export` | `export function ConnectGate({ isConnected, onConnect }: ConnectGateProps)` | [apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx) |
| `ts_export` | `export interface ConnectGateProps` | [apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx) |
| `ts_export` | `export function EmptyState({ title, description }: EmptyStateProps)` | [apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx) |
| `ts_export` | `export interface EmptyStateProps` | [apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx) |
| `ts_export` | `export function PlaceholderPanel({ title, subtitle, hint }: PlaceholderPanelProps)` | [apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx) |
| `ts_export` | `export interface PlaceholderPanelProps` | [apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx) |
| `ts_export` | `export function SectionCard({ title, count, children }: SectionCardProps)` | [apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx) |
| `ts_export` | `export interface SectionCardProps` | [apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx) |
| `ts_export` | `export function PortfolioSettingsPanel({ onOpenFullSettings, }: PortfolioSettingsPanelProps)` | [apps/perps/src/widgets/portfolio/PortfolioSettingsPanel.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioSettingsPanel.tsx) |
| `ts_export` | `export interface PortfolioSettingsPanelProps` | [apps/perps/src/widgets/portfolio/PortfolioSettingsPanel.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioSettingsPanel.tsx) |
| `ts_export` | `const NAV_ITEMS: NavItem[] = [` | [apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx) |
| `ts_export` | `export type PortfolioNavKey =` | [apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx) |
| `ts_export` | `export function PortfolioSideNav({ active, onChange, collapsed, onToggleCollapsed, }: PortfolioSideNavProps)` | [apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx) |
| `ts_export` | `export function PortfolioSpotSection({ title, emptyMessage }: PortfolioSpotSectionProps)` | [apps/perps/src/widgets/portfolio/PortfolioSpotSection.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioSpotSection.tsx) |
| `ts_export` | `export function PortfolioTopBar({` | [apps/perps/src/widgets/portfolio/PortfolioTopBar.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioTopBar.tsx) |
| `ts_export` | `export interface PortfolioTopBarProps` | [apps/perps/src/widgets/portfolio/PortfolioTopBar.tsx](../../../apps/perps/src/widgets/portfolio/PortfolioTopBar.tsx) |
| `ts_export` | `export function absUsd(v: string \| null \| undefined): number` | [apps/perps/src/widgets/portfolio/TradeSettlementHistory.tsx](../../../apps/perps/src/widgets/portfolio/TradeSettlementHistory.tsx) |
| `ts_export` | `export function TradeSettlementHistory({` | [apps/perps/src/widgets/portfolio/TradeSettlementHistory.tsx](../../../apps/perps/src/widgets/portfolio/TradeSettlementHistory.tsx) |
| `ts_export` | `export interface TokenBalance` | [apps/perps/src/widgets/portfolio/types.ts](../../../apps/perps/src/widgets/portfolio/types.ts) |
| `ts_export` | `export function SpotMarketWatchlist({ selectedSymbol, onSelect, }: SpotMarketWatchlistProps)` | [apps/perps/src/widgets/spot/SpotMarketWatchlist.tsx](../../../apps/perps/src/widgets/spot/SpotMarketWatchlist.tsx) |
| `ts_export` | `export function SpotOrderbook({ symbol, height = 580, onPriceClick, compact = false, }: SpotOrderbookProps)` | [apps/perps/src/widgets/spot/SpotOrderbook.tsx](../../../apps/perps/src/widgets/spot/SpotOrderbook.tsx) |
| `ts_export` | `export interface SpotOrderbookLevel` | [apps/perps/src/widgets/spot/SpotOrderbook.tsx](../../../apps/perps/src/widgets/spot/SpotOrderbook.tsx) |
| `ts_export` | `export function spotConversionPrice(p: { orderType: SpotOrderType; side: "BUY" \| "SELL"; limitPrice: string; book: Pick<SpotOrderbook, "bids" \| "asks"> \| null; }):` | [apps/perps/src/widgets/spot/SpotOrderForm.tsx](../../../apps/perps/src/widgets/spot/SpotOrderForm.tsx) |
| `ts_export` | `export function SpotOrderForm({ market, compact = false }: SpotOrderFormProps)` | [apps/perps/src/widgets/spot/SpotOrderForm.tsx](../../../apps/perps/src/widgets/spot/SpotOrderForm.tsx) |
| `ts_export` | `export function spotOrderUnits(p: { sizeMode: SizeMode; amount: string; price: string; baseDecimals: number; quoteDecimals: number; }):` | [apps/perps/src/widgets/spot/SpotOrderForm.tsx](../../../apps/perps/src/widgets/spot/SpotOrderForm.tsx) |
| `ts_export` | `export function spotSellExceedsBalance( baseUnits: bigint, holding: { raw: string; decimals: number } \| null, baseDecimals: number, ): boolean` | [apps/perps/src/widgets/spot/SpotOrderForm.tsx](../../../apps/perps/src/widgets/spot/SpotOrderForm.tsx) |
| `ts_export` | `export function SpotPairHeader({ selectedPair, onPairChange }: SpotPairHeaderProps)` | [apps/perps/src/widgets/spot/SpotPairHeader.tsx](../../../apps/perps/src/widgets/spot/SpotPairHeader.tsx) |
| `ts_export` | `export function SpotPositionsPanel()` | [apps/perps/src/widgets/spot/SpotPositionsPanel.tsx](../../../apps/perps/src/widgets/spot/SpotPositionsPanel.tsx) |
| `ts_export` | `export function SpotTradesTape({` | [apps/perps/src/widgets/spot/SpotTradesTape.tsx](../../../apps/perps/src/widgets/spot/SpotTradesTape.tsx) |
| `ts_export` | `export interface SpotTradesTapeProps` | [apps/perps/src/widgets/spot/SpotTradesTape.tsx](../../../apps/perps/src/widgets/spot/SpotTradesTape.tsx) |
| `ts_export` | `export function Chart({` | [apps/perps/src/widgets/trading/Chart.tsx](../../../apps/perps/src/widgets/trading/Chart.tsx) |
| `ts_export` | `export { MarketWatchlist as MarketSelectorPanel };` | [apps/perps/src/widgets/trading/MarketSelectorPanel.tsx](../../../apps/perps/src/widgets/trading/MarketSelectorPanel.tsx) |
| `ts_export` | `export const MarketWatchlist = forwardRef<HTMLDivElement, MarketWatchlistProps>(function MarketWatchlist({ chrome = "rail", width, collapsed, onToggle, onSelect, selectedSymbol }, ref)` | [apps/perps/src/widgets/trading/MarketSelectorPanel.tsx](../../../apps/perps/src/widgets/trading/MarketSelectorPanel.tsx) |
| `ts_export` | `export function MarketTradesTape({ symbol }: MarketTradesTapeProps)` | [apps/perps/src/widgets/trading/MarketTradesTape.tsx](../../../apps/perps/src/widgets/trading/MarketTradesTape.tsx) |
| `ts_export` | `export interface MarketTradesTapeProps` | [apps/perps/src/widgets/trading/MarketTradesTape.tsx](../../../apps/perps/src/widgets/trading/MarketTradesTape.tsx) |
| `ts_export` | `export function AdjustCollateralModal({ positionId, mode, currentCollateral, currentSize, currentLeverage, onClose, onSuccess, }: AdjustCollateralModalProps)` | [apps/perps/src/widgets/trading/modals/AdjustCollateralModal.tsx](../../../apps/perps/src/widgets/trading/modals/AdjustCollateralModal.tsx) |
| `ts_export` | `export function SetTpSlModal({ positionId, currentEntryPrice, isLong, markPrice, onClose, onSuccess, }: SetTpSlModalProps)` | [apps/perps/src/widgets/trading/modals/SetTpSlModal.tsx](../../../apps/perps/src/widgets/trading/modals/SetTpSlModal.tsx) |
| `ts_export` | `export function TopUpPositionModal({ positionId, userAddress, onClose, onSuccess, }: TopUpPositionModalProps)` | [apps/perps/src/widgets/trading/modals/TopUpPositionModal.tsx](../../../apps/perps/src/widgets/trading/modals/TopUpPositionModal.tsx) |
| `ts_export` | `export function DepthPanel({ symbol, height, onPriceClick }: DepthPanelProps)` | [apps/perps/src/widgets/trading/Orderbook.tsx](../../../apps/perps/src/widgets/trading/Orderbook.tsx) |
| `ts_export` | `export function Orderbook({ symbol, height = 580, onPriceClick, compact = false }: OrderbookProps)` | [apps/perps/src/widgets/trading/Orderbook.tsx](../../../apps/perps/src/widgets/trading/Orderbook.tsx) |
| `ts_export` | `export interface OrderbookLevel` | [apps/perps/src/widgets/trading/Orderbook.tsx](../../../apps/perps/src/widgets/trading/Orderbook.tsx) |
| `ts_export` | `export function PairHeader({ selectedPair, onPairChange, onOpenMarketPanel }: PairHeaderProps)` | [apps/perps/src/widgets/trading/PairHeader.tsx](../../../apps/perps/src/widgets/trading/PairHeader.tsx) |
| `ts_export` | `export function PerpsOrderForm({` | [apps/perps/src/widgets/trading/PerpsOrderForm.tsx](../../../apps/perps/src/widgets/trading/PerpsOrderForm.tsx) |
| `ts_export` | `export function PerpsPositionsPanel()` | [apps/perps/src/widgets/trading/PerpsPositionsPanel.tsx](../../../apps/perps/src/widgets/trading/PerpsPositionsPanel.tsx) |
| `ts_export` | `export interface PnlCardData` | [apps/perps/src/widgets/trading/PnlShareCard.tsx](../../../apps/perps/src/widgets/trading/PnlShareCard.tsx) |
| `ts_export` | `export function PnlShareCard({ data, onClose }: PnlShareCardProps)` | [apps/perps/src/widgets/trading/PnlShareCard.tsx](../../../apps/perps/src/widgets/trading/PnlShareCard.tsx) |
| `ts_export` | `export function loadTradingView(): Promise<void>` | [apps/perps/src/widgets/trading/tradingViewLoader.ts](../../../apps/perps/src/widgets/trading/tradingViewLoader.ts) |
