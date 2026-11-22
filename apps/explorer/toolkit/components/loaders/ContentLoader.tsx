import type { BoxProps } from '@chakra-ui/react';
import { Box, Text } from '@chakra-ui/react';
import React from 'react';

interface Props extends BoxProps {
  text?: string;
}

export const ContentLoader = React.memo(({ text, ...props }: Props) => {
  return (
    <Box display="inline-block" { ...props }>
      <Box
        width="100%"
        height={ 'var(--chakra-spacing-1\\.5)' }
        position="relative"
        _after={{
          content: `" "`,
          position: 'absolute',
          width: 'var(--chakra-spacing-15)',
          height: 'var(--chakra-spacing-1\\.5)',
          _motionSafe: { animation: 'fromLeftToRight var(--chakra-durations-loader) var(--chakra-easings-standard) infinite alternate' },
          left: '0%',
          top: 0,
          backgroundColor: 'blue.300',
          borderRadius: 'full',
        }}
      />
      <Text mt={ 6 } color="text.secondary">
        { text || 'Loading data, please wait...' }
      </Text>
    </Box>
  );
});
