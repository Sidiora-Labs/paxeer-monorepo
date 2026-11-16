import { chakra } from '@chakra-ui/react';
import React from 'react';

import { Button } from 'toolkit/chakra/button';
import type { PopoverContentProps } from 'toolkit/chakra/popover';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import IconSvg from 'ui/shared/IconSvg';

interface Props extends PopoverContentProps {
  columnName: string;
  isLoading?: boolean;
  selected?: boolean;
  value?: string;
}

const ActivityColumnFilter = ({ columnName, children, isLoading, selected, value, ...props }: Props) => (
  <PopoverRoot autoFocus>
    <PopoverTrigger>
      <Button
        display="inline-flex"
        aria-label={ `filter by ${ columnName }` }
        variant="icon_secondary"
        borderWidth={ 0 }
        minH="touchTarget"
        minW="touchTarget"
        disabled={ isLoading }
        selected={ selected }
        borderRadius="control"
        size="sm"
        textStyle="sm"
        fontWeight="medium"
        px={ 1 }
      >
        <IconSvg name="filter" boxSize={ 5 }/>
        { Boolean(value) && <chakra.span>{ value }</chakra.span> }
      </Button>
    </PopoverTrigger>
    <PopoverContent
      { ...props }
      maxH="min(var(--available-height, 100dvh), calc(100dvh - var(--chakra-spacing-8)))"
      overflowY="auto"
      overscrollBehavior="contain"
      css={{
        '& button, & input:not([type="checkbox"]):not([type="radio"]), & [data-scope="checkbox"][data-part="root"], & [data-scope="radio"][data-part="root"]': {
          minW: 'touchTarget', minH: 'touchTarget',
        },
      }}
      aria-label={ `Filter by ${ columnName }` }
    >
      <PopoverBody display="flex" flexDir="column" rowGap={ 3 }>
        { children }
      </PopoverBody>
    </PopoverContent>
  </PopoverRoot>
);

export default ActivityColumnFilter;
