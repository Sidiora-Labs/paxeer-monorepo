import styles from "../../presentation.module.css";
import { FC } from "react";
import { Trans, useTranslation } from "@paxeer-network/i18n";
import { Box, Button, Flex, textVariants, Text, cn } from "@paxeer-network/ui";
import { AuthGuard } from "@paxeer-network/ui-connector";
import { TransferVerticalIcon } from "../../icons";
import { AccountSelect } from "../accountSelect";
import { AvailableQuantity } from "../availableQuantity";
import { ExchangeDivider } from "../exchangeDivider";
import { QuantityInput } from "../quantityInput";
import { UnsettlePnlInfo } from "../unsettlePnlInfo";
import { TransferFormScriptReturn } from "./transferForm.script";

export type TransferFormProps = TransferFormScriptReturn;

export const TransferForm: FC<TransferFormProps> = (props) => {
  const {
    networkId,
    disabled,
    onTransfer,
    quantity,
    onQuantityChange,
    amount,
    tokens,
    token,
    onTokenChange,
    maxQuantity,
    submitting,
    hintMessage,
    inputStatus,
    hasPositions,
    onSettlePnl,
    unsettledPnL,
    toAccountAsset,
    fromAccounts,
    toAccounts,
    fromAccount,
    onFromAccountChange,
    toAccount,
    onToAccountChange,
    onExchange,
  } = props;

  const { t } = useTranslation();

  const buttonSize = { initial: "md", lg: "lg" } as const;

  return (
    <Box
      id="oui-deposit-form"
      className={`${styles.surface} ${textVariants({ weight: "semibold" })}`}
    >
      <Box className="oui-mb-6 lg:oui-mb-8">
        <Text size="sm" intensity={98}>
          {t("transfer.internalTransfer.from")}
        </Text>
        <Box mt={1} mb={1}>
          <AccountSelect
            subAccounts={fromAccounts}
            value={fromAccount}
            onValueChange={onFromAccountChange}
          />
          <QuantityInput
            classNames={{
              root: "oui-mt-0.5 oui-rounded-t-sm oui-rounded-b-xl",
            }}
            value={quantity}
            onValueChange={onQuantityChange}
            tokens={tokens}
            token={token}
            onTokenChange={onTokenChange}
            hintMessage={hintMessage}
            status={inputStatus}
          />
        </Box>
        <AvailableQuantity
          token={token}
          quantity={amount}
          maxQuantity={maxQuantity}
          onClick={() => {
            onQuantityChange(maxQuantity.toString());
          }}
        />
        <Box mx={2} mt={1}>
          <UnsettlePnlInfo
            unsettledPnl={unsettledPnL}
            hasPositions={hasPositions}
            onSettlePnl={onSettlePnl}
            tooltipContent={t("transfer.internalTransfer.unsettled.tooltip")}
            dialogContent={
              // @ts-ignore
              <Trans i18nKey="transfer.internalTransfer.settlePnl.description" />
            }
          />
        </Box>

        <ExchangeDivider
          icon={
            <button
              type="button"
              onClick={onExchange}
              aria-label="Swap accounts"
              className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
            >
              <TransferVerticalIcon
                // TODO: determines if the current paxeer-network key is the main account
                // className={cn(
                //   isMainAccount
                //     ? "oui-cursor-pointer oui-text-primary"
                //     : " oui-cursor-not-allowed oui-text-base-contrast-80",
                // )}
                // onClick={isMainAccount ? onExchange : undefined}
                className="oui-cursor-pointer oui-text-primary"
              />
            </button>
          }
        />

        <Text size="sm" intensity={98}>
          {t("transfer.internalTransfer.to")}
        </Text>
        <Box mt={1}>
          <AccountSelect
            subAccounts={toAccounts}
            value={toAccount}
            onValueChange={onToAccountChange}
          />
          <Flex
            className={cn(
              "oui-mt-0.5 oui-h-8 oui-text-base-contrast-80 oui-bg-base-6",
              "oui-rounded-b-xl oui-rounded-t-sm",
            )}
            justify="between"
            itemAlign="center"
            px={3}
          >
            <Text size="2xs">
              {t("transfer.internalTransfer.currentAssetValue")}
            </Text>
            <Text.numeral
              size="2xs"
              intensity={80}
              unit={` ${token.symbol}`}
              dp={token?.precision}
              padding={false}
            >
              {toAccountAsset}
            </Text.numeral>
          </Flex>
        </Box>
      </Box>

      <Flex justify="center">
        <Box className="oui-w-full lg:oui-w-auto lg:oui-min-w-44">
          <AuthGuard
            networkId={networkId}
            buttonProps={{
              fullWidth: true,
              size: buttonSize,
            }}
          >
            <Button
              fullWidth
              disabled={disabled}
              loading={submitting}
              size={buttonSize}
              onClick={onTransfer}
            >
              {t("common.transfer")}
            </Button>
          </AuthGuard>
        </Box>
      </Flex>
    </Box>
  );
};
