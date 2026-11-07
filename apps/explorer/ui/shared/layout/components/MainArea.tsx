import { Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import { CONTENT_MAX_WIDTH } from '../utils';

interface Props {
  children: React.ReactNode;
  className?: string;
}

const MainArea = ({ children, className }: Props) => {
  return (
    <Flex
      className={ className }
      w="100%"
      minW={ 0 }
      maxW={ `${ CONTENT_MAX_WIDTH }px` }
      m="0 auto"
      minH="calc(100dvh - 56px)"
      alignItems="stretch"
    >
      { children }
    </Flex>
  );
};

export default React.memo(chakra(MainArea));
