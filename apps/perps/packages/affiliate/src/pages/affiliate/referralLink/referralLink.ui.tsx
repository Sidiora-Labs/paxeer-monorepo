import { InfoPopover } from "../../../components/infoPopover";
import surface from "../../../surface.module.css";
import { FC } from "react";
import { Trans, useTranslation } from "@paxeer-network/i18n";
import {
  CopyIcon,
  Divider,
  Flex,
  modal,
  Text,
  Tooltip,
} from "@paxeer-network/ui";
import { AutoHideText } from "../../../components/autoHideText";
import { ReferralLinkReturns } from "./referralLink.script";

export const ReferralLink: FC<ReferralLinkReturns> = (props) => {
  const { t } = useTranslation();

  return (
    <Flex
      id="oui-affiliate-affiliate-referralLink"
      r={"2xl"}
      p={6}
      width={"100%"}
      gap={4}
      direction={"column"}
      className={`${surface.surface} oui-bg-base-9`}
    >
      <Title {...props} />
      <Flex
        width={"100%"}
        className="oui-flex oui-flex-col 3xl:oui-flex-row"
        gap={4}
      >
        <div className="oui-w-full 3xl:oui-w-1/3">
          <Subtitle {...props} />
        </div>
        <Flex
          direction={"column"}
          width={"100%"}
          className="3xl:oui-w-2/3 3xl:oui-gap-2"
        >
          <Input
            title={t("affiliate.referralCode")}
            value={props.refCode}
            {...props}
          />
          <Input
            title={t("affiliate.referralLink")}
            value={props.refLink}
            {...props}
          />
        </Flex>
      </Flex>
    </Flex>
  );
};

const Title: FC<ReferralLinkReturns> = (props) => {
  const { t } = useTranslation();
  return (
    <Flex direction={"row"} justify={"between"} width={"100%"}>
      <Text className="oui-text-lg">{t("affiliate.referralLink")}</Text>
    </Flex>
  );
};

const Subtitle: FC<ReferralLinkReturns> = (props) => {
  const { t } = useTranslation();

  const item = (
    title: string,
    value: string,
    gradient: boolean,
    className?: string,
    tooltip?: any,
  ) => {
    const valueClsName =
      "oui-text-lg md:oui-text-xl lg:oui-text-2xl xl:oui-text-3xl";
    return (
      <Flex direction={"column"} itemAlign={"start"} className={className}>
        <Flex direction={"row"} gap={2}>
          {/* <Text>{title}</Text> */}
          <Alert title={title} tooltip={tooltip} />
        </Flex>
        {gradient ? (
          <Text.gradient color="brand" className={valueClsName}>
            {value}
          </Text.gradient>
        ) : (
          <Text className={valueClsName}>{value}</Text>
        )}
      </Flex>
    );
  };

  return (
    <Flex direction={"row"} width={"100%"}>
      {item(
        t("affiliate.referralLink.earn"),
        props.earn || "-",
        true,
        "oui-flex-1",
        <span>
          <Text.gradient color="brand">{props.earn || "-"}</Text.gradient>{" "}
          {t("affiliate.referralLink.earn.tooltip", {
            brokerName: props.brokerName,
          })}
        </span>,
        // @ts-ignore
        // <Trans
        //   i18nKey="affiliate.referralLink.earn.tooltip"
        //   values={{
        //     value: props.earn || "-",
        //     brokerName: props.brokerName,
        //   }}
        //   components={[<Text.gradient color="brand" />]}
        // />
      )}
      {item(
        t("affiliate.referralLink.share"),
        props.share || "-",
        false,
        "oui-flex-1",
        // @ts-ignore
        <Trans
          i18nKey="affiliate.referralLink.share.tooltip"
          values={{
            value: props.share || "-",
            brokerName: props.brokerName,
          }}
          components={[<Text.gradient color="brand" key="0" />]}
        />,
      )}
    </Flex>
  );
};

const Input: FC<
  ReferralLinkReturns & {
    title: string;
    value?: string;
  }
> = (props) => {
  const { title, value = "-" } = props;
  const { t } = useTranslation();
  return (
    <Flex
      r="xl"
      p={3}
      gap={4}
      direction={"row"}
      width={"100%"}
      className="oui-bg-base-8"
    >
      <Text className="oui-text-base-contrast-54 oui-flex-shrink oui-text-2xs md:oui-text-xs xl:oui-text-sm">
        {title}
      </Text>
      <Flex
        direction={"row"}
        justify={"end"}
        gap={2}
        className="oui-flex-1 oui-text-xs md:oui-text-sm xl:oui-text-base"
      >
        <AutoHideText text={value} />
        <button
          type="button"
          aria-label={`${t("common.copy")} ${value}`}
          className="oui-cursor-pointer oui-text-sm oui-flex-shrink"
          onClick={() => {
            navigator.clipboard.writeText(value);
            props.onCopy?.(value);
          }}
        >
          <CopyIcon
            size={12}
            color="white"
            className="oui-w-3 oui-h-3 md:oui-w-3 md:oui-h-3 xl:oui-w-4 xl:oui-h-4"
          />
        </button>
      </Flex>
    </Flex>
  );
};

const Alert: FC<{
  title: string;
  tooltip: any;
}> = (props) => {
  return (
    <InfoPopover content={props.tooltip} title={props.title}>
      <span className="oui-text-2xs md:oui-text-xs xl:oui-text-sm">
        {props.title}
      </span>
    </InfoPopover>
  );
};
