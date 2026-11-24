import { chakra } from '@chakra-ui/react';
import React from 'react';

import getNetworkUtilizationParams from 'lib/networks/getNetworkUtilizationParams';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { mdash } from 'toolkit/utils/htmlEntities';
import { getNetworkUtilizationColor } from 'ui/blocks/networkUtilizationColors';

interface Props {
  percentage: number;
  isLoading: boolean;
}

const GasTrackerNetworkUtilization = ({ percentage, isLoading }: Props) => {
  const { load } = getNetworkUtilizationParams(percentage);

  return (
    <Skeleton loading={ isLoading } whiteSpace="pre-wrap" textWrap="balance">
      <span>Network utilization </span>
      <chakra.span color={ getNetworkUtilizationColor(load) }>{ percentage.toFixed(2) }% { mdash } { load } load</chakra.span>
    </Skeleton>
  );
};

export default React.memo(GasTrackerNetworkUtilization);
