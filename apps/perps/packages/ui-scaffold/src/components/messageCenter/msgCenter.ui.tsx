import { SimpleDialog } from "../../../../ui-connector/src/platformOverlays";
import {
  PopoverRoot,
  PopoverTrigger,
  PopoverContent,
  Tooltip,
} from "../../../../ui-connector/src/platformOverlays";
import { FC, useMemo, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { API } from "@paxeer-network/types";
import { BellIcon } from "@paxeer-network/ui";
import { AnnouncementCenterUI } from "@paxeer-network/ui-notification";
import { useScaffoldContext } from "../scaffold";

/**
 * Notify bell with Popover (click) + Tooltip (hover) coexisting.
 * - We control Popover open state; when Popover is open, we disable Tooltip to prevent hover/click conflicts.
 * - Both triggers use `asChild` and stack on the same real DOM element to avoid extra wrappers.
 */
export const MessageCenter: FC<{
  maintenanceDialogInfo?: string;
  messages: API.AnnouncementRow[];
  showAnnouncement: boolean;
  closeTips: () => void;
}> = (props) => {
  const { maintenanceDialogInfo, messages, showAnnouncement, closeTips } =
    props;
  const { routerAdapter } = useScaffoldContext();
  const { t } = useTranslation();
  const messageSize = useMemo(() => messages.length, [messages]);
  // Track Popover open to suppress Tooltip while the popover is visible
  const [popoverOpen, setPopoverOpen] = useState(false);

  const onItemClick = (url: string) => {
    if (!url) return;
    routerAdapter?.onRouteChange({
      href: url,
      name: url,
      target: "_blank",
    });
  };

  return (
    <>
      {!!maintenanceDialogInfo && (
        <SimpleDialog
          open
          closable={false}
          title={t("maintenance.dialog.title")}
          classNames={{ content: "oui-w-80 lg:oui-w-auto" }}
        >
          {maintenanceDialogInfo}
        </SimpleDialog>
      )}
      <PopoverRoot
        open={popoverOpen}
        onOpenChange={(open) => {
          setPopoverOpen(open);
          if (open) {
            closeTips();
          }
        }}
      >
        <Tooltip
          open={popoverOpen ? false : undefined}
          content={t("ui.messageCenter.tooltip")}
          delayDuration={250}
        >
          <PopoverTrigger asChild>
            <button
              type="button"
              aria-label={t("ui.messageCenter.tooltip")}
              className="oui-relative oui-shell-icon-button"
            >
              <BellIcon
                color="white"
                opacity={0.8}
                size={20}
                viewBox="0 0 20 20"
                className="hover:oui-opacity-100"
              />
              {messageSize > 0 && showAnnouncement && (
                <div className="oui-absolute oui-end-0 oui-top-0 oui-flex oui-size-4 oui-items-center oui-justify-center oui-rounded-full oui-bg-primary-darken oui-text-3xs oui-leading-none oui-text-primary-contrast">
                  {messageSize}
                </div>
              )}
            </button>
          </PopoverTrigger>
        </Tooltip>
        <PopoverContent
          align="end"
          className="oui-shell-overlay oui-w-96 oui-shadow-lg oui-p-0"
        >
          <AnnouncementCenterUI
            dataSource={messages}
            onItemClick={onItemClick}
          />
        </PopoverContent>
      </PopoverRoot>
    </>
  );
};
