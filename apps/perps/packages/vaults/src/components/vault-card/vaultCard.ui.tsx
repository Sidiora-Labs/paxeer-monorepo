import styles from "../../presentation.module.css";
import { FC, useMemo } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Text,
  cn,
  Button,
  ArrowRightUpSquareFillIcon,
  Tooltip,
  InfoCircleIcon,
} from "@paxeer-network/ui";
import { AuthGuard } from "@paxeer-network/ui-connector";
import { Decimal } from "@paxeer-network/utils";
import { VaultStatus } from "../../types/vault";
import { parseMarkdownLinks } from "../../utils/parseMarkdownLinks";
import { VaultCardScript } from "./vaultCard.script";

export const formatAllTimeReturn = (
  status: VaultStatus,
  vaultAge: number | null,
  lifetimeApy: number,
): string => {
  if (status === "pre_launch" || (vaultAge !== null && vaultAge < 7)) {
    return "--";
  }
  if (lifetimeApy > 100) {
    return ">10000%";
  }
  return (
    new Decimal(lifetimeApy)
      .mul(100)
      .toDecimalPlaces(2, Decimal.ROUND_UP)
      .toFixed(2) + "%"
  );
};

export const VaultCard: FC<VaultCardScript> = (props) => {
  const {
    title,
    vaultInfo,
    lpInfo,
    description,
    isEVMConnected,
    isSOLConnected,
    openDepositAndWithdraw,
    availableBalance,
    openVaultWebsite,
    isDepositDisabled,
    isWithdrawDisabled,
  } = props;

  const { t } = useTranslation();

  const formattedAllTimeReturn = useMemo(
    () =>
      formatAllTimeReturn(
        vaultInfo.status,
        vaultInfo.vault_age,
        vaultInfo.lifetime_apy,
      ),
    [vaultInfo.status, vaultInfo.vault_age, vaultInfo.lifetime_apy],
  );

  const statusText = {
    live: t("vaults.card.status.active"),
    pre_launch: t("vaults.card.launchingSoon"),
    closing: t("vaults.card.status.closing"),
    closed: t("vaults.card.status.closed"),
  }[vaultInfo.status];

  return (
    <article className={styles.card}>
      <div>
        <div className="oui-flex oui-flex-col oui-gap-2">
          {/* Title + Status tag group, 4px spacing */}
          <div className="oui-flex oui-flex-col oui-gap-1">
            {/* Vault name */}
            <h3 className="oui-text-lg oui-font-semibold oui-text-base-contrast oui-break-words">
              {title}
            </h3>
            {/* Status tag */}
            <span className={styles.status} data-status={vaultInfo.status}>
              {statusText}
            </span>
          </div>

          {/* Description + View more block (gap-1 = 4px) */}
          <div className="oui-flex oui-flex-col oui-gap-1">
            <div className="oui-min-h-13">
              <div className="oui-text-2xs oui-font-normal oui-leading-normal oui-text-base-contrast-80">
                {parseMarkdownLinks(description)}
              </div>
            </div>
            {/* View more link */}
            <button
              type="button"
              onClick={openVaultWebsite}
              className="oui-flex oui-w-fit oui-items-center oui-gap-1 oui-text-xs oui-font-medium oui-text-primary"
            >
              <span>{t("vaults.card.viewMore")}</span>
              <ArrowRightUpSquareFillIcon
                className="oui-text-primary"
                width={16}
                height={16}
                viewBox="0 0 18 18"
              />
            </button>
          </div>

          {/* KPI + LP block (gap-6 = 24px) */}
          <div className="oui-flex oui-flex-col oui-gap-6">
            <div className={styles.kpis}>
              <VaultInfoItem
                label={t("vaults.card.tvl")}
                value={vaultInfo.tvl}
                textProps={{
                  currency: "$",
                  dp: 0,
                  type: "numeral",
                }}
              />
              <VaultInfoItem
                label={t("vaults.card.allTimeReturn")}
                value={formattedAllTimeReturn}
                textProps={{
                  color: "brand",
                  type: "gradient",
                }}
                showTooltip={
                  vaultInfo.status === "pre_launch" ||
                  (vaultInfo.vault_age !== null && vaultInfo.vault_age < 7)
                }
                tooltipContent={t("vaults.card.allTimeReturnTooltip")}
              />
            </div>

            <div className="oui-flex oui-flex-col oui-items-center oui-gap-2 oui-rounded-lg oui-bg-base-contrast-6 oui-p-3">
              <LpInfoItem
                label={t("vaults.card.myDeposits")}
                value={lpInfo.deposits}
              />
              <LpInfoItem
                label={t("vaults.card.myEarnings")}
                value={lpInfo.earnings}
              />
            </div>
          </div>

          {/* Account balance + Buttons block (gap-4 = 16px, mt-2 for total 16px from My Deposits) */}
          <div className="oui-flex oui-flex-col oui-gap-4 oui-mt-2">
            <LpInfoItem
              label={t("vaults.card.accountBalance")}
              value={availableBalance}
            />

            <VaultCardOperation
              isEVMConnected={isEVMConnected}
              isSOLConnected={isSOLConnected}
              openDepositAndWithdraw={openDepositAndWithdraw}
              isDepositDisabled={isDepositDisabled}
              isWithdrawDisabled={isWithdrawDisabled}
            />
          </div>
        </div>
      </div>
    </article>
  );
};

