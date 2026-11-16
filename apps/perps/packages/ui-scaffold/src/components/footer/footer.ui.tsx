import { FC, useMemo } from "react";
import { WsNetworkStatus } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { Divider, Flex, Text } from "@paxeer-network/ui";
import {
  CommuntiyDiscordIcon,
  CommuntiyTelegramIcon,
  CommuntiyXIcon,
  PaxeerNetworkTextIcon,
  SignalIcon,
} from "../icons/index";
import type { FooterReturns } from "./footer.script";
import type { FooterProps } from "./footer.widget";

export const Footer: FC<FooterReturns & FooterProps> = (props) => {
  const { t } = useTranslation();

  const signalClsName = useMemo(() => {
    switch (props.wsStatus) {
      case WsNetworkStatus.Connected:
        return "oui-fill-success-light oui-text-success-light";
      case WsNetworkStatus.Disconnected:
        return "oui-fill-danger-light oui-text-danger-light";
      case WsNetworkStatus.Unstable:
        return "oui-fill-warning-light oui-text-warning-light";
    }
  }, [props.wsStatus]);

  const poweredBy =
    props.poweredBy === true ? (
      <>
        <Text intensity={80} size="2xs">
          {t("scaffold.footer.poweredBy")}
        </Text>
        <PaxeerNetworkTextIcon />
      </>
    ) : (
      props.poweredBy
    );

  return (
    <Flex
      direction={"row"}
      justify={"between"}
      className="oui-min-h-12 oui-flex-wrap oui-gap-2"
      px={3}
      width={"100%"}
    >
      <Flex className="oui-flex-wrap oui-gap-2">
        <Flex
          direction={"row"}
          itemAlign={"center"}
          gap={1}
          className={signalClsName}
        >
          <SignalIcon
            // className={cn(signalClsName, "oui-fill-success-light")}
            fillOpacity={1}
            fill="currentColor"
          />
          <Text size="2xs">{t("scaffold.footer.operational")}</Text>
        </Flex>
        <Divider
          direction="vertical"
          className="oui-h-5 oui-px-1 oui-ms-2 oui-border-line-12"
        />
        <Flex gap={2} className="oui-flex-wrap">
          <Text intensity={80} size="2xs">
            {t("scaffold.footer.joinCommunity")}
          </Text>
          <Flex direction={"row"} gap={1}>
            {typeof props.telegramUrl !== "undefined" && (
              <a
                className="oui-shell-icon-button"
                aria-label="Telegram"
                href={props.telegramUrl}
                target="_blank"
                rel="noopener noreferrer"
              >
                <CommuntiyTelegramIcon
                  className="oui-fill-base-contrast-54 hover:oui-fill-base-contrast oui-cursor-pointer"
                  fill="currentColor"
                  fillOpacity={1}
                />
              </a>
            )}
            {typeof props.discordUrl !== "undefined" && (
              <a
                className="oui-shell-icon-button"
                aria-label="Discord"
                href={props.discordUrl}
                target="_blank"
                rel="noopener noreferrer"
              >
                <CommuntiyDiscordIcon
                  className="oui-fill-base-contrast-54 hover:oui-fill-base-contrast oui-cursor-pointer"
                  fill="currentColor"
                  fillOpacity={1}
                />
              </a>
            )}
            {typeof props.twitterUrl !== "undefined" && (
              <a
                className="oui-shell-icon-button"
                aria-label="X"
                href={props.twitterUrl}
                target="_blank"
                rel="noopener noreferrer"
              >
                <CommuntiyXIcon
                  className="oui-fill-base-contrast-54 hover:oui-fill-base-contrast oui-cursor-pointer"
                  fill="currentColor"
                  fillOpacity={1}
                />
              </a>
            )}
          </Flex>
          {typeof props?.trailing !== "undefined" && (
            <>
              <Divider
                direction="vertical"
                className="oui-h-5 oui-border-line-12"
              />
              {props?.trailing}
            </>
          )}
        </Flex>
      </Flex>
      {poweredBy && (
        <Flex
          direction={"row"}
          gap={1}
          className="oui-scaffold-footer-powered-by"
        >
          {poweredBy}
        </Flex>
      )}
    </Flex>
  );
};
