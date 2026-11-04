"use client";

import { ErrorSurface, type ErrorPresentation } from "./error.tsx";
import { humanStateMatrix, type HumanShell } from "./matrix.ts";
import { redactSupportReportRoute } from "./report-routes.ts";
import { LoadingSurface, EmptySurface, OfflineSurface, DegradedSurface, StillCheckingSurface } from "./surfaces.tsx";

type MatrixState =
  | Readonly<{ state: "loading" | "empty" }>
  | Readonly<{ state: "error"; error: ErrorPresentation; onRetry: () => void; onReload: () => void }>
  | Readonly<{ state: "offline"; onRetry: () => void }>
  | Readonly<{ state: "degraded"; age: string; referencePoint: string; onReload: () => void }>
  | Readonly<{ state: "still-checking"; lookupOutcome: () => Promise<"pending" | "resolved">; onResolved: () => void }>;

export type MatrixStateSurfaceProps = MatrixState & Readonly<{ route: string; shell: HumanShell }>;

export function MatrixStateSurface(props: MatrixStateSurfaceProps) {
  const template = redactSupportReportRoute(props.route);
  const matrix = humanStateMatrix();
  matrix.assertComplete();
  const entry = matrix.enumerate().find((target) => target.kind === "route" && target.route === template);
  if (entry === undefined || !entry.shells.includes(props.shell)) throw new Error("Missing route state wiring");
  switch (props.state) {
    case "loading": return <LoadingSurface />;
    case "empty": return <EmptySurface />;
    case "error": return <ErrorSurface error={props.error} route={template} platform={props.shell}
      onRetry={props.onRetry} onReload={props.onReload} />;
    case "offline": return <OfflineSurface onRetry={props.onRetry} />;
    case "degraded": return <DegradedSurface age={props.age} referencePoint={props.referencePoint} onReload={props.onReload} />;
    case "still-checking": return <StillCheckingSurface lookupOutcome={props.lookupOutcome} onResolved={props.onResolved} />;
  }
}
