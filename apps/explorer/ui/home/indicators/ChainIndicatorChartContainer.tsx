import { chakra, Box } from '@chakra-ui/react';
import React from 'react';

import type { TimeChartData } from 'toolkit/components/charts/types';

import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import DataFetchAlert from 'ui/shared/DataFetchAlert';

import ChainIndicatorChartContent from './ChainIndicatorChartContent';

type Props = {
  data: TimeChartData;
  isError: boolean;
  isPending: boolean;
  caption?: string;
};

const ChainIndicatorChartContainer = ({ data, isError, isPending, caption }: Props) => {

  if (isPending) {
    return <ContentLoader mt="auto" fontSize="xs"/>;
  }

  if (isError) {
    return <DataFetchAlert fontSize="xs"/>;
  }

  if (!data[0] || data[0].items.length < 2) {
    return <chakra.span textStyle="small" color="text.muted" aria-label="Network chart unavailable">—</chakra.span>;
  }

  return (
    <Box mx="calc(var(--chakra-spacing-2\.5) * -1)" my="calc(var(--chakra-spacing-1\.25) * -1)"
      h="calc(100% + var(--chakra-spacing-2\.5))" w="calc(100% + var(--chakra-spacing-5))">
      <ChainIndicatorChartContent data={ data } caption={ caption }/>
    </Box>
  );
};

export default React.memo(ChainIndicatorChartContainer);
