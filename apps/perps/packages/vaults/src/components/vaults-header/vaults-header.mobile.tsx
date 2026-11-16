import { FC, useMemo } from "react";
import { useConfig } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { VaultsHeaderScript } from "./vaults-header.script";
import styles from "../../presentation.module.css";

export const VaultsHeaderMobile: FC<VaultsHeaderScript> = (props) => {
  const { supportVaults, headerImage } = props;
  const { t } = useTranslation();
  const brokerName = useConfig("brokerName");

  const supportVaultsList = useMemo(() => {
    return (
      <div className="oui-flex oui-items-center">
        {supportVaults.map((chain, index) => (
          <img
            key={chain.chain_id}
            src={`/assets/static/network_logo/${chain.chain_id}.png`}
            alt={chain.chain_name}
            width={20}
            height={20}
            className="oui-relative oui-size-4"
            style={{
              marginInlineStart:
                index > 0 ? "calc(-1 * var(--oui-space-1))" : "0",
              zIndex: supportVaults.length - index,
            }}
          />
        ))}
      </div>
    );
  }, [supportVaults]);

  const headerImageElement = useMemo(() => {
    if (typeof headerImage === "string") {
      return (
        <img
          src={headerImage}
          alt=""
          width={384}
          height={240}
          className="oui-size-full oui-object-contain"
        />
      );
    }
    return headerImage;
  }, [headerImage]);

  return (
    <div className={styles.header}>
      <div className="oui-flex oui-min-w-0 oui-flex-1 oui-flex-col">
        <div className="oui-flex oui-items-center oui-gap-1 oui-text-2xs oui-font-semibold oui-text-base-contrast-80">
          {t("vaults.availableOn")} {supportVaultsList}
        </div>
        <h1 className="oui-mb-4 oui-mt-6 oui-min-h-14 oui-w-full oui-text-xl oui-font-bold oui-text-base-contrast">
          {t("vaults.header.title")}
        </h1>
        <div className="oui-max-w-2xl oui-text-2xs oui-font-normal oui-text-base-contrast-80">
          {t("vaults.header.description", { brokerName })}
        </div>
      </div>
      {headerImageElement && (
        <div className="oui-h-32 oui-w-48 oui-max-w-full">
          {headerImageElement}
        </div>
      )}
    </div>
  );
};
