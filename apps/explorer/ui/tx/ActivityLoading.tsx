import { Box, Text } from '@chakra-ui/react';
import React from 'react';

import { Skeleton } from 'toolkit/chakra/skeleton';

interface Props {
  label: string;
}

const ActivityLoading = ({ label }: Props) => (
  <Box role="status" aria-busy="true" w="full" minW={ 0 }>
    <Text srOnly>{ label }</Text>
    <Skeleton loading h="var(--chakra-spacing-96)" w="full" borderRadius="card"/>
  </Box>
);

export default ActivityLoading;
