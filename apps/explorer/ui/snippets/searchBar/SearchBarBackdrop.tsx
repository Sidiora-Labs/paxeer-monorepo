import { Box } from '@chakra-ui/react';
import React from 'react';

interface Props {
  isOpen: boolean;
}

const SearchBarBackdrop = ({ isOpen }: Props) => {
  return (
    <Box
      position="fixed"
      top={ 0 }
      left={ 0 }
      w="100vw"
      h="100vh"
      bgColor="canvas"
      opacity={ 0.6 }
      zIndex="overlay"
      display={{ base: 'none', lg: isOpen ? 'block' : 'none' }}
    />
  );
};

export default React.memo(SearchBarBackdrop);
