import { copyEntry } from "../../../../../copy/runtime";
import { programRecord } from "../../../../explorer/client";
import { ExplorerFrame } from "../../../../explorer/components";
import { ProgramDetails, ProgramUnavailable } from "./program-details";

export const dynamic = "force-dynamic";

export default async function ProgramPage({
  params,
}: Readonly<{ params: Promise<{ programId: string }> }>) {
  let program;
  try {
    program = await programRecord((await params).programId);
  } catch {
    return <ProgramUnavailable />;
  }
  if (program === undefined) {
    return (
      <ExplorerFrame
        title={copyEntry("explorer.program.title").message}
        description={copyEntry("explorer.not_found").message}
      >
        <p className="text-sm text-foreground-secondary">
          {copyEntry("explorer.not_found.body").message}
        </p>
      </ExplorerFrame>
    );
  }
  return <ProgramDetails program={program} />;
}
