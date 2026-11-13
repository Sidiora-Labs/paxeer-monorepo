import { copyEntry } from "../../copy/runtime";
import { formatCopy } from "../../copy/format";
import type { NativeAccountPageModel } from "./native-page.ts";

export function NativeAccountPage({ model }: Readonly<{ model: NativeAccountPageModel }>) {
  if (model.state === "cursor-refused") {
    return (
      <section aria-label={copyEntry("explorer.copy.account_snapshot_refused").message} data-state={model.state}>
        <p>{formatCopy("explorer.account.snapshot_refused_detail", { reason: model.reason })}</p>
        <a href={model.resetHref}>{copyEntry("explorer.copy.start_a_new_account_snapshot").message}</a>
      </section>
    );
  }
  if (model.state === "not-found") return <p data-state={model.state}>{copyEntry("explorer.copy.account_not_found").message}</p>;
  if (model.state === "unavailable") return <p data-state={model.state}>{copyEntry("explorer.copy.the_account_explorer_is_unavailable").message}</p>;
  const account = model.account;
  return (
    <article aria-label={copyEntry("explorer.copy.native_account").message} data-state="ready">
      <h1>{account.identities.evm}</h1>
      <dl aria-label={copyEntry("explorer.copy.account_identities").message}>
        <dt>{copyEntry("explorer.copy.evm_address").message}</dt><dd>{account.identities.evm}</dd>
        <dt>{copyEntry("explorer.copy.paxeer_address").message}</dt><dd>{account.identities.pax ?? "Unavailable"}</dd>
        <dt>{copyEntry("explorer.copy.did").message}</dt><dd>{account.identities.did ?? "Unavailable"}</dd>
      </dl>
      <table>
        <caption>{copyEntry("explorer.copy.account_totals_in_base_units").message}</caption>
        <thead><tr><th scope="col">{copyEntry("explorer.copy.asset").message}</th><th scope="col">{copyEntry("explorer.copy.total").message}</th></tr></thead>
        <tbody>{account.balances.map((balance) => (
          <tr key={balance.asset.id}><th scope="row">{balance.asset.denom}</th><td>{balance.total}</td></tr>
        ))}</tbody>
      </table>
      <table>
        <caption>{copyEntry("explorer.copy.committed_transactions").message}</caption>
        <thead><tr><th scope="col">{copyEntry("explorer.copy.transaction").message}</th><th scope="col">{copyEntry("explorer.copy.block").message}</th><th scope="col">{copyEntry("explorer.copy.ordinal").message}</th><th scope="col">{copyEntry("explorer.copy.status").message}</th><th scope="col">{copyEntry("explorer.copy.result").message}</th><th scope="col">{copyEntry("explorer.copy.amount_in_base_units").message}</th><th scope="col">{copyEntry("explorer.copy.counterparty").message}</th><th scope="col">{copyEntry("explorer.copy.time").message}</th></tr></thead>
        <tbody>{account.activity.map((item) => (
          <tr key={item.hash}>
            <th scope="row">{item.hash}</th><td>{item.block_number}</td><td>{item.ordinal}</td>
            <td>{item.status}</td><td>{item.result ?? "Unavailable"}</td><td>{item.amount} {item.asset.denom}</td>
            <td>{item.counterparty ?? "Unavailable"}</td><td><time dateTime={item.timestamp}>{item.timestamp}</time></td>
          </tr>
        ))}</tbody>
      </table>
      <p>{copyEntry("explorer.account.page").message} {account.page_number}{account.activity_total === null ? "" : formatCopy("explorer.account.transaction_total", { count: account.activity_total })}</p>
      <nav aria-label={copyEntry("explorer.copy.account_activity_pages").message}>
        <a href={model.refreshHref}>{copyEntry("explorer.copy.refresh_this_snapshot").message}</a>{" "}
        <a href={model.firstHref}>{copyEntry("explorer.copy.first_page_of_this_snapshot").message}</a>{" "}
        {model.nextHref === undefined ? null : <a href={model.nextHref}>{copyEntry("explorer.copy.older_transactions").message}</a>}
      </nav>
    </article>
  );
}
