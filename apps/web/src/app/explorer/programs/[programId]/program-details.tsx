import { copyEntry } from "../../../../../copy/runtime";
import { ExplorerFrame, explorerReceiptPath } from "../../../../explorer/components";
import type { ProgramRecord } from "../../../../explorer/model";
import {
  ExplorerLink,
  ExplorerFreshness,
  ExplorerTable,
  ExplorerVerificationBadge,
} from "../../../../kit/explorer";

export function ProgramUnavailable() {
  return (
    <ExplorerFrame
      title={copyEntry("explorer.program.title").message}
      description={copyEntry("explorer.freshness.unavailable.body").message}
    >
      <ExplorerFreshness
        title={copyEntry("explorer.freshness.unavailable").message}
        description={copyEntry("explorer.freshness.unavailable.body").message}
        current={false}
      />
    </ExplorerFrame>
  );
}

export function reportedObservationAge(observedAt: string, now: number): string {
  const elapsed = BigInt(Math.trunc(now)) - BigInt(observedAt);
  return elapsed < 0n
    ? `Unknown: reported timestamp is ${(-elapsed).toString()} ms ahead of this server's clock.`
    : `${elapsed.toString()} ms at page render.`;
}

export function ProgramDetails({
  program,
  now = Date.now(),
}: Readonly<{ program: ProgramRecord; now?: number }>) {
  const reported = "Reported";
  const policy = program.upgradePolicy.kind === "immutable"
    ? "immutable"
    : `upgradeable by ${program.upgradePolicy.authority}`;
  return (
    <ExplorerFrame
      title={copyEntry("explorer.program.title").message}
      description={program.program}
    >
      <ExplorerFreshness
        title={copyEntry("explorer.copy.reported_program_observation").message}
        description={`Reported sequence ${program.observedSequence} at ${program.observedAt} (Unix milliseconds). Reported age: ${reportedObservationAge(program.observedAt, now)} Currentness unknown.`}
        current={false}
      />
      <ExplorerTable
        caption={copyEntry("explorer.program.facts").message}
        columns={[
          copyEntry("explorer.column.fact").message,
          copyEntry("explorer.column.value").message,
          copyEntry("explorer.column.verification").message,
        ]}
        rows={[
          { id: "lifecycle", cells: [copyEntry("explorer.program.fact.lifecycle").message, program.lifecycle, reported] },
          { id: "policy", cells: [copyEntry("explorer.program.fact.upgrade_policy").message, policy, reported] },
          { id: "sequence", cells: [copyEntry("explorer.program.fact.observed_sequence").message, program.observedSequence, reported] },
          { id: "observed", cells: [copyEntry("explorer.program.fact.observed_at").message, program.observedAt, reported] },
          {
            id: "receipt",
            cells: [
              copyEntry("explorer.column.receipt").message,
              <ExplorerLink key="receipt" href={explorerReceiptPath(program.receiptDigest)}>
                {program.receiptDigest}
              </ExplorerLink>,
              reported,
            ],
          },
          { id: "root", cells: [copyEntry("explorer.program.fact.state_root").message, program.stateRoot, reported] },
        ]}
      />
      <ExplorerTable
        caption={copyEntry("explorer.program.versions").message}
        columns={[
          copyEntry("explorer.program.column.version").message,
          copyEntry("explorer.program.column.code_hash").message,
          copyEntry("explorer.program.column.abi").message,
          copyEntry("explorer.program.column.interface").message,
          copyEntry("explorer.program.column.source").message,
        ]}
        rows={program.versions.map((version) => ({
          id: version.version,
          cells: [
            version.version,
            version.codeHash,
            version.abiVersion,
            version.interfaceDigest ?? copyEntry("explorer.value.none").message,
            <ExplorerVerificationBadge
              key="source"
              label={copyEntry(`explorer.program.source.${version.source.status}`).message}
              unverified={version.source.status !== "verified"}
            />,
          ],
        }))}
      />
      <ExplorerTable
        caption={copyEntry("explorer.program.accounts").message}
        columns={[
          copyEntry("explorer.program.column.account").message,
          copyEntry("explorer.program.column.asset").message,
          copyEntry("explorer.program.column.balance").message,
          copyEntry("explorer.program.column.frozen").message,
          copyEntry("explorer.column.verification").message,
        ]}
        rows={program.valueAccounts.map((account) => ({
          id: `${account.account}-${account.asset}`,
          cells: [account.account, account.asset, account.balance, String(account.frozen), reported],
        }))}
      />
    </ExplorerFrame>
  );
}
