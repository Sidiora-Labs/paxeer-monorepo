import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

interface Props {
  children: React.ReactNode;
  className?: string;
}

const Container = ({ children, className }: Props) => {
  return (
    <Box
      className={ className }
      width="100%"
      minWidth={ 0 }
      m="0 auto"
      bgColor="bg.primary"
    >
      { children }
    </Box>
  );
};

export default React.memo(chakra(Container));
