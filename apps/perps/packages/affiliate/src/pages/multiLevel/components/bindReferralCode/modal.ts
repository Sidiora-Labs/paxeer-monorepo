import { registerAffiliateDialog } from "../../../../components/dialog";
import { BindReferralCodeWidget } from "./bindReferralCode.widget";

export const BindReferralCodeDialogId = "BindReferralCodeDialogId";

registerAffiliateDialog(BindReferralCodeDialogId, BindReferralCodeWidget, {
  size: "sm",
  classNames: {
    content: "oui-border oui-border-line-6",
    body: "!oui-pt-3",
  },
});
