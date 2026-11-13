import { Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import dayjs from 'lib/date/dayjs';
import useIsMobile from 'lib/hooks/useIsMobile';
import { HOMEPAGE_STATS } from 'stubs/stats';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import GasInfoTooltip from 'ui/shared/gas/GasInfoTooltip';
import GasPrice from 'ui/shared/gas/GasPrice';
import TextSeparator from 'ui/shared/TextSeparator';

import GetGasButton from './GetGasButton';
import TopBarCoinPrice from './TopBarCoinPrice';

const TopBarStats = ({ showGasInfo = true }: { showGasInfo?: boolean }) => {
  const isMobile = useIsMobile();

  const { data, isPlaceholderData, isError, refetch, dataUpdatedAt } = useApiQuery('general:stats', {
    queryOptions: {
      placeholderData: HOMEPAGE_STATS,
      refetchOnMount: false,
    },
  });

  React.useEffect(() => {
    if (isPlaceholderData || !data?.gas_price_updated_at) {
      return;
    }

    const endDate = dayjs(dataUpdatedAt).add(data.gas_prices_update_in, 'ms');
    const timeout = endDate.diff(dayjs(), 'ms');

    if (timeout <= 0) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      refetch();
    }, timeout);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [ isPlaceholderData, data?.gas_price_updated_at, dataUpdatedAt, data?.gas_prices_update_in, refetch ]);

  if (isError) {
    return <div/>;
  }

  const nativeSymbol = config.chain.currency.symbol;
  const secondarySymbol = config.chain.secondaryCoin.symbol;
  const hasNativeCoinPrice = Boolean(nativeSymbol) && !config.UI.nativeCoinPrice.isHidden;
  const hasSecondaryCoinPrice = Boolean(secondarySymbol);
  const hasGasInfo = showGasInfo && data?.gas_prices && data.gas_prices.average !== null && config.features.gasTracker.isEnabled && !isMobile;

  return (
    <Flex
      alignItems="center"
      fontWeight={ 500 }
      flexWrap="wrap"
      columnGap={ 3 }
      rowGap={ 1 }
    >
      { hasNativeCoinPrice && nativeSymbol && (
        <TopBarCoinPrice
          symbol={ nativeSymbol }
          price={ data?.coin_price }
          change={ data?.coin_price_change_percentage }
          isLoading={ isPlaceholderData }
        />
      ) }
      { hasSecondaryCoinPrice && secondarySymbol && (
        <TopBarCoinPrice
          symbol={ secondarySymbol }
          price={ data?.secondary_coin_price }
          change={ data?.secondary_coin_price_change_percentage }
          isLoading={ isPlaceholderData }
        />
      ) }
      { (hasNativeCoinPrice || hasSecondaryCoinPrice) && hasGasInfo && <TextSeparator/> }
      { hasGasInfo && (
        <>
          <Skeleton loading={ isPlaceholderData } whiteSpace="pre-wrap">
            <chakra.span color="text.secondary">Gas: </chakra.span>
            <GasInfoTooltip data={ data } dataUpdatedAt={ dataUpdatedAt } placement={ !data?.coin_price ? 'bottom-start' : undefined }>
              <Link color="accent">
                <GasPrice data={ data.gas_prices?.average ?? null }/>
              </Link>
            </GasInfoTooltip>
          </Skeleton>
          { !isPlaceholderData && <GetGasButton/> }
        </>
      ) }
    </Flex>
  );
};

export default React.memo(TopBarStats);
