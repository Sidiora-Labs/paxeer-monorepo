import { registerAffiliateDialog } from "../../../../../components/dialog";
import { RefereeDescriptionFormWidget } from "./refereeDescriptionForm.widget";

export const RefereeDescriptionFormDialogId = "RefereeDescriptionFormDialogId";

registerAffiliateDialog(
  RefereeDescriptionFormDialogId,
  RefereeDescriptionFormWidget,
  {
    size: "sm",
    classNames: {
      content:
        "oui-border oui-border-line-6 !oui-max-w-[calc(var(--oui-space-24)*4)] !oui-px-0",
      body: "!oui-py-0",
    },
  },
);
