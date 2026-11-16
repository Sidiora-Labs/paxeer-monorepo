import { Tooltip } from "../../../ui-connector/src/platformOverlays";

import React from "react";
import { useWalletConnector } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { ChainNamespace } from "@paxeer-network/types";
import {
  Checkbox,
  cn,
  CopyIcon,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  formatAddress,
  toast,
} from "@paxeer-network/ui";
import { usePrivyWallet } from "../providers/privy/privyWalletProvider";
import { WalletConnectType, WalletType } from "../types";
import { PrivyConnectorImagePath } from "../util";
import { EVMChainPopover } from "./common";
import { DisconnectIcon, MoreIcon } from "./icons";

interface WalletCardProps {
  type: WalletType;
  address: string;
  isActive: boolean;
  isPrivy?: boolean;
  isMulti?: boolean;
  onActiveChange: (active: boolean) => void;
}

const getCardBgClassName = (type: WalletType = WalletType.EVM) => {
  // Wallet cards use the surface roles in both color schemes.
  const cardBgColorMap: { [key in WalletType]: string } = {
    [WalletType.EVM]: "oui-bg-base-5",
    [WalletType.ABSTRACT]: "oui-bg-base-6",
    [WalletType.SOL]: "oui-bg-base-7",
  };
  return cardBgColorMap[type];
};

const getCardActiveClassName = (
  isActive: boolean,
  isMulti: boolean,
  type: WalletType = WalletType.EVM,
) => {
  const cardActiveColorMap: { [key in WalletType]: string } = {
    [WalletType.EVM]: "oui-border-2 oui-border-primary-light",
    [WalletType.ABSTRACT]: "oui-border-2 oui-border-primary-light",
    [WalletType.SOL]: "oui-border-2 oui-border-link-light",
  };
  return isActive && isMulti && cardActiveColorMap[type];
};

