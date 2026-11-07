export {
  StateMatrix,
  REQUIRED_SCREEN_STATES,
  CURRENT_ROUTES,
  STATE_ROUTE_MANIFEST,
  CURRENT_SHELLS,
  humanStateMatrix,
  human_state_matrix_registry,
  type HumanShell,
  type ScreenState,
  type ScreenStateEntry,
  type StatePresentation,
  type StateTargetKind,
} from "./matrix";
export {
  ErrorBoundary,
  ErrorSurface,
  RouteErrorBoundary,
  errorPresentation,
  type ErrorPresentation,
} from "./error";
export {
  LoadingSurface,
  EmptySurface,
  OfflineSurface,
  DegradedSurface,
  StillCheckingSurface,
  type StillCheckingSurfaceProps,
} from "./surfaces";
export {
  OfflineBanner,
  QueuedActionNotice,
} from "./offline";
export {
  OfflineActionQueue,
  type MoneyOfflineAction,
  type QueueDecision,
  type QueueableOfflineAction,
} from "./queue";
export {
  BrowserSupportReportOutbox,
  browserSupportReportOutbox,
  redactErrorReport,
  clearPrivateSupportReports,
  requireSupportReportPrincipal,
  SupportReportPrincipalError,
  SUPPORT_REPORT_PRINCIPAL_HEADER,
  type SupportReportAttempt,
  type ErrorReporter,
  type ErrorReportDraft,
  type ErrorReportReceipt,
} from "./report";
export {
  parseSavedReportReceipt,
  parseStoredSupportReport,
  parseSupportReportRequest,
  supportReportTrace,
  type ReportShell,
  type SavedReportReceipt,
  type StoredSupportReport,
  type SupportReportRequest,
} from "./report-schema";
export { ApplicationStateBoundary } from "./application";
export {
  SUPPORT_REPORT_ROUTES,
  supportReportRoute,
  redactSupportReportRoute,
  type SupportReportRoute,
} from "./report-routes";
export { SupportReportSessionBoundary, useSupportReportOutbox } from "./report-session";
export { MatrixStateSurface, type MatrixStateSurfaceProps } from "./matrix-surface";
