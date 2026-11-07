import { Grid } from '@chakra-ui/react';
import type { UseQueryResult } from '@tanstack/react-query';
import BigNumber from 'bignumber.js';
import React from 'react';

import type { TokenInfo } from 'types/api/token';

import type { ResourceError } from 'lib/api/resources';
import throwOnResourceLoadError from 'lib/errors/throwOnResourceLoadError';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import StatTile from 'ui/shared/Page/StatTile';

interface Props {
  tokenQuery: UseQueryResult<TokenInfo, ResourceError<unknown>>;
}

export function formatMarketValue(value: string | null | undefined, currency: boolean = true): string {
  if (value === null || value === undefined || value === '') {
    return '—';
  }
  const number = BigNumber(value);
  if (!number.isFinite()) {
    return '—';
  }
  return `${ currency ? '$' : '' }${ number.toFormat() }`;
}

const TokenMarkets = ({ tokenQuery }: Props) => {
  throwOnResourceLoadError(tokenQuery);
  const data = tokenQuery.isPlaceholderData ? undefined : tokenQuery.data;

  return (
    <DetailedInfo.Section label="MARKETS">
      <Grid templateColumns={{ base: 'minmax(0, 1fr)', sm: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(4, minmax(0, 1fr))' }} gap={ 4 } data-token-markets>
        <StatTile label="Price" isLoading={ tokenQuery.isPlaceholderData }>{ formatMarketValue(data?.exchange_rate) }</StatTile>
        <StatTile label="Market cap" isLoading={ tokenQuery.isPlaceholderData }>{ formatMarketValue(data?.circulating_market_cap) }</StatTile>
        <StatTile label="Volume 24h" isLoading={ tokenQuery.isPlaceholderData }>{ formatMarketValue(data?.volume_24h) }</StatTile>
        <StatTile label="Holders" isLoading={ tokenQuery.isPlaceholderData }>{ formatMarketValue(data?.holders_count, false) }</StatTile>
      </Grid>
    </DetailedInfo.Section>
  );
};

export default TokenMarkets;
