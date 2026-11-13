import { copyEntry } from "../../../../../copy/runtime";
import { archiveReceiptRecord, explorerLink } from "../../../../explorer/client";
import { ExplorerFrame } from "../../../../explorer/components";
import { ExplorerPanel, ExplorerTable } from "../../../../kit/explorer";
import { readNativeReceiptTransaction } from "./native-receipt.ts";

export const dynamic = "force-dynamic";

export default async function ReceiptPage({
  params,
  searchParams,
}: Readonly<{
  params: Promise<{ receiptId: string }>;
  searchParams?: Promise<{ archive?: string }>;
}>) {
  const { receiptId } = await params;
  const archive = (await searchParams)?.archive === "1";
  try {
    if (archive) {
      const receipt = await archiveReceiptRecord(receiptId.replace(/^0x/u, ""));
      if (receipt === undefined) {
        return <ExplorerFrame title={copyEntry("explorer.copy.historical_receipt_archive").message}><p>{copyEntry("explorer.copy.archived_receipt_not_found").message}</p></ExplorerFrame>;
      }
      return (
        <ExplorerFrame title={copyEntry("explorer.copy.historical_receipt_archive").message} description={receipt.id}>
          <ExplorerPanel title={copyEntry("explorer.copy.frozen_historical_evidence").message}>
            <p>{copyEntry("explorer.copy.this_archived_receipt_describes_pre_merge_evidence_i").message}</p>
            <dl>
              <dt>{copyEntry("explorer.copy.origin").message}</dt><dd>{receipt.origin}</dd>
              <dt>{copyEntry("explorer.copy.archive_verification").message}</dt><dd>{receipt.verification_status}</dd>
            </dl>
            <pre className="overflow-x-auto text-xs">{JSON.stringify(receipt.provenance, null, 2)}</pre>
            {receipt.transaction_hash === null ? null : (
              <a href={explorerLink({ kind: "transaction", transactionHash: receipt.transaction_hash })}>{copyEntry("explorer.copy.view_native_transaction").message}</a>
            )}
            <p>{copyEntry("explorer.copy.canonical_receipt_bytes_are_unavailable_this_archive").message}</p>
          </ExplorerPanel>
        </ExplorerFrame>
      );
    }

    const transaction = await readNativeReceiptTransaction(receiptId);
    const archiveHref = `/explorer/receipts/${encodeURIComponent(receiptId)}?archive=1`;
    if (transaction === undefined) {
      return (
        <ExplorerFrame title={copyEntry("explorer.receipt.title").message}>
          <p>{copyEntry("explorer.copy.native_transaction_not_found").message}</p>
          <a href={archiveHref}>{copyEntry("explorer.copy.view_historical_receipt_archive").message}</a>
        </ExplorerFrame>
      );
    }
    const committed = transaction.block_number !== null;
    return (
      <ExplorerFrame title={copyEntry("explorer.receipt.title").message} description={transaction.hash}>
        <ExplorerTable
          caption="Native transaction position"
          columns={[copyEntry("explorer.column.fact").message, copyEntry("explorer.column.value").message]}
          rows={[
            { id: "transaction", cells: ["Native transaction", <a key="transaction" href={explorerLink({ kind: "transaction", transactionHash: transaction.hash })}>{transaction.hash}</a>] },
            { id: "commitment", cells: ["Commitment", committed ? "Committed" : "Pending"] },
            { id: "block", cells: ["Committed block", transaction.block_number ?? "Not committed"] },
            { id: "position", cells: ["Transaction position in block", transaction.position ?? "Not committed"] },
            { id: "result", cells: ["Execution status", transaction.status ?? "Unavailable"] },
          ]}
        />
        <a href={archiveHref}>{copyEntry("explorer.copy.view_historical_receipt_archive").message}</a>
      </ExplorerFrame>
    );
  } catch {
    return (
      <ExplorerFrame title={archive ? "Historical receipt archive" : copyEntry("explorer.receipt.title").message}>
        <p role="status">{archive ? "The historical receipt archive is unavailable." : "The native transaction reader is unavailable."}</p>
      </ExplorerFrame>
    );
  }
}
