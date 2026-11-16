import { Box } from '@chakra-ui/react';
import React from 'react';

interface Props {
  label: string;
  value: string;
}

const BlockCountdownTimerItem = ({ label, value }: Props) => {
  return (
    <Box
      minW={ 0 }
      textAlign="center"
      overflowWrap="anywhere"
    >
      <Box
        textStyle="statValue"
        w="100%"
      >{ value }</Box>
      <Box textStyle="small" mt={ 1 } color="text.secondary">{ label }</Box>
    </Box>
  );
};

export default React.memo(BlockCountdownTimerItem);
