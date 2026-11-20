import { Box } from '@chakra-ui/react';
import React from 'react';

import { TruncatedText } from 'toolkit/components/truncation/TruncatedText';

interface Props {
  label: string;
  value: string;
}

const BlockCountdownTimerItem = ({ label, value }: Props) => {
  return (
    <Box
      minW={ 0 }
      flex="1 1 0"
      textAlign="center"
      overflowWrap="anywhere"
    >
      <TruncatedText
        text={ value }
        fontFamily="heading"
        fontSize={{ base: 'fluidDisplay', lg: '5xl' }}
        lineHeight="var(--chakra-spacing-12)"
        fontWeight="semibold"
        w="100%"
      />
      <Box textStyle="body" mt={ 1 } color="text.secondary">{ label }</Box>
    </Box>
  );
};

export default React.memo(BlockCountdownTimerItem);
