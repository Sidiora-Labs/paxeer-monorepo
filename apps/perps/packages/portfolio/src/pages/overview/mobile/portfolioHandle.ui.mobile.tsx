import { FC } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { RouterAdapter } from "@paxeer-network/types";
import {
  Flex,
  Text,
  ArrowDownSquareFillIcon,
  ArrowUpSquareFillIcon,
  CalendarMinusIcon,
  ArrowLeftRightIcon,
} from "@paxeer-network/ui";
import { PortfolioLeftSidebarPath } from "../../../layout";

type Props = {
  disabled: boolean;
  onWithdraw?: () => void;
  onDeposit?: () => void;
  onTransfer?: () => void;
  isMainAccount?: boolean;
  routerAdapter?: RouterAdapter;
  hasSubAccount?: boolean;
};

export const PortfolioHandleMobile: FC<Props> = (props) => {
  const { t } = useTranslation();

  const onGotoHistory = () => {
    props.routerAdapter?.onRouteChange({
      href: PortfolioLeftSidebarPath.History,
      name: t("trading.history"),
    });
  };

  return (
    <Flex
      direction={"row"}
      width={"100%"}
      className="oui-flex-wrap oui-gap-3 oui-bg-transparent"
    >
      {props.isMainAccount && (
        <Flex
          asChild
          direction="column"
          gapY={2}
          itemAlign={"center"}
          className="oui-min-w-24 oui-flex-1 oui-cursor-pointer"
        >
          <button
            type="button"
            onClick={props?.onDeposit}
            className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
          >
            <div className="oui-flex oui-size-12 oui-items-center oui-justify-center oui-rounded-xl oui-bg-base-9">
              <ArrowDownSquareFillIcon size={28} color="inherit" opacity={1} />
            </div>
            <Text className="oui-text-base-80 oui-text-2xs">
              {t("common.deposit")}
            </Text>
          </button>
        </Flex>
      )}
      {props.hasSubAccount && (
        <Flex
          asChild
          direction="column"
          gapY={2}
          itemAlign={"center"}
          className="oui-min-w-24 oui-flex-1 oui-cursor-pointer"
        >
          <button
            type="button"
            onClick={props?.onTransfer}
            className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
          >
            <div className="oui-flex oui-size-12 oui-items-center oui-justify-center oui-rounded-xl oui-bg-base-9">
              <ArrowLeftRightIcon size={28} color="inherit" opacity={1} />
            </div>
            <Text className="oui-text-base-80 oui-text-2xs">
              {t("common.transfer")}
            </Text>
          </button>
        </Flex>
      )}
      {props.isMainAccount && (
        <Flex
          asChild
          direction="column"
          gapY={2}
          itemAlign={"center"}
          className="oui-min-w-24 oui-flex-1 oui-cursor-pointer"
        >
          <button
            type="button"
            onClick={props?.onWithdraw}
            className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
          >
            <div className="oui-flex oui-size-12 oui-items-center oui-justify-center oui-rounded-xl oui-bg-base-9">
              <ArrowUpSquareFillIcon size={28} color="inherit" opacity={1} />
            </div>
            <Text className="oui-text-base-80 oui-text-2xs">
              {t("common.withdraw")}
            </Text>
          </button>
        </Flex>
      )}
      <Flex
        direction="column"
        gapY={2}
        itemAlign={"center"}
        className="oui-min-w-24 oui-flex-1 oui-cursor-pointer"
      >
        <button
          type="button"
          onClick={onGotoHistory}
          aria-label={t("trading.history")}
          className={[
            "oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed",
            "oui-flex oui-size-12 oui-items-center oui-justify-center oui-rounded-xl oui-bg-base-9",
          ].join(" ")}
        >
          <CalendarMinusIcon
            size={28}
            color="inherit"
            opacity={1}
            viewBox="0 0 28 28"
            className="oui-w-7 oui-h-7"
          />
        </button>
        <Text className="oui-text-base-80 oui-text-2xs">
          {t("trading.history")}
        </Text>
      </Flex>
    </Flex>
  );
};
