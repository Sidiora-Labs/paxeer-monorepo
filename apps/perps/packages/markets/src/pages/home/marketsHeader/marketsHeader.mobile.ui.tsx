import styles from "../../../presentation.module.css";
import { FC, ReactNode, useMemo, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Box, cn, Flex, TabPanel, Tabs, Text } from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import { useMarketsContext } from "../../../components/marketsProvider";
import { SymbolDisplay } from "../../../components/symbolDisplay";
import { PaxeerNetworkIcon } from "../../../icons";
import { MarketsHeaderReturns } from "./marketsHeader.script";

type MobileMarketsHeaderProps = MarketsHeaderReturns & {
  className?: string;
};

export const MobileMarketsHeader: FC<MobileMarketsHeaderProps> = (props) => {
  const {
    news,
    gainers,
    losers,
    total24Amount,
    totalOpenInterest,
    tvl,
    favorite,
  } = props;
  const [activeTab, setActiveTab] = useState("newListings");
  const { onSymbolChange } = useMarketsContext();
  const { t } = useTranslation();

  const onSymbol = (item: any) => {
    onSymbolChange?.(item);
    favorite.addToHistory(item);
  };

  return (
    <div
      id="oui-markets-header"
      className={cn(styles.surface, props.className)}
    >
      <BlockList
        total24Amount={total24Amount}
        totalOpenInterest={totalOpenInterest}
        tvl={tvl}
      />

      <Box intensity={900} r="xl" p={3} mt={3}>
        <Tabs size="md" value={activeTab} onValueChange={setActiveTab}>
          <TabPanel
            title={t("markets.newListings")}
            value="newListings"
            testid="oui-testid-markets-tab"
          >
            <CardItem data={news} onSymbol={onSymbol} />
          </TabPanel>
          <TabPanel
            title={t("markets.topGainers")}
            value="topGainers"
            testid="oui-testid-funding-tab"
          >
            <CardItem data={gainers} onSymbol={onSymbol} />
          </TabPanel>
          <TabPanel
            title={t("markets.topLosers")}
            value="topLosers"
            testid="oui-testid-funding-tab"
          >
            <CardItem data={losers} onSymbol={onSymbol} />
          </TabPanel>
        </Tabs>
      </Box>
    </div>
  );
};

type BlockListProps = {
  className?: string;
  total24Amount?: number;
  totalOpenInterest?: number;
  tvl?: number;
};

const BlockList: React.FC<BlockListProps> = (props) => {
  const { total24Amount, totalOpenInterest, tvl } = props;
  const { t } = useTranslation();

  const list = useMemo(() => {
    return [
      {
        label: (
          <Flex gapX={1}>
            <PaxeerNetworkIcon /> {t("markets.column.24hVolume")}
          </Flex>
        ),
        value: total24Amount,
      },
      {
        label: (
          <Flex gapX={1}>
            <PaxeerNetworkIcon /> {t("markets.openInterest")}
          </Flex>
        ),
        value: totalOpenInterest,
      },
      {
        label: (
          <Flex gapX={1}>
            <PaxeerNetworkIcon /> {`${t("common.assets")} (TVL)`}
          </Flex>
        ),
        value: tvl,
      },
    ];
  }, [t, total24Amount, totalOpenInterest, tvl]);

  return (
    <div className={cn(styles.metrics, props.className)}>
      {list?.map((item, index) => (
        <BlockItem key={`item-${index}`} {...item} />
      ))}
    </div>
  );
};

type BlockItemProps = {
  label: ReactNode;
  value?: number;
  rule?: string;
  dp?: number;
  className?: string;
};

const BlockItem: React.FC<BlockItemProps> = (props) => {
  return (
    <Box
      r="xl"
      className={cn(
        "oui-flex oui-min-w-0 oui-flex-col oui-justify-between",
        "oui-min-h-28 oui-p-3",
        "oui-bg-primary-darken oui-text-primary-contrast",
        props.className,
      )}
    >
      <Text
        as="div"
        weight="semibold"
        className="oui-text-2xs oui-leading-normal oui-text-primary-contrast/80"
      >
        {props.label}
      </Text>

      <Text.numeral
        currency="$"
        dp={props.dp || 2}
        rm={Decimal.ROUND_DOWN}
        rule={(props.rule as any) || "human"}
        className={cn(
          "oui-text-xl oui-font-semibold",
          "oui-tabular-nums oui-tracking-tight oui-leading-none",
        )}
      >
        {props.value!}
      </Text.numeral>
    </Box>
  );
};

type CardItemProps = {
  data?: TListItem[];
  className?: string;
  onSymbol: (item: any) => void;
};

const CardItem: React.FC<CardItemProps> = (props) => {
  return (
    <Flex direction="column" itemAlign="start" mt={3}>
      {props.data?.map((item) => (
        <ListItem
          key={item.symbol}
          item={item}
          onSymbol={props.onSymbol}
          className="oui-px-0"
        />
      ))}
    </Flex>
  );
};

type TListItem = {
  symbol: string;
  price: string;
  change: number;
  precision: number;
  [x: string]: any;
};

type ListItemProps = {
  item: TListItem;
  className?: string;
  onSymbol: (item: any) => void;
};

const ListItem: React.FC<ListItemProps> = (props) => {
  const { item } = props;

  return (
    <Box
      asChild
      width="100%"
      py={2}
      px={4}
      className={cn(
        styles.marketRow,
        "oui-items-center",
        "oui-cursor-pointer hover:oui-bg-base-8",
        props.className,
      )}
      onClick={() => {
        props.onSymbol(item);
      }}
    >
      <button type="button">
        <SymbolDisplay formatString="base" showIcon className="oui-min-w-0">
          {item.symbol}
        </SymbolDisplay>

        <Flex justify="end">
          <Text.numeral
            currency="$"
            size="xs"
            weight="semibold"
            dp={item.quote_dp}
            className="oui-tabular-nums oui-text-end"
          >
            {item["24h_close"]}
          </Text.numeral>
        </Flex>

        <Flex justify="end">
          <Text.numeral
            rule="percentages"
            coloring
            size="xs"
            weight="semibold"
            showIdentifier
            className="oui-tabular-nums oui-text-end"
          >
            {item.change}
          </Text.numeral>
        </Flex>
      </button>
    </Box>
  );
};
