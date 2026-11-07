import { Box } from '@chakra-ui/react';
import React from 'react';

import type { GasPriceInfo, GasPrices } from 'types/api/stats';

import GasPrice from 'ui/shared/gas/GasPrice';
import { ScanStatCard } from 'ui/shared/scan';

interface Props {
  type: keyof GasPrices;
  data: GasPriceInfo;
  isLoading: boolean;
}

export const GAS_PRICE_TITLES: Record<keyof GasPrices, string> = {
  fast: 'Fast',
  average: 'Normal',
  slow: 'Slow',
};

const GasTrackerPriceSnippet = ({ data, type, isLoading }: Props) => (
  <Box as="li" listStyleType="none" minW={ 0 }>
    <ScanStatCard
      label={ GAS_PRICE_TITLES[type] }
      value={ data.price === null && data.fiat_price === null ? '—' : <GasPrice data={ data } textStyle="statValue"/> }
      secondary={ data.price !== null && data.fiat_price !== null ? <GasPrice data={ data } unitMode="secondary"/> : undefined }
      isLoading={ isLoading }
    />
  </Box>
);

export default React.memo(GasTrackerPriceSnippet);
