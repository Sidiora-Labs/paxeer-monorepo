import { Flex, chakra } from '@chakra-ui/react';
import React from 'react';

interface Props {
  children: React.ReactNode;
  className?: string;
  animation?: string;
}

const ListItemMobile = ({ children, className, animation }: Props) => {
  return (
    <Flex
      animation={ animation }
      rowGap={ 6 }
      alignItems="flex-start"
      flexDirection="column"
      paddingY={ 4 }
      w="100%"
      minW={ 0 }
      data-list-item-mobile
      borderColor="hairline"
      bg="canvas"
      color="text.primary"
      textStyle="small"
      borderTopWidth="1px"
      _last={{
        borderBottomWidth: '1px',
      }}
      className={ className }
      fontSize="13px"
      lineHeight="18px"
    >
      { children }
    </Flex>
  );
};

export default chakra(ListItemMobile);
