import { chakra, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { Hint } from 'toolkit/components/Hint/Hint';
import { mdash } from 'toolkit/utils/htmlEntities';
import IconSvg from 'ui/shared/IconSvg';

import ChainIndicatorChartContainer from './ChainIndicatorChartContainer';
import type { UseFetchChartDataResult } from './useChartDataQuery';

interface Props {
  isLoading: boolean;
  value?: string;
  valueDiff?: number;
  chartQuery: UseFetchChartDataResult;
  title: string;
  hint?: string;
}

const ChainIndicatorsChart = ({ isLoading: isLoadingProp, value, valueDiff, chartQuery, title, hint }: Props) => {
  const isLoading = isLoadingProp || chartQuery.isPending;
  const data = chartQuery.data;

  const valueTitleElement = (() => {
    if (value === undefined) {
      return null;
    }

    if (isLoading) {
      return <Skeleton loading h="28px" w="160px"/>;
    }

    if (value.includes('N/A')) {
      return <Text textStyle="statValue" color="text.muted">{ mdash }</Text>;
    }

    return <Text textStyle="statValue">{ value }</Text>;
  })();

  const valueDiffElement = (() => {
    if (valueDiff === undefined || (!isLoading && value?.includes('N/A'))) {
      return null;
    }

    const diffColor = valueDiff >= 0 ? 'stat.indicator.up' : 'stat.indicator.down';

    return (
      <Skeleton loading={ isLoading } display="flex" alignItems="center" color={ diffColor } ml={ 2 }>
        <IconSvg name="arrows/up-head" boxSize={ 5 } mr={ 1 } transform={ valueDiff < 0 ? 'rotate(180deg)' : 'rotate(0)' }/>
        <Text color={ diffColor } fontWeight="600">{ valueDiff }%</Text>
      </Skeleton>
    );
  })();

  if (chartQuery.isError) {
    return (
      <Flex flexDirection="column" gap={ 2 } color="text.muted" role="status">
        <Text textStyle="label">{ title }</Text>
        <Text textStyle="statValue">—</Text>
        <Text textStyle="small">Network chart unavailable</Text>
      </Flex>
    );
  }

  return (
    <Flex flexGrow={ 1 } flexDir="column" h="100%" data-label="chain-indicator-chart" data-points={ data[0]?.items.length ?? 0 }>
      <Skeleton loading={ isLoading } display="flex" alignItems="center" w="fit-content" columnGap={ 1 }>
        <chakra.span
          data-title
          textStyle="label"
          color="text.muted"
        >
          { title }
        </chakra.span>
        { hint && <Hint label={ hint } boxSize={ 4 }/> }
      </Skeleton>
      { valueTitleElement && (
        <Flex mt={ 1 } alignItems="flex-end">
          { valueTitleElement }
          { valueDiffElement }
        </Flex>
      ) }
      <Flex mt={ 2 } h={{ base: '160px', lg: '200px' }} alignItems="flex-start" flexGrow={ 1 }>
        <ChainIndicatorChartContainer data={ data } isError={ chartQuery.isError } isPending={ isLoading }/>
      </Flex>
    </Flex>
  );
};

export default React.memo(ChainIndicatorsChart);
