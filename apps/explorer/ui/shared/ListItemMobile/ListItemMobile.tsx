import { Flex, chakra } from '@chakra-ui/react';
import React from 'react';

interface Props {
  children: React.ReactNode;
  className?: string;
  animation?: string;
  'aria-busy'?: React.AriaAttributes['aria-busy'];
}

const ListItemMobile = ({ children, className, animation, ...rest }: Props) => {
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
      { ...rest }
    >
      { children }
    </Flex>
  );
};

export default chakra(ListItemMobile);
