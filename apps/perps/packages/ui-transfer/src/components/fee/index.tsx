import { FC } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { modal, Text } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import { UseDepositFeeReturn } from "../depositForm/hooks/useDepositFee";

type FeeProps = Partial<UseDepositFeeReturn> & {
  nativeSymbol?: string;
};

export const Fee: FC<FeeProps> = (props) => {
  const { dstGasFee, feeQty, feeAmount, dp, nativeSymbol } = props;
  const { t } = useTranslation();

  const onShowFee = () => {
    modal.alert({
      title: t("transfer.deposit.estGasFee"),
      message: (
        <Text intensity={80} size="2xs">
          {t("transfer.deposit.estGasFee.tooltip")}
        </Text>
      ),
    });
  };

  const showFeeQty = !!dstGasFee && dstGasFee !== "0";

  return (
    <Text
      asChild
      size="2xs"
      intensity={80}
      className="oui-cursor-pointer oui-border-b oui-border-dashed oui-border-line-12"
    >
      <button
        type="button"
        onClick={onShowFee}
        className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
      >
        {`${t("transfer.deposit.estGasFee")} ≈ `}
        <Text size="2xs" intensity={80}>
          $
          <Text.numeral dp={2} padding={false} rm={Decimal.ROUND_UP}>
            {feeAmount!}
          </Text.numeral>{" "}
        </Text>
        {showFeeQty && (
          <span>
            (
            <Text intensity={80}>
              <Text.numeral dp={dp} padding={false} rm={Decimal.ROUND_UP}>
                {feeQty!}
              </Text.numeral>
              {nativeSymbol}
            </Text>
            )
          </span>
        )}
      </button>
    </Text>
  );
};
