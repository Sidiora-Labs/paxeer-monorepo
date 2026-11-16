import surface from "../../../../../surface.module.css";
import { InfoPopover } from "../../../../../components/infoPopover";
import { FC, ReactNode } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Tooltip,
  formatAddress,
  Text,
  Flex,
  Grid,
  cn,
  useScreen,
  modal,
} from "@paxeer-network/ui";
import { commifyOptional } from "@paxeer-network/utils";

export const AddressCell: FC<{
  address: string;
  title?: ReactNode;
}> = ({ address, title }) => {
  return (
    <InfoPopover content={address} title={title}>
      {formatAddress(address)}
    </InfoPopover>
  );
};

export const TooltipCell: FC<{
  text: string;
  tooltip: string | ReactNode;
  title?: ReactNode;
}> = ({ text, tooltip, title }) => {
  return (
    <InfoPopover content={tooltip} title={title}>
      {text}
    </InfoPopover>
  );
};

export const BreakdownCell: FC<{
  total: number;
  direct: number;
  indirect: number;
  directBonus?: number;
  prefix?: string;
  fix?: number;
  title?: ReactNode;
}> = ({ total, direct, indirect, directBonus, prefix, fix = 0, title }) => {
  const { t } = useTranslation();
  const format = (val: number) =>
    commifyOptional(val, {
      fix,
      fallback: "0",
      padEnd: true,
      prefix,
    });

  const directLine = directBonus ? (
    <>
      • {t("affiliate.direct")}: {format(direct + directBonus)}
    </>
  ) : (
    <>
      • {t("affiliate.direct")}: {format(direct)}
    </>
  );

  const tooltipContent = (
    <div className="oui-flex oui-flex-col oui-gap-1">
      <div>{directLine}</div>
      <div>
        • {t("affiliate.indirect")}: {format(indirect)}
      </div>
    </div>
  );

  return (
    <InfoPopover content={tooltipContent} title={title}>
      {format(total)}
    </InfoPopover>
  );
};

export const MobileCell: FC<{
  label: string;
  children: ReactNode;
  align?: "start" | "end";
  className?: string;
}> = ({ label, children, align = "start", className }) => {
  return (
    <Flex
      direction="column"
      itemAlign={align}
      className={cn(
        "oui-gap-1.5 oui-font-medium oui-tracking-normal",
        className,
      )}
    >
      <Text intensity={36} size="2xs" className="oui-leading-normal">
        {label}
      </Text>
      <div className="oui-text-sm oui-leading-normal">{children}</div>
    </Flex>
  );
};

export const MobileCard: FC<{
  children: ReactNode;
  className?: string;
}> = ({ children, className }) => {
  return (
    <Grid
      cols={3}
      py={3}
      r="xl"
      width="100%"
      className={cn(surface.mobileCard, "oui-bg-base-9", className)}
    >
      {children}
    </Grid>
  );
};
