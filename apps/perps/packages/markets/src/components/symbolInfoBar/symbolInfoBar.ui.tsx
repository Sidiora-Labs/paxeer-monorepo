import styles from "../../presentation.module.css";
import { FC, ReactNode } from "react";
import { Flex, Text, cn, Divider, Badge, TokenIcon } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import { TriangleDownIcon } from "../../icons";
import { MarketsProviderProps } from "../marketsProvider";
import { SymbolBadge } from "../symbolBadge";
import { SymbolDisplay } from "../symbolDisplay";
import { RwaTooltip } from "../symbolInfoBarFull/rwaTooltip";
import { UseSymbolInfoBarScriptReturn } from "./symbolInfoBar.script";

export type Layout = "left" | "right";

export type SymbolInfoBarProps = Pick<MarketsProviderProps, "onSymbolChange"> &
  UseSymbolInfoBarScriptReturn & {
    className?: string;
    trailing?: ReactNode;
    onSymbol?: () => void;
  };

export const SymbolInfoBar: FC<SymbolInfoBarProps> = (props) => {
  const {
    symbol,
    data,
    leverage,
    onSymbol,
    isRwa,
    open,
    closeTimeInterval,
    openTimeInterval,
  } = props;

  const symbolView = (
    <Flex asChild className="oui-h-5 oui-cursor-pointer oui-gap-x-1.5">
      <button
        type="button"
        onClick={onSymbol}
        className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
      >
        <SymbolDisplay formatString="base" size="sm" showBadge={false}>
          {symbol}
        </SymbolDisplay>
        <TriangleDownIcon className="oui-size-3.5 oui-text-base-contrast-80" />
      </button>
    </Flex>
  );

  return (
    <Flex
      intensity={900}
      mx={1}
      px={3}
      py={2}
      className={cn(
        styles.surface,
        "oui-symbol-info-bar-mobile",
        "oui-h-full oui-font-semibold oui-rounded-xl oui-flex-wrap oui-gap-3",
        props.className,
      )}
    >
      <Flex
        gapX={3}
        className="oui-h-full oui-min-w-0 oui-flex-1 oui-flex-wrap oui-gap-y-3"
      >
        <Flex gapX={3}>
          <TokenIcon symbol={symbol} size="xs" />
          <Flex direction="column" itemAlign="start" gap={1}>
            {symbolView}
            <Flex gap={1}>
              <Badge size="xs" color="primary">
                {leverage}x
              </Badge>
              <SymbolBadge symbol={symbol} />
            </Flex>
            <RwaTooltip
              isRwa={isRwa}
              open={open}
              closeTimeInterval={closeTimeInterval}
              openTimeInterval={openTimeInterval}
            />
          </Flex>
        </Flex>

        <Divider className="oui-h-9" direction="vertical" intensity={8} />
        <Text.numeral
          size="2xs"
          rule="percentages"
          coloring
          rm={Decimal.ROUND_DOWN}
          showIdentifier
        >
          {data?.["change"]!}
        </Text.numeral>
      </Flex>
      {props.trailing}
    </Flex>
  );
};
