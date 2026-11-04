import type { NativeAccountPageModel } from "./native-page.ts";

export function NativeAccountPage({ model }: Readonly<{ model: NativeAccountPageModel }>) {
  if (model.state === "cursor-refused") {
    return (
      <section aria-label="Account snapshot refused" data-state={model.state}>
        <p>The account snapshot cannot be continued: {model.reason}.</p>
        <a href={model.resetHref}>Start a new account snapshot</a>
      </section>
    );
  }
  if (model.state === "not-found") return <p data-state={model.state}>Account not found.</p>;
  if (model.state === "unavailable") return <p data-state={model.state}>The account explorer is unavailable.</p>;
  const account = model.account;
  return (
    <article aria-label="Native account" data-state="ready">
      <h1>{account.identities.evm}</h1>
      <dl aria-label="Account identities">
        <dt>EVM address</dt><dd>{account.identities.evm}</dd>
        <dt>Paxeer address</dt><dd>{account.identities.pax ?? "Unavailable"}</dd>
        <dt>DID</dt><dd>{account.identities.did ?? "Unavailable"}</dd>
      </dl>
      <table>
        <caption>Account totals in base units</caption>
        <thead><tr><th scope="col">Asset</th><th scope="col">Total</th></tr></thead>
        <tbody>{account.balances.map((balance) => (
          <tr key={balance.asset.id}><th scope="row">{balance.asset.denom}</th><td>{balance.total}</td></tr>
        ))}</tbody>
      </table>
      <table>
        <caption>Committed transactions</caption>
        <thead><tr><th scope="col">Transaction</th><th scope="col">Block</th><th scope="col">Ordinal</th><th scope="col">Status</th><th scope="col">Result</th><th scope="col">Amount in base units</th><th scope="col">Counterparty</th><th scope="col">Time</th></tr></thead>
        <tbody>{account.activity.map((item) => (
          <tr key={item.hash}>
            <th scope="row">{item.hash}</th><td>{item.block_number}</td><td>{item.ordinal}</td>
            <td>{item.status}</td><td>{item.result ?? "Unavailable"}</td><td>{item.amount} {item.asset.denom}</td>
            <td>{item.counterparty ?? "Unavailable"}</td><td><time dateTime={item.timestamp}>{item.timestamp}</time></td>
          </tr>
        ))}</tbody>
      </table>
      <p>Page {account.page_number}{account.activity_total === null ? "" : `; ${account.activity_total} transactions`}</p>
      <nav aria-label="Account activity pages">
        <a href={model.refreshHref}>Refresh this snapshot</a>{" "}
        <a href={model.firstHref}>First page of this snapshot</a>{" "}
        {model.nextHref === undefined ? null : <a href={model.nextHref}>Older transactions</a>}
      </nav>
    </article>
  );
}
