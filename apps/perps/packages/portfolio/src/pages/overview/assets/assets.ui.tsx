import styles from "../../../presentation.module.css";
import React, { FC } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Card,
  Divider,
  Flex,
  Grid,
  Either,
  Statistic,
  Text,
  EyeIcon,
  gradientTextVariants,
  EditIcon,
  EyeCloseIcon,
  Tooltip,
} from "@paxeer-network/ui";
import { AuthGuard } from "@paxeer-network/ui-connector";
import { AssetScriptReturn } from "./assets.script";
import { AssetsHeader } from "./assetsHeader";

export const AssetsUI: React.FC<
  AssetScriptReturn & { onConnectWallet?: () => void }
> = (props) => {
  const { t } = useTranslation();
  return (
    <Card
      classNames={{ footer: "oui-min-h-12", root: "oui-h-auto oui-min-h-60" }}
      title={
        <AssetsHeader
          disabled={!props.canTrade}
          isMainAccount={props.isMainAccount}
          onDeposit={props.onDeposit}
          onWithdraw={props.onWithdraw}
          onTransfer={props.onTransfer}
          hasSubAccount={props.hasSubAccount}
        />
      }
    >
      <>
        <Statistic
          label={
            <Flex gap={1}>
              <Text intensity={80}>{t("common.totalValue")}</Text>
              <button
                type="button"
                aria-label={
                  props.visible
                    ? "Hide portfolio value"
                    : "Show portfolio value"
                }
                aria-pressed={props.visible}
                onClick={() => {
                  props.toggleVisible();
                }}
                data-testid="oui-testid-portfolio-assets-eye-btn"
              >
                {props.visible ? (
                  <EyeIcon size={16} color="inherit" />
                ) : (
                  <EyeCloseIcon size={16} color="inherit" />
                )}
              </button>
            </Flex>
          }
        >
          <Either value={props.canTrade!} left={<NoValue />}>
            <Text.numeral
              visible={props.visible}
              unit="USDC"
              // @ts-ignore
              style={{ "--oui-gradient-angle": "45deg" }}
              unitClassName="oui-text-base oui-text-base-contrast-80 oui-h-9 oui-ms-1"
              className={gradientTextVariants({
                className: "oui-font-bold oui-text-3xl",
                color: "brand",
              })}
            >
              {props.portfolioValue ?? "--"}
            </Text.numeral>
          </Either>
        </Statistic>
        <Divider className="oui-my-4" intensity={8} />
        <AuthGuard buttonProps={{ size: "lg", fullWidth: true }}>
          <AssetStatistic
            unrealROI={props.unrealROI}
            unrealPnL={props.unrealPnL}
            freeCollateral={props.freeCollateral}
            maxWithdrawAmount={props.maxWithdrawAmount}
            currentLeverage={props.currentLeverage}
            onLeverageEdit={props.onLeverageEdit}
            visible={props.visible}
          />
        </AuthGuard>
      </>
    </Card>
  );
};

const NoValue: FC = () => {
  return (
    <Flex gap={1} className={"oui-h-9"}>
      <Text.gradient color="brand" weight="bold">
        --
      </Text.gradient>
      <Text>USDC</Text>
    </Flex>
  );
};

type AssetStatisticProps = Pick<
  AssetScriptReturn,
  | "currentLeverage"
  | "unrealPnL"
  | "unrealROI"
  | "freeCollateral"
  | "onLeverageEdit"
  | "visible"
  | "maxWithdrawAmount"
>;

export const AssetStatistic = (props: AssetStatisticProps) => {
  const { t } = useTranslation();

  const maxWithdrawAmountText =
    props.maxWithdrawAmount === undefined || props.maxWithdrawAmount === null
      ? "--"
      : props.maxWithdrawAmount.toString();

  return (
    <div className={styles.metrics}>
      <Statistic label={t("common.unrealizedPnl")}>
        <Flex wrap="wrap">
          <Text.pnl
            coloring
            size="lg"
            weight="semibold"
            visible={props.visible}
          >
            {props.unrealPnL}
          </Text.pnl>
          <Text.roi
            coloring
            rule="percentages"
            size="sm"
            weight="semibold"
            prefix={"("}
            suffix={")"}
            visible={props.visible}
          >
            {props.unrealROI}
          </Text.roi>
        </Flex>
      </Statistic>
      <Statistic
        label={
          <Tooltip
            content={t("transfer.withdraw.available.tooltip", {
              amount: maxWithdrawAmountText,
            })}
            className="oui-max-w-72"
            delayDuration={300}
          >
            <Text
              size="xs"
              intensity={80}
              className="oui-cursor-pointer oui-border-b oui-border-dashed oui-border-line-12"
            >
              {t("portfolio.overview.availableWithdraw")}
            </Text>
          </Tooltip>
        }
        // @ts-ignore
        valueProps={{ size: "lg", visible: props.visible }}
      >
        {props.freeCollateral}
      </Statistic>
    </div>
  );
};
