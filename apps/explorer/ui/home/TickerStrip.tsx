import { Flex, Text } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import type { TokenInfo } from 'types/api/token';

import { route } from 'nextjs-routes';

import useApiQuery from 'lib/api/useApiQuery';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';

function tokenChange(token: TokenInfo) {
  if (!('price_change_percentage_24h' in token)) {
    return undefined;
  }
  const value = token.price_change_percentage_24h;
  if (typeof value !== 'number' && (typeof value !== 'string' || value.trim() === '')) {
    return undefined;
  }
  const change = new BigNumber(value);
  return change.isFinite() ? change : undefined;
}

const TickerStrip = () => {
  const { data, isError, isPlaceholderData, isPending } = useApiQuery('general:tokens', {
    queryOptions: { refetchOnMount: false },
  });
  const tokens = data?.items?.filter((token) => token.exchange_rate != null) ?? [];

  const isLoading = isPending || isPlaceholderData;

  if (isError || (!isLoading && tokens.length === 0)) {
    return null;
  }

  return (
    <Flex
      as="section"
      aria-label="Token prices"
      tabIndex={ isLoading ? -1 : 0 }
      aria-busy={ isLoading }
      data-label="ticker-strip"
      overflowX="auto"
      minH="touchTarget"
      css={{ '& a': { display: 'inline-flex', alignItems: 'center', minWidth: 'touchTarget',
        minHeight: 'touchTarget', _motionReduce: { transitionProperty: 'none' } } }}
      overscrollBehaviorX="contain"
      borderBottomWidth="1"
      borderColor="border.divider"
      gap={ 6 }
      py={ 3 }
    >
      { isLoading && <Skeleton loading h="touchTarget" w="full"/> }
      { !isLoading && tokens.map((token) => {
        const price = new BigNumber(token.exchange_rate!);
        const change = tokenChange(token);
        let changeColor = 'text.muted';
        if (change && !change.isZero()) {
          changeColor = change.isPositive() ? 'positive' : 'negative';
        }
        return (
          <Flex key={ token.address_hash } data-ticker-token={ token.address_hash } alignItems="center" gap={ 2 } flexShrink={ 0 }>
            <Link href={ route({ pathname: '/token/[hash]', query: { hash: token.address_hash } }) } textStyle="small" color="text.primary">
              { token.symbol ?? '—' }
            </Link>
            <Text data-price textStyle="number" color={ price.isFinite() ? 'text.primary' : 'text.muted' }>
              { price.isFinite() ? `$${ price.toFormat(Math.max(2, price.decimalPlaces() ?? 2), BigNumber.ROUND_HALF_UP) }` : '—' }
            </Text>
            <Text data-change textStyle="number" color={ changeColor } aria-label="24 hour change">
              { change ? `${ change.isPositive() && !change.isZero() ? '+' : '' }${ change.toFormat() }%` : '—' }
            </Text>
          </Flex>
        );
      }) }
    </Flex>
  );
};

export default React.memo(TickerStrip);
