import React, { useMemo } from "react";
import { useStorageChain } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { AbstractChains, SolanaChains } from "@paxeer-network/types";
import {
  ExclamationFillIcon,
  Flex,
  modal,
  useScreen,
} from "@paxeer-network/ui";
import { useWalletConnectorPrivy } from "../provider";
import { WalletType } from "../types";
import { ArrowRightIcon } from "./icons";

export function SwitchNetworkTips({
  tipsContent,
}: {
  tipsContent: string | null;
}) {
  const { t } = useTranslation();
  const { isMobile } = useScreen();
  const { network, setOpenConnectDrawer } = useWalletConnectorPrivy();
  if (!tipsContent) {
    return null;
  }

  const onSwitchNetwork = () => {
    setOpenConnectDrawer(false);
    modal
      .show<{
        wrongNetwork: boolean;
      }>(isMobile ? "ChainSelectorSheetId" : "ChainSelectorDialogId", {
        networkId: network,
        bridgeLessOnly: false,
      })
      .then(
        (r) => {
          console.log("[switchChain success]", r);
        },
        (error) => console.log("[switchChain error]", error),
      );
  };
  return (
    <button
      type="button"
      onClick={onSwitchNetwork}
      className="oui-mb-3 oui-flex oui-cursor-pointer oui-items-center oui-justify-between  oui-gap-1 oui-rounded-lg oui-bg-[rgb(var(--oui-color-warning-darken)_/_0.1)] oui-px-2 oui-py-2 "
    >
      <Flex gap={1}>
        <ExclamationFillIcon
          size={14}
          className=" oui-shrink-0 oui-text-base-contrast-80"
        />
        <div className="oui-text-2xs oui-text-base-contrast-80">
          {t("connector.privy.switchNetwork.tips", {
            chainName: tipsContent,
          })}
        </div>
      </Flex>
      <ArrowRightIcon size={14} className="oui-text-base-contrast-80" />
    </button>
  );
}
export const StorageChainNotCurrentWalletType = ({
  currentWalletType,
}: {
  currentWalletType: Set<WalletType>;
}) => {
  const { storageChain } = useStorageChain();
  const tipsContent = useMemo(() => {
    if (!currentWalletType || currentWalletType.size === 0 || !storageChain) {
      return null;
    }
    let text = null;
    const isSolana = SolanaChains.has(parseInt(storageChain?.chainId));
    const isAbstract = AbstractChains.has(parseInt(storageChain?.chainId));
    const isEvm = !isSolana && !isAbstract;

    if (isSolana && currentWalletType.has(WalletType.SOL)) {
      return null;
    } else if (isAbstract && currentWalletType.has(WalletType.ABSTRACT)) {
      return null;
    } else if (isEvm && currentWalletType.has(WalletType.EVM)) {
      return null;
    }

    text = Array.from(currentWalletType)
      .map((item) => {
        if (item === WalletType.EVM) {
          return "Evm";
        } else if (item === WalletType.SOL) {
          return "Solana";
        } else if (item === WalletType.ABSTRACT) {
          return "Abstract";
        }
      })
      .join("/");
    return text;
  }, [storageChain, currentWalletType]);

  return <SwitchNetworkTips tipsContent={tipsContent} />;
};
