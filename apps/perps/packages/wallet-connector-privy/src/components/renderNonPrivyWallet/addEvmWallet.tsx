import { Tooltip } from "../../../../ui-connector/src/platformOverlays";
import React, { useEffect, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { cn } from "@paxeer-network/ui";
import { ChevronDownIcon, ChevronUpIcon } from "@paxeer-network/ui";
import { useWallet } from "../../hooks/useWallet";
import { useWalletConnectorPrivy } from "../../provider";
import { useWagmiWallet } from "../../providers/wagmi/wagmiWalletProvider";
import { WalletConnectType, WalletType } from "../../types";
import { RenderWalletIcon } from "../common";
import { EVMChainPopover } from "../common";
import { MoreIcon } from "../icons";

export function AddEvmWallet() {
  const { t } = useTranslation();
  const [visible, setVisible] = useState(false);
  const { connect } = useWallet();
  const [open, setOpen] = useState(false);
  const { connectors } = useWagmiWallet();
  const { targetWalletType } = useWalletConnectorPrivy();

  useEffect(() => {
    let timer = 0;
    if (targetWalletType === WalletType.EVM) {
      timer = window.setTimeout(() => {
        setOpen(true);
      }, 200);
    }
    return () => {
      if (timer) {
        window.clearTimeout(timer);
      }
    };
  }, [targetWalletType]);
  useEffect(() => {
    if (open === false) {
      return;
    }
    const timeId = window.setTimeout(() => {
      setOpen(false);
    }, 5000);
    return () => {
      if (timeId) {
        window.clearTimeout(timeId);
      }
    };
  }, [open]);

  return (
    <div className="oui-rounded-lg oui-bg-base-10 oui-px-2 oui-py-3">
      <Tooltip
        className="oui-z-[65] oui-max-w-52 oui-text-base-contrast-80"
        open={open}
        onOpenChange={setOpen}
        content={t("connector.privy.addEvmWallet.tips")}
      >
        <div
          tabIndex={0}
          className="oui-flex oui-items-center oui-justify-center oui-gap-1"
        >
          <div className="oui-flex oui-items-center oui-justify-center oui-gap-1">
            <div className="oui-relative oui-flex oui-w-14 oui-items-center oui-justify-start">
              <img
                width={24}
                height={24}
                alt=""
                src="/assets/static/sdk/chains.png"
                className="oui-relative oui-z-0 oui-h-5"
              />
              <div className="oui-absolute oui-end-0 oui-flex oui-size-5 oui-items-center oui-justify-center oui-rounded-full oui-bg-base-5">
                <EVMChainPopover>
                  <MoreIcon
                    className="oui-relative oui-z-10 oui-size-3 oui-text-base-contrast-80 hover:oui-text-base-contrast"
                    style={{ zIndex: 1 }}
                  />
                </EVMChainPopover>
              </div>
            </div>
          </div>
          <div className="oui-text-2xs oui-font-semibold oui-text-base-contrast-80">
            {t("connector.privy.addEvmWallet")}
          </div>

          <button
            type="button"
            className="oui-shell-icon-button"
            aria-expanded={!visible}
            aria-label={t("connector.privy.addEvmWallet")}
            onClick={() => setVisible(!visible)}
          >
            {visible ? (
              <ChevronDownIcon
                size={16}
                opacity={1}
                className="oui-text-base-contrast-80"
              />
            ) : (
              <ChevronUpIcon
                size={16}
                opacity={1}
                className="oui-text-base-contrast"
              />
            )}
          </button>
        </div>
      </Tooltip>
      <div
        hidden={visible}
        className={cn(
          "oui-transition-height oui-grid oui-grid-cols-2 oui-gap-2 oui-overflow-hidden oui-[transition-duration:var(--oui-duration-fast)]",
          visible ? "oui-mt-0 oui-max-h-0" : "oui-mt-3 oui-max-h-96",
        )}
      >
        {connectors.map((item, index) => (
          <button
            type="button"
            key={index}
            className="oui-flex oui-cursor-pointer oui-items-center oui-justify-center  oui-gap-1 oui-bg-base-9 oui-px-2 oui-py-3 oui-text-2xs"
            onClick={() =>
              connect({ walletType: WalletConnectType.EVM, connector: item })
            }
          >
            <RenderWalletIcon connector={item} />
            <div className="oui-text-base-contrast">{item.name}</div>
          </button>
        ))}
      </div>
    </div>
  );
}
