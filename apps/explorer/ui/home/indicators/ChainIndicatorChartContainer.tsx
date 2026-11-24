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
    <Box h="full" w="full" minW={ 0 }>
      <ChainIndicatorChartContent data={ data } caption={ caption }/>
    </Box>
  );
};

export default React.memo(ChainIndicatorChartContainer);
