import { redirect } from "next/navigation";

import { copyEntry } from "../../../../../copy/runtime";
import { ExplorerFrame } from "../../../../explorer/components";
import { NativeAccountPage } from "../../../../explorer/native-components";
import { nativeAddress, type NativeUnifiedAccount } from "../../../../explorer/native-model";
import { loadNativeAccountPage, nativeAccountPagePath } from "../../../../explorer/native-page";

export const dynamic = "force-dynamic";

function ObservationPosition({ account }: Readonly<{ account: NativeUnifiedAccount }>) {
  const newest = account.activity.at(0);
  const oldest = account.activity.at(-1);
  return (
    <section aria-label="Observation position">
      <h2>Observation position</h2>
      {newest === undefined || oldest === undefined ? (
        <p>No committed activity position is published on this page.</p>
      ) : (
        <p>
          Observed activity on this page: committed block {newest.block_number}, ordinal {newest.ordinal}
          {" through committed block "}{oldest.block_number}, ordinal {oldest.ordinal}.
        </p>
      )}
      <p>This account snapshot does not establish the current network head.</p>
    </section>
  );
}

export default async function AccountPage({
  params,
  searchParams,
}: Readonly<{
  params: Promise<{ accountId: string }>;
  searchParams: Promise<{ cursor?: string; before?: string; beforeBlock?: string; before_block?: string }>;
}>) {
  const requested = (await params).accountId;
  let address: string;
  try {
    address = nativeAddress(requested).toLowerCase();
  } catch {
    return (
      <ExplorerFrame
        title={copyEntry("explorer.account.title").message}
        description={copyEntry("explorer.account.invalid").message}
      >
        <p>{copyEntry("explorer.not_found.body").message}</p>
      </ExplorerFrame>
    );
  }
  const query = await searchParams;
  if (query.before !== undefined || query.beforeBlock !== undefined || query.before_block !== undefined) {
    return (
      <ExplorerFrame title={copyEntry("explorer.account.title").message} description={address}>
        <section aria-label="Account snapshot refused" data-state="cursor-refused">
          <p>This account requires the returned snapshot cursor. The former activity position cannot be continued.</p>
          <a href={nativeAccountPagePath(address)}>Start a new account snapshot</a>
        </section>
      </ExplorerFrame>
    );
  }
  if (requested !== address) redirect(nativeAccountPagePath(address, query.cursor));
  const model = await loadNativeAccountPage(address, query.cursor);
  return (
    <ExplorerFrame title={copyEntry("explorer.account.title").message} description={address}>
      {model.state === "ready" ? <ObservationPosition account={model.account} /> : null}
      <NativeAccountPage model={model} />
    </ExplorerFrame>
  );
}
