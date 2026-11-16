import { copyEntry } from "../../copy/runtime";
import { formatCopy } from "../../copy/format";
import type { NativeAccountPageModel } from "./native-page.ts";

export function NativeAccountPage({
  model,
}: Readonly<{ model: NativeAccountPageModel }>) {
  if (model.state === "cursor-refused") {
    return (
      <section
        aria-label={copyEntry("explorer.copy.account_snapshot_refused").message}
        data-state={model.state}
      >
        <p>
          {formatCopy("explorer.account.snapshot_refused_detail", {
            reason: model.reason,
          })}
        </p>
        <a href={model.resetHref}>
          {copyEntry("explorer.copy.start_a_new_account_snapshot").message}
        </a>
      </section>
    );
  }
  if (model.state === "not-found")
    return (
      <p data-state={model.state}>
        {copyEntry("explorer.copy.account_not_found").message}
      </p>
    );
  if (model.state === "unavailable")
    return (
      <p data-state={model.state}>
        {copyEntry("explorer.copy.the_account_explorer_is_unavailable").message}
      </p>
    );
  const account = model.account;
  return (
    <article
      aria-label={copyEntry("explorer.copy.native_account").message}
      data-state="ready"
      className="flex min-w-0 flex-col gap-4"
    >
      <h1 className="break-all text-lg font-bold">{account.identities.evm}</h1>
      <dl
        className="grid gap-x-6 gap-y-2 sm:grid-cols-[auto_minmax(0,1fr)] [&_dt]:font-semibold [&_dd]:break-all"
        aria-label={copyEntry("explorer.copy.account_identities").message}
      >
        <dt>{copyEntry("explorer.copy.evm_address").message}</dt>
        <dd>{account.identities.evm}</dd>
        <dt>{copyEntry("explorer.copy.paxeer_address").message}</dt>
        <dd>{account.identities.pax ?? "Unavailable"}</dd>
        <dt>{copyEntry("explorer.copy.did").message}</dt>
        <dd>{account.identities.did ?? "Unavailable"}</dd>
      </dl>
      <div
        className="human-table-scroll rounded-md border border-border"
        tabIndex={-1}
      >
        <table className="human-data-table border-collapse text-left text-sm [&_th]:px-3 [&_th]:py-3 [&_td]:px-3 [&_td]:py-3 [&_thead]:bg-surface-sunken [&_tbody_tr]:border-t [&_tbody_tr]:border-border">
          <caption className="px-3 py-3 text-left font-semibold">
            {copyEntry("explorer.copy.account_totals_in_base_units").message}
          </caption>
          <thead>
            <tr>
              <th scope="col">{copyEntry("explorer.copy.asset").message}</th>
              <th scope="col" className="text-right">
                {copyEntry("explorer.copy.total").message}
              </th>
            </tr>
          </thead>
          <tbody>
            {account.balances.map((balance) => (
              <tr key={balance.asset.id}>
                <th scope="row">{balance.asset.denom}</th>
                <td className="text-right tabular-nums">{balance.total}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div
        className="human-table-scroll rounded-md border border-border"
        tabIndex={-1}
      >
        <table className="human-data-table border-collapse text-left text-sm [&_th]:px-3 [&_th]:py-3 [&_td]:px-3 [&_td]:py-3 [&_thead]:bg-surface-sunken [&_tbody_tr]:border-t [&_tbody_tr]:border-border">
          <caption className="px-3 py-3 text-left font-semibold">
            {copyEntry("explorer.copy.committed_transactions").message}
          </caption>
          <thead>
            <tr>
              <th scope="col">
                {copyEntry("explorer.copy.transaction").message}
              </th>
              <th scope="col" className="text-right">
                {copyEntry("explorer.copy.block").message}
              </th>
              <th scope="col" className="text-right">
                {copyEntry("explorer.copy.ordinal").message}
              </th>
              <th scope="col">{copyEntry("explorer.copy.status").message}</th>
              <th scope="col">{copyEntry("explorer.copy.result").message}</th>
              <th scope="col" className="text-right">
                {copyEntry("explorer.copy.amount_in_base_units").message}
              </th>
              <th scope="col">
                {copyEntry("explorer.copy.counterparty").message}
              </th>
              <th scope="col">{copyEntry("explorer.copy.time").message}</th>
            </tr>
          </thead>
          <tbody>
            {account.activity.map((item) => (
              <tr key={item.hash}>
                <th scope="row">{item.hash}</th>
                <td className="text-right tabular-nums">{item.block_number}</td>
                <td className="text-right tabular-nums">{item.ordinal}</td>
                <td>{item.status}</td>
                <td>{item.result ?? "Unavailable"}</td>
                <td className="text-right tabular-nums">
                  {item.amount} {item.asset.denom}
                </td>
                <td>{item.counterparty ?? "Unavailable"}</td>
                <td>
                  <time dateTime={item.timestamp}>{item.timestamp}</time>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p>
        {copyEntry("explorer.account.page").message} {account.page_number}
        {account.activity_total === null
          ? ""
          : formatCopy("explorer.account.transaction_total", {
              count: account.activity_total,
            })}
      </p>
      <nav
        className="flex flex-wrap gap-3 [&_a]:inline-flex [&_a]:min-h-11 [&_a]:items-center [&_a]:font-semibold [&_a]:text-accent"
        aria-label={copyEntry("explorer.copy.account_activity_pages").message}
      >
        <a href={model.refreshHref}>
          {copyEntry("explorer.copy.refresh_this_snapshot").message}
        </a>{" "}
        <a href={model.firstHref}>
          {copyEntry("explorer.copy.first_page_of_this_snapshot").message}
        </a>{" "}
        {model.nextHref === undefined ? null : (
          <a href={model.nextHref}>
            {copyEntry("explorer.copy.older_transactions").message}
          </a>
        )}
      </nav>
    </article>
  );
}
