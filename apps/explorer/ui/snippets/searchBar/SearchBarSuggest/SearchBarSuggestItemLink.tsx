import React from 'react';

import type { LinkProps } from 'toolkit/chakra/link';
import { Link } from 'toolkit/chakra/link';

interface Props extends LinkProps {};

const SearchBarSuggestItemLink = React.forwardRef<HTMLAnchorElement, Props>(({ children, ...rest }, ref) => {
  return (
    <Link
      ref={ ref }
      py={ 2 }
      minH="44px"
      justifyContent="center"
      color="text.primary"
      px={{ base: 0, lg: 1 }}
      display="flex"
      flexDir="column"
      alignItems="stretch"
      variant="plain"
      rowGap={ 1 }
      borderColor="hairline"
      borderBottomWidth="1px"
      _last={{
        borderBottomWidth: '0',
      }}
      _hover={{
        bgColor: 'accent.soft',
      }}
      textStyle="sm"
      { ...rest }
    >
      { children }
    </Link>
  );
});

export default SearchBarSuggestItemLink;
