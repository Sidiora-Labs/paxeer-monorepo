import { registerAffiliateDialog } from "../../../../components/dialog";
import { TradingFeeRebateWidget } from "./tradingFeeRebate.widget";

export const TradingFeeRebateDialogId = "TradingFeeRebateDialogId";

registerAffiliateDialog(TradingFeeRebateDialogId, TradingFeeRebateWidget, {
  size: "sm",
  closable: true,
  classNames: {
    content: "oui-border oui-border-line-6 !oui-px-5 oui-pt-2",
    body: "!oui-p-0",
  },
});
