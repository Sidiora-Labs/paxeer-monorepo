import { Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import type { GeneralApiMiscResourcePayload } from 'lib/api/services/general/misc';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { mdash } from 'toolkit/utils/htmlEntities';

interface Props {
  symbol: string;
  price?: GeneralApiMiscResourcePayload<'general:stats'>['secondary_coin_price'];
  change?: GeneralApiMiscResourcePayload<'general:stats'>['secondary_coin_price_change_percentage'];
  isLoading: boolean;
}

const TopBarCoinPrice = ({ symbol, price, change, isLoading }: Props) => {
  const hasPrice = price != null && price !== '' && Number.isFinite(Number(price));
  const hasChange = change != null && Number.isFinite(change);
  let changeTrend = 'flat';
  let changeColor = 'text.muted';
  if (hasChange) {
    const isRising = change >= 0;
    changeTrend = isRising ? 'up' : 'down';
    changeColor = isRising ? 'positive' : 'negative';
  }

  return (
    <Flex alignItems="baseline" gap={ 1 } flexWrap="wrap" minW={ 0 } maxW="100%" role="group" aria-label={ `${ symbol } market price` }>
      <Skeleton loading={ isLoading } minW={ 0 } maxW="100%">
        <chakra.span color="text.secondary">{ symbol } Price: </chakra.span>
        <chakra.span data-price whiteSpace="nowrap" textStyle="number" color={ hasPrice ? 'accent' : 'text.muted' }>
          { hasPrice ? '$' + Number(price).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 }) : mdash }
        </chakra.span>
      </Skeleton>
      <Skeleton loading={ isLoading } minW={ 0 } maxW="100%">
        <chakra.span
          data-change
          data-trend={ changeTrend }
          aria-label={ `${ symbol } price change in 24 hours` }
          textStyle="number"
          color={ changeColor }
        >
          { hasChange ? `${ change.toFixed(2) }%` : mdash }
        </chakra.span>
      </Skeleton>
    </Flex>
  );
};

export default React.memo(TopBarCoinPrice);
