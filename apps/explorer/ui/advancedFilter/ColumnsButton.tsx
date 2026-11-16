import { chakra } from '@chakra-ui/react';
import React from 'react';

import { Button } from 'toolkit/chakra/button';
import { Checkbox, CheckboxGroup } from 'toolkit/chakra/checkbox';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import type { ColumnsIds } from 'ui/advancedFilter/constants';
import { TABLE_COLUMNS } from 'ui/advancedFilter/constants';
import IconSvg from 'ui/shared/IconSvg';

interface Props {
  columns: Record<ColumnsIds, boolean>;
  onChange: (val: Record<ColumnsIds, boolean>) => void;
}

const ColumnsButton = ({ columns, onChange }: Props) => {
  const handleValueChange = React.useCallback((value: Array<string>) => {
    const newCols = value.reduce((acc, key) => {
      acc[key as ColumnsIds] = true;
      return acc;
    }, {} as Record<ColumnsIds, boolean>);
    onChange(newCols);
  }, [ onChange ]);

  return (
    <PopoverRoot autoFocus>
      <PopoverTrigger>
        <Button
          variant="dropdown"
          size="sm"
          aria-label="Columns"
          minW="touchTarget"
          minH="touchTarget"
          px={{ base: 1, lg: 3 }}
        >
          <IconSvg name="columns" boxSize={ 5 } color="inherit"/>
          <chakra.span hideBelow="lg">Columns</chakra.span>
        </Button>
      </PopoverTrigger>
      <PopoverContent
        aria-label="Visible columns"
        maxH="min(var(--available-height, 100dvh), calc(100dvh - var(--chakra-spacing-8)))"
        overflowY="auto"
        overscrollBehavior="contain"
        maxW="calc(100dvw - var(--chakra-spacing-8))"
      >
        <PopoverBody px={ 4 } py={ 6 } display="flex" flexDir="column" rowGap={ 5 }>
          <CheckboxGroup
            defaultValue={ Object.keys(columns).filter((key) => columns[key as ColumnsIds]) }
            onValueChange={ handleValueChange }
            display="grid"
            gridTemplateColumns="repeat(auto-fit, minmax(min(100%, var(--chakra-spacing-40)), 1fr))"
            w="full"
            gap={ 3 }
          >
            { TABLE_COLUMNS.map(col => (
              <Checkbox
                key={ col.id }
                value={ col.id }
                size="md"
                minW="touchTarget"
                minH="touchTarget"
              >
                { col.id === 'or_and' ? 'And/Or' : col.name }
              </Checkbox>
            )) }
          </CheckboxGroup>
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default ColumnsButton;
