import { Flex, chakra } from '@chakra-ui/react';
import React from 'react';

interface Props {
  className?: string;
  children: React.ReactNode;
}

const MainColumn = ({ children, className }: Props) => {
  return (
    <Flex
      className={ className }
      flexDir="column"
      flexGrow={ 1 }
      w="100%"
      minW={ 0 }
      paddingX={{ base: 4, lg: 6 }}
      paddingRight={{ '2xl': 6 }}
      paddingTop={{ base: '12px', lg: 6 }} // 12px is top padding of content area
      paddingBottom={ 8 }
    >
      { children }
    </Flex>
  );
};

export default React.memo(chakra(MainColumn));
