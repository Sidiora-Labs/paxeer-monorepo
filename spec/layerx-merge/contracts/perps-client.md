# Perps client contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## app

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-app-modals-connectwalletmodal-connectwalletmodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/modals/ConnectWalletModal.tsx:45</code> | <code>export function ConnectWalletModal()</code> |
| <code>interface.3.ts-perps-app-modals-depositwithdrawmodal-depositwithdrawmodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/modals/DepositWithdrawModal.tsx:30</code> | <code>export function DepositWithdrawModal({ isOpen, onClose }: DepositWithdrawModalProps)</code> |
| <code>interface.3.ts-perps-app-modals-settingsmodal-settingsmodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/modals/SettingsModal.tsx:30</code> | <code>export function SettingsModal({ isOpen, onClose }: SettingsModalProps)</code> |
| <code>interface.3.ts-perps-app-modals-walletcontrolpanel-walletcontrolpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/modals/WalletControlPanel.tsx:45</code> | <code>export function WalletControlPanel({ open, onOpenChange, trigger, onDeposit, onWithdraw, }: WalletControlPanelProps)</code> |
| <code>interface.3.ts-perps-app-providers-paxscanprovider-paxscanprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/providers/PaxscanProvider.tsx:6</code> | <code>export function PaxscanProvider({ children }: { children: React.ReactNode })</code> |
| <code>interface.3.ts-perps-app-shell-appshell-appshell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/AppShell.tsx:21</code> | <code>export function AppShell({ Component, pageProps }: AppShellProps)</code> |
| <code>interface.3.ts-perps-app-shell-cookiebanner-cookiebanner</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/CookieBanner.tsx:13</code> | <code>export function CookieBanner()</code> |
| <code>interface.3.ts-perps-app-shell-desktopsidebar-desktopsidebar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/DesktopSidebar.tsx:100</code> | <code>export function DesktopSidebar()</code> |
| <code>interface.3.ts-perps-app-shell-errorboundary-errorboundary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/ErrorBoundary.tsx:30</code> | <code>export class ErrorBoundary extends React.Component&lt;Props, State&gt;</code> |
| <code>interface.3.ts-perps-app-shell-footer-footer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/Footer.tsx:21</code> | <code>export function Footer()</code> |
| <code>interface.3.ts-perps-app-shell-footer-parsestatuspageurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/Footer.tsx:6</code> | <code>export function parseStatusPageUrl(raw: string &#124; undefined): string &#124; null</code> |
| <code>interface.3.ts-perps-app-shell-footersettings-footersettings</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/FooterSettings.tsx:19</code> | <code>export function FooterSettings({ isPaused, setIsPaused, showPrice, setShowPrice, tickerDisplayMode, setTickerDisplayMode }: FooterSettingsProps)</code> |
| <code>interface.3.ts-perps-app-shell-header-header</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/Header.tsx:21</code> | <code>export function Header()</code> |
| <code>interface.3.ts-perps-app-shell-mobilenav-mobilenavdrawer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/MobileNav.tsx:20</code> | <code>export function MobileNavDrawer({ open, onClose }: { open: boolean; onClose: () =&gt; void })</code> |
| <code>interface.3.ts-perps-app-shell-mobilenav-mobilenavtrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/MobileNav.tsx:8</code> | <code>export function MobileNavTrigger({ onClick }: { onClick: () =&gt; void })</code> |
| <code>interface.3.ts-perps-app-shell-mobilenav-usemobilenav</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/MobileNav.tsx:83</code> | <code>export function useMobileNav()</code> |
| <code>interface.3.ts-perps-app-shell-mobiletradeheader-mobiletradeheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/MobileTradeHeader.tsx:32</code> | <code>export function MobileTradeHeader({ selectedPair, onOpenMarketPanel, onSelectMarket }: MobileTradeHeaderProps)</code> |
| <code>interface.3.ts-perps-app-shell-splashscreen-splashscreen</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/app/shell/SplashScreen.tsx:8</code> | <code>export function SplashScreen()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-app-shell-footersettings-footersettings</code> | notes | no in-repo importer at HEAD |

## components

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-components-paxeer-activeorders-activeorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ActiveOrders.tsx:4</code> | <code>export function ActiveOrders()</code> |
| <code>interface.3.ts-perps-components-paxeer-activeorders-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ActiveOrders.tsx:54</code> | <code>export default ActiveOrders;</code> |
| <code>interface.3.ts-perps-components-paxeer-addresschip-addresschip</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/AddressChip.tsx:7</code> | <code>export function AddressChip()</code> |
| <code>interface.3.ts-perps-components-paxeer-addresschip-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/AddressChip.tsx:97</code> | <code>export default AddressChip;</code> |
| <code>interface.3.ts-perps-components-paxeer-closedpositions-closedpositions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ClosedPositions.tsx:5</code> | <code>export function ClosedPositions()</code> |
| <code>interface.3.ts-perps-components-paxeer-closedpositions-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ClosedPositions.tsx:60</code> | <code>export default ClosedPositions;</code> |
| <code>interface.3.ts-perps-components-paxeer-commandpalette-commandpalette</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/CommandPalette.tsx:4</code> | <code>export function CommandPalette()</code> |
| <code>interface.3.ts-perps-components-paxeer-commandpalette-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/CommandPalette.tsx:74</code> | <code>export default CommandPalette;</code> |
| <code>interface.3.ts-perps-components-paxeer-confirmslider-confirmslider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ConfirmSlider.tsx:6</code> | <code>export function ConfirmSlider({ label = "Slide to confirm swap", confirmedLabel = "Confirmed", accent = "var(--accent-solid)" })</code> |
| <code>interface.3.ts-perps-components-paxeer-confirmslider-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ConfirmSlider.tsx:113</code> | <code>export default ConfirmSlider;</code> |
| <code>interface.3.ts-perps-components-paxeer-cumulativepnl-cumulativepnl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/CumulativePnL.tsx:4</code> | <code>export function CumulativePnL()</code> |
| <code>interface.3.ts-perps-components-paxeer-cumulativepnl-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/CumulativePnL.tsx:69</code> | <code>export default CumulativePnL;</code> |
| <code>interface.3.ts-perps-components-paxeer-datainput-datainput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/DataInput.tsx:7</code> | <code>export function DataInput()</code> |
| <code>interface.3.ts-perps-components-paxeer-datainput-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/DataInput.tsx:103</code> | <code>export default DataInput;</code> |
| <code>interface.3.ts-perps-components-paxeer-depthchart-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/DepthChart.tsx:66</code> | <code>export default DepthChart;</code> |
| <code>interface.3.ts-perps-components-paxeer-depthchart-depthchart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/DepthChart.tsx:5</code> | <code>export function DepthChart()</code> |
| <code>interface.3.ts-perps-components-paxeer-epochcountdown-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/EpochCountdown.tsx:67</code> | <code>export default EpochCountdown;</code> |
| <code>interface.3.ts-perps-components-paxeer-epochcountdown-epochcountdown</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/EpochCountdown.tsx:5</code> | <code>export function EpochCountdown()</code> |
| <code>interface.3.ts-perps-components-paxeer-feesbreakdown-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/FeesBreakdown.tsx:71</code> | <code>export default FeesBreakdown;</code> |
| <code>interface.3.ts-perps-components-paxeer-feesbreakdown-feesbreakdown</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/FeesBreakdown.tsx:4</code> | <code>export function FeesBreakdown()</code> |
| <code>interface.3.ts-perps-components-paxeer-gasweather-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/GasWeather.tsx:71</code> | <code>export default GasWeather;</code> |
| <code>interface.3.ts-perps-components-paxeer-gasweather-gasweather</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/GasWeather.tsx:4</code> | <code>export function GasWeather()</code> |
| <code>interface.3.ts-perps-components-paxeer-gestureunlock-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/GestureUnlock.tsx:91</code> | <code>export default GestureUnlock;</code> |
| <code>interface.3.ts-perps-components-paxeer-gestureunlock-gestureunlock</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/GestureUnlock.tsx:4</code> | <code>export function GestureUnlock()</code> |
| <code>interface.3.ts-perps-components-paxeer-ghosthold-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/GhostHold.tsx:54</code> | <code>export default GhostHold;</code> |
| <code>interface.3.ts-perps-components-paxeer-ghosthold-ghosthold</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/GhostHold.tsx:4</code> | <code>export function GhostHold()</code> |
| <code>interface.3.ts-perps-components-paxeer-leaderboard-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/Leaderboard.tsx:114</code> | <code>export default Leaderboard;</code> |
| <code>interface.3.ts-perps-components-paxeer-leaderboard-leaderboard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/Leaderboard.tsx:4</code> | <code>export function Leaderboard()</code> |
| <code>interface.3.ts-perps-components-paxeer-leaderboardstats-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/LeaderboardStats.tsx:40</code> | <code>export default LeaderboardStats;</code> |
| <code>interface.3.ts-perps-components-paxeer-leaderboardstats-leaderboardstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/LeaderboardStats.tsx:4</code> | <code>export function LeaderboardStats()</code> |
| <code>interface.3.ts-perps-components-paxeer-leveragedial-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/LeverageDial.tsx:104</code> | <code>export default LeverageDial;</code> |
| <code>interface.3.ts-perps-components-paxeer-leveragedial-leveragedial</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/LeverageDial.tsx:4</code> | <code>export function LeverageDial()</code> |
| <code>interface.3.ts-perps-components-paxeer-liquidstat-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/LiquidStat.tsx:92</code> | <code>export default LiquidStat;</code> |
| <code>interface.3.ts-perps-components-paxeer-liquidstat-liquidstat</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/LiquidStat.tsx:7</code> | <code>export function LiquidStat()</code> |
| <code>interface.3.ts-perps-components-paxeer-magneticswap-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/MagneticSwap.tsx:168</code> | <code>export default MagneticSwap;</code> |
| <code>interface.3.ts-perps-components-paxeer-magneticswap-magneticswap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/MagneticSwap.tsx:8</code> | <code>export function MagneticSwap()</code> |
| <code>interface.3.ts-perps-components-paxeer-networkpill-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/NetworkPill.tsx:88</code> | <code>export default NetworkPill;</code> |
| <code>interface.3.ts-perps-components-paxeer-networkpill-networkpill</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/NetworkPill.tsx:7</code> | <code>export function NetworkPill()</code> |
| <code>interface.3.ts-perps-components-paxeer-orbitalselector-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/OrbitalSelector.tsx:109</code> | <code>export default OrbitalSelector;</code> |
| <code>interface.3.ts-perps-components-paxeer-orbitalselector-orbitalselector</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/OrbitalSelector.tsx:7</code> | <code>export function OrbitalSelector()</code> |
| <code>interface.3.ts-perps-components-paxeer-orderbook-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/OrderBook.tsx:97</code> | <code>export default OrderBook;</code> |
| <code>interface.3.ts-perps-components-paxeer-orderbook-orderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/OrderBook.tsx:7</code> | <code>export function OrderBook()</code> |
| <code>interface.3.ts-perps-components-paxeer-orderbooktopo-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/OrderbookTopo.tsx:65</code> | <code>export default OrderbookTopo;</code> |
| <code>interface.3.ts-perps-components-paxeer-orderbooktopo-orderbooktopo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/OrderbookTopo.tsx:4</code> | <code>export function OrderbookTopo()</code> |
| <code>interface.3.ts-perps-components-paxeer-orderentry-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/OrderEntry.tsx:146</code> | <code>export default OrderEntry;</code> |
| <code>interface.3.ts-perps-components-paxeer-orderentry-orderentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/OrderEntry.tsx:7</code> | <code>export function OrderEntry()</code> |
| <code>interface.3.ts-perps-components-paxeer-overview-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/Overview.tsx:51</code> | <code>export default Overview;</code> |
| <code>interface.3.ts-perps-components-paxeer-overview-overview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/Overview.tsx:5</code> | <code>export function Overview()</code> |
| <code>interface.3.ts-perps-components-paxeer-performance-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/Performance.tsx:62</code> | <code>export default Performance;</code> |
| <code>interface.3.ts-perps-components-paxeer-performance-performance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/Performance.tsx:4</code> | <code>export function Performance()</code> |
| <code>interface.3.ts-perps-components-paxeer-positioncard-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/PositionCard.tsx:122</code> | <code>export default PositionCard;</code> |
| <code>interface.3.ts-perps-components-paxeer-positioncard-positioncard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/PositionCard.tsx:6</code> | <code>export function PositionCard({ data })</code> |
| <code>interface.3.ts-perps-components-paxeer-pulsefeed-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/PulseFeed.tsx:81</code> | <code>export default PulseFeed;</code> |
| <code>interface.3.ts-perps-components-paxeer-pulsefeed-pulsefeed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/PulseFeed.tsx:7</code> | <code>export function PulseFeed()</code> |
| <code>interface.3.ts-perps-components-paxeer-radialallocator-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/RadialAllocator.tsx:93</code> | <code>export default RadialAllocator;</code> |
| <code>interface.3.ts-perps-components-paxeer-radialallocator-radialallocator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/RadialAllocator.tsx:7</code> | <code>export function RadialAllocator()</code> |
| <code>interface.3.ts-perps-components-paxeer-reactionrail-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ReactionRail.tsx:74</code> | <code>export default ReactionRail;</code> |
| <code>interface.3.ts-perps-components-paxeer-reactionrail-reactionrail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ReactionRail.tsx:4</code> | <code>export function ReactionRail()</code> |
| <code>interface.3.ts-perps-components-paxeer-recenttrades-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/RecentTrades.tsx:76</code> | <code>export default RecentTrades;</code> |
| <code>interface.3.ts-perps-components-paxeer-recenttrades-recenttrades</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/RecentTrades.tsx:7</code> | <code>export function RecentTrades()</code> |
| <code>interface.3.ts-perps-components-paxeer-riskdial-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/RiskDial.tsx:59</code> | <code>export default RiskDial;</code> |
| <code>interface.3.ts-perps-components-paxeer-riskdial-riskdial</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/RiskDial.tsx:4</code> | <code>export function RiskDial()</code> |
| <code>interface.3.ts-perps-components-paxeer-shardstatus-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ShardStatus.tsx:84</code> | <code>export default ShardStatus;</code> |
| <code>interface.3.ts-perps-components-paxeer-shardstatus-shardstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/ShardStatus.tsx:6</code> | <code>export function ShardStatus()</code> |
| <code>interface.3.ts-perps-components-paxeer-shared-cx</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/_shared.tsx:42</code> | <code>export function cx(...parts: Array&lt;string &#124; false &#124; null &#124; undefined&gt;): string</code> |
| <code>interface.3.ts-perps-components-paxeer-shared-fmt</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/_shared.tsx:46</code> | <code>export function fmt(n: number, digits = 2): string</code> |
| <code>interface.3.ts-perps-components-paxeer-shared-icon</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/_shared.tsx:26</code> | <code>export const Icon =</code> |
| <code>interface.3.ts-perps-components-paxeer-shared-shortaddr</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/_shared.tsx:50</code> | <code>export function shortAddr(a: string): string</code> |
| <code>interface.3.ts-perps-components-paxeer-shared-token</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/_shared.tsx:4</code> | <code>export type Token =</code> |
| <code>interface.3.ts-perps-components-paxeer-shared-tokenbadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/_shared.tsx:54</code> | <code>export function TokenBadge({ token, size = 28 }: { token: Token; size?: number })</code> |
| <code>interface.3.ts-perps-components-paxeer-shared-tokens</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/_shared.tsx:13</code> | <code>export const TOKENS: Token[] = [</code> |
| <code>interface.3.ts-perps-components-paxeer-shared-useanimatednumber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/_shared.tsx:69</code> | <code>export function useAnimatedNumber(target: number, duration = 600): number</code> |
| <code>interface.3.ts-perps-components-paxeer-smartbutton-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/SmartButton.tsx:64</code> | <code>export default SmartButton;</code> |
| <code>interface.3.ts-perps-components-paxeer-smartbutton-smartbutton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/SmartButton.tsx:7</code> | <code>export function SmartButton()</code> |
| <code>interface.3.ts-perps-components-paxeer-tradepathsankey-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TradePathSankey.tsx:85</code> | <code>export default TradePathSankey;</code> |
| <code>interface.3.ts-perps-components-paxeer-tradepathsankey-tradepathsankey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TradePathSankey.tsx:4</code> | <code>export function TradePathSankey()</code> |
| <code>interface.3.ts-perps-components-paxeer-tradercard-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TraderCard.tsx:72</code> | <code>export default TraderCard;</code> |
| <code>interface.3.ts-perps-components-paxeer-tradercard-tradercard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TraderCard.tsx:4</code> | <code>export function TraderCard()</code> |
| <code>interface.3.ts-perps-components-paxeer-tradestable-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TradesTable.tsx:55</code> | <code>export default TradesTable;</code> |
| <code>interface.3.ts-perps-components-paxeer-tradestable-tradestable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TradesTable.tsx:4</code> | <code>export function TradesTable()</code> |
| <code>interface.3.ts-perps-components-paxeer-transfermodal-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TransferModal.tsx:170</code> | <code>export default TransferModal;</code> |
| <code>interface.3.ts-perps-components-paxeer-transfermodal-transfermodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TransferModal.tsx:7</code> | <code>export function TransferModal({ initialMode = 'deposit' } = {})</code> |
| <code>interface.3.ts-perps-components-paxeer-transfershistory-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TransfersHistory.tsx:67</code> | <code>export default TransfersHistory;</code> |
| <code>interface.3.ts-perps-components-paxeer-transfershistory-transfershistory</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TransfersHistory.tsx:5</code> | <code>export function TransfersHistory()</code> |
| <code>interface.3.ts-perps-components-paxeer-txsimulator-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TxSimulator.tsx:85</code> | <code>export default TxSimulator;</code> |
| <code>interface.3.ts-perps-components-paxeer-txsimulator-txsimulator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/components/paxeer/TxSimulator.tsx:4</code> | <code>export function TxSimulator()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-components-paxeer-activeorders-activeorders</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-activeorders-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-addresschip-addresschip</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-addresschip-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-closedpositions-closedpositions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-closedpositions-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-commandpalette-commandpalette</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-commandpalette-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-confirmslider-confirmslider</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-confirmslider-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-cumulativepnl-cumulativepnl</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-cumulativepnl-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-datainput-datainput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-datainput-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-depthchart-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-depthchart-depthchart</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-epochcountdown-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-epochcountdown-epochcountdown</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-feesbreakdown-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-feesbreakdown-feesbreakdown</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-gasweather-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-gasweather-gasweather</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-gestureunlock-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-gestureunlock-gestureunlock</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-ghosthold-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-ghosthold-ghosthold</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-leaderboard-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-leaderboard-leaderboard</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-leaderboardstats-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-leaderboardstats-leaderboardstats</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-leveragedial-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-leveragedial-leveragedial</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-liquidstat-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-liquidstat-liquidstat</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-magneticswap-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-magneticswap-magneticswap</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-networkpill-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-networkpill-networkpill</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-orbitalselector-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-orbitalselector-orbitalselector</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-orderbook-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-orderbook-orderbook</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-orderbooktopo-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-orderbooktopo-orderbooktopo</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-orderentry-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-orderentry-orderentry</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-overview-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-overview-overview</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-performance-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-performance-performance</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-positioncard-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-positioncard-positioncard</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-pulsefeed-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-pulsefeed-pulsefeed</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-radialallocator-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-radialallocator-radialallocator</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-reactionrail-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-reactionrail-reactionrail</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-recenttrades-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-recenttrades-recenttrades</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-riskdial-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-riskdial-riskdial</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-shardstatus-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-shardstatus-shardstatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-shared-cx</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-shared-token</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-shared-tokenbadge</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-smartbutton-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-smartbutton-smartbutton</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-tradepathsankey-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-tradepathsankey-tradepathsankey</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-tradercard-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-tradercard-tradercard</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-tradestable-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-tradestable-tradestable</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-transfermodal-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-transfermodal-transfermodal</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-transfershistory-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-transfershistory-transfershistory</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-txsimulator-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-components-paxeer-txsimulator-txsimulator</code> | notes | no in-repo importer at HEAD |

## core

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-calls-buildgetfeetier-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/calls.ts</code> (planned) | <code>export function buildGetFeeTier(account: Address) { return call("getFeeTier", [requireAccount(account)] as const) }</code> |
| <code>interface.3.ts-perps-calls-buildgetmargintiers-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/calls.ts</code> (planned) | <code>export function buildGetMarginTiers(marketId: bigint) { return call("getMarginTiers", [marketId] as const) }</code> |
| <code>interface.3.ts-perps-calls-buildgetmarketconfig-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/calls.ts</code> (planned) | <code>export function buildGetMarketConfig(marketId: bigint) { return call("getMarketConfig", [marketId] as const) }</code> |
| <code>interface.3.ts-perps-calls-buildgetmarkpricedetails-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/calls.ts</code> (planned) | <code>export function buildGetMarkPriceDetails(marketId: bigint) { return call("getMarkPriceDetails", [marketId] as const) }</code> |
| <code>interface.3.ts-perps-core-chain-explorer-base-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/chain.ts:14</code> | <code>export const EXPLORER_BASE_URL = PAXSCAN_BASE_URL ?? MAINNET_EXPLORER;</code> |
| <code>interface.3.ts-perps-core-chain-paxeerchain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/chain.ts:21</code> | <code>export const paxeerChain = defineChain(</code> |
| <code>interface.3.ts-perps-core-datafeed-perps-perpsdatafeed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/datafeed/perps.ts:60</code> | <code>export class PerpsDatafeed implements IBasicDataFeed</code> |
| <code>interface.3.ts-perps-core-errors-catalog-error-catalog</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/catalog.ts:19</code> | <code>export const ERROR_CATALOG: Record&lt;ErrorCode, CatalogEntry&gt; =</code> |
| <code>interface.3.ts-perps-core-errors-catalog-getcatalogentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/catalog.ts:428</code> | <code>export function getCatalogEntry(code: ErrorCode &#124; undefined &#124; null): CatalogEntry</code> |
| <code>interface.3.ts-perps-core-errors-classify-classifyerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/classify.ts:468</code> | <code>export function classifyError(raw: unknown, ctx?: ErrorContext): UserFacingError</code> |
| <code>interface.3.ts-perps-core-errors-classify-isuserfacingerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/classify.ts:529</code> | <code>export function isUserFacingError(v: unknown): v is UserFacingError</code> |
| <code>interface.3.ts-perps-core-errors-classify-makeerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/classify.ts:515</code> | <code>export function makeError(code: ErrorCode, ctx?: ErrorContext, cause?: unknown): UserFacingError</code> |
| <code>interface.3.ts-perps-core-errors-classify-precompile-error-codes</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/core/errors/classify.ts:311</code> | <code>export const PRECOMPILE_ERROR_CODES: Readonly&lt;Record&lt;PrecompileErrorName, PrecompileErrorEntry&gt;&gt; =</code> |
| <code>interface.3.ts-perps-core-errors-classify-precompileerrorentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/classify.ts:281</code> | <code>export interface PrecompileErrorEntry</code> |
| <code>interface.3.ts-perps-core-errors-classify-precompileerrorname</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/classify.ts:279</code> | <code>export type PrecompileErrorName = Extract&lt;(typeof PRECOMPILE_ABI)[number], { type: "error" }&gt;["name"];</code> |
| <code>interface.3.ts-perps-core-errors-global-installglobalerrorhandlers</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/global.ts:33</code> | <code>export function installGlobalErrorHandlers(): () =&gt; void</code> |
| <code>interface.3.ts-perps-core-errors-report-leavebreadcrumb</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/report.ts:118</code> | <code>export async function leaveBreadcrumb( category: string, message: string, data?: Record&lt;string, unknown&gt;, ): Promise&lt;void&gt;</code> |
| <code>interface.3.ts-perps-core-errors-report-reporterror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/report.ts:48</code> | <code>export async function reportError( err: unknown, opts: ReportErrorOptions = {}, ): Promise&lt;void&gt;</code> |
| <code>interface.3.ts-perps-core-errors-types-errorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/types.ts:19</code> | <code>export type ErrorCode =</code> |
| <code>interface.3.ts-perps-core-errors-types-errorcontext</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/types.ts:77</code> | <code>export type ErrorContext = string;</code> |
| <code>interface.3.ts-perps-core-errors-types-errorseverity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/types.ts:12</code> | <code>export type ErrorSeverity = "info" &#124; "warning" &#124; "error" &#124; "fatal";</code> |
| <code>interface.3.ts-perps-core-errors-types-handleerroroptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/types.ts:136</code> | <code>export interface HandleErrorOptions extends ReportErrorOptions</code> |
| <code>interface.3.ts-perps-core-errors-types-reporterroroptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/types.ts:121</code> | <code>export interface ReportErrorOptions</code> |
| <code>interface.3.ts-perps-core-errors-types-userfacingerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/errors/types.ts:80</code> | <code>export interface UserFacingError</code> |
| <code>interface.3.ts-perps-core-feeds-candlefeedurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/feeds.ts:8</code> | <code>export function candleFeedUrl(endpoint: "config" &#124; "symbols" &#124; "history", symbol = "SID", params?: URLSearchParams): string</code> |
| <code>interface.3.ts-perps-core-feeds-feed-origin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/feeds.ts:2</code> | <code>export const FEED_ORIGIN = `&lt;url&gt; &#124;&#124; "&lt;host&gt;"}`;</code> |
| <code>interface.3.ts-perps-core-feeds-feed-symbols</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/feeds.ts:4</code> | <code>export const FEED_SYMBOLS = ["BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "TRUMP", "BNB", "SID", "PAX", "TSLA", "NVDA", "GOOGL", "XAU", "NAS100", "SPX500"] as const;</code> |
| <code>interface.3.ts-perps-core-feeds-market-data-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/feeds.ts:3</code> | <code>export const MARKET_DATA_URL = `${FEED_ORIGIN}/api/markets`;</code> |
| <code>interface.3.ts-perps-core-feeds-normalizefeedsymbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/feeds.ts:5</code> | <code>export function normalizeFeedSymbol(symbol: string): string</code> |
| <code>interface.3.ts-perps-core-graphql-client-gqlendpoint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/graphql/client.ts:59</code> | <code>export function gqlEndpoint(url: string, opts: Partial&lt;GraphQLEndpoint&gt; = {}): GraphQLEndpoint</code> |
| <code>interface.3.ts-perps-core-graphql-client-gqlquery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/graphql/client.ts:113</code> | <code>export function gqlQuery&lt;T&gt;( endpoint: GraphQLEndpoint, query: string, variables?: Record&lt;string, unknown&gt;, ): Promise&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-graphql-client-graphqlendpoint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/graphql/client.ts:45</code> | <code>export interface GraphQLEndpoint</code> |
| <code>interface.3.ts-perps-core-graphql-client-polledsubscriptionoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/graphql/client.ts:132</code> | <code>export interface PolledSubscriptionOptions&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-graphql-client-streamsubscriptionoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/graphql/client.ts:419</code> | <code>export interface StreamSubscriptionOptions&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-graphql-client-subscribequery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/graphql/client.ts:230</code> | <code>export function subscribeQuery&lt;T&gt;(opts: PolledSubscriptionOptions&lt;T&gt;): () =&gt; void</code> |
| <code>interface.3.ts-perps-core-graphql-client-subscribestream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/graphql/client.ts:435</code> | <code>export function subscribeStream&lt;T&gt;(opts: StreamSubscriptionOptions&lt;T&gt;): () =&gt; void</code> |
| <code>interface.3.ts-perps-core-log-logger-createlogger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/log/logger.ts:65</code> | <code>export function createLogger(ns: string): Logger</code> |
| <code>interface.3.ts-perps-core-log-logger-logevent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/log/logger.ts:19</code> | <code>export type LogEvent =</code> |
| <code>interface.3.ts-perps-core-log-logger-logger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/log/logger.ts:57</code> | <code>export interface Logger</code> |
| <code>interface.3.ts-perps-core-log-logger-loglevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/log/logger.ts:16</code> | <code>export type LogLevel = "debug" &#124; "info" &#124; "warn" &#124; "error";</code> |
| <code>interface.3.ts-perps-core-log-logger-logsink</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/log/logger.ts:26</code> | <code>export type LogSink = (event: LogEvent) =&gt; void;</code> |
| <code>interface.3.ts-perps-core-log-logger-setsink</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/log/logger.ts:48</code> | <code>export function setSink(sink: LogSink): void</code> |
| <code>interface.3.ts-perps-core-nav-isspotopen</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:16</code> | <code>export function isSpotOpen( value: string &#124; undefined = process.env.NEXT_PUBLIC_RELEASE_SPOT_OPEN, ): boolean</code> |
| <code>interface.3.ts-perps-core-nav-nav-group-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:93</code> | <code>export const NAV_GROUP_LABELS: Record&lt;NavGroup, string&gt; =</code> |
| <code>interface.3.ts-perps-core-nav-nav-group-order</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:101</code> | <code>export const NAV_GROUP_ORDER: NavGroup[] = ["trade", "account"];</code> |
| <code>interface.3.ts-perps-core-nav-nav-items</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:87</code> | <code>export const NAV_ITEMS: NavItem[] = navItems();</code> |
| <code>interface.3.ts-perps-core-nav-navgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:2</code> | <code>export type NavGroup = "trade" &#124; "account";</code> |
| <code>interface.3.ts-perps-core-nav-navitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:4</code> | <code>export interface NavItem</code> |
| <code>interface.3.ts-perps-core-nav-navitems</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:29</code> | <code>export function navItems(spotOpen: boolean = isSpotOpen()): NavItem[]</code> |
| <code>interface.3.ts-perps-core-nav-navsurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:1</code> | <code>export type NavSurface = "desktopPro" &#124; "mobileDrawer";</code> |
| <code>interface.3.ts-perps-core-net-fetch-fetcherrorkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/fetch.ts:2</code> | <code>export type FetchErrorKind = "timeout" &#124; "abort" &#124; "network" &#124; "http" &#124; "parse" &#124; "unknown";</code> |
| <code>interface.3.ts-perps-core-net-fetch-fetchjsonwithpolicy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/fetch.ts:110</code> | <code>export async function fetchJsonWithPolicy&lt;T = unknown&gt;(url: string, policy: FetchPolicy = {}): Promise&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-net-fetch-fetchpolicy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/fetch.ts:13</code> | <code>export interface FetchPolicy extends Omit&lt;RequestInit, "body"&gt;</code> |
| <code>interface.3.ts-perps-core-net-fetch-fetchpolicyerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/fetch.ts:3</code> | <code>export class FetchPolicyError extends Error</code> |
| <code>interface.3.ts-perps-core-net-fetch-fetchwithpolicy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/fetch.ts:36</code> | <code>export async function fetchWithPolicy(input: string &#124; URL &#124; Request, policy: FetchPolicy = {}): Promise&lt;Response&gt;</code> |
| <code>interface.3.ts-perps-core-net-fetch-retryafterms</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/fetch.ts:22</code> | <code>export function retryAfterMs(value: string &#124; null, now = Date.now()): number</code> |
| <code>interface.3.ts-perps-core-net-realtime-activepoll</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:90</code> | <code>export function activePoll(task: () =&gt; unknown &#124; Promise&lt;unknown&gt;, intervalMs: number, immediate = true): CancelTask</code> |
| <code>interface.3.ts-perps-core-net-realtime-canceltask</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:2</code> | <code>export type CancelTask = () =&gt; void;</code> |
| <code>interface.3.ts-perps-core-net-realtime-ispageactive</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:3</code> | <code>export const isPageActive = () =&gt; typeof document === 'undefined' &#124;&#124; (!document.hidden &amp;&amp; (typeof navigator === 'undefined' &#124;&#124; navigator.onLine !== false));</code> |
| <code>interface.3.ts-perps-core-net-realtime-jitter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:4</code> | <code>export const jitter = (ceiling: number, random = Math.random) =&gt; Math.floor(random() * Math.min(60000, Math.max(0, ceiling)));</code> |
| <code>interface.3.ts-perps-core-net-realtime-notifystreamresync</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:26</code> | <code>export function notifyStreamResync(url: string)</code> |
| <code>interface.3.ts-perps-core-net-realtime-onstreamresync</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:25</code> | <code>export function onStreamResync(callback: (url: string) =&gt; void): CancelTask</code> |
| <code>interface.3.ts-perps-core-net-realtime-scheduleactive</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:8</code> | <code>export function scheduleActive(task: () =&gt; void, ceiling = 1000, url = ''): CancelTask</code> |
| <code>interface.3.ts-perps-core-net-realtime-sequenceguard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:29</code> | <code>export function sequenceGuard()</code> |
| <code>interface.3.ts-perps-core-net-realtime-watchsocket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/realtime.ts:50</code> | <code>export function watchSocket(ws: WebSocket, onStable: () =&gt; void, ping?: unknown, requireData = false): void</code> |
| <code>interface.3.ts-perps-core-net-venuestream-fill-cap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:43</code> | <code>export const FILL_CAP = 100;</code> |
| <code>interface.3.ts-perps-core-net-venuestream-gap-close-code</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:45</code> | <code>export const GAP_CLOSE_CODE = 4000;</code> |
| <code>interface.3.ts-perps-core-net-venuestream-isspotvenuesymbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:84</code> | <code>export function isSpotVenueSymbol(symbol: string &#124; null &#124; undefined): boolean</code> |
| <code>interface.3.ts-perps-core-net-venuestream-isvenuesymbol</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/core/net/venueStream.ts:80</code> | <code>export function isVenueSymbol(symbol: string &#124; null &#124; undefined): boolean</code> |
| <code>interface.3.ts-perps-core-net-venuestream-max-subscriptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:42</code> | <code>export const MAX_SUBSCRIPTIONS = 200;</code> |
| <code>interface.3.ts-perps-core-net-venuestream-parsespotvenuetrade</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:150</code> | <code>export function parseSpotVenueTrade(raw: unknown): SpotVenueTrade &#124; null</code> |
| <code>interface.3.ts-perps-core-net-venuestream-parsevenuebook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:116</code> | <code>export function parseVenueBook(data: unknown, ts: number): VenueBook &#124; null</code> |
| <code>interface.3.ts-perps-core-net-venuestream-parsevenuefill</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:130</code> | <code>export function parseVenueFill(raw: unknown): VenueFill &#124; null</code> |
| <code>interface.3.ts-perps-core-net-venuestream-perpbooktopic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:88</code> | <code>export function perpBookTopic(symbol: string): string</code> |
| <code>interface.3.ts-perps-core-net-venuestream-perptradetopic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:92</code> | <code>export function perpTradeTopic(symbol: string): string</code> |
| <code>interface.3.ts-perps-core-net-venuestream-resync-depth</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:44</code> | <code>export const RESYNC_DEPTH = 100;</code> |
| <code>interface.3.ts-perps-core-net-venuestream-spotbooktopic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:96</code> | <code>export function spotBookTopic(symbol: string): string</code> |
| <code>interface.3.ts-perps-core-net-venuestream-spottradetopic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:100</code> | <code>export function spotTradeTopic(symbol: string): string</code> |
| <code>interface.3.ts-perps-core-net-venuestream-spotvenuetrade</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:32</code> | <code>export interface SpotVenueTrade</code> |
| <code>interface.3.ts-perps-core-net-venuestream-subscribespotvenuebook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:395</code> | <code>export function subscribeSpotVenueBook(symbol: string, listener: (book: VenueBook &#124; null) =&gt; void): CancelTask</code> |
| <code>interface.3.ts-perps-core-net-venuestream-subscribespotvenuetrades</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:400</code> | <code>export function subscribeSpotVenueTrades(symbol: string, listener: (trades: SpotVenueTrade[] &#124; null) =&gt; void): CancelTask</code> |
| <code>interface.3.ts-perps-core-net-venuestream-subscribevenuebook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:367</code> | <code>export function subscribeVenueBook(symbol: string, listener: (book: VenueBook &#124; null) =&gt; void): CancelTask</code> |
| <code>interface.3.ts-perps-core-net-venuestream-subscribevenuefills</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:372</code> | <code>export function subscribeVenueFills(symbol: string, listener: (fills: VenueFill[] &#124; null) =&gt; void): CancelTask</code> |
| <code>interface.3.ts-perps-core-net-venuestream-usespotvenuebook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:404</code> | <code>export function useSpotVenueBook(symbol: string &#124; undefined): VenueBook &#124; null</code> |
| <code>interface.3.ts-perps-core-net-venuestream-usespotvenuetrades</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:413</code> | <code>export function useSpotVenueTrades(symbol: string &#124; undefined): SpotVenueTrade[] &#124; null</code> |
| <code>interface.3.ts-perps-core-net-venuestream-usevenuebook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:376</code> | <code>export function useVenueBook(symbol: string &#124; undefined): VenueBook &#124; null</code> |
| <code>interface.3.ts-perps-core-net-venuestream-usevenuefills</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:385</code> | <code>export function useVenueFills(symbol: string &#124; undefined): VenueFill[] &#124; null</code> |
| <code>interface.3.ts-perps-core-net-venuestream-venuebook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:13</code> | <code>export interface VenueBook</code> |
| <code>interface.3.ts-perps-core-net-venuestream-venuefill</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:22</code> | <code>export interface VenueFill</code> |
| <code>interface.3.ts-perps-core-net-venuestream-venueframe</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:40</code> | <code>export type VenueFrame = { event: "subscribe" &#124; "unsubscribe"; topic: string };</code> |
| <code>interface.3.ts-perps-core-net-venuestream-venuelevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/net/venueStream.ts:11</code> | <code>export type VenueLevel = SpotBookLevel;</code> |
| <code>interface.3.ts-perps-core-network-abi-ierc20-ierc20abi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/IERC20.js:201</code> | <code>module.exports = { IERC20ABI };</code> |
| <code>interface.3.ts-perps-core-network-abi-ierc20-ierc20abi-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/IERC20.ts:2</code> | <code>export const IERC20ABI = [</code> |
| <code>interface.3.ts-perps-core-network-abi-index-ierc20abi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/index.js:5</code> | <code>module.exports = {</code> |
| <code>interface.3.ts-perps-core-network-abi-index-ordertype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/index.js:5</code> | <code>module.exports = {</code> |
| <code>interface.3.ts-perps-core-network-abi-index-perpsprecompileabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/index.js:5</code> | <code>module.exports = {</code> |
| <code>interface.3.ts-perps-core-network-abi-index-timeinforce</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/index.js:5</code> | <code>module.exports = {</code> |
| <code>interface.3.ts-perps-core-network-abi-perpsprecompile-ordertype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/PerpsPrecompile.ts:3447</code> | <code>export const OrderType =</code> |
| <code>interface.3.ts-perps-core-network-abi-perpsprecompile-ordertype-type</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/PerpsPrecompile.ts:3455</code> | <code>export type OrderType = (typeof OrderType)[keyof typeof OrderType];</code> |
| <code>interface.3.ts-perps-core-network-abi-perpsprecompile-perpsprecompileabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/PerpsPrecompile.ts:2</code> | <code>export const PerpsPrecompileABI = [</code> |
| <code>interface.3.ts-perps-core-network-abi-perpsprecompile-timeinforce</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/PerpsPrecompile.ts:3457</code> | <code>export const TimeInForce =</code> |
| <code>interface.3.ts-perps-core-network-abi-perpsprecompile-timeinforce-type</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/abi/PerpsPrecompile.ts:3463</code> | <code>export type TimeInForce = (typeof TimeInForce)[keyof typeof TimeInForce];</code> |
| <code>interface.3.ts-perps-core-network-approvals-approvalplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:27</code> | <code>export type ApprovalPlan = readonly ApproveCall[];</code> |
| <code>interface.3.ts-perps-core-network-approvals-approvaltoken</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:16</code> | <code>export type ApprovalToken = Address &#124; null;</code> |
| <code>interface.3.ts-perps-core-network-approvals-approvecall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:21</code> | <code>export interface ApproveCall</code> |
| <code>interface.3.ts-perps-core-network-approvals-launchpad-spender</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:6</code> | <code>export const LAUNCHPAD_SPENDER: Address = PRECOMPILES.launchpad;</code> |
| <code>interface.3.ts-perps-core-network-approvals-nativetokens</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:19</code> | <code>export type NativeTokens = ReadonlySet&lt;Address&gt;;</code> |
| <code>interface.3.ts-perps-core-network-approvals-needsapproval</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:44</code> | <code>export function needsApproval( token: ApprovalToken, allowance: bigint, amount: bigint, native: NativeTokens, ): boolean</code> |
| <code>interface.3.ts-perps-core-network-approvals-perps-spender</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:5</code> | <code>export const PERPS_SPENDER: Address = PRECOMPILES.perps;</code> |
| <code>interface.3.ts-perps-core-network-approvals-planapproval</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:59</code> | <code>export async function planApproval( token: ApprovalToken, spender: Address, allowance: () =&gt; Promise&lt;bigint&gt;, amount: bigint, native: NativeTokens, ): Promise&lt;ApprovalPlan&gt;</code> |
| <code>interface.3.ts-perps-core-network-approvals-spot-spenders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/approvals.ts:7</code> | <code>export const SPOT_SPENDERS =</code> |
| <code>interface.3.ts-perps-core-network-gas-gaswithmargin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/gas.ts:1</code> | <code>export function gasWithMargin(estimate: bigint): bigint</code> |
| <code>interface.3.ts-perps-core-network-gateway-gatewayurls</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/gateway.ts:3</code> | <code>export interface GatewayUrls</code> |
| <code>interface.3.ts-perps-core-network-gateway-gatewayurls-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/gateway.ts:44</code> | <code>export function gatewayUrls(): GatewayUrls</code> |
| <code>interface.3.ts-perps-core-network-precompiles-precompiles</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/precompiles.ts:12</code> | <code>export const PRECOMPILES =</code> |
| <code>interface.3.ts-perps-core-network-spot-abi-pecor-pecorabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/spot_abi/PECOR.ts:5</code> | <code>export const pecorAbi = [</code> |
| <code>interface.3.ts-perps-core-network-spot-abi-pecororders-pecorordersabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/spot_abi/PECOROrders.ts:5</code> | <code>export const pecorOrdersAbi = [</code> |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorquoter-pecorquoterabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/spot_abi/PECORQuoter.ts:5</code> | <code>export const pecorQuoterAbi = [</code> |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorstoporders-pecorstopordersabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/spot_abi/PECORStopOrders.ts:5</code> | <code>export const pecorStopOrdersAbi = [</code> |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorvault-pecorvaultabi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/network/spot_abi/PECORVault.ts:5</code> | <code>export const pecorVaultAbi = [</code> |
| <code>interface.3.ts-perps-core-paxscan-api-buildurl-buildurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/buildUrl.ts:47</code> | <code>export function buildUrl&lt;R extends ResourceName&gt;( resource: R, pathParams?: PathParams&lt;R&gt;, queryParams?: QueryParams&lt;R&gt;, ): string</code> |
| <code>interface.3.ts-perps-core-paxscan-api-fetch-paxscanerror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/fetch.ts:12</code> | <code>export class PaxscanError extends Error</code> |
| <code>interface.3.ts-perps-core-paxscan-api-fetch-paxscanfetch</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/fetch.ts:23</code> | <code>export async function paxscanFetch&lt;T&gt;(url: string, init?: RequestInit): Promise&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-paxscan-api-resources-apiresource</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/resources.ts:39</code> | <code>export interface ApiResource</code> |
| <code>interface.3.ts-perps-core-paxscan-api-resources-paginatedresourcename</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/resources.ts:193</code> | <code>export type PaginatedResourceName =</code> |
| <code>interface.3.ts-perps-core-paxscan-api-resources-pathparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/resources.ts:229</code> | <code>export type PathParams&lt;R extends ResourceName&gt; = R extends keyof PathParamsMap ? PathParamsMap[R] : never;</code> |
| <code>interface.3.ts-perps-core-paxscan-api-resources-queryparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/resources.ts:237</code> | <code>export type QueryParams&lt;_R extends ResourceName = ResourceName&gt; =</code> |
| <code>interface.3.ts-perps-core-paxscan-api-resources-resourcename</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/resources.ts:191</code> | <code>export type ResourceName = keyof typeof RESOURCES;</code> |
| <code>interface.3.ts-perps-core-paxscan-api-resources-resourcepayload</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/resources.ts:274</code> | <code>export type ResourcePayload&lt;R extends ResourceName&gt; = R extends keyof PayloadMap ? PayloadMap[R] : never;</code> |
| <code>interface.3.ts-perps-core-paxscan-api-resources-resources</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/api/resources.ts:51</code> | <code>export const RESOURCES =</code> |
| <code>interface.3.ts-perps-core-paxscan-config-paxscan-api-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/config.ts:20</code> | <code>export const PAXSCAN_API_URL = process.env.NEXT_PUBLIC_PAXSCAN_API_URL ?? '';</code> |
| <code>interface.3.ts-perps-core-paxscan-config-paxscan-base-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/config.ts:22</code> | <code>export const PAXSCAN_BASE_URL = parseBaseUrl(process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL);</code> |
| <code>interface.3.ts-perps-core-paxscan-config-paxscan-ws-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/config.ts:21</code> | <code>export const PAXSCAN_WS_URL = process.env.NEXT_PUBLIC_PAXSCAN_WS_URL ?? '';</code> |
| <code>interface.3.ts-perps-core-paxscan-socket-context-paxscansocketprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/socket/context.tsx:53</code> | <code>export function PaxscanSocketProvider({ children, options, url, name = 'default' }: PaxscanSocketProviderProps)</code> |
| <code>interface.3.ts-perps-core-paxscan-socket-context-usepaxscansocket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/socket/context.tsx:99</code> | <code>export function usePaxscanSocket(name: string = 'default')</code> |
| <code>interface.3.ts-perps-core-paxscan-socket-types-socketmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/socket/types.ts:57</code> | <code>export namespace SocketMessage</code> |
| <code>interface.3.ts-perps-core-paxscan-socket-types-socketmessageparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/socket/types.ts:19</code> | <code>export type SocketMessageParams =</code> |
| <code>interface.3.ts-perps-core-paxscan-socket-usesocketchannel-usepaxscansocketchannel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/socket/useSocketChannel.tsx:16</code> | <code>export function usePaxscanSocketChannel({ topic, params, isDisabled, onJoin, onSocketClose, onSocketError, socketName }: Params)</code> |
| <code>interface.3.ts-perps-core-paxscan-socket-usesocketmessage-usepaxscansocketmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/socket/useSocketMessage.tsx:5</code> | <code>export function usePaxscanSocketMessage({ channel, event, handler }: SocketMessageParams)</code> |
| <code>interface.3.ts-perps-core-paxscan-urls-paxscanaddressurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/urls.ts:4</code> | <code>export const paxscanAddressUrl = (addr: string) =&gt; `${ EXPLORER_BASE_URL }/address/${ addr }`;</code> |
| <code>interface.3.ts-perps-core-paxscan-urls-paxscanblockurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/urls.ts:5</code> | <code>export const paxscanBlockUrl = (n: number &#124; string) =&gt; `${ EXPLORER_BASE_URL }/block/${ n }`;</code> |
| <code>interface.3.ts-perps-core-paxscan-urls-paxscansearchurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/urls.ts:7</code> | <code>export const paxscanSearchUrl = (q: string) =&gt; `${ EXPLORER_BASE_URL }/search-results?q=${ encodeURIComponent(q) }`;</code> |
| <code>interface.3.ts-perps-core-paxscan-urls-paxscantokenurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/urls.ts:6</code> | <code>export const paxscanTokenUrl = (addr: string) =&gt; `${ EXPLORER_BASE_URL }/token/${ addr }`;</code> |
| <code>interface.3.ts-perps-core-paxscan-urls-paxscantxurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/urls.ts:3</code> | <code>export const paxscanTxUrl = (hash: string) =&gt; `${ EXPLORER_BASE_URL }/tx/${ hash }`;</code> |
| <code>interface.3.ts-perps-core-perfdb-client-getauthheaders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:43</code> | <code>export function getAuthHeaders(): Record&lt;string, string&gt;</code> |
| <code>interface.3.ts-perps-core-perfdb-client-getauthtoken</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:27</code> | <code>export function getAuthToken(): string &#124; null</code> |
| <code>interface.3.ts-perps-core-perfdb-client-perfdb-graphql</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:116</code> | <code>export const PERFDB_GRAPHQL: GraphQLEndpoint = gqlEndpoint(</code> |
| <code>interface.3.ts-perps-core-perfdb-client-perfdb-rest-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:15</code> | <code>export const PERFDB_REST_URL = PERFDB_URL;</code> |
| <code>interface.3.ts-perps-core-perfdb-client-perfdb-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:13</code> | <code>export const PERFDB_URL = gatewayUrls().perfdbOrigin;</code> |
| <code>interface.3.ts-perps-core-perfdb-client-perfdb-ws-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:17</code> | <code>export const PERFDB_WS_URL = PERFDB_URL.replace(/^https/, "wss").replace(</code> |
| <code>interface.3.ts-perps-core-perfdb-client-perfdbquery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:50</code> | <code>export async function perfdbQuery&lt;T&gt;( document: string, variables?: Record&lt;string, unknown&gt; ): Promise&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-perfdb-client-perfdbqueryauth</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:66</code> | <code>export async function perfdbQueryAuth&lt;T&gt;( document: string, variables?: Record&lt;string, unknown&gt; ): Promise&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-perfdb-client-perfdbrest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:85</code> | <code>export async function perfdbRest&lt;T&gt;( path: string, options?: RequestInit ): Promise&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-perfdb-client-perfdbrestauth</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perfdb/client.ts:97</code> | <code>export async function perfdbRestAuth&lt;T&gt;( path: string, options?: RequestInit ): Promise&lt;T&gt;</code> |
| <code>interface.3.ts-perps-core-perps-accountview-accountview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/accountView.ts:7</code> | <code>export interface AccountView</code> |
| <code>interface.3.ts-perps-core-perps-accountview-accountviewfromstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/accountView.ts:20</code> | <code>export function accountViewFromState(accountState: AccountState): AccountView</code> |
| <code>interface.3.ts-perps-core-perps-accountview-pickcollateral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/accountView.ts:39</code> | <code>export function pickCollateral(accountState: Pick&lt;AccountState, "collateral"&gt;): PickedCollateral</code> |
| <code>interface.3.ts-perps-core-perps-accountview-pickedcollateral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/accountView.ts:31</code> | <code>export interface PickedCollateral</code> |
| <code>interface.3.ts-perps-core-perps-amounts-deriveimpliedcollateral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/amounts.ts:58</code> | <code>export function deriveImpliedCollateral( sizeUsd1e18: bigint, leverage1e18: bigint, collateralDecimals: number, ): bigint</code> |
| <code>interface.3.ts-perps-core-perps-amounts-getcollateraldecimals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/amounts.ts:40</code> | <code>export function getCollateralDecimals(tokenAddress: string): number</code> |
| <code>interface.3.ts-perps-core-perps-amounts-safeparseunits</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/amounts.ts:26</code> | <code>export function safeParseUnits(value: string &#124; undefined &#124; null, decimals: number): bigint</code> |
| <code>interface.3.ts-perps-core-perps-booklevels-normalizelevels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/bookLevels.ts:29</code> | <code>export function normalizeLevels(raw: unknown, side: "asks" &#124; "bids"): SpotBookLevel[]</code> |
| <code>interface.3.ts-perps-core-perps-booklevels-spotbooklevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/bookLevels.ts:1</code> | <code>export interface SpotBookLevel</code> |
| <code>interface.3.ts-perps-core-perps-calls-acceptableprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:97</code> | <code>export function acceptablePrice( price: bigint, isLong: boolean, action: "open" &#124; "close", bps: number = DEFAULT_SLIPPAGE_BPS, ): bigint</code> |
| <code>interface.3.ts-perps-core-perps-calls-accountstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:44</code> | <code>export type AccountState = ContractFunctionReturnType&lt;PerpsAbi, "view", "getAccount"&gt;;</code> |
| <code>interface.3.ts-perps-core-perps-calls-addsizeinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:186</code> | <code>export interface AddSizeInput</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildaddcollateral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:270</code> | <code>export function buildAddCollateral(positionId: bigint, amount: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildaddsize</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:195</code> | <code>export function buildAddSize(input: AddSizeInput)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildcancelbycloid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:350</code> | <code>export function buildCancelByCloid(marketId: bigint, cloid: Hex)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildcancelorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:346</code> | <code>export function buildCancelOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildcloseposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:226</code> | <code>export function buildClosePosition(input: ClosePositionInput)</code> |
| <code>interface.3.ts-perps-core-perps-calls-builddeposit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:357</code> | <code>export function buildDeposit(amount: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-builddepositplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:369</code> | <code>export async function buildDepositPlan( amount: bigint, allowance: () =&gt; Promise&lt;bigint&gt;, native: NativeTokens, ): Promise&lt;DepositPlan&gt;</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:386</code> | <code>export function buildGetAccount(account: Address)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetaccountsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:390</code> | <code>export function buildGetAccountSummary(account: Address, token: Address)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:394</code> | <code>export function buildGetBalance(account: Address, token: Address)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetlockedbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:398</code> | <code>export function buildGetLockedBalance(account: Address, token: Address)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:418</code> | <code>export function buildGetOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:406</code> | <code>export function buildGetPosition(positionId: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetpositiondetails</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:410</code> | <code>export function buildGetPositionDetails(positionId: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetpositionmargin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:402</code> | <code>export function buildGetPositionMargin(positionId: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetuserorderids</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:422</code> | <code>export function buildGetUserOrderIds(account: Address)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildgetuserpositionids</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:414</code> | <code>export function buildGetUserPositionIds(account: Address)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildopenposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:160</code> | <code>export function buildOpenPosition(input: OpenPositionInput)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildpartialclose</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:244</code> | <code>export function buildPartialClose(input: PartialCloseInput)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildplacelimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:338</code> | <code>export function buildPlaceLimitOrder(input: LimitOrderInput)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildplacestoplimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:342</code> | <code>export function buildPlaceStopLimitOrder(input: StopLimitOrderInput)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildremovecollateral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:275</code> | <code>export function buildRemoveCollateral(positionId: bigint, amount: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildsetpositiontpsl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:262</code> | <code>export function buildSetPositionTpSl(input: SetPositionTpSlInput)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildtopupposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:280</code> | <code>export function buildTopUpPosition(positionId: bigint, amount: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildwithdraw</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:362</code> | <code>export function buildWithdraw(amount: bigint)</code> |
| <code>interface.3.ts-perps-core-perps-calls-buildwithdrawall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:380</code> | <code>export function buildWithdrawAll(accountState: Pick&lt;AccountState, "withdrawable"&gt;)</code> |
| <code>interface.3.ts-perps-core-perps-calls-closepositioninput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:219</code> | <code>export interface ClosePositionInput</code> |
| <code>interface.3.ts-perps-core-perps-calls-depositplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:366</code> | <code>export type DepositPlan = readonly (ApproveCall &#124; ReturnType&lt;typeof buildDeposit&gt;)[];</code> |
| <code>interface.3.ts-perps-core-perps-calls-laddercloid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:113</code> | <code>export function ladderCloid(ladderId: bigint, rung: number): Hex</code> |
| <code>interface.3.ts-perps-core-perps-calls-limitorderinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:286</code> | <code>export interface LimitOrderInput</code> |
| <code>interface.3.ts-perps-core-perps-calls-openpositioninput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:147</code> | <code>export interface OpenPositionInput</code> |
| <code>interface.3.ts-perps-core-perps-calls-openquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:140</code> | <code>export interface OpenQuote</code> |
| <code>interface.3.ts-perps-core-perps-calls-orderparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:43</code> | <code>export type OrderParams = ContractFunctionArgs&lt;PerpsAbi, "nonpayable", "placeOrder"&gt;[0];</code> |
| <code>interface.3.ts-perps-core-perps-calls-partialcloseinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:235</code> | <code>export interface PartialCloseInput</code> |
| <code>interface.3.ts-perps-core-perps-calls-perpscall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:29</code> | <code>export interface PerpsCall&lt;</code> |
| <code>interface.3.ts-perps-core-perps-calls-setpositiontpslinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:253</code> | <code>export interface SetPositionTpSlInput</code> |
| <code>interface.3.ts-perps-core-perps-calls-stoplimitorderinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/calls.ts:304</code> | <code>export interface StopLimitOrderInput extends LimitOrderInput</code> |
| <code>interface.3.ts-perps-core-perps-collateralinputs-tocollateralunits</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/collateralInputs.ts:4</code> | <code>export function toCollateralUnits(text: string): bigint</code> |
| <code>interface.3.ts-perps-core-perps-config-analytics-axis-formatters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:233</code> | <code>export const ANALYTICS_AXIS_FORMATTERS =</code> |
| <code>interface.3.ts-perps-core-perps-config-analytics-chart-colors</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:222</code> | <code>export const ANALYTICS_CHART_COLORS =</code> |
| <code>interface.3.ts-perps-core-perps-config-coinlogourl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:136</code> | <code>export function coinLogoUrl(symbol: string): string</code> |
| <code>interface.3.ts-perps-core-perps-config-collateral-options</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:40</code> | <code>export const COLLATERAL_OPTIONS = Object.entries(COLLATERAL_TOKENS).map(</code> |
| <code>interface.3.ts-perps-core-perps-config-collateral-tokens</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:32</code> | <code>export const COLLATERAL_TOKENS =</code> |
| <code>interface.3.ts-perps-core-perps-config-default-slippage-bps</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:264</code> | <code>export const DEFAULT_SLIPPAGE_BPS = 50; // 0.5%</code> |
| <code>interface.3.ts-perps-core-perps-config-erc20abi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:142</code> | <code>export const erc20Abi = [</code> |
| <code>interface.3.ts-perps-core-perps-config-explorer-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:20</code> | <code>export const EXPLORER_URL = paxeerChain.blockExplorers.default.url;</code> |
| <code>interface.3.ts-perps-core-perps-config-featured-market-symbols</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:255</code> | <code>export const FEATURED_MARKET_SYMBOLS: readonly string[] = [</code> |
| <code>interface.3.ts-perps-core-perps-config-fee-type-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:169</code> | <code>export const FEE_TYPE_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-fromorderlysymbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:27</code> | <code>export function fromOrderlySymbol(orderlySymbol: string): string</code> |
| <code>interface.3.ts-perps-core-perps-config-getmarketcategory</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:94</code> | <code>export function getMarketCategory(marketId: number): string</code> |
| <code>interface.3.ts-perps-core-perps-config-indexer-graphql</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:15</code> | <code>export const INDEXER_GRAPHQL: GraphQLEndpoint = gqlEndpoint(gatewayUrls().graphql.url,</code> |
| <code>interface.3.ts-perps-core-perps-config-margin-mode-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:197</code> | <code>export const MARGIN_MODE_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-margin-mode-toggle-copy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:287</code> | <code>export const MARGIN_MODE_TOGGLE_COPY =</code> |
| <code>interface.3.ts-perps-core-perps-config-market-categories</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:79</code> | <code>export const MARKET_CATEGORIES: Record&lt;string, number[]&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-market-category-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:86</code> | <code>export const MARKET_CATEGORY_LABELS: Record&lt;string, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-market-logo-files</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:125</code> | <code>export const MARKET_LOGO_FILES: Readonly&lt;Record&lt;string, string&gt;&gt; = Object.freeze(</code> |
| <code>interface.3.ts-perps-core-perps-config-market-logo-placeholder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:134</code> | <code>export const MARKET_LOGO_PLACEHOLDER = "/market-logos/generic.svg";</code> |
| <code>interface.3.ts-perps-core-perps-config-market-logo-symbols</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:132</code> | <code>export const MARKET_LOGO_SYMBOLS: ReadonlySet&lt;string&gt; = new Set(Object.keys(MARKET_LOGO_FILES));</code> |
| <code>interface.3.ts-perps-core-perps-config-marketlogourl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:120</code> | <code>export function marketLogoUrl(symbol: string): string</code> |
| <code>interface.3.ts-perps-core-perps-config-order-type-colors</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:111</code> | <code>export const ORDER_TYPE_COLORS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-order-type-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:102</code> | <code>export const ORDER_TYPE_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-perps-markets</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/core/perps/config.ts:57</code> | <code>export const PERPS_MARKETS: PerpsMarket[] = [</code> |
| <code>interface.3.ts-perps-core-perps-config-perpsmarket</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/core/perps/config.ts:48</code> | <code>export interface PerpsMarket</code> |
| <code>interface.3.ts-perps-core-perps-config-pofq-tier-labels</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/core/perps/config.ts:189</code> | <code>export const POFQ_TIER_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-position-action-copy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:267</code> | <code>export const POSITION_ACTION_COPY =</code> |
| <code>interface.3.ts-perps-core-perps-config-spot-mode-banner-copy</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/core/perps/config.ts:260</code> | <code>export const SPOT_MODE_BANNER_COPY =</code> |
| <code>interface.3.ts-perps-core-perps-config-spot-mode-labels</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/core/perps/config.ts:163</code> | <code>export const SPOT_MODE_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-spot-order-type-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:157</code> | <code>export const SPOT_ORDER_TYPE_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-spot-side-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:151</code> | <code>export const SPOT_SIDE_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-toorderlysymbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:23</code> | <code>export function toOrderlySymbol(paxeerSymbol: string): string</code> |
| <code>interface.3.ts-perps-core-perps-config-tp-sl-copy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:298</code> | <code>export const TP_SL_COPY =</code> |
| <code>interface.3.ts-perps-core-perps-config-track-b-strings</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:209</code> | <code>export const TRACK_B_STRINGS =</code> |
| <code>interface.3.ts-perps-core-perps-config-trade-type-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:178</code> | <code>export const TRADE_TYPE_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-config-vault-type-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/config.ts:202</code> | <code>export const VAULT_TYPE_LABELS: Record&lt;number, string&gt; =</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-formatfundingdecimalpct</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:93</code> | <code>export function formatFundingDecimalPct(v: string &#124; null, dp = 4): string</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-formatfundingratepct</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:48</code> | <code>export function formatFundingRatePct(rate: bigint, dp = 4): string</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-formatsignedfundingdecimalpct</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:115</code> | <code>export function formatSignedFundingDecimalPct(v: string &#124; null, dp = 4): string</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-funding-label-1h</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:19</code> | <code>export const FUNDING_LABEL_1H = "last settled funding (1h)";</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-funding-label-24h</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:20</code> | <code>export const FUNDING_LABEL_24H = "last settled funding (1h) ?24 projection";</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-funding-label-predicted</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:21</code> | <code>export const FUNDING_LABEL_PREDICTED = "Pred. funding rate";</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-funding-rate-decimals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:14</code> | <code>export const FUNDING_RATE_DECIMALS = 18;</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-fundingdecimalsign</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:107</code> | <code>export function fundingDecimalSign(v: string &#124; null): -1 &#124; 0 &#124; 1 &#124; null</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-fundingfigures</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:59</code> | <code>export function fundingFigures(hourly: bigint &#124; null):</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-fundingratefromjsonnumber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:78</code> | <code>export function fundingRateFromJsonNumber(v: unknown): string &#124; null</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-fundingrowfigure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:127</code> | <code>export function fundingRowFigure( socketRate: string &#124; null &#124; undefined, venueHourly: bigint &#124; null, ):</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-fundingwaddecimal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:145</code> | <code>export function fundingWadDecimal(rate: bigint &#124; null): string &#124; null</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-fundingwadfigure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:53</code> | <code>export function fundingWadFigure(rate: bigint &#124; null):</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-parsefundingratehourly</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:35</code> | <code>export function parseFundingRateHourly(v: string &#124; null &#124; undefined): bigint</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-parsefundingratewad</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:140</code> | <code>export function parseFundingRateWad(v: string &#124; null &#124; undefined): bigint &#124; null</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-projectfunding24h</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:43</code> | <code>export function projectFunding24h(hourly: bigint): bigint</code> |
| <code>interface.3.ts-perps-core-perps-fundingrate-venueticker</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/fundingRate.ts:24</code> | <code>export interface VenueTicker</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-buildconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:247</code> | <code>export function buildConfirmation(quote: OpenPositionQuote &#124; null, plan: OrderPlan): OrderConfirmation &#124; null</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-buildorderplan</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/core/perps/orderPlan.ts:201</code> | <code>export function buildOrderPlan(state: OrderFormState, quote: OpenPositionQuote &#124; null): OrderPlan</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-clearedinputs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:271</code> | <code>export type ClearedInputs = Pick&lt;</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-max-scaled-orders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:15</code> | <code>export const MAX_SCALED_ORDERS = 20;</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-min-scaled-orders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:14</code> | <code>export const MIN_SCALED_ORDERS = 2;</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-needsconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:218</code> | <code>export function needsConfirmation(orderConfirm: boolean): boolean</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-needsrequote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:213</code> | <code>export function needsRequote(quoteAgeMs: number): boolean</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-orderconfirmation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:222</code> | <code>export interface OrderConfirmation</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-orderformstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:33</code> | <code>export interface OrderFormState extends OrderOptions</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-orderkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:23</code> | <code>export type OrderKind = "market" &#124; "limit" &#124; "stop-limit" &#124; "scaled";</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-orderoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:26</code> | <code>export interface OrderOptions</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-orderplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:60</code> | <code>export interface OrderPlan</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-parseleverage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:71</code> | <code>export function parseLeverage(value: string): bigint</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-placeordercall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:58</code> | <code>export type PlaceOrderCall = ReturnType&lt;typeof buildOpenPosition&gt;;</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-quote-max-age-ms</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:13</code> | <code>export const QUOTE_MAX_AGE_MS = 6_000;</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-scaleddistribution</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:24</code> | <code>export type ScaledDistribution = "flat" &#124; "asc" &#124; "desc" &#124; "custom";</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-selectorderoption</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:77</code> | <code>export function selectOrderOption(options: OrderOptions, option: keyof OrderOptions, on: boolean): OrderOptions</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-successreset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:283</code> | <code>export interface SuccessReset</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-successreset-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:289</code> | <code>export function successReset(status: "success" &#124; "reverted"): SuccessReset &#124; null</code> |
| <code>interface.3.ts-perps-core-perps-orderplan-timeinforceof</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/orderPlan.ts:86</code> | <code>export function timeInForceOf(kind: OrderKind, options: OrderOptions): TimeInForce</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-collateralhealthcolor</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:259</code> | <code>export function collateralHealthColor(healthPct: bigint): string</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-computepositionpnl</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:148</code> | <code>export function computePositionPnl(pos: ParsedPosition, markPrice: bigint): PositionPnl</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-decimaltowad</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:120</code> | <code>export function decimalToWad(v: string): bigint</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-formatpnlpercent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:245</code> | <code>export function formatPnlPercent(value: bigint): string</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-formatpnlvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:240</code> | <code>export function formatPnlValue(value: bigint): string</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-formatwadfixed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:229</code> | <code>export function formatWadFixed(v: bigint, dp: number, group = false): string</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-markpricewad</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:132</code> | <code>export function markPriceWad(v: number &#124; string &#124; null &#124; undefined): bigint</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-parsedposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:36</code> | <code>export interface ParsedPosition</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-parsepositionrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:100</code> | <code>export function parsePositionRow(row: PositionRowInput, symbol: string): ParsedPosition</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-pctwad</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:140</code> | <code>export function pctWad(num: bigint, den: bigint): bigint</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-pnlcolorclass</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:250</code> | <code>export function pnlColorClass(value: bigint): string</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-positionpnl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:57</code> | <code>export interface PositionPnl</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-positionrowinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:72</code> | <code>export interface PositionRowInput</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-wad</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:30</code> | <code>export const WAD = 10n ** 18n;</code> |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-wadtodecimal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/pnlHelpers.ts:221</code> | <code>export function wadToDecimal(v: bigint): string</code> |
| <code>interface.3.ts-perps-core-perps-portfolio-portfoliofromaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/portfolio.ts:34</code> | <code>export function portfolioFromAccount( accountState: Pick&lt;AccountState, "collateral" &#124; "equityUsd" &#124; "withdrawable"&gt;, positions: readonly PortfolioPosition[], ): PortfolioTotals</code> |
| <code>interface.3.ts-perps-core-perps-portfolio-portfolioposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/portfolio.ts:13</code> | <code>export interface PortfolioPosition</code> |
| <code>interface.3.ts-perps-core-perps-portfolio-portfoliototals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/portfolio.ts:18</code> | <code>export interface PortfolioTotals</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-partial-close-bps</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:33</code> | <code>export const PARTIAL_CLOSE_BPS = [2_500, 5_000, 7_500] as const;</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-partialcloseaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:66</code> | <code>export interface PartialCloseAction</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-partialcloseaction-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:75</code> | <code>export function partialCloseAction(input: PartialCloseAction)</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-partialclosesize</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:55</code> | <code>export function partialCloseSize(sizeUsd: bigint, bps: number): bigint</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-positionaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:8</code> | <code>export interface PositionAction</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-positionactionkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:6</code> | <code>export type PositionActionKind = "close" &#124; "partialClose" &#124; "adjustCollateral" &#124; "topUp" &#124; "setTpSl" &#124; "cancel";</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-positionactions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:18</code> | <code>export const positionActions: readonly PositionAction[] = [</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-settpslaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:91</code> | <code>export function setTpSlAction(positionId: bigint, isLong: boolean, takeProfit: string, stopLoss: string)</code> |
| <code>interface.3.ts-perps-core-perps-positionactions-unitsfromindexed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionActions.ts:41</code> | <code>export function unitsFromIndexed(value: string): bigint</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-allowancesource</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:22</code> | <code>export interface AllowanceSource</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-closecall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:28</code> | <code>export type CloseCall = ReturnType&lt;typeof buildClosePosition&gt;;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-collateralcall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:29</code> | <code>export type CollateralCall = ReturnType&lt;typeof buildAddCollateral&gt; &#124; ReturnType&lt;typeof buildRemoveCollateral&gt;;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-collateralinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:65</code> | <code>export interface CollateralInput</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-placeordercall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:27</code> | <code>export type PlaceOrderCall = ReturnType&lt;typeof buildOpenPosition&gt;;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-planaddsize</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:51</code> | <code>export async function planAddSize(input: AddSizeInput, source: AllowanceSource): Promise&lt;WritePlan&lt;PlaceOrderCall&gt;&gt;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-planclose</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:56</code> | <code>export function planClose(input: ClosePositionInput): WritePlan&lt;CloseCall&gt;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-plancollateral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:73</code> | <code>export async function planCollateral(input: CollateralInput, source: AllowanceSource): Promise&lt;WritePlan&lt;CollateralCall&gt;&gt;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-planopen</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:46</code> | <code>export async function planOpen(input: OpenPositionInput, source: AllowanceSource): Promise&lt;WritePlan&lt;PlaceOrderCall&gt;&gt;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-planorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:81</code> | <code>export async function planOrder( calls: readonly PlaceOrderCall[], source: AllowanceSource, ): Promise&lt;WritePlan&lt;PlaceOrderCall&gt;&gt;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-planpartialclose</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:61</code> | <code>export function planPartialClose(input: PartialCloseInput): WritePlan&lt;CloseCall&gt;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-positionfromtuple</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:109</code> | <code>export function positionFromTuple(tuple: PositionTuple): PositionRecord</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-positionrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:93</code> | <code>export interface PositionRecord</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-positiontuple</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:91</code> | <code>export type PositionTuple = ContractFunctionReturnType&lt;typeof PerpsPrecompileABI, "view", "getPosition"&gt;;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-positionwrite</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:30</code> | <code>export type PositionWrite = PlaceOrderCall &#124; CloseCall &#124; CollateralCall;</code> |
| <code>interface.3.ts-perps-core-perps-positionwrites-writeplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/positionWrites.ts:33</code> | <code>export type WritePlan&lt;C extends PositionWrite = PositionWrite&gt; = readonly (ApproveCall &#124; C)[];</code> |
| <code>interface.3.ts-perps-core-perps-quotefallback-marketquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/quoteFallback.ts:4</code> | <code>export interface MarketQuote</code> |
| <code>interface.3.ts-perps-core-perps-quotefallback-marketquotefromprecompile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/quoteFallback.ts:33</code> | <code>export function marketQuoteFromPrecompile(result: PrecompileMarketQuote): MarketQuote</code> |
| <code>interface.3.ts-perps-core-perps-quotefallback-partialclosequote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/quoteFallback.ts:18</code> | <code>export interface PartialCloseQuote</code> |
| <code>interface.3.ts-perps-core-perps-quotefallback-partialclosequotefromprecompile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/quoteFallback.ts:50</code> | <code>export function partialCloseQuoteFromPrecompile(result: PrecompilePartialCloseQuote): PartialCloseQuote</code> |
| <code>interface.3.ts-perps-core-perps-quotefallback-precompilemarketquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/quoteFallback.ts:25</code> | <code>export type PrecompileMarketQuote = ContractFunctionReturnType&lt;typeof PerpsPrecompileABI, "view", "quoteMarket"&gt;;</code> |
| <code>interface.3.ts-perps-core-perps-quotefallback-precompilepartialclosequote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/perps/quoteFallback.ts:26</code> | <code>export type PrecompilePartialCloseQuote = ContractFunctionReturnType&lt;</code> |
| <code>interface.3.ts-perps-core-rpc-buildpaxeertransport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/rpc.ts:90</code> | <code>export function buildPaxeerTransport(): Transport</code> |
| <code>interface.3.ts-perps-core-rpc-getpaxeerrpcurls</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/rpc.ts:62</code> | <code>export function getPaxeerRpcUrls(): string[]</code> |
| <code>interface.3.ts-perps-core-spot-addresses-gettokenbyaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:89</code> | <code>export function getTokenByAddress( addressLike: string, ): SpotTokenMeta &#124; undefined</code> |
| <code>interface.3.ts-perps-core-spot-addresses-gettokenbysymbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:100</code> | <code>export function getTokenBySymbol(symbol: string): SpotTokenMeta &#124; undefined</code> |
| <code>interface.3.ts-perps-core-spot-addresses-isstablecoinaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:109</code> | <code>export function isStablecoinAddress(addressLike: string): boolean</code> |
| <code>interface.3.ts-perps-core-spot-addresses-native-pax-address</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:43</code> | <code>export const NATIVE_PAX_ADDRESS: Address =</code> |
| <code>interface.3.ts-perps-core-spot-addresses-pecor-contracts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:23</code> | <code>export const PECOR_CONTRACTS = PRECOMPILES.spot;</code> |
| <code>interface.3.ts-perps-core-spot-addresses-spot-stablecoin-addresses</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:82</code> | <code>export const SPOT_STABLECOIN_ADDRESSES: ReadonlySet&lt;string&gt; = new Set(</code> |
| <code>interface.3.ts-perps-core-spot-addresses-spot-stablecoin-list</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:79</code> | <code>export const SPOT_STABLECOIN_LIST: readonly SpotTokenMeta[] = [SPOT_TOKENS.USDL];</code> |
| <code>interface.3.ts-perps-core-spot-addresses-spot-tokens</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:46</code> | <code>export const SPOT_TOKENS =</code> |
| <code>interface.3.ts-perps-core-spot-addresses-spottokenmeta</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:27</code> | <code>export interface SpotTokenMeta</code> |
| <code>interface.3.ts-perps-core-spot-addresses-spottokensymbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/addresses.ts:74</code> | <code>export type SpotTokenSymbol = keyof typeof SPOT_TOKENS;</code> |
| <code>interface.3.ts-perps-core-spot-api-dstr</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:23</code> | <code>export function dStr(v: string &#124; null &#124; undefined, fallback = 0): number</code> |
| <code>interface.3.ts-perps-core-spot-api-dstrtotoken</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:33</code> | <code>export function dStrToToken( raw: string &#124; null &#124; undefined, decimals: number &#124; null &#124; undefined, fallback = 0, ): number</code> |
| <code>interface.3.ts-perps-core-spot-api-dstrtousd</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:49</code> | <code>export function dStrToUsd(raw: string &#124; null &#124; undefined, fallback = 0): number</code> |
| <code>interface.3.ts-perps-core-spot-api-positionusdvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:370</code> | <code>export function positionUsdValue(p: SpotPosition, currentPrice: number): number</code> |
| <code>interface.3.ts-perps-core-spot-api-spotapi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:277</code> | <code>export const spotApi =</code> |
| <code>interface.3.ts-perps-core-spot-api-spotapierror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:229</code> | <code>export class SpotApiError extends Error</code> |
| <code>interface.3.ts-perps-core-spot-api-spotclosedleg</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:172</code> | <code>export interface SpotClosedLeg</code> |
| <code>interface.3.ts-perps-core-spot-api-spotclosedlegsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:191</code> | <code>export interface SpotClosedLegsResponse</code> |
| <code>interface.3.ts-perps-core-spot-api-spotholdingsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:58</code> | <code>export interface SpotHoldingsResponse</code> |
| <code>interface.3.ts-perps-core-spot-api-spotidentity</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:210</code> | <code>export interface SpotIdentity</code> |
| <code>interface.3.ts-perps-core-spot-api-spotorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:110</code> | <code>export interface SpotOrder</code> |
| <code>interface.3.ts-perps-core-spot-api-spotordersresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:133</code> | <code>export interface SpotOrdersResponse</code> |
| <code>interface.3.ts-perps-core-spot-api-spotorderstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:108</code> | <code>export type SpotOrderStatus = "open" &#124; "executed" &#124; "cancelled" &#124; "activated";</code> |
| <code>interface.3.ts-perps-core-spot-api-spotordertype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:107</code> | <code>export type SpotOrderType = "limit" &#124; "stop_limit" &#124; "stop_loss";</code> |
| <code>interface.3.ts-perps-core-spot-api-spotpnlsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:199</code> | <code>export interface SpotPnlSummary</code> |
| <code>interface.3.ts-perps-core-spot-api-spotposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:77</code> | <code>export interface SpotPosition</code> |
| <code>interface.3.ts-perps-core-spot-api-spotpositionsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:102</code> | <code>export interface SpotPositionsResponse</code> |
| <code>interface.3.ts-perps-core-spot-api-spottrade</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:139</code> | <code>export interface SpotTrade</code> |
| <code>interface.3.ts-perps-core-spot-api-spottradesresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:164</code> | <code>export interface SpotTradesResponse</code> |
| <code>interface.3.ts-perps-core-spot-api-spottradesstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:219</code> | <code>export interface SpotTradesStats</code> |
| <code>interface.3.ts-perps-core-spot-api-symbolorshort</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/api.ts:379</code> | <code>export function symbolOrShort(token: string, symbol: string &#124; null &#124; undefined): string</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildcancellimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:163</code> | <code>export function buildCancelLimitOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildcancelstoplimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:255</code> | <code>export function buildCancelStopLimitOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildcancelstoplossorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:286</code> | <code>export function buildCancelStopLossOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildgetlimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:191</code> | <code>export function buildGetLimitOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildgetpaxpointer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:117</code> | <code>export function buildGetPaxPointer()</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildgetstoplimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:316</code> | <code>export function buildGetStopLimitOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildgetstoplossorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:326</code> | <code>export function buildGetStopLossOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildgetuserlimitorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:183</code> | <code>export function buildGetUserLimitOrders(user: Address)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildgetuserstoplimitorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:296</code> | <code>export function buildGetUserStopLimitOrders(user: Address)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildgetuserstoplossorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:306</code> | <code>export function buildGetUserStopLossOrders(user: Address)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildmarketbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:57</code> | <code>export function buildMarketBuy(input: MarketBuyCallInput)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildmarketsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:81</code> | <code>export function buildMarketSell(input: MarketSellCallInput)</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildmodifylimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:171</code> | <code>export function buildModifyLimitOrder(input: { orderId: bigint; newTargetPrice: bigint; newExpiresAt: bigint })</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildplacelimitbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:135</code> | <code>export function buildPlaceLimitBuy(input: LimitCallInput &amp; { stablecoin: Address; token: Address })</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildplacelimitsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:149</code> | <code>export function buildPlaceLimitSell(input: LimitCallInput &amp; { token: Address; stablecoin: Address })</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildplacestoplimitbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:218</code> | <code>export function buildPlaceStopLimitBuy(input: StopLimitCallInput &amp; { stablecoin: Address; token: Address })</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildplacestoplimitsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:237</code> | <code>export function buildPlaceStopLimitSell(input: StopLimitCallInput &amp; { token: Address; stablecoin: Address })</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildplacestoploss</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:265</code> | <code>export function buildPlaceStopLoss(input: { token: Address; stablecoin: Address; tokenAmount: bigint; /** Trigger price, 18 decimals: the sell fires when the mark is at or below it. */ triggerPrice: bigint; expiresAt: bigint; })</code> |
| <code>interface.3.ts-perps-core-spot-calls-buildswapexactinnative</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:104</code> | <code>export function buildSwapExactInNative(input: SwapExactInNativeCallInput)</code> |
| <code>interface.3.ts-perps-core-spot-calls-limitcallinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:126</code> | <code>export interface LimitCallInput</code> |
| <code>interface.3.ts-perps-core-spot-calls-marketbuycallinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:46</code> | <code>export interface MarketBuyCallInput</code> |
| <code>interface.3.ts-perps-core-spot-calls-marketsellcallinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:71</code> | <code>export interface MarketSellCallInput</code> |
| <code>interface.3.ts-perps-core-spot-calls-spotcall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:11</code> | <code>export interface SpotCall&lt;</code> |
| <code>interface.3.ts-perps-core-spot-calls-stoplimitcallinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:201</code> | <code>export interface StopLimitCallInput</code> |
| <code>interface.3.ts-perps-core-spot-calls-swapexactinnativecallinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:95</code> | <code>export interface SwapExactInNativeCallInput</code> |
| <code>interface.3.ts-perps-core-spot-calls-wei-per-uhpx</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/calls.ts:24</code> | <code>export const WEI_PER_UHPX = 10n ** 12n;</code> |
| <code>interface.3.ts-perps-core-spot-config-default-spot-symbol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:95</code> | <code>export const DEFAULT_SPOT_SYMBOL = "SID";</code> |
| <code>interface.3.ts-perps-core-spot-config-getspotmarket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:85</code> | <code>export function getSpotMarket(symbol: string): SpotMarket &#124; undefined</code> |
| <code>interface.3.ts-perps-core-spot-config-spot-categories</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:98</code> | <code>export const SPOT_CATEGORIES: Record&lt;string, string[]&gt; =</code> |
| <code>interface.3.ts-perps-core-spot-config-spot-category-labels</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:102</code> | <code>export const SPOT_CATEGORY_LABELS: Record&lt;string, string&gt; =</code> |
| <code>interface.3.ts-perps-core-spot-config-spot-markets</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:53</code> | <code>export const SPOT_MARKETS: SpotMarket[] = [</code> |
| <code>interface.3.ts-perps-core-spot-config-spot-stablecoins</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:77</code> | <code>export const SPOT_STABLECOINS: readonly StablecoinMeta[] = SPOT_STABLECOIN_LIST;</code> |
| <code>interface.3.ts-perps-core-spot-config-spot-symbols</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:82</code> | <code>export const SPOT_SYMBOLS = new Set(SPOT_MARKETS.map((m) =&gt; m.symbol));</code> |
| <code>interface.3.ts-perps-core-spot-config-spotcoinlogourl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:90</code> | <code>export function spotCoinLogoUrl(symbol: string): string</code> |
| <code>interface.3.ts-perps-core-spot-config-spotmarket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:9</code> | <code>export interface SpotMarket</code> |
| <code>interface.3.ts-perps-core-spot-config-stablecoinmeta</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/config.ts:75</code> | <code>export type StablecoinMeta = SpotTokenMeta;</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-batchquoterequests</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:24</code> | <code>export type BatchQuoteRequests = ContractFunctionArgs&lt;QuoterAbi, "view", "batchQuote"&gt;[0];</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildbatchquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:49</code> | <code>export function buildBatchQuote(requests: BatchQuoteRequests)</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildgetallliquidityinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:61</code> | <code>export function buildGetAllLiquidityInfo()</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildgetliquidityinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:57</code> | <code>export function buildGetLiquidityInfo(token: Address)</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildgettokenprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:53</code> | <code>export function buildGetTokenPrice(token: Address)</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildgettwap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:65</code> | <code>export function buildGetTWAP(token: Address, period: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildquoteexactin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:41</code> | <code>export function buildQuoteExactIn(tokenIn: Address, tokenOut: Address, amountIn: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildquoteexactout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:45</code> | <code>export function buildQuoteExactOut(tokenIn: Address, tokenOut: Address, amountOut: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildquotemarketbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:33</code> | <code>export function buildQuoteMarketBuy(stablecoin: Address, token: Address, stablecoinAmount: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-buildquotemarketsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:37</code> | <code>export function buildQuoteMarketSell(token: Address, stablecoin: Address, tokenAmount: bigint)</code> |
| <code>interface.3.ts-perps-core-spot-quotercalls-quotercall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/quoterCalls.ts:10</code> | <code>export interface QuoterCall&lt;</code> |
| <code>interface.3.ts-perps-core-spot-slippage-applyslippagemaxin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:60</code> | <code>export function applySlippageMaxIn( expectedIn: bigint, slippageBps: number, ): bigint</code> |
| <code>interface.3.ts-perps-core-spot-slippage-applyslippageminout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:43</code> | <code>export function applySlippageMinOut( expectedOut: bigint, slippageBps: number, ): bigint</code> |
| <code>interface.3.ts-perps-core-spot-slippage-bps-denominator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:24</code> | <code>export const BPS_DENOMINATOR = 10_000n;</code> |
| <code>interface.3.ts-perps-core-spot-slippage-builddeadline</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:75</code> | <code>export function buildDeadline(secondsFromNow: number = DEFAULT_MARKET_DEADLINE_SECONDS): bigint</code> |
| <code>interface.3.ts-perps-core-spot-slippage-buildlimitexpiry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:86</code> | <code>export function buildLimitExpiry( secondsFromNow: number = DEFAULT_LIMIT_EXPIRY_SECONDS, ): bigint</code> |
| <code>interface.3.ts-perps-core-spot-slippage-default-limit-expiry-seconds</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:30</code> | <code>export const DEFAULT_LIMIT_EXPIRY_SECONDS = 24 * 60 * 60; // 24 h</code> |
| <code>interface.3.ts-perps-core-spot-slippage-default-market-deadline-seconds</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:27</code> | <code>export const DEFAULT_MARKET_DEADLINE_SECONDS = 60;</code> |
| <code>interface.3.ts-perps-core-spot-slippage-expectedoutfromprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:160</code> | <code>export function expectedOutFromPrice(params: { amountIn: bigint; price: bigint; /** Decimals of `amountIn`. */ amountInDecimals: number; /** Decimals of the output token. */ amountOutDecimals: number; }): bigint</code> |
| <code>interface.3.ts-perps-core-spot-slippage-formatprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:145</code> | <code>export function formatPrice(value: bigint): string</code> |
| <code>interface.3.ts-perps-core-spot-slippage-formattokenamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:120</code> | <code>export function formatTokenAmount(value: bigint, decimals: number): string</code> |
| <code>interface.3.ts-perps-core-spot-slippage-limitminout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:184</code> | <code>export function limitMinOut( amountIn: bigint, limitPrice: bigint, isBuy: boolean, decimalsIn: number, decimalsOut: number, ): bigint</code> |
| <code>interface.3.ts-perps-core-spot-slippage-parseprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:140</code> | <code>export function parsePrice(value: string &#124; number): bigint</code> |
| <code>interface.3.ts-perps-core-spot-slippage-parsetokenamount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:99</code> | <code>export function parseTokenAmount(value: string &#124; number, decimals: number): bigint</code> |
| <code>interface.3.ts-perps-core-spot-slippage-price-decimals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:20</code> | <code>export const PRICE_DECIMALS = 18;</code> |
| <code>interface.3.ts-perps-core-spot-slippage-price-scale</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:21</code> | <code>export const PRICE_SCALE = 10n ** BigInt(PRICE_DECIMALS);</code> |
| <code>interface.3.ts-perps-core-spot-slippage-quotedecimals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:237</code> | <code>export function quoteDecimals( stables: readonly { readonly decimals: number }[], ): number</code> |
| <code>interface.3.ts-perps-core-spot-slippage-spotorderrail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:222</code> | <code>export function spotOrderRail( orderType: SpotOrderType, tif: SpotTimeInForce, postOnly: boolean, ): SpotRail</code> |
| <code>interface.3.ts-perps-core-spot-slippage-spotordertype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:214</code> | <code>export type SpotOrderType = "market" &#124; "limit" &#124; "stop-limit";</code> |
| <code>interface.3.ts-perps-core-spot-slippage-spotrail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:216</code> | <code>export type SpotRail = "market" &#124; "limit" &#124; "stop";</code> |
| <code>interface.3.ts-perps-core-spot-slippage-spottimeinforce</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/slippage.ts:215</code> | <code>export type SpotTimeInForce = "GTC" &#124; "IOC" &#124; "FOK";</code> |
| <code>interface.3.ts-perps-core-spot-stableselector-buildstableamountmap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/stableSelector.ts:95</code> | <code>export function buildStableAmountMap( entries: ReadonlyArray&lt;{ address: Address &#124; string; amount: bigint }&gt;, ): StableAmountMap</code> |
| <code>interface.3.ts-perps-core-spot-stableselector-selectstableforbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/stableSelector.ts:65</code> | <code>export function selectStableForBuy(input: SelectStableForBuyInput): StableSelection</code> |
| <code>interface.3.ts-perps-core-spot-stableselector-selectstableforbuyinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/stableSelector.ts:21</code> | <code>export interface SelectStableForBuyInput</code> |
| <code>interface.3.ts-perps-core-spot-stableselector-selectstableforsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/stableSelector.ts:73</code> | <code>export function selectStableForSell(input: SelectStableForSellInput): StableSelection</code> |
| <code>interface.3.ts-perps-core-spot-stableselector-selectstableforsellinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/stableSelector.ts:31</code> | <code>export interface SelectStableForSellInput</code> |
| <code>interface.3.ts-perps-core-spot-stableselector-stableamountmap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/stableSelector.ts:19</code> | <code>export type StableAmountMap = Readonly&lt;Record&lt;string, bigint&gt;&gt;;</code> |
| <code>interface.3.ts-perps-core-spot-stableselector-stableselection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/stableSelector.ts:42</code> | <code>export interface StableSelection</code> |
| <code>interface.3.ts-perps-core-spot-stableselector-sumstableamounts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/spot/stableSelector.ts:83</code> | <code>export function sumStableAmounts(map: StableAmountMap): bigint</code> |
| <code>interface.3.ts-perps-core-viem-clients-getconnectedaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/viem/clients.ts:49</code> | <code>export function getConnectedAddress(): `0x$</code> |
| <code>interface.3.ts-perps-core-viem-clients-getpublicclient</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/viem/clients.ts:16</code> | <code>export function getPublicClient(): PublicClient</code> |
| <code>interface.3.ts-perps-core-viem-clients-getwalletaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/viem/clients.ts:61</code> | <code>export async function getWalletAccount(): Promise&lt;`0x$</code> |
| <code>interface.3.ts-perps-core-viem-clients-getwalletclient</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/viem/clients.ts:33</code> | <code>export async function getWalletClient(): Promise&lt;WalletClient &#124; null&gt;</code> |
| <code>interface.3.ts-perps-core-wagmi-wagmiconfig</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wagmi.ts:50</code> | <code>export const wagmiConfig = createConfig(</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-client-getauthredirecturl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/client.ts:74</code> | <code>export function getAuthRedirectUrl(): string</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-client-getpaxeerwallet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/client.ts:48</code> | <code>export function getPaxeerWallet(): PaxeerWallet &#124; null</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-client-resolvedpaxeerconfig</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/client.ts:22</code> | <code>export interface ResolvedPaxeerConfig extends PaxeerWalletConfig</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-client-resolvepaxeerconfig</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/client.ts:27</code> | <code>export function resolvePaxeerConfig(): ResolvedPaxeerConfig &#124; null</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-eip1193-createpaxeereip1193provider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/eip1193.ts:54</code> | <code>export function createPaxeerEIP1193Provider(opts: PaxeerProviderOptions): PaxeerEIP1193Provider</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-eip1193-eip1193event</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/eip1193.ts:31</code> | <code>export type EIP1193Event =</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-eip1193-paxeereip1193provider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/eip1193.ts:38</code> | <code>export interface PaxeerEIP1193Provider</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-eip1193-paxeerprovideroptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/eip1193.ts:47</code> | <code>export interface PaxeerProviderOptions</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerconnector-paxeer-connector-id</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/paxeerConnector.ts:17</code> | <code>export const PAXEER_CONNECTOR_ID = "paxeer-embedded";</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerconnector-paxeerconnectorparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/paxeerConnector.ts:19</code> | <code>export interface PaxeerConnectorParams</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerconnector-paxeerembeddedconnector</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/paxeerConnector.ts:25</code> | <code>export function paxeerEmbeddedConnector(params: PaxeerConnectorParams)</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerprovider-embeddedsigninresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx:32</code> | <code>export interface EmbeddedSignInResult</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerprovider-paxeerprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx:63</code> | <code>export function PaxeerProvider({ children }: PaxeerProviderProps)</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerprovider-signinprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx:30</code> | <code>export type SignInProvider = OAuthProvider;</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerprovider-useembeddedwalletavailable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx:226</code> | <code>export function useEmbeddedWalletAvailable(): boolean</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerprovider-usepaxeer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/PaxeerProvider.tsx:217</code> | <code>export function usePaxeer(): PaxeerContextValue</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-index-oauthprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/index.ts:33</code> | <code>export type OAuthProvider = "google" &#124; "apple" &#124; "twitter" &#124; "github" &#124; "discord";</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-index-paxeerwallet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/index.ts:35</code> | <code>export class PaxeerWallet</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usepaxeerwallet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/react.ts:97</code> | <code>export function usePaxeerWallet(config: PaxeerWalletConfig): PaxeerWallet</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usesession</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/react.ts:19</code> | <code>export function useSession(wallet: PaxeerWallet): UseSessionResult</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usesessionresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/react.ts:13</code> | <code>export interface UseSessionResult</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usewallet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/react.ts:54</code> | <code>export function useWallet(client: PaxeerWallet): UseWalletResult</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usewalletresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/react.ts:46</code> | <code>export interface UseWalletResult</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-types-chaininfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/types.ts:29</code> | <code>export interface ChainInfo</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-types-paxeerwalletconfig</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/types.ts:10</code> | <code>export interface PaxeerWalletConfig</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-types-paxeerwalleterror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/types.ts:65</code> | <code>export class PaxeerWalletError extends Error</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-types-publicwallet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/types.ts:21</code> | <code>export interface PublicWallet</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-types-sendtxresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/types.ts:54</code> | <code>export interface SendTxResponse</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-types-signmessageresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/types.ts:60</code> | <code>export interface SignMessageResponse</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-types-signtxresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/types.ts:48</code> | <code>export interface SignTxResponse</code> |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-types-txrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/sdk/types.ts:35</code> | <code>export interface TxRequest</code> |
| <code>interface.3.ts-perps-core-wallet-postmessageeip1193-installpostmessageeip1193provider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/postMessageEip1193.ts:139</code> | <code>export function installPostMessageEip1193Provider()</code> |
| <code>interface.3.ts-perps-core-wallet-postmessageeip1193-postmessageeip1193provider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/postMessageEip1193.ts:52</code> | <code>export class PostMessageEip1193Provider</code> |
| <code>interface.3.ts-perps-feetier-feetierview-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/feeTier.ts</code> (planned) | <code>export function feeTierView(raw: PrecompileFeeTier): FeeTierView</code> |
| <code>interface.3.ts-perps-marketcatalog-catalogfromgateway-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/marketCatalog.ts</code> (planned) | <code>export function catalogFromGateway(rows: readonly GatewayMarketRow[]): MarketCatalogEntry[]</code> |
| <code>interface.3.ts-perps-marketconfig-decodemarketconfig-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/marketConfig.ts</code> (planned) | <code>export function decodeMarketConfig(raw: PrecompileMarketConfig): MarketConfig</code> |
| <code>interface.3.ts-perps-marketconfig-maintenancemargin-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/marketConfig.ts</code> (planned) | <code>export function maintenanceMargin(tiers: readonly MarginTier[], notionalUsd: bigint): bigint</code> |
| <code>interface.3.ts-perps-marketconfig-maxleveragefornotional-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/marketConfig.ts</code> (planned) | <code>export function maxLeverageForNotional(tiers: readonly MarginTier[], notionalUsd: bigint): bigint</code> |
| <code>interface.3.ts-perps-marketlabel-marketlabel-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/marketLabel.ts</code> (planned) | <code>export function marketLabel(marketId: bigint, catalog: readonly MarketCatalogEntry[]): string</code> |
| <code>interface.3.ts-perps-markstatus-markstatus-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/markStatus.ts</code> (planned) | <code>export function markStatus(details: MarkPriceDetails, enabled: boolean): 'open' &#124; 'stale' &#124; 'disabled'</code> |
| <code>interface.3.ts-perps-orderplan-aligntotick-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/orderPlan.ts</code> (planned) | <code>export function alignToTick(price: bigint, tickSize: bigint, side: 'down' &#124; 'up'): bigint</code> |
| <code>interface.3.ts-perps-orderplan-checkorderlimits-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/orderPlan.ts</code> (planned) | <code>export function checkOrderLimits(state: OrderFormState, limits: OrderLimits): OrderPlan &#124; null</code> |
| <code>interface.3.ts-perps-quotefallback-tickerquotewithconfig-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/perps/quoteFallback.ts</code> (planned) | <code>export function tickerQuoteWithConfig(ticker: TickerQuote, config: MarketConfig, tiers: readonly MarginTier[], mark: MarkPriceDetails): MarketQuote</code> |
| <code>interface.3.ts-perps-spot-calls-buildistokenpaused-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/spot/calls.ts</code> (planned) | <code>export function buildIsTokenPaused(token: Address) { return { address: SPOT_VAULT, abi: pecorVaultAbi, functionName: "isTokenPaused", args: [token] } }</code> |
| <code>interface.3.ts-perps-spot-tokenstate-spottokenstate-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/spot/tokenState.ts</code> (planned) | <code>export function spotTokenState(paused: boolean, info: VaultTokenInfo): SpotTokenState</code> |
| <code>interface.3.ts-perps-venuestream-registervenuesymbols-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/core/net/venueStream.ts</code> (planned) | <code>export function registerVenueSymbols(symbols: readonly string[]): void</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-core-errors-classify-precompile-error-codes</code> | overridden_by | override.J27.19 |
| <code>interface.3.ts-perps-core-errors-classify-precompileerrorentry</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-errors-classify-precompileerrorname</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-errors-global-installglobalerrorhandlers</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-errors-report-leavebreadcrumb</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-errors-types-errorseverity</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-log-logger-logger</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-log-logger-loglevel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-net-venuestream-isvenuesymbol</code> | overridden_by | override.J27.22 |
| <code>interface.3.ts-perps-core-network-abi-ierc20-ierc20abi</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-network-abi-index-ierc20abi</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-network-abi-index-ordertype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-network-abi-index-perpsprecompileabi</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-network-abi-index-timeinforce</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-network-abi-perpsprecompile-perpsprecompileabi</code> | notes | 0x1015 conflict: this ABI equals precompiles/perps/abi/abi.json while stale feature 3 plans layerxexchange v2 methods at the same address (capability groups 3.1-3.5); the merge design does not yet decide which ABI 0x1015 serves |
| <code>interface.3.ts-perps-core-network-abi-perpsprecompile-perpsprecompileabi</code> | overridden_by | override.J27.1 |
| <code>interface.3.ts-perps-core-network-spot-abi-pecor-pecorabi</code> | notes | spot conflict: this ABI equals the precompiles/spot/abi JSON served at 0x1021-0x1025 while stale feature 3 plans spot inside layerxexchange 0x1015; the merge design does not yet decide |
| <code>interface.3.ts-perps-core-network-spot-abi-pecor-pecorabi</code> | overridden_by | override.J27.2 |
| <code>interface.3.ts-perps-core-network-spot-abi-pecororders-pecorordersabi</code> | notes | spot conflict: this ABI equals the precompiles/spot/abi JSON served at 0x1021-0x1025 while stale feature 3 plans spot inside layerxexchange 0x1015; the merge design does not yet decide |
| <code>interface.3.ts-perps-core-network-spot-abi-pecororders-pecorordersabi</code> | overridden_by | override.J27.3 |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorquoter-pecorquoterabi</code> | notes | spot conflict: this ABI equals the precompiles/spot/abi JSON served at 0x1021-0x1025 while stale feature 3 plans spot inside layerxexchange 0x1015; the merge design does not yet decide |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorquoter-pecorquoterabi</code> | overridden_by | override.J27.4 |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorstoporders-pecorstopordersabi</code> | notes | spot conflict: this ABI equals the precompiles/spot/abi JSON served at 0x1021-0x1025 while stale feature 3 plans spot inside layerxexchange 0x1015; the merge design does not yet decide |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorstoporders-pecorstopordersabi</code> | overridden_by | override.J27.5 |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorvault-pecorvaultabi</code> | notes | spot conflict: this ABI equals the precompiles/spot/abi JSON served at 0x1021-0x1025 while stale feature 3 plans spot inside layerxexchange 0x1015; the merge design does not yet decide |
| <code>interface.3.ts-perps-core-network-spot-abi-pecorvault-pecorvaultabi</code> | overridden_by | override.J27.6 |
| <code>interface.3.ts-perps-core-paxscan-api-resources-apiresource</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-paxscan-socket-types-socketmessage</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-paxscan-socket-usesocketchannel-usepaxscansocketchannel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-paxscan-socket-usesocketmessage-usepaxscansocketmessage</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-accountview-pickedcollateral</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-config-perps-markets</code> | overridden_by | override.J27.15 |
| <code>interface.3.ts-perps-core-perps-config-perpsmarket</code> | overridden_by | override.J27.14 |
| <code>interface.3.ts-perps-core-perps-config-pofq-tier-labels</code> | overridden_by | override.J27.36 |
| <code>interface.3.ts-perps-core-perps-config-spot-mode-banner-copy</code> | overridden_by | override.J27.37 |
| <code>interface.3.ts-perps-core-perps-config-spot-mode-labels</code> | overridden_by | override.J27.35 |
| <code>interface.3.ts-perps-core-perps-fundingrate-funding-rate-decimals</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-orderplan-buildorderplan</code> | overridden_by | override.J27.16 |
| <code>interface.3.ts-perps-core-perps-orderplan-clearedinputs</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-orderplan-min-scaled-orders</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-orderplan-orderkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-orderplan-placeordercall</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-orderplan-scaleddistribution</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-orderplan-successreset</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-collateralhealthcolor</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-computepositionpnl</code> | overridden_by | override.J27.23 |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-pnlcolorclass</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-pnlhelpers-positionrowinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-portfolio-portfolioposition</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-portfolio-portfoliototals</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-positionactions-partialcloseaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-positionactions-positionaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-perps-positionactions-positionactionkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-addresses-gettokenbyaddress</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-addresses-gettokenbysymbol</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-addresses-isstablecoinaddress</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-addresses-pecor-contracts</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-addresses-spot-stablecoin-addresses</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-addresses-spottokensymbol</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-calls-limitcallinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-calls-marketbuycallinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-calls-marketsellcallinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-calls-spotcall</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-calls-stoplimitcallinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-calls-swapexactinnativecallinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-quotercalls-batchquoterequests</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-quotercalls-quotercall</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-stableselector-selectstableforbuyinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-stableselector-selectstableforsellinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-stableselector-stableamountmap</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-stableselector-stableselection</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-spot-stableselector-sumstableamounts</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-client-resolvedpaxeerconfig</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-eip1193-eip1193event</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-eip1193-paxeerprovideroptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerconnector-paxeerconnectorparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-paxeerprovider-embeddedsigninresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usepaxeerwallet</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usesession</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usesessionresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usewallet</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-embedded-sdk-react-usewalletresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-core-wallet-postmessageeip1193-postmessageeip1193provider</code> | notes | no in-repo importer at HEAD |

Contract context <code>contract_context.3.perps-app-markets-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | The perps app market catalog, order limits, quote, fee tier and spot token state read from gateway GraphQL and 0x1015 views |
| storage | No app storage; the catalog is held in memory per session |
| errors | VALIDATION_PRECISION, VALIDATION_ABOVE_MAX, VALIDATION_BELOW_MIN and TRADE_MARKET_HALTED of apps/perps/src/core/errors/types.ts |
| events | None emitted; reads only |
| gas | View calls only; writes keep the sponsored send path |
| activation | Clients of the v6.12.0 surfaces |
| invariants | An order the app submits passes the chain tick, lot and leverage checks it read; a paused spot token cannot be submitted |
| pause | A disabled market or paused token blocks submit in the app |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.509.1</code> | req | 3509.1 |
| <code>acceptance.3.509.1</code> | scenario | Given a getMarketConfig answer for BTC encoded from PerpsPrecompileABI, when decodeMarketConfig decodes it, then szDecimals, pxDecimals, tickSize, lotSize, maxLeverage and enabled equal the encoded values |
| <code>acceptance.3.509.1</code> | test | apps/perps/tests/market-config.test.ts::decodeMarketConfigFromAbi (planned) |
| <code>acceptance.3.509.1</code> | qualification | UNRUN |
| <code>req.3509</code> | ac_1 | decodeMarketConfig of a getMarketConfig answer encoded from the ABI entry at apps/perps/src/core/network/abi/PerpsPrecompile.ts:2097 returns every field with szDecimals, pxDecimals, tickSize and lotSize unchanged. |
| <code>acceptance.3.509.2</code> | req | 3509.2 |
| <code>acceptance.3.509.2</code> | scenario | Given tiers with bounds 0, 1e6 and 5e6 USD and leverages 50, 20 and 10, when maxLeverageForNotional runs for 5e5, 2e6 and 9e6 USD, then it returns 50, 20 and 10 |
| <code>acceptance.3.509.2</code> | test | apps/perps/tests/market-config.test.ts::maxLeverageByTier (planned) |
| <code>acceptance.3.509.2</code> | qualification | UNRUN |
| <code>req.3509</code> | ac_2 | maxLeverageForNotional over three tiers decoded from a getMarginTiers answer (apps/perps/src/core/network/abi/PerpsPrecompile.ts:1959) returns tier 0 leverage below the first bound and the tier 2 leverage above the last bound. |
| <code>acceptance.3.509.3</code> | req | 3509.3 |
| <code>acceptance.3.509.3</code> | scenario | Given a tier with maintenanceBps 250 and deductionUsd 1000e18, when maintenanceMargin runs for 2e6 USD notional, then it returns 49000 USD in 18 decimals |
| <code>acceptance.3.509.3</code> | test | apps/perps/tests/market-config.test.ts::maintenanceMarginTiered (planned) |
| <code>acceptance.3.509.3</code> | qualification | UNRUN |
| <code>req.3509</code> | ac_3 | maintenanceMargin equals notional * maintenanceBps / 10000 - deductionUsd of the matching tier, the formula of modules/perps/types/tiers.go:35. |
| <code>acceptance.3.510.1</code> | req | 3510.1 |
| <code>acceptance.3.510.1</code> | scenario | Given limits with tickSize 0.5 USD and a limit order at 100.3 USD, when checkOrderLimits runs, then it refuses with VALIDATION_PRECISION, and at 100.5 USD it returns null |
| <code>acceptance.3.510.1</code> | test | apps/perps/tests/order-limits.test.ts::offTickPriceRefused (planned) |
| <code>acceptance.3.510.1</code> | qualification | UNRUN |
| <code>req.3510</code> | ac_1 | checkOrderLimits refuses a limit order whose price is not a multiple of tickSize with code VALIDATION_PRECISION (apps/perps/src/core/errors/types.ts:55) and accepts the aligned price. |
| <code>acceptance.3.510.2</code> | req | 3510.2 |
| <code>acceptance.3.510.2</code> | scenario | Given limits with maxLeverage 20 and a form at leverage 25, when checkOrderLimits runs, then it refuses with VALIDATION_ABOVE_MAX |
| <code>acceptance.3.510.2</code> | test | apps/perps/tests/order-limits.test.ts::leverageAboveChainMaxRefused (planned) |
| <code>acceptance.3.510.2</code> | qualification | UNRUN |
| <code>req.3510</code> | ac_2 | checkOrderLimits refuses a leverage above the chain maxLeverage with VALIDATION_ABOVE_MAX (apps/perps/src/core/errors/types.ts:54) where parseLeverage (apps/perps/src/core/perps/orderPlan.ts:71) alone accepts it. |
| <code>acceptance.3.510.3</code> | req | 3510.3 |
| <code>acceptance.3.510.3</code> | scenario | Given tickSize 0.5 USD, when alignToTick runs on 100.3 USD down and up, then it returns 100.0 USD and 100.5 USD |
| <code>acceptance.3.510.3</code> | test | apps/perps/tests/order-limits.test.ts::alignToTickRoundsBySide (planned) |
| <code>acceptance.3.510.3</code> | qualification | UNRUN |
| <code>req.3510</code> | ac_3 | alignToTick rounds 100.3 down to 100.0 for a buy and up to 100.5 for a sell with tickSize 0.5, so the rungs of apps/perps/src/core/perps/orderPlan.ts:171 land on ticks. |
| <code>acceptance.3.511.1</code> | req | 3511.1 |
| <code>acceptance.3.511.1</code> | scenario | Given a gateway markets answer with ids 0 to 18 including DOGE, when catalogFromGateway runs, then the catalog has 19 entries and entry 18 is DOGE |
| <code>acceptance.3.511.1</code> | test | apps/perps/tests/market-catalog.test.ts::catalogIncludesChainListedMarket (planned) |
| <code>acceptance.3.511.1</code> | qualification | UNRUN |
| <code>req.3511</code> | ac_1 | catalogFromGateway of a gateway answer holding market 18 DOGE returns a DOGE entry absent from the static PERPS_MARKETS of apps/perps/src/core/perps/config.ts:57-76, ordered after market 17. |
| <code>acceptance.3.511.2</code> | req | 3511.2 |
| <code>acceptance.3.511.2</code> | scenario | Given two rows with createdAt 3 and 30 days ago, when catalogFromGateway runs with that now, then the first is new and the second keeps its category |
| <code>acceptance.3.511.2</code> | test | apps/perps/tests/market-catalog.test.ts::catalogMarksRecentListingNew (planned) |
| <code>acceptance.3.511.2</code> | qualification | UNRUN |
| <code>req.3511</code> | ac_2 | An entry listed 3 days before now gets category new and one listed 30 days before keeps getMarketCategory (apps/perps/src/core/perps/config.ts:94). |
| <code>acceptance.3.512.1</code> | req | 3512.1 |
| <code>acceptance.3.512.1</code> | scenario | Given decoded mark details with fresh false and true and enabled false and true, when markStatus runs, then it returns stale, disabled and open |
| <code>acceptance.3.512.1</code> | test | apps/perps/tests/mark-status.test.ts::markStatusStaleAndDisabled (planned) |
| <code>acceptance.3.512.1</code> | qualification | UNRUN |
| <code>req.3512</code> | ac_1 | markStatus returns stale for a getMarkPriceDetails answer with fresh false, disabled for an enabled false market, and open otherwise, where apps/perps/src/hooks/perps/useQuoter.ts:128-129 always reports enabled and fresh. |
| <code>acceptance.3.513.1</code> | req | 3513.1 |
| <code>acceptance.3.513.1</code> | scenario | Given isTokenPaused true and a getTokenInfo answer encoded from pecorVaultAbi, when spotTokenState runs, then tradeable is false and reason is TokenPaused |
| <code>acceptance.3.513.1</code> | test | apps/perps/tests/spot-token-state.test.ts::pausedTokenNotTradeable (planned) |
| <code>acceptance.3.513.1</code> | qualification | UNRUN |
| <code>req.3513</code> | ac_1 | spotTokenState of an isTokenPaused true answer encoded from apps/perps/src/core/network/spot_abi/PECORVault.ts:353 returns tradeable false and reason TokenPaused. |
| <code>acceptance.3.513.2</code> | req | 3513.2 |
| <code>acceptance.3.513.2</code> | scenario | Given a token address, when buildIsTokenPaused and buildGetTokenInfo are built, then each targets the vault with that token as its only argument |
| <code>acceptance.3.513.2</code> | test | apps/perps/tests/spot-token-state.test.ts::tokenStateCallsTargetVault (planned) |
| <code>acceptance.3.513.2</code> | qualification | UNRUN |
| <code>req.3513</code> | ac_2 | buildIsTokenPaused and buildGetTokenInfo name the vault address and the functions of apps/perps/src/core/network/spot_abi/PECORVault.ts:353 and apps/perps/src/core/network/spot_abi/PECORVault.ts:247 with the token argument. |
| <code>acceptance.3.514.1</code> | req | 3514.1 |
| <code>acceptance.3.514.1</code> | scenario | Given a getFeeTier answer encoded from PerpsPrecompileABI, when feeTierView runs, then tier, takerBps, makerBps and volume14dUsd equal the encoded values in bps |
| <code>acceptance.3.514.1</code> | test | apps/perps/tests/fee-tier.test.ts::feeTierViewFromAbi (planned) |
| <code>acceptance.3.514.1</code> | qualification | UNRUN |
| <code>req.3514</code> | ac_1 | feeTierView of a getFeeTier answer encoded from apps/perps/src/core/network/abi/PerpsPrecompile.ts:1711 with tier 2, takerFeePpm 250 and makerFeePpm 0 returns tier 2, takerBps 2.5 and makerBps 0. |
| <code>acceptance.3.515.1</code> | req | 3515.1 |
| <code>acceptance.3.515.1</code> | scenario | Given revert data of the five perps limit errors encoded from PerpsPrecompileABI, when classifyError runs, then each returns its specific validation code |
| <code>acceptance.3.515.1</code> | test | apps/perps/tests/classify-limits.test.ts::limitRevertsMapped (planned) |
| <code>acceptance.3.515.1</code> | qualification | UNRUN |
| <code>req.3515</code> | ac_1 | classifyError of revert data holding the PerpsTickSize, PerpsLotSize, PerpsLeverageTooHigh, PerpsMinOrderValue and PerpsMaxOrderValue selectors returns VALIDATION_PRECISION, VALIDATION_PRECISION, VALIDATION_ABOVE_MAX, VALIDATION_BELOW_MIN and VALIDATION_ABOVE_MAX instead of the REVERT of apps/perps/src/core/errors/classify.ts:321-325. |
| <code>acceptance.3.515.2</code> | req | 3515.2 |
| <code>acceptance.3.515.2</code> | scenario | Given revert data of PerpsMarketDisabled and of TokenPaused, when classifyError runs, then both return TRADE_MARKET_HALTED |
| <code>acceptance.3.515.2</code> | test | apps/perps/tests/classify-limits.test.ts::haltedRevertsMapped (planned) |
| <code>acceptance.3.515.2</code> | qualification | UNRUN |
| <code>req.3515</code> | ac_2 | classifyError of PerpsMarketDisabled (apps/perps/src/core/errors/classify.ts:315) and spot TokenPaused (apps/perps/src/core/errors/classify.ts:407) returns TRADE_MARKET_HALTED. |
| <code>acceptance.3.516.1</code> | req | 3516.1 |
| <code>acceptance.3.516.1</code> | scenario | Given the default venue set, when isVenueSymbol is asked for DOGE before and after registerVenueSymbols([BTC, DOGE]), then it returns false and then true |
| <code>acceptance.3.516.1</code> | test | apps/perps/tests/venue-symbols.test.ts::registeredSymbolIsVenueSymbol (planned) |
| <code>acceptance.3.516.1</code> | qualification | UNRUN |
| <code>req.3516</code> | ac_1 | After registerVenueSymbols with DOGE, isVenueSymbol (apps/perps/src/core/net/venueStream.ts:80) returns true for doge, while before registration it returns false as the static set of apps/perps/src/core/net/venueStream.ts:75 lacks DOGE. |
| <code>acceptance.3.517.1</code> | req | 3517.1 |
| <code>acceptance.3.517.1</code> | scenario | Given a catalog with market 18 DOGE and an empty catalog, when marketLabel runs for 18n, then it returns DOGE and then 18 |
| <code>acceptance.3.517.1</code> | test | apps/perps/tests/market-label.test.ts::marketLabelFromCatalog (planned) |
| <code>acceptance.3.517.1</code> | qualification | UNRUN |
| <code>req.3517</code> | ac_1 | marketLabel(18n, catalog) returns DOGE for a catalog holding market 18 and 18 for an empty catalog, never the MKT-18 of apps/perps/src/widgets/trading/PerpsPositionsPanel.tsx:66. |
| <code>acceptance.3.528.1</code> | req | 3528.1 |
| <code>acceptance.3.528.1</code> | scenario | Given the app ABI and the chain abi.json, when both are normalized and compared, then the sets are equal |
| <code>acceptance.3.528.1</code> | test | apps/perps/tests/perps-abi-parity.test.ts::perpsAbiMatchesChain (planned) |
| <code>acceptance.3.528.1</code> | qualification | UNRUN |
| <code>req.3528</code> | ac_1 | Every function, event and error of PerpsPrecompileABI (apps/perps/src/core/network/abi/PerpsPrecompile.ts:1) equals the entry of precompiles/perps/abi/abi.json:1 with the same name and signature, and neither side has an extra entry. |
| <code>acceptance.3.529.1</code> | req | 3529.1 |
| <code>acceptance.3.529.1</code> | scenario | Given the five app spot ABIs and the chain JSON files, when each pair is normalized and compared, then every pair is equal |
| <code>acceptance.3.529.1</code> | test | apps/perps/tests/spot-abi-parity.test.ts::spotAbisMatchChain (planned) |
| <code>acceptance.3.529.1</code> | qualification | UNRUN |
| <code>req.3529</code> | ac_1 | Each of the five app spot ABIs imported at apps/perps/tests/spot-abi.test.ts:10-14 equals its chain file under precompiles/spot/abi/vault.json:1 and its siblings entry for entry. |
| <code>acceptance.3.530.1</code> | req | 3530.1 |
| <code>acceptance.3.530.1</code> | scenario | Given the config module after the retirement, when its exports are read, then the three spot-design names are absent and no market row has maxLeverage |
| <code>acceptance.3.530.1</code> | test | apps/perps/tests/perps-config.test.ts::configHasNoSpotDesignExports (planned) |
| <code>acceptance.3.530.1</code> | qualification | UNRUN |
| <code>req.3530</code> | ac_1 | The module of apps/perps/src/core/perps/config.ts exports no SPOT_MODE_LABELS, POFQ_TIER_LABELS or SPOT_MODE_BANNER_COPY and no PERPS_MARKETS row carries maxLeverage, where apps/perps/src/core/perps/config.ts:57-76 sets 1000. |
| <code>acceptance.3.531.1</code> | req | 3531.1 |
| <code>acceptance.3.531.1</code> | scenario | Given a 10x long of 2e6 USD at entry 100 and a tier with maintenanceBps 250 and deductionUsd 1000, when computePositionPnl runs, then liqPrice equals the price where remaining collateral equals 49000 USD |
| <code>acceptance.3.531.1</code> | test | apps/perps/tests/position-pnl-tiers.test.ts::liquidationPriceUsesTiers (planned) |
| <code>acceptance.3.531.1</code> | qualification | UNRUN |
| <code>req.3531</code> | ac_1 | computePositionPnl (apps/perps/src/core/perps/pnlHelpers.ts:148) of a 2e6 USD long with tier maintenanceBps 250 and deduction 1000 USD returns the liquidation price where collateral equals that maintenance, not the 10 percent of apps/perps/src/core/perps/pnlHelpers.ts:92. |
| <code>acceptance.3.532.1</code> | req | 3532.1 |
| <code>acceptance.3.532.1</code> | scenario | Given a local node with LOCAL_NODE_RPC, LOCAL_NODE_KEY and NEXT_PUBLIC_PERPS_GATEWAY_URL naming a local node and gateway, when an aligned and an off-tick BTC limit order are sent, then the first is mined and the second reverts PerpsTickSize |
| <code>acceptance.3.532.1</code> | test | apps/perps/tests/local-order-tick.test.ts::localOffTickOrderReverts (planned) |
| <code>acceptance.3.532.1</code> | qualification | UNRUN |
| <code>req.3532</code> | ac_1 | On the local node an off-tick placeOrder reverts with PerpsTickSize classified VALIDATION_PRECISION by apps/perps/src/core/errors/classify.ts:322 after task 3.47.3, and the aligned order is accepted. |
| <code>acceptance.3.533.1</code> | req | 3533.1 |
| <code>acceptance.3.533.1</code> | scenario | Given a local node with LOCAL_NODE_RPC, LOCAL_NODE_KEY and NEXT_PUBLIC_PERPS_GATEWAY_URL naming a local node and gateway, when every active market config is read and decoded, then each has a symbol and positive tick and lot sizes |
| <code>acceptance.3.533.1</code> | test | apps/perps/tests/local-market-config.test.ts::localMarketConfigDecodes (planned) |
| <code>acceptance.3.533.1</code> | qualification | UNRUN |
| <code>req.3533</code> | ac_1 | For every id of getActiveMarketIds (apps/perps/src/core/network/abi/PerpsPrecompile.ts:1630) on the local node decodeMarketConfig returns a symbol, tickSize above 0 and lotSize above 0. |
| <code>acceptance.3.534.1</code> | req | 3534.1 |
| <code>acceptance.3.534.1</code> | scenario | Given a local node with LOCAL_NODE_RPC, LOCAL_NODE_KEY and NEXT_PUBLIC_PERPS_GATEWAY_URL naming a local node and gateway and an account with no volume, when getFeeTier is read and mapped, then the view is tier 0 with the base taker fee |
| <code>acceptance.3.534.1</code> | test | apps/perps/tests/local-fee-tier.test.ts::localFeeTierDecodes (planned) |
| <code>acceptance.3.534.1</code> | qualification | UNRUN |
| <code>req.3534</code> | ac_1 | feeTierView of getFeeTier (apps/perps/src/core/network/abi/PerpsPrecompile.ts:1711) for a fresh local account returns tier 0 with takerBps equal to getFees takerFeePpm / 100. |
| <code>acceptance.3.535.1</code> | req | 3535.1 |
| <code>acceptance.3.535.1</code> | scenario | Given a local node with LOCAL_NODE_RPC, LOCAL_NODE_KEY and NEXT_PUBLIC_PERPS_GATEWAY_URL naming a local node and gateway, when the USDL vault state is read and mapped, then it is tradeable and not paused |
| <code>acceptance.3.535.1</code> | test | apps/perps/tests/local-spot-token-state.test.ts::localStableTokenTradeable (planned) |
| <code>acceptance.3.535.1</code> | qualification | UNRUN |
| <code>req.3535</code> | ac_1 | spotTokenState of the vault answers of apps/perps/src/core/network/spot_abi/PECORVault.ts:353 and apps/perps/src/core/network/spot_abi/PECORVault.ts:247 for USDL on the local node is tradeable with paused false. |
| <code>capability.3.47.1</code> | title | Conformance: the app perps ABI equals precompiles/perps/abi/abi.json |
| <code>capability.3.47.1</code> | reqs | 3528.1 |
| <code>capability.3.47.2</code> | title | Conformance: the app spot ABIs equal precompiles/spot/abi |
| <code>capability.3.47.2</code> | reqs | 3529.1 |
| <code>capability.3.47.3</code> | title | Map the perps limit reverts and TokenPaused to specific error codes |
| <code>capability.3.47.3</code> | reqs | 3515.1<br>3515.2 |
| <code>capability.3.48.1</code> | title | Decode getMarketConfig and getMarginTiers in a core marketConfig module |
| <code>capability.3.48.1</code> | reqs | 3509.1<br>3509.2<br>3509.3 |
| <code>capability.3.48.10</code> | title | Label markets from the catalog instead of MKT-id fallbacks |
| <code>capability.3.48.10</code> | reqs | 3517.1 |
| <code>capability.3.48.13</code> | title | Classify a market as open, stale or disabled from getMarkPriceDetails |
| <code>capability.3.48.13</code> | reqs | 3512.1 |
| <code>capability.3.48.2</code> | title | Check tick, lot and leverage limits and align prices in orderPlan |
| <code>capability.3.48.2</code> | reqs | 3510.1<br>3510.2<br>3510.3 |
| <code>capability.3.48.3</code> | title | Build the app market catalog from the gateway markets answer |
| <code>capability.3.48.3</code> | reqs | 3511.1<br>3511.2 |
| <code>capability.3.48.9</code> | title | Register the venue stream symbols from the market catalog |
| <code>capability.3.48.9</code> | reqs | 3516.1 |
| <code>capability.3.52.1</code> | title | Read spot token pause and info from the vault in a core tokenState module |
| <code>capability.3.52.1</code> | reqs | 3513.1<br>3513.2 |
| <code>capability.3.53.1</code> | title | Decode the account fee tier from getFeeTier |
| <code>capability.3.53.1</code> | reqs | 3514.1 |
| <code>capability.3.54.2</code> | title | Local node: an aligned limit order lands and an off-tick one reverts PerpsTickSize |
| <code>capability.3.54.2</code> | reqs | 3532.1 |
| <code>capability.3.54.3</code> | title | Local node: getMarketConfig decodes for every active market |
| <code>capability.3.54.3</code> | reqs | 3533.1 |
| <code>capability.3.54.5</code> | title | Local node: getFeeTier of a fresh account decodes to tier 0 |
| <code>capability.3.54.5</code> | reqs | 3534.1 |
| <code>capability.3.54.6</code> | title | Local node: the vault reports the stable token as unpaused |
| <code>capability.3.54.6</code> | reqs | 3535.1 |
| <code>capability.3.55.1</code> | title | Retire the spot-design exports and the static perps market list |
| <code>capability.3.55.1</code> | reqs | 3530.1 |
| <code>capability.3.59.1</code> | title | Compute app liquidation prices with the chain tiered maintenance |
| <code>capability.3.59.1</code> | reqs | 3531.1 |
| <code>resolution.17.603</code> | title | decision.341 binds: 0x1015 is served by the Go perps precompile and the app ABI equals its JSON |
| <code>resolution.17.603</code> | applies_to | decision.341<br>task.3.47.1<br>task.3.47.2 |
| <code>resolution.17.603</code> | source | decision.341 in fragments/J27/decision.kvx; tasks from affects |
| <code>resolution.17.609</code> | title | decision.347 binds: Minimum and maximum order value are enforced by mapping the chain reverts |
| <code>resolution.17.609</code> | applies_to | decision.347<br>task.3.47.3 |
| <code>resolution.17.609</code> | source | decision.347 in fragments/J27/decision.kvx; tasks from affects |
| <code>resolution.17.612</code> | title | decision.350 binds: Spot venue topics stay fixed to PAX and SID |
| <code>resolution.17.612</code> | applies_to | decision.350<br>task.3.48.9 |
| <code>resolution.17.612</code> | source | decision.350 in fragments/J27/decision.kvx; tasks from affects |

## env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-perps-node-env</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/log/logger.ts:32</code> | <code>NODE_ENV</code> |
| <code>interface.3.env-perps-market-proxy-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/pages/api/candle/[...path].ts:5</code> | <code>MARKET_PROXY_HOST</code> |
| <code>interface.3.env-perps-market-proxy-origin</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/instrumentation.ts:7</code> | <code>MARKET_PROXY_ORIGIN</code> |
| <code>interface.3.env-perps-next-public-auth-redirect-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/client.ts:75</code> | <code>NEXT_PUBLIC_AUTH_REDIRECT_URL</code> |
| <code>interface.3.env-perps-next-public-launchpad-precompile</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/network/precompiles.ts:20</code> | <code>NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE</code> |
| <code>interface.3.env-perps-next-public-log-level</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/log/logger.ts:30</code> | <code>NEXT_PUBLIC_LOG_LEVEL</code> |
| <code>interface.3.env-perps-next-public-market-proxy-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/feeds.ts:2</code> | <code>NEXT_PUBLIC_MARKET_PROXY_HOST</code> |
| <code>interface.3.env-perps-next-public-paxeer-rpc-urls</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/rpc.ts:63</code> | <code>NEXT_PUBLIC_PAXEER_RPC_URLS</code> |
| <code>interface.3.env-perps-next-public-paxeer-wallet-api</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/client.ts:30</code> | <code>NEXT_PUBLIC_PAXEER_WALLET_API</code> |
| <code>interface.3.env-perps-next-public-paxscan-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/config.ts:20</code> | <code>NEXT_PUBLIC_PAXSCAN_API_URL</code> |
| <code>interface.3.env-perps-next-public-paxscan-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/config.ts:1</code> | <code>NEXT_PUBLIC_PAXSCAN_BASE_URL</code> |
| <code>interface.3.env-perps-next-public-paxscan-ws-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/paxscan/config.ts:21</code> | <code>NEXT_PUBLIC_PAXSCAN_WS_URL</code> |
| <code>interface.3.env-perps-next-public-pecor-market-precompile</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/network/precompiles.ts:26</code> | <code>NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE</code> |
| <code>interface.3.env-perps-next-public-pecor-orders-precompile</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/network/precompiles.ts:31</code> | <code>NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE</code> |
| <code>interface.3.env-perps-next-public-pecor-quoter-precompile</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/network/precompiles.ts:41</code> | <code>NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE</code> |
| <code>interface.3.env-perps-next-public-pecor-stop-precompile</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/network/precompiles.ts:36</code> | <code>NEXT_PUBLIC_PECOR_STOP_PRECOMPILE</code> |
| <code>interface.3.env-perps-next-public-pecor-vault-precompile</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/network/precompiles.ts:46</code> | <code>NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE</code> |
| <code>interface.3.env-perps-next-public-perps-gateway-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/network/gateway.ts:1</code> | <code>NEXT_PUBLIC_PERPS_GATEWAY_URL</code> |
| <code>interface.3.env-perps-next-public-perps-precompile</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/network/precompiles.ts:15</code> | <code>NEXT_PUBLIC_PERPS_PRECOMPILE</code> |
| <code>interface.3.env-perps-next-public-release-spot-open</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/nav.ts:17</code> | <code>NEXT_PUBLIC_RELEASE_SPOT_OPEN</code> |
| <code>interface.3.env-perps-next-public-status-page-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/app/shell/Footer.tsx:4</code> | <code>NEXT_PUBLIC_STATUS_PAGE_URL</code> |
| <code>interface.3.env-perps-next-public-supabase-publishable-key</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/client.ts:32</code> | <code>NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY</code> |
| <code>interface.3.env-perps-next-public-supabase-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/embedded/client.ts:31</code> | <code>NEXT_PUBLIC_SUPABASE_URL</code> |
| <code>interface.3.env-perps-next-public-vercel-git-commit-sha</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/pages/_document.tsx:9</code> | <code>NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA</code> |
| <code>interface.3.env-perps-next-public-wallet-iframe-origin</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/core/wallet/postMessageEip1193.ts:143</code> | <code>NEXT_PUBLIC_WALLET_IFRAME_ORIGIN</code> |
| <code>interface.3.env-perps-next-runtime</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/instrumentation.ts:3</code> | <code>NEXT_RUNTIME</code> |
| <code>interface.3.env-perps-railway-git-commit-sha</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/pages/api/build-id.ts:11</code> | <code>RAILWAY_GIT_COMMIT_SHA</code> |
| <code>interface.3.env-perps-template-chain-id</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/env/.env.template:21</code> | <code>CHAIN_ID=</code> |
| <code>interface.3.env-perps-template-chain-name</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/env/.env.template:22</code> | <code>CHAIN_NAME=</code> |
| <code>interface.3.env-perps-template-explorer-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/env/.env.template:23</code> | <code>EXPLORER_URL=</code> |
| <code>interface.3.env-perps-template-native-symbol</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/env/.env.template:24</code> | <code>NATIVE_SYMBOL=</code> |
| <code>interface.3.env-perps-template-rpc-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/env/.env.template:20</code> | <code>RPC_URL=</code> |
| <code>interface.3.env-perps-template-usdl-address</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/env/.env.template:27</code> | <code>USDL_ADDRESS=</code> |
| <code>interface.3.env-perps-template-walletconnect-project-id</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/env/.env.template:43</code> | <code>WALLETCONNECT_PROJECT_ID=</code> |
| <code>interface.3.env-perps-vercel-git-commit-sha</code> | <code>env_var</code> | <code>stable</code> | <code>apps/perps/src/pages/api/build-id.ts:11</code> | <code>VERCEL_GIT_COMMIT_SHA</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-perps-node-env</code> | notes | read by perps, web; set by the runtime or the hosting platform, not by the app |
| <code>interface.3.env-perps-next-public-auth-redirect-url</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-launchpad-precompile</code> | notes | declared in apps/perps/src/env/.env.template:35 |
| <code>interface.3.env-perps-next-public-log-level</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-market-proxy-host</code> | notes | declared in apps/perps/src/env/.env.template:31 |
| <code>interface.3.env-perps-next-public-paxeer-rpc-urls</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-paxeer-wallet-api</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-paxscan-api-url</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-paxscan-base-url</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-paxscan-ws-url</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-pecor-market-precompile</code> | notes | declared in apps/perps/src/env/.env.template:36 |
| <code>interface.3.env-perps-next-public-pecor-orders-precompile</code> | notes | declared in apps/perps/src/env/.env.template:37 |
| <code>interface.3.env-perps-next-public-pecor-quoter-precompile</code> | notes | declared in apps/perps/src/env/.env.template:39 |
| <code>interface.3.env-perps-next-public-pecor-stop-precompile</code> | notes | declared in apps/perps/src/env/.env.template:38 |
| <code>interface.3.env-perps-next-public-pecor-vault-precompile</code> | notes | declared in apps/perps/src/env/.env.template:40 |
| <code>interface.3.env-perps-next-public-perps-gateway-url</code> | notes | declared in apps/perps/src/env/.env.template:30 |
| <code>interface.3.env-perps-next-public-perps-precompile</code> | notes | declared in apps/perps/src/env/.env.template:34 |
| <code>interface.3.env-perps-next-public-release-spot-open</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-status-page-url</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-supabase-publishable-key</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-supabase-url</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-public-vercel-git-commit-sha</code> | notes | not declared in apps/perps/src/env/.env.template; set by the runtime or the hosting platform, not by the app |
| <code>interface.3.env-perps-next-public-wallet-iframe-origin</code> | notes | not declared in apps/perps/src/env/.env.template |
| <code>interface.3.env-perps-next-runtime</code> | notes | set by the runtime or the hosting platform, not by the app |
| <code>interface.3.env-perps-railway-git-commit-sha</code> | notes | set by the runtime or the hosting platform, not by the app |
| <code>interface.3.env-perps-template-chain-id</code> | notes | template drift: declared in the template but no code under apps/perps/src reads it |
| <code>interface.3.env-perps-template-chain-id</code> | overridden_by | override.J27.8 |
| <code>interface.3.env-perps-template-chain-name</code> | notes | template drift: declared in the template but no code under apps/perps/src reads it |
| <code>interface.3.env-perps-template-chain-name</code> | overridden_by | override.J27.9 |
| <code>interface.3.env-perps-template-explorer-url</code> | notes | template drift: declared in the template but no code under apps/perps/src reads it |
| <code>interface.3.env-perps-template-explorer-url</code> | overridden_by | override.J27.10 |
| <code>interface.3.env-perps-template-native-symbol</code> | notes | template drift: declared in the template but no code under apps/perps/src reads it |
| <code>interface.3.env-perps-template-native-symbol</code> | overridden_by | override.J27.11 |
| <code>interface.3.env-perps-template-rpc-url</code> | notes | template drift: declared in the template but no code under apps/perps/src reads it |
| <code>interface.3.env-perps-template-rpc-url</code> | overridden_by | override.J27.7 |
| <code>interface.3.env-perps-template-usdl-address</code> | notes | template drift: declared in the template but no code under apps/perps/src reads it |
| <code>interface.3.env-perps-template-usdl-address</code> | overridden_by | override.J27.12 |
| <code>interface.3.env-perps-template-walletconnect-project-id</code> | notes | template drift: declared in the template but no code under apps/perps/src reads it |
| <code>interface.3.env-perps-template-walletconnect-project-id</code> | overridden_by | override.J27.13 |
| <code>interface.3.env-perps-vercel-git-commit-sha</code> | notes | set by the runtime or the hosting platform, not by the app |

Contract context <code>contract_context.3.apps-perps-src-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 7 surfaces under apps/perps/src (7 env_var; 7 change) frozen at the anchors contracts/perps-client.md#env: CHAIN_ID= (apps/perps/src/env/.env.template:21); CHAIN_NAME= (apps/perps/src/env/.env.template:22); EXPLORER_URL= (apps/perps/src/env/.env.template:23); NATIVE_SYMBOL= (apps/perps/src/env/.env.template:24); RPC_URL= (apps/perps/src/env/.env.template:20); USDL_ADDRESS= (apps/perps/src/env/.env.template:27); WALLETCONNECT_PROJECT_ID= (apps/perps/src/env/.env.template:43) |
| storage | no storage_key item in this context; the apps/perps/src items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | no task names these items yet |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

## hooks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-hooks-chain-useevmblockstream-evmblockstreamstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/chain/useEvmBlockStream.ts:93</code> | <code>export interface EvmBlockStreamState</code> |
| <code>interface.3.ts-perps-hooks-chain-useevmblockstream-useevmblockstream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/chain/useEvmBlockStream.ts:97</code> | <code>export function useEvmBlockStream( options: UseEvmBlockStreamOptions = {}, ): EvmBlockStreamState</code> |
| <code>interface.3.ts-perps-hooks-chain-useevmblockstream-useevmblockstreamoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/chain/useEvmBlockStream.ts:84</code> | <code>export interface UseEvmBlockStreamOptions</code> |
| <code>interface.3.ts-perps-hooks-chain-useexplorerstream-explorer-ws-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/chain/useExplorerStream.ts:30</code> | <code>export const EXPLORER_WS_URL = `${FEED_ORIGIN.replace(/^http/, "ws")}/ws/explorer`;</code> |
| <code>interface.3.ts-perps-hooks-chain-useexplorerstream-explorerblock</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/chain/useExplorerStream.ts:40</code> | <code>export interface ExplorerBlock</code> |
| <code>interface.3.ts-perps-hooks-chain-useexplorerstream-explorertx</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/chain/useExplorerStream.ts:50</code> | <code>export interface ExplorerTx</code> |
| <code>interface.3.ts-perps-hooks-chain-useexplorerstream-useexplorerstream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/chain/useExplorerStream.ts:118</code> | <code>export function useExplorerStream(): UseExplorerStreamResult</code> |
| <code>interface.3.ts-perps-hooks-chain-useexplorerstream-useexplorerstreamresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/chain/useExplorerStream.ts:65</code> | <code>export interface UseExplorerStreamResult</code> |
| <code>interface.3.ts-perps-hooks-paxscan-useblockscoutinfinitequery-paxscanpageparam</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts:34</code> | <code>export type PaxscanPageParam =</code> |
| <code>interface.3.ts-perps-hooks-paxscan-useblockscoutinfinitequery-useblockscoutinfinitequery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts:53</code> | <code>export function useBlockscoutInfiniteQuery&lt;R extends PaginatedResourceName&gt;( resource: R, opts: UseBlockscoutInfiniteQueryOptions&lt;R&gt; = {}, ): UseInfiniteQueryResult&lt;InfiniteData&lt;ResourcePayload&lt;R&gt;&gt;, PaxscanError&gt;</code> |
| <code>interface.3.ts-perps-hooks-paxscan-useblockscoutinfinitequery-useblockscoutinfinitequeryoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/paxscan/useBlockscoutInfiniteQuery.ts:38</code> | <code>export interface UseBlockscoutInfiniteQueryOptions&lt;R extends PaginatedResourceName&gt;</code> |
| <code>interface.3.ts-perps-hooks-paxscan-useblockscoutquery-useblockscoutquery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/paxscan/useBlockscoutQuery.ts:38</code> | <code>export function useBlockscoutQuery&lt;R extends ResourceName&gt;( resource: R, opts: UseBlockscoutQueryOptions&lt;R&gt; = {}, ): UseQueryResult&lt;ResourcePayload&lt;R&gt;, PaxscanError&gt;</code> |
| <code>interface.3.ts-perps-hooks-paxscan-useblockscoutquery-useblockscoutqueryoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/paxscan/useBlockscoutQuery.ts:29</code> | <code>export interface UseBlockscoutQueryOptions&lt;R extends ResourceName&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useaccount-deposit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAccount.ts:50</code> | <code>export async function deposit( account: Address, amount: bigint, onStep?: (step: DepositStep) =&gt; void, ): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useaccount-depositstep</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAccount.ts:18</code> | <code>export type DepositStep = "approving" &#124; "executing";</code> |
| <code>interface.3.ts-perps-hooks-perps-useaccount-getaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAccount.ts:21</code> | <code>export async function getAccount(account: Address): Promise&lt;AccountView&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useaccount-getallowance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAccount.ts:37</code> | <code>export async function getAllowance(account: Address): Promise&lt;bigint&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useaccount-getwalletbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAccount.ts:27</code> | <code>export async function getWalletBalance(account: Address): Promise&lt;bigint&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useaccount-useaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAccount.ts:80</code> | <code>export function useAccount()</code> |
| <code>interface.3.ts-perps-hooks-perps-useaccount-withdraw</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAccount.ts:71</code> | <code>export async function withdraw(amount: bigint): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useaccount-withdrawall</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAccount.ts:76</code> | <code>export async function withdrawAll(account: Address): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useanalytics-usemarketsnapshots</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAnalytics.ts:72</code> | <code>export function useMarketSnapshots(marketId: number &#124; undefined, limit = 100)</code> |
| <code>interface.3.ts-perps-hooks-perps-useanalytics-usemarkpricehistory</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAnalytics.ts:45</code> | <code>export function useMarkPriceHistory(marketId: number &#124; undefined, limit = 100)</code> |
| <code>interface.3.ts-perps-hooks-perps-useanalytics-useoisnapshots</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAnalytics.ts:33</code> | <code>export function useOiSnapshots(marketId: number &#124; undefined, limit = 100)</code> |
| <code>interface.3.ts-perps-hooks-perps-useanalytics-useprotocolsnapshots</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAnalytics.ts:84</code> | <code>export function useProtocolSnapshots(limit = 100)</code> |
| <code>interface.3.ts-perps-hooks-perps-useanalytics-usetradesettlements</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAnalytics.ts:99</code> | <code>export function useTradeSettlements(params?: { userAddress?: string; marketId?: number; positionId?: string; tradeType?: number; limit?: number; offset?: number; })</code> |
| <code>interface.3.ts-perps-hooks-perps-useanalytics-usevaultbalancehistory</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAnalytics.ts:57</code> | <code>export function useVaultBalanceHistory( tokenAddress?: string, vaultType?: number, limit = 100, )</code> |
| <code>interface.3.ts-perps-hooks-perps-useautocollateral-autocollateralresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAutoCollateral.ts:10</code> | <code>export interface AutoCollateralResult</code> |
| <code>interface.3.ts-perps-hooks-perps-useautocollateral-useautocollateral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useAutoCollateral.ts:24</code> | <code>export function useAutoCollateral(): AutoCollateralResult</code> |
| <code>interface.3.ts-perps-hooks-perps-usecontract-readcontract</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useContract.ts:9</code> | <code>export async function readContract(params: { address: Address; abi: readonly unknown[]; functionName: string; args?: readonly unknown[]; })</code> |
| <code>interface.3.ts-perps-hooks-perps-usecontract-waitfortx</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useContract.ts:150</code> | <code>export async function waitForTx(hash: `0x${string}`): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-usecontract-waitfortxresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useContract.ts:132</code> | <code>export interface WaitForTxResult</code> |
| <code>interface.3.ts-perps-hooks-perps-usecontract-writecontract</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useContract.ts:50</code> | <code>export async function writeContract(params: { address: Address; abi: readonly unknown[]; functionName: string; args?: readonly unknown[]; value?: bigint; }): Promise&lt;`0x$</code> |
| <code>interface.3.ts-perps-hooks-perps-usedailypnl-dailypnl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useDailyPnl.ts:8</code> | <code>export interface DailyPnl</code> |
| <code>interface.3.ts-perps-hooks-perps-usedailypnl-dailypnlresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useDailyPnl.ts:39</code> | <code>export interface DailyPnlResult</code> |
| <code>interface.3.ts-perps-hooks-perps-usedailypnl-fetchdailypnl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useDailyPnl.ts:77</code> | <code>export async function fetchDailyPnl(addr: string): Promise&lt;DailyPnlResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-usedailypnl-usedailypnl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useDailyPnl.ts:88</code> | <code>export function useDailyPnl(address: string &#124; undefined):</code> |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useenrichedglobalstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useEnrichedStats.ts:37</code> | <code>export function useEnrichedGlobalStats()</code> |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useenrichedmarketstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useEnrichedStats.ts:50</code> | <code>export function useEnrichedMarketStats(marketId: number &#124; undefined)</code> |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useorderlyfundingrates</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useEnrichedStats.ts:78</code> | <code>export function useOrderlyFundingRates()</code> |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useorderlyopeninterests</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useEnrichedStats.ts:104</code> | <code>export function useOrderlyOpenInterests()</code> |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useorderlypricechanges</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useEnrichedStats.ts:91</code> | <code>export function useOrderlyPriceChanges()</code> |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useorderlytickers</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useEnrichedStats.ts:65</code> | <code>export function useOrderlyTickers()</code> |
| <code>interface.3.ts-perps-hooks-perps-usefees-usefees</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useFees.ts:12</code> | <code>export function useFees(params?: { userAddress?: string; marketId?: number; positionId?: string; feeType?: number; limit?: number; offset?: number; })</code> |
| <code>interface.3.ts-perps-hooks-perps-usefundingpayments-usefundingpayments</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useFundingPayments.ts:12</code> | <code>export function useFundingPayments(params?: { userAddress?: string; marketId?: number; positionId?: string; limit?: number; offset?: number; })</code> |
| <code>interface.3.ts-perps-hooks-perps-usegraphqlquery-sharedpolledqueryoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useGraphQLQuery.ts:26</code> | <code>export interface SharedPolledQueryOptions&lt;TRaw, TPicked&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-usegraphqlquery-sharedpolledqueryresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useGraphQLQuery.ts:48</code> | <code>export interface SharedPolledQueryResult&lt;T&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-usegraphqlquery-usesharedpolledquery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useGraphQLQuery.ts:63</code> | <code>export function useSharedPolledQuery&lt;TRaw = unknown, TPicked = TRaw&gt;( opts: SharedPolledQueryOptions&lt;TRaw, TPicked&gt;, ): SharedPolledQueryResult&lt;TPicked&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerliquidation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:68</code> | <code>export interface IndexerLiquidation</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:50</code> | <code>export interface IndexerOrder</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:39</code> | <code>export interface IndexerPosition</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerpriceupdate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:32</code> | <code>export interface IndexerPriceUpdate</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:24</code> | <code>export interface IndexerStatus</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexertrade</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:59</code> | <code>export interface IndexerTrade</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-useindexerstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:90</code> | <code>export function useIndexerStatus()</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-uselatestpricessubscription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:117</code> | <code>export function useLatestPricesSubscription()</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-useliquidationsubscription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:269</code> | <code>export function useLiquidationSubscription(marketId: number &#124; undefined)</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-useorderchangedsubscription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:208</code> | <code>export function useOrderChangedSubscription(userAddress: string &#124; undefined)</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-usepositionchangedsubscription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:177</code> | <code>export function usePositionChangedSubscription(userAddress: string &#124; undefined)</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-usesinglemarketpricesubscription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:144</code> | <code>export function useSingleMarketPriceSubscription(marketId: number &#124; undefined)</code> |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-usetradecreatedsubscription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useIndexerSubscriptions.ts:239</code> | <code>export function useTradeCreatedSubscription(marketId: number &#124; undefined)</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarginmode-useaccountsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarginMode.ts:84</code> | <code>export function useAccountSummary( userAddress: `0x${string}` &#124; undefined, tokenAddress: `0x${string}` &#124; undefined ):</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarginmode-usepositionmargin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarginMode.ts:12</code> | <code>export function usePositionMargin( userAddress: `0x${string}` &#124; undefined, positionId: bigint &#124; undefined ):</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarginmode-usetopupposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarginMode.ts:57</code> | <code>export function useTopUpPosition(userAddress: `0x${string}` &#124; undefined):</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-connectionstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:76</code> | <code>export type ConnectionState = "idle" &#124; "connecting" &#124; "open" &#124; "closed";</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-marketfundinghourly</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:309</code> | <code>export function marketFundingHourly(symbol: string, field: string, v: unknown): bigint &#124; null</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-markets-ws-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:27</code> | <code>export const MARKETS_WS_URL = `${FEED_ORIGIN.replace(/^http/, "ws")}/ws/markets`;</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-marketsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:35</code> | <code>export interface MarketSnapshot</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-marketstotals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:66</code> | <code>export interface MarketsTotals</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-normalizemarket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:324</code> | <code>export function normalizeMarket(raw: unknown): MarketSnapshot &#124; null</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-normalizemarketsframe</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:354</code> | <code>export function normalizeMarketsFrame(data: unknown[]): MarketSnapshot[]</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-parsevenuefundinghourly</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:296</code> | <code>export function parseVenueFundingHourly(v: unknown): bigint &#124; null</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-usemarketsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:404</code> | <code>export function useMarketSnapshot(symbol: string &#124; undefined &#124; null): MarketSnapshot &#124; undefined</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-usemarketssnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:378</code> | <code>export function useMarketsSnapshot(enabled = true): MarketSnapshot[]</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-usemarketsstreamconnection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:459</code> | <code>export function useMarketsStreamConnection(): ConnectionState</code> |
| <code>interface.3.ts-perps-hooks-perps-usemarketsstream-usemarketstotals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useMarketsStream.ts:439</code> | <code>export function useMarketsTotals(): MarketsTotals &#124; null</code> |
| <code>interface.3.ts-perps-hooks-perps-useorderlinestore-draftorderlines</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useOrderLineStore.ts:3</code> | <code>export interface DraftOrderLines</code> |
| <code>interface.3.ts-perps-hooks-perps-useorderlinestore-useorderlinestore</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useOrderLineStore.ts:38</code> | <code>export const useOrderLineStore = create&lt;DraftOrderLines&gt;()((set) =&gt; (</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbalerts-alerttype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbAlerts.ts:7</code> | <code>export type AlertType =</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbalerts-perfdbalert</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbAlerts.ts:17</code> | <code>export interface PerfdbAlert</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbalerts-useperfdbalerts</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbAlerts.ts:69</code> | <code>export function usePerfdbAlerts(address: string &#124; undefined):</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbauth-useperfdbauth</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbAuth.ts:92</code> | <code>export function usePerfdbAuth(): UsePerfdbAuthReturn</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbauth-useperfdbauthreturn</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbAuth.ts:15</code> | <code>export interface UsePerfdbAuthReturn extends AuthState</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbbalances-perfdbbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbBalances.ts:7</code> | <code>export interface PerfdbBalance</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbbalances-useperfdbbalances</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbBalances.ts:38</code> | <code>export function usePerfdbBalances(address: string &#124; undefined):</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-competition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbCompetitions.ts:16</code> | <code>export interface Competition</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-competitiondetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbCompetitions.ts:40</code> | <code>export interface CompetitionDetail extends Competition</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-competitionmetric</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbCompetitions.ts:13</code> | <code>export type CompetitionMetric = "pnl" &#124; "volume" &#124; "trades" &#124; "roi" &#124; string;</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-competitionranking</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbCompetitions.ts:33</code> | <code>export interface CompetitionRanking</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-competitionstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbCompetitions.ts:14</code> | <code>export type CompetitionStatus = "upcoming" &#124; "active" &#124; "ended";</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-useperfdbcompetitiondetail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbCompetitions.ts:186</code> | <code>export function usePerfdbCompetitionDetail(id: string &#124; null):</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-useperfdbcompetitions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbCompetitions.ts:143</code> | <code>export function usePerfdbCompetitions():</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbreferrals-referraltier</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbReferrals.ts:15</code> | <code>export interface ReferralTier</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbreferrals-useperfdbreferrals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbReferrals.ts:116</code> | <code>export function usePerfdbReferrals(address: string &#124; undefined):</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbreferrals-userreferraldata</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbReferrals.ts:28</code> | <code>export interface UserReferralData</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbsubscriptions-priceupdate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts:18</code> | <code>export interface PriceUpdate</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbsubscriptions-tradeupdate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts:24</code> | <code>export interface TradeUpdate</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbsubscriptions-usepricesubscription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts:44</code> | <code>export function usePriceSubscription(marketId?: number)</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbsubscriptions-usetradesubscription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbSubscriptions.ts:80</code> | <code>export function useTradeSubscription(marketId?: number)</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbtraderstats-perfdbtraderstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbTraderStats.ts:7</code> | <code>export interface PerfdbTraderStats</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbtraderstats-useperfdbtraderstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbTraderStats.ts:44</code> | <code>export function usePerfdbTraderStats(address: string &#124; undefined):</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbuser-perfdbuser</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbUser.ts:7</code> | <code>export interface PerfdbUser</code> |
| <code>interface.3.ts-perps-hooks-perps-useperfdbuser-useperfdbuser</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerfdbUser.ts:36</code> | <code>export function usePerfdbUser(address: string &#124; undefined):</code> |
| <code>interface.3.ts-perps-hooks-perps-useperpsorders-useperpsorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerpsOrders.ts:19</code> | <code>export function usePerpsOrders()</code> |
| <code>interface.3.ts-perps-hooks-perps-useperpswallet-useperpswallet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePerpsWallet.ts:43</code> | <code>export function usePerpsWallet()</code> |
| <code>interface.3.ts-perps-hooks-perps-useportfoliostats-markpositionlocallyclosed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePortfolioStats.ts:97</code> | <code>export function markPositionLocallyClosed(positionId: string &#124; bigint)</code> |
| <code>interface.3.ts-perps-hooks-perps-useportfoliostats-portfoliostats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePortfolioStats.ts:17</code> | <code>export interface PortfolioStats</code> |
| <code>interface.3.ts-perps-hooks-perps-useportfoliostats-refreshpositions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePortfolioStats.ts:167</code> | <code>export function refreshPositions()</code> |
| <code>interface.3.ts-perps-hooks-perps-useportfoliostats-unmarkpositionlocallyclosed</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePortfolioStats.ts:104</code> | <code>export function unmarkPositionLocallyClosed(positionId: string &#124; bigint)</code> |
| <code>interface.3.ts-perps-hooks-perps-useportfoliostats-useportfoliostats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePortfolioStats.ts:222</code> | <code>export function usePortfolioStats(): PortfolioStats</code> |
| <code>interface.3.ts-perps-hooks-perps-useposition-perpsallowancesource</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePosition.ts:40</code> | <code>export function perpsAllowanceSource(): AllowanceSource</code> |
| <code>interface.3.ts-perps-hooks-perps-useposition-sendplan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePosition.ts:46</code> | <code>export async function sendPlan(plan: WritePlan): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-useposition-useposition</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/usePosition.ts:57</code> | <code>export function usePosition()</code> |
| <code>interface.3.ts-perps-hooks-perps-usequoter-closepositionquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useQuoter.ts:30</code> | <code>export interface ClosePositionQuote</code> |
| <code>interface.3.ts-perps-hooks-perps-usequoter-marketquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useQuoter.ts:40</code> | <code>export type { MarketQuote, PartialCloseQuote };</code> |
| <code>interface.3.ts-perps-hooks-perps-usequoter-openpositionquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useQuoter.ts:17</code> | <code>export interface OpenPositionQuote</code> |
| <code>interface.3.ts-perps-hooks-perps-usequoter-partialclosequote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useQuoter.ts:40</code> | <code>export type { MarketQuote, PartialCloseQuote };</code> |
| <code>interface.3.ts-perps-hooks-perps-usequoter-quotermarketquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useQuoter.ts:47</code> | <code>export interface QuoterMarketQuote extends MarketQuote</code> |
| <code>interface.3.ts-perps-hooks-perps-usequoter-usequoter</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/hooks/perps/useQuoter.ts:89</code> | <code>export function useQuoter()</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-fetchtickers</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:69</code> | <code>export function fetchTickers(): Promise&lt;TickerMap&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-realtimepricesresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:25</code> | <code>export interface RealtimePricesResult</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickerentry</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:6</code> | <code>export interface TickerEntry</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickerfromsnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:30</code> | <code>export function tickerFromSnapshot(row: MarketSnapshot): TickerEntry</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickerfromwire</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:44</code> | <code>export function tickerFromWire(row: TickerWire): TickerEntry</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickermap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:23</code> | <code>export type TickerMap = Record&lt;string, TickerEntry&gt;;</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickersfromwire</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:49</code> | <code>export function tickersFromWire(rows: TickerWire[]): TickerMap</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickerwire</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:42</code> | <code>export type TickerWire = Omit&lt;TickerEntry, "last_funding_rate_hourly"&gt; &amp; { last_funding_rate_hourly: string &#124; null };</code> |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-userealtimeprices</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useRealtimePrices.ts:53</code> | <code>export function useRealtimePrices({ enabled = true }: { enabled?: boolean } = {}): RealtimePricesResult</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-hasdedicatedklinestream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:583</code> | <code>export function hasDedicatedKlineStream(symbol: string &#124; undefined &#124; null): boolean</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-hasdedicatedmarketstream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:70</code> | <code>export function hasDedicatedMarketStream(symbol: string &#124; undefined &#124; null): boolean</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-sidpaxbooklevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:76</code> | <code>export interface SidPaxBookLevel</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-sidpaxcandle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:144</code> | <code>export interface SidPaxCandle</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-sidpaxklinesubscriber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:159</code> | <code>export interface SidPaxKlineSubscriber</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-sidpaxorderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:81</code> | <code>export interface SidPaxOrderbook</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-sidpaxstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:105</code> | <code>export interface SidPaxStats</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-sidpaxtrade</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:87</code> | <code>export interface SidPaxTrade</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-subscribekline</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:635</code> | <code>export function subscribeKline( symbol: string &#124; undefined &#124; null, interval: string &#124; undefined &#124; null, subscriber: SidPaxKlineSubscriber, limit?: number, ): () =&gt; void</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-subscribeorderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:480</code> | <code>export function subscribeOrderbook(symbol: string, cb: (book: SidPaxOrderbook &#124; null) =&gt; void): () =&gt; void</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-tvresolutiontoklineinterval</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:597</code> | <code>export function tvResolutionToKlineInterval(resolution: string &#124; undefined &#124; null): string &#124; null</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-usesidpaxorderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:497</code> | <code>export function useSidPaxOrderbook(symbol: string &#124; undefined): SidPaxOrderbook &#124; null</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-usesidpaxstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:551</code> | <code>export function useSidPaxStats(symbol: string &#124; undefined): SidPaxStats &#124; null</code> |
| <code>interface.3.ts-perps-hooks-perps-usesidpaxmarketstream-usesidpaxtrades</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSidPaxMarketStream.ts:522</code> | <code>export function useSidPaxTrades(symbol: string &#124; undefined): SidPaxTrade[]</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-account-ledger-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:152</code> | <code>export const ACCOUNT_LEDGER_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-all-orders-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:117</code> | <code>export const ALL_ORDERS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-all-orders-v2-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:226</code> | <code>export const ALL_ORDERS_V2_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-closed-positions-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:32</code> | <code>export const CLOSED_POSITIONS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-dashboard-v2-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:204</code> | <code>export const DASHBOARD_V2_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-delegates-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:168</code> | <code>export const DELEGATES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-enriched-global-stats-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:319</code> | <code>export const ENRICHED_GLOBAL_STATS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-enriched-market-stats-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:333</code> | <code>export const ENRICHED_MARKET_STATS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-fees-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:246</code> | <code>export const FEES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-filled-orders-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:108</code> | <code>export const FILLED_ORDERS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-funding-payments-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:290</code> | <code>export const FUNDING_PAYMENTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-global-stats-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:83</code> | <code>export const GLOBAL_STATS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-keeper-cycles-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:184</code> | <code>export const KEEPER_CYCLES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-latest-prices-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:134</code> | <code>export const LATEST_PRICES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-leaderboard-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:235</code> | <code>export const LEADERBOARD_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-liquidations-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:126</code> | <code>export const LIQUIDATIONS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-mark-price-history-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:273</code> | <code>export const MARK_PRICE_HISTORY_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-market-info-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:60</code> | <code>export const MARKET_INFO_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-market-snapshots-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:299</code> | <code>export const MARKET_SNAPSHOTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-markets-query</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:73</code> | <code>export const MARKETS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-oi-snapshots-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:265</code> | <code>export const OI_SNAPSHOTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-orderly-funding-rates-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:371</code> | <code>export const ORDERLY_FUNDING_RATES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-orderly-open-interests-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:389</code> | <code>export const ORDERLY_OPEN_INTERESTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-orderly-price-changes-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:380</code> | <code>export const ORDERLY_PRICE_CHANGES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-orderly-tickers-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:361</code> | <code>export const ORDERLY_TICKERS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-orders-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:51</code> | <code>export const ORDERS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-position-ledger-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:160</code> | <code>export const POSITION_LEDGER_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-positions-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:22</code> | <code>export const POSITIONS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-protocol-snapshots-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:308</code> | <code>export const PROTOCOL_SNAPSHOTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-batch-clear-failures-query</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:527</code> | <code>export const SPOT_BATCH_CLEAR_FAILURES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-batch-clearings-query</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:454</code> | <code>export const SPOT_BATCH_CLEARINGS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-collateral-movements-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:481</code> | <code>export const SPOT_COLLATERAL_MOVEMENTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-epoch-settlements-query</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:463</code> | <code>export const SPOT_EPOCH_SETTLEMENTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-fast-settlements-query</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:472</code> | <code>export const SPOT_FAST_SETTLEMENTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-keeper-cycles-query</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:518</code> | <code>export const SPOT_KEEPER_CYCLES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-market-mode-changes-query</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:490</code> | <code>export const SPOT_MARKET_MODE_CHANGES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-market-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:409</code> | <code>export const SPOT_MARKET_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-markets-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:399</code> | <code>export const SPOT_MARKETS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-order-fills-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:445</code> | <code>export const SPOT_ORDER_FILLS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-order-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:430</code> | <code>export const SPOT_ORDER_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-orders-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:419</code> | <code>export const SPOT_ORDERS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-plvs-query</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:508</code> | <code>export const SPOT_PLVS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-pofq-updates-query</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:499</code> | <code>export const SPOT_POFQ_UPDATES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-subgraphquery</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:4</code> | <code>export async function subgraphQuery&lt;T = unknown&gt;( document: string, variables?: Record&lt;string, unknown&gt; ): Promise&lt;T&gt;</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-tpsl-orders-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:193</code> | <code>export const TPSL_ORDERS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-trade-settlements-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:255</code> | <code>export const TRADE_SETTLEMENTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-trades-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:42</code> | <code>export const TRADES_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-trading-account-events-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:176</code> | <code>export const TRADING_ACCOUNT_EVENTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-user-stats-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:91</code> | <code>export const USER_STATS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-user-vault-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:100</code> | <code>export const USER_VAULT_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-vault-balance-history-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:281</code> | <code>export const VAULT_BALANCE_HISTORY_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-vault-events-query</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/perps/useSubgraph.ts:142</code> | <code>export const VAULT_EVENTS_QUERY = /* GraphQL */ `</code> |
| <code>interface.3.ts-perps-hooks-presence-usepresence-presencesnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/presence/usePresence.ts:10</code> | <code>export interface PresenceSnapshot</code> |
| <code>interface.3.ts-perps-hooks-presence-usepresence-usepresenceheartbeat</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/presence/usePresence.ts:47</code> | <code>export function usePresenceHeartbeat(market: string = "home")</code> |
| <code>interface.3.ts-perps-hooks-presence-usepresence-usepresencesnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/presence/usePresence.ts:94</code> | <code>export function usePresenceSnapshot(intervalMs: number = SNAPSHOT_INTERVAL_MS)</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotapprovals-approveexact</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotApprovals.ts:62</code> | <code>export async function approveExact(params: { token: Address; spender: Address; amount: bigint; }): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotapprovals-ensureallowance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotApprovals.ts:96</code> | <code>export async function ensureAllowance(params: { token: ApprovalToken; owner: Address; spender: Address; amount: bigint; native?: NativeTokens; }): Promise&lt;EnsureAllowanceResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotapprovals-ensureallowanceresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotApprovals.ts:79</code> | <code>export interface EnsureAllowanceResult</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotapprovals-getallowance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotApprovals.ts:40</code> | <code>export async function getAllowance(params: { token: Address; owner: Address; spender: Address; }): Promise&lt;bigint&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotapprovals-spot-native-tokens</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotApprovals.ts:31</code> | <code>export const SPOT_NATIVE_TOKENS: NativeTokens = new Set&lt;Address&gt;(</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotapprovals-usespotapprovals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotApprovals.ts:147</code> | <code>export function useSpotApprovals()</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-buildsymbolwsurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:33</code> | <code>export function buildSymbolWsUrl(symbol: string): string</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-hasspotmarketstream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:38</code> | <code>export function hasSpotMarketStream(symbol: string &#124; undefined &#124; null): boolean</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-spotorderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:46</code> | <code>export interface SpotOrderbook</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-spotstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:65</code> | <code>export interface SpotStats</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-spottrade</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:52</code> | <code>export interface SpotTrade</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-subscribespotorderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:278</code> | <code>export function subscribeSpotOrderbook(symbol: string, cb: (book: SpotOrderbook &#124; null) =&gt; void): () =&gt; void</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-usespotorderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:292</code> | <code>export function useSpotOrderbook(symbol: string &#124; undefined): SpotOrderbook &#124; null</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-usespotstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:338</code> | <code>export function useSpotStats(symbol: string &#124; undefined): SpotStats &#124; null</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotmarketstream-usespottrades</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotMarketStream.ts:313</code> | <code>export function useSpotTrades(symbol: string &#124; undefined): SpotTrade[]</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-cancellimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:354</code> | <code>export async function cancelLimitOrder(orderId: bigint): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-cancelstoplimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:473</code> | <code>export async function cancelStopLimitOrder(orderId: bigint): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-cancelstoplossorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:519</code> | <code>export async function cancelStopLossOrder(orderId: bigint): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getlimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:373</code> | <code>export async function getLimitOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getpaxpointer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:122</code> | <code>export async function getPaxPointer(): Promise&lt;Address&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getstoplimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:535</code> | <code>export async function getStopLimitOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getstoplossorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:539</code> | <code>export async function getStopLossOrder(orderId: bigint)</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getuserlimitorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:368</code> | <code>export async function getUserLimitOrders(user: Address): Promise&lt;readonly bigint[]&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getuserstoplimitorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:525</code> | <code>export async function getUserStopLimitOrders(user: Address): Promise&lt;readonly bigint[]&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getuserstoplossorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:530</code> | <code>export async function getUserStopLossOrders(user: Address): Promise&lt;readonly bigint[]&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-limitorderresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:94</code> | <code>export interface LimitOrderResult extends SpotOrderResult</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-marketbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:150</code> | <code>export async function marketBuy(params: MarketBuyParams): Promise&lt;MarketOrderResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-marketbuyparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:134</code> | <code>export interface MarketBuyParams</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-marketorderresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:87</code> | <code>export interface MarketOrderResult extends SpotOrderResult</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-marketsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:218</code> | <code>export async function marketSell(params: MarketSellParams): Promise&lt;MarketOrderResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-marketsellparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:195</code> | <code>export interface MarketSellParams</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-modifylimitorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:358</code> | <code>export async function modifyLimitOrder(params: { orderId: bigint; newTargetPrice: bigint; newExpiresAt: bigint; }): Promise&lt;WaitForTxResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placelimitbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:289</code> | <code>export async function placeLimitBuy( params: PlaceLimitBuyParams, ): Promise&lt;LimitOrderResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placelimitbuyparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:277</code> | <code>export interface PlaceLimitBuyParams</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placelimitsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:326</code> | <code>export async function placeLimitSell( params: PlaceLimitSellParams, ): Promise&lt;LimitOrderResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placelimitsellparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:317</code> | <code>export interface PlaceLimitSellParams</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoplimitbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:393</code> | <code>export async function placeStopLimitBuy( params: PlaceStopLimitBuyParams, ): Promise&lt;StopLimitOrderResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoplimitbuyparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:381</code> | <code>export interface PlaceStopLimitBuyParams</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoplimitsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:439</code> | <code>export async function placeStopLimitSell( params: PlaceStopLimitSellParams, ): Promise&lt;StopLimitOrderResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoplimitsellparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:428</code> | <code>export interface PlaceStopLimitSellParams</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoploss</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:491</code> | <code>export async function placeStopLoss( params: PlaceStopLossParams, ): Promise&lt;StopLossOrderResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoplossparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:481</code> | <code>export interface PlaceStopLossParams</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-spotorderresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:80</code> | <code>export interface SpotOrderResult</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-stoplimitorderresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:99</code> | <code>export type StopLimitOrderResult = LimitOrderResult;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-stoplossorderresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:100</code> | <code>export type StopLossOrderResult = LimitOrderResult;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-usespotorders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotOrders.ts:552</code> | <code>export function useSpotOrders()</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotportfolio-spotportfoliostate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotPortfolio.ts:47</code> | <code>export interface SpotPortfolioState</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotportfolio-usespotportfolio</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotPortfolio.ts:122</code> | <code>export function useSpotPortfolio(address: string &#124; null &#124; undefined): SpotPortfolioState</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-batchquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:199</code> | <code>export async function batchQuote( requests: ReadonlyArray&lt;BatchQuoteRequest&gt;, ): Promise&lt;SpotQuoteResult[]&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-batchquoterequest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:191</code> | <code>export interface BatchQuoteRequest</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-getallliquidityinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:234</code> | <code>export async function getAllLiquidityInfo(): Promise&lt;SpotAllLiquidityRow[]&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-getliquidityinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:221</code> | <code>export async function getLiquidityInfo(token: Address): Promise&lt;SpotLiquidityInfo&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-gettokenprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:211</code> | <code>export async function getTokenPrice(token: Address): Promise&lt;SpotTokenPrice&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-gettwap</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:256</code> | <code>export async function getTWAP(token: Address, period: bigint): Promise&lt;bigint&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-quoteexactin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:168</code> | <code>export async function quoteExactIn(params: { tokenIn: Address; tokenOut: Address; amountIn: bigint; }): Promise&lt;SpotQuoteResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-quoteexactout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:179</code> | <code>export async function quoteExactOut(params: { tokenIn: Address; tokenOut: Address; amountOut: bigint; }): Promise&lt;SpotQuoteResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-quotemarketbuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:141</code> | <code>export async function quoteMarketBuy(params: { stablecoin: Address; token: Address; stablecoinAmount: bigint; }): Promise&lt;SpotQuoteResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-quotemarketsell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:155</code> | <code>export async function quoteMarketSell(params: { token: Address; stablecoin: Address; tokenAmount: bigint; }): Promise&lt;SpotQuoteResult&gt;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-spotallliquidityrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:88</code> | <code>export interface SpotAllLiquidityRow</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-spotliquidityinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:78</code> | <code>export interface SpotLiquidityInfo</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-spotquoteresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:41</code> | <code>export interface SpotQuoteResult</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-spottokenprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:68</code> | <code>export interface SpotTokenPrice</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-usespotquoter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotQuoter.ts:267</code> | <code>export function useSpotQuoter()</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotuserstream-spot-user-ws-url</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotUserStream.ts:33</code> | <code>export const SPOT_USER_WS_URL = gatewayUrls().spotUserWsUrl;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotuserstream-spotuserconnectionstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotUserStream.ts:46</code> | <code>export type SpotUserConnectionState = "idle" &#124; "connecting" &#124; "open" &#124; "closed";</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotuserstream-spotuserframe</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotUserStream.ts:49</code> | <code>export type SpotUserFrame = Record&lt;string, unknown&gt;;</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotuserstream-spotuserstreamname</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotUserStream.ts:39</code> | <code>export type SpotUserStreamName =</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotuserstream-subscribespotuserstream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotUserStream.ts:257</code> | <code>export function subscribeSpotUserStream( stream: SpotUserStreamName, address: string, handler: FrameHandler, ): () =&gt; void</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotuserstream-usespotuserconnection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotUserStream.ts:320</code> | <code>export function useSpotUserConnection(): SpotUserConnectionState</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotuserstream-usespotuserstream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotUserStream.ts:343</code> | <code>export function useSpotUserStream( stream: SpotUserStreamName, address: string &#124; null &#124; undefined, ): SpotUserFrame &#124; null</code> |
| <code>interface.3.ts-perps-hooks-spot-usespotuserstream-usespotuserstreams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/spot/useSpotUserStream.ts:365</code> | <code>export function useSpotUserStreams( streams: SpotUserStreamName[], address: string &#124; null &#124; undefined, ): Partial&lt;Record&lt;SpotUserStreamName, SpotUserFrame&gt;&gt;</code> |
| <code>interface.3.ts-perps-hooks-ui-usecookieconsent-cookieconsent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useCookieConsent.ts:10</code> | <code>export interface CookieConsent</code> |
| <code>interface.3.ts-perps-hooks-ui-usecookieconsent-cookieconsentstore</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useCookieConsent.ts:28</code> | <code>export const cookieConsentStore = create&lt;CookieConsentStore&gt;()((set, get) =&gt; (</code> |
| <code>interface.3.ts-perps-hooks-ui-usecookieconsent-cookieconsentvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useCookieConsent.ts:8</code> | <code>export type CookieConsentValue = "accepted" &#124; "declined" &#124; null;</code> |
| <code>interface.3.ts-perps-hooks-ui-usecookieconsent-usecookieconsent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useCookieConsent.ts:54</code> | <code>export function useCookieConsent(): CookieConsent</code> |
| <code>interface.3.ts-perps-hooks-ui-usefavoritemarkets-favoritemarketsapi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useFavoriteMarkets.ts:60</code> | <code>export interface FavoriteMarketsApi</code> |
| <code>interface.3.ts-perps-hooks-ui-usefavoritemarkets-usefavoritemarkets</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useFavoriteMarkets.ts:73</code> | <code>export function useFavoriteMarkets(): FavoriteMarketsApi</code> |
| <code>interface.3.ts-perps-hooks-ui-uselayoutpresets-layoutpreset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useLayoutPresets.ts:3</code> | <code>export interface LayoutPreset</code> |
| <code>interface.3.ts-perps-hooks-ui-uselayoutpresets-uselayoutpresets</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useLayoutPresets.ts:63</code> | <code>export function useLayoutPresets()</code> |
| <code>interface.3.ts-perps-hooks-ui-usemobile-useismobile</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useMobile.tsx:19</code> | <code>export function useIsMobile(): boolean</code> |
| <code>interface.3.ts-perps-hooks-ui-usesidebar-usesidebar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/ui/useSidebar.ts:15</code> | <code>export const useSidebar = create&lt;SidebarStore&gt;()(</code> |
| <code>interface.3.ts-perps-hooks-useerrorhandler-errorhandlerapi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/useErrorHandler.ts:33</code> | <code>export interface ErrorHandlerApi</code> |
| <code>interface.3.ts-perps-hooks-useerrorhandler-useerrorhandler</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/useErrorHandler.ts:43</code> | <code>export function useErrorHandler(defaults: UseErrorHandlerOptions = {}): ErrorHandlerApi</code> |
| <code>interface.3.ts-perps-hooks-useerrorhandler-useerrorhandleroptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/useErrorHandler.ts:28</code> | <code>export interface UseErrorHandlerOptions</code> |
| <code>interface.3.ts-perps-hooks-usefeetier-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/hooks/perps/useFeeTier.ts</code> (planned) | <code>export function useFeeTier(account: Address &#124; undefined): FeeTierView &#124; undefined</code> |
| <code>interface.3.ts-perps-hooks-usemarketconfig-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/hooks/perps/useMarketConfig.ts</code> (planned) | <code>export function useMarketConfig(marketId: bigint &#124; undefined): { config?: MarketConfig; tiers: MarginTier[]; error?: Error }</code> |
| <code>interface.3.ts-perps-hooks-useperpsmarkets-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/hooks/perps/usePerpsMarkets.ts</code> (planned) | <code>export function usePerpsMarkets(): { markets: MarketCatalogEntry[]; loading: boolean; error?: Error }</code> |
| <code>interface.3.ts-perps-hooks-usespottokenstate-j27</code> | <code>ts_export</code> | <code>new</code> | <code>apps/perps/src/hooks/spot/useSpotTokenState.ts</code> (planned) | <code>export function useSpotTokenState(token: Address &#124; undefined): SpotTokenState &#124; undefined</code> |
| <code>interface.3.ts-perps-hooks-wallet-usewalletstore-usewalletstore</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/hooks/wallet/useWalletStore.ts:33</code> | <code>export const useWalletStore = create&lt;WalletState&gt;()(</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-hooks-chain-useevmblockstream-evmblockstreamstate</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-chain-useevmblockstream-useevmblockstreamoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-paxscan-useblockscoutinfinitequery-paxscanpageparam</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-paxscan-useblockscoutinfinitequery-useblockscoutinfinitequeryoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-paxscan-useblockscoutquery-useblockscoutqueryoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useautocollateral-autocollateralresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useautocollateral-useautocollateral</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useorderlyfundingrates</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useorderlyopeninterests</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useorderlypricechanges</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useenrichedstats-useorderlytickers</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usegraphqlquery-sharedpolledqueryoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usegraphqlquery-sharedpolledqueryresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerliquidation</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerposition</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerpriceupdate</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexerstatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-indexertrade</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-useindexerstatus</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-uselatestpricessubscription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-useliquidationsubscription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-useorderchangedsubscription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-usepositionchangedsubscription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-usesinglemarketpricesubscription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useindexersubscriptions-usetradecreatedsubscription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbauth-useperfdbauthreturn</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-competitiondetail</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-competitionmetric</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbcompetitions-competitionranking</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbreferrals-referraltier</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbreferrals-userreferraldata</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbsubscriptions-priceupdate</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbsubscriptions-tradeupdate</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbsubscriptions-usepricesubscription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbsubscriptions-usetradesubscription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbtraderstats-perfdbtraderstats</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbuser-perfdbuser</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useperfdbuser-useperfdbuser</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-useportfoliostats-portfoliostats</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usequoter-closepositionquote</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usequoter-marketquote</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usequoter-partialclosequote</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usequoter-quotermarketquote</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usequoter-usequoter</code> | overridden_by | override.J27.17 |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-fetchtickers</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-realtimepricesresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickerentry</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickermap</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-userealtimeprices-tickerwire</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-account-ledger-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-all-orders-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-dashboard-v2-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-delegates-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-filled-orders-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-global-stats-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-keeper-cycles-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-latest-prices-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-leaderboard-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-market-info-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-markets-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-markets-query</code> | overridden_by | override.J27.20 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-position-ledger-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-batch-clear-failures-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-batch-clear-failures-query</code> | overridden_by | override.J27.34 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-batch-clearings-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-batch-clearings-query</code> | overridden_by | override.J27.27 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-collateral-movements-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-epoch-settlements-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-epoch-settlements-query</code> | overridden_by | override.J27.28 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-fast-settlements-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-fast-settlements-query</code> | overridden_by | override.J27.29 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-keeper-cycles-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-keeper-cycles-query</code> | overridden_by | override.J27.33 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-market-mode-changes-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-market-mode-changes-query</code> | overridden_by | override.J27.30 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-market-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-markets-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-order-fills-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-order-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-orders-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-plvs-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-plvs-query</code> | overridden_by | override.J27.32 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-pofq-updates-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-spot-pofq-updates-query</code> | overridden_by | override.J27.31 |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-trading-account-events-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-user-stats-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-user-vault-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-perps-usesubgraph-vault-events-query</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-presence-usepresence-presencesnapshot</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-presence-usepresence-usepresenceheartbeat</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-presence-usepresence-usepresencesnapshot</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-cancellimitorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-cancelstoplimitorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-cancelstoplossorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getlimitorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getstoplimitorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getstoplossorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getuserlimitorders</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getuserstoplimitorders</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-getuserstoplossorders</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-limitorderresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-marketbuyparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-marketorderresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-marketsellparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-modifylimitorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placelimitbuyparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placelimitsellparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoplimitbuyparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoplimitsellparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoploss</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-placestoplossparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-spotorderresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-stoplimitorderresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-stoplossorderresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotorders-usespotorders</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotportfolio-spotportfoliostate</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-batchquote</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-batchquoterequest</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-getliquidityinfo</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-gettokenprice</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-gettwap</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-quoteexactin</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-quoteexactout</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-spotallliquidityrow</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-spotliquidityinfo</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-spottokenprice</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-spot-usespotquoter-usespotquoter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-ui-usecookieconsent-cookieconsent</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-ui-usecookieconsent-cookieconsentvalue</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-ui-usefavoritemarkets-favoritemarketsapi</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-ui-uselayoutpresets-layoutpreset</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-useerrorhandler-errorhandlerapi</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-useerrorhandler-useerrorhandler</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-hooks-useerrorhandler-useerrorhandleroptions</code> | notes | no in-repo importer at HEAD |

Contract context <code>contract_context.3.perps-app-markets-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | The perps app market catalog, order limits, quote, fee tier and spot token state read from gateway GraphQL and 0x1015 views |
| storage | No app storage; the catalog is held in memory per session |
| errors | VALIDATION_PRECISION, VALIDATION_ABOVE_MAX, VALIDATION_BELOW_MIN and TRADE_MARKET_HALTED of apps/perps/src/core/errors/types.ts |
| events | None emitted; reads only |
| gas | View calls only; writes keep the sponsored send path |
| activation | Clients of the v6.12.0 surfaces |
| invariants | An order the app submits passes the chain tick, lot and leverage checks it read; a paused spot token cannot be submitted |
| pause | A disabled market or paused token blocks submit in the app |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.518.1</code> | req | 3518.1 |
| <code>acceptance.3.518.1</code> | scenario | Given a local gateway with LOCAL_NODE_RPC, LOCAL_NODE_KEY and NEXT_PUBLIC_PERPS_GATEWAY_URL naming a local node and gateway, when the usePerpsMarkets loader runs, then every catalog entry has szDecimals and tickSize and the count equals the gateway markets count |
| <code>acceptance.3.518.1</code> | test | apps/perps/tests/perps-markets-gateway.test.ts::perpsMarketsLoadFromGateway (planned) |
| <code>acceptance.3.518.1</code> | qualification | UNRUN |
| <code>req.3518</code> | ac_1 | The loader of usePerpsMarkets posting MARKETS_QUERY (apps/perps/src/hooks/perps/useSubgraph.ts:73) to the local gateway returns one catalog entry per gateway market with szDecimals and tickSize set. |
| <code>acceptance.3.520.1</code> | req | 3520.1 |
| <code>acceptance.3.520.1</code> | scenario | Given a ticker quote, a config with maxLeverage 20 and a tier with maintenanceBps 250, when tickerQuoteWithConfig runs, then the quote carries 20 and 250 |
| <code>acceptance.3.520.1</code> | test | apps/perps/tests/quoter-chain-config.test.ts::tickerQuoteUsesChainConfig (planned) |
| <code>acceptance.3.520.1</code> | qualification | UNRUN |
| <code>req.3520</code> | ac_1 | tickerQuoteWithConfig returns maxLeverage 20 and maintenanceMarginBps 250 from a config and tier encoded from the ABI, not the 100 and 1000 defaults of apps/perps/src/hooks/perps/useQuoter.ts:126-127. |
| <code>acceptance.3.520.2</code> | req | 3520.2 |
| <code>acceptance.3.520.2</code> | scenario | Given mark details with fresh false and a disabled config, when tickerQuoteWithConfig runs, then priceStale is true and enabled is false |
| <code>acceptance.3.520.2</code> | test | apps/perps/tests/quoter-chain-config.test.ts::tickerQuoteReportsStale (planned) |
| <code>acceptance.3.520.2</code> | qualification | UNRUN |
| <code>req.3520</code> | ac_2 | tickerQuoteWithConfig returns priceStale true for mark details with fresh false and enabled false for a disabled market, where apps/perps/src/hooks/perps/useQuoter.ts:128-129 hardcodes both. |
| <code>acceptance.3.527.1</code> | req | 3527.1 |
| <code>acceptance.3.527.1</code> | scenario | Given the app operations after the retirement, when the golden list and the gateway roots are read, then none of the eight spot roots appears |
| <code>acceptance.3.527.1</code> | test | apps/perps/tests/graphql-operations.test.ts::retiredSpotOperationsAbsent (planned) |
| <code>acceptance.3.527.1</code> | qualification | UNRUN |
| <code>req.3527</code> | ac_1 | No export of apps/perps/src/hooks/perps/useSubgraph.ts and no GOLDEN_OPERATIONS entry names spotBatchClearings, spotEpochSettlements, spotFastSettlements, spotMarketModeChanges, spotPofqUpdates, spotPLVs, spotKeeperCycles or spotBatchClearFailures, and GATEWAY_QUERY_ROOTS (apps/perps/tests/graphql-operations.test.ts:106) lacks them. |
| <code>capability.3.43.3</code> | title | Retire the eight spot subgraph queries and their golden operations |
| <code>capability.3.43.3</code> | reqs | 3527.1 |
| <code>capability.3.48.6</code> | title | Take quoter leverage, margin and market status from the chain |
| <code>capability.3.48.6</code> | reqs | 3520.1<br>3520.2 |
| <code>capability.3.48.7</code> | title | Load the market catalog from gateway GraphQL with chain config fields |
| <code>capability.3.48.7</code> | reqs | 3518.1 |
| <code>resolution.17.605</code> | title | decision.343 binds: Market and explorer streams and candles stay on the external market-data feed |
| <code>resolution.17.605</code> | applies_to | decision.343<br>task.3.48.6 |
| <code>resolution.17.605</code> | source | decision.343 in fragments/J27/decision.kvx; tasks from affects |

## instrumentation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-instrumentation-register</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/instrumentation.ts:2</code> | <code>export async function register(): Promise&lt;void&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-instrumentation-register</code> | notes | Next.js framework hook |

## page-modules

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-page-modules-analytics-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/analytics/index.tsx:12</code> | <code>export default function AnalyticsPage()</code> |
| <code>interface.3.ts-perps-page-modules-block-detail-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/block-detail/index.tsx:9</code> | <code>export default function BlockRedirectShell()</code> |
| <code>interface.3.ts-perps-page-modules-block-detail-index-getserversideprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/block-detail/index.tsx:13</code> | <code>export const getServerSideProps: GetServerSideProps = async (context) =&gt;</code> |
| <code>interface.3.ts-perps-page-modules-competitions-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/competitions/index.tsx:347</code> | <code>export default function CompetitionsPage()</code> |
| <code>interface.3.ts-perps-page-modules-explorer-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/explorer/index.tsx:219</code> | <code>export default function ExplorerPage()</code> |
| <code>interface.3.ts-perps-page-modules-home-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/home/index.tsx:11</code> | <code>export default function HomePage()</code> |
| <code>interface.3.ts-perps-page-modules-leaderboard-index-comparewei</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/leaderboard/index.tsx:47</code> | <code>export function compareWei(a: string, b: string): number</code> |
| <code>interface.3.ts-perps-page-modules-leaderboard-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/leaderboard/index.tsx:86</code> | <code>export default function LeaderboardPage()</code> |
| <code>interface.3.ts-perps-page-modules-leaderboard-index-normalizeweistr</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/leaderboard/index.tsx:37</code> | <code>export function normalizeWeiStr(v: string &#124; null &#124; undefined): string</code> |
| <code>interface.3.ts-perps-page-modules-leaderboard-index-totalpnlwei</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/leaderboard/index.tsx:54</code> | <code>export function totalPnlWei(entries: readonly { totalRealizedPnl: string }[]): bigint</code> |
| <code>interface.3.ts-perps-page-modules-legal-cookiepolicy-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/legal/CookiePolicy.tsx:35</code> | <code>export default function CookiePolicy()</code> |
| <code>interface.3.ts-perps-page-modules-legal-legallayout-legallayout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/legal/LegalLayout.tsx:15</code> | <code>export function LegalLayout({ title, lastUpdated, children }: LegalLayoutProps)</code> |
| <code>interface.3.ts-perps-page-modules-legal-privacypolicy-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/legal/PrivacyPolicy.tsx:3</code> | <code>export default function PrivacyPolicy()</code> |
| <code>interface.3.ts-perps-page-modules-legal-termsofservice-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/legal/TermsOfService.tsx:3</code> | <code>export default function TermsOfService()</code> |
| <code>interface.3.ts-perps-page-modules-markets-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/markets/index.tsx:38</code> | <code>export default function MarketsPage()</code> |
| <code>interface.3.ts-perps-page-modules-perps-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/perps/index.tsx:93</code> | <code>export default function TradingInterface()</code> |
| <code>interface.3.ts-perps-page-modules-portfolio-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/portfolio/index.tsx:70</code> | <code>export default function PortfolioPage()</code> |
| <code>interface.3.ts-perps-page-modules-portfolio-index-overviewdailypnlprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/portfolio/index.tsx:50</code> | <code>export function overviewDailyPnlProps(result: ReturnType&lt;typeof useDailyPnl&gt;)</code> |
| <code>interface.3.ts-perps-page-modules-referrals-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/referrals/index.tsx:19</code> | <code>export default function ReferralsPage()</code> |
| <code>interface.3.ts-perps-page-modules-spot-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/spot/index.tsx:81</code> | <code>export default function SpotTradingInterface()</code> |
| <code>interface.3.ts-perps-page-modules-tx-detail-index-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/tx-detail/index.tsx:8</code> | <code>export default function TxRedirectShell()</code> |
| <code>interface.3.ts-perps-page-modules-tx-detail-index-getserversideprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/page-modules/tx-detail/index.tsx:14</code> | <code>export const getServerSideProps: GetServerSideProps = async (context) =&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-page-modules-block-detail-index-getserversideprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-page-modules-spot-index-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-page-modules-tx-detail-index-getserversideprops</code> | notes | no in-repo importer at HEAD |

## pages

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-pages-api-candle-path-parsemarketproxyorigin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/pages/api/candle/[...path].ts:71</code> | <code>export function parseMarketProxyOrigin(raw: string &#124; undefined): string</code> |
| <code>interface.3.ts-perps-pages-app-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/pages/_app.tsx:112</code> | <code>export default MyApp;</code> |
| <code>interface.3.ts-perps-pages-document-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/pages/_document.tsx:3</code> | <code>export default function Document()</code> |
| <code>interface.3.ts-perps-pages-spot-getserversideprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/pages/spot.tsx:34</code> | <code>export const getServerSideProps = async (</code> |
| <code>interface.3.ts-perps-pages-spot-spotpage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/pages/spot.tsx:44</code> | <code>export function SpotPage({ symbol }: SpotPageProps)</code> |
| <code>interface.3.ts-perps-pages-spot-spotpageprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/pages/spot.tsx:20</code> | <code>export interface SpotPageProps</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-pages-api-candle-path-parsemarketproxyorigin</code> | notes | Next.js framework entry (api) for /api/candle/{...path} |
| <code>interface.3.ts-perps-pages-app-default</code> | notes | Next.js framework entry (special) for /_app |
| <code>interface.3.ts-perps-pages-document-default</code> | notes | Next.js framework entry (special) for /_document |
| <code>interface.3.ts-perps-pages-spot-getserversideprops</code> | notes | Next.js framework entry (page) for /spot |
| <code>interface.3.ts-perps-pages-spot-spotpage</code> | notes | Next.js framework entry (page) for /spot |
| <code>interface.3.ts-perps-pages-spot-spotpageprops</code> | notes | Next.js framework entry (page) for /spot |

## routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.http-perps-any-api-build-id</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/api/build-id.ts:7</code> | <code>ANY /api/build-id</code> |
| <code>interface.3.http-perps-get</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/index.tsx:1</code> | <code>GET / (page)</code> |
| <code>interface.3.http-perps-get-analytics</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/analytics.tsx:1</code> | <code>GET /analytics (page)</code> |
| <code>interface.3.http-perps-get-auth-callback</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/auth/callback.tsx:18</code> | <code>GET /auth/callback (page)</code> |
| <code>interface.3.http-perps-get-block-height</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/block/[height].tsx:1</code> | <code>GET /block/{height} (page)</code> |
| <code>interface.3.http-perps-get-competitions</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/competitions.tsx:1</code> | <code>GET /competitions (page)</code> |
| <code>interface.3.http-perps-get-cookies</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/cookies.tsx:1</code> | <code>GET /cookies (page)</code> |
| <code>interface.3.http-perps-get-explorer</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/explorer.tsx:1</code> | <code>GET /explorer (page)</code> |
| <code>interface.3.http-perps-get-head-api-candle-path</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/api/candle/[...path].ts:142</code> | <code>GET&#124;HEAD /api/candle/{...path}</code> |
| <code>interface.3.http-perps-get-leaderboard</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/leaderboard.tsx:1</code> | <code>GET /leaderboard (page)</code> |
| <code>interface.3.http-perps-get-markets</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/markets.tsx:1</code> | <code>GET /markets (page)</code> |
| <code>interface.3.http-perps-get-perps</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/perps.tsx:1</code> | <code>GET /perps (page)</code> |
| <code>interface.3.http-perps-get-portfolio</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/portfolio.tsx:1</code> | <code>GET /portfolio (page)</code> |
| <code>interface.3.http-perps-get-post-api-presence</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/api/presence.ts:127</code> | <code>GET&#124;POST /api/presence</code> |
| <code>interface.3.http-perps-get-privacy</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/privacy.tsx:1</code> | <code>GET /privacy (page)</code> |
| <code>interface.3.http-perps-get-referrals</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/referrals.tsx:1</code> | <code>GET /referrals (page)</code> |
| <code>interface.3.http-perps-get-spot</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/spot.tsx:68</code> | <code>GET /spot (page)</code> |
| <code>interface.3.http-perps-get-terms</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/terms.tsx:1</code> | <code>GET /terms (page)</code> |
| <code>interface.3.http-perps-get-tx-hash</code> | <code>http_route</code> | <code>stable</code> | <code>apps/perps/src/pages/tx/[hash].tsx:1</code> | <code>GET /tx/{hash} (page)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.http-perps-get</code> | notes | page body re-exported: export { default } from "@/page-modules/home"; site root |
| <code>interface.3.http-perps-get-analytics</code> | notes | page body re-exported: export { default } from "@/page-modules/analytics" |
| <code>interface.3.http-perps-get-auth-callback</code> | notes | no in-scope string reference to this path |
| <code>interface.3.http-perps-get-block-height</code> | notes | page body re-exported: export { default } from "@/page-modules/block-detail"; no in-scope string reference to this path |
| <code>interface.3.http-perps-get-competitions</code> | notes | page body re-exported: export { default } from "@/page-modules/competitions"; no in-scope string reference to this path |
| <code>interface.3.http-perps-get-cookies</code> | notes | page body re-exported: export { default } from "@/page-modules/legal/CookiePolicy" |
| <code>interface.3.http-perps-get-explorer</code> | notes | page body re-exported: export { default } from "@/page-modules/explorer" |
| <code>interface.3.http-perps-get-leaderboard</code> | notes | page body re-exported: export { default } from "@/page-modules/leaderboard" |
| <code>interface.3.http-perps-get-markets</code> | notes | page body re-exported: export { default } from "@/page-modules/markets" |
| <code>interface.3.http-perps-get-perps</code> | notes | page body re-exported: export { default } from "@/page-modules/perps" |
| <code>interface.3.http-perps-get-portfolio</code> | notes | page body re-exported: export { default } from "@/page-modules/portfolio" |
| <code>interface.3.http-perps-get-privacy</code> | notes | page body re-exported: export { default } from "@/page-modules/legal/PrivacyPolicy" |
| <code>interface.3.http-perps-get-referrals</code> | notes | page body re-exported: export { default } from "@/page-modules/referrals" |
| <code>interface.3.http-perps-get-terms</code> | notes | page body re-exported: export { default } from "@/page-modules/legal/TermsOfService" |
| <code>interface.3.http-perps-get-tx-hash</code> | notes | page body re-exported: export { default } from "@/page-modules/tx-detail"; no in-scope string reference to this path |

## types

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-types-paxscan-api-address-address</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:11</code> | <code>export interface Address extends UserTags</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressblocksvalidatedresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:179</code> | <code>export interface AddressBlocksValidatedResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressceloparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:45</code> | <code>export interface AddressCeloParams</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscoinbalancehistorychart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:171</code> | <code>export type AddressCoinBalanceHistoryChart =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscoinbalancehistoryitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:155</code> | <code>export interface AddressCoinBalanceHistoryItem</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscoinbalancehistoryresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:163</code> | <code>export interface AddressCoinBalanceHistoryResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscollection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:79</code> | <code>export type AddressCollection =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscollectionsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:106</code> | <code>export interface AddressCollectionsResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscounters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:58</code> | <code>export interface AddressCounters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressepochrewardsitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:295</code> | <code>export type AddressEpochRewardsItem =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressepochrewardsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:283</code> | <code>export type AddressEpochRewardsResponse =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressfromtofilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:130</code> | <code>export type AddressFromToFilter = typeof AddressFromToFilterValues[number] &#124; undefined;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressfromtofiltervalues</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:128</code> | <code>export const AddressFromToFilterValues = [ 'from', 'to' ] as const;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressinternaltxsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:186</code> | <code>export interface AddressInternalTxsResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:275</code> | <code>export type AddressMudRecord =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecords</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:244</code> | <code>export type AddressMudRecords =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecordsfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:265</code> | <code>export type AddressMudRecordsFilter =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecordsitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:258</code> | <code>export type AddressMudRecordsItem =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecordssorting</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:270</code> | <code>export type AddressMudRecordsSorting =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudtableitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:225</code> | <code>export type AddressMudTableItem =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudtables</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:232</code> | <code>export type AddressMudTables =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudtablesfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:240</code> | <code>export type AddressMudTablesFilter =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressnft</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:73</code> | <code>export type AddressNFT = TokenInstance &amp;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressnftsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:96</code> | <code>export interface AddressNFTsResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressnfttokensfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:151</code> | <code>export type AddressNFTTokensFilter =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstabscounters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:212</code> | <code>export type AddressTabsCounters =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokenbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:65</code> | <code>export interface AddressTokenBalance</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokenbalancesresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:71</code> | <code>export type AddressTokenBalancesResponse = Array&lt;AddressTokenBalance&gt;;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokensbalancessocketmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:114</code> | <code>export interface AddressTokensBalancesSocketMessage</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokensfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:147</code> | <code>export type AddressTokensFilter =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokensresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:85</code> | <code>export interface AddressTokensResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokentransferfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:141</code> | <code>export type AddressTokenTransferFilters =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokentransferresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:136</code> | <code>export interface AddressTokenTransferResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstransactionsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:119</code> | <code>export interface AddressTransactionsResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstxsfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:132</code> | <code>export type AddressTxsFilters =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresswithdrawalsitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:204</code> | <code>export type AddressWithdrawalsItem =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresswithdrawalsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:196</code> | <code>export type AddressWithdrawalsResponse =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressxstarresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:306</code> | <code>export type AddressXStarResponse =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresszilliqaparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/address.ts:41</code> | <code>export interface AddressZilliqaParams</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addresses-addressesitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addresses.ts:3</code> | <code>export type AddressesItem = AddressParam &amp; { transactions_count: string; coin_balance: string &#124; null };</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addresses-addressesmetadatasearchfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addresses.ts:20</code> | <code>export interface AddressesMetadataSearchFilters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addresses-addressesmetadatasearchresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addresses.ts:15</code> | <code>export interface AddressesMetadataSearchResult</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addresses-addressesresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addresses.ts:5</code> | <code>export type AddressesResponse =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-addressmetadatainfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressMetadata.ts:1</code> | <code>export interface AddressMetadataInfo</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-addressmetadatatag</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressMetadata.ts:11</code> | <code>export interface AddressMetadataTag</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-addressmetadatatagapi</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressMetadata.ts:20</code> | <code>export interface AddressMetadataTagApi extends Omit&lt;AddressMetadataTag, 'meta'&gt;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-addressmetadatatagtype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressMetadata.ts:8</code> | <code>export type AddressMetadataTagType = 'name' &#124; 'generic' &#124; 'classifier' &#124; 'information' &#124; 'note' &#124; 'protocol';</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-publictagtype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressMetadata.ts:47</code> | <code>export interface PublicTagType</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-publictagtypesresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressMetadata.ts:53</code> | <code>export interface PublicTagTypesResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-addressfilecoinparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressParams.ts:21</code> | <code>export type AddressFilecoinParams =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-addressimplementation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressParams.ts:4</code> | <code>export interface AddressImplementation</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-addressparam</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressParams.ts:68</code> | <code>export type AddressParam = UserTags &amp; AddressParamBasic;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-addressparambasic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressParams.ts:51</code> | <code>export type AddressParamBasic =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-addresstag</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressParams.ts:10</code> | <code>export interface AddressTag</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-filecoinactortype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressParams.ts:27</code> | <code>export type FilecoinActorType =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-usertags</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressParams.ts:45</code> | <code>export interface UserTags</code> |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-watchlistname</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/addressParams.ts:16</code> | <code>export interface WatchlistName</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-block</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:20</code> | <code>export interface Block</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockbasefeecelo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:13</code> | <code>export interface BlockBaseFeeCelo</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockcountdownresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:145</code> | <code>export interface BlockCountdownResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:126</code> | <code>export interface BlockFilters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockinternaltransactionsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:113</code> | <code>export interface BlockInternalTransactionsResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blocksresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:96</code> | <code>export interface BlocksResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blocktransactionsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:104</code> | <code>export interface BlockTransactionsResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blocktype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:11</code> | <code>export type BlockType = 'block' &#124; 'reorg' &#124; 'uncle';</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockwithdrawalsitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:138</code> | <code>export type BlockWithdrawalsItem =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockwithdrawalsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:130</code> | <code>export type BlockWithdrawalsResponse =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-newblocksocketresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:121</code> | <code>export interface NewBlockSocketResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-zilliqablockdata</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:78</code> | <code>export interface ZilliqaBlockData</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-zilliqanestedquorumcertificate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:92</code> | <code>export interface ZilliqaNestedQuorumCertificate extends ZilliqaQuorumCertificate</code> |
| <code>interface.3.ts-perps-types-paxscan-api-block-zilliqaquorumcertificate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/block.ts:86</code> | <code>export interface ZilliqaQuorumCertificate</code> |
| <code>interface.3.ts-perps-types-paxscan-api-decodedinput-decodedinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/decodedInput.ts:1</code> | <code>export interface DecodedInput</code> |
| <code>interface.3.ts-perps-types-paxscan-api-decodedinput-decodedinputparams</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/decodedInput.ts:7</code> | <code>export interface DecodedInputParams</code> |
| <code>interface.3.ts-perps-types-paxscan-api-ens-ensaddresslookupfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/ens.ts:8</code> | <code>export interface EnsAddressLookupFilters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-ens-ensdomainlookupfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/ens.ts:16</code> | <code>export interface EnsDomainLookupFilters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-ens-ensdomainlookupfiltersoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/ens.ts:27</code> | <code>export type EnsDomainLookupFiltersOptions = Array&lt;'resolved_to' &#124; 'owned_by' &#124; 'with_inactive'&gt;;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-ens-enslookupsorting</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/ens.ts:22</code> | <code>export interface EnsLookupSorting</code> |
| <code>interface.3.ts-perps-types-paxscan-api-fee-fee</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/fee.ts:1</code> | <code>export interface Fee</code> |
| <code>interface.3.ts-perps-types-paxscan-api-indexingstatus-indexingstatus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/indexingStatus.ts:1</code> | <code>export type IndexingStatus =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-internaltransaction-internaltransaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/internalTransaction.ts:5</code> | <code>export type InternalTransaction = (</code> |
| <code>interface.3.ts-perps-types-paxscan-api-internaltransaction-internaltransactionfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/internalTransaction.ts:38</code> | <code>export interface InternalTransactionFilters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-internaltransaction-internaltransactionsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/internalTransaction.ts:27</code> | <code>export interface InternalTransactionsResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-internaltransaction-txinternalstype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/internalTransaction.ts:3</code> | <code>export type TxInternalsType = 'call' &#124; 'delegatecall' &#124; 'staticcall' &#124; 'create' &#124; 'create2' &#124; 'selfdestruct' &#124; 'reward';</code> |
| <code>interface.3.ts-perps-types-paxscan-api-log-log</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/log.ts:4</code> | <code>export interface Log</code> |
| <code>interface.3.ts-perps-types-paxscan-api-log-logsresponseaddress</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/log.ts:23</code> | <code>export interface LogsResponseAddress</code> |
| <code>interface.3.ts-perps-types-paxscan-api-log-logsresponsetx</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/log.ts:14</code> | <code>export interface LogsResponseTx</code> |
| <code>interface.3.ts-perps-types-paxscan-api-rawtrace-rawtrace</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/rawTrace.ts:1</code> | <code>export interface RawTrace</code> |
| <code>interface.3.ts-perps-types-paxscan-api-rawtrace-rawtracesresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/rawTrace.ts:20</code> | <code>export type RawTracesResponse = Array&lt;RawTrace&gt;;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-reward-reward</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/reward.ts:1</code> | <code>export interface Reward</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-search-result-types</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:6</code> | <code>export const SEARCH_RESULT_TYPES =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchredirectresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:173</code> | <code>export interface SearchRedirectResult</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresult</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:154</code> | <code>export interface SearchResult</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultaddressorcontract</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:59</code> | <code>export interface SearchResultAddressOrContract extends SearchResultAddressData</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultblob</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:128</code> | <code>export interface SearchResultBlob</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultblock</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:112</code> | <code>export interface SearchResultBlock</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultcluster</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:89</code> | <code>export interface SearchResultCluster extends SearchResultAddressData</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultdomain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:76</code> | <code>export interface SearchResultDomain extends Omit&lt;SearchResultAddressData, 'address_hash'&gt;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:169</code> | <code>export interface SearchResultFilters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:141</code> | <code>export type SearchResultItem =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultlabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:102</code> | <code>export interface SearchResultLabel</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultmetadatatag</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:70</code> | <code>export interface SearchResultMetadataTag extends SearchResultAddressData</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresulttacoperation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:64</code> | <code>export interface SearchResultTacOperation</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresulttoken</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:23</code> | <code>export interface SearchResultToken</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresulttx</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:121</code> | <code>export interface SearchResultTx</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresulttype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:21</code> | <code>export type SearchResultType = typeof SEARCH_RESULT_TYPES[keyof typeof SEARCH_RESULT_TYPES];</code> |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultuserop</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/search.ts:134</code> | <code>export interface SearchResultUserOp</code> |
| <code>interface.3.ts-perps-types-paxscan-api-stats-gaspriceinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/stats.ts:34</code> | <code>export interface GasPriceInfo</code> |
| <code>interface.3.ts-perps-types-paxscan-api-stats-gasprices</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/stats.ts:28</code> | <code>export type GasPrices =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-stats-homestats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/stats.ts:1</code> | <code>export type HomeStats =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-nfttokentype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:3</code> | <code>export type NFTTokenType = 'ERC-721' &#124; 'ERC-1155' &#124; 'ERC-404';</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-thumbnailsize</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:58</code> | <code>export type ThumbnailSize = '60x60' &#124; '250x250' &#124; '500x500' &#124; 'original';</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokencounters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:30</code> | <code>export interface TokenCounters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:40</code> | <code>export type TokenHolder = TokenHolderERC20ERC721 &#124; TokenHolderERC1155;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholderbase</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:42</code> | <code>export type TokenHolderBase =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholdererc1155</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:49</code> | <code>export type TokenHolderERC1155 = TokenHolderBase &amp;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholdererc20erc721</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:47</code> | <code>export type TokenHolderERC20ERC721 = TokenHolderBase;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholders</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:35</code> | <code>export interface TokenHolders</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholderspagination</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:53</code> | <code>export type TokenHoldersPagination =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:9</code> | <code>export interface TokenInfo</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninstance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:60</code> | <code>export interface TokenInstance</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninstancemetadatasocketmessage</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:75</code> | <code>export interface TokenInstanceMetadataSocketMessage</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninstancetransferscount</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:80</code> | <code>export interface TokenInstanceTransfersCount</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninventoryfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:96</code> | <code>export type TokenInventoryFilters =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninventorypagination</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:89</code> | <code>export type TokenInventoryPagination =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninventoryresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:84</code> | <code>export interface TokenInventoryResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenreputation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:7</code> | <code>export type TokenReputation = 'ok' &#124; 'scam';</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokentype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:5</code> | <code>export type TokenType = string;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenverifiedinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/token.ts:94</code> | <code>export type TokenVerifiedInfo = unknown;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokeninstancetransferpagination</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokens.ts:23</code> | <code>export interface TokenInstanceTransferPagination</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokeninstancetransferresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokens.ts:18</code> | <code>export interface TokenInstanceTransferResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokensbridgedfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokens.ts:16</code> | <code>export type TokensBridgedFilters = { q: string; chain_ids: Array&lt;string&gt; &#124; undefined };</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokensfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokens.ts:14</code> | <code>export type TokensFilters = { q: string; type: Array&lt;TokenType&gt; &#124; undefined };</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokensresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokens.ts:4</code> | <code>export type TokensResponse =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokenssorting</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokens.ts:30</code> | <code>export interface TokensSorting</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokenssortingfield</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokens.ts:35</code> | <code>export type TokensSortingField = TokensSorting['sort'];</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokenssortingvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokens.ts:37</code> | <code>export type TokensSortingValue = `${ TokensSortingField }-${ TokensSorting['order'] }` &#124; 'default';</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-erc1155totalpayload</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:14</code> | <code>export type Erc1155TotalPayload =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-erc20totalpayload</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:4</code> | <code>export type Erc20TotalPayload =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-erc404totalpayload</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:21</code> | <code>export type Erc404TotalPayload =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-erc721totalpayload</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:9</code> | <code>export type Erc721TotalPayload =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-tokentotal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:50</code> | <code>export type TokenTotal = Erc20TotalPayload &#124; Erc721TotalPayload &#124; Erc1155TotalPayload;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-tokentransfer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:31</code> | <code>export type TokenTransfer = (</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-tokentransferfilters</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:75</code> | <code>export interface TokenTransferFilters</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-tokentransferpagination</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:64</code> | <code>export type TokenTransferPagination =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-tokentransferresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/tokenTransfer.ts:70</code> | <code>export interface TokenTransferResponse</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:22</code> | <code>export type Transaction =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionrevertreason</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:15</code> | <code>export type TransactionRevertReason =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:123</code> | <code>export type TransactionsResponse = TransactionsResponseValidated &#124; TransactionsResponsePending;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponsepending</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:135</code> | <code>export interface TransactionsResponsePending</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponsevalidated</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:125</code> | <code>export interface TransactionsResponseValidated</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponsewatchlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:153</code> | <code>export interface TransactionsResponseWatchlist</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponsewithblobs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:144</code> | <code>export interface TransactionsResponseWithBlobs</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionssorting</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:173</code> | <code>export interface TransactionsSorting</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionssortingfield</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:178</code> | <code>export type TransactionsSortingField = TransactionsSorting['sort'];</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionssortingvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:180</code> | <code>export type TransactionsSortingValue = `${ TransactionsSortingField }-${ TransactionsSorting['order'] }` &#124; 'default';</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:116</code> | <code>export interface TransactionsStats</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactiontype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:162</code> | <code>export type TransactionType = 'rootstock_remasc' &#124;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-txauthorization</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:182</code> | <code>export interface TxAuthorization</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-txsresponse</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:171</code> | <code>export type TxsResponse = TransactionsResponseValidated &#124; TransactionsResponsePending &#124; BlockTransactionsResponse;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-wrappedtransactionfields</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/transaction.ts:19</code> | <code>export type WrappedTransactionFields = 'decoded_input' &#124; 'fee' &#124; 'gas_limit' &#124; 'gas_price' &#124; 'hash' &#124; 'max_fee_per_gas' &#124;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txaction-txaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txAction.ts:24</code> | <code>export type TxAction =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txaction-txactiongeneral</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txAction.ts:1</code> | <code>export interface TxActionGeneral</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txaction-txactionnft</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txAction.ts:13</code> | <code>export interface TxActionNft</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechange</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txStateChanges.ts:5</code> | <code>export type TxStateChange = (TxStateChangeCoin &#124; TxStateChangeToken) &amp;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangecoin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txStateChanges.ts:12</code> | <code>export interface TxStateChangeCoin</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechanges</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txStateChanges.ts:51</code> | <code>export type TxStateChanges =</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetoken</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txStateChanges.ts:18</code> | <code>export type TxStateChangeToken = TxStateChangeTokenErc20 &#124; TxStateChangeTokenErc721 &#124; TxStateChangeTokenErc1155;</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetokenerc1155</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txStateChanges.ts:37</code> | <code>export interface TxStateChangeTokenErc1155</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetokenerc20</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txStateChanges.ts:22</code> | <code>export interface TxStateChangeTokenErc20</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetokenerc404</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txStateChanges.ts:44</code> | <code>export interface TxStateChangeTokenErc404</code> |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetokenerc721</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/paxscan/api/txStateChanges.ts:28</code> | <code>export interface TxStateChangeTokenErc721</code> |
| <code>interface.3.ts-perps-types-perps-accountsummary</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:334</code> | <code>export interface AccountSummary</code> |
| <code>interface.3.ts-perps-types-perps-enrichedglobalstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:213</code> | <code>export interface EnrichedGlobalStats</code> |
| <code>interface.3.ts-perps-types-perps-enrichedmarketstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:218</code> | <code>export interface EnrichedMarketStats</code> |
| <code>interface.3.ts-perps-types-perps-fee</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:7</code> | <code>export interface Fee</code> |
| <code>interface.3.ts-perps-types-perps-feetierconfig</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:316</code> | <code>export interface FeeTierConfig</code> |
| <code>interface.3.ts-perps-types-perps-fundingpayment</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:84</code> | <code>export interface FundingPayment</code> |
| <code>interface.3.ts-perps-types-perps-globalstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:127</code> | <code>export interface GlobalStats</code> |
| <code>interface.3.ts-perps-types-perps-marginmode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:322</code> | <code>export enum MarginMode</code> |
| <code>interface.3.ts-perps-types-perps-marketsnapshotrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:98</code> | <code>export interface MarketSnapshotRecord</code> |
| <code>interface.3.ts-perps-types-perps-marketstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:138</code> | <code>export interface MarketStats</code> |
| <code>interface.3.ts-perps-types-perps-markpricerecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:57</code> | <code>export interface MarkPriceRecord</code> |
| <code>interface.3.ts-perps-types-perps-oisnapshot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:44</code> | <code>export interface OiSnapshot</code> |
| <code>interface.3.ts-perps-types-perps-orderlyfundingrate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:201</code> | <code>export interface OrderlyFundingRate</code> |
| <code>interface.3.ts-perps-types-perps-orderlyopeninterest</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:194</code> | <code>export interface OrderlyOpenInterest</code> |
| <code>interface.3.ts-perps-types-perps-orderlypricechange</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:180</code> | <code>export interface OrderlyPriceChange</code> |
| <code>interface.3.ts-perps-types-perps-orderlyticker</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:151</code> | <code>export interface OrderlyTicker</code> |
| <code>interface.3.ts-perps-types-perps-orderlyvolumestats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:171</code> | <code>export interface OrderlyVolumeStats</code> |
| <code>interface.3.ts-perps-types-perps-placespotorderargs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:269</code> | <code>export interface PlaceSpotOrderArgs</code> |
| <code>interface.3.ts-perps-types-perps-plvquote</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:291</code> | <code>export interface PLVQuote</code> |
| <code>interface.3.ts-perps-types-perps-plvquoteargs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:296</code> | <code>export interface PLVQuoteArgs</code> |
| <code>interface.3.ts-perps-types-perps-plvscore</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:284</code> | <code>export interface PLVScore</code> |
| <code>interface.3.ts-perps-types-perps-pofqscore</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:305</code> | <code>export interface PoFQScore</code> |
| <code>interface.3.ts-perps-types-perps-pofqtierindex</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:352</code> | <code>export type PoFQTierIndex = 0 &#124; 1 &#124; 2 &#124; 3;</code> |
| <code>interface.3.ts-perps-types-perps-positionmargin</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:327</code> | <code>export interface PositionMargin</code> |
| <code>interface.3.ts-perps-types-perps-protocolsnapshotrecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:114</code> | <code>export interface ProtocolSnapshotRecord</code> |
| <code>interface.3.ts-perps-types-perps-sgspotbatchclearfailure</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:517</code> | <code>export interface SgSpotBatchClearFailure</code> |
| <code>interface.3.ts-perps-types-perps-sgspotbatchclearing</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:423</code> | <code>export interface SgSpotBatchClearing</code> |
| <code>interface.3.ts-perps-types-perps-sgspotcollateralmovement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:459</code> | <code>export interface SgSpotCollateralMovement</code> |
| <code>interface.3.ts-perps-types-perps-sgspotepochsettlement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:437</code> | <code>export interface SgSpotEpochSettlement</code> |
| <code>interface.3.ts-perps-types-perps-sgspotfastsettlement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:447</code> | <code>export interface SgSpotFastSettlement</code> |
| <code>interface.3.ts-perps-types-perps-sgspotkeepercycle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:506</code> | <code>export interface SgSpotKeeperCycle</code> |
| <code>interface.3.ts-perps-types-perps-sgspotmarket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:371</code> | <code>export interface SgSpotMarket</code> |
| <code>interface.3.ts-perps-types-perps-sgspotmarketmodechange</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:471</code> | <code>export interface SgSpotMarketModeChange</code> |
| <code>interface.3.ts-perps-types-perps-sgspotorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:386</code> | <code>export interface SgSpotOrder</code> |
| <code>interface.3.ts-perps-types-perps-sgspotorderfill</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:409</code> | <code>export interface SgSpotOrderFill</code> |
| <code>interface.3.ts-perps-types-perps-sgspotplv</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:495</code> | <code>export interface SgSpotPLV</code> |
| <code>interface.3.ts-perps-types-perps-sgspotpofqupdate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:483</code> | <code>export interface SgSpotPofqUpdate</code> |
| <code>interface.3.ts-perps-types-perps-spotbooklevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:255</code> | <code>export interface SpotBookLevel</code> |
| <code>interface.3.ts-perps-types-perps-spotcollateralargs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:278</code> | <code>export interface SpotCollateralArgs</code> |
| <code>interface.3.ts-perps-types-perps-spotepochinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:261</code> | <code>export interface SpotEpochInfo</code> |
| <code>interface.3.ts-perps-types-perps-spotmarket</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:227</code> | <code>export interface SpotMarket</code> |
| <code>interface.3.ts-perps-types-perps-spotorder</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:241</code> | <code>export interface SpotOrder</code> |
| <code>interface.3.ts-perps-types-perps-spotordertype</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:360</code> | <code>export enum SpotOrderType</code> |
| <code>interface.3.ts-perps-types-perps-spotside</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:355</code> | <code>export enum SpotSide</code> |
| <code>interface.3.ts-perps-types-perps-topuppositionargs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:340</code> | <code>export interface TopUpPositionArgs</code> |
| <code>interface.3.ts-perps-types-perps-traderfeetier</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:310</code> | <code>export interface TraderFeeTier</code> |
| <code>interface.3.ts-perps-types-perps-tradesettlement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:24</code> | <code>export interface TradeSettlement</code> |
| <code>interface.3.ts-perps-types-perps-transfermarginargs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:345</code> | <code>export interface TransferMarginArgs</code> |
| <code>interface.3.ts-perps-types-perps-vaultbalancerecord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/types/perps.ts:70</code> | <code>export interface VaultBalanceRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-types-paxscan-api-address-addressblocksvalidatedresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressceloparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscoinbalancehistorychart</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscoinbalancehistoryresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscollection</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresscollectionsresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressepochrewardsitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressepochrewardsresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressfromtofilter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressfromtofiltervalues</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecord</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecords</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecordsfilter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecordsitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudrecordssorting</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudtableitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudtables</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressmudtablesfilter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressnft</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressnfttokensfilter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstabscounters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokenbalance</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokenbalancesresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokensfilter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstokentransferfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresstxsfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresswithdrawalsitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresswithdrawalsresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addressxstarresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-address-addresszilliqaparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addresses-addressesitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addresses-addressesmetadatasearchfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addresses-addressesmetadatasearchresult</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addresses-addressesresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-addressmetadatainfo</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-addressmetadatatag</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-addressmetadatatagtype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-publictagtype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressmetadata-publictagtypesresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-addressparambasic</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-addresstag</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-filecoinactortype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-addressparams-watchlistname</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockbasefeecelo</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockcountdownresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockinternaltransactionsresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-blocktype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockwithdrawalsitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-blockwithdrawalsresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-zilliqablockdata</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-zilliqanestedquorumcertificate</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-block-zilliqaquorumcertificate</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-decodedinput-decodedinputparams</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-ens-ensaddresslookupfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-ens-ensdomainlookupfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-ens-ensdomainlookupfiltersoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-ens-enslookupsorting</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-internaltransaction-internaltransactionfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-internaltransaction-txinternalstype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-log-log</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-rawtrace-rawtrace</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-search-result-types</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultaddressorcontract</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultblob</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultblock</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultcluster</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultdomain</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultlabel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultmetadatatag</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresulttacoperation</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresulttoken</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresulttx</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresulttype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-search-searchresultuserop</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-stats-gaspriceinfo</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-stats-gasprices</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-thumbnailsize</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholderbase</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholdererc1155</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholdererc20erc721</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenholderspagination</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninstancetransferscount</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninventoryfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninventorypagination</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokeninventoryresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-token-tokenverifiedinfo</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokeninstancetransferpagination</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokeninstancetransferresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokensbridgedfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokensfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokensresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokenssorting</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokenssortingfield</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokens-tokenssortingvalue</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-erc1155totalpayload</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-erc20totalpayload</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-erc404totalpayload</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-tokentotal</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-tokentransfer-tokentransferfilters</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionrevertreason</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponsepending</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponsevalidated</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponsewatchlist</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsresponsewithblobs</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionssorting</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionssortingfield</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionssortingvalue</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactionsstats</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-transactiontype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-txauthorization</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-txsresponse</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-transaction-wrappedtransactionfields</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txaction-txactiongeneral</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txaction-txactionnft</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechange</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangecoin</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetoken</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetokenerc1155</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetokenerc20</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetokenerc404</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-paxscan-api-txstatechanges-txstatechangetokenerc721</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-feetierconfig</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-globalstats</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-marginmode</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-marketstats</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-orderlyvolumestats</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-placespotorderargs</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-plvquote</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-plvquoteargs</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-plvscore</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-pofqscore</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-pofqtierindex</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotbatchclearfailure</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotbatchclearing</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotcollateralmovement</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotepochsettlement</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotfastsettlement</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotkeepercycle</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotmarket</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotmarketmodechange</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotorderfill</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotplv</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-sgspotpofqupdate</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-spotbooklevel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-spotcollateralargs</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-spotepochinfo</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-spotmarket</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-spotorder</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-spotordertype</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-spotside</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-traderfeetier</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-types-perps-transfermarginargs</code> | notes | no in-repo importer at HEAD |

## ui

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-ui-atoms-alert-alert</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/alert.tsx:22</code> | <code>const Alert = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-alert-alertdescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/alert.tsx:47</code> | <code>const AlertDescription = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-alert-alerttitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/alert.tsx:35</code> | <code>const AlertTitle = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-animated-input-skiper68</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/animated-input.tsx:63</code> | <code>const Skiper68 = () =&gt;</code> |
| <code>interface.3.ts-perps-ui-atoms-badge-badge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/badge.tsx:29</code> | <code>function Badge({ className, variant, ...props }: BadgeProps)</code> |
| <code>interface.3.ts-perps-ui-atoms-badge-badgeprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/badge.tsx:25</code> | <code>export interface BadgeProps</code> |
| <code>interface.3.ts-perps-ui-atoms-badge-badgevariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/badge.tsx:5</code> | <code>const badgeVariants = cva(</code> |
| <code>interface.3.ts-perps-ui-atoms-button-button</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/button.tsx:43</code> | <code>const Button = React.forwardRef&lt;HTMLButtonElement, ButtonProps&gt;(</code> |
| <code>interface.3.ts-perps-ui-atoms-button-buttonprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/button.tsx:37</code> | <code>export interface ButtonProps</code> |
| <code>interface.3.ts-perps-ui-atoms-button-buttonvariants</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/button.tsx:7</code> | <code>const buttonVariants = cva(</code> |
| <code>interface.3.ts-perps-ui-atoms-card-card</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/card.tsx:5</code> | <code>const Card = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-card-cardcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/card.tsx:56</code> | <code>const CardContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-card-carddescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/card.tsx:44</code> | <code>const CardDescription = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-card-cardfooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/card.tsx:64</code> | <code>const CardFooter = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-card-cardheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/card.tsx:20</code> | <code>const CardHeader = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-card-cardtitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/card.tsx:32</code> | <code>const CardTitle = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-checkbox-checkbox</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/checkbox.tsx:10</code> | <code>const Checkbox = React.forwardRef&lt;HTMLInputElement, CheckboxProps&gt;(</code> |
| <code>interface.3.ts-perps-ui-atoms-collapsible-collapsible</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/collapsible.tsx:3</code> | <code>const Collapsible = CollapsiblePrimitive.Root</code> |
| <code>interface.3.ts-perps-ui-atoms-collapsible-collapsiblecontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/collapsible.tsx:7</code> | <code>const CollapsibleContent = CollapsiblePrimitive.CollapsibleContent</code> |
| <code>interface.3.ts-perps-ui-atoms-collapsible-collapsibletrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/collapsible.tsx:5</code> | <code>const CollapsibleTrigger = CollapsiblePrimitive.CollapsibleTrigger</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoai</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:65</code> | <code>export const IcoAi = icon("/assets/ai.svg", "AI");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoalert</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:47</code> | <code>export const IcoAlert = icon("/assets/alert.svg", "Alert");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoalertlarge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:80</code> | <code>export const IcoAlertLarge = icon("/assets/alert_large.svg", "AlertLarge");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoarrowsleftright</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:53</code> | <code>export const IcoArrowsLeftRight = icon("/assets/ArrowsLeftRight.svg", "ArrowsLeftRight");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoavatar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:50</code> | <code>export const IcoAvatar = icon("/assets/avatar.svg", "Avatar");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoavatardark</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:79</code> | <code>export const IcoAvatarDark = icon("/assets/avatar_dark.svg", "AvatarDark");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icobuy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:37</code> | <code>export const IcoBuy = icon("/assets/buy.svg", "Buy");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icocirclechain</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:72</code> | <code>export const IcoCircleChain = icon("/assets/circle_chain.svg", "CircleChain");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoclock</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:29</code> | <code>export const IcoClock = icon("/assets/clock.svg", "Clock");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icocopy</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:27</code> | <code>export const IcoCopy = icon("/assets/copy.svg", "Copy");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodiscord</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:58</code> | <code>export const IcoDiscord = icon("/assets/Discord.svg", "Discord");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodocker</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:68</code> | <code>export const IcoDocker = icon("/assets/docker.svg", "Docker");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodocument</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:52</code> | <code>export const IcoDocument = icon("/assets/Document.svg", "Document");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodownmenu</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:62</code> | <code>export const IcoDownMenu = icon("/assets/downmenu.svg", "DownMenu");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodraggable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:63</code> | <code>export const IcoDraggable = icon("/assets/draggable.svg", "Draggable");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodropdown</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:49</code> | <code>export const IcoDropdown = icon("/assets/dropdown.svg", "Dropdown");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodropdownwhite</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:78</code> | <code>export const IcoDropdownWhite = icon("/assets/dropdown_white.svg", "DropdownWhite");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoexchange</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:36</code> | <code>export const IcoExchange = icon("/assets/exchange.svg", "Exchange");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoexclamation</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:60</code> | <code>export const IcoExclamation = icon("/assets/exclamation.svg", "Exclamation");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoexit</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:61</code> | <code>export const IcoExit = icon("/assets/exit.svg", "Exit");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icofilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:30</code> | <code>export const IcoFilter = icon("/assets/filter.svg", "Filter");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icofire</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:44</code> | <code>export const IcoFire = icon("/assets/fire.svg", "Fire");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoflag</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:43</code> | <code>export const IcoFlag = icon("/assets/flag.svg", "Flag");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icogas</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:55</code> | <code>export const IcoGas = icon("/assets/gas.svg", "Gas");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icograph</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:35</code> | <code>export const IcoGraph = icon("/assets/graph.svg", "Graph");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolayer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:64</code> | <code>export const IcoLayer = icon("/assets/layer.svg", "Layer");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoleave</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:46</code> | <code>export const IcoLeave = icon("/assets/leave.svg", "Leave");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolink</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:40</code> | <code>export const IcoLink = icon("/assets/link.svg", "Link");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolinkblue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:73</code> | <code>export const IcoLinkBlue = icon("/assets/link_blue.svg", "LinkBlue");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolinkcyan</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:75</code> | <code>export const IcoLinkCyan = icon("/assets/link_cyan.svg", "LinkCyan");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolinkwhite</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:74</code> | <code>export const IcoLinkWhite = icon("/assets/link_white.svg", "LinkWhite");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolock</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:34</code> | <code>export const IcoLock = icon("/assets/lock.svg", "Lock");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icomoney</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:38</code> | <code>export const IcoMoney = icon("/assets/money.svg", "Money");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-iconextarrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:54</code> | <code>export const IcoNextArrow = icon("/assets/nextarrow.svg", "NextArrow");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icopaint</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:70</code> | <code>export const IcoPaint = icon("/assets/Paint.svg", "Paint");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icopeople</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:41</code> | <code>export const IcoPeople = icon("/assets/people.svg", "People");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoplus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:33</code> | <code>export const IcoPlus = icon("/assets/plus.svg", "Plus");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icopluswhite</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:77</code> | <code>export const IcoPlusWhite = icon("/assets/plus_white.svg", "PlusWhite");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icopremium</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:66</code> | <code>export const IcoPremium = icon("/assets/premium.svg", "Premium");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoquitwhite</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:76</code> | <code>export const IcoQuitWhite = icon("/assets/quit_white.svg", "QuitWhite");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icorun</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:56</code> | <code>export const IcoRun = icon("/assets/run.svg", "Run");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icosearch</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:24</code> | <code>export const IcoSearch = icon("/assets/search.svg", "Search");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:71</code> | <code>export const IcoSet = icon("/assets/set.svg", "Set");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icosettings</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:26</code> | <code>export const IcoSettings = icon("/assets/setting.svg", "Settings");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoshare</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:31</code> | <code>export const IcoShare = icon("/assets/share.svg", "Share");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoshield</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:51</code> | <code>export const IcoShield = icon("/assets/case.svg", "Shield");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icospeaker</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:69</code> | <code>export const IcoSpeaker = icon("/assets/Speaker.svg", "Speaker");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icostar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:28</code> | <code>export const IcoStar = icon("/assets/star.svg", "Star");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icotarget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:39</code> | <code>export const IcoTarget = icon("/assets/target.svg", "Target");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icotrash</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:32</code> | <code>export const IcoTrash = icon("/assets/trash.svg", "Trash");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icovolume</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:42</code> | <code>export const IcoVolume = icon("/assets/volume.svg", "Volume");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icowallet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:25</code> | <code>export const IcoWallet = icon("/assets/wallet_svgrepo.svg", "Wallet");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icowarn</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:48</code> | <code>export const IcoWarn = icon("/assets/warn.svg", "Warn");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icowidget</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:57</code> | <code>export const IcoWidget = icon("/assets/widget.svg", "Widget");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icowind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:67</code> | <code>export const IcoWind = icon("/assets/wind.svg", "Wind");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoworld</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:45</code> | <code>export const IcoWorld = icon("/assets/world.svg", "World");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icox</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-icon.tsx:59</code> | <code>export const IcoX = icon("/assets/X.svg", "X");</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-leverage-slider-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-leverage-slider.tsx:17</code> | <code>export default function CustomLeverageSlider({ min = 1, max = 100, step = 1, defaultValue = 10, value, onChange, className, }: CustomLeverageSliderProps)</code> |
| <code>interface.3.ts-perps-ui-atoms-custom-slider-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/custom-slider.tsx:18</code> | <code>export default function CustomSlider({ min = 0, max = 100, step = 1, defaultValue = 0, value, onChange, className, }: CustomSliderProps)</code> |
| <code>interface.3.ts-perps-ui-atoms-dialog-dialog</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dialog.tsx:7</code> | <code>const Dialog = DialogPrimitive.Root</code> |
| <code>interface.3.ts-perps-ui-atoms-dialog-dialogcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dialog.tsx:36</code> | <code>const DialogContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dialog-dialogdescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dialog.tsx:107</code> | <code>const DialogDescription = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dialog-dialogfooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dialog.tsx:78</code> | <code>const DialogFooter = (</code> |
| <code>interface.3.ts-perps-ui-atoms-dialog-dialogheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dialog.tsx:64</code> | <code>const DialogHeader = (</code> |
| <code>interface.3.ts-perps-ui-atoms-dialog-dialogtitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dialog.tsx:92</code> | <code>const DialogTitle = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dialog-dialogtrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dialog.tsx:9</code> | <code>const DialogTrigger = DialogPrimitive.Trigger</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenu</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:6</code> | <code>const DropdownMenu = DropdownMenuPrimitive.Root</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenucheckboxitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:93</code> | <code>const DropdownMenuCheckboxItem = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenucontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:56</code> | <code>const DropdownMenuContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenugroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:10</code> | <code>const DropdownMenuGroup = DropdownMenuPrimitive.Group</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:75</code> | <code>const DropdownMenuItem = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenulabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:139</code> | <code>const DropdownMenuLabel = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuportal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:12</code> | <code>const DropdownMenuPortal = DropdownMenuPrimitive.Portal</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuradiogroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:16</code> | <code>const DropdownMenuRadioGroup = DropdownMenuPrimitive.RadioGroup</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuradioitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:117</code> | <code>const DropdownMenuRadioItem = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuseparator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:157</code> | <code>const DropdownMenuSeparator = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenushortcut</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:169</code> | <code>const DropdownMenuShortcut = (</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenusub</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:14</code> | <code>const DropdownMenuSub = DropdownMenuPrimitive.Sub</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenusubcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:40</code> | <code>const DropdownMenuSubContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenusubtrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:18</code> | <code>const DropdownMenuSubTrigger = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenutrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/dropdown-menu.tsx:8</code> | <code>const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger</code> |
| <code>interface.3.ts-perps-ui-atoms-glass-surface-glasssurface</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/glass-surface.tsx:9</code> | <code>const GlassSurface = React.forwardRef&lt;HTMLDivElement, GlassSurfaceProps&gt;(</code> |
| <code>interface.3.ts-perps-ui-atoms-glass-surface-glasssurfaceprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/glass-surface.tsx:4</code> | <code>export interface GlassSurfaceProps extends React.HTMLAttributes&lt;HTMLDivElement&gt;</code> |
| <code>interface.3.ts-perps-ui-atoms-hyperpaxwordmark-hyperpaxwordmark</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/HyperPaxWordmark.tsx:34</code> | <code>export function HyperPaxWordmark({ size = "md", textOnly = false, markOnly = false, color, className, }: HyperPaxWordmarkProps)</code> |
| <code>interface.3.ts-perps-ui-atoms-hyperpaxwordmark-hyperpaxwordmarkprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/HyperPaxWordmark.tsx:15</code> | <code>export interface HyperPaxWordmarkProps</code> |
| <code>interface.3.ts-perps-ui-atoms-input-input</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/input.tsx:10</code> | <code>const Input = React.forwardRef&lt;HTMLInputElement, InputProps&gt;(</code> |
| <code>interface.3.ts-perps-ui-atoms-input-inputprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/input.tsx:5</code> | <code>export interface InputProps</code> |
| <code>interface.3.ts-perps-ui-atoms-label-label</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/label.tsx:11</code> | <code>const Label = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-paxscanlink-paxscanlink</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/PaxscanLink.tsx:45</code> | <code>export function PaxscanLink({ kind, value, label, iconClassName, className, title, }: PaxscanLinkProps): React.ReactElement &#124; null</code> |
| <code>interface.3.ts-perps-ui-atoms-paxscanlink-paxscanlinkkind</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/PaxscanLink.tsx:14</code> | <code>export type PaxscanLinkKind = "tx" &#124; "block" &#124; "address";</code> |
| <code>interface.3.ts-perps-ui-atoms-paxscanlink-paxscanlinkprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/PaxscanLink.tsx:16</code> | <code>export interface PaxscanLinkProps</code> |
| <code>interface.3.ts-perps-ui-atoms-paxspinner-paxspinner</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/PaxSpinner.tsx:59</code> | <code>export function PaxSpinner({ className, style, speed = 0.6, cameraZ = 4, active = true, }: PaxSpinnerProps)</code> |
| <code>interface.3.ts-perps-ui-atoms-popover-popover</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/popover.tsx:6</code> | <code>const Popover = PopoverPrimitive.Root</code> |
| <code>interface.3.ts-perps-ui-atoms-popover-popoveranchor</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/popover.tsx:10</code> | <code>const PopoverAnchor = PopoverPrimitive.Anchor</code> |
| <code>interface.3.ts-perps-ui-atoms-popover-popovercontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/popover.tsx:12</code> | <code>const PopoverContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-popover-popovertrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/popover.tsx:8</code> | <code>const PopoverTrigger = PopoverPrimitive.Trigger</code> |
| <code>interface.3.ts-perps-ui-atoms-priceflash-priceflash</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/PriceFlash.tsx:10</code> | <code>export function PriceFlash({ value, children, className, as: Tag = "span", durationMs = 420, }: { value: number; children: React.ReactNode; className?: string; as?: keyof JSX.IntrinsicElements; durationMs?: number;</code> |
| <code>interface.3.ts-perps-ui-atoms-resize-handle-resizehandle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/resize-handle.tsx:31</code> | <code>export function ResizeHandle({ direction, targetRef, cssProp = direction === "horizontal" ? "width" : "height", min = 0, max = 9999, onResizeEnd, invert = false, className, }: ResizeHandleProps)</code> |
| <code>interface.3.ts-perps-ui-atoms-segmented-control-segmentedcontrol</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/segmented-control.tsx:18</code> | <code>export function SegmentedControl({ options, value, onValueChange, className, size = "md", }: SegmentedControlProps)</code> |
| <code>interface.3.ts-perps-ui-atoms-segmented-control-segmentedcontroloption</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/segmented-control.tsx:5</code> | <code>export interface SegmentedControlOption</code> |
| <code>interface.3.ts-perps-ui-atoms-segmented-control-segmentedcontrolprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/segmented-control.tsx:10</code> | <code>export interface SegmentedControlProps</code> |
| <code>interface.3.ts-perps-ui-atoms-select-select</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:12</code> | <code>const Select = SelectPrimitive.Root</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selectcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:73</code> | <code>const SelectContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selectgroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:14</code> | <code>const SelectGroup = SelectPrimitive.Group</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selectitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:117</code> | <code>const SelectItem = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selectlabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:105</code> | <code>const SelectLabel = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selectscrolldownbutton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:55</code> | <code>const SelectScrollDownButton = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selectscrollupbutton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:38</code> | <code>const SelectScrollUpButton = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selectseparator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:139</code> | <code>const SelectSeparator = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selecttrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:18</code> | <code>const SelectTrigger = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-select-selectvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/select.tsx:16</code> | <code>const SelectValue = SelectPrimitive.Value</code> |
| <code>interface.3.ts-perps-ui-atoms-separator-separator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/separator.tsx:8</code> | <code>const Separator = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheet</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:7</code> | <code>const Sheet = SheetPrimitive.Root</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetclose</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:11</code> | <code>const SheetClose = SheetPrimitive.Close</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:53</code> | <code>const SheetContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetdescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:114</code> | <code>const SheetDescription = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetfooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:88</code> | <code>const SheetFooter = (</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:74</code> | <code>const SheetHeader = (</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetoverlay</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:15</code> | <code>const SheetOverlay = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetportal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:13</code> | <code>const SheetPortal = SheetPrimitive.Portal</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheettitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:102</code> | <code>const SheetTitle = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheettrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sheet.tsx:9</code> | <code>const SheetTrigger = SheetPrimitive.Trigger</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:156</code> | <code>const Sidebar = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:395</code> | <code>const SidebarContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarfooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:365</code> | <code>const SidebarFooter = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebargroup</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:413</code> | <code>const SidebarGroup = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebargroupaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:449</code> | <code>const SidebarGroupAction = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebargroupcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:472</code> | <code>const SidebarGroupContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebargrouplabel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:428</code> | <code>const SidebarGroupLabel = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:350</code> | <code>const SidebarHeader = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarinput</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:332</code> | <code>const SidebarInput = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarinset</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:314</code> | <code>const SidebarInset = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenu</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:485</code> | <code>const SidebarMenu = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenuaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:592</code> | <code>const SidebarMenuAction = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenubadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:623</code> | <code>const SidebarMenuBadge = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenubutton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:533</code> | <code>const SidebarMenuButton = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenuitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:498</code> | <code>const SidebarMenuItem = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenuskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:644</code> | <code>const SidebarMenuSkeleton = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenusub</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:682</code> | <code>const SidebarMenuSub = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenusubbutton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:705</code> | <code>const SidebarMenuSubButton = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenusubitem</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:699</code> | <code>const SidebarMenuSubItem = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:47</code> | <code>const SidebarProvider = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarrail</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:285</code> | <code>const SidebarRail = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarseparator</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:380</code> | <code>const SidebarSeparator = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebartrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:259</code> | <code>const SidebarTrigger = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-sidebar-usesidebar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/sidebar.tsx:38</code> | <code>function useSidebar()</code> |
| <code>interface.3.ts-perps-ui-atoms-skeleton-panel-chartskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/skeleton-panel.tsx:16</code> | <code>export function ChartSkeleton()</code> |
| <code>interface.3.ts-perps-ui-atoms-skeleton-panel-orderbookskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/skeleton-panel.tsx:38</code> | <code>export function OrderbookSkeleton()</code> |
| <code>interface.3.ts-perps-ui-atoms-skeleton-panel-orderformskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/skeleton-panel.tsx:88</code> | <code>export function OrderFormSkeleton()</code> |
| <code>interface.3.ts-perps-ui-atoms-skeleton-panel-positionsskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/skeleton-panel.tsx:64</code> | <code>export function PositionsSkeleton()</code> |
| <code>interface.3.ts-perps-ui-atoms-skeleton-skeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/skeleton.tsx:3</code> | <code>function Skeleton({ className, ...props }: React.HTMLAttributes&lt;HTMLDivElement&gt;)</code> |
| <code>interface.3.ts-perps-ui-atoms-slider-slider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/slider.tsx:6</code> | <code>const Slider = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-squicirclefilter-skiper63</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/SquiCircleFilter.tsx:70</code> | <code>const Skiper63 = () =&gt;</code> |
| <code>interface.3.ts-perps-ui-atoms-squicirclefilter-squicirclefilterstatic</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/SquiCircleFilter.tsx:21</code> | <code>export const SquiCircleFilterStatic = () =&gt;</code> |
| <code>interface.3.ts-perps-ui-atoms-switch-switch</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/switch.tsx:6</code> | <code>const Switch = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-table-table</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/table.tsx:5</code> | <code>const Table = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-table-tablebody</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/table.tsx:27</code> | <code>const TableBody = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-table-tablecaption</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/table.tsx:99</code> | <code>const TableCaption = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-table-tablecell</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/table.tsx:84</code> | <code>const TableCell = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-table-tablefooter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/table.tsx:39</code> | <code>const TableFooter = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-table-tablehead</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/table.tsx:69</code> | <code>const TableHead = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-table-tableheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/table.tsx:19</code> | <code>const TableHeader = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-table-tablerow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/table.tsx:54</code> | <code>const TableRow = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-tabs-tabs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/tabs.tsx:6</code> | <code>const Tabs = TabsPrimitive.Root</code> |
| <code>interface.3.ts-perps-ui-atoms-tabs-tabscontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/tabs.tsx:38</code> | <code>const TabsContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-tabs-tabslist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/tabs.tsx:8</code> | <code>const TabsList = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-tabs-tabstrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/tabs.tsx:23</code> | <code>const TabsTrigger = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-toast-toast</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toast.tsx:44</code> | <code>const Toast = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-toast-toastactionelement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toast.tsx:42</code> | <code>export type ToastActionElement = React.ReactElement</code> |
| <code>interface.3.ts-perps-ui-atoms-toast-toastclose</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toast.tsx:58</code> | <code>const ToastClose = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-toast-toastdescription</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toast.tsx:88</code> | <code>const ToastDescription = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-toast-toastprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toast.tsx:39</code> | <code>export type ToastProps = React.ComponentPropsWithoutRef&lt;typeof ToastPrimitives.Root&gt; &amp;</code> |
| <code>interface.3.ts-perps-ui-atoms-toast-toastprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toast.tsx:7</code> | <code>const ToastProvider = ToastPrimitives.Provider</code> |
| <code>interface.3.ts-perps-ui-atoms-toast-toasttitle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toast.tsx:76</code> | <code>const ToastTitle = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-toast-toastviewport</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toast.tsx:8</code> | <code>const ToastViewport = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-toaster-toaster</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/toaster.tsx:11</code> | <code>export function Toaster()</code> |
| <code>interface.3.ts-perps-ui-atoms-tooltip-tooltip</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/tooltip.tsx:8</code> | <code>const Tooltip = TooltipPrimitive.Root</code> |
| <code>interface.3.ts-perps-ui-atoms-tooltip-tooltipcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/tooltip.tsx:12</code> | <code>const TooltipContent = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-ui-atoms-tooltip-tooltipprovider</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/tooltip.tsx:6</code> | <code>const TooltipProvider = TooltipPrimitive.Provider</code> |
| <code>interface.3.ts-perps-ui-atoms-tooltip-tooltiptrigger</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/tooltip.tsx:10</code> | <code>const TooltipTrigger = TooltipPrimitive.Trigger</code> |
| <code>interface.3.ts-perps-ui-atoms-use-toast-reducer</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/use-toast.tsx:75</code> | <code>export const reducer = (state: State, action: Action): State =&gt;</code> |
| <code>interface.3.ts-perps-ui-atoms-use-toast-toast</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/use-toast.tsx:141</code> | <code>function toast({ ...props }: Toast)</code> |
| <code>interface.3.ts-perps-ui-atoms-use-toast-toastprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/use-toast.tsx:3</code> | <code>type ToastProps =</code> |
| <code>interface.3.ts-perps-ui-atoms-use-toast-usetoast</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/atoms/use-toast.tsx:170</code> | <code>function useToast()</code> |
| <code>interface.3.ts-perps-ui-hooks-useanimatednumber-useanimatednumber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/hooks/useAnimatedNumber.ts:21</code> | <code>export function useAnimatedNumber(target: number, duration = 600): number</code> |
| <code>interface.3.ts-perps-ui-tokens-gasweather-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/GasWeather.tsx:255</code> | <code>export default GasWeather;</code> |
| <code>interface.3.ts-perps-ui-tokens-gasweather-gasweather</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/GasWeather.tsx:170</code> | <code>export function GasWeather({ className, style }: GasWeatherProps)</code> |
| <code>interface.3.ts-perps-ui-tokens-gasweather-gasweatherprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/GasWeather.tsx:145</code> | <code>export interface GasWeatherProps</code> |
| <code>interface.3.ts-perps-ui-tokens-icon-icon</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/Icon.tsx:35</code> | <code>export const Icon: Record&lt;</code> |
| <code>interface.3.ts-perps-ui-tokens-liquidstat-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/LiquidStat.tsx:160</code> | <code>export default LiquidStat;</code> |
| <code>interface.3.ts-perps-ui-tokens-liquidstat-liquidstat</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/LiquidStat.tsx:30</code> | <code>export function LiquidStat({ states, width = 360, className, style }: LiquidStatProps)</code> |
| <code>interface.3.ts-perps-ui-tokens-liquidstat-liquidstatprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/LiquidStat.tsx:16</code> | <code>export interface LiquidStatProps</code> |
| <code>interface.3.ts-perps-ui-tokens-liquidstat-liquidstatstate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/LiquidStat.tsx:8</code> | <code>export interface LiquidStatState</code> |
| <code>interface.3.ts-perps-ui-tokens-overview-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/Overview.tsx:148</code> | <code>export default Overview;</code> |
| <code>interface.3.ts-perps-ui-tokens-overview-overview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/Overview.tsx:68</code> | <code>export function Overview({ totalValue, totalDelta24h, unrealized, unrealizedPct, openCount, openTokens, realized, realizedTradeCount, className, style, }: OverviewProps)</code> |
| <code>interface.3.ts-perps-ui-tokens-overview-overviewprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/Overview.tsx:13</code> | <code>export interface OverviewProps</code> |
| <code>interface.3.ts-perps-ui-tokens-overview-overviewtokendot</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/Overview.tsx:8</code> | <code>export interface OverviewTokenDot</code> |
| <code>interface.3.ts-perps-ui-tokens-performance-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/Performance.tsx:205</code> | <code>export default Performance;</code> |
| <code>interface.3.ts-perps-ui-tokens-performance-performance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/Performance.tsx:96</code> | <code>export function Performance({ sevenDPnl, sevenDPnlVsPrevPct, sevenDVolume, sevenDTrades, sevenDRoiPct, sevenDRoiAnnualizedPct, dailyPnls, dailyLabels, className, style, }: PerformanceProps)</code> |
| <code>interface.3.ts-perps-ui-tokens-performance-performanceprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/Performance.tsx:6</code> | <code>export interface PerformanceProps</code> |
| <code>interface.3.ts-perps-ui-tokens-tokenbadge-tokenbadge</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/TokenBadge.tsx:26</code> | <code>export function TokenBadge({ token, size = 28, className }: TokenBadgeProps)</code> |
| <code>interface.3.ts-perps-ui-tokens-tokenbadge-tokenspec</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/tokens/TokenBadge.tsx:11</code> | <code>export interface TokenSpec</code> |
| <code>interface.3.ts-perps-ui-utils-cn-cn</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/cn.ts:4</code> | <code>export function cn(...inputs: ClassValue[])</code> |
| <code>interface.3.ts-perps-ui-utils-format-fmt</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:162</code> | <code>export function fmt(n: number, digits = 2): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-fmtchangepct</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:63</code> | <code>export function fmtChangePct(v: number): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-fmtcompact</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:177</code> | <code>export function fmtCompact(n: number): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-fmtdate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:70</code> | <code>export function fmtDate(v: string &#124; null &#124; undefined): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-fmtpercent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:55</code> | <code>export function fmtPercent(v: number, digits: number = 4): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-fmtprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:44</code> | <code>export function fmtPrice(price: number): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-fmtusd</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:17</code> | <code>export function fmtUsd(v: number &#124; string &#124; null &#124; undefined): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-fmtusdcompact</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:29</code> | <code>export function fmtUsdCompact(v: number &#124; string &#124; null &#124; undefined): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-parse18</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:143</code> | <code>export function parse18(v: string &#124; null &#124; undefined): number</code> |
| <code>interface.3.ts-perps-ui-utils-format-parsewei</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:112</code> | <code>export function parseWei(v: string): bigint</code> |
| <code>interface.3.ts-perps-ui-utils-format-shortaddr</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:85</code> | <code>export function shortAddr(a: string): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-toplaindecimal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:95</code> | <code>export function toPlainDecimal(v: string): string</code> |
| <code>interface.3.ts-perps-ui-utils-format-weiorzero</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:124</code> | <code>export function weiOrZero(v: string &#124; null &#124; undefined): bigint</code> |
| <code>interface.3.ts-perps-ui-utils-format-weipct</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:134</code> | <code>export function weiPct(a: bigint, b: bigint): number &#124; null</code> |
| <code>interface.3.ts-perps-ui-utils-format-weitonumber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/ui/utils/format.ts:119</code> | <code>export function weiToNumber(wei: bigint): number</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-ui-atoms-alert-alert</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-alert-alertdescription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-alert-alerttitle</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-animated-input-skiper68</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-badge-badge</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-badge-badgeprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-badge-badgevariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-button-buttonprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-button-buttonvariants</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-card-card</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-card-cardcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-card-carddescription</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-card-cardfooter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-card-cardheader</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-card-cardtitle</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-checkbox-checkbox</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-collapsible-collapsible</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-collapsible-collapsiblecontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-collapsible-collapsibletrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoai</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoalertlarge</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoarrowsleftright</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoavatar</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoavatardark</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icobuy</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icocirclechain</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icocopy</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodiscord</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodocker</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodocument</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodownmenu</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodraggable</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodropdown</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icodropdownwhite</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoexchange</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoexit</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icofilter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icofire</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoflag</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icogas</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icograph</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolayer</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoleave</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolink</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolinkblue</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolinkcyan</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolinkwhite</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icolock</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icomoney</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-iconextarrow</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icopaint</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icopeople</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoplus</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icopluswhite</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icopremium</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoquitwhite</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icorun</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoset</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoshield</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icospeaker</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icostar</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icotarget</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icotrash</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icovolume</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icowarn</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icowind</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icoworld</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-icon-icox</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-leverage-slider-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-custom-slider-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dialog-dialogtrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenucheckboxitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenugroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenulabel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuportal</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuradiogroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuradioitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenuseparator</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenushortcut</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenusub</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenusubcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-dropdown-menu-dropdownmenusubtrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-glass-surface-glasssurface</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-glass-surface-glasssurfaceprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-hyperpaxwordmark-hyperpaxwordmarkprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-input-inputprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-paxscanlink-paxscanlink</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-paxscanlink-paxscanlinkkind</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-paxscanlink-paxscanlinkprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-popover-popoveranchor</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-segmented-control-segmentedcontrol</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-segmented-control-segmentedcontroloption</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-segmented-control-segmentedcontrolprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-select</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selectcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selectgroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selectitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selectlabel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selectscrolldownbutton</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selectscrollupbutton</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selectseparator</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selecttrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-select-selectvalue</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetclose</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetfooter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetoverlay</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheetportal</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sheet-sheettrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebar</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarfooter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebargroup</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebargroupaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebargroupcontent</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebargrouplabel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarheader</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarinput</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarinset</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenu</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenuaction</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenubadge</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenubutton</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenuitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenuskeleton</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenusub</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenusubbutton</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarmenusubitem</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarprovider</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarrail</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebarseparator</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-sidebartrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-sidebar-usesidebar</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-squicirclefilter-skiper63</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-table-table</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-table-tablebody</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-table-tablecaption</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-table-tablecell</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-table-tablefooter</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-table-tablehead</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-table-tableheader</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-table-tablerow</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-tabs-tabslist</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-tabs-tabstrigger</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-toast-toastactionelement</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-toast-toastprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-use-toast-reducer</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-atoms-use-toast-toastprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-gasweather-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-gasweather-gasweatherprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-icon-icon</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-liquidstat-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-liquidstat-liquidstatprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-overview-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-overview-overview</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-overview-overviewprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-overview-overviewtokendot</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-performance-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-performance-performanceprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-tokenbadge-tokenbadge</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-tokens-tokenbadge-tokenspec</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-utils-cn-cn</code> | notes | 87 callers, first 69 listed |
| <code>interface.3.ts-perps-ui-utils-format-fmtcompact</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-ui-utils-format-fmtpercent</code> | notes | no in-repo importer at HEAD |

## utils

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-utils-cn-cn</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/utils/cn.ts:4</code> | <code>export function cn(...inputs: ClassValue[])</code> |
| <code>interface.3.ts-perps-utils-format-formatlargenumber</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/utils/format.ts:2</code> | <code>export function formatLargeNumber(value: string): string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-utils-cn-cn</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-utils-format-formatlargenumber</code> | notes | no in-repo importer at HEAD |

## widgets

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.ts-perps-widgets-alerts-alertspanel-alertspanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/alerts/AlertsPanel.tsx:39</code> | <code>export function AlertsPanel({ address }: AlertsPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-analytics-analyticsliquidstat-analyticsliquidstat</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/analytics/AnalyticsLiquidStat.tsx:99</code> | <code>export function AnalyticsLiquidStat()</code> |
| <code>interface.3.ts-perps-widgets-analytics-analyticsliquidstat-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/analytics/AnalyticsLiquidStat.tsx:198</code> | <code>export default AnalyticsLiquidStat;</code> |
| <code>interface.3.ts-perps-widgets-analytics-fundinghistorychart-fundinghistorychart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/analytics/FundingHistoryChart.tsx:42</code> | <code>export function FundingHistoryChart({ marketId, isMobile }: FundingHistoryChartProps)</code> |
| <code>interface.3.ts-perps-widgets-analytics-marketsnapshottable-funding-label-onchain-24h</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/analytics/MarketSnapshotTable.tsx:13</code> | <code>export const FUNDING_LABEL_ONCHAIN_24H = "funding paid (24h, on-chain)";</code> |
| <code>interface.3.ts-perps-widgets-analytics-marketsnapshottable-marketsnapshottable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/analytics/MarketSnapshotTable.tsx:34</code> | <code>export function MarketSnapshotTable({ marketId }: MarketSnapshotTableProps)</code> |
| <code>interface.3.ts-perps-widgets-analytics-markpricechart-markpricechart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/analytics/MarkPriceChart.tsx:36</code> | <code>export function MarkPriceChart({ marketId, isMobile }: MarkPriceChartProps)</code> |
| <code>interface.3.ts-perps-widgets-analytics-oichart-oichart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/analytics/OiChart.tsx:36</code> | <code>export function OiChart({ marketId, isMobile }: OiChartProps)</code> |
| <code>interface.3.ts-perps-widgets-analytics-protocoltvlchart-protocoltvlchart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/analytics/ProtocolTvlChart.tsx:34</code> | <code>export function ProtocolTvlChart({ isMobile }: ProtocolTvlChartProps)</code> |
| <code>interface.3.ts-perps-widgets-errors-errorpanel-errorpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ErrorPanel.tsx:42</code> | <code>export function ErrorPanel({ error, onRetry, size = "md", bare = false, className, }: ErrorPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-errors-errortoast-errorglyph</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ErrorToast.tsx:154</code> | <code>export { AlertCircle as ErrorGlyph };</code> |
| <code>interface.3.ts-perps-widgets-errors-errortoast-showerrortoast</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ErrorToast.tsx:68</code> | <code>export function showErrorToast( ufe: UserFacingError, opts: ShowErrorToastOptions = {}, ):</code> |
| <code>interface.3.ts-perps-widgets-errors-errortoast-showerrortoastoptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ErrorToast.tsx:28</code> | <code>export interface ShowErrorToastOptions</code> |
| <code>interface.3.ts-perps-widgets-errors-errortoast-toasterror</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ErrorToast.tsx:172</code> | <code>export function toastError(err: unknown, opts: ToastErrorOptions = {}): void</code> |
| <code>interface.3.ts-perps-widgets-errors-errortoast-toasterroroptions</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ErrorToast.tsx:158</code> | <code>export interface ToastErrorOptions extends ShowErrorToastOptions</code> |
| <code>interface.3.ts-perps-widgets-errors-reportfeedbackmodal-closereportmodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ReportFeedbackModal.tsx:43</code> | <code>export function closeReportModal(): void</code> |
| <code>interface.3.ts-perps-widgets-errors-reportfeedbackmodal-openreportmodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ReportFeedbackModal.tsx:39</code> | <code>export function openReportModal(error: UserFacingError): void</code> |
| <code>interface.3.ts-perps-widgets-errors-reportfeedbackmodal-reportfeedbackmodalhost</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ReportFeedbackModal.tsx:51</code> | <code>export function ReportFeedbackModalHost()</code> |
| <code>interface.3.ts-perps-widgets-errors-toastactionbridge-toastaction</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ToastActionBridge.tsx:26</code> | <code>export const ToastAction = React.forwardRef&lt;</code> |
| <code>interface.3.ts-perps-widgets-errors-toastactionbridge-toastactionelement</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/errors/ToastActionBridge.tsx:18</code> | <code>export type ToastActionElement = React.ReactElement;</code> |
| <code>interface.3.ts-perps-widgets-explorer-blockdetailpanel-blockdetailpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/BlockDetailPanel.tsx:51</code> | <code>export function BlockDetailPanel({ identifier, seedBlock, onClose }: BlockDetailPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-explorer-blockrow-blockrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/BlockRow.tsx:32</code> | <code>export function BlockRow({ block, isNew = false, onSelect, isSelected = false }: BlockRowProps)</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-buildexplorerurl</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:105</code> | <code>export function buildExplorerUrl(focus: ExplorerFocus): string</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-explorerfocus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:74</code> | <code>export type ExplorerFocus =</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-fmtgas</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:46</code> | <code>export function fmtGas(v: string &#124; null &#124; undefined): string</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-fmtgaspct</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:59</code> | <code>export function fmtGasPct(used: string &#124; null &#124; undefined, limit: string &#124; null &#124; undefined): string</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-lookslikeblockheight</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:123</code> | <code>export function looksLikeBlockHeight(s: string): boolean</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-lookslikehash</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:116</code> | <code>export function looksLikeHash(s: string): boolean</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-parsefocus</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:83</code> | <code>export function parseFocus( query: Record&lt;string, string &#124; Array&lt;string&gt; &#124; undefined&gt; &#124; URLSearchParams, ): ExplorerFocus</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-torelativetime</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:15</code> | <code>export function toRelativeTime(input: string &#124; number &#124; Date &#124; null &#124; undefined): string</code> |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-truncatehash</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/explorer-utils.ts:36</code> | <code>export function truncateHash(s: string &#124; null &#124; undefined, front = 8, back = 4): string</code> |
| <code>interface.3.ts-perps-widgets-explorer-liveblockstream-liveblockstream</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/LiveBlockStream.tsx:35</code> | <code>export function LiveBlockStream({ onSelectBlock, selectedBlockHash, maxBlocks = 100, }: LiveBlockStreamProps)</code> |
| <code>interface.3.ts-perps-widgets-explorer-txdetailpanel-txdetailpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/explorer/TxDetailPanel.tsx:45</code> | <code>export function TxDetailPanel({ txHash, onClose }: TxDetailPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsfilterbar-marketsfilterbar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsFilterBar.tsx:27</code> | <code>export function MarketsFilterBar({ category, onChangeCategory, search, onChangeSearch, favoriteCount, className, }: MarketsFilterBarProps)</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsheadertabs-marketsheadertabs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx:35</code> | <code>export function MarketsHeaderTabs({ activeTab, onChangeTab, mode = "perps", onChangeMode, rightSlot, }: MarketsHeaderTabsProps)</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsheadertabs-marketsmode</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx:7</code> | <code>export type MarketsMode = "perps" &#124; "spot";</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsheadertabs-marketsmodes</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx:9</code> | <code>export function marketsModes(spotOpen: boolean = isSpotOpen()): MarketsMode[]</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsheadertabs-marketstab</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsHeaderTabs.tsx:6</code> | <code>export type MarketsTab = "markets" &#124; "funding";</code> |
| <code>interface.3.ts-perps-widgets-markets-marketshighlights-marketshighlights</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsHighlights.tsx:56</code> | <code>export function MarketsHighlights({ rows, hasAnyLive, variant = "columns", className, }: MarketsHighlightsProps)</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsoverviewstats-marketsoverviewstats</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsOverviewStats.tsx:67</code> | <code>export function MarketsOverviewStats({ rows, layout = "column", className, }: MarketsOverviewStatsProps)</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsskeletons-highlightspanelskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsSkeletons.tsx:51</code> | <code>export function HighlightsPanelSkeleton({ rows = 5 }: { rows?: number })</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsskeletons-marketstableskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsSkeletons.tsx:80</code> | <code>export function MarketsTableSkeleton({ rows = 8, hiddenColsMd = false, hiddenColsLg = true, }: { rows?: number; hiddenColsMd?: boolean; hiddenColsLg?: boolean; })</code> |
| <code>interface.3.ts-perps-widgets-markets-marketsskeletons-overviewstatsskeleton</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsSkeletons.tsx:31</code> | <code>export function OverviewStatsSkeleton()</code> |
| <code>interface.3.ts-perps-widgets-markets-marketstable-marketstable</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsTable.tsx:32</code> | <code>export function MarketsTable({ rows, loading, emptyMessage, favorites, onToggleFavorite, sortKey, sortDir, onToggleSort, }: MarketsTableProps)</code> |
| <code>interface.3.ts-perps-widgets-markets-marketstable-usesortedpagerows</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/MarketsTable.tsx:339</code> | <code>export function useSortedPageRows( rows: readonly MarketRow[], pageSize: number, page: number, ):</code> |
| <code>interface.3.ts-perps-widgets-markets-types-marketrow</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/types.ts:10</code> | <code>export interface MarketRow extends PerpsMarket</code> |
| <code>interface.3.ts-perps-widgets-markets-types-marketscategoryfilter</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/types.ts:37</code> | <code>export type MarketsCategoryFilter =</code> |
| <code>interface.3.ts-perps-widgets-markets-types-marketshighlight</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/types.ts:45</code> | <code>export type MarketsHighlight = "new" &#124; "gainers" &#124; "losers";</code> |
| <code>interface.3.ts-perps-widgets-markets-types-sortdir</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/types.ts:55</code> | <code>export type SortDir = "asc" &#124; "desc";</code> |
| <code>interface.3.ts-perps-widgets-markets-types-sortkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/types.ts:47</code> | <code>export type SortKey =</code> |
| <code>interface.3.ts-perps-widgets-markets-usemarketsrows-filterandsort</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/markets/useMarketsRows.ts:88</code> | <code>export function filterAndSort( rows: readonly MarketRow[], opts: { search?: string; category?: string; favorites?: ReadonlySet&lt;string&gt;; sortKey?: import("./types").SortKey; sortDir?: import("./types").SortDir; }, ): MarketRow[]</code> |
| <code>interface.3.ts-perps-widgets-markets-usemarketsrows-new-market-ids</code> | <code>ts_export</code> | <code>retire</code> | <code>apps/perps/src/widgets/markets/useMarketsRows.ts:20</code> | <code>export const NEW_MARKET_IDS: ReadonlySet&lt;number&gt; = new Set([5, 6, 7, 8, 9, 10]);</code> |
| <code>interface.3.ts-perps-widgets-markets-usemarketsrows-usemarketsrows</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/widgets/markets/useMarketsRows.ts:53</code> | <code>export function useMarketsRows():</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobileaccountview-mobileaccountview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileAccountView.tsx:9</code> | <code>export function MobileAccountView()</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobileapplayout-mobileapplayout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileAppLayout.tsx:18</code> | <code>export function MobileAppLayout({ market, onSelectMarket }: MobileAppLayoutProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilebalancepanel-mobilebalancepanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileBalancePanel.tsx:9</code> | <code>export function MobileBalancePanel()</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilebottomnav-mobilebottomnav</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileBottomNav.tsx:16</code> | <code>export function MobileBottomNav({ active, onChange }: MobileBottomNavProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilebottomnav-mobileglobaltab</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileBottomNav.tsx:3</code> | <code>export type MobileGlobalTab = "markets" &#124; "trade" &#124; "account";</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobileheader-mobileheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileHeader.tsx:11</code> | <code>export function MobileHeader()</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilelanguagetoggle-languagetoggle</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileLanguageToggle.tsx:14</code> | <code>export function LanguageToggle()</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilemarketview-mobilemarketview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileMarketView.tsx:36</code> | <code>export function MobileMarketView({ market }: MobileMarketViewProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilepaneltoolbar-mobilepaneltoolbar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobilePanelToolbar.tsx:7</code> | <code>export function MobilePanelToolbar({ sortValue = "Market", onSortChange, onExpandAll }: MobilePanelToolbarProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilesecondarytabcontent-mobilesecondarytabcontent</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileSecondaryTabContent.tsx:63</code> | <code>export function MobileSecondaryTabContent({ activeTab }: MobileSecondaryTabContentProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilesecondarytabs-mobilesecondarytabs</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileSecondaryTabs.tsx:18</code> | <code>export function MobileSecondaryTabs({ active, onChange }: MobileSecondaryTabsProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilespotaccountview-mobilespotaccountview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileSpotAccountView.tsx:24</code> | <code>export function MobileSpotAccountView()</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilespotapplayout-mobilespotapplayout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileSpotAppLayout.tsx:38</code> | <code>export function MobileSpotAppLayout({ market, onSelectMarket, }: MobileSpotAppLayoutProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilespotbalancepanel-mobilespotbalancepanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileSpotBalancePanel.tsx:24</code> | <code>export function MobileSpotBalancePanel()</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilespotmarketview-mobilespotmarketview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileSpotMarketView.tsx:49</code> | <code>export function MobileSpotMarketView({ market }: MobileSpotMarketViewProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilespottickerbar-mobilespottickerbar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileSpotTickerBar.tsx:37</code> | <code>export function MobileSpotTickerBar({ market, onOpenSelector }: MobileSpotTickerBarProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilespottradeview-mobilespottradeview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileSpotTradeView.tsx:38</code> | <code>export function MobileSpotTradeView({ market }: MobileSpotTradeViewProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobiletickerbar-mobiletickerbar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileTickerBar.tsx:31</code> | <code>export function MobileTickerBar({ market, onOpenSelector }: MobileTickerBarProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobiletradeview-mobiletradeview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileTradeView.tsx:23</code> | <code>export function MobileTradeView({ market }: MobileTradeViewProps)</code> |
| <code>interface.3.ts-perps-widgets-mobile-mobilevaultview-mobilevaultview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/mobile/MobileVaultView.tsx:15</code> | <code>export function MobileVaultView()</code> |
| <code>interface.3.ts-perps-widgets-network-pill-networkpill-default</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/network-pill/NetworkPill.tsx:314</code> | <code>export default NetworkPill;</code> |
| <code>interface.3.ts-perps-widgets-network-pill-networkpill-networkpill</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/network-pill/NetworkPill.tsx:95</code> | <code>export function NetworkPill({ className, style }: NetworkPillProps)</code> |
| <code>interface.3.ts-perps-widgets-network-pill-networkpill-networkpillprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/network-pill/NetworkPill.tsx:46</code> | <code>export interface NetworkPillProps</code> |
| <code>interface.3.ts-perps-widgets-paxscan-search-paxscansearch-paxscansearch</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/paxscan-search/PaxscanSearch.tsx:50</code> | <code>export function PaxscanSearch({ className, placeholder = "Search tx, address, block, token...", }: PaxscanSearchProps)</code> |
| <code>interface.3.ts-perps-widgets-paxscan-search-paxscansearch-paxscansearchprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/paxscan-search/PaxscanSearch.tsx:40</code> | <code>export interface PaxscanSearchProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-feesbreakdowncard-feesbreakdowncard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/FeesBreakdownCard.tsx:52</code> | <code>export function FeesBreakdownCard({ userAddress }: FeesBreakdownCardProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-feesbreakdowncard-groupfees</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/FeesBreakdownCard.tsx:32</code> | <code>export function groupFees(data: readonly { feeType: number; feeTypeName?: string &#124; null; feeUsd: string &#124; null &#124; undefined }[]): FeeGroup[]</code> |
| <code>interface.3.ts-perps-widgets-portfolio-mobileportfoliooverview-mobileportfoliooverview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/MobilePortfolioOverview.tsx:71</code> | <code>export function MobilePortfolioOverview({ address, totalVaultUsd, totalPnl, userStats, balances, trades, vaultEvents, isLoading, onRefresh, action, onActionChange,</code> |
| <code>interface.3.ts-perps-widgets-portfolio-onchainhistorypanel-onchainhistorypanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx:49</code> | <code>export function OnChainHistoryPanel({ address, variant = "desktop", className, initialSubTab = "transfers", }: OnChainHistoryPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-onchainhistorypanel-onchainhistorypanelprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx:33</code> | <code>export interface OnChainHistoryPanelProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-onchainhistorypanel-onchainhistorysubtab</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx:30</code> | <code>export type OnChainHistorySubTab = "transfers" &#124; "transactions";</code> |
| <code>interface.3.ts-perps-widgets-portfolio-onchainhistorypanel-onchainhistoryvariant</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/OnChainHistoryPanel.tsx:31</code> | <code>export type OnChainHistoryVariant = "desktop" &#124; "mobile";</code> |
| <code>interface.3.ts-perps-widgets-portfolio-performancecards-performancecards</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PerformanceCards.tsx:32</code> | <code>export function PerformanceCards({ dailyPnl, isLoading, }: PerformanceCardsProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-performancecards-sevendaytotals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PerformanceCards.tsx:26</code> | <code>export function sevenDayTotals(dailyPnl: DailyPnl[]):</code> |
| <code>interface.3.ts-perps-widgets-portfolio-pnlchart-pnlchart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PnlChart.tsx:24</code> | <code>export function PnlChart({ dailyPnl }: PnlChartProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioassetschart-assetstimerange</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx:18</code> | <code>export type AssetsTimeRange = "7D" &#124; "30D" &#124; "90D" &#124; "ALL";</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioassetschart-buildassetseries</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx:189</code> | <code>export function buildAssetSeries( dailyPnl: DailyPnl[], anchorValue: number, range: AssetsTimeRange, ): ChartPoint[]</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioassetschart-portfolioassetschart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioAssetsChart.tsx:39</code> | <code>export function PortfolioAssetsChart({ dailyPnl, totalVaultUsd, isLoading, range: externalRange, onChangeRange, className, }: PortfolioAssetsChartProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliofeetierpanel-portfoliofeetierpanel</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/widgets/portfolio/PortfolioFeeTierPanel.tsx:25</code> | <code>export function PortfolioFeeTierPanel({ address }: PortfolioFeeTierPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliofeetierpanel-portfoliofeetierpanelprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioFeeTierPanel.tsx:21</code> | <code>export interface PortfolioFeeTierPanelProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliomobilenav-portfoliomobilenav</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioMobileNav.tsx:39</code> | <code>export function PortfolioMobileNav({ active, onChange }: PortfolioMobileNavProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliomobilenav-portfoliomobilenavprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioMobileNav.tsx:34</code> | <code>export interface PortfolioMobileNavProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliooverviewpanel-portfoliooverviewpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioOverviewPanel.tsx:67</code> | <code>export function PortfolioOverviewPanel(props: PortfolioOverviewPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioperformancegrid-buildseries</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx:426</code> | <code>export function buildSeries(dailyPnl: DailyPnl[], range: PerformanceRange, now: Date = new Date()): SeriesPoint[]</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioperformancegrid-performancerange</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx:21</code> | <code>export type PerformanceRange = "7D" &#124; "30D" &#124; "90D" &#124; "ALL";</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioperformancegrid-portfolioperformancegrid</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx:52</code> | <code>export function PortfolioPerformanceGrid({ dailyPnl, isLoading, error, skippedDays = 0, className }: Props)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioperformancegrid-slicebyrange</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx:392</code> | <code>export function sliceByRange(dailyPnl: DailyPnl[], range: PerformanceRange, now: Date = new Date()): DailyPnl[]</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioperformancegrid-windowstartkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx:386</code> | <code>export function windowStartKey(range: Exclude&lt;PerformanceRange, "ALL"&gt;, now: Date = new Date()): string</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioperformancegrid-windowtotals</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx:400</code> | <code>export interface WindowTotals</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioperformancegrid-windowtotals-2</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPerformanceGrid.tsx:409</code> | <code>export function windowTotals(rows: DailyPnl[]): WindowTotals &#124; null</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliopositionspanel-portfoliopositionspanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPositionsPanel.tsx:41</code> | <code>export function PortfolioPositionsPanel({ positions, isConnected, hasVault, onConnect, }: PortfolioPositionsPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliopositionspanel-portfoliopositionspanelprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPositionsPanel.tsx:21</code> | <code>export interface PortfolioPositionsPanelProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-connectgate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx:88</code> | <code>export function ConnectGate({ isConnected, onConnect }: ConnectGateProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-connectgateprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx:83</code> | <code>export interface ConnectGateProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-emptystate</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx:47</code> | <code>export function EmptyState({ title, description }: EmptyStateProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-emptystateprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx:42</code> | <code>export interface EmptyStateProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-placeholderpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx:68</code> | <code>export function PlaceholderPanel({ title, subtitle, hint }: PlaceholderPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-placeholderpanelprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx:60</code> | <code>export interface PlaceholderPanelProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-sectioncard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx:22</code> | <code>export function SectionCard({ title, count, children }: SectionCardProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-sectioncardprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioPrimitives.tsx:15</code> | <code>export interface SectionCardProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliosettingspanel-portfoliosettingspanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioSettingsPanel.tsx:15</code> | <code>export function PortfolioSettingsPanel({ onOpenFullSettings, }: PortfolioSettingsPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliosettingspanel-portfoliosettingspanelprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioSettingsPanel.tsx:11</code> | <code>export interface PortfolioSettingsPanelProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliosidenav-portfolio-nav-items</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx:33</code> | <code>const NAV_ITEMS: NavItem[] = [</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliosidenav-portfolionavkey</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx:16</code> | <code>export type PortfolioNavKey =</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliosidenav-portfoliosidenav</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioSideNav.tsx:56</code> | <code>export function PortfolioSideNav({ active, onChange, collapsed, onToggleCollapsed, }: PortfolioSideNavProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliospotsection-portfoliospotsection</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioSpotSection.tsx:12</code> | <code>export function PortfolioSpotSection({ title, emptyMessage }: PortfolioSpotSectionProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliotopbar-portfoliotopbar</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioTopBar.tsx:39</code> | <code>export function PortfolioTopBar({ activeNav, isConnected, hasVault, vaultResolved, onDeposit, onWithdraw, onConnect, }: PortfolioTopBarProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliotopbar-portfoliotopbarprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/PortfolioTopBar.tsx:28</code> | <code>export interface PortfolioTopBarProps</code> |
| <code>interface.3.ts-perps-widgets-portfolio-tradesettlementhistory-absusd</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/TradeSettlementHistory.tsx:35</code> | <code>export function absUsd(v: string &#124; null &#124; undefined): number</code> |
| <code>interface.3.ts-perps-widgets-portfolio-tradesettlementhistory-tradesettlementhistory</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/TradeSettlementHistory.tsx:40</code> | <code>export function TradeSettlementHistory({ userAddress }: TradeSettlementHistoryProps)</code> |
| <code>interface.3.ts-perps-widgets-portfolio-types-tokenbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/portfolio/types.ts:9</code> | <code>export interface TokenBalance</code> |
| <code>interface.3.ts-perps-widgets-spot-spotmarketwatchlist-spotmarketwatchlist</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/widgets/spot/SpotMarketWatchlist.tsx:164</code> | <code>export function SpotMarketWatchlist({ selectedSymbol, onSelect, }: SpotMarketWatchlistProps)</code> |
| <code>interface.3.ts-perps-widgets-spot-spotorderbook-spotorderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotOrderbook.tsx:65</code> | <code>export function SpotOrderbook({ symbol, height = 580, onPriceClick, compact = false, }: SpotOrderbookProps)</code> |
| <code>interface.3.ts-perps-widgets-spot-spotorderbook-spotorderbooklevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotOrderbook.tsx:7</code> | <code>export interface SpotOrderbookLevel</code> |
| <code>interface.3.ts-perps-widgets-spot-spotorderform-spotconversionprice</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotOrderForm.tsx:139</code> | <code>export function spotConversionPrice(p: { orderType: SpotOrderType; side: "BUY" &#124; "SELL"; limitPrice: string; book: Pick&lt;SpotOrderbook, "bids" &#124; "asks"&gt; &#124; null; }):</code> |
| <code>interface.3.ts-perps-widgets-spot-spotorderform-spotorderform</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/widgets/spot/SpotOrderForm.tsx:307</code> | <code>export function SpotOrderForm({ market, compact = false }: SpotOrderFormProps)</code> |
| <code>interface.3.ts-perps-widgets-spot-spotorderform-spotorderunits</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotOrderForm.tsx:102</code> | <code>export function spotOrderUnits(p: { sizeMode: SizeMode; amount: string; price: string; baseDecimals: number; quoteDecimals: number; }):</code> |
| <code>interface.3.ts-perps-widgets-spot-spotorderform-spotsellexceedsbalance</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotOrderForm.tsx:164</code> | <code>export function spotSellExceedsBalance( baseUnits: bigint, holding: { raw: string; decimals: number } &#124; null, baseDecimals: number, ): boolean</code> |
| <code>interface.3.ts-perps-widgets-spot-spotpairheader-spotpairheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotPairHeader.tsx:64</code> | <code>export function SpotPairHeader({ selectedPair, onPairChange }: SpotPairHeaderProps)</code> |
| <code>interface.3.ts-perps-widgets-spot-spotpositionspanel-spotpositionspanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotPositionsPanel.tsx:110</code> | <code>export function SpotPositionsPanel()</code> |
| <code>interface.3.ts-perps-widgets-spot-spottradestape-spottradestape</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotTradesTape.tsx:87</code> | <code>export function SpotTradesTape({ symbol, compact = false }: SpotTradesTapeProps)</code> |
| <code>interface.3.ts-perps-widgets-spot-spottradestape-spottradestapeprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/spot/SpotTradesTape.tsx:8</code> | <code>export interface SpotTradesTapeProps</code> |
| <code>interface.3.ts-perps-widgets-trading-chart-chart</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/Chart.tsx:131</code> | <code>export function Chart({ selectedPair = "ETH/USD", height, onHeightChange }: ChartProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-marketselectorpanel-marketselectorpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/MarketSelectorPanel.tsx:403</code> | <code>export { MarketWatchlist as MarketSelectorPanel };</code> |
| <code>interface.3.ts-perps-widgets-trading-marketselectorpanel-marketwatchlist</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/MarketSelectorPanel.tsx:36</code> | <code>export const MarketWatchlist = forwardRef&lt;HTMLDivElement, MarketWatchlistProps&gt;(function MarketWatchlist({ chrome = "rail", width, collapsed, onToggle, onSelect, selectedSymbol }, ref)</code> |
| <code>interface.3.ts-perps-widgets-trading-markettradestape-markettradestape</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/MarketTradesTape.tsx:39</code> | <code>export function MarketTradesTape({ symbol }: MarketTradesTapeProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-markettradestape-markettradestapeprops</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/MarketTradesTape.tsx:6</code> | <code>export interface MarketTradesTapeProps</code> |
| <code>interface.3.ts-perps-widgets-trading-mobiletradinglayout-mobiletradinglayout</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/MobileTradingLayout.tsx:210</code> | <code>export function MobileTradingLayout({ market, onSelectMarket }: { market: PerpsMarket; onSelectMarket: (symbol: string) =&gt; void })</code> |
| <code>interface.3.ts-perps-widgets-trading-modals-adjustcollateralmodal-adjustcollateralmodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/modals/AdjustCollateralModal.tsx:37</code> | <code>export function AdjustCollateralModal({ positionId, mode, currentCollateral, currentSize, currentLeverage, onClose, onSuccess, }: AdjustCollateralModalProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-modals-settpslmodal-settpslmodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/modals/SetTpSlModal.tsx:28</code> | <code>export function SetTpSlModal({ positionId, currentEntryPrice, isLong, markPrice, onClose, onSuccess, }: SetTpSlModalProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-modals-topuppositionmodal-topuppositionmodal</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/modals/TopUpPositionModal.tsx:25</code> | <code>export function TopUpPositionModal({ positionId, userAddress, onClose, onSuccess, }: TopUpPositionModalProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-orderbook-depthpanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/Orderbook.tsx:468</code> | <code>export function DepthPanel({ symbol, height, onPriceClick }: DepthPanelProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-orderbook-orderbook</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/Orderbook.tsx:72</code> | <code>export function Orderbook({ symbol, height = 580, onPriceClick, compact = false }: OrderbookProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-orderbook-orderbooklevel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/Orderbook.tsx:7</code> | <code>export interface OrderbookLevel</code> |
| <code>interface.3.ts-perps-widgets-trading-pairheader-pairheader</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/PairHeader.tsx:67</code> | <code>export function PairHeader({ selectedPair, onPairChange, onOpenMarketPanel }: PairHeaderProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-perpsorderform-perpsorderform</code> | <code>ts_export</code> | <code>change</code> | <code>apps/perps/src/widgets/trading/PerpsOrderForm.tsx:225</code> | <code>export function PerpsOrderForm({ market, compact = false }: { market: PerpsMarket; compact?: boolean })</code> |
| <code>interface.3.ts-perps-widgets-trading-perpspositionspanel-perpspositionspanel</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/PerpsPositionsPanel.tsx:670</code> | <code>export function PerpsPositionsPanel()</code> |
| <code>interface.3.ts-perps-widgets-trading-pnlsharecard-pnlcarddata</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/PnlShareCard.tsx:13</code> | <code>export interface PnlCardData</code> |
| <code>interface.3.ts-perps-widgets-trading-pnlsharecard-pnlsharecard</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/PnlShareCard.tsx:28</code> | <code>export function PnlShareCard({ data, onClose }: PnlShareCardProps)</code> |
| <code>interface.3.ts-perps-widgets-trading-tradingviewloader-loadtradingview</code> | <code>ts_export</code> | <code>stable</code> | <code>apps/perps/src/widgets/trading/tradingViewLoader.ts:23</code> | <code>export function loadTradingView(): Promise&lt;void&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.ts-perps-widgets-alerts-alertspanel-alertspanel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-analytics-analyticsliquidstat-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-errors-errortoast-errorglyph</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-errors-errortoast-showerrortoastoptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-errors-errortoast-toasterroroptions</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-errors-reportfeedbackmodal-closereportmodal</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-explorer-blockdetailpanel-blockdetailpanel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-buildexplorerurl</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-explorerfocus</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-lookslikeblockheight</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-lookslikehash</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-explorer-explorer-utils-parsefocus</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-explorer-liveblockstream-liveblockstream</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-explorer-txdetailpanel-txdetailpanel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-markets-usemarketsrows-new-market-ids</code> | overridden_by | override.J27.38 |
| <code>interface.3.ts-perps-widgets-markets-usemarketsrows-usemarketsrows</code> | overridden_by | override.J27.21 |
| <code>interface.3.ts-perps-widgets-mobile-mobilevaultview-mobilevaultview</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-network-pill-networkpill-default</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-network-pill-networkpill-networkpillprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-paxscan-search-paxscansearch-paxscansearchprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-onchainhistorypanel-onchainhistorypanelprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-onchainhistorypanel-onchainhistorysubtab</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-onchainhistorypanel-onchainhistoryvariant</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-performancecards-performancecards</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-pnlchart-pnlchart</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioassetschart-assetstimerange</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliofeetierpanel-portfoliofeetierpanel</code> | overridden_by | override.J27.24 |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliofeetierpanel-portfoliofeetierpanelprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliomobilenav-portfoliomobilenavprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliopositionspanel-portfoliopositionspanelprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-connectgateprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-emptystateprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-placeholderpanelprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfolioprimitives-sectioncardprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliosettingspanel-portfoliosettingspanelprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliosidenav-portfolio-nav-items</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-portfolio-portfoliotopbar-portfoliotopbarprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-spot-spotmarketwatchlist-spotmarketwatchlist</code> | overridden_by | override.J27.26 |
| <code>interface.3.ts-perps-widgets-spot-spotorderform-spotorderform</code> | overridden_by | override.J27.25 |
| <code>interface.3.ts-perps-widgets-trading-marketselectorpanel-marketselectorpanel</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-trading-markettradestape-markettradestape</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-trading-markettradestape-markettradestapeprops</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-trading-mobiletradinglayout-mobiletradinglayout</code> | notes | no in-repo importer at HEAD |
| <code>interface.3.ts-perps-widgets-trading-perpsorderform-perpsorderform</code> | overridden_by | override.J27.18 |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.519.1</code> | req | 3519.1 |
| <code>acceptance.3.519.1</code> | scenario | Given limits with tickSize 0.5 USD and lotSize 0.001, when buildOrderPlan runs for limit orders at 100.3 and 100.5 USD, then the first is refused with VALIDATION_PRECISION and the second plans one placeOrder call |
| <code>acceptance.3.519.1</code> | test | apps/perps/tests/order-plan.test.ts::buildOrderPlanAppliesLimits (planned) |
| <code>acceptance.3.519.1</code> | qualification | UNRUN |
| <code>req.3519</code> | ac_1 | buildOrderPlan (apps/perps/src/core/perps/orderPlan.ts:201) given limits with tickSize 0.5 refuses a limit order at 100.3 with VALIDATION_PRECISION and builds the placeOrder call at 100.5. |
| <code>acceptance.3.521.1</code> | req | 3521.1 |
| <code>acceptance.3.521.1</code> | scenario | Given rows built from a catalog where only DOGE was listed 3 days ago, when filterAndSort runs with category new, then it returns only DOGE |
| <code>acceptance.3.521.1</code> | test | apps/perps/tests/markets-rows.test.ts::newFilterUsesListingTime (planned) |
| <code>acceptance.3.521.1</code> | qualification | UNRUN |
| <code>req.3521</code> | ac_1 | filterAndSort with category new keeps the catalog rows listed within 14 days and drops markets 5 to 10 that the fixed list of apps/perps/src/widgets/markets/useMarketsRows.ts:20 marked new. |
| <code>acceptance.3.522.1</code> | req | 3522.1 |
| <code>acceptance.3.522.1</code> | scenario | Given a catalog with DOGE, when MobileTradeHeader renders to static markup, then the markup names DOGE |
| <code>acceptance.3.522.1</code> | test | apps/perps/tests/mobile-markets.test.ts::mobileHeaderListsCatalogMarket (planned) |
| <code>acceptance.3.522.1</code> | qualification | UNRUN |
| <code>req.3522</code> | ac_1 | MobileTradeHeader rendered with a catalog holding DOGE shows DOGE in its market list, which the PERPS_MARKETS list of apps/perps/src/app/shell/MobileTradeHeader.tsx:45 lacks. |
| <code>acceptance.3.523.1</code> | req | 3523.1 |
| <code>acceptance.3.523.1</code> | scenario | Given a settlement of market 18 and a catalog with DOGE, when the row renders to static markup, then it names DOGE |
| <code>acceptance.3.523.1</code> | test | apps/perps/tests/market-label-widgets.test.ts::settlementRowNamesChainMarket (planned) |
| <code>acceptance.3.523.1</code> | qualification | UNRUN |
| <code>req.3523</code> | ac_1 | A TradeSettlementHistory row for market 18 renders DOGE from the catalog, where the lookup of apps/perps/src/widgets/portfolio/TradeSettlementHistory.tsx:31 finds no PERPS_MARKETS entry. |
| <code>acceptance.3.524.1</code> | req | 3524.1 |
| <code>acceptance.3.524.1</code> | scenario | Given a SpotTokenState with paused true, when spotSubmitBlocked runs, then it returns TRADE_MARKET_HALTED, and for paused false it returns null |
| <code>acceptance.3.524.1</code> | test | apps/perps/tests/spot-order-paused.test.ts::pausedTokenBlocksSubmit (planned) |
| <code>acceptance.3.524.1</code> | qualification | UNRUN |
| <code>req.3524</code> | ac_1 | spotSubmitBlocked returns TRADE_MARKET_HALTED for a paused token state, a condition submitDisabled of apps/perps/src/widgets/spot/SpotOrderForm.tsx:464-473 does not check. |
| <code>acceptance.3.525.1</code> | req | 3525.1 |
| <code>acceptance.3.525.1</code> | scenario | Given SpotTokenState values with paused true and false, when watchlistRowState runs, then it returns paused and live |
| <code>acceptance.3.525.1</code> | test | apps/perps/tests/spot-watchlist-paused.test.ts::watchlistMarksPausedToken (planned) |
| <code>acceptance.3.525.1</code> | qualification | UNRUN |
| <code>req.3525</code> | ac_1 | watchlistRowState returns paused for a paused SpotTokenState and live otherwise, so the row of apps/perps/src/widgets/spot/SpotMarketWatchlist.tsx:175 shows the badge. |
| <code>acceptance.3.526.1</code> | req | 3526.1 |
| <code>acceptance.3.526.1</code> | scenario | Given a FeeTierView with tier 2, takerBps 2.5 and makerBps 0, when PortfolioFeeTierPanel renders to static markup, then it shows Tier 2 and Taker 2.5 bps |
| <code>acceptance.3.526.1</code> | test | apps/perps/tests/fee-tier-panel.test.ts::feeTierPanelShowsChainTier (planned) |
| <code>acceptance.3.526.1</code> | qualification | UNRUN |
| <code>req.3526</code> | ac_1 | PortfolioFeeTierPanel (apps/perps/src/widgets/portfolio/PortfolioFeeTierPanel.tsx:25) given a view with tier 2, takerBps 2.5 and makerBps 0 renders Tier 2 · Taker 2.5 bps / Maker 0 bps and not the fixed text of apps/perps/src/widgets/portfolio/PortfolioFeeTierPanel.tsx:39. |
| <code>capability.3.48.11</code> | title | List mobile trade header and layout markets from the catalog |
| <code>capability.3.48.11</code> | reqs | 3522.1 |
| <code>capability.3.48.14</code> | title | Label positions, alerts and settlements with catalog symbols |
| <code>capability.3.48.14</code> | reqs | 3523.1 |
| <code>capability.3.48.5</code> | title | Apply chain order limits in PerpsOrderForm and buildOrderPlan |
| <code>capability.3.48.5</code> | reqs | 3519.1 |
| <code>capability.3.48.8</code> | title | List selector rows, pair header and analytics from the market catalog |
| <code>capability.3.48.8</code> | reqs | 3521.1 |
| <code>capability.3.52.2</code> | title | Block spot order submit for a paused token |
| <code>capability.3.52.2</code> | reqs | 3524.1 |
| <code>capability.3.52.3</code> | title | Badge paused tokens in the spot watchlist |
| <code>capability.3.52.3</code> | reqs | 3525.1 |
| <code>capability.3.53.2</code> | title | Show the account fee tier from getFeeTier in the portfolio panel |
| <code>capability.3.53.2</code> | reqs | 3526.1 |
| <code>resolution.17.606</code> | title | decision.344 binds: The app market catalog comes from gateway GraphQL; order limits come from chain views |
| <code>resolution.17.606</code> | applies_to | decision.344<br>task.3.48.5<br>task.3.48.7<br>task.3.55.1 |
| <code>resolution.17.606</code> | source | decision.344 in fragments/J27/decision.kvx; tasks from affects |
