import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

import { CONTENT_MAX_WIDTH } from '../utils';

interface Props {
  className?: string;
  children: React.ReactNode;
}

const Content = ({ children, className }: Props) => {
  return (
    <Box
      as="main"
      data-label="content"
      className={ className }
      pt={ 0 }
      w="100%"
      minW={ 0 }
      maxW={ `${ CONTENT_MAX_WIDTH }px` }
      mx="auto"
      bgColor="bg.primary"
      flexGrow={ 1 }
    >
      { children }
    </Box>
  );
};

export default React.memo(chakra(Content));
