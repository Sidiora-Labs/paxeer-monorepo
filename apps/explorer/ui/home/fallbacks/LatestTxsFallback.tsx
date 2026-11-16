import type { BoxProps } from '@chakra-ui/react';
import { Box, HStack, VStack } from '@chakra-ui/react';
import React from 'react';

import { TableBody, TableCell, TableRoot, TableRow } from 'toolkit/chakra/table';
import FallbackBox from 'ui/shared/fallbacks/FallbackBox';
import IconSvg from 'ui/shared/IconSvg';

const LatestTxsFallback = (props: BoxProps) => {
  return (
    <Box { ...props }>
      <Box color="text.secondary" textStyle="body">Failed to load data. Please try again later.</Box>
      <TableRoot mt={ 3 } borderTopWidth="1" borderColor="border.divider">
        <TableBody>
          { Array.from({ length: 2 }).map((_, index) => (
            <TableRow key={ index }>
              <TableCell pl={ 0 } pr={{ base: 3, lg: 4 }} w={{ base: 'var(--chakra-spacing-62\\.5)', lg: '33%' }}>
                <VStack alignItems="stretch">
                  <HStack>
                    <IconSvg name="transactions" boxSize={ 5 } color="text.muted"/>
                    <FallbackBox w="var(--chakra-spacing-24)" bgColor="accent.soft"/>
                    <FallbackBox w="var(--chakra-spacing-24)" bgColor="raised"/>
                  </HStack>
                  <FallbackBox bgColor="raised" w="100%"/>
                </VStack>
              </TableCell>
              <TableCell px={{ base: 3, lg: 4 }}>
                <VStack alignItems="stretch">
                  <FallbackBox bgColor="raised" w="100%" my={ 1 }/>
                  <FallbackBox bgColor="raised" w="100%"/>
                </VStack>
              </TableCell>
              <TableCell pl={{ base: 3, lg: 4 }} pr={{ base: 0, lg: 2 }}>
                <VStack alignItems="stretch">
                  <FallbackBox bgColor="raised" w="100%" my={ 1 }/>
                  <FallbackBox bgColor="raised" w="100%"/>
                </VStack>
              </TableCell>
            </TableRow>
          )) }
        </TableBody>
      </TableRoot>
    </Box>
  );
};

export default React.memo(LatestTxsFallback);
