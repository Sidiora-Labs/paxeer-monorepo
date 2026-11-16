import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

type Props = {
  children: React.ReactNode;
  className?: string;
};

const NFTItemContainer = ({ children, className }: Props) => {
  return (
    <Box
      w="100%"
      minW={ 0 }
      border="1px solid"
      borderColor="border.divider"
      borderRadius="card"
      p={ 2.5 }
      fontSize="sm"
      fontWeight="medium"
      textStyle="small"
      className={ className }
    >
      { children }
    </Box>
  );
};

export default chakra(NFTItemContainer);
