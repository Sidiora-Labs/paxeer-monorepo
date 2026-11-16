import { FC, useMemo, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Text,
  Button,
  ArrowRightUpSquareFillIcon,
  Tooltip,
  InfoCircleIcon,
} from "@paxeer-network/ui";
import { AuthGuard } from "@paxeer-network/ui-connector";
import styles from "../../presentation.module.css";
import { useVaultsStore } from "../../store/vaultsStore";
import { VaultInfo } from "../../types/vault";
import { useVaultCardScript } from "../vault-card/vaultCard.script";
import { formatAllTimeReturn } from "../vault-card/vaultCard.ui";

type SortField = "tvl" | "apy" | "deposits" | "pnl";
type SortDirection = "asc" | "desc";

interface VaultsListProps {
  vaults: VaultInfo[];
}

export const VaultsList: FC<VaultsListProps> = ({ vaults }) => {
  const { t } = useTranslation();
  const [sortField, setSortField] = useState<SortField>("apy");
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
  const lpData = useVaultsStore((state) => state.vaultLpInfo.data);
  const shouldShowApyTooltip = vaults.some(
    (vault) =>
      vault.status === "pre_launch" ||
      (vault.vault_age !== null && vault.vault_age < 7),
  );

  const sortedVaults = useMemo(() => {
    const getValue = (vault: VaultInfo): number | undefined => {
      if (sortField === "tvl") return vault.tvl;
      if (sortField === "apy") return vault.lifetime_apy;
      const lp = lpData[vault.vault_id]?.[0];
      if (!lp) return undefined;
      return sortField === "deposits"
        ? lp.lp_tvl - lp.potential_pnl
        : lp.potential_pnl;
    };
    return [...vaults].sort((a, b) => {
      const av = getValue(a);
      const bv = getValue(b);
      if (av === undefined) return bv === undefined ? 0 : 1;
      if (bv === undefined) return -1;
      return sortDirection === "asc" ? av - bv : bv - av;
    });
  }, [vaults, sortField, sortDirection, lpData]);

  const sortHeader = (field: SortField, label: string) => (
    <th
      scope="col"
      aria-sort={
        sortField === field
          ? sortDirection === "asc"
            ? "ascending"
            : "descending"
          : undefined
      }
    >
      <button
        type="button"
        className="oui-inline-flex oui-items-center oui-gap-1"
        onClick={() => {
          setSortDirection(
            sortField === field && sortDirection === "desc" ? "asc" : "desc",
          );
          setSortField(field);
        }}
      >
        {label}
        <span aria-hidden="true">
          {sortField === field ? (sortDirection === "asc" ? "↑" : "↓") : "↕"}
        </span>
      </button>
      {field === "apy" && shouldShowApyTooltip && (
        <Tooltip content={t("vaults.card.allTimeReturnTooltip")}>
          <button
            type="button"
            aria-label={t("vaults.card.allTimeReturnTooltip")}
          >
            <InfoCircleIcon size={16} />
          </button>
        </Tooltip>
      )}
    </th>
  );

  return (
    <div
      className={`${styles.surface} ${styles.tableRegion}`}
      role="region"
      aria-label={t("vaults.allVaults")}
      tabIndex={0}
    >
      <table className={styles.table}>
        <caption className="oui-sr-only">{t("vaults.allVaults")}</caption>
        <thead>
          <tr>
            <th scope="col">{t("vaults.list.poolName")}</th>
            {sortHeader("tvl", "TVL")}
            {sortHeader("apy", t("vaults.card.allTimeReturn"))}
            {sortHeader("deposits", t("vaults.list.myDeposits"))}
            {sortHeader("pnl", t("vaults.list.allTimePnl"))}
            <th scope="col">{t("vaults.card.accountBalance")}</th>
            <th scope="col">{t("vaults.list.operate")}</th>
          </tr>
        </thead>
        <tbody>
          {sortedVaults.map((vault) => (
            <VaultListRow key={vault.vault_id} vault={vault} />
          ))}
        </tbody>
      </table>
    </div>
  );
};

const VaultListRow: FC<{ vault: VaultInfo }> = ({ vault }) => {
  const { t } = useTranslation();
  const {
    title,
    vaultInfo,
    lpInfo,
    isEVMConnected,
    isSOLConnected,
    openDepositAndWithdraw,
    availableBalance,
    openVaultWebsite,
    isDepositDisabled,
    isWithdrawDisabled,
  } = useVaultCardScript(vault);
  const statusText = {
    live: t("vaults.card.status.active"),
    pre_launch: t("vaults.card.launchingSoon"),
    closing: t("vaults.card.status.closing"),
    closed: t("vaults.card.status.closed"),
  }[vaultInfo.status];

  return (
    <tr>
      <th scope="row">
        <div className="oui-flex oui-flex-col oui-items-start oui-gap-1">
          <span className="oui-text-sm oui-font-semibold oui-text-base-contrast">
            {title}
          </span>
          <span className={styles.status} data-status={vaultInfo.status}>
            {statusText}
          </span>
          <button
            type="button"
            onClick={openVaultWebsite}
            className="oui-flex oui-items-center oui-gap-1 oui-text-xs oui-text-primary"
          >
            {t("vaults.card.viewMore")}
            <ArrowRightUpSquareFillIcon size={16} />
          </button>
        </div>
      </th>
      <td>
        <Text.numeral currency="$" dp={0}>
          {vaultInfo.tvl}
        </Text.numeral>
      </td>
      <td>
        <Text.gradient color="brand">
          {formatAllTimeReturn(
            vaultInfo.status,
            vaultInfo.vault_age,
            vaultInfo.lifetime_apy,
          )}
        </Text.gradient>
      </td>
      <td>
        <Text.numeral dp={2}>{lpInfo.deposits}</Text.numeral>
      </td>
      <td>
        <Text.numeral dp={2}>{lpInfo.earnings}</Text.numeral>
      </td>
      <td>
        <Text.numeral dp={2}>{availableBalance}</Text.numeral>
      </td>
      <td>
        <AuthGuard buttonProps={{ size: "sm" }}>
          {isEVMConnected || isSOLConnected ? (
            <div className="oui-flex oui-flex-wrap oui-gap-2">
              <Button
                size="sm"
                disabled={isDepositDisabled}
                onClick={() => openDepositAndWithdraw("deposit")}
              >
                {t("common.deposit")}
              </Button>
              <Button
                size="sm"
                color="secondary"
                disabled={isWithdrawDisabled}
                onClick={() => openDepositAndWithdraw("withdraw")}
              >
                {t("common.withdraw")}
              </Button>
            </div>
          ) : (
            <Button size="sm" color="warning" fullWidth>
              {t("connector.wrongNetwork")}
            </Button>
          )}
        </AuthGuard>
      </td>
    </tr>
  );
};