const VaultInfoItem: FC<{
  label: string;
  value: string | number;
  textProps?: any;
  showTooltip?: boolean;
  tooltipContent?: string;
}> = (props) => {
  const { label, value, textProps, showTooltip, tooltipContent } = props;

  return (
    <div
      className={cn(
        "oui-flex oui-flex-1 oui-flex-col oui-items-center oui-justify-center oui-px-3 oui-py-2",
        "oui-rounded-lg oui-border oui-border-solid oui-border-line-12",
      )}
    >
      <div className="oui-flex oui-items-center oui-gap-1 oui-text-2xs oui-font-normal oui-leading-normal oui-text-base-contrast-80">
        {label}
        {showTooltip && tooltipContent && (
          <Tooltip content={tooltipContent}>
            <button type="button" aria-label={label}>
              <InfoCircleIcon
                size={12}
                opacity={1}
                className="oui-text-base-contrast"
              />
            </button>
          </Tooltip>
        )}
      </div>
      {textProps.type === "gradient" ? (
        <Text.gradient
          className="oui-text-base oui-font-semibold"
          {...textProps}
        >
          {value}
        </Text.gradient>
      ) : (
        <Text.numeral
          className="oui-text-base oui-font-semibold"
          {...textProps}
        >
          {value}
        </Text.numeral>
      )}
    </div>
  );
};

const LpInfoItem: FC<{
  label: string;
  value: string | number;
  textProps?: any;
}> = (props) => {
  const { label, value, textProps } = props;

  return (
    <div className="oui-flex oui-w-full oui-items-center oui-justify-between oui-text-2xs oui-font-normal">
      <div className="oui-text-base-contrast-80">{label}</div>
      <Text.numeral
        className="oui-text-base-contrast-80"
        {...textProps}
        dp={2}
        suffix={
          <span className="oui-ms-1 oui-text-base-contrast-80">USDC</span>
        }
      >
        {value}
      </Text.numeral>
    </div>
  );
};

type VaultCardOperationProps = {
  isEVMConnected: boolean;
  isSOLConnected: boolean;
  openDepositAndWithdraw: (activeTab: "deposit" | "withdraw") => void;
  isDepositDisabled: boolean;
  isWithdrawDisabled: boolean;
};

const VaultCardOperation: FC<VaultCardOperationProps> = (props) => {
  const {
    isEVMConnected,
    isSOLConnected,
    openDepositAndWithdraw,
    isDepositDisabled,
    isWithdrawDisabled,
  } = props;
  const { t } = useTranslation();

  return (
    <AuthGuard buttonProps={{ size: "md", fullWidth: true }}>
      {isEVMConnected || isSOLConnected ? (
        <div className="oui-flex oui-items-center oui-gap-2">
          <Button
            className="oui-flex-1"
            size="md"
            disabled={isDepositDisabled}
            onClick={() => openDepositAndWithdraw("deposit")}
          >
            {t("common.deposit")}
          </Button>
          <Button
            className="oui-flex-1"
            size="md"
            color="secondary"
            disabled={isWithdrawDisabled}
            onClick={() => openDepositAndWithdraw("withdraw")}
          >
            {t("common.withdraw")}
          </Button>
        </div>
      ) : (
        <Button size="md" color="warning">
          {t("connector.wrongNetwork")}
        </Button>
      )}
    </AuthGuard>
  );
};
