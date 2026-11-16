import { registerAffiliateDialog } from "../../../../components/dialog";
import { ReferralCodeFormWidget } from "./referralCodeForm.widget";

export const ReferralCodeFormDialogId = "ReferralCodeFormDialogId";
export const ReferralCodeFormSheetId = "ReferralCodeFormSheetId";

registerAffiliateDialog(ReferralCodeFormDialogId, ReferralCodeFormWidget, {
  size: "sm",
  classNames: {
    content: "oui-border oui-border-line-6",
    body: "!oui-pt-3",
  },
});

registerAffiliateDialog(ReferralCodeFormSheetId, ReferralCodeFormWidget);
