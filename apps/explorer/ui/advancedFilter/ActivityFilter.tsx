import { Flex, Text } from '@chakra-ui/react';
import React from 'react';

import { Button } from 'toolkit/chakra/button';
import { PopoverCloseTriggerWrapper } from 'toolkit/chakra/popover';

interface Props {
  title: string;
  isFilled?: boolean;
  isTouched?: boolean;
  hasReset?: boolean;
  onFilter: () => void;
  onReset?: () => void;
  children: React.ReactNode;
}

const ActivityFilter = ({ title, isFilled, isTouched, hasReset, onFilter, onReset, children }: Props) => (
  <>
    <Flex alignItems="center" justifyContent="space-between" flexWrap="wrap" gap={ 3 }>
      <Text color="text.secondary" fontWeight="semibold">{ title }</Text>
      { hasReset && (
        <Button type="button" variant="link" onClick={ onReset } disabled={ !isFilled } textStyle="sm">
          Reset
        </Button>
      ) }
    </Flex>
    { children }
    <PopoverCloseTriggerWrapper aria-label="Filter">
      <Button type="button" aria-label="Filter" disabled={ !isTouched } onClick={ onFilter } w="fit-content">
        Filter
      </Button>
    </PopoverCloseTriggerWrapper>
  </>
);

export default ActivityFilter;
