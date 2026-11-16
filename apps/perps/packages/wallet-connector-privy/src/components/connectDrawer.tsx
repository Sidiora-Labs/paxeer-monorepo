import { SimpleSheet } from "../../../ui-connector/src/platformOverlays";
import React, { useCallback, useMemo } from "react";
import { useLocalStorage } from "@paxeer-network/hooks";
import { useTranslation, Trans } from "@paxeer-network/i18n";
import { ConnectorKey } from "@paxeer-network/types";
import {
  useScreen,
  cn,
  Flex,
  Text,
  modal,
  CloseSquareFillIcon,
} from "@paxeer-network/ui";
import { useWalletConnectorPrivy } from "../provider";
import { useAbstractWallet } from "../providers/abstractWallet/abstractWalletProvider";
import { usePrivyWallet } from "../providers/privy/privyWalletProvider";
import { useSolanaWallet } from "../providers/solana/solanaWalletProvider";
import { useWagmiWallet } from "../providers/wagmi/wagmiWalletProvider";
import { WalletConnectType } from "../types";

import { ArrowRightLinearGradientIcon } from "./icons";
import { PwaDialog } from "./pwaDilaog";
import { RenderConnector } from "./renderConnector";
import { RenderNonPrivyWallet } from "./renderNonPrivyWallet";
import { RenderPrivyWallet } from "./renderPrivyWallet";

function MyWallet() {
  const [connectorKey, setConnectorKey] = useLocalStorage(ConnectorKey, "");

  return (
    <div>
      {connectorKey === "privy" && <RenderPrivyWallet />}

      {connectorKey !== "privy" && <RenderNonPrivyWallet />}
    </div>
  );
}

export function ConnectDrawer(props: {
  open: boolean;
  onChangeOpen: (open: boolean) => void;
  headerProps?: {
    mobile: React.ReactNode;
  };
}) {
  const { t } = useTranslation();
  const { isConnected: isConnectedPrivy } = usePrivyWallet();
  const { isConnected: isConnectedEvm } = useWagmiWallet();
  const { isConnected: isConnectedSolana } = useSolanaWallet();
  const { isConnected: isConnectedAbstract } = useAbstractWallet();
  const { termsOfUse } = useWalletConnectorPrivy();
  const [connectorKey] = useLocalStorage(ConnectorKey, "");

  const isConnected = useMemo(() => {
    if (connectorKey === WalletConnectType.PRIVY && isConnectedPrivy) {
      return true;
    }
    if (connectorKey !== WalletConnectType.PRIVY) {
      if (isConnectedEvm) {
        return true;
      }
      if (isConnectedSolana) {
        return true;
      }
      if (isConnectedAbstract) {
        return true;
      }
    }
    return false;
  }, [
    isConnectedPrivy,
    isConnectedEvm,
    isConnectedSolana,
    isConnectedAbstract,
    connectorKey,
  ]);

  const { isMobile } = useScreen();

  const showPwaDialog = () => {
    modal.show(PwaDialog);
  };

  const renderHeader = useCallback(() => {
    return (
      <div
        className={cn(
          "oui-font-semibold oui-text-base-contrast-80 ",
          "oui-pb-2 oui-text-xl",
          "md:oui-py-0 md:oui-text-base",
        )}
      >
        {isConnected
          ? t("connector.privy.myWallet")
          : t("connector.connectWallet")}
      </div>
    );
  }, [isConnected, t]);
  return (
    <SimpleSheet
      title={
        isConnected
          ? t("connector.privy.myWallet")
          : t("connector.connectWallet")
      }
      open={props.open}
      onOpenChange={props.onChangeOpen}
      classNames={{
        body: "oui-h-full oui-py-0  oui-border-none oui-relative",
        // overlay: "!oui-bg-base-10/60",
        content: cn(
          "oui-shell-overlay !oui-p-4  !oui-bg-base-9 !oui-border !oui-border-solid !oui-border-line-12",

          isMobile
            ? "oui-inset-y-0 oui-end-0 oui-w-72 oui-rounded-none !oui-bg-base-8"
            : "!oui-bottom-8 oui-end-3 oui-top-12 !oui-h-auto oui-w-80 oui-overflow-hidden oui-rounded-2xl !oui-bg-base-9 ",
        ),
      }}
      contentProps={{
        side: "right",
        closeable: false,
      }}
    >
      {!isMobile && (
        <div
          className="oui-absolute oui-inset-x-12 -oui-top-[calc(100dvh/2)] oui-z-0 oui-h-dvh"
          style={{
            background:
              "radial-gradient(ellipse, rgb(var(--oui-gradient-brand-start) / 0.2), transparent)",
            filter: "blur(var(--oui-space-12))",
          }}
        />
      )}
      <div className="oui-relative oui-z-10 oui-flex oui-h-full oui-flex-col oui-gap-4 md:oui-gap-5">
        <div className="oui-flex oui-flex-none oui-items-center oui-justify-between">
          {renderHeader()}
          <button
            type="button"
            className="oui-shell-icon-button"
            aria-label="Close"
            onClick={() => props.onChangeOpen(false)}
          >
            <CloseSquareFillIcon className="oui-size-5 oui-cursor-pointer oui-text-base-contrast-80 hover:oui-text-base-contrast-80" />
          </button>
        </div>
        {isConnected ? <MyWallet /> : <RenderConnector />}

        {!isConnected && (
          <Flex gap={4} direction="column">
            {isMobile && (
              <Flex
                asChild
                itemAlign="center"
                justify="between"
                gap={4}
                className="oui-rounded-lg oui-px-3 oui-py-2 oui-text-2xs oui-bg-[linear-gradient(270deg,rgb(var(--oui-gradient-brand-end)_/_0.10)_0%,rgb(var(--oui-gradient-brand-start)_/_0.10)_100%)] oui-w-full"
                onClick={() => showPwaDialog()}
              >
                <button type="button" className="oui-shell-action">
                  <Text className="oui-bg-[linear-gradient(270deg,rgb(var(--oui-gradient-brand-end))_0%,rgb(var(--oui-gradient-brand-start))_100%)] oui-bg-clip-text oui-text-transparent">
                    {t("connector.privy.pwa.title")}
                  </Text>
                  <ArrowRightLinearGradientIcon
                    size={20}
                    className="oui-flex-shrink-0"
                  />
                </button>
              </Flex>
            )}
            {termsOfUse && (
              <div className="oui-flex-none oui-text-center oui-text-2xs oui-font-semibold  oui-text-base-contrast-80">
                {/* @ts-ignore */}
                <Trans
                  i18nKey="connector.privy.termsOfUse"
                  components={[
                    <a
                      key="termsOfUse"
                      href={termsOfUse}
                      className="oui-cursor-pointer oui-text-primary oui-underline"
                      target="_blank"
                      rel="noreferrer"
                    />,
                  ]}
                />
              </div>
            )}
          </Flex>
        )}
      </div>
    </SimpleSheet>
  );
}
