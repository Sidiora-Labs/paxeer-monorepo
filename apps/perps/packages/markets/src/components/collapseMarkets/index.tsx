import { useTranslation } from "@paxeer-network/i18n";
import { TokenIcon, Flex, Text, cn, Tooltip, Badge } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import { useMarketsContext } from "../marketsProvider";
import { SymbolDisplay } from "../symbolDisplay";

export type CollapseMarketsProps = {
  dataSource: any[];
};

export const CollapseMarkets: React.FC<CollapseMarketsProps> = (props) => {
  const { symbol, onSymbolChange } = useMarketsContext();
  const { t } = useTranslation();
  if (props.dataSource?.length === 0) {
    return null;
  }
  return (
    <div className="oui-custom-scrollbar oui-h-full oui-overflow-y-auto">
      <Flex direction="column" px={2} gapY={1}>
        {props.dataSource?.map((item) => {
          const content = (
            <Flex intensity={800} p={2} className="oui-gap-x-7" r="base">
              <Flex direction="column" itemAlign="start" gapY={1}>
                <Flex gapX={1}>
                  <TokenIcon symbol={item.symbol} className="oui-size-4" />
                  <SymbolDisplay formatString="base" size="2xs">
                    {item.symbol}
                  </SymbolDisplay>
                </Flex>
                <Text size="2xs" intensity={80}>
                  {t("markets.column.last")}
                </Text>
                <Text size="2xs" intensity={80}>
                  {t("markets.column.24hPercentage")}
                </Text>
              </Flex>

              <Flex direction="column" itemAlign="end" gapY={1}>
                <Badge size="xs" color="primary">
                  {item.leverage}x
                </Badge>
                <Text.numeral
                  dp={item.quote_dp || 2}
                  currency="$"
                  size="2xs"
                  intensity={80}
                >
                  {item["24h_close"]}
                </Text.numeral>
                <Text.numeral
                  rule="percentages"
                  coloring
                  rm={Decimal.ROUND_DOWN}
                  showIdentifier
                  size="2xs"
                >
                  {item.change}
                </Text.numeral>
              </Flex>
            </Flex>
          );

          return (
            <Tooltip
              side="right"
              sideOffset={6}
              content={content}
              delayDuration={0}
              key={item.symbol}
            >
              <Flex
                asChild
                direction="column"
                justify="center"
                itemAlign="center"
                gapY={1}
                width="var(--oui-space-12)"
                height="var(--oui-space-12)"
                r="lg"
                className={cn(
                  "oui-cursor-pointer",
                  "hover:oui-bg-base-7",
                  symbol === item.symbol && "oui-bg-base-6 hover:oui-bg-base-6",
                )}
              >
                <button
                  type="button"
                  aria-label={item.symbol}
                  onClick={() => {
                    onSymbolChange?.(item);
                  }}
                  className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
                >
                  <TokenIcon symbol={item.symbol} className="oui-size-4" />
                  <Text.numeral
                    rule="percentages"
                    coloring
                    rm={Decimal.ROUND_DOWN}
                    showIdentifier
                    size="2xs"
                  >
                    {item.change}
                  </Text.numeral>
                </button>
              </Flex>
            </Tooltip>
          );
        })}
      </Flex>
    </div>
  );
};