export function WalletCard(props: WalletCardProps) {
  const { t } = useTranslation();
  const copyAddress = async (address: string) => {
    await navigator.clipboard.writeText(address);
    toast.success(t("common.copy.copied"));
  };

  return (
    <div
      className={cn(
        "oui-relative oui-min-h-28 oui-overflow-hidden oui-rounded-2xl  oui-p-4",
        getCardBgClassName(props.type),
        getCardActiveClassName(
          props.isActive,
          props.isMulti || false,
          props.type,
        ),
      )}
    >
      <div
        style={{
          position: "absolute",
          top: 0,
          right: "calc(-1 * var(--oui-space-5))",
          background: "url('/assets/static/sdk/wallet-card-bg.png')",
          width: "calc(var(--oui-space-24) + var(--oui-space-4))",
          height: "calc(var(--oui-space-24) + var(--oui-space-4))",
          backgroundSize: "contain",
          backgroundRepeat: "no-repeat",
          backgroundPosition: "center",
          zIndex: 0,
        }}
      />
      <div className="oui-relative oui-z-10 oui-flex oui-h-full oui-flex-col oui-justify-between">
        <div className="oui-flex oui-items-center oui-justify-between">
          <div className="oui-text-sm oui-font-semibold oui-text-base-contrast">
            {formatAddress(props.address)}
          </div>
          <div className="oui-flex oui-items-center oui-justify-center oui-gap-2">
            <Tooltip content={t("common.copy")} className="oui-z-[65]">
              <button
                type="button"
                className="oui-shell-icon-button"
                aria-label="Copy"
                onClick={() => copyAddress(props.address)}
              >
                <CopyIcon
                  size={16}
                  opacity={1}
                  className="oui-cursor-pointer oui-text-base-contrast/80 hover:oui-text-base-contrast"
                />
              </button>
            </Tooltip>
            {props.isPrivy ? (
              <PrivyWalletHandleOption
                address={props.address}
                type={props.type}
              />
            ) : (
              <NonPrivyWalletHandleOption walletType={props.type} />
            )}
          </div>
        </div>

        <div className="oui-flex oui-items-center oui-justify-between">
          <RenderWalletType walletType={props.type} />

          {props.isMulti && (
            <div className="[&_button]:oui-border-base-contrast/80 [&_svg]:oui-text-base-contrast">
              <Checkbox
                aria-label={`Select ${props.type} wallet ${props.address}`}
                checked={props.isActive}
                onCheckedChange={props.onActiveChange}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
function NonPrivyWalletHandleOption({
  walletType,
}: {
  walletType: WalletType;
}) {
  const { disconnect } = useWalletConnector();
  const { t } = useTranslation();
  const disconnectWallet = () => {
    let walletConnectType: WalletConnectType = WalletConnectType.EVM;
    switch (walletType) {
      case WalletType.EVM:
        walletConnectType = WalletConnectType.EVM;
        break;
      case WalletType.SOL:
        walletConnectType = WalletConnectType.SOL;
        break;
      case WalletType.ABSTRACT:
        walletConnectType = WalletConnectType.ABSTRACT;
        break;
    }
    disconnect({ walletType: walletConnectType } as any);
  };
  return (
    <button
      type="button"
      className="oui-shell-icon-button"
      aria-label={t("connector.disconnectWallet")}
      onClick={() => disconnectWallet()}
    >
      <DisconnectIcon className="oui-size-4 oui-cursor-pointer oui-text-base-contrast/80 hover:oui-text-base-contrast" />
    </button>
  );
}

function PrivyWalletHandleOption({
  address,
  type,
}: {
  address: string;
  type: WalletType;
}) {
  const { t } = useTranslation();
  const { exportWallet } = usePrivyWallet();
  return (
    <DropdownMenuRoot>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="oui-shell-icon-button"
          aria-label={t("common.export")}
        >
          <MoreIcon className="oui-cursor-pointer oui-text-base-contrast/80 hover:oui-text-base-contrast" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent
          size={"xl"}
          align={"end"}
          side="top"
          style={{ minWidth: "var(--oui-space-24)" }}
          className={"oui-z-[65] oui-rounded oui-p-1 oui-font-semibold"}
          sideOffset={0}
        >
          <DropdownMenuItem
            className="oui-cursor-pointer oui-px-2 oui-py-1 oui-text-2xs oui-text-base-contrast-80 hover:oui-text-base-contrast"
            onClick={() => {
              const ns =
                type === WalletType.EVM
                  ? ChainNamespace.evm
                  : ChainNamespace.solana;
              exportWallet(ns, address);
            }}
          >
            <div>{t("common.export")}</div>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenuRoot>
  );
}

const RenderWalletType = ({ walletType }: { walletType: WalletType }) => {
  if (walletType === WalletType.SOL) {
    return (
      <div className="oui-flex oui-items-center oui-justify-start oui-gap-1">
        <div className="">
          <img
            width={16}
            height={16}
            alt=""
            src="/assets/static/sdk/solana-logo.png"
            className="oui-w-4"
          />
        </div>
        <div className="oui-text-2xs oui-font-semibold oui-text-base-contrast">
          Solana
        </div>
      </div>
    );
  }
  if (walletType === WalletType.ABSTRACT) {
    return (
      <div className="oui-flex oui-items-center oui-justify-start oui-gap-2">
        <img
          width={16}
          height={16}
          alt=""
          src={`${PrivyConnectorImagePath}/abstract-transparent.png`}
          className="oui-w-4"
        />
        <div className="oui-text-2xs oui-font-semibold oui-text-base-contrast">
          Abstract
        </div>
      </div>
    );
  }
  if (walletType === WalletType.EVM) {
    return (
      <div className="oui-flex oui-items-center oui-justify-center oui-gap-1">
        <div className="oui-relative oui-flex oui-h-5 oui-w-14 oui-items-center">
          <img
            width={24}
            height={24}
            alt=""
            src="/assets/static/sdk/chains.png"
            className="oui-h-5 oui-w-12"
          />
          <div className="oui-absolute oui-end-0 oui-flex oui-size-5 oui-items-center oui-justify-center oui-rounded-full oui-bg-base-5">
            <EVMChainPopover>
              <MoreIcon className="oui-size-3 oui-text-base-contrast-80 hover:oui-text-base-contrast" />
            </EVMChainPopover>
          </div>
        </div>
        <div className="oui-text-2xs oui-font-semibold oui-text-base-contrast">
          Evm
        </div>
      </div>
    );
  }
  return <></>;
};
