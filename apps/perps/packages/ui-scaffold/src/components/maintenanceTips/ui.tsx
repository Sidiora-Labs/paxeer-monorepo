import { SimpleDialog } from "../../../../ui-connector/src/platformOverlays";
import { useTranslation } from "@paxeer-network/i18n";
import { Box, cn, Flex } from "@paxeer-network/ui";
import { CloseIcon, InfoIcon } from "../icons";
import { MaintenanceTipInterface } from "./script";

export const MaintenanceTipsUI = (props: MaintenanceTipInterface) => {
  const { showTips, showDialog, tipsContent, closeTips, dialogContent } = props;
  const { t } = useTranslation();

  if (showDialog) {
    return (
      <SimpleDialog
        open
        closable={false}
        title={t("maintenance.dialog.title")}
        classNames={{ content: "oui-w-80 md:oui-w-auto" }}
      >
        {dialogContent}
      </SimpleDialog>
    );
  }
  if (!showTips) {
    return <></>;
  }
  return (
    <Flex
      className={cn(
        "oui-bg-warning-darken/10 oui-text-base-contrast-80 oui-text-2xs md:oui-text-sm oui-relative oui-gap-1",
        "oui-mx-1 md:oui-mx-0 md:oui-w-full oui-ps-2 md:oui-ps-4 oui-pe-9 md:oui-pe-16 oui-py-2 md:oui-py-3",
        "oui-leading-normal",
        "oui-rounded-xl md:oui-rounded-none",
        "oui-min-h-12",
        "oui-justify-start oui-items-start",
        "xl:oui-justify-center xl:oui-items-center xl:oui-px-4",
      )}
    >
      <div className="oui-flex oui-items-start oui-justify-start oui-gap-1 ">
        <InfoIcon
          size={20}
          className="oui-flex-shrink-0 oui-w-4 md:oui-w-5 oui-h-4 md:oui-h-5 "
        />
        <Box className="oui-font-semibold oui-leading-normal">
          {tipsContent}
        </Box>
      </div>
      <button
        type="button"
        className="oui-shell-icon-button oui-absolute oui-end-2 oui-top-0"
        aria-label="Close"
        onClick={closeTips}
      >
        <CloseIcon size={16} className="oui-text-base-contrast-80" />
      </button>
    </Flex>
  );
};
