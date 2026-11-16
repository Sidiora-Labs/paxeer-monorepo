import { FC } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Flex, Text, ChevronRightIcon } from "@paxeer-network/ui";

type DepositStatusBlockProps = {
  amount: number;
  symbol?: string;
  pendingCount: number;
  explorerUrl: string;
};

export const DepositStatusBlock: FC<DepositStatusBlockProps> = ({
  amount,
  symbol = "USDC",
  pendingCount,
  explorerUrl,
}) => {
  const { t: t0 } = useTranslation();
  const t = t0 as any;
  const label = t("transfer.exclusiveDeposit.depositPending", {
    amount,
    symbol,
  });

  const handleClick = () => {
    window.open(explorerUrl, "_blank", "noopener,noreferrer");
  };

  return (
    <Flex
      asChild
      itemAlign="center"
      gap={2}
      className="oui-group oui-mt-6 oui-w-full oui-cursor-pointer"
    >
      <button
        type="button"
        onClick={handleClick}
        className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
      >
        <div className="oui-size-2 oui-shrink-0 oui-rounded-full oui-bg-primary" />
        <Text size="xs" intensity={98} className="oui-flex-1">
          {label}
        </Text>
        {pendingCount > 1 && (
          <Flex
            justify="center"
            itemAlign="center"
            r="full"
            width="var(--oui-space-4)"
            height="var(--oui-space-4)"
            className="oui-bg-line-12"
          >
            <Text size="3xs" intensity={80}>
              {pendingCount}
            </Text>
          </Flex>
        )}
        <ChevronRightIcon className="oui-size-4 oui-text-base-contrast-80 oui-transition-colors group-hover:oui-text-base-contrast-80" />
      </button>
    </Flex>
  );
};
