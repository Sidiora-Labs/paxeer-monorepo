import { Text } from '@chakra-ui/react';
import { useQuery } from '@tanstack/react-query';
import React from 'react';

import type { HomeStats } from 'types/api/stats';
import type { TChainIndicator } from 'ui/home/indicators/types';

import useApiQuery from 'lib/api/useApiQuery';
import { HOMEPAGE_STATS } from 'stubs/multichain';
import { Image } from 'toolkit/chakra/image';
import ChainIndicatorsChart from 'ui/home/indicators/ChainIndicatorsChart';
import ChainIndicatorsContainer from 'ui/home/indicators/ChainIndicatorsContainer';
import ChainIndicatorsList from 'ui/home/indicators/ChainIndicatorsList';
import { isIndicatorEnabled, sortIndicators } from 'ui/home/indicators/utils/indicators';
import IconSvg from 'ui/shared/IconSvg';
import TokenLogoPlaceholder from 'ui/shared/TokenLogoPlaceholder';

import useChartDataQuery from './useChartDataQuery';
import useFetchParentChainApi from './useFetchParentChainApi';

const ChainIndicators = () => {
  const statsQuery = useApiQuery('multichainStats:pages_main', {
    queryOptions: {
      refetchOnMount: false,
      placeholderData: HOMEPAGE_STATS,
    },
  });

  const parentChainApiFetch = useFetchParentChainApi();
  const { market } = parentChainApiFetch;

  const parentChainStatsQuery = useQuery({
    queryKey: [ ...parentChainApiFetch.queryKey, 'stats' ],
    queryFn: () => parentChainApiFetch({ path: '/stats' }) as Promise<HomeStats>,
    refetchOnMount: false,
    enabled: Boolean(market),
  });

  const hasCoinPrice = parentChainStatsQuery.data?.coin_price != null;
  const hasMarketCap = parentChainStatsQuery.data?.market_cap != null;
  const marketUnavailable = !market || parentChainStatsQuery.isError ||
    (parentChainStatsQuery.isSuccess && (!hasCoinPrice || !hasMarketCap));

  const indicators: Array<TChainIndicator> = React.useMemo(() => ([
    statsQuery.data && {
      id: 'daily_txs' as const,
      title: statsQuery.data.new_txns_multichain_window?.info?.title || 'Daily transactions',
      value: (() => {
        if (statsQuery.data.yesterday_txns_multichain?.value != null) {
          return Number(statsQuery.data.yesterday_txns_multichain.value).toLocaleString(undefined, { maximumFractionDigits: 2, notation: 'compact' });
        }
        return 'N/A';
      })(),
      valueDiff: (() => {
        if (statsQuery.data.new_txns_multichain_window && statsQuery.data.new_txns_multichain_window.chart.length >= 2) {
          const lastDay = statsQuery.data.new_txns_multichain_window.chart[statsQuery.data.new_txns_multichain_window.chart.length - 1].value;
          const secondLastDay = statsQuery.data.new_txns_multichain_window.chart[statsQuery.data.new_txns_multichain_window.chart.length - 2].value;
          if (Number(secondLastDay) !== 0) {
            return Number(((Number(lastDay) - Number(secondLastDay)) / Number(secondLastDay) * 100).toFixed(2));
          }
        }
        return undefined;
      })(),
      // FIXME use non-navigation icon
      icon: <IconSvg name="navigation/transactions" boxSize={ 6 } bgColor="indicator.transactions" borderRadius="base" color="white"/>,
      hint: (() => {
        if (statsQuery.data.new_txns_multichain_window?.info) {
          return statsQuery.data.new_txns_multichain_window.info.description;
        }
        return `Number of transactions yesterday (0:00 - 23:59 UTC). The chart displays daily transactions for the past 30 days.`;
      })(),
    },
    market && hasCoinPrice && parentChainStatsQuery.data && {
      id: 'coin_price' as const,
      title: `${ market.symbol } price`,
      value: (() => {
        if (parentChainStatsQuery.data.coin_price != null) {
          return '$' + Number(parentChainStatsQuery.data.coin_price).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 });
        }
        return '$N/A';
      })(),
      valueDiff: parentChainStatsQuery.data.coin_price_change_percentage ?? undefined,
      icon: <Image
        boxSize={ 6 }
        borderRadius="base"
        src={ parentChainStatsQuery.data.coin_image || undefined }
        alt={ `${ market.symbol } token logo` }
        fallback={ <TokenLogoPlaceholder/> }
      />,
      hint: `${ market.symbol } token daily price in USD.`,
    },
    market && hasMarketCap && parentChainStatsQuery.data && {
      id: 'market_cap' as const,
      title: `${ market.symbol } market cap`,
      value: (() => {
        if (parentChainStatsQuery.data.market_cap != null) {
          return '$' + Number(parentChainStatsQuery.data.market_cap).toLocaleString(undefined, { maximumFractionDigits: 2, notation: 'compact' });
        }
        return '$N/A';
      })(),
      icon: <IconSvg name="globe" boxSize={ 6 } bgColor="indicator.addresses" borderRadius="base" color="white"/>,
      // eslint-disable-next-line max-len
      hint: 'The total market value of a cryptocurrency\'s circulating supply. It is analogous to the free-float capitalization in the stock market. Market Cap = Current Price x Circulating Supply.',
    },
  ]
    .filter(Boolean)
    .filter(isIndicatorEnabled)
    .sort(sortIndicators)
  ), [ hasCoinPrice, hasMarketCap, market, parentChainStatsQuery.data, statsQuery.data ]);

  const [ selectedIndicatorId, selectIndicatorId ] = React.useState(indicators[0]?.id);
  const selectedIndicator = indicators.find(({ id }) => id === selectedIndicatorId) || indicators[0];

  const chartQuery = useChartDataQuery({ indicatorId: selectedIndicator?.id || selectedIndicatorId });

  return (
    <ChainIndicatorsContainer>
      { selectedIndicator && (
        <ChainIndicatorsChart
          isLoading={ statsQuery.isPlaceholderData || chartQuery.isPending }
          title={ selectedIndicator.title }
          hint={ selectedIndicator.hint }
          value={ selectedIndicator?.value }
          valueDiff={ selectedIndicator?.valueDiff }
          chartQuery={ chartQuery }
        />
      ) }
      { marketUnavailable && (
        <Text role="status" fontSize="xs" color="text.secondary">
          { market?.symbol ? `${ market.symbol } market statistics unavailable.` : 'Native asset market statistics unavailable.' }
        </Text>
      ) }
      <ChainIndicatorsList
        indicators={ indicators }
        isLoading={ statsQuery.isPlaceholderData || (Boolean(market) && parentChainStatsQuery.isPending) }
        selectedId={ selectedIndicator?.id || selectedIndicatorId }
        onItemClick={ selectIndicatorId }
      />
    </ChainIndicatorsContainer>
  );
};

export default React.memo(ChainIndicators);
