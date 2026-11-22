import { chakra } from '@chakra-ui/react';
import React from 'react';

import { Button } from 'toolkit/chakra/button';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import IconSvg from 'ui/shared/IconSvg';

interface Props {
  columnName: string;
  isLoading?: boolean;
  selected?: boolean;
  className?: string;
  children: React.ReactNode;
  value?: string;
}

const TableColumnFilterWrapper = ({ columnName, className, children, isLoading, selected, value }: Props) => {
  return (
    <PopoverRoot>
      <PopoverTrigger>
        <Button
          display="inline-flex"
          aria-label={ `filter by ${ columnName }` }
          variant="icon_secondary"
          borderWidth="0"
          disabled={ isLoading }
          selected={ selected }
          borderRadius="sm"
          size="sm"
          textStyle="sm"
          fontWeight="medium"
          padding={ 0 }
        >
          <IconSvg name="filter" boxSize={ 5 }/>
          { Boolean(value) && <chakra.span>{ value }</chakra.span> }
        </Button>
      </PopoverTrigger>
      <PopoverContent className={ className }>
        <PopoverBody display="flex" flexDir="column" rowGap={ 3 }>
          { children }
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default chakra(TableColumnFilterWrapper);
