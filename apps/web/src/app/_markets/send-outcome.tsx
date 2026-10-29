import type { PrecompileSendOutcome } from "../../api/wallet";
import { InlineNotice } from "../../kit/surface";

export function SendOutcome({ outcome }: Readonly<{ outcome: PrecompileSendOutcome | undefined }>) {
  if (outcome === undefined) {
    return null;
  }
  switch (outcome.outcome) {
    case "sent":
      if (outcome.receipt === undefined) {
        return (
          <InlineNotice tone="warning">
            Submitted. Awaiting confirmation. Transaction {outcome.transactionHash}. Check its status before sending again.
          </InlineNotice>
        );
      }
      return <InlineNotice tone="success">Sent. Transaction {outcome.transactionHash}</InlineNotice>;
    case "unconfirmed":
      return (
        <InlineNotice tone="warning">
          Submitted, but confirmation is unavailable. Transaction {outcome.transactionHash}. Check its status before sending again.
        </InlineNotice>
      );
    case "reverted":
      return <InlineNotice tone="danger">Transaction {outcome.transactionHash} reverted.</InlineNotice>;
    case "cancelled":
      return <InlineNotice tone="neutral">You cancelled in the wallet.</InlineNotice>;
    case "rejected":
      return <InlineNotice tone="danger">The wallet refused this account.</InlineNotice>;
    case "unavailable":
      return <InlineNotice tone="warning">No Paxeer wallet is connected.</InlineNotice>;
    case "failed":
      return (
        <InlineNotice tone="danger" role="alert">
          The transaction was not sent{outcome.detail === undefined ? "." : `: ${outcome.detail}`}
        </InlineNotice>
      );
  }
}
