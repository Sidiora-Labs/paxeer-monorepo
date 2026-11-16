import { Box, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import { Skeleton } from 'toolkit/chakra/skeleton';

type Props = {
  name: string;
  value: string;
  icon: React.ReactNode;
  valueSecondary?: string;
  isLoading: boolean;
  contentAfter?: React.ReactNode;
};

const TokenBalancesItem = ({ name, icon, value, valueSecondary, isLoading, contentAfter }: Props) => {

  return (
    <Box px={ 3 } py={ 2.5 } bgColor="bg.surface" borderRadius="base" minW={ 0 }>
      <Text color="text.secondary" textStyle="xs" fontWeight="medium" mb={ 1 }>{ name }</Text>
      <Flex alignItems="center" minW={ 0 }>
        { icon }
        <Skeleton loading={ isLoading } fontWeight="medium" display="flex" flexWrap="wrap" columnGap={ 1 } minW={ 0 } ml={ 2 }>
          <Text maxW="100%" overflowWrap="anywhere">{ value }</Text>
          { Boolean(valueSecondary) && <Text maxW="100%" color="text.secondary" overflowWrap="anywhere">({ valueSecondary })</Text> }
        </Skeleton>
        { contentAfter }
      </Flex>
    </Box>
  );
};

export default React.memo(TokenBalancesItem);
