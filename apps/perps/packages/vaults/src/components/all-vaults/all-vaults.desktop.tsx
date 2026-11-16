import styles from "../../presentation.module.css";
import { FC, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { VaultInfo } from "../../types/vault";
import { VaultCardWidget } from "../vault-card";
import { VaultsList } from "./vaults-list";
import { ViewModeToggle, ViewMode } from "./view-mode-toggle";

export const AllVaultsDesktop: FC<{ vaults: VaultInfo[] }> = (props) => {
  const { vaults } = props;
  const { t } = useTranslation();
  const [viewMode, setViewMode] = useState<ViewMode>("grid");

  return (
    <div className="oui-flex oui-flex-col oui-gap-6">
      <div className="oui-flex oui-items-center oui-justify-between">
        <h2 className="oui-text-xl oui-font-normal oui-text-base-contrast">
          {t("vaults.allVaults")}
        </h2>
        <ViewModeToggle mode={viewMode} onChange={setViewMode} />
      </div>
      {viewMode === "grid" ? (
        <div className={styles.grid}>
          {vaults.map((vault) => (
            <VaultCardWidget key={vault.vault_id} vault={vault} />
          ))}
        </div>
      ) : (
        <VaultsList vaults={vaults} />
      )}
    </div>
  );
};
