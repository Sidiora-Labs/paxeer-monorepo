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
    <section aria-label={copyEntry("explorer.copy.observation_position").message}>
      <h2>{copyEntry("explorer.copy.observation_position").message}</h2>
      {newest === undefined || oldest === undefined ? (
        <p>{copyEntry("explorer.copy.no_committed_activity_position_is_published_on_this_").message}</p>
      ) : (
        <p>
          Observed activity on this page: committed block {newest.block_number}, ordinal {newest.ordinal}
          {" through committed block "}{oldest.block_number}, ordinal {oldest.ordinal}.
        </p>
      )}
      <p>{copyEntry("explorer.copy.this_account_snapshot_does_not_establish_the_current").message}</p>
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
        <section aria-label={copyEntry("explorer.copy.account_snapshot_refused").message} data-state="cursor-refused">
          <p>{copyEntry("explorer.copy.this_account_requires_the_returned_snapshot_cursor_t").message}</p>
          <a href={nativeAccountPagePath(address)}>{copyEntry("explorer.copy.start_a_new_account_snapshot").message}</a>
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
