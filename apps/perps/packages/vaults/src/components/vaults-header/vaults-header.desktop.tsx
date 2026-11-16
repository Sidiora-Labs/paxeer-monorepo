import { FC, useMemo } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { VaultsHeaderScript } from "./vaults-header.script";
import styles from "../../presentation.module.css";

export const VaultsHeaderDesktop: FC<VaultsHeaderScript> = (props) => {
  const { supportVaults, headerImage, brokerName } = props;
  const { t } = useTranslation();

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
            className="oui-relative oui-size-5"
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
      <div className="oui-flex oui-max-w-3xl oui-flex-col">
        <div className="oui-flex oui-items-center oui-gap-1 oui-text-lg oui-font-normal oui-text-base-contrast-80">
          {t("vaults.availableOn")} {supportVaultsList}
        </div>
        <h1 className="oui-mb-6 oui-mt-8 oui-text-5xl oui-font-bold oui-leading-normal oui-text-base-contrast">
          {t("vaults.header.title")}
        </h1>
        <div className="oui-text-xl oui-font-normal oui-text-base-contrast-80">
          {t("vaults.header.description", { brokerName })}
        </div>
      </div>
      {headerImageElement && (
        <div className="oui-h-60 oui-w-96 oui-max-w-full">
          {headerImageElement}
        </div>
      )}
    </div>
  );
};
