import { Box, Text } from '@chakra-ui/react';
import React from 'react';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { Hint } from 'toolkit/components/Hint/Hint';

interface Props {
  label: string;
  children: React.ReactNode;
  hint?: string;
  isLoading?: boolean;
}

const StatTile = ({ label, children, isLoading, hint }: Props) => (
  <Box bg="bg.surface" borderWidth="1px" borderColor="hairline" borderRadius="card" p={ 4 } minW={ 0 } data-stat-tile={ label }>
    <Text textStyle="label" color="text.muted" textTransform="uppercase" mb={ 2 } display="flex" alignItems="center" gap={ 2 }>
      { label }
      { hint && <Hint inline label={ hint } isLoading={ isLoading }/> }
    </Text>
    <Skeleton loading={ isLoading } textStyle="statValue" data-stat-value>
      { children }
    </Skeleton>
  </Box>
);

export default StatTile;
