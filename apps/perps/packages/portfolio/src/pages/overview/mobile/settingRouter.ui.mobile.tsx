import { FC } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { RouterAdapter } from "@paxeer-network/types";
import {
  Flex,
  Text,
  SettingFillIcon,
  ChevronRightIcon,
} from "@paxeer-network/ui";
import { PortfolioLeftSidebarPath } from "../../../layout";

type Props = {
  routerAdapter?: RouterAdapter;
};

export const SettingRouterMobile: FC<Props> = (props) => {
  const { t } = useTranslation();
  return (
    <Flex
      asChild
      direction={"row"}
      width={"100%"}
      className="oui-cursor-pointer oui-items-center oui-gap-2 oui-rounded-xl oui-bg-base-9 oui-p-4 oui-mb-3"
    >
      <button
        type="button"
        onClick={() =>
          props?.routerAdapter?.onRouteChange({
            href: PortfolioLeftSidebarPath.Setting,
            name: t("portfolio.setting"),
          })
        }
        className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
      >
        <SettingFillIcon size={18} opacity={0.98} color="inherit" />
        <Text className="oui-text-base oui-font-semibold oui-text-base-contrast-80">
          {t("portfolio.setting")}
        </Text>
        <ChevronRightIcon
          className="oui-ms-auto"
          size={18}
          opacity={0.36}
          color="inherit"
        />
      </button>
    </Flex>
  );
};
