import { copyEntry } from "../../../copy/runtime";
import { formatCopy } from "../../../copy/format";
import { isPrecompileSendOutcome, type PrecompileSendOutcome } from "../../api/wallet";
import { InlineNotice } from "../../kit/surface";

export function SendOutcome({ outcome }: Readonly<{ outcome: PrecompileSendOutcome | undefined }>) {
  if (outcome === undefined) {
    return null;
  }
  if (!isPrecompileSendOutcome(outcome)) {
    return <InlineNotice tone="danger" role="alert">{copyEntry("markets.copy.the_wallet_returned_a_malformed_result_transaction_s").message}</InlineNotice>;
  }
  const receiptLine = "receipt" in outcome && outcome.receipt !== undefined ? (
    <p>
      {copyEntry("markets.send.receipt").message
        .replace("{blockNumber}", outcome.receipt.blockNumber.toString())
        .replace("{logCount}", outcome.receipt.logs.length.toString())}
    </p>
  ) : null;
  switch (outcome.outcome) {
    case "sent":
      if (outcome.receipt === undefined) {
        return (
          <InlineNotice tone="warning">
            Submitted. Awaiting confirmation. Transaction {outcome.transactionHash}. Check its status before sending again.
          </InlineNotice>
        );
      }
      return (
        <InlineNotice tone="success">
          <p>{formatCopy("markets.send.sent_transaction", { transactionHash: outcome.transactionHash })}</p>
          {receiptLine}
        </InlineNotice>
      );
    case "unconfirmed":
      return (
        <InlineNotice tone="warning">
          Submitted, but confirmation is unavailable. Transaction {outcome.transactionHash}. Check its status before sending again.
        </InlineNotice>
      );
    case "reverted":
      return (
        <InlineNotice tone="danger">
          <p>{copyEntry("markets.send.reverted").message.replace("{transactionHash}", outcome.transactionHash)}</p>
          {receiptLine}
        </InlineNotice>
      );
    case "cancelled":
      return <InlineNotice tone="neutral">{copyEntry("markets.copy.you_cancelled_in_the_wallet").message}</InlineNotice>;
    case "rejected":
      return <InlineNotice tone="danger">{copyEntry("markets.copy.the_wallet_refused_this_account").message}</InlineNotice>;
    case "unavailable":
      return <InlineNotice tone="warning">{copyEntry("markets.copy.no_paxeer_wallet_is_connected").message}</InlineNotice>;
    case "failed":
      return (
        <InlineNotice tone="danger" role="alert">
          The transaction was not sent{outcome.detail === undefined ? "." : `: ${outcome.detail}`}
        </InlineNotice>
      );
  }
}
