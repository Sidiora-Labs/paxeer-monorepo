import { Box, Grid } from '@chakra-ui/react';
import React from 'react';

import type { GasPrices } from 'types/api/stats';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';

import GasTrackerPriceSnippet, { GAS_PRICE_TITLES } from './GasTrackerPriceSnippet';

interface Props {
  prices: GasPrices;
  isLoading: boolean;
}

const PRICE_ORDER: Array<keyof GasPrices> = [ 'fast', 'average', 'slow' ];

const GasTrackerPrices = ({ prices, isLoading }: Props) => (
  <Box>
    <Grid as="ul" templateColumns={{ base: '1fr', lg: 'repeat(3, minmax(0, 1fr))' }} gap={ 4 } mb={ 8 }>
      { PRICE_ORDER.map((type) => prices[type] && <GasTrackerPriceSnippet key={ type } type={ type } data={ prices[type] } isLoading={ isLoading }/>) }
    </Grid>
    <DetailedInfo.Section label="Gas fee estimates">
      <Box overflowX="auto">
        <TableRoot variant="scan" css={{ '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'canvas' } }} minW="480px">
          <TableHeader><TableRow>
            <TableColumnHeader>Speed</TableColumnHeader>
            <TableColumnHeader isNumeric>Base fee (Gwei)</TableColumnHeader>
            <TableColumnHeader isNumeric>Priority fee (Gwei)</TableColumnHeader>
            <TableColumnHeader isNumeric>Confirmation time (s)</TableColumnHeader>
          </TableRow></TableHeader>
          <TableBody>
            { PRICE_ORDER.map((type) => {
              const price = prices[type];
              if (!price) {
                return null;
              }
              return (
                <TableRow key={ type }>
                  <TableCell><Skeleton loading={ isLoading }>{ GAS_PRICE_TITLES[type] }</Skeleton></TableCell>
                  <TableCell isNumeric><Skeleton loading={ isLoading }>{ price.base_fee?.toLocaleString(undefined, { maximumFractionDigits: 3 }) ?? '—' }</Skeleton></TableCell>
                  <TableCell isNumeric><Skeleton loading={ isLoading }>{ price.priority_fee?.toLocaleString(undefined, { maximumFractionDigits: 3 }) ?? '—' }</Skeleton></TableCell>
                  <TableCell isNumeric><Skeleton loading={ isLoading }>{ price.time === null ? '—' : (price.time / 1000).toLocaleString(undefined, { maximumFractionDigits: 1 }) }</Skeleton></TableCell>
                </TableRow>
              );
            }) }
          </TableBody>
        </TableRoot>
      </Box>
    </DetailedInfo.Section>
  </Box>
);

export default React.memo(GasTrackerPrices);
